# Runbook: Authentication failures

Use this runbook when clients report `401 Unauthorized` errors on
`/mcp`, when the `ClockifyMCPAuthFailures` alert fires, or when you
suspect the bearer token has been compromised or rotated without
distribution.

## Symptoms

- Spike in `clockify_mcp_http_requests_total{path="/mcp",status="401"}`
- Clients see `Unauthorized` responses from `/mcp`
- Log events: `http_auth_failure` with a `reason` field

PromQL for the 401 rate over the last 5 minutes:

```promql
sum(rate(clockify_mcp_http_requests_total{path="/mcp",status="401"}[5m]))
```

Nominal value is near zero. Any sustained non-zero rate warrants
investigation.

## Immediate mitigation

### If a token rotation was just performed (expected auth storm)

1. Confirm the new token reached all authorized clients out-of-band.
2. Let the 401 rate drop naturally as clients update.
3. If the rate does not drop within 10 minutes, escalate — some
   clients may be hardcoded to the old token.

### If no rotation was performed (unexpected failures)

Rotate the bearer token immediately:

```bash
NEW_TOKEN=$(openssl rand -base64 24)

# Generate the new Secret manifest, preserving the existing CLOCKIFY_API_KEY:
EXISTING_KEY=$(kubectl -n clockify-mcp get secret clockify-mcp-secrets \
  -o jsonpath='{.data.CLOCKIFY_API_KEY}' | base64 -d)

kubectl -n clockify-mcp create secret generic clockify-mcp-secrets \
  --from-literal=CLOCKIFY_API_KEY="$EXISTING_KEY" \
  --from-literal=MCP_BEARER_TOKEN="$NEW_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n clockify-mcp rollout restart deployment/clockify-mcp
kubectl -n clockify-mcp rollout status deployment/clockify-mcp --timeout=2m
```

Distribute `$NEW_TOKEN` to authorized clients via your secret
distribution channel (password manager, vault, secure chat).

### If the old token was leaked

In addition to rotating the bearer token:

1. Audit recent `/mcp` access logs for unauthorized requests:
   ```bash
   kubectl -n clockify-mcp logs -l app.kubernetes.io/name=clockify-mcp \
     --since=24h | grep -E "http_request|http_auth"
   ```
2. Check the Clockify audit trail for any actions taken through this
   MCP server in the leak window. Review any writes (time entries
   created/deleted, projects modified).
3. Consider rotating the upstream `CLOCKIFY_API_KEY` as well since a
   leaked bearer token could have proxied Clockify API calls.
4. File an internal incident report per your org's security policy.

## Root cause investigation

Once the immediate fire is out, determine how the auth state drifted:

### Recent Secret changes

```bash
kubectl -n clockify-mcp get events --sort-by=.lastTimestamp | tail -20
kubectl -n clockify-mcp describe secret clockify-mcp-secrets
```

Look for unexpected Secret modifications or deletes.

### Git history audit

Confirm the bearer token is not committed anywhere:

```bash
git log --all -p -- deploy/k8s/secret.example.yaml | grep -v REPLACE_ME
```

The example file must contain only placeholder values. If a real
token ever landed in a commit, rotate immediately AND rewrite history
using `git filter-repo` to purge the leaked value from the remote.

### Access review

Who has `kubectl` access to the `clockify-mcp` namespace?

```bash
kubectl auth can-i --list --namespace=clockify-mcp
```

Review RBAC bindings and consider tightening if more principals have
Secret read access than necessary.

### Token strength

The server enforces `len(MCP_BEARER_TOKEN) >= 16` at startup. Longer
tokens are better. The recommended generator is
`openssl rand -base64 24` which produces a 32-character token.

## Recovery

- Confirm the 401 rate returns to its baseline (typically zero).
- Confirm the deployment is ready: `kubectl -n clockify-mcp get pods`
- Confirm legitimate clients can reach `/mcp` with the new token:
  ```bash
  curl -s -H "Authorization: Bearer $NEW_TOKEN" \
    -X POST \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    https://clockify-mcp.your-domain/mcp
  ```
- Watch `clockify_mcp_http_requests_total{status="200"}` recover.

## Related

- [deploy/k8s/README.md](../../deploy/k8s/README.md) — secret rotation guidance
- [SECURITY.md](../../SECURITY.md) — reporting a vulnerability
- [docs/observability.md](../observability.md) — metrics and alert rules
