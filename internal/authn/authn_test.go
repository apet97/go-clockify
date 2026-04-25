package authn

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestNewDefaults verifies New() applies defaults to empty Config fields
// across every supported mode and rejects unknown modes / missing tokens.
func TestNewDefaults(t *testing.T) {
	t.Run("static_bearer_default_mode", func(t *testing.T) {
		auth, err := New(Config{BearerToken: "abcdef0123456789"})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := auth.(staticBearerAuthenticator); !ok {
			t.Fatalf("expected staticBearerAuthenticator, got %T", auth)
		}
	})
	t.Run("static_bearer_missing_token", func(t *testing.T) {
		if _, err := New(Config{Mode: ModeStaticBearer}); err == nil {
			t.Fatal("expected error for missing token")
		}
	})
	t.Run("forward_auth_defaults", func(t *testing.T) {
		auth, err := New(Config{Mode: ModeForwardAuth})
		if err != nil {
			t.Fatal(err)
		}
		fa, ok := auth.(forwardAuthAuthenticator)
		if !ok {
			t.Fatalf("expected forwardAuthAuthenticator, got %T", auth)
		}
		if fa.cfg.ForwardSubjectHeader != "X-Forwarded-User" || fa.cfg.ForwardTenantHeader != "X-Forwarded-Tenant" {
			t.Fatalf("default headers not applied: %+v", fa.cfg)
		}
		if fa.cfg.DefaultTenantID != "default" {
			t.Fatalf("default tenant id not applied: %q", fa.cfg.DefaultTenantID)
		}
	})
	t.Run("mtls_defaults", func(t *testing.T) {
		auth, err := New(Config{Mode: ModeMTLS})
		if err != nil {
			t.Fatal(err)
		}
		m, ok := auth.(mtlsAuthenticator)
		if !ok {
			t.Fatalf("expected mtlsAuthenticator, got %T", auth)
		}
		if m.cfg.MTLSTenantHeader != "X-Tenant-ID" {
			t.Fatalf("default mtls tenant header not applied: %q", m.cfg.MTLSTenantHeader)
		}
	})
	t.Run("oidc_requires_issuer", func(t *testing.T) {
		if _, err := New(Config{Mode: ModeOIDC}); err == nil {
			t.Fatal("expected error for missing OIDC issuer")
		}
	})
	t.Run("oidc_default_jwks_url", func(t *testing.T) {
		auth, err := New(Config{Mode: ModeOIDC, OIDCIssuer: "https://issuer.example.com/"})
		if err != nil {
			t.Fatal(err)
		}
		o, ok := auth.(oidcAuthenticator)
		if !ok {
			t.Fatalf("expected oidcAuthenticator, got %T", auth)
		}
		want := "https://issuer.example.com/.well-known/jwks.json"
		if o.cache.url != want {
			t.Fatalf("default JWKS URL: got %q want %q", o.cache.url, want)
		}
	})
	// Strict mode must bind tokens to this server. Without an audience or
	// resource URI configured, validateClaims has no value to require in
	// the aud claim, so a token issued by the trusted issuer for a
	// different relying party would still be accepted. internal/config
	// enforces the same invariant on the documented startup path; this
	// guard catches programmatic embedders that build authn.Config
	// directly. Both audience-only and resource-only configs satisfy the
	// requirement.
	t.Run("oidc_strict_requires_audience_or_resource", func(t *testing.T) {
		_, err := New(Config{
			Mode:       ModeOIDC,
			OIDCIssuer: "https://issuer.example.com/",
			OIDCStrict: true,
		})
		if err == nil {
			t.Fatal("expected error for OIDC strict mode without audience or resource URI")
		}
		if !strings.Contains(err.Error(), "OIDCAudience or OIDCResourceURI") {
			t.Fatalf("expected strict-mode validation error, got: %v", err)
		}
	})
	t.Run("oidc_strict_with_audience_ok", func(t *testing.T) {
		_, err := New(Config{
			Mode:         ModeOIDC,
			OIDCIssuer:   "https://issuer.example.com/",
			OIDCStrict:   true,
			OIDCAudience: "clockify-mcp",
		})
		if err != nil {
			t.Fatalf("strict mode with audience should succeed: %v", err)
		}
	})
	t.Run("oidc_strict_with_resource_uri_ok", func(t *testing.T) {
		_, err := New(Config{
			Mode:            ModeOIDC,
			OIDCIssuer:      "https://issuer.example.com/",
			OIDCStrict:      true,
			OIDCResourceURI: "https://mcp.example.com/",
		})
		if err != nil {
			t.Fatalf("strict mode with resource URI should succeed: %v", err)
		}
	})
	// Permissive mode (default) deliberately does NOT require an audience
	// — operators who haven't opted into strict mode can run with
	// issuer-only validation. This codifies that the new strict-mode
	// guard does not regress permissive callers.
	t.Run("oidc_permissive_without_audience_ok", func(t *testing.T) {
		_, err := New(Config{
			Mode:       ModeOIDC,
			OIDCIssuer: "https://issuer.example.com/",
		})
		if err != nil {
			t.Fatalf("permissive mode without audience should succeed: %v", err)
		}
	})
	t.Run("unsupported_mode", func(t *testing.T) {
		if _, err := New(Config{Mode: Mode("ldap")}); err == nil {
			t.Fatal("expected error for unsupported mode")
		}
	})
}

