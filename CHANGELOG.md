# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-04-12

Initial stable release.

> **Stability commitment.** The current API surface — tool names, resource URI templates, configuration env vars, delta-sync wire format, and JSON-RPC protocol behaviour — is now the v1 baseline. No breaking changes will be made without a major version bump. Tier 2 tool groups, the RFC 6902 JSON Patch delta format, and the gRPC transport are considered stable at this release.

### Highlights

- **124 tools** — 33 Tier 1 registered at startup, 91 Tier 2 activated on demand across 11 domain groups.
- **MCP capabilities**: `tools`, `resources` (2 concrete + 6 parametric URI templates), and `prompts` (5 built-in templates).
- **Transports**: stdio (default), streamable HTTP 2025-03-26, legacy POST-only HTTP, and opt-in gRPC (`-tags=grpc`).
- **Auth modes**: `static_bearer`, `oidc`, `forward_auth`, `mtls` — routed via a shared `authn.Authenticator` interface with per-stream validation on gRPC.
- **Four policy modes** (`read_only`, `safe_core`, `standard`, `full`) plus three-strategy dry-run for every destructive tool.
- **Three-layer rate limiting**: stdio dispatch semaphore, per-process concurrency + window limiter, and per-`Principal.Subject` sub-layer.
- **Stdlib-only default build** — the default binary links no OpenTelemetry, gRPC, or protobuf symbols. Verified in CI via `go tool nm`.
- **Opt-in observability**: OpenTelemetry tracing behind `-tags=otel`, Prometheus metrics always on, PII-scrubbed structured logs.
- **Signed releases** — cosign keyless signatures, SPDX SBOMs, and SLSA build provenance on every binary and container image.
- **Reference Kubernetes manifests** — Deployment (non-root distroless, read-only root FS), NetworkPolicy (default-deny), PodDisruptionBudget, ServiceMonitor, and PrometheusRule with multi-window burn-rate alerts for a 99.9% SLO. Helm chart and Kustomize overlays included.

[Unreleased]: https://github.com/apet97/go-clockify/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/apet97/go-clockify/releases/tag/v1.0.0
