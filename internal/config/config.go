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
	Transport          string
	AuthMode           string
	HTTPBind           string
	BearerToken        string
	AllowedOrigins     []string
	AllowAnyOrigin     bool
	StrictHostCheck    bool
	MaxBodySize        int64
	MetricsBind        string
	MetricsAuthMode    string
	MetricsBearerToken string

	// Enterprise shared-service
	ControlPlaneDSN      string
	SessionTTL           time.Duration
	TenantClaim          string
	SubjectClaim         string
	DefaultTenantID      string
	OIDCIssuer           string
	OIDCAudience         string
	OIDCJWKSURL          string
	OIDCJWKSPath         string
	ForwardTenantHeader  string
	ForwardSubjectHeader string
	MTLSTenantHeader     string

	// Tool execution
	ToolTimeout               time.Duration
	ConcurrencyAcquireTimeout time.Duration

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
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
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
	case "stdio", "http", "streamable_http":
		// valid
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT %q: must be \"stdio\", \"http\", or \"streamable_http\"", cfg.Transport)
	}
	if cfg.Transport != "streamable_http" && cfg.APIKey == "" {
		return Config{}, fmt.Errorf("CLOCKIFY_API_KEY is required")
	}
	if cfg.APIKey != "" {
		if err := validateBaseURL(cfg.BaseURL, cfg.Insecure); err != nil {
			return Config{}, err
		}
	}

	cfg.AuthMode = strings.TrimSpace(os.Getenv("MCP_AUTH_MODE"))
	if cfg.AuthMode == "" {
		switch cfg.Transport {
		case "streamable_http":
			cfg.AuthMode = "oidc"
		case "http":
			cfg.AuthMode = "static_bearer"
		default:
			cfg.AuthMode = ""
		}
	}
	switch cfg.AuthMode {
	case "", "static_bearer", "oidc", "forward_auth", "mtls":
	default:
		return Config{}, fmt.Errorf("invalid MCP_AUTH_MODE %q", cfg.AuthMode)
	}
	if cfg.Transport == "stdio" && cfg.AuthMode != "" {
		return Config{}, fmt.Errorf("MCP_AUTH_MODE is only valid for HTTP transports")
	}

	cfg.HTTPBind = os.Getenv("MCP_HTTP_BIND")
	if cfg.HTTPBind == "" {
		cfg.HTTPBind = ":8080"
	}

	cfg.BearerToken = os.Getenv("MCP_BEARER_TOKEN")
	if cfg.Transport != "stdio" && cfg.AuthMode == "static_bearer" {
		if cfg.BearerToken == "" {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN is required for static bearer auth")
		}
		if len(cfg.BearerToken) < 16 {
			return Config{}, fmt.Errorf("MCP_BEARER_TOKEN must be at least 16 characters for security")
		}
	}
	cfg.MetricsBind = strings.TrimSpace(os.Getenv("MCP_METRICS_BIND"))
	cfg.MetricsAuthMode = strings.TrimSpace(os.Getenv("MCP_METRICS_AUTH_MODE"))
	if cfg.MetricsAuthMode == "" {
		cfg.MetricsAuthMode = "none"
	}
	switch cfg.MetricsAuthMode {
	case "none", "static_bearer":
	default:
		return Config{}, fmt.Errorf("invalid MCP_METRICS_AUTH_MODE %q", cfg.MetricsAuthMode)
	}
	cfg.MetricsBearerToken = strings.TrimSpace(os.Getenv("MCP_METRICS_BEARER_TOKEN"))
	if cfg.MetricsAuthMode == "static_bearer" {
		if cfg.MetricsBearerToken == "" {
			return Config{}, fmt.Errorf("MCP_METRICS_BEARER_TOKEN is required when MCP_METRICS_AUTH_MODE=static_bearer")
		}
		if len(cfg.MetricsBearerToken) < 16 {
			return Config{}, fmt.Errorf("MCP_METRICS_BEARER_TOKEN must be at least 16 characters for security")
		}
	}

	cfg.ControlPlaneDSN = strings.TrimSpace(os.Getenv("MCP_CONTROL_PLANE_DSN"))
	if cfg.ControlPlaneDSN == "" {
		cfg.ControlPlaneDSN = "memory"
	}
	cfg.SessionTTL = 30 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("MCP_SESSION_TTL")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_SESSION_TTL %q: %w", raw, err)
		}
		if d < time.Minute || d > 24*time.Hour {
			return Config{}, fmt.Errorf("MCP_SESSION_TTL must be between 1m and 24h")
		}
		cfg.SessionTTL = d
	}
	cfg.TenantClaim = strings.TrimSpace(os.Getenv("MCP_TENANT_CLAIM"))
	if cfg.TenantClaim == "" {
		cfg.TenantClaim = "tenant_id"
	}
	cfg.SubjectClaim = strings.TrimSpace(os.Getenv("MCP_SUBJECT_CLAIM"))
	if cfg.SubjectClaim == "" {
		cfg.SubjectClaim = "sub"
	}
	cfg.DefaultTenantID = strings.TrimSpace(os.Getenv("MCP_DEFAULT_TENANT_ID"))
	if cfg.DefaultTenantID == "" {
		cfg.DefaultTenantID = "default"
	}
	cfg.OIDCIssuer = strings.TrimSpace(os.Getenv("MCP_OIDC_ISSUER"))
	cfg.OIDCAudience = strings.TrimSpace(os.Getenv("MCP_OIDC_AUDIENCE"))
	cfg.OIDCJWKSURL = strings.TrimSpace(os.Getenv("MCP_OIDC_JWKS_URL"))
	cfg.OIDCJWKSPath = strings.TrimSpace(os.Getenv("MCP_OIDC_JWKS_PATH"))
	cfg.ForwardTenantHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_TENANT_HEADER"))
	cfg.ForwardSubjectHeader = strings.TrimSpace(os.Getenv("MCP_FORWARD_SUBJECT_HEADER"))
	cfg.MTLSTenantHeader = strings.TrimSpace(os.Getenv("MCP_MTLS_TENANT_HEADER"))
	if cfg.AuthMode == "oidc" && cfg.Transport == "streamable_http" && cfg.OIDCIssuer == "" {
		return Config{}, fmt.Errorf("MCP_OIDC_ISSUER is required when MCP_TRANSPORT=streamable_http and MCP_AUTH_MODE=oidc")
	}
	if cfg.Transport == "streamable_http" && cfg.ControlPlaneDSN == "" {
		return Config{}, fmt.Errorf("MCP_CONTROL_PLANE_DSN is required for streamable_http")
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
	strictHostCheck, err := optionalBoolEnv("MCP_STRICT_HOST_CHECK")
	if err != nil {
		return Config{}, err
	}
	cfg.StrictHostCheck = strictHostCheck

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

	cfg.ConcurrencyAcquireTimeout = 100 * time.Millisecond
	if tt := os.Getenv("CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT"); tt != "" {
		d, err := time.ParseDuration(tt)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT %q: %w", tt, err)
		}
		if d < time.Millisecond || d > 30*time.Second {
			return Config{}, fmt.Errorf("CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT must be between 1ms and 30s")
		}
		cfg.ConcurrencyAcquireTimeout = d
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

func (c Config) Fingerprint() map[string]any {
	return map[string]any{
		"transport":               c.Transport,
		"auth_mode":               c.AuthMode,
		"http_bind":               c.HTTPBind,
		"metrics_bind":            c.MetricsBind,
		"metrics_auth_mode":       c.MetricsAuthMode,
		"clockify_base_url":       c.BaseURL,
		"workspace_id":            c.WorkspaceID,
		"timezone":                c.Timezone,
		"policy_claim_tenant":     c.TenantClaim,
		"policy_claim_subject":    c.SubjectClaim,
		"default_tenant_id":       c.DefaultTenantID,
		"control_plane_dsn":       c.ControlPlaneDSN,
		"session_ttl":             c.SessionTTL.String(),
		"allow_any_origin":        c.AllowAnyOrigin,
		"strict_host_check":       c.StrictHostCheck,
		"http_max_body_bytes":     c.MaxBodySize,
		"tool_timeout":            c.ToolTimeout.String(),
		"max_inflight_tool_calls": c.MaxInFlightToolCalls,
		"report_max_entries":      c.ReportMaxEntries,
	}
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

func optionalBoolEnv(key string) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: must be a boolean", key, raw)
	}
	return value, nil
}
