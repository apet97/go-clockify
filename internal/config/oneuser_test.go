package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadOneUserRequiresAPIKeyAndWorkspace(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected missing API key error")
	}

	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected missing workspace error")
	}
}

func TestLoadOneUserMinimalConfig(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "")
	t.Setenv("CLOCKIFY_TIMEZONE", "Europe/Belgrade")
	t.Setenv("MCP_LOG_LEVEL", "debug")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.Timezone != "Europe/Belgrade" {
		t.Fatalf("Timezone = %q", cfg.Timezone)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.MaxInFlightToolCalls != DefaultMaxInFlightToolCalls {
		t.Fatalf("MaxInFlightToolCalls = %d", cfg.MaxInFlightToolCalls)
	}
	if cfg.ToolTimeout != DefaultToolTimeout {
		t.Fatalf("ToolTimeout = %s", cfg.ToolTimeout)
	}
	if cfg.MaxMessageSize != DefaultMaxMessageSize {
		t.Fatalf("MaxMessageSize = %d", cfg.MaxMessageSize)
	}
	if cfg.MaxToolResultBytes != DefaultMaxToolResultBytes {
		t.Fatalf("MaxToolResultBytes = %d", cfg.MaxToolResultBytes)
	}
	if cfg.ToolRateLimitPerMinute != DefaultToolRateLimitPerMinute {
		t.Fatalf("ToolRateLimitPerMinute = %d, want %d", cfg.ToolRateLimitPerMinute, DefaultToolRateLimitPerMinute)
	}
	if cfg.ToolRateLimitDisabled {
		t.Fatal("ToolRateLimitDisabled defaulted true")
	}
	if cfg.ReadRateLimitPerMinute != DefaultReadRateLimitPerMinute ||
		cfg.WriteRateLimitPerMinute != DefaultWriteRateLimitPerMinute ||
		cfg.BillingAdminRateLimitPerMinute != DefaultBillingAdminRateLimitPerMinute ||
		cfg.DestructiveRateLimitPerMinute != DefaultDestructiveRateLimitPerMinute {
		t.Fatalf("risk rate limits = read:%d write:%d billing_admin:%d destructive:%d",
			cfg.ReadRateLimitPerMinute,
			cfg.WriteRateLimitPerMinute,
			cfg.BillingAdminRateLimitPerMinute,
			cfg.DestructiveRateLimitPerMinute)
	}
	if cfg.Toolset != DefaultToolset {
		t.Fatalf("Toolset = %q, want %q", cfg.Toolset, DefaultToolset)
	}
	if cfg.EnableRawWrites {
		t.Fatal("EnableRawWrites defaulted true")
	}
	if cfg.EnableRawTools {
		t.Fatal("EnableRawTools defaulted true")
	}
	if cfg.EnableRawGet {
		t.Fatal("EnableRawGet defaulted true")
	}
	if !cfg.RawWriteDocumentedOnly {
		t.Fatal("RawWriteDocumentedOnly defaulted false")
	}
	if len(cfg.WebhookAllowedDomains) != 0 {
		t.Fatalf("WebhookAllowedDomains = %#v", cfg.WebhookAllowedDomains)
	}
	if !cfg.CircuitBreaker.Enabled {
		t.Fatal("CircuitBreaker defaulted disabled")
	}
	if cfg.CircuitBreaker.FailureThreshold != DefaultCircuitBreakerFailureThreshold {
		t.Fatalf("CircuitBreaker.FailureThreshold = %d", cfg.CircuitBreaker.FailureThreshold)
	}
	if cfg.CircuitBreaker.OpenDuration != DefaultCircuitBreakerOpenDuration {
		t.Fatalf("CircuitBreaker.OpenDuration = %s", cfg.CircuitBreaker.OpenDuration)
	}
	if cfg.CircuitBreaker.HalfOpenProbes != DefaultCircuitBreakerHalfOpenProbes {
		t.Fatalf("CircuitBreaker.HalfOpenProbes = %d", cfg.CircuitBreaker.HalfOpenProbes)
	}
}

