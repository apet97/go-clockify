# Production Profile: Shared Service

This document defines the single blessed production profile for deploying `clockify-mcp` as a shared service. It prioritizes reliability, security, and strict observability over flexibility.

## Required Environment Variables

For a production deployment, the following environment variables MUST be configured exactly as shown:

```env
MCP_TRANSPORT=streamable_http
MCP_CONTROL_PLANE_DSN=postgres://...
MCP_AUTH_MODE=oidc
MCP_METRICS_BIND=:9091
MCP_METRICS_AUTH_MODE=static_bearer
MCP_HTTP_INLINE_METRICS_ENABLED=false
MCP_AUDIT_DURABILITY=fail_closed
CLOCKIFY_POLICY=safe_core
MCP_STRICT_HOST_CHECK=1
```

## TLS Proxy Requirements

`clockify-mcp` does **not** terminate TLS itself. It must be deployed behind a TLS-terminating reverse proxy.
*   **Exposure:** The proxy should only expose the `/mcp` endpoints and the readiness/liveness health endpoints to the public internet (or internal network, depending on use case).
*   **Metrics:** The metrics port (`:9091`) must **never** be exposed publicly.
*   **CORS/Origins:** Set explicit values for `MCP_ALLOWED_ORIGINS` at the proxy or application level. Do not use `MCP_ALLOW_ANY_ORIGIN=true`.

## Database Constraints

*   **Postgres Only:** Only Postgres is supported for the control plane backend in a production shared-service environment.
*   **Dev Backends:** In-memory or file-based dev backends are strictly prohibited in production and will cause the application to fail to start if `ENVIRONMENT=prod`.

## Rollout and Rollback Steps

### Rollout
Deployments must use digest-pinned images. See the deployment verification runbook for details on `cosign` and `gh attestation` verification before rollout.

### Rollback
If a rollout fails, revert to the previous working image digest. Do not attempt to fix issues by manually editing deployment YAML files in the cluster to use different mutable tags.

## Health and Readiness Endpoints

*   **Liveness:** `/healthz` (Ensures the process is running)
*   **Readiness:** `/readyz` (Ensures the process is ready to accept traffic and can connect to its dependencies, like Postgres)
