# P5 · Nightly Clockify API drift detection

**TL;DR.** Add a scheduled GitHub Actions workflow that runs
`make perfect-live` against the sacrificial workspace nightly. On
failure, open/update a single rolling GitHub Issue. Addresses critique
item 6.

Estimated effort: **S** (≤1 day). No upstream dependency. Safe in
parallel with P1, P2, P3, P6.

## Problem

`AGENTS.md` lists 27+ Clockify API gotchas — fence posts against silent
drift. Today's defenses are static (gotchas list, generated OpenAPI) or
opt-in (`CLOCKIFY_RUN_LIVE_E2E`). There is no continuous detection.
Clockify can ship a change and we won't know until a user hits it.

## Goal

A scheduled workflow runs `make perfect-live` once per night against the
sacrificial workspace. On failure, the workflow opens (or updates) a
single rolling GitHub Issue labeled `drift` with the failing test name
and a redacted snippet of the upstream response. Cleanup runs always.

## Non-goals

- Generating a Clockify client from OpenAPI (big lift, small payoff).
- Running live tests on every PR (sacrificial workspace would thrash).
- Blocking releases on nightly status (initially).

## Decided

- **Schedule:** `0 7 * * *` UTC (≈ before US/EU work hours).
- **Trigger:** `schedule` + `workflow_dispatch`.
- **Runner:** `ubuntu-latest`.
- **Secrets:** repurpose existing `CLOCKIFY_API_KEY` and
  `CLOCKIFY_WORKSPACE_ID`. The CI key must point at the same sacrificial
  workspace operators use locally.
- **Prefix:** `MCP-LIVE-CI-${{ github.run_id }}-` for any created data.
- **Cleanup:** `scripts/live-clean-prefix MCP-LIVE-CI-` under `always()`.
- **Issue automation:** one rolling issue with label `drift`. Updated on
  failure, closed on next green run.
- **Retry:** one in-workflow retry before opening an issue, to absorb
  transient hiccups.
- **Not on required-checks list.** This is nightly, not a PR check. Do
  not add to `docs/branch-protection-required-checks.md`.

## Source locations to read first

| Read | Why |
|---|---|
| `Makefile` (or wherever `make perfect-live` is defined) | Confirm the target exists and what env it needs. |
| `docs/live-tests.md` | Where the new "Nightly drift" subsection lives. |
| `scripts/live-clean-prefix` | Verify the script accepts a prefix as `$1` and is idempotent. |
| `.github/workflows/` (existing files) | Match style (Go setup, module cache, secrets pattern). Pull the Go version + cache pattern from a sibling workflow. |
| Recent commit `d303659` (tag-triggered release workflow) | Reference for how the repo writes Actions YAML. |

Pre-flight:

```
grep -n 'perfect-live\|perfect_live' Makefile | head
ls .github/workflows/
```

## Implementation tasks

### Task 1 — Confirm `make perfect-live` runs from CI env

The target exists per `CLAUDE.md`. Verify it expects env, not flags:

```
grep -n 'perfect-live:' Makefile
```

Read the recipe. Note which env vars it requires (`CLOCKIFY_API_KEY`,
`CLOCKIFY_WORKSPACE_ID`, `CLOCKIFY_RUN_LIVE_E2E`, `CLOCKIFY_LIVE_PREFIX`,
`CLOCKIFY_LIVE_WORKSPACE_CONFIRM` per `AGENTS.md` / `CLAUDE.md`).

No commit — this task is read-only confirmation.

### Task 2 — Write the workflow file

**File (new):** `.github/workflows/nightly-live.yml`.

