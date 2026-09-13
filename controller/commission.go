package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type commissionSettingsRequest struct {
	Enabled           *bool    `json:"enabled"`
	GlobalRateEnabled *bool    `json:"global_rate_enabled"`
	GlobalRatePercent *float64 `json:"global_rate_percent"`
}

type commissionAgentRequest struct {
	UseCustomRate bool    `json:"use_custom_rate"`
	RatePercent   float64 `json:"rate_percent"`
}

func commissionRateToBasisPoints(percent float64) int {
	return int(percent*100 + 0.5)
}

func commissionBasisPointsToPercent(basisPoints int) float64 {
	return float64(basisPoints) / 100
}

func GetCommissionSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"enabled":             common.CommissionEnabled,
		"global_rate_enabled": common.CommissionGlobalRateEnabled,
		"global_rate_percent": commissionBasisPointsToPercent(common.CommissionGlobalRateBasisPoints),
	}})
}

func UpdateCommissionSettings(c *gin.Context) {
	var req commissionSettingsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	values := make(map[string]string)
	if req.Enabled != nil {
		values["CommissionEnabled"] = strconv.FormatBool(*req.Enabled)
	}
	if req.GlobalRateEnabled != nil {
		values["CommissionGlobalRateEnabled"] = strconv.FormatBool(*req.GlobalRateEnabled)
	}
	if req.GlobalRatePercent != nil {
		if math.IsNaN(*req.GlobalRatePercent) || math.IsInf(*req.GlobalRatePercent, 0) {
			common.ApiErrorMsg(c, "全局反佣比例必须是有效数字")
			return
		}
		if *req.GlobalRatePercent < 0 || *req.GlobalRatePercent > float64(model.CommissionRateMaxBasisPoints)/100 {
			common.ApiErrorMsg(c, "全局反佣比例必须在 0% 到 100% 之间")
			return
		}
		basisPoints := commissionRateToBasisPoints(*req.GlobalRatePercent)
		if basisPoints > model.CommissionRateMaxBasisPoints {
			common.ApiErrorMsg(c, "全局反佣比例必须在 0% 到 100% 之间")
			return
		}
		values["CommissionGlobalRateBasisPoints"] = strconv.Itoa(basisPoints)
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	GetCommissionSettings(c)
}

func ListCommissionAgents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agents, total, err := model.ListCommissionAgents(c.Query("keyword"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(agents)
	common.ApiSuccess(c, pageInfo)
}

func UpdateCommissionAgent(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("userId"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 UID")
		return
	}
	var req commissionAgentRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if math.IsNaN(req.RatePercent) || math.IsInf(req.RatePercent, 0) || req.RatePercent < 0 || req.RatePercent > float64(model.CommissionRateMaxBasisPoints)/100 {
		common.ApiErrorMsg(c, "代理反佣比例必须在 0% 到 100% 之间")
		return
	}
	basisPoints := commissionRateToBasisPoints(req.RatePercent)
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return model.SetCommissionAgent(tx, userId, req.UseCustomRate, basisPoints)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func commissionTimeRange(c *gin.Context) (int64, int64, error) {
	startTime, err := strconv.ParseInt(strings.TrimSpace(c.Query("start_time")), 10, 64)
	if c.Query("start_time") == "" {
		startTime = 0
		err = nil
	}
	if err != nil {
		return 0, 0, err
	}
	if startTime > 1_000_000_000_000 {
		startTime /= 1000
	}
	endTime, err := strconv.ParseInt(strings.TrimSpace(c.Query("end_time")), 10, 64)
	if c.Query("end_time") == "" {
		endTime = 0
		err = nil
	}
	if err != nil {
		return 0, 0, err
	}
	if endTime > 1_000_000_000_000 {
		endTime /= 1000
	}
	if startTime > 0 && endTime == 0 {
		endTime = common.GetTimestamp()
	}
	if startTime > 0 && endTime > 0 && endTime <= startTime {
		return 0, 0, errors.New("结束时间必须晚于开始时间")
	}
	return startTime, endTime, err
}

func GetCommissionSummary(c *gin.Context) {
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	summaries, err := model.SummarizeCommissionRecords(c.Query("keyword"), startTime, endTime, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": summaries})
}

func GetCommissionRecords(c *gin.Context) {
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	pageInfo := common.GetPageQuery(c)
	agentUserId, _ := strconv.Atoi(c.Query("agent_user_id"))
	records, total, err := model.ListCommissionRecords(c.Query("keyword"), startTime, endTime, agentUserId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

func GetCommissionAgentReferrals(c *gin.Context) {
	agentUserId, err := strconv.Atoi(c.Param("userId"))
	if err != nil || agentUserId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 UID")
		return
	}
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	pageInfo := common.GetPageQuery(c)
	users, total, err := model.ListCommissionReferrals(agentUserId, c.Query("keyword"), startTime, endTime, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func requireCommissionSelf(c *gin.Context) (*model.User, *model.CommissionAgent, bool) {
	user, agent, eligible := model.GetCommissionEligibility(c.GetInt("id"))
	if !eligible {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "当前用户无反佣资格"})
		return user, agent, false
	}
	return user, agent, true
}

func GetCommissionSelf(c *gin.Context) {
	user, ok := requireCommissionReadableSelf(c)
	if !ok {
		return
	}
	_, agent, eligible := model.GetCommissionEligibility(user.Id)
	rate := 0
	source := "none"
	useCustomRate := false
	if eligible {
		rate, source = model.EffectiveCommissionRate(agent)
		useCustomRate = agent != nil && agent.UseCustomRate
	}
	common.ApiSuccess(c, gin.H{"user_id": user.Id, "use_custom_rate": useCustomRate,
		"rate_percent": commissionBasisPointsToPercent(rate), "rate_source": source})
}

func GetCommissionSelfRecords(c *gin.Context) {
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	records, total, err := model.ListCommissionRecords(c.Query("keyword"), startTime, endTime, c.GetInt("id"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

func GetCommissionSelfReferrals(c *gin.Context) {
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	users, total, err := model.ListCommissionReferrals(c.GetInt("id"), c.Query("keyword"), startTime, endTime, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}
