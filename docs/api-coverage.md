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
| Read-only | 27 | 33 | 60 |
| Mutating (non-destructive) | 20 | 50 | 70 |
| Destructive | 5 | 13 | 18 |
| Billing | 0 | 12 | 12 |
| Admin | 0 | 12 | 12 |
| **Total tools** | **52** | **96** | **148** |

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

## Tier 1 — Core tools (52)

The per-tool tables below list the stable local test coverage that
ships with normal CI. The manual sacrificial-workspace section later
in this document records the exhaustive live-probe layer added on top
of these unit/mocked tests.

Clockify endpoints: `GET/POST/PUT/PATCH/DELETE /workspaces/{ws}/time-entries`,
`/workspaces/{ws}/projects`, `/workspaces/{ws}/clients`,
`/workspaces/{ws}/tags`, `/workspaces/{ws}/tasks`,
`/workspaces/{ws}/users`, `/workspaces/{ws}/reports/*`,
`/user`, `/workspaces`.

### Read-only (27 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_attendance_report` | `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/attendance` | unit, live-doc-coverage (`TestLiveReportsDocCoverage`) |
| `clockify_current_user` | `GET /user` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_detailed_report` | `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/detailed` | unit, live-doc-coverage (`TestLiveReportsDocCoverage`) |
| `clockify_get_client` | `GET /workspaces/{ws}/clients/{id}` | unit |
| `clockify_get_entry` | `GET /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_get_project` | `GET /workspaces/{ws}/projects/{id}` | unit |
| `clockify_get_tag` | `GET /workspaces/{ws}/tags/{id}` | unit |
| `clockify_get_task` | `GET /workspaces/{ws}/projects/{id}/tasks/{tid}` | unit |
| `clockify_get_workspace` | `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_clients` | `GET /workspaces/{ws}/clients` | unit |
| `clockify_list_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` | unit |
| `clockify_list_projects` | `GET /workspaces/{ws}/projects` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_tags` | `GET /workspaces/{ws}/tags` | unit |
| `clockify_list_tasks` | `GET /workspaces/{ws}/projects/{id}/tasks` | unit |
| `clockify_list_tools` | local (catalog query) | unit |
| `clockify_list_users` | `GET /workspaces/{ws}/users` | unit |
| `clockify_list_workspaces` | `GET /workspaces` | unit |
| `clockify_policy_info` | local (no API call) | unit |
| `clockify_quick_report` | wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries`) | unit |
| `clockify_resolve_debug` | compatibility alias over name resolution lookup | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_resolve_name` | name resolution lookup over project/client/tag/user list endpoints | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_summary_report` | `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/summary` | unit, live-doc-coverage (`TestLiveReportsDocCoverage`) |
| `clockify_timer_status` | `GET /workspaces/{ws}/user/{uid}/time-entries?in-progress=true` | unit |
| `clockify_timesheet_review` | workflow wrapper over `GET /workspaces/{ws}/user/{uid}/time-entries` | unit, live-read-only (TestLiveTier1ReadOnly) |
| `clockify_today_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` (filtered) | unit |
| `clockify_weekly_summary` | `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/weekly` | unit, live-doc-coverage (`TestLiveReportsDocCoverage`) |
| `clockify_whoami` | `GET /user` + `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |

### Reports API document ledger

Source docs: `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/ATTENDANCEANDTIMEREPORTS.md` plus the expense detailed report excerpt. MCP inputs use snake_case and the handlers emit the upstream camelCase JSON body. All rows return the upstream JSON object directly in `data` with `meta.source="reports-api"` and `meta.workspaceId`.

