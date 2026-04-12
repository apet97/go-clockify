//go:build grpc

package main

import (
	"context"

	grpctransport "github.com/apet97/go-clockify/internal/transport/grpc"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

func serveGRPC(ctx context.Context, bind string, server *mcp.Server, auth authn.Authenticator, gcfg grpcConfig) error {
	return grpctransport.Serve(ctx, grpctransport.Options{
		Bind:                 bind,
		Server:               server,
		Authenticator:        auth,
		ReauthInterval:       gcfg.reauthInterval,
		ForwardTenantHeader:  gcfg.forwardTenantHeader,
		ForwardSubjectHeader: gcfg.forwardSubjectHeader,
	})
}
