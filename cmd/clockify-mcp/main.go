package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/tools"
	"github.com/apet97/go-clockify/internal/truncate"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v0.4.1" ./cmd/clockify-mcp
var version = "0.4.1"

func main() {
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

	// Configure slog to stderr
	var logHandler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	if os.Getenv("MCP_LOG_FORMAT") == "json" {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	}
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

	rl := ratelimit.FromEnv()
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

	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
	defer client.Close()
	client.SetUserAgent("clockify-mcp-go/" + version)
	service := tools.New(client, cfg.WorkspaceID)
	if cfg.Timezone != "" {
		loc, _ := time.LoadLocation(cfg.Timezone) // already validated in config.Load
		service.DefaultTimezone = loc
	}
	service.DedupeConfig = &dd
	service.PolicyDescribe = pol.Describe

	registry := service.Registry()

	// Set Tier 1 tool names on policy and bootstrap
	tier1Names := make(map[string]bool, len(registry))
	for _, d := range registry {
		tier1Names[d.Tool.Name] = true
	}
	pol.SetTier1Tools(tier1Names)
	bc.SetTier1Tools(tier1Names)

	pipeline := &enforcement.Pipeline{
		Policy:     pol,
		Bootstrap:  &bc,
		RateLimit:  rl,
		DryRun:     dc,
		Truncation: tc,
	}
	gate := &enforcement.Gate{
		Policy:    pol,
		Bootstrap: &bc,
	}
	server := mcp.NewServer(version, registry, pipeline, gate)
	server.ToolTimeout = cfg.ToolTimeout
	server.MaxInFlightToolCalls = cfg.MaxInFlightToolCalls

	service.ActivateGroup = func(group string) (tools.ActivationResult, error) {
		descriptors, ok := service.Tier2Handlers(group)
		if !ok {
			return tools.ActivationResult{}, fmt.Errorf("unknown group: %s", group)
		}
		if err := server.ActivateGroup(group, descriptors); err != nil {
			return tools.ActivationResult{}, err
		}
		return tools.ActivationResult{
			Kind:      "group",
			Name:      group,
			Group:     group,
			ToolCount: len(descriptors),
		}, nil
	}

	service.ActivateTool = func(name string) (tools.ActivationResult, error) {
		if tier1Names[name] {
			if err := server.ActivateTier1Tool(name); err != nil {
				return tools.ActivationResult{}, err
			}
			return tools.ActivationResult{
				Kind:      "tool",
				Name:      name,
				ToolCount: 1,
			}, nil
		}
		for groupName := range tools.Tier2Groups {
			descriptors, ok := service.Tier2Handlers(groupName)
			if !ok {
				continue
			}
			for _, d := range descriptors {
				if d.Tool.Name != name {
					continue
				}
				if err := server.ActivateGroup(groupName, descriptors); err != nil {
					return tools.ActivationResult{}, err
				}
				return tools.ActivationResult{
					Kind:      "tool",
					Name:      name,
					Group:     groupName,
					ToolCount: len(descriptors),
				}, nil
			}
		}
		return tools.ActivationResult{}, fmt.Errorf("unknown tool: %s", name)
	}

	slog.Info("server_start",
		"version", version,
		"tools", len(registry),
		"policy", string(pol.Mode),
		"bootstrap", bc.Mode.String(),
		"transport", cfg.Transport,
		"workspace", cfg.WorkspaceID,
	)

	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cfg.Transport == "http" {
		if cfg.AllowAnyOrigin {
			slog.Warn("cors_any_origin", "msg", "MCP_ALLOW_ANY_ORIGIN=1 is set — all cross-origin requests will be accepted. This is not recommended for production.")
		}
		// Wire upstream health check: lightweight GET /api/v1/user
		server.ReadyChecker = func(ctx context.Context) error {
			var user struct{ ID string }
			return client.Get(ctx, "/user", nil, &user)
		}
		return server.ServeHTTP(ctx, cfg.HTTPBind, cfg.BearerToken, cfg.AllowedOrigins, cfg.AllowAnyOrigin, cfg.MaxBodySize)
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
    CLOCKIFY_API_KEY          API key (required)
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
    CLOCKIFY_RATE_LIMIT            Tool calls per minute, 0=off (default: 120)
    CLOCKIFY_TOKEN_BUDGET          Response token budget, 0=off (default: 8000)
    MCP_MAX_INFLIGHT_TOOL_CALLS    Stdio dispatch goroutine cap, 0=off (default: 64)
    CLOCKIFY_REPORT_MAX_ENTRIES    Hard cap on entries aggregated by report tools, 0=off (default: 10000)

  Bootstrap:
    CLOCKIFY_BOOTSTRAP_MODE   full_tier1 (default), minimal, custom
    CLOCKIFY_BOOTSTRAP_TOOLS  Tool list for custom mode

  Transport:
    MCP_TRANSPORT             stdio (default) or http
    MCP_HTTP_BIND             HTTP listen address (default: :8080)
    MCP_BEARER_TOKEN          Required for HTTP mode; send as Authorization: Bearer <token>
    MCP_ALLOWED_ORIGINS       Comma-separated CORS origins
    MCP_ALLOW_ANY_ORIGIN      Set 1 to allow all origins
    MCP_HTTP_MAX_BODY         Positive max request body in bytes (default: 2097152)

  Logging:
    MCP_LOG_FORMAT            text (default) or json
    MCP_LOG_LEVEL             debug, info (default), warn, error
`, version)
}
