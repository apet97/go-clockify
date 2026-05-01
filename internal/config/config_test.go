package config

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestValidateBaseURL(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		insecure  bool
		wantError bool
	}{
		{"https ok", "https://api.clockify.me/api/v1", false, false},
		{"loopback http ok", "http://localhost:8080", false, false},
		{"insecure override ok", "http://example.com", true, false},
		{"http remote blocked", "http://example.com", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.url, tc.insecure)
			if tc.wantError && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBaseURL_HostedRejectsHTTP(t *testing.T) {
	// Hosted mode must refuse plain http even when the host is remote
	// — same posture as the env-level guardrail in Load().
	if err := ValidateBaseURL("http://example.com", ValidateBaseURLOptions{Hosted: true}); err == nil {
		t.Fatal("expected error for http baseURL under hosted profile")
	}
}

func TestValidateBaseURL_HostedRejectsLoopback(t *testing.T) {
	// Hosted mode must refuse loopback http even though loopback is
	// trusted in the self-hosted path. A tenant that smuggled
	// http://localhost via control-plane credentials would otherwise
	// be allowed to hairpin off-host through a sidecar proxy.
	if err := ValidateBaseURL("http://localhost:8080", ValidateBaseURLOptions{Hosted: true}); err == nil {
		t.Fatal("expected error for loopback http baseURL under hosted profile")
	}
}

func TestValidateBaseURL_HostedIgnoresAllowInsecure(t *testing.T) {
	// AllowInsecure is the operator-level CLOCKIFY_INSECURE escape;
	// hosted mode ignores it so an inherited env var cannot defeat
	// the production posture.
	if err := ValidateBaseURL("http://example.com", ValidateBaseURLOptions{Hosted: true, AllowInsecure: true}); err == nil {
		t.Fatal("expected error: hosted mode must not honor AllowInsecure")
	}
}

func TestValidateBaseURL_NonHostedAllowsLoopback(t *testing.T) {
	// Self-hosted path keeps the loopback bypass — Claude/Cursor/Codex
	// stdio installs and local docker-compose stacks rely on it.
	if err := ValidateBaseURL("http://127.0.0.1:8080", ValidateBaseURLOptions{}); err != nil {
		t.Fatalf("unexpected error for loopback baseURL in non-hosted mode: %v", err)
	}
}

func TestValidateBaseURL_NonHostedAllowsInsecure(t *testing.T) {
	// AllowInsecure remains the documented operator escape hatch
	// outside hosted profiles.
	if err := ValidateBaseURL("http://example.com", ValidateBaseURLOptions{AllowInsecure: true}); err != nil {
		t.Fatalf("unexpected error for insecure baseURL in non-hosted mode: %v", err)
	}
}

// setEnvs is a test helper that sets multiple env vars and returns a cleanup function.
func setEnvs(t *testing.T, envs map[string]string) {
	t.Helper()
	t.Setenv("MCP_METRICS_AUTH_MODE", "none")
	// applyProfile leaks process-global env via os.Setenv without going
	// through t.Setenv, so previous tests can leave MCP_OIDC_STRICT etc.
	// set when this one starts. Register a cleanup-restorable version
	// via t.Setenv("", "") only when the test isn't supplying one
	// itself, so subsequent subtests get a clean slate.
	for _, k := range profileLeakedEnvs {
		if _, ok := envs[k]; ok {
			continue
		}
		t.Setenv(k, "")
	}
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

// profileLeakedEnvs lists env vars that hosted profiles set via
// os.Setenv inside applyProfile. Without explicit reset, a previous
// hosted-profile test can leave them lingering and confuse later
// non-hosted tests.
var profileLeakedEnvs = []string{
	"MCP_OIDC_STRICT",
	"MCP_REQUIRE_TENANT_CLAIM",
	"MCP_DISABLE_INLINE_SECRETS",
	"MCP_HTTP_LEGACY_POLICY",
	"MCP_AUDIT_DURABILITY",
	"CLOCKIFY_POLICY",
	"MCP_TRANSPORT",
	"MCP_AUTH_MODE",
	"ENVIRONMENT",
	"MCP_CONTROL_PLANE_DSN",
	"MCP_ALLOW_DEV_BACKEND",
	"MCP_OIDC_ISSUER",
	"MCP_OIDC_AUDIENCE",
	"CLOCKIFY_INSECURE",
	"CLOCKIFY_SANITIZE_UPSTREAM_ERRORS",
	"CLOCKIFY_WEBHOOK_VALIDATE_DNS",
}

func TestLoadReportsURLRemoved(t *testing.T) {
	// CLOCKIFY_REPORTS_URL was removed — setting it is harmlessly ignored.
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":     "test-key",
		"CLOCKIFY_REPORTS_URL": "https://reports.clockify.me/v1/",
	})
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadTimezone(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"CLOCKIFY_TIMEZONE": "Europe/Belgrade",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timezone != "Europe/Belgrade" {
		t.Fatalf("expected Europe/Belgrade, got %q", cfg.Timezone)
	}
}

func TestLoadTransportDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	// Ensure MCP_TRANSPORT is unset.
	os.Unsetenv("MCP_TRANSPORT")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "stdio" {
		t.Fatalf("expected default transport stdio, got %q", cfg.Transport)
	}
}

func TestLoadTransportHTTP(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "http",
		"MCP_BEARER_TOKEN": "my-strong-token-1234567890",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "http" {
		t.Fatalf("expected http, got %q", cfg.Transport)
	}
}

func TestLoadHTTPBindDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("MCP_HTTP_BIND")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPBind != ":8080" {
		t.Fatalf("expected :8080 default, got %q", cfg.HTTPBind)
	}
}

func TestLoadHTTPBindCustom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_HTTP_BIND":    ":9090",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPBind != ":9090" {
		t.Fatalf("expected :9090, got %q", cfg.HTTPBind)
	}
}

func TestLoadBearerToken(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_BEARER_TOKEN": "secret-token",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BearerToken != "secret-token" {
		t.Fatalf("expected secret-token, got %q", cfg.BearerToken)
	}
}

