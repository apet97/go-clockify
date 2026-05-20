# CONTEXT.md - Repo-Specific Language

Read this after `AGENTS.md` when a repo term feels overloaded. This file is a
compact glossary, not an architecture guide or coverage ledger.

## Glossary

**Model-visible tool surface**:
The set of tools an MCP client sees through `tools/list` for the current
`CLOCKIFY_TOOLSET`. This can be smaller than the startup registry.
_Avoid_: "all tools", "loaded tools" (those usually mean the full registry).
_See_: `docs/tool-catalog.md`, `docs/protocol-notes.md`.

**Advertised output schema**:
The optional `outputSchema` included in a `tools/list` descriptor. Omitting it
from the wire does not remove the descriptor's internal output schema or weaken
server-side validation.
_Avoid_: "schema removed", "validation disabled".
_See_: `internal/mcp/tools.go`, `internal/tools/oneuser_quality_test.go`.

**Registry vs. advertised surface**:
The full set of 156 tools that `Service.FullAccessRegistry()` loads at startup
("registry") versus the subset visible in `tools/list` to a given client
("advertised surface"). Tools not advertised remain dispatch-callable by name.
_Avoid_: "tool count", "available tools" (ambiguous between the two).

**Live evidence (protocol/recovery vs happy-path)**:
Two distinct columns in `docs/goals/oneuser-tool-coverage.md`.
Protocol/recovery evidence proves a tool returns a useful `ok:false` envelope
on a known bad input. Happy-path evidence requires `ok:true` against a real
entity. They are not interchangeable.
_Avoid_: "live coverage" alone - always qualify which column.
_See_: `docs/live-tests.md`.

**Workspace prefix**:
The token, such as `MCP-LIVE-RECON-`, attached to every entity name a live test
creates, used by `scripts/live-clean-prefix` to delete only that batch of test
data on cleanup. The sacrificial workspace is shared, so prefixes are the
isolation boundary.
_Avoid_: "test data tag", "label" - those mean other Clockify concepts.

## Relationships

- The registry is a runtime capability set; the advertised surface is a client
  context budget decision.
- Advertised output schemas are a wire-format choice; output validation remains
  an internal correctness check.
- Live evidence records what was proven against Clockify; workspace prefixes
  make those proof runs cleanable and attributable.

## Example Dialogue

Reviewer: "Did narrowing the default toolset remove the other tools?"

Answer: "No. It narrowed the model-visible tool surface. The startup registry
still loads all 156 tools, and unadvertised tools remain callable by name."

Reviewer: "If `outputSchema` is not advertised, are results unvalidated?"

Answer: "No. That only changes the descriptor returned by `tools/list`.
Server-side tests and runtime validation still use the descriptor's internal
schema."

## Flagged Ambiguities

- "Tool count" is ambiguous. Say "registry count" or "advertised count".
- "Live coverage" is ambiguous. Say "protocol/recovery evidence" or
  "happy-path evidence".
- "Workspace cleanup" is ambiguous. Say whether cleanup is prefix-scoped,
  whole-workspace audit, or manual operator action.
