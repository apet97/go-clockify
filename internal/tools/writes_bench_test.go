package tools_test

// Per-tool micro-benchmarks for the Tier-1 destructive write surface.
//
// The generic dispatch benches in internal/mcp (BenchmarkDispatchToolsList,
// BenchmarkDispatchInitialize) measure the JSON-RPC parse → route → serialize
// floor against a synthetic no-op handler. They cannot detect a regression
// inside any specific Tier-1 write handler — a 30% slowdown in
// UpdateEntry's fetch-then-merge patching, or an extra allocation per call
// in LogTime's payload assembly, would be invisible to those benches and
// invisible to the weekly bench-regression gate that consumes them.
//
// The benches below put one call per Go iteration through the FULL pipeline:
//
//   testharness.Invoke
//     → mcp.NewServer + enforcement.Pipeline (Standard policy)
//     → svc.<Handler>
//       → clockify.Client.Post/Get/Put/Patch
//         → httptest loopback (this file's stubClockify handler)
//
// Every iteration constructs a fresh tools/call request body so allocation
// numbers are not artificially deflated by request reuse. The upstream is
// a single httptest.Server stood up in the bench setup helper; it returns
// minimal canned JSON for every Clockify endpoint the six handlers touch
// and counts no logic — the bench is about dispatch + handler overhead,
// not upstream latency.
//
// Run locally:
//
//	go test -bench=BenchmarkClockify -benchmem -benchtime=1s -count=1 \
//	  -run='^$' ./internal/tools/...

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
)

// stubClockifyForWrites returns a fake upstream that satisfies every HTTP
// path the six Tier-1 write handlers touch with the smallest possible
// JSON payload. The handler is intentionally branch-light so the bench's
// upstream cost stays sub-millisecond on a laptop.
func stubClockifyForWrites(b *testing.B) *testharness.FakeClockify {
	b.Helper()
	const userJSON = `{"id":"u-bench","email":"bench@example.test","name":"bench"}`
	// One canonical entry shape reused across all responses. Includes a
	// timeInterval so UpdateEntry's merge path has a populated source
	// struct to copy from.
	const entryJSON = `{"id":"e-bench","description":"bench","workspaceId":"test-workspace",` +
		`"projectId":"p-bench","billable":false,` +
		`"timeInterval":{"start":"2026-01-01T09:00:00Z","end":"2026-01-01T10:00:00Z","duration":"PT1H"}}`
	const entryListJSON = `[` + entryJSON + `]`

	return testharness.NewFakeClockify(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		// /user (current-user resolver used by StopTimer + listEntriesWithQuery).
		case path == "/user" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(userJSON))
		// GET single entry (UpdateEntry pre-fetch).
		case strings.Contains(path, "/time-entries/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(entryJSON))
		// PUT single entry (UpdateEntry / FindAndUpdateEntry write).
		case strings.Contains(path, "/time-entries/") && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(entryJSON))
		// PATCH user time-entries collection (StopTimer).
		case strings.HasSuffix(path, "/time-entries") && r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(entryJSON))
		// POST workspace time-entries collection (StartTimer / LogTime / AddEntry).
		case strings.HasSuffix(path, "/time-entries") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(entryJSON))
		// GET user time-entries list (FindAndUpdateEntry search).
		case strings.HasSuffix(path, "/time-entries") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(entryListJSON))
		default:
			http.NotFound(w, r)
		}
	}))
}

