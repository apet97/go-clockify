package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceSettingsViewOmitsRawAndSummaryFeatureCopies(t *testing.T) {
	view := workspaceViewFromRaw(map[string]any{
		"id":                      "ws1",
		"name":                    "Workspace",
		"featureSubscriptionType": "PRO",
		"features":                []any{"INVOICES", "EXPENSES"},
		"workspaceSettings": map[string]any{
			"durationFormat": "FULL",
			"workingDays":    []any{"MONDAY", "TUESDAY"},
		},
	})

	b, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal workspace view: %v", err)
	}
	body := string(b)
	if strings.Contains(body, `"raw"`) {
		t.Fatalf("workspace view still contains raw echo: %s", body)
	}
	if len(view.Features) != 2 || len(view.FeaturesHuman) != 2 {
		t.Fatalf("top-level features not preserved: %#v", view)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal workspace view: %v", err)
	}
	summary, _ := decoded["settings_summary"].(map[string]any)
	if _, ok := summary["features"]; ok {
		t.Fatalf("settings summary duplicates features: %#v", summary)
	}
	if _, ok := summary["features_human"]; ok {
		t.Fatalf("settings summary duplicates human features: %#v", summary)
	}
}

func TestTimeOffBalanceViewKeepsOnlyCanonicalNestedBlocks(t *testing.T) {
	view := timeOffBalanceViewFromRaw(map[string]any{
		"policyId":   "policy-1",
		"policyName": "Vacation",
		"userId":     "user-1",
		"userName":   "Ada",
		"available":  40,
		"used":       8,
		"total":      48,
		"timeUnit":   "HOURS",
	})

	if _, ok := view["raw"]; ok {
		t.Fatalf("time-off balance still contains raw echo: %#v", view)
	}
	for _, key := range []string{"policyId", "policyName", "userId", "userName", "available", "used", "total"} {
		if _, ok := view[key]; ok {
			t.Fatalf("time-off balance duplicated %q at top level: %#v", key, view)
		}
	}
	if view["policy"] == nil || view["user"] == nil || view["balance"] == nil {
		t.Fatalf("time-off balance missing canonical nested blocks: %#v", view)
	}
}

func TestSchedulingViewsOmitRawAndDuplicateBlocks(t *testing.T) {
	start := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	assignment := assignmentViewFromRaw(map[string]any{
		"id":          "assign-1",
		"period":      map[string]any{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339)},
		"hoursPerDay": 2,
		"userId":      "user-1",
	}, start, end)
	if _, ok := assignment["assignment"]; ok {
		t.Fatalf("assignment list row still has nested duplicate assignment block: %#v", assignment)
	}

	total := scheduleTotalView(map[string]any{
		"userId":           "user-1",
		"userName":         "Ada",
		"userStatus":       "ACTIVE",
		"user_status":      "ACTIVE",
		"userImage":        "https://example.test/avatar.png",
		"workingDays":      []any{"MONDAY"},
		"totalHoursPerDay": []map[string]any{{"date": "2026-05-17", "hours": 2}},
		"assignments":      []map[string]any{{"date": "2026-05-17", "hasAssignment": true}},
	}, "USER")

	for _, key := range []string{"raw", "totalHoursPerDay", "total_hours_per_day", "userStatus", "userImage", "workingDays"} {
		if _, ok := total[key]; ok {
			t.Fatalf("schedule total duplicated %q at top level: %#v", key, total)
		}
	}
	if total["total_hours_by_day"] == nil || total["user_status"] == nil || total["user_image"] == nil || total["working_days"] == nil {
		t.Fatalf("schedule total missing canonical fields: %#v", total)
	}
	for _, row := range total["total_hours_by_day"].([]map[string]any) {
		if _, ok := row["raw"]; ok {
			t.Fatalf("schedule day duration still contains raw echo: %#v", row)
		}
	}
	for _, row := range total["project_heatmap"].([]map[string]any) {
		if _, ok := row["raw"]; ok {
			t.Fatalf("schedule heatmap row still contains raw echo: %#v", row)
		}
	}
}

func TestEntryCustomFieldsNormalizeWithoutRawOrDefinitionAndListOmitsNulls(t *testing.T) {
	fields := customFieldValuesFromRaw([]any{map[string]any{
		"id":    "value-1",
		"value": "EU",
		"customField": map[string]any{
			"id":         "cf-1",
			"name":       "Region",
			"type":       "TXT",
			"status":     "ACTIVE",
			"entityType": "TIMEENTRY",
		},
	}})
	if len(fields) != 1 {
		t.Fatalf("expected one custom field, got %#v", fields)
	}
	b, err := json.Marshal(fields[0])
	if err != nil {
		t.Fatalf("marshal custom field: %v", err)
	}
	body := string(b)
	if strings.Contains(body, `"raw"`) || strings.Contains(body, `"definition"`) {
		t.Fatalf("custom field normalization still echoes raw/definition: %s", body)
	}
	if fields[0].CustomFieldID != "cf-1" || fields[0].Name != "Region" || fields[0].Type != "TXT" || fields[0].Value != "EU" {
		t.Fatalf("custom field identity/value not preserved: %#v", fields[0])
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{"id": "u1"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{{
				"id":          "entry-1",
				"description": "List row",
				"timeInterval": map[string]any{
					"start": "2026-05-17T09:00:00Z",
					"end":   "2026-05-17T10:00:00Z",
				},
				"customFieldValues": []map[string]any{
					{"customFieldId": "cf-null", "name": "Empty", "value": nil},
					{"customFieldId": "cf-set", "name": "Region", "value": "EU"},
				},
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.ListEntries(context.Background(), map[string]any{"page_size": 50})
	mustOK(t, res, err, "clockify_entries_list")
	entries, ok := res.Data.([]EntryView)
	if !ok || len(entries) != 1 {
		t.Fatalf("unexpected entries data: %#v", res.Data)
	}
	if len(entries[0].CustomFields) != 1 || entries[0].CustomFields[0].CustomFieldID != "cf-set" {
		t.Fatalf("entries_list should omit null custom fields, got %#v", entries[0].CustomFields)
	}
	if entries[0].CustomFieldValues != nil {
		t.Fatalf("entries_list should omit raw customFieldValues echo, got %#v", entries[0].CustomFieldValues)
	}
}

func TestPaginationMetaAddsConsistentTopLevelSignals(t *testing.T) {
	meta := addPaginationMeta(map[string]any{"count": 50}, map[string]any{"page_size": 50}, 2, 50)

	if meta["page"] != 2 || meta["pageSize"] != 50 {
		t.Fatalf("missing top-level page/pageSize: %#v", meta)
	}
	if meta["total_min"] != 100 {
		t.Fatalf("total_min should be lower-bound rows reached through this page, got %#v", meta["total_min"])
	}
	if _, ok := meta["total"]; ok {
		t.Fatalf("full page without upstream count must not expose authoritative total: %#v", meta)
	}
	if meta["has_more"] != true {
		t.Fatalf("full page should expose has_more=true, got %#v", meta)
	}
	if meta["total_is_lower_bound"] != true {
		t.Fatalf("full page should mark lower-bound total: %#v", meta)
	}
}
