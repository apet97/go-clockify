# Chart / Kustomize env var parity audit

**Last updated:** 2026-04-12 (Wave 4 / W4-02)
**Ground truth:** `internal/config/config.go::Load`

This document tracks the delta between the env vars `Config.Load`
consumes and the env vars reachable through the Helm chart
(`deploy/helm/clockify-mcp/`) and Kustomize base
(`deploy/k8s/base/`). It exists because Wave 3 added the gRPC
transport and the entire authn surface (static-bearer, OIDC, forward
auth, mTLS, per-tenant claims, control plane) without plumbing the
knobs through the manifest renderers. Wave 4 exposes the gRPC
transport only; the remaining 22 gaps are backlog.

## Coverage matrix

| Env var | `config.go` site | Helm | Kustomize | Status |
|---|---|:---:|:---:|---|
| **Core (Clockify API)** | | | | |
| `CLOCKIFY_API_KEY` | `:70` | ✅ Secret | ✅ Secret | Covered |
| `CLOCKIFY_WORKSPACE_ID` | `:71` | ❌ | ❌ | W5 |
| `CLOCKIFY_BASE_URL` | `:72` | ❌ | ❌ | W5 |
| `CLOCKIFY_TIMEZONE` | `:83` | ❌ | ❌ | W5 |
| `CLOCKIFY_INSECURE` | `:73` | ❌ | ❌ | W5 |
| **Transport** | | | | |
| `MCP_TRANSPORT` | `:91` | ✅ `transport.mode` | ✅ hardcoded | Covered |
| `MCP_HTTP_BIND` | `:133` | ✅ `transport.bind` | ✅ hardcoded | Covered |
| `MCP_GRPC_BIND` | `:138` | ✅ `transport.grpcBind` *(W4-02)* | ✅ commented | **W4-02** |
| `MCP_HTTP_MAX_BODY` | `:232` | ✅ ConfigMap | ✅ ConfigMap | Covered |
| **Auth (HTTP transports only)** | | | | |
| `MCP_AUTH_MODE` | `:110` | ❌ | ❌ | W5 |
| `MCP_BEARER_TOKEN` | `:143` | ✅ Secret | ✅ Secret | Covered |
| `MCP_OIDC_ISSUER` | `:199` | ❌ | ❌ | W5 |
| `MCP_OIDC_AUDIENCE` | `:200` | ❌ | ❌ | W5 |
| `MCP_OIDC_JWKS_URL` | `:201` | ❌ | ❌ | W5 |
| `MCP_OIDC_JWKS_PATH` | `:202` | ❌ | ❌ | W5 |
| `MCP_RESOURCE_URI` | `:203` | ❌ | ❌ | W5 |
| `MCP_FORWARD_TENANT_HEADER` | `:204` | ❌ | ❌ | W5 |
| `MCP_FORWARD_SUBJECT_HEADER` | `:205` | ❌ | ❌ | W5 |
| `MCP_MTLS_TENANT_HEADER` | `:206` | ❌ | ❌ | W5 |
| **CORS / Host check** | | | | |
| `MCP_ALLOWED_ORIGINS` | `:214` | ❌ | ❌ | W5 |
| `MCP_ALLOW_ANY_ORIGIN` | `:224` | ❌ | ❌ | W5 |
| `MCP_STRICT_HOST_CHECK` | `:225` | ✅ `transport.strictHostCheck` | ❌ | W5 |
| **Metrics** | | | | |
| `MCP_METRICS_BIND` | `:152` | ❌ | ❌ | W5 |
| `MCP_METRICS_AUTH_MODE` | `:153` | ❌ | ❌ | W5 |
| `MCP_METRICS_BEARER_TOKEN` | `:162` | ❌ | ❌ | W5 |
| **Control plane (streamable_http)** | | | | |
| `MCP_CONTROL_PLANE_DSN` | `:172` | ❌ | ❌ | W5 |
| `MCP_SESSION_TTL` | `:177` | ❌ | ❌ | W5 |
| `MCP_TENANT_CLAIM` | `:187` | ❌ | ❌ | W5 |
| `MCP_SUBJECT_CLAIM` | `:191` | ❌ | ❌ | W5 |
| `MCP_DEFAULT_TENANT_ID` | `:195` | ❌ | ❌ | W5 |
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

**Totals (post-W4-02):** Helm covers 20/42, Kustomize covers 19/42. 22 gaps remain.

## Latent drift fixed in W4-02

Two bugs surfaced during the audit — both pre-dated Wave 3:

- **`deploy/helm/clockify-mcp/values.yaml:9`** had `image.tag: "0.7.0"`,
  meaning the chart shipped with a stale image since v0.7.1 — the
  `| default .Chart.AppVersion` fallback in `templates/deployment.yaml:40`
  only fires when the tag is empty. **Fix:** set `image.tag: ""` so
  the fallback tracks `.Chart.AppVersion` automatically.
- **`deploy/k8s/base/deployment.yaml:39`** pinned
  `ghcr.io/apet97/go-clockify:v0.5.0`, 3 releases behind. **Fix:**
  bump to `:v0.8.0` so the base tracks the supported release. Operators
  using overlays must still bump intentionally at release time.

## Wave 5 backlog

Cluster the 22 gaps above into three batches for a future session:

1. **Authn surface** (10 vars): `MCP_AUTH_MODE`, all `MCP_OIDC_*`,
   forward/mtls headers. Requires a values schema for the choice
   between `static_bearer`/`oidc`/`forward_auth`/`mtls` and a Secret
   template for the OIDC JWKS JSON blob.
2. **Streamable-HTTP / control plane** (6 vars): `MCP_CONTROL_PLANE_DSN`,
   session TTL, tenant/subject claims, `MCP_RESOURCE_URI`,
   `MCP_DEFAULT_TENANT_ID`. Needed before the chart can meaningfully
   run `MCP_TRANSPORT=streamable_http`.
3. **Observability / CORS** (6 vars): `MCP_METRICS_BIND`,
   metrics auth pair, `MCP_ALLOWED_ORIGINS`, `MCP_ALLOW_ANY_ORIGIN`,
   Kustomize `MCP_STRICT_HOST_CHECK`, plus `CLOCKIFY_WORKSPACE_ID` /
   `CLOCKIFY_BASE_URL` / `CLOCKIFY_TIMEZONE` / `CLOCKIFY_INSECURE` as
   ConfigMap entries.

## Meta: prevent future drift

The right long-term fix is a CI check that diffs a code-generated
list of `Config.Load` env vars against the chart/kustomize surface
and fails the PR if a new env var isn't exposed (or explicitly
opted out). That belongs in Wave 5 alongside the authn surface work.
