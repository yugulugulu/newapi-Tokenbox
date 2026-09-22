package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSumUsedQuotaSeparatesUsageAndRefundStats(t *testing.T) {
	dsn := fmt.Sprintf("file:log-stat-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	previousLogDB := LOG_DB
	LOG_DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		_ = sqlDB.Close()
	})

	now := time.Now().Unix()
	logs := []Log{
		{CreatedAt: now, Type: LogTypeConsume, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 100, PromptTokens: 10, CompletionTokens: 5},
		{CreatedAt: now, Type: LogTypeConsume, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 200, PromptTokens: 20, CompletionTokens: 10},
		{CreatedAt: now, Type: LogTypeRefund, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 70, PromptTokens: 500, CompletionTokens: 500},
		{CreatedAt: now, Type: LogTypeRefund, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 30},
		{CreatedAt: now, Type: LogTypeManage, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 900, PromptTokens: 900, CompletionTokens: 900},
		{CreatedAt: now, Type: LogTypeConsume, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "other", Quota: 400, PromptTokens: 40, CompletionTokens: 40},
		{CreatedAt: now, Type: LogTypeRefund, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "other", Quota: 200},
		{CreatedAt: now - 120, Type: LogTypeConsume, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 500, PromptTokens: 50, CompletionTokens: 50},
		{CreatedAt: now - 120, Type: LogTypeRefund, Username: "alice", TokenName: "target-token", ModelName: "target-model", ChannelId: 7, Group: "target", Quota: 400},
	}
	require.NoError(t, db.Create(&logs).Error)

	stat, err := SumUsedQuota(LogTypeRefund, now-30, now+30, "target-model", "alice", "target-token", 7, "target")
	require.NoError(t, err)

	assert.Equal(t, 300, stat.Quota)
	assert.Equal(t, 100, stat.RefundQuota)
	assert.Equal(t, 2, stat.Rpm)
	assert.Equal(t, 45, stat.Tpm)
}

func TestSumUsedQuotaSeparatesRedemptionAndDirectTopups(t *testing.T) {
	dsn := fmt.Sprintf("file:log-topup-stat-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &User{}, &TopUp{}, &Redemption{}))

	previousDB := DB
	previousLogDB := LOG_DB
	DB = db
	LOG_DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		_ = sqlDB.Close()
	})

	now := time.Now().Unix()
	users := []User{
		{Username: "alice", AffCode: "alice-stat-code"},
		{Username: "bob", AffCode: "bob-stat-code"},
	}
	require.NoError(t, db.Create(&users).Error)

	quotaPerUnit := int64(common.QuotaPerUnit)
	topUps := []TopUp{
		{UserId: users[0].Id, Amount: 2, TradeNo: "alice-direct", PaymentProvider: PaymentProviderEpay, CompleteTime: now, Status: common.TopUpStatusSuccess},
		{UserId: users[0].Id, Amount: 8, TradeNo: "alice-old", PaymentMethod: "wxpay", CompleteTime: now - 120, Status: common.TopUpStatusSuccess},
		{UserId: users[1].Id, Amount: 4, TradeNo: "bob-direct", PaymentProvider: PaymentProviderEpay, CompleteTime: now, Status: common.TopUpStatusSuccess},
		{UserId: users[0].Id, Amount: 99, TradeNo: "alice-pending", PaymentProvider: PaymentProviderEpay, CompleteTime: now, Status: common.TopUpStatusPending},
		{UserId: users[0].Id, Amount: 0, Money: 20, TradeNo: "subscription-order", CompleteTime: now, Status: common.TopUpStatusSuccess},
	}
	require.NoError(t, db.Create(&topUps).Error)
	redemptions := []Redemption{
		{Key: "00000000000000000000000000000001", Status: common.RedemptionCodeStatusUsed, Quota: int(quotaPerUnit), RedeemedTime: now, UsedUserId: users[0].Id},
		{Key: "00000000000000000000000000000002", Status: common.RedemptionCodeStatusUsed, Quota: int(5 * quotaPerUnit), RedeemedTime: now - 120, UsedUserId: users[0].Id},
		{Key: "00000000000000000000000000000003", Status: common.RedemptionCodeStatusUsed, Quota: int(3 * quotaPerUnit), RedeemedTime: now, UsedUserId: users[1].Id},
	}
	require.NoError(t, db.Create(&redemptions).Error)

	// Old top-up logs had quota=0; statistics must come from durable payment records.
	require.NoError(t, db.Create(&Log{CreatedAt: now, Type: LogTypeTopup, Username: "alice", Quota: 0}).Error)

	stat, err := SumUsedQuota(LogTypeTopup, now-30, now+30, "", "alice", "", 0, "")
	require.NoError(t, err)

	assert.Equal(t, 3*quotaPerUnit, stat.TopupQuota)
	assert.Equal(t, quotaPerUnit, stat.RedemptionQuota)
	assert.Equal(t, 2*quotaPerUnit, stat.DirectTopupQuota)
}

func TestRecordTopupLogStoresQuotaAndSource(t *testing.T) {
	dsn := fmt.Sprintf("file:record-topup-log-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	previousDB := DB
	previousLogDB := LOG_DB
	DB = db
	LOG_DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		_ = sqlDB.Close()
	})

	RecordTopupLog(42, 7500, TopupSourceDirect, "top-up completed", "127.0.0.1", "wxpay", "epay")

	var log Log
	require.NoError(t, db.First(&log).Error)
	assert.Equal(t, LogTypeTopup, log.Type)
	assert.Equal(t, 7500, log.Quota)
	assert.Equal(t, "127.0.0.1", log.Ip)

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, TopupSourceDirect, other["topup_source"])
}
