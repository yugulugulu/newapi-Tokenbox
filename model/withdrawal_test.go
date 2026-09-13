package model

import (
	"fmt"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useWithdrawalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:withdrawal-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&TopUp{},
		&CommissionAgent{},
		&CommissionRecord{},
		&CommissionBalanceMovement{},
		&WithdrawalConfig{},
		&WithdrawalRequest{},
	))

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})
	return db
}

func createActiveWithdrawalConfig(t *testing.T, db *gorm.DB, feeMinor int64, minMinor int64) *WithdrawalConfig {
	t.Helper()
	config, err := CreateWithdrawalConfigTx(db, &WithdrawalConfig{
		ExchangeRateMinor:  WithdrawalExchangeRateScale,
		FeeUsdtMinor:       feeMinor,
		MinAmountUsdtMinor: minMinor,
		Enabled:            true,
	})
	require.NoError(t, err)
	return config
}

func TestCommissionWithdrawalBalanceForUserCalculatesNetUsdt(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "withdraw-agent", AffCode: "withdraw-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               1,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	config := createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)

	require.NoError(t, err)
	assert.Equal(t, config.ExchangeRateMinor, balance.ExchangeRateMinor)
	assert.Equal(t, int64(10000), balance.WithdrawableUsdtMinor)
	assert.Equal(t, common.QuotaFromDecimal(
		decimal.NewFromInt(100).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	), balance.TransferableQuota)
	assert.Equal(t, int64(10000), balance.ActualUsdtMinor)
	assert.True(t, balance.CanWithdraw)
	require.Len(t, balance.CurrencyBalances, 1)
	assert.Equal(t, "USD", balance.CurrencyBalances[0].Currency)
	assert.Equal(t, int64(10000), balance.CurrencyBalances[0].AmountMinor)
}

func TestCommissionWithdrawalBalanceReturnsCommissionWithoutWithdrawalConfig(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "no-config-agent", AffCode: "no-config-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               101,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    50000,
		CommissionAmountMinor: 7500,
	}).Error)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)

	require.NoError(t, err)
	assert.Equal(t, int64(7500), balance.TotalCommissionUsdMinor)
	assert.Equal(t, common.QuotaFromDecimal(
		decimal.NewFromInt(75).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	), balance.TransferableQuota)
	assert.Zero(t, balance.WithdrawableUsdtMinor)
	assert.False(t, balance.CanWithdraw)
}

func TestCommissionWithdrawalBalanceKeepsUsdQuotaSeparateFromUsdtConversion(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "cny-withdraw-agent", AffCode: "cny-withdraw-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               100,
		PaymentCurrency:       "CNY",
		PaymentAmountMinor:    10000,
		CommissionAmountMinor: 3000,
	}).Error)
	config := createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)
	config.ExchangeRateMinor = 7200000 // 1 USD = 7.2 CNY
	require.NoError(t, db.Save(config).Error)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)

	require.NoError(t, err)
	assert.Equal(t, int64(3000), balance.TotalCommissionUsdMinor) // $30.00
	assert.Equal(t, int64(416), balance.WithdrawableUsdtMinor)    // $30 / 7.2
	assert.Equal(t, common.QuotaFromDecimal(
		decimal.NewFromInt(30).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	), balance.TransferableQuota)
}

func TestTransferCommissionBalanceToQuotaMovesFullNetBalance(t *testing.T) {
	db := useWithdrawalTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	agent := User{Username: "transfer-agent", AffCode: "transfer-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               2,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)

	quota, err := TransferCommissionBalanceToQuota(agent.Id)

	require.NoError(t, err)
	assert.Equal(t, common.QuotaFromDecimal(
		decimal.NewFromInt(100).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	), quota)

	var user User
	require.NoError(t, db.First(&user, agent.Id).Error)
	assert.Equal(t, quota, user.Quota)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Zero(t, balance.WithdrawableUsdtMinor)
	assert.Len(t, balance.CurrencyBalances, 0)
}

func TestTransferCommissionBalanceToQuotaIgnoresExchangeRate(t *testing.T) {
	db := useWithdrawalTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	agent := User{Username: "transfer-no-exchange", AffCode: "transfer-no-exchange", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               20,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)
	config, err := ActiveWithdrawalConfig(db)
	require.NoError(t, err)
	config.ExchangeRateMinor = WithdrawalExchangeRateScale * 2
	require.NoError(t, db.Save(config).Error)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), balance.WithdrawableUsdtMinor)
	expectedQuota := common.QuotaFromDecimal(
		decimal.NewFromInt(100).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	assert.Equal(t, expectedQuota, balance.TransferableQuota)

	quota, err := TransferCommissionBalanceToQuota(agent.Id)

	require.NoError(t, err)
	assert.Equal(t, expectedQuota, quota)

	var user User
	require.NoError(t, db.First(&user, agent.Id).Error)
	assert.Equal(t, quota, user.Quota)
}

