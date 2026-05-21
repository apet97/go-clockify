# P2 · Remove platform-era docs from main; preserve on archive branch

> **Historical artifact. Not current one-user MCP product documentation.**
> This implementation plan discusses archived platform-era material so it can
> be removed from current setup paths; do not use it as product guidance.

**TL;DR.** Remove the 9 platform-era docs (and clean up the 7 redirect
breadcrumbs that point to them) from `main`. Push them to a long-lived
`archive/platform-era` branch. Addresses critique item 2.

Estimated effort: **S** (≤1 day). No upstream dependency. Safe in parallel
with P1, P3, P5, P6.

## Problem

`docs/archive/platform-era/` plus two sibling one-shots under `docs/archive/`
describe tenants, OIDC, HTTP/gRPC, Postgres, and policy — concepts the
current one-user product explicitly forbids per `AGENTS.md` safety rules.
The current `docs/` tree also carries 7 redirect-stub files that link into
the archive (e.g. `docs/production-readiness.md` points at
`archive/platform-era/production-readiness.md`). Both are confusion
surfaces for new contributors and agents that grep.

## Goal

A clean `docs/` tree on `main` with no platform-era files and no redirect
stubs pointing into a removed directory. History preserved on the branch
`archive/platform-era`.

## Non-goals

- Rewriting any legacy doc.
- Touching current docs that don't reference the archive.
- Deleting the `docs/archive/README.md` breadcrumb — that file stays on
  `main` to point at the archive branch.

## Decided

- **Files removed from `main`** (9 platform-era + 2 archive-root one-shots,
  11 total):

  ```
  docs/archive/platform-era/launch-candidate-checklist.md
  docs/archive/platform-era/launch-readiness-review-may-8.md
  docs/archive/platform-era/official-clockify-mcp-gap-analysis.md
  docs/archive/platform-era/production-readiness.md
  docs/archive/platform-era/README.md
  docs/archive/platform-era/release/public-hosted-launch-checklist.md
  docs/archive/platform-era/support-matrix.md
  docs/archive/fwbranch-review-handoff.md
  docs/archive/live-campaign-next-prs-53-59.md
  ```

  (The `docs/archive/README.md` stays. The directory `docs/archive/platform-era/`
  is removed entirely.)

- **Redirect stubs to delete from `docs/`** (7 files; each is currently a
  one-paragraph "moved to archive" stub):

  ```
  docs/production-readiness.md
  docs/launch-readiness-review-may-8.md
  docs/official-clockify-mcp-gap-analysis.md
  docs/support-matrix.md
  docs/launch-candidate-checklist.md
  docs/release/public-hosted-launch-checklist.md
  ```

  (Confirm each is a stub-only file before deletion — see Task 2.)

- **Branch name:** `archive/platform-era`. Pushed to origin. The
  pre-removal `main` HEAD is the branch HEAD.
- **`docs/archive/README.md`** is rewritten in this work to point at the
  branch. It is the only artifact remaining in `docs/archive/` on `main`.
- **AGENTS.md and README.md** lose their inbound links to the removed paths.

## Source locations to read first

| Read | Why |
|---|---|
| `docs/archive/README.md` | Current breadcrumb. Will be rewritten in Task 4. |
| `README.md` lines 14–18 | The README's "Current docs" paragraph mentions the archive. |
| `AGENTS.md` lines 44–47 | The "Historical docs explain prior decisions" paragraph. |
| The 7 redirect stubs above | Confirm each is stub-only (single short paragraph + link) before Task 2. |

Run before starting:

```
git status --short --branch
git log -1 --oneline
git grep -n 'archive/platform-era' -- ':!docs/goals/' ':!docs/archive/README.md'
```

The last grep is the canonical "stale references on main" check; you will
re-run it after the work as the validation gate.

## Implementation tasks

### Task 1 — Snapshot history onto a long-lived branch

```
git fetch origin
git branch archive/platform-era origin/main
git push origin archive/platform-era
git switch main
```

**Verify:**

```
git ls-remote --heads origin archive/platform-era
git show archive/platform-era:docs/archive/platform-era/production-readiness.md | head -5
```

Expect the branch to exist and the file's first lines to render. The branch
is the canonical citation point from now on.

**Commit:** none for this task — it's a branch-push only.

### Task 2 — Confirm stubs are stubs

For each of the 7 redirect files in `docs/`, run:

```
wc -l docs/production-readiness.md docs/launch-readiness-review-may-8.md \
      docs/official-clockify-mcp-gap-analysis.md docs/support-matrix.md \
      docs/launch-candidate-checklist.md \
      docs/release/public-hosted-launch-checklist.md
```

Each should be ≤ 25 lines and consist only of a banner pointing at the
archive copy. If any file is longer or carries unique content not in the
archive copy, **stop**. That file isn't a stub; it's a kept doc that
happens to link into the archive. In that case:

- Delete only the link line, not the file.
- Note this case in the PR description.
- Proceed with the rest.

(Expected: all 7 are short stubs. If they are, this task confirms it; no
commit.)

### Task 3 — Remove the archive files and redirect stubs

In a single commit:

```
git rm -r docs/archive/platform-era/
git rm docs/archive/fwbranch-review-handoff.md
git rm docs/archive/live-campaign-next-prs-53-59.md
git rm docs/production-readiness.md
git rm docs/launch-readiness-review-may-8.md
git rm docs/official-clockify-mcp-gap-analysis.md
git rm docs/support-matrix.md
git rm docs/launch-candidate-checklist.md
git rm docs/release/public-hosted-launch-checklist.md
```

