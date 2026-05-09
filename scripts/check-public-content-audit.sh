#!/usr/bin/env bash
#
# check-public-content-audit.sh
#
# Read-only helper for the public-repository content audit described in
# the May 8 final launch plan. Default mode reports open/unknown items
# and exits 0; pass --fail-open when a non-zero exit is useful.

set -euo pipefail

repo_root="$(pwd)"
mode="run"
fail_open=0

usage() {
  cat <<'EOF'
Usage:
  scripts/check-public-content-audit.sh [--repo-root DIR] [--fail-open]
  scripts/check-public-content-audit.sh --plan

Options:
  --repo-root DIR   Repository root to inspect. Default: current directory.
  --fail-open      Exit 1 when any checked item is open or unknown.
  --plan           Print the read-only checks without scanning.
  -h, --help       Show this help.

Checks:
  - gitleaks detect on candidate branch content and the full working tree;
  - tracked references to workstation-private names and scratch dirs;
  - TLS verification bypass markers in candidate branch content;
  - root LICENSE presence and MIT license wording;
  - .gitignore coverage for workstation-private and generated artifacts;
  - .gitleaks.toml allowlist entries carry one-line descriptions;
  - candidate branch content does not include workstation CLAUDE.md files;
  - candidate branch content does not assign live Clockify secret env vars;
  - env-like files in candidate content and the full working tree;
  - TODO lines in tracked Go/Markdown that mention internal/private context;
  - non-test internal/cmd Go task markers in operator-facing code;
  - recent commit messages matching secret/token/password/key= patterns;
  - tracked apet97 references as rebrand context, not a blocker.

Open or unknown findings include maintainer action hints. The helper
does not remove files, rewrite history, or change repository visibility.
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo-root)
      [ "$#" -ge 2 ] || die "--repo-root requires DIR"
      repo_root="$2"
      shift 2
      ;;
    --fail-open)
      fail_open=1
      shift
      ;;
    --plan)
      mode="plan"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      die "unknown option: $1"
      ;;
    *)
      die "unexpected argument: $1"
      ;;
  esac
done

repo_root="$(cd "$repo_root" && pwd)"

if [ "$mode" = "plan" ]; then
  cat <<EOF
Public content audit plan for ${repo_root}

Read-only checks:
  - gitleaks detect --no-git over tracked plus unignored files copied to <tmp>
  - gitleaks detect --no-git --source ${repo_root} --redact --config ${repo_root}/.gitleaks.toml --report-format json --report-path <tmp>
  - git grep tracked files for workstation-private names and scratch dirs
  - grep candidate branch content for TLS verification bypass markers
  - test -f LICENSE && grep -q 'MIT License' LICENSE
  - test .gitignore coverage for local assistant state and generated artifacts
  - test .gitleaks.toml allowlist entries for non-empty descriptions
  - git ls-files --cached --others --exclude-standard for CLAUDE.md files
  - git ls-files --cached --others --exclude-standard for CLOCKIFY_LIVE_* assignments
  - git ls-files --cached --others --exclude-standard for candidate .env* files
  - find ${repo_root} -type f -name '.env*' excluding .git and node_modules
  - git grep -n -I -E 'TODO.*(internal|private)' -- '*.go' '*.md'
  - git grep -n -I -E -w 'TODO|FIXME|XXX|HACK|hack' -- 'internal/**/*.go' 'cmd/**/*.go' ':(exclude)**/*_test.go'
  - report non-test internal/cmd Go task markers separately from fixtures
  - git log --all --format='%h%x09%s' -200 matched against secret/token/password/key=
  - git grep -l apet97 for public rebrand context

This helper does not mutate GitHub, git history, branches, files, or
launch checklist boxes. It reports whether public-flip content evidence
is closed, open, or unknown, and prints maintainer actions for open or
unknown findings.
EOF
  exit 0
fi

command -v git >/dev/null 2>&1 || die "git is required for public content audit"
git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git checkout: $repo_root"

open_count=0
unknown_count=0
candidate_open_count=0
candidate_unknown_count=0
history_open_count=0
history_unknown_count=0
local_open_count=0
local_unknown_count=0

record() {
  local status="$1"
  shift
  printf '[%s] %s\n' "$status" "$*"
  case "$status" in
    open) open_count=$((open_count + 1)) ;;
    unknown) unknown_count=$((unknown_count + 1)) ;;
  esac
}

