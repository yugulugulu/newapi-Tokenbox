package upstreamcapture

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/capture_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const conversationRequest = `{"model":"m","messages":[{"role":"user","content":"hi"}]}`

func TestIsQualifiedRecordMirrorsAcceptance(t *testing.T) {
	tests := []struct {
		name         string
		request      string
		statusCode   int
		truncated    bool
		responseData string
		want         bool
	}{
		{name: "claude thinking signature and tool_use", request: conversationRequest, statusCode: 200, want: true,
			responseData: `{"type":"message","role":"assistant","content":[{"type":"thinking","thinking":"t","signature":"sig"},{"type":"tool_use","id":"tu","name":"ls","input":{}}]}`},
		{name: "claude redacted thinking and tool_use", request: conversationRequest, statusCode: 200, want: true,
			responseData: `{"type":"message","content":[{"type":"redacted_thinking","data":"x"},{"type":"tool_use","name":"ls"}]}`},
		{name: "claude tool_use step without thinking", request: conversationRequest, statusCode: 200, want: false,
			responseData: `{"type":"message","content":[{"type":"tool_use","name":"ls","input":{}}]}`},
		{name: "claude thinking without signature", request: conversationRequest, statusCode: 200, want: false,
			responseData: `{"type":"message","content":[{"type":"thinking","thinking":"t","signature":""},{"type":"tool_use","name":"ls"}]}`},
		{name: "claude final text turn", request: conversationRequest, statusCode: 200, want: false,
			responseData: `{"type":"message","content":[{"type":"thinking","signature":"sig"},{"type":"text","text":"done"}]}`},
		{name: "responses reasoning and function_call", request: `{"input":[{"role":"user","content":"hi"}]}`, statusCode: 200, want: true,
			responseData: `{"object":"response","output":[{"type":"reasoning","encrypted_content":"enc"},{"type":"function_call","name":"ls"}]}`},
		{name: "responses reasoning and custom_tool_call", request: `{"input":"hi"}`, statusCode: 200, want: true,
			responseData: `{"object":"response","output":[{"type":"reasoning","encrypted_content":"enc"},{"type":"custom_tool_call","name":"apply_patch"}]}`},
		{name: "responses reasoning without encrypted content", request: `{"input":"hi"}`, statusCode: 200, want: false,
			responseData: `{"object":"response","output":[{"type":"reasoning","summary":[]},{"type":"function_call","name":"ls"}]}`},
		{name: "responses message only", request: `{"input":"hi"}`, statusCode: 200, want: false,
			responseData: `{"object":"response","output":[{"type":"message","content":[{"type":"output_text","text":"hi"}]}]}`},
		{name: "chat completion cannot carry signatures", request: conversationRequest, statusCode: 200, want: false,
			responseData: `{"object":"chat.completion","choices":[{"message":{"role":"assistant","content":"","reasoning_content":"r","tool_calls":[{"id":"c","type":"function"}]}}]}`},
		{name: "unfinished stream fell back to event array", request: `{"input":"hi"}`, statusCode: 200, want: false,
			responseData: `[{"type":"response.created"},{"type":"codex.rate_limits"}]`},
		{name: "upstream error status", request: conversationRequest, statusCode: 500, want: false,
			responseData: `{"type":"message","content":[{"type":"thinking","signature":"sig"},{"type":"tool_use","name":"ls"}]}`},
		{name: "truncated body", request: conversationRequest, statusCode: 200, truncated: true, want: false,
			responseData: `{"type":"message","content":[{"type":"thinking","signature":"sig"},{"type":"tool_use","name":"ls"}]}`},
		{name: "empty conversation is noise", request: `{"messages":[],"input":""}`, statusCode: 200, want: false,
			responseData: `{"type":"message","content":[{"type":"thinking","signature":"sig"},{"type":"tool_use","name":"ls"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hints requestHints
			require.NoError(t, common.UnmarshalJsonStr(tt.request, &hints))
			got := isQualifiedRecord(hints.hasConversation(), tt.statusCode, tt.truncated, []byte(tt.responseData))
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPersistOnlyQualifiedRecords covers the sink decision synchronously: with
// only_qualified on, an unqualified record is dropped and a qualified one is written;
// with it off, every record is written.
func TestPersistOnlyQualifiedRecords(t *testing.T) {
	setting := capture_setting.GetCaptureSetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
		capture_setting.RebuildFilter()
	})
	qualified := `{"type":"message","content":[{"type":"thinking","signature":"sig"},{"type":"tool_use","name":"ls"}]}`
	unqualified := `{"type":"message","content":[{"type":"text","text":"done"}]}`

	for _, onlyQualified := range []bool{true, false} {
		localDir := t.TempDir()
		setting.LocalEnabled = true
		setting.LocalDir = localDir
		setting.OnlyQualified = onlyQualified

		persist(newTestJob(false, conversationRequest, qualified, "application/json", 200, 1<<20))
		persist(newTestJob(false, conversationRequest, unqualified, "application/json", 200, 1<<20))

		files, err := filepath.Glob(filepath.Join(localDir, rawDirName, "*", "*", "*.json"))
		require.NoError(t, err)
		if onlyQualified {
			require.Len(t, files, 1)
			data, err := os.ReadFile(files[0])
			require.NoError(t, err)
			assert.Contains(t, string(data), `"signature":"sig"`)
		} else {
			assert.Len(t, files, 2)
		}
	}
}

func TestBeginSkipsChatCompletionsWhenOnlyQualified(t *testing.T) {
	setting := capture_setting.GetCaptureSetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
		capture_setting.RebuildFilter()
	})
	setting.ChannelIds = []int{1}
	setting.LocalEnabled = true
	capture_setting.RebuildFilter()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UserId: 10, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 1}}
	newReq := func(path string) *http.Request {
		req, err := http.NewRequest(http.MethodPost, "https://upstream.example"+path, nil)
		require.NoError(t, err)
		return req
	}

	setting.OnlyQualified = true
	assert.Nil(t, Begin(c, info, newReq("/v1/chat/completions")))
	assert.NotNil(t, Begin(c, info, newReq("/v1/responses")))
	assert.NotNil(t, Begin(c, info, newReq("/v1/messages")))

	setting.OnlyQualified = false
	assert.NotNil(t, Begin(c, info, newReq("/v1/chat/completions")))
}
