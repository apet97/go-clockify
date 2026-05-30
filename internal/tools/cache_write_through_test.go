package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// TestCacheWriteThrough_EntriesCreate_SkipsReadResource is the W4-04d
// load-bearing assertion: when a subscribed client triggers
// clockify_entries_create, the write-through path feeds the POST
// response directly into the subscription gate without a follow-up
// GET /time-entries/{id}. In Wave 3 (and Wave 4 pre-T-4d), that GET
// happened on every mutation.
//
// The test counts GETs against the entry endpoint during a single
// subscribed create. Expected count: zero. Any non-zero count would
// mean emitEntryAndWeeklyWithState silently fell through to the
// ReadResource-based path.
func TestCacheWriteThrough_EntriesCreate_SkipsReadResource(t *testing.T) {
	const entryID = "e-wt1"
	const wsID = "w1"

	var getCount atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			getCount.Add(1)
			respondJSON(t, w, map[string]any{"id": entryID, "description": "stale"})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-entries") {
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "fresh",
				"billable":    false,
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
			return
		}
		http.NotFound(w, r)
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()
	// Subscribe every URI so the gate fires but doesn't short-circuit.
	svc.SubscriptionGate = func(_ string) bool { return true }

	_, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "fresh",
	})
	if err != nil {
		t.Fatalf("EntriesCreate: %v", err)
	}

	if n := getCount.Load(); n != 0 {
		t.Fatalf("expected zero ReadResource GETs (write-through should bypass), got %d", n)
	}

	// At least two emits should still fire — the entry URI (from
	// write-through) and the weekly-report URI (via the fall-through
	// path, which will also succeed because no GET is required for
	// the weekly report: that path still calls ReadResource, but the
	// weekly-report URI dispatches through WeeklySummary, not
	// /time-entries/{id}).
	calls := emit.snapshot()
	if len(calls) < 1 {
		t.Fatalf("expected at least 1 emit from write-through path, got %d", len(calls))
	}
	var sawEntry bool
	for _, c := range calls {
		if strings.Contains(c.URI, "/entry/"+entryID) {
			sawEntry = true
			break
		}
	}
	if !sawEntry {
		t.Fatalf("write-through did not emit the entry URI; calls=%+v", calls)
	}
}

// TestCacheWriteThrough_PrimesCacheForMergePatch verifies that the
// write-through path writes to the cache so a subsequent mutation
// emits a proper RFC 7396 merge patch (format=merge) instead of
// format=none. In Wave 3 the first mutation always emits format=none
// because ReadResource populates the cache. With write-through, the
// caller's payload is the authoritative initial state — which means
// the SECOND mutation can already produce a minimal patch.
func TestCacheWriteThrough_PrimesCacheForMergePatch(t *testing.T) {
	const entryID = "e-wt2"
	const wsID = "w1"

	var callCount atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// First POST: create with billable=false.
		// Follow-up GET fetch-for-update mirrors the current state.
		// PUT updates to billable=true.
		path := r.URL.Path
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/time-entries"):
			callCount.Add(1)
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "initial",
				"billable":    false,
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/time-entries/"+entryID):
			// Fetched by UpdateEntry's pre-update GET step — returns
			// the post-create state. userId is populated so the
			// ownership guard sees a match against the pre-primed
			// cachedUser below; the live Clockify API always returns
			// userId on GET.
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"userId":      "u-self",
				"description": "initial",
				"billable":    false,
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
		case r.Method == http.MethodPut && strings.HasSuffix(path, "/time-entries/"+entryID):
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "initial",
				"billable":    true,
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()
	svc.SubscriptionGate = func(_ string) bool { return true }
	// Pre-prime the current-user cache so UpdateEntry's ownership
	// guard does not HTTP-fetch /user. The test focuses on the
	// merge-patch cache-priming behaviour, not auth resolution.
	svc.identity.cachedUser = &clockify.User{ID: "u-self"}
	if _, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "initial",
		"billable":    false,
	}); err != nil {
		t.Fatalf("EntriesCreate: %v", err)
	}

	// Step 2: flip billable to true. This time the cache holds the
	// previous write-through state, so emit should produce
	// format=merge with {"billable": true} (the only changed field).
	if _, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id": entryID,
		"billable": true,
		"dry_run":  false,
	}); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	calls := emit.snapshot()
	// Expect: EntriesCreate → entry format=none + weekly format=none;
	//        UpdateEntry → entry format=merge + weekly format=? (none
	//        on first emit for that URI).
	var entryMergeCall *recordingEmitCall
	for i := range calls {
		if strings.Contains(calls[i].URI, "/entry/"+entryID) && calls[i].Delta.Format == "merge" {
			entryMergeCall = &calls[i]
			break
		}
	}
	if entryMergeCall == nil {
		t.Fatalf("expected at least one entry-URI emit with format=merge after UpdateEntry; calls=%+v", calls)
		return
	}
	patch, ok := entryMergeCall.Delta.Patch.(map[string]any)
	if !ok {
		t.Fatalf("patch is not an object: %T", entryMergeCall.Delta.Patch)
	}
	if patch["billable"] != true {
		t.Fatalf("merge patch missing billable=true: %+v", patch)
	}
}

