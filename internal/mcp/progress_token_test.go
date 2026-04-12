package mcp

import (
	"context"
	"testing"
)

func TestToolsCallExtractsProgressToken(t *testing.T) {
	var gotToken any
	var sawToken bool
	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		gotToken, sawToken = ProgressTokenFromContext(ctx)
		return map[string]any{"ok": true}, nil
	}
	server := NewServer("test", []ToolDescriptor{
		{
			Tool:    Tool{Name: "probe", Description: "x", InputSchema: map[string]any{"type": "object"}},
			Handler: handler,
		},
	}, nil, nil)
	server.initialized.Store(true)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "probe",
			"arguments": map[string]any{},
			"_meta":     map[string]any{"progressToken": "abc-123"},
		},
	}
	resp := server.handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	if !sawToken {
		t.Fatal("handler did not observe a progress token on context")
	}
	if gotToken != "abc-123" {
		t.Fatalf("token: %v", gotToken)
	}
}

func TestToolsCallNoTokenWhenMetaAbsent(t *testing.T) {
	var sawToken bool
	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		_, sawToken = ProgressTokenFromContext(ctx)
		return map[string]any{"ok": true}, nil
	}
	server := NewServer("test", []ToolDescriptor{
		{
			Tool:    Tool{Name: "probe", Description: "x", InputSchema: map[string]any{"type": "object"}},
			Handler: handler,
		},
	}, nil, nil)
	server.initialized.Store(true)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}},
	}
	resp := server.handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	if sawToken {
		t.Fatal("handler saw a token when _meta was absent")
	}
}
