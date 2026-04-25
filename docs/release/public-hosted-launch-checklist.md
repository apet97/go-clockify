# Public Hosted Launch Checklist

The pre-flight gates for taking a clockify-mcp deployment from
"works on my cluster" to "accepting traffic from clients we don't
control." Run through every section before opening the front door.

## Pre-flight command gate
- [ ] `clockify-mcp doctor --profile=prod-postgres --strict` exits 0 against the deployment environment
- [ ] `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` exits 0 against the deployment environment
- [ ] strict-doctor output archived with the deploy PR or release notes

The pre-flight gate uses two distinct binaries on purpose:

- **Default binary `clockify-mcp-{linux,darwin,windows}-{x64,arm64}`** —
  stdlib-only by design (ADR 0001 keeps `pgx` out of `go.mod`). Use it
  for local self-hosted stdio mode and for the strict-posture
  *config-only* gate (`doctor --strict`). It deliberately *fails*
  `--check-backends` with a finding that reads
  `--check-backends requires a binary built with -tags=postgres to
  verify Postgres reachability and migrations`. Default release
  binaries cannot satisfy the hosted backend launch gate.
- **Postgres binary `clockify-mcp-postgres-linux-{x64,arm64}`** — the
  hosted-deploy artifact. Links the pgx-backed control-plane store
  and runs the live `DoctorCheck(ctx)` round-trip on a real Postgres
  instance. This is the binary that backs production hosted
  deployments and is the only binary that satisfies
  `doctor --strict --check-backends`.

Operators can either:

1. **(Recommended)** Download
   `clockify-mcp-postgres-linux-x64` /
   `clockify-mcp-postgres-linux-arm64` from the GitHub release the
   deploy is rolling out. Verify its sigstore bundle and SLSA
   attestation alongside the default binary (the release-smoke
   workflow does this on every published tag and weekly).
2. **(Self-build)** Build from a tagged source tree:

   ```sh
   git checkout vX.Y.Z
   go build -tags=postgres -o clockify-mcp-postgres ./cmd/clockify-mcp
   ```

   Self-builds bypass the cosign / SLSA chain and are only acceptable
   for emergency rollouts; document the deviation in the deploy PR.

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
- [ ] TLS termination wired (one of):
  - reverse-proxy TLS in front of `streamable_http` (recommended), **or**
  - native streamable TLS via `MCP_HTTP_TLS_CERT` + `MCP_HTTP_TLS_KEY`, **or**
  - native streamable mTLS via `MCP_HTTP_TLS_CERT` + `MCP_HTTP_TLS_KEY` + `MCP_MTLS_CA_CERT_PATH` with `MCP_AUTH_MODE=mtls`

## Storage
- [ ] Hosted deploy uses `clockify-mcp-postgres-linux-{x64,arm64}` (or a self-built `-tags=postgres` binary documented in the deploy PR)
- [ ] Migration 002_audit_phase applied and backend reachable (`clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` exits 0)
- [ ] Audit retention (`MCP_CONTROL_PLANE_AUDIT_RETENTION`) set per compliance
- [ ] Backup / restore runbook tested in staging within the last 90 days

With the postgres-tagged binary, `--check-backends` verifies that the database
is reachable, the control-plane backend opens cleanly, the
`schema_migrations` row for migration 002 is present, the
`audit_events.phase` column exists, and a synthetic audit event written by
`DoctorCheck(ctx)` round-trips. This is a live test, not a config-string
check — strict posture passing on the default binary is *not* a substitute.

## CI / release
- [ ] Docker smoke uses streamable_http with the static-bearer + memory + dev-backend env
- [ ] Live contract job is required on main (not optional)
- [ ] Release smoke (tag-driven) green for the version being shipped
- [ ] SLSA build provenance attested for the image digest you're rolling out
- [ ] Container image pinned by digest in the deployment manifest (no `:latest`)
- [ ] Live coverage gaps tracked or closed before paid traffic
      (`TestLiveDryRunDoesNotMutate`, `TestLivePolicyTimeTrackingSafeBlocksProjectCreate`,
      `TestLiveCreateUpdateDeleteEntryAuditPhases` — see
      [`docs/live-tests.md`](../live-tests.md#required-live-coverage-before-paid-hosted-launch))

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
