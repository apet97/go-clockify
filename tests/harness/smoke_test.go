package harness_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// TestHarness_Smoke_AllDefaultTransports drives stdio, legacy HTTP, and
// streamable HTTP through the minimum lifecycle — Initialize, ListTools,
// CallTool — and asserts protocol-level correctness. gRPC is tested under
// -tags=grpc in tests/e2e_grpc_test.go; leaving it out of this suite keeps
// the smoke test runnable on the default build.
func TestHarness_Smoke_AllDefaultTransports(t *testing.T) {
	factories := map[string]harness.Factory{
		"stdio":           harness.NewStdio,
		"legacy_http":     harness.NewLegacyHTTP,
		"streamable_http": harness.NewStreamable,
	}
	for name, factory := range factories {
		factory := factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{})
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			init, err := h.Initialize(ctx)
			if err != nil {
				t.Fatalf("initialize: %v", err)
			}
			if init.Error != nil {
				t.Fatalf("initialize error: %v", init.Error)
			}
			pv, err := harness.ProtocolVersion(init)
			if err != nil {
				t.Fatalf("protocolVersion decode: %v", err)
			}
			if pv == "" {
				t.Fatal("initialize returned empty protocolVersion")
			}

			list, err := h.ListTools(ctx)
			if err != nil {
				t.Fatalf("tools/list: %v", err)
			}
			if list.Error != nil {
				t.Fatalf("tools/list error: %v", list.Error)
			}
			tools, err := harness.ToolsFromListResult(list)
			if err != nil {
				t.Fatalf("decode tools: %v", err)
			}
			if !harness.ContainsTool(tools, "mock_tool") {
				t.Fatalf("%s: mock_tool missing from tools/list (got %d tools)", h.Name(), len(tools))
			}

			call, err := h.CallTool(ctx, "mock_tool", nil)
			if err != nil {
				t.Fatalf("tools/call: %v", err)
			}
			if call.Error != nil {
				t.Fatalf("tools/call error: %v", call.Error)
			}
		})
	}
}
