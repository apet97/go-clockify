package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolsCallResultSizeGuardTruncatesArrayData(t *testing.T) {
	rows := make([]map[string]any, 0, 12)
	for i := range 12 {
		rows = append(rows, map[string]any{
			"id":   i,
			"note": strings.Repeat("x", 120),
		})
	}
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "large_list", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{
				"ok":     true,
				"action": "large_list",
				"data":   rows,
				"meta":   map[string]any{"page": 1, "pageSize": 12},
			}, nil
		},
	}})
	server.MaxToolResultBytes = 700
	server.MarkInitialized(SupportedProtocolVersions[0], "test", "0")

	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "large_list", "arguments": map[string]any{}},
	})
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}
	text := toolCallResultText(t, resp)
	if len([]byte(text)) > server.MaxToolResultBytes {
		t.Fatalf("guarded tool result is %d bytes, want <= %d\n%s", len([]byte(text)), server.MaxToolResultBytes, text)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("unmarshal result text: %v", err)
	}
	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data type=%T want array", env["data"])
	}
	if len(data) == 0 || len(data) >= len(rows) {
		t.Fatalf("data length=%d, want truncated non-empty list below %d", len(data), len(rows))
	}
	meta, ok := env["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing: %+v", env)
	}
	if meta["truncated"] != true || meta["size_capped"] != true {
		t.Fatalf("missing truncation flags: %+v", meta)
	}
	if got := int(meta["returned"].(float64)); got != len(data) {
		t.Fatalf("meta.returned=%d want %d", got, len(data))
	}
	if got := int(meta["dropped"].(float64)); got != len(rows)-len(data) {
		t.Fatalf("meta.dropped=%d want %d", got, len(rows)-len(data))
	}
	if hint, _ := meta["next_hint"].(string); !strings.Contains(hint, "size-capped") {
		t.Fatalf("next_hint should explain size cap, got %q", hint)
	}
}

func TestToolsCallResultSizeGuardReturnsToolErrorForNonTruncatableData(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "large_object", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{
				"ok":     true,
				"action": "large_object",
				"data": map[string]any{
					"note": strings.Repeat("x", 2_000),
				},
			}, nil
		},
	}})
	server.MaxToolResultBytes = 500
	server.MarkInitialized(SupportedProtocolVersions[0], "test", "0")

	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "large_object", "arguments": map[string]any{}},
	})
	if resp.Error != nil {
		t.Fatalf("result-size guard should return a tool envelope, got RPC error: %+v", resp.Error)
	}
	text := toolCallResultText(t, resp)
	var env map[string]any
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("unmarshal result text: %v", err)
	}
	if env["ok"] != false {
		t.Fatalf("ok=%v want false: %+v", env["ok"], env)
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("error missing: %+v", env)
	}
	if errObj["code"] != "result_too_large" {
		t.Fatalf("error.code=%v want result_too_large", errObj["code"])
	}
	recovery, ok := env["recovery"].(map[string]any)
	if !ok || recovery["hint"] == "" {
		t.Fatalf("missing recovery hint: %+v", env)
	}
}

