package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// runLegacyHTTP serves the legacy HTTP transport (MCP_TRANSPORT=http).
// Behaviour mirrors the prior inline branch in
// cmd/clockify-mcp/main.go: honour MCP_HTTP_LEGACY_POLICY (deny /
// allow / warn), emit risky-config warnings for AllowAnyOrigin and
// unauthenticated inline metrics, wire a lightweight upstream
// ready-check, mount pprof extras if present, and build the legacy
// authenticator through the shared buildAuthnConfig helper.
func (r *Runtime) runLegacyHTTP(ctx context.Context, client *clockify.Client, server *mcp.Server) error {
	switch r.cfg.HTTPLegacyPolicy {
	case "deny":
		return fmt.Errorf(
			"legacy HTTP transport is denied by MCP_HTTP_LEGACY_POLICY=deny; " +
				"use MCP_TRANSPORT=streamable_http for spec-strict shared-service deployments, " +
				"or set MCP_HTTP_LEGACY_POLICY=allow to permit legacy HTTP explicitly",
		)
	case "allow":
		// Intentional; operator has acknowledged the tradeoffs. No warnings.
	default: // "warn"
		slog.Warn("legacy_http_transport",
			"transport", "http",
			"msg", "MCP_TRANSPORT=http is the legacy HTTP transport. "+
				"Server-initiated notifications (tools/list_changed) are dropped. "+
				"Use MCP_TRANSPORT=streamable_http for spec-strict shared-service deployments.",
			"recommendation", "streamable_http",
			"mitigation", "set MCP_HTTP_LEGACY_POLICY=allow to suppress this warning if legacy HTTP is intentional",
		)
	}
	if r.cfg.AllowAnyOrigin {
		slog.Warn("risky_config",
			"transport", "http",
			"risk", "cors_any_origin",
			"msg", "MCP_ALLOW_ANY_ORIGIN=1 — all cross-origin requests accepted. Not recommended for production.",
		)
	}
	if r.cfg.HTTPInlineMetricsEnabled && r.cfg.HTTPInlineMetricsAuthMode == "none" {
		slog.Warn("risky_config",
			"transport", "http",
			"risk", "inline_metrics_no_auth",
			"msg", "MCP_HTTP_INLINE_METRICS_AUTH_MODE=none — /metrics on the main HTTP listener is unauthenticated. "+
				"Consider inherit_main_bearer or static_bearer, or use the dedicated MCP_METRICS_BIND listener instead.",
		)
	}
	server.ReadyChecker = func(ctx context.Context) error {
		var user struct{ ID string }
		return client.Get(ctx, "/user", nil, &user)
	}
	server.ExposeAuthErrors = r.cfg.ExposeAuthErrors
	server.ExtraHTTPHandlers = r.extraHandlers
	legacyAuth, err := authn.New(buildAuthnConfig(r.cfg))
	if err != nil {
		return fmt.Errorf("build legacy HTTP authenticator: %w", err)
	}
	return server.ServeHTTP(ctx, r.cfg.HTTPBind, legacyAuth, r.cfg.BearerToken, r.cfg.AllowedOrigins, r.cfg.AllowAnyOrigin, r.cfg.MaxMessageSize,
		mcp.InlineMetricsOptions{
			Enabled:     r.cfg.HTTPInlineMetricsEnabled,
			AuthMode:    r.cfg.HTTPInlineMetricsAuthMode,
			BearerToken: r.cfg.HTTPInlineMetricsBearerToken,
		},
	)
}
