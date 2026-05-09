# Runbook — Release Candidate Evidence

Use this runbook immediately after Group 1 scheduled-cron evidence
closes and before reporting a launch candidate for official-product
review.
It prepares the Group 6 and Group 7 evidence bundle without changing
security defaults or widening the live-test blast radius.

This runbook does **not** close Group 1, cut the candidate tag, or
declare launch readiness by itself. It starts only after
`docs/launch-candidate-checklist.md` records the required scheduled
`live-contract.yml` cron evidence on the candidate SHA.

## Evidence Targets

Group 6 needs candidate-tag security evidence:

- `make check`
- `make verify-vuln`, which installs and runs the pinned
  `tools/govulncheck` module under the repo Go pin
- `make secret-scan` with `gitleaks` installed
- Semgrep: `semgrep scan --config p/default --metrics=off --error
  --exclude .git --exclude .bench --exclude clockify-mcp .`
- `make verify-fips` on a FIPS-capable Go host
- `// nosemgrep` context reviewed against the relevant ADR/runbook

Group 7 needs candidate-tag release evidence:

- `make release-check` from a clean checkout on Linux x64 and macOS
  arm64
- `make verify-bench` and `make bench-baseline-check`
- `gh release view vX.Y.Z-rc.N --json assets` archived and validated
  against the 46-asset contract in `scripts/check-release-assets.sh`
- Required workflows on `main` green, including the latest scheduled
  `live-contract.yml`
- `scripts/check-launch-external-status.sh --candidate-sha <sha>
  --expected-npm-version vX.Y.Z-rc.N --fail-open` green after the
  candidate release path publishes, so scheduled workflow, repository
  state, branch-protection, stale issue/PR, and npm expected-version
  proof are checked from one read-only snapshot
- `release-smoke.yml` green for `vX.Y.Z-rc.N`
- Signed binaries, SBOMs, Docker image signature, and SLSA evidence.
  `release-smoke.yml` samples the default and Postgres linux-x64
  artifacts; use `docs/verification.md` with the candidate tag and
  target filename for required variants it does not sample, including
  FIPS.
- Reference `clockify-mcp doctor --strict` and
  `clockify-mcp-postgres doctor --strict --check-backends` output
  archived with the release notes. The `release-smoke-doctor-output`
  artifact from `release-smoke.yml` must contain
  `release-doctor-strict-ok.txt`, `release-doctor-strict-fail.txt`,
  and `release-doctor-postgres-ok.txt`.

## Dry Run The Plan

Before cutting the candidate tag, print the exact command list:

```sh
make rc-evidence-plan TAG=vX.Y.Z-rc.N
```

This is safe from any branch. It does not call Clockify, create a
tag, dispatch workflows, or write evidence logs.

## After Group 1 Closes

1. Confirm `docs/launch-candidate-checklist.md` links the two
   scheduled cron runs for Group 1 and `make launch-checklist-parity`
   still passes.

2. Cut the candidate tag using the normal release procedure. Do not
   reuse a failed candidate tag.

3. Check out the exact candidate tag on each required host:

   ```sh
   git fetch --tags origin
   git switch --detach vX.Y.Z-rc.N
   git status -sb
   ```

4. Run the evidence automation:

   ```sh
   scripts/prepare-rc-evidence.sh vX.Y.Z-rc.N
   ```

   By default it writes under:

   ```text
   ${TMPDIR:-/tmp}/go-clockify-rc-evidence/vX.Y.Z-rc.N/
   ```

   The script refuses to run if the checkout is dirty or if `HEAD`
   is not the tag. For rehearsals only, set
   `RC_EVIDENCE_ALLOW_NON_TAG=1`; do not use that override for real
   launch evidence.

5. Repeat step 4 on both required local gate hosts:

   - Linux x64
   - macOS arm64

   Group 7's `release-check` box is not complete until both host
   logs exist.

