# QA Agent 30 - no-real-email-safety

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Repo-wide scan for hardcoded real email addresses** — searched for `@gmail.com`, `@outlook.com`, `@yahoo.com`, `@hotmail.com`, and all email-like patterns across every file in the repository.
2. **MCP tool email exposure surface** — tested `clockify_current_user`, `clockify_list_users`, `clockify_resolve_name`, and `clockify_whoami` via live MCP stdio session to verify what email data they return.
3. **Invite-user tool absence** — confirmed no invite-user or create-user MCP tool exists in any tier (Tier 1 or Tier 2).
4. **Email parameter filtering** — verified `clockify_list_users` rejects unknown properties like `email` (strict schema validation).
5. **Email resolution safety** — tested resolving users by both real and fake email addresses through `clockify_resolve_name`.
6. **Resolve cache behavior** — verified email lookups are explicitly excluded from the resolve cache (privacy/safety measure).
7. **Risk classification for email-delivery tools** — checked `RiskExternalSideEffect` annotations on `clockify_send_invoice` and `clockify_test_webhook`.
8. **Vault/redaction presence** — checked whether any email sanitization or redaction mechanism exists in the Vault package or output pipeline.
9. **Unit test suite** — ran `go test ./...` across all packages; all pass.
10. **MCP doctor/config command** — ran `clockify-mcp doctor` successfully.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (never echoed)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/probes/lib/common.sh` — helper functions (inspected for conventions)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — agent rules

## Commands run

```bash
# Build
go build ./cmd/clockify-mcp/

# Doctor
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> ./clockify-mcp doctor

# Unit tests
go test ./...  # all 28 packages ok, no failures

# Live MCP stdio smoke tests (tool calls via MCP protocol)
# - clockify_current_user
# - clockify_list_users
# - clockify_resolve_name with real email, fake email, non-email string
# - clockify_resolve_name with entity_type=user, name_or_id=<redacted>
# - tools/list to confirm no invite-user tool

