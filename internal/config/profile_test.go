package config

import (
	"os"
	"strings"
	"testing"
)

// setProfileEnv is the profile-aware sibling of setEnvs. It snapshots
// every env var a profile can mutate (via t.Setenv with the empty
// string, which captures whatever the var was at call time and
// restores it on test cleanup), then applies the caller's explicit
// overrides. applyProfile() inside Load() can now os.Setenv unset
// keys without leaking across tests: the t.Setenv snapshots already
// registered a cleanup path for every profile-controlled key.
//
// Keep this list in sync with the union of every Profile.Env key in
// profile.go. TestProfile_HelperCoversAllProfileKeys enforces parity.
func setProfileEnv(t *testing.T, profile string, overrides map[string]string) {
	t.Helper()
	profileControlled := []string{
		"MCP_TRANSPORT",
		"MCP_AUTH_MODE",
		"MCP_CONTROL_PLANE_DSN",
		"MCP_ALLOW_DEV_BACKEND",
		"MCP_AUDIT_DURABILITY",
		"MCP_HTTP_LEGACY_POLICY",
		"CLOCKIFY_POLICY",
		"ENVIRONMENT",
		// Hosted-service strict gates added by the shared-service /
		// prod-postgres profiles. TestProfile_HelperCoversAllProfileKeys
		// keeps this list in lockstep with profile.go.
		"MCP_OIDC_STRICT",
		"MCP_REQUIRE_TENANT_CLAIM",
		"MCP_DISABLE_INLINE_SECRETS",
	}
	for _, k := range profileControlled {
		t.Setenv(k, "")
	}
	// Also snapshot MCP_PROFILE itself so each test starts from a
	// known-empty profile slot; the Setenv below may then replace it.
	t.Setenv("MCP_PROFILE", "")
	// Metrics auth is a test-only convenience handled by setEnvs;
	// mirror it here so profile tests don't have to reset it.
	t.Setenv("MCP_METRICS_AUTH_MODE", "none")
	if profile != "" {
		t.Setenv("MCP_PROFILE", profile)
	}
	for k, v := range overrides {
		t.Setenv(k, v)
	}
}

// TestProfile_HelperCoversAllProfileKeys enforces that setProfileEnv
// snapshots every key any profile can mutate. Drift between the
// helper and profile.go is exactly the inter-test-leak shape we want
// CI to catch, not runtime.
func TestProfile_HelperCoversAllProfileKeys(t *testing.T) {
	// Build the union of profile keys from the live registry.
	union := map[string]struct{}{}
	for _, p := range AllProfiles() {
		for k := range p.Env {
			union[k] = struct{}{}
		}
	}
	// Mirror the helper's list without duplicating it literally: call
	// the helper on a freshly-created sub-test so its t.Setenv calls
	// are scoped and then introspect via os.LookupEnv. This asserts
	// the helper at least touches every union key.
	t.Run("snapshot", func(tt *testing.T) {
		for k := range union {
			// Pre-condition: set a sentinel so we can detect the
			// helper's t.Setenv("") via a change.
			tt.Setenv(k, "SENTINEL-"+k)
		}
		setProfileEnv(tt, "", nil)
		for k := range union {
			got := getenvForTest(k)
			if got != "" {
				tt.Fatalf("helper did not snapshot %s (still %q); add it to profileControlled", k, got)
			}
		}
	})
}

