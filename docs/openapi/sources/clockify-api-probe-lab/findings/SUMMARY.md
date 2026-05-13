# clockify-api-probe-lab — finding summary

Generated 2026-05-03 (rev 4 — shared-reports write/export probes land).
Source documents: `findings/<domain>.md` for probe evidence, `<DOMAIN>DOC.md` for docs.

**Rev 4 changes vs rev 3:**
- New changes #24–#27: shared-reports write/export tools (`createSharedReport`, `updateSharedReport`, `deleteSharedReport`, `exportSharedReport`) — body field renames (`reportType`→`type`, `filters`→`filter`), required nested filter fields, ws-prefixed PUT/DELETE confirmed, `/export` segment proved bogus, binary-aware export envelope required.
- Open question #5 (`getSharedReport` path with no workspace segment) — **resolved**: handler is correct; ws-prefixed per-id GET returns 405 (path exists, GET disallowed).
- Open question #6 (`exportType` query on bare-id GET) — **resolved** with fixture evidence per format (PDF/CSV/XLSX content types confirmed live).
- 3 new low-priority open questions added (cross-type validation on update, non-SUMMARY filter sub-object requirements, full-blob PUT roundtrip).

**Rev 3 changes vs rev 2:**
- Change #6 updated: PATCH response shape + REPLACE semantics both confirmed live
- Change #18 merged into #6 as sub-bullet (not a separate change — same function)
- New change #20: `createHoliday` must include `users`/`userGroups` — user assignment is required (400 without it)
- New change #21: `createExpense` `file` field is optional despite docs saying required
- New change #22: scheduling project totals path confirmed (`/scheduling/assignments/projects/totals`)
- New change #23: shared-reports `pageSize` param name confirmed (camelCase, not hyphenated)
- Open question #8 (PATCH replace vs upsert) **resolved** — REPLACE confirmed live
- Open question #2 (empty POST body) resolved for time-off
- 4 new open questions added (holidays user assignment required, scheduling totals path, DELETE project archive-first)
- "Next Opus implementation batch" section added

---

## Recommended go-clockify changes (in priority order)

Priority key: **BLOCK** = outright failure (4xx), **SHAPE** = silent data loss / wrong
parse, **ENUM** = wrong constant / param name.

