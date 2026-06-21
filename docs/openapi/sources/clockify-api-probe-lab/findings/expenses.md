# Finding: expenses

## Endpoint(s) probed

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{ws}/expenses?page=1&page-size=5 | 200 | fixtures/expenses/list-page1.json |
| GET | api.clockify.me | /workspaces/{ws}/expenses/categories | 200 | fixtures/expenses/categories-list.json (v2: categories-list-v2.json — wrapped `{count, categories}`) |
| POST | api.clockify.me | /workspaces/{ws}/expenses/categories | 201 | fixtures/expenses/category-create.json |
| PUT | api.clockify.me | /workspaces/{ws}/expenses/categories/{id} | 200 | fixtures/expenses/category-update.json |
| POST | api.clockify.me | /workspaces/{ws}/expenses (multipart, no userId) | 400 | fixtures/expenses/expense-create-multipart.json (first attempt) |
| POST | api.clockify.me | /workspaces/{ws}/expenses (multipart, with userId) | 201 | fixtures/expenses/expense-create-multipart.json (final) |
| DELETE | api.clockify.me | /workspaces/{ws}/expenses/{expenseId} | 200 | live (created+deleted, cleaned=1) |
| PATCH | api.clockify.me | /workspaces/{ws}/expenses/categories/{categoryId}/status | 200 | live (archive then remove, cleaned=1) |

Note: `PROBE_LAST_STATUS` is `unknown` in all probe-script-generated `.status.txt` files
due to the bash subshell scoping bug documented in the invoices finding. Status for the
manual multipart retries was captured correctly (400, 201).

## Request headers (no secrets)

- `X-Api-Key: [REDACTED]`
- `Content-Type: multipart/form-data` (set automatically by curl `--form`; not
  `application/json`)

## Request body (when applicable)

### POST /expenses/categories (JSON)
```json
{"name": "mcp-probe-<prefix>-cat-0"}
```
Content-Type: `application/json`. Standard JSON body accepted.

### POST /expenses (multipart, successful)
Form fields sent:
```
userId=<user-id>               ← required (non-empty string)
amount=100                     ← required (number<double>)
date=2026-05-02T18:03:15Z      ← required, must be ISO 8601 yyyy-MM-ddThh:mm:ssZ
notes=mcp-probe-<prefix>-exp-0
categoryId=<category-id>       ← required
billable=true
file=@<file>;type=text/plain   ← required per docs (binary); probe used synthetic in-memory file
```

Fields the docs list as required but probe succeeded without:
- `projectId` — docs mark required; probe omitted it and still got 201. May be nullable in practice.
- `file` — docs mark as `required string<binary>`; live probe (2026-05-02) sent zero file fields and received 201. `fileId` in response is `""` (empty string, not null). The field is truly optional despite the docs.

First attempt omitted `userId` → 400 `{"message":"User ID is required","code":501}`.
Second attempt omitted ISO time component in `date` → 400 with date-format error.
Third attempt (all fields correct) → 201.

## Response shape

### GET /expenses list

```json
{
  "expenses": {
    "expenses": [
      {
        "id": "<string>",
        "workspaceId": "<string>",
        "userId": "<string>",
        "date": "<RFC3339>",
        "project": { "id": "<string>", "name": "<string>", "color": "<string>", "clientId": "<string>", "clientName": "<string>" } | null,
        "task": null | <object>,
        "category": {
          "id": "<string>",
          "name": "<string>",
          "hasUnitPrice": <bool>,
          "priceInCents": <integer>,
          "unit": "<string>",
          "workspaceId": "<string>",
          "archived": <bool>
        },
        "notes": "<string>",
        "quantity": <float>,
        "billable": <bool>,
        "fileId": "<string>",
        "fileName": "<string>" | null,
        "total": <float>,
        "locked": <bool>
      }
    ],
    "count": <integer>
  },
  "dailyTotals": [
    { "date": "<YYYY-MM-DD>", "total": <float>, "dateAsInstant": "<RFC3339>" }
  ],
  "weeklyTotals": [
    { "date": "<YYYY-MM-DD>", "total": <float> }
  ]
}
```

