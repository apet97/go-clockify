package runtime

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// runStreamableHTTP serves the spec-strict streamable HTTP transport.
// Semantically identical to the prior cmd/clockify-mcp/main.go
// branch: open the control-plane store (fail-closed dev-backend guard
// lives inside BuildStore), seed the default tenant, start the audit
// retention reaper, build the authenticator, and hand a per-tenant
// session factory to mcp.ServeStreamableHTTP. The ready-check client
// is only created when an API key is present; each session's client
// is owned by tenantRuntime.
func (r *Runtime) runStreamableHTTP(ctx context.Context) error {
	store, err := BuildStore(r.cfg)
	if err != nil {
		return err
	}
	if err := bootstrapDefaultTenant(store, r.cfg); err != nil {
		return err
	}
	if r.cfg.ControlPlaneAuditRetention > 0 {
		go RetainAuditLoop(ctx, store, r.cfg.ControlPlaneAuditRetention, time.Hour)
	}
	authnCfg := buildAuthnConfig(r.cfg)
	authenticator, err := authn.New(authnCfg)
	if err != nil {
		return err
	}
	protectedResource := authn.ProtectedResourceHandler(authnCfg)
	deps := r.deps
	deps.auditor = controlPlaneAuditor{store: store}
	var readyChecker func(context.Context) error
	if r.cfg.APIKey != "" {
		client := clockify.NewClient(r.cfg.APIKey, r.cfg.BaseURL, r.cfg.RequestTimeout, r.cfg.MaxRetries)
		defer client.Close()
		readyChecker = func(ctx context.Context) error {
			var user struct{ ID string }
			return client.Get(ctx, "/user", nil, &user)
		}
	}
	// Native TLS / mTLS for the streamable HTTP transport. Driven from
	// MCP_HTTP_TLS_CERT / MCP_HTTP_TLS_KEY (set together; config.Load
	// rejects half-configurations) plus MCP_MTLS_CA_CERT_PATH for
	// client-cert verification when MCP_AUTH_MODE=mtls. Without
	// HTTPTLSCert the listener stays plain HTTP, preserving the
	// long-standing default for self-hosted single-tenant use.
	var tlsConfig *tls.Config
	if r.cfg.HTTPTLSCert != "" {
		tlsConfig, err = buildServerTLSConfig(
			r.cfg.HTTPTLSCert,
			r.cfg.HTTPTLSKey,
			r.cfg.MTLSCACertPath,
			r.cfg.AuthMode == "mtls",
			tls.VersionTLS12,
		)
		if err != nil {
			return err
		}
	}
	return mcp.ServeStreamableHTTP(ctx, mcp.StreamableHTTPOptions{
		Version:                r.version,
		Bind:                   r.cfg.HTTPBind,
		MaxBodySize:            r.cfg.MaxMessageSize,
		AllowedOrigins:         r.cfg.AllowedOrigins,
		AllowAnyOrigin:         r.cfg.AllowAnyOrigin,
		StrictHostCheck:        r.cfg.StrictHostCheck,
		BehindHTTPSProxy:       r.cfg.BehindHTTPSProxy,
		ExposeAuthErrors:       r.cfg.ExposeAuthErrors,
		SanitizeUpstreamErrors: r.cfg.SanitizeUpstreamErrors,
		SessionTTL:             r.cfg.SessionTTL,
		ReadyChecker:           readyChecker,
		Authenticator:          authenticator,
		ControlPlane:           store,
		ProtectedResource:      protectedResource,
		ExtraHandlers:          r.extraHandlers,
		TLSConfig:              tlsConfig,
		Factory: func(ctx context.Context, principal authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
			return tenantRuntime(ctx, principal.TenantID, deps, store)
		},
	})
}
