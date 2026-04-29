package runtime

import (
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/config"
)

// TestBuildStore_GRPCDevBackendRefused locks the defence-in-depth guard
// at internal/runtime/store.go: a caller that constructs a Config{} by
// hand (bypassing config.Load) and asks for gRPC + a dev DSN must be
// refused. Mirrors the streamable_http coverage that already lives in
// the transport_auth_matrix table at the config layer; this test
// exercises the BuildStore-direct path that the matrix cannot reach.
func TestBuildStore_GRPCDevBackendRefused(t *testing.T) {
	t.Setenv("MCP_ALLOW_DEV_BACKEND", "")
	cfg := config.Config{
		Transport:       "grpc",
		ControlPlaneDSN: "memory",
	}
	if _, err := BuildStore(cfg); err == nil {
		t.Fatal("expected refusal for grpc + memory DSN without MCP_ALLOW_DEV_BACKEND, got nil")
	} else if !strings.Contains(err.Error(), "dev backend) is disallowed by default") {
		t.Fatalf("error should match the dev-backend message, got: %v", err)
	} else if !strings.Contains(err.Error(), `MCP_TRANSPORT="grpc"`) {
		t.Fatalf("error should name the actual transport via %%q (grpc), got: %v", err)
	}
}

// TestBuildStore_GRPCDevBackendAllowedWithFlag confirms the escape
// hatch still works on gRPC: MCP_ALLOW_DEV_BACKEND=1 lets the operator
// run the single-process path knowingly.
func TestBuildStore_GRPCDevBackendAllowedWithFlag(t *testing.T) {
	t.Setenv("MCP_ALLOW_DEV_BACKEND", "1")
	cfg := config.Config{
		Transport:       "grpc",
		ControlPlaneDSN: "memory",
	}
	store, err := BuildStore(cfg)
	if err != nil {
		t.Fatalf("flag should permit grpc + memory: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	_ = store.Close()
}

// TestBuildStore_StreamableHTTPDevBackendRefusedRegression keeps the
// pre-existing streamable_http coverage as a regression net so the
// %q-interpolation refactor in the error message can't silently break
// the streamable_http path.
func TestBuildStore_StreamableHTTPDevBackendRefusedRegression(t *testing.T) {
	t.Setenv("MCP_ALLOW_DEV_BACKEND", "")
	cfg := config.Config{
		Transport:       "streamable_http",
		ControlPlaneDSN: "memory",
	}
	if _, err := BuildStore(cfg); err == nil {
		t.Fatal("expected refusal for streamable_http + memory DSN, got nil")
	} else if !strings.Contains(err.Error(), `MCP_TRANSPORT="streamable_http"`) {
		t.Fatalf("error should name the actual transport via %%q, got: %v", err)
	}
}
