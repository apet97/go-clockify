package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apet97/go-clockify/internal/config"
	logslog "github.com/apet97/go-clockify/internal/logging"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"
	svcruntime "github.com/apet97/go-clockify/internal/runtime"
)

// version, commit, and buildDate are populated at build time via ldflags:
//
//	go build -ldflags "-X main.version=v0.5.0 \
//	                   -X main.commit=$(git rev-parse HEAD) \
//	                   -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
//	        ./cmd/clockify-mcp
//
// commit and buildDate default to placeholder strings when ldflags are not
// set (local `go run`, `go build` without flags), so the /metrics build_info
// gauge always emits a sample.
var (
	version   = "1.0.0"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Run the FIPS startup assertion first. Default build is a no-op.
	// Under -tags=fips this fails the process if crypto/fips140 reports
	// the module is not active. See ADR 011.
	fipsStartupCheck()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println(version)
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		}
	}

	// Configure log level
	rawLevel := os.Getenv("MCP_LOG_LEVEL")
	logLevel := parseLogLevel(rawLevel)
	if rawLevel != "" && !isKnownLogLevel(rawLevel) {
		fmt.Fprintf(os.Stderr, "warning: unknown MCP_LOG_LEVEL %q, defaulting to info\n", rawLevel)
	}

	// Configure slog to stderr. The chosen handler is always wrapped in a
	// RedactingHandler so that any attribute matching a well-known secret
	// key (authorization, api_key, bearer, token, ...) is masked before it
	// reaches the underlying encoder. This is defence-in-depth; hot-path
	// code should still avoid logging secrets explicitly.
	var logHandler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	if os.Getenv("MCP_LOG_FORMAT") == "json" {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	}
	logHandler = logslog.NewRedactingHandler(logHandler)
	slog.SetDefault(slog.New(logHandler))

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rt, err := svcruntime.New(cfg, svcruntime.NewOpts{
		Version:       version,
		ExtraHandlers: pprofExtras(),
	})
	if err != nil {
		return err
	}

	// BuildInfo is a process-level gauge wired once here. ReadyState /
	// InFlightToolCalls are rewired per-transport inside Runtime.Run
	// once the server is built.
	metrics.BuildInfo.SetFunc(
		func() float64 { return 1 },
		version, commit, buildDate, runtime.Version(),
	)

	slog.Info("server_start",
		"version", version,
		"policy", string(rt.Policy().Mode),
		"bootstrap", rt.Bootstrap().Mode.String(),
		"transport", cfg.Transport,
		"workspace", cfg.WorkspaceID,
		"config", cfg.Fingerprint(),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// B3: start the per-subject reaper so the subjects map can't grow
	// unbounded in long-lived multi-tenant deployments. Defaults: 1h
	// idle TTL swept every 5m. Override via CLOCKIFY_SUBJECT_IDLE_TTL
	// and CLOCKIFY_SUBJECT_SWEEP_INTERVAL. Nil receiver (limiter
	// disabled) is a no-op.
	rt.RateLimit().StartSubjectReaper(ctx, subjectSweepInterval(), subjectIdleTTL())

	// Install the OTel exporter when built with -tags=otel and
	// OTEL_EXPORTER_OTLP_ENDPOINT is set. Default build is a no-op. See ADR 009.
	otelShutdown := installOTel(ctx)
	defer otelShutdown()
	if cfg.MetricsBind != "" {
		go func() {
			if err := mcp.ServeMetrics(ctx, mcp.MetricsServerOptions{
				Bind:        cfg.MetricsBind,
				AuthMode:    cfg.MetricsAuthMode,
				BearerToken: cfg.MetricsBearerToken,
			}); err != nil {
				slog.Error("metrics_server_error", "error", err.Error())
				cancel()
			}
		}()
	}

	return rt.Run(ctx)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isKnownLogLevel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "info", "warn", "warning", "error":
		return true
	}
	return false
}

// subjectIdleTTL returns the cutoff at which a per-subject rate limiter
// entry becomes eligible for reap. 0 disables reaping entirely.
// Default 1h keeps steady-state memory bounded without being so
// aggressive that bursty subjects with hour-between-calls patterns
// pay repeat allocation cost.
func subjectIdleTTL() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CLOCKIFY_SUBJECT_IDLE_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 1 * time.Hour
}

