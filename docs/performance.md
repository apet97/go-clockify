# Performance envelope

This document records `go-clockify`'s observed performance on a
reference machine and the safe operating envelope derived from it.
**It is a snapshot, not a service level agreement.** Numbers shift
with hardware, Go version, network locality, and the upstream
Clockify API's behavior.

The numbers exist for two reasons:

1. To give an operator a starting point for sizing
   `MCP_MAX_INFLIGHT_TOOL_CALLS` and `CLOCKIFY_RATE_LIMIT` without
   having to instrument production.
2. To give a contributor a baseline for catching performance
   regressions during development. The hot-path microbenchmarks below
   exist for *regression detection*, not capacity planning. The load
   harness in `tests/load/` exists for *envelope characterisation*.

## Reference machine

| Field          | Value                                  |
|----------------|----------------------------------------|
| Date taken     | 2026-04-13                             |
| Branch         | `wave-a`                               |
| Commit         | (this commit; check `git log -1`)      |
| OS             | macOS 15 (Darwin 24.6.0)               |
| Architecture   | `darwin/arm64`                         |
| CPU            | Apple M1 (4P + 4E)                     |
| Go             | 1.25.9                                 |
| Build tags     | none (default stdlib-only build)       |

A linux/amd64 cloud VM will produce different absolute numbers but
the *ratios* between the benchmarks should stay close to the reference.

## Hot-path microbenchmarks

All numbers below come from
`go test -bench=. -benchtime=100x -run=^$ ./internal/...` on the
reference machine. They are deterministic and short (each finishes in
under a second) so they are safe to run on a developer laptop and to
compare across PRs with `benchstat`.

| Benchmark                          | ns/op   | B/op    | allocs/op |
|------------------------------------|---------|---------|-----------|
| `BenchmarkParseDatetime`           | 873     | 316     | 7         |
| `BenchmarkParseDuration`           | 185     | 65      | 2         |
| `BenchmarkValidateID`              | 170     | 45      | 1         |
| `BenchmarkDispatchToolsList`       | 6,553   | 1,737   | 21        |
| `BenchmarkDispatchInitialize`      | 19,101  | 6,517   | 98        |
| `BenchmarkClient_Get` (localhost)  | 139,970 | 10,143  | 114       |
| `BenchmarkClient_Post` (localhost) | 125,575 | 12,818  | 135       |

How to read this:

- `ParseDatetime`, `ParseDuration`, `ValidateID` are pure-Go
  validators that run inline on every tool call. They are
  sub-microsecond. A regression that pushes them >10× would still
  not dominate dispatch overhead — but it would be a code smell
  worth investigating.
- `DispatchToolsList` is the pure dispatcher path: JSON-RPC parse →
  route → serialize, with no upstream HTTP. Five no-op descriptors
  in the registry, no enforcement pipeline. It is the floor.
- `DispatchInitialize` is the protocol entry point. Higher than
  `tools/list` because the handshake updates server state and emits a
  structured-log line per call.
- `Client_Get` / `Client_Post` measure the upstream HTTP client
  against a localhost `httptest.Server`. The 100µs+ floor is
  dominated by the connection-pool path, JSON marshal/unmarshal, and
  the HTTP/1.1 keep-alive write — not by the upstream's response
  generation, which returns a tiny payload.

### Running the benchmarks yourself

```sh
go test -bench=. -benchtime=10x -run=^$ \
  ./internal/timeparse \
  ./internal/resolve \
  ./internal/mcp \
  ./internal/clockify
```

For a regression check, capture two runs and compare:

```sh
go install golang.org/x/perf/cmd/benchstat@latest
go test -bench=. -count=10 -run=^$ ./internal/... > before.txt
# ... make change ...
go test -bench=. -count=10 -run=^$ ./internal/... > after.txt
benchstat before.txt after.txt
```

