package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/resolve"
)

// DefaultBaseURL is the production Clockify REST API base used when
// CLOCKIFY_BASE_URL is not set. LoadOneUser rejects overrides that do not
// point at a documented Clockify host (api.clockify.me,
// reports.api.clockify.me, auditlog-api.api.clockify.me) or at a loopback
// target (localhost / 127.x.x.x, for the fake-server fixture and local
// proxies), unless CLOCKIFY_ALLOW_CUSTOM_BASE_URL=true is set to permit an
// arbitrary HTTPS host. The allowlist prevents a stray override from
// silently exfiltrating the API key to a non-Clockify endpoint.
const DefaultBaseURL = "https://api.clockify.me/api/v1"

// allowedBaseURLHosts is the set of bare hostnames LoadOneUser accepts for
// CLOCKIFY_BASE_URL without the explicit custom-override opt-in. It mirrors
// the documented Clockify API hosts the client targets
// (clockify.DocumentedHost*).
var allowedBaseURLHosts = map[string]struct{}{
	"api.clockify.me":              {},
	"reports.api.clockify.me":      {},
	"auditlog-api.api.clockify.me": {},
}

// Runtime defaults and ceilings for OneUserConfig. Defaults apply when
// the corresponding environment variable is unset or empty; Min/Max
// constants bound user-supplied overrides so a misconfigured env cannot
// disable tool timeouts or accept unbounded request bodies. The rate
// limit defaults are per-minute token-bucket budgets and `0` means
// unlimited (the limiter is disabled).
const (
	DefaultMaxInFlightToolCalls           = 4
	DefaultToolTimeout                    = 45 * time.Second
	MinToolTimeout                        = 5 * time.Second
	MaxToolTimeout                        = 10 * time.Minute
	DefaultMaxMessageSize                 = 4 * 1024 * 1024
	DefaultMaxToolResultBytes             = 50_000
	MaxMessageSize                        = 100 * 1024 * 1024
	MaxToolResultBytes                    = 100 * 1024 * 1024
	DefaultToolset                        = "default"
	DefaultToolRateLimitPerMinute         = 120
	DefaultReadRateLimitPerMinute         = 120
	DefaultWriteRateLimitPerMinute        = 30
	DefaultBillingAdminRateLimitPerMinute = 10
	DefaultDestructiveRateLimitPerMinute  = 5
	DefaultCircuitBreakerFailureThreshold = 5
	DefaultCircuitBreakerOpenDuration     = 45 * time.Second
	DefaultCircuitBreakerHalfOpenProbes   = 1
)

// OneUserConfig is the complete runtime configuration for the
// one-user/full-access stdio product path.
type OneUserConfig struct {
	APIKey                         string
	WorkspaceID                    string
	Timezone                       string
	BaseURL                        string
	LogLevel                       string
	MaxInFlightToolCalls           int
	ToolTimeout                    time.Duration
	MaxMessageSize                 int64
	MaxToolResultBytes             int
	ToolRateLimitPerMinute         int
	ToolRateLimitDisabled          bool
	ReadRateLimitPerMinute         int
	WriteRateLimitPerMinute        int
	BillingAdminRateLimitPerMinute int
	DestructiveRateLimitPerMinute  int
	Toolset                        string
	EnableRawTools                 bool
	EnableRawGet                   bool
	EnableRawWrites                bool
	RawWriteDocumentedOnly         bool
	AuditLogPath                   string
	AuditLogMode                   string
	WebhookAllowedDomains          []string
	CircuitBreaker                 clockify.CircuitBreakerConfig
}