func TestLoadAllowedOrigins(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":    "test-key",
		"MCP_ALLOWED_ORIGINS": " http://localhost:3000 , https://example.com , ",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 origins, got %d: %v", len(cfg.AllowedOrigins), cfg.AllowedOrigins)
	}
	if cfg.AllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("expected trimmed first origin, got %q", cfg.AllowedOrigins[0])
	}
	if cfg.AllowedOrigins[1] != "https://example.com" {
		t.Fatalf("expected trimmed second origin, got %q", cfg.AllowedOrigins[1])
	}
}

func TestLoadAllowedOriginsEmpty(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("MCP_ALLOWED_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowedOrigins != nil {
		t.Fatalf("expected nil origins, got %v", cfg.AllowedOrigins)
	}
}

func TestLoadStrictHostCheck(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_STRICT_HOST_CHECK": "true",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.StrictHostCheck {
		t.Fatal("expected StrictHostCheck to be true")
	}
}

func TestLoadStrictHostCheckInvalidReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_STRICT_HOST_CHECK": "maybe",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid MCP_STRICT_HOST_CHECK")
	}
}

// TestLoadStrictHostCheckEmptyAllowlistRejected fails Load() when an
// HTTP-flavoured transport bound to a non-loopback address has the
// strict host check enabled but no allowlist; the runtime would then
// reject every request with a 403. The error message must mention
// MCP_ALLOWED_ORIGINS so the operator knows what to set.
func TestLoadStrictHostCheckEmptyAllowlistRejected(t *testing.T) {
	base := func() map[string]string {
		return map[string]string{
			"MCP_TRANSPORT":         "streamable_http",
			"MCP_OIDC_ISSUER":       "https://example.com",
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
			"MCP_STRICT_HOST_CHECK": "1",
		}
	}
	cases := []struct {
		name string
		bind string
	}{
		{"any-interface ':8080'", ":8080"},
		{"explicit 0.0.0.0", "0.0.0.0:8080"},
		{"explicit ipv6 unspecified", "[::]:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envs := base()
			envs["MCP_HTTP_BIND"] = tc.bind
			setEnvs(t, envs)
			_, err := Load()
			if err == nil {
				t.Fatal("expected Load() to fail with strict host check + empty allowlist")
			}
			if !strings.Contains(err.Error(), "MCP_ALLOWED_ORIGINS") {
				t.Fatalf("error must reference MCP_ALLOWED_ORIGINS; got %v", err)
			}
		})
	}
}

// TestLoadStrictHostCheckAcceptsEscapeHatches confirms the preflight
// passes when any of the documented escape hatches is set: an explicit
// allowlist, MCP_ALLOW_ANY_ORIGIN=1, or a loopback bind. Each escape
// hatch must independently satisfy the gate.
func TestLoadStrictHostCheckAcceptsEscapeHatches(t *testing.T) {
	streamable := func(extra map[string]string) map[string]string {
		envs := map[string]string{
			"MCP_TRANSPORT":         "streamable_http",
			"MCP_OIDC_ISSUER":       "https://example.com",
			"MCP_CONTROL_PLANE_DSN": "memory",
			"MCP_ALLOW_DEV_BACKEND": "1",
			"MCP_STRICT_HOST_CHECK": "1",
			"MCP_HTTP_BIND":         ":8080",
		}
		maps.Copy(envs, extra)
		return envs
	}
	cases := []struct {
		name string
		envs map[string]string
	}{
		{"allowlist set", streamable(map[string]string{"MCP_ALLOWED_ORIGINS": "https://client.example.com"})},
		{"allow-any-origin", streamable(map[string]string{"MCP_ALLOW_ANY_ORIGIN": "1"})},
		{"loopback bind", streamable(map[string]string{"MCP_HTTP_BIND": "127.0.0.1:8080"})},
		{"ipv6 loopback bind", streamable(map[string]string{"MCP_HTTP_BIND": "[::1]:8080"})},
		{"stdio transport unaffected", map[string]string{
			"CLOCKIFY_API_KEY":      "test-key",
			"MCP_STRICT_HOST_CHECK": "1",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnvs(t, tc.envs)
			if _, err := Load(); err != nil {
				t.Fatalf("Load() should succeed for %s: %v", tc.name, err)
			}
		})
	}
}

func TestLoadMaxMessageSizeDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("MCP_MAX_MESSAGE_SIZE")
	os.Unsetenv("MCP_HTTP_MAX_BODY")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxMessageSize != 4194304 {
		t.Fatalf("expected 4194304 default, got %d", cfg.MaxMessageSize)
	}
}

func TestLoadMaxMessageSizeCustom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":     "test-key",
		"MCP_MAX_MESSAGE_SIZE": "8388608",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxMessageSize != 8388608 {
		t.Fatalf("expected 8388608, got %d", cfg.MaxMessageSize)
	}
}

func TestLoadMaxMessageSizeLegacyFallback(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "2097152",
	})
	os.Unsetenv("MCP_MAX_MESSAGE_SIZE")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxMessageSize != 2097152 {
		t.Fatalf("expected 2097152, got %d", cfg.MaxMessageSize)
	}
}

// TestLoadSingleTenantHTTPRequiresAPIKey verifies that the
// single-tenant-http profile fails Load() when CLOCKIFY_API_KEY
// is empty: the profile bootstraps the only tenant from the env
// key, so without it /ready is never wired and the first session
// fails with "tenant not found in control plane". Other streamable
// HTTP deployments (shared-service, prod-postgres) still allow an
// empty key because hosted credentials come from the control plane.
func TestLoadSingleTenantHTTPRequiresAPIKey(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_PROFILE":      "single-tenant-http",
		"MCP_BEARER_TOKEN": "single-tenant-bearer-token-1234",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to fail when single-tenant-http has no API key")
	}
	if !strings.Contains(err.Error(), "single-tenant-http") {
		t.Fatalf("error must reference the profile name; got %v", err)
	}
	if !strings.Contains(err.Error(), "CLOCKIFY_API_KEY") {
		t.Fatalf("error must reference CLOCKIFY_API_KEY; got %v", err)
	}
}