The outer key is `expenses`, the inner key is also `expenses`. The inner count is also at
`expenses.count`, not at the top level. The double-nesting is confirmed by the live fixture.

### GET /expenses/categories

```json
{
  "count": <integer>,
  "categories": [
    {
      "id": "<string>",
      "name": "<string>",
      "hasUnitPrice": <bool>,
      "priceInCents": <integer>,
      "unit": "<string>",
      "workspaceId": "<string>",
      "archived": <bool>
    }
  ]
}
```

### POST /expenses/categories (create) and PUT (update) response

Both return the same flat category object:
```json
{
  "id": "<string>",
  "name": "<string>",
  "hasUnitPrice": false,
  "priceInCents": 0,
  "unit": "",
  "workspaceId": "<string>",
  "archived": false
}
```

### POST /expenses (create) response

```json
{
  "id": "<string>",
  "workspaceId": "<string>",
  "userId": "<string>",
  "date": "<RFC3339>",
  "projectId": null | "<string>",
  "taskId": null | "<string>",
  "categoryId": "<string>",
  "notes": "<string>",
  "quantity": <float>,
  "billable": <bool>,
  "fileId": "<string>",
  "total": <float>,
  "locked": false
}
```

Note: the create response uses `projectId`/`taskId` (flat IDs) rather than the nested
`project`/`task` objects returned by the list endpoint.

### Amount scaling observation

Sent `amount=100` form field; response showed `total: 10000.0`. The `amount` multipart
field appears to be multiplied by 100 before being stored as `total`. This may mean
`amount` is in major currency units (dollars/euros) while `total` is stored in minor
units (cents), or there is a different scaling mechanism. See Open questions.

## Cleanup behavior

- **Expense category** `mcp-probe-1777744952-c1dc74-cat-0` (id `69f63c38755f4967f838e02c`)
  was created and renamed via the mutating probe. Archived via
  `PATCH /v1/workspaces/{ws}/expenses/categories/{id}/status` with body `{"archived":true}`
  → 200. Marked `cleaned=1` in the registry.
  **Correction from official docs:** `DELETE /workspaces/{ws}/expenses/categories/{id}` →
  204 also exists; future probes should prefer DELETE for full removal. PATCH /status
  is a soft-archive (reversible); DELETE is permanent.

- **Expense** `mcp-probe-1777744995-1118fb-exp-0` (id `69f63c63307374b4631f1681`) was
  created and immediately deleted via `DELETE /workspaces/{ws}/expenses/{id}` → 200 OK.
  Marked `cleaned=1` in the registry.

## Recommended go-clockify change

### Bug 1 — listExpenses double-nested envelope

- **File:** `internal/tools/tier2_expenses.go`
- **Function:** `listExpenses`
- **Change:** Deserialise into:
  ```go
  var result struct {
      Expenses struct {
          Expenses []map[string]any `json:"expenses"`
          Count    int              `json:"count"`
      } `json:"expenses"`
      DailyTotals  []map[string]any `json:"dailyTotals"`
      WeeklyTotals []map[string]any `json:"weeklyTotals"`
  }
  ```
  and return `result.Expenses.Expenses`. The `Count` and totals fields should be exposed
  if the tool surfaces them; otherwise discard.

### Bug 2 — listExpenseCategories envelope

- **File:** `internal/tools/tier2_expenses.go`
- **Function:** `listExpenseCategories`
- **Change:** Deserialise into `struct{ Count int; Categories []map[string]any \`json:"categories"\` }` and return `res.Categories`.

### Bug 3 — createExpense Content-Type and required fields

- **File:** `internal/tools/tier2_expenses.go`
- **Function:** `createExpense`
- **Change:** Switch from `application/json` body to `multipart/form-data`. Required
  fields per docs: `userId` (non-empty string), `amount` (`number<double>`), `date` (ISO
  8601 full datetime `yyyy-MM-ddThh:mm:ssZ`), `categoryId` (string), `file` (binary
  attachment, field name `file` not `receipt`). Optional: `notes`, `billable`,
  `projectId`, `taskId`. Remove the `Content-Type: application/json` header; let the
  HTTP client set the multipart boundary automatically.

