#!/usr/bin/env bash
#
# check-launch-external-status.sh
#
# Read-only helper for the external launch gates that cannot be proven
# by local tests. Default mode prints a status snapshot and exits 0 so
# it is safe for handoff/debugging; pass --fail-open to make open or
# unknown gates return exit 1.

set -euo pipefail

repo="${GITHUB_REPOSITORY:-apet97/go-clockify}"
branch_base_ref="${LAUNCH_BRANCH_CLEANUP_BASE_REF:-origin/main}"
expected_npm_version="${LAUNCH_EXPECTED_NPM_VERSION:-}"
candidate_sha=""
candidate_sha_explicit=0
working_tree_dirty=0
mode="run"
fail_open=0
target_repo_description="128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases."

usage() {
  cat <<'EOF'
Usage:
  scripts/check-launch-external-status.sh [--repo OWNER/NAME] [--candidate-sha SHA] [--expected-npm-version VERSION] [--fail-open]
  scripts/check-launch-external-status.sh --plan

Options:
  --repo OWNER/NAME       Repository to inspect. Default: GITHUB_REPOSITORY
                          or apet97/go-clockify.
  --candidate-sha SHA     Candidate SHA expected for scheduled workflow runs.
                          Default: current git HEAD.
  --expected-npm-version VERSION
                          Expected @apet97/clockify-mcp-go version in npm
                          for release-path proof. Leading "v" is accepted.
                          Default: LAUNCH_EXPECTED_NPM_VERSION or unset.
                          When unset, the next-release npm publish proof
                          remains open even if the current package is visible.
  --fail-open             Exit 1 when any checked external gate is open or
                          unknown. Default mode only reports.
  --plan                  Print the read-only checks without calling GitHub.
  -h, --help              Show this help.

Environment:
  LAUNCH_BRANCH_CLEANUP_BASE_REF
                          Base ref for local branch ahead/behind reporting.
                          Default: origin/main.
  LAUNCH_EXPECTED_NPM_VERSION
                          Default expected npm wrapper version when the
                          option above is not passed.

Checks:
  - working tree cleanliness, because a dirty tree has no candidate SHA;
  - non-main local branches that still need maintainer disposition;
  - CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is set to true;
  - two latest scheduled live-contract.yml runs on the candidate SHA;
  - latest scheduled mutation.yml run on the candidate SHA;
  - default-branch candidate-SHA runs for codeql.yml, dependency-review.yml, semgrep.yml;
  - main branch-protection API readability and required-status context data;
  - GitHub repository description tool-count / policy-count drift;
  - issue #28 closed state;
  - stale open PRs older than 14 days without wip/blocked labels;
  - npm wrapper visibility and optional expected-version match for
    next-release publish proof.
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      [ "$#" -ge 2 ] || die "--repo requires OWNER/NAME"
      repo="$2"
      shift 2
      ;;
    --candidate-sha)
      [ "$#" -ge 2 ] || die "--candidate-sha requires a SHA"
      candidate_sha="$2"
      candidate_sha_explicit=1
      shift 2
      ;;
    --expected-npm-version)
      [ "$#" -ge 2 ] || die "--expected-npm-version requires a version"
      expected_npm_version="$2"
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

if [ -z "$candidate_sha" ]; then
  candidate_sha="$(git rev-parse HEAD 2>/dev/null || true)"
fi

if [ "$mode" = "plan" ]; then
  cat <<EOF
Launch external status plan for ${repo}

Candidate SHA:
  ${candidate_sha:-<current git HEAD at runtime>}

Expected npm wrapper version:
  ${expected_npm_version:-<unset; next-release npm proof remains open>}

