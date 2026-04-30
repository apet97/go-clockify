package authn

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

type Mode string

const (
	ModeStaticBearer Mode = "static_bearer"
	ModeOIDC         Mode = "oidc"
	ModeForwardAuth  Mode = "forward_auth"
	ModeMTLS         Mode = "mtls"
)

type Principal struct {
	Subject   string
	TenantID  string
	AuthMode  Mode
	Claims    map[string]string
	SessionID string
}

type Config struct {
	Mode                 Mode
	BearerToken          string
	DefaultTenantID      string
	TenantClaim          string
	SubjectClaim         string
	OIDCIssuer           string
	OIDCAudience         string
	OIDCJWKSURL          string
	OIDCJWKSPath         string
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	// ForwardAuthTrustedProxies, when non-empty, gates the
	// forward_auth authenticator: a request whose direct source
	// (r.RemoteAddr) is not inside one of these networks is
	// rejected before X-Forwarded-User / X-Forwarded-Tenant are
	// inspected. Empty preserves the historical "trust every
	// source" posture for self-hosted single-tenant deployments
	// where the operator owns the network boundary.
	ForwardAuthTrustedProxies []*net.IPNet
	MTLSTenantHeader          string
	// MTLSTenantSource selects how the mtls authenticator derives the
	// tenant identifier. Valid values:
	//   "cert"           — verified client certificate only (URI SAN
	//                      patterns clockify-mcp://tenant/<id> or
	//                      spiffe://.../tenant/<id>, then Subject
	//                      Organization fallback). Default; the only
	//                      sound choice for direct native mTLS because
	//                      a client-controlled header would let any
	//                      authenticated client claim any tenant.
	//   "header"         — request header (MTLSTenantHeader) only.
	//                      Reserve for deployments where an upstream
	//                      proxy terminates mTLS, validates it, and
	//                      stamps the tenant header from a trusted
	//                      source after stripping any client copy.
	//   "header_or_cert" — header first, then cert. Hybrid; mainly
	//                      useful for the brief window of migrating
	//                      from header-based to cert-based identity.
	// Empty string is treated as "cert" (the safe default).
	MTLSTenantSource string
	// RequireMTLSTenant rejects authentication when no tenant could be
	// derived from the configured source(s). Default false retains the
	// historical "fall back to DefaultTenantID" behaviour for
	// self-hosted single-tenant deployments.
	RequireMTLSTenant bool
	// OIDCResourceURI is the canonical resource URI this server represents
	// per RFC 8707 (OAuth 2.0 Resource Indicators) and the MCP OAuth 2.1
	// profile. When set, every OIDC token must list this URI in its `aud`
	// claim — token-binding to the protected resource. Empty disables the
	// extra check (back-compat with the simple OIDCAudience match).
	OIDCResourceURI string
	// OIDCStrict enables hosted-service-grade claim validation: tokens
	// missing an `exp` claim are rejected. Config.Load enforces the
	// audience/resource binding requirement at startup; this flag
	// covers the per-token claim checks. Default false preserves
	// self-hosted behaviour.
	OIDCStrict bool
	// RequireTenantClaim, when true, makes the OIDC authenticator
	// reject any token whose tenant claim is empty — instead of
	// quietly falling back to DefaultTenantID. Default false preserves
	// self-hosted single-tenant behaviour.
	RequireTenantClaim bool
	// OIDCVerifyCacheTTL is the hard ceiling on cached verify results.
	// Zero selects the default (oidcVerifyCacheMaxTTL); values are
	// clamped to [oidcVerifyCacheMinTTL, oidcVerifyCacheTTLCeiling].
	// Larger values amortise the ~54µs verify cost further, but extend
	// the window after a token is revoked before the next Authenticate
	// call re-checks the claims. Operators should pick this
	// consciously; the default stays conservative at 60s.
	OIDCVerifyCacheTTL time.Duration
	// HTTPClient overrides the JWKS fetcher's transport. Tests inject
	// httptest-backed clients here; production code leaves it nil and
	// uses http.DefaultClient.
	HTTPClient *http.Client
}

type Authenticator interface {
	Authenticate(context.Context, *http.Request) (Principal, error)
}

