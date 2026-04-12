package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestIsoWeekStart verifies the Monday-00:00 anchor matches what the
// reports weekBounds() function already computes. Keeping the two in
// sync is load-bearing because they both key the same weekly-report
// resource URI namespace.
func TestIsoWeekStart(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name string
		in   time.Time
		want string // YYYY-MM-DD of the ISO Monday
	}{
		{"monday_itself", time.Date(2026, 4, 6, 9, 0, 0, 0, utc), "2026-04-06"},
		{"tuesday_same_week", time.Date(2026, 4, 7, 0, 0, 0, 0, utc), "2026-04-06"},
		{"saturday_same_week", time.Date(2026, 4, 11, 22, 30, 0, 0, utc), "2026-04-06"},
		{"sunday_23_59_same_week", time.Date(2026, 4, 12, 23, 59, 0, 0, utc), "2026-04-06"},
		{"next_monday_rolls_over", time.Date(2026, 4, 13, 0, 0, 0, 0, utc), "2026-04-13"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isoWeekStart(c.in, utc).Format("2006-01-02")
			if got != c.want {
				t.Fatalf("isoWeekStart(%s) = %s, want %s", c.in.Format(time.RFC3339), got, c.want)
			}
		})
	}
}

// TestWeeklyReportURIsForEntry_SingleWeek covers the common shape —
// start and end in the same ISO week produce exactly one URI.
func TestWeeklyReportURIsForEntry_SingleWeek(t *testing.T) {
	got := weeklyReportURIsForEntry("w1",
		"2026-04-11T10:00:00Z",
		"2026-04-11T11:30:00Z",
		time.UTC,
	)
	want := []string{"clockify://workspace/w1/report/weekly/2026-04-06"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestWeeklyReportURIsForEntry_CrossWeek covers the rare but valid
// shape — an entry that starts Sunday 23:00 (in week N) and ends
// Monday 01:00 (in week N+1) invalidates both weekly reports.
func TestWeeklyReportURIsForEntry_CrossWeek(t *testing.T) {
	got := weeklyReportURIsForEntry("w1",
		"2026-04-12T23:00:00Z", // Sunday 2026-04-12 → week of 2026-04-06
		"2026-04-13T01:00:00Z", // Monday 2026-04-13 → week of 2026-04-13
		time.UTC,
	)
	if len(got) != 2 {
		t.Fatalf("expected 2 URIs on cross-week span, got %d: %v", len(got), got)
	}
	// Order: startWeek first, endWeek second.
	if !strings.Contains(got[0], "2026-04-06") {
		t.Fatalf("first URI should be start-week 2026-04-06, got %q", got[0])
	}
	if !strings.Contains(got[1], "2026-04-13") {
		t.Fatalf("second URI should be end-week 2026-04-13, got %q", got[1])
	}
}

// TestWeeklyReportURIsForEntry_RunningTimer covers the running-timer
// shape — end is empty so only the start week is returned.
func TestWeeklyReportURIsForEntry_RunningTimer(t *testing.T) {
	got := weeklyReportURIsForEntry("w1",
		"2026-04-11T10:00:00Z",
		"",
		time.UTC,
	)
	if len(got) != 1 {
		t.Fatalf("running timer: expected 1 URI, got %d: %v", len(got), got)
	}
}

// TestWeeklyReportURIsForEntry_BadInputsDegrade covers the safety-net
// contract — any parsing failure returns nil so the mutation handler's
// primary entry-URI emit path is not blocked by a weekly-report edge
// case.
func TestWeeklyReportURIsForEntry_BadInputsDegrade(t *testing.T) {
	cases := []struct {
		name, ws, start, end string
	}{
		{"empty_workspace", "", "2026-04-11T10:00:00Z", ""},
		{"empty_start", "w1", "", ""},
		{"malformed_start", "w1", "not a timestamp", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := weeklyReportURIsForEntry(c.ws, c.start, c.end, time.UTC)
			if got != nil {
				t.Fatalf("expected nil, got %v", got)
			}
		})
	}
}

// TestAddEntry_CrossWeekSpanEmitsBothWeeklyReports is the end-to-end
// assertion that a mutation producing a two-week-spanning entry fires
// three notifications: the entry URI and both weekly-report URIs.
func TestAddEntry_CrossWeekSpanEmitsBothWeeklyReports(t *testing.T) {
	const entryID = "xw1"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-entries"):
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "boundary",
				"timeInterval": map[string]any{
					"start":    "2026-04-12T23:00:00Z",
					"end":      "2026-04-13T01:00:00Z",
					"duration": "PT2H",
				},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID):
			respondJSON(t, w, map[string]any{"id": entryID, "description": "boundary"})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()

	_, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-12T23:00:00Z",
		"end":         "2026-04-13T01:00:00Z",
		"description": "boundary",
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	calls := emit.snapshot()
	if len(calls) != 3 {
		t.Fatalf("expected 3 emits (entry + 2 weekly-report), got %d: %+v", len(calls), calls)
	}
	// Assertion: the set of URIs is exactly {entry, weekly(N), weekly(N+1)}.
	want := map[string]bool{
		"clockify://workspace/" + wsID + "/entry/" + entryID:         true,
		"clockify://workspace/" + wsID + "/report/weekly/2026-04-06": true,
		"clockify://workspace/" + wsID + "/report/weekly/2026-04-13": true,
	}
	for _, c := range calls {
		if !want[c.URI] {
			t.Fatalf("unexpected URI emitted: %q (calls: %+v)", c.URI, calls)
		}
		delete(want, c.URI)
	}
	if len(want) != 0 {
		t.Fatalf("missing expected URIs: %v", want)
	}
}
