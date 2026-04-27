package main

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/config"
)

func TestEffectiveVersionDefaultIsNotStaleReleaseLiteral(t *testing.T) {
	if version == "1.0.0" {
		t.Fatal("default version literal must not be a stale release number")
	}
	if got := effectiveVersion(); got == "1.0.0" {
		t.Fatalf("effectiveVersion() = %q, want non-stale default", got)
	}
}

func TestEffectiveVersionPrefersInjectedVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v9.9.9-test"

	if got := effectiveVersion(); got != "v9.9.9-test" {
		t.Fatalf("effectiveVersion() = %q, want injected version", got)
	}
}

func TestDoctorStrictFailsUnsafeHostedPosture(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict"}, map[string]string{
		"MCP_TRANSPORT":              "streamable_http",
		"MCP_AUTH_MODE":              "oidc",
		"MCP_OIDC_ISSUER":            "https://issuer.example",
		"MCP_CONTROL_PLANE_DSN":      "memory",
		"MCP_ALLOW_DEV_BACKEND":      "1",
		"MCP_AUDIT_DURABILITY":       "best_effort",
		"MCP_EXPOSE_AUTH_ERRORS":     "1",
		"MCP_DISABLE_INLINE_SECRETS": "0",
		"CLOCKIFY_POLICY":            "standard",
	})
	if code != 3 {
		t.Fatalf("runDoctor strict exit = %d, want 3; output:\n%s", code, out)
	}
	for _, want := range []string{
		"Strict posture", "Severity", "MCP_OIDC_STRICT", "MCP_OIDC_AUDIENCE/MCP_RESOURCE_URI",
		"MCP_REQUIRE_TENANT_CLAIM", "MCP_DISABLE_INLINE_SECRETS", "MCP_EXPOSE_AUTH_ERRORS",
		"MCP_CONTROL_PLANE_DSN", "MCP_AUDIT_DURABILITY", "CLOCKIFY_POLICY",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorStrictProdPostgresPasses(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--profile=prod-postgres", "--strict"}, map[string]string{
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-prod",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
	})
	if code != 0 {
		t.Fatalf("runDoctor strict prod-postgres exit = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Strict posture") || !strings.Contains(out, "OK") {
		t.Fatalf("doctor strict success output missing OK posture:\n%s", out)
	}
}

func TestDoctorStrictSyntheticConfigPasses(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict"}, strictCleanDoctorEnv(nil))
	if code != 0 {
		t.Fatalf("doctor --strict synthetic config exit = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Strict posture") || !strings.Contains(out, "OK") {
		t.Fatalf("doctor strict success output missing OK posture:\n%s", out)
	}
}

func TestDoctorStrictAllowBroadPolicyFlag(t *testing.T) {
	env := map[string]string{
		"MCP_TRANSPORT":              "streamable_http",
		"MCP_AUTH_MODE":              "oidc",
		"MCP_OIDC_ISSUER":            "https://issuer.example",
		"MCP_OIDC_AUDIENCE":          "clockify-mcp-prod",
		"MCP_OIDC_STRICT":            "1",
		"MCP_REQUIRE_TENANT_CLAIM":   "1",
		"MCP_DISABLE_INLINE_SECRETS": "1",
		"MCP_CONTROL_PLANE_DSN":      "postgres://db/mcp",
		"MCP_AUDIT_DURABILITY":       "fail_closed",
		"CLOCKIFY_POLICY":            "safe_core",
	}
	code, out := runDoctorForTest(t, []string{"--strict"}, env)
	if code != 3 || !strings.Contains(out, "CLOCKIFY_POLICY") {
		t.Fatalf("strict broad policy exit/output mismatch: code=%d output:\n%s", code, out)
	}

	code, out = runDoctorForTest(t, []string{"--strict", "--allow-broad-policy"}, env)
	if code != 0 {
		t.Fatalf("strict --allow-broad-policy exit = %d, want 0; output:\n%s", code, out)
	}
}

func TestDoctorStrictIndividualPostureFindings(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "missing OIDC audience or resource",
			env: strictCleanDoctorEnv(map[string]string{
				"MCP_OIDC_STRICT":   "0",
				"MCP_OIDC_AUDIENCE": "",
				"MCP_RESOURCE_URI":  "",
			}),
			want: "MCP_OIDC_AUDIENCE/MCP_RESOURCE_URI",
		},
		{
			name: "exposed auth errors",
			env: strictCleanDoctorEnv(map[string]string{
				"MCP_EXPOSE_AUTH_ERRORS": "1",
			}),
			want: "MCP_EXPOSE_AUTH_ERRORS",
		},
		{
			name: "memory control plane",
			env: strictCleanDoctorEnv(map[string]string{
				"MCP_CONTROL_PLANE_DSN": "memory",
				"MCP_ALLOW_DEV_BACKEND": "1",
			}),
			want: "MCP_CONTROL_PLANE_DSN",
		},
		{
			name: "file control plane",
			env: strictCleanDoctorEnv(map[string]string{
				"MCP_CONTROL_PLANE_DSN": "file:///tmp/clockify-mcp-cp.json",
				"MCP_ALLOW_DEV_BACKEND": "1",
			}),
			want: "MCP_CONTROL_PLANE_DSN",
		},
		{
			name: "safe_core policy",
			env: strictCleanDoctorEnv(map[string]string{
				"CLOCKIFY_POLICY": "safe_core",
			}),
			want: "CLOCKIFY_POLICY",
		},
		{
			name: "standard policy",
			env: strictCleanDoctorEnv(map[string]string{
				"CLOCKIFY_POLICY": "standard",
			}),
			want: "CLOCKIFY_POLICY",
		},
		{
			name: "full policy",
			env: strictCleanDoctorEnv(map[string]string{
				"CLOCKIFY_POLICY": "full",
			}),
			want: "CLOCKIFY_POLICY",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := runDoctorForTest(t, []string{"--strict"}, tc.env)
			if code != 3 || !strings.Contains(out, tc.want) {
				t.Fatalf("strict finding exit/output mismatch: code=%d want key %q output:\n%s", code, tc.want, out)
			}
		})
	}
}