action() {
  printf '       action: %s\n' "$*"
}

record_candidate() {
  local status="$1"
  shift
  record "$status" "$@"
  case "$status" in
    open) candidate_open_count=$((candidate_open_count + 1)) ;;
    unknown) candidate_unknown_count=$((candidate_unknown_count + 1)) ;;
  esac
}

record_history() {
  local status="$1"
  shift
  record "$status" "$@"
  case "$status" in
    open) history_open_count=$((history_open_count + 1)) ;;
    unknown) history_unknown_count=$((history_unknown_count + 1)) ;;
  esac
}

record_local() {
  local status="$1"
  shift
  record "$status" "$@"
  case "$status" in
    open) local_open_count=$((local_open_count + 1)) ;;
    unknown) local_unknown_count=$((local_unknown_count + 1)) ;;
  esac
}

path_state() {
  local rel="$1"
  if git -C "$repo_root" ls-files --error-unmatch -- "$rel" >/dev/null 2>&1; then
    printf 'tracked'
  elif git -C "$repo_root" check-ignore -q -- "$rel" 2>/dev/null; then
    printf 'ignored'
  elif [ -e "$repo_root/$rel" ]; then
    printf 'untracked'
  else
    printf 'unknown'
  fi
}

repo_relative_path() {
  local path="$1"
  path="$(normalize_existing_path "$path")"
  case "$path" in
    "$repo_root"/*) printf '%s' "${path#"$repo_root"/}" ;;
    *) printf '%s' "$path" ;;
  esac
}

normalize_existing_path() {
  local path="$1"
  local base
  local dir
  if [ -e "$path" ]; then
    base="$(basename "$path")"
    dir="$(dirname "$path")"
    if dir="$(cd "$dir" 2>/dev/null && pwd)"; then
      printf '%s/%s' "$dir" "$base"
      return
    fi
  fi
  printf '%s' "$path"
}

path_is_documented() {
  local review_file="$1"
  local rel="$2"
  [ -f "$review_file" ] && grep -qF -- "$rel" "$review_file"
}

tree_relative_path() {
  local root="$1"
  local path="$2"
  path="$(normalize_existing_path "$path")"
  case "$path" in
    "$root"/*) printf '%s' "${path#"$root"/}" ;;
    *) printf '%s' "$path" ;;
  esac
}

printf 'Public content audit for %s\n\n' "$repo_root"

if command -v gitleaks >/dev/null 2>&1; then
  if command -v jq >/dev/null 2>&1; then
    candidate_tree="$(mktemp -d "${TMPDIR:-/tmp}/public-content-candidate.XXXXXX")"
    candidate_manifest="$(mktemp "${TMPDIR:-/tmp}/public-content-candidate-files.XXXXXX")"
    git -C "$repo_root" ls-files -z --cached --others --exclude-standard > "$candidate_manifest"
    while IFS= read -r -d '' rel; do
      [ -f "$repo_root/$rel" ] || continue
      mkdir -p "$candidate_tree/$(dirname "$rel")"
      cp -p "$repo_root/$rel" "$candidate_tree/$rel"
    done < "$candidate_manifest"

    candidate_report_path="$(mktemp "${TMPDIR:-/tmp}/public-content-candidate-gitleaks.XXXXXX")"
    candidate_gitleaks_output=""
    if candidate_gitleaks_output="$(gitleaks detect --no-git --source "$candidate_tree" --redact \
        --config "$repo_root/.gitleaks.toml" --report-format json \
        --report-path "$candidate_report_path" --no-banner 2>&1)"; then
      record_candidate closed "gitleaks candidate branch-content scan returned no findings"
    else
      candidate_finding_count="$(jq 'length' "$candidate_report_path" 2>/dev/null || printf '0')"
      if [ "$candidate_finding_count" -gt 0 ] 2>/dev/null; then
        record_candidate open "gitleaks candidate branch-content scan found ${candidate_finding_count} redacted findings"
        jq -r '.[] | [.RuleID, .File, (.StartLine|tostring), .Description] | @tsv' "$candidate_report_path" |
          while IFS=$'\t' read -r rule file line description; do
            rel_file="$(tree_relative_path "$candidate_tree" "$file")"
            state="$(path_state "$rel_file")"
            printf '       gitleaks-candidate: %s:%s [%s, %s] %s\n' "$rel_file" "$line" "$rule" "$state" "$description"
          done
        action "remove, redact, or explicitly accept candidate branch findings before a public visibility flip."
      else
        record_candidate unknown "candidate branch-content gitleaks failed but no parseable findings were written: $candidate_gitleaks_output"
        action "rerun with a working gitleaks/jq setup and review the raw candidate branch scan output before public visibility."
      fi
    fi
    rm -rf "$candidate_tree"
    rm -f "$candidate_manifest" "$candidate_report_path"

    report_path="$(mktemp "${TMPDIR:-/tmp}/public-content-gitleaks.XXXXXX")"
    gitleaks_output=""
    if gitleaks_output="$(gitleaks detect --no-git --source "$repo_root" --redact \
        --config "$repo_root/.gitleaks.toml" --report-format json \
        --report-path "$report_path" --no-banner 2>&1)"; then
      record_local closed "gitleaks working-tree scan returned no findings"
    else
      finding_count="$(jq 'length' "$report_path" 2>/dev/null || printf '0')"
      if [ "$finding_count" -gt 0 ] 2>/dev/null; then
        local_artifact_review="$repo_root/docs/release/local-artifact-review.md"
        undocumented_gitleaks=""
        gitleaks_details="$(jq -r '.[] | [.RuleID, .File, (.StartLine|tostring), .Description] | @tsv' "$report_path" |
          while IFS=$'\t' read -r rule file line description; do
            rel_file="$(repo_relative_path "$file")"
            state="$(path_state "$rel_file")"
            printf '%s\t%s\t%s\t%s\t%s\n' "$rel_file" "$line" "$rule" "$state" "$description"
          done)"
        while IFS=$'\t' read -r rel_file _line _rule state _description; do
          [ -n "$rel_file" ] || continue
          if [ "$state" = "tracked" ] || ! path_is_documented "$local_artifact_review" "$rel_file"; then
            undocumented_gitleaks="${undocumented_gitleaks}${rel_file}"$'\n'
          fi
        done <<< "$gitleaks_details"
        if [ -z "$undocumented_gitleaks" ]; then
          record_local closed "gitleaks working-tree findings are documented ignored local artifacts"
          while IFS=$'\t' read -r rel_file line rule state description; do
            [ -n "$rel_file" ] || continue
            printf '       gitleaks-local-reviewed: %s:%s [%s, %s] %s\n' "$rel_file" "$line" "$rule" "$state" "$description"
          done <<< "$gitleaks_details"
        else
          record_local open "gitleaks working-tree scan found ${finding_count} redacted findings"
          while IFS=$'\t' read -r rel_file line rule state description; do
            [ -n "$rel_file" ] || continue
            printf '       gitleaks: %s:%s [%s, %s] %s\n' "$rel_file" "$line" "$rule" "$state" "$description"
          done <<< "$gitleaks_details"
          action "clean ignored/local artifacts with maintainer approval, for example make clean-deep CONFIRM=1, or document explicit acceptance in docs/release/local-artifact-review.md."
        fi
      else
        record_local unknown "gitleaks failed but no parseable findings were written: $gitleaks_output"
        action "rerun with a working gitleaks/jq setup and review the raw full working-tree scan output before public visibility."
      fi
    fi
    rm -f "$report_path"
  else
    record_candidate unknown "jq is not installed; cannot safely summarize candidate gitleaks JSON"
    action "install jq or run this audit in CI before relying on candidate branch gitleaks evidence."
    record_local unknown "jq is not installed; cannot safely summarize full working-tree gitleaks JSON"
    action "install jq or run this audit in CI before relying on full working-tree gitleaks evidence."
  fi
else
  record_candidate unknown "gitleaks is not installed; cannot run candidate public secret scan"
  action "install gitleaks or run this audit in CI before relying on candidate branch public-content evidence."
  record_local unknown "gitleaks is not installed; cannot run full working-tree public secret scan"
  action "install gitleaks or run this audit in CI before relying on local artifact/full-tree evidence."
fi

personal_marker="pet""kovic"
loop_marker="claude""-loop"
scratch_pattern="${personal_marker}|${loop_marker}|[.]planning|[.]claude|[.]remember"
scratch_refs="$(git -C "$repo_root" grep -n -I -E "$scratch_pattern" -- . ':(exclude).gitignore' ':(exclude).gitleaks.toml' 2>/dev/null |
  awk -F: '{ print $1 ":" $2 }' || true)"
if [ -n "$scratch_refs" ]; then
  record_candidate open "tracked personal/scratch references require review before a public repo flip"
  printf '%s\n' "$scratch_refs" | sed 's/^/       tracked-ref: /'
  action "remove or neutralize tracked workstation-private references, or document why each is public-safe."
else
  record_candidate closed "no tracked personal/scratch references outside .gitignore/.gitleaks.toml"
fi

insecure_skip="Insecure""SkipVerify"
reject_unauthorized="reject""Unauthorized"
node_tls_reject="NODE_TLS_REJECT""_UNAUTHORIZED"
curl_insecure="--""insecure"
tls_bypass_pattern="${insecure_skip}|verify[[:space:]]*=[[:space:]]*[Ff]alse|${reject_unauthorized}[[:space:]]*:[[:space:]]*false|${node_tls_reject}[[:space:]]*=[[:space:]]*0|curl([^[:alnum:]]|[[:space:]])[^[:cntrl:]]*(${curl_insecure}|[[:space:]]-k([[:space:]]|$))"
tls_bypass_hits="$(git -C "$repo_root" ls-files -z --cached --others --exclude-standard |
  while IFS= read -r -d '' rel; do
    [ -f "$repo_root/$rel" ] || continue
    grep -n -I -E "$tls_bypass_pattern" -- "$repo_root/$rel" 2>/dev/null |
      while IFS=: read -r _file line_no _rest; do
        [ -n "$line_no" ] || continue
        printf '%s:%s\n' "$rel" "$line_no"
      done
  done || true)"
if [ -n "$tls_bypass_hits" ]; then
  record_candidate open "candidate branch-content TLS verification bypass markers require review before a public repo flip"
  printf '%s\n' "$tls_bypass_hits" | sed 's/^/       tls-bypass: /'
  action "remove TLS verification bypasses, restrict them to non-production test fixtures, or document explicit public-safe acceptance."
else
  record_candidate closed "no candidate branch-content TLS verification bypass markers"
fi

if [ ! -f "$repo_root/LICENSE" ]; then
  record_candidate open "candidate branch-content LICENSE file is missing"
  action "add or restore the root MIT LICENSE before public visibility."
elif ! grep -qF "MIT License" "$repo_root/LICENSE"; then
  record_candidate open "candidate branch-content LICENSE is present but does not declare MIT License"
  action "restore the approved MIT LICENSE text or document legal approval for a license change before public visibility."
else
  record_candidate closed "candidate branch-content LICENSE declares MIT License"
fi

required_gitignore_patterns=(
  ".claude/"
  ".serena/"
  ".remember/"
  ".planning/"
  ".bench/"
  "coverage.out"
  "dist/"
  "staging/"
  ".DS_Store"
  "Thumbs.db"
  "desktop.ini"
  "CLAUDE.md"
)

if [ ! -f "$repo_root/.gitignore" ]; then
  record_candidate open "candidate branch-content .gitignore file is missing"
  action "restore .gitignore before public visibility so local assistant state and generated artifacts stay out of the branch."
else
  missing_gitignore_patterns=""
  for pattern in "${required_gitignore_patterns[@]}"; do
    if ! grep -qxF -- "$pattern" "$repo_root/.gitignore"; then
      missing_gitignore_patterns="${missing_gitignore_patterns}${pattern}"$'\n'
    fi
  done
  if [ -n "$missing_gitignore_patterns" ]; then
    record_candidate open "candidate branch-content .gitignore coverage is missing required entries"
    printf '%s' "$missing_gitignore_patterns" | sed '/^$/d; s/^/       gitignore-missing: /'
    action "restore .gitignore coverage for workstation-private state and generated artifacts before public visibility."
  else
    record_candidate closed "candidate branch-content .gitignore covers workstation-private and generated artifact paths"
  fi
fi

if [ ! -f "$repo_root/.gitleaks.toml" ]; then
  record_candidate open "candidate branch-content .gitleaks.toml file is missing"
  action "restore .gitleaks.toml before public visibility so local and CI secret scans use the same allowlist policy."
else
  missing_allowlist_descriptions="$(awk '
    function finish_block() {
      if (in_block && !has_description) {
        printf "allowlist-%d\n", block_number
      }
    }
    /^\[\[allowlists\]\][[:space:]]*$/ {
      finish_block()
      in_block = 1
      has_description = 0
      block_number++
      next
    }
    /^\[\[/ {
      finish_block()
      in_block = 0
      has_description = 0
      next
    }
    in_block && /^[[:space:]]*description[[:space:]]*=[[:space:]]*"[^"]+"/ {
      has_description = 1
      next
    }
    END {
      finish_block()
    }
  ' "$repo_root/.gitleaks.toml")"
  if [ -n "$missing_allowlist_descriptions" ]; then
    record_candidate open "candidate branch-content .gitleaks.toml allowlists are missing descriptions"
    printf '%s\n' "$missing_allowlist_descriptions" | sed '/^$/d; s/^/       gitleaks-allowlist: /'
    action "add one-line descriptions to every gitleaks allowlist block before public visibility."
  else
    record_candidate closed "candidate branch-content .gitleaks.toml allowlists have descriptions"
  fi
fi

candidate_claude_files="$(git -C "$repo_root" ls-files --cached --others --exclude-standard |
  while IFS= read -r rel; do
    case "$rel" in
      CLAUDE.md|*/CLAUDE.md) printf '%s\n' "$rel" ;;
    esac
  done | sort)"
