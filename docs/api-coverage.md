# MCP API Coverage Matrix

Maps every Clockify MCP tool to its upstream Clockify API endpoint,
safety classification, and test coverage. Generated from
`docs/tool-catalog.md`, `internal/tools/`, `internal/clockify/`,
`internal/paths/`, and `tests/`.

> **WARNING: Skipped local live tests are non-evidence.** `go test
> -tags=livee2e ./tests/...` without `CLOCKIFY_RUN_LIVE_E2E=1` +
> `CLOCKIFY_API_KEY` + `CLOCKIFY_WORKSPACE_ID` silently skips every
> live test. A fast `ok` (~0.5s) means the gate was not visible to
> the test process. `TestLiveContractSkipSentinel` now fails
> explicitly when every test skips. Use `make live-contract-local`
> for pre-flight debugging. The authoritative evidence path is
> `.github/workflows/live-contract.yml` (scheduled cron).

## Summary

| Classification | Tier 1 | Tier 2 | Total |
|----------------|--------|--------|-------|
| Read-only | 25 | 33 | 58 |
| Mutating (non-destructive) | 18 | 42 | 60 |
| Destructive | 2 | 13 | 15 |
| Billing | 0 | 8 | 8 |
| Admin | 0 | 7 | 7 |
| **Total tools** | **45** | **88** | **133** |

## Evidence types

| Type | Meaning | Used for |
|------|---------|----------|
| **local unit** | `go test` without external deps | Handler logic, schema validation, policy enforcement, dry-run |
| **mocked integration** | `go test` with httptest mocks | Client→API path, error mapping, retry behaviour |
| **live read-only** | `-tags=livee2e` read-only tier | Schema drift, auth flow, rate-limit behaviour |
| **sacrificial mutating** | `-tags=livee2e` with `CLOCKIFY_LIVE_WRITE_ENABLED=true` | Full CRUD, audit phases |
| **live-probed unsupported** | `-tags=livee2e` tool call reaches Clockify and asserts a concrete 4xx / workspace-state response | Upstream gaps, plan gates, permission gates, unsupported routes |
| **scheduled workflow** | `.github/workflows/live-contract.yml` cron | Authoritative evidence for launch gates |

---

## Tier 1 — Core tools (45)

The per-tool tables below list the stable local test coverage that
ships with normal CI. The manual sacrificial-workspace section later
in this document records the exhaustive live-probe layer added on top
of these unit/mocked tests.

Clockify endpoints: `GET/POST/PUT/PATCH/DELETE /workspaces/{ws}/time-entries`,
`/workspaces/{ws}/projects`, `/workspaces/{ws}/clients`,
`/workspaces/{ws}/tags`, `/workspaces/{ws}/tasks`,
`/workspaces/{ws}/users`, `/workspaces/{ws}/reports/*`,
`/user`, `/workspaces`.

### Read-only (24 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_current_user` | `GET /user` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_detailed_report` | `GET /workspaces/{ws}/reports/detailed` | unit |
| `clockify_get_client` | `GET /workspaces/{ws}/clients/{id}` | unit |
| `clockify_get_entry` | `GET /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_get_project` | `GET /workspaces/{ws}/projects/{id}` | unit |
| `clockify_get_task` | `GET /workspaces/{ws}/projects/{id}/tasks/{tid}` | unit |
| `clockify_get_workspace` | `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_clients` | `GET /workspaces/{ws}/clients` | unit |
| `clockify_list_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` | unit |
| `clockify_list_projects` | `GET /workspaces/{ws}/projects` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_tags` | `GET /workspaces/{ws}/tags` | unit |
| `clockify_list_tasks` | `GET /workspaces/{ws}/projects/{id}/tasks` | unit |
| `clockify_list_users` | `GET /workspaces/{ws}/users` | unit |
| `clockify_list_workspaces` | `GET /workspaces` | unit |
| `clockify_policy_info` | local (no API call) | unit |
| `clockify_quick_report` | wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries`) | unit |
| `clockify_resolve_debug` | compatibility alias over name resolution lookup | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_resolve_name` | name resolution lookup over project/client/tag/user list endpoints | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_summary_report` | wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries`) | unit |
| `clockify_timer_status` | `GET /workspaces/{ws}/user/{uid}/time-entries?in-progress=true` | unit |
| `clockify_timesheet_review` | workflow wrapper over `GET /workspaces/{ws}/user/{uid}/time-entries` | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_today_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` (filtered) | unit |
| `clockify_weekly_summary` | wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries` by day + project) | unit |
| `clockify_whoami` | `GET /user` + `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |

### Mutating — non-destructive (19 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_add_entry` | `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_client` | `POST /workspaces/{ws}/clients` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_project` | `POST /workspaces/{ws}/projects` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_tag` | `POST /workspaces/{ws}/tags` | unit |
| `clockify_create_task` | `POST /workspaces/{ws}/projects/{id}/tasks` | unit |
| `clockify_activate_group` | local (tool-surface mutation) | unit |
| `clockify_activate_tool` | local (tool-surface mutation) | unit |
| `clockify_deactivate_group` | local (tool-surface mutation) | unit |
| `clockify_find_and_update_entry` | `GET` + `PUT /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_list_tools` | local (catalog query) | unit |
| `clockify_log_time` | `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_search_tools` | local (deprecated catalog/activation shim) | unit |
| `clockify_start_timer` | `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_stop_timer` | `PATCH /workspaces/{ws}/user/{uid}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_switch_project` | `PATCH` + `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_timesheet_fill_gap` | `GET` overlap validation + `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestLiveTier1RemainingCRUD) |
| `clockify_update_client` | `GET` + `PUT /workspaces/{ws}/clients/{id}` (fetch-then-merge) | unit |
| `clockify_update_entry` | `GET` + `PUT /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_update_task` | `GET` + `PUT /workspaces/{ws}/projects/{id}/tasks/{tid}` (fetch-then-merge) | unit |

### Destructive (2 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_delete_client` | `GET` + `PUT {archived:true}` + `DELETE /workspaces/{ws}/clients/{id}` | unit, dry-run |
| `clockify_delete_entry` | `DELETE /workspaces/{ws}/time-entries/{id}` | unit, dry-run (TestLiveDryRunDoesNotMutate), sacrificial-mutating (TestE2EMutating) |

---

## Tier 2 — Domain groups (88 tools)

### `approvals` (6 tools)

Clockify endpoints: `GET/POST/PUT /workspaces/{ws}/approval-requests/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_approve_timesheet` | mutating | unit |
| `clockify_get_approval_request` | read-only | unit |
| `clockify_list_approval_requests` | read-only | unit |
| `clockify_reject_timesheet` | mutating | unit |
| `clockify_submit_for_approval` | mutating | unit |
| `clockify_withdraw_approval` | mutating | unit |

### `custom_fields` (6 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/custom-fields/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_custom_field` | mutating | unit |
| `clockify_delete_custom_field` | destructive | unit |
| `clockify_get_custom_field` | read-only | unit |
| `clockify_list_custom_fields` | read-only | unit |
| `clockify_set_custom_field_value` | mutating | unit |
| `clockify_update_custom_field` | mutating | unit |

### `expenses` (10 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/expenses/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_expense` | mutating | unit |
| `clockify_create_expense_category` | mutating | unit |
| `clockify_delete_expense` | destructive | unit |
| `clockify_delete_expense_category` | destructive | unit |
| `clockify_expense_report` | read-only | unit |
| `clockify_get_expense` | read-only | unit |
| `clockify_list_expense_categories` | read-only | unit |
| `clockify_list_expenses` | read-only | unit |
| `clockify_update_expense` | mutating | unit |
| `clockify_update_expense_category` | mutating | unit |

### `groups_holidays` (8 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/groups/*`, `/workspaces/{ws}/holidays/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_holiday` | mutating | unit |
| `clockify_create_user_group_admin` | mutating | unit |
| `clockify_delete_holiday` | destructive | unit |
| `clockify_delete_user_group_admin` | destructive | unit |
| `clockify_get_user_group` | read-only | unit |
| `clockify_list_holidays` | read-only | unit |
| `clockify_list_user_groups_admin` | read-only | unit |
| `clockify_update_user_group_admin` | mutating | unit |

### `invoices` (12 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/invoices/*`

| Tool | Classification | Risk tags | Tests |
|------|---------------|-----------|-------|
| `clockify_add_invoice_item` | mutating | `billing` | unit |
| `clockify_create_invoice` | mutating | `billing` | unit |
| `clockify_delete_invoice` | destructive | `billing` | unit |
| `clockify_delete_invoice_item` | destructive | `billing` | unit |
| `clockify_get_invoice` | read-only | | unit |
| `clockify_invoice_report` | read-only | | unit |
| `clockify_list_invoice_items` | read-only | | unit |
| `clockify_list_invoices` | read-only | | unit |
| `clockify_mark_invoice_paid` | mutating | `billing` | unit |
| `clockify_send_invoice` | mutating | `billing`, `external_side_effect` | unit |
| `clockify_update_invoice` | mutating | `billing` | unit |
| `clockify_update_invoice_item` | mutating | `billing` | unit |

