# CLAUDE_CODE_GUIDE.md

Guide for Claude Code to continue work on the `GOCLMCP` codebase.

---

## Mission

Continue turning `GOCLMCP` into a **production-ready Go MCP server for Clockify**.

This is no longer a toy scaffold. Treat it as an actively evolving product-quality codebase with:
- MCP stdio server behavior
- Clockify API client layer
- policy/dry-run foundations
- growing Tier 1 tool surface
- report/workflow helper tools
- tests already in place

Your job is to extend it **carefully**, not explode it with random features.

---

## Primary Goal

Advance the codebase toward production readiness while preserving:
- correctness
- safety
- testability
- clear architecture
- honest behavior

Do not fake capabilities. If an upstream Clockify API surface is uncertain, prefer a safe, documented, pragmatic implementation over pretending parity exists.

---

## Current Status Snapshot

As of this handoff, the repo already includes:

### Working foundations
- MCP stdio request loop
- `initialize`, `tools/list`, `tools/call`, `ping`
- registry-based tool definitions
- policy-aware tool filtering/calling
- Clockify HTTP client with:
  - timeout
  - retries/backoff for 429/5xx
  - typed API errors
  - GET/POST/PUT/DELETE helpers
  - pagination helper (`ListAll`)
- config validation
- name/ID resolution helpers
- initial dry-run support
- tests passing

### Existing tool coverage
Read/context:
- `clockify_whoami`
- `clockify_list_workspaces`
- `clockify_get_workspace`
- `clockify_current_user`
- `clockify_list_users`
- `clockify_list_projects`
- `clockify_get_project`
- `clockify_list_clients`
- `clockify_list_tags`
- `clockify_list_tasks`
- `clockify_list_entries`

Reports/helpers:
- `clockify_summary_report`
- `clockify_weekly_summary`
- `clockify_quick_report`

Time/workflows:
- `clockify_start_timer`
- `clockify_stop_timer`
- `clockify_log_time`
- `clockify_find_and_update_entry`

### Safety foundations
- `CLOCKIFY_POLICY` with:
  - `read_only`
  - `safe_core`
  - `standard`
  - `full`
- first dry-run behavior wired in
- ambiguity-safe resolver behavior

---

## Source of Truth Files

Start with these files:

- `README.md`
- `PRODUCTION_PLAN.md`
- `cmd/clockify-mcp/main.go`
- `internal/mcp/server.go`
- `internal/tools/tools.go`
- `internal/clockify/client.go`
- `internal/resolve/resolve.go`
- `internal/policy/policy.go`
- `internal/dryrun/dryrun.go`

If you change direction materially, update:
- `README.md`
- `PRODUCTION_PLAN.md`

---

## What “good” looks like here

### 1. Prefer incremental improvement over rewrites
Do not throw away working structure unless it is clearly blocking progress.

### 2. Keep stdout clean
This is an MCP stdio server.
- stdout = protocol responses only
- logs/errors = stderr only

### 3. Fail closed
If a tool cannot safely decide:
- do not guess
- do not auto-pick ambiguous matches
- return a clear error

### 4. Use typed models where practical
Avoid turning the codebase into `map[string]any` soup.
Raw maps are okay at protocol boundaries or uncertain response edges, but prefer typed structs for stable entities.

### 5. Keep tests green
Every meaningful extension should keep:
- `go test ./...`
- `go build ./...`

passing.

### 6. Be honest in docs
If a report helper is really an aggregation over time entries, say so.
Do not claim full Clockify reports API coverage unless it actually exists.

---

## Highest-Priority Remaining Work

Continue in roughly this order.

### Priority 1 — Result envelope consistency
Normalize tool outputs so the codebase behaves predictably.

Target shape:
- `ok`
- `action`
- `data`
- `meta`

Tasks:
- audit all tool handlers in `internal/tools/tools.go`
- remove inconsistent/raw response shapes where still present
- ensure read and write tools use coherent envelopes
- keep tests aligned

### Priority 2 — Dry-run coverage expansion
Current dry-run support is only partial.

Tasks:
- identify write/destructive tools that should support `dry_run`
- expand dry-run preview logic safely
- use preview/no-op envelopes where appropriate
- do **not** pretend a destructive action was performed during dry-run

### Priority 3 — Policy refinement
Current policy foundation is good but basic.

Tasks:
- widen policy coverage as tool surface grows
- ensure `tools/list` and `tools/call` stay aligned
- consider allow/deny groups later if useful
- preserve always-available introspection/context tools if that still makes sense

### Priority 4 — More Tier 1 coverage
Potential additions if done safely:
- timer status
- get entry
- today entries
- add/update/delete entry
- create project/client/tag/task
- safer workflow helpers
- resolve/debug helpers

Do not add destructive/admin tools casually unless safety behavior arrives with them.

### Priority 5 — HTTP transport
Only after core behavior is solid.

If added:
- keep it optional
- bearer auth required
- health/ready endpoints
- don’t break stdio mode

### Priority 6 — Observability / release plumbing
After core/server behavior is stable:
- structured logs
- maybe metrics
- CI polish
- packaging/release automation

---

## Constraints and Expectations

### Do not over-promise the reports API
If you do not have high confidence in a Clockify endpoint, prefer:
- current-user time-entry aggregation
- clear naming
- explicit README note

### Do not weaken safety for convenience
Examples:
- no fuzzy destructive updates that guess the target
- no ambiguous entry selection
- no silent destructive behavior

### Do not introduce noisy architecture churn
If you split packages, do it because it genuinely improves maintainability.
Do not fragment the project just to look enterprise-y.

### Keep MCP behavior compatible
If changing request/response handling:
- maintain `initialize`
- maintain `tools/list`
- maintain `tools/call`
- preserve JSON-RPC shape

---

## Useful Commands

Run from repo root:

```bash
cd /Users/15x/.openclaw/workspace/GOCLMCP

gofmt -w ./cmd ./internal
go test ./...
go build ./...
```

Before finishing any meaningful change, run all three.

---

## Definition of “safe to merge” for your changes

A change is safe to leave behind when:
- it builds
- tests pass
- docs are not lying
- behavior is coherent
- it doesn’t silently broaden dangerous behavior

If you must leave a partial implementation, make it explicit in code comments and/or README.

---

## Recommended Next Task for Claude Code

If you want a concrete first task, do this:

### Task: Normalize result envelopes across all tool handlers

Goal:
- ensure every tool returns a predictable result envelope
- remove remaining ad hoc return shapes
- update tests accordingly

Success criteria:
- all tools consistently return the same outer structure
- `go test ./...` passes
- `go build ./...` passes
- README updated if output contract changes materially

After that, move to:
1. broader dry-run coverage
2. policy refinement
3. additional Tier 1 tools

---

## Final Instruction

Treat this codebase like something meant to survive.
Do not just make it bigger. Make it more solid.