Note: `docs/release/` may now be empty or near-empty. Check with `ls
docs/release/`. If it has other files, leave the directory. If it is empty,
let `git rm` clean it up (git removes empty directories automatically — no
extra command needed).

**Verify:**

```
git status --short
git diff --cached --stat
ls docs/archive/        # expect: only README.md
git grep -n 'archive/platform-era' -- ':!docs/goals/' ':!docs/archive/README.md'
```

The last grep should return matches **only** in `README.md` and `AGENTS.md`
— those get fixed in Task 4. If it returns matches in other docs, list
them and either delete the link or update it before continuing.

**Commit:**

```
chore(docs): remove platform-era archive and redirect stubs from main

Files preserved on branch archive/platform-era (pushed in a prior step
of this PR). docs/archive/README.md becomes the breadcrumb to the branch
in the next commit.
```

### Task 4 — Rewrite the breadcrumb and update inbound links

Rewrite `docs/archive/README.md` to a short pointer:

```markdown
# Archive

Pre-one-user product docs (the platform-era tenants/OIDC/HTTP/gRPC/Postgres
designs and one-shot operational records) are preserved on the long-lived
branch `archive/platform-era`. They are not setup instructions for the
current product.

To read a historical file:

    git show archive/platform-era:docs/archive/platform-era/<filename>

Or check out the branch locally for browsing:

    git fetch origin archive/platform-era
    git switch archive/platform-era

Current product docs live on `main` and start from `README.md` and
`AGENTS.md`.
```

In `README.md`, find the paragraph (around lines 14–18 on the pre-removal
tree) that mentions the archive. Replace with:

```markdown
## Current docs

This README is the setup entry point. The complete tracked doc set lives
under `docs/`. Pre-one-user platform-era designs are preserved on branch
`archive/platform-era`; see `docs/archive/README.md`.
```

In `AGENTS.md`, find the paragraph at lines 44–47 ("Historical docs explain
prior decisions; current work starts from the files above plus the code.
Do not route users to archived or bannered platform-era docs as setup
instructions.") Replace with:

```markdown
Historical docs explain prior decisions and are preserved on branch
`archive/platform-era`. Current work starts from the files above plus the
code. Do not route users to the archive branch as setup instructions.
```

**Verify:**

```
git grep -n 'archive/platform-era' -- ':!docs/goals/' ':!docs/archive/README.md'
```

Expect: zero matches in any file other than `docs/archive/README.md`
(which intentionally mentions the branch).

```
git grep -n 'docs/archive/platform-era' -- AGENTS.md README.md
```

Expect: zero matches.

**Commit:**

```
docs(archive): rewrite breadcrumb and update README/AGENTS inbound links

docs/archive/README.md now points readers at the archive/platform-era
branch instead of the removed directory. README.md and AGENTS.md drop
the bannered-platform-era language and replace it with a single sentence
about the branch.
```

### Task 5 — Confirm the build is unaffected

The archive is doc-only. Tests should not depend on these files, but verify:

```
make perfect
```

If anything fails, the failure is unrelated to this PR (or you accidentally
deleted a kept doc). Investigate before opening the PR.

## Full validation

```
git grep -n 'docs/archive/platform-era' -- ':!docs/archive/README.md' ':!docs/goals/'
# expect: zero matches

git show archive/platform-era:docs/archive/platform-era/production-readiness.md | head -3
# expect: file header renders

ls docs/archive/
# expect: README.md (and nothing else)

make perfect
# expect: green
```

## Rollback

If you discover you removed a file that should have stayed (e.g. a
"redirect stub" turned out to have unique content), restore from the
branch:

```
git checkout archive/platform-era -- docs/<file>
```

Then commit the restoration and update the PR description. Do **not** roll
back the branch push; the branch is the safety net.

## PR description template

```
## Summary

- Push the pre-removal `main` HEAD to a long-lived branch
  `archive/platform-era`.
- Remove 9 platform-era docs and 2 archive-root one-shots from `main`.
- Remove 7 redirect-stub files in `docs/` that pointed into the archive.
- Rewrite `docs/archive/README.md` to a short pointer at the branch.
- Update `README.md` and `AGENTS.md` to remove inbound links.

## Why

These docs predate the one-user stdio product. Banners required reading
to ignore; grep ignored the banners. Removing from `main` cleans the
working tree without losing history.

## How to cite a historical doc

`git show archive/platform-era:<path>` or check out the branch.

## Test plan

- [ ] `make perfect`
- [ ] `git grep 'docs/archive/platform-era' -- ':!docs/archive/README.md'`
      returns no matches.
- [ ] `git show archive/platform-era:docs/archive/platform-era/production-readiness.md`
      renders the file.
```

## Anti-patterns

- **Do not** force-push or delete the `archive/platform-era` branch. It is
  the safety net.
- **Do not** delete `docs/archive/README.md` — it is the on-`main`
  breadcrumb.
- **Do not** batch this with another PR. Doc removal is loud and easy to
  review in isolation.
- **Do not** `git rm -r docs/archive` — that would remove the breadcrumb
  too. Use the explicit file list in Task 3.
- **Do not** preserve the redirect stubs "just in case". They have no
  unique content; the archive branch is the source.
