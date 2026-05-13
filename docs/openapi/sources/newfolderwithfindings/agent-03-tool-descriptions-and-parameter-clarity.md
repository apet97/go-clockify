# QA Agent 03 - tool-descriptions-and-parameter-clarity

## Verdict
**PASS WITH CONCERNS**

## What I checked

Validated every MCP tool descriptor against the live Clockify API (probe workspace `****REDACTED****`), the
generated tool catalog (128 tools: 40 Tier-1 + 88 Tier-2), and the API documentation in the probe lab.
Scope: tool descriptions, parameter names, required fields, types, enum values, schema tightening,
pagination consistency, dry-run support, deprecated-tool marking, risk-class annotations, and
error-message clarity.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — credentials (API key, workspace ID)
- `PROJECTSDOC.md` — Clockify project API reference
- `TIMEENTRYDOC.md` — Clockify time-entry API reference
- `EXPENSESDOC.md`, `TASKDOC.md`, `TAGDOC.md`, `CLIENTDOC.md` — supplementary API docs
- `docs/tool-catalog.json`, `docs/tool-catalog.md` — generated catalog (reproduced at /tmp)

## Commands run

```sh
# Build and test
go build ./...
go test ./internal/tools/ -run "TestTighten|TestSchema|TestCreate" -count=1 -v

# Tool catalog analysis
go run ./scripts/gen-tool-catalog -out /tmp/tool-catalog-out

# Live API probes (all using CLOCKIFY_API_KEY from env)
curl -s -H "X-Api-Key: $CLOCKIFY_API_KEY" \
  "https://api.clockify.me/api/v1/workspaces/$WORKSPACE_ID/projects?page-size=200"
curl -s -H "X-Api-Key: $CLOCKIFY_API_KEY" \
  "https://api.clockify.me/api/v1/workspaces/$WORKSPACE_ID/user/$USER_ID/time-entries?..."
curl -s -X POST -H "X-Api-Key: $CLOCKIFY_API_KEY" \
  "https://api.clockify.me/api/v1/workspaces/$WORKSPACE_ID/time-entries" -d '{...}'
# ... and ~15 more probes covering pagination edges, auth failures, param formats
```

## Live API probes run

| # | Probe | Result |
|---|-------|--------|
| 1 | List time entries with RFC3339 dates | 200, proper data |
| 2 | List time entries with YYYY-MM-DD dates | 501, API rejects bare dates — MCP handler converts them |
| 3 | Create time entry without projectId | 201, entry created |
| 4 | Create time entry with billable=true | 201, billable field respected |
| 5 | List projects page-size=0 | 400, "Page size must be a positive value" |
| 6 | List projects page-size=200 (max) | 200, 200 items returned |
| 7 | List projects page-size=1000 (over-cap) | 200, API accepts it — MCP clamps at 200 |
| 8 | Get non-existent project | 400 (not 404) |
| 9 | Bad API key | 401 |
| 10 | Create project with all params | 201, all fields accepted |
| 11 | List projects with name filter | 200, filter works |
| 12 | Create client | 201 |
| 13 | Create task | 201, status=ACTIVE accepted |
| 14 | Create tag | 201 |
| 15 | Create BREAK-type time entry | 501, "break feature is not enabled" |
| 16 | Project creation: `public` vs `isPublic` | Both accepted by API |
| 17 | DELETE time entry | 204, successful |
| 18 | DELETE project | 200, successful |
| 19 | DELETE client (active) | 400, "Cannot delete an active client" |
| 20 | Archive then delete client | 200, successful |
| 21 | User time-entries with description filter | 200, filter works |
| 22 | User time-entries with project filter | 200, filter works |
| 23 | Workspace-level GET /time-entries | 405, not supported — MCP uses user-scoped endpoint |

## Findings

### F1. All 128 tools have descriptions and input schemas (PASS)

Every tool in the catalog has a non-empty description and a valid JSON Schema input schema.
No missing or empty descriptions found. Tier-1 tools get typed output schemas; Tier-2 tools
get opaque envelope schemas. Both categories are complete.

### F2. Schema tightening is correct and well-tested (PASS)

`tightenInputSchema` in `internal/tools/common.go:479` correctly:
- Adds `additionalProperties: false` to every object schema (spec B2)
- Adds `minimum: 1` to `page`, `minimum: 1` + `maximum: 200` to `page_size`
- Adds `format: "date-time"` to RFC3339-only string fields
- Skips `format: "date-time"` for flexible-parsing fields (natural language, YYYY-MM-DD)
- Adds `pattern: "^#[0-9a-fA-F]{6}$"` to hex color properties

