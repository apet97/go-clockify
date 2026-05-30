# GOCLMCP Audit

**Repo:** `apet97/go-clockify` — local working tree at `/Users/15x/Downloads/WORKING/addons-me/GOCLMCP/`
**Audit date:** 2026-05-25
**Auditor:** Claude Sonnet 4.6 (read-only pass)

---

## 1. Repo Layout Summary

```
GOCLMCP/
├── cmd/clockify-mcp/          # One-user stdio entrypoint + doctor subcommand
├── internal/
│   ├── clockify/              # HTTP client, pagination, circuit-breaker, models
│   ├── config/                # OneUserConfig env loader + validators
│   ├── dryrun/                # Dry-run preview plumbing
│   ├── jsonmergepatch/        # RFC 7396 merge-patch helper
│   ├── jsonschema/            # Runtime JSON Schema validator
│   ├── logging/               # Redacting slog handler
│   ├── mcp/                   # JSON-RPC / MCP protocol core + server loop
│   ├── paths/                 # Workspace path builder + ID validation
│   ├── resolve/               # Name-to-ID resolution
│   ├── safety/                # Token store + confirmation policy
│   ├── testclockify/          # Fake Clockify HTTP server for tests
│   ├── timeparse/             # Human-date parser
│   ├── tools/                 # ~160 tool handlers + descriptor registry (~60k LOC)
│   └── tracing/               # No-op OpenTelemetry stub
├── tests/                     # Build-tagged live e2e harness (15 test files)
├── scripts/
│   ├── gen-clockify-openapi   # Ruby generator (2056 lines) — the canonical spec pipeline
│   ├── gen-tool-catalog/      # Go generator for docs/tool-catalog.{json,md}
│   ├── gen-raw-allowlist/     # Go generator for raw_allowlist_gen.go
│   └── live-clean-prefix/     # Go cleanup helper for sacrificial workspace
├── docs/openapi/              # clockify-openapi.yaml (23618 lines, 185 operationIds) + sources/
├── Makefile                   # 20 named targets incl. drift gates
├── go.mod                     # module github.com/apet97/go-clockify, go 1.25.10
└── .github/workflows/         # ci.yml, release.yml, fuzz.yml, nightly-live.yml, + 5 more
```

**Language mix:** ~90% Go, ~10% Ruby (generator script only). Shell scripts for smoke, benchmarks, and CI helpers. Single external Go dependency: `gopkg.in/yaml.v3`.

**Build / test entry points:**

- `make build` — single `go build` with `-trimpath` + ldflags version stamp
- `make test` — `go test -race -count=1 ./...`
- `make perfect` — fmt + vet + test + 7 drift checks; required before release
- `make live-contract-local` — sacrificial-workspace e2e suite (opt-in)

---

## 2. What's Working Well

**Drift-gate architecture** (`Makefile` lines 43–138) is the strongest structural asset. Five independent idempotent gates — `catalog-drift`, `openapi-drift`, `raw-allowlist-drift`, `selfinspect-drift`, `mod-tidy-drift` — each follow the same pattern: copy committed artifact, regenerate, diff, restore-on-exit. All five run in CI (`ci.yml` lines 133–244). A sixth gate (`api-parity-matrix-drift`) cross-checks the generated catalog against the live coverage ledger. The result is that _any_ silent spec or catalog drift is caught before merge rather than discovered by a downstream consumer.

**PHANTOM_PATHS quarantine** (`scripts/gen-clockify-openapi` lines 438–511) is exemplary documentation practice. Each entry carries: the live-probe date, the sandbox workspace ID used, the exact HTTP status + Clockify error code returned, the probe fixture paths, and a discrepancy ledger cross-reference. This is better provenance than most production codebases.

**Annotation tables** (`SDK_METHOD_NAMES` line 956, `PAGINATED_LIST_OPS` line 769, `LAST_PAGE_HEADER_OPS` line 833, `TAG_RENAMES` line 721, `PATH_PARAM_PATTERNS` line 753) are co-located in a single 2056-line script with comments explaining the _why_ for every non-obvious decision. The `ensure_pagination!` docblock (lines 871–893) explicitly explains why `x-fern-pagination` cannot be used and links the Fern issue tracker. Good.