func TestRechargeEpayCreatesWalletAndAdminCommissionRecord(t *testing.T) {
	db := useWithdrawalTestDB(t)
	previousCommissionEnabled := common.CommissionEnabled
	previousGlobalRateEnabled := common.CommissionGlobalRateEnabled
	previousGlobalRate := common.CommissionGlobalRateBasisPoints
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = false
	common.CommissionGlobalRateBasisPoints = 0
	t.Cleanup(func() {
		common.CommissionEnabled = previousCommissionEnabled
		common.CommissionGlobalRateEnabled = previousGlobalRateEnabled
		common.CommissionGlobalRateBasisPoints = previousGlobalRate
	})

	agent := User{Username: "recharge-commission-agent", AffCode: "recharge-commission-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionAgent{
		UserId: agent.Id, UseCustomRate: true, RateBasisPoints: 3000,
	}).Error)
	descendant := User{
		Username: "recharge-commission-descendant", AffCode: "recharge-commission-descendant", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, ParentUserId: agent.Id,
	}
	require.NoError(t, db.Create(&descendant).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: descendant.Id, Amount: 500, Money: 500, TradeNo: "recharge-commission-500-cny",
		PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending,
	}).Error)

	quotaAdded, err := RechargeEpay("recharge-commission-500-cny", PaymentMethodBalance)
	require.NoError(t, err)
	assert.Equal(t, int(common.QuotaPerUnit*500), quotaAdded)

	var updatedDescendant User
	require.NoError(t, db.First(&updatedDescendant, descendant.Id).Error)
	assert.Equal(t, quotaAdded, updatedDescendant.Quota)

	wallet, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), wallet.TotalCommissionUsdMinor)
	assert.Equal(t, []CommissionCurrencyBalance{{Currency: "CNY", AmountMinor: 15000}}, wallet.CurrencyBalances)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	userRecords, userTotal, err := ListCommissionRecords("", 0, 0, agent.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), userTotal)
	require.Len(t, userRecords, 1)
	assert.Equal(t, int64(50000), userRecords[0].PaymentAmountMinor)
	assert.Equal(t, int64(15000), userRecords[0].CommissionAmountMinor)
	assert.Equal(t, 3000, userRecords[0].CommissionRateBasisPoints)

	summaries, err := SummarizeCommissionRecords("", 0, 0, 0)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, CommissionCurrencySummary{
		Currency: "CNY", PaymentAmountMinor: 50000, CommissionMinor: 15000, RecordCount: 1,
	}, summaries[0])
}

func TestCreateCommissionSelfWithdrawalRejectsBelowMinimumAndNonPositiveActual(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "withdraw-reject", AffCode: "withdraw-reject", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               3,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    5000,
		CommissionAmountMinor: 500,
	}).Error)
	createActiveWithdrawalConfig(t, db, 0, 1000)

	_, err := CreateCommissionSelfWithdrawal(agent.Id, "0x0000000000000000000000000000000000000001")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "低于最低提现额")

	feeAgent := User{Username: "withdraw-fee", AffCode: "withdraw-fee", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&feeAgent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           feeAgent.Id,
		DescendantUserId:      999,
		TopUpId:               33,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, 10000, DefaultWithdrawalMinAmountUsdtMinor)

	_, err = CreateCommissionSelfWithdrawal(feeAgent.Id, "0x0000000000000000000000000000000000000001")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "手续费")
}

func TestWithdrawalFeeDoesNotUnderflowActualAmount(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "withdraw-overflow-fee", AffCode: "withdraw-overflow-fee", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               34,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, math.MaxInt64, DefaultWithdrawalMinAmountUsdtMinor)

	balance, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Zero(t, balance.ActualUsdtMinor)
	assert.False(t, balance.CanWithdraw)

	_, err = CreateCommissionSelfWithdrawal(agent.Id, "0x0000000000000000000000000000000000000001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "手续费")
}

