# AGENTS.md - go-clockify

Read this first. This is the tracked, binding agent contract for the repo.
`CLAUDE.md` is local workstation context and is git-ignored.

## Product Contract

`go-clockify` is a local one-user Clockify MCP server in Go.

- One local trusted user.
- One `CLOCKIFY_API_KEY`.
- One required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only.
- Full access from startup. The default `CLOCKIFY_TOOLSET=default` advertises
  16 everyday tools. `CLOCKIFY_TOOLSET` may also be `core`, `business`,
  `admin`, or `all`. The startup registry always loads 156 tools regardless
  of toolset; tools not advertised are still dispatch-callable by name.
- Exactly 156 tools loaded at startup; the default advertised surface is 16
  tools.
- Workflow tools first, domain tools second, raw API fallback last.
- Every write returns useful IDs.
- Recoverable failures return `ok:false`, an error code, and recovery guidance.
- Optional live evidence stays split into protocol/recovery vs happy-path.

Do not change these invariants unless the maintainer explicitly changes the
product definition.

## Start Here

1. `README.md` - setup and product overview.
2. `CONTEXT.md` - compact glossary of repo-specific domain terms.
3. `docs/agent-cookbook.md` - workflow-first agent examples.
4. `docs/tool-catalog.md` - generated runtime tool list and order.
5. `docs/goals/oneuser-tool-coverage.md` - conservative coverage ledger.
6. `docs/live-tests.md` - live-test gates and sacrificial workspace rules.
7. `docs/permissions.md` - role, plan, and feature requirements by tool family.
8. `docs/dangerous-tools.md` - destructive, billing, admin, permission-change,
   and external-side-effect tools plus dry-run coverage.
9. `docs/raw-fallback.md` - raw API path fence and raw-write environment gates.
10. `docs/error-recovery.md` - common `ok:false` codes and operator recovery.
11. `docs/protocol-notes.md` - pagination, progress, resources, and rate-control
    posture.
12. `docs/release-checklist.md` - deterministic and live release gate sequence.
13. `docs/branch-protection-required-checks.md` - required `main` CI checks.

Historical docs explain prior decisions and are preserved off-main; see
`docs/archive/README.md` for the archive branch pointer. Current work starts
from the files above plus the code. Do not route users to the archive branch
as setup instructions.

## Current State

- `main` is at or beyond `830cc12` from PR #132 (MCP guardrails and live-cleanup
  rework): the stdio surface audit, single-marshal hot path, schema bounds, the
  `warn` stdio log default, timer-tool live coverage, and the reworked
  `live-clean-prefix` sweeper.
- Branch protection requires the 16 current one-user checks in
  `docs/branch-protection-required-checks.md`, including `Module tidy drift`.
- `make perfect`, `make perfect-local`, and `make perfect-live` were green for
  the finalization stack; the latest live run is recorded in `docs/live-tests.md`
  with a commit column and prefix-object `Leftovers: 0`.
- Claude's local binary was rebuilt to `/Users/15x/.local/bin/clockify-mcp`.
- There is no required follow-up work from the finalization plan. Treat release
  tagging or new Clockify API drift as new work, not leftover finalization.

## Safety Rules

- Never print, commit, or log API keys, full workspace IDs, or tokens. Doctor
  and logs may show only a redacted/suffixed workspace-ID hint.
- Use only the configured sacrificial workspace for live tests, and do not
  mutate live Clockify unless the user asked or a test gate requires it.
- Do not weaken validation, schemas, or recovery behavior to pass tests.
- Do not remove tools to simplify the catalog.
- Do not reintroduce old activation, policy, control-plane, or multi-user
  concepts.
- Preserve user changes in a dirty tree; inspect before editing; prefer small
  focused diffs and repo-local helpers.
- If MCP tool errors ever become remotely exposed, revisit error-message
  sanitization before shipping that path.

## Common Commands

