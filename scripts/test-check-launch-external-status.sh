#!/usr/bin/env bash
#
# Regression tests for check-launch-external-status.sh. Live GitHub/npm
# calls are covered with PATH stubs so script-tests stay offline and
# deterministic.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-launch-external-status.sh"

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

write_stub_tools() {
  local dir="$1"
  mkdir -p "$dir/bin"

  cat > "$dir/bin/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  "diff --quiet"|"diff --cached --quiet")
    if [ "${TEST_EXTERNAL_STATUS_DIRTY:-0}" = "1" ]; then
      exit 1
    fi
    exit 0
    ;;
  "rev-parse HEAD")
    printf 'deadbeef\n'
    ;;
  "for-each-ref --format=%(refname:short) refs/heads")
    if [ "${TEST_EXTERNAL_STATUS_BRANCHES:-0}" = "1" ]; then
      printf 'main\nwave-a\nfwbranch\n'
    else
      printf 'main\n'
    fi
    ;;
  "rev-parse --short wave-a")
    printf 'aaaaaaa\n'
    ;;
  "rev-parse --short fwbranch")
    printf 'bbbbbbb\n'
    ;;
  "rev-list --count origin/main..wave-a")
    printf '2\n'
    ;;
  "rev-list --count wave-a..origin/main")
    printf '5\n'
    ;;
  "rev-list --count origin/main..fwbranch")
    printf '0\n'
    ;;
  "rev-list --count fwbranch..origin/main")
    printf '11\n'
    ;;
  *)
    printf 'unexpected git args: %s\n' "$*" >&2
    exit 9
    ;;
esac
EOF

  cat > "$dir/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
args="$*"

if [[ "$args" == run\ list* ]]; then
  case "$args" in
    *"--workflow live-contract.yml"*)
      if [ "${TEST_EXTERNAL_STATUS_RUNS_OPEN:-0}" = "1" ]; then
        printf '101\tcompleted\tsuccess\tdeadbeef\t2026-05-09T01:00:00Z\thttps://example.test/live/101\n'
        printf '100\tcompleted\tsuccess\tcafebabe\t2026-05-08T01:00:00Z\thttps://example.test/live/100\n'
      else
        printf '101\tcompleted\tsuccess\tdeadbeef\t2026-05-09T01:00:00Z\thttps://example.test/live/101\n'
        printf '100\tcompleted\tsuccess\tdeadbeef\t2026-05-08T01:00:00Z\thttps://example.test/live/100\n'
      fi
      ;;
    *"--workflow mutation.yml"*)
      if [ "${TEST_EXTERNAL_STATUS_RUNS_OPEN:-0}" = "1" ]; then
        printf '201\tcompleted\tcancelled\tcafebabe\t2026-05-09T02:00:00Z\thttps://example.test/mutation/201\n'
      else
        printf '201\tcompleted\tsuccess\tdeadbeef\t2026-05-09T02:00:00Z\thttps://example.test/mutation/201\n'
      fi
      ;;
	    *"--workflow codeql.yml"*)
	      if [ "${TEST_EXTERNAL_STATUS_OPEN:-0}" = "1" ]; then
	        printf 'HTTP 404: workflow codeql.yml not found on the default branch\n' >&2
	        exit 1
	      fi
	      if [ "${TEST_EXTERNAL_STATUS_WORKFLOW_SHA_OPEN:-0}" = "1" ]; then
	        printf '301\tpush\tcompleted\tsuccess\tcafebabe\t2026-05-09T03:00:00Z\thttps://example.test/codeql/301\n'
	        exit 0
	      fi
	      printf '301\tpush\tcompleted\tsuccess\tdeadbeef\t2026-05-09T03:00:00Z\thttps://example.test/codeql/301\n'
	      ;;
	    *"--workflow dependency-review.yml"*)
	      if [ "${TEST_EXTERNAL_STATUS_WORKFLOW_EVENT_OPEN:-0}" = "1" ]; then
	        printf '302\tpull_request\tcompleted\tsuccess\tdeadbeef\t2026-05-09T03:05:00Z\thttps://example.test/dependency/302\n'
	        exit 0
	      fi
	      printf '302\tpush\tcompleted\tsuccess\tdeadbeef\t2026-05-09T03:05:00Z\thttps://example.test/dependency/302\n'
	      ;;
    *"--workflow semgrep.yml"*)
      printf '303\tpush\tcompleted\tsuccess\tdeadbeef\t2026-05-09T03:10:00Z\thttps://example.test/semgrep/303\n'
      ;;
    *)
      printf 'unexpected gh run list args: %s\n' "$args" >&2
      exit 9
      ;;
  esac
  exit 0
