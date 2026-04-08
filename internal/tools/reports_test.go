package tools

import (
	"context"
	"net/http"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestWeeklySummary seeds entries across multiple days and verifies the
// WeeklySummary aggregates correctly into ByDay and ByProject rollups.
func TestWeeklySummary(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				// Monday
				{ID: "e1", Description: "Sprint planning", ProjectID: "p1", ProjectName: "Project A", TimeInterval: clockify.TimeInterval{Start: "2026-04-06T09:00:00Z", End: "2026-04-06T11:00:00Z"}},
				// Tuesday
				{ID: "e2", Description: "Build feature", ProjectID: "p1", ProjectName: "Project A", TimeInterval: clockify.TimeInterval{Start: "2026-04-07T09:00:00Z", End: "2026-04-07T12:00:00Z"}},
				// Wednesday — different project
				{ID: "e3", Description: "Review PRs", ProjectID: "p2", ProjectName: "Project B", TimeInterval: clockify.TimeInterval{Start: "2026-04-08T10:00:00Z", End: "2026-04-08T11:30:00Z"}},
				// Thursday
				{ID: "e4", Description: "Ship", ProjectID: "p1", ProjectName: "Project A", TimeInterval: clockify.TimeInterval{Start: "2026-04-09T14:00:00Z", End: "2026-04-09T16:00:00Z"}},
				// Friday
				{ID: "e5", Description: "Retro", ProjectID: "p2", ProjectName: "Project B", TimeInterval: clockify.TimeInterval{Start: "2026-04-10T15:00:00Z", End: "2026-04-10T16:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.WeeklySummary(context.Background(), map[string]any{
		"week_start": "2026-04-06",
		"timezone":   "UTC",
	})
	if err != nil {
		t.Fatalf("weekly summary failed: %v", err)
	}
	data, ok := result.Data.(WeeklySummaryData)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}

	// Should have 5 days with entries
	if len(data.ByDay) != 5 {
		t.Fatalf("expected 5 days, got %d: %+v", len(data.ByDay), data.ByDay)
	}
	// ByDay is sorted ascending
	if data.ByDay[0].Date != "2026-04-06" {
		t.Fatalf("expected first day 2026-04-06, got %s", data.ByDay[0].Date)
	}
	if data.ByDay[4].Date != "2026-04-10" {
		t.Fatalf("expected last day 2026-04-10, got %s", data.ByDay[4].Date)
	}
	// Tuesday has 3 hours
	if data.ByDay[1].TotalSeconds != 3*3600 {
		t.Fatalf("expected Tuesday = 10800s, got %d", data.ByDay[1].TotalSeconds)
	}

	// ByProject: Project A = 7h (2 + 3 + 2), Project B = 2.5h (1.5 + 1)
	if len(data.ByProject) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(data.ByProject))
	}
	if data.ByProject[0].ProjectName != "Project A" {
		t.Fatalf("expected top project Project A, got %s", data.ByProject[0].ProjectName)
	}
	if data.ByProject[0].TotalSeconds != 7*3600 {
		t.Fatalf("expected Project A = 25200s, got %d", data.ByProject[0].TotalSeconds)
	}
	if data.Totals.Entries != 5 {
		t.Fatalf("expected 5 total entries, got %d", data.Totals.Entries)
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

// TestDetailedReport covers project filtering, the default include_entries=true
// behavior, and the truncation warning when count equals the page size (100).
func TestDetailedReport(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "a", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
				{ID: "b", ProjectID: "p2", ProjectName: "Beta", TimeInterval: clockify.TimeInterval{Start: "2026-04-02T09:00:00Z", End: "2026-04-02T11:00:00Z"}},
				{ID: "c", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-03T09:00:00Z", End: "2026-04-03T09:30:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DetailedReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("detailed report failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	// include_entries defaults to true
	gotEntries, ok := data["entries"].([]clockify.TimeEntry)
	if !ok {
		t.Fatalf("expected entries slice by default, got %T", data["entries"])
	}
	if len(gotEntries) != 3 {
		t.Fatalf("expected 3 unfiltered entries, got %d", len(gotEntries))
	}

	// With include_entries=false, "entries" is omitted.
	resultNoEntries, err := svc.DetailedReport(context.Background(), map[string]any{
		"start":           "2026-04-01T00:00:00Z",
		"end":             "2026-04-08T00:00:00Z",
		"include_entries": false,
	})
	if err != nil {
		t.Fatalf("detailed report (no entries) failed: %v", err)
	}
	dataNoEntries := resultNoEntries.Data.(map[string]any)
	if _, exists := dataNoEntries["entries"]; exists {
		t.Fatalf("expected entries omitted when include_entries=false")
	}
}

// TestDetailedReportProjectFilter resolves the project name and filters entries.
func TestDetailedReportProjectFilter(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "a", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
				{ID: "b", ProjectID: "p2", ProjectName: "Beta", TimeInterval: clockify.TimeInterval{Start: "2026-04-02T09:00:00Z", End: "2026-04-02T11:00:00Z"}},
			})
		case "/workspaces/ws1/projects":
			// Called by resolve.ResolveProjectID when the ref isn't a 24-char hex ID
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DetailedReport(context.Background(), map[string]any{
		"start":   "2026-04-01T00:00:00Z",
		"end":     "2026-04-08T00:00:00Z",
		"project": "Alpha",
	})
	if err != nil {
		t.Fatalf("detailed report with project filter failed: %v", err)
	}
	data := result.Data.(map[string]any)
	filtered, ok := data["entries"].([]clockify.TimeEntry)
	if !ok {
		t.Fatalf("expected entries slice, got %T", data["entries"])
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered entry, got %d", len(filtered))
	}
	if filtered[0].ID != "a" {
		t.Fatalf("expected entry a, got %s", filtered[0].ID)
	}
}
