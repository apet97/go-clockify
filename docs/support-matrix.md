# Support Matrix

This document outlines the supported configurations and clients for `clockify-mcp-go`.

## Transports and Auth Modes

The following matrix shows supported authentication modes for each transport.

| Transport | `static_bearer` | `oidc` | `forward_auth` | `mtls` | Use Case |
|-----------|:---------------:|:------:|:--------------:|:------:|----------|
| `stdio`   | N/A | N/A | N/A | N/A | Local CLI clients (Claude Code, Cursor) |
| `http` (Legacy) | ✅ | ✅ | ✅ | ❌ | Single-tenant HTTP service |
| `streamable_http` | ✅ | ✅ | ✅ | ✅ | Multi-tenant shared service (Recommended) |
| `grpc` | ✅ | ✅ | ✅ | ✅ | Low-latency private network (Requires build tags) |

*   **`mtls` for `http` (legacy):** Not supported. The legacy
    HTTP transport does not terminate TLS in-process; setting
    `MCP_HTTP_TLS_CERT` with `MCP_TRANSPORT=http` is rejected at
    `config.Load`. Terminate TLS upstream and use `forward_auth`
    to pass user context.
*   **`mtls` for `streamable_http`:** Supported natively via
    `MCP_HTTP_TLS_CERT` + `MCP_HTTP_TLS_KEY` + `MCP_MTLS_CA_CERT_PATH`.
    All three are required when `MCP_AUTH_MODE=mtls` is selected
    on the streamable transport; `config.Load` rejects partial
    configurations at startup.
*   **`mtls` for `grpc`:** Same cert / key / CA requirement as
    streamable HTTP. Plus the `grpc` build tag (see below).
*   **`grpc`:** Requires building with `-tags=grpc`.

## Production-readiness classification

"✅" above means a combination is functionally supported. This
table classifies combinations by production suitability — a
combination can be supported in principle but still be the wrong
choice for a given shape of deployment.

| Deployment shape | Transport | Auth | Control-plane | Classification |
|------------------|-----------|------|---------------|----------------|
| Single user, laptop subprocess | `stdio` | n/a | `memory` | ✅ Recommended |
| Small team, shared HTTP endpoint | `streamable_http` | `static_bearer` | `file://` | ✅ Recommended |
| Small team, shared HTTP endpoint | `http` (legacy) | `static_bearer` | `file://` | ⚠️ Tolerated (no server-initiated notifications) |
| Multi-tenant shared service | `streamable_http` | `oidc` | `postgres://` | ✅ Recommended |
| Multi-tenant shared service | `streamable_http` | `forward_auth` | `postgres://` | ⚠️ Tolerated (proxy owns identity; double-check header stripping) |
| Multi-tenant shared service | `http` (legacy) | any | any | ❌ Unsupported (no per-session notifications) |
| Multi-tenant shared service | any | `static_bearer` | any | ❌ Unsupported (no per-user identity) |
| Private mesh, low-latency RPC | `grpc` | `oidc` or `mtls` | `postgres://` | ✅ Recommended |
| Any | any | any | `memory` (ENVIRONMENT=prod) | ❌ Fails closed at startup |

Legend:

- **✅ Recommended** — The documented deployment profile uses
  this combination; CI smoke tests cover it; runbooks reference
  it by name. Pick one of these unless you have a specific
  reason to deviate.
- **⚠️ Tolerated** — Functionally works; release tests cover
  it; but the combination has a known sharp edge (missing
  notifications, external trust boundary, etc.). Safe to run if
  you understand the tradeoff.
- **❌ Unsupported** — Either blocked at startup (e.g. `memory`
  backend with `ENVIRONMENT=prod`) or actively discouraged
  because the security/operational posture of the combination
  fails at least one of: fail-closed audit, authenticated
  transport, multi-process-safe control plane, or deny-default
  legacy HTTP.

Every "Recommended" row has a corresponding file under
`docs/deploy/`:

- `docs/deploy/profile-local-stdio.md`
- `docs/deploy/profile-single-tenant-http.md`
- `docs/deploy/production-profile-shared-service.md`

## Supported MCP Clients

`clockify-mcp-go` is tested against the following MCP-compliant clients.

| Client | Mode | Transport | Notes |
|--------|------|-----------|-------|
| Claude Code | CLI | `stdio` | Recommended for terminal users |
| Claude Desktop | Desktop App | `stdio` | Official Anthropic client |
| Cursor | IDE | `stdio` | Deep IDE integration via `.cursor/mcp.json` |
| Codex | CLI | `stdio` | Lightweight CLI |
| Custom HTTP Client | API | `streamable_http` | For building custom dashboards/tools |

## Runtime Environments

| Platform | Support Level | Notes |
|----------|---------------|-------|
| Linux (amd64/arm64) | Tier 1 | Full CI coverage, distroless images |
| macOS (Darwin/Silicon) | Tier 1 | Primary development platform |
| Windows (x64) | Tier 2 | Binary release, limited CI |
| Kubernetes | Tier 1 | Reference Helm/Kustomize manifests |

## Control-Plane Backends

| Backend | Stability | Use Case |
|---------|-----------|----------|
| `memory` | Stable | Tests, single-user `stdio` |
| `file://` | Stable | Small-team `stdio`, local development |
| `postgres://` | Stable | Production shared-service (Recommended) |