func New(cfg Config) (Authenticator, error) {
	if cfg.DefaultTenantID == "" {
		cfg.DefaultTenantID = "default"
	}
	if cfg.TenantClaim == "" {
		cfg.TenantClaim = "tenant_id"
	}
	if cfg.SubjectClaim == "" {
		cfg.SubjectClaim = "sub"
	}
	if cfg.ForwardTenantHeader == "" {
		cfg.ForwardTenantHeader = "X-Forwarded-Tenant"
	}
	if cfg.ForwardSubjectHeader == "" {
		cfg.ForwardSubjectHeader = "X-Forwarded-User"
	}
	if cfg.MTLSTenantHeader == "" {
		cfg.MTLSTenantHeader = "X-Tenant-ID"
	}
	if cfg.MTLSTenantSource == "" {
		// Default to certificate-derived tenant identity. A header-
		// based default would let any authenticated client claim any
		// tenant by setting X-Tenant-ID, which inverts the trust
		// model of native mTLS.
		cfg.MTLSTenantSource = "cert"
	}
	switch cfg.Mode {
	case "", ModeStaticBearer:
		if cfg.BearerToken == "" {
			return nil, fmt.Errorf("static bearer auth requires a token")
		}
		return staticBearerAuthenticator{cfg: cfg}, nil
	case ModeForwardAuth:
		return forwardAuthAuthenticator{cfg: cfg}, nil
	case ModeMTLS:
		return mtlsAuthenticator{cfg: cfg}, nil
	case ModeOIDC:
		return newOIDCAuthenticator(cfg)
	default:
		return nil, fmt.Errorf("unsupported auth mode %q", cfg.Mode)
	}
}

type staticBearerAuthenticator struct {
	cfg Config
}

func (a staticBearerAuthenticator) Authenticate(_ context.Context, r *http.Request) (Principal, error) {
	token, ok := bearerToken(r)
	if !ok {
		return Principal{}, fmt.Errorf("missing bearer token")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(a.cfg.BearerToken)) != 1 {
		return Principal{}, fmt.Errorf("invalid bearer token")
	}
	return Principal{
		Subject:  "static-bearer",
		TenantID: a.cfg.DefaultTenantID,
		AuthMode: ModeStaticBearer,
		Claims:   map[string]string{},
	}, nil
}

type forwardAuthAuthenticator struct {
	cfg Config
}

func (a forwardAuthAuthenticator) Authenticate(_ context.Context, r *http.Request) (Principal, error) {
	if len(a.cfg.ForwardAuthTrustedProxies) > 0 {
		if err := requireTrustedProxySource(r, a.cfg.ForwardAuthTrustedProxies); err != nil {
			return Principal{}, err
		}
	}
	subject := strings.TrimSpace(r.Header.Get(a.cfg.ForwardSubjectHeader))
	if subject == "" {
		return Principal{}, fmt.Errorf("missing %s header", a.cfg.ForwardSubjectHeader)
	}
	tenant := strings.TrimSpace(r.Header.Get(a.cfg.ForwardTenantHeader))
	if tenant == "" {
		tenant = a.cfg.DefaultTenantID
	}
	return Principal{
		Subject:  subject,
		TenantID: tenant,
		AuthMode: ModeForwardAuth,
		Claims: map[string]string{
			"forward_subject_header": a.cfg.ForwardSubjectHeader,
			"forward_tenant_header":  a.cfg.ForwardTenantHeader,
		},
	}, nil
}

// requireTrustedProxySource enforces the
// MCP_FORWARD_AUTH_TRUSTED_PROXIES allow-list. ChatGPT's audit
// flagged the original forwardAuthAuthenticator as unsafe for any
// internet-facing deployment because it trusted X-Forwarded-User /
// X-Forwarded-Tenant headers from any source — a direct request
// from the public internet could spoof them.
//
// The check inspects r.RemoteAddr, which is the *direct* TCP peer
// the Go HTTP server saw — i.e. the reverse proxy hop, not the
// original client. That is exactly what should be trusted. We do
// NOT walk X-Forwarded-For: the goal is to confirm the proxy that
// actually sent the request is one we trust, not to reconstruct
// the original client IP (which is out of scope for this gate).
func requireTrustedProxySource(r *http.Request, trusted []*net.IPNet) error {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// http.Server always populates RemoteAddr as host:port;
		// a malformed value is a programmer error or a hostile
		// embedder. Refuse to forward-auth anything we can't pin
		// to a network identity.
		return fmt.Errorf("forward_auth: cannot parse RemoteAddr %q: %w", r.RemoteAddr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("forward_auth: cannot parse source IP %q", host)
	}
	for _, n := range trusted {
		if n.Contains(ip) {
			return nil
		}
	}
	return fmt.Errorf("forward_auth: source %s not in MCP_FORWARD_AUTH_TRUSTED_PROXIES allow-list", ip)
}

