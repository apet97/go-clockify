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

For a production deployment, the following environment variables MUST be configured exactly as shown:

```env
# Transport: Spec-strict streamable HTTP (MCP 2025-03-26+)
MCP_TRANSPORT=streamable_http

# Control Plane: Postgres is mandatory for multi-process / HA
MCP_CONTROL_PLANE_DSN=postgres://user:pass@db-host:5432/clockify_mcp?sslmode=verify-full

# Auth: OIDC with JWT verification
MCP_AUTH_MODE=oidc
MCP_OIDC_ISSUER=https://auth.example.com/
MCP_OIDC_AUDIENCE=clockify-mcp-shared
# Hosted-service strict mode: rejects tokens that aren't bound to this
# server (no audience/resource claim) and tokens missing an exp claim.
MCP_OIDC_STRICT=1
# Reject tokens whose tenant claim is empty rather than collapsing them
# into MCP_DEFAULT_TENANT_ID. Required for any multi-tenant deployment.
MCP_REQUIRE_TENANT_CLAIM=1

# Observability: Dedicated metrics port
MCP_METRICS_BIND=:9091
MCP_HTTP_INLINE_METRICS_ENABLED=false

# Safety and Compliance
MCP_AUDIT_DURABILITY=fail_closed
CLOCKIFY_POLICY=safe_core
```

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