| # | Domain | File | Function | Change | Category | Source |
|---|---|---|---|---|---|---|
| 1 | invoices | `internal/tools/tier2_invoices.go` | `listInvoiceItems` | Drop the separate `GET /invoices/{id}/items` call (returns 405). Instead call `GET /workspaces/{ws}/invoices/{id}` and return the embedded `items` array. | BLOCK | findings/invoices.md |
| 2 | expenses | `internal/tools/tier2_expenses.go` | `createExpense` | Switch from `application/json` to `multipart/form-data`; required fields are `userId`, `amount`, `date` (`yyyy-MM-ddThh:mm:ssZ`), `categoryId`. `file` is documented as required but API accepts absence — `fileId` comes back as `""`. `projectId` also accepted as absent. | BLOCK | findings/expenses.md |
| 3 | shared-reports | `internal/tools/tier2_shared_reports.go` | base-URL constant | Replace `https://api.clockify.me/api/v1` with `https://reports.api.clockify.me/v1` — both host AND path prefix must change. `/api/v1` on the reports host is also 404. | BLOCK | findings/shared-reports.md |
| 4 | scheduling | `internal/tools/tier2_scheduling.go` | `listSchedulingAssignments` | Append `/all` to path → `/workspaces/{ws}/scheduling/assignments/all`; add mandatory `start` and `end` query params (`yyyy-MM-ddThh:mm:ssZ`). Host is `api.clockify.me`. | BLOCK | findings/scheduling.md |
| 5 | time-off | `internal/tools/tier2_time_off.go` | `listTimeOffRequests` | Change verb GET→POST; `Content-Type: application/json`; body `{"page":1,"pageSize":50}` (all fields optional — `{}` also returns 200); deserialise into `struct{ Count int; Requests []map[string]any }` and return `.Requests`. | BLOCK | findings/time-off.md |
| 6 | project-memberships | `internal/tools/tier2_project_admin.go` | `setProjectMemberships` | (a) Change verb PUT→PATCH — `PUT /projects/{id}/memberships` is 405. (b) After PATCH, response is the **full project object** (keys: `id, name, hourlyRate, clientId, workspaceId, billable, memberships, color, estimate, archived, duration, clientName, note, costRate, timeEstimate, budgetEstimate, estimateReset, template, public`); extract `result["memberships"]`. (c) **REPLACE semantics confirmed live**: PATCH with `[userId_A]` on a 2-member project leaves only `[userId_A]` — callers must send the full desired member list. | BLOCK + SHAPE | findings/project-memberships.md |
| 7 | invoices | `internal/tools/tier2_invoices.go` | `invoiceReport` | Replace GET with `POST /workspaces/{ws}/invoices/info`; JSON body `{"statuses":["PAID"]}`. Apply same `{total, invoices:[]}` envelope fix. | BLOCK | INVOICEDOC.md |
| 8 | holidays | `internal/tools/tier2_groups_holidays.go` | `createHoliday` | Replace flat `"date"` with nested `"datePeriod": {"startDate":"…","endDate":"…"}` (`yyyy-MM-dd`). Also: body must include at least one of `users.ids` or `userGroups.ids` — omitting both returns `400 code:501 "At least one user or user group must be assigned"`. Rename `"recurring"` → `"occursAnnually"`. | BLOCK | findings/holidays.md |
| 9 | invoices | `internal/tools/tier2_invoices.go` | `listInvoices` | Deserialise into `struct{ Total int \`json:"total"\`; Invoices []map[string]any \`json:"invoices"\` }` and return `res.Invoices`. | SHAPE | findings/invoices.md |
| 10 | invoices | `internal/tools/tier2_invoices.go` | `listInvoices` | Change status filter from `?status=PAID` to `?statuses=PAID` (plural, repeatable). Singular `status` is silently ignored. | ENUM | INVOICEDOC.md |
| 11 | expenses | `internal/tools/tier2_expenses.go` | `listExpenses` | Deserialise into double-nested envelope `struct{ Expenses struct{ Expenses []map[string]any; Count int } \`json:"expenses"\` }` and return `res.Expenses.Expenses`. Top-level also has `dailyTotals` and `weeklyTotals`. | SHAPE | findings/expenses.md |
| 12 | expenses | `internal/tools/tier2_expenses.go` | `listExpenseCategories` | Deserialise into `struct{ Count int; Categories []map[string]any \`json:"categories"\` }` and return `res.Categories`. The endpoint returns `{count, categories}` — not a bare array. | SHAPE | findings/expenses.md |
| 13 | expenses | `internal/tools/tier2_expenses.go` | `updateExpense` | `PUT /expenses/{id}` is multipart; add required `changeFields` array: valid values are `USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE, FILE`. Omitting it silently ignores changes. | ENUM | findings/expenses.md |
| 14 | webhooks | `internal/tools/tier2_webhooks.go` | `listWebhooks` | Deserialise into `struct{ WorkspaceWebhookCount int \`json:"workspaceWebhookCount"\`; Webhooks []map[string]any \`json:"webhooks"\` }` and return `.Webhooks`. | SHAPE | findings/webhooks.md |
| 15 | webhooks | `internal/tools/tier2_webhooks.go` | `listWebhookEvents` | Remove live HTTP call (`/webhooks/events` → 400). Return the static enum (52 values, all listed in findings/webhooks.md). | ENUM | findings/webhooks.md |
| 16 | shared-reports | `internal/tools/tier2_shared_reports.go` | `listSharedReports` | Deserialise into `struct{ Reports []map[string]any \`json:"reports"\`; Count int }` and return `.Reports`. | SHAPE | findings/shared-reports.md |
| 17 | shared-reports | `internal/tools/tier2_shared_reports.go` | `listSharedReports` | Use `pageSize` (camelCase) not `page-size` (hyphenated) — hyphenated is silently ignored and returns the default 50. Live confirmed: `?pageSize=2` returns exactly 2, `?page-size=2` returns 50. | ENUM | findings/shared-reports.md |
| 18 | scheduling | `internal/tools/tier2_scheduling.go` | `listSchedulingProjectTotals` | Path is `POST /workspaces/{ws}/scheduling/assignments/projects/totals` (not `/scheduling/projects/totals` — missing `assignments/` segment). Body: `{start, end, pageSize}`. Response: bare array of `{workspaceId, projectId, projectName, projectColor, projectArchived, clientName, totalHours, assignments:[{date, hasAssignment}], milestones, projectBillable, taskId, taskName}`. | BLOCK | findings/scheduling.md |
| 19 | custom-fields | `internal/tools/tier2_custom_fields.go` | `createCustomField` | Update `type` enum from `{TEXT, DROPDOWN, …}` to `{TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX, LINK}` — upstream rejects `TEXT` and `DROPDOWN` with explicit error listing all valid values. All 6 correct values confirmed live. | ENUM | findings/custom-fields.md |
| 24 | shared-reports | `internal/tools/tier2_shared_reports.go` | `createSharedReport` | (a) Rename body key `reportType` → `type` (server rejects `reportType`, accepts `type` from a 19-value enum). (b) Rename body key `filters` → `filter` (singular — `ReportFilterV1`). (c) Make `filter.{exportType, dateRangeStart, dateRangeEnd}` required (server returns 400 listing each missing field). (d) Update the JSON Schema descriptor at `tier2_shared_reports.go:53-63` so `report_type` → `type` and `filter` is documented as required. | BLOCK + ENUM | findings/shared-reports.md |
| 25 | shared-reports | `internal/tools/tier2_shared_reports.go` | `updateSharedReport` | Keep `PUT /workspaces/{ws}/shared-reports/{id}` (workspace-prefixed PUT works — bare-id PUT/PATCH both 405). Apply the same body-key fixes from #24: `reportType` → `type`, `filters` → `filter`. PUT is **merge** semantics (`{name}` alone preserves the existing filter), distinct from #6. | BLOCK | findings/shared-reports.md |
| 26 | shared-reports | `internal/tools/tier2_shared_reports.go` | `deleteSharedReport` | **No code change required** — `DELETE /workspaces/{ws}/shared-reports/{id}` returns 204. The handler path is correct. Update the comment at `tier2_shared_reports.go:153-156` to clarify that only GET goes bare-id; PUT/DELETE stay workspace-prefixed. | DOC | findings/shared-reports.md |
| 27 | shared-reports | `internal/tools/tier2_shared_reports.go` | `exportSharedReport` | Drop the `/export` path segment (404). Switch path to bare `/shared-reports/{id}` and pass `exportType` (not `format`) as the query param mapped to `PDF|CSV|XLSX|JSON_V1`. Switch the response handling to a binary-aware envelope `{contentType, filename, bytes, body(base64)}` because PDF/XLSX bodies are binary and CSV is text — `map[string]any` decode fails. The filename should be parsed from `Content-Disposition`. | BLOCK + SHAPE | findings/shared-reports.md |

