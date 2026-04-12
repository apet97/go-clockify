package tools

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestCacheWriteThrough_AddEntry_SkipsReadResource is the W4-04d
// load-bearing assertion: when a subscribed client triggers AddEntry,
// the write-through path feeds the POST response directly into the
// subscription gate without a follow-up GET /time-entries/{id}. In
// Wave 3 (and Wave 4 pre-T-4d), that GET happened on every mutation.
//
// The test counts GETs against the entry endpoint during a single
// subscribed AddEntry. Expected count: zero. Any non-zero count
// would mean emitEntryAndWeeklyWithState silently fell through to
// the ReadResource-based path.
func TestCacheWriteThrough_AddEntry_SkipsReadResource(t *testing.T) {
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

	_, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "fresh",
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
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
			// the post-create state.
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

	// Step 1: create the entry. Write-through primes the cache with
	// billable=false, emits format=none (no prior state).
	if _, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "initial",
		"billable":    false,
		"dry_run":     false,
	}); err != nil {
		t.Fatalf("AddEntry: %v", err)
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
	// Expect: AddEntry → entry format=none + weekly format=none;
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
	}
	patch, ok := entryMergeCall.Delta.Patch.(map[string]any)
	if !ok {
		t.Fatalf("patch is not an object: %T", entryMergeCall.Delta.Patch)
	}
	if patch["billable"] != true {
		t.Fatalf("merge patch missing billable=true: %+v", patch)
	}
}
