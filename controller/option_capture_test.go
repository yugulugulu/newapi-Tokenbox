package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The S3 secret key must never be returned by GET /api/option; the access key id may be.
func TestGetOptionsHidesCaptureS3Secret(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap["capture_setting.s3_secret"] = "super-secret"
	common.OptionMap["capture_setting.s3_access_key_id"] = "AKIDEXAMPLE"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		delete(common.OptionMap, "capture_setting.s3_secret")
		delete(common.OptionMap, "capture_setting.s3_access_key_id")
		common.OptionMapRWMutex.Unlock()
	})

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(context)

	var payload struct {
		Data []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	keys := map[string]string{}
	for _, option := range payload.Data {
		keys[option.Key] = option.Value
	}
	assert.NotContains(t, keys, "capture_setting.s3_secret")
	assert.Equal(t, "AKIDEXAMPLE", keys["capture_setting.s3_access_key_id"])
}
