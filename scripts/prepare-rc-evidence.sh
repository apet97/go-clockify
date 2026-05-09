#!/usr/bin/env bash
#
# prepare-rc-evidence.sh
#
# Runs or prints the release-candidate evidence sequence for launch
# checklist Groups 6 and 7 after Group 1 scheduled-cron evidence has
# closed. The default mode writes evidence under /tmp so it does not
# dirty the checkout.

set -euo pipefail

usage() {
    cat <<'EOF'
Usage:
  scripts/prepare-rc-evidence.sh [--plan] [--trigger-release-smoke] TAG

Arguments:
  TAG                       Candidate tag, e.g. v1.2.1-rc.1.

Options:
  --plan                    Print the evidence sequence without running it.
  --trigger-release-smoke   Dispatch release-smoke.yml for TAG after local
                            evidence commands finish.
  -h, --help                Show this help.

Environment:
  RC_EVIDENCE_DIR           Output directory. Default:
                            ${TMPDIR:-/tmp}/go-clockify-rc-evidence/TAG
  RC_EVIDENCE_ALLOW_NON_TAG Set to 1 to allow running while HEAD is not TAG.
                            This is for rehearsal only; launch evidence must
                            be collected from the candidate tag.

This script intentionally does not create or push tags. Cut vX.Y.Z-rc.N only
after Group 1 scheduled-cron evidence closes, then run this script from a
clean checkout of that tag on each required host.
EOF
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

mode="run"
trigger_release_smoke=0
tag=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        --plan)
            mode="plan"
            shift
            ;;
        --trigger-release-smoke)
            trigger_release_smoke=1
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
            if [ -n "$tag" ]; then
                die "unexpected extra argument: $1"
            fi
            tag="$1"
            shift
            ;;
    esac
done

if [ -z "$tag" ]; then
    usage >&2
    exit 2
fi

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+$ ]]; then
    die "candidate tag must look like vX.Y.Z-rc.N, got: $tag"
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

repo="${GITHUB_REPOSITORY:-apet97/go-clockify}"
host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
host_arch="$(uname -m)"
case "$host_arch" in
    x86_64) host_arch="x64" ;;
    aarch64|arm64) host_arch="arm64" ;;
esac
host_label="${host_os}-${host_arch}"

evidence_dir="${RC_EVIDENCE_DIR:-${TMPDIR:-/tmp}/go-clockify-rc-evidence/${tag}}"
log_dir="${evidence_dir}/logs/${host_label}"

group6_commands=(
    "make check"
    "make verify-vuln"
    "make secret-scan"
    "semgrep scan --config p/default --metrics=off --error --exclude .git --exclude .bench --exclude clockify-mcp ."
    "git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md' || true"
    "make verify-fips"
)

group7_commands=(
    "make release-check"
    "make verify-bench"
    "make bench-baseline-check"
)

workflow_files=(
    "ci.yml"
    "build-matrix.yml"
    "codeql.yml"
    "dependency-review.yml"
    "semgrep.yml"
    "live-contract.yml"
    "release.yml"
    "release-smoke.yml"
    "docker-image.yml"
    "link-check.yml"
    "chaos.yml"
    "mutation.yml"
    "reproducibility.yml"
    "bench.yml"
)

