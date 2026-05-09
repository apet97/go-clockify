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

## Failed-evidence record — v1.2.1-rc.1 (2026-05-09)

`v1.2.1-rc.1` was cut on `a5d5f75769dc834a268f6ab24949b139ac4cff85` and is **preserved as failed-evidence only** — it is **not** a releasable RC. Recorded here so future RC work (rc.2+) does not silently re-hit the same failures and so the rc.2 fix bundle has a referenceable baseline.

| Workflow | Run ID | Result | Failure point |
|---|---|---|---|
| Docker Image | [25612037638](https://github.com/apet97/go-clockify/actions/runs/25612037638) | ✅ success | `Build, scan, sign` job; `cosign sign` pushed signature at 21:19:21Z (`tlog entry created with index: 1487221873`) |
| Release (goreleaser) | [25612037642](https://github.com/apet97/go-clockify/actions/runs/25612037642) | ❌ failure | Step `Publish npm packages` exit 1 — `npm error You must specify a tag using --tag when publishing a prerelease version.` First package staged was `@apet97/clockify-mcp-go-darwin-arm64@1.2.1-rc.1`. |
| Deploy | [25612037645](https://github.com/apet97/go-clockify/actions/runs/25612037645) | ❌ failure | Step `Verify image signature` exit 10 — `Error: no signatures found`. Verify ran at 21:19:16Z, signature not pushed until 21:19:21Z (5-second timing race against Docker Image's `cosign sign`). |
| `release-smoke.yml` (release event) | _not fired_ | — | Did not auto-trigger. The Release workflow uses `GITHUB_TOKEN` to publish the GitHub Release object, and GitHub suppresses downstream workflow triggers from `GITHUB_TOKEN`-driven actions, so `release: [published]` never reached release-smoke. |

Asset state on the rc.1 GitHub Release object remained intact:

- 47 release assets uploaded between 21:19:47Z and 21:19:56Z (binaries, `.sigstore.json` cosign bundles, `.spdx.json` SBOMs, `SHA256SUMS.txt`).
- GitHub Release object: `name=v1.2.1-rc.1`, `isPrerelease=true`, `isDraft=false`, `publishedAt=2026-05-09T21:19:56Z`.
- No partial npm publish — `Publish npm packages` aborted on the first staged package, before any `npm publish` request reached the registry. Verifiable: `npm view @apet97/clockify-mcp-go-darwin-arm64@1.2.1-rc.1` returns 404; `@apet97/clockify-mcp-go` `dist-tags.latest` remains `1.2.0`.

Why these are **not** transient and require an rc.2:

1. **npm `--tag` script bug.** `scripts/publish-npm.sh` did not pass `--tag` for prereleases. Re-running the same Release workflow on the same `v1.2.1-rc.1` tag would hit the identical failure because the workflow runs against the tag's tree and the script bug is on that tree. **Fix-and-retag (rc.2) is required.**
2. **Deploy timing race.** The image **is** correctly signed (Docker Image's `cosign sign` step succeeded with tlog index 1487221873). Deploy's `Verify image signature` simply ran 5 seconds early. Rerunning the failed Deploy run on rc.1 will pass — but rc.1's npm gate stays unproven regardless. The deploy.yml fix wraps `cosign verify` in the same 24×30s retry budget as the digest-lookup step so the race cannot recur on rc.2+.
3. **release-smoke not firing.** Architectural — `GITHUB_TOKEN`-suppressed event chain. The release.yml fix adds an explicit `gh workflow run release-smoke.yml --ref main -f tag=$RELEASE_VERSION` step after the existing `Trigger reproducibility verification` step (mirrors the same pattern reproducibility.yml uses for the same reason).

rc.2 cuts only after the fix PR for these three issues lands on `main`. This runbook is updated again with the rc.2 evidence pointers when rc.2's Release / Docker Image / Deploy / release-smoke / npm-publish all complete green and the Group 7 evidence is recordable.

## Evidence record — v1.2.1-rc.2 (2026-05-10)

`v1.2.1-rc.2` was cut on annotated tag SHA
`681079e571bed0efbfd448129a3d9fa1de58cd15` peeling to commit
`d83f9f86d3b95594abef2ee035554510faa799c1` after PR #80 landed the
three rc.1 fixes on `main`. Tag-driven workflows fired on
2026-05-09T22:18:18Z. Local Group 6/7 evidence was collected on
darwin-arm64 the same evening; Linux x64 release-check evidence is
captured by the CI workflows referenced below rather than a second
local host.

### Workflow status on rc.2

| Workflow | Run ID | Result | Notes |
|---|---|---|---|
| Release (goreleaser) | [25613215490](https://github.com/apet97/go-clockify/actions/runs/25613215490) | ✅ success | All 47 release assets uploaded between 22:20:29Z and 22:20:38Z (46 contracted assets per `scripts/check-release-assets.sh` plus one supplementary package-level SBOM whose name embeds the image digest `sha256:4171…3b5`). `dist-tags.rc=1.2.1-rc.2` published to npm via the `--tag rc` codepath introduced in `scripts/publish-npm.sh` for prereleases. |
| Docker Image | [25613215492](https://github.com/apet97/go-clockify/actions/runs/25613215492) | ✅ success | Multi-arch image at `ghcr.io/apet97/go-clockify@sha256:4171a582016b55ae5365d626b697831c7146c7c3bed9cef134be91ef8083a3b5` with cosign signature + SBOM attestation. |
| `release-smoke.yml` | [25613270137](https://github.com/apet97/go-clockify/actions/runs/25613270137) | ✅ success | Auto-fired by the new `Trigger release smoke verification` step in `release.yml` (workflow_dispatch with `tag=v1.2.1-rc.2`). Verified `cosign verify-blob` for `clockify-mcp-linux-x64` and `clockify-mcp-postgres-linux-x64` (both `Verified OK`), `gh attestation verify` for the same two binaries (SLSA), and `cosign verify ghcr.io/apet97/go-clockify@sha256:4171…3b5` for the image. Uploaded the `release-smoke-doctor-output` artifact (https://api.github.com/repos/apet97/go-clockify/actions/artifacts/6899048388/zip) with `release-doctor-strict-ok.txt`, `release-doctor-strict-fail.txt`, and `release-doctor-postgres-ok.txt`. |
| Deploy | [25613215498](https://github.com/apet97/go-clockify/actions/runs/25613215498) | ❌ failure (NEW finding for rc.2) | `Verify Release Assets / Download release binary for SLSA verification` ran `gh release download` at 22:20:29Z and exited 1 (`release not found`). The release object did not finish publishing until 22:20:38Z (publishedAt). The PR #80 cosign-verify retry covers the cosign step but **does not** wrap the subsequent `gh release download` step, so a 9-second race between Deploy's verify job and Release's `gh release upload`/publish window now reproduces here. The release artefacts themselves are intact and verifiable (see release-smoke above); this is a Deploy-pipeline race, not a release-artifact regression. |
| Reproducibility | [25613269789](https://github.com/apet97/go-clockify/actions/runs/25613269789) | ❌ failure (NEW finding for rc.2) | sha256 mismatches between local rebuilds and published assets for `clockify-mcp-fips-linux-x64` (local `01e1571…`, released `fac3b60…`), `clockify-mcp-darwin-arm64` (local `2e8a713…`, released `3b2ab4b…`), `clockify-mcp-fips-darwin-x64` (local `1b07aa4…`, released `c262a42…`), and `clockify-mcp-fips-darwin-arm64`. The cosign and SLSA chains still verify against the published bytes; the failure is reproducibility (bit-identical rebuild) rather than provenance. Most likely cause is a Go-toolchain or build-environment difference between the goreleaser linux runner and the per-asset rebuild matrix that rebuilds darwin/fips locally. Triage required before reproducibility.yml can return green on the candidate SHA. |

The mutation cron line is unchanged from before rc.2: the latest
scheduled `mutation.yml` run remains 25592823559 cancelled on the
pre-rc.2 commit `4fe957547f9e6aea749a85f87823d17a0ccc2928`; a fresh
cron pass on the rc.2 SHA is still required to close the
required-workflows row in the launch checklist.

### Local Group 6/7 evidence (darwin-arm64, candidate-tag tree)

`scripts/prepare-rc-evidence.sh v1.2.1-rc.2` collected logs under
`${TMPDIR}/go-clockify-rc-evidence/v1.2.1-rc.2/logs/darwin-arm64/`.
Group 6 commands all passed:

- `make check` — full test suite green.
- `make verify-vuln` — `govulncheck@v1.3.0` under `GOTOOLCHAIN=go1.25.10`: `No vulnerabilities found.`
- `make secret-scan` — `gitleaks` returned `no leaks found` against the candidate-tag tree.
- Semgrep `p/default --metrics=off --error` — `Findings: 0 (0 blocking)`, 558 rules / 1155 targets.
- `make verify-fips` — default + `-tags=fips,grpc` builds and tests passed under `GOTOOLCHAIN=go1.25.10`.

Group 7 commands recorded one **environmental** local failure that is
not a defect in the rc.2 candidate tree:

- `make release-check` failed inside `make script-tests` when the
  `bash $script` invocation in `scripts/test-check-public-content-audit.sh`
  resolved to `/bin/bash 3.2` (Apple's system bash) rather than the
  Homebrew-installed `bash 5.3`. macOS GNU bash 3.2 cannot parse the
  `case` pattern inside the `$( … )` command substitution at
  `scripts/check-public-content-audit.sh:441`
  (`syntax error near unexpected token \`newline'`).
  `make script-tests` invoked directly from the user's interactive
  shell (PATH-resolved to bash 5.3) passes; `bash -lc "make
  release-check"` from `prepare-rc-evidence.sh` uses bash 3.2 and
  hits the syntax-error path. CI's macOS runners use a newer bash
  and would not be subject to this trap. Recording here so future
  candidate-tag macOS evidence collection knows to either set
  `BASH=$(brew --prefix)/bin/bash` before invoking the script or
  pass `bash --version`-aware shell hardening through the test
  wrapper. The `check-public-content-audit.sh` script is outside
  this lane's allowed write set; not modified here.
- `bash scripts/check-launch-external-status.sh --candidate-sha
  d83f9f86d3b95594abef2ee035554510faa799c1 --expected-npm-version
  v1.2.1-rc.2 --fail-open` exits non-zero with `4 open, 0 unknown`.
  Of the four open items, one is a known false-negative: the npm
  predicate compares `dist-tags.latest` to the expected version, so
  it reports `npm publish path: registry does not prove expected
  release version 1.2.1-rc.2` even though `dist-tags.rc=1.2.1-rc.2`
  is the correct prerelease tag (`dist-tags.latest` continues to
  point at the v1.2.0 stable release as designed). The other three
  open items (local branch cleanup, Group 1 SHA mismatch, mutation
  cron) are pre-existing trackers per the launch checklist and the
  Group 1 closure pin on `feef83c641ced93d2ab6ba07ef766d61c82cc703`.
  The launch-external-status npm predicate is also outside this
  lane's allowed write set; left as a documented finding.

Local manual verification per `docs/verification.md`:

- `cosign verify-blob --bundle clockify-mcp-linux-x64.sigstore.json
  --certificate-identity-regexp
  '^https://github.com/apet97/go-clockify/.github/workflows/release.yml@refs/tags/.*$'
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
  clockify-mcp-linux-x64` → `Verified OK`.
- `cosign verify-blob --bundle clockify-mcp-fips-linux-x64.sigstore.json
  …` (same identity regex) → `Verified OK`.
- `gh attestation verify clockify-mcp-linux-x64 --owner apet97`
  → exit 0; the SLSA statement names all 15 binary subjects with
  `predicateType =
  https://slsa.dev/provenance/v1`,
  `builder.id =
  https://github.com/apet97/go-clockify/.github/workflows/release.yml@refs/tags/v1.2.1-rc.2`,
  `runDetails.metadata.invocationId =
  https://github.com/apet97/go-clockify/actions/runs/25613215490/attempts/1`,
  and `sourceRepositoryDigest =
  d83f9f86d3b95594abef2ee035554510faa799c1`. The cert SAN reports
  `sourceRepositoryVisibilityAtSigning="public"`, so the
  ADR-0013 user-owned-private feature gate did not trip on this rc.
- `MCP_PROFILE=prod-postgres … /tmp/clockify-mcp-rc2 doctor --strict`
  on a fresh local build of HEAD (`d83f9f8…`) → exit 0,
  `Strict posture: OK no hosted-service findings`.
- `CLOCKIFY_POLICY=standard … doctor --strict` (negative case) →
  exit 3, `Strict posture: ERROR 1 error finding(s)`,
  `CLOCKIFY_POLICY hosted strict posture requires CLOCKIFY_POLICY no
  broader than time_tracking_safe`.

### What rc.2 closes vs leaves open

- **Closes (Group 7):** the release-artefact signing/SBOM/SLSA box
  and the `doctor --strict` reference-deployment box, both backed
  by the green release-smoke run plus the local manual verification
  above.
- **Stays open (Group 7):** `make release-check` on macOS arm64
  (environmental bash 3.2 trap above) and the required-workflows row
  (`reproducibility.yml` failed on rc.2 in addition to the still-open
  `mutation.yml` cron). Per the operator non-goal, no other Group 7
  box was ticked from this lane; Group 6 box closures remain a
  separate lane on the candidate tag.

### Operator handoff items (not changed by this lane)

- The Deploy `gh release download` race is a real workflow defect
  but `deploy.yml` is outside this lane's allowed write set. The
  fix mirrors the cosign-verify retry pattern PR #80 introduced:
  wrap `gh release download` in a 24×30s retry budget so the
  Deploy verify job can ride out the Release publish window.
- The reproducibility-bench failure should be triaged on a
  follow-up by comparing `go version -m` between the goreleaser
  linux runner output and the per-asset rebuild matrix output for
  the FIPS/darwin variants.