### `project_admin` (6 tools)

Clockify endpoints: `PUT/DELETE /workspaces/{ws}/projects/*`, `/workspaces/{ws}/project-templates/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_archive_projects` | mutating | unit |
| `clockify_create_project_template` | mutating | unit |
| `clockify_get_project_template` | read-only | unit |
| `clockify_list_project_templates` | read-only | unit |
| `clockify_set_project_memberships` | mutating | unit |
| `clockify_update_project_estimate` | mutating | unit |

### `scheduling` (7 tools)

Clockify endpoints: `GET /workspaces/{ws}/scheduling/assignments/all`,
`POST /workspaces/{ws}/scheduling/assignments/projects/totals`,
`GET /workspaces/{ws}/scheduling/assignments/users/{userId}/totals`,
and recurring-assignment write routes under
`/workspaces/{ws}/scheduling/assignments/recurring`.

Two phantom schedule tools (`get` and `create`) were removed alongside
the earlier `list_schedules` removal once the probe lab confirmed
Clockify has no `/scheduling/{id}` or `POST /scheduling` surface (only
`/scheduling/assignments/...` paths exist).

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_assignment` | mutating | unit + live (`TestLiveT2SchedulingRecurringCRUD`) |
| `clockify_delete_assignment` | destructive | unit + live (`TestLiveT2SchedulingRecurringCRUD`) |
| `clockify_filter_schedule_capacity` | read-only | unit + live |
| `clockify_get_assignment` | read-only | unit + live (`TestLiveT2SchedulingRecurringCRUD`; scans list endpoint because no single GET exists) |
| `clockify_get_project_schedule_totals` | read-only | unit |
| `clockify_list_assignments` | read-only | unit |
| `clockify_update_assignment` | mutating | unit + live (`TestLiveT2SchedulingRecurringCRUD`) |

### `shared_reports` (6 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/shared-reports/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_shared_report` | mutating | unit |
| `clockify_delete_shared_report` | destructive | unit |
| `clockify_export_shared_report` | read-only | unit |
| `clockify_get_shared_report` | read-only | unit |
| `clockify_list_shared_reports` | read-only | unit |
| `clockify_update_shared_report` | mutating | unit |

### `time_off` (12 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/time-off/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_approve_time_off` | mutating | unit |
| `clockify_create_time_off_policy` | mutating | unit |
| `clockify_create_time_off_request` | mutating | unit |
| `clockify_delete_time_off_request` | destructive | unit |
| `clockify_deny_time_off` | mutating | unit |
| `clockify_get_time_off_policy` | read-only | unit |
| `clockify_get_time_off_request` | read-only | unit |
| `clockify_list_time_off_policies` | read-only | unit |
| `clockify_list_time_off_requests` | read-only | unit |
| `clockify_time_off_balance` | read-only | unit |
| `clockify_update_time_off_policy` | mutating | unit |
| `clockify_update_time_off_request` | mutating | unit |

### `user_admin` (8 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/users/*`, `/workspaces/{ws}/user-groups/*`

| Tool | Classification | Risk tags | Tests |
|------|---------------|-----------|-------|
| `clockify_add_user_to_group` | mutating | `admin` | unit |
| `clockify_create_user_group` | mutating | `admin` | unit |
| `clockify_deactivate_user` | mutating | `admin` | unit |
| `clockify_delete_user_group` | destructive | `admin` | unit |
| `clockify_list_user_groups` | read-only | | unit |
| `clockify_remove_user_from_group` | destructive | `admin` | unit |
| `clockify_update_user_group` | mutating | `admin` | unit |
| `clockify_update_user_role` | mutating | `admin`, `permission_change` | unit |

**Invite-user route note:** Clockify documents
`POST /workspaces/{workspaceId}/users?send-email=...` for adding a
user to a workspace, but this MCP does not expose a
dedicated invite-user catalog tool. The manual live campaign pins the
route as a raw validation probe in `TestLiveT2UserInviteValidationProbe`
with `send-email=false` and an empty email. That exercises the current
route/plan/permission surface without sending mail or creating a
pending workspace member.

### `webhooks` (7 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/webhooks/*`

| Tool | Classification | Risk tags | Tests |
|------|---------------|-----------|-------|
| `clockify_create_webhook` | mutating | `external_side_effect` | unit |
| `clockify_delete_webhook` | destructive | `external_side_effect` | unit |
| `clockify_get_webhook` | read-only | | unit |
| `clockify_list_webhook_events` | read-only | | unit |
| `clockify_list_webhooks` | read-only | | unit |
| `clockify_test_webhook` | mutating | `external_side_effect` | unit |
| `clockify_update_webhook` | mutating | `external_side_effect` | unit |

