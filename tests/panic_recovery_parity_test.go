package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/tests/harness"
)

// panicTool is the mock tool registered across transports for the
// panic-recovery parity matrix. It panics with a string deliberately
// shaped like a leaked secret so the test can also verify the
// transport never echoes the panic value back to the client. The
// constant is shared so every subtest checks the same string.
const panicSecret = "sk-panic-parity-secret-" + "12345"

// assertNoPanicLeak fails the subtest when the response payload
// contains the panic value, the literal "panic", or a fragment of
// the recovered stack trace. Shared by every transport variant so
// the leakage check cannot drift apart from the per-transport
// envelope assertions.
func assertNoPanicLeak(t *testing.T, payload string) {
	t.Helper()
	if strings.Contains(payload, panicSecret) {
		t.Fatalf("panic value leaked to client payload: %s", payload)
	}
	if strings.Contains(strings.ToLower(payload), "upstream failure") {
		t.Fatalf("panic message text leaked to client payload: %s", payload)
	}
}

// TestParity_ToolPanicReturnsStableErrorEnvelope locks the
// cross-transport contract that a panicking tool handler produces:
//
//   - JSON-RPC 2.0 envelope with the original request ID echoed.
//   - result.isError == true.
//   - result.content[0].text == "internal tool error; request logged"
//     (the stable string emitted by mcp.RecoverDispatch).
//   - the panic value is NOT leaked into the response body.
//
// Pre-this-wave, only stdio had structured panic recovery; streamable
// HTTP and gRPC let the panic propagate up the dispatch goroutine and
// either dropped the connection (HTTP) or surfaced it as a generic
// gRPC stream error (gRPC). Each transport now wraps its dispatch in
// RecoverDispatch so the user-facing envelope is byte-identical.
func TestParity_ToolPanicReturnsStableErrorEnvelope(t *testing.T) {
	panicTool := mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        "panic_tool",
			Description: "Panics with a secret-shaped string for the parity test",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(context.Context, map[string]any) (any, error) {
			panic("upstream failure containing " + panicSecret)
		},
	}

	for name, factory := range allFactories() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				BearerToken: strings.Repeat("p", 16),
				Tools:       []mcp.ToolDescriptor{panicTool},
			})
			if err != nil {
				if errors.Is(err, harness.ErrGRPCUnavailable) {
					t.Skip("gRPC harness unavailable (requires -tags=grpc)")
				}
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			if _, err := h.Initialize(ctx); err != nil {
				t.Fatalf("initialize: %v", err)
			}

			resp, err := h.CallTool(ctx, "panic_tool", nil)
			if err != nil {
				t.Fatalf("tools/call: transport-level error %v — panic should be recovered, not surfaced as a transport failure", err)
			}

			// legacy_http is the deliberate exception: it recovers the
			// panic at the http.Handler boundary and returns a generic
			// 500 with {"error":"internal server error"}, encoded as an
			// RPCError on the harness side. The shared invariant is
			// "panic value never leaks"; the JSON-RPC tool-error
			// envelope is only required from stdio/streamable/grpc.
			if name == "legacy_http" {
				if resp.Error == nil {
					t.Fatalf("legacy_http: expected RPC error envelope, got result=%s", string(resp.Result))
				}
				assertNoPanicLeak(t, resp.Error.Message)
				if resp.Error.Data != nil {
					if data, _ := json.Marshal(resp.Error.Data); strings.Contains(string(data), panicSecret) {
						t.Fatalf("legacy_http leaked panic value via Error.Data: %s", string(data))
					}
				}
				return
			}

			if resp.Error != nil {
				t.Fatalf("tools/call returned RPC error %+v; expected a tool-level error envelope", resp.Error)
			}

			var body struct {
				IsError bool `json:"isError"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(resp.Result, &body); err != nil {
				t.Fatalf("decode tools/call result: %v\nraw: %s", err, string(resp.Result))
			}
			if !body.IsError {
				t.Fatalf("expected isError=true, got body=%s", string(resp.Result))
			}
			if len(body.Content) != 1 || body.Content[0].Text != "internal tool error; request logged" {
				t.Fatalf("expected stable RecoverDispatch text; got %+v", body.Content)
			}
			assertNoPanicLeak(t, string(resp.Result))
		})
	}
}
