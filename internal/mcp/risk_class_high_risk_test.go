package mcp

import "testing"

// TestRiskClassIsHighRisk pins the RiskHighMask membership used by
// the confirmation-token gate per docs/adr/0018-risk-class-confirmation-tokens.md.
// Ordinary RiskWrite stays confirmation-free so the time-tracking
// surface is not slowed down. The five high-risk bits each pass the
// check; combined bits and bits AND'd with RiskRead/RiskWrite also
// pass when any high-risk bit is present.
func TestRiskClassIsHighRisk(t *testing.T) {
	cases := []struct {
		name string
		rc   RiskClass
		want bool
	}{
		{"zero", 0, false},
		{"read only", RiskRead, false},
		{"write only", RiskWrite, false},
		{"read + write", RiskRead | RiskWrite, false},
		{"billing", RiskBilling, true},
		{"admin", RiskAdmin, true},
		{"permission change", RiskPermissionChange, true},
		{"external side effect", RiskExternalSideEffect, true},
		{"destructive", RiskDestructive, true},
		{"write + billing", RiskWrite | RiskBilling, true},
		{"destructive + admin", RiskDestructive | RiskAdmin, true},
		{"all bits", RiskRead | RiskWrite | RiskBilling | RiskAdmin | RiskPermissionChange | RiskExternalSideEffect | RiskDestructive, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rc.IsHighRisk(); got != c.want {
				t.Errorf("(%b).IsHighRisk() = %v, want %v", c.rc, got, c.want)
			}
		})
	}
}

// TestRiskHighMaskValue records the literal bitmask so a future
// refactor that adds a new high-risk bit either updates this test
// (deliberate) or breaks it (loud).
func TestRiskHighMaskValue(t *testing.T) {
	want := RiskBilling | RiskAdmin | RiskPermissionChange | RiskExternalSideEffect | RiskDestructive
	if RiskHighMask != want {
		t.Fatalf("RiskHighMask = %b, want %b — review docs/adr/0018-risk-class-confirmation-tokens.md before changing", RiskHighMask, want)
	}
}
