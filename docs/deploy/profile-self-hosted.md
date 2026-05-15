# Deployment profile: self-hosted

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


There is no registered `self-hosted` profile. This page exists only
for legacy continuity with
[`deploy/examples/env.self-hosted.example`](../../deploy/examples/env.self-hosted.example),
which pre-dates the profile registry. New deployments should choose a
concrete profile name so `clockify-mcp doctor --profile=<name>` can
audit the intended defaults.

Use one of these profiles instead:

| Shape | Profile | Verification |
|---|---|---|
| One user, local subprocess | [`local-stdio`](profile-local-stdio.md) | `make stdio-smoke` |
| Small team, one HTTP listener | [`single-tenant-http`](profile-single-tenant-http.md) | `make http-smoke` |
| Multi-tenant SaaS in your own cloud | [`shared-service`](production-profile-shared-service.md) | shared-service runbook gates |

If you already use `deploy/examples/env.self-hosted.example`, the
no-op migration is to add `MCP_PROFILE=local-stdio` for the existing
stdio + `best_effort` audit shape. See
[ADR-0015](../adr/0015-profile-centric-configuration.md) for why the
legacy shape remains documented without becoming a profile.
