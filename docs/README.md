# Documentation Index

This repo is now documented as a local one-user Clockify MCP:
one API key, one workspace id, stdio transport, full tool access at startup.

## Start Here

- [../README.md](../README.md) - setup, environment, and MCP client wiring.
- [agent-cookbook.md](agent-cookbook.md) - workflow-first examples for agents.
- [tool-catalog.md](tool-catalog.md) / [tool-catalog.json](tool-catalog.json) -
  generated list of the 156 startup-loaded tools.
- [goals/oneuser-tool-coverage.md](goals/oneuser-tool-coverage.md) -
  conservative coverage ledger for native handlers, fake smoke, and live probes.
- [live-tests.md](live-tests.md) - sacrificial-workspace live test instructions.

## Contributor Docs

- [../CONTRIBUTING.md](../CONTRIBUTING.md) - local build and test loop.
- [../GOVERNANCE.md](../GOVERNANCE.md) - maintainer and review expectations.
- [../SUPPORT.md](../SUPPORT.md) - support boundaries and upgrade expectations.
- [coverage-policy.md](coverage-policy.md) - coverage floors and ratchet rule.
- [api-coverage.md](api-coverage.md) - API coverage notes.
- [performance.md](performance.md) - benchmark posture.

## Agent Handoffs

- [agent-handoff.md](agent-handoff.md) - current repo state for autonomous agents.
- [goals/perfect-one-user-full-mcp.md](goals/perfect-one-user-full-mcp.md) -
  product-boundary spec for the one-user rewrite.

## Generated Artifacts

Regenerate the tool catalog after descriptor changes:

```sh
make gen-tool-catalog
make catalog-drift
```

Regenerate the OpenAPI artifact only when the documented fallback contract
changes:

```sh
make gen-openapi
make openapi-drift
```

## Historical Material

Older launch, deployment, profile, release, and ADR material may still exist for
audit history. Treat it as historical context, not active product guidance. The
active product path is the one-user stdio server described by the files above.

## Operator references

- [permissions.md](permissions.md) — role, plan, and feature requirements by tool family
- [dangerous-tools.md](dangerous-tools.md) — destructive / billing / admin tools and dry-run coverage
- [raw-fallback.md](raw-fallback.md) — raw API path fence and raw-write gates
- [error-recovery.md](error-recovery.md) — common `ok:false` codes and recovery
- [protocol-notes.md](protocol-notes.md) — pagination posture, progress, and rate-control model
- [release-checklist.md](release-checklist.md) — pre-release gate sequence
- [branch-protection-required-checks.md](branch-protection-required-checks.md) — the required CI check set
