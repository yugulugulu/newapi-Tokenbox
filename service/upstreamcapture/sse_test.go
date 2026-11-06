package upstreamcapture

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildStreamResponseDataResponsesKeepsCompletedObjectVerbatim(t *testing.T) {
	completed := `{"id":"resp_1","object":"response","status":"completed","access_programs":{"cyber":"standard"},"usage":{"input_tokens":3,"attribution":{"x":1}}}`
	body := "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}` + "\n\n" +
		"event: response.output_text.delta\n" +
		`data: {"type":"response.output_text.delta","delta":"hi"}` + "\n\n" +
		"event: response.completed\n" +
		`data: {"type":"response.completed","response":` + completed + `}` + "\n\n"

	got := buildStreamResponseData([]byte(body))

	assert.Equal(t, completed, string(got))
}

func TestBuildStreamResponseDataMergesChatChunks(t *testing.T) {
	body := `data: {"id":"c1","object":"chat.completion.chunk","created":10,"model":"gpt","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel","reasoning_content":"think"}}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":10,"model":"gpt","choices":[{"index":0,"delta":{"content":"lo","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"f","arguments":"{\"a\":"}}]}}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":10,"model":"gpt","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":10,"model":"gpt","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}` + "\n\n" +
		"data: [DONE]\n\n"

	got := buildStreamResponseData([]byte(body))

	var merged struct {
		Id      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					Id       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(got, &merged))
	assert.Equal(t, "c1", merged.Id)
	assert.Equal(t, "chat.completion", merged.Object)
	require.Len(t, merged.Choices, 1)
	choice := merged.Choices[0]
	assert.Equal(t, "assistant", choice.Message.Role)
	assert.Equal(t, "Hello", choice.Message.Content)
	assert.Equal(t, "think", choice.Message.ReasoningContent)
	require.Len(t, choice.Message.ToolCalls, 1)
	assert.Equal(t, "call_1", choice.Message.ToolCalls[0].Id)
	assert.Equal(t, "f", choice.Message.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"a":1}`, choice.Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "tool_calls", choice.FinishReason)
	assert.Equal(t, 7, merged.Usage.TotalTokens)
}

func TestBuildStreamResponseDataMergesClaudeEvents(t *testing.T) {
	body := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[],"stop_reason":null,"usage":{"input_tokens":12,"output_tokens":1}}}` + "\n\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}` + "\n\n" +
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hi "}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"there"}}` + "\n\n" +
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"tu_1","name":"get","input":{}}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"x\"}"}}` + "\n\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":9}}` + "\n\n" +
		`data: {"type":"message_stop"}` + "\n\n"

	got := buildStreamResponseData([]byte(body))

	var merged struct {
		Id         string `json:"id"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type      string         `json:"type"`
			Text      string         `json:"text"`
			Thinking  string         `json:"thinking"`
			Signature string         `json:"signature"`
			Name      string         `json:"name"`
			Input     map[string]any `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(got, &merged))
	assert.Equal(t, "msg_1", merged.Id)
	assert.Equal(t, "tool_use", merged.StopReason)
	require.Len(t, merged.Content, 3)
	assert.Equal(t, "hmm", merged.Content[0].Thinking)
	assert.Equal(t, "sig", merged.Content[0].Signature)
	assert.Equal(t, "Hi there", merged.Content[1].Text)
	assert.Equal(t, "get", merged.Content[2].Name)
	assert.Equal(t, map[string]any{"q": "x"}, merged.Content[2].Input)
	assert.Equal(t, 12, merged.Usage.InputTokens)
	assert.Equal(t, 9, merged.Usage.OutputTokens)
}

func TestBuildStreamResponseDataFallsBackToRawEvents(t *testing.T) {
	body := `data: {"candidates":[{"content":{"parts":[{"text":"a"}]}}]}` + "\n\n" +
		"data: not-json\n\n" +
		`data: {"type":"response.output_text.delta","delta":"cut off"}` + "\n\n"

	got := buildStreamResponseData([]byte(body))

	assert.JSONEq(t, `[{"candidates":[{"content":{"parts":[{"text":"a"}]}}]},"not-json",{"type":"response.output_text.delta","delta":"cut off"}]`, string(got))
}
