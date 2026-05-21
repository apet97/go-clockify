# P6 · Promote CONTEXT.md to a tracked, linked glossary

**TL;DR.** The repo has rich docs but no compact domain-language map for
agents. An untracked `CONTEXT.md` was drafted in-session; stage it,
expand it modestly, link from `AGENTS.md` and `README.md`, and set a
keep-current discipline. Addresses critique item 7.

Estimated effort: **XS** (<2 hours). No upstream dependency. Safe in
parallel with everything.

## Problem

Multiple plans and audits during the conversation introduced terms like
"Model-visible tool surface" and "Advertised output schema" — concepts
that conflict-resolve confusion in code review and PR descriptions. The
repo has lots of docs but no single glossary. A draft `CONTEXT.md` was
created during the conversation and remains untracked in the working
tree.

## Goal

`CONTEXT.md` is tracked, linked from `AGENTS.md` and `README.md`, holds a
compact glossary (<200 lines) of repo-specific domain terms, and is the
*first* doc an agent reads after `AGENTS.md`. It does not duplicate
content from other docs; it disambiguates the language they use.

## Non-goals

- Replacing any current doc.
- Adding architecture or process content. CONTEXT.md is glossary +
  relationships, not a design doc.
- Making CONTEXT.md a coverage doc for every term in the codebase. The
  bar is "did this term confuse someone in the last 90 days?".

## Decided

- **Location:** repo root, `CONTEXT.md`. The file already exists
  untracked at this path.
- **Scope:** ≤ 200 lines. Entries follow the existing draft's shape:
  term, one-line definition, "Avoid:" list of confusable alternatives,
  optional cross-reference link.
- **Discipline:** add a one-line item to the AGENTS.md "Start Here"
  list, and a one-line breadcrumb in `README.md`'s "Current docs"
  section. No CI check for now — drift is acceptable at this size.
- **Initial entries:** keep the two terms already drafted
  (Model-visible tool surface, Advertised output schema). Add three
  more that emerged across the audit cycle: "Registry vs. advertised
  surface", "Live evidence (protocol/recovery vs happy-path)",
  "Workspace prefix (live data scoping)".

## Source locations to read first

| Read | Why |
|---|---|
| `CONTEXT.md` (untracked at repo root) | The current draft — keep its shape and tone. |
| `AGENTS.md` "Start Here" list (lines 27–43) | Where the new entry slots in. |
| `README.md` "Current docs" section (~ lines 14–18) | Where the breadcrumb lives. |

## Implementation tasks

### Task 1 — Confirm draft content and extend modestly

Read the existing `CONTEXT.md`. It currently defines two terms. Add three
more — keep the same template:

```markdown
**Registry vs. advertised surface**:
The full set of 156 tools that `Service.FullAccessRegistry()` loads at
startup ("registry") versus the subset visible in `tools/list` to a
given client ("advertised surface"). Tools not advertised remain
dispatch-callable by name.
_Avoid_: "tool count", "available tools" (ambiguous between the two).

**Live evidence (protocol/recovery vs happy-path)**:
Two distinct columns in `docs/goals/oneuser-tool-coverage.md`.
Protocol/recovery evidence proves a tool returns a useful `ok:false`
envelope on a known bad input. Happy-path evidence requires `ok:true`
against a real entity. They are not interchangeable.
_Avoid_: "live coverage" alone — always qualify which column.

**Workspace prefix**:
The token (e.g. `MCP-LIVE-RECON-`) attached to every entity name a live
test creates, used by `scripts/live-clean-prefix` to delete only that
batch of test data on cleanup. The sacrificial workspace is shared, so
prefixes are the isolation boundary.
_Avoid_: "test data tag", "label" — those mean other Clockify concepts.
```

Place these after the existing two entries. Keep the existing
"Relationships", "Example dialogue", and "Flagged ambiguities" sections.
If the relationships section is now too small to justify, that's fine —
do not invent relationships.

