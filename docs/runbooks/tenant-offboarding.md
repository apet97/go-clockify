# Tenant Offboarding

Use this runbook when a hosted or shared-service tenant must lose
access to `clockify-mcp`: contract end, suspected compromise, failed
payment, workspace ownership transfer, or emergency suspension.

## Guarantees And Limits

The hosted profiles clamp `MCP_OIDC_VERIFY_CACHE_TTL` to at most
`60s`, so a token that was accepted only because of the MCP verify
cache stops using that cached result within one minute.

That is not the same as revoking an otherwise valid JWT. This server
verifies JWTs locally against the issuer's JWKS; it does not call an
OAuth introspection or RFC 7009 revocation endpoint on every request.
If the IdP keeps issuing or accepting a JWT until its `exp`, the MCP
server will still accept it after a cache miss. For immediate
offboarding, the IdP, forward-auth proxy, or mTLS client-certificate
layer must block the tenant before traffic reaches MCP.

## Immediate Suspension

1. Block the tenant at the auth source.
   - OIDC: remove the tenant/user from the application assignment or
     deny the tenant claim at the IdP or gateway.
   - `forward_auth`: block the tenant at the trusted proxy before it
     forwards `MCP_FORWARD_TENANT_HEADER`.
   - mTLS: revoke or remove the tenant client certificate at the
     fronting proxy or CA.
2. Revoke or rotate the tenant's Clockify API key or vault entry.
   `CLOCKIFY_API_KEY` is only the single-tenant env path; hosted
   tenants normally resolve credentials through the control-plane
   `credential_refs` table and the selected vault backend.
3. Stop active MCP sessions for that tenant. The safest generic action
   is a rolling restart of the hosted deployment, which clears the
   in-memory OIDC verify cache and streamable HTTP sessions. If the
   control-plane store exposes tenant-scoped session deletion in the
   operator tooling, delete only that tenant's sessions instead.
4. Wait at least `60s` after the auth-source block or restart before
   treating the MCP verify-cache path as drained.

## Verification

Use a tenant-scoped test principal that was valid before suspension:

```sh
curl -sf -X POST https://<host>/mcp \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <old-token>' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

Expected result after the auth-source block has propagated:

- HTTP 401 or 403 for HTTP transports.
- No new `sessions` rows for that tenant in the control-plane store.
- No successful mutating audit `outcome` rows after the suspension
  timestamp.

If the old JWT still succeeds after the MCP verify cache should have
expired, inspect the IdP token lifetime and gateway policy. The MCP
server cannot invalidate a still-valid offline JWT without an upstream
deny decision or a key/claim change that causes local verification to
fail.

## Recovery Or Reinstatement

1. Restore the tenant's IdP/proxy/certificate access.
2. Restore or rotate the Clockify credential reference.
3. Confirm `clockify_mcp_http_requests_total{status="401"}` and
   `clockify_mcp_http_requests_total{status="403"}` return to baseline.
4. Open a fresh MCP session and run a read-only tool such as
   `clockify_list_workspaces`.

## See Also

- [`auth-failures.md`](auth-failures.md) - auth-mode-specific triage.
- [`credential-leak-response.md`](credential-leak-response.md) -
  rotating tenant credentials after suspected exposure.
- [`audit-durability.md`](audit-durability.md) - checking whether
  mutations happened during the offboarding window.
