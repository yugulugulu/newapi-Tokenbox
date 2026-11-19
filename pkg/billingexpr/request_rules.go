package billingexpr

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
)

// RequestRuleMatch reports one request rule of an expression: a top-level
// `(condition ? multiplier : 1)` factor multiplied onto the base price, which
// is how the pricing editor writes request-rule pricing.
type RequestRuleMatch struct {
	Multiplier float64 `json:"multiplier"`
	Matched    bool    `json:"matched"`
}

// MatchRequestRules evaluates the condition of every request rule of exprStr
// against this request and reports, in source order, which ones matched. It
// only explains a charge for display: the charge itself always comes from
// RunExpr*, and a nil result simply means "nothing to show".
func MatchRequestRules(exprStr string, params TokenParams, request RequestInput) ([]RequestRuleMatch, error) {
	version, body := ParseExprVersion(exprStr)
	tree, err := parser.Parse(body)
	if err != nil {
		return nil, err
	}

	var factors []ast.Node
	collectProductFactors(tree.Node, &factors)
	if len(factors) < 2 {
		return nil, nil
	}

	env := newEvalEnv(params, request, version, &TraceResult{})
	var matches []RequestRuleMatch
	for _, factor := range factors {
		rule, ok := factor.(*ast.ConditionalNode)
		if !ok {
			continue
		}
		multiplier, ok := numberLiteral(rule.Exp1)
		if !ok {
			continue
		}
		if otherwise, ok := numberLiteral(rule.Exp2); !ok || otherwise != 1 {
			continue
		}
		prog, err := expr.Compile(rule.Cond.String(), expr.Env(getCompileEnv(version)), expr.AsBool())
		if err != nil {
			return nil, fmt.Errorf("compile request rule %q: %w", rule.Cond.String(), err)
		}
		out, err := expr.Run(prog, env)
		if err != nil {
			return nil, fmt.Errorf("run request rule %q: %w", rule.Cond.String(), err)
		}
		matched, _ := out.(bool)
		matches = append(matches, RequestRuleMatch{Multiplier: multiplier, Matched: matched})
	}
	return matches, nil
}

// collectProductFactors flattens a chain of top-level multiplications into its
// factors, left to right.
func collectProductFactors(node ast.Node, factors *[]ast.Node) {
	if product, ok := node.(*ast.BinaryNode); ok && product.Operator == "*" {
		collectProductFactors(product.Left, factors)
		collectProductFactors(product.Right, factors)
		return
	}
	*factors = append(*factors, node)
}

func numberLiteral(node ast.Node) (float64, bool) {
	switch n := node.(type) {
	case *ast.IntegerNode:
		return float64(n.Value), true
	case *ast.FloatNode:
		return n.Value, true
	}
	return 0, false
}
