#!/usr/bin/env bash
set -euo pipefail

# Extract every os.Getenv("...") call from config.go.
CONFIG_FILE="internal/config/config.go"
HELM_DIR="deploy/helm/clockify-mcp"
K8S_DIR="deploy/k8s/base"
OPT_OUT="deploy/.config-parity-opt-out.txt"

if [ ! -f "$CONFIG_FILE" ]; then
  echo "ERROR: $CONFIG_FILE not found" >&2
  exit 1
fi

env_vars=$(grep -o 'os\.Getenv("[^"]*")' "$CONFIG_FILE" | sed 's/os\.Getenv("//;s/")//' | sort -u)

helm_surface=""
for f in "$HELM_DIR"/values.yaml "$HELM_DIR"/templates/*.yaml; do
  [ -f "$f" ] && helm_surface="$helm_surface $(cat "$f")"
done

k8s_surface=""
for f in "$K8S_DIR"/configmap.yaml "$K8S_DIR"/secret.example.yaml "$K8S_DIR"/deployment.yaml; do
  [ -f "$f" ] && k8s_surface="$k8s_surface $(cat "$f")"
done

opt_out_list=""
if [ -f "$OPT_OUT" ]; then
  opt_out_list=$(grep -v '^#' "$OPT_OUT" | grep -v '^$' || true)
fi

missing=0
# Use here-strings instead of `echo | grep -q`. Under `set -o pipefail`
# grep -q exits as soon as it finds a match, and the SIGPIPE sent back
# to echo makes the pipeline exit non-zero — which then flipped the
# `&& in_helm=true` assignment into a no-op. Symptom was a broken-pipe
# write error followed by a spurious "MISSING in Helm" for a variable
# that was actually present. Locally fast enough to hide; CI runners
# hit it every time.
for var in $env_vars; do
  in_helm=false
  in_k8s=false
  opted_out=false

  if grep -q "$var" <<< "$helm_surface"; then in_helm=true; fi
  if grep -q "$var" <<< "$k8s_surface"; then in_k8s=true; fi
  if grep -qE "^${var}([[:space:]]|$)" <<< "$opt_out_list"; then opted_out=true; fi

  if ! $in_helm && ! $opted_out; then
    echo "MISSING in Helm: $var" >&2
    missing=$((missing + 1))
  fi
  if ! $in_k8s && ! $opted_out; then
    echo "MISSING in Kustomize: $var" >&2
    missing=$((missing + 1))
  fi
done

if [ "$missing" -gt 0 ]; then
  echo "" >&2
  echo "ERROR: $missing env var(s) from $CONFIG_FILE are not reachable" >&2
  echo "through the Helm chart or Kustomize base. Either:" >&2
  echo "  1. Add them to the chart/kustomize templates, or" >&2
  echo "  2. Add them to $OPT_OUT (one var per line)" >&2
  exit 1
fi

echo "OK: all env vars from $CONFIG_FILE are covered (or opted out)"
