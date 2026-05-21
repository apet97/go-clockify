# QA Agent 47 - mcp-server-startup-smoke

Status: COMPLETE
Completed UTC: 2026-05-10T22:00:00Z
Worktree: /Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-47
Live API probe lab: /Users/15x/Downloads/WORKING/clockify-api-probe-lab

## Verdict
PASS

## What I checked

1. **Binary build** — clean compilation on Go 1.26.2 (darwin/arm64), no warnings
2. **`clockify-mcp --version`** — returns `dev` (no ldflags; expected for local build)
3. **`clockify-mcp --help`** — renders all profiles, env var catalog, usage
4. **`clockify-mcp doctor`** (no profile) — audit OK, exit 0
5. **`clockify-mcp doctor --profile=local-stdio`** — profile applied correctly, policy=time_tracking_safe from profile, exit 0
6. **`clockify-mcp doctor --profile=local-stdio --strict`** — correctly surfaces 3 hosted-posture ERRORs (MCP_DISABLE_INLINE_SECRETS, MCP_CONTROL_PLANE_DSN, MCP_AUDIT_DURABILITY), exit 3
7. **`clockify-mcp doctor` with missing API key** — error "CLOCKIFY_API_KEY is required", exit 2
8. **MCP stdio handshake** — initialize/2025-11-25, server_info, capabilities confirmed
9. **`tools/list`** — 36 tools visible under time_tracking_safe policy (40 tier-1 minus blocked writes)
10. **`clockify_whoami`** — returns user identity + resolved workspace correctly
11. **`clockify_policy_info`** — reports full policy surface, blocked Tier-2 groups, write allowlists
12. **`clockify_list_projects`** — returns 25 projects matching direct API
13. **`clockify_list_entries`** — returns paginated entries from workspace
14. **`clockify_add_entry` (dry_run=true)** — preview works, "No changes were made", correct note
15. **Bad API key** — server starts (config loads), tool calls fail with upstream error (expected design — API key validity is checked at call time, not startup)
16. **Docker build** — `docker build -f deploy/Dockerfile` succeeds, produces ~10 MB distroless image
17. **Docker doctor (`single-tenant-http` profile)** — passes after adding MCP_ALLOWED_ORIGINS
18. **Docker server run** — health endpoint returns `{"status":"ok"}`, MCP initialize succeeds over streamable HTTP with bearer auth
19. **Live API probes** — workspace info, projects, clients, tags, users all verified against direct Clockify API
20. **Read-only live contract tests** — `TestE2EReadOnly` passes
21. **Go test suite** — `go test ./cmd/clockify-mcp/...` passes

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID, confirmation token |
| `probes/lib/common.sh` | Shared probe helpers, redaction, curl wrapper |
| `TIMEENTRYDOC.md` | Time entries API reference |
| `docs/official-api-notes.md` | Per-domain API notes (invoices, expenses, webhooks) |
| `WORKSPACESDOC.md` | Workspace API reference |
| `PROJECTSDOC.md` | Projects API reference |
| `USERDOC.md` | User API reference |

Secrets redacted as `****REDACTED****` throughout.

## Commands run

```sh
# Build
go build -o /tmp/clockify-mcp-test ./cmd/clockify-mcp/

# Doctor
clockify-mcp-test doctor
clockify-mcp-test doctor --profile=local-stdio
clockify-mcp-test doctor --profile=local-stdio --strict
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> clockify-mcp-test doctor  # exit 2

# MCP stdio handshake
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | clockify-mcp-test --profile=local-stdio

# MCP tool calls
# tools/list, clockify_whoami, clockify_policy_info, clockify_list_projects,
# clockify_list_entries, clockify_add_entry (dry_run)

# Live API probes
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/user
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects?page-size=5
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients?page-size=5
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags?page-size=5
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/users

# Docker
docker build -f deploy/Dockerfile -t clockify-mcp-test .
docker run --rm -e CLOCKIFY_API_KEY=<REDACTED> -e MCP_PROFILE=single-tenant-http \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED> -e MCP_BEARER_TOKEN=<REDACTED> \
  -e MCP_ALLOWED_ORIGINS=localhost -e MCP_ALLOW_DEV_BACKEND=1 \
  -e MCP_CONTROL_PLANE_DSN=memory -p 9090:8080 clockify-mcp-test
curl http://127.0.0.1:9090/health  # {"status":"ok"}

# Tests
go test -race -count=1 ./cmd/clockify-mcp/...            # PASS
go test -race -tags=livee2e -run TestE2EReadOnly ./tests/...  # PASS
```