if [ -n "$candidate_claude_files" ]; then
  record_candidate open "candidate branch-content CLAUDE.md workstation context files require review before a public repo flip"
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    printf '       claude-context: %s [%s]\n' "$rel" "$(path_state "$rel")"
  done <<< "$candidate_claude_files"
  action "remove workstation-private CLAUDE.md files from tracked/unignored candidate content before public visibility."
else
  record_candidate closed "no candidate branch-content CLAUDE.md workstation context files"
fi

live_secret_pattern='CLOCKIFY_LIVE_(API_KEY|WORKSPACE_ID)[[:space:]]*[:=]'
candidate_live_secret_hits="$(git -C "$repo_root" ls-files -z --cached --others --exclude-standard |
  while IFS= read -r -d '' rel; do
    [ -f "$repo_root/$rel" ] || continue
    grep -n -I -E "$live_secret_pattern" -- "$repo_root/$rel" 2>/dev/null |
      while IFS=: read -r _file line_no _rest; do
        [ -n "$line_no" ] || continue
        printf '%s:%s\n' "$rel" "$line_no"
      done
  done || true)"
if [ -n "$candidate_live_secret_hits" ]; then
  record_candidate open "candidate branch-content live Clockify secret env assignments require review before a public repo flip"
  printf '%s\n' "$candidate_live_secret_hits" | sed 's/^/       live-secret-assignment: /'
  action "remove live Clockify secret/workspace assignments and keep values in CI secrets or local ignored env files."