Read-only checks:
  - git diff --quiet && git diff --cached --quiet
  - git for-each-ref --format='%(refname:short)' refs/heads
  - git rev-list --count ${branch_base_ref}..<branch> and <branch>..${branch_base_ref}
  - gh variable list --repo ${repo} --json name,value,updatedAt
    and confirm CLOCKIFY_LIVE_AUDIT_REQUIRED is set to true
  - gh run list --repo ${repo} --workflow live-contract.yml --limit 10 --json ...
  - gh run view <live-contract-run-id> --repo ${repo} --log
    and grep for TestE2EMutating, TestLiveCreateUpdateDeleteEntryAuditPhases,
    and TestLiveReadSideSchemaDiff before counting a scheduled green
  - gh run list --repo ${repo} --workflow mutation.yml --limit 10 --json ...
  - gh run view <mutation-run-id> --repo ${repo} --json jobs
    to name failed, cancelled, or still-running matrix legs
  - gh run list --repo ${repo} --workflow codeql.yml --branch main --limit 1 --json ...
  - gh run list --repo ${repo} --workflow dependency-review.yml --branch main --limit 1 --json ...
  - gh run list --repo ${repo} --workflow semgrep.yml --branch main --limit 1 --json ...
  - gh api repos/${repo}/branches/main/protection --jq ...
    and confirm Doctor strict smoke, Doctor Postgres backend, and
    Shared-service Postgres E2E are required checks when the API is readable
  - gh repo view ${repo} --json description,visibility,isPrivate,url
    and compare the description with:
      ${target_repo_description}
  - gh issue view 28 --repo ${repo} --json state,title,url,updatedAt
  - gh pr list --repo ${repo} --state open --limit 100 --json number,title,url,updatedAt,labels
  - npm view @apet97/clockify-mcp-go version dist-tags --json
  - compare npm version/dist-tags.latest with --expected-npm-version, when set

This helper does not mutate GitHub, npm, the repository, or launch
checklist boxes. It reports whether external evidence is closed, open,
or unknown, and prints maintainer actions for gates that still need
external disposition; use --fail-open only when a non-zero exit is
useful.
EOF
  exit 0
fi

command -v gh >/dev/null 2>&1 || die "gh is required for external status checks"

open_count=0
unknown_count=0
required_live_contract_log_markers=(
  "TestE2EMutating"
  "TestLiveCreateUpdateDeleteEntryAuditPhases"
  "TestLiveReadSideSchemaDiff"
)
required_branch_protection_contexts=(
  "Doctor strict smoke"
  "Doctor Postgres backend"
  "Shared-service Postgres E2E"
)

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

first_line() {
  awk 'NR == 1 { print; exit }'
}

second_line() {
  awk 'NR == 2 { print; exit }'
}

json_string_field_matches() {
  local json="$1"
  local field="$2"
  local value="$3"
  grep -Eq "\"${field}\"[[:space:]]*:[[:space:]]*\"${value}\"" <<< "$json"
}

iso8601_epoch() {
  local value="$1"
  date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$value" +%s 2>/dev/null ||
    date -u -d "$value" +%s 2>/dev/null
}

scheduled_runs() {
  local workflow="$1"
  gh run list --repo "$repo" --workflow "$workflow" --limit 10 \
    --json databaseId,event,status,conclusion,createdAt,headSha,url \
    --jq 'def value($fallback): if . == null or . == "" then $fallback else . end; .[] | select(.event == "schedule") | [(.databaseId | tostring), (.status | value("unknown")), (.conclusion | value("none")), (.headSha | value("unknown")), (.createdAt | value("unknown")), (.url | value("unknown"))] | @tsv'
}

latest_run() {
  local workflow="$1"
  gh run list --repo "$repo" --workflow "$workflow" --branch main --limit 1 \
    --json databaseId,event,status,conclusion,createdAt,headSha,url \
    --jq 'def value($fallback): if . == null or . == "" then $fallback else . end; .[] | [(.databaseId | tostring), (.event | value("unknown")), (.status | value("unknown")), (.conclusion | value("none")), (.headSha | value("unknown")), (.createdAt | value("unknown")), (.url | value("unknown"))] | @tsv'
}

missing_live_contract_markers() {
  local run_id="$1"
  local log_out marker
  if ! log_out="$(gh run view "$run_id" --repo "$repo" --log 2>&1)"; then
    printf '__LOG_READ_FAILED__ %s\n' "$log_out"
    return 0
  fi
  for marker in "${required_live_contract_log_markers[@]}"; do
    if ! grep -qF "$marker" <<< "$log_out"; then
      printf '%s\n' "$marker"
    fi
  done
}

