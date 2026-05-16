package tools

import (
	"testing"
	"time"
)

// TestSchedulingRangeArgsRejectsInvertedRange locks P2-1: schedulingRangeArgs
// rejects end <= start instead of silently forwarding a backwards window to
// the Clockify scheduling API.
func TestSchedulingRangeArgsRejectsInvertedRange(t *testing.T) {
	if _, _, err := schedulingRangeArgs(map[string]any{
		"start": "2026-05-10T09:00:00Z",
		"end":   "2026-05-01T09:00:00Z",
	}, time.UTC); err == nil {
		t.Fatal("expected an error when end precedes start")
	}
	start, end, err := schedulingRangeArgs(map[string]any{
		"start": "2026-05-01T09:00:00Z",
		"end":   "2026-05-10T17:00:00Z",
	}, time.UTC)
	if err != nil {
		t.Fatalf("valid range rejected: %v", err)
	}
	if start != "2026-05-01T09:00:00Z" || end != "2026-05-10T17:00:00Z" {
		t.Fatalf("range round-trip = %q..%q", start, end)
	}
}

// TestAddRecurringAssignmentClampsAndReportsWeeks locks N-9: a missing or
// sub-1 weeks value defaults to 1, and the applied interval is returned so
// the handler can echo weeksApplied.
func TestAddRecurringAssignmentClampsAndReportsWeeks(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want int
	}{
		{"explicit", map[string]any{"weeks": 4}, 4},
		{"missing", map[string]any{}, 1},
		{"zero", map[string]any{"weeks": 0}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{}
			if got := addRecurringAssignment(payload, tc.args); got != tc.want {
				t.Errorf("addRecurringAssignment = %d, want %d", got, tc.want)
			}
			recurring, _ := payload["recurringAssignment"].(map[string]any)
			if recurring["weeks"] != tc.want {
				t.Errorf("payload recurringAssignment.weeks = %v, want %d", recurring["weeks"], tc.want)
			}
		})
	}
}
