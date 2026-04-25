package mcp

import (
	"context"
	"strings"
	"testing"
)

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
	}}, nil, nil)

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