func TestLoadOneUserOptionalRuntimeConfig(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS", "8")
	t.Setenv("CLOCKIFY_TOOL_TIMEOUT", "2m")
	t.Setenv("CLOCKIFY_MAX_MESSAGE_SIZE", "8388608")
	t.Setenv("CLOCKIFY_MAX_TOOL_RESULT_BYTES", "12345")
	t.Setenv("CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE", "60")
	t.Setenv("CLOCKIFY_TOOLSET", "business")
	t.Setenv("CLOCKIFY_ENABLE_RAW_WRITES", "true")
	t.Setenv("CLOCKIFY_ENABLE_RAW_TOOLS", "true")
	t.Setenv("CLOCKIFY_ENABLE_RAW_GET", "true")
	t.Setenv("CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY", "false")
	t.Setenv("CLOCKIFY_AUDIT_LOG", "/tmp/clockify-audit.jsonl")
	t.Setenv("CLOCKIFY_AUDIT_LOG_MODE", "all")
	t.Setenv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS", "hooks.example.com, .trusted.test, , api.example.com ")
	t.Setenv("CLOCKIFY_CIRCUIT_BREAKER", "disabled")
	t.Setenv("CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "7")
	t.Setenv("CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION", "90s")
	t.Setenv("CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES", "3")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxInFlightToolCalls != 8 {
		t.Fatalf("MaxInFlightToolCalls = %d", cfg.MaxInFlightToolCalls)
	}
	if cfg.ToolTimeout != 2*time.Minute {
		t.Fatalf("ToolTimeout = %s", cfg.ToolTimeout)
	}
	if cfg.MaxMessageSize != 8388608 {
		t.Fatalf("MaxMessageSize = %d", cfg.MaxMessageSize)
	}
	if cfg.MaxToolResultBytes != 12345 {
		t.Fatalf("MaxToolResultBytes = %d", cfg.MaxToolResultBytes)
	}
	if cfg.ToolRateLimitPerMinute != 60 {
		t.Fatalf("ToolRateLimitPerMinute = %d", cfg.ToolRateLimitPerMinute)
	}
	if cfg.ReadRateLimitPerMinute != 60 || cfg.WriteRateLimitPerMinute != 30 ||
		cfg.BillingAdminRateLimitPerMinute != 10 || cfg.DestructiveRateLimitPerMinute != 5 {
		t.Fatalf("risk rate limits = read:%d write:%d billing_admin:%d destructive:%d",
			cfg.ReadRateLimitPerMinute,
			cfg.WriteRateLimitPerMinute,
			cfg.BillingAdminRateLimitPerMinute,
			cfg.DestructiveRateLimitPerMinute)
	}
	if cfg.Toolset != "business" {
		t.Fatalf("Toolset = %q", cfg.Toolset)
	}
	if !cfg.EnableRawWrites {
		t.Fatal("EnableRawWrites = false")
	}
	if !cfg.EnableRawTools {
		t.Fatal("EnableRawTools = false")
	}
	if !cfg.EnableRawGet {
		t.Fatal("EnableRawGet = false")
	}
	if cfg.RawWriteDocumentedOnly {
		t.Fatal("RawWriteDocumentedOnly = true")
	}
	if cfg.AuditLogPath != "/tmp/clockify-audit.jsonl" {
		t.Fatalf("AuditLogPath = %q", cfg.AuditLogPath)
	}
	if cfg.AuditLogMode != "all" {
		t.Fatalf("AuditLogMode = %q", cfg.AuditLogMode)
	}
	wantDomains := []string{"hooks.example.com", ".trusted.test", "api.example.com"}
	if !reflect.DeepEqual(cfg.WebhookAllowedDomains, wantDomains) {
		t.Fatalf("WebhookAllowedDomains = %#v, want %#v", cfg.WebhookAllowedDomains, wantDomains)
	}
	if cfg.CircuitBreaker.Enabled {
		t.Fatal("CircuitBreaker.Enabled = true")
	}
	if cfg.CircuitBreaker.FailureThreshold != 7 {
		t.Fatalf("CircuitBreaker.FailureThreshold = %d", cfg.CircuitBreaker.FailureThreshold)
	}
	if cfg.CircuitBreaker.OpenDuration != 90*time.Second {
		t.Fatalf("CircuitBreaker.OpenDuration = %s", cfg.CircuitBreaker.OpenDuration)
	}
	if cfg.CircuitBreaker.HalfOpenProbes != 3 {
		t.Fatalf("CircuitBreaker.HalfOpenProbes = %d", cfg.CircuitBreaker.HalfOpenProbes)
	}
}

func TestLoadOneUserOptionalRuntimeConfigOnlyRequiresKeyAndWorkspace(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS", "")
	t.Setenv("CLOCKIFY_TOOL_TIMEOUT", "")
	t.Setenv("CLOCKIFY_MAX_MESSAGE_SIZE", "")
	t.Setenv("CLOCKIFY_MAX_TOOL_RESULT_BYTES", "")
	t.Setenv("CLOCKIFY_TOOLSET", "")
	t.Setenv("CLOCKIFY_ENABLE_RAW_WRITES", "")
	t.Setenv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS", "")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxInFlightToolCalls != DefaultMaxInFlightToolCalls {
		t.Fatalf("MaxInFlightToolCalls = %d", cfg.MaxInFlightToolCalls)
	}
	if cfg.ToolTimeout != DefaultToolTimeout {
		t.Fatalf("ToolTimeout = %s", cfg.ToolTimeout)
	}
	if cfg.MaxMessageSize != DefaultMaxMessageSize {
		t.Fatalf("MaxMessageSize = %d", cfg.MaxMessageSize)
	}
	if cfg.MaxToolResultBytes != DefaultMaxToolResultBytes {
		t.Fatalf("MaxToolResultBytes = %d", cfg.MaxToolResultBytes)
	}
	if cfg.Toolset != DefaultToolset {
		t.Fatalf("Toolset = %q, want %q", cfg.Toolset, DefaultToolset)
	}
	if cfg.EnableRawWrites {
		t.Fatal("EnableRawWrites defaulted true")
	}
	if cfg.EnableRawTools {
		t.Fatal("EnableRawTools defaulted true")
	}
	if cfg.EnableRawGet {
		t.Fatal("EnableRawGet defaulted true")
	}
	if len(cfg.WebhookAllowedDomains) != 0 {
		t.Fatalf("WebhookAllowedDomains = %#v", cfg.WebhookAllowedDomains)
	}
}