---

## Tests that flip from pinned-error to success-path

| # | Domain | Test (file) | Action |
|---|---|---|---|
| 1 | invoices | `listInvoices` in `tests/tier2_invoices_test.go` | Delete `expectErr` for unmarshal error. Assert `total` (int ≥ 0) and non-nil `invoices` slice. |
| 2 | invoices | `listInvoiceItems` in `tests/tier2_invoices_test.go` | Delete `expectErr` for the 405. Assert returned array matches `items` embedded in single-GET fixture. |
| 3 | invoices | `invoiceReport` in `tests/tier2_invoices_test.go` | Delete `expectErr`. Replace GET mock with POST mock to `/invoices/info` with body `{"statuses":["PAID"]}`. Assert `total` and `invoices` slice with `status == "PAID"`. |
| 4 | expenses | `listExpenses` in `tests/tier2_expenses_test.go` | Delete `expectErr`. Assert top-level `expenses` key, inner `expenses` array, `count` integer. |
| 5 | expenses | `listExpenseCategories` in `tests/tier2_expenses_test.go` | Delete `expectErr` for bare-slice unmarshal error. Assert `count >= 1` and `categories[0]` has `id` and `name`. |
| 6 | expenses | `createExpense` in `tests/tier2_expenses_test.go` | Delete `expectErr` for 415. Assert `id` and `total > 0`. Switch to multipart form encoding with `userId`, ISO 8601 `date`, `categoryId`. `file` field is optional. |
| 7 | webhooks | `listWebhooks` in `tests/tier2_webhooks_test.go` | Delete `expectErr`. Assert `len(result) >= 1`, `result[0]["id"]` non-empty, `result[0]["webhookEvent"]` non-empty. |
| 8 | webhooks | `listWebhookEvents` in `tests/tier2_webhooks_test.go` | Delete `expectErr`. Assert returned list contains `"NEW_TIME_ENTRY"` and `"TIMER_STOPPED"`. |
| 9 | shared-reports | `listSharedReports` in `tests/tier2_shared_reports_test.go` | Delete `expectErr` (404 from wrong host). Assert `len(result) >= 1`, `result[0]["id"]` non-empty, `result[0]["type"]` is one of `SUMMARY|DETAILED|WEEKLY`. |
| 10 | scheduling | `listSchedulingAssignments` in `tests/tier2_scheduling_test.go` | Delete `expectErr` (404 from missing `/all`). Assert result is a slice; when assignments exist, `result[0]["period"]` is a map with `"start"` and `"end"` string keys. Test must supply `start` and `end` params. |
| 11 | time-off | `listTimeOffRequests` in `tests/tier2_time_off_test.go` | Delete `expectErr` (405 from GET). Assert result is a slice; when requests exist, `result[0]["policyId"]` and `result[0]["status"]` non-nil. Test HTTP mock must use POST, not GET. |
| 12 | holidays | `createHoliday` in `tests/tier2_groups_holidays_test.go` | Delete `expectErr` (400 from flat-date body). Assert `result["id"]` non-nil and `result["datePeriod"]` is a non-nil map. Test body must include `datePeriod.{startDate,endDate}` and at least one of `users.ids`/`userGroups.ids`. |
| 13 | project-memberships | `setProjectMemberships` in `tests/tier2_project_admin_test.go` | Delete `expectErr` (405 from PUT). Assert `result["id"]` non-empty (project ID) and `result["memberships"]` non-nil array. Test mock must use PATCH and expect full project object response. |
| 14 | custom-fields | `createCustomField` in `tests/tier2_custom_fields_test.go` | Delete `expectErr` (rejection from `type: "TEXT"`). Assert `result["id"]` non-nil and `result["type"]` non-empty. Test body must pass `"type": "TXT"` (not `"TEXT"`); for dropdown tests use `"DROPDOWN_SINGLE"` with a non-empty `allowedValues`. |
| 15 | shared-reports | new `TestTier2Dispatch_SharedReports_Create` in `internal/tools/tier2_shared_reports_dispatch_test.go` | Add a `mux.HandleFunc("/workspaces/test-workspace/shared-reports", POST)` that fails the test if the body has `"reportType"` or `"filters"` keys, and that asserts presence of `"type"`, `"filter.exportType"`, `"filter.dateRangeStart"`, `"filter.dateRangeEnd"`. Reply with the canonical `fixtures/shared-reports/create-4-correct-fieldnames.json` shape. Assert the handler returns `result["id"]` non-empty and `result["type"]` non-empty. |
| 16 | shared-reports | new `TestTier2Dispatch_SharedReports_Update` in same file | Assert PUT lands at `/workspaces/test-workspace/shared-reports/{id}` (not bare-id). Body must use `"filter"` (singular) and `"type"` if the user passes `report_type`/`filters` args. Round-trip via the canonical update response shape. |
| 17 | shared-reports | new `TestTier2Dispatch_SharedReports_Delete` in same file | Assert DELETE lands at `/workspaces/test-workspace/shared-reports/{id}` and that the handler tolerates 204 with empty body. |
| 18 | shared-reports | new `TestTier2Dispatch_SharedReports_Export` in same file | Assert GET lands at bare `/shared-reports/{id}` (no workspace segment) and carries `?exportType=PDF\|CSV\|XLSX\|JSON_V1`. Return a 4-byte binary fixture (e.g. `%PDF`) and assert the handler returns `{contentType, filename, body}` instead of trying to JSON-decode. |

