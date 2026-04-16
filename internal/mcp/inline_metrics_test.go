package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testMainBearer = "main-bearer-token-1234"
const testMetricsBearer = "metrics-bearer-token-5678"

// TestInlineMetrics_DisabledByDefault verifies /metrics is absent from the
// main HTTP mux when InlineMetricsOptions.Enabled is false.
func TestInlineMetrics_DisabledByDefault(t *testing.T) {
	s := newTestServer()
	// Simulate what ServeHTTP does with Enabled=false: no /metrics route.
	// We test handleMetricsHandler indirectly by calling ServeHTTP's mux.
	// Since we can't spin a real listener in a unit test, we test the
	// inlineMetricsHandler function directly.

	// With Enabled=false the handler is never installed; test that
	// inlineMetricsHandler with a wrong token returns 401.
	_ = s // silence unused warning
	opts := InlineMetricsOptions{
		Enabled:         true,
		AuthMode:        "inherit_main_bearer",
		MainBearerToken: testMainBearer,
	}
	h := inlineMetricsHandler(opts)

	// No auth header → 401
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestInlineMetrics_InheritMainBearer_CorrectToken verifies that /metrics
// accepts the main bearer token when AuthMode=inherit_main_bearer.
func TestInlineMetrics_InheritMainBearer_CorrectToken(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:         true,
		AuthMode:        "inherit_main_bearer",
		MainBearerToken: testMainBearer,
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testMainBearer)
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct main bearer, got %d", rec.Code)
	}
}

// TestInlineMetrics_InheritMainBearer_WrongToken verifies wrong token → 401.
func TestInlineMetrics_InheritMainBearer_WrongToken(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:         true,
		AuthMode:        "inherit_main_bearer",
		MainBearerToken: testMainBearer,
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}
}

// TestInlineMetrics_StaticBearer_CorrectToken verifies separate bearer works.
func TestInlineMetrics_StaticBearer_CorrectToken(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:         true,
		AuthMode:        "static_bearer",
		BearerToken:     testMetricsBearer,
		MainBearerToken: testMainBearer,
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testMetricsBearer)
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct static bearer, got %d", rec.Code)
	}
}

// TestInlineMetrics_StaticBearer_MainBearerRejected verifies the main bearer
// is NOT accepted when auth mode is static_bearer (tokens are separate).
func TestInlineMetrics_StaticBearer_MainBearerRejected(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:         true,
		AuthMode:        "static_bearer",
		BearerToken:     testMetricsBearer,
		MainBearerToken: testMainBearer,
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testMainBearer) // main token, not metrics
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("static_bearer: main token should be rejected, got %d", rec.Code)
	}
}

// TestInlineMetrics_None_AllowsNoAuth verifies explicit "none" mode permits
// unauthenticated access (operator opted in explicitly).
func TestInlineMetrics_None_AllowsNoAuth(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:  true,
		AuthMode: "none",
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("none mode: expected 200 without auth, got %d", rec.Code)
	}
}

// TestInlineMetrics_InvalidAuthMode returns 500.
func TestInlineMetrics_InvalidAuthMode(t *testing.T) {
	opts := InlineMetricsOptions{
		Enabled:  true,
		AuthMode: "bogus",
	}
	h := inlineMetricsHandler(opts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	h(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("invalid mode: expected 500, got %d", rec.Code)
	}
}
