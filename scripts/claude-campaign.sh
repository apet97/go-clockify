#!/usr/bin/env bash
#
# Launch a parallel Claude Code campaign in isolated git worktrees.
# The default campaign creates five lanes:
#   code-quality, performance, stability, observability, ai-agent-ux
#
# Non-dry runs create one branch + worktree per lane, write a lane prompt
# under .claude-campaign/, then open one iTerm window per lane and type:
#   claude --model opus "$(cat <prompt-file>)"

set -euo pipefail

DEFAULT_LANES=(code-quality performance stability observability ai-agent-ux)

DRY_RUN=0
PREPARE_ONLY=0
CAMPAIGN_ID="${CLAUDE_CAMPAIGN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
BASE_REF="${CLAUDE_CAMPAIGN_BASE_REF:-HEAD}"
WORKTREE_ROOT="${CLAUDE_CAMPAIGN_WORKTREE_ROOT:-../.claude-campaign-worktrees}"
MODEL="${CLAUDE_CAMPAIGN_MODEL:-opus}"
CLAUDE_BIN="${CLAUDE_BIN:-claude}"
ITERM_APP="${CLAUDE_CAMPAIGN_ITERM_APP:-iTerm}"
LAUNCH_DELAY="${CLAUDE_CAMPAIGN_LAUNCH_DELAY:-0.5}"
LANES_CSV="${CLAUDE_CAMPAIGN_LANES:-}"

usage() {
  cat <<'EOF'
Usage:
  scripts/claude-campaign.sh [options]

Options:
  --dry-run                 Print the campaign plan without creating worktrees.
  --prepare-only, --no-launch
                            Create worktrees and prompt files, but do not open iTerm.
  --campaign-id ID          Stable ID used in branch/worktree names.
  --base-ref REF            Commit/ref to branch every worktree from (default: HEAD).
  --worktree-root DIR       Parent directory for campaign worktrees.
  --model MODEL             Claude model passed to every lane (default: opus).
  --claude-bin PATH         Claude executable name/path (default: claude).
  --lanes CSV               Comma-separated subset of default lanes.
  --launch-delay SECONDS    iTerm delay before typing command (default: 0.5).
  --help                    Show this help.

Environment aliases:
  CLAUDE_CAMPAIGN_ID, CLAUDE_CAMPAIGN_BASE_REF, CLAUDE_CAMPAIGN_WORKTREE_ROOT,
  CLAUDE_CAMPAIGN_MODEL, CLAUDE_CAMPAIGN_LANES, CLAUDE_CAMPAIGN_ITERM_APP,
  CLAUDE_CAMPAIGN_LAUNCH_DELAY, CLAUDE_BIN.
EOF
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --prepare-only|--no-launch)
      PREPARE_ONLY=1
      ;;
    --campaign-id)
      [ "$#" -ge 2 ] || fail "--campaign-id requires a value"
      CAMPAIGN_ID="$2"
      shift
      ;;
    --base-ref)
      [ "$#" -ge 2 ] || fail "--base-ref requires a value"
      BASE_REF="$2"
      shift
      ;;
    --worktree-root)
      [ "$#" -ge 2 ] || fail "--worktree-root requires a value"
      WORKTREE_ROOT="$2"
      shift
      ;;
    --model)
      [ "$#" -ge 2 ] || fail "--model requires a value"
      MODEL="$2"
      shift
      ;;
    --claude-bin)
      [ "$#" -ge 2 ] || fail "--claude-bin requires a value"
      CLAUDE_BIN="$2"
      shift
      ;;
    --lanes)
      [ "$#" -ge 2 ] || fail "--lanes requires a value"
      LANES_CSV="$2"
      shift
      ;;
    --launch-delay)
      [ "$#" -ge 2 ] || fail "--launch-delay requires a value"
      LAUNCH_DELAY="$2"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
  shift
done

case "$CAMPAIGN_ID" in
  *[!A-Za-z0-9._-]*|'') fail "campaign id must contain only A-Z, a-z, 0-9, dot, underscore, or dash" ;;
esac
case "$MODEL" in
  *[!A-Za-z0-9._:-]*|'') fail "model must contain only A-Z, a-z, 0-9, dot, underscore, colon, or dash" ;;
esac
case "$LAUNCH_DELAY" in
  ''|*[!0-9.]*|.*|*.*.*) fail "launch delay must be a non-negative number" ;;
