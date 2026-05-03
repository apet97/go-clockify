package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
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
	assertSuggestionTool(t, data.SuggestedActions, "clockify_find_and_update_entry")
	assertSuggestionTool(t, data.SuggestedActions, "clockify_update_entry")
	assertSuggestionTool(t, data.SuggestedActions, "clockify_timesheet_fill_gap")
}

func TestTimesheetFillGapDryRunValidatesWithoutPost(t *testing.T) {
	var postCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-entries":
			postCalled = true
			t.Fatalf("POST must not be called for dry_run")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetFillGap(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"project_id":  "p1",
		"description": "Deep work",
		"dry_run":     true,
	})
	if err != nil {
		t.Fatalf("TimesheetFillGap dry-run: %v", err)
	}
	if postCalled {
		t.Fatal("POST called during dry-run")
	}
	data, ok := result.Data.(TimesheetFillGapData)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if !data.DryRun || !data.Validated {
		t.Fatalf("expected dry_run validated result, got %+v", data)
	}
	if data.Proposed.ProjectID != "p1" || data.Proposed.Description != "Deep work" {
		t.Fatalf("unexpected draft: %+v", data.Proposed)
	}
}

func TestTimesheetFillGapRejectsOverlap(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{
				{
					"id":          "e1",
					"description": "already tracked",
					"projectId":   "p1",
					"timeInterval": map[string]any{
						"start": "2026-04-06T09:30:00Z",
						"end":   "2026-04-06T10:00:00Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.TimesheetFillGap(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"project_id":  "p1",
		"description": "Deep work",
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps 1 existing entry") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
}

func TestTimesheetFillGapCreatesEntryAfterValidation(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Test"})
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-entries":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["projectId"] != "p1" || body["description"] != "Deep work" {
				t.Fatalf("unexpected body: %v", body)
			}
			if body["billable"] != true {
				t.Fatalf("expected billable=true, got %v", body["billable"])
			}
			respondJSON(t, w, map[string]any{
				"id":          "new1",
				"description": "Deep work",
				"projectId":   "p1",
				"timeInterval": map[string]any{
					"start": body["start"],
					"end":   body["end"],
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimesheetFillGap(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"project_id":  "p1",
		"description": "Deep work",
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("TimesheetFillGap create: %v", err)
	}
	data := result.Data.(TimesheetFillGapData)
	if data.Entry.ID != "new1" {
		t.Fatalf("expected created entry new1, got %+v", data.Entry)
	}
	if data.DryRun {
		t.Fatal("created result should not be dry_run")
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

func assertSuggestionTool(t *testing.T, suggestions []ToolSuggestion, tool string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Tool == tool {
			return
		}
	}
	t.Fatalf("missing suggestion tool %q in %+v", tool, suggestions)
}
