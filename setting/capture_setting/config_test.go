package capture_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldCapture(t *testing.T) {
	original := captureSetting
	t.Cleanup(func() {
		captureSetting = original
		RebuildFilter()
	})

	tests := []struct {
		name      string
		local     bool
		channels  []int
		excluded  []int
		models    []string
		channelId int
		userId    int
		origin    string
		upstream  string
		want      bool
	}{
		{name: "selected channel, local only", local: true, channels: []int{1, 2}, channelId: 1, userId: 10, want: true},
		{name: "unselected channel", local: true, channels: []int{1, 2}, channelId: 3, userId: 10, want: false},
		{name: "excluded user on selected channel", local: true, channels: []int{1}, excluded: []int{10}, channelId: 1, userId: 10, want: false},
		{name: "other user not excluded", local: true, channels: []int{1}, excluded: []int{10}, channelId: 1, userId: 11, want: true},
		{name: "local storage disabled", channels: []int{1}, channelId: 1, userId: 10, want: false},
		{name: "empty model list captures every model", local: true, channels: []int{1}, channelId: 1, userId: 10, origin: "gpt-6-sol", upstream: "gpt-6-sol", want: true},
		{name: "listed model", local: true, channels: []int{1}, models: []string{"claude-opus-5-5"}, channelId: 1, userId: 10, origin: "claude-opus-5-5", upstream: "claude-opus-5-5", want: true},
		{name: "unlisted model", local: true, channels: []int{1}, models: []string{"claude-opus-5-5"}, channelId: 1, userId: 10, origin: "gpt-6-sol", upstream: "gpt-6-sol", want: false},
		{name: "mapped upstream model listed", local: true, channels: []int{1}, models: []string{"gpt-6.1-sol"}, channelId: 1, userId: 10, origin: "gpt-6-sol", upstream: "gpt-6.1-sol", want: true},
		{name: "listed model on unselected channel", local: true, channels: []int{1}, models: []string{"gpt-6-sol"}, channelId: 2, userId: 10, origin: "gpt-6-sol", upstream: "gpt-6-sol", want: false},
		{name: "blank entries do not restrict", local: true, channels: []int{1}, models: []string{" "}, channelId: 1, userId: 10, origin: "gpt-6-sol", upstream: "gpt-6-sol", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureSetting.LocalEnabled = tt.local
			captureSetting.ChannelIds = tt.channels
			captureSetting.ExcludedUserIds = tt.excluded
			captureSetting.ModelNames = tt.models
			RebuildFilter()
			assert.Equal(t, tt.want, ShouldCapture(tt.channelId, tt.userId, tt.origin, tt.upstream))
		})
	}
}

func TestValidateOption(t *testing.T) {
	tests := []struct {
		key     string
		value   string
		wantErr bool
	}{
		{key: "channel_ids", value: "[1,2]"},
		{key: "channel_ids", value: "[]"},
		{key: "channel_ids", value: "[0]", wantErr: true},
		{key: "channel_ids", value: "1,2", wantErr: true},
		{key: "excluded_user_ids", value: "[5]"},
		{key: "excluded_user_ids", value: "[-1]", wantErr: true},
		{key: "max_body_mb", value: "0", wantErr: true},
		{key: "max_body_mb", value: "64"},
		{key: "queue_size", value: "1000"},
		{key: "queue_size", value: "abc", wantErr: true},
		{key: "model_names", value: `["claude-opus-5-5","gpt-6-sol"]`},
		{key: "model_names", value: "[]"},
		{key: "model_names", value: `[""]`, wantErr: true},
		{key: "model_names", value: "[1]", wantErr: true},
		{key: "model_names", value: "gpt-6-sol", wantErr: true},
		{key: "s3_bucket", value: "any"},
	}
	for _, tt := range tests {
		err := ValidateOption(tt.key, tt.value)
		if tt.wantErr {
			assert.Error(t, err, "%s=%s", tt.key, tt.value)
		} else {
			assert.NoError(t, err, "%s=%s", tt.key, tt.value)
		}
	}
}