func TestWeeklyReportDeltaFromCachedState(t *testing.T) {
	const wsID = "w1"
	weeklyURI := weeklyReportResourceURI(wsID, "2026-04-06")
	initialEntry := weeklyDeltaEntry("e1", "p1", "Alpha", "2026-04-06T09:00:00Z", "2026-04-06T10:00:00Z")

	tests := []struct {
		name              string
		before            *clockify.TimeEntry
		after             *clockify.TimeEntry
		wantEntries       int
		wantSeconds       int64
		wantProjectID     string
		wantProjectCount  int
		wantDay           string
		wantDayCount      int
		wantSuggestedTool string
	}{
		{
			name:              "add",
			after:             ptrTimeEntry(weeklyDeltaEntry("e2", "p2", "Beta", "2026-04-07T11:00:00Z", "2026-04-07T13:00:00Z")),
			wantEntries:       2,
			wantSeconds:       3 * 3600,
			wantProjectID:     "p2",
			wantProjectCount:  2,
			wantDay:           "2026-04-07",
			wantDayCount:      2,
			wantSuggestedTool: "clockify_entries_list",
		},
		{
			name:              "update",
			before:            ptrTimeEntry(initialEntry),
			after:             ptrTimeEntry(weeklyDeltaEntry("e1", "p2", "Beta", "2026-04-07T11:00:00Z", "2026-04-07T13:00:00Z")),
			wantEntries:       1,
			wantSeconds:       2 * 3600,
			wantProjectID:     "p2",
			wantProjectCount:  1,
			wantDay:           "2026-04-07",
			wantDayCount:      1,
			wantSuggestedTool: "clockify_entries_list",
		},
		{
			name:              "delete",
			before:            ptrTimeEntry(initialEntry),
			wantEntries:       0,
			wantSeconds:       0,
			wantProjectCount:  0,
			wantDayCount:      0,
			wantSuggestedTool: oneUserToolLogWork,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(nil, wsID)
			emit := &recordingEmit{}
			svc.EmitResourceUpdate = emit.hook()
			svc.SubscriptionGate = func(_ string) bool { return true }
			svc.resources.cache.put(weeklyURI, mustMarshalWeeklyData(t, cachedWeeklyReportFixture()))

			svc.emitWeeklyReportsForEntryChange(context.Background(), wsID, tt.before, tt.after)

			calls := emit.snapshot()
			if len(calls) != 1 || calls[0].URI != weeklyURI {
				t.Fatalf("expected one weekly emit for %s, got %+v", weeklyURI, calls)
			}
			data := mustCachedWeeklyData(t, svc, weeklyURI)
			if data.Totals.Entries != tt.wantEntries || data.Totals.TotalSeconds != tt.wantSeconds {
				t.Fatalf("totals = %+v, want entries=%d seconds=%d", data.Totals, tt.wantEntries, tt.wantSeconds)
			}
			if len(data.ByProject) != tt.wantProjectCount {
				t.Fatalf("ByProject = %+v, want count %d", data.ByProject, tt.wantProjectCount)
			}
			if tt.wantProjectID != "" && data.ByProject[0].ProjectID != tt.wantProjectID {
				t.Fatalf("top project = %+v, want %s", data.ByProject[0], tt.wantProjectID)
			}
			if len(data.ByDay) != tt.wantDayCount {
				t.Fatalf("ByDay = %+v, want count %d", data.ByDay, tt.wantDayCount)
			}
			if tt.wantDay != "" && data.ByDay[len(data.ByDay)-1].Date != tt.wantDay {
				t.Fatalf("last day = %+v, want %s", data.ByDay, tt.wantDay)
			}
			assertSuggestionTool(t, data.SuggestedActions, tt.wantSuggestedTool)
		})
	}
}