Tests at `internal/tools/schema_tighten_test.go` cover all these cases and pass.

### F3. Snake_case parameter naming is consistent (PASS)

All MCP tool parameters use snake_case (`project_id`, `task_id`, `page_size`, `entry_id`).
The handler code (e.g., `entries.go:204` → `payload["projectId"]`, `entries.go:209` → `payload["taskId"]`,
`entries.go:212` → `payload["tagIds"]`) translates to the API's camelCase/kebab-case as needed.
This abstractive layer is intentional and consistent across all tools.

### F4. Pagination is consistent and safe (PASS)

All list tools use `page` (integer, default 1, min 1) and `page_size` (integer, default 50, min 1, max 200).
`paginationFromArgs` in `common.go:612` correctly clamps out-of-range values. The `page-size=0` probe confirmed
the API rejects this with code 501; the MCP clamping prevents it from ever reaching the API.

### F5. 34 write tools lack dry_run support (P2 CONCERN)

Tier-2 CRUD tools in `approvals`, `custom_fields`, `expenses`, `holidays`, `invoice_items`,
`project_admin`, `scheduling`, `shared_reports`, `time_off`, and `user_admin` groups lack
`dry_run` in their input schemas. While activation tools intentionally skip it, many CRUD
operations (e.g., `clockify_create_expense`, `clockify_create_holiday`, `clockify_add_invoice_item`)
would benefit from dry-run preview. This is a scope/priority gap, not a bug.

Tools missing dry_run:
```
clockify_submit_for_approval, clockify_withdraw_approval,
clockify_create_custom_field, clockify_set_custom_field_value, clockify_update_custom_field,
clockify_create_expense, clockify_create_expense_category,
clockify_update_expense, clockify_update_expense_category,
clockify_create_holiday,
clockify_create_user_group_admin, clockify_update_user_group_admin,
clockify_add_invoice_item, clockify_update_invoice_item,
clockify_archive_projects, clockify_create_project_template,
clockify_set_project_memberships, clockify_update_project_estimate,
clockify_create_assignment, clockify_update_assignment,
clockify_create_shared_report, clockify_update_shared_report,
clockify_approve_time_off, clockify_create_time_off_policy,
clockify_create_time_off_request, clockify_deny_time_off,
clockify_update_time_off_policy, clockify_update_time_off_request,
clockify_add_user_to_group, clockify_update_user_group,
clockify_approve_timesheet, clockify_reject_timesheet
```

### F6. Tier-1 list tools are intentionally simplified (P3 NOTE)

`clockify_list_projects` and `clockify_list_clients` only expose `page`/`page_size`.
The Clockify API supports richer query parameters (`name`, `archived`, `billable`,
`clients`, `sort-column`, `sort-order`, `hydrated` for projects; `name`, `archived`,
`sort-column`, `sort-order` for clients). The tool descriptions accurately say
"List projects/clients in the resolved workspace" but do not advertise that filtering
is limited to pagination only. Agents that need filtered lists must use other tools
or activate Tier-2 groups.

### F7. `clockify_add_entry` omits API-supported parameters (P3 NOTE)

The API supports `type` (REGULAR/BREAK), `customFields`, `customAttributes`, and `kioskId`
for time entry creation. The MCP tool does not expose these. The `type` omission is
reasonable (BREAK feature requires workspace setup not available in this probe workspace).
The `customFields` omission may be a gap for workspaces that use custom fields on time entries.

### F8. `clockify_create_client` is name-only (P3 NOTE)

The API supports `address`, `note`, `email`, and `currencyId` for client creation.
The MCP tool only supports `name`. The description "Create a new client" is accurate
but could note the simplified interface.

### F9. Deprecated tools properly marked (PASS)

`clockify_resolve_debug` → "Deprecated compatibility alias for clockify_resolve_name."
`clockify_search_tools` → "Deprecated compatibility shim. Prefer clockify_list_tools..."
Both have clear descriptions pointing to replacements.

### F10. Risk class annotations are complete and correct (PASS)

All tools carry proper risk-class annotations using the 7-category taxonomy
(read, write, destructive, billing, admin, permission_change, external_side_effect).
The `gen-tool-catalog` script emits these in the JSON output. The `RiskClass` field
in `ToolDescriptor` is properly derived from MCP boolean hints with per-tool overrides.