func TestLoadOneUserToolRateLimitExplicitZeroDisablesLimiter(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE", "0")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ToolRateLimitPerMinute != 0 {
		t.Fatalf("ToolRateLimitPerMinute = %d, want 0", cfg.ToolRateLimitPerMinute)
	}
	if !cfg.ToolRateLimitDisabled {
		t.Fatal("ToolRateLimitDisabled = false, want true")
	}
	if cfg.ReadRateLimitPerMinute != 0 || cfg.WriteRateLimitPerMinute != 0 ||
		cfg.BillingAdminRateLimitPerMinute != 0 || cfg.DestructiveRateLimitPerMinute != 0 {
		t.Fatalf("risk rate limits should be disabled, got read:%d write:%d billing_admin:%d destructive:%d",
			cfg.ReadRateLimitPerMinute,
			cfg.WriteRateLimitPerMinute,
			cfg.BillingAdminRateLimitPerMinute,
			cfg.DestructiveRateLimitPerMinute)
	}
}

func TestLoadOneUserAcceptsRuntimeConfigBoundsAndNormalization(t *testing.T) {
	tests := map[string]struct {
		envName string
		value   string
		assert  func(*testing.T, OneUserConfig)
	}{
		"minimum timeout": {
			envName: "CLOCKIFY_TOOL_TIMEOUT",
			value:   MinToolTimeout.String(),
			assert: func(t *testing.T, cfg OneUserConfig) {
				t.Helper()
				if cfg.ToolTimeout != MinToolTimeout {
					t.Fatalf("ToolTimeout = %s, want %s", cfg.ToolTimeout, MinToolTimeout)
				}
			},
		},
		"maximum timeout": {
			envName: "CLOCKIFY_TOOL_TIMEOUT",
			value:   MaxToolTimeout.String(),
			assert: func(t *testing.T, cfg OneUserConfig) {
				t.Helper()
				if cfg.ToolTimeout != MaxToolTimeout {
					t.Fatalf("ToolTimeout = %s, want %s", cfg.ToolTimeout, MaxToolTimeout)
				}
			},
		},
		"trimmed mixed case toolset": {
			envName: "CLOCKIFY_TOOLSET",
			value:   " Admin ",
			assert: func(t *testing.T, cfg OneUserConfig) {
				t.Helper()
				if cfg.Toolset != "admin" {
					t.Fatalf("Toolset = %q, want admin", cfg.Toolset)
				}
			},
		},
		"trimmed mixed case default toolset": {
			envName: "CLOCKIFY_TOOLSET",
			value:   " Default ",
			assert: func(t *testing.T, cfg OneUserConfig) {
				t.Helper()
				if cfg.Toolset != "default" {
					t.Fatalf("Toolset = %q, want default", cfg.Toolset)
				}
			},
		},
		"explicit raw writes false": {
			envName: "CLOCKIFY_ENABLE_RAW_WRITES",
			value:   "false",
			assert: func(t *testing.T, cfg OneUserConfig) {
				t.Helper()
				if cfg.EnableRawWrites {
					t.Fatal("EnableRawWrites = true")
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLOCKIFY_API_KEY", "test-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
			t.Setenv(tt.envName, tt.value)

			cfg, err := LoadOneUser()
			if err != nil {
				t.Fatal(err)
			}
			tt.assert(t, cfg)
		})
	}
}

func TestLoadOneUserAllowsLoopbackBaseURL(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "http://127.0.0.1:18080/api/v1")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://127.0.0.1:18080/api/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
}

