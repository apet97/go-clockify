# Credential Leak Response

## Why this runbook exists

Clockify MCP deployments handle bearer tokens, tenant API keys, OIDC
configuration, mTLS material, and control-plane credential references.
If any of those values leaves its intended trust boundary, the response
must rotate first, then audit, then notify. Do not wait for proof of
abuse before revoking a leaked credential.

This runbook covers operator response for:

- `CLOCKIFY_API_KEY` exposed in logs, shell history, screenshots,
  support tickets, crash dumps, or chat messages.
- Tenant credential refs or inline control-plane credentials exposed
  from Postgres, backups, admin tooling, or debug output.
- Static bearer tokens used by `MCP_AUTH_MODE=static_bearer`.
- OIDC client secrets, JWKS private keys, or mTLS private keys.
- Release, CI, or live-contract secrets exposed outside the intended
  secret store.

## 1. Severity

Treat a confirmed credential leak as a security incident.

| Leak type | Default severity |
|---|---|
| Tenant Clockify API key | High |
| Hosted shared-service credential ref or inline secret | High |
| Static bearer token for internet-facing transport | High |
| OIDC client secret or signing private key | High |
| mTLS private key | High |
| Sacrificial live-test API key | Medium unless it reaches a real workspace |

Escalate to Critical if the credential can reach a production
workspace, admin-level Clockify APIs, billing/invoice tools, or a
multi-tenant hosted deployment.

## 2. Immediate Containment

1. Remove the exposed value from the public location if you control it.
   Do not paste the secret into a ticket or chat while coordinating the
   removal.
2. Revoke or rotate the leaked credential at the source:
   - Clockify API key: rotate in Clockify, then update the deployment
     secret store or tenant credential ref.
   - Static bearer token: generate a new value, update the runtime
     secret, restart affected transports, and invalidate old clients.
   - OIDC client secret / signing key: rotate at the identity provider,
     publish the new JWKS if needed, and restart the server if it caches
     local key material.
   - mTLS private key: issue a new cert/key pair, revoke the old cert,
     and update trust bundles if the CA changed.
   - CI / release / live-test secret: rotate in GitHub Actions or the
     relevant secret manager before rerunning workflows.
3. If the secret appeared in git history, open a private security issue
   and coordinate history cleanup separately. Rotation is still the
   first fix.
4. For hosted profiles, temporarily disable the affected tenant or
   credential ref if rotation cannot complete immediately.

## 3. Audit

After rotation, collect evidence without printing secrets:

- Identify the first known exposure time and the last known valid use
  time of the leaked credential.
- Query audit events for successful write, delete, admin, billing, and
  permission-change tools during the exposure window.
- Query authentication logs for unexpected subjects, tenants, client
  IPs, user agents, session IDs, or transport modes.
- For streamable HTTP, inspect session records and delete sessions that
  predate rotation if the leaked material authenticated them.
- For Clockify API keys, compare recent Clockify workspace activity
  against expected MCP audit events.
- For release or CI secrets, inspect workflow run history and artifact
  uploads during the exposure window.

Do not copy raw API keys, bearer tokens, cert private keys, or credential
refs into the incident record. Refer to them by env var name, tenant ID,
credential ref ID, key fingerprint, or secret-store path.

## 4. Recovery Checks

- [ ] Old credential rejected by the source system.
- [ ] New credential deployed and verified with the narrowest possible
      smoke test.
- [ ] Affected MCP server processes restarted or reloaded.
- [ ] Active sessions minted before rotation expired or were deleted.
- [ ] `make secret-scan` or equivalent gitleaks scan run on the repo or
      artifact that leaked.
- [ ] Audit window reviewed for unexpected writes/admin/billing actions.
- [ ] Tenant or stakeholder notification decision recorded.

## 5. Notification

Notify affected tenants or stakeholders when:

- A tenant credential, tenant session, or tenant-owned Clockify API key
  was exposed outside the operator trust boundary.
- Audit logs show unexplained activity during the exposure window.
- The leak involved production CI/release secrets or signing material.
- You cannot prove the leaked value was inaccessible to third parties.

For sacrificial live-test keys, record the rotation and workspace scope
in the incident note; tenant notification is not needed unless the key
was accidentally pointed at a real workspace.

## 6. Follow-up

- Add or tighten secret scanning for the location where the leak
  happened.
- If inline credentials were involved, migrate the tenant to a vault
  backend and set `MCP_DISABLE_INLINE_SECRETS=1` where the profile
  supports it.
- If upstream errors exposed tenant data to an MCP client, enable
  `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` and review
  `docs/runbooks/hosted-error-sanitization.md`.
- If logs included credentials, rotate log storage access if needed and
  reduce log retention for the affected shard.
- Add a regression test or documented guard if a code path printed a
  secret.

## 7. Related

- `SECURITY.md` — reporting scope and hardening posture.
- `docs/runbooks/auth-failures.md` — auth-mode-specific failure triage.
- `docs/runbooks/audit-durability.md` — audit-store failure response.
- `docs/runbooks/hosted-error-sanitization.md` — upstream error body
  redaction.
- `docs/live-tests.md` — sacrificial live workspace key rotation.

## 8. Privacy / DPA Gate Boundary

This runbook is operational guidance for credential rotation and
audit. It does **not** close the `DPA / terms / privacy posture`
external approval gate in `docs/launch-readiness-review-may-8.md`.
That gate requires a counsel-signed data-flow review and DPA
disposition, with the artifact format documented in the gate's
evidence-artifact bullet. Following this runbook during an incident is
necessary, but it does not constitute a privacy / data-handling
review.
