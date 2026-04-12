# Kubernetes reference manifests

Stock YAML manifests for running `clockify-mcp` in HTTP mode on any vanilla
Kubernetes cluster. As of W2-06 (v0.7.0) the manifests are organised as a
Kustomize base + three environment overlays; operators who prefer raw
`kubectl apply` can still `kubectl apply -k deploy/k8s/base` to get the
pre-W2-06 flat stream back.

## Layout

```
deploy/k8s/
├── base/                # shared manifests
│   ├── kustomization.yaml
│   ├── configmap.yaml
│   ├── deployment.yaml
│   ├── networkpolicy.yaml
│   ├── pdb.yaml
│   ├── prometheus-rule.yaml
│   ├── secret.example.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   └── servicemonitor.yaml
└── overlays/
    ├── dev/             # smaller requests, debug logs, read_only policy
    ├── staging/         # 3 replicas, safe_core policy, environment label
    └── prod/            # 4 replicas, pinned image tag, strict host check
```

| Base manifest | Role |
|---|---|
| `deployment.yaml` | Deployment with pod security context, resource limits, and three probes (startup, liveness, readiness). |
| `service.yaml` | ClusterIP service exposing port 8080 (`http`) for in-cluster clients and ingress controllers. |
| `configmap.yaml` | Non-secret environment variables (policy, rate limits, timeouts, bootstrap mode). |
| `secret.example.yaml` | Template for the Secret holding `CLOCKIFY_API_KEY` and `MCP_BEARER_TOKEN`. Do not commit real values. |
| `networkpolicy.yaml` | Default-deny ingress/egress policies. |
| `pdb.yaml` | PodDisruptionBudget preserving at least one healthy replica during voluntary disruptions. |
| `serviceaccount.yaml` | Dedicated ServiceAccount used by the Deployment. |
| `servicemonitor.yaml` | kube-prometheus-stack `ServiceMonitor` scraping `/metrics` on the `http` port every 30s. |
| `prometheus-rule.yaml` | `PrometheusRule` mirroring `docs/observability.md` alerts: multi-window burn-rate (99.9% SLO), `ClockifyMCPUpstreamUnavailable`, `ClockifyMCPHighToolErrorRate`, `ClockifyMCPRateLimitSaturation`, `ClockifyMCPHighLatency`, `ClockifyMCPNotReady`, `ClockifyMCPAuthFailures`. |

The server runs in HTTP transport mode in-cluster. Stdio is reserved for
local MCP clients (Claude Desktop, Cursor).

## Overlay summary

| Overlay | Replicas | Policy | Log level | Notes |
|---|---|---|---|---|
| `dev` | 1 | `read_only` | `debug` | Smaller CPU/memory requests so pods schedule on laptops. Safe default for iteration — the read-only policy makes it impossible to accidentally mutate Clockify state via a misconfigured client. |
| `staging` | 3 | `safe_core` | `info` | Production-ish with canary-friendly replica count. `safe_core` policy blocks the most destructive tool classes while leaving the core read + write surface available. |
| `prod` | 4 | `standard` | `info` | Pinned image tag (`0.7.0`), `MCP_STRICT_HOST_CHECK=1`, full resource requests. Replace `newTag` in `overlays/prod/kustomization.yaml` with a pinned digest before applying. |

## Quickstart

```bash
kubectl create namespace clockify-mcp

# Create the Secret (DO NOT commit real values).
kubectl -n clockify-mcp create secret generic clockify-mcp-secrets \
  --from-literal=CLOCKIFY_API_KEY="$CLOCKIFY_API_KEY" \
  --from-literal=MCP_BEARER_TOKEN="$(openssl rand -base64 24)"

# Apply an overlay (choose dev/staging/prod based on environment):
kubectl apply -k deploy/k8s/overlays/prod

# Or apply the base directly if you prefer the pre-W2-06 flat stream:
kubectl apply -k deploy/k8s/base

kubectl -n clockify-mcp rollout status deployment/clockify-mcp
```

Verify the pod is healthy and authenticated:

```bash
kubectl -n clockify-mcp port-forward svc/clockify-mcp 8080:8080 &
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/ready
curl -fsS -H "Authorization: Bearer $MCP_BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}' \
  http://127.0.0.1:8080/mcp
```

