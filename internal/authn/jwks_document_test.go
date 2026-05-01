package authn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

// TestJWKSCache_RefreshesOnKidMissAfterRotation pins the F2 invariant:
// when the IdP rotates keys between scheduled JWKS reloads, the
// authenticator must trigger one extra refresh on kid-miss instead of
// rejecting valid post-rotation tokens for the full 5-minute cache
// window. Pre-fix, jwksCache.key returned `oidc key "<kid>" not found`
// for every token signed by the new key until the cache expired —
// every operator-side rotation produced up to a 5-minute customer-
// visible auth outage.
//
// The test stands up a JWKS server whose response can be atomically
// swapped, primes the cache with key A, "rotates" to key B by
// swapping the payload, and drives a token signed with B through the
// authenticator. The verify must succeed and the server must have
// been hit exactly twice (initial fetch + kid-miss-triggered refresh).
func TestJWKSCache_RefreshesOnKidMissAfterRotation(t *testing.T) {
	const issuer = "https://issuer.example.test"

	privA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa A: %v", err)
	}
	privB, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa B: %v", err)
	}

	jwksA := buildJWKS(t, "kA", &privA.PublicKey)
	jwksB := buildJWKS(t, "kB", &privB.PublicKey)
	var current atomic.Pointer[[]byte]
	current.Store(&jwksA)

	var fetches atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		body := *current.Load()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	// Tests use a near-zero rate-limit so kid-miss refresh fires
	// without a real-time wall-clock wait. Production default is 30s
	// — the testing seam is the package-level var.
	defer setKidMissRefreshInterval(0)()

	auth, err := New(Config{
		Mode:        ModeOIDC,
		OIDCIssuer:  issuer,
		OIDCJWKSURL: ts.URL,
		HTTPClient:  ts.Client(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	tokA := signJWT(t, privA, "kA", map[string]any{
		"iss": issuer,
		"sub": "alice",
		"exp": now + 300,
	})
	if _, err := authenticate(t, auth, tokA); err != nil {
		t.Fatalf("kA verify (pre-rotation) failed: %v", err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("expected 1 fetch after priming with kA, got %d", got)
	}

	// Operator rotates: JWKS server now returns kB.
	current.Store(&jwksB)

	tokB := signJWT(t, privB, "kB", map[string]any{
		"iss": issuer,
		"sub": "alice",
		"exp": now + 300,
	})
	if _, err := authenticate(t, auth, tokB); err != nil {
		t.Fatalf("kB verify after rotation failed; kid-miss refresh did not pick up the new JWKS: %v", err)
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("expected 2 fetches after rotation (initial + kid-miss refresh), got %d", got)
	}
}

// TestJWKSCache_KidMissRateLimited locks the rate-limit gate. A flood
// of tokens carrying an unknown kid must NOT amplify into a flood of
// JWKS fetches — that would let an attacker DoS the IdP via the
// authenticator. The test sends six unknown-kid tokens within a 200ms
// window; only one refresh fires (the first). Past the window, a
// fresh kid-miss must trigger another refresh.
func TestJWKSCache_KidMissRateLimited(t *testing.T) {
	const issuer = "https://issuer.example.test"

	privA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa A: %v", err)
	}
	privB, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa B: %v", err)
	}

	// JWKS only ever publishes kA — kB never appears, so every kid-miss
	// refresh ends in not-found. That isolates the rate-limit invariant
	// from the success path.
	jwks := buildJWKS(t, "kA", &privA.PublicKey)
	var fetches atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	const window = 200 * time.Millisecond
	defer setKidMissRefreshInterval(window)()

	auth, err := New(Config{
		Mode:        ModeOIDC,
		OIDCIssuer:  issuer,
		OIDCJWKSURL: ts.URL,
		HTTPClient:  ts.Client(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now().Unix()
	tokA := signJWT(t, privA, "kA", map[string]any{
		"iss": issuer,
		"sub": "alice",
		"exp": now + 300,
	})
	if _, err := authenticate(t, auth, tokA); err != nil {
		t.Fatalf("kA prime failed: %v", err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("expected 1 fetch after priming, got %d", got)
	}

	mkUnknown := func(salt int64) string {
		return signJWT(t, privB, "kB", map[string]any{
			"iss": issuer,
			"sub": "alice",
			"exp": now + 300,
			"iat": salt,
		})
	}

	// First kid-miss: rate-limit window crossed (since c.lastReload
	// was set during priming and we set the window to 200ms — the
	// prime is ~immediate, so we sleep just past the window first).
	time.Sleep(window + 25*time.Millisecond)
	if _, err := authenticate(t, auth, mkUnknown(1)); err == nil {
		t.Fatal("kid-miss for kB must fail (kB not in JWKS)")
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("expected refresh on first kid-miss past window: got %d, want 2", got)
	}

	// Five subsequent kid-misses inside the rate-limit window must
	// not refresh. The second authenticate(...) above bumped
	// c.lastReload, so we are now inside the 200ms window again.
	for i := range 5 {
		if _, err := authenticate(t, auth, mkUnknown(int64(10+i))); err == nil {
			t.Fatal("kid-miss for kB must fail (kB not in JWKS)")
		}
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("rate-limit allowed extra fetches inside window: got %d, want 2", got)
	}

	// Past the window, a fresh kid-miss must trigger another refresh.
	time.Sleep(window + 25*time.Millisecond)
	if _, err := authenticate(t, auth, mkUnknown(99)); err == nil {
		t.Fatal("kid-miss for kB must fail (kB not in JWKS)")
	}
	if got := fetches.Load(); got != 3 {
		t.Fatalf("expected refresh after window expired: got %d, want 3", got)
	}
}

// setKidMissRefreshInterval swaps the package-level rate-limit and
// returns a restore func. Used as `defer setKidMissRefreshInterval(0)()`
// to keep tests fast without leaking the override into other suites.
func setKidMissRefreshInterval(d time.Duration) func() {
	prev := jwksKidMissRefreshMinInterval
	jwksKidMissRefreshMinInterval = d
	return func() { jwksKidMissRefreshMinInterval = prev }
}

// TestJWKPublicKey_RejectsShortRSAModulus pins the F3 invariant:
// jwkPublicKey must refuse RSA keys whose modulus is below 2048
// bits. PKCS1v15 verification is mathematically sound at any bit
// length, so without this check a JWKS that publishes a 1024-bit
// modulus produces a verifier that accepts forged tokens after the
// modulus is factored — feasible in hours/days for ≤1024-bit RSA on
// commodity hardware. NIST SP 800-131A and RFC 7518 §6.3.2 require
// ≥2048.
func TestJWKPublicKey_RejectsShortRSAModulus(t *testing.T) {
	short, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa-1024: %v", err)
	}
	nEnc := base64.RawURLEncoding.EncodeToString(short.N.Bytes())
	eEnc := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(short.E)).Bytes())
	_, err = jwkPublicKey("RSA", nEnc, eEnc, "", "", "")
	if err == nil {
		t.Fatal("expected jwkPublicKey to reject 1024-bit RSA modulus")
	}
	if !strings.Contains(err.Error(), "modulus") {
		t.Fatalf("expected error to mention modulus, got: %v", err)
	}
}

// TestJWKPublicKey_RejectsSmallRSAExponent pins the F7 invariant for
// a too-small exponent: e=1 makes encryption the identity function
// (every "ciphertext" equals the plaintext), so any signature
// verifier built on it accepts arbitrary forgeries. RFC 7518 §6.3.1.2
// permits e≥3; real IdPs publish 65537. Pre-fix only e=0 / empty /
// overflow were rejected, leaving e=1 silently accepted.
func TestJWKPublicKey_RejectsSmallRSAExponent(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	nEnc := base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
	for _, e := range []int{1, 2} {
		eEnc := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(e)).Bytes())
		_, err := jwkPublicKey("RSA", nEnc, eEnc, "", "", "")
		if err == nil {
			t.Fatalf("expected jwkPublicKey to reject exponent %d", e)
		}
		if !strings.Contains(err.Error(), "exponent") {
			t.Fatalf("e=%d: expected error to mention exponent, got: %v", e, err)
		}
	}
}

