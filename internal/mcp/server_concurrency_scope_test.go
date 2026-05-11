package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDispatchMessage_BypassesStdioSemaphore pins that the stdio
// dispatch-layer semaphore (toolCallSem) only gates Run — the
// DispatchMessage path used by streamable HTTP and the
// DispatchMessageWithRecover path used by gRPC must run concurrent
// tools/call frames without acquiring it. If a future refactor wired
// the semaphore into DispatchMessage, the documented stdio-only
// contract in docs/performance.md would silently change for hosted
// deployments; this test goes RED on that regression.
func TestDispatchMessage_BypassesStdioSemaphore(t *testing.T) {
	const callers = 8
	const semCap = 1

	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64
	release := make(chan struct{})
	entered := make(chan struct{}, callers)

	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		cur := concurrent.Add(1)
		defer concurrent.Add(-1)
		for {
			prev := maxConcurrent.Load()
			if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		return map[string]any{"ok": true}, nil
	}

	descriptors := []ToolDescriptor{{
		Tool: Tool{
			Name:        "block",
			Description: "block",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler:      handler,
		ReadOnlyHint: true,
	}}

	srv := NewServer("test", descriptors, nil, nil)
	srv.initialized.Store(true)
	srv.MaxInFlightToolCalls = semCap
	srv.toolCallSem = make(chan struct{}, semCap) // simulate Run having allocated it

	var wg sync.WaitGroup
	for i := 1; i <= callers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := fmt.Appendf(nil, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"block","arguments":{}}}`, id)
			if _, err := srv.DispatchMessage(context.Background(), msg); err != nil {
				t.Errorf("dispatch %d: %v", id, err)
			}
		}(i)
	}

	// Wait until at least 2 handlers are concurrently in flight — that
	// alone proves the cap (semCap=1) was not enforced. Use a generous
	// timeout so a slow CI machine doesn't false-fail.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for got := 0; got < 2; {
		select {
		case <-entered:
			got++
		case <-deadline.C:
			t.Fatalf("only %d handler(s) reached the body within 3s — the stdio semaphore is gating DispatchMessage", got)
		}
	}

	close(release)

	doneAll := make(chan struct{})
	go func() { wg.Wait(); close(doneAll) }()
	select {
	case <-doneAll:
	case <-time.After(3 * time.Second):
		t.Fatal("DispatchMessage callers did not finish in 3s")
	}

	if got := maxConcurrent.Load(); got < 2 {
		t.Fatalf("expected at least 2 concurrent handlers under DispatchMessage, observed max=%d", got)
	}
	if got := int64(callers); maxConcurrent.Load() < got {
		// Not strictly required for the contract, but useful for log
		// triage when this test surfaces in CI: report the actual peak.
		t.Logf("max concurrent dispatch handlers: %d (callers=%d)", maxConcurrent.Load(), got)
	}
}
