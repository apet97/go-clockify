package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestTimesheetReviewFindsIssuesAndSuggestedActions(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{
				{
					"id":          "e1",
					"description": "",
					"projectId":   "p1",
					"projectName": "Project A",
					"timeInterval": map[string]any{
						"start": "2026-04-06T09:00:00Z",
						"end":   "2026-04-06T10:00:00Z",
					},
				},
				{
					"id":          "e2",
					"description": "overlaps e1",
					"projectId":   "p2",
					"projectName": "Project B",
					"timeInterval": map[string]any{
						"start": "2026-04-06T09:30:00Z",
						"end":   "2026-04-06T10:30:00Z",
					},
				},
				{
					"id":          "e3",
					"description": "needs project",
					"timeInterval": map[string]any{
						"start": "2026-04-06T11:00:00Z",
						"end":   "2026-04-06T12:00:00Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetReview(context.Background(), map[string]any{
		"start":           "2026-04-06T09:00:00Z",
		"end":             "2026-04-06T13:00:00Z",
		"timezone":        "UTC",
		"workday_start":   "09:00",
		"workday_end":     "13:00",
		"min_gap_minutes": 30,
		"max_suggestions": 20,
	})
	if err != nil {
		t.Fatalf("TimesheetReview: %v", err)
	}
	data, ok := result.Data.(TimesheetReviewData)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data.Totals.Entries != 3 {
		t.Fatalf("expected 3 reviewed entries, got %d", data.Totals.Entries)
	}
	assertIssueType(t, data.Issues, "missing_description")
	assertIssueType(t, data.Issues, "missing_project")
	assertIssueType(t, data.Issues, "overlap")
	assertIssueType(t, data.Issues, "gap")
	assertSuggestionTool(t, data.SuggestedActions, oneUserToolFixEntry)
	assertSuggestionTool(t, data.SuggestedActions, "clockify_entries_update")
	assertSuggestionTool(t, data.SuggestedActions, oneUserToolEntriesCreateFromGap)
}

func TestTimesheetReviewMaxRowsCapsDetailLists(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{
				reviewEntryPayload("e1", "", "p1", "Project 1", "2026-04-06T09:00:00Z", "2026-04-06T10:00:00Z"),
				reviewEntryPayload("e2", "", "p2", "Project 2", "2026-04-06T10:00:00Z", "2026-04-06T11:00:00Z"),
				reviewEntryPayload("e3", "", "p3", "Project 3", "2026-04-06T11:00:00Z", "2026-04-06T12:00:00Z"),
				reviewEntryPayload("e4", "", "p4", "Project 4", "2026-04-06T12:00:00Z", "2026-04-06T13:00:00Z"),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetReview(context.Background(), map[string]any{
		"start":           "2026-04-06T09:00:00Z",
		"end":             "2026-04-06T13:00:00Z",
		"timezone":        "UTC",
		"include_entries": true,
		"min_gap_minutes": 0,
		"max_rows":        2,
	})
	if err != nil {
		t.Fatalf("TimesheetReview: %v", err)
	}
	data := result.Data.(TimesheetReviewData)
	if data.Totals.Entries != 4 {
		t.Fatalf("totals should remain complete, got %+v", data.Totals)
	}
	if len(data.ByDay) != 1 || data.ByDay[0].Entries != 4 {
		t.Fatalf("byDay aggregates should remain complete: %+v", data.ByDay)
	}
	if len(data.ByProject) != 2 || len(data.Issues) != 2 || len(data.Entries) != 2 {
		t.Fatalf("detail lists not capped: projects=%d issues=%d entries=%d", len(data.ByProject), len(data.Issues), len(data.Entries))
	}
	if result.Meta["truncated"] != true || result.Meta["byProjectTotal"] != 4 || result.Meta["issuesTotal"] != 4 || result.Meta["entriesTotal"] != 4 {
		t.Fatalf("missing truncation meta: %#v", result.Meta)
	}
}

func TestTimesheetReviewDefaultMaxRowsCapsIncludedEntries(t *testing.T) {
	start := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	entries := make([]map[string]any, defaultTimesheetReviewMaxRows+1)
	for i := range entries {
		entryStart := start.Add(time.Duration(i) * time.Hour)
		entries[i] = reviewEntryPayload(
			fmt.Sprintf("e%d", i+1),
			"",
			fmt.Sprintf("p%d", i+1),
			fmt.Sprintf("Project %d", i+1),
			entryStart.Format(time.RFC3339),
			entryStart.Add(30*time.Minute).Format(time.RFC3339),
		)
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, entries)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetReview(context.Background(), map[string]any{
		"start":           start.Format(time.RFC3339),
		"end":             start.Add(time.Duration(len(entries)) * time.Hour).Format(time.RFC3339),
		"timezone":        "UTC",
		"include_entries": true,
		"min_gap_minutes": 0,
	})
	if err != nil {
		t.Fatalf("TimesheetReview: %v", err)
	}
	data := result.Data.(TimesheetReviewData)
	if data.Totals.Entries != len(entries) {
		t.Fatalf("totals should remain complete, got %+v", data.Totals)
	}
	if len(data.ByProject) != defaultTimesheetReviewMaxRows || len(data.Issues) != defaultTimesheetReviewMaxRows || len(data.Entries) != defaultTimesheetReviewMaxRows {
		t.Fatalf("default detail cap not applied: projects=%d issues=%d entries=%d", len(data.ByProject), len(data.Issues), len(data.Entries))
	}
	if result.Meta["truncated"] != true || result.Meta["entriesReturned"] != defaultTimesheetReviewMaxRows {
		t.Fatalf("missing default truncation meta: %#v", result.Meta)
	}
}

func TestTimesheetReviewRangeUsesLocalTimezoneAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	start, end, mode, err := timesheetReviewRange(map[string]any{"date": "2026-03-08"}, loc)
	if err != nil {
		t.Fatalf("timesheetReviewRange: %v", err)
	}
	if mode != "day" {
		t.Fatalf("mode=%q, want day", mode)
	}
	if got, want := start.Format(time.RFC3339), "2026-03-08T05:00:00Z"; got != want {
		t.Fatalf("start=%s, want %s", got, want)
	}
	if got, want := end.Format(time.RFC3339), "2026-03-09T04:00:00Z"; got != want {
		t.Fatalf("end=%s, want %s", got, want)
	}
	if got := end.Sub(start); got != 23*time.Hour {
		t.Fatalf("DST day duration=%v, want 23h", got)
	}
}

func reviewEntryPayload(id, description, projectID, projectName, start, end string) map[string]any {
	return map[string]any{
		"id":          id,
		"description": description,
		"projectId":   projectID,
		"projectName": projectName,
		"timeInterval": map[string]any{
			"start": start,
			"end":   end,
		},
	}
}

func TestBuildTimesheetReviewCoversTimeTrackingEdges(t *testing.T) {
	now := time.Now().UTC()
	entries := []clockify.TimeEntry{
		reviewEntry("midnight", "crosses midnight", "p1", "REGULAR", "2026-04-06T23:30:00Z", "2026-04-07T00:30:00Z"),
		reviewEntry("overlap-a", "overlap A", "p1", "REGULAR", "2026-04-07T09:00:00Z", "2026-04-07T10:00:00Z"),
		reviewEntry("overlap-b", "overlap B", "p1", "REGULAR", "2026-04-07T09:45:00Z", "2026-04-07T10:15:00Z"),
		reviewEntry("zero", "zero duration", "p1", "REGULAR", "2026-04-07T11:00:00Z", "2026-04-07T11:00:00Z"),
		reviewEntry("running", "still going", "p1", "REGULAR", now.Add(-time.Hour).Format(time.RFC3339), ""),
		reviewEntry("no-project", "needs project", "", "REGULAR", "2026-04-07T13:00:00Z", "2026-04-07T14:00:00Z"),
		reviewEntry("break", "coffee", "", "BREAK", "2026-04-07T14:00:00Z", "2026-04-07T14:15:00Z"),
	}
	issues, suggestions := buildTimesheetReview(entries,
		mustTime(t, "2026-04-06T00:00:00Z"),
		mustTime(t, "2026-04-08T00:00:00Z"),
		time.UTC,
		"09:00",
		"17:00",
		30,
		20,
	)

	for _, typ := range []string{"overlap", "gap", "running_timer", "missing_project", "zero_duration"} {
		assertIssueType(t, issues, typ)
	}
	assertNoIssueForEntry(t, issues, "missing_project", "break")
	assertSuggestionTool(t, suggestions, oneUserToolEntriesCreateFromGap)
	assertSuggestionTool(t, suggestions, "clockify_entries_timer_stop")
}

func TestClockifyFixEntryReturnsRecoveryForProtectedTimeEntryStates(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantText string
	}{
		{name: "locked", message: "entry is locked", wantText: "locked"},
		{name: "approved", message: "entry belongs to an approved timesheet", wantText: "approved"},
		{name: "invoiced", message: "entry has already been invoiced", wantText: "invoiced"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
				case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-entries/e1":
					respondJSON(t, w, reviewEntry("e1", "old", "p1", "REGULAR", "2026-04-07T09:00:00Z", "2026-04-07T10:00:00Z"))
				case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/time-entries/e1":
					http.Error(w, `{"message":"`+tc.message+`"}`, http.StatusForbidden)
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			})
			defer cleanup()

			svc := New(client, "ws1")
			server := mcp.NewServer("test", svc.FullAccessRegistry())
			initializeServer(t, server)
			errResult := callToolError(t, server, oneUserToolFixEntry, map[string]any{
				"entry_id":        "e1",
				"new_description": "new",
			})
			if errResult.Error.Code != "auth_or_permission" {
				t.Fatalf("code=%q, want auth_or_permission: %+v", errResult.Error.Code, errResult)
			}
			text := strings.ToLower(errResult.Error.Message + " " + errResult.Recovery.Hint)
			if !strings.Contains(text, tc.wantText) {
				t.Fatalf("recovery did not mention %q: %+v", tc.wantText, errResult)
			}
		})
	}
}

