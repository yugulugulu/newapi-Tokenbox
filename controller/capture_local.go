package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/upstreamcapture"
	"github.com/QuantumNous/new-api/setting/capture_setting"

	"github.com/gin-gonic/gin"
)

// capturePackHandler packs finished capture days into <day>/<model>.zip.
type capturePackHandler struct{}

func (capturePackHandler) Type() string { return model.SystemTaskTypeCapturePack }

func (capturePackHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	root := capture_setting.GetCaptureSetting().ResolvedLocalDir()
	result, err := upstreamcapture.PackEndedDays(ctx, root, time.Now(), service.NewSystemTaskProgressReporter(task, runnerID))
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, result, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
}

// captureUploadHandler uploads packages that have no matching successful upload.
type captureUploadHandler struct{}

func (captureUploadHandler) Type() string { return model.SystemTaskTypeCaptureUpload }

func (captureUploadHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	setting := capture_setting.GetCaptureSetting()
	result, err := upstreamcapture.UploadPending(ctx, setting, task.TaskID, service.NewSystemTaskProgressReporter(task, runnerID))
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, result, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
}

func GetCaptureLocalStats(c *gin.Context) {
	uploads, err := model.ListSuccessfulCaptureUploads()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stats, err := upstreamcapture.CollectLocalStats(capture_setting.GetCaptureSetting().ResolvedLocalDir(), uploads)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func CreateCapturePackTask(c *gin.Context) {
	enqueueCaptureTask(c, model.SystemTaskTypeCapturePack, model.SystemTaskTypeCaptureUpload)
}

func CreateCaptureUploadTask(c *gin.Context) {
	setting := capture_setting.GetCaptureSetting()
	if setting.S3Bucket == "" || setting.S3Region == "" {
		common.ApiErrorMsg(c, "S3 bucket and region must be configured before uploading")
		return
	}
	enqueueCaptureTask(c, model.SystemTaskTypeCaptureUpload, model.SystemTaskTypeCapturePack)
}

// enqueueCaptureTask starts taskType unless conflictType is active: packing
// rewrites zips that an upload may be reading, and vice versa.
func enqueueCaptureTask(c *gin.Context, taskType string, conflictType string) {
	conflict, err := model.GetActiveSystemTask(conflictType)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if conflict != nil {
		common.ApiErrorMsg(c, "another capture pack or upload task is running")
		return
	}
	task, _, err := service.EnqueueSystemTask(taskType, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, task.ToResponse())
}

func DeleteCaptureLocalData(c *gin.Context) {
	for _, taskType := range []string{model.SystemTaskTypeCapturePack, model.SystemTaskTypeCaptureUpload} {
		active, err := model.GetActiveSystemTask(taskType)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if active != nil {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "wait for the running capture pack or upload task to finish"})
			return
		}
	}
	if err := upstreamcapture.DeleteAllLocal(capture_setting.GetCaptureSetting().ResolvedLocalDir()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListCaptureUploads(c *gin.Context) {
	captureDate := c.Query("date")
	if captureDate != "" {
		if _, err := time.Parse("2006-01-02", captureDate); err != nil {
			common.ApiErrorMsg(c, "date must be in YYYY-MM-DD format")
			return
		}
	}
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListCaptureUploadRecords(captureDate, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}
