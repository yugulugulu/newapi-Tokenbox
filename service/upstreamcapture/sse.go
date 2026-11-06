package upstreamcapture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// buildStreamResponseData turns a raw SSE body into the final response object:
//   - OpenAI Responses API: the "response" of the terminal response.* event, verbatim
//   - Chat Completions: chunks merged into one chat.completion object
//   - Claude Messages: events merged into one message object
//
// Anything else (or a stream that cannot be merged) falls back to the array of
// raw data payloads so no information is lost.
func buildStreamResponseData(body []byte) []byte {
	payloads := ssePayloads(body)
	if len(payloads) == 0 {
		return rawJSONOrString(body, true)
	}
	if final := responsesFinalObject(payloads); final != nil {
		return final
	}
	switch streamKind(payloads) {
	case "chat":
		if merged, ok := mergeChatChunks(payloads); ok {
			return merged
		}
	case "claude":
		if merged, ok := mergeClaudeEvents(payloads); ok {
			return merged
		}
	}
	return rawPayloadArray(payloads)
}

// ssePayloads extracts every data: payload, skipping the [DONE] sentinel.
func ssePayloads(body []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64<<10), len(body)+1)
	var payloads []string
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		payloads = append(payloads, data)
	}
	return payloads
}

type sseEnvelope struct {
	Type     string          `json:"type"`
	Object   string          `json:"object"`
	Response json.RawMessage `json:"response"`
}

var responsesTerminalTypes = map[string]struct{}{
	"response.completed":  {},
	"response.done":       {},
	"response.incomplete": {},
	"response.failed":     {},
}

func responsesFinalObject(payloads []string) []byte {
	for i := len(payloads) - 1; i >= 0; i-- {
		var env sseEnvelope
		if err := common.UnmarshalJsonStr(payloads[i], &env); err != nil {
			continue
		}
		if _, ok := responsesTerminalTypes[env.Type]; !ok {
			continue
		}
		if common.GetJsonType(env.Response) == "object" {
			return bytes.TrimSpace(env.Response)
		}
	}
	return nil
}

func streamKind(payloads []string) string {
	for _, p := range payloads {
		var env sseEnvelope
		if err := common.UnmarshalJsonStr(p, &env); err != nil {
			continue
		}
		if env.Object == "chat.completion.chunk" {
			return "chat"
		}
		if env.Type == "message_start" {
			return "claude"
		}
	}
	return ""
}

type chatChunk struct {
	Id                string          `json:"id"`
	Created           int64           `json:"created"`
	Model             string          `json:"model"`
	SystemFingerprint string          `json:"system_fingerprint"`
	Usage             json.RawMessage `json:"usage"`
	Choices           []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string  `json:"role"`
			Content          *string `json:"content"`
			ReasoningContent *string `json:"reasoning_content"`
			Reasoning        *string `json:"reasoning"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				Id       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type chatToolCallOut struct {
	Index    int    `json:"index"`
	Id       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatChoiceOut struct {
	Index   int `json:"index"`
	Message struct {
		Role             string            `json:"role"`
		Content          string            `json:"content"`
		ReasoningContent string            `json:"reasoning_content,omitempty"`
		ToolCalls        []chatToolCallOut `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason *string `json:"finish_reason"`

	content   strings.Builder
	reasoning strings.Builder
	toolCalls map[int]*chatToolCallOut
}

type chatCompletionOut struct {
	Id                string           `json:"id"`
	Object            string           `json:"object"`
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	SystemFingerprint string           `json:"system_fingerprint,omitempty"`
	Choices           []*chatChoiceOut `json:"choices"`
	Usage             json.RawMessage  `json:"usage,omitempty"`
}

