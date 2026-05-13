# QA Agent 50 - openapi-generated-code-consistency

Status: COMPLETE
Started UTC: 2026-05-10T20:31:22Z
Completed UTC: 2026-05-10T22:00:00Z
Worktree: /Users/15x/Downloads/go-clockify-qa-swarm/worktrees/agent-50

## Verdict
**PASS WITH CONCERNS**

## What I checked

This audit assessed whether the go-clockify MCP server's reflection-based schema generation (`internal/tools/schemagen.go`) and tool input/output schemas are consistent with the actual Clockify API, using the clockify-api-probe-lab findings as ground truth.

Coverage:
1. Cross-referenced all 23 probe-lab findings against current implementation
2. Verified all 9 probed domains: invoices, expenses, webhooks, shared-reports, scheduling, time-off, holidays, project-memberships, custom-fields
3. Ran full build/test suite (`make check`, `make catalog-drift`, `doctor --strict`)
4. Ran live API probes against the sacrificial workspace for 8 endpoints
5. Examined schema generation code, model structs, tool input/output schemas
6. Verified enum values, parameter names, body field mappings, host routing, HTTP methods

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID (65b382b606de527a7ee2b60e) |
| `docs/official-api-notes.md` | Per-domain API documentation notes |
| `findings/SUMMARY.md` | 23 recommended changes in priority order |
| `findings/custom-fields.md` | Custom field type enum discoveries |
| `findings/expenses.md` | Multipart expense create/update shapes |
| `findings/holidays.md` | Holiday datePeriod vs flat date |
| `findings/invoices.md` | Invoice envelope and 405 discoveries |
| `findings/project-memberships.md` | PATCH vs PUT, REPLACE semantics |
| `findings/scheduling.md` | Missing /all suffix, required params |
| `findings/shared-reports.md` | Reports host, body keys, export path |
| `findings/time-off.md` | POST-only list, search body |
| `findings/webhooks.md` | Envelope shape, static events enum |
| `fixtures/` (all JSON fixtures) | Redacted response shapes per domain |

All secrets are redacted as `****REDACTED****` throughout this report.

## Commands run

```sh
# Build and test (redacted secrets)
make check                                              # PASS
make gen-tool-catalog                                   # OK
make catalog-drift                                      # OK (no drift)
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=****REDACTED**** \
  go run ./cmd/clockify-mcp -- doctor --profile=local-stdio --strict   # PASS
go test -race -count=1 -timeout 120s ./...              # ALL 29 packages PASS
go test -race -run TestSchemaFor -v ./internal/tools/   # ALL PASS
go test -race -run TestTightenInputSchema -v ./internal/tools/  # ALL PASS

# Live API probes (all via curl, API key redacted)
GET /api/v1/workspaces/{ws}/custom-fields?page-size=5             # 200
GET /api/v1/workspaces/{ws}/expenses?page-size=2                  # 200
GET /api/v1/workspaces/{ws}/expenses/categories                   # 200
GET /api/v1/workspaces/{ws}/invoices?page-size=2                  # 200
GET /api/v1/workspaces/{ws}/user/{uid}/time-entries?page-size=2   # 200
GET /api/v1/workspaces/{ws}/holidays                              # 200
GET /api/v1/workspaces/{ws}/webhooks?page-size=2                  # 200
GET reports.api.clockify.me/v1/workspaces/{ws}/shared-reports?pageSize=2  # 200
GET /api/v1/user                                                   # 200
GET /api/v1/workspaces/{ws}/projects?page-size=1                   # 200
GET /api/v1/workspaces/{ws}/tags?page-size=2                       # 200
```

## Live API probes run

### Probe 1: Custom field type enum
```
GET /api/v1/workspaces/{ws}/custom-fields?page-size=5
Result: type values observed: TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK
Status: MATCH — code enum at tier2_custom_fields.go:72 uses correct 6 values
```

### Probe 2: Expense double-nested envelope
```
GET /api/v1/workspaces/{ws}/expenses?page-size=2
Result: {expenses: {expenses: [...], count: 2829}, dailyTotals: [...], weeklyTotals: [...]}
Status: MATCH — code at tier2_expenses.go:187-193 uses correct double-nested struct
```