fi

if [[ "$args" == run\ view* ]]; then
  case "$args" in
    "run view 100 --repo example/repo --log"|"run view 101 --repo example/repo --log")
      if [ "${TEST_EXTERNAL_STATUS_LIVE_LOG_MISSING:-0}" = "1" ]; then
        printf '=== RUN   TestE2EMutating\n'
        printf '=== RUN   TestLiveReadSideSchemaDiff\n'
      else
        printf '=== RUN   TestE2EMutating\n'
        printf '=== RUN   TestLiveCreateUpdateDeleteEntryAuditPhases\n'
        printf '=== RUN   TestLiveReadSideSchemaDiff\n'
      fi
      ;;
    run\ view\ 201\ --repo\ example/repo\ --json\ jobs\ --jq*)
      printf 'Mutation (internal/tools) (completed/cancelled)\n'
      ;;
    *)
      printf 'unexpected gh run view args: %s\n' "$args" >&2
      exit 9
      ;;
  esac
  exit 0
fi

if [[ "$args" == variable\ list* ]]; then
  if [ "${TEST_EXTERNAL_STATUS_AUDIT_VAR_OPEN:-0}" = "1" ]; then
    exit 0
  fi
  if [ "${TEST_EXTERNAL_STATUS_AUDIT_VAR_FALSE:-0}" = "1" ]; then
    printf 'CLOCKIFY_LIVE_AUDIT_REQUIRED\tfalse\t2026-05-09T00:00:00Z\n'
  else
    printf 'CLOCKIFY_LIVE_AUDIT_REQUIRED\ttrue\t2026-05-09T00:00:00Z\n'
  fi
  exit 0
fi

if [[ "$args" == repo\ view* ]]; then
  if [ "${TEST_EXTERNAL_STATUS_REPO_DESC_OPEN:-0}" = "1" ]; then
    printf 'Model Context Protocol server for Clockify. 124 tools, four policy modes.\tfalse\tPUBLIC\thttps://example.test/repo\n'
  else
    printf '128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.\tfalse\tPUBLIC\thttps://example.test/repo\n'
  fi
  exit 0
fi

if [[ "$args" == issue\ view\ 28* ]]; then
  if [ "${TEST_EXTERNAL_STATUS_ISSUE_OPEN:-0}" = "1" ]; then
    printf 'OPEN\tPostgres-backed shared-service integration test\thttps://example.test/issues/28\t2026-05-09T04:00:00Z\n'
  else
    printf 'CLOSED\tPostgres-backed shared-service integration test\thttps://example.test/issues/28\t2026-05-09T04:00:00Z\n'
  fi
  exit 0
fi

if [[ "$args" == pr\ list* ]]; then
  if [ "${TEST_EXTERNAL_STATUS_STALE_PRS:-0}" = "1" ]; then
    printf '42\tOld launch PR\thttps://example.test/pull/42\t2000-01-01T00:00:00Z\treview-needed\n'
    printf '43\tBlocked old PR\thttps://example.test/pull/43\t2000-01-01T00:00:00Z\tblocked\n'
  else
    printf '43\tBlocked old PR\thttps://example.test/pull/43\t2000-01-01T00:00:00Z\tblocked\n'
  fi
  exit 0
fi

if [[ "$args" == api\ repos/example/repo/branches/main/protection* ]]; then
  if [ "${TEST_EXTERNAL_STATUS_PROTECTION_OPEN:-0}" = "1" ]; then
    printf 'HTTP 404: Upgrade to GitHub Pro or make this repository public\n' >&2
    exit 1
  fi
  if [ "${TEST_EXTERNAL_STATUS_PROTECTION_MISSING_CHECK:-0}" = "1" ]; then
    printf 'true\tFormat,Doctor strict smoke\ttrue\ttrue\n'
  elif [ "${TEST_EXTERNAL_STATUS_PROTECTION_CHECK_OBJECTS:-0}" = "1" ]; then
    printf 'true\tDoctor Postgres backend,Doctor strict smoke,Shared-service Postgres E2E\ttrue\ttrue\n'
  else
    printf 'true\tDoctor strict smoke,Doctor Postgres backend,Shared-service Postgres E2E\ttrue\ttrue\n'
  fi
  exit 0
