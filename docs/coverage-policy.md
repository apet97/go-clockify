# Coverage policy

## The rule

**No regressions, only ratchets.** Every PR must leave coverage at or above
the current floor for every gated package AND the global total. Floors are
enforced by `scripts/check-coverage.sh`, called from CI (`coverage` job) and
from `make cover-check` / `make verify-core` locally.

Raising a floor is a one-line edit to `FLOORS_DEFAULT` in
`scripts/check-coverage.sh` and requires no discussion. Lowering a floor
requires the PR description to explain *why* the regression is acceptable;
common legitimate reasons are:

- a package shrank because dead code was deleted (the *absolute* number of
  covered lines dropped even though every remaining line is still covered),
- a whole file moved to a new package and the old package's percentage
  shifted mechanically,
- a genuinely flaky test was removed without a replacement (rare; document
  the follow-up ticket).

"The tests I'm adding in this PR push the number over the floor" is not a
valid reason to lower the floor — that's a raise, not a lower.

## Current floors (as of 2026-04-13)

These were calibrated on branch `wave-a` after the A3.1 dispatcher-level
test suite landed. Each per-package floor is set ~1% below the measured
current to leave headroom for CI flakiness from incremental coverage drift.

| Package | Floor | Measured at calibration |
|---|---|---|
| **Global** | **69%** | 69.0% |
| `internal/mcp` | 70% | 71.2% |
| `internal/tools` | 63% | 64.2% |
| `internal/clockify` | 70% | 72.8% |
| `internal/config` | 78% | 80.1% |
| `internal/enforcement` | 85% | 89.0% |
| `internal/ratelimit` | 80% | 81.8% |
| `internal/logging` | 95% | 97.2% |
| `internal/jsonschema` | 85% | 86.4% |
| `internal/authn` | 85% | 88.5% |
| `internal/policy` | 75% | 77.2% |
| `internal/resolve` | 78% | 80.8% |
| `internal/timeparse` | 88% | 90.4% |
| `internal/truncate` | 90% | 92.3% |
| `internal/tracing` | 95% | 100.0% |
| `internal/vault` | 92% | 95.2% |

## Planned ratchets

The target for the next ratchet PR is **global 70%**, which requires lifting
`internal/tools` into the low 60s. The dispatcher-level negative-path tests
in `internal/tools/dispatch_test.go` cover the enforcement surface but
intentionally do not re-cover what the service-layer tests in
`internal/tools/tools_test.go` already hit — follow-up PRs should add
harness-driven happy-path tests for the tier-1 write tools that don't yet
have them (`clockify_log_time`, `clockify_add_entry`, `clockify_update_entry`,
`clockify_find_and_update_entry`, `clockify_create_project`,
`clockify_create_task`, `clockify_switch_project`, `clockify_start_timer`,
`clockify_stop_timer`).

## Why these exact numbers

Before this calibration the CI floor was **global 55%** with six per-package
floors ranging 62–85%. That 55% was wildly below the actual measured
coverage (68.5%), so the floor was not meaningfully guarding against
regressions — a PR could delete 13% worth of tests and still merge.

The new 69% global floor closes that gap. The per-package floors cover every
non-trivial package in `internal/` (the previous six became fifteen),
including the safety-critical ones that had no floor at all: `internal/tools`,
`internal/authn`, `internal/policy`, `internal/clockify`.

## Running locally

```sh
# CI-equivalent check (runs ./internal/... under -race):
make cover-check

# Full pipeline including coverage:
make verify-core
```

Override floors for a local experiment (not for PR-gated use):

```sh
COVERAGE_GLOBAL_FLOOR=72 bash scripts/check-coverage.sh
```
