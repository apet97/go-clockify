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
	"math/big"
	"net/http"
	"os"
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
	MTLSTenantHeader     string
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
	tenant := strings.TrimSpace(r.Header.Get(a.cfg.MTLSTenantHeader))
	if tenant == "" && len(leaf.Subject.Organization) > 0 {
		tenant = strings.TrimSpace(leaf.Subject.Organization[0])
	}
	if tenant == "" {
		tenant = a.cfg.DefaultTenantID
	}
	return Principal{
		Subject:  subject,
		TenantID: tenant,
		AuthMode: ModeMTLS,
		Claims: map[string]string{
			"cert_subject": leaf.Subject.String(),
		},
	}, nil
}

func peerLeaf(state *tls.ConnectionState) *x509.Certificate {
	if state == nil || len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 {
		return nil
	}
	return state.VerifiedChains[0][0]
}

type oidcAuthenticator struct {
	cfg   Config
	cache *jwksCache
}

func newOIDCAuthenticator(cfg Config) (Authenticator, error) {
	if cfg.OIDCIssuer == "" {
		return nil, fmt.Errorf("oidc auth requires MCP_OIDC_ISSUER")
	}
	if cfg.OIDCJWKSURL == "" && cfg.OIDCJWKSPath == "" {
		cfg.OIDCJWKSURL = strings.TrimRight(cfg.OIDCIssuer, "/") + "/.well-known/jwks.json"
	}
	return oidcAuthenticator{
		cfg: cfg,
		cache: &jwksCache{
			url:  cfg.OIDCJWKSURL,
			path: cfg.OIDCJWKSPath,
		},
	}, nil
}

func (a oidcAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Principal, error) {
	token, ok := bearerToken(r)
	if !ok {
		return Principal{}, fmt.Errorf("missing bearer token")
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
		tenant = a.cfg.DefaultTenantID
	}
	return Principal{
		Subject:  subject,
		TenantID: tenant,
		AuthMode: ModeOIDC,
		Claims: map[string]string{
			"issuer":   claims.Issuer,
			"audience": strings.Join(claims.Audience, ","),
		},
	}, nil
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
	if cfg.OIDCAudience != "" {
		matched := false
		for _, aud := range claims.Audience {
			if aud == cfg.OIDCAudience {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("unexpected audience")
		}
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

type jwksCache struct {
	mu      sync.Mutex
	url     string
	path    string
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
		resp, doErr := http.DefaultClient.Do(req)
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
		exp := 0
		for _, b := range eb {
			exp = exp<<8 + int(b)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: exp}, nil
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
