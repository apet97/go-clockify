# Mutation testing floors

This file is the source of truth for the per-package mutation-score
floors enforced by `.github/workflows/mutation.yml`. The workflow runs
[gremlins](https://gremlins.dev/) nightly at 02:00 UTC plus on manual
`workflow_dispatch`. Each package target fails the job if its
efficacy (the percent of KILLED mutants over KILLED + LIVED mutants)
drops below the floor listed here.

## Why mutation testing

Coverage is necessary but not sufficient: a test suite can execute
every line of source while asserting nothing meaningful. Mutation
testing catches that by generating small, semantics-breaking edits
(negating conditionals, flipping comparison boundaries, changing
arithmetic operators, dropping return values, etc.) and asserting
that the test suite fails on each mutant. A mutant that survives
means the test suite did not distinguish the mutant from the
original — either the assertion was missing, the test case was
absent, or the branch was dead code.

The goal is not 100% mutation score. The goal is an early-warning
signal when a package's test discipline regresses: if a PR lowers
the mutation score below the floor, the nightly run flags it.

## Floors

The floors start conservative (30–40%) and are ratcheted up as the
suites get tightened. Raise a floor only after a run lands above the
new value — never lower a floor to paper over a regression. When a
floor is raised, document the bump here with a commit SHA.

| Package | Floor (efficacy %) | Rationale | Last raised |
|---|---|---|---|
| `internal/jsonschema` | 40 | Stdlib-only schema validator with clear input/output contracts. Single-purpose package, easy to cover deeply. | initial |
| `internal/enforcement` | 40 | The full tool-call gate pipeline. Every branch is exercised by at least one integration test, so the mutant kill rate should be well above the floor. | initial |
| `internal/ratelimit` | 40 | Three-layer rate limiter with deterministic time-based tests. Non-time-dependent mutants are straightforward to kill. | initial |
| `internal/mcp` | 35 | Protocol core with a lot of JSON-RPC boilerplate. Floor starts lower because dispatch plumbing is harder to kill via unit tests alone. | initial |
| `internal/truncate` | 40 | Schema-stable truncation walker — property tests give strong kill signals. | initial |
| `internal/tools` | 30 | Tier 1 + Tier 2 handlers with a lot of repetitive HTTP-client wrappers. Floor starts low because many handlers are thin wrappers over the clockify client and mutations there are hard to catch without an integration test. | initial |

## Running locally

```bash
# Single package:
make mutation PKG=internal/jsonschema

# Dry-run (no test execution, just mutant generation):
gremlins unleash --dry-run ./internal/jsonschema

# Explicit efficacy threshold:
gremlins unleash --threshold-efficacy 40 ./internal/jsonschema
```

Gremlins installs via `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`.

## Interpreting output

Gremlins prints per-mutation status codes:

- `KILLED` — the test suite detected the mutant (good).
- `LIVED` — the test suite passed against the mutant (bad — coverage
  gap).
- `NOT COVERED` — no test exercises the mutated line at all (worse
  gap, but scored outside efficacy; see `--threshold-m-coverage`).
- `TIMED OUT` — the mutant caused an infinite loop that the runner
  killed. Counted as KILLED.
- `SKIPPED` — gremlins could not compile the mutant (rare; usually a
  type-check error from an edge-case mutation).

Efficacy (`KILLED / (KILLED + LIVED)`) is the number the floor
compares against. A LIVED mutant is a coverage gap worth
investigating — open the source line and ask "should a test have
caught this?". If yes, add the test. If no (dead code, trivial
getter, boilerplate), the floor should stay low enough to
accommodate it.

## Not in scope

- Mutation testing does not replace coverage thresholds. The existing
  `check_pkg` gates in `.github/workflows/ci.yml` still enforce line
  coverage per package. Mutation is the second layer.
- `internal/metrics`, `internal/clockify`, `internal/helpers`, and
  small utility packages are intentionally omitted. They either have
  property tests that exercise a narrow API surface or they have
  simple wrappers whose mutations are hard to kill without mocking
  Clockify itself — not a good fit for mutation testing.
