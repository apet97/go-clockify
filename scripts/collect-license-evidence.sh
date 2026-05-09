#!/usr/bin/env bash
#
# collect-license-evidence.sh
#
# Produces raw dependency/license evidence for the B.08 brand/legal
# review without making legal conclusions. The inventory is based on
# actual package dependency graphs (`go list -deps`) for release-relevant
# build variants, not `go list -m all`, which over-reports workspace
# modules under go.work.

set -euo pipefail

repo_root="$(pwd)"
mode="run"
fail_missing_license=0

usage() {
  cat <<'EOF'
Usage:
  scripts/collect-license-evidence.sh [--repo-root DIR] [--fail-missing-license]
  scripts/collect-license-evidence.sh --plan

Options:
  --repo-root DIR          Repository root to inspect. Default: current directory.
  --fail-missing-license   Exit 1 if any non-main module lacks a local
                           LICENSE, NOTICE, or COPYING candidate file.
  --plan                   Print the evidence commands without running go list.
  -h, --help               Show this help.

This helper prints raw dependency and license-file evidence only. It is
not legal advice, does not classify licenses, and does not close the
brand/legal approval gate.
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
    --fail-missing-license)
      fail_missing_license=1
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

variants=(
  "default||./cmd/clockify-mcp"
  "fips|fips|./cmd/clockify-mcp"
  "postgres|postgres|./cmd/clockify-mcp"
  "grpc|grpc|./cmd/clockify-mcp"
  "grpc-postgres|grpc,postgres|./cmd/clockify-mcp"
  "otel|otel|./cmd/clockify-mcp"
)

if [ "$mode" = "plan" ]; then
  cat <<EOF
License evidence plan for ${repo_root}

Read-only checks:
  - For each build variant, run go list -deps against ./cmd/clockify-mcp.
  - Variants:
EOF
  for spec in "${variants[@]}"; do
    IFS='|' read -r name tags pkg <<< "$spec"
    if [ -n "$tags" ]; then
      printf '    - %s: go list -deps -tags=%q -f <module fields> %s\n' "$name" "$tags" "$pkg"
    else
      printf '    - %s: go list -deps -f <module fields> %s\n' "$name" "$pkg"
    fi
  done
  cat <<'EOF'
  - For each module returned by the package graph, locate local
    LICENSE*, NOTICE*, or COPYING* candidate files in that module root.

The output is a raw evidence inventory for legal/product review. It
does not classify licenses, does not inspect source headers, does not
scan npm transitive dependencies, and does not close B.08/L-10.
EOF
  exit 0
fi

command -v go >/dev/null 2>&1 || die "go is required for license evidence collection"

license_candidates() {
  local dir="$1"
  [ -n "$dir" ] || return 0
  [ -d "$dir" ] || return 0
  find "$dir" -maxdepth 1 -type f \
    \( -iname 'LICENSE' -o -iname 'LICENSE.*' -o -iname 'LICENSE-*' \
      -o -iname 'NOTICE' -o -iname 'NOTICE.*' -o -iname 'NOTICE-*' \
      -o -iname 'COPYING' -o -iname 'COPYING.*' -o -iname 'COPYING-*' \) \
    -print 2>/dev/null | sort
}

display_path() {
  local path="$1"
  case "$path" in
    "$repo_root"/*) printf '%s' "${path#"$repo_root"/}" ;;
    "$repo_root") printf '.' ;;
    *) printf '%s' "$path" ;;
  esac
}

go_list_modules() {
  local tags="$1"
  local pkg="$2"
  if [ -n "$tags" ]; then
    go list -deps -tags="$tags" \
      -f '{{with .Module}}{{.Path}}|{{.Version}}|{{.Dir}}|{{.Main}}{{end}}' \
      "$pkg"
  else
    go list -deps \
      -f '{{with .Module}}{{.Path}}|{{.Version}}|{{.Dir}}|{{.Main}}{{end}}' \
      "$pkg"
  fi | sed '/^$/d' | sort -u
}

printf 'License evidence for %s\n' "$repo_root"
printf 'Generated UTC: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
cat <<'EOF'
Scope: raw Go package dependency graphs and local license-file
candidates for build variants. This is not legal advice and does not
approve trademark, partnership, product, or license posture.

EOF

open_count=0
unknown_count=0

for spec in "${variants[@]}"; do
  IFS='|' read -r name tags pkg <<< "$spec"
  printf '## Variant: %s\n' "$name"
  printf 'package: %s\n' "$pkg"
  printf 'tags: %s\n' "${tags:-<none>}"

  modules_tmp="$(mktemp "${TMPDIR:-/tmp}/license-evidence-modules.XXXXXX")"
  if ! (cd "$repo_root" && go_list_modules "$tags" "$pkg" > "$modules_tmp"); then
    printf '[unknown] go list failed for variant %s\n\n' "$name"
    unknown_count=$((unknown_count + 1))
    rm -f "$modules_tmp"
    continue
  fi

  module_count="$(sed '/^$/d' "$modules_tmp" | wc -l | tr -d '[:space:]')"
  printf 'module_count: %s\n' "$module_count"

  while IFS='|' read -r module version dir main_flag; do
    [ -n "$module" ] || continue
    if [ -z "$version" ]; then
      version="<main>"
    fi
    printf 'module: %s\n' "$module"
    printf '  version: %s\n' "$version"
    printf '  main_module: %s\n' "${main_flag:-false}"
    printf '  dir: %s\n' "$(display_path "$dir")"

    candidates="$(license_candidates "$dir" || true)"
    if [ -z "$candidates" ] && [ "${main_flag:-false}" = "true" ] &&
       [ -n "$dir" ] && [ -f "$repo_root/LICENSE" ]; then
      case "$dir" in
        "$repo_root"|"$repo_root"/*) candidates="$repo_root/LICENSE" ;;
      esac
    fi
    if [ -n "$candidates" ]; then
      printf '  license_candidates:\n'
      while IFS= read -r candidate; do
        [ -n "$candidate" ] || continue
        printf '    - %s\n' "$(display_path "$candidate")"
      done <<< "$candidates"
    else
      printf '  license_candidates: <none found>\n'
      if [ "${main_flag:-false}" != "true" ]; then
        open_count=$((open_count + 1))
      fi
    fi
  done < "$modules_tmp"

  rm -f "$modules_tmp"
  printf '\n'
done

printf 'Summary: %d module(s) without local license candidates, %d unknown variant(s)\n' \
  "$open_count" "$unknown_count"

if [ "$unknown_count" -ne 0 ]; then
  exit 1
fi

if [ "$fail_missing_license" = "1" ] && [ "$open_count" -ne 0 ]; then
  exit 1
fi
