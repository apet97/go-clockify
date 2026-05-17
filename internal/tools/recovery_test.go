package tools

import (
	"fmt"
	"strings"
	"testing"
)

// TestDefaultRecoveryRoutesScheduleWorkToProjects asserts the schedule_work
// workflow tool routes recovery to clockify_projects_list — listing existing
// assignments cannot resolve the invalid project/user ID that made the call
// fail. The scheduling domain tools still route to the assignments list.
// [audit Item 4]
func TestDefaultRecoveryRoutesScheduleWorkToProjects(t *testing.T) {
	if got := defaultRecovery("clockify_schedule_work", nil).Tool; got != "clockify_projects_list" {
		t.Errorf("clockify_schedule_work recovery Tool = %q, want clockify_projects_list", got)
	}
	if got := defaultRecovery("clockify_scheduling_assignments_create", nil).Tool; got != "clockify_scheduling_assignments_list" {
		t.Errorf("scheduling domain recovery Tool = %q, want clockify_scheduling_assignments_list", got)
	}
}

func TestProjectlessTimerRecoveryPointsAtProjectsList(t *testing.T) {
	te := recoverable("clockify_entries_timer_start",
		fmt.Errorf("this workspace requires a project on every time entry — pass project_id or project"),
		RecoveryHint{})
	if te.Recovery.Tool != "clockify_projects_list" {
		t.Fatalf("recovery tool = %q, want clockify_projects_list", te.Recovery.Tool)
	}
}

// TestDefaultRecoveryCoversAuditTools asserts the audit-log and entity-changes
// tools get domain-specific recovery hints instead of the generic
// clockify_status fallback. [audit N-14]
func TestDefaultRecoveryCoversAuditTools(t *testing.T) {
	audit := defaultRecovery("clockify_audit_logs_search", nil)
	if audit.Tool != "clockify_users_list" {
		t.Errorf("audit-log recovery Tool = %q, want clockify_users_list", audit.Tool)
	}
	if audit.Hint == "" {
		t.Error("audit-log recovery hint is empty")
	}
	entity := defaultRecovery("clockify_entity_changes_list", nil)
	if entity.Hint == "" {
		t.Error("entity-changes recovery hint is empty")
	}
}

// TestLooksLikeFeatureUnavailableMatchesRoutingLayerGate asserts Clockify's
// routing-layer "No static resource" 404 — returned for unprovisioned
// features such as scheduling — is recognised as feature-unavailable so the
// caller gets a structured degrade hint, not an opaque error. [audit P3-2]
func TestLooksLikeFeatureUnavailableMatchesRoutingLayerGate(t *testing.T) {
	cases := []struct {
		message string
		want    bool
	}{
		{"No static resource v1/workspaces/abc/scheduling/assignments/all.", true},
		{"this feature is not supported on the current plan", true},
		{"upgrade your subscription", true},
		{"entry not found", false},
		{"invalid request", false},
	}
	for _, tc := range cases {
		if got := looksLikeFeatureUnavailable(tc.message); got != tc.want {
			t.Errorf("looksLikeFeatureUnavailable(%q) = %v, want %v", tc.message, got, tc.want)
		}
	}
}

// TestDefaultRecoveryCoversReportTools asserts report tools get a report-specific
// recovery hint instead of the generic clockify_status fallback. [audit P1-4]
func TestDefaultRecoveryCoversReportTools(t *testing.T) {
	for _, action := range []string{
		"clockify_reports_money",
		"clockify_reports_export",
		"clockify_reports_attendance",
		"clockify_reports_weekly",
	} {
		got := defaultRecovery(action, nil)
		if got.Tool != "clockify_reports_summary" {
			t.Errorf("%s recovery Tool = %q, want clockify_reports_summary", action, got.Tool)
		}
		if !strings.Contains(got.Hint, "date range") {
			t.Errorf("%s recovery hint should mention the date range, got %q", action, got.Hint)
		}
	}
}

// TestDefaultRecoverySchedulingMentionsRole asserts scheduling-write recovery
// hints explain the 403 role/publish precondition. [audit P2-6]
func TestDefaultRecoverySchedulingMentionsRole(t *testing.T) {
	for _, action := range []string{"clockify_schedule_work", "clockify_scheduling_assignments_create"} {
		got := defaultRecovery(action, nil)
		if !strings.Contains(got.Hint, "403") {
			t.Errorf("%s recovery hint should mention the 403 precondition, got %q", action, got.Hint)
		}
	}
}
