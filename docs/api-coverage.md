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
| Read-only | 20 | 35 | 55 |
| Mutating (non-destructive) | 12 | 43 | 55 |
| Destructive | 1 | 13 | 14 |
| Billing | 0 | 8 | 8 |
| Admin | 0 | 7 | 7 |
| **Total tools** | **33** | **91** | **124** |

## Evidence types

| Type | Meaning | Used for |
|------|---------|----------|
| **local unit** | `go test` without external deps | Handler logic, schema validation, policy enforcement, dry-run |
| **mocked integration** | `go test` with httptest mocks | Client→API path, error mapping, retry behaviour |
| **live read-only** | `-tags=livee2e` read-only tier | Schema drift, auth flow, rate-limit behaviour |
| **sacrificial mutating** | `-tags=livee2e` with `CLOCKIFY_LIVE_WRITE_ENABLED=true` | Full CRUD, audit phases |
| **scheduled workflow** | `.github/workflows/live-contract.yml` cron | Authoritative evidence for launch gates |

---

## Tier 1 — Core tools (33)

Clockify endpoints: `GET/POST/PUT/PATCH/DELETE /workspaces/{ws}/time-entries`,
`/workspaces/{ws}/projects`, `/workspaces/{ws}/clients`,
`/workspaces/{ws}/tags`, `/workspaces/{ws}/tasks`,
`/workspaces/{ws}/users`, `/workspaces/{ws}/reports/*`,
`/user`, `/workspaces`.

### Read-only (20 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_current_user` | `GET /user` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_detailed_report` | `GET /workspaces/{ws}/reports/detailed` | unit |
| `clockify_get_entry` | `GET /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_get_project` | `GET /workspaces/{ws}/projects/{id}` | unit |
| `clockify_get_workspace` | `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_clients` | `GET /workspaces/{ws}/clients` | unit |
| `clockify_list_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` | unit |
| `clockify_list_projects` | `GET /workspaces/{ws}/projects` | unit, live-read-only (TestE2EReadOnly) |
| `clockify_list_tags` | `GET /workspaces/{ws}/tags` | unit |
| `clockify_list_tasks` | `GET /workspaces/{ws}/projects/{id}/tasks` | unit |
| `clockify_list_users` | `GET /workspaces/{ws}/users` | unit |
| `clockify_list_workspaces` | `GET /workspaces` | unit |
| `clockify_policy_info` | local (no API call) | unit |
| `clockify_quick_report` | `GET /workspaces/{ws}/reports/summary` (wrapped) | unit |
| `clockify_resolve_debug` | local (no API call) | unit |
| `clockify_summary_report` | `GET /workspaces/{ws}/reports/summary` | unit |
| `clockify_timer_status` | `GET /workspaces/{ws}/user/{uid}/time-entries?in-progress=true` | unit |
| `clockify_today_entries` | `GET /workspaces/{ws}/user/{uid}/time-entries` (filtered) | unit |
| `clockify_weekly_summary` | wrapper (aggregates `GET /workspaces/{ws}/user/{uid}/time-entries` by day + project) | unit |
| `clockify_whoami` | `GET /user` + `GET /workspaces/{ws}` | unit, live-read-only (TestE2EReadOnly) |

