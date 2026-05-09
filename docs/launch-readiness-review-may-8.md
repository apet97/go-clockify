# May 8 Launch-Readiness Review Disposition

Source of truth: `~/Downloads/review may 8/` as provided on
2026-05-08. This file is a maintainer-facing disposition ledger, not a
replacement for `docs/launch-candidate-checklist.md`.

## Closed in this remediation pass

- **T-02 initialize protocol header mismatch.** `streamable_http`
  now rejects an `initialize` request when the `Mcp-Protocol-Version`
  header is present but unsupported or contradictory with
  `params.protocolVersion`. Pinned by
  `TestStreamableInitializeProtocolVersionHeaderMismatch`.
- **T-04 HTTP admission error envelopes.** MCP endpoint admission
  failures now preserve their HTTP status while returning JSON-RPC 2.0
  error envelopes with `id:null`. The reserved transport error-code
  registry is documented in `docs/operators/error-codes.md`, and the
  contract is pinned by `TestHTTPTransportAdmissionErrorsUseJSONRPCEnvelope`.
- **T-22 optional protocol-version strictness.** Post-initialize
  `streamable_http` POST and SSE GET requests still accept missing
  `Mcp-Protocol-Version` headers by default for backwards
  compatibility, but every miss increments
  `clockify_mcp_protocol_version_header_missing_total`. Operators can
  set `MCP_HTTP_REQUIRE_PROTOCOL_VERSION=1` to reject missing headers
  after measuring client impact.
- **T-05 / T-19 legacy HTTP machine-readable deprecation and preflight.**
  Legacy POST-only HTTP responses now carry `Deprecation: true` and a
  successor `Link: </mcp>` header. Legacy preflight now admits
  `MCP-Protocol-Version` and `Last-Event-ID` headers, matching the
  streamable HTTP spelling used elsewhere. `docs/runbooks/legacy-http-eol.md`
  documents operator migration steps and keeps the future `Sunset`
  header tied to a concrete major-version release decision instead of
  an invented local date.
- **P1-1 audit outcome durability option.** Added
  `MCP_AUDIT_DURABILITY=fail_closed_strict` for operators who need
  post-mutation outcome persistence failures surfaced to clients.
  Existing `fail_closed` behavior remains intent-fail-closed for
  backwards compatibility. Audit persistence failures now expose a
  `phase="intent|outcome|single"` metric label, and the deploy
  monitoring includes a critical `ClockifyMCPAuditOutcomeNotDurable`
  alert for the `phase="outcome"` path. See
  `docs/runbooks/audit-durability.md`.
- **P1-2 govulncheck release pin and Go patch level.** Replaced the
  temporary unreleased `golang.org/x/vuln` commit pin in CI with
  `govulncheck@v1.3.0`, whose module declares `go 1.25.0`. During
  verification, the tagged scanner reported standard-library findings
  against Go 1.25.9/1.26.2 (GO-2026-4971 and GO-2026-4918), so the
  repo's Go pin moved to 1.25.10 across `go.mod`, `go.work`, nested
  build-tag modules, Actions workflows, support docs, and the Docker
  builder digest. `GOTOOLCHAIN=go1.25.10 govulncheck@v1.3.0 ./...`
  now reports no vulnerabilities. `make go-version-parity` now fails
  closed if module directives, workflow pins, current public docs, the
  CHANGELOG support-matrix wording, the FIPS go.mod-floor comment, or
  the Docker builder digest drift away from the root Go patch pin.
  Dependabot now watches the root module and all build-tag submodules
  (`internal/controlplane/postgres`, `internal/transport/grpc`, and
  `internal/tracing/otel`) so the third-party Go dependency surface is
  not hidden behind the root module's stdlib-only posture. The root
  watcher ignores build-tag dependency families so `go.work` does not
  produce broad root/workspace PRs for packages owned by the submodule
  watchers. Added
  `tools/govulncheck` as a separate tool module that pins
  `golang.org/x/vuln v1.3.0`; Dependabot watches that module and CI
  installs govulncheck from it instead of duplicating a shell-step
  version. `make verify-vuln` now installs and runs that pinned module
  under the repo Go pin instead of skipping when `govulncheck` is
  absent from `PATH`. The Vulncheck CI job now prints
  `govulncheck -version` before scanning so CI logs show the tagged
  scanner that actually ran. The final integrated plan's `L-08`
  candidate-tag govulncheck caveat is the same
  finding class and is covered by this `P1-2` disposition; final
  candidate-tag `make verify-vuln` evidence remains a Group 6 external
  gate.
- **P1-5 hosted OIDC verify-cache ceiling.** Hosted profiles clamp an
  explicit `MCP_OIDC_VERIFY_CACHE_TTL` above 60s back to 60s.
  Added `docs/runbooks/tenant-offboarding.md` for the final plan's
  tenant-offboarding operational requirement; it distinguishes the
  hosted 60s verify-cache ceiling from the separate IdP/proxy
  requirement to block still-valid offline JWTs.
- **P1-6 hosted OIDC `kid` requirement.** Hosted profiles now default
  `MCP_OIDC_REQUIRE_KID=1`; kid-less JWTs are rejected under that
  posture.
- **P1-9 forward-auth loopback narrowing.** An empty
  `MCP_FORWARD_AUTH_TRUSTED_PROXIES` is still only allowed on loopback
  binds, and config load now narrows that case to loopback CIDRs rather
  than trusting every source.
- **P1-10 hosted streamable HTTP TLS floor.** In-process TLS for
  `streamable_http` uses TLS 1.3 on hosted profiles and keeps TLS 1.2
  for self-hosted compatibility.
- **P2-1 / P2-2 bearer parsing.** Main HTTP auth and metrics bearer
  parsing now accepts case-insensitive `Bearer` schemes and trims token
  whitespace before constant-time comparison. Inline `/metrics`
  inheritance and static-bearer modes are both pinned so they cannot
  regress away from the shared bearer parser.
- **P2-3 streamable CORS resume header.** Streamable HTTP preflight now
  advertises `Last-Event-ID` so browser SSE shims can resume with the
  same header the event stream consumes.
- **P2-10 credential payload bound.** Inline/env/file JSON credential
  material now fails closed above 64 KiB before JSON decoding.
- **D1 / D2 / D4 / D6 doc drift.** Publishable docs now align on the
  current 128-tool catalog and five policy modes; `CONTRIBUTING.md`
  calls out private-repo clone requirements; `SECURITY.md` scope now
  includes the live auth/audit/tenant isolation surfaces. The
  `doc-parity` gate now fails closed when current public-surface docs
  claim a stale total tool count or stale "all tool handlers" count
  instead of the generated `docs/tool-catalog.json` total, when the OCI
  image descriptions in `.github/workflows/docker-image.yml` or
  `deploy/Dockerfile` omit the current generated-tool and policy-mode
  counts, and when `SECURITY.md` scope drops the live auth, audit,
  tenant-isolation, or transport surfaces advertised by the security
  features section. A follow-up observability drift guard keeps
  `SECURITY.md` on the phase-labeled audit failure selector and keeps
  `docs/auth-model.md` on the live
  `clockify_mcp_rate_limit_rejections_total{kind,scope}` metric instead
  of the removed per-subject-only metric name.
- **Production-readiness blocker-scope wording.**
  `docs/production-readiness.md` and
  `docs/official-clockify-mcp-gap-analysis.md` no longer narrow the
  remaining launch blockers to "external evidence only." They now keep
  local green checks below the still-open Group 1, Group 6, Group 7,
  pushed-workflow, repository-state, public-readiness,
  hosted/platform, and legal/product approval gates.
  `docs/agent-handoff.md` also scopes the Group 1/6/7 summary as an
  incomplete launch-evidence list rather than the whole blocker set.
  `doc-parity` fails closed if these pages collapse back to local-green
  or evidence-only launch readiness.
- **Agent handoff permissioned landing sequence.** `docs/agent-handoff.md`
  preserves the explicit-approval landing sequence for the dirty
  remediation tree: rerun status/doc/diff checks before staging, do not
  use `git add .` from a parent workspace, keep commit `Why:` and
  `Verified:` lines, do not push until local green, and refresh
  `launch-external-status` on the landed SHA. `doc-parity` now fails
  closed if that sequence is removed.
- **Launch-state baseline wording.** `AGENTS.md`,
  `docs/agent-handoff.md`, and `docs/production-readiness.md` now name
  the actual current pushed `main` / `origin/main` baseline
  (`2e7b6bd4a7968ba45921e103d948f74dd82175b8`) instead of treating the
  older cron-proven, post-PR #62, or post-PR #63 SHAs as current. The older
  `4fe957547f9e6aea749a85f87823d17a0ccc2928`,
  `ff0047aa50cdcd4bb43037c72d66b218d51f13e8` and
  `0960bfa03db143778deb59f9b9522012116c9c9b` baselines remain documented
  as manual/historical evidence, not candidate-SHA closure.
- **Agent entrypoint read-first routing.** `AGENTS.md` now points
  future agents at this May 8 disposition ledger and its
  objective-to-artifact completion audit before the narrative gap
  analysis, so the repo-level entrypoint cannot bypass the current
  source-of-truth remediation record. `doc-parity` has a regression
  guard for the pointer.
- **Public onboarding security/ownership hygiene.** Added
  `.github/ISSUE_TEMPLATE/config.yml` so the issue chooser disables
  blank public issues and links vulnerability reports to GitHub
  Security Advisories. The bug and feature forms now warn reporters not
  to post secrets, personal data, or vulnerability details publicly,
  and the bug reproduction placeholder uses
  `CLOCKIFY_API_KEY=<redacted>`. `doc-parity` now fails closed if the
  advisory link, public-warning text, redacted placeholder contract,
  SECURITY.md / SUPPORT.md response-time posture, or CODEOWNERS
  sensitive-path ownership contract drifts. It also pins the
  `CONTRIBUTING.md` public-compatible HTTPS clone command and the
  no-additional-auth public visibility caveat from the final integrated
  plan's onboarding checklist.
- **D5 ADR-0017 status drift.** `docs/adr/README.md` no longer lists
  ADR 0017 as Proposed, and the launch checklist now references the
  ADR as Accepted. The ADR file itself was already Accepted and
  records the Path A decision. The `doc-parity` gate now checks ADR
  index status labels against each ADR file's `## Status` section so
  accepted/proposed drift fails closed.
- **Review-ledger coverage guard.** `make doc-parity` now runs
  `scripts/check-launch-review-ledger.sh`, which pins the May 8 review
  ID inventory (`T-01`..`T-24`, `MP-01`..`MP-13`, `P1-1`..`P3-7`,
  `D1`..`D10`, `G-01`..`G-05`, `L-01`..`L-05`, `L-08`, and `L-10`)
  and fails if the disposition ledger drops any reviewed finding
  outside the summary/verification sections. When
  `REVIEW_SOURCE_DIR` or `~/Downloads/review may 8` exists, it also
  compares the pinned inventory against the actual source bundle so a
  newly discovered source ID cannot be hidden by a stale hardcoded
  manifest. It also fails if the concrete external-gate ledger drops
  Group 1/6/7 evidence, SLSA/private-repo stance, repository
  description, npm publish, branch-protection/local-branch, stale-PR,
  issue #28, pushed CodeQL/dependency-review/Semgrep evidence, RLS,
  cross-replica quota, or trademark/legal approval entries. It also
  fails if the
  objective-to-artifact completion audit is removed, loses the explicit
  not-complete status, or omits the still-open Group 1/6/7 and
  external approval blockers.
- **External launch-status helper.** Added
  `scripts/check-launch-external-status.sh` and `make launch-external-status`
  as a read-only snapshot for the gates that local tests cannot close:
  final-SHA scheduled live-contract runs, mutation cron evidence, pushed
  CodeQL/dependency-review/Semgrep workflow runs, repository description
  drift, issue #28, stale open PRs without `wip` / `blocked` labels,
  npm wrapper visibility, and optional expected-version npm release
  proof. It also inspects `gh run view --log` before closing Group 1,
  so two green scheduled runs only count if the logs include the
  mutating, audit, and schema-diff tier markers
  (`TestE2EMutating`, `TestLiveCreateUpdateDeleteEntryAuditPhases`,
  and `TestLiveReadSideSchemaDiff`). The helper is
  report-only by default and has an optional `--fail-open` mode for
  maintainers who want a non-zero exit while any external gate remains
  open or unknown. It also prints explicit maintainer action hints for
  every open or unknown gate, including the still-needed npm publish
  evidence on the next rc/release. Its regression test covers plan
  mode, all-closed live status, dirty trees, stale scheduled runs,
  missing workflows, unreadable branch protection, stale local
  branches, stale open PRs, stale repository metadata, and stale issue
  state, missing npm expected-version proof, and missing
  live-contract cron log markers through offline `gh` / `npm` / `git`
  stubs.