func TestDoctorStrictAcceptsPostgresqlDSN(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict"}, strictCleanDoctorEnv(map[string]string{
		"MCP_CONTROL_PLANE_DSN": "postgresql://db/mcp",
	}))
	if code != 0 {
		t.Fatalf("strict postgresql DSN exit = %d, want 0; output:\n%s", code, out)
	}
}

func TestDoctorStrictForbidsLegacyHTTP(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict"}, map[string]string{
		"CLOCKIFY_API_KEY":           "test-key",
		"MCP_TRANSPORT":              "http",
		"MCP_AUTH_MODE":              "static_bearer",
		"MCP_BEARER_TOKEN":           "super-secret-token-at-least-sixteen",
		"MCP_CONTROL_PLANE_DSN":      "postgres://db/mcp",
		"MCP_AUDIT_DURABILITY":       "fail_closed",
		"MCP_DISABLE_INLINE_SECRETS": "1",
		"CLOCKIFY_POLICY":            "time_tracking_safe",
	})
	if code != 3 || !strings.Contains(out, "MCP_TRANSPORT") {
		t.Fatalf("strict legacy-http exit/output mismatch: code=%d output:\n%s", code, out)
	}
}

func TestDoctorStrictMTLSRequiresCertTenantSource(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict"}, map[string]string{
		"CLOCKIFY_API_KEY":           "test-key",
		"MCP_TRANSPORT":              "grpc",
		"MCP_AUTH_MODE":              "mtls",
		"MCP_GRPC_TLS_CERT":          "/tmp/server.crt",
		"MCP_GRPC_TLS_KEY":           "/tmp/server.key",
		"MCP_MTLS_CA_CERT_PATH":      "/tmp/ca.crt",
		"MCP_MTLS_TENANT_SOURCE":     "header_or_cert",
		"MCP_CONTROL_PLANE_DSN":      "postgres://db/mcp",
		"MCP_AUDIT_DURABILITY":       "fail_closed",
		"MCP_DISABLE_INLINE_SECRETS": "1",
		"CLOCKIFY_POLICY":            "time_tracking_safe",
	})
	if code != 3 || !strings.Contains(out, "MCP_MTLS_TENANT_SOURCE") {
		t.Fatalf("strict mtls tenant source exit/output mismatch: code=%d output:\n%s", code, out)
	}
}

// strictMTLSDoctorEnv returns a self-consistent strict-posture env for
// MCP_AUTH_MODE=mtls. Pinning every required strict flag here keeps the
// mTLS-specific tests below focused on the one assertion they care about
// (require-mtls-tenant) rather than tripping over unrelated findings.
func strictMTLSDoctorEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"CLOCKIFY_API_KEY":           "test-key",
		"MCP_TRANSPORT":              "grpc",
		"MCP_AUTH_MODE":              "mtls",
		"MCP_GRPC_TLS_CERT":          "/tmp/server.crt",
		"MCP_GRPC_TLS_KEY":           "/tmp/server.key",
		"MCP_MTLS_CA_CERT_PATH":      "/tmp/ca.crt",
		"MCP_MTLS_TENANT_SOURCE":     "cert",
		"MCP_REQUIRE_MTLS_TENANT":    "1",
		"MCP_CONTROL_PLANE_DSN":      "postgres://db/mcp",
		"MCP_AUDIT_DURABILITY":       "fail_closed",
		"MCP_DISABLE_INLINE_SECRETS": "1",
		"CLOCKIFY_POLICY":            "time_tracking_safe",
	}
	maps.Copy(env, overrides)
	return env
}