# Live API calls
curl -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/user"
curl -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/workspaces/<REDACTED>/users?page-size=50"
curl -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/workspaces/<REDACTED>/users?email=<redacted>"
```

## Live API probes run

### Probe 1: Current user email exposure
- **Tool**: `clockify_current_user`
- **Result**: Returns the real email address of the authenticated user in the `email` field of the `clockify.User` struct. The email is exposed in both the structured output and the text content.
- **Verdict**: This is the Clockify API's design — the `/user` endpoint always returns the email field. The MCP server passes it through without redaction.

### Probe 2: User list email exposure
- **Tool**: `clockify_list_users`
- **Result**: Returns all workspace users with their real email addresses. All 7 users in the probe workspace have their emails exposed, including 4 Gmail addresses and 3 disposable-email-service addresses.
- **Verdict**: This is the Clockify API's design — the `/workspaces/{id}/users` endpoint always returns user emails. No redaction layer exists in the MCP server.

### Probe 3: Email-based user resolution
- **Tool**: `clockify_resolve_name` with `entity_type=user`, `name_or_id=<real email>`
- **Result (real email)**: Successfully resolved a real user email to a user ID with status `exact_match`.
- **Result (fake email)**: Gracefully returned `not_found` status with a helpful error message suggesting `clockify_list_users`.
- **Result (non-email string)**: Treated as a name search via `strict-name-search=true`.
- **Verdict**: Email resolution works as designed. The `not_found` handling for fake emails is graceful and safe.

### Probe 4: Invite-user tool absence
- **Result**: No `clockify_invite_user` or similar tool exists in Tier 1 (40 tools) or Tier 2 `user_admin` group (8 tools). The invite route (`POST /workspaces/{id}/users`) is only accessed via a raw API probe in the live E2E test suite (`TestLiveT2UserInviteValidationProbe`), which uses `send-email=false` and an empty email address.
- **Verdict**: The MCP server deliberately does NOT expose the invite-user route as a tool. This is a positive safety finding.

### Probe 5: Email parameter filtering on list_users
- **Result**: `clockify_list_users` rejects an `email` parameter with `invalid params at /email: unknown property`. The tool schema only accepts `page` and `page_size`.
- **Verdict**: Strict schema validation prevents email-based filtering through the MCP tool. The Clockify API itself supports `?email=` filtering, but the MCP tool schema does not expose it.

## Findings

### F1: Real email hardcoded in documentation (P2)
- **File**: `docs/testing/standard-http-dogfood-2026-05-07.md:17`
- **Detail**: The file contained `alpettest1@gmail.com` — a real Gmail address used as a test user identifier in a dogfood evidence table.
- **Risk**: Real emails in a public repository are scrapable and could lead to spam or phishing targeting the test user.
- **Fixed**: Replaced with `<REDACTED>`.

### F2: No email redaction layer in tool outputs (P2)
- **Detail**: The MCP server passes through the `email` field from `clockify.User` structs without any redaction, sanitization, or filtering. This means `clockify_current_user`, `clockify_list_users`, `clockify_whoami`, and `clockify_resolve_name` (candidates path) all expose real email addresses in their outputs.
- **Risk**: Anyone with access to the MCP server can see all workspace users' email addresses. For self-hosted deployments, this is typically acceptable (the operator already has API key access), but it means the MCP server is not safer than raw API access regarding email privacy.
- **Mitigation**: The probe workspace contains test-only email addresses (Gmail aliases and disposable-email-service addresses), so no real production user emails are at risk in testing. For production deployments, operators should be aware that user emails are not filtered.

### F3: Email resolution is possible with any email address (P3 - informational)
- **Detail**: `clockify_resolve_name` with `entity_type=user` accepts any string that looks like an email (`@` followed by `.`) and passes it to the Clockify API as an `email` query parameter. While fake emails gracefully return `not_found`, real emails resolve to their user IDs.
- **Risk**: Low. This is the intended behavior of the resolve tool. Email resolution is a documented feature for convenience.
- **Safeguard**: Email lookups are explicitly excluded from the resolve cache (`cacheableResolveKey` in `internal/tools/resolve_cache.go:81` returns `false` for email-looking strings), so they are never cached and always hit the upstream API.

## Fixes made

1. **Redacted real email in docs**: Replaced `alpettest1@gmail.com` with `<REDACTED>` in `docs/testing/standard-http-dogfood-2026-05-07.md` line 17.

## Reproduction steps for each issue

### F1 (Real email in docs):
1. `grep -rn '@gmail\.com' docs/testing/standard-http-dogfood-2026-05-07.md`
2. Observe the real Gmail address on line 17.
3. Fixed by redacting the email.

### F2 (No email redaction):
1. Start the MCP server with a valid API key and workspace ID.
2. Call `clockify_list_users`.
3. Observe that all user email addresses are returned in plain text.
4. No configuration option exists to redact or omit the email field.

### F3 (Email resolution):
1. Start the MCP server with a valid API key and workspace ID.
2. Call `clockify_resolve_name` with `entity_type=user` and `name_or_id=<any valid workspace user email>`.
3. The tool returns the user's Clockify ID.

## Cleanup performed

- Removed compiled `clockify-mcp` binary from the worktree.
- No test resources were created in the probe workspace (all probes were read-only).
- No Docker containers, images, or volumes were created.

## Leftover test resources

None. All probes were read-only. No test resources were created in the probe workspace.

## Severity

| ID | Severity | Description |
|----|----------|-------------|
| F1 | P2 | Real Gmail address in documentation file |
| F2 | P2 | No email redaction in MCP tool outputs |
| F3 | P3 | Email resolution exposes user IDs for real emails (by design) |

## Files changed

- `docs/testing/standard-http-dogfood-2026-05-07.md` — redacted real email address

## Suggested next action

1. Consider adding an optional `CLOCKIFY_REDACT_EMAILS` configuration variable that, when enabled, replaces email fields with `"<redacted>"` in tool outputs. This would give operators of self-hosted/delegated deployments a privacy lever.
2. Run the public-content audit script (`scripts/test-check-public-content-audit.sh`) to catch similar real-email-in-docs issues in the future. The script already configures `user.email` as `test@example.invalid` but doesn't scan for hardcoded emails in documentation.
3. Add a pre-commit or CI check that greps for common real-email domains (`@gmail.com`, `@outlook.com`, `@yahoo.com`, `@hotmail.com`) in the repository and blocks commits containing them.

## False positives / uncertainty

- The probe workspace contains test-only email addresses that appear to be registered test accounts, not real user accounts belonging to external individuals. The risk of email exposure from tools is mitigated by the fact that the MCP server requires an API key (same access level as having the email visibility directly via the Clockify API).
- `looksLikeEmail` is intentionally permissive (just checks for `@` + `.` after `@`). This is appropriate for its use as a routing heuristic (deciding whether to query by `email` vs `name` parameter). A stricter email validation would add complexity for no security benefit since the API itself handles validation.

## Final recommendation

The repo is in good shape regarding email safety. The single hardcoded real email in documentation has been fixed. The MCP server does not expose any invite-user or user-creation tools, and email-based resolution has appropriate safeguards (no caching of email lookups, graceful not-found handling for fake emails).

The primary concern (F2) is the lack of an email redaction option for delegated/self-hosted deployments where the MCP server operator may not want downstream tool consumers to see all user emails. This is a feature request rather than a bug — the current behavior matches raw Clockify API access. For the immediate launch scope, this is acceptable.
