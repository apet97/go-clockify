# 0005 - Startup-Loaded One-User Tool Registry

## Status

Superseded by the one-user product rewrite.

## Context

The current `clockify-mcp` process serves one local user and one pinned
workspace. The useful agent behavior is to see the complete tool surface at
startup, then choose workflow tools first.

## Decision

The runtime registry is `Service.FullAccessRegistry()`:

- all 151 tools are registered at startup,
- workflow tools appear first,
- domain tools appear second,
- raw API fallback tools appear last,
- no tool needs a separate discovery or enablement step before use.

The generated catalog in `docs/tool-catalog.md` mirrors that runtime order.

## Consequences

- Agents can rely on `tools/list` as the complete session tool list.
- The MCP server marks the tool list static for the one-user process.
- Future work should improve handler quality and schemas without changing the
  product invariant or removing tools.
