package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/dryrun"
	logslog "github.com/apet97/go-clockify/internal/logging"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
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

	pol, err := policy.FromEnv()
	if err != nil {
		return err
	}

	rl := ratelimit.FromEnvWithAcquireTimeout(cfg.ConcurrencyAcquireTimeout)
	tc := truncate.ConfigFromEnv()
	dc := dryrun.ConfigFromEnv()

	bc, err := bootstrap.ConfigFromEnv()
	if err != nil {
		return err
	}

	dd, err := dedupe.ConfigFromEnv()
	if err != nil {
		return err
	}
	deps := runtimeDeps{
		cfg:       cfg,
		dd:        dd,
		dc:        dc,
		tc:        tc,
		rl:        rl,
		policy:    pol,
		bootstrap: bc,
	}

	// Wire observability gauges. ReadyState uses the server's cached
	// readiness snapshot so /metrics scrapes do not trigger upstream
	// Clockify probes; scrapers that need a fresh probe should hit /ready.
	metrics.BuildInfo.SetFunc(
		func() float64 { return 1 },
		version, commit, buildDate, runtime.Version(),
	)

	slog.Info("server_start",
		"version", version,
		"policy", string(pol.Mode),
		"bootstrap", bc.Mode.String(),
		"transport", cfg.Transport,
		"workspace", cfg.WorkspaceID,
		"config", cfg.Fingerprint(),
	)

	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

	if cfg.Transport == "streamable_http" {
		store, err := controlplane.Open(cfg.ControlPlaneDSN)
		if err != nil {
			return err
		}
		if err := bootstrapDefaultTenant(store, cfg); err != nil {
			return err
		}
		authnCfg := authn.Config{
			Mode:                 authn.Mode(cfg.AuthMode),
			BearerToken:          cfg.BearerToken,
			DefaultTenantID:      cfg.DefaultTenantID,
			TenantClaim:          cfg.TenantClaim,
			SubjectClaim:         cfg.SubjectClaim,
			OIDCIssuer:           cfg.OIDCIssuer,
			OIDCAudience:         cfg.OIDCAudience,
			OIDCJWKSURL:          cfg.OIDCJWKSURL,
			OIDCJWKSPath:         cfg.OIDCJWKSPath,
			OIDCResourceURI:      cfg.OIDCResourceURI,
			ForwardTenantHeader:  cfg.ForwardTenantHeader,
			ForwardSubjectHeader: cfg.ForwardSubjectHeader,
			MTLSTenantHeader:     cfg.MTLSTenantHeader,
		}
		authenticator, err := authn.New(authnCfg)
		if err != nil {
			return err
		}
		protectedResource := authn.ProtectedResourceHandler(authnCfg)
		deps.auditor = controlPlaneAuditor{store: store}
		var readyChecker func(context.Context) error
		if cfg.APIKey != "" {
			client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
			defer client.Close()
			readyChecker = func(ctx context.Context) error {
				var user struct{ ID string }
				return client.Get(ctx, "/user", nil, &user)
			}
		}
		return mcp.ServeStreamableHTTP(ctx, mcp.StreamableHTTPOptions{
			Version:           version,
			Bind:              cfg.HTTPBind,
			MaxBodySize:       cfg.MaxBodySize,
			AllowedOrigins:    cfg.AllowedOrigins,
			AllowAnyOrigin:    cfg.AllowAnyOrigin,
			StrictHostCheck:   cfg.StrictHostCheck,
			SessionTTL:        cfg.SessionTTL,
			ReadyChecker:      readyChecker,
			Authenticator:     authenticator,
			ControlPlane:      store,
			ProtectedResource: protectedResource,
			ExtraHandlers:     pprofExtras(),
			Factory: func(ctx context.Context, principal authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
				return tenantRuntime(ctx, principal.TenantID, deps, store)
			},
		})
	}

	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
	defer client.Close()
	client.SetUserAgent("clockify-mcp-go/" + version)
	service := newService(client, cfg.WorkspaceID, cfg.Timezone, dd, pol, cfg.ReportMaxEntries)
	service.DeltaFormat = cfg.DeltaFormat
	server := buildServer(version, deps, service, pol, &bc)
	metrics.ReadyState.SetFunc(func() float64 {
		if server.IsReadyCached() {
			return 1
		}
		return 0
	})
	metrics.InFlightToolCalls.SetFunc(func() float64 {
		return float64(server.InFlightToolCalls())
	})

	if cfg.Transport == "http" {
		if cfg.AllowAnyOrigin {
			slog.Warn("cors_any_origin", "msg", "MCP_ALLOW_ANY_ORIGIN=1 is set — all cross-origin requests will be accepted. This is not recommended for production.")
		}
		// Wire upstream health check: lightweight GET /api/v1/user
		server.ReadyChecker = func(ctx context.Context) error {
			var user struct{ ID string }
			return client.Get(ctx, "/user", nil, &user)
		}
		// Opt-in debug handlers (e.g. /debug/pprof/* under -tags=pprof).
		// Default build returns nil so ServeHTTP sees an empty slice.
		server.ExtraHTTPHandlers = pprofExtras()
		return server.ServeHTTP(ctx, cfg.HTTPBind, cfg.BearerToken, cfg.AllowedOrigins, cfg.AllowAnyOrigin, cfg.MaxBodySize)
	}
	if cfg.Transport == "grpc" {
		// gRPC transport is built only with -tags=grpc. The default build
		// stub returns a clear error explaining the build-tag requirement.
		// See ADR 012.
		//
		// Wire upstream readiness: a lightweight GET /user call warms the
		// cached readiness flag consumed by the native gRPC health protocol
		// (grpc.health.v1.Health/Check). A background goroutine refreshes
		// the cache every 15s so kubelet probes see fresh state.
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			check := func() {
				checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				var user struct{ ID string }
				server.SetReadyCached(client.Get(checkCtx, "/user", nil, &user) == nil)
			}
			check()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					check()
				}
			}
		}()
		// When MCP_AUTH_MODE is set, build the shared authn.Authenticator
		// (same construction path as the streamable_http branch above) and
		// let the gRPC transport install its auth stream interceptor. The
		// config layer already rejects forward_auth/mtls for grpc, so only
		// static_bearer and oidc reach here.
		var grpcAuthenticator authn.Authenticator
		if cfg.AuthMode != "" {
			authnCfg := authn.Config{
				Mode:                 authn.Mode(cfg.AuthMode),
				BearerToken:          cfg.BearerToken,
				DefaultTenantID:      cfg.DefaultTenantID,
				TenantClaim:          cfg.TenantClaim,
				SubjectClaim:         cfg.SubjectClaim,
				OIDCIssuer:           cfg.OIDCIssuer,
				OIDCAudience:         cfg.OIDCAudience,
				OIDCJWKSURL:          cfg.OIDCJWKSURL,
				OIDCJWKSPath:         cfg.OIDCJWKSPath,
				OIDCResourceURI:      cfg.OIDCResourceURI,
				ForwardTenantHeader:  cfg.ForwardTenantHeader,
				ForwardSubjectHeader: cfg.ForwardSubjectHeader,
				MTLSTenantHeader:     cfg.MTLSTenantHeader,
			}
			var err error
			grpcAuthenticator, err = authn.New(authnCfg)
			if err != nil {
				return err
			}
		}
		var grpcTLS *tls.Config
		if cfg.GRPCTLSCert != "" {
			cert, err := tls.LoadX509KeyPair(cfg.GRPCTLSCert, cfg.GRPCTLSKey)
			if err != nil {
				return fmt.Errorf("load gRPC TLS cert/key: %w", err)
			}
			grpcTLS = &tls.Config{Certificates: []tls.Certificate{cert}}
			if cfg.MTLSCACertPath != "" {
				caCert, err := os.ReadFile(cfg.MTLSCACertPath)
				if err != nil {
					return fmt.Errorf("read mTLS CA cert: %w", err)
				}
				pool := x509.NewCertPool()
				if !pool.AppendCertsFromPEM(caCert) {
					return fmt.Errorf("mTLS CA cert: no valid PEM certificates found")
				}
				grpcTLS.ClientCAs = pool
				grpcTLS.ClientAuth = tls.RequireAndVerifyClientCert
			}
		}
		return serveGRPC(ctx, cfg.GRPCBind, server, grpcAuthenticator, grpcConfig{
			reauthInterval:       cfg.GRPCReauthInterval,
			forwardTenantHeader:  cfg.ForwardTenantHeader,
			forwardSubjectHeader: cfg.ForwardSubjectHeader,
			tlsConfig:            grpcTLS,
		})
	}
	return server.Run(ctx, os.Stdin, os.Stdout)
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

