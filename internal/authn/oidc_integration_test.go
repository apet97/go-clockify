package authn

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOIDCAuthenticator_JWKSIntegration drives a real RSA-signed JWT
// through an httptest-backed JWKS server end to end. This single test
// exercises the JWKS HTTP fetch, decodeJWT, verifyJWT, validateClaims
// (issuer/audience/exp/nbf/resource indicator), and the principal
// extraction logic.
func TestOIDCAuthenticator_JWKSIntegration(t *testing.T) {
	const (
		issuer      = "https://issuer.example.test"
		audience    = "clockify-mcp"
		resourceURI = "https://mcp.example.test/mcp"
		subject     = "user-42"
		tenant      = "tenant-7"
		kid         = "test-key-1"
	)

	// 1. Generate a fresh RSA-2048 key + matching JWK document.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	jwks := buildJWKS(t, kid, &privKey.PublicKey)

	// 2. Stand up an httptest server that serves the JWKS.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks.json" {
			http.NotFound(w, r)
			return
		}
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
		TenantClaim:     "tenant_id",
		SubjectClaim:    "sub",
		DefaultTenantID: "default",
		HTTPClient:      ts.Client(),
	}
	auth, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 3. Issue a valid token: claims include the resource URI in `aud`
	// alongside the configured audience, an issuer match, exp 5min in
	// the future, and the test subject + tenant.
	now := time.Now().Unix()
	validToken := signJWT(t, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       subject,
		"aud":       []string{audience, resourceURI},
		"exp":       now + 300,
		"nbf":       now - 5,
		"iat":       now,
		"tenant_id": tenant,
	})

	// Happy path
	princ, err := authenticate(t, auth, validToken)
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if princ.Subject != subject {
		t.Errorf("subject = %q, want %q", princ.Subject, subject)
	}
	if princ.TenantID != tenant {
		t.Errorf("tenant = %q, want %q", princ.TenantID, tenant)
	}
	if princ.AuthMode != ModeOIDC {
		t.Errorf("mode = %q, want %q", princ.AuthMode, ModeOIDC)
	}

	// Tampered signature: decode the signature segment, XOR a middle byte
	// with 0xFF to guarantee a change, and re-encode. The previous
	// implementation overwrote the last 2 base64url characters with "AA",
	// which was a silent no-op whenever the signature happened to end in 12
	// zero bits (probability ~1/4096 per RSA-2048 signature) — an observed
	// flake under CI reruns that masquerades as a sporadic regression.
	parts := strings.Split(validToken, ".")
	if len(parts) != 3 {
		t.Fatalf("validToken not 3 parts")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if len(sigBytes) == 0 {
		t.Fatalf("signature bytes empty")
	}
	sigBytes[len(sigBytes)/2] ^= 0xFF
	tampered := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sigBytes)
	if _, err := authenticate(t, auth, tampered); err == nil {
		t.Error("expected tampered signature to fail")
	}

	// Mismatched audience (no resource URI in `aud`).
	missingResourceToken := signJWT(t, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       subject,
		"aud":       []string{audience}, // no resourceURI
		"exp":       now + 300,
		"tenant_id": tenant,
	})
	if _, err := authenticate(t, auth, missingResourceToken); err == nil || !strings.Contains(err.Error(), "resource URI") {
		t.Errorf("expected resource URI mismatch error, got: %v", err)
	}

	// Wrong issuer.
	wrongIssuerToken := signJWT(t, privKey, kid, map[string]any{
		"iss":       "https://attacker.example",
		"sub":       subject,
		"aud":       []string{audience, resourceURI},
		"exp":       now + 300,
		"tenant_id": tenant,
	})
	if _, err := authenticate(t, auth, wrongIssuerToken); err == nil || !strings.Contains(err.Error(), "issuer") {
		t.Errorf("expected issuer mismatch error, got: %v", err)
	}

	// Expired token.
	expiredToken := signJWT(t, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       subject,
		"aud":       []string{audience, resourceURI},
		"exp":       now - 60,
		"tenant_id": tenant,
	})
	if _, err := authenticate(t, auth, expiredToken); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected token expired, got: %v", err)
	}

	// Token not yet valid (nbf in the future).
	futureToken := signJWT(t, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       subject,
		"aud":       []string{audience, resourceURI},
		"exp":       now + 3600,
		"nbf":       now + 600,
		"tenant_id": tenant,
	})
	if _, err := authenticate(t, auth, futureToken); err == nil || !strings.Contains(err.Error(), "not valid yet") {
		t.Errorf("expected nbf failure, got: %v", err)
	}

	// Missing bearer header.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := auth.Authenticate(context.Background(), r); err == nil {
		t.Error("expected missing bearer error")
	}
}

