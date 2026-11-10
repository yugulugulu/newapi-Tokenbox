package upstreamcapture

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/capture_setting"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
)

const (
	uploadPartSize    = 64 << 20
	uploadConcurrency = 4
	maxUploadErrorLen = 1000
)

type UploadResult struct {
	Succeeded int   `json:"succeeded"`
	Failed    int   `json:"failed"`
	Bytes     int64 `json:"bytes"`
}

// PackageObjectKey is the S3 key of a package: <prefix>/<day>/<model>.zip.
func PackageObjectKey(prefix, day, modelName string) string {
	key := day + "/" + modelName + zipExt
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return key
	}
	return prefix + "/" + key
}

// UploadPending uploads every local package that has no successful upload
// matching its current size and mtime. Each attempt is recorded; one failed
// package does not stop the others.
func UploadPending(ctx context.Context, setting *capture_setting.CaptureSetting, taskId string, progress func(done, total int)) (UploadResult, error) {
	var result UploadResult
	if strings.TrimSpace(setting.S3Bucket) == "" || strings.TrimSpace(setting.S3Region) == "" {
		return result, errors.New("s3 bucket and region are required")
	}
	uploads, err := model.ListSuccessfulCaptureUploads()
	if err != nil {
		return result, err
	}
	uploaded := make(map[string]struct{}, len(uploads))
	for _, u := range uploads {
		uploaded[uploadRecordKey(u)] = struct{}{}
	}
	pkgs, err := listPackages(setting.ResolvedLocalDir())
	if err != nil {
		return result, err
	}
	var pending []localPackage
	for _, p := range pkgs {
		if _, ok := uploaded[p.uploadKey()]; !ok {
			pending = append(pending, p)
		}
	}

	uploader := transfermanager.New(s3ClientFor(setting), func(o *transfermanager.Options) {
		o.PartSizeBytes = uploadPartSize
		o.Concurrency = uploadConcurrency
	})
	for i, p := range pending {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		record := uploadPackage(ctx, uploader, setting, p)
		record.TaskId = taskId
		if err := model.CreateCaptureUploadRecord(record); err != nil {
			return result, err
		}
		if record.Status == model.CaptureUploadStatusSuccess {
			result.Succeeded++
			result.Bytes += record.ZipSize
		} else {
			result.Failed++
		}
		if progress != nil {
			progress(i+1, len(pending))
		}
	}
	return result, nil
}

func uploadPackage(ctx context.Context, uploader *transfermanager.Client, setting *capture_setting.CaptureSetting, p localPackage) *model.CaptureUploadRecord {
	started := time.Now()
	record := &model.CaptureUploadRecord{
		CaptureDate: p.day,
		ModelName:   p.model,
		S3Bucket:    setting.S3Bucket,
		S3Key:       PackageObjectKey(setting.S3Prefix, p.day, p.model),
		FileCount:   countPackageEntries(p.path),
		ZipSize:     p.size,
		ZipMtime:    p.mtime,
		StartedAt:   started.Unix(),
	}
	err := func() error {
		f, err := os.Open(p.path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Bucket:      aws.String(setting.S3Bucket),
			Key:         aws.String(record.S3Key),
			Body:        f,
			ContentType: aws.String("application/zip"),
		})
		return err
	}()
	finished := time.Now()
	record.FinishedAt = finished.Unix()
	record.DurationMs = finished.Sub(started).Milliseconds()
	if err != nil {
		record.Status = model.CaptureUploadStatusFailed
		record.Error = truncateUploadError(err.Error())
	} else {
		record.Status = model.CaptureUploadStatusSuccess
	}
	return record
}

func truncateUploadError(msg string) string {
	if len(msg) <= maxUploadErrorLen {
		return msg
	}
	msg = msg[:maxUploadErrorLen]
	for !utf8.ValidString(msg) {
		msg = msg[:len(msg)-1]
	}
	return msg
}
