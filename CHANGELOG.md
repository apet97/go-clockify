# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Cited image-pin examples bumped to v1.2.0 + linked to SUPPORT.md.**
  Two stale operator-pointers found by re-grepping for version
  strings: `deploy/k8s/README.md` (pin example was `v0.5.0` —
  pre-v1 placeholder) and `docs/production-readiness.md` (image-
  tag column was `v1.0.0` — pre-Wave-G). Both now name `v1.2.0`
  (the current Active line per SUPPORT.md) and add an explicit
  link to SUPPORT.md so future readers find the canonical
  current line directly. Pure operator-doc fix; no behaviour
  change. Companion to a005f82 (the SUPPORT.md realignment).
- **`internal/mcp/resources.go` ADR-0009 reference renumbered.**
  The `ResourceUpdateDelta` doc-comment pointed reviewers at
  `docs/adr/013-resource-delta-sync.md` — wrong ADR number,
  AND missing the canonical 4-digit zero-pad. The actual ADR is
  `docs/adr/0009-resource-delta-sync.md`. Pure typo fix; reader
  following the link no longer hits a 404.
- **`pprof_on.go` / `pprof_off.go` drop reference to never-written
  `oom-or-goroutine-leak.md` runbook.** The original feat commit
  (da2fa8b — "pprof endpoints behind -tags=pprof") promised an
  operator runbook at `docs/runbooks/oom-or-goroutine-leak.md`
  but the file was never authored. Two source comments still
  pointed at the 404. Both rewritten to inline the security
  caveat directly: pprof_off.go points at the pprof_on.go
  doc-comment for the full security rationale; pprof_on.go now
  spells out *what* `/debug/pprof/heap` and `/debug/pprof/goroutine`
  leak (process layout, allocation patterns, handler frame
  strings) instead of deferring to a phantom runbook. Operators
  reading the source now get the why-trusted-network-only
  explanation inline.
- **Stale `w2-12-digest-pinning.md` runbook references repointed.**
  The runbook was renamed `w2-12-digest-pinning.md` →
  `image-digest-pinning.md` (already documented in CHANGELOG)
  but three call sites still referenced the old name and pointed
  operators at a 404: `deploy/Dockerfile` (the comment near the
  base-image digest pin), and two sites in
  `scripts/check-overlay-structure.sh` (the policy block-comment
  and — most impactful — the operator-facing error message that
  prints when the structural guard trips). All three repointed
  to the current filename. Pure operator-doc-pointer fix; no
  behaviour change. The check-overlay-structure.sh error message
  in particular was a real bug operators would hit.
- **`deploy/helm/README.md` image.tag default cell corrected.**
  The Highlights table claimed `image.tag` defaults to `0.7.0`,
  which was wrong on two axes: (a) `values.yaml` actually
  defaults `tag: ""` and the chart template falls back to
  `.Chart.AppVersion` when blank — there is no literal default
  tag; (b) `0.7.0` doesn't match any current release and
  predates the v1.0 wire-format guarantee. Replaced with the
  accurate `""` (falls back to `.Chart.AppVersion`) so
  operators reading the table get a true picture of the chart's
  default behaviour.
- **`docs/verify-release.md` image name + example tag corrected.**
  Three image references named `ghcr.io/apet97/clockify-mcp` —
  the registry path is `ghcr.io/apet97/go-clockify` (the same
  one referenced in `verification.md`, the deploy templates,
  the k8s manifests, and the certificate-identity-regexp on
  the very next line). Operators following the verify-release
  guide would `crane digest` / `cosign verify` against a
  non-existent image and hit a 404 immediately. All three
  occurrences renamed to `go-clockify`. Same file: the
  example `TAG=v0.7.1` (and downstream literal artifact
  names like `clockify-mcp_0.7.1_*.tar.gz`) bumped to
  `v1.2.0` so the doc-as-runbook produces a verifiable result
  end-to-end without manual substitution. The image-name fix
  is a real bug operators would hit; the tag bump is the
  doc-currency hygiene that keeps the runbook copy-paste-able.
