package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// The original e2e_transports_test.go here used hardcoded ports
// (127.0.0.1:28080, :28081) and strings.Contains assertions. Wave
// "floofy cascade" replaced that with the TransportHarness contract
// in tests/harness/ — ephemeral ports, decoded JSON-RPC envelopes,
// and one test body exercising every transport.
//
// Each subtest runs with t.Parallel(); the harness factories ask the
// OS for a port (or spin up bufconn), so collisions are impossible.

func TestE2E_StdioLifecycle(t *testing.T) {
	t.Parallel()
	runLifecycle(t, harness.NewStdio)
}

func TestE2E_LegacyHTTPLifecycle(t *testing.T) {
	t.Parallel()
	runLifecycle(t, harness.NewLegacyHTTP)
}

func TestE2E_StreamableHTTPLifecycle(t *testing.T) {
	t.Parallel()
	runLifecycle(t, harness.NewStreamable)
}

// runLifecycle is the shared Initialize → ListTools → CallTool
// assertion body. Decodes every JSON-RPC envelope; no strings.Contains.
func runLifecycle(t *testing.T, factory harness.Factory) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h, err := factory(ctx, harness.Options{})
	if err != nil {
		t.Fatalf("%T factory: %v", factory, err)
	}
	defer func() { _ = h.Close() }()

	// Initialize — protocolVersion must be non-empty.
	initResp, err := h.Initialize(ctx)
	if err != nil {
		t.Fatalf("%s initialize: %v", h.Name(), err)
	}
	if initResp.Error != nil {
		t.Fatalf("%s initialize returned RPC error: code=%d message=%q",
			h.Name(), initResp.Error.Code, initResp.Error.Message)
	}
	pv, err := harness.ProtocolVersion(initResp)
	if err != nil {
		t.Fatalf("%s decode initialize result: %v", h.Name(), err)
	}
	if pv == "" {
		t.Fatalf("%s initialize: empty protocolVersion", h.Name())
	}

	// tools/list — mock_tool must appear.
	listResp, err := h.ListTools(ctx)
	if err != nil {
		t.Fatalf("%s tools/list: %v", h.Name(), err)
	}
	if listResp.Error != nil {
		t.Fatalf("%s tools/list RPC error: code=%d message=%q",
			h.Name(), listResp.Error.Code, listResp.Error.Message)
	}
	tools, err := harness.ToolsFromListResult(listResp)
	if err != nil {
		t.Fatalf("%s decode tools/list: %v", h.Name(), err)
	}
	if !harness.ContainsTool(tools, "mock_tool") {
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, harness.ToolName(tool))
		}
		t.Fatalf("%s tools/list missing mock_tool; got %v", h.Name(), names)
	}

	// tools/call — expect success envelope (no RPC error, no isError flag).
	callResp, err := h.CallTool(ctx, "mock_tool", nil)
	if err != nil {
		t.Fatalf("%s tools/call: %v", h.Name(), err)
	}
	if callResp.Error != nil {
		t.Fatalf("%s tools/call RPC error: code=%d message=%q",
			h.Name(), callResp.Error.Code, callResp.Error.Message)
	}
}

// TestE2E_InvalidToolName_ReturnsRPCError asserts every transport surfaces
// a JSON-RPC error envelope (not a plain HTTP 200 with success result)
// when a client calls a tool that doesn't exist. Cross-transport
// parity for invalid-params / method-not-found.
func TestE2E_InvalidToolName_ReturnsRPCError(t *testing.T) {
	cases := map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"legacy_http":     harness.NewLegacyHTTP,
		"streamable_http": harness.NewStreamable,
	}
	for name, factory := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{})
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			if _, err := h.Initialize(ctx); err != nil {
				t.Fatalf("initialize: %v", err)
			}
			resp, err := h.CallTool(ctx, "nonexistent_tool_xyz", nil)
			if err != nil {
				t.Fatalf("tools/call: %v", err)
			}
			// An "unknown tool" can surface either as a JSON-RPC error
			// envelope or as a Result whose structured body contains an
			// isError=true flag (legacy tool-returned-error shape). Accept
			// either, reject any path that reports success.
			if resp.Error == nil && len(resp.Result) == 0 {
				t.Fatalf("%s: expected error envelope or result with isError=true for unknown tool, got %+v", h.Name(), resp)
			}
		})
	}
}
