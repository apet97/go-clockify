# Finding: live straggler write+read promotions (2026-06-23, round 2)

Captured live this session against the sacrificial sandbox
(`65b382b606de527a7ee2b60e`) with an API key, via a `fetch` probe harness.
These are the "route exists, needs body/perm" stragglers from the
2026-06-23 surface audit, now probed with the correct request shapes
extracted from the fresh official OpenAPI pull. Each row is a clean
canonical path so the generator's `normalize_path` matches the merged
operation key and `status_bucket` flips the op to `live-success`.

Cleanup discipline (Leftovers:0): every created entity was prefixed
`sdk-live-probe-` and removed at teardown and re-verified absent —
the probe group was created+deleted, the approval request was
submitted then withdrawn (PATCH state WITHDRAWN_SUBMISSION), the
invoice payment was created then deleted, the webhook was created then
deleted. The two detailed-report POSTs are read-only (no residue).
Fixtures are documentary.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/webhooks/{webhookId}/statuses | 200 | live-probe 2026-06-23 (documentary) |
| POST | api.clockify.me | /workspaces/{workspaceId}/user-groups/{groupId}/users | 200 | live-probe 2026-06-23 (documentary) |
| POST | reports.api.clockify.me | /workspaces/{workspaceId}/reports/expenses/detailed | 200 | live-probe 2026-06-23 (documentary) |
| POST | api.clockify.me | /workspaces/{workspaceId}/approval-requests | 201 | live-probe 2026-06-23 (documentary) |
| PATCH | api.clockify.me | /workspaces/{workspaceId}/approval-requests/{approvalRequestId} | 200 | live-probe 2026-06-23 (documentary) |
| POST | api.clockify.me | /workspaces/{workspaceId}/invoices/{invoiceId}/payments | 201 | live-probe 2026-06-23 (documentary) |

## Probed but NOT promoted (route confirmed live; not a clean 2xx in this sandbox)

| Op | Method/Path | Live result | Why not promoted |
|---|---|---|---|
| createTimeOffRequest / createTimeOffRequestForUser / changeTimeOffRequestStatus | POST/PATCH /time-off/policies/{policyId}[/users/{userId}]/requests[/{requestId}] | 400 "Requested start or end date are non-working days" | The sandbox DAYS policy has no working days configured, so every date 400s on the business rule. Route + period shape confirmed valid; not a 2xx in **this** workspace state. **NOTE:** `createTimeOffRequest` (POST .../time-off/policies/{policyId}/requests) is ALREADY stamped `live-success` from a prior clean-2xx wave against a properly-configured policy — this session's 400 is a sandbox-policy-config artifact and does **not** demote the standing stamp (it is not among this round's +6 promotions). A REJECTED request is also terminal (undeletable), so a `changeTimeOffRequestStatus` probe would leave residue. |
| importInvoiceItems | POST /invoices/{invoiceId}/items/import | 400 progressive field validation (projectFilter.status, then projectFilter.contains required) | Route confirmed (walks required nested projectFilter fields). Needs a payable invoice with importable entries; importing mutates the invoice with non-trivial cleanup. Left documented. |
| updateUserCustomFieldValue | PUT /users/{userId}/custom-field/{customFieldId}/value | 404 | No USER-entity custom field exists in the sandbox (all custom fields are TIMEENTRY-scoped). Path shape (singular `/custom-field/`, trailing `/value`) confirmed; no target to write. |
| createProjectFromTemplate | POST /projects/from-template | 403 "You don't have permission to perform this action." | Permission-gated (project-template is a paid feature; the API key lacks the grant). Unpromotable for this key. |
| updateUserStatus | PUT /users/{userId} | 400 "You're trying to add more users than your subscription allows." | Subscription seat-limit gated on the workspace-user PUT. Unpromotable for this key/plan. |

## Webhook-create `name` requiredness — live observation (Goal 3)

Live A/B on the API-key path against the sandbox, isolating `name` on
otherwise-identical valid configs (untouched webhook events, to avoid
Clockify's duplicate-detection):

- POST /workspaces/{wsId}/webhooks WITH a 2–30 char `name` → **201 Created**.
- Same body with `name` OMITTED → **400** (`{"message":"Webhook already
  exists","code":501}` — a misleading generic rejection, but unambiguously
  a 400, no webhook created).
- Same body with `name: ""` (empty) → **400** `"length must be between 2
  and 30"` — the clean validation message confirming the 2–30 constraint.

So on the API-key (user) creation path this SDK family uses, `name` is
REQUIRED (omitting or empty → 400; a valid 2–30 name → 201). This
converts the prior maintainer-confirmed inference in
`clockify-ts-sdk` `webhook.create.name-required-on-api-key-not-addon`
to a live observation. The fresh OFFICIAL spec still marks `name`
OPTIONAL (addon-token path), consistent with the auth-scheme-dependent
nuance. Documentary; no spec change to the webhook-create op required.