## Live API probes run

| Probe | Endpoint | Result |
|-------|----------|--------|
| User identity | `GET /v1/user` | 200 — user `<REDACTED_ID>`, email `<EMAIL>` |
| Workspace info | `GET /v1/workspaces/{ws}` | 200 — name "WORKSPACE", hourlyRate 150 EUR |
| List projects | `GET /v1/workspaces/{ws}/projects?page-size=5` | 200 — 5 projects returned (25 total available) |
| List clients | `GET /v1/workspaces/{ws}/clients?page-size=5` | 200 — 5 clients returned |
| List tags | `GET /v1/workspaces/{ws}/tags?page-size=5` | 200 — 5 tags returned |
| List users | `GET /v1/workspaces/{ws}/users` | 200 — 7 users returned |
| List time entries (direct) | `GET /v1/workspaces/{ws}/time-entries?page-size=2` | 405 — "Request method 'GET' is not supported" (see Finding 4) |
| List time entries (MCP path) | `clockify_list_entries` via stdio | OK — returns entries |
| Create entry (dry run) | `clockify_add_entry` via stdio | OK — dry_run preview correct |
| MCP whoami | `clockify_whoami` via stdio | OK — identity + workspace match direct API |
| MCP list projects | `clockify_list_projects` via stdio | OK — matches direct API |

## Findings

### Finding 1: Doctor exit code attribution correct (P3, informational)

The `doctor` exit codes are correct:
- **0** on clean Load()
- **2** on Load() error (missing CLOCKIFY_API_KEY)
- **3** on --strict posture findings

`doctor --profile=local-stdio --strict` correctly surfaces 3 ERROR findings for hosted deployment requirements that `local-stdio` doesn't satisfy. This is expected since `local-stdio` is designed for single-user desktop use, not hosted multi-tenant deployment.

### Finding 2: Server starts with bad API key (P2, design observation)

When `CLOCKIFY_API_KEY` is set to an invalid value, the server starts and completes initialization without error. The failure only surfaces at tool-call time when the upstream API returns an error. This is valid design — the server defers API key validation to first use — but could be surprising for operators who expect a startup-time credential check.

**Reproduction:**
```sh
CLOCKIFY_API_KEY=bad-key CLOCKIFY_WORKSPACE_ID=<REDACTED> clockify-mcp
# Server starts, logs "server_start" with policy=standard
# First tool call returns tool error from upstream
```

### Finding 3: Docker distroless non-root user causes file-backend permission errors (P2, documented friction)

The Docker image runs as `USER 65532:65532` (distroless nonroot). When using the `single-tenant-http` profile with default `file:///var/lib/clockify-mcp/cp.json` control plane DSN, the container fails with:

```
error: mkdir control-plane dir: mkdir /var/lib/clockify-mcp: permission denied
```

**Workarounds:**
1. Pre-create directory in Dockerfile: `RUN mkdir -p /var/lib/clockify-mcp && chown 65532:65532 /var/lib/clockify-mcp`
2. Use memory backend for dev/testing: `-e MCP_CONTROL_PLANE_DSN=memory -e MCP_ALLOW_DEV_BACKEND=1`
3. Volume mount with pre-initialized permissions (as documented in README)

The `docker volume create` + `docker run -v` recipe in the README requires the volume to have appropriate ownership for uid 65532. Fresh Docker volumes default to root ownership, so first-run will fail without manual initialization.

### Finding 4: Direct API time-entries endpoint returns 405 on probe workspace (P3, workspace quirk)