// TestDoctorStrictMTLSRequiresTenantRequired locks that hosted strict
// posture refuses an mTLS deployment that has not set
// MCP_REQUIRE_MTLS_TENANT=1. Without that flag a client whose verified
// cert exposes no tenant identity silently collapses onto
// MCP_DEFAULT_TENANT_ID — the multi-tenant footgun this gate closes.
func TestDoctorStrictMTLSRequiresTenantRequired(t *testing.T) {
	env := strictMTLSDoctorEnv(map[string]string{
		"MCP_REQUIRE_MTLS_TENANT": "0",
	})
	code, out := runDoctorForTest(t, []string{"--strict"}, env)
	if code != 3 {
		t.Fatalf("strict mtls without require-tenant exit = %d, want 3; output:\n%s", code, out)
	}
	if !strings.Contains(out, "hosted strict mTLS posture requires MCP_REQUIRE_MTLS_TENANT=1") {
		t.Fatalf("doctor output missing MCP_REQUIRE_MTLS_TENANT finding message:\n%s", out)
	}
}

// TestDoctorStrictMTLSWithRequireTenantPasses confirms the happy path:
// mTLS + tenant source cert + require-mtls-tenant + every other strict
// flag self-consistent → no findings.
func TestDoctorStrictMTLSWithRequireTenantPasses(t *testing.T) {
	env := strictMTLSDoctorEnv(nil)
	code, out := runDoctorForTest(t, []string{"--strict"}, env)
	if code != 0 {
		t.Fatalf("strict mtls happy path exit = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Strict posture") || !strings.Contains(out, "OK") {
		t.Fatalf("doctor strict success output missing OK posture:\n%s", out)
	}
}

// TestDoctorStrictNonMTLSDoesNotRequireMTLSTenantRequired pins the
// negative half: an OIDC deployment must not be flagged for missing
// MCP_REQUIRE_MTLS_TENANT — the gate is mTLS-specific.
func TestDoctorStrictNonMTLSDoesNotRequireMTLSTenantRequired(t *testing.T) {
	env := strictCleanDoctorEnv(map[string]string{
		"MCP_REQUIRE_MTLS_TENANT": "0",
	})
	code, out := runDoctorForTest(t, []string{"--strict"}, env)
	if code != 0 {
		t.Fatalf("strict OIDC without require-mtls-tenant exit = %d, want 0; output:\n%s", code, out)
	}
	if strings.Contains(out, "hosted strict mTLS posture requires MCP_REQUIRE_MTLS_TENANT=1") {
		t.Fatalf("OIDC posture flagged MCP_REQUIRE_MTLS_TENANT (mTLS-only gate); output:\n%s", out)
	}
}

func TestDoctorStrictCheckBackendsPreservesLoadErrorExit(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict", "--check-backends"}, strictCleanDoctorEnv(map[string]string{
		"MCP_AUDIT_DURABILITY": "sometimes",
	}))
	if code != 2 {
		t.Fatalf("doctor load-error exit = %d, want 2; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Load() result") || !strings.Contains(out, "invalid MCP_AUDIT_DURABILITY") {
		t.Fatalf("doctor load-error output missing config failure:\n%s", out)
	}
}

func TestParseDoctorArgsCheckBackends(t *testing.T) {
	opts := parseDoctorArgs([]string{"--strict", "--check-backends"})
	if !opts.strict {
		t.Fatal("parseDoctorArgs did not set strict")
	}
	if !opts.checkBackends {
		t.Fatal("parseDoctorArgs did not set checkBackends")
	}
}

func runDoctorForTest(t *testing.T, args []string, env map[string]string) (int, string) {
	t.Helper()
	for _, spec := range config.AllSpecs() {
		t.Setenv(spec.Name, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	var out bytes.Buffer
	code := runDoctorReport(args, &out)
	return code, out.String()
}

func strictCleanDoctorEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"MCP_TRANSPORT":              "streamable_http",
		"MCP_AUTH_MODE":              "oidc",
		"MCP_OIDC_ISSUER":            "https://issuer.example",
		"MCP_OIDC_AUDIENCE":          "clockify-mcp-prod",
		"MCP_OIDC_STRICT":            "1",
		"MCP_REQUIRE_TENANT_CLAIM":   "1",
		"MCP_DISABLE_INLINE_SECRETS": "1",
		"MCP_CONTROL_PLANE_DSN":      "postgres://db/mcp",
		"MCP_AUDIT_DURABILITY":       "fail_closed",
		"CLOCKIFY_POLICY":            "time_tracking_safe",
	}
	maps.Copy(env, overrides)
	return env
}
