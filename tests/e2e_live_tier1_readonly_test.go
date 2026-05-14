//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"
)

// TestLiveTier1ReadOnly exercises the Tier-1 read-only tools that had
// no live coverage in the original baseline (docs/api-coverage.md only
// counted only the 3 read-only and a handful of mutating Tier-1 tools
// covered by the original e2e_live_test.go suite).
//
// Each subtest goes through the production-shaped MCP enforcement
// pipeline (setupLiveMCPHarness wires policy + dry-run + audit), so a
// regression in the protocol layer surfaces here even when the
// underlying clockify package is unchanged. Empty workspaces are
// handled by checking shape rather than count whenever the tool's
// result depends on whether the workspace has any of a given entity.
func TestLiveTier1ReadOnly(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	t.Run("list_workspaces", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_list_workspaces", nil)
		ws := extractList(t, result)
		if len(ws) == 0 {
			t.Fatalf("expected the API key to grant access to at least one workspace, got 0")
		}
		// The sacrificial workspace must be in the list. We probe by ID
		// so a renamed workspace doesn't cause a flaky failure.
		var found bool
		for _, item := range ws {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := entry["id"].(string); id == c.WorkspaceID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sacrificial workspace %s not visible to the configured API key (got %d workspaces)", c.WorkspaceID, len(ws))
		}
	})

	t.Run("current_user", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_current_user", nil)
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id != c.OwnerUserID {
			t.Fatalf("current_user.id=%q expected %q (the workspace-owner identity captured by setup)", id, c.OwnerUserID)
		}
	})

	t.Run("list_users", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_list_users", nil)
		users := extractList(t, result)
		if len(users) == 0 {
			t.Fatalf("expected at least 1 user (the workspace owner) in list_users, got 0")
		}
	})

	t.Run("list_tags", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_list_tags", nil)
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
		result := h.callOK(ctx, "clockify_list_tasks", map[string]any{
			"project": projects[0].Name,
		})
		_ = extractList(t, result)
	})

	t.Run("today_entries", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_today_entries", nil)
		_ = extractList(t, result)
	})

	t.Run("summary_report", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)
		result := h.callOK(ctx, "clockify_summary_report", map[string]any{
			"start": start.Format("2006-01-02T15:04:05.000"),
			"end":   end.Format("2006-01-02T15:04:05.000"),
			"summary_filter": map[string]any{
				"groups": []any{"CLIENT", "PROJECT", "DATE"},
			},
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("summary_report response missing totals field: %#v", data)
		}
	})

	t.Run("weekly_summary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_weekly_summary", map[string]any{
			"week_start": time.Now().UTC().Format("2006-01-02"),
			"weekly_filter": map[string]any{
				"group":    "PROJECT",
				"subgroup": "TIME",
			},
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("weekly_summary response missing totals field: %#v", data)
		}
	})

	t.Run("attendance_report", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)
		result := h.callOK(ctx, "clockify_attendance_report", map[string]any{
			"start": start.Format("2006-01-02T15:04:05.000"),
			"end":   end.Format("2006-01-02T15:04:05.000"),
			"attendance_filter": map[string]any{
				"page":        1,
				"page_size":   10,
				"sort_column": "USER",
			},
		})
		data := extractDataMap(t, result)
		if _, ok := data["entities"]; !ok {
			t.Fatalf("attendance_report response missing entities field: %#v", data)
		}
	})

	t.Run("quick_report", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_quick_report", map[string]any{"days": 7})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("quick_report response missing totals field: %#v", data)
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
		result := h.callOK(ctx, "clockify_detailed_report", map[string]any{
			"start": start.Format("2006-01-02T15:04:05.000"),
			"end":   end.Format("2006-01-02T15:04:05.000"),
			"detailed_filter": map[string]any{
				"page":        1,
				"page_size":   10,
				"sort_column": "ID",
				"options":     map[string]any{"totals": "CALCULATE"},
			},
		})
		data := extractDataMap(t, result)
		if _, ok := data["totals"]; !ok {
			t.Fatalf("detailed_report response missing totals field: %#v", data)
		}
	})

	t.Run("resolve_debug", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Probe a name that almost certainly does not exist — the run-id
		// prefix itself is unique to this campaign and never used as a
		// project name. resolve_debug must surface "no match" cleanly
		// instead of panicking or returning a malformed envelope.
		probe := c.LivePrefix("resolve-probe", 0)
		result := h.callOK(ctx, "clockify_resolve_debug", map[string]any{
			"entity_type": "project",
			"name_or_id":  probe,
		})
		data := extractDataMap(t, result)
		// matches should be a list (possibly empty) — the field name
		// is set by ResolveDebug. Tolerate either matches:[] or
		// candidates:[] since the handler may evolve, but at minimum
		// the response must not be empty.
		if len(data) == 0 {
			t.Fatalf("resolve_debug returned empty data envelope for probe %q", probe)
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
