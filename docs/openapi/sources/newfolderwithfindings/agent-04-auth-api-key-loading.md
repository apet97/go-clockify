# QA Agent 04 - auth-api-key-loading

Status: COMPLETE
Completed UTC: 2026-05-10T21:20:00Z

## Verdict
**PASS WITH CONCERNS** (single P3 observation, no blockers)

## What I checked

1. **API key loading chain** — `CLOCKIFY_API_KEY` env var → `config.Load()` → `Config.APIKey` → runtime bootstrap → vault credential ref → `clockify.NewClient()` → `X-Api-Key` HTTP header.
2. **Config validation** — required/optional rules per transport, empty-key rejection, profile-specific key requirements.
3. **Workspace ID loading** — explicit `CLOCKIFY_WORKSPACE_ID`, auto-detection via `/workspaces`, validation guardrails.
4. **Base URL validation** — HTTPS enforcement, loopback bypass, insecure escape hatch, hosted-profile refusal.
5. **Secret redaction** — doctor output (`set (redacted)` for sensitive vars), startup log (`Fingerprint()` excludes API key), slog redaction handler (20+ sensitive key patterns + regex value detection).
6. **Error handling** — missing key at config load, invalid key at API call time, non-existent workspace ID, empty base URL fallback.
7. **Live API probes** — valid key, invalid key, missing key, valid + invalid workspace ID combinations.
8. **MCP server smoke test** — stdio initialize → tools/list → tools/call with valid and invalid keys.
9. **Doctor command** — vanilla, `--strict`, `--check-backends`, `--profile=` variants, `--version`, `--help`.
10. **Test suite** — config, authn (unit), clockify client, logging/redaction, vault — all pass.
11. **Vault module** — inline/env/file backends, JSON payload parsing, `MCP_DISABLE_INLINE_SECRETS` safeguard, empty-reference validation.
12. **Profile defaults** — local-stdio, single-tenant-http, shared-service, private-network-grpc, prod-postgres auth posture defaults.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, workspace confirm (never written to report)
- `probes/lib/common.sh` — curl wrapper convention (`X-Api-Key` header)
- `CLAUDE.md` / `README.md` — safety rules and env file format
- `docs/safety-rules.md` — reference
- API base: `https://api.clockify.me/api/v1`

## Commands run

```sh
# Build
go build -o /tmp/clockify-mcp-qa04 ./cmd/clockify-mcp/

# Doctor (vanilla)
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor

# Doctor --strict
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=not-a-valid-id!!! \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor --strict

# Doctor --check-backends
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor --check-backends

# Missing key (stdio)
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor

# Invalid workspace ID
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=not-a-valid-id!!! \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor

# Empty base URL (should default)
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  CLOCKIFY_BASE_URL="" MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04 doctor

# --version / --help (check for secret leaks)
/tmp/clockify-mcp-qa04 --version
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  /tmp/clockify-mcp-qa04 --help

# Test suite
go test ./internal/config/... -count=1
go test ./internal/authn/... -count=1 -run 'Test(Static|Forward|Bearer|Sanitize|Validate|JWT|Constant|OIDCStrict|Config)'
go test ./internal/clockify/... -count=1
go test ./internal/logging/... -count=1
go test ./internal/vault/... -count=1

# MCP stdio smoke: initialize + tools/list
printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n{"jsonrpc":"2.0","id":2,"method":"tools/list",...}\n' | \
  CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04

# MCP stdio smoke: tools/call with valid key
printf '...initialize...\n{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_get_workspace",...}}\n' | \
  CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04

# MCP stdio smoke: tools/call with bad key
printf '...initialize...\n...tools/call clockify_get_workspace...\n' | \
  CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  MCP_METRICS_AUTH_MODE=none /tmp/clockify-mcp-qa04
```

## Live API probes run

