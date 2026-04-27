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
- Hosted profiles automatically tighten three additional defaults that
  would otherwise be operator footguns. Each is overridable with an
  explicit env var, but the default is the safe one:
  - `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` — strip upstream Clockify
    response bodies from MCP tool-error responses (full bodies still
    flow into slog).
  - `CLOCKIFY_WEBHOOK_VALIDATE_DNS=1` — DNS-resolve webhook hosts and
    reject any private/reserved IP reply (SSRF protection). Pair
    with `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]` to
    admit known-trusted hostnames whose split-horizon DNS reply
    would otherwise look like a private IP — see
    `docs/runbooks/webhook-dns-validation.md` §4b.
  - `CLOCKIFY_INSECURE=1` is **refused** at startup; remote HTTP
    leaks per-tenant API keys.

## Canonical Configuration

Apply one of the two registered profiles that match this guide's
shape (see [`docs/deploy/`](../deploy/) for full profile notes):

- `clockify-mcp --profile=shared-service` — multi-tenant baseline.
  Sets `MCP_TRANSPORT=streamable_http`, `MCP_AUTH_MODE=oidc`,
  `MCP_AUDIT_DURABILITY=fail_closed`, `MCP_HTTP_LEGACY_POLICY=deny`,
  `MCP_OIDC_STRICT=1`, `MCP_REQUIRE_TENANT_CLAIM=1`,
  `MCP_DISABLE_INLINE_SECRETS=1`,
  `CLOCKIFY_POLICY=time_tracking_safe`.
- `clockify-mcp --profile=prod-postgres` — same posture plus
  `ENVIRONMENT=prod` so downstream prod-only assertions
  (release-asset checks, hosted-profile drills) treat this as the
  production line. Use this for the production blue/green; keep
  `shared-service` for staging.

`deploy/examples/env.shared-service.example` is preserved as a
starting reference for operators populating Helm values or
Kustomize secrets, but the env keys it sets are now also covered
by the profiles above. Override individual keys (e.g.
`CLOCKIFY_POLICY=safe_core` for trusted assistants that need
workspace object creation) by setting them explicitly — the
profile leaves any explicit value through.

Other knobs the profile does not own:

- **`CLOCKIFY_POLICY=time_tracking_safe`** (profile default):
  Mandatory AI-facing default for multi-tenant environments. Use
  `safe_core` only for trusted assistants that need workspace
  object creation.
- **`MCP_METRICS_BIND=:9091`**: Dedicated listener for Prometheus.

## Audit + Risk Metadata

Every Tier-1 and Tier-2 tool descriptor carries structured risk
metadata that the audit recorder consumes:

- **`RiskClass` (bitmask):** `Read | Write | Billing | Admin |
  PermissionChange | ExternalSideEffect | Destructive`. Defaults
  derive from the existing read-only / destructive boolean hints; the
  `internal/tools/risk_overrides.go` registry layers finer
  distinctions on top (billing for invoice tools, admin +
  permission_change for `clockify_update_user_role`, external
  side effect for `clockify_test_webhook` and outbound invoice sends).
- **`AuditKeys`:** action-defining argument keys that get captured in
  audit events alongside the implicit `*_id` scan — so a
  permission-change record carries the new role and a billing record
  carries the quantity / unit_price / status that defines the action.

The matrix test `internal/tools/risk_class_test.go` fails the build
if a new tool descriptor is added without a `RiskClass`. Consumers
(policy, enforcement, audit) can pattern-match against bits to gate
sensitive actions. See `docs/policy/production-tool-scope.md` for the
taxonomy mapped to category names.
