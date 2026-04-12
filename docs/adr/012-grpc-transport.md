# ADR 012 — gRPC transport via isolated sub-module (no protobuf codegen)

**Status**: Accepted, 2026-04-11. Amended 2026-04-12 (W4-03) to land
the native `static_bearer` / `oidc` auth interceptor via a synthetic
`*http.Request` bridge instead of the service-mesh-only posture the
Wave 3 version required.

## Context

The MCP server shipped three transports through v0.7.1: stdio (JSON-RPC
over stdin/stdout), legacy POST-only HTTP (one request per round-trip),
and streamable HTTP (2025-03-26 spec with long-lived SSE per session).
A subset of prospective operators deploy the server inside a gRPC-heavy
service mesh and asked for a native gRPC transport so that:

- bidirectional streaming lives on one long-lived HTTP/2 connection
  instead of the SSE-over-HTTP/1.1 fallback the streamable HTTP
  transport uses for most intermediaries,
- mTLS, deadlines, and load balancing can be configured through the
  same mesh machinery that already fronts the other gRPC services,
- language bindings that already consume gRPC can reuse their codegen
  toolchains.

Adding gRPC forces a tension against ADR 001 (stdlib-only default
build). The `google.golang.org/grpc` module transitively pulls in
`golang.org/x/net`, `golang.org/x/sys`, `golang.org/x/text`,
`google.golang.org/protobuf`, and `google.golang.org/genproto`. None of
those would run in the default build — the transport selector only
reaches the gRPC code path under `cfg.Transport == "grpc"` — but their
symbols would still be linked unless the gRPC imports sit behind a
build tag, and their `go.mod` rows would show up as `// indirect` in
the top-level module graph unless the imports live in a Go sub-module.

ADR 009 already solved the same problem for OpenTelemetry: the OTel
SDK lives in `internal/tracing/otel/` as a sibling Go module with its
own `go.mod`, linked only under `-tags=otel` via a pair of
`otel_on.go` / `otel_off.go` installer files in `cmd/clockify-mcp/`,
and the top-level `go.mod` stays clean. ADR 012 reuses that shape
verbatim for gRPC so the isolation mechanics are uniform across both
opt-in subsystems.

Two alternatives were considered before landing:

1. **Put the gRPC transport in `package mcp` alongside
   `transport_http.go` and `transport_streamable_http.go`.** Rejected
   because it forces the gRPC import set into the top-level `go.mod`,
   breaking ADR 001 at the module-graph level even with build tags
   protecting the default binary symbol set.
2. **Generate protobuf bindings via `protoc` + `protoc-gen-go` +
   `protoc-gen-go-grpc` and commit the generated `.pb.go` files.**
   Rejected because it adds two code-generation plugins to the build
   toolchain, requires every contributor to install `protoc`, and
   creates a second regeneration ladder (`make proto-gen`) that must
   be kept in sync with release branches. The value the codegen
   provides — strongly-typed request/response structs — is marginal
   because MCP frames are already defined by their JSON-RPC schema,
   and gRPC is only acting as a thin framing transport for the bytes
   the JSON-RPC layer produces. A hand-wired `grpc.ServiceDesc` with
   a custom byte-passthrough `encoding.Codec` delivers the same
   transport semantics with zero generated code.

## Decision

Add a new Go sub-module at `internal/transport/grpc/` with its own
`go.mod` (module path
`github.com/apet97/go-clockify/internal/transport/grpc`). The
sub-module imports the parent `internal/mcp` package for the exported
`Server.DispatchMessage` entry point via a `replace
github.com/apet97/go-clockify => ../../..` directive. `go.work` at the
repo root is extended to list `./internal/transport/grpc` alongside
`./internal/tracing/otel`, so local `go build -tags=grpc ./...` from
the parent resolves both sub-modules without an external proxy fetch.

The sub-module exposes a single public function:

```go
func Serve(ctx context.Context, opts Options) error
```

where `Options{Bind, Server, MaxRecvSize}` names the listener, the
shared `*mcp.Server`, and an optional max frame size. `Serve` installs
a per-stream notifier on the server, listens on TCP, and dispatches
each inbound frame through `Server.DispatchMessage` until the context
cancels. Graceful shutdown drains streams for up to 10 seconds before
forcing `grpc.Server.Stop`.

