#!/usr/bin/env bash
#
# Regression tests for check-public-content-audit.sh. gitleaks is stubbed
# so script-tests stay offline and do not depend on a local install.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-public-content-audit.sh"

tests_run=0
tests_failed=0

pass() {
  printf 'PASS: %s\n' "$1"
}

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  tests_failed=$((tests_failed + 1))
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if grep -qF -- "$needle" <<< "$haystack"; then
    pass "$label"
  else
    fail "$label"
    printf '  missing: %s\n' "$needle" >&2
  fi
}

write_stub_gitleaks() {
  local dir="$1"
  mkdir -p "$dir/bin"

  cat > "$dir/bin/gitleaks" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

report_path=""
source_path=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --source)
      source_path="$2"
      shift 2
      ;;
    --report-path)
      report_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ -z "$report_path" ]; then
  printf 'missing --report-path\n' >&2
  exit 9
fi

if [[ "$source_path" == *public-content-candidate* ]]; then
  should_leak="${TEST_PUBLIC_AUDIT_CANDIDATE_LEAK:-0}"
else
  should_leak="${TEST_PUBLIC_AUDIT_LEAK:-0}"
fi

if [ "$should_leak" = "1" ]; then
  leak_file="${TEST_PUBLIC_AUDIT_LEAK_FILE:-README.md}"
  cat > "$report_path" <<JSON
[
  {
    "RuleID": "generic-api-key",
    "File": "${leak_file}",
    "StartLine": 1,
    "Description": "Detected a Generic API Key"
  }
]
JSON
  exit 1
fi

printf '[]\n' > "$report_path"
exit 0
EOF

  chmod +x "$dir/bin/gitleaks"
}

make_repo() {
  local dir="$1"
  git -C "$dir" init -q
  git -C "$dir" config user.email "test@example.invalid"
  git -C "$dir" config user.name "Test User"
  cat > "$dir/.gitleaks.toml" <<'EOF'
[extend]
useDefault = true

[[allowlists]]
description = "Fixture allowlist"
paths = [
    '''^testdata/.*''',
]
EOF
  cat > "$dir/.gitignore" <<'EOF'
.claude/
.serena/
.remember/
.planning/
.bench/
coverage.out
dist/
staging/
.DS_Store
Thumbs.db
desktop.ini
CLAUDE.md
.local/
EOF
  printf '# Fixture\n' > "$dir/README.md"
  printf 'MIT License\n\nCopyright (c) 2026 fixture\n' > "$dir/LICENSE"
  git -C "$dir" add .gitleaks.toml .gitignore LICENSE README.md
  git -C "$dir" commit -q -m "initial fixture"
}

write_history_review() {
  local dir="$1"
  shift
  mkdir -p "$dir/docs/release"
  {
    printf '# Public History Review\n\n'
    for sha in "$@"; do
      printf -- '- `%s` accepted false positive.\n' "$sha"
    done
  } > "$dir/docs/release/public-history-review.md"
}