mutation_problem_jobs() {
  local run_id="$1"
  local jobs_out
  if ! jobs_out="$(gh run view "$run_id" --repo "$repo" --json jobs \
      --jq '.jobs[] | select((.status != "completed") or (.conclusion != "success")) | "\(.name) (\(.status)/\(if .conclusion == null or .conclusion == "" then "none" else .conclusion end))"' 2>&1)"; then
    printf '__JOB_READ_FAILED__ %s\n' "$jobs_out"
    return 0
  fi
  printf '%s\n' "$jobs_out"
}

printf 'Launch external status for %s\n' "$repo"
printf 'Candidate SHA: %s\n\n' "${candidate_sha:-unknown}"

if [ -z "$candidate_sha" ]; then
  record unknown "candidate SHA is unknown; run inside a git checkout or pass --candidate-sha"
  action "run from the intended checkout or pass the final candidate SHA explicitly."
elif ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
  working_tree_dirty=1
  record open "working tree is dirty; current HEAD does not represent this remediation tree"
  action "commit and push the remediation tree before treating HEAD as the candidate SHA."
else
  record closed "working tree is clean for candidate SHA comparison"
fi

local_branches=""
if local_branches="$(git for-each-ref --format='%(refname:short)' refs/heads 2>&1)"; then
  non_main_branches="$(printf '%s\n' "$local_branches" | sed '/^$/d; /^main$/d')"
  if [ -n "$non_main_branches" ]; then
    branch_count="$(printf '%s\n' "$non_main_branches" | wc -l | tr -d '[:space:]')"
    record open "local branch cleanup: ${branch_count} non-main local branches require maintainer disposition against ${branch_base_ref}"
    action "review ahead counts; merge, archive, or delete branches manually after maintainer approval."
    while IFS= read -r branch; do
      [ -n "$branch" ] || continue
      branch_head="$(git rev-parse --short "$branch" 2>/dev/null || printf '?')"
      branch_ahead="$(git rev-list --count "${branch_base_ref}..${branch}" 2>/dev/null || printf '?')"
      branch_behind="$(git rev-list --count "${branch}..${branch_base_ref}" 2>/dev/null || printf '?')"
      printf '       branch: %s (head=%s, base=%s, ahead=%s, behind=%s)\n' \
        "$branch" "$branch_head" "$branch_base_ref" "$branch_ahead" "$branch_behind"
    done <<< "$non_main_branches"
  else
    record closed "local branch cleanup: no non-main local branches are present"
  fi
else
  record unknown "unable to inspect local branches: $local_branches"
  action "rerun from a valid git checkout and confirm ${branch_base_ref} exists locally."
fi

audit_required_row=""
if audit_required_row="$(gh variable list --repo "$repo" --json name,value,updatedAt \
  --jq '.[] | select(.name=="CLOCKIFY_LIVE_AUDIT_REQUIRED") | [.name,.value,.updatedAt] | @tsv' 2>&1)"; then
  if [ -z "$audit_required_row" ]; then
    record open "CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is missing"
    action "set CLOCKIFY_LIVE_AUDIT_REQUIRED=true on ${repo} so missing MCP_LIVE_CONTROL_PLANE_DSN fails the main-repo live-contract cron."
  else
    IFS=$'\t' read -r _audit_required_name audit_required_value audit_required_updated <<< "$audit_required_row"
    if [ "$audit_required_value" = "true" ]; then
      record closed "CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is true"
      printf '       updatedAt: %s\n' "$audit_required_updated"
    else
      record open "CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable is ${audit_required_value:-<empty>}, not true"
      action "set CLOCKIFY_LIVE_AUDIT_REQUIRED=true on ${repo} before counting Group 1 audit-phase cron evidence."
    fi
  fi
else
  record unknown "unable to read CLOCKIFY_LIVE_AUDIT_REQUIRED repository variable: $audit_required_row"
  action "rerun with GitHub Actions variable read access, then verify CLOCKIFY_LIVE_AUDIT_REQUIRED=true before final launch sign-off."
fi

