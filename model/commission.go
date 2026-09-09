package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const CommissionRateMaxBasisPoints = 10000

type CommissionAgent struct {
	UserId          int   `json:"user_id" gorm:"primaryKey;column:user_id"`
	UseCustomRate   bool  `json:"use_custom_rate"`
	RateBasisPoints int   `json:"rate_basis_points"`
	CreatedAt       int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt       int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type CommissionRecord struct {
	Id                        int    `json:"id"`
	AgentUserId               int    `json:"agent_user_id" gorm:"index"`
	DescendantUserId          int    `json:"descendant_user_id" gorm:"index"`
	TopUpId                   int    `json:"top_up_id" gorm:"uniqueIndex"`
	TradeNo                   string `json:"trade_no" gorm:"type:varchar(255);index"`
	PaymentAmountMinor        int64  `json:"payment_amount_minor"`
	PaymentCurrency           string `json:"payment_currency" gorm:"type:varchar(16);index"`
	CommissionRateBasisPoints int    `json:"commission_rate_basis_points"`
	CommissionAmountMinor     int64  `json:"commission_amount_minor"`
	CreatedAt                 int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

type CommissionRecordView struct {
	CommissionRecord
	AgentUsername      string `json:"agent_username"`
	AgentDisplayName   string `json:"agent_display_name"`
	AgentEmail         string `json:"agent_email"`
	DescendantUsername string `json:"descendant_username"`
	DescendantName     string `json:"descendant_name"`
	DescendantEmail    string `json:"descendant_email"`
}

type CommissionAgentView struct {
	UserId                   int    `json:"user_id"`
	Username                 string `json:"username"`
	DisplayName              string `json:"display_name"`
	Email                    string `json:"email"`
	Role                     int    `json:"role"`
	Status                   int    `json:"status"`
	ParentUserId             int    `json:"parent_user_id"`
	UseCustomRate            bool   `json:"use_custom_rate"`
	RateBasisPoints          int    `json:"rate_basis_points"`
	EffectiveRateBasisPoints int    `json:"effective_rate_basis_points" gorm:"-"`
	RateSource               string `json:"rate_source" gorm:"-"`
}

type CommissionCurrencySummary struct {
	Currency           string `json:"currency"`
	PaymentAmountMinor int64  `json:"payment_amount_minor"`
	CommissionMinor    int64  `json:"commission_amount_minor"`
	RecordCount        int64  `json:"record_count"`
}

func EffectiveCommissionRate(agent *CommissionAgent) (int, string) {
	if agent != nil && agent.UseCustomRate && agent.RateBasisPoints >= 0 && agent.RateBasisPoints <= CommissionRateMaxBasisPoints {
		return agent.RateBasisPoints, "agent"
	}
	if common.CommissionGlobalRateEnabled && common.CommissionGlobalRateBasisPoints >= 0 && common.CommissionGlobalRateBasisPoints <= CommissionRateMaxBasisPoints {
		return common.CommissionGlobalRateBasisPoints, "global"
	}
	return 0, "none"
}

func ResolveCommissionParentByAffCode(affCode string) int {
	if strings.TrimSpace(affCode) == "" {
		return 0
	}
	var user User
	if err := DB.Where("aff_code = ?", affCode).First(&user).Error; err != nil {
		return 0
	}
	if !IsCommissionParticipant(&user) {
		return 0
	}
	return user.Id
}

func normalizeCommissionCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return "USD"
	}
	return currency
}

func IsCommissionParticipant(user *User) bool {
	return user != nil && user.Role != common.RoleRootUser &&
		(user.Role == common.RoleAgentUser || user.Role == common.RoleAdminUser) &&
		user.Status == common.UserStatusEnabled
}

