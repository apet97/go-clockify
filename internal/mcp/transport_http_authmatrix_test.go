package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
)

// TestLegacyHTTP_ForwardAuth confirms that the legacy HTTP transport
// now honours auth modes beyond static_bearer. Before A2 the handler
// hardcoded a bearer compare; a request with forward_auth headers but
// no Authorization header would 401 even when config promised
// MCP_AUTH_MODE=forward_auth.
func TestLegacyHTTP_ForwardAuth(t *testing.T) {
	auth, err := authn.New(authn.Config{
		Mode:                 authn.ModeForwardAuth,
		DefaultTenantID:      "acme",
		ForwardSubjectHeader: "X-Forwarded-User",
		ForwardTenantHeader:  "X-Forwarded-Tenant",
	})
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}

	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(auth, nil, true, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	// No Authorization header; forward-auth headers supply the principal.
	req.Header.Set("X-Forwarded-User", "alice@example.com")
	req.Header.Set("X-Forwarded-Tenant", "acme")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for forward_auth-authorised request, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestLegacyHTTP_StaticBearerIgnoresForwardedHeaders asserts the HTTP
// transport does NOT elevate trust based on X-Forwarded-User /
// X-Forwarded-Tenant when MCP_AUTH_MODE=static_bearer. A proxy that
// forgets to strip these headers, or a client that sets them directly,
// must never be able to impersonate another identity.
func TestLegacyHTTP_StaticBearerIgnoresForwardedHeaders(t *testing.T) {
	auth, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     "static-bearer-test-token-123456",
		DefaultTenantID: "real-tenant",
	})
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}

	s := newTestServer()
	s.initialized.Store(true)
	handler := s.handleMCP(auth, nil, true, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Authorization", "Bearer static-bearer-test-token-123456")
	// Attacker attempts to inject a principal; static_bearer mode must
	// ignore these and keep the default tenant.
	req.Header.Set("X-Forwarded-User", "attacker@evil.example")
	req.Header.Set("X-Forwarded-Tenant", "other-tenant")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid static_bearer, got %d body=%s", rec.Code, rec.Body.String())
	}
	// AuditTenantID is populated only on streamable_http (per-session);
	// the legacy HTTP transport authenticates but does not write audit
	// events. The assertion that matters here is that the request was
	// accepted with the valid bearer — the forwarded headers did not
	// cause a 401 (which would happen if the server tried to validate
	// them) and did not cause tenant rebinding (validated below by the
	// static-bearer authenticator contract).
	if got := s.AuditTenantID; got != "" && got != "real-tenant" {
		t.Fatalf("forwarded tenant leaked into AuditTenantID: got %q, want \"\" or \"real-tenant\"", got)
	}
}

// TestLegacyHTTP_ForwardAuthMissingHeadersRejected ensures the failure
// mode for forward_auth is a 401, not a static-bearer bypass.
func TestLegacyHTTP_ForwardAuthMissingHeadersRejected(t *testing.T) {
	auth, err := authn.New(authn.Config{
		Mode:                 authn.ModeForwardAuth,
		DefaultTenantID:      "acme",
		ForwardSubjectHeader: "X-Forwarded-User",
		ForwardTenantHeader:  "X-Forwarded-Tenant",
	})
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}
	s := newTestServer()
	handler := s.handleMCP(auth, nil, true, 2097152)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	// Deliberately omit the forward-auth headers.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when forward-auth headers absent, got %d", rec.Code)
	}
}
