package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	logslog "github.com/apet97/go-clockify/internal/logging"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
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
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func effectiveVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			return bi.Main.Version
		}
	}
	return version
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println(effectiveVersion())
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "doctor":
			os.Exit(runDoctor(os.Args[2:], os.Stdout, os.Stderr))
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
	logHandler = logslog.NewRedactingHandler(logHandler)
	slog.SetDefault(slog.New(logHandler))

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadOneUser()
	if err != nil {
		return err
	}
	effective := effectiveVersion()
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, 30*time.Second, 2)
	client.SetUserAgent("clockify-mcp-one-user/" + effective)
	defer client.Close()

	service := tools.New(client, cfg.WorkspaceID)
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return err
		}
		service.DefaultTimezone = loc
	}
	server := mcp.NewServer(effective, service.FullAccessRegistry(), nil, nil)
	server.StaticToolList = true
	server.ResourceProvider = service
	service.EmitResourceUpdate = server.NotifyResourceUpdated
	service.SubscriptionGate = server.HasResourceSubscription

	slog.Info("one_user_server_start",
		"version", effective,
		"transport", "stdio",
		"workspace", cfg.WorkspaceID,
		"base_url", cfg.BaseURL,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

func runDoctor(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		_, _ = fmt.Fprintf(stderr, "unknown doctor argument %q\n", args[0])
		_, _ = fmt.Fprintln(stderr, "usage: clockify-mcp doctor")
		return 2
	}
	cfg, err := config.LoadOneUser()
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "clockify-mcp doctor - one-user configuration\n\n")
		_, _ = fmt.Fprintf(stdout, "Result: ERROR\n")
		_, _ = fmt.Fprintf(stdout, "Recovery: set CLOCKIFY_API_KEY and CLOCKIFY_WORKSPACE_ID for the one pinned workspace, then run doctor again.\n")
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "clockify-mcp doctor - one-user configuration\n\n")
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_API_KEY       set (redacted)\n")
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_WORKSPACE_ID  %s\n", cfg.WorkspaceID)
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_TIMEZONE      %s\n", optionalValue(cfg.Timezone, "(local/default)"))
	_, _ = fmt.Fprintf(stdout, "CLOCKIFY_BASE_URL      %s\n", cfg.BaseURL)
	_, _ = fmt.Fprintf(stdout, "MCP_LOG_LEVEL          %s\n", optionalValue(cfg.LogLevel, "info"))
	_, _ = fmt.Fprintf(stdout, "\nResult: OK\n")
	return 0
}

func optionalValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

// printHelp emits the one-user branch help banner.
func printHelp() {
	// Writes to stderr never fail in any actionable way at this call
	// site — if the OS-level write has gone south, help-text drops
	// are the least of our problems. Explicit _, _ = satisfies the
	// errcheck linter.
	w := os.Stderr
	_, _ = fmt.Fprintf(w, "clockify-mcp v%s — one-user full-access Clockify MCP\n\n", effectiveVersion())
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  clockify-mcp                 Start the stdio MCP server")
	_, _ = fmt.Fprintln(w, "  clockify-mcp doctor          Validate the one-user configuration")
	_, _ = fmt.Fprintln(w, "  clockify-mcp --version | -v  Print version and exit")
	_, _ = fmt.Fprintln(w, "  clockify-mcp --help    | -h  Print this help and exit")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Environment:")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_API_KEY       required")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_WORKSPACE_ID  required")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_TIMEZONE      optional")
	_, _ = fmt.Fprintln(w, "  CLOCKIFY_BASE_URL      optional, defaults to https://api.clockify.me/api/v1")
	_, _ = fmt.Fprintln(w, "  MCP_LOG_LEVEL          optional: debug, info, warn, error")
}