### Probe 3: Expense categories envelope
```
GET /api/v1/workspaces/{ws}/expenses/categories
Result: {count: N, categories: [...]}
Status: MATCH — code at tier2_expenses.go:514-517 uses correct {count, categories} struct
```

### Probe 4: Invoice envelope
```
GET /api/v1/workspaces/{ws}/invoices?page-size=2
Result: {total: 113, invoices: [...]}
Status: MATCH — code at tier2_invoices.go:223-226 uses correct {total, invoices} struct
```

### Probe 5: Holiday shape
```
GET /api/v1/workspaces/{ws}/holidays
Result: datePeriod: {startDate, endDate}, occursAnnually (NOT recurring)
Status: MATCH — code at tier2_groups_holidays.go:360-366 uses datePeriod + occursAnnually
```

### Probe 6: Webhook envelope
```
GET /api/v1/workspaces/{ws}/webhooks?page-size=2
Result: {workspaceWebhookCount: 7, webhooks: [...]}
Status: MATCH — code at tier2_webhooks.go:420-423 uses correct struct
```

### Probe 7: Shared reports (reports host)
```
GET reports.api.clockify.me/v1/workspaces/{ws}/shared-reports?pageSize=2
Result: {count: 74, reports: [...]}, pageSize (camelCase) works, page-size (hyphenated) ignored
Status: MATCH — client.go:169-174 routes to reports.api.clockify.me/v1, tier2_shared_reports.go:166 uses pageSize
```

### Probe 8: Time entries require user path
```
GET /api/v1/workspaces/{ws}/time-entries → 405 "Request method 'GET' is not supported"
GET /api/v1/workspaces/{ws}/user/{uid}/time-entries → 200
Status: MATCH — entries.go:539 uses correct /user/{userId}/time-entries path
```

## Findings

### Finding 1: All 23 probe-lab findings are resolved (PASS)
Every recommended change from `clockify-api-probe-lab/findings/SUMMARY.md` has been applied. Cross-reference:

| # | Domain | Issue | Fixed in | Status |
|---|--------|-------|----------|--------|
| 1 | invoices | listInvoiceItems 405 → delegate to getInvoice | tier2_invoices.go:476-508 | FIXED |
| 2 | expenses | createExpense multipart/form-data | tier2_expenses.go:266-292 | FIXED |
| 3 | shared-reports | reports.api.clockify.me/v1 host | client.go:169-174 | FIXED |
| 4 | scheduling | /all suffix + required start/end | tier2_scheduling.go:228,40-41 | FIXED |
| 5 | time-off | listTimeOffRequests POST | tier2_time_off.go:262 | FIXED |
| 6 | memberships | PATCH + extract memberships from project | tier2_project_admin.go:299,304-309 | FIXED |
| 7 | invoices | invoiceReport POST /invoices/info | tier2_invoices.go:666-686 | FIXED |
| 8 | holidays | datePeriod + occursAnnually | tier2_groups_holidays.go:360-366 | FIXED |
| 9 | invoices | {total, invoices} envelope | tier2_invoices.go:223-226 | FIXED |
| 10 | invoices | statuses (plural) param | tier2_invoices.go:233 | FIXED |
| 11 | expenses | double-nested expenses envelope | tier2_expenses.go:187-193 | FIXED |
| 12 | expenses | {count, categories} envelope | tier2_expenses.go:514-517 | FIXED |
| 13 | expenses | updateExpense multipart + changeFields | tier2_expenses.go:460,310-386 | FIXED |
| 14 | webhooks | {workspaceWebhookCount, webhooks} envelope | tier2_webhooks.go:420-423 | FIXED |
| 15 | webhooks | listWebhookEvents static 52-value enum | tier2_webhooks.go:663-730 | FIXED |
| 16 | shared-reports | {reports, count} envelope | tier2_shared_reports.go:173-176 | FIXED |
| 17 | shared-reports | pageSize (camelCase) | tier2_shared_reports.go:166 | FIXED |
| 18 | scheduling | project totals POST to correct path | tier2_scheduling.go:541 | FIXED |
| 19 | custom-fields | TXT/DROPDOWN_SINGLE enums (not TEXT/DROPDOWN) | tier2_custom_fields.go:72 | FIXED |
| 24 | shared-reports | create: reportType→type, filters→filter | tier2_shared_reports.go:229-237 | FIXED |
| 25 | shared-reports | update: ws-prefixed PUT, merge semantics | tier2_shared_reports.go:274-279 | FIXED |
| 26 | shared-reports | delete: ws-prefixed DELETE, bare-id dry run | tier2_shared_reports.go:294-316 | FIXED |
| 27 | shared-reports | export: bare path, binary-aware envelope | tier2_shared_reports.go:386-419 | FIXED |