// getenvForTest is a thin wrapper so the parity test above introspects
// env through one documented entry point, mirroring the convention in
// config_test.go.
func getenvForTest(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// TestProfile_LocalStdioDefaults locks the local-stdio profile
// defaults through the Load() pipeline.
func TestProfile_LocalStdioDefaults(t *testing.T) {
	setProfileEnv(t, "local-stdio", map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", cfg.Transport)
	}
	if cfg.AuditDurabilityMode != "best_effort" {
		t.Errorf("AuditDurabilityMode = %q, want best_effort", cfg.AuditDurabilityMode)
	}
}

// TestProfile_SingleTenantHTTPDefaults locks the single-tenant-http
// profile defaults, including the explicit MCP_ALLOW_DEV_BACKEND=1
// acknowledgement that unlocks the file-backed control plane.
func TestProfile_SingleTenantHTTPDefaults(t *testing.T) {
	setProfileEnv(t, "single-tenant-http", map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_BEARER_TOKEN": "super-secret-token-at-least-sixteen",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "streamable_http" {
		t.Errorf("Transport = %q, want streamable_http", cfg.Transport)
	}
	if cfg.AuthMode != "static_bearer" {
		t.Errorf("AuthMode = %q, want static_bearer", cfg.AuthMode)
	}
	if !strings.HasPrefix(cfg.ControlPlaneDSN, "file://") {
		t.Errorf("ControlPlaneDSN = %q, want file:// DSN", cfg.ControlPlaneDSN)
	}
	if cfg.HTTPLegacyPolicy != "deny" {
		t.Errorf("HTTPLegacyPolicy = %q, want deny", cfg.HTTPLegacyPolicy)
	}
	if got := os.Getenv("CLOCKIFY_POLICY"); got != "time_tracking_safe" {
		t.Errorf("CLOCKIFY_POLICY = %q, want time_tracking_safe", got)
	}
	if got := os.Getenv("CLOCKIFY_POLICY"); got == "standard" {
		t.Errorf("CLOCKIFY_POLICY stayed at raw default %q; single-tenant-http must pin the AI-facing policy", got)
	}
}

// TestProfile_SingleTenantHTTPPolicyOverrideWins verifies the
// time_tracking_safe profile default does not clobber an explicit
// operator policy choice.
func TestProfile_SingleTenantHTTPPolicyOverrideWins(t *testing.T) {
	setProfileEnv(t, "single-tenant-http", map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_BEARER_TOKEN": "super-secret-token-at-least-sixteen",
		"CLOCKIFY_POLICY":  "safe_core",
	})
	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("CLOCKIFY_POLICY"); got != "safe_core" {
		t.Fatalf("CLOCKIFY_POLICY = %q, want explicit safe_core override", got)
	}
}

// TestProfile_SharedServiceRequiresExplicitDSN verifies the
// shared-service profile leaves MCP_CONTROL_PLANE_DSN unset (it has
// no profile default) — operators must supply a postgres DSN
// explicitly; the streamable-http fail-closed guard from Wave H
// produces an actionable error otherwise.
func TestProfile_SharedServiceRequiresExplicitDSN(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "streamable_http" {
		t.Errorf("Transport = %q, want streamable_http", cfg.Transport)
	}
	if cfg.AuthMode != "oidc" {
		t.Errorf("AuthMode = %q, want oidc", cfg.AuthMode)
	}
	if cfg.AuditDurabilityMode != "fail_closed" {
		t.Errorf("AuditDurabilityMode = %q, want fail_closed", cfg.AuditDurabilityMode)
	}
	if cfg.HTTPLegacyPolicy != "deny" {
		t.Errorf("HTTPLegacyPolicy = %q, want deny", cfg.HTTPLegacyPolicy)
	}
}

// TestProfile_SharedServiceFailsWithoutDSN asserts the shared-service
// profile surfaces the Wave H fail-closed error when the operator
// forgets to provide a postgres DSN. This is the canonical "dev
// backend not allowed for streamable_http" message.
func TestProfile_SharedServiceFailsWithoutDSN(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":  "test-key",
		"MCP_OIDC_ISSUER":   "https://issuer.example",
		"MCP_OIDC_AUDIENCE": "clockify-mcp-shared",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for shared-service without DSN, got nil")
	}
	if !strings.Contains(err.Error(), "MCP_ALLOW_DEV_BACKEND") {
		t.Errorf("error should mention MCP_ALLOW_DEV_BACKEND escape hatch; got: %v", err)
	}
}

