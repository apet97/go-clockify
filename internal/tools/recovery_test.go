package tools

import "testing"

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