// TestLoadSingleTenantHTTPAcceptsAPIKey confirms the same profile
// loads cleanly when the API key is set — this is the happy path
// the gate is protecting.
func TestLoadSingleTenantHTTPAcceptsAPIKey(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_PROFILE":      "single-tenant-http",
		"MCP_BEARER_TOKEN": "single-tenant-bearer-token-1234",
		"CLOCKIFY_API_KEY": "test-key",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with API key set: %v", err)
	}
	if cfg.APIKey != "test-key" {
		t.Fatalf("expected APIKey to be propagated; got %q", cfg.APIKey)
	}
}

// TestLoadSharedServiceAllowsEmptyAPIKey ensures the new gate is
// scoped to single-tenant-http only — shared-service deployments
// still resolve credentials per-tenant from the control plane.
func TestLoadSharedServiceAllowsEmptyAPIKey(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_PROFILE":           "shared-service",
		"MCP_OIDC_ISSUER":       "https://example.com",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp",
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	if _, err := Load(); err != nil {
		t.Fatalf("shared-service should load without API key: %v", err)
	}
}

func TestLoadStreamableHTTPAllowsEmptyAPIKey(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_TRANSPORT":         "streamable_http",
		"MCP_METRICS_BIND":      "",
		"MCP_OIDC_ISSUER":       "https://example.com",
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthMode != "oidc" {
		t.Fatalf("expected default auth mode oidc, got %q", cfg.AuthMode)
	}
}

func TestLoadInvalidBaseURLReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"CLOCKIFY_BASE_URL": "://invalid", // This triggers url.Parse error
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestLoadMetricsValidation(t *testing.T) {
	t.Run("valid_static_bearer", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":         "test-key",
			"MCP_METRICS_BIND":         ":9091",
			"MCP_METRICS_BEARER_TOKEN": "1234567890123456",
		})
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.MetricsBind != ":9091" {
			t.Fatalf("expected :9091, got %q", cfg.MetricsBind)
		}
	})

	t.Run("invalid_auth_mode", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":      "test-key",
			"MCP_METRICS_BIND":      ":9091",
			"MCP_METRICS_AUTH_MODE": "invalid",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for invalid auth mode")
		}
	})

	t.Run("missing_bearer_token", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":      "test-key",
			"MCP_METRICS_BIND":      ":9091",
			"MCP_METRICS_AUTH_MODE": "static_bearer",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for missing bearer token")
		}
	})

	t.Run("short_bearer_token", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":         "test-key",
			"MCP_METRICS_BIND":         ":9091",
			"MCP_METRICS_AUTH_MODE":    "static_bearer",
			"MCP_METRICS_BEARER_TOKEN": "short",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for short bearer token")
		}
	})
}

func TestLoadMaxMessageSizeLegacyFallbackInvalidReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "not-a-number",
	})
	os.Unsetenv("MCP_MAX_MESSAGE_SIZE")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid MCP_HTTP_MAX_BODY")
	}
}

func TestLoadMaxMessageSizeLegacyFallbackZeroReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "0",
	})
	os.Unsetenv("MCP_MAX_MESSAGE_SIZE")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero MCP_HTTP_MAX_BODY")
	}
}

func TestLoadMaxMessageSizeInvalidReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":     "test-key",
		"MCP_MAX_MESSAGE_SIZE": "not-a-number",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid MCP_MAX_MESSAGE_SIZE")
	}
}

func TestLoadMaxMessageSizeZeroReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":     "test-key",
		"MCP_MAX_MESSAGE_SIZE": "0",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero MCP_MAX_MESSAGE_SIZE")
	}
}

func TestLoadMaxMessageSizeTooLargeReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":     "test-key",
		"MCP_MAX_MESSAGE_SIZE": "99999999999",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for MCP_MAX_MESSAGE_SIZE exceeding 100MB")
	}
}

// --- Transport validation tests ---

func TestLoadTransportInvalid(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "carrier-pigeon",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid transport value")
	}
}

// TestLoadTransportGRPC verifies that the gRPC transport is accepted and
// that MCP_GRPC_BIND is wired into Config. The transport binary itself is
// only linked under -tags=grpc (see ADR 012); Config.Load just validates
// the selection and records the bind address.
//
// MCP_ALLOW_DEV_BACKEND=1 is set here (and on every gRPC happy-path test
// below) because the dev-backend guard at config.go:493 covers gRPC as
// well as streamable_http — fail_closed audit on a memory backend cannot
// honour pod restarts on either transport. See ADR-0014.
func TestLoadTransportGRPC(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_GRPC_BIND":         "127.0.0.1:7777",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("grpc transport should be accepted: %v", err)
	}
	if cfg.Transport != "grpc" {
		t.Fatalf("expected grpc, got %q", cfg.Transport)
	}
	if cfg.GRPCBind != "127.0.0.1:7777" {
		t.Fatalf("expected 127.0.0.1:7777 bind, got %q", cfg.GRPCBind)
	}
}

// TestLoadTransportGRPCDefaultBind confirms :9090 is the default when
// MCP_GRPC_BIND is not set.
func TestLoadTransportGRPCDefaultBind(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	os.Unsetenv("MCP_GRPC_BIND")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("grpc transport should be accepted: %v", err)
	}
	if cfg.GRPCBind != ":9090" {
		t.Fatalf("expected :9090 default, got %q", cfg.GRPCBind)
	}
}

// TestLoadTransportGRPCStaticBearer verifies the W4-03 auth amendment:
// gRPC + static_bearer is accepted when MCP_BEARER_TOKEN is set, matching
// the legacy HTTP transport's shape so operators get a consistent knob.
func TestLoadTransportGRPCStaticBearer(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_AUTH_MODE":         "static_bearer",
		"MCP_BEARER_TOKEN":      "1234567890abcdef",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("grpc + static_bearer should be accepted: %v", err)
	}
	if cfg.AuthMode != "static_bearer" {
		t.Fatalf("expected static_bearer, got %q", cfg.AuthMode)
	}
}

