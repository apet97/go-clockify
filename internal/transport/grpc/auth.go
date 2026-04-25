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
//
// The mtls flag selects the synthetic-request shape:
//
//   - cfg.mtls == true: Authorization metadata is NOT required. The
//     authenticator (authn.ModeMTLS) inspects the verified client
//     certificate carried via peer.AuthInfo. Forward-tenant/subject
//     headers are still copied if configured but are advisory only.
//
//   - cfg.mtls == false: Authorization metadata IS required (static
//     bearer / OIDC / forward_auth). Missing or empty values are
//     rejected with codes.Unauthenticated before the authenticator
//     ever runs.
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
		var (
			synth *http.Request
			err   error
		)
		if cfg.mtls {
			synth = buildMTLSSynthRequest(ss.Context(), md, cfg.forwardTenantHeader, cfg.forwardSubjectHeader)
		} else {
			synth, err = buildSynthRequest(md, cfg.forwardTenantHeader, cfg.forwardSubjectHeader)
			if err != nil {
				return err
			}
		}
		principal, err := auth.Authenticate(ss.Context(), synth)
		if err != nil {
			metrics.GRPCAuthRejectionsTotal.Inc("auth_failed")
			// Detail leak guard: the failure reason can name issuers,
			// JWKS keys, tenant claims, or expiry timestamps. Log the
			// raw error server-side (with a coarse category for
			// dashboards) and return only the generic phrase to the
			// caller. Mirrors the streamable_http behaviour gated by
			// MCP_EXPOSE_AUTH_ERRORS.
			slog.Warn("grpc_auth_failed",
				"error", err.Error(),
				"category", authn.FailureCategory(err),
				"method", info.FullMethod,
			)
			return status.Error(codes.Unauthenticated, "authentication failed")
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

// buildSynthRequest is the bearer/OIDC/forward_auth path. It requires the
// caller to have stamped an "authorization" metadata pair; missing or empty
// values are rejected before the authenticator runs so we never call into
// e.g. the OIDC verifier with an obviously empty token. Forward-tenant and
// forward-subject headers are copied through if configured.
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
	copyForwardHeaders(md, synth, forwardTenantHeader, forwardSubjectHeader)
	return synth, nil
}

// buildMTLSSynthRequest is the mTLS path. Authorization metadata is NOT
// required — the verified client certificate is the credential. The TLS
// state is lifted off peer.AuthInfo so authn.ModeMTLS can inspect
// VerifiedChains and PeerCertificates exactly as it would on the HTTP
// transport. Forward-tenant/subject headers are copied through if
// configured (advisory only — the cert is the source of truth when
// MCP_MTLS_TENANT_SOURCE=cert, the safe default).
//
// When peer.AuthInfo is missing or not credentials.TLSInfo (e.g. the
// gRPC server was misconfigured without TLS), synth.TLS stays nil and
// authn.ModeMTLS rejects with "verified mTLS client certificate
// required" — the correct failure category, not "missing authorization
// metadata".
func buildMTLSSynthRequest(ctx context.Context, md metadata.MD, forwardTenantHeader, forwardSubjectHeader string) *http.Request {
	synth := &http.Request{Header: http.Header{}}
	copyForwardHeaders(md, synth, forwardTenantHeader, forwardSubjectHeader)
	if p, ok := peer.FromContext(ctx); ok {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			synth.TLS = &tls.ConnectionState{
				VerifiedChains:   tlsInfo.State.VerifiedChains,
				PeerCertificates: tlsInfo.State.PeerCertificates,
			}
		}
	}
	return synth
}

// copyForwardHeaders mirrors the shape used by the streamable_http
// transport: when the operator has configured a forward-tenant or
// forward-subject header name, the matching metadata pair (looked up
// case-insensitively as gRPC requires) is copied onto the synthetic
// http.Header. Empty/unset names are skipped.
func copyForwardHeaders(md metadata.MD, synth *http.Request, forwardTenantHeader, forwardSubjectHeader string) {
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
				slog.Warn("grpc_reauth_failed",
					"error", err.Error(),
					"category", authn.FailureCategory(err),
				)
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
