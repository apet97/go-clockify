package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testBearerToken = "test-secret-token"

func newTestServer() *Server {
	return NewServer("test", []ToolDescriptor{{
		Tool:    Tool{Name: "echo", Description: "echoes input"},
		Handler: func(_ context.Context, args map[string]any) (any, error) { return args, nil },
	}}, nil, nil)
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	s.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
	if body["version"] != "test" {
		t.Fatalf("expected version test, got %q", body["version"])
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
}

func TestReadyNotInitialized(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("expected status not_ready, got %q", body["status"])
	}
}

func TestReadyInitialized(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	s.handleReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
}

func TestReadyUpstreamUnhealthy(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	s.ReadyChecker = func(ctx context.Context) error {
		return fmt.Errorf("clockify API unreachable")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when upstream unhealthy, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("expected status not_ready, got %q", body["status"])
	}
}

func TestReadyUpstreamHealthy(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	s.ReadyChecker = func(ctx context.Context) error {
		return nil
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	s.handleReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when upstream healthy, got %d", rec.Code)
	}
}

func TestCORSVaryOriginHeader(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(testBearerToken, []string{"https://app.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin header when reflecting origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected ACAO to reflect origin, got %q", got)
	}
}

func TestCORSNoVaryOnWildcard(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(testBearerToken, nil, true, 2097152) // allowAnyOrigin=true

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Origin", "https://anything.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got == "Origin" {
		t.Fatal("should NOT set Vary: Origin when using wildcard ACAO")
	}
}

func TestPreflightMaxAge(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, []string{"https://app.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Fatalf("expected Access-Control-Max-Age 86400, got %q", got)
	}
}

func TestMCPUnauthorized(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, nil, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMCPRejectsTokenWithoutBearerPrefix(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, nil, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", testBearerToken)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMCPAuthorized(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(testBearerToken, nil, true, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %q", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestMCPCORSBlocked(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, []string{"https://allowed.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Origin", "https://evil.example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestMCPCORSAllowed(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(testBearerToken, []string{"https://allowed.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Origin", "https://allowed.example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example.com" {
		t.Fatalf("expected CORS header %q, got %q", "https://allowed.example.com", got)
	}
}

func TestMCPMethodNotAllowed(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, nil, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestMCPCORSPreflight(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, []string{"https://allowed.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://allowed.example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "POST" {
		t.Fatalf("expected Allow-Methods POST, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("expected Allow-Headers 'Content-Type, Authorization', got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example.com" {
		t.Fatalf("expected CORS origin header, got %q", got)
	}
}

func TestMCPCORSPreflightBlocked(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(testBearerToken, []string{"https://allowed.example.com"}, false, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked origin preflight, got %d", rec.Code)
	}
}

func TestHTTPToolsCall(t *testing.T) {
	s := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "test_tool", Description: "returns hello world"},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return map[string]any{"hello": "world"}, nil
		},
		ReadOnlyHint: true,
	}}, nil, nil)
	s.initialized.Store(true)

	const bearer = "test-bearer-token-1234"
	handler := s.handleMCP(bearer, nil, true, 2097152)

	rec := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	// Result should contain content with the tool's output
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	content, ok := resultMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content array, got %v", resultMap["content"])
	}
	firstContent, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map in content[0], got %T", content[0])
	}
	text, ok := firstContent["text"].(string)
	if !ok || text == "" {
		t.Fatalf("expected non-empty text in content, got %v", firstContent["text"])
	}
	// The text should contain our tool's output (JSON-encoded)
	if !strings.Contains(text, `"hello"`) || !strings.Contains(text, `"world"`) {
		t.Fatalf("expected tool output in text, got %q", text)
	}
}

func TestHTTPToolsList(t *testing.T) {
	s := NewServer("test", []ToolDescriptor{
		{
			Tool:         Tool{Name: "alpha_tool", Description: "first tool"},
			Handler:      func(_ context.Context, args map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: true,
		},
		{
			Tool:         Tool{Name: "beta_tool", Description: "second tool"},
			Handler:      func(_ context.Context, args map[string]any) (any, error) { return nil, nil },
			ReadOnlyHint: true,
		},
	}, nil, nil)
	s.initialized.Store(true)

	const bearer = "test-bearer-token-1234"
	handler := s.handleMCP(bearer, nil, true, 2097152)

	rec := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := resultMap["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", resultMap["tools"])
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Tools should be sorted alphabetically
	first, _ := tools[0].(map[string]any)
	second, _ := tools[1].(map[string]any)
	if first["name"] != "alpha_tool" {
		t.Fatalf("expected first tool alpha_tool, got %v", first["name"])
	}
	if second["name"] != "beta_tool" {
		t.Fatalf("expected second tool beta_tool, got %v", second["name"])
	}
}

func TestHTTPBodyTooLarge(t *testing.T) {
	s := newTestServer()
	s.initialized.Store(true)

	const bearer = "test-bearer-token-1234"
	// Set a very small max body size (64 bytes)
	handler := s.handleMCP(bearer, nil, true, 64)

	// Create a body that exceeds the limit
	largeBody := strings.Repeat("x", 256)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(largeBody))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowed        []string
		allowAnyOrigin bool
		want           bool
	}{
		{"empty list rejects by default", "https://any.example.com", nil, false, false},
		{"allowAnyOrigin allows all", "https://any.example.com", nil, true, true},
		{"exact match", "https://foo.com", []string{"https://foo.com"}, false, true},
		{"case insensitive", "HTTPS://FOO.COM", []string{"https://foo.com"}, false, true},
		{"not in list", "https://bar.com", []string{"https://foo.com"}, false, false},
		{"multiple allowed", "https://bar.com", []string{"https://foo.com", "https://bar.com"}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOriginAllowed(tt.origin, tt.allowed, tt.allowAnyOrigin); got != tt.want {
				t.Fatalf("isOriginAllowed(%q, %v, %v) = %v, want %v", tt.origin, tt.allowed, tt.allowAnyOrigin, got, tt.want)
			}
		})
	}
}