func BenchmarkEntryMutationWeeklyEmit_CachedDelta500Entries(b *testing.B) {
	const wsID = "w1"
	weeklyURI := weeklyReportResourceURI(wsID, "2026-04-06")
	weeklyState := cachedWeeklyReportFixtureN(500)
	raw, err := json.Marshal(weeklyState)
	if err != nil {
		b.Fatalf("marshal weekly fixture: %v", err)
	}
	after := weeklyDeltaEntry("new-entry", "p-extra", "Extra", "2026-04-10T09:00:00Z", "2026-04-10T10:00:00Z")

	svc := New(nil, wsID)
	svc.EmitResourceUpdate = func(string, mcp.ResourceUpdateDelta) {}
	svc.SubscriptionGate = func(_ string) bool { return true }

	b.ReportAllocs()
	for b.Loop() {
		svc.resources.cache.put(weeklyURI, raw)
		svc.emitWeeklyReportsForEntryChange(context.Background(), wsID, nil, &after)
	}
}

func cachedWeeklyReportFixture() WeeklySummaryData {
	return WeeklySummaryData{
		Range:  DateRange{Start: "2026-04-06T00:00:00Z", End: "2026-04-13T00:00:00Z"},
		Totals: SummaryTotals{Entries: 1, TotalSeconds: 3600, TotalHours: 1},
		ByDay: []DaySummary{
			{Date: "2026-04-06", Entries: 1, TotalSeconds: 3600, TotalHours: 1},
		},
		ByProject: []ProjectSummary{
			{ProjectID: "p1", ProjectName: "Alpha", Entries: 1, TotalSeconds: 3600, TotalHours: 1},
		},
		UnassignedKey: "(no project)",
	}
}

func cachedWeeklyReportFixtureN(entries int) WeeklySummaryData {
	if entries <= 0 {
		return WeeklySummaryData{
			Range:         DateRange{Start: "2026-04-06T00:00:00Z", End: "2026-04-13T00:00:00Z"},
			UnassignedKey: "(no project)",
		}
	}
	data := WeeklySummaryData{
		Range:         DateRange{Start: "2026-04-06T00:00:00Z", End: "2026-04-13T00:00:00Z"},
		UnassignedKey: "(no project)",
	}
	projectTotals := map[string]*ProjectSummary{}
	dayTotals := map[string]*DaySummary{}
	for i := range entries {
		projectID := "p-alpha"
		projectName := "Alpha"
		switch i % 3 {
		case 1:
			projectID = "p-beta"
			projectName = "Beta"
		case 2:
			projectID = "p-gamma"
			projectName = "Gamma"
		}
		day := fmt.Sprintf("2026-04-%02d", 6+(i%5))
		if _, ok := projectTotals[projectID]; !ok {
			projectTotals[projectID] = &ProjectSummary{ProjectID: projectID, ProjectName: projectName}
		}
		projectTotals[projectID].Entries++
		projectTotals[projectID].TotalSeconds += 3600
		if _, ok := dayTotals[day]; !ok {
			dayTotals[day] = &DaySummary{Date: day}
		}
		dayTotals[day].Entries++
		dayTotals[day].TotalSeconds += 3600
		data.Totals.Entries++
		data.Totals.TotalSeconds += 3600
	}
	data.Totals.TotalHours = hours(data.Totals.TotalSeconds)
	for _, project := range projectTotals {
		project.TotalHours = hours(project.TotalSeconds)
		data.ByProject = append(data.ByProject, *project)
	}
	for _, day := range dayTotals {
		day.TotalHours = hours(day.TotalSeconds)
		data.ByDay = append(data.ByDay, *day)
	}
	sortProjectSummaries(data.ByProject)
	sortDaySummaries(data.ByDay)
	start := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	data.SuggestedActions = reportSuggestedActions(data.ByProject, data.Totals, start, end)
	return data
}

func weeklyDeltaEntry(id, projectID, projectName, start, end string) clockify.TimeEntry {
	return clockify.TimeEntry{
		ID:          id,
		Description: "cached delta",
		ProjectID:   projectID,
		ProjectName: projectName,
		TimeInterval: clockify.TimeInterval{
			Start: start,
			End:   end,
		},
	}
}

func ptrTimeEntry(entry clockify.TimeEntry) *clockify.TimeEntry {
	return &entry
}

func mustMarshalWeeklyData(t *testing.T, data WeeklySummaryData) []byte {
	t.Helper()
	out, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal weekly fixture: %v", err)
	}
	return out
}

func mustCachedWeeklyData(t *testing.T, svc *Service, uri string) WeeklySummaryData {
	t.Helper()
	raw, ok := svc.resources.cache.get(uri)
	if !ok {
		t.Fatalf("missing cached weekly state for %s", uri)
	}
	var data WeeklySummaryData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode cached weekly state: %v", err)
	}
	return data
}