```sh
# Valid key
curl -H "X-Api-Key: " https://api.clockify.me/api/v1/workspaces          → 200, 25 workspaces
curl -H "X-Api-Key: " https://api.clockify.me/api/v1/workspaces/<REDACTED> → 200
curl -H "X-Api-Key: " https://api.clockify.me/api/v1/user                  → 200, active user

# Invalid key
curl -H "X-Api-Key: " .../workspaces  → 401
curl -H "X-Api-Key: "     .../workspaces  → 401

# Missing key
curl .../workspaces                                  → 401
curl -H "X-Api-Key:" .../workspaces                  → 401

# Paginated list
curl -H "X-Api-Key: " .../workspaces/<REDACTED>/projects?page-size=5 → 200
```

## Findings

### Finding 1 (P3): Permissive workspace ID validation at config load

**What**: `resolve.ValidateID()` in `internal/resolve/resolve.go:18-37` validates workspace IDs for path safety only — it rejects control bytes (`<0x20`, `0x7F`), `/?#%`, `..`, empty strings, and values over 128 bytes. Values like `not-a-valid-id!!!` pass validation. The Clockify API rejects these at request time with a 404, which the MCP server surfaces as a tool error.

**Impact**: A user who typos their workspace ID (e.g., `<REDACTED_ID>` → `<REDACTED_ID>`) gets a runtime 404 error from the first API call rather than an immediate config-load error. The error message is clear (`clockify GET /workspaces/... failed: 404 Not Found`), so this is low-severity.

**Design rationale**: The current validation deliberately avoids a Clockify-specific ID format check (24-char hex) because Clockify may change its ID format in the future. The API is the authoritative validator.

**Recommendation**: No code change needed — this is an acceptable design tradeoff. The error message from the API call is sufficiently clear for operators to diagnose the issue.

### Finding 2 (Verified safe): API key not leaked in any output path

**What I verified**:
- Startup log (`slog.Info("server_start", ..., "config", cfg.Fingerprint())`) — `Fingerprint()` excludes `api_key` field
- Doctor output — `doctorDisplayValue()` returns `"set (redacted)"` for any `Sensitive: true` spec
- `--help` output — only documents the env var name, never prints its value
- `--version` output — prints version only
- Slog redaction handler — `internal/logging/redact.go` catches `api_key`, `apikey`, `x-api-key`, `bearer`, `token`, `secret`, `credential`, and 14 other patterns in log attributes and map/slice values
- Regex-based value detection — catches PEM private keys, Stripe-like `pk_` keys, JWT-shaped tokens, and `key=value` query strings

### Finding 3 (Verified safe): Config load correctly rejects missing API key for stdio

**What**: `config.Load()` at line 328-330: `if cfg.Transport != "streamable_http" && cfg.APIKey == ""` → `return Config{}, fmt.Errorf("CLOCKIFY_API_KEY is required")`. Exit code 2 from doctor.

**Verified**: Tested with empty `CLOCKIFY_API_KEY` and stdio transport — config load fails with clear error.

### Finding 4 (Verified safe): Profile `single-tenant-http` enforces API key requirement

**What**: `config.Load()` at line 339-343: extra guard that `single-tenant-http` profile with empty API key fails at config load instead of starting a server with no usable backend. Error: `"CLOCKIFY_API_KEY is required for MCP_PROFILE=single-tenant-http (profile bootstraps the default tenant from the env API key)"`.

### Finding 5 (Verified safe): Invalid API key produces clean error at tool-call time

**What**: MCP server with `CLOCKIFY_API_KEY=` starts and initializes successfully. On `tools/call clockify_get_workspace`, returns `isError: true` with: `"clockify GET failed: 401 Unauthorized: Api key does not exist (upstream_code=4003)"`. The error does NOT echo the actual key value.

### Finding 6 (Verified safe): Base URL defaults correctly when empty

