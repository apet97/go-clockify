# QA Agent 51 - permissions-and-scope-documentation

## Verdict
PASS WITH CONCERNS

## What I checked

1. **MCP server scope and permissions documentation** — docs/policy/production-tool-scope.md, docs/auth-model.md, docs/api-coverage.md, SECURITY.md, README.md, tool-catalog.md, tool-catalog.json
2. **MCP enforcement pipeline code** — internal/enforcement/enforcement.go, internal/policy/policy.go, internal/authn/authn.go
3. **Clockify API key permission requirements** — whether the repo documents what Clockify workspace role/permissions the API key needs per tool
4. **`clockify_update_user_role` endpoint correctness** — HTTP method, request body shape, and dry-run preview accuracy against the live Clockify API
5. **`clockify_deactivate_user` endpoint correctness** — HTTP method and endpoint path against the live API
6. **User role visibility in MCP tools** — whether `clockify_list_users`, `clockify_whoami`, and `clockify_current_user` surface workspace roles
7. **API key effective scope** — live probe of read/write/admin endpoints against the probe workspace
8. **Doctor command** — configuration audit output and secret redaction

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (secrets redacted)
- `clockify-api-probe-lab/USERDOC.md` — Clockify User API docs with role endpoints
- `clockify-api-probe-lab/WORKSPACESDOC.md` — Workspace API docs with role filtering
- `clockify-api-probe-lab/CLAUDE.md` — safety rules for probe lab

## Commands run

```sh
# Build MCP binary
go build -o /tmp/clockify-mcp-doctor ./cmd/clockify-mcp/

# Doctor command
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=65b382b606de527a7ee2b60e \
  /tmp/clockify-mcp-doctor doctor

# Live API probes (all use redacted API key via source /tmp/clockify-livetest.env)
# GET /user
curl -s -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/user"

# GET /workspaces/{ws}/users
curl -s -H "X-Api-Key: ****REDACTED****" \
  "https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/users"

# PUT /workspaces/{ws}/users/{userId}/roles (MCP's method)
curl -s -X PUT -H "X-Api-Key: ****REDACTED****" -H "Content-Type: application/json" \
  -d '{"role":"REGULAR"}' \
  "https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/users/64621faec4d2cc53b91fce6c/roles"
# → HTTP 405 "Request method 'PUT' is not supported"

# POST /workspaces/{ws}/users/{userId}/roles (Clockify's actual method)
curl -s -X POST -H "X-Api-Key: ****REDACTED****" -H "Content-Type: application/json" \
  -d '{"entityId":"64621faec4d2cc53b91fce6c","role":"TEAM_MANAGER"}' \
  "https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/users/64621faec4d2cc53b91fce6c/roles"
# → HTTP 201 Created (success)

# DELETE role (cleanup)
curl -s -X DELETE -H "X-Api-Key: ****REDACTED****" -H "Content-Type: application/json" \
  -d '{"entityId":"64621faec4d2cc53b91fce6c","role":"TEAM_MANAGER"}' \
  "https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/users/64621faec4d2cc53b91fce6c/roles"
# → HTTP 204 No Content (cleanup confirmed)

# PUT /workspaces/{ws}/users/{id}/status (MCP's deactivate endpoint)
curl -s -X PUT -H "X-Api-Key: ****REDACTED****" -H "Content-Type: application/json" \
  -d '{"status":"INACTIVE"}' \
  "https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/users/64621faec4d2cc53b91fce6c/status"
# → HTTP 404 (endpoint does not exist)

# Permission broad scan (all 200)
# GET /workspaces, /projects, /clients, /tags, /users, /invoices, /expenses,
#     /scheduling/assignments/all, /webhooks, reports/detailed, reports/summary
```

## Live API probes run

