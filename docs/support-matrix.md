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

*   **`mtls` for `http`:** Not supported directly. Terminate TLS upstream and use `forward_auth` to pass user context.
*   **`grpc`:** Requires building with `-tags=grpc`.

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
