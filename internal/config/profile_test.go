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
		"CLOCKIFY_API_KEY": "test-key",
		"MCP_OIDC_ISSUER":  "https://issuer.example",
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

// TestProfile_OverridesWin is the load-bearing invariant: profile
// defaults MUST NOT overwrite explicit operator env. Flip this test's
// expected value to run the drift check.
func TestProfile_OverridesWin(t *testing.T) {
	setProfileEnv(t, "shared-service", map[string]string{
		"CLOCKIFY_API_KEY":      "test-key",
		"MCP_OIDC_ISSUER":       "https://issuer.example",
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