**Consistent error envelope.** `ToolResult` + `ToolError` (`internal/tools/firstslice_types.go`) and `ResultEnvelope` (`internal/tools/common.go:180`) are well-defined wire types. `ToolError` includes `ErrorInfo{Code, Message}` + `RecoveryHint{Hint, Tool, Args, Retryable, RetryAfterSeconds}` — LLMs can act on these directly.

**Goroutine hygiene** in the server dispatch loop (`internal/mcp/server.go` lines 600–697): the scan goroutine selects on `stdioCtx.Done()` before pushing to the buffered `lines` channel, so it cannot leak. Tool-call goroutines are bounded by a semaphore (acquired _before_ `go func()` to prevent burst amplification) and tracked with a `sync.WaitGroup` that `Run()` defers on. Panic recovery via `RecoverDispatch` ensures a crashing tool handler never takes down the stdio loop.

**Circuit-breaker integration** (`internal/clockify/circuit_breaker.go`) with configurable failure threshold, open duration, and half-open probes. Correctly distinguishes 4xx (not upstream failure) from 5xx + transport errors (upstream failure). Configurable via five `CLOCKIFY_CIRCUIT_BREAKER*` env vars.

**Security posture:** Redacting slog handler wraps all output before reaching the log backend. API key never appears in error messages (sanitized in `APIError.Sanitized()`). Gitleaks in CI. Govulncheck in CI. Semgrep in CI. Webhook DNS validation with SSRF protection. Confirmation token store for destructive operations.

---

## 3. Code Quality Issues

### 3a. Panics in library code at registry-build time

Three `panic()` calls exist in non-test library code, all triggered at startup:

- `internal/tools/oneuser_domains.go:14` — `FullAccessRegistry()` panics on build error.
- `internal/tools/oneuser_native_descriptors.go:14` — `nativeHighValueDescriptors()` panics on build error.
- `internal/tools/firstslice_output_schemas.go:378` — panics if a registered tool has no output schema case.

The first two are "panic-at-init-time" patterns used as convenience wrappers over the `*Checked()` variants that return errors. The panic propagates through `service.FullAccessRegistry()` called from `main()`, where there is no `recover()`. This means `clockify-mcp doctor` would also crash rather than reporting a registry error message. The `*Checked()` variants exist and are already used at all call sites that can surface the error to the operator (e.g., `runWithContext` at `cmd/clockify-mcp/main.go:145`). The `panic`-wrapping convenience functions are not needed and should be removed.

**Recommendation:** Delete `FullAccessRegistry()` and `nativeHighValueDescriptors()` (the non-checked wrappers) and call `*Checked()` directly at all remaining call sites.

### 3b. Dual result-type split (`ResultEnvelope` vs `ToolResult`)

Two parallel success-result types coexist:

- `ResultEnvelope{OK, Action, Data, Meta}` at `internal/tools/common.go:180` — used by ~824 return sites.
- `ToolResult{OK, Action, Entity, IDs, Data, Meta, Changed, Warnings, Next}` at `internal/tools/firstslice_types.go:5` — richer, used by the first-slice handlers.

The asymmetry means callers receiving `any` from tool handlers must type-switch to know which envelope shape they got. The `firstslice_handlers.go` layer bridges this by wrapping `ResultEnvelope` into `ToolResult`, but the mapping is implicit. A new contributor cannot tell which type a handler _should_ return without reading the wrapping layer.

**Recommendation:** Converge on `ToolResult` as the single success type. The added fields (`Entity`, `IDs`, `Changed`, `Warnings`, `Next`) are a strict superset; `ResultEnvelope` return sites can be mechanically migrated. Worth ~1 day effort, high readability gain.

### 3c. `fmt.Errorf` without `%w` on validation errors