```yaml
name: nightly-live

on:
  schedule:
    - cron: "0 7 * * *"  # 07:00 UTC daily
  workflow_dispatch: {}

permissions:
  contents: read
  issues: write   # required to open/update the rolling drift issue

concurrency:
  group: nightly-live
  cancel-in-progress: false

jobs:
  perfect-live:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    env:
      CLOCKIFY_API_KEY: ${{ secrets.CLOCKIFY_API_KEY }}
      CLOCKIFY_WORKSPACE_ID: ${{ secrets.CLOCKIFY_WORKSPACE_ID }}
      CLOCKIFY_RUN_LIVE_E2E: "1"
      CLOCKIFY_LIVE_PREFIX: "MCP-LIVE-CI-${{ github.run_id }}-"
      CLOCKIFY_LIVE_WORKSPACE_CONFIRM: ${{ secrets.CLOCKIFY_WORKSPACE_ID }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run perfect-live (attempt 1)
        id: run1
        continue-on-error: true
        run: make perfect-live 2>&1 | tee /tmp/run1.log

      - name: Retry perfect-live (attempt 2, only if attempt 1 failed)
        if: steps.run1.outcome == 'failure'
        id: run2
        continue-on-error: true
        run: make perfect-live 2>&1 | tee /tmp/run2.log

      - name: Decide overall outcome
        id: decide
        run: |
          if [ "${{ steps.run1.outcome }}" = "success" ] || [ "${{ steps.run2.outcome }}" = "success" ]; then
            echo "result=pass" >> "$GITHUB_OUTPUT"
          else
            echo "result=fail" >> "$GITHUB_OUTPUT"
          fi

      - name: Cleanup live data (always)
        if: always()
        run: bash scripts/live-clean-prefix "MCP-LIVE-CI-"

      - name: Open or update drift issue on failure
        if: steps.decide.outputs.result == 'fail'
        uses: actions/github-script@v7
        env:
          RUN_URL: "${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}"
        with:
          script: |
            const fs = require('fs');
            const title = 'Nightly live drift';
            const label = 'drift';
            const runUrl = process.env.RUN_URL;
            let tail = '';
            try {
              const log = fs.readFileSync('/tmp/run2.log', 'utf8');
              tail = log.split('\n').slice(-80).join('\n');
            } catch (e) {
              try {
                const log = fs.readFileSync('/tmp/run1.log', 'utf8');
                tail = log.split('\n').slice(-80).join('\n');
              } catch (e2) {
                tail = '(no captured log)';
              }
            }
            // Redact any obvious key-looking lines.
            tail = tail.replace(/[A-Za-z0-9]{20,}/g, '[REDACTED]');
            const body = `Nightly live run failed.\n\nRun: ${runUrl}\n\nTail (last 80 lines, redacted):\n\n\`\`\`\n${tail}\n\`\`\``;
            const existing = await github.rest.issues.listForRepo({
              owner: context.repo.owner,
              repo: context.repo.repo,
              labels: label,
              state: 'open',
              per_page: 50,
            });
            const open = existing.data.find(i => i.title === title);
            if (open) {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: open.number,
                body,
              });
            } else {
              await github.rest.issues.create({
                owner: context.repo.owner,
                repo: context.repo.repo,
                title,
                body,
                labels: [label],
              });
            }

      - name: Close drift issue on success
        if: steps.decide.outputs.result == 'pass'
        uses: actions/github-script@v7
        with:
          script: |
            const title = 'Nightly live drift';
            const label = 'drift';
            const existing = await github.rest.issues.listForRepo({
              owner: context.repo.owner,
              repo: context.repo.repo,
              labels: label,
              state: 'open',
              per_page: 50,
            });
            const open = existing.data.find(i => i.title === title);
            if (open) {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: open.number,
                body: `Nightly live run passed. Auto-closing.\n\nRun: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}`,
              });
              await github.rest.issues.update({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: open.number,
                state: 'closed',
              });
            }
```

**Verify:**

```
# YAML parse check
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/nightly-live.yml'))"
```

**Commit:**

```
ci: nightly-live workflow against sacrificial workspace