print_plan() {
    cat <<EOF
Release-candidate evidence plan for ${tag}

Preconditions:
  - Group 1 scheduled-cron evidence is closed in docs/launch-candidate-checklist.md.
  - ${tag} exists and this checkout is clean at that tag.
  - Run on both required release-check hosts: linux-x64 and darwin-arm64.
  - govulncheck, gitleaks, semgrep, gh, npm, and FIPS-capable Go tooling are installed.

Default evidence directory:
  ${evidence_dir}

Group 6 commands:
EOF
    for cmd in "${group6_commands[@]}"; do
        printf '  - %s\n' "$cmd"
    done
    cat <<'EOF'

Group 7 local commands:
EOF
    for cmd in "${group7_commands[@]}"; do
        printf '  - %s\n' "$cmd"
    done
    cat <<EOF

Group 7 release asset evidence:
  - gh release view ${tag} --repo ${repo} --json tagName,targetCommitish,isDraft,isPrerelease,url,assets
  - validate the GitHub Release asset names with scripts/check-release-assets.sh
EOF
    cat <<'EOF'

Group 7 GitHub evidence collected:
EOF
    for wf in "${workflow_files[@]}"; do
        printf '  - raw latest-run metadata snapshot for %s via gh run list\n' "$wf"
    done
    cat <<EOF

Workflow snapshot warning:
  The metadata logs above are audit context only. Treat the
  check-launch-external-status --fail-open result below as the fail-closed
  validator for final-candidate-SHA workflow evidence.

Group 7 external-status snapshot:
  LAUNCH_EXPECTED_NPM_VERSION=${tag} scripts/check-launch-external-status.sh --candidate-sha <candidate-sha>

Final external gate before sign-off:
  scripts/check-launch-external-status.sh \\
    --candidate-sha <candidate-sha> \\
    --expected-npm-version ${tag} \\
    --fail-open

Optional release-smoke dispatch:
  scripts/prepare-rc-evidence.sh --trigger-release-smoke ${tag}

Release-smoke doctor output artifact:
  Archive or link release-smoke-doctor-output from the green release-smoke.yml
  run. It must contain release-doctor-strict-ok.txt,
  release-doctor-strict-fail.txt, and release-doctor-postgres-ok.txt.

Manual variant evidence:
  Use docs/verification.md with ${tag} and any required artifact filename not
  sampled by release-smoke.yml, including the FIPS variant named in Group 7.

Checklist update rule:
  Do not tick Groups 6 or 7 until each checked box has a nearby
  GitHub Actions URL, workflow_run_id, or _Closed_ annotation accepted
  by scripts/check-launch-evidence-gate.sh.
EOF
}

if [ "$mode" = "plan" ]; then
    print_plan
    exit 0
fi

require_tool() {
    local tool="$1"
    command -v "$tool" >/dev/null 2>&1 || die "$tool is required for candidate evidence"
}

run_capture() {
    local label="$1"
    shift
    local log="${log_dir}/${label}.log"
    echo "== ${label} =="
    {
        printf '$'
        printf ' %q' "$@"
        printf '\n\n'
        "$@"
    } >"$log" 2>&1
}

run_shell_capture() {
    local label="$1"
    local cmd="$2"
    local log="${log_dir}/${label}.log"
    echo "== ${label} =="
    {
        printf '$ %s\n\n' "$cmd"
        bash -lc "$cmd"
    } >"$log" 2>&1
}

run_gh_capture() {
    local workflow="$1"
    local safe_name
    safe_name="$(printf '%s' "$workflow" | tr '/.' '--')"
    run_capture "workflow-${safe_name}" \
        gh run list --workflow "$workflow" --limit 5 \
        --json databaseId,workflowName,displayTitle,headBranch,headSha,event,status,conclusion,createdAt,updatedAt,url
}