fi

printf 'unexpected gh args: %s\n' "$args" >&2
exit 9
EOF

  cat > "$dir/bin/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "$*" = "view @apet97/clockify-mcp-go version dist-tags --json" ]; then
  case "${TEST_EXTERNAL_STATUS_NPM_FIXTURE:-stable}" in
    rc-published)
      # Mirror the post-rc.3 npm registry shape: latest unchanged, rc bumped.
      printf '%s\n' '{"version":"1.2.0","dist-tags":{"latest":"1.2.0","rc":"1.2.1-rc.3"}}'
      ;;
    rc-missing)
      # Prerelease never published; rc dist-tag absent.
      printf '%s\n' '{"version":"1.2.0","dist-tags":{"latest":"1.2.0"}}'
      ;;
    beta-published)
      printf '%s\n' '{"version":"1.2.0","dist-tags":{"latest":"1.2.0","beta":"2.0.0-beta.1"}}'
      ;;
    stable-stale)
      # Stable expected but registry latest is older.
      printf '%s\n' '{"version":"1.2.0","dist-tags":{"latest":"1.2.0"}}'
      ;;
    stable|*)
      printf '%s\n' '{"version":"1.2.0","dist-tags":{"latest":"1.2.0"}}'
      ;;
  esac
  exit 0
fi
printf 'unexpected npm args: %s\n' "$*" >&2
exit 9
EOF

  chmod +x "$dir/bin/git" "$dir/bin/gh" "$dir/bin/npm"
}

tests_run=$((tests_run + 1))
plan_output="$(bash "$script" --plan --repo example/repo --candidate-sha deadbeef)"
assert_contains "$plan_output" "Launch external status plan for example/repo" "plan names repo"
assert_contains "$plan_output" "deadbeef" "plan names candidate sha"
assert_contains "$plan_output" "Expected npm wrapper version:" "plan names expected npm version"
assert_contains "$plan_output" "git for-each-ref" "plan includes local branch inventory"
assert_contains "$plan_output" "git rev-list --count origin/main" "plan includes branch ahead/behind inventory"
assert_contains "$plan_output" "gh variable list --repo example/repo" "plan includes live audit-required repo variable check"
assert_contains "$plan_output" "CLOCKIFY_LIVE_AUDIT_REQUIRED is set to true" "plan names live audit-required expected value"
assert_contains "$plan_output" "live-contract.yml" "plan includes live-contract"
assert_contains "$plan_output" "gh run view <live-contract-run-id>" "plan includes live-contract log inspection"
assert_contains "$plan_output" "TestLiveCreateUpdateDeleteEntryAuditPhases" "plan names audit-tier log marker"
assert_contains "$plan_output" "mutation.yml" "plan includes mutation"
assert_contains "$plan_output" "codeql.yml" "plan includes CodeQL workflow"
assert_contains "$plan_output" "dependency-review.yml" "plan includes dependency-review workflow"
assert_contains "$plan_output" "semgrep.yml" "plan includes Semgrep workflow"
assert_contains "$plan_output" "branches/main/protection" "plan includes branch protection API"
assert_contains "$plan_output" "Doctor strict smoke, Doctor Postgres backend, and" "plan names D9 required branch-protection checks"
assert_contains "$plan_output" "Shared-service Postgres E2E are required checks" "plan names Group 2 branch-protection check"
assert_contains "$plan_output" "gh repo view example/repo" "plan includes repo description"
assert_contains "$plan_output" "128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases." "plan includes exact repo description target"
assert_contains "$plan_output" "gh issue view 28" "plan includes issue 28"
assert_contains "$plan_output" "gh pr list --repo example/repo --state open" "plan includes stale PR check"
assert_contains "$plan_output" "npm view @apet97/clockify-mcp-go" "plan includes npm check"
assert_contains "$plan_output" "compare npm version/dist-tags.latest" "plan includes npm expected-version proof"
assert_contains "$plan_output" "does not mutate GitHub" "plan states read-only posture"
assert_contains "$plan_output" "prints maintainer actions" "plan states action-hint posture"

