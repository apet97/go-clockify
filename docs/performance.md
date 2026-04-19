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
| Date taken     | 2026-04-14                             |
| Branch         | `perf/fast-and-quality-wave-f`         |
| Commit         | (this commit; check `git log -1`)      |
| OS             | macOS 15 (Darwin 24.6.0)               |
| Architecture   | `darwin/arm64`                         |
| CPU            | Apple M1 (4P + 4E)                     |
| Go             | 1.25.9                                 |
| Build tags     | none (default stdlib-only build)       |

A linux/amd64 cloud VM will produce different absolute numbers but
the *ratios* between the benchmarks should stay close to the reference.

### Hardware vs. CI regression gate

The numbers in this document were captured on the reference machine
above (darwin/arm64, Apple M1). The weekly regression gate in
`.github/workflows/bench.yml` runs on `ubuntu-latest` (linux/amd64)
and compares against a committed baseline at
`internal/benchdata/baseline.txt`. Absolute numbers differ between
the two platforms by ~1.5-2× in either direction depending on the
benchmark — the gate's 20% threshold works because the *ratios*
between hot paths are stable across hardware. When wall-clock
numbers in this document and the CI baseline disagree, **the CI
baseline is authoritative for regression detection** and this
document is authoritative for human understanding.

To refresh `internal/benchdata/baseline.txt` on the CI hardware,
dispatch `bench.yml` manually with `regenerate-baseline=true` and
commit the uploaded artifact (procedure in the workflow comment).

## Hot-path microbenchmarks

All numbers below come from
`go test -bench=. -benchtime=100x -run=^$ ./internal/...` on the
reference machine. They are deterministic and short (each finishes in
under a second) so they are safe to run on a developer laptop and to
compare across PRs with `benchstat`.

| Benchmark                                | ns/op   | B/op    | allocs/op |
|------------------------------------------|---------|---------|-----------|
| `BenchmarkParseDatetime`                 | 873     | 316     | 7         |
| `BenchmarkParseDuration`                 | 185     | 65      | 2         |
| `BenchmarkValidateID`                    | 170     | 45      | 1         |
| `BenchmarkDispatchToolsList`             | 6,553   | 1,737   | 21        |
| `BenchmarkDispatchInitialize`            | 19,101  | 6,517   | 98        |
| `BenchmarkClient_Get` (localhost)        | 139,970 | 10,143  | 114       |
| `BenchmarkClient_Post` (localhost)       | 125,575 | 12,818  | 135       |
| `BenchmarkPipelineBeforeCall`            | 568     | 552     | 9         |
| `BenchmarkAcquireForSubjectSteady`       | 554     | 552     | 9         |
| `BenchmarkStaticBearer` (auth)           | 235     | 48      | 1         |
| `BenchmarkOIDCVerifyCached` (auth)       | 53,774  | 5,384   | 86        |
| `BenchmarkClockifyLogTime` (amortised)   | 104,728 | 22,905  | 296       |
| `BenchmarkClockifyStartTimer` (amortised)| 71,608  | 22,569  | 278       |
| `BenchmarkClockifyStopTimer` (amortised) | 72,164  | 21,027  | 254       |
| `BenchmarkClockifyAddEntry` (amortised)  | 76,095  | 23,137  | 308       |
| `BenchmarkClockifyUpdateEntry` (amortised)| 169,481 | 35,427  | 431      |
| `BenchmarkClockifyFindAndUpdateEntry` (amortised) | 128,619 | 36,157 | 421 |

The `BenchmarkClockify*` rows are per-tool micro-benchmarks for the
Tier-1 destructive write surface. They drive one `tools/call` per
iteration through the **full** dispatch pipeline (JSON-RPC parse →
enforcement → handler → `clockify.Client.Post/Get/Put` → loopback
`httptest`). Source: `internal/tools/writes_bench_test.go`.

**The "amortised" label** on those rows is load-bearing. Prior to the
harness refactor in commit 70defbb (wave F), these benchmarks built
a fresh `tools.Service` + rebuilt the full tool registry on every
iteration, so 82% of the measured allocations came from
`applyTier1OutputSchemas` / `schemaForType` / `structSchema` and
the numbers were dominated by cold-start cost rather than the
actual handler + HTTP path. The `testharness.BenchHarness` helper
builds the stack once and reuses it — which is what a real
long-running MCP server does — so the current rows reflect real
per-dispatch cost (~250-430 allocs, ~70-170 µs) rather than
cold-boot overhead (~3000 allocs, ~300-500 µs that the pre-wave-F
numbers showed).

`BenchmarkPipelineBeforeCall`, `BenchmarkAcquireForSubjectSteady`,
`BenchmarkStaticBearer`, and `BenchmarkOIDCVerifyCached` were added
in the same wave to measure the three hot-path layers every
authenticated tools/call traverses: the enforcement pipeline, the
three-tier rate limiter, and the authn verify step. Two
observations the numbers make actionable:

1. The rate limiter is ~97% of the enforcement pipeline's cost —
   `BenchmarkAcquireForSubjectSteady` (554 ns) almost exactly
   equals `BenchmarkPipelineBeforeCall` (568 ns). Future perf work
   on "enforcement is slow" should start at ratelimit.
