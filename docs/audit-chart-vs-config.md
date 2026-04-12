# Chart / Kustomize env var parity audit

**Last updated:** 2026-04-12 (Wave 5 / W5-03)
**Ground truth:** `internal/config/config.go::Load`

This document tracks the delta between the env vars `Config.Load`
consumes and the env vars reachable through the Helm chart
(`deploy/helm/clockify-mcp/`) and Kustomize base
(`deploy/k8s/base/`). Wave 5 closed all 22 gaps remaining from
Wave 4. A CI gate (`scripts/check-config-parity.sh`) now prevents
future drift.

## Coverage matrix

| Env var | `config.go` site | Helm | Kustomize | Status |
|---|---|:---:|:---:|---|
| **Core (Clockify API)** | | | | |
| `CLOCKIFY_API_KEY` | `:70` | ✅ Secret | ✅ Secret | Covered |
| `CLOCKIFY_WORKSPACE_ID` | `:71` | ✅ `clockify.workspaceId` | ✅ ConfigMap | **W5-03** |
| `CLOCKIFY_BASE_URL` | `:72` | ✅ `clockify.baseUrl` | ✅ ConfigMap | **W5-03** |
| `CLOCKIFY_TIMEZONE` | `:83` | ✅ `clockify.timezone` | ✅ ConfigMap | **W5-03** |
| `CLOCKIFY_INSECURE` | `:73` | ✅ `clockify.insecure` | ✅ ConfigMap | **W5-03** |
| **Transport** | | | | |
| `MCP_TRANSPORT` | `:91` | ✅ `transport.mode` | ✅ hardcoded | Covered |
| `MCP_HTTP_BIND` | `:133` | ✅ `transport.bind` | ✅ hardcoded | Covered |
| `MCP_GRPC_BIND` | `:138` | ✅ `transport.grpcBind` | ✅ commented | W4-02 |
| `MCP_HTTP_MAX_BODY` | `:232` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| **Auth** | | | | |
| `MCP_AUTH_MODE` | `:110` | ✅ `auth.mode` | ✅ ConfigMap | **W5-03** |
| `MCP_BEARER_TOKEN` | `:143` | ✅ Secret | ✅ Secret | Covered |
| `MCP_OIDC_ISSUER` | `:199` | ✅ `auth.oidcIssuer` | ✅ ConfigMap | **W5-03** |
| `MCP_OIDC_AUDIENCE` | `:200` | ✅ `auth.oidcAudience` | ✅ ConfigMap | **W5-03** |
| `MCP_OIDC_JWKS_URL` | `:201` | ✅ `auth.oidcJwksUrl` | ✅ ConfigMap | **W5-03** |
| `MCP_OIDC_JWKS_PATH` | `:202` | ✅ `auth.oidcJwksPath` | ✅ Secret | **W5-03** |
| `MCP_RESOURCE_URI` | `:203` | ✅ `auth.resourceUri` | ✅ ConfigMap | **W5-03** |
| `MCP_FORWARD_TENANT_HEADER` | `:204` | ✅ `auth.forwardTenantHeader` | ✅ ConfigMap | **W5-03** |
| `MCP_FORWARD_SUBJECT_HEADER` | `:205` | ✅ `auth.forwardSubjectHeader` | ✅ ConfigMap | **W5-03** |
| `MCP_MTLS_TENANT_HEADER` | `:206` | ✅ `auth.mtlsTenantHeader` | ✅ ConfigMap | **W5-03** |
| **CORS / Host check** | | | | |
| `MCP_ALLOWED_ORIGINS` | `:214` | ✅ `cors.allowedOrigins` | ✅ ConfigMap | **W5-03** |
| `MCP_ALLOW_ANY_ORIGIN` | `:224` | ✅ `cors.allowAnyOrigin` | ✅ ConfigMap | **W5-03** |
| `MCP_STRICT_HOST_CHECK` | `:225` | ✅ `transport.strictHostCheck` | ✅ ConfigMap | **W5-03** |
| **Metrics** | | | | |
| `MCP_METRICS_BIND` | `:152` | ✅ `metricsEndpoint.bind` | ✅ ConfigMap | **W5-03** |
| `MCP_METRICS_AUTH_MODE` | `:153` | ✅ `metricsEndpoint.authMode` | ✅ ConfigMap | **W5-03** |
| `MCP_METRICS_BEARER_TOKEN` | `:162` | ✅ Secret | ✅ Secret | **W5-03** |
| **Control plane (streamable_http)** | | | | |
| `MCP_CONTROL_PLANE_DSN` | `:172` | ✅ `controlPlane.dsn` | ✅ ConfigMap | **W5-03** |
| `MCP_SESSION_TTL` | `:177` | ✅ `controlPlane.sessionTtl` | ✅ ConfigMap | **W5-03** |
| `MCP_TENANT_CLAIM` | `:187` | ✅ `controlPlane.tenantClaim` | ✅ ConfigMap | **W5-03** |
| `MCP_SUBJECT_CLAIM` | `:191` | ✅ `controlPlane.subjectClaim` | ✅ ConfigMap | **W5-03** |
| `MCP_DEFAULT_TENANT_ID` | `:195` | ✅ `controlPlane.defaultTenantId` | ✅ ConfigMap | **W5-03** |
| **Tool execution** | | | | |
| `CLOCKIFY_TOOL_TIMEOUT` | `:248` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT` | `:260` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `MCP_MAX_INFLIGHT_TOOL_CALLS` | `:272` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_REPORT_MAX_ENTRIES` | `:287` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| **Safety / control (existing coverage)** | | | | |
| `CLOCKIFY_POLICY` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_DRY_RUN` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_DEDUPE_MODE` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_DEDUPE_LOOKBACK` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_OVERLAP_CHECK` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_MAX_CONCURRENT` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_RATE_LIMIT` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_TOKEN_BUDGET` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |
| `CLOCKIFY_BOOTSTRAP_MODE` | env | ✅ ConfigMap | ✅ ConfigMap | Covered |

**Totals (post-W5-03):** Helm covers 42/42, Kustomize covers 42/42. Zero gaps.

## CI gate

`scripts/check-config-parity.sh` extracts every `os.Getenv` call from
`internal/config/config.go`, scans Helm values + templates + Kustomize
ConfigMap + Secret + deployment for each var, and fails the PR if any
env var is unreachable and not listed in
`deploy/.config-parity-opt-out.txt`. Added in W5-03d.
