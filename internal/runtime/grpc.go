//go:build grpc

package runtime

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	grpctransport "github.com/apet97/go-clockify/internal/transport/grpc"
)

// runGRPC serves the gRPC transport. Behaviour mirrors the prior
// inline arm in cmd/clockify-mcp/main.go: a 15s ticker warms the
// cached readiness flag consumed by grpc.health.v1, the authenticator
// (optional; only when MCP_AUTH_MODE is set) follows the same path as
// streamable_http, TLS and mTLS load from cfg.GRPCTLSCert/Key and
// cfg.MTLSCACertPath, and the transport is handed off to
// grpctransport.Serve. Only linked into the default build when
// -tags=grpc is set (see grpc_stub.go for the stub path).
func (r *Runtime) runGRPC(ctx context.Context, client *clockify.Client, server *mcp.Server) error {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		check := func() {
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			var user struct{ ID string }
			server.SetReadyCached(client.Get(checkCtx, "/user", nil, &user) == nil)
		}
		check()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check()
			}
		}
	}()
	var grpcAuthenticator authn.Authenticator
	if r.cfg.AuthMode != "" {
		var err error
		grpcAuthenticator, err = authn.New(buildAuthnConfig(r.cfg))
		if err != nil {
			return err
		}
	}
	var grpcTLS *tls.Config
	if r.cfg.GRPCTLSCert != "" {
		cfg, err := buildServerTLSConfig(
			r.cfg.GRPCTLSCert, r.cfg.GRPCTLSKey,
			r.cfg.MTLSCACertPath,
			r.cfg.MTLSCACertPath != "",
			tls.VersionTLS13,
		)
		if err != nil {
			return err
		}
		grpcTLS = cfg
	}
	return grpctransport.Serve(ctx, grpctransport.Options{
		Bind:                 r.cfg.GRPCBind,
		Server:               server,
		Authenticator:        grpcAuthenticator,
		ReauthInterval:       r.cfg.GRPCReauthInterval,
		ForwardTenantHeader:  r.cfg.ForwardTenantHeader,
		ForwardSubjectHeader: r.cfg.ForwardSubjectHeader,
		TLSConfig:            grpcTLS,
	})
}
