# Finding: shared-reports

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /api/v1/workspaces/{ws}/shared-reports | 404 | fixtures/shared-reports/api__workspaces_{ws}_shared-reports.json |
| GET | api.clockify.me | /api/v1/workspaces/{ws}/reports/shared | 404 | fixtures/shared-reports/api__workspaces_{ws}_reports_shared.json |
| GET | reports.api.clockify.me | /v1/workspaces/{ws}/shared-reports | **200** | fixtures/shared-reports/reports__workspaces_{ws}_shared-reports.json |
| GET | reports.api.clockify.me | /v1/workspaces/{ws}/reports/shared | 404 | fixtures/shared-reports/reports__workspaces_{ws}_reports_shared.json |
| GET | reports.api.clockify.me | /api/v1/workspaces/{ws}/shared-reports | 404 | fixtures/shared-reports/reports2__workspaces_{ws}_shared-reports.json |
| GET | reports.api.clockify.me | /api/v1/workspaces/{ws}/reports/shared | 404 | fixtures/shared-reports/reports2__workspaces_{ws}_reports_shared.json |
| GET | reports.api.clockify.me | /v1/shared-reports/{id} | **200** | fixtures/shared-reports/reports_single-get.json |

All `api.clockify.me` paths returned 404. All `reports.api.clockify.me/api/v1` paths returned 404. Only `reports.api.clockify.me/v1` is live.

## Request headers (no secrets)
- X-Api-Key: [REDACTED]
- Content-Type: not sent (all GET; no body)

## Request body (when applicable)
n/a — all probes were read-only GETs.

## Response shape

### GET /v1/workspaces/{ws}/shared-reports — 200
```json
{
  "reports": [
    {
      "reportAuthor": "<string>",
      "name": "<string>",
      "link": "https://app.clockify.me/shared/<id>",
      "id": "<string>",
      "visibleToUsers": [{ "id": "<string>", "name": "<string>" }],
      "fixedDate": false,
      "type": "SUMMARY",
      "visibleToUserGroups": [],
      "isPublic": true
    }
  ],
  "count": 74
}
```
Wrapped object — `reports` array + top-level `count`. `type` observed values in live data: `SUMMARY`, `DETAILED`, `WEEKLY`.

Query params accepted (confirmed live): `page` (integer), `pageSize` (integer), `sharedReportsFilter` (`ALL` | `CREATED_BY_ME` | `SHARED_WITH_ME`).

### GET /v1/shared-reports/{id} — 200
Full report data object — **no workspace segment in this path**. Returns report results plus full configuration:
```json
{
  "totals": [],
  "donutChart": [],
  "groupTotals": {},
  "groupOne": [],
  "filters": {
    "id": "<string>",
    "workspaceId": "<string>",
    "userId": "<string>",
    "name": "<string>",
    "isPublic": <boolean>,
    "visibleToUserGroups": [],
    "visibleToUsers": ["<userId>"],
    "fixedDate": <boolean>,
    "type": "<string>",
    "exportVisibleCustomFieldIds": [],
    "exportVisibleElements": [],
    "filter": {
      "dateRangeStart": "<ISO8601>",
      "dateRangeEnd": "<ISO8601>",
      "dateRangeType": "<string>",
      "projects": { "contains": "CONTAINS", "ids": [], "status": "ACTIVE" },
      "summaryFilter": { "groups": [], "sortColumn": "<string>", ... },
      "exportType": "JSON_V1",
      "sortOrder": "DESCENDING",
      ...
    },
    "workspace": { "id": "<string>", "name": "<string>", "workspaceSettings": { ... } },
    "subscriptionPlan": "<string>",
    "isAdminOrOwner": <boolean>
  }
}
```
The `totals`/`groupOne` arrays are empty for a SUMMARY report with no time entries in the filtered range. The `filters` object embeds the full workspace settings — significantly larger than the list shape.

### GET /api/v1/workspaces/{ws}/shared-reports (wrong host) — 404
```json
{"message": "No static resource v1/workspaces/{ws}/shared-reports.", "code": 3000}
```

## Cleanup behavior
Read-only probe — no entities were created. `cleanup-registry/shared-reports.tsv` was not written. Nothing to clean up.

## Recommended go-clockify change

