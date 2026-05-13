# QA Agent 46 - cli-help-version-output

## Verdict
PASS WITH CONCERNS

## What I checked

- `--version` and `-v` output correctness
- `--help` and `-h` output completeness and correctness
- Version injection via ldflags (`git describe --tags`)
- Help text generation pipeline (spec.go → gen-config-docs → help_generated.go)
- Doctor subcommand exit codes (0/2/3) for known profiles, unknown profiles, strict posture
- `--help` output destination (stdout vs stderr)
- Arg-parsing robustness (position dependence, `=` syntax, unknown flags, `--profile` anywhere)
- Docker HEALTHCHECK correctness (`--version` exec form)
- Config-doc-parity gate (help_generated.go vs spec.go)
- Launch-checklist-parity gate (CLI flags in help vs checklist)
- Go unit test coverage for version/help paths (main_test.go)
- Stdio MCP server smoke test with live API credentials
- Live Clockify API workspace access probe

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (redacted)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — safety rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/README.md` — project overview
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/docs/safety-rules.md` — expanded safety rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/probes/lib/common.sh` — curl wrapper with auth

## Commands run

```bash
# Build
go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" \
  -o /tmp/clockify-mcp-qa46-fixed ./cmd/clockify-mcp

# Version output
/tmp/clockify-mcp-qa46 --version      # → v1.2.1-11-gabf9459
/tmp/clockify-mcp-qa46 -v             # → identical

# Help output
/tmp/clockify-mcp-qa46 --help 2>&1    # → 137 lines, ~15KB, all 5 profiles, all env vars
/tmp/clockify-mcp-qa46 -h 2>&1        # → identical to --help

# Doctor
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  clockify-mcp doctor                                  # → EXIT:0 (clean)
CLOCKIFY_API_KEY=<REDACTED> MCP_PROFILE=nonexistent \
  clockify-mcp doctor                                  # → EXIT:2 (Load error)
CLOCKIFY_API_KEY=<REDACTED> \
  clockify-mcp doctor --strict                         # → EXIT:3 (4 posture findings)

# Parity gates
bash scripts/check-config-doc-parity.sh                # → OK
bash scripts/check-launch-checklist-parity.sh           # → OK
go run ./cmd/gen-config-docs -mode=all                  # → no drift

# Unit tests
go test -race -count=1 ./cmd/clockify-mcp/...           # → OK

# Stdio MCP smoke
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  bash scripts/smoke-stdio.sh                           # → 40 tools, OK

