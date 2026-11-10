package upstreamcapture

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJob(isStream bool, request string, response string, contentType string, statusCode int, limit int) *job {
	s := &Session{
		newAPIRequestId: "20261106000000000000000abcd",
		channelId:       1,
		userId:          7,
		tokenId:         3,
		isStream:        isStream,
		startedAt:       time.Date(2026, 10, 20, 4, 8, 15, 316880413, time.UTC),
		maxBytes:        limit,
		request:         newCappedBuffer(limit),
	}
	_, _ = s.request.Write([]byte(request))
	resp := newCappedBuffer(limit)
	_, _ = resp.Write([]byte(response))
	return &job{session: s, response: resp, statusCode: statusCode, contentType: contentType}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestBuildRecordMatchesReferenceLayout(t *testing.T) {
	request := `{"model":"gpt","stream":true,"reasoning":{"effort":"xhigh"},"prompt_cache_key":"pck-1","input":"<b>&</b>"}`
	stream := `data: {"type":"response.completed","response":{"id":"resp_1","status":"completed"}}` + "\n\n"

	rec := buildRecord(newTestJob(true, request, stream, "text/event-stream", 200, 1<<20))
	body := string(rec.body)

	// Top-level keys appear in the reference order.
	order := []string{`"session_id"`, `"request_id"`, `"timestamp"`, `"reasoning_effort"`, `"request"`, `"response"`}
	last := -1
	for _, key := range order {
		idx := strings.Index(body, key)
		require.Greater(t, idx, last, "key %s out of order", key)
		last = idx
	}
	// The upstream request is embedded byte-for-byte (no HTML escaping or re-marshal).
	assert.Contains(t, body, `"request":`+request)

	var parsed struct {
		SessionId       string `json:"session_id"`
		RequestId       string `json:"request_id"`
		Timestamp       string `json:"timestamp"`
		ReasoningEffort string `json:"reasoning_effort"`
		Response        struct {
			ResponseData struct {
				Id string `json:"id"`
			} `json:"response_data"`
			StatusCode int `json:"status_code"`
		} `json:"response"`
		Truncated bool `json:"truncated"`
	}
	require.NoError(t, common.Unmarshal(rec.body, &parsed))
	assert.Equal(t, sha256Hex("pck-1"), parsed.SessionId)
	assert.Len(t, parsed.RequestId, 32)
	assert.Equal(t, rec.requestId, parsed.RequestId)
	assert.Equal(t, "2026-10-20T04:08:15.316880413Z", parsed.Timestamp)
	assert.Equal(t, "xhigh", parsed.ReasoningEffort)
	assert.Equal(t, "resp_1", parsed.Response.ResponseData.Id)
	assert.Zero(t, parsed.Response.StatusCode)
	assert.False(t, parsed.Truncated)
}

func TestSessionIdPriority(t *testing.T) {
	tests := []struct {
		name    string
		request string
		header  string
		wantKey string
	}{
		{name: "prompt_cache_key wins", request: `{"prompt_cache_key":"a","client_metadata":{"session_id":"b"}}`, header: "c", wantKey: "a"},
		{name: "client_metadata session", request: `{"client_metadata":{"session_id":"b"}}`, header: "c", wantKey: "b"},
		{name: "metadata session", request: `{"metadata":{"session_id":"m"}}`, header: "c", wantKey: "m"},
		{name: "claude code json user_id", request: `{"metadata":{"user_id":"{\"device_id\":\"d\",\"account_uuid\":\"\",\"session_id\":\"s-1\"}"}}`, header: "c", wantKey: "s-1"},
		{name: "claude code legacy user_id", request: `{"metadata":{"user_id":"user_abc_account_u1_session_s-2"}}`, header: "c", wantKey: "s-2"},
		{name: "plain user_id is not a session", request: `{"metadata":{"user_id":"alice"}}`, header: "c", wantKey: "c"},
		{name: "session header", request: `{}`, header: "c", wantKey: "c"},
		{name: "token and user fallback", request: `{}`, wantKey: "token:3:user:7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := newTestJob(false, tt.request, `{}`, "application/json", 200, 1<<20)
			j.session.sessionHeader = tt.header
			var parsed struct {
				SessionId string `json:"session_id"`
			}
			require.NoError(t, common.Unmarshal(buildRecord(j).body, &parsed))
			assert.Equal(t, sha256Hex(tt.wantKey), parsed.SessionId)
		})
	}
}

func TestBuildRecordNonJSONAndTruncatedBodies(t *testing.T) {
	j := newTestJob(false, "--boundary\r\nform-data", strings.Repeat("x", 64), "text/plain", 502, 16)

	var parsed struct {
		Request         string  `json:"request"`
		ReasoningEffort *string `json:"reasoning_effort"`
		Response        struct {
			ResponseData string `json:"response_data"`
			StatusCode   int    `json:"status_code"`
		} `json:"response"`
		Truncated bool `json:"truncated"`
	}
	require.NoError(t, common.Unmarshal(buildRecord(j).body, &parsed))
	assert.Equal(t, "--boundary\r\nform", parsed.Request)
	assert.Nil(t, parsed.ReasoningEffort)
	assert.Equal(t, strings.Repeat("x", 16), parsed.Response.ResponseData)
	assert.Equal(t, 502, parsed.Response.StatusCode)
	assert.True(t, parsed.Truncated)
}

func TestS3ObjectKey(t *testing.T) {
	rec := &builtRecord{requestId: "abc", channelId: 2, startedAt: time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC)}
	assert.Equal(t, "2026-11-06/channel_2/abc.json", s3ObjectKey("", rec))
	assert.Equal(t, "captures/2026-11-06/channel_2/abc.json", s3ObjectKey("/captures/", rec))
}