// subjectSweepInterval is how often the background reaper runs. 0
// disables. Default 5m balances reap latency against goroutine wakes.
func subjectSweepInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CLOCKIFY_SUBJECT_SWEEP_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Minute
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `clockify-mcp v%s — MCP server for Clockify

Usage: clockify-mcp [--version | --help]

Environment Variables:
  Core:
    CLOCKIFY_API_KEY          API key (required for stdio/http; optional for streamable_http)
    CLOCKIFY_WORKSPACE_ID     Workspace ID (auto-detected if only one)
    CLOCKIFY_BASE_URL         API base URL (default: https://api.clockify.me/api/v1)
    CLOCKIFY_TIMEZONE         IANA timezone for time parsing
    CLOCKIFY_INSECURE         Set to 1 to allow non-HTTPS base URLs

  Safety:
    CLOCKIFY_POLICY           read_only, safe_core, standard (default), full
    CLOCKIFY_DRY_RUN          Dry-run for destructive tools (default: enabled)
    CLOCKIFY_DENY_TOOLS       Comma-separated tools to block
    CLOCKIFY_DENY_GROUPS      Comma-separated groups to block
    CLOCKIFY_ALLOW_GROUPS     Comma-separated allowed groups
    CLOCKIFY_DEDUPE_MODE      warn (default), block, off
    CLOCKIFY_DEDUPE_LOOKBACK  Recent entries to check (default: 25)
    CLOCKIFY_OVERLAP_CHECK    Overlapping entry detection (default: true)

  Performance:
    CLOCKIFY_MAX_CONCURRENT        Concurrent tool call limit, 0=off (default: 10)
    CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT  Time to wait for a concurrency slot (default: 100ms)
    CLOCKIFY_RATE_LIMIT            Tool calls per fixed 60s window, 0=off (default: 120)
    CLOCKIFY_TOKEN_BUDGET          Response token budget, 0=off (default: 8000)
    MCP_MAX_INFLIGHT_TOOL_CALLS    Stdio dispatch goroutine cap, 0=off (default: 64)
    CLOCKIFY_REPORT_MAX_ENTRIES    Hard cap on entries aggregated by report tools, 0=off (default: 10000)

  Bootstrap:
    CLOCKIFY_BOOTSTRAP_MODE   full_tier1 (default), minimal, custom
    CLOCKIFY_BOOTSTRAP_TOOLS  Tool list for custom mode

  Transport:
    MCP_TRANSPORT                        stdio (default), http, streamable_http, or grpc
    MCP_GRPC_BIND                        gRPC listen address when MCP_TRANSPORT=grpc (default: :9090, requires -tags=grpc)
    MCP_AUTH_MODE                        static_bearer, oidc, forward_auth, mtls (grpc: static_bearer+oidc only)
    MCP_HTTP_BIND                        HTTP listen address (default: :8080)
    MCP_BEARER_TOKEN                     Required for static bearer auth; send as Authorization: Bearer <token>
    MCP_ALLOWED_ORIGINS                  Comma-separated CORS origins
    MCP_ALLOW_ANY_ORIGIN                 Set 1 to allow all origins
    MCP_STRICT_HOST_CHECK                Set 1 to require Host match localhost/127.0.0.1/::1 or MCP_ALLOWED_ORIGINS
    MCP_HTTP_MAX_BODY                    Positive max request body in bytes (default: 2097152)
    MCP_METRICS_BIND                     Optional dedicated metrics listener (recommended for streamable_http)
    MCP_METRICS_AUTH_MODE                none (default) or static_bearer
    MCP_METRICS_BEARER_TOKEN             Bearer token for dedicated metrics listener
    MCP_HTTP_LEGACY_POLICY               warn (default), allow, or deny — controls MCP_TRANSPORT=http startup behaviour
    MCP_HTTP_INLINE_METRICS_ENABLED      Set 1 to expose /metrics on the main HTTP listener (default: off)
    MCP_HTTP_INLINE_METRICS_AUTH_MODE    inherit_main_bearer (default), static_bearer, or none
    MCP_HTTP_INLINE_METRICS_BEARER_TOKEN Separate bearer token for /metrics when auth mode is static_bearer

  Audit:
    MCP_AUDIT_DURABILITY      best_effort (default) or fail_closed — controls whether audit persist failures abort tool calls

  Enterprise Shared-Service:
    MCP_CONTROL_PLANE_DSN     Control-plane store DSN (memory, /path/file.json, or file:///path/file.json)
    MCP_SESSION_TTL           Session TTL for streamable_http (default: 30m)
    MCP_TENANT_CLAIM          Tenant claim name for OIDC (default: tenant_id)
    MCP_SUBJECT_CLAIM         Subject claim name for OIDC (default: sub)
    MCP_DEFAULT_TENANT_ID     Default tenant for static_bearer/forward_auth/mtls (default: default)
    MCP_OIDC_ISSUER           Required issuer URL for OIDC auth
    MCP_OIDC_AUDIENCE         Optional audience for OIDC auth
    MCP_OIDC_JWKS_URL         Optional JWKS URL override
    MCP_OIDC_JWKS_PATH        Optional local JWKS file for tests/dev
    MCP_FORWARD_TENANT_HEADER Tenant header for forward_auth (default: X-Forwarded-Tenant)
    MCP_FORWARD_SUBJECT_HEADER Subject header for forward_auth (default: X-Forwarded-User)
    MCP_MTLS_TENANT_HEADER    Tenant header override for mtls (default: X-Tenant-ID)

  Logging:
    MCP_LOG_FORMAT            text (default) or json
    MCP_LOG_LEVEL             debug, info (default), warn, error
`, version)
}