tests_run=$((tests_run + 1))
help_output="$(bash "$script" --help)"
assert_contains "$help_output" "--fail-open" "help documents fail-open"
assert_contains "$help_output" "--candidate-sha" "help documents candidate sha"
assert_contains "$help_output" "--expected-npm-version" "help documents expected npm version"
assert_contains "$help_output" "--plan" "help documents plan"

tests_run=$((tests_run + 1))
invalid_output=""
if invalid_output="$(bash "$script" --bogus 2>&1)"; then
  fail "invalid option fails"
else
  if grep -q "unknown option: --bogus" <<< "$invalid_output"; then
    pass "invalid option fails"
  else
    fail "invalid option reports actionable error"
    printf '%s\n' "$invalid_output" >&2
  fi
fi

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
closed_output="$(PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open)"
assert_contains "$closed_output" "[closed] Group 1: latest two scheduled live-contract.yml runs are green on deadbeef and include required cron tier log markers" "stubbed live mode closes Group 1 with log markers"
assert_contains "$closed_output" "[closed] CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is true" "stubbed live mode closes audit-required repo variable"
assert_contains "$closed_output" "[closed] mutation.yml: latest scheduled run is green on deadbeef" "stubbed live mode closes mutation"
assert_contains "$closed_output" "[closed] codeql.yml: latest default-branch run is green on deadbeef" "stubbed live mode closes CodeQL"
assert_contains "$closed_output" "[closed] branch protection: main protection API is readable and D9 launch checks are required" "stubbed live mode closes branch protection"
assert_contains "$closed_output" "[closed] local branch cleanup: no non-main local branches are present" "stubbed live mode closes local branch cleanup"
assert_contains "$closed_output" "[closed] repository description matches the May 8 target wording" "stubbed live mode closes repo description"
assert_contains "$closed_output" "[closed] issue #28 is closed" "stubbed live mode closes issue 28"
assert_contains "$closed_output" "[closed] stale open PR hygiene: no open PRs older than 14 days without wip/blocked labels" "stubbed live mode closes stale PR hygiene"
assert_contains "$closed_output" "[closed] npm wrapper package is visible" "stubbed live mode proves npm package visibility"
assert_contains "$closed_output" "[closed] npm publish path: registry version and latest dist-tag match expected 1.2.0" "stubbed live mode closes expected npm proof"
assert_contains "$closed_output" "Summary: 0 open, 0 unknown" "stubbed live mode exits clean when all checked gates close"
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-protection-check-objects.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
protection_checks_output="$(TEST_EXTERNAL_STATUS_PROTECTION_CHECK_OBJECTS=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open)"
assert_contains "$protection_checks_output" "[closed] branch protection: main protection API is readable and D9 launch checks are required" "stubbed readable branch protection closes when required checks come from check objects"
assert_contains "$protection_checks_output" "Summary: 0 open, 0 unknown" "check-object branch protection fixture exits clean"
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-audit-var.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
audit_var_output=""
if audit_var_output="$(TEST_EXTERNAL_STATUS_AUDIT_VAR_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when live audit-required variable is missing"
else
  if grep -q "CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is missing" <<< "$audit_var_output" &&
     grep -q "action: set CLOCKIFY_LIVE_AUDIT_REQUIRED=true on example/repo" <<< "$audit_var_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$audit_var_output"; then
    pass "fail-open returns non-zero when live audit-required variable is missing"
  else
    fail "fail-open reports missing live audit-required variable"
    printf '%s\n' "$audit_var_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-audit-var-false.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
audit_var_false_output=""
if audit_var_false_output="$(TEST_EXTERNAL_STATUS_AUDIT_VAR_FALSE=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when live audit-required variable is false"
else
  if grep -q "CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is false, not true" <<< "$audit_var_false_output" &&
     grep -q "action: set CLOCKIFY_LIVE_AUDIT_REQUIRED=true on example/repo before counting Group 1 audit-phase cron evidence." <<< "$audit_var_false_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$audit_var_false_output"; then
    pass "fail-open returns non-zero when live audit-required variable is false"
  else
    fail "fail-open reports false live audit-required variable"
    printf '%s\n' "$audit_var_false_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-npm-proof.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
