#!/usr/bin/env bash
# run_claude_stabilize_loop.sh
#
# Autonomous Claude Code loop for stabilizing and optimizing go-clockify.
# One pass = one issue = one commit. Reviewer pass after each impl pass.
# Stops on BLOCKER/REVERT verdicts or gate failures. Never destructive
# to working tree, never force-resets, never skips git hooks.
#
# Usage:
#   MAX_PASSES=1 ./run_claude_stabilize_loop.sh
#   MAX_PASSES=3 ./run_claude_stabilize_loop.sh
#   MAX_PASSES=3 AUTO_PUSH=1 ./run_claude_stabilize_loop.sh
#
# Env knobs (with defaults):
#   MAX_PASSES=3
#   MODEL_IMPL=sonnet
#   MODEL_REVIEW=opus
#   BRANCH=stabilize/quality-perf
#   AUTO_PUSH=0
#   CLAUDE_TIMEOUT_SECONDS=1800
#   LOG_DIR=.claude-loop

set -euo pipefail

export GIT_PAGER=cat
export PAGER=cat

# ---- Config -----------------------------------------------------------------

MAX_PASSES="${MAX_PASSES:-3}"
MODEL_IMPL="${MODEL_IMPL:-sonnet}"
MODEL_REVIEW="${MODEL_REVIEW:-opus}"
BRANCH="${BRANCH:-stabilize/quality-perf}"
AUTO_PUSH="${AUTO_PUSH:-0}"
CLAUDE_TIMEOUT_SECONDS="${CLAUDE_TIMEOUT_SECONDS:-1800}"
LOG_DIR="${LOG_DIR:-.claude-loop}"

mkdir -p "$LOG_DIR"

# ---- Preflight --------------------------------------------------------------

if [ ! -f Makefile ] || [ ! -f go.mod ]; then
  echo "ERROR: must run from the go-clockify repo root (Makefile + go.mod required)." >&2
  exit 2
fi

if ! command -v claude >/dev/null 2>&1; then
  echo "ERROR: 'claude' CLI not found on PATH." >&2
  exit 2
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "ERROR: not inside a git working tree." >&2
  exit 2
fi

# Branch handling: switch to BRANCH if it exists, else create it from HEAD.
# Never reset, never force-checkout, never touch main.
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
  if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
    echo "== switching to existing branch '$BRANCH' =="
    git switch "$BRANCH"
  else
    echo "== creating branch '$BRANCH' from current HEAD =="
    git switch -c "$BRANCH"
  fi
fi

echo "== branch: $(git rev-parse --abbrev-ref HEAD) =="
echo "== HEAD:   $(git rev-parse HEAD) =="

# ---- Local loop junk ignored ------------------------------------------------

GITIGNORE_CHANGED=0
ensure_ignored() {
  local pat="$1"
  if [ ! -f .gitignore ] || ! grep -qxF "$pat" .gitignore 2>/dev/null; then
    printf '%s\n' "$pat" >> .gitignore
    GITIGNORE_CHANGED=1
  fi
}
ensure_ignored ".claude-loop/"
ensure_ignored ".bench/"

if [ "$GITIGNORE_CHANGED" = "1" ]; then
  # Only commit .gitignore if no other unstaged user files would be swept up.
  OTHER_DIRTY="$(git status --porcelain | awk '$2 != ".gitignore"' | wc -l | tr -d ' ')"
  if [ "$OTHER_DIRTY" = "0" ]; then
    git add .gitignore
    git commit -m "chore: ignore loop artifacts (.claude-loop, .bench)"
  else
    echo "== .gitignore updated; other changes present, leaving uncommitted =="
  fi
fi

# ---- Claude runner with timeout --------------------------------------------

run_claude() {
  # run_claude <model> <prompt_file> <log_file>
  local model="$1"
  local prompt_file="$2"
  local log_file="$3"

  if [ ! -f "$prompt_file" ]; then
    echo "ERROR: prompt file not found: $prompt_file" >&2
    return 2
  fi

  echo "== claude run model=$model timeout=${CLAUDE_TIMEOUT_SECONDS}s log=$log_file =="

  local prompt
  prompt="$(cat "$prompt_file")"

  set +e
  (
    claude --permission-mode bypassPermissions --model "$model" -p "$prompt" 2>&1 &
    local cpid=$!

    (
      sleep "$CLAUDE_TIMEOUT_SECONDS"
      if kill -0 "$cpid" 2>/dev/null; then
        echo "== TIMEOUT after ${CLAUDE_TIMEOUT_SECONDS}s; terminating claude PID $cpid ==" >&2
        kill -TERM "$cpid" 2>/dev/null || true
        sleep 3
        kill -KILL "$cpid" 2>/dev/null || true
      fi
    ) &
    local wpid=$!

    wait "$cpid"
    local crc=$?
    kill "$wpid" 2>/dev/null || true
    wait "$wpid" 2>/dev/null || true
    exit "$crc"
  ) | tee "$log_file"
  local rc=${PIPESTATUS[0]}
  set -e

  echo "== claude run exit rc=$rc (143/137 typically => timeout) =="
  return "$rc"
}