- **Public content audit helper.** Added
  `scripts/check-public-content-audit.sh` and `make public-content-audit`
  as a read-only wrapper around the final integrated plan's public repo
  flip checks. It runs a candidate branch-content gitleaks scan over
  tracked plus unignored files, also summarizes full working-tree
  gitleaks findings from redacted metadata only, reports tracked
  personal/scratch references by file and line, TLS verification
  bypass markers in candidate branch content, root MIT license
  presence, `.gitignore` coverage for workstation-private state and
  generated artifacts, `.gitleaks.toml` allowlist descriptions,
  `CLAUDE.md` workstation context files, live Clockify secret env
  assignments, lists env-like files separately for candidate branch
  content and the full working tree with tracked/ignored/untracked
  classification, reports tracked Go/Markdown task markers mentioning
  internal/private context by file and line, and redacts matching
  commit messages to commit IDs only.
  The summary separates candidate branch file content from public
  history review and local artifact/full-tree cleanup so ignored
  workstation files cannot be mistaken for publishable branch content.
  Default mode exits 0 for handoff snapshots; `--fail-open` gives
  maintainers a strict pre-public-flip gate.
- **T-12 client guidance.** `docs/clients.md` now documents that
  oversized HTTP bodies intentionally fail as HTTP 413 before JSON-RPC
  dispatch.
- **T-01 process-local HTTP admission limits.** Legacy HTTP and
  streamable HTTP now enforce app-layer admission limits before JSON-RPC
  dispatch: per source IP, per authenticated subject+tenant, and
  concurrent streamable SSE GETs per session. Hosted profiles and
  deployment manifests default to `600`/`300`/`4`; public hosted
  deployments still need cross-replica gateway limits.
- **T-06 / T-07 gRPC drain and slow-consumer hardening.** gRPC
  server-initiated notifications now use a bounded per-stream enqueue
  wait and count slow-consumer drops with
  `clockify_mcp_grpc_notification_drops_total{reason="slow_consumer"}`.
  gRPC shutdown now flips cached readiness to `NOT_SERVING` before
  `GracefulStop` drains existing streams.
- **T-15 / T-17 / T-18 gRPC launch hardening.** Default-build
  `MCP_TRANSPORT=grpc` now fails at process entry and in `doctor`
  with `-tags=grpc` artifact guidance. Plaintext gRPC is refused on
  non-loopback binds, and the private-network gRPC profile documents
  that reflection remains intentionally off in supported release
  artifacts. A development-only `grpcreflection` build tag now
  registers server reflection for local protocol exploration only, and
  `check-build-tags` proves the tag compiles without adding it to the
  release matrix. A build-tagged regression test also asserts the
  optional hook registers a `grpc.reflection.*` service only when that
  tag is present. The `grpc-auth-smoke` helper now explicitly
  distinguishes direct submodule auth tests from the production
  binary's `-tags=grpc` link posture.
- **T-16 gRPC peer-address allowlist.** Added optional
  `MCP_GRPC_PEER_CIDR_ALLOW` defence-in-depth for private-network gRPC
  deployments. When set, stream and unary auth interceptors reject
  missing, non-TCP, or out-of-range peers before authentication and
  increment
  `clockify_mcp_grpc_auth_rejections_total{reason="peer_addr_disallowed"}`.
  Empty remains the default to preserve bufconn, service-mesh, and
  existing deployment compatibility.
- **T-08 cross-pod rehydration race pin.** Concurrent cold
  rehydration now has a regression test that forces multiple
  `Factory` calls for one persisted session ID, asserts every caller
  receives the same retained `streamSession`, and verifies discarded
  runtimes are closed.
- **T-09 list-changed notification parity.** The cross-transport
  parity test now fires and expects
  `notifications/tools/list_changed`,
  `notifications/resources/list_changed`, and
  `notifications/prompts/list_changed` across stdio,
  `streamable_http`, and gRPC.
- **T-10 repeat `initialize` state reset.** A repeated
  `initialize` now cancels and untracks in-flight `tools/call`
  requests, invalidates the cached `tools/list` payload, and
  documents that transport-owned listChanged advertisement remains
  sticky.
- **T-14 cross-pod cancellation contract.** Streamable HTTP now has a
  negative regression test for the ADR 0017 best-effort cancellation
  boundary: instance B can rehydrate a session and accept
  `notifications/cancelled` while an in-flight call on instance A
  continues until local completion.
- **MP-01..MP-13 cross-transport parity matrix.** The May 8
  transport report's parity-matrix backlog is dispositioned through
  its matching transport findings: `MP-01` via `T-20` plus
  `TestDispatchToolsListSerializedCacheStillRequiresInitialize`;
  `MP-02` and `MP-03` via `T-09`; `MP-04` via `T-08`; `MP-05` via
  `T-14`; `MP-06` via `T-02`; `MP-07` and `MP-08` via `T-06` /
  `T-07`; `MP-09` via `T-10`; `MP-10` via `T-04`; `MP-11` via
  `T-01`; and `MP-12` via `T-05` plus
  `docs/runbooks/legacy-http-eol.md`, with the concrete `Sunset` date
  still left to the major-version release decision. `MP-13` is covered
  by `T-24` through the `/mcp/events` counter and starter alert rules.
- **P2-3 / T-24 SSE resume and legacy alias observability.**
  Streamable HTTP preflight now admits `Last-Event-ID`, and
  `GET /mcp/events` increments
  `clockify_mcp_streamable_events_alias_requests_total` so operators can
  identify legacy clients still using the alias. The starter
  Prometheus alerts, Kustomize `PrometheusRule`, and Helm
  `PrometheusRule` now include `ClockifyMCPStreamableEventsAliasUse`
  so non-zero alias use becomes an operator migration signal. The full
  streamable HTTP mux now has a regression test that verifies the same
  alias request also lands in
  `clockify_mcp_http_requests_total{path="/mcp/events",method="GET",status="200"}`.
- **T-13 SSE direct-write justification.** The `nosemgrep`
  suppressions on streamable HTTP SSE direct writes now carry local
  call-site comments explaining that session IDs and event IDs are
  server-generated, event names are server constants, and event payloads
  are JSON-marshaled before `text/event-stream` framing.
- **P2-7 panic stack path hygiene.** Recovered panic logs still emit a
  `panic_recovered` event and counter, but stack traces are bounded and
  scrub `$GOROOT`, `$GOPATH`, current working directory, and `$HOME`
  prefixes before logging.
- **P2-6 secret-shaped log value scanning.** The slog redactor now masks
  obvious JWT, private-key, `pk_...`, and token/API-key query string
  values even when they arrive under otherwise neutral attribute keys.
  Panic recovery now has a regression test that installs the standard
  redacting logger and proves a secret-shaped panic value is masked in
  the `panic_recovered` operator log.
- **P3-5 cross-origin isolation headers.** The shared HTTP baseline
  header helper now emits `Cross-Origin-Opener-Policy: same-origin`,
  `Cross-Origin-Embedder-Policy: require-corp`, and
  `Cross-Origin-Resource-Policy: same-origin` alongside the existing
  CSP, frame, referrer, permissions, nosniff, cache, and conditional
  HSTS headers. `TestApplyHTTPBaselineHeaders` pins the emitted header
  set, and `SECURITY.md` documents the browser-facing hardening.
- **P3-6 hosted default tenant warning.** `doctor --strict` now emits a
  non-fatal warning when a hosted profile still resolves
  `MCP_DEFAULT_TENANT_ID` to the literal `default`; strict exit code 3
  remains reserved for fatal posture errors. The shared-service example
  env, verification smoke, and hosted launch checklist now use a
  deployment-specific fallback sentinel.
- **P3-7 JWT JSON decode bounds.** `decodeJWT` now rejects oversized
  decoded JOSE header and claims JSON before unmarshalling (16 KiB /
  128 KiB respectively). `docs/auth-model.md` documents the local
  parser hardening and the direct unit test pins both rejection paths.
- **P3-3 tenant-scoped audit index.** Added Postgres migration
  `004_audit_events_tenant_at_index.sql` with
  `idx_audit_events_tenant_id_at` on `(tenant_id, at)` so
  hosted/support audit review can filter one tenant over a bounded
  time window without depending on the global retention index.
  `internal/controlplane/COMPAT.md` now records migrations 002, 003,
  and 004, and a migration unit test pins the new index definition.
- **P1-7 `session_affinity_id` phantom column.** Removed
  `SessionAffinityID` from the durable `SessionRecord` contract and
  from Postgres session SELECT/UPSERT paths. Added migration
  `003_drop_session_affinity_id.sql` and doctor drift checks so upgraded
  databases must record the migration and must not keep the old column.
- **P2-4 redaction false positives.** Default redaction remains the
  conservative substring match used for launch hardening. Operators or
  tests that can maintain a tighter key list can now opt into
  `RedactingHandler.WithSensitiveKeyBoundaryMatching()` to avoid false
  positives such as `tokenizer_results` while still redacting delimited
  keys like `session_token`, `x-api-key`, and `access_token`.
- **P1-4 CodeQL / SAST workflow artifact.** Added
  `.github/workflows/codeql.yml` for Go with SHA-pinned CodeQL actions,
  weekly/push/PR triggers, `security-events: write`, and a focused
  `.github/codeql/codeql-config.yml` that ignores the upstream-shaped
  `internal/clockify/**` client. The first successful GitHub run and
  SARIF visibility remain external evidence.
- **Semgrep CI coverage gap.** Added `.github/workflows/semgrep.yml`
  as a recurring Semgrep CE scan for push, pull request, manual, and
  weekly runs. The workflow uses the same `p/default`, metrics-off,
  fail-on-finding command shape as the Group 6 candidate-tag evidence
  plan, with SHA-pinned checkout and a digest-pinned Semgrep
  `1.162.0` container. This does not replace the final candidate-tag
  Semgrep evidence.
- **P2-5 dependency-review workflow artifact.** Added
  `.github/workflows/dependency-review.yml` for PR-time and
  default-branch dependency vulnerability review with SHA-pinned
  `actions/dependency-review-action@v5.0.0`, read-only contents/PR
  permissions, high-severity blocking, and explicit push compare refs
  so the first post-landing `main` run can serve as workflow evidence.
  First-run GitHub evidence remains external.
- **P1-8 RLS posture documentation.** `docs/auth-model.md` now states
  that v1.x uses application-layer tenant scoping in the Postgres
  control plane, with database-enforced RLS deferred as a paid
  commercial hosted-plane defense-in-depth gate. `docs/runbooks/postgres-restore.md`
  now carries the RLS-aware restore checks to run once an RLS migration
  exists, while explicitly stating that the current v1.x schema does
  not close the paid-hosted RLS gate.
- **Final plan 10.3 operational runbooks.** Added
  `docs/runbooks/rate-limit.md` as the named launch-checklist entry
  point for T-01 rate-limit operations and linked it with the existing
  saturation runbook. `docs/release/public-hosted-launch-checklist.md`,
  `docs/README.md`, and `docs/production-readiness.md` now link the
  rate-limit and tenant-offboarding runbooks. `doc-parity` fails
  closed if the required hosted-launch runbook entry points disappear
  or lose their key rate-limit, tenant-offboarding, or RLS restore
  guidance.
- **Final plan 10.3 on-call observability.** Added
  `ClockifyMCPHTTPAdmissionRejections` alongside the existing
  rate-limit alert in `deploy/monitoring/prometheus-alerts.yaml`,
  `deploy/k8s/base/prometheus-rule.yaml`, and the Helm
  `PrometheusRule`. Added the missing `ClockifyMCPAuditFailure` alert
  to the deploy/Helm PrometheusRule surfaces. Added the
  `phase="intent|outcome|single"` label to
  `clockify_mcp_audit_failures_total`, plus the required
  `ClockifyMCPAuditOutcomeNotDurable` alert on
  `clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}`.
  Added `clockify_mcp_http_admission_rejections_total{path,reason}` to the
  Grafana rate-limit panel and grouped the audit-failure panel by
  reason/phase. `doc-parity` now fails closed if the hosted-launch
  monitoring artifacts lose either the outcome audit-failure alert or
  `clockify_mcp_http_admission_rejections_total`.
- **T-23 stdio message-size documentation.** `docs/support-matrix.md`
  now states that `MCP_MAX_MESSAGE_SIZE` defaults to 4 MiB and caps
  stdio scanner frames, HTTP request bodies, and gRPC inbound frames.
- **T-21 protocol version capability map.** Official MCP docs confirm
  `2025-11-25` is a published/latest revision. Rechecked on
  2026-05-09 against the official lifecycle, changelog, and tasks pages:
  the lifecycle page shows server capability examples for
  prompts/tools/resources `listChanged`, resources `subscribe`, and
  tasks when a server implements task-augmented requests
  (`https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle`;
  `https://modelcontextprotocol.io/specification/2025-11-25/changelog`).
  The compatibility test now pins that every advertised protocol version
  gets the same implemented capability map: tools/prompts listChanged,
  resources subscribe/listChanged when resources are installed, and no
  2025-11-25 task capability until task-augmented requests are implemented
  (`https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks`).
- **Future-client server identity.** The review flagged that the
  protocol `serverInfo.name` is `clockify-go-mcp` while packaging
  surfaces use `clockify-mcp` / `@apet97/clockify-mcp-go`. The wire
  value is already pinned by `scripts/smoke-stdio.sh`; `docs/clients.md`
  now tells exact-match clients to compare `serverInfo.name` against
  `clockify-go-mcp` and treat binary/npm names as packaging identities.
- **Future-client default protocol version.** The review suggested an
  operator escape hatch for clients that omit `params.protocolVersion`
  while a newer MCP revision is rolling out. Added
  `MCP_DEFAULT_PROTOCOL_VERSION` as an opt-in fallback for omitted
  initialize versions only: explicit supported client versions are still
  echoed, and unsupported requested versions still negotiate to the
  newest supported version. `docs/clients.md`, config validation, and
  protocol-version tests pin the behavior.
