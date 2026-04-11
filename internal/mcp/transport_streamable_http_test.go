package mcp

import (
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
