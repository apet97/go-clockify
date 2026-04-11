//go:build grpc

package main

import (
	"context"

	// Side-import of the gRPC transport sub-module. This is the ONLY main-
	// module file that imports github.com/apet97/go-clockify/internal/transport/grpc;
	// the //go:build grpc tag ensures the default build never links it — the
	// top-level go.mod has zero google.golang.org/grpc rows and the nm-gate in
	// .github/workflows/ci.yml enforces the symbol absence. See ADR 012.
	grpctransport "github.com/apet97/go-clockify/internal/transport/grpc"

	"github.com/apet97/go-clockify/internal/mcp"
)

// serveGRPC wires the shared mcp.Server into a gRPC listener on bind and
// blocks until ctx cancels. Returns nil on clean shutdown, or a non-nil
// error if the listener could not be opened or Serve failed unexpectedly.
func serveGRPC(ctx context.Context, bind string, server *mcp.Server) error {
	return grpctransport.Serve(ctx, grpctransport.Options{
		Bind:   bind,
		Server: server,
	})
}
