package truncate

import (
	"os"
	"strings"
	"testing"
)

func TestConfigFromEnvDefault(t *testing.T) {
	os.Unsetenv("CLOCKIFY_TOKEN_BUDGET")
	cfg := ConfigFromEnv()
	if cfg.TokenBudget != 8000 {
		t.Errorf("expected default budget 8000, got %d", cfg.TokenBudget)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true for default budget")
	}
}

func TestConfigFromEnvDisabled(t *testing.T) {
	t.Setenv("CLOCKIFY_TOKEN_BUDGET", "0")
	cfg := ConfigFromEnv()
	if cfg.TokenBudget != 0 {
		t.Errorf("expected budget 0, got %d", cfg.TokenBudget)
	}
	if cfg.Enabled {
		t.Error("expected Enabled=false when budget is 0")
	}
}

func TestPassthrough(t *testing.T) {
	cfg := Config{TokenBudget: 8000, Enabled: true}
	data := map[string]any{"name": "small"}
	result, truncated := cfg.Truncate(data)
	if truncated {
		t.Error("expected no truncation for small data")
	}
	m := result.(map[string]any)
	if m["name"] != "small" {
		t.Errorf("expected name=small, got %v", m["name"])
	}
}

func TestDisabledPassthrough(t *testing.T) {
	cfg := Config{TokenBudget: 0, Enabled: false}
	data := map[string]any{"big": strings.Repeat("x", 100000)}
	result, truncated := cfg.Truncate(data)
	if truncated {
		t.Error("expected no truncation when disabled")
	}
	m := result.(map[string]any)
	if len(m["big"].(string)) != 100000 {
		t.Error("data should be unchanged when disabled")
	}
}

func TestStripNulls(t *testing.T) {
	data := map[string]any{
		"name":    "test",
		"removed": nil,
		"nested": map[string]any{
			"keep":   "yes",
			"remove": nil,
		},
	}
	result := stripNulls(data).(map[string]any)
	if _, ok := result["removed"]; ok {
		t.Error("expected nil value to be removed")
	}
	nested := result["nested"].(map[string]any)
	if _, ok := nested["remove"]; ok {
		t.Error("expected nested nil value to be removed")
	}
	if nested["keep"] != "yes" {
		t.Error("expected non-nil value to be preserved")
	}
}

func TestStripNullsPreservesPagination(t *testing.T) {
	data := map[string]any{
		"count":     nil,
		"page":      nil,
		"page_size": nil,
		"has_more":  nil,
		"other":     nil,
	}
	result := stripNulls(data).(map[string]any)

	for _, key := range []string{"count", "page", "page_size", "has_more"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected pagination key %q to be preserved even if nil", key)
		}
	}
	if _, ok := result["other"]; ok {
		t.Error("expected non-pagination nil key to be removed")
	}
}

func TestStripEmpties(t *testing.T) {
	data := map[string]any{
		"name":      "test",
		"emptyList": []any{},
		"emptyMap":  map[string]any{},
		"full":      []any{"a"},
	}
	result := stripEmpties(data).(map[string]any)
	if _, ok := result["emptyList"]; ok {
		t.Error("expected empty slice to be removed")
	}
	if _, ok := result["emptyMap"]; ok {
		t.Error("expected empty map to be removed")
	}
	if result["name"] != "test" {
		t.Error("expected non-empty value to be preserved")
	}
	if result["full"] == nil {
		t.Error("expected non-empty slice to be preserved")
	}
}

func TestTruncateStrings(t *testing.T) {
	longStr := strings.Repeat("a", 300)
	data := map[string]any{
		"short": "hello",
		"long":  longStr,
	}
	result := truncateStrings(data, 200).(map[string]any)
	if result["short"] != "hello" {
		t.Error("short string should be unchanged")
	}
	truncated := result["long"].(string)
	if len(truncated) > 203 { // 200 chars + "..."
		t.Errorf("expected truncated string <=203 bytes, got %d", len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("expected truncated string to end with ...")
	}
}

func TestTruncateStringsUTF8(t *testing.T) {
	// Each emoji is a multi-byte rune. Build a string of 250 emoji runes.
	emoji := "🎉"
	longStr := strings.Repeat(emoji, 250) // 250 runes, 1000 bytes
	result := truncateStringUTF8(longStr, 200)

	// Verify it ends with "..."
	if !strings.HasSuffix(result, "...") {
		t.Error("expected ... suffix")
	}

	// Strip "..." and count runes — should be exactly 200
	trimmed := strings.TrimSuffix(result, "...")
	runeCount := 0
	for range trimmed {
		runeCount++
	}
	if runeCount != 200 {
		t.Errorf("expected 200 runes before ..., got %d", runeCount)
	}

	// Verify the trimmed portion is valid UTF-8
	for i := 0; i < len(trimmed); {
		r, size := rune(trimmed[i]), 0
		for trimmed[i]&(0x80>>(size)) != 0 {
			size++
		}
		_ = r
		// Just use range which already validates
		break
	}
	// Simpler: range over string never produces invalid runes if input is valid
	for _, r := range trimmed {
		if r == '\uFFFD' {
			t.Fatal("truncation split a multi-byte character")
		}
	}
}

func TestReduceArrays(t *testing.T) {
	items := make([]any, 20)
	for i := range items {
		items[i] = map[string]any{"id": i}
	}
	data := map[string]any{"items": items}

	result := reduceArrays(data).(map[string]any)
	arr := result["items"].([]any)

	// Should be halved: 10 items + 1 truncation indicator = 11
	if len(arr) != 11 {
		t.Errorf("expected 11 elements (10 + indicator), got %d", len(arr))
	}

	// Last element should be the truncation indicator
	indicator := arr[len(arr)-1].(map[string]any)
	if indicator["_truncated"] != true {
		t.Error("expected _truncated=true in indicator")
	}
	if indicator["_remaining"] != 10 {
		t.Errorf("expected _remaining=10, got %v", indicator["_remaining"])
	}
}

func TestMetadataInjected(t *testing.T) {
	// Use a very small budget to force truncation
	cfg := Config{TokenBudget: 1, Enabled: true}
	data := map[string]any{
		"items": make([]any, 100),
		"big":   strings.Repeat("x", 1000),
	}
	result, truncated := cfg.Truncate(data)
	if !truncated {
		t.Fatal("expected truncation")
	}
	m := result.(map[string]any)
	meta, ok := m["_truncation"].(map[string]any)
	if !ok {
		t.Fatal("expected _truncation metadata map")
	}
	if meta["truncated"] != true {
		t.Error("expected truncated=true in metadata")
	}
	if meta["budget"] != 1 {
		t.Error("expected budget=1 in metadata")
	}
	if meta["original_token_estimate"] == nil {
		t.Error("expected original_token_estimate in metadata")
	}
}

func TestEstimateTokens(t *testing.T) {
	// 16-byte JSON → ceil(16/4) = 4 tokens
	data := map[string]any{"a": "b"}
	est := estimateTokens(data)
	if est <= 0 {
		t.Errorf("expected positive estimate, got %d", est)
	}
	// Sanity: a simple {"a":"b"} is 9 bytes → ceil(9/4)=3
	if est > 10 {
		t.Errorf("estimate unreasonably high for tiny data: %d", est)
	}

	// Larger data should produce larger estimate
	big := map[string]any{"data": strings.Repeat("x", 400)}
	bigEst := estimateTokens(big)
	if bigEst <= est {
		t.Error("larger data should have larger token estimate")
	}
}
