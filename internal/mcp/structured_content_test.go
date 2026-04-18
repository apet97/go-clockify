package mcp

import (
	"context"
	"encoding/json"
	"strings"
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
		"params":  map[string]any{"name": toolName, "arguments": args},
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
	}}, nil, nil)

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
			}}, nil, nil)

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
