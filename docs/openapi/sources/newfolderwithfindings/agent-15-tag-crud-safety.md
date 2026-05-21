# QA Agent 15 - tag-crud-safety

## Verdict
PASS WITH CONCERNS

## What I checked

1. **MCP tool surface for tags** — Verified which tag tools are registered in the MCP server registry vs what the Clockify API supports.
2. **Live API CRUD for tags** — Tested all 5 CRUD operations (LIST, CREATE, GET, UPDATE, DELETE) directly against the live Clockify API in the probe workspace.
3. **MCP server build and startup** — Built the server from source, ran `doctor` audit, and tested MCP `tools/list` + `tools/call` against the live API via stdio transport.
4. **ListTags query parameter handling** — Confirmed the implementation only passes `page`/`page-size` to the API, silently ignoring `name`, `archived`, `sort-column`, `sort-order`, `strict-name-search`, and `excluded-ids`.
5. **CreateTag input validation** — Checked empty name, 101-char name, and dry_run mode.
6. **Error handling** — Tested nonexistent tag GET/DELETE, duplicate name CREATE, bad API key auth, and unknown-tool MCP errors.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key (****REDACTED****) and workspace ID
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TAGDOC.md` — API documentation for tag endpoints
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — Lab rules and conventions

## Commands run

```bash
# Build and doctor
go build ./cmd/...
go run ./cmd/clockify-mcp doctor --strict

# MCP stdio smoke test (initialize, tools/list, tools/call)
printf '...' | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp

# Live API probes
curl -s -X GET    "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags" -H "X-Api-Key: <REDACTED>"
curl -s -X POST   "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags" ...
curl -s -X GET    "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags/<ID>" ...
curl -s -X PUT    "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags/<ID>" ...
curl -s -X DELETE "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags/<ID>" ...

# MCP tools/call via Python subprocess
python3 -c "..."  # initialize + tools/list + tools/call for tag tools