---

## Open questions (6 remaining — all low-priority)

| Domain | # | Question | Blocking? |
|---|---|---|---|
| invoices | 1 | `amount` / `unitPrice` in items is integer cents (e.g. `unitPrice: 100000` = $1000.00). Confirm go-clockify exposes raw cents or converts — pick one and document. | No |
| expenses | 2 | `amount=1.50` in multipart form → `total: 150.0` in response. Unit unclear: likely major-unit input stored as minor-unit (cents) output. Confirm before exposing `total`. | No |
| expenses | 3 | `projectId` marked required in docs but API accepted absence (201). Confirm optional vs defaulted. | No |
| webhooks | 4 | `authToken` in responses is per-webhook HMAC secret — `probe_redact` does not strip it. Add `"authToken":"..."` pattern to `probe_redact`. (Ops issue, not go-clockify bug.) | No |
| shared-reports | 7 | Update body cross-validation: PUT `{type:"DETAILED"}` against a `SUMMARY` report — does the server cross-check that `filter.summaryFilter` is now invalid for the new type? Untested. | No |
| shared-reports | 8 | Type-specific filter requirements for non-`SUMMARY` types (`EXPENSE_DETAILED`, `INVOICE_TIME`, `KIOSK_PIN_LIST`, etc.) — what `filter` sub-object is required? Probe only created `SUMMARY`. | No |
| shared-reports | 9 | Full-blob PUT roundtrip: if a client PUTs the bare-id GET response back (with `workspace.workspaceSettings` embedded), does the server accept the extra fields or 400? Untested. | No |

