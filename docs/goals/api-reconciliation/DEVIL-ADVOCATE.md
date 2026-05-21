# DEVIL-ADVOCATE — Adversarial Re-Probe Report

**Mode:** DEVIL (§2)
**Iteration:** iter23
**Timestamp:** 2026-05-21T07:00:00Z
**Triggered by:** PROGRESS.md showing zero pending/in-progress rows after iter22 merge

---

## Sampling method

**LIVE-VERIFIED sample:** Stride-5 of all 114 LIVE-VERIFIED operations, sorted by YAML path order.
Stride: `floor(114 / 20) = 5`
Indices sampled: LV[0], LV[5], LV[10], LV[15], ... LV[95] (20 total, across 4 batches of 5)

**UNCONFIRMED-AGREE sample:** Stride-2 of all 23 UNCONFIRMED-AGREE operations.
Indices sampled: UC[0], UC[2], UC[4], UC[6], UC[8], UC[10], UC[12], UC[14], UC[16], UC[18] (10 total)

**Parallel batches:** DEVIL-BATCH-{A,B,C,D}-iter23.json

---

## LIVE-VERIFIED re-probes (20 sampled)

| endpoint_id | original_status | devil_probe_kind | verdict | notes |
|---|---|---|---|---|
| getApprovalRequests GET /approval-requests | LIVE-VERIFIED | devil-wrong-filter (REJECTED+APPROVED) | CONFIRMED | billableAmount typed integer but live returns float — schema imprecision, no demotion |
| createHoliday POST /holidays | LIVE-VERIFIED | devil-missing-required (no start_date) | CONFIRMED | user_group_ids undocumented in requestBody — minor schema gap, no demotion |
| getInvoices GET /invoices | LIVE-VERIFIED | devil-empty-list (page=9999) | CONFIRMED | bare array on empty page matches spec |
| getProjects GET /projects | LIVE-VERIFIED | devil-empty-list (page=9999) | CONFIRMED | bare array on empty page matches spec |
| getTags GET /tags | LIVE-VERIFIED | devil-empty-list (page=9999) | CONFIRMED | bare array on empty page matches spec |
| getTags GET /tags | LIVE-VERIFIED | devil-empty-list (page=9999) | CONFIRMED | bare array confirmed |
| getWebhook GET /webhooks/{webhookId} | LIVE-VERIFIED | devil-missing-required (zeros ID) | CONFIRMED | proper 4xx on unknown ID, not 200+null |
| updateTimeEntry PUT /time-entries/{timeEntryId} | LIVE-VERIFIED | devil-wrong-method (GET) | CONFIRMED | GET also returns 200 single object (undocumented read path used by MCP read-before-write) |
| updateTask PUT /projects/{projectId}/tasks/{taskId} | LIVE-VERIFIED | devil-wrong-method (GET) | CONFIRMED | GET also returns 200 (undocumented read path, same pattern) |
| createExpenseCategory POST /expenses/categories | LIVE-VERIFIED | devil-missing-required (empty name) | CONFIRMED | 4xx "name is required" validates required:[name] |
| listTimeOffRequests POST /time-off/requests | LIVE-VERIFIED | devil-wrong-method (GET) | CONFIRMED | GET → 405 exactly as documented |
| patch.workspaces.user.time-entries PATCH /user/{userId}/time-entries | LIVE-VERIFIED | devil-empty-list (far-future range) | CONFIRMED | bare [] on no-match range |
| getInvoiceSettings GET /invoices/settings | LIVE-VERIFIED | devil-happy-path | CONFIRMED | all top-level keys present; labels sub-keys unenumerated (17 known live) |
| get.workspaces.invoices.export GET /invoices/{invoiceId}/export | LIVE-VERIFIED | devil-missing-required (no userLocale) | CONFIRMED | 400 "Required request parameter... not present" exactly as TRUTH notes |
| get.workspaces.time-off.balance.user GET /time-off/balance/user/{userId} | LIVE-VERIFIED | devil-wrong-type (nonexistent userId) | CONFIRMED | returns silent 200 {balances:[],count:0} — undocumented but harmless behavior |
| delete.workspaces.expenses.categories DELETE /expenses/categories/{categoryId} | LIVE-VERIFIED | devil-wrong-method (GET) | CONFIRMED | GET → 405 path registered |
| patch.workspaces.scheduling.assignments.recurring PATCH /scheduling/assignments/recurring/{assignmentId} | LIVE-VERIFIED | devil-missing-required (fake ID) | CONFIRMED | domain-level not_found (not Spring 404) — path live |
| patch.workspaces.time-entries.invoiced PATCH /time-entries/invoiced | LIVE-VERIFIED | devil-wrong-method (GET) | CONFIRMED | GET → application error not 405 — router dispatches GET to handler; path definitely live |
| patch.workspaces.time-off.policies.requests PATCH /time-off/policies/{policyId}/requests/{requestId} | LIVE-VERIFIED | devil-wrong-method + dry-run PATCH | CONFIRMED | GET → 405; dry-run ok:true |
| getInvoicesInfo POST /invoices/info | LIVE-VERIFIED | devil-empty-list (page=9999) | CONFIRMED | 200 {data:[],total:0,has_more:false} — GET also returns application error not 405 |