live_rows=""
if live_rows="$(scheduled_runs live-contract.yml 2>&1)"; then
  live_first="$(printf '%s\n' "$live_rows" | first_line)"
  live_second="$(printf '%s\n' "$live_rows" | second_line)"
  if [ -z "$live_first" ] || [ -z "$live_second" ]; then
    record open "Group 1: fewer than two scheduled live-contract.yml runs are available"
    action "after the remediation tree lands, wait for two scheduled cron greens on the same final candidate SHA."
  else
    IFS=$'\t' read -r live_id live_status live_conclusion live_sha live_created live_url <<< "$live_first"
    IFS=$'\t' read -r live2_id live2_status live2_conclusion live2_sha live2_created live2_url <<< "$live_second"
    if [ "$live_status" = "completed" ] && [ "$live_conclusion" = "success" ] &&
       [ "$live2_status" = "completed" ] && [ "$live2_conclusion" = "success" ] &&
       [ -n "$candidate_sha" ] && [ "$live_sha" = "$candidate_sha" ] && [ "$live2_sha" = "$candidate_sha" ]; then
      live_marker_missing=""
      while IFS= read -r marker; do
        [ -n "$marker" ] || continue
        live_marker_missing="${live_marker_missing} run ${live_id}: ${marker};"
      done < <(missing_live_contract_markers "$live_id")
      while IFS= read -r marker; do
        [ -n "$marker" ] || continue
        live_marker_missing="${live_marker_missing} run ${live2_id}: ${marker};"
      done < <(missing_live_contract_markers "$live2_id")
      if grep -qF "__LOG_READ_FAILED__" <<< "$live_marker_missing"; then
        record unknown "Group 1: unable to read live-contract.yml logs for required cron tier markers:${live_marker_missing}"
        action "rerun with GitHub Actions log access and verify each counted cron run logs TestE2EMutating, TestLiveCreateUpdateDeleteEntryAuditPhases, and TestLiveReadSideSchemaDiff."
      elif [ -n "$live_marker_missing" ]; then
        record open "Group 1: scheduled live-contract.yml runs are green on ${candidate_sha}, but required cron tier markers are missing:${live_marker_missing}"
        action "verify CLOCKIFY_LIVE_WRITE_ENABLED, CLOCKIFY_LIVE_AUDIT_REQUIRED, and MCP_LIVE_CONTROL_PLANE_DSN, then wait for two scheduled greens whose logs include the mutating, audit, and schema-diff tests."
      elif [ "$working_tree_dirty" = "1" ] && [ "$candidate_sha_explicit" = "0" ]; then
        record open "Group 1: latest two scheduled live-contract.yml runs are green on ${candidate_sha} and include required cron tier log markers, but the dirty remediation tree has no final candidate SHA yet (${live_id}, ${live2_id})"
        action "commit and push the remediation tree, then wait for two consecutive scheduled live-contract.yml successes on that final SHA with the required cron tier log markers."
      else
        record closed "Group 1: latest two scheduled live-contract.yml runs are green on ${candidate_sha} and include required cron tier log markers (${live_id}, ${live2_id})"
      fi
    else
      record open "Group 1: latest scheduled live-contract.yml runs do not prove two greens on ${candidate_sha:-unknown}; latest=${live_id}/${live_status}/${live_conclusion}/${live_sha}/${live_created} previous=${live2_id}/${live2_status}/${live2_conclusion}/${live2_sha}/${live2_created}"
      action "wait for two consecutive scheduled live-contract.yml successes on the final candidate SHA, then link both run URLs."
    fi
    printf '       latest URL: %s\n' "$live_url"
    printf '       previous URL: %s\n' "$live2_url"
  fi
else
  record unknown "Group 1: unable to read live-contract.yml runs: $live_rows"
  action "verify gh authentication, repository access, and workflow name, then rerun this helper."
fi

