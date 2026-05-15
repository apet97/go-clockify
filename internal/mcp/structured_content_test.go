package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/jsonschema"
)

// callToolViaRun drives a single tools/call through Server.Run and returns the
// decoded Response. Reused by every test in this file.
func callToolViaRun(t *testing.T, server *Server, toolName string, args map[string]any) Response {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": toolName},
	}
	if args != nil {
		payload["params"].(map[string]any)["arguments"] = args
	}
	// Initialize first so tools/call is accepted.
	inputLines := []string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	inputLines = append(inputLines, string(raw))

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(strings.Join(inputLines, "\n")), &out); err != nil {
		t.Fatalf("server.Run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 responses, got %d: %s", len(lines), out.String())
	}
	var resp Response
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

// TestToolsCall_StructuredContent_Object verifies the dual-emit contract:
// a tool that returns an object-shaped value must yield both a text content
// block (back-compat) and a structuredContent field that validates against
// the advertised outputSchema.
func TestToolsCall_StructuredContent_Object(t *testing.T) {
	outputSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action", "data"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": "fake_ok"},
			"data": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{"type": "integer"},
				},
				"required": []string{"count"},
			},
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}

	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:         "fake_ok",
			Description:  "returns an envelope",
			InputSchema:  map[string]any{"type": "object"},
			OutputSchema: outputSchema,
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			return map[string]any{
				"ok":     true,
				"action": "fake_ok",
				"data":   map[string]any{"count": 7},
			}, nil
		},
		ReadOnlyHint: true,
	}})

	resp := callToolViaRun(t, server, "fake_ok", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}

	// Back-compat: text content must still exist.
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content array, got %v", result["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["type"] != "text" || first["text"] == "" {
		t.Fatalf("expected text content block, got %+v", first)
	}

	// Structured content must be present and valid per schema.
	structured, ok := result["structuredContent"]
	if !ok {
		t.Fatalf("structuredContent missing: %+v", result)
	}
	if err := jsonschema.Validate(outputSchema, structured); err != nil {
		t.Fatalf("structuredContent fails outputSchema validation: %v (value=%+v)", err, structured)
	}
}

func TestToolsCall_StructuredContent_SuccessEnvelopeGolden(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "contract_ok",
			Description: "returns a stable success envelope",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			return map[string]any{
				"ok":     true,
				"action": "contract_ok",
				"data": map[string]any{
					"count": 2,
					"id":    "entry-1",
				},
			}, nil
		},
		ReadOnlyHint: true,
	}})

	resp := callToolViaRun(t, server, "contract_ok", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}
	result := requireToolResultMap(t, resp)
	assertTextContentMatchesStructuredContent(t, result)
	assertMCPGoldenJSON(t, "tools_call_structured_success.golden.json", result)
}

func TestToolsCall_StructuredContent_RecoverableEnvelopeGolden(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "contract_recovery",
			Description: "returns a recoverable tool envelope",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			return map[string]any{
				"ok":     false,
				"action": "contract_recovery",
				"error": map[string]any{
					"code":    "feature_unavailable",
					"message": "Optional Clockify feature is unavailable in this workspace.",
				},
				"recovery": map[string]any{
					"hint": "Use a workspace plan that enables this feature or try a core time-entry workflow.",
				},
			}, nil
		},
		ReadOnlyHint: true,
	}})

	resp := callToolViaRun(t, server, "contract_recovery", nil)
	if resp.Error != nil {
		t.Fatalf("recoverable tool envelope must not become an RPC error: %+v", resp.Error)
	}
	result := requireToolResultMap(t, resp)
	assertTextContentMatchesStructuredContent(t, result)
	assertMCPGoldenJSON(t, "tools_call_structured_recovery.golden.json", result)
}

func TestToolsCall_StructuredContent_ResultMarshaledOnce(t *testing.T) {
	var marshals atomic.Int32
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "counted",
			Description: "returns a counted envelope",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			return countedStructuredResult{marshals: &marshals}, nil
		},
		ReadOnlyHint: true,
	}})

	resp := callToolViaRun(t, server, "counted", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}
	if result["structuredContent"] == nil {
		t.Fatalf("structuredContent missing: %+v", result)
	}
	if got := marshals.Load(); got != 1 {
		t.Fatalf("tool result marshaled %d times, want 1", got)
	}
}

func requireToolResultMap(t *testing.T, resp Response) map[string]any {
	t.Helper()
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", resp.Result)
	}
	return result
}

func assertTextContentMatchesStructuredContent(t *testing.T, result map[string]any) {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content array, got %v", result["content"])
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("first content item is not an object: %T", content[0])
	}
	text, ok := first["text"].(string)
	if !ok || text == "" {
		t.Fatalf("expected text content block, got %+v", first)
	}
	structured, ok := result["structuredContent"]
	if !ok {
		t.Fatalf("structuredContent missing: %+v", result)
	}
	var decodedText any
	if err := json.Unmarshal([]byte(text), &decodedText); err != nil {
		t.Fatalf("content text is not valid JSON: %v\ntext=%s", err, text)
	}
	if canonicalJSON(t, decodedText) != canonicalJSON(t, structured) {
		t.Fatalf("content text and structuredContent diverged\ntext=%s\nstructured=%s", canonicalJSON(t, decodedText), canonicalJSON(t, structured))
	}
}

func assertMCPGoldenJSON(t *testing.T, name string, got any) {
	t.Helper()
	raw := canonicalJSON(t, got)
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if raw != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s--- want ---\n%s", name, raw, want)
	}
}

func canonicalJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(append(raw, '\n'))
}

type countedStructuredResult struct {
	marshals *atomic.Int32
}

func (r countedStructuredResult) MarshalJSON() ([]byte, error) {
	r.marshals.Add(1)
	return []byte(`{"ok":true,"action":"counted","data":{"count":1}}`), nil
}

// TestToolsCall_StructuredContent_NonObject verifies the spec guardrail:
// structuredContent must be a JSON object, so tools whose result marshals
// to an array/scalar/nil must keep text-only output.
func TestToolsCall_StructuredContent_NonObject(t *testing.T) {
	cases := []struct {
		name   string
		result any
	}{
		{"slice", []any{1, 2, 3}},
		{"string", "hello"},
		{"nil", nil},
		{"int", 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer("test", []ToolDescriptor{{
				Tool: Tool{
					Name:        "fake_scalar",
					Description: "returns a non-object",
					InputSchema: map[string]any{"type": "object"},
				},
				Handler: func(_ context.Context, _ map[string]any) (any, error) {
					return tc.result, nil
				},
				ReadOnlyHint: true,
			}})

			resp := callToolViaRun(t, server, "fake_scalar", nil)
			if resp.Error != nil {
				t.Fatalf("unexpected rpc error: %+v", resp.Error)
			}
			result, _ := resp.Result.(map[string]any)
			if _, hasContent := result["content"]; !hasContent {
				t.Fatalf("expected text content, got %+v", result)
			}
			if _, hasStructured := result["structuredContent"]; hasStructured {
				t.Fatalf("structuredContent must be absent for %s result, got %+v", tc.name, result)
			}
		})
	}
}