Many validation-path errors use `fmt.Errorf("field X is required")` without wrapping a sentinel (examples: `internal/tools/approvals.go:184,253,301`, `internal/tools/user_admin.go:312,347,405`). These are input-validation errors that callers handle by returning a tool error to the MCP client — wrapping is not strictly needed here. However, several _upstream errors_ are also propagated without `%w`:

- `internal/tools/project_view.go:552`: `fmt.Errorf("reports API enrichment disabled...")` is a sentinel-style condition that callers check for by message text, not `errors.Is`.

The golangci-lint config (`errcheck`, `staticcheck`) does not include `wrapcheck`. The linter would catch new missing wraps if added.

### 3d. Single `context.Background()` bypass

`internal/mcp/panic.go:165` uses `context.Background()` with a 1-second timeout to probe `go env GOROOT`. This is called from a `sync.Once` inside a log-path helper — safe for its purpose. Only one instance total in non-test code; not a meaningful issue.

### 3e. `internal/tools/` file count and `*Service` surface

The `tools/` package contains 160+ files and an `*Service` type with ~200 methods. This is not a bug, but it creates real onboarding friction: a new contributor cannot easily determine which file to open for a given tool. The files are named by domain (`approvals.go`, `webhooks.go`) and by layer (`*_view.go`, `*_handlers.go`, `*_schemas.go`), which partially mitigates this, but a `CODEOWNERS`-style mapping or package-doc comment explaining the layer conventions is absent.

**Receiver naming** is consistent: always `s *Service`. All exported identifiers that are part of the public tool surface have doc comments. Minor: some unexported helpers in `common.go` and `schema_helpers.go` lack comments.

### 3f. No `golangci-lint` check for `revive` or `cyclop`

The `.golangci.yml` enables only five linters: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. Higher-value linters for this codebase — `revive` (consistent Go style), `cyclop` (cyclomatic complexity), `wrapcheck` (error wrapping), `noctx` (context propagation) — are absent. Several handler functions in `approvals.go` and `user_admin.go` have cyclomatic complexity >10; these are the highest-risk bugs-per-line sites.

---

## 4. MCP Server Quality

### 4a. Tool descriptions: LLM-readable

Workflow tools have strong descriptions with concise intent statements. Example from `oneuser_workflows.go:9`:

> "Show current user, pinned workspace, timezone, features, and current timer."

Domain tools inherit descriptions from the first-slice layer and the native-descriptor indirection. The `nativeHighValueDescriptors` path derives descriptions from the raw-API tool names via `nativeDomainDescriptorMap()`, which reconstructs them from the generated spec — readable but not hand-tuned for LLM consumption.

The `ServerInstructions` constant (`internal/mcp/server.go:61`) — returned in the MCP `initialize` response — is clear and actionable: _"Use workflow tools first. Use IDs returned by previous calls."_ This is surfaced to every MCP client at session start.

**Gap:** Tool descriptions do not include examples in the description field. The `agent-cookbook.md` has examples but they are not embedded in tool metadata. LLMs that only see the tool list (not the doc set) miss the "common task guidance" the system promises.

### 4b. Tool grouping

Workflow tools (priority 0–16) are registered first, domain tools (priority 20–1104) second, raw API fallback (priority 9000–9001) last. The `RegistryForToolset()` filter exposes five named surfaces: `default`, `core`, `business`, `admin`, `all`. The `default` toolset targets the everyday use case; `all` exposes raw tools. This is well-structured.

The generated `docs/default-toolset.md` + `docs/tool-catalog.md` give operators a navigable reference. The catalog-drift gate ensures they stay current.

### 4c. Error envelope consistency

All tool errors bubble up through `firstslice_recovery.go` (or the parallel `firstSliceHandlers` closure), which translates them into `ToolError{ok:false, error:{code, message}, recovery:{hint, tool, args, retryable}}`. The MCP `tools/call` response always carries `isError: true` on the JSON-RPC result, not an RPC-level error. This is correct MCP spec behavior.

**Minor inconsistency:** The `ToolError.Recovery.Tool` field is sometimes empty on validation errors (where there is no obvious recovery tool), which is fine, but some callers set `recovery.hint` to the empty string. The wire-level contract test (`result_envelope_contract_test.go`) verifies the critical fields but does not assert `recovery.hint` is non-empty.