---

## Schema-drift coverage

| Coverage | Status |
|----------|--------|
| `internal/clockify/models.go` struct tags | Full — every model field has a `json:"..."` tag |
| `TestLiveReadSideSchemaDiff` | Active — fetches raw Clockify JSON per read-side endpoint and fails on top-level fields not represented in `models.go` |
| Schema runs when | `live-contract.yml` read-only step (always) |

**Gap:** Only read-side (GET) endpoints are schema-checked. Mutating endpoints
(POST/PUT/PATCH) accept request payloads whose schemas are validated by the
MCP tool's JSON Schema descriptors, but there is no automated drift check
between those descriptors and the live Clockify API's current accepted fields.

---

## Dry-run / policy coverage

| Coverage | Status |
|----------|--------|
| `TestLiveDryRunDoesNotMutate` | Active — confirms `dry_run:true` on `clockify_delete_entry` previews instead of deleting |
| `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` | Active — confirms `time_tracking_safe` policy blocks `clockify_create_project` |
| Policy modes | `read_only`, `safe_core`, `standard`, `full` — tested via `internal/enforcement/` unit tests |

### Dry-run support per destructive tool (14 total)

| Tool | Tier | dry_run in schema | Live-tested | Notes |
|------|------|-------------------|-------------|-------|
| `clockify_delete_entry` | 1 | via enforcement pipeline | yes (`TestLiveDryRunDoesNotMutate`) | Only Tier 1 destructive tool |
| `clockify_delete_custom_field` | 2 | yes | conditional | Live test exercises this when the workspace is below the 50 custom-field cap; cap-skip is documented |
| `clockify_delete_expense` | 2 | yes | yes | Expense dry-run + real delete in `TestLiveT2ExpensesCRUD` |
| `clockify_delete_expense_category` | 2 | no | yes (4xx) | Live-probed archive-before-delete constraint; API exposes no archive route |
| `clockify_delete_holiday` | 2 | yes | yes (4xx) | Bogus-id/not-found path pinned; real created holiday cleanup uses raw client |
| `clockify_delete_user_group_admin` | 2 | yes | yes | Dry-run + real delete in `TestLiveT2GroupsHolidaysCRUD` |
| `clockify_delete_invoice` | 2 | yes | yes | Billing dry-run + real delete in `TestLiveT2InvoicesCRUD` |
| `clockify_delete_invoice_item` | 2 | yes | yes | Billing line-order delete in `TestLiveT2InvoicesCRUD` |
| `clockify_delete_shared_report` | 2 | yes | yes | Dry-run + real delete in `TestLiveT2SharedReportsCRUDAndExports` |
| `clockify_delete_assignment` | 2 | yes | yes | Minimal dry-run; real delete on recurring-assignment route |
| `clockify_delete_time_off_request` | 2 | yes | yes / 4xx | Real request delete when permitted; otherwise permission/route response pinned |
| `clockify_delete_webhook` | 2 | yes | yes | Dry-run + real delete in `TestLiveT2WebhooksCRUD` |
| `clockify_remove_user_from_group` | 2 | yes | yes | Dry-run + real remove in `TestLiveT2UserAdminCRUDAndOwnerSafety` |
| `clockify_deactivate_user` | 2 | yes | yes (dry-run only) | Owner deactivation is never executed; dry-run envelope is pinned |

**Dry-run support:** destructive/admin/billing handlers now expose
dry-run previews where the descriptor supports them. The exhaustive
manual suite live-probes every destructive tool name; some assertions
intentionally capture upstream 4xx constraints rather than a successful
preview/delete pair.

### Policy-mode live test coverage

| Mode | Unit-tested | Live-tested | Test |
|------|-------------|-------------|------|
| `read_only` | yes | yes | `TestLivePolicyModes` |
| `safe_core` | yes | yes | `TestLivePolicyModes` |
| `standard` | yes | implicitly (`TestE2EMutating` runs under standard) | enforcement + live mutating |
| `time_tracking_safe` | yes | yes | `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` |
| `full` | yes | yes | `TestLivePolicyModes` |

**Policy modes live-tested: 5 of 5** (100%) after the
`test/full-live-workspace-validation` campaign — see
`TestLivePolicyModes`, which parametrically exercises every mode
through the MCP path against a real Clockify backend.

---

## Live-contract test coverage

### Scheduled-workflow evidence (authoritative for launch gates)

