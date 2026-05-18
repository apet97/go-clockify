# Release checklist

Run these in order before tagging a release.

## Deterministic gates

- [ ] `make perfect` — tests (race), all drift checks, clean diff
- [ ] `make perfect-local` — adds `golangci-lint run` and the benchmark baseline check

## Live verification (sacrificial workspace only)

- [ ] Export the live env vars from `docs/live-tests.md`
- [ ] `make perfect-live` — runs `live-contract-local` then `live-clean-prefix`
- [ ] Confirm `make live-clean-prefix` reported `Leftovers: 0`

## Documentation

- [ ] Update `docs/live-tests.md` "Recorded Runs" with the new live run
- [ ] Move the `## [Unreleased]` section of `CHANGELOG.md` under the new version number

## Branch protection

`main` must require the CI checks listed in
`docs/branch-protection-required-checks.md`. Verify they are all enforced before merging
release commits.

## Tag

- [ ] `git tag vX.Y.Z` and push the tag