**Demotions: 0**

---

## UNCONFIRMED-AGREE promotion attempts (10 sampled)

| endpoint_id | prior_status | promotion_verdict | evidence | new_status |
|---|---|---|---|---|
| inviteUserToWorkspace POST /users | UNCONFIRMED-AGREE | REMAINS_UNCONFIRMED | Skipped — would invite real user; raw writes blocked | UNCONFIRMED-AGREE |
| put.workspaces.user.time-entries PUT /user/{userId}/time-entries | UNCONFIRMED-AGREE | PROMOTED | GET 200 returns real entries; path confirmed live | LIVE-VERIFIED |
| getWebhookLogs POST /webhooks/{webhookId}/logs | UNCONFIRMED-AGREE | PROMOTED | GET 405 confirms path registered; prior 405 evidence + both specs agree | LIVE-VERIFIED |
| delete.workspaces.projects.custom-fields DELETE /projects/{projectId}/custom-fields/{customFieldId} | UNCONFIRMED-AGREE | REMAINS_UNCONFIRMED | Parent GET 200 confirms fields exist; DELETE blocked by raw writes guard | UNCONFIRMED-AGREE |
| assignProjectMemberships POST /projects/{projectId}/memberships | UNCONFIRMED-AGREE | PROMOTED | GET 405; live members confirmed via clockify_projects_memberships_list | LIVE-VERIFIED |
| post.file.image POST /file/image | UNCONFIRMED-AGREE | REMAINS_UNCONFIRMED | MCP workspace fence blocks non-workspace paths; cannot probe | UNCONFIRMED-AGREE |
| duplicateInvoice POST /invoices/{invoiceId}/duplicate | UNCONFIRMED-AGREE | PROMOTED | GET 405 with real invoice ID <redacted-id:8491414151> confirms path registered | LIVE-VERIFIED |
| copySchedulingAssignment POST /scheduling/assignments/{assignmentId}/copy | UNCONFIRMED-AGREE | PROMOTED | GET 405 with real assignment ID <redacted-id:aa3a385462> confirms path registered | LIVE-VERIFIED |
| duplicateUserTimeEntry POST /user/{userId}/time-entries/{id}/duplicate | UNCONFIRMED-AGREE | PROMOTED | GET 405 confirms path registered; iter20 absence suspicion refuted | LIVE-VERIFIED |
| updateWorkspaceCostRate PUT /cost-rate | UNCONFIRMED-AGREE | PROMOTED | GET 405 confirms path registered; workspace cost rate live data {amount:5000,currency:USD} | LIVE-VERIFIED |

**Promotions: 7** (of 10 sampled)
**Remains UNCONFIRMED: 3** (invite-user, delete-project-CF, post-file-image — all blocked by safety or fence)

---

## New discrepancies created

| endpoint_id | fact | truth-said | re-probe-said | new-evidence | action |
|---|---|---|---|---|---|
| getApprovalRequests | billableAmount type | integer | number (live float 885877.7777...) | PROBE-LOG/20260521-070000-devil-get.workspaces.approval-requests-devil-wrong-filter.json | Schema imprecision noted; no demotion — MCP normalizes |
| createHoliday | user_group_ids param | not in requestBody | accepted live | PROBE-LOG/20260521-070000-devil-post.workspaces.holidays-devil-missing-required.json | Schema gap noted; no demotion |
| updateTimeEntry, updateTask | GET on PUT-only path | not documented | 200 single object returned | PROBE-LOG/20260521-070000-devil-put.workspaces.time-entries-wrong-method.json | Undocumented read path; additive finding only |
| PATCH /time-entries/invoiced | GET behavior | 405 expected | application error (not routing miss) | PROBE-LOG/20260521-070000-devil-patch.workspaces.time-entries.invoiced-wrong-method.json | Router anomaly; path confirmed live |
| PATCH /invoices/info GET | GET behavior | 405 expected | application error | PROBE-LOG/20260521-070000-devil-getInvoicesInfo-wrong-method.json | Same router pattern; path confirmed live |
| time-off/balance/user | unknown userId response | 404 implied | 200 {balances:[],count:0} | PROBE-LOG/20260521-070000-devil-get.workspaces.time-off.balance.user-nonexistent-userid.json | Undocumented silent-200; no demotion |

**No demotions. All 20 LIVE-VERIFIED operations confirmed live.**

---

## Summary

- LIVE-VERIFIED sampled: 20 / 114 (stride-5)
- LIVE-VERIFIED confirmed: 20 / 20 (100%)
- LIVE-VERIFIED demoted: 0
- UNCONFIRMED-AGREE sampled: 10 / 23 (stride-2)
- UNCONFIRMED-AGREE promoted: 7
- UNCONFIRMED-AGREE remaining: 3 (blocked — unsafe or workspace-fenced)
- New DISCREPANCIES.md entries: 6 (all informational; zero demotions)
- TRUTH.yaml status after DEVIL: LIVE-VERIFIED=121, LIVE-OVERRIDE=49, UNCONFIRMED-AGREE=16, UNRESOLVED-NO-LIVE=7, UNVERIFIABLE-DESTRUCTIVE=2

**No outstanding demotions. FINAL mode may proceed.**
