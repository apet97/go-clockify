# Deploy-Readiness Checklist

This checklist must be satisfied before promoting a release to production.

## 1. Artifact Verification
- [ ] **Signature Verification:** Verify the binary or container image using `cosign` and `gh attestation`.
- [ ] **Digest Pinning:** Use the immutable image digest (`sha256:...`) instead of a mutable tag (`:latest`, `:v1`).
- [ ] **SBOM Check:** Review the SPDX SBOM for any unexpected dependencies.

## 2. Environment Configuration
- [ ] **Config Parity:** Ensure the production `.env` or Kubernetes ConfigMap matches the `env.shared-service.example` preset.
- [ ] **Policy Check:** Verify public AI-facing deployments use `CLOCKIFY_POLICY=time_tracking_safe` or stricter; document any trusted-assistant exception that uses `safe_core` or broader.
- [ ] **Metrics Isolation:** Confirm `MCP_METRICS_BIND` is listening on a separate, non-exposed port.

## 3. Pre-Flight Tests
- [ ] **Staging Smoke Tests:** Run `make http-smoke` (or `make stdio-smoke`) against a staging instance.
- [ ] **Live-Contract Status:** Check the latest nightly live-contract test run in GitHub Actions.
- [ ] **Doctor — Config:** Run `clockify-mcp doctor --profile=<your-profile> --strict` against the production environment; expect exit 0.
- [ ] **Doctor — Backends (Postgres deployments):** Run `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends` against a production clone; expect exit 0. The default `clockify-mcp` binary deliberately fails `--check-backends` because ADR-0001 keeps `pgx` out of the default `go.mod` — only the Postgres-tagged binary satisfies this gate. See [`public-hosted-launch-checklist.md`](public-hosted-launch-checklist.md) for the full hosted pre-flight.

## 4. Rollback and Recovery
- [ ] **Rollback Artifact:** Confirm the previous working image digest is documented and accessible.
- [ ] **Runbook Awareness:** Review the outage-drill runbook and postgres-restore-drill.

## 5. Security and Compliance
- [ ] **Secrets Management:** Verify that no API keys or OIDC secrets are logged or exposed.
- [ ] **Vulnerability Scan:** Ensure the Trivy scan for the release image has zero HIGH or CRITICAL findings.

## 6. Support and Monitoring
- [ ] **Dashboard Check:** Confirm metrics are flowing to the production Prometheus/Grafana instance.
- [ ] **Alerting:** Ensure the burn-rate alerts for the 99.9% SLO are active.

---

**Signature:** ____________________  **Date:** ____________________
