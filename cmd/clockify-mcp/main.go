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

// printHelp emits the --help banner. The environment-variable catalog is
// rendered from internal/config/AllSpecs() via cmd/gen-config-docs and
// embedded as generatedHelp in help_generated.go. Regenerate after any
// EnvSpec edit with: go run ./cmd/gen-config-docs -mode=all
func printHelp() {
	fmt.Fprintf(os.Stderr, "clockify-mcp v%s — MCP server for Clockify\n\nUsage: clockify-mcp [--version | --help]\n\n%s", version, generatedHelp)
}
