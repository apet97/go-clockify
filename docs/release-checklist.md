# Release checklist

Run these in order before tagging a release.

## Deterministic gates

- [ ] `make perfect` - tests (race) and all drift checks (catalog, API parity,
      OpenAPI, raw allowlist, self-inspection, module tidy), plus a clean diff
- [ ] `make perfect-local` - adds `golangci-lint run` and the benchmark
      baseline check
- [ ] `make mod-tidy-drift` produces no diff - `go mod tidy` would not change
      `go.mod` or `go.sum` (this also runs inside `make perfect`)

## Live verification (sacrificial workspace only)

- [ ] Export the live env vars from `docs/live-tests.md`
- [ ] `make perfect-live` - runs `live-contract-local` then `live-clean-prefix`
- [ ] Confirm `make live-clean-prefix` reported `Leftovers: 0` after its
      post-delete rescan

## Documentation

- [ ] Update `docs/live-tests.md` "Recorded Runs" with the new live run,
      including the tested commit SHA, then run `make sync-selfinspect-assets`
      so `internal/tools/selfinspect_assets/live-tests.md` stays in sync
- [ ] Move the `## [Unreleased]` section of `CHANGELOG.md` under the new
      version number

## GitHub Actions verification

The release commit must have a visible, green Actions run before it is tagged.
This repo reports CI through GitHub Actions check-runs, not the legacy combined
commit-status API.

- [ ] `gh run list --commit <release-commit>` lists a workflow run for the
      exact commit being tagged
- [ ] `gh api repos/apet97/go-clockify/commits/<release-commit>/check-runs
      --jq '[.check_runs[] | {name, conclusion}]'` shows every required check
      from `docs/branch-protection-required-checks.md` with
      `conclusion: "success"`
- [ ] Ignore the legacy combined-status API
      (`gh api repos/apet97/go-clockify/commits/<release-commit>/status`): with
      no legacy commit statuses configured it reports an empty `pending` state
      that does not reflect the real check-run results
- [ ] **Do not tag a commit that has no visible workflow run.** A missing run
      means CI never executed for that exact commit - push it, wait for CI to
      finish green, and only then tag.

## Branch protection

`main` must require every CI check listed in
`docs/branch-protection-required-checks.md`, including `Module tidy drift`.
Verify they are all enforced before merging release commits.

## Tag

- [ ] `git tag vX.Y.Z` and push the tag
