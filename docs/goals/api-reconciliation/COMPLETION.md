# COMPLETION — API Reconciliation

**Status: COMPLETE, CONDITIONALLY USABLE**
**Final iteration:** 23 (DEVIL)

---

## Timestamps

| Event | Timestamp |
|---|---|
| Setup (iter1) | 2026-05-20T20:46:03Z |
| First WORK iteration (iter2) | 2026-05-20T21:02:26Z |
| Last WORK iteration (iter22) | 2026-05-21T06:00:00Z |
| DEVIL mode (iter23) | 2026-05-21T07:00:00Z |
| FINAL | 2026-05-21T07:30:00Z |

---

## Operation counts by verification status

| Status | Count | Meaning |
|---|---|---|
| LIVE-VERIFIED | 148 | Live probe confirmed + spec aligned |
| LIVE-OVERRIDE | 50 | Live disagreed with spec; live wins |
| UNCONFIRMED-AGREE | 1 | Specs agree or path is registered, but happy-path semantics are not live-proven |
| UNRESOLVED-NO-LIVE | 0 | No remaining unresolved operations |
| UNVERIFIABLE-DESTRUCTIVE | 1 | Not probed — destructive or billing-affecting |
| **TOTAL** | **200** | across 128 paths |

**Post-completion fixes (2026-05-21, after job handoff):**
- Fixed 42 OpenAPI schema validation errors (arrays missing `items`, parameters missing `schema`, ops missing `responses`, path template mismatches, dangling `#/components/schemas/*` refs to non-existent components section). TRUTH.openapi.yaml now passes `openapi-spec-validator` and `@apidevtools/swagger-cli validate`.
- Promoted `GET /workspaces/{workspaceId}/users` from UNRESOLVED-NO-LIVE → LIVE-VERIFIED by probing via direct curl (bypassing the 50KB MCP tool-result cap). Bare-array shape confirmed; pagination IS respected (one user record is just large due to embedded memberships).
- **Major correction — shared-reports family**: the original job declared the entire `/shared-reports*` family "phantom" because probes returned 404. The probes were on the wrong host. Shared reports live on `reports.api.clockify.me/v1/`, not `api.clockify.me/api/v1/`. Re-probed all 5 ops on the correct host: all LIVE-VERIFIED.
  - `GET /shared-reports/{id}` (reports host, NOT workspace-scoped) — was UNRESOLVED → LIVE-VERIFIED
  - `GET /workspaces/{workspaceId}/shared-reports` — was LIVE-OVERRIDE (phantom) → LIVE-VERIFIED; response is `{reports:[...], count:N}` wrapper, NOT bare array (both specs were wrong)
  - `POST /workspaces/{workspaceId}/shared-reports` — was LIVE-OVERRIDE → LIVE-VERIFIED; required: `filter.dateRangeStart`, `filter.dateRangeEnd`, and `filter.summaryFilter` when `type=SUMMARY`
  - `PUT /workspaces/{workspaceId}/shared-reports/{id}` — was LIVE-OVERRIDE → LIVE-VERIFIED
  - `DELETE /workspaces/{workspaceId}/shared-reports/{id}` — was UNRESOLVED → LIVE-VERIFIED (returns 204)
  - Consolidated duplicate `{sharedReportId}` alias path into `{id}` (per official). Path count: 129 → 128.
  - Added top-level `servers:` block (main + reports hosts) and per-path `servers:` override for all shared-reports operations.
- Codex stabilization pass corrected stale tallies from 195 to 197 operations, resolved all missing `x-evidence` file references, demoted seven path-registered but happy-path-unproven operations from LIVE-VERIFIED to UNCONFIRMED-AGREE, repaired malformed/empty probe JSON, and sanitized raw live identifiers/emails from the untracked evidence pack.
- Codex narrow reconciliation pass added three source-covered operations that were missing from `TRUTH.openapi.yaml`: `GET /workspaces/{workspaceId}/clients/{clientId}` (LIVE-VERIFIED from existing GET lifecycle evidence), `DELETE /workspaces/{workspaceId}/user/{userId}/time-entries` (UNRESOLVED-NO-LIVE; bulk raw DELETE blocked), and `PUT /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}` (UNCONFIRMED-AGREE; PATCH is live-proven but PUT is not).
- Codex live workspace closure pass resolved the last two UNRESOLVED-NO-LIVE rows and direct-probed the scheduling PUT method: `GET /workspaces/{workspaceId}/templates/{templateId}` is LIVE-OVERRIDE (real project-template candidates return HTTP 400 code 501 on the workspace-level route), `DELETE /workspaces/{workspaceId}/user/{userId}/time-entries` is LIVE-VERIFIED (HTTP 200 bare array; follow-up GET no longer reads the entry), and `PUT /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}` is LIVE-OVERRIDE (HTTP 405; PATCH is the supported update method).
- Codex remaining-gap closure pass live-probed 13 safe/fixture-backed UNCONFIRMED-AGREE operations and promoted all 13 to LIVE-VERIFIED: webhook create/logs, client update, user-scoped bulk time-entry PUT, user-scoped time-entry duplicate, project custom-field PATCH/DELETE, project memberships POST, invoice duplicate, project-from-template, scheduling assignment copy, scheduling series PUT, and users/info POST. Disposable projects, clients, entries, assignments, custom fields, invoices, and webhooks were cleaned up; prefix leftover checks returned zero matches.
- Codex final authorized sacrificial-workspace pass promoted 9 more rows to LIVE-VERIFIED: time-off balance PATCH, user-scoped time-off request POST, user-scoped approval resubmit POST, image upload, workspace cost rate PUT, user cost/hourly rate PUT, user custom-field value PUT, and role grant POST. `DELETE /workspaces/{workspaceId}/users/{userId}` moved to LIVE-OVERRIDE: a non-self test fixture could be deactivated, but live DELETE returned a Cake-migration removal error. The invite POST remains UNCONFIRMED-AGREE because live rejected disposable invites with a subscription seat-limit error.
- Codex lint-clean pass removed the remaining Redocly warnings without changing operation/status counts: added license metadata, generic `400` responses to success-response operations, and a narrow Redocly policy for LIVE-OVERRIDE non-2xx rows plus source-covered ambiguous Clockify paths.


