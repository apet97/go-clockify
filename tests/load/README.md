# clockify-mcp load harness (W2-09)

The load harness under `tests/load/` drives the three-layer rate
limiter (global semaphore, global window, per-subject sub-layer) under
configurable tenant mixes and reports per-tenant success + rejection
counters. It does not need Clockify credentials — it exercises
`ratelimit.RateLimiter.AcquireForSubject` directly, which is the same
entry point `enforcement.Pipeline.BeforeCall` uses in production after
reading the `Principal` off the request context.

The harness is intentionally **not** a test file (no `_test.go`
suffix). It ships as a `package main` binary so operators can invoke
it via `go run` without needing a build tag or `CLOCKIFY_RUN_LIVE_E2E`
gate. Run it on demand; do not put it on the PR critical path.

## Running

```bash
# List available scenarios:
go run ./tests/load -list

# Run a specific scenario:
go run ./tests/load -scenario steady
go run ./tests/load -scenario burst
go run ./tests/load -scenario tenant-mix
go run ./tests/load -scenario per-token-saturation
go run ./tests/load -scenario ratelimit-reap-correctness
```

## Scenarios

| Scenario | Shape | Purpose |
|---|---|---|
| `steady` | 5 tenants × 20 calls, 5 ms pacing | Baseline — rate limiter should pass every call. Useful for confirming the harness itself is correctly wired before running the adversarial scenarios. |
| `burst` | 5 tenants × 50 calls, no pacing | Maximum throughput with a small global concurrency cap (20). Stress test for the global semaphore layer. |
| `tenant-mix` | 10 tenants, tenant-0 fires 5× | Realistic multi-tenant mix with one noisy neighbour. Should show per-token rejections concentrated on tenant-0 without starving the others. |
| `per-token-saturation` | 4 tenants, tenant-0 fires 10× | **W2-09 acceptance scenario.** The noisy tenant is expected to exhaust its per-token budget while quiet tenants keep flowing at 100% success. The harness encodes an explicit acceptance check that the noisy tenant's per-token rejections exceed 3× the quiet average, otherwise it `log.Fatal`s. |
| `ratelimit-reap-correctness` | 2 tenants, noisy tenant-0 saturates → idles past one window → resumes | Verifies the per-subject limiter reaps correctly: after the noisy tenant idles past one rate-limit window, the reap must restore its full budget while the cold tenant stays unaffected. Two-phase scenario; uses a short 1.5 s window so the reap completes in seconds. |

## Acceptance criteria

Running `go run ./tests/load -scenario per-token-saturation` must print:

```
PASS — noisy tenant isolated; quiet tenants kept flowing
```

A `FAIL` outcome indicates the per-token sub-layer regressed and
should be investigated before shipping a release. Typical regressions:

- A PR that inlines per-subject bookkeeping into the global window
  limiter, losing the isolation boundary.
- A PR that introduces a shared cache keyed on global state rather
  than per-subject state.
- A PR that alters the `AcquireForSubject` release order so the
  global slot is released before the per-subject slot, creating a
  window where the global budget is double-counted.

## CI integration

`.github/workflows/load.yml` runs the harness on `workflow_dispatch`
only. It is never on the PR critical path because the harness is
deliberately noisy with timing assertions that can flake on slow
runners. When an operator suspects a regression they trigger the
workflow manually; otherwise, the local `go run` invocation is
preferred.

## Adding a scenario

Scenarios are Go structs in the `scenarios` map inside `main.go`.
Add a new entry; it is picked up automatically by `-scenario <name>`
and `-list`. Every scenario has a `description` field so `-list`
produces self-documenting output.

The workflow intentionally does not use YAML config files — Go
struct literals give type safety at compile time and avoid one more
schema to maintain.
