package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// newPaginatedHandler returns an http.HandlerFunc that serves the given
// pages of time entries for the /workspaces/{ws}/user/{uid}/time-entries
// route, dispatching by the ?page query parameter. Pages are 1-indexed.
// It also serves the /user endpoint so getCurrentUser works. Callers may
// pass pages of any size (including zero-length tail pages).
func newPaginatedHandler(t testing.TB, pages [][]clockify.TimeEntry) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case strings.HasSuffix(r.URL.Path, "/time-entries"):
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			if page-1 >= len(pages) {
				respondJSON(t, w, []clockify.TimeEntry{})
				return
			}
			respondJSON(t, w, pages[page-1])
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}
}

// seedEntries builds a deterministic slice of n entries of a given constant
// duration starting on baseDate. Entries rotate across two projects to
// exercise bucketing.
func seedEntries(baseDate time.Time, n int, durationSec int) []clockify.TimeEntry {
	out := make([]clockify.TimeEntry, n)
	for i := 0; i < n; i++ {
		startTS := baseDate.Add(time.Duration(i) * time.Minute)
		endTS := startTS.Add(time.Duration(durationSec) * time.Second)
		projectID := "p1"
		projectName := "Project A"
		if i%2 == 1 {
			projectID = "p2"
			projectName = "Project B"
		}
		out[i] = clockify.TimeEntry{
			ID:          fmt.Sprintf("e%d", i),
			ProjectID:   projectID,
			ProjectName: projectName,
			TimeInterval: clockify.TimeInterval{
				Start: startTS.UTC().Format(time.RFC3339),
				End:   endTS.UTC().Format(time.RFC3339),
			},
		}
	}
	return out
}

// chunkEntries splits a flat slice into pages of the given size.
func chunkEntries(entries []clockify.TimeEntry, pageSize int) [][]clockify.TimeEntry {
	if pageSize <= 0 {
		return [][]clockify.TimeEntry{entries}
	}
	var pages [][]clockify.TimeEntry
	for i := 0; i < len(entries); i += pageSize {
		end := i + pageSize
		if end > len(entries) {
			end = len(entries)
		}
		pages = append(pages, entries[i:end])
	}
	if len(pages) == 0 {
		pages = append(pages, []clockify.TimeEntry{})
	}
	return pages
}

