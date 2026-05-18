package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
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
	})

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

func TestProgressTokenValidString(t *testing.T) {
	TestToolsCallExtractsProgressToken(t)
}

func TestProgressTokenValidInteger(t *testing.T) {
	var gotToken any
	server := progressTokenTestServer(func(ctx context.Context, _ map[string]any) (any, error) {
		gotToken, _ = ProgressTokenFromContext(ctx)
		return map[string]any{"ok": true}, nil
	})

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"probe","arguments":{},"_meta":{"progressToken":7}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	if got, ok := gotToken.(json.Number); !ok || got.String() != "7" {
		t.Fatalf("token: %#v", gotToken)
	}
}

func TestProgressTokenInvalidFloat(t *testing.T) {
	resp := dispatchProgressTokenRaw(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"probe","arguments":{},"_meta":{"progressToken":1.5}}}`)
	assertInvalidProgressToken(t, resp)
}

func TestProgressTokenInvalidBool(t *testing.T) {
	server := progressTokenTestServer(nil)
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}, "_meta": map[string]any{"progressToken": true}},
	})
	assertInvalidProgressToken(t, resp)
}

func TestProgressTokenInvalidObject(t *testing.T) {
	server := progressTokenTestServer(nil)
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}, "_meta": map[string]any{"progressToken": map[string]any{}}},
	})
	assertInvalidProgressToken(t, resp)
}

func TestProgressTokenNullRejected(t *testing.T) {
	server := progressTokenTestServer(nil)
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "probe", "arguments": map[string]any{}, "_meta": map[string]any{"progressToken": nil}},
	})
	assertInvalidProgressToken(t, resp)
}

func TestProgressTokenDuplicateRejected(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := progressTokenTestServer(func(ctx context.Context, _ map[string]any) (any, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return map[string]any{"ok": true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	firstDone := make(chan Response, 1)
	go func() {
		firstDone <- server.handle(context.Background(), progressTokenRequest(1, "dup"))
	}()
	<-started

	resp := server.handle(context.Background(), progressTokenRequest(2, "dup"))
	assertInvalidProgressToken(t, resp)
	if !strings.Contains(resp.Error.Message, "already in use") {
		t.Fatalf("message: %s", resp.Error.Message)
	}

	close(release)
	if resp := <-firstDone; resp.Error != nil {
		t.Fatalf("first call error: %+v", resp.Error)
	}

	resp = server.handle(context.Background(), progressTokenRequest(3, "dup"))
	if resp.Error != nil {
		t.Fatalf("token should be reusable after completion: %+v", resp.Error)
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
	})

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

func progressTokenTestServer(handler ToolHandler) *Server {
	if handler == nil {
		handler = func(context.Context, map[string]any) (any, error) {
			return map[string]any{"ok": true}, nil
		}
	}
	server := NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "probe", Description: "x", InputSchema: map[string]any{"type": "object"}},
		Handler: handler,
	}})
	server.initialized.Store(true)
	return server
}

func progressTokenRequest(id any, token any) Request {
	return Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "probe",
			"arguments": map[string]any{},
			"_meta":     map[string]any{"progressToken": token},
		},
	}
}

func dispatchProgressTokenRaw(t *testing.T, msg string) Response {
	t.Helper()
	server := progressTokenTestServer(nil)
	raw, err := server.DispatchMessage(context.Background(), []byte(msg))
	if err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertInvalidProgressToken(t *testing.T, resp Response) {
	t.Helper()
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("expected -32602, got %+v", resp.Error)
	}
}