2. OIDC verify is ~230× more expensive than static bearer
   (53.8 µs vs 235 ns). The dominant cost is RSA-2048 signature
   verification, not JWT parsing. Any JWKS-cache design work
   needs to keep the verify call path cold-cache-free.

All numbers in the table were captured locally via
`go test -bench=. -benchmem -benchtime=1000x-3000x -count=1 -run='^$'
./internal/...` with log filtering (`2>&1 | grep -E '^Benchmark'`)
because slog output from the MCP server's initialize path
interleaves with bench output on rerun.

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
authoritative comparison. Regression checks run through the
`make verify-bench` target:

```sh
# 1. Capture a known-good baseline on the branch point.
make bench BENCH_OUT=.bench/baseline.txt

# 2. ... make changes ...

# 3. Compare. verify-bench writes a fresh profile and prints a
#    benchstat diff. Non-zero exit on unexplained regressions.
make verify-bench
```

`.bench/` is gitignored — baselines live locally per workstation.
If a regression lands in `main` without going through this flow,
the next operator to run `make verify-bench` against their own
baseline will surface it.

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

### Load Harness vs. Capacity Planning

It is critical to distinguish between what the **load harness**
(`tests/load/`) proves and what **capacity planning** requires:

- **The Load Harness proves** that the multi-layer rate limiter correctly
  isolates tenants and protects the system from internal saturation
  (e.g., a "noisy neighbor" scenario). It validates the logic of
  `CLOCKIFY_RATE_LIMIT` and `MCP_MAX_INFLIGHT_TOOL_CALLS` but does NOT
  simulate the upstream Clockify API's actual performance or latency.
- **Capacity Planning** must account for the **upstream Clockify quota**.
  If your Clockify workspace has a global rate limit (typically
  expressed in requests per minute across all API keys), your
  `CLOCKIFY_RATE_LIMIT` across all MCP replicas should not exceed it.

### Suggested Production Values

For a production deployment, use these values as a starting point and
tune based on observed metrics (`clockify_mcp_rate_limit_rejections_total`):

| User Population | `CLOCKIFY_RATE_LIMIT` | `MCP_MAX_INFLIGHT_TOOL_CALLS` | Notes |
|-----------------|-----------------------|-------------------------------|-------|
| **Small** (<20 users) | `120/min` | `64` | Standard developer team; fits well within default Clockify quotas. |
| **Medium** (20-100 users) | `600/min` | `128` | Department-level usage. Ensure your Clockify plan supports this volume. |
| **Large** (100-500 users) | `1200/min` | `256` | Multi-replica deployment recommended (e.g. 2-3 replicas). |
| **Enterprise** (>500 users)| `2400/min` | `512` (Aggregate) | Scaling horizontally is mandatory. Limit each replica to `256` inflight. |

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

## Automated regression detection

`.github/workflows/bench.yml` runs the hot-path benchmarks (the ones
listed in the "Reference numbers" table, sourced from the
`*_bench_test.go` files under `internal/`) every Monday at
04:15 UTC. It compares the fresh run against the committed baseline
at `internal/benchdata/baseline.txt` via `benchstat` and fails the
run — opening a rolling issue labelled `bench-regression` — when any
benchmark regresses by more than **20%** on `sec/op`. Negative deltas
(speedups) are never a failure.

The 20% threshold is loose enough to tolerate runner variance on
GitHub-hosted `ubuntu-latest` shared infrastructure. Microbenchmarks
on shared runners flake — the threshold exists to catch meaningful
shifts, not CPU noise. Tighten the threshold only after moving to a
self-hosted runner with dedicated hardware.

### Baseline hardware variance

The committed baseline's `goos`/`goarch`/`cpu` header must match the
CI runner hardware. Benchmarks run on a different CPU architecture
produce numbers that can't be compared to the committed baseline at
all. The seed baseline shipped in wave D was generated on Apple M1
(darwin/arm64) — the first real scheduled run on
`ubuntu-latest` will appear to regress on every metric, because the
CPUs differ.

### Bootstrap procedure (first run + intentional refresh)

Whenever the baseline needs to be regenerated on runner hardware —
either to bootstrap it the first time, after an upstream Go-runtime
upgrade, or after an intentional perf change — follow this procedure:

1. Dispatch the workflow with the regenerate input:
   ```sh
   gh workflow run bench.yml -f regenerate-baseline=true
   ```
2. Wait for the run to finish. The workflow runs the benches, uploads
   the filtered output as the `bench-current-<run-id>` artifact, and
   **skips** the regression comparison (so the run is green).
3. Download the artifact:
   ```sh
   gh run download <run-id> -n bench-current-<run-id>
   ```
4. Replace `internal/benchdata/baseline.txt` with the downloaded
   file, diff against the previous baseline so the commit message can
   document the deltas, and commit under the conventional prefix
   `chore(bench): refresh baseline`.
5. The next scheduled run (or a manual dispatch without the
   `regenerate-baseline` flag) will compare against the new baseline.

The workflow file has more detail inline; see
`.github/workflows/bench.yml`.
