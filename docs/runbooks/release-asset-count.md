# Runbook — release asset count mismatch

You are here because `scripts/check-release-assets.sh` failed inside
`release.yml` between the `Run goreleaser` step and the subsequent
staging / attestation / npm steps. The release has NOT been tagged
with an incomplete asset set — the workflow fails before any of the
downstream side effects run.

## Symptoms

Workflow log shows one of:

- `FAIL: N expected release asset(s) missing from dist` — goreleaser
  produced strictly fewer artifacts than the matrix expects.
- `FAIL: found N matching top-level files in dist, expected 28` —
  goreleaser produced too many, too few, or the wrong shape.
- `BUG: expected array has N entries, script says 28` — the script
  itself is internally inconsistent. Either `DEFAULT_UNIX_PLATFORMS`,
  `FIPS_PLATFORMS`, or `EXPECTED_COUNT` in the script has drifted
  from the others.

## Immediate mitigation

1. **Do not cherry-pick past the check.** The check exists because a
   partial release shipped to users in v0.7.0 and nobody noticed for
   several days. Running the release with `--ignore-error` or
   deleting the check step defeats the entire point.

2. **Delete the failed tag immediately** if goreleaser already pushed
   assets to the GitHub release:
   ```bash
   gh release delete "$TAG" --cleanup-tag --yes
   ```
   Assets that already uploaded from goreleaser's first attempt will
   go with the release. Re-run the release workflow on a fresh tag
   (`v1.0.N+1` or `v1.0.N-rc1` if the fix needs testing) instead of
   rerunning on the same tag — `release.yml` uses
   `mode: keep-existing` for idempotency, which will happily skip
   re-uploading the incomplete assets you just deleted.

3. **File an incident note in the release PR / commit.** Future
   contributors need to know which tags shipped cleanly and which
   didn't — goreleaser can silently skip steps on rerun.

## Root-cause checklist

The check is structural, so every failure mode maps to a small number
of upstream causes. Work through them in order:

**Missing artifacts (pass 1 fail):**

- **Missing SBOMs** → `.goreleaser.yaml` `sboms.artifacts:` filter is
  wrong for the `archives.formats` value. Binary-format archives
  match `artifacts: binary`, not `artifacts: archive`. This is the
  exact bug that caused the v0.7.0 incident; see commit `bf8df44`
  for the fix shape.
- **Missing sigstore bundles** → `signs.artifacts:` same class of
  filter drift.
- **Missing binaries** → goreleaser `builds.ignore:` list has grown
  (e.g. windows/arm64 is ignored intentionally today; a new entry
  would drop more). Check `git log -p -- .goreleaser.yaml` for
  recent edits.
- **Missing `SHA256SUMS.txt`** → `checksum.disable: true` was set,
  or `checksum.name_template` was renamed.

**Wrong count (pass 2 fail):**

- **N > 28** → goreleaser started producing an extra artifact format
  (e.g. `.sbom.json` added alongside `.spdx.json`), or a new build
  matrix entry was added without updating the script's platform
  arrays. Either fix the script to match the intended new shape
  or revert the yaml change.
- **N < 28** but pass 1 passed → impossible; if pass 1 passes, all
  28 expected files exist and pass 2 counts at least 28. If you
  see this, the script's `shopt -s nullglob` or glob patterns are
  broken — bisect the script.

**Script self-bug:**

- Someone edited `DEFAULT_UNIX_PLATFORMS` / `FIPS_PLATFORMS` /
  `EXPECTED_COUNT` without keeping them in sync. Fix both together
  in one commit with a clear message explaining the matrix change.
  Do NOT bump `EXPECTED_COUNT` alone — the script's job is to catch
  exactly that kind of one-sided edit.

## Post-incident

After the next clean release:

1. Confirm the check passed in the workflow log (grep
   `OK: all 28 expected release assets present`).
2. Cross-check the published GitHub release page: the asset count
   should match.
3. Verify via `gh release view "$TAG" --json assets | jq '.assets | length'`.

If the incident revealed a class of artifacts the script doesn't
think about yet, add them to the expected array and bump
`EXPECTED_COUNT` in the same commit. Update this runbook with the
new class and the bug it would have caught.

## Related

- `scripts/check-release-assets.sh` — the check itself
- `.goreleaser.yaml` — single source of truth for the artifact matrix
- `.github/workflows/release.yml` — where the check is called
- `docs/coverage-policy.md` — the general "ratchet, don't lower" principle
- Commit `bf8df44` — v0.7.1 fix for the sboms filter bug this check guards
