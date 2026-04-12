//go:build !grpc

package main

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

func serveGRPC(_ context.Context, _ string, _ *mcp.Server, _ authn.Authenticator, _ grpcConfig) error {
	return fmt.Errorf("gRPC transport not compiled in: rebuild with -tags=grpc to link the sub-module")
}
