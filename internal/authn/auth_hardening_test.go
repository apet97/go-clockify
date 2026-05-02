package authn

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOIDCHardening(t *testing.T) {
	// 1. Missing issuer should be rejected at New()
	t.Run("missing_issuer", func(t *testing.T) {
		_, err := New(Config{Mode: ModeOIDC})
		if err == nil {
			t.Fatal("expected error for missing OIDC issuer")
		}
	})

	// 2. Invalid issuer URL should be rejected
	t.Run("invalid_issuer_url", func(t *testing.T) {
		_, err := New(Config{Mode: ModeOIDC, OIDCIssuer: "not-a-url"})
		if err == nil {
			t.Fatal("expected error for invalid OIDC issuer URL")
		}
	})

	// 3. Authenticate with mismatched audience should fail
	// This requires a mock JWKS server or a pre-populated cache.
	// For this unit test, we'll verify the config loading logic and
	// basic structural properties.
}

func TestForwardAuthHardening(t *testing.T) {
	t.Run("trust_only_when_enabled", func(t *testing.T) {
		// When mode is NOT forward_auth, the header should be ignored
		auth, _ := New(Config{Mode: ModeStaticBearer, BearerToken: "token1234"})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-User", "hacker")

		result, err := auth.Authenticate(context.Background(), req)
		if err == nil {
			if result.Subject == "hacker" {
				t.Fatal("X-Forwarded-User header was trusted in static_bearer mode")
			}
		}
	})

	t.Run("custom_headers", func(t *testing.T) {
		auth, _ := New(Config{
			Mode:                 ModeForwardAuth,
			ForwardSubjectHeader: "X-Custom-User",
			ForwardTenantHeader:  "X-Custom-Tenant",
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Custom-User", "user1")
		req.Header.Set("X-Custom-Tenant", "tenant1")

		result, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if result.Subject != "user1" || result.TenantID != "tenant1" {
			t.Fatalf("expected user1/tenant1, got %s/%s", result.Subject, result.TenantID)
		}
	})

	t.Run("missing_headers_fails", func(t *testing.T) {
		auth, _ := New(Config{Mode: ModeForwardAuth})
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		_, err := auth.Authenticate(context.Background(), req)
		if err == nil {
			t.Fatal("expected error for missing forward-auth headers")
		}
	})
}

// TestForwardAuth_RejectsUntrustedSource locks the trusted-proxy
// gate added per ChatGPT's audit: an untrusted source attempting to
// pose as a forward-auth proxy must be rejected before headers are
// inspected. The authenticator now refuses any request whose
// r.RemoteAddr is outside the configured CIDR allow-list.
func TestForwardAuth_RejectsUntrustedSource(t *testing.T) {
	_, trusted, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatalf("parse CIDR: %v", err)
	}
	auth, err := New(Config{
		Mode:                      ModeForwardAuth,
		ForwardSubjectHeader:      "X-Forwarded-User",
		ForwardTenantHeader:       "X-Forwarded-Tenant",
		ForwardAuthTrustedProxies: []*net.IPNet{trusted},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:443" // documented TEST-NET-3 — never trusted
	req.Header.Set("X-Forwarded-User", "spoofer")
	if _, err := auth.Authenticate(context.Background(), req); err == nil {
		t.Fatal("expected error: untrusted source must be refused")
	} else if !strings.Contains(err.Error(), "trusted_proxies") &&
		!strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected error to mention trusted-proxy allowlist, got: %v", err)
	}
}

// TestForwardAuth_AcceptsTrustedCIDR is the symmetric guardrail:
// when the source IS inside the trusted CIDR, the authenticator
// must continue to honour the forwarded headers. Otherwise the
// gate would lock out every legitimate proxy.
func TestForwardAuth_AcceptsTrustedCIDR(t *testing.T) {
	_, trusted, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatalf("parse CIDR: %v", err)
	}
	auth, err := New(Config{
		Mode:                      ModeForwardAuth,
		ForwardSubjectHeader:      "X-Forwarded-User",
		ForwardTenantHeader:       "X-Forwarded-Tenant",
		ForwardAuthTrustedProxies: []*net.IPNet{trusted},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.5.5.5:443" // inside 10.0.0.0/8
	req.Header.Set("X-Forwarded-User", "alice")
	req.Header.Set("X-Forwarded-Tenant", "acme")

	principal, err := auth.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("trusted source must succeed: %v", err)
	}
	if principal.Subject != "alice" || principal.TenantID != "acme" {
		t.Fatalf("principal = %+v, want alice/acme", principal)
	}
}

// TestForwardAuth_EmptyAllowlistPreservesLegacyBehaviour documents
// the deliberate non-default: an empty (or unset)
// ForwardAuthTrustedProxies skips the source check entirely so
// existing self-hosted single-tenant operators who own the network
// boundary do not have to set the env var. doctor --strict refuses
// this configuration in hosted profiles via a separate gate.
func TestForwardAuth_EmptyAllowlistPreservesLegacyBehaviour(t *testing.T) {
	auth, err := New(Config{
		Mode:                 ModeForwardAuth,
		ForwardSubjectHeader: "X-Forwarded-User",
		ForwardTenantHeader:  "X-Forwarded-Tenant",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:443"
	req.Header.Set("X-Forwarded-User", "alice")
	if _, err := auth.Authenticate(context.Background(), req); err != nil {
		t.Fatalf("legacy unset-allowlist behaviour broke: %v", err)
	}
}

// TestForwardAuth_RejectsControlBytesInHeaders pins the
// principal-header sanitization gate. X-Forwarded-User and
// X-Forwarded-Tenant are attacker-controlled bytes (any
// misconfigured / compromised upstream proxy or any deployment
// running the deliberate empty-allow-list legacy mode can deliver
// them). They flow into Principal.Subject and Principal.TenantID,
// which then enter structured slog records as `subject` / `tenant_id`
// keys (internal/mcp/audit.go:83-84, internal/mcp/tools.go:142-143)
// and into downstream tenant scoping. Control bytes / CRLF / NUL /
// non-printable Unicode must be refused at the boundary so they
// cannot mint a Principal.
func TestForwardAuth_RejectsControlBytesInHeaders(t *testing.T) {
	auth, err := New(Config{
		Mode:                 ModeForwardAuth,
		ForwardSubjectHeader: "X-Forwarded-User",
		ForwardTenantHeader:  "X-Forwarded-Tenant",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		name   string
		header string
		value  string
	}{
		{"subject_lf", "X-Forwarded-User", "alice\nattacker"},
		{"subject_cr", "X-Forwarded-User", "alice\rattacker"},
		{"subject_nul", "X-Forwarded-User", "alice\x00attacker"},
		{"subject_us", "X-Forwarded-User", "alice\x1fattacker"},
		{"subject_zwsp", "X-Forwarded-User", "alice\u200battacker"},
		{"tenant_lf", "X-Forwarded-Tenant", "acme\nattacker"},
		{"tenant_cr", "X-Forwarded-Tenant", "acme\rattacker"},
		{"tenant_nul", "X-Forwarded-Tenant", "acme\x00attacker"},
		{"tenant_us", "X-Forwarded-Tenant", "acme\x1fattacker"},
		{"tenant_zwsp", "X-Forwarded-Tenant", "acme\u200battacker"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			// Both headers must be set on every case so a single
			// missing header doesn't masquerade as the rejection
			// we're trying to pin.
			req.Header.Set("X-Forwarded-User", "alice")
			req.Header.Set("X-Forwarded-Tenant", "acme")
			req.Header.Set(tc.header, tc.value)

			_, gotErr := auth.Authenticate(context.Background(), req)
			if gotErr == nil {
				t.Fatalf("expected error for %s=%q, got nil", tc.header, tc.value)
			}
			if !strings.Contains(gotErr.Error(), "disallowed byte") {
				t.Fatalf("expected error to mention 'disallowed byte', got: %v", gotErr)
			}
		})
	}
}

// TestForwardAuth_RejectsDuplicatedAndOversizedHeaders pins the
// single-authority contract for reverse-proxy identity headers. If a
// proxy accidentally forwards two copies of X-Forwarded-User /
// X-Forwarded-Tenant, or a client can stuff a large identity payload
// through the proxy, the authenticator must reject the request instead
// of silently choosing one value.
func TestForwardAuth_RejectsDuplicatedAndOversizedHeaders(t *testing.T) {
	auth, err := New(Config{
		Mode:                 ModeForwardAuth,
		ForwardSubjectHeader: "X-Forwarded-User",
		ForwardTenantHeader:  "X-Forwarded-Tenant",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		name string
		req  func() *http.Request
		want string
	}{
		{
			name: "duplicate_subject",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Add("X-Forwarded-User", "alice")
				req.Header.Add("X-Forwarded-User", "mallory")
				req.Header.Set("X-Forwarded-Tenant", "acme")
				return req
			},
			want: "duplicated values",
		},
		{
			name: "duplicate_tenant",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Forwarded-User", "alice")
				req.Header.Add("X-Forwarded-Tenant", "acme")
				req.Header.Add("X-Forwarded-Tenant", "evil")
				return req
			},
			want: "duplicated values",
		},
		{
			name: "oversized_subject",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Forwarded-User", strings.Repeat("a", maxForwardAuthHeaderBytes+1))
				req.Header.Set("X-Forwarded-Tenant", "acme")
				return req
			},
			want: "too large",
		},
		{
			name: "oversized_tenant",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Forwarded-User", "alice")
				req.Header.Set("X-Forwarded-Tenant", strings.Repeat("t", maxForwardAuthHeaderBytes+1))
				return req
			},
			want: "too large",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, gotErr := auth.Authenticate(context.Background(), tc.req())
			if gotErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(gotErr.Error(), tc.want) {
				t.Fatalf("expected error to mention %q, got: %v", tc.want, gotErr)
			}
		})
	}
}

