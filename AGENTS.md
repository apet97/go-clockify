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
  `admin`, or `all`. The startup registry always loads 156 tools for
  deterministic startup and self-inspection, but default/core/business/admin
  reject unadvertised `tools/call` names. `CLOCKIFY_TOOLSET=all` advertises
  and authorizes the full registry.
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
5. `docs/default-toolset.md` - generated 16-tool default advertised surface.
6. `docs/goals/oneuser-tool-coverage.md` - conservative coverage ledger.
7. `docs/tool-coverage-dashboard.md` - short release-readiness coverage view.
8. `docs/live-tests.md` - live-test gates and sacrificial workspace rules.
9. `docs/permissions.md` - role, plan, and feature requirements by tool family.
10. `docs/dangerous-tools.md` - destructive, billing, admin, permission-change,
   and external-side-effect tools plus dry-run coverage.
11. `docs/raw-fallback.md` - raw API path fence and raw-write environment gates.
12. `docs/error-recovery.md` - common `ok:false` codes and operator recovery.
13. `docs/protocol-notes.md` - pagination, progress, resources, and rate-control
    posture.
14. `docs/release-checklist.md` - deterministic and live release gate sequence.
15. `docs/branch-protection-required-checks.md` - required `main` CI checks.

Historical docs explain prior decisions and are preserved off-main; see
`docs/archive/README.md` for the archive branch pointer. Current work starts
from the files above plus the code. Do not route users to the archive branch
as setup instructions.

## Current State

- `main` is at or beyond the adversarial-review hardening stack after
  `54d62d0`: fail-fast registry construction, workflow business-write
  dry-runs/risk metadata, clearer error semantics docs, generated default
  toolset and coverage dashboard artifacts, and a modest `tools.Service` state
  split.
- **Audit T1.1 + T1.2 landed** (2026-05-26). `FullAccessRegistry()` and
  `nativeHighValueDescriptors()` panic-wrapping convenience constructors were
  removed; callers now use `FullAccessRegistryChecked()` /
  `nativeHighValueDescriptorsChecked()` and propagate errors. Startup registry
  bugs now surface as clean `clockify-mcp doctor` errors instead of a binary
  crash. The `openapi-drift` CI job pins Ruby via `ruby/setup-ruby@v1` +
  `.ruby-version` (3.3); previously it relied silently on the `ubuntu-latest`
  runner image's pre-installed Ruby. Full audit at
  `docs/audits/2026-05-26-claude-audit.md`.
- **Audit T2.1 landed** (2026-05-26, commit `e43f0cb`). `ResultEnvelope`
  is gone; `ToolResult` is the single success type across every handler
  (51 files renamed). Wire compatibility preserved via `,omitzero` on
  `Changed ChangeSet` (Go 1.24+ tag — narrow envelopes still emit the
  historical four-key shape). The bridge in
  `oneuser_result_helpers.go:standardizeDomainResult` distinguishes
  narrow-vs-rich results by the value-level signal `Entity != ""`
  (set by `result()`, never by `ok()`) instead of the old type switch.
  Per-handler IDs / Changed population was already done by the
  `standardizeDomainResult` lifting pass; nothing further is needed
  beyond the type unification itself. Regression tests in
  `internal/tools/result_envelope_alias_test.go`.
- **Audit T2.2 phase 1 + phase 2 landed** (2026-05-26). Phase 1
  (commit `2a4fa43`) added `auto_paginate: true` + `max_rows` (default
  5000, hard cap 50000) to the five first-slice handlers
  (`clockify_clients_list`, `_projects_list`, `_tasks_list`,
  `_tags_list`, `_entries_list`) via the generic
  `runListWithAutoPaginate[T]` helper in
  `internal/tools/helpers_pagination.go`. Phase 2 wires the remaining
  13 native list handlers — `clockify_{invoices,invoices_payments,
  projects_templates,expenses,expenses_categories,custom_fields,
  time_off_requests,time_off_policies,scheduling_assignments,approvals,
  webhooks,groups,users}_list` — via the new
  `autoPaginated(handler)` wrapper which loops at the `ToolResult`
  level (merging Data slices via reflection) so each per-handler body
  stays focused on the single-page case. Six list tools remain
  intentionally unwrapped because their upstream has no
  pagination to walk (`holidays_list`, `holidays_list_for_user_period`,
  `webhooks_events`, `projects_memberships_list`,
  `invoices_items_list`, `entity_changes_list`). See
  `docs/architecture.md §5a` for the full wired-handler list, the
  out-of-scope list, and the pattern.
- **Audit T2.3 landed** (2026-05-26, commit `4ee1d2a`).
  `docs/architecture.md` (~300 lines) documents the five tool layers,
  the `buildFullAccessRegistry()` call graph, the toolset filter, the
  `ToolResult` envelope (post-T2.1), the auto_paginate helper
  (post-T2.2 phase 1), the end-to-end "add a new tool" recipe, the
  seven drift gates, and the glossary.
- **Audit T3.1 landed** (2026-05-26).
  `paginated_ops_live_evidence_errors` in `scripts/gen-clockify-openapi`
  now runs inside `validate_document` and asserts every entry in
  `PAGINATED_LIST_OPS` (a) appears in the emitted spec and (b) carries
  `x-clockify-live-status ∈ {live-success, probe-documented}`. The
  `documented` bucket is rejected so a contributor cannot land a
  paginated annotation without a passing live probe or a recorded
  probe fixture. Failures bubble through the existing `abort` so the
  failure surfaces at `make gen-openapi` time, before drift fixtures
  regenerate.
