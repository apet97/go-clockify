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

// TestCancellation_AbortsInflightHandler verifies that a notifications/cancelled
// message with a matching requestId aborts a slow tool handler via context
// cancellation. The handler signals when it starts, blocks on ctx.Done(), and
// must return promptly after the cancellation is sent.
func TestCancellation_AbortsInflightHandler(t *testing.T) {
	started := make(chan struct{}, 1)
	finished := make(chan error, 1)

	handler := func(ctx context.Context, _ map[string]any) (any, error) {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			finished <- ctx.Err()
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			finished <- nil
			return map[string]any{"ok": true}, nil
		}
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

	pr, pw := io.Pipe()
	defer pw.Close()

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, pr, syncWriter{&output}) }()

	// Initialize, then submit a slow tools/call with id=42.
	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`+"\n")
	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"slow","arguments":{}}}`+"\n")

	// Wait for the handler to register itself in the inflight map.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for slow handler to start")
	}
	if got := srv.InflightCount(); got != 1 {
		t.Fatalf("expected 1 in-flight call, got %d", got)
	}

	// Send notifications/cancelled targeting id=42.
	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":42,"reason":"client requested"}}`+"\n")

	// Handler must observe ctx cancellation and return.
	select {
	case err := <-finished:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow handler did not exit within 2s of cancellation")
	}

	// Drain Run by closing the pipe and ctx.
	_ = pw.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after pipe close + cancel")
	}

	// The inflight map must have been cleaned up.
	if got := srv.InflightCount(); got != 0 {
		t.Fatalf("expected inflight to be empty after cancellation, got %d", got)
	}

	// The cancelled response should be an MCP isError envelope (tool returned err).
	out := output.String()
	if !strings.Contains(out, `"id":42`) {
		t.Fatalf("expected response for id=42 in output, got: %s", out)
	}
	if !strings.Contains(out, `"isError":true`) {
		t.Fatalf("expected isError:true in cancellation response, got: %s", out)
	}
}

// TestCancellation_UnknownIDNoOp verifies that a cancellation for an
// unknown request id is silently dropped: no panic, no response written,
// inflight map untouched.
func TestCancellation_UnknownIDNoOp(t *testing.T) {
	descriptors := []ToolDescriptor{{
		Tool:         Tool{Name: "noop", Description: "noop", InputSchema: map[string]any{"type": "object"}},
		Handler:      func(_ context.Context, _ map[string]any) (any, error) { return map[string]any{}, nil },
		ReadOnlyHint: true,
	}}
	srv := NewServer("test", descriptors, nil, nil)

	pr, pw := io.Pipe()
	defer pw.Close()

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, pr, syncWriter{&output}) }()

	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`+"\n")
	// Targeting an id that does not exist in the inflight map.
	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"does-not-exist"}}`+"\n")

	// Give the loop a moment to process both messages.
	time.Sleep(150 * time.Millisecond)

	_ = pw.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after pipe close + cancel")
	}

	out := output.String()
	// Notifications must NEVER produce a response, so the only message we
	// should see is the initialize response.
	respCount := strings.Count(out, `"jsonrpc":"2.0"`)
	if respCount != 1 {
		t.Fatalf("expected exactly 1 response (initialize), got %d in: %s", respCount, out)
	}
	// And there must be no error envelope.
	if strings.Contains(out, `"error"`) {
		t.Fatalf("unknown-id cancellation must not produce an error response, got: %s", out)
	}
}

// TestCancellation_NormalCompletionCleansUp verifies that the inflight
// map is empty after a tools/call completes normally (no cancellation).
func TestCancellation_NormalCompletionCleansUp(t *testing.T) {
	const total = 5
	var seen atomic.Int64

	descriptors := []ToolDescriptor{{
		Tool: Tool{
			Name:        "fast",
			Description: "fast",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			seen.Add(1)
			return map[string]any{"ok": true}, nil
		},
		ReadOnlyHint: true,
	}}
	srv := NewServer("test", descriptors, nil, nil)

	var input bytes.Buffer
	input.WriteString(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}` + "\n")
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&input, `{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"fast","arguments":{}}}`+"\n", i)
	}

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, &input, syncWriter{&output}) }()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit on EOF")
	}

	if got := seen.Load(); got != total {
		t.Fatalf("expected %d handler invocations, got %d", total, got)
	}
	if got := srv.InflightCount(); got != 0 {
		t.Fatalf("expected inflight to be empty after normal completion, got %d", got)
	}
}

// TestCancellation_HandleCancelledMalformed exercises the early-return
// branches of handleCancelled for branch coverage.
func TestCancellation_HandleCancelledMalformed(t *testing.T) {
	srv := NewServer("test", nil, nil, nil)
	// Nil params: must not panic.
	srv.handleCancelled(nil)
	// Missing requestId: silent drop.
	srv.handleCancelled(map[string]any{"reason": "no id"})
	// requestId nil: silent drop.
	srv.handleCancelled(map[string]any{"requestId": nil})
	// Unmarshal-incompatible payload (channel can't roundtrip JSON).
	srv.handleCancelled(make(chan int))

	if got := srv.InflightCount(); got != 0 {
		t.Fatalf("inflight should remain empty, got %d", got)
	}
}

// TestCancellation_RegisterUnregisterHelpers exercises the helper
// methods directly for full branch coverage.
func TestCancellation_RegisterUnregisterHelpers(t *testing.T) {
	srv := NewServer("test", nil, nil, nil)

	// Nil ID is a no-op for all helpers.
	srv.registerInflight(nil, func() {})
	srv.unregisterInflight(nil)
	if srv.cancelInflight(nil) {
		t.Fatal("cancelInflight(nil) must return false")
	}

	// Register, then cancel via cancelInflight: counter+presence.
	var fired atomic.Bool
	srv.registerInflight("req-1", func() { fired.Store(true) })
	if got := srv.InflightCount(); got != 1 {
		t.Fatalf("expected 1 entry after register, got %d", got)
	}
	if !srv.cancelInflight("req-1") {
		t.Fatal("cancelInflight should return true for existing ID")
	}
	if !fired.Load() {
		t.Fatal("registered cancel func was not invoked")
	}
	if got := srv.InflightCount(); got != 0 {
		t.Fatalf("expected 0 entries after cancel, got %d", got)
	}

	// Cancel a missing ID returns false.
	if srv.cancelInflight("never-registered") {
		t.Fatal("cancelInflight should return false for missing ID")
	}

	// Register and unregister via the explicit helper.
	srv.registerInflight("req-2", func() {})
	srv.unregisterInflight("req-2")
	if got := srv.InflightCount(); got != 0 {
		t.Fatalf("expected 0 entries after unregister, got %d", got)
	}

	// Lazy map init via nil-map fallback in registerInflight.
	srv.inflightMu.Lock()
	srv.inflight = nil
	srv.inflightMu.Unlock()
	srv.registerInflight("req-3", func() {})
	if got := srv.InflightCount(); got != 1 {
		t.Fatalf("expected lazy init to recreate map, got %d", got)
	}
}
