//go:build !grpc

package main

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

// serveGRPC is the default-build stub. Rebuild with `go build -tags=grpc`
// to link the internal/transport/grpc sub-module and enable the transport.
// See ADR 012. Signature mirrors grpc_on.go so cmd/clockify-mcp/main.go
// can call it uniformly — the `auth` parameter is ignored here because no
// transport is ever constructed.
func serveGRPC(_ context.Context, _ string, _ *mcp.Server, _ authn.Authenticator) error {
	return fmt.Errorf("gRPC transport not compiled in: rebuild with -tags=grpc to link the sub-module")
}
