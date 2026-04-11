package config

import (
	"os"
	"strings"
	"testing"
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

func TestLoadMaxBodySizeDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	os.Unsetenv("MCP_HTTP_MAX_BODY")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxBodySize != 2097152 {
		t.Fatalf("expected 2097152 default, got %d", cfg.MaxBodySize)
	}
}

func TestLoadMaxBodySizeCustom(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "4194304",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxBodySize != 4194304 {
		t.Fatalf("expected 4194304, got %d", cfg.MaxBodySize)
	}
}

func TestLoadMaxBodySizeInvalidReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "not-a-number",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid MCP_HTTP_MAX_BODY")
	}
}

func TestLoadMaxBodySizeZeroReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "0",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero MCP_HTTP_MAX_BODY")
	}
}

func TestLoadMaxBodySizeTooLargeReturnsError(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_HTTP_MAX_BODY": "99999999999",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for MCP_HTTP_MAX_BODY exceeding 50MB")
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

// TestLoadTransportGRPCForwardAuthRejected documents the W4-03 stop
// condition: forward_auth needs X-Forwarded-{User,Tenant} headers that
// the synthetic request in the gRPC interceptor does not carry.
func TestLoadTransportGRPCForwardAuthRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_AUTH_MODE":    "forward_auth",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected gRPC + forward_auth to be rejected")
	}
	if !strings.Contains(err.Error(), "forward_auth") {
		t.Fatalf("expected error mentioning forward_auth, got: %v", err)
	}
}

// TestLoadTransportGRPCMTLSRejected documents the W4-03 stop condition:
// mtls needs real TLS VerifiedChains that the synthetic request cannot
// expose. Operators still front gRPC with an external mTLS terminator.
func TestLoadTransportGRPCMTLSRejected(t *testing.T) {
	setEnvs(t, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_TRANSPORT":    "grpc",
		"MCP_AUTH_MODE":    "mtls",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected gRPC + mtls to be rejected")
	}
	if !strings.Contains(err.Error(), "mtls") {
		t.Fatalf("expected error mentioning mtls, got: %v", err)
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
	})
	os.Unsetenv("CLOCKIFY_API_KEY")
	os.Unsetenv("MCP_OIDC_ISSUER")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing OIDC issuer to fail")
	}
}
