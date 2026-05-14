package config

import "testing"

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