// TestLoadTransportGRPCOIDC verifies that oidc auth is accepted under gRPC.
// OIDC only touches the Authorization header (plus JWKS fetch via the
// request context), so the synthetic *http.Request bridge in the gRPC
// interceptor is compatible with it out of the box.
func TestLoadTransportGRPCOIDC(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_AUTH_MODE":         "oidc",
		"MCP_OIDC_ISSUER":       "https://idp.example.com",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("grpc + oidc should be accepted: %v", err)
	}
	if cfg.AuthMode != "oidc" {
		t.Fatalf("expected oidc, got %q", cfg.AuthMode)
	}
}

// TestLoadTransportGRPCForwardAuthAccepted verifies forward_auth is
// accepted on gRPC (W5-05b: metadata passthrough).
func TestLoadTransportGRPCForwardAuthAccepted(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_AUTH_MODE":         "forward_auth",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	_, err := Load()
	if err != nil {
		t.Fatalf("expected gRPC + forward_auth to be accepted, got: %v", err)
	}
}

// TestLoadTransportGRPCMTLSAccepted verifies mtls is accepted on gRPC
// when the operator supplies the full TLS cert material
// (W5-05c: credentials.TLSInfo passthrough; security audit C2026-04-25
// H4 promoted the cert/key/CA from optional-runtime-load to
// required-at-config-load).
func TestLoadTransportGRPCMTLSAccepted(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_TRANSPORT":         "grpc",
		"MCP_AUTH_MODE":         "mtls",
		"MCP_GRPC_TLS_CERT":     "/dev/null",
		"MCP_GRPC_TLS_KEY":      "/dev/null",
		"MCP_MTLS_CA_CERT_PATH": "/dev/null",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	_, err := Load()
	if err != nil {
		t.Fatalf("expected gRPC + mtls (with cert/key/CA) to be accepted, got: %v", err)
	}
}

// TestOIDCStrictRequiresAudienceOrResourceURI locks in the C1 finding
// from the 2026-04-25 audit: when MCP_OIDC_STRICT=1 is set on an oidc
// deployment, config.Load must reject configurations that bind tokens
// only by issuer (no audience, no resource URI). Hosted-service
// deployments need to bind tokens to *this* server, not just any server
// trusted by the issuer.
func TestOIDCStrictRequiresAudienceOrResourceURI(t *testing.T) {
	t.Run("strict_no_audience_no_resource_rejected", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY": "test-key",
			"MCP_TRANSPORT":    "http",
			"MCP_AUTH_MODE":    "oidc",
			"MCP_OIDC_ISSUER":  "https://issuer.example",
			"MCP_OIDC_STRICT":  "1",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when MCP_OIDC_STRICT=1 has no audience/resource")
		}
		if !strings.Contains(err.Error(), "MCP_OIDC_AUDIENCE or MCP_RESOURCE_URI") {
			t.Errorf("error should name both vars, got: %v", err)
		}
	})
	t.Run("strict_with_audience_accepted", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":  "test-key",
			"MCP_TRANSPORT":     "http",
			"MCP_AUTH_MODE":     "oidc",
			"MCP_OIDC_ISSUER":   "https://issuer.example",
			"MCP_OIDC_AUDIENCE": "clockify-mcp",
			"MCP_OIDC_STRICT":   "1",
		})
		if _, err := Load(); err != nil {
			t.Fatalf("strict + audience should pass: %v", err)
		}
	})
	t.Run("strict_with_resource_uri_accepted", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY": "test-key",
			"MCP_TRANSPORT":    "http",
			"MCP_AUTH_MODE":    "oidc",
			"MCP_OIDC_ISSUER":  "https://issuer.example",
			"MCP_RESOURCE_URI": "https://mcp.example/mcp",
			"MCP_OIDC_STRICT":  "1",
		})
		if _, err := Load(); err != nil {
			t.Fatalf("strict + resource URI should pass: %v", err)
		}
	})
	t.Run("non_strict_default_unchanged", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY": "test-key",
			"MCP_TRANSPORT":    "http",
			"MCP_AUTH_MODE":    "oidc",
			"MCP_OIDC_ISSUER":  "https://issuer.example",
			// no audience, no resource, no MCP_OIDC_STRICT
		})
		if _, err := Load(); err != nil {
			t.Fatalf("non-strict back-compat path should pass: %v", err)
		}
	})
}

func TestLoadDeltaFormat(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_DELTA_FORMAT": "jsonpatch",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("jsonpatch format should be accepted: %v", err)
	}
	if cfg.DeltaFormat != "jsonpatch" {
		t.Fatalf("expected jsonpatch, got %q", cfg.DeltaFormat)
	}
}

func TestLoadDeltaFormatInvalid(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_DELTA_FORMAT": "rfc999",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid delta format to be rejected")
	}
}

func TestLoadGRPCReauthInterval(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":         "test-key",
		"MCP_TRANSPORT":            "grpc",
		"MCP_GRPC_REAUTH_INTERVAL": "60s",
		"MCP_ALLOW_DEV_BACKEND":    "1",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("60s reauth interval should be accepted: %v", err)
	}
	if cfg.GRPCReauthInterval.Seconds() != 60 {
		t.Fatalf("expected 60s, got %v", cfg.GRPCReauthInterval)
	}
}

func TestLoadGRPCTLSMismatch(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_GRPC_TLS_CERT": "/path/to/cert.pem",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected TLS cert without key to be rejected")
	}
}

func TestLoadTransportStdioExplicit(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "stdio",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("explicit stdio should pass: %v", err)
	}
	if cfg.Transport != "stdio" {
		t.Fatalf("expected stdio, got %q", cfg.Transport)
	}
}

// --- Timezone validation tests ---

func TestLoadTimezoneInvalid(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"CLOCKIFY_TIMEZONE": "US/Eastrn",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestLoadTimezoneEmpty(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("CLOCKIFY_TIMEZONE")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("empty timezone should pass: %v", err)
	}
	if cfg.Timezone != "" {
		t.Fatalf("expected empty timezone, got %q", cfg.Timezone)
	}
}