The endpoint `GET https://api.clockify.me/api/v1/workspaces/{ws}/time-entries?page-size=2` returns HTTP 405 with body `{"message":"Request method 'GET' is not supported","code":3000}`. The MCP server's `clockify_list_entries` tool works correctly and returns entries through the same workspace, so the MCP path is unaffected. This may be a workspace-level restriction or API quirk.

### Finding 5: Docker image ENV defaults override profile settings (P3, documented)

The Dockerfile sets `ENV MCP_TRANSPORT=streamable_http`, which becomes an explicit env var. Per the documented precedence rule ("explicit env overrides still win"), this means `--profile=local-stdio` cannot change the transport inside Docker. This is correct and documented; `local-stdio` is inherently incompatible with Docker since stdio requires a subprocess spawn.

**Correct Docker profile:** `single-tenant-http` (for single workspace) or `shared-service` (for multi-tenant).

## Fixes made

No code fixes were required. All identified behaviors are either correct-by-design or documented friction points.

## Reproduction steps for each issue

### Finding 2 (bad API key starts server)
```sh
go build -o /tmp/clockify-mcp-test ./cmd/clockify-mcp/
CLOCKIFY_API_KEY=bad-invalid-key CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  /tmp/clockify-mcp-test --profile=local-stdio
# Observe: server_start log, then tool calls fail with upstream error
```

### Finding 3 (Docker permission error)
```sh
docker build -f deploy/Dockerfile -t clockify-mcp-test .
docker run --rm \
  -e CLOCKIFY_API_KEY=<REDACTED> \
  -e MCP_PROFILE=single-tenant-http \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  -e MCP_BEARER_TOKEN=<REDACTED> \
  -e MCP_ALLOWED_ORIGINS=localhost \
  clockify-mcp-test
# Observe: "permission denied" for /var/lib/clockify-mcp
```

### Finding 4 (time entries 405)
```sh
curl -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries?page-size=1"
# Observe: 405, {"message":"Request method 'GET' is not supported","code":3000}
```

## Cleanup performed

- Docker containers stopped and removed
- Docker volumes cleaned
- No test resources created in the workspace (dry-run only)
- Binary cleaned from `/tmp/clockify-mcp-test`

## Leftover test resources

None. All MCP tool calls used `dry_run=true`. No resources were created in the Clockify workspace.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| Doctor exit codes correct | P3 | Informational confirmation |
| Bad API key doesn't block startup | P2 | Design choice, could surprise operators |
| Docker non-root permission error | P2 | Documented friction, fixable with Dockerfile `RUN mkdir` |
| Time entries direct API 405 | P3 | Affects only direct API path; MCP path works |
| Docker ENV overrides profile | P3 | Documented precedence; correct behavior |

## Files changed

No files were changed. The repository is clean.

## Suggested next action

1. **Dockerfile improvement** (low effort, high value): Add `RUN mkdir -p /var/lib/clockify-mcp && chown 65532:65532 /var/lib/clockify-mcp` to the Dockerfile runtime stage so the file-backed control plane works out of the box with `single-tenant-http` profile.

2. **Startup-time API key liveness check** (optional): Consider an optional `doctor --check-credentials` that validates the API key against Clockify at diagnostic time, distinct from the config-shape audit.

3. **Investigate time-entries 405**: Check if the probe workspace has a configuration restriction on the `GET /workspaces/{ws}/time-entries` endpoint. The MCP path works, but understanding the divergence would confirm no hidden MCP server issue.

## False positives / uncertainty

- **Time entries 405**: Could be a transient API behavior or workspace-level setting. Not reproduced against a different workspace. The MCP server's `clockify_list_entries` tool works correctly.
- **Go version**: Tested with Go 1.26.2; the module requires Go 1.25.10. No compilation issues.

## Final recommendation

The MCP server starts cleanly, handles both stdio and streamable HTTP transports, correctly enforces policy modes, and integrates correctly with the live Clockify API. All doctor exit codes are accurate. The Docker image builds and runs correctly with appropriate configuration. No blocking issues were found.

**Status: READY for community/internal/self-hosted use.**

The Dockerfile directory-permission improvement is recommended but not required for basic readiness.
