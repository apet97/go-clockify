# Report for Codex — Post-Reconciliation Audit

**Author:** Claude Opus 4.7 (handoff session)
**Date:** 2026-05-21
**Scope:** Post-completion review of Sonnet's 23-iteration adversarial API reconciliation in `docs/goals/api-reconciliation/`.
**Subject under review:** `TRUTH.openapi.yaml` (the reconciled spec) and the surrounding evidence files.

---

## 1. What Sonnet delivered (before this session)

23-iteration adversarial reconciliation of three sources (official Clockify OpenAPI, GOCLMCP generated OpenAPI, live probes) against the sacrificial workspace. Produced:

- `TRUTH.openapi.yaml` — 129 paths, 195 operations, every op tagged with `x-verification-status` ∈ {LIVE-VERIFIED, LIVE-OVERRIDE, UNCONFIRMED-AGREE, UNRESOLVED-NO-LIVE, UNVERIFIABLE-DESTRUCTIVE}
- `PROBE-LOG/` — 274 raw probe responses (~1.4 MB)
- `DISCREPANCIES.md`, `DEVIL-ADVOCATE.md`, `COMPLETION.md`, `PROGRESS.md`
- A DEVIL'S ADVOCATE re-probe pass (20 LV + 10 UA samples; 7 promotions, 0 demotions)

**Sonnet's tallies on handoff:**
- 121 LIVE-VERIFIED, 49 LIVE-OVERRIDE, 16 UNCONFIRMED-AGREE, 7 UNRESOLVED-NO-LIVE, 2 UNVERIFIABLE-DESTRUCTIVE = 195 ops / 129 paths.

---

## 2. Defects this session found and fixed

### 2.1 OpenAPI schema validity (42 errors → 0)

`TRUTH.openapi.yaml` did not pass any OpenAPI 3.0 validator. Defect classes:

| # | Class | Count | Fix |
|---|---|---|---|
| 1 | `type: array` declarations without `items` | 10 | Added `items: {type: object}` |
| 2 | Path/query parameters missing `schema` or `content` | 28 | Added `schema: {type: string}` |
| 3 | Operations missing `responses:` | 2 | Added `'200': {description: OK}` |
| 4 | Path template parameter missing from `parameters` (`POST /approval-requests` missing `workspaceId`) | 1 | Added |
| 5 | Path parameter not in template (`PATCH …/requests/{requestId}/status` had stray `policyId`) | 1 | Removed |
| 6 | Dangling `$ref: '#/components/schemas/ProjectObject'` and `TimeOffBalance` (no `components` section ever existed) | 2 | Inlined as `type: object` |

**Validation status after fix:**
```
openapi-spec-validator TRUTH.openapi.yaml: OK
npx @apidevtools/swagger-cli validate TRUTH.openapi.yaml: TRUTH.openapi.yaml is valid
```

### 2.2 `GET /workspaces/{workspaceId}/users` promoted

UNRESOLVED-NO-LIVE → LIVE-VERIFIED. The MCP tool blocked this with its 50 KB result cap; direct `curl` confirmed HTTP 200, bare array response (consistent with both specs), pagination respected. Per-user payload is large (~70 KB for 1 user) due to embedded memberships. Real response schema added.

Probe: `PROBE-LOG/20260521-020846-get.workspaces.users-direct-curl.json`

### 2.3 Shared-reports family — major correction (user-flagged)

**Sonnet's verdict:** Entire `/shared-reports*` family is "phantom" — Spring-MVC 404 on every probe.

**Reality:** All 5 ops are real and routed, **but on a different host** (`reports.api.clockify.me/v1/`, not `api.clockify.me/api/v1/`). Sonnet never probed the reports host.

Action taken: live-probed CRUD on the reports host against the sacrificial workspace (created a test report, updated it, deleted it). All 5 ops returned HTTP 200/204 with documented bodies.