# Live API probe
curl -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>"  # → HTTP 200
```

## Live API probes run

| Probe | Endpoint | Purpose | Result |
|-------|----------|---------|--------|
| Workspace access | `GET /workspaces/{id}` | Verify API key + workspace ID validity | HTTP 200, "WORKSPACE" workspace confirmed |
| Stdio MCP smoke | `initialize` + `tools/list` | MCP server startup with live credentials | 40 tools returned, serverInfo.name=clockify-go-mcp |
| Doctor with live creds | `doctor` subcommand | Config audit with real credentials | EXIT:0, all vars attributed correctly, secret redacted |
| Doctor strict posture | `doctor --strict` | Hosted-strict posture audit with real env | EXIT:3, 4 posture findings flagged correctly |

## Findings

### F1: Double "v" prefix in help banner (P1) — FIXED

**Location**: `cmd/clockify-mcp/main.go:234`

**Description**: `printHelp()` used `"clockify-mcp v%s"` as the format string. Since `git describe --tags` already prefixes tags with "v" (e.g., `v1.2.1-11-gabf9459`), and `debug.ReadBuildInfo().Main.Version` also includes the "v" prefix, the help banner displayed a double "v":
- Before fix: `clockify-mcp vv1.2.1-11-gabf9459 — MCP server for Clockify`
- After fix: `clockify-mcp v1.2.1-11-gabf9459 — MCP server for Clockify`

**Fix**: Removed the redundant "v" from the format string. `--version`/`-v` output was not affected (it prints `effectiveVersion()` directly without prepending anything).

### F2: --version and --help only checked at position 1 (P2)

**Location**: `cmd/clockify-mcp/main.go:69-79`

**Description**: The argument dispatch uses `os.Args[1]` for `--version`, `-v`, `--help`, `-h`, and `doctor`. However, `--profile=<name>` is scanned across all positions (lines 63-67). This creates an asymmetry:

```bash
clockify-mcp --version                              # ✓ prints version
clockify-mcp --profile=local-stdio --version         # ✗ tries to start server (needs API key)
clockify-mcp doctor --version                       # ✗ runs doctor (treats --version as unknown doctor arg)
clockify-mcp --version --profile=local-stdio         # ✓ prints version (--version at pos 1)
```

Users familiar with GNU-style flag parsing (where flags can appear anywhere) will find this surprising. The Docker HEALTHCHECK (`CMD ["/usr/local/bin/clockify-mcp", "--version"]`) is unaffected since it only passes `--version`.

**Recommendation**: Consider scanning all args for `--version`, `--help`, and `-v`/`-h` in the same loop that already scans for `--profile=`, so they work from any position. This is a backwards-compatible change.

### F3: --help writes to stderr, --version writes to stdout (P2)

**Location**: `cmd/clockify-mcp/main.go:228-248` (`printHelp`) vs lines 71-73 (`--version`)

**Description**: `--version` outputs to stdout (`fmt.Println`), while `--help` outputs to stderr (`os.Stderr`). This means:

```bash
clockify-mcp --version > version.txt   # ✓ works, file contains version
clockify-mcp --help > help.txt         # ✗ file is empty, help went to stderr
clockify-mcp --help 2>&1 | grep doctor # ✓ works but requires 2>&1
```

The Docker HEALTHCHECK only relies on exit code, so it's unaffected. However, CI scripts and operators piping help output through grep/pagers need `2>&1`. The codebase's own `check-launch-checklist-parity.sh` correctly uses `--help >"$HELP" 2>&1` (line 84).

**Analysis**: Writing help to stderr is a defensible choice (keeps stdout clean for data output), but `--version` going to stdout creates inconsistency. Standard Unix convention is to write both to stdout for `--version`/`--help`.

### F4: --version= syntax not supported (P3)

**Location**: `cmd/clockify-mcp/main.go:71`

**Description**: `--version=v1.0.0` is not recognized as a version flag. The code uses exact string matching (`os.Args[1] == "--version"`), not prefix matching. This means `--version=...` falls through to server startup, which fails with "CLOCKIFY_API_KEY is required".

### F5: doctor subcommand has no --help (P3)

**Location**: `cmd/clockify-mcp/doctor.go` — `parseDoctorArgs()` only recognizes `--strict`, `--allow-broad-policy`, `--check-backends`, `--profile=`

**Description**: Running `clockify-mcp doctor --help` runs the full doctor report rather than showing doctor-specific help. The doctor usage is documented in the main `--help` output, but there's no shorthand to get just the doctor usage.

### F6: Doctor doesn't validate API connectivity (P3)

**Location**: `cmd/clockify-mcp/doctor.go`

**Description**: The doctor subcommand audits config loading but doesn't verify that the API key is valid or that the workspace is reachable. An invalid API key is loaded without error (it's just a string). The MCP server itself would fail at first tool call, but the doctor can't warn about it at config-audit time.

This is a deliberate tradeoff — adding live connectivity checks would make the doctor slower and could fail transiently on network issues.

## Fixes made

### Fix 1: Remove double "v" in help banner

**File**: `cmd/clockify-mcp/main.go:234`

**Change**:
```diff
-_, _ = fmt.Fprintf(w, "clockify-mcp v%s — MCP server for Clockify\n\n", effectiveVersion())
+_, _ = fmt.Fprintf(w, "clockify-mcp %s — MCP server for Clockify\n\n", effectiveVersion())
```

**Rationale**: Both `git describe --tags` and `debug.ReadBuildInfo().Main.Version` include the "v" prefix. Prepending another "v" produces `vv1.2.1` which is incorrect. The version displayed by `--version` is unaffected (it prints the raw version string).

**Verification**: All Go tests pass. `check-launch-checklist-parity.sh` passes. `check-config-doc-parity.sh` passes. No other code references the old format string.

## Reproduction steps for each issue

### F1 (FIXED)
```bash
go build -ldflags "-X main.version=v1.0.0" -o clockify-mcp ./cmd/clockify-mcp
./clockify-mcp --help 2>&1 | head -1
# Before fix: clockify-mcp vv1.0.0 — MCP server for Clockify
# After fix:  clockify-mcp v1.0.0 — MCP server for Clockify
```

### F2
```bash
./clockify-mcp --profile=local-stdio --version
# Expected: prints version
# Actual: error: CLOCKIFY_API_KEY is required (tries to start server)
```

### F3
```bash
./clockify-mcp --help > /tmp/help.txt && wc -c /tmp/help.txt
# Output: 0 (help went to stderr)
./clockify-mcp --version > /tmp/ver.txt && wc -c /tmp/ver.txt
# Output: non-zero (version went to stdout)
```

### F4
```bash
./clockify-mcp --version=v1.0.0
# Expected: prints version
# Actual: error: CLOCKIFY_API_KEY is required
```

### F5
```bash
./clockify-mcp doctor --help 2>&1 | head -3
# Shows full doctor report, not doctor-specific help
```

## Cleanup performed

No test resources were created in the live workspace — all tests were read-only (workspace info fetch, MCP server stdio smoke test listing tools). No cleanup needed.

## Leftover test resources

None.

## Severity

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| F1 | P1 | Double "v" prefix in help banner | FIXED |
| F2 | P2 | --version/--help only checked at position 1 | Documented |
| F3 | P2 | --help writes to stderr, --version to stdout | Design choice |
| F4 | P3 | --version= syntax not supported | Documented |
| F5 | P3 | Doctor has no --help flag | Documented |
| F6 | P3 | Doctor doesn't validate API connectivity | Tradeoff |

## Files changed

- `cmd/clockify-mcp/main.go:234` — Removed redundant "v" from help banner format string

## Suggested next action

1. **Merge the F1 fix** — it's a one-line change, tests pass, no downstream impact.
2. **Consider F2** — scan all args for `--version`/`--help`/`-v`/`-h` alongside the existing `--profile=` scan. Low risk, high UX payoff for CLI users.
3. **Consider F4** — use `strings.HasPrefix` instead of `==` for `--version` and `--help` to accept `--version=value` as equivalent to `--version`.
4. **Add Go unit test for printHelp()** — currently only bash-level smoke tests exercise the help output format. A simple Go test checking that the banner line doesn't contain "vv" would have caught F1.

## False positives / uncertainty

- The `--help`→stderr behavior (F3) is a deliberate design choice, not a bug. I've documented it for awareness but don't recommend changing it.
- F4 (`--version=` syntax) is not commonly used in practice. Most users invoke `--version` or `-v` without `=`.
- The "dirty" suffix in the version string (`v1.2.1-11-gabf9459-dirty`) in these tests is due to the uncommitted fix in the working tree. A clean build would omit `-dirty`.

## Final recommendation

**Accept the fix (F1) and proceed.** The double-v bug is a minor but real defect in the help banner. All other findings are either documented design choices or low-severity UX improvements that can be addressed in a future polish pass. The CLI help/version output is otherwise correct, complete, and well-structured.
