# QA Agent 02 - mcp-tool-schema-quality

## Verdict
**PASS WITH CONCERNS** (P2 — no P0/P1 issues found)

## What I checked

- All 128 MCP tool input schemas (40 Tier-1 + 88 Tier-2) for JSON Schema validity
- `additionalProperties: false` enforcement on every object level across all tools
- Supported JSON Schema keyword subset (no `$ref`, `oneOf`, `not`, `if/then/else`, etc.)
- Schema tightening: flexible datetime detection, color pattern enforcement, pagination bounds, RFC3339 format injection
- Schema vs handler agreement: flexible-time fields correctly exempted from `format:date-time` to allow natural-language inputs like "now" and "today 9:00"
- Output schemas: every tool has one, `action.const` matches tool name
- Known probe-lab bugs (holiday `datePeriod`, custom-field `TEXT` vs `TXT`, `recurring` vs `occursAnnually`) already fixed upstream
- MCP server startup and initialization via stdio transport
- Live MCP tool calls against the probe workspace: `clockify_whoami`, `clockify_list_projects`, `clockify_policy_info`
- Direct Clockify API calls: `GET /user`, `GET /workspaces/{ws}/projects`, `GET /workspaces/{ws}/user/{userId}/time-entries`

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key + workspace ID (redacted) |
| `TIMEENTRYDOC.md` | Time entry API reference schema |
| `PROJECTSDOC.md` | Project API reference schema |
| `HOLIDAYSDOC.md` | Holiday API reference schema |
| `docs/official-api-notes.md` | Back-filled per-domain probe findings |
| `findings/SUMMARY.md` | 27 recommended go-clockify changes |

## Commands run

```
# Run all schema-related tests (all PASS)
go test ./internal/tools/ -run "TestGolden|TestAll|TestSchema|TestTighten|TestRegistry|TestTier2Schema" -v -count=1

# Run all tools tests (all PASS, 28.6s)
go test ./internal/tools/ -count=1

# Build MCP binary (SUCCESS)
go build ./cmd/clockify-mcp/

# MCP initialize via stdio (SUCCESS)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | \
  CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE=<REDACTED> ./clockify-mcp

# Live API: current user (SUCCESS)
curl -s -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/user"

# Live API: list projects (SUCCESS)
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects?page-size=2"

# Live API: time entries by user (SUCCESS)
curl -s -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<REDACTED>/time-entries?page-size=2"

# Live MCP: whoami (SUCCESS)
# Live MCP: list_projects (SUCCESS)
# Live MCP: policy_info (SUCCESS)
# Live MCP: tools/list (SUCCESS — 40 Tier-1 tools with full schemas)
```

## Live API probes run

1. **Direct API: GET /user** — returned user (<EMAIL>), confirmed active workspace and full settings object
2. **Direct API: GET /workspaces/{ws}/projects** — returned paginated array with proper `hourlyRate`, `costRate`, `memberships`, `estimate`, `timeEstimate` nested objects
3. **Direct API: GET /workspaces/{ws}/user/{uid}/time-entries** — returned time entries with `timeInterval: {start, end, duration}`, `customFieldValues[]`, `hourlyRate`, `costRate` — confirmed nested object shapes match API docs
4. **MCP: clockify_whoami** — returned `{ok, action, data: {user, workspaceId}}` envelope; user object contained `id, name, email, activeWorkspace, settings, memberships, profilePicture`
5. **MCP: clockify_list_projects** — returned `{ok, action, data: [...], meta: {count, page, pageSize, workspaceId}}`; each project contained `id, name, color, archived, billable, duration, estimate, hourlyRate, memberships, timeEstimate, workspaceId`
6. **MCP: clockify_policy_info** — returned `mode: "standard"`, 40 visible Tier-1 tools, 0 hidden/denied, lists of safe_core_writes and time_tracking_safe_writes
7. **MCP: tools/list** — returned all 40 Tier-1 tools with full input schemas, output schemas, and annotations

## Findings

### F1: Schema quality is strong — P3 (observation)

All 128 tool schemas are well-formed JSON Schema objects. The enforcement pipeline ensures:
- Every object-level schema has `additionalProperties: false` (confirmed by `TestRegistrySchemasAllHaveAdditionalPropertiesFalse` and `TestTier2SchemasAllHaveAdditionalPropertiesFalse`)
- Only supported JSON Schema keywords are used: `type, required, additionalProperties, properties, items, minItems, minimum, maximum, minLength, maxLength, pattern, format, enum, anyOf, description, title`
- No forbidden keywords (`$ref, oneOf, not, if/then/else, const, exclusiveMinimum/Maximum, multipleOf, propertyNames, patternProperties, dependentSchemas, allOf`) appear in any schema

### F2: Flexible datetime fields correctly handled — P3 (confirmation)

Fields documented as accepting natural-language datetime (e.g., "now", "today 9:00") correctly lack `format:date-time` constraints. Verified for: `clockify_add_entry.start`, `clockify_list_entries.start`, `clockify_list_entries.end`, `clockify_weekly_summary.week_start`. The `TestRegistrySchemaAcceptsNaturalLanguageDatetime` test passes `start="now"` through the validator successfully.

### F3: Probe-lab bugs already fixed upstream — P3 (confirmation)

