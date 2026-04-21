# Rate-limit saturation

## Why this runbook exists

`go-clockify` enforces two independent rate limits on every tool call:

- **Concurrency** — `MCP_MAX_INFLIGHT_TOOL_CALLS` (default 64) caps
  how many tool calls can be in flight at once. The 65th call waits up
  to `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT` (default 100ms) for a slot
  before failing fast.
- **Throughput** — `CLOCKIFY_RATE_LIMIT` (default 120/min) caps total
  tool calls per fixed 60-second window.

Either limit can saturate independently. The upstream Clockify API
also rate-limits, with its own quotas. Saturation usually means one
of those three knobs is wrong, or a client is misbehaving in a way
the knobs were not sized for.

## 1. Symptoms

- `clockify_mcp_rate_limit_rejections_total` rises sustained for >5
  minutes.
- `clockify_mcp_inflight_tool_calls` pinned near
  `MCP_MAX_INFLIGHT_TOOL_CALLS` for >5 minutes.
- `clockify_mcp_tool_call_duration_seconds` p99 climbs by >2x baseline.
- Upstream Clockify `429 Too Many Requests` responses show up in
  `msg=tool_call` errors and in
  `clockify_upstream_requests_total{status="4xx"}` /
  `clockify_upstream_retries_total{reason="rate_limited"}`.
- A single client appears to dominate recent traffic. For write-heavy
  workloads use audit / session metadata; for read-heavy abuse use
  ingress logs, because there is no per-subject Prometheus series.

## 2. Where to look first

```sh
# Inflight + recent rejections
curl -sf http://<host>:8080/metrics | grep -E '^clockify_mcp_(inflight|rate_limit_rejections)_'

# Per-tool call counts (which tools are hot?)
curl -sf http://<host>:8080/metrics | grep '^clockify_mcp_tool_calls_total'

# Upstream 4xx / retry pressure
curl -sf http://<host>:8080/metrics \
  | grep -E '^clockify_(upstream_requests_total|upstream_retries_total)'

# Recent tool-call failures
kubectl -n clockify-mcp logs deploy/clockify-mcp --since=15m \
  | grep 'msg=tool_call' \
  | grep -E '429 Too Many Requests|rate limit|context deadline exceeded'
```

## 3. Immediate mitigation

The mitigation depends on which limit is saturated.

### Local concurrency saturation (inflight pinned at the cap)

Increase the cap by env-var rollout:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  MCP_MAX_INFLIGHT_TOOL_CALLS=128
```

Or relax the acquire timeout so callers wait longer instead of failing
fast:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT=500ms
```

### Local throughput saturation (rejections rising, inflight low)

Raise the per-window budget:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  CLOCKIFY_RATE_LIMIT=240
```

### Upstream Clockify saturation (429s in logs)

Lower the local budget to back off and protect the upstream quota:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  CLOCKIFY_RATE_LIMIT=60
```

Then file an issue against the abusive client to get them onto a
batched workflow.

## 4. Root-cause checklist

Work this list top-to-bottom; the most common causes are first.

- [ ] **Misbehaving client.** There is no `auth_subject` label on
  `clockify_mcp_tool_calls_total`. For write-heavy traffic, query
  recent audit or control-plane session data by `subject`; for
  read-heavy traffic, use ingress / reverse-proxy logs to identify
  the caller. If one client dominates, fix their polling / backoff
  loop rather than masking it in the server.
- [ ] **Resolution cache miss storm.** A schema change or an ID
  rotation can invalidate the resolve-cache, causing every tool
  call to issue extra Clockify lookups. Symptoms: spike in
  `clockify_upstream_requests_total` per tool call, no
  corresponding spike in unique tool calls. Fix: deploy the
  schema-aware cache key.
- [ ] **Retry storm.** A network blip causes the client SDK to retry
  every failed call without jitter. Symptoms: bimodal latency
  histogram, repeated identical request bodies in the upstream
  client log. Fix: lower `CLOCKIFY_TOOL_TIMEOUT` so the upstream
  retry budget is smaller, or fix the client SDK.
- [ ] **Sized for the wrong workload.** If saturation persists at a
  steady state with no misbehaving client, the server is sized for
  a smaller team than it serves. Update `docs/performance.md`
  envelope and bump `CLOCKIFY_RATE_LIMIT` /
  `MCP_MAX_INFLIGHT_TOOL_CALLS` permanently in the Kustomize
  overlay.
- [ ] **Upstream Clockify quota change.** Clockify occasionally
  tightens per-workspace quotas. Symptoms: 429s appear with no
  matching local saturation. Fix: lower `CLOCKIFY_RATE_LIMIT` to
  match the new upstream ceiling and notify operators of the new
  envelope.

## 5. Postmortem template

After the incident, write a postmortem covering:

- **Trigger** — What was the proximate cause? (Misbehaving client,
  retry storm, schema change, upstream quota.)
- **Detection** — Did the rate-limit-saturation alert fire? How
  long between trigger and detection?
- **Mitigation** — Which env var(s) did you change? What was the
  before/after value? Why this knob and not another?
- **Permanent fix** — If the mitigation was a temporary env var
  bump, what permanent change is needed? (Higher overlay defaults,
  client-side fix, performance envelope update.)
- **Prevention** — What signal would have caught this earlier? File
  a follow-up to add the signal if it's missing.

## See also

- `internal/ratelimit/` — where the throughput + concurrency limits
  are enforced.
- `internal/metrics/metrics.go` — full list of exported metrics
  (search for `clockify_mcp_rate_limit`).
- `docs/performance.md` — published safe operating envelope.
- `SECURITY.md` — `clockify_mcp_panics_recovered_total` is the
  related fail-closed counter for unrelated saturation modes.
