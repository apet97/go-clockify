# 0018 - Risk-class enforcement and confirmation tokens

## Status

Proposed — recorded as a placeholder for the long-standing
`.planning/loop-followups.md` queue items "Risk-class-driven
enforcement" and "Confirmation-token enforcement for minimal-fallback
destructive tools." Both have been deferred multiple times because
shipping them requires inventing a confirmation-token protocol that
does not exist in MCP today; this ADR captures the design questions
that any future implementation must answer.

## Context

Tool descriptors carry a `RiskClass` taxonomy
(`internal/mcp/server.go:134-153`):

```go
type RiskClass uint32

const (
    RiskRead             RiskClass = 1 << iota  // safe, idempotent reads
    // ...
    RiskBilling                                  // invoices / payments
    RiskAdmin                                    // workspace-admin scope
    RiskPermissionChange                         // role / permission changes
)
```

This taxonomy has been **observable** since the 2026-04-19 deeper-read
wave (`internal/tools/registry.go`, `docs/tool-catalog.json` via
commit `97c20da`). It surfaces in `docs/tool-catalog.md` as the
"Risk" column (commit `c70360a`) and in audit log payloads via
`AuditKeys`.

But the taxonomy is **observable, not enforced.** The
`enforcement.Pipeline.BeforeCall` flow at
`internal/enforcement/enforcement.go` has four decision points
(schema validation → policy gate → rate limit → dry-run intercept)
and **none** of them consult `RiskClass`. The hint surface that
travels into the pipeline (`ToolHints` in `internal/mcp/types.go`)
carries `ReadOnly`, `Destructive`, `Idempotent`, and `AuditKeys` —
no RiskClass field.

The dry-run side of the safety story was meant to be the
confirmation-token belt-and-braces:
`internal/dryrun/dryrun.go:87-103` declares `confirmTools` (an empty
map by design) and the surrounding comment explicitly documents the
gap:

> Audit finding 6 follow-up: today ConfirmPattern returns the same
> minimal envelope as MinimalFallback ... A real confirmation-token
> requirement on non-dry-run execution (e.g.
> `confirm:"delete_invoice_item:inv1:item7"`) would block the most
> dangerous "agent fires off a destructive call without dry-run
> first" scenarios; that is tracked as a separate follow-up rather
> than half-wired here.

The current safety story is:

1. `CLOCKIFY_DRY_RUN=enabled` is on by default (operators must opt
   out, not opt in).
2. The policy mode (`CLOCKIFY_POLICY`) defaults to `standard`, which
   denies the destructive tools that carry `RiskBilling`,
   `RiskAdmin`, or `RiskPermissionChange`. Operators must explicitly
   switch to `CLOCKIFY_POLICY=full` (or use the new
   `time_tracking_safe` AI-facing default in the hosted profiles)
   to expose them.
3. 18 high-risk tools currently rely on this two-layer gate:
   - **Invoices** (7): `clockify_send_invoice`, `_mark_invoice_paid`,
     `_create_invoice`, `_update_invoice`, `_delete_invoice`,
     `_add_invoice_item`, `_update_invoice_item`,
     `_delete_invoice_item`.
   - **User management** (5): `_update_user_role`,
     `_deactivate_user`, `_activate_user`, `_invite_user`,
     `_remove_user_from_workspace`.
   - **User groups** (5): `_create_user_group`, `_update_user_group`,
     `_delete_user_group`, `_add_user_to_group`,
     `_remove_user_from_group`.

The agent-with-typo scenario is specifically what confirmation
tokens are designed to defeat: an LLM that decides to delete an
invoice without first running a dry-run preview cannot get past a
gate that requires a non-default `confirm` argument keyed to the
specific resource ID.

## Decision (to be made)

Defer until the four design questions below are settled. The
substantive value of confirmation tokens only emerges when an
operator has explicitly opted into `CLOCKIFY_POLICY=full` (the
default-deny tier covers most of the threat model already), so the
current shape is acceptable for default deployments but leaves
trusted-team `full`-mode deployments without belt-and-braces.

### Q1: Confirmation-token format

Two broad shapes:

**A. Client-supplied "echo" tokens.** The client MUST pass an
argument like `confirm: "delete_invoice_item:inv1:item7"` whose
contents derive from the resource being acted on. The server checks
the token against a deterministic projection of the arguments. This
shape is simple, stateless, and matches the comment example in
`dryrun.go`. Cost: an LLM that learned the format can synthesise
tokens trivially. Mitigation: include a high-entropy suffix the
server pins on dry-run preview.

**B. Server-issued "preview" tokens.** A successful dry-run preview
returns a one-shot token (HMAC-signed, short-lived, bound to
arguments). The client must echo this token on the non-dry-run call
to be allowed to execute. Cost: server now keeps token-issuing
state (or a stateless HMAC verifier with a server-side secret); the
client flow becomes "dry-run → execute" which is a UX shift; the
MCP protocol does not have a standard way to surface "this tool
needs a token from a prior call" to the client.

**Recommendation in this ADR**: **Option B** for `RiskBilling`,
`RiskAdmin`, `RiskPermissionChange`. The HMAC variant keeps the
server stateless. The "dry-run first" UX shift is exactly the
behaviour we want — it forces the agent to preview before
executing, which is the documented intent of the existing dry-run
default.

### Q2: Where the gate lives

Three candidate sites in `enforcement.Pipeline.BeforeCall`:

- **After policy.** A policy-allowed tool with high RiskClass plus
  no token plus non-dry-run argument → reject with
  "confirmation token required". Mirrors the four existing gates
  in shape.
