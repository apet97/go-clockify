package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestSchedulingCapacityHandlerAllowsMissingUserIDs(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	// The unreachable client URL makes the upstream call fail; we only
	// assert the handler no longer rejects a missing user_ids up front.
	_, err := svc.schedulingCapacityOneUser(context.Background(), map[string]any{
		"start": "2026-05-01",
		"end":   "2026-05-07",
	})
	if err != nil && strings.Contains(err.Error(), "user_ids is required") {
		t.Fatalf("user_ids should be optional: %v", err)
	}
}

func TestAssignmentReportJoinsScheduledAndTrackedMoney(t *testing.T) {
	var sawUserTotals, sawProjectTotals, sawReports bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/all" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{{
				"id":          "a1",
				"userId":      "u1",
				"userName":    "Alice",
				"projectId":   "p1",
				"projectName": "Build",
				"taskId":      "t1",
				"taskName":    "Implementation",
				"hoursPerDay": 4.0,
				"period": map[string]any{
					"start": "2026-05-01T00:00:00Z",
					"end":   "2026-05-01T23:59:59Z",
				},
			}})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/user-filter/totals" && r.Method == http.MethodPost:
			sawUserTotals = true
			respondJSON(t, w, []map[string]any{{
				"userId": "u1",
				"totalHoursPerDay": []map[string]any{
					{"date": "2026-05-01T00:00:00Z", "totalHours": 8.0},
				},
			}})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/projects/totals" && r.Method == http.MethodPost:
			sawProjectTotals = true
			respondJSON(t, w, []map[string]any{{
				"projectId":       "p1",
				"amountScheduled": 40000,
				"costScheduled":   10000,
				"currency":        "USD",
			}})
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			sawReports = true
			body := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode report body: %v", err)
			}
			filter, _ := body["detailedFilter"].(map[string]any)
			if got := filter["page"]; got != float64(1) && got != 1 {
				t.Fatalf("detailed page = %#v, want 1", got)
			}
			if got := body["amountShown"]; got != "EARNED" {
				t.Fatalf("amountShown = %#v, want EARNED", got)
			}
			respondJSON(t, w, map[string]any{"timeEntries": []map[string]any{{
				"userId":    "u1",
				"projectId": "p1",
				"taskId":    "t1",
				"duration":  7200,
				"amounts": []map[string]any{
					{"type": "EARNED", "value": 12500, "currency": "USD"},
					{"type": "COST", "value": 5000, "currency": "USD"},
					{"type": "PROFIT", "value": 7500, "currency": "USD"},
				},
			}}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.AssignmentReport(context.Background(), map[string]any{
		"start":    "2026-05-01T00:00:00Z",
		"end":      "2026-05-01T23:59:59Z",
		"group_by": []string{"USER", "PROJECT", "TASK"},
	})
	if err != nil {
		t.Fatalf("assignment report failed: %v", err)
	}
	data, ok := result.Data.(AssignmentReportData)
	if !ok {
		t.Fatalf("data type = %T, want AssignmentReportData", result.Data)
	}
	if len(data.Rows) != 1 {
		t.Fatalf("rows = %#v, want one joined row", data.Rows)
	}
	row := data.Rows[0]
	if row.Scheduled.Seconds != 14400 || row.Available.Seconds != 28800 || row.Tracked.Seconds != 7200 {
		t.Fatalf("durations = scheduled %d available %d tracked %d", row.Scheduled.Seconds, row.Available.Seconds, row.Tracked.Seconds)
	}
	assertMoneyCents(t, row.AmountScheduled, 40000, "USD")
	assertMoneyCents(t, row.CostScheduled, 10000, "USD")
	assertMoneyCents(t, row.AmountTracked, 12500, "USD")
	assertMoneyCents(t, row.CostTracked, 5000, "USD")
	assertMoneyCents(t, row.RealizedProfit, 7500, "USD")
	if row.Difference.Seconds != 7200 {
		t.Fatalf("difference seconds = %d, want 7200", row.Difference.Seconds)
	}
	if !sawUserTotals || !sawProjectTotals || !sawReports {
		t.Fatalf("expected all assignment report sources; user=%v project=%v reports=%v", sawUserTotals, sawProjectTotals, sawReports)
	}
}

func TestReportEntitiesDoNotReuseGenericGroupIDForEveryEntity(t *testing.T) {
	entities := reportEntities(map[string]any{
		"id":          "user-group-id",
		"name":        "Alice",
		"projectName": "Build",
		"clientName":  "Acme",
	})
	project, ok := entities["project"].(map[string]any)
	if !ok {
		t.Fatalf("expected project entity map, got %#v", entities["project"])
	}
	if project["id"] == "user-group-id" {
		t.Fatalf("generic report group id leaked into project entity: %#v", entities)
	}
	client, ok := entities["client"].(map[string]any)
	if !ok {
		t.Fatalf("expected client entity map, got %#v", entities["client"])
	}
	if client["id"] == "user-group-id" {
		t.Fatalf("generic report group id leaked into client entity: %#v", entities)
	}
}

func TestAssignmentReportReportFailureDoesNotFail(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/all" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{{"id": "a1", "userId": "u1", "hoursPerDay": 1.0, "start": "2026-05-01T00:00:00Z", "end": "2026-05-01T23:59:59Z"}})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/user-filter/totals" && r.Method == http.MethodPost:
			respondJSON(t, w, []map[string]any{})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/projects/totals" && r.Method == http.MethodPost:
			respondJSON(t, w, []map[string]any{})
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			http.Error(w, `{"message":"reports down"}`, http.StatusBadGateway)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.AssignmentReport(context.Background(), map[string]any{
		"start": "2026-05-01T00:00:00Z",
		"end":   "2026-05-01T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("assignment report should survive reports failure: %v", err)
	}
	data := result.Data.(AssignmentReportData)
	if len(data.Rows) != 1 {
		t.Fatalf("rows = %#v, want scheduled row", data.Rows)
	}
	if data.Rows[0].Financials.Source != entryFinancialSourceUnavailable {
		t.Fatalf("financial source = %q, want unavailable", data.Rows[0].Financials.Source)
	}
	if result.Meta["reports_api_error"] == nil {
		t.Fatalf("meta missing reports_api_error: %#v", result.Meta)
	}
}

func TestAssignmentReportTrackedOnlyRowsAreNotDoubleCounted(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/all" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/user-filter/totals" && r.Method == http.MethodPost:
			respondJSON(t, w, []map[string]any{})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/projects/totals" && r.Method == http.MethodPost:
			respondJSON(t, w, []map[string]any{})
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			respondJSON(t, w, map[string]any{"timeEntries": []map[string]any{{
				"userId":   "u1",
				"duration": 3600,
				"amounts":  []map[string]any{{"type": "EARNED", "value": 10000, "currency": "USD"}},
			}}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.AssignmentReport(context.Background(), map[string]any{
		"start":    "2026-05-01T00:00:00Z",
		"end":      "2026-05-01T23:59:59Z",
		"group_by": []string{"USER"},
	})
	if err != nil {
		t.Fatalf("assignment report failed: %v", err)
	}
	data := result.Data.(AssignmentReportData)
	if len(data.Rows) != 1 {
		t.Fatalf("rows = %#v, want one tracked-only row", data.Rows)
	}
	if data.Rows[0].Tracked.Seconds != 3600 {
		t.Fatalf("tracked seconds = %d, want 3600", data.Rows[0].Tracked.Seconds)
	}
	assertMoneyCents(t, data.Rows[0].AmountTracked, 10000, "USD")
}

func TestAssignmentGroupByAcceptsSupportedGroups(t *testing.T) {
	for _, group := range assignmentReportGroups {
		got, err := assignmentGroupsFromArgs(map[string]any{"group_by": []string{group}})
		if err != nil {
			t.Fatalf("group %s rejected: %v", group, err)
		}
		if len(got) != 1 || got[0] != group {
			t.Fatalf("group %s normalized to %#v", group, got)
		}
	}
	got, err := assignmentGroupsFromArgs(map[string]any{})
	if err != nil {
		t.Fatalf("default groups failed: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"USER", "PROJECT", "TASK"}) {
		t.Fatalf("default groups = %#v", got)
	}
	if _, err := assignmentGroupsFromArgs(map[string]any{"group_by": []string{"USER", "BAD"}}); err == nil {
		t.Fatal("expected unsupported group error")
	}
}

func TestAssignmentDurationDisplay(t *testing.T) {
	view := durationView(int64((2*time.Hour + 5*time.Minute) / time.Second))
	if view.Display != "2:05" || view.Hours != 2+float64(5)/60 {
		t.Fatalf("duration view = %+v", view)
	}
}

func TestSchedulingListAssignmentsRequiresRangeAndUsesDocBackedPageSize(t *testing.T) {
	svc := New(nil, "ws1")
	var schema map[string]any
	for _, desc := range schedulingHandlers(svc) {
		if desc.Tool.Name == "clockify_list_assignments" {
			schema = desc.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("missing list assignments schema")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("list assignments required schema = %T", schema["required"])
	}
	for _, want := range []string{"start", "end"} {
		if !containsString(required, want) {
			t.Fatalf("clockify_list_assignments required missing %s: %#v", want, required)
		}
	}

	var gotQuery string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/scheduling/assignments/all" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		respondJSON(t, w, []map[string]any{})
	})
	defer cleanup()

	svc = New(client, "ws1")
	_, err := svc.listAssignments(context.Background(), map[string]any{
		"start":     "2026-05-01T00:00:00Z",
		"end":       "2026-05-02T00:00:00Z",
		"page_size": 7,
	})
	if err != nil {
		t.Fatalf("list assignments failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", gotQuery, err)
	}
	if got := values.Get("page-size"); got != "7" {
		t.Fatalf("expected doc-backed page-size=7, got %q in %q", got, gotQuery)
	}
	if got := values.Get("pageSize"); got != "" {
		t.Fatalf("must not send ignored pageSize spelling for assignments/all, got %q in %q", got, gotQuery)
	}

	_, err = svc.listAssignments(context.Background(), map[string]any{"start": "2026-05-01T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "start and end are required") {
		t.Fatalf("expected missing range error, got %v", err)
	}
}
