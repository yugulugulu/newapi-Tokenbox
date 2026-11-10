package upstreamcapture

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecordDirUsesBeijingWriteDayAndModel(t *testing.T) {
	// The request started before Beijing midnight; only the write time decides the day.
	rec := &builtRecord{startedAt: time.Date(2026, 11, 9, 15, 58, 0, 0, time.UTC), model: "gpt-6-sol"}
	tests := []struct {
		name      string
		writtenAt time.Time
		want      string
	}{
		{name: "written before Beijing midnight", writtenAt: time.Date(2026, 11, 9, 15, 59, 59, 0, time.UTC), want: "raw/2026-11-09/gpt-6-sol"},
		{name: "spans Beijing midnight", writtenAt: time.Date(2026, 11, 9, 16, 20, 0, 0, time.UTC), want: "raw/2026-11-10/gpt-6-sol"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, filepath.FromSlash(tt.want), recordDir(rec, tt.writtenAt))
		})
	}
}

func TestIsPackableDate(t *testing.T) {
	tests := []struct {
		name string
		day  string
		now  time.Time
		want bool
	}{
		{name: "last second of the day", day: "2026-11-09", now: time.Date(2026, 11, 9, 23, 59, 59, 0, captureZone), want: false},
		{name: "exactly Beijing midnight", day: "2026-11-09", now: time.Date(2026, 11, 10, 0, 0, 0, 0, captureZone), want: true},
		{name: "midnight expressed in UTC", day: "2026-11-09", now: time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC), want: true},
		{name: "today", day: "2026-11-10", now: time.Date(2026, 11, 10, 12, 0, 0, 0, captureZone), want: false},
		{name: "invalid day", day: "channel_44", now: time.Date(2026, 11, 10, 12, 0, 0, 0, captureZone), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPackableDate(tt.day, tt.now))
		})
	}
}

func TestModelDirName(t *testing.T) {
	tests := map[string]string{
		"gpt-6.1-sol":                 "gpt-6.1-sol",
		"claude-opus-5-5":             "claude-opus-5-5",
		"openai/gpt-6":                "openai_gpt-6",
		"../../etc":                   "_.._etc",
		" model with space ":          "model_with_space",
		"":                            "unknown",
		"..":                          "unknown",
		"us.anthropic.claude:v1@2026": "us.anthropic.claude_v1_2026",
	}
	for in, want := range tests {
		assert.Equal(t, want, modelDirName(in), in)
	}
}

func TestPackageObjectKey(t *testing.T) {
	assert.Equal(t, "2026-11-10/gpt-6-sol.zip", PackageObjectKey("", "2026-11-10", "gpt-6-sol"))
	assert.Equal(t, "captures/2026-11-10/gpt-6-sol.zip", PackageObjectKey("/captures/", "2026-11-10", "gpt-6-sol"))
}