| Test | Tools exercised | Evidence type |
|------|----------------|---------------|
| `TestE2EReadOnly` | `clockify_whoami`, `clockify_current_user`, `clockify_get_workspace`, `clockify_list_projects` | scheduled workflow |
| `TestE2EErrors` | error paths (invalid IDs, missing args) | scheduled workflow |
| `TestLiveReadSideSchemaDiff` | raw Clockify JSON vs `models.go` structs | scheduled workflow |
| `TestE2EMutating` | `clockify_create_client`, `clockify_create_project`, `clockify_start_timer`, `clockify_stop_timer`, `clockify_delete_entry` | scheduled workflow (requires `CLOCKIFY_LIVE_WRITE_ENABLED=true`) |
| `TestLiveDryRunDoesNotMutate` | `clockify_delete_entry` (dry-run) | scheduled workflow (requires write) |
| `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` | `clockify_create_project` (policy block) | scheduled workflow (requires write) |
| `TestLiveCreateUpdateDeleteEntryAuditPhases` | MCP-path + Postgres audit | scheduled workflow (requires write + `MCP_LIVE_CONTROL_PLANE_DSN`) |

### Manual sacrificial-workspace evidence (campaign expansion, NOT scheduled cron)

The tests below are gated by `//go:build livee2e` and ship in this
repo, but the cron workflow's `-run` regex is anchored and does not
include them by design — they are local-only / manual-dispatch-only
until the maintainer reviews each surface and chooses to add them.
This is a deliberate blast-radius decision: cron-driving these tests
would mail every night where the upstream surface allows it. Per
AGENTS.md:114-118 these tests do not constitute launch-readiness
evidence; they are coverage-expansion artefacts that quantify the
surface and surface latent handler / upstream bugs.

| Test | Tools / surface exercised | Outcome shape |
|------|----------------------------|---------------|
| `TestLiveTier1ReadOnly` | 15 Tier-1 read-only tools that lacked live evidence: `list_workspaces`, `list_users`, `current_user`, `list_tags`, `list_tasks`, `today_entries`, `summary_report`, `weekly_summary`, `quick_report`, `timesheet_review`, `timer_status`, `detailed_report`, `resolve_name`, `resolve_debug`, `policy_info` | success path |
| `TestLiveTier2ReadOnlySweep` | 22 Tier-2 read-only and report tools across 11 groups | success path for the current list/report surface |
| `TestLiveT2SchedulingRecurringCRUD` | `create_assignment`, `get_assignment` via list scan, `update_assignment`, `delete_assignment` on recurring-assignment routes | success path |
| `TestLiveT2ExpensesCRUD` | `create_expense_category`, `update_expense_category`, `create_expense`, `get_expense` bogus-id handling; delete-category archive constraint remains pinned | mixed success / documented upstream constraint |
| `TestLiveT2CustomFieldsCRUD` | `seed_project` works; `create_custom_field` and downstream tests cap-skipped at the upstream's 50-field-per-workspace limit | success on seed; cap-skipped on field tools |
| `TestLiveT2GroupsHolidaysCRUD` | `create_user_group_admin`, `get_user_group` via list scan, `update_user_group_admin`, `delete_user_group_admin` (real + dry-run), `create_holiday`, `delete_holiday` bogus-id handling | success path / documented not-found handling |
| `TestLiveT2ProjectAdminCRUD` | `seed_project`, `create_project_template`, `get_project_template`, `update_project_estimate`, `set_project_memberships`, `archive_projects` | success path |
| `TestLivePolicyModes` | `clockify_create_client` parametrised over all 5 policy modes (`read_only`, `time_tracking_safe`, `safe_core`, `standard`, `full`) | gate behaviour pinned |
| `TestLivePaginationOnTags` | `clockify_list_tags` pagination meta envelope; `clockify_create_tag` (Tier 1) seeded × 11 | success path |
| `TestLiveTier1RemainingCRUD` | remaining Tier-1 tool names not covered by the first campaign: `get_project`, `list_clients`, `list_entries`, `create_task`, `log_time`, `update_entry`, `find_and_update_entry`, `timesheet_fill_gap`, `switch_project`, `stop_timer`, `search_tools` | success path |
| `TestLiveT2InvoicesCRUD` | all 12 invoice tools: invoice create/list/get/update/delete, invoice report, item add/list/delete, send dry-run, mark-paid dry-run + real; update-item route asserted as unsupported on this Clockify version | success path + documented unsupported `PUT /items/{order}` 405 |
| `TestLiveT2SharedReportsCRUDAndExports` | all 6 shared-report tools: SUMMARY create/get/update/export JSON/PDF/delete; DETAILED/WEEKLY JSON export when workspace fixtures exist | success path / conditional export breadth |
| `TestLiveT2UserAdminCRUDAndOwnerSafety` | all 8 user-admin tools: user-group create/update/add/remove/delete, remove dry-run, deactivate owner dry-run, update-role unsupported route probe | success path + documented unsupported role route 405 |
| `TestLiveT2UserInviteValidationProbe` | raw Clockify invite-user route `POST /workspaces/{workspaceId}/users?send-email=false`; no catalog tool exposed | validation / plan / permission / unsupported-method refusal, with no email sent |
| `TestLiveT2WebhooksCRUD` | all 7 webhook tools: create/get/update/list-events/test dry-run/delete dry-run/delete using live `webhookEvent` + trigger-source body shape | success path |
| `TestLiveT2TimeOffRemainingTools` | all 12 time-off tool names: policy/request list/get/create/update/delete/status/balance paths; request create reaches live body contract and unsupported/permission routes are asserted where Clockify rejects the operation | success path + plan/permission/unsupported-route probes |
| `TestLiveT2ApprovalsRemainingTools` | all 6 approvals tool names: list, submit period/periodStart, get, approve/reject dry-run, withdraw | success path + documented approval period / GET-route 4xx probes |