npm_proof_output=""
if npm_proof_output="$(PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when expected npm version is not supplied"
else
  if grep -q "npm wrapper package is visible" <<< "$npm_proof_output" &&
     grep -q "npm publish path: next rc/release proof is missing; no expected npm version was provided" <<< "$npm_proof_output" &&
     grep -q "action: on the next rc/release, rerun with --expected-npm-version vX.Y.Z" <<< "$npm_proof_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$npm_proof_output"; then
    pass "fail-open returns non-zero when expected npm version is not supplied"
  else
    fail "fail-open reports missing npm expected-version proof"
    printf '%s\n' "$npm_proof_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-live-log.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
live_log_output=""
if live_log_output="$(TEST_EXTERNAL_STATUS_LIVE_LOG_MISSING=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when live-contract cron logs miss required markers"
else
  if grep -q "Group 1: scheduled live-contract.yml runs are green on deadbeef, but required cron tier markers are missing" <<< "$live_log_output" &&
     grep -q "run 101: TestLiveCreateUpdateDeleteEntryAuditPhases" <<< "$live_log_output" &&
     grep -q "run 100: TestLiveCreateUpdateDeleteEntryAuditPhases" <<< "$live_log_output" &&
     grep -q "action: verify CLOCKIFY_LIVE_WRITE_ENABLED" <<< "$live_log_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$live_log_output"; then
    pass "fail-open returns non-zero when live-contract cron logs miss required markers"
  else
    fail "fail-open reports missing live-contract cron log markers"
    printf '%s\n' "$live_log_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-dirty.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
dirty_output=""
if dirty_output="$(TEST_EXTERNAL_STATUS_DIRTY=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when working tree is dirty"
else
  if grep -q "working tree is dirty; current HEAD does not represent this remediation tree" <<< "$dirty_output" &&
     grep -q "action: commit and push the remediation tree before treating HEAD as the candidate SHA." <<< "$dirty_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$dirty_output"; then
    pass "fail-open returns non-zero when working tree is dirty"
  else
    fail "fail-open reports dirty tree action details"
    printf '%s\n' "$dirty_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-dirty-default-sha.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
dirty_default_output=""
if dirty_default_output="$(TEST_EXTERNAL_STATUS_DIRTY=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open keeps Group 1 open when dirty tree uses default HEAD"
else
  if grep -q "working tree is dirty; current HEAD does not represent this remediation tree" <<< "$dirty_default_output" &&
     grep -q "Group 1: latest two scheduled live-contract.yml runs are green on deadbeef and include required cron tier log markers, but the dirty remediation tree has no final candidate SHA yet" <<< "$dirty_default_output" &&
     grep -q "action: commit and push the remediation tree, then wait for two consecutive scheduled live-contract.yml successes on that final SHA" <<< "$dirty_default_output" &&
     grep -q "Summary: 2 open, 0 unknown" <<< "$dirty_default_output"; then
    pass "fail-open keeps Group 1 open when dirty tree uses default HEAD"
  else
    fail "fail-open reports dirty default-HEAD Group 1 evidence caveat"
    printf '%s\n' "$dirty_default_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-open.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
open_output=""
if open_output="$(TEST_EXTERNAL_STATUS_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when a workflow is missing"
else
  if grep -q "codeql.yml: not available on the default branch" <<< "$open_output" &&
     grep -q "action: land the remediation tree containing codeql.yml, then verify a default-branch green run on the final candidate SHA." <<< "$open_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$open_output"; then
    pass "fail-open returns non-zero when a workflow is missing"
  else
    fail "fail-open reports missing workflow details"
    printf '%s\n' "$open_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-workflow-sha.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
workflow_sha_output=""
if workflow_sha_output="$(TEST_EXTERNAL_STATUS_WORKFLOW_SHA_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when workflow evidence is green on a stale SHA"
else
  if grep -q "codeql.yml: latest default-branch run is green but not on candidate SHA deadbeef" <<< "$workflow_sha_output" &&
     grep -q "action: wait for or rerun codeql.yml on the final candidate SHA" <<< "$workflow_sha_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$workflow_sha_output"; then
    pass "fail-open returns non-zero when workflow evidence is green on a stale SHA"
  else
    fail "fail-open reports stale workflow SHA details"
    printf '%s\n' "$workflow_sha_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-workflow-event.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
