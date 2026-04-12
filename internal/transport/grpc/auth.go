package grpctransport

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authStreamInterceptor returns a grpc.StreamServerInterceptor that bridges
// the shared internal/authn Authenticator contract onto gRPC metadata.
//
// When reauthInterval > 0, a background goroutine re-validates the token
// every interval. If re-validation fails, it cancels the stream context
// so the Exchange loop exits cleanly with codes.Unauthenticated.
func authStreamInterceptor(auth authn.Authenticator, reauthInterval time.Duration) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			metrics.GRPCAuthRejectionsTotal.Inc("missing_metadata")
			return status.Error(codes.Unauthenticated, "missing gRPC metadata")
		}
		synth, err := buildSynthRequest(md)
		if err != nil {
			return err
		}
		principal, err := auth.Authenticate(ss.Context(), synth)
		if err != nil {
			metrics.GRPCAuthRejectionsTotal.Inc("auth_failed")
			return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
		}
		ctx := authn.WithPrincipal(ss.Context(), &principal)
		if reauthInterval > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			go reauthLoop(ctx, cancel, auth, synth, reauthInterval)
		}
		return handler(srv, &authServerStream{ServerStream: ss, ctx: ctx})
	}
}

func buildSynthRequest(md metadata.MD) (*http.Request, error) {
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