tests_run=$((tests_run + 1))
plan_output="$(bash "$script" --plan --repo-root "$repo_root")"
assert_contains "$plan_output" "Public content audit plan" "plan prints header"
assert_contains "$plan_output" "gitleaks detect --no-git" "plan includes gitleaks"
assert_contains "$plan_output" "tracked plus unignored files" "plan includes candidate content scan"
assert_contains "$plan_output" "git grep tracked files" "plan includes tracked grep"
assert_contains "$plan_output" "TLS verification bypass markers" "plan includes TLS bypass marker check"
assert_contains "$plan_output" "MIT License" "plan includes MIT license check"
assert_contains "$plan_output" ".gitignore coverage" "plan includes gitignore coverage check"
assert_contains "$plan_output" ".gitleaks.toml allowlist entries" "plan includes gitleaks allowlist description check"
assert_contains "$plan_output" "CLAUDE.md files" "plan includes CLAUDE.md workstation context check"
assert_contains "$plan_output" "CLOCKIFY_LIVE_* assignments" "plan includes live Clockify secret assignment check"
assert_contains "$plan_output" "candidate .env* files" "plan includes candidate env-like file check"
assert_contains "$plan_output" "find" "plan includes env-like file check"
assert_contains "$plan_output" "TODO.*(internal|private)" "plan includes internal/private TODO check"
assert_contains "$plan_output" "non-test internal/cmd Go task markers" "plan includes operator marker check"
assert_contains "$plan_output" "prints maintainer actions" "plan documents action hints"

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-stub.XXXXXX")"
repo_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-repo.XXXXXX")"
trap 'rm -rf "$stub_dir" "$repo_dir"' EXIT
write_stub_gitleaks "$stub_dir"
make_repo "$repo_dir"
clean_output="$(PATH="$stub_dir/bin:$PATH" bash "$script" --repo-root "$repo_dir" --fail-open)"
assert_contains "$clean_output" "[closed] gitleaks candidate branch-content scan returned no findings" "clean fixture closes candidate gitleaks"
assert_contains "$clean_output" "[closed] gitleaks working-tree scan returned no findings" "clean fixture closes gitleaks"
assert_contains "$clean_output" "[closed] no candidate branch-content TLS verification bypass markers" "clean fixture closes TLS bypass check"
assert_contains "$clean_output" "[closed] candidate branch-content LICENSE declares MIT License" "clean fixture closes MIT license check"
assert_contains "$clean_output" "[closed] candidate branch-content .gitignore covers workstation-private and generated artifact paths" "clean fixture closes gitignore coverage check"
assert_contains "$clean_output" "[closed] candidate branch-content .gitleaks.toml allowlists have descriptions" "clean fixture closes gitleaks allowlist description check"
assert_contains "$clean_output" "[closed] no candidate branch-content CLAUDE.md workstation context files" "clean fixture closes CLAUDE.md workstation context check"
assert_contains "$clean_output" "[closed] no candidate branch-content live Clockify secret env assignments" "clean fixture closes live Clockify secret assignment check"
assert_contains "$clean_output" "[closed] no candidate branch-content env-like files" "clean fixture closes candidate env check"
assert_contains "$clean_output" "[closed] no tracked Go/Markdown TODO lines mention internal/private context" "clean fixture closes TODO check"
assert_contains "$clean_output" "[closed] no non-test internal/cmd Go task markers in operator-facing code" "clean fixture closes operator marker check"
assert_contains "$clean_output" "Summary: 0 open, 0 unknown" "clean fixture exits with no open items"
assert_contains "$clean_output" "Candidate branch file content: 0 open, 0 unknown" "clean fixture reports candidate scope"
assert_contains "$clean_output" "Public history review: 0 open, 0 unknown" "clean fixture reports history scope"
assert_contains "$clean_output" "Local artifact/full-tree review: 0 open, 0 unknown" "clean fixture reports local scope"
rm -rf "$stub_dir" "$repo_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-self-stub.XXXXXX")"
repo_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-self-repo.XXXXXX")"
trap 'rm -rf "$stub_dir" "$repo_dir"' EXIT
write_stub_gitleaks "$stub_dir"
make_repo "$repo_dir"
mkdir -p "$repo_dir/scripts"
cat > "$repo_dir/scripts/check-public-content-audit.sh" <<'EOF'
required_gitignore_patterns=(
  ".claude/"
  ".serena/"
  ".remember/"
  ".planning/"
)
EOF
cat > "$repo_dir/scripts/test-check-public-content-audit.sh" <<'EOF'
cat > "$dir/.gitignore" <<'INNER'
.claude/
.serena/
.remember/
.planning/
INNER
EOF
git -C "$repo_dir" add scripts/check-public-content-audit.sh scripts/test-check-public-content-audit.sh
git -C "$repo_dir" commit -q -m "test: add audit script fixtures"
self_output="$(PATH="$stub_dir/bin:$PATH" bash "$script" --repo-root "$repo_dir" --fail-open)"
assert_contains "$self_output" "[closed] no tracked personal/scratch references outside .gitignore/.gitleaks.toml" "audit script fixture references do not trip scratch scan"
assert_contains "$self_output" "Summary: 0 open, 0 unknown" "audit script fixture references keep audit clean"
rm -rf "$stub_dir" "$repo_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-open-stub.XXXXXX")"
repo_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-open-repo.XXXXXX")"
trap 'rm -rf "$stub_dir" "$repo_dir"' EXIT
write_stub_gitleaks "$stub_dir"
make_repo "$repo_dir"
scratch_marker="pet""kovic"
printf 'Use %s notes locally.\n' "$scratch_marker" >> "$repo_dir/README.md"
printf '%s=true\n' "Insecure""SkipVerify" >> "$repo_dir/README.md"
printf 'Proprietary\n' > "$repo_dir/LICENSE"
perl -0pi -e 's/^\.remember\/\n//m' "$repo_dir/.gitignore"
cat >> "$repo_dir/.gitleaks.toml" <<'EOF'

