package model

import (
	"fmt"
	"math"
	"strconv"
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

func TestCommissionHierarchyAllowsUnlimitedParticipantLevels(t *testing.T) {
	db := useCommissionTestDB(t)
	agentA := User{Username: "agent-a", AffCode: "agent-a", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agentA).Error)
	agentB := User{Username: "agent-b", AffCode: "agent-b", Role: common.RoleAgentUser, Status: common.UserStatusEnabled, ParentUserId: agentA.Id}
	require.NoError(t, db.Create(&agentB).Error)
	candidate := User{Username: "agent-c", AffCode: "agent-c", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agentB.Id}
	require.NoError(t, db.Create(&candidate).Error)

	err := ValidateCommissionAgentRoleChange(db, candidate.Id)

	require.NoError(t, err)
}

func TestSetCommissionParentPersistsRelationshipWithoutChangingInviter(t *testing.T) {
	db := useCommissionTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	agent := User{Username: "manual-parent-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	target := User{
		Username:  "manual-parent-target",
		AffCode:   "manual-parent-target",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		InviterId: agent.Id,
	}
	require.NoError(t, db.Create(&target).Error)

	require.NoError(t, SetCommissionParent(target.Id, agent.Id))

	var updated User
	require.NoError(t, db.First(&updated, target.Id).Error)
	assert.Equal(t, agent.Id, updated.ParentUserId)
	assert.Equal(t, agent.Id, updated.InviterId)

	require.NoError(t, SetCommissionParent(target.Id, 0))
	require.NoError(t, db.First(&updated, target.Id).Error)
	assert.Zero(t, updated.ParentUserId)
	assert.Equal(t, agent.Id, updated.InviterId)
}