// LoadOneUser reads the intentionally tiny environment surface used by the
// one-user stdio runtime. It ignores the broader platform-era configuration
// matrix by design.
func LoadOneUser() (OneUserConfig, error) {
	cfg := OneUserConfig{
		APIKey:                         strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY")),
		WorkspaceID:                    strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID")),
		Timezone:                       strings.TrimSpace(os.Getenv("CLOCKIFY_TIMEZONE")),
		BaseURL:                        strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL")),
		LogLevel:                       strings.TrimSpace(os.Getenv("MCP_LOG_LEVEL")),
		MaxInFlightToolCalls:           DefaultMaxInFlightToolCalls,
		ToolTimeout:                    DefaultToolTimeout,
		MaxMessageSize:                 DefaultMaxMessageSize,
		MaxToolResultBytes:             DefaultMaxToolResultBytes,
		ToolRateLimitPerMinute:         DefaultToolRateLimitPerMinute,
		ReadRateLimitPerMinute:         DefaultReadRateLimitPerMinute,
		WriteRateLimitPerMinute:        DefaultWriteRateLimitPerMinute,
		BillingAdminRateLimitPerMinute: DefaultBillingAdminRateLimitPerMinute,
		DestructiveRateLimitPerMinute:  DefaultDestructiveRateLimitPerMinute,
		Toolset:                        DefaultToolset,
		RawWriteDocumentedOnly:         true,
		WebhookAllowedDomains:          parseCommaListEnv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS"),
		CircuitBreaker: clockify.CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: DefaultCircuitBreakerFailureThreshold,
			OpenDuration:     DefaultCircuitBreakerOpenDuration,
			HalfOpenProbes:   DefaultCircuitBreakerHalfOpenProbes,
		},
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.APIKey == "" {
		return OneUserConfig{}, fmt.Errorf("CLOCKIFY_API_KEY is required")
	}
	if cfg.WorkspaceID == "" {
		return OneUserConfig{}, fmt.Errorf("CLOCKIFY_WORKSPACE_ID is required")
	}
	if err := resolve.ValidateID(cfg.WorkspaceID, "CLOCKIFY_WORKSPACE_ID"); err != nil {
		return OneUserConfig{}, err
	}
	if cfg.Timezone != "" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			return OneUserConfig{}, fmt.Errorf("invalid CLOCKIFY_TIMEZONE: %w", err)
		}
	}
	if err := validateOneUserBaseURL(cfg.BaseURL); err != nil {
		return OneUserConfig{}, err
	}
	enableRawWrites, err := parseBoolEnv("CLOCKIFY_ENABLE_RAW_WRITES")
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.EnableRawWrites = enableRawWrites
	enableRawTools, err := parseBoolEnv("CLOCKIFY_ENABLE_RAW_TOOLS")
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.EnableRawTools = enableRawTools
	enableRawGet, err := parseBoolEnv("CLOCKIFY_ENABLE_RAW_GET")
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.EnableRawGet = enableRawGet
	rawWriteDocumentedOnly, err := parseBoolEnvDefault("CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY", true)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.RawWriteDocumentedOnly = rawWriteDocumentedOnly
	cfg.AuditLogPath = strings.TrimSpace(os.Getenv("CLOCKIFY_AUDIT_LOG"))
	auditLogMode, err := parseAuditLogModeEnv()
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.AuditLogMode = auditLogMode
	maxInFlight, err := parsePositiveIntEnv("CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS", DefaultMaxInFlightToolCalls)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.MaxInFlightToolCalls = maxInFlight
	toolTimeout, err := parseDurationEnv("CLOCKIFY_TOOL_TIMEOUT", DefaultToolTimeout, MinToolTimeout, MaxToolTimeout)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.ToolTimeout = toolTimeout
	maxMessageSize, err := parseBoundedPositiveInt64Env("CLOCKIFY_MAX_MESSAGE_SIZE", DefaultMaxMessageSize, MaxMessageSize)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.MaxMessageSize = maxMessageSize
	maxToolResultBytes, err := parseBoundedPositiveIntEnv("CLOCKIFY_MAX_TOOL_RESULT_BYTES", DefaultMaxToolResultBytes, MaxToolResultBytes)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.MaxToolResultBytes = maxToolResultBytes
	rateLimit, rateLimitSet, err := parseNonNegativeIntEnvPresence("CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE", DefaultToolRateLimitPerMinute)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.ToolRateLimitPerMinute = rateLimit
	if rateLimitSet && rateLimit == 0 {
		cfg.ToolRateLimitDisabled = true
		cfg.ReadRateLimitPerMinute = 0
		cfg.WriteRateLimitPerMinute = 0
		cfg.BillingAdminRateLimitPerMinute = 0
		cfg.DestructiveRateLimitPerMinute = 0
	} else {
		cfg.ReadRateLimitPerMinute = rateLimit
		cfg.WriteRateLimitPerMinute = min(rateLimit, DefaultWriteRateLimitPerMinute)
		cfg.BillingAdminRateLimitPerMinute = min(rateLimit, DefaultBillingAdminRateLimitPerMinute)
		cfg.DestructiveRateLimitPerMinute = min(rateLimit, DefaultDestructiveRateLimitPerMinute)
	}
	toolset, err := parseToolsetEnv()
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.Toolset = toolset
	breaker, err := parseCircuitBreakerEnv()
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.CircuitBreaker = breaker
	return cfg, nil
}

func parseBoolEnv(name string) (bool, error) {
	return parseBoolEnvDefault(name, false)
}

func parseBoolEnvDefault(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return v, nil
}

