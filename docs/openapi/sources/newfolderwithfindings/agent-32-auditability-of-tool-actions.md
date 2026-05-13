# QA Agent 32 - auditability-of-tool-actions

## Verdict
**PASS WITH CONCERNS**

## What I checked

This audit assessed the go-clockify MCP server's auditability for tool actions — its ability to record a durable, queryable trail of every non-read-only tool invocation, including intent, outcome, resource IDs, and risk classification. The scope covered local/self-hosted/community readiness (not paid-hosted compliance gates).

### Areas evaluated
1. **Audit pipeline architecture** — `internal/mcp/audit.go` two-phase audit (intent → outcome), durability modes (`best_effort` / `fail_closed` / `fail_closed_strict`), auditbridge from `mcp.AuditEvent` → `controlplane.AuditEvent`
2. **Tool annotation correctness** — `readOnlyHint`, `destructiveHint`, `idempotentHint`, `riskClass`, `dryRun` annotations on all 128 tools
3. **Risk classification** — `internal/tools/risk_overrides.go` per-tool risk bits (billing, admin, permission_change, external_side_effect, destructive) and audit key capture
4. **Enforcement pipeline** — schema validation → policy gate → rate limit → dry-run intercept, all before the handler
5. **Audit event completeness** — every code path (success, tool_error, timeout, cancelled, policy_denied, rate_limited, invalid_params, dry_run, unknown_tool, audit_intent_failed)
6. **Panic recovery audit gap** — audit intent written, but outcome record missing on handler panic
7. **Doctor command** — `clockify-mcp doctor --strict` correctly reports audit posture
8. **Live API probes** — basic CRUD lifecycle against sacrificial workspace

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID |
| `clockify-api-probe-lab/CLAUDE.md` | Safety rules |
| `clockify-api-probe-lab/README.md` | Lab overview |
| `clockify-api-probe-lab/APPROVALDOC.md` | Approval API docs (referenced for context) |

## Commands run

```
# Build
go build ./...
go build -o /tmp/clockify-mcp ./cmd/clockify-mcp/

# Unit tests (all PASS)
go test -race -run "TestAudit" ./internal/mcp/ -v
go test -race -run "TestAudit|TestProd.*Audit|TestControlPlaneAuditor" ./internal/config/ ./internal/runtime/ -v
go test -race -run "TestSafety" ./internal/enforcement/ -v
go test -race -run "TestRiskClass|TestContractMatrix|TestPathSafety" ./internal/tools/ -v

# Doctor command
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  /tmp/clockify-mcp doctor --profile=local-stdio --strict

# Stdio smoke test
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> \
  /tmp/clockify-mcp stdio-smoke

# Full test suite
go test -race ./internal/mcp/ ./internal/tools/ ./internal/enforcement/ \
  ./internal/config/ ./internal/auditbridge/ ./internal/runtime/ -count=1
```

All tests passed (exit code 0, no races detected).

## Live API probes run

1. **User/workspace verification** — GET `/user`, GET `/workspaces/{id}` — both returned expected data
2. **Project CRUD lifecycle** — POST create → PUT archive → DELETE — all succeeded (project `qa-agent-32-audit-project-*`)
3. **Time entry CRUD lifecycle** — POST create → GET read → PUT update end time → DELETE — all succeeded (entry `qa-agent-32 audit test entry`)

All probes were against workspace `65b382b606de527a7ee2b60e` (WORKSPACE), user `alpettest1@gmail.com`. All test resources were cleaned up.

## Findings

### Finding 1: Handler-panic audit outcome gap (P3)

**Severity**: P3 (edge case, production handlers should not panic)

When a write-tool handler panics, the pre-handler intent audit record is already written (`mcp/tools.go:244`), but the paired outcome record is never emitted because the panic bypasses the normal error path (`mcp/tools.go:272-300`). The `RecoverDispatch` at `mcp/panic.go:55` catches the panic and logs a structured `panic_recovered` event, but does not call `recordAuditOutcome`.

