# QA Agent 07 - doctor-command-output

Status: COMPLETED
Completed UTC: 2026-05-10T21:30:00Z

Worktree: /Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-07
Live API probe lab: /Users/15x/Downloads/WORKING/clockify-api-probe-lab

## Verdict
PASS

## What I checked

The `clockify-mcp doctor` subcommand audits effective configuration, attributes the source of every spec'd env var (explicit | profile | default | empty), and reports any Load() errors or hosted-service posture violations via `--strict`. I verified correctness across:

1. **Basic invocation**: `doctor` with no flags, minimal env (API key + workspace). Verified exit 0, all groups rendered, sources attributed correctly.
2. **Strict posture gate**: `doctor --strict` against the default stdio config. Verified 4 error findings (MCP_DISABLE_INLINE_SECRETS, MCP_CONTROL_PLANE_DSN, MCP_AUDIT_DURABILITY, CLOCKIFY_POLICY) and exit 3.
3. **Profile application**: `doctor --strict` with `--profile=prod-postgres` + required OIDC/postgres env. Verified profile-defaulted keys show source "profile", strict posture passes (no findings), exit 0.
4. **--allow-broad-policy**: Verified the flag suppresses only the CLOCKIFY_POLICY finding, leaving other strict errors intact.
5. **--check-backends**: Verified the flag auto-enables `--strict` and adds backend-specific findings. Without postgres tags, reports the correct build-tag guidance.
6. **Config load errors**: Verified invalid profile name and invalid enum values produce exit 2 with the exact Load() error text displayed.
7. **Secret redaction**: Verified API_KEY, BEARER_TOKEN, DSN passwords, TLS key paths, and JWKS paths are rendered as `set (redacted)`. Workspace IDs (not marked sensitive) are shown in plaintext.
8. **Source attribution**: Verified the 4 source categories (explicit, profile, default, empty) are correctly assigned. Profile-populated defaults are correctly distinguished from operator-explicit values.
9. **--help integration**: Verified `doctor` appears in the usage banner with flag documentation.
10. **Live API connectivity**: Verified the Clockify API key and workspace are valid by calling `GET /workspaces/{id}` and `GET /user` directly.
11. **MCP server smoke test**: Started the server in HTTP mode, verified `initialize` returns correct serverInfo/capabilities/instructions, `tools/list` returns 40 tools, `tools/call` executes correctly.
12. **Docker image build and smoke test**: Built the Docker image, ran `doctor --strict` inside the container. Verified it correctly surfaces Load() errors caused by the image's default strict settings.
13. **Unit tests**: Ran all 25 doctor-specific tests (`TestDoctor*`) — all pass.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, workspace confirmation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — safety rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/README.md` — project layout
- API key and workspace ID from the lab; no other credential source was used

## Commands run

```bash
# Basic doctor
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp/ doctor

# Doctor with strict posture
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp/ doctor --strict

# Doctor with profile (prod-postgres + strict)
MCP_PROFILE=prod-postgres \
  MCP_CONTROL_PLANE_DSN="postgres://<REDACTED>@localhost:5432/clockify?sslmode=disable" \
  MCP_OIDC_ISSUER="https://issuer.example.com" MCP_OIDC_AUDIENCE="clockify-mcp" \
  CLOCKIFY_POLICY=time_tracking_safe MCP_DEFAULT_TENANT_ID="prod-fallback-disabled" \
  go run ./cmd/clockify-mcp/ doctor --strict

# Doctor with invalid profile
MCP_PROFILE=bogus-profile go run ./cmd/clockify-mcp/ doctor

# Doctor with invalid config value
MCP_AUDIT_DURABILITY=invalid_value go run ./cmd/clockify-mcp/ doctor

# Doctor --check-backends (non-postgres build)
go run ./cmd/clockify-mcp/ doctor --check-backends

# Built binary tests
go build -o /tmp/clockify-mcp-test ./cmd/clockify-mcp/
/tmp/clockify-mcp-test doctor --strict          # exit 3
/tmp/clockify-mcp-test doctor                   # exit 0

# Unit tests
go test ./cmd/clockify-mcp/ -run TestDoctor -v -count=1

# Live API probes
curl -s -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/workspaces/<REDACTED>"
curl -s -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/user"

# MCP server smoke test
MCP_TRANSPORT=http MCP_BEARER_TOKEN="<REDACTED>" MCP_AUTH_MODE=static_bearer \
  MCP_HTTP_BIND="127.0.0.1:19882" MCP_HTTP_LEGACY_POLICY=allow /tmp/clockify-mcp-test &
curl -X POST http://127.0.0.1:19882/mcp -H "Content-Type: application/json" \
  -H "Authorization: Bearer <REDACTED>" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"qa-test","version":"1.0.0"},"capabilities":{}}}'

# Docker build and smoke
docker build -f deploy/Dockerfile -t clockify-mcp-qa-test .
docker run --rm -e CLOCKIFY_API_KEY=<REDACTED> -e CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  -e MCP_OIDC_ISSUER="https://issuer.example.com" -e MCP_OIDC_AUDIENCE="clockify-mcp" \
  -e CLOCKIFY_POLICY=time_tracking_safe -e MCP_DISABLE_INLINE_SECRETS=1 \
  -e MCP_CONTROL_PLANE_DSN="postgres://localhost/mcp" -e MCP_AUDIT_DURABILITY=fail_closed \
  clockify-mcp-qa-test doctor --strict
