#!/usr/bin/env bash
#
# check-launch-checklist-parity.sh
#
# Keeps the public hosted launch checklist tied to actual CLI behaviour.
# The checklist is an operator gate, so command lines in it must
# reference flags the binary really implements *and* the strict
# prod-postgres doctor gate must remain visible.
#
# Two layers of defence:
#
# 1. Source greps — fast, no compile, catch obvious removals from
#    cmd/clockify-mcp source (parseDoctorArgs case statements,
#    backendDoctorFindings symbols).
# 2. Executable check — compile clockify-mcp, run --help, and assert
#    the help banner exposes every flag the checklist names. This
#    catches the case where a flag is parsed in source but never
#    documented (or removed from --help while leaving stale parse
#    code), which the source-grep layer cannot see. The execution
#    happens against a freshly-built binary so a refactor that breaks
#    --help fails the gate before a release ships.
#
# Both layers must pass — neither alone is sufficient.

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
  echo "launch-checklist-parity: FAIL" >&2
  exit 1
fi

# Layer 1: source greps. The checklist must require the strict
# prod-postgres doctor invocation, and any --check-backends mention
# must be backed by parseDoctorArgs + backendDoctorFindings in source.
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

# Layer 2: build the binary and verify --help advertises every flag the
# checklist mentions. This forecloses the "flag still parsed but
# silently undocumented" failure mode that the source-grep layer cannot
# detect.

BIN=""
HELP=""
cleanup() {
  if [ -n "${BIN:-}" ]; then rm -f "$BIN"; fi
  if [ -n "${HELP:-}" ]; then rm -f "$HELP"; fi
}
trap cleanup EXIT

if ! command -v go >/dev/null 2>&1; then
  err "Go toolchain not on PATH; cannot perform executable parity check"
else
  BIN="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-help.XXXXXX")"
  HELP="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-help-out.XXXXXX")"
  if ! go build -o "$BIN" ./cmd/clockify-mcp 2>/dev/null; then
    err "go build ./cmd/clockify-mcp failed; cannot perform executable parity check"
  else
    # printHelp() writes to stderr; redirect both streams so the test
    # is robust to a future move to stdout.
    "$BIN" --help >"$HELP" 2>&1 || true

    # Always-required: the doctor subcommand and the prod-postgres
    # profile must be discoverable from --help. Operators following the
    # checklist need to find them without reading the source.
    if ! grep -Fq "clockify-mcp doctor" "$HELP"; then
      err "clockify-mcp --help does not advertise the doctor subcommand"
    fi
    if ! grep -Fq "prod-postgres" "$HELP"; then
      err "clockify-mcp --help does not list the prod-postgres profile"
    fi
    if ! grep -Fq -- "--strict" "$HELP"; then
      err "clockify-mcp --help does not document --strict"
    fi
    if ! grep -Fq -- "--profile=" "$HELP"; then
      err "clockify-mcp --help does not document --profile=<name>"
    fi

    # Conditional: if the checklist mentions --check-backends, the
    # binary's --help must expose it too. This is the executable
    # counterpart to the source-grep on parseDoctorArgs above.
    if grep -Fq -- "--check-backends" "$CHECKLIST"; then
      if ! grep -Fq -- "--check-backends" "$HELP"; then
        err "checklist references --check-backends, but clockify-mcp --help does not document it"
      fi
    fi

    # Conditional: if the checklist mentions --allow-broad-policy, the
    # binary's --help must expose it too.
    if grep -Fq -- "--allow-broad-policy" "$CHECKLIST"; then
      if ! grep -Fq -- "--allow-broad-policy" "$HELP"; then
        err "checklist references --allow-broad-policy, but clockify-mcp --help does not document it"
      fi
    fi
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "launch-checklist-parity: FAIL" >&2
  exit 1
fi

echo "launch-checklist-parity: OK"
