package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	MaxVideoInputDurationSeconds = 31
	maxVideoInputFileBytes       = 100 * 1024 * 1024
	maxVideoInputTotalBytes      = 300 * 1024 * 1024
	videoProbeQueueTimeout       = 10 * time.Second
	videoProbeFileTimeout        = 30 * time.Second
	videoProbeTotalTimeout       = 60 * time.Second
)

var (
	videoProbeSlots       = make(chan struct{}, 2)
	publicVideoClientOnce sync.Once
	publicVideoClient     *http.Client
)

// VideoInputProbeError classifies failures before a task is sent upstream.
type VideoInputProbeError struct {
	StatusCode int
	Err        error
}

func (e *VideoInputProbeError) Error() string {
	if e == nil || e.Err == nil {
		return "video input probe failed"
	}
	return e.Err.Error()
}

func (e *VideoInputProbeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func videoProbeClient() *http.Client {
	publicVideoClientOnce.Do(func() {
		protection := &common.SSRFProtection{
			AllowPrivateIp:         false,
			DomainFilterMode:       false,
			IpFilterMode:           false,
			AllowedPorts:           []int{80, 443},
			ApplyIPFilterForDomain: true,
		}
		publicVideoClient = newProtectedFetchHTTPClientWithProxy(
			nil,
			nil,
			func() (*common.SSRFProtection, bool, error) { return protection, true, nil },
			func(*http.Request) (*url.URL, error) { return nil, nil },
		)
		publicVideoClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if err := protection.ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		}
	})
	return publicVideoClient
}

// ProbeVideoInputDurations downloads and probes public video URLs before
// billing. It returns the sum rounded up to whole seconds.
func ProbeVideoInputDurations(ctx context.Context, videoURLs []string) (float64, error) {
	if len(videoURLs) == 0 {
		return 0, nil
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, &VideoInputProbeError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("ffprobe is not available: %w", err)}
	}

	queueCtx, cancelQueue := context.WithTimeout(ctx, videoProbeQueueTimeout)
	defer cancelQueue()
	select {
	case videoProbeSlots <- struct{}{}:
		defer func() { <-videoProbeSlots }()
	case <-queueCtx.Done():
		return 0, &VideoInputProbeError{StatusCode: http.StatusServiceUnavailable, Err: fmt.Errorf("video duration probe is busy")}
	}

	totalCtx, cancelTotal := context.WithTimeout(ctx, videoProbeTotalTimeout)
	defer cancelTotal()
	return probeVideoInputDurations(totalCtx, videoURLs, func(ctx context.Context, videoURL string) (float64, int64, error) {
		return downloadAndProbeVideo(ctx, ffprobePath, videoURL)
	})
}

func probeVideoInputDurations(ctx context.Context, videoURLs []string, probe func(context.Context, string) (float64, int64, error)) (float64, error) {
	urlCounts := make(map[string]int, len(videoURLs))
	for _, rawURL := range videoURLs {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("video input URL must not be empty")}
		}
		urlCounts[trimmed]++
	}

	var totalBytes int64
	var totalDuration float64
	for videoURL, count := range urlCounts {
		fileCtx, cancelFile := context.WithTimeout(ctx, videoProbeFileTimeout)
		duration, size, err := probe(fileCtx, videoURL)
		cancelFile()
		if err != nil {
			return 0, err
		}
		if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
			return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("video input duration is invalid")}
		}
		if size < 0 {
			return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("video input size is invalid")}
		}
		totalBytes += size
		if totalBytes > maxVideoInputTotalBytes {
			return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("total video input size must not exceed 300MB")}
		}
		totalDuration += duration * float64(count)
		if math.IsNaN(totalDuration) || math.IsInf(totalDuration, 0) {
			return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("video input duration is invalid")}
		}
	}

	billableSeconds := math.Ceil(totalDuration)
	if billableSeconds <= 0 || billableSeconds > MaxVideoInputDurationSeconds {
		return 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("total video input duration must be between 1 and %d seconds", MaxVideoInputDurationSeconds)}
	}
	return billableSeconds, nil
}

func downloadAndProbeVideo(ctx context.Context, ffprobePath, videoURL string) (float64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("invalid video input URL: %w", err)}
	}
	resp, err := videoProbeClient().Do(req)
	if err != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("failed to download video input: %w", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("failed to download video input: HTTP %d", resp.StatusCode)}
	}
	if resp.ContentLength > maxVideoInputFileBytes {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("single video input size must not exceed 100MB")}
	}

	tmp, err := os.CreateTemp("", "new-api-video-probe-*")
	if err != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("failed to create video probe file: %w", err)}
	}
	path := tmp.Name()
	defer os.Remove(path)

	written, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, maxVideoInputFileBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("failed to read video input: %w", copyErr)}
	}
	if closeErr != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("failed to close video probe file: %w", closeErr)}
	}
	if written > maxVideoInputFileBytes {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("single video input size must not exceed 100MB")}
	}

	output, err := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("failed to probe video input duration: %w", err)}
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, 0, &VideoInputProbeError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("video input duration is invalid")}
	}
	return duration, written, nil
}
