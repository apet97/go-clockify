#!/usr/bin/env bash
#
# check-launch-checklist-parity.sh
#
# Keeps the public hosted launch checklist tied to CLI behavior. The
# checklist is an operator gate, so command lines in it must reference
# implemented flags and must keep the strict prod-postgres doctor gate.

set -euo pipefail

CHECKLIST="docs/release/public-hosted-launch-checklist.md"
CMD_DIR="cmd/clockify-mcp"
fail=0

err() {
  echo "[fail] launch-checklist-parity: $*" >&2
  fail=1
}

if [ ! -f "$CHECKLIST" ]; then
  err "missing checklist: $CHECKLIST"
else
  if ! grep -Eq '`clockify-mcp doctor --profile=prod-postgres --strict`[[:space:]]+exits 0' "$CHECKLIST"; then
    err "checklist must require \`clockify-mcp doctor --profile=prod-postgres --strict\` exits 0"
  fi

  if grep -Fq -- "--check-backends" "$CHECKLIST"; then
    if ! grep -Rqs -- 'case a == "--check-backends"' "$CMD_DIR"; then
      err "checklist references --check-backends, but cmd/clockify-mcp does not parse it"
    fi
    if ! grep -Rqs -- "checkBackends" "$CMD_DIR"; then
      err "checklist references --check-backends, but cmd/clockify-mcp has no checkBackends option"
    fi
    if ! grep -Rqs -- "func backendDoctorFindings" "$CMD_DIR"; then
      err "checklist references --check-backends, but cmd/clockify-mcp has no backendDoctorFindings implementation"
    fi
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "launch-checklist-parity: FAIL" >&2
  exit 1
fi

echo "launch-checklist-parity: OK"
