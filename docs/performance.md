# Performance Notes

The one-user Clockify MCP has a small local runtime envelope: stdio process,
one API key, one pinned workspace, and a static tool registry.

## Hot Paths

The important local hot paths are:

- server startup and registry construction,
- `initialize`,
- `tools/list`,
- `resources/list`,
- `resources/read` for static resources,
- workflow tool calls that chain multiple Clockify API requests.

The static tool registry is cached on the service after the first build.
`tools/list` is cached by the MCP server after initialization. The
`clockify://tools` resource also uses cached registry data and returns a
defensive copy so callers cannot mutate cached state.

## Benchmarks

Run the local benchmark slice with:

```sh
go test -run=^$ -bench 'BenchmarkFullAccessRegistry|BenchmarkOneUserToolsResourceData|BenchmarkOneUserToolsListRealRegistry' ./internal/tools
```

Use benchmark numbers for regression detection, not product claims. Absolute
values vary by machine and Go version.

## Startup Expectations

Startup and list methods must not contact Clockify. This is covered by
`TestMCPStartupListingsMakeNoClockifyRequests`.

Network-backed resources are allowed to call Clockify only when the resource is
actually read. Static resources such as `clockify://tools` should remain local.

## API Latency

Most user-visible latency comes from the upstream Clockify API, not local MCP
dispatch. Workflow tools may call several endpoints to resolve names, create or
update objects, and return useful next actions. Prefer passing IDs returned by
previous calls when you want fewer API lookups.