[[allowlists]]
paths = [
    '''^unjustified/.*''',
]
EOF
printf 'local machine-only context\n' > "$repo_dir/CLAUDE.md"
live_workspace_var="CLOCKIFY_LIVE_""WORKSPACE_ID"
printf '%s=workspace-from-ci-secret-only\n' "$live_workspace_var" > "$repo_dir/live-secret.txt"
printf 'TODO internal launch note.\n' >> "$repo_dir/README.md"
mkdir -p "$repo_dir/internal/operator"
printf 'package operator\n\n// TODO resolve before launch.\nconst Ready = true\n' > "$repo_dir/internal/operator/marker.go"
printf 'TOKEN=example\n' > "$repo_dir/.env.local"
git -C "$repo_dir" add .gitleaks.toml .gitignore README.md .env.local internal/operator/marker.go live-secret.txt
git -C "$repo_dir" add -f CLAUDE.md
git -C "$repo_dir" commit -q -m "docs: token wording"
open_output=""
if open_output="$(TEST_PUBLIC_AUDIT_LEAK=1 TEST_PUBLIC_AUDIT_CANDIDATE_LEAK=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo-root "$repo_dir" --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when public content checks are open"
else
  if grep -q "gitleaks candidate branch-content scan found 1 redacted findings" <<< "$open_output" &&
     grep -q "gitleaks working-tree scan found 1 redacted findings" <<< "$open_output" &&
     grep -q "tracked personal/scratch references require review" <<< "$open_output" &&
     grep -q "candidate branch-content TLS verification bypass markers require review before a public repo flip" <<< "$open_output" &&
     grep -q "candidate branch-content LICENSE is present but does not declare MIT License" <<< "$open_output" &&
     grep -q "candidate branch-content .gitignore coverage is missing required entries" <<< "$open_output" &&
     grep -q "candidate branch-content .gitleaks.toml allowlists are missing descriptions" <<< "$open_output" &&
     grep -q "candidate branch-content CLAUDE.md workstation context files require review before a public repo flip" <<< "$open_output" &&
     grep -q "candidate branch-content live Clockify secret env assignments require review before a public repo flip" <<< "$open_output" &&
     grep -q "candidate branch-content env-like files require review" <<< "$open_output" &&
     grep -q "env-like files require review" <<< "$open_output" &&
     grep -q "tracked TODO lines mentioning internal/private context require review" <<< "$open_output" &&
     grep -q "non-test internal/cmd Go task markers require review before launch" <<< "$open_output" &&
     grep -q "recent commit messages match public-content sensitive-word review patterns" <<< "$open_output" &&
     grep -q "action: remove, redact, or explicitly accept candidate branch findings before a public visibility flip." <<< "$open_output" &&
     grep -q "action: clean ignored/local artifacts with maintainer approval, for example make clean-deep CONFIRM=1, or document explicit acceptance in docs/release/local-artifact-review.md." <<< "$open_output" &&
     grep -q "action: delete, rename, or move candidate env-like files out of the branch before public visibility." <<< "$open_output" &&
     grep -q "action: review listed commit subjects and document acceptance in docs/release/public-history-review.md, or rewrite history only with maintainer approval." <<< "$open_output" &&
     grep -q "Candidate branch file content: 11 open, 0 unknown" <<< "$open_output" &&
     grep -q "Public history review: 1 open, 0 unknown" <<< "$open_output" &&
     grep -q "Local artifact/full-tree review: 2 open, 0 unknown" <<< "$open_output"; then
    pass "fail-open returns non-zero when public content checks are open"
  else
    fail "fail-open reports public-content details"
    printf '%s\n' "$open_output" >&2
  fi
