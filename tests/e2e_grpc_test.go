//go:build grpc

package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/tests/harness"
)

// TestE2E_GRPCLifecycle drives the initialize → tools/list → tools/call
// sequence over gRPC via bufconn. Parity-matched to the stdio/legacy/
// streamable lifecycle tests in e2e_transports_test.go.
func TestE2E_GRPCLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h, err := harness.NewGRPC(ctx, harness.Options{
		BearerToken: strings.Repeat("x", 16),
	})
	if err != nil {
		if errors.Is(err, harness.ErrGRPCUnavailable) {
			t.Skip("gRPC harness unavailable (missing -tags=grpc)")
		}
		t.Fatalf("factory: %v", err)
	}
	defer func() { _ = h.Close() }()

	init, err := h.Initialize(ctx)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if init.Error != nil {
		t.Fatalf("initialize RPC error: code=%d message=%q", init.Error.Code, init.Error.Message)
	}
	pv, err := harness.ProtocolVersion(init)
	if err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if pv == "" {
		t.Fatal("initialize: empty protocolVersion")
	}

	listResp, err := h.ListTools(ctx)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list RPC error: code=%d message=%q", listResp.Error.Code, listResp.Error.Message)
	}
	tools, err := harness.ToolsFromListResult(listResp)
	if err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if !harness.ContainsTool(tools, "mock_tool") {
		t.Fatalf("tools/list missing mock_tool; got %d tools", len(tools))
	}

	call, err := h.CallTool(ctx, "mock_tool", nil)
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if call.Error != nil {
		t.Fatalf("tools/call RPC error: code=%d message=%q", call.Error.Code, call.Error.Message)
	}
}

// TestE2E_GRPC_BearerAuth_RoundTrip verifies the gRPC transport accepts a
// correctly-signed static_bearer request and that the auth metadata
// survives from the client's Authorization header through the server's
// authStreamInterceptor. A regression that drops the metadata would
// manifest as an Unauthenticated error on stream open, which the harness
// surfaces as a factory-level error.
//
// A meaningful wrong-bearer negative test lives alongside the real auth
// matrix in internal/transport/grpc/auth_test.go — constructing a
// mismatched-token transport pair requires direct access to the gRPC
// server Options, which the shared harness does not expose today.
func TestE2E_GRPC_BearerAuth_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token := strings.Repeat("A", 16)
	h, err := harness.NewGRPC(ctx, harness.Options{BearerToken: token})
	if err != nil {
		if errors.Is(err, harness.ErrGRPCUnavailable) {
			t.Skip("gRPC harness unavailable")
		}
		t.Fatalf("factory: %v", err)
	}
	defer func() { _ = h.Close() }()

	if _, err := h.Initialize(ctx); err != nil {
		t.Fatalf("initialize with correct bearer: %v", err)
	}
}
