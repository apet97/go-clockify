#!/usr/bin/env bash
# Renders and validates Kustomize base + overlays and the Helm chart
# (default and monitoring-enabled variants) with kubeconform.
#
# Requires: kubectl, kubeconform, helm.
# Called from CI deploy-render and from `make verify-k8s`.

set -euo pipefail

need() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 not found on PATH" >&2
        exit 1
    fi
}

need kubectl
need kubeconform
need helm

SKIP="ServiceMonitor,PrometheusRule"

echo "== Kustomize base =="
kubectl kustomize deploy/k8s/base | kubeconform -strict -summary -skip "$SKIP"

for env in dev staging prod; do
    echo "== Kustomize overlay: $env =="
    kubectl kustomize "deploy/k8s/overlays/$env" | kubeconform -strict -summary -skip "$SKIP"
done

echo "== Overlay structure =="
bash scripts/check-overlay-structure.sh

echo "== Config parity =="
bash scripts/check-config-parity.sh

echo "== Helm lint =="
helm lint deploy/helm/clockify-mcp

echo "== Helm template (defaults) =="
helm template clockify-mcp deploy/helm/clockify-mcp | kubeconform -strict -summary -skip "$SKIP"

echo "== Helm template (monitoring enabled) =="
helm template clockify-mcp deploy/helm/clockify-mcp \
    --set metrics.serviceMonitor.enabled=true \
    --set metrics.prometheusRule.enabled=true \
    | kubeconform -strict -summary -skip "$SKIP"