type mtlsAuthenticator struct {
	cfg Config
}

func (a mtlsAuthenticator) Authenticate(_ context.Context, r *http.Request) (Principal, error) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		return Principal{}, fmt.Errorf("verified mTLS client certificate required")
	}
	leaf := peerLeaf(r.TLS)
	if leaf == nil {
		return Principal{}, fmt.Errorf("missing client certificate")
	}
	subject := strings.TrimSpace(leaf.Subject.CommonName)
	if subject == "" {
		subject = strings.TrimSpace(leaf.Subject.String())
	}

	source := a.cfg.MTLSTenantSource
	if source == "" {
		source = "cert"
	}
	var tenant string
	switch source {
	case "header":
		// Header-only: the operator has explicitly opted into trusting
		// the upstream proxy with tenant identity. The cert is
		// verified for authentication, but the tenant attribute comes
		// from the header.
		tenant = strings.TrimSpace(r.Header.Get(a.cfg.MTLSTenantHeader))
	case "header_or_cert":
		// Hybrid: header wins when present, otherwise fall through to
		// the cert. Useful only for short migration windows.
		tenant = strings.TrimSpace(r.Header.Get(a.cfg.MTLSTenantHeader))
		if tenant == "" {
			tenant = tenantFromCert(leaf)
		}
	default:
		// "cert" or anything unrecognised: cert-only. Any tenant
		// header on the request is silently ignored — a client-
		// controlled header must NEVER mint identity in the default
		// posture.
		tenant = tenantFromCert(leaf)
	}

	if tenant == "" {
		if a.cfg.RequireMTLSTenant {
			return Principal{}, fmt.Errorf("mtls client has no tenant identity (source=%s)", source)
		}
		tenant = a.cfg.DefaultTenantID
	}
	return Principal{
		Subject:  subject,
		TenantID: tenant,
		AuthMode: ModeMTLS,
		Claims: map[string]string{
			"cert_subject":  leaf.Subject.String(),
			"tenant_source": source,
		},
	}, nil
}

// tenantFromCert extracts a tenant identifier from a verified client
// certificate. The lookup order is:
//  1. URI SAN clockify-mcp://tenant/<id>  (this server's namespace)
//  2. URI SAN spiffe://*/tenant/<id>      (SPIFFE/SPIRE convention)
//  3. Subject Organization (first entry, historical behaviour)
//
// The first non-empty match wins. Returns "" when no source matches —
// callers decide whether to fail closed (RequireMTLSTenant) or fall
// back to DefaultTenantID.
func tenantFromCert(leaf *x509.Certificate) string {
	for _, u := range leaf.URIs {
		if u == nil {
			continue
		}
		if id := tenantFromURI(u); id != "" {
			return id
		}
	}
	if len(leaf.Subject.Organization) > 0 {
		return strings.TrimSpace(leaf.Subject.Organization[0])
	}
	return ""
}

// tenantFromURI parses the two supported URI SAN shapes. Returns "" on
// no match or an empty tenant segment (so callers fall through to the
// next lookup tier instead of accepting a blank).
func tenantFromURI(u *url.URL) string {
	switch u.Scheme {
	case "clockify-mcp":
		// clockify-mcp://tenant/<id>
		if u.Host != "tenant" {
			return ""
		}
		return strings.TrimSpace(strings.TrimPrefix(u.Path, "/"))
	case "spiffe":
		// spiffe://<trust-domain>/.../tenant/<id>/...
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "tenant" && parts[i+1] != "" {
				return strings.TrimSpace(parts[i+1])
			}
		}
	}
	return ""
}

func peerLeaf(state *tls.ConnectionState) *x509.Certificate {
	if state == nil || len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 {
		return nil
	}
	return state.VerifiedChains[0][0]
}

type oidcAuthenticator struct {
	cfg         Config
	cache       *jwksCache
	verifyCache *oidcVerifyCache
}

