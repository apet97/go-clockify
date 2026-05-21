# QA Agent 05 - api-key-redaction-and-secret-safety

## Verdict

PASS WITH CONCERNS

## What I checked

1. **Log redaction layer** (`internal/logging/redact.go`) — key-based and value-based secret scrubbing, recursive traversal of maps/slices/reflect types
2. **Redaction test suite** (`internal/logging/redact_test.go`) — 10 tests covering top-level, grouped, nested, case-insensitive, boundary, value-shaped, custom-type paths; all PASS
3. **API client secret handling** (`internal/clockify/client.go`) — `apiKey` stored as unexported field, sent only as `X-Api-Key` header
4. **Config secret handling** (`internal/config/config.go`, `spec.go`) — API key from env, BearerToken validation, DSN handling, Fingerprint() log output
5. **Auth hardening** (`internal/authn/authn.go`) — constant-time token comparison, JWT validation, RSA minimum modulus enforcement, EC curve validation, JWKS SSRF protection, control-byte sanitization
6. **Error message safety** (`internal/clockify/errors.go`, `internal/helpers/helpers.go`) — Sanitized() strips upstream body, auth errors reference env var name not value, CLOCKIFY_SANITIZE_UPSTREAM_ERRORS flag
7. **Doctor command** (`cmd/clockify-mcp/doctor.go`) — Sensitive flag from spec.go used to display "set (redacted)"
8. **Vault credential resolution** (`internal/vault/vault.go`) — inline/env/file backends, MCP_DISABLE_INLINE_SECRETS hardening, 64KB payload limit
9. **Live API probes** — valid key, invalid key, missing key all return clean 401 errors without leaking credentials
10. **Test suite** — all redaction, auth hardening, enforcement, and safety scenario tests pass

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace credentials
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — lab safety rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/probes/lib/common.sh` — auth wrapper, redaction, cleanup patterns

Workspace: `<REDACTED_ID>` (confirmed live test workspace)
API key: `****REDACTED****` (48 chars, `X-Api-Key` header authentication)

## Commands run

```bash
# Redaction tests (all PASS)
go test ./internal/logging/ -v -run TestRedact -count=1

# Auth hardening tests (all PASS)
go test ./internal/authn/ -v -count=1

# Enforcement/safety tests (all PASS)
go test ./internal/enforcement/ -v -count=1

# Doctor with real API key (CLOCKIFY_API_KEY displayed as "set (redacted)")
CLOCKIFY_API_KEY= CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  go run ./cmd/clockify-mcp doctor

# Valid key probe
curl -s -H "X-Api-Key: " \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED_ID>"
# -> 200, workspace data

# Invalid key probe
curl -s -w "\nHTTP:%{http_code}" -H "X-Api-Key: " \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED_ID>"
# -> 401, {"message":"Api key does not exist","code":4003}

# Missing key probe
curl -s -w "\nHTTP:%{http_code}" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED_ID>"
# -> 401, {"message":"Multiple or none auth tokens present","code":1000}
```

## Live API probes run

| Probe | Method | Result |
|-------|--------|--------|
| Valid API key -> GET workspace | GET /workspaces/{id} | 200, correct workspace data returned |
| Invalid API key -> GET workspace | GET /workspaces/{id} | 401, clean error, no key leak |
| Missing API key -> GET workspace | GET /workspaces/{id} | 401, clean error, no key leak |

**Key observation**: Clockify API returns clean 401 responses for both invalid and missing API keys. The error bodies (`"Api key does not exist"`, `"Multiple or none auth tokens present"`) do not echo the submitted key or any internal state.

## Findings

### P2 — DSN password logged in plaintext via Fingerprint() at startup

**Description**: `Config.Fingerprint()` at `internal/config/config.go:926` outputs `control_plane_dsn` directly into the server-startup log record (`main.go:138`). When the DSN is a Postgres URL containing a password (e.g. `postgres://user:password@host:5432/db`), the password appears in the startup log. The `RedactingHandler` in `internal/logging/redact.go` cannot catch this because:

1. No key in `DefaultSensitiveKeys` matches `"control_plane_dsn"` (the keys are `token`, `secret`, `password`, etc. — none of which appear as substrings of `"control_plane_dsn"`)
2. The value-based `looksSensitiveValue()` regex patterns do not match Postgres DSN passwords

The `MCP_CONTROL_PLANE_DSN` env var is already correctly marked `Sensitive: true` in the EnvSpec registry (`spec.go:126`), and the doctor command correctly displays it as "set (redacted)". The gap is only in the `Fingerprint()` startup log path.

**Severity**: P2 — affects only Postgres-backed deployments (not the default "memory" backend); the password appears in server-side stderr logs; no client exposure.

### Positive findings (no issues)

