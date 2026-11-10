package upstreamcapture

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/capture_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if r.Method == http.MethodPut {
		f.mu.Lock()
		f.objects[r.URL.Path] = body
		f.mu.Unlock()
	}
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3) get(path string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[path]
	return b, ok
}

// sendThroughCapture mimics doRequest: Begin before sending, WrapResponse after,
// then a relay handler reading and closing the upstream body.
func sendThroughCapture(t *testing.T, upstreamURL string, channelId, userId int, requestId string, isStream bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		UserId:      userId,
		TokenId:     9,
		RequestId:   requestId,
		IsStream:    isStream,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channelId},
	}
	reqBody := `{"model":"gpt-x","stream":true,"prompt_cache_key":"conv-1","reasoning":{"effort":"high"},"input":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, upstreamURL, strings.NewReader(reqBody))
	require.NoError(t, err)

	capture := Begin(c, info, req)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	capture.WrapResponse(resp)
	_, _ = io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
}

func TestCaptureFlowPersistsLocalAndS3ForSelectedChannelOnly(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"type":"response.output_text.delta","delta":"hi"}`+"\n\n")
		_, _ = io.WriteString(w, `data: {"type":"response.completed","response":{"id":"resp_flow","status":"completed","output":[{"type":"reasoning","encrypted_content":"enc"},{"type":"function_call","name":"ls","arguments":"{}"}]}}`+"\n\n")
	}))
	defer upstream.Close()
	s3 := &fakeS3{objects: map[string][]byte{}}
	s3Server := httptest.NewServer(s3)
	defer s3Server.Close()

	localDir := t.TempDir()
	setting := capture_setting.GetCaptureSetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
		capture_setting.RebuildFilter()
	})
	setting.ChannelIds = []int{1}
	setting.ExcludedUserIds = []int{66}
	setting.LocalEnabled = true
	setting.LocalDir = localDir
	setting.S3Enabled = true
	setting.S3Bucket = "bucket"
	setting.S3Region = "ap-southeast-1"
	setting.S3Endpoint = s3Server.URL
	setting.S3Prefix = "cap"
	setting.S3AccessKeyId = "AKIDTEST"
	setting.S3Secret = "secret"
	capture_setting.RebuildFilter()

	sendThroughCapture(t, upstream.URL, 1, 10, "reqselected", true)
	sendThroughCapture(t, upstream.URL, 2, 10, "requnselected", true)
	sendThroughCapture(t, upstream.URL, 1, 66, "reqexcluded", true)

	var localFile string
	require.Eventually(t, func() bool {
		matches, _ := filepath.Glob(filepath.Join(localDir, "*", "channel_1", "reqselected_*.json"))
		if len(matches) == 1 {
			localFile = matches[0]
			return true
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)

	data, err := os.ReadFile(localFile)
	require.NoError(t, err)
	var rec struct {
		SessionId       string `json:"session_id"`
		RequestId       string `json:"request_id"`
		ReasoningEffort string `json:"reasoning_effort"`
		Request         struct {
			Model string `json:"model"`
		} `json:"request"`
		Response struct {
			ResponseData struct {
				Id string `json:"id"`
			} `json:"response_data"`
		} `json:"response"`
	}
	require.NoError(t, common.Unmarshal(data, &rec))
	assert.Equal(t, sha256Hex("conv-1"), rec.SessionId)
	assert.Equal(t, "high", rec.ReasoningEffort)
	assert.Equal(t, "gpt-x", rec.Request.Model)
	assert.Equal(t, "resp_flow", rec.Response.ResponseData.Id)

	date := filepath.Base(filepath.Dir(filepath.Dir(localFile)))
	s3Path := "/bucket/cap/" + date + "/channel_1/" + rec.RequestId + ".json"
	require.Eventually(t, func() bool {
		_, ok := s3.get(s3Path)
		return ok
	}, 5*time.Second, 20*time.Millisecond)
	uploaded, _ := s3.get(s3Path)
	assert.Equal(t, data, uploaded)

	// Unselected channel and excluded user leave no trace.
	others, _ := filepath.Glob(filepath.Join(localDir, "*", "*", "*.json"))
	assert.Len(t, others, 1)
	s3.mu.Lock()
	assert.Len(t, s3.objects, 1)
	s3.mu.Unlock()
}
