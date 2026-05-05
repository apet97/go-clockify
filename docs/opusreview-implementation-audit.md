# Opus Review Implementation Audit

This document records the local implementation pass for the review
bundle under `~/Downloads/opusreview/`:

- `performance/REPORT.md`
- `quality/REPORT.md`
- `security/REPORT.md`

It is a prompt-to-artifact checklist, not launch-candidate evidence.
Local green is necessary but not sufficient: AGENTS.md still requires
scheduled live-contract evidence plus candidate-tag security and release
evidence before any launch-ready claim.

## Objective

Implement the actionable Opus review findings and make the MCP safer for
owner API-key testing across many workspaces/subjects.

Concrete success criteria:

- Tier 1 time-entry writes must not silently lose tags, task IDs, or
  custom-field payloads when updating an entry.
- Time-entry overlap protection, date parsing, dry-run envelopes, and
  report aggregation must be trustworthy enough for agent-driven local
  testing.
- Owner-key/multi-subject deployments must have clearer rate-limit,
  report-size, truncation, and load-harness guardrails.
- Security quick wins from the review must be implemented without
  weakening defaults or changing CI/CD without maintainer permission.
- Generated docs and tool catalogs must remain in sync with source.

## Performance Findings

| Finding | Status | Evidence |
|---|---|---|
| F1 broken `make verify-bench` fixtures | Implemented locally | `internal/tools/tier2_writes_bench_test.go` now uses the current `notes` and `TXT` schemas. Local `make verify-bench` progressed through `BenchmarkClockifyCreateExpense` and `BenchmarkClockifyCreateCustomField`; it still exits non-zero on this Mac because the committed baseline is linux/amd64 and local output is darwin/arm64. CI bench remains release evidence. |
| F2 global/per-token rate-limit relationship unclear | Documented, not default-changed | `docs/performance.md`, `docs/runbooks/rate-limit-saturation.md`, `internal/config/spec.go`, generated `README.md`, and `cmd/clockify-mcp/help_generated.go` now state that shared HTTP/gRPC deployments should size `CLOCKIFY_RATE_LIMIT >= active_subjects * CLOCKIFY_PER_TOKEN_RATE_LIMIT`. The default was not changed because the upstream Clockify budget is deployment-specific. |
| F3 `Pipeline.AfterCall` over-budget cost | Implemented | `internal/enforcement/enforcement.go` normalizes typed results directly before truncation when possible, and `internal/truncate/truncate.go` mutates the throwaway generic tree in place. Local benchmark moved `BenchmarkPipelineAfterCallLargeResult` from about 2.625 ms / 2.356 MB / 25,593 allocs to about 1.932 ms / 1.477 MB / 21,565 allocs on Apple M1. |
| F4 `tools/list` payload size | Partially addressed by existing cache; no schema compaction in this pass | The serialized tools/list cache remains the primary mitigation. Full schema-fragment compaction would require a broader descriptor-format change and was not included in this review wave. |
| F5 report memory ceiling | Implemented/documented | `CLOCKIFY_REPORT_MAX_ENTRIES` is documented in operator/testing docs and report metadata includes applied/requested/server caps. `docs/performance.md` now includes the report working-set formula and shared-deployment guidance. |
| F6 HTTP request allocation | Implemented for request bodies | `internal/mcp/transport_decode.go` replaces legacy and streamable HTTP `ReadAll` request parsing with a strict streaming decoder while preserving `413` for oversized malformed bodies and `-32700` for malformed JSON. Response encoder pooling was not added; the remaining cost is small and existing response semantics stayed unchanged. |
| F7 load harness lab-only coverage | Implemented | `tests/load/main.go` adds `tenant-churn`, `upstream-slow`, and `transport-fan-out`. Latest local runs passed all acceptance checks. |
| F8 `BenchHarness.requestID` unsynchronized | Implemented | `internal/testharness/harness.go` uses an atomic request ID and clarifies benchmark pipeline coverage. |

## Quality Findings

