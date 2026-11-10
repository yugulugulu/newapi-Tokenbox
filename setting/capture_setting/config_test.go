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
		channelId int
		userId    int
		want      bool
	}{
		{name: "selected channel, local only", local: true, channels: []int{1, 2}, channelId: 1, userId: 10, want: true},
		{name: "unselected channel", local: true, channels: []int{1, 2}, channelId: 3, userId: 10, want: false},
		{name: "excluded user on selected channel", local: true, channels: []int{1}, excluded: []int{10}, channelId: 1, userId: 10, want: false},
		{name: "other user not excluded", local: true, channels: []int{1}, excluded: []int{10}, channelId: 1, userId: 11, want: true},
		{name: "local storage disabled", channels: []int{1}, channelId: 1, userId: 10, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureSetting.LocalEnabled = tt.local
			captureSetting.ChannelIds = tt.channels
			captureSetting.ExcludedUserIds = tt.excluded
			RebuildFilter()
			assert.Equal(t, tt.want, ShouldCapture(tt.channelId, tt.userId))
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
