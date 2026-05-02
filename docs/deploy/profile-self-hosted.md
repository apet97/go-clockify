# Deployment profile: self-hosted

> There is no `self-hosted` profile name in the registry — this
> document describes the **shape** covered by
> [`deploy/examples/env.self-hosted.example`](../../deploy/examples/env.self-hosted.example),
> which pre-dates the Wave I profile system.
>
> Operators who match this shape should either:
>
>   1. Apply `clockify-mcp --profile=local-stdio` (stdio transport, no
>      HTTP listener) — the most common self-hosted single-user case,
>      or
>   2. Apply `clockify-mcp --profile=single-tenant-http` (HTTP on one
>      bind, static bearer auth, file-backed control plane) — when a
>      small team shares one workspace over HTTP.
>
> The legacy example file at `deploy/examples/env.self-hosted.example`
> is kept to preserve operator muscle memory; new deployments should
> pick a profile name instead. See
> [ADR-0015](../adr/0015-profile-centric-configuration.md) for why.

## When "self-hosted" means stdio

Single user, single machine, MCP client spawns the binary as a
subprocess. Apply `--profile=local-stdio`; see
[`profile-local-stdio.md`](profile-local-stdio.md) for wiring.

## When "self-hosted" means small-team HTTP

One process, one team, one workspace, shared via a TLS-terminating
reverse proxy. Apply `--profile=single-tenant-http`; see
[`profile-single-tenant-http.md`](profile-single-tenant-http.md)
for the TLS / bearer-token / file-backed control-plane setup.

## When "self-hosted" means multi-tenant SaaS in your own cloud

This is the shared-service profile, not self-hosted. See
[`production-profile-shared-service.md`](production-profile-shared-service.md).

## Upgrade path from the legacy env file

If you are currently running with
`deploy/examples/env.self-hosted.example`, the no-op upgrade is to
add `MCP_PROFILE=local-stdio` to the file (matches the existing
stdio + `best_effort` audit shape). The profile then turns into a
documented default surface that the `clockify-mcp doctor`
subcommand can audit.

## How to verify this deployment

`self-hosted` is not a registered profile name, so verification starts
by choosing the concrete profile that matches your shape:

| Shape | Profile | Doctor command | Smoke target |
|---|---|---|---|
| One user, local subprocess | `local-stdio` | `CLOCKIFY_API_KEY=pk_xxx clockify-mcp doctor --profile=local-stdio` | `make stdio-smoke` |
| Small team, one HTTP listener | `single-tenant-http` | `CLOCKIFY_API_KEY=pk_xxx MCP_BEARER_TOKEN=<32+ random chars> clockify-mcp doctor --profile=single-tenant-http` | `make http-smoke` |

For both rows, `doctor --strict` is a negative hosted-posture check,
not the success criterion. It is still useful during migration:

```bash
CLOCKIFY_API_KEY=pk_xxx \
  clockify-mcp doctor --profile=local-stdio --strict
```

Expected result for the local row is exit 3 with hosted-strict
findings; the single-tenant row behaves the same unless you have
promoted it all the way to Postgres + `fail_closed`, in which case it
is no longer this legacy self-hosted shape and should move to
`production-profile-shared-service.md`.

Use the smoke target named in the table as the CI-backed proof that
the selected transport still speaks MCP.