- **T-11 session ID log hygiene.** Streamable session IDs are still
  generated from 16 bytes of `crypto/rand` and constrained to visible
  ASCII. Store-error logs no longer include the raw session ID, and the
  regression test asserts both metrics and warning logs remain present
  without leaking `session_id`.
- **P2-8 audit external-id phase/outcome semantics.** No code change:
  the review classifies this as a documented forensics boundary, and
  `internal/auditbridge/auditbridge.go` plus its tests already pin that
  phase and outcome stay in the synthesized external ID so two-phase
  audit rows are not collapsed by Postgres idempotency.
- **P2-9 live audit DSN proof.** Rechecked read-only on 2026-05-09:
  `gh variable list --repo apet97/go-clockify` reports
  `CLOCKIFY_LIVE_AUDIT_REQUIRED` exists, and scheduled
  `live-contract.yml` run 25538247771 logs show
  `AUDIT_REQUIRED: true`, a redacted `MCP_LIVE_CONTROL_PLANE_DSN`,
  and `go test -tags=postgres,livee2e -run
  '^TestLiveCreateUpdateDeleteEntryAuditPhases$' ./...` ending
  `ok github.com/apet97/go-clockify/internal/controlplane/postgres`.
  `scripts/check-launch-external-status.sh` now verifies the repository
  variable is present and set to `true` before it will report the
  external launch snapshot. This closes the standalone P2-9
  configuration proof; Group 1 still needs scheduled live-contract
  greens on the final candidate SHA.
- **Mutation cron timeout.** The 2026-05-07 and 2026-05-08
  scheduled mutation runs were cancelled because only the
  `internal/tools` matrix leg hit the old 45-minute job timeout. The
  workflow now allows 90 minutes so a slow but otherwise healthy package
  reports success/failure instead of cancellation.
- **B.07 operator-facing marker audit.** Rechecked the code-quality
  marker scan requested by the final integrated plan against non-test
  `internal/**/*.go` and `cmd/**/*.go`; the current tree has no hits in
  those operator-facing paths. `make public-content-audit` now reports
  that scan separately from test fixtures, scripts, and documentation
  examples so public-flip evidence cannot confuse fixture text with
  launch-facing implementation debt.
- **B.07 per-package coverage outlier.** `internal/runtime` remains the
  lowest visible package in the full coverage run because it is mostly
  profile/build-capability glue, but it is now covered by a conservative
  40% floor in `scripts/check-coverage.sh` and documented in
  `docs/coverage-policy.md`. This makes regressions visible while
  leaving follow-up branch coverage work explicit.
- **B.07 proposed-ADR audit.** ADR 0010 was carrying a shipped metrics
  architecture decision while still marked Proposed, so it is now
  Accepted and the ADR index reflects that status. ADR 0018 remains
  Proposed because confirmation-token enforcement still depends on a
  future MCP-client-coordinated token format.
- **B.07 `internal/jsonschema` architecture trade-off.** ADR 0001 now
  documents why the runtime tool-argument validator remains a narrow
  stdlib-only Draft 2020-12 subset instead of importing a full engine
  such as `github.com/santhosh-tekuri/jsonschema/v6`: the generated
  catalog does not need `$ref`, `$defs`, conditionals, or full schema
  composition, and `internal/tools/schema_keyword_test.go` fails when a
  descriptor uses unsupported keywords. `doc-parity` now fails closed
  if that B.07 rationale disappears.
- **B.08 dependency/license evidence path.** Added
  `scripts/collect-license-evidence.sh` with `make license-evidence`
  and `make license-evidence-plan` so legal/product reviewers can get a
  repeatable raw inventory of `go list -deps` module graphs plus local
  `LICENSE*`, `NOTICE*`, or `COPYING*` candidates for the default,
  FIPS, Postgres, gRPC, gRPC+Postgres, and OTel build variants. The
  helper deliberately does not classify licenses, inspect source
  headers, scan npm transitives, or close `L-10`; it is evidence input
  for counsel, not legal clearance.
- **Coordinator Appendix B file-scope audit.** The coordinator's
  "files this coordinator did NOT deeply read" list is covered by the
  specialist dispositions above: transport/session code, auth/policy
  and enforcement, Clockify pagination, parity tests, JSON Schema,
  GoReleaser/release assets, deploy manifests, runbooks, API coverage,
  and CODEOWNERS all route to concrete code/doc/test evidence or to
  still-open external approval gates. `scripts/check-launch-review-ledger.sh`
  now fails closed if this un-IDed file-scope coverage disappears.
- **Final integrated plan checklist coverage.** The final plan's
  §9 public repo reopening checklist is covered by
  `make public-content-audit`, `make launch-external-status`, issue
  template / SECURITY / CODEOWNERS doc-parity guards, onboarding drift
  guards, the closed public-history and local-artifact review notes,
  and the still-open branch-protection, local-branch, and
  repository-description gates. The
  §10 paid hosted launch checklist is covered by the local T/P/L
  dispositions above, the rate-limit / audit-durability /
  postgres-restore / tenant-offboarding runbooks and monitoring guards,
  plus the still-open product/platform, supply-chain, commercial,
  contractual, privacy, trademark, and external-attestation gates in
  §10.1, §10.2, §10.3, §10.4, and §10.5. `scripts/check-launch-review-ledger.sh`
  now fails closed if this un-IDed checklist coverage disappears.
- **Group 6 local security preflight refresh.** Rechecked the current
  dirty May 8 remediation tree on 2026-05-09 with the pinned Go
  toolchain. `govulncheck@v1.3.0` reported no vulnerabilities,
  Semgrep `p/default` returned 0 findings, and `make verify-fips`
  passed on this FIPS-capable macOS arm64 host including the
  `-tags=fips,grpc` build combination. `make secret-scan` is not green
  on this workstation because it scans the full local working tree and
  the remaining findings are in ignored/local artifacts (`.local/`,
  `.serena/`, and the duplicate `go-clockify/` checkout); the
  candidate branch-content gitleaks scan remains closed through
  `make public-content-audit`.
  Group 6 still needs the same suite from a clean checkout of the final
  candidate tag.
- **B.03 live-contract coverage inventory.** Added
  `scripts/check-live-tool-coverage.sh` plus a regression test so the
  generated catalog cannot gain a Tier-2 or API-backed Tier-1 tool that
  is absent from the livee2e source inventory. The guard currently
  reports all 88 Tier-2 tools named in livee2e source, all API-backed
  Tier-1 tools named in livee2e source, and four explicitly local-only
  Tier-1 catalog/tool-surface helpers that are covered by unit and
  contract tests instead of fake live Clockify calls. Added a safe
  `clockify_resolve_name` read-only live probe to close the last
  API-backed Tier-1 name gap. This is a static inventory guard only:
  Group 1 still requires scheduled cron evidence, and
  `docs/api-coverage.md` still distinguishes full success paths from
  documented unsupported, permission-gated, plan-gated, or
  workspace-state-limited live probes.
- **B.04 tool contract matrix.** `TestToolContractMatrix` now proves
  the full 128-tool registry has policy and annotation coverage, every
  destructive tool is globally dry-run intercepted and advertises
  `dry_run` / `dryRun`, `time_tracking_safe` permits only the eight
  timer/time-entry write helpers while blocking Tier 2 groups, and the
  `safe_core` write list remains explicit.
- **B.05 performance and large-workspace evidence.** Rechecked the
  existing synthetic large-workspace and report aggregation benchmarks
  locally on 2026-05-09 with `go test ./internal/tools -run '^$'
  -bench 'Benchmark(LargeWorkspaceReadiness|AggregateEntriesRange)'
  -benchmem -benchtime=200ms -count=1` on darwin/arm64 Apple M1:
  aggregate no-raw-retention 1000-entry report was 6.4 ms / 1.0 MiB,
  11999-entry summary report was 35.0 ms / 12.2 MiB, project-filtered
  list was 2.1 ms / 1.0 MiB, and page-five find/update was 2.5 ms /
  1.3 MiB. This is local regression evidence for the review's
  pagination and bounded-report questions, not a replacement for the
  committed linux/amd64 Bench workflow baseline. `check-bench-baseline`
  still passes locally; the latest read-only `gh run list` snapshot
  shows manual Bench run 25327364929 green and the latest scheduled
  Bench run 25306464015 still failed on an older SHA, so final-SHA
  scheduled Bench evidence remains a separate CI observation.
- **B.06 CI/release evidence pipeline.** Rechecked all workflow files
  with `go run github.com/rhysd/actionlint/cmd/actionlint@latest
  -color=false .github/workflows/*.yml`; no findings were emitted.
  `release-smoke.yml` treats only GitHub's user-owned-private
  attestation feature-gate wording as non-fatal and does not skip on a
  bare 404; wrong owner, signature mismatch, missing attestation,
  tampering, and other attestation errors still fail. `deploy.yml`
  now uses the same narrow note-skip for the GitHub attestation
  feature gate, no longer claims SLSA provenance always runs, and has
  its checkout plus cosign-installer actions pinned to full commit
  SHAs. `doc-parity` now fails closed if the release-smoke or deploy
  SLSA skip regresses to broad bare-404 matching, if any workflow action
  reference reverts from a full commit SHA to a tag or branch, and if
  README reverts to unqualified "every binary and container image ships
  with ... SLSA" wording. `docker-image.yml`, `release.yml`, and
  `docs/verify-release.md` now qualify SLSA provenance as available
  only when GitHub artifact attestations are available, while keeping
  image build, scan, cosign signature, raw binary, and SBOM gates
  mandatory. `docker-image.yml` also emits an explicit ADR-0013 notice
  if the best-effort image SLSA attestation step fails, so the workflow
  log calls out the user-owned-private feature gate while preserving the
  mandatory image build, Trivy, cosign signature, and SBOM attestation
  gates. `SUPPORT.md` no longer claims SLSA has been mandatory on every
  release since the public flip; it now names the private-repo
  attestation limitation and the cosign binary/image chain as the
  mandatory cryptographic gate. `release.yml` runs
  `scripts/check-release-assets.sh` immediately after GoReleaser, and
  `ci.yml` runs
  `scripts/test-check-release-assets.sh`, which also passed under the
  current `make release-check`. `docs/runbooks/release-candidate-evidence.md`
  now tells the operator to run
  `scripts/check-launch-external-status.sh --candidate-sha <sha>
  --expected-npm-version vX.Y.Z-rc.N --fail-open` after the candidate
  release path publishes, and `scripts/prepare-rc-evidence.sh --plan`
  now includes both the non-failing `LAUNCH_EXPECTED_NPM_VERSION`
  snapshot and the final fail-open command, so scheduled workflow,
  repo-state, and npm expected-version proof are verified from the
  same read-only snapshot. It also captures `gh release view <tag>
  --json assets` and validates the GitHub Release asset names against
  `scripts/check-release-assets.sh`, closing the source-plan command
  gap for L-01's 46-asset validation. `release-smoke.yml` now writes
  default positive, default negative, and Postgres
  `doctor --strict` outputs under `release-smoke-evidence/`, validates
  those files before the job can pass, and uploads them as the
  `release-smoke-doctor-output` artifact. The release-candidate plan
  and runbook also label per-workflow `gh run list` logs as raw
  latest-run metadata snapshots, not final-candidate-SHA proof, and now
  include the `release.yml` and `docker-image.yml` workflows named by
  the final plan's Day 2 and evidence-command sections. The fail-open
  external-status command is the validator for workflow-backed launch
  boxes. The helper now also requires `npm` before run-mode evidence
  capture so the expected-version proof cannot silently degrade to an
  unknown log entry.

## Already closed before this pass

- **T-03 stdio dispatch concurrency.** `MCP_MAX_INFLIGHT_TOOL_CALLS`
  already defaults to 64 at the reviewed HEAD and is covered by
  `internal/mcp/server_concurrency_test.go`.
- **T-20 `tools/list` before initialize.** The cached `tools/list`
  fast path already checks `s.initialized.Load()` before returning
  cached bytes.
- **ADR 0017 Path A implementation.** `docs/adr/0017-streamable-http-session-rehydration.md`
  was already Accepted, and the implementation is pinned by the
  cross-instance rehydration tests.
- **D10 working-tree clutter.** Added `make clean-deep` as an explicit
  opt-in cleanup target for ignored build/scratch artifacts such as
  `clockify-mcp`, `coverage.out`, `.bench/`, `.local/`, `.review/`,
  `.agent-*`, `.serena/`, and the duplicate `go-clockify/` checkout.
  The target refuses to run unless `CONFIRM=1` is set, so this pass did
  not delete maintainer-local artifacts.

## Still open code or CI hardening

- No remaining medium-risk protocol parity pins from
  `01_MCP_PROTOCOL_TRANSPORTS.md` are open in this local remediation
  wave. Remaining items in this section are larger compatibility,
  CI, or external-approval work.

## Review ID coverage audit

- **Source inventory.** The review source folder currently contains
  `00_COORDINATOR_INDEX.md`, `01_MCP_PROTOCOL_TRANSPORTS.md`,
  `02_SECURITY_AUTH_TENANT_ISOLATION.md`, and
  `10_FINAL_INTEGRATED_LAUNCH_PLAN.md`.
- **MCP / transport findings.** `T-01` through `T-24` each have a
  disposition above: local fixes for the actionable blockers and
  parity gaps, or "already closed" for `T-03` and `T-20`.
