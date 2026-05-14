//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"
)

// TestLiveOneUserReadOnly exercises current one-user read-only tools through
// the same MCP harness used by the executable path. Empty workspaces are
// handled by checking shape rather than count whenever the result depends on
// whether the workspace has a given entity.
func TestLiveOneUserReadOnly(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	t.Run("pinned_workspace_status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_status", nil)
		data := extractDataMap(t, result)
		workspace, _ := data["pinnedWorkspace"].(map[string]any)
		if id, _ := workspace["id"].(string); id != c.WorkspaceID {
			t.Fatalf("pinned workspace id=%q expected %q", id, c.WorkspaceID)
		}
	})

	t.Run("current_user", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_users_profile", nil)
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id != c.OwnerUserID {
			t.Fatalf("current_user.id=%q expected %q (the workspace-owner identity captured by setup)", id, c.OwnerUserID)
		}
	})

	t.Run("list_users", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_users_list", nil)
		users := extractList(t, result)
		if len(users) == 0 {
			t.Fatalf("expected at least 1 user (the workspace owner) in list_users, got 0")
		}
	})

	t.Run("list_tags", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_tags_list", nil)
		// list_tags returns a slice — empty workspace is allowed.
		_ = extractList(t, result)
	})

	t.Run("list_tasks", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// list_tasks needs a project; pick the first one if any exist.
		// An empty workspace skips this subtest cleanly — the production
		// path is exercised the moment any project is created later in
		// the campaign (Phase 2+ creates projects under category gates).
		projects := h.listProjectsRaw(ctx)
		if len(projects) == 0 {
			t.Skip("no projects in workspace; list_tasks needs a project to query")
		}
		result := h.callOK(ctx, "clockify_tasks_list", map[string]any{
			"project_id": projects[0].ID,
		})
		_ = extractList(t, result)
	})

	t.Run("today_entries", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		now := time.Now().UTC()
		result := h.callOK(ctx, "clockify_entries_list", map[string]any{
			"start": now.Format("2006-01-02"),
			"end":   now.Add(24 * time.Hour).Format("2006-01-02"),
		})
		_ = extractList(t, result)
	})

	t.Run("summary_report", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)
		result := h.callOK(ctx, "clockify_reports_summary", map[string]any{
			"start": start.Format("2006-01-02T15:04:05.000"),
			"end":   end.Format("2006-01-02T15:04:05.000"),
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("summary_report response missing totals field: %#v", data)
		}
	})

	t.Run("weekly_summary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_reports_weekly", map[string]any{
			"week_start": time.Now().UTC().Format("2006-01-02"),
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("weekly_summary response missing totals field: %#v", data)
		}
	})

	t.Run("review_day", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_review_day", map[string]any{
			"date":            time.Now().UTC().Format("2006-01-02"),
			"timezone":        "UTC",
			"min_gap_minutes": 0,
		})
		data := extractDataMap(t, result)
		if _, ok := data["issues"]; !ok {
			t.Fatalf("review_day response missing issues field: %#v", data)
		}
		if _, ok := data["suggestedActions"]; !ok {
			t.Fatalf("review_day response missing suggestedActions field: %#v", data)
		}
	})

	t.Run("status_orientation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_status", nil)
		data := extractDataMap(t, result)
		if _, ok := data["recommendedFirstTools"]; !ok {
			t.Fatalf("status response missing recommendedFirstTools field: %#v", data)
		}
	})

	t.Run("detailed_report", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)
		result := h.callOK(ctx, "clockify_reports_detailed", map[string]any{
			"start": start.Format("2006-01-02T15:04:05.000"),
			"end":   end.Format("2006-01-02T15:04:05.000"),
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("detailed_report response missing totals field: %#v", data)
		}
	})

	t.Run("tools_guide", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_tools_guide", nil)
		data := extractDataMap(t, result)
		if _, ok := data["workflows"]; !ok {
			t.Fatalf("tools_guide response missing workflows: %#v", data)
		}
	})

	t.Run("status_feature_status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_status", nil)
		data := extractDataMap(t, result)
		if _, ok := data["featureStatus"]; !ok {
			t.Fatalf("status response missing featureStatus: %#v", data)
		}
	})
}
