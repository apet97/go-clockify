package mcp

import (
	"context"
	"encoding/json"
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