**Impact**: An auditor querying for `WHERE phase='intent' AND outcome='attempted'` would see orphaned intent records for panicked calls. The operator can still correlate via the `panic_recovered` slog event (keyed by `site` + `tool`), but this requires checking logs separately from the audit store.

**Mitigation present**: The `release()` defer for rate-limit tokens still fires (it's set up before the handler call). The structured `panic_recovered` slog event includes `site`, `tool`, and panic value.

**Files**: `internal/mcp/tools.go:269`, `internal/mcp/panic.go:55`

### Finding 2: `recordAuditBestEffort` for enforcement errors uses empty Phase (P3)

**Severity**: P3 (cosmetic — doesn't break auditability, but metadata is inconsistent)

When a tool call is blocked by enforcement (schema validation, policy, rate limit), `recordAuditBestEffort` is called at `mcp/tools.go:225` with `Phase: ""` (empty string, the zero value). The two-phase model (intent/outcome) is only activated for successful gate passage. Enforcement-rejected calls get a single audit record with no phase label.

**Why this matters**: A downstream audit consumer looking for `phase='intent'` or `phase='outcome'` would not find these rejection records. The `outcome` field (e.g., `policy_denied`, `rate_limited`, `invalid_params`) is present and is the correct signal, but a consumer filtering on phase would miss them.

**Suggested improvement**: Use a new phase constant like `PhaseRejected` or reuse `PhaseOutcome` for enforcement-rejected calls so they appear in phase-filtered queries.

**Files**: `internal/mcp/audit.go:15`, `internal/mcp/tools.go:225`

### Finding 3: Risk classification is complete and correct

**Severity**: None (confirmation)

All 26 Tier-1 tools and 102 Tier-2 tools have correct:
- `readOnlyHint` (true for reads, false for writes)
- `destructiveHint` (true only for DELETE operations)
- `idempotentHint` (true for reads, updates, activations)
- `riskClass` annotations (read / write / destructive / billing / admin / permission_change / external_side_effect)
- `auditKeys` for sensitive operations (18 tools with explicit audit keys)

The `TestPolicyCoverageIsComplete` guard ensures new write tools must be added to the dispatch test table, preventing silent enforcement bypass.

### Finding 4: Foreign key consistency in audit system (positive)

**Severity**: None (positive finding)

The `auditbridge.ToControlPlaneEvent` function at `internal/auditbridge/auditbridge.go:42` synthesizes the `external_id` as `{nanos}-{sessionID}-{tool}-{phase}-{outcome}`. This design ensures that:
- Intent and outcome records for the same call share the same nanosecond timestamp
- The Postgres `ON CONFLICT (external_id) DO NOTHING` does NOT collapse intent + outcome rows
- The live test (`TestLiveCreateUpdateDeleteEntryAuditPhases`) uses the same bridge function as the runtime, preventing drift

### Finding 5: Dry-run path correctly records audit (positive)

**Severity**: None (positive finding)

The dry-run intercept path at `mcp/tools.go:229-233` calls `recordAuditBestEffort` with `outcome="dry_run"` and `reason="dry_run_intercepted"`. The mutation handler is never invoked, so no intent record is created — only a single outcome-carrying event. This is the correct behavior: a dry-run is a read-like operation from the API perspective.

## Fixes made

No code fixes were applied. The findings above are documented concerns that should be addressed by the maintainer as a follow-up. The P3 severity means they do not block community/self-hosted readiness.

## Reproduction steps for each issue

### Finding 1 (panic audit gap)
1. Register a write-tool handler that panics
2. Call the tool via `HandleWithRecover`
3. Observe that `recordAuditIntent` fires before the handler
4. The panic is caught by `RecoverDispatch` — observe `panic_recovered` in logs
5. Observe that no outcome audit record is written (the intent is orphaned)

Unit test to add:
```go
func TestAuditPhase_OutcomeWrittenOnHandlerPanic(t *testing.T) {
    rec := &recordingAuditor{}
    s := NewServer("test", []ToolDescriptor{{
        Tool:    Tool{Name: "write_tool"},
        Handler: func(context.Context, map[string]any) (any, error) { panic("boom") },
        ReadOnlyHint: false,
    }}, nil, nil)
    s.Auditor = rec
    s.AuditDurabilityMode = "best_effort"
    s.initialized.Store(true)
    
    _ = s.HandleWithRecover(context.Background(), Request{
        JSONRPC: "2.0", ID: 1, Method: "tools/call",
        Params: map[string]any{"name": "write_tool", "arguments": map[string]any{}},
    }, "test_site")
    
    // Should have intent + outcome, not just intent
    if len(rec.events) != 2 {
        t.Fatalf("expected 2 events (intent + outcome), got %d", len(rec.events))
    }
}
```

### Finding 2 (empty phase for enforcement errors)
1. Call a write tool under `read_only` policy
2. Observe `recordAuditBestEffort` is called with `outcome="policy_denied"` and `Phase=""`

## Cleanup performed

- Project `6a00fa30284e03fc79352ca1` (qa-agent-32-audit-project-*) — archived via PUT then deleted via DELETE (HTTP 200)
- Time entry `6a00fa31d9647159dc1063ab` — deleted via DELETE (HTTP 204)

## Leftover test resources

None. All resources created during this run were cleaned up.

## Severity

| Finding | Severity | Blocking? |
|---------|----------|-----------|
| Handler-panic audit outcome gap | P3 | No |
| Empty phase for enforcement errors | P3 | No |
| Risk classification completeness | None (confirmation) | No |
| Foreign key consistency (auditbridge) | None (confirmation) | No |
| Dry-run audit correctness | None (confirmation) | No |

## Files changed

None — no fixes were applied.

## Suggested next action

1. **Fix Finding 1 (P3)**: In `RecoverDispatch` or `HandleWithRecover`, after recovering a panic, call `recordAuditBestEffort` with `outcome="panic"` and `reason=<sanitized panic value>` if the recovered tool was a write tool. This requires plumbing a reference to the `Server` or a callback into `RecoverDispatch`.

2. **Fix Finding 2 (P3)**: Either introduce a `PhaseEnforcementRejected` constant and use it in `recordAuditBestEffort` calls for enforcement-rejected paths, or document that enforcement rejections are intentionally single-phase events and update the audit runbook.

3. **Add unit test for Finding 1** (see reproduction steps above) to prevent regression.

## False positives / uncertainty

- The `MCP_DISABLE_INLINE_SECRETS`, `MCP_CONTROL_PLANE_DSN`, and `MCP_AUDIT_DURABILITY` errors from `doctor --strict` are expected for the `local-stdio` profile — they are hosted-strict-posture requirements, not bugs.
- The 400 error on direct project DELETE (before archiving) is Clockify API behavior — the MCP server correctly requires archive-before-delete through its handler logic.
- The "empty phase" finding (P3) may be intentional design — read the note at `mcp/types.go:138-140`: "Empty Phase ("") is preserved for backward compatibility with audit consumers that pre-date the phased model." If the maintainer considers enforcement rejections as pre-phase events, this is not a bug.

## Final recommendation

**Community/self-hosted readiness**: The auditability system is well-designed and production-quality for community use. The two-phase audit model with `fail_closed` durability, comprehensive risk classification, and `TestPolicyCoverageIsComplete` guard make it hard to accidentally drop audit coverage for new tools. The two P3 findings are edge cases that do not affect normal operation.

**For official launch**: Fix Finding 1 (panic outcome gap) and Finding 2 (enforcement phase labels) before declaring the audit pipeline launch-ready under a paid-hosted compliance framework where auditors may query for complete intent → outcome chains.
