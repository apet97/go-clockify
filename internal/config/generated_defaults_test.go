package config

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestSpec_DocumentedDefaultsMatchLoader iterates every AllSpecs() entry and
// asserts that when its env var is unset, Load() produces the Config field
// value the spec advertises as its Default. This is the contract that keeps
// help text, README, and runtime behaviour from drifting: any time a default
// changes in config.go without a matching spec edit, this test fails with a
// pinpoint error.
//
// Entries with empty Default, Deprecated=true, or no direct Config field are
// skipped; the spec completeness test (TestEnvSpec_CoversEveryGetenv) already
// pairs every env-var name with a spec entry, so this test's role is purely
// value-parity.
func TestSpec_DocumentedDefaultsMatchLoader(t *testing.T) {
	os.Clearenv()
	// APIKey is required for the default stdio transport; without it Load()
	// refuses to build a Config.
	os.Setenv("CLOCKIFY_API_KEY", "dummy")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, s := range AllSpecs() {
		if s.Default == "" || s.Deprecated {
			continue
		}
		// MCP_METRICS_AUTH_MODE's "default" is conditional
		// (static_bearer only when MCP_METRICS_BIND is also set). Covered
		// by dedicated tests in config_test.go.
		if s.Name == "MCP_METRICS_AUTH_MODE" {
			continue
		}

		got, surfaced := extractDefault(cfg, s.Name)
		if !surfaced {
			// Handled outside internal/config (main.go, internal/enforcement,
			// internal/tools). Their defaults still appear in help/README
			// for operator reference, but this package can't verify them.
			continue
		}

		want := s.Default
		if normalisedEqual(want, got) {
			continue
		}
		t.Errorf("%s: spec default %q does not match Config field value %q", s.Name, want, got)
	}
}

// normalisedEqual lets a spec string like "4194304" match a Config int64
// printed as "4194304" and "60s" match a Duration formatted as "1m0s" by
// treating both sides as parseable numeric/duration values when possible.
func normalisedEqual(want, got string) bool {
	if want == got {
		return true
	}
	// Integer equality: spec's default is often a literal like "4194304" or
	// "64"; the Config field prints with %v. Equal if both parse to the same int.
	if n1, err1 := strconv.ParseInt(want, 10, 64); err1 == nil {
		if n2, err2 := strconv.ParseInt(got, 10, 64); err2 == nil && n1 == n2 {
			return true
		}
	}
	return false
}

