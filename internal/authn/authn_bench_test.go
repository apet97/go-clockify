package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkStaticBearer measures the cheapest auth path: a constant-time
// byte compare against a configured token. This is the baseline every
// other auth mode is compared against.
func BenchmarkStaticBearer(b *testing.B) {
	const token = "bench-bearer-token-0123456789abcdef"
	auth, err := New(Config{Mode: ModeStaticBearer, BearerToken: token})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := auth.Authenticate(ctx, r); err != nil {
			b.Fatalf("Authenticate: %v", err)
		}
	}
}

// BenchmarkForwardAuth measures the header-driven path used when an
// upstream (e.g. Cloudflare Access, oauth2-proxy) has already validated
// the principal and forwarded it via X-Forwarded-User / X-Forwarded-Tenant.
func BenchmarkForwardAuth(b *testing.B) {
	auth, err := New(Config{Mode: ModeForwardAuth})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("X-Forwarded-User", "alice@example.test")
	r.Header.Set("X-Forwarded-Tenant", "acme")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := auth.Authenticate(ctx, r); err != nil {
			b.Fatalf("Authenticate: %v", err)
		}
	}
}

// BenchmarkOIDCVerifyCached measures the steady-state OIDC verify path
// with a warm JWKS cache. Each iteration: decode JWT header/claims +
// validate claims + cached key lookup + RSA signature verify. The JWKS
// HTTP fetch is deliberately outside the timer (warmed once before
// ResetTimer) — this isolates the CPU cost from network cost.
func BenchmarkOIDCVerifyCached(b *testing.B) {
	const (
		issuer      = "https://issuer.bench.test"
		audience    = "clockify-mcp-bench"
		resourceURI = "https://mcp.bench.test/mcp"
		kid         = "bench-key-1"
	)
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("rsa: %v", err)
	}
	jwks := buildJWKS(b, kid, &privKey.PublicKey)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	cfg := Config{
		Mode:            ModeOIDC,
		OIDCIssuer:      issuer,
		OIDCAudience:    audience,
		OIDCJWKSURL:     ts.URL + "/jwks.json",
		OIDCResourceURI: resourceURI,
		DefaultTenantID: "default",
		HTTPClient:      ts.Client(),
	}
	auth, err := New(cfg)
	if err != nil {
		b.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	token := signJWT(b, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       "bench-subject",
		"aud":       []string{audience, resourceURI},
		"exp":       now + 3600,
		"nbf":       now - 5,
		"iat":       now,
		"tenant_id": "bench-tenant",
	})

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	ctx := context.Background()

	// Warm the JWKS cache so the loop measures verify, not HTTP fetch.
	if _, err := auth.Authenticate(ctx, r); err != nil {
		b.Fatalf("warm-up Authenticate: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := auth.Authenticate(ctx, r); err != nil {
			b.Fatalf("Authenticate: %v", err)
		}
	}
}
