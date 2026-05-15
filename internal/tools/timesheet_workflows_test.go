package tools

import (
	"context"
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
