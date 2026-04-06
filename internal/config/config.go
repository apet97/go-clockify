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
	ReportsURL     string
	Timezone       string

	// MCP transport
	Transport      string
	HTTPBind       string
	BearerToken    string
	AllowedOrigins []string
	MaxBodySize    int64
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

	// Reports URL
	cfg.ReportsURL = strings.TrimRight(os.Getenv("CLOCKIFY_REPORTS_URL"), "/")

	// Timezone
	cfg.Timezone = os.Getenv("CLOCKIFY_TIMEZONE")

	// MCP transport settings
	cfg.Transport = os.Getenv("MCP_TRANSPORT")
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}

	cfg.HTTPBind = os.Getenv("MCP_HTTP_BIND")
	if cfg.HTTPBind == "" {
		cfg.HTTPBind = ":8080"
	}

	cfg.BearerToken = os.Getenv("MCP_BEARER_TOKEN")

	if origins := os.Getenv("MCP_ALLOWED_ORIGINS"); origins != "" {
		parts := strings.Split(origins, ",")
		cfg.AllowedOrigins = make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	cfg.MaxBodySize = 2097152 // 2 MB default
	if mbs := os.Getenv("MCP_HTTP_MAX_BODY"); mbs != "" {
		if v, err := strconv.ParseInt(mbs, 10, 64); err == nil {
			cfg.MaxBodySize = v
		}
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
