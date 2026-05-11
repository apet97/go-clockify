package policy

import (
	"strings"
	"testing"
)

// TestRank pins the explicit total ordering of policy modes. The
// ceiling contract surface lives on this ordering, not on the
// IsAllowed equivalence between Standard and Full. See ADR 0021.
func TestRank(t *testing.T) {
	cases := []struct {
		mode Mode
		want int
	}{
		{ReadOnly, 0},
		{TimeTrackingSafe, 1},
		{SafeCore, 2},
		{Standard, 3},
		{Full, 4},
	}
	for _, c := range cases {
		if got := Rank(c.mode); got != c.want {
			t.Errorf("Rank(%q) = %d, want %d", c.mode, got, c.want)
		}
	}
}

// TestRank_UnknownModeFailsClosed locks the contract that any string
// not in the five-mode enum gets a rank lower than ReadOnly. Every
// comparison against a real ceiling must therefore reject it.
func TestRank_UnknownModeFailsClosed(t *testing.T) {
	for _, raw := range []string{"", "bogus", "FULL", "tenant-admin", "STANDARD"} {
		if got := Rank(Mode(raw)); got >= 0 {
			t.Errorf("Rank(%q) = %d, want negative for unknown mode", raw, got)
		}
	}
}

// TestRank_FullStrictlyAboveStandard pins the deliberate divergence
// from IsAllowed equivalence. Conflating the two ranks would couple
// the ceiling contract to a current IsAllowed accident; any future
// feature that distinguishes Standard from Full (e.g. unlocking
// workspace-admin tools only under Full) must not silently widen
// deployments whose ceiling is pinned at Standard. See ADR 0021.
func TestRank_FullStrictlyAboveStandard(t *testing.T) {
	if Rank(Full) <= Rank(Standard) {
		t.Fatalf("Rank(Full)=%d must be strictly above Rank(Standard)=%d", Rank(Full), Rank(Standard))
	}
}

// TestIsAtMost pins the candidate <= ceiling check, including the
// "empty ceiling = no explicit constraint" carve-out used by the
// EffectiveTenantMode helper to fall back to the process mode.
func TestIsAtMost(t *testing.T) {
	cases := []struct {
		name      string
		candidate Mode
		ceiling   Mode
		want      bool
	}{
		{"empty ceiling permits any known mode", Standard, "", true},
		{"empty ceiling rejects unknown mode", Mode("bogus"), "", false},
		{"read_only<=time_tracking_safe", ReadOnly, TimeTrackingSafe, true},
		{"time_tracking_safe<=time_tracking_safe", TimeTrackingSafe, TimeTrackingSafe, true},
		{"safe_core>time_tracking_safe", SafeCore, TimeTrackingSafe, false},
		{"standard>time_tracking_safe", Standard, TimeTrackingSafe, false},
		{"full>time_tracking_safe", Full, TimeTrackingSafe, false},
		{"read_only<=safe_core", ReadOnly, SafeCore, true},
		{"safe_core<=safe_core", SafeCore, SafeCore, true},
		{"standard>safe_core", Standard, SafeCore, false},
		{"full>safe_core", Full, SafeCore, false},
		{"standard<=standard", Standard, Standard, true},
		{"full>standard", Full, Standard, false},
		{"standard<=full", Standard, Full, true},
		{"full<=full", Full, Full, true},
	}
	for _, c := range cases {
		if got := IsAtMost(c.candidate, c.ceiling); got != c.want {
			t.Errorf("%s: IsAtMost(%q, %q) = %v, want %v", c.name, c.candidate, c.ceiling, got, c.want)
		}
	}
}

