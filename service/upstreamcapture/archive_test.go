package upstreamcapture

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// packNow is midday of 2026-11-10 in Beijing time: 2026-11-05 has long ended,
// 2026-11-10 is still in progress.
var packNow = time.Date(2026, 11, 10, 12, 0, 0, 0, captureZone)

func writeCaptureFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readPackageLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(data), "\n"), "every record must end with a newline")
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestPackEndedDaysPacksFinishedDaysOnly(t *testing.T) {
	root := t.TempDir()
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/gpt-6-sol/20261105020000_b.json"), `{"n":2}`)
	// Pretty-printed records are folded onto one line.
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/gpt-6-sol/20261105010000_a.json"), "{\n  \"n\": 1,\r\n  \"s\": \"a\\nb\"\n}\n")
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/claude-opus-5-5/20261105030000_c.json"), `{"n":3}`)
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/20261110010000_d.json"), `{"n":4}`)

	result, err := PackEndedDays(context.Background(), root, packNow, nil)
	require.NoError(t, err)
	assert.Equal(t, PackResult{Packages: 2, Records: 3}, result)

	lines := readPackageLines(t, filepath.Join(root, "packages/2026-11-05/gpt-6-sol.jsonl"))
	assert.Equal(t, []string{`{   "n": 1,    "s": "a\nb" }`, `{"n":2}`}, lines)
	for _, line := range lines {
		var v map[string]any
		require.NoError(t, common.UnmarshalJsonStr(line, &v), line)
	}
	assert.Equal(t, []string{`{"n":3}`}, readPackageLines(t, filepath.Join(root, "packages/2026-11-05/claude-opus-5-5.jsonl")))

	assert.NoDirExists(t, filepath.Join(root, "raw/2026-11-05"))
	assert.FileExists(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/20261110010000_d.json"))
	assert.NoDirExists(t, filepath.Join(root, "packages/2026-11-10"))
}

func TestPackEndedDaysAppendsLateRecordsAndInvalidatesUpload(t *testing.T) {
	root := t.TempDir()
	pkgPath := filepath.Join(root, "packages/2026-11-05/gpt-6-sol.jsonl")
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/gpt-6-sol/20261105010000_a.json"), `{"n":1}`)
	_, err := PackEndedDays(context.Background(), root, packNow, nil)
	require.NoError(t, err)

	info, err := os.Stat(pkgPath)
	require.NoError(t, err)
	upload := &model.CaptureUploadRecord{CaptureDate: "2026-11-05", ModelName: "gpt-6-sol", ZipSize: info.Size(), ZipMtime: info.ModTime().UnixNano(), FinishedAt: 1}
	stats, err := CollectLocalStats(root, []*model.CaptureUploadRecord{upload})
	require.NoError(t, err)
	require.Len(t, stats.Items, 1)
	assert.True(t, stats.Items[0].Uploaded)

	// A long request that finished after the day was packed.
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/gpt-6-sol/20261105235900_late.json"), `{"n":9}`)
	result, err := PackEndedDays(context.Background(), root, packNow, nil)
	require.NoError(t, err)
	assert.Equal(t, PackResult{Packages: 1, Records: 1}, result)

	assert.Equal(t, []string{`{"n":1}`, `{"n":9}`}, readPackageLines(t, pkgPath))

	stats, err = CollectLocalStats(root, []*model.CaptureUploadRecord{upload})
	require.NoError(t, err)
	require.Len(t, stats.Items, 1)
	assert.False(t, stats.Items[0].Uploaded, "a package with appended records must be uploaded again")
	assert.Equal(t, 2, stats.Items[0].PackageFiles)
	assert.Equal(t, stats.PackageBytes, stats.PendingUploadBytes)
}

func TestCollectLocalStats(t *testing.T) {
	root := t.TempDir()
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/a.json"), "12345")
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/b.json"), "123")
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/c.json.tmp"), "ignored")
	writeCaptureFile(t, filepath.Join(root, "packages/2026-11-05/gpt-6-sol.jsonl"), "{}\n{}\n{}\n{}\n")
	writeCaptureFile(t, filepath.Join(root, "packages/2026-11-05/gpt-6-sol.jsonl.count"), "4")
	// Legacy zip package from before the JSONL format.
	writeCaptureFile(t, filepath.Join(root, "packages/2026-11-05/claude-opus-5-5.zip"), "zip-2")
	writeCaptureFile(t, filepath.Join(root, "2026-11-06/channel_44/legacy.json"), "legacy!")

	uploadedInfo, err := os.Stat(filepath.Join(root, "packages/2026-11-05/claude-opus-5-5.zip"))
	require.NoError(t, err)
	uploads := []*model.CaptureUploadRecord{
		{CaptureDate: "2026-11-05", ModelName: "claude-opus-5-5", ZipSize: uploadedInfo.Size(), ZipMtime: uploadedInfo.ModTime().UnixNano(), FinishedAt: 100},
		// Same package name but a different size: an older version of the package.
		{CaptureDate: "2026-11-05", ModelName: "gpt-6-sol", ZipSize: 1, ZipMtime: 1, FinishedAt: 50},
	}

	stats, err := CollectLocalStats(root, uploads)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.RawFiles)
	assert.Equal(t, int64(8), stats.RawBytes)
	assert.Equal(t, 2, stats.PackageCount)
	assert.Equal(t, int64(12+5), stats.PackageBytes)
	assert.Equal(t, 1, stats.UploadedCount)
	assert.Equal(t, int64(5), stats.UploadedBytes)
	assert.Equal(t, int64(8+12), stats.PendingUploadBytes)
	assert.Equal(t, 1, stats.LegacyFiles)
	assert.Equal(t, int64(7), stats.LegacyBytes)
	assert.Equal(t, int64(8+17+7), stats.TotalBytes)

	require.Len(t, stats.Items, 3)
	assert.Equal(t, LocalStatsItem{Day: "2026-11-10", Model: "gpt-6-sol", RawFiles: 2, RawBytes: 8}, stats.Items[0])
	assert.Equal(t, "claude-opus-5-5", stats.Items[1].Model)
	assert.True(t, stats.Items[1].Uploaded)
	assert.Equal(t, int64(100), stats.Items[1].UploadedAt)
	assert.Equal(t, "gpt-6-sol", stats.Items[2].Model)
	assert.False(t, stats.Items[2].Uploaded)
	assert.Equal(t, 4, stats.Items[2].PackageFiles)
}