// --- HTTP bearer token fail-fast tests ---

func TestLoadHTTPTransportRequiresBearerToken(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "http",
	})
	os.Unsetenv("MCP_BEARER_TOKEN")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for HTTP transport without bearer token")
	}
}

func TestLoadHTTPTransportWithBearerToken(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "http",
		"MCP_BEARER_TOKEN": "my-strong-secret-token",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("HTTP with bearer token should pass: %v", err)
	}
	if cfg.BearerToken != "my-strong-secret-token" {
		t.Fatalf("expected my-strong-secret-token, got %q", cfg.BearerToken)
	}
}

func TestLoadHTTPBearerTokenMinLength(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "http",
		"MCP_BEARER_TOKEN": "short",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for bearer token shorter than 16 characters")
	}
}

func TestLoadStdioBearerTokenNoLengthCheck(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_BEARER_TOKEN": "short",
	})
	os.Unsetenv("MCP_TRANSPORT")
	_, err := Load()
	if err != nil {
		t.Fatalf("stdio mode should not enforce bearer token length: %v", err)
	}
}

func TestLoadToolTimeoutDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("CLOCKIFY_TOOL_TIMEOUT")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ToolTimeout != 45*1000000000 { // 45s in nanoseconds
		t.Fatalf("expected 45s default, got %v", cfg.ToolTimeout)
	}
}

func TestLoadToolTimeoutCustom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_TOOL_TIMEOUT": "2m",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ToolTimeout != 120*1000000000 { // 2m
		t.Fatalf("expected 2m, got %v", cfg.ToolTimeout)
	}
}

func TestLoadToolTimeoutTooShort(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_TOOL_TIMEOUT": "1s",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for timeout < 5s")
	}
}

func TestLoadToolTimeoutTooLong(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_TOOL_TIMEOUT": "15m",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for timeout > 10m")
	}
}

func TestLoadToolTimeoutInvalid(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_TOOL_TIMEOUT": "not-a-duration",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLoadConcurrencyAcquireTimeoutDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConcurrencyAcquireTimeout != 100*1000*1000 { // 100ms
		t.Fatalf("expected 100ms default, got %v", cfg.ConcurrencyAcquireTimeout)
	}
}

func TestLoadConcurrencyAcquireTimeoutCustom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":                     "test-key",
		"CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT": "250ms",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConcurrencyAcquireTimeout != 250*1000*1000 {
		t.Fatalf("expected 250ms, got %v", cfg.ConcurrencyAcquireTimeout)
	}
}

func TestLoadConcurrencyAcquireTimeoutInvalid(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":                     "test-key",
		"CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT": "not-a-duration",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid concurrency acquire timeout")
	}
}

func TestLoadConcurrencyAcquireTimeoutOutOfRange(t *testing.T) {
	tests := []string{"0", "31s"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			setEnvs(t, map[string]string{
				"CLOCKIFY_API_KEY":                     "test-key",
				"CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT": value,
			})

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for out-of-range timeout %q", value)
			}
		})
	}
}

func TestLoadStreamableHTTPWithoutStaticAPIKey(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_TRANSPORT":         "streamable_http",
		"MCP_AUTH_MODE":         "oidc",
		"MCP_OIDC_ISSUER":       "https://issuer.example.com",
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	os.Unsetenv("CLOCKIFY_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected streamable_http config to load without CLOCKIFY_API_KEY, got %v", err)
	}
	if cfg.Transport != "streamable_http" {
		t.Fatalf("expected streamable_http, got %q", cfg.Transport)
	}
	if cfg.AuthMode != "oidc" {
		t.Fatalf("expected oidc auth, got %q", cfg.AuthMode)
	}
}

func TestLoadStreamableHTTPRequiresOIDCIssuer(t *testing.T) {
	setEnvs(t, map[string]string{
		"MCP_TRANSPORT":         "streamable_http",
		"MCP_AUTH_MODE":         "oidc",
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	os.Unsetenv("CLOCKIFY_API_KEY")
	os.Unsetenv("MCP_OIDC_ISSUER")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing OIDC issuer to fail")
	}
}

func baseHTTPEnvs() map[string]string {
	return map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "http",
		"MCP_BEARER_TOKEN": "my-strong-token-1234567890",
	}
}

func TestAuditDurabilityDefault(t *testing.T) {
	setEnvs(t, map[string]string{"CLOCKIFY_API_KEY": "test-key"})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuditDurabilityMode != "best_effort" {
		t.Fatalf("expected default best_effort, got %q", cfg.AuditDurabilityMode)
	}
}

func TestAuditDurabilityFailClosed(t *testing.T) {
	setEnvs(t, map[string]string{"CLOCKIFY_API_KEY": "test-key", "MCP_AUDIT_DURABILITY": "fail_closed"})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuditDurabilityMode != "fail_closed" {
		t.Fatalf("expected fail_closed, got %q", cfg.AuditDurabilityMode)
	}
}

func TestAuditDurabilityInvalid(t *testing.T) {
	setEnvs(t, map[string]string{"CLOCKIFY_API_KEY": "test-key", "MCP_AUDIT_DURABILITY": "bogus"})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid MCP_AUDIT_DURABILITY")
	}
}

func TestHTTPLegacyPolicyDefault(t *testing.T) {
	setEnvs(t, baseHTTPEnvs())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPLegacyPolicy != "warn" {
		t.Fatalf("expected default warn, got %q", cfg.HTTPLegacyPolicy)
	}
}

func TestHTTPLegacyPolicyDeny(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_LEGACY_POLICY"] = "deny"
	setEnvs(t, m)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPLegacyPolicy != "deny" {
		t.Fatalf("expected deny, got %q", cfg.HTTPLegacyPolicy)
	}
}

func TestHTTPLegacyPolicyInvalid(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_LEGACY_POLICY"] = "maybe"
	setEnvs(t, m)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid MCP_HTTP_LEGACY_POLICY")
	}
}