---

## Resolved questions (for the record)

| Domain | Original OQ | Resolution | Source |
|---|---|---|---|
| invoices | Status filter silently ignored | Param is `statuses` (plural, repeatable). Fixed in change #10. | INVOICEDOC.md |
| invoices | `invoiceReport` endpoint | `POST /workspaces/{ws}/invoices/info` with JSON body. Fixed in change #7. | INVOICEDOC.md |
| invoices | `GET /invoices/{id}` items pagination | No pagination param; items fully embedded, no truncation. | INVOICEDOC.md |
| expenses | Single-GET nested vs flat | Single-GET returns flat form (same as create response). | EXPENSESDOC.md |
| expenses | `file` field required? | **NO** — live probe: POST without file returns 201; `fileId` is `""`. | fixtures/expenses/create-no-file.json |
| scheduling | `page-size` vs `pageSize` | `/all` uses `page-size`; user-totals and project-totals use `pageSize`. | SCHEDULINGDOC.md |
| scheduling | `POST /assignments/projects/totals` exists and path | Confirmed live: `POST /workspaces/{ws}/scheduling/assignments/projects/totals`. Fixed in change #18. | fixtures/scheduling/projects-totals.json |
| scheduling | `capacityPerDay` unit | Seconds (25200 = 7hr/day default; 3600 = 1hr/day workspace override). | SCHEDULINGDOC.md |
| time-off | Empty POST body valid? | **YES** — `{}` returns 200 confirmed live. | fixtures/time-off/requests-list-v2.json |
| time-off | No `policyId` filter in POST body | Confirmed — no such field; callers filter client-side. | TIMEOFFDOC.md |
| time-off | `createTimeOffRequest` body shape | `POST /time-off/policies/{policyId}/requests` with `{timeOffPeriod:{period:{start,end}}, note?}`. | TIMEOFFDOC.md |
| holidays | Minimum required fields | `name` + `datePeriod.{startDate,endDate}` + at least one `users.ids`/`userGroups.ids`. | fixtures/holidays/create.json (live probe) |
| holidays | `everyoneIncludingNew` default | Default is `false` per docs; live values of `true` were explicitly set. | HOLIDAYSDOC.md |
| holidays | `GET /holidays/in-period` | Confirmed — requires `assigned-to`, `start`, `end` query params. | HOLIDAYSDOC.md |
| holidays | DELETE requires archive-first? | **NO** — `DELETE /holidays/{id}` returns 200 directly. | fixtures/holidays/delete.json (live probe) |
| project-memberships | PATCH replace vs upsert? | **REPLACE** confirmed live — PATCH `[userId_A]` on 2-member project → 1 member. Fixed in change #6. | fixtures/project-memberships/patch-replace-semantics.json |
| project-memberships | `POST /memberships` for assign/remove | Confirmed — `POST /projects/{id}/memberships` with `{userIds, remove}`. | fixtures/project-memberships/post-add-member.json |
| project-memberships | PATCH response shape | Full project object confirmed live; extract `result["memberships"]`. Fixed in change #6. | fixtures/project-memberships/patch-response-full-project.json |
| custom-fields | `allowedValues` required for DROPDOWN? | Not marked required in docs; workspace at cap so live create not possible. Treat as implicitly required per all observed instances having non-empty values. | CUSTOMFIELDSDOC.md |
| custom-fields | Project-level CF endpoints | `GET /projects/{id}/custom-fields` (bare array), `DELETE …/{cfId}`, `PATCH …/{cfId}` with `{defaultValue, status}`. | CUSTOMFIELDSDOC.md |
| webhooks | `webhookEvent` vs `events` in create body | `webhookEvent` (singular string) confirmed in create body. | WEBHOOKDOC.md |
| shared-reports | `pageSize` vs `page-size` | **`pageSize` (camelCase)** confirmed live — `page-size` silently returns default 50. Fixed in change #17. | fixtures/shared-reports/ (live probe) |
| shared-reports | OQ #5 — `getSharedReport` workspace-segment | **Resolved**: bare-id is the only path that accepts GET; ws-prefixed per-id GET returns 405 (not 404 — path exists, GET disallowed). Existing handler is correct. | fixtures/shared-reports/discover_ws-prefixed-get.json (live probe) |
| shared-reports | OQ #6 — `exportType` query on single-get | **Resolved**: bare-id GET accepts `exportType=PDF\|CSV\|XLSX\|JSON_V1`. Each non-JSON value returns binary or text with the matching `Content-Type` and a `Content-Disposition: filename=Clockify_Time_Report_…` header. | fixtures/shared-reports/export-{pdf,csv,xlsx}.{headers.txt,binary-summary.json} (live probe) |
| shared-reports | OQ — write/export field names | `type` (not `reportType`) and `filter` (not `filters`) confirmed live as the canonical body keys. `filter.{exportType, dateRangeStart, dateRangeEnd}` are the minimum required nested fields for create. PUT semantics is **merge** (partial body preserves existing filter). | fixtures/shared-reports/create-{1-min,2-with-filter,3-with-summaryFilter,4-correct-fieldnames}.json + update-wsprefixed-put.json (live probe) |
| shared-reports | OQ — write/export route shape | `POST` and `PUT` and `DELETE` all live on `/v1/workspaces/{ws}/shared-reports[/{id}]`. Bare-id PUT/PATCH/DELETE all 405. The previously assumed `/export` segment does not exist (404). | fixtures/shared-reports/{update-bare-put,update-bare-patch,delete-bare,discover_ws-prefixed-export}.json (live probe) |