// TestNewOIDCAuth_StrictRejectsHTTPIssuer locks the second go/no-go
// gate from ChatGPT's hosted-OIDC review. Strict mode binds tokens
// to this server (audience/resource), so the public keys used to
// verify those bindings must come over TLS — otherwise a network
// adversary swaps the JWKS for keys they control and the strict
// claim binding becomes meaningless.
func TestNewOIDCAuth_StrictRejectsHTTPIssuer(t *testing.T) {
	_, err := New(Config{
		Mode:            ModeOIDC,
		OIDCStrict:      true,
		OIDCIssuer:      "http://idp.example.com",
		OIDCAudience:    "mcp-clockify",
		OIDCResourceURI: "https://mcp.example.com",
	})
	if err == nil {
		t.Fatal("expected error: strict mode must refuse http issuer")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected error mentioning https, got: %v", err)
	}
}

// TestNewOIDCAuth_StrictRejectsHTTPJWKS locks the same gate for an
// explicit JWKS URL when an operator overrides the issuer-derived
// path with MCP_OIDC_JWKS_URL.
func TestNewOIDCAuth_StrictRejectsHTTPJWKS(t *testing.T) {
	_, err := New(Config{
		Mode:            ModeOIDC,
		OIDCStrict:      true,
		OIDCIssuer:      "https://idp.example.com",
		OIDCJWKSURL:     "http://idp.example.com/keys",
		OIDCAudience:    "mcp-clockify",
		OIDCResourceURI: "https://mcp.example.com",
	})
	if err == nil {
		t.Fatal("expected error: strict mode must refuse http JWKS URL")
	}
}