func TestLoadOneUserRejectsUnsafeBaseURLAndWorkspaceID(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "workspace/with/slash")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected invalid workspace id error")
	}

	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "http://clockify.example.test/api/v1")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected non-HTTPS base URL error")
	}
}

func TestLoadOneUserAcceptsDocumentedClockifyHosts(t *testing.T) {
	hosts := []string{
		"https://api.clockify.me/api/v1",
		"https://reports.api.clockify.me/v1",
		"https://auditlog-api.api.clockify.me/v1",
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			t.Setenv("CLOCKIFY_API_KEY", "test-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
			t.Setenv("CLOCKIFY_BASE_URL", host)

			cfg, err := LoadOneUser()
			if err != nil {
				t.Fatalf("LoadOneUser(%q) returned error: %v", host, err)
			}
			if cfg.BaseURL != host {
				t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, host)
			}
		})
	}
}

func TestLoadOneUserRejectsNonClockifyHTTPSBaseURL(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "https://evil.example.com/api/v1")

	_, err := LoadOneUser()
	if err == nil {
		t.Fatal("expected error for arbitrary HTTPS host without override")
	}
	if !strings.Contains(err.Error(), "CLOCKIFY_ALLOW_CUSTOM_BASE_URL") {
		t.Fatalf("error %q should mention the custom-override env var", err)
	}
	if !strings.Contains(err.Error(), "api.clockify.me") {
		t.Fatalf("error %q should list the documented Clockify hosts", err)
	}
}

func TestLoadOneUserAllowsArbitraryHTTPSWithExplicitOverride(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "https://custom.internal/api/v1")
	t.Setenv("CLOCKIFY_ALLOW_CUSTOM_BASE_URL", "true")

	cfg, err := LoadOneUser()
	if err != nil {
		t.Fatalf("LoadOneUser returned error: %v", err)
	}
	if cfg.BaseURL != "https://custom.internal/api/v1" {
		t.Fatalf("BaseURL = %q, want custom override URL", cfg.BaseURL)
	}
}

func TestLoadOneUserRejectsArbitraryHTTPEvenWithOverride(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "http://evil.example.com/api/v1")
	t.Setenv("CLOCKIFY_ALLOW_CUSTOM_BASE_URL", "true")

	_, err := LoadOneUser()
	if err == nil {
		t.Fatal("expected error for non-HTTPS host even with custom override")
	}
	if !strings.Contains(err.Error(), "HTTPS is required") {
		t.Fatalf("error %q should require HTTPS for non-loopback hosts", err)
	}
}

func TestLoadOneUserAcceptsLoopbackHTTPSUnchanged(t *testing.T) {
	// localhost is loopback, so it is accepted regardless of scheme; this
	// guards that the loopback allowance also covers the HTTPS form.
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "https://localhost:8080/api/v1")

	if _, err := LoadOneUser(); err != nil {
		t.Fatalf("LoadOneUser returned error for loopback https: %v", err)
	}
}