- **MCP parity matrix.** `MP-01` through `MP-13` each map to a
  disposition above through their corresponding `T-*` finding and
  concrete regression tests, with the `T-05` `Sunset` timing documented
  as a release-versioning decision rather than an invented local date.
- **Security findings.** `P1-1` through `P1-10` and `P2-1` through
  `P2-10` each have a disposition above. Local-safe items are fixed or
  covered by new workflow artifacts; `P1-3`, `P1-8`, and `P2-5`
  remain evidence/product/platform gates. `P2-9` is verified for the
  current main-repo live-contract configuration, while final-SHA
  audit-phase evidence remains part of Group 1. `P3-3`, `P3-5`,
  `P3-6`, and `P3-7` are fixed locally; `P3-1`, `P3-2`, and `P3-4`
  remain low-risk deferred follow-ups or no-action items.
- **Coordinator drift findings.** `D1`, `D2`, `D4`, `D5`, `D6`, and
  `D10` are fixed or documented locally. `D3`, `D7`, `D8`, and `D9`
  remain maintainer or GitHub-state actions and are listed under
  external evidence or approval gates.
- **Final integrated plan IDs.** `G-01` through `G-05`, `L-01`
  through `L-05`, `L-08`, and `L-10` are dispositioned above or
  through their matching `D*` / `P*` entries. `L-08` appears once in
  `10_FINAL_INTEGRATED_LAUNCH_PLAN.md` as a govulncheck caveat pointer,
  but no standalone `L-08` backlog entry exists; it is covered by the
  `P1-2` disposition. The final plan's §10.3 operational runbook
  and on-call monitoring artifacts now have local entry points and
  doc-parity guards, but the paid-hosted launch gate remains open until
  the product/platform items in §10.1, §10.2, §10.4, and §10.5 have
  real evidence.
- **Completion status.** This local tree is now coherent and
  `make release-check` is green, but the objective cannot be marked
  complete because launch readiness still depends on external
  scheduled-cron, candidate-tag, repository-state, and product/legal
  evidence.

## Objective-to-artifact completion audit

The active objective is: make this repository launch-ready by using
`~/Downloads/review may 8/` as the source of truth, implementing safe
actionable findings, documenting human/legal/product gates, and keeping
the codebase, docs, tests, CI/release posture, and public-readiness
story coherent and verifiable.

| Objective requirement | Evidence in this tree | Current status |
| --- | --- | --- |
| Use the review folder as the source of truth. | Source inventory above names the four current review files. `scripts/check-launch-review-ledger.sh` pins every `T-*`, `MP-*`, `P*`, `D*`, `G-*`, and `L-*` finding ID from that source inventory, compares against the actual review folder when it is present, and runs through `make doc-parity`. | Locally satisfied; guarded against dropped reviewed IDs and stale pinned inventories. |
| Prioritize the highest-impact blockers first. | The closed section starts with the multi-tenant protocol/auth/security blockers from `01_MCP_PROTOCOL_TRANSPORTS.md` and `02_SECURITY_AUTH_TENANT_ISOLATION.md`: admission limits, initialize version mismatch, HTTP error envelopes, audit strict durability, OIDC hosted posture, gRPC hardening, and supply-chain workflow artifacts. | Locally satisfied for safe repository changes. |
| Fix safe actionable code findings. | Each safe `T-*`, `P1-*`, `P2-*`, and selected `P3-*` finding has a concrete disposition above, with either implementation notes and test names or an explicit no-action/deferred rationale. `T-03`, `T-20`, ADR 0017 Path A, and `D10` were already closed before this pass and are separated to avoid double-counting. | Locally satisfied for the review findings that do not require external approval or platform evidence. |
| Fix safe documentation, CI, and public-surface drift. | D1/D2/D4/D5/D6 are corrected locally; CodeQL, dependency-review, Semgrep, Go-version parity, doc-parity ADR status checks, stale tool-count guards, SECURITY.md scope coverage, premature official-claim guards, and the review-ledger guard are added or updated. | Locally satisfied; first GitHub runs for new workflows remain external evidence. |
| Leave human/legal/product approval items clearly documented. | External gates below name repository visibility/SLSA stance, GitHub repo description, issue #28, stale local branches, branch protection, pushed CodeQL/dependency-review/Semgrep evidence, main-branch freeze while Group 1 is pending, paid-commercial RLS, hosted global quota proof, NPM next-release proof, launch-candidate tracking issue creation, paid-hosted external security review, DPA / terms / privacy counsel review, trademark / official-language approval, and the `clockify://` URI plus gRPC service-name branding review. `scripts/check-launch-review-ledger.sh` fails closed if these concrete external gate entries disappear. | Open by design; these require maintainer, GitHub-platform, release-tag, hosted-infra, product, or legal action. |
| Keep tests and release posture verifiable. | Verification log below records targeted tests, doc/config parity, review-ledger parity, Go-version parity, `make verify-vuln`, `make launch-external-status`, `make public-content-audit`, `git diff --check`, and full `make release-check`. `docs/launch-candidate-checklist.md` still holds the binding Group 1/6/7 launch evidence boxes open until real evidence exists. | Locally green; not sufficient for launch-ready status. |
| Decide whether the objective is actually complete. | Completion requires no missing objective requirement. Group 1 scheduled final-SHA evidence, Group 6 candidate-tag security evidence, Group 7 release/sigstore/SLSA evidence, pushed workflow evidence, repository-state cleanup, hosted quota evidence, and legal/product approval are still missing. | **Not complete. Do not mark launch-ready.** |

## Prompt-to-artifact checklist

This checklist maps the prompt requirements, named source files,
commands, tests, gates, and deliverables to concrete current evidence.
It is intentionally separate from the long verification log so a future
agent can audit coverage without treating "many commands ran" as proof
that every requirement is covered.

| Prompt requirement / gate | Artifact or verifier | Concrete evidence inspected | Current status |
| --- | --- | --- | --- |
| Use `~/Downloads/review may 8/` as source of truth. | `00_COORDINATOR_INDEX.md`, `01_MCP_PROTOCOL_TRANSPORTS.md`, `02_SECURITY_AUTH_TENANT_ISOLATION.md`, `10_FINAL_INTEGRATED_LAUNCH_PLAN.md`; `scripts/check-launch-review-ledger.sh`. | The ledger lists all four source files. The verifier extracts `T-*`, `MP-*`, `P*`, `D*`, `G-*`, and `L-*` IDs from the actual source bundle when present and fails if a source ID is unpinned or a pinned ID is absent. | Locally covered and guarded. |
| Preserve every reviewed finding disposition. | `docs/launch-readiness-review-may-8.md`; `scripts/test-check-launch-review-ledger.sh`. | The verifier requires dispositions for `T-01`..`T-24`, `MP-01`..`MP-13`, `P1-1`..`P3-7`, `D1`..`D10`, `G-01`..`G-05`, `L-01`..`L-05`, `L-08`, and `L-10`, excluding the summary and verification sections. It also requires local dispositions for the un-IDed Appendix B open-question groups `B.03` through `B.08`, the coordinator's un-IDed "files not deeply read" specialist scope, and the final plan's §9/§10 checklist coverage. Regression tests cover missing IDs, unexpected IDs, Unicode hyphens, source-bundle drift, dropped Appendix B coverage, dropped coordinator file-scope coverage, and dropped final checklist coverage. | Locally covered and guarded. |
| Prioritize high-impact blockers before polish. | Closed finding order in this file; code areas under `internal/mcp/`, `internal/authn/`, `internal/config/`, `internal/controlplane/`, `internal/transport/grpc/`, and `internal/metrics/`. | The closed section starts with protocol/admission/audit/OIDC/gRPC/supply-chain blockers, then lower-risk docs, public-readiness, and evidence helpers. | Locally covered by disposition order. |
| Fix safe code findings. | Targeted unit/integration tests, pinned CI-lint/workflow-lint proof, plus `GOTOOLCHAIN=go1.25.10 make release-check`. | Release check passed on 2026-05-09 after the latest doc/security-evidence refresh; it includes coverage floors, script tests, config/doc parity, build-tag checks, HTTP/stdio smokes, strict doctor smoke, gRPC race E2E, and deploy render. The CI-pinned `golangci-lint` v2.5.0 command also ran locally via `go run` and reported `0 issues` after the final lint cleanup. The CI-pinned actionlint revision also ran locally against `.github/workflows/*.yml`. | Locally green. |
| Fix safe docs, CI, release, and public-surface drift. | `make doc-parity`; `bash scripts/test-check-doc-parity.sh`; `.github/workflows/codeql.yml`, `.github/workflows/dependency-review.yml`, `.github/dependabot.yml`; `.github/workflows/semgrep.yml`; `scripts/check-go-version-parity.sh`; `.github/workflows/ci.yml`; `tools/govulncheck`. | `doc-parity` passed, including the launch-review ledger, launch-checklist parity, and launch-evidence gate. The current doc-parity regression suite has 70 cases covering tool-count drift, public onboarding, ADR status, official-claim wording, brand/legal evidence, JSON Schema rationale, README/CONTRIBUTING local-verification wording, Makefile release-check wording, stale shippable release-check wording in docs, shared-service profile Group 2 scoping, production-readiness blocker-scope wording, gap-analysis blocker-scope wording, P3-5 baseline header docs, serverInfo identity guidance, default protocol-version guidance, May 8 ledger read-first routing, brand/legal URI plus gRPC service-name review docs, T-17 gRPC reflection dev-only posture, build-tag/tool-module Dependabot watcher coverage, root Dependabot build-tag ignore coverage, pinned verify-vuln tool-module execution, govulncheck CI version proof, SUPPORT.md SLSA private-repo cosign fallback, stale unconditional SLSA public wording, release-smoke SLSA bare-404 skip guard, README SLSA provenance availability wording, workflow action SHA-pin guard, deploy SLSA bare-404 skip guard, release workflow/docs SLSA availability wording, release-smoke doctor-output artifact guard, docker-image SLSA feature-gate notice guard, legacy HTTP EOL runbook, stale public-content local-artifact wording, stale shared-service launch-blocking wording, agent handoff permissioned landing sequence, and dependency-review default-branch evidence trigger. The RC evidence regression suite also now guards that raw workflow snapshots are not treated as final-SHA proof. | Locally green; workflow first-run evidence still external. |
| Keep Group 6 security posture verifiable. | `docs/launch-candidate-checklist.md`; `docs/runbooks/release-candidate-evidence.md`; `scripts/prepare-rc-evidence.sh`. | Current local preflight: pinned `govulncheck@v1.3.0` found no vulnerabilities under `GOTOOLCHAIN=go1.25.10`, Semgrep `p/default` scanned 1153 tracked files with 0 findings, `nosemgrep` context still maps to ADR 0008 / ADR 0017, and `make verify-fips` passed. A host-toolchain govulncheck scan with Go 1.26.2 reports standard-library issues fixed in Go 1.26.3, so README/CONTRIBUTING now avoid broad `1.25.10+` support wording and keep the exact Go 1.25.10 launch-candidate pin. `make secret-scan` is not green on this dirty workstation because ignored/local artifacts remain; clean candidate-tag gitleaks remains required. | Locally documented; final candidate-tag evidence open. |
| Keep CI/release/external state honest. | `make launch-external-status`; `docs/launch-candidate-checklist.md`; `scripts/check-launch-evidence-gate.sh`. | Latest read-only snapshot reports `11 open, 0 unknown`: dirty remediation tree, non-main local branches, missing final-SHA live/mutation cron evidence, workflow first-runs not on the final candidate SHA, private-repo branch-protection API limitation, stale repo description, issue #28 open, and missing next-release npm expected-version proof. The helper now directly verifies `CLOCKIFY_LIVE_AUDIT_REQUIRED=true`, verifies live-contract cron log markers including `TestLiveCreateUpdateDeleteEntryAuditPhases` and `TestLiveReadSideSchemaDiff` before Group 1 can close, fails open if readable branch protection omits `Doctor strict smoke`, `Doctor Postgres backend`, or `Shared-service Postgres E2E` from either GitHub required-check API shape, and rejects stale/PR-only CodeQL/dependency-review/Semgrep runs as launch evidence. The RC evidence bundle keeps raw workflow metadata for audit context, while `check-launch-external-status --fail-open` remains the fail-closed final-SHA validator. | Open external/repo-state gates. |
| Keep public-readiness story honest. | `make public-content-audit`; `scripts/check-public-content-audit.sh`; `docs/release/public-history-review.md`; `docs/release/local-artifact-review.md`. | Latest read-only snapshot reports `0 open, 0 unknown`: candidate branch file content is `0 open, 0 unknown`, public-history review is `0 open, 0 unknown`, and local artifact/full-tree review is `0 open, 0 unknown`. | Public-content audit clean locally; public flip still requires external, repo-state, and legal/product gates. |
| Leave human/legal/product approvals documented, not guessed. | `docs/release/brand-legal-review.md`; `make license-evidence`; `scripts/collect-license-evidence.sh`. | License helper produced a raw build-variant dependency/license-candidate inventory with 0 modules missing local license candidates and 0 unknown variants, but it is explicitly not legal advice or license clearance. Trademark/official-product wording, `clockify://`, and gRPC service-name branding review remain written approval or rebrand decisions. | Evidence input exists; legal/product approval open. |
| Decide completion from real evidence only. | This checklist; `make launch-external-status`; `make public-content-audit`; `git status --short --branch`. | The tree is locally coherent and release-check green, but the current branch is dirty/uncommitted, external launch gates remain open, and legal/product approval remains incomplete. | **Not complete. Do not mark launch-ready.** |

