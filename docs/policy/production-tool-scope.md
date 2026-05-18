# One-User Tool Scope

The current product is a local, one-user, full-access Clockify MCP for a single
pinned workspace. There are no runtime allow lists to configure: all 156 tools
are registered at startup and visible in `tools/list`.

## Scope Rules

- Use workflow tools first for agent-facing work.
- Use domain tools for exact Clockify object operations.
- Use raw fallback tools only when no workflow or domain tool fits.
- Keep all calls within `CLOCKIFY_WORKSPACE_ID`.
- Preserve ID-rich write results and structured recoverable failures.

## Risk Metadata

Tool descriptors still expose MCP hints and risk metadata so clients can make
better choices:

- `readOnlyHint`
- `destructiveHint`
- `idempotentHint`
- `annotations.riskClass`
- `annotations.handlerKind`

This metadata is descriptive. It does not hide tools at runtime.

## Write Behavior

Write-style workflow tools return:

- `ids` for workspace/user/entity IDs the next call can reuse.
- `changed.created`, `changed.updated`, `changed.deleted`, or
  `changed.reused` when applicable.
- `next` actions when another tool is a natural follow-up.

If Clockify rejects a call in a way the agent can recover from, the tool returns
`ok:false` with `error` and `recovery` fields rather than leaving the client to
guess.

## Safety

- Never log or commit API keys.
- Never broaden live tests beyond the configured sacrificial workspace.
- Do not remove tools as a cleanup tactic.
- Do not weaken validation to make a fake or live test pass.
- Keep generated tool docs in lockstep with descriptor changes.