- **`docs/runbooks/image-digest-pinning.md` examples bumped to v1.2.0.**
  Three operator-facing copy-paste examples (the
  `docker buildx imagetools inspect` digest-resolution command,
  the Argo CD `images:` override, and the Flux `newTag:`
  override) all named `v1.0.0`. Operators following the
  runbook today should pin the current Active line — bumped
  to `v1.2.0`. The historical narrative line ("the overlay was
  stuck at 0.7.0 while the base pointed at v1.0.0") is left
  as-is since that's documenting a past incident, not a
  prescription.
- **k8s base manifest + Helm chart realigned to v1.2.0.**
  `deploy/k8s/base/deployment.yaml`'s pinned image was still
  `v1.0.0` — the comment on that line says "Bump on release"
  but the bump was missed for both v1.1.0 (2026-04-22) and
  v1.2.0 (2026-04-25). Operators applying the base verbatim
  via `kubectl apply -k deploy/k8s/base` were getting a
  release without the audit-finding security wave. The Helm
  chart's `version` and `appVersion` were similarly stuck at
  `1.0.0` — `helm list` would then show that misleading
  appVersion alongside whatever image tag the operator
  actually deployed. Both bumped to `v1.2.0`. Chart-comment
  guidance corrected: only Chart.yaml needs editing on
  release because `cmd/clockify-mcp/main.go` reads its
  version via ldflags + `debug.ReadBuildInfo()`, not from a
  literal source string.
- **`SUPPORT.md` version matrix realigned to v1.2.x as Active.**
  The matrix still named `v1.1.x` (released 2026-04-22) as the
  active line, but `v1.2.0` shipped 2026-04-25 with the audit-
  finding security wave and is now where new features land. New
  row added for `v1.2.x`; the `v1.1.x` row reclassified as
  superseded. The "Backports" example also pointed at a
  hypothetical `v1.1.2` that never shipped — replaced with a
  generic "latest `v1.2.x`" pointer that ages better.
- **Hosted-profile docs reference the new
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` escape hatch.**
  `docs/operators/shared-service.md` and
  `docs/deploy/production-profile-shared-service.md` previously
  documented `CLOCKIFY_WEBHOOK_VALIDATE_DNS` without naming its
  escape hatch — operators reading those docs would learn about the
  DNS gate but not how to admit a known-trusted hostname stuck
  behind split-horizon DNS. Both pages now point at the new env
  var and link to the `webhook-dns-validation.md` §4b runbook.
  Closes the doc-sync gap left after ab010e6 lit up the env
  surface.
- **`CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` env var lights up the
  webhook DNS-allowlist escape hatch.** Operators set a
  comma-separated list of hostnames that bypass the
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS` private-IP check. Each entry
  matches either exactly (`webhook.example.com`) or as a
  leading-dot suffix that anchors a full DNS label
  (`.example.com` matches `webhook.example.com` and
  `api.eu.example.com` but NOT `attacker.example.com.evil.com`).
  Whitespace around each entry is trimmed and empty entries are
  dropped, so a leading or trailing comma is harmless. Empty
  list (default) preserves the historical reject-on-private
  behaviour exactly. Use case: split-horizon DNS where a known-
  trusted hostname legitimately resolves to a private IP only
  on the control-plane network — see
  `docs/runbooks/webhook-dns-validation.md` §4b for the operator
  runbook. Surface change: `EnvSpec` entry, Helm
  `clockify.webhookAllowedDomains` value, k8s ConfigMap
  commented placeholder, runtime wiring through
  `internal/runtime/service.go` `newService`. Validator side
  landed in e0c825d; this commit closes the env-surface slice
  of the multi-commit feature. Helm `values.yaml` /
  `deployment.yaml` / k8s `configmap.yaml` updated in the same
  commit because `make release-check`'s `config-parity` gate
  refuses any new env var that isn't reachable through the
  deploy templates.
- **Structured risk taxonomy on `ToolDescriptor`.** New `RiskClass`
  bitmask (`Read | Write | Billing | Admin | PermissionChange |
  ExternalSideEffect | Destructive`) and `AuditKeys []string` fields,
  populated by default from the existing MCP boolean hints and
  refined per tool in `internal/tools/risk_overrides.go`. Audit
  recorder consumes `AuditKeys` so events for permission/billing
  changes carry the action-defining fields (role, status, quantity,
  unit_price) — not just the *_id arguments.
- **Hosted-profile error sanitisation.** Tool-error responses on
  `shared-service` and `prod-postgres` profiles omit upstream
  Clockify response bodies. Operator override:
  `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0/1`. Server-side slog records
  still carry the full APIError for debugging.
- **Hosted-profile webhook DNS validation.** `CreateWebhook` /
  `UpdateWebhook` resolve the host and reject any reply containing
  a private, reserved, link-local, or loopback IP — closes the
  literal-IP-only gap (a hostname pointing at 169.254.169.254 would
  previously sail through). Operator override:
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS=0/1`.
- **Tier-2 activation enumerates the activated group.**
  `clockify_search_tools` responses now include an `activated_tools`
  list and a message naming every tool the activation brought online.
  Tool description spells out the contract: each Tier-2 group is the
  unit of activation.
- **Dry-run on `clockify_mark_invoice_paid` and
  `clockify_test_webhook`.** Both honour `dry_run:true`: GET the
  preview and skip the PUT/POST. Closes the inconsistency where
  `send_invoice` and `deactivate_user` already had handler-level
  dry-run.
- **Static path-safety gate.**
  `TestPathSafety_HandlersValidateIDsBeforeConcat` fails the build
  if any non-test file in `internal/tools/` concatenates a
  non-workspace ID into a URL path without calling
  `resolve.ValidateID` or a `resolve.Resolve*ID` helper.

### Security

- **Hosted profiles refuse `CLOCKIFY_INSECURE=1`.**
  `MCP_PROFILE=shared-service` and `prod-postgres` reject the
  override at startup with an actionable error. Local profiles
  preserve the existing developer behaviour.
- **`CLOCKIFY_WORKSPACE_ID` validated at startup.** Path-traversal
  shaped values (`/`, `?`, `#`, `%`, `..`, control bytes) now fail
  config load instead of silently propagating into every
  `/workspaces/{id}/...` call. `GetWorkspace` adds a belt-and-suspenders
  validate-before-concat in case `ResolveWorkspaceID` returns an
  auto-detected ID from a compromised upstream.

### Changed

- **`config_test.go` uses `maps.Copy` for fixture overlays.** Three
  `for k, v := range hostedProfileEnv { env[k] = v }` loops added
  during the audit-finding wave (Wave G + H) replaced with the
  idiomatic `maps.Copy` call. Functionally equivalent; clears the
  `mapsloop` lint hint.
- **`internal/tools/common.go` modernised.** `buildPaginationSchema`
  copies the optional `properties` overlay via `maps.Copy` instead of
  a hand-written for-range loop; `paginationFromArgs` clamps the
  `page` floor with `max()` instead of an `if`-statement. Pure
  refactor — no behaviour change. Clears the `mapsloop` and `minmax`
  hints accumulated on this file during the audit-finding wave.
- **`internal/mcp/transport_streamable_http.go` modernised.**
  `validateProtocolVersion` swaps a hand-written for-range protocol
  membership check to `slices.Contains(SupportedProtocolVersions, v)`;
  `addSessionToInitializeResult` swaps the map-overlay loop to
  `maps.Copy`. Pure refactor — no behaviour change. Clears the
  `slicescontains` and `mapsloop` hints on this file.
- **`internal/mcp/server.go` modernised.** The stdio scanner-buffer
  sizing collapses to `initial := min(64*1024, maxMsg)`, and the
  `initialize` protocol-version negotiation uses
  `slices.Contains(SupportedProtocolVersions, requested)` instead of
  a for-range scan with break. Pure refactor — no behaviour change.
  Clears the `minmax` and `slicescontains` hints on this file.
- **`internal/tools/reports.go` modernised.** Two `for k, v :=
  range src { dst[k] = v }` loops (the per-page query buffer in
  `aggregateEntriesRange` and the pagination-meta merge in
  `mergeMeta`) replaced with `maps.Copy`. The pre-allocated
  capacity hint on the per-page query map is preserved. Pure
  refactor — no behaviour change. Clears the `mapsloop` hints
  queued during ce5f12b.
- **`internal/policy/policy.go` modernised.** Three
  `for _, item := range strings.Split(os.Getenv(...), ",")`
  loops over the `CLOCKIFY_DENY_TOOLS` / `CLOCKIFY_DENY_GROUPS`
  / `CLOCKIFY_ALLOW_GROUPS` env vars rewritten as
  `for item := range strings.SplitSeq(...)` (same
  modernisation already applied to the new
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` parser in ab010e6); the
  `cloneBoolMap` helper's `for k, v := range in { out[k] = v }`
  body collapses to `maps.Copy(out, in)` while keeping the
  pre-allocated capacity hint. Pure refactor — no behaviour
  change. Clears the `stringsseq` and `mapsloop` gopls hints
  surfaced during the iter40 webhook-allowlist parser landing.
- **`internal/bootstrap/bootstrap.go` modernised.** The
  `CLOCKIFY_BOOTSTRAP_TOOLS` parser swaps
  `for _, t := range strings.Split(toolsStr, ",")` for
  `for t := range strings.SplitSeq(...)` and the `Clone`
  helper's `for k, v := range in { out[k] = v }` body
  collapses to `maps.Copy(out, in)` while keeping the
  pre-allocated capacity hint. Pure refactor — no behaviour
  change. Same lint sweep that cleared `policy.go` in
  0953132.
- **`internal/clockify/client.go` `cloneQuery` modernised.**
  The `for k, v := range in { out[k] = v }` body collapses
  to `maps.Copy(out, in)` while preserving the
  `make(map[string]string, len(in)+2)` capacity hint (the
  +2 buffer for the per-page `page` / `page-size` entries
  injected by callers like `aggregateEntriesRange`). Pure
  refactor — no behaviour change. Same lint sweep
  (`policy.go` 0953132, `bootstrap.go` 3c5592e).
- **`internal/jsonmergepatch/merge_patch.go` modernised.** Two
  hints cleared: the `applyAny` object-merge `for k, v := range
  prevObj { out[k] = v }` body collapses to
  `maps.Copy(out, prevObj)` while preserving the
  `make(map[string]any, len(prevObj)+len(patchObj))` capacity
  hint; the `hasNull` slice case `for _, inner := range val { if
  hasNull(inner) { return true } }` collapses to
  `slices.ContainsFunc(val, hasNull)`. Pure refactor — no
  behaviour change. Closes the iter43-queued lint sweep across
  the codebase.
- **`cmd/clockify-mcp/main_test.go` test fixtures modernised.**
  Two identical `for k, v := range overrides { env[k] = v }`
  helper bodies (in the strict-doctor mTLS-tenant-required and
  prod-control-plane-DSN test fixtures) collapse to
  `maps.Copy(env, overrides)`. Pure test-fixture refactor —
  no behaviour change. Same lint sweep continues into test
  files.
- **`internal/config/transport_auth_matrix_test.go` modernised.**
  Per-cell `for k, v := range tc.extra { envs[k] = v }` overlay
  collapses to `maps.Copy(envs, tc.extra)`. Pure test-fixture
  refactor — no behaviour change. Closes the iter46-queued
  test-file lint cleanup.

### Added

- **`internal/paths` package (foundation).** New `paths.Workspace(wsID, sub...)`
  helper validates the workspace ID via `resolve.ValidateID` and
  `url.PathEscape`-s every sub-segment before joining. Empty
  sub-segments and segments containing `/` are rejected at
  construction time so caller bugs surface locally rather than as a
  later 404. No callers migrated yet — this commit lands the
  foundation; future iterations swap handler-level
  `"/workspaces/"+wsID+"/..."` concats over to it.

### Changed

- **`GetWorkspace` migrated to `paths.Workspace`.** First caller of
  the new typed builder. Inline `resolve.ValidateID` + path concat
  swapped for one `paths.Workspace(wsID)` call; identical wire
  shape, identical validation semantics. The `resolve` import drops
  off this file.
- **`ListClients` + `CreateClient` migrated to `paths.Workspace`.**
  Both swap `"/workspaces/"+wsID+"/clients"` for
  `paths.Workspace(wsID, "clients")`. Identical wire shape; gains
  workspace-ID validation on every call (which `ResolveWorkspaceID`
  did not enforce on the env-supplied path).
- **`ListTags` + `CreateTag` migrated to `paths.Workspace`.** Same
  shape as `clients.go` — workspace-ID validation on every call,
  byte-identical wire output for normal Clockify IDs.
- **`ListUsers` migrated to `paths.Workspace`.** Last wsID-only
  caller before sub-segment ID territory (`projects.go`,
  `entries.go`, `tasks.go`, `tier2_*.go`). `WhoAmI` /
  `CurrentUser` hit `/user` (no workspace prefix) and stay as-is.
- **`projects.go` migrated to `paths.Workspace`.** First sub-segment
  exercise of the helper: `GetProject` now uses
  `paths.Workspace(wsID, "projects", projectID)` — the project ID
  comes from `resolve.ResolveProjectID` so it's already validated;
  the helper adds defensive percent-encoding on top.
  `ListProjects` and `CreateProject` use the simpler two-segment
  form. Identical wire shape for normal Clockify IDs.
- **`tasks.go` migrated to `paths.Workspace`.** `ListTasks` +
  `CreateTask` now use the four-segment form
  `paths.Workspace(wsID, "projects", projectID, "tasks")`. The
  project ID is resolved upstream via `resolve.ResolveProjectID`;
  the helper percent-encodes each segment. Identical wire shape for
  normal Clockify IDs.
- **`entries.go` migrated to `paths.Workspace`.** Seven concat sites
  across `GetEntry`, `AddEntry`, `UpdateEntry` (GET fetch + PUT),
  `DeleteEntry` (GET preview + DELETE), and `listEntriesWithQuery`
  (`/workspaces/<ws>/user/<uid>/time-entries`) all swap to
  `paths.Workspace(...)`. Largest single-file migration so far —
  exercises 2-, 3-, and 4-segment forms in one file.
- **`entries.go` modernised.** `ListEntries` clamps `pageSize` with
  `min()` instead of an `if`-statement; `UpdateEntry`'s outdated-URI
  loop uses `slices.Contains` instead of a hand-written found-loop.
  Pure refactor — no behaviour change. Clears the lint hints that
  surfaced during the f372814 migration.
- **`timer.go` migrated to `paths.Workspace`.** `StartTimer` POSTs
  to `paths.Workspace(wsID, "time-entries")`; `StopTimer` PATCHes
  `paths.Workspace(wsID, "user", user.ID, "time-entries")`.
  `TimerStatus` is unchanged — it routes through
  `listEntriesWithQuery`, which was migrated in f372814.
- **`reports.go` migrated to `paths.Workspace`.** Single concat in
  `aggregateEntriesRange` (used inside the pagination loop) swaps
  to `paths.Workspace(wsID, "user", user.ID, "time-entries")`. Same
  4-segment shape as the entries.go helper.
- **`workflows.go` migrated to `paths.Workspace`.** `LogTime` POSTs
  to `paths.Workspace(wsID, "time-entries")`; `FindAndUpdateEntry`
  PUTs to `paths.Workspace(wsID, "time-entries", entry.ID)`.
  `SwitchProject` is unchanged — it delegates to `StopTimer` /
  `StartTimer`, both already migrated in 3e7ae44.

### Security

- **`resources.go` migrated to `paths.Workspace` — adds first-ever
  `ValidateID` on URI-parsed IDs.** `ReadResource` parses workspace
  / user / project / entry / group IDs straight out of the
  `clockify://workspace/{id}/...` URI supplied by the MCP client.
  Pre-fix none of those IDs were validated before reaching the
  Clockify URL path. After migration, `paths.Workspace` runs
  `resolve.ValidateID` on the workspace ID and `url.PathEscape` on
  every sub-segment, so a URI containing `/`, `?`, `#`, `%`, `..`
  or a control byte is rejected at the resource layer instead of
  being silently forwarded.
- **`tier2_project_admin.go` migrated to `paths.Workspace`.** All 6
  concats (list templates, get template, create template, update
  estimate, set memberships, archive projects bulk) swap to the
  typed builder. First Tier-2 file in the migration; same pattern
  as the Tier-1 sweep — pure refactor for tools whose IDs are
  already validated upstream by `resolve.ValidateID`.
- **`tier2_shared_reports.go` migrated to `paths.Workspace`.** All
  7 concats — list, get, create, update, dry-run preview, delete,
  export — swap to the typed builder. `deleteSharedReport` builds
  `reportPath` once and reuses it for the dry-run GET preview and
  the actual DELETE.
- **`tier2_approvals.go` migrated to `paths.Workspace`.** All 8
  concats across 6 handlers (list, get, submit, approve, reject,
  withdraw). `approveTimesheet` and `rejectTimesheet` build
  `approvalPath` once at the top, used for both the dry-run GET
  preview and the PUT — same pattern as `entries.go`.
- **`tier2_custom_fields.go` migrated to `paths.Workspace`.** All 8
  concats: list/get/create/update; `DeleteCustomField` builds
  `fieldPath` once for dry-run GET + DELETE; `SetCustomFieldValue`
  picks the projects-vs-time-entries branch via the helper while
  keeping the conditional shape.
- **`tier2_groups_holidays.go` migrated to `paths.Workspace`.** All
  8 concats — 5× user-groups (list/get/create/update/delete) +
  3× holidays (list/create/delete). Standard `paths.Workspace`
  swaps; helper validates the workspace ID and percent-encodes
  every sub-segment.
- **`tier2_user_admin.go` migrated to `paths.Workspace`.** All 8
  concats across the admin surface (list/create/update/delete
  user-groups, add/remove user from group, update user role,
  deactivate user). The `update_user_role` and `deactivate_user`
  PUT paths target `/workspaces/<ws>/users/<uid>{,/roles}`, which
  carries `RiskAdmin | RiskPermissionChange` per the descriptor
  taxonomy — defence-in-depth percent-encoding is doubly welcome
  here.
- **`tier2_webhooks.go` migrated to `paths.Workspace`.** All 9
  concats — list/get/create/update/delete + ListWebhookEvents
  (special `/webhooks/events` literal sub-path) + TestWebhook
  (which has separate dry-run preview path and `/test` POST path).
  `DeleteWebhook` builds `webhookPath` once for the dry-run GET +
  DELETE pair. Webhooks carry `RiskExternalSideEffect`; the helper
  re-validates the workspace ID even though the literal-IP webhook
  URL check still runs in the body.
- **`tier2_scheduling.go` migrated to `paths.Workspace`.** All 10
  concats across 8 handlers (assignments CRUD, schedules CRUD,
  project-totals report, capacity filter). Cleanest pattern in the
  Tier-2 cluster — every concat was already in the form
  `path := "..."`, so the swap is a literal RHS replacement.
- **`tier2_expenses.go` migrated to `paths.Workspace`.** All 11
  concats: 5 expense handlers (list/get/create/update/delete) +
  4 category handlers (list/create/update/delete) + 1 report.
  `deleteExpense` builds `expensePath` once for dry-run preview +
  DELETE; `deleteExpenseCategory` uses minimal-fallback short-circuit
  before the path is needed.
- **`tier2_time_off.go` migrated to `paths.Workspace`.** All 12
  concats across 10 handlers — request CRUD nested under policy ID
  (5-segment), approve/deny PUTs (6-segment), policy CRUD
  (3-4 segment), balance lookup (6-segment).
  `deleteTimeOffRequest` and `updateTimeOffRequest` /
  `updateTimeOffPolicy` build path once for the GET-then-mutate
  pair. First Tier-2 file with consistent 6-segment paths.
- **`tier2_invoices.go` migrated to `paths.Workspace`.** All 15
  concats across 12 handlers — list/get/create/update/delete
  invoices, send (3-segment `/send`), mark-paid, item CRUD
  (4/5-segment `/items[/id]`), and report. `deleteInvoice` and
  `markInvoicePaid` build path once for the dry-run GET preview +
  the real PUT/DELETE; `sendInvoice` uses two paths (the bare
  invoice for the preview, plus the `/send` sub-path for the POST).
  Closes the Tier-2 caller-migration sweep (12/12 files).
- **`docs/tool-catalog.json` exposes `risk_class` + `audit_keys`.**
  The catalog generator now decomposes every tool's `mcp.RiskClass`
  bitmask into stable lowercase taxonomy names (`read`, `write`,
  `billing`, `admin`, `permission_change`, `external_side_effect`,
  `destructive`) and surfaces the `AuditKeys` slice. Consumers
  (policy agents, ops dashboards, audit harnesses) can now filter
  on the structured taxonomy without grep-ing source.
- **`docs/tool-catalog.md` gains a `Risk` column.** Both the Tier-1
  table and every Tier-2 group sub-table render the same risk
  taxonomy as inline-coded names joined with `, `. Empty risk (the
  zero-value bitmask, which never occurs today) renders as an em
  dash so the column never collapses to a blank cell. Closes the
  human-browsing gap left when 97c20da landed the JSON surface
  without touching the markdown rendering.

### Fixed

- **`internal/paths` package doc no longer leaks an absolute path.**
  The package comment previously pointed reviewers at
  `/Users/15x/.claude/plans/...` for the audit-finding context — a
  personal local path with no meaning on any other machine. Now
  references the in-repo CHANGELOG entries (0de5458, 1919006) and
  describes the migration sweep in past tense since it completed.
- **`/mcp/events` legacy-alias comment matches reality.** The
  comment in `streamableHTTPMux` previously promised the alias
  would be removed in v0.7 — the project shipped v1.0 in 2026-04-12
  and is now at v1.2.0 with the alias still mounted, so the comment
  was stale by ~9 months. Replaced with an ADR-0012 reference noting
  the route stays indefinitely (operator-facing route removal would
  need a v2.0 bump). `TestStreamableEventsBackCompatAlias` docstring
  updated in the same commit so test + production prose agree.
- **ADR-0002 file references unstuck from pre-C2.2 line numbers.**
  Two sites pointed at `internal/config/config.go:107-116` for the
  `MCP_TRANSPORT` validation switch (now at lines 239–247 after the
  auth surface grew) and one at `cmd/clockify-mcp/main.go:161-260`
  for dispatch wiring (which moved to `internal/runtime/runtime.go`
  in the dea1cc3 C2.2 refactor — main.go is now a 236-line shim).
  Replaced with function-name search anchors (`Load()`,
  `Runtime.Run`) so future reorgs do not invalidate the ADR again.
  Landed in two passes (References section in 6a3c25b, body
  paragraph in this commit) to stay within the per-commit file
  budget.
- **ADR-0003 file references unstuck from pre-1.x line numbers.**
  Four anchors invalidated by post-write growth of the auth surface
  in `internal/config/config.go` (the auth-mode switch + token-length
  checks moved ~130 lines down) and the gRPC transport file (the
  Authenticator-wiring block shifted ~10 lines after the TLS option
  block grew). Replaced with the same function-name + grep-string
  search anchors used in the ADR-0002 sweep so the same reorg-drift
  cannot recur.
- **ADR-0005 file references unstuck from pre-C2.2 line numbers.**
  Three stale anchors fixed: `internal/tools/context.go:75-90`
  (the activation handler grew past line 90 when the activated_tools
  enumeration landed); `bootstrap.go:55-68` (mis-described the
  `AlwaysVisible` location — that map starts at line 47, not 55);
  and `cmd/clockify-mcp/runtime.go:113-150` (file was removed in
  the dea1cc3 C2.2 refactor — wiring now lives in
  `internal/runtime/runtime.go` `New()`). Anchors at `bootstrap.go:71`
  and `:56` were verified still accurate and left as-is — this
  iteration only touched the genuinely stale ones.
- **ADR-0006 historical-rename anchors realigned.** The
  References section lists the source comments that say
  "See ADR 009" (the historical name for ADR-0006). Two of six
  line refs had drifted: `cmd/clockify-mcp/main.go:145` (now :147)
  and `scripts/check-build-tags.sh:64` (now :68). Line refs are
  intentional here — the comment text is the same in every file,
  so a grep would not disambiguate. The other four anchors
  (otel_on.go:15, otel_off.go:10, otel.go:5, span_emit_test.go:39)
  were verified still accurate.
- **`normalizeEndpoint` comment matches behaviour.** Doc now
  precisely describes the 24/32/36-char ID-shape match instead of
  overstating "any other non-letter leading segment". Companion
  test (`TestNormalizeEndpoint_NonIDShapesPreserved`) locks in
  both the collapse and the preserve paths so comment + code can
  no longer drift apart silently.
- **Stdio honours `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1`.** The
  flag is now assigned in `buildServer()`, so every transport
  (stdio, legacy_http, streamable_http session, grpc) picks it up
  uniformly. Pre-fix the assignment lived in `runLegacyHTTP()` and
  the streamable-session overlay only, leaving stdio operators
  with no way to opt in.
- **Name resolution accepts legitimate Clockify names with
  punctuation.** New `resolve.ValidateNameRef` is a permissive
  sibling of `ValidateID`: empty / oversized / control-byte input
  still fails, but `/`, `?`, `#`, `%`, `&`, `..`, and Unicode pass
  through. `resolveByNameOrID` and `ResolveUserID` now dispatch
  on shape — strict `ValidateID` only when the input is being
  returned verbatim as a path-segment ID, permissive `ValidateNameRef`
  when it goes to a `name=` query parameter (which `url.Values`
  safely encodes). Pre-fix, project / client / tag / task names
  like "ACME / Support" or "R&D 50%" failed validation before the
  safe lookup could run.

## [1.2.0] - 2026-04-25

> **Scope note.** Security-hardening wave following the
> 2026-04-25 audit, plus a follow-on wave that lands first-class
> gRPC release artifacts, MCP-path live safety tests, and a few
> supply-chain repairs that surfaced during release verification.
> Self-hosted single-tenant behaviour is preserved by default —
> every new restriction is opt-in via a flag (`MCP_OIDC_STRICT`,
> `MCP_REQUIRE_TENANT_CLAIM`, `MCP_DISABLE_INLINE_SECRETS`) so
> existing deployments continue to work unchanged.

### Added (post-Wave-G additions)

- **First-class gRPC release artifacts.** GoReleaser now publishes
  four new linux-only binaries:
  `clockify-mcp-grpc-{linux-x64,linux-arm64}` (private-network
  gRPC, no postgres) and
  `clockify-mcp-grpc-postgres-{linux-x64,linux-arm64}` (HA
  private-network gRPC + pgx control plane). Each ships through
  the same SBOM (syft) + cosign sigstore + SLSA build-provenance
  chain as the default and Postgres binaries.
  `scripts/check-release-assets.sh` raised `EXPECTED_COUNT` from
  34 → 46 and gained `GRPC_PLATFORMS` / `GRPC_POSTGRES_PLATFORMS`
  arrays; the regex was reordered so `-grpc-postgres` matches
  before `-grpc`. The hosted launch checklist references the new
  artifact names.
- **`scripts/check-grpc-release-parity.sh`** — release-blocking
  drift gate: the private-network-grpc profile doc must not claim
  tenant defaults to `X-Tenant-ID`, must not claim Docker images
  include gRPC unless either `.goreleaser.yaml` ships a `-grpc`
  artifact or the Dockerfile / docker-image workflow exposes a
  `GO_TAGS` build arg, and any doc reference to a `-grpc`
  artifact must be backed by a matching GoReleaser build id +
  asset-count enumeration. Wired into `verify-core` and
  `release-check`.
- **`make build-grpc` / `make build-grpc-postgres`** — local
  build targets that exercise the gRPC and gRPC+postgres tag
  matrices so `make verify` is honest about the private-network
  gRPC profile compiling against the working tree.
- **MCP-path live safety contracts** — three new tests that
  exercise the production enforcement / audit pipeline against a
  real Clockify backend instead of the bare tool handlers:
  - `TestLiveDryRunDoesNotMutate` (`tests/e2e_live_mcp_test.go`,
    build tag `livee2e`) — confirms `clockify_delete_entry` with
    `dry_run:true` previews via the GET counterpart and never
    deletes the entry upstream.
  - `TestLivePolicyTimeTrackingSafeBlocksProjectCreate`
    (`tests/e2e_live_mcp_test.go`) — confirms
    `CLOCKIFY_POLICY=time_tracking_safe` rejects
    `clockify_create_project` at the policy gate before the
    handler runs.
  - `TestLiveCreateUpdateDeleteEntryAuditPhases`
    (`internal/controlplane/postgres/live_audit_phases_test.go`,
    build tags `postgres,livee2e`) — confirms a real
    create→update→delete entry cycle persists six audit rows
    (3 intent + 3 outcome) in a Postgres-backed control plane,
    distinguished only by phase + outcome segments embedded in
    the synthesised `external_id`.
  All three are wired into `.github/workflows/live-contract.yml`
  under the existing `CLOCKIFY_LIVE_WRITE_ENABLED=true` gate.
  The audit-phase test additionally requires
  `MCP_LIVE_CONTROL_PLANE_DSN`; missing on a fork is a soft skip,
  missing on the main repo is a hard fail when the new repo
  variable `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` is set.
- **Docker `GO_TAGS` build-arg path.** `deploy/Dockerfile` now
  accepts `--build-arg GO_TAGS=grpc[,postgres]` so operators can
  build a gRPC-capable image directly from the published
  Dockerfile:
  `docker build --build-arg GO_TAGS=grpc,postgres -f deploy/Dockerfile -t clockify-mcp:grpc-postgres .`
  Default image is byte-equivalent (empty `-tags=""` is a Go
  toolchain no-op). The Dockerfile also copies `go.work`,
  `go.work.sum`, and the per-sub-module `go.mod` / `go.sum` pairs
  so tagged builds resolve workspace modules correctly without
  silently falling back to a stale remote sub-module version.
- **Docker PR-only gRPC smoke test** in
  `.github/workflows/docker-image.yml` builds a side image with
  `GO_TAGS=grpc,postgres` and verifies the runtime no longer hits
  the `!grpc` stub error, so the documented self-build path can't
  rot.
- **`internal/auditbridge/`** — shared `ToControlPlaneEvent(event,
  now)` helper used by both the runtime auditor and the live
  audit-phase contract test. Centralises the
  `mcp.AuditEvent → controlplane.AuditEvent` conversion plus the
  external_id synthesis that keeps PhaseIntent + PhaseOutcome rows
  distinct under the Postgres unique constraint. Three direct
  unit tests pin the field mapping, the IDs-differ contract, and
  nil-metadata defensive behaviour.
- **`internal/authn/category_test.go`** — pins the
  `FailureCategory` substring → bucket contract that every
  transport's auth-failure log/metric label depends on. Closes
  the pre-existing `internal/authn` coverage shortfall (86.0% →
  89.8%).

### Fixed (post-Wave-G)

- **FIPS binaries now get SLSA build provenance.** Every release
  back to v1.0.x shipped FIPS binaries cosign-signed but with no
  attest-build-provenance subject — `gh attestation verify
  clockify-mcp-fips-*` returned HTTP 404. The Wave-G FIPS row was
  added to `.goreleaser.yaml` without extending `release.yml`'s
  staging step or the `attest-build-provenance` subject-path.
  Operators running the FIPS binary previously could not satisfy
  the launch checklist's "SLSA build provenance attested" gate.
  Closes the gap manually surfaced during v1.1.0 verification.
- **Dockerfile previously failed on `GO_TAGS=grpc[,postgres]`.**
  The build context only included `go.mod`; tagged builds need
  `go.work` + the workspace sub-module manifests
  (`internal/transport/grpc`, `internal/controlplane/postgres`,
  `internal/tracing/otel`). Without them the Go toolchain either
  failed offline or silently downloaded a stale remote version of
  the sub-module — shipping an image whose gRPC code did not match
  its source tree. The published `docker build --build-arg
  GO_TAGS=grpc,postgres ...` recipe now actually works.

### Changed (post-Wave-G)

- **`docs/deploy/profile-private-network-grpc.md`** corrected:
  tenant extraction defaults to `MCP_MTLS_TENANT_SOURCE=cert`
  (not `X-Tenant-ID`); the Docker default image does NOT include
  gRPC; auth modes other than mtls are supported but not the
  recommended posture.
- **`docs/support-matrix.md`** gRPC row spells out both supported
  build paths (published artifact vs. self-build).
- **`docs/release/public-hosted-launch-checklist.md`** Storage row
  references `clockify-mcp-grpc-postgres-*` for HA gRPC; live
  coverage gate is now executable rather than "tracked or
  closed" prose.
- **`internal/runtime/service.go`** controlPlaneAuditor delegates
  to `internal/auditbridge.ToControlPlaneEvent` instead of
  inlining the conversion + ID synthesis. Same contract, one
  source of truth.

### Wave G — Security-hardening wave (the original 2026-04-25 audit)

> **Scope note.** Security-hardening wave following the
> 2026-04-25 audit. Six atomic commits closing the seven blockers
> the audit flagged for paid/public hosted-service deployment, plus
> the M-tier docs/defaults drift. Self-hosted single-tenant
> behaviour is preserved by default — every new restriction is
> opt-in via a flag (`MCP_OIDC_STRICT`, `MCP_REQUIRE_TENANT_CLAIM`,
> `MCP_DISABLE_INLINE_SECRETS`) so existing deployments continue to
> work unchanged.

### Added

- **Native TLS / mTLS on the streamable HTTP transport** via two
  new env vars: `MCP_HTTP_TLS_CERT` and `MCP_HTTP_TLS_KEY`.
  `MCP_TRANSPORT=streamable_http` with non-empty cert/key paths
  wraps the listener with `tls.NewListener`; combined with
  `MCP_AUTH_MODE=mtls` and `MCP_MTLS_CA_CERT_PATH`, it enables
  end-to-end mutually-authenticated TLS without a fronting proxy.
  The shared TLS helpers live in `internal/runtime/tlsutil.go`
  and are reused by the gRPC transport. Closes audit finding H3.
- **`MCP_OIDC_STRICT=1`** — fails `config.Load` when oidc is
  selected without `MCP_OIDC_AUDIENCE` or `MCP_RESOURCE_URI`,
  and rejects tokens missing an `exp` claim at the per-token
  level. Default unchanged (back-compat). Closes finding C1.
- **`MCP_REQUIRE_TENANT_CLAIM=1`** — rejects oidc tokens whose
  tenant claim is empty instead of falling back to
  `MCP_DEFAULT_TENANT_ID`. Required for any multi-tenant hosted
  deployment. Closes finding H6.
- **`MCP_DISABLE_INLINE_SECRETS=1`** — rejects credential refs
  with `backend=inline` so secrets are forced through env / file /
  external vault backends. Closes finding L3.
- **`time_tracking_safe` policy mode** — new `CLOCKIFY_POLICY`
  tier strictly between `read_only` and `safe_core`. Allows
  reads + own-time-entry mutations + timer control; blocks
  workspace-wide `create_*` tools (project / client / tag /
  task). Recommended default for untrusted AI agents. Closes
  finding M4.
- **Audit phase concept** — `AuditEvent` gains a `Phase` field
  (`PhaseIntent` / `PhaseOutcome`). Non-read-only tool calls
  now write a pre-handler intent record AND a post-handler
  outcome record. Empty `Phase` ("") preserved for backward
  compatibility with audit consumers that pre-date this change.
- **Hosted-service hardening guards** — production profile docs
  set `MCP_OIDC_STRICT=1` + `MCP_REQUIRE_TENANT_CLAIM=1` +
  `MCP_DISABLE_INLINE_SECRETS=1`. Branch-protection target
  state for paid launch documented in
  `docs/branch-protection.md`. Live contract tests now fail
  the workflow on the main repo when secrets are absent (forks
  keep the warning-and-skip behaviour). Closes findings M7, L2.
- **`deploy/k8s/overlays/legacy-http/`** — explicit opt-in
  overlay for operators still on pre-v1.1.0 clients that have
  not migrated to streamable HTTP.
- **`tests/deploy_defaults_test.go`** — guards the Dockerfile,
  Helm values, and Kustomize base against drifting back to
  `MCP_TRANSPORT=http` or `CLOCKIFY_POLICY=standard`.
- **`tests/doc_parity_test.go`** — asserts the README MCP
  protocol badge AND support matrix row both equal
  `mcp.SupportedProtocolVersions[0]`.

### Changed

- **fail_closed audit now actually blocks mutation.** Previously
  `MCP_AUDIT_DURABILITY=fail_closed` returned an error to the
  client AFTER the mutation had already committed upstream. The
  two-phase intent/outcome model writes the intent record
  pre-handler — when intent persistence fails in fail_closed
  mode, the handler is skipped entirely and the mutation never
  happens. The new behaviour is documented in
  `docs/runbooks/audit-durability.md`. Closes finding H5.
- **Default deployment transport flipped from `http` to
  `streamable_http`** in `deploy/Dockerfile`, the Helm
  `values.yaml`, and the Kustomize base. Closes finding H1.
- **Default deployment policy flipped from `standard` to
  `safe_core`** in the Helm `values.yaml` and Kustomize base
  (Dockerfile inherits via env). Closes finding M3.
- **`MCP_STRICT_HOST_CHECK=1`** by default in Dockerfile, Helm
  values, and Kustomize base — DNS-rebinding mitigated by
  default for any deployment exposed beyond loopback.
- **Stdio panic recovery returns a generic message** instead
  of the raw panic value. Full panic and stack remain in
  `slog.Error("panic_recovered", ...)`. Closes finding H7.
- **Config validation requires TLS cert material when
  `MCP_AUTH_MODE=mtls`** on either the streamable HTTP or
  gRPC transport. Previously these combinations would start
  successfully and fail every request at runtime; now they
  fail at startup with a message naming the missing variable.
  Closes finding H4.
- **README MCP protocol version** and back-compat list aligned
  with `SupportedProtocolVersions` (newest: `2025-11-25`).
  Closes finding H2.
- **Production-profile docs** corrected to reference `/health`
  and `/ready` (the actual registered routes; previously
  `/healthz` and `/readyz`). Closes finding M2.
- **`SECURITY.md` and `SUPPORT.md` version matrix** brought in
  line with v1.1.0's release date (2026-04-22). Closes
  finding M1.
- **`docs/branch-protection.md`** gains a "Target state for
  paid / public hosted launch" section documenting the
  governance tightening blocked on adding a second maintainer.
  Closes finding L2.
- **`scripts/check-doc-parity.sh`** excludes
  `docs/superpowers/` from the operator-facing parity scan
  (design specs by definition describe future state).

### Security

- The audit refactor closes the gap where `fail_closed`
  delivered post-hoc evidence rather than acting as a
  preventive control. Operators in fail_closed mode now have
  a real durability guarantee: a broken audit pipeline blocks
  mutations rather than committing them and complaining.
- Stdio panic recovery no longer leaks panic values
  (potentially containing request data, internal state, or
  upstream credential fragments) to MCP clients.
- OIDC accepts issuer-only tokens by default for back-compat;
  hosted deployments must opt into the strict mode flags.

### Fixed

> **Follow-up audit pass (2026-04-25).** Five additional atomic
> commits caught by a deeper read-through of the post-wave repo
> state. Each closes a defense-in-depth or wiring gap that the
> primary security wave did not cover.

- **Metrics listener no longer silently un-authenticates.**
  `mcp.ServeMetrics` and `metricsMux` now refuse
  `AuthMode=static_bearer` with an empty `BearerToken`. The
  production startup path through `cmd/clockify-mcp/main.go` was
  already protected by config-load validation
  (`internal/config/config.go:280-285`); this commit closes the same
  gap at the library API surface so a programmatic embedder building
  `MetricsServerOptions` directly cannot regress the property. Without
  the guard, `subtle.ConstantTimeCompare("","")==1` would treat any
  client (including a bare `Authorization: Bearer ` header) as
  authenticated despite a startup log claiming bearer mode.
- **OIDC strict mode rejects `authn.Config` without audience or
  resource URI.** `authn.New` now refuses to construct the OIDC
  authenticator when `OIDCStrict=true` with both `OIDCAudience` and
  `OIDCResourceURI` empty. Mirrors the pre-existing config-load
  check at `internal/config/config.go:360-361` (which only fires
  on the env-var path) and matches the documented `MCP_OIDC_STRICT`
  contract that strict mode binds tokens to this server.
- **Schema/handler date-time drift on `clockify_add_entry`,
  `clockify_list_entries`, `clockify_weekly_summary`, and several
  Tier 2 timesheet/approval/expenses tools.** The schema tightener
  added `format: "date-time"` to any string property mentioning
  "RFC3339" in its description, even when that description also
  documented a flexible parser (`natural language`, `YYYY-MM-DD`).
  The jsonschema validator's strict `time.Parse(time.RFC3339, ...)`
  then rejected valid input like `start="now"` before the handler's
  lenient `timeparse.ParseDatetime` ever saw it. The tightener now
  skips the `format` constraint on flexible-time fields;
  `docs/tool-catalog.json` regenerated to match.
- **Helm ServiceMonitor wires the dedicated metrics listener
  correctly.** Setting `metricsEndpoint.bind` non-empty now renders a
  `metrics` Service port + matching containerPort + ServiceMonitor
  `port: metrics` in one toggle. A new
  `metrics.serviceMonitor.bearerTokenSecret` block attaches the
  Authorization header that `static_bearer` auth requires. The
  previous chart left the dedicated listener unreachable from
  Prometheus and never carried an auth header. Kustomize base
  ServiceMonitor gains an inline comment clarifying that the
  `port: http` default works only with inline metrics
  (`MCP_HTTP_INLINE_METRICS_ENABLED=1`); use the Helm toggle or
  layer an overlay for the dedicated-listener pattern.
- **Postgres integration test gate fails loud when Testcontainers
  unavailable.** `internal/controlplane/postgres` now honours an
  `INTEGRATION_REQUIRED` env var that turns Testcontainers failure
  into `t.Fatal` instead of `t.Skip`. The Makefile target
  `test-postgres` sets the var so a Docker-less CI run can no longer
  report green vacuously. Developer laptops without Docker keep the
  historic skip behaviour by running `go test
  -tags=postgres,integration` directly.

## [1.1.0] - 2026-04-22

> **Scope note.** First minor release after v1.0.0 (2026-04-12).
> Lands the profile-centric configuration model, the `doctor`
> subcommand, governance alignment, the toolchain + build-tag
> matrix tightening, and the public-repo flip that makes SLSA
> attestation a mandatory gate.

### Added

- **Canonical deployment profiles (`--profile=<name>`).** Five
  code-enforced profiles — `local-stdio`,
  `single-tenant-http`, `shared-service`,
  `private-network-grpc`, `prod-postgres` — bundle the pinned
  defaults for each supported deployment shape. Apply via
  `clockify-mcp --profile=<name>` or `MCP_PROFILE=<name>`.
  Explicit env overrides always win; the Wave H fail-closed
  guards run unchanged. Each profile has a matching
  `deploy/examples/env.<name>.example` file and a doc in
  `docs/deploy/profile-<name>.md`.
- **`clockify-mcp doctor` subcommand.** Audits the effective
  configuration, attributing every spec'd env var as
  `explicit` / `profile` / `default` / `empty` via a
  text/tabwriter report. Exit code 0 on clean Load(), 2 on a
  Load() error. Takes the same `--profile=<name>` flag as the
  server.
- **`SUPPORT.md`** at repo root covers where to ask questions,
  response expectations (best effort, no SLA), v1.x wire-format
  stability guarantee, and the version support matrix.
- **Build-tag matrix workflow.** New
  `.github/workflows/build-matrix.yml` runs compile-only checks
  on six tag combinations (`grpc`, `fips`, `otel`, `pprof`,
  `grpc,otel`, `fips,grpc`) on every push, PR, and weekly cron.
- **Four new ADRs.** ADR-0013 (private-repo SLSA posture;
  superseded at release time), ADR-0014 (prod fail-closed
  defaults), ADR-0015 (profile-centric configuration model),
  ADR-0016 (single-maintainer governance reality).

### Changed

- **SLSA attestation is now mandatory on release.** The repo
  flipped to public on 2026-04-22, which unblocked the GitHub
  attestation service for this account tier.
  `actions/attest-build-provenance` in `release.yml` and
  `docker-image.yml` is no longer `continue-on-error: true`; the
  `gh attestation verify` step in `release-smoke.yml` no longer
  treats HTTP 404 as a skip. A missing or invalid attestation
  will now fail the release or the smoke. ADR-0013 is marked
  superseded; the workaround it documented is no longer live.
- **Production fail-closed defaults (`ENVIRONMENT=prod`).**
  With `ENVIRONMENT=prod` unset values of
  `MCP_HTTP_LEGACY_POLICY` resolve to `deny` (was `warn`) and
  `MCP_AUDIT_DURABILITY` resolves to `fail_closed` (was
  `best_effort`). Explicit operator values always win. ADR-0014
  captures the rationale. Load() also fails closed at the
  streamable_http + dev-DSN boundary without the explicit
  `MCP_ALLOW_DEV_BACKEND=1` acknowledgement.
- **Governance documentation aligned on single-maintainer
  reality.** `GOVERNANCE.md`, `.github/CODEOWNERS`, the PR
  template, and the new `SUPPORT.md` now tell one consistent
  story. Drops the `@backup-maintainer` placeholder that never
  resolved to a real handle. ADR-0016 codifies the decision.
- **Release smoke strips the `v` prefix before ghcr lookup.**
  `docker-image.yml` publishes semver tags without the leading
  `v` (via the metadata-action `{{version}}` pattern), so
  `release-smoke.yml` now normalises the tag before calling
  `cosign triangulate`. Closed the last layer tracked under
  issue #7.
- **Release smoke authenticates to ghcr.io.** Added a
  `cosign login ghcr.io` step so the container manifest lookup
  works after the visibility switch paths were proven.
- **Release smoke cosign version aligned with release signer.**
  Bumped cosign-installer to match `release.yml`'s v2.4.3 so
  the verifier can read the `--new-bundle-format` bundles the
  signer writes.
- **govulncheck pinned to a commit SHA** instead of `@master`,
  for supply-chain reproducibility. Tracked to revisit once a
  tagged release supports go1.25.

### Fixed

- **Issue #7 closed.** The four-layer smoke failure (SLSA 404
  on private repos, cosign format skew, ghcr auth, tag prefix)
  was resolved across PRs #16, #18, #19, #20.

### Security

- Streamable HTTP fail-closed guard at config load time
  prevents multi-process deployments from silently running
  against an in-memory control plane. The existing runtime
  guard remains as defence-in-depth.

## [1.0.3] - 2026-04-20

### Fixed

- **Release workflow continues past SLSA attestation failure.**
  `actions/attest-build-provenance` hard-fails on user-owned
  private repositories (GitHub feature gate). `release.yml`'s
  attestation step is now `continue-on-error: true`, same treatment
  as `docker-image.yml`. Attestations activate automatically if
  the repo moves to an org or goes public.

### Context

v1.0.1 and v1.0.2 both published 28 signed+SBOM'd assets to the
GitHub Release but never reached npm or completed SLSA attestation
because a downstream step in the release workflow killed the
pipeline under `set -e`. v1.0.3 is the first release to complete
the full pipeline — GitHub Release, cosign signatures, SBOMs, npm
publish, and reproducibility trigger. Code-wise v1.0.3 and v1.0.2
are identical except for this CHANGELOG entry and the workflow
fix.

## [1.0.2] - 2026-04-20

### Fixed

- **Release pipeline.** `scripts/check-release-assets.sh` now
  understands goreleaser 2.x's per-build output layout. Previous
  versions assumed every artefact sat at `dist/` top level, which
  was true for goreleaser 1.x but not 2.x — the latter places raw
  binaries and cosign sigstore bundles under per-build subdirs
  (`dist/clockify-mcp_linux_amd64_v1/clockify-mcp`). The v1.0.1
  release workflow hit this and the script flagged 18 of 28 assets
  as missing even though goreleaser had already published them
  under their correct names. The fix consults `dist/artifacts.json`
  for the name→path mapping when available, falls back to a
  recursive filesystem walk, and uses a precise regex to
  distinguish published assets from intermediate binary IDs. v1.0.1
  did ship its 28 assets to the GH Release but the post-check
  failure blocked npm publish + SLSA attestation; v1.0.2 is the
  clean end-to-end release.

## [1.0.1] - 2026-04-20

> **Scope note.** The eight days between v1.0.0 and v1.0.1 accumulated
> a large volume of backwards-compatible work — EnvSpec registry,
> Postgres control-plane backend, expanded auth matrix, audit
> retention reaper, transport parity matrix, async gRPC dispatch,
> SSE resume verification, and a full pre-ship gate (`make
> release-check`). No public API changed; tool names, resource URI
> templates, env-var surface, and protocol behaviour remain the v1
> baseline. The patch version reflects the absence of breaking
> changes, not the size of the delta.

### Added

- **Async gRPC Exchange dispatch.** `internal/transport/grpc/transport.go`
  now dispatches each inbound frame in its own goroutine and funnels
  all outbound frames through a single send-pump goroutine. A
  `notifications/cancelled` queued behind an in-flight `tools/call`
  now reaches the dispatcher immediately rather than waiting for the
  blocking handler to return. gRPC rows are re-enabled in the
  cancellation and `tools/list_changed` parity suites, giving those
  contracts full-transport coverage.
- **SSE `Last-Event-ID` resume parity test.** `tests/sse_resume_test.go`
  drives the streamable HTTP server through a drop-and-reconnect
  cycle, proving that `sessionEventHub`'s ring buffer replays the
  exact gap a client missed while disconnected.
- **Raw-send harness primitive + malformed-JSON parity.**
  `Transport.SendRaw` is now part of the `tests/harness` contract
  on stdio, legacy HTTP, streamable HTTP, and gRPC.
  `TestSizeLimit_MalformedJSONParity` sends a deliberately invalid
  frame and asserts every transport surfaces JSON-RPC parse error
  `-32700`. Closes the third boundary the size-limit suite had
  deferred alongside at-limit and over-limit.

- **Structured tool responses (A1).** Every successful `tools/call`
  now emits `structuredContent` alongside the existing text content
  block, validating against the tool's advertised `outputSchema`.
  Old clients that read `content[0].text` keep working unchanged.
- **Full auth matrix on legacy HTTP (A2).** `MCP_TRANSPORT=http` now
  plumbs `authn.Authenticator` (static_bearer / oidc / forward_auth).
  `mtls` on legacy HTTP is rejected at config load with a recovery
  hint (terminate TLS upstream and use `forward_auth`, or use gRPC).
- **SSE GET origin/CORS parity (A3).** `GET /mcp` now applies the
  same `AllowedOrigins` list and CORS headers as `POST /mcp`.
- **Configurable OIDC verify-cache TTL (A4).**
  `MCP_OIDC_VERIFY_CACHE_TTL` replaces the hardcoded 60s ceiling
  (clamped to `[1s, 5m]`). Startup logs a warning when raised above
  the default so the revocation tradeoff is visible.
- **Transport × auth matrix test (A5).** Every supported and
  unsupported combination is locked down at `config.Load()`.
- **Per-subject rate-limiter eviction (B3).** Idle subject entries
  are reaped on a background ticker
  (`CLOCKIFY_SUBJECT_IDLE_TTL`, `CLOCKIFY_SUBJECT_SWEEP_INTERVAL`).
  The subjects map no longer grows unbounded.
- **SSE observability counters (B4).**
  `clockify_mcp_sse_subscriber_drops_total{reason}`,
  `clockify_mcp_sse_replay_misses_total`, and
  `clockify_mcp_sessions_reaped_total{reason}` surface hub / reaper
  eviction reasons that were previously silent.
- **File-store audit cap (B5).** `MCP_CONTROL_PLANE_AUDIT_CAP`
  bounds the in-memory audit slice on the file-backed control
  plane; FIFO eviction keeps dev deployments from growing forever.
- **Fail-closed dev-backend guard (C1).** `streamable_http` refuses
  to start against a `memory`/`file://` control plane unless
  `MCP_ALLOW_DEV_BACKEND=1` acknowledges the single-process limits.
- **Bootstrap + policy drift tests (D1).** Every name in
  `AlwaysVisible`, `MinimalSet`, `Tier1Catalog`, `introspection`,
  and `safeCoreWrites` must resolve to a registered tool.
- **`make verify-bench` Makefile target (D3).** Capture a baseline
  with `make bench BENCH_OUT=.bench/baseline.txt`, then
  `make verify-bench` diffs fresh profiles via `benchstat`.
- **Descriptor-runtime contract tests (D4).** `action` const in
  every outputSchema must match the tool name; Tier 2 descriptors
  must carry `readOnlyHint`/`destructiveHint`/`idempotentHint` in
  their Annotations map.
- **Protocol-version compat suite (E1).** Negotiation, capability
  shape, and dual-emit tools/call are now asserted across
  `2024-11-05`, `2025-03-26`, and `2025-06-18`.
- **ADR 0010 — metrics stack direction (E3, proposed).** Keep the
  homegrown metrics facade for v0.x; revisit with an OTel adapter
  on the ADR 0006 pattern at v1.0.
- **Postgres control-plane backend (B1).** pgx-backed
  `controlplane.Store` implementation lives in a dedicated
  `internal/controlplane/postgres` sub-module behind `-tags=postgres`
  so the default binary stays stdlib-only (ADR 0001). Selected by
  `MCP_CONTROL_PLANE_DSN=postgres://...`; migrations are embedded,
  run under a `pg_advisory_lock`, and version-tracked in a
  `schema_migrations` table. testcontainers-based integration tests
  cover round-trip, migration idempotence, and concurrent writes.
- **Control-plane schema compat guard (E2, ADR 0011).** The applier
  refuses to boot when the database reports a schema newer than the
  embedded migrations, protecting against silent rollback over a
  forward-only change. Integration test plants a bogus version and
  asserts the refuse-to-start error.
- **`RetainAudit(ctx, maxAge)` on Store + retention reaper (B2).**
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` (default 720h, range 1h–8760h,
  0 disables) drives a 1h ticker that drops old audit events from
  both the file store and the Postgres store.
  `clockify_mcp_audit_events_retained_total{outcome="deleted|error"}`
  exposes the per-tick outcome.
- **`internal/runtime` scaffold (C2.1).** Dev-backend predicate,
  control-plane store construction (C1 fail-closed guard included),
  and the retention reaper moved out of `cmd/clockify-mcp` so the
  boot-time plumbing is unit-testable and reusable.
- **Transport dispatch extraction (C2.2).** The streamable_http,
  legacy http, grpc, and stdio arms now live in
  `internal/runtime/{streamable,legacy_http,grpc,grpc_stub,stdio}.go`
  behind `Runtime.Run(ctx)`. `cmd/clockify-mcp/main.go` is a
  ~120-line boot shim (logging, signals, OTel, metrics listener,
  BuildInfo gauge) that delegates the rest. gRPC stays behind
  `//go:build grpc` with a stub for the default binary so the
  ADR 0012 stdlib-only guarantee holds. `auth.go:buildAuthnConfig`
  deduplicates the three previously drifting `authn.Config`
  constructions (grpc had omitted `MTLSTenantHeader` and
  `OIDCVerifyCacheTTL`).

### Added (infrastructure)

- **ADR 0011 — control-plane schema versioning.** Forward-only
  embedded migrations + refuse-to-boot-on-future-schema, with
  `internal/controlplane/COMPAT.md` tracking every version and
  interface addition.
- **`-tags=postgres` CI gate.** `scripts/check-build-tags.sh`
  asserts zero pgx symbols / zero pgx rows in the default build
  and that `-tags=postgres` actually links pgx.
- **`Makefile` targets** `build-postgres` and `test-postgres` for
  the sub-module.

### Changed

- **Legacy HTTP `ServeHTTP` signature** now takes an
  `authn.Authenticator`. Callers that passed only a bearer token
  construct one via `authn.New(authn.Config{Mode: ModeStaticBearer, …})`.
- **`controlplane.Open` accepts options.** Add
  `controlplane.WithAuditCap(n)` to cap the file-backed audit
  slice; back-compat: zero args keeps the historical unbounded
  behaviour.
- **`controlplane.Store` is now an interface** (B1.0). The
  file-backed implementation is renamed to `DevFileStore`; external
  backends (Postgres today) plug in via `RegisterOpener`. Callers
  that typed `*controlplane.Store` switch to the interface type;
  in-package tests type-assert to `*DevFileStore` when they need
  unexported state. A `Close()` method releases backend-owned
  resources (pool, handles); the file store returns nil.

### Docs

- **Runbook rename.** `docs/runbooks/w2-12-digest-pinning.md` is now
  `docs/runbooks/image-digest-pinning.md`. Content unchanged; the
  internal wave label is dropped from the filename and title so the
  runbook reads as a durable operator doc rather than a ticket
  reference. The three callers in `docs/production-readiness.md`,
  `docs/verification.md`, and `docs/verify-release.md` follow.
- Auth × transport matrix in `docs/production-readiness.md` and
  `README.md` now matches the code. mTLS-on-legacy-http is
  documented as rejected; OIDC TTL + dev-backend knobs are
  listed in the main env-var table.
- `docs/production-readiness.md` gains a "Pick a control-plane
  backend" section. `MCP_CONTROL_PLANE_DSN`,
  `MCP_CONTROL_PLANE_AUDIT_CAP`, and
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` are documented in the
  README env-var table.

## [1.0.0] - 2026-04-12

Initial stable release.

> **Stability commitment.** The current API surface — tool names, resource URI templates, configuration env vars, delta-sync wire format, and JSON-RPC protocol behaviour — is now the v1 baseline. No breaking changes will be made without a major version bump. Tier 2 tool groups, the RFC 6902 JSON Patch delta format, and the gRPC transport are considered stable at this release.

### Highlights

- **124 tools** — 33 Tier 1 registered at startup, 91 Tier 2 activated on demand across 11 domain groups.
- **MCP capabilities**: `tools`, `resources` (2 concrete + 6 parametric URI templates), and `prompts` (5 built-in templates).
- **Transports**: stdio (default), streamable HTTP 2025-03-26, legacy POST-only HTTP, and opt-in gRPC (`-tags=grpc`).
- **Auth modes**: `static_bearer`, `oidc`, `forward_auth`, `mtls` — routed via a shared `authn.Authenticator` interface with per-stream validation on gRPC.
- **Four policy modes** (`read_only`, `safe_core`, `standard`, `full`) plus three-strategy dry-run for every destructive tool.
- **Three-layer rate limiting**: stdio dispatch semaphore, per-process concurrency + window limiter, and per-`Principal.Subject` sub-layer.
- **Stdlib-only default build** — the default binary links no OpenTelemetry, gRPC, or protobuf symbols. Verified in CI via `go tool nm`.
- **Opt-in observability**: OpenTelemetry tracing behind `-tags=otel`, Prometheus metrics always on, PII-scrubbed structured logs.
- **Signed releases** — cosign keyless signatures, SPDX SBOMs, and SLSA build provenance on every binary and container image.
- **Reference Kubernetes manifests** — Deployment (non-root distroless, read-only root FS), NetworkPolicy (default-deny), PodDisruptionBudget, ServiceMonitor, and PrometheusRule with multi-window burn-rate alerts for a 99.9% SLO. Helm chart and Kustomize overlays included.

[Unreleased]: https://github.com/apet97/go-clockify/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/apet97/go-clockify/releases/tag/v1.0.0
