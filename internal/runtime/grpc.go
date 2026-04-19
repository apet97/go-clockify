//go:build grpc

package runtime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
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
		cert, err := tls.LoadX509KeyPair(r.cfg.GRPCTLSCert, r.cfg.GRPCTLSKey)
		if err != nil {
			return fmt.Errorf("load gRPC TLS cert/key: %w", err)
		}
		grpcTLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
		if r.cfg.MTLSCACertPath != "" {
			caCert, err := os.ReadFile(r.cfg.MTLSCACertPath)
			if err != nil {
				return fmt.Errorf("read mTLS CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("mTLS CA cert: no valid PEM certificates found")
			}
			grpcTLS.ClientCAs = pool
			grpcTLS.ClientAuth = tls.RequireAndVerifyClientCert
		}
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