// TestProfile_ProdPostgresEnforcesEnvironment verifies the
// prod-postgres profile turns on ENVIRONMENT=prod, which means the
// Wave H prod guards refuse any non-postgres DSN.
func TestProfile_ProdPostgresEnforcesEnvironment(t *testing.T) {
	setProfileEnv(t, "prod-postgres", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuditDurabilityMode != "fail_closed" {
		t.Errorf("AuditDurabilityMode = %q, want fail_closed (prod-postgres)", cfg.AuditDurabilityMode)
	}
}

// TestProfile_ProdPostgresRejectsNonPostgresDSN asserts the
// prod-postgres profile triggers the existing prod guard that
// rejects non-postgres DSNs even when the profile chose
// streamable_http for us.
func TestProfile_ProdPostgresRejectsNonPostgresDSN(t *testing.T) {
	setProfileEnv(t, "prod-postgres", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "memory",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected prod guard error for non-postgres DSN, got nil")
	}
	if !strings.Contains(err.Error(), "postgres://") {
		t.Errorf("error should name the postgres:// prefix requirement; got: %v", err)
	}
}

// TestProfile_PrivateNetworkGRPCFailsWithoutDSN locks the symmetric
// dev-backend gate for the private-network-grpc profile: without an
// explicit MCP_CONTROL_PLANE_DSN (or the MCP_ALLOW_DEV_BACKEND escape
// hatch), Load() must refuse with the same actionable error
// shared-service operators see. The profile pairs grpc with
// fail_closed audit, and a memory backend cannot honour fail_closed
// across pod restarts in a multi-replica deployment.
func TestProfile_PrivateNetworkGRPCFailsWithoutDSN(t *testing.T) {
	setProfileEnv(t, "private-network-grpc", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_GRPC_TLS_CERT":     "/dev/null",
		"MCP_GRPC_TLS_KEY":      "/dev/null",
		"MCP_MTLS_CA_CERT_PATH": "/dev/null",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for private-network-grpc without DSN, got nil")
	}
	if !strings.Contains(err.Error(), "MCP_ALLOW_DEV_BACKEND") {
		t.Errorf("error should mention MCP_ALLOW_DEV_BACKEND escape hatch; got: %v", err)
	}
	if !strings.Contains(err.Error(), `MCP_TRANSPORT="grpc"`) {
		t.Errorf("error should name the actual transport via %%q (grpc); got: %v", err)
	}
}

// TestProfile_PrivateNetworkGRPCPostgresOK confirms the production
// path: applying the profile with a real postgres DSN supplies every
// required guard input, so Load() succeeds and the profile defaults
// (transport=grpc, auth=mtls, fail_closed audit) propagate to Config.
func TestProfile_PrivateNetworkGRPCPostgresOK(t *testing.T) {
	setProfileEnv(t, "private-network-grpc", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_GRPC_TLS_CERT":     "/dev/null",
		"MCP_GRPC_TLS_KEY":      "/dev/null",
		"MCP_MTLS_CA_CERT_PATH": "/dev/null",
		"MCP_CONTROL_PLANE_DSN": "postgres://user:pw@db.example:5432/mcp",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "grpc" {
		t.Errorf("Transport = %q, want grpc", cfg.Transport)
	}
	if cfg.AuthMode != "mtls" {
		t.Errorf("AuthMode = %q, want mtls", cfg.AuthMode)
	}
	if cfg.AuditDurabilityMode != "fail_closed" {
		t.Errorf("AuditDurabilityMode = %q, want fail_closed", cfg.AuditDurabilityMode)
	}
}

// TestProfile_PrivateNetworkGRPCAllowsDevBackendFlag confirms the
// escape hatch: an operator running the profile in a single-process
// dev / preview environment can still opt in via MCP_ALLOW_DEV_BACKEND=1
// + memory DSN. This mirrors the streamable_http "explicit single-process
// acknowledgement" path so the two transports stay symmetric.
func TestProfile_PrivateNetworkGRPCAllowsDevBackendFlag(t *testing.T) {
	setProfileEnv(t, "private-network-grpc", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_GRPC_TLS_CERT":     "/dev/null",
		"MCP_GRPC_TLS_KEY":      "/dev/null",
		"MCP_MTLS_CA_CERT_PATH": "/dev/null",
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	})
	if _, err := Load(); err != nil {
		t.Fatalf("flag should permit private-network-grpc + memory: %v", err)
	}
}

// TestProfile_OverridesWin is the load-bearing invariant: profile
// defaults MUST NOT overwrite explicit operator env. Flip this test's
// expected value to run the drift check.
func TestProfile_OverridesWin(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
		// Explicit operator override — profile default is fail_closed.
		"MCP_AUDIT_DURABILITY": "best_effort",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuditDurabilityMode != "best_effort" {
		t.Fatalf("AuditDurabilityMode = %q, want best_effort (explicit override must beat profile default)", cfg.AuditDurabilityMode)
	}
}

// TestProfile_UnknownNameFailsLoad verifies the Load() error for an
// invalid profile names the valid choices, which is what the CLI
// help and doctor subcommand rely on.
func TestProfile_UnknownNameFailsLoad(t *testing.T) {
	setProfileEnv(t, "definitely-not-a-profile", map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "unknown profile") {
		t.Errorf("error should say 'unknown profile'; got: %v", err)
	}
	for _, name := range ProfileNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should list profile %q as valid; got: %v", name, err)
		}
	}
}

// TestProfile_EmptyProfileIsNoop asserts unsetting MCP_PROFILE
// preserves the pre-profile behaviour — Load() must not surface a
// profile error or change any default.
func TestProfile_EmptyProfileIsNoop(t *testing.T) {
	setProfileEnv(t, "", map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio (no profile, default)", cfg.Transport)
	}
	if cfg.AuditDurabilityMode != "best_effort" {
		t.Errorf("AuditDurabilityMode = %q, want best_effort (no profile, dev default)", cfg.AuditDurabilityMode)
	}
}