func TestHTTPInlineMetrics_DefaultDisabled(t *testing.T) {
	setEnvs(t, baseHTTPEnvs())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPInlineMetricsEnabled {
		t.Fatal("expected inline metrics disabled by default")
	}
}

func TestHTTPInlineMetrics_EnabledDefaultsToInheritBearer(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
	setEnvs(t, m)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HTTPInlineMetricsEnabled {
		t.Fatal("expected inline metrics enabled")
	}
	if cfg.HTTPInlineMetricsAuthMode != "inherit_main_bearer" {
		t.Fatalf("expected default auth inherit_main_bearer, got %q", cfg.HTTPInlineMetricsAuthMode)
	}
}

func TestHTTPInlineMetrics_StaticBearerRequiresToken(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
	m["MCP_HTTP_INLINE_METRICS_AUTH_MODE"] = "static_bearer"
	setEnvs(t, m)
	if _, err := Load(); err == nil {
		t.Fatal("expected error: static_bearer without token")
	}
}

func TestHTTPInlineMetrics_StaticBearerShortToken(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
	m["MCP_HTTP_INLINE_METRICS_AUTH_MODE"] = "static_bearer"
	m["MCP_HTTP_INLINE_METRICS_BEARER_TOKEN"] = "short"
	setEnvs(t, m)
	if _, err := Load(); err == nil {
		t.Fatal("expected error: static_bearer with short token")
	}
}

func TestHTTPInlineMetrics_StaticBearerValidToken(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
	m["MCP_HTTP_INLINE_METRICS_AUTH_MODE"] = "static_bearer"
	m["MCP_HTTP_INLINE_METRICS_BEARER_TOKEN"] = "metrics-bearer-token-long-enough"
	setEnvs(t, m)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPInlineMetricsBearerToken != "metrics-bearer-token-long-enough" {
		t.Fatalf("expected token, got %q", cfg.HTTPInlineMetricsBearerToken)
	}
}

func TestHTTPInlineMetrics_InvalidAuthMode(t *testing.T) {
	m := baseHTTPEnvs()
	m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
	m["MCP_HTTP_INLINE_METRICS_AUTH_MODE"] = "bogus"
	setEnvs(t, m)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid auth mode")
	}
}

// TestHTTPInlineMetrics_InheritBearerRequiresStaticBearer locks the
// gate that catches a silent /metrics-401 trap: when MCP_TRANSPORT=http
// + MCP_HTTP_INLINE_METRICS_ENABLED=1 + the default auth mode
// inherit_main_bearer, the inline handler at transport_http.go reuses
// MainBearerToken — populated only when MCP_AUTH_MODE=static_bearer.
// With OIDC or forward_auth, MainBearerToken is empty and the
// constant-time compare rejects every scrape. Failing at config load
// is friendlier than a silently dead /metrics endpoint.
func TestHTTPInlineMetrics_InheritBearerRequiresStaticBearer(t *testing.T) {
	t.Run("oidc_rejected", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":                "test-key",
			"MCP_TRANSPORT":                   "http",
			"MCP_AUTH_MODE":                   "oidc",
			"MCP_OIDC_ISSUER":                 "https://issuer.example",
			"MCP_HTTP_INLINE_METRICS_ENABLED": "1",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for http + oidc + inline metrics + inherit_main_bearer")
		}
		if !strings.Contains(err.Error(), "inherit_main_bearer requires MCP_AUTH_MODE=static_bearer") {
			t.Errorf("error should explain the static_bearer requirement; got: %v", err)
		}
	})
	t.Run("forward_auth_rejected", func(t *testing.T) {
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":                "test-key",
			"MCP_TRANSPORT":                   "http",
			"MCP_AUTH_MODE":                   "forward_auth",
			"MCP_HTTP_INLINE_METRICS_ENABLED": "1",
		})
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for http + forward_auth + inline metrics + inherit_main_bearer")
		}
		if !strings.Contains(err.Error(), "inherit_main_bearer requires MCP_AUTH_MODE=static_bearer") {
			t.Errorf("error should explain the static_bearer requirement; got: %v", err)
		}
	})
	t.Run("static_bearer_ok", func(t *testing.T) {
		// Same shape as TestHTTPInlineMetrics_EnabledDefaultsToInheritBearer
		// but pinned alongside the negative cases so the contract is
		// readable end-to-end.
		m := baseHTTPEnvs()
		m["MCP_HTTP_INLINE_METRICS_ENABLED"] = "1"
		setEnvs(t, m)
		if _, err := Load(); err != nil {
			t.Fatalf("http + static_bearer + inline + inherit_main_bearer should pass: %v", err)
		}
	})
	t.Run("streamable_http_oidc_unaffected", func(t *testing.T) {
		// Setting inline metrics options outside legacy http is a
		// documented no-op (config.go comment block above the gate).
		// The gate scopes itself to Transport=="http" so operators
		// who share config across environments can keep the same
		// MCP_HTTP_INLINE_METRICS_ENABLED=1 in a streamable_http
		// deployment without spurious errors.
		setEnvs(t, map[string]string{
			"CLOCKIFY_API_KEY":                "test-key",
			"MCP_TRANSPORT":                   "streamable_http",
			"MCP_AUTH_MODE":                   "oidc",
			"MCP_OIDC_ISSUER":                 "https://issuer.example",
			"MCP_CONTROL_PLANE_DSN":           "memory",
			"MCP_ALLOW_DEV_BACKEND":           "1",
			"MCP_HTTP_INLINE_METRICS_ENABLED": "1",
		})
		if _, err := Load(); err != nil {
			t.Fatalf("streamable_http + oidc + inline metrics should pass (no-op path): %v", err)
		}
	})
}

// --- OIDC verify-cache TTL --------------------------------------------------

func TestOIDCVerifyCacheTTL_Default(t *testing.T) {
	setEnvs(t, map[string]string{"CLOCKIFY_API_KEY": "test-key"})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OIDCVerifyCacheTTL != 0 {
		t.Fatalf("unset TTL should leave config at 0 (authn picks default), got %s", cfg.OIDCVerifyCacheTTL)
	}
}