| Goal | Command |
| --- | --- |
| Full tests | `go test -count=1 ./...` |
| Perfect deterministic gate | `make perfect` |
| Perfect live gate | `make perfect-live` |
| Build Claude binary | `go build -o /Users/15x/.local/bin/clockify-mcp ./cmd/clockify-mcp` |
| Race/check gate | `make check` |
| Diff hygiene | `git diff --check` |
| Local lint | `golangci-lint run` |
| Catalog drift / regenerate | `make catalog-drift` · `make gen-tool-catalog` |
| OpenAPI drift / regenerate | `make openapi-drift` · `make gen-openapi` |
| Raw allowlist drift / regenerate | `make raw-allowlist-drift` · `make gen-raw-allowlist` |
| Self-inspection drift / sync | `make selfinspect-drift` · `make sync-selfinspect-assets` |
| Focus tools / MCP | `go test -count=1 ./internal/tools` · `./internal/mcp` |
| Live compile only | `go test -tags=livee2e -count=0 ./tests/...` |

The default command must stay free of controlplane/oidc/grpc/vault/policy/
postgres/auth dependencies; check with
`go list -deps ./cmd/clockify-mcp` (`internal/runtime/...` and Go `runtime`
hits are expected, not regressions).

Claude's global MCP config on this workstation points the `clockify` server at
`/Users/15x/.local/bin/clockify-mcp`. After any runtime change that should be
available to Claude, rebuild exactly to that path and verify the config still
references it. Do not print the env block from the Claude config; it contains
live credentials.

## Live Tests

```sh
export CLOCKIFY_API_KEY='...' CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_LIVE_PREFIX='MCP-LIVE-YYYYMMDD'
```

Extra mutation gates: `CLOCKIFY_LIVE_OPTIONAL_DOMAINS`,
`CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS`, `CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS`,
`CLOCKIFY_LIVE_WORKSPACE_CONFIRM`, `CLOCKIFY_LIVE_ADMIN_ENABLED`,
`CLOCKIFY_LIVE_BILLING_ENABLED`, `CLOCKIFY_LIVE_SETTINGS_ENABLED`.

Mark live happy-path evidence only when the tool returns `ok:true` against a
real entity; a useful recovery envelope is protocol/recovery evidence only.

## Code Map

| Need | Start Here |
| --- | --- |
| Process wiring | `cmd/clockify-mcp/main.go` |
| One-user config | `internal/config/oneuser.go` |
| MCP protocol | `internal/mcp/server.go` |
| Workflow tools | `internal/tools/oneuser_workflows.go` |
| Domain registry | `internal/tools/oneuser_domains.go` |
| Native domain logic | `internal/tools/*_view.go`, domain files in `internal/tools/*.go` |
| Resources / prompts | `internal/tools/oneuser_resources.go`, `oneuser_prompts.go` |
| Clockify client | `internal/clockify/client.go` |
| Fake server | `internal/testclockify/fake_server.go` |
| Live tests | `internal/tools/oneuser_live_test.go`, `tests/e2e_live*.go` |
| Generated catalog / ledger | `docs/tool-catalog.{md,json}`, `docs/goals/oneuser-tool-coverage.md` |

## Registry Shape

`Service.FullAccessRegistry()` composes the registry in order:
`workflowDescriptors` → `FirstSliceRegistry` → `nativeCoreDescriptors` →
`nativeHighValueDescriptors` → `nativeDomainExtras` → `timerAndReportDescriptors`
→ `rawAPIDescriptors`. The registry is fully native.

`docs/tool-catalog.{md,json}` are generated from the registry. After any
descriptor, schema, or order change, run `make gen-tool-catalog` then
`make catalog-drift`. The catalog stays at 156 tools, workflow-first, raw-last.

`docs/openapi/clockify-openapi.yaml` is generated by
`scripts/gen-clockify-openapi`. Keep that generator deterministic across macOS
Ruby and Linux CI Ruby: load repo-owned YAML as UTF-8, permit YAML aliases and
Date/Time scalar classes, and keep quarantined invalid-source reasons stable
instead of depending on Psych's exact parser wording.