// TestOIDCAuthenticator_NoResourceURI verifies that omitting the
// resource URI keeps the legacy OIDCAudience-only behaviour.
func TestOIDCAuthenticator_NoResourceURI(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "clockify-mcp"
		subject  = "user-42"
		kid      = "test-key-1"
	)
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := buildJWKS(t, kid, &privKey.PublicKey)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	auth, err := New(Config{
		Mode:         ModeOIDC,
		OIDCIssuer:   issuer,
		OIDCAudience: audience,
		OIDCJWKSURL:  ts.URL,
		HTTPClient:   ts.Client(),
		// OIDCResourceURI deliberately omitted
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	tok := signJWT(t, privKey, kid, map[string]any{
		"iss": issuer,
		"sub": subject,
		"aud": []string{audience},
		"exp": now + 300,
	})
	if _, err := authenticate(t, auth, tok); err != nil {
		t.Errorf("legacy audience-only path failed: %v", err)
	}
}

// TestOIDCAuthenticator_RequireTenantClaim covers the hosted-service
// switch that disables silent fallback to MCP_DEFAULT_TENANT_ID when a
// token omits the tenant claim. Without the flag, missing tenant
// collapses into the default; with the flag, it is rejected outright.
func TestOIDCAuthenticator_RequireTenantClaim(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "clockify-mcp"
		subject  = "user-42"
		kid      = "test-key-1"
	)
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := buildJWKS(t, kid, &privKey.PublicKey)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	now := time.Now().Unix()
	tokenWithoutTenant := signJWT(t, privKey, kid, map[string]any{
		"iss": issuer,
		"sub": subject,
		"aud": []string{audience},
		"exp": now + 300,
	})
	tokenWithTenant := signJWT(t, privKey, kid, map[string]any{
		"iss":       issuer,
		"sub":       subject,
		"aud":       []string{audience},
		"exp":       now + 300,
		"tenant_id": "tenant-7",
	})

	t.Run("default_falls_back_when_flag_off", func(t *testing.T) {
		auth, err := New(Config{
			Mode:            ModeOIDC,
			OIDCIssuer:      issuer,
			OIDCAudience:    audience,
			OIDCJWKSURL:     ts.URL,
			TenantClaim:     "tenant_id",
			DefaultTenantID: "fallback-tenant",
			HTTPClient:      ts.Client(),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		princ, err := authenticate(t, auth, tokenWithoutTenant)
		if err != nil {
			t.Fatalf("expected fallback to succeed: %v", err)
		}
		if princ.TenantID != "fallback-tenant" {
			t.Errorf("expected fallback tenant, got %q", princ.TenantID)
		}
	})

	t.Run("strict_rejects_missing_tenant", func(t *testing.T) {
		auth, err := New(Config{
			Mode:               ModeOIDC,
			OIDCIssuer:         issuer,
			OIDCAudience:       audience,
			OIDCJWKSURL:        ts.URL,
			TenantClaim:        "tenant_id",
			DefaultTenantID:    "fallback-tenant",
			RequireTenantClaim: true,
			HTTPClient:         ts.Client(),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := authenticate(t, auth, tokenWithoutTenant); err == nil {
			t.Fatal("expected missing-tenant rejection in strict mode")
		} else if !strings.Contains(err.Error(), "missing tenant claim") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("strict_accepts_present_tenant", func(t *testing.T) {
		auth, err := New(Config{
			Mode:               ModeOIDC,
			OIDCIssuer:         issuer,
			OIDCAudience:       audience,
			OIDCJWKSURL:        ts.URL,
			TenantClaim:        "tenant_id",
			DefaultTenantID:    "fallback-tenant",
			RequireTenantClaim: true,
			HTTPClient:         ts.Client(),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		princ, err := authenticate(t, auth, tokenWithTenant)
		if err != nil {
			t.Fatalf("expected token-with-tenant to succeed: %v", err)
		}
		if princ.TenantID != "tenant-7" {
			t.Errorf("expected tenant from claim, got %q", princ.TenantID)
		}
	})
}

// TestProtectedResourceHandler covers the metadata document endpoint.
func TestProtectedResourceHandler(t *testing.T) {
	cfg := Config{
		OIDCIssuer:      "https://issuer.example.test",
		OIDCResourceURI: "https://mcp.example.test/mcp",
	}
	h := ProtectedResourceHandler(cfg)
	if h == nil {
		t.Fatal("handler should not be nil when resource URI set")
	}

	// GET happy path.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var doc ProtectedResourceMetadata
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc.Resource != cfg.OIDCResourceURI {
		t.Errorf("resource = %q", doc.Resource)
	}
	if len(doc.AuthorizationServers) != 1 || doc.AuthorizationServers[0] != cfg.OIDCIssuer {
		t.Errorf("authorization_servers = %v", doc.AuthorizationServers)
	}

	// HEAD must succeed and not write a body.
	wh := httptest.NewRecorder()
	h.ServeHTTP(wh, httptest.NewRequest(http.MethodHead, "/.well-known/oauth-protected-resource", nil))
	if wh.Code != http.StatusOK {
		t.Errorf("HEAD status = %d", wh.Code)
	}
	if wh.Body.Len() != 0 {
		t.Errorf("HEAD body should be empty, got %d bytes", wh.Body.Len())
	}

	// POST must be rejected.
	wp := httptest.NewRecorder()
	h.ServeHTTP(wp, httptest.NewRequest(http.MethodPost, "/.well-known/oauth-protected-resource", nil))
	if wp.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", wp.Code)
	}

	// Empty resource URI returns nil handler (do not mount).
	if got := ProtectedResourceHandler(Config{}); got != nil {
		t.Error("empty resource URI should yield nil handler")
	}
}

// TestWriteUnauthorized verifies the WWW-Authenticate header format.
func TestWriteUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	WriteUnauthorized(w, "invalid_token", "expired token")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
	wa := w.Header().Get("WWW-Authenticate")
	if !strings.Contains(wa, `realm="clockify-mcp"`) {
		t.Errorf("missing realm in WWW-Authenticate: %s", wa)
	}
	if !strings.Contains(wa, `error="invalid_token"`) {
		t.Errorf("missing error param: %s", wa)
	}
	if !strings.Contains(wa, `error_description="expired token"`) {
		t.Errorf("missing error_description: %s", wa)
	}

	// No error code: realm-only header.
	w2 := httptest.NewRecorder()
	WriteUnauthorized(w2, "", "")
	if got := w2.Header().Get("WWW-Authenticate"); got != `Bearer realm="clockify-mcp"` {
		t.Errorf("unexpected header: %q", got)
	}
}

// TestSanitizeHeaderValue covers the strip-table for header injection
// safety.
func TestSanitizeHeaderValue(t *testing.T) {
	got := sanitizeHeaderValue("hello\nworld\"\\")
	if strings.ContainsAny(got, "\n\r\"\\") {
		t.Errorf("sanitize left dangerous chars: %q", got)
	}
}

// --- helpers ----------------------------------------------------------------

// buildJWKS produces a JWKS document containing the supplied RSA public
// key keyed by `kid`. Takes testing.TB so both tests and benchmarks can
// reuse it.
func buildJWKS(tb testing.TB, kid string, pub *rsa.PublicKey) []byte {
	tb.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	doc := map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"use": "sig",
			"n":   n,
			"e":   e,
		}},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		tb.Fatalf("jwks marshal: %v", err)
	}
	return body
}

// signJWT crafts a compact-serialised RS256 JWT for the supplied claims
// signed with the supplied key. Takes testing.TB so both tests and
// benchmarks can reuse it.
func signJWT(tb testing.TB, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	tb.Helper()
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	hash := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		tb.Fatalf("sign: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// authenticate wraps Authenticator.Authenticate with a stub HTTP request
// carrying the supplied bearer token.
func authenticate(t *testing.T, a Authenticator, token string) (Principal, error) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return a.Authenticate(context.Background(), r)
}
