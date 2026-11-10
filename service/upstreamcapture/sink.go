package upstreamcapture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/capture_setting"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const workerCount = 4

var (
	jobs         chan *job
	workerOnce   sync.Once
	droppedCount atomic.Int64
)

// startWorkers creates the bounded queue on first use. The queue size is read
// once, so changing queue_size takes effect after a restart.
func startWorkers() {
	workerOnce.Do(func() {
		size := capture_setting.GetCaptureSetting().QueueSize
		if size <= 0 {
			size = 1000
		}
		jobs = make(chan *job, size)
		for i := 0; i < workerCount; i++ {
			go runWorker()
		}
	})
}

// enqueue hands a finished capture to the background writers. It is called
// from the relay goroutine when the upstream response body is closed, so it
// must never block the relay for long.
func enqueue(j *job) {
	startWorkers()
	select {
	case jobs <- j:
	default:
		// Queue full: drop the new record rather than delay the relay.
		// Log the first drop and then every 100th to avoid flooding logs.
		if n := droppedCount.Add(1); n%100 == 1 {
			common.SysError(fmt.Sprintf("upstream capture: queue full, dropped %d record(s) so far; consider raising queue_size", n))
		}
	}
}

func runWorker() {
	for j := range jobs {
		persist(j)
	}
}

func persist(j *job) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("upstream capture: panic while persisting record: %v", r))
		}
	}()
	setting := capture_setting.GetCaptureSetting()
	if !setting.LocalEnabled {
		return
	}
	rec := buildRecord(j)
	if setting.OnlyQualified && !rec.qualified {
		return
	}
	if err := writeLocal(setting, rec); err != nil {
		common.SysError("upstream capture: local write failed: " + err.Error())
	}
}

// recordDir groups raw records by Beijing-time day and upstream model, which is
// exactly the unit the pack task turns into one <date>/<model>.zip. The day is
// taken from writtenAt rather than the request start, so a request that spans
// midnight belongs to the next day and a finished day never receives new files.
func recordDir(rec *builtRecord, writtenAt time.Time) string {
	return filepath.Join(rawDirName, captureDay(writtenAt), rec.model)
}

// writeLocal writes to a temp file first and renames it, so readers never see partial records.
func writeLocal(setting *capture_setting.CaptureSetting, rec *builtRecord) error {
	dir := filepath.Join(setting.ResolvedLocalDir(), recordDir(rec, time.Now()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := rec.requestId + ".json"
	if rec.newAPIRequestId != "" {
		name = rec.newAPIRequestId + "_" + name
	}
	final := filepath.Join(dir, name)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, rec.body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

type s3ClientKey struct {
	region, endpoint, accessKeyId, secret string
}

var (
	s3ClientMu     sync.Mutex
	s3ClientCached *s3.Client
	s3ClientCfg    s3ClientKey
)

// s3ClientFor reuses one client until the connection settings change.
func s3ClientFor(setting *capture_setting.CaptureSetting) *s3.Client {
	key := s3ClientKey{
		region:      setting.S3Region,
		endpoint:    strings.TrimSpace(setting.S3Endpoint),
		accessKeyId: setting.S3AccessKeyId,
		secret:      setting.S3Secret,
	}
	s3ClientMu.Lock()
	defer s3ClientMu.Unlock()
	if s3ClientCached != nil && s3ClientCfg == key {
		return s3ClientCached
	}
	opts := s3.Options{
		Region:      key.region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(key.accessKeyId, key.secret, "")),
	}
	if key.endpoint != "" {
		opts.BaseEndpoint = aws.String(key.endpoint)
		// S3-compatible services (MinIO, R2, local fakes) generally need path-style
		// addressing; AWS endpoints keep virtual-hosted style.
		opts.UsePathStyle = !strings.Contains(key.endpoint, "amazonaws.com")
	}
	s3ClientCached = s3.New(opts)
	s3ClientCfg = key
	return s3ClientCached
}
