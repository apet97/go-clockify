package config

import (
	"os"
	"regexp"
	"testing"
)

// TestEnvSpec_CoversEveryGetenv fails if any os.Getenv("MCP_*"/"CLOCKIFY_*"/"ENVIRONMENT")
// call in config.go is missing from AllSpecs(). This is the guarantee that
// EnvSpec is the single source of truth, not a convenience registry that drifts.
func TestEnvSpec_CoversEveryGetenv(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	// Catch both direct os.Getenv and any helper whose single string arg names
	// the env var (e.g. optionalBoolEnv, optionalDurationEnv). This matches
	// every call-site pattern in config.go today.
	re := regexp.MustCompile(`(?:os\.Getenv|optional[A-Za-z]*Env|require[A-Za-z]*Env)\("((?:MCP_|CLOCKIFY_|ENVIRONMENT)[A-Z0-9_]*)"\)`)
	referenced := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		referenced[m[1]] = true
	}

	// Allowlist: names intentionally handled outside config.go (e.g. in main.go
	// or tool-layer packages) but not surfaced as Config fields. They are
	// declared in AllSpecs so help/README still document them, but they
	// wouldn't be matched by a grep on config.go. Keep tight.
	outOfConfigGo := map[string]bool{
		"MCP_LOG_LEVEL":                   true, // main.go
		"MCP_LOG_FORMAT":                  true, // main.go
		"CLOCKIFY_SUBJECT_IDLE_TTL":       true, // main.go (subject reaper)
		"CLOCKIFY_SUBJECT_SWEEP_INTERVAL": true, // main.go (subject reaper)
		"CLOCKIFY_POLICY":                 true, // internal/enforcement
		"CLOCKIFY_DRY_RUN":                true, // internal/enforcement
		"CLOCKIFY_DEDUPE_MODE":            true, // internal/enforcement
		"CLOCKIFY_DEDUPE_LOOKBACK":        true, // internal/enforcement
		"CLOCKIFY_OVERLAP_CHECK":          true, // internal/enforcement
		"CLOCKIFY_DENY_TOOLS":             true, // internal/enforcement
		"CLOCKIFY_DENY_GROUPS":            true, // internal/enforcement
		"CLOCKIFY_ALLOW_GROUPS":           true, // internal/enforcement
		"CLOCKIFY_MAX_CONCURRENT":         true, // main.go
		"CLOCKIFY_RATE_LIMIT":             true, // internal/ratelimit
		"CLOCKIFY_PER_TOKEN_CONCURRENCY":  true, // internal/ratelimit
		"CLOCKIFY_PER_TOKEN_RATE_LIMIT":   true, // internal/ratelimit
		"CLOCKIFY_TOKEN_BUDGET":           true, // internal/truncate
		"CLOCKIFY_BOOTSTRAP_MODE":         true, // internal/tools
		"CLOCKIFY_BOOTSTRAP_TOOLS":        true, // internal/tools
	}

	spec := map[string]bool{}
	for _, s := range AllSpecs() {
		spec[s.Name] = true
	}

	// Direction 1: every os.Getenv in config.go must be in the spec.
	for name := range referenced {
		if !spec[name] {
			t.Errorf("os.Getenv(%q) in config.go but missing from AllSpecs()", name)
		}
	}

	// Direction 2: every spec entry must either be referenced in config.go
	// or explicitly marked as handled elsewhere. Prevents stale entries from
	// lingering in the registry.
	for _, s := range AllSpecs() {
		if referenced[s.Name] || outOfConfigGo[s.Name] {
			continue
		}
		t.Errorf("spec entry %q not referenced in config.go and not in outOfConfigGo allowlist", s.Name)
	}
}

// TestEnvSpec_NoDuplicates prevents accidental double entries when a new
// variable is added in a merge conflict.
func TestEnvSpec_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range AllSpecs() {
		if seen[s.Name] {
			t.Fatalf("duplicate spec entry: %s", s.Name)
		}
		seen[s.Name] = true
	}
}

// TestEnvSpec_EssentialDocsIncludeCoreFields asserts the README essentials
// table surfaces the settings an operator actually needs on day one: transport,
// auth, size, metrics, control-plane, audit, policy. If EssentialDoc is removed
// from one of these by mistake, the README silently loses coverage.
func TestEnvSpec_EssentialDocsIncludeCoreFields(t *testing.T) {
	required := []string{
		"CLOCKIFY_API_KEY",
		"CLOCKIFY_POLICY",
		"MCP_TRANSPORT",
		"MCP_HTTP_BIND",
		"MCP_AUTH_MODE",
		"MCP_MAX_MESSAGE_SIZE",
		"MCP_METRICS_BIND",
		"MCP_CONTROL_PLANE_DSN",
		"MCP_AUDIT_DURABILITY",
	}
	essential := map[string]bool{}
	for _, s := range AllSpecs() {
		if s.EssentialDoc {
			essential[s.Name] = true
		}
	}
	for _, name := range required {
		if !essential[name] {
			t.Errorf("expected %s in README essentials (EssentialDoc=true)", name)
		}
	}
}