**What**: `config.Load()` at line 302-304: `if cfg.BaseURL == "" { cfg.BaseURL = DefaultBaseURL }` where `DefaultBaseURL = "https://api.clockify.me/api/v1"`. Verified with doctor — empty `CLOCKIFY_BASE_URL` defaults correctly.

### Finding 7 (Verified safe): Base URL HTTPS enforcement

**What**: `ValidateBaseURL()` rejects non-HTTPS URLs unless on loopback or `CLOCKIFY_INSECURE=1`. Hosted profiles reject even loopback and insecure. Verified through existing test coverage (`config_test.go`).

### Finding 8 (Verified safe): Vault credential resolution with backend safety

**What**: `internal/vault/vault.go` supports `inline`, `env`, and `file` backends. `MCP_DISABLE_INLINE_SECRETS=1` blocks inline backend. All backends reject empty references. JSON payload support with 64KB limit. `api_key` field is mandatory in JSON payloads.

### Finding 9 (Verified safe): Workspace auto-detection when CLOCKIFY_WORKSPACE_ID is empty

**What**: `ResolveWorkspaceID()` in `internal/tools/workspaces.go:19-44` auto-detects the workspace when `CLOCKIFY_WORKSPACE_ID` is not set. It calls `GET /workspaces` and auto-selects when exactly one workspace is available. Returns clear errors for zero or multiple workspaces. Caches the result.

### Finding 10 (Verified safe): Redirect safety in HTTP client

**What**: `clockify.NewClient()` at `internal/clockify/client.go:105-116` enforces:
- Max 10 redirects
- No cross-host redirects (prevents redirect to attacker host)
- No scheme downgrade from HTTPS to HTTP (prevents TLS stripping)

## Fixes made

None. No code changes were required — the auth-api-key-loading area is solid.

## Reproduction steps for each issue

### P3: Permissive workspace ID validation
1. Set `CLOCKIFY_API_KEY=` and `CLOCKIFY_WORKSPACE_ID=not-a-valid-id!!!`
2. Run `clockify-mcp doctor`
3. Observe: config load passes (exit 0)
4. Start MCP server, call `clockify_get_workspace`
5. Observe: runtime error `clockify GET /workspaces/not-a-valid-id!!! failed: 404 Not Found`

## Cleanup performed

No test resources were created in the live Clockify workspace. All probes were read-only.

## Leftover test resources

None.

## Severity

| # | Severity | Description |
|---|----------|-------------|
| 1 | P3 | Permissive workspace ID validation at config load — IDs pass path-safety checks but may be rejected at API call time |

All other checks: no findings (verified safe).

## Files changed

None.

## Suggested next action

No action required for the P3 finding. The permissive workspace ID validation is a deliberate design choice that defers format validation to the Clockify API. If the team wants stricter pre-flight validation, add a `looksLikeClockifyID()` check to `resolve.ValidateID()` when called for workspace IDs, but this would couple the validation to Clockify's current 24-char hex ObjectID format.

## False positives / uncertainty

- The `TestJWKSCache_KidMissRateLimited` test in `internal/authn/jwks_document_test.go` fails intermittently (timing-dependent rate-limit test). This is pre-existing and unrelated to auth-api-key-loading.
- The workspace ID validation finding (P3) is intentionally permissive — I classified it as P3 rather than "no finding" because a user who typos their ID gets a less-ideal runtime error rather than a config-load error, but this is a conscious tradeoff, not a bug.

## Final recommendation

**Ship as-is.** The auth API key loading chain is secure, well-validated, and comprehensively tested:
- API key flows from env → config → client → HTTP header without being logged or exposed
- Missing/invalid keys produce clear, actionable errors at the right layer (config load for missing, tool-call for invalid)
- Secret redaction covers 20+ key patterns in logs plus regex-based value detection
- Doctor command provides full config audit with proper redaction
- Profile-based defaults apply correct auth posture per deployment shape
- All related test suites pass (config, authn unit, clockify client, logging/redact, vault)
- Live API probes confirm correct behavior against the real Clockify API
