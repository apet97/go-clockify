package runtime

import (
	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/config"
)

// buildAuthnConfig projects the relevant fields of config.Config into
// authn.Config. Shared by streamable_http, legacy http, and grpc so
// the three transports construct authenticators from the same surface
// (prior inline copies had drifted: the grpc arm omitted
// MTLSTenantHeader and OIDCVerifyCacheTTL). Callers still decide
// whether to honour Mode=="": legacy http maps empty to static_bearer
// inside authn.New, and grpc skips the authn.New call entirely when
// cfg.AuthMode is empty.
func buildAuthnConfig(cfg config.Config) authn.Config {
	return authn.Config{
		Mode:                      authn.Mode(cfg.AuthMode),
		BearerToken:               cfg.BearerToken,
		DefaultTenantID:           cfg.DefaultTenantID,
		TenantClaim:               cfg.TenantClaim,
		SubjectClaim:              cfg.SubjectClaim,
		OIDCIssuer:                cfg.OIDCIssuer,
		OIDCAudience:              cfg.OIDCAudience,
		OIDCJWKSURL:               cfg.OIDCJWKSURL,
		OIDCJWKSPath:              cfg.OIDCJWKSPath,
		OIDCResourceURI:           cfg.OIDCResourceURI,
		OIDCVerifyCacheTTL:        cfg.OIDCVerifyCacheTTL,
		OIDCStrict:                cfg.OIDCStrict,
		RequireTenantClaim:        cfg.RequireTenantClaim,
		ForwardTenantHeader:       cfg.ForwardTenantHeader,
		ForwardSubjectHeader:      cfg.ForwardSubjectHeader,
		ForwardAuthTrustedProxies: cfg.ForwardAuthTrustedProxies,
		MTLSTenantHeader:          cfg.MTLSTenantHeader,
		MTLSTenantSource:          cfg.MTLSTenantSource,
		RequireMTLSTenant:         cfg.RequireMTLSTenant,
	}
}
