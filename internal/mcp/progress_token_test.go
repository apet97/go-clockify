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

func TestProgressTokenRejectsNonStringNonInteger(t *testing.T) {
	for _, tok := range []any{1.5, float32(2), true, false, nil, []any{1}, map[string]any{"a": 1}} {
		if err := validateProgressToken(tok); err == nil {
			t.Fatalf("validateProgressToken(%#v) = nil, want error", tok)
		}
	}
}

func TestProgressTokenRejectsDuplicateActiveToken(t *testing.T) {
	s := NewServer("test", nil)
	if err := s.registerProgressToken("dup"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := s.registerProgressToken("dup"); err == nil {
		t.Fatal("second register of an active token should fail")
	}
	s.releaseProgressToken("dup")
	if err := s.registerProgressToken("dup"); err != nil {
		t.Fatalf("register after release should succeed: %v", err)
	}
}

func TestProgressNotificationMustIncrease(t *testing.T) {
	s := NewServer("test", nil)
	if err := s.registerProgressToken("inc"); err != nil {
		t.Fatalf("register: %v", err)
	}
	steps := []struct {
		progress float64
		want     bool
	}{
		{1, true},
		{2, true},
		{2, false},
		{1, false},
		{3, true},
	}
	for i, step := range steps {
		if got := s.AllowProgress("inc", step.progress); got != step.want {
			t.Fatalf("step %d: AllowProgress(%v) = %v, want %v", i, step.progress, got, step.want)
		}
	}
}

func TestProgressTokenReleasedAfterCompletion(t *testing.T) {
	s := NewServer("test", nil)
	if err := s.registerProgressToken("rel"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if !s.AllowProgress("rel", 1) {
		t.Fatal("AllowProgress should succeed while the token is active")
	}
	s.releaseProgressToken("rel")
	if s.AllowProgress("rel", 2) {
		t.Fatal("AllowProgress should fail once the token has been released")
	}
}

func TestProgressFloodGuardDropsExcessNotifications(t *testing.T) {
	s := NewServer("test", nil)
	if err := s.registerProgressToken("flood"); err != nil {
		t.Fatalf("register: %v", err)
	}
	allowed := 0
	for i := 1; i <= 25; i++ {
		if s.AllowProgress("flood", float64(i)) {
			allowed++
		}
	}
	if allowed != maxProgressNotificationsPerSecond {
		t.Fatalf("flood guard allowed %d notifications, want %d", allowed, maxProgressNotificationsPerSecond)
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