mutation_rows=""
if mutation_rows="$(scheduled_runs mutation.yml 2>&1)"; then
  mutation_first="$(printf '%s\n' "$mutation_rows" | first_line)"
  if [ -z "$mutation_first" ]; then
    record open "mutation.yml: no scheduled runs are available"
    action "wait for the next scheduled mutation.yml run after the workflow fix lands, then archive the run URL."
  else
    IFS=$'\t' read -r mutation_id mutation_status mutation_conclusion mutation_sha mutation_created mutation_url <<< "$mutation_first"
    if [ "$mutation_status" = "completed" ] && [ "$mutation_conclusion" = "success" ] &&
       [ -n "$candidate_sha" ] && [ "$mutation_sha" = "$candidate_sha" ]; then
      record closed "mutation.yml: latest scheduled run is green on ${candidate_sha} (${mutation_id})"
    else
      record open "mutation.yml: latest scheduled run is not green on ${candidate_sha:-unknown}; latest=${mutation_id}/${mutation_status}/${mutation_conclusion}/${mutation_sha}/${mutation_created}"
      action "land the remediation tree containing the mutation timeout fix if it is not on the default branch, then wait for a scheduled mutation.yml success on the final candidate SHA and link the run URL."
      mutation_jobs="$(mutation_problem_jobs "$mutation_id")"
      if [ -n "$mutation_jobs" ]; then
        if grep -q '^__JOB_READ_FAILED__' <<< "$mutation_jobs"; then
          printf '       detail: unable to inspect mutation matrix jobs: %s\n' "${mutation_jobs#__JOB_READ_FAILED__ }"
        else
          while IFS= read -r mutation_job; do
            [ -n "$mutation_job" ] || continue
            printf '       job: %s\n' "$mutation_job"
          done <<< "$mutation_jobs"
        fi
      fi
    fi
    printf '       latest URL: %s\n' "$mutation_url"
  fi
else
  record unknown "mutation.yml: unable to read scheduled runs: $mutation_rows"
  action "verify gh authentication, repository access, and workflow name, then rerun this helper."
fi

for workflow in codeql.yml dependency-review.yml semgrep.yml; do
  workflow_out=""
  if workflow_out="$(latest_run "$workflow" 2>&1)"; then
    workflow_line="$(printf '%s\n' "$workflow_out" | first_line)"
    if [ -z "$workflow_line" ]; then
      record open "${workflow}: workflow exists but has no recorded run"
      action "push the remediation tree to the default branch, then capture the first successful ${workflow} run on the final candidate SHA."
      continue
    fi
    IFS=$'\t' read -r wf_id wf_event wf_status wf_conclusion wf_sha wf_created wf_url <<< "$workflow_line"
    if [ "$wf_status" = "completed" ] && [ "$wf_conclusion" = "success" ] &&
       { [ "$wf_event" = "push" ] || [ "$wf_event" = "schedule" ] || [ "$wf_event" = "workflow_dispatch" ]; } &&
       [ -n "$candidate_sha" ] && [ "$wf_sha" = "$candidate_sha" ]; then
      record closed "${workflow}: latest default-branch run is green on ${candidate_sha} (${wf_id}, ${wf_event}, ${wf_created})"
    elif [ "$wf_status" = "completed" ] && [ "$wf_conclusion" = "success" ] &&
         [ "$wf_event" = "pull_request" ]; then
      record open "${workflow}: latest run is green but is not default-branch evidence (${wf_id}, ${wf_event}, ${wf_sha}, ${wf_created})"
      action "wait for a push, schedule, or workflow_dispatch ${workflow} run on the final candidate SHA, then link the run URL."
    elif [ "$wf_status" = "completed" ] && [ "$wf_conclusion" = "success" ] &&
         { [ -z "$candidate_sha" ] || [ "$wf_sha" != "$candidate_sha" ]; }; then
      record open "${workflow}: latest default-branch run is green but not on candidate SHA ${candidate_sha:-unknown} (${wf_id}, ${wf_event}, ${wf_sha}, ${wf_created})"
      action "wait for or rerun ${workflow} on the final candidate SHA, then link the run URL."
    else
      record open "${workflow}: latest run is not green (${wf_id}, ${wf_event}, ${wf_status}/${wf_conclusion}, ${wf_sha}, ${wf_created})"
      action "fix or rerun ${workflow} until the default-branch run is green on the final candidate SHA, then link the run URL."
    fi
    printf '       latest URL: %s\n' "$wf_url"
  else
    record open "${workflow}: not available on the default branch or unreadable: $workflow_out"
    action "land the remediation tree containing ${workflow}, then verify a default-branch green run on the final candidate SHA."
  fi
done

