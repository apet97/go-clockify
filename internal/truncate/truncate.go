package truncate

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"unicode/utf8"
)

// Config holds token-budget truncation settings.
type Config struct {
	TokenBudget int
	Enabled     bool
}

// TruncationReport captures side-channel metadata about reductions applied
// during truncation. It is populated by reduceArrays and surfaced via the
// _truncation field in the resulting map, keeping arrays homogeneous.
type TruncationReport struct {
	ArrayReductions []ArrayReduction `json:"array_reductions,omitempty"`
}

// ArrayReduction records a single array halving: its path, original length,
// new (kept) length, and the number of removed elements.
type ArrayReduction struct {
	Path        string `json:"path"`
	OriginalLen int    `json:"original_len"`
	NewLen      int    `json:"new_len"`
	Removed     int    `json:"removed"`
}

// maxArrayReductions caps the TruncationReport growth so pathological
// deeply-nested inputs can't blow up metadata size.
const maxArrayReductions = 50

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

	rep := &TruncationReport{}
	result := v

	// Stage 1: strip nil values from maps
	result = stripNulls(result)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget, rep)
		return result, true
	}

	// Stage 2: strip empty slices and empty maps
	result = stripEmpties(result)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget, rep)
		return result, true
	}

	// Stage 3: truncate long strings to 200 chars
	result = truncateStrings(result, 200)
	if estimateTokens(result) <= c.TokenBudget {
		result = injectMetadata(result, originalEstimate, c.TokenBudget, rep)
		return result, true
	}

	// Stage 4: halve arrays up to 8 iterations
	for i := 0; i < 8; i++ {
		result = reduceArrays(result, "", rep)
		if estimateTokens(result) <= c.TokenBudget {
			break
		}
	}

	result = injectMetadata(result, originalEstimate, c.TokenBudget, rep)
	return result, true
}

// injectMetadata adds _truncation metadata if the result is a map[string]any.
// Non-map values are returned unchanged (the JSON roundtrip in AfterCall
// normalizes typed tool envelopes to maps before truncation runs, so in
// practice the top level is always a map here).
func injectMetadata(v any, originalEstimate, budget int, rep *TruncationReport) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	info := map[string]any{
		"truncated":               true,
		"original_token_estimate": originalEstimate,
		"budget":                  budget,
	}
	if rep != nil && len(rep.ArrayReductions) > 0 {
		info["array_reductions"] = rep.ArrayReductions
	}
	m["_truncation"] = info
	return m
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

// reduceArrays recursively halves any slice, preserving homogeneity (no
// sentinel objects are appended). Per-reduction details are recorded in rep
// so the information can be exposed via _truncation metadata.
func reduceArrays(v any, path string, rep *TruncationReport) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			result[k] = reduceArrays(child, joinPath(path, k), rep)
		}
		return result
	case []any:
		if len(val) <= 1 {
			// Don't halve single-element or empty arrays, but recurse into children.
			result := make([]any, len(val))
			for i, child := range val {
				result[i] = reduceArrays(child, fmt.Sprintf("%s[%d]", path, i), rep)
			}
			return result
		}
		half := len(val) / 2
		removed := len(val) - half
		result := make([]any, half)
		for i := 0; i < half; i++ {
			result[i] = reduceArrays(val[i], fmt.Sprintf("%s[%d]", path, i), rep)
		}
		if rep != nil && len(rep.ArrayReductions) < maxArrayReductions {
			rep.ArrayReductions = append(rep.ArrayReductions, ArrayReduction{
				Path:        path,
				OriginalLen: len(val),
				NewLen:      half,
				Removed:     removed,
			})
		}
		return result
	default:
		return v
	}
}

// joinPath appends key to parent using dot notation.
func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
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