// TestJWKPublicKey_RejectsEvenRSAExponent pins the F7 parity invariant:
// a valid RSA encryption exponent must be coprime to lambda(N). Since
// lambda(N) is always even for non-trivial N, an even e violates the
// gcd=1 constraint and breaks RSA's invertibility. e=4 (and any even
// value) must be rejected. Pre-fix the checks looked only at zero/
// negative/overflow and any odd-or-even positive value was accepted.
func TestJWKPublicKey_RejectsEvenRSAExponent(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	nEnc := base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
	for _, e := range []int{4, 6, 256} {
		eEnc := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(e)).Bytes())
		_, err := jwkPublicKey("RSA", nEnc, eEnc, "", "", "")
		if err == nil {
			t.Fatalf("expected jwkPublicKey to reject even exponent %d", e)
		}
		if !strings.Contains(err.Error(), "exponent") {
			t.Fatalf("e=%d: expected error to mention exponent, got: %v", e, err)
		}
	}
}

// TestJWKPublicKey_RejectsOffCurveECPoint pins the F4 invariant:
// jwkPublicKey must validate that (X, Y) is a point on the named
// curve at parse time. ecdsa.VerifyASN1 (Go ≥ 1.20) does check curve
// membership, so an off-curve key in the JWKS would already fail
// signature verification — but the gap is that jwkPublicKey happily
// returns a malformed *ecdsa.PublicKey, leaving any future caller
// that does not go through ecdsa.VerifyASN1 (a custom verifier, a
// non-ECDSA consumer) silently accepting an unverifiable key.
//
// Construction: take the X coordinate of a real P-256 key but flip a
// byte in Y so the resulting (X, Y) is no longer on the curve.
func TestJWKPublicKey_RejectsOffCurveECPoint(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ec: %v", err)
	}
	xb := ecKey.X.Bytes()
	yb := ecKey.Y.Bytes()
	// Flip one byte in Y to put the point off the curve.
	yb[len(yb)/2] ^= 0xFF

	xEnc := base64.RawURLEncoding.EncodeToString(xb)
	yEnc := base64.RawURLEncoding.EncodeToString(yb)
	_, err = jwkPublicKey("EC", "", "", xEnc, yEnc, "P-256")
	if err == nil {
		t.Fatal("expected jwkPublicKey to reject off-curve EC point")
	}
	if !strings.Contains(err.Error(), "curve") && !strings.Contains(err.Error(), "point") {
		t.Fatalf("expected error to mention curve/point, got: %v", err)
	}
}