6. If the release event did not already run `release-smoke.yml`,
   dispatch it explicitly:

   ```sh
   scripts/prepare-rc-evidence.sh --trigger-release-smoke vX.Y.Z-rc.N
   ```

   Then watch the workflow through GitHub Actions or:

   ```sh
   gh run list --workflow release-smoke.yml --limit 5
   gh run view <run-id> --log-failed
   ```

   Archive or link the workflow's `release-smoke-doctor-output`
   artifact with the launch-candidate tracking issue. A green
   `release-smoke.yml` run is expected to validate and upload all
   three doctor output files named in the Group 7 evidence list above.

7. Run the manual variant checks from `docs/verification.md` for any
   required release artifact not sampled by `release-smoke.yml`.
   At launch, that includes the FIPS variant evidence named in
   Group 7.

8. Attach or link the evidence directory, workflow URLs, and
   `workflow_run_id:` values from the launch checklist before
   checking any Group 6 or Group 7 boxes.

9. After the candidate release path publishes the wrapper package,
   verify the external gates with the expected npm version. This is
   read-only, but unlike the Makefile convenience target it fails when
   any external gate remains open or unknown:

   ```sh
   scripts/check-launch-external-status.sh \
     --candidate-sha "$(git rev-parse HEAD)" \
     --expected-npm-version vX.Y.Z-rc.N \
     --fail-open
   ```

   For a non-failing handoff snapshot, use:

   ```sh
   LAUNCH_EXPECTED_NPM_VERSION=vX.Y.Z-rc.N make launch-external-status
   ```

10. Run the evidence gate after editing the checklist:

   ```sh
   make launch-checklist-parity
   ```

## What The Script Captures

`scripts/prepare-rc-evidence.sh` writes one log per command under
`logs/<host>/`:

- Git status and exact SHA
- Group 6 local security commands
- Group 7 local release commands
- GitHub Release metadata and asset names for `vX.Y.Z-rc.N`, with the
  asset names checked by `scripts/check-release-assets.sh`
- Raw latest-run GitHub metadata snapshots for:
  - `ci.yml`
  - `build-matrix.yml`
  - `codeql.yml`
  - `dependency-review.yml`
  - `semgrep.yml`
  - `live-contract.yml`
  - `release.yml`
  - `release-smoke.yml`
  - `docker-image.yml`
  - `link-check.yml`
  - `chaos.yml`
  - `mutation.yml`
  - `reproducibility.yml`
  - `bench.yml`
- A read-only `launch-external-status` log with
  `LAUNCH_EXPECTED_NPM_VERSION=<tag>` and the candidate SHA, so the
  handoff records scheduled workflow, repository-state, branch-protection,
  stale issue/PR, and npm expected-version status in one place
- Optional `release-smoke.yml` manual-dispatch output

It also fails closed when:

- The tag does not look like `vX.Y.Z-rc.N`
- The tag does not exist locally
- `HEAD` is not the candidate tag
- The worktree is dirty
- `govulncheck`, `gitleaks`, `semgrep`, `gh`, or `npm` is missing
- `make verify-fips` reports a non-FIPS-capable local toolchain

## Checklist Update Pattern

Every Group 6 or Group 7 box that changes from `[ ]` to `[x]` must
carry nearby evidence accepted by `scripts/check-launch-evidence-gate.sh`:

- A GitHub Actions run URL
- `workflow_run_id: <id>`
- `_Closed YYYY-MM-DD by <commit-or-run>`

Local logs alone are not enough for workflow-backed boxes. Do not
check a Group 7 release-artifact box until `release-smoke.yml` is
green for the candidate tag and the release assets, including
required non-sampled variants, have verified.

Workflow metadata snapshots are audit context, not proof by
themselves. Use `scripts/check-launch-external-status.sh --fail-open`
with the final candidate SHA and expected npm version as the
fail-closed validator before treating workflow-backed boxes as closed.

## Related Files

- `scripts/prepare-rc-evidence.sh`
- `scripts/check-launch-evidence-gate.sh`
- `docs/launch-candidate-checklist.md`
- `docs/release-policy.md`
- `docs/verification.md`
- `.github/workflows/release-smoke.yml`