func TestOIDCVerifyCacheTTL_Custom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":          "test-key",
		"MCP_OIDC_VERIFY_CACHE_TTL": "90s",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OIDCVerifyCacheTTL != 90*time.Second {
		t.Fatalf("expected 90s, got %s", cfg.OIDCVerifyCacheTTL)
	}
}

func TestOIDCVerifyCacheTTL_BelowMinRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":          "test-key",
		"MCP_OIDC_VERIFY_CACHE_TTL": "500ms",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TTL below 1s")
	}
}

func TestOIDCVerifyCacheTTL_AboveMaxRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":          "test-key",
		"MCP_OIDC_VERIFY_CACHE_TTL": "10m",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TTL above 5m")
	}
}

func TestOIDCVerifyCacheTTL_InvalidDurationRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":          "test-key",
		"MCP_OIDC_VERIFY_CACHE_TTL": "carrots",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unparseable duration")
	}
}

// --- OIDC JWKS cache TTL ----------------------------------------------------
//
// Mirrors the verify-cache TTL coverage above. Pre-F5 the JWKS cache
// hardcoded a 5-minute TTL with no operator knob; an IdP that rotates
// keys hourly forced a process restart for the rotation to land
// promptly. MCP_OIDC_JWKS_CACHE_TTL accepts [1m, 24h]; values outside
// the bracket are rejected at config load. authn picks 5m if Config
// leaves it zero.

func TestOIDCJWKSCacheTTL_Default(t *testing.T) {
	setEnvs(t, map[string]string{"CLOCKIFY_API_KEY": "test-key"})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OIDCJWKSCacheTTL != 0 {
		t.Fatalf("unset TTL should leave config at 0 (authn picks default), got %s", cfg.OIDCJWKSCacheTTL)
	}
}

func TestOIDCJWKSCacheTTL_Custom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"MCP_OIDC_JWKS_CACHE_TTL": "30m",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OIDCJWKSCacheTTL != 30*time.Minute {
		t.Fatalf("expected 30m, got %s", cfg.OIDCJWKSCacheTTL)
	}
}

func TestOIDCJWKSCacheTTL_BelowMinRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"MCP_OIDC_JWKS_CACHE_TTL": "30s",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TTL below 1m")
	}
}

func TestOIDCJWKSCacheTTL_AboveMaxRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"MCP_OIDC_JWKS_CACHE_TTL": "48h",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TTL above 24h")
	}
}

func TestOIDCJWKSCacheTTL_InvalidDurationRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"MCP_OIDC_JWKS_CACHE_TTL": "carrots",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unparseable duration")
	}
}

// TestLoad_RejectsBadWorkspaceID verifies CLOCKIFY_WORKSPACE_ID is run
// through resolve.ValidateID at startup so a malformed value (path-injection
// shaped, fragment, query, or .. traversal) cannot reach handler-level
// path concatenation. Pre-fix, these would silently propagate to every
// /workspaces/{id}/... request.
func TestLoad_RejectsBadWorkspaceID(t *testing.T) {
	bad := []string{
		"bad/path",
		"bad?query",
		"bad#fragment",
		"bad%2Fpath",
		"foo..bar",
		"ws\x01id", // control byte rejected by ValidateID's rune loop
	}
	for _, val := range bad {
		t.Run(val, func(t *testing.T) {
			setEnvs(t, map[string]string{
				"CLOCKIFY_API_KEY":      "test-key",
				"CLOCKIFY_WORKSPACE_ID": val,
			})
			_, err := Load()
			if err == nil {
				t.Fatalf("expected validation error for %q, got nil", val)
			}
			if !strings.Contains(err.Error(), "CLOCKIFY_WORKSPACE_ID") {
				t.Fatalf("error %q should reference CLOCKIFY_WORKSPACE_ID", err)
			}
		})
	}
}

// TestLoad_AcceptsValidWorkspaceID locks in the negative direction:
// well-formed IDs (real Clockify IDs are 24-hex BSON ObjectIDs, but the
// validator accepts any safe string of bounded length) keep working.
func TestLoad_AcceptsValidWorkspaceID(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"CLOCKIFY_WORKSPACE_ID": "5e0fa5cb6c5dc403da9f1234",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkspaceID != "5e0fa5cb6c5dc403da9f1234" {
		t.Fatalf("workspace mismatch: %q", cfg.WorkspaceID)
	}
}

// hostedProfileEnv supplies the minimum env to make Load() succeed for
// shared-service / prod-postgres profiles. They set MCP_OIDC_STRICT=1
// which requires either MCP_OIDC_AUDIENCE or MCP_RESOURCE_URI to bind
// tokens to this server, plus an OIDC issuer for streamable_http with
// AuthMode=oidc. Tests that aren't asserting OIDC behaviour just need
// these placeholders to clear the gates.
var hostedProfileEnv = map[string]string{
	"MCP_OIDC_AUDIENCE":     "test-audience",
	"MCP_OIDC_ISSUER":       "https://example.com",
	"MCP_CONTROL_PLANE_DSN": "memory",
	"MCP_ALLOW_DEV_BACKEND": "1",
}