func TestLoadOneUserBaseURLValidationErrorMessages(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_BASE_URL", "https://not-clockify.test/api/v1")

	_, err := LoadOneUser()
	if err == nil {
		t.Fatal("expected error for non-Clockify HTTPS host")
	}
	for _, want := range []string{
		"api.clockify.me",
		"reports.api.clockify.me",
		"auditlog-api.api.clockify.me",
		"CLOCKIFY_ALLOW_CUSTOM_BASE_URL",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should mention %q", err, want)
		}
	}
}

func TestLoadOneUserRejectsInvalidTimezone(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_TIMEZONE", "Mars/Base")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected invalid timezone error")
	}
}

func TestLoadOneUserRejectsInvalidRuntimeConfig(t *testing.T) {
	tests := map[string]struct {
		envName string
		value   string
	}{
		"zero concurrency": {
			envName: "CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS",
			value:   "0",
		},
		"negative concurrency": {
			envName: "CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS",
			value:   "-1",
		},
		"bad concurrency": {
			envName: "CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS",
			value:   "many",
		},
		"too short timeout": {
			envName: "CLOCKIFY_TOOL_TIMEOUT",
			value:   "4s",
		},
		"too long timeout": {
			envName: "CLOCKIFY_TOOL_TIMEOUT",
			value:   "11m",
		},
		"bad timeout": {
			envName: "CLOCKIFY_TOOL_TIMEOUT",
			value:   "soon",
		},
		"zero message size": {
			envName: "CLOCKIFY_MAX_MESSAGE_SIZE",
			value:   "0",
		},
		"negative message size": {
			envName: "CLOCKIFY_MAX_MESSAGE_SIZE",
			value:   "-1",
		},
		"bad message size": {
			envName: "CLOCKIFY_MAX_MESSAGE_SIZE",
			value:   "large",
		},
		"too large message size": {
			envName: "CLOCKIFY_MAX_MESSAGE_SIZE",
			value:   "104857601",
		},
		"zero tool result bytes": {
			envName: "CLOCKIFY_MAX_TOOL_RESULT_BYTES",
			value:   "0",
		},
		"negative tool result bytes": {
			envName: "CLOCKIFY_MAX_TOOL_RESULT_BYTES",
			value:   "-1",
		},
		"bad tool result bytes": {
			envName: "CLOCKIFY_MAX_TOOL_RESULT_BYTES",
			value:   "large",
		},
		"too large tool result bytes": {
			envName: "CLOCKIFY_MAX_TOOL_RESULT_BYTES",
			value:   "104857601",
		},
		"overflow tool result bytes": {
			envName: "CLOCKIFY_MAX_TOOL_RESULT_BYTES",
			value:   "9223372036854775807",
		},
		"bad raw writes": {
			envName: "CLOCKIFY_ENABLE_RAW_WRITES",
			value:   "maybe",
		},
		"bad raw tools": {
			envName: "CLOCKIFY_ENABLE_RAW_TOOLS",
			value:   "maybe",
		},
		"bad raw get": {
			envName: "CLOCKIFY_ENABLE_RAW_GET",
			value:   "maybe",
		},
		"bad toolset": {
			envName: "CLOCKIFY_TOOLSET",
			value:   "everything",
		},
		"bad audit log mode": {
			envName: "CLOCKIFY_AUDIT_LOG_MODE",
			value:   "sometimes",
		},
		"bad circuit breaker mode": {
			envName: "CLOCKIFY_CIRCUIT_BREAKER",
			value:   "sometimes",
		},
		"zero circuit breaker threshold": {
			envName: "CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD",
			value:   "0",
		},
		"bad circuit breaker duration": {
			envName: "CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION",
			value:   "later",
		},
		"zero circuit breaker half-open probes": {
			envName: "CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES",
			value:   "0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLOCKIFY_API_KEY", "test-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
			t.Setenv(tt.envName, tt.value)
			if _, err := LoadOneUser(); err == nil {
				t.Fatalf("expected %s=%q error", tt.envName, tt.value)
			}
		})
	}
}
