package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// recordingEmit captures every call to Service.EmitResourceUpdate so
// tests can assert which URIs fired and what envelope shape they
// carried. Thread-safe so concurrent mutation tests do not race.
type recordingEmit struct {
	mu    sync.Mutex
	calls []recordingEmitCall
}

type recordingEmitCall struct {
	URI   string
	Delta mcp.ResourceUpdateDelta
}

func (r *recordingEmit) hook() func(string, mcp.ResourceUpdateDelta) {
	return func(uri string, delta mcp.ResourceUpdateDelta) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, recordingEmitCall{URI: uri, Delta: delta})
	}
}

func (r *recordingEmit) snapshot() []recordingEmitCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordingEmitCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestAddEntryEmitsFirstNotificationAsFormatNone covers the W3-03d
// wiring on AddEntry: the first notification for a previously-unseen
// URI arrives as format=none because there is no cached state to diff
// against. The handler fetches the new entry's resource view and
// populates the cache so the *next* mutation emits a proper merge
// patch.
func TestAddEntryEmitsFirstNotificationAsFormatNone(t *testing.T) {
	const entryID = "e1"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-entries"):
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "first",
				"billable":    false,
				"projectId":   "",
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:00:00Z",
					"duration": "PT1H",
				},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID):
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "first",
				"billable":    false,
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()

	_, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-11T10:00:00Z",
		"end":         "2026-04-11T11:00:00Z",
		"description": "first",
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	// W4-04b: AddEntry now emits the concrete entry URI AND the weekly-
	// report URI for the ISO week containing the entry's start. The
	// entry on 2026-04-11 (Saturday) falls in the week starting
	// 2026-04-06 (Monday).
	calls := emit.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 emits (entry + weekly-report), got %d: %+v", len(calls), calls)
	}
	wantEntry := "clockify://workspace/" + wsID + "/entry/" + entryID
	wantWeekly := "clockify://workspace/" + wsID + "/report/weekly/2026-04-06"
	if calls[0].URI != wantEntry {
		t.Fatalf("first URI = %q, want %q", calls[0].URI, wantEntry)
	}
	if calls[0].Delta.Format != "none" {
		t.Fatalf("first emit should be format=none, got %q", calls[0].Delta.Format)
	}
	if calls[1].URI != wantWeekly {
		t.Fatalf("second URI = %q, want %q", calls[1].URI, wantWeekly)
	}
}

// TestUpdateEntryEmitsMergePatchOnCachedURI verifies that when the
// cache already holds a prior serialisation of the entry, a subsequent
// mutation publishes a minimal RFC 7396 merge patch instead of
// format=none. The cache state is synthesised so the test only depends
// on the tools layer, not the preceding AddEntry path.
func TestUpdateEntryEmitsMergePatchOnCachedURI(t *testing.T) {
	const entryID = "e2"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// PUT updates: respond with the new state.
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "after",
				"billable":    true,
				"projectId":   "p1",
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:30:00Z",
					"duration": "PT1H30M",
				},
			})
			return
		}
		// GETs for both the initial fetch and the post-emit re-read.
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			respondJSON(t, w, map[string]any{
				"id":          entryID,
				"description": "after",
				"billable":    true,
				"projectId":   "p1",
				"timeInterval": map[string]any{
					"start":    "2026-04-11T10:00:00Z",
					"end":      "2026-04-11T11:30:00Z",
					"duration": "PT1H30M",
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

	// Seed the cache with a "before" snapshot — this is what a prior
	// subscription would have captured.
	prior := map[string]any{
		"id":          entryID,
		"description": "before",
		"billable":    false,
		"projectId":   "p1",
	}
	priorBytes, _ := json.Marshal(prior)
	uri := "clockify://workspace/" + wsID + "/entry/" + entryID
	svc.resourceCache.put(uri, priorBytes)

	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    entryID,
		"description": "after",
		"billable":    true,
		"dry_run":     false,
	})
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	// W4-04b: the entry-URI emit now has a sibling weekly-report emit.
	// Filter to the entry URI for the existing merge-patch assertion.
	calls := emit.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 emits (entry + weekly-report), got %d: %+v", len(calls), calls)
	}
	var got *recordingEmitCall
	for i := range calls {
		if calls[i].URI == uri {
			got = &calls[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("entry URI %q not emitted; got %+v", uri, calls)
	}
	if got.Delta.Format != "merge" {
		t.Fatalf("format = %q, want merge", got.Delta.Format)
	}
	patch, ok := got.Delta.Patch.(map[string]any)
	if !ok {
		t.Fatalf("patch is not an object: %T", got.Delta.Patch)
	}
	// Minimal patch should only contain the two fields that actually
	// differ (description and billable). timeInterval is new vs the
	// pared-down prior snapshot, so it will appear too.
	if patch["description"] != "after" {
		t.Fatalf("patch missing description=after: %+v", patch)
	}
	if patch["billable"] != true {
		t.Fatalf("patch missing billable=true: %+v", patch)
	}
}

// TestDeleteEntryEmitsFormatDeleted confirms the delete path drops
// the cache entry and publishes format=deleted so subscribed clients
// can remove their cached state without re-fetching into a 404.
func TestDeleteEntryEmitsFormatDeleted(t *testing.T) {
	const entryID = "e3"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"e3","timeInterval":{"start":"2026-04-07T09:00:00Z","end":"2026-04-07T17:00:00Z"}}`))
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/time-entries/"+entryID) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()

	uri := "clockify://workspace/" + wsID + "/entry/" + entryID
	svc.resourceCache.put(uri, []byte(`{"id":"e3"}`))

	_, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": entryID,
		"dry_run":  false,
	})
	if err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}

	calls := emit.snapshot()
	if len(calls) < 1 {
		t.Fatalf("expected at least 1 emit, got %d", len(calls))
	}
	if calls[0].Delta.Format != "deleted" {
		t.Fatalf("format = %q, want deleted", calls[0].Delta.Format)
	}
	if _, stillCached := svc.resourceCache.get(uri); stillCached {
		t.Fatalf("cache should be empty for deleted URI")
	}
}
