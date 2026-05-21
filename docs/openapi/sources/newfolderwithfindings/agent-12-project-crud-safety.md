# QA Agent 12 - project-crud-safety

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Repo structure inspection** — Reviewed `internal/tools/projects.go`, `internal/tools/registry.go`, `internal/tools/tier2_project_admin.go`, `internal/clockify/models.go`, `internal/clockify/client.go`, `internal/paths/paths.go`, `internal/resolve/resolve.go`, and related test files.

2. **Tool catalog audit** — Mapped every project-related Clockify API endpoint against the MCP tool registry to identify coverage gaps.

3. **Project model audit** — Verified the `clockify.Project` struct fields against the actual API response shapes documented in `PROJECTSDOC.md` and confirmed via live probes.

4. **Build and unit tests** — `go build ./...` passes cleanly. `go test ./internal/tools/... -run "Project" -count=1` passes (2.6s).

5. **Doctor command** — `go run ./cmd/clockify-mcp doctor` runs successfully with live credentials, reports `Load() result: OK transport=stdio`.

6. **Live API probes** — Tested full project CRUD lifecycle against the Clockify API probe workspace: create, read, update (PUT), archive/unarchive, delete, error cases, and filter parameters.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, confirmation token
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/PROJECTSDOC.md` — Full Clockify Projects API reference
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/probes/lib/common.sh` — Probe library (referenced conventions; probes run via direct curl)

## Commands run

```bash
# Build
go build ./...

# Unit tests
go test ./internal/tools/... -run "Project" -count=1 -timeout 60s

# Doctor command
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  go run ./cmd/clockify-mcp doctor

# Live API: list projects
curl -s -X GET \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects?page-size=5" \
  -H "X-Api-Key: <REDACTED>"

# Live API: create project
curl -s -X POST \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects" \
  -H "X-Api-Key: <REDACTED>" \
  -H "Content-Type: application/json" \
  -d '{"name":"qa-agent-12-<ts>-test-create","billable":true,"isPublic":false,"color":"#FF5733"}'

# Live API: get project by ID
curl -s -X GET \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/<id>" \
  -H "X-Api-Key: <REDACTED>"

# Live API: update project (PUT)
curl -s -X PUT \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/<id>" \
  -H "X-Api-Key: <REDACTED>" \
  -H "Content-Type: application/json" \
  -d '{"name":"qa-agent-12-<ts>-test-updated","billable":false,"isPublic":true,"color":"#00FF00","note":"Updated via QA probe"}'

# Live API: archive project
curl -s -X PUT \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/<id>" \
  -H "X-Api-Key: <REDACTED>" \
  -H "Content-Type: application/json" \
  -d '{"archived":true}'

# Live API: delete project (must archive first)
curl -s -X DELETE \
  "https://api.clockify.me/api/v1/workspaces/${CLOCKIFY_WORKSPACE_ID}/projects/<id>" \
  -H "X-Api-Key: <REDACTED>"

# Live API: error cases
curl -s -X POST "..." -H "X-Api-Key: <REDACTED>" -d '{"billable":true}'  # missing name -> 400
curl -s -X GET "..." -H "X-Api-Key: <REDACTED>"  # invalid ID format -> 400
curl -s -X DELETE "..." -H "X-Api-Key: <REDACTED>"  # active project -> 400 "Cannot delete an active project"
```

## Live API probes run