branch_protection_line=""
if branch_protection_line="$(gh api "repos/${repo}/branches/main/protection" \
    --jq '[((.required_status_checks.strict // false) | tostring), ((((.required_status_checks.contexts // []) + ((.required_status_checks.checks // []) | map(.context // empty))) | unique) | join(",")), ((.required_pull_request_reviews != null) | tostring), ((.enforce_admins.enabled // false) | tostring)] | @tsv' 2>&1)"; then
  IFS=$'\t' read -r bp_strict bp_contexts bp_reviews bp_admins <<< "$branch_protection_line"
  if [ -z "$bp_contexts" ]; then
    record open "branch protection: main protection API is readable but no required status checks were returned"
    action "configure or verify required main-branch checks in GitHub before launch sign-off."
  else
    missing_bp_contexts=""
    for required_context in "${required_branch_protection_contexts[@]}"; do
      if ! grep -qxF "$required_context" <<< "$(printf '%s\n' "$bp_contexts" | tr ',' '\n')"; then
        missing_bp_contexts="${missing_bp_contexts}${required_context}"$'\n'
      fi
    done
    if [ -n "$missing_bp_contexts" ]; then
      record open "branch protection: main protection API is readable but D9 launch required checks are missing"
      printf '%s' "$missing_bp_contexts" | sed '/^$/d; s/^/       required-check-missing: /'
      printf '       current required_checks: %s\n' "$bp_contexts"
      action "add or verify the missing launch required checks in GitHub branch protection before launch sign-off."
    else
      record closed "branch protection: main protection API is readable and D9 launch checks are required (strict=${bp_strict}, required_checks=${bp_contexts}, pr_reviews=${bp_reviews}, enforce_admins=${bp_admins})"
    fi
  fi
else
  if grep -qiE 'Upgrade to GitHub Pro|make this repository public|Branch not protected|Not Found|HTTP 404|Resource not accessible' <<< "$branch_protection_line"; then
    record open "branch protection: main protection API is not readable; maintainer must verify D9 required checks or repository visibility: $branch_protection_line"
    action "use a repo-admin account or GitHub UI/visibility change to verify required checks, then archive the evidence."
  else
    record unknown "unable to read branch protection API for main: $branch_protection_line"
    action "rerun with repo-admin gh authentication or capture the exact GitHub UI protection state."
  fi
fi

repo_line=""
if repo_line="$(gh repo view "$repo" --json description,isPrivate,visibility,url --jq '[.description, (.isPrivate|tostring), .visibility, .url] | @tsv' 2>&1)"; then
  IFS=$'\t' read -r repo_description repo_private repo_visibility repo_url <<< "$repo_line"
  if [ "$repo_description" != "$target_repo_description" ]; then
    record open "repository description still needs the May 8 target wording (${repo_visibility}, private=${repo_private})"
    action "update the GitHub repository description to: ${target_repo_description}"
  else
    record closed "repository description matches the May 8 target wording (${repo_visibility}, private=${repo_private})"
  fi
  printf '       repo: %s\n' "$repo_url"
else
  record unknown "unable to read repository description: $repo_line"
  action "rerun with repository read access before using this as launch evidence."
fi

issue_line=""
if issue_line="$(gh issue view 28 --repo "$repo" --json state,title,url,updatedAt --jq '[.state, .title, .url, .updatedAt] | @tsv' 2>&1)"; then
  IFS=$'\t' read -r issue_state issue_title issue_url issue_updated <<< "$issue_line"
  if [ "$issue_state" = "CLOSED" ]; then
    record closed "issue #28 is closed (${issue_updated})"
  else
    record open "issue #28 is still ${issue_state}: ${issue_title} (${issue_updated})"
    action "close or update issue #28 with the Group 2 shared-service evidence before final launch sign-off."
  fi
  printf '       issue: %s\n' "$issue_url"
else
  record unknown "unable to read issue #28: $issue_line"
  action "rerun with issue read access or manually verify the issue state in GitHub."
fi

