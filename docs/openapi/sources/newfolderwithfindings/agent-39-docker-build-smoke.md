# QA Agent 39 - docker-build-smoke

## Verdict
PASS WITH CONCERNS

## What I checked
Docker build, image content, runtime behavior, live API connectivity through the container, MCP protocol compliance (streamable HTTP), authentication, policy enforcement, SSE streaming, docker-compose configuration, CI workflow alignment, and cleanup of test resources. All core mechanisms work correctly. Four documentation issues noted below.

## Live API probe lab files used
- `/tmp/clockify-livetest.env` — API key and workspace ID (secrets, not printed)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — safety rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/README.md` — lab layout and credential paths
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TIMEENTRYDOC.md` — endpoint docs (referenced for entry shape)

API key and workspace ID were sourced from `/tmp/clockify-livetest.env`. Neither was written to disk, echoed in clear text, or included in this report.

## Commands run

```bash
# Build (succeeded in ~80s)
docker build \
  --build-arg VERSION=qa-smoke-39 \
  --build-arg COMMIT=abf9459 \
  --build-arg BUILD_DATE=2026-05-10T20:47:57Z \
  -f deploy/Dockerfile \
  -t clockify-mcp:qa-smoke-39 .

# Version / help
docker run --rm clockify-mcp:qa-smoke-39 --version   # qa-smoke-39
docker run --rm clockify-mcp:qa-smoke-39 --help       # full help output

# Doctor (with live creds)
docker run --rm \
  -e CLOCKIFY_API_KEY=<REDACTED> \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  -e MCP_TRANSPORT=stdio \
  clockify-mcp:qa-smoke-39 doctor --strict

# Streamable HTTP server
docker run --rm -d --name clockify-mcp-smoke-39 \
  -e CLOCKIFY_API_KEY=<REDACTED> \
  -e CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  -e MCP_TRANSPORT=streamable_http \
  -e MCP_HTTP_BIND=0.0.0.0:8080 \
  -e MCP_AUTH_MODE=static_bearer \
  -e MCP_BEARER_TOKEN=qa-smoke-39-test-token-16chars \
  -e CLOCKIFY_POLICY=time_tracking_safe \
  -e MCP_STRICT_HOST_CHECK=0 \
  -e MCP_ALLOW_DEV_BACKEND=1 \
  -p 127.0.0.1:9080:8080 \
  clockify-mcp:qa-smoke-39

# Health checks
curl http://127.0.0.1:9080/health   # {"status":"ok"}
curl http://127.0.0.1:9080/ready    # {"status":"ok"}

# MCP initialize (succeeded — sessionId, capabilities, serverInfo)
curl -X POST http://127.0.0.1:9080/mcp \
  -H "Authorization: Bearer qa-smoke-39-test-token-16chars" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'

# Auth rejection test (no bearer token) → HTTP 401 + JSON-RPC error
# tools/list → 35 Tier-1 tools registered
# tools/call clockify_current_user → live API call succeeds
# tools/call clockify_list_entries → 25 entries returned
# tools/call clockify_add_entry → creates entry 6a00fc23385b9fac085a7b31
# tools/call clockify_delete_entry → blocked by time_tracking_safe policy (correct)
# tools/call clockify_get_workspace → workspace resolved

# SSE endpoint (establishes session, streams keepalive)
curl -s -N --max-time 3 \
  -H "Authorization: Bearer qa-smoke-39-test-token-16chars" \
  -H "Accept: text/event-stream" \
  -H "Mcp-Session-Id: <session>" \
  http://127.0.0.1:9080/mcp
# Returns: ": session <id>"

# Docker Compose validation
docker compose -f deploy/docker-compose.yml config   # validates OK

# Cleanup
docker stop clockify-mcp-smoke-39
curl -X DELETE "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries/6a00fc23385b9fac085a7b31" \
  -H "X-Api-Key: <REDACTED>"   # HTTP 204
```

## Live API probes run

