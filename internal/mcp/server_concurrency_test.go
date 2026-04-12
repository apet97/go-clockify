package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestStdioDispatch_BoundedConcurrency verifies the dispatch-layer
// semaphore caps in-flight tools/call goroutines before they are spawned.
func TestStdioDispatch_BoundedConcurrency(t *testing.T) {
	const limit = 3
	const total = 10

	var inflight, maxSeen atomic.Int64
	block := make(chan struct{})
	started := make(chan struct{}, total)

	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		cur := inflight.Add(1)
		defer inflight.Add(-1)
		for {
			prev := maxSeen.Load()
			if cur <= prev || maxSeen.CompareAndSwap(prev, cur) {
				break
			}
		}
		started <- struct{}{}
		<-block
		return map[string]any{"ok": true}, nil
	}

	descriptors := []ToolDescriptor{{
		Tool: Tool{
			Name:        "slow",
			Description: "slow",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler:      handler,
		ReadOnlyHint: true,
	}}

	srv := NewServer("test", descriptors, nil, nil)
	srv.MaxInFlightToolCalls = limit

	var input bytes.Buffer
	input.WriteString(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}` + "\n")
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"slow","arguments":{}}}`+"\n", i)
	}

	// Keep the input open so Run does not EOF-exit while handlers block.
	// A pipe with buffered writes lets Run's scanner observe EOF only after
	// we explicitly close it.
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.Copy(pw, &input)
		// Do not close yet — we close once handlers are released.
	}()

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, pr, syncWriter{&output}) }()

	// Wait until exactly `limit` handlers are in flight.
	for i := 0; i < limit; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for handler %d to start", i+1)
		}
	}
	// Give the scheduler a moment to confirm a (limit+1)-th handler did not slip in.
	time.Sleep(100 * time.Millisecond)
	if got := maxSeen.Load(); got > int64(limit) {
		t.Fatalf("expected max in-flight <= %d, got %d (before release)", limit, got)
	}

	// Release all blocked handlers so the remainder can run.
	close(block)

	// Drain the remaining started signals.
	for i := limit; i < total; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for handler %d to start (after release)", i+1)
		}
	}

	// Close the pipe so Run's scanner sees EOF and exits.
	_ = pw.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after EOF")
	}

	if got := maxSeen.Load(); got > int64(limit) {
		t.Fatalf("final max in-flight %d exceeded limit %d", got, limit)
	}

	// Sanity: all 10 tools/call responses should have been written.
	count := strings.Count(output.String(), `"jsonrpc":"2.0"`)
	if count < total+1 { // +1 for initialize
		t.Fatalf("expected at least %d responses, got %d in: %s", total+1, count, output.String())
	}
}

// TestStdioDispatch_ContextCancelReleases verifies that cancelling the
// context while dispatch is saturated unblocks Run rather than deadlocking.
func TestStdioDispatch_ContextCancelReleases(t *testing.T) {
	const limit = 2
	const total = 5

	blockForever := make(chan struct{})
	started := make(chan struct{}, total)
	defer close(blockForever) // release any lingering handlers on cleanup

	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		started <- struct{}{}
		select {
		case <-blockForever:
		case <-ctx.Done():
		}
		return map[string]any{"ok": true}, nil
	}

	descriptors := []ToolDescriptor{{
		Tool: Tool{
			Name:        "hang",
			Description: "hang",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler:      handler,
		ReadOnlyHint: true,
	}}

	srv := NewServer("test", descriptors, nil, nil)
	srv.MaxInFlightToolCalls = limit

	var input bytes.Buffer
	input.WriteString(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}` + "\n")
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"hang","arguments":{}}}`+"\n", i)
	}

	pr, pw := io.Pipe()
	go func() {
		_, _ = io.Copy(pw, &input)
		// leave open to keep Run scanning
	}()
	defer pw.Close()

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, pr, syncWriter{&output}) }()

	// Wait until `limit` handlers are stuck.
	for i := 0; i < limit; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for handler %d to start", i+1)
		}
	}

	// Cancel context — Run should exit even though the dispatch loop is
	// waiting for a semaphore slot (and handlers themselves are blocked).
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit within 3s of ctx cancel — deadlock suspected")
	}
}

// TestStdioDispatch_Unlimited verifies MaxInFlightToolCalls=0 disables the
// dispatch-layer semaphore entirely.
func TestStdioDispatch_Unlimited(t *testing.T) {
	const total = 20

	var completed atomic.Int64

	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		completed.Add(1)
		return map[string]any{"ok": true}, nil
	}

	descriptors := []ToolDescriptor{{
		Tool: Tool{
			Name:        "quick",
			Description: "quick",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler:      handler,
		ReadOnlyHint: true,
	}}

	srv := NewServer("test", descriptors, nil, nil)
	srv.MaxInFlightToolCalls = 0 // disabled

	var input bytes.Buffer
	input.WriteString(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}` + "\n")
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"quick","arguments":{}}}`+"\n", i)
	}

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, &input, syncWriter{&output}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit with unlimited dispatch")
	}

	if got := completed.Load(); got != total {
		t.Fatalf("expected %d handlers to complete, got %d", total, got)
	}
	if srv.toolCallSem != nil {
		t.Fatal("expected nil toolCallSem when MaxInFlightToolCalls=0")
	}
}

// syncWriter serializes concurrent Write calls on top of an io.Writer.
// bytes.Buffer is not safe for concurrent use, and the dispatch loop
// uses the server's encoder mutex for its own writes but tests here can
// race the scanner-EOF path with in-flight handler responses.
type syncWriter struct {
	w io.Writer
}

func (s syncWriter) Write(p []byte) (int, error) {
	return s.w.Write(p)
}