func TestSetCommissionParentRejectsInvalidParentAndPreservesExistingParent(t *testing.T) {
	db := useCommissionTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	agent := User{Username: "valid-parent", AffCode: "valid-parent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	disabledAgent := User{Username: "disabled-parent", AffCode: "disabled-parent", Role: common.RoleAgentUser, Status: common.UserStatusDisabled}
	regularUser := User{Username: "regular-parent", AffCode: "regular-parent", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	root := User{Username: "root-parent", AffCode: "root-parent", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	target := User{Username: "parent-target", AffCode: "parent-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	for _, user := range []*User{&agent, &disabledAgent, &regularUser, &root, &target} {
		require.NoError(t, db.Create(user).Error)
	}
	require.NoError(t, SetCommissionParent(target.Id, agent.Id))

	for _, parentId := range []int{target.Id, 999999, disabledAgent.Id, regularUser.Id, root.Id} {
		err := SetCommissionParent(target.Id, parentId)
		require.Error(t, err)
	}

	var unchanged User
	require.NoError(t, db.First(&unchanged, target.Id).Error)
	assert.Equal(t, agent.Id, unchanged.ParentUserId)
}

func TestSetCommissionParentRejectsCyclesAndAllowsUnlimitedLevels(t *testing.T) {
	db := useCommissionTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	agentA := User{Username: "cycle-a", AffCode: "cycle-a", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	agentB := User{Username: "cycle-b", AffCode: "cycle-b", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	agentC := User{Username: "cycle-c", AffCode: "cycle-c", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	for _, user := range []*User{&agentA, &agentB, &agentC} {
		require.NoError(t, db.Create(user).Error)
	}

	require.NoError(t, SetCommissionParent(agentB.Id, agentA.Id))
	require.Error(t, SetCommissionParent(agentA.Id, agentB.Id))
	require.NoError(t, SetCommissionParent(agentC.Id, agentB.Id))

	var unchanged User
	require.NoError(t, db.First(&unchanged, agentC.Id).Error)
	assert.Equal(t, agentB.Id, unchanged.ParentUserId)
}

func TestSetCommissionParentChangesOnlyFutureCommissionOwnership(t *testing.T) {
	db := useCommissionTestDB(t)
	previousRedisEnabled := common.RedisEnabled
	previousCommissionEnabled := common.CommissionEnabled
	previousGlobalEnabled := common.CommissionGlobalRateEnabled
	previousGlobalRate := common.CommissionGlobalRateBasisPoints
	common.RedisEnabled = false
	common.CommissionEnabled = true
	common.CommissionGlobalRateEnabled = false
	common.CommissionGlobalRateBasisPoints = 0
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.CommissionEnabled = previousCommissionEnabled
		common.CommissionGlobalRateEnabled = previousGlobalEnabled
		common.CommissionGlobalRateBasisPoints = previousGlobalRate
	})

	agentA := User{Username: "future-a", AffCode: "future-a", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	agentB := User{Username: "future-b", AffCode: "future-b", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	descendant := User{Username: "future-descendant", AffCode: "future-descendant", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	for _, user := range []*User{&agentA, &agentB, &descendant} {
		require.NoError(t, db.Create(user).Error)
	}
	require.NoError(t, db.Create(&CommissionAgent{UserId: agentA.Id, UseCustomRate: true, RateBasisPoints: 1000}).Error)
	require.NoError(t, db.Create(&CommissionAgent{UserId: agentB.Id, UseCustomRate: true, RateBasisPoints: 2000}).Error)

	require.NoError(t, SetCommissionParent(descendant.Id, agentA.Id))
	firstTopUp := TopUp{UserId: descendant.Id, TradeNo: "before-parent-change", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&firstTopUp).Error)
	require.NoError(t, CreateCommissionForTopUpTx(db, &firstTopUp))

	require.NoError(t, SetCommissionParent(descendant.Id, agentB.Id))
	secondTopUp := TopUp{UserId: descendant.Id, TradeNo: "after-parent-change", PaymentAmountMinor: 10000, PaymentCurrency: "USD"}
	require.NoError(t, db.Create(&secondTopUp).Error)
	require.NoError(t, CreateCommissionForTopUpTx(db, &secondTopUp))

	var records []CommissionRecord
	require.NoError(t, db.Order("top_up_id ASC").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, agentA.Id, records[0].AgentUserId)
	assert.Equal(t, agentB.Id, records[1].AgentUserId)
	assert.Equal(t, 1000, records[0].CommissionRateBasisPoints)
	assert.Equal(t, 2000, records[1].CommissionRateBasisPoints)
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

func TestCommissionDateFiltersUseExclusiveEndTime(t *testing.T) {
	db := useCommissionTestDB(t)
	agent := User{Username: "date-agent", Email: "date-agent@example.com", AffCode: "date-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled, CreatedAt: 100}
	require.NoError(t, db.Create(&agent).Error)
	firstReferral := User{Username: "date-first", Email: "date-first@example.com", AffCode: "date-first", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id, CreatedAt: 100}
	secondReferral := User{Username: "date-second", Email: "date-second@example.com", AffCode: "date-second", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id, CreatedAt: 200}
	require.NoError(t, db.Create(&firstReferral).Error)
	require.NoError(t, db.Create(&secondReferral).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId: agent.Id, DescendantUserId: firstReferral.Id, TopUpId: 101,
		PaymentCurrency: "CNY", PaymentAmountMinor: 10000, CommissionAmountMinor: 3000, CreatedAt: 100,
	}).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId: agent.Id, DescendantUserId: secondReferral.Id, TopUpId: 102,
		PaymentCurrency: "CNY", PaymentAmountMinor: 20000, CommissionAmountMinor: 6000, CreatedAt: 200,
	}).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}

	records, total, err := ListCommissionRecords("", 150, 250, agent.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	assert.Equal(t, int64(200), records[0].CreatedAt)

	referrals, total, err := ListCommissionReferrals(agent.Id, "", 150, 250, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, referrals, 1)
	assert.Equal(t, secondReferral.Id, referrals[0].Id)
}

func TestCommissionSelfListsOrderNewestFirst(t *testing.T) {
	db := useCommissionTestDB(t)
	agent := User{Username: "order-agent", AffCode: "order-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&agent).Error)
	olderReferral := User{Username: "order-older", AffCode: "order-older", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id, CreatedAt: 100}
	newerReferral := User{Username: "order-newer", AffCode: "order-newer", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: agent.Id, CreatedAt: 200}
	require.NoError(t, db.Create(&olderReferral).Error)
	require.NoError(t, db.Create(&newerReferral).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId: agent.Id, DescendantUserId: olderReferral.Id, TopUpId: 201,
		PaymentCurrency: "CNY", PaymentAmountMinor: 10000, CommissionAmountMinor: 3000, CreatedAt: 100,
	}).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId: agent.Id, DescendantUserId: newerReferral.Id, TopUpId: 202,
		PaymentCurrency: "CNY", PaymentAmountMinor: 20000, CommissionAmountMinor: 6000, CreatedAt: 200,
	}).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}

	records, total, err := ListCommissionRecords("", 0, 0, agent.Id, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, records, 2)
	assert.Equal(t, int64(200), records[0].CreatedAt)
	assert.Equal(t, int64(100), records[1].CreatedAt)

	referrals, total, err := ListCommissionReferrals(agent.Id, "", 0, 0, pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, referrals, 2)
	assert.Equal(t, newerReferral.Id, referrals[0].Id)
	assert.Equal(t, olderReferral.Id, referrals[1].Id)
}

func TestCommissionRecordSearchMatchesAgentAndDescendantIdentity(t *testing.T) {
	db := useCommissionTestDB(t)
	agent := User{
		Username:    "search-agent",
		DisplayName: "Primary Agent",
		Email:       "agent-search@example.com",
		AffCode:     "search-agent",
		Role:        common.RoleAgentUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&agent).Error)
	descendant := User{
		Username:     "search-descendant",
		DisplayName:  "Primary Descendant",
		Email:        "descendant-search@example.com",
		AffCode:      "search-descendant",
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		ParentUserId: agent.Id,
	}
	require.NoError(t, db.Create(&descendant).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           agent.Id,
		DescendantUserId:      descendant.Id,
		TopUpId:               1,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    10000,
		CommissionAmountMinor: 1000,
	}).Error)

	otherAgent := User{Username: "other-agent", Email: "other-agent@example.com", AffCode: "other-agent", Role: common.RoleAgentUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&otherAgent).Error)
	otherDescendant := User{Username: "other-descendant", Email: "other-descendant@example.com", AffCode: "other-descendant", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, ParentUserId: otherAgent.Id}
	require.NoError(t, db.Create(&otherDescendant).Error)
	require.NoError(t, db.Create(&CommissionRecord{
		AgentUserId:           otherAgent.Id,
		DescendantUserId:      otherDescendant.Id,
		TopUpId:               2,
		PaymentCurrency:       "USD",
		PaymentAmountMinor:    20000,
		CommissionAmountMinor: 2000,
	}).Error)

	tests := []struct {
		name    string
		keyword string
	}{
		{name: "agent email", keyword: "AGENT-SEARCH@EXAMPLE.COM"},
		{name: "agent username", keyword: "search-agent"},
		{name: "agent UID", keyword: strconv.Itoa(agent.Id)},
		{name: "descendant email", keyword: "DESCENDANT-SEARCH@EXAMPLE.COM"},
		{name: "descendant username", keyword: "search-descendant"},
		{name: "descendant UID", keyword: strconv.Itoa(descendant.Id)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, total, err := ListCommissionRecords(tt.keyword, 0, 0, 0, &common.PageInfo{Page: 1, PageSize: 10})
			require.NoError(t, err)
			require.Len(t, records, 1)
			assert.Equal(t, int64(1), total)
			assert.Equal(t, agent.Id, records[0].AgentUserId)
			assert.Equal(t, descendant.Id, records[0].DescendantUserId)

			summaries, err := SummarizeCommissionRecords(tt.keyword, 0, 0, 0)
			require.NoError(t, err)
			require.Len(t, summaries, 1)
			assert.Equal(t, CommissionCurrencySummary{
				Currency:           "USD",
				PaymentAmountMinor: 10000,
				CommissionMinor:    1000,
				RecordCount:        1,
			}, summaries[0])
		})
	}
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
