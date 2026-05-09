#!/usr/bin/env bash
#
# test-publish-npm.sh — regression test for scripts/publish-npm.sh.
#
# Locks the prerelease-tag contract that broke v1.2.1-rc.1's Release
# workflow at the "Publish npm packages" step: npm refuses to publish a
# prerelease version without an explicit --tag. The fix branch makes
# publish-npm.sh derive --tag from the semver prerelease identifier;
# this test pins the contract so a future refactor cannot regress to
# the v1.2.1-rc.1 failure mode.
#
# Cases:
#   1. v1.2.1-rc.2     → all 6 publishes pass --tag rc
#   2. v1.2.1          → no publish carries --tag (npm defaults to latest)
#   3. v2.0.0-beta.1   → all 6 publishes pass --tag beta
#   4. v2.0.0-alpha.1  → all 6 publishes pass --tag alpha
#   5. v1.0.0-pre.5    → all 6 publishes pass --tag next (catch-all)
#   6. V1.2.1-RC.3     → uppercase prerelease still routes to --tag rc
#                        (case-insensitive identifier matching)
#   7. v1.2.3-rc       → bare "rc" identifier (no .N) still routes to rc
#
# Each case sandboxes the script in a fresh tmp REPO_ROOT with a fake
# npm command that records its argv. No real npm publish happens.
#
# This test is a sibling to scripts/test-check-release-assets.sh in
# spirit (per-case tmp fixtures, exit-code + output assertions, pure
# bash, no external services). It is wired into the Makefile under
# the script-tests target.

set -euo pipefail

repo_root_real="$(cd "$(dirname "$0")/.." && pwd)"
script_under_test="$repo_root_real/scripts/publish-npm.sh"
template_real="$repo_root_real/npm/package.json.tmpl"
base_dir_real="$repo_root_real/npm/clockify-mcp-go"

if [ ! -x "$script_under_test" ]; then
  echo "FAIL: $script_under_test not executable" >&2
  exit 1
fi
if [ ! -f "$template_real" ]; then
  echo "FAIL: $template_real not found (real template required as a copy source)" >&2
  exit 1
fi
if [ ! -d "$base_dir_real" ]; then
  echo "FAIL: $base_dir_real not found (real base package required as a copy source)" >&2
  exit 1
fi

tmproot="$(mktemp -d)"
trap 'rm -rf "$tmproot"' EXIT

tests_run=0
tests_failed=0

# Per-case sandbox: copy the script to $sandbox/scripts/publish-npm.sh
# so the script's REPO_ROOT (resolved via $(dirname "$0")/..) lands at
# $sandbox. Provide a minimal npm/package.json.tmpl + base package +
# empty platform binaries so the script runs without touching the
# real repo. Inject a fake npm via PATH that just records argv.
build_sandbox() {
  local sandbox="$1"
  mkdir -p "$sandbox/scripts" "$sandbox/npm/clockify-mcp-go/bin" "$sandbox/bin"

  cp "$script_under_test" "$sandbox/scripts/publish-npm.sh"
  chmod +x "$sandbox/scripts/publish-npm.sh"

  cp "$template_real" "$sandbox/npm/package.json.tmpl"
  cp -R "$base_dir_real/." "$sandbox/npm/clockify-mcp-go/"

  for path in \
    "dist/clockify-mcp_darwin_arm64_v8.0/clockify-mcp" \
    "dist/clockify-mcp_darwin_amd64_v1/clockify-mcp" \
    "dist/clockify-mcp_linux_amd64_v1/clockify-mcp" \
    "dist/clockify-mcp_linux_arm64_v8.0/clockify-mcp" \
    "dist/clockify-mcp_windows_amd64_v1/clockify-mcp.exe"
  do
    mkdir -p "$sandbox/$(dirname "$path")"
    : > "$sandbox/$path"
  done

  cat > "$sandbox/bin/npm" <<'NPMEOF'
#!/usr/bin/env bash
# Mock npm — record argv on its own line and succeed.
# The real test harness greps the log for ^publish vs ^deprecate.
echo "$@" >> "${NPM_INVOCATIONS_LOG:?NPM_INVOCATIONS_LOG must be set}"
exit 0
NPMEOF
  chmod +x "$sandbox/bin/npm"
}

run_case() {
  local case_name="$1" version="$2" expected_tag="$3"
  tests_run=$((tests_run+1))

  local sandbox="$tmproot/case-$tests_run"
  build_sandbox "$sandbox"

  local invocations_log="$sandbox/npm-invocations.log"
  : > "$invocations_log"

  if ! NPM_INVOCATIONS_LOG="$invocations_log" \
       PATH="$sandbox/bin:$PATH" \
       bash "$sandbox/scripts/publish-npm.sh" "$version" \
       >"$sandbox/script.out" 2>"$sandbox/script.err"; then
    echo "FAIL [$case_name]: script exited non-zero (version=$version)"
    sed 's/^/    /' "$sandbox/script.err" >&2 || true
    tests_failed=$((tests_failed+1))
    return
  fi

  local publish_count
  publish_count=$(grep -c '^publish ' "$invocations_log" || true)
  if [ "$publish_count" -ne 6 ]; then
    echo "FAIL [$case_name]: expected exactly 6 'npm publish' invocations, got $publish_count"
    echo "  invocations log:" >&2
    sed 's/^/    /' "$invocations_log" >&2 || true
    tests_failed=$((tests_failed+1))
    return
  fi

  if [ "$expected_tag" = "-" ]; then
    if grep -q '^publish .*--tag' "$invocations_log"; then
      echo "FAIL [$case_name]: stable version $version should NOT pass --tag, but did"
      sed 's/^/    /' "$invocations_log" >&2 || true
      tests_failed=$((tests_failed+1))
      return
    fi
    local access_count
    access_count=$(grep -c '^publish --access public$' "$invocations_log" || true)
    if [ "$access_count" -ne 6 ]; then
      echo "FAIL [$case_name]: expected 6 'publish --access public' (no extras), got $access_count"
      sed 's/^/    /' "$invocations_log" >&2 || true
      tests_failed=$((tests_failed+1))
      return
    fi
  else
    local tagged_count
    tagged_count=$(grep -cE "^publish --access public --tag ${expected_tag}\$" "$invocations_log" || true)
    if [ "$tagged_count" -ne 6 ]; then
      echo "FAIL [$case_name]: expected 6 'publish --access public --tag $expected_tag', got $tagged_count"
      sed 's/^/    /' "$invocations_log" >&2 || true
      tests_failed=$((tests_failed+1))
      return
    fi
  fi

  echo "[pass] $case_name (version=$version → tag=${expected_tag/-/<none>})"
}

run_case "rc.2 prerelease"       "v1.2.1-rc.2"     "rc"
run_case "stable release"        "v1.2.1"          "-"
run_case "beta prerelease"       "v2.0.0-beta.1"   "beta"
run_case "alpha prerelease"      "v2.0.0-alpha.1"  "alpha"
run_case "unknown prerelease"    "v1.0.0-pre.5"    "next"
run_case "uppercase rc"          "V1.2.1-RC.3"     "rc"
run_case "bare rc identifier"    "v1.2.3-rc"       "rc"

echo
echo "publish-npm tests: $tests_run run, $tests_failed failed"
[ "$tests_failed" -eq 0 ]
