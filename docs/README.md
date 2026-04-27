# Documentation index

A single navigation page for the `docs/` tree, grouped by audience. If
you landed here from the repo root, the short answer is: **operators
probably want [Start Here](#start-here) or [Operator Guides](#operator-guides);
contributors probably want [Architecture & Decisions](#architecture--decisions);
security reviewers probably want [Release Trust](#release-trust).**

## Start Here

- [README.md](../README.md) — project landing page, install, connect a client.
- [production-readiness.md](production-readiness.md) — single-page operator overview.
- [deploy/](deploy/) — the five canonical deployment profiles
  (`local-stdio`, `single-tenant-http`, `shared-service`,
  `private-network-grpc`, `prod-postgres`). Apply with `clockify-mcp
  --profile=<name>` or `MCP_PROFILE=<name>`. (The legacy
  `self-hosted` shape pre-dates the profile system and is now
  served by `local-stdio` or `single-tenant-http`; see
  [`deploy/profile-self-hosted.md`](deploy/profile-self-hosted.md).)

## Operator Guides

Run-the-service documentation for the two supported profiles.

- [operators/](operators/) — cross-referenced operator guides for
  shared-service and self-hosted deployments.
- [operators/shared-service.md](operators/shared-service.md) —
  multi-tenant `streamable_http` + Postgres + OIDC.
- [operators/self-hosted.md](operators/self-hosted.md) —
  single-user / small-team deployments.
- [clients.md](clients.md) — MCP client compatibility matrix.
- [support-matrix.md](support-matrix.md) — what we support, in what
  combination, on which OS.

## Deployment Profiles

One doc per canonical shape; each profile has a matching example
env file at [`deploy/examples/`](../deploy/examples/).

- [deploy/profile-local-stdio.md](deploy/profile-local-stdio.md)
- [deploy/profile-single-tenant-http.md](deploy/profile-single-tenant-http.md)
- [deploy/production-profile-shared-service.md](deploy/production-profile-shared-service.md)
- [deploy/profile-private-network-grpc.md](deploy/profile-private-network-grpc.md)
- [deploy/profile-self-hosted.md](deploy/profile-self-hosted.md)

## Runbooks

Step-by-step incident response. Every runbook names the exact log
events, metrics, and escape hatches the current implementation
emits — no prose-only guesses.

- [runbooks/auth-failures.md](runbooks/auth-failures.md) — 401 / 403
  triage.
- [runbooks/rate-limit-saturation.md](runbooks/rate-limit-saturation.md)
  — hot tenant diagnosis.
- [runbooks/audit-durability.md](runbooks/audit-durability.md) —
  what to do when `fail_closed` aborts a call.
- [runbooks/clockify-upstream-outage.md](runbooks/clockify-upstream-outage.md)
  — Clockify API outage playbook.
- [runbooks/clockify-outage-drill.md](runbooks/clockify-outage-drill.md)
  — synthetic drill for the above.
- [runbooks/postgres-restore.md](runbooks/postgres-restore.md) —
  restore procedure for the Postgres control plane.
- [runbooks/postgres-restore-drill.md](runbooks/postgres-restore-drill.md)
  — synthetic drill for the above.
- [runbooks/image-digest-pinning.md](runbooks/image-digest-pinning.md)
  — how to pin and rotate the container image digest.
- [runbooks/release-asset-count.md](runbooks/release-asset-count.md)
  — release-asset sanity check.
- [runbooks/production-incident-drill.md](runbooks/production-incident-drill.md)
  — end-to-end incident simulation.

## Release Trust

Supply-chain verification, release process, and the smoke cadence.

- [verification.md](verification.md) — hands-on binary + image
  verification recipe.
- [verify-release.md](verify-release.md) — release-smoke reasoning.
- [release-policy.md](release-policy.md) — SemVer contract,
  deprecation windows, support timeline.
- [release/deploy-readiness-checklist.md](release/deploy-readiness-checklist.md)
  — pre-production checklist.
- [upgrade-checklist.md](upgrade-checklist.md) — per-release
  upgrade notes.
- [live-tests.md](live-tests.md) — why `release-smoke` and
  `live-contract` don't gate PR merges.

## Governance & Support

What this project promises, and how the one-of-one maintainer
population is reflected across policy surfaces.

- [../GOVERNANCE.md](../GOVERNANCE.md) — merge gate, sensitive-area
  self-review expectations, security disclosure process.
- [../SUPPORT.md](../SUPPORT.md) — where to ask questions,
  response expectations, v1.x stability guarantee.
- [../SECURITY.md](../SECURITY.md) — security disclosure contact.
- [../CONTRIBUTING.md](../CONTRIBUTING.md) — how to build, test,
  and submit a change.
- [branch-protection.md](branch-protection.md) — live snapshot of
  GitHub branch-protection rules.
- [coverage-policy.md](coverage-policy.md) — per-package coverage
  floors and the ratchet rule.

## Architecture & Decisions

Design-level reading for contributors and reviewers.

- [adr/](adr/) — architectural decision records. Start with
  [adr/README.md](adr/README.md).
- [performance.md](performance.md) — benchmark numbers and the
  workload envelope.
- [testing/soak-and-profile.md](testing/soak-and-profile.md) —
  long-running profile + soak methodology.
- [fuzz-corpus.md](fuzz-corpus.md) — fuzz coverage strategy.
- [policy/production-tool-scope.md](policy/production-tool-scope.md)
  — which tools are safe to expose in production.
- [tool-catalog.md](tool-catalog.md) / [tool-catalog.json](tool-catalog.json)
  — generated tool catalog (regenerate via `make gen-tool-catalog`).

## Forward Pointers

Capturing shape without committing implementation.

- [future/observability-correlation.md](future/observability-correlation.md)
  — end-to-end trace correlation across the MCP boundary; stub
  only, explains why Wave D deferred it.

---

If something is missing from this index, either the doc is newer
than the index or the index is stale — open an issue or PR.