**Live-test coverage (manual campaign expansion + hooks):** the
API-backed catalog surface is named in `tests/e2e_live*.go`; the
current 128-tool catalog is 40 Tier 1 tools + 88 Tier 2 tools, with
four local discovery/activation helpers covered by unit tests instead
of live Clockify calls. `scripts/check-live-tool-coverage.sh` is the
static guard for this inventory: it fails when a Tier-2 catalog tool or
API-backed Tier-1 tool is not named in the livee2e source bundle, while
explicitly allowing local-only Tier-1 catalog/tool-surface helpers that
should not pretend to call Clockify. PR #59 manually exercised the then-current
121 generated catalog tools through the MCP path against the
sacrificial workspace; PR #62 added the invite-user raw-route
validation probe. The two later timesheet workflow helpers are covered
by unit tests and live-test hooks, but have not by themselves been
promoted into launch evidence.
That does **not** mean every tool has a live success path: the tests
distinguish successful CRUD from concrete upstream constraints such as
expense-category archive-before-delete, custom-field workspace caps,
invoice-item update 405, user-role update 405, time-off permission /
unsupported routes, and approval-period or approval GET-route 4xx
responses.

### API contract fixes closed by the campaign (May 2026)

The original campaign surfaced 13 handler or descriptor mismatches.
They are now closed in code and test coverage except where noted as an
upstream limitation:

1. Invoice list/report envelopes and invoice-item embedding are handled.
2. Expense list/category envelopes and multipart create/update are
   handled; category deletion still requires UI-only archival upstream.
3. Webhook list envelopes are handled; event listing returns the static
   event enum instead of probing a non-workspace route.
4. Shared reports use `reports.api.clockify.me/v1`, `pageSize`,
   `type`/`filter` body keys, workspace-prefixed write/delete routes,
   bare-id GET/export, and binary-aware export envelopes.
5. Scheduling list/capacity/project totals use the live assignment
   paths; recurring-assignment create/update/delete now use
   `/scheduling/assignments/recurring`.
6. Time-off request listing uses the POST search envelope.
7. User-group lookup scans the supported list endpoint because
   per-id GET returns 405 live.
8. Holiday create uses `datePeriod`, `occursAnnually`, and required
   user/user-group assignment envelopes.
9. Project memberships use PATCH replace semantics and return the
   updated memberships from the full project object.
10. Custom-field descriptors advertise the live enum values.
11. Invoice create/update carry live-required `number` and RFC3339
    `issuedDate`; mark-paid carries forward the required invoice
    fields before setting `status=PAID`; invoice items use
    workspace item type names, `applyTaxes`, and line-order path
    parameters. `update_invoice_item` is live-pinned as unsupported
    (`PUT /items/{order}` returns 405 on this Clockify version).
12. Expense update now fetches and carries forward live-required
    multipart fields (`userId`, `date`, `amount`, `categoryId`,
    `billable`) before applying the requested `change_fields`.
13. Webhook create/update use the live `webhookEvent`,
    `triggerSourceType`, and `triggerSource` body shape rather than
    the old plural `events` array. Webhook names are constrained to
    2-30 characters.