The **wire format is raw JSON-RPC 2.0 bytes**. A single bidirectional
streaming RPC named `Exchange` on service
`clockify.mcp.v1.MCP` carries both requests and responses. No
`.proto` file is compiled; instead the sub-module registers a hand-
wired `grpc.ServiceDesc` and an `encoding.Codec` (`bytesCodec`) that
accepts only `*[]byte` values. `grpc.ForceServerCodec(bytesCodec{})`
on the server side and `grpc.ForceCodec(bytesCodec{})` on the client
side make grpc-go marshal frames via the codec verbatim. The codec's
`Name()` returns `"clockify-mcp-bytes"` so it cannot be confused with
the default protobuf codec or with any existing registered codec.

A new `Server.DispatchMessage(ctx, msg []byte) ([]byte, error)` method
in `internal/mcp/server.go` exports a thin wrapper around the existing
unexported `Server.handle`. It parses a single JSON-RPC request from
raw bytes, routes it through the standard pipeline, and returns the
serialized response (or `(nil, nil)` for notifications that require no
reply). Non-stdio transports that own their own concurrency model use
this method; the stdio `Server.Run` loop continues to call `handle`
directly so its dispatch-layer semaphore and per-request cancellation
machinery are unchanged.

A new build-tag pair in `cmd/clockify-mcp/` — `grpc_on.go`
(`//go:build grpc`) and `grpc_off.go` (`//go:build !grpc`) — wraps the
`Serve` call so `run()` in `main.go` can dispatch
`cfg.Transport == "grpc"` unconditionally. The on-side imports the
sub-module and forwards to `grpctransport.Serve`; the off-side returns
a clear error asking the operator to rebuild with `-tags=grpc`. This
mirrors the `otel_on.go` / `otel_off.go` and `pprof_on.go` /
`pprof_off.go` pairs that previous ADRs established.

`Config.Load` is extended with a new `MCP_GRPC_BIND` environment
variable (default `:9090`) surfaced through `Config.GRPCBind`, and the
transport validator accepts `"grpc"` alongside `"stdio"`, `"http"`,
and `"streamable_http"`.

### Auth bridge (W4-03)

The W4-03 amendment lands a native stream interceptor in
`internal/transport/grpc/auth.go` that bridges the shared
`internal/authn.Authenticator` contract onto gRPC metadata. The
interceptor reads the `authorization` metadata key, constructs a
synthetic `*http.Request` that carries only an `Authorization: Bearer
<token>` header, and invokes `Authenticator.Authenticate(ctx, req)`
through the same code path the streamable HTTP transport uses. On
success it wraps the stream in an `authServerStream` whose `Context()`
returns a principal-augmented context via `authn.WithPrincipal`, so
downstream enforcement (rate limiting, policy, audit) buckets calls
by `Principal.Subject` exactly as it does for HTTP.

Two auth modes are supported:

- `static_bearer` — the existing
  `staticBearerAuthenticator.Authenticate` reads only the Authorization
  header; the synthetic request is indistinguishable from a real HTTP
  request for its purposes.
- `oidc` — the existing `oidcAuthenticator` reads only the
  Authorization header and fetches JWKS via the real request context
  (not the synthetic body). Works without modification.

`forward_auth` and `mtls` remain HTTP-only because they require data
the synthetic request cannot carry: `forward_auth` needs the
`X-Forwarded-User` / `X-Forwarded-Tenant` headers from an upstream
auth gateway, and `mtls` needs `Request.TLS.VerifiedChains` from the
actual TLS handshake. `Config.Load` rejects both modes for
`MCP_TRANSPORT=grpc` with a clear error pointing operators at
`static_bearer`, `oidc`, or an external mTLS termination layer.

Validation lifetime is **per-stream, not per-message**. The
interceptor fires once when a new `Exchange` stream opens and the
principal persists for the life of that stream. Long-lived streams
that outlast an OIDC token's `exp` claim retain the original
principal — operators that need per-message re-validation should
rotate streams. Per-message re-validation is follow-up work.

Metrics rejections. Policy denials and rate-limit hits from the
enforcement pipeline continue to emit
`clockify_mcp_rate_limit_rejections_total{scope=per_token}` exactly as
they do for HTTP, because the interceptor populates `Principal.Subject`
through the same context key the enforcement layer reads. The only
gRPC-specific failure mode is `codes.Unauthenticated` from the
interceptor itself; those rejections never reach the enforcement
layer and are not counted in the rate-limit metric. A dedicated
`clockify_mcp_grpc_auth_rejections_total` counter is follow-up work.

Two new CI gates live in `.github/workflows/ci.yml`:

1. **`Verify default build has zero gRPC symbols`** — `go tool nm
   /tmp/clockify-mcp-default | grep -c 'google.golang.org/grpc'` must
   return 0 on the default binary.