| Document row | Upstream endpoint | MCP tool / required filter | Documented params and enums covered | Example values covered | Unit coverage | Live evidence |
|--------------|-------------------|----------------------------|-------------------------------------|------------------------|---------------|---------------|
| Attendance report | `POST /workspaces/{workspaceId}/reports/attendance` | `clockify_attendance_report`; requires `attendance_filter` only | Common report fields; `attendance_filter.page`, `page_size`, `sort_column`, `break_filters`, `capacity_filters`, `end_filters`, `overtime_filters`, `start_filters`, `work_filters`, `has_time_off`; sort enum `USER`, `DATE`, `START`, `END`, `BREAK`, `WORK`, `CAPACITY`, `OVERTIME`, `TIME_OFF`; compare enum `EXACTLY`, `LARGER_THAN`, `SMALLER_THAN` | `start`, `end`, `attendance_filter.start_filters[].filtration_type=LARGER_THAN`, `value=00:00` | `TestAttendanceReportUsesReportsAPI`; `TestReportToolSchemasExposeOnlyTheirDocumentedFilters` | `TestLiveReportsDocCoverage`; planning probe returned `200` with `entities` |
| Detailed report | `POST /workspaces/{workspaceId}/reports/detailed` | `clockify_detailed_report`; requires `detailed_filter` only | Common report fields; `detailed_filter.page`, `page_size`, `sort_column`, `audit_filter`, `options`; sort enum `ID`, `DESCRIPTION`, `USER`, `DURATION`, `DATE`, `ZONED_DATE`, `NATURAL`, `USER_DATE`; `options.totals=CALCULATE|EXCLUDE`; audit booleans; `amount_shown`/`amounts` `EARNED`, `COST`, `PROFIT`, `HIDE_AMOUNT`, `EXPORT` | `amount_shown=PROFIT`, `amounts=[EARNED,COST,PROFIT]`, `options.totals=CALCULATE`, `audit_filter.without_task=true` | `TestDetailedReportUsesReportsAPI`; `TestReportToolSchemasExposeDocumentedEnumsAndAliases` | `TestLiveReportsDocCoverage`; live enum probe accepted `EARNED`, `COST`, `PROFIT`, `HIDE_AMOUNT`, `EXPORT` |
| Summary report | `POST /workspaces/{workspaceId}/reports/summary` | `clockify_summary_report`; requires `summary_filter` only | Common report fields; `summary_filter.groups` 1-3 levels, sort/chart; groups exposed as `CLIENT`, `PROJECT`, `DAY`, `WEEK`, `MONTH`, `TIMEENTRY`, `TASK`; MCP `DAY` translates to upstream `DATE`; sort enum `GROUP`, `DURATION`, `AMOUNT`, `EARNED`, `COST`, `PROFIT`; chart enum `BILLABILITY`, `PROJECT`; `amount_shown`/`amounts` enums as documented | `groups=[CLIENT,PROJECT,DAY]`, `sort_column=PROFIT`, `summary_chart_type=PROJECT` | `TestSummaryReportUsesReportsAPI`; `TestSummaryReportGroupAliasBuilder` | `TestLiveReportsDocCoverage`; planning probe confirmed upstream `DATE`; upstream rejected literal `DAY` with `400`, so `DAY` is documented as an MCP alias |
| Weekly report | `POST /workspaces/{workspaceId}/reports/weekly` | `clockify_weekly_summary`; requires `weekly_filter` only | Common report fields; exact 7-day range required or derived from `week_start`; `weekly_filter.group=PROJECT|USER`; `weekly_filter.subgroup=TIME`; no summary/detailed/attendance filters | `week_start=2026-04-06`, `weekly_filter.group=PROJECT`, `subgroup=TIME` | `TestWeeklySummaryUsesReportsAPIAndDerivesWeekRange`; `TestWeeklyReportRejectsInvalidSubgroup` | `TestLiveReportsDocCoverage`; planning probe returned `200` for `PROJECT/TIME` and `USER/TIME`; invalid group/subgroup returned `400` |
| Expense detailed report | `POST /workspaces/{workspaceId}/reports/expenses/detailed` | `clockify_expense_report`; no time-report filter object | `approval_state`, `billable`, `categories`, `clients`, `currency`, `date_range_start`, `date_range_end`, `date_range_type`, `export_type`, `invoicing_state`, `note`, `page`, `page_size`, `projects`, `sort_column`, `sort_order`, `tasks`, `time_zone`, `user_groups`, `user_locale`, `users`, `week_start_day`, `without_note`, `zoom_level`; sort enum `ID`, `PROJECT`, `USER`, `CATEGORY`, `DATE`, `AMOUNT`; contains/status/user-status/date-range/export/week/zoom enums | `page=1`, `page_size=25`, `sort_column=ID`, `projects.contains=CONTAINS` | `TestExpenseReport`; `TestExpenseReportSchemaCoversDetailedExpenseReportBody` | `TestLiveReportsDocCoverage`; planning probe returned `200` with `expenses` and `totals` |

