# 0008 - gRPC auth via stream interceptor

## Status

Accepted — landed in commit `4c864e9` (W4-03); per-interval
re-validation, forward-auth, and mTLS extensions in W5-05a/b/c
(`241572e`, `86bdbe1`, `d53186c`).

## Context

ADR 0002 introduced the `grpc` transport for bidirectional
low-latency clients on a private network. ADR 0003 defined the four
auth modes and committed to a single `internal/authn.Authenticator`
contract shared across transports. The HTTP transport already had
authn wiring; the gRPC transport needed a way to plug into the same
contract without reinventing the auth pipeline.

gRPC has its own authentication idioms — interceptors, metadata
headers, `credentials.PerRPCCredentials`, `tls.ConnectionState` for
mTLS — none of which map naturally to a `http.Request`. We needed a
small adapter that bridges gRPC metadata onto the `Authenticator`
contract so the same code-paths handle bearer tokens, OIDC JWTs,
forward-auth headers, and mTLS regardless of transport.

## Decision

A `grpc.StreamServerInterceptor` lives next to the gRPC transport in
`internal/transport/grpc/` and wraps the shared
`internal/authn.Authenticator`:

- **Single interceptor, all auth modes.** `authStreamInterceptor`
  reads `Authorization` metadata for `static_bearer` and `oidc`,
  reads the configured forward-auth headers for `forward_auth`, and
  reads `tls.ConnectionState.VerifiedChains` from the gRPC peer for
  `mtls`. The mode dispatch is in
  `internal/transport/grpc/transport.go:76-83`.
- **Shared authn contract.** The interceptor calls
  `authn.Authenticator.Authenticate` (the same method the HTTP
  transport calls), so the OIDC validation, JWKS rotation, and
  bearer-token comparison logic lives once in `internal/authn`.
- **Per-interval re-validation (W5-05a).** When `ReauthInterval` is
  set, the interceptor re-validates the principal periodically over
  the lifetime of the long-lived stream. This catches token
  expiration on bidirectional streams that would otherwise hold a
  principal indefinitely.
- **Forward-auth via metadata (W5-05b).** The interceptor honours
  `ForwardTenantHeader` and `ForwardSubjectHeader` so a service
  mesh can authenticate upstream and pass tenant/subject claims
  through gRPC metadata.
- **mTLS via TLS ConnectionState (W5-05c).** When the gRPC server
  is configured with TLS (`MCP_GRPC_TLS_CERT` + `MCP_GRPC_TLS_KEY`)
  and `tls.RequireAndVerifyClientCert`, the interceptor extracts
  the client cert chain from the peer's `credentials.TLSInfo` and
  passes it to the authn layer for principal extraction.
- **Per-stream notifier registration.** Authentication runs once
  per stream (plus the periodic re-validation). A per-stream
  `streamNotifier` is also installed via
  `Server.AddNotifier` so server-initiated notifications fan out
  to every active client (`transport.go:132-167`).

The gRPC code path lives in a sub-tree that is only compiled with
`-tags=grpc`; the default binary's `main()` uses a stub that returns
an explicit "rebuild with -tags=grpc" error. ADR 0001 and the
`scripts/check-build-tags.sh` gate enforce that the top-level
`go.mod` has zero `google.golang.org/grpc` rows.

## Consequences

### Positive

- One `internal/authn.Authenticator` powers HTTP, streamable HTTP,
  and gRPC. A security fix to (e.g.) JWKS rotation or bearer-token
  comparison lands once and protects every transport.
- The four auth modes from ADR 0003 work transparently on gRPC.
  Operators who deploy gRPC on a service mesh do not need to
  reinvent forward-auth or mTLS handling.
- Per-interval re-validation closes the long-lived-stream token
  expiry hole that bidirectional gRPC streams would otherwise have.
  HTTP requests are short-lived enough that periodic re-validation
  is unnecessary; gRPC streams are not.
- gRPC stays out of the default build entirely, preserving the
  stdlib-only invariant from ADR 0001.

### Negative

- The interceptor adds gRPC-specific glue that has no HTTP analogue.
  A reviewer comparing the two transports' authn paths sees
  divergent code at the metadata-extraction level even though the
  underlying validation is shared.
- mTLS auth requires the gRPC server to be configured with TLS
  *and* `RequireAndVerifyClientCert`. Both must be set; a
  partial configuration surfaces as a misleading auth failure
  rather than a startup error. We consider documenting this in
  `docs/runbooks/auth-failures.md` sufficient for now.
- The build-tag isolation means a contributor working on the gRPC
  transport cannot use the default `make check` workflow; they
  must pass `-tags=grpc` explicitly.

### Neutral

- The `streamNotifier` uses a mutex around `SendMsg` because gRPC
  server streams require single-writer semantics. The mutex is
  per-stream and therefore not a global contention point.
- The interceptor uses a `Stream` interceptor rather than a
  `Unary` interceptor because the gRPC transport exposes exactly
  one bidirectional streaming method (`Exchange`) — there are no
  unary RPCs to intercept.

## Alternatives considered

- **Re-implement the four auth modes inline in
  `internal/transport/grpc/`** — rejected because it would
  duplicate the JWKS rotation and bearer-comparison logic and
  invite drift between HTTP and gRPC authn.
- **Use gRPC `credentials.PerRPCCredentials` instead of a stream
  interceptor** — rejected because PerRPCCredentials runs on the
  client side; we needed server-side validation.
- **Run an HTTP auth proxy in front of gRPC** — rejected because
  the operator UX would then be "deploy gRPC + sidecar", and the
  shared `Authenticator` contract already handles the validation
  in-process.

## References

- Previously referred to as "ADR 012" in
  `cmd/clockify-mcp/main.go:251`, `internal/transport/grpc/transport.go:29`,
  `scripts/check-build-tags.sh:68`, `deploy/helm/clockify-mcp/values.yaml:81`,
  `deploy/k8s/base/deployment.yaml:48`, `internal/config/config_test.go:282`.
- Interceptor: `internal/transport/grpc/transport.go:76-83`
  (`authStreamInterceptor` invocation).
- Per-stream notifier: `internal/transport/grpc/transport.go:132-167`.
- Shared authn: `internal/authn/`.
- Related ADRs: 0001 (stdlib-only invariant), 0002 (transport
  selection), 0003 (auth-mode compatibility matrix).
- Related docs: `docs/production-readiness.md` "Pick a transport",
  `docs/runbooks/auth-failures.md`.
- Spec: <https://grpc.io/docs/guides/auth/> (gRPC authentication
  guide).
