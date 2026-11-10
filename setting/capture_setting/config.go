package capture_setting

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ConfigName = "capture_setting"

// CaptureSetting controls upstream request/response capture for selected channels.
type CaptureSetting struct {
	// ChannelIds lists the channels whose upstream traffic is captured.
	ChannelIds []int `json:"channel_ids"`
	// ExcludedUserIds lists users whose requests are never captured, even on captured channels.
	ExcludedUserIds []int `json:"excluded_user_ids"`

	LocalEnabled bool `json:"local_enabled"`
	// LocalDir is the root directory for local capture files; empty means <log-dir>/captures.
	LocalDir string `json:"local_dir"`

	S3Enabled     bool   `json:"s3_enabled"`
	S3Bucket      string `json:"s3_bucket"`
	S3Region      string `json:"s3_region"`
	S3Endpoint    string `json:"s3_endpoint"`
	S3Prefix      string `json:"s3_prefix"`
	S3AccessKeyId string `json:"s3_access_key_id"`
	// S3Secret must keep the "secret" suffix so GET /api/option hides it.
	S3Secret string `json:"s3_secret"`

	// OnlyQualified keeps only records that can pass downstream acceptance:
	// a complete 2xx response whose assistant output carries both a reasoning
	// signature and a tool call. Chat Completions traffic is skipped up front
	// because that format cannot carry reasoning signatures.
	OnlyQualified bool `json:"only_qualified"`

	// MaxBodyMB caps the captured request and response body sizes separately.
	MaxBodyMB int `json:"max_body_mb"`
	// QueueSize bounds the number of records waiting to be persisted.
	QueueSize int `json:"queue_size"`
}

var captureSetting = CaptureSetting{
	ChannelIds:      []int{},
	ExcludedUserIds: []int{},
	OnlyQualified:   true,
	MaxBodyMB:       32,
	QueueSize:       1000,
}

type captureFilter struct {
	channels      map[int]struct{}
	excludedUsers map[int]struct{}
	anySink       bool
}

var filter atomic.Pointer[captureFilter]

func init() {
	config.GlobalConfig.Register(ConfigName, &captureSetting)
	RebuildFilter()
}

func GetCaptureSetting() *CaptureSetting {
	return &captureSetting
}

// RebuildFilter refreshes the lookup sets used on the relay hot path.
// It must be called after any capture_setting option is updated.
func RebuildFilter() {
	next := &captureFilter{
		channels:      make(map[int]struct{}, len(captureSetting.ChannelIds)),
		excludedUsers: make(map[int]struct{}, len(captureSetting.ExcludedUserIds)),
		anySink:       captureSetting.LocalEnabled || captureSetting.S3Enabled,
	}
	for _, id := range captureSetting.ChannelIds {
		next.channels[id] = struct{}{}
	}
	for _, id := range captureSetting.ExcludedUserIds {
		next.excludedUsers[id] = struct{}{}
	}
	filter.Store(next)
}

// ShouldCapture reports whether traffic for this channel and user must be captured.
func ShouldCapture(channelId int, userId int) bool {
	f := filter.Load()
	if f == nil || !f.anySink {
		return false
	}
	if _, ok := f.channels[channelId]; !ok {
		return false
	}
	_, excluded := f.excludedUsers[userId]
	return !excluded
}

func (s *CaptureSetting) ResolvedLocalDir() string {
	if strings.TrimSpace(s.LocalDir) != "" {
		return s.LocalDir
	}
	return filepath.Join(*common.LogDir, "captures")
}

func (s *CaptureSetting) MaxBodyBytes() int {
	mb := s.MaxBodyMB
	if mb <= 0 {
		mb = 32
	}
	return mb << 20
}

// ValidateOption checks a single capture_setting.* option value before it is saved.
func ValidateOption(key string, value string) error {
	switch key {
	case "channel_ids", "excluded_user_ids":
		var ids []int
		if err := common.UnmarshalJsonStr(value, &ids); err != nil {
			return fmt.Errorf("%s must be a JSON array of integers", key)
		}
		for _, id := range ids {
			if id <= 0 {
				return fmt.Errorf("%s must contain positive integers only", key)
			}
		}
	case "max_body_mb":
		var mb int
		if err := common.UnmarshalJsonStr(value, &mb); err != nil || mb < 1 || mb > 1024 {
			return fmt.Errorf("max_body_mb must be between 1 and 1024")
		}
	case "queue_size":
		var size int
		if err := common.UnmarshalJsonStr(value, &size); err != nil || size < 1 || size > 100000 {
			return fmt.Errorf("queue_size must be between 1 and 100000")
		}
	}
	return nil
}
