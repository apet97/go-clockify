//go:build !grpc

package runtime

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// runGRPC is the default-build stub: the gRPC sub-module is behind a
// build tag (see ADR 0012), so the default binary refuses
// MCP_TRANSPORT=grpc with a clear diagnostic. Rebuild with
// `go build -tags=grpc ./cmd/clockify-mcp` to link the real runGRPC
// in grpc.go.
func (r *Runtime) runGRPC(_ context.Context, _ *clockify.Client, _ *mcp.Server) error {
	return fmt.Errorf("gRPC transport not compiled in: rebuild with -tags=grpc to link the sub-module")
}