### Mutating — non-destructive (12 tools)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_add_entry` | `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_client` | `POST /workspaces/{ws}/clients` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_project` | `POST /workspaces/{ws}/projects` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_create_tag` | `POST /workspaces/{ws}/tags` | unit |
| `clockify_create_task` | `POST /workspaces/{ws}/projects/{id}/tasks` | unit |
| `clockify_find_and_update_entry` | `GET` + `PUT /workspaces/{ws}/time-entries/{id}` | unit |
| `clockify_log_time` | `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_search_tools` | local (catalog query) | unit |
| `clockify_start_timer` | `POST /workspaces/{ws}/time-entries` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_stop_timer` | `PATCH /workspaces/{ws}/user/{uid}/time-entries/{id}` | unit, sacrificial-mutating (TestE2EMutating) |
| `clockify_switch_project` | `PATCH` + `POST /workspaces/{ws}/time-entries` | unit |
| `clockify_update_entry` | `GET` + `PUT /workspaces/{ws}/time-entries/{id}` | unit |

### Destructive (1 tool)

| Tool | Endpoint | Tests |
|------|----------|-------|
| `clockify_delete_entry` | `DELETE /workspaces/{ws}/time-entries/{id}` | unit, dry-run (TestLiveDryRunDoesNotMutate), sacrificial-mutating (TestE2EMutating) |

---

## Tier 2 — Domain groups (91 tools)

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

### `scheduling` (10 tools)

Clockify endpoints: `GET/POST/PUT/DELETE /workspaces/{ws}/scheduling/*`

| Tool | Classification | Tests |
|------|---------------|-------|
| `clockify_create_assignment` | mutating | unit |
| `clockify_create_schedule` | mutating | unit |
| `clockify_delete_assignment` | destructive | unit |
| `clockify_filter_schedule_capacity` | read-only | unit |
| `clockify_get_assignment` | read-only | unit |
| `clockify_get_project_schedule_totals` | read-only | unit |
| `clockify_get_schedule` | read-only | unit |
| `clockify_list_assignments` | read-only | unit |
| `clockify_list_schedules` | read-only | unit |
| `clockify_update_assignment` | mutating | unit |

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
| `clockify_delete_custom_field` | 2 | yes | no | Described as "supports dry_run preview" |
| `clockify_delete_expense` | 2 | no | no | |
| `clockify_delete_expense_category` | 2 | no | no | |
| `clockify_delete_holiday` | 2 | yes | no | "supports dry_run preview" |
| `clockify_delete_user_group_admin` | 2 | yes | no | "supports dry_run preview" |
| `clockify_delete_invoice` | 2 | no | no | Billing |
| `clockify_delete_invoice_item` | 2 | no | no | Billing |
| `clockify_delete_shared_report` | 2 | no | no | |
| `clockify_delete_assignment` | 2 | yes | no | "supports dry_run preview" |
| `clockify_delete_time_off_request` | 2 | yes | no | "supports dry_run preview" |
| `clockify_delete_webhook` | 2 | no | no | external_side_effect |
| `clockify_remove_user_from_group` | 2 | no | no | Admin (destructive user op) |
| `clockify_deactivate_user` | 2 | no | no | Admin (classified mutating, not destructive) |

**Dry-run support: 6 of 14** (43%) destructive tools have dry_run wired.
**Dry-run live-tested: 1 of 14** (7%). The 5 Tier 2 tools with `dry_run`
in schema have never been exercised against a live Clockify workspace.

### Policy-mode live test coverage

| Mode | Unit-tested | Live-tested | Test |
|------|-------------|-------------|------|
| `read_only` | yes | no | enforcement unit tests only |
| `safe_core` | yes | no | enforcement unit tests only |
| `standard` | yes | implicitly (`TestE2EMutating` runs under standard) | enforcement + live mutating |
| `time_tracking_safe` | yes | yes | `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` |
| `full` | yes | no | enforcement unit tests only |

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
| `TestLiveTier1ReadOnly` | 13 Tier-1 read-only tools that lacked live evidence: `list_workspaces`, `list_users`, `current_user`, `list_tags`, `list_tasks`, `today_entries`, `summary_report`, `weekly_summary`, `quick_report`, `timer_status`, `detailed_report`, `resolve_debug`, `policy_info` | success path |
| `TestLiveTier2ReadOnlySweep` | 22 Tier-2 read-only and report tools across 11 groups | mixed success / pinned-error |
| `TestLiveT2BlockedGroups` | 11 tools across `shared_reports` + `scheduling` (groups blocked by upstream-host mismatch) | pinned-error (404 "No static resource") |
| `TestLiveT2ExpensesCRUD` | `create_expense_category`, `update_expense_category`, plus pinned-error contracts on `delete_expense_category` (archive-required), `create_expense` (handler content-type bug), `get_expense` (rejects bogus id) | mixed success / pinned-error |
| `TestLiveT2CustomFieldsCRUD` | `seed_project` works; `create_custom_field` and downstream tests cap-skipped at the upstream's 50-field-per-workspace limit | success on seed; cap-skipped on field tools |
| `TestLiveT2GroupsHolidaysCRUD` | `create_user_group_admin`, `update_user_group_admin`, `delete_user_group_admin` (real + dry-run); pinned-error on `get_user_group` (upstream 405) and `create_holiday` (handler date-shape bug) | mixed |
| `TestLiveT2ProjectAdminCRUD` | `seed_project`, `create_project_template`, `get_project_template`, `update_project_estimate`, `archive_projects`; pinned-error on `set_project_memberships` (handler PUT method bug) | mixed |
| `TestLivePolicyModes` | `clockify_create_client` parametrised over all 5 policy modes (`read_only`, `time_tracking_safe`, `safe_core`, `standard`, `full`) | gate behaviour pinned |
| `TestLivePaginationOnTags` | `clockify_list_tags` pagination meta envelope; `clockify_create_tag` (Tier 1) seeded × 11 | success path |

**Live-tested tools (campaign expansion):** ~25 of 33 Tier 1 tools
(76%) and ~38 of 91 Tier 2 tools (42%) are now exercised through the
MCP path against the sacrificial workspace — counting both success
paths and pinned-error contracts. Note that pinned-error coverage is
not "this tool works" coverage; it is "the protocol layer reaches
the handler, which then surfaces the upstream/handler bug we
documented" coverage. The bug inventory below records every pinned
error and the likely fix.

### Bug inventory surfaced by the campaign (May 2026)

Each item is a pinned-error assertion in one of the campaign tests.
A future fix to the relevant handler will flip the assertion and
force the annotation to be reviewed; deletion of the annotation is
the contract for landing the fix.

1. **`clockify_list_invoices`, `clockify_invoice_report`** — handler
   reads `[]map[string]any` but upstream returns `{total, invoices:
   [...]}`. Pinned by `TestLiveTier2ReadOnlySweep`. Likely fix in
   `internal/tools/tier2_invoices.go`: deserialise wrapping struct.
2. **`clockify_list_expenses`, `clockify_expense_report`** — same
   shape mismatch class; upstream wraps `{expenses: {expenses:
   [...]}}` (double-nested). Pinned in `TestLiveTier2ReadOnlySweep`
   and `TestLiveT2ExpensesCRUD`.
3. **`clockify_list_expense_categories`** — upstream returns
   `{count, categories: [...]}`; handler expects bare slice. Pinned
   in `TestLiveTier2ReadOnlySweep`.
4. **`clockify_list_webhooks`** — upstream returns
   `{workspaceWebhookCount, webhooks: [...]}`; handler expects bare
   slice. Pinned in `TestLiveTier2ReadOnlySweep`.
5. **`clockify_list_webhook_events`** — handler hits
   `/workspaces/{id}/webhooks/events` but the upstream events route
   is per-webhook (`/webhooks/{webhookId}/events`); response is 400
   "Webhook doesn't belong to Workspace". Pinned in
   `TestLiveTier2ReadOnlySweep`.
6. **All `shared_reports` tools** — handler routes via
   `api.clockify.me/.../shared-reports*` but Clockify exposes shared
   reports on `reports.api.clockify.me`. 404 across the surface.
   Pinned in `TestLiveT2BlockedGroups` (5 tools).
7. **All `scheduling` tools** — same root cause class: scheduling
   lives on a separate Clockify scheduling host. 404 across the
   surface. Pinned in `TestLiveT2BlockedGroups` (6 tools) and
   `TestLiveTier2ReadOnlySweep` (4 tools).
8. **`clockify_list_time_off_requests`** — handler GETs
   `/time-off/requests` but upstream returns 405 "Request method
   'GET' is not supported"; the endpoint requires a POST search
   body or different verb. Pinned in `TestLiveTier2ReadOnlySweep`.
9. **`clockify_create_expense`** — handler POSTs `application/json`
   but the upstream expenses endpoint expects `multipart/form-data`.
   Verified by direct curl: JSON → 415 Unsupported Media Type;
   multipart with named fields → progresses to "Amount is required".
   Pinned in `TestLiveT2ExpensesCRUD`. Likely fix: thread a
   multipart body through the client.
10. **`clockify_get_user_group`** — upstream returns 405 "Request
    method 'GET' is not supported" on the per-id route; only
    mutating verbs are accepted. Pinned in
    `TestLiveT2GroupsHolidaysCRUD`. Likely fix: scan the LIST
    response or document the upstream limitation.
11. **`clockify_create_holiday`** — handler sends `{name, date,
    recurring?}` (flat date) but upstream wants `datePeriod:
    {startDate, endDate}` plus `userIds`/`userGroupIds`/etc. Pinned
    in `TestLiveT2GroupsHolidaysCRUD`. Likely fix: rewrite handler
    body to match the upstream envelope.
12. **`clockify_set_project_memberships`** — handler PUTs to
    `/projects/{id}/memberships` but upstream returns 405; the
    Clockify v1 API exposes memberships via PATCH on the project
    resource (or via a different subroute). Pinned in
    `TestLiveT2ProjectAdminCRUD`. Likely fix: switch to PATCH.
13. **`clockify_create_custom_field`** — descriptor enum drift:
    docstring advertises "TEXT, NUMBER, DROPDOWN, CHECKBOX, LINK"
    but upstream only accepts `{TXT, NUMBER, DROPDOWN_SINGLE,
    DROPDOWN_MULTIPLE, CHECKBOX, LINK}`. Likely fix: descriptor
    docstring update or handler-side translation. Detected during
    `TestLiveT2CustomFieldsCRUD` development.

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

1. **Tier 2 live coverage (success path):** ~38 of 91 Tier 2 tools
   are now exercised through the MCP path; most of the remaining
   53 are blocked by the bug inventory above. Once those bugs are
   fixed, the campaign tests will flip from pinned-error to
   success-path automatically (the assertions are inverted by
   design).
   Read-only Tier 2 tools (39) could safely receive live-read-only tests.
2. **Schema-drift for mutating endpoints:** Only read-side schemas are
   drift-checked. Request payload schemas (tool JSON Schema descriptors)
   are not automatically compared against the live API's accepted fields.
3. **Dry-run exhaustiveness:** 6 of 14 destructive tools have `dry_run` wired;
   only 1 (`clockify_delete_entry`) is live-tested. See Dry-run/policy section
   above for the full per-tool breakdown.
4. **Policy exhaustiveness:** 2 of 5 policy modes are live-tested
   (`standard` implicitly via `TestE2EMutating`, `time_tracking_safe`
   explicitly). See Policy-mode table above for per-mode status.
5. **Rate-limit behaviour:** No automated tests exercise Clockify's rate
   limiter or the MCP server's retry/backoff behaviour under live load.
6. **Pagination consistency:** `clockify_list_entries`, `clockify_list_projects`,
   and similar paginated endpoints are not live-tested for page-boundary
   correctness.

## Recommended next tests (safe, in priority order)

1. Read-only live tests for the remaining 16 Tier 1 read-only tools
   (4 of 20 are covered by `TestE2EReadOnly`)
2. Read-only live tests for high-value Tier 2 read-only tools
   (e.g., `clockify_list_custom_fields`, `clockify_list_webhooks`,
   `clockify_list_schedules`)
3. Schema-drift test extension to mutating endpoint request schemas
4. Dry-run exhaustiveness sweep across all 14 destructive tools

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

*Tool names and classification counts verified against `docs/tool-catalog.json` on 2026-05-02. Re-run verification after `make gen-tool-catalog`.*