## Coverage Ledger Rules

- `docs/goals/oneuser-tool-coverage.md` is the source of truth.
- Fake smoke is not live proof; live protocol/recovery and live happy-path are
  separate columns.
- Do not count bogus-ID or unavailable-feature recovery as happy path.
- Preserve recovery probes for destructive, noisy, or permission-sensitive paths.
- Update ledger validation evidence when coverage changes: a flipped cell needs
  the row and the summary count in the ledger, the `oneUserNamedLive*Evidence`
  map in `internal/tools/oneuser_quality_test.go`, and a regenerated API parity
  matrix plus synced self-inspection assets. The drift gates fail until all of
  it agrees.

## Known Clockify API Gotchas

- Time-off request listing is POST, not GET (GET returned 405 in live probes):
  `POST /workspaces/{workspaceId}/time-off/requests`.
- Invoice mark-paid may require payment creation rather than a direct status
  change; keep the ledger honest when the API rejects direct mutation.
- Holiday get/update behavior differs from list/create/delete; do not mark
  happy-path without live evidence.
- `clockify_holidays_get` / `clockify_holidays_update` resolve the holiday by
  scanning the holiday list (no get-one endpoint); `clockify_projects_memberships_list`
  reads the hydrated project record.
- No Clockify endpoint exists for `clockify_invoices_send`,
  `clockify_webhooks_test`, or `clockify_invoices_items_update`; these tools
  return a clean `unsupported` error with recovery guidance instead of calling
  upstream.
- Raw fallback is workspace fenced. Raw writes require
  `CLOCKIFY_ENABLE_RAW_WRITES=true` and default to documented Clockify routes
  only via `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=true`; raw `DELETE` preserves
  upstream response bodies.
- Clockify's invoice export endpoint only produces PDF; `clockify_invoices_export`
  advertises `format: PDF` only — use `clockify_reports_export` for CSV/XLSX.
- Tool results are capped by `CLOCKIFY_MAX_TOOL_RESULT_BYTES` (default 50000):
  oversized list results truncate with `meta.truncated`/`size_capped`, and
  CSV/PDF/XLSX/ZIP exports return `bodyEncoding:"file"` with a temp-file
  `path`.
- JSON-RPC params are decoded with `UseNumber`, so numeric tool arguments
  arrive as `json.Number`. Schema validation and numeric arg extraction must
  accept it — extract via `numberFromAny` (`intArg` / `numberArg` / `int64Arg`
  / `numberFromMap`), never a bare `.(float64)` assertion.
- Project-backed `clockify_entries_create` and `clockify_entries_timer_start`
  inherit the project's billable default when `billable` is omitted; explicit
  timer `billable` overrides report `meta.billable_source`.
- `clockify_holidays_update` is a partial update: only `holiday_id` is required,
  and unspecified fields are merged from the existing holiday record.
- `clockify_expenses_categories_update` is a partial update too — Clockify's PUT
  requires `name`, so the handler pre-fetches the category and merges the
  existing name when the caller omits it.
- `clockify_time_off_approve` / `_deny` re-hydrate their response with a
  follow-up read because the Clockify PATCH body is sparse; a failed
  re-hydration is reported in `meta` and does not fail the approve/deny.
- `clockify_time_off_requests_update` re-hydrates its result with a
  follow-up read because the Clockify PATCH body is sparse, mirroring
  `clockify_time_off_approve` / `_deny`; a failed re-hydration is reported
  in `meta` and does not fail the update.
- `clockify_scheduling_capacity` accepts an optional `user_ids`; omitting
  it returns capacity totals for every workspace user.
- Expense-category lookups (`clockify_expenses_categories_*`,
  `clockify_record_expense` category resolution) page through every
  server page of `GET /expenses/categories`; the endpoint caps a single
  response, so an un-paginated read silently drops categories.