func newOIDCAuthenticator(cfg Config) (Authenticator, error) {
	if cfg.OIDCIssuer == "" {
		return nil, fmt.Errorf("oidc auth requires MCP_OIDC_ISSUER")
	}
	// Strict mode binds tokens to this server. Without an audience or
	// resource URI configured, validateClaims has no value to require in
	// the aud claim, so a token issued by the trusted issuer for a
	// different relying party would still be accepted. internal/config
	// enforces this on the documented startup path; this guard catches
	// programmatic embedders that build authn.Config directly.
	if cfg.OIDCStrict && cfg.OIDCAudience == "" && cfg.OIDCResourceURI == "" {
		return nil, fmt.Errorf("oidc strict mode requires OIDCAudience or OIDCResourceURI to be set")
	}
	u, err := url.Parse(cfg.OIDCIssuer)
	if err != nil || u.Scheme == "" {
		return nil, fmt.Errorf("invalid MCP_OIDC_ISSUER %q (must be absolute URL with scheme)", cfg.OIDCIssuer)
	}
	// Hosted-grade posture: an OIDC strict deployment must fetch JWKS
	// over TLS. Without HTTPS, the JWKS payload (the public keys used
	// to verify every JWT) is fetched in cleartext and any on-path
	// adversary can swap it for keys they control. ChatGPT flagged
	// this as the second go/no-go gate for shared-service.
	if cfg.OIDCStrict && u.Scheme != "https" {
		return nil, fmt.Errorf("MCP_OIDC_ISSUER %q must use https in OIDC strict mode", cfg.OIDCIssuer)
	}
	if cfg.OIDCJWKSURL == "" && cfg.OIDCJWKSPath == "" {
		cfg.OIDCJWKSURL = strings.TrimRight(cfg.OIDCIssuer, "/") + "/.well-known/jwks.json"
	}
	if cfg.OIDCJWKSURL != "" {
		uj, err := url.Parse(cfg.OIDCJWKSURL)
		if err != nil || uj.Scheme == "" {
			return nil, fmt.Errorf("invalid OIDCJWKSURL %q (must be absolute URL with scheme)", cfg.OIDCJWKSURL)
		}
		if cfg.OIDCStrict && uj.Scheme != "https" {
			return nil, fmt.Errorf("OIDCJWKSURL %q must use https in OIDC strict mode", cfg.OIDCJWKSURL)
		}
	}
	// Surface the revocation-window tradeoff when operators raise the
	// ceiling past the conservative default. The cache clamps the value
	// itself; the log just makes the operator's choice visible in audit
	// output so we don't hide a longer revocation window behind a quiet
	// env var.
	if cfg.OIDCVerifyCacheTTL > oidcVerifyCacheMaxTTL {
		slog.Warn("oidc_verify_cache_ttl_above_default",
			"ttl", cfg.OIDCVerifyCacheTTL,
			"default", oidcVerifyCacheMaxTTL,
			"note", "cached verify results live longer; revocation propagates only after ttl expires")
	}
	return oidcAuthenticator{
		cfg: cfg,
		cache: &jwksCache{
			url:    cfg.OIDCJWKSURL,
			path:   cfg.OIDCJWKSPath,
			client: cfg.HTTPClient,
		},
		verifyCache: newOIDCVerifyCache(oidcVerifyCacheSize, cfg.OIDCVerifyCacheTTL),
	}, nil
}

func (a oidcAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Principal, error) {
	token, ok := bearerToken(r)
	if !ok {
		return Principal{}, fmt.Errorf("missing bearer token")
	}
	// Fast path: the same bearer was validated within the cache TTL
	// (oidcVerifyCacheMaxTTL ceiling, capped further by the token's
	// own exp claim). Skips JWT decode, claims validation, JWKS lookup
	// and RSA signature verify — all of which amortise to <50ns on a
	// hit vs ~53.8µs on a miss (BenchmarkOIDCVerifyCached).
	now := time.Now()
	if principal, ok := a.verifyCache.get(token, now); ok {
		return principal, nil
	}

	header, claims, signed, sig, err := decodeJWT(token)
	if err != nil {
		return Principal{}, err
	}
	if err := validateClaims(claims, a.cfg); err != nil {
		return Principal{}, err
	}
	key, err := a.cache.key(ctx, header.KID)
	if err != nil {
		return Principal{}, err
	}
	if err := verifyJWT(header.Alg, key, signed, sig); err != nil {
		return Principal{}, err
	}
	subject := claimString(claims.Raw, a.cfg.SubjectClaim)
	if subject == "" {
		subject = claimString(claims.Raw, "sub")
	}
	if subject == "" {
		return Principal{}, fmt.Errorf("oidc token missing subject claim %q", a.cfg.SubjectClaim)
	}
	tenant := claimString(claims.Raw, a.cfg.TenantClaim)
	if tenant == "" {
		// Hosted-service mode: missing tenant claim is a hard reject.
		// Falling back to DefaultTenantID would silently collapse all
		// tokens that omit the claim into a single shared tenant —
		// dangerous for a public multi-tenant service.
		if a.cfg.RequireTenantClaim {
			return Principal{}, fmt.Errorf("oidc token missing tenant claim %q", a.cfg.TenantClaim)
		}
		tenant = a.cfg.DefaultTenantID
	}
	principal := Principal{
		Subject:  subject,
		TenantID: tenant,
		AuthMode: ModeOIDC,
		Claims: map[string]string{
			"issuer":   claims.Issuer,
			"audience": strings.Join(claims.Audience, ","),
		},
	}
	a.verifyCache.put(token, principal, claims.Expires, now)
	return principal, nil
}

