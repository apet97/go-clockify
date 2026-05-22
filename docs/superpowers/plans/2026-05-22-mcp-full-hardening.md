# MCP Full Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make `CLOCKIFY_TOOLSET` a real callable authority boundary, add default/risk-aware rate limits, gate raw fallback, advertise useful output contracts, add local audit logging, clarify guidance-only tools, reconcile raw allowlist generation with path policy, and paginate large `tools/list` responses.

**Architecture:** Keep the one-user full registry as the internal catalog and self-inspection source, but make dispatch authorization explicitly use the advertised toolset unless `CLOCKIFY_TOOLSET=all` or raw/audit opt-ins say otherwise. Put cross-cutting guards in `internal/mcp`, raw and guidance semantics in `internal/tools`, and user-visible contract changes in generated docs plus focused policy docs.

**Tech Stack:** Go MCP server, stdio JSON-RPC, repo-native `mcp.ToolDescriptor`, generated catalog/parity scripts, shell drift gates.

---

### Task 1: Enforce Advertised Tool Authority

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/tools.go`
- Modify: `cmd/clockify-mcp/main.go`
- Modify: `internal/mcp/server_test.go`
- Modify: `cmd/clockify-mcp/main_test.go`
- Modify: `README.md`
- Modify: `docs/policy/production-tool-scope.md`
- Modify: `docs/protocol-notes.md`
- Regenerate: `docs/tool-catalog.md`
- Regenerate: `docs/tool-catalog.json`
- Regenerate: `internal/tools/selfinspect_assets/tool-catalog.md`
- Regenerate: `internal/tools/selfinspect_assets/tool-catalog.json`

- [x] Write a failing `internal/mcp` test showing `EnforceAdvertisedTools=true` rejects a hidden loaded tool before the handler runs.
- [x] Write a preserving test showing `EnforceAdvertisedTools=false` keeps the old full-registry dispatch behavior for `CLOCKIFY_TOOLSET=all`.
- [x] Add `Server.EnforceAdvertisedTools bool`.
- [x] Check `advertisedTools` in `callTool` before schema validation and return `UnknownToolError` for hidden names.
- [x] Set `server.EnforceAdvertisedTools = cfg.Toolset != "all"` in `runWithContext`.
- [x] Update docs and generated catalog text so non-`all` toolsets are described as callable authority boundaries.
- [x] Run `go test -count=1 ./internal/mcp ./cmd/clockify-mcp`.

### Task 2: Enable Default And Risk-Aware Rate Limits

**Files:**
- Modify: `internal/config/oneuser.go`
- Modify: `internal/mcp/rate_limit.go`
- Modify: `internal/mcp/tools.go`
- Modify: `cmd/clockify-mcp/main.go`
- Modify: `internal/tools/workflows_status_and_review.go`
- Modify: `internal/mcp/rate_limit_test.go`
- Modify: `internal/tools/rate_limit_runtime_test.go`
- Modify: `cmd/clockify-mcp/main_test.go`
- Modify: `README.md`
- Modify: `docs/protocol-notes.md`

- [x] Write failing config tests for default `CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE=120` and explicit `0` opt-out.
- [x] Write failing `internal/mcp` tests for risk buckets: read/write/billing-admin/destructive limits select different buckets.
- [x] Replace single global bucket use with a risk-aware bucket set while preserving explicit global env compatibility.
- [x] Return structured `ok:false` recovery with `retryAfterSeconds`.
- [x] Add doctor warning and `clockify_status` metadata when the operator explicitly disables rate limiting.
- [x] Run `go test -count=1 ./internal/config ./internal/mcp ./internal/tools ./cmd/clockify-mcp`.

### Task 3: Gate Raw Fallback Reads And Writes

**Files:**
- Modify: `internal/config/oneuser.go`
- Modify: `internal/tools/common.go`
- Modify: `internal/tools/oneuser_raw_api.go`
- Modify: `cmd/clockify-mcp/main.go`
- Modify: `internal/tools/raw_fallback_safety_test.go`
- Modify: `internal/tools/fullaccess_test.go`
- Modify: `README.md`
- Modify: `docs/raw-fallback.md`

- [x] Write failing tests that raw GET is blocked unless `EnableRawTools` and `EnableRawGet` are true.
- [x] Write failing tests for sensitive raw GET paths blocked outside admin/all when raw GET is enabled.
- [x] Add `CLOCKIFY_ENABLE_RAW_TOOLS` and `CLOCKIFY_ENABLE_RAW_GET` parsing.
- [x] Wire service fields from config and make `CLOCKIFY_TOOLSET=all` enable raw tools by default unless explicitly gated off.
- [x] Keep raw writes behind `CLOCKIFY_ENABLE_RAW_WRITES=true`.
- [x] Run `go test -count=1 ./internal/config ./internal/tools ./cmd/clockify-mcp`.

### Task 4: Advertise Compact Output Schema Where It Matters

**Files:**
- Modify: `internal/tools/schemagen.go`
- Modify: `internal/tools/oneuser_workflows.go`
- Modify: `internal/tools/oneuser_quality_test.go`
- Modify: `docs/protocol-notes.md`
- Regenerate: `docs/tool-catalog.md`
- Regenerate: `docs/tool-catalog.json`
- Regenerate: `internal/tools/selfinspect_assets/tool-catalog.md`
- Regenerate: `internal/tools/selfinspect_assets/tool-catalog.json`

- [x] Write failing test that every default advertised tool has a visible `outputSchema` in `tools/list`.
- [x] Add or reuse a compact shared result-envelope schema with required `ok` and `action`.
- [x] Ensure all 16 default tools advertise an output schema without exploding `tools/list` size budgets.
- [x] Run `go test -count=1 ./internal/tools ./internal/mcp`.

### Task 5: Add Optional Durable Audit JSONL

**Files:**
- Create: `internal/mcp/audit_log.go`
- Create: `internal/mcp/audit_log_test.go`
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/tools.go`
- Modify: `internal/config/oneuser.go`
- Modify: `cmd/clockify-mcp/main.go`
- Modify: `internal/tools/oneuser_resources.go`
- Modify: `README.md`
- Modify: `docs/protocol-notes.md`

