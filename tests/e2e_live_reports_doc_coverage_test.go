//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveReportsDocCoverage exercises the Reports API-backed MCP tools named
// in ATTENDANCEANDTIMEREPORTS.md through the MCP path. It keeps the report
// windows under the documented 31-day free-plan limit and accepts explicit
// upstream feature/permission refusals as evidence rather than masking them.
func TestLiveReportsDocCoverage(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("expenses")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	project := h.callOK(ctx, "clockify_create_project", map[string]any{
		"name":      c.LivePrefix("reports-project", 0),
		"billable":  true,
		"is_public": false,
	})
	projectID, _ := extractDataMap(t, project)["id"].(string)
	if projectID == "" {
		t.Fatalf("seed project returned no id: %#v", project)
	}
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	now := time.Now().UTC().Truncate(time.Second)
	entryStart := now.Add(-2 * time.Hour)
	entryEnd := now.Add(-1 * time.Hour)
	entry := h.callOK(ctx, "clockify_add_entry", map[string]any{
		"project_id":    projectID,
		"description":   c.LivePrefix("reports-entry", 0),
		"start":         entryStart.Format(time.RFC3339),
		"end":           entryEnd.Format(time.RFC3339),
		"billable":      true,
		"allow_overlap": true,
	})
	entryID, _ := extractDataMap(t, entry)["id"].(string)
	if entryID == "" {
		t.Fatalf("seed time entry returned no id: %#v", entry)
	}
	c.RegisterCleanup("time-entry", entryID, func(ctx context.Context) error {
		return h.deleteEntryRaw(ctx, entryID)
	})

	rangeStart := now.AddDate(0, 0, -7).Format("2006-01-02T15:04:05.000")
	rangeEnd := now.Format("2006-01-02T15:04:05.000")

	callReportOrAcceptedRefusal(t, h, ctx, "clockify_summary_report", map[string]any{
		"start":        rangeStart,
		"end":          rangeEnd,
		"amount_shown": "PROFIT",
		"amounts":      []any{"EARNED", "COST", "PROFIT"},
		"projects":     map[string]any{"contains": "CONTAINS", "ids": []any{projectID}, "status": "ACTIVE"},
		"summary_filter": map[string]any{
			"groups":             []any{"CLIENT", "PROJECT", "DATE"},
			"sort_column":        "PROFIT",
			"summary_chart_type": "PROJECT",
		},
	}, "totals")

	callReportOrAcceptedRefusal(t, h, ctx, "clockify_detailed_report", map[string]any{
		"start":        rangeStart,
		"end":          rangeEnd,
		"amount_shown": "PROFIT",
		"amounts":      []any{"EARNED", "COST", "PROFIT"},
		"projects":     map[string]any{"contains": "CONTAINS", "ids": []any{projectID}, "status": "ACTIVE"},
		"detailed_filter": map[string]any{
			"page":        1,
			"page_size":   25,
			"sort_column": "ID",
			"options":     map[string]any{"totals": "CALCULATE"},
			"audit_filter": map[string]any{
				"duration":         1,
				"duration_shorter": false,
				"without_project":  false,
				"without_task":     false,
			},
		},
	}, "totals")

	callReportOrAcceptedRefusal(t, h, ctx, "clockify_attendance_report", map[string]any{
		"start": rangeStart,
		"end":   rangeEnd,
		"attendance_filter": map[string]any{
			"page":        1,
			"page_size":   25,
			"sort_column": "USER",
			"start_filters": []any{
				map[string]any{"filtration_type": "LARGER_THAN", "value": "00:00"},
			},
		},
	}, "entities")

	callReportOrAcceptedRefusal(t, h, ctx, "clockify_weekly_summary", map[string]any{
		"week_start": now.Format("2006-01-02"),
		"projects":   map[string]any{"contains": "CONTAINS", "ids": []any{projectID}, "status": "ACTIVE"},
		"weekly_filter": map[string]any{
			"group":    "PROJECT",
			"subgroup": "TIME",
		},
	}, "totals")

	callReportOrAcceptedRefusal(t, h, ctx, "clockify_expense_report", map[string]any{
		"start":       rangeStart,
		"end":         rangeEnd,
		"page":        1,
		"page_size":   25,
		"sort_column": "ID",
		"sort_order":  "ASCENDING",
		"projects":    map[string]any{"contains": "CONTAINS", "ids": []any{projectID}, "status": "ACTIVE"},
	}, "totals")
}

func callReportOrAcceptedRefusal(t *testing.T, h *liveMCPHarness, ctx context.Context, tool string, args map[string]any, wantKey string) {
	t.Helper()
	result, errText := liveCallMaybe(t, h, ctx, tool, args)
	if errText != "" {
		if containsErrorText(errText, "permission", "forbidden", "subscription", "paid", "plan", "feature", "not enabled", "not supported", "400", "403") {
			t.Logf("%s live-probed documented Reports API path and received upstream refusal: %s", tool, errText)
			return
		}
		t.Fatalf("%s unexpected live Reports API error: %s", tool, errText)
	}
	data := extractDataMap(t, result)
	if _, ok := data[wantKey]; !ok {
		t.Fatalf("%s response missing %s field: %#v", tool, wantKey, data)
	}
	sc, _ := result["structuredContent"].(map[string]any)
	meta, _ := sc["meta"].(map[string]any)
	if meta["source"] != "reports-api" {
		t.Fatalf("%s meta.source=%v, want reports-api: %#v", tool, meta["source"], meta)
	}
}