## Deferred low-risk follow-ups

- **Security P3 findings.** P3-1 and P3-2 are correct-by-spec
  cryptographic implementation choices; no action. P3-4
  (multi-region live-contract cron) depends on whether supported
  contracts become region-specific.

## External evidence or approval gates

Use `make launch-external-status` for a read-only snapshot of the
GitHub/npm/local-branch/open-PR-facing items below. The command is
diagnostic only; it does not mutate GitHub, delete branches, label PRs,
or close any checklist boxes. Rechecked on 2026-05-09 after stale-PR
hygiene was added: the helper reports the still-open external gates and
prints a specific maintainer action beside each open gate.

- **Group 1 scheduled live-contract evidence.** Still requires two
  consecutive scheduled cron greens on the final candidate SHA
  (`L-02`). Local or manual-dispatch greens are useful debug evidence
  only. Rechecked on 2026-05-09: scheduled runs 25593042387 and
  25538247771 are green on
  `4fe957547f9e6aea749a85f87823d17a0ccc2928` and include the required
  mutating, audit, and schema-diff log markers, but that SHA is still
  not current `origin/main` (`2e7b6bd4a7968ba45921e103d948f74dd82175b8`)
  or this dirty local remediation tree. `make launch-external-status`
  therefore keeps Group 1 open unless the final remediation SHA is
  explicit or the tree is clean.
- **Main freeze while Group 1 is pending.** The coordinator's Day 0
  plan says to freeze `main` until Group 1 closes after the
  remediation tree lands. This pass documents the operator
  coordination rule but does not change branch protection, push
  permissions, or any remote settings.
- **Group 6 candidate-tag security walk-through.** Still requires the
  final candidate tag and evidence for `make verify-vuln`,
  `make verify-fips`, gitleaks, and semgrep/no-finding disposition.
  This is `L-04`.
- **Group 7 release/sigstore/SLSA evidence.** Still requires an
  actual `vX.Y.Z-rc.N` tag (`L-01`), release smoke, cosign/SLSA
  verification, and archived `doctor --strict` output.
- **Launch-candidate tracking issue.** The coordinator's Day 2 plan
  requires opening `Launch candidate vX.Y.Z-rc.N` after the rc exists
  and linking every green workflow run, archived `doctor --strict`
  output, the `SECURITY.md` walk-through, and the `release-smoke.yml`
  URL before any "launch candidate ready" report. This is externally
  visible issue creation and remains approval-gated; this pass only
  preserves it in the handoff and fail-closed ledger.
- **D3 SLSA/private-repo stance.** The user-owned private repository
  attestation limitation is a maintainer/repo-visibility decision.
  Do not remove best-effort handling or claim official SLSA readiness
  without evidence on the final public/private posture. This is `L-05`.
- **D1 / D2 GitHub repository description drift.** Rechecked on 2026-05-09:
  `gh repo view apet97/go-clockify --json description,visibility`
  still reports a private repository and stale tool-count /
  policy-count wording in the repository description. Updating the
  description to `128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.`
  is an externally visible maintainer action and the remaining external
  part of `L-03`.
- **P1-3 SLSA fail-closed posture.** Dropping `continue-on-error` on
  release attestations depends on the private-repo/platform stance and
  must be proven on an rc tag before Group 7 closes.
- **NPM publish path on the next release.** The current package exists:
  `npm view @apet97/clockify-mcp-go version dist-tags --json` returned
  `1.2.0`, and `npx -y @apet97/clockify-mcp-go --version` printed
  `v1.2.0` on 2026-05-08. The remaining launch gate is proof that the
  next rc/release path still has `NPM_TOKEN` configured and publishes
  the new version; the release-candidate evidence runbook now includes
  the expected-version `check-launch-external-status` command for that
  proof. This is `G-01`.
- **D7 / D9 branch protection, mutation cron, and stale local branches.**
  Branch protection audit is currently blocked by GitHub's private-repo
  protection API response (`Upgrade to GitHub Pro or make this
  repository public`); `make launch-external-status` now records that
  response as an open D9 item instead of relying on prose only. If the
  branch-protection API becomes readable, the helper now also fails
  open unless the D9 launch required checks include
  `Doctor strict smoke`, `Doctor Postgres backend`, and
  `Shared-service Postgres E2E`, whether GitHub returns them through
  the classic `required_status_checks.contexts` array or the newer
  `required_status_checks.checks[].context` objects.
  `scripts/audit-branch-protection.sh` now fails clearly for that
  private-repo limitation and avoids emitting a null-field
  pseudo-snapshot. Seven
  non-main local branches remain on this workstation and require
  maintainer disposition: `codex/resolve-benchmark-worktree`,
  `docs/document-f3897b2-bypass`, `fwbranch`,
  `stabilize/quality-perf`, `wave-a`, `wave-d`, and `wave-e`. The
  helper reports them without deleting or pushing branches, and now
  includes each branch head plus ahead/behind counts against
  `origin/main`, plus an explicit action to review, merge, archive, or
  delete branches only after maintainer approval. Rechecked on
  2026-05-09: only
  `docs/document-f3897b2-bypass` (`ahead=1`) and `fwbranch`
  (`ahead=20`) still contain commits ahead of `origin/main`; the other
  five are stale local pointers with `ahead=0`. The mutation cron
  timeout has a local workflow fix in
  this pass, but still needs the next scheduled run as evidence.
  Rechecked on 2026-05-09 via `make launch-external-status`: latest
  `mutation.yml` scheduled run 25592823559 is now
  `completed/cancelled` on pushed commit
  `4fe957547f9e6aea749a85f87823d17a0ccc2928`. `gh run view` shows the
  `internal/tools` matrix leg was cancelled while the other mutation
  legs succeeded. `make launch-external-status` now also prints that
  non-green mutation matrix job and tells maintainers to land the
  local timeout fix before waiting for the next scheduled success, so
  handoffs do not need a separate `gh run view` inspection. These are
  the open external pieces of
  `G-02`, `G-03`, and `G-04`; the local timeout increase still needs
  to land and prove a scheduled run on the final candidate SHA.
- **Public-repo stale PR hygiene.** The final integrated launch plan
  requires no open PRs older than 14 days unless they carry a `wip` or
  `blocked` label. `make launch-external-status` now checks the first
  100 open PRs read-only, reports stale unlabeled PRs with action
  hints, and treats parse/API failures as unknown rather than silently
  passing the public-reopen gate.
- **D8 issue #28 stale.** Rechecked on 2026-05-09:
  `gh issue view 28 --repo apet97/go-clockify --json state,title,url`
  still reports the "Postgres-backed shared-service integration test"
  issue open even though Group 2 is locally documented as closed.
  Closing it is externally visible and should reference the Group 2
  closure commits or checklist evidence. This is `G-05`.
- **P2-5 dependency-review first-run evidence.** The workflow file
  exists locally, but launch evidence still needs a pushed `main` run
  or PR run showing the dependency-review gate executes with the
  repository's permissions.
  Rechecked on 2026-05-09: `gh run list --workflow=dependency-review.yml`
  still returns `HTTP 404: workflow dependency-review.yml not found on
  the default branch`, which is expected until this remediation tree is
  pushed.
- **P1-4 CodeQL first-run evidence.** The workflow file exists locally,
  but the launch record still needs a successful GitHub run and visible
  SARIF upload under the repository's Security tab. Rechecked on
  2026-05-09: `gh run list --workflow=codeql.yml` still returns
  `HTTP 404: workflow codeql.yml not found on the default branch`,
  which is expected until this remediation tree is pushed.
- **Semgrep first-run evidence.** The workflow file exists locally, but
  launch evidence still needs a successful GitHub run under the
  repository's Actions tab. Candidate-tag Group 6 Semgrep evidence is
  still separate. Rechecked on 2026-05-09:
  `gh run list --workflow=semgrep.yml` still returns `HTTP 404:
  workflow semgrep.yml not found on the default branch`, which is
  expected until this remediation tree is pushed.
- **P1-8 paid-commercial RLS decision.** The v1.x app-layer scoping
  posture is documented, but implementing database-enforced RLS still
  requires a product/commercial design decision and tenant-context
  plumbing through the Postgres store API.
- **Cross-replica hosted HTTP quotas.** The in-process
  `MCP_HTTP_RATELIMIT_*` guards reject obvious abuse on each pod, but
  an official hosted plane still needs gateway/load-balancer evidence
  for global source/principal quotas.
- **Paid-hosted external security review.** The final integrated plan
  requires at least one third-party or peer security review recorded
  in `SECURITY.md` against the candidate tag before a paid hosted
  launch. Local tests, internal review, and candidate-tag Group 6
  scans are necessary evidence, not a substitute for this external
  review gate.
- **DPA / terms / privacy posture.** The paid-hosted checklist requires
  customer terms / DPA and privacy / data-handling review by counsel,
  including `docs/auth-model.md` and the credential-leak response
  posture. This is explicitly out of scope for local code changes and
  remains a legal/commercial approval gate.
- **Trademark / "official Clockify" language.** This needs legal or
  product approval before any public promotion language claims official
  Clockify status. Safe local wording changes now frame the work as a
  launch-candidate evidence/review pass rather than an approved
  official-product claim, and `doc-parity` fails closed if the
  riskiest official-launch-candidate phrasing returns. This
  does not close `L-10`; `docs/release/brand-legal-review.md` now
  frames the reviewer questions, but written legal/product approval or
  a final rebrand decision is still required.
- **Clockify URI scheme and gRPC service-name branding review.**
  `clockify://` resource templates and the opt-in
  `clockify.mcp.v1.MCP` gRPC service name remain part of the same
  brand/legal approval or rebrand decision. The May 8 transport review
  flagged both before public client dependency, and no local test or
  doc cleanup closes the question.
- **Public repo content audit before visibility flip.** `make
  public-content-audit` is now the read-only local snapshot for the
  final integrated plan's public-flip content audit. Rechecked on
  2026-05-09 after neutralizing tracked workstation scratch references,
  documenting benign public-history keyword matches, and documenting
  ignored local artifacts: it reports `0 open, 0 unknown`.
  `Candidate branch file content: 0 open, 0 unknown`.
  `Public history review: 0 open, 0 unknown`.
  `Local artifact/full-tree review: 0 open, 0 unknown`.
  The candidate branch-content gitleaks scan is closed. The candidate branch-content TLS verification bypass marker check is closed. The candidate branch-content MIT LICENSE check is closed. The candidate branch-content .gitignore coverage check is closed. The candidate branch-content .gitleaks.toml allowlist description check is closed. The candidate branch-content CLAUDE.md workstation context check is closed. The candidate branch-content live Clockify secret assignment check is closed. The candidate branch-content env-like file check is closed.
  The tracked personal/scratch grep and tracked Go/Markdown
  internal/private task-marker check are closed. The recent commit
  message sensitive-word matches are documented in
  `docs/release/public-history-review.md` as false positives, and the
  ignored full-tree findings are documented in
  `docs/release/local-artifact-review.md`. This closes the local
  public-content bucket only; public visibility still depends on the
  external, repository-state, and legal/product gates above.

## Verification used for this pass

- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }'
  .github/workflows/docker-image.yml .github/workflows/release.yml
  .github/workflows/deploy.yml .github/workflows/release-smoke.yml`
- `bash scripts/test-check-doc-parity.sh` (70/70 OK, including the
  workflow action SHA-pin, deploy SLSA bare-404 skip, and release
  workflow/docs SLSA availability wording, and agent handoff
  permissioned landing sequence plus dependency-review default-branch
  evidence, root Dependabot build-tag ignore coverage,
  release-smoke doctor-output artifact, and docker-image SLSA
  feature-gate notice guards)
- `bash scripts/test-check-launch-review-ledger.sh` (39/39 OK,
  including the 69-case prompt-to-artifact guard and
  launch-candidate tracking issue, main-freeze, external security
  review, and DPA / privacy counsel gates)
- `make doc-parity`
- `git diff --check`
- `GOBIN=<tmp> GOTOOLCHAIN=go1.25.10 go install
  github.com/rhysd/actionlint/cmd/actionlint@914e7df21a07ef503a81201c76d2b11c789d3fca`
  followed by `<tmp>/actionlint -color=false .github/workflows/*.yml`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open; `CLOCKIFY_LIVE_AUDIT_REQUIRED`
  is directly checked and closed as `true`)
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path, while the pinned actionlint command
  above separately covered workflow lint; the latest run covered
  coverage floors with total coverage 79.6%, config/doc/catalog/gRPC
  release parity, repo hygiene, script tests, Go-version parity,
  build-tag wiring, HTTP and stdio smokes, strict doctor smoke,
  gRPC-tagged race E2E tests, Kustomize render, Helm lint, and Helm
  template validation)
- `make shellcheck` (local `shellcheck` installed; all `scripts/*.sh`
  clean)
- `make script-tests` (all script regression suites passed, including
  doc parity 70/70, launch-review ledger 39/39,
  launch-external-status 20 run / 0 failed, public-content audit
  6 run / 0 failed, Go-version parity 9/9, live-tool coverage
  6 run / 0 failed, license evidence 3/3, and RC evidence
  4 run / 0 failed with raw-snapshot-vs-validator assertions)
- `make bench-baseline-check` (committed
  `internal/benchdata/baseline.txt` passed; baseline is Linux/amd64
  with the required 10 samples per benchmark)
- `make rc-evidence-plan TAG=v1.2.1-rc.1` (printed the expected
  Group 6/7 evidence plan, including the warning that workflow
  snapshots are audit metadata and `check-launch-external-status
  --fail-open` is the final-SHA validator; no tag was cut and no
  release evidence was collected)
- `GOTOOLCHAIN=go1.25.10 make verify-vuln verify-fips`
  (`govulncheck@v1.3.0` reported no vulnerabilities against the
  2026-05-07 vuln DB snapshot, and FIPS-tagged tests passed)
- `semgrep scan --config p/default --metrics=off --error --exclude
  .git --exclude .bench --exclude clockify-mcp .` (scanned 1153
  tracked files, 0 findings)
- `semgrep scan --config p/default --metrics=off --error
  .github/workflows/semgrep.yml` (0 findings)
- `bash scripts/check-live-tool-coverage.sh` (0 open, 0 unknown; all
  88 Tier-2 tools and all API-backed Tier-1 tools are named in live
  E2E source, the four local-only Tier-1 helpers are explicitly
  allowed, and no unknown `clockify_*` live-test references exist)
- `bash scripts/check-launch-external-status.sh --fail-open` (exits
  nonzero with 11 open, 0 unknown on the current dirty tree, so
  launch-ready claims fail closed while external gates remain open)
- `bash scripts/check-public-content-audit.sh --fail-open` (exits 0
  with 0 open, 0 unknown; candidate branch file content, public-history
  review, and local-artifact/full-tree review all clean)
- Generator idempotence check: reran `go run ./cmd/gen-config-docs
  -mode=all` and `make gen-tool-catalog`, then compared working-tree
  checksums for `cmd/clockify-mcp/help_generated.go`, `README.md`, and
  `docs/tool-catalog.{json,md}`; no changes.
- `make config-doc-parity catalog-drift doc-parity`
- Candidate Markdown link check over tracked plus unignored Markdown
  files: 99 files, no missing relative targets. Offline `lychee`
  reported 390 total, 352 OK, 0 errors, 38 excluded.
- Staging-manifest audit over 198 candidate paths: no nested checkout,
  local state, review scratch, benchmark/coverage artifact, macOS
  artifact, sensitive-extension file, or uncategorized path in the
  candidate set. The only env-like candidate path is the intentional
  deletion of `.env.example`. Current bucket counts:
  `.github/ISSUE_TEMPLATE` 3, `.github/codeql` 1,
  `.github/workflows` 14, root policy/build docs 10 including
  `.env.example`, `cmd` 5, `deploy` 11, `docs` 46, `internal` 70,
  `scripts` 26, `tests` 6, `tools` 3, plus `go.mod` and `go.work`.
- Post-`release-check` artifact hygiene: `git ls-files -o
  --exclude-standard` showed 42 intended untracked commit-candidate
  files, no unignored generated/local artifact candidates, and
  `git check-ignore -v` confirmed `.DS_Store`, `.bench/`,
  `coverage.out`, and the local loop-artifact directory are ignored by
  `.gitignore`.
- `bash scripts/collect-license-evidence.sh --fail-missing-license`
  (0 modules without local license candidates, 0 unknown variants; raw
  evidence input only, not legal advice or license clearance)
- `go test ./internal/mcp -run 'TestStreamable.*ProtocolVersion|TestMCP.*CORS|TestLegacyHTTPDeprecationHeaders|TestMetricsMuxAuth' -count=1`
- `go test ./internal/mcp -run 'TestAuditDurability|TestAuditPhase' -count=1`
- `go test ./internal/authn -run 'TestBearerTokenParsing|TestOIDCAuthenticator_RequireKID' -count=1`
- `go test ./internal/config -run 'TestOIDCVerifyCacheTTL|TestProfile_.*Strict|TestLoad_ForwardAuthRequiresTrustedProxiesOnNonLoopback|TestProfile_HelperCoversAllProfileKeys' -count=1`
- `go test ./internal/config -run 'TestAuditDurability|TestGeneratedDefaults|TestSpec|TestProfile_HelperCoversAllProfileKeys' -count=1`
- `go test ./internal/runtime -run TestStreamableHTTPMinTLSVersion -count=1`
- `go test ./internal/mcp ./internal/authn ./internal/config ./internal/runtime -count=1`
- `go test ./tests -run 'TestParity|TestStreamable|TestSSE|TestHTTP' -count=1`
- `make check`
- `make config-parity`
- `make doc-parity`
- `make launch-checklist-parity`
- `make config-doc-parity`
- `git diff --check`
- `make release-check`
- `go test ./internal/mcp -run 'TestHTTPAdmission|TestStreamableHTTPAdmission' -count=1`
- `go test ./internal/config -run 'TestLoadHTTPAdmission|TestProfile_.*Strict|TestProfile_HelperCoversAllProfileKeys|TestEnvSpec' -count=1`
- `go test ./internal/config -run 'TestLoadHTTPAdmission|TestProfile_.*Strict|TestProfile_HelperCoversAllProfileKeys|TestEnvSpec|TestGeneratedDefaults|TestSpec' -count=1`
- `go test ./internal/runtime ./internal/metrics -count=1`
- `go test ./internal/mcp ./internal/config ./internal/runtime ./internal/metrics -count=1`
- `make config-parity`
- `make config-doc-parity`
- `make doc-parity`
- `go test ./internal/mcp -run TestHTTPTransportAdmissionErrorsUseJSONRPCEnvelope -count=1` (first run failed against the old plain JSON body shape)
- `go test ./internal/mcp -run 'TestHTTPTransportAdmissionErrorsUseJSONRPCEnvelope|TestUnauthorizedDoesNotExposeOIDCDetailsByDefault|TestUnauthorizedCanExposeDetailsWhenExplicitlyEnabled|TestStreamableProtocolVersionRequired|TestStreamableSSE_OriginRejected|TestMCPCORSBlocked|TestHTTPAdmission' -count=1`
- `go test ./internal/authn -run TestWriteUnauthorized -count=1`
- `go test ./internal/mcp -count=1`
- `go test ./tests/harness ./tests -run 'TestSizeLimit|TestHTTP|TestStreamable|TestParity' -count=1`
- `go test ./tests -count=1`
- `go test -tags=grpc ./tests -run 'TestParity|TestStreamable|TestSSE|TestHTTP|TestListChanged_ParityAcrossTransports|TestCancellation' -count=1`
- `make config-parity`
- `make config-doc-parity`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `go test ./cmd/clockify-mcp`
- `bash scripts/smoke-doctor-strict.sh`
- `go test ./internal/authn -run TestDecodeJWT -count=1`
- `go test ./internal/authn -count=1`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `python3` Docker Registry probe for `semgrep/semgrep:1.162.0` (returned manifest digest `sha256:9349edbadf90c3f3c0c3f55867625354e89680e6fa10d9034042af52fdb0e0d0`)
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }' .github/workflows/semgrep.yml .github/workflows/codeql.yml .github/workflows/dependency-review.yml`
- `semgrep scan --config p/default --metrics=off --error --exclude .git --exclude .bench --exclude clockify-mcp .` (0 findings)
- `semgrep scan --config p/default --metrics=off --error .github/workflows/semgrep.yml` (0 findings)
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest -color=false .github/workflows/semgrep.yml`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `bash scripts/test-check-doc-parity.sh` (19/19 OK, including ADR status drift cases)
- `bash scripts/check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (6/6 OK)
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `ruby -ryaml -e 'YAML.load_file(ARGV.fetch(0)); puts "ok"' .github/workflows/mutation.yml`
- `make doc-parity`
- `git diff --check`
- `gh repo view apet97/go-clockify --json description,visibility,url`
- `gh issue view 28 --repo apet97/go-clockify --json number,title,state,url,closed,closedAt`
- `bash scripts/audit-branch-protection.sh` (blocked by GitHub private-repo protection API limitation)
- `gh run list --repo apet97/go-clockify --workflow=mutation.yml --limit=10 --json databaseId,status,conclusion,event,createdAt,headSha,displayTitle`
- `gh run view 25537573499 --repo apet97/go-clockify --json databaseId,status,conclusion,createdAt,updatedAt,event,headSha,jobs`
- `gh run view 25477911218 --repo apet97/go-clockify --json databaseId,status,conclusion,createdAt,updatedAt,event,headSha,jobs`
- `npm view @apet97/clockify-mcp-go version dist-tags --json`
- `npx -y @apet97/clockify-mcp-go --version`
- `go test ./internal/mcp -run 'TestStreamable|TestSSE' -count=1`
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }' deploy/monitoring/prometheus-alerts.yaml deploy/k8s/base/prometheus-rule.yaml`
- `make verify-k8s`
- `go test ./internal/mcp -run 'TestStreamableSessionStorePersistenceFailures|TestProtocolVersion|TestStreamable|TestSSE' -count=1`
- `rg -n "session_id" internal/mcp/transport_streamable_http.go internal/mcp/transport_streamable_http_test.go`
- `go test ./internal/logging ./internal/mcp ./cmd/clockify-mcp -count=1`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }' .github/workflows/codeql.yml .github/codeql/codeql-config.yml`
- `gh api repos/actions/dependency-review-action/git/matching-refs/tags/v4 --jq '.[-1].ref + " " + .[-1].object.sha'`
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }' .github/workflows/dependency-review.yml`
- `make doc-parity`
- `git diff --check`
- `make doc-parity`
- `git diff --check`
- `go test ./internal/logging -run TestRedactingHandlerBoundaryKeyMatchingOptIn -count=1` (first run failed because the opt-in boundary API did not exist)
- `go test ./internal/logging -run 'TestRedactingHandlerBoundaryKeyMatchingOptIn|TestRedactingHandlerCaseInsensitiveSubstringMatching|TestRedactingHandlerPreservesAuthModeAndTTL|TestRedactingHandlerScrubsSecretShapedStringValues' -count=1`
- `git diff --check`
- `go test -tags=grpc ./internal/transport/grpc -run 'TestStreamNotifierDropsSlowConsumer|TestServeMarksNotReadyBeforeGracefulDrain' -count=1`
- `go test ./cmd/clockify-mcp ./internal/config ./internal/runtime -run 'TestValidateBuildCapabilities|TestDoctorReportsMissingGRPCBuildTag|TestLoadTransportGRPC|TestLoadGRPCReauthInterval|TestProfile_PrivateNetworkGRPC' -count=1`
- `go test -tags=grpc ./internal/transport/grpc -run 'TestPlaintextGRPCNonLoopbackDetection|TestStreamNotifierDropsSlowConsumer|TestServeMarksNotReadyBeforeGracefulDrain' -count=1`
- `go test -tags=grpc ./cmd/clockify-mcp ./internal/runtime -count=1`
- `make release-check`
- `go test ./internal/mcp -run TestStreamSessionManagerConcurrentRehydrationCoalesces -count=1`
- `go test -tags=grpc ./tests -run TestListChanged_ParityAcrossTransports -count=1`
- `go test ./internal/mcp -run 'TestRepeatInitialize|TestDispatchToolsListSerializedCache|TestRunToolsListUsesSerializedCache|TestCancellation' -count=1`
- `go test ./internal/mcp -run TestStreamableHTTP_CrossPodCancel_IsBestEffort -count=1`
- `go test ./internal/mcp -count=1`
- `go test ./tests -count=1`
- `go test -tags=grpc ./tests -run 'TestListChanged_ParityAcrossTransports|TestCancellation|TestStreamable|TestParity' -count=1`
- `git diff --check`
- `make release-check`
- `go test ./internal/vault -run 'TestResolve|TestDecodeMaterial' -count=1`
- `go test ./internal/mcp -run 'TestStreamableCORSPreflightAllowsResumeHeaders|TestMCP.*CORS|TestStreamableProtocolVersion|TestStreamableInitializeProtocolVersionHeaderMismatch' -count=1`
- `go test ./internal/mcp ./internal/metrics -run 'TestStreamableEventsBackCompatAlias|TestStreamableCORSPreflightAllowsResumeHeaders|TestMetricsRender|TestDefaultMetricsRegistered' -count=1`
- `go test ./internal/mcp -run 'TestSanitizePanicStackScrubsPathsAndTruncates|TestRecoverDispatch|TestHandleWithRecover|TestObserveHTTPH' -count=1`
- `go test ./internal/logging -count=1`
- `go test ./internal/mcp -run 'TestStreamableProtocolVersionAbsent|TestStreamableProtocolVersionRequired|TestStreamableProtocolVersionMismatch|TestStreamableInitializeProtocolVersionHeaderMismatch|TestStreamableEventsBackCompatAlias' -count=1`
- `go test ./internal/config -run 'TestLoadHTTPRequireProtocolVersion|TestLoadHTTPAdmissionLimits|TestEnvSpec|TestGeneratedDefaults|TestSpec|TestTransportAuthMatrix' -count=1`
- `go test ./internal/metrics ./internal/runtime -count=1`
- `go run ./cmd/gen-config-docs -mode=all`
- `make config-parity`
- `make config-doc-parity`
- `make doc-parity`
- `go test -tags=grpc ./internal/transport/grpc -run 'TestAuthInterceptor_PeerCIDRAllow|TestAuthUnaryInterceptor_PeerCIDRAllow' -count=1` (first run failed because `peerCIDRAllow` was not implemented)
- `go test ./internal/config -run 'TestLoadGRPCPeerCIDRAllow' -count=1` (first run failed because `Config.GRPCPeerCIDRAllow` was not implemented)
- `go test -tags=grpc ./internal/transport/grpc -run 'TestAuthInterceptor_PeerCIDRAllow|TestAuthUnaryInterceptor_PeerCIDRAllow|TestPlaintextGRPCNonLoopbackDetection|TestStreamNotifierDropsSlowConsumer|TestServeMarksNotReadyBeforeGracefulDrain' -count=1`
- `go test ./internal/config -run 'TestLoadGRPCPeerCIDRAllow|TestLoadGRPCReauthInterval|TestEnvSpec|TestGeneratedDefaults|TestSpec|TestTransportAuthMatrix' -count=1`
- `go test -tags=grpc ./internal/runtime ./internal/transport/grpc -count=1`
- `go run ./cmd/gen-config-docs -mode=all`
- `make config-parity`
- `make config-doc-parity`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `go test ./internal/controlplane -run TestSessionRecordDoesNotExposeSessionAffinityID -count=1` (first run failed because `SessionRecord` still exposed `SessionAffinityID`)
- `go test -tags=postgres ./internal/controlplane/postgres -run TestMigrationsDropSessionAffinityColumn -count=1` (first run failed because migration 003 was missing)
- `go test ./internal/controlplane -run 'TestSessionRecordDoesNotExposeSessionAffinityID|TestOpenFileDSNRoundTrip' -count=1`
- `go test -tags=postgres ./internal/controlplane/postgres -run TestMigrationsDropSessionAffinityColumn -count=1`
- `go test ./internal/controlplane ./internal/mcp -count=1`
- `make build-postgres`
- `make test-postgres` (blocked locally: Docker/Testcontainers could not connect to `/var/run/docker.sock`, Colima, or Docker Desktop socket)
- `make shared-service-e2e` (skipped locally because
  `MCP_LIVE_CONTROL_PLANE_DSN` was unset)
- `colima start`
- `docker info --format '{{.ServerVersion}} {{.OperatingSystem}} {{.Architecture}}'`
  (reported Docker 29.2.0 on Ubuntu 24.04.3 LTS aarch64)
- `make test-postgres` (passed after starting Colima; Testcontainers
  exercised the Postgres control-plane package with
  `INTEGRATION_REQUIRED=1`)
- `make build-postgres build-grpc-postgres` (passed; the expected
  ignored `clockify-mcp` local binary was then removed with
  `make clean`)
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `bash scripts/test-check-doc-parity.sh`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `comm -23 <(rg -o --no-filename "\bT[-‑][0-9]{2}\b" ~/Downloads/'review may 8'/01_MCP_PROTOCOL_TRANSPORTS.md | tr '‑' '-' | sort -V -u) <(rg -o --no-filename "\b(T[-‑][0-9]{2}|P[123]-[0-9]+|D[0-9]+)\b" docs/launch-readiness-review-may-8.md | tr '‑' '-' | sort -V -u)` (no missing transport IDs)
- `comm -23 <(rg -o --no-filename "\bP[123]-[0-9]+\b" ~/Downloads/'review may 8'/02_SECURITY_AUTH_TENANT_ISOLATION.md | sort -V -u) <(rg -o --no-filename "\b(T[-‑][0-9]{2}|P[123]-[0-9]+|D[0-9]+)\b" docs/launch-readiness-review-may-8.md | tr '‑' '-' | sort -V -u)` (no missing security IDs)
- `comm -23 <(rg -o --no-filename "\bD[0-9]+\b" ~/Downloads/'review may 8'/00_COORDINATOR_INDEX.md | sort -V -u) <(rg -o --no-filename "\b(T[-‑][0-9]{2}|P[123]-[0-9]+|D[0-9]+)\b" docs/launch-readiness-review-may-8.md | tr '‑' '-' | sort -V -u)` (no missing coordinator drift IDs)
- `gh run list --repo apet97/go-clockify --workflow=live-contract.yml --limit=8 --json databaseId,status,conclusion,event,createdAt,headSha,displayTitle,url`
- `gh run list --repo apet97/go-clockify --workflow=mutation.yml --limit=8 --json databaseId,status,conclusion,event,createdAt,headSha,displayTitle,url`
- `gh issue view 28 --repo apet97/go-clockify --json number,title,state,url,closed,closedAt`
- `gh repo view apet97/go-clockify --json description,visibility,url`
- `bash scripts/check-launch-external-status.sh --plan`
- `bash scripts/test-check-launch-external-status.sh` (14 offline-stub cases)
- `make launch-external-status`
- `bash scripts/check-public-content-audit.sh --plan`
- `bash scripts/test-check-public-content-audit.sh` (3 offline-stub cases,
  including the candidate branch-content TLS verification bypass marker
  check, the candidate branch-content MIT LICENSE check, and the
  candidate branch-content .gitignore coverage and `.gitleaks.toml`
  allowlist description checks, plus the candidate branch-content
  `CLAUDE.md` workstation context and live Clockify secret assignment
  checks)
- `make public-content-audit`
- `bash scripts/check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (12/12 OK, including
  `10_FINAL_INTEGRATED_LAUNCH_PLAN.md` and `G-*` / `L-*` guard cases)
- `gh variable list --repo apet97/go-clockify --json name,value,updatedAt --jq '.[] | select(.name=="CLOCKIFY_LIVE_AUDIT_REQUIRED") | [.name,.value,.updatedAt] | @tsv'`
- `gh run view 25538247771 --repo apet97/go-clockify --log | sed -n '228,280p'`
- `go list -m -versions golang.org/x/vuln`
- `go list -m -json golang.org/x/vuln@v1.3.0`
- `gh api repos/golang/vuln/git/ref/tags/v1.3.0 --jq '{ref:.ref, object:.object}'`
- `GOTOOLCHAIN=go1.25.9 govulncheck@v1.3.0 ./...` (failed before the
  patch with GO-2026-4971 and GO-2026-4918 fixed in Go 1.25.10)
- `go test ./internal/mcp -run TestDispatchToolsListSerializedCacheStillRequiresInitialize -count=1`
- `bash -n scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (16/16 OK, including
  `MP-*` parity-matrix guard coverage)
- `bash -n scripts/check-doc-parity.sh scripts/test-check-doc-parity.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh scripts/test-check-doc-parity.sh`
- `bash scripts/test-check-doc-parity.sh` (26/26 OK, including public
  onboarding SECURITY/SUPPORT/CODEOWNERS guard coverage)
- `bash scripts/test-check-doc-parity.sh` (27/27 OK, including the
  CONTRIBUTING.md public HTTPS clone guard)
- `GOTOOLCHAIN=go1.25.10 govulncheck@v1.3.0 ./...`
- `docker buildx imagetools inspect golang:1.25-bookworm`
- `make doc-parity`
- `git diff --check`
- `make release-check`
- `perl -CSDA -ne 's/[\x{2010}\x{2011}\x{2012}\x{2013}\x{2014}]/-/g; while(/\b(T-[0-9]{2}|MP-[0-9]{2}|P[123]-[0-9]+|D[0-9]+|G-[0-9]{2}|L-[0-9]{2})\b/g){print "$1\n"}' ~/Downloads/'review may 8'/*.md | sort -u` (confirmed `L-08` appears in the source bundle)
- `bash -n scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (17/17 OK, including
  the `L-08` source-alias guard)
- `bash scripts/check-launch-review-ledger.sh`
- `bash -n scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (19/19 OK, including
  actual-source-bundle drift cases for unpinned source IDs and stale
  pinned IDs)
- `bash scripts/check-launch-review-ledger.sh`
- `bash -n scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (21/21 OK, including
  concrete external-gate guard coverage for issue #28 and
  trademark/legal approval)
- `bash scripts/check-launch-review-ledger.sh`
- `GOTOOLCHAIN=go1.25.10 go test ./internal/tools -run
  'TestToolContractMatrix|TestToolDescriptors_AnnotationsMatchHints|TestAnnotationConsistency'
  -count=1`
- `make doc-parity`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check`
- `make launch-external-status` (11 open, 0 unknown; external gates
  remain open)
- `make public-content-audit` (2 open, 0 unknown; candidate branch
  file content and public-history review clean, local-artifact/full-tree
  review remains open)
- `bash -n scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-public-content-audit.sh` (3 offline-stub
  cases, including candidate branch-content checks for TLS bypasses,
  MIT license, `.gitignore`, `.gitleaks.toml` allowlist
  descriptions, `CLAUDE.md`, live Clockify secret assignments, and
  env-like files)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including public-content candidate, history, and local-scope guards)
- `bash scripts/check-launch-review-ledger.sh`
- `make doc-parity`
- `make script-tests`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `GOTOOLCHAIN=go1.25.10 go test ./internal/config ./internal/mcp
  -run 'TestLoadDefaultProtocolVersion|TestProtocolVersion_|TestStreamableInitializeProtocolVersionHeader'
  -count=1`
- `bash scripts/test-check-doc-parity.sh` (57/57 OK, including
  default protocol-version guidance)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make config-parity config-doc-parity doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `GOTOOLCHAIN=go1.25.10 go run
  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0 run
  ./...` (reported `0 issues` after fixing the local errcheck /
  staticcheck findings that release-check skipped without a local
  `golangci-lint` binary)
- `GOBIN=<tmp> GOTOOLCHAIN=go1.25.10 go install
  github.com/rhysd/actionlint/cmd/actionlint@914e7df21a07ef503a81201c76d2b11c789d3fca`
  followed by `<tmp>/actionlint -color=false .github/workflows/*.yml`
  (matched the pinned CI install revision and returned 0 findings)
- Pinned Group 6 local security preflight after the final lint cleanup:
  `GOTOOLCHAIN=go1.25.10 <tmp>/govulncheck ./...` with
  `govulncheck@v1.3.0` returned `No vulnerabilities found`;
  `GOTOOLCHAIN=go1.26.2 <tmp>/govulncheck ./...` found Go 1.26.2
  standard-library vulnerabilities GO-2026-4971 and GO-2026-4918
  fixed in Go 1.26.3, so the supported launch-candidate toolchain
  remains the exact Go 1.25.10 pin until a checked bump lands.
- `semgrep scan --config p/default --metrics=off --error --exclude
  .git --exclude .bench --exclude clockify-mcp .` (scanned 1153
  tracked files, 0 findings)
- `git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md' || true`
  (suppression context remains documented in ADR 0008 and ADR 0017)
- `GOTOOLCHAIN=go1.25.10 make verify-fips` (default FIPS tests and
  the `-tags=fips,grpc` build combination passed)
- `bash scripts/test-check-go-version-parity.sh` (7/7 OK) and
  `bash scripts/check-go-version-parity.sh` after replacing broad
  `1.25.10+` wording with exact pinned-toolchain wording in README
  and CONTRIBUTING
- `go test ./internal/mcp -run
  'TestSanitizePanicStackScrubsPathsAndTruncates|TestRecoverDispatch|TestAuditDurability'
  -count=1`
- `go test ./internal/mcp -run
  'TestStreamableHTTPCrossInstanceCancellationIsBestEffort|TestSanitizePanicStackScrubsPathsAndTruncates|TestAuditDurability'
  -count=1`
- `go test ./internal/enforcement -run 'TestPipelineClone|TestGateClone'
  -count=1`
- `go test ./internal/ratelimit -run
  'TestFromEnvDefaults|TestFromEnvWithAcquireTimeoutRespectsOverride'
  -count=1`
- `go test ./internal/tools -run
  'TestQuickReport|TestQuickReportLargeRangeStreamsBoundedSample|TestDetailedReport|TestUpdateEntryEmitsMergePatchOnCachedURI|TestCacheWriteThrough_PrimesCacheForMergePatch'
  -count=1`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed` after the lint
  cleanup)
- `GOTOOLCHAIN=go1.25.10 go test ./internal/mcp -run
  'TestProtocolVersion_|TestStreamableInitializeProtocolVersionHeader'
  -count=1`
- `GOTOOLCHAIN=go1.25.10 go test ./internal/config -run
  'TestLoadDefaultProtocolVersion|TestEnvSpec_CoversEveryGetenv|TestSpec_DocumentedDefaultsMatchLoader'
  -count=1`
- `bash scripts/test-check-doc-parity.sh` (57/57 OK, including
  default protocol-version guidance)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make config-parity config-doc-parity doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `(cd tools/govulncheck && GOWORK=off GOBIN=<tmp> go install
  golang.org/x/vuln/cmd/govulncheck) && <tmp>/govulncheck -version`
  (confirmed `Scanner: govulncheck@v1.3.0`)
- `bash scripts/check-go-version-parity.sh`
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (57/57 OK, including the
  govulncheck tool-module guard, unconditional SLSA wording guard,
  legacy HTTP EOL runbook guard, serverInfo identity guidance, and
  default protocol-version guidance)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `bash scripts/test-check-launch-evidence-gate.sh` (9/9 OK, including
  the reworded Group 7 release-artifact checkbox guard)
- `make doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 -version`
  (confirmed `Scanner: govulncheck@v1.3.0`)
- `bash scripts/test-check-doc-parity.sh` (52/52 OK, including the
  govulncheck CI version-proof guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (51/51 OK, including the
  build-tag submodule Dependabot watcher guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `go test ./internal/mcp -run
  'TestInlineMetrics_BearerSchemeCaseAndTokenWhitespace|TestMetricsMuxAuth'
  -count=1`
- `gofmt -w internal/mcp/inline_metrics_test.go` (after the first
  release-check attempt failed at `fmt` on the new test alignment)
- `git diff --check`
- `make doc-parity`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- Official MCP 2025-11-25 spec check:
  `https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle`,
  `https://modelcontextprotocol.io/specification/2025-11-25/changelog`,
  and
  `https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks`
  (confirmed `2025-11-25` is published and that tasks should not be
  advertised unless task-augmented requests are implemented)
- `go test ./internal/mcp -run TestProtocolVersion_CapabilitiesShape -count=1`
- `make doc-parity`
- `git diff --check`
- `go test ./internal/mcp -run TestMCPCORSPreflight -count=1`
- `make doc-parity`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `go test ./internal/mcp -run
  'TestInlineMetrics_BearerSchemeCaseAndTokenWhitespace|TestMetricsMuxAuth'
  -count=1`
- `git diff --check`
- `make launch-external-status` (11 open, 0 unknown; external and
  repo-state gates remain open)
- `make public-content-audit` (3 open, 0 unknown; candidate branch
  file content clean, public-history and local-artifact/full-tree
  review remain open)
- `git diff --check`
- `make doc-parity`
- `bash scripts/check-launch-review-ledger.sh`
- `git diff --check`
- `make public-content-audit` (3 open, 0 unknown; candidate branch
  file content clean; local/full-tree findings include ignored
  `.local/`, `.serena/`, and nested `go-clockify/` artifacts)
- `bash -n scripts/check-launch-external-status.sh
  scripts/test-check-launch-external-status.sh`
- `shellcheck -S warning scripts/check-launch-external-status.sh
  scripts/test-check-launch-external-status.sh`
- `bash scripts/test-check-launch-external-status.sh` (16 run, 0
  failed; repository-description action text now prints the exact
  suggested public metadata)
- `make doc-parity`
- `git diff --check`
- `make launch-external-status` (11 open, 0 unknown; repository
  description gate now suggests `128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.`)
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed` after the ignored
  artifact wording and repo-description action-text patches)
- `make launch-external-status` (11 open, 0 unknown; still external or
  repo-state gated)
- `make public-content-audit` (3 open, 0 unknown; candidate branch
  file content clean, public-history and local-artifact/full-tree
  review remain open)
- `bash -n scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (31/31 OK,
  including the exact repository-description action guard)
- `bash scripts/check-launch-review-ledger.sh`
- `make doc-parity`
- `git diff --check`
- `make script-tests` (all script regression suites passed,
  including `test-check-launch-review-ledger.sh` at 31/31)
- `GOTOOLCHAIN=go1.25.10 go test ./internal/tools -run '^$' -bench
  'Benchmark(LargeWorkspaceReadiness|AggregateEntriesRange)' -benchmem
  -benchtime=200ms -count=1`
- `GOTOOLCHAIN=go1.25.10 go run github.com/rhysd/actionlint/cmd/actionlint@latest
  -color=false .github/workflows/*.yml`
- `gh run list --repo apet97/go-clockify --workflow=bench.yml --limit=6
  --json databaseId,status,conclusion,event,createdAt,headSha,displayTitle,url`
- `bash scripts/check-bench-baseline.sh`
- `bash -n scripts/check-live-tool-coverage.sh scripts/test-check-live-tool-coverage.sh`
- `bash scripts/check-live-tool-coverage.sh --plan`
- `bash scripts/check-live-tool-coverage.sh` (0 open, 0 unknown)
- `bash scripts/test-check-live-tool-coverage.sh` (6 cases, 0 failed)
- `GOTOOLCHAIN=go1.25.10 go test -tags=livee2e ./tests -run
  TestLiveTier1ReadOnly -count=1` (compile/skip-path check only; no
  live env vars, not launch evidence)
- `bash -n scripts/collect-license-evidence.sh
  scripts/test-collect-license-evidence.sh scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh`
- `shellcheck -S warning scripts/collect-license-evidence.sh
  scripts/test-collect-license-evidence.sh scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh`
- `make license-evidence-plan`
- `make license-evidence` (raw build-variant inventory; 0 modules
  without local license candidates, 0 unknown variants; not legal
  clearance)
- `bash scripts/test-collect-license-evidence.sh` (3/3 OK)
- `bash scripts/test-check-doc-parity.sh` (37/37 OK)
- `make doc-parity`
- `make script-tests`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check`
- Historical pre-hardening check: `GOTOOLCHAIN=go1.25.10 make
  verify-vuln` skipped because `govulncheck` was not installed on
  `PATH`; superseded by the latest `make verify-vuln` entry below,
  which installs and runs the pinned tool module.
- `GOTOOLCHAIN=go1.25.10 go run
  golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...` (no
  vulnerabilities found)
- `GOTOOLCHAIN=go1.25.10 make verify-fips` (passed on macOS arm64,
  including the `-tags=fips,grpc` build combination)
- `semgrep scan --config p/default --metrics=off --error --exclude
  .git --exclude .bench --exclude clockify-mcp .` (1145 tracked files,
  0 findings)
- `make secret-scan` (failed on ignored/local artifacts only:
  `.local/http-test.env`, `.serena/`, and the duplicate
  `go-clockify/` checkout; not candidate-tag evidence)
- `bash scripts/test-check-doc-parity.sh` (57/57 OK, including
  README / CONTRIBUTING local-verification wording and Makefile
  release-check wording drift guards plus stale shippable wording in
  docs, shared-service profile Group 2 scoping, and
  production-readiness blocker-scope wording, plus P3-5 baseline
  header docs, brand/legal URI plus gRPC service-name review docs, and
  SUPPORT.md SLSA private-repo cosign fallback plus stale unconditional
  SLSA public wording plus legacy HTTP EOL runbook coverage,
  serverInfo identity guidance, and default protocol-version guidance)
- `bash -n scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-launch-review-ledger.sh` (28/28 OK,
  including the stale doc-parity case-count, Appendix B
  open-question, coordinator file-scope, final checklist, and gRPC
  service-name external-gate guards)
- `bash scripts/check-launch-review-ledger.sh`
- `make doc-parity`
- `make script-tests`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`)
- `make launch-external-status` (11 open, 0 unknown; external gates
  remain open)
- `make public-content-audit` (3 open, 0 unknown; candidate branch
  file content clean, public-history and local-artifact/full-tree
  review remain open)
- `git log --all --format='%h%x09%ad%x09%s' --date=short -200 |
  grep -Ei 'secret|token|password|key='` (three benign matches:
  confirmation-token design and per-token rate-limit commits)
- `bash -n scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-public-content-audit.sh` (4 offline-stub
  cases, including the documented public-history false-positive
  closure path)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `bash scripts/check-launch-review-ledger.sh`
- `make public-content-audit` (2 open, 0 unknown; candidate branch
  file content and public-history review clean, local-artifact/full-tree
  review remains open)
- `make doc-parity`
- `make script-tests`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `make launch-external-status` (11 open, 0 unknown; external and
  repo-state gates remain open)
- `make public-content-audit` (2 open, 0 unknown; candidate branch
  file content and public-history review clean, local-artifact/full-tree
  review remains open)
- `git diff --check`
- `bash scripts/test-check-public-content-audit.sh` (5 offline-stub
  cases, including documented ignored local-artifact and documented
  public-history closure paths)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including public-content candidate, history, and local-scope guards)
- `bash scripts/check-launch-review-ledger.sh`
- `bash -n scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-public-content-audit.sh
  scripts/test-check-public-content-audit.sh
  scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make doc-parity`
- `make script-tests`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `make launch-external-status` (11 open, 0 unknown; external and
  repo-state gates remain open)
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (48/48 OK, including the
  stale public-content local-artifact wording guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make doc-parity`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (49/49 OK, including the
  stale shared-service launch-blocking wording guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make doc-parity`
- `make script-tests`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `gh run view 25592823559 --json databaseId,status,conclusion,headSha,createdAt,updatedAt,event,displayTitle,jobs,url`
  (confirmed scheduled `mutation.yml` run 25592823559 was
  `completed/cancelled` on
  `4fe957547f9e6aea749a85f87823d17a0ccc2928`; only
  `Mutation (internal/tools)` was cancelled after roughly the old
  45-minute timeout while the other matrix legs succeeded)
- `bash -n scripts/check-launch-external-status.sh
  scripts/test-check-launch-external-status.sh`
- `shellcheck -S warning scripts/check-launch-external-status.sh
  scripts/test-check-launch-external-status.sh`
- `bash scripts/test-check-launch-external-status.sh` (16 run, 0
  failed; stale scheduled mutation evidence now requires the timeout
  fix to land before waiting for a scheduled success)
- `make script-tests`
- `make doc-parity`
- `make launch-external-status` (11 open, 0 unknown; mutation action
  now names the local timeout-fix landing step before the scheduled-run
  wait)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `go test ./internal/mcp -run
  'TestRecoverDispatch_RedactingLoggerMasksSecretShapedPanicValue|TestSanitizePanicStackScrubsPathsAndTruncates|TestHandleWithRecover_ReturnsStableEnvelopeOnPanic'
  -count=1`
- `bash scripts/smoke-grpc-auth.sh`
- `go test ./internal/mcp -run
  'TestStreamableEventsAliasFullMuxRecordsHTTPPathMetric|TestStreamableEventsBackCompatAlias'
  -count=1`
- `go build -tags=grpc,grpcreflection ./...`
- `(cd internal/transport/grpc && go test -tags=grpcreflection
  -run TestRegisterOptionalReflectionRegistersReflectionService
  -count=1 ./...)`
- `SKIP_FIPS=1 bash scripts/check-build-tags.sh`
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `shellcheck -S warning scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (50/50 OK, including
  the T-17 gRPC reflection dev-only posture guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `make doc-parity`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `GOTOOLCHAIN=go1.25.10 go test ./internal/config ./internal/mcp
  -run 'TestLoadDefaultProtocolVersion|TestProtocolVersion_|TestStreamableInitializeProtocolVersionHeader'
  -count=1`
- `bash scripts/test-check-doc-parity.sh` (57/57 OK, including
  default protocol-version guidance)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK)
- `make config-parity config-doc-parity doc-parity`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; external,
  repo-state, release, and maintainer-action gates remain open)
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (59/59 OK, including
  Makefile `verify-vuln` pinned tool-module execution)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including the 59-case prompt-to-artifact guard)
- `make verify-vuln` (installed and ran pinned
  `tools/govulncheck`; printed `Go: go1.25.10`,
  `Scanner: govulncheck@v1.3.0`, and `No vulnerabilities found`)
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open)
- `bash -n scripts/check-go-version-parity.sh
  scripts/test-check-go-version-parity.sh`
- `bash scripts/test-check-go-version-parity.sh` (9/9 OK, including
  the CHANGELOG support-matrix wording and FIPS go.mod-floor comment
  guards)
- `bash scripts/check-go-version-parity.sh` (`go-version-parity: OK
  (1.25.10)`)
- `make doc-parity`
- `git diff --check`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open)
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (62/62 OK, including the
  README SLSA provenance availability wording guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including the 62-case prompt-to-artifact guard)
- `rg -n "every binary and container image ships with .*SLSA|Signed
  releases.*SLSA build provenance|ships with cosign signatures, SPDX
  SBOM, and SLSA|SLSA build provenance is attached when GitHub artifact
  attestations are available" README.md docs SECURITY.md SUPPORT.md
  AGENTS.md scripts/test-check-doc-parity.sh scripts/check-doc-parity.sh`
- `make doc-parity`
- `git diff --check`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open)
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `ruby -ryaml -e 'ARGV.each { |p| YAML.load_file(p); puts "OK #{p}" }'
  .github/workflows/release-smoke.yml`
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (61/61 OK, including the
  release-smoke SLSA bare-404 skip guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including the 61-case prompt-to-artifact guard)
- `make doc-parity`
- `git diff --check`
- `GOBIN=<tmp> GOTOOLCHAIN=go1.25.10 go install
  github.com/rhysd/actionlint/cmd/actionlint@914e7df21a07ef503a81201c76d2b11c789d3fca`
  followed by `<tmp>/actionlint -color=false
  .github/workflows/release-smoke.yml`
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open)
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `git diff --check`
- `bash -n scripts/check-doc-parity.sh
  scripts/test-check-doc-parity.sh scripts/check-launch-review-ledger.sh
  scripts/test-check-launch-review-ledger.sh`
- `bash scripts/test-check-doc-parity.sh` (60/60 OK, including the
  gap-analysis blocker-scope wording guard)
- `bash scripts/test-check-launch-review-ledger.sh` (35/35 OK,
  including the 60-case prompt-to-artifact guard)
- `make doc-parity`
- `git diff --check`
- `GOTOOLCHAIN=go1.25.10 make release-check` (ended with
  `release-check: OK — local pre-ship gate passed`; local
  `golangci-lint` and `actionlint` were unavailable and skipped via
  the documented CI-enforced path)
- `make public-content-audit` (0 open, 0 unknown; candidate branch
  file content, public-history review, and local-artifact/full-tree
  review all clean)
- `make launch-external-status` (11 open, 0 unknown; final-SHA,
  default-branch workflow, repository-state, release, and
  maintainer-action gates remain open)
