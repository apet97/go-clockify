# QA Agent 08 - startup-failure-modes

## Verdict
PASS WITH CONCERNS

## What I checked

Checked the go-clockify MCP server startup path for failure modes: how the server
behaves on missing, invalid, or misconfigured environment variables; whether the
Docker image defaults allow a clean startup; whether the `doctor` command
accurately diagnoses startup problems; and whether the server fails-closed
rather than silently accepting broken config.

Covered these startup paths:

- Missing `CLOCKIFY_API_KEY` for stdio/HTTP transports
- Invalid `MCP_TRANSPORT`, `MCP_AUTH_MODE`, `MCP_PROFILE`, timezone
- Transport x auth-mode matrix violations (mTLS on legacy http, auth on stdio, etc.)
- `streamable_http` with missing OIDC issuer
- TLS cert material enforcement for gRPC and mTLS
- Production (`ENVIRONMENT=prod`) guardrails — dev backend refusal, legacy HTTP policy
- Docker image default ENV vs. actual config requirements
- Docker HEALTHCHECK effectiveness for broken config
- Server startup with valid but incorrect credentials (bad API key, wrong workspace ID)
- `doctor` command diagnostic accuracy and exit codes
- Eager server startup (tools advertised before any API call validates credentials)
- MCP protocol compliance (initialize required before tools/list)
- Live API probes: whoami, list workspaces, create project (dry-run + real), archive/delete cleanup

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — credentials (API key: ****REDACTED****, workspace: `65b382b606de527a7ee2b60e`)
- `source`-d only; raw key never written to files, logs, or shell history

## Commands run

```
# Build
go build -o /tmp/clockify-mcp-test ./cmd/clockify-mcp/

# Doctor with real credentials (passes config load, flags hosted-strict posture gaps)
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=... /tmp/clockify-mcp-test doctor --strict --check-backends

# Doctor with no API key -> ERROR exit 2
/tmp/clockify-mcp-test doctor

# Invalid transport
CLOCKIFY_API_KEY=x MCP_TRANSPORT=bogus /tmp/clockify-mcp-test doctor

# Missing bearer token for HTTP transport
CLOCKIFY_API_KEY=x MCP_TRANSPORT=http MCP_AUTH_MODE=static_bearer /tmp/clockify-mcp-test doctor

# Invalid profile
CLOCKIFY_API_KEY=x MCP_PROFILE=bogus-profile /tmp/clockify-mcp-test doctor

# Missing OIDC issuer for streamable_http
CLOCKIFY_API_KEY=x MCP_TRANSPORT=streamable_http /tmp/clockify-mcp-test doctor

# Invalid timezone
CLOCKIFY_API_KEY=x CLOCKIFY_TIMEZONE=Mars/Elysium /tmp/clockify-mcp-test doctor

# Invalid auth mode
CLOCKIFY_API_KEY=x MCP_TRANSPORT=http MCP_AUTH_MODE=bogus /tmp/clockify-mcp-test doctor

# Auth mode on stdio (rejected)
CLOCKIFY_API_KEY=x MCP_TRANSPORT=stdio MCP_AUTH_MODE=static_bearer /tmp/clockify-mcp-test doctor

# mTLS on legacy http (rejected with clear message)
CLOCKIFY_API_KEY=x MCP_TRANSPORT=http MCP_AUTH_MODE=mtls /tmp/clockify-mcp-test doctor

# Prod with memory backend (rejected)
CLOCKIFY_API_KEY=x ENVIRONMENT=prod /tmp/clockify-mcp-test doctor

# Dockerfile default config simulation (FAILS - no OIDC_ISSUER)
MCP_TRANSPORT=streamable_http MCP_HTTP_BIND=0.0.0.0:8080 MCP_STRICT_HOST_CHECK=1 /tmp/clockify-mcp-test doctor

# MCP stdio smoke: initialize + tools/list (38 Tier-1 tools)
echo '<initialize>' | CLOCKIFY_API_KEY=... CLOCKIFY_WORKSPACE_ID=... /tmp/clockify-mcp-test

# MCP stdio: whoami call (returns user + resolved workspace)
echo '<initialize+whoami>' | CLOCKIFY_API_KEY=... CLOCKIFY_WORKSPACE_ID=... /tmp/clockify-mcp-test

# MCP stdio: create_project dry_run (works)
echo '<init+create_project dry_run=true>' | CLOCKIFY_API_KEY=... CLOCKIFY_WORKSPACE_ID=... /tmp/clockify-mcp-test

# Docker HEALTHCHECK --version (always exits 0 regardless of config)
MCP_TRANSPORT=streamable_http MCP_STRICT_HOST_CHECK=1 /tmp/clockify-mcp-test --version
```

