package authn

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestOIDCAuthenticator_RejectsHS256Token pins the authenticator boundary
// against the canonical JWT alg-confusion attack: an attacker who knows the
// RSA public key (always public, e.g. via the JWKS endpoint) forges a token
// with header alg=HS256 and signs it with HMAC-SHA256 using the public-key
// bytes as the HMAC secret. A naive verifier that dispatches HS* to HMAC
// using the public key as the secret would accept the forgery. The chokepoint
// is hashForAlg, which only whitelists RS*/ES*; this test fails closed at the
// authenticator boundary so a future refactor can't silently re-open the
// attack class without tripping a regression here.
func TestOIDCAuthenticator_RejectsHS256Token(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "clockify-mcp"
		subject  = "user-42"
		kid      = "k1"
	)

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
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
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	claims := map[string]any{
		"iss": issuer,
		"sub": subject,
		"aud": []string{audience},
		"exp": now + 300,
		"nbf": now - 5,
		"iat": now,
	}

	// Forge an HS256 token whose HMAC key is the RSA modulus bytes — the
	// canonical alg-confusion shape. Any public-key-derived secret would
	// do; modulus bytes are the most directly attacker-accessible form.
	header := map[string]any{"alg": "HS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, privKey.N.Bytes())
	_, _ = mac.Write([]byte(signing))
	forged := signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if _, err := authenticate(t, auth, forged); err == nil {
		t.Fatal("expected HS256 alg-confusion token to be rejected")
	}
}

// TestOIDCAuthenticator_RejectsAlgNone pins rejection of the unsigned-token
// attack: header alg=none with an empty signature segment. hashForAlg's
// default branch is the chokepoint; a regression that special-cased "none"
// to skip verification would silently accept any token whose claims pass
// validateClaims.
func TestOIDCAuthenticator_RejectsAlgNone(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "clockify-mcp"
		subject  = "user-42"
		kid      = "k1"
	)

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
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
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	claims := map[string]any{
		"iss": issuer,
		"sub": subject,
		"aud": []string{audience},
		"exp": now + 300,
		"nbf": now - 5,
		"iat": now,
	}

	header := map[string]any{"alg": "none", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	// Empty third segment — RawURLEncoding.DecodeString("") returns an
	// empty byte slice with no error, so the token shape is valid and
	// rejection has to come from verifyJWT, not decodeJWT.
	unsigned := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb) + "."

	if _, err := authenticate(t, auth, unsigned); err == nil {
		t.Fatal("expected alg=none token to be rejected")
	}
}
