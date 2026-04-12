//go:build !grpc

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

func serveGRPC(_ context.Context, _ string, _ *mcp.Server, _ authn.Authenticator, _ time.Duration) error {
	return fmt.Errorf("gRPC transport not compiled in: rebuild with -tags=grpc to link the sub-module")
}