func GetCommissionAgent(userId int) (*CommissionAgent, error) {
	var agent CommissionAgent
	err := DB.Where("user_id = ?", userId).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func GetCommissionEligibility(userId int) (*User, *CommissionAgent, bool) {
	var user User
	if DB.Select("id", "role", "status", "parent_user_id", "username", "display_name", "email", "aff_code").First(&user, userId).Error != nil {
		return nil, nil, false
	}
	agent, err := GetCommissionAgent(userId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		agent = nil
	}
	return &user, agent, IsCommissionParticipant(&user)
}

func validateCommissionParticipantHierarchy(tx *gorm.DB, userId int) error {
	var relation struct {
		ParentUserId      int
		GrandparentUserId int
	}
	if err := tx.Table("users AS target").
		Select("target.parent_user_id, parent.parent_user_id AS grandparent_user_id").
		Joins("LEFT JOIN users AS parent ON parent.id = target.parent_user_id").
		Where("target.id = ?", userId).Scan(&relation).Error; err != nil {
		return err
	}
	participantRoles := []int{common.RoleAgentUser, common.RoleAdminUser}
	parentIsParticipant := false
	if relation.ParentUserId > 0 {
		var parent User
		if tx.Select("id", "role").First(&parent, relation.ParentUserId).Error == nil {
			parentIsParticipant = parent.Role == common.RoleAgentUser || parent.Role == common.RoleAdminUser
		}
	}
	if parentIsParticipant && relation.GrandparentUserId > 0 {
		var grandparent User
		if tx.Select("id", "role").First(&grandparent, relation.GrandparentUserId).Error == nil &&
			(grandparent.Role == common.RoleAgentUser || grandparent.Role == common.RoleAdminUser) {
			return errors.New("一条反佣链最多包含两级代理")
		}
	}
	var childCount int64
	if err := tx.Model(&User{}).Where("parent_user_id = ? AND role IN ?", userId, participantRoles).Count(&childCount).Error; err != nil {
		return err
	}
	if parentIsParticipant && childCount > 0 {
		return errors.New("已有代理上级的用户不能再拥有下级代理")
	}

	// A target with an existing participant child may be promoted only when
	// that child has no participant descendants; otherwise the change would
	// create a three-level commission chain.
	if childCount > 0 {
		var nestedChildCount int64
		if err := tx.Table("users AS child").
			Joins("JOIN users AS grandchild ON grandchild.parent_user_id = child.id").
			Where("child.parent_user_id = ? AND child.role IN ? AND grandchild.role IN ?", userId, participantRoles, participantRoles).
			Count(&nestedChildCount).Error; err != nil {
			return err
		}
		if nestedChildCount > 0 {
			return errors.New("一条反佣链最多包含两级代理")
		}
	}
	return nil
}

func ValidateCommissionAgentRoleChange(tx *gorm.DB, userId int) error {
	var user User
	if err := tx.Select("id", "role").First(&user, userId).Error; err != nil {
		return err
	}
	if user.Role == common.RoleRootUser {
		return errors.New("root 用户不能参与反佣")
	}
	return validateCommissionParticipantHierarchy(tx, userId)
}

func ValidateCommissionAgentRole(tx *gorm.DB, userId int, role int) error {
	if role != common.RoleAgentUser && role != common.RoleAdminUser {
		return nil
	}
	return ValidateCommissionAgentRoleChange(tx, userId)
}

func DisableCommissionAgent(tx *gorm.DB, userId int) error {
	if !tx.Migrator().HasTable(&CommissionAgent{}) {
		return nil
	}
	return tx.Model(&CommissionAgent{}).Where("user_id = ?", userId).
		Updates(map[string]interface{}{"use_custom_rate": false, "rate_basis_points": 0}).Error
}

func SetCommissionAgent(tx *gorm.DB, userId int, useCustomRate bool, rateBasisPoints int) error {
	var user User
	if err := tx.Select("id", "role", "status").First(&user, userId).Error; err != nil {
		return err
	}
	if user.Role == common.RoleRootUser {
		return errors.New("root 用户不能参与反佣")
	}
	if user.Role != common.RoleAgentUser && user.Role != common.RoleAdminUser {
		return errors.New("只有代理或管理员可以参与反佣")
	}
	if rateBasisPoints < 0 || rateBasisPoints > CommissionRateMaxBasisPoints {
		return fmt.Errorf("反佣比例必须在 0 到 %d 基点之间", CommissionRateMaxBasisPoints)
	}
	if !useCustomRate {
		rateBasisPoints = 0
	}
	if useCustomRate {
		if err := validateCommissionParticipantHierarchy(tx, userId); err != nil {
			return err
		}
	}
	return tx.Save(&CommissionAgent{UserId: userId, UseCustomRate: useCustomRate, RateBasisPoints: rateBasisPoints}).Error
}

func CreateCommissionForTopUpTx(tx *gorm.DB, topUp *TopUp) error {
	if !common.CommissionEnabled || topUp == nil || topUp.PaymentAmountMinor <= 0 {
		return nil
	}
	var descendant User
	if err := tx.Select("id", "parent_user_id").First(&descendant, topUp.UserId).Error; err != nil {
		return err
	}
	if descendant.ParentUserId == 0 {
		return nil
	}
	var agentUser User
	if err := tx.Select("id", "role", "status", "parent_user_id").First(&agentUser, descendant.ParentUserId).Error; err != nil {
		return nil
	}
	if !IsCommissionParticipant(&agentUser) {
		return nil
	}
	var agentConfig CommissionAgent
	var agent *CommissionAgent
	if err := tx.Where("user_id = ?", agentUser.Id).First(&agentConfig).Error; err == nil {
		agent = &agentConfig
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	currency := normalizeCommissionCurrency(topUp.PaymentCurrency)
	rateBasisPoints, _ := EffectiveCommissionRate(agent)
	amount := calculateCommissionAmount(topUp.PaymentAmountMinor, rateBasisPoints)
	if amount <= 0 {
		return nil
	}
	record := &CommissionRecord{
		AgentUserId: agentUser.Id, DescendantUserId: descendant.Id, TopUpId: topUp.Id,
		TradeNo: topUp.TradeNo, PaymentAmountMinor: topUp.PaymentAmountMinor, PaymentCurrency: currency,
		CommissionRateBasisPoints: rateBasisPoints, CommissionAmountMinor: amount,
		CreatedAt: common.GetTimestamp(),
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "top_up_id"}},
		DoNothing: true,
	}).Create(record).Error
}