### Finding 2: `any`-typed fields in Go models lose schema detail (P2)
`schemagen.go` generates JSON Schema from Go structs via reflection. The `any` type maps to `{}` (no constraint). Eight fields across models use `any`:

- `Workspace`: CostRate, Currencies, HourlyRate, Memberships, Subdomain, WorkspaceSettings
- `User`: CustomFields, Memberships, Settings
- `Project`: BudgetEstimate, CostRate, Estimate, EstimateReset, HourlyRate, Memberships, TimeEstimate
- `TimeEntry`: CostRate, CustomFieldValues, HourlyRate
- `ClientEntity`: CCEemails
- `Task`: CostRate, HourlyRate

Live API probes confirm these fields have real structure (e.g., `HourlyRate: {amount: 0, currency: "EUR"}`), but the generated schemas don't capture it. Only MCP clients performing strict output-schema validation are affected; the Go server itself decodes these via `encoding/json` without issue.

### Finding 3: `additionalProperties: false` on generated schemas (P3)
`schemagen.go:108` and `schema_tighten_test.go:13-47` apply `additionalProperties: false` to all object schemas that don't explicitly set one. If the Clockify API adds a new field, MCP clients doing strict output-schema validation will reject the response. The Go server is unaffected (`encoding/json` ignores unknown fields). This is a defensive choice, not a bug.

### Finding 4: Tool catalog and tests are in sync (PASS)
- `make catalog-drift` passes with zero diff
- 40 Tier 1 + 88 Tier 2 tools match the registry
- All 29 packages pass with `-race`
- Schema generation tests cover all primitive types, structs, pointers, slices, maps, interfaces, and time.Time

## Fixes made

No fixes were needed. All 23 probe-lab findings were already resolved in the current codebase.

## Reproduction steps for each issue

N/A — no new issues requiring reproduction were found.

## Cleanup performed

No test resources were created during this audit (read-only probes only).

## Leftover test resources

None.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| Finding 2 — `any` fields lose schema detail | P2 | Informational; client-side schema validation gap. No functional impact on server. |
| Finding 3 — `additionalProperties: false` | P3 | Low risk; only affects strict client-side validation. Server is unaffected. |

## Files changed

No files were changed.

## Suggested next action

1. **(P2) Consider typed sub-structs for `any` fields**: Replace `any` with typed structs for `HourlyRate`, `CostRate`, `CustomFieldValues`, `Memberships` in models.go. This would let `schemaFor[T]()` generate complete schemas for nested API structures.

2. **(P3) Review `additionalProperties: false` posture**: Consider whether allowing additional properties on output schemas would increase resilience to upstream API additions.

3. **(Ongoing) Periodic probe-lab re-run**: Probe-lab findings were generated 2026-05-02/03. Re-running a subset of probes would catch API drift before it affects users.

## False positives / uncertainty

- The `any`-typed fields may be intentionally loose to avoid schema breakage when the Clockify API evolves. Typing them would improve schema quality but increase maintenance burden.
- The `additionalProperties: false` stance is a deliberate safety choice (fail closed). Reversing it would also be a reasonable posture for resilience.
- Tool input schemas use MCP-friendly snake_case (`start_date`, `page_size`, `user_ids`) while mapping to API-specific conventions (`page-size`, `pageSize`). This translation is correct and well-documented.

## Final recommendation

**PASS** — the go-clockify MCP server's schemas are consistent with the live Clockify API. All 23 probe-lab discrepancies have been resolved. The reflection-based schema generator correctly models the API surface, with the acknowledged limitation that `any`-typed fields produce open schemas. The server starts correctly with credentials, passes all tests, and handles the sacrificial workspace correctly. The two concerns noted (P2: `any` fields lose typed detail, P3: strict `additionalProperties:false`) are design tradeoffs, not bugs.