The benchmarks are **not** wired into PR CI. Microbenchmarks on
shared CI runners flake on noise; local benchstat is the
authoritative comparison. If a regression slips into `main` it
shows up the next time someone runs `make verify-bench` (not yet
defined) or compares against the table above.

## Throughput envelope (load harness)

`tests/load/main.go` drives the three-layer rate limiter (global
semaphore → global window → per-subject sub-layer) under configurable
scenarios. The harness does NOT need real Clockify credentials — it
exercises the same `RateLimiter.AcquireForSubject` entry point that
`enforcement.Pipeline.BeforeCall` uses in production.

Reference run on the same hardware as above:

```
=== scenario: per-token-saturation ===
4 tenants; noisy tenant[0] fires 10× the volume and is expected to
exhaust its per-token budget while the other three tenants keep
flowing. This is the W2-09 acceptance scenario.

scenario=per-token-saturation duration=347ms success=130 rejected=260

per-tenant breakdown:
  tenant       attempts  success  rej(pt)  rej(gl)    obs_qps
  tenant-0          300       40      260        0     115.19
  tenant-1           30       30        0        0      97.19
  tenant-2           30       30        0        0      97.18
  tenant-3           30       30        0        0      97.18

PASS — noisy tenant isolated; quiet tenants kept flowing
```

Reproduce locally:

```sh
go run ./tests/load -scenario per-token-saturation
go run ./tests/load -scenario steady
go run ./tests/load -scenario burst
go run ./tests/load -scenario tenant-mix
```

Or via CI: `gh workflow run load.yml -f scenario=per-token-saturation`.

The scenario catalogue and the W2-09 acceptance criteria live in
[`tests/load/README.md`](../tests/load/README.md).

## Recommended operating envelope

The defaults shipped in `internal/config/config.go` are tuned for a
small-team workload (~10 active users, ~100 tool calls/min steady).
Operators serving a larger workload should bump these settings in
their Kustomize overlay or Helm values:

| Workload size              | `CLOCKIFY_RATE_LIMIT` | `MCP_MAX_INFLIGHT_TOOL_CALLS` |
|----------------------------|-----------------------|-------------------------------|
| ≤10 active users (default) | 120/min               | 64                            |
| ~50 active users           | 600/min               | 128                           |
| ~100 active users          | 1200/min              | 256                           |
| >200 active users          | Run multiple replicas behind a load balancer; do not stretch one process beyond 256 inflight. |

The `MCP_MAX_INFLIGHT_TOOL_CALLS` cap is a goroutine cap on the stdio
dispatch loop and a connection cap on the HTTP transport. Raising it
above ~256 on a single process risks tail latency from goroutine
scheduling pressure; horizontal scaling is the right answer at that
point.

`CLOCKIFY_RATE_LIMIT` should always be set with the upstream
Clockify quota in mind — exceed it and the upstream will start
returning `429`s, which the local rate limiter cannot prevent. See
[`docs/runbooks/rate-limit-saturation.md`](runbooks/rate-limit-saturation.md)
for the triage flow when this happens.

## What the envelope does NOT cover

- **Long-tail latency under p99.9.** The microbenchmarks above
  measure mean. Production tail latency is dominated by upstream
  Clockify behavior (especially during their own incidents) and
  network jitter. There is no SLO for tail latency in this document.
- **Memory under sustained load.** The `tests/chaos/` package covers
  panic-recovery memory growth under fault injection but not
  long-running memory profile. If you suspect a leak, run
  `pprof` against `/debug/pprof/heap` (build with `-tags=pprof`).
- **Cold-start latency.** Startup is dominated by Go runtime init
  and tool registration, both of which are O(constant) and do not
  scale with workload. Not measured here.

## Updating this document

When the reference numbers shift by more than ~20% on the same
hardware, retake the table and bump the "Date taken" row. Keep the
old numbers in the commit message so an auditor can reconstruct the
trend from `git log docs/performance.md`.