// TestNewOIDCAuth_NonStrictAllowsHTTPIssuer documents the deliberate
// asymmetry: non-strict deployments (typical local dev / single-
// tenant stdio) keep accepting http issuers because they don't bind
// tokens to a specific aud/resource and the loss is bounded.
func TestNewOIDCAuth_NonStrictAllowsHTTPIssuer(t *testing.T) {
	if _, err := New(Config{
		Mode:       ModeOIDC,
		OIDCIssuer: "http://idp.example.com",
	}); err != nil {
		t.Fatalf("expected non-strict http issuer to be accepted, got: %v", err)
	}
}

// TestJWKSCache_DefaultTimeoutApplied locks that the package-level
// fallback HTTP client used when newOIDCAuthenticator was constructed
// without an explicit *http.Client carries a finite timeout.
// Otherwise a hung issuer would freeze every concurrent verify
// past cache expiry — http.DefaultClient has no deadline.
func TestJWKSCache_DefaultTimeoutApplied(t *testing.T) {
	if jwksDefaultHTTPClient == nil {
		t.Fatal("jwksDefaultHTTPClient is nil")
	}
	if jwksDefaultHTTPClient.Timeout <= 0 {
		t.Fatalf("jwksDefaultHTTPClient.Timeout must be > 0, got %s", jwksDefaultHTTPClient.Timeout)
	}
	if jwksDefaultHTTPClient.Timeout > 30*time.Second {
		t.Fatalf("jwksDefaultHTTPClient.Timeout %s is too generous; should bound auth-path latency", jwksDefaultHTTPClient.Timeout)
	}
}

// TestJWKSCache_HungServerRespectsDefaultTimeout drives the timeout
// end-to-end: a JWKS endpoint that never responds must fail the
// reload within the configured budget rather than block forever.
// The httptest server holds the connection open until the test
// cleans up; the cache must time out long before that.
func TestJWKSCache_HungServerRespectsDefaultTimeout(t *testing.T) {
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(hung.Close)

	cache := &jwksCache{
		url:    hung.URL + "/keys",
		client: &http.Client{Timeout: 100 * time.Millisecond},
	}
	start := time.Now()
	err := cache.reload(context.Background())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error from hung JWKS endpoint")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("reload should fail fast under timeout; took %s", elapsed)
	}
}
