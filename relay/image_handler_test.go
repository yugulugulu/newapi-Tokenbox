package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateImageResolutionLimit(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		size    string
		limit   dto.ImageResolutionLimit
		wantErr string
	}{
		{name: "1k square", model: "gpt-image-2", size: "1024x1024", limit: dto.ImageResolutionLimit1K},
		{name: "1k portrait", model: "gpt-image-2", size: "512x1024", limit: dto.ImageResolutionLimit1K},
		{name: "1k width exceeded", model: "gpt-image-2", size: "1025x1024", limit: dto.ImageResolutionLimit1K, wantErr: "exceeds channel resolution limit 1k"},
		{name: "1k height exceeded", model: "gpt-image-2", size: "1024x1025", limit: dto.ImageResolutionLimit1K, wantErr: "exceeds channel resolution limit 1k"},
		{name: "2k square", model: "gpt-image-2-edit", size: "2048x2048", limit: dto.ImageResolutionLimit2K},
		{name: "2k exceeded", model: "gpt-image-2-edit", size: "2048x2049", limit: dto.ImageResolutionLimit2K, wantErr: "exceeds channel resolution limit 2k"},
		{name: "4k square", model: "gpt-image-2", size: "4096x4096", limit: dto.ImageResolutionLimit4K},
		{name: "4k exceeded", model: "gpt-image-2", size: "4097x4096", limit: dto.ImageResolutionLimit4K, wantErr: "exceeds channel resolution limit 4k"},
		{name: "empty size defaults to 1k", model: "gpt-image-2", size: "", limit: dto.ImageResolutionLimit1K},
		{name: "outer whitespace is allowed", model: "gpt-image-2", size: " 1024x1024 ", limit: dto.ImageResolutionLimit1K},
		{name: "missing height", model: "gpt-image-2", size: "1024", limit: dto.ImageResolutionLimit1K, wantErr: "expected widthxheight"},
		{name: "uppercase separator", model: "gpt-image-2", size: "1024X1024", limit: dto.ImageResolutionLimit1K, wantErr: "expected widthxheight"},
		{name: "invalid width", model: "gpt-image-2", size: "abcx1024", limit: dto.ImageResolutionLimit1K, wantErr: "width must be a positive integer"},
		{name: "signed width", model: "gpt-image-2", size: "+1024x1024", limit: dto.ImageResolutionLimit1K, wantErr: "width must be a positive integer"},
		{name: "missing width", model: "gpt-image-2", size: "x1024", limit: dto.ImageResolutionLimit1K, wantErr: "width must be a positive integer"},
		{name: "missing height value", model: "gpt-image-2", size: "1024x", limit: dto.ImageResolutionLimit1K, wantErr: "height must be a positive integer"},
		{name: "zero width", model: "gpt-image-2", size: "0x1024", limit: dto.ImageResolutionLimit1K, wantErr: "width must be a positive integer"},
		{name: "negative height", model: "gpt-image-2", size: "1024x-1", limit: dto.ImageResolutionLimit1K, wantErr: "height must be a positive integer"},
		{name: "multiple separators", model: "gpt-image-2", size: "1024x1024x1", limit: dto.ImageResolutionLimit1K, wantErr: "expected widthxheight"},
		{name: "unlimited ignores size", model: "gpt-image-2", size: "invalid", limit: dto.ImageResolutionLimitUnlimited},
		{name: "empty setting is unlimited", model: "gpt-image-2", size: "invalid", limit: ""},
		{name: "other image models are unchanged", model: "gpt-image-1", size: "invalid", limit: dto.ImageResolutionLimit1K},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageResolutionLimit(tt.model, tt.size, tt.limit)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestImageHelperResolutionLimitErrorSkipsRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		ImageResolutionLimit: dto.ImageResolutionLimit1K,
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		Request: &dto.ImageRequest{
			Model: "gpt-image-2",
			Size:  "2048x2048",
		},
	}

	err := ImageHelper(c, info)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.True(t, types.IsSkipRetryError(err))
	assert.False(t, types.IsChannelError(err))
}