// TestStaticBearerAuthenticate exercises the bearer-token comparison happy
// path and the missing/invalid-token error branches.
func TestStaticBearerAuthenticate(t *testing.T) {
	auth, err := New(Config{Mode: ModeStaticBearer, BearerToken: "right-token-1234"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, err := auth.Authenticate(context.Background(), req); err == nil {
			t.Fatal("expected missing-token error")
		}
	})
	t.Run("wrong_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		if _, err := auth.Authenticate(context.Background(), req); err == nil {
			t.Fatal("expected invalid-token error")
		}
	})
	t.Run("right_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer right-token-1234")
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.Subject != "static-bearer" || p.AuthMode != ModeStaticBearer {
			t.Fatalf("principal wrong: %+v", p)
		}
		if p.TenantID != "default" {
			t.Fatalf("default tenant: got %q", p.TenantID)
		}
	})
}

// TestForwardAuthAuthenticate covers the header-based identity propagation.
func TestForwardAuthAuthenticate(t *testing.T) {
	auth, err := New(Config{Mode: ModeForwardAuth, DefaultTenantID: "default-tenant"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("missing_subject_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, err := auth.Authenticate(context.Background(), req); err == nil {
			t.Fatal("expected missing-header error")
		}
	})
	t.Run("subject_only_uses_default_tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-User", "alice@example.com")
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.Subject != "alice@example.com" || p.TenantID != "default-tenant" || p.AuthMode != ModeForwardAuth {
			t.Fatalf("principal wrong: %+v", p)
		}
	})
	t.Run("explicit_tenant_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-User", "bob@example.com")
		req.Header.Set("X-Forwarded-Tenant", "team-bravo")
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.TenantID != "team-bravo" {
			t.Fatalf("tenant: got %q", p.TenantID)
		}
	})
}

// TestMTLSAuthenticate covers all dispatch branches in mtlsAuthenticator
// without spinning up a real TLS handshake — we craft a Request with a
// fabricated *tls.ConnectionState that satisfies VerifiedChains.
func TestMTLSAuthenticate(t *testing.T) {
	auth, err := New(Config{Mode: ModeMTLS, DefaultTenantID: "tenant-default"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("missing_tls", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, err := auth.Authenticate(context.Background(), req); err == nil {
			t.Fatal("expected missing-cert error")
		}
	})

	leaf := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "alice",
			Organization: []string{"Acme Corp"},
		},
	}

	t.Run("uses_organization_for_tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.TLS = &tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{{leaf}},
		}
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.Subject != "alice" || p.TenantID != "Acme Corp" || p.AuthMode != ModeMTLS {
			t.Fatalf("principal wrong: %+v", p)
		}
	})

	t.Run("explicit_tenant_header_overrides_org", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "team-charlie")
		req.TLS = &tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{{leaf}},
		}
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.TenantID != "team-charlie" {
			t.Fatalf("tenant: got %q", p.TenantID)
		}
	})

	t.Run("falls_back_to_default_tenant", func(t *testing.T) {
		bare := &x509.Certificate{Subject: pkix.Name{CommonName: "noorg"}}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.TLS = &tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{{bare}},
		}
		p, err := auth.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if p.TenantID != "tenant-default" {
			t.Fatalf("default tenant: got %q", p.TenantID)
		}
	})
}