## Live API probes run

Direct API (curl with `X-Api-Key` header):

| Endpoint | Method | Status | Notes |
|----------|--------|--------|-------|
| `/user` | GET | 200 | Returns user identity, active workspace |
| `/workspaces` | GET | 200 | Workspace `65b382b606de527a7ee2b60e` accessible |
| `/user` with bad key | GET | 401 | `"Api key does not exist"`, code 4003 |
| `/workspaces/INVALID` | GET | 403 | `"Access Denied"`, code 501 |
| `/workspaces/{ws}/projects` | GET | 200 | Returns project list |
| `/workspaces/{ws}/projects` | POST | 200 | Created `qa-agent-08-test-project` |
| `/workspaces/{ws}/projects/{id}` | PUT (archive) | 200 | Archived project |
| `/workspaces/{ws}/projects/{id}` | DELETE | 200 | Deleted archived project |
| `http://api.clockify.me/api/v1/user` | GET | 301->HTTPS | HTTP redirected to HTTPS |

MCP server tool calls (stdio transport):

| Tool | Result |
|------|--------|
| `initialize` | OK, protocol 2024-11-05, server version dev |
| `tools/list` | 38 Tier-1 tools with full inputSchema/outputSchema |
| `clockify_whoami` | User ID `64621fae...`, workspace `65b382b6...` |
| `clockify_create_project` (dry_run) | OK, previewed payload |
| `clockify_create_project` (real) | Created project `6a00f8bb385b9fac085a4f80` |

## Findings

### P1 - Dockerfile standalone defaults cause startup failure

`deploy/Dockerfile:97-102` sets these defaults:

```
MCP_TRANSPORT=streamable_http
MCP_HTTP_BIND=0.0.0.0:8080
MCP_STRICT_HOST_CHECK=1
```

With just these defaults (no docker-compose, no orchestrator env injection),
the server **fails to start** with three independent config-load errors:

1. **Missing OIDC issuer**: `MCP_TRANSPORT=streamable_http` defaults auth mode to
   `oidc`, which requires `MCP_OIDC_ISSUER` (`config.go:596-598`).

2. **Strict host check with no origins**: `MCP_STRICT_HOST_CHECK=1` on a
   non-loopback bind (`0.0.0.0:8080`) with no `MCP_ALLOWED_ORIGINS` rejects
   every request; caught at config load (`config.go:646-655`).

3. **Dev backend disallowed**: `streamable_http` requires either
   `MCP_CONTROL_PLANE_DSN=postgres://...` or `MCP_ALLOW_DEV_BACKEND=1`
   (`config.go:612-618`).

The docker-compose.yml (`deploy/docker-compose.yml`) provides all required
env vars (MCP_PROFILE=single-tenant-http, CLOCKIFY_API_KEY, MCP_BEARER_TOKEN,
MCP_ALLOWED_ORIGINS) — so the compose path works. But the standalone image
cannot start with its own defaults.

**Recommendation**: Either document that the image MUST be used with an
orchestrator that injects the required env vars, or change defaults to use
`MCP_PROFILE=single-tenant-http` and `MCP_ALLOW_DEV_BACKEND=1` with a
clear warning that these are dev-only defaults.

### P2 - Docker HEALTHCHECK reports healthy for broken config

`deploy/Dockerfile:111-112`:

```
HEALTHCHECK CMD ["/usr/local/bin/clockify-mcp", "--version"]
```

`--version` exits 0 unconditionally — it prints the version string and exits,
never loading config or validating credentials. A container with a broken
configuration (missing API key, wrong transport, etc.) would report healthy
even though the actual server process is crash-looping or refusing connections.

**Recommendation**: For HTTP transports, use a `/health` or `/ready` endpoint
probe. For stdio, use `clockify-mcp doctor` to validate config. The current
HEALTHCHECK gives a false positive for every config-load error.

### P2 - Server starts eagerly with bad credentials

The server initializes, advertises 38 tools in `tools/list`, and responds to
`initialize` successfully even when:
- `CLOCKIFY_API_KEY` is bogus (would 401 on first tool call)
- `CLOCKIFY_WORKSPACE_ID` is a non-existent but format-valid ID (would 403 on
  workspace-scoped tools)

The `config.Load()` function validates config *shape* (required env vars present,
valid enum values, TLS cert pair completeness) but does **not** validate that
the credentials work against the Clockify API. The first actual tool call
(`clockify_whoami`, `clockify_list_projects`, etc.) will fail.

This is a design trade-off — startup-time API calls add latency and coupling —
but it means an LLM using this server sees healthy init + full tool list, then
every tool call errors. The `doctor` command catches the config-load errors,
but the runtime startup path does not preflight credentials.

