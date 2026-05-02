package config

import (
	"strings"
	"testing"
)

// TestProdDefaults_AuditDurability locks the prod default flip:
// unset MCP_AUDIT_DURABILITY in prod resolves to fail_closed;
// unset in dev keeps best_effort; explicit values always win.
func TestProdDefaults_AuditDurability(t *testing.T) {
	cases := []struct {
		name      string
		env       map[string]string
		wantMode  string
		wantError string
	}{
		{
			name: "dev default is best_effort",
			env: map[string]string{
				"CLOCKIFY_API_KEY": "test-key",
			},
			wantMode: "best_effort",
		},
		{
			name: "prod default flips to fail_closed",
			env: map[string]string{
				"CLOCKIFY_API_KEY":      "test-key",
				"ENVIRONMENT":           "prod",
				"MCP_TRANSPORT":         "streamable_http",
				"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
				"MCP_AUTH_MODE":         "oidc",
				"MCP_OIDC_ISSUER":       "https://issuer.example",
			},
			wantMode: "fail_closed",
		},
		{
			name: "prod honours explicit best_effort",
			env: map[string]string{
				"CLOCKIFY_API_KEY":      "test-key",
				"ENVIRONMENT":           "prod",
				"MCP_AUDIT_DURABILITY":  "best_effort",
				"MCP_TRANSPORT":         "streamable_http",
				"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
				"MCP_AUTH_MODE":         "oidc",
				"MCP_OIDC_ISSUER":       "https://issuer.example",
			},
			wantMode: "best_effort",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnvs(t, tc.env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.AuditDurabilityMode != tc.wantMode {
				t.Fatalf("AuditDurabilityMode = %q, want %q",
					cfg.AuditDurabilityMode, tc.wantMode)
			}
		})
	}
}

// TestProdDefaults_HTTPLegacyPolicy locks the prod default flip:
// unset MCP_HTTP_LEGACY_POLICY in prod resolves to deny; unset in
// dev keeps warn; explicit allow is honoured in both.
func TestProdDefaults_HTTPLegacyPolicy(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		wantPolicy string
	}{
		{
			name: "dev default is warn",
			env: map[string]string{
				"CLOCKIFY_API_KEY": "test-key",
			},
			wantPolicy: "warn",
		},
		{
			name: "prod default flips to deny",
			env: map[string]string{
				"CLOCKIFY_API_KEY":      "test-key",
				"ENVIRONMENT":           "prod",
				"MCP_TRANSPORT":         "streamable_http",
				"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
				"MCP_AUTH_MODE":         "oidc",
				"MCP_OIDC_ISSUER":       "https://issuer.example",
			},
			wantPolicy: "deny",
		},
		{
			name: "prod honours explicit allow",
			env: map[string]string{
				"CLOCKIFY_API_KEY":       "test-key",
				"ENVIRONMENT":            "prod",
				"MCP_HTTP_LEGACY_POLICY": "allow",
				"MCP_TRANSPORT":          "streamable_http",
				"MCP_CONTROL_PLANE_DSN":  "postgres://db/mcp",
				"MCP_AUTH_MODE":          "oidc",
				"MCP_OIDC_ISSUER":        "https://issuer.example",
			},
			wantPolicy: "allow",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnvs(t, tc.env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.HTTPLegacyPolicy != tc.wantPolicy {
				t.Fatalf("HTTPLegacyPolicy = %q, want %q",
					cfg.HTTPLegacyPolicy, tc.wantPolicy)
			}
		})
	}
}

// TestProdDefaults_ErrorMessages spot-checks that misconfiguration
// errors mention the escape hatches so operators know how to fix.
func TestProdDefaults_ErrorMessages(t *testing.T) {
	// streamable_http + dev DSN without ack flag — error names both
	// remediations (flag or postgres://).
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "streamable_http",
		"MCP_AUTH_MODE":         "static_bearer",
		"MCP_BEARER_TOKEN":      "abcdef0123456789abcdef",
		"MCP_CONTROL_PLANE_DSN": "memory",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected fail-closed error, got nil")
	}
	for _, want := range []string{
		"MCP_ALLOW_DEV_BACKEND=1",
		"postgres://",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should mention %q", err.Error(), want)
		}
	}
}

// TestProdDefaults_RejectsDevBackendEscapeHatch locks the production
// guard that forbids carrying the single-process dev-backend acknowledgement
// into ENVIRONMENT=prod, even when the DSN itself is production-shaped.
func TestProdDefaults_RejectsDevBackendEscapeHatch(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"ENVIRONMENT":           "prod",
		"MCP_TRANSPORT":         "streamable_http",
		"MCP_AUTH_MODE":         "oidc",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected prod guard to reject MCP_ALLOW_DEV_BACKEND=1, got nil")
	}
	if !strings.Contains(err.Error(), "MCP_ALLOW_DEV_BACKEND=1 is prohibited") {
		t.Fatalf("error %q should name the prohibited dev-backend escape hatch", err.Error())
	}
}