func printHelp() {
	fmt.Fprintf(os.Stderr, `clockify-mcp v%s — MCP server for Clockify

Usage: clockify-mcp [--version | --help]

Environment Variables:
  Core:
    CLOCKIFY_API_KEY          API key (required for stdio/legacy http; optional for streamable_http)
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
    MCP_TRANSPORT             stdio (default), legacy http, streamable_http, or grpc
    MCP_GRPC_BIND             gRPC listen address when MCP_TRANSPORT=grpc (default: :9090, requires -tags=grpc)
    MCP_AUTH_MODE             static_bearer, oidc, forward_auth, mtls (grpc: static_bearer+oidc only)
    MCP_HTTP_BIND             HTTP listen address (default: :8080)
    MCP_BEARER_TOKEN          Required for static bearer auth; send as Authorization: Bearer <token>
    MCP_ALLOWED_ORIGINS       Comma-separated CORS origins
    MCP_ALLOW_ANY_ORIGIN      Set 1 to allow all origins
    MCP_STRICT_HOST_CHECK     Set 1 to require Host match localhost/127.0.0.1/::1 or MCP_ALLOWED_ORIGINS
    MCP_HTTP_MAX_BODY         Positive max request body in bytes (default: 2097152)
    MCP_METRICS_BIND          Optional dedicated metrics listener (recommended for streamable_http)
    MCP_METRICS_AUTH_MODE     none (default) or static_bearer
    MCP_METRICS_BEARER_TOKEN  Bearer token for dedicated metrics listener

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