| Probe | Method | Result |
|-------|--------|--------|
| `clockify_current_user` | MCP tools/call via container | User returned with activeWorkspace |
| `clockify_list_entries` | MCP tools/call via container | 25 entries returned |
| `clockify_get_workspace` | MCP tools/call via container | Workspace resolved |
| `clockify_add_entry` | MCP tools/call via container | Created entry `6a00fc23385b9fac085a7b31` |
| `clockify_delete_entry` | MCP tools/call via container | Blocked by time_tracking_safe (correct) |
| Direct API DELETE | curl to api.clockify.me | HTTP 204, entry cleaned up |

## Findings

### 1. [P3] Dockerfile image size comment is outdated
**Location:** `deploy/Dockerfile:61`
The comment says `# Stage 2: Minimal runtime image (~10MB)` but the actual image is 18.2 MB. The Go binary is 8.61 MB; the distroless base layers (tzdata, base-files, cacerts, etc.) add ~9.5 MB. The ~10 MB figure reflects the binary alone, not the full image.

**Recommendation:** Update comment to `(~18 MB)` or similar.

### 2. [P3] docker-compose.yml references env file that is easy to miss
**Location:** `deploy/docker-compose.yml` references `CLOCKIFY_API_KEY=${CLOCKIFY_API_KEY}` etc., but the example `.env` file lives at `examples/docker-compose.env` (repo root), not inside `deploy/`. The `deploy/examples/` directory contains profile-specific env templates but no direct docker-compose env template.

The README at `examples/docker-compose.env` correctly documents `cp examples/docker-compose.env deploy/.env`, but the docker-compose.yml itself has no comment pointing there. A new user running `docker compose up` from `deploy/` gets warnings about unset variables with no immediate pointer to the env template.

**Recommendation:** Add a comment at the top of `deploy/docker-compose.yml` pointing to `examples/docker-compose.env`.

### 3. [P3] Placeholder domain lacks explicit replacement warning
**Location:** `deploy/Caddyfile:1`, `deploy/docker-compose.yml:41`
Both use `your-domain.example.com` as a placeholder for `MCP_ALLOWED_ORIGINS` and the Caddy host. This is standard template practice, but the Caddyfile has no "you must replace this" comment.

**Recommendation:** Add a `# REPLACE THIS with your actual domain` comment above the domain line in the Caddyfile.

### 4. [P2] Docker HEALTHCHECK scope is binary-liveness only
The HEALTHCHECK `CMD ["/usr/local/bin/clockify-mcp", "--version"]` only verifies the binary can execute. It does not verify Clockify API reachability or credential validity. A container can be "healthy" while unable to serve real requests (e.g., invalid API key). For the distroless base (no shell, no curl), this is the best available option — the Dockerfile already documents this on lines 108-111 and directs operators to use `/health` and `/ready` for orchestrator probes.

**Recommendation:** No code change needed. This is already well-documented in the Dockerfile. Flagged for operator awareness.

### 5. [OK] Confirmed working correctly
- Digest-pinned base images (builder `golang:1.25-bookworm@sha256:...`, runtime `distroless/static-debian12:nonroot@sha256:...`)
- Multi-arch build support (--platform=$BUILDPLATFORM, TARGETOS/TARGETARCH)
- ldflags propagation (VERSION, COMMIT, BUILD_DATE all appear in --version and logs)
- Non-root user (uid/gid 65532)
- OCI image labels (org.opencontainers.image.*)
- STOPSIGNAL SIGTERM
- MCP_TRANSPORT, MCP_HTTP_BIND, MCP_LOG_FORMAT, MCP_STRICT_HOST_CHECK defaults from Dockerfile ENV
- HEALTHCHECK interval=30s, timeout=5s, retries=3
- Session management (initialize → initialized notification → tools/list → tools/call)
- Bearer token auth (rejects missing/invalid tokens with HTTP 401 + proper JSON-RPC error)
- MCP protocol version negotiation (2025-03-26)
- streamable HTTP SSE endpoint (establishes session, returns `: session <id>`)
- Policy enforcement (time_tracking_safe blocks delete_entry, allows add_entry)
- Tool-error envelope (isError:true for policy blocks — correct per MCP spec)
- `doctor` command config audit with secret redaction
- Graceful error messages for missing required config (OIDC issuer, dev backend block)
- Docker Compose config validates (`docker compose config`)
- GoReleaser correctly excludes Docker image builds (delegated to docker-image.yml workflow)
- CI workflow `docker-image.yml` covers build, Trivy scan, cosign sign, SBOM, multi-arch push

