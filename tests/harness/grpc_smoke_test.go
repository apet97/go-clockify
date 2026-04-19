//go:build grpc

package harness_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// TestHarness_Smoke_GRPC exercises the gRPC adapter under -tags=grpc. Pulled
// into its own file so the default smoke_test.go stays buildable on the
// stdlib-only default binary.
func TestHarness_Smoke_GRPC(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h, err := harness.NewGRPC(ctx, harness.Options{
		BearerToken: strings.Repeat("a", 16),
	})
	if err != nil {
		if errors.Is(err, harness.ErrGRPCUnavailable) {
			t.Skip("gRPC harness not available")
		}
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
	pv, _ := harness.ProtocolVersion(init)
	if pv == "" {
		t.Fatal("empty protocolVersion from gRPC")
	}

	list, err := h.ListTools(ctx)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	tools, err := harness.ToolsFromListResult(list)
	if err != nil {
		t.Fatalf("decode tools: %v", err)
	}
	if !harness.ContainsTool(tools, "mock_tool") {
		t.Fatalf("mock_tool missing from gRPC tools/list (got %d tools)", len(tools))
	}

	call, err := h.CallTool(ctx, "mock_tool", nil)
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if call.Error != nil {
		t.Fatalf("tools/call error: %v", call.Error)
	}
}