The following bugs documented in `docs/official-api-notes.md` are already addressed in the MCP schemas:
- **Holiday datePeriod**: `clockify_create_holiday` schema uses flat `start_date`/`end_date` params; handler correctly nests them into `datePeriod: {startDate, endDate}` (`tier2_groups_holidays.go:358-363`)
- **Holiday occursAnnually**: schema uses `occurs_annually` → handler maps to `occursAnnually` (`tier2_groups_holidays.go:365-367`)
- **Custom field type enum**: schema uses `TXT` (not `TEXT`) and includes all 6 valid values: `TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK` (`tier2_custom_fields.go:72`)
- **No `recurring` field**: confirmed absent from holiday schema

### F4: MCP schema intentionally narrower than full API — P3 (design note, not a bug)

The MCP tool schemas are deliberately simpler than the full Clockify API. Examples:
- `clockify_list_projects` exposes only `page, page_size` (API supports `name, archived, billable, clients[], sort-column, sort-order, hydrated, users[]`, etc.)
- `clockify_create_project` exposes `name, client, color, billable, is_public` (API supports `hourlyRate, costRate, estimate, note, memberships, tasks`)
- `clockify_start_timer` exposes `project_id, project, description` (API supports `taskId, tagIds, billable`)

This is a valid design choice for AI agent safety — the MCP resolves names to IDs and applies policy controls. No action needed.

### F5: Page size bounds enforcement — P3 (confirmation)

Pagination schemas correctly enforce `page.minimum: 1` and `page_size.minimum: 1, page_size.maximum: 200` via the `tightenInputSchema` walker. Confirmed by `TestTightenInputSchemaPaginationBounds`.

### F6: Color pattern enforcement — P3 (confirmation)

Fields with description "Hex color code" automatically receive `pattern: "^#[0-9a-fA-F]{6}$"` via schema tightening. Confirmed by `TestTightenInputSchemaColorPattern`.

### F7: No tool uses unsupported keywords — P3 (confirmed)

`TestRegistrySchemasUseOnlySupportedValidatorKeywords` walks all 128 tools and confirms zero unsupported keyword violations. The test also guards against empty registry (vacuous pass prevention).

### F8: All schemas accept happy-path arguments — P3 (confirmed)

`TestRegistrySchemasAcceptHappyPathArgs` synthesizes valid arguments for every tool schema and validates them through `jsonschema.Validate`. All tools pass.

### F9: Minor schema detail — `clockify_update_entry` missing `tag_ids`, `task_id` — P3

The MCP schema for `clockify_update_entry` excludes `tag_ids` and `task_id` parameters that the Clockify API accepts. The handler does a fetch-then-update merge, so these fields would require additional merge logic.

### F10: Time entries list endpoint discovery — P3 (documentation note)

The direct `GET /workspaces/{ws}/time-entries` endpoint returned 405 "Request method 'GET' is not supported". The MCP correctly uses `GET /workspaces/{ws}/user/{userId}/time-entries` instead, as confirmed by the `ListEntries` handler code in `entries.go`.

## Fixes made

No code changes were needed. The schema quality in this repository is already at a high standard. All previous probe-lab findings relevant to this area were already incorporated.

## Reproduction steps for each issue

No reproducible bugs found. All tests pass, MCP server starts successfully, and live tool calls return correct responses.

## Cleanup performed

No test resources were created during this QA run (read-only probes only).

## Leftover test resources

None.

## Severity

All findings are P3 (low severity / observations / design notes). No P0, P1, or P2 issues identified.

## Files changed

None.

## Suggested next action

1. **P3 - Expand `clockify_update_entry`**: Consider adding `tag_ids` and `task_id` to the input schema for fuller API coverage (fetch-then-merge handler would need to handle these fields too)
2. **P3 - Expand `clockify_start_timer`**: Consider adding `task_id`, `tag_ids`, `billable` support
3. **P3 - Documentation**: The intentional schema narrowing (F4) could be documented in `docs/tool-catalog.md` to explain the design rationale for AI-facing simplification
4. **P3 - Future**: If the project wants richer filtering on list endpoints, the pagination schema helper could accept an optional filter-properties map

## False positives / uncertainty

- **F4 (intentional narrowing)**: The gap between MCP schema and full API surface is intentional. Verified by reading handler code which explicitly resolves names to IDs and adds safety layers. Not a bug.
- **F10 (405 on workspace-level time entries)**: This is the expected API behavior. The MCP correctly uses the user-scoped endpoint. Not a bug.
- All schema tests passed on the first run with no flakes.

## Final recommendation

**The MCP tool schema quality is production-ready for local/internal/community/self-hosted use.** The schema infrastructure is thorough — every tool has a validated JSON Schema with closed objects (`additionalProperties: false`), only supported keywords, correct format constraints, and output schema envelopes. The known bugs from the API probe lab (holiday date structure, custom-field type enum, occursAnnually naming) have been fixed upstream. The MCP server starts correctly and responds to tool calls with properly structured results.

The one gap is intentional simplification for AI agent safety (F4), which is a design choice rather than a defect. If full API surface parity is needed for self-hosted users who expect raw Clockify API access, the schema narrowing should be documented explicitly.