- Recoverable `error.message` carries the cleaned upstream message — the HTTP
  method/path prefix and internal `clockify_error_code` suffix are stripped,
  while `clockify.APIError.Error()` keeps the full diagnostic for server logs.
- `clockify_invoices_import_time` / `clockify_invoices_import_expenses`:
  `time_entry_group_type` `GROUPED` also requires `time_entry_primary_group_by`
  (`USER`/`PROJECT`/`DATE`); `SINGLE_ITEM` and `DETAILED` stand alone.
- List tools with no upstream count (`projects`/`clients`/`tags`) expose
  `total_min` + `total_is_lower_bound` on a full page instead of an
  authoritative `total`; `clockify_time_off_balances` paginates with honest
  `has_more`/`dropped`.
- `clockify_audit_logs_search` defaults to `page_size:50` and sends
  `pageSize` upstream, but Clockify caps/ignores it; metadata reports
  `requested_page_size`, a lower-bound total, and the limitation note. Keep its
  max range at 31 days.
- `clockify_entries_list` and `clockify_reports_detailed` accept a same-day
  range; a bare `YYYY-MM-DD` `end` is coerced to end-of-day so `start == end`
  is a full one-day window, not a zero-width one.
- The approval-requests list filter accepts only `PENDING`, `APPROVED`, and
  `WITHDRAWN_APPROVAL`; `clockify_approvals_list` / `_get` expose exactly that
  `status` enum (`REJECTED` reverts to `UNSUBMITTED` and is not listable).
- `clockify_approvals_list`, `_get`, and the approve/reject/withdraw `dry_run`
  preview strip heavy `entries` payloads; `findApprovalRequest` scans every
  valid state, so get / resubmit-preflight / patch-dry_run resolve non-PENDING
  approvals. `clockify_invoices_payments_delete` supports a `dry_run` preview;
  `clockify_approvals_resubmit` preflights approval state for a precise
  recovery hint.
- Live Clockify currently accepts direct deletion of a freshly-created expense
  category; do not preserve older "must archive first" assumptions without a
  fresh live failure.
- Report tools may return `totals_summary`, `group_totals_summary`, or
  `weekly_totals_summary` instead of legacy top-level `totals`; tests should
  allow the current family-specific total shape.
- `clockify_status` returns the pinned workspace under `data.workspace`.
- Prefix cleanup archives clients with the existing `name` before deletion
  because Clockify validates client `PUT` bodies even for archive-only updates.
- Scheduling assignments are deleted via
  `DELETE /workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}`
  (optionally `seriesUpdateOption=THIS_ONE|THIS_AND_FOLLOWING|ALL`); the bare
  `/scheduling/assignments/{assignmentId}` route returns a 404 `No static
  resource`. `clockify_scheduling_assignments_delete` and the
  `scripts/live-clean-prefix` sweep both use the recurring route.

## Testing Discipline

- Add or update tests before changing behavior.
- Registry/schema edits: `go test -count=1 ./internal/tools` plus catalog drift.
  MCP protocol edits: `go test -count=1 ./internal/mcp`.
- For narrow docs edits, run the focused doc tests covering the touched surface.
- Before claiming completion, run fresh verification and report exactly what
  passed and what was not run.

## Git Discipline

- Do not use destructive git commands unless explicitly asked.
- Do not commit ignored local files unless the user requests it.
- The repo has a tracked `.gitignore` for common local artifacts such as the
  root `clockify-mcp` binary, coverage output, local env files, and assistant
  state. Still stage explicit paths and never `git add -A` / `git add .`:
  secrets and local state remain too risky to scoop up blindly.
- Keep commits atomic and evidence-backed.
- Keep `main` as the only maintained branch unless the maintainer explicitly
  asks for a work branch; delete/prune merged branches after landing.
- When pushing direct to `main`, watch GitHub checks and report their final
  status, including any branch-protection bypass notice.
