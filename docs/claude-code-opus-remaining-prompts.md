# Claude Code Opus 4.7 Prompts — Remaining Launch Work

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


> **Historical / operator handoff material.** This file is the prompt
> packet that was used to launch the 2026-05-09 multi-lane Claude Code
> Opus 4.7 work — lanes 1, 4, and 5 ran from these prompts and landed
> their outputs in PRs **#75** (mutation cron), **#77** (repo-state
> cleanup, including a follow-up recovery commit and two reviewer
> follow-ups), and **#76** (hosted/legal/product approval audit). The
> "Current State" section below reflects the snapshot the operator
> handed lanes at launch time; some of its claims have been
> **superseded** by the lane PR outputs (notably the repo-state and
> branch-protection rows in PR #77). This document is **not launch
> evidence** and does **not close any gates**. Merge only if it is
> clearly framed as historical and does not conflict with newer lane
> outputs; otherwise close it as superseded.

Prepared 2026-05-09 from `main` at
`a07443bb260974f263ae8f39349544b72d351aab`
(`docs(launch): archive scheduled live evidence`).

Use one prompt per Claude Code Opus 4.7 session. Run each session in a
separate branch or worktree. The prompts below intentionally split the
remaining launch work by ownership boundary so parallel sessions do not
rewrite the same files.

These prompts are not launch evidence by themselves. They are operator
handoffs for collecting or documenting the evidence that the launch
checklist already requires.

## Current State

- Group 1 scheduled live-contract evidence is archived on
  `feef83c641ced93d2ab6ba07ef766d61c82cc703`.
- Scheduled `live-contract.yml` runs 25608259477 and 25607242862 are
  green on that SHA and include read-only/schema-diff, mutating,
  MCP-path safety, and audit-phase markers.
- The temporary high-frequency live-contract cron was removed in
  `a07443bb260974f263ae8f39349544b72d351aab`.
- Push workflows on `a07443b` are green: CI 25609129982, CodeQL
  25609129989, Dependency Review 25609129978, Semgrep 25609129983,
  Build matrix 25609129988, Docker Image 25609129976, and Link check
  25609129994.
- Current launch blockers are not ordinary local test failures:
  candidate-tag Group 6 evidence, Group 7 release/sigstore/SLSA/npm
  evidence, mutation cron evidence, repository-state cleanup,
  branch-protection proof, issue #28, hosted/commercial approval
  gates, and legal/product approval remain open.

The default `scripts/check-launch-external-status.sh` uses current
`HEAD` as the candidate SHA. Because the temporary cron had to be
removed after evidence capture, current `HEAD` is `a07443b` while the
archived Group 1 scheduled evidence is on `feef83c`. When a session is
auditing Group 1 specifically, it must run:

```sh
scripts/check-launch-external-status.sh \
  --candidate-sha feef83c641ced93d2ab6ba07ef766d61c82cc703
```

When a session is auditing the current branch tip, it should use the
default current `HEAD` behavior and explain this SHA nuance instead of
silently reopening or re-closing Group 1.

## Shared Rules For Every Opus Session

Start every session with:

```sh
cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
git fetch origin
git status --short --branch
git rev-parse HEAD
git ls-remote origin refs/heads/main
```

Then create or enter an isolated worktree or branch. Do not run multiple
sessions in the same checkout.

Read first:

1. `AGENTS.md`
2. `docs/agent-handoff.md`
3. `docs/launch-candidate-checklist.md`
4. `docs/launch-readiness-review-may-8.md`
5. `docs/runbooks/release-candidate-evidence.md`
6. This file

Non-negotiables:

- Do not declare launch-ready.
- Do not weaken security defaults, profile defaults, or evidence gates.
- Do not print or commit secrets. Reference secrets only by env-var name.
- Do not use `git add .`.
- Do not check Group 6 or Group 7 boxes without candidate-tag evidence.
- Do not treat workflow metadata snapshots as proof by themselves; use
  the fail-closed validators named in the runbooks.
- Ask before taking externally visible maintainer actions unless the
  prompt explicitly grants the action and the required evidence is
  already present.

Recommended branch naming:

```sh
git switch -c opus/<lane>-<date>
```

Recommended model invocation, if launching manually:

```sh
claude --model opus-4.7 "$(cat /path/to/prompt.txt)"
```

## Prompt 1 — Mutation Cron Evidence Lane

Ownership: scheduled `mutation.yml` evidence and only the docs/scripts
that report it.

Allowed write set:

- `docs/launch-candidate-checklist.md`
- `docs/agent-handoff.md`
- `docs/launch-readiness-review-may-8.md`
- `scripts/check-launch-external-status.sh`
- `scripts/test-check-launch-external-status.sh`
- tightly related docs if a verifier proves they are stale

Do not edit unrelated workflows unless a fresh failed/cancelled
scheduled run proves a workflow bug that must be fixed.

```text
You are Claude Code Opus 4.7 working the mutation cron evidence lane for
go-clockify.

Goal:
Close or precisely update the remaining mutation.yml scheduled-run
evidence gate without disturbing Group 1, Group 6, or Group 7.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin
3. git status --short --branch
4. git switch -c opus/mutation-cron-evidence-$(date -u +%Y%m%d)
5. Read AGENTS.md, docs/agent-handoff.md, docs/launch-candidate-checklist.md,
   docs/launch-readiness-review-may-8.md, and scripts/check-launch-external-status.sh.

Evidence to inspect:
- gh run list --repo apet97/go-clockify --workflow mutation.yml --event schedule --limit 10 --json databaseId,status,conclusion,headSha,createdAt,url
- If the latest scheduled run is not green, inspect the run and failed/cancelled jobs with gh run view.
- Run scripts/check-launch-external-status.sh and capture the mutation line.

What to do:
- If a scheduled mutation.yml run is green on the relevant current candidate
  SHA, update the launch checklist/handoff/review ledger with the exact run URL
  and workflow_run_id.
- If the latest scheduled run is still stale, cancelled, or on an old SHA,
  do not check boxes. Update docs only if their current tracking text is stale
  or misleading.
- If the run exposes a real workflow bug, make the smallest workflow/test/doc
  fix that addresses that bug, then add regression coverage where practical.

Verification:
- make doc-parity
- make launch-checklist-parity
- bash scripts/test-check-launch-external-status.sh
- git diff --check

Commit:
Use one commit only if you changed files. Include Why: and Verified: lines.
Do not push unless explicitly told to push this lane branch.
```

## Prompt 2 — Group 6 Candidate-Tag Security Evidence Lane

Ownership: candidate-tag security evidence, `SECURITY.md`, and Group 6
checklist entries.

Allowed write set:

- `SECURITY.md`
- `docs/launch-candidate-checklist.md`
- `docs/agent-handoff.md`
- `docs/launch-readiness-review-may-8.md`
- `docs/runbooks/release-candidate-evidence.md`
- evidence-command scripts only if they fail to capture required
  evidence correctly

```text
You are Claude Code Opus 4.7 working the Group 6 candidate-tag security
evidence lane for go-clockify.

Goal:
Collect or prepare the candidate-tag security walk-through evidence without
weakening security defaults and without checking Group 6 boxes prematurely.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin --tags
3. git status --short --branch
4. Ask the operator for the exact candidate tag if one is not provided in the
   session prompt. Do not invent or cut a tag.
5. If a tag is provided, git switch --detach <tag> and confirm clean status.

Required evidence commands on the candidate tag:
- make check
- make verify-vuln
- make secret-scan
- semgrep scan --config p/default --metrics=off --error --exclude .git --exclude .bench --exclude clockify-mcp .
- git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md' || true
- make verify-fips

What to do:
- If no candidate tag exists, produce a concise handoff note naming exactly
  what remains and stop without editing checklist boxes.
- If the tag exists and all commands pass, update SECURITY.md and the Group 6
  checklist boxes with exact commands, host, date, tag, SHA, and evidence links
  or artifact paths accepted by scripts/check-launch-evidence-gate.sh.
- If any command fails, do not mask or bypass it. Fix only real repo issues,
  or document external/tooling blockers clearly.

Verification:
- make doc-parity
- make launch-checklist-parity
- git diff --check
- Re-run the failed/pass evidence command after any fix that affects it.

Commit:
Use one commit only if you changed files. Include Why: and Verified: lines.
Do not claim launch-ready. Do not push unless explicitly told to push this lane branch.
```

## Prompt 3 — Group 7 Release, Sigstore, SLSA, And npm Evidence Lane

Ownership: release-candidate evidence collection, release-smoke evidence,
asset validation, and npm expected-version proof.

Allowed write set:

- `docs/launch-candidate-checklist.md`
- `docs/agent-handoff.md`
- `docs/launch-readiness-review-may-8.md`
- `docs/runbooks/release-candidate-evidence.md`
- `docs/release-policy.md`
- `docs/verification.md`
- `scripts/prepare-rc-evidence.sh`
- `scripts/test-prepare-rc-evidence.sh`
- `scripts/check-release-assets.sh`
- `scripts/test-check-release-assets.sh`

```text
You are Claude Code Opus 4.7 working the Group 7 release evidence lane for
go-clockify.

Goal:
Drive or prepare release-candidate evidence for Group 7: release-check,
release assets, release-smoke, sigstore/cosign/SLSA/SBOM proof, doctor output,
and npm expected-version proof.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin --tags
3. git status --short --branch
4. Read docs/runbooks/release-candidate-evidence.md, docs/launch-candidate-checklist.md,
   scripts/prepare-rc-evidence.sh, scripts/check-release-assets.sh, and docs/verification.md.
5. Ask the operator for the exact rc tag, for example v1.2.1-rc.1. Do not
   create or push a tag unless the operator explicitly instructs you to cut
   that tag in this session.

If no tag is provided:
- Run make rc-evidence-plan TAG=vX.Y.Z-rc.N with a placeholder tag only as a
  planning check if useful.
- Do not check Group 7 boxes.
- Return a command-by-command runbook for the operator.

If a tag is provided:
- Check out the exact tag, confirm clean detached HEAD, and run
  scripts/prepare-rc-evidence.sh <tag> on the current host.
- If release-smoke did not run, ask before dispatching it with
  scripts/prepare-rc-evidence.sh --trigger-release-smoke <tag>.
- Verify release assets with scripts/check-release-assets.sh.
- Verify npm proof with:
  scripts/check-launch-external-status.sh --candidate-sha "$(git rev-parse HEAD)" --expected-npm-version <tag> --fail-open

What to update:
- Only update Group 7 checklist boxes when the exact evidence exists.
- Link workflow_run_id values, release URLs, artifact names, and evidence
  directories. Local logs alone do not close workflow-backed boxes.

Verification:
- make doc-parity
- make launch-checklist-parity
- bash scripts/test-prepare-rc-evidence.sh
- bash scripts/test-check-release-assets.sh
- git diff --check

Commit:
Use one commit only if you changed files. Include Why: and Verified: lines.
Do not claim launch-ready. Do not push unless explicitly told to push this lane branch.
```

## Prompt 4 — Repository-State External Cleanup Lane

Ownership: GitHub repository description, issue #28, branch-protection
evidence, stale branch inventory, and stale PR hygiene.

Allowed write set:

- `docs/agent-handoff.md`
- `docs/launch-readiness-review-may-8.md`
- `docs/branch-protection.md`
- `docs/release/public-history-review.md`
- `scripts/check-launch-external-status.sh`
- `scripts/test-check-launch-external-status.sh`

This lane may require externally visible GitHub actions. Ask before
mutating repository settings, closing issues, deleting branches, or
commenting on issues.

```text
You are Claude Code Opus 4.7 working the repository-state external cleanup
lane for go-clockify.

Goal:
Close or produce exact maintainer-action packets for repository-state gates:
repository description, issue #28, branch protection, stale local branches,
and stale PR hygiene.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin
3. git status --short --branch
4. Read scripts/check-launch-external-status.sh, docs/branch-protection.md,
   docs/launch-readiness-review-may-8.md, and docs/agent-handoff.md.

Inspect:
- scripts/check-launch-external-status.sh
- gh repo view apet97/go-clockify --json description,visibility,isPrivate,url
- gh issue view 28 --repo apet97/go-clockify --json state,title,url,updatedAt
- gh pr list --repo apet97/go-clockify --state open --limit 100 --json number,title,url,updatedAt,labels
- gh api repos/apet97/go-clockify/branches/main/protection --jq '.'
- git for-each-ref --format='%(refname:short)' refs/heads
- For non-main local branches, compute ahead/behind against origin/main.

What to do:
- If the operator grants permission, update the GitHub repository description
  to exactly:
  128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.
- If the operator grants permission, close or update issue #28 with links to
  Group 2 evidence.
- Do not delete local branches without explicit maintainer approval for each
  branch.
- If branch protection is unreadable or unconfigured, produce a concise action
  packet naming the required checks: Doctor strict smoke, Doctor Postgres
  backend, Shared-service Postgres E2E.
- Update docs/scripts only if the current helper output or action guidance is
  stale or incomplete.

Verification:
- bash scripts/check-launch-external-status.sh
- bash scripts/test-check-launch-external-status.sh
- make doc-parity
- git diff --check

Commit:
Use one commit only if you changed files. Include Why: and Verified: lines.
Do not push unless explicitly told to push this lane branch.
```

## Prompt 5 — Hosted, Commercial, Legal, And Product Approval Lane

Ownership: non-code approval gates and evidence packaging for paid hosted
launch posture.

Allowed write set:

- `docs/release/brand-legal-review.md`
- `docs/release/public-history-review.md`
- `docs/release/local-artifact-review.md`
- `docs/launch-readiness-review-may-8.md`
- `docs/agent-handoff.md`
- `docs/auth-model.md`
- `docs/runbooks/credential-leak-response.md`
- narrowly related hosted runbooks if the approval checklist is stale

Do not invent approvals. Do not mark legal/product/security-review gates
closed unless written evidence exists.

```text
You are Claude Code Opus 4.7 working the hosted/commercial/legal/product
approval lane for go-clockify.

Goal:
Make the remaining non-code gates executable for maintainers without claiming
approval that does not exist: paid-hosted external security review, DPA/terms,
privacy/data-handling review, trademark/official-language approval,
clockify:// URI review, gRPC service-name branding review, paid-commercial RLS
decision, and hosted global quota evidence.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin
3. git status --short --branch
4. Read docs/launch-readiness-review-may-8.md, docs/release/brand-legal-review.md,
   docs/auth-model.md, docs/runbooks/credential-leak-response.md, and
   docs/production-readiness.md.

What to do:
- Audit whether every remaining legal/product/hosted gate has a named owner,
  exact evidence artifact, and non-goal language that prevents local tests from
  being treated as approval.
- If an approval checklist or runbook is vague, make a docs-only improvement
  that turns it into concrete evidence requests.
- Do not add product features, Clockify writes, auth/security relaxations, or
  public "official" claims.
- Do not close any approval gate without written external evidence.

Verification:
- make doc-parity
- bash scripts/test-check-doc-parity.sh
- bash scripts/test-check-launch-review-ledger.sh
- git diff --check

Commit:
Use one commit only if you changed files. Include Why: and Verified: lines.
Do not push unless explicitly told to push this lane branch.
```

## Prompt 6 — Final Integration And Launch-Readiness Audit Lane

Ownership: integrate other lane outputs after they finish. Do not run this
lane until at least one other lane has produced a branch, commit, or evidence
packet.

Allowed write set:

- Docs/checklist/handoff files needed to reconcile landed evidence
- Script guards only when they no longer match documented evidence rules
- Pull request text and final completion audit

```text
You are Claude Code Opus 4.7 working the final integration lane for
go-clockify.

Goal:
Integrate completed lane outputs into one coherent branch and produce a
completion audit that separates closed evidence from still-open launch gates.

Start:
1. cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
2. git fetch origin
3. git status --short --branch
4. List available lane branches and inspect each with git log --oneline and
   git diff --stat origin/main...<branch>.
5. Read AGENTS.md, docs/agent-handoff.md, docs/launch-candidate-checklist.md,
   and docs/launch-readiness-review-may-8.md.

What to do:
- Integrate only narrow, verified lane changes.
- Resolve doc/checklist conflicts in favor of exact evidence, not optimism.
- Preserve the Group 1 SHA nuance: evidence on feef83c, cleanup on a07443b.
- Run the relevant gates for the final touched surface.
- Update the PR description with a prompt-to-artifact checklist.

Minimum verification before opening or updating a PR:
- git status --short --branch
- make doc-parity
- make launch-checklist-parity
- make script-tests
- git diff --check
- scripts/check-launch-external-status.sh
- scripts/check-launch-external-status.sh --candidate-sha feef83c641ced93d2ab6ba07ef766d61c82cc703

Completion audit:
- List every explicit remaining gate and mark closed/open with evidence.
- Do not call the repository launch-ready while Group 6, Group 7,
  mutation cron, repo-state, npm, hosted/commercial, or legal/product gates
  remain open.

Commit:
Use one commit per logical integrated change. Include Why: and Verified: lines.
Open a PR only after the branch is clean and the verification commands above
have run or their blockers are explicitly documented.
```