func TestResultTooLargeEnvelopeSchemaConformance(t *testing.T) {
	env := resultTooLargeEnvelope("clockify_projects_list", 100_000, 50_000)
	if env["ok"] != false {
		t.Fatalf("ok=%v want false", env["ok"])
	}
	if env["action"] != "clockify_projects_list" {
		t.Fatalf("action=%v", env["action"])
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok || errObj["code"] != "result_too_large" {
		t.Fatalf("error object = %+v", env["error"])
	}
	recovery, ok := env["recovery"].(map[string]any)
	if !ok || recovery["hint"] == "" || recovery["retryable"] != true {
		t.Fatalf("recovery object = %+v", env["recovery"])
	}
	meta, ok := env["meta"].(map[string]any)
	if !ok || meta["size_capped"] != true || meta["truncationSuccessful"] != false {
		t.Fatalf("meta object = %+v", env["meta"])
	}
}

func TestToolsCallResultSizeGuardLeavesUnderBudgetResponseByteIdentical(t *testing.T) {
	descriptor := ToolDescriptor{
		Tool: Tool{Name: "small_result", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{
				"ok":     true,
				"action": "small_result",
				"data":   []map[string]any{{"id": "one", "name": "small"}},
				"meta":   map[string]any{"count": 1},
			}, nil
		},
	}
	withoutGuard := NewServer("test", []ToolDescriptor{descriptor})
	withoutGuard.MarkInitialized(SupportedProtocolVersions[0], "test", "0")
	withGuard := NewServer("test", []ToolDescriptor{descriptor})
	withGuard.MaxToolResultBytes = 50_000
	withGuard.MarkInitialized(SupportedProtocolVersions[0], "test", "0")

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "small_result", "arguments": map[string]any{}},
	}
	plain := toolCallResultText(t, withoutGuard.handle(context.Background(), req))
	guarded := toolCallResultText(t, withGuard.handle(context.Background(), req))
	if guarded != plain {
		t.Fatalf("under-budget response changed\nplain:   %s\nguarded: %s", plain, guarded)
	}
}

func BenchmarkShrinkListToBudgetLargeList(b *testing.B) {
	items := make([]any, 5_000)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": "row", "note": strings.Repeat("x", 24)}
	}
	budget := 4_000

	b.ReportAllocs()
	for b.Loop() {
		envelope := map[string]any{
			"ok":     true,
			"action": "large_list",
			"data":   append([]any(nil), items...),
			"meta":   map[string]any{"page": 1, "pageSize": len(items)},
		}
		data := envelope["data"].([]any)
		if !shrinkListToBudget(envelope, data, budget, func(kept []any) {
			envelope["data"] = kept
		}) {
			b.Fatal("shrinkListToBudget returned false")
		}
	}
}

func BenchmarkResultSizeGuardLargeList(b *testing.B) {
	items := make([]map[string]any, 5_000)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": "row", "note": strings.Repeat("x", 24)}
	}
	const budget = 4_000

	b.ReportAllocs()
	for b.Loop() {
		envelope := map[string]any{
			"ok":     true,
			"action": "large_list",
			"data":   append([]map[string]any(nil), items...),
			"meta":   map[string]any{"page": 1, "pageSize": len(items)},
		}
		env := toolResultEnvelopeGuarded("large_list", envelope, budget)
		if env["structuredContent"] == nil {
			b.Fatal("expected a structured truncation envelope")
		}
	}
}

func TestToolResultEnvelopeGuardedUnderBudgetReusesMarshal(t *testing.T) {
	result := map[string]any{
		"ok":     true,
		"action": "small_result",
		"data":   []map[string]any{{"id": "one", "name": "small"}},
	}
	plain := toolResultEnvelope(result)
	guarded := toolResultEnvelopeGuarded("small_result", result, 50_000)
	disabled := toolResultEnvelopeGuarded("small_result", result, 0)

	for label, env := range map[string]map[string]any{"guarded": guarded, "disabled": disabled} {
		plainText := envelopeContentText(t, plain)
		gotText := envelopeContentText(t, env)
		if gotText != plainText {
			t.Fatalf("%s content[0].text diverged from the unguarded envelope\nplain:   %s\n%s: %s", label, plainText, label, gotText)
		}
		if _, ok := env["structuredContent"]; !ok {
			t.Fatalf("%s envelope dropped structuredContent for an object result", label)
		}
	}
}

func envelopeContentText(t *testing.T, env map[string]any) string {
	t.Helper()
	content, ok := env["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("envelope content type=%T value=%+v", env["content"], env["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("envelope content[0].text type=%T", content[0]["text"])
	}
	return text
}

func toolCallResultText(t *testing.T, resp Response) string {
	t.Helper()
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type=%T want map", resp.Result)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content type=%T value=%+v", result["content"], result["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok || text == "" {
		t.Fatalf("content text missing: %+v", content[0])
	}
	return text
}