14. Time-off request create uses nested `timeOffPeriod.period` with
    `start`, `end`, and computed inclusive `days`; request status
    changes use `PATCH /time-off/policies/{policyId}/requests/{id}`
    with `status=APPROVED|REJECTED`.
15. Approval submit uses the documented `period` + `periodStart`
    body. The sacrificial workspace currently returns
    "Approval period has changed..." for the probed weekly submit,
    and per-id approval GET is pinned as 405 unsupported.

### Workspace-state findings

- **Custom-field cap.** Clockify enforces a 50-field-per-workspace
  cap. `TestLiveT2CustomFieldsCRUD` t.Skip()s when the cap is hit.
  The sacrificial workspace was at 50/50 at the time of campaign
  authoring; pruning is required before the test can run.
- **Archive-before-delete is the only path** for projects, clients,
  and expense categories on this Clockify version. Tags and user
  groups accept DELETE directly. The harness provides
  `rawArchiveAndDeleteProject` and `rawArchiveAndDeleteClient`
  cleanup primitives; raw `DELETE` works for tags and user groups.

---

## Gaps

1. **Full-success live coverage:** the current 128-tool catalog has a
   manual probe, live-test hook, or local unit coverage path, but some
   API-backed tools remain asserted as upstream unsupported,
   permission-gated, plan-gated, or workspace-state limited rather
   than full success paths. The original exhaustive manual probe
   evidence covered the then-current 121-tool catalog; later workflow
   and local helper additions should be included in a future
   current-catalog campaign if the maintainer wants refreshed live
   coverage wording.
2. **Schema-drift for mutating endpoints:** Only read-side schemas are
   drift-checked. Request payload schemas (tool JSON Schema descriptors)
   are not automatically compared against the live API's accepted fields.
3. **Scheduled evidence breadth:** the exhaustive tool probes are
   manual/local artefacts. The scheduled cron intentionally keeps a
   narrower `-run` regex until the maintainer separately approves
   nightly blast radius for admin/billing/webhook/time-off/approval
   side effects.
4. **Dry-run semantics:** destructive dry-run is broader than before,
   but not every destructive tool exposes a dry-run preview route.
   Several tools rely on minimal dry-run envelopes or upstream 4xx
   constraints because Clockify lacks a safe preview endpoint.
5. **Rate-limit behaviour:** No automated tests exercise Clockify's rate
   limiter or the MCP server's retry/backoff behaviour under live load.
6. **Pagination consistency:** `clockify_list_entries`, `clockify_list_projects`,
   and similar paginated endpoints are not live-tested for page-boundary
   correctness.

## Recommended next tests (safe, in priority order)

1. Promote only the low-blast-radius portions of the exhaustive manual
   suite into `live-contract.yml` after separate cron review.
2. Schema-drift test extension to mutating endpoint request schemas.
3. Dedicated pagination boundary tests for the list endpoints whose
   page/page-size behaviour is still only unit-covered.
4. Optional rate-limit / retry-behaviour probe against a disposable
   workspace with explicit operator approval.

## Known API contract notes

Probe-lab evidence (`clockify-api-probe-lab/findings/`) raised a small
number of numeric / shape questions during the live campaign. The
handlers intentionally pass through upstream money values as raw API
numbers; no cents-to-decimal or decimal-to-cents conversion is applied
inside go-clockify.

### invoice line-item amounts (raw minor units)

- **Source:** `clockify-api-probe-lab/findings/invoices.md` lines
  77–95 and the open-questions section.
- **Observation:** `GET /workspaces/{ws}/invoices/{id}` returns each
  line item as `{ quantity, unitPrice, amount, ... }`. In one probed
  fixture, `unitPrice == 100000` and `quantity == 100` produced a row
  the workspace billed as `$1,000.00`, suggesting `unitPrice` is in
  integer minor units (cents). The list-invoice envelope's top-level
  `amount`, `paid`, `balance` fields use the same `<integer cents>`
  notation in the probe write-up.
- **Status in go-clockify:** `clockify_get_invoice`,
  `clockify_list_invoices`, `clockify_list_invoice_items`,
  `clockify_add_invoice_item`, and `clockify_update_invoice_item`
  surface/send these numbers verbatim. The descriptors now call this
  out as a raw upstream value; the live API uses minor units/cents for
  invoice item `unitPrice`.

### expense `amount` vs `total` scaling (raw pass-through)

- **Source:** `clockify-api-probe-lab/findings/expenses.md`
  lines 155–161 and open-question #1.
