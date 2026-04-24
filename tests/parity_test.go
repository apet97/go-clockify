package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// defaultFactories returns the transport factories always available on the
// default build. gRPC is appended by the -tags=grpc variant in
// parity_grpc_test.go.
func defaultFactories() map[string]harness.Factory {
	return map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"legacy_http":     harness.NewLegacyHTTP,
		"streamable_http": harness.NewStreamable,
	}
}

// withAllTransports runs the given body for each transport factory. Each
// subtest is t.Parallel — ephemeral ports / bufconn make this safe. The
// gRPC harness returns ErrGRPCUnavailable when compiled without
// -tags=grpc; individual subtests skip rather than fail in that case.
func withAllTransports(t *testing.T, body func(t *testing.T, h harness.Transport)) {
	t.Helper()
	factories := allFactories()
	for name, factory := range factories {
		factory := factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				BearerToken: strings.Repeat("p", 16),
			})
			if err != nil {
				if errors.Is(err, harness.ErrGRPCUnavailable) {
					t.Skip("gRPC harness unavailable (requires -tags=grpc)")
				}
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()
			body(t, h)
		})
	}
}

func TestParity_InitializeReturnsProtocolVersion(t *testing.T) {
	withAllTransports(t, func(t *testing.T, h harness.Transport) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := h.Initialize(ctx)
		if err != nil {
			t.Fatalf("initialize: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("initialize error: %+v", resp.Error)
		}
		pv, err := harness.ProtocolVersion(resp)
		if err != nil {
			t.Fatalf("decode protocolVersion: %v", err)
		}
		if pv == "" {
			t.Fatal("empty protocolVersion")
		}
	})
}

func TestParity_ToolsListContainsMockTool(t *testing.T) {
	withAllTransports(t, func(t *testing.T, h harness.Transport) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := h.Initialize(ctx); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		resp, err := h.ListTools(ctx)
		if err != nil {
			t.Fatalf("tools/list: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/list error: %+v", resp.Error)
		}
		tools, err := harness.ToolsFromListResult(resp)
		if err != nil {
			t.Fatalf("decode tools: %v", err)
		}
		if !harness.ContainsTool(tools, "mock_tool") {
			names := make([]string, 0, len(tools))
			for _, tool := range tools {
				names = append(names, harness.ToolName(tool))
			}
			t.Fatalf("mock_tool missing; got %v", names)
		}
	})
}

func TestParity_ToolsCallSucceeds(t *testing.T) {
	withAllTransports(t, func(t *testing.T, h harness.Transport) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := h.Initialize(ctx); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		resp, err := h.CallTool(ctx, "mock_tool", nil)
		if err != nil {
			t.Fatalf("tools/call: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("tools/call error: %+v", resp.Error)
		}
		// Structured content contract: the mock_tool handler returns
		// {"status": "ok"}. The server wraps that in a structuredContent
		// field. Accept either structuredContent or a plain Result.
		var body struct {
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []any          `json:"content"`
			IsError           bool           `json:"isError"`
		}
		if err := json.Unmarshal(resp.Result, &body); err != nil {
			t.Fatalf("decode tools/call result: %v", err)
		}
		if body.IsError {
			t.Fatalf("tools/call flagged isError with body %s", string(resp.Result))
		}
		if body.StructuredContent != nil {
			if body.StructuredContent["status"] != "ok" {
				t.Fatalf("expected structuredContent.status = ok, got %v", body.StructuredContent)
			}
		}
	})
}

func TestParity_ToolsCallUnknown_ReturnsError(t *testing.T) {
	withAllTransports(t, func(t *testing.T, h harness.Transport) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := h.Initialize(ctx); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		resp, err := h.CallTool(ctx, "this_tool_does_not_exist", nil)
		if err != nil {
			t.Fatalf("tools/call: %v", err)
		}
		if resp.Error == nil {
			t.Fatalf("expected RPC error for unknown tool, got result %s", string(resp.Result))
		}
		if resp.Error.Code != -32602 {
			t.Fatalf("expected -32602 for unknown tool, got %+v", resp.Error)
		}
	})
}