workflow_event_output=""
if workflow_event_output="$(TEST_EXTERNAL_STATUS_WORKFLOW_EVENT_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when workflow evidence is only a PR run"
else
  if grep -q "dependency-review.yml: latest run is green but is not default-branch evidence" <<< "$workflow_event_output" &&
     grep -q "action: wait for a push, schedule, or workflow_dispatch dependency-review.yml run on the final candidate SHA" <<< "$workflow_event_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$workflow_event_output"; then
    pass "fail-open returns non-zero when workflow evidence is only a PR run"
  else
    fail "fail-open reports non-default workflow event details"
    printf '%s\n' "$workflow_event_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-protection.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
protection_output=""
if protection_output="$(TEST_EXTERNAL_STATUS_PROTECTION_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when branch protection is unreadable"
else
  if grep -q "branch protection: main protection API is not readable" <<< "$protection_output" &&
     grep -q "action: use a repo-admin account or GitHub UI/visibility change to verify required checks, then archive the evidence." <<< "$protection_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$protection_output"; then
    pass "fail-open returns non-zero when branch protection is unreadable"
  else
    fail "fail-open reports branch-protection API details"
    printf '%s\n' "$protection_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-protection-missing-check.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
protection_missing_output=""
if protection_missing_output="$(TEST_EXTERNAL_STATUS_PROTECTION_MISSING_CHECK=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when D9 branch-protection checks are missing"
else
  if grep -q "branch protection: main protection API is readable but D9 launch required checks are missing" <<< "$protection_missing_output" &&
     grep -q "required-check-missing: Doctor Postgres backend" <<< "$protection_missing_output" &&
     grep -q "required-check-missing: Shared-service Postgres E2E" <<< "$protection_missing_output" &&
     grep -q "action: add or verify the missing launch required checks in GitHub branch protection before launch sign-off." <<< "$protection_missing_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$protection_missing_output"; then
    pass "fail-open returns non-zero when D9 branch-protection checks are missing"
  else
    fail "fail-open reports missing D9 branch-protection checks"
    printf '%s\n' "$protection_missing_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-branches.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
branches_output=""
if branches_output="$(TEST_EXTERNAL_STATUS_BRANCHES=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when non-main local branches remain"
else
  if grep -q "local branch cleanup: 2 non-main local branches require maintainer disposition against origin/main" <<< "$branches_output" &&
     grep -q "action: review ahead counts; merge, archive, or delete branches manually after maintainer approval." <<< "$branches_output" &&
     grep -q "branch: wave-a (head=aaaaaaa, base=origin/main, ahead=2, behind=5)" <<< "$branches_output" &&
     grep -q "branch: fwbranch (head=bbbbbbb, base=origin/main, ahead=0, behind=11)" <<< "$branches_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$branches_output"; then
    pass "fail-open returns non-zero when non-main local branches remain"
  else
    fail "fail-open reports local branch cleanup details"
    printf '%s\n' "$branches_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-runs.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
runs_output=""
if runs_output="$(TEST_EXTERNAL_STATUS_RUNS_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when scheduled workflow evidence is stale"
else
  if grep -q "Group 1: latest scheduled live-contract.yml runs do not prove two greens on deadbeef" <<< "$runs_output" &&
     grep -q "action: wait for two consecutive scheduled live-contract.yml successes on the final candidate SHA, then link both run URLs." <<< "$runs_output" &&
     grep -q "mutation.yml: latest scheduled run is not green on deadbeef" <<< "$runs_output" &&
     grep -q "job: Mutation (internal/tools) (completed/cancelled)" <<< "$runs_output" &&
     grep -q "action: land the remediation tree containing the mutation timeout fix if it is not on the default branch, then wait for a scheduled mutation.yml success on the final candidate SHA and link the run URL." <<< "$runs_output" &&
     grep -q "Summary: 2 open, 0 unknown" <<< "$runs_output"; then
    pass "fail-open returns non-zero when scheduled workflow evidence is stale"
  else
    fail "fail-open reports scheduled workflow action details"
    printf '%s\n' "$runs_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-stale-prs.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
stale_pr_output=""
if stale_pr_output="$(TEST_EXTERNAL_STATUS_STALE_PRS=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when stale open PRs lack labels"
else
  if grep -q "stale open PR hygiene: 1 PR(s) older than 14 days lack a wip or blocked label" <<< "$stale_pr_output" &&
     grep -q "action: close, update, merge, or label stale PRs with wip/blocked before public launch sign-off." <<< "$stale_pr_output" &&
     grep -q "pr: #42 Old launch PR (updated=2000-01-01T00:00:00Z, labels=review-needed)" <<< "$stale_pr_output" &&
     ! grep -q "pr: #43 Blocked old PR" <<< "$stale_pr_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$stale_pr_output"; then
    pass "fail-open returns non-zero when stale open PRs lack labels"
  else
    fail "fail-open reports stale PR action details"
    printf '%s\n' "$stale_pr_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-repo-issue.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
