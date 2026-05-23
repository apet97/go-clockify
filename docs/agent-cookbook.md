# Agent cookbook

Concise workflow-first recipes for the one-user Clockify MCP. All tools are
loaded at startup. Prefer workflow tools first, domain tools for precise CRUD,
and `clockify_api_get` / `clockify_api_request` only when no workflow or domain
tool fits.

Each successful write returns `ok=true`, useful `ids`, a `changed` summary,
metadata when relevant, and `next` actions. On recoverable failure, read `error.code` and
`recovery.hint`; use `recovery.tool` when it is present.

High-risk writes use a two-step confirmation flow. Run the tool with
`dry_run:true`, inspect the preview, then retry the same arguments without
`dry_run` and with the returned `confirm_token`. If the token expires, is reused,
or the arguments change, run a fresh dry run.

Every MCP `tools/call` is schema-validated before handlers run. Missing required
arguments or unknown properties return JSON-RPC `-32602` with
`error.data.pointer`; fix the arguments instead of retrying the same payload.

## Safe Daily Time Tracking

1. Use the `safe-daily-time-tracking` prompt or start directly with
   `clockify_status {}`.
2. Keep the tool surface to `CLOCKIFY_TOOLSET=core` when the task is only daily
   tracking and review.
3. Use `clockify_create_work_package`, `clockify_log_work`,
   `clockify_start_work`, `clockify_stop_work`, `clockify_switch_work`,
   `clockify_review_day`, `clockify_review_week`, and `clockify_fix_entry`.
4. Avoid invoice, expense, admin, webhook, and raw fallback tools in this mode.

## First Call Orientation

1. Call `clockify_status {}`.
2. Confirm `ids.workspaceId`, `ids.userId`, user details, pinned workspace,
   current timer, feature status, and `recommendedFirstTools`.
3. Call `clockify_tools_guide {}` when choosing between workflow, domain, and
   raw fallback tools.
4. Next action behavior: continue with the recommended workflow tool, usually
   `clockify_create_work_package`, `clockify_log_work`, or
   `clockify_start_work`.

## Set Up Client Project Task Tag

1. Call `clockify_create_work_package` with names or known IDs:
   `{ "client": "Acme", "project": "Website", "task": "Implementation", "tag": "billable" }`.
2. Expect `ids.workspaceId`, `ids.clientId`, `ids.projectId`, and optional
   `ids.taskId` / `ids.tagId`.
3. Expect `changed.created` for new objects or `changed.reused` when names
   already exist.
4. Next action behavior: use returned IDs with `clockify_log_work` or
   `clockify_start_work`.

## Log Finished Work

1. Prefer IDs returned by setup; names are also accepted:
   `clockify_log_work { "project_id": "project123", "task_id": "task123", "start": "2026-05-14 09:00", "end": "2026-05-14 10:30", "description": "Implementation" }`.
2. Expect `ids.workspaceId`, `ids.entryId`, `ids.projectId`, and any task/tag
   IDs used.
3. Expect `changed.created` with the time entry.
4. Next action behavior: call `clockify_review_day` to verify the day, or
   `clockify_fix_entry` with the returned `entryId` if details are wrong.

## Start Stop Switch Timer

1. Start work:
   `clockify_start_work { "project_id": "project123", "description": "Focus block" }`.
2. Expect `ids.entryId` and `changed.created`.
3. Stop the running timer with `clockify_stop_work {}`; expect
   `changed.updated` and the stopped `entryId`.
4. Switch directly with
   `clockify_switch_work { "project_id": "project456", "description": "Review" }`.
5. Next action behavior: after start or switch, the returned `next` usually
   points back to `clockify_stop_work` and `clockify_switch_work`.

## Review Day Week

1. Review a day:
   `clockify_review_day { "date": "2026-05-14", "include_entries": true }`.
2. Review a week:
   `clockify_review_week { "week_start": "2026-05-11", "include_entries": true }`.
3. Expect totals, entries when requested, gaps or overlaps when detected, and
   `ids.workspaceId`.
4. Next action behavior: follow suggested `clockify_fix_entry` or
   `clockify_log_work` actions for missing or incorrect work.

## Fix Entry

1. Use an exact ID when possible:
   `clockify_fix_entry { "entry_id": "entry123", "project_id": "project456", "description": "Corrected work" }`.
2. If filtering instead of using `entry_id`, keep the filter strict enough to
   identify one entry.
