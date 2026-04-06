package truncate

import (
	"encoding/json"
	"os"
	"strconv"
	"unicode/utf8"
)

// Config holds token-budget truncation settings.
type Config struct {
	TokenBudget int
	Enabled     bool
}

// ConfigFromEnv reads CLOCKIFY_TOKEN_BUDGET from the environment.
// Default is 8000. Setting it to 0 disables truncation.
func ConfigFromEnv() Config {
	budget := 8000
	if v := os.Getenv("CLOCKIFY_TOKEN_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			budget = n
		}
	}
	return Config{
		TokenBudget: budget,
		Enabled:     budget > 0,
	}
}

// Truncate applies progressive truncation stages to keep the token estimate
// within the configured budget. It returns the (possibly modified) value and
// a boolean indicating whether any truncation occurred.
func (c Config) Truncate(v any) (any, bool) {
	if !c.Enabled {
		return v, false
	}

	originalEstimate := estimateTokens(v)
	if originalEstimate <= c.TokenBudget {
		return v, false
	}

	result := v

	// Stage 1: strip nil values from maps
	result = stripNulls(result)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget)
		return result, true
	}

	// Stage 2: strip empty slices and empty maps
	result = stripEmpties(result)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget)
		return result, true
	}

	// Stage 3: truncate long strings to 200 chars
	result = truncateStrings(result, 200)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget)
		return result, true
	}

	// Stage 4: halve arrays up to 8 iterations
	for i := 0; i < 8; i++ {
		result = reduceArrays(result)
		if estimateTokens(result) <= c.TokenBudget {
			break
		}
	}

	result = injectMetadata(result, originalEstimate, c.TokenBudget)
	return result, true
}

// injectMetadata adds _truncation metadata if the result is a map[string]any.
func injectMetadata(v any, originalEstimate, budget int) any {
	if m, ok := v.(map[string]any); ok {
		m["_truncation"] = map[string]any{
			"truncated":               true,
			"original_token_estimate": originalEstimate,
			"budget":                  budget,
		}
		return m
	}
	return v
}

// estimateTokens marshals v to JSON and estimates tokens as ceil(len/4).
func estimateTokens(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	n := len(b)
	return (n + 3) / 4 // ceiling division
}

// stripNulls recursively removes nil values from maps, preserving pagination keys.
func stripNulls(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			if child == nil && !isPaginationKey(k) {
				continue
			}
			result[k] = stripNulls(child)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, child := range val {
			result[i] = stripNulls(child)
		}
		return result
	default:
		return v
	}
}

// stripEmpties recursively removes empty slices and empty maps, preserving pagination keys.
func stripEmpties(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			child = stripEmpties(child)
			if !isPaginationKey(k) {
				if isEmptyCollection(child) {
					continue
				}
			}
			result[k] = child
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, child := range val {
			result[i] = stripEmpties(child)
		}
		return result
	default:
		return v
	}
}

// isEmptyCollection returns true for empty []any or empty map[string]any.
func isEmptyCollection(v any) bool {
	switch val := v.(type) {
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

// truncateStrings recursively truncates all string values to maxLen characters.
func truncateStrings(v any, maxLen int) any {
	switch val := v.(type) {
	case string:
		return truncateStringUTF8(val, maxLen)
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			result[k] = truncateStrings(child, maxLen)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, child := range val {
			result[i] = truncateStrings(child, maxLen)
		}
		return result
	default:
		return v
	}
}

// reduceArrays recursively halves any slice, appending a truncation indicator.
func reduceArrays(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			result[k] = reduceArrays(child)
		}
		return result
	case []any:
		if len(val) <= 1 {
			// Don't halve single-element or empty arrays, but recurse into children
			result := make([]any, len(val))
			for i, child := range val {
				result[i] = reduceArrays(child)
			}
			return result
		}
		half := len(val) / 2
		remaining := len(val) - half
		result := make([]any, half+1)
		for i := 0; i < half; i++ {
			result[i] = reduceArrays(val[i])
		}
		result[half] = map[string]any{
			"_truncated": true,
			"_remaining": remaining,
		}
		return result
	default:
		return v
	}
}

// truncateStringUTF8 truncates s to at most maxLen characters at a valid
// UTF-8 boundary and appends "..." if truncated.
func truncateStringUTF8(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	count := 0
	for i := range s {
		if count == maxLen {
			return s[:i] + "..."
		}
		count++
	}
	return s
}

// isPaginationKey returns true for keys that should be preserved even if nil/empty.
func isPaginationKey(key string) bool {
	switch key {
	case "count", "page", "page_size", "has_more":
		return true
	default:
		return false
	}
}
