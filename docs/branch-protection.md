# Branch protection snapshot

This file is a **snapshot** of the GitHub branch-protection settings
applied to `main` in this repository. It is updated when the settings
in the GitHub UI change. The source of truth is the GitHub repository
settings, not this file — when the two diverge, GitHub wins.

The snapshot exists so an auditor or external reviewer can see what
the merge gate actually enforces without having admin access to the
repository.

Last reviewed: 2026-05-02 (Shared-service Postgres E2E promoted
to required-status check after three consecutive green runs on
`main`; see Group 2 closure in
[`launch-candidate-checklist.md`](launch-candidate-checklist.md);
admin bypass logged once for `f3897b2` — see "Bypass log" below).

> ✅ **Applied.** `main` has a classic branch-protection rule applied
> via `gh api PUT repos/apet97/go-clockify/branches/main/protection`.
> Run `bash scripts/audit-branch-protection.sh` to dump the live
> state; the snapshot table below should track it one-for-one.
> Four settings diverge from the original wave-E aspiration — all
> four are a direct consequence of the single-maintainer reality
> documented in [`GOVERNANCE.md`](../GOVERNANCE.md) and
> [`docs/adr/0016`](adr/0016-single-maintainer-governance.md).

## Applied protection rules on `main`

| Setting                                       | Applied state     | Note |
|-----------------------------------------------|-------------------|------|
| Require a pull request before merging         | Enabled           |      |
| Required approvals                            | 0                 | ¶    |
| Dismiss stale pull request approvals on push  | Enabled           |      |
| Require review from Code Owners               | Disabled          | ¶    |
| Require status checks to pass before merging  | Enabled           |      |
| Require branches to be up to date before merge| Enabled           |      |
| Require conversation resolution before merge  | Enabled           |      |
| Require signed commits                        | Disabled          | ★    |
| Require linear history                        | Enabled           |      |
| Require deployments to succeed                | Disabled          |      |
| Lock branch                                   | Disabled          |      |
| Restrict who can push to matching branches    | Disabled          | ‡    |
| Allow force pushes                            | Disabled          |      |
| Allow deletions                               | Disabled          |      |
| Enforce for admins                            | Disabled          | §    |