- [x] Write failing audit-recorder tests that side-effect calls produce redacted JSONL records.
- [x] Write failing tests that read calls are omitted in `side_effects_only` and included in `all`.
- [x] Parse `CLOCKIFY_AUDIT_LOG` and `CLOCKIFY_AUDIT_LOG_MODE=side_effects_only|all|off`.
- [x] Record timestamp, tool, risk names, redacted workspace suffix, whitelisted audit arguments, dry-run flag, result, and error code.
- [x] Expose a read-only `clockify://mcp/audit-tail` resource for local inspection.
- [x] Run `go test -count=1 ./internal/config ./internal/mcp ./internal/tools ./cmd/clockify-mcp`.

### Task 6: Rename Unsupported Operations As Guidance Tools

**Files:**
- Modify: `internal/tools/invoices.go`
- Modify: `internal/tools/admin.go`
- Modify: `internal/tools/tool_descriptors.go`
- Modify: `internal/tools/risk_overrides.go`
- Modify: `internal/tools/workflows_business.go`
- Modify: `internal/tools/invoice_view.go`
- Modify: `internal/tools/oneuser_write_contract_test.go`
- Modify: `internal/tools/oneuser_tool_description_test.go`
- Modify: `docs/dangerous-tools.md`
- Regenerate: `docs/tool-catalog.md`
- Regenerate: `docs/tool-catalog.json`
- Regenerate: `docs/api-parity-matrix.md`
- Regenerate: `internal/tools/selfinspect_assets/*`

- [x] Write failing tests that the old names are absent and new `_guidance` names are read-only.
- [x] Rename `clockify_invoices_send`, `clockify_invoices_items_update`, and `clockify_webhooks_test` to `_guidance` names.
- [x] Return `ok:false`, `supported:false`, `performed:false`, and recovery guidance.
- [x] Update workflow suggestions and generated docs.
- [x] Run `go test -count=1 ./internal/tools`.

### Task 7: Filter Raw Write Allowlist Against Path Policy

**Files:**
- Modify: `scripts/gen-raw-allowlist/main.go`
- Modify: `internal/tools/raw_allowlist_gen.go`
- Modify: `internal/tools/raw_fallback_safety_test.go`
- Modify: `docs/raw-fallback.md`

- [x] Write failing test that every generated raw write allowlist route passes `safeRawPath` after workspace substitution.
- [x] Add `documentedButRawUnsupportedRoutes` for documented global routes outside pinned workspace scope.
- [x] Update generator to put unsupported global routes in the separate map instead of the allow map.
- [x] Run `make gen-raw-allowlist` and `make raw-allowlist-drift`.

### Task 8: Add `tools/list` Cursor Pagination

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/types.go`
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/server_test.go`
- Modify: `docs/protocol-notes.md`
- Regenerate: `docs/tool-catalog.md`
- Regenerate: `docs/tool-catalog.json`

- [x] Write failing tests for `tools/list` with `cursor` returning a page and `nextCursor`.
- [x] Keep backward compatibility by returning one page when no cursor and advertised count is below threshold.
- [x] Add cursor parsing and stable numeric cursor encoding over the cached sorted tool list.
- [x] Preserve cached non-paginated response for the common default path.
- [x] Run `go test -count=1 ./internal/mcp`.

### Task 9: Regenerate, Verify, And Clean Up

**Files:**
- Regenerate: generated catalog, parity, raw allowlist, and self-inspection assets touched above.
- Modify: docs only where drift checks require it.

- [x] Run `make gen-tool-catalog`.
- [x] Run `bash scripts/check-api-parity-matrix.sh --write`.
- [x] Run `make sync-selfinspect-assets`.
- [x] Run `go test -count=1 ./internal/mcp ./internal/config ./internal/tools ./cmd/clockify-mcp`.
- [x] Run `go test -count=1 ./...`.
- [x] Run `make check`.
- [x] Run `make catalog-drift`.
- [x] Run `make raw-allowlist-drift`.
- [x] Run `git diff --check`.
