# QA Agent 26 - rate-limit-retry-behavior

## Verdict
**PASS WITH CONCERNS**

## What I checked

- Rate limiter implementation (`internal/ratelimit/ratelimit.go`) — two-layer design (global + per-subject), fixed-window counting, concurrency semaphore, subject reaper, env-var configuration
- Retry logic (`internal/clockify/client.go`) — exponential backoff with crypto/rand jitter, Retry-After header parsing, context deadline awareness, bounded drain on error bodies
- Enforcement pipeline (`internal/enforcement/enforcement.go`) — integration of rate limiting with policy gates, dry-run intercept, schema validation in `BeforeCall`
- HTTP admission guard (`internal/mcp/http_admission.go`) — per-IP, per-principal, per-SSE-session limits with proper 429 + Retry-After responses
- Error messages (`internal/helpers/helpers.go` and `internal/clockify/errors.go`) — user-facing messages for 429 and other error codes, upstream body redaction
- Configuration (`internal/config/config.go`) — how `MaxRetries` and rate-limit env vars are wired
- Runbooks (`docs/runbooks/rate-limit.md` and `docs/runbooks/rate-limit-saturation.md`) — operational documentation quality
- Unit tests — all rate-limit (25 tests), retry (14 tests), enforcement (5 tests) pass
- Docker build — succeeds
- Doctor command — works, reports all config including rate-limit defaults
- Live API probes — confirmed 429 behavior from real Clockify API

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID, second-factor confirmation |
| `probes/lib/common.sh` | Curl wrapper with retry logic, redaction helpers |
| `README.md` | Lab overview and safety rules |
| `CLAUDE.md` | Agent rules and hard limits |

Credentials were sourced from `/tmp/clockify-livetest.env` and never written to any file.

## Commands run

```sh
# Build check
go build ./...

# Rate limit unit tests (25 tests, all pass)
go test ./internal/ratelimit/... -v -count=1 -timeout=60s

# Client retry unit tests (14 tests, all pass)
go test ./internal/clockify/... -v -run 'Retry|Backoff|Retryable' -count=1 -timeout=120s

# Enforcement rate limit tests (5 tests, all pass)
go test ./internal/enforcement/... -v -run 'Rate|Limit|Subject|Token' -count=1 -timeout=60s

# Full test suite (all pass)
go test ./internal/clockify/... ./internal/ratelimit/... ./internal/enforcement/... ./internal/mcp/... -count=1

# Doctor command
CLOCKIFY_API_KEY='****REDACTED****' CLOCKIFY_WORKSPACE_ID='****REDACTED****' \
  go run ./cmd/clockify-mcp doctor --strict --allow-broad-policy --check-backends

# Docker build
docker build -t clockify-mcp-qa-test -f deploy/Dockerfile .
```

## Live API probes run

### Probe 1: Rate limit header inspection
```
GET https://api.clockify.me/api/v1/user
Response: 200, headers include x-xss-protection, x-auth-checksum, x-cache, x-amz-cf-*
No X-RateLimit-* headers present
No Retry-After on normal responses
```

### Probe 2: Parallel request burst (30 requests)
```
Result: 25x 200, 5x 429 (CloudFront layer)
x-cache: Error from cloudfront on 429 responses
No Retry-After header in 429 responses from CloudFront
```

### Probe 3: Larger burst (80 requests)
```
Result: 36x 200, 44x 429
429 body: empty or CloudFront error page
No Retry-After header
```

### Probe 4: Create and cleanup test resource
```
Created client: qa-agent-26-1778447989-24e987-client (id=<REDACTED_ID>)
DELETE returned 400: "Cannot delete an active client" (code=501)
Leftover resource documented below
```

## Findings

### F1: MaxRetries is hardcoded to 3 with no env var override (P2)

**Location:** `internal/config/config.go:307`

```go
cfg.MaxRetries = 3
```

The `Clockify` client constructor accepts `maxRetries` as a parameter and the retry loop respects it, but there is no `CLOCKIFY_MAX_RETRIES` env var to let operators tune this. All other performance knobs (rate limits, concurrency, timeouts) have env var overrides. A `CLOCKIFY_MAX_RETRIES` env var would let operators reduce retries in saturation scenarios (runbook recommendation) or increase them for flaky-network environments without a code change.

Severity P2 because the runbook's "retry storm" and "upstream Clockify saturation" mitigations rely on lowering retry budgets, but operators can't do that without a rebuild.

### F2: Clockify upstream 429s lack Retry-After headers — backoff fallback is correct but base may be too aggressive (P3)

**Location:** `internal/clockify/client.go:418-420` and `backoff()` at line 632

The Clockify API (via CloudFront) returns 429s without a `Retry-After` header. The client correctly falls back to exponential backoff:
```
backoff(1) = 250ms + rand(0..124ms)   -> [250, 374ms]
backoff(2) = 500ms + jitter           -> [500, 624ms]
backoff(3) = 1000ms + jitter          -> [1000, 1124ms]
```

