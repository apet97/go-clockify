# 0004 - Policy enforcement architecture

## Status

Accepted — the five policy modes and the BeforeCall pipeline have
been stable since v0.6.0 and are exercised by the dispatch-level
test harness added in `45e95a0`.

## Context

MCP clients differ wildly in how much autonomy they should have. A
local dev environment can run with every tool unlocked; a shared
production service should expose only the safe subset that cannot
mutate state unexpectedly. We want policy to be a single config knob
(`CLOCKIFY_POLICY`), not a bespoke allow/deny list per tenant — and
we want the enforcement code to live in exactly one place so a
reviewer can audit it without grepping the whole tree.

In addition to the policy gate, three other safety subsystems run on
the hot path: schema validation, rate limiting, and dry-run
interception. They share the same lifecycle (run before the handler,
short-circuit on failure, release resources on the way out) so they
should compose into a single pipeline rather than each being wired
ad-hoc per tool.

## Decision

Five named policy modes, configured via `CLOCKIFY_POLICY` and
defined as constants in `internal/policy/policy.go:13-17`:

| Mode | Read | Write | Delete | Tier 2 |
|------|:----:|:-----:|:------:|:------:|
| `read_only` | yes | no | no | no |
| `time_tracking_safe` | yes | time-entry allowlist (`timeTrackingSafeWriteList`) | no | no |
| `safe_core` | yes | allowlist (`safeCoreWriteList`) | no | no |
| `standard` (default) | yes | yes | yes | on demand |
| `full` | yes | yes | yes | yes |

Enforcement lives in `internal/enforcement.Pipeline`, which
implements the `mcp.Enforcement` interface and composes five
subsystems in a fixed order. `BeforeCall`
(`internal/enforcement/enforcement.go:65-...`) runs:

1. **Schema validation** (`internal/jsonschema.Validate`) — runs
   first so a malformed call never consumes a rate-limit slot or
   triggers a dry-run preview.
2. **Policy gate** — calls `policy.IsAllowed(name, hints.ReadOnly)`
   and rejects with `BlockReason` on failure.
3. **Rate limit acquire** (`internal/ratelimit`) — per-subject when
   a `Principal` is on the context, global-only fallback otherwise.
4. **Dry-run intercept** (`internal/dryrun`) — if `CLOCKIFY_DRY_RUN`
   is enabled and the call is destructive, replace the handler with
   one of three preview strategies (confirm-pattern, GET-counterpart,
   minimal-fallback) so no mutation actually hits the upstream API.
5. **Handler dispatch** — happens in the protocol core, not in the
   pipeline.

`Enforcement` is a pluggable interface on `mcp.Server` so the
protocol core does not import the enforcement package or any of its
dependencies. The wiring happens once in `internal/runtime/service.go`
`buildServer()` (extracted there from the pre-C2.2 `cmd/clockify-mcp/
runtime.go` during the dea1cc3 runtime extraction).

The same `Pipeline` also implements `FilterTool`, called from the
`tools/list` handler. This means tools blocked by the current policy
are hidden from the list, not just rejected at call time —
discoverability matches enforcement.

The six-element introspection allowlist
(`clockify_whoami`, `clockify_policy_info`, `clockify_search_tools`,
`clockify_resolve_debug`, plus `clockify_current_user` and
`clockify_list_workspaces` in `policy.go:174-181`) bypasses the
policy gate so an operator can always introspect the running server's
state regardless of mode.

## Consequences

### Positive

- A single env var (`CLOCKIFY_POLICY`) communicates intent. Operators
  do not need to enumerate which tools are "safe" — the registered
  hint flags already mark that, and the policy maps hints to
  decisions.
- The protocol core has zero domain imports. `internal/mcp` does not
  know about policy, dry-run, rate limiting, or schemas — it only
  sees the `Enforcement` interface.
- Adding a new safety subsystem is a one-place edit: append a step
  to `BeforeCall` and a new field to `Pipeline`. Past examples:
  the per-token rate limiter (W1-07) and runtime JSON-schema
  validation (W2-01).
- Policy decisions surface in metrics (`clockify_mcp_policy_denials_total`)
  and in the audit log, so operators can see "client is hitting
  tools outside its policy" without reading code.

### Negative

- New tools must register accurate `ReadOnlyHint` /
  `DestructiveHint` flags. A tool that mutates state and forgets the
  hint would slip through `safe_core` — this is regression-tested by
  `internal/tools/dispatch_test.go` and the contract matrix.
- The fixed pipeline order is a contract. Reordering the steps (e.g.
  rate-limiting before schema validation) would change which kind of
  failure operators see for a malformed-and-also-rate-limited
  request. The current order prefers "fail on malformed input" over
  "fail on rate limit" because the former is deterministic.

### Neutral

- The policy module is `internal/policy`, separate from
  `internal/enforcement`, even though enforcement consumes policy.
  This split keeps the policy types stable and free of pipeline
  concerns.
- Per-token rate limiting reads `Principal.Subject` from the
  context. Stdio has no principal, so it falls through to the global
  bucket; HTTP transports always have a principal after authn.

## Alternatives considered

- **Per-tool config files** — rejected because operators cannot
  audit a YAML file as fast as a single env var, and per-tool denial
  is already covered by `CLOCKIFY_DENY_TOOLS`.
- **Decorator middleware on each tool handler** — rejected because
  the enforcement order would then be implicit in the order of
  decorators, and a missed decorator on a new tool would silently
  bypass enforcement.
- **Run policy and rate-limit in parallel goroutines** — rejected
  because the gate decisions are cheap, the goroutines cost more
  than the work, and the ordering matters for error reporting.

## References

- Modes: `internal/policy/policy.go:13-17` (constants),
  `policy.go:91-115` (`IsAllowed` switch).
- Pipeline: `internal/enforcement/enforcement.go:27-...` (`Pipeline`
  struct and `BeforeCall`).
- Wiring: `internal/runtime/service.go` `buildServer()` (where
  `Pipeline` is installed on `mcp.Server`; pre-C2.2 the wiring
  lived in `cmd/clockify-mcp/runtime.go`, removed during dea1cc3).
- Dry-run strategies: `internal/dryrun/dryrun.go:62-69` (the
  `Action` enum: `ConfirmPattern`, `PreviewTool`, `MinimalFallback`,
  `NotDestructive`).
- Tool annotation source: `internal/tools/registry.go` and the
  `Tier1Catalog` slice in `internal/bootstrap/bootstrap.go:71`.
- Related ADRs: 0005 (tier activation feeds the same `FilterTool`
  pipeline).
- Related docs: `README.md` "Policy modes",
  `docs/production-readiness.md` "Threat model summary",
  `docs/troubleshooting.md`.
