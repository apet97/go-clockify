# Claude Code Campaign Harness

This runbook launches a local multi-agent Claude Code campaign. It is
for development discovery only; it is not launch evidence and does not
run live Clockify tests.

## What It Creates

`scripts/claude-campaign.sh` creates one isolated git worktree and one
branch per lane:

- `code-quality`
- `performance`
- `stability`
- `observability`
- `ai-agent-ux`

Each worktree gets an ignored `.claude-campaign/prompt.md` file. The
launcher opens one iTerm window per lane, waits `0.5` seconds, then
types a command shaped like:

```sh
cd <lane-worktree> && claude --model opus "$(cat <lane-worktree>/.claude-campaign/prompt.md)"
```

The sessions then run in parallel. Each prompt tells the agent to make
at most one narrow local commit, run appropriate tests, avoid live
Clockify credentials, and never push.

## Plan First

```sh
make claude-campaign-plan
```

The dry-run prints the campaign ID, base SHA, lane branches, worktree
paths, prompt paths, and exact Claude command. It does not create
worktrees or open iTerm.

## Launch

```sh
make claude-campaign
```

Useful overrides:

```sh
CLAUDE_CAMPAIGN_ID=lc-audit-1 make claude-campaign
CLAUDE_CAMPAIGN_BASE_REF=main make claude-campaign
CLAUDE_CAMPAIGN_WORKTREE_ROOT=/tmp/go-clockify-campaigns make claude-campaign
CLAUDE_CAMPAIGN_LANES=performance,observability make claude-campaign
CLAUDE_CAMPAIGN_ITERM_APP=iTerm2 make claude-campaign
```

To create worktrees and prompt files without opening iTerm:

```sh
bash scripts/claude-campaign.sh --prepare-only
```

## Review And Integrate

After agents stop:

```sh
git worktree list
git -C <lane-worktree> status -sb
git -C <lane-worktree> log --oneline -3
```

Review each lane branch separately. Cherry-pick or merge only changes
that are narrow, tested, and consistent with `AGENTS.md`. Before any
push, run the repo gates required by the touched files; at minimum:

```sh
make check
make doc-parity
make config-doc-parity
make catalog-drift
git diff --check
```

Do not treat campaign output, local green checks, or manual review as
official launch readiness. Groups 1, 6, and 7 still require the
external evidence named in `docs/launch-candidate-checklist.md`.