// TestDecodeJWT covers decodeJWT directly with hand-crafted tokens so the
// happy path and each error branch (wrong segment count, bad base64, bad
// JSON in header / claims / signature) are all exercised without needing
// real signing keys or an HTTP fixture.
func TestDecodeJWT(t *testing.T) {
	enc := func(s string) string { return base64URL(s) }

	header := enc(`{"alg":"RS256","kid":"k1"}`)
	claims := enc(`{"iss":"https://issuer.example.com","sub":"alice","aud":"clockify","exp":9999999999,"nbf":1,"iat":1}`)
	sig := enc("signature-bytes")

	t.Run("happy_path", func(t *testing.T) {
		token := header + "." + claims + "." + sig
		h, c, signed, sigBytes, err := decodeJWT(token)
		if err != nil {
			t.Fatalf("decodeJWT: %v", err)
		}
		if h.Alg != "RS256" || h.KID != "k1" {
			t.Fatalf("header: %+v", h)
		}
		if c.Issuer != "https://issuer.example.com" || c.Subject != "alice" {
			t.Fatalf("claims: %+v", c)
		}
		if len(c.Audience) != 1 || c.Audience[0] != "clockify" {
			t.Fatalf("audience: %+v", c.Audience)
		}
		if signed != header+"."+claims {
			t.Fatalf("signed payload mismatch")
		}
		if string(sigBytes) != "signature-bytes" {
			t.Fatalf("sig bytes: %q", string(sigBytes))
		}
		if c.Raw["sub"] != "alice" {
			t.Fatalf("raw claims missing sub: %+v", c.Raw)
		}
	})

	t.Run("wrong_segment_count", func(t *testing.T) {
		if _, _, _, _, err := decodeJWT("a.b"); err == nil {
			t.Fatal("expected segment-count error")
		}
	})

	t.Run("bad_base64_header", func(t *testing.T) {
		token := "$$bad$$." + claims + "." + sig
		if _, _, _, _, err := decodeJWT(token); err == nil {
			t.Fatal("expected base64 error")
		}
	})

	t.Run("bad_json_header", func(t *testing.T) {
		token := enc("not-json") + "." + claims + "." + sig
		if _, _, _, _, err := decodeJWT(token); err == nil {
			t.Fatal("expected JSON header error")
		}
	})

	t.Run("bad_json_claims", func(t *testing.T) {
		token := header + "." + enc("not-json") + "." + sig
		if _, _, _, _, err := decodeJWT(token); err == nil {
			t.Fatal("expected JSON claims error")
		}
	})

	t.Run("bad_base64_signature", func(t *testing.T) {
		token := header + "." + claims + ".$$bad$$"
		if _, _, _, _, err := decodeJWT(token); err == nil {
			t.Fatal("expected sig base64 error")
		}
	})
}

// TestValidateClaims exercises issuer + audience + exp/nbf checks.
func TestValidateClaims(t *testing.T) {
	cfg := Config{OIDCIssuer: "https://issuer.example.com", OIDCAudience: "clockify"}

	t.Run("happy_path", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"clockify"}, Expires: 9999999999, NotBefore: 1}
		if err := validateClaims(c, cfg); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
	t.Run("wrong_issuer", func(t *testing.T) {
		c := jwtClaims{Issuer: "https://other.example.com"}
		if err := validateClaims(c, cfg); err == nil {
			t.Fatal("expected issuer error")
		}
	})
	t.Run("wrong_audience", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"different-aud"}}
		if err := validateClaims(c, cfg); err == nil {
			t.Fatal("expected audience error")
		}
	})
	t.Run("expired", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"clockify"}, Expires: 1}
		if err := validateClaims(c, cfg); err == nil {
			t.Fatal("expected expired error")
		}
	})
	t.Run("not_yet_valid", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"clockify"}, Expires: 9999999999, NotBefore: 9999999999}
		if err := validateClaims(c, cfg); err == nil {
			t.Fatal("expected nbf error")
		}
	})
	t.Run("audience_unset_skips_check", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Expires: 9999999999}
		open := Config{OIDCIssuer: cfg.OIDCIssuer}
		if err := validateClaims(c, open); err != nil {
			t.Fatalf("audience-less config should pass: %v", err)
		}
	})
	t.Run("strict_rejects_missing_exp", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"clockify"}}
		strict := Config{OIDCIssuer: cfg.OIDCIssuer, OIDCAudience: "clockify", OIDCStrict: true}
		err := validateClaims(c, strict)
		if err == nil {
			t.Fatal("expected exp=0 to be rejected in strict mode")
		}
		if !strings.Contains(err.Error(), "missing exp") {
			t.Fatalf("expected exp-missing error, got %v", err)
		}
	})
	t.Run("non_strict_accepts_missing_exp", func(t *testing.T) {
		c := jwtClaims{Issuer: cfg.OIDCIssuer, Audience: claimAudience{"clockify"}}
		open := Config{OIDCIssuer: cfg.OIDCIssuer, OIDCAudience: "clockify"}
		if err := validateClaims(c, open); err != nil {
			t.Fatalf("non-strict should accept missing exp (back-compat): %v", err)
		}
	})
}

