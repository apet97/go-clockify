package mcp

import (
	"context"
	"strings"
	"testing"
)

// panicResourceProvider panics from every entry point so resource-dispatch
// tests can prove the stdio loop survives.
type panicResourceProvider struct {
	panicWith string
}

func (p *panicResourceProvider) ListResources(_ context.Context) ([]Resource, error) {
	panic(p.panicWith)
}

func (p *panicResourceProvider) ListResourceTemplates(_ context.Context) ([]ResourceTemplate, error) {
	panic(p.panicWith)
}

func (p *panicResourceProvider) ReadResource(_ context.Context, _ string) ([]ResourceContents, error) {
	panic(p.panicWith)
}

// TestPanicResponseDoesNotExposePanicValue locks in the contract that the
// stdio panic-recovery path returns a generic error message to the client.
// Returning the raw panic value risked leaking internal state, request data,
// or upstream error strings (e.g. credential fragments embedded in panic
// messages from a buggy handler).
//
// The full panic value and stack continue to be emitted via slog at
// ERROR level — operators retain full observability.
func TestPanicResponseDoesNotExposePanicValue(t *testing.T) {
	const fakeSecret = "sk-secret-test-12345"

	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "panicker", Description: "panics with a secret-shaped string"},
		Handler: func(context.Context, map[string]any) (any, error) {
			panic("upstream failure containing " + fakeSecret)
		},
	}})

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"panicker","arguments":{}}}`,
	}, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "internal tool error; request logged") {
		t.Fatalf("expected generic panic message, got: %s", got)
	}
	if strings.Contains(got, fakeSecret) {
		t.Fatalf("panic response leaked secret %q to client: %s", fakeSecret, got)
	}
	if strings.Contains(got, "tool panic:") {
		t.Fatalf("panic response still uses old leaky prefix: %s", got)
	}
	if !strings.Contains(got, `"isError":true`) {
		t.Fatalf("panic response missing isError flag: %s", got)
	}
}

// TestPanicInResourceDispatchDoesNotCrashLoop closes the gap surfaced by the
// 2026-05-29 audit: the async goroutine that handles resources/list,
// resources/read, resources/templates/list, prompts/get, and prompts/list
// must defer RecoverDispatch just like the sibling tools/call goroutine.
// Without it, a panic inside a ResourceProvider crashes the entire stdio
// dispatch loop and violates the SECURITY.md panic-containment promise.
//
// The test drives a resources/read through Server.Run against a provider
// that panics with a secret-shaped string and asserts:
//   - Run terminates without an error (the panic was contained)
//   - the client sees a stable JSON-RPC error envelope on the panic reply
//   - no fragment of the panic value reaches the client
func TestPanicInResourceDispatchDoesNotCrashLoop(t *testing.T) {
	const fakeSecret = "sk-resource-panic-secret-9999"

	server := NewServer("test", nil)
	server.ResourceProvider = &panicResourceProvider{
		panicWith: "resource read failure: " + fakeSecret,
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"clockify://anything"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`,
	}, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run returned error after resource panic; loop crashed instead of recovering: %v", err)
	}

	got := out.String()
	if strings.Contains(got, fakeSecret) {
		t.Fatalf("panic response leaked secret to client: %s", got)
	}
	if !strings.Contains(got, `"id":2`) {
		t.Fatalf("response for resources/read (id=2) missing: %s", got)
	}
	// At least one of the two resource requests must be reported as an error
	// envelope — the panic recovery turns the panicking dispatch into the
	// stable internal-tool-error response rather than silently dropping it.
	if !strings.Contains(got, "internal tool error; request logged") {
		t.Fatalf("expected generic panic envelope for resource dispatch, got: %s", got)
	}
}
