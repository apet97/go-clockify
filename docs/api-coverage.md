# API Coverage

This document summarizes the current one-user Clockify MCP coverage. The
authoritative tool list is generated in `docs/tool-catalog.md` and
`docs/tool-catalog.json`; the conservative test ledger is
`docs/goals/oneuser-tool-coverage.md`.

## Summary

| Class | Count | Notes |
|------|------:|-------|
| Workflow tools | 17 | Preferred agent-facing tools for common work |
| Domain tools | 137 | Direct Clockify domain operations |
| Raw fallback tools | 2 | Last-resort pinned-workspace API access |
| Total | 156 | All loaded at startup |

The runtime order is intentional: workflow tools first, domain tools second,
raw fallback tools last.

## Evidence Levels

| Evidence | Meaning |
|----------|---------|
| Unit | Normal `go test` coverage for schemas, envelopes, ordering, and helper behavior |
| Fake smoke | A fake Clockify server exercises the tool path and asserts envelope/ID/recovery shape |
| Live smoke | A sacrificial workspace exercises real Clockify API behavior |

Fake smoke is not a promise that every Clockify plan enables a feature. For
paid or permission-sensitive Clockify features, success and recoverable failure
are both acceptable when the response is structured and useful.

## Workflow Coverage

The workflow layer has the strongest coverage because it is the primary agent
surface:

- `clockify_status`
- `clockify_tools_guide`
- `clockify_create_work_package`
- `clockify_log_work`
- `clockify_start_work`
- `clockify_stop_work`
- `clockify_switch_work`
- `clockify_review_day`
- `clockify_review_week`
- `clockify_fix_entry`
- `clockify_invoice_client_work`
- `clockify_record_expense`
- `clockify_request_time_off`
- `clockify_schedule_work`
- `clockify_setup_webhook`
- `clockify_demo_seed`
- `clockify_demo_cleanup`

`internal/tools/oneuser_quality_test.go` validates workflow output schemas
against fake-server outputs. `internal/tools/oneuser_live_test.go` contains the
live workflow smoke and the opt-in paid-feature recovery smoke.

## Domain Coverage

Domain tools cover clients, projects, tasks, tags, entries, reports, invoices,
expenses, custom fields, time off, scheduling, approvals, webhooks, groups,
holidays, users, workspace settings, the audit log, and experimental entity
changes. Some domain tools are still thin
wrappers over older internal handlers; this is tracked honestly in
`docs/goals/oneuser-tool-coverage.md` and should be reduced over time without
removing tools.

## Raw Fallback

`clockify_api_get` and `clockify_api_request` are intentionally last. They
guard paths to the pinned workspace and are for gaps where no workflow or
domain tool fits.

## Local Verification

```sh
go test -count=1 ./...
git diff --check
go list ./...
make gen-tool-catalog
```

After `make gen-tool-catalog`, inspect `docs/tool-catalog.md` and
`docs/tool-catalog.json` for intentional descriptor changes.