func TestDeleteAllLocalKeepsUnrelatedFiles(t *testing.T) {
	root := t.TempDir()
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-10/gpt-6-sol/a.json"), "x")
	writeCaptureFile(t, filepath.Join(root, "packages/2026-11-05/gpt-6-sol.jsonl"), "x")
	writeCaptureFile(t, filepath.Join(root, "2026-11-06/channel_44/legacy.json"), "x")
	writeCaptureFile(t, filepath.Join(root, "notes.txt"), "keep")
	writeCaptureFile(t, filepath.Join(root, "other/data.json"), "keep")

	require.NoError(t, DeleteAllLocal(root))

	assert.NoDirExists(t, filepath.Join(root, "raw"))
	assert.NoDirExists(t, filepath.Join(root, "packages"))
	assert.NoDirExists(t, filepath.Join(root, "2026-11-06"))
	assert.FileExists(t, filepath.Join(root, "notes.txt"))
	assert.FileExists(t, filepath.Join(root, "other/data.json"))

	stats, err := CollectLocalStats(root, nil)
	require.NoError(t, err)
	assert.Zero(t, stats.TotalBytes)
	assert.Empty(t, stats.Items)
}

func TestCollectLocalStatsLegacyZipWithLateJSONL(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "packages/2026-11-17/gpt-6-sol.zip")
	writeCaptureFile(t, zipPath, "zip")
	zipInfo, err := os.Stat(zipPath)
	require.NoError(t, err)
	zipUpload := &model.CaptureUploadRecord{CaptureDate: "2026-11-17", ModelName: "gpt-6-sol", ZipSize: zipInfo.Size(), ZipMtime: zipInfo.ModTime().UnixNano(), FinishedAt: 10}

	// Records that finished after the zip was uploaded land in a JSONL package.
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-17/gpt-6-sol/late.json"), `{"n":1}`)
	_, err = PackEndedDays(context.Background(), root, time.Date(2026, 11, 18, 1, 0, 0, 0, captureZone), nil)
	require.NoError(t, err)

	stats, err := CollectLocalStats(root, []*model.CaptureUploadRecord{zipUpload})
	require.NoError(t, err)
	require.Len(t, stats.Items, 1)
	assert.Equal(t, 2, stats.PackageCount)
	assert.False(t, stats.Items[0].Uploaded, "the late JSONL package is still pending")
	assert.Equal(t, 1, stats.UploadedCount)
	assert.Equal(t, int64(len(`{"n":1}`)+1), stats.PendingUploadBytes)
}
