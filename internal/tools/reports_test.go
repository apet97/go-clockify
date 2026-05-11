package tools

import (
	"context"
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
	assertReportSuggestedActions(t, data.SuggestedActions, "p1")
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
	data, ok := result.Data.(SummaryData)
	if !ok {
		t.Fatalf("expected SummaryData, got %T", result.Data)
	}
	// include_entries defaults to true
	if len(data.Entries) != 3 {
		t.Fatalf("expected 3 unfiltered entries, got %d", len(data.Entries))
	}
	if len(data.ByProject) != 2 || data.ByProject[0].ProjectID != "p2" {
		t.Fatalf("expected top detailed project p2, got %+v", data.ByProject)
	}
	assertReportSuggestedActions(t, data.SuggestedActions, "p2")

	// With include_entries=false, the SummaryData.Entries slice stays empty
	// and the field is omitted from the JSON encoding (omitempty).
	resultNoEntries, err := svc.DetailedReport(context.Background(), map[string]any{
		"start":           "2026-04-01T00:00:00Z",
		"end":             "2026-04-08T00:00:00Z",
		"include_entries": false,
	})
	if err != nil {
		t.Fatalf("detailed report (no entries) failed: %v", err)
	}
	dataNoEntries := resultNoEntries.Data.(SummaryData)
	if len(dataNoEntries.Entries) != 0 {
		t.Fatalf("expected entries omitted when include_entries=false, got %d", len(dataNoEntries.Entries))
	}
	if len(dataNoEntries.SuggestedActions) == 0 {
		t.Fatalf("expected suggestedActions when include_entries=false")
	}
}