- **Audit T3.2 phase 1 landed** (2026-05-26). `revive` is enabled in
  `.golangci.yml` with the three audit-specified rules (`exported`,
  `var-naming`, `unused-parameter`) under `enable-all-rules: false`.
  Production `unused-parameter` (5) and `var-naming` (1) violations
  are fixed. The 813 pre-existing `exported` violations across
  `internal/` are silenced via a single repo-wide exclusion pointing
  back to the audit; phase 2 is the godoc-coverage campaign that
  removes that exclusion package by package. CI's existing `lint` job
  picks the new rules up automatically because it runs
  `golangci-lint run` against the same config.
- Branch protection requires the 16 current one-user checks in
  `docs/branch-protection-required-checks.md`, including `Module tidy drift`.
- `make perfect` and `make perfect-live` were green on 2026-05-25 for the
  review-hardening stack. The latest live run is recorded in
  `docs/live-tests.md`; the optional-domain/high-risk/happy-path gates were not
  enabled for that run, and prefix cleanup reported `Leftovers: 0`.
- The locally cached Claude binary (when used) lives at
  `$HOME/.local/bin/clockify-mcp`. Each contributor may pin a different path
  in their MCP client configuration.
- The rolling nightly-live issue was still open and classified as env/config
  drift at the time of this refresh. Treat release tagging, issue closure/waiver,
  optional live campaigns, or new Clockify API drift as new work.

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
| Build binary into your MCP client path | `go build -o "$HOME/.local/bin/clockify-mcp" ./cmd/clockify-mcp` |
| Race/check gate | `make check` |
| Diff hygiene | `git diff --check` |
| Local lint | `golangci-lint run` |
| Catalog drift / regenerate | `make catalog-drift` · `make gen-tool-catalog` |
| Coverage dashboard drift / regenerate | `make coverage-dashboard-drift` · `make gen-coverage-dashboard` |
| OpenAPI drift / regenerate | `make openapi-drift` · `make gen-openapi` |
| Raw allowlist drift / regenerate | `make raw-allowlist-drift` · `make gen-raw-allowlist` |
| Self-inspection drift / sync | `make selfinspect-drift` · `make sync-selfinspect-assets` |
| Focus tools / MCP | `go test -count=1 ./internal/tools` · `./internal/mcp` |
| Live compile only | `go test -tags=livee2e -count=0 ./tests/...` |

The default command must stay free of controlplane/oidc/grpc/vault/policy/
postgres/auth dependencies; check with
`go list -deps ./cmd/clockify-mcp` (`internal/runtime/...` and Go `runtime`
hits are expected, not regressions).

If your MCP client launches `clockify-mcp` from a pinned path (a common setup
points Claude or another MCP client at `$HOME/.local/bin/clockify-mcp`), rebuild
to that path after any runtime change so the client picks the change up. Do not
print the client's env block; it contains live credentials.

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
| Service runtime state | `internal/tools/service_state.go`, `common.go` |
| Clockify client | `internal/clockify/client.go` |
| Fake server | `internal/testclockify/fake_server.go` |
| Live tests | `internal/tools/oneuser_live_test.go`, `tests/e2e_live*.go` |
| Generated catalog / ledger | `docs/tool-catalog.{md,json}`, `docs/default-toolset.{md,json}`, `docs/goals/oneuser-tool-coverage.md`, `docs/tool-coverage-dashboard.md` |

## Registry Shape

`Service.FullAccessRegistry()` composes the registry in order:
`workflowDescriptors` → `FirstSliceRegistry` → `nativeCoreDescriptors` →
`nativeHighValueDescriptors` → `nativeDomainExtras` → `timerAndReportDescriptors`
→ `rawAPIDescriptors`. The registry is fully native.

`docs/tool-catalog.{md,json}` and `docs/default-toolset.{md,json}` are
generated from the registry. After any descriptor, schema, or order change, run
`make gen-tool-catalog` then `make catalog-drift`. The full catalog stays at
156 tools, workflow-first, raw-last; the default catalog stays at 16 tools.

`docs/tool-coverage-dashboard.md` is generated from the full catalog plus
`docs/goals/oneuser-tool-coverage.md`. After catalog or ledger changes, run
`make gen-coverage-dashboard` then `make coverage-dashboard-drift`.

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
  map in `internal/tools/oneuser_quality_test.go`, a regenerated API parity
  matrix, regenerated coverage dashboard, and synced self-inspection assets.
  The drift gates fail until all of it agrees.

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
- No Clockify endpoint exists for `clockify_invoices_send_guidance`,
  `clockify_webhooks_test_guidance`, or
  `clockify_invoices_items_update_guidance`; these read-only guidance tools
  return a clean `unsupported` envelope instead of calling upstream.
- Raw fallback is workspace fenced and disabled outside `CLOCKIFY_TOOLSET=all`
  unless `CLOCKIFY_ENABLE_RAW_TOOLS=true`; raw GET additionally requires
  `CLOCKIFY_ENABLE_RAW_GET=true`. Raw writes require
  `CLOCKIFY_ENABLE_RAW_WRITES=true` and default to documented, pinned-workspace
  Clockify routes only via `CLOCKIFY_RAW_WRITE_DOCUMENTED_ONLY=true`; raw
  `DELETE` preserves upstream response bodies.
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
- Coverage ledger edits also require `make gen-coverage-dashboard` and
  `make sync-selfinspect-assets`.
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
