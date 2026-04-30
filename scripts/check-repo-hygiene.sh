#!/usr/bin/env bash
#
# check-repo-hygiene.sh — PR-blocking hygiene gate.
#
# Fails when the repo accidentally tracks OS / editor / coverage
# artefacts. These files slip in through `git add .` and bloat the
# history without ever belonging in the repo. The ignore list in
# .gitignore keeps them out of future stages; this script keeps them
# out of HEAD.
#
# Offenders flagged:
#   .DS_Store / Thumbs.db / desktop.ini — OS-generated metadata
#   *.swp / *.swo / *~                  — editor swap/backup files
#   *.orig                              — merge leftovers
#   coverage.out / *.prof                — build/profile artefacts
#   *.test / *.exe                      — compiled Go test binaries and Windows executables
#
# Usage:
#   bash scripts/check-repo-hygiene.sh
#
# Exit codes:
#   0 — tree is clean
#   1 — one or more tracked junk files detected; script prints each
#       path and suggests `git rm --cached <path>` + an .gitignore
#       update if necessary.

set -euo pipefail

echo "== repo-hygiene =="

pattern='(^|/)(\.DS_Store|Thumbs\.db|desktop\.ini|coverage\.out)$|\.(swp|swo|orig|prof|test|exe)$|~$'

offenders=$(git ls-files | grep -E "$pattern" || true)

if [ -n "$offenders" ]; then
  echo >&2 "[fail] repo-hygiene: tracked junk files detected:"
  while IFS= read -r path; do
    echo "  $path" >&2
  done <<< "$offenders"
  echo >&2
  echo >&2 "       Remove with:"
  echo >&2 "         git rm --cached <path>"
  echo >&2 "         git commit -m 'chore: untrack <path>'"
  echo >&2 "       And verify .gitignore covers the file class."
  exit 1
fi

echo "repo-hygiene: OK"
