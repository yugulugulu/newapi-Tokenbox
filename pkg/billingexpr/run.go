package billingexpr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/tidwall/gjson"
)

// RunExpr compiles (with cache) and executes an expression string.
// The environment exposes:
//   - p, c             — prompt / completion tokens (auto-excluding separately-priced sub-categories)
//   - len              — total input context length for tier conditions (never reduced by sub-category exclusion)
//   - cr, cc, cc1h     — cache read / creation / creation-1h tokens
//   - tier(name, value) — trace callback that records which tier matched
//   - max, min, abs, ceil, floor — standard math helpers
//
// Returns the resulting float64 quota (before group ratio) and a TraceResult
// with side-channel info captured by tier() during execution.
func RunExpr(exprStr string, params TokenParams) (float64, TraceResult, error) {
	return RunExprWithRequest(exprStr, params, RequestInput{})
}

func RunExprWithRequest(exprStr string, params TokenParams, request RequestInput) (float64, TraceResult, error) {
	prog, err := CompileFromCache(exprStr)
	if err != nil {
		return 0, TraceResult{}, err
	}
	return runProgram(prog, params, request, ExprVersion(exprStr))
}

// RunExprByHash is like RunExpr but accepts a pre-computed hash for the cache
// lookup, avoiding a redundant SHA-256 computation when the caller already
// holds BillingSnapshot.ExprHash.
func RunExprByHash(exprStr, hash string, params TokenParams) (float64, TraceResult, error) {
	return RunExprByHashWithRequest(exprStr, hash, params, RequestInput{})
}

func RunExprByHashWithRequest(exprStr, hash string, params TokenParams, request RequestInput) (float64, TraceResult, error) {
	prog, err := CompileFromCacheByHash(exprStr, hash)
	if err != nil {
		return 0, TraceResult{}, err
	}
	return runProgram(prog, params, request, ExprVersion(exprStr))
}

func runProgram(prog *vm.Program, params TokenParams, request RequestInput, version int) (float64, TraceResult, error) {
	trace := TraceResult{}
	headers := normalizeHeaders(request.Headers)

	env := map[string]interface{}{
		"p":        params.P,
		"c":        params.C,
		"len":      params.Len,
		"cr":       params.CR,
		"cc":       params.CC,
		"cc1h":     params.CC1h,
		"img":      params.Img,
		"img_o":    params.ImgO,
		"ai":       params.AI,
		"ao":       params.AO,
		"quantity": params.Quantity,
		"charge": func(method string, quantity float64, price float64) (float64, error) {
			if version != 2 {
				return 0, fmt.Errorf("charge is only available in v2 expressions")
			}
			if math.IsNaN(quantity) || math.IsInf(quantity, 0) || quantity < 0 {
				return 0, fmt.Errorf("quantity must be finite and non-negative")
			}
			if math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
				return 0, fmt.Errorf("price must be finite and non-negative")
			}
			switch strings.TrimSpace(method) {
			case "per_second":
				trace.BillingMethod = "per_second"
				return quantity * price, nil
			case "per_call":
				trace.BillingMethod = "per_call"
				return price, nil
			default:
				return 0, fmt.Errorf("unsupported charge method %q", method)
			}
		},
		"tier": func(name string, value float64) float64 {
			trace.MatchedTier = name
			trace.Cost = value
			return value
		},
		"header": func(key string) string {
			return headers[strings.ToLower(strings.TrimSpace(key))]
		},
		"param": func(path string) interface{} {
			path = strings.TrimSpace(path)
			if path == "" || len(request.Body) == 0 {
				return nil
			}
			result := gjson.GetBytes(request.Body, path)
			if !result.Exists() {
				return nil
			}
			return result.Value()
		},
		"has": func(source interface{}, substr string) bool {
			if source == nil || substr == "" {
				return false
			}
			return strings.Contains(fmt.Sprint(source), substr)
		},
		"has_media": func(mediaType string) bool {
			if version != 2 {
				return false
			}
			return RequestHasMedia(request.Body, mediaType)
		},
		"hour":    func(tz string) int { return timeInZone(tz).Hour() },
		"minute":  func(tz string) int { return timeInZone(tz).Minute() },
		"weekday": func(tz string) int { return int(timeInZone(tz).Weekday()) },
		"month":   func(tz string) int { return int(timeInZone(tz).Month()) },
		"day":     func(tz string) int { return timeInZone(tz).Day() },
		"max":     math.Max,
		"min":     math.Min,
		"abs":     math.Abs,
		"ceil":    math.Ceil,
		"floor":   math.Floor,
	}

	out, err := expr.Run(prog, env)
	if err != nil {
		return 0, trace, fmt.Errorf("expr run error: %w", err)
	}
	f, ok := out.(float64)
	if !ok {
		return 0, trace, fmt.Errorf("expr result is %T, want float64", out)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, trace, fmt.Errorf("expr result must be finite")
	}
	if version == 2 && f < 0 {
		return 0, trace, fmt.Errorf("v2 expr result cannot be negative")
	}
	return f, trace, nil
}

// RequestHasMedia reports whether top-level content contains a usable media
// entry. Seedance video-input billing intentionally requires a non-empty URL;
// merely sending an empty video_url object must not select the video tier.
func RequestHasMedia(body []byte, mediaType string) bool {
	mediaType = strings.TrimSpace(mediaType)
	if len(body) == 0 || mediaType != "video" {
		return false
	}
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() {
		return false
	}
	for _, item := range content.Array() {
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType != "" && itemType != "video_url" {
			continue
		}
		if strings.TrimSpace(item.Get("video_url.url").String()) != "" {
			return true
		}
	}
	return false
}

func timeInZone(tz string) time.Time {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return time.Now().UTC()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

func normalizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		k := strings.ToLower(strings.TrimSpace(key))
		v := strings.TrimSpace(value)
		if k == "" || v == "" {
			continue
		}
		normalized[k] = v
	}
	return normalized
}
