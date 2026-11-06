package upstreamcapture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/google/uuid"
)

// builtRecord is a finished capture ready to be written to the sinks.
type builtRecord struct {
	requestId       string
	newAPIRequestId string
	channelId       int
	startedAt       time.Time
	body            []byte
}

// requestHints are the request JSON fields used to derive record metadata.
type requestHints struct {
	PromptCacheKey  string `json:"prompt_cache_key"`
	ReasoningEffort string `json:"reasoning_effort"`
	Reasoning       *struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	ClientMetadata *struct {
		SessionId string `json:"session_id"`
	} `json:"client_metadata"`
	Metadata *struct {
		SessionId string `json:"session_id"`
	} `json:"metadata"`
}

// buildRecord assembles the record with the same key order as the reference format:
// session_id, request_id, timestamp, reasoning_effort, request, response.response_data.
// Raw upstream JSON is written verbatim instead of being re-marshaled, so fields
// unknown to new-api's DTOs and the original byte content are preserved.
func buildRecord(j *job) *builtRecord {
	s := j.session
	requestBody, requestTruncated := s.request.snapshot()
	responseBody, responseTruncated := j.response.snapshot()

	var hints requestHints
	requestIsJSON := !requestTruncated && len(bytes.TrimSpace(requestBody)) > 0 && json.Valid(requestBody)
	if requestIsJSON {
		_ = common.Unmarshal(requestBody, &hints)
	}

	reasoningEffort := s.reasoningEffort
	if reasoningEffort == "" && hints.Reasoning != nil {
		reasoningEffort = hints.Reasoning.Effort
	}
	if reasoningEffort == "" {
		reasoningEffort = hints.ReasoningEffort
	}

	var responseData []byte
	if s.isStream && isEventStream(j.contentType, responseBody) {
		responseData = buildStreamResponseData(responseBody)
	} else {
		responseData = rawJSONOrString(responseBody, responseTruncated)
	}

	requestId := strings.ReplaceAll(uuid.NewString(), "-", "")

	var buf bytes.Buffer
	buf.Grow(len(requestBody) + len(responseData) + 512)
	buf.WriteString(`{"session_id":`)
	writeJSONString(&buf, sessionId(s, &hints))
	buf.WriteString(`,"request_id":`)
	writeJSONString(&buf, requestId)
	buf.WriteString(`,"timestamp":`)
	writeJSONString(&buf, s.startedAt.Format(time.RFC3339Nano))
	buf.WriteString(`,"reasoning_effort":`)
	if reasoningEffort == "" {
		buf.WriteString("null")
	} else {
		writeJSONString(&buf, reasoningEffort)
	}
	buf.WriteString(`,"request":`)
	if requestIsJSON {
		buf.Write(bytes.TrimSpace(requestBody))
	} else {
		buf.Write(rawJSONOrString(requestBody, true))
	}
	buf.WriteString(`,"response":{"response_data":`)
	buf.Write(responseData)
	if j.statusCode < 200 || j.statusCode >= 300 {
		buf.WriteString(`,"status_code":`)
		buf.WriteString(strconv.Itoa(j.statusCode))
	}
	buf.WriteString(`}`)
	if requestTruncated || responseTruncated {
		buf.WriteString(`,"truncated":true`)
	}
	buf.WriteString(`}`)

	return &builtRecord{
		requestId:       requestId,
		newAPIRequestId: s.newAPIRequestId,
		channelId:       s.channelId,
		startedAt:       s.startedAt,
		body:            buf.Bytes(),
	}
}

// sessionId hashes the most specific client conversation identifier available,
// so requests from one conversation share a session_id.
func sessionId(s *Session, hints *requestHints) string {
	var clientSession, metadataSession string
	if hints.ClientMetadata != nil {
		clientSession = hints.ClientMetadata.SessionId
	}
	if hints.Metadata != nil {
		metadataSession = hints.Metadata.SessionId
	}
	key := firstNonEmpty(hints.PromptCacheKey, clientSession, metadataSession, s.sessionHeader)
	if key == "" {
		key = "token:" + strconv.Itoa(s.tokenId) + ":user:" + strconv.Itoa(s.userId)
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func isEventStream(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return true
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:"))
}

// rawJSONOrString returns body verbatim when it is complete valid JSON,
// otherwise the body encoded as a JSON string.
func rawJSONOrString(body []byte, truncated bool) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return []byte("null")
	}
	if !truncated && json.Valid(trimmed) {
		return trimmed
	}
	var buf bytes.Buffer
	writeJSONString(&buf, string(body))
	return buf.Bytes()
}

func writeJSONString(buf *bytes.Buffer, s string) {
	encoded, err := common.Marshal(s)
	if err != nil {
		buf.WriteString(`""`)
		return
	}
	buf.Write(encoded)
}