- **Inside dry-run.** Extend `confirmTools` to fire on every
  high-RiskClass tool; the `Action` enum gains a new
  `RequireConfirmation` constant; `executeDryRun` in
  `internal/enforcement` returns the actionable error.
- **As a new layer before dry-run.** Cleanest separation: token
  validation is its own concern, dry-run echoing is its own
  concern, the two only interact via the token-mint step on the
  dry-run path.

**Recommendation**: the third — a dedicated layer. Keeping the
concerns separated makes the audit chain clearer (an audit entry
can record "confirmation_required" vs. "policy_denied" vs.
"rate_limited" without ambiguity).

### Q3: ToolHints schema dependency

`enforcement.Pipeline.BeforeCall` consumes `ToolHints`, which
currently lacks a `RiskClass` field. The dependency chain is:

1. `internal/mcp/server.go ToolDescriptor.RiskClass` exists and is
   populated by every `tier1*` and `tier2*` registration.
2. `internal/mcp/tools.go` populates `ToolHints` from
   `ToolDescriptor` but does NOT copy `RiskClass`.
3. `internal/mcp/types.go ToolHints` would need to gain a
   `RiskClass RiskClass` field.

This is the **prerequisite refactor**: ~10 lines across three
files, no behaviour change, ships separately as a tooling commit
that makes the future feature landable. Without it, any
confirmation-token gate has no risk signal to consult.

### Q4: Client-side discoverability

How does Claude Code (or any MCP client) learn that a tool requires
a confirmation token before it tries to call?

- **MCP `tools/list` annotation.** Add a custom annotation field to
  the descriptor — the `_meta` envelope MCP defines for tool
  schemas can carry a `requires_confirmation_token: true` boolean.
  Clients that recognise it surface a UI prompt; clients that
  don't will see the server's reject error and fall back to
  whatever retry behaviour they have.
- **Server error code.** Reserve a JSON-RPC error code (e.g.
  `-32099` "confirmation token required") and standardise the
  error data shape. Clients can mechanically detect it and either
  re-run with dry-run first (Option B's intended flow) or surface
  a user-visible "this destructive action needs a preview"
  message.

**Recommendation**: ship both. Annotation is the discoverability
surface for new clients; the error code is the safety net for
clients that don't read annotations.

### Q5: Dependency on other deferred work

This ADR's predecessor in `.planning/loop-followups.md` paired
risk-class enforcement with the empty `confirmTools` follow-up.
ADR-0017 (session rehydration) is a separate concern — it touches
the transport layer; this ADR touches the enforcement layer. The
two are independent.

The single hard dependency is **the prerequisite refactor in Q3**:
add `RiskClass` to `ToolHints` and populate it from
`ToolDescriptor`. That refactor can ship today; it's not blocked on
the design questions above.

## Consequences

**If the architectural fix is landed (positive).**
- 18 high-risk tools gain a default-on safety net for
  `CLOCKIFY_POLICY=full` deployments. The agent-with-typo scenario
  is mitigated without forcing operators to manually deny tools.
- Audit chain gains structured `confirmation_required` events,
  improving incident-response coverage of "the agent attempted X
  but was blocked".
- Tool catalog (`docs/tool-catalog.md`) gains a "requires
  confirmation" column derived from the same `RiskClass` taxonomy.

**Until the architectural fix is landed (negative — accept these).**
- Operators on `CLOCKIFY_POLICY=full` rely on `CLOCKIFY_DRY_RUN`
  + audit review to catch destructive-tool typos. This is the
  current safety story for trusted-team deployments and has been
  considered acceptable since v1.0.0; documenting that explicitly
  here makes the trade-off visible.
- The `confirmTools` map at `internal/dryrun/dryrun.go:103` stays
  empty. Adding entries before the gate exists would be visible
  half-wiring (the map's behaviour would not change).

**Documentation contract (must be honoured by any implementation).**
- The chosen option for Q1 must be recorded inline in this ADR's
  Status section. Any client team consuming the MCP server needs
  the token format pinned before they can implement the receiving
  side.
- The Q2 gate placement determines audit-event shape; coordinate
  with `docs/runbooks/audit-durability.md` so operators can grep
  the right log lines.
- The Q3 prerequisite refactor SHOULD ship before this feature so
  the surface area is broken into reviewable pieces. A monolithic
  PR that does both is harder to review and harder to drift-check.

## References

- `internal/mcp/server.go:134-153` — `RiskClass` taxonomy.
- `internal/mcp/types.go` — `ToolHints` (the field that needs to
  gain `RiskClass`).
- `internal/mcp/tools.go` — the descriptor → hints copy site that
  the Q3 refactor would extend.
- `internal/enforcement/enforcement.go` — `Pipeline.BeforeCall`
  with the four existing decision points.
- `internal/dryrun/dryrun.go:87-103` — the empty `confirmTools` map
  and the comment that documents the gap.
- `internal/tools/registry.go` — descriptor registration with
  `RiskClass` populated for every Tier-1 and Tier-2 tool.
- `docs/tool-catalog.md` — the "Risk" column rendered from the
  taxonomy (commit `c70360a`).
- ADR-0004 — Policy enforcement architecture (covers the existing
  policy-mode gate this ADR slots in alongside).
- ADR-0005 — Tool tier activation (covers disclosure vs.
  availability — orthogonal concern).
- `.planning/loop-followups.md` Queue entries "Risk-class-driven
  enforcement" and "Confirmation-token enforcement for
  minimal-fallback destructive tools" — superseded by this ADR.