func TestTimesheetReviewFiltersEntriesOutsideReportedRange(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{
				reviewEntryPayload("outside", "previous evening", "p1", "Project", "2026-05-16T20:17:10Z", "2026-05-16T21:17:10Z"),
				reviewEntryPayload("inside", "inside local day", "p1", "Project", "2026-05-16T22:30:00Z", "2026-05-16T23:30:00Z"),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetReview(context.Background(), map[string]any{
		"date":            "2026-05-17",
		"timezone":        "Europe/Belgrade",
		"include_entries": true,
		"min_gap_minutes": 0,
	})
	if err != nil {
		t.Fatalf("TimesheetReview: %v", err)
	}
	data := result.Data.(TimesheetReviewData)
	if data.Range.Start != "2026-05-16T22:00:00Z" || data.Range.End != "2026-05-17T22:00:00Z" {
		t.Fatalf("unexpected reported range: %+v", data.Range)
	}
	if data.Totals.Entries != 1 {
		t.Fatalf("expected only in-range entry to be reviewed, got totals=%+v", data.Totals)
	}
	if len(data.ByDay) != 1 || data.ByDay[0].Date != "2026-05-17" {
		t.Fatalf("byDay should stay inside requested local day, got %+v", data.ByDay)
	}
	if len(data.Entries) != 1 || data.Entries[0].ID != "inside" {
		t.Fatalf("entries should exclude starts before range, got %+v", data.Entries)
	}
	for _, issue := range data.Issues {
		if issue.EntryID == "outside" || strings.Contains(issue.Message, "outside") {
			t.Fatalf("issues should not reference outside-range entry: %+v", issue)
		}
	}
}

func assertIssueType(t *testing.T, issues []TimesheetIssue, typ string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Type == typ {
			return
		}
	}
	t.Fatalf("missing issue type %q in %+v", typ, issues)
}

func assertNoIssueForEntry(t *testing.T, issues []TimesheetIssue, typ, entryID string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Type == typ && issue.EntryID == entryID {
			t.Fatalf("unexpected issue type %q for entry %q in %+v", typ, entryID, issues)
		}
	}
}

func assertSuggestionTool(t *testing.T, suggestions []ToolSuggestion, tool string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Tool == tool {
			return
		}
	}
	t.Fatalf("missing suggestion tool %q in %+v", tool, suggestions)
}

func reviewEntry(id, description, projectID, entryType, start, end string) clockify.TimeEntry {
	return clockify.TimeEntry{
		ID:          id,
		Description: description,
		ProjectID:   projectID,
		Type:        entryType,
		UserID:      "u1",
		TimeInterval: clockify.TimeInterval{
			Start: start,
			End:   end,
		},
	}
}

func mustTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return parsed
}