With `maxRetries=3`, the client makes up to 4 total attempts (1 initial + 3 retries) over ~2 seconds. This is reasonable but the 250ms base for attempt 1 may be too aggressive when CloudFront is doing sub-second rate limiting. A burst of retries at 250ms intervals could prolong CloudFront rate-limiting windows. Most rate-limit backoffs use a 1-second minimum gap.

Severity P3 — not a bug, but worth documenting that the aggressive first retry may not help with CloudFront-layer rate limiting.

### F3: No per-subject Prometheus metrics for rate-limit rejections (documented gap)

**Location:** `docs/runbooks/rate-limit-saturation.md` line 51

The runbook explicitly notes: "there is no per-subject Prometheus series." The `RateLimitRejections` metric in `internal/metrics/metrics.go` tracks `{kind,scope}` but not `{subject}`. This means when a single noisy tenant saturates rate limits, operators must correlate audit/ingress logs rather than getting an immediate Prometheus alert. The runbook documents this limitation clearly.

Severity P3 — documented limitation, not a code bug.

### F4: HTTP admission layer uses proper 429 + Retry-After (PASS)

**Location:** `internal/mcp/http_admission.go:176-183`

The HTTP admission guard correctly returns 429 with a computed `Retry-After` header (in seconds). This is the right contract for a pre-dispatch rate limit reject — the client knows exactly when to retry. Contrasts with the upstream CloudFront 429s which lack this header.

### F5: Rate limiter window reset has a theoretical race (defense in depth applied)

**Location:** `internal/ratelimit/ratelimit.go:183-199`

The window reset uses a mutex + double-check pattern which is correct. However, there's a theoretical window between the pre-check at line 196 (`windowCount.Load() >= maxPerWindow`) and the increment at line 215 (`windowCount.Add(1)`). Two goroutines could both pass the pre-check when `windowCount == maxPerWindow - 1`, then both increment past the cap. The post-increment check at line 216 catches this and releases the slot — so the worst case is a semaphore slot held momentarily then released, not an actual over-admission. The code handles this cleanly in practice.

Severity PASS — the post-increment guard is correct.

## Fixes made

No code fixes were applied. The findings are documentation/config gaps, not runtime bugs.

## Reproduction steps for each issue

### F1 (MaxRetries hardcoded):
1. Start the server with any transport
2. Observe that retry count is always 3 regardless of environment
3. No `CLOCKIFY_MAX_RETRIES` env var exists in the help output or config spec

### F2 (Aggressive first backoff):
1. Trigger 429 from Clockify API (parallel burst of 30+ requests)
2. MCP client retries after 250ms base backoff
3. CloudFront may still be in rate-limit window, causing subsequent 429s
4. All retries exhaust within ~2 seconds

## Cleanup performed

- Created test client `qa-agent-26-1778447989-24e987-client` (id=<REDACTED_ID>)
- Attempted DELETE — failed with 400 "Cannot delete an active client" (Clockify API restriction)
- Docker image `clockify-mcp-qa-test:latest` built in local Docker daemon (not pushed)

## Leftover test resources

| Resource | ID | Name | Note |
|----------|----|------|------|
| client | <REDACTED_ID> | qa-agent-26-1778447989-24e987-client | Cannot delete (active client); has no projects/tasks/time entries |

## Severity

| ID | Severity | Area | Summary |
|----|----------|------|----------|
| F1 | P2 | Config | MaxRetries=3 hardcoded, no env var override |
| F2 | P3 | Retry | Backoff base 250ms aggressive for CloudFront 429s without Retry-After |
| F3 | P3 | Observability | No per-subject label on rate-limit rejection metrics |
| F4 | PASS | HTTP admission | Proper 429 + Retry-After on pre-dispatch rejects |
| F5 | PASS | Rate limiter | Window reset race guarded by post-increment check |

## Files changed

None. No code fixes were applied.

## Suggested next action

1. Add `CLOCKIFY_MAX_RETRIES` env var to `internal/config/spec.go` and wire it in `internal/config/config.go` ~line 307 (P2)
2. Add `CLOCKIFY_MAX_RETRIES` to the help text via `cmd/gen-config-docs` (P2)
3. Consider bumping the backoff base from 250ms to 1000ms for the first retry when `reason="rate_limited"` and no `Retry-After` header is present (P3, optional)
4. The existing runbook documentation is thorough and covers all major scenarios — no doc changes needed

## False positives / uncertainty

- The CloudFront-originated 429s may behave differently at higher traffic volumes or with different API endpoints. The probe used only `GET /user`.
- The rate limiter window-reset race was assessed theoretically; a dedicated concurrent stress test at exact window boundaries could surface the post-increment recovery path but the current behavior is correct.
- The client-deletion failure (400 "active client") is a Clockify API behavior, not an MCP server issue.

## Final recommendation

**Ship with the P2 fix** (CLOCKIFY_MAX_RETRIES env var). The core rate-limit and retry mechanisms are solid, well-tested, and correctly handle the real Clockify API's rate-limiting behavior (including CloudFront-layer 429s without Retry-After headers). The P3 items are documentation/observation improvements that don't block launch.
