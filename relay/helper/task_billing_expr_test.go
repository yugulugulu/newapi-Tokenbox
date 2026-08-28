package helper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const seedanceV2BillingExpr = `v2:param("resolution") == "720p" && has_media("video") ? tier("720p_video", charge("per_second", quantity, 0.31) + charge("per_second", video_input_durations, 0.12)) : param("resolution") == "720p" ? tier("720p_no_video", charge("per_second", quantity, 0.51)) : tier("fallback", charge("per_second", quantity, 0.46))`

func configureTaskV2BillingTest(t *testing.T, modes, expressions string) {
	t.Helper()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    modes,
		"billing_setting.billing_expr":    expressions,
		"group_ratio_setting.group_ratio": `{"default":1,"premium":2}`,
	}))
}

func billingSettingJSON(t *testing.T, values map[string]string) string {
	t.Helper()
	data, err := common.Marshal(values)
	require.NoError(t, err)
	return string(data)
}

func newTaskV2BillingContext(req relaycommon.TaskSubmitReq) (*gin.Context, *relaycommon.RelayInfo) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	c.Set("group", "default")
	c.Set("task_request", req)
	return c, &relaycommon.RelayInfo{
		OriginModelName: "seedance-v2-test",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
	}
}

func TestNormalizeTaskResolution(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		want    string
		wantErr bool
	}{
		{
			name: "outer resolution has priority",
			req: relaycommon.TaskSubmitReq{
				Resolution: " 720p ",
				Metadata:   map[string]interface{}{"resolution": "1080p"},
			},
			want: "720p",
		},
		{name: "metadata resolution is rejected", req: relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"resolution": "1080p"}}, wantErr: true},
		{name: "missing resolution is rejected", req: relaycommon.TaskSubmitReq{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTaskResolution(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

}

func TestTaskHasVideoInput(t *testing.T) {
	tests := []struct {
		name    string
		content []relaycommon.TaskContentItem
		want    bool
	}{
		{
			name: "valid video url",
			content: []relaycommon.TaskContentItem{{
				Type:     "video_url",
				VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/reference.mp4"},
			}},
			want: true,
		},
		{
			name:    "empty video url",
			content: []relaycommon.TaskContentItem{{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{}}},
			want:    false,
		},
		{
			name: "missing video is not video input",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := taskHasVideoInput(relaycommon.TaskSubmitReq{Content: tt.content})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := taskHasVideoInput(relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{
		"content": []interface{}{map[string]interface{}{
			"type":      "video_url",
			"video_url": map[string]interface{}{"url": "https://example.com/reference.mp4"},
		}},
	}})
	require.ErrorContains(t, err, "content is required at the top level")
}

func TestTaskVideoInputURLsCollectsTopLevelVideos(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Content: []relaycommon.TaskContentItem{
		{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: " https://example.com/one.mp4 "}},
		{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: "https://example.com/image.jpg"}},
		{VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/two.mp4"}},
		{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{}},
	}}

	urls, err := taskVideoInputURLs(req)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://example.com/one.mp4",
		"https://example.com/two.mp4",
	}, urls)
}