### 4d. Pagination handling

The MCP layer does **not** auto-paginate. Tools that list resources accept explicit `page` / `page_size` parameters (`helpers_pagination.go`). This is a conscious product decision: the MCP tool surface is intended for LLM agents that should walk pages themselves. The `clockify_review_day` / `clockify_review_week` workflows use `ListAll` internally to fetch all relevant entries without exposing raw page params.

The `addPaginationMeta()` helper (`helpers_pagination.go:42`) returns `has_more`, `total_min`, and `total_is_lower_bound` in the `meta` field so an agent knows when more pages exist. This is good design.

**The trade-off:** For large workspaces, an agent must make many sequential tool calls to list all projects/clients/users. An optional `auto_paginate: true` parameter that uses `ListAll` would reduce round-trips without removing the explicit-paging path. This is a Tier 2 improvement.

### 4e. Rate-limit awareness

Four risk-class rate limiters (`read`, `write`, `billing_admin`, `destructive`) are enforced at the MCP server layer before tool dispatch (`server.go` via `ConfigureRiskRateLimits`). The `CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE` env var provides a single override knob. The circuit breaker provides upstream-failure backoff. Retry-After header values from 429 responses are parsed and respected by the client (`client.go:710`). This is solid.

### 4f. Workspace pinning UX

The workspace is selected at startup via `CLOCKIFY_WORKSPACE_ID` — validated against MongoDB ObjectID format at load time. The `clockify_status` tool reports the active workspace ID, name, and feature plan so an LLM can confirm it is operating in the right context. The `clockify-mcp doctor --live` subcommand confirms workspace access and warns when the API key is neither owner nor admin.

**No workspace-switching mechanism exists at runtime**, which is intentional per `CONTRIBUTING.md`: "Keep the one-user product boundary explicit." This is a load-bearing non-feature.

### 4g. Auth

API key via `CLOCKIFY_API_KEY` env var only — no config file, no CLI argument. This is correct: CLI arguments leak into process lists; env vars are the standard secret-injection mechanism. The `NewClient()` call stores the key in `Client.apiKey` (unexported field, never logged). The `X-Api-Key` header is set per-request at `client.go:675`.

---

## 5. OpenAPI / Spec Generator Quality

### 5a. Idempotency and determinism

The generator is deterministic: it sorts path keys alphabetically in output (via `canonical_json` + `sort_deep`), stamps a `generated_at` timestamp in the YAML header, and runs `redact_sensitive_examples!` before output. The `openapi-drift` gate runs the generator twice and diffs; clean CI proves idempotency holds.

**One minor non-determinism risk:** `domain_from_path()` uses `File.basename` heuristics on source file names (e.g., `return "invoices" if base.include?("inveoice")` — note the typo at line 195). If a source file is renamed with a different typo, the tag assignment changes. This is defended by the fact that all source files are version-controlled, but a source-file rename would silently change tag assignments without a lint failure.

### 5b. Consistent operationIds + tags

OperationIds are derived from `SDK_METHOD_NAMES` (185 entries) with a deterministic fallback for unmapped operations. Tags are normalized via `TAG_RENAMES` (14 entries). The `normalize_op_tags!` + `normalize_security!` + `stamp_path_param_patterns!` passes run on every operation. The generated spec passes `fern check --from-openapi` (8 documented warnings for literal-vs-`{id}` siblings that are OpenAPI 3.0.3-conformant, logged in `spec/evidence/discrepancies.md`).

### 5c. Annotation tables: organization and discoverability

All annotation tables are defined as Ruby constants at module level in a single file, 438–1251 lines. They are well-commented and mutually cross-referenced. A new contributor adding a paginated endpoint needs to: (a) add to `PAGINATED_LIST_OPS`, (b) optionally add to `LAST_PAGE_HEADER_OPS`, (c) run `make gen-openapi`. The discrepancy ledger in `spec/evidence/discrepancies.md` (TS SDK side) references these tables by constant name.

