# PROGRESS — API Reconciliation

NOTE: all paths normalized by stripping /v1/ prefix (official spec has no server URL and embeds /v1/ in paths; our spec uses https://api.clockify.me/api/v1 as base)

| endpoint_id | method | normalized_path | source | status | last_probe_at | notes |
|---|---|---|---|---|---|---|
| delete.workspaces.clients | DELETE | /workspaces/{workspaceId}/clients/{clientId} | ours | probed | 20260521-001500 | iter11 LIVE-VERIFIED; deleted test client |
| delete.workspaces.clients | DELETE | /workspaces/{workspaceId}/clients/{id} | official | probed | 20260521-002300 | iter12 LIVE-VERIFIED; alias for {clientId} iter11 |
| delete.workspaces.custom-fields | DELETE | /workspaces/{workspaceId}/custom-fields/{customFieldId} | both | probed | 20260521-002300 | iter12 LIVE-VERIFIED |
| delete.workspaces.expenses | DELETE | /workspaces/{workspaceId}/expenses/{expenseId} | both | probed | 20260521-001500 | iter11 LIVE-VERIFIED; deleted test expense |
| delete.workspaces.expenses.categories | DELETE | /workspaces/{workspaceId}/expenses/categories/{categoryId} | both | probed | 20260521-002300 | iter12 LIVE-VERIFIED; archived+deleted |
| delete.workspaces.holidays | DELETE | /workspaces/{workspaceId}/holidays/{holidayId} | both | probed | 20260521-002300 | iter12 LIVE-VERIFIED |
| delete.workspaces.invoices | DELETE | /workspaces/{workspaceId}/invoices/{invoiceId} | both | probed | 20260521-002300 | iter12 LIVE-VERIFIED |
| delete.workspaces.invoices.items | DELETE | /workspaces/{workspaceId}/invoices/{invoiceId}/items/{order} | both | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.invoices.payments | DELETE | /workspaces/{workspaceId}/invoices/{invoiceId}/payments/{paymentId} | both | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.policies | DELETE | /workspaces/{workspaceId}/policies/{policyId} | ours | probed | 20260521-003100 | iter13 LIVE-OVERRIDE; Spring-MVC 404 inferred from GET pattern |
| delete.workspaces.policies.requests | DELETE | /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId} | ours | probed | 20260521-003100 | iter13 LIVE-OVERRIDE; parent path absent |
| delete.workspaces.projects | DELETE | /workspaces/{workspaceId}/projects/{projectId} | both | probed | 20260521-003100 | iter13 LIVE-VERIFIED; {deleted:true,projectId} |
| delete.workspaces.projects.custom-fields | DELETE | /workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId} | both | probed | 20260521T200917Z | LIVE-VERIFIED; project-specific default removal returned HTTP 200; workspace-visible fields return HTTP 400 |
| delete.workspaces.projects.tasks | DELETE | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | both | probed | 20260521-003100 | iter13 LIVE-VERIFIED; {deleted:true,taskId} |
| delete.workspaces.scheduling.assignments | DELETE | /workspaces/{workspaceId}/scheduling/assignments/{assignmentId} | ours | probed | 20260521-003100 | iter13 LIVE-OVERRIDE; bare path 404; recurring path exists |
| delete.workspaces.scheduling.assignments.recurring | DELETE | /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId} | both | probed | 20260521-004000 | iter14 |
| delete.workspaces.shared-reports | DELETE | /workspaces/{workspaceId}/shared-reports/{id} | official | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.shared-reports | DELETE | /workspaces/{workspaceId}/shared-reports/{sharedReportId} | ours | probed | | |
| delete.workspaces.tags | DELETE | /workspaces/{workspaceId}/tags/{tagId} | ours | probed | 20260521-004000 | iter14 |
| delete.workspaces.tags | DELETE | /workspaces/{workspaceId}/tags/{id} | official | probed | 20260521-004000 | iter14 LIVE-VERIFIED; official {id} alias for ours {tagId}, same probe |
| delete.workspaces.templates | DELETE | /workspaces/{workspaceId}/templates/{templateId} | official | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.time-entries | DELETE | /workspaces/{workspaceId}/time-entries/{id} | official | probed | 20260521-004000 | iter14 |
| delete.workspaces.time-entries | DELETE | /workspaces/{workspaceId}/time-entries/{timeEntryId} | ours | probed | 20260521-004000 | iter14 LIVE-VERIFIED; ours {timeEntryId} alias for official {id}, same probe |
| delete.workspaces.time-off.policies | DELETE | /workspaces/{workspaceId}/time-off/policies/{id} | official | probed | 20260521-010000 | iter17 UNRESOLVED-NO-LIVE; path family confirmed, raw DELETE blocked; {id}={policyId} alias |
| delete.workspaces.time-off.policies | DELETE | /workspaces/{workspaceId}/time-off/policies/{policyId} | ours | probed | 20260521-010000 | iter17 UNRESOLVED-NO-LIVE; same probe as official row |
| delete.workspaces.time-off.policies.requests | DELETE | /workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId} | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; {deleted:true} confirmed |
| delete.workspaces.time-off.requests | DELETE | /workspaces/{workspaceId}/time-off/requests/{requestId} | ours | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.user-groups | DELETE | /workspaces/{workspaceId}/user-groups/{groupId} | ours | probed | 20260521-004000 | iter14 |
| delete.workspaces.user-groups | DELETE | /workspaces/{workspaceId}/user-groups/{id} | official | probed | 20260521-004000 | iter14 LIVE-VERIFIED; official {id} alias for ours {groupId}, same probe |
| delete.workspaces.user-groups.users | DELETE | /workspaces/{workspaceId}/user-groups/{groupId}/users/{userId} | ours | probed | 20260521-PARALLEL | iter15-parallel |
| delete.workspaces.user-groups.users | DELETE | /workspaces/{workspaceId}/user-groups/{userGroupId}/users/{userId} | official | probed | | |
| delete.workspaces.user.time-entries | DELETE | /workspaces/{workspaceId}/user/{userId}/time-entries | both | probed | 20260521T194802Z | iter26 LIVE-VERIFIED; disposable entry bulk DELETE returned HTTP 200 bare array and follow-up GET no longer read entry |
| delete.workspaces.users | DELETE | /workspaces/{workspaceId}/users/{userId} | official | probed | 20260521T203625Z | LIVE-OVERRIDE; non-self test fixture deactivated with PUT 200, then DELETE returned HTTP 400 Cake-migration removal error |
| delete.workspaces.users.roles | DELETE | /workspaces/{workspaceId}/users/{userId}/roles | both | probed | 20260521-004500 | iter16 LIVE-OVERRIDE; spec says DELETE, tool uses PATCH |
| delete.workspaces.webhooks | DELETE | /workspaces/{workspaceId}/webhooks/{webhookId} | both | probed | 20260521-004000 | iter14 |
| get.shared-reports | GET | /shared-reports/{sharedReportId} | ours | probed | 20260520-204725 | |
| get.shared-reports | GET | /shared-reports/{id} | official | probed | 20260520-204725 | |
| get.user | GET | /user | both | probed | 20260520-204725 | |
| get.workspaces | GET | /workspaces | both | probed | 20260520-204725 | |
| get.workspaces | GET | /workspaces/{workspaceId} | both | probed | 20260520-204725 | |
| get.workspaces.addons.webhooks | GET | /workspaces/{workspaceId}/addons/{addonId}/webhooks | both | probed | 20260520-204725 | |
| get.workspaces.approval-requests | GET | /workspaces/{workspaceId}/approval-requests | both | probed | 20260520-210956 | bare array of approval-period objects with inline entries/expenses |
| get.workspaces.balance | GET | /workspaces/{workspaceId}/balance | ours | probed | 20260520-221429 | iter7 |
| get.workspaces.clients | GET | /workspaces/{workspaceId}/clients/{clientId} | ours | probed | 20260520-204725 | LIVE-VERIFIED; existing receipt explicitly records iter11 client lifecycle GET before delete |
| get.workspaces.clients | GET | /workspaces/{workspaceId}/clients | both | probed | 20260520-204725 | |
| get.workspaces.clients | GET | /workspaces/{workspaceId}/clients/{id} | official | probed | 20260520-204725 | |
| get.workspaces.custom-fields | GET | /workspaces/{workspaceId}/custom-fields | both | probed | 20260520-210956 | page-size ignored; all returned; DISCREPANCY logged |
| get.workspaces.entities.created | GET | /workspaces/{workspaceId}/entities/created | both | probed | 20260520-212439 | LIVE-OVERRIDE: required type param missing from both specs; page-size ignored; response adds auditMetadata+documentCode |
| get.workspaces.entities.deleted | GET | /workspaces/{workspaceId}/entities/deleted | both | probed | 20260520-221429 | iter7 |
| get.workspaces.entities.updated | GET | /workspaces/{workspaceId}/entities/updated | both | probed | 20260520-221429 | iter7 |
| get.workspaces.expenses | GET | /workspaces/{workspaceId}/expenses/{expenseId} | both | probed | 20260520-210956 | flat shape (categoryId not nested category) |
| get.workspaces.expenses | GET | /workspaces/{workspaceId}/expenses | both | probed | 20260520-210956 | LIVE-OVERRIDE: response is wrapped object not array; DISCREPANCY logged |
| get.workspaces.expenses.categories | GET | /workspaces/{workspaceId}/expenses/categories | both | probed | 20260520-214436 | LIVE-OVERRIDE: wrapped {categories:[], count:N}; page-size respected; single GET 405 |
| get.workspaces.expenses.files | GET | /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId} | both | probed | 20260521-000000 | iter9 UNRESOLVED-NO-LIVE: endpoint exists (403) but no expenses with files to test |
| get.workspaces.holidays | GET | /workspaces/{workspaceId}/holidays | both | probed | 20260520-210956 | page-size ignored; all returned; dates are YYYY-MM-DD strings |
| get.workspaces.holidays.in-period | GET | /workspaces/{workspaceId}/holidays/in-period | both | probed | 20260520-221429 | iter7 |
| get.workspaces.invoices | GET | /workspaces/{workspaceId}/invoices/{invoiceId} | both | probed | 20260520-210956 | richer than list: includes items/taxes/billing metadata |
| get.workspaces.invoices | GET | /workspaces/{workspaceId}/invoices | both | probed | 20260520-210956 | wrapped {invoices:[], total:N} not bare array |
| get.workspaces.invoices.export | GET | /workspaces/{workspaceId}/invoices/{invoiceId}/export | both | probed | 20260521-000000 | iter9 LIVE-VERIFIED: binary PDF; userLocale required |
| get.workspaces.invoices.payments | GET | /workspaces/{workspaceId}/invoices/{invoiceId}/payments | both | probed | 20260520-215801 | LIVE-VERIFIED; bare empty array []; payment item shape unconfirmed (no payments exist) |
| get.workspaces.invoices.settings | GET | /workspaces/{workspaceId}/invoices/settings | both | probed | 20260520-221429 | iter7 |
| get.workspaces.member-profile | GET | /workspaces/{workspaceId}/member-profile/{userId} | both | probed | 20260520-215801 | LIVE-VERIFIED; rich profile with inline userCustomFieldValues embedding full field defs |
| get.workspaces.policies | GET | /workspaces/{workspaceId}/policies | ours | probed | 20260520-224037 | iter8 LIVE-OVERRIDE; 404 endpoint absent |
| get.workspaces.policies | GET | /workspaces/{workspaceId}/policies/{policyId} | ours | probed | 20260521-000000 | iter9 LIVE-OVERRIDE: 404 endpoint absent |
| get.workspaces.projects | GET | /workspaces/{workspaceId}/projects/{projectId} | both | probed | 20260520-212439 | LIVE-VERIFIED; single shape identical to list item shape |
| get.workspaces.projects | GET | /workspaces/{workspaceId}/projects | both | probed | 20260520-212439 | LIVE-VERIFIED; bare array; page-size respected |
| get.workspaces.projects.custom-fields | GET | /workspaces/{workspaceId}/projects/{projectId}/custom-fields | both | probed | 20260520-224037 | iter8 LIVE-VERIFIED; bare array; pagination ignored |
| get.workspaces.projects.tasks | GET | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | both | probed | 20260520-214436 | LIVE-VERIFIED; shape identical to list item |
| get.workspaces.projects.tasks | GET | /workspaces/{workspaceId}/projects/{projectId}/tasks | both | probed | 20260520-214436 | LIVE-VERIFIED; bare array; page-size respected |
| get.workspaces.scheduling.assignments.all | GET | /workspaces/{workspaceId}/scheduling/assignments/all | both | probed | 20260520-215801 | LIVE-OVERRIDE: start/end are REQUIRED (400 without start); bare array; page-size respected |
| get.workspaces.scheduling.assignments.projects.totals | GET | /workspaces/{workspaceId}/scheduling/assignments/projects/totals | official | probed | 20260521-000000 | iter9 LIVE-VERIFIED: bare array; start required |
| get.workspaces.scheduling.assignments.projects.totals | GET | /workspaces/{workspaceId}/scheduling/assignments/projects/totals/{projectId} | both | probed | 20260521-000000 | iter9 LIVE-VERIFIED: single object; start required |
| get.workspaces.scheduling.assignments.users.totals | GET | /workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals | both | probed | 20260521-000800 | iter10 LIVE-VERIFIED; single wrapped obj; start required |
| get.workspaces.shared-reports | GET | /workspaces/{workspaceId}/shared-reports | both | probed | 20260521-000800 | iter10 LIVE-OVERRIDE: 404 endpoint absent |
| get.workspaces.tags | GET | /workspaces/{workspaceId}/tags/{id} | official | probed | 20260520-212439 | LIVE-VERIFIED; 4-key shape: archived/id/name/workspaceId |
| get.workspaces.tags | GET | /workspaces/{workspaceId}/tags | both | probed | 20260520-212439 | LIVE-VERIFIED; bare array; page-size respected |
| get.workspaces.tags | GET | /workspaces/{workspaceId}/tags/{tagId} | ours | probed | 20260520-212439 | LIVE-VERIFIED; 4-key shape: archived/id/name/workspaceId |
| get.workspaces.templates | GET | /workspaces/{workspaceId}/templates | official | probed | 20260521-000800 | iter10 LIVE-OVERRIDE: 404 endpoint absent/unavailable |
| get.workspaces.templates | GET | /workspaces/{workspaceId}/templates/{templateId} | official | probed | 20260521T194101Z | iter26 LIVE-OVERRIDE: real project-template candidates return HTTP 400 code 501 on workspace-level route |
| get.workspaces.time-entries | GET | /workspaces/{workspaceId}/time-entries/{timeEntryId} | ours | probed | 20260520-214436 | LIVE-VERIFIED; also confirmed bare GET list (no ID) returns 405 |
| get.workspaces.time-entries | GET | /workspaces/{workspaceId}/time-entries/{id} | official | probed | 20260520-214436 | LIVE-VERIFIED; single item GET works |
| get.workspaces.time-entries.status.in-progress | GET | /workspaces/{workspaceId}/time-entries/status/in-progress | both | probed | 20260520-224037 | iter8 LIVE-VERIFIED; bare array of running timers |
| get.workspaces.time-off.balance.policy | GET | /workspaces/{workspaceId}/time-off/balance/policy/{policyId} | both | probed | 20260520-224037 | iter8 LIVE-VERIFIED; {balances:[],count:N}; pagination works |
| get.workspaces.time-off.balance.user | GET | /workspaces/{workspaceId}/time-off/balance/user/{userId} | both | probed | 20260521-000800 | iter10 LIVE-VERIFIED; wrapped; pagination works |
| get.workspaces.time-off.policies | GET | /workspaces/{workspaceId}/time-off/policies | both | probed | 20260520-214436 | LIVE-VERIFIED; bare array; page-size respected |
| get.workspaces.time-off.policies | GET | /workspaces/{workspaceId}/time-off/policies/{id} | official | probed | 20260520-214436 | LIVE-VERIFIED; single shape identical to list |
| get.workspaces.time-off.policies | GET | /workspaces/{workspaceId}/time-off/policies/{policyId} | ours | probed | 20260520-214436 | LIVE-VERIFIED; single shape identical to list |
| get.workspaces.time-off.requests | GET | /workspaces/{workspaceId}/time-off/requests/{requestId} | ours | probed | 20260520-215801 | UNRESOLVED-NO-LIVE: 404 No static resource with fake ID; cannot confirm route exists without real request ID |
| get.workspaces.time-off.requests | GET | /workspaces/{workspaceId}/time-off/requests | ours | probed | 20260520-215801 | LIVE-OVERRIDE: 405 GET not supported; listing is POST-based |
| get.workspaces.user-groups | GET | /workspaces/{workspaceId}/user-groups | both | probed | 20260520-214436 | LIVE-VERIFIED; bare array; page-size respected; 26 groups total |
| get.workspaces.user-groups | GET | /workspaces/{workspaceId}/user-groups/{groupId} | ours | probed | 20260520-214436 | LIVE-OVERRIDE: GET single returns 405; only list supported |
| get.workspaces.user-groups.users | GET | /workspaces/{workspaceId}/user-groups/{groupId}/users | ours | probed | 20260520-224037 | iter8 LIVE-OVERRIDE: 405 GET not supported |
| get.workspaces.user.time-entries | GET | /workspaces/{workspaceId}/user/{userId}/time-entries | both | probed | 20260520-215801 | LIVE-VERIFIED; bare array; page-size respected; shape matches single time entry |
| get.workspaces.users | GET | /workspaces/{workspaceId}/users | both | probed | 20260520-212439 | UNRESOLVED-NO-LIVE: even page-size=1 returns ~70KB which exceeds MCP tool limit |
| get.workspaces.users.managers | GET | /workspaces/{workspaceId}/users/{userId}/managers | both | probed | 20260521-001500 | iter11 LIVE-VERIFIED; bare array |
| get.workspaces.users.time-off.balances | GET | /workspaces/{workspaceId}/users/{userId}/time-off/balances | ours | probed | 20260521-001500 | iter11 LIVE-OVERRIDE: 404 absent |
| get.workspaces.webhooks | GET | /workspaces/{workspaceId}/webhooks | both | probed | 20260520-212439 | LIVE-OVERRIDE: wrapped {webhooks:[], workspaceWebhookCount:N}; page-size ignored |
| get.workspaces.webhooks | GET | /workspaces/{workspaceId}/webhooks/{webhookId} | both | probed | 20260520-212439 | LIVE-VERIFIED; same shape as list items |
| get.workspaces.webhooks.logs | GET | /workspaces/{workspaceId}/webhooks/{webhookId}/logs | ours | probed | 20260521-001500 | iter11 LIVE-OVERRIDE: 405 GET not supported |
| patch.workspaces.approval-requests | PATCH | /workspaces/{workspaceId}/approval-requests/{approvalRequestId} | both | probed | 20260521-004500 | iter16 LIVE-VERIFIED; status=APPROVED confirmed |
| patch.workspaces.balance | PATCH | /workspaces/{workspaceId}/balance | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; Spring 404 path absent |
| patch.workspaces.expenses.categories.status | PATCH | /workspaces/{workspaceId}/expenses/categories/{categoryId}/status | both | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.invoices.status | PATCH | /workspaces/{workspaceId}/invoices/{invoiceId}/status | both | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.member-profile | PATCH | /workspaces/{workspaceId}/member-profile/{userId} | both | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.policies.archive | PATCH | /workspaces/{workspaceId}/policies/{policyId}/archive | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; /policies/* absent, use /time-off/policies |
| patch.workspaces.policies.requests | PATCH | /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId} | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; /policies/* absent, use /time-off/policies |
| patch.workspaces.projects.custom-fields | PATCH | /workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId} | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable project custom-field defaultValue PATCH returned HTTP 200 with projectDefaultValues |
| patch.workspaces.projects.estimate | PATCH | /workspaces/{workspaceId}/projects/{projectId}/estimate | both | probed | 20260521-004500 | iter16 LIVE-VERIFIED; returns full project object |
| patch.workspaces.projects.memberships | PATCH | /workspaces/{workspaceId}/projects/{projectId}/memberships | both | probed | 20260521-004500 | iter16 LIVE-VERIFIED; response is array not object |
| patch.workspaces.projects.template | PATCH | /workspaces/{workspaceId}/projects/{projectId}/template | both | probed | 20260521-010000 | iter17 UNCONFIRMED-AGREE; 405 path registered |
| patch.workspaces.scheduling.assignments.recurring | PATCH | /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId} | both | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.templates | PATCH | /workspaces/{workspaceId}/templates/{templateId} | official | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; workspace-level templates absent |
| patch.workspaces.time-entries.invoiced | PATCH | /workspaces/{workspaceId}/time-entries/invoiced | both | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.time-entries.invoiced.bulk | PATCH | /workspaces/{workspaceId}/time-entries/invoiced/bulk | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; /invoiced/bulk absent, base /invoiced already bulk |
| patch.workspaces.time-off.balance.policy | PATCH | /workspaces/{workspaceId}/time-off/balance/policy/{policyId} | both | probed | 20260521T203428Z | LIVE-VERIFIED; disposable time-off policy balance PATCH returned HTTP 204 |
| patch.workspaces.time-off.policies | PATCH | /workspaces/{workspaceId}/time-off/policies/{id} | official | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.time-off.policies | PATCH | /workspaces/{workspaceId}/time-off/policies/{policyId} | ours | probed | | |
| patch.workspaces.time-off.policies.requests | PATCH | /workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId} | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; canonical approve/deny path |
| patch.workspaces.time-off.requests.status | PATCH | /workspaces/{workspaceId}/time-off/requests/{requestId}/status | ours | probed | 20260521-PARALLEL | iter15-parallel |
| patch.workspaces.user.time-entries | PATCH | /workspaces/{workspaceId}/user/{userId}/time-entries | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; timer-stop path, GET 200 confirmed |
| patch.workspaces.user.time-entries.stop | PATCH | /workspaces/{workspaceId}/user/{userId}/time-entries/stop | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; /stop absent, stale allowlist entry |
| patch.workspaces.webhooks.generateNewToken | PATCH | /workspaces/{workspaceId}/webhooks/{webhookId}/generateNewToken | ours | probed | 20260521-010000 | iter17 LIVE-OVERRIDE; path absent, use /token |
| patch.workspaces.webhooks.token | PATCH | /workspaces/{workspaceId}/webhooks/{webhookId}/token | both | probed | 20260521-PARALLEL | iter15-parallel |
| post.file.image | POST | /file/image | both | probed | 20260521-010000 | iter17 UNCONFIRMED-AGREE; 405 confirmed path registered |
| post.workspaces | POST | /workspaces | both | probed | 20260521-010000 | iter17 UNVERIFIABLE-DESTRUCTIVE |
| post.workspaces.approval-requests | POST | /workspaces/{workspaceId}/approval-requests | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; tool confirmed |
| post.workspaces.approval-requests.resubmit-entries-for-approval | POST | /workspaces/{workspaceId}/approval-requests/resubmit-entries-for-approval | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; POST 400 biz-error confirms live |
| post.workspaces.approval-requests.users | POST | /workspaces/{workspaceId}/approval-requests/users/{userId} | both | probed | 20260521-010000 | iter17 LIVE-VERIFIED; POST 201 confirmed |
| post.workspaces.approval-requests.users.resubmit-entries-for-approval | POST | /workspaces/{workspaceId}/approval-requests/users/{userId}/resubmit-entries-for-approval | both | probed | 20260521T203428Z | LIVE-VERIFIED; user-scoped resubmit POST returned HTTP 200 approval object |
| post.workspaces.audit-log | POST | /workspaces/{workspaceId}/audit-log | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; tool returned 7 entries; content field is stringified JSON |
| post.workspaces.clients | POST | /workspaces/{workspaceId}/clients | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+delete; response id/name/currencyCode/currencyId/workspaceId |
| post.workspaces.custom-fields | POST | /workspaces/{workspaceId}/custom-fields | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+delete; entityType defaults TIMEENTRY |
| post.workspaces.expenses | POST | /workspaces/{workspaceId}/expenses | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; dry_run; categoryId required; amount coerced to string |
| post.workspaces.expenses.categories | POST | /workspaces/{workspaceId}/expenses/categories | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; dry_run ok:true |
| post.workspaces.holidays | POST | /workspaces/{workspaceId}/holidays | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+delete; date_period object shape |
| post.workspaces.invoices | POST | /workspaces/{workspaceId}/invoices | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; dry_run; default status UNSENT |
| post.workspaces.invoices.duplicate | POST | /workspaces/{workspaceId}/invoices/{invoiceId}/duplicate | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable invoice duplicate returned HTTP 201 invoice object |
| post.workspaces.invoices.info | POST | /workspaces/{workspaceId}/invoices/info | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; returns 121 invoices + total |
| post.workspaces.invoices.items | POST | /workspaces/{workspaceId}/invoices/{invoiceId}/items | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; dry_run; itemType/unitPrice/applyTaxes |
| post.workspaces.invoices.items.import | POST | /workspaces/{workspaceId}/invoices/{invoiceId}/items/import | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; single path for import_time+import_expenses; importExpenses flag |
| post.workspaces.invoices.payments | POST | /workspaces/{workspaceId}/invoices/{invoiceId}/payments | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+delete; source=invoice_payments_api |
| post.workspaces.policies | POST | /workspaces/{workspaceId}/policies | ours | probed | 20260521-020000 | iter18 LIVE-OVERRIDE; /policies/ absent; ours-only phantom |
| post.workspaces.policies.requests | POST | /workspaces/{workspaceId}/policies/{policyId}/requests | ours | probed | 20260521-020000 | iter18 LIVE-OVERRIDE; /policies/ subtree absent; ours-only phantom |
| post.workspaces.projects | POST | /workspaces/{workspaceId}/projects | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+archive+delete; creator auto-added as member |
| post.workspaces.projects.from-template | POST | /workspaces/{workspaceId}/projects/from-template | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable template project cloned with templateProjectId, cleanup zero leftovers |
| post.workspaces.projects.memberships | POST | /workspaces/{workspaceId}/projects/{projectId}/memberships | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable project current-user delta POST returned HTTP 200 project object |
| post.workspaces.projects.tasks | POST | /workspaces/{workspaceId}/projects/{projectId}/tasks | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; create+delete; estimate is plain string |
| post.workspaces.reports.attendance | POST | /workspaces/{workspaceId}/reports/attendance | both | probed | 20260521-020000 | iter18 LIVE-VERIFIED; {entities:[...]}; durations in seconds |
| post.workspaces.reports.detailed | POST | /workspaces/{workspaceId}/reports/detailed | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.reports.expenses.detailed | POST | /workspaces/{workspaceId}/reports/expenses/detailed | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.reports.summary | POST | /workspaces/{workspaceId}/reports/summary | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.reports.weekly | POST | /workspaces/{workspaceId}/reports/weekly | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.scheduling.assignments | POST | /workspaces/{workspaceId}/scheduling/assignments | ours | probed | 20260521-030000 | iter19 LIVE-OVERRIDE |
| post.workspaces.scheduling.assignments.copy | POST | /workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable assignment copied with {userId,seriesUpdateOption}, returned bare array |
| post.workspaces.scheduling.assignments.projects.totals | POST | /workspaces/{workspaceId}/scheduling/assignments/projects/totals | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.scheduling.assignments.recurring | POST | /workspaces/{workspaceId}/scheduling/assignments/recurring | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.scheduling.assignments.user-filter.totals | POST | /workspaces/{workspaceId}/scheduling/assignments/user-filter/totals | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.scheduling.assignments.users.totals | POST | /workspaces/{workspaceId}/scheduling/assignments/users/totals | ours | probed | 20260521-030000 | iter19 LIVE-OVERRIDE |
| post.workspaces.shared-reports | POST | /workspaces/{workspaceId}/shared-reports | both | probed | 20260521-030000 | iter19 LIVE-OVERRIDE |
| post.workspaces.tags | POST | /workspaces/{workspaceId}/tags | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.templates | POST | /workspaces/{workspaceId}/templates | official | probed | 20260521-030000 | iter19 LIVE-OVERRIDE |
| post.workspaces.time-entries | POST | /workspaces/{workspaceId}/time-entries | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.time-off.policies | POST | /workspaces/{workspaceId}/time-off/policies | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.time-off.policies.requests | POST | /workspaces/{workspaceId}/time-off/policies/{policyId}/requests | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.time-off.policies.users.requests | POST | /workspaces/{workspaceId}/time-off/policies/{policyId}/users/{userId}/requests | both | probed | 20260521T203428Z | LIVE-VERIFIED; disposable policy user-scoped request POST returned HTTP 200 |
| post.workspaces.time-off.requests | POST | /workspaces/{workspaceId}/time-off/requests | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.time-off.requests.users | POST | /workspaces/{workspaceId}/time-off/requests/users/{userId} | ours | probed | 20260521-030000 | iter19 LIVE-OVERRIDE |
| post.workspaces.user-groups | POST | /workspaces/{workspaceId}/user-groups | both | probed | 20260521-030000 | iter19 LIVE-VERIFIED |
| post.workspaces.user-groups.users | POST | /workspaces/{workspaceId}/user-groups/{userGroupId}/users | official | probed | 20260521-040000 | iter20 LIVE-VERIFIED; lifecycle evidence from user_admin.go:450 |
| post.workspaces.user-groups.users | POST | /workspaces/{workspaceId}/user-groups/{groupId}/users | ours | probed | 20260521-040000 | iter20 LIVE-VERIFIED; cosmetic param name diff (userGroupId vs groupId) |
| post.workspaces.user.time-entries | POST | /workspaces/{workspaceId}/user/{userId}/time-entries | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; POST=create, GET=list on same path |
| post.workspaces.user.time-entries.duplicate | POST | /workspaces/{workspaceId}/user/{userId}/time-entries/{id}/duplicate | official | probed | 20260521T200626Z | LIVE-VERIFIED; disposable entry duplicate returned HTTP 201 time-entry object |
| post.workspaces.user.time-entries.duplicate | POST | /workspaces/{workspaceId}/user/{userId}/time-entries/{timeEntryId}/duplicate | ours | probed | 20260521T200626Z | LIVE-VERIFIED; cosmetic param alias for official {id}; same live duplicate evidence |
| post.workspaces.users | POST | /workspaces/{workspaceId}/users | both | probed | 20260521T203625Z | UNCONFIRMED-AGREE; send-email=false disposable invite POST blocked by subscription seat limit |
| post.workspaces.users.info | POST | /workspaces/{workspaceId}/users/info | both | probed | 20260521T200626Z | LIVE-VERIFIED; page-size 1 POST-as-filter returned HTTP 200 bare user array |
| post.workspaces.users.roles | POST | /workspaces/{workspaceId}/users/{userId}/roles | both | probed | 20260521T203746Z | LIVE-VERIFIED; PROJECT_MANAGER granted on disposable project to non-self test fixture, then project cleaned up |
| post.workspaces.webhooks | POST | /workspaces/{workspaceId}/webhooks | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable webhook create returned HTTP 201 and cleanup deleted it |
| post.workspaces.webhooks.logs | POST | /workspaces/{workspaceId}/webhooks/{webhookId}/logs | both | probed | 20260521T200626Z | LIVE-VERIFIED; POST logs query on disposable webhook returned HTTP 200 bare array |
| put.workspaces | PUT | /workspaces/{workspaceId} | ours | probed | 20260521-040000 | iter20 LIVE-OVERRIDE; PUT returns 405; phantom endpoint; official spec has GET only |
| put.workspaces.clients | PUT | /workspaces/{workspaceId}/clients/{clientId} | ours | probed | 20260521T200626Z | LIVE-VERIFIED; disposable client PUT rename returned HTTP 200, then archived+deleted |
| put.workspaces.clients | PUT | /workspaces/{workspaceId}/clients/{id} | official | probed | 20260521T200626Z | LIVE-VERIFIED; {id} vs {clientId} cosmetic param name difference, same live PUT evidence |
| put.workspaces.clients.archive | PUT | /workspaces/{workspaceId}/clients/{clientId}/archive | ours | probed | 20260521-040000 | iter20 LIVE-OVERRIDE; 404 Spring-MVC; archive via body field on PUT /clients/{id} |
| put.workspaces.cost-rate | PUT | /workspaces/{workspaceId}/cost-rate | both | probed | 20260521T203428Z | LIVE-VERIFIED; sacrificial workspace cost-rate PUT returned HTTP 200 workspace object |
| put.workspaces.custom-fields | PUT | /workspaces/{workspaceId}/custom-fields/{customFieldId} | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; PUT 200 with custom field object; type required by Clockify |
| put.workspaces.expenses | PUT | /workspaces/{workspaceId}/expenses/{expenseId} | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; PUT 200; expense updated+restored to original state |
| put.workspaces.expenses.categories | PUT | /workspaces/{workspaceId}/expenses/categories/{categoryId} | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; dry_run:true confirmed; partial update with name pre-fetch |
| put.workspaces.holidays | PUT | /workspaces/{workspaceId}/holidays/{holidayId} | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; PUT 200; automaticTimeEntryCreation field must be omitted |
| put.workspaces.hourly-rate | PUT | /workspaces/{workspaceId}/hourly-rate | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; PUT 200 with full workspace object; {amount: minor_units} |
| put.workspaces.invoices | PUT | /workspaces/{workspaceId}/invoices/{invoiceId} | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; dry_run:true; invoice <redacted-id:8491414151> confirmed live |
| put.workspaces.invoices.settings | PUT | /workspaces/{workspaceId}/invoices/settings | both | probed | 20260521-040000 | iter20 LIVE-VERIFIED; GET 200 settings; PUT round-trip returns 200 empty body |
| put.workspaces.member-profile | PUT | /workspaces/{workspaceId}/member-profile/{userId} | ours | probed | 20260521-040000 | iter20 LIVE-OVERRIDE; PUT 405; GET 200; PATCH is correct verb |
| put.workspaces.policies | PUT | /workspaces/{workspaceId}/policies/{policyId} | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; entire /policies/* subtree absent (404); confirmed iter13/21 |
| put.workspaces.projects | PUT | /workspaces/{workspaceId}/projects/{projectId} | both | probed | 20260521-050000 | iter21 LIVE-VERIFIED; PUT 200; required name; archival via archived:true in body |
| put.workspaces.projects.archive | PUT | /workspaces/{workspaceId}/projects/{projectId}/archive | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; Spring-MVC 404; archival via PUT /projects/{id} with archived:true |
| put.workspaces.projects.cost-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/cost-rate | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; 404; correct path is /users/{userId}/cost-rate |
| put.workspaces.projects.hourly-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/hourly-rate | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; 404; correct path is /users/{userId}/hourly-rate |
| put.workspaces.projects.tasks | PUT | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | both | probed | 20260521-050000 | iter21 LIVE-VERIFIED; PUT 200; required name; GET-then-merge |
| put.workspaces.projects.tasks.cost-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate | official | probed | 20260521-050000 | iter21 LIVE-VERIFIED; PUT 200 costRate.amount confirmed; {id} vs {taskId} cosmetic |
| put.workspaces.projects.tasks.cost-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/cost-rate | ours | probed | 20260521-050000 | iter21 LIVE-VERIFIED; same as official variant; clockify_tasks_rates_update |
| put.workspaces.projects.tasks.hourly-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate | official | probed | 20260521-050000 | iter21 LIVE-VERIFIED; PUT 200 hourlyRate.amount confirmed |
| put.workspaces.projects.tasks.hourly-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/hourly-rate | ours | probed | 20260521-050000 | iter21 LIVE-VERIFIED; same as official variant |
| put.workspaces.projects.users.cost-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate | both | probed | 20260521-050000 | iter21 LIVE-VERIFIED; GET 405; PUT 200 costRate.amount=3000 confirmed |
| put.workspaces.projects.users.hourly-rate | PUT | /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate | both | probed | 20260521-050000 | iter21 LIVE-VERIFIED; GET 405; PUT 200 hourlyRate.amount=5000 confirmed |
| put.workspaces.scheduling.assignments | PUT | /workspaces/{workspaceId}/scheduling/assignments/{assignmentId} | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; Spring-MVC 404; correct path is PATCH /recurring/{id} |
| put.workspaces.scheduling.assignments.publish | PUT | /workspaces/{workspaceId}/scheduling/assignments/publish | both | probed | 20260521-050000 | iter21 LIVE-VERIFIED; GET 405; PUT 200 published:true; clockify_scheduling_publish |
| put.workspaces.scheduling.assignments.recurring | PUT | /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId} | ours | probed | 20260521T194802Z | iter26 LIVE-OVERRIDE; direct PUT on fresh recurring assignment returned HTTP 405; PATCH is supported update method |
| put.workspaces.scheduling.assignments.series | PUT | /workspaces/{workspaceId}/scheduling/assignments/series/{assignmentId} | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable assignment accepted RecurringAssignmentRequestV1 {weeks,repeat}, returned bare array |
| put.workspaces.shared-reports | PUT | /workspaces/{workspaceId}/shared-reports/{id} | official | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; parent path 404 No static resource (iter19); entire shared-reports family absent |
| put.workspaces.shared-reports | PUT | /workspaces/{workspaceId}/shared-reports/{sharedReportId} | ours | probed | 20260521-050000 | iter21 LIVE-OVERRIDE; same as official row; ours-only param name cosmetic |
| put.workspaces.tags | PUT | /workspaces/{workspaceId}/tags/{tagId} | ours | probed | 20260521-050000 | iter21 LIVE-VERIFIED; PUT 200 no-op update; tag id=<redacted-id:0458fb1a5e>; full replacement semantics; name+archived required |
| put.workspaces.tags | PUT | /workspaces/{workspaceId}/tags/{id} | official | probed | 20260521-050000 | iter21 LIVE-VERIFIED; official {id} alias for ours {tagId}; same probe |
| put.workspaces.time-entries | PUT | /workspaces/{workspaceId}/time-entries/{id} | official | probed | 20260521-060000 | iter22 LIVE-VERIFIED; official {id} alias for ours {timeEntryId}, same probe |
| put.workspaces.time-entries | PUT | /workspaces/{workspaceId}/time-entries/{timeEntryId} | ours | probed | 20260521-060000 | iter22 LIVE-VERIFIED; entry <redacted-id:1dac181586> updated via MCP tool HTTP 200 |
| put.workspaces.time-off.policies | PUT | /workspaces/{workspaceId}/time-off/policies/{id} | official | probed | 20260521-060000 | iter22 LIVE-VERIFIED; official {id} alias for ours {policyId}, same probe |
| put.workspaces.time-off.policies | PUT | /workspaces/{workspaceId}/time-off/policies/{policyId} | ours | probed | 20260521-060000 | iter22 LIVE-VERIFIED; policy <redacted-id:084a4a18c0> PUT HTTP 200; coexists with PATCH at same path |
| put.workspaces.user-groups | PUT | /workspaces/{workspaceId}/user-groups/{id} | official | probed | 20260521-060000 | iter22 LIVE-VERIFIED; official {id} alias for ours {groupId}, same probe |
| put.workspaces.user-groups | PUT | /workspaces/{workspaceId}/user-groups/{groupId} | ours | probed | 20260521-060000 | iter22 LIVE-VERIFIED; PUT 200 with full group object; name+userIds full-replacement semantics |
| put.workspaces.user.time-entries | PUT | /workspaces/{workspaceId}/user/{userId}/time-entries | both | probed | 20260521T200626Z | LIVE-VERIFIED; disposable entry bulk PUT with array body returned HTTP 200 bare hydrated array |
| put.workspaces.users | PUT | /workspaces/{workspaceId}/users/{userId} | both | probed | 20260521-060000 | iter22 LIVE-VERIFIED; GET 405 confirms path registered; clockify_users_deactivate uses PUT {status:INACTIVE} |
| put.workspaces.users.cost-rate | PUT | /workspaces/{workspaceId}/users/{userId}/cost-rate | both | probed | 20260521T203428Z | LIVE-VERIFIED; PUT returned HTTP 200 workspace object |
| put.workspaces.users.custom-field.value | PUT | /workspaces/{workspaceId}/users/{userId}/custom-field/{customFieldId}/value | both | probed | 20260521T203428Z | LIVE-VERIFIED; disposable USER custom field value PUT returned HTTP 201 and field was deleted |
| put.workspaces.users.hourly-rate | PUT | /workspaces/{workspaceId}/users/{userId}/hourly-rate | both | probed | 20260521T203428Z | LIVE-VERIFIED; PUT returned HTTP 200 workspace object |
| put.workspaces.webhooks | PUT | /workspaces/{workspaceId}/webhooks/{webhookId} | both | probed | 20260521-060000 | iter22 LIVE-VERIFIED; webhook <redacted-id:508f41ec5c> PUT HTTP 200; same-value no-op confirmed |