capture_release_assets() {
    local log="${log_dir}/github-release-assets.log"
    local asset_dir
    asset_dir="$(mktemp -d "${TMPDIR:-/tmp}/go-clockify-release-assets.XXXXXX")"
    trap 'rm -rf "$asset_dir"' RETURN

    echo "== github-release-assets =="
    {
        printf '$ gh release view %q --repo %q --json tagName,targetCommitish,isDraft,isPrerelease,url,assets\n\n' "$tag" "$repo"
        gh release view "$tag" --repo "$repo" \
            --json tagName,targetCommitish,isDraft,isPrerelease,url,assets

        printf '\n$ gh release view %q --repo %q --json assets --jq %q\n\n' "$tag" "$repo" '.assets[].name'
        while IFS= read -r asset; do
            [ -n "$asset" ] || continue
            : >"${asset_dir}/${asset}"
            printf '%s\n' "$asset"
        done < <(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name')

        printf '\n$ scripts/check-release-assets.sh <github-release-asset-list>\n\n'
        scripts/check-release-assets.sh "$asset_dir"
    } >"$log" 2>&1

    rm -rf "$asset_dir"
    trap - RETURN
}

if ! git rev-parse -q --verify "refs/tags/${tag}^{commit}" >/dev/null; then
    die "tag ${tag} does not exist locally; fetch tags or cut the candidate tag first"
fi

head_sha="$(git rev-parse HEAD)"
tag_sha="$(git rev-parse "refs/tags/${tag}^{commit}")"
if [ "$head_sha" != "$tag_sha" ] && [ "${RC_EVIDENCE_ALLOW_NON_TAG:-0}" != "1" ]; then
    die "HEAD (${head_sha}) is not ${tag} (${tag_sha}); checkout the candidate tag before collecting evidence"
fi

git diff --quiet || die "working tree has unstaged changes"
git diff --cached --quiet || die "working tree has staged changes"

require_tool govulncheck
require_tool gitleaks
require_tool semgrep
require_tool gh
require_tool npm

mkdir -p "$log_dir"

cat >"${evidence_dir}/README.md" <<EOF
# Release-candidate evidence for ${tag}

- Tag: ${tag}
- Tag SHA: ${tag_sha}
- Host: ${host_label}
- Generated: $(date -u '+%Y-%m-%dT%H:%M:%SZ')

This directory was generated by \`scripts/prepare-rc-evidence.sh\`.
Attach or link these logs from \`docs/launch-candidate-checklist.md\`
before checking any Group 6 or Group 7 evidence box.

Workflow logs in this bundle are raw latest-run metadata snapshots.
They are not final-candidate-SHA proof by themselves; use the
\`launch-external-status\` log and rerun
\`scripts/check-launch-external-status.sh --fail-open\` after the
candidate release path publishes.
EOF

run_capture "git-status" git status -sb
run_capture "git-rev-parse-head" git rev-parse HEAD

for cmd in "${group6_commands[@]}"; do
    label="group6-$(printf '%s' "$cmd" | tr ' /.' '---' | tr -cd '[:alnum:]_-')"
    run_shell_capture "$label" "$cmd"
done

fips_log="$(find "$log_dir" -name 'group6-make-verify-fips.log' -print -quit)"
if [ -n "$fips_log" ] && grep -q '\[verify-fips\] FIPS build failed' "$fips_log"; then
    die "make verify-fips reported a non-FIPS-capable local toolchain; rerun on a FIPS-capable host"
fi

for cmd in "${group7_commands[@]}"; do
    label="group7-$(printf '%s' "$cmd" | tr ' /.' '---' | tr -cd '[:alnum:]_-')"
    run_shell_capture "$label" "$cmd"
done

capture_release_assets

for wf in "${workflow_files[@]}"; do
    run_gh_capture "$wf"
done

run_shell_capture "launch-external-status" \
    "LAUNCH_EXPECTED_NPM_VERSION=${tag} scripts/check-launch-external-status.sh --candidate-sha ${tag_sha}"

if [ "$trigger_release_smoke" = "1" ]; then
    run_capture "trigger-release-smoke" \
        gh workflow run release-smoke.yml -f "tag=${tag}"
fi

cat <<EOF

Evidence captured under:
  ${evidence_dir}

Next required human steps:
  1. Repeat this script on the other required release-check host if needed.
  2. Confirm release-smoke.yml is green for ${tag}; dispatch with
     --trigger-release-smoke if the release event did not run it.
  3. Archive or link the release-smoke-doctor-output artifact from the
     green release-smoke.yml run.
  4. Run docs/verification.md checks for required variants not sampled
     by release-smoke.yml, including the Group 7 FIPS variant evidence.
  5. After the candidate release path publishes, rerun
     scripts/check-launch-external-status.sh with --expected-npm-version
     and --fail-open before final sign-off.
  6. Add evidence URLs / workflow_run_id values to Groups 6 and 7 in
     docs/launch-candidate-checklist.md before checking boxes.
  7. Run make launch-checklist-parity after editing the checklist.
EOF