func TestBuildTaskV2BillingRequestBody(t *testing.T) {
	body, err := buildTaskV2BillingRequestBody("720p", true)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"resolution":"720p",
		"content":[{"type":"video_url","video_url":{"url":"present"}}]
	}`, string(body))
}

func TestNormalizeTaskQuantity(t *testing.T) {
	tests := []struct {
		name    string
		req     relaycommon.TaskSubmitReq
		want    int
		wantErr bool
	}{
		{name: "duration has priority", req: relaycommon.TaskSubmitReq{Duration: 5, Seconds: "10"}, want: 5},
		{name: "seconds without duration is rejected", req: relaycommon.TaskSubmitReq{Seconds: "10"}, wantErr: true},
		{name: "missing duration is rejected", req: relaycommon.TaskSubmitReq{}, wantErr: true},
		{name: "negative duration is invalid", req: relaycommon.TaskSubmitReq{Duration: -1}, wantErr: true},
		{name: "duration over limit is invalid", req: relaycommon.TaskSubmitReq{Duration: relaycommon.MaxTaskDurationSeconds + 1}, wantErr: true},
		{name: "seconds over limit is invalid", req: relaycommon.TaskSubmitReq{Seconds: "3601"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTaskQuantity(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrepareTaskV2BillingSkipsNonVideoExpressions(t *testing.T) {
	configureTaskV2BillingTest(t,
		`{"v1-model":"tiered_expr","unrelated-v2-model":"tiered_expr"}`,
		`{"v1-model":"tier(\"base\", p * 2)","unrelated-v2-model":"v2:tier(\"base\", p * 2)"}`,
	)

	for _, model := range []string{"v1-model", "unrelated-v2-model"} {
		c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{Duration: 5, Resolution: "720p"})
		info.OriginModelName = model

		_, handled, err := PrepareTaskV2Billing(c, info)

		require.NoError(t, err)
		assert.False(t, handled)
		assert.Nil(t, info.TieredBillingSnapshot)
	}
}

func TestPrepareTaskV2BillingUsesVideoDurationAndCapturesSnapshot(t *testing.T) {
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": seedanceV2BillingExpr}),
	)
	originalProbe := probeTaskVideoInputDurations
	t.Cleanup(func() { probeTaskVideoInputDurations = originalProbe })
	probeCalls := 0
	probeTaskVideoInputDurations = func(_ context.Context, urls []string) (float64, error) {
		probeCalls++
		assert.Equal(t, []string{"https://example.com/action.mp4"}, urls)
		return 10, nil
	}

	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{
		Duration:   5,
		Resolution: " 720p ",
		Content: []relaycommon.TaskContentItem{{
			Type:     "video_url",
			VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"},
		}},
	})

	priceData, handled, err := PrepareTaskV2Billing(c, info)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, 1, probeCalls)
	assert.Equal(t, 1_375_000, priceData.Quota)
	assert.Equal(t, 2.75, priceData.ModelPrice)
	assert.Empty(t, priceData.OtherRatios())
	require.NotNil(t, info.TieredBillingSnapshot)
	snapshot := info.TieredBillingSnapshot
	assert.Equal(t, billing_setting.BillingModeTieredExpr, snapshot.BillingMode)
	assert.Equal(t, seedanceV2BillingExpr, snapshot.ExprString)
	assert.Equal(t, billingexpr.ExprHashString(seedanceV2BillingExpr), snapshot.ExprHash)
	assert.Equal(t, 2, snapshot.ExprVersion)
	assert.Equal(t, "720p_video", snapshot.EstimatedTier)
	assert.Equal(t, "per_second", snapshot.BillingMethod)
	assert.Equal(t, "720p", snapshot.Resolution)
	assert.True(t, snapshot.HasVideoInput)
	assert.Equal(t, 5.0, snapshot.Quantity)
	assert.Equal(t, 10.0, snapshot.VideoInputDurations)
	assert.Equal(t, 1, snapshot.VideoInputCount)
	assert.Equal(t, 1, snapshot.TaskCount)
	assert.Equal(t, 2.75*common.QuotaPerUnit, snapshot.EstimatedQuotaBeforeGroup)
}

func TestPrepareTaskV2BillingOnlyProbesSelectedPerSecondVideoTier(t *testing.T) {
	perCallExpr := `v2:has_media("video") ? tier("video_call", charge("per_call", video_input_durations, 0.47)) : tier("no_video", charge("per_second", quantity, 0.51))`
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": perCallExpr}),
	)
	originalProbe := probeTaskVideoInputDurations
	t.Cleanup(func() { probeTaskVideoInputDurations = originalProbe })
	probeCalls := 0
	probeTaskVideoInputDurations = func(_ context.Context, _ []string) (float64, error) {
		probeCalls++
		return 10, nil
	}

	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{
		Duration:   5,
		Resolution: "720p",
		Content: []relaycommon.TaskContentItem{{
			Type:     "video_url",
			VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"},
		}},
	})
	priceData, handled, err := PrepareTaskV2Billing(c, info)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Zero(t, probeCalls)
	assert.Equal(t, 235_000, priceData.Quota)
	assert.Equal(t, "per_call", info.TieredBillingSnapshot.BillingMethod)
	assert.Zero(t, info.TieredBillingSnapshot.VideoInputDurations)
}

func TestPrepareTaskV2BillingNoVideoDoesNotProbe(t *testing.T) {
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": seedanceV2BillingExpr}),
	)
	originalProbe := probeTaskVideoInputDurations
	t.Cleanup(func() { probeTaskVideoInputDurations = originalProbe })
	probeTaskVideoInputDurations = func(_ context.Context, _ []string) (float64, error) {
		t.Fatal("probe must not run without a video input")
		return 0, nil
	}

	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{Duration: 5, Resolution: "720p"})
	priceData, handled, err := PrepareTaskV2Billing(c, info)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, 1_275_000, priceData.Quota)
	assert.Equal(t, "720p_no_video", info.TieredBillingSnapshot.EstimatedTier)
	assert.False(t, info.TieredBillingSnapshot.HasVideoInput)
}

func TestPrepareTaskV2BillingRetryReusesFrozenSnapshot(t *testing.T) {
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": seedanceV2BillingExpr}),
	)
	originalProbe := probeTaskVideoInputDurations
	t.Cleanup(func() { probeTaskVideoInputDurations = originalProbe })
	probeCalls := 0
	probeTaskVideoInputDurations = func(_ context.Context, _ []string) (float64, error) {
		probeCalls++
		return 10, nil
	}

	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{
		Duration:   5,
		Resolution: "720p",
		Content: []relaycommon.TaskContentItem{{
			Type:     "video_url",
			VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/action.mp4"},
		}},
	})
	firstPrice, handled, err := PrepareTaskV2Billing(c, info)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 1_375_000, firstPrice.Quota)

	changedExpr := `v2:tier("changed", charge("per_second", quantity, 99))`
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_expr": billingSettingJSON(t, map[string]string{"seedance-v2-test": changedExpr}),
	}))
	c.Set("auto_group", "premium")
	c.Set("task_request", relaycommon.TaskSubmitReq{})

	retryPrice, handled, err := PrepareTaskV2Billing(c, info)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, 1, probeCalls)
	assert.Equal(t, seedanceV2BillingExpr, info.TieredBillingSnapshot.ExprString)
	assert.Equal(t, "720p_video", info.TieredBillingSnapshot.EstimatedTier)
	assert.Equal(t, 2.0, info.TieredBillingSnapshot.GroupRatio)
	assert.Equal(t, 2_750_000, retryPrice.Quota)
}

func TestPrepareTaskV2BillingRejectsMetadataOnlyContent(t *testing.T) {
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": seedanceV2BillingExpr}),
	)
	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{
		Duration:   5,
		Resolution: "720p",
		Metadata: map[string]interface{}{
			"content": []interface{}{map[string]interface{}{
				"type":      "video_url",
				"video_url": map[string]interface{}{"url": "https://example.com/action.mp4"},
			}},
		},
	})

	_, handled, err := PrepareTaskV2Billing(c, info)

	assert.True(t, handled)
	require.ErrorContains(t, err, "content is required at the top level")
	assert.Nil(t, info.TieredBillingSnapshot)
}

func TestPrepareTaskV2BillingAllowsMoreThanThreeVideos(t *testing.T) {
	configureTaskV2BillingTest(t,
		billingSettingJSON(t, map[string]string{"seedance-v2-test": "tiered_expr"}),
		billingSettingJSON(t, map[string]string{"seedance-v2-test": seedanceV2BillingExpr}),
	)
	originalProbe := probeTaskVideoInputDurations
	t.Cleanup(func() { probeTaskVideoInputDurations = originalProbe })
	probeTaskVideoInputDurations = func(_ context.Context, urls []string) (float64, error) {
		assert.Len(t, urls, 4)
		return 4, nil
	}
	content := make([]relaycommon.TaskContentItem, 4)
	for i := range content {
		content[i] = relaycommon.TaskContentItem{VideoURL: &relaycommon.TaskMediaURL{URL: "https://example.com/video.mp4"}}
	}
	c, info := newTaskV2BillingContext(relaycommon.TaskSubmitReq{Duration: 5, Resolution: "720p", Content: content})

	priceData, handled, err := PrepareTaskV2Billing(c, info)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, 1_015_000, priceData.Quota)
	assert.Equal(t, 4, info.TieredBillingSnapshot.VideoInputCount)
	assert.Equal(t, 4.0, info.TieredBillingSnapshot.VideoInputDurations)
}
