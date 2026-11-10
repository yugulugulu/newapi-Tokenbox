package upstreamcapture

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"context"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/capture_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useCaptureUploadTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CaptureUploadRecord{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		_ = sqlDB.Close()
	})
}

// rejectingS3 stores PUT bodies and answers 403 for keys containing reject.
type rejectingS3 struct {
	mu      sync.Mutex
	reject  string
	objects map[string][]byte
}

func (f *rejectingS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reject != "" && strings.Contains(r.URL.Path, f.reject) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
		return
	}
	if r.Method == http.MethodPut {
		f.objects[r.URL.Path] = body
	}
	w.WriteHeader(http.StatusOK)
}

func TestUploadPendingRecordsEveryAttempt(t *testing.T) {
	useCaptureUploadTestDB(t)
	fake := &rejectingS3{reject: "claude", objects: map[string][]byte{}}
	server := httptest.NewServer(fake)
	defer server.Close()

	root := t.TempDir()
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/gpt-6-sol/a.json"), `{"n":1}`)
	writeCaptureFile(t, filepath.Join(root, "raw/2026-11-05/claude-opus-5-5/b.json"), `{"n":2}`)
	_, err := PackEndedDays(context.Background(), root, packNow, nil)
	require.NoError(t, err)

	setting := &capture_setting.CaptureSetting{
		LocalDir:      root,
		S3Bucket:      "bucket",
		S3Region:      "ap-southeast-1",
		S3Endpoint:    server.URL,
		S3Prefix:      "cap",
		S3AccessKeyId: "AKIDTEST",
		S3Secret:      "secret",
	}
	result, err := UploadPending(context.Background(), setting, "task-1", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)

	zipBytes, err := os.ReadFile(filepath.Join(root, "packages/2026-11-05/gpt-6-sol.zip"))
	require.NoError(t, err)
	fake.mu.Lock()
	assert.Equal(t, zipBytes, fake.objects["/bucket/cap/2026-11-05/gpt-6-sol.zip"])
	fake.mu.Unlock()

	records, total, err := model.ListCaptureUploadRecords("", 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	byModel := map[string]*model.CaptureUploadRecord{}
	for _, r := range records {
		byModel[r.ModelName] = r
	}
	ok := byModel["gpt-6-sol"]
	require.NotNil(t, ok)
	assert.Equal(t, model.CaptureUploadStatusSuccess, ok.Status)
	assert.Equal(t, "task-1", ok.TaskId)
	assert.Equal(t, "cap/2026-11-05/gpt-6-sol.zip", ok.S3Key)
	assert.Equal(t, 1, ok.FileCount)
	assert.Equal(t, int64(len(zipBytes)), ok.ZipSize)
	failed := byModel["claude-opus-5-5"]
	require.NotNil(t, failed)
	assert.Equal(t, model.CaptureUploadStatusFailed, failed.Status)
	assert.Contains(t, failed.Error, "AccessDenied")

	_, dayTotal, err := model.ListCaptureUploadRecords("2026-11-05", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), dayTotal)
	_, otherDayTotal, err := model.ListCaptureUploadRecords("2026-11-06", 0, 10)
	require.NoError(t, err)
	assert.Zero(t, otherDayTotal)

	// The next run retries only the package that has no successful upload.
	fake.mu.Lock()
	fake.reject = ""
	fake.mu.Unlock()
	result, err = UploadPending(context.Background(), setting, "task-2", nil)
	require.NoError(t, err)
	assert.Equal(t, UploadResult{Succeeded: 1, Bytes: failed.ZipSize}, result)

	successes, err := model.ListSuccessfulCaptureUploads()
	require.NoError(t, err)
	stats, err := CollectLocalStats(root, successes)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.UploadedCount)
	assert.Zero(t, stats.PendingUploadBytes)
}

func TestUploadPendingRequiresBucketAndRegion(t *testing.T) {
	_, err := UploadPending(context.Background(), &capture_setting.CaptureSetting{LocalDir: t.TempDir()}, "task", nil)
	require.Error(t, err)
}