**Spec updates:**
- Added top-level `servers:` block (was missing entirely — the spec had a *comment* claiming `api.clockify.me/api/v1` as base but no machine-readable declaration).
- Added per-path `servers:` override on all `/shared-reports*` paths pointing at the reports host.
- Consolidated duplicate alias paths (`{sharedReportId}` and `{id}`) into a single `{id}` (per the user's official spec). Net: 129 paths → 128.
- Rewrote 5 operations with verified shapes (no more "PHANTOM PATH" markers):
  - `GET /shared-reports/{id}` (not workspace-scoped): response `{totals, donutChart, groupTotals, groupOne, filters{…}}` — was UNRESOLVED
  - `GET /workspaces/{wId}/shared-reports`: response `{reports:[…], count}` wrapper — was LIVE-OVERRIDE; **both specs incorrectly say bare array**
  - `POST /workspaces/{wId}/shared-reports`: required body fields confirmed: `name`, `type`, `filter.dateRangeStart`, `filter.dateRangeEnd`, plus `filter.summaryFilter` when `type=SUMMARY` (returns 501 "Please provide summary filter" otherwise) — was LIVE-OVERRIDE
  - `PUT /workspaces/{wId}/shared-reports/{id}`: full report object response — was LIVE-OVERRIDE
  - `DELETE /workspaces/{wId}/shared-reports/{id}`: 204 no-body — was UNRESOLVED

Probes:
```
PROBE-LOG/20260521-021218-get.shared-reports-list-reports-host.json
PROBE-LOG/20260521-021218-get.shared-reports-list-createdbyme.json
PROBE-LOG/20260521-021234-get.shared-reports-by-id-reports-host.json
PROBE-LOG/20260521-021310-post.shared-reports-create.json
PROBE-LOG/20260521-021310-put.shared-reports-update.json
PROBE-LOG/20260521-021311-delete.shared-reports.json
```

### 2.4 Cross-checks against user-provided official specs

The user supplied four canonical OpenAPI files from a separate "real OpenAPI" stash:
- `BALANCEOPEANI.yaml` — covers `/time-off/balance/policy/{policyId}`, `/time-off/balance/user/{userId}`
- `HOLIDAYSOPEAPI.YAML` — `/holidays`, `/holidays/in-period`, `/holidays/{holidayId}`
- `POLICIESOPENAPI.YAML` — `/time-off/policies`, `/time-off/policies/{id}` (incl. PATCH/PUT/DELETE)
- `TIMEOFFOPENAPI.YAML` — `/time-off/policies/{policyId}/requests`, `…/{policyId}/users/{userId}/requests`, `/time-off/requests`

All of these are already present in `TRUTH.openapi.yaml` with the correct `/time-off/` prefix and are LIVE-VERIFIED (or LIVE-OVERRIDE for known-stale generated-spec aliases like bare `/policies` and bare `/balance` paths — those are correctly flagged because GOCLMCP's generated spec emitted them without the prefix).

---

## 3. Final tallies (post this session)

Superseded by Codex stabilization, narrow reconciliation, and remaining-gap
closure on 2026-05-21: the current `TRUTH.openapi.yaml` has 128 paths / 200
operations. The safe live pass promoted 13 fixture-backed operations from
UNCONFIRMED-AGREE to LIVE-VERIFIED. A later authorized sacrificial-workspace
pass promoted 9 more rows and moved `DELETE /users/{userId}` to LIVE-OVERRIDE
after Clockify returned a Cake-migration removal error.

| Status | Before | After | Δ |
|---|---|---|---|
| LIVE-VERIFIED | 126 | **148** | +22 |
| LIVE-OVERRIDE | 49 | **50** | +1 |
| UNCONFIRMED-AGREE | 23 | **1** | -22 |
| UNRESOLVED-NO-LIVE | 0 | **0** | 0 |
| UNVERIFIABLE-DESTRUCTIVE | 2 | **1** | -1 |
| **TOTAL** | 200 / 128 paths | **200 / 128 paths** | 0 |

Validator status: **OK** (`openapi-spec-validator`, `@apidevtools/swagger-cli validate`, and `@redocly/cli lint --max-problems 1000` all pass with no warnings/errors under the reconciliation Redocly policy).

---

## 4. Remaining UNRESOLVED-NO-LIVE (0)

None remain. Follow-up live receipts closed both rows:

| Method | Path | Result | Evidence |
|---|---|---|---|
| `GET` | `/workspaces/{wId}/templates/{templateId}` | LIVE-OVERRIDE: real project-template candidates return HTTP 400 code 501 on the workspace-level route | `PROBE-LOG/20260521T194101Z-codex-get-workspaces-templates-single-live.json` |
| `DELETE` | `/workspaces/{wId}/user/{userId}/time-entries` | LIVE-VERIFIED: disposable entry bulk delete returned HTTP 200 bare array, and follow-up GET no longer read the entry | `PROBE-LOG/20260521T194802Z-codex-delete-workspaces-user-time-entries-live-reprobe3.json` |
| `PUT` | `/workspaces/{wId}/scheduling/assignments/recurring/{assignmentId}` | LIVE-OVERRIDE: direct PUT against a freshly-created assignment returned HTTP 405; PATCH remains the supported update method | `PROBE-LOG/20260521T194802Z-codex-put-workspaces-scheduling-assignments-recurring-live-reprobe3.json` |

---

## 5. UNCONFIRMED-AGREE (1)

Codex remaining-gap closure on 2026-05-21 promoted 22 fixture-backed rows from
UNCONFIRMED-AGREE to LIVE-VERIFIED across the safe and final authorized live
passes. The remaining UNCONFIRMED-AGREE row is `POST /workspaces/{workspaceId}/users`:
disposable `send-email=false` invites were attempted, but the sacrificial workspace
is at its subscription seat limit. Do not treat it as a hard live-truth assertion
until a workspace with available seats produces a happy-path invite receipt.

UNVERIFIABLE-DESTRUCTIVE (1): `POST /workspaces` remains unprobed because it creates
an account-level workspace outside the configured sacrificial workspace and the
official source snapshot exposes no cleanup route.

---

## 6. Files changed this session

```
M docs/goals/api-reconciliation/TRUTH.openapi.yaml          (42 lint fixes + 5 shared-reports ops rewritten + servers block + 1 path consolidated)
M docs/goals/api-reconciliation/COMPLETION.md               (status tallies + post-completion notes)
+ docs/goals/api-reconciliation/REPORT-FOR-CODEX.md         (this file)
M docs/goals/api-reconciliation/PROGRESS.md                 (live closure rows)
M docs/goals/api-reconciliation/DISCREPANCIES.md            (template single + scheduling PUT live override)
M docs/goals/api-reconciliation/CODEX-VERIFICATION-RESULTS.md (final counts and unresolved closure)
M docs/goals/api-reconciliation/TRUTH.openapi.yaml          (lint-clean metadata + generic 400 responses; counts unchanged)
+ redocly.yaml                                               (repo-root reconciliation Redocly policy)
+ docs/goals/api-reconciliation/redocly.yaml                 (same policy for directory-local documented commands)
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-020846-get.workspaces.users-direct-curl.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021218-get.shared-reports-list-reports-host.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021218-get.shared-reports-list-createdbyme.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021234-get.shared-reports-by-id-reports-host.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021243-post.shared-reports-create.json   (400 response — missing dateRange)
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021257-post.shared-reports-create.json   (400 response — missing summaryFilter)
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021310-post.shared-reports-create.json   (200 happy path)
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021310-put.shared-reports-update.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521-021311-delete.shared-reports.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194101Z-codex-delete-workspaces-user-time-entries-live.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194101Z-codex-get-workspaces-templates-single-live.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194101Z-codex-put-workspaces-scheduling-assignments-recurring-live.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194101Z-codex-rest-cleanup-summary.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194255Z-codex-delete-workspaces-user-time-entries-live-reprobe.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194255Z-codex-put-workspaces-scheduling-assignments-recurring-live-reprobe.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194255Z-codex-rest2-cleanup-summary.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194802Z-codex-delete-workspaces-user-time-entries-live-reprobe3.json
+ docs/goals/api-reconciliation/PROBE-LOG/20260521T194802Z-codex-put-workspaces-scheduling-assignments-recurring-live-reprobe3.json
```

Nothing committed. Repo policy (`AGENTS.md`, `CLAUDE.md`) requires user-initiated commits.

---

## 7. Open questions / known limitations

- The spec describes **subdomain split** only by per-path `servers:` overrides. Tooling that flattens server URLs from the top-level array will get the wrong host for shared-reports. Downstream consumers should respect path-level `servers:`.
- Probed Clockify subdomains that returned non-API responses (likely SPA HTML): `time-off.api.clockify.me`, `invoice.api.clockify.me`, `invoices.api.clockify.me`. `pto.api.clockify.me` returned a valid Spring 404 (looks like an internal microservice) and `/policies` on that host returned 200 — but this is NOT the public API path (the public path is `/time-off/policies` on the main host). The pto subdomain is likely internal; do not document it as public.
- Some response schemas are still permissively typed (`type: object` with no properties). Tightening these is a separate exercise.