// TestEffectiveTenantMode covers the full ceiling matrix the
// runtime layer relies on. Each row exercises one of the four
// invariants documented in ADR 0021: empty-inherit, unknown-fail,
// implicit ceiling via process mode, explicit ceiling enforcement.
func TestEffectiveTenantMode(t *testing.T) {
	cases := []struct {
		name        string
		processMode Mode
		tenantMode  Mode
		ceiling     Mode
		want        Mode
		wantErr     string // substring; "" means expect success
	}{
		{
			name:        "empty tenant inherits process mode",
			processMode: TimeTrackingSafe,
			tenantMode:  "",
			ceiling:     "",
			want:        TimeTrackingSafe,
		},
		{
			name:        "process broader than explicit ceiling fails before tenant is considered",
			processMode: Standard,
			tenantMode:  "",
			ceiling:     TimeTrackingSafe,
			wantErr:     "exceeds ceiling",
		},
		{
			name:        "process equal to ceiling with empty tenant returns process mode",
			processMode: TimeTrackingSafe,
			tenantMode:  "",
			ceiling:     TimeTrackingSafe,
			want:        TimeTrackingSafe,
		},
		{
			name:        "process narrower than ceiling with empty tenant returns process mode",
			processMode: ReadOnly,
			tenantMode:  "",
			ceiling:     TimeTrackingSafe,
			want:        ReadOnly,
		},
		{
			name:        "invalid process mode fails closed",
			processMode: Mode("tenant-admin"),
			tenantMode:  "",
			ceiling:     "",
			wantErr:     "invalid process mode",
		},
		{
			name:        "invalid ceiling fails closed",
			processMode: TimeTrackingSafe,
			tenantMode:  "",
			ceiling:     Mode("tenant-admin"),
			wantErr:     "invalid ceiling",
		},
		{
			name:        "tenant narrowing under explicit ceiling succeeds",
			processMode: SafeCore,
			tenantMode:  TimeTrackingSafe,
			ceiling:     SafeCore,
			want:        TimeTrackingSafe,
		},
		{
			name:        "tenant equal to process under matching ceiling succeeds",
			processMode: TimeTrackingSafe,
			tenantMode:  TimeTrackingSafe,
			ceiling:     TimeTrackingSafe,
			want:        TimeTrackingSafe,
		},
		{
			name:        "tenant broadening past implicit process ceiling fails closed",
			processMode: TimeTrackingSafe,
			tenantMode:  Standard,
			ceiling:     "",
			wantErr:     "exceeds",
		},
		{
			name:        "tenant broadening past explicit process+ceiling fails closed",
			processMode: TimeTrackingSafe,
			tenantMode:  Standard,
			ceiling:     TimeTrackingSafe,
			wantErr:     "exceeds",
		},
		{
			name:        "unknown tenant mode fails closed",
			processMode: Standard,
			tenantMode:  Mode("tenant-admin"),
			ceiling:     "",
			wantErr:     "invalid tenant",
		},
		{
			name:        "process standard + ceiling time_tracking_safe fails before tenant override",
			processMode: Standard,
			tenantMode:  ReadOnly, // tenant would have been valid but process check fires first
			ceiling:     TimeTrackingSafe,
			wantErr:     "process mode \"standard\" exceeds ceiling \"time_tracking_safe\"",
		},
		{
			name:        "read_only process+ceiling accepts read_only tenant",
			processMode: ReadOnly,
			tenantMode:  ReadOnly,
			ceiling:     ReadOnly,
			want:        ReadOnly,
		},
		{
			name:        "safe_core process+ceiling accepts safe_core tenant",
			processMode: SafeCore,
			tenantMode:  SafeCore,
			ceiling:     SafeCore,
			want:        SafeCore,
		},
		{
			name:        "standard ceiling rejects full tenant when process is standard",
			processMode: Standard,
			tenantMode:  Full,
			ceiling:     Standard,
			wantErr:     "exceeds",
		},
		{
			name:        "full ceiling accepts full",
			processMode: Full,
			tenantMode:  Full,
			ceiling:     Full,
			want:        Full,
		},
		{
			name:        "tenant cannot widen past process mode even when ceiling is broader",
			processMode: TimeTrackingSafe,
			tenantMode:  SafeCore,
			ceiling:     Full,
			wantErr:     "exceeds",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := EffectiveTenantMode(c.processMode, c.tenantMode, c.ceiling)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil and mode %q", c.wantErr, got)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("EffectiveTenantMode = %q, want %q", got, c.want)
			}
		})
	}
}

// TestFromEnv_RejectsProcessExceedingCeiling pins the config-load
// fail-closed shape. An operator who explicitly overrides
// CLOCKIFY_POLICY to standard while leaving MCP_TENANT_POLICY_CEILING
// at the hosted default time_tracking_safe gets a clear startup
// error rather than silently broadening every tenant past the
// hosted ceiling. ADR 0021.
func TestFromEnv_RejectsProcessExceedingCeiling(t *testing.T) {
	t.Setenv("CLOCKIFY_POLICY", "standard")
	t.Setenv("CLOCKIFY_DENY_TOOLS", "")
	t.Setenv("CLOCKIFY_DENY_GROUPS", "")
	t.Setenv("CLOCKIFY_ALLOW_GROUPS", "")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected error when CLOCKIFY_POLICY exceeds MCP_TENANT_POLICY_CEILING")
	}
	for _, want := range []string{"CLOCKIFY_POLICY", "MCP_TENANT_POLICY_CEILING", "standard", "time_tracking_safe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message should name %q so the operator can act on it; got: %v", want, err)
		}
	}
}

// TestFromEnv_AcceptsProcessEqualToCeiling confirms the common
// hosted shape (both pinned to time_tracking_safe) is accepted.
func TestFromEnv_AcceptsProcessEqualToCeiling(t *testing.T) {
	t.Setenv("CLOCKIFY_POLICY", "time_tracking_safe")
	t.Setenv("CLOCKIFY_DENY_TOOLS", "")
	t.Setenv("CLOCKIFY_DENY_GROUPS", "")
	t.Setenv("CLOCKIFY_ALLOW_GROUPS", "")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	p, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if p.Mode != TimeTrackingSafe {
		t.Errorf("Mode = %q, want time_tracking_safe", p.Mode)
	}
	if p.Ceiling != TimeTrackingSafe {
		t.Errorf("Ceiling = %q, want time_tracking_safe", p.Ceiling)
	}
}