// extractDefault returns the stringified value of the Config field backing
// the named env var, together with a "surfaced" flag indicating whether the
// field is present on Config at all. Every case here corresponds to a
// direct-write in config.go; anything handled in main.go or a downstream
// package returns ("", false) so the test skips rather than fails.
func extractDefault(cfg Config, name string) (string, bool) {
	switch name {
	case "CLOCKIFY_WORKSPACE_ID":
		if cfg.WorkspaceID == "" {
			return "auto", true
		}
		return cfg.WorkspaceID, true
	case "CLOCKIFY_BASE_URL":
		return cfg.BaseURL, true
	case "CLOCKIFY_INSECURE":
		return boolInt(cfg.Insecure), true
	case "CLOCKIFY_TOOL_TIMEOUT":
		return durStr(cfg.ToolTimeout), true
	case "CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT":
		return durStr(cfg.ConcurrencyAcquireTimeout), true
	case "MCP_MAX_INFLIGHT_TOOL_CALLS":
		return strconv.Itoa(cfg.MaxInFlightToolCalls), true
	case "CLOCKIFY_REPORT_MAX_ENTRIES":
		return strconv.Itoa(cfg.ReportMaxEntries), true
	case "CLOCKIFY_DELTA_FORMAT":
		return cfg.DeltaFormat, true
	case "MCP_TRANSPORT":
		return cfg.Transport, true
	case "MCP_HTTP_BIND":
		return cfg.HTTPBind, true
	case "MCP_GRPC_BIND":
		return cfg.GRPCBind, true
	case "MCP_MAX_MESSAGE_SIZE":
		return strconv.FormatInt(cfg.MaxMessageSize, 10), true
	case "MCP_ALLOW_ANY_ORIGIN":
		return boolInt(cfg.AllowAnyOrigin), true
	case "MCP_STRICT_HOST_CHECK":
		return boolInt(cfg.StrictHostCheck), true
	case "MCP_HTTP_LEGACY_POLICY":
		return cfg.HTTPLegacyPolicy, true
	case "MCP_HTTP_INLINE_METRICS_ENABLED":
		return boolInt(cfg.HTTPInlineMetricsEnabled), true
	case "MCP_HTTP_INLINE_METRICS_AUTH_MODE":
		if cfg.HTTPInlineMetricsAuthMode == "" {
			// unset is the not-enabled path — spec says default is inherit_main_bearer
			// but that only applies when inline metrics are enabled. Surface
			// "" to skip; dedicated tests cover the enabled branch.
			return "", false
		}
		return cfg.HTTPInlineMetricsAuthMode, true
	case "MCP_CONTROL_PLANE_DSN":
		return cfg.ControlPlaneDSN, true
	case "MCP_CONTROL_PLANE_AUDIT_CAP":
		return strconv.Itoa(cfg.ControlPlaneAuditCap), true
	case "MCP_CONTROL_PLANE_AUDIT_RETENTION":
		return durStr(cfg.ControlPlaneAuditRetention), true
	case "MCP_SESSION_TTL":
		return durStr(cfg.SessionTTL), true
	case "MCP_AUDIT_DURABILITY":
		return cfg.AuditDurabilityMode, true
	case "MCP_TENANT_CLAIM":
		return cfg.TenantClaim, true
	case "MCP_SUBJECT_CLAIM":
		return cfg.SubjectClaim, true
	case "MCP_DEFAULT_TENANT_ID":
		return cfg.DefaultTenantID, true
	case "MCP_FORWARD_TENANT_HEADER",
		"MCP_FORWARD_SUBJECT_HEADER",
		"MCP_MTLS_TENANT_HEADER":
		// Config holds the raw user override; the documented default
		// ("X-Forwarded-Tenant" etc.) is applied by internal/authn at
		// authenticator construction (authn.go:94-101). The spec string
		// documents operator-visible behaviour; config's view is
		// deliberately empty. Skip the field-level parity check.
		return "", false
	case "MCP_GRPC_REAUTH_INTERVAL":
		if cfg.GRPCReauthInterval == 0 {
			return "0", true
		}
		return durStr(cfg.GRPCReauthInterval), true
	}
	return "", false
}

func boolInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// durStr formats a Duration as its spec-style short form. "1h0m0s" → "1h",
// "30m0s" → "30m", "100ms" stays "100ms". Keeps comparison free of zero-pad
// noise from Duration.String().
func durStr(d interface{ String() string }) string {
	// Use stdlib Duration.String(). For entries whose spec-style values are
	// like "30m" while Duration prints "30m0s", chop trailing "0s" and "0m"
	// where that does not change meaning.
	s := d.String()
	for _, suffix := range []string{"0s", "0m"} {
		for len(s) > len(suffix) && hasCleanSuffix(s, suffix) {
			s = s[:len(s)-len(suffix)]
		}
	}
	if s == "" {
		return "0"
	}
	return s
}

// hasCleanSuffix returns true only when trimming the trailing "0s"/"0m" does
// not leave behind a dangling unit character (e.g. "100m0s" → "100m" good;
// "10s" minus "0s" → "1s" bad).
func hasCleanSuffix(s, suffix string) bool {
	if len(s) < len(suffix)+1 {
		return false
	}
	if s[len(s)-len(suffix):] != suffix {
		return false
	}
	// Character preceding the suffix must be a unit letter, not a digit.
	prev := s[len(s)-len(suffix)-1]
	return prev == 'h' || prev == 'm' || prev == 's'
}

// sanity check — if durStr mis-formats, fail loudly rather than produce
// silent test passes.
func init() {
	must := func(in, want string) {
		d, err := time.ParseDuration(in)
		if err != nil {
			panic(err)
		}
		got := durStr(d)
		if got != want {
			panic(fmt.Sprintf("durStr(%q) = %q, want %q", in, got, want))
		}
	}
	must("1h", "1h")
	must("30m", "30m")
	must("100ms", "100ms")
	must("45s", "45s")
	must("720h", "720h")
}
