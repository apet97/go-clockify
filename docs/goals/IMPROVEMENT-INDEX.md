# Improvement initiative — index

Entry point for the 7 plan files in `docs/goals/` that respond to the 8-point
critique of the current one-user stdio Clockify MCP. Read this first; pick
the next plan by priority; implement; verify; commit; repeat.

## Operating context for an implementer

You are implementing these plans against the `main` branch at
`/Users/15x/Downloads/WORKING/addons-me/GOCLMCP`. Read `AGENTS.md` and
`CLAUDE.md` before touching code. The 156-tool product contract is
non-negotiable; the registry must keep loading all 156. What changes is
*what we advertise on the wire* and *how the source tree is organized*.

Run from a clean `main`:

```
git status --short --branch
git log -1 --oneline --decorate
go test -count=1 ./...
make perfect
```

If any of those is dirty before you start, stop and resolve it first.

## Priority order

The user's priority list, mapped to plan files.

| # | Critique | Plan file | Effort | Depends on | Status |
|---|---|---|---|---|---|
| P1 | Shrink advertised `tools/list` context without removing internal schemas. | `advertise-output-schema-decoupling.md` | S | – | pending |
| P2 | Archive/quarantine platform-era docs harder so current one-user docs dominate. | `archive-platform-era-removal.md` | S | – | pending |
| P3 | Split `internal/tools` by clearer capability boundaries. | `split-dense-tools-files.md` | M | – | pending |
| P4 | Make `core` or compact advertised metadata the recommended AI-client default. | `narrow-default-toolset.md` | M | P1 | pending |
| P5 | Keep live-test evidence fresh because Clockify behavior is a moving target. | `nightly-api-drift-detection.md` | S | – | pending |
| P6 | Promote the in-session glossary (`CONTEXT.md`) into a tracked artifact and link it from `AGENTS.md`. | `context-md-formalization.md` | XS | – | pending |
| P7 | Reduce test rigidity so non-behavioral changes don't cascade through 100+ test files. | `test-rigidity-reduction.md` | L | P1 (as model) | pending |

S = ≤1 day. M = 2–5 days. L = >5 days. XS = <2 hours.

Critique items 1+4 map to P1; item 5 maps to P4; item 7 to P6; item 8 to P7.
The mapping is in each plan's TL;DR.

## Recommended execution waves

These plans are mostly independent — execute in waves.

- **Wave A (parallel-safe):** P1, P2, P3, P5, P6. None touches the others'
  files. Land them in any order.
- **Wave B:** P4. Builds on the descriptor flag P1 introduces.
- **Wave C:** P7. Uses P1's wire-budget test as the pattern to copy.

A reasonable single-implementer path: P6 → P2 → P1 → P5 → P3 → P4 → P7. P6
and P2 are quick wins that don't touch Go code; doing them first builds
muscle memory on the commit/verify discipline. P1 is the highest-value
single change.

## File conventions every plan follows

- **Atomic commits.** Each numbered task ends in a single commit, with the
  commit message template included in the plan. Do not batch.
- **Verify before commit.** Each task has an exact verification command and
  the expected output (or output shape). If the command fails, do not
  commit; fix or escalate.
- **No `git add -A`.** The repo ships no tracked `.gitignore`; staging
  everything risks committing build artifacts. Stage explicit paths.
- **No `--no-verify`, no `--amend` on landed commits.** Standard repo
  rules per `CLAUDE.md`.
- **`make perfect` is the wide gate.** Run it before opening a PR, and any
  time you touch the registry or schemas. CI's required-checks list is in
  `docs/branch-protection-required-checks.md`.

## What "Sonnet-grade specificity" means in these plans

Each plan is written so an implementer who is competent but not Opus-fast
can execute without judgement calls:

- Every code change cites file path and (where stable) line number or
  function name, taken from the current `main` tree.
- Every external command has its expected output described.
- Every decision the user already made is pre-recorded as "Decided" — do
  not re-litigate it.
- Every plan has an "Anti-patterns" section listing the failure modes the
  user has flagged.
- Every plan ends with a PR description template — copy, fill, ship.

## When in doubt

- Read `AGENTS.md` for the binding product contract.
- Read the plan's "Source locations to read first" section before touching
  any file.
- If the plan says "Decided: X" and your gut says Y — do X, raise Y in the
  PR description as a question. Do not redesign mid-implementation.
