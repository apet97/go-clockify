# Finding: invoices

## Endpoint(s) probed

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{ws}/invoices?page=1&page-size=5 | 200 | fixtures/invoices/list-page1.json |
| GET | api.clockify.me | /workspaces/{ws}/invoices?page=1&page-size=50 | 200 | fixtures/invoices/list-bigger.json |
| GET | api.clockify.me | /workspaces/{ws}/invoices/{id} | 200 | fixtures/invoices/single-get.json |
| GET | api.clockify.me | /workspaces/{ws}/invoices/{id}/items | 405 | fixtures/invoices/items-of-1.json |
| GET | api.clockify.me | /workspaces/{ws}/invoices?status=PAID | 200 | fixtures/invoices/report-paid.json |

Note: `PROBE_LAST_STATUS` was not captured in `.status.txt` files due to a bash subshell
scoping bug in `probe_save_fixture` (status is set inside `$(...)` and does not propagate).
HTTP statuses above were inferred from response body content (JSON error object vs valid
data). The `probes/lib/common.sh` library should assign `PROBE_LAST_STATUS` in the parent
shell, not inside a subshell capture.

## Request headers (no secrets)

- `X-Api-Key: [REDACTED]`
- No `Content-Type` sent for GET requests (correct)

## Request body (when applicable)

n/a — all probed endpoints are GET.

## Response shape

### List response (all three list-like endpoints)

```json
{
  "total": <integer>,
  "invoices": [
    {
      "id": "<string>",
      "number": "<string>",
      "status": "UNSENT|SENT|OVERDUE|PAID|...",
      "issuedDate": "<RFC3339>",
      "dueDate": "<RFC3339>",
      "clientId": "<string>",
      "clientName": "<string>",
      "amount": <integer cents>,
      "paid": <null|integer cents>,
      "balance": <integer cents>,
      "currency": "<ISO 4217>"
    }
  ]
}
```

The `total` field is the workspace-wide count regardless of page. The `invoices` array
length is bounded by `page-size`.

### Single GET response

The single-GET response is a superset of the list item shape — it includes all list fields
plus:

```json
{
  "calculationType": "ITEM_BASED|...",
  "subtotal": <integer>,
  "discount": <float>,
  "tax": <float>,
  "tax2": <float>,
  "taxType": "SIMPLE|...",
  "discountAmount": <integer>,
  "taxAmount": <integer>,
  "tax2Amount": <integer>,
  "subject": "<string>",
  "note": "<string>",
  "clientAddress": "<string>",
  "userId": "<string>",
  "items": [
    {
      "order": <integer>,
      "description": "<string>",
      "quantity": <integer>,
      "unitPrice": <integer>,
      "amount": <integer>",
      "itemType": "<string>",
      "importType": "<string>",
      "timeEntryIds": [],
      "expenseIds": [],
      "applyTaxes": "<string>"
    }
  ],
  "visibleZeroFields": ["<string>"],
  "companyId": "<string>",
  "containsImportedTimes": <bool>,
  "containsImportedExpenses": <bool>,
  "billFrom": "WORKSPACE|..."
}
```

### Items endpoint (405 error)

```json
{"message": "Request method 'GET' is not supported", "code": 3000}
```

`GET /workspaces/{ws}/invoices/{id}/items` is not supported. Items are embedded directly
in the single-GET response under the `items` array.

### status filter behaviour

`GET /workspaces/{ws}/invoices?status=PAID` does NOT filter. The probe workspace has 107
invoices (5 OVERDUE, 3 PAID, 42 UNSENT in the first 50 results) and all were returned
regardless of the filter value. Either the `status` query param is silently ignored, or
the filter uses a different parameter name.

## Cleanup behavior

Read-only probe — nothing was created. No cleanup needed. `cleanup-registry/invoices.tsv`
does not exist.

## Recommended go-clockify change

### Bug 1 — listInvoices list-shape mismatch (primary)

- **File:** `internal/tools/tier2_invoices.go`
- **Function:** `listInvoices`
- **Change:** Deserialise into `struct{ Total int \`json:"total"\`; Invoices []map[string]any \`json:"invoices"\` }` instead of `[]map[string]any`, then return `res.Invoices`.

The same fix applies to `invoiceReport` — it calls the same endpoint (`GET
/workspaces/{ws}/invoices`) with an optional `status` query param and gets the identical
envelope.

### Bug 2 — listInvoiceItems: GET 405

- **File:** `internal/tools/tier2_invoices.go`
- **Function:** `listInvoiceItems`
- **Change:** Drop the separate `GET /invoices/{id}/items` request. Instead call `GET
  /workspaces/{ws}/invoices/{id}` (the single-get) and return the embedded `items` array.
  Evidence: `single-get.json` contains a fully populated `items` array; `items-of-1.json`
  shows 405 on the dedicated path.

## Test that flips from pinned-error to success-path

### listInvoices

The campaign pinned `"json: cannot unmarshal object into Go value of type
[]map[string]interface {}"` as a known-fail. After the fix:

1. Delete the `expectErr` annotation for the unmarshal error in the `listInvoices` test.
2. Replace it with a shape assertion: response must contain `total` (int ≥ 0) and
   `invoices` (array, possibly empty). One concrete assertion: `total == 107` for the
   probe workspace, or `len(invoices) <= pageSize`.

Test file is expected at `tests/` or `internal/tools/` inside go-clockify — confirm by
searching for the literal string `listInvoices` in test files.

### listInvoiceItems

Replace the `expectErr` for the 405 response with an assertion that the returned items
array matches the `items` array from the single-GET fixture (`single-get.json` shows
`items[0].unitPrice == 100000`, `items[0].quantity == 100`).

## Open questions

1. **status filter ignored:** `?status=PAID` returned all invoice statuses. Confirm
   whether Clockify supports invoice filtering at all, and if so what parameter name it
   uses. If not supported, `invoiceReport` in go-clockify may need to filter client-side.

2. **PROBE_LAST_STATUS not captured:** All `.status.txt` files contain `unknown`. This
   is a bug in `probes/lib/common.sh`: `PROBE_LAST_STATUS` is assigned inside a
   `$(...)` subshell (in `probe_curl`) but is read by `probe_save_fixture` in the parent
   shell. Fix: use a temp file to pass the status out of the subshell, or restructure
   `probe_curl` so it writes to a known temp path and the caller reads it.

3. **invoice_report separate endpoint?** The probe tested `GET /invoices?status=PAID`
   as the invoice_report path (per probe script comment). If the go-clockify handler
   uses a different URL for invoice_report (e.g. `GET /invoices/report`), that URL
   still needs to be probed. Confirm by reading `listInvoices` vs `invoiceReport` in
   `tier2_invoices.go`.

4. **Pagination for listInvoiceItems:** The single-GET route (`GET /invoices/{id}`)
   embeds items inline. Confirm there is no `page` parameter on the single-get route
   that could truncate items for invoices with large item counts.