¶ **Required approvals = 0** and **Code-owner reviews disabled**
because this is a single-maintainer repository. GitHub hard-blocks
authors from approving their own pull requests; with one maintainer
and CODEOWNERS listing only that maintainer, any non-zero approval
requirement combined with code-owner enforcement means no PR can ever
merge. The merge gate at one-of-one scale is: **CI green + linear
history + conversation resolution**. When a second maintainer joins
(tracked in Wave L issue #26), both settings flip: approvals = 1
and code-owner reviews back on. The CODEOWNERS file still lists
paths so ownership is visible in the GitHub UI on every PR.

★ **Require signed commits = Disabled** because the maintainer's
primary workflow does not have a local GPG/SSH signing key set up
and API-authored commits on user-owned accounts are not web-flow
signed. Squash-merge commits on `main` ARE signed by GitHub's
web-flow bot when merged via the UI/API, so the history of `main`
itself is effectively fully-signed even though the PR-branch
commits that get squashed are not. Re-enable once local signing
is configured (tracked in Wave L — the "signing setup" follow-up
is worth a dedicated issue if this churns).

‡ **Restrict who can push to matching branches** is disabled because
this is a single-maintainer repository. The other protection
settings already prevent unauthorized push without needing an actor
allow-list; turning restrictions on for a single user adds UI
friction with no security benefit.

§ **Enforce for admins** is disabled so the repository administrator
can reach `main` during an incident without first disabling
protection. Standard practice for single-maintainer projects; the
trade-off is documented in the "Bypass policy" section below.

## Required status checks

These are the exact GitHub check-run names currently in the
required-checks list on `main`, as reported by
`scripts/audit-branch-protection.sh`:

- `Format` — `gofmt` parity.
- `Vet` — `go vet ./...`.
- `Lint` — `golangci-lint run ./...`.
- `Vulncheck` — `govulncheck ./...`.
- `Build` — cross-compile for every release target, plus the
  build-tag matrix (default/otel/grpc/pprof/fips) via
  `scripts/check-build-tags.sh`.
- `Test` — race-enabled `go test ./...`.
- `Coverage` — every per-package floor cleared, global floor cleared
  (driven by `scripts/check-coverage.sh`; ratchet rule in
  `docs/coverage-policy.md`).
- `Fuzz` — the three `Fuzz*` targets run with a count-based budget
  (`-fuzztime=300000x`) to sidestep the wall-clock race documented
  in commit `a67ee39`. Previously named "Fuzz (30s per target)"
  until the rename in wave E.
- `Deploy render (k8s + helm)` — `kubectl kustomize` of every
  overlay parses cleanly, `scripts/check-overlay-structure.sh`
  blocks any overlay re-introducing an `images:` block, helm
  template renders.
- `Test (HTTP smoke)` — `scripts/smoke-http.sh` and
  `scripts/smoke-stdio.sh` exercise HTTP and stdio transports
  end-to-end against dummy credentials.
- `Doctor strict smoke` — `scripts/smoke-doctor-strict.sh` verifies
  hosted strict-posture behavior, including the broad-policy
  negative exit path. Runs as a standalone CI job so a strict-doctor
  regression does not hide behind an HTTP/stdio smoke failure.
- `Doctor Postgres backend` — builds `clockify-mcp` with
  `-tags=postgres` against an ephemeral `postgres:16-alpine` service
  and runs `doctor --strict --check-backends`. Exit 0 proves the
  embedded migrations apply, `audit_events.phase` exists, and the
  audit health round-trip through `DoctorCheck(ctx)` succeeds.
- `Shared-service Postgres E2E` — drives `mcp.ServeStreamableHTTP`
  in-process against an ephemeral `postgres:16-alpine` service
  with two distinct `forward_auth` principals and asserts tenant
  isolation in `audit_events` + `sessions`, the cross-tenant
  negative, per-tenant policy enforcement
  (`time_tracking_safe` blocks `clockify_create_project`), and
  read-only-tools-emit-no-audit. Closes Group 2 of
  `docs/launch-candidate-checklist.md`. Promoted after three
  consecutive green runs on `main`
  (25240007056, 25240085916, 25240163213 on 2026-05-02).
- `Build, scan, sign` — the container image builds, Trivy passes on
  HIGH/CRITICAL, cosign signs, SBOM and SLSA attest.
- `Lychee` — external Markdown link check across the repo.
- `Build -tags=grpc` — Wave K compile-only cell for the gRPC
  transport.
- `Build -tags=fips` — Wave K compile-only cell for the FIPS build.
- `Build -tags=otel` — Wave K compile-only cell for the OTel
  exporter wiring.
- `Build -tags=pprof` — Wave K compile-only cell for the pprof
  handlers.
- `Build -tags=grpc,otel` — Wave K combinatorial covering the
  gRPC + OTel interceptor adapter path.
- `Build -tags=fips,grpc` — Wave K combinatorial covering the FIPS
  + gRPC dependency chain (the risky one, because gRPC transitively
  pulls in crypto choices that must be FIPS-capable).

`Reproducibility`, `live-contract`, and `release-smoke` are **not**
PR-blocking. Reproducibility triggers on release events, not pull
requests, so a required PR check would never report and would
silently deadlock every merge. Live contract runs nightly against
the sacrificial Clockify workspace and opens a `live-test-failure`
issue on regression. Release smoke runs on tag publish and weekly
thereafter and opens a `release-smoke-failure` issue on regression.
Neither blocks a PR merge because their inputs (upstream Clockify,
sigstore TUF root) are not under PR control — see
`docs/live-tests.md` and `.github/workflows/release-smoke.yml` for
the rationale.

## Bypass policy

Branch protection bypass is **not** granted to any user, app, or
team. The repository administrator can technically override branch
protection in an emergency (this is a GitHub mechanism the project
does not use); when invoked, the override must be documented in a PR
or issue with the reason and the change must be reviewed in a
follow-up PR.

The administrator bypass mechanism has been invoked one time as of
2026-05-02. Each invocation is recorded below with commit, reason,
risk, mitigation, and an explicit non-claim that the bypass does not
substitute for any launch-candidate evidence gate.

### Bypass log

| Date (UTC) | Commit | Branch event | Bypass | Reason |
|---|---|---|---|---|
| 2026-05-02 | `f3897b2563e03ba8b924a383bdfcbb75214ac88e` | direct push to `main` (single docs commit, fast-forward over `adce316`) | PR-required gate + 19 expected required-status-check pre-merge contexts | docs-only handoff continuation commit after PR #51, intended to publish a workstation-private continuation packet to `origin/main` so the next agent picks up from the same state |

**Risk for `f3897b2`.** The push bypassed two pre-merge mechanisms:
the "require pull request before merging" rule and the "expect all 19
required status checks" merge-time gate. No CI run was *prevented*
by the bypass — every workflow file with an `on: push` trigger fired
and reported back. What was bypassed is the procedural enforcement
that those checks must finish *before* the merge happens (the merge
in this case being a direct fast-forward push).

**Mitigation.**

- The commit body contained a `Verified:` line documenting the local
  pre-push gates: `make check; make doc-parity; make
  config-doc-parity; make catalog-drift; make bench-baseline-check;
  git diff --check; bash scripts/test-check-launch-evidence-gate.sh`.
- All 19 PR-required checks plus 9 additional non-required checks
  (28 total) executed on the push and reported `success`. List
  re-derived from
  `gh api repos/apet97/go-clockify/commits/<sha>/check-runs`:
  `Actionlint`, `Build`, `Build -tags=fips`, `Build -tags=fips,grpc`,
  `Build -tags=grpc`, `Build -tags=grpc,otel`, `Build -tags=otel`,
  `Build -tags=pprof`, `Build, scan, sign`, `Config doc parity`,
  `Coverage`, `Deploy render (k8s + helm)`, `Doctor Postgres
  backend`, `Doctor strict smoke`, `Format`, `Fuzz`, `gRPC auth
  smoke`, `Lint`, `Lychee`, `Repo hygiene`, `Secret scan (gitleaks)`,
  `Shared-service Postgres E2E`, `Shellcheck`, `Test`, `Test (gRPC
  tag)`, `Test (HTTP smoke)`, `Vet`, `Vulncheck`.
- This follow-up PR documents the bypass and lets the same checks
  run again under the PR-required protocol on the documentation
  reconciliation commit, restoring the audit trail.

**Non-claim.** Logging this bypass does **not** count as
launch-candidate evidence and does **not** close any group of
[`launch-candidate-checklist.md`](launch-candidate-checklist.md).
Group 1 still requires two consecutive scheduled-cron greens of
`live-contract.yml` on the candidate SHA. Group 6 still requires
the candidate-tag security walk-through. Group 7 still requires
release/sigstore/SLSA evidence on the candidate tag. The bypass
log is a governance audit artefact; it is not a substitute for
any of those gates.

## How to audit

Run:

```sh
bash scripts/audit-branch-protection.sh
```

This hits `gh api repos/{owner}/{repo}/branches/main/protection` and
projects the fields the snapshot table covers. Any divergence from
the target table above should be either reconciled (update the table
in a PR labelled `governance-snapshot`) or fixed (re-apply the target
rules via the GitHub UI). The script exits non-zero if the branch is
unprotected, which is itself the signal to reconcile.

## Target state for paid / public hosted launch

The current rules are the honest snapshot of a single-maintainer
project (see ADR-0016). Before `clockify-mcp-go` is offered as a paid
hosted service — or before the repo admits a second maintainer — the
2026-04-25 security audit (finding L2) flagged the following as the
minimum required tightening:

| Setting | Current | Target |
|---------|---------|--------|
| Required approvals | 0 | **1** (review from a non-author) |
| Require review from CODEOWNERS | Disabled | **Enabled** |
| Require signed commits | Disabled | **Enabled** (web-flow squash counts) |
| Include administrators | Disabled | **Enabled** (no break-glass around the bot) |
| Restrict who can dismiss PR reviews | Disabled | **Enabled** (audit trail intact) |

The tightening is gated on adding a second maintainer: enabling
"required approvals = 1" with a single maintainer blocks every PR.
`GOVERNANCE.md` carries the request — contact the maintainer via
the Security Advisory process or a public issue tagged
`governance-volunteer`.

Until then this file documents the gap honestly so downstream
consumers can factor it into their trust model.

## How to update this file

When a setting in the GitHub UI changes:

1. Make the UI change.
2. Update the table above in the same PR (or in a follow-up labelled
   `governance-snapshot`).
3. Bump the "Last reviewed" date.
4. Re-run `scripts/audit-branch-protection.sh` to confirm the live
   state matches.

The CODEOWNERS file routes changes to this document through
`@apet97`.