### Bug: wrong host + wrong base path prefix
- File: `internal/tools/tier2_shared_reports.go`
- Function: any function that constructs or passes the base URL for shared-report API calls (likely a `newSharedReportsClient()` helper or a `const` / `var` at the top of the file)
- Change: Replace `https://api.clockify.me/api/v1` with `https://reports.api.clockify.me/v1`. Both the host (`api.clockify.me` → `reports.api.clockify.me`) and the path prefix (`/api/v1` → `/v1`) must change. Using `/api/v1` on the reports host is also a 404.

### Bug: list-shape mismatch
- File: `internal/tools/tier2_shared_reports.go`
- Function: `listSharedReports` (or equivalent)
- Change: Deserialize into `struct{ Reports []map[string]any \`json:"reports"\`; Count int \`json:"count"\` }` and return `.Reports`. The current handler almost certainly deserializes into a bare slice and gets nothing because the root value is an object.

## Test that flips from pinned-error to success-path
- Test: the test for `listSharedReports` in `tests/tier2_shared_reports_test.go`
- Action: Remove the `expectErr` annotation (the 404 from the wrong host). Replace with an assertion that `len(result) >= 1` and that `result[0]["id"]` and `result[0]["name"]` are non-empty strings, and that `result[0]["type"]` is one of `SUMMARY|DETAILED|WEEKLY`.

## Open questions

1. **`getSharedReport` path shape — RESOLVED.** Single-get is `GET /v1/shared-reports/{id}` (no workspace segment). The handler at `tier2_shared_reports.go:157` was already fixed in PR #53; pinned by `tier2_shared_reports_dispatch_test.go::TestTier2Dispatch_SharedReports_Get`. The new write/export probe (2026-05-03) re-confirms the bare-id GET path is the only one that accepts GET — workspace-prefixed per-id GET returns **405 Method Not Allowed** (path exists, but only PUT/DELETE are accepted there; see "Write/export probes" below).

2. **Report data vs. report metadata — RESOLVED.** The single-get (`/v1/shared-reports/{id}`) returns the full report data object (totals, chart data, filter config, workspace settings). The handler returns this blob. The list endpoint returns lightweight metadata. Both behaviours are acceptable.

3. **`exportType` query param for single-get — RESOLVED.** Confirmed live (2026-05-03): the bare-id GET accepts `exportType=PDF|CSV|XLSX|JSON_V1`. JSON_V1 returns a JSON body; the others return binary (PDF magic `%PDF-1.5`, XLSX magic `PK\x03\x04`, CSV is plaintext). Each non-JSON export ships `Content-Type` and `Content-Disposition: filename=Clockify_Time_Report_<...>.{ext}`. See `fixtures/shared-reports/export-{pdf,csv,xlsx}.headers.txt` and `…binary-summary.json`.

4. **`page-size` vs `pageSize` — RESOLVED.** Live probe (2026-05-02) with `?pageSize=2` returned exactly 2 results (`count=74`). The correct param name is `pageSize` (camelCase). Using `page-size` (hyphenated) silently returns the default of 50. go-clockify must send `pageSize` not `page-size` to this endpoint.

---

## Write/export probes (2026-05-03)

Probe script: `probes/shared-reports-write.sh`. All bodies were redacted before save. The probe ran read-only discovery first (Phase A), then a bounded create ladder (B.1→B.4), per-id update/delete with bare-id-first fallback (Phase C), then best-effort cleanup (Phase D). Total live HTTP calls: 14. Net cleanup: 0 leftover entries (the one created `mcp-probe-...` report was deleted in C.6 with status 204).

### Endpoints probed

