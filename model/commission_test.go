package model

import (
	"fmt"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useCommissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:commission-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &CommissionAgent{}, &CommissionRecord{}))

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})
	return db
}

func TestEffectiveCommissionRateCustomRateOverridesGlobal(t *testing.T) {
	previousEnabled := common.CommissionGlobalRateEnabled
	previousRate := common.CommissionGlobalRateBasisPoints
	t.Cleanup(func() {
		common.CommissionGlobalRateEnabled = previousEnabled
		common.CommissionGlobalRateBasisPoints = previousRate
	})
	common.CommissionGlobalRateEnabled = true
	common.CommissionGlobalRateBasisPoints = 1200

	tests := []struct {
		name       string
		agent      *CommissionAgent
		wantRate   int
		wantSource string
	}{
		{name: "positive custom rate", agent: &CommissionAgent{UseCustomRate: true, RateBasisPoints: 800}, wantRate: 800, wantSource: "agent"},
		{name: "zero custom rate", agent: &CommissionAgent{UseCustomRate: true, RateBasisPoints: 0}, wantRate: 0, wantSource: "agent"},
		{name: "global fallback", agent: &CommissionAgent{}, wantRate: 1200, wantSource: "global"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, source := EffectiveCommissionRate(tt.agent)
			assert.Equal(t, tt.wantRate, rate)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}

func TestEffectiveCommissionRateCustomRateWorksWhenGlobalRateIsDisabled(t *testing.T) {
	previousEnabled := common.CommissionGlobalRateEnabled
	previousRate := common.CommissionGlobalRateBasisPoints
	t.Cleanup(func() {
		common.CommissionGlobalRateEnabled = previousEnabled
		common.CommissionGlobalRateBasisPoints = previousRate
	})

	common.CommissionGlobalRateEnabled = false
	common.CommissionGlobalRateBasisPoints = 1200

	rate, source := EffectiveCommissionRate(&CommissionAgent{
		UseCustomRate:   true,
		RateBasisPoints: 800,
	})
	assert.Equal(t, 800, rate)
	assert.Equal(t, "agent", source)

	rate, source = EffectiveCommissionRate(&CommissionAgent{})
	assert.Zero(t, rate)
	assert.Equal(t, "none", source)
}

func TestCalculateCommissionAmountAvoidsOverflow(t *testing.T) {
	require.Equal(t, int64(1234), calculateCommissionAmount(12345, 1000))
	require.Equal(t, int64(math.MaxInt64), calculateCommissionAmount(math.MaxInt64, CommissionRateMaxBasisPoints))
	require.Zero(t, calculateCommissionAmount(10000, 0))
}

func TestPaymentAmountToMinorBoundsConversion(t *testing.T) {
	assert.Equal(t, int64(1235), PaymentAmountToMinor(12.345, "USD"))
	assert.Equal(t, int64(12), PaymentAmountToMinor(12.345, "JPY"))
	assert.Zero(t, PaymentAmountToMinor(-1, "USD"))
	assert.Zero(t, PaymentAmountToMinor(math.NaN(), "USD"))
	assert.Equal(t, int64(math.MaxInt64), PaymentAmountToMinor(math.MaxFloat64, "USD"))
}

func TestCreateCommissionForTopUpUsesCustomRateBeforeGlobalRate(t *testing.T) {
	db := useCommissionTestDB(t)
	previousEnabled := common.CommissionEnabled
	previousGlobalEnabled := common.CommissionGlobalRateEnabled
	previousGlobalRate := common.CommissionGlobalRateBasisPoints
	t.Cleanup(func() {
		common.CommissionEnabled = previousEnabled
		common.CommissionGlobalRateEnabled = previousGlobalEnabled
		common.CommissionGlobalRateBasisPoints = previousGlobalRate
	})
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = true
	common.CommissionGlobalRateBasisPoints = 1200

	agent := User{Username: "agent-custom", AffCode: "agent-custom", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{Username: "descendant-custom", AffCode: "descendant-custom", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id}
	require.NoError(t, db.Create(&descendant).Error)
	require.NoError(t, db.Create(&CommissionAgent{UserId: agent.Id, UseCustomRate: true, RateBasisPoints: 800}).Error)
	topUp := TopUp{UserId: descendant.Id, TradeNo: "custom-rate", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&topUp).Error)

	require.NoError(t, CreateCommissionForTopUpTx(db, &topUp))
	require.NoError(t, CreateCommissionForTopUpTx(db, &topUp))

	var records []CommissionRecord
	require.NoError(t, db.Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, 800, records[0].CommissionRateBasisPoints)
	assert.Equal(t, int64(800), records[0].CommissionAmountMinor)
}

func TestCreateCommissionForTopUpFallsBackToEnabledGlobalRate(t *testing.T) {
	db := useCommissionTestDB(t)
	previousEnabled := common.CommissionEnabled
	previousGlobalEnabled := common.CommissionGlobalRateEnabled
	previousGlobalRate := common.CommissionGlobalRateBasisPoints
	t.Cleanup(func() {
		common.CommissionEnabled = previousEnabled
		common.CommissionGlobalRateEnabled = previousGlobalEnabled
		common.CommissionGlobalRateBasisPoints = previousGlobalRate
	})
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = true
	common.CommissionGlobalRateBasisPoints = 1200

	agent := User{Username: "agent-global", AffCode: "agent-global", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{Username: "descendant-global", AffCode: "descendant-global", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id}
	require.NoError(t, db.Create(&descendant).Error)
	topUp := TopUp{UserId: descendant.Id, TradeNo: "global-rate", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&topUp).Error)

	require.NoError(t, CreateCommissionForTopUpTx(db, &topUp))

	var record CommissionRecord
	require.NoError(t, db.First(&record).Error)
	assert.Equal(t, 1200, record.CommissionRateBasisPoints)
	assert.Equal(t, int64(1200), record.CommissionAmountMinor)
}

func TestCreateCommissionForTopUpHonorsMasterSwitch(t *testing.T) {
	db := useCommissionTestDB(t)
	previousEnabled := common.CommissionEnabled
	t.Cleanup(func() {
		common.CommissionEnabled = previousEnabled
	})
	common.CommissionEnabled = false

	require.NoError(t, CreateCommissionForTopUpTx(db, &TopUp{Id: 1, UserId: 1, PaymentAmountMinor: 10000}))

	var count int64
	require.NoError(t, db.Model(&CommissionRecord{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCreateCommissionForTopUpCustomZeroDoesNotFallBackToGlobal(t *testing.T) {
	db := useCommissionTestDB(t)
	previousEnabled := common.CommissionEnabled
	previousGlobalEnabled := common.CommissionGlobalRateEnabled
	previousGlobalRate := common.CommissionGlobalRateBasisPoints
	t.Cleanup(func() {
		common.CommissionEnabled = previousEnabled
		common.CommissionGlobalRateEnabled = previousGlobalEnabled
		common.CommissionGlobalRateBasisPoints = previousGlobalRate
	})
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = true
	common.CommissionGlobalRateBasisPoints = 1200

	agent := User{Username: "agent-zero", AffCode: "agent-zero", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{Username: "descendant-zero", AffCode: "descendant-zero", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id}
	require.NoError(t, db.Create(&descendant).Error)
	require.NoError(t, db.Create(&CommissionAgent{UserId: agent.Id, UseCustomRate: true, RateBasisPoints: 0}).Error)
	topUp := TopUp{UserId: descendant.Id, TradeNo: "zero-rate", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&topUp).Error)

	require.NoError(t, CreateCommissionForTopUpTx(db, &topUp))

	var count int64
	require.NoError(t, db.Model(&CommissionRecord{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCreateCommissionForTopUpWithoutEnabledRateCreatesNoRecord(t *testing.T) {
	db := useCommissionTestDB(t)
	previousEnabled := common.CommissionEnabled
	previousGlobalEnabled := common.CommissionGlobalRateEnabled
	t.Cleanup(func() {
		common.CommissionEnabled = previousEnabled
		common.CommissionGlobalRateEnabled = previousGlobalEnabled
	})
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = false

	agent := User{Username: "agent-no-rate", AffCode: "agent-no-rate", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{Username: "descendant-no-rate", AffCode: "descendant-no-rate", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id}
	require.NoError(t, db.Create(&descendant).Error)
	topUp := TopUp{UserId: descendant.Id, TradeNo: "no-rate", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&topUp).Error)

	require.NoError(t, CreateCommissionForTopUpTx(db, &topUp))

	var count int64
	require.NoError(t, db.Model(&CommissionRecord{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestSetCommissionAgentRejectsRootUser(t *testing.T) {
	db := useCommissionTestDB(t)
	root := User{Username: "commission-root", AffCode: "commission-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&root).Error)

	err := SetCommissionAgent(db, root.Id, true, 1000)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
}

func TestCommissionHierarchyRejectsThirdParticipantLevel(t *testing.T) {
	db := useCommissionTestDB(t)
	agentA := User{Username: "agent-a", AffCode: "agent-a", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agentA).Error)
	agentB := User{Username: "agent-b", AffCode: "agent-b", Role: common.RoleAgentUser, Status: common.UserStatusEnabled, ParentUserId: agentA.Id}
	require.NoError(t, db.Create(&agentB).Error)
	candidate := User{Username: "agent-c", AffCode: "agent-c", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agentB.Id}
	require.NoError(t, db.Create(&candidate).Error)

	err := ValidateCommissionAgentRoleChange(db, candidate.Id)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "最多包含两级代理")
}

func TestSummarizeCommissionRecordsSeparatesCurrencies(t *testing.T) {
	db := useCommissionTestDB(t)
	agent := User{Username: "summary-agent", AffCode: "summary-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{Username: "summary-descendant", AffCode: "summary-descendant", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id}
	require.NoError(t, db.Create(&descendant).Error)
	require.NoError(t, db.Create(&CommissionRecord{AgentUserId: agent.Id, DescendantUserId: descendant.Id, TopUpId: 1, PaymentCurrency: "USD", PaymentAmountMinor: 1000, CommissionAmountMinor: 100}).Error)
	require.NoError(t, db.Create(&CommissionRecord{AgentUserId: agent.Id, DescendantUserId: descendant.Id, TopUpId: 2, PaymentCurrency: "USD", PaymentAmountMinor: 2000, CommissionAmountMinor: 200}).Error)
	require.NoError(t, db.Create(&CommissionRecord{AgentUserId: agent.Id, DescendantUserId: descendant.Id, TopUpId: 3, PaymentCurrency: "CNY", PaymentAmountMinor: 5000, CommissionAmountMinor: 500}).Error)

	summaries, err := SummarizeCommissionRecords("", 0, 0, 0)

	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.Equal(t, CommissionCurrencySummary{Currency: "CNY", PaymentAmountMinor: 5000, CommissionMinor: 500, RecordCount: 1}, summaries[0])
	assert.Equal(t, CommissionCurrencySummary{Currency: "USD", PaymentAmountMinor: 3000, CommissionMinor: 300, RecordCount: 2}, summaries[1])
}

func TestValidateCommissionOptionValues(t *testing.T) {
	for _, value := range []string{"-1", "10001", "invalid"} {
		assert.Error(t, validateOptionValue("CommissionGlobalRateBasisPoints", value))
	}
	require.NoError(t, validateOptionValue("CommissionGlobalRateBasisPoints", "0"))
	require.NoError(t, validateOptionValue("CommissionGlobalRateBasisPoints", "10000"))
	assert.Error(t, validateOptionValue("CommissionEnabled", "1"))
	require.NoError(t, validateOptionValue("CommissionEnabled", "true"))
}
