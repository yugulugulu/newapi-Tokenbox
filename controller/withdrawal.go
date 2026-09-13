package controller

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var web3AddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

type withdrawalCreateRequest struct {
	Address string `json:"address"`
}

type withdrawalConfigRequest struct {
	ExchangeRate  float64 `json:"exchange_rate"`
	FeeUsdt       float64 `json:"fee_usdt"`
	MinAmountUsdt float64 `json:"min_amount_usdt"`
	Enabled       bool    `json:"enabled"`
}

type withdrawalApproveRequest struct {
	TxHash string `json:"tx_hash"`
}

type withdrawalRejectRequest struct {
	RejectReason string `json:"reject_reason"`
}

func requireCommissionReadableSelf(c *gin.Context) (*model.User, bool) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if user.Role == common.RoleRootUser {
		common.ApiErrorMsg(c, "root 用户不能查看反佣余额")
		return nil, false
	}
	return user, true
}

func floatToDecimalMinor(value float64, scale int64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, false
	}
	minor := decimal.NewFromFloat(value).Mul(decimal.NewFromInt(scale)).IntPart()
	if minor < 0 || minor > math.MaxInt64 {
		return 0, false
	}
	return minor, true
}

func configFromRequest(req withdrawalConfigRequest) (*model.WithdrawalConfig, error) {
	exchangeRateMinor, ok := floatToDecimalMinor(req.ExchangeRate, model.WithdrawalExchangeRateScale)
	if !ok || exchangeRateMinor <= 0 {
		return nil, errors.New("无效的提现配置")
	}
	feeMinor, ok := floatToDecimalMinor(req.FeeUsdt, model.WithdrawalUsdtScale)
	if !ok {
		return nil, errors.New("无效的提现配置")
	}
	minMinor, ok := floatToDecimalMinor(req.MinAmountUsdt, model.WithdrawalUsdtScale)
	if !ok || minMinor <= 0 {
		return nil, errors.New("无效的提现配置")
	}
	return &model.WithdrawalConfig{
		Currency:           model.WithdrawalCurrency,
		Network:            model.WithdrawalNetwork,
		ExchangeRateMinor:  exchangeRateMinor,
		FeeUsdtMinor:       feeMinor,
		MinAmountUsdtMinor: minMinor,
		Enabled:            req.Enabled,
	}, nil
}

func GetCommissionSelfBalance(c *gin.Context) {
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	balance, err := model.CommissionWithdrawalBalanceForUser(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, balance)
}

func TransferCommissionSelfBalance(c *gin.Context) {
	userId := c.GetInt("id")
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	quota, err := model.TransferCommissionBalanceToQuota(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.PublishUserAuthCache(userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"quota": quota})
}

func CreateCommissionSelfWithdrawal(c *gin.Context) {
	userId := c.GetInt("id")
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	var req withdrawalCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if !web3AddressPattern.MatchString(strings.TrimSpace(req.Address)) {
		common.ApiErrorMsg(c, "钱包地址必须是有效的 BSC 地址")
		return
	}
	request, err := model.CreateCommissionSelfWithdrawal(userId, req.Address)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, request)
}

func GetCommissionSelfWithdrawals(c *gin.Context) {
	if _, ok := requireCommissionReadableSelf(c); !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	requests, total, err := model.GetCommissionSelfWithdrawals(c.GetInt("id"), startTime, endTime, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(requests)
	common.ApiSuccess(c, pageInfo)
}

func GetWithdrawalConfigs(c *gin.Context) {
	configs, err := model.ListWithdrawalConfigs()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": configs})
}

func CreateWithdrawalConfig(c *gin.Context) {
	var req withdrawalConfigRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	config, err := configFromRequest(req)
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现配置")
		return
	}
	var created *model.WithdrawalConfig
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		created, txErr = model.CreateWithdrawalConfigTx(tx, config)
		return txErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "withdrawal.config_create", map[string]interface{}{"config_id": created.Id})
	common.ApiSuccess(c, created)
}

func UpdateWithdrawalConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现配置 ID")
		return
	}
	var req withdrawalConfigRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	config, err := configFromRequest(req)
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现配置")
		return
	}
	var updated *model.WithdrawalConfig
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		updated, txErr = model.UpdateWithdrawalConfigTx(tx, id, config)
		return txErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "withdrawal.config_update", map[string]interface{}{"config_id": id})
	common.ApiSuccess(c, updated)
}

func DeleteWithdrawalConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现配置 ID")
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return model.DeleteWithdrawalConfigTx(tx, id)
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "withdrawal.config_delete", map[string]interface{}{"config_id": id})
	common.ApiSuccess(c, nil)
}

func GetWithdrawalRequests(c *gin.Context) {
	startTime, endTime, err := commissionTimeRange(c)
	if err != nil {
		common.ApiErrorMsg(c, "无效的时间范围")
		return
	}
	pageInfo := common.GetPageQuery(c)
	requests, total, err := model.ListWithdrawalRequests(c.Query("keyword"), c.Query("status"), startTime, endTime, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(requests)
	common.ApiSuccess(c, pageInfo)
}

func ApproveWithdrawalRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现申请 ID")
		return
	}
	var req withdrawalApproveRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	var request *model.WithdrawalRequest
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		request, txErr = model.ApproveWithdrawalRequestTx(tx, id, req.TxHash)
		return txErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, request.UserId, "withdrawal.approve", map[string]interface{}{
		"withdrawal_id": id,
		"tx_hash":       req.TxHash,
	})
	common.ApiSuccess(c, request)
}

func RejectWithdrawalRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现申请 ID")
		return
	}
	var req withdrawalRejectRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	var request *model.WithdrawalRequest
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		request, txErr = model.RejectWithdrawalRequestTx(tx, id, req.RejectReason)
		return txErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, request.UserId, "withdrawal.reject", map[string]interface{}{
		"withdrawal_id": id,
		"reject_reason": req.RejectReason,
	})
	common.ApiSuccess(c, request)
}
