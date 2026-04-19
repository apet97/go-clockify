# Operator Guide: Shared Service (Managed)

This guide is for platform teams operating `clockify-mcp-go` as a multi-tenant shared service for their organization.

## Architecture
- **Transport:** `streamable_http`
- **Control Plane:** Postgres (Mandatory)
- **Auth:** OIDC (JWT verification)
- **Infrastructure:** Kubernetes (Helm or Kustomize)

## Key Responsibilities

### 1. Database Management
- Maintain a highly available Postgres cluster.
- Schedule regular backups and perform restore drills (see `docs/runbooks/postgres-restore-drill.md`).
- Monitor database connections and performance.

### 2. Identity and Access
- Manage the OIDC provider (Auth0, Okta, etc.) and its integration.
- Revoke tokens as necessary by updating the provider's JWKS or decreasing `MCP_OIDC_VERIFY_CACHE_TTL`.

### 3. Monitoring and SLOs
- Define and track Service Level Objectives (SLOs) based on metrics exposed at `:9091`.
- Act on burn-rate alerts for the 99.9% availability target.

### 4. Security and Compliance
- Ensure image signatures are verified before deployment.
- Maintain audit log durability by setting `MCP_AUDIT_DURABILITY=fail_closed`.
- Perform regular vulnerability scans on the container image.

## Canonical Configuration
Use the `deploy/examples/env.shared-service.example` preset as a starting point.
- **`CLOCKIFY_POLICY=safe_core`**: Mandatory for multi-tenant environments.
- **`MCP_METRICS_BIND=:9091`**: Dedicated listener for Prometheus.