fi
rm -rf "$stub_dir" "$repo_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-local-stub.XXXXXX")"
repo_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-local-repo.XXXXXX")"
trap 'rm -rf "$stub_dir" "$repo_dir"' EXIT
write_stub_gitleaks "$stub_dir"
make_repo "$repo_dir"
mkdir -p "$repo_dir/.local" "$repo_dir/docs/release"
printf 'LOCAL_ONLY_KEY=fixture\n' > "$repo_dir/.local/http-test.env"
printf 'LOCAL_ENV=fixture\n' > "$repo_dir/.local/.env.local"
cat > "$repo_dir/docs/release/local-artifact-review.md" <<'EOF'
# Local Artifact Review

| Path | State | Disposition |
| --- | --- | --- |
| `.local/http-test.env` | ignored | Fixture local env file. |
| `.local/.env.local` | ignored | Fixture env-like local file. |
EOF
git -C "$repo_dir" add docs/release/local-artifact-review.md
git -C "$repo_dir" commit -q -m "docs: local artifact review"
local_output=""
if ! local_output="$(TEST_PUBLIC_AUDIT_LEAK=1 TEST_PUBLIC_AUDIT_LEAK_FILE="$repo_dir/.local/http-test.env" PATH="$stub_dir/bin:$PATH" bash "$script" --repo-root "$repo_dir" --fail-open 2>&1)"; then
  fail "documented ignored local artifacts close local bucket"
  printf '%s\n' "$local_output" >&2
elif grep -q "gitleaks working-tree findings are documented ignored local artifacts" <<< "$local_output" &&
   grep -q "env-like files are documented ignored local artifacts" <<< "$local_output" &&
   grep -q "Local artifact/full-tree review: 0 open, 0 unknown" <<< "$local_output" &&
   grep -q "Summary: 0 open, 0 unknown" <<< "$local_output"; then
  pass "documented ignored local artifacts close local bucket"
else
  fail "documented ignored local artifacts close local bucket"
  printf '%s\n' "$local_output" >&2
fi
rm -rf "$stub_dir" "$repo_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-history-stub.XXXXXX")"
repo_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-public-content-audit-history-repo.XXXXXX")"
trap 'rm -rf "$stub_dir" "$repo_dir"' EXIT
write_stub_gitleaks "$stub_dir"
make_repo "$repo_dir"
printf 'history review fixture\n' >> "$repo_dir/README.md"
git -C "$repo_dir" add README.md
git -C "$repo_dir" commit -q -m "docs: token wording"
history_sha="$(git -C "$repo_dir" rev-parse --short HEAD)"
write_history_review "$repo_dir" "$history_sha"
git -C "$repo_dir" add docs/release/public-history-review.md
git -C "$repo_dir" commit -q -m "docs: record public history review"
history_output="$(PATH="$stub_dir/bin:$PATH" bash "$script" --repo-root "$repo_dir" --fail-open)"
if grep -q "recent commit message sensitive-word matches are documented" <<< "$history_output" &&
   grep -q "Public history review: 0 open, 0 unknown" <<< "$history_output" &&
   grep -q "Summary: 0 open, 0 unknown" <<< "$history_output"; then
  pass "documented public-history matches close history bucket"
else
  fail "documented public-history matches close history bucket"
  printf '%s\n' "$history_output" >&2
fi
rm -rf "$stub_dir" "$repo_dir"
trap - EXIT

if [ "$tests_failed" -ne 0 ]; then
  printf '\ncheck-public-content-audit tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
  exit 1
fi

printf '\ncheck-public-content-audit tests: %d run, 0 failed\n' "$tests_run"