3. Expect `ids.workspaceId`, `ids.entryId`, and `changed.updated`.
4. Next action behavior: re-run `clockify_review_day` or `clockify_review_week`
   for the affected range.

## Invoice Client

1. Call
   `clockify_invoice_client_work { "client_id": "client123", "number": "INV-2026-05", "issued_date": "2026-05-14", "due_date": "2026-05-28", "currency": "USD" }`.
2. Include `currency`; invoice creation rejects missing currency before the API
   call.
3. Expect `ids.workspaceId`, `ids.clientId`, and `ids.invoiceId` when invoicing
   is available.
4. Expect `changed.created` for the invoice draft; imported time entry IDs may
   appear when import arguments are supplied.
5. Next action behavior: use returned invoice IDs with `clockify_invoices_*`
   domain tools for detailed edits, send, export, or payment state.

## Record Expense

1. Call
   `clockify_record_expense { "category_id": "category123", "amount": 42.5, "date": "2026-05-14", "project_id": "project123", "notes": "Taxi" }`.
2. Ask for a receipt file only when the workspace/API returns recovery that says
   a file is required. The narrowed owner workflow supports no-file expenses
   where Clockify accepts them.
3. Expect `ids.workspaceId`, `ids.expenseId`, and any project/task/user IDs
   used.
4. Expect `changed.created`.
5. Next action behavior: use `clockify_expenses_*` domain tools for detailed
   edits, listing, or deletion.

## Time Off

1. Call
   `clockify_request_time_off { "policy_id": "policy123", "start": "2026-06-01", "end": "2026-06-03", "note": "Vacation" }`.
2. Expect `ids.workspaceId` and `ids.timeOffRequestId` when the workspace
   supports this feature.
3. Expect `changed.created`.
4. Next action behavior: use `clockify_time_off_*` domain tools to inspect
   requests, balances, and approval flow.

## Schedule Work

1. Call
   `clockify_schedule_work { "user_id": "user123", "project_id": "project123", "start": "2026-05-18", "end": "2026-05-22", "hours_per_day": 6 }`.
2. Expect `ids.workspaceId`, `ids.assignmentId`, `ids.userId`, and
   `ids.projectId`.
3. Expect `changed.created`.
4. Next action behavior: use `clockify_scheduling_*` domain tools for list,
   totals, and capacity checks.

## Webhook

1. Prefer a dry run first with the domain tool:
   `clockify_webhooks_create { "name": "New time entry sink", "url": "https://example.com/clockify/webhook", "webhook_event": "NEW_TIME_ENTRY", "dry_run": true }`.
2. Then call
   `clockify_setup_webhook { "name": "New time entry sink", "url": "https://example.com/clockify/webhook", "webhook_event": "NEW_TIME_ENTRY" }`.
3. Expect `ids.workspaceId` and `ids.webhookId`.
4. Expect `changed.created`.
5. DNS validation is on by default: HTTPS is required, embedded credentials are
   rejected, and hostnames resolving to localhost/private/reserved/link-local
   addresses fail unless explicitly allowlisted through
   `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS`.
6. Next action behavior: use `clockify_webhooks_*` domain tools to inspect
   events, logs, updates, and deletion.

## Raw API Fallback

1. Prefer typed workflow and domain tools.
2. Use `clockify_api_get` only for pinned-workspace or `/user` inspection that
   has no typed equivalent.
3. Raw non-GET calls require `CLOCKIFY_ENABLE_RAW_WRITES=true`; leave it false
   for normal daily use.
4. Raw paths must stay inside `/user`, `/workspaces/{workspaceId}`, or pinned
   workspace descendants. Absolute URLs, hosts, path traversal, backslashes, and
   encoded traversal are rejected before the API call.
5. For raw writes, call `clockify_api_request` with `dry_run:true` first and
   reuse the returned `confirm_token` only for the identical live retry.

## Demo Smoke

1. Seed deterministic objects:
   `clockify_demo_seed { "run_id": "local-smoke" }`.
2. Expect `ids.workspaceId`, `ids.clientId`, `ids.projectId`, `ids.taskId`,
   `ids.tagId`, and `ids.entryId`; expect `changed.created` or
   `changed.reused`.
3. Clean up repeatedly:
   `clockify_demo_cleanup { "run_id": "local-smoke" }`.
4. Next action behavior: read `clockify://demo/local-smoke` or re-run cleanup
   until no created demo objects remain.