## Image

The manifest references `ghcr.io/apet97/go-clockify:latest`. The project's
Dockerfile lives in [`deploy/Dockerfile`](../Dockerfile). In production:

- Pin to a released tag (for example `ghcr.io/apet97/go-clockify:v0.5.0`)
  instead of `:latest` so rollouts are deterministic.
- Consider building your own image from a verified release binary and
  publishing it to your internal registry.
- Mirror the image to keep supply-chain provenance under your control.

## Security posture

The pod spec ships with:

- `runAsNonRoot: true`, `runAsUser: 65532`, `runAsGroup: 65532` — runs as
  the unprivileged `nonroot` user shipped with distroless base images.
- `readOnlyRootFilesystem: true` — the container cannot write to its own
  filesystem. The server writes nothing on disk; all state is in memory.
- `allowPrivilegeEscalation: false` — blocks `setuid`/`setgid` escalation.
- `capabilities.drop: ["ALL"]` — no Linux capabilities are granted.
- `seccompProfile.type: RuntimeDefault` — uses the runtime's default
  seccomp filter, blocking unexpected syscalls.
- `automountServiceAccountToken: false` — the container has no in-cluster
  API credential. The MCP server never talks to the Kubernetes API.

Secrets are injected via `envFrom: secretRef`, not baked into the image
or the Deployment manifest.

## Observability

Legacy `MCP_TRANSPORT=http` exposes three unauthenticated endpoints on the main listener:

- `GET /health` — liveness (always 200 if the process is alive).
- `GET /ready` — readiness (503 if the upstream Clockify API is unreachable).
- `GET /metrics` — Prometheus text-format metrics.

For `MCP_TRANSPORT=streamable_http`, prefer a dedicated metrics listener via
`MCP_METRICS_BIND` and scrape that port instead of the public MCP listener.

Structured JSON logs are written to stderr (`MCP_LOG_FORMAT=json`).

If your cluster uses annotation-based Prometheus discovery, add to the
pod template:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

For a `ServiceMonitor`-based scrape (kube-prometheus-stack), create a
`ServiceMonitor` that selects on `app.kubernetes.io/name: clockify-mcp`.

## Scaling

The provided Deployment runs 2 replicas for availability during rolling
updates. Horizontal scaling notes:

- The server is stateless apart from in-memory caches (current user and
  workspace). Scaling replicas is safe.
- Every replica authenticates with the same `CLOCKIFY_API_KEY`. Clockify
  rate limits are applied per-key at the upstream, so adding replicas
  does **not** raise the aggregate rate-limit ceiling — it only raises
  local concurrency headroom. If you regularly see upstream 429s, scale
  the policy (`CLOCKIFY_POLICY`) or request volume, not the replica count.
- An HPA on CPU (target around 70%) is a reasonable default once traffic
  patterns are known. Memory-based autoscaling is unnecessary — the
  process footprint is effectively constant.

## Secret rotation

Both `CLOCKIFY_API_KEY` and `MCP_BEARER_TOKEN` can be rotated without
downtime by updating the Secret and restarting the rollout:

```bash
# Generate a new bearer token and patch the Secret in one step.
NEW_TOKEN=$(openssl rand -base64 24)
kubectl -n clockify-mcp create secret generic clockify-mcp-secrets \
  --from-literal=CLOCKIFY_API_KEY="$CLOCKIFY_API_KEY" \
  --from-literal=MCP_BEARER_TOKEN="$NEW_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

# Roll pods so the new env values are loaded.
kubectl -n clockify-mcp rollout restart deployment/clockify-mcp
kubectl -n clockify-mcp rollout status deployment/clockify-mcp
```

Distribute the new bearer token to authorized clients out-of-band.
Because the Deployment uses `maxUnavailable: 0`, the service stays
available through the restart.

## Troubleshooting

Incident runbooks live under [`docs/runbooks/`](../../docs/runbooks/):

- `rate-limit-saturation.md` — upstream or local rate-limit spikes.
- `clockify-upstream-outage.md` — Clockify API 5xx or timeout spikes.
- `auth-failures.md` — 401 spikes and bearer token rotation.
