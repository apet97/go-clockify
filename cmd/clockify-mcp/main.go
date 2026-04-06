package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"goclmcp/internal/bootstrap"
	"goclmcp/internal/clockify"
	"goclmcp/internal/config"
	"goclmcp/internal/dedupe"
	"goclmcp/internal/dryrun"
	"goclmcp/internal/mcp"
	"goclmcp/internal/policy"
	"goclmcp/internal/ratelimit"
	"goclmcp/internal/tools"
	"goclmcp/internal/truncate"
)

const version = "0.2.0"

func main() {
	// Configure slog to stderr
	var logHandler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	if os.Getenv("MCP_LOG_FORMAT") == "json" {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
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
	service := tools.New(client, cfg.WorkspaceID)
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

	server := mcp.NewServer(version, pol, registry, rl, tc, dc, &bc)

	slog.Info("server_start",
		"version", version,
		"tools", len(registry),
		"policy", string(pol.Mode),
		"bootstrap", bc.Mode.String(),
	)

	if cfg.Transport == "http" {
		return server.ServeHTTP(context.Background(), cfg.HTTPBind, cfg.BearerToken, cfg.AllowedOrigins, cfg.MaxBodySize)
	}
	return server.Run(context.Background(), os.Stdin, os.Stdout)
}
