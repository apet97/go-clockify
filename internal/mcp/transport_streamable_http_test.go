package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
)

func TestStreamableHTTPSessionIsolation(t *testing.T) {
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-a",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	mgr := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	opts := StreamableHTTPOptions{
		Version:       "test",
		MaxBodySize:   2097152,
		SessionTTL:    30 * time.Minute,
		Authenticator: authenticator,
		ControlPlane:  store,
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			var server *Server
			activateTool := ToolDescriptor{
				Tool:    Tool{Name: "extra_tool", Description: "activated tool"},
				Handler: func(_ context.Context, _ map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
			}
			search := ToolDescriptor{
				Tool: Tool{Name: "search_tools", Description: "activates tools"},
				Handler: func(_ context.Context, args map[string]any) (any, error) {
					if group, _ := args["activate_group"].(string); group == "extra" {
						if err := server.ActivateGroup(group, []ToolDescriptor{activateTool}); err != nil {
							return nil, err
						}
					}
					return map[string]any{"activated": args["activate_group"]}, nil
				},
			}
			server = NewServer("test", []ToolDescriptor{search}, nil, nil)
			return &StreamableSessionRuntime{
				Server:          server,
				Close:           func() {},
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
	}

	handler := streamableRPCHandler(opts, mgr)
	session1 := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"client-1","version":"1"}}}`)
	session2 := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"client-2","version":"2"}}}`)

	_ = callStreamSession(t, handler, session1, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_tools","arguments":{"activate_group":"extra"}}}`)
	list1 := callStreamSession(t, handler, session1, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	list2 := callStreamSession(t, handler, session2, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`)

	if !toolsListContains(list1, "extra_tool") {
		t.Fatal("session1 should include activated tool")
	}
	if toolsListContains(list2, "extra_tool") {
		t.Fatal("session2 should not inherit activated tool")
	}

	s1, err := mgr.get(session1)
	if err != nil {
		t.Fatalf("session1 lookup: %v", err)
	}
	s2, err := mgr.get(session2)
	if err != nil {
		t.Fatalf("session2 lookup: %v", err)
	}
	if len(s1.events.backlog) == 0 {
		t.Fatal("session1 should have a queued listChanged notification")
	}
	if len(s2.events.backlog) != 0 {
		t.Fatal("session2 should not receive session1 notifications")
	}
}

func initializeStreamSession(t *testing.T, handler http.Handler, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	sessionID := rec.Header().Get("X-MCP-Session-ID")
	if sessionID == "" {
		t.Fatal("missing session id header")
	}
	return sessionID
}

func callStreamSession(t *testing.T, handler http.Handler, sessionID, body string) Response {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)
	handler.ServeHTTP(rec, req)
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// newTestStreamableStack builds a minimal streamable HTTP stack wired for
// unit tests: static bearer auth, in-memory control plane, no-op tool
// handlers. Returns the session manager + the configured opts so tests can
// hand them to streamableRPCHandler / streamableEventsHandler directly or
// mount them on a test mux.
func newTestStreamableStack(t *testing.T) (*streamSessionManager, StreamableHTTPOptions) {
	t.Helper()
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-a",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	mgr := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	opts := StreamableHTTPOptions{
		Version:       "test",
		MaxBodySize:   2097152,
		SessionTTL:    30 * time.Minute,
		Authenticator: authenticator,
		ControlPlane:  store,
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			server := NewServer("test", []ToolDescriptor{}, nil, nil)
			return &StreamableSessionRuntime{
				Server:          server,
				Close:           func() {},
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
	}
	return mgr, opts
}

// TestStreamableUnifiedRouteSSE verifies that GET /mcp serves the SSE stream
// (the spec-canonical location per MCP Streamable HTTP 2025-03-26 §3.3),
// including the new per-event `id:` line required for Last-Event-ID resumability.
func TestStreamableUnifiedRouteSSE(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))
	mux.Handle("GET /mcp/events", streamableEventsHandler(opts, mgr))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sessionID := initializeStreamSession(t, mux, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	session, err := mgr.get(sessionID)
	if err != nil {
		t.Fatalf("session lookup: %v", err)
	}
	if err := session.events.Notify("test/event", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type: %q", ct)
	}

	// Read line by line until we see the id/event pair or context fires.
	reader := bufio.NewReader(resp.Body)
	var sawID, sawEvent bool
	for !sawID || !sawEvent {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v (sawID=%v sawEvent=%v)", err, sawID, sawEvent)
		}
		if strings.HasPrefix(line, "id: 1") {
			sawID = true
		}
		if strings.HasPrefix(line, "event: test/event") {
			sawEvent = true
		}
	}
}

// TestStreamableEventsBackCompatAlias verifies that GET /mcp/events still
// serves the SSE stream during the 0.6 deprecation window.
func TestStreamableEventsBackCompatAlias(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))
	mux.Handle("GET /mcp/events", streamableEventsHandler(opts, mgr))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sessionID := initializeStreamSession(t, mux, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp/events", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type: %q", ct)
	}
}

// TestStreamableProtocolVersionMismatch rejects a non-initialize request
// with an unsupported or mismatched Mcp-Protocol-Version header.
func TestStreamableProtocolVersionMismatch(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	// Unsupported version entirely.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)
	req.Header.Set("Mcp-Protocol-Version", "1999-01-01")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported: status %d body %s", rec.Code, rec.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected -32600, got %+v", resp.Error)
	}

	// Supported but wrong version for this session.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)
	req.Header.Set("Mcp-Protocol-Version", "2024-11-05")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched: status %d body %s", rec.Code, rec.Body.String())
	}
}

// TestStreamableProtocolVersionAbsent allows non-initialize requests without
// the Mcp-Protocol-Version header (pre-2025-03-26 clients).
func TestStreamableProtocolVersionAbsent(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("X-MCP-Session-ID", sessionID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("absent header: status %d body %s", rec.Code, rec.Body.String())
	}
}

func toolsListContains(resp Response, name string) bool {
	result, ok := resp.Result.(map[string]any)
	if !ok {
		return false
	}
	items, ok := result["tools"].([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tool["name"] == name {
			return true
		}
	}
	return false
}