**Gap:** The constants have no automated completeness check — adding a route to `PAGINATED_LIST_OPS` without a backing live probe is not detected at `make gen-openapi` time. The comment "Adding a new entry must be backed by a live probe" (line 768) is policy-only.

### 5d. Evidence comments and ledger entries

Every `PHANTOM_PATHS` entry has: date, sandbox workspace ID, exact HTTP status, Clockify error code, probe fixture path, and ledger cross-reference. This is exemplary. New phantom path additions must follow the same format to maintain the audit trail.

---

## 6. CI / Drift Gates

The `ci.yml` workflow has 14 named jobs, all running on `ubuntu-latest` with pinned action SHAs (good supply-chain hygiene). All four main drift gates run in CI: `catalog-drift`, `openapi-drift`, `raw-allowlist-drift`, `selfinspect-drift`. Additionally: `api-parity-matrix-drift`, `mod-tidy-drift`, `shellcheck`, `actionlint`, `secret-scan`, `vulncheck`. Every job has a `timeout-minutes` ceiling (5–10 min).

**One gap:** The `openapi-drift` job (`ci.yml:160`) requires Ruby to be present on the runner for `make openapi-drift`, but the job does not install Ruby — it relies on the `ubuntu-latest` runner's pre-installed Ruby. If GitHub changes the runner image and drops Ruby, this job will fail opaquely. A `ruby --version` pre-step or a `uses: ruby/setup-ruby` action would make this explicit.

**Speed:** The test job runs with `-timeout 120s`; on a warm Go module cache this is fast. Benchmark jobs are separate and use a `-count=10` sample for statistical noise reduction.

**Nightly live tests** (`nightly-live.yml`) run the sacrificial-workspace contract suite on a schedule — a strong regression net for live API drift.

---

## 7. Documentation

**README.md:** Clear install flow (npm launcher, prebuilt binary, source build), configuration table, MCP client examples. Links to the full doc set under `docs/`. No unnecessary length.

**CONTRIBUTING.md:** Project structure, common commands, PR process, commit conventions, and design principles in 100 lines. Explicitly warns about the live test sacrificial workspace.

**CLAUDE.md / AGENTS.md:** Both present and cross-referenced. CLAUDE.md (16KB) covers tactical gotchas, tool defaults, "where to look first" table, and a "what NOT to do" top-5. AGENTS.md (17KB) has the full contributor + agent contract. These are unusually complete for a project of this size.

**docs/ directory:** Generated catalogs, cookbook, error-recovery guide, dangerous-tools reference, live-tests guide, permissions matrix, performance notes, protocol notes. The generated files stay current via drift gates.

**Onboarding gap:** There is no `docs/architecture.md` or equivalent that explains the `firstslice` layer, the `ResultEnvelope` vs `ToolResult` split, or the descriptor-build pipeline (`nativeDomainDescriptorMap` → `nativeHighValueDescriptors` → `FullAccessRegistry`). A new contributor must infer this from reading multiple files. CONTRIBUTING.md lists directory names but not the call graph between layers.

---

## 8. Concrete Improvement Opportunities

### Tier 1 — High impact, low risk

**T1.1: Delete panic-wrapping convenience constructors**

- **WHY:** `FullAccessRegistry()` and `nativeHighValueDescriptors()` wrap `*Checked()` variants with `panic()`. A startup registry bug produces a binary crash rather than a clean `clockify-mcp doctor` error message. A user cannot distinguish a registry bug from a segfault.
- **HOW:** Remove `FullAccessRegistry()` and `nativeHighValueDescriptors()`. Update the 4 call sites that use the non-checked variant to call `FullAccessRegistryChecked()` / `nativeHighValueDescriptorsChecked()` and propagate the error. The `main.go` call path already does this correctly (line 145); only the convenience callers in `oneuser_domains.go` and `oneuser_native_descriptors.go` need updating.
- **EFFORT:** 2 hours.

**T1.2: Protect the `openapi-drift` CI job against Ruby availability**