func calculateCommissionAmount(paymentAmountMinor int64, rateBasisPoints int) int64 {
	if paymentAmountMinor <= 0 || rateBasisPoints <= 0 || rateBasisPoints > CommissionRateMaxBasisPoints {
		return 0
	}
	rate := int64(rateBasisPoints)
	base := int64(CommissionRateMaxBasisPoints)
	return paymentAmountMinor/base*rate + paymentAmountMinor%base*rate/base
}

func CreateCommissionForTopUpWithFallbackTx(tx *gorm.DB, topUp *TopUp, defaultCurrency string) error {
	updates := make(map[string]interface{})
	if topUp.PaymentAmountMinor == 0 {
		topUp.PaymentAmountMinor = PaymentAmountToMinor(topUp.Money, defaultCurrency)
		updates["payment_amount_minor"] = topUp.PaymentAmountMinor
	}
	if strings.TrimSpace(topUp.PaymentCurrency) == "" {
		topUp.PaymentCurrency = normalizeCommissionCurrency(defaultCurrency)
		updates["payment_currency"] = topUp.PaymentCurrency
	}
	if len(updates) > 0 {
		if err := tx.Model(topUp).Updates(updates).Error; err != nil {
			return err
		}
	}
	return CreateCommissionForTopUpTx(tx, topUp)
}

func PaymentAmountToMinor(amount float64, currency string) int64 {
	currency = normalizeCommissionCurrency(currency)
	multiplier := int64(100)
	switch currency {
	case "IDR", "JPY", "KRW", "VND":
		multiplier = 1
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0
	}
	scaled := amount * float64(multiplier)
	if scaled >= float64(math.MaxInt64)-1 {
		return math.MaxInt64
	}
	return int64(scaled + 0.5)
}