// TestLoad_SanitizeUpstreamErrors_HostedProfileDefault locks in the
// profile-driven default added in audit finding 9: shared-service
// silently flips SanitizeUpstreamErrors=true so a 4xx from Clockify
// cannot leak per-tenant info across tenants. local-stdio keeps
// verbose errors for fast operator debugging. The other two
// profiles (prod-postgres, single-tenant-http) have additional
// production gates that need real postgres / a bearer token to clear,
// so they're covered indirectly by the isHostedProfile() unit logic.
func TestLoad_SanitizeUpstreamErrors_HostedProfileDefault(t *testing.T) {
	cases := []struct {
		profile string
		want    bool
	}{
		{"shared-service", true},
		{"local-stdio", false},
	}
	for _, c := range cases {
		t.Run(c.profile, func(t *testing.T) {
			env := map[string]string{
				"CLOCKIFY_API_KEY": "test-key",
				"MCP_PROFILE":      c.profile,
			}
			if isHostedProfile(c.profile) {
				maps.Copy(env, hostedProfileEnv)
			}
			setEnvs(t, env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.SanitizeUpstreamErrors != c.want {
				t.Fatalf("profile %q SanitizeUpstreamErrors=%v want=%v", c.profile, cfg.SanitizeUpstreamErrors, c.want)
			}
		})
	}
}

// TestIsHostedProfile_Predicate locks in the predicate independently
// of Load() so the profile classification is testable without OIDC
// strict gates getting in the way.
func TestIsHostedProfile_Predicate(t *testing.T) {
	cases := map[string]bool{
		"shared-service":       true,
		"prod-postgres":        true,
		"local-stdio":          false,
		"single-tenant-http":   false,
		"private-network-grpc": false,
		"unknown":              false,
		"":                     false,
	}
	for name, want := range cases {
		if got := isHostedProfile(name); got != want {
			t.Errorf("isHostedProfile(%q)=%v want=%v", name, got, want)
		}
	}
}

// TestLoad_WebhookValidateDNS_HostedProfileDefault locks in the
// profile-driven default added in audit finding 10: shared-service
// flips WebhookValidateDNS=true so a hostname that resolves to a
// private/reserved IP is rejected. local-stdio keeps the legacy
// literal-only check.
func TestLoad_WebhookValidateDNS_HostedProfileDefault(t *testing.T) {
	cases := []struct {
		profile string
		want    bool
	}{
		{"shared-service", true},
		{"local-stdio", false},
	}
	for _, c := range cases {
		t.Run(c.profile, func(t *testing.T) {
			env := map[string]string{
				"CLOCKIFY_API_KEY": "test-key",
				"MCP_PROFILE":      c.profile,
			}
			if isHostedProfile(c.profile) {
				maps.Copy(env, hostedProfileEnv)
			}
			setEnvs(t, env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.WebhookValidateDNS != c.want {
				t.Fatalf("profile %q WebhookValidateDNS=%v want=%v", c.profile, cfg.WebhookValidateDNS, c.want)
			}
		})
	}
}

// TestLoad_WebhookAllowedDomains_Parser exercises the comma-separated
// parser added for CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS: whitespace
// around each entry is trimmed and empty entries are dropped at
// config-load time. The validator helper (`isWebhookHostAllowed`)
// also trims/skips as defence-in-depth — this test pins the
// config-load surface so we don't double-pay or under-trim.
func TestLoad_WebhookAllowedDomains_Parser(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"unset_yields_nil", "", nil},
		{"single_entry", "webhook.example.com", []string{"webhook.example.com"}},
		{"two_entries", "webhook.example.com,api.example.com", []string{"webhook.example.com", "api.example.com"}},
		{"trims_whitespace", " webhook.example.com , api.example.com ", []string{"webhook.example.com", "api.example.com"}},
		{"drops_empty_entries", ",,foo.com,,bar.com,,", []string{"foo.com", "bar.com"}},
		{"only_empties_yields_nil", ",, ,\t,,", nil},
		{"leading_dot_suffix_preserved", ".example.com", []string{".example.com"}},
		{"mixed_exact_and_suffix", "webhook.example.com,.internal.example.com", []string{"webhook.example.com", ".internal.example.com"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]string{
				"CLOCKIFY_API_KEY": "test-key",
			}
			if c.raw != "" {
				env["CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS"] = c.raw
			}
			setEnvs(t, env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !slices.Equal(cfg.WebhookAllowedDomains, c.want) {
				t.Fatalf("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=%q → %v, want %v", c.raw, cfg.WebhookAllowedDomains, c.want)
			}
		})
	}
}

// TestLoad_SanitizeUpstreamErrors_ExplicitOverride confirms
// CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0 wins over the hosted-profile
// default (operator-explicit always overrides profile defaults).
func TestLoad_SanitizeUpstreamErrors_ExplicitOverride(t *testing.T) {
	env := map[string]string{
		"CLOCKIFY_API_KEY":                  "test-key",
		"MCP_PROFILE":                       "shared-service",
		"CLOCKIFY_SANITIZE_UPSTREAM_ERRORS": "0",
	}
	maps.Copy(env, hostedProfileEnv)
	setEnvs(t, env)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SanitizeUpstreamErrors {
		t.Fatal("explicit CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0 must override hosted default")
	}
}

// TestLoad_HostedProfileRefusesInsecure exercises the new hosted-profile
// guardrail: shared-service and prod-postgres must reject CLOCKIFY_INSECURE=1
// because remote HTTP in a multi-tenant deployment is a credential-leak
// risk. Local profiles still accept the override.
func TestLoad_HostedProfileRefusesInsecure(t *testing.T) {
	cases := []struct {
		profile string
		want    bool // want error?
	}{
		{"shared-service", true},
		{"prod-postgres", true},
		{"local-stdio", false},
		{"single-tenant-http", false},
	}
	for _, c := range cases {
		t.Run(c.profile, func(t *testing.T) {
			setEnvs(t, map[string]string{
				"CLOCKIFY_API_KEY":  "test-key",
				"CLOCKIFY_INSECURE": "1",
				"MCP_PROFILE":       c.profile,
				// shared-service / prod-postgres set MCP_TRANSPORT=streamable_http,
				// which doesn't require API key — but supplying one keeps the
				// rest of Load() on the happy path.
			})
			_, err := Load()
			if c.want && err == nil {
				t.Fatalf("expected hosted profile %q to reject CLOCKIFY_INSECURE=1", c.profile)
			}
			if c.want && !strings.Contains(err.Error(), "CLOCKIFY_INSECURE=1") {
				t.Fatalf("error %q should reference CLOCKIFY_INSECURE=1", err)
			}
			if !c.want && err != nil && strings.Contains(err.Error(), "CLOCKIFY_INSECURE=1") {
				t.Fatalf("non-hosted profile %q should not reject INSECURE: %v", c.profile, err)
			}
		})
	}
}