### Mutating — non-destructive (20 tools)

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
| `clockify_log_time` | `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_search_tools` | local (deprecated catalog/activation shim) | unit |
| `clockify_start_timer` | `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_stop_timer` | `PATCH /workspaces/{ws}/user/{uid}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_switch_project` | `PATCH` + `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_timesheet_fill_gap` | `GET` overlap validation + `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestLiveTier1RemainingCRUD) |
| `clockify_update_client` | `GET` + `PUT /workspaces/{ws}/clients/{id}` (fetch-then-merge) | unit |
| `clockify_update_entry` | `GET` + `PUT /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_update_project` | `GET` + `PUT /workspaces/{ws}/projects/{id}` (fetch-then-merge) | unit |
| `clockify_update_tag` | `GET` + `PUT /workspaces/{ws}/tags/{id}` (fetch-then-merge) | unit |
| `clockify_update_task` | `GET` + `PUT /workspaces/{ws}/projects/{id}/tasks/{tid}` (fetch-then-merge) | unit |

### Destructive (5 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_delete_client` | `GET` + `PUT {archived:true}` + `DELETE /workspaces/{ws}/clients/{id}` | unit, dry-run |
| `clockify_delete_entry` | `DELETE /workspaces/{ws}/time-entries/{id}` | unit, dry-run (TestLiveDryRunDoesNotMutate), sacrificial-mutating (TestE2EMutating) |
| `clockify_delete_project` | `GET` + `PUT {archived:true}` + `DELETE /workspaces/{ws}/projects/{id}` | unit, dry-run |
| `clockify_delete_tag` | `GET` + `DELETE /workspaces/{ws}/tags/{id}` | unit, dry-run |
| `clockify_delete_task` | `GET` + `DELETE /workspaces/{ws}/projects/{id}/tasks/{tid}` | unit, dry-run |

---

