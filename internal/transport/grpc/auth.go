package grpctransport

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// authStreamInterceptor returns a grpc.StreamServerInterceptor that bridges
// the shared internal/authn Authenticator contract onto gRPC metadata.
//
// When reauthInterval > 0, a background goroutine re-validates the token
// every interval. If re-validation fails, it cancels the stream context
// so the Exchange loop exits cleanly with codes.Unauthenticated.
type authInterceptorConfig struct {
	reauthInterval       time.Duration
	forwardTenantHeader  string
	forwardSubjectHeader string
	mtls                 bool
}

func authStreamInterceptor(auth authn.Authenticator, cfg authInterceptorConfig) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			metrics.GRPCAuthRejectionsTotal.Inc("missing_metadata")
			return status.Error(codes.Unauthenticated, "missing gRPC metadata")
		}
		synth, err := buildSynthRequest(md, cfg.forwardTenantHeader, cfg.forwardSubjectHeader)
		if err != nil {
			return err
		}
		if cfg.mtls {
			if p, ok := peer.FromContext(ss.Context()); ok {
				if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
					synth.TLS = &tls.ConnectionState{
						VerifiedChains:   tlsInfo.State.VerifiedChains,
						PeerCertificates: tlsInfo.State.PeerCertificates,
					}
				}
			}
		}
		principal, err := auth.Authenticate(ss.Context(), synth)
		if err != nil {
			metrics.GRPCAuthRejectionsTotal.Inc("auth_failed")
			return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
		}
		ctx := authn.WithPrincipal(ss.Context(), &principal)
		if cfg.reauthInterval > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			go reauthLoop(ctx, cancel, auth, synth, cfg.reauthInterval)
		}
		return handler(srv, &authServerStream{ServerStream: ss, ctx: ctx})
	}
}

func buildSynthRequest(md metadata.MD, forwardTenantHeader, forwardSubjectHeader string) (*http.Request, error) {
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		metrics.GRPCAuthRejectionsTotal.Inc("missing_authorization")
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	if authHeaders[0] == "" {
		metrics.GRPCAuthRejectionsTotal.Inc("empty_authorization")
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	synth := &http.Request{Header: http.Header{}}
	synth.Header.Set("Authorization", authHeaders[0])
	if forwardTenantHeader != "" {
		if vals := md.Get(strings.ToLower(forwardTenantHeader)); len(vals) > 0 {
			synth.Header.Set(forwardTenantHeader, vals[0])
		}
	}
	if forwardSubjectHeader != "" {
		if vals := md.Get(strings.ToLower(forwardSubjectHeader)); len(vals) > 0 {
			synth.Header.Set(forwardSubjectHeader, vals[0])
		}
	}
	return synth, nil
}

func reauthLoop(ctx context.Context, cancel context.CancelFunc, auth authn.Authenticator, synth *http.Request, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := auth.Authenticate(ctx, synth); err != nil {
				slog.Warn("grpc_reauth_failed", "error", err.Error())
				metrics.GRPCAuthRejectionsTotal.Inc("reauth_expired")
				cancel()
				return
			}
		}
	}
}

// authServerStream wraps a grpc.ServerStream so handlers observing
// ServerStream.Context() see the principal-augmented context computed once
// at interceptor entry. Computing it per-Context() call would be correct but
// racier to reason about under the stream's single-writer semantics.
type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authServerStream) Context() context.Context { return s.ctx }