---

## UNRESOLVED-NO-LIVE operations

None remain after the Codex live workspace closure pass.

---

## UNVERIFIABLE-DESTRUCTIVE operations

| Method | Path | OperationId | Reason |
|---|---|---|---|
| POST | /workspaces | post.workspaces | Creating a workspace may have billing/subscription implications |

---

## Probe log statistics

- Total probe-log files: 435
- JSON probe receipts: 442
- Total size: 3.9 MB
- Location: `docs/goals/api-reconciliation/PROBE-LOG/`
- Iterations: 22 WORK iterations + 1 DEVIL iteration = 23 total

---

## Key discrepancies discovered

Major findings from the reconciliation (full details in DISCREPANCIES.md):

1. **GET /workspaces/{wId}/time-entries returns 405** — both specs say GET is supported; live says use GET /user/{userId}/time-entries instead
2. **GET /workspaces/{wId}/time-off/requests returns 405** — listing requires POST body filter
3. **expenses response shape** — live returns `{dailyTotals,expenses:{count,expenses:[]},weeklyTotals}` object; both specs say bare array
4. **GET /user-groups/{groupId} returns 405** — single-item GET absent; list only
5. **webhooks response shape** — live returns `{webhooks:[],workspaceWebhookCount:N}` wrapper; both specs say bare array
6. **expenses/categories response shape** — live returns `{categories:[],count:N}` wrapper; both specs say bare array
7. **pagination ignored** — custom-fields, holidays, webhooks, custom-fields ignore page-size and return all records
8. **entities.created requires `type` query param** — documented as optional; live requires it
9. **scheduling assignments require `start` param** — documented as optional; live requires it
10. **~15 phantom paths** — policies/*, templates/*, shared-reports/*, balance, various archive/cost-rate sub-paths absent from live API
11. **clockify_users_role uses POST not PATCH** — CLAUDE.md API note was wrong; code inspection confirmed POST (s.Client.Post at user_admin.go:572)
12. **PUT and PATCH coexist at /time-off/policies/{policyId}** — PUT=field update, PATCH=status toggle; iter15 mislabeled
13. **scheduling assignments: bare path absent; recurring path is real** — /scheduling/assignments/{id} 404; correct is /scheduling/assignments/recurring/{id}
14. **webhooks/logs uses POST not GET** — GET returns 405; POST is correct per both specs and code
15. **DEVIL found**: PUT paths for time-entries and tasks also accept GET (undocumented read used by MCP read-before-write); multiple paths return application error on GET instead of 405 (router anomaly)

---

## DEVIL-ADVOCATE results

- 20 LIVE-VERIFIED re-probed: 20/20 confirmed (0 demotions)
- 10 UNCONFIRMED-AGREE sampled for promotion: 7 promoted, 3 remain (blocked)
- No outstanding demotions
- 6 new informational discrepancy entries (schema imprecisions, undocumented behaviors)

---

## Did we reach "100% truthful"?

**Honest answer: conditionally usable, not 100% live-proven.**

- 148/200 ops (74%) are LIVE-VERIFIED with direct probe evidence.
- 50/200 ops (25%) are LIVE-OVERRIDE — live behavior contradicted one or both source specs.
- 1/200 ops (0.5%) is UNCONFIRMED-AGREE — both specs agree or the route is registered, but safe happy-path proof is missing.
- 0/200 ops (0%) are UNRESOLVED-NO-LIVE.
- 1/200 ops (0.5%) is UNVERIFIABLE-DESTRUCTIVE — deliberately skipped.

**What's left:** Treat LIVE-VERIFIED and LIVE-OVERRIDE as the only hard drift assertions. Keep UNCONFIRMED-AGREE and UNVERIFIABLE-DESTRUCTIVE out of hard truth checks unless fresh live probes promote them. Further work is needed only if:
- A workspace with available subscription seats enables a happy-path `POST /workspaces/{workspaceId}/users` invite.
- The operator explicitly accepts creating an account-level workspace despite no source-covered cleanup route for `POST /workspaces`.

**Next:** The `TRUTH.openapi.yaml` at 128 paths / 200 ops is structurally valid and Redocly-clean. It can seed nightly drift detection as a status-partitioned baseline, not as a blanket "all operations are live-proven" oracle.
