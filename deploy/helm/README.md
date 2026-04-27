# clockify-mcp Helm chart

Helm chart for deploying [clockify-mcp](https://github.com/apet97/go-clockify)
to any vanilla Kubernetes cluster. Complementary to the raw manifests
under [`deploy/k8s/base/`](../k8s/base/) and the Kustomize overlays
under [`deploy/k8s/overlays/`](../k8s/overlays/) — pick whichever
distribution format matches your cluster's deploy tooling.

## Quickstart

```bash
# Minimal install with real credentials:
helm install clockify-mcp deploy/helm/clockify-mcp \
  --namespace clockify-mcp --create-namespace \
  --set secrets.clockifyApiKey="$CLOCKIFY_API_KEY" \
  --set secrets.mcpBearerToken="$(openssl rand -base64 24)"

# With kube-prometheus-stack ServiceMonitor + alert rules:
helm install clockify-mcp deploy/helm/clockify-mcp \
  --namespace clockify-mcp --create-namespace \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.prometheusRule.enabled=true \
  --set secrets.clockifyApiKey="$CLOCKIFY_API_KEY" \
  --set secrets.mcpBearerToken="$(openssl rand -base64 24)"

# Render templates offline without installing:
helm template clockify-mcp deploy/helm/clockify-mcp \
  --set secrets.clockifyApiKey=dummy \
  --set secrets.mcpBearerToken=dummy
```

## Values

The full knob surface lives in [`values.yaml`](clockify-mcp/values.yaml).
Every base Kustomize manifest field is reachable from a value here.
Highlights:

| Value | Default | Purpose |
|---|---|---|
| `image.repository` / `image.tag` | `ghcr.io/apet97/go-clockify` / `""` (falls back to `.Chart.AppVersion`) | Image coordinates. Pin to a digest in production. |
| `replicaCount` | `2` | Deployment replicas. `pdb.minAvailable: 1` stays at 1 regardless. |
| `config.CLOCKIFY_POLICY` | `time_tracking_safe` | `read_only`, `time_tracking_safe`, `safe_core`, `standard`, or `full`. Use `safe_core` only for trusted assistants that may create projects/clients/tags/tasks. |
| `config.CLOCKIFY_RATE_LIMIT` | `120` | Global rate-limit window (calls per 60s). |
| `transport.strictHostCheck` | `"1"` | Require Host header match. |
| `transport.logLevel` | `info` | `debug`, `info`, `warn`, `error`. |
| `networkPolicy.enabled` | `true` | Default-deny ingress/egress policy. Disable if your CNI does not support it. |
| `pdb.enabled` / `pdb.minAvailable` | `true` / `1` | PodDisruptionBudget for voluntary disruptions. |
| `metrics.serviceMonitor.enabled` | `false` | kube-prometheus-stack ServiceMonitor (requires the CRDs). |
| `metrics.prometheusRule.enabled` | `false` | Alert rules from `docs/observability.md` (requires the CRDs). |
| `secrets.create` | `true` | Chart creates the Secret. Set to `false` to use an external Secret (sealed-secrets, external-secrets). |
| `secrets.clockifyApiKey` | `""` | Must be set before the pod becomes ready. Pod readiness fails loudly otherwise. |
| `secrets.mcpBearerToken` | `""` | 16+ character random string. Generate with `openssl rand -base64 24`. |

## Equivalence with `deploy/k8s/base`

`helm template clockify-mcp deploy/helm/clockify-mcp --set secrets.clockifyApiKey=x
--set secrets.mcpBearerToken=x --set metrics.serviceMonitor.enabled=true
--set metrics.prometheusRule.enabled=true` produces the same nine-resource
stream (Deployment, Service, ConfigMap, Secret, NetworkPolicy, PDB,
ServiceAccount, ServiceMonitor, PrometheusRule) that `kubectl kustomize
deploy/k8s/base` produces, modulo `metadata.labels` which the Helm chart
augments with `helm.sh/chart`, `app.kubernetes.io/version`, and
`app.kubernetes.io/managed-by: Helm`.

## Verifying locally

```bash
helm lint deploy/helm/clockify-mcp
helm template clockify-mcp deploy/helm/clockify-mcp | kubeconform -strict -skip ServiceMonitor,PrometheusRule
```

Both commands are re-run on every push by the `deploy-render` job in
`.github/workflows/ci.yml` (landed in Track E).

## Versioning

The chart's `version` and `appVersion` in `Chart.yaml` track the MCP
server semver one-to-one. Bump both together when cutting a new MCP
release (`cmd/clockify-mcp/main.go` + `deploy/helm/clockify-mcp/Chart.yaml`).
