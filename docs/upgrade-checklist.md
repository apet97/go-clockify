# Operator upgrade checklist

This is the flow for upgrading `clockify-mcp` between releases or
moving between deployment profiles. It covers every change that
can silently alter request semantics — config keys, policy modes,
control-plane backend, and MCP protocol version negotiation.

Release cadence and the support-matrix commitments live in
`docs/release-policy.md`. This document is about the mechanical
pre-flight and the per-minor operational tasks.

## 1. Before the upgrade (pre-flight)

### Config diff

- [ ] Compare the source config schema at the target version
      against your current config:

```sh
git diff --no-index \
  <(git show v$CURRENT:internal/config/config.go | grep -E '^\s*[A-Z].*os\.Getenv') \
  <(git show v$TARGET:internal/config/config.go  | grep -E '^\s*[A-Z].*os\.Getenv')
```

- [ ] For every env var added between `$CURRENT` and `$TARGET`,
      check `deploy/.config-parity-opt-out.txt` — if it's listed
      there, the default is safe; if it's not, the var is
      required and must be set before the new binary boots.
- [ ] For every env var removed, confirm your deployment isn't
      still passing it. Extra env vars are ignored, but a stale
      config signals a missed release note.

### Policy mode

- [ ] `clockify_policy_info` on the old server — snapshot the
      effective policy (read_only / time_tracking_safe / safe_core /
      standard / full)
      and the per-tool overrides.
- [ ] If the target release narrows `safe_core` or widens
      `standard`, validate clients don't depend on a tool that's
      about to change allowlist status. Release notes call these
      out explicitly.

### Control-plane backend

- [ ] If moving between backends (`memory` → `file://` →
      `postgres://`), plan a migration window. Audit events are
      not automatically migrated — the old backend goes
      read-only while the new one starts empty.
- [ ] Postgres: run the migrations in `internal/controlplane/postgres/migrations/`
      against the target DB before deploying the new binary.
- [ ] File backend: confirm the target volume is sized for
      `MCP_CONTROL_PLANE_AUDIT_CAP` entries (each audit record
      is ~2KB on disk).

### MCP protocol version

- [ ] Inspect the supported protocol versions in the target
      release (`internal/mcp/server.go:SupportedProtocolVersions`).
      If any client speaks an older version that was dropped,
      upgrade the client first.
- [ ] A client that cannot downgrade will disconnect on
      `initialize` — this is the spec-compliant behaviour and
      not a regression.

## 2. During the rollout

### Single-tenant HTTP (one process)

- [ ] Deploy the new binary to a staging host, pointed at the
      same Clockify API key and the same audit store.
- [ ] Run the smoke tests: `clockify_whoami`, one write call,
      one list call, and a Tier 2 activation.
- [ ] Cut over by updating the systemd unit / reverse proxy;
      the old process drains on SIGTERM (default 30s grace).

### Shared service (multi-replica, Kubernetes)

- [ ] `kubectl rollout restart` with `maxUnavailable: 0` so the
      service stays available through the rollout.
- [ ] Watch `clockify_mcp_http_requests_total` and
      `clockify_mcp_tool_calls_total` deltas; a cutover where
      deltas drop to zero for more than the rollout window
      means traffic is going to a broken pod.
- [ ] Watch `clockify_mcp_audit_failures_total`. A spike here
      during a rollout means the new pods can't reach the
      audit backend — halt the rollout.

## 3. After the rollout (post-flight)

- [ ] `clockify_policy_info` on the new server — confirm the
      effective policy matches what you expected after the
      config diff.
- [ ] `clockify_whoami` confirms the workspace binding is
      correct.
- [ ] Metric deltas over the first 15 minutes are in the
      expected ranges: tool-call latency p99, error rates,
      session counts (streamable_http).
- [ ] Audit records are flowing: pick a recent audit event and
      confirm it's queryable via the control-plane store.
- [ ] If you changed `MCP_AUDIT_DURABILITY`, update the
      on-call team's runbook reference in
      `docs/runbooks/audit-durability.md`.

## 4. Rollback criteria

Roll back if any of the following are true 15 minutes into the
rollout:

- `clockify_mcp_audit_failures_total` is rising against a
  previously-zero baseline.
- `clockify_mcp_tool_calls_total{outcome="tool_error"}` rises more
  than 10% over the pre-rollout baseline.
- The new protocol version negotiation rejects a client that
  worked on the old version (check the `msg=initialize` log line
  for `protocol_version=` and `requested_version=`).
- Any `msg=http_request status=401 reason=auth_failed` spike (your bearer
  rotation may have landed without the client config catching
  up — see `docs/runbooks/auth-failures.md`).

Rollback is a digest revert; don't try to patch the live
deployment.

## 5. Migration-specific pages

Each deployment-profile crossover has its own steps on top of
the general checklist above:

- stdio → single-tenant HTTP: read `profile-local-stdio.md`'s
  "Upgrade path" plus `profile-single-tenant-http.md`'s
  canonical config.
- single-tenant HTTP → shared-service: read
  `profile-single-tenant-http.md`'s "Upgrade path" plus
  `production-profile-shared-service.md`'s canonical config.
  Plan a separate migration window because the control-plane
  backend swaps from file to Postgres and `MCP_AUDIT_DURABILITY`
  flips to `fail_closed`.

## See also

- `docs/release-policy.md` — semver commitments and EOL
  windows.
- `docs/verify-release.md` — how to verify signed artifacts
  (checksums, SBOM, provenance) before you deploy them.
- `docs/production-readiness.md` — pre-production checklist
  covering the full transport / auth / backend matrix.
- `scripts/check-config-parity.sh` — the CI-enforced list of
  env vars; opt-outs live in `deploy/.config-parity-opt-out.txt`.
