package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/tests/harness"
)

// TestCancellation_AbortsInflightHandler verifies every notification-capable
// transport's cancellation path actually unblocks the server handler. The
// blocking tool registered by BlockingTool() signals start and then waits
// on ctx.Done(); without cancellation it would sit there until the 45s
// per-tool deadline. The test must complete in far less.
//
// Legacy HTTP is deliberately excluded: its request/response model has no
// cancellation channel once the POST is in-flight. Client-side ctx
// cancellation is covered by net/http tests in the stdlib.
func TestCancellation_AbortsInflightHandler(t *testing.T) {
	cases := map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"streamable_http": harness.NewStreamable,
		"grpc":            harness.NewGRPC,
	}

	for name, factory := range cases {
		factory := factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			started := make(chan struct{}, 1)
			h, err := factory(ctx, harness.Options{
				BearerToken: strings.Repeat("c", 16),
				Tools: []mcp.ToolDescriptor{
					harness.BlockingTool("blocking_tool", started),
				},
			})
			if err != nil {
				if errors.Is(err, harness.ErrGRPCUnavailable) {
					t.Skip("gRPC harness unavailable")
				}
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			if _, err := h.Initialize(ctx); err != nil {
				t.Fatalf("initialize: %v", err)
			}

			reqID, done, err := h.CallToolAsync(ctx, "blocking_tool", nil)
			if err != nil {
				t.Fatalf("CallToolAsync: %v", err)
			}

			// Wait for the handler to actually start before cancelling.
			// Without this, we risk racing the cancel notification past
			// the dispatcher and hitting a "no matching inflight id"
			// silent-drop path in the server.
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s: handler never signaled start within 2s", h.Name())
			}

			if err := h.Cancel(ctx, reqID); err != nil {
				t.Fatalf("%s Cancel: %v", h.Name(), err)
			}

			select {
			case resp := <-done:
				// Either a normal RPC error envelope carrying
				// context-cancellation, OR a successful response with
				// isError=true. The important invariant is that the
				// handler returned in < 2s, proving cancellation reached
				// the ctx.
				if resp.Error == nil && len(resp.Result) == 0 {
					t.Fatalf("%s: expected cancellation to surface an error or result, got empty response", h.Name())
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s: handler did not return within 3s of Cancel — context.Canceled did not propagate", h.Name())
			}
		})
	}
}
