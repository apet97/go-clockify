# 0005 - Tool tier activation

## Status

Accepted — Tier 1 / Tier 2 split has been stable since v0.6.0; the
`clockify_search_tools` activation surface is exercised by
`internal/mcp/activation_integration_test.go`.

## Context

The Clockify API is large: 124 MCP tools cover the full surface
(timer, entries, projects, reports, invoices, expenses, scheduling,
approvals, custom fields, admin, …). Exposing all 124 in `tools/list`
on every connection produces three failure modes:

1. **Discoverability collapse.** LLMs given 124 tools to pick from
   pick the wrong one — the model spends tokens reasoning about
   tools it will never need on a typical "what did I work on this
   week" query.
2. **Token cost.** A full `tools/list` response with 124 schemas is
   ~30 KB and is sent on every reconnect. For multi-tenant
   streamable HTTP this multiplies by the session count.
3. **Surface area.** Operators auditing the running server want to
   see the smallest tool surface that still covers the dominant
   workflows. "All 124 by default" makes the audit harder.

We need a way to expose a small, well-curated default surface and
let the LLM (or the operator) widen it on demand.

## Decision

Tools split into two tiers:

- **Tier 1 (33 tools, always loaded).** Registered at startup and
  visible in `tools/list` immediately. Covers the dominant
  workflows: timer, entries, projects, clients, tags, tasks, users,
  workspaces, reports, workflows, search, context. Catalog lives in
  `internal/bootstrap/bootstrap.go:71` (`Tier1Catalog`).
- **Tier 2 (91 tools, 11 groups, on demand).** Not visible in
  `tools/list` until activated. Covers invoices, expenses,
  scheduling, time off, approvals, shared reports, user admin,
  webhooks, custom fields, groups/holidays, project admin.

Activation happens through `clockify_search_tools`, an introspection
tool that is always visible regardless of policy or bootstrap mode.
Two activation paths:

1. `activate_group: "<group>"` — activates every tool in a Tier 2
   group at once. Wired in `internal/tools/context.go` (search for `activateGroup := stringArg(args, "activate_group")`).
2. `activate_tool: "<name>"` — activates a single Tier 2 tool by
   name. Wired in the same handler.

After activation, the server emits `notifications/tools/list_changed`
on transports that support server-initiated notifications (stdio,
streamable_http, gRPC). HTTP clients on the legacy `http` transport
must re-fetch `tools/list` because the legacy transport does not
deliver server-initiated notifications.

`CLOCKIFY_BOOTSTRAP_MODE` controls the Tier 1 surface itself:

- `full_tier1` (default) — all 33 Tier 1 tools visible.
- `minimal` — only 11 core tools (`MinimalSet` in
  `bootstrap.go:56`). The rest of Tier 1 is hidden until activated.
- `custom` — operator-supplied list via `CLOCKIFY_BOOTSTRAP_TOOLS`.

The bootstrap filter and the policy filter both run inside
`Enforcement.FilterTool` (ADR 0004), so a tool is visible only when
both gates allow it.

## Consequences

### Positive

- LLMs see a small, focused tool surface on first connection (33
  tools, ~10 KB schema). Discoverability and token cost are both
  bounded.
- The 91 Tier 2 tools are still reachable: any LLM that calls
  `clockify_search_tools { "query": "invoice" }` discovers the
  invoice group and can activate it in one round trip.
- Operators auditing the running server with `clockify_policy_info`
  see the active surface, not the full 124. The surface is
  observable, not just configurable.
- Tier 2 activation is reversible per session — the activation lives
  in the `bootstrap.Config` clone owned by the session runtime, not
  in shared global state, so a multi-tenant streamable HTTP server
  can have different sessions with different active surfaces.

### Negative

- A user who knows exactly which Tier 2 tool they need still has to
  call `clockify_search_tools` first. We accept the round trip
  because the alternative (always-on full surface) costs more on the
  dominant case.
- `notifications/tools/list_changed` is the canonical signal but the
  legacy `http` transport (ADR 0002) cannot deliver it. Operators
  on legacy `http` who activate Tier 2 tools must re-fetch
  `tools/list` manually. This is documented in
  `README.md` "Troubleshooting".
- Adding a new Tier 2 group requires touching the registration
  files, the `bootstrap.Config` activation lookup, and the
  `clockify_search_tools` catalog metadata. There is no single
  source of truth — the catalog is hand-curated to keep keyword
  search good.

### Neutral

- The `MinimalSet` in `bootstrap.go:56` is opinionated about which
  11 tools are "core". Operators who disagree can switch to
  `bootstrap.full_tier1` or supply their own list via
  `bootstrap.custom`.
- `clockify_search_tools` lives in Tier 1 and bypasses the policy
  gate via the introspection allowlist. Removing it would break
  Tier 2 discoverability entirely, so any future refactor of the
  introspection allowlist must keep it.

## Alternatives considered

- **Always expose all 124 tools** — rejected on discoverability and
  token-cost grounds (see Context).
- **Static tiers, no on-demand activation** — rejected because
  operators would need to set `CLOCKIFY_BOOTSTRAP_TOOLS` for every
  workflow that touches a Tier 2 group, and the per-workflow list
  would drift from the catalog.
- **A single "tools by capability" RPC like `tools/search`** —
  partially adopted (`clockify_search_tools` plays this role) but
  layered on top of MCP's existing `tools/list` so spec-strict
  clients still see a working `tools/list` surface on connect.

## References

- Tier 1 catalog: `internal/bootstrap/bootstrap.go:71`
  (`Tier1Catalog`).
- Activation handler: `internal/tools/context.go` (search for `activateGroup := stringArg(args, "activate_group")`)
  (`activate_group` and `activate_tool` arguments).
- Bootstrap modes: `internal/bootstrap/bootstrap.go` `Mode` enum
  (search for `type Mode int`) and the `AlwaysVisible` / `MinimalSet`
  maps (search for `var AlwaysVisible`).
- Wiring: `internal/runtime/runtime.go` `New()` — bootstrap config
  is loaded by `bootstrap.ConfigFromEnv()` (search for that call)
  and threaded through `runtimeDeps`. Pre-C2.2 (dea1cc3) the wiring
  lived in `cmd/clockify-mcp/runtime.go`; that file was removed when
  the runtime extracted into `internal/runtime/`.
- Notifications: `internal/mcp/server.go` (search for
  `tools/list_changed`).
- Related ADRs: 0004 (the same `FilterTool` pipeline gates both
  policy and bootstrap visibility).
- Related docs: `README.md` "Tool tiers",
  `docs/production-readiness.md`.
