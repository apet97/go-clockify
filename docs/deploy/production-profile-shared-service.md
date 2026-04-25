# Production Profile: Shared Service

> Apply with `clockify-mcp --profile=shared-service` (or
> `clockify-mcp --profile=prod-postgres` to add `ENVIRONMENT=prod`
> enforcement in one shot) or `MCP_PROFILE=shared-service`. Example
> env file:
> [`deploy/examples/env.shared-service.example`](../../deploy/examples/env.shared-service.example).
> See also: [`internal/config/profile.go`](../../internal/config/profile.go)
> for the pinned defaults, [ADR-0015](../adr/0015-profile-centric-configuration.md)
> for the design rationale.

This document defines the single blessed production profile for deploying `clockify-mcp` as a shared service. It prioritizes reliability, security, and strict observability over flexibility.

## Canonical Configuration

The `shared-service` profile pins the strict hosted-service defaults below;
operators only need to provide the deployment-specific values (DSN, issuer,
audience). Apply via `MCP_PROFILE=shared-service` or
`--profile=shared-service`. Use `prod-postgres` for the same posture plus
`ENVIRONMENT=prod` enforcement.

```env
MCP_PROFILE=shared-service

# Operator-supplied (no profile defaults)
MCP_CONTROL_PLANE_DSN=postgres://user:pass@db-host:5432/clockify_mcp?sslmode=verify-full
MCP_OIDC_ISSUER=https://auth.example.com/
MCP_OIDC_AUDIENCE=clockify-mcp-shared
# MCP_RESOURCE_URI=https://mcp.example.com   # optional RFC 8707 resource indicator

# Pinned by the profile (override if you really know what you're doing):
#   MCP_TRANSPORT=streamable_http
#   MCP_AUTH_MODE=oidc
#   MCP_AUDIT_DURABILITY=fail_closed
#   MCP_HTTP_LEGACY_POLICY=deny
#   MCP_OIDC_STRICT=1
#   MCP_REQUIRE_TENANT_CLAIM=1
#   MCP_DISABLE_INLINE_SECRETS=1
#   CLOCKIFY_POLICY=time_tracking_safe

# Observability: Dedicated metrics port (bind to localhost or scrape behind
# a NetworkPolicy; never expose publicly).
MCP_METRICS_BIND=:9091
MCP_HTTP_INLINE_METRICS_ENABLED=false
```

### Strict-gate rationale

Each profile-pinned strict flag closes a specific hosted-service
footgun. Removing one without a documented reason re-introduces the
finding it was added to fix.

| Flag | What it does | Footgun without it |
|---|---|---|
| `MCP_OIDC_STRICT=1` | Reject tokens without `aud` matching `MCP_OIDC_AUDIENCE` or `MCP_RESOURCE_URI`; reject tokens missing `exp`. | Any-audience tokens minted by the same issuer for a different relying party are accepted. |
| `MCP_REQUIRE_TENANT_CLAIM=1` | Reject tokens whose tenant claim is empty. | Tokens omitting the claim collapse into `MCP_DEFAULT_TENANT_ID`, sharing one tenant across every misconfigured caller. |
| `MCP_DISABLE_INLINE_SECRETS=1` | Reject credential refs with `backend=inline`. | Inline credentials sit in the control-plane DB and survive operator forgetfulness; vault-backed refs rotate on revoke. |
| `CLOCKIFY_POLICY=time_tracking_safe` | Permit time-entry CRUD + tags; deny workspace-level project / client / task create writes. | The default `standard` policy lets an AI agent create projects in the operator's workspace without explicit consent. |

The `time_tracking_safe` choice is the AI-facing default. Trusted-team
deployments that genuinely need broader writes can override to
`safe_core` (or higher) explicitly — the override path is preserved
because the profile only writes to unset keys.

## TLS Proxy Requirements

`clockify-mcp` does **not** terminate TLS itself. It must be deployed behind a TLS-terminating reverse proxy (e.g., Caddy, Envoy, or NGINX).
*   **Exposure:** The proxy should only expose the `/mcp` endpoints and the readiness/liveness health endpoints to the public internet.
*   **Metrics:** The metrics port (`:9091`) must **never** be exposed publicly.
*   **Headers:** Use `forward_auth` if the proxy handles OIDC and passes user context via headers.

## Database Constraints

*   **Postgres Only:** Only Postgres is supported for the control plane backend in a production shared-service environment.
*   **Dev Backends:** In-memory or file-based dev backends are strictly prohibited in production and will cause the application to fail to start if `ENVIRONMENT=prod`.

## Rollout and Rollback Steps

### Rollout
Deployments must use digest-pinned images. See the deployment verification runbook for details on `cosign` and `gh attestation` verification before rollout.

### Rollback
If a rollout fails, revert to the previous working image digest. Do not attempt to fix issues by manually editing deployment YAML files in the cluster.

## Health and Readiness Endpoints

*   **Liveness:** `/health` (Ensures the process is running)
*   **Readiness:** `/ready` (Ensures the process is ready to accept traffic and can connect to its dependencies, like Postgres)