| # | Method | Host | Path | Status | Fixture |
|---|---|---|---|---|---|
| A.1 | GET | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports/{id}` | 405 | `fixtures/shared-reports/discover_ws-prefixed-get.json` |
| A.2 | GET | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports/{id}/export?format=PDF` | 404 | `fixtures/shared-reports/discover_ws-prefixed-export.json` |
| A.3 | GET | reports.api.clockify.me | `/v1/shared-reports/{id}?exportType=PDF` | 200 | `fixtures/shared-reports/export-pdf.{headers.txt,binary-summary.json,status.txt}` |
| A.4 | GET | reports.api.clockify.me | `/v1/shared-reports/{id}?exportType=CSV` | 200 | `fixtures/shared-reports/export-csv.{headers.txt,binary-summary.json,status.txt}` |
| A.5 | GET | reports.api.clockify.me | `/v1/shared-reports/{id}?exportType=XLSX` | 200 | `fixtures/shared-reports/export-xlsx.{headers.txt,binary-summary.json,status.txt}` |
| B.1 | POST | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports` body `{name,reportType:"SUMMARY"}` | 400 | `fixtures/shared-reports/create-1-min.json` |
| B.2 | POST | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports` body `{name,reportType,filter:{exportType:"JSON_V1"}}` | 400 | `fixtures/shared-reports/create-2-with-filter.json` |
| B.3 | POST | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports` body `{name,reportType,filter:{exportType,dateRangeType:"THIS_WEEK",sortOrder,summaryFilter}}` | 400 | `fixtures/shared-reports/create-3-with-summaryFilter.json` |
| B.4 | POST | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports` body `{name,type:"SUMMARY",filter:{exportType,dateRangeStart,dateRangeEnd,sortOrder,summaryFilter}}` | **200** | `fixtures/shared-reports/create-4-correct-fieldnames.json` |
| C.1 | PUT | reports.api.clockify.me | `/v1/shared-reports/{id}` body `{name}` | 405 | `fixtures/shared-reports/update-bare-put.json` |
| C.2 | PATCH | reports.api.clockify.me | `/v1/shared-reports/{id}` body `{name}` | 405 | `fixtures/shared-reports/update-bare-patch.json` |
| C.3 | PUT | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports/{id}` body `{name}` | **200** | `fixtures/shared-reports/update-wsprefixed-put.json` |
| C.4 | GET | reports.api.clockify.me | `/v1/shared-reports/{id}?exportType=JSON_V1` | 200 | `fixtures/shared-reports/round-trip-after-update.json` |
| C.5 | DELETE | reports.api.clockify.me | `/v1/shared-reports/{id}` | 405 | `fixtures/shared-reports/delete-bare.json` |
| C.6 | DELETE | reports.api.clockify.me | `/v1/workspaces/{ws}/shared-reports/{id}` | **204** | `fixtures/shared-reports/delete-wsprefixed.json` (empty body) |

### What this proves

- **The `/v1/workspaces/{ws}/shared-reports/{id}` route exists and accepts PUT and DELETE — but rejects GET (405).** That is the inverse of how `getSharedReport` was wired in earlier handler revisions: GET goes bare-id, write goes ws-prefixed.
- **There is no `/export` segment.** Export is driven by `?exportType=` on the bare-id `/v1/shared-reports/{id}` GET. The current `exportSharedReport` handler at `tier2_shared_reports.go:282` constructs `/workspaces/{ws}/shared-reports/{id}/export?format=…`, which is a guaranteed 404.
- **Create-body field names matter.**
  - The field is `type`, not `reportType`. The B.2/B.3 errors call it out by exact JSON path (`saveSharedReportV1.arg0.type`).
  - The `filter` field (singular) is required and contains the date range, export type, and summary/detailed/weekly sub-filters. The current handler sends `filters` (plural), which is silently ignored.
  - Required nested fields confirmed: `filter.exportType`, `filter.dateRangeStart`, `filter.dateRangeEnd`. `filter.summaryFilter.{groups,sortColumn}` was accepted; defaults like `summaryChartType:"BILLABILITY"` and `dateRangeType:"ABSOLUTE"` (after server normalises the explicit dates) were applied automatically.
  - Accepted `type` enum (19 values, server-listed in B.2 error): `DETAILED`, `WEEKLY`, `SUMMARY`, `SCHEDULED`, `EXPENSE_DETAILED`, `EXPENSE_RECEIPT`, `PTO_REQUESTS`, `PTO_BALANCE`, `ATTENDANCE`, `INVOICE_EXPENSE`, `INVOICE_TIME`, `PROJECT`, `TEAM_FULL`, `TEAM_LIMITED`, `TEAM_GROUPS`, `INVOICES`, `KIOSK_PIN_LIST`, `KIOSK_ASSIGNEES`, `USER_DATA_EXPORT`. The user-facing handler currently advertises only "SUMMARY, DETAILED, WEEKLY" — it should accept the full set or document the subset.
- **PUT semantics: merge, not replace.** C.3 sent only `{"name":"…"}` and the response retained the entire `filter` object (with `dateRangeType` server-normalised from `null` to `"ABSOLUTE"`). Callers can update partially without resending the whole filter blob.
- **Create response shape = report metadata** (not the data blob). Top-level keys (sample): `id, workspaceId, userId, name, isPublic, visibleToUserGroups, visibleToUsers, fixedDate, type, filter`. Update response has the same shape. Both are distinct from the bare-id GET/JSON_V1 response (which embeds `totals/donutChart/groupTotals/groupOne/filters`).
- **Export Content-Types observed**:
  - PDF → `application/pdf`
  - CSV → `text/csv`
  - XLSX → `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
  - All three set `Content-Disposition: filename=Clockify_Time_Report_<reportType>_<MM%2FDD%2FYYYY>-<MM%2FDD%2FYYYY>.<ext>`.
  - Body is binary for PDF/XLSX, plaintext CSV for CSV. The current `exportSharedReport` handler decodes into `map[string]any`, which fails on any non-JSON content type — this is the second blocker.

