# Agent cookbook

Intent-keyed recipes for agents calling this MCP server. These examples
assume a client has already initialized the server and that policy allows
the named tools. Use `clockify_policy_info` and `clockify_list_tools`
first when the visible tool list is smaller than expected.

## Log Time For Past Work

1. Confirm identity and workspace:
   `clockify_whoami {}`
2. If the project name is ambiguous, resolve it:
   `clockify_resolve_name { "entity_type": "project", "name_or_id": "Project Alpha" }`
3. Preview the entry:
   `clockify_log_time { "project": "Project Alpha", "start": "yesterday 13:00", "end": "yesterday 15:00", "description": "Code review", "dry_run": true }`
4. Ask the user to confirm the project, time range, and description.
5. Create the entry:
   `clockify_log_time { "project": "Project Alpha", "start": "yesterday 13:00", "end": "yesterday 15:00", "description": "Code review" }`
6. If the call reports an overlap, re-read the affected day and pass
   `allow_overlap:true` only after the user confirms it is intentional.
7. Re-read the affected day:
   `clockify_list_entries { "start": "yesterday", "end": "today", "page_size": 20 }`

## Catch Up An Empty Timesheet For The Week

1. Review the week:
   `clockify_timesheet_review { "week_start": "2026-05-04", "timezone": "UTC", "include_entries": true }`
2. For each gap, ask the user for the project and description if the
   review did not return enough context.
3. Preview each fill:
   `clockify_timesheet_fill_gap { "project": "Project Alpha", "start": "2026-05-04T09:00:00Z", "end": "2026-05-04T10:00:00Z", "description": "Planning", "dry_run": true }`
4. After confirmation, create each approved fill without `dry_run`.
5. Re-run the review and report remaining gaps or overlaps:
   `clockify_timesheet_review { "week_start": "2026-05-04", "timezone": "UTC" }`

## Fix A Wrong Project On Yesterday's Entries

1. List yesterday's entries:
   `clockify_list_entries { "start": "yesterday", "end": "today", "page_size": 50 }`
2. Resolve the intended project:
   `clockify_resolve_name { "entity_type": "project", "name_or_id": "Correct Project" }`
3. Select one exact entry ID; do not update multiple ambiguous entries in
   a single step.
4. Update the selected entry:
   `clockify_update_entry { "entry_id": "abc123", "project_id": "def456" }`
5. Re-read the day and summarize the changed entries:
   `clockify_list_entries { "start": "yesterday", "end": "today", "page_size": 50 }`

## Stop The Timer And Start A New One

1. Check the current timer:
   `clockify_timer_status {}`
2. Switch projects in one call:
   `clockify_switch_project { "project": "Project Beta", "description": "Implementation" }`
3. Read `data.stop_outcome`; `stopped` means an old timer was closed,
   and `no_running_timer` means only the new timer was started.
4. Confirm the active timer:
   `clockify_timer_status {}`

## Bill A Client For Last Month's Hours

1. Activate billing tools:
   `clockify_activate_group { "name": "invoices" }`
2. Summarize billable time before creating anything:
   `clockify_detailed_report { "start": "2026-04-01T00:00:00Z", "end": "2026-05-01T00:00:00Z", "project": "Client Project", "include_entries": false }`
3. Resolve the Clockify client ID:
   `clockify_resolve_name { "entity_type": "client", "name_or_id": "Acme" }`
4. Ask the user to confirm invoice number, dates, currency, amount, and
   line-item wording; `clockify_create_invoice` creates a real draft and
   does not have `dry_run`.
5. Create the confirmed invoice draft:
   `clockify_create_invoice { "client_id": "client123", "number": "INV-2026-04", "issued_date": "2026-05-01T00:00:00Z", "due_date": "2026-05-15T00:00:00Z", "currency": "USD" }`
6. Before delivery, preview the send side effect:
   `clockify_send_invoice { "invoice_id": "invoice123", "dry_run": true }`

## Subscribe To New Time Entry Events And Validate The URL

1. Activate webhook tools:
   `clockify_activate_group { "name": "webhooks" }`
2. Confirm the event token:
   `clockify_list_webhook_events {}`
3. Create the webhook with an HTTPS public URL:
   `clockify_create_webhook { "name": "New time entry sink", "url": "https://example.com/clockify/webhook", "webhook_event": "NEW_TIME_ENTRY" }`
4. Dry-run the validation delivery first:
   `clockify_test_webhook { "webhook_id": "webhook123", "dry_run": true }`
5. After confirmation, send the test delivery:
   `clockify_test_webhook { "webhook_id": "webhook123" }`
6. Keep the webhook ID for later updates or deletion.