// TestDetailedReportProjectFilter resolves the project name and filters entries.
func TestDetailedReportProjectFilter(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			if got := r.URL.Query().Get("project"); got != "p1" {
				t.Fatalf("expected detailed report to push project=p1 upstream, got %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "a", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
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
	data := result.Data.(SummaryData)
	if len(data.Entries) != 1 {
		t.Fatalf("expected 1 filtered entry, got %d", len(data.Entries))
	}
	if data.Entries[0].ID != "a" {
		t.Fatalf("expected entry a, got %s", data.Entries[0].ID)
	}
	pagination := result.Meta["pagination"].(map[string]any)
	if pagination["project_filter_resolved_id"] != "p1" {
		t.Fatalf("expected project_filter_resolved_id=p1, got %+v", pagination)
	}
}

func TestReportProjectFiltersPushDown(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *Service) (ResultEnvelope, error)
	}{
		{
			name: "summary",
			run: func(ctx context.Context, svc *Service) (ResultEnvelope, error) {
				return svc.SummaryReport(ctx, map[string]any{
					"start":   "2026-04-01T00:00:00Z",
					"end":     "2026-04-08T00:00:00Z",
					"project": "Alpha",
				})
			},
		},
		{
			name: "weekly",
			run: func(ctx context.Context, svc *Service) (ResultEnvelope, error) {
				return svc.WeeklySummary(ctx, map[string]any{
					"week_start": "2026-04-06",
					"timezone":   "UTC",
					"project":    "Alpha",
				})
			},
		},
		{
			name: "quick",
			run: func(ctx context.Context, svc *Service) (ResultEnvelope, error) {
				return svc.QuickReport(ctx, map[string]any{
					"days":    7,
					"project": "Alpha",
				})
			},
		},
		{
			name: "detailed",
			run: func(ctx context.Context, svc *Service) (ResultEnvelope, error) {
				return svc.DetailedReport(ctx, map[string]any{
					"start":           "2026-04-01T00:00:00Z",
					"end":             "2026-04-08T00:00:00Z",
					"project":         "Alpha",
					"include_entries": false,
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			result, err := tt.run(context.Background(), svc)
			if err != nil {
				t.Fatalf("%s report failed: %v", tt.name, err)
			}
			if gotProject != "p1" {
				t.Fatalf("expected %s report to push project=p1 upstream, got %q", tt.name, gotProject)
			}
			pagination := result.Meta["pagination"].(map[string]any)
			if pagination["project_filter_resolved_id"] != "p1" {
				t.Fatalf("expected project_filter_resolved_id=p1, got %+v", pagination)
			}
		})
	}
}

// TestSummaryReport_MultiPage verifies the streaming paginator walks
// multiple pages and the totals reflect every entry, not just the first page.
func TestSummaryReport_MultiPage(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	const durSec = 3600 // 1h per entry
	entries := seedEntries(base, 600, durSec)
	pages := chunkEntries(entries, reportPageSize) // 3 pages of 200

	client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-30T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}
	data, ok := result.Data.(SummaryData)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	want := int64(600 * durSec)
	if data.Totals.TotalSeconds != want {
		t.Fatalf("TotalSeconds = %d, want %d (proof paginator walked all pages)", data.Totals.TotalSeconds, want)
	}
	if data.Totals.Entries != 600 {
		t.Fatalf("Totals.Entries = %d, want 600", data.Totals.Entries)
	}
	// Two projects alternating -> both should appear.
	if len(data.ByProject) != 2 {
		t.Fatalf("ByProject len = %d, want 2", len(data.ByProject))
	}
}

// TestWeeklySummary_MultiPage verifies day bucketing works across paginated
// batches.
func TestWeeklySummary_MultiPage(t *testing.T) {
	// Build 3 pages of 200 entries, each 1-hour long, stepping by 4 minutes
	// so they fit inside one week.
	base := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday
	entries := make([]clockify.TimeEntry, 600)
	for i := 0; i < 600; i++ {
		// Day 0, 1, 2 across the three pages.
		day := i / 200
		start := base.AddDate(0, 0, day).Add(time.Duration(i%200) * time.Minute)
		end := start.Add(time.Hour)
		entries[i] = clockify.TimeEntry{
			ID:          fmt.Sprintf("e%d", i),
			ProjectID:   "p1",
			ProjectName: "Project A",
			TimeInterval: clockify.TimeInterval{
				Start: start.UTC().Format(time.RFC3339),
				End:   end.UTC().Format(time.RFC3339),
			},
		}
	}
	pages := chunkEntries(entries, reportPageSize)

	client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.WeeklySummary(context.Background(), map[string]any{
		"week_start": "2026-04-06",
		"timezone":   "UTC",
	})
	if err != nil {
		t.Fatalf("weekly summary failed: %v", err)
	}
	data := result.Data.(WeeklySummaryData)
	if len(data.ByDay) < 3 {
		t.Fatalf("expected at least 3 day buckets across pages, got %d", len(data.ByDay))
	}
	if data.Totals.Entries != 600 {
		t.Fatalf("Totals.Entries = %d, want 600", data.Totals.Entries)
	}
}

// TestDetailedReport_CapExceeded_IncludeTrue_Errors ensures the streaming
// aggregator fails closed when include_entries=true and the hard cap is
// exceeded, rather than silently truncating or OOMing.
func TestDetailedReport_CapExceeded_IncludeTrue_Errors(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	entries := seedEntries(base, 150, 3600)
	pages := chunkEntries(entries, reportPageSize)

	client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
	defer cleanup()

	svc := New(client, "ws1")
	svc.ReportMaxEntries = 100
	_, err := svc.DetailedReport(context.Background(), map[string]any{
		"start":           "2026-04-01T00:00:00Z",
		"end":             "2026-04-30T00:00:00Z",
		"include_entries": true,
	})
	if err == nil {
		t.Fatalf("expected error when entry cap exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "entry cap") {
		t.Fatalf("expected error to mention 'entry cap', got: %v", err)
	}
}

// TestDetailedReport_CapExceeded_IncludeFalse_Succeeds verifies the cap is
// not enforced when include_entries=false: totals should be correct for the
// full range and memory is bounded by design.
func TestDetailedReport_CapExceeded_IncludeFalse_Succeeds(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	const durSec = 3600
	entries := seedEntries(base, 150, durSec)
	pages := chunkEntries(entries, reportPageSize)

	client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
	defer cleanup()

	svc := New(client, "ws1")
	svc.ReportMaxEntries = 100
	result, err := svc.DetailedReport(context.Background(), map[string]any{
		"start":           "2026-04-01T00:00:00Z",
		"end":             "2026-04-30T00:00:00Z",
		"include_entries": false,
	})
	if err != nil {
		t.Fatalf("expected success when include_entries=false, got: %v", err)
	}
	data := result.Data.(SummaryData)
	if len(data.Entries) != 0 {
		t.Fatalf("expected entries omitted, got %d", len(data.Entries))
	}
	totals := data.Totals
	if totals.Entries != 150 {
		t.Fatalf("Totals.Entries = %d, want 150", totals.Entries)
	}
	if totals.TotalSeconds != int64(150*durSec) {
		t.Fatalf("Totals.TotalSeconds = %d, want %d", totals.TotalSeconds, 150*durSec)
	}
}

// TestReports_PaginationMeta verifies the structured pagination/limits meta
// replaces the old warning-string shape.
func TestReports_PaginationMeta(t *testing.T) {
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	entries := seedEntries(base, 450, 3600)
	pages := chunkEntries(entries, reportPageSize) // 3 pages (200, 200, 50)

	client, cleanup := newTestClient(t, newPaginatedHandler(t, pages))
	defer cleanup()

	svc := New(client, "ws1")
	svc.ReportMaxEntries = 10000
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start":       "2026-04-01T00:00:00Z",
		"end":         "2026-04-30T00:00:00Z",
		"max_entries": 20000,
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}
	pagination, ok := result.Meta["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured pagination meta, got %T", result.Meta["pagination"])
	}
	if pagination["pages_fetched"].(int) != 3 {
		t.Fatalf("pages_fetched = %v, want 3", pagination["pages_fetched"])
	}
	if pagination["entries_total"].(int) != 450 {
		t.Fatalf("entries_total = %v, want 450", pagination["entries_total"])
	}
	if pagination["page_size"].(int) != reportPageSize {
		t.Fatalf("page_size = %v, want %d", pagination["page_size"], reportPageSize)
	}
	if pagination["applied_page_size"].(int) != reportPageSize {
		t.Fatalf("applied_page_size = %v, want %d", pagination["applied_page_size"], reportPageSize)
	}
	limits, ok := result.Meta["limits"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured limits meta, got %T", result.Meta["limits"])
	}
	if limits["max_entries"].(int) != 10000 {
		t.Fatalf("max_entries = %v, want 10000", limits["max_entries"])
	}
	if limits["requested_max_entries"].(int) != 20000 || limits["applied_max_entries"].(int) != 10000 || limits["clamped"] != true {
		t.Fatalf("unexpected limit clamp meta: %+v", limits)
	}
	if _, exists := result.Meta["warning"]; exists {
		t.Fatalf("legacy warning string must be removed from meta")
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