pr_lines=""
if pr_lines="$(gh pr list --repo "$repo" --state open --limit 100 \
    --json number,title,url,updatedAt,labels \
    --jq '.[] | [.number, .title, .url, .updatedAt, ((.labels // []) | map(.name) | join(","))] | @tsv' 2>&1)"; then
  stale_prs=""
  unparsable_prs=""
  now_epoch="$(date -u +%s)"
  stale_after_seconds=$((14 * 24 * 60 * 60))
  while IFS=$'\t' read -r pr_number pr_title pr_url pr_updated pr_labels; do
    [ -n "$pr_number" ] || continue
    pr_epoch="$(iso8601_epoch "$pr_updated" || true)"
    if [ -z "$pr_epoch" ]; then
      unparsable_prs="${unparsable_prs}${pr_number}	${pr_title}	${pr_url}	${pr_updated}	${pr_labels}"$'\n'
      continue
    fi
    age_seconds=$((now_epoch - pr_epoch))
    labels_lower="$(printf '%s' "$pr_labels" | tr '[:upper:]' '[:lower:]')"
    if [ "$age_seconds" -gt "$stale_after_seconds" ] &&
       ! grep -Eq '(^|,)(wip|blocked)(,|$)' <<< "$labels_lower"; then
      stale_prs="${stale_prs}${pr_number}	${pr_title}	${pr_url}	${pr_updated}	${pr_labels}"$'\n'
    fi
  done <<< "$pr_lines"

  if [ -n "$unparsable_prs" ]; then
    record unknown "unable to parse updatedAt for one or more open PRs"
    action "verify stale-PR state in the GitHub UI; PRs older than 14 days need a wip or blocked label."
    while IFS=$'\t' read -r pr_number pr_title pr_url pr_updated pr_labels; do
      [ -n "$pr_number" ] || continue
      printf '       pr: #%s %s (updated=%s, labels=%s)\n' "$pr_number" "$pr_title" "$pr_updated" "${pr_labels:-none}"
      printf '       pr URL: %s\n' "$pr_url"
    done <<< "$unparsable_prs"
  elif [ -n "$stale_prs" ]; then
    stale_count="$(printf '%s\n' "$stale_prs" | sed '/^$/d' | wc -l | tr -d '[:space:]')"
    record open "stale open PR hygiene: ${stale_count} PR(s) older than 14 days lack a wip or blocked label"
    action "close, update, merge, or label stale PRs with wip/blocked before public launch sign-off."
    while IFS=$'\t' read -r pr_number pr_title pr_url pr_updated pr_labels; do
      [ -n "$pr_number" ] || continue
      printf '       pr: #%s %s (updated=%s, labels=%s)\n' "$pr_number" "$pr_title" "$pr_updated" "${pr_labels:-none}"
      printf '       pr URL: %s\n' "$pr_url"
    done <<< "$stale_prs"
  else
    record closed "stale open PR hygiene: no open PRs older than 14 days without wip/blocked labels"
  fi
else
  record unknown "unable to read open PR stale-state: $pr_lines"
  action "rerun with pull-request read access or manually verify no stale open PRs lack wip/blocked labels."
fi

if command -v npm >/dev/null 2>&1; then
  npm_out=""
  if npm_out="$(npm view @apet97/clockify-mcp-go version dist-tags --json 2>&1)"; then
    record closed "npm wrapper package is visible"
    printf '%s\n' "$npm_out" | sed 's/^/       npm: /'
    if [ -n "$expected_npm_version" ]; then
      normalized_npm_version="${expected_npm_version#v}"
      if json_string_field_matches "$npm_out" "version" "$normalized_npm_version" &&
         json_string_field_matches "$npm_out" "latest" "$normalized_npm_version"; then
        record closed "npm publish path: registry version and latest dist-tag match expected ${normalized_npm_version}"
      else
        record open "npm publish path: registry does not prove expected release version ${normalized_npm_version}"
        action "verify the release workflow published @apet97/clockify-mcp-go@${normalized_npm_version} with NPM_TOKEN, then rerun with --expected-npm-version ${normalized_npm_version}."
      fi
    else
      record open "npm publish path: next rc/release proof is missing; no expected npm version was provided"
      action "on the next rc/release, rerun with --expected-npm-version vX.Y.Z[-rc.N] and verify NPM_TOKEN-backed publish evidence for that version."
    fi
  else
    record unknown "unable to read npm package status: $npm_out"
    action "verify npm registry access now, then capture publish evidence during the rc/release."
  fi
else
  record unknown "npm is not installed; cannot check current wrapper package visibility"
  action "install npm or verify the package from an environment that can read the npm registry."
fi

printf '\nSummary: %d open, %d unknown\n' "$open_count" "$unknown_count"

if [ "$fail_open" = "1" ] && { [ "$open_count" -ne 0 ] || [ "$unknown_count" -ne 0 ]; }; then
  exit 1
fi
