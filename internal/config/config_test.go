package config

import (
	"os"
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

// setEnvs is a test helper that sets multiple env vars and returns a cleanup function.
func setEnvs(t *testing.T, envs map[string]string) {
	t.Helper()
	t.Setenv("MCP_METRICS_AUTH_MODE", "none")
	for k, v := range envs {
		t.Setenv(k, v)
	}
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
func TestLoadTransportGRPC(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_GRPC_BIND":    "127.0.0.1:7777",
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
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
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
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_AUTH_MODE":    "static_bearer",
		"MCP_BEARER_TOKEN": "1234567890abcdef",
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
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_AUTH_MODE":    "oidc",
		"MCP_OIDC_ISSUER":  "https://idp.example.com",
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
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_AUTH_MODE":    "forward_auth",
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