// TestFromEnv_AcceptsCeilingBroaderThanProcess confirms the
// inverse — an operator pinning a broader ceiling than the
// process is harmless because the tenant cannot exceed process
// anyway (the process is its own implicit ceiling).
func TestFromEnv_AcceptsCeilingBroaderThanProcess(t *testing.T) {
	t.Setenv("CLOCKIFY_POLICY", "time_tracking_safe")
	t.Setenv("CLOCKIFY_DENY_TOOLS", "")
	t.Setenv("CLOCKIFY_DENY_GROUPS", "")
	t.Setenv("CLOCKIFY_ALLOW_GROUPS", "")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "standard")

	p, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if p.Mode != TimeTrackingSafe {
		t.Errorf("Mode = %q, want time_tracking_safe", p.Mode)
	}
	if p.Ceiling != Standard {
		t.Errorf("Ceiling = %q, want standard", p.Ceiling)
	}
}

// TestDescribe_CeilingFields pins the three ceiling-related keys
// surfaced through clockify_policy_info (PR #99 review). The
// effective_ceiling key must reflect the implicit-process-mode
// fallback when MCP_TENANT_POLICY_CEILING is unset, so operators
// can tell from policy_info what the live cap actually is. ADR 0021.
func TestDescribe_CeilingFields(t *testing.T) {
	t.Run("explicit ceiling", func(t *testing.T) {
		p := &Policy{Mode: TimeTrackingSafe, Ceiling: TimeTrackingSafe}
		d := p.Describe()
		if got := d["configured_ceiling"]; got != "time_tracking_safe" {
			t.Errorf("configured_ceiling = %v, want time_tracking_safe", got)
		}
		if got := d["effective_ceiling"]; got != "time_tracking_safe" {
			t.Errorf("effective_ceiling = %v, want time_tracking_safe", got)
		}
		if got := d["ceiling_source"]; got != "explicit" {
			t.Errorf("ceiling_source = %v, want explicit", got)
		}
	})

	t.Run("implicit ceiling = process mode", func(t *testing.T) {
		p := &Policy{Mode: SafeCore, Ceiling: ""}
		d := p.Describe()
		if got := d["configured_ceiling"]; got != "" {
			t.Errorf("configured_ceiling = %v, want empty", got)
		}
		if got := d["effective_ceiling"]; got != "safe_core" {
			t.Errorf("effective_ceiling = %v, want safe_core (process mode fallback)", got)
		}
		if got := d["ceiling_source"]; got != "implicit_process_mode" {
			t.Errorf("ceiling_source = %v, want implicit_process_mode", got)
		}
	})

	t.Run("explicit ceiling broader than process still distinct", func(t *testing.T) {
		p := &Policy{Mode: TimeTrackingSafe, Ceiling: Standard}
		d := p.Describe()
		if got := d["configured_ceiling"]; got != "standard" {
			t.Errorf("configured_ceiling = %v, want standard", got)
		}
		// effective_ceiling reports the operator-set value; the
		// runtime-effective tenant cap is min(process, ceiling) but
		// that's a tenant-scoped derivation. policy_info exposes the
		// process-level configured/effective ceiling pair.
		if got := d["effective_ceiling"]; got != "standard" {
			t.Errorf("effective_ceiling = %v, want standard", got)
		}
		if got := d["ceiling_source"]; got != "explicit" {
			t.Errorf("ceiling_source = %v, want explicit", got)
		}
	})

	t.Run("deprecated ceiling key no longer present", func(t *testing.T) {
		p := &Policy{Mode: TimeTrackingSafe, Ceiling: TimeTrackingSafe}
		d := p.Describe()
		if _, ok := d["ceiling"]; ok {
			t.Error("deprecated 'ceiling' key must be replaced by configured_ceiling/effective_ceiling/ceiling_source")
		}
	})
}

// TestIsGroupBlockingMode pins which modes nullify AllowGroups.
// tenantRuntime relies on this to decide whether to honour or
// silently drop the tenant AllowGroups list.
func TestIsGroupBlockingMode(t *testing.T) {
	cases := []struct {
		mode Mode
		want bool
	}{
		{ReadOnly, true},
		{TimeTrackingSafe, true},
		{SafeCore, true},
		{Standard, false},
		{Full, false},
		{Mode(""), true}, // unknown / empty fails closed (block groups)
		{Mode("bogus"), true},
	}
	for _, c := range cases {
		if got := IsGroupBlockingMode(c.mode); got != c.want {
			t.Errorf("IsGroupBlockingMode(%q) = %v, want %v", c.mode, got, c.want)
		}
	}
}
