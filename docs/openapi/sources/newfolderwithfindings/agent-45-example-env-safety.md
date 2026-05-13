# QA Agent 45 - example-env-safety

## Verdict
PASS WITH CONCERNS

## What I checked

- **Example env file safety**: All 5 `deploy/examples/env.*.example` files, plus `examples/docker-compose.env`, `examples/claude-desktop.json`, `examples/cursor-mcp.json`.
- **`.gitignore` env coverage**: Whether `.env` and related patterns prevent accidental secret commits.
- **Hardcoded secrets**: Scanned Go source, test files, config specs, Helm/K8s templates for real credentials.
- **`clockify-mcp doctor` command**: Built binary, ran with `local-stdio` and `single-tenant-http` profiles, tested `--strict` mode, verified exit codes (0=clean, 2=error, 3=strict findings), confirmed sensitive values are redacted.
- **Live API probes**: Verified API connectivity against probe workspace, created/deleted a test project to confirm write paths work.
- **MCP stdio handshake**: Successful `initialize` exchange with the built binary, confirmed protocol version negotiation and server capabilities.
- **Env var error handling**: Tested missing `CLOCKIFY_API_KEY` (exit 2), missing `MCP_BEARER_TOKEN` under `single-tenant-http` (exit 2), invalid API key (doctor passes — by design, it's a config audit, not API validator).
- **Credential vault system**: Inspected `internal/vault/vault.go` — supports `env`, `file`, `inline` backends with `MCP_DISABLE_INLINE_SECRETS=1` opt-out for hosted hardening.
- **Deployment templates**: Helm chart secret template, K8s secret example (`REPLACE_ME` placeholders), docker-compose.yml variable interpolation.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key (****REDACTED****), workspace ID (`65b382b606de527a7ee2b60e`), workspace confirm guard
- `probes/lib/common.sh` — shared probe library with `probe_redact()`, `probe_curl()`, cleanup registry
- `CLAUDE.md` — lab rules (hard limits, soft preferences)
- `USERDOC.md` — user API docs
- Live API endpoints tested: `GET /workspaces/{id}`, `GET /workspaces/{id}/projects`, `GET /user`, `POST /workspaces/{id}/projects`, `PUT /workspaces/{id}/projects/{id}`, `DELETE /workspaces/{id}/projects/{id}`

## Commands run

```sh
# Build
go build -o /tmp/clockify-mcp-test ./cmd/clockify-mcp/

# Doctor — local-stdio profile
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=****REDACTED**** \
  MCP_PROFILE=local-stdio /tmp/clockify-mcp-test doctor
# Exit: 0 (OK)

# Doctor — strict posture
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=****REDACTED**** \
  MCP_PROFILE=local-stdio /tmp/clockify-mcp-test doctor --strict
# Exit: 3 (3 ERROR findings — expected for non-hosted profile)

# Doctor — single-tenant-http without bearer token
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=****REDACTED**** \
  MCP_PROFILE=single-tenant-http /tmp/clockify-mcp-test doctor
# Exit: 2 (MCP_BEARER_TOKEN required for static_bearer auth)

# Doctor — empty API key
CLOCKIFY_API_KEY= /tmp/clockify-mcp-test doctor
# Exit: 2 (CLOCKIFY_API_KEY is required)

# MCP stdio initialize handshake
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | \
  CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=****REDACTED**** \
  MCP_PROFILE=local-stdio /tmp/clockify-mcp-test
# Response: Successful initialize with capabilities, serverInfo, instructions

# Live API probes (via curl + sourced credentials from /tmp/clockify-livetest.env)
curl -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/workspaces/****REDACTED****"
# 200 OK — workspace: WORKSPACE

curl -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/user"
# 200 OK — user: alpettest1@gmail.com

# Create + cleanup test project
POST /workspaces/{id}/projects {"name":"qa-agent-45-env-test-project"}  # 201 → id=6a00f6...
PUT  /workspaces/{id}/projects/{id} {"archived":true}                    # 200
DELETE /workspaces/{id}/projects/{id}                                    # 200

# Gitignore verification
git check-ignore -v .env deploy/.env examples/docker-compose.env
# .env → ignored (line 21)
# deploy/.env → ignored (line 22)
# examples/docker-compose.env → NOT ignored (tracked, correct)
```

## Live API probes run

| Probe | Endpoint | Method | Result |
|-------|----------|--------|--------|
| Workspace info | `/workspaces/{id}` | GET | 200, name="WORKSPACE" |
| Current user | `/user` | GET | 200, email="alpettest1@gmail.com" |
| List projects | `/workspaces/{id}/projects?page-size=5` | GET | 200, 5 projects |
| Create test project | `/workspaces/{id}/projects` | POST | 201, name="qa-agent-45-env-test-project" |
| Archive project | `/workspaces/{id}/projects/{id}` | PUT | 200, archived=true |
| Delete project | `/workspaces/{id}/projects/{id}` | DELETE | 200 |
| Invalid API key | `/workspaces/{id}` | GET | 401 (correct rejection) |

## Findings

### Finding 1: .gitignore missing .env patterns (P1) — FIXED

**Severity**: P1

**Description**: The `.gitignore` had no rules to prevent `.env` files from being committed. The `examples/docker-compose.env` template explicitly instructs users:
```
# Copy to .env and fill in your values:
#   cp examples/docker-compose.env deploy/.env
```
If a user follows these instructions and fills in their real Clockify API key and bearer token, the resulting `deploy/.env` file would be tracked by git — no existing gitignore rule would block it.

**Risk**: Accidental credential commit to git history. If the repo is public or shared, this leaks the user's Clockify API key (full workspace access) and MCP bearer token.

**Fix applied**: Added `.env` and `deploy/.env` to `.gitignore` (lines 17-22). Verified that `examples/docker-compose.env` remains tracked while `deploy/.env` and `.env` are properly ignored.

### Finding 2: `deploy/.gitignore` does not exist (P2)

**Severity**: P2

**Description**: There is no `deploy/.gitignore` file. While the root `.gitignore` now covers `deploy/.env`, a `deploy/.gitignore` would be defense-in-depth for any future secret-bearing files that land under `deploy/`.

**Recommendation**: Consider adding `deploy/.gitignore` with at minimum `!.env.example` to reinforce that `.example` files are tracked.

### Finding 3: `env.self-hosted.example` has no corresponding profile (P3 — documented)

**Severity**: P3

**Description**: `deploy/examples/env.self-hosted.example` exists as a template but there is no registered `self-hosted` MCP profile. The file doesn't set `MCP_PROFILE`. This is a known limitation documented in `docs/deploy/profile-self-hosted.md` and the profile.go source ("legacy-shape upgrade pointer without a corresponding registered profile"). The doc recommends migrating to `local-stdio` or `single-tenant-http`.

**Risk**: Low — a user sourcing this file without setting `MCP_PROFILE` would get explicit-env-only behavior (still works, just no profile defaults). But the naming mismatch between the file and the profile system could confuse operators.

**Recommendation**: Add a comment header to `env.self-hosted.example` noting this is a legacy template and pointing to `profile-self-hosted.md`.

### Finding 4: `env.self-hosted.example` omits CLOCKIFY_WORKSPACE_ID (P3)

**Severity**: P3

**Description**: Unlike the other four env example files which all prominently show `CLOCKIFY_WORKSPACE_ID=your-workspace-id`, the self-hosted example omits this variable entirely. The README recommends pinning the workspace ID for safety.

**Recommendation**: Add `# CLOCKIFY_WORKSPACE_ID=your-workspace-id` commented-out line to match the pattern of other env examples.

### Positive: Strong env safety posture overall (no severity)

The codebase demonstrates strong env safety practices:

1. **`clockify-mcp doctor`**: Redacts sensitive values as `set (redacted)`, shows source attribution (explicit/profile/default/empty), and provides `--strict` hosted-service posture checks. Exit codes are semantically meaningful.

2. **`spec.go` Sensitive tagging**: Six env vars are marked `Sensitive: true` — `CLOCKIFY_API_KEY`, `MCP_BEARER_TOKEN`, `MCP_METRICS_BEARER_TOKEN`, `MCP_GRPC_TLS_KEY`, `MCP_HTTP_TLS_KEY`, `MCP_CONTROL_PLANE_DSN`.

3. **No hardcoded secrets**: All test code uses placeholder values (`"test-key"`, `"correcthorsebatterystaple"`, `"dummy"`). The Helm/K8s templates use `REPLACE_ME` sentinels or empty values.

4. **`CLOCKIFY_INSECURE=1` guard**: Hosted profiles refuse to start with `CLOCKIFY_INSECURE=1` (config.go line 294-296), preventing accidental plaintext API key transmission.

5. **Credential leak response runbook**: `docs/runbooks/credential-leak-response.md` documents severity classification, immediate containment, and rotation procedures.

6. **`MCP_DISABLE_INLINE_SECRETS=1`**: The vault system supports disabling inline credential refs for hosted hardening. Pinned by `single-tenant-http` and `shared-service` profiles.

7. **SSRF protection**: `CLOCKIFY_WEBHOOK_VALIDATE_DNS=1` (default) resolves webhook hosts via DNS and rejects private/reserved IPs.

8. **Error sanitization**: `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` strips upstream Clockify response bodies from MCP tool-error envelopes under hosted profiles.

## Fixes made

### `.gitignore` — added `.env` and `deploy/.env` rules

**File**: `.gitignore` (lines 17-22)

Added targeted gitignore rules to prevent `.env` and `deploy/.env` from being committed, while preserving tracking of existing template files (`examples/docker-compose.env`, `deploy/examples/*.example`).

**Before**: No `.env` protection in gitignore.
**After**: `.env` and `deploy/.env` are gitignored. `examples/docker-compose.env` and `deploy/examples/*.example` files remain tracked.

**Verification**:
```
$ git check-ignore -v .env deploy/.env examples/docker-compose.env
.gitignore:21:.env           .env
.gitignore:22:deploy/.env    deploy/.env
# examples/docker-compose.env is NOT ignored (correct)
```

## Reproduction steps for each issue

### Finding 1 (P1) — .env commit risk

1. Follow README: `cp examples/docker-compose.env deploy/.env`
2. Edit `deploy/.env` with real API key and bearer token
3. Run `git status` — before the fix, `deploy/.env` shows as untracked (ready to `git add`)
4. Run `git add deploy/.env && git commit` — secrets land in git history
5. After the fix, `deploy/.env` is ignored and does not appear in `git status`

### Finding 3 (P3) — self-hosted example confusion

1. `source deploy/examples/env.self-hosted.example`
2. Run `clockify-mcp doctor` — shows `Profile: (none)` with "Explicit env only"
3. Operator may be confused why `self-hosted` didn't register as a profile
4. Resolution: add `MCP_PROFILE=local-stdio` or see `docs/deploy/profile-self-hosted.md`

## Cleanup performed

- Created test project `qa-agent-45-env-test-project` (id: `6a00f660d9647159dc10349c`)
- Archived then deleted the test project — confirmed 200 DELETE
- No other resources created or modified

## Leftover test resources

None. Test project was fully cleaned up (archive + delete).

## Severity

| Finding | Severity |
|---------|----------|
| .gitignore missing .env patterns | P1 |
| deploy/.gitignore missing (defense-in-depth) | P2 |
| env.self-hosted.example has no profile | P3 |
| env.self-hosted.example omits WORKSPACE_ID | P3 |

## Files changed

- `.gitignore` — added `.env` and `deploy/.env` rules (lines 17-22)

## Suggested next action

1. **Apply the `.gitignore` fix** (already done in this worktree)
2. **Add a comment header to `deploy/examples/env.self-hosted.example`** pointing to `docs/deploy/profile-self-hosted.md` and recommending `MCP_PROFILE=local-stdio`
3. **Add `# CLOCKIFY_WORKSPACE_ID=your-workspace-id`** to `env.self-hosted.example` for consistency with other examples
4. **Consider adding a CI check** (e.g., in `scripts/check-repo-hygiene.sh`) that scans for `.env` files with values that don't match known safe placeholder patterns
5. **Consider adding `deploy/.gitignore`** as defense-in-depth with `!.env.example` and `!examples/*` rules
6. **Add `docker-compose.env` note** about the gitignore safety: mention that `deploy/.env` is now gitignored

## False positives / uncertainty

- **Doctor doesn't validate API key against Clockify**: The doctor command treats an invalid API key as OK (exit 0) because `Load()` succeeds — it's a config audit, not an API validator. This is by design. The actual API validation happens at first tool use. No action needed.
- **`env.self-hosted.example` is intentionally legacy**: The profile-self-hosted.md doc and profile.go source both acknowledge this is a legacy template without a profile. The documentation is clear. The naming mismatch is not a bug but creates operator confusion risk.

## Final recommendation

**PASS WITH CONCERNS** — the codebase has strong env safety fundamentals (sensitive tagging, doctor redaction, hosted guards, leak runbook), but the missing `.gitignore` pattern for `.env` files was a real safety gap that has been fixed. The remaining findings are P2/P3 documentation/consistency issues that don't block safety but should be addressed for operator clarity.
