package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToRequestPayloadUsesTopLevelSeedanceFields(t *testing.T) {
	generateAudio := true
	watermark := false
	req := relaycommon.TaskSubmitReq{
		Model:         "doubao-seedance-2-0-260128",
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      5,
		GenerateAudio: &generateAudio,
		Watermark:     &watermark,
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: "generate a video"},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"}, Role: "reference_video"},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)
	require.NotNil(t, payload.Duration)
	require.NotNil(t, payload.GenerateAudio)
	require.NotNil(t, payload.Watermark)
	assert.Equal(t, "720p", payload.Resolution)
	assert.Equal(t, "16:9", payload.Ratio)
	assert.Equal(t, 5, int(*payload.Duration))
	assert.True(t, bool(*payload.GenerateAudio))
	assert.False(t, bool(*payload.Watermark))
	assert.Equal(t, req.Content, payload.Content)
}

func TestConvertToRequestPayloadTopLevelFieldsOverrideMetadata(t *testing.T) {
	generateAudio := false
	watermark := true
	req := relaycommon.TaskSubmitReq{
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      5,
		GenerateAudio: &generateAudio,
		Watermark:     &watermark,
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: "top-level text"},
		},
		Metadata: map[string]interface{}{
			"resolution":     "1080p",
			"ratio":          "9:16",
			"duration":       10,
			"generate_audio": true,
			"watermark":      false,
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "metadata text"},
			},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)
	assert.Equal(t, "720p", payload.Resolution)
	assert.Equal(t, "16:9", payload.Ratio)
	assert.Equal(t, 5, int(*payload.Duration))
	assert.False(t, bool(*payload.GenerateAudio))
	assert.True(t, bool(*payload.Watermark))
	assert.Equal(t, req.Content, payload.Content)
}

func TestConvertToRequestPayloadPreservesCompleteSeedanceJSON(t *testing.T) {
	generateAudio := false
	watermark := false
	req := relaycommon.TaskSubmitReq{
		Model:         "doubao-seedance-2-0-260128",
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      5,
		GenerateAudio: &generateAudio,
		Watermark:     &watermark,
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: "generate a video"},
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: "https://example.com/person.jpg"}, Role: "reference_image"},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"}, Role: "reference_video"},
			{Type: "audio_url", AudioURL: &relaycommon.TaskMediaURL{URL: "https://example.com/mood.mp3"}, Role: "reference_audio"},
		},
	}

	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)
	data, err := common.Marshal(payload)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"model":"doubao-seedance-2-0-260128",
		"content":[
			{"type":"text","text":"generate a video"},
			{"type":"image_url","image_url":{"url":"https://example.com/person.jpg"},"role":"reference_image"},
			{"type":"video_url","video_url":{"url":"https://example.com/action.mp4"},"role":"reference_video"},
			{"type":"audio_url","audio_url":{"url":"https://example.com/mood.mp3"},"role":"reference_audio"}
		],
		"resolution":"720p",
		"ratio":"16:9",
		"duration":5,
		"generate_audio":false,
		"watermark":false
	}`, string(data))
}

func TestHasVideoInContentRequiresUsableURL(t *testing.T) {
	assert.True(t, hasVideoInContent([]relaycommon.TaskContentItem{{
		Type:     "video_url",
		VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"},
	}}))
	assert.False(t, hasVideoInContent([]relaycommon.TaskContentItem{{
		Type:     "video_url",
		VideoURL: &relaycommon.TaskMediaURL{},
	}}))
}