// TestJWKPublicKey_RejectsZeroECPoint exercises the special case
// (0, 0): a point that no NIST curve contains and that some naive
// verifiers accidentally treat as the point at infinity. The on-
// curve check (and crypto/ecdh's NewPublicKey) reject it explicitly.
func TestJWKPublicKey_RejectsZeroECPoint(t *testing.T) {
	zero := base64.RawURLEncoding.EncodeToString([]byte{0})
	_, err := jwkPublicKey("EC", "", "", zero, zero, "P-256")
	if err == nil {
		t.Fatal("expected jwkPublicKey to reject (0, 0)")
	}
}

// TestJWKSCache_ConfigurableTTL pins the F5 invariant: an operator
// who sets Config.OIDCJWKSCacheTTL (wired from MCP_OIDC_JWKS_CACHE_TTL)
// gets that TTL applied to the JWKS-document cache, not the
// hardcoded 5-minute default. Pre-fix the cache always lived for 5
// minutes regardless of operator config — fine for a single-tenant
// dev setup, too coarse for a hosted service whose IdP rotates keys
// hourly. Combined with F2 (kid-miss refresh) the JWKS path is now
// fully tunable.
func TestJWKSCache_ConfigurableTTL(t *testing.T) {
	const issuer = "https://issuer.example.test"

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	jwks := buildJWKS(t, "kA", &priv.PublicKey)
	var fetches atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	// The MinTTL clamp would otherwise pin our 100ms to 1 minute; the
	// var seam (matching the kid-miss-refresh pattern) lets the test
	// drive the TTL boundary in milliseconds. Production never mutates it.
	defer func(orig time.Duration) { jwksCacheMinTTL = orig }(jwksCacheMinTTL)
	jwksCacheMinTTL = time.Millisecond

	const ttl = 100 * time.Millisecond
	auth, err := New(Config{
		Mode:             ModeOIDC,
		OIDCIssuer:       issuer,
		OIDCJWKSURL:      ts.URL,
		OIDCJWKSCacheTTL: ttl,
		HTTPClient:       ts.Client(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sign := func(salt int64) string {
		return signJWT(t, priv, "kA", map[string]any{
			"iss": issuer,
			"sub": "alice",
			"exp": time.Now().Unix() + 300,
			"iat": salt,
		})
	}

	// First call primes the cache via a single fetch.
	if _, err := authenticate(t, auth, sign(1)); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("expected 1 fetch after prime, got %d", got)
	}

	// Second call inside the TTL window must hit the cache — different
	// token bytes (salted iat) bypass the verify cache, so this isolates
	// jwksCache reuse. If F5 wired the TTL correctly, no extra fetch.
	if _, err := authenticate(t, auth, sign(2)); err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("TTL-cached call must not fetch, got %d", got)
	}

	// Past the TTL the cache must reload. Pre-F5 the cache lived 5min
	// regardless and would still see 1 fetch. F6 turned the post-TTL
	// refresh async (stale-while-revalidate) so the verify call
	// returns immediately and the refresh fires in the background —
	// poll briefly for fetches=2.
	time.Sleep(ttl + 75*time.Millisecond)
	if _, err := authenticate(t, auth, sign(3)); err != nil {
		t.Fatalf("third verify: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && fetches.Load() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("expected refresh after TTL, got %d", got)
	}
}

// TestJWKSCache_StaleServesDuringRefresh pins the F6 stale-while-
// revalidate invariant: when the cache holds a kid the request asks
// for, an expired-TTL lookup must return the (stale) key
// immediately and kick the refresh in the background. Pre-F6 the
// caller paid the full JWKS HTTP round trip (up to 5s) under the
// cache mutex, blocking every concurrent verify.
func TestJWKSCache_StaleServesDuringRefresh(t *testing.T) {
	const issuer = "https://issuer.example.test"

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	jwks := buildJWKS(t, "kA", &priv.PublicKey)

	var fetches atomic.Int32
	const slowDelay = 750 * time.Millisecond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := fetches.Add(1)
		if n > 1 {
			// Slow path on every refresh after the prime so a
			// pre-F6 synchronous reload would block the caller.
			time.Sleep(slowDelay)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer ts.Close()

	defer func(orig time.Duration) { jwksCacheMinTTL = orig }(jwksCacheMinTTL)
	jwksCacheMinTTL = time.Millisecond

	auth, err := New(Config{
		Mode:             ModeOIDC,
		OIDCIssuer:       issuer,
		OIDCJWKSURL:      ts.URL,
		OIDCJWKSCacheTTL: 50 * time.Millisecond,
		HTTPClient:       ts.Client(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sign := func(salt int64) string {
		return signJWT(t, priv, "kA", map[string]any{
			"iss": issuer,
			"sub": "alice",
			"exp": time.Now().Unix() + 300,
			"iat": salt,
		})
	}

	// Prime cache (first fetch is fast).
	if _, err := authenticate(t, auth, sign(1)); err != nil {
		t.Fatalf("prime verify: %v", err)
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("expected 1 fetch after prime, got %d", got)
	}

	// Wait past TTL — the cache is now stale.
	time.Sleep(75 * time.Millisecond)

	// Stale-while-revalidate: this call must return immediately with
	// the cached kA and kick the refresh asynchronously. Pre-F6 it
	// would have paid `slowDelay` waiting on the mutex-held HTTP fetch.
	start := time.Now()
	if _, err := authenticate(t, auth, sign(2)); err != nil {
		t.Fatalf("stale verify: %v", err)
	}
	elapsed := time.Since(start)
	const fastBudget = 200 * time.Millisecond
	if elapsed > fastBudget {
		t.Fatalf("stale-while-revalidate broke: call took %s; pre-F6 it would have been ~%s", elapsed, slowDelay)
	}

	// The async refresh must eventually fire so the next TTL window
	// gets fresh material. Poll briefly — the slow path sleeps 750ms.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && fetches.Load() < 2 {
		time.Sleep(25 * time.Millisecond)
	}
	if got := fetches.Load(); got < 2 {
		t.Fatalf("async refresh never fired: fetches=%d, want ≥2", got)
	}
}

// TestJWKSCache_SingleFlightCoalescesConcurrentReloads pins the F6
// single-flight invariant: when N concurrent verify calls arrive on
// an empty cache, only ONE JWKS HTTP fetch fires; the rest wait for
// it. Pre-F6 the cache mutex serialised the requests but each one
// re-entered reload(); with a slow IdP, every concurrent call paid
// the round-trip latency.
func TestJWKSCache_SingleFlightCoalescesConcurrentReloads(t *testing.T) {
	const issuer = "https://issuer.example.test"

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	jwks := buildJWKS(t, "kA", &priv.PublicKey)

	var fetches atomic.Int32
	const fetchDelay = 200 * time.Millisecond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		time.Sleep(fetchDelay)
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

	const N = 20
	var wg sync.WaitGroup
	errs := make(chan error, N)
	start := time.Now()
	for i := range N {
		wg.Add(1)
		go func(salt int64) {
			defer wg.Done()
			tok := signJWT(t, priv, "kA", map[string]any{
				"iss": issuer,
				"sub": "alice",
				"exp": time.Now().Unix() + 300,
				"iat": salt,
			})
			_, err := authenticate(t, auth, tok)
			errs <- err
		}(int64(i))
	}
	wg.Wait()
	close(errs)
	elapsed := time.Since(start)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent verify failed: %v", err)
		}
	}

	if got := fetches.Load(); got != 1 {
		t.Fatalf("single-flight broke: got %d JWKS fetches, want 1", got)
	}
	// Concurrent callers should all complete within roughly the
	// fetch window plus signing overhead. Pre-F6 with mutex held
	// during reload, N callers serialise → N × fetchDelay = ~4s for
	// N=20. Generous ceiling here covers slow CI; the discriminating
	// signal is fetches==1.
	const ceiling = 1500 * time.Millisecond
	if elapsed > ceiling {
		t.Fatalf("concurrent calls did not coalesce: took %s for %d callers (fetch was %s)", elapsed, N, fetchDelay)
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
