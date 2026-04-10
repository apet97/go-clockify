package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.clockify.me/api/v1"

type Config struct {
	// Clockify
	APIKey         string
	WorkspaceID    string
	BaseURL        string
	RequestTimeout time.Duration
	MaxRetries     int
	Insecure       bool
	Timezone       string

	// MCP transport
	Transport      string
	HTTPBind       string
	BearerToken    string
	AllowedOrigins []string
	AllowAnyOrigin bool
	MaxBodySize    int64

	// Tool execution
	ToolTimeout time.Duration

	// Dispatch-layer concurrency bound for stdio tools/call. 0 disables.
	MaxInFlightToolCalls int

	// Hard cap on entries aggregated by report tools. 0 disables.
	ReportMaxEntries int
}

func Load() (Config, error) {
	cfg := Config{
		APIKey:      os.Getenv("CLOCKIFY_API_KEY"),
		WorkspaceID: os.Getenv("CLOCKIFY_WORKSPACE_ID"),
		BaseURL:     strings.TrimRight(os.Getenv("CLOCKIFY_BASE_URL"), "/"),
		Insecure:    os.Getenv("CLOCKIFY_INSECURE") == "1",
	}

	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("CLOCKIFY_API_KEY is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if err := validateBaseURL(cfg.BaseURL, cfg.Insecure); err != nil {
		return Config{}, err
	}

	cfg.RequestTimeout = 30 * time.Second
	cfg.MaxRetries = 3

	// Timezone
	cfg.Timezone = os.Getenv("CLOCKIFY_TIMEZONE")
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_TIMEZONE %q: %w", cfg.Timezone, err)
		}
	}

	// MCP transport settings
	cfg.Transport = os.Getenv("MCP_TRANSPORT")
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	switch cfg.Transport {
	case "stdio", "http":
		// valid
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT %q: must be \"stdio\" or \"http\"", cfg.Transport)
	}

	cfg.HTTPBind = os.Getenv("MCP_HTTP_BIND")
	if cfg.HTTPBind == "" {
		cfg.HTTPBind = ":8080"
	}

	cfg.BearerToken = os.Getenv("MCP_BEARER_TOKEN")
	if cfg.Transport == "http" {
		if cfg.BearerToken == "" {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN is required when MCP_TRANSPORT=http")
		}
		if len(cfg.BearerToken) < 16 {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN must be at least 16 characters for security")
		}
	}

	if origins := os.Getenv("MCP_ALLOWED_ORIGINS"); origins != "" {
		parts := strings.Split(origins, ",")
		cfg.AllowedOrigins = make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	cfg.AllowAnyOrigin = os.Getenv("MCP_ALLOW_ANY_ORIGIN") == "1"

	cfg.MaxBodySize = 2097152 // 2 MB default
	if mbs := os.Getenv("MCP_HTTP_MAX_BODY"); mbs != "" {
		v, err := strconv.ParseInt(mbs, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_HTTP_MAX_BODY: %w", err)
		}
		if v <= 0 {
			return Config{}, fmt.Errorf("MCP_HTTP_MAX_BODY must be greater than 0")
		}
		if v > 52428800 {
			return Config{}, fmt.Errorf("MCP_HTTP_MAX_BODY must be at most 50 MB (52428800)")
		}
		cfg.MaxBodySize = v
	}

	// Tool timeout
	cfg.ToolTimeout = 45 * time.Second
	if tt := os.Getenv("CLOCKIFY_TOOL_TIMEOUT"); tt != "" {
		d, err := time.ParseDuration(tt)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_TOOL_TIMEOUT %q: %w", tt, err)
		}
		if d < 5*time.Second || d > 10*time.Minute {
			return Config{}, fmt.Errorf("CLOCKIFY_TOOL_TIMEOUT must be between 5s and 10m")
		}
		cfg.ToolTimeout = d
	}

	cfg.MaxInFlightToolCalls = 64
	if v := os.Getenv("MCP_MAX_INFLIGHT_TOOL_CALLS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_MAX_INFLIGHT_TOOL_CALLS %q: %w", v, err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("MCP_MAX_INFLIGHT_TOOL_CALLS must be >= 0")
		}
		if n > 10000 {
			return Config{}, fmt.Errorf("MCP_MAX_INFLIGHT_TOOL_CALLS must be <= 10000")
		}
		cfg.MaxInFlightToolCalls = n
	}

	cfg.ReportMaxEntries = 10000
	if v := os.Getenv("CLOCKIFY_REPORT_MAX_ENTRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_REPORT_MAX_ENTRIES %q: %w", v, err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("CLOCKIFY_REPORT_MAX_ENTRIES must be >= 0")
		}
		cfg.ReportMaxEntries = n
	}

	return cfg, nil
}

func validateBaseURL(raw string, insecure bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("CLOCKIFY_BASE_URL must be an absolute URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme != "http" {
		return fmt.Errorf("unsupported CLOCKIFY_BASE_URL scheme: %s", u.Scheme)
	}
	if isLoopbackHost(u.Hostname()) || insecure {
		return nil
	}
	return fmt.Errorf("insecure CLOCKIFY_BASE_URL requires loopback host or CLOCKIFY_INSECURE=1")
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
