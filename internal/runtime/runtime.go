//go:build legacy_platform

// Package runtime wires the clockify-mcp process: it loads the
// config-derived dependencies (policy, rate limit, dedupe, dry-run,
// truncation, bootstrap) and owns transport dispatch. It sits ABOVE
// the transports in the import graph — internal/runtime may import
// internal/mcp, internal/authn, internal/controlplane,
// internal/clockify, and the tools layer; none of those may import
// runtime. main.go is reduced to process-global concerns (logging,
// signals, OTel, metrics listener) and a single Runtime.Run call.
package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/confirmation"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

// Runtime is the boot-time integrator for clockify-mcp. Callers build
// it once in main (after logging/OTel/signal setup) and invoke Run,
// which dispatches to the configured transport. The struct is opaque
// to callers beyond a handful of accessors used for the startup log
// line and the per-subject rate-limit reaper.
type Runtime struct {
	cfg           config.Config
	deps          runtimeDeps
	version       string
	extraHandlers []mcp.ExtraHandler
}

// NewOpts bundles process-level inputs that main owns: the version
// string (threaded into the MCP initialize response and HTTP
// User-Agent) and transport-agnostic extra HTTP handlers (e.g.
// /debug/pprof/* from pprofExtras() under -tags=pprof). A zero-value
// NewOpts is valid.
type NewOpts struct {
	Version       string
	ExtraHandlers []mcp.ExtraHandler
}

// New constructs a Runtime from cfg and opts. It loads policy,
// bootstrap, dedupe, dry-run, truncation, and rate limit state from
// the environment; any config error aborts boot before the transport
// is selected.
func New(cfg config.Config, opts NewOpts) (*Runtime, error) {
	pol, err := policy.FromEnv()
	if err != nil {
		return nil, err
	}
	// Transport-scoped ceiling gate. MCP_TENANT_POLICY_CEILING is
	// only load-bearing on streamable_http (the only transport that
	// consumes control-plane TenantRecord overrides). Other
	// transports may inherit the env var from a misconfigured
	// shell / container env without it actually doing anything;
	// failing startup for "policy > ceiling" there would reject a
	// configuration that has no effect. See ADR 0021 and
	// policy.ValidateForTransport.
	if err := policy.ValidateForTransport(pol, cfg.Transport); err != nil {
		return nil, err
	}
	rl := ratelimit.FromEnvWithAcquireTimeout(cfg.ConcurrencyAcquireTimeout)
	tc := truncate.ConfigFromEnv(config.IsHostedProfile(cfg.Profile))
	dc := dryrun.ConfigFromEnv()
	bc, err := bootstrap.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	dd, err := dedupe.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	confResult, err := confirmation.ConfigFromEnv(config.IsHostedProfile(cfg.Profile))
	if err != nil {
		return nil, err
	}
	// Confirmation tokens are minted on the dry-run preview path; with
	// CLOCKIFY_DRY_RUN disabled the pipeline has no mint hook, so a
	// high-risk tool would be permanently unexecutable. Refuse to boot
	// rather than silently degrading every high-risk call to
	// confirmation_required. Operators who genuinely want the gate
	// inert should set CLOCKIFY_CONFIRMATION_TOKENS=disabled.
	if confResult.Config.Enabled && !dc.Enabled {
		return nil, fmt.Errorf("CLOCKIFY_CONFIRMATION_TOKENS=enabled requires CLOCKIFY_DRY_RUN=enabled (high-risk tokens are minted on the dry-run preview path)")
	}
	var confSigner *confirmation.Signer
	if confResult.Config.Enabled {
		signer, err := confirmation.NewSigner(confResult.Config)
		if err != nil {
			return nil, err
		}
		confSigner = signer
	}
	for _, note := range confResult.Notes {
		slog.Warn("confirmation_config", "note", note)
	}
	return &Runtime{
		cfg: cfg,
		deps: runtimeDeps{
			cfg:                       cfg,
			dd:                        dd,
			dc:                        dc,
			tc:                        tc,
			rl:                        rl,
			policy:                    pol,
			bootstrap:                 bc,
			confirmation:              confSigner,
			confirmationReplayEnabled: confResult.Config.ReplayProtection,
			version:                   opts.Version,
		},
		version:       opts.Version,
		extraHandlers: opts.ExtraHandlers,
	}, nil
}

// Policy returns the process-level policy so main can include it in
// the server_start log line without peeking inside runtimeDeps.
func (r *Runtime) Policy() *policy.Policy { return r.deps.policy }

// Bootstrap returns the bootstrap config (exposed for the same
// server_start log line).
func (r *Runtime) Bootstrap() bootstrap.Config { return r.deps.bootstrap }

// RateLimit returns the rate limiter so main can start its
// per-subject reaper (CLOCKIFY_SUBJECT_IDLE_TTL /
// CLOCKIFY_SUBJECT_SWEEP_INTERVAL).
func (r *Runtime) RateLimit() *ratelimit.RateLimiter { return r.deps.rl }

// Run dispatches on cfg.Transport. Streamable HTTP owns its own
// per-session client lifecycle via tenantRuntime; the other arms
// share a single process-level Clockify client built inside Run so
// each transport method can focus on protocol concerns.
func (r *Runtime) Run(ctx context.Context) error {
	if r.cfg.Transport == "streamable_http" {
		return r.runStreamableHTTP(ctx)
	}
	deps := r.deps
	if deps.confirmationReplayEnabled {
		deps.confirmationReplay = confirmation.NewMemoryReplayStore()
	}
	client := clockify.NewClient(r.cfg.APIKey, r.cfg.BaseURL, r.cfg.RequestTimeout, r.cfg.MaxRetries)
	defer client.Close()
	configureClockifyClient(client, r.cfg)
	client.SetUserAgent("clockify-mcp-go/" + r.version)
	service := newService(client, r.cfg.WorkspaceID, r.cfg.Timezone, deps.dd, deps.policy, r.cfg.ReportMaxEntries, r.cfg.WebhookValidateDNS, r.cfg.WebhookAllowedDomains, r.cfg.DocumentedAPIWrites)
	service.DeltaFormat = r.cfg.DeltaFormat
	server := buildServer(r.version, deps, service, deps.policy, &deps.bootstrap)
	metrics.ReadyState.SetFunc(func() float64 {
		if server.IsReadyCached() {
			return 1
		}
		return 0
	})
	metrics.InFlightToolCalls.SetFunc(func() float64 {
		return float64(server.InFlightToolCalls())
	})
	switch r.cfg.Transport {
	case "http":
		return r.runLegacyHTTP(ctx, client, server)
	case "grpc":
		return r.runGRPC(ctx, client, server)
	}
	return r.runStdio(ctx, server)
}
