package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTaskResolution(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		want    string
		wantErr bool
	}{
		{
			name: "outer resolution has priority",
			req: relaycommon.TaskSubmitReq{
				Resolution: " 720p ",
				Metadata:   map[string]interface{}{"resolution": "1080p"},
			},
			want: "720p",
		},
		{name: "metadata resolution is rejected", req: relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"resolution": "1080p"}}, wantErr: true},
		{name: "missing resolution is rejected", req: relaycommon.TaskSubmitReq{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTaskResolution(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

}

func TestTaskHasVideoInput(t *testing.T) {
	tests := []struct {
		name    string
		content []relaycommon.TaskContentItem
		want    bool
	}{
		{
			name: "valid video url",
			content: []relaycommon.TaskContentItem{{
				Type:     "video_url",
				VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/reference.mp4"},
			}},
			want: true,
		},
		{
			name:    "empty video url",
			content: []relaycommon.TaskContentItem{{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{}}},
			want:    false,
		},
		{
			name: "missing video is not video input",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := taskHasVideoInput(relaycommon.TaskSubmitReq{Content: tt.content})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := taskHasVideoInput(relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{
		"content": []interface{}{map[string]interface{}{
			"type":      "video_url",
			"video_url": map[string]interface{}{"url": "https://example.com/reference.mp4"},
		}},
	}})
	require.ErrorContains(t, err, "content is required at the top level")
}

func TestBuildTaskV2BillingRequestBody(t *testing.T) {
	body, err := buildTaskV2BillingRequestBody("720p", true)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"resolution":"720p",
		"content":[{"type":"video_url","video_url":{"url":"present"}}]
	}`, string(body))
}

func TestNormalizeTaskQuantity(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		want    int
		wantErr bool
	}{
		{name: "duration has priority", req: relaycommon.TaskSubmitReq{Duration: 5, Seconds: "10"}, want: 5},
		{name: "seconds without duration is rejected", req: relaycommon.TaskSubmitReq{Seconds: "10"}, wantErr: true},
		{name: "missing duration is rejected", req: relaycommon.TaskSubmitReq{}, wantErr: true},
		{name: "negative duration is invalid", req: relaycommon.TaskSubmitReq{Duration: -1}, wantErr: true},
		{name: "duration over limit is invalid", req: relaycommon.TaskSubmitReq{Duration: relaycommon.MaxTaskDurationSeconds + 1}, wantErr: true},
		{name: "seconds over limit is invalid", req: relaycommon.TaskSubmitReq{Seconds: "3601"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTaskQuantity(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
