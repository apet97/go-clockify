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

The gRPC transport lives behind the `grpc` build tag and is **not**
included in the default `clockify-mcp` binary or the default Docker
image (built without `-tags=grpc`). Operators have two supported
paths to a gRPC-capable build:

1. **(Recommended)** Download a first-class published artifact from
   the GitHub release the deploy is rolling out:
   - `clockify-mcp-grpc-linux-x64` / `clockify-mcp-grpc-linux-arm64`
     for single-process gRPC.
   - `clockify-mcp-grpc-postgres-linux-x64` /
     `clockify-mcp-grpc-postgres-linux-arm64` for HA gRPC with the
     pgx-backed control-plane store. The hosted-launch checklist
     contracts on these names.
   These ship with the same SBOM + cosign sigstore + SLSA
   attestation chain as the default and Postgres binaries; the
   `scripts/check-release-assets.sh` post-goreleaser gate refuses to
   ship a release that drops them.

2. **(Self-build)** Compile from a tagged source tree:

   ```sh
   go build -tags=grpc          ./cmd/clockify-mcp     # private-network gRPC, no postgres
   go build -tags=grpc,postgres ./cmd/clockify-mcp     # HA gRPC + pgx control plane
   ```

   Or build a custom container image:

   ```sh
   docker build \
     --build-arg GO_TAGS=grpc,postgres \
     -f deploy/Dockerfile \
     -t clockify-mcp:grpc-postgres .
   ```

   Self-builds bypass the cosign / SLSA chain and should be reserved
   for emergency rollouts; document the deviation in the deploy PR.

If `clockify-mcp` exits with `MCP_TRANSPORT=grpc requires
-tags=grpc at build time`, the running binary is the default build
— switch to a `-grpc` artifact or rebuild with the tag.

## Security model

- The caller's identity is the Subject/SAN of their client
  certificate. Tenant extraction defaults to
  `MCP_MTLS_TENANT_SOURCE=cert`, which reads URI SAN
  `clockify-mcp://tenant/<id>` or `spiffe://*/tenant/<id>`, then
  falls back to Subject Organization. `X-Tenant-ID` request
  metadata is **ignored** unless the operator explicitly sets
  `MCP_MTLS_TENANT_SOURCE=header` or `header_or_cert`; those modes
  invert the trust model and are only acceptable behind a proxy
  that strips client-supplied tenant headers.
- The server cert / key must be PEM files readable by the process
  user. Paths come from `MCP_GRPC_TLS_CERT` and
  `MCP_GRPC_TLS_KEY`.
- The client CA bundle (`MCP_MTLS_CA_CERT_PATH`) must contain
  every CA whose certs you trust. Revoke via your CA's CRL /
  OCSP, not by editing the bundle.
- This profile defaults to `MCP_AUTH_MODE=mtls`. Other gRPC auth
  modes (static_bearer, OIDC, forward_auth) exist for specialised
  deployments — they are not the recommended private-network
  posture. If you override auth mode, re-run `clockify-mcp doctor
  --strict` and document the deployment threat model in the deploy
  PR; the profile's `MCP_REQUIRE_MTLS_TENANT=1` invariant only
  applies to mTLS.

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