- **WHY:** The job silently depends on the runner's pre-installed Ruby. GitHub has removed pre-installed language runtimes from runners before. A quiet failure here means phantom paths and pagination annotations can drift without detection.
- **HOW:** Add `uses: ruby/setup-ruby@v1` with `ruby-version: '3.3'` (or pin `ruby-version-file: .ruby-version`) before the `make openapi-drift` step in `ci.yml`. Add a `.ruby-version` file at repo root.
- **EFFORT:** 30 minutes.

**T1.3: Add `wrapcheck` + `cyclop` to golangci-lint**

- **WHY:** The current 5-linter config leaves large handler functions (approvals.go has a function exceeding cyclomatic complexity 10) and unwrapped upstream errors undetected. Wrapped errors allow callers to use `errors.Is`/`errors.As` for structured handling.
- **HOW:** Add `wrapcheck` and `cyclop` to `.golangci.yml` `enable` list. Set `cyclop.max-complexity: 12` to avoid immediate noise. Fix the ~5 high-complexity handler functions by extracting sub-functions. The `wrapcheck` pass will surface the upstream-error propagation sites for review.
- **EFFORT:** 1 day (lint config + fix pass).

### Tier 2 — Medium impact, moderate effort

**T2.1: Converge on a single result type (`ToolResult` everywhere)**

- **WHY:** The parallel `ResultEnvelope` / `ToolResult` split adds a hidden translation layer in `firstslice_handlers.go`. New contributors returning `ResultEnvelope` from a first-slice handler silently lose `IDs`, `Changed`, and `Warnings` fields. This has already happened — several workflow handlers return `ResultEnvelope` where `ToolResult.Changed` would make the mutation audit trail visible.
- **HOW:** Replace `ResultEnvelope` with `ToolResult` as the single success type. The `IDs`, `Changed`, `Warnings`, `Next` fields default to zero values (empty/nil) without changing the wire format. Migrate return sites mechanically; `go vet` and tests catch missed sites.
- **EFFORT:** 1–2 days.

**T2.2: Add an `auto_paginate` parameter to list tools**

- **WHY:** Large workspace operators must chain many `clockify_projects_list` calls to list all projects. The MCP session latency grows linearly with page count. An `auto_paginate: true` flag using the existing `ListAll[T]` helper would fix this for read-only tools without removing the explicit paging path.
- **HOW:** In `helpers_pagination.go`, add a `autoPaginateArg(args map[string]any) bool` helper. In each list-tool handler that uses `paginationFromArgs`, when `auto_paginate: true` is detected, switch to `ListAll` and return the full slice in a single envelope. Preserve `has_more: false` in meta. Add a `max_rows` safety parameter defaulting to the existing 5000 cap.
- **EFFORT:** 2–3 days (implementation + tests for each affected tool).

**T2.3: Add a layer-architecture doc to `docs/` or `CONTRIBUTING.md`**

- **WHY:** The `firstslice` → `nativeDomainDescriptorMap` → `nativeHighValueDescriptors` → `FullAccessRegistryChecked` call graph is non-obvious. Without it, contributors wire new tools to the wrong layer, producing inconsistent envelope shapes or missing drift-gate updates.
- **HOW:** Add a `docs/architecture.md` (~200 lines) covering: the five tool layers (workflow / first-slice / native-domain / native-route / raw-fallback), descriptor build sequence, how to add a new tool end-to-end, and the `ToolResult` vs `ResultEnvelope` current state with the convergence note.
- **EFFORT:** 4 hours.

### Tier 3 — Lower urgency

**T3.1: Add completeness check for `PAGINATED_LIST_OPS` in the generator** — _landed 2026-05-26._

- **WHY:** A contributor can add a route to `PAGINATED_LIST_OPS` without a live probe; there was no gate that enforced "all paginated ops must have a `x-clockify-live-status: live-success` annotation."
- **HOW:** `paginated_ops_live_evidence_errors(doc)` runs inside `validate_document` after the merge pipeline; it walks every entry in `PAGINATED_LIST_OPS`, asserts the emitted operation exists, and asserts `x-clockify-live-status ∈ {live-success, probe-documented}`. Mismatches bubble up through the existing `abort(errors.join("\n"))` so a contributor sees the failure at `make gen-openapi` time. `PAGINATED_LIVE_EVIDENCE_OK` (Set) is the single source of truth for accepted bucket names. `make perfect` green on commit; manual probe with `documented`/missing entries confirms the gate fires loudly.
- **EFFORT:** 4 hours actual — Ruby helper + comment.

