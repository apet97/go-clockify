package truncate

import (
	"fmt"
	"math/rand/v2"
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

	// Verify the trimmed portion is valid UTF-8. Range over string substitutes
	// \uFFFD for invalid sequences, so a U+FFFD rune means truncation split a
	// multi-byte character.
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

	rep := &TruncationReport{}
	result := reduceArrays(data, "", rep).(map[string]any)
	arr := result["items"].([]any)

	// Should be halved to 10 with NO sentinel element.
	if len(arr) != 10 {
		t.Errorf("expected 10 elements (halved, no sentinel), got %d", len(arr))
	}

	// Every surviving element must still be a homogeneous {id: int} object —
	// no _truncated/_remaining sentinel leaked into the array.
	for i, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			t.Fatalf("element %d: expected map[string]any, got %T", i, el)
		}
		if _, ok := m["id"]; !ok {
			t.Fatalf("element %d: missing 'id' key, got %v", i, m)
		}
		if _, bad := m["_truncated"]; bad {
			t.Fatalf("element %d: sentinel _truncated leaked into array", i)
		}
		if _, bad := m["_remaining"]; bad {
			t.Fatalf("element %d: sentinel _remaining leaked into array", i)
		}
	}
}

func TestReduceArrays_ReportPopulated(t *testing.T) {
	items := make([]any, 20)
	for i := range items {
		items[i] = map[string]any{"id": i}
	}
	data := map[string]any{"items": items}

	rep := &TruncationReport{}
	_ = reduceArrays(data, "", rep)

	if len(rep.ArrayReductions) != 1 {
		t.Fatalf("expected 1 array reduction, got %d", len(rep.ArrayReductions))
	}
	r := rep.ArrayReductions[0]
	if r.Path != "items" {
		t.Errorf("expected path=items, got %q", r.Path)
	}
	if r.OriginalLen != 20 {
		t.Errorf("expected OriginalLen=20, got %d", r.OriginalLen)
	}
	if r.NewLen != 10 {
		t.Errorf("expected NewLen=10, got %d", r.NewLen)
	}
	if r.Removed != 10 {
		t.Errorf("expected Removed=10, got %d", r.Removed)
	}
}

func TestReduceArrays_Homogeneous(t *testing.T) {
	// Seed an envelope-shaped map with a 20-element homogeneous array.
	items := make([]any, 20)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": fmt.Sprintf("item-%d", i)}
	}
	data := map[string]any{
		"ok":     true,
		"action": "list",
		"items":  items,
	}

	// Very small budget forces stage 4 (array reduction) to run.
	cfg := Config{TokenBudget: 1, Enabled: true}
	out, truncated := cfg.Truncate(data)
	if !truncated {
		t.Fatal("expected truncation at budget=1")
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	arr, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("expected items to be []any, got %T", m["items"])
	}
	if len(arr) >= 20 {
		t.Fatalf("expected items to shrink, got len=%d", len(arr))
	}
	for i, el := range arr {
		em, ok := el.(map[string]any)
		if !ok {
			t.Fatalf("element %d: expected map[string]any, got %T", i, el)
		}
		if _, ok := em["id"]; !ok {
			t.Fatalf("element %d: missing 'id' key", i)
		}
		if _, bad := em["_truncated"]; bad {
			t.Fatalf("element %d: sentinel _truncated leaked in", i)
		}
		if _, bad := em["_remaining"]; bad {
			t.Fatalf("element %d: sentinel _remaining leaked in", i)
		}
	}
}

// TestTruncate_PropertyArraysStayHomogeneous generates random nested maps with
// homogeneous arrays of {id:int} objects and verifies that after Truncate at
// low budgets, every array in the output still contains only map elements.
func TestTruncate_PropertyArraysStayHomogeneous(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 1337))
	for iter := 0; iter < 60; iter++ {
		data := genNestedMap(r, 3)
		budget := 1 + r.IntN(20)
		cfg := Config{TokenBudget: budget, Enabled: true}
		out, _ := cfg.Truncate(data)
		assertArraysHomogeneous(t, out, fmt.Sprintf("iter=%d root", iter))
	}
}

// genNestedMap builds a map with up to depth levels containing some
// homogeneous arrays of {id:int} objects plus scalar fields.
func genNestedMap(r *rand.Rand, depth int) map[string]any {
	m := map[string]any{
		"id":   r.IntN(1000),
		"name": strings.Repeat("n", 5+r.IntN(10)),
	}
	// Homogeneous object array.
	arrLen := 2 + r.IntN(10)
	arr := make([]any, arrLen)
	for i := 0; i < arrLen; i++ {
		arr[i] = map[string]any{"id": i, "text": strings.Repeat("x", 5+r.IntN(10))}
	}
	m["items"] = arr
	if depth > 0 && r.IntN(2) == 0 {
		m["child"] = genNestedMap(r, depth-1)
	}
	return m
}

// assertArraysHomogeneous walks v and fails if any []any contains a
// non-map element (sentinels would be maps too, but per contract we
// also reject objects with _truncated/_remaining keys).
func assertArraysHomogeneous(t *testing.T, v any, path string) {
	t.Helper()
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if k == "_truncation" {
				// Metadata lives here; skip its internal shape.
				continue
			}
			assertArraysHomogeneous(t, child, path+"."+k)
		}
	case []any:
		for i, el := range val {
			em, ok := el.(map[string]any)
			if !ok {
				t.Fatalf("%s[%d]: expected map element, got %T", path, i, el)
			}
			if _, bad := em["_truncated"]; bad {
				t.Fatalf("%s[%d]: array sentinel _truncated leaked", path, i)
			}
			if _, bad := em["_remaining"]; bad {
				t.Fatalf("%s[%d]: array sentinel _remaining leaked", path, i)
			}
			assertArraysHomogeneous(t, el, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

func TestMetadataInjected(t *testing.T) {
	// Use a very small budget to force truncation through stage 4 so
	// array_reductions is populated.
	cfg := Config{TokenBudget: 1, Enabled: true}
	items := make([]any, 100)
	for i := range items {
		items[i] = map[string]any{"id": i, "text": strings.Repeat("x", 20)}
	}
	data := map[string]any{
		"items": items,
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
	reductions, ok := meta["array_reductions"].([]ArrayReduction)
	if !ok {
		t.Fatalf("expected array_reductions []ArrayReduction in metadata, got %T", meta["array_reductions"])
	}
	if len(reductions) == 0 {
		t.Fatal("expected at least one array reduction recorded")
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
