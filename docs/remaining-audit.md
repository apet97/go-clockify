# Remaining audit

This note captures the current review context for the one-user Clockify MCP.
It is a handoff for follow-up audit work, not a launch-ready claim.

## Repo context

- `AGENTS.md` is the tracked binding contract; `CLAUDE.md` is local
  workstation context.
- Product shape is one local trusted user, one `CLOCKIFY_API_KEY`, one required
  `CLOCKIFY_WORKSPACE_ID`, and stdio transport only.
- `Service.FullAccessRegistry()` must keep workflow tools first, domain tools
  second, and raw API fallback last.
- The generated startup catalog currently contains 156 tools.
- `docs/tool-catalog.md` and `docs/tool-catalog.json` are generated. Regenerate
  them after any descriptor, schema, or tool-order change.
- `docs/goals/oneuser-tool-coverage.md` is the conservative coverage ledger.
  Fake smoke, live protocol/recovery, and live happy-path evidence are separate
  claims.

## Current remaining ledger gaps

As of this audit handoff, the coverage ledger reports:

- Total tools: 156.
- Fake-smoke coverage: 156.
- Live protocol/recovery coverage: 152.
- Live happy-path coverage: 129.
- `clockify_invoices_info` and `clockify_scheduling_publish` still need named
  live evidence before their live protocol/recovery or happy-path columns can be
  marked complete.
- Raw fallback tools remain raw-fallback-only and must stay last in the catalog.

## Audit priorities

1. Preserve product invariants before expanding coverage: local one-user,
   stdio-only, pinned workspace, and full startup registry.
2. Add only evidence-backed ledger changes. Do not count fake-server coverage,
   unavailable-feature recovery, or bogus-ID recovery as happy-path proof.
3. Prefer workflow and domain tool fixes over raw API fallback usage unless a
   typed tool genuinely does not fit the user intent.
4. Keep destructive, externally noisy, admin, billing, and permission-changing
   paths behind explicit dry-run or live-test gates.
5. Run focused tests for touched surfaces, plus catalog drift checks whenever
   descriptors, schemas, or ordering change.

## MCP server axioms

1. The MCP server is the source of truth; the model may infer, but the server
   must validate.
2. Tool names are routing primitives; name tools by user intent, not internal
   implementation.
3. One tool equals one agent-intelligible action.
4. Endpoint parity is internal; intent parity is external.
5. Every parameter must define meaning, format, source, constraints, defaults,
   examples, and allowed values.
6. Unknown or ambiguous identity must stop writes.
7. Reads should be forgiving; writes must be strict.
8. Every mutation must return a receipt proving what happened.
9. Success must not hide partial failure.
10. Errors must be recovery instructions, not just failure labels.
11. Empty results must explain what was searched and what to try next.
12. State, cursors, sessions, and retry tokens must be explicit and opaque.
13. Unsafe operations must be idempotent or duplicate-safe.
14. Time, timezone, and resolved date ranges must always be explicit.
15. Output must be structured first, textual second.
16. The model should never need to parse prose to continue work.
17. Tool annotations are hints, not security boundaries.
18. Least privilege beats convenience.
19. Full coverage must not mean full noise.
20. Generated code is allowed; generated behavior is not.
21. Invalid states should be unrepresentable through the tool schema.
22. Defaults must be visible in responses.
23. Tool descriptions are executable UX, not passive documentation.
24. Every tool must have a failure contract.
25. Performance is part of correctness.
26. The server must degrade gracefully when features, permissions, or plans
    block an operation.
27. Every tool call must be observable.
28. Backward compatibility beats clever renames.
29. The MCP boundary must be tested directly.
30. Perfect means recoverable, not magical.
