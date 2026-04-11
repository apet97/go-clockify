package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// schemaValidationEnforcement is a minimal Enforcement that defers to a
// caller-supplied hook for BeforeCall. We use it here (rather than the
// real enforcement.Pipeline) to avoid an import cycle with
// internal/enforcement — this is the protocol-core test for the -32602
// translation, not the enforcement-layer test.
type schemaValidationEnforcement struct {
	beforeCall func(ctx context.Context, name string, args map[string]any, hints ToolHints, schema map[string]any, lookup func(string) (ToolHandler, bool)) (any, func(), error)
}

func (e *schemaValidationEnforcement) FilterTool(string, ToolHints) bool { return true }
func (e *schemaValidationEnforcement) BeforeCall(ctx context.Context, name string, args map[string]any, hints ToolHints, schema map[string]any, lookup func(string) (ToolHandler, bool)) (any, func(), error) {
	return e.beforeCall(ctx, name, args, hints, schema, lookup)
}
func (e *schemaValidationEnforcement) AfterCall(r any) (any, error) { return r, nil }

// TestToolsCallInvalidParamsErrorMapsTo32602 asserts that when
// Enforcement.BeforeCall returns *InvalidParamsError, the tools/call
// dispatch renders a JSON-RPC -32602 response with the JSON Pointer
// under error.data.pointer. The error must NOT be wrapped in the
// isError:true tool-error envelope.
func TestToolsCallInvalidParamsErrorMapsTo32602(t *testing.T) {
	enf := &schemaValidationEnforcement{
		beforeCall: func(_ context.Context, _ string, _ map[string]any, _ ToolHints, _ map[string]any, _ func(string) (ToolHandler, bool)) (any, func(), error) {
			return nil, nil, &InvalidParamsError{Pointer: "/start", Message: "missing required property"}
		},
	}
	handler := func(context.Context, map[string]any) (any, error) {
		t.Fatal("handler should not be invoked when BeforeCall rejects")
		return nil, nil
	}
	srv := NewServer("test", []ToolDescriptor{
		{
			Tool: Tool{
				Name:        "probe",
				Description: "probe",
				InputSchema: map[string]any{"type": "object", "required": []string{"start"}},
			},
			Handler:      handler,
			ReadOnlyHint: true,
		},
	}, enf, nil)
	// Mark initialized so tools/call passes the init guard.
	srv.initialized.Store(true)

	req := Request{
		JSONRPC: "2.0",
		ID:      float64(42),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "probe",
			"arguments": map[string]any{},
		},
	}
	resp := srv.handle(context.Background(), req)

	if resp.Error == nil {
		t.Fatalf("expected JSON-RPC error, got result: %#v", resp.Result)
	}
	if resp.Error.Code != -32602 {
		t.Errorf("code = %d, want -32602", resp.Error.Code)
	}
	if resp.Error.Data == nil {
		t.Fatalf("error.data is nil; want map with pointer")
	}
	if got, _ := resp.Error.Data["pointer"].(string); got != "/start" {
		t.Errorf("error.data.pointer = %q, want /start", got)
	}
	if resp.Result != nil {
		t.Errorf("resp.Result should be nil on -32602 path, got %#v", resp.Result)
	}
}

// TestToolsCallInvalidParamsErrorSurvivesJSON asserts the wire JSON shape
// of an InvalidParamsError response matches RFC-compliant JSON-RPC: a
// top-level error object with code -32602 and a data field carrying the
// pointer.
func TestToolsCallInvalidParamsErrorSurvivesJSON(t *testing.T) {
	enf := &schemaValidationEnforcement{
		beforeCall: func(_ context.Context, _ string, _ map[string]any, _ ToolHints, _ map[string]any, _ func(string) (ToolHandler, bool)) (any, func(), error) {
			return nil, nil, &InvalidParamsError{Pointer: "/billable", Message: "expected boolean, got string"}
		},
	}
	srv := NewServer("test", []ToolDescriptor{
		{
			Tool:         Tool{Name: "probe", InputSchema: map[string]any{"type": "object"}},
			Handler:      func(context.Context, map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: true,
		},
	}, enf, nil)
	srv.initialized.Store(true)

	req := Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}},
	}
	resp := srv.handle(context.Background(), req)

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Error *RPCError `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Error == nil {
		t.Fatalf("no error on wire; raw=%s", string(raw))
	}
	if decoded.Error.Code != -32602 {
		t.Errorf("wire code = %d, want -32602", decoded.Error.Code)
	}
	ptr, _ := decoded.Error.Data["pointer"].(string)
	if ptr != "/billable" {
		t.Errorf("wire pointer = %q, want /billable", ptr)
	}
}

// TestToolsCallNonSchemaErrorStillIsErrorEnvelope asserts that non-
// InvalidParamsError errors from BeforeCall (policy denial, rate limit,
// generic tool errors) continue to surface as the isError:true envelope,
// not as JSON-RPC errors. This is the regression guard for the
// errors.As branch point in server.go.
func TestToolsCallNonSchemaErrorStillIsErrorEnvelope(t *testing.T) {
	enf := &schemaValidationEnforcement{
		beforeCall: func(_ context.Context, _ string, _ map[string]any, _ ToolHints, _ map[string]any, _ func(string) (ToolHandler, bool)) (any, func(), error) {
			return nil, nil, errors.New("tool blocked by policy: standard does not allow probe")
		},
	}
	srv := NewServer("test", []ToolDescriptor{
		{
			Tool:         Tool{Name: "probe", InputSchema: map[string]any{"type": "object"}},
			Handler:      func(context.Context, map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: true,
		},
	}, enf, nil)
	srv.initialized.Store(true)

	req := Request{
		JSONRPC: "2.0",
		ID:      float64(7),
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}},
	}
	resp := srv.handle(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("expected isError:true envelope, got JSON-RPC error: %+v", resp.Error)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result not a map: %#v", resp.Result)
	}
	if b, _ := m["isError"].(bool); !b {
		t.Errorf("isError should be true; got %#v", m["isError"])
	}
}
