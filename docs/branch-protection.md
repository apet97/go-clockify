# Branch protection snapshot

This file is a **snapshot** of the GitHub branch-protection settings
applied to `main` in this repository. It is updated when the settings
in the GitHub UI change. The source of truth is the GitHub repository
settings, not this file — when the two diverge, GitHub wins.

The snapshot exists so an auditor or external reviewer can see what
the merge gate actually enforces without having admin access to the
repository.

Last reviewed: 2026-04-22 (post repo-visibility flip + Wave M).

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
- `Test (HTTP smoke)` — `scripts/smoke-http.sh`,
  `scripts/smoke-stdio.sh`, and `scripts/smoke-doctor-strict.sh`
  exercise HTTP, stdio, and hosted strict-doctor posture end-to-end
  against dummy credentials.
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
follow-up PR. To date this has not been used.

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