| # | Probe | Method | Path | Result |
|---|-------|--------|------|--------|
| 1 | List projects | GET | `/workspaces/{ws}/projects?page-size=5` | 200, 5 items with full shape (id, name, archived, billable, public, clientId, clientName) |
| 2 | Create project | POST | `/workspaces/{ws}/projects` | 201, all fields populated correctly |
| 3 | Get project by ID | GET | `/workspaces/{ws}/projects/{id}` | 200, round-trips match creation shape |
| 4 | Update project (PUT) | PUT | `/workspaces/{ws}/projects/{id}` | 200, name/color/billable/isPublic/note all updated |
| 5 | Archive project | PUT | `...` + `{"archived":true}` | 200, `archived` flips to true |
| 6 | Unarchive project | PUT | `...` + `{"archived":false}` | 200, `archived` flips to false |
| 7 | Delete (active) | DELETE | `/workspaces/{ws}/projects/{id}` | 400, `"Cannot delete an active project"` — must archive first |
| 8 | Delete (archived) | DELETE | `/workspaces/{ws}/projects/{id}` | 200, full project object returned in response body |
| 9 | Verify deletion | GET | `/workspaces/{ws}/projects/{id}` | 400, `"Project doesn't belong to Workspace"` |
| 10 | Create without name | POST | `...` + `{"billable":true}` | 400, rejected by API |
| 11 | Invalid project ID | GET | `.../invalid-id-format` | 400, `"Project doesn't belong to Workspace"` |
| 12 | Nonexistent project | GET | `.../<REDACTED_ID>` | 400, `"Project doesn't belong to Workspace"` |
| 13 | List with archived filter | GET | `...?archived=true&page-size=3` | 200, only archived projects returned |
| 14 | List with is-template filter | GET | `...?is-template=true&page-size=5` | 200, only template projects returned |
| 15 | List with name filter | GET | `...?name=qa-agent-12&page-size=5` | 200, 0 items (expected — none matched after cleanup) |

## Findings

### F1: Missing generic project-update tool (P2)

The Clockify API supports `PUT /workspaces/{wsId}/projects/{id}` for updating project name, color, billable, isPublic, note, and other fields. Live probe #4 confirmed this works. However, the MCP server has no `clockify_update_project` tool.

Current project mutation coverage:
- `clockify_create_project` — Tier 1 (always visible): OK
- `clockify_update_project_estimate` — Tier 2 group `project_admin` (estimate only): partial
- `clockify_archive_projects` — Tier 2 group `project_admin` (archive only): partial
- `clockify_set_project_memberships` — Tier 2 group `project_admin` (memberships only): OK

Missing: a general `clockify_update_project` that can update name, color, billable, isPublic, note, clientId, etc. Users who want to rename a project or change its color today have no MCP tool path — they must use the Clockify web UI or direct API.

### F2: Missing project-delete tool (P2)

The Clockify API supports `DELETE /workspaces/{wsId}/projects/{id}` for hard-deleting archived projects. Live probe #8 confirmed this works (after archiving first). The MCP server has no `clockify_delete_project` tool.

