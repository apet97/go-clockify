package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	logslog "github.com/apet97/go-clockify/internal/logging"
)

// TestRecoverDispatch_StableErrorEnvelope locks the contract that
// every transport relies on: a recovered panic produces a JSON-RPC
// 2.0 response with the request ID echoed, isError=true, and the
// fixed user-facing string. Drift here would split stdio,
// alternate HTTP, and alternate RPC error envelopes apart.
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

func TestRecoverDispatch_RedactingLoggerMasksSecretShapedPanicValue(t *testing.T) {
	const secret = "pk_abcdefghijklmnopqrstuvwxyz123456"

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(logslog.NewRedactingHandler(
		slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError}),
	)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	func() {
		defer RecoverDispatch("req-1", "test_site", "tool", func(Response) {})
		panic("upstream failure containing " + secret)
	}()

	got := logs.String()
	if !strings.Contains(got, "panic_recovered") {
		t.Fatalf("expected panic recovery log, got %q", got)
	}
	if strings.Contains(got, secret) {
		t.Fatalf("secret-shaped panic value leaked into operator log: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redacted panic value in operator log, got %s", got)
	}
}

func TestSanitizePanicStackScrubsPathsAndTruncates(t *testing.T) {
	gopath := t.TempDir()
	t.Setenv("GOPATH", gopath)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	goroot := goRootForStackSanitization()
	if goroot == "" {
		t.Skip("go env GOROOT unavailable")
	}

	var stack strings.Builder
	stack.WriteString("goroutine 123 [running]:\n")
	for i := 0; i < maxLoggedPanicStackFrames+3; i++ {
		fmt.Fprintf(&stack, "example.com/project.frame%d()\n", i)
		var path string
		switch i {
		case 0:
			path = filepath.Join(goroot, "src", "runtime", "panic.go")
		case 1:
			path = filepath.Join(gopath, "pkg", "mod", "example.com", "dep.go")
		default:
			path = filepath.Join(wd, "internal", "mcp", "panic.go")
		}
		stack.WriteString("\t" + path + ":12 +0x1\n")
	}

	got := sanitizePanicStack(stack.String())
	for _, raw := range []string{goroot, gopath, wd} {
		if raw != "" && strings.Contains(got, raw) {
			t.Fatalf("sanitized stack still contains raw path %q:\n%s", raw, got)
		}
	}
	for _, marker := range []string{"$GOROOT", "$GOPATH", "$PWD", "... stack truncated ..."} {
		if !strings.Contains(got, marker) {
			t.Fatalf("sanitized stack missing marker %q:\n%s", marker, got)
		}
	}
	if strings.Contains(got, "frame8") {
		t.Fatalf("sanitized stack should include only %d frames:\n%s", maxLoggedPanicStackFrames, got)
	}
}

// TestHandleWithRecover_ReturnsStableEnvelopeOnPanic locks the
// alternate transport recovery path: a panicking tool routed through
// HandleWithRecover must produce the same JSON-RPC tool-error envelope
// stdio emits. Without the named-return recovery wrapper, the panic
// would propagate to the http.Handler boundary and surface as a 500
// with no JSON-RPC framing.
func TestHandleWithRecover_ReturnsStableEnvelopeOnPanic(t *testing.T) {
	const fakeSecret = "sk-handle-with-recover-12345"
	srv := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "panicker", Description: "panics with a secret-shaped string"},
		Handler: func(context.Context, map[string]any) (any, error) {
			panic("upstream failure containing " + fakeSecret)
		},
	}}, nil, nil)
	srv.initialized.Store(true)

	resp := srv.HandleWithRecover(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      99,
		Method:  "tools/call",
		Params:  map[string]any{"name": "panicker", "arguments": map[string]any{}},
	}, "test_site")

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q", resp.JSONRPC)
	}
	if id, ok := resp.ID.(int); !ok || id != 99 {
		t.Errorf("ID = %v (%T), want int 99", resp.ID, resp.ID)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("Result is not map[string]any: %T", resp.Result)
	}
	if result["isError"] != true {
		t.Errorf("isError = %v, want true", result["isError"])
	}
	content, ok := result["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content not []map[string]any: %T", result["content"])
	}
	if len(content) != 1 || content[0]["text"] != "internal tool error; request logged" {
		t.Errorf("content text = %v, want generic helper text", content)
	}
	// Marshal+stringify the entire response and confirm the panic
	// value did not leak.
	full, _ := jsonMarshalForTest(resp)
	if strings.Contains(full, fakeSecret) {
		t.Fatalf("panic value leaked through HandleWithRecover envelope: %s", full)
	}
}

// jsonMarshalForTest is a tiny helper used by recovery tests so
// they can stringify a Response for substring checks.
func jsonMarshalForTest(r Response) (string, error) {
	b, err := json.Marshal(r)
	return string(b), err
}

// TestHandleWithRecover_PassthroughWhenNoPanic verifies the wrapper
// is transparent when the handler returns normally — no spurious
// envelope, ID/Result preserved.
func TestHandleWithRecover_PassthroughWhenNoPanic(t *testing.T) {
	srv := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "echo", Description: "returns ok"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]string{"status": "ok"}, nil
		},
	}}, nil, nil)
	srv.initialized.Store(true)

	resp := srv.HandleWithRecover(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  map[string]any{"name": "echo", "arguments": map[string]any{}},
	}, "test_site")

	if id, ok := resp.ID.(int); !ok || id != 7 {
		t.Fatalf("ID = %v, want 7", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}
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