func mergeChatChunks(payloads []string) ([]byte, bool) {
	out := &chatCompletionOut{Object: "chat.completion"}
	choices := map[int]*chatChoiceOut{}
	parsed := 0
	for _, p := range payloads {
		var chunk chatChunk
		if err := common.UnmarshalJsonStr(p, &chunk); err != nil {
			continue
		}
		parsed++
		if out.Id == "" {
			out.Id = chunk.Id
		}
		if out.Created == 0 {
			out.Created = chunk.Created
		}
		if chunk.Model != "" {
			out.Model = chunk.Model
		}
		if chunk.SystemFingerprint != "" {
			out.SystemFingerprint = chunk.SystemFingerprint
		}
		if common.GetJsonType(chunk.Usage) == "object" {
			out.Usage = chunk.Usage
		}
		for _, ch := range chunk.Choices {
			choice := choices[ch.Index]
			if choice == nil {
				choice = &chatChoiceOut{Index: ch.Index, toolCalls: map[int]*chatToolCallOut{}}
				choice.Message.Role = "assistant"
				choices[ch.Index] = choice
			}
			if ch.Delta.Role != "" {
				choice.Message.Role = ch.Delta.Role
			}
			if ch.Delta.Content != nil {
				choice.content.WriteString(*ch.Delta.Content)
			}
			if ch.Delta.ReasoningContent != nil {
				choice.reasoning.WriteString(*ch.Delta.ReasoningContent)
			} else if ch.Delta.Reasoning != nil {
				choice.reasoning.WriteString(*ch.Delta.Reasoning)
			}
			for _, tc := range ch.Delta.ToolCalls {
				call := choice.toolCalls[tc.Index]
				if call == nil {
					call = &chatToolCallOut{Index: tc.Index, Type: "function"}
					choice.toolCalls[tc.Index] = call
				}
				if tc.Id != "" {
					call.Id = tc.Id
				}
				if tc.Type != "" {
					call.Type = tc.Type
				}
				if tc.Function.Name != "" {
					call.Function.Name = tc.Function.Name
				}
				call.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != nil {
				choice.FinishReason = ch.FinishReason
			}
		}
	}
	if parsed == 0 {
		return nil, false
	}

	indexes := make([]int, 0, len(choices))
	for idx := range choices {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out.Choices = make([]*chatChoiceOut, 0, len(indexes))
	for _, idx := range indexes {
		choice := choices[idx]
		choice.Message.Content = choice.content.String()
		choice.Message.ReasoningContent = choice.reasoning.String()
		callIndexes := make([]int, 0, len(choice.toolCalls))
		for ci := range choice.toolCalls {
			callIndexes = append(callIndexes, ci)
		}
		sort.Ints(callIndexes)
		for _, ci := range callIndexes {
			choice.Message.ToolCalls = append(choice.Message.ToolCalls, *choice.toolCalls[ci])
		}
		out.Choices = append(out.Choices, choice)
	}
	merged, err := common.Marshal(out)
	if err != nil {
		return nil, false
	}
	return merged, true
}

type claudeEvent struct {
	Type         string                     `json:"type"`
	Index        int                        `json:"index"`
	Message      map[string]json.RawMessage `json:"message"`
	ContentBlock map[string]json.RawMessage `json:"content_block"`
	Delta        struct {
		Type         string          `json:"type"`
		Text         string          `json:"text"`
		Thinking     string          `json:"thinking"`
		Signature    string          `json:"signature"`
		PartialJson  string          `json:"partial_json"`
		StopReason   json.RawMessage `json:"stop_reason"`
		StopSequence json.RawMessage `json:"stop_sequence"`
	} `json:"delta"`
	Usage map[string]json.RawMessage `json:"usage"`
}

type claudeBlock struct {
	fields    map[string]json.RawMessage
	text      strings.Builder
	thinking  strings.Builder
	signature string
	inputJson strings.Builder
}

func mergeClaudeEvents(payloads []string) ([]byte, bool) {
	var message map[string]json.RawMessage
	usage := map[string]json.RawMessage{}
	blocks := map[int]*claudeBlock{}

	for _, p := range payloads {
		var ev claudeEvent
		if err := common.UnmarshalJsonStr(p, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			message = ev.Message
			if raw, ok := message["usage"]; ok {
				_ = common.Unmarshal(raw, &usage)
			}
		case "content_block_start":
			blocks[ev.Index] = &claudeBlock{fields: ev.ContentBlock}
		case "content_block_delta":
			block := blocks[ev.Index]
			if block == nil {
				block = &claudeBlock{fields: map[string]json.RawMessage{}}
				blocks[ev.Index] = block
			}
			switch ev.Delta.Type {
			case "text_delta":
				block.text.WriteString(ev.Delta.Text)
			case "thinking_delta":
				block.thinking.WriteString(ev.Delta.Thinking)
			case "signature_delta":
				block.signature += ev.Delta.Signature
			case "input_json_delta":
				block.inputJson.WriteString(ev.Delta.PartialJson)
			}
		case "message_delta":
			if message == nil {
				continue
			}
			if len(ev.Delta.StopReason) > 0 {
				message["stop_reason"] = ev.Delta.StopReason
			}
			if len(ev.Delta.StopSequence) > 0 {
				message["stop_sequence"] = ev.Delta.StopSequence
			}
			for k, v := range ev.Usage {
				usage[k] = v
			}
		}
	}
	if message == nil {
		return nil, false
	}

	indexes := make([]int, 0, len(blocks))
	for idx := range blocks {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	content := make([]map[string]json.RawMessage, 0, len(indexes))
	for _, idx := range indexes {
		block := blocks[idx]
		fields := block.fields
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
		if block.text.Len() > 0 {
			fields["text"] = mustMarshalString(block.text.String())
		}
		if block.thinking.Len() > 0 {
			fields["thinking"] = mustMarshalString(block.thinking.String())
		}
		if block.signature != "" {
			fields["signature"] = mustMarshalString(block.signature)
		}
		if block.inputJson.Len() > 0 {
			input := []byte(block.inputJson.String())
			if json.Valid(input) {
				fields["input"] = input
			} else {
				fields["input"] = mustMarshalString(block.inputJson.String())
			}
		}
		content = append(content, fields)
	}

	contentJson, err := common.Marshal(content)
	if err != nil {
		return nil, false
	}
	message["content"] = contentJson
	if len(usage) > 0 {
		usageJson, err := common.Marshal(usage)
		if err != nil {
			return nil, false
		}
		message["usage"] = usageJson
	}
	merged, err := common.Marshal(message)
	if err != nil {
		return nil, false
	}
	return merged, true
}

func rawPayloadArray(payloads []string) []byte {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, p := range payloads {
		if i > 0 {
			buf.WriteByte(',')
		}
		if json.Valid([]byte(p)) {
			buf.WriteString(p)
		} else {
			writeJSONString(&buf, p)
		}
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

func mustMarshalString(s string) json.RawMessage {
	encoded, err := common.Marshal(s)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}