func TestWeeklySummaryUsesReportsAPIAndDerivesWeekRange(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/weekly" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode weekly body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"totals":      []map[string]any{{"entriesCount": 5}},
				"groupOne":    []map[string]any{{"id": "p1", "name": "Project A"}},
				"totalsByDay": []map[string]any{{"date": "2026-04-06", "duration": 7200}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.WeeklySummary(context.Background(), map[string]any{
		"week_start": "2026-04-06",
		"timezone":   "UTC",
		"weekly_filter": map[string]any{
			"group":    "PROJECT",
			"subgroup": "TIME",
		},
	})
	if err != nil {
		t.Fatalf("weekly summary failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if _, ok := data["totals"]; !ok {
		t.Fatalf("expected upstream totals in data, got %#v", data)
	}
	if gotBody["dateRangeStart"] != "2026-04-06T00:00:00.000" || gotBody["dateRangeEnd"] != "2026-04-12T23:59:59.999" {
		t.Fatalf("weekly range not derived as exact local week: %#v", gotBody)
	}
	filter, _ := gotBody["weeklyFilter"].(map[string]any)
	if filter["group"] != "PROJECT" || filter["subgroup"] != "TIME" {
		t.Fatalf("unexpected weeklyFilter body: %#v", filter)
	}
	if _, has := gotBody["summaryFilter"]; has {
		t.Fatalf("weekly report must not send summaryFilter: %#v", gotBody)
	}
	if result.Meta["source"] != "reports-api" {
		t.Fatalf("source meta = %v, want reports-api", result.Meta["source"])
	}
}

// TestQuickReport verifies TopProject selection, RunningEntries detection,
// and the EntriesSample cap (<=5 when include_entries is false).
func TestQuickReport(t *testing.T) {
	// Build 7 entries, one of which is currently running (End="").
	entries := []clockify.TimeEntry{
		{ID: "e1", ProjectID: "p1", ProjectName: "Top Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}},
		{ID: "e2", ProjectID: "p1", ProjectName: "Top Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-02T09:00:00Z", End: "2026-04-02T11:00:00Z"}},
		{ID: "e3", ProjectID: "p1", ProjectName: "Top Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-03T09:00:00Z", End: "2026-04-03T11:00:00Z"}},
		{ID: "e4", ProjectID: "p2", ProjectName: "Second Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-04T09:00:00Z", End: "2026-04-04T10:00:00Z"}},
		{ID: "e5", ProjectID: "p2", ProjectName: "Second Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-05T09:00:00Z", End: "2026-04-05T09:30:00Z"}},
		{ID: "e6", ProjectID: "p3", ProjectName: "Third Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-06T09:00:00Z", End: "2026-04-06T09:15:00Z"}},
		// Running entry (no end)
		{ID: "e7", ProjectID: "p1", ProjectName: "Top Project", TimeInterval: clockify.TimeInterval{Start: "2026-04-07T09:00:00Z", End: ""}},
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, entries)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.QuickReport(context.Background(), map[string]any{"days": 10})
	if err != nil {
		t.Fatalf("quick report failed: %v", err)
	}
	data, ok := result.Data.(QuickReportData)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data.TopProject == nil || data.TopProject.ProjectName != "Top Project" {
		t.Fatalf("expected TopProject Top Project, got %+v", data.TopProject)
	}
	if data.ProjectsRepresented != 3 {
		t.Fatalf("expected 3 projects represented, got %d", data.ProjectsRepresented)
	}
	if len(data.RunningEntries) != 1 || data.RunningEntries[0].ID != "e7" {
		t.Fatalf("expected 1 running entry e7, got %+v", data.RunningEntries)
	}
	// EntriesSample capped at 5 when include_entries is not set
	if len(data.EntriesSample) != 5 {
		t.Fatalf("expected EntriesSample len 5, got %d", len(data.EntriesSample))
	}
	assertReportSuggestedActions(t, data.SuggestedActions, "p1")

	// With include_entries=true, sample should contain all 7
	result2, err := svc.QuickReport(context.Background(), map[string]any{
		"days":            10,
		"include_entries": true,
	})
	if err != nil {
		t.Fatalf("quick report (include_entries) failed: %v", err)
	}
	data2 := result2.Data.(QuickReportData)
	if len(data2.EntriesSample) != 7 {
		t.Fatalf("expected full 7 entries with include_entries=true, got %d", len(data2.EntriesSample))
	}
}

func TestQuickReportLargeRangeStreamsBoundedSample(t *testing.T) {
	entries := seedEntries(time.Now().UTC().AddDate(0, 0, -1), 250, 60)
	client, cleanup := newTestClient(t, newPaginatedHandler(t, chunkEntries(entries, 200)))
	defer cleanup()

	svc := New(client, "ws1")
	svc.ReportMaxEntries = 100

	result, err := svc.QuickReport(context.Background(), map[string]any{"days": 2})
	if err != nil {
		t.Fatalf("quick report failed: %v", err)
	}
	data := result.Data.(QuickReportData)
	if data.Totals.Entries != 250 {
		t.Fatalf("expected 250 total entries, got %d", data.Totals.Entries)
	}
	if len(data.EntriesSample) != 5 {
		t.Fatalf("expected bounded sample of 5 entries, got %d", len(data.EntriesSample))
	}
	pagination := result.Meta["pagination"].(map[string]any)
	if pagination["pages_fetched"] != 2 || pagination["entries_total"] != 250 {
		t.Fatalf("unexpected pagination meta: %+v", pagination)
	}

	_, err = svc.QuickReport(context.Background(), map[string]any{
		"days":            2,
		"include_entries": true,
	})
	if err == nil || !strings.Contains(err.Error(), "entry cap of 100 exceeded") {
		t.Fatalf("expected cap error for include_entries=true, got %v", err)
	}
}

