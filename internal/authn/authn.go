package authn

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Mode string

const (
	ModeStaticBearer Mode = "static_bearer"
	ModeOIDC         Mode = "oidc"
	ModeForwardAuth  Mode = "forward_auth"
	ModeMTLS         Mode = "mtls"
)

const maxForwardAuthHeaderBytes = 1024

type Principal struct {
	Subject   string
	TenantID  string
	AuthMode  Mode
	Claims    map[string]string
	SessionID string
}

type Config struct {
	Mode            Mode
	BearerToken     string
	DefaultTenantID string
	TenantClaim     string
	SubjectClaim    string
	OIDCIssuer      string
	OIDCAudience    string
	OIDCJWKSURL     string
	OIDCJWKSPath    string
	// OIDCJWKSAllowPrivate permits an explicit OIDCJWKSURL to target
	// loopback, private, link-local, or reserved addresses. Default false
	// rejects those hosts so a misconfigured JWKS URL cannot probe local
	// or cloud-metadata networks. Enable only for local tests or a trusted
	// private IdP endpoint.
	OIDCJWKSAllowPrivate bool
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	// RequireForwardTenantClaim rejects forward_auth requests when
	// ForwardTenantHeader is absent or empty, instead of falling back to
	// DefaultTenantID. Default false preserves self-hosted single-tenant
	// behaviour; hosted profiles should enable it.
	RequireForwardTenantClaim bool
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
	// OIDCRequireKID rejects JWTs that omit the JOSE kid header instead of
	// falling back to the lone key in a JWKS. Hosted profiles enable this to
	// make key rotation and provenance explicit.
	OIDCRequireKID bool
	// OIDCVerifyCacheTTL is the hard ceiling on cached verify results.
	// Zero selects the default (oidcVerifyCacheMaxTTL); values are
	// clamped to [oidcVerifyCacheMinTTL, oidcVerifyCacheTTLCeiling].
	// Larger values amortise the ~54µs verify cost further, but extend
	// the window after a token is revoked before the next Authenticate
	// call re-checks the claims. Operators should pick this
	// consciously; the default stays conservative at 60s.
	OIDCVerifyCacheTTL time.Duration
	// OIDCJWKSCacheTTL is the lifetime of the in-memory JWKS document
	// cache. Zero selects the conservative 5-minute default; values
	// are clamped to [jwksCacheMinTTL, jwksCacheMaxTTL] (1 minute to
	// 24 hours). Hosted services that rotate keys frequently can
	// shorten the window so a rotation lands without waiting for the
	// next periodic reload; F2's kid-miss-triggered refresh covers
	// the rotation-in-flight case independently. Wired from
	// MCP_OIDC_JWKS_CACHE_TTL.
	OIDCJWKSCacheTTL time.Duration
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
	subject, err := forwardAuthHeaderValue(r.Header, a.cfg.ForwardSubjectHeader, "subject", true)
	if err != nil {
		return Principal{}, err
	}
	tenant, err := forwardAuthHeaderValue(r.Header, a.cfg.ForwardTenantHeader, "tenant", false)
	if err != nil {
		return Principal{}, err
	}
	if tenant == "" {
		if a.cfg.RequireForwardTenantClaim {
			return Principal{}, fmt.Errorf("forward_auth request missing tenant header %s", a.cfg.ForwardTenantHeader)
		}
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

func forwardAuthHeaderValue(h http.Header, name, label string, required bool) (string, error) {
	values := h.Values(name)
	if len(values) == 0 {
		if required {
			return "", fmt.Errorf("missing %s header", name)
		}
		return "", nil
	}
	if len(values) > 1 {
		return "", fmt.Errorf("forward_auth: %s header %s has duplicated values", label, name)
	}
	raw := values[0]
	if len(raw) > maxForwardAuthHeaderBytes {
		return "", fmt.Errorf("forward_auth: %s header %s is too large: %d > %d bytes", label, name, len(raw), maxForwardAuthHeaderBytes)
	}
	if required && strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("missing %s header", name)
	}
	return sanitizePrincipalString(raw, label)
}

// sanitizePrincipalString rejects bytes that have no business minting an
// authentication identity. The caller passes the raw principal value plus a
// short label ("subject" / "tenant") for the error message. The function
// trims surrounding whitespace and then walks the runes:
//
//   - utf8.RuneError is refused so a malformed UTF-8 sequence
//     cannot smuggle a byte the rest of the system mis-decodes.
//   - !unicode.IsPrint is refused so control bytes (\n, \r, \x00,
//     \x1f, \x7f), surrogate halves, format characters
//     (zero-width space U+200B, RTL override U+202E, BOM U+FEFF),
//     and other non-printable codepoints are kept out of
//     Principal.Subject / Principal.TenantID — and out of the
//     downstream slog `subject` / `tenant_id` keys
//     (internal/mcp/audit.go, internal/mcp/tools.go) and
//     tenant-scoping keys.
//
// ASCII space (0x20) is unicode.IsPrint, so legitimate values
// like "alice doe" or "my org" still pass. Returns the trimmed value when
// accepted; an error of the form "<source>: <label> contains disallowed byte
// 0x<hex>" otherwise.
func sanitizePrincipalString(s, label string) (string, error) {
	return sanitizePrincipalStringForSource("forward_auth", s, label)
}

func sanitizePrincipalStringForSource(source, s, label string) (string, error) {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			return "", fmt.Errorf("%s: %s contains disallowed byte 0x%x", source, label, r)
		}
	}
	return s, nil
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
	var err error
	subject, err = sanitizePrincipalStringForSource("mtls", subject, "subject")
	if err != nil {
		return Principal{}, err
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
	tenant, err = sanitizePrincipalStringForSource("mtls", tenant, "tenant")
	if err != nil {
		return Principal{}, err
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

func bearerToken(r *http.Request) (string, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, token, ok := strings.Cut(auth, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
