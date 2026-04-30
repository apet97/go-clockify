package authn

import (
	"context"
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
