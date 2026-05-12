//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveTier1RemainingCRUD covers Tier-1 MCP tools that were not part
// of the original live campaign. Every mutating call creates only
// prefix-owned sacrificial entities and registers raw cleanup.
func TestLiveTier1RemainingCRUD(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Standard})
	c := setupLiveCampaign(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	projectName := c.LivePrefix("tier1-proj", 0)
	project := h.callOK(ctx, "clockify_create_project", map[string]any{"name": projectName})
	projectID, _ := extractDataMap(t, project)["id"].(string)
	if projectID == "" {
		t.Fatalf("clockify_create_project returned no id: %#v", project)
	}
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	t.Run("read_remaining_lists_and_getters", func(t *testing.T) {
		_ = h.callOK(ctx, "clockify_get_project", map[string]any{"project": projectID})
		_ = h.callOK(ctx, "clockify_list_clients", map[string]any{"page": 1, "page_size": 5})
		_ = h.callOK(ctx, "clockify_list_entries", map[string]any{
			"start":     time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339),
			"end":       time.Now().UTC().AddDate(0, 0, 1).Format(time.RFC3339),
			"page_size": 10,
		})
		_ = h.callOK(ctx, "clockify_search_tools", map[string]any{"query": "project"})
	})

	t.Run("create_task", func(t *testing.T) {
		result := h.callOK(ctx, "clockify_create_task", map[string]any{
			"project": projectID,
			"name":    c.LivePrefix("task", 0),
		})
		if id, _ := extractDataMap(t, result)["id"].(string); id == "" {
			t.Fatalf("clockify_create_task returned no id: %#v", result)
		}
	})

	start := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Second)
	entryStart := start.Format(time.RFC3339)
	entryEnd := start.Add(30 * time.Minute).Format(time.RFC3339)

	t.Run("log_update_find_and_update_entry", func(t *testing.T) {
		result := h.callOK(ctx, "clockify_log_time", map[string]any{
			"project_id":  projectID,
			"description": c.LivePrefix("log", 0),
			"start":       entryStart,
			"end":         entryEnd,
			"billable":    false,
		})
		entryID := liveEntryIDFromResult(t, result)
		c.RegisterCleanup("entry", entryID, func(ctx context.Context) error {
			return h.deleteEntryRaw(ctx, entryID)
		})

		update := h.callOK(ctx, "clockify_update_entry", map[string]any{
			"entry_id":    entryID,
			"description": c.LivePrefix("update", 0),
			"billable":    true,
		})
		if got := liveEntryIDFromResult(t, update); got != entryID {
			t.Fatalf("clockify_update_entry id mismatch: got %q want %q", got, entryID)
		}

		find := h.callOK(ctx, "clockify_find_and_update_entry", map[string]any{
			"entry_id":        entryID,
			"new_description": c.LivePrefix("find-update", 0),
			"billable":        false,
		})
		if got := liveEntryIDFromResult(t, find); got != entryID {
			t.Fatalf("clockify_find_and_update_entry id mismatch: got %q want %q", got, entryID)
		}
	})

	t.Run("timesheet_fill_gap", func(t *testing.T) {
		gapStartTime, gapEndTime := findLiveEmptyEntryWindow(t, h, ctx, 30*time.Minute)
		result := h.callOK(ctx, "clockify_timesheet_fill_gap", map[string]any{
			"project_id":  projectID,
			"description": c.LivePrefix("fill-gap", 0),
			"start":       gapStartTime.Format(time.RFC3339),
			"end":         gapEndTime.Format(time.RFC3339),
			"billable":    false,
		})
		entryID := liveNestedEntryID(t, result, "entry")
		if entryID == "" {
			t.Fatalf("clockify_timesheet_fill_gap returned no created entry id: %#v", result)
		}
		c.RegisterCleanup("entry", entryID, func(ctx context.Context) error {
			return h.deleteEntryRaw(ctx, entryID)
		})
	})

	t.Run("switch_project_starts_timer_then_stop_cleanup", func(t *testing.T) {
		result := h.callOK(ctx, "clockify_switch_project", map[string]any{
			"project":     projectID,
			"description": c.LivePrefix("switch", 0),
		})
		startedID := liveNestedEntryID(t, result, "started")
		if startedID == "" {
			t.Fatalf("clockify_switch_project did not expose started entry id: %#v", result)
		}
		c.RegisterCleanup("entry", startedID, func(ctx context.Context) error {
			return h.deleteEntryRaw(ctx, startedID)
		})

		stopped := h.callOK(ctx, "clockify_stop_timer", nil)
		stoppedID := liveEntryIDFromResult(t, stopped)
		if stoppedID != startedID {
			t.Fatalf("clockify_stop_timer id mismatch after switch: got %q want %q", stoppedID, startedID)
		}
	})
}

func findLiveEmptyEntryWindow(t *testing.T, h *liveMCPHarness, ctx context.Context, duration time.Duration) (time.Time, time.Time) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	for daysAgo := 30; daysAgo >= 2; daysAgo-- {
		day := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo)
		for hour := 0; hour <= 10; hour += 2 {
			start := day.Add(time.Duration(hour) * time.Hour)
			end := start.Add(duration)
			result := h.callOK(ctx, "clockify_list_entries", map[string]any{
				"start":     start.Format(time.RFC3339),
				"end":       end.Format(time.RFC3339),
				"page_size": 50,
			})
			if len(extractList(t, result)) == 0 {
				return start, end
			}
		}
	}
	t.Skip("could not find an empty sacrificial-workspace time-entry window in the last 30 days")
	return time.Time{}, time.Time{}
}

func liveEntryIDFromResult(t *testing.T, result map[string]any) string {
	t.Helper()
	data := extractDataMap(t, result)
	if id, _ := data["id"].(string); id != "" {
		return id
	}
	if entry, ok := data["entry"].(map[string]any); ok {
		if id, _ := entry["id"].(string); id != "" {
			return id
		}
	}
	t.Fatalf("result did not contain an entry id: %#v", result)
	return ""
}

func liveNestedEntryID(t *testing.T, result map[string]any, field string) string {
	t.Helper()
	data := extractDataMap(t, result)
	nested, ok := data[field].(map[string]any)
	if !ok {
		return ""
	}
	if id, _ := nested["id"].(string); id != "" {
		return id
	}
	if entry, ok := nested["entry"].(map[string]any); ok {
		id, _ := entry["id"].(string)
		return id
	}
	return ""
}