func TestRejectWithdrawalRequestRefundsSourceBalance(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "withdraw-refund", AffCode: "withdraw-refund", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               4,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)

	request, err := CreateCommissionSelfWithdrawal(agent.Id, "0x0000000000000000000000000000000000000001")
	require.NoError(t, err)

	balanceAfterCreate, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Zero(t, balanceAfterCreate.WithdrawableUsdtMinor)

	rejected, err := RejectWithdrawalRequestTx(db, request.Id, "invalid address")
	require.NoError(t, err)
	assert.Equal(t, WithdrawalStatusRejected, rejected.Status)
	assert.Equal(t, "invalid address", rejected.RejectReason)

	balanceAfterReject, err := CommissionWithdrawalBalanceForUser(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), balanceAfterReject.WithdrawableUsdtMinor)
}

func TestWithdrawalConfigEnableDisablesPreviousConfig(t *testing.T) {
	db := useWithdrawalTestDB(t)
	first := createActiveWithdrawalConfig(t, db, 0, 1000)
	second := createActiveWithdrawalConfig(t, db, 100, 1000)

	var configs []WithdrawalConfig
	require.NoError(t, db.Order("id DESC").Find(&configs).Error)
	require.Len(t, configs, 2)

	var enabledCount int64
	require.NoError(t, db.Model(&WithdrawalConfig{}).Where("enabled = ?", true).Count(&enabledCount).Error)
	assert.Equal(t, int64(1), enabledCount)
	assert.Equal(t, second.Id, configs[0].Id)
	assert.True(t, configs[0].Enabled)
	assert.False(t, configs[1].Enabled)
	assert.Equal(t, first.Id, configs[1].Id)
}

func TestWithdrawalDateFiltersUseExclusiveEndTime(t *testing.T) {
	db := useWithdrawalTestDB(t)
	firstUser := User{Username: "withdraw-date-first", Email: "withdraw-date-first@example.com", AffCode: "withdraw-date-first", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	secondUser := User{Username: "withdraw-date-second", Email: "withdraw-date-second@example.com", AffCode: "withdraw-date-second", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&firstUser).Error)
	require.NoError(t, db.Create(&secondUser).Error)
	require.NoError(t, db.Create(&WithdrawalRequest{UserId: firstUser.Id, Status: WithdrawalStatusPending, CreatedAt: 100, UpdatedAt: 100}).Error)
	require.NoError(t, db.Create(&WithdrawalRequest{UserId: secondUser.Id, Status: WithdrawalStatusPending, CreatedAt: 200, UpdatedAt: 200}).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}

	requests, total, err := ListWithdrawalRequests("", "", 150, 250, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, requests, 1)
	assert.Equal(t, secondUser.Id, requests[0].UserId)

	selfRequests, selfTotal, err := GetCommissionSelfWithdrawals(secondUser.Id, 150, 250, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), selfTotal)
	require.Len(t, selfRequests, 1)
	assert.Equal(t, int64(200), selfRequests[0].CreatedAt)
}

func TestCommissionSelfWithdrawalsOrderNewestFirst(t *testing.T) {
	db := useWithdrawalTestDB(t)
	user := User{Username: "withdraw-order", AffCode: "withdraw-order", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&WithdrawalRequest{UserId: user.Id, Status: WithdrawalStatusPending, CreatedAt: 100, UpdatedAt: 100}).Error)
	require.NoError(t, db.Create(&WithdrawalRequest{UserId: user.Id, Status: WithdrawalStatusApproved, CreatedAt: 200, UpdatedAt: 200}).Error)

	requests, total, err := GetCommissionSelfWithdrawals(user.Id, 0, 0, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, requests, 2)
	assert.Equal(t, int64(200), requests[0].CreatedAt)
	assert.Equal(t, int64(100), requests[1].CreatedAt)
}

func TestWithdrawalRequestStateTransitions(t *testing.T) {
	db := useWithdrawalTestDB(t)
	agent := User{Username: "withdraw-state", AffCode: "withdraw-state", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      999,
		TopUpId:               5,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    100000,
		CommissionAmountMinor: 10000,
	}).Error)
	createActiveWithdrawalConfig(t, db, 0, DefaultWithdrawalMinAmountUsdtMinor)

	request, err := CreateCommissionSelfWithdrawal(agent.Id, "0x0000000000000000000000000000000000000001")
	require.NoError(t, err)

	approved, err := ApproveWithdrawalRequestTx(db, request.Id, "0xabc")
	require.NoError(t, err)
	assert.Equal(t, WithdrawalStatusApproved, approved.Status)
	assert.Equal(t, "0xabc", approved.TxHash)

	_, err = ApproveWithdrawalRequestTx(db, request.Id, "0xdef")
	require.Error(t, err)

	_, err = RejectWithdrawalRequestTx(db, request.Id, "too late")
	require.Error(t, err)
}
