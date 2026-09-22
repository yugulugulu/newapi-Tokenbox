package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

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