## Fixes made
No code fixes were required. The Docker build is clean and all functional paths work correctly. The four findings above are documentation polish items only.

## Reproduction steps for each issue

### Issue 1 (image size comment)
1. `docker build -f deploy/Dockerfile -t test .`
2. `docker images test` → shows 18.2 MB
3. Read `deploy/Dockerfile:61` → says "~10MB"

### Issue 2 (env file pointer missing)
1. `cd deploy && docker compose up` without a `.env` file
2. Output shows warnings: `The "CLOCKIFY_API_KEY" variable is not set` etc.
3. No pointer in docker-compose.yml to `examples/docker-compose.env`

### Issue 3 (placeholder domain)
1. Read `deploy/Caddyfile:1` — `your-domain.example.com` with no replacement instruction
2. Read `deploy/docker-compose.yml:41` — same placeholder in MCP_ALLOWED_ORIGINS default

### Issue 4 (HEALTHCHECK scope)
1. Start container with intentionally invalid API key
2. `docker ps` shows "(healthy)" — binary executes but API calls would fail
3. `/health` and `/ready` endpoints correctly reflect real health status; Docker HEALTHCHECK is binary-only

## Cleanup performed
- Stopped and removed test container `clockify-mcp-smoke-39`
- Deleted test time entry `6a00fc23385b9fac085a7b31` via direct Clockify API (HTTP 204)
- Removed test Docker image `clockify-mcp:qa-smoke-39`

## Leftover test resources
None. All test resources were cleaned up successfully.

## Severity

| ID | Severity | Area |
|----|----------|------|
| 1 | P3 | Documentation — image size comment outdated |
| 2 | P3 | Documentation — env file pointer missing in docker-compose.yml |
| 3 | P3 | Documentation — placeholder domain comment missing in Caddyfile |
| 4 | P2 | Observability — Docker HEALTHCHECK is binary-liveness only (already documented) |

No P0 or P1 issues found. No functional bugs, no security vulnerabilities, no API contract violations.

## Files changed
None. No code changes were necessary.

## Suggested next action
1. Fix the "~10MB" comment in `deploy/Dockerfile:61` to reflect actual size (~18 MB)
2. Add a comment in `deploy/docker-compose.yml` pointing to `examples/docker-compose.env`
3. Add a `# REPLACE THIS with your actual domain` comment above the placeholder in `deploy/Caddyfile`
4. Consider a docker-compose up + smoke curl as part of release-smoke checklist (requires credentials, so it would be a manual step or CI with repo secrets)

## False positives / uncertainty
- The `doctor --strict` command reports 4 ERRORs for stdio mode (MCP_DISABLE_INLINE_SECRETS, MCP_CONTROL_PLANE_DSN, MCP_AUDIT_DURABILITY, CLOCKIFY_POLICY). These are expected — they are hosted-profile requirements that do not apply to local stdio. The doctor correctly identifies the posture mismatch.
- The streamable_http transport locally requires `MCP_ALLOW_DEV_BACKEND=1` because the default control plane is memory-backed. This is not a bug; it is a deliberate safety gate. The docker-compose.yml correctly uses `MCP_PROFILE=single-tenant-http` which auto-sets this flag.

## Final recommendation
The Docker build is production-ready for community/self-hosted use. The image builds cleanly, runs correctly, enforces auth and policy, and communicates with the live Clockify API through all tested paths. The four findings are documentation polish items; none block usage. Recommend **PASS WITH CONCERNS** to reflect the minor doc issues.