func TestQuickReportAllowsQuarterAndYearWindows(t *testing.T) {
	var gotDaySpans []int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
			if err != nil {
				t.Fatalf("parse start query: %v", err)
			}
			end, err := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
			if err != nil {
				t.Fatalf("parse end query: %v", err)
			}
			gotDaySpans = append(gotDaySpans, int(end.Sub(start).Hours()/24))
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DefaultTimezone = time.UTC
	for _, days := range []int{90, 365} {
		result, err := svc.QuickReport(context.Background(), map[string]any{"days": days})
		if err != nil {
			t.Fatalf("quick report days=%d failed: %v", days, err)
		}
		if result.Meta["days"] != days {
			t.Fatalf("quick report days=%d meta days = %v", days, result.Meta["days"])
		}
	}
	if len(gotDaySpans) != 2 || gotDaySpans[0] != 90 || gotDaySpans[1] != 365 {
		t.Fatalf("expected queried day spans [90 365], got %v", gotDaySpans)
	}

	_, err := svc.QuickReport(context.Background(), map[string]any{"days": 366})
	if err == nil || !strings.Contains(err.Error(), "days must be between 1 and 365") {
		t.Fatalf("expected max-days error for 366, got %v", err)
	}
}

func TestDetailedReportUsesReportsAPI(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode detailed body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"totals":      []map[string]any{{"entriesCount": 3}},
				"timeentries": []map[string]any{{"id": "e1", "description": "Build"}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DetailedReport(context.Background(), map[string]any{
		"date_range_start": "2026-04-01T00:00:00.000",
		"date_range_end":   "2026-04-08T23:59:59.999",
		"amount_shown":     "COST",
		"amounts":          []any{"EARNED", "COST", "PROFIT"},
		"detailed_filter": map[string]any{
			"page":        1,
			"page_size":   20,
			"sort_column": "ID",
			"options":     map[string]any{"totals": "CALCULATE"},
			"audit_filter": map[string]any{
				"duration":         2,
				"duration_shorter": false,
				"without_project":  false,
				"without_task":     true,
			},
		},
	})
	if err != nil {
		t.Fatalf("detailed report failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["timeentries"]; !ok {
		t.Fatalf("expected upstream timeentries in data, got %#v", data)
	}
	filter, _ := gotBody["detailedFilter"].(map[string]any)
	if filter["pageSize"] != float64(20) || filter["sortColumn"] != "ID" {
		t.Fatalf("detailedFilter not normalized: %#v", filter)
	}
	audit, _ := filter["auditFilter"].(map[string]any)
	if audit["withoutTask"] != true {
		t.Fatalf("auditFilter not normalized: %#v", audit)
	}
	if _, has := gotBody["summaryFilter"]; has {
		t.Fatalf("detailed report must not send summaryFilter: %#v", gotBody)
	}
	if result.Meta["source"] != "reports-api" {
		t.Fatalf("source meta = %v, want reports-api", result.Meta["source"])
	}
}

func TestAttendanceReportUsesReportsAPI(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/attendance" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode attendance body: %v", err)
			}
			respondJSON(t, w, map[string]any{"entities": []map[string]any{{"userId": "u1"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.AttendanceReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-08T00:00:00Z",
		"attendance_filter": map[string]any{
			"page":        1,
			"page_size":   5,
			"sort_column": "USER",
			"start_filters": []any{
				map[string]any{"filtration_type": "EXACTLY", "value": "09:00"},
			},
		},
	})
	if err != nil {
		t.Fatalf("attendance report failed: %v", err)
	}
	data := result.Data.(map[string]any)
	if _, ok := data["entities"]; !ok {
		t.Fatalf("expected upstream entities in data, got %#v", data)
	}
	filter, _ := gotBody["attendanceFilter"].(map[string]any)
	if filter["pageSize"] != float64(5) || filter["sortColumn"] != "USER" {
		t.Fatalf("attendanceFilter not normalized: %#v", filter)
	}
	if _, has := gotBody["detailedFilter"]; has {
		t.Fatalf("attendance report must not send detailedFilter: %#v", gotBody)
	}
}

func TestQuickReportProjectFilterPushesDown(t *testing.T) {
	var gotProject string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			if got := r.URL.Query().Get("name"); got != "Alpha" {
				t.Fatalf("expected project name lookup Alpha, got %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []map[string]any{{"id": "p1", "name": "Alpha"}})
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			gotProject = r.URL.Query().Get("project")
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "a", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.QuickReport(context.Background(), map[string]any{
		"days":    7,
		"project": "Alpha",
	})
	if err != nil {
		t.Fatalf("quick report failed: %v", err)
	}
	if gotProject != "p1" {
		t.Fatalf("expected quick report to push project=p1 upstream, got %q", gotProject)
	}
	pagination := result.Meta["pagination"].(map[string]any)
	if pagination["project_filter_resolved_id"] != "p1" {
		t.Fatalf("expected project_filter_resolved_id=p1, got %+v", pagination)
	}
}

func TestReportFilterValidation(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	_, err := svc.SummaryReport(context.Background(), map[string]any{
		"start":          "2026-04-01T00:00:00Z",
		"end":            "2026-04-08T00:00:00Z",
		"summary_filter": map[string]any{"groups": []any{"CLIENT", "BOGUS"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported value") {
		t.Fatalf("expected invalid summary group error, got %v", err)
	}
	_, err = svc.WeeklySummary(context.Background(), map[string]any{
		"week_start": "2026-04-06",
		"weekly_filter": map[string]any{
			"group":    "CLIENT",
			"subgroup": "TIME",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "PROJECT or USER") {
		t.Fatalf("expected invalid weekly group error, got %v", err)
	}
}

func TestSummaryReportGroupBuilderAcceptsCanonicalDateAndUser(t *testing.T) {
	body, err := buildReportsAPIBody(map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-30T00:00:00Z",
		"summary_filter": map[string]any{
			"groups": []any{"CLIENT", "PROJECT", "DATE"},
		},
	}, "summary_filter", false, time.UTC)
	if err != nil {
		t.Fatalf("build summary body: %v", err)
	}
	filter := body["summaryFilter"].(map[string]any)
	groups := filter["groups"].([]string)
	if len(groups) != 3 || groups[2] != "DATE" {
		t.Fatalf("DATE should be sent upstream unchanged, got %#v", groups)
	}

	body, err = buildReportsAPIBody(map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-30T00:00:00Z",
		"summary_filter": map[string]any{
			"groups": []any{"USER", "TASK"},
		},
	}, "summary_filter", false, time.UTC)
	if err != nil {
		t.Fatalf("build summary body with user/task groups: %v", err)
	}
	filter = body["summaryFilter"].(map[string]any)
	groups = filter["groups"].([]string)
	if len(groups) != 2 || groups[0] != "USER" || groups[1] != "TASK" {
		t.Fatalf("USER/TASK groups should be accepted, got %#v", groups)
	}
}

func TestSummaryReportLegacyDayAliasBuilder(t *testing.T) {
	body, err := buildReportsAPIBody(map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-30T00:00:00Z",
		"summary_filter": map[string]any{
			"groups": []any{"CLIENT", "PROJECT", "DAY"},
		},
	}, "summary_filter", false, time.UTC)
	if err != nil {
		t.Fatalf("build summary body: %v", err)
	}
	filter := body["summaryFilter"].(map[string]any)
	groups := filter["groups"].([]string)
	if len(groups) != 3 || groups[2] != "DATE" {
		t.Fatalf("legacy DAY alias should be sent upstream as DATE, got %#v", groups)
	}
}

func TestWeeklyReportRejectsInvalidSubgroup(t *testing.T) {
	_, err := buildReportsAPIBody(map[string]any{
		"week_start": "2026-04-06",
		"weekly_filter": map[string]any{
			"group":    "PROJECT",
			"subgroup": "USER",
		},
	}, "weekly_filter", true, time.UTC)
	if err == nil || !strings.Contains(err.Error(), "subgroup must be TIME") {
		t.Fatalf("expected invalid weekly subgroup error, got %v", err)
	}
}

func TestReportsAPIBinaryExportEnvelope(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode summary body: %v", err)
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="summary.pdf"`)
			_, _ = w.Write([]byte("pdf-bytes"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start":       "2026-04-01T00:00:00Z",
		"end":         "2026-04-30T00:00:00Z",
		"export_type": "PDF",
		"summary_filter": map[string]any{
			"groups": []any{"CLIENT", "PROJECT", "MONTH"},
		},
	})
	if err != nil {
		t.Fatalf("summary report PDF failed: %v", err)
	}
	if gotBody["exportType"] != "PDF" {
		t.Fatalf("exportType = %v, want PDF", gotBody["exportType"])
	}
	data := result.Data.(map[string]any)
	if data["filename"] != "summary.pdf" || data["bytes"] != float64(len("pdf-bytes")) && data["bytes"] != len("pdf-bytes") {
		t.Fatalf("unexpected binary envelope: %#v", data)
	}
	if result.Meta["binary"] != true || result.Meta["source"] != "reports-api" {
		t.Fatalf("unexpected binary meta: %#v", result.Meta)
	}
}

func assertReportSuggestedActions(t *testing.T, suggestions []ToolSuggestion, wantProject string) {
	t.Helper()
	if len(suggestions) < 2 {
		t.Fatalf("expected at least 2 suggested actions, got %+v", suggestions)
	}

	var listEntries *ToolSuggestion
	var logTime *ToolSuggestion
	for i := range suggestions {
		switch suggestions[i].Tool {
		case "clockify_list_entries":
			listEntries = &suggestions[i]
		case "clockify_log_time":
			logTime = &suggestions[i]
		}
	}
	if listEntries == nil {
		t.Fatalf("missing clockify_list_entries suggestion in %+v", suggestions)
		return
	}
	if got := listEntries.Arguments["project"]; got != wantProject {
		t.Fatalf("list_entries project argument = %v, want %s", got, wantProject)
	}
	if listEntries.Arguments["start"] == "" || listEntries.Arguments["end"] == "" {
		t.Fatalf("list_entries suggestion missing start/end arguments: %+v", listEntries.Arguments)
	}
	if got := listEntries.Arguments["page_size"]; got != 50 {
		t.Fatalf("list_entries page_size argument = %v, want 50", got)
	}

	if logTime == nil {
		t.Fatalf("missing clockify_log_time suggestion in %+v", suggestions)
		return
	}
	if got := logTime.Arguments["dry_run"]; got != true {
		t.Fatalf("log_time dry_run argument = %v, want true", got)
	}
	for _, want := range []string{"project", "description", "start", "end"} {
		if !stringSliceContains(logTime.MissingArgs, want) {
			t.Fatalf("log_time missingArgs = %+v, want %s", logTime.MissingArgs, want)
		}
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestAggregatePaginationMetaReportsPageSizeClamp(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			if got := r.URL.Query().Get("page-size"); got != "200" {
				t.Fatalf("expected clamped page-size=200, got %q", got)
			}
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	agg, _, _, err := svc.aggregateEntriesRange(context.Background(), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), time.UTC, aggregateOptions{
		PageSize: 500,
	})
	if err != nil {
		t.Fatalf("aggregate failed: %v", err)
	}
	meta := paginationMeta(agg, 500, svc.reportLimitsForArgs(map[string]any{}))
	pagination := meta["pagination"].(map[string]any)
	if pagination["requested_page_size"].(int) != 500 || pagination["applied_page_size"].(int) != 200 || pagination["clamped"] != true {
		t.Fatalf("unexpected pagination clamp meta: %+v", pagination)
	}
}

// TestAggregateEntriesRange_NeverLosesData is a property-style table test that
// exercises the paginator at page-boundary edge cases from 0 to 1000 entries.
// TotalSeconds must equal N * knownDuration for every N, proving no data is
// lost across the page boundary.
func TestAggregateEntriesRange_NeverLosesData(t *testing.T) {
	const durSec = 60
	cases := []int{0, 1, 199, 200, 201, 400, 599, 600, 999, 1000}
	for _, n := range cases {
		n := n
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
			entries := seedEntries(base, n, durSec)
			pages := chunkEntries(entries, reportPageSize)

			client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
			defer cleanup()

			svc := New(client, "ws1")
			start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
			agg, _, _, err := svc.aggregateEntriesRange(context.Background(), start, end, time.UTC, aggregateOptions{
				PageSize:       reportPageSize,
				IncludeEntries: false,
			})
			if err != nil {
				t.Fatalf("aggregate failed: %v", err)
			}
			want := int64(n) * int64(durSec)
			if agg.TotalSeconds != want {
				t.Fatalf("TotalSeconds = %d, want %d (n=%d)", agg.TotalSeconds, want, n)
			}
			if agg.EntriesCount != n {
				t.Fatalf("EntriesCount = %d, want %d", agg.EntriesCount, n)
			}
		})
	}
}