## Recommended go-clockify changes (write/export — 2026-05-03)

Each item lists the exact source location, the smallest correct change, and the fixture(s) it is pinned by.

### `createSharedReport` — body shape + field-name fix

- File: `internal/tools/tier2_shared_reports.go:166-194`
- Symptom: handler sends `{"name", "reportType", "filters"}`. Live API requires `{"name", "type", "filter"}` and rejects the call with 400 unless `filter.{exportType, dateRangeStart, dateRangeEnd}` are set.
- Change:
  - Rename body key `reportType` → `type` (drop the camelCase mismatch).
  - Rename body key `filters` → `filter` (singular — the upstream Java DTO is `ReportFilterV1`).
  - Treat `filters` arg from the user as the same shape and forward it under the new key.
  - Make `filters` (the user-facing arg) effectively required, and document `filter.{exportType, dateRangeStart, dateRangeEnd}` as the minimum nested fields. Continue to accept the full filter object so the caller can drive `summaryFilter` / `detailedFilter` / `weeklyFilter`.
  - Update the JSON Schema descriptor at `tier2_shared_reports.go:53-63`: rename `report_type` → `type`, mark `filters` (or `filter` in the new shape) required, and document the 19-value `type` enum (or at least the user-facing subset and a free-text fallback).
- Pinned by: `fixtures/shared-reports/create-{1-min,2-with-filter,3-with-summaryFilter,4-correct-fieldnames}.json`. The first three are 400s with explicit field-by-field rejection; the fourth is the 200 with the canonical body and the response shape.

### `updateSharedReport` — keep workspace-prefixed PUT, fix body key

- File: `internal/tools/tier2_shared_reports.go:196-226`
- Symptom: path is correct (PUT works on the workspace-prefixed per-id route), but the body key `filters` is wrong (server expects `filter`), and `reportType` is wrong (server expects `type`).
- Change:
  - Keep `paths.Workspace(wsID, "shared-reports", reportID)` and keep `s.Client.PutReports(...)` — Phase C.3 confirms PUT is correct.
  - Rename body key `reportType` → `type`.
  - Rename body key `filters` → `filter`.
  - Document PUT as merge semantics (PUT with `{"name":"x"}` retains the existing filter — see `fixtures/shared-reports/update-wsprefixed-put.json`).
- Pinned by: `fixtures/shared-reports/update-bare-put.json` (405), `update-bare-patch.json` (405), `update-wsprefixed-put.json` (200 + merged response).

### `deleteSharedReport` — keep workspace-prefixed DELETE

- File: `internal/tools/tier2_shared_reports.go:228-263`
- Symptom: the current path is correct. Phase C confirms.
- Change: **none required.** The bare-id DELETE attempted in C.5 returns 405; the existing workspace-prefixed DELETE path is right. Update the comment at `tier2_shared_reports.go:153-156` so it does not give the impression that **all** per-id ops are bare — only GET is bare; PUT/DELETE go via the workspace prefix.
- Pinned by: `fixtures/shared-reports/delete-bare.json` (405), `delete-wsprefixed.json` (204, empty body).

### `exportSharedReport` — drop `/export` segment, switch to bare-id `?exportType=`, return binary-aware envelope