| Finding | Status | Evidence |
|---|---|---|
| F-QUALITY-01 update paths strip `tagIds` | Implemented | `internal/tools/common.go` centralizes `timeEntryPutPayload`; `internal/tools/entries.go` and `internal/tools/workflows.go` use it. Tests in `internal/tools/tools_test.go` cover preservation. |
| F-QUALITY-02 overlap detection misses entries started before query start | Implemented | `internal/tools/timesheet_workflows.go` pads overlap lookup by 24h. Tests cover padded lookup and overlap behavior. |
| F-QUALITY-03 report project names show `(no project)` | Implemented through hydrated reads | `internal/tools/reports.go` requests hydrated entries where needed. Test coverage asserts hydrated aggregate requests. |
| F-QUALITY-04 same-date bare ranges fail | Implemented | `parseRange` treats same bare-date start/end as the next midnight boundary. Tests cover same-date bare input. |
| F-QUALITY-05 inconsistent dry-run envelope shape | Implemented | `LogTime`, `StopTimer`, and `FindAndUpdateEntry` dry runs now return `ResultEnvelope`; test coverage pins handler dry-run envelopes. |
| F-QUALITY-06 webhook `auth_token` missing | Implemented | Webhook create/update accept `auth_token` and mask auth tokens in outputs and dry-runs. |
| F-QUALITY-07 `resolve_name` lacks task support | Implemented | `resolveNameInputSchema` and resolver code support project-scoped tasks; tests cover exact task match. |
| F-QUALITY-08 missing `dry_run` flags | Implemented | Tier 1 write schemas and handlers now support dry-run previews for create/start/switch paths. |
| F-QUALITY-09 time-entry writes omit `tag_ids` | Implemented for add/log paths | `clockify_log_time` and `clockify_add_entry` accept and send `tag_ids`. |
| F-QUALITY-10 `find_and_update_entry` drops `task_id` | Implemented | The shared PUT payload preserves existing task IDs and applies caller overrides. |
| F-QUALITY-11 natural-language datetime parsed in UTC | Implemented | Tier 1 entry/report/workflow paths use per-call `timezone`, `CLOCKIFY_TIMEZONE`, or local/server timezone. Generated tool catalog wording was updated. |
| F-QUALITY-12 webhook `authToken` returned in plain output | Implemented | List/get/create/update webhook outputs mask auth tokens. |
| F-QUALITY-13 truncation `meta.count` mismatch | Implemented | `internal/truncate/truncate.go` annotates envelope `meta.truncated`, `meta.returned`, and `meta.fetched` when top-level `data` arrays are reduced. |
| F-QUALITY-14 upstream error leaks full workspace URL/path | Implemented | `internal/clockify/errors.go` compacts upstream JSON errors to status/message/code without echoing full request URLs. |
| F-QUALITY-15 `search_tools` write classification | Intentionally unchanged | The deprecated shim can activate tools via `activate_group`/`activate_tool`; keeping it write-classified avoids bypassing activation audit semantics. |
| F-QUALITY-16 `list_tools` says keyword not query | Implemented | Tool description now says query string. |
| F-QUALITY-17 duplicate-name raw upstream JSON | Implemented via compact upstream errors | Duplicate-name responses use the compact error formatter. |

## Security Findings

| Finding | Status | Evidence |
|---|---|---|
| F-SECURITY-01 redactor masks operator diagnostics | Implemented | `internal/logging/redact.go` narrows sensitive-key matching; tests assert auth mode and TTL diagnostics remain visible. |
| F-SECURITY-02 ES* JWT verifier uses ASN.1 | Implemented | ECDSA JWT verification now uses raw JOSE R/S bytes; `TestVerifyJWTECDSARoundTrip` covers ES algorithms. |
| F-SECURITY-03 webhook DNS validation off by default | Implemented | `CLOCKIFY_WEBHOOK_VALIDATE_DNS` defaults on for every profile, with docs and generated help updated. |
| F-SECURITY-04 private-network gRPC lacks mTLS tenant requirement | Implemented | `private-network-grpc` profile sets `MCP_REQUIRE_MTLS_TENANT=1`; profile tests cover it. |
| F-SECURITY-05 redirect scheme downgrade | Implemented | Clockify HTTP client rejects HTTPS-to-HTTP redirects; test coverage pins it. |
| F-SECURITY-06 vault file backend arbitrary path | Deferred | The review marks this as ADR-required. No ad hoc behavior change in this pass. |
| F-SECURITY-07 `single-tenant-http` dev backend/audit posture | Deferred | The review marks this as an operator/deprecation decision. |
| F-SECURITY-08 destructive confirm-token contract | Deferred | This is an agent-contract/ADR change. |
| F-SECURITY-09 HSTS behind proxy | Implemented | Baseline headers emit HSTS behind proxy only when `X-Forwarded-Proto: https`. Tests cover the conditional. |
| F-SECURITY-10 local stdio owner-key posture | Implemented as docs | `docs/internal-test-posture.md` documents owner-key local testing guardrails. |
| F-SECURITY-11 deprecated `resolve_debug` alias | Implemented | `ResolveDebug` logs a deprecation warning. |

## Verification Evidence

Latest local verification on the dirty working tree:

- `make check` — passed.
- `make release-check` — passed with `release-check: OK - shippable`.
- `make config-doc-parity`, `make doc-parity`, `make catalog-drift`,
  and `git diff --check` — passed.
- Load scenarios:
  - `go run ./tests/load -scenario tenant-churn` — passed; final
    tracked subjects `0`.
  - `go run ./tests/load -scenario upstream-slow` — passed; noisy
    tenant isolated, quiet tenants completed.
  - `go run ./tests/load -scenario transport-fan-out` — passed;
    `200/200` calls reached fake upstream.
- Security preflight:
  - `make verify-fips` — passed locally.
  - `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` —
    no vulnerabilities found.
  - `semgrep scan --config auto --error --exclude go-clockify` —
    zero findings.
  - current-tree gitleaks snapshot still reports two
    `curl-auth-header` findings in `.github/workflows/docker-image.yml`.

## Remaining Blockers

These are not local code/test implementation gaps:

1. **CI workflow gitleaks findings.** The only current-tree gitleaks
   findings are dummy curl `Authorization: Bearer ...` smoke-test
   headers in `.github/workflows/docker-image.yml`. Fixing them
   requires explicit maintainer permission because AGENTS.md says not
   to change CI/CD unless asked.
2. **External launch evidence.** Group 1 needs two consecutive
   scheduled live-contract cron greens on the candidate SHA. Group 6
   needs candidate-tag security evidence. Group 7 needs candidate-tag
   release/sigstore/SLSA evidence.
3. **Deferred ADR/operator decisions.** Vault file backend hardening,
   `single-tenant-http` dev-backend posture, and destructive
   confirm-token contracts remain out of scope for ad hoc changes.

## Completion Assessment

The Opus implementation wave is locally shippable but not complete
against the full objective. The MCP is materially safer for local
owner-key testing across many workspaces/subjects, but launch-ready
status still depends on the explicit external evidence gates above and
on maintainer direction for the CI workflow gitleaks findings.
