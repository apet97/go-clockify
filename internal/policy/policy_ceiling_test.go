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
			name:        "empty tenant inherits even when ceiling is tighter",
			processMode: SafeCore,
			tenantMode:  "",
			ceiling:     ReadOnly,
			want:        SafeCore, // process mode wins; the ceiling only constrains tenant overrides
		},
		{
			name:        "tenant narrowing under explicit ceiling succeeds",
			processMode: SafeCore,
			tenantMode:  TimeTrackingSafe,
			ceiling:     SafeCore,
			want:        TimeTrackingSafe,
		},
		{
			name:        "tenant equal to explicit ceiling succeeds",
			processMode: Standard,
			tenantMode:  TimeTrackingSafe,
			ceiling:     TimeTrackingSafe,
			want:        TimeTrackingSafe,
		},
		{
			name:        "tenant broadening past explicit ceiling fails closed",
			processMode: Standard,
			tenantMode:  Standard,
			ceiling:     TimeTrackingSafe,
			wantErr:     "exceeds",
		},
		{
			name:        "tenant broadening past implicit process ceiling fails closed",
			processMode: TimeTrackingSafe,
			tenantMode:  Standard,
			ceiling:     "",
			wantErr:     "exceeds",
		},
		{
			name:        "tenant broadening to full past standard ceiling fails closed",
			processMode: Full,
			tenantMode:  Full,
			ceiling:     Standard,
			wantErr:     "exceeds",
		},
		{
			name:        "unknown tenant mode fails closed",
			processMode: Standard,
			tenantMode:  Mode("tenant-admin"),
			ceiling:     "",
			wantErr:     "invalid",
		},
		{
			name:        "read_only ceiling rejects time_tracking_safe",
			processMode: Standard,
			tenantMode:  TimeTrackingSafe,
			ceiling:     ReadOnly,
			wantErr:     "exceeds",
		},
		{
			name:        "read_only ceiling accepts read_only",
			processMode: Standard,
			tenantMode:  ReadOnly,
			ceiling:     ReadOnly,
			want:        ReadOnly,
		},
		{
			name:        "safe_core ceiling accepts safe_core",
			processMode: Standard,
			tenantMode:  SafeCore,
			ceiling:     SafeCore,
			want:        SafeCore,
		},
		{
			name:        "safe_core ceiling rejects standard",
			processMode: Standard,
			tenantMode:  Standard,
			ceiling:     SafeCore,
			wantErr:     "exceeds",
		},
		{
			name:        "standard ceiling accepts standard but rejects full",
			processMode: Full,
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
			name:        "ceiling cannot widen process mode (tenant matches process)",
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
