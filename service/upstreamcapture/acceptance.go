package upstreamcapture

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// acceptanceItem is the subset of an assistant output (Claude message or
// content block, Responses output item, Chat message) that acceptance inspects.
type acceptanceItem struct {
	Type             string            `json:"type"`
	Signature        string            `json:"signature"`
	Data             string            `json:"data"`
	EncryptedContent string            `json:"encrypted_content"`
	Content          json.RawMessage   `json:"content"`
	ToolCalls        []json.RawMessage `json:"tool_calls"`
}

type acceptanceResponse struct {
	acceptanceItem
	Output  []acceptanceItem `json:"output"`
	Choices []struct {
		Message acceptanceItem `json:"message"`
	} `json:"choices"`
}

// isQualifiedRecord mirrors the downstream per-call acceptance check: the
// request carries a conversation, the upstream finished with a complete 2xx
// body, and the assistant output contains both a reasoning signature and a
// tool call. A record failing any rule is rejected by acceptance anyway.
func isQualifiedRecord(hasConversation bool, statusCode int, truncated bool, responseData []byte) bool {
	if !hasConversation || truncated || statusCode < 200 || statusCode >= 300 {
		return false
	}
	// Streams that never reached a terminal event fall back to a raw event array.
	if common.GetJsonType(responseData) != "object" {
		return false
	}
	var resp acceptanceResponse
	if err := common.Unmarshal(responseData, &resp); err != nil {
		return false
	}
	outputs := append([]acceptanceItem{resp.acceptanceItem}, resp.Output...)
	for _, choice := range resp.Choices {
		outputs = append(outputs, choice.Message)
	}
	hasSignature, hasToolCall := false, false
	for i := range outputs {
		hasSignature = hasSignature || outputs[i].hasReasoningSignature()
		hasToolCall = hasToolCall || outputs[i].hasToolCall()
	}
	return hasSignature && hasToolCall
}

func (item *acceptanceItem) hasReasoningSignature() bool {
	switch item.Type {
	case "thinking":
		if strings.TrimSpace(item.Signature) != "" {
			return true
		}
	case "redacted_thinking":
		if strings.TrimSpace(item.Data) != "" {
			return true
		}
	case "reasoning":
		if strings.TrimSpace(item.EncryptedContent) != "" {
			return true
		}
	}
	for _, block := range item.contentBlocks() {
		if block.hasReasoningSignature() {
			return true
		}
	}
	return false
}

func (item *acceptanceItem) hasToolCall() bool {
	if item.Type == "tool_use" || strings.HasSuffix(item.Type, "_call") || len(item.ToolCalls) > 0 {
		return true
	}
	for _, block := range item.contentBlocks() {
		if block.Type == "tool_use" {
			return true
		}
	}
	return false
}

// contentBlocks decodes content only when it is an array of blocks; string
// content (plain chat text) has no nested outputs.
func (item *acceptanceItem) contentBlocks() []acceptanceItem {
	if common.GetJsonType(item.Content) != "array" {
		return nil
	}
	var blocks []acceptanceItem
	if err := common.Unmarshal(item.Content, &blocks); err != nil {
		return nil
	}
	return blocks
}