func parseCommaListEnv(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parsePositiveIntEnv(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return v, nil
}

func parseNonNegativeIntEnvPresence(name string, def int) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, false, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, true, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return v, true, nil
}

func parseBoundedPositiveIntEnv(name string, fallback, maxValue int) (int, error) {
	v, err := parsePositiveIntEnv(name, fallback)
	if err != nil {
		return 0, err
	}
	if v > maxValue {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maxValue)
	}
	return v, nil
}

func parsePositiveInt64Env(name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return v, nil
}

func parseBoundedPositiveInt64Env(name string, fallback, maxValue int64) (int64, error) {
	v, err := parsePositiveInt64Env(name, fallback)
	if err != nil {
		return 0, err
	}
	if v > maxValue {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maxValue)
	}
	return v, nil
}

func parseDurationEnv(name string, fallback, minValue, maxValue time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration like 45s or 2m: %w", name, err)
	}
	if v < minValue || v > maxValue {
		return 0, fmt.Errorf("%s must be between %s and %s", name, minValue, maxValue)
	}
	return v, nil
}

func parseToolsetEnv() (string, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_TOOLSET")))
	if raw == "" {
		return DefaultToolset, nil
	}
	switch raw {
	case "default", "core", "business", "admin", "all":
		return raw, nil
	default:
		return "", fmt.Errorf("CLOCKIFY_TOOLSET must be one of default, core, business, admin, all")
	}
}

func parseAuditLogModeEnv() (string, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_AUDIT_LOG_MODE")))
	if raw == "" {
		return "off", nil
	}
	switch raw {
	case "off", "side_effects_only", "all":
		return raw, nil
	default:
		return "", fmt.Errorf("CLOCKIFY_AUDIT_LOG_MODE must be one of off, side_effects_only, all")
	}
}

func parseCircuitBreakerEnv() (clockify.CircuitBreakerConfig, error) {
	enabled, err := parseCircuitBreakerModeEnv()
	if err != nil {
		return clockify.CircuitBreakerConfig{}, err
	}
	threshold, err := parsePositiveIntEnv("CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD", DefaultCircuitBreakerFailureThreshold)
	if err != nil {
		return clockify.CircuitBreakerConfig{}, err
	}
	openDuration, err := parsePositiveDurationEnv("CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION", DefaultCircuitBreakerOpenDuration)
	if err != nil {
		return clockify.CircuitBreakerConfig{}, err
	}
	halfOpenProbes, err := parsePositiveIntEnv("CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES", DefaultCircuitBreakerHalfOpenProbes)
	if err != nil {
		return clockify.CircuitBreakerConfig{}, err
	}
	return clockify.CircuitBreakerConfig{
		Enabled:          enabled,
		FailureThreshold: threshold,
		OpenDuration:     openDuration,
		HalfOpenProbes:   halfOpenProbes,
	}, nil
}

func parseCircuitBreakerModeEnv() (bool, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_CIRCUIT_BREAKER")))
	switch raw {
	case "", "auto", "enabled", "enable", "on", "true", "1":
		return true, nil
	case "disabled", "disable", "off", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("CLOCKIFY_CIRCUIT_BREAKER must be auto, enabled, or disabled")
	}
}

func parsePositiveDurationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration like 45s or 2m: %w", name, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return v, nil
}

func validateOneUserBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: host is required")
	}
	host := u.Hostname()
	// Loopback targets (fake-server fixtures, local proxies) may use HTTP or
	// HTTPS and are always allowed.
	if isLoopbackHost(host) {
		return nil
	}
	// Non-loopback targets must use HTTPS so the API key is never sent in the
	// clear.
	if u.Scheme != "https" {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: HTTPS is required unless the host is loopback")
	}
	if _, ok := allowedBaseURLHosts[strings.ToLower(host)]; ok {
		return nil
	}
	allowCustom, err := parseBoolEnv("CLOCKIFY_ALLOW_CUSTOM_BASE_URL")
	if err != nil {
		return err
	}
	if allowCustom {
		slog.Warn("clockify config: CLOCKIFY_BASE_URL points at a non-Clockify host; allowed only because CLOCKIFY_ALLOW_CUSTOM_BASE_URL=true. Ensure this host is trusted with your API key.",
			"host", host)
		return nil
	}
	return fmt.Errorf(
		"invalid CLOCKIFY_BASE_URL: host %q is not a documented Clockify host (api.clockify.me, reports.api.clockify.me, auditlog-api.api.clockify.me) or loopback; set CLOCKIFY_ALLOW_CUSTOM_BASE_URL=true to allow an arbitrary HTTPS host",
		host,
	)
}

func isLoopbackHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}
