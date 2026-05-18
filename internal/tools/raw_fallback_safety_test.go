package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

func TestSafeRawPathRejectsEscapingAttempts(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"absolute_url", "https://evil.example/workspaces/{workspaceId}/clients"},
		{"encoded_host", "/%2f%2fevil.example/workspaces/{workspaceId}/clients"},
		{"dotdot", "/workspaces/{workspaceId}/../users"},
		{"backslash", `\workspaces\{workspaceId}\clients`},
		{"encoded_dotdot", "/workspaces/{workspaceId}/%2e%2e/users"},
		{"duplicated_slash", "/workspaces/{workspaceId}//clients"},
		{"query_path_confusion", "/workspaces/{workspaceId}?path=/workspaces/ws2/clients"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := safeRawPath("ws1", c.path); err == nil {
				t.Fatalf("safeRawPath(%q) = %q, want rejection", c.path, got)
			}
		})
	}
}

func TestSafeRawPathRejectsControlBytes(t *testing.T) {
	cases := []string{
		"/workspaces/{workspaceId}/projects%0d%0aX",
		"/workspaces/{workspaceId}/projects\x00",
	}
	for _, path := range cases {
		if got, err := safeRawPath("ws1", path); err == nil {
			t.Fatalf("safeRawPath(%q) = %q, want rejection", path, got)
		}
	}
	if got, err := safeRawPath("ws1", "/user"); err != nil || got != "/user" {
		t.Fatalf("safeRawPath(/user) = %q, %v; want /user, nil", got, err)
	}
}

func TestSafeRawPathAllowsPinnedWorkspaceAndUserPaths(t *testing.T) {
	cases := map[string]string{
		"/user":                                  "/user",
		"/workspaces/{workspaceId}":              "/workspaces/ws1",
		"/workspaces/{workspaceId}/clients":      "/workspaces/ws1/clients",
		"/api/v1/workspaces/{workspaceId}/users": "/workspaces/ws1/users",
		"/v1/workspaces/{workspaceId}/projects":  "/workspaces/ws1/projects",
	}

	for input, want := range cases {
		got, err := safeRawPath("ws1", input)
		if err != nil {
			t.Fatalf("safeRawPath(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("safeRawPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSafeRawPathAllowsDocumentedSchedulingCopyExample(t *testing.T) {
	got, err := safeRawPath("ws1", "/workspaces/{workspaceId}/scheduling/assignments/assignment-1/copy")
	if err != nil {
		t.Fatalf("documented scheduling-copy path was rejected: %v", err)
	}
	want := "/workspaces/ws1/scheduling/assignments/assignment-1/copy"
	if got != want {
		t.Fatalf("safeRawPath = %q, want %q", got, want)
	}
}

func TestRawFallbackCannotHitFileImage(t *testing.T) {
	called := false
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("raw fallback reached upstream: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.RawAPIGet(context.Background(), map[string]any{"path": "/file/image"}); err == nil {
		t.Fatal("RawAPIGet(/file/image) succeeded, want rejection")
	}
	svc.EnableRawWrites = true
	if _, err := svc.RawAPIRequest(context.Background(), map[string]any{"method": "POST", "path": "/file/image", "body": map[string]any{}}); err == nil {
		t.Fatal("RawAPIRequest(POST /file/image) succeeded, want rejection")
	}
	if called {
		t.Fatal("raw fallback must reject /file/image before any upstream request")
	}
}

func TestRawFallbackNonGETMethodsRequireExplicitEnablement(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/clients" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "client-1"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/clients",
		"body":   map[string]any{"name": "Blocked"},
	})
	if err == nil || !strings.Contains(err.Error(), "CLOCKIFY_ENABLE_RAW_WRITES") {
		t.Fatalf("disabled raw POST err = %v, want CLOCKIFY_ENABLE_RAW_WRITES gate", err)
	}

	svc.EnableRawWrites = true
	result, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/clients",
		"body":   map[string]any{"name": "Allowed"},
	})
	if err != nil {
		t.Fatalf("enabled raw POST failed: %v", err)
	}
	envelope, ok := result.(ToolResult)
	if !ok {
		t.Fatalf("raw POST result type = %T, want ToolResult", result)
	}
	if envelope.Action != "clockify_api_request" {
		t.Fatalf("action = %s, want clockify_api_request", envelope.Action)
	}
}