repo_issue_output=""
if repo_issue_output="$(TEST_EXTERNAL_STATUS_REPO_DESC_OPEN=1 TEST_EXTERNAL_STATUS_ISSUE_OPEN=1 PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.0 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when repo metadata and issue evidence remain open"
else
  if grep -q "repository description still needs the May 8 target wording" <<< "$repo_issue_output" &&
     grep -q "action: update the GitHub repository description to: 128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases." <<< "$repo_issue_output" &&
     grep -q "issue #28 is still OPEN" <<< "$repo_issue_output" &&
     grep -q "action: close or update issue #28 with the Group 2 shared-service evidence before final launch sign-off." <<< "$repo_issue_output" &&
     grep -q "Summary: 2 open, 0 unknown" <<< "$repo_issue_output"; then
    pass "fail-open returns non-zero when repo metadata and issue evidence remain open"
  else
    fail "fail-open reports repo metadata and issue action details"
    printf '%s\n' "$repo_issue_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-npm-rc-published.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
rc_published_output="$(TEST_EXTERNAL_STATUS_NPM_FIXTURE=rc-published PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.1-rc.3 --fail-open)"
assert_contains "$rc_published_output" "[closed] npm publish path: registry dist-tags.rc matches expected prerelease 1.2.1-rc.3" "rc prerelease closes against rc dist-tag"
assert_contains "$rc_published_output" "Summary: 0 open, 0 unknown" "rc prerelease fixture exits clean"
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-npm-beta-published.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
beta_published_output="$(TEST_EXTERNAL_STATUS_NPM_FIXTURE=beta-published PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v2.0.0-beta.1 --fail-open)"
assert_contains "$beta_published_output" "[closed] npm publish path: registry dist-tags.beta matches expected prerelease 2.0.0-beta.1" "beta prerelease closes against beta dist-tag"
assert_contains "$beta_published_output" "Summary: 0 open, 0 unknown" "beta prerelease fixture exits clean"
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-npm-rc-missing.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
rc_missing_output=""
if rc_missing_output="$(TEST_EXTERNAL_STATUS_NPM_FIXTURE=rc-missing PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.1-rc.3 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when expected prerelease dist-tag is absent"
else
  if grep -q "npm publish path: registry does not prove expected prerelease version 1.2.1-rc.3 on dist-tags.rc" <<< "$rc_missing_output" &&
     grep -q "action: verify the release workflow published @apet97/clockify-mcp-go@1.2.1-rc.3 with --tag rc" <<< "$rc_missing_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$rc_missing_output"; then
    pass "fail-open returns non-zero when expected prerelease dist-tag is absent"
  else
    fail "fail-open reports missing prerelease dist-tag details"
    printf '%s\n' "$rc_missing_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-external-status-npm-stable-stale.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_tools "$stub_dir"
stable_stale_output=""
if stable_stale_output="$(TEST_EXTERNAL_STATUS_NPM_FIXTURE=stable-stale PATH="$stub_dir/bin:$PATH" bash "$script" --repo example/repo --candidate-sha deadbeef --expected-npm-version v1.2.1 --fail-open 2>&1)"; then
  fail "fail-open returns non-zero when stable expected version is older than registry latest"
else
  if grep -q "npm publish path: registry does not prove expected release version 1.2.1" <<< "$stable_stale_output" &&
     grep -q "action: verify the release workflow published @apet97/clockify-mcp-go@1.2.1 with NPM_TOKEN" <<< "$stable_stale_output" &&
     grep -q "Summary: 1 open, 0 unknown" <<< "$stable_stale_output"; then
    pass "fail-open returns non-zero when stable expected version is older than registry latest"
  else
    fail "fail-open reports stable stale registry details"
    printf '%s\n' "$stable_stale_output" >&2
  fi
fi
rm -rf "$stub_dir"
trap - EXIT

if [ "$tests_failed" -ne 0 ]; then
  printf '\ncheck-launch-external-status tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
  exit 1
fi

printf '\ncheck-launch-external-status tests: %d run, 0 failed\n' "$tests_run"