### Additional — updateExpense `changeFields` requirement

- **File:** `internal/tools/tier2_expenses.go`
- **Function:** `updateExpense` (if it exists)
- **Change:** `PUT /expenses/{id}` is also multipart and requires a `changeFields` array
  enumerating which fields are being updated. Valid values per docs:
  `USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE, FILE`. Any update
  request that omits `changeFields` will silently ignore the changed values. Example:
  to update the notes field, send `changeFields=NOTES` (or as a repeated form field for
  multiple values) alongside the new `notes` value.

## Test that flips from pinned-error to success-path

### listExpenses

Delete the `expectErr` annotation for the unmarshal error. Replace with a shape
assertion that the response contains a top-level `expenses` key, an inner `expenses`
array, and a `count` integer. Example: `result.Expenses.Count == 2826` for this workspace
or `len(result.Expenses.Expenses) <= pageSize`.

Test file expected in `tests/` — search for `listExpenses` in go-clockify test files.

### listExpenseCategories

Delete the `expectErr` for the bare-slice unmarshal error. Assert `count >= 1` and
`categories[0]` has `id` and `name` keys. Evidence: `categories-list.json` shows 60
categories.

### createExpense

Replace the `expectErr` for the 415 Unsupported Media Type. Assert the create response
has an `id` field and `total > 0`. The test's request construction must be updated to
use multipart form encoding rather than JSON marshalling, and must include `userId` and a
full ISO 8601 `date`.

## Open questions

1. **Amount scaling:** Sent `amount=100`, received `total: 10000.0`. It is unclear whether
   `amount` is in major currency units (and Clockify stores in minor units ×100), or
   if `amount` is treated as a multiplier against some workspace default rate.
   The go-clockify `createExpense` handler will need to document and test the correct
   scale. Compare with existing expenses in the workspace: `list-page1.json` shows
   entries with `quantity: 2200.0, total: 2200.0` (no scaling for non-unit-price
   categories) vs `quantity: 2.0, total: 2000.0` for unit-price category with
   `priceInCents: 1000` (total = quantity × priceInCents).

2. **Expense category DELETE confirmed:** Official docs confirm `DELETE
   /v1/workspaces/{ws}/expenses/categories/{id}` → 204 exists. The earlier claim of
   "no DELETE endpoint" was incorrect (based on outdated probe-script comments). A
   separate `PATCH /categories/{id}/status` with `{"archived":true}` → 200 also exists
   for soft-archive. The probe used the PATCH route. go-clockify's
   `deleteExpenseCategory` handler calling DELETE should work correctly.

3. **`projectId` vs `project` object:** The list endpoint embeds the full `project`
   object in each expense; the create response returns a flat `projectId`. Confirm
   whether a single-GET endpoint for an expense returns the flat or nested form, and
   whether go-clockify's handler needs to handle both shapes.

4. **Orphaned `mcp-live-*` categories:** The `categories-list.json` fixture shows many
   `mcp-live-*` and `mcp-probe-*` prefixed categories from prior campaign runs that are
   not in the cleanup registry. These are known orphans from previous sessions and
   require manual UI archival. They do not affect this probe's findings.

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox by the
read-side schema oracle; clean canonical paths (no query string) so the
generator's `normalize_path` matches the merged operation key and
`status_bucket` flips each op to `live-success`. Fixtures are
documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/expenses | 200 | fixtures/live-shape/expenses-list.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/expenses/{expenseId} | 200 | fixtures/live-shape/expenses-get.json |

## Live write-side promotion (2026-06-21)

The historical probe table above records the multipart create returning
HTTP 201 (the "with userId" attempt), but its Path cell carries a
` (multipart, with userId)` qualifier that `normalize_path` cannot strip,
so that row never bound the canonical `createExpense` operation key and
the op stayed `probe-documented`. The clean-path row below binds the
already-captured 201 to `live-success`. The create (multipart, with
userId, no file) + `DELETE /workspaces/{ws}/expenses/{id}` cleanup is
documented above (entry `69f63c63307374b4631f1681`, `cleaned=1`); the
fixture is documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/expenses | 201 | fixtures/expenses/expense-create-multipart.json |
