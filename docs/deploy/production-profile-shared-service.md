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
#
# Profile-driven hosted-mode safety defaults (override with the matching
# env var only when you understand the cross-tenant implications):
#   CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1   (omit Clockify response bodies on the wire)
#   CLOCKIFY_WEBHOOK_VALIDATE_DNS=1       (resolve webhook hosts; reject private/reserved replies)
#
# Optional escape hatch when the DNS gate is on (no profile default):
#   CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=webhook.example.com,.internal.example.com
#   (admit known-trusted hosts whose split-horizon DNS reply would
#   otherwise look private; exact or leading-dot suffix; see
#   docs/runbooks/webhook-dns-validation.md §4b)
#
# Refused outright under shared-service / prod-postgres:
#   CLOCKIFY_INSECURE=1                   (remote HTTP would leak per-tenant API keys)

# Observability: Dedicated metrics port (bind to localhost or scrape behind
# a NetworkPolicy; never expose publicly).
MCP_METRICS_BIND=:9091
MCP_HTTP_INLINE_METRICS_ENABLED=0
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
| `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` (profile default) | Tool-error responses to MCP clients omit upstream Clockify response bodies; full bodies still flow into server-side slog. | A 4xx body from Clockify can carry per-tenant identifiers; without sanitisation those leak across tenant boundaries via the MCP wire. |
| `CLOCKIFY_WEBHOOK_VALIDATE_DNS=1` (profile default) | `CreateWebhook` / `UpdateWebhook` resolve the host and reject any reply containing a private, reserved, link-local, or loopback IP. | A hostname resolving to `169.254.169.254` (cloud metadata) or `10.0.0.x` turns the Clockify outbound webhook delivery into an SSRF probe across the hosted control plane. |
| `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]` (no default) | Comma-separated escape-hatch list that bypasses the DNS gate above. Each entry matches exactly (`webhook.example.com`) or as a leading-dot suffix anchored at a full DNS label (`.example.com`). Use case: split-horizon DNS where a known-trusted hostname legitimately resolves to a private IP only on the control-plane network — see `docs/runbooks/webhook-dns-validation.md` §4b. | Without this hatch, operators on split-horizon DNS would have to disable `CLOCKIFY_WEBHOOK_VALIDATE_DNS` entirely (loose for every host) instead of admitting the one trusted hostname (tight for every other host). |
| `CLOCKIFY_INSECURE=1` is **refused** | Profile rejects the override at startup with an actionable error. | Remote HTTP in a multi-tenant deployment leaks per-tenant Clockify API keys to anything in the network path. |

The `time_tracking_safe` choice is the AI-facing default. Trusted-team
deployments that genuinely need broader writes can override to
`safe_core` (or higher) explicitly — the override path is preserved
because the profile only writes to unset keys.

## TLS Deployment Options

`clockify-mcp` supports two deployment shapes for shared-service TLS.
Pick one explicitly per environment — silently mixing them is how
"who terminates TLS?" incidents start.

### Recommended: reverse-proxy TLS termination

Run `clockify-mcp` on a private listener behind a TLS-terminating
reverse proxy (Caddy, Envoy, NGINX, or an ingress controller). This is
the default shape the `shared-service` profile assumes — `MCP_HTTP_BIND`
is unset and HTTP traffic is plaintext on the private network.

*   **Exposure:** The proxy should only expose the `/mcp` endpoints and
    the readiness/liveness health endpoints to the public internet.
*   **Metrics:** The metrics port (`:9091`) must **never** be exposed
    publicly. Bind it to localhost or scrape it behind a NetworkPolicy.
*   **Headers:** Preserve `Host` and `X-Forwarded-Proto`. Set
    `MCP_ALLOWED_ORIGINS` to the public origin and keep
    `MCP_STRICT_HOST_CHECK=1`.
*   **Auth:** If the proxy already handles OIDC, pass user context via
    `forward_auth` headers (`X-Forwarded-User` / `X-Forwarded-Tenant`).
    Otherwise let `clockify-mcp` validate OIDC tokens directly.

### Supported: native streamable-HTTP TLS

The `streamable_http` transport can terminate TLS itself when both env
vars are set. Use this when the operator owns the certificate
material and wants to remove the proxy hop.

```env
MCP_TRANSPORT=streamable_http      # already pinned by the profile
MCP_HTTP_TLS_CERT=/etc/clockify-mcp/server.crt
MCP_HTTP_TLS_KEY=/etc/clockify-mcp/server.key
```

For native mTLS, additionally set:

```env
MCP_AUTH_MODE=mtls                 # overrides the profile's oidc default
MCP_MTLS_CA_CERT_PATH=/etc/clockify-mcp/client-ca.pem
MCP_MTLS_TENANT_SOURCE=cert        # default; do not change in hosted strict
MCP_REQUIRE_MTLS_TENANT=1          # reject clients whose cert lacks tenant identity
```

Native mTLS expects every client to present a verified certificate;
behind a proxy, switch back to `forward_auth` or OIDC.

### Unsupported: legacy `http` transport

Legacy `MCP_TRANSPORT=http` is POST-only and does **not** terminate TLS
natively. The `shared-service` profile pins
`MCP_HTTP_LEGACY_POLICY=deny`, so opting back into legacy http is a
deliberate operator choice — and even then, terminate TLS at a reverse
proxy (`forward_auth` or upstream OIDC). Prefer `streamable_http` for
all new deployments.

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

## How to verify this deployment

Two artefacts gate the shared-service profile:

1.  **Strict doctor** — confirms the configured profile, backend
    DSN, and audit posture are production-shaped:

    ```bash
    clockify-mcp-postgres doctor \
      --profile=shared-service \
      --strict --check-backends
    ```

    Exit 0 proves migrations have been applied, the
    `audit_events.phase` column is present, and the audit
    round-trip (DoctorCheck) succeeded against the live Postgres
    DSN. This is the same check the
    `Doctor Postgres backend` job in `.github/workflows/ci.yml`
    runs on every PR.

2.  **Shared-service E2E** — proves that streamable HTTP, the
    Postgres control plane, the per-tenant runtime factory, and
    the two-phase audit pipeline integrate correctly under
    multi-tenant traffic:

    ```bash
    MCP_LIVE_CONTROL_PLANE_DSN=postgres://... \
      make shared-service-e2e
    ```

    The target boots `mcp.ServeStreamableHTTP` in-process against
    the supplied Postgres, drives 5 calls across two distinct
    `forward_auth` principals (one operator persona, one
    AI-facing persona on `time_tracking_safe`), and asserts
    tenant isolation in `audit_events` and `sessions` rows. The
    test stands up an `httptest` fake Clockify locally so no live
    upstream secrets are required. CI runs the same target in the
    `Shared-service Postgres E2E` job in
    `.github/workflows/ci.yml` against a `postgres:16-alpine`
    service container.

A green run of both artefacts is the launch-candidate gate for
this profile (Group 2 of `docs/launch-candidate-checklist.md`).
