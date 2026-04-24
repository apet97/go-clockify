package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStreamableSSE_OriginRejected verifies that the SSE GET handler
// enforces the same origin policy as the POST RPC handler. Before A3,
// a browser with a disallowed Origin could subscribe to the event
// stream even though the same Origin would be rejected on POST.
func TestStreamableSSE_OriginRejected(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.AllowedOrigins = []string{"https://allowed.example"}

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))

	// Fresh session via POST with the allowed origin so the SSE case is
	// testing origin enforcement, not an unrelated missing-session path.
	initReq := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	initReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	initReq.Header.Set("Origin", "https://allowed.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, initReq)
	sessionID := rec.Header().Get(MCPSessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialize failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Disallowed origin on SSE GET must be rejected before auth, matching
	// POST behaviour.
	badReq := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	badReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	badReq.Header.Set(MCPSessionIDHeader, sessionID)
	badReq.Header.Set("Origin", "https://evil.example")
	badReq.Header.Set("Accept", "text/event-stream")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, badReq)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed origin, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "origin not allowed") {
		t.Fatalf("expected 'origin not allowed' in body, got %s", rec.Body.String())
	}
}

// TestStreamableSSE_OriginAllowedSetsCORS verifies the SSE GET handler
// sets Access-Control-Allow-Origin and Vary on an allowed Origin, so
// browsers actually accept the event stream.
func TestStreamableSSE_OriginAllowedSetsCORS(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.AllowedOrigins = []string{"https://allowed.example"}

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))

	initReq := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	initReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	initReq.Header.Set("Origin", "https://allowed.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, initReq)
	sessionID := rec.Header().Get(MCPSessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialize failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Missing-session error still gives us a response where we can inspect
	// CORS headers that applyOriginPolicy wrote before the auth/session
	// check ran. Using a bogus session avoids opening a long-lived stream.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, "does-not-exist")
	req.Header.Set("Origin", "https://allowed.example")
	req.Header.Set("Accept", "text/event-stream")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example" {
		t.Fatalf("expected Access-Control-Allow-Origin=https://allowed.example, got %q", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("expected Vary to contain Origin, got %q", got)
	}
}