func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

type jwtHeader struct {
	Alg string `json:"alg"`
	KID string `json:"kid"`
}

type jwtClaims struct {
	Issuer    string         `json:"iss"`
	Subject   string         `json:"sub"`
	Audience  claimAudience  `json:"aud"`
	Expires   int64          `json:"exp"`
	NotBefore int64          `json:"nbf"`
	IssuedAt  int64          `json:"iat"`
	Raw       map[string]any `json:"-"`
}

type claimAudience []string

func (a *claimAudience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

func decodeJWT(token string) (jwtHeader, jwtClaims, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("invalid JWT")
	}
	var header jwtHeader
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("decode JWT header: %w", err)
	}
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("parse JWT header: %w", err)
	}
	rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("decode JWT claims: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(rawClaims, &raw); err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("parse JWT claims: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("decode typed JWT claims: %w", err)
	}
	claims.Raw = raw
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("decode JWT signature: %w", err)
	}
	return header, claims, parts[0] + "." + parts[1], sig, nil
}

func validateClaims(claims jwtClaims, cfg Config) error {
	now := time.Now().Unix()
	if claims.Issuer != cfg.OIDCIssuer {
		return fmt.Errorf("unexpected issuer %q", claims.Issuer)
	}
	if cfg.OIDCAudience != "" && !slices.Contains([]string(claims.Audience), cfg.OIDCAudience) {
		return fmt.Errorf("unexpected audience")
	}
	// Resource indicator binding (RFC 8707 / MCP OAuth 2.1 profile): if a
	// canonical resource URI is configured, every token must list it in
	// the audience claim. This is independent of OIDCAudience so an
	// authorization server may issue tokens with multiple audiences and
	// the protected resource still validates only those targeted at it.
	if cfg.OIDCResourceURI != "" && !slices.Contains([]string(claims.Audience), cfg.OIDCResourceURI) {
		return fmt.Errorf("token aud does not contain resource URI %q", cfg.OIDCResourceURI)
	}
	// Strict mode: reject tokens issued without an explicit expiry.
	// In permissive mode an exp=0 (claim absent) is treated as
	// non-expiring, which is unsafe for shared-service deployments.
	if cfg.OIDCStrict && claims.Expires == 0 {
		return fmt.Errorf("token missing exp claim (strict mode)")
	}
	if claims.Expires != 0 && now >= claims.Expires {
		return fmt.Errorf("token expired")
	}
	if claims.NotBefore != 0 && now < claims.NotBefore {
		return fmt.Errorf("token not valid yet")
	}
	return nil
}

func claimString(raw map[string]any, key string) string {
	if v, ok := raw[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// jwksFetchTimeout bounds the per-request JWKS fetch when the
// authenticator did not receive an explicit *http.Client. Five
// seconds is generous for a regional IdP yet small enough that a
// hung issuer cannot stall the auth path past the typical client
// request budget.
const jwksFetchTimeout = 5 * time.Second

// jwksDefaultHTTPClient is the package-level fallback used by
// jwksCache.reload when the authenticator was constructed without
// an explicit client. Reused across cache instances to avoid
// repeated transport allocation.
var jwksDefaultHTTPClient = &http.Client{Timeout: jwksFetchTimeout}

type jwksCache struct {
	mu      sync.Mutex
	url     string
	path    string
	client  *http.Client // nil = jwksDefaultHTTPClient (5s timeout)
	expires time.Time
	keys    map[string]crypto.PublicKey
}

func (c *jwksCache) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys == nil || time.Now().After(c.expires) {
		if err := c.reload(ctx); err != nil {
			return nil, err
		}
	}
	if kid == "" && len(c.keys) == 1 {
		for _, key := range c.keys {
			return key, nil
		}
	}
	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("oidc key %q not found", kid)
	}
	return key, nil
}