// TestClaimAudienceUnmarshalJSON covers both shapes the spec allows: a single
// string and an array of strings.
func TestClaimAudienceUnmarshalJSON(t *testing.T) {
	var single claimAudience
	if err := single.UnmarshalJSON([]byte(`"only"`)); err != nil {
		t.Fatal(err)
	}
	if len(single) != 1 || single[0] != "only" {
		t.Fatalf("single: %+v", single)
	}
	var many claimAudience
	if err := many.UnmarshalJSON([]byte(`["a","b"]`)); err != nil {
		t.Fatal(err)
	}
	if len(many) != 2 || many[0] != "a" || many[1] != "b" {
		t.Fatalf("many: %+v", many)
	}
	var bad claimAudience
	if err := bad.UnmarshalJSON([]byte(`{"oops":1}`)); err == nil {
		t.Fatal("expected error on object")
	}
}

// TestClaimString covers the helper that pulls trimmed string claims out of
// the raw claims map.
func TestClaimString(t *testing.T) {
	raw := map[string]any{"sub": "  alice  ", "missing": nil, "wrong_type": 42}
	if claimString(raw, "sub") != "alice" {
		t.Fatal("trimmed sub")
	}
	if claimString(raw, "missing") != "" {
		t.Fatal("missing should be empty")
	}
	if claimString(raw, "wrong_type") != "" {
		t.Fatal("wrong type should be empty")
	}
}

func base64URL(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// TestJWKPublicKey covers the JWK → crypto.PublicKey conversion for both
// supported key types and the unsupported-kty error branch.
func TestJWKPublicKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaN := base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
	rsaE := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes())

	got, err := jwkPublicKey("RSA", rsaN, rsaE, "", "", "")
	if err != nil {
		t.Fatalf("RSA: %v", err)
	}
	if rk, ok := got.(*rsa.PublicKey); !ok || rk.N.Cmp(rsaKey.N) != 0 || rk.E != rsaKey.E {
		t.Fatalf("RSA round-trip wrong: %#v", got)
	}

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecX := base64.RawURLEncoding.EncodeToString(ecKey.X.Bytes())
	ecY := base64.RawURLEncoding.EncodeToString(ecKey.Y.Bytes())
	got, err = jwkPublicKey("EC", "", "", ecX, ecY, "P-256")
	if err != nil {
		t.Fatalf("EC: %v", err)
	}
	if ek, ok := got.(*ecdsa.PublicKey); !ok || ek.X.Cmp(ecKey.X) != 0 || ek.Y.Cmp(ecKey.Y) != 0 {
		t.Fatalf("EC round-trip wrong: %#v", got)
	}

	if _, err := jwkPublicKey("oct", "", "", "", "", ""); err == nil {
		t.Fatal("expected unsupported-kty error")
	}
	if _, err := jwkPublicKey("RSA", "$$bad$$", rsaE, "", "", ""); err == nil {
		t.Fatal("expected RSA n decode error")
	}
	if _, err := jwkPublicKey("RSA", rsaN, "", "", "", ""); err == nil {
		t.Fatal("expected RSA empty exponent error")
	}
	if _, err := jwkPublicKey("RSA", rsaN, base64.RawURLEncoding.EncodeToString([]byte{0}), "", "", ""); err == nil {
		t.Fatal("expected RSA zero exponent error")
	}
	if strconv.IntSize >= 64 {
		largeExp := uint64(1<<32 + 3)
		largeExpBytes := new(big.Int).SetUint64(largeExp).Bytes()
		got, err := jwkPublicKey("RSA", rsaN, base64.RawURLEncoding.EncodeToString(largeExpBytes), "", "", "")
		if err != nil {
			t.Fatalf("RSA large exponent: %v", err)
		}
		rk, ok := got.(*rsa.PublicKey)
		if !ok {
			t.Fatalf("RSA large exponent type: got %T", got)
		}
		if rk.E != int(largeExp) {
			t.Fatalf("RSA large exponent mismatch: got %d want %d", rk.E, largeExp)
		}
	}
	overflowExp := make([]byte, strconv.IntSize/8+1)
	overflowExp[0] = 0x01
	if _, err := jwkPublicKey("RSA", rsaN, base64.RawURLEncoding.EncodeToString(overflowExp), "", "", ""); err == nil || !strings.Contains(err.Error(), "overflows int") {
		t.Fatalf("expected RSA exponent overflow error, got %v", err)
	}
	if _, err := jwkPublicKey("EC", "", "", "$$bad$$", ecY, "P-256"); err == nil {
		t.Fatal("expected EC x decode error")
	}
}

