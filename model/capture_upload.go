package model

const (
	CaptureUploadStatusSuccess = "success"
	CaptureUploadStatusFailed  = "failed"

	SystemTaskTypeCapturePack   = "capture_pack"
	SystemTaskTypeCaptureUpload = "capture_upload"
)

// CaptureUploadRecord is one attempt to upload a <date>/<model>.zip capture
// package to S3. Records outlive the local files, so they double as the audit
// trail of what has been delivered.
type CaptureUploadRecord struct {
	Id          int    `json:"id"`
	TaskId      string `json:"task_id" gorm:"type:varchar(64);index"`
	CaptureDate string `json:"capture_date" gorm:"type:varchar(10);index:idx_capture_upload_date_model"`
	ModelName   string `json:"model_name" gorm:"type:varchar(255);index:idx_capture_upload_date_model"`
	S3Bucket    string `json:"s3_bucket" gorm:"type:varchar(255)"`
	S3Key       string `json:"s3_key" gorm:"type:varchar(1024)"`
	FileCount   int    `json:"file_count"`
	ZipSize     int64  `json:"zip_size" gorm:"bigint"`
	// ZipMtime is the package's modification time (unix nanoseconds) when it
	// was uploaded; together with ZipSize it identifies the exact package.
	ZipMtime   int64  `json:"zip_mtime" gorm:"bigint"`
	Status     string `json:"status" gorm:"type:varchar(16);index"`
	Error      string `json:"error" gorm:"type:text"`
	StartedAt  int64  `json:"started_at" gorm:"bigint;index"`
	FinishedAt int64  `json:"finished_at" gorm:"bigint"`
	DurationMs int64  `json:"duration_ms" gorm:"bigint"`
}

func CreateCaptureUploadRecord(record *CaptureUploadRecord) error {
	return DB.Create(record).Error
}

// ListCaptureUploadRecords returns upload attempts, newest first.
func ListCaptureUploadRecords(offset int, limit int) ([]*CaptureUploadRecord, int64, error) {
	var total int64
	if err := DB.Model(&CaptureUploadRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []*CaptureUploadRecord
	err := DB.Order("id desc").Offset(offset).Limit(limit).Find(&records).Error
	return records, total, err
}

// ListSuccessfulCaptureUploads returns every successful upload; the stats view
// matches them against local packages by date, model, size and mtime.
func ListSuccessfulCaptureUploads() ([]*CaptureUploadRecord, error) {
	var records []*CaptureUploadRecord
	err := DB.Select("capture_date", "model_name", "zip_size", "zip_mtime", "finished_at").
		Where("status = ?", CaptureUploadStatusSuccess).
		Find(&records).Error
	return records, err
}