# Tests
go test ./internal/tools/ -run "TestCreateTag|TestListTags|TestHappyPath_CreateTag" -v -count=1
```

## Live API probes run

| # | Probe | Method | Endpoint | Result |
|---|-------|--------|----------|--------|
| 1 | List all tags | GET | /v1/workspaces/{ws}/tags | 200, returned 31 tags |
| 2 | Create tag | POST | /v1/workspaces/{ws}/tags | 201, tag created with id |
| 3 | Get tag by ID | GET | /v1/workspaces/{ws}/tags/{id} | 200, correct tag returned |
| 4 | Get nonexistent tag | GET | /v1/workspaces/{ws}/tags/nonexistent | 400, "Tag doesn't belong to Workspace" |
| 5 | Update tag (rename) | PUT | /v1/workspaces/{ws}/tags/{id} | 200, name updated |
| 6 | Update tag (archive) | PUT | /v1/workspaces/{ws}/tags/{id} | 200, archived=true |
| 7 | Delete tag | DELETE | /v1/workspaces/{ws}/tags/{id} | 200, returns deleted tag |
| 8 | Delete nonexistent | DELETE | /v1/workspaces/{ws}/tags/nonexistent | 400, "Tag doesn't belong to Workspace" |
| 9 | Create empty name | POST | /v1/workspaces/{ws}/tags | 400, "Tag name is required" |
| 10 | Create missing name | POST | /v1/workspaces/{ws}/tags | 400, "Tag name is required" |
| 11 | Create duplicate name | POST | /v1/workspaces/{ws}/tags | 400, "Tag with name 'Epic' already exists" |
| 12 | Create 101-char name | POST | /v1/workspaces/{ws}/tags | 400, "Tag name cannot be longer than 100." |
| 13 | Pagination (page-size=5) | GET | /v1/workspaces/{ws}/tags?page=1&page-size=5 | 200, returned 5 items |
| 14 | Sort by NAME ASCENDING | GET | /v1/workspaces/{ws}/tags?sort-column=NAME&sort-order=ASCENDING | 200, sorted correctly |
| 15 | Search by name | GET | /v1/workspaces/{ws}/tags?name=qa-agent-15 | 200, returned 0 (correct, tag deleted) |
| 16 | Bad API key | GET | /v1/workspaces/{ws}/tags | 401, "Api key does not exist" |
| 17 | MCP list_tags via stdio | tools/call | clockify_list_tags | OK, returned tag list |
| 18 | MCP create_tag via stdio | tools/call | clockify_create_tag | OK, created tag |
| 19 | MCP create_tag dry_run | tools/call | clockify_create_tag {dry_run:true} | OK, dry run preview returned |
| 20 | MCP get_tag (nonexistent) | tools/call | clockify_get_tag | Error: "unknown tool: clockify_get_tag" |
| 21 | MCP update_tag (nonexistent) | tools/call | clockify_update_tag | Error: "unknown tool: clockify_update_tag" |
| 22 | MCP delete_tag (nonexistent) | tools/call | clockify_delete_tag | Error: "unknown tool: clockify_delete_tag" |
| 23 | MCP resolve_name for tag | tools/call | clockify_resolve_name {entity_type:tag} | OK, resolved "Epic" -> ID |

## Findings

### F1 (P1): Missing CRUD tools for tags — get, update, delete

The Clockify API supports the full tag CRUD surface:

| Operation | API Endpoint | MCP Tool | Status |
|-----------|-------------|----------|--------|
| List tags | GET /v1/workspaces/{ws}/tags | `clockify_list_tags` | EXISTS |
| Create tag | POST /v1/workspaces/{ws}/tags | `clockify_create_tag` | EXISTS |
| Get tag | GET /v1/workspaces/{ws}/tags/{id} | — | **MISSING** |
| Update tag | PUT /v1/workspaces/{ws}/tags/{id} | — | **MISSING** |
| Delete tag | DELETE /v1/workspaces/{ws}/tags/{id} | — | **MISSING** |

MCP `tools/call` for `clockify_get_tag`, `clockify_update_tag`, and `clockify_delete_tag` all return:
```
{"code": -32602, "message": "unknown tool: clockify_<operation>"}
```

The `clockify_resolve_name` tool can resolve tag names to IDs, serving as a partial workaround for the missing `get_tag`. But there is no way to update tag name/archived status or delete tags through MCP.

Compare with projects: `clockify_get_project`, `clockify_create_project` both exist. The tag domain is incomplete.

**File:** `internal/tools/registry.go` — no entries for get/update/delete tag handlers.  
**File:** `internal/tools/tags.go` — only `ListTags` and `CreateTag` are implemented.

### F2 (P2): ListTags ignores API query filter parameters

The `ListTags` implementation only constructs query params for `page` and `page-size`:

```go
// internal/tools/tags.go:22-26
page, pageSize := paginationFromArgs(args)
query := map[string]string{
    "page":      strconv.Itoa(page),
    "page-size": strconv.Itoa(pageSize),
}
```

The Clockify API supports these additional query parameters that are silently ignored:
- `name` — substring filter by tag name
- `strict-name-search` — exact name match toggle
- `archived` — filter by archived status
- `sort-column` — "ID" or "NAME"
- `sort-order` — "ASCENDING" or "DESCENDING"
- `excluded-ids` — exclude specific tag IDs

The tool schema (via `paginationSchema`) also doesn't expose these as input parameters, so callers cannot pass them.

### F3 (P3): CreateTag did not validate name length (FIXED)

The Clockify API enforces a 100-character maximum for tag names. The MCP `CreateTag` only checked for empty names and forwarded all non-empty names to the API. A 101-character name would hit the API, return a 400 error with "Tag name cannot be longer than 100.", and produce an opaque tool error.

**Fix applied:** Added `len(name) > 100` check in `CreateTag` that returns a clear error before hitting the API. Verified with MCP tools/call — 101-char name now returns "tag name cannot be longer than 100 characters" at the MCP layer.

### F4 (P3): No negative test coverage for tag tools

The test suite for tag tools covers only happy paths:
- `TestCreateTag` — creates a tag, verifies response
- `TestListTags` — verifies default pagination
- `TestHappyPath_CreateTag` — dispatch-layer test

No tests for: empty name, 101-char name, duplicate name handling, API error propagation, or query parameter passthrough.

## Fixes made

**File:** `internal/tools/tags.go`
**Change:** Added name length validation (max 100 characters) to `CreateTag`, matching the upstream API constraint.

```diff
+	if len(name) > 100 {
+		return ResultEnvelope{}, fmt.Errorf("tag name cannot be longer than 100 characters")
+	}
```

## Reproduction steps for each issue

### F1: Missing CRUD
1. Start the MCP server with valid credentials
2. Send `tools/call` for `clockify_get_tag` with any `id` argument
3. Observe: `{"code": -32602, "message": "unknown tool: clockify_get_tag"}`
4. Repeat for `clockify_update_tag` and `clockify_delete_tag`

### F2: Missing query params
1. Call `clockify_list_tags` with any `name` parameter via MCP
2. Observe: the `name` parameter is silently ignored; all tags are returned
3. Inspect `internal/tools/tags.go:22-26` — only `page` and `page-size` are passed to the API

### F3: Name length (FIXED)
1. Before fix: call `clockify_create_tag` with `name` = 101 "x" characters
2. Observed: API returns 400 "Tag name cannot be longer than 100."
3. After fix: MCP returns "tag name cannot be longer than 100 characters" without hitting API

## Cleanup performed

All `qa-agent-15-*` test tags were deleted during the session:

| Tag ID | Name | Action |
|--------|------|--------|
| `<REDACTED_ID>` | qa-agent-15-tag-create-test | Created, updated, archived, deleted |
| `<REDACTED_ID>` | qa-agent-15-mcp-test | Created via MCP, deleted via API |
| `<REDACTED_ID>` | qa-agent-15-archived-test | Created, archived, deleted |
| `<REDACTED_ID>` | qa-agent-15-validation-test | Created, deleted |

## Leftover test resources

None. All qa-agent-15-* resources were cleaned up.

## Severity

| ID | Severity | Description |
|----|----------|-------------|
| F1 | P1 | Missing get/update/delete tag tools — API supports them, MCP doesn't expose them |
| F2 | P2 | ListTags doesn't pass through name/archived/sort/excluded-ids query params |
| F3 | P3 | CreateTag didn't validate name length (FIXED) |
| F4 | P3 | No negative test coverage for tag error paths |

## Files changed

- `internal/tools/tags.go` — Added name length validation to `CreateTag`

## Suggested next action

1. **Add the missing three tag tools** (`clockify_get_tag`, `clockify_update_tag`, `clockify_delete_tag`) to the registry and implement handlers in `internal/tools/tags.go`. Follow the patterns used by `clockify_get_project` and `clockify_delete_entry` (the latter pre-fetches before deleting for audit logging).
2. **Add query parameter passthrough** to `ListTags` — extend the handler to accept and forward `name`, `archived`, `sort-column`, `sort-order`, `strict-name-search`, and `excluded-ids` when provided by the caller. Update the tool schema accordingly.
3. **Add negative tests** for `CreateTag` (empty name, 101-char name) and `ListTags` (with filter params).

## False positives / uncertainty

- The `archived` field ignored by the API on POST /tags (always returns `false` on create) is expected upstream behavior, not a bug.
- The "Tag doesn't belong to Workspace" error message for nonexistent IDs (GET/DELETE) returns HTTP 400 rather than 404. This is the Clockify API's chosen behavior, not an MCP issue.
- `clockify_resolve_name` with `entity_type: "tag"` works and can serve as a partial workaround for the missing `get_tag` tool. However, it only resolves by exact name match — it cannot fetch a tag by ID alone.

## Final recommendation

**PASS WITH CONCERNS.** The tag tools that exist (`list_tags`, `create_tag`) work correctly against the live API. Dry-run mode functions properly. However, the MCP server is missing three of the five standard CRUD operations for tags (get, update, delete), and the list operation doesn't passthrough API-supported filtering/sorting parameters. These are gaps that limit the usefulness of the tag tool surface for AI agents. The missing tools should be implemented to achieve parity with other domains (e.g., projects, clients, entries).