esac

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  fail "must run inside the go-clockify git worktree"
fi
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if [ ! -f Makefile ] || [ ! -f go.mod ] || [ ! -f AGENTS.md ]; then
  fail "must run from the go-clockify repo root (Makefile, go.mod, AGENTS.md required)"
fi

BASE_SHA="$(git rev-parse --verify "${BASE_REF}^{commit}")" || fail "base ref not found: $BASE_REF"

if [ -n "$LANES_CSV" ]; then
  IFS=',' read -r -a LANES <<<"$LANES_CSV"
else
  LANES=("${DEFAULT_LANES[@]}")
fi
[ "${#LANES[@]}" -gt 0 ] || fail "at least one lane is required"

normalize_lane() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr '_' '-'
}

validate_lane() {
  case "$1" in
    code-quality|performance|stability|observability|ai-agent-ux) return 0 ;;
    *) fail "unknown lane '$1' (expected one of: ${DEFAULT_LANES[*]})" ;;
  esac
}

ABS_WORKTREE_ROOT="$WORKTREE_ROOT"
case "$ABS_WORKTREE_ROOT" in
  /*) ;;
  *) ABS_WORKTREE_ROOT="$REPO_ROOT/$ABS_WORKTREE_ROOT" ;;
esac
CAMPAIGN_DIR="$ABS_WORKTREE_ROOT/$CAMPAIGN_ID"

lane_guidance() {
  case "$1" in
    code-quality)
      cat <<'EOF'
Find one narrow code-quality improvement: clearer validation, simpler control
flow, tighter test coverage, or a small removal of meaningful duplication.
Prefer existing patterns over new abstractions. Do not make broad style churn.
EOF
      ;;
    performance)
      cat <<'EOF'
Find one narrow performance improvement. Start from existing benchmarks or a
cheap allocation/CPU inspection. Keep the change only if targeted tests pass
and benchmark evidence is neutral or better. Do not weaken behavior for speed.
EOF
      ;;
    stability)
      cat <<'EOF'
Find one narrow stability hardening: race prevention, cancellation handling,
retry/error-boundary clarity, panic prevention, or flaky-test reduction. Add or
tighten tests around the failure mode you address.
EOF
      ;;
    observability)
      cat <<'EOF'
Find one narrow observability improvement: safer logs, clearer metrics/tracing,
audit/debug context, or better operator-visible error data. Never log secrets,
tokens, raw authorization headers, or live Clockify payloads containing private
workspace data.
EOF
      ;;
    ai-agent-ux)
      cat <<'EOF'
Find one narrow AI-agent UX improvement: better tool descriptions, safer error
messages, clearer docs for agent workflows, or more actionable suggested next
steps. Do not add a new feature unless it is tiny and directly improves agent
use of existing behavior.
EOF
      ;;
  esac
}

render_prompt() {
  local lane="$1"
  local branch="$2"
  local worktree="$3"
  local guidance
  guidance="$(lane_guidance "$lane")"
  cat <<EOF
You are Claude Code running as the $lane lane of a parallel Opus campaign for
go-clockify.

Worktree: $worktree
Branch: $branch
Base ref: $BASE_REF
Base SHA: $BASE_SHA

Campaign objective:
Improve exactly one small, defensible part of the repo in your lane. You may
edit files and commit in this isolated worktree. Other Claude agents are
working in sibling worktrees on different lanes, so avoid sweeping refactors
and do not touch unrelated files.

Lane brief:
$guidance

Hard rules:
- Read AGENTS.md first, then docs/agent-handoff.md if launch-state context is
  relevant.
- Do not run destructive live Clockify tests. Do not use live credentials.
- Do not weaken auth, audit, policy, dry-run, rate-limit, validation, MCP
  protocol behavior, or production defaults.
- Do not modify CI/CD, auth, or security settings unless your lane has found a
  concrete bug and the fix is narrow.
- Do not edit generated docs/catalog/config tables by hand. If descriptors or
  config specs change, run the repo generator commands named in AGENTS.md.
- Do not push. Commit locally on this lane branch only.
- Stop after exactly one logical change, or stop with a clear no-op summary if
  no safe change is found.

Required workflow:
1. Run \`git status -sb\` and confirm you are on branch \`$branch\`.
2. Inspect the lane-relevant files and choose one narrow target.
3. Make the smallest viable change.
4. Run targeted tests for touched packages.
5. Run \`make check\`.
6. Run any additional parity target required by the files you changed.
7. Commit with a conventional subject and a body ending in \`Why:\` and
   \`Verified:\` lines.

Final response format:
- Lane:
- Change:
- Commit SHA:
- Commands run:
- Residual risk or follow-up:
EOF
}

command_for_lane() {
  local worktree="$1"
  local prompt_file="$2"
  local cmd
  printf -v cmd 'cd %q && %q --model %q "$(cat %q)"' "$worktree" "$CLAUDE_BIN" "$MODEL" "$prompt_file"
  printf '%s\n' "$cmd"
}

escape_applescript() {
  sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' <<<"$1"
}

launch_iterm() {
  local lane="$1"
  local command="$2"
  local escaped_command escaped_app
  escaped_command="$(escape_applescript "$command")"
  escaped_app="$(escape_applescript "$ITERM_APP")"
  osascript <<EOF
tell application "$escaped_app"
  activate
  set newWindow to (create window with default profile)
  delay $LAUNCH_DELAY
  tell current session of newWindow
    write text "$escaped_command"
  end tell
end tell
EOF
  printf 'launched lane=%s app=%s\n' "$lane" "$ITERM_APP"
}

print_plan() {
  printf 'claude-campaign plan\n'
  printf 'repo_root=%s\n' "$REPO_ROOT"
  printf 'base_ref=%s\n' "$BASE_REF"
  printf 'base_sha=%s\n' "$BASE_SHA"
  printf 'campaign_id=%s\n' "$CAMPAIGN_ID"
  printf 'campaign_dir=%s\n' "$CAMPAIGN_DIR"
  printf 'model=%s\n' "$MODEL"
  printf 'launch_delay=%s\n' "$LAUNCH_DELAY"
  printf 'lanes=%s\n' "${LANES[*]}"
  for lane_raw in "${LANES[@]}"; do
    lane="$(normalize_lane "$lane_raw")"
    validate_lane "$lane"
    branch="campaign/$CAMPAIGN_ID/$lane"
    worktree="$CAMPAIGN_DIR/$lane"
    prompt_file="$worktree/.claude-campaign/prompt.md"
    printf 'lane=%s branch=%s worktree=%s prompt=%s\n' "$lane" "$branch" "$worktree" "$prompt_file"
    printf 'command=%s\n' "$(command_for_lane "$worktree" "$prompt_file")"
  done
}

if [ "$DRY_RUN" = "1" ]; then
  print_plan
  exit 0
fi

if [ -n "$(git status --porcelain)" ] && [ "${ALLOW_DIRTY:-0}" != "1" ]; then
  fail "working tree is not clean; commit/stash first or set ALLOW_DIRTY=1"
fi

if [ -e "$CAMPAIGN_DIR" ]; then
  fail "campaign directory already exists: $CAMPAIGN_DIR"
fi

if [ "$PREPARE_ONLY" != "1" ]; then
  command -v osascript >/dev/null 2>&1 || fail "osascript is required to launch iTerm"
fi

mkdir -p "$CAMPAIGN_DIR"
MANIFEST="$CAMPAIGN_DIR/manifest.tsv"
printf 'lane\tbranch\tworktree\tprompt\tcommand\n' >"$MANIFEST"

for lane_raw in "${LANES[@]}"; do
  lane="$(normalize_lane "$lane_raw")"
  validate_lane "$lane"
  branch="campaign/$CAMPAIGN_ID/$lane"
  worktree="$CAMPAIGN_DIR/$lane"
  prompt_file="$worktree/.claude-campaign/prompt.md"

  if git show-ref --verify --quiet "refs/heads/$branch"; then
    fail "branch already exists: $branch"
  fi
  if [ -e "$worktree" ]; then
    fail "worktree path already exists: $worktree"
  fi

  git worktree add -b "$branch" "$worktree" "$BASE_SHA"
  mkdir -p "$worktree/.claude-campaign"
  render_prompt "$lane" "$branch" "$worktree" >"$prompt_file"
  command="$(command_for_lane "$worktree" "$prompt_file")"
  printf '%s\t%s\t%s\t%s\t%s\n' "$lane" "$branch" "$worktree" "$prompt_file" "$command" >>"$MANIFEST"

  if [ "$PREPARE_ONLY" = "1" ]; then
    printf 'prepared lane=%s branch=%s worktree=%s\n' "$lane" "$branch" "$worktree"
  else
    launch_iterm "$lane" "$command"
  fi
done

printf 'campaign_manifest=%s\n' "$MANIFEST"
printf 'campaign_dir=%s\n' "$CAMPAIGN_DIR"
