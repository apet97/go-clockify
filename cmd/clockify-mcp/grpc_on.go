//go:build grpc

package main

import (
	"context"
	"time"

	grpctransport "github.com/apet97/go-clockify/internal/transport/grpc"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

func serveGRPC(ctx context.Context, bind string, server *mcp.Server, auth authn.Authenticator, reauthInterval time.Duration) error {
	return grpctransport.Serve(ctx, grpctransport.Options{
		Bind:           bind,
		Server:         server,
		Authenticator:  auth,
		ReauthInterval: reauthInterval,
	})
}
