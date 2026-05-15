package mcp

import (
	"context"
	"testing"
)

// TestToolsCallRuntimeSchemaValidationRejectsRequiredField asserts the
// tools/call dispatch rejects a missing required field via runtime
// JSON-schema validation, returning a JSON-RPC -32602 with the offending
// JSON Pointer under error.data.pointer — without invoking the handler.
func TestToolsCallRuntimeSchemaValidationRejectsRequiredField(t *testing.T) {
	handlerRan := false
	srv := NewServer("test", []ToolDescriptor{
		{
			Tool: Tool{
				Name:        "probe",
				Description: "probe",
				InputSchema: map[string]any{
					"type":       "object",
					"required":   []string{"start"},
					"properties": map[string]any{"start": map[string]any{"type": "string"}},
				},
			},
			Handler: func(context.Context, map[string]any) (any, error) {
				handlerRan = true
				return map[string]any{"ok": true}, nil
			},
			ReadOnlyHint: true,
		},
	})
	srv.initialized.Store(true)

	resp := srv.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      float64(42),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "probe",
			"arguments": map[string]any{},
		},
	})

	if handlerRan {
		t.Fatal("handler should not be invoked when runtime schema validation rejects arguments")
	}
	assertInvalidParamsPointer(t, resp, "/start")
}

// TestToolsCallRuntimeSchemaValidationRejectsUnknownProperty asserts the
// dispatch rejects an undeclared property when the schema sets
// additionalProperties:false.
func TestToolsCallRuntimeSchemaValidationRejectsUnknownProperty(t *testing.T) {
	handlerRan := false
	srv := NewServer("test", []ToolDescriptor{
		{
			Tool: Tool{
				Name:        "probe",
				Description: "probe",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]any{"start": map[string]any{"type": "string"}},
				},
			},
			Handler: func(context.Context, map[string]any) (any, error) {
				handlerRan = true
				return map[string]any{"ok": true}, nil
			},
			ReadOnlyHint: true,
		},
	})
	srv.initialized.Store(true)

	resp := srv.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      float64(43),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "probe",
			"arguments": map[string]any{"start": "2026-05-15", "extra": true},
		},
	})

	if handlerRan {
		t.Fatal("handler should not be invoked when runtime schema validation rejects arguments")
	}
	assertInvalidParamsPointer(t, resp, "/extra")
}

func assertInvalidParamsPointer(t *testing.T, resp Response, want string) {
	t.Helper()
	if resp.Error == nil {
		t.Fatalf("expected JSON-RPC error, got result: %#v", resp.Result)
	}
	if resp.Error.Code != -32602 {
		t.Errorf("code = %d, want -32602", resp.Error.Code)
	}
	if resp.Error.Data == nil {
		t.Fatalf("error.data is nil; want map with pointer")
	}
	if got, _ := resp.Error.Data["pointer"].(string); got != want {
		t.Errorf("error.data.pointer = %q, want %s", got, want)
	}
	if resp.Result != nil {
		t.Errorf("resp.Result should be nil on -32602 path, got %#v", resp.Result)
	}
}