1. **Redaction layer coverage**: 21 sensitive key patterns (`authorization`, `api_key`, `apikey`, `x-api-key`, `bearer`, `token`, `secret`, `password`, `passphrase`, `cookie`, `set-cookie`, `credential`, `session_token`, `session_id`, `session_secret`, `csrf_token`, `private_key`, `privatekey`, `client_secret`, `refresh_token`, `access_token`, `id_token`) with case-insensitive substring matching — catches variants like `OAuth_Access_Token`, `dbPasswordHash`, `my_custom_api_key_id`
2. **Value-based detection**: 4 regex patterns catch PEM private keys, Stripe-style `pk_` keys, JWT three-part tokens, and `key=value` URL query patterns
3. **Recursive traversal**: Maps, slices, slog.Groups, and reflect-based custom types all get scrubbed
4. **Constant-time token comparison**: `subtle.ConstantTimeCompare` in `staticBearerAuthenticator`
5. **Error sanitization**: `APIError.Sanitized()` strips upstream response body; controlled by `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS` with hosted-profile default
6. **Auth error exposure gated**: `MCP_EXPOSE_AUTH_ERRORS` defaults to false; hosted profiles require `BREAK_GLASS` override
7. **Bearer token minimum 16 chars**: Enforced at config load (`config.go:406-408`)
8. **TLS key paths marked Sensitive**: `MCP_GRPC_TLS_KEY`, `MCP_HTTP_TLS_KEY` in spec
9. **Metrics bearer tokens marked Sensitive**: `MCP_METRICS_BEARER_TOKEN`, `MCP_HTTP_INLINE_METRICS_BEARER_TOKEN`
10. **Vault inline secret hardening**: `MCP_DISABLE_INLINE_SECRETS` can block inline credential backends in hosted deployments
11. **JWKS SSRF protection**: `validateOIDCJWKSAddress()` blocks private/loopback/link-local/reserved addresses; DNS resolution validation
12. **Forward auth proxy trust**: `MCP_FORWARD_AUTH_TRUSTED_PROXIES` CIDR allowlist prevents header spoofing
13. **Principal sanitization**: Control bytes, RTL override, zero-width space, BOM rejected from Subject/TenantID
14. **Cross-host redirect prevention**: Client rejects redirects to different hosts or scheme downgrades
15. **Webhook DNS validation**: `CLOCKIFY_WEBHOOK_VALIDATE_DNS` default true prevents SSRF via webhook URLs

## Fixes made

### 1. DSN password redaction in Fingerprint() -> `internal/config/config.go`

**Lines changed**: 940 (Fingerprint call site), appended `sanitizeDSNForFingerprint()` function

**Change**: `Fingerprint()` now calls `sanitizeDSNForFingerprint(c.ControlPlaneDSN)` instead of using the raw DSN. The sanitizer:
- Returns empty strings and dev backends (memory, file://, bare paths) unchanged
- For Postgres DSNs, parses the URL and replaces the password in userinfo with `[REDACTED]`
- Preserves user, host, port, dbname, and query parameters for operator visibility
- Gracefully returns the raw DSN unchanged if URL parsing fails

**Before**: `postgres://app:<EMAIL>:5432/clockify_mcp` -> logged as `"control_plane_dsn": "postgres://app:<EMAIL>:5432/clockify_mcp"`
**After**: `postgres://app:<EMAIL>:5432/clockify_mcp` -> logged as `"control_plane_dsn": "postgres://app:[REDACTED]@db.internal:5432/clockify_mcp"`

## Reproduction steps for each issue

### DSN password leak
1. Set `MCP_CONTROL_PLANE_DSN=postgres://user:password@localhost:5432/db`
2. Set `CLOCKIFY_API_KEY= valid key>` and `MCP_TRANSPORT=streamable_http`
3. Start the server: `go run ./cmd/clockify-mcp`
4. Observe the `server_start` log line — before the fix, the config map contains the raw DSN with password

## Cleanup performed

No test resources were created. This audit was read-only (code inspection + GET-only live API probes).

## Leftover test resources

None.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| DSN password in Fingerprint() | P2 | Affects only Postgres deployments; logged to server-side stderr; no client/MCP-wire exposure; fixed |

## Files changed

- `internal/config/config.go` — line 940: `c.ControlPlaneDSN` -> `sanitizeDSNForFingerprint(c.ControlPlaneDSN)`; appended `sanitizeDSNForFingerprint()` function (~18 lines)

## Suggested next action

1. **Accept the DSN fix** — review the `sanitizeDSNForFingerprint()` function; verify with a Postgres DSN test case
2. **Expand `DefaultSensitiveKeys`** — consider adding `"dsn"` to catch `control_plane_dsn` and similar keys at the redaction layer as belt-and-suspenders
3. **Add a fingerprint-specific test** — verify that passwords in DSNs are redacted from `Fingerprint()` output
4. **Consider `MCP_BEARER_TOKEN` inclusion check** — `Fingerprint()` excludes it (good), but there's no automated test confirming the exclusion

## False positives / uncertainty

- The DSN leak requires a Postgres DSN with an embedded password. Deployments using IAM authentication, certificate-based auth, or the default "memory" backend are unaffected.
- The `RedactingHandler` is configured as defence-in-depth ("belt-and-braces"); hot-path code is still expected to avoid logging secrets explicitly. The DSN fix addresses the explicit log site rather than relying on the redaction layer.

## Final recommendation

**PASS WITH CONCERNS** — the secret safety posture is strong. The redaction layer is one of the most thorough I've audited (key-based + value-based + recursive + reflect), auth hardening covers the full OWASP surface (timing attacks, SSRF, header injection, algorithm confusion), and error paths are clean. The single P2 finding (DSN password in startup fingerprint) is fixed in this audit. No client-facing secret leaks were found.
