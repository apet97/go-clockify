package config

import (
	"reflect"
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
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
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
	if cfg.Toolset != DefaultToolset {
		t.Fatalf("Toolset = %q, want %q", cfg.Toolset, DefaultToolset)
	}
	if cfg.EnableRawWrites {
		t.Fatal("EnableRawWrites defaulted true")
	}
	if len(cfg.WebhookAllowedDomains) != 0 {
		t.Fatalf("WebhookAllowedDomains = %#v", cfg.WebhookAllowedDomains)
	}
}

func TestLoadOneUserOptionalRuntimeConfig(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
	t.Setenv("CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS", "8")
	t.Setenv("CLOCKIFY_TOOL_TIMEOUT", "2m")
	t.Setenv("CLOCKIFY_MAX_MESSAGE_SIZE", "8388608")
	t.Setenv("CLOCKIFY_TOOLSET", "business")
	t.Setenv("CLOCKIFY_ENABLE_RAW_WRITES", "true")
	t.Setenv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS", "hooks.example.com, .trusted.test, , api.example.com ")

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
	if cfg.Toolset != "business" {
		t.Fatalf("Toolset = %q", cfg.Toolset)
	}
	if !cfg.EnableRawWrites {
		t.Fatal("EnableRawWrites = false")
	}
	wantDomains := []string{"hooks.example.com", ".trusted.test", "api.example.com"}
	if !reflect.DeepEqual(cfg.WebhookAllowedDomains, wantDomains) {
		t.Fatalf("WebhookAllowedDomains = %#v, want %#v", cfg.WebhookAllowedDomains, wantDomains)
	}
}

func TestLoadOneUserAllowsLoopbackBaseURL(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
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

	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
	t.Setenv("CLOCKIFY_BASE_URL", "http://clockify.example.test/api/v1")
	if _, err := LoadOneUser(); err == nil {
		t.Fatal("expected non-HTTPS base URL error")
	}
}

func TestLoadOneUserRejectsInvalidTimezone(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
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
		"bad raw writes": {
			envName: "CLOCKIFY_ENABLE_RAW_WRITES",
			value:   "maybe",
		},
		"bad toolset": {
			envName: "CLOCKIFY_TOOLSET",
			value:   "everything",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLOCKIFY_API_KEY", "test-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
			t.Setenv(tt.envName, tt.value)
			if _, err := LoadOneUser(); err == nil {
				t.Fatalf("expected %s=%q error", tt.envName, tt.value)
			}
		})
	}
}
