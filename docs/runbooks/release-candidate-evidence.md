# Runbook — Release Candidate Evidence

Use this runbook immediately after Group 1 scheduled-cron evidence
closes and before reporting an official Clockify launch candidate.
It prepares the Group 6 and Group 7 evidence bundle without changing
security defaults or widening the live-test blast radius.

This runbook does **not** close Group 1, cut the candidate tag, or
declare launch readiness by itself. It starts only after
`docs/launch-candidate-checklist.md` records the required scheduled
`live-contract.yml` cron evidence on the candidate SHA.

## Evidence Targets

Group 6 needs candidate-tag security evidence:

- `make check`
- `make verify-vuln` with `govulncheck` installed
- `make secret-scan` with `gitleaks` installed
- Semgrep: `semgrep scan --config p/default --metrics=off --error
  --exclude .git --exclude .bench --exclude clockify-mcp .`
- `make verify-fips` on a FIPS-capable Go host
- `// nosemgrep` context reviewed against the relevant ADR/runbook

Group 7 needs candidate-tag release evidence:

- `make release-check` from a clean checkout on Linux x64 and macOS
  arm64
- `make verify-bench` and `make bench-baseline-check`
- Required workflows on `main` green, including the latest scheduled
  `live-contract.yml`
- `release-smoke.yml` green for `vX.Y.Z-rc.N`
- Signed binaries, SBOMs, Docker image signature, and SLSA evidence.
  `release-smoke.yml` samples the default and Postgres linux-x64
  artifacts; use `docs/verification.md` with the candidate tag and
  target filename for required variants it does not sample, including
  FIPS.
- Reference `clockify-mcp doctor --strict` and
  `clockify-mcp-postgres doctor --strict --check-backends` output
  archived with the release notes

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

7. Run the manual variant checks from `docs/verification.md` for any
   required release artifact not sampled by `release-smoke.yml`.
   At launch, that includes the FIPS variant evidence named in
   Group 7.

8. Attach or link the evidence directory, workflow URLs, and
   `workflow_run_id:` values from the launch checklist before
   checking any Group 6 or Group 7 boxes.

9. Run the evidence gate after editing the checklist:

   ```sh
   make launch-checklist-parity
   ```

## What The Script Captures

`scripts/prepare-rc-evidence.sh` writes one log per command under
`logs/<host>/`:

- Git status and exact SHA
- Group 6 local security commands
- Group 7 local release commands
- Latest GitHub run metadata for:
  - `ci.yml`
  - `build-matrix.yml`
  - `live-contract.yml`
  - `release-smoke.yml`
  - `link-check.yml`
  - `chaos.yml`
  - `mutation.yml`
  - `reproducibility.yml`
  - `bench.yml`
- Optional `release-smoke.yml` manual-dispatch output

It also fails closed when:

- The tag does not look like `vX.Y.Z-rc.N`
- The tag does not exist locally
- `HEAD` is not the candidate tag
- The worktree is dirty
- `govulncheck`, `gitleaks`, `semgrep`, or `gh` is missing
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

## Related Files

- `scripts/prepare-rc-evidence.sh`
- `scripts/check-launch-evidence-gate.sh`
- `docs/launch-candidate-checklist.md`
- `docs/release-policy.md`
- `docs/verification.md`
- `.github/workflows/release-smoke.yml`