# ---- Helper: commit any uncommitted changes from a pass ---------------------

commit_loop_changes() {
  local pass_num="$1"

  # Anything to commit? (modified, staged, or untracked)
  if git diff --quiet && git diff --cached --quiet \
     && [ -z "$(git ls-files --others --exclude-standard)" ]; then
    return 0
  fi

  echo "== committing uncommitted changes from pass $pass_num =="
  git add -A
  # Defensive: never sweep loop artifacts even if .gitignore was bypassed.
  git restore --staged .claude-loop .bench 2>/dev/null || true

  if git diff --cached --quiet; then
    echo "== nothing staged after unstaging loop artifacts =="
    return 0
  fi

  git commit -m "chore(loop): stabilization pass $pass_num"
}

# ---- Implementation + review loop ------------------------------------------

for i in $(seq 1 "$MAX_PASSES"); do
  echo
  echo "============================================================"
  echo "== Pass $i / $MAX_PASSES                                     "
  echo "============================================================"

  BEFORE_SHA="$(git rev-parse HEAD)"
  PROMPT_FILE="$LOG_DIR/prompt-$i.txt"
  PASS_LOG="$LOG_DIR/pass-$i.log"

  cat > "$PROMPT_FILE" <<'EOF'
You are running an autonomous stabilization+optimization pass on the
go-clockify repo. You may edit files and commit. You will be reviewed
afterwards by a stricter model, so keep changes narrow and defensible.

Hard rules (do NOT violate any):
- Fix exactly ONE issue or perform exactly ONE low-risk optimization.
- Stabilize before optimizing.
- Do NOT weaken auth, audit, policy, dry-run, rate-limit, validation,
  MCP protocol behavior, or the stdlib-only default build.
- Do NOT do broad refactors or unrelated cleanup.
- No placeholder code, no `// TODO later`, no stub returns.
- Generated docs/catalog/config tables must be regenerated via repo
  commands, never hand-edited.
- Stop after one completed change.

Workflow:
1. Run `git status` to see the working tree.
2. Run `make check`.
3. If `make check` FAILS:
   - Fix the FIRST root cause only.
   - Re-run `make check` until it passes.
   - Commit the fix with a useful conventional-commit message.
   - Stop.
4. Else run `make release-check`.
5. If `make release-check` FAILS:
   - Fix the FIRST root cause only.
   - Re-run the failing sub-target, then `make check`, then
     `make release-check`.
   - Commit the fix with a useful conventional-commit message.
   - Stop.
6. Else if `.bench/baseline.txt` does NOT exist:
   - Run `make bench BENCH_OUT=.bench/baseline.txt`.
   - Note: `.bench/` is gitignored — there is nothing to commit here.
   - Stop.