- **Observation:** `POST /workspaces/{ws}/expenses` was probed with
  `amount=100` (multipart form field). The response surfaced
  `total: 10000.0`. Two plausible interpretations: (a) the request
  `amount` is in major units and the response `total` is in minor
  units (×100 scaling); (b) `amount` is a multiplier against a
  workspace default rate. The probe could not distinguish these
  without more workspaces.
- **Status in go-clockify:** `clockify_create_expense` accepts
  whatever the caller passes for `amount`; `clockify_update_expense`
  carries the existing required amount forward unless the caller
  explicitly changes it; `clockify_list_expenses` and
  `clockify_get_expense` return upstream totals verbatim. The
  descriptor now describes `amount` as a raw upstream value with no
  client-side scaling.

### expense `projectId` optional-vs-required (unresolved)

- **Source:** `clockify-api-probe-lab/findings/expenses.md` lines
  44–47 (note in the probe), 141 (response sample), and
  open-question #2.
- **Observation:** Clockify's published OpenAPI marks `projectId` as
  required for `POST /workspaces/{ws}/expenses`. The probe omitted
  it and still received `201` with `projectId: null` in the
  response. The probe could not tell whether the workspace had a
  default project, whether the field is silently nullable, or
  whether enforcement varies by plan.
- **Status in go-clockify:** the `clockify_create_expense`
  descriptor still marks `project_id` as required (matching the
  documented contract). PR #53 did not flip this.
- **Open question (low priority):** if Clockify confirms `projectId`
  is optional, drop it from the descriptor's `required` list. Until
  then, the conservative descriptor stays in place — callers can
  always pass a project ID even if the upstream would have accepted
  none.

### shared-reports cross-type / non-summary filter requirements (unresolved)

- **Source:** `clockify-api-probe-lab/findings/SUMMARY.md` open
  questions #7, #8, #9 (rev 4, 2026-05-03).
- **Observation:** the probe lab proved `createSharedReport` and
  `updateSharedReport` body shape only for `type=SUMMARY`. The
  remaining 18 enum values (`DETAILED`, `EXPENSE_DETAILED`,
  `INVOICE_TIME`, `KIOSK_PIN_LIST`, …) and the cross-type validation
  semantics on `PUT` are untested. Whether `PUT` of a full bare-id
  GET response (with nested `workspace.workspaceSettings`)
  round-trips is also untested.
- **Status in go-clockify:** PR #56 fixed body field names
  (`type`/`filter`) and pinned them with dispatcher tests. The
  descriptor exposes the broader enum verbatim. Callers using a
  non-`SUMMARY` type will see whatever 4xx the upstream returns if
  the required filter sub-object is wrong.
- **Open question (low priority):** probe the per-type filter
  requirements before adding any client-side validation. The
  descriptor should not over-promise validation it can't enforce.

### scheduling `capacityPerDay` unit (resolved as raw upstream value)

- **Source:** `clockify-api-probe-lab/findings/scheduling.md`
  open question #4.
- **Resolution:** the unit is **seconds** per the live response
  (`25200 = 7 hr/day` default; `3600 = 1 hr/day` workspace
  override). PR #55 already attaches `capacityUnit: "seconds"` to
  the `clockify_filter_schedule_capacity` meta envelope. Recording
  the resolution here so the open-question list does not relitigate
  it.

### Where to look next

- Per-domain probe summary: `clockify-api-probe-lab/findings/SUMMARY.md`
- Per-domain fixtures: `clockify-api-probe-lab/fixtures/<domain>/`
- Official Clockify docs (probe-lab mirror): `clockify-api-probe-lab/<DOMAIN>DOC.md`

The probe lab is a separate workspace; nothing under it is
committed into go-clockify.

## Evidence authority

| Source | Evidentiary weight |
|--------|--------------------|
| `go test ./...` (local) | Necessary — must be green before every commit |
| `make release-check` (local) | Necessary — must be green before any push |
| `go test -tags=livee2e ./tests/...` (local, no env vars) | **Non-evidence** — every test silently skips; `TestLiveContractSkipSentinel` now fails explicitly in this case |
| `go test -tags=livee2e ./tests/...` (local, with env vars) | Advisory only — demonstrates the test logic is sound but does not constitute launch-gate evidence |
| `.github/workflows/ci.yml` (PR) | Authoritative for unit/integration tests |
| `.github/workflows/live-contract.yml` (manual dispatch) | Strong evidence — one-time verification |
| `.github/workflows/live-contract.yml` (scheduled cron) | **Authoritative** — the only evidence that counts for Group 1 launch-gate checkboxes |

---

*Tool names and classification counts verified against `docs/tool-catalog.json` on 2026-05-03. Re-run verification after `make gen-tool-catalog`.*