else
  record_candidate closed "no candidate branch-content live Clockify secret env assignments"
fi

candidate_env_files="$(git -C "$repo_root" ls-files --cached --others --exclude-standard |
  while IFS= read -r rel; do
    [ -f "$repo_root/$rel" ] || continue
    case "$(basename "$rel")" in
      .env*) printf '%s\n' "$rel" ;;
    esac
  done | sort)"
if [ -n "$candidate_env_files" ]; then
  record_candidate open "candidate branch-content env-like files require review before a public repo flip"
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    printf '       env-candidate: %s [%s]\n' "$rel" "$(path_state "$rel")"
  done <<< "$candidate_env_files"
  action "delete, rename, or move candidate env-like files out of the branch before public visibility."
else
  record_candidate closed "no candidate branch-content env-like files"
fi

env_files="$(find "$repo_root" -type f -name '.env*' \
  -not -path "$repo_root/.git/*" \
  -not -path "$repo_root/node_modules/*" \
  -print | sed "s#^$repo_root/##" | sort)"
if [ -n "$env_files" ]; then
  local_artifact_review="$repo_root/docs/release/local-artifact-review.md"
  undocumented_env_files=""
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    state="$(path_state "$rel")"
    if [ "$state" = "tracked" ] || ! path_is_documented "$local_artifact_review" "$rel"; then
      undocumented_env_files="${undocumented_env_files}${rel}"$'\n'
    fi
  done <<< "$env_files"
  if [ -z "$undocumented_env_files" ]; then
    record_local closed "env-like files are documented ignored local artifacts"
    while IFS= read -r rel; do
      [ -n "$rel" ] || continue
      printf '       env-local-reviewed: %s [%s]\n' "$rel" "$(path_state "$rel")"
    done <<< "$env_files"
  else
    record_local open "env-like files require review before a public repo flip"
    while IFS= read -r rel; do
      [ -n "$rel" ] || continue
      printf '       env-file: %s [%s]\n' "$rel" "$(path_state "$rel")"
    done <<< "$env_files"
    action "review ignored/untracked env-like files; remove them with approved cleanup such as make clean-deep CONFIRM=1 when appropriate, or document explicit acceptance in docs/release/local-artifact-review.md."
  fi
else
  record_local closed "no env-like files found outside .git and node_modules"
fi

todo_hits="$(git -C "$repo_root" grep -n -I -E 'TODO.*(internal|private)' -- '*.go' '*.md' 2>/dev/null |
  awk -F: '{ print $1 ":" $2 }' || true)"
if [ -n "$todo_hits" ]; then
  record_candidate open "tracked TODO lines mentioning internal/private context require review before a public repo flip"
  printf '%s\n' "$todo_hits" | sed 's/^/       todo-ref: /'
  action "rewrite, remove, or explicitly approve public-safe tracked TODO wording before public visibility."
else
  record_candidate closed "no tracked Go/Markdown TODO lines mention internal/private context"
fi

operator_marker_hits="$(git -C "$repo_root" grep -n -I -E -w 'TODO|FIXME|XXX|HACK|hack' -- 'internal/**/*.go' 'cmd/**/*.go' ':(exclude)**/*_test.go' 2>/dev/null |
  awk -F: '{ print $1 ":" $2 }' || true)"
if [ -n "$operator_marker_hits" ]; then
  record_candidate open "non-test internal/cmd Go task markers require review before launch"
  printf '%s\n' "$operator_marker_hits" | sed 's/^/       operator-marker: /'
  action "resolve launch-facing task markers in operator-facing code, or document why each is public-safe."
else
  record_candidate closed "no non-test internal/cmd Go task markers in operator-facing code"
fi

message_hits="$(git -C "$repo_root" log --all --format='%h%x09%s' -200 2>/dev/null |
  grep -Ei 'secret|token|password|key=' || true)"
if [ -n "$message_hits" ]; then
  history_review_file="$repo_root/docs/release/public-history-review.md"
  unreviewed_messages=""
  while IFS=$'\t' read -r sha _subject; do
    [ -n "$sha" ] || continue
    if [ ! -f "$history_review_file" ] || ! grep -qF -- "$sha" "$history_review_file"; then
      unreviewed_messages="${unreviewed_messages}${sha}"$'\n'
    fi
  done <<< "$message_hits"

  if [ -n "$unreviewed_messages" ]; then
    record_history open "recent commit messages match public-content sensitive-word review patterns"
    while IFS=$'\t' read -r sha _subject; do
      [ -n "$sha" ] || continue
      printf '       commit: %s (message matched sensitive-word pattern)\n' "$sha"
    done <<< "$message_hits"
    action "review listed commit subjects and document acceptance in docs/release/public-history-review.md, or rewrite history only with maintainer approval."
  else
    record_history closed "recent commit message sensitive-word matches are documented in docs/release/public-history-review.md"
    while IFS=$'\t' read -r sha _subject; do
      [ -n "$sha" ] || continue
      printf '       history-reviewed: %s\n' "$sha"
    done <<< "$message_hits"
  fi
else
  record_history closed "recent commit messages do not match sensitive-word review patterns"
fi

apet_refs="$(git -C "$repo_root" grep -l -I 'apet97' -- . 2>/dev/null || true)"
apet_count="$(printf '%s\n' "$apet_refs" | sed '/^$/d' | wc -l | tr -d '[:space:]')"
printf '[info] tracked apet97 references: %s; review if moving from personal account to CAKE.com org\n' "$apet_count"

printf '\nSummary: %d open, %d unknown\n' "$open_count" "$unknown_count"
printf 'Candidate branch file content: %d open, %d unknown\n' "$candidate_open_count" "$candidate_unknown_count"
printf 'Public history review: %d open, %d unknown\n' "$history_open_count" "$history_unknown_count"
printf 'Local artifact/full-tree review: %d open, %d unknown\n' "$local_open_count" "$local_unknown_count"

if [ "$fail_open" = "1" ] && { [ "$open_count" -ne 0 ] || [ "$unknown_count" -ne 0 ]; }; then
  exit 1
fi
