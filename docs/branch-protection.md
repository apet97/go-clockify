# Branch protection snapshot

This file is a **snapshot** of the GitHub branch-protection settings
applied to `main` in this repository. It is updated when the settings
in the GitHub UI change. The source of truth is the GitHub repository
settings, not this file — when the two diverge, GitHub wins.

The snapshot exists so an auditor or external reviewer can see what
the merge gate actually enforces without having admin access to the
repository.

Last reviewed: 2026-04-14 (wave-d D5).

> ⚠️ **Gap — protection is currently unconfigured.** As of the
> 2026-04-14 audit (see `scripts/audit-branch-protection.sh`), `main`
> has neither a classic branch-protection rule
> (`GET /repos/{owner}/{repo}/branches/main/protection` returns
> `404 Branch not protected`) nor a ruleset
> (`GET /repos/{owner}/{repo}/rulesets` returns `[]`). A repo
> administrator can push or force-push to `main` without triggering
> any of the checks listed below. **The table in the next section
> describes the intended protection target, not the state GitHub is
> currently enforcing.** Enabling the target rules is tracked as a
> follow-up from wave D; they must be applied via the GitHub UI or
> via `gh api PUT /repos/{owner}/{repo}/branches/main/protection`
> before this section can claim to be a post-hoc audit record.

## Intended protection rules on `main`

The table describes the configuration this project is designed to
run under. Until the gap above is closed, treat it as a target, not
a snapshot of reality.

| Setting                                       | Target state |
|-----------------------------------------------|--------------|
| Require a pull request before merging         | Enabled      |
| Required approvals                            | 0\*          |
| Dismiss stale pull request approvals on push  | Enabled      |
| Require review from Code Owners               | Enabled      |
| Require status checks to pass before merging  | Enabled      |
| Require branches to be up to date before merge| Enabled      |
| Require conversation resolution before merge  | Enabled      |
| Require signed commits                        | Enabled      |
| Require linear history                        | Enabled      |
| Require deployments to succeed                | Disabled     |
| Lock branch                                   | Disabled     |
| Restrict who can push to matching branches    | Enabled      |
| Allow force pushes                            | Disabled     |
| Allow deletions                               | Disabled     |

\* Required approvals is `0` because this is a single-maintainer
project (see [`GOVERNANCE.md`](../GOVERNANCE.md)). When a second
maintainer joins, this should move to `1` and CODEOWNERS will start
enforcing dual review on the security-sensitive paths.

## Required status checks

The following CI jobs are the target required-checks list for a PR
to merge to `main`:

- `Lint and test` — golangci-lint + race-enabled `go test ./...` +
  fuzz smoke + build-tags audit + http smoke + config parity
  (driven by `make verify-core`).
- `Coverage` — every per-package floor cleared, global floor cleared
  (driven by `scripts/check-coverage.sh`; ratchet rule in
  `docs/coverage-policy.md`).
- `Deploy render` — `kubectl kustomize` of every overlay parses
  cleanly, `scripts/check-overlay-structure.sh` blocks any overlay
  re-introducing an `images:` block, helm template renders.
- `Docker Image / Build, scan, sign` — the image builds, Trivy passes
  on HIGH/CRITICAL, and (on tag) cosign signs and SBOM/SLSA attest.
- `Reproducibility` — the binary built from the tagged commit matches
  the binary published to the release.

`live-contract` and `release-smoke` are **not** PR-blocking. Live
contract runs nightly against the sacrificial Clockify workspace and
opens a `live-test-failure` issue on regression. Release smoke runs
on tag publish and weekly thereafter and opens a
`release-smoke-failure` issue on regression. Neither blocks a PR
merge because their inputs (upstream Clockify, sigstore TUF root) are
not under PR control — see `docs/live-tests.md` and
`.github/workflows/release-smoke.yml` for the rationale.

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
