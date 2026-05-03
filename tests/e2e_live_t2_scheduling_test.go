//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveT2SchedulingRecurringCRUD covers the scheduling assignment
// write surface against the sacrificial workspace. Clockify's live API
// exposes assignment CRUD under /scheduling/assignments/recurring; the
// non-recurring /scheduling/assignments/{id} route used by earlier
// handler revisions is not a live CRUD endpoint.
func TestLiveT2SchedulingRecurringCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("scheduling")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	projectName := c.LivePrefix("sched", 0)
	project := h.callOK(ctx, "clockify_create_project", map[string]any{
		"name": projectName,
	})
	projectID, _ := extractDataMap(t, project)["id"].(string)
	if projectID == "" {
		t.Fatalf("seed project returned no id: %#v", project)
	}
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	start := time.Now().UTC().AddDate(0, 0, 14).Truncate(time.Second)
	end := start.Add(24 * time.Hour)
	startText := start.Format("2006-01-02T15:04:05Z")
	endText := end.Format("2006-01-02T15:04:05Z")

	create := h.callOK(ctx, "clockify_create_assignment", map[string]any{
		"user_id":       c.OwnerUserID,
		"project_id":    projectID,
		"start":         startText,
		"end":           endText,
		"hours_per_day": 1.0,
		"repeat":        true,
		"weeks":         1,
		"note":          c.LivePrefix("assignment", 0),
	})
	assignments := extractList(t, create)
	if len(assignments) == 0 {
		t.Fatalf("create_assignment returned no assignments: %#v", create)
	}
	first, ok := assignments[0].(map[string]any)
	if !ok {
		t.Fatalf("create_assignment first item is not an object: %T", assignments[0])
	}
	assignmentID, _ := first["id"].(string)
	if assignmentID == "" {
		t.Fatalf("create_assignment returned no id: %#v", first)
	}
	deleted := false
	c.RegisterCleanup("assignment", assignmentID, func(ctx context.Context) error {
		if deleted {
			return nil
		}
		_, err := h.rawCall(ctx, "clockify_delete_assignment", map[string]any{
			"assignment_id":        assignmentID,
			"series_update_option": "ALL",
		})
		return err
	})

	got := h.callOK(ctx, "clockify_get_assignment", map[string]any{
		"assignment_id": assignmentID,
		"start":         startText,
		"end":           endText,
	})
	gotData := extractDataMap(t, got)
	if gotID, _ := gotData["id"].(string); gotID != assignmentID {
		t.Fatalf("get_assignment id mismatch: got %q want %q; data=%#v", gotID, assignmentID, gotData)
	}

	updateEnd := end.Add(24 * time.Hour).Format("2006-01-02T15:04:05Z")
	updated := h.callOK(ctx, "clockify_update_assignment", map[string]any{
		"assignment_id":        assignmentID,
		"start":                startText,
		"end":                  updateEnd,
		"hours_per_day":        2.0,
		"note":                 c.LivePrefix("assignment", 1),
		"series_update_option": "ALL",
	})
	sc, ok := updated["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("update_assignment missing structuredContent: %#v", updated)
	}
	if okFlag, _ := sc["ok"].(bool); !okFlag {
		t.Fatalf("update_assignment carried ok=false: %#v", sc)
	}

	_ = h.callOK(ctx, "clockify_delete_assignment", map[string]any{
		"assignment_id":        assignmentID,
		"series_update_option": "ALL",
	})
	deleted = true
}
