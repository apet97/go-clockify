# Webhook URL DNS Validation

## Why this runbook exists

`CreateWebhook` and `UpdateWebhook` reject webhook URLs whose host
resolves to a private, reserved, link-local, or loopback IP address
when DNS-aware validation is enabled. This protects a hosted control
plane from being turned into an SSRF probe via the Clockify outbound
webhook delivery.

This runbook covers the operator-visible behaviour, when the gate
applies, and how to debug a "valid hostname rejected" complaint.

## 1. Symptom

A tenant reports:

> I created a webhook pointing at `https://internal.example.com/hook`
> and got `webhook host "internal.example.com" resolves to
> private/reserved address 10.0.0.1`.

…but they intended to register that hostname.

## 2. When the gate fires

| Profile | Default | Override |
|---|---|---|
| `local-stdio` | literal-IP-only | `CLOCKIFY_WEBHOOK_VALIDATE_DNS=1` to opt in |
| `single-tenant-http` | literal-IP-only | same |
| `private-network-grpc` | literal-IP-only | same |
| `shared-service` | DNS-aware | `CLOCKIFY_WEBHOOK_VALIDATE_DNS=0` to opt out |
| `prod-postgres` | DNS-aware | same |

The literal-IP path always rejects URLs containing IPv4 / IPv6
literals in the private/reserved space. DNS resolution is the
additional layer that catches hostnames that resolve to one.

## 3. Common rejection patterns

| Pattern | Resolves to | Why it's blocked |
|---|---|---|
| `internal.example.com` | `10.0.0.1` | Private RFC 1918 |
| `corp.example.com` | `172.16.0.1` | Private RFC 1918 |
| `metadata.google.internal` | `169.254.169.254` | Link-local cloud metadata (SSRF target) |
| `localhost-mirror.example.com` | `127.0.0.1` | Loopback |
| `private.example.com` | `192.168.1.1` | Private RFC 1918 |

`isPublicWebhookAddr` in `internal/tools/tier2_webhooks.go` is the
classifier; it mirrors the literal-IP check exactly so both paths
speak the same language.

## 4. Operator response

### 4a. The reject is correct

This is the expected outcome. Tell the tenant:

> Your hostname resolves to a private IP. We require webhook
> targets to live on public infrastructure so the Clockify outbound
> delivery can never be used as an internal-network probe. Use a
> publicly-resolvable hostname or a tunnel (e.g. ngrok, Cloudflare
> Tunnel) for development.

### 4b. The hostname legitimately points outside the org

Audit the DNS reply. Use `dig +short <host>` or `nslookup` from the
control-plane network. If the result is genuinely public, the gate
should have allowed it; check whether the resolver is returning a
private split-horizon answer specific to the control-plane network.

If split-horizon DNS is the cause, the right fix is to either:

1. Run `clockify-mcp` with a resolver that sees the public answer
   (e.g. `1.1.1.1` / `8.8.8.8`), or
2. Set `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]` to
   admit the known-trusted hostname. Each entry matches either
   exactly (`webhook.example.com`) or as a leading-dot suffix
   that anchors a full DNS label (`.example.com` matches
   `webhook.example.com` and `api.eu.example.com` but NOT
   `attacker.example.com.evil.com`). Whitespace around each
   entry is trimmed and empty entries are dropped, so a
   leading or trailing comma is harmless. Empty list = no
   bypass; the DNS check applies to every host.

### 4c. The reject is wrong

Reproduce locally with a stubbed resolver:

```go
svc := &tools.Service{
    WebhookValidateDNS: true,
    WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
        // return what dig returns for the user's hostname
        return []netip.Addr{netip.MustParseAddr("203.0.113.7")}, nil
    },
}
err := svc.validateWebhookURLForService(context.Background(),
    "https://"+userHost+"/hook")
```

If the result with a known public IP also rejects, something is wrong
with `isPublicWebhookAddr` — open an issue with the failing
`netip.Addr.String()` value.

## 5. Residual risk: DNS rebinding

There is an inherent TOCTOU window between this validation and the
upstream Clockify→host delivery. An attacker who controls the
authoritative DNS for a hostname can answer "public IP" to the
control-plane resolver and "private IP" to the Clockify resolver. The
gate does not close that window.

Mitigations the gate *does* cover:

- Operator/tenant supplying a hostname known to point at a private
  target (e.g. `metadata.google.internal`).
- A `CNAME` chain whose final answer is in private space.

For a fuller defence, run Clockify-side webhook delivery through a
proxy that re-resolves and rejects on the wire. Track that as a
separate hardening item.

## 6. Verification checklist

- [ ] Hosted profile rejects a hostname resolving to `127.0.0.1`,
      `10.0.0.1`, `169.254.169.254`, `192.168.1.1`.
- [ ] Hosted profile accepts a hostname resolving to a public IP.
- [ ] Local profile (no override) still accepts both — literal-IP
      check unchanged.
- [ ] `isPublicWebhookAddr` test cases in
      `internal/tools/tier2_admin_test.go` cover every blocked range.

## 7. Related

- `internal/tools/tier2_webhooks.go` — `validateWebhookURL` (literal
  path) and `validateWebhookURLForService` (DNS-aware wrapper).
- `internal/tools/common.go` — `Service.WebhookValidateDNS`,
  `Service.WebhookHostResolver` (test injection point).
- `internal/config/config.go` — profile-driven default + explicit
  override via `CLOCKIFY_WEBHOOK_VALIDATE_DNS`.
- `docs/deploy/production-profile-shared-service.md` — strict-gate
  rationale table.
- Audit finding 10 (closed in the 2026-04-27 hardening wave).