**T3.2: Expand golangci-lint to cover `revive` style rules** — _phase 1 landed 2026-05-26._

- **WHY:** The code is generally idiomatic but some unexported helpers in large files (`common.go`, `schema_helpers.go`) lack comments and have inconsistent short-variable names (e.g., `p`, `s` reused as both pagination page and service receiver in different scopes).
- **HOW (phase 1, landed):** `revive` enabled in `.golangci.yml` with the three audit-specified rules (`exported`, `var-naming`, `unused-parameter`) and `enable-all-rules: false` so noisier defaults (e.g. `package-comments`) stay quiet. Five `unused-parameter` violations renamed to `_`; one `var-naming` violation (`scanApiCoverageCounts` → `scanAPICoverageCounts`) renamed. At phase-1 landing, the 813 pre-existing `exported` violations across `internal/` were silenced via a single `text: ^exported: ` exclusion with a load-bearing comment pointing back to T3.2. (Phase 2, started 2026-05-29, has since converted that blanket exclusion to per-package opt-outs and brought the godoc-clean count to eleven of thirteen `internal/` packages — see HOW (phase 2) below.)
- **HOW (phase 2, COMPLETE — finished 2026-05-30, commit `72f0ee5`):**
  Godoc-coverage campaign across `internal/`. The blanket
  `text: ^exported: ` exclusion was split into per-package `path:`
  opt-outs, then each opt-out was removed as its package reached full
  coverage. The final three packages were documented on 2026-05-30 —
  `internal/mcp` (25 exported symbols), `internal/clockify` (45), and
  `internal/tools` (128); 198 total — and their opt-outs removed. Every
  `internal/` package now enforces `exported`; no godoc opt-outs remain
  in `.golangci.yml`.
- **EFFORT:** Phase 1 ≈ 1 h actual. Phase 2 ≈ 2 h for the four small
  packages (config, resolve, safety, testclockify) plus the final
  three-package documentation pass that closed the track.

---

## 9. Risks — What NOT to Change

**Do not change the tool count at startup.** The 156-tool full registry is a product contract: `docs/tool-catalog.md` + drift gate + live tests all assert exactly this surface. Any registry size change requires updating the drift baseline, the live tests, and the catalog simultaneously.

**Do not change `ResultEnvelope` wire field names without a deprecation cycle.** The `ok`, `action`, `data`, `meta` fields are emitted to MCP clients. Callers may pattern-match on these in prompts.

**Do not add `x-fern-pagination` annotations to the generated spec.** The `ensure_pagination!` docblock explicitly documents why this breaks Fern on bare-array responses (tested against fern CLI 5.37.9). Attempting to add it will cause `fern check` failures and must stay quarantined until Fern adds bare-array support.

**Do not move the `scripts/gen-clockify-openapi` annotation tables to a separate config file.** The constants (`PHANTOM_PATHS`, `PAGINATED_LIST_OPS`, etc.) are load-bearing Ruby constants that the generator function bodies reference directly. Externalizing them to YAML or JSON would require re-testing the entire pipeline and would lose the inline comments that constitute the evidence trail.

**Do not implement workspace-switching at runtime.** The one-user / one-workspace product boundary is explicit and load-bearing: the safety system, rate limiters, and the `WorkspaceID` field in `*Service` all assume a single pinned workspace for the process lifetime. Multi-workspace support requires a different product design.

**The `go 1.25.10` pin is intentional.** The module uses `min()` and `maps.Copy()` from Go 1.21+; the pin keeps CI, local builds, and the release binaries reproducible. Do not change it without updating all Go version surfaces simultaneously (`.github/workflows/*.yml`, `CONTRIBUTING.md`, `go.mod`).