// TestCurveFor covers the well-known curve names and the default branch.
func TestCurveFor(t *testing.T) {
	cases := []struct {
		name string
		want elliptic.Curve
	}{
		{"P-256", elliptic.P256()},
		{"P-384", elliptic.P384()},
		{"P-521", elliptic.P521()},
		{"unknown", elliptic.P256()}, // default fallback
	}
	for _, tc := range cases {
		got := curveFor(tc.name)
		if got != tc.want {
			t.Fatalf("curveFor(%q): got %T, want %T", tc.name, got, tc.want)
		}
	}
}

// TestHashForAlg covers each supported alg + the unsupported error branch.
func TestHashForAlg(t *testing.T) {
	cases := []struct {
		alg     string
		want    crypto.Hash
		wantLen int
	}{
		{"RS256", crypto.SHA256, 32},
		{"ES256", crypto.SHA256, 32},
		{"RS384", crypto.SHA384, 48},
		{"ES384", crypto.SHA384, 48},
		{"RS512", crypto.SHA512, 64},
		{"ES512", crypto.SHA512, 64},
	}
	for _, tc := range cases {
		t.Run(tc.alg, func(t *testing.T) {
			sum, h, err := hashForAlg(tc.alg, "payload")
			if err != nil {
				t.Fatal(err)
			}
			if h != tc.want {
				t.Fatalf("hash: got %v want %v", h, tc.want)
			}
			if len(sum) != tc.wantLen {
				t.Fatalf("sum length: got %d want %d", len(sum), tc.wantLen)
			}
		})
	}
	if _, _, err := hashForAlg("HS256", "payload"); err == nil {
		t.Fatal("expected unsupported alg error")
	}
}

// TestVerifyJWTRSARoundTrip generates an RSA key, signs a JWT payload, and
// verifies it via verifyJWT to cover the RSA branch end-to-end.
func TestVerifyJWTRSARoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signed := "header.claims"
	hashed := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyJWT("RS256", &key.PublicKey, signed, sig); err != nil {
		t.Fatalf("RSA verify: %v", err)
	}
	// Tamper to confirm verification fails.
	if err := verifyJWT("RS256", &key.PublicKey, signed+"-tamper", sig); err == nil {
		t.Fatal("expected verify to fail on tampered payload")
	}
	// Unsupported key type.
	if err := verifyJWT("RS256", "not-a-key", signed, sig); err == nil {
		t.Fatal("expected unsupported key type error")
	}
}

// TestBearerTokenParsing exercises the bearerToken helper directly so its
// edge cases (no header, wrong scheme, empty token) are visible in coverage.
func TestBearerTokenParsing(t *testing.T) {
	t.Run("no_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, ok := bearerToken(req); ok {
			t.Fatal("expected ok=false")
		}
	})
	t.Run("wrong_scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		if _, ok := bearerToken(req); ok {
			t.Fatal("expected ok=false")
		}
	})
	t.Run("empty_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer    ")
		if _, ok := bearerToken(req); ok {
			t.Fatal("expected ok=false")
		}
	})
	t.Run("good", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer my-token")
		got, ok := bearerToken(req)
		if !ok || got != "my-token" {
			t.Fatalf("got %q ok=%v", got, ok)
		}
	})
}