**Verify:** read the file end-to-end; confirm ≤ 200 lines.

```
wc -l CONTEXT.md
```

### Task 2 — Stage and commit CONTEXT.md

```
git add CONTEXT.md
git status --short
```

Expect `A  CONTEXT.md` only. Nothing else should be staged.

**Commit:**

```
docs: introduce CONTEXT.md — compact domain-language glossary

Five entries to start: model-visible tool surface, advertised output
schema, registry vs advertised surface, live evidence columns,
workspace prefix. The first doc to read after AGENTS.md for agents
needing to disambiguate repo-specific terms.
```

### Task 3 — Link from AGENTS.md

**File:** `AGENTS.md`. In the "Start Here" numbered list (around lines
27–43), insert as a new item between current items 1 and 2 (after
`README.md`, before `docs/agent-cookbook.md`):

```markdown
2. `CONTEXT.md` - compact glossary of repo-specific domain terms.
```

Renumber the rest of the list. Keep the surrounding paragraph intact.

**Verify:**

```
grep -n '^[0-9]\+\. ' AGENTS.md | head -15
```

Expect a clean numbered sequence with `CONTEXT.md` as item 2.

**Commit:**

```
docs(agents): link CONTEXT.md from Start Here
```

### Task 4 — Link from README.md

**File:** `README.md`. In the "Current docs" section (around lines
14–18), add a bullet alongside the existing setup/cookbook/catalog links:

```markdown
- Compact domain-language glossary: [CONTEXT.md](CONTEXT.md)
```

Place it as the second bullet (right after the README link or right
before the cookbook).

**Verify:**

```
grep -n 'CONTEXT.md' README.md
```

**Commit:**

```
docs(readme): link CONTEXT.md from current docs list
```

### Task 5 — Add a maintenance note to CLAUDE.md (optional)

`CLAUDE.md` is project-local context, not the binding contract. If you
want a soft nudge for future sessions, append a one-line bullet to its
"Session start" guidance:

```markdown
Skim `CONTEXT.md` (≤2 min) whenever an agent reaches for a term you
suspect is repo-specific.
```

This is *optional* — drop the task if the maintainer prefers CLAUDE.md
to stay focused on workstation state.

**Commit (if done):**

```
docs(claude): point future sessions at CONTEXT.md when terms ambiguous
```

## Full validation

```
git grep -n 'CONTEXT.md' AGENTS.md README.md
wc -l CONTEXT.md
make perfect    # sanity, though doc-only changes shouldn't affect anything
```

## Rollback

Each task is a single commit. Revert in reverse order if needed. Worst
case: `git rm CONTEXT.md` to remove the file entirely; the AGENTS.md /
README.md link updates revert with their own commits.

## PR description template

```
## Summary

- Stage CONTEXT.md (previously untracked) with 5 glossary entries.
- Link from AGENTS.md "Start Here" and README.md "Current docs".

## Why

The repo has lots of docs but no single compact glossary. Several recent
audit conversations introduced new terms (model-visible tool surface,
advertised output schema, registry vs advertised surface) that benefit
from a one-sentence definition with "Avoid:" pointers. This doc is the
first 2-minute read for any agent that suspects a repo-specific term.

## Test plan

- [ ] `git grep -n 'CONTEXT.md' AGENTS.md README.md` finds the links.
- [ ] `wc -l CONTEXT.md` ≤ 200.
- [ ] `make perfect` green.
```

## Anti-patterns

- **Do not** expand CONTEXT.md beyond 200 lines. If it grows past that,
  split it or move overflow into a topical doc.
- **Do not** duplicate content from `AGENTS.md`, the cookbook, or
  protocol-notes. The glossary defines terms; other docs use them.
- **Do not** add architecture or process content. This is a vocabulary
  doc.
- **Do not** introduce CI enforcement for glossary coverage. Discipline,
  not gates.
