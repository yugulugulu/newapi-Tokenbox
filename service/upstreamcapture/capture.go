// Package upstreamcapture records the exact request body sent to selected upstream
// channels together with the upstream response body, then persists the record
// asynchronously to local disk and/or S3 according to capture_setting.
package upstreamcapture

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/capture_setting"

	"github.com/gin-gonic/gin"
)

// Session holds everything copied out of the request context at send time.
// The gin context is recycled after the relay finishes, so nothing here may
// reference it once Begin returns.
type Session struct {
	newAPIRequestId string
	channelId       int
	userId          int
	tokenId         int
	isStream        bool
	reasoningEffort string
	sessionHeader   string
	startedAt       time.Time
	maxBytes        int

	request *cappedBuffer
}

// Begin starts a capture for this upstream attempt, or returns nil when the
// channel/user is not selected. It must run before the request is sent.
func Begin(c *gin.Context, info *relaycommon.RelayInfo, req *http.Request) *Session {
	if info == nil || info.ChannelMeta == nil || info.IsChannelTest || req == nil {
		return nil
	}
	if !capture_setting.ShouldCapture(info.ChannelId, info.UserId) {
		return nil
	}

	maxBytes := capture_setting.GetCaptureSetting().MaxBodyBytes()
	s := &Session{
		newAPIRequestId: info.RequestId,
		channelId:       info.ChannelId,
		userId:          info.UserId,
		tokenId:         info.TokenId,
		isStream:        info.IsStream,
		reasoningEffort: info.ReasoningEffort,
		startedAt:       time.Now().UTC(),
		maxBytes:        maxBytes,
		request:         newCappedBuffer(maxBytes),
	}
	if c != nil && c.Request != nil {
		s.sessionHeader = firstNonEmpty(c.Request.Header.Get("session_id"), c.Request.Header.Get("X-Session-Id"))
	}

	if req.Body == nil || req.Body == http.NoBody {
		return s
	}
	// Replayable bodies can be re-read without disturbing the transport.
	if req.GetBody != nil {
		if rc, err := req.GetBody(); err == nil {
			_, _ = io.Copy(s.request, rc)
			_ = rc.Close()
			return s
		}
	}
	// Otherwise tee the body while the transport sends it.
	req.Body = &teeReadCloser{rc: req.Body, sink: s.request}
	return s
}

// WrapResponse tees the upstream response body. When the relay handler closes
// the body (after a non-stream read or at the end of a stream), the captured
// record is handed to the background writers exactly once.
func (s *Session) WrapResponse(resp *http.Response) {
	if s == nil || resp == nil || resp.Body == nil {
		return
	}
	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")
	response := newCappedBuffer(s.maxBytes)
	resp.Body = &teeReadCloser{
		rc:   resp.Body,
		sink: response,
		onClose: func() {
			enqueue(&job{
				session:     s,
				response:    response,
				statusCode:  statusCode,
				contentType: contentType,
			})
		},
	}
}

type job struct {
	session     *Session
	response    *cappedBuffer
	statusCode  int
	contentType string
}

// cappedBuffer keeps at most limit bytes and remembers whether it dropped any.
// The transport may write the request body from its own goroutine, hence the lock.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		if len(p) > 0 {
			b.truncated = true
		}
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

func (b *cappedBuffer) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes(), b.truncated
}

type teeReadCloser struct {
	rc      io.ReadCloser
	sink    io.Writer
	once    sync.Once
	onClose func()
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		_, _ = t.sink.Write(p[:n])
	}
	return n, err
}

func (t *teeReadCloser) Close() error {
	err := t.rc.Close()
	t.once.Do(func() {
		if t.onClose == nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				common.SysError("upstream capture: panic while finishing capture")
			}
		}()
		t.onClose()
	})
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