## Tier 2 — Domain groups (96 tools)

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

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/expenses/*`;
expense detailed report uses `POST https://reports.api.clockify.me/v1/workspaces/{ws}/reports/expenses/detailed`.

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_expense` | mutating | unit |
| `clockify_create_expense_category` | mutating | unit |
| `clockify_delete_expense` | destructive | unit |
| `clockify_delete_expense_category` | destructive | unit |
| `clockify_expense_report` | read-only | unit, live-doc-coverage (`TestLiveReportsDocCoverage`) |
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

### `project_admin` (14 tools)

Clockify endpoints: `GET/POST/PUT/PATCH/DELETE /workspaces/{ws}/projects/*`, `/workspaces/{ws}/project-templates/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_archive_projects` | mutating | unit |
| `clockify_assign_project_memberships` | mutating, admin, permission_change | unit, live-probed (`TestLiveProjectAdminDocCoverage`) |
| `clockify_create_project_from_template` | mutating | unit, live-probed (`TestLiveProjectAdminDocCoverage`; upstream may return permission/plan 403) |
| `clockify_create_project_template` | mutating | unit |
| `clockify_get_project_template` | read-only | unit |
| `clockify_list_project_templates` | read-only | unit |
| `clockify_set_project_memberships` | mutating | unit |
| `clockify_update_project_estimate` | mutating | unit |
| `clockify_update_project_memberships` | mutating, admin, permission_change | unit, live-probed (`TestLiveProjectAdminDocCoverage`) |
| `clockify_update_project_template` | mutating | unit, live-probed (`TestLiveProjectAdminDocCoverage`) |
| `clockify_update_project_user_cost_rate` | mutating, billing, admin | unit, live-probed (`TestLiveProjectAdminDocCoverage`; upstream may return permission/plan 4xx) |
| `clockify_update_project_user_hourly_rate` | mutating, billing, admin | unit, live-probed (`TestLiveProjectAdminDocCoverage`; upstream may return permission/plan 4xx) |
| `clockify_update_task_cost_rate` | mutating, billing | unit, live-probed (`TestLiveProjectAdminDocCoverage`; upstream may return permission/plan 4xx) |
| `clockify_update_task_hourly_rate` | mutating, billing | unit, live-probed (`TestLiveProjectAdminDocCoverage`; upstream may return permission/plan 4xx) |

### Client / Project / Task Doc Parity Ledger (2026-05-12)

Source docs: `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLIENTSDOC.md`, `PROJECTSDOC.md`, and `TASKDOC.md`. MCP inputs stay snake_case; handlers translate to Clockify camelCase body fields and hyphenated query parameters. Rate amounts are forwarded as raw upstream integers with no currency scaling.

Live evidence command run locally against the confirmed sacrificial workspace on 2026-05-12:

```bash
set -a; . /tmp/clockify-livetest.env; set +a; export CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true CLOCKIFY_LIVE_WRITE_ENABLED=true CLOCKIFY_LIVE_ADMIN_ENABLED=true CLOCKIFY_LIVE_BILLING_ENABLED=true; go test -tags=livee2e -run 'TestLive.*(ClientProjectTask|ProjectAdmin).*DocCoverage' ./tests/...
```

The Tier 1 `TestLiveClientProjectTaskDocCoverage` path succeeded end-to-end. The project-admin doc test reaches every new admin/rate route; documented routes that Clockify refuses for the sacrificial account are accepted only when the upstream response is an explicit permission/plan/membership refusal and are logged by the test.

#### Client Rows

| Doc row | Endpoint | Parameters / fields / examples accounted for | MCP surface | Unit and live evidence |
|---------|----------|-----------------------------------------------|-------------|------------------------|
| Find clients | `GET /workspaces/{workspaceId}/clients` | `workspaceId` example `64a687e29ae1f428e7ebe303`; query `name=Client X`, `sort-column=NAME`, `sort-order=ASCENDING`, `page=1`, `page-size=50`, `archived` string filter | `clockify_list_clients`: `name`, `sort_column`, `sort_order`, `page`, `page_size`, `archived` | `TestClientDocListFiltersForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Add client | `POST /workspaces/{workspaceId}/clients` | Body `address`, `email`, `name`, `note`; examples use Palo Alto address, `clientx@example.com`, `Client X`, sample note | `clockify_create_client`: `address`, `email`, `name`, `note` | `TestCreateClientForwardsAddressEmailNote`; live `TestLiveClientProjectTaskDocCoverage` |
| Delete client | `DELETE /workspaces/{workspaceId}/clients/{id}` | Path `id` example `44a687e29ae1f428e7ebe305`; response fields `address`, `archived`, `ccEmails`, `email`, `id`, `name`, `note`, `workspaceId` | `clockify_delete_client`: `client`; archives active clients before DELETE | `TestDeleteClientArchivesActiveClient`; live `TestLiveClientProjectTaskDocCoverage` |
| Get client | `GET /workspaces/{workspaceId}/clients/{id}` | Path `id`; response fields `address`, `archived`, `ccEmails`, `currencyCode`, `currencyId`, `email`, `id`, `name`, `note`, `workspaceId` | `clockify_get_client`: `client` | `TestGetClientByID`; live `TestLiveClientProjectTaskDocCoverage` |
| Update client | `PUT /workspaces/{workspaceId}/clients/{id}` | Query `archive-projects`, `mark-tasks-as-done`; body `address`, `archived`, `ccEmails`, `currencyId`, `email`, `name`, `note`; examples include `user@example.com` and `53a687e29ae1f428e7ebe888` | `clockify_update_client`: `archive_projects`, `mark_tasks_as_done`, `cc_emails`, `currency_id`, plus existing merge fields | `TestUpdateClientDocFieldsAndArchiveQueryForwarded`; live `TestLiveClientProjectTaskDocCoverage` (`currency_id` used when workspace currency id is discoverable) |

#### Project Rows

| Doc row | Endpoint | Parameters / fields / examples accounted for | MCP surface | Unit and live evidence |
|---------|----------|-----------------------------------------------|-------------|------------------------|
| Find projects | `GET /workspaces/{workspaceId}/projects` | Query `name`, `strict-name-search`, `archived`, `billable`, repeated `clients`, `contains-client`, `client-status` enum `ACTIVE`/`ARCHIVED`/`ALL`, repeated `users`, `contains-user`, `user-status` enum `PENDING`/`ACTIVE`/`DECLINED`/`INACTIVE`/`ALL`, `is-template`, `sort-column` enum `ID`/`NAME`/`CLIENT_NAME`/`DURATION`/`BUDGET`/`PROGRESS`, `sort-order` enum `ASCENDING`/`DESCENDING`, `hydrated`, `access` enum `PUBLIC`/`PRIVATE`, `expense-limit`, `expense-date`, repeated `userGroups`, `contains-group`, `page`, `page-size` | `clockify_list_projects`: same names in snake_case, repeated lists encoded as repeated upstream query keys | `TestProjectDocListFiltersForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Add project | `POST /workspaces/{workspaceId}/projects` | Body `name`, `clientId`, `color`, `billable`, `isPublic`, `note`, `costRate.amount/since`, `hourlyRate.amount/since`, `estimate.estimate/type` (`AUTO`, `MANUAL`), `budgetEstimate.active/estimate/includeExpenses/resetOption/type`, `timeEstimate.active/estimate/includeNonBillable/resetOption/type`, `memberships`, `tasks`; examples include `PT1H30M` and raw rate amounts | `clockify_create_project`: `client_id` plus existing `client`, `cost_rate`, `hourly_rate`, `estimate`, `budget_estimate`, `time_estimate`, `memberships`, `tasks` | `TestProjectDocCreateRichBodyForwarded`; live safe-field path in `TestLiveClientProjectTaskDocCoverage` |
| Create project from template | `POST /workspaces/{workspaceId}/projects/from-template` | Body `name`, `templateProjectId`, optional `clientId`, `color`, `isPublic` | `clockify_create_project_from_template`: `name`, `template_project_id`, `client_id`, `color`, `is_public` | `TestTier2Dispatch_ProjectAdmin_CreateProjectFromTemplate`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Delete project | `DELETE /workspaces/{workspaceId}/projects/{projectId}` | Path `projectId`; active projects must be archived first in this MCP | `clockify_delete_project`: `project`; `clockify_archive_projects` remains bulk archive helper | `TestDeleteProjectArchivesActiveProject`; live `TestLiveClientProjectTaskDocCoverage` |
| Get project | `GET /workspaces/{workspaceId}/projects/{projectId}` | Path `projectId`; response fields include `archived`, `billable`, `budgetEstimate`, `clientId`, `clientName`, `color`, `costRate`, `duration`, `estimate`, `estimateReset`, `hourlyRate`, `id`, `memberships`, `name`, `note`, `public`, `template`, `timeEstimate`, `workspaceId` | `clockify_get_project`: `project` | `TestUpdateProjectFetchThenMerge` and existing get coverage; live `TestLiveClientProjectTaskDocCoverage` |
| Update project | `PUT /workspaces/{workspaceId}/projects/{projectId}` | Full body merge supports `name`, `clientId`, `color`, `archived`, `billable`, `isPublic`, `note`, rates, estimates, memberships, tasks, `estimateReset` | `clockify_update_project`: `client_id`, `cost_rate`, `hourly_rate`, `estimate`, `budget_estimate`, `estimate_reset`, `time_estimate`, `memberships`, `tasks` | `TestProjectDocUpdateRichBodyForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Update project estimate | `PATCH /workspaces/{workspaceId}/projects/{projectId}/estimate` | Body `budgetEstimate`, `estimateReset`, `timeEstimate`; enums `AUTO`/`MANUAL`, `WEEKLY`/`MONTHLY`/`YEARLY`, weekdays/months; legacy `estimate_type`/`estimate_value` remain aliases | `clockify_update_project_estimate`: `budget_estimate`, `estimate_reset`, `time_estimate`, legacy aliases | `TestTier2Dispatch_ProjectAdmin_UpdateProjectEstimate`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Update project memberships | `PATCH /workspaces/{workspaceId}/projects/{projectId}/memberships` | Body `memberships[]` with `userId`, `hourlyRate`, `costRate`, `membershipStatus`, `membershipType`; `userGroups.ids/status/contains` with `CONTAINS`/`DOES_NOT_CONTAIN` and `ALL`/`ACTIVE`/`INACTIVE` | `clockify_update_project_memberships`; `clockify_set_project_memberships` is backward-compatible alias | `TestTier2Dispatch_ProjectAdmin_DocumentedMembershipAndTemplateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Assign/remove project memberships | `POST /workspaces/{workspaceId}/projects/{projectId}/memberships` | Body `remove`, `userIds`, `userGroups` with same `contains` and `status` enums | `clockify_assign_project_memberships`: `remove`, `user_ids`, `user_groups` | `TestTier2Dispatch_ProjectAdmin_DocumentedMembershipAndTemplateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Update project template | `PATCH /workspaces/{workspaceId}/projects/{projectId}/template` | Body `isTemplate` | `clockify_update_project_template`: `is_template` | `TestTier2Dispatch_ProjectAdmin_DocumentedMembershipAndTemplateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Update project user cost rate | `PUT /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate` | Body `amount` raw integer and optional `since` timestamp | `clockify_update_project_user_cost_rate`: `project_id`, `user_id`, `amount`, `since` | `TestTier2Dispatch_ProjectAdmin_DocumentedRateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Update project user billable rate | `PUT /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate` | Body `amount` raw integer and optional `since` timestamp | `clockify_update_project_user_hourly_rate`: `project_id`, `user_id`, `amount`, `since` | `TestTier2Dispatch_ProjectAdmin_DocumentedRateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |

#### Task Rows

| Doc row | Endpoint | Parameters / fields / examples accounted for | MCP surface | Unit and live evidence |
|---------|----------|-----------------------------------------------|-------------|------------------------|
| Find tasks | `GET /workspaces/{workspaceId}/projects/{projectId}/tasks` | Query `name`, `strict-name-search`, `is-active`, `sort-column` enum `ID`/`NAME`, `sort-order` enum `ASCENDING`/`DESCENDING`, `page`, `page-size` | `clockify_list_tasks`: `project`, `name`, `strict_name_search`, `is_active`, `sort_column`, `sort_order`, pagination | `TestTaskDocListFiltersForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Add task | `POST /workspaces/{workspaceId}/projects/{projectId}/tasks` | Query `contains-assignee`; body `assigneeId`, `assigneeIds`, `billable`, `budgetEstimate`, `estimate` example `PT1H30M`, `name`, `status` enum `ACTIVE`/`DONE`/`ALL`, `userGroupIds` | `clockify_create_task`: `project_id`/`project`, `contains_assignee`, `assignee_id`, `assignee_ids`, `budget_estimate`, `estimate`, `status`, `user_group_ids` | `TestTaskDocCreateAndUpdateFieldsForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Update task | `PUT /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}` | TASKDOC truncates the method/path line; MCP uses the established Clockify `PUT` route. Query `contains-assignee`, `membership-status`; body same task fields as create | `clockify_update_task`: `project`, `task`, `contains_assignee`, `membership_status`, assignee/budget/estimate/status/user-group fields | `TestTaskDocCreateAndUpdateFieldsForwarded`; live `TestLiveClientProjectTaskDocCoverage` |
| Update task cost rate | `PUT /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate` | Body `amount` raw integer and optional `since`; response returns task with `costRate` | `clockify_update_task_cost_rate`: `project_id`/`project`, `task_id`/`task`, `amount`, `since` | `TestTier2Dispatch_ProjectAdmin_DocumentedRateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Update task billable rate | `PUT /workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate` | Body `amount` raw integer and optional `since`; response returns task with `hourlyRate` | `clockify_update_task_hourly_rate`: `project_id`/`project`, `task_id`/`task`, `amount`, `since` | `TestTier2Dispatch_ProjectAdmin_DocumentedRateTools`; live-probed in `TestLiveProjectAdminDocCoverage` |
| Delete task | `DELETE /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}` | Path `projectId`, `taskId`; response echoes task fields | `clockify_delete_task`: `project`, `task` | `TestDeleteTaskDeletesDirectly`; live `TestLiveClientProjectTaskDocCoverage` |
| Get task | `GET /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}` | Path `projectId`, `taskId`; response fields `assigneeId`, `assigneeIds`, `billable`, `budgetEstimate`, `costRate`, `duration`, `estimate`, `hourlyRate`, `id`, `name`, `projectId`, `status`, `userGroupIds` | `clockify_get_task`: `project`, `task` | `TestGetTaskByID`; live `TestLiveClientProjectTaskDocCoverage` |

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
| `TestLiveTier1ReadOnly` | 16 Tier-1 read-only tools that lacked live evidence: `list_workspaces`, `list_users`, `current_user`, `list_tags`, `list_tasks`, `today_entries`, `summary_report`, `weekly_summary`, `attendance_report`, `quick_report`, `timesheet_review`, `timer_status`, `detailed_report`, `resolve_name`, `resolve_debug`, `policy_info` | success path |
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
| `TestLiveClientProjectTaskDocCoverage` | client/project/task doc rows from `CLIENTSDOC.md`, `PROJECTSDOC.md`, and `TASKDOC.md` through the MCP path | success path |
| `TestLiveProjectAdminDocCoverage` | project template clone, project template flag, project estimate/membership endpoints, project-user rates, task rates | success path where allowed; explicit upstream permission/plan refusals pinned |
| `TestLiveReportsDocCoverage` | ATTENDANCEANDTIMEREPORTS.md + expense detailed report rows: attendance, detailed, summary, weekly, expense detailed reports with documented filters and amount fields | success path where enabled; explicit upstream permission/plan refusals pinned |

**Live-test coverage (manual campaign expansion + hooks):** the
API-backed catalog surface is named in `tests/e2e_live*.go`; the
current 148-tool catalog is 52 Tier 1 tools + 96 Tier 2 tools, with
local discovery/activation/name-resolution helpers covered by unit
tests instead of live Clockify calls. `scripts/check-live-tool-coverage.sh` is the
static guard for this inventory: it fails when a Tier-2 catalog tool or
API-backed Tier-1 tool is not named in the livee2e source bundle, while
explicitly allowing local-only Tier-1 catalog/tool-surface helpers that
should not pretend to call Clockify. PR #59 manually exercised the then-current
121 generated catalog tools through the MCP path against the
sacrificial workspace; PR #62 added the invite-user raw-route
validation probe. Later timesheet workflow helpers and the client /
project / task plus Reports API doc-parity expansions are covered by
unit tests and live-test hooks, but have not by themselves been
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

*Tool names and classification counts verified against `docs/tool-catalog.json` on 2026-05-12. Re-run verification after `make gen-tool-catalog`.*