- File: `internal/tools/tier2_shared_reports.go:265-295`
- Symptom: handler hits `/workspaces/{ws}/shared-reports/{id}/export?format=…` which is a flat 404; even on the right path, the response is binary for PDF/XLSX and plaintext for CSV — JSON-decoding fails.
- Change:
  - Replace the path construction with the bare-id route used by `getSharedReport`: `"/shared-reports/" + reportID`.
  - Map the user-facing `format` arg (`csv|json|pdf|excel|xlsx`) to upstream `exportType` enum (`CSV|JSON_V1|PDF|XLSX`). Default to `JSON_V1` to match `getSharedReport`.
  - Stop using `s.Client.GetReports(...)` for non-JSON formats (it deserialises into a map). Either:
    1. Add a new client method `GetReportsRaw(ctx, path, query) (status int, contentType string, body []byte, err error)` that returns the raw body, **or**
    2. For JSON_V1 only, keep `GetReports`; for other formats, route through a `Client.DoReports` raw-body helper.
  - Return shape: `{"contentType": "...", "filename": "Clockify_Time_Report_…", "bytes": <int>, "body": "<base64>"}`. The filename should be parsed from `Content-Disposition` (URL-decode the slashes that arrive as `%2F`).
- Pinned by: `fixtures/shared-reports/discover_ws-prefixed-export.json` (404 — proves `/export` segment is bogus), `export-{pdf,csv,xlsx}.headers.txt`, `export-{pdf,csv,xlsx}.binary-summary.json`.

## Tests that flip from pinned-error to success-path

- **`createSharedReport` dispatch test** (new — there is no existing test pinning this handler against a fake upstream): in `internal/tools/tier2_shared_reports_dispatch_test.go` add a `mux.HandleFunc("/workspaces/test-workspace/shared-reports", POST)` that asserts the body has `"type"` (not `"reportType"`), `"filter"` (not `"filters"`), and rejects bodies that lack `filter.exportType` / `dateRangeStart` / `dateRangeEnd`. Return the canonical create response shape from `fixtures/shared-reports/create-4-correct-fieldnames.json`.
- **`updateSharedReport` dispatch test** (new): assert the PUT goes to `/workspaces/test-workspace/shared-reports/{id}` (not bare-id) and the body uses `filter`, not `filters`.
- **`deleteSharedReport` dispatch test** (new): assert DELETE goes to `/workspaces/test-workspace/shared-reports/{id}`; assert the response handles a 204 with empty body without erroring.
- **`exportSharedReport` dispatch test** (new): assert GET goes to bare `/shared-reports/{id}` (no workspace segment) with `?exportType=PDF|CSV|XLSX|JSON_V1`. Return a small binary fixture for `PDF` and assert the handler returns `{contentType, filename, body}` rather than a `map[string]any`.

## Cleanup

`cleanup-registry/shared-reports.tsv` — one row from this run, `cleaned=1`. The `mcp-probe-1777763233-57e4d5-shared-correct` report (id `69f683a2a86b2086ac83be22`) was deleted in Phase C.6 with status 204; no residual objects.

## Open questions (still open, low-priority)

- **Update body validation gap.** The probe only exercised `{name}` partial update. What happens if a caller PUTs `{type: "DETAILED"}` against a `SUMMARY` report — does the server cross-validate `filter.summaryFilter` vs `filter.detailedFilter`? Untested. Not blocking, but should be documented as "callers are responsible for sending a coherent filter for the new type."
- **Type-specific filter requirements.** For non-`SUMMARY` types (e.g. `EXPENSE_DETAILED`, `INVOICE_TIME`, `KIOSK_PIN_LIST`), what `filter` sub-object is required? The probe only created a `SUMMARY` report. Lab follow-up could probe one of the non-time types if go-clockify ever wants to expose them.
- **`PUT` body size.** Live probe sent only `{name}`. The existing read-side fixture's `filters.filter` blob is several KB (full workspace settings embedded). If a client tries to PUT the full read-blob back, will the server accept it or reject extra fields like `workspace.workspaceSettings`? Untested.

## Write CRUD promotion (re-probed 2026-06-21)

Re-probe of the B.4/C.3/C.6 create->update->delete ladder with a fresh sandbox
key, written as canonical phase-id-free rows so the generator's table parser
binds them (the original `Write/export probes` table prefixes each row with a
phase-id cell that shifts Method out of `cells[1]`). Net cleanup: 0 leftover
entries — the `sdk-live-probe-sr` report was PUT (merge) then DELETEd (204), and
a follow-up list showed zero `sdk-live-probe` residue.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | reports.api.clockify.me | /v1/workspaces/{workspaceId}/shared-reports | 200 | live-probe 2026-06-21 |
| PUT | reports.api.clockify.me | /v1/workspaces/{workspaceId}/shared-reports/{sharedReportId} | 200 | live-probe 2026-06-21 |
| DELETE | reports.api.clockify.me | /v1/workspaces/{workspaceId}/shared-reports/{sharedReportId} | 204 | live-probe 2026-06-21 |
