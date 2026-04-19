package runtime

import (
	"context"
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
	return mcp.ServeStreamableHTTP(ctx, mcp.StreamableHTTPOptions{
		Version:           r.version,
		Bind:              r.cfg.HTTPBind,
		MaxBodySize:       r.cfg.MaxBodySize,
		AllowedOrigins:    r.cfg.AllowedOrigins,
		AllowAnyOrigin:    r.cfg.AllowAnyOrigin,
		StrictHostCheck:   r.cfg.StrictHostCheck,
		SessionTTL:        r.cfg.SessionTTL,
		ReadyChecker:      readyChecker,
		Authenticator:     authenticator,
		ControlPlane:      store,
		ProtectedResource: protectedResource,
		ExtraHandlers:     r.extraHandlers,
		Factory: func(ctx context.Context, principal authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
			return tenantRuntime(ctx, principal.TenantID, deps, store)
		},
	})
}