func ListCommissionAgents(keyword string, pageInfo *common.PageInfo) ([]CommissionAgentView, int64, error) {
	query := DB.Model(&User{}).Where("role IN ?", []int{common.RoleAgentUser, common.RoleAdminUser})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ? OR LOWER(display_name) LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like, like, like)
		if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
			query = DB.Model(&User{}).Where("role IN ?", []int{common.RoleAgentUser, common.RoleAdminUser}).
				Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ? OR LOWER(display_name) LIKE ? OR CAST(id AS TEXT) LIKE ?", like, like, like, like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := query.Select("id", "username", "display_name", "email", "role", "status", "parent_user_id").
		Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	agentsByUserId := make(map[int]CommissionAgent, len(ids))
	if len(ids) > 0 {
		var agents []CommissionAgent
		if err := DB.Where("user_id IN ?", ids).Find(&agents).Error; err != nil {
			return nil, 0, err
		}
		for _, agent := range agents {
			agentsByUserId[agent.UserId] = agent
		}
	}
	views := make([]CommissionAgentView, 0, len(users))
	for _, user := range users {
		agent, hasAgentConfig := agentsByUserId[user.Id]
		var agentConfig *CommissionAgent
		if hasAgentConfig {
			agentConfig = &agent
		}
		rate, source := EffectiveCommissionRate(agentConfig)
		useCustomRate := false
		rateBasisPoints := 0
		if agentConfig != nil {
			useCustomRate = agentConfig.UseCustomRate
			rateBasisPoints = agentConfig.RateBasisPoints
		}
		views = append(views, CommissionAgentView{
			UserId: user.Id, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email,
			Role: user.Role, Status: user.Status, ParentUserId: user.ParentUserId,
			UseCustomRate: useCustomRate, RateBasisPoints: rateBasisPoints,
			EffectiveRateBasisPoints: rate, RateSource: source,
		})
	}
	return views, total, nil
}

func commissionRecordQuery(keyword string, startTime int64, endTime int64, agentUserId int) *gorm.DB {
	query := DB.Table("commission_records AS cr").
		Joins("JOIN users AS agent ON agent.id = cr.agent_user_id").
		Joins("JOIN users AS descendant ON descendant.id = cr.descendant_user_id")
	if agentUserId > 0 {
		query = query.Where("cr.agent_user_id = ?", agentUserId)
	}
	if startTime > 0 {
		query = query.Where("cr.created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("cr.created_at <= ?", endTime)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		idCast := "CAST(agent.id AS CHAR)"
		descendantIdCast := "CAST(descendant.id AS CHAR)"
		if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
			idCast = "CAST(agent.id AS TEXT)"
			descendantIdCast = "CAST(descendant.id AS TEXT)"
		}
		query = query.Where("LOWER(agent.username) LIKE ? OR LOWER(agent.email) LIKE ? OR LOWER(agent.display_name) LIKE ? OR "+
			"LOWER(descendant.username) LIKE ? OR LOWER(descendant.email) LIKE ? OR LOWER(descendant.display_name) LIKE ? OR "+
			idCast+" LIKE ? OR "+descendantIdCast+" LIKE ?", like, like, like, like, like, like, like, like)
	}
	return query
}

func ListCommissionRecords(keyword string, startTime int64, endTime int64, agentUserId int, pageInfo *common.PageInfo) ([]CommissionRecordView, int64, error) {
	query := commissionRecordQuery(keyword, startTime, endTime, agentUserId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []CommissionRecordView
	err := query.Select("cr.*, agent.username AS agent_username, agent.display_name AS agent_display_name, agent.email AS agent_email, " +
		"descendant.username AS descendant_username, descendant.display_name AS descendant_name, descendant.email AS descendant_email").
		Order("cr.id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&records).Error
	return records, total, err
}

func SummarizeCommissionRecords(keyword string, startTime int64, endTime int64, agentUserId int) ([]CommissionCurrencySummary, error) {
	var summaries []CommissionCurrencySummary
	err := commissionRecordQuery(keyword, startTime, endTime, agentUserId).
		Select("cr.payment_currency AS currency, SUM(cr.payment_amount_minor) AS payment_amount_minor, " +
			"SUM(cr.commission_amount_minor) AS commission_minor, COUNT(*) AS record_count").
		Group("cr.payment_currency").Order("cr.payment_currency ASC").Scan(&summaries).Error
	return summaries, err
}

func ListCommissionReferrals(agentUserId int, keyword string, pageInfo *common.PageInfo) ([]User, int64, error) {
	query := DB.Model(&User{}).Where("parent_user_id = ?", agentUserId)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ? OR LOWER(display_name) LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	err := query.Select("id", "username", "display_name", "email", "role", "status", "created_at").
		Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error
	return users, total, err
}