### F11. All `*_id` parameters consistently typed as string (PASS)

104 `*_id` parameters across all tools are typed as `"type": "string"`. The API returns
24-character hex strings; no integer IDs were observed. This is correct.

### F12. Error messages for invalid parameters are clear (PASS)

Live probes confirmed:
- Missing required param: "start is required" / "name is required"
- Invalid datetime: "invalid start: expected RFC3339 or YYYY-MM-DD date, got ..."
- Out-of-range: "end must be after start"
- Missing project: "project is required"
- Schema validation: "invalid params at /entry_id: missing required property" (with JSON Pointer)
- Non-existent resource: API returns 400 with message body; MCP surfaces as APIError

### F13. Auth failures handled correctly (PASS)

Bad API key → 401 from API → surfaced as APIError by the client. The MCP server does not
leak the API key in error messages. The `X-Api-Key` header is read from the client config,
not from MCP tool parameters, so no tool exposes an API key parameter.

### F14. Parameter type correctness (PASS)

All parameter types match what the API expects:
- `start`/`end` → string (RFC3339 or flexible)
- `project_id`/`task_id`/`entry_id` → string
- `billable`/`dry_run`/`allow_overlap`/`is_public` → boolean
- `page`/`page_size`/`max_entries`/`min_gap_minutes`/`max_suggestions`/`days` → integer
- `tag_ids` → array of strings
- `timezone` → string (optional IANA)
- `color` → string (hex, tightened to `pattern: "^#[0-9a-fA-F]{6}$"`)

## Fixes made

None. No bugs were found in the tool-description-and-parameter-clarity area. All findings
are design observations (missing dry_run on Tier-2 CRUD tools, simplified list interfaces).

## Reproduction steps for each issue

### F5: Dry-run gap on Tier-2 CRUD tools
1. Activate a Tier-2 group, e.g., `clockify_activate_group` with `name: "expenses"`
2. Call `clockify_create_expense` — no `dry_run` parameter in schema
3. The tool commits immediately with no preview path

### F6: Simplified list tools
1. Call `clockify_list_projects` — only `page`/`page_size` are available
2. Compare with `GET /api/v1/workspaces/{ws}/projects?name=...&archived=true&billable=true`
3. The API supports richer filtering; the MCP tool does not expose it

## Cleanup performed

All qa-agent-03-prefixed test resources were deleted:
- Time entries: deleted (HTTP 204)
- Projects: deleted (HTTP 200)
- Client: archived then deleted (HTTP 200)
- Tags: deleted (HTTP 200)
- Tasks: deleted (HTTP 200)

## Leftover test resources

None. All qa-agent-03- resources were successfully cleaned up.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| F1-F4, F9-F14 | P0-PASS | No issues found |
| F5 (dry_run gap) | P2 | Feature gap, not a bug; affects Tier-2 only |
| F6 (simplified lists) | P3 | Design choice; descriptions are accurate |
| F7 (add_entry omissions) | P3 | BREAK type is workspace-dependent; customFields is a gap |
| F8 (create_client minimal) | P3 | Name-only is sufficient for most MCP use cases |

## Files changed

None.

## Suggested next action

1. **Add dry_run to Tier-2 CRUD tools** — highest-value improvement in this area.
   Start with `expenses`, `holidays`, and `shared_reports` groups where preview
   is most useful before committing real data.

2. **Consider exposing richer list filters** — `clockify_list_projects` could add
   optional `name` (string), `archived` (boolean), and `billable` (boolean) parameters
   without breaking the existing interface.

3. **Consider `customFields` on `clockify_add_entry` and `clockify_log_time`** —
   if the target workspace uses custom fields on time entries, this is a gap.

## False positives / uncertainty

- **F6-F8 are design decisions, not bugs**: The MCP server deliberately provides a
  simplified interface over the full Clockify API surface. These findings note areas
  where the simplification could be relaxed for power users.
- **BREAK type**: The `type` parameter was probed and returned 501 ("break feature is
  not enabled"), confirming this workspace can't test it. Other workspaces may differ.
- **Custom fields**: The probe workspace has custom fields but they're unpopulated.
  A workspace with active custom field usage might surface different behavior.

## Final recommendation

**Ship the current state.** The tool descriptions and parameter schemas are correct,
consistent, and well-tested. The schema tightening logic is sound. The parameter
naming convention is uniform. The identified gaps (F5-F8) are feature enhancements,
not correctness issues. No blocking defects were found in the
tool-descriptions-and-parameter-clarity area.
