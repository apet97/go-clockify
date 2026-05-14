package tools

import "testing"

func TestReportRouteBodyAliasesStartEnd(t *testing.T) {
	spec := reportRT(1, "clockify_reports_expense", "Run expense report.", "reports/expenses/detailed")

	body := routeBody(spec, map[string]any{
		"start":       "2026-05-01T00:00:00.000",
		"end":         "2026-05-02T00:00:00.000",
		"export_type": "JSON",
	})

	if body["dateRangeStart"] != "2026-05-01T00:00:00.000" || body["dateRangeEnd"] != "2026-05-02T00:00:00.000" {
		t.Fatalf("report start/end aliases not normalized: %#v", body)
	}
	if _, ok := body["start"]; ok {
		t.Fatalf("report body leaked start alias upstream: %#v", body)
	}
	if _, ok := body["end"]; ok {
		t.Fatalf("report body leaked end alias upstream: %#v", body)
	}
	if body["exportType"] != "JSON" {
		t.Fatalf("export_type not normalized: %#v", body)
	}
}

func TestReportRouteBodyPrefersExplicitDateRangeOverAliases(t *testing.T) {
	spec := reportRT(1, "clockify_reports_expense", "Run expense report.", "reports/expenses/detailed")

	body := routeBody(spec, map[string]any{
		"date_range_start": "2026-05-01T00:00:00.000",
		"date_range_end":   "2026-05-02T00:00:00.000",
		"start":            "2026-04-01T00:00:00.000",
		"end":              "2026-04-02T00:00:00.000",
	})

	if body["dateRangeStart"] != "2026-05-01T00:00:00.000" || body["dateRangeEnd"] != "2026-05-02T00:00:00.000" {
		t.Fatalf("explicit report date range should win over aliases: %#v", body)
	}
}

func TestSchedulingCapacityRouteUsesPostBody(t *testing.T) {
	spec := rt(607, "POST", "clockify_scheduling_capacity", "Get workspace capacity totals.", "scheduling/assignments/user-filter/totals", "scheduling", nil, fields("start", "end", "user_ids"), nil, []string{"start", "end", "user_ids"}, true, false, false, "")

	query := routeQuery(spec, map[string]any{
		"start":    "2026-05-01T00:00:00Z",
		"end":      "2026-05-02T00:00:00Z",
		"user_ids": []any{"u1"},
	})
	body := routeBody(spec, map[string]any{
		"start":    "2026-05-01T00:00:00Z",
		"end":      "2026-05-02T00:00:00Z",
		"user_ids": []any{"u1"},
	})

	if query != nil {
		t.Fatalf("capacity route should not send filters as query params: %#v", query)
	}
	if body["start"] != "2026-05-01T00:00:00Z" || body["end"] != "2026-05-02T00:00:00Z" {
		t.Fatalf("capacity route missing date body fields: %#v", body)
	}
	userIDs, ok := body["userIds"].([]any)
	if !ok || len(userIDs) != 1 || userIDs[0] != "u1" {
		t.Fatalf("capacity route missing userIds body field: %#v", body)
	}
}