```

## Live API probes run

| Probe | Endpoint | Result | Notes |
|-------|----------|--------|-------|
| Workspace lookup | GET /workspaces/{id} | 200 | Confirmed workspace "WORKSPACE" is valid |
| User info | GET /user | 200 | Confirmed API key is valid |
| MCP initialize | POST /mcp JSON-RPC | 200 | serverInfo, capabilities, protocol version, instructions all correct |
| MCP tools/list | POST /mcp JSON-RPC | 200 | 40 tools returned |
| MCP tools/call | POST /mcp JSON-RPC (clockify_current_user) | 200 | Tool execution works |

## Findings

### Finding 1: Doctor correctly handles all config states (PASS)
All tested combinations of valid/invalid config, profiles, and flags produce the correct exit codes and output. The source attribution (explicit/profile/default/empty) is accurate, even when a profile populates defaults that an operator did not set.

### Finding 2: Secret redaction is comprehensive (PASS)
All 8 Sensitive-marked specs (CLOCKIFY_API_KEY, MCP_BEARER_TOKEN, MCP_METRICS_BEARER_TOKEN, MCP_HTTP_INLINE_METRICS_BEARER_TOKEN, MCP_CONTROL_PLANE_DSN, MCP_GRPC_TLS_KEY, MCP_HTTP_TLS_KEY, MCP_OIDC_JWKS_PATH) are rendered as `set (redacted)` when non-empty. Non-sensitive values like workspace IDs are shown in plaintext, which is correct per their spec definitions.

### Finding 3: --check-backends produces duplicate MCP_CONTROL_PLANE_DSN errors when DSN is not postgres (P3)
When `--check-backends` runs without a postgres DSN, the output contains two `MCP_CONTROL_PLANE_DSN` error rows. The messages differ slightly but appear redundant. The behavior is correct — both gates independently require the DSN — but having two similar lines for the same key could cause confusion.

### Finding 4: Docker image defaults require additional config for strict posture (P3, informational)
The Docker image sets `MCP_STRICT_HOST_CHECK=1` with `MCP_HTTP_BIND=0.0.0.0:8080`. Without `MCP_ALLOWED_ORIGINS` or `MCP_ALLOW_ANY_ORIGIN=1`, this causes `Load()` to fail with a clear actionable error. This is intentional and safe, but means the Docker image cannot pass `doctor --strict` out of the box without additional env configuration.

### Finding 5: auth= display when transport is stdio (P3, informational)
When transport is stdio (default), the `Load() result` line shows `auth=;` (empty auth mode). While technically correct (stdio has no auth mode), the empty value could confuse operators unfamiliar with the transport/auth matrix.

## Fixes made

No code fixes were made. The doctor command output is correct and comprehensive. The findings above are minor UX observations (P3) that do not affect correctness or safety.

## Reproduction steps for each issue

### Finding 3 — Duplicate DSN errors:
```bash
CLOCKIFY_API_KEY=<REDACTED> go run ./cmd/clockify-mcp/ doctor --check-backends 2>&1 | grep MCP_CONTROL_PLANE_DSN
# Observe two MCP_CONTROL_PLANE_DSN lines with slightly different messages
```

### Finding 4 — Docker strict defaults:
```bash
docker build -f deploy/Dockerfile -t clockify-mcp-qa .
docker run --rm clockify-mcp-qa doctor 2>&1 | head -5
# Observe Load() error about MCP_STRICT_HOST_CHECK + MCP_ALLOWED_ORIGINS
```

### Finding 5 — Empty auth display:
```bash
go run ./cmd/clockify-mcp/ doctor 2>&1 | grep "Load() result"
# Observe "auth=;" in the output
```

## Cleanup performed

- Removed temporary binaries: `/tmp/clockify-mcp-test`
- Removed temporary log files: `/tmp/mcp-server-*.log`, `/tmp/mcp-err*.log`
- Stopped background MCP server processes on ports 19876-19883
- Docker test image removed: `docker rmi clockify-mcp-qa-test`

## Leftover test resources

No test resources were created on the live Clockify workspace. All probes were read-only.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| Duplicate DSN errors in --check-backends | P3 | Minor UX; no functional impact |
| Docker image strict defaults | P3 | Intentional design; clear error message |
| Empty auth= display for stdio | P3 | Cosmetic; correct behavior |

## Files changed

None. No code changes were required.

## Suggested next action

1. Consider de-duplicating the `MCP_CONTROL_PLANE_DSN` error display in `--check-backends` mode by having `backendDoctorFindings` skip the DSN-scheme check when `strictDoctorFindings` already flagged it.
2. Optionally add `--profile=prod-postgres` with basic OIDC env to the Docker smoke test in CI to prove the strict posture is reachable from the container image.
3. Consider displaying `auth=N/A` instead of `auth=;` when transport is stdio and no auth mode is configured.

## False positives / uncertainty

- The `EXIT=1` observed in some `go run` shell pipelines is a shell artifact (SIGPIPE from `| head` or similar), not a doctor bug. Exit codes verified with the built binary are always correct (0/2/3).
- The `doctor --check-backends` test could not exercise the actual Postgres connection path because no Postgres instance was available. The build-time constraint (missing `-tags=postgres`) is correctly reported.

## Final recommendation

The `clockify-mcp doctor` command is production-ready for the `doctor-command-output` area. It correctly audits all 50+ env vars, attributes sources accurately, redacts secrets, and produces actionable posture findings. The 25 tests covering strict posture gates, secret redaction, auth-mode-specific gates (mTLS, forward_auth, OIDC), and error/edge cases all pass. The three P3 observations are cosmetic and do not affect correctness.
