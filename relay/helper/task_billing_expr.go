package helper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// PrepareTaskV2Billing evaluates a v2 task expression at submission time.
// It returns handled=false for legacy/non-v2 task pricing so callers can keep
// the existing per-call/Sora billing path unchanged.
func PrepareTaskV2Billing(c *gin.Context, info *relaycommon.RelayInfo) (priceData hosttypes.PriceData, handled bool, err error) {
	if info == nil {
		return hosttypes.PriceData{}, false, nil
	}

	// A task retry must reuse the expression and request snapshot captured by
	// the first attempt. Only the selected group's multiplier is routing
	// dependent and may change between attempts.
	if snapshot := info.TieredBillingSnapshot; snapshot != nil && snapshot.BillingMode == billing_setting.BillingModeTieredExpr && snapshot.ExprVersion == 2 {
		groupRatioInfo := HandleGroupRatio(c, info)
		if snapshot.GroupRatio != groupRatioInfo.GroupRatio {
			quota, clamp := common.QuotaRoundChecked(snapshot.EstimatedQuotaBeforeGroup * groupRatioInfo.GroupRatio)
			if clamp != nil {
				info.QuotaClamp = clamp
				return hosttypes.PriceData{}, true, clamp
			}
			snapshot.GroupRatio = groupRatioInfo.GroupRatio
			snapshot.EstimatedQuotaAfterGroup = quota
		}
		freeModel := !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume && groupRatioInfo.GroupRatio == 0
		priceData = hosttypes.PriceData{
			FreeModel:         freeModel,
			GroupRatioInfo:    groupRatioInfo,
			Quota:             snapshot.EstimatedQuotaAfterGroup,
			QuotaToPreConsume: snapshot.EstimatedQuotaAfterGroup,
			ModelPrice:        snapshot.EstimatedQuotaBeforeGroup / common.QuotaPerUnit,
			UsePrice:          true,
		}
		if freeModel {
			priceData.Quota = 0
			priceData.QuotaToPreConsume = 0
		}
		return priceData, true, nil
	}

	if billing_setting.GetBillingMode(info.OriginModelName) != billing_setting.BillingModeTieredExpr {
		return hosttypes.PriceData{}, false, nil
	}
	exprString, ok := billing_setting.GetBillingExpr(info.OriginModelName)
	if !ok || !billingexpr.IsV2VideoExpr(exprString) {
		return hosttypes.PriceData{}, false, nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	quantity, err := normalizeTaskQuantity(req)
	if err != nil {
		return hosttypes.PriceData{}, true, err
	}
	resolution := strings.TrimSpace(req.Resolution)
	requestBody, err := common.Marshal(struct {
		Resolution string `json:"resolution,omitempty"`
	}{Resolution: resolution})
	if err != nil {
		return hosttypes.PriceData{}, true, fmt.Errorf("marshal task billing request: %w", err)
	}

	groupRatioInfo := HandleGroupRatio(c, info)
	requestInput := billingexpr.RequestInput{
		Headers: info.RequestHeaders,
		Body:    requestBody,
	}
	cost, trace, err := billingexpr.RunExprWithRequest(exprString, billingexpr.TokenParams{Quantity: float64(quantity)}, requestInput)
	if err != nil {
		return hosttypes.PriceData{}, true, fmt.Errorf("model %s v2 task expression failed: %w", info.OriginModelName, err)
	}
	if trace.BillingMethod != "per_second" && trace.BillingMethod != "per_call" {
		return hosttypes.PriceData{}, true, fmt.Errorf("model %s v2 task expression must use charge per_second or per_call", info.OriginModelName)
	}
	if cost < 0 {
		return hosttypes.PriceData{}, true, fmt.Errorf("model %s v2 task expression returned a negative amount", info.OriginModelName)
	}

	quotaBeforeGroup := cost * common.QuotaPerUnit
	quota, clamp := common.QuotaRoundChecked(quotaBeforeGroup * groupRatioInfo.GroupRatio)
	if clamp != nil {
		info.QuotaClamp = clamp
		return hosttypes.PriceData{}, true, clamp
	}

	freeModel := false
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume && groupRatioInfo.GroupRatio == 0 {
		quota = 0
		freeModel = true
	}

	method := billingMethodFromTrace(exprString, trace)
	snapshot := &billingexpr.BillingSnapshot{
		BillingMode:               billing_setting.BillingModeTieredExpr,
		ModelName:                 info.OriginModelName,
		ExprString:                exprString,
		ExprHash:                  billingexpr.ExprHashString(exprString),
		GroupRatio:                groupRatioInfo.GroupRatio,
		EstimatedQuotaBeforeGroup: quotaBeforeGroup,
		EstimatedQuotaAfterGroup:  quota,
		EstimatedTier:             trace.MatchedTier,
		QuotaPerUnit:              common.QuotaPerUnit,
		ExprVersion:               2,
		BillingMethod:             method,
		Resolution:                resolution,
		Quantity:                  float64(quantity),
		TaskCount:                 1,
	}
	info.TieredBillingSnapshot = snapshot
	info.BillingRequestInput = &requestInput
	info.PriceData = hosttypes.PriceData{
		FreeModel:         freeModel,
		GroupRatioInfo:    groupRatioInfo,
		Quota:             quota,
		QuotaToPreConsume: quota,
	}
	info.PriceData.ModelPrice = cost
	info.PriceData.UsePrice = true
	return info.PriceData, true, nil
}

func normalizeTaskQuantity(req relaycommon.TaskSubmitReq) (int, error) {
	if req.Duration > 0 {
		if req.Duration > relaycommon.MaxTaskDurationSeconds {
			return 0, fmt.Errorf("duration must be between 1 and %d", relaycommon.MaxTaskDurationSeconds)
		}
		return req.Duration, nil
	}
	if strings.TrimSpace(req.Seconds) != "" {
		seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
		if err != nil || seconds <= 0 || seconds > relaycommon.MaxTaskDurationSeconds {
			return 0, fmt.Errorf("seconds must be between 1 and %d", relaycommon.MaxTaskDurationSeconds)
		}
		return seconds, nil
	}
	return 5, nil
}

// billingMethodFromTrace returns the method of the charge branch that actually
// matched during expression evaluation. This matters because a Seedance
// expression may use different billing methods in different resolution tiers.
func billingMethodFromTrace(_ string, trace billingexpr.TraceResult) string {
	return trace.BillingMethod
}
