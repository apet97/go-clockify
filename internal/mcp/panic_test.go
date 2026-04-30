package mcp

import (
	"sync"
	"testing"
)

// TestRecoverDispatch_StableErrorEnvelope locks the contract that
// every transport relies on: a recovered panic produces a JSON-RPC
// 2.0 response with the request ID echoed, isError=true, and the
// fixed user-facing string. Drift here would split stdio,
// streamable HTTP, and gRPC error envelopes apart.
func TestRecoverDispatch_StableErrorEnvelope(t *testing.T) {
	var captured Response
	var calls int
	sink := func(r Response) {
		calls++
		captured = r
	}

	func() {
		defer RecoverDispatch(42, "test_site", "test_tool", sink)
		panic("boom containing secret")
	}()

	if calls != 1 {
		t.Fatalf("sink called %d times, expected 1", calls)
	}
	if captured.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", captured.JSONRPC)
	}
	if id, ok := captured.ID.(int); !ok || id != 42 {
		t.Errorf("ID = %v (%T), want int 42", captured.ID, captured.ID)
	}
	result, ok := captured.Result.(map[string]any)
	if !ok {
		t.Fatalf("Result is not map[string]any: %T", captured.Result)
	}
	if result["isError"] != true {
		t.Errorf("isError = %v, want true", result["isError"])
	}
	content, ok := result["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content is not []map[string]any: %T", result["content"])
	}
	if len(content) != 1 || content[0]["text"] != "internal tool error; request logged" {
		t.Errorf("content text = %v, want generic 'internal tool error; request logged'", content)
	}
}

// TestRecoverDispatch_NoPanic_Passthrough verifies the helper is a
// no-op when the dispatch returns normally — sink must never be
// invoked, otherwise non-panicking requests would emit stray
// tool-error responses.
func TestRecoverDispatch_NoPanic_Passthrough(t *testing.T) {
	calls := 0
	sink := func(Response) { calls++ }

	func() {
		defer RecoverDispatch(1, "test_site", "tool", sink)
		// no panic
	}()

	if calls != 0 {
		t.Fatalf("sink invoked %d times on normal return; expected 0", calls)
	}
}

// TestRecoverDispatch_NilSink documents the helper tolerating a
// nil sink: the metric/log emission still runs (operator
// observability) but no transport-side write is attempted. Useful
// for tests and embedders that just want the side effects.
func TestRecoverDispatch_NilSink(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("RecoverDispatch let panic escape with nil sink: %v", rec)
		}
	}()
	func() {
		defer RecoverDispatch("req-1", "test_site", "tool", nil)
		panic("boom")
	}()
}

// TestRecoverDispatch_ConcurrentSafety smokes the helper under
// concurrent panics — the metric counter and slog logger are both
// already concurrency-safe, but the test guards against an
// accidental shared-state regression in the response builder.
func TestRecoverDispatch_ConcurrentSafety(t *testing.T) {
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			func() {
				defer RecoverDispatch(id, "concurrent_site", "tool", func(Response) {})
				panic("concurrent boom")
			}()
		}(i)
	}
	wg.Wait()
}
