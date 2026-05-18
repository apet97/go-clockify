package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/resolve"
)

const DefaultBaseURL = "https://api.clockify.me/api/v1"

const (
	DefaultMaxInFlightToolCalls           = 4
	DefaultToolTimeout                    = 45 * time.Second
	MinToolTimeout                        = 5 * time.Second
	MaxToolTimeout                        = 10 * time.Minute
	DefaultMaxMessageSize                 = 4 * 1024 * 1024
	DefaultMaxToolResultBytes             = 50_000
	MaxMessageSize                        = 100 * 1024 * 1024
	MaxToolResultBytes                    = 100 * 1024 * 1024
	DefaultToolset                        = "all"
	DefaultCircuitBreakerFailureThreshold = 5
	DefaultCircuitBreakerOpenDuration     = 45 * time.Second
	DefaultCircuitBreakerHalfOpenProbes   = 1
)

// OneUserConfig is the complete runtime configuration for the
// one-user/full-access stdio product path.
type OneUserConfig struct {
	APIKey                 string
	WorkspaceID            string
	Timezone               string
	BaseURL                string
	LogLevel               string
	MaxInFlightToolCalls   int
	ToolTimeout            time.Duration
	MaxMessageSize         int64
	MaxToolResultBytes     int
	Toolset                string
	EnableRawWrites        bool
	RawWriteDocumentedOnly bool
	WebhookAllowedDomains  []string
	CircuitBreaker         clockify.CircuitBreakerConfig
}

// LoadOneUser reads the intentionally tiny environment surface used by the
// one-user stdio runtime. It ignores the broader platform-era configuration
// matrix by design.
func LoadOneUser() (OneUserConfig, error) {
	cfg := OneUserConfig{
		APIKey:                 strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY")),
		WorkspaceID:            strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID")),
		Timezone:               strings.TrimSpace(os.Getenv("CLOCKIFY_TIMEZONE")),
		BaseURL:                strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL")),
		LogLevel:               strings.TrimSpace(os.Getenv("MCP_LOG_LEVEL")),
		MaxInFlightToolCalls:   DefaultMaxInFlightToolCalls,
		ToolTimeout:            DefaultToolTimeout,
		MaxMessageSize:         DefaultMaxMessageSize,
		MaxToolResultBytes:     DefaultMaxToolResultBytes,
		Toolset:                DefaultToolset,
		RawWriteDocumentedOnly: true,
		WebhookAllowedDomains:  parseCommaListEnv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS"),
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
	rawWriteDocumentedOnly, err := parseBoolEnvDefault("CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY", true)
	if err != nil {
		return OneUserConfig{}, err
	}
	cfg.RawWriteDocumentedOnly = rawWriteDocumentedOnly
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
	case "core", "business", "admin", "all":
		return raw, nil
	default:
		return "", fmt.Errorf("CLOCKIFY_TOOLSET must be one of core, business, admin, all")
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
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: HTTPS is required unless the host is loopback")
	}
	if u.Host == "" {
		return fmt.Errorf("invalid CLOCKIFY_BASE_URL: host is required")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}
