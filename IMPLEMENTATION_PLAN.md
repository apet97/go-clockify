# Implementation Plan

**Date:** 2026-04-06
**Based on:** PRODUCTION_READINESS_PLAN.md audit

## Phase B — Critical Correctness (dry-run)

| # | Item | Status | Rationale |
|---|------|--------|-----------|
| B1 | Fix CheckDryRun to pass through for non-destructive write tools | DONE | Reordered delete(args, "dry_run") to after isDestructive check |
| B2 | Remove confirmTools dead entries, fix ConfirmPattern to use MinimalResult | DONE | All 4 tools use handler-level dry-run; ConfirmPattern no longer executes handler |
| B3 | Wire CLOCKIFY_DRY_RUN env var to enforcement pipeline | DONE | Gate dryrun.CheckDryRun behind p.DryRun.Enabled check |
| B4 | Update tests for new dry-run behavior | DONE | Updated enforcement_test.go, dryrun_test.go, integration_test.go |

## Phase C — High Priority Operational

| # | Item | Status | Rationale |
|---|------|--------|-----------|
| C1 | Add defer client.Close() | DONE | Prevents connection pool leak on shutdown |
| C2 | Add Vary: Origin on reflected CORS | DONE | Prevents cache poisoning |
| C3 | Log srv.Shutdown() error | DONE | Was silently discarded |
| C4 | Remove CLOCKIFY_REPORTS_URL | DONE | Dead config — Go implementation builds reports from time entries |
| C5 | Wire CLOCKIFY_TIMEZONE to Service | DONE | Now used as default fallback in loadLocation |
| C6 | Enhance /ready with upstream health check | DONE | GET /api/v1/user with 15s cache, returns 503 on failure |
| C7 | Add Access-Control-Max-Age to preflight | DONE | Prevents per-request preflight |

## Phase D — Medium Priority

| # | Item | Status | Rationale |
|---|------|--------|-----------|
| D1 | Add MCP_HTTP_MAX_BODY upper bound (50MB) | DONE | Prevents misconfiguration |
| D2 | JSON-RPC id type validation | DONE | Validates string/number/null per spec |
| D3 | JSON-RPC version validation | DONE | Validates jsonrpc=="2.0" per spec |
| D4 | Remove WriteResult dead code | DONE | Never called; updated CLAUDE.md/AGENTS.md |
| D5 | Fix fragile error string matching in SwitchProject | DONE | Uses typed clockify.APIError now |
| D6 | Add truncation warning for 100-entry report cap | DONE | Warning in meta when result count == page size |
| D7 | Add Access-Control-Max-Age to preflight | DONE | 86400s cache |
| D8 | Documentation fixes (bootstrap/policy, HTTP notifications, INSECURE, Standard/Full) | DONE | Updated safe-usage.md, http-transport.md, README.md |

## Deferred

| Item | Reason |
|------|--------|
| M-4: List tools pagination | Schema changes affect golden tests; separate PR |
| M-7: golangci-lint in CI | CI infra change; separate PR |
| M-8: SHA-pin GitHub Actions | CI infra change; separate PR |
| M-11: Fuzz tests | Nice-to-have; separate PR |
| L-6: Rate limiter over-counting | Fails safe; low priority |
| L-7: ListAll pagination ceiling | Acceptable limit (50k) |
| L-5: Truncation array metadata | Would break consumers |
| L-10: Coverage threshold | Requires coverage investment first |
| L-11: govulncheck | CI infra change; separate PR |
