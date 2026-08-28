package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeVideoInputDurationsRoundsTotalAfterAccumulation(t *testing.T) {
	durations := map[string]float64{
		"https://example.com/one.mp4": 10.2,
		"https://example.com/two.mp4": 20.1,
	}

	got, err := probeVideoInputDurations(context.Background(), []string{
		"https://example.com/one.mp4",
		"https://example.com/two.mp4",
	}, func(_ context.Context, videoURL string) (float64, int64, error) {
		return durations[videoURL], 1024, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 31.0, got)
}

func TestProbeVideoInputDurationsCountsRepeatedVideoOccurrences(t *testing.T) {
	probeCalls := 0

	got, err := probeVideoInputDurations(context.Background(), []string{
		"https://example.com/repeated.mp4",
		"https://example.com/repeated.mp4",
	}, func(_ context.Context, _ string) (float64, int64, error) {
		probeCalls++
		return 5.1, 1024, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 11.0, got)
	assert.Equal(t, 1, probeCalls)
}

func TestProbeVideoInputDurationsRejectsTotalOverLimit(t *testing.T) {
	_, err := probeVideoInputDurations(context.Background(), []string{
		"https://example.com/one.mp4",
		"https://example.com/two.mp4",
	}, func(_ context.Context, _ string) (float64, int64, error) {
		return 15.51, 1024, nil
	})

	var probeErr *VideoInputProbeError
	require.ErrorAs(t, err, &probeErr)
	assert.Equal(t, http.StatusBadRequest, probeErr.StatusCode)
	assert.ErrorContains(t, err, "between 1 and 31 seconds")
}

func TestProbeVideoInputDurationsRejectsInvalidProbedDuration(t *testing.T) {
	for _, duration := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		t.Run(fmt.Sprintf("duration_%v", duration), func(t *testing.T) {
			_, err := probeVideoInputDurations(context.Background(), []string{
				"https://example.com/video.mp4",
			}, func(_ context.Context, _ string) (float64, int64, error) {
				return duration, 1024, nil
			})

			var probeErr *VideoInputProbeError
			require.ErrorAs(t, err, &probeErr)
			assert.Equal(t, http.StatusBadRequest, probeErr.StatusCode)
			assert.ErrorContains(t, err, "duration is invalid")
		})
	}
}

func TestProbeVideoInputDurationsRejectsTotalDownloadOverLimit(t *testing.T) {
	_, err := probeVideoInputDurations(context.Background(), []string{
		"https://example.com/one.mp4",
		"https://example.com/two.mp4",
	}, func(_ context.Context, _ string) (float64, int64, error) {
		return 1, maxVideoInputTotalBytes/2 + 1, nil
	})

	var probeErr *VideoInputProbeError
	require.ErrorAs(t, err, &probeErr)
	assert.Equal(t, http.StatusBadRequest, probeErr.StatusCode)
	assert.ErrorContains(t, err, "total video input size must not exceed 300MB")
}
