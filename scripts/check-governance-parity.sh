#!/bin/sh
#
# check-governance-parity.sh — fail when GOVERNANCE.md claims controls
# that docs/branch-protection.md says are not currently enforced.

set -eu

branch_doc="docs/branch-protection.md"
governance_doc="GOVERNANCE.md"

fail() {
  echo "governance-parity: FAIL — $*" >&2
  exit 1
}

require_branch_line() {
  pattern=$1
  label=$2
  if ! grep -Eq "$pattern" "$branch_doc"; then
    fail "branch-protection snapshot missing canonical line: $label"
  fi
}

require_branch_line '^\| Required approvals[[:space:]]*\|[[:space:]]*0[[:space:]]*\|' 'Required approvals: 0'
require_branch_line '^\| Require review from Code Owners[[:space:]]*\|[[:space:]]*Disabled[[:space:]]*\|' 'Require review from Code Owners: Disabled'
require_branch_line '^\| Require signed commits[[:space:]]*\|[[:space:]]*Disabled[[:space:]]*\|' 'Require signed commits: Disabled'
require_branch_line '^\| Enforce for admins[[:space:]]*\|[[:space:]]*Disabled[[:space:]]*\|' 'Enforce for admins: Disabled'

current_state=$(awk '
  /^## Current state/ { in_section = 1; next }
  /^## Target state/ { in_section = 0 }
  in_section { print }
' "$governance_doc")

if [ -z "$current_state" ]; then
  fail "GOVERNANCE.md is missing a current-state section"
fi

require_current_line() {
  needle=$1
  if ! printf '%s\n' "$current_state" | grep -Fq "$needle"; then
    fail "GOVERNANCE.md current-state section missing: $needle"
  fi
}

forbid_current_pattern() {
  pattern=$1
  label=$2
  if printf '%s\n' "$current_state" | grep -Eiq "$pattern"; then
    fail "GOVERNANCE.md current-state section contradicts branch protection: $label"
  fi
}

require_current_line 'Required approvals: 0 enforced.'
require_current_line 'Code-owner reviews: disabled.'
require_current_line 'Signed commits: disabled.'
require_current_line 'Admin enforcement: disabled.'

forbid_current_pattern 'Required approvals:[[:space:]]*[1-9][0-9]*' 'required approvals are non-zero'
forbid_current_pattern 'Code-owner reviews:[[:space:]]*(required|enabled|enforced)' 'CODEOWNERS review is required'
forbid_current_pattern 'Require review from Code Owners:[[:space:]]*(required|enabled|enforced)' 'CODEOWNERS review is required'
forbid_current_pattern 'Signed commits:[[:space:]]*(required|enabled|enforced)' 'signed commits are required'
forbid_current_pattern 'Require signed commits:[[:space:]]*(required|enabled|enforced)' 'signed commits are required'
forbid_current_pattern 'Admin enforcement:[[:space:]]*(required|enabled|enforced)' 'admin enforcement is enabled'
forbid_current_pattern 'Enforce for admins:[[:space:]]*(required|enabled|enforced)' 'admin enforcement is enabled'

echo "governance-parity: OK"