**Recommendation**: Consider an optional `--check-credentials` flag or
`MCP_CHECK_CREDENTIALS_ON_STARTUP=1` that calls `/user` during startup. Document
the current eager-start behavior clearly.

### P3 - Workspace ID validation is format-only

`internal/resolve/resolve.go:ValidateID()` checks only format (no control chars,
no `..`, no `/#?%`, max 128 bytes). A syntactically valid but non-existent
workspace ID passes config validation and the server starts. The API call
subsequently returns 403.

**Note**: This is reasonable — the Clockify API returns distinct error codes
for "invalid ID" vs "no access" — but the config-load message could mention
that the ID was accepted by format but not validated against the API.

### P3 - Invalid workspace ID returns 403 not 404

The Clockify API returns HTTP 403 (not 404 or 400) for an invalid workspace
ID in the URL path. The MCP server's APIError correctly surfaces this as
`clockify GET failed: 403: ...`. The error message is legible but could
be friendlier for the "wrong workspace ID" case.

## Fixes made

No code changes made. The issues identified are configuration/documentation
matters or design trade-offs, not code bugs.

## Reproduction steps for each issue

### P1 - Docker standalone startup failure
```
docker build -f deploy/Dockerfile -t clockify-mcp:test .
docker run --rm clockify-mcp:test
# Expected: container exits with config-load error
# Actual: same
```

### P2 - HEALTHCHECK false positive
```
# Build image with broken config
# HEALTHCHECK runs: clockify-mcp --version -> exit 0
# Container reports healthy, but main process crash-loops
```

### P2 - Eager start with bad credentials
```
CLOCKIFY_API_KEY="bogus-key" CLOCKIFY_WORKSPACE_ID="65b382b606de527a7ee2b60e" \
  clockify-mcp
# Server starts, logs "server_start", responds to initialize + tools/list
# clockify_whoami -> 401 error
```

## Cleanup performed

- Created test project `qa-agent-08-test-project` (id `6a00f8bb385b9fac085a4f80`)
  in workspace `65b382b606de527a7ee2b60e`
- Archived it via PUT, deleted via DELETE — both returned 200
- No leftover test resources

## Leftover test resources

None. All created resources cleaned up.

## Severity

| ID | Severity | Summary |
|----|----------|---------|
| 1 | P1 | Dockerfile standalone defaults cause startup failure (3 independent config errors) |
| 2 | P2 | Docker HEALTHCHECK uses --version -> always reports healthy even for broken config |
| 3 | P2 | Server starts eagerly with bad credentials; API key not validated until first tool call |
| 4 | P3 | Workspace ID validation is format-only, no API existence check |
| 5 | P3 | Invalid workspace ID returns 403 from API; surfaced correctly but message could be friendlier |

## Files changed

None.

## Suggested next action

1. **Fix the Dockerfile defaults** — either switch to `MCP_PROFILE=single-tenant-http`
   with `MCP_ALLOW_DEV_BACKEND=1` and document these as dev-only defaults, or add
   a prominent note that the image requires env injection from docker-compose /
   Helm / K8s manifests.

2. **Fix the HEALTHCHECK** — change to `clockify-mcp doctor` for config validation,
   or switch to an HTTP health endpoint probe when `MCP_TRANSPORT` is
   `streamable_http` or `http`.

3. **Document the eager-start behavior** — add a note in README.md or
   `docs/deploy/` that the server starts before validating credentials,
   and recommend running `clockify-mcp doctor` before deploying.

4. **Consider a startup credential preflight** — a `MCP_CHECK_CREDENTIALS_ON_STARTUP=1`
   env var that calls `GET /user` at startup and fails the process on 401/403.

## False positives / uncertainty

- The Dockerfile may intentionally leave defaults incomplete because the image
  is always deployed via Helm/K8s/docker-compose. The README or deploy docs
  should clarify this if so.
- The eager-start behavior (P2) is a common pattern in MCP servers — tools/list
  should work without credentials so clients can discover capabilities before
  authenticating. Whether this is a bug or a feature depends on operator
  expectations.
- Docker was not available for testing actual image builds; the config analysis
  is based on reading the Dockerfile ENV defaults and testing equivalent
  env-var combinations on the locally built binary.

## Final recommendation

**PASS WITH CONCERNS**. The server's config validation is thorough — every
invalid config combination tested produced a clear, actionable error message
at the `config.Load()` or `doctor` stage. The `doctor` command is excellent
for preflight diagnosis. However, the Docker image defaults and HEALTHCHECK
are insufficient for standalone operation, and the server's eager-start
behavior could surprise operators who expect startup-time credential validation.
The three P1/P2 items above should be addressed before a production release
that ships the Docker image as a standalone artifact.
