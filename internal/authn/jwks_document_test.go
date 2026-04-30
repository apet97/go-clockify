package authn

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestJWKSCache_RejectsDuplicateKid pins the JWKS-document-level
// invariant from RFC 7517 §4.5: every JWK in a JWKS MUST have a
// unique kid. Pre-fix, jwksCache.reload's `keys[key.KID] = pub` loop
// silently overwrote duplicates, so a malformed or hostile JWKS
// could mask a legitimate key without raising any error — the
// hardest class of bug to detect post-deployment because both
// /.well-known/jwks.json fetches succeed and only signature
// verification surfaces the loss, attributed to the wrong kid.
//
// The test builds a two-RSA-key JWKS where both entries claim kid
// "k1", serves it from an httptest server, and drives a real RS256
// JWT through the OIDC authenticator. The reload must fail with an
// error mentioning "duplicate kid"; the cache must NOT be partially
// populated (next reload still observes the same failure).
func TestJWKSCache_RejectsDuplicateKid(t *testing.T) {
	const (
		issuer   = "https://issuer.example.test"
		audience = "clockify-mcp"
		subject  = "user-42"
		kid      = "k1"
	)

	// Two distinct RSA keys, both labelled kid="k1". Either one
	// would verify a token signed with itself; the duplicate is the
	// invariant violation, independent of which key "wins" the
	// silent overwrite.
	privA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa A: %v", err)
	}
	privB, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa B: %v", err)
	}

	jwks := buildDuplicateKidJWKS(t, kid, &privA.PublicKey, &privB.PublicKey)
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
	token := signJWT(t, privA, kid, map[string]any{
		"iss": issuer,
		"sub": subject,
		"aud": []string{audience},
		"exp": now + 300,
		"nbf": now - 5,
		"iat": now,
	})

	_, err = authenticate(t, auth, token)
	if err == nil {
		t.Fatal("expected duplicate-kid JWKS to fail authentication")
	}
	if !strings.Contains(err.Error(), "duplicate kid") {
		t.Fatalf("expected error to mention duplicate kid, got: %v", err)
	}

	// Atomicity: a second Authenticate call must observe the same
	// failure. If reload had partially populated c.keys with one of
	// the duplicates before the error fired, the second call would
	// hit the cached map and verify the token as if the JWKS were
	// well-formed.
	if _, err2 := authenticate(t, auth, token); err2 == nil {
		t.Fatal("expected second authenticate to also fail; cache must not be partially populated on duplicate-kid error")
	} else if !strings.Contains(err2.Error(), "duplicate kid") {
		t.Fatalf("expected second error to mention duplicate kid, got: %v", err2)
	}
}

// TestJWKSCache_RejectsDuplicateEmptyKid covers the corner case
// where a JWKS publishes two unkeyed entries. The empty kid is its
// own valid bucket per spec (used by the kid-less single-key
// fallback in jwksCache.key); two entries sharing it are still a
// uniqueness violation and must be rejected with the same error
// shape.
func TestJWKSCache_RejectsDuplicateEmptyKid(t *testing.T) {
	const issuer = "https://issuer.example.test"

	privA, _ := rsa.GenerateKey(rand.Reader, 2048)
	privB, _ := rsa.GenerateKey(rand.Reader, 2048)

	jwks := buildDuplicateKidJWKS(t, "", &privA.PublicKey, &privB.PublicKey)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	auth, err := New(Config{
		Mode:        ModeOIDC,
		OIDCIssuer:  issuer,
		OIDCJWKSURL: ts.URL,
		HTTPClient:  ts.Client(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Any well-formed token will do — the reload error fires before
	// signature verification is reached.
	now := time.Now().Unix()
	token := signJWT(t, privA, "", map[string]any{
		"iss": issuer,
		"sub": "anyone",
		"exp": now + 300,
	})

	_, err = authenticate(t, auth, token)
	if err == nil {
		t.Fatal("expected duplicate empty-kid JWKS to fail authentication")
	}
	if !strings.Contains(err.Error(), "duplicate kid") {
		t.Fatalf("expected error to mention duplicate kid, got: %v", err)
	}
}

// buildDuplicateKidJWKS produces a JWKS document with two RSA
// public keys both stamped with the same kid. Variant of buildJWKS
// (oidc_integration_test.go) extended to two keys; kept here so the
// existing helper's single-key contract isn't perturbed.
func buildDuplicateKidJWKS(tb testing.TB, kid string, a, b *rsa.PublicKey) []byte {
	tb.Helper()
	enc := func(pub *rsa.PublicKey) map[string]any {
		return map[string]any{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}
	}
	doc := map[string]any{
		"keys": []map[string]any{enc(a), enc(b)},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		tb.Fatalf("jwks marshal: %v", err)
	}
	return body
}
