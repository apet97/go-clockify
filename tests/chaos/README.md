# clockify-mcp chaos harness (W2-08)

The chaos harness under `tests/chaos/` drives the stdlib-only
Clockify HTTP client (`internal/clockify/client.go`) under five
failure-injection scenarios and asserts the client's retry / backoff
/ error-classification behaviour is correct.

## Running

```bash
# Run every scenario:
go run ./tests/chaos -scenario all

# List scenarios:
go run ./tests/chaos -list

# Run a specific scenario:
go run ./tests/chaos -scenario 429-storm
go run ./tests/chaos -scenario 503-burst
go run ./tests/chaos -scenario mid-body-reset
go run ./tests/chaos -scenario tls-handshake-fail
go run ./tests/chaos -scenario dns-fail
```

`all` exits non-zero if any scenario fails. Individual scenarios
exit non-zero on failure too.

## Scenarios

| Scenario | Failure mode | Expected client behaviour |
|---|---|---|
| `429-storm` | httptest returns `429 Retry-After: 1` for the first N requests, then `200`. | Client honours `Retry-After`, waits, retries, eventually succeeds. Total attempts ≥ N+1. |
| `503-burst` | httptest returns `503` without `Retry-After` for the first N requests, then `200`. | Client applies jittered exponential backoff, retries, eventually succeeds. |
| `mid-body-reset` | httptest hijacks the connection, writes partial headers + body, then closes. | Client surfaces a clean `unexpected EOF` error without panicking or hanging. Reader cleanup path is exercised. |
| `tls-handshake-fail` | httptest TLS server with a self-signed certificate that will not validate against the system CA pool. | Client surfaces a `tls: failed to verify certificate` error; no retry loop. |
| `dns-fail` | Client points at a `.invalid` TLD hostname that cannot resolve. | Client surfaces a `no such host` error within the context deadline. DNS failures are local; infinite retries would be pointless. |

## Acceptance

`go run ./tests/chaos -scenario all` must print `all 5 scenarios
passed` and exit zero. A `FAIL` outcome on any scenario indicates the
HTTP client regressed one of its invariants and should be
investigated before shipping a release.

Recorded first-run output (2026-04-11 session):

```
[PASS] 429-storm               2× 429 Retry-After: 1 then 200 — client must retry and succeed
         elapsed=2.00s 3 attempts, final success

[PASS] 503-burst               2× 503 without Retry-After then 200 — jittered backoff recovery
         elapsed=0.94s 3 attempts, final success

[PASS] dns-fail                .invalid TLD hostname — client must fail fast, no retry loops
         elapsed=0.32s error: lookup ...invalid: no such host

[PASS] mid-body-reset          server writes partial body then closes — client must error cleanly
         elapsed=0.70ms error: unexpected EOF

[PASS] tls-handshake-fail      self-signed TLS server — handshake must fail cleanly
         elapsed=38ms error: x509: certificate signed by unknown authority

all 5 scenarios passed
```

## CI integration

`.github/workflows/chaos.yml` runs the harness on `workflow_dispatch`
only. Never on the PR critical path — chaos scenarios are inherently
timing-sensitive and would flake on shared runners. When an operator
suspects a regression they trigger the workflow manually.

## Adding a scenario

Scenarios are Go functions with the `scenarioFunc` signature
(`func() scenarioResult`). Register them in the `scenarios` map in
`main.go`. Each scenario should:

1. Set up an `httptest.NewServer` or direct configuration that
   reproduces the failure mode.
2. Construct a `clockify.NewClient` pointed at the faulty upstream.
3. Issue a GET against a known endpoint.
4. Assert against the expected error type, attempt count, or
   elapsed-time bound.
5. Return a `scenarioResult{name, pass, description, notes, elapsed}`.

The harness does not use testify or any third-party assertion
library — it returns plain Go values and the main loop collates
them. Matches the stdlib-only default build constraint from ADR 001.