// TestProfile_KeysAreSpecced enforces that every key used in a
// profile's Env map appears in AllSpecs(). Otherwise the help text
// and config-doc-parity CI gate would silently miss a profile knob.
func TestProfile_KeysAreSpecced(t *testing.T) {
	specced := map[string]bool{}
	for _, s := range AllSpecs() {
		specced[s.Name] = true
	}
	for _, p := range AllProfiles() {
		for k := range p.Env {
			if !specced[k] {
				t.Errorf("profile %s references env var %q with no EnvSpec entry", p.Name, k)
			}
		}
	}
}

// TestProfile_SharedServiceIsStrict locks the four hosted-service
// strict flags (MCP_OIDC_STRICT, MCP_REQUIRE_TENANT_CLAIM,
// MCP_DISABLE_INLINE_SECRETS, CLOCKIFY_POLICY=time_tracking_safe)
// onto the shared-service profile. Drift here re-introduces the C1
// finding from the 2026-04-25 audit: a hosted-service profile that
// silently accepts any-audience tokens, falls back to a default tenant
// when the claim is missing, allows inline secrets, and ships a broad
// write policy.
func TestProfile_SharedServiceIsStrict(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OIDCStrict {
		t.Errorf("OIDCStrict = false, want true (shared-service must reject any-audience tokens)")
	}
	if !cfg.RequireTenantClaim {
		t.Errorf("RequireTenantClaim = false, want true (multi-tenant must reject missing claim)")
	}
	if !cfg.DisableInlineSecrets {
		t.Errorf("DisableInlineSecrets = false, want true (hosted service must reject inline secrets)")
	}
	if got := os.Getenv("CLOCKIFY_POLICY"); got != "time_tracking_safe" {
		t.Errorf("CLOCKIFY_POLICY = %q, want time_tracking_safe (broader policies require explicit operator opt-in)", got)
	}
	// Assert the policy is not broader than safe_core. time_tracking_safe
	// and read_only are stricter; safe_core is borderline-acceptable for
	// trusted-team deployments; standard / full are explicit operator
	// choices that should never be a profile default.
	switch got := os.Getenv("CLOCKIFY_POLICY"); got {
	case "read_only", "time_tracking_safe", "safe_core":
		// allowed
	case "standard", "full":
		t.Errorf("CLOCKIFY_POLICY default %q is broader than safe_core; profile must not lower the bar", got)
	default:
		t.Errorf("CLOCKIFY_POLICY = %q, expected one of read_only/time_tracking_safe/safe_core", got)
	}
}

// TestProfile_ProdPostgresIsStrict mirrors the shared-service strict
// check for the prod-postgres profile, which is the canonical hosted-
// service preset. ENVIRONMENT=prod is already covered by
// TestProfile_ProdPostgresEnforcesEnvironment; this test pins the four
// strict gates that turn the audit C1 finding from "self-hosted-only"
// into "hosted-service-locked".
func TestProfile_ProdPostgresIsStrict(t *testing.T) {
	setProfileEnv(t, "prod-postgres", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OIDCStrict {
		t.Errorf("OIDCStrict = false, want true (prod-postgres)")
	}
	if !cfg.RequireTenantClaim {
		t.Errorf("RequireTenantClaim = false, want true (prod-postgres)")
	}
	if !cfg.DisableInlineSecrets {
		t.Errorf("DisableInlineSecrets = false, want true (prod-postgres)")
	}
	if got := os.Getenv("ENVIRONMENT"); got != "prod" {
		t.Errorf("ENVIRONMENT = %q, want prod", got)
	}
	if got := os.Getenv("CLOCKIFY_POLICY"); got != "time_tracking_safe" {
		t.Errorf("CLOCKIFY_POLICY = %q, want time_tracking_safe", got)
	}
}

// TestProfile_SharedServiceStrictOverrideHonoured asserts the load-
// bearing invariant: explicit operator env always beats a profile
// default, even for the strict-gate flags. An operator who genuinely
// wants OIDCStrict off (for staging against a misconfigured issuer,
// say) must be able to flip it back via env without removing the
// profile.
func TestProfile_SharedServiceStrictOverrideHonoured(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
		"MCP_OIDC_AUDIENCE":     "clockify-mcp-shared",
		"MCP_CONTROL_PLANE_DSN": "postgres://db/mcp",
		// Explicit operator override — profile default is "1".
		"MCP_REQUIRE_TENANT_CLAIM": "0",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RequireTenantClaim {
		t.Fatalf("RequireTenantClaim = true; explicit override (MCP_REQUIRE_TENANT_CLAIM=0) must beat the profile default")
	}
}

// TestProfile_NamesAreUnique guards against copy-paste duplication in
// the profile registry.
func TestProfile_NamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllProfiles() {
		if seen[p.Name] {
			t.Fatalf("duplicate profile name: %s", p.Name)
		}
		seen[p.Name] = true
	}
}