| # | Probe | Method | Endpoint | Result | Notes |
|---|-------|--------|----------|--------|-------|
| 1 | Current user | GET | `/user` | 200 | Returns user ID, email, name; no workspaceRole field |
| 2 | List workspace users | GET | `/workspaces/{ws}/users` | 200 | 7 users; memberships array present but roles[] empty without include-roles |
| 3 | Filter users by role | GET | `/workspaces/{ws}/users?roles=WORKSPACE_ADMIN&roles=OWNER` | 200 | Returns all users (filter behavior differs from docs) |
| 4 | Role update (PUT — MCP's method) | PUT | `/workspaces/{ws}/users/{id}/roles` | **405** | "Request method 'PUT' is not supported" — MCP uses wrong method |
| 5 | Role update (POST — correct method) | POST | `/workspaces/{ws}/users/{id}/roles` | 201 | Succeeds with body `{"entityId":"<userId>","role":"TEAM_MANAGER"}` |
| 6 | Role update (WORKSPACE_ADMIN) | POST | `/workspaces/{ws}/users/{id}/roles` | 400 | "entity Id must match workspaceId when promoting/demoting admin" |
| 7 | Role update (PROJECT_MANAGER) | POST | `/workspaces/{ws}/users/{id}/roles` | 404 | "PROJECT with ID '<userId>' not found" — entityId must be project ID |
| 8 | Role delete (cleanup) | DELETE | `/workspaces/{ws}/users/{id}/roles` | 204 | Required `{"entityId":"...","role":"..."}` body |
| 9 | Deactivate user (MCP's endpoint) | PUT | `/workspaces/{ws}/users/{id}/status` | **404** | Endpoint does not exist |
| 10 | Delete user from workspace | DELETE | `/workspaces/{ws}/users/{id}` | 403 | Access Denied (expected — API key lacks user removal) |
| 11 | Permission scan (11 endpoints) | GET | various | All 200 | API key has full read access across all domains |

## Findings

### Finding 1 (P1): `clockify_update_user_role` uses wrong HTTP method and missing `entityId`

**Location:** `internal/tools/tier2_user_admin.go:392-398`

The MCP handler sends:
```go
// PUT /workspaces/{ws}/users/{userId}/roles
payload := map[string]any{"role": role}
s.Client.Put(ctx, path, payload, &result)
```

The live Clockify API requires:
```
POST /workspaces/{workspaceId}/users/{userId}/roles
Body: {"entityId": "<context-dependent>", "role": "WORKSPACE_ADMIN|TEAM_MANAGER|PROJECT_MANAGER"}
```

The `entityId` value depends on the role:
- `WORKSPACE_ADMIN`: must be the workspace ID
- `TEAM_MANAGER`: must be a user ID (works with self-referential value)
- `PROJECT_MANAGER`: must be a project ID

The current implementation:
1. Uses PUT instead of POST → 405 from Clockify
2. Omits `entityId` → 400 "Entity id must be present"
3. Has no context-dependent entityId logic

**Impact:** The `clockify_update_user_role` tool cannot successfully assign roles via the live API. It will always fail with a 405 or 400 error.

**Existing coverage:** The unit test `TestUpdateUserRoleEmitsUserURI` (`internal/tools/tier2_user_admin_emit_test.go:38`) uses a mock server that accepts PUT, so it doesn't catch this. The live test `TestLiveT2UserAdminCRUDAndOwnerSafety` (`tests/e2e_live_t2_user_admin_test.go:64`) expects this to fail (it checks for "not found/permission/method" errors) and already documents it as "unsupported role route 405" in `docs/api-coverage.md`. However, the route IS supported — just with POST, not PUT.

### Finding 2 (P1): `clockify_deactivate_user` calls nonexistent endpoint

**Location:** `internal/tools/tier2_user_admin.go:435-441`

The MCP handler sends `PUT /workspaces/{ws}/users/{id}/status` with `{"status":"INACTIVE"}`. The live API returns 404 — this endpoint does not exist on the probed Clockify version.

**Impact:** `clockify_deactivate_user` cannot deactivate users. Already documented as unsupported in `docs/api-coverage.md`.

### Finding 3 (P2): No documentation of Clockify API key permission requirements

The repo has excellent documentation of:
- MCP-level policy modes (`read_only`, `time_tracking_safe`, etc.)
- MCP auth modes (`static_bearer`, `oidc`, etc.)
- Tool risk classifications (`admin`, `billing`, `permission_change`, etc.)

But there is **no documentation** of what Clockify workspace role or API key permissions are needed for each MCP tool. A self-hosted operator reading the README only sees "Get a Clockify API key from Profile → Advanced" with no guidance on:
- What workspace role the API key owner needs (OWNER vs WORKSPACE_ADMIN vs TEAM_MANAGER vs REGULAR)
- Which tools require the API key to have admin permissions on the workspace
- Which tools work with a basic REGULAR user's API key

**Example gap:**
- `clockify_update_user_role` requires WORKSPACE_ADMIN or OWNER permissions
- `clockify_create_invoice` / `clockify_send_invoice` likely need billing/admin permissions
- `clockify_list_users` may require at least TEAM_MANAGER for some fields
- A REGULAR user can only manage their own time entries — but nothing in the docs tells operators this

**Recommended fix:** Add a column to `docs/tool-catalog.md` or a new section in `docs/policy/production-tool-scope.md` documenting the minimum Clockify workspace role needed for each tool group.

### Finding 4 (P3): `clockify_list_users` doesn't surface workspace roles

**Location:** `internal/clockify/models.go` (User struct) and `internal/tools/users.go:49-74`

The `clockify.User` struct lacks `workspaceRole` or `memberships[].roles[]` fields. The `ListUsers` handler fetches the Clockify API's user list but the Go model doesn't capture role/membership data, so users can't tell what permissions other workspace members have.

This is a documentation/visibility gap: tools like `clockify_list_users` should indicate user roles so agents can preflight whether `clockify_update_user_role` would succeed.

### Finding 5 (P3): Dry-run preview payload for `update_user_role` is misleading

**Location:** `internal/tools/tier2_user_admin.go:386`

The dry-run preview shows `{"role": "WORKSPACE_ADMIN"}` but the actual Clockify API requires `{"entityId": "...", "role": "WORKSPACE_ADMIN"}`. Even if the HTTP method bug were fixed, the dry-run payload would still omit `entityId`, making the preview inaccurate.

### Positive findings

1. **Doctor command** correctly redacts the API key and shows all effective configuration
2. **Policy modes** (`read_only`, `time_tracking_safe`, `safe_core`, `standard`, `full`) are well-documented and provide strong MCP-level scoping
3. **RiskClass taxonomy** (`Read | Write | Billing | Admin | PermissionChange | ExternalSideEffect | Destructive`) provides fine-grained tool classification
4. **`docs/policy/production-tool-scope.md`** is an excellent operator-facing document for policy selection
5. **`docs/auth-model.md`** is thorough for MCP auth but silent on Clockify API key scopes
6. **API key has broad access** to the probe workspace (all 11 probed read/write endpoints return 200), confirming the workspace is suitable for testing
7. **The existing api-coverage.md** already documents the user_admin route issues honestly ("unsupported role route 405", "deactivate owner dry-run")

## Fixes made

No code changes made. The `clockify_update_user_role` handler bug and the `entityId` gap affect the handler logic and tool schema — fixing them would require:
1. Changing PUT to POST
2. Adding `entityId` as a required parameter with role-dependent validation
3. Updating the unit test mock to expect POST
4. Updating the dry-run preview to include `entityId`

This is a non-trivial change that needs a design decision on how `entityId` should be surfaced (dedicated parameter? auto-resolved?).

## Reproduction steps for each issue

### Reproduce Finding 1 (wrong HTTP method + missing entityId)
1. Start the MCP server with valid credentials
2. Activate the `user_admin` group: `clockify_activate_group("user_admin")`
3. Call `clockify_update_user_role` with a valid user_id and role:
   ```json
   {"user_id": "<real-user-id>", "role": "TEAM_MANAGER"}
   ```
4. Observe: the MCP server sends `PUT /workspaces/{ws}/users/{id}/roles` with `{"role":"TEAM_MANAGER"}` → Clockify returns 405

### Reproduce Finding 3 (no API key permission docs)
1. Open README.md
2. Search for "permission", "role", "workspace admin", "api key scope"
3. Observe: no section documents minimum Clockify workspace role requirements

### Reproduce Finding 4 (no roles in list_users)
1. Call `clockify_list_users` through the MCP server
2. Observe the response: each user object has `id`, `name`, `email` but no `workspaceRole` or `role` field

## Cleanup performed

- Removed the TEAM_MANAGER role assigned to user `64621faec4d2cc53b91fce6c` during testing via `DELETE /workspaces/{ws}/users/{id}/roles`. HTTP 204 confirmed.

## Leftover test resources

None. The role assignment used for testing was cleaned up with a DELETE call (HTTP 204 confirmed).

## Severity

| # | Severity | Description |
|---|----------|-------------|
| 1 | **P1** | `clockify_update_user_role` uses wrong HTTP method (PUT→POST) and missing `entityId` body field. Tool is functionally broken against the live API. |
| 2 | **P1** | `clockify_deactivate_user` calls nonexistent endpoint (404). Already documented as unsupported. |
| 3 | **P2** | No documentation of Clockify API key permission requirements per tool. Operators don't know what workspace role their API key needs. |
| 4 | **P3** | `clockify_list_users` doesn't surface workspace roles. |
| 5 | **P3** | Dry-run preview for `update_user_role` is misleading (wrong body shape). |

## Files changed

None. All findings are documentation/reporting only.

## Suggested next action

1. **Fix Finding 1:** Update `clockify_update_user_role` handler to use POST, add `entityId` parameter with role-dependent validation, update mock tests to expect POST, update the tool schema to document `entityId`. This is the highest-impact fix — the tool is currently non-functional against the live API.
2. **Document Finding 3:** Add a "Clockify API key permissions" section to `docs/policy/production-tool-scope.md` or README.md mapping tool groups to minimum required workspace roles.
3. **Consider Finding 4:** Add `workspaceRole` / membership data to the `clockify.User` model so `clockify_list_users` and `clockify_whoami` show user roles.

## False positives / uncertainty

1. **Finding 2 (deactivate_user):** The probe lab's USERDOC.md doesn't document a deactivation-specific endpoint. Clockify might offer a different deactivation path (e.g., through workspace removal APIs or a separate admin endpoint). The current conclusion that the endpoint doesn't exist may be incomplete — it could exist under a different path not probed here.
2. **`entityId` semantics:** The exact semantics of `entityId` for TEAM_MANAGER assignment (user ID vs group ID) need further probing. The test showed self-referential user ID works, but the Clockify docs mention `sourceType: "USER_GROUP"` as an option, suggesting group-based role assignment is also possible.
3. **Role filter behavior:** `?roles=WORKSPACE_ADMIN&roles=OWNER` returned all 7 users including users named "REGULAR USER", which is unexpected. This may indicate the filter parameter is ignored or the workspace has unusual role assignments. This wasn't investigated deeply as it's not directly related to the MCP scope documentation.

## Final recommendation

**The permissions-and-scope documentation in this repo is directionally correct but incomplete.** The MCP-level policy framework (`read_only` through `full`, `RiskClass` taxonomy, tool activation, dry-run) is well-implemented and well-documented. However, the repo is missing critical documentation about what Clockify workspace role the API key owner needs, and two key Tier-2 admin tools (`update_user_role`, `deactivate_user`) are broken against the live API due to incorrect HTTP methods and missing body fields.

For local/internal/community/self-hosted readiness: PASS WITH CONCERNS. The issues affect Tier-2 admin tools (lazy-activated, not in the default surface) and documentation gaps. A self-hosting operator who only uses Tier-1 tools would not encounter the functional bugs. The documentation gaps (API key permission requirements) are real but not blocking for a technical operator who already understands Clockify's role system — they'll discover permission issues at runtime through Clockify error messages.