2. **`Verify go.mod has zero gRPC rows`** — `grep -c
   'google.golang.org/grpc' go.mod` must return 0 on the top-level
   module graph.

Three additional gates exercise the opt-in path: `go build
-tags=grpc ./...`, `cd internal/transport/grpc && go build ./... && go
vet ./... && go test ./...`, and a positive symbol-count check on the
`-tags=grpc` binary to catch a silent build-tag regression. These
mirror the equivalent OTel and FIPS gates and make it impossible to
ship a default build that accidentally links gRPC.

### The `go mod tidy` trap (same as ADR 009)

Running `go mod tidy` on the top-level module will re-add every
transitive gRPC dependency as `// indirect` because Go 1.17+ lazy-
loading requires the main module to list every module reachable
through the workspace replaces. The new "Verify go.mod has zero gRPC
rows" CI gate catches this immediately, and the mitigation is the
same as ADR 009: run `git restore go.mod` after tidy and apply any
unrelated edits by hand.

## Consequences

- **Top-level `go.mod` remains clean.** Zero
  `google.golang.org/grpc` rows, zero `golang.org/x/*` rows added
  by gRPC, zero `google.golang.org/protobuf` rows. The default
  binary size and link graph are byte-identical to pre-W3-04.
- **Default binary symbol gate extended.** The existing
  `opentelemetry` nm gate is joined by a sibling
  `google.golang.org/grpc` gate. Both run against the same
  `/tmp/clockify-mcp-default` artefact so the build step only
  compiles the default binary once.
- **No protoc toolchain dependency.** Contributors do not need
  `protoc`, `protoc-gen-go`, or `protoc-gen-go-grpc` installed to
  build, test, or release the gRPC transport. The sub-module is
  self-contained: hand-rolled codec, hand-wired `ServiceDesc`, and
  an integration test via `google.golang.org/grpc/test/bufconn`.
- **`-tags=grpc` build size.** `go build -tags=grpc -o
  /tmp/clockify-mcp-grpc ./cmd/clockify-mcp` links approximately
  the grpc-go runtime plus its transitive closure. Exact symbol
  counts will vary with the gRPC release; the positive-side CI
  gate asserts non-zero linkage rather than a specific count.
- **gRPC authentication landed in v0.9.0 (W4-03).** The W4-03
  amendment introduces a native stream interceptor supporting
  `static_bearer` and `oidc` via a synthetic `*http.Request` bridge
  onto the existing `internal/authn` package. Operators can now set
  `MCP_AUTH_MODE=static_bearer` (or `oidc`) alongside `MCP_TRANSPORT=grpc`
  without fronting the listener with an external mTLS gateway.
  `forward_auth` and `mtls` remain HTTP-only; see the Auth bridge
  section above for the rationale.
- **Stream notifier semantics match the existing transports.** The
  per-stream `streamNotifier` is installed via `Server.SetNotifier`
  each time a new `Exchange` stream opens; the most recently opened
  stream wins for server-wide broadcasts, mirroring the existing
  stdio / streamable HTTP behaviour. Multiplexing multiple concurrent
  streams onto independent notifier targets is follow-up work.

## Follow-ups

- ~~Authentication interceptor for `MCP_AUTH_MODE=static_bearer` and
  `MCP_AUTH_MODE=oidc` on gRPC.~~ **Landed in W4-03 (2026-04-12).**
- Multi-stream notifier fan-out so server-initiated notifications
  reach every active `Exchange` stream rather than only the most
  recently attached one.
- Streaming `notifications/progress` from long-running tool handlers
  through the gRPC bidirectional channel (the existing
  `EmitProgress` helper already routes through `Server.notifier` and
  should work unchanged once multi-stream notifier fan-out lands).
- Per-message auth re-validation so long-lived streams do not retain
  a principal past the token's `exp` claim.
- ~~`clockify_mcp_grpc_auth_rejections_total` counter for interceptor-
  level `codes.Unauthenticated` rejections (current metrics only
  cover post-auth enforcement rejections).~~
  **Landed in W5-02b (2026-04-12).**
- ~~Native gRPC health protocol probes so Kubernetes `readinessProbe`
  can use `grpc: { port: N }` instead of `tcpSocket`.~~
  **Landed in W5-02a (2026-04-12).**

## Status

Landed on `main` in the W3-04 commit of the 2026-04-11 session. Wave 3
backlog T-4 moved to Landed. Amended 2026-04-12 by the W4-03 auth
interceptor commit. Amended 2026-04-12 by the W5-02a health protocol
commit.