---

## Next Opus implementation batch

These are ordered from highest confidence (fixture evidence + single-function fix)
to lowest (shape changes that touch response parsing):

### Batch 1 — verb/host/path fixes (each fix is a 1-line change)

1. **`setProjectMemberships` PUT→PATCH** (change #6a) — 1 line: change `.Put(` to `.Patch(`
2. **`listSchedulingAssignments` `/all` suffix + params** (change #4) — add `/all` to path string; add `start`/`end` to query builder
3. **`listTimeOffRequests` GET→POST** (change #5) — change `.Get(` to `.Post(`; add JSON body; switch deserialiser to `{Count, Requests}` struct
4. **Shared-reports host** (change #3) — change base URL constant from `api.clockify.me/api/v1` to `reports.api.clockify.me/v1`

### Batch 2 — response-shape fixes (deserialiser changes only)

5. **`listSharedReports` envelope** (change #16) — add `struct{ Reports []map[string]any; Count int }` wrapper
6. **`setProjectMemberships` response extraction** (change #6b) — after PATCH, extract `result["memberships"]` from full project object
7. **`listInvoices` envelope** (change #9) — add `struct{ Total int; Invoices []map[string]any }` wrapper
8. **`listExpenses` double-nested envelope** (change #11) — add double `struct{ Expenses struct{Expenses []; Count int} }` wrapper
9. **`listExpenseCategories` envelope** (change #12) — add `struct{ Count int; Categories []map[string]any }` wrapper
10. **`listWebhooks` envelope** (change #14) — add `struct{ WorkspaceWebhookCount int; Webhooks []map[string]any }` wrapper

### Batch 3 — body / multipart / enum fixes

11. **`createExpense` multipart** (change #2) — switch HTTP body construction from JSON to multipart; required fields: `userId`, `amount`, `date`, `categoryId`
12. **`createHoliday` body restructure** (change #8) — replace `date` with `datePeriod.{startDate,endDate}`; add `users.ids` or `userGroups.ids`; rename `recurring` → `occursAnnually`
13. **`createCustomField` type enum** (change #19) — update enum map/const from `TEXT→TXT`, `DROPDOWN→DROPDOWN_SINGLE`
14. **`listInvoices` param name** (change #10) — rename `status` → `statuses`
15. **`listSharedReports` pageSize param** (change #17) — rename `page-size` → `pageSize`

### Batch 4 — compound fixes (need both body and deserialiser)

16. **`invoiceReport` GET→POST + envelope** (change #7) — new endpoint, new verb, new body, new deserialiser
17. **`listInvoiceItems` restructure** (change #1) — replace dedicated call with single-GET; extract embedded `items` array
18. **`listWebhookEvents` static** (change #15) — delete HTTP call, return static 52-value enum slice
19. **`updateExpense` changeFields** (change #13) — add `changeFields` multipart field to existing PUT call
20. **`listSchedulingProjectTotals` path** (change #18) — fix path to include `assignments/` segment

### Batch 5 — shared-reports write/export (rev 4 additions)

21. **`createSharedReport` body keys + required filter** (change #24) — rename `reportType`→`type`, `filters`→`filter`; mark `filter.{exportType, dateRangeStart, dateRangeEnd}` required; widen the `type` enum to the 19-value server set or document the user-facing subset
22. **`updateSharedReport` body keys** (change #25) — same body-key fix; **path is already correct** (workspace-prefixed PUT), do not move it bare-id
23. **`deleteSharedReport` no-op** (change #26) — no code change; only update the comment block at `tier2_shared_reports.go:153-156` so future readers don't generalise the bare-id GET path to PUT/DELETE
24. **`exportSharedReport` route + binary envelope** (change #27) — drop `/export`, switch to bare `/shared-reports/{id}` with `?exportType=`; replace `map[string]any` decode with binary-aware envelope `{contentType, filename, body(base64)}`

---

## Cross-references

- Per-domain finding: `findings/<domain>.md`
- Per-domain redacted fixtures: `fixtures/<domain>/`
- Cleanup registry: `cleanup-registry/<domain>.tsv`
- Official API docs: `<DOMAIN>DOC.md` in project root
