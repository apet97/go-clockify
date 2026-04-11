package grpctransport

import (
	"context"
	"net/http"

	"github.com/apet97/go-clockify/internal/authn"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authStreamInterceptor returns a grpc.StreamServerInterceptor that bridges
// the shared internal/authn Authenticator contract onto gRPC metadata. The
// interceptor reads the `authorization` metadata key, wraps it in a synthetic
// *http.Request so the existing Authenticator implementations keep working
// without a transport-specific variant, and attaches the resulting Principal
// to the stream context via authn.WithPrincipal so downstream enforcement
// (rate limiting, policy, audit) can bucket by Principal.Subject.
//
// Supported auth modes: static_bearer, oidc. Both read only the Authorization
// header and ignore TLS/peer state, so the synthetic request is a faithful
// bridge. forward_auth needs additional headers and mtls needs real TLS
// verified chains — both are rejected upstream in internal/config/config.go
// before the interceptor is ever reached, so no runtime-mode guard is needed
// here. See ADR 012 §auth for rationale.
//
// Validation lifetime: the interceptor fires once per stream open, not per
// message. Long-lived streams accept an OIDC token that was valid at stream
// start even after the token's `exp` elapses. Per-message re-validation is
// Wave 5 backlog — not a regression because the pre-W4 contract was "no
// gRPC auth at all".
func authStreamInterceptor(auth authn.Authenticator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing gRPC metadata")
		}
		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 || authHeaders[0] == "" {
			return status.Error(codes.Unauthenticated, "missing authorization metadata")
		}
		synth := &http.Request{Header: http.Header{}}
		synth.Header.Set("Authorization", authHeaders[0])
		principal, err := auth.Authenticate(ss.Context(), synth)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
		}
		ctx := authn.WithPrincipal(ss.Context(), &principal)
		return handler(srv, &authServerStream{ServerStream: ss, ctx: ctx})
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