Runs make perfect-live at 07:00 UTC daily. Retries once on failure.
Opens or updates a single rolling "Nightly live drift" GitHub Issue
(label: drift) on failure; auto-closes on next green run. Cleanup
always() via scripts/live-clean-prefix MCP-LIVE-CI-.
```

### Task 3 — Document the workflow

**File:** `docs/live-tests.md`. Append a new section:

```markdown
## Nightly drift detection

`.github/workflows/nightly-live.yml` runs `make perfect-live` at 07:00 UTC
every day. The job uses the same sacrificial workspace credentials as
local live tests, scoped to prefix `MCP-LIVE-CI-${run_id}-`. Cleanup
runs unconditionally after the job.

On failure, the workflow opens a single rolling GitHub Issue titled
"Nightly live drift" (label: `drift`) with the run URL and the last
80 lines of redacted output. The same issue is updated on subsequent
failures and auto-closed on the next green run.

To trigger a manual run: GitHub Actions → "nightly-live" → "Run workflow".

This workflow is **not** on the branch-protection required-checks list;
it is a continuous-detection signal, not a PR gate.
```

**File:** `README.md`, "Tests" section near the bottom. Add one line:

```markdown
A nightly GitHub Actions workflow re-runs the live suite against the
sacrificial workspace; see [docs/live-tests.md](docs/live-tests.md#nightly-drift-detection).
```

**Verify:** doc-only; `grep -n 'nightly-live' docs/live-tests.md README.md`.

**Commit:**

```
docs: document the nightly-live workflow and its rolling drift issue
```

### Task 4 — One-time manual smoke (operator action, post-merge)

The PR cannot fully verify automation without secrets running. Document
the post-merge smoke in the PR description (Task 5) so the operator runs:

```
gh workflow run nightly-live
```

…then watches the run. Expect green. Then force one controlled failure
to confirm the issue-open path:

- Edit the workflow temporarily to add a step that fails after cleanup
  (e.g. `run: exit 1` in a new step before "Decide overall outcome").
- Re-run, watch the rolling issue open.
- Revert the temporary failure.
- Watch the next run close the issue.

This is operator due-diligence, not part of the merge PR.

## Full validation (PR-time)

```
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/nightly-live.yml'))"
grep -n 'nightly-live' docs/live-tests.md README.md
make perfect    # ensure nothing else regressed
```

## Rollback

Single commit, single file: `git revert <ci-commit-sha>`. If the
workflow is misbehaving, disable temporarily with:

```yaml
on:
  workflow_dispatch: {}
```

…dropping the `schedule:` block. That keeps the file intact for fixes
without nightly invocations.

## PR description template

```
## Summary

- New workflow `.github/workflows/nightly-live.yml` runs
  `make perfect-live` at 07:00 UTC daily.
- One retry on failure absorbs transient hiccups.
- Cleanup `scripts/live-clean-prefix MCP-LIVE-CI-` runs always().
- Rolling GitHub Issue (label `drift`) opened/updated on failure;
  auto-closed on next green.
- Documented in `docs/live-tests.md` + one-liner in `README.md`.

## Secrets used

`CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID` (existing). Both must
point at the sacrificial workspace operators use locally.

## Manual smoke (operator, post-merge)

1. Trigger via `gh workflow run nightly-live`. Expect green.
2. Temporary failure-injection round-trip to confirm issue open/close
   automation.

## Test plan

- [ ] YAML parses.
- [ ] `make perfect` still green.
- [ ] Post-merge: one manual run lands green.
- [ ] Post-merge: forced-failure run opens the rolling issue.
```

## Anti-patterns

- **Do not** add the nightly workflow to required checks.
- **Do not** echo env values from the workflow. The script preserves
  redaction; do not add `set -x` or `echo "$CLOCKIFY_API_KEY"`.
- **Do not** use a separate label per failure type. One rolling issue,
  one label.
- **Do not** introduce a new CI credential. Use the existing
  sacrificial-workspace key.
- **Do not** schedule more than once per day. Cumulative live data
  shouldn't grow faster than the cleanup script can handle.
- **Do not** skip cleanup, ever. `always()` is the contract.