Note: the API enforces archive-before-delete safety (probe #7 returned 400 `"Cannot delete an active project"`). Any implementation should either enforce the same two-step workflow or automate the archive step internally.

### F3: `resolveByNameOrID` path construction bypasses path validation (P3)

In `internal/resolve/resolve.go:65-67`, `ResolveProjectID` constructs the path via string concatenation:

```go
func ResolveProjectID(...) (string, error) {
    return resolveByNameOrID(ctx, client, "/workspaces/"+workspaceID+"/projects", ref, "project")
}
```

Unlike handler code that uses `paths.Workspace()` with `resolve.ValidateID()`, this path bypasses validation. In practice, `workspaceID` comes from `s.ResolveWorkspaceID()` which returns a validated value, so this is not currently exploitable. Still, it is an inconsistency — every other code path that constructs `/workspaces/{wsId}/...` URLs goes through `paths.Workspace()`.

### F4: No paginated filter support for project listing (P3)

The `clockify_list_projects` tool only supports `page` and `page-size` parameters. The Clockify API supports additional filters: `name`, `strict-name-search`, `archived`, `billable`, `clients`, `users`, `is-template`, `sort-column`, `sort-order`, `access`, `hydrated`, and more (see PROJECTSDOC.md lines 12-130). None of these are exposed by the MCP tool.

This means an AI agent cannot ask "list all archived projects" or "find projects named X" through the MCP — it must pull all pages and filter client-side.

### F5: `clockify_list_projects` cannot search by name via MCP (P3)

Related to F4 — the API's `name` query parameter is the primary way to search for projects by name. Both `ResolveProjectID` (internal/resolve) and `clockify_get_project` do exact-name resolution via `strict-name-search=true`, but `clockify_list_projects` does not pipe through a `name` argument. An agent that wants to fuzzy-search projects has no path.

## Fixes made

None. The missing tools (F1, F2) are feature additions, not bugs in existing code. The path validation inconsistency (F3) and missing filter parameters (F4, F5) are quality improvements that should be addressed in a planned PR rather than as a rushed in-session fix.

## Reproduction steps for each issue

### F1 (missing update project tool)
1. Create a project via `clockify_create_project` with `{"name": "test-project"}`
2. Attempt to rename it to "test-project-renamed" via any MCP tool
3. **Expected:** A tool like `clockify_update_project` accepting `name`, `color`, `billable`, `is_public`, `note`
4. **Actual:** No such tool exists; `clockify_update_project_estimate` only handles estimates

### F2 (missing delete project tool)
1. Create and archive a project via `clockify_create_project` then `clockify_archive_projects`
2. Attempt to permanently delete it
3. **Expected:** A tool like `clockify_delete_project` accepting a project ID
4. **Actual:** No delete tool exists; the only path is direct API call

### F3 (resolve path bypasses validation)
1. Review `internal/resolve/resolve.go:65-67` — path is `"/workspaces/" + workspaceID + "/projects"`
2. Review any handler (e.g., `internal/tools/projects.go:18`) — path is built via `paths.Workspace(wsID, "projects")` which calls `resolve.ValidateID()`
3. **Inconsistency:** Same path, two different construction methods, one validated, one not

## Cleanup performed

All test resources created with the `qa-agent-12-` prefix were fully cleaned up:

| Resource | ID | Action | Status |
|----------|----|--------|--------|
| qa-agent-12-1778446861-4a5d42-test-create | <REDACTED_ID> | Archived then deleted | Cleaned |
| qa-agent-12-1778447148-4826f0-test-crud | <REDACTED_ID> | Archived then deleted | Cleaned |

## Leftover test resources

None. All qa-agent-12- prefixed resources were cleaned up.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| F1: Missing update_project tool | P2 | Feature gap; no user-facing workaround in MCP. Users must use web UI or direct API. |
| F2: Missing delete_project tool | P2 | Feature gap; no user-facing workaround in MCP. Archive-before-delete safety must be preserved. |
| F3: Resolve path bypasses validation | P3 | Defense-in-depth inconsistency. Not exploitable because callers pass pre-validated workspace IDs. |
| F4: No filter parameters on list | P3 | Usability gap. Agents must pull all pages and filter client-side. |
| F5: No name search on list | P3 | Usability gap. Resolve-by-name exists but requires exact match; fuzzy listing doesn't. |

## Files changed

None.

## Suggested next action

1. **Add `clockify_update_project`** to the Tier-1 registry. Accept `project_id` (required), plus optional `name`, `color`, `billable`, `is_public`, `note`, `client_id`. Use `PUT /workspaces/{wsId}/projects/{id}`.

2. **Add `clockify_delete_project`** to the Tier-1 registry. Accept `project_id` (required). Use `DELETE /workspaces/{wsId}/projects/{id}`. Consider auto-archiving first to match the API's archive-before-delete safety requirement, or document the two-step workflow.

3. **Extend `clockify_list_projects`** input schema with optional filter parameters: `name`, `archived`, `billable`, `is_template`, `sort_column`, `sort_order`.

4. **Align `ResolveProjectID` path construction** to use `paths.Workspace()` for consistency with every other handler.

## False positives / uncertainty

- The `ClientID` and `ClientName` fields ARE present in the `clockify.Project` struct (models.go:36-37). I initially flagged them as missing but confirmed they exist on re-inspection. No action needed.
- The 400 "Project doesn't belong to Workspace" error for invalid/nonexistent project IDs is the API's actual behavior, not a bug in the MCP server. The MCP server correctly proxies this error.
- The `rawArchiveAndDeleteProject` helper in tests correctly handles the archive-before-delete requirement. This pattern should be preserved in any `clockify_delete_project` implementation.

## Final recommendation

The project CRUD safety posture is **PASS WITH CONCERNS**. The existing tools (`list`, `get`, `create`) work correctly and handle errors safely. The path validation, auth, and resource cleanup patterns are solid. However, the absence of `update_project` and `delete_project` tools means the MCP server cannot fully manage projects — users are forced to use the Clockify web UI or direct API calls for basic rename/delete operations. These are feature gaps (not bugs) that should be addressed in the next development cycle.