func TestRawAPIWriteDocumentedOnly(t *testing.T) {
	var called bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/clients" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "client-1"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	svc.RawWriteDocumentedOnly = true
	if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/clients",
		"body":   map[string]any{"name": "Allowed"},
	}); err != nil {
		t.Fatalf("documented raw write rejected: %v", err)
	}
	if !called {
		t.Fatal("documented raw write did not reach upstream")
	}
}

// TestRawAPIWriteDocumentedOnlyAllowsByIDRoute pins that the documented-only
// allowlist matches templated path parameters: a real DELETE targeting a
// project by id matches DELETE /workspaces/{workspaceId}/projects/{projectId}.
func TestRawAPIWriteDocumentedOnlyAllowsByIDRoute(t *testing.T) {
	var called bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/workspaces/ws1/projects/proj-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	svc.RawWriteDocumentedOnly = true
	if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "DELETE",
		"path":   "/workspaces/{workspaceId}/projects/proj-1",
	}); err != nil {
		t.Fatalf("documented by-id raw write rejected: %v", err)
	}
	if !called {
		t.Fatal("documented by-id raw write did not reach upstream")
	}
}

func TestRawAPIWriteDocumentedOnlyRejectsUndocumented(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("undocumented raw write reached upstream: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	svc.RawWriteDocumentedOnly = true
	_, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/not-documented",
		"body":   map[string]any{"name": "Blocked"},
	})
	if err == nil || !strings.Contains(err.Error(), "CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=false") {
		t.Fatalf("undocumented raw write err = %v", err)
	}
}

func TestRawAPIWriteDocumentedOnlyFalseAllowsUndocumented(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/not-documented" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"ok": true})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	svc.RawWriteDocumentedOnly = false
	if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/not-documented",
		"body":   map[string]any{"name": "Allowed"},
	}); err != nil {
		t.Fatalf("undocumented raw write with flag false failed: %v", err)
	}
}

func TestRawAPIDeletePreservesBody(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/workspaces/ws1/deletions/body" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "deleted-123", "status": "DELETED"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	result, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "DELETE",
		"path":   "/workspaces/{workspaceId}/deletions/body",
	})
	if err != nil {
		t.Fatalf("raw DELETE failed: %v", err)
	}
	envelope := result.(ToolResult)
	data := envelope.Data.(map[string]any)
	if data["id"] != "deleted-123" || data["status"] != "DELETED" {
		t.Fatalf("raw DELETE data = %+v", data)
	}
	if data["deleted"] == true {
		t.Fatalf("raw DELETE discarded upstream body: %+v", data)
	}
}

func TestRawAPIDeleteEmptyBody(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/workspaces/ws1/deletions/empty" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawWrites = true
	result, err := svc.RawAPIRequest(context.Background(), map[string]any{
		"method": "DELETE",
		"path":   "/workspaces/{workspaceId}/deletions/empty",
	})
	if err != nil {
		t.Fatalf("raw DELETE failed: %v", err)
	}
	envelope := result.(ToolResult)
	data := envelope.Data.(map[string]any)
	if data["deleted"] != true {
		t.Fatalf("raw DELETE empty data = %+v", data)
	}
}

func TestRawFallbackRecoveryHintsPreferTypedTools(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("raw fallback reached upstream before recovery: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	initializeServer(t, server)

	cases := []struct {
		name     string
		args     map[string]any
		wantText []string
	}{
		{
			name: "clockify_api_get",
			args: map[string]any{
				"path": "https://evil.example/workspaces/{workspaceId}/clients",
			},
			wantText: []string{"workflow", "domain", "typed", "raw get"},
		},
		{
			name: "clockify_api_request",
			args: map[string]any{
				"method": "POST",
				"path":   "/workspaces/{workspaceId}/clients",
				"body":   map[string]any{"name": "Blocked"},
			},
			wantText: []string{"workflow", "domain", "typed", "raw writes"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := callToolError(t, server, c.name, c.args)
			if result.Recovery.Tool != "clockify_tools_guide" {
				t.Fatalf("recovery tool = %q, want clockify_tools_guide; envelope=%+v", result.Recovery.Tool, result)
			}
			hint := strings.ToLower(result.Recovery.Hint)
			for _, want := range c.wantText {
				if !strings.Contains(hint, want) {
					t.Fatalf("recovery hint %q missing %q", result.Recovery.Hint, want)
				}
			}
		})
	}
}
