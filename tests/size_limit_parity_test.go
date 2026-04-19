package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// TestSizeLimit_ParityAcrossTransports configures a 4 KiB message cap and
// sends a tools/call whose arguments push the request well over the limit.
// Every transport must reject it (error envelope, isError=true result, or
// factory/transport-level error) — a silent success would mean the cap
// was honoured nowhere.
//
// Below-limit control case also runs per transport to guard against the
// opposite failure mode (the cap rejecting everything).
func TestSizeLimit_ParityAcrossTransports(t *testing.T) {
	const cap int64 = 4096 // 4 KiB — small enough to hit with a tiny payload

	cases := allFactories()
	for name, factory := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				MaxMessageSize: cap,
				BearerToken:    strings.Repeat("s", 16),
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

			// --- under-limit control ---
			small := map[string]any{"payload": strings.Repeat("a", 128)}
			resp, err := h.CallTool(ctx, "mock_tool", small)
			if err != nil {
				t.Fatalf("%s: under-limit call returned transport error: %v", h.Name(), err)
			}
			if resp.Error != nil {
				t.Fatalf("%s: under-limit call rejected: code=%d msg=%q",
					h.Name(), resp.Error.Code, resp.Error.Message)
			}

			// --- over-limit payload ---
			huge := map[string]any{"payload": strings.Repeat("b", int(cap*2))}
			resp, err = h.CallTool(ctx, "mock_tool", huge)
			// "Rejected" can surface three ways:
			//   1. Transport-level error (e.g. stdio pipe closed, gRPC
			//      code=ResourceExhausted); harness wraps as err != nil.
			//   2. RPC error envelope (HTTP transports: ReadAll on a
			//      MaxBytesReader returns an error, server writes a 413
			//      which the adapter maps to code=-32001).
			//   3. JSON-RPC parse error (-32700) if the server read the
			//      body, hit the limit, and mis-parsed the truncated
			//      payload.
			// Any of the three proves the cap is enforced.
			if err != nil {
				return
			}
			if resp.Error != nil {
				return
			}
			t.Fatalf("%s: over-limit payload returned success — cap not enforced (result=%s)",
				h.Name(), string(resp.Result))
		})
	}
}

// TestSizeLimit_MalformedJSONParity sends a deliberately invalid frame
// through each transport's raw-send primitive and asserts the server
// surfaces JSON-RPC parse error -32700. Fills the third boundary the
// size-limit plan called out (at-limit / over-limit / malformed); the
// first two are covered above, this one pins the last.
//
// "Malformed" here is a request missing its closing brace — a shape
// json.Unmarshal rejects outright. The server's DispatchMessage and
// each HTTP handler convert that to {"jsonrpc":"2.0","error":{"code":
// -32700,"message":"invalid JSON"}}.
func TestSizeLimit_MalformedJSONParity(t *testing.T) {
	cases := allFactories()
	for name, factory := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				BearerToken: strings.Repeat("m", 16),
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

			// Intentionally malformed: missing closing brace.
			malformed := []byte(`{"jsonrpc":"2.0","id":99,"method":"tools/list"`)
			resp, err := h.SendRaw(ctx, malformed)
			if err != nil {
				t.Fatalf("%s: SendRaw returned transport error: %v", h.Name(), err)
			}
			if resp.Error == nil {
				t.Fatalf("%s: malformed JSON returned non-error response: %+v", h.Name(), resp)
			}
			if resp.Error.Code != -32700 {
				t.Fatalf("%s: expected parse error -32700, got code=%d msg=%q",
					h.Name(), resp.Error.Code, resp.Error.Message)
			}
		})
	}
}
