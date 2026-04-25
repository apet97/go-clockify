# Deployment profile: private-network gRPC

> Apply with `clockify-mcp --profile=private-network-grpc` or
> `MCP_PROFILE=private-network-grpc`. Requires a build with
> `-tags=grpc` (the default binary does not include the gRPC
> transport — see [ADR-0001](../adr/0001-stdlib-only-default-build.md)).
> Example env file:
> [`deploy/examples/env.private-network-grpc.example`](../../deploy/examples/env.private-network-grpc.example).
> See also: [`internal/config/profile.go`](../../internal/config/profile.go)
> for the pinned defaults, [ADR-0015](../adr/0015-profile-centric-configuration.md)
> for the design rationale.

A deployment where `clockify-mcp` is reachable from a private
network segment over gRPC, with mutual TLS authenticating the
caller via certificates issued by an internal CA. This is the
profile for service-to-service integrations inside a VPC or
private cluster, not for public-internet-facing endpoints.

Use this when: an internal service wants to call Clockify tools
over a typed RPC surface on a private network, and your
infrastructure already issues client certs for inter-service auth.
Do **not** use this when: any caller is on the public internet
(stick with `shared-service` + OIDC behind a reverse proxy).

## Pinned defaults

This profile sets the following on your behalf; explicit env
overrides still win:

| Variable | Profile default | Why |
|----------|-----------------|-----|
| `MCP_TRANSPORT` | `grpc` | profile target |
| `MCP_AUTH_MODE` | `mtls` | caller identity is the client cert |
| `MCP_AUDIT_DURABILITY` | `fail_closed` | private-network callers are treated as trusted infrastructure; failed audit writes must abort the call |

Everything else (listen bind, TLS cert paths, CA bundle, control
plane DSN, metrics) is your responsibility — there is no sane
default for those. See the example env file for the minimum
required set.

### Required operator-supplied env

The profile pins `MCP_TRANSPORT=grpc` + `MCP_AUTH_MODE=mtls`, which
makes every variable below a hard requirement at startup. Without
the full set, either `config.Load()` rejects the configuration or
`clockify-mcp doctor --strict` flags it.

| Variable | Why |
|----------|-----|
| `MCP_GRPC_TLS_CERT` | Server certificate PEM. Required by mTLS — `config.Load()` rejects mTLS without it. |
| `MCP_GRPC_TLS_KEY` | Server key PEM. Same requirement. |
| `MCP_MTLS_CA_CERT_PATH` | Client CA bundle. Same requirement. |
| `MCP_MTLS_TENANT_SOURCE=cert` | The hosted-strict posture (default). Header variants invert the trust model — only acceptable behind a proxy that strips client-supplied tenant headers. |
| `MCP_REQUIRE_MTLS_TENANT=1` | Reject clients whose cert exposes no tenant identity. Without this, misissued certs collapse onto `MCP_DEFAULT_TENANT_ID` — the multi-tenant footgun this gate closes. `doctor --strict` enforces this when `MCP_AUTH_MODE=mtls`. |
| `MCP_CONTROL_PLANE_DSN=postgres://…` | Production HA — required by `doctor --strict`. |

## Build requirement

The gRPC transport lives behind the `grpc` build tag. Binaries in
our `docker-image.yml` image builds include it; a `go build`
without `-tags=grpc` does not. If `clockify-mcp` exits with
`MCP_TRANSPORT=grpc requires -tags=grpc at build time`, rebuild
with the tag or use a published image.

## Security model

- The caller's identity is the Subject/SAN of their client
  certificate. Tenant extraction defaults to `X-Tenant-ID` in the
  request metadata; override with `MCP_MTLS_TENANT_HEADER` if
  your CA encodes tenant elsewhere.
- The server cert / key must be PEM files readable by the process
  user. Paths come from `MCP_GRPC_TLS_CERT` and
  `MCP_GRPC_TLS_KEY`.
- The client CA bundle (`MCP_MTLS_CA_CERT_PATH`) must contain
  every CA whose certs you trust. Revoke via your CA's CRL /
  OCSP, not by editing the bundle.
- There is no inbound network listener for OIDC, forward-auth,
  or static-bearer. Those auth modes are ignored under this
  profile (Load() will reject them in gRPC transport).

## Control plane

The example env file pins a postgres DSN. A single-process gRPC
deployment could technically use a file backend, but production
gRPC tends to run HA, so the profile guidance is to skip the file
stage entirely — the upgrade from "single-process file" to
"multi-process postgres" is painful under load. If you truly have
a single-process constraint, set
`MCP_CONTROL_PLANE_DSN=file:///...` and `MCP_ALLOW_DEV_BACKEND=1`
explicitly; the existing Wave H fail-closed guard will accept it
because the override is explicit.

## Upgrade path

When a public-internet caller appears, move to
`profile-single-tenant-http.md` (static bearer) or
`production-profile-shared-service.md` (OIDC). The gRPC transport
does not ship a "public" variant — public-internet gRPC is
typically terminated at a gateway that translates to HTTP anyway.
