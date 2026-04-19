# Branch protection snapshot

This file is a **snapshot** of the GitHub branch-protection settings
applied to `main` in this repository. It is updated when the settings
in the GitHub UI change. The source of truth is the GitHub repository
settings, not this file — when the two diverge, GitHub wins.

The snapshot exists so an auditor or external reviewer can see what
the merge gate actually enforces without having admin access to the
repository.

Last reviewed: 2026-04-14 (wave-e E1).

> ✅ **Applied.** As of 2026-04-14 E1, `main` has a classic
> branch-protection rule applied via
> `gh api PUT repos/apet97/go-clockify/branches/main/protection`.
> Run `bash scripts/audit-branch-protection.sh` to dump the live
> state; the snapshot table below should track it one-for-one. Two
> settings diverge from the original wave D aspiration; both are
> documented below with their reason.

## Applied protection rules on `main`

| Setting                                       | Applied state     | Note |
|-----------------------------------------------|-------------------|------|
| Require a pull request before merging         | Enabled           |      |
| Required approvals                            | 1                 |      |
| Dismiss stale pull request approvals on push  | Enabled           |      |
| Require review from Code Owners               | Enabled           |      |
| Require status checks to pass before merging  | Enabled           |      |
| Require branches to be up to date before merge| Enabled           |      |
| Require conversation resolution before merge  | Enabled           |      |
| Require signed commits                        | Enabled           |      |
| Require linear history                        | Enabled           |      |
| Require deployments to succeed                | Disabled          |      |
| Lock branch                                   | Disabled          |      |
| Restrict who can push to matching branches    | Disabled          | ‡    |
| Allow force pushes                            | Disabled          |      |
| Allow deletions                               | Disabled          |      |
| Enforce for admins                            | Disabled          | §    |

‡ **Restrict who can push to matching branches** is disabled because
this is a single-maintainer repository (see `GOVERNANCE.md`). The
other protection settings already prevent unauthorized push without
needing an actor allow-list; turning restrictions on for a single
user adds UI friction with no security benefit.

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
  `scripts/smoke-stdio.sh` exercise the HTTP and stdio transports
  end-to-end against dummy credentials.
- `Build, scan, sign` — the container image builds, Trivy passes on
  HIGH/CRITICAL, cosign signs, SBOM and SLSA attest.
- `Lychee` — external Markdown link check across the repo.

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
