# ADR 005 — Policy modes

**Status**: Accepted, 2026-04-11.

## Context

MCP clients differ wildly in how much autonomy they should have. A
local dev environment can run with every tool unlocked, but a
shared-service deployment in production should expose only the safe
subset that won't mutate state unexpectedly. Picking the right set
of tools per environment is a recurring decision — we want it to
be a single config knob, not a bespoke allow/deny list per tenant.

## Decision

Four named policy modes, configured via `CLOCKIFY_POLICY`:

| Mode | Tool visibility | Intended use |
|---|---|---|
| `read_only` | Only `ReadOnlyHint: true` tools | Auditors, dashboards, read-only LLM workflows |
| `safe_core` | Read-only + idempotent writes (create/update) — no deletes | Managed shared services where deletes are human-only |
| `standard` | Safe core + deletes — default | Typical enterprise deployments |
| `full` | Every tool, no hints filtering | Local dev, trusted automation |

The policy gate is implemented in `internal/policy.Policy` and
consulted from two places:

1. `Enforcement.FilterTool` — hides disallowed tools from
   `tools/list` so clients don't see them at all.
2. `Enforcement.BeforeCall` — final allow/deny check right before
   handler dispatch, in case a client calls a disallowed tool by
   name without listing it first.

Hints come from each tool's registered `ReadOnlyHint`,
`DestructiveHint`, and `IdempotentHint` flags, which are set in
`registry.go` / `tier2_*.go` at registration time. The policy mode
maps hint combinations to allow/deny decisions.

In addition to the mode, `CLOCKIFY_DENY_TOOLS` and
`CLOCKIFY_DENY_GROUPS` allow an operator to explicitly deny
specific tools or Tier 2 groups on top of the mode's default.
`CLOCKIFY_ALLOW_GROUPS` turns the Tier 2 gate into a strict
whitelist.

## Consequences

- A single env var (`CLOCKIFY_POLICY`) communicates intent. Ops
  teams don't need to audit which individual tools are "safe" —
  the hint flags already mark that.
- New tools must register accurate hint flags. A tool that mutates
  state and forgets to set `DestructiveHint: true` would be
  visible under `safe_core` — a regression.
- Changes to the hint-to-allow mapping are an operational contract
  change. Add new modes rather than changing existing modes'
  semantics. The `normalizeDescriptors` helper in
  `internal/tools/common.go` normalises hints from the
  `Annotations` map into the struct fields so the two stay in sync.
- Policy decisions are visible in the audit log and the
  `clockify_mcp_tool_calls_total{outcome="policy_denied"}` metric.
  Operators can spot the "client is trying to use tools outside
  its policy" signal without reading code.