7. Else perform exactly ONE low-risk optimization:
   - Identify a concrete, narrow target (a hot path, an allocation,
     a redundant computation). State it clearly in the output.
   - Make the change.
   - Run targeted tests (the package(s) you touched, with -race).
   - Run `make check`.
   - Run `make verify-bench`.
   - Keep ONLY if benchmarks improve (or the code is meaningfully
     simpler with no benchmark regression). Otherwise revert in-tree
     and stop with a clear "no-op pass" message.
   - Commit the kept change with a useful conventional-commit message
     (subject + body; body ends with `Why:` and `Verified:` lines as
     this repo's convention).
   - Stop.

At the end print:
- one-line summary of what changed
- commit SHA if you committed
- the gate commands you ran and their pass/fail
- any benchmark deltas if a perf change was attempted
EOF

  # Run the implementation pass. Don't abort the script on claude rc;
  # we still want to inspect any changes it made.
  set +e
  run_claude "$MODEL_IMPL" "$PROMPT_FILE" "$PASS_LOG"
  IMPL_RC=$?
  set -e

  if [ "$IMPL_RC" -ne 0 ]; then
    echo "== impl pass $i exited rc=$IMPL_RC (timeout or claude error) =="
    echo "== inspect: $PASS_LOG =="
    # Still try to commit any local edits Claude made before dying.
  fi

  # Commit anything Claude left uncommitted.
  commit_loop_changes "$i"

  AFTER_SHA="$(git rev-parse HEAD)"

  if [ "$AFTER_SHA" = "$BEFORE_SHA" ]; then
    echo "== pass $i produced no commit; ending loop early =="
    break
  fi

  # ---- Review pass ----------------------------------------------------------

  REVIEW_PROMPT="$LOG_DIR/review-$i.txt"
  REVIEW_LOG="$LOG_DIR/review-$i.log"

  cat > "$REVIEW_PROMPT" <<EOF
You are reviewing commit $AFTER_SHA in the go-clockify repo as a strict
Go / MCP / security / performance reviewer. DO NOT edit any files.

Compare the commit against its parent:
  git show --stat $AFTER_SHA
  git show $AFTER_SHA

Evaluate:
- behavior equivalence
- MCP protocol compatibility
- auth safety
- audit safety
- policy / risk-class semantics
- dry-run semantics
- rate-limit semantics
- input validation and error clarity
- concurrency / race risks
- memory lifetime risks
- benchmark validity (if this is a perf change)
- missing tests
- generated docs / tool catalog / config-table parity

Return EXACTLY this format and nothing else after it:

VERDICT: KEEP or BLOCKER or REVERT

BLOCKERS:
- ...

NON_BLOCKERS:
- ...

MISSING_TESTS:
- ...

RECOMMENDED_COMMANDS:
- ...
EOF

  set +e
  run_claude "$MODEL_REVIEW" "$REVIEW_PROMPT" "$REVIEW_LOG"
  REVIEW_RC=$?
  set -e

  if [ "$REVIEW_RC" -ne 0 ]; then
    echo "== review pass exited rc=$REVIEW_RC; treating as inconclusive =="
    echo "== inspect: $REVIEW_LOG =="
    echo "== stopping loop for human review =="
    exit 1
  fi

  if grep -Eq "^VERDICT: (BLOCKER|REVERT)" "$REVIEW_LOG"; then
    echo
    echo "!! Reviewer returned BLOCKER or REVERT verdict on commit $AFTER_SHA"
    echo "!! Review log: $REVIEW_LOG"
    echo "!! NOT auto-reverting. Inspect, then either revert manually or amend."
    exit 1
  fi

  # ---- Post-review gates ----------------------------------------------------

  echo "== post-review gates after pass $i =="
  if ! make check; then
    echo "ERROR: post-review 'make check' failed after commit $AFTER_SHA" >&2
    exit 1
  fi
  if ! make release-check; then
    echo "ERROR: post-review 'make release-check' failed after commit $AFTER_SHA" >&2
    exit 1
  fi

  if [ "$AUTO_PUSH" = "1" ]; then
    echo "== AUTO_PUSH=1: pushing $BRANCH =="
    git push -u origin "$BRANCH"
  else
    echo "== AUTO_PUSH=0: not pushing (set AUTO_PUSH=1 to enable) =="
  fi

  echo "== pass $i complete =="
done

# ---- Final review -----------------------------------------------------------

echo
echo "============================================================"
echo "== Final release review                                     "
echo "============================================================"

FINAL_PROMPT="$LOG_DIR/final-review-prompt.txt"
FINAL_LOG="$LOG_DIR/final-review.log"

cat > "$FINAL_PROMPT" <<'EOF'
You are doing a final release review of the go-clockify repo.

DO NOT edit code unless you find a CONCRETE release blocker — and even
then, narrow the change to the minimum required to unblock release.

Run, in order:
  git status
  make check
  make release-check
  make verify-bench
  go test -tags=grpc ./...

Then create FINAL_STABILIZATION_REPORT.md at the repo root containing:
- Verdict: SHIP or DO NOT SHIP
- Commands you ran and pass/fail for each
- Benchmark result summary (verify-bench output highlights)
- Blockers (each with file:line and rationale)
- Non-blockers (deferred items)
- Remaining risks
- Suggested release notes (1-paragraph summary + bullet list)

Keep the report tight; this is a shippability signal, not a novel.
EOF

set +e
run_claude "$MODEL_REVIEW" "$FINAL_PROMPT" "$FINAL_LOG"
FINAL_RC=$?
set -e

if [ "$FINAL_RC" -ne 0 ]; then
  echo "== final review exited rc=$FINAL_RC; inspect $FINAL_LOG =="
fi

# Commit only the report file if it was created/modified.
if [ -f FINAL_STABILIZATION_REPORT.md ] \
   && (! git diff --quiet -- FINAL_STABILIZATION_REPORT.md \
       || git ls-files --others --exclude-standard --error-unmatch \
            FINAL_STABILIZATION_REPORT.md >/dev/null 2>&1); then
  git add FINAL_STABILIZATION_REPORT.md
  if ! git diff --cached --quiet -- FINAL_STABILIZATION_REPORT.md; then
    git commit -m "docs: final stabilization report"
  fi
fi

if [ "$AUTO_PUSH" = "1" ]; then
  git push -u origin "$BRANCH" || true
fi

# ---- Summary ----------------------------------------------------------------

echo
echo "============================================================"
echo "== Loop finished                                             "
echo "============================================================"
echo "Branch:     $(git rev-parse --abbrev-ref HEAD)"
echo "Recent commits:"
git --no-pager log --oneline -5
echo
echo "Working tree status:"
git status
echo
echo "Logs:       $LOG_DIR"
echo "AUTO_PUSH:  $AUTO_PUSH"
