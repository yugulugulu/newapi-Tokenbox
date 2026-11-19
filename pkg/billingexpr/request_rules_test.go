package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tierWithServiceTierRules is shaped like what the pricing editor saves: a
// parenthesized tier expression followed by request-rule factors.
const tierWithServiceTierRules = `(len <= 272000 ? tier("short", p * 2 + c * 10) : tier("long", p * 4 + c * 15)) * (param("service_tier") == "priority" ? 2 : 1) * (param("service_tier") == "fast" ? 1.5 : 1)`

func TestMatchRequestRules(t *testing.T) {
	tests := []struct {
		name string
		expr string
		body string
		want []billingexpr.RequestRuleMatch
	}{
		{
			name: "first rule matches",
			expr: tierWithServiceTierRules,
			body: `{"service_tier":"priority"}`,
			want: []billingexpr.RequestRuleMatch{{Multiplier: 2, Matched: true}, {Multiplier: 1.5, Matched: false}},
		},
		{
			name: "second rule matches",
			expr: tierWithServiceTierRules,
			body: `{"service_tier":"fast"}`,
			want: []billingexpr.RequestRuleMatch{{Multiplier: 2, Matched: false}, {Multiplier: 1.5, Matched: true}},
		},
		{
			name: "field absent",
			expr: tierWithServiceTierRules,
			body: `{"model":"gpt-6-sol"}`,
			want: []billingexpr.RequestRuleMatch{{Multiplier: 2, Matched: false}, {Multiplier: 1.5, Matched: false}},
		},
		{
			name: "multi-condition rule",
			expr: `tier("base", p * 2) * (param("service_tier") == "priority" && param("stream") == true ? 3 : 1)`,
			body: `{"service_tier":"priority","stream":true}`,
			want: []billingexpr.RequestRuleMatch{{Multiplier: 3, Matched: true}},
		},
		{
			name: "no request rules",
			expr: `len <= 272000 ? tier("short", p * 2 + c * 10) : tier("long", p * 4 + c * 15)`,
			body: `{"service_tier":"priority"}`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := billingexpr.MatchRequestRules(tt.expr, billingexpr.TokenParams{P: 1000, C: 100, Len: 1000}, billingexpr.RequestInput{Body: []byte(tt.body)})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestComputeTieredQuotaReportsRequestRules(t *testing.T) {
	snap := &billingexpr.BillingSnapshot{
		BillingMode:   "tiered_expr",
		ExprString:    tierWithServiceTierRules,
		ExprHash:      billingexpr.ExprHashString(tierWithServiceTierRules),
		GroupRatio:    1,
		EstimatedTier: "short",
		QuotaPerUnit:  500000,
		ExprVersion:   1,
	}
	params := billingexpr.TokenParams{P: 1000, C: 100, Len: 1000}

	plain, err := billingexpr.ComputeTieredQuotaWithRequest(snap, params, billingexpr.RequestInput{Body: []byte(`{}`)})
	require.NoError(t, err)
	priority, err := billingexpr.ComputeTieredQuotaWithRequest(snap, params, billingexpr.RequestInput{Body: []byte(`{"service_tier":"priority"}`)})
	require.NoError(t, err)

	assert.Equal(t, []billingexpr.RequestRuleMatch{{Multiplier: 2, Matched: true}, {Multiplier: 1.5, Matched: false}}, priority.RequestRules)
	assert.Equal(t, []billingexpr.RequestRuleMatch{{Multiplier: 2, Matched: false}, {Multiplier: 1.5, Matched: false}}, plain.RequestRules)
	// The reported rule is the one that actually doubled the charge.
	assert.Equal(t, 2*plain.ActualQuotaAfterGroup, priority.ActualQuotaAfterGroup)
}