// invokeWriteBench runs the supplied tools/call through the full harness
// once per benchmark iteration. The args map is rebuilt every iteration so
// allocation accounting reflects the per-request cost the way it would in
// production traffic — reusing one map across iterations would understate
// the dispatcher's real allocation floor.
//
// The clockify.Client is constructed once per benchmark and threaded
// through InvokeOpts.Client so its keep-alive transport is reused across
// all iterations. Constructing a fresh client per iteration burns an
// ephemeral TCP port per upstream call (each handler issues 1–3 calls);
// the OS exhausts the loopback port range within a few thousand
// iterations and the bench starts failing with "can't assign requested
// address" before the timer ever runs to convergence.
func invokeWriteBench(b *testing.B, tool string, argsFn func() map[string]any) {
	b.Helper()
	upstream := stubClockifyForWrites(b)
	client := clockify.NewClient("test-api-key", upstream.URL(), 5*time.Second, 0)
	b.Cleanup(client.Close)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result := testharness.Invoke(b, testharness.InvokeOpts{
			Tool:       tool,
			Args:       argsFn(),
			PolicyMode: policy.Standard,
			Upstream:   upstream,
			Client:     client,
		})
		// Sanity check prevents the compiler from eliminating the call,
		// and surfaces handler-side regressions (e.g. an upstream stub
		// path drift) as a hard fail rather than a misleading "fast"
		// number.
		if result.Outcome != testharness.OutcomeSuccess {
			b.Fatalf("%s: outcome=%q err=%q raw=%s",
				tool, result.Outcome, result.ErrorMessage, string(result.Raw))
		}
	}
}

// BenchmarkClockifyLogTime measures the workflows.go LogTime path:
// validate args → parse start/end → POST /workspaces/{ws}/time-entries.
// One upstream HTTP request per iteration.
func BenchmarkClockifyLogTime(b *testing.B) {
	invokeWriteBench(b, "clockify_log_time", func() map[string]any {
		return map[string]any{
			"start":       "2026-01-01T09:00:00Z",
			"end":         "2026-01-01T10:00:00Z",
			"description": "bench",
		}
	})
}

// BenchmarkClockifyStartTimer measures the timer.go StartTimer path:
// resolve workspace → POST /workspaces/{ws}/time-entries → emit
// resource-update. One upstream HTTP request per iteration.
func BenchmarkClockifyStartTimer(b *testing.B) {
	invokeWriteBench(b, "clockify_start_timer", func() map[string]any {
		return map[string]any{
			"description": "bench",
		}
	})
}

// BenchmarkClockifyStopTimer measures the timer.go StopTimer path:
// resolve workspace → GET /user → PATCH user time-entries collection.
// Two upstream HTTP requests per iteration (no per-Service caching across
// invocations because the harness builds a fresh Service every call).
func BenchmarkClockifyStopTimer(b *testing.B) {
	invokeWriteBench(b, "clockify_stop_timer", func() map[string]any {
		return map[string]any{}
	})
}

// BenchmarkClockifyAddEntry measures the entries.go AddEntry path:
// validate start → optional project resolve → POST workspace
// time-entries. One upstream HTTP request per iteration.
func BenchmarkClockifyAddEntry(b *testing.B) {
	invokeWriteBench(b, "clockify_add_entry", func() map[string]any {
		return map[string]any{
			"start":       "2026-01-01T09:00:00Z",
			"end":         "2026-01-01T10:00:00Z",
			"description": "bench",
		}
	})
}

// BenchmarkClockifyUpdateEntry measures the entries.go UpdateEntry path:
// GET existing entry → merge args over fetched fields → PUT full payload.
// Two upstream HTTP requests per iteration plus the field-level diff loop
// that is the most regression-prone part of the handler.
func BenchmarkClockifyUpdateEntry(b *testing.B) {
	invokeWriteBench(b, "clockify_update_entry", func() map[string]any {
		// new_description (delivered as "description") forces the merge
		// branch to fire so the bench measures the patching cost, not
		// just the no-op fast path.
		return map[string]any{
			"entry_id":    "e-bench",
			"description": "bench-updated",
		}
	})
}

// BenchmarkClockifyFindAndUpdateEntry measures the workflows.go
// FindAndUpdateEntry path: parse find/update args → GET /user → GET
// user time-entries list → in-memory match → PUT updated entry.
// Three upstream HTTP requests per iteration, the most expensive of the
// six. Still <1ms per iteration on a laptop because the upstream stub
// returns a one-entry list and the matching loop runs once.
func BenchmarkClockifyFindAndUpdateEntry(b *testing.B) {
	invokeWriteBench(b, "clockify_find_and_update_entry", func() map[string]any {
		return map[string]any{
			"entry_id":        "e-bench",
			"new_description": "bench-updated",
		}
	})
}