func (c *jwksCache) reload(ctx context.Context) error {
	var b []byte
	var err error
	switch {
	case c.path != "":
		b, err = os.ReadFile(c.path)
	case c.url != "":
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
		if reqErr != nil {
			return reqErr
		}
		client := c.client
		if client == nil {
			// Bound the JWKS fetch so a slow/hung issuer cannot stall the
			// auth path indefinitely. http.DefaultClient has no timeout,
			// which would let a non-responsive issuer freeze every
			// concurrent verify after the cache expires. ChatGPT flagged
			// this as a hosted-OIDC reliability gap.
			client = jwksDefaultHTTPClient
		}
		resp, doErr := client.Do(req)
		if doErr != nil {
			return doErr
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("jwks fetch failed: %s", resp.Status)
		}
		b, err = io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	default:
		return fmt.Errorf("no JWKS source configured")
	}
	if err != nil {
		return err
	}
	var doc struct {
		Keys []struct {
			KTY string `json:"kty"`
			KID string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			X   string `json:"x"`
			Y   string `json:"y"`
			CRV string `json:"crv"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}
	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		pub, err := jwkPublicKey(key.KTY, key.N, key.E, key.X, key.Y, key.CRV)
		if err != nil {
			return err
		}
		keys[key.KID] = pub
	}
	c.keys = keys
	c.expires = time.Now().Add(5 * time.Minute)
	return nil
}

func jwkPublicKey(kty, n, e, x, y, crv string) (crypto.PublicKey, error) {
	switch kty {
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(n)
		if err != nil {
			return nil, fmt.Errorf("decode rsa n: %w", err)
		}
		eb, err := base64.RawURLEncoding.DecodeString(e)
		if err != nil {
			return nil, fmt.Errorf("decode rsa e: %w", err)
		}
		if len(eb) == 0 {
			return nil, fmt.Errorf("rsa exponent 'e' is empty")
		}
		maxInt := uint64(^uint(0) >> 1)
		var exp uint64
		for _, b := range eb {
			if exp > (maxInt-uint64(b))>>8 {
				return nil, fmt.Errorf("rsa exponent 'e' overflows int")
			}
			exp = exp<<8 + uint64(b)
		}
		if exp == 0 {
			return nil, fmt.Errorf("rsa exponent 'e' must be positive")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp)}, nil
	case "EC":
		xb, err := base64.RawURLEncoding.DecodeString(x)
		if err != nil {
			return nil, fmt.Errorf("decode ec x: %w", err)
		}
		yb, err := base64.RawURLEncoding.DecodeString(y)
		if err != nil {
			return nil, fmt.Errorf("decode ec y: %w", err)
		}
		return &ecdsa.PublicKey{
			Curve: curveFor(crv),
			X:     new(big.Int).SetBytes(xb),
			Y:     new(big.Int).SetBytes(yb),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported jwk kty %q", kty)
	}
}

func curveFor(name string) elliptic.Curve {
	switch name {
	case "P-384":
		return elliptic.P384()
	case "P-521":
		return elliptic.P521()
	default:
		return elliptic.P256()
	}
}

func verifyJWT(alg string, key crypto.PublicKey, signed string, sig []byte) error {
	sum, hash, err := hashForAlg(alg, signed)
	if err != nil {
		return err
	}
	switch pub := key.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pub, hash, sum, sig)
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, sum, sig) {
			return fmt.Errorf("ecdsa signature verification failed")
		}
		return nil
	default:
		return fmt.Errorf("unsupported public key type %T", key)
	}
}

func hashForAlg(alg, signed string) ([]byte, crypto.Hash, error) {
	var h func() hash.Hash
	var ch crypto.Hash
	switch alg {
	case "RS256", "ES256":
		h = sha256.New
		ch = crypto.SHA256
	case "RS384", "ES384":
		h = sha512.New384
		ch = crypto.SHA384
	case "RS512", "ES512":
		h = sha512.New
		ch = crypto.SHA512
	default:
		return nil, 0, fmt.Errorf("unsupported jwt alg %q", alg)
	}
	hasher := h()
	_, _ = hasher.Write([]byte(signed))
	return hasher.Sum(nil), ch, nil
}
