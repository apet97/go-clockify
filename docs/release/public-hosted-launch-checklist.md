# Public Hosted Launch Checklist

The pre-flight gates for taking a clockify-mcp deployment from
"works on my cluster" to "accepting traffic from clients we don't
control." Run through every section before opening the front door.

## Pre-flight command gate
- [ ] `clockify-mcp doctor --profile=prod-postgres --strict` exits 0 against the deployment environment
- [ ] `clockify-mcp doctor --profile=prod-postgres --strict --check-backends` exits 0 against the deployment environment
- [ ] strict-doctor output archived with the deploy PR or release notes

`--check-backends` must be run with the Postgres-capable binary built with
`-tags=postgres`. The default binary reports a strict finding explaining that
Postgres backend checks require `-tags=postgres`; it does not silently pass
hosted backend verification.

## Security
- [ ] MCP_PROFILE=prod-postgres applied
- [ ] MCP_OIDC_STRICT=1 (audience or resource URI bound)
- [ ] MCP_REQUIRE_TENANT_CLAIM=1
- [ ] MCP_DISABLE_INLINE_SECRETS=1
- [ ] CLOCKIFY_POLICY=time_tracking_safe (or stricter, with documented reason)
- [ ] No inline credentials in the control-plane DB
- [ ] OIDC `MCP_OIDC_AUDIENCE` or `MCP_RESOURCE_URI` set (RFC 8707 binding)
- [ ] If mTLS is used: `MCP_MTLS_TENANT_SOURCE=cert` (default) and `MCP_REQUIRE_MTLS_TENANT=1`
- [ ] `MCP_EXPOSE_AUTH_ERRORS=0` (default; clients must not see internal error detail)

## Storage
- [ ] Postgres backend built with `-tags=postgres`
- [ ] Migration 002_audit_phase applied and backend reachable (`clockify-mcp doctor --profile=prod-postgres --strict --check-backends` exits 0)
- [ ] Audit retention (`MCP_CONTROL_PLANE_AUDIT_RETENTION`) set per compliance
- [ ] Backup / restore runbook tested in staging within the last 90 days

With the Postgres-capable binary, `--check-backends` verifies that the database
is reachable, the control-plane backend opens cleanly, and the audit phase
migration is visible through the audit write/read health check.

## CI / release
- [ ] Docker smoke uses streamable_http with the static-bearer + memory + dev-backend env
- [ ] Live contract job is required on main (not optional)
- [ ] Release smoke (tag-driven) green for the version being shipped
- [ ] SLSA build provenance attested for the image digest you're rolling out
- [ ] Container image pinned by digest in the deployment manifest (no `:latest`)

## Governance
- [ ] At least one non-author review on the deploy PR
- [ ] CODEOWNERS review enabled (target state — track the branch-protection snapshot)
- [ ] Signed commits enabled (target state)
- [ ] Admin enforcement enabled (target state)
- [ ] Security disclosure dry-run completed against SECURITY.md within the last quarter

## References
- [Production Profile: Shared Service](../deploy/production-profile-shared-service.md)
- [Support Matrix](../support-matrix.md)
- [Branch Protection Snapshot](../branch-protection.md)
- [Governance](../../GOVERNANCE.md)
