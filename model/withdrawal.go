package model

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	WithdrawalStatusPending  = "pending"
	WithdrawalStatusApproved = "approved"
	WithdrawalStatusRejected = "rejected"

	MovementKindWithdraw       = "withdraw"
	MovementKindTransfer       = "transfer"
	MovementKindWithdrawRefund = "withdraw_refund"

	WithdrawalCurrency = "USDT"
	WithdrawalNetwork  = "BSC"

	WithdrawalExchangeRateScale = 1000000
	WithdrawalUsdtScale         = 100

	DefaultWithdrawalMinAmountUsdtMinor = 1000
)

type CommissionBalanceMovement struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"index"`
	Currency      string `json:"currency" gorm:"type:varchar(16);index"`
	AmountMinor   int64  `json:"amount_minor"`
	Kind          string `json:"kind" gorm:"type:varchar(32);index"`
	ReferenceType string `json:"reference_type" gorm:"type:varchar(32);index"`
	ReferenceId   int    `json:"reference_id" gorm:"index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
}

type WithdrawalConfig struct {
	Id                 int    `json:"id"`
	Currency           string `json:"currency" gorm:"type:varchar(16);index"`
	Network            string `json:"network" gorm:"type:varchar(16);index"`
	ExchangeRateMinor  int64  `json:"exchange_rate_minor"`
	FeeUsdtMinor       int64  `json:"fee_usdt_minor"`
	MinAmountUsdtMinor int64  `json:"min_amount_usdt_minor"`
	Enabled            bool   `json:"enabled" gorm:"column:enabled"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type WithdrawalRequest struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index"`
	AmountUsdtMinor   int64  `json:"amount_usdt_minor"`
	FeeUsdtMinor      int64  `json:"fee_usdt_minor"`
	ActualUsdtMinor   int64  `json:"actual_usdt_minor"`
	Network           string `json:"network" gorm:"type:varchar(16);index"`
	Address           string `json:"address" gorm:"type:varchar(128)"`
	Status            string `json:"status" gorm:"type:varchar(32);index"`
	TxHash            string `json:"tx_hash" gorm:"type:varchar(255)"`
	RejectReason      string `json:"reject_reason" gorm:"type:varchar(255)"`
	SourceEntriesJson string `json:"-" gorm:"type:text;column:source_entries_json"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type WithdrawalSourceEntry struct {
	Currency    string `json:"currency"`
	AmountMinor int64  `json:"amount_minor"`
}

type CommissionCurrencyBalance struct {
	Currency    string `json:"currency"`
	AmountMinor int64  `json:"amount_minor"`
}

type CommissionWithdrawalBalance struct {
	CurrencyBalances        []CommissionCurrencyBalance `json:"currency_balances"`
	TotalCommissionUsdMinor int64                       `json:"total_commission_usd_minor"`
	WithdrawableUsdtMinor   int64                       `json:"withdrawable_usdt_minor"`
	TransferableQuota       int                         `json:"transferable_quota"`
	ExchangeRateMinor       int64                       `json:"exchange_rate_minor"`
	FeeUsdtMinor            int64                       `json:"fee_usdt_minor"`
	ActualUsdtMinor         int64                       `json:"actual_usdt_minor"`
	MinAmountUsdtMinor      int64                       `json:"min_amount_usdt_minor"`
	CanWithdraw             bool                        `json:"can_withdraw"`
}

type WithdrawalRequestView struct {
	WithdrawalRequest
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

func commissionCurrencyMinorScale(currency string) int64 {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "IDR", "JPY", "KRW", "VND":
		return 1
	default:
		return 100
	}
}

func ActiveWithdrawalConfig(tx *gorm.DB) (*WithdrawalConfig, error) {
	var config WithdrawalConfig
	err := lockForUpdate(tx).Where("enabled = ?", true).Order("id DESC").First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func normalizeWithdrawalConfig(config *WithdrawalConfig) {
	config.Currency = WithdrawalCurrency
	config.Network = WithdrawalNetwork
	if config.ExchangeRateMinor <= 0 {
		config.ExchangeRateMinor = WithdrawalExchangeRateScale
	}
	if config.FeeUsdtMinor < 0 {
		config.FeeUsdtMinor = 0
	}
	if config.MinAmountUsdtMinor <= 0 {
		config.MinAmountUsdtMinor = DefaultWithdrawalMinAmountUsdtMinor
	}
}

func validateWithdrawalConfig(config *WithdrawalConfig) error {
	if config.Currency != WithdrawalCurrency {
		return errors.New("提现币种仅支持 USDT")
	}
	if config.Network != WithdrawalNetwork {
		return errors.New("提现网络仅支持 BSC")
	}
	if config.ExchangeRateMinor <= 0 {
		return errors.New("汇率必须大于 0")
	}
	if config.FeeUsdtMinor < 0 {
		return errors.New("手续费不能小于 0")
	}
	if config.MinAmountUsdtMinor <= 0 {
		return errors.New("起始金额必须大于 0")
	}
	return nil
}

func sourceAmountToUsdtMinor(amountMinor int64, currency string, exchangeRateMinor int64) (int64, error) {
	if amountMinor <= 0 {
		return 0, nil
	}
	if exchangeRateMinor <= 0 {
		return 0, errors.New("提现配置汇率无效")
	}
	scale := commissionCurrencyMinorScale(currency)
	sourceMajor := decimal.NewFromInt(amountMinor).Div(decimal.NewFromInt(scale))
	exchangeMajor := decimal.NewFromInt(exchangeRateMinor).Div(decimal.NewFromInt(WithdrawalExchangeRateScale))
	if exchangeMajor.LessThanOrEqual(decimal.Zero) {
		return 0, errors.New("提现配置汇率无效")
	}
	usdtMajor := sourceMajor.Div(exchangeMajor)
	usdtMinor := usdtMajor.Mul(decimal.NewFromInt(WithdrawalUsdtScale))
	if usdtMinor.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, errors.New("反佣余额换算结果超出可处理范围")
	}
	return usdtMinor.IntPart(), nil
}

func addCommissionAmount(total int64, delta int64) (int64, error) {
	if delta > 0 && total > math.MaxInt64-delta {
		return 0, errors.New("反佣余额超出可处理范围")
	}
	if delta < 0 && total < math.MinInt64-delta {
		return 0, errors.New("反佣余额超出可处理范围")
	}
	return total + delta, nil
}

func commissionBalancesToQuota(balances []CommissionCurrencyBalance) (int, error) {
	totalUsdMinor, err := commissionBalancesToUsdMinor(balances)
	if err != nil {
		return 0, err
	}
	quota, clamp := common.QuotaFromDecimalChecked(
		decimal.NewFromInt(totalUsdMinor).
			Div(decimal.NewFromInt(WithdrawalUsdtScale)).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	if clamp != nil {
		return 0, errors.New("可划转额度超出平台余额上限")
	}
	return quota, nil
}

// Commission amounts use the platform's dollar quota semantics: the numeric
// major-unit value of a commission is the corresponding USD value. The
// withdrawal exchange rate is applied only when converting this balance to
// USDT for an external payout.
func commissionBalancesToUsdMinor(balances []CommissionCurrencyBalance) (int64, error) {
	totalUsdMinor := decimal.Zero
	for _, balance := range balances {
		if balance.AmountMinor <= 0 {
			continue
		}
		scale := commissionCurrencyMinorScale(balance.Currency)
		totalUsdMinor = totalUsdMinor.Add(
			decimal.NewFromInt(balance.AmountMinor).
				Div(decimal.NewFromInt(scale)).
				Mul(decimal.NewFromInt(WithdrawalUsdtScale)),
		)
	}
	if totalUsdMinor.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, errors.New("反佣余额超出可处理范围")
	}
	return totalUsdMinor.IntPart(), nil
}

func availableCommissionBalancesLocked(tx *gorm.DB, userId int) ([]CommissionCurrencyBalance, error) {
	var records []CommissionRecord
	if err := lockForUpdate(tx).Model(&CommissionRecord{}).
		Where("agent_user_id = ?", userId).
		Select("payment_currency", "commission_amount_minor").
		Find(&records).Error; err != nil {
		return nil, err
	}
	var movements []CommissionBalanceMovement
	if err := lockForUpdate(tx).Model(&CommissionBalanceMovement{}).
		Where("user_id = ?", userId).
		Select("currency", "amount_minor").
		Find(&movements).Error; err != nil {
		return nil, err
	}

	commissionTotal := make(map[string]int64)
	for _, record := range records {
		currency := normalizeCommissionCurrency(record.PaymentCurrency)
		total, err := addCommissionAmount(commissionTotal[currency], record.CommissionAmountMinor)
		if err != nil {
			return nil, err
		}
		commissionTotal[currency] = total
	}
	for _, movement := range movements {
		currency := normalizeCommissionCurrency(movement.Currency)
		if movement.AmountMinor == math.MinInt64 {
			return nil, errors.New("反佣余额流水超出可处理范围")
		}
		total, err := addCommissionAmount(commissionTotal[currency], -movement.AmountMinor)
		if err != nil {
			return nil, err
		}
		commissionTotal[currency] = total
	}

	balances := make([]CommissionCurrencyBalance, 0, len(commissionTotal))
	for currency, amount := range commissionTotal {
		if amount > 0 {
			balances = append(balances, CommissionCurrencyBalance{
				Currency:    currency,
				AmountMinor: amount,
			})
		}
	}
	return balances, nil
}

func CommissionWithdrawalBalanceForUser(userId int) (*CommissionWithdrawalBalance, error) {
	var result *CommissionWithdrawalBalance
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "role").First(&user, userId).Error; err != nil {
			return err
		}
		if user.Role == common.RoleRootUser {
			return errors.New("root 用户不能参与反佣提现")
		}
		balances, err := availableCommissionBalancesLocked(tx, userId)
		if err != nil {
			return err
		}
		totalCommissionUsdMinor, err := commissionBalancesToUsdMinor(balances)
		if err != nil {
			return err
		}
		transferableQuota, err := commissionBalancesToQuota(balances)
		if err != nil {
			return err
		}

		// The commission total and balance transfer remain available even before
		// root configures an external withdrawal network.
		config, configErr := ActiveWithdrawalConfig(tx)
		if configErr != nil && !errors.Is(configErr, gorm.ErrRecordNotFound) {
			return configErr
		}
		var withdrawable int64
		var exchangeRateMinor, feeUsdtMinor, minAmountUsdtMinor int64
		if configErr == nil {
			for _, balance := range balances {
				usdtMinor, err := sourceAmountToUsdtMinor(balance.AmountMinor, balance.Currency, config.ExchangeRateMinor)
				if err != nil {
					return err
				}
				withdrawable, err = addCommissionAmount(withdrawable, usdtMinor)
				if err != nil {
					return err
				}
			}
			exchangeRateMinor = config.ExchangeRateMinor
			feeUsdtMinor = config.FeeUsdtMinor
			minAmountUsdtMinor = config.MinAmountUsdtMinor
		}
		var actual int64
		if feeUsdtMinor < withdrawable {
			actual = withdrawable - feeUsdtMinor
		}
		result = &CommissionWithdrawalBalance{
			CurrencyBalances:        balances,
			TotalCommissionUsdMinor: totalCommissionUsdMinor,
			WithdrawableUsdtMinor:   withdrawable,
			TransferableQuota:       transferableQuota,
			ExchangeRateMinor:       exchangeRateMinor,
			FeeUsdtMinor:            feeUsdtMinor,
			ActualUsdtMinor:         actual,
			MinAmountUsdtMinor:      minAmountUsdtMinor,
			CanWithdraw:             configErr == nil && withdrawable >= minAmountUsdtMinor && actual > 0,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func createCommissionBalanceMovementsTx(tx *gorm.DB, userId int, balances []CommissionCurrencyBalance, kind string, referenceType string, referenceId int) ([]WithdrawalSourceEntry, error) {
	entries := make([]WithdrawalSourceEntry, 0, len(balances))
	for _, balance := range balances {
		if balance.AmountMinor <= 0 {
			continue
		}
		entries = append(entries, WithdrawalSourceEntry{
			Currency:    balance.Currency,
			AmountMinor: balance.AmountMinor,
		})
		if err := tx.Create(&CommissionBalanceMovement{
			UserId:        userId,
			Currency:      balance.Currency,
			AmountMinor:   balance.AmountMinor,
			Kind:          kind,
			ReferenceType: referenceType,
			ReferenceId:   referenceId,
			CreatedAt:     common.GetTimestamp(),
		}).Error; err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func transferCommissionBalanceToQuotaTx(tx *gorm.DB, userId int) (int, error) {
	var user User
	if err := lockForUpdate(tx).Select("id", "role", "quota").First(&user, userId).Error; err != nil {
		return 0, err
	}
	if user.Role == common.RoleRootUser {
		return 0, errors.New("root 用户不能参与反佣提现")
	}
	balances, err := availableCommissionBalancesLocked(tx, userId)
	if err != nil {
		return 0, err
	}
	if len(balances) == 0 {
		return 0, errors.New("没有可划转的反佣余额")
	}
	quota, err := commissionBalancesToQuota(balances)
	if err != nil {
		return 0, err
	}
	if quota <= 0 {
		return 0, errors.New("可划转额度不足")
	}
	if int64(user.Quota)+int64(quota) > int64(common.MaxQuota) {
		return 0, errors.New("划转后余额超出平台余额上限")
	}
	if _, err := createCommissionBalanceMovementsTx(tx, userId, balances, MovementKindTransfer, "transfer", userId); err != nil {
		return 0, err
	}
	if err := tx.Model(&User{}).Where("id = ?", userId).
		Update("quota", gorm.Expr("quota + ?", quota)).Error; err != nil {
		return 0, err
	}
	return quota, nil
}

func TransferCommissionBalanceToQuota(userId int) (int, error) {
	var quota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		quota, err = transferCommissionBalanceToQuotaTx(tx, userId)
		return err
	})
	return quota, err
}

func CreateWithdrawalRequestTx(tx *gorm.DB, userId int, address string) (*WithdrawalRequest, error) {
	var user User
	if err := lockForUpdate(tx).Select("id", "role").First(&user, userId).Error; err != nil {
		return nil, err
	}
	if user.Role == common.RoleRootUser {
		return nil, errors.New("root 用户不能参与反佣提现")
	}
	config, err := ActiveWithdrawalConfig(tx)
	if err != nil {
		return nil, err
	}
	balances, err := availableCommissionBalancesLocked(tx, userId)
	if err != nil {
		return nil, err
	}
	if len(balances) == 0 {
		return nil, errors.New("没有可提现的反佣余额")
	}
	var withdrawableUsdtMinor int64
	for _, balance := range balances {
		usdtMinor, err := sourceAmountToUsdtMinor(balance.AmountMinor, balance.Currency, config.ExchangeRateMinor)
		if err != nil {
			return nil, err
		}
		withdrawableUsdtMinor, err = addCommissionAmount(withdrawableUsdtMinor, usdtMinor)
		if err != nil {
			return nil, err
		}
	}
	if withdrawableUsdtMinor < config.MinAmountUsdtMinor {
		return nil, errors.New("可提现金额低于最低提现额")
	}
	if config.FeeUsdtMinor >= withdrawableUsdtMinor {
		return nil, errors.New("提现手续费不能大于或等于提现金额")
	}
	actualUsdtMinor := withdrawableUsdtMinor - config.FeeUsdtMinor

	request := &WithdrawalRequest{
		UserId:          userId,
		AmountUsdtMinor: withdrawableUsdtMinor,
		FeeUsdtMinor:    config.FeeUsdtMinor,
		ActualUsdtMinor: actualUsdtMinor,
		Network:         WithdrawalNetwork,
		Address:         strings.TrimSpace(address),
		Status:          WithdrawalStatusPending,
		CreatedAt:       common.GetTimestamp(),
		UpdatedAt:       common.GetTimestamp(),
	}
	if err := tx.Create(request).Error; err != nil {
		return nil, err
	}
	entries, err := createCommissionBalanceMovementsTx(tx, userId, balances, MovementKindWithdraw, "withdrawal", request.Id)
	if err != nil {
		return nil, err
	}
	sourceJSON, err := common.Marshal(entries)
	if err != nil {
		return nil, err
	}
	request.SourceEntriesJson = string(sourceJSON)
	if err := tx.Model(request).Update("source_entries_json", request.SourceEntriesJson).Error; err != nil {
		return nil, err
	}
	return request, nil
}

func CreateCommissionSelfWithdrawal(userId int, address string) (*WithdrawalRequest, error) {
	var request *WithdrawalRequest
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		request, err = CreateWithdrawalRequestTx(tx, userId, address)
		return err
	})
	return request, err
}

func ApproveWithdrawalRequestTx(tx *gorm.DB, requestId int, txHash string) (*WithdrawalRequest, error) {
	var request WithdrawalRequest
	if err := lockForUpdate(tx).First(&request, requestId).Error; err != nil {
		return nil, err
	}
	if request.Status != WithdrawalStatusPending {
		return nil, errors.New("提现申请当前状态不允许通过")
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, errors.New("交易 hash 不能为空")
	}
	updates := map[string]interface{}{
		"status":     WithdrawalStatusApproved,
		"tx_hash":    txHash,
		"updated_at": common.GetTimestamp(),
	}
	if err := tx.Model(&WithdrawalRequest{}).Where("id = ?", requestId).Updates(updates).Error; err != nil {
		return nil, err
	}
	request.Status = WithdrawalStatusApproved
	request.TxHash = txHash
	request.UpdatedAt = updates["updated_at"].(int64)
	return &request, nil
}

func RejectWithdrawalRequestTx(tx *gorm.DB, requestId int, reason string) (*WithdrawalRequest, error) {
	var request WithdrawalRequest
	if err := lockForUpdate(tx).First(&request, requestId).Error; err != nil {
		return nil, err
	}
	if request.Status != WithdrawalStatusPending {
		return nil, errors.New("提现申请当前状态不允许驳回")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("驳回原因不能为空")
	}
	var entries []WithdrawalSourceEntry
	if strings.TrimSpace(request.SourceEntriesJson) != "" {
		if err := common.Unmarshal([]byte(request.SourceEntriesJson), &entries); err != nil {
			return nil, errors.New("提现来源数据无效")
		}
	}
	now := common.GetTimestamp()
	for _, entry := range entries {
		if entry.AmountMinor <= 0 {
			continue
		}
		if err := tx.Create(&CommissionBalanceMovement{
			UserId:        request.UserId,
			Currency:      entry.Currency,
			AmountMinor:   -entry.AmountMinor,
			Kind:          MovementKindWithdrawRefund,
			ReferenceType: "withdrawal",
			ReferenceId:   request.Id,
			CreatedAt:     now,
		}).Error; err != nil {
			return nil, err
		}
	}
	updates := map[string]interface{}{
		"status":        WithdrawalStatusRejected,
		"reject_reason": reason,
		"updated_at":    now,
	}
	if err := tx.Model(&WithdrawalRequest{}).Where("id = ?", requestId).Updates(updates).Error; err != nil {
		return nil, err
	}
	request.Status = WithdrawalStatusRejected
	request.RejectReason = reason
	request.UpdatedAt = now
	return &request, nil
}

func ListWithdrawalConfigs() ([]WithdrawalConfig, error) {
	var configs []WithdrawalConfig
	err := DB.Order("id DESC").Find(&configs).Error
	return configs, err
}

func CreateWithdrawalConfigTx(tx *gorm.DB, config *WithdrawalConfig) (*WithdrawalConfig, error) {
	normalizeWithdrawalConfig(config)
	if err := validateWithdrawalConfig(config); err != nil {
		return nil, err
	}
	if config.Enabled {
		if err := tx.Model(&WithdrawalConfig{}).Where("id <> ?", config.Id).
			Update("enabled", false).Error; err != nil {
			return nil, err
		}
	}
	config.Id = 0
	config.CreatedAt = common.GetTimestamp()
	config.UpdatedAt = common.GetTimestamp()
	if err := tx.Create(config).Error; err != nil {
		return nil, err
	}
	return config, nil
}

func UpdateWithdrawalConfigTx(tx *gorm.DB, id int, config *WithdrawalConfig) (*WithdrawalConfig, error) {
	var current WithdrawalConfig
	if err := lockForUpdate(tx).First(&current, id).Error; err != nil {
		return nil, err
	}
	normalizeWithdrawalConfig(config)
	if err := validateWithdrawalConfig(config); err != nil {
		return nil, err
	}
	if config.Enabled {
		if err := tx.Model(&WithdrawalConfig{}).Where("id <> ?", id).
			Update("enabled", false).Error; err != nil {
			return nil, err
		}
	}
	updates := map[string]interface{}{
		"currency":              WithdrawalCurrency,
		"network":               WithdrawalNetwork,
		"exchange_rate_minor":   config.ExchangeRateMinor,
		"fee_usdt_minor":        config.FeeUsdtMinor,
		"min_amount_usdt_minor": config.MinAmountUsdtMinor,
		"enabled":               config.Enabled,
		"updated_at":            common.GetTimestamp(),
	}
	if err := tx.Model(&WithdrawalConfig{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	current = *config
	current.Id = id
	current.UpdatedAt = updates["updated_at"].(int64)
	return &current, nil
}

func DeleteWithdrawalConfigTx(tx *gorm.DB, id int) error {
	var config WithdrawalConfig
	if err := lockForUpdate(tx).First(&config, id).Error; err != nil {
		return err
	}
	if config.Enabled {
		return errors.New("不能删除启用中的提现配置，请先启用其他配置")
	}
	return tx.Delete(&WithdrawalConfig{}, id).Error
}

func ListWithdrawalRequests(keyword string, status string, startTime int64, endTime int64, pageInfo *common.PageInfo) ([]WithdrawalRequestView, int64, error) {
	query := DB.Table("withdrawal_requests AS wr").
		Joins("JOIN users AS u ON u.id = wr.user_id")
	if status != "" {
		query = query.Where("wr.status = ?", status)
	}
	if startTime > 0 {
		query = query.Where("wr.created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("wr.created_at < ?", endTime)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		idCast := "CAST(u.id AS CHAR)"
		if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
			idCast = "CAST(u.id AS TEXT)"
		}
		query = query.Where("LOWER(u.username) LIKE ? OR LOWER(u.email) LIKE ? OR LOWER(u.display_name) LIKE ? OR "+idCast+" LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var views []WithdrawalRequestView
	err := query.Select("wr.*, u.username AS username, u.display_name AS display_name, u.email AS email").
		Order("wr.created_at DESC").Order("wr.id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&views).Error
	return views, total, err
}

func GetCommissionSelfWithdrawals(userId int, startTime int64, endTime int64, pageInfo *common.PageInfo) ([]WithdrawalRequest, int64, error) {
	query := DB.Model(&WithdrawalRequest{}).Where("user_id = ?", userId)
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at < ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var requests []WithdrawalRequest
	err := query.Order("created_at DESC").Order("id DESC").
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&requests).Error
	return requests, total, err
}
