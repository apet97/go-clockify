# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`docs/upgrade-checklist.md` post-rollout grep matches the real
  401 log line.** §"4. Watch the rollout for regressions" listed
  `msg=http_request status=401 reason=auth_failed` as the spike
  pattern to watch — but iter127 already confirmed the actual
  401 auth-failure record at
  `internal/mcp/transport_auth_errors.go:35` emits
  `msg=http_auth_failed`, not `msg=http_request`. iter127 closed
  the drift in `docs/runbooks/auth-failures.md` but missed this
  parallel surface; an operator running an upgrade and grepping
  the suggested string would see zero hits even when bearer-token
  drift was happening, exactly the failure mode the checklist is
  meant to catch. Pattern now reads `msg=http_auth_failed
  status=401 reason=auth_failed` with a one-line clarification
  pointing at auth-failures.md as the canonical recipe.
- **`docs/adr/0002-transport-selection.md` Spec link points at
  `2025-11-25`.** ADR-0002 References block linked the spec at
  `modelcontextprotocol.io/specification/2025-06-18`, the version
  the ADR was written against. iter149's transport-table fix
  brought the streamable_http row up to four-version coverage,
  but this companion link still pinned an older spec URL. Now
  points at `2025-11-25` (the newest supported version) with a
  parenthetical naming the older accepted versions and pointing
  at `internal/mcp/server.go:SupportedProtocolVersions` as the
  canonical list. Same iter144/iter149 protocol-version sweep
  but at the spec-URL surface.
- **`docs/adr/0002-transport-selection.md` streamable_http row
  covers 2025-11-25 protocol version.** The Decision-section
  transport table described streamable_http as targeting
  "spec-strict 2025-06-18 / 2025-03-26 MCP clients" — missing
  `2025-11-25`, the newest version that also defines streamable
  HTTP (every version since the 2025-03-26 introduction supports
  it). Same iter144 protocol-version drift class iter144 fixed in
  ADR-0012's compat-policy committed window. Row now reads "every
  version since the 2025-03-26 introduction" with the three
  current streamable-HTTP-capable versions enumerated; future
  protocol versions don't re-stale the wording. Source of truth
  remains `internal/mcp/server.go:SupportedProtocolVersions`.
- **`docs/deploy/profile-single-tenant-http.md` env-file annotation
  doesn't claim streamable_http is the *only* notification-capable
  transport.** The `MCP_TRANSPORT=streamable_http` env-block comment
  said "streamable_http is the only transport that can emit
  tools/list_changed after Tier 2 group activation". Operator-correct
  within the HTTP-shape this profile covers, but unconditionally
  wrong: stdio (per `internal/mcp/transport_stdio.go`) and gRPC
  (per `internal/transport/grpc/transport.go:230-243` `streamNotifier`
  + ADR-0008) also emit `tools/list_changed`. Same iter146 / iter147
  transport-coverage drift class at the deploy-profile env-annotation
  surface. Comment now scopes the claim to "the recommended HTTP-shape
  transport" with explicit pointers to profile-local-stdio.md and
  profile-private-network-grpc.md for the other two
  notification-capable transports.
- **`README.md` "Stale tool list" troubleshooting line covers
  gRPC.** Iter146 fixed the same gRPC omission in
  `docs/clients.md` Tool Discovery; this commit closes the
  parallel surface in the README. The line previously read
  "Stdio and `streamable_http` clients receive
  `notifications/tools/list_changed` after activation ... only
  legacy `http` clients must manually re-fetch `tools/list`",
  silently leaving gRPC out even though
  `internal/transport/grpc/transport.go:230-243` `streamNotifier`
  fans the same notification through every active `Exchange`
  stream. Operators on the `private-network-grpc` profile reading
  the README troubleshooting now see gRPC alongside stdio /
  streamable_http with the matching bidirectional-stream
  delivery model. Same iter111→iter112 parallel-surface pattern
  applied to gRPC.
- **`docs/clients.md` Tool Discovery section adds the gRPC
  bullet.** The transport-by-transport behaviour list named
  Stdio, streamable_http, and Legacy http but omitted gRPC,
  even though `internal/transport/grpc/transport.go:230-243`
  defines a `streamNotifier` that fans `tools/list_changed`,
  `notifications/progress`, and `notifications/resources/updated`
  out through every active `Exchange` stream (see ADR-0008
  §"Per-stream notifier registration"). Operators following the
  `private-network-grpc` profile would have read the section,
  not seen gRPC mentioned, and either polled `tools/list`
  unnecessarily or assumed gRPC had the legacy-http limitation
  it doesn't have. Same iter111-class transport-bullet drift
  pattern that closed the streamable_http vs legacy-http split,
  this time at the gRPC surface.
- **`docs/adr/0010-metrics-stack-direction.md` reflects post-v1.0
  reality.** Two stale claims: Context paragraph said "~15
  hand-registered series" (actual is 22 per
  `internal/metrics/metrics.go`) and Decision text said "Keep
  homegrown for v0.x, revisit at v1.0" without acknowledging
  that v1.0/v1.1/v1.2 all shipped without a revisit. Context
  bumped to 22 with a `grep -cE` recipe so future drift is
  trivial to recheck. Decision keeps the original v0.x/v1.0
  framing as the original commitment text but adds a
  parenthetical status update naming v1.0.0's 2026-04-12 ship
  date and "still keep homegrown through v1.x" as the
  implicit extension until a follow-up ADR. Same iter142
  post-flip-state-update class at the Proposed-ADR surface.
- **`docs/adr/0012-backward-compatibility-policy.md` MCP-protocol
  committed window matches the canonical four-version slice.**
  ADR-0012 said "the **Committed window:** the last three
  published protocol versions ... (today: `2025-06-18`,
  `2025-03-26`, `2024-11-05`)" but
  `internal/mcp/server.go:SupportedProtocolVersions` lists FOUR
  entries (`2025-11-25`, `2025-06-18`, `2025-03-26`,
  `2024-11-05`). iter79 closed the same drift in clients.md and
  iter80 in the README Compatibility table; ADR-0012 was the
  canonical compat-policy surface still showing the pre-iter79
  three-version slice. Bullet now reads "every published
  protocol version" with the slice as source of truth, so future
  additions don't re-stale the wording. Same drift class as
  iter79/iter80 at the ADR surface.
- **`docs/adr/0013-private-repo-slsa-posture.md` Status block
  cites the real workaround-introduction date.** The Status block
  said the workaround existed "from 2026-04-22 (SLSA workaround
  introduction via Wave G) through 2026-04-22 (this flip)" — both
  dates were the same, implying a zero-day workaround window. Git
  history shows the actual SLSA-non-fatal commit landed
  2026-04-20 (`fix(ci): make SLSA attestation non-fatal for
  private user-owned repo`, shipped with v1.0.3) and the
  public-flip removal was 2026-04-22, so the workaround was live
  for roughly 2 days during the v1.0.3 cut. The "v1.0.0–v1.0.3
  era" claim was also slightly off — v1.0.0 / v1.0.1 / v1.0.2
  predated the workaround. Status block now reads "v1.0.3 era
  only" with the introduction-commit subject quoted so a reader
  doing forensics on a v1.0.3 build can find the exact change.
  Pure historical-precision fix; doesn't change the Superseded
  outcome.
- **`docs/adr/0016-single-maintainer-governance.md` "Auto-generate
  branch-protection.md" follow-up reflects post-flip reality.**
  The follow-up's "Currently blocked by the GitHub API returning
  403 on user-owned private repos; the auto-gen gate becomes
  viable once the repo flips to public or to an org" became stale
  on 2026-04-22 when the repo flipped public (per ADR-0013, now
  Superseded). `scripts/audit-branch-protection.sh` already reads
  the live protection state via `gh api`; the remaining gap is
  the rendering + CI-parity glue, not the API blocker. Bullet now
  describes the actual remaining work so a contributor reading
  the follow-up isn't pointed at a non-blocker. Same iter141-class
  post-flip context drift, this time at the ADR-0016 follow-ups
  surface.
- **`docs/verification.md` SLSA note frames pre-public-flip
  behaviour as historical context.** The "user-owned private
  repositories" note still described `release.yml`'s
  `actions/attest-build-provenance` step as running with
  `continue-on-error: true` and the release-smoke workflow
  treating attestation failures as `::notice::`-skipped. That was
  the pre-2026-04-22 behaviour; after the public flip
  (`release.yml:90-94`: "Repo is public (2026-04-22) —
  attestation is now a mandatory gate") the workaround was
  removed and every release since carries a real attestation.
  An operator verifying a current binary today would not hit the
  fallback the note describes; an operator verifying a v1.0.x
  binary still might. Note rephrased as historical context for
  the v1.0.x window with explicit "v1.1.0 onward should succeed"
  guidance, plus updated cross-reference framing for ADR-0013
  (now Superseded). Same iter109/iter125 ADR-0013-superseded
  drift class, this time at the verification.md surface.
- **Global coverage floor reads `71%` consistently across script
  and policy doc.** `scripts/check-coverage.sh:6` doc-comment said
  "default: 55" but the actual default at line 19 is `71`.
  `docs/coverage-policy.md:37` table said `Global = 69%` and the
  Planned-ratchets section named "global 70%" as the next target,
  but the floor has already ratcheted to 71%. Both the comment and
  the doc table now read `71`, and the Planned-ratchets section
  acknowledges the previous target was reached and names global
  72% as the next ratchet target. Same iter107-class doc-vs-code
  drift but at the global-floor surface.
- **`docs/performance.md` "Reproduce locally" lists all 5 load
  scenarios.** Section §"Throughput envelope (load harness)"
  reproduce-locally bash block listed the 4 pre-iter138
  scenarios (`per-token-saturation`, `steady`, `burst`,
  `tenant-mix`) but `tests/load/main.go:63-136` registers 5,
  the 5th being `ratelimit-reap-correctness`. Continuation of
  the iter138 sweep at the third operator-visible surface
  (after `tests/load/README.md` and `.github/workflows/load.yml`)
  so an operator copy-pasting from performance.md gets the
  canonical scenario set.
- **`tests/load/README.md` and `.github/workflows/load.yml` cover
  `ratelimit-reap-correctness`.** Both surfaces listed only 4
  load scenarios (`steady`, `burst`, `tenant-mix`,
  `per-token-saturation`) but `tests/load/main.go:63-136`
  registers 5 — the 5th, `ratelimit-reap-correctness`, is the
  two-phase scenario that verifies the per-subject limiter reaps
  correctly: noisy tenant saturates, idles past one window, then
  resumes; reap must restore full budget without affecting the
  cold tenant. README's Running examples + Scenarios table and
  the workflow's `workflow_dispatch` input description both
  updated to match the canonical 5-scenario registry. Same iter136
  / iter137 fictional-list drift class at the load-harness
  surfaces.
- **`.github/workflows/chaos.yml` workflow_dispatch description
  lists `upstream-429-concurrent`.** The scenario-input dropdown
  hint named only the original 5 scenarios (429-storm, 503-burst,
  mid-body-reset, tls-handshake-fail, dns-fail) but
  `tests/chaos/main.go:54-61` registers 6 — the
  `upstream-429-concurrent` scenario. iter136 fixed the same
  drift in the README; this commit closes it at the workflow-
  input surface so an operator dispatching the workflow
  manually sees every scenario name in the description hint.
  Same iter136 fictional-list drift class.
- **`tests/chaos/README.md` covers the `upstream-429-concurrent`
  scenario.** The README's scenario table listed 5 scenarios
  (429-storm, 503-burst, mid-body-reset, tls-handshake-fail,
  dns-fail), the Acceptance section claimed `all 5 scenarios
  passed` is the expected output, and the Recorded run showed 5
  PASS lines — but `tests/chaos/main.go:54-61` registers a 6th
  scenario, `upstream-429-concurrent` (added to verify retries
  don't serialise behind a shared lock; cited in
  `docs/runbooks/production-incident-drill.md` Scenario A as
  the canonical concurrent-throttling chaos test). Operators
  running `go run ./tests/chaos -scenario all` today see "all 6
  scenarios passed" with the extra PASS line, then have to
  decide whether the README is stale or their run is wrong.
  Scenarios table now includes the row, Acceptance text says
  6, and the Recorded run shows the additional `[PASS]`. Same
  iter134/iter135 fictional-number drift class.
- **`docs/runbooks/release-asset-count.md` "Wrong count" /
  "Post-incident" sections updated to 46 (sweep continuation).**
  iter134 fixed the Symptoms section's two "expected 28"
  quotes but missed four more pre-gRPC-era `28` references in
  the runbook body: the "Wrong count (pass 2 fail)" bucket
  named "N > 28" / "N < 28" / "all 28 expected files exist"
  thresholds, and the Post-incident grep recipe still searched
  for `OK: all 28 expected release assets present`. Script's
  actual success log at `scripts/check-release-assets.sh:256`
  prints `OK: all 46 expected release assets present`. Sweep
  also extended the Script-self-bug bullet's array enumeration
  from `DEFAULT_UNIX_PLATFORMS / FIPS_PLATFORMS / EXPECTED_COUNT`
  to the full six-array list (DEFAULT_UNIX, DEFAULT_WINDOWS,
  FIPS, POSTGRES, GRPC, GRPC_POSTGRES) so a contributor fixing
  matrix drift knows every input that contributes to the count.
  Confirms `grep -n 28 docs/runbooks/release-asset-count.md`
  returns zero hits — the runbook now reads consistently.
- **`docs/runbooks/release-asset-count.md` symptom log lines
  show `expected 46` matching the canonical script.** The
  Symptoms section quoted log lines as
  `FAIL: found N matching top-level files in dist, expected 28`
  and `BUG: expected array has N entries, script says 28`, but
  `scripts/check-release-assets.sh:143` declares
  `EXPECTED_COUNT=46` (15 binaries × 3 artifacts +
  `SHA256SUMS.txt`). 28 was the pre-gRPC era count (5 default
  + 4 FIPS binaries), valid only before iter101's wave that
  added Postgres / gRPC / gRPC + Postgres rows to the canonical
  matrix. An operator running the release and hitting the asset-
  count failure would have grepped the Symptoms list, found
  numbers that didn't match the actual log line, and concluded
  either the runbook or the script was wrong. Symptoms now show
  `expected 46` with a one-line breakdown of the 5 tag
  combinations + 3 artifacts per binary, and the `BUG`
  enumeration includes the four newer platform arrays
  (POSTGRES / GRPC / GRPC_POSTGRES) so a reviewer auditing
  internal consistency knows which arrays to cross-check. Same
  iter114/iter129 fictional-number drift class.
- **Deploy-profile docs use canonical `0`/`1` for boolean env
  vars instead of `true`/`false`.** `docs/deploy/profile-single-
  tenant-http.md` set `MCP_HTTP_INLINE_METRICS_ENABLED=true` and
  `MCP_STRICT_HOST_CHECK=true`, while
  `docs/deploy/production-profile-shared-service.md` used
  `MCP_HTTP_INLINE_METRICS_ENABLED=false` — both work because
  `optionalBoolEnv` (`internal/config/config.go:780`) goes through
  `strconv.ParseBool` which accepts the wider boolean set, but
  spec.go's `Enum` for both vars is `["0", "1"]` and every other
  surface (README CONFIG-TABLE, configmap, runbooks, Dockerfile)
  uses `0`/`1`. Now the deploy profile docs match — operators
  copy-pasting between docs see consistent quoting and don't
  question whether `true` and `1` differ semantically. Pure
  consistency fix; no semantic change at runtime.
- **`CLOCKIFY_TIMEZONE` fallback documentation acknowledges
  loadLocation exception.** Iter131's commit body claimed "every
  call site that consumes Service.DefaultTimezone has the same
  guard `if loc == nil { loc = time.UTC }`" — accurate for
  entries.go, resources.go, and reports.go's aggregate path, but
  it missed reports.go:250 where `WeeklySummary` does
  `loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)`.
  `loadLocation` (common.go:522-528) ultimately falls back to
  `time.Now().Location()` (system-local) when both the arg and
  DefaultTimezone are nil, not UTC. The behaviour is split: most
  consumers fall through to UTC, but the WeeklySummary path keeps
  the historical system-local default for backward compatibility
  with the `Defaults to Monday of the current week in local time`
  tool-descriptor claim. Updated `internal/tools/common.go:21`
  doc comment + `deploy/helm/clockify-mcp/values.yaml:115` Helm
  comment to spell out the split fallback explicitly so a
  contributor or operator following either trail sees the actual
  behaviour. Pure doc-correction; iter131's substantive fix
  (UTC for the most-trafficked path) stands.
- **`CLOCKIFY_TIMEZONE` documented fallback is UTC, not system.**
  Both `deploy/helm/clockify-mcp/values.yaml:115` ("default:
  system") and `internal/tools/common.go:21` ("nil = system
  timezone") claimed an unset `CLOCKIFY_TIMEZONE` falls back to
  the host's local timezone. Reality, per
  `internal/tools/entries.go:233-235`,
  `internal/tools/reports.go:76`,
  `internal/tools/resources.go:86`,
  `internal/tools/resources.go:146`,
  `internal/tools/entries.go:367-369`: every call site does
  `if loc == nil { loc = time.UTC }`. An operator on a
  Europe/Berlin host expecting locale-aware time parsing for
  log-time / report-aggregation tools would have got UTC instead,
  potentially shifting day-boundary aggregation by up to a full
  day. Both surfaces now name UTC as the actual fallback and
  the values.yaml comment includes a sample IANA value
  (`Europe/Berlin`) so operators know they need to set the var
  explicitly. Same hosted-default-doc-vs-code-reality drift
  class as iter130 (`MCP_METRICS_AUTH_MODE`).
- **`deploy/k8s/base/configmap.yaml` and
  `deploy/helm/clockify-mcp/values.yaml` describe the real
  `MCP_METRICS_AUTH_MODE` default.** Both deploy templates'
  inline comments described the default as `none`, but
  `internal/config/config.go:331-334` defaults it to
  `static_bearer` when `MCP_METRICS_BIND` is set. An operator
  setting `MCP_METRICS_BIND=:9091` and leaving `_AUTH_MODE`
  blank — under the impression "none" was the documented
  default — would have hit the startup error
  `MCP_METRICS_BEARER_TOKEN is required when
  MCP_METRICS_AUTH_MODE=static_bearer`. Comments now name the
  real default with the conditional ("static_bearer when
  MCP_METRICS_BIND is set"), make explicit that `none` opts
  out, and remind operators to set the bearer token. Same
  iter40-era hosted-fail-closed posture as iter114/iter127–129
  drift class but at the deploy-template surface instead of
  runbook prose.
- **`docs/runbooks/clockify-outage-drill.md` Scenario A names the
  real upstream-failure metric.** The drill checklist told the
  operator "Check that `clockify_mcp_upstream_errors_total` metric
  increments" but no such metric exists. The actual metric is
  `clockify_upstream_requests_total` (no `mcp` infix, no
  `_errors_` base — outbound metrics live under the
  `clockify_upstream_*` namespace per
  `internal/metrics/metrics.go:654`), and failures are
  distinguished by the `status="5xx"` (or `4xx` for 429) label
  on the same series as success. The retry counter
  `clockify_upstream_retries_total` is the secondary signal once
  client retries kick in. Drill step now names the real metric +
  filter expression + source line so an operator running the
  drill can verify the right gauge moved instead of grepping for
  a metric that never existed. Same iter114/iter127/iter128
  fictional-string drift class.
- **`docs/runbooks/hosted-error-sanitization.md` "Temporary debug
  procedure" stops referencing fictional `tool_call_error`
  record.** Section §4 told operators "raise `MCP_LOG_LEVEL=debug`
  and reproduce — the debug-level `tool_call_error` enriched
  record may already carry what you need." But no log message
  named `tool_call_error` exists in the codebase: tool failures
  emit `slog.Warn("tool_call", ..., "error", err.Error(), ...)`
  at `internal/mcp/tools.go:117` and `:179`, regardless of
  `MCP_LOG_LEVEL`. An operator following the runbook with
  `MCP_LOG_LEVEL=debug` set and grepping for `tool_call_error`
  would have found nothing and concluded the slog wasn't capturing
  the body — when in fact the WARN-level `msg=tool_call` line
  always carries the full body in `error=`. Step now names the
  real source lines, the canonical record name, and clarifies
  that the WARN line is unaffected by `MCP_LOG_LEVEL`. Same
  iter114/iter127 fictional-string drift class — operator
  following stale recipe gets silence rather than the right
  signal.
- **`docs/runbooks/auth-failures.md` symptoms + grep pattern match
  the actual log msg field.** Runbook section §1 told operators
  401 auth failures emit `msg=http_request status=401
  reason=auth_failed` and the §2 grep recipe matched only
  `msg=http_request`. Reality: 401 auth failures emit
  `msg=http_auth_failed` (via
  `logHTTPAuthFailure` in `internal/mcp/transport_auth_errors.go`),
  while 403 cases (cors_rejected, host_rejected) emit
  `msg=http_request`. An operator following the runbook would
  have grepped `msg=http_request` and missed every 401 because
  those carry the `http_auth_failed` msg field. Symptoms list now
  names both msg fields and the grep regex matches both. Same
  iter114 fictional-string drift class as the
  `metrics_auth_mode_unsafe`-vs-`risky_config` rename, except this
  one would have made the operator-on-call think there were no
  auth failures when there were.
- **`internal/transport/grpc/transport.go` `MaxRecvSize` doc-comment
  matches actual default.** The Options struct comment said
  `MaxRecvSize` "caps per-frame inbound bytes to match the legacy
  HTTP `MCP_HTTP_MAX_BODY` default (2 MiB) when unset" but two
  things had drifted: (1) the actual default at line 69 is
  `4194304` (4 MiB) since the iter83-era body-limit
  standardisation, not 2 MiB; (2) the primary knob is
  `MCP_MAX_MESSAGE_SIZE`, with `MCP_HTTP_MAX_BODY` as the deprecated
  alias since v1.0.1. Comment now describes the actual fall-through
  (Server.MaxMessageSize first, then 4 MiB literal) and names the
  current primary knob. Pure source-comment drift fix matching
  iter83 (SECURITY.md / production-readiness.md 2 MB → 4 MB) at the
  Go-source surface.
- **`GOVERNANCE.md` and `docs/production-readiness.md` audit-trail
  enumeration drops stale SLSA "where available" qualifier.**
  Both docs phrased the audit trail as including "SLSA build
  provenance where available" — the qualifier dated from the
  pre-2026-04-22 ADR-0013 era when SLSA was conditional on the
  repo being user-owned-public. iter100 deliberately mirrored
  GOVERNANCE.md's phrasing into production-readiness.md and
  acknowledged the post-flip reality in the commit body but kept
  the qualifier; iter124 then dropped it from SUPPORT.md as the
  fifth canonical surface. Now closing the loop at the upstream
  GOVERNANCE.md (the canonical source) and its
  production-readiness.md mirror, so all six "what is supported /
  what ships" surfaces (release-policy, verification,
  production-readiness, SECURITY, SUPPORT, GOVERNANCE) read
  consistently. The web-flow signed-squash-commits "where
  available" qualifier on the same line stays — that's still
  load-bearing because direct-push commits aren't web-flow signed.
- **`SUPPORT.md` "Signed releases" line drops "(where available)"
  SLSA qualifier.** SUPPORT.md said "every tagged release ships
  with cosign signatures, SBOM, and (where available) SLSA build
  provenance" — the "(where available)" qualifier dated from the
  pre-2026-04-22 era when ADR-0013 documented SLSA as conditional
  on the repo being public. Since the public-flip on 2026-04-22,
  SLSA is mandatory on every release. Same iter101 SLSA-paragraph
  drift class iter101 fixed in release-policy.md, iter102 in
  verification.md, iter94 in production-readiness.md, and iter123
  in SECURITY.md — SUPPORT.md is the fifth canonical "what is
  supported / what ships" doc surface, now aligned. Bullet now
  also enumerates the 15-binary contract by tag combination so
  it's not a single-binary-family claim either.
- **`SECURITY.md` "Verifying release artifacts" section enumerates
  all 15 binaries.** The section described releases as shipping a
  single binary family `clockify-mcp-<platform>[.exe]` plus its
  sigstore/SBOM/attestation siblings, but iter101 / iter102 already
  fixed the same v1.0.x-era artefact list shape in
  `docs/release-policy.md` and `docs/verification.md` — releases
  produce 15 binaries across five tag combinations (5 default + 4
  FIPS + 2 Postgres + 2 gRPC + 2 gRPC + Postgres). Operators
  reading SECURITY.md for the per-binary signature claim wouldn't
  have learnt the FIPS / Postgres / gRPC / gRPC-Postgres binaries
  also ship signed; section now mirrors release-policy's
  enumeration plus the iter113 SHA256SUMS-is-unsigned and
  iter109 ADR-0013 public-flip clarifications. Pure operator-doc
  fix; closes the iter101 sweep at the security-policy surface.
- **`.github/workflows/reproducibility.yml` workflow_dispatch
  example bumped to v1.2.0.** The reproducibility workflow's
  manual-dispatch tag input described the parameter as "Release
  tag to verify (e.g. v0.7.1)" — same iter48-era v0.x example-
  string drift that iter92 already fixed in release-smoke.yml's
  parallel description (iter92 commit `3c639f6`). v0.7.1 was the
  supported release when the reproducibility workflow landed but
  v1.0.0/v1.1.0/v1.2.0 have since shipped; an operator dispatching
  the workflow today reflexively verifying a v1.x tag would have
  copied the stale example. Pure description text bump; the
  load-bearing `v0.X.Y+dirty` mention at :137 stays — that's a
  placeholder pattern reference, not a specific tag example.
- **`docs/adr/0009-resource-delta-sync.md` "ADR 013" pointer uses
  grep anchor.** ADR-0009's References section cited
  `internal/tools/common.go:50` for the legacy "ADR 013" inline-
  comment trail, but the audit-finding wave inserted
  `WebhookAllowedDomains` ahead of that field — the ADR-013
  string moved to `:77` (now sitting in the `EmitResourceUpdate`
  field doc). Replaced the stale line anchor with
  `git grep -n 'ADR 013'` plus symbol context (`EmitResourceUpdate`
  field doc, README index), matching the iter34/iter65/iter115/
  iter116/iter117 ADR-sweep pattern. The mcp/resources.go anchors
  in the same References block (:48-54, :148-198, :165-167) are
  still accurate — only the common.go pointer drifted. Same line-
  anchor failure mode as ADR-0007 (post-Wave-I main.go growth) and
  ADR-0008 (C2.2 runtime move).
- **`docs/adr/0007-fips-build-tag.md` line anchors replaced with
  grep anchors.** ADR-0007 had five line-anchored references for
  the FIPS startup hook: Decision §"Mandatory startup assertion"
  cited `cmd/clockify-mcp/main.go:48-51` (close to but not
  pinpoint at the `fipsStartupCheck()` call at line 54), plus
  References-section anchors at `:50`, `fips_on.go:22`,
  `fips_off.go:9`, and `.goreleaser.yaml:73`. All five replaced
  with symbol-name + `git grep` instructions following the same
  iter34/iter65/iter115/iter116 ADR-sweep pattern. Same drift
  class — line anchors go stale on post-write growth or refactors;
  the function name `fipsStartupCheck` and the goreleaser
  `clockify-mcp-fips` builder ID survive most reorganisations.
- **`docs/adr/0004-policy-enforcement-architecture.md` line
  anchors replaced with grep anchors.** Four line-anchored
  references had drifted: `policy.go:13-17` covered ReadOnly
  through Standard but missed the `Full` constant at line 18
  (the canonical enum has five modes per iter99 finding);
  `policy.go:91-115` (`IsAllowed` switch) and
  `policy.go:174-181` (introspection allowlist) had drifted to
  `:93-…` and `:184-193` respectively after post-write growth;
  `enforcement.go:65-...` (`BeforeCall`) and `dryrun.go:62-69`
  (`Action` enum) stayed accurate but were also pinned to fragile
  line numbers. Switched all five anchors to symbol-name +
  `git grep` instructions matching the iter34/iter65 ADR sweep
  pattern. Same drift class as iter115 (ADR-0008): line anchors
  go stale on post-write code growth or when const blocks gain
  new entries; symbol-grep anchors survive most reorganisations.
- **`docs/adr/0008-grpc-auth-interceptor.md` References section
  uses grep anchors instead of stale line numbers.** The ADR's
  References section listed five line-anchored cross-references
  for the legacy "ADR 012" inline comment hits, plus two
  per-line code anchors. The line numbers had drifted post-write
  in every cited file: cmd/clockify-mcp/main.go:251 doesn't even
  exist (the file moved most content to `internal/runtime/grpc.go`
  during the dea1cc3 C2.2 extraction; current file is 236 lines),
  scripts/check-build-tags.sh hits actually live at :4 and :72
  (was :68), values.yaml at :85 (was :81), deployment.yaml at
  :51 (was :48), config_test.go at :442 (was :282). Pattern is
  identical to the iter34/iter65 ADR sweeps that switched
  ADR-0002/0003/0004/0005/0006 from line anchors to function-
  name + grep-string anchors. References section now points at
  symbol names (`authStreamInterceptor`, `streamNotifier`) and
  recommends `git grep -n "ADR 012"` for the inline-comment
  trail; the cmd/clockify-mcp move is documented inline so
  CHANGELOG-trace continuity is preserved. Pure operator-doc fix.
- **`docs/runbooks/production-incident-drill.md` references the
  real risky-config log line.** The drill's "Metrics-auth drift"
  step told operators to "Scan for `msg=metrics_auth_mode_unsafe`
  log lines" but no such log message exists in the codebase. The
  actual warning emitted at startup when
  `MCP_HTTP_INLINE_METRICS_ENABLED=1` is paired with
  `MCP_HTTP_INLINE_METRICS_AUTH_MODE=none` is
  `msg=risky_config risk=inline_metrics_no_auth`
  (`internal/runtime/legacy_http.go:48-53`). An operator running
  the drill would have grepped for a string that never appears
  and concluded the drift wasn't present — masking exactly the
  failure mode the drill exists to surface. Step now names the
  real log line plus the file/lines that emit it. Pure
  operator-doc fix; closes a fictional-string drift the same way
  iter103 closed `--dry-run` and iter105 closed `make smoke-http`.
- **`docs/release-policy.md` SHA256SUMS.txt described as
  unsigned manifest, not "signed" file.** The Release artifacts
  trailer said "A signed `SHA256SUMS.txt` covering every binary
  in the release", but the canonical goreleaser config has
  `signs: cosign-keyless` with `artifacts: binary` — only the
  binaries are cosign-signed (per-binary `.sigstore.json`
  bundles). The checksum file itself is unsigned; its role is
  letting `sha256sum -c` cross-check downloads against
  goreleaser's staged hashes once a binary is independently
  verified via cosign. iter110's verify-release.md fix already
  pivoted to per-binary verification — this commit closes the
  parallel claim in release-policy.md so a reviewer auditing
  release-artifact provenance no longer infers a signature on
  SHA256SUMS.txt that doesn't exist.
- **`README.md` Troubleshooting "Stale tool list" entry matches
  reality.** The Troubleshooting line said "Stdio clients receive
  `notifications/tools/list_changed` after activation; HTTP
  clients must re-fetch `tools/list`." Same drift iter111 fixed
  in `docs/clients.md` — only legacy `http` is POST-only;
  `streamable_http` clients get the same notifications via the
  SSE stream on `GET /mcp`. Line now distinguishes the two
  transports so a streamable_http operator hitting "stale tool
  list" doesn't waste time implementing manual re-fetch logic
  the server already pushes.
- **`docs/clients.md` Tool Discovery distinguishes
  `streamable_http` from legacy `http`.** The Tool Discovery
  section grouped all "HTTP Clients" together with "Must
  manually re-fetch the tool list or handle session-based tool
  visibility updates." That was correct only for the legacy
  `http` transport, which is POST-only and carries no
  server-initiated notifications. The `streamable_http` transport
  (the spec-strict default for new HTTP deployments) DOES
  deliver `notifications/tools/list_changed` via the SSE stream
  on `GET /mcp` — see
  `internal/mcp/transport_streamable_http.go:159-166` and the
  Streamable HTTP 2025-03-26 §3.3 reference. A streamable_http
  client following the doc would have implemented unnecessary
  manual re-fetch logic and missed the spec-canonical SSE flow
  the server provides for free. Section now has three bullets
  (Stdio, `streamable_http`, legacy `http`) describing each
  transport's notification model accurately.
- **`docs/verify-release.md` recipe aligned with actual release
  artifacts.** The verify-release recipe was structurally
  drifted across §1, §2, §3, §4, §6, and §7 against the goreleaser
  config and `scripts/check-release-assets.sh`:
  (1) §1 listed `<name>.intoto.jsonl` SLSA files alongside
      binaries — those don't ship; SLSA goes to the GitHub
      attestation service via `actions/attest-build-provenance`.
  (2) §2 used `cosign verify-blob --bundle SHA256SUMS.bundle
      SHA256SUMS` — neither file exists; goreleaser only signs
      `artifacts: binary` (per-binary `.sigstore.json`), and the
      checksum file is `SHA256SUMS.txt`.
  (3) Example "success looks like" listed
      `clockify-mcp_1.2.0_linux_amd64.tar.gz` — releases ship
      raw binaries (`clockify-mcp-linux-x64`, no version, no
      archive wrapper).
  (4) §3 used `slsa-verifier verify-artifact` against an
      `.intoto.jsonl` file — the actual flow is `gh attestation
      verify` against the binary itself (matching iter102's
      verification.md fix).
  (5) §4 SBOM commands referenced
      `clockify-mcp_1.2.0_linux_amd64.spdx.json` — actual
      filename is `clockify-mcp-linux-x64.spdx.json`.
  (6) §6 release-gate pseudocode and §7 failure-mode list both
      named slsa-verifier; both updated to gh-attestation flow.
  Recipe is now self-consistent with what `gh release download`
  actually returns. Operators following it verbatim no longer
  hit "no such file" / "unknown subcommand" errors. Same drift
  class as iter95/iter103/iter105 (commands have to match what
  exists), but covering substantially more surface than a
  single-line fix.
- **`SECURITY.md` "Zero external dependencies" bullet matches
  the workspace reality.** The Security Features bullet said
  "minimal supply chain attack surface; no `go.sum` at all" but
  the repo root has had a 61-line `go.sum` since v1.0.0
  (`2e5d258`) — it covers the build-tagged sub-module dependencies
  under `go.work` (grpc, postgres, otel). The "no go.sum at all"
  framing was correct for the default binary's attack surface
  (root `go.mod` has zero external `require` lines) but
  misleading at the file level. Bullet now states "Zero external
  dependencies in the default binary", names the build-tagged
  sub-modules that bring in deps under their own `go.mod`, and
  notes that the root `go.sum` covers those for reproducibility
  but does not apply to the default-binary surface. Pure
  operator-doc fix; reviewer auditing supply chain claims sees
  the accurate scope rather than a literal-false statement.
- **`docs/upgrade-checklist.md` Config diff names spec.go as
  authoritative.** The Config diff command grepped for
  `os.Getenv` patterns in `internal/config/config.go` — that
  finds ~65 of the 79 EnvSpec entries (the rest are read through
  helpers like `getEnvDuration`/`getEnvBool` or live in
  `internal/config/profile.go`). An operator running the grep
  verbatim before an upgrade would have missed 14 env vars,
  including profile-bundle-default keys that change behaviour
  silently. Section now keeps the quick-skim grep command but
  adds a second `git diff` against `internal/config/spec.go` —
  the canonical EnvSpec — and explicitly labels which command
  is authoritative. Pure operator-doc fix; the upgrade pre-
  flight no longer pretends config.go is exhaustive.
- **`docs/coverage-policy.md` per-package floors realigned with
  `scripts/check-coverage.sh`.** The Current floors table was
  anchored at "as of 2026-04-13" but six floors had been
  ratcheted up in the canonical FLOORS_DEFAULT since: clockify
  70%→73%, enforcement 85%→88%, authn 85%→87%, timeparse
  88%→94%, tracing 95%→99%, vault 92%→94%. A reviewer reading
  the doc to assess "is the test suite as strict as advertised"
  would have inferred slacker floors than CI actually enforces.
  Table now mirrors `FLOORS_DEFAULT` exactly, drops the brittle
  date stamp (the script is the source of truth), and marks each
  ratcheted entry. Pure operator-doc fix.
- **`docs/future/observability-correlation.md` Reference Points
  cites `make build-tags`, not `make verify-tags`.** The
  "Reference points in the current code" table named the Make
  target as `make verify-tags under otel tag`, but no
  `verify-tags` target exists in the Makefile. The actual target
  is `make build-tags` (Makefile L130 area; runs
  `SKIP_FIPS=1 bash scripts/check-build-tags.sh`), and the script
  exercises the `otel` build tag at `scripts/check-build-tags.sh:80`.
  A future contributor reading this future-pointer doc to find
  the right verification entry-point would have hit
  "no rule to make target". Cell now names the real target with
  a one-line description of what it actually exercises. Pure
  doc-vs-Makefile drift, low-impact (future-pointer doc) but
  same iter103/iter105 class — fictional commands silently turn
  guidance into noops.
- **`docs/release/deploy-readiness-checklist.md` references the
  real `make http-smoke` target.** Pre-Flight Tests section said
  "Run `make smoke-http` against a staging instance" but the
  Makefile target name is `http-smoke` (`Makefile` L137:
  `http-smoke:` followed by `bash scripts/smoke-http.sh`). The
  word order was reversed in the checklist; `make smoke-http`
  would have failed with "no rule to make target". Same iter103-
  class fix as the `--dry-run` flag — operator-facing checklist
  items have to match the actual Makefile/CLI surface or the
  pre-production gate becomes a no-op. Item now references both
  `make http-smoke` and `make stdio-smoke` so operators on either
  transport hit the right entry point. Pure operator-doc fix.
- **`README.md` Configuration section bumps env-var count to 75+.**
  The Configuration teaser said "Run `clockify-mcp --help` for the
  complete list (60+ variables ...)" but
  `internal/config/spec.go` registers 79 EnvSpec entries today.
  Tighter bound now reads "75+ variables" and adds "webhook DNS
  validation" to the topical-coverage list since iter40 added the
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS` and
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` controls. Pure operator-doc
  fix; closes a stale-floor count drift between README copy and
  the canonical EnvSpec.
- **`docs/release/deploy-readiness-checklist.md` references real
  doctor commands instead of fictional `--dry-run` flag.** The
  Pre-Flight Tests section had a "Postgres Migration: Run
  `clockify-mcp --dry-run` against a production clone to verify
  database migrations" item, but `--dry-run` is not a CLI flag —
  the binary's subcommand is `clockify-mcp doctor` (with
  `--strict` and `--check-backends` modifiers per
  `cmd/clockify-mcp/main.go:225`). An operator following this
  checklist verbatim would have hit "unknown flag" and either
  skipped the verification or improvised. Item replaced with the
  two real commands from `public-hosted-launch-checklist.md`:
  config-only `clockify-mcp doctor --strict` for the default
  binary and `clockify-mcp-postgres doctor --strict
  --check-backends` for hosted Postgres deployments (ADR-0001
  keeps `pgx` out of the default binary so only the Postgres-
  tagged variant satisfies the backend gate). Pure operator-doc
  fix — but a high-impact one because the wrong flag turns the
  pre-production gate into a noop.
- **`docs/verification.md` SLSA section now says "all 15 binaries"
  instead of "five binaries".** Same iter101 drift — the SLSA
  build-provenance section claimed `release.yml` stages "the five
  binaries" but the workflow stages 15 (full matrix in
  `scripts/check-release-assets.sh`). A reader following the
  verification recipe to confirm SLSA coverage on a non-default
  binary (e.g. the Postgres-tagged binary backing their hosted
  deployment, or a FIPS variant) would have inferred the
  attestation only covers the default 5 — but every binary
  carries its own SLSA attestation. Section now names all five
  tag combinations and points at the canonical platform list.
  Pure operator-doc fix; closes the verification-side of iter101.
- **`docs/release-policy.md` Release Artifacts now lists all 15
  binaries.** The Release Artifacts section claimed "Five
  binaries" but the canonical platform matrix in
  `scripts/check-release-assets.sh` produces 15 binaries across
  five tag combinations: 5 default (darwin/linux × arm64/x64
  + windows-x64.exe), 4 FIPS-tagged (UNIX only — no Windows FIPS
  toolchain), 2 Postgres-tagged (linux only — backs
  shared-service `doctor --check-backends`), 2 gRPC-tagged
  (linux only — `private-network-grpc` profile), and 2
  gRPC+Postgres (hosted gRPC shape). Section now enumerates each
  group with its naming pattern and operator-facing role, plus
  notes that SLSA was conditional pre-2026-04-22 (per ADR-0013)
  and mandatory after the repo flipped public. Closes a
  multi-binary drift that affected operators reading
  release-policy as the canonical artifact contract — anyone
  expecting a Postgres binary or gRPC variant would have
  concluded they didn't ship.
- **`docs/production-readiness.md` Governance audit-trail
  description now matches `GOVERNANCE.md`.** The Governance
  section claimed the audit trail came from "signed commits,
  public CI logs, and SLSA build provenance on every release."
  Two factual problems: signed-commit enforcement on `main` is
  intentionally disabled (per `docs/branch-protection.md` L57:
  "Signed commits: disabled" — the single-maintainer reality
  documented in ADR-0016), and SLSA is conditional, not
  "every release" (per ADR-0013: SLSA depends on the GitHub
  attestation service which was unavailable while the repo was
  user-owned-private). GOVERNANCE.md L28-33 lists the actual
  audit-trail components: public CI logs, GitHub web-flow signed
  squash commits on `main` where available, SLSA where
  available, and the release-smoke workflow. Section now uses
  the same enumeration. Pure operator-doc fix; reviewer
  evaluating audit-trail claims now sees consistent language
  across both surfaces.
- **`CONTRIBUTING.md` Project Structure section now reflects the
  real `internal/` package tree.** The ASCII tree listed 13
  internal packages but `ls internal/` returns 28. Missing
  load-bearing entries: `runtime/` (extracted in C2.2 — the
  current entry-point wiring), `controlplane/` + `auditbridge/`
  (audit + tenant store + bridge), `transport/` (gRPC adapter),
  `paths/` (typed URL path builder), `authn/`, `vault/`,
  `logging/` (PII redaction), `metrics/`, `tracing/`,
  `jsonschema/`, `jsonpatch/`, `jsonmergepatch/`, `testharness/`,
  `benchdata/`. Also: the `policy/` line listed only four modes
  (`read_only/safe_core/standard/full`), missing
  `time_tracking_safe` — the recommended default for hosted-AI
  deployments per `docs/policy/production-tool-scope.md`. Tree
  now lists all 28 packages with one-line descriptions and the
  policy-mode list matches `internal/policy/policy.go:14-19`.
  New contributors landing on CONTRIBUTING.md no longer have to
  cross-reference `ls internal/` to find packages.
- **`docs/README.md` Release Trust section lists
  `public-hosted-launch-checklist.md`.** docs/release/ ships two
  checklists — `deploy-readiness-checklist.md` (general
  pre-production) and `public-hosted-launch-checklist.md` (the
  pre-flight gates for accepting traffic from clients you don't
  control, including the Postgres-binary backend check via
  `doctor --strict --check-backends`). The navigation index
  listed only the first; the public-launch checklist was on disk
  but absent from the index. Same doc-vs-filesystem drift pattern
  as iter96 (ADR index) and iter97 (runbooks index). Pure
  operator-doc fix.
- **`docs/README.md` Runbooks section lists all 12 runbooks.** The
  Runbooks section listed 10 entries but `docs/runbooks/` has 12
  files. The two missing entries are the runbooks that ship with
  the audit-finding wave: `hosted-error-sanitization.md` (Finding 9
  guidance for when sanitised errors hide upstream signal) and
  `webhook-dns-validation.md` (Finding 10 — DNS-rebinding guard +
  allowlist escape hatch). Both are referenced from SECURITY.md
  bullets but were never added to the docs/ navigation index.
  Section now lists all 12 in topic-affinity order. Same
  doc-vs-filesystem drift pattern as iter96's ADR README index
  fix. Pure operator-doc fix.
- **`docs/adr/README.md` index lists ADRs 0012-0016.** The ADR
  index table stopped at 0011 (Control-plane schema versioning)
  but the directory has five additional ADRs: 0012
  (Backward-compatibility policy, Accepted), 0013 (Private-repo
  SLSA posture, Superseded 2026-04-22 when the repo flipped
  public), 0014 (Production fail-closed defaults, Accepted),
  0015 (Profile-centric configuration model, Accepted), and
  0016 (Single-maintainer governance reality, Accepted). Index
  now lists all 16 ADRs and the status summary paragraph
  enumerates which are Accepted, which is Proposed (0010), and
  which is Superseded (0013) with the reason. Pure operator-doc
  fix; closes a doc-vs-filesystem drift that left contributors
  navigating from the index unaware that five recent ADRs
  existed.
- **`deploy/k8s/README.md` Observability section now describes the
  real endpoint surface.** The section claimed
  "Legacy `MCP_TRANSPORT=http` exposes three unauthenticated
  endpoints on the main listener: /health, /ready, /metrics" but
  the actual code disagrees on three points:
  (1) `/health` and `/ready` are mounted on **both** legacy http
      and streamable_http (`internal/mcp/transport_streamable_http.go:138-143`),
  (2) `/metrics` is **not** mounted on the main listener by default
      — it is gated by `MCP_HTTP_INLINE_METRICS_ENABLED`
      (`internal/mcp/transport_http.go:30-58`), and
  (3) when inline metrics are enabled, auth defaults to
      `inherit_main_bearer`, not "unauthenticated".
  Section rewritten to describe both transports' shared health
  endpoints, the recommended `MCP_METRICS_BIND` side-channel
  listener, and the inline-metrics opt-in with its three auth
  modes (`inherit_main_bearer` / `static_bearer` / `none`)
  matching the SECURITY.md "Inline /metrics security" bullet.
  Pure operator-doc fix; closes a year-old drift between the
  k8s deploy README and the actual transport code.
- **`docs/production-readiness.md` TLS-termination bullet matches
  reality.** Compliance posture had the same blanket "the HTTP
  transport does NOT terminate TLS by design" claim that iter93
  fixed in SECURITY.md — parallel surface drift. Bullet now
  distinguishes the proxy-fronted default (static_bearer / oidc /
  forward_auth) from in-process termination on `streamable_http`
  with `MCP_HTTP_TLS_CERT`/`_KEY` and `grpc` with
  `MCP_GRPC_TLS_CERT`/`_KEY`, plus the legacy-http
  config.Load rejection. Operator-facing parity restored across
  both canonical surfaces.
- **`SECURITY.md` TLS section now distinguishes proxy-terminated
  vs in-process TLS modes.** The TLS / HTTP Transport section
  blankly stated "The HTTP transport does **not** terminate TLS"
  with a "MUST front with proxy" mandate. That was correct only
  for the default static_bearer / oidc / forward_auth deployments;
  `streamable_http` with `MCP_HTTP_TLS_CERT` + `MCP_HTTP_TLS_KEY`
  and `grpc` with `MCP_GRPC_TLS_CERT` + `MCP_GRPC_TLS_KEY` both
  terminate TLS in-process — and mTLS-anchored deployments
  (per `support-matrix.md` line 21) rely on exactly that path
  via `internal/runtime/streamable.go:56-83`. Section now
  describes both modes accurately: default = proxy-fronted, with
  the explicit cert+key paths called out for the in-process
  termination case. Operators considering mTLS no longer have to
  cross-reference the support matrix to discover the in-process
  TLS option exists.
- **`.github/workflows/release-smoke.yml` workflow_dispatch tag
  example bumped to `v1.2.0`.** The manual-trigger input for the
  release-smoke workflow described the tag parameter as
  "Release tag to verify (e.g. v1.0.0)" — same drift pattern as
  iter78's `docs/verification.md` fix and iter89/iter90/iter91's
  v1.0.x → v1.2.x sweep across release-policy / SECURITY /
  production-readiness. Operators landing on the GitHub Actions
  UI to dispatch a re-verification would have copied the v1.0.0
  example by default. Tag bumped to v1.2.0 (current Active per
  SUPPORT.md). Pure operator-doc fix (workflow input
  description); no behaviour change.
- **`docs/production-readiness.md` Upgrade path no longer anchors
  at `1.0.x today`.** The Upgrade path section's short-version
  paragraph said "only the current minor (1.0.x today) is
  supported; when 1.1 ships, 1.0.x gets security-only fixes...".
  This was anchored at the v1.0.x → v1.1 era — same drift iter89
  caught in release-policy.md and iter90 caught in SECURITY.md
  Supported Versions. Paragraph rewritten in abstract terms
  (Active / Superseded / patch-only) and points at SUPPORT.md as
  the canonical state, so future minors do not restale the doc.
  Closes the iter48-era version sweep at the production-readiness
  surface — the four canonical "what is supported" docs
  (SUPPORT.md, release-policy.md, SECURITY.md,
  production-readiness.md) now all agree.

### Security

- **`SECURITY.md` Supported Versions table now lists `1.2.x` as
  Active.** The Supported Versions table was anchored at the
  v1.0.x → v1.1.x era — claiming 1.1.x and 1.0.x were the supported
  lines, with no entry for v1.2.x. Since v1.2.0 (2026-04-25) is
  the Active line per SUPPORT.md (and per iter89's release-policy
  realignment), an operator looking up "is my v1.2.0 receiving
  security fixes" would have concluded the Active line is **not**
  supported — the inverse of the truth. iter48's version-string
  sweep (a005f82) bumped SUPPORT.md but missed both
  release-policy.md (closed in iter89) and SECURITY.md (closed
  here). Table now lists 1.2.x (Active), 1.1.x (Superseded —
  upgrade), 1.0.x (patch-only on the stable v1 wire format), and
  0.x (EOL since v1.0.0). New prose explicitly tells operators on
  superseded minors to upgrade rather than wait for a backport,
  and points at SUPPORT.md as the canonical version-status state.
  Closes the iter48-era version sweep at the security-policy
  surface.

### Fixed

- **`docs/release-policy.md` Supported-versions table realigned
  with current minor cadence.** The release-policy table claimed
  `1.0.x` was Active and `0.x` was EOL — anchored at the v1.0.x
  era before v1.1.0 (2026-04-22) and v1.2.0 (2026-04-25) shipped.
  iter48 (a005f82) bumped SUPPORT.md to name v1.2.x as Active but
  release-policy was missed by that pass; the parallel section in
  release-policy continued to misrepresent the support window.
  Table now lists v1.2.x (Active), v1.1.x (Superseded), v1.0.x
  (Patch-only on stable v1 wire format), v0.x (EOL), matching
  SUPPORT.md exactly. Cadence example bullets switched from
  hardcoded 1.0.x → 1.0.x+1 to abstract `1.x.y` → `1.x.y+1` so
  the policy doc no longer goes stale every minor. Backport
  criteria intro line updated to describe the patch-only track
  generically rather than naming a specific version pair. Pure
  operator-doc fix; closes the iter48-era version-string sweep
  at the release-policy surface.
- **`docs/support-matrix.md` "Recommended" deploy-doc list now
  includes `private-network-grpc`.** The post-table sentence
  promised "Every 'Recommended' row has a corresponding file
  under `docs/deploy/`" but listed only three entries (local-stdio,
  single-tenant-http, production-profile-shared-service). The
  matrix has four Recommended rows: the fourth is "Private mesh,
  low-latency RPC" with `grpc + oidc or mtls + postgres://`,
  which has its corresponding file at
  `docs/deploy/profile-private-network-grpc.md`. List now
  enumerates all four. Pure operator-doc fix; closes the
  iter84-87 chain at the support-matrix level by giving every
  registered profile (except the prod-postgres alias which is
  documented inside production-profile-shared-service.md) a
  named pointer.
- **`docs/operators/README.md` landing page lists profile coverage +
  drops legacy-HTTP reference.** The operator-guide landing page
  described the self-hosted guide as covering "stdio or legacy
  HTTP" — same drift iter85 closed in self-hosted.md, persisting
  one level up at the index. Page now names the registered
  profile(s) each guide covers (shared-service.md →
  `shared-service` + `prod-postgres`; self-hosted.md →
  `local-stdio` + `single-tenant-http`), drops the legacy-http
  reference in favour of `streamable_http`, and notes the
  fifth profile (`private-network-grpc`) lives in deploy/
  rather than under either operator guide. Pure operator-doc
  fix; closes the iter84-86 chain at the operator landing-page
  level.
- **`docs/operators/shared-service.md` Canonical Configuration leads
  with `--profile=` apply commands.** The shared-service operator
  guide pointed operators at `deploy/examples/env.shared-service.example`
  directly, the same legacy pattern iter85 closed for the
  self-hosted guide. iter84 mapped this guide to the
  `shared-service` and `prod-postgres` profiles; this commit
  updates the doc to lead with both `--profile=` apply commands,
  enumerates the env defaults each one sets (matching profile.go's
  shared-service entry at line 59 and the prod-postgres entry at
  line 102), and re-frames the legacy env file as the
  Helm/Kustomize starting reference for operators populating
  values + secrets. Pure operator-doc fix.
- **`docs/operators/self-hosted.md` updated to the profile system +
  `streamable_http`.** The Architecture section recommended
  `MCP_TRANSPORT=stdio or http (Standard)` but the legacy `http`
  transport has been deprecated since v1.0.1 (covered by
  `MCP_HTTP_LEGACY_POLICY=deny` in the `single-tenant-http`
  profile). The Recommended Configuration section pointed at
  `deploy/examples/env.self-hosted.example` directly without
  referencing the Wave I profile system. iter84 mapped this guide
  to the `local-stdio` and `single-tenant-http` profiles; this
  commit updates the doc to match: Architecture now names
  `stdio` and `streamable_http` as the two transports (legacy
  `http` flagged as deprecated), Recommended Configuration leads
  with `--profile=local-stdio` and `--profile=single-tenant-http`
  apply commands, and the legacy env file is described as
  preserved-for-muscle-memory with a pointer to
  `deploy/profile-self-hosted.md` for the upgrade path. Pure
  operator-doc fix; no behaviour change.
- **`docs/README.md` Operator Guides section no longer says "two
  supported profiles".** The intro line conflated "operator-guide
  categories" (shared-service vs. self-hosted shapes) with
  "supported profiles" — but five canonical profiles are
  registered (local-stdio, single-tenant-http, shared-service,
  private-network-grpc, prod-postgres) and `private-network-grpc`
  is fully supported despite not having a dedicated operator
  guide. Iter81/82 caught the same conflation in adjacent doc
  surfaces; this is the third instance in the chain. Section
  intro now says guides are grouped by operator shape and all
  five profiles are supported, plus each guide bullet now names
  the profiles it covers (shared-service guide → `shared-service`
  + `prod-postgres`; self-hosted guide → `local-stdio` +
  `single-tenant-http`). `private-network-grpc` is intentionally
  not under either operator guide — it is documented in the
  Deployment Profiles section instead. Pure operator-doc fix.
- **`SECURITY.md` + `docs/production-readiness.md` Response limits
  claim now reflects the 4 MB default.** Both surfaces stated
  "2MB default on HTTP request bodies" — pinning back to the v0.6-
  era streamable-HTTP fallback. The actual default has been
  `MCP_MAX_MESSAGE_SIZE=4194304` (4 MB) since v1.0.1's transport
  consistency standardisation (commit 13d2a0c); the 2 MB literal
  in `internal/mcp/transport_streamable_http.go` is an inner-loop
  fallback that never fires once the runtime wires `MaxBodySize`
  from the config (`internal/runtime/streamable.go:71`).
  production-readiness.md also pointed operators at
  `MCP_HTTP_MAX_BODY` which has been the deprecated alias since
  v1.0.1 — primary knob is `MCP_MAX_MESSAGE_SIZE`. Both files
  now state 4 MB and name the primary env var, with the
  deprecated alias parenthesised for back-compat readers. Pure
  operator-doc fix.
- **`internal/config/profile.go` package doc no longer claims a
  1:1 profile↔doc mapping.** The Profile type comment said "The five
  canonical profiles map onto the five docs/deploy/ profile notes" —
  but the mapping is not 1:1: prod-postgres has no dedicated doc
  (covered inline in production-profile-shared-service.md as an
  alias) and docs/deploy/profile-self-hosted.md exists for a legacy
  shape without any registered profile. iter81 surfaced the same
  drift on the docs/README.md side; this is the symmetric fix at
  the source-comment layer. Comment now enumerates the five
  registered names and notes the prod-postgres alias inline +
  the self-hosted legacy-shape pointer. Pure code-comment fix.
- **`docs/README.md` profile list realigned with canonical
  registry.** The Start Here block claimed "the five canonical
  deployment profiles (`local-stdio`, `single-tenant-http`,
  `shared-service`, `private-network-grpc`, `self-hosted`)" — but
  `internal/config/profile.go`'s `allProfilesSlice` registers five
  profiles where the fifth is `prod-postgres`, not `self-hosted`.
  `self-hosted` is a documented shape (covered by
  `deploy/profile-self-hosted.md`) but is not a valid `MCP_PROFILE`
  value — `clockify-mcp --profile=self-hosted` would fail with
  "unknown profile". Doc now matches the registry, plus a
  parenthetical pointing at the legacy-shape upgrade-path doc so
  operators searching for "self-hosted" still land somewhere
  useful. Pure operator-doc fix.
- **`README.md` Compatibility table now lists all four MCP
  protocol versions.** The Compatibility table's MCP Protocol row
  named `2025-11-25` plus back-compat for `2025-06-18` and
  `2025-03-26` — but the canonical
  `internal/mcp/server.go` `SupportedProtocolVersions` slice
  advertises four entries; the oldest (`2024-11-05`) was missing.
  iter79's clients.md fix touched the prose paragraph but missed
  the table row in README. Both surfaces now match the source
  array. Pure operator-doc fix; clients reading the table for the
  back-compat floor learn the correct lowest version.
- **`docs/clients.md` Backwards Compatibility section now lists all
  four supported MCP protocol versions.** The Backwards Compatibility
  blurb claimed support for `2025-06-18`, `2025-03-26`, and
  `2024-11-05` — but `internal/mcp/server.go`
  `SupportedProtocolVersions` actually advertises four versions
  (the latest being `2025-11-25`). The MCP protocol-version compat
  test (`internal/mcp/protocol_version_compat_test.go`) covers all
  four against the same source of truth, so the doc was the only
  drift point. Section now matches `SupportedProtocolVersions` and
  adds an explicit cross-reference so future protocol-version
  additions land both places. Pure operator-doc fix; clients reading
  the supported-versions list to plan their handshake floor now see
  the correct ceiling.

### Added

- **`docs/verification.md` example tags bumped to v1.2.0.** The
  three supply-chain verification recipes (SLSA build provenance,
  cosign keyless on the binary, cosign on the container image) plus
  the §5 SBOM download example all hard-coded `TAG=v1.0.0` /
  `gh release download v1.0.0`. iter52 (720a1e0) bumped the parallel
  `docs/verify-release.md` to v1.2.0 (current Active per SUPPORT.md)
  but missed `docs/verification.md`. Operators copy-pasting the
  examples were verifying the v1.0.0 GA release rather than the
  current line. All four blocks now use `TAG=v1.2.0`; the
  intentional "Bundle format (v1.0.0 vs v1.0.1+)" historical note
  in §2 is preserved since it documents why offline verify-blob
  specifically fails on the v1.0.0 legacy rekor-bundle artifact.
  Lead paragraph also picks up an explicit SUPPORT.md cross-link
  matching the iter49 pattern. Pure operator-doc fix.
- **`docs/tool-catalog.md` adds Audit-tracked argument capture
  section.** Iter27 (97c20da) emitted `risk_class` and `audit_keys`
  on every tool descriptor in `docs/tool-catalog.json`; iter31
  (c70360a) added the Risk column to the markdown rendering, but
  `audit_keys` had no markdown surface — readers without `jq`
  couldn't see which tools record action-defining arguments
  (role, status, quantity, unit_price) alongside the default `*_id`
  capture in audit events. New focused section after the Tier-2
  tables lists the 19 tools that carry `audit_keys`, sorted by
  tier+name. Symmetric completion of the iter27→iter31 catalog
  rendering chain; pure docs change. Compliance reviewers now have
  a one-screen view of which mutations emit enriched audit events.
- **`SECURITY.md` Dry-run bullet expanded to cover non-destructive RW.**
  The Security Features "Dry-run" entry was scoped to "every
  destructive operation", but audit Finding 7 added `dry_run:true`
  support to four non-destructive RW tools whose execution triggers
  an external side effect: `clockify_send_invoice`,
  `clockify_mark_invoice_paid`, `clockify_test_webhook`, and
  `clockify_deactivate_user`. Each handler calls
  `dryrun.Enabled(args)` at the top of the body and returns a wrapped
  GET preview without issuing the PUT/POST. `docs/clients.md`
  documented this in the "Safety and Destructive Operations" section,
  but SECURITY.md as the canonical security summary missed the
  propagation. Bullet now names all four tools inline so an auditor
  reading the security surface gets the staged-preview path without
  cross-referencing clients.md. Pure operator-doc fix; closes the
  iter40-era doc-sync chain at the security-summary level.
- **Cited image-pin examples bumped to v1.2.0 + linked to SUPPORT.md.**
  Two stale operator-pointers found by re-grepping for version
  strings: `deploy/k8s/README.md` (pin example was `v0.5.0` —
  pre-v1 placeholder) and `docs/production-readiness.md` (image-
  tag column was `v1.0.0` — pre-Wave-G). Both now name `v1.2.0`
  (the current Active line per SUPPORT.md) and add an explicit
  link to SUPPORT.md so future readers find the canonical
  current line directly. Pure operator-doc fix; no behaviour
  change. Companion to a005f82 (the SUPPORT.md realignment).
- **Phantom `docs/observability.md` references repointed.**
  Four references promised an operator doc that was never
  written: `cmd/clockify-mcp/otel_on.go` (the OTel install
  failure hint), `deploy/helm/README.md` (PrometheusRule
  Purpose column), `deploy/k8s/README.md` (prometheus-rule.yaml
  description), and `docs/adr/0006-otel-build-tag.md`
  (Related docs). Same situation as iter56's pprof phantom-
  runbook find. All four rewritten to reference the actual
  artifacts: the OTel hint now spells out what to check
  (`OTEL_EXPORTER_OTLP_ENDPOINT` + collector reachability)
  instead of deferring to a 404; the helm/k8s table cells now
  point at the real `prometheus-rule.yaml` /
  `prometheusrule.yaml` files as the canonical alert
  definitions; the ADR's Related-docs block points at the
  actual ServiceMonitor + PrometheusRule template files.
  Pure operator-doc fix; reviewers no longer chase a phantom
  observability doc.
- **SECURITY.md drops broken `docs/safe-usage.md` pointer; future
  observability plan adds historical-context note.** SECURITY.md
  §"TLS / HTTP Transport" tail-pointed at `docs/safe-usage.md`
  for the full INSECURE scope, but that doc existed in the v0.6
  era and isn't tracked in current main. Replaced with the actual
  full-scope sentence inline (hosted profiles refuse INSECURE at
  startup; only `local-stdio` / `single-tenant-http` honour it).
  `docs/future/observability-correlation.md` also referenced
  `docs/safe-usage.md` as a future landing page; rewrote to
  reference the current `docs/operators/` home + a parenthetical
  noting the original target is no longer in the repo (preserves
  the planning trail without sending future authors at a 404).
- **`internal/controlplane/COMPAT.md` retention-reaper pointer
  unstuck from pre-C2.2 path.** The compat-matrix row for
  `RetainAudit(ctx, maxAge)` cited `cmd/clockify-mcp/retain.go`
  as the call site — that file was moved to
  `internal/runtime/retain.go` during the dea1cc3 C2.2 runtime
  extraction (along with `RetainAuditOnce` → `RetainAuditLoop`).
  Compat-matrix readers now land on the actual current call site
  and get a one-line note about the historical move so the
  CHANGELOG reference resolves.
- **`internal/mcp/resources.go` ADR-0009 reference renumbered.**
  The `ResourceUpdateDelta` doc-comment pointed reviewers at
  `docs/adr/013-resource-delta-sync.md` — wrong ADR number,
  AND missing the canonical 4-digit zero-pad. The actual ADR is
  `docs/adr/0009-resource-delta-sync.md`. Pure typo fix; reader
  following the link no longer hits a 404.
- **`pprof_on.go` / `pprof_off.go` drop reference to never-written
  `oom-or-goroutine-leak.md` runbook.** The original feat commit
  (da2fa8b — "pprof endpoints behind -tags=pprof") promised an
  operator runbook at `docs/runbooks/oom-or-goroutine-leak.md`
  but the file was never authored. Two source comments still
  pointed at the 404. Both rewritten to inline the security
  caveat directly: pprof_off.go points at the pprof_on.go
  doc-comment for the full security rationale; pprof_on.go now
  spells out *what* `/debug/pprof/heap` and `/debug/pprof/goroutine`
  leak (process layout, allocation patterns, handler frame
  strings) instead of deferring to a phantom runbook. Operators
  reading the source now get the why-trusted-network-only
  explanation inline.
- **`SECURITY.md` Config validation bullet flags hosted-profile
  refusal of `CLOCKIFY_INSECURE=1`.** The bullet described the
  override as universally available ("non-HTTPS BASE_URL rejected
  unless loopback or explicitly opted in with INSECURE=1"), which
  is misleading for hosted profiles where the override is refused
  outright at startup. The TLS / HTTP Transport section already
  documents the refusal — added an inline cross-reference so a
  reader scanning the Security Features list doesn't reach a
  wrong inference about what overrides are honoured. Pure
  operator-doc fix.
- **`SECURITY.md` adds Audit fidelity bullet for
  `RiskClass`/`AuditKeys`.** Existing Audit durability bullet
  described persistence semantics (fail_closed vs best_effort)
  but never the *content* improvement from audit Finding 8 —
  `RiskClass` bitmask recorded on every event, `AuditKeys`
  causing the recorder to capture action-defining arguments
  (role, status, quantity, unit_price) alongside the `*_id`
  fields. Closes the audit-completeness gap at the security-
  summary level: operators auditing for compliance reviews
  now see that audit events capture *what change* was applied,
  not just *what was touched*.
- **`SECURITY.md` Config validation entry covers
  `CLOCKIFY_WORKSPACE_ID` startup validation.** The Config
  validation bullet only mentioned the `CLOCKIFY_BASE_URL` /
  `CLOCKIFY_INSECURE` check; it never gained the workspace-ID
  startup gate from audit Finding 4 (the gate that fails config
  load rather than silently propagating path-traversal-shaped IDs
  like `bad/path` or `bad?query` into every `/workspaces/{id}/...`
  URL). Bullet now describes both checks. Pure operator-doc fix;
  closes another iter40-era doc-sync gap at the security-summary
  level.
- **`SECURITY.md` adds Hosted-profile error sanitisation bullet.**
  The Security Features list never gained the
  `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS` entry that shipped with the
  audit-finding wave (closes Finding 9 — per-tenant identifier
  leakage via 4xx response bodies). New bullet describes both the
  profile default (on for `shared-service` / `prod-postgres`) and
  the operator override, plus the slog-side preservation for
  debugging. Operators reading SECURITY.md for compliance reviews
  now see the per-tenant data-leak prevention surface.
- **`SECURITY.md` Webhook URL validation entry brought to current
  state.** The Security Features bullet described only the
  pre-audit literal-IP check (which has been the baseline since
  v0.x). Missing: the hosted-profile DNS-resolve gate that ships
  with shared-service / prod-postgres (closes the literal-IP-only
  gap that would let `metadata.google.internal` style hostnames
  resolve to `169.254.169.254` past the literal check), the
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS` operator override, and the
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` allowlist escape hatch.
  Bullet now describes all three layers. Pure operator-doc fix;
  closes the iter40-era doc-sync chain at the security-summary
  level.
- **`docs/clients.md` Hosted-Mode Webhook URL Validation section
  references the allowlist escape hatch.** Iter40 added
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` and iter41 propagated it into
  the shared-service operator docs (00b5561), but the
  client-facing `docs/clients.md` section still described the DNS
  gate without the escape hatch. Clients seeing an unexpectedly-
  accepted webhook in a split-horizon environment had no signal
  whether they were watching a security regression or an operator-
  admitted hostname. Section now explains: operators can opt
  specific hostnames out via the env var, and a successful
  allowlist hit is not a regression. Pure operator-doc fix; no
  behaviour change. Closes the iter41-era doc-sync deferment.
- **Reproducibility workflow drops phantom `docs/reproducibility.md`
  pointers.** `.github/workflows/reproducibility.yml` had two
  references to a `docs/reproducibility.md` operator doc that was
  never authored — the workflow's own comment block already
  contains the full 6-step recipe, making the external pointer
  redundant. Both repointed to "this comment block": the recipe
  intro now says "full detail in this comment block"; the
  failure-recovery instruction now says "this comment block
  should be updated to document the gap" instead of pointing at
  a 404. Pure operator-doc fix.
- **Phantom `docs/troubleshooting.md` references dropped.** Two ADRs
  (0004 + 0005) listed `docs/troubleshooting.md` in their Related-docs
  blocks; ADR-0005 even named a specific section ("Stale tool list").
  The doc was created in 40ef8d3 (2026-04-11) but isn't in current
  main, and no successor runbook covers the same scope (the `docs/
  runbooks/` directory has per-symptom pages but no general
  symptom→fix matrix). Both Related-docs entries dropped; the
  remaining items in each block (`README.md` "Policy modes" /
  "Tool tiers", `docs/production-readiness.md`) carry the load.
  Same per-iteration pattern as iter56–60: when a doc was promised
  but is no longer in the tree and has no successor, drop the
  pointer rather than recreate the doc unilaterally.
- **ADR-0004 wiring pointers unstuck from pre-C2.2 path.** Two
  references in `docs/adr/0004-policy-enforcement-architecture.md`
  pointed at `cmd/clockify-mcp/runtime.go` as the place where
  `Pipeline` is installed on `mcp.Server`. That file was removed
  in the dea1cc3 C2.2 runtime extraction; the actual wiring lives
  in `internal/runtime/service.go` `buildServer()`. Both repointed
  with a one-line C2.2 historical note (same pattern as ADR-0005's
  iter37 fix in 7e120d4). Closes the per-ADR sweep (ADR-0004 was
  missed by the iter34-38 wave because its grep didn't surface
  the `cmd/clockify-mcp/runtime.go` references).
- **Makefile `mutation` target repointed to current floor source.**
  The target's comment block told operators to read
  `docs/testing/mutation-floors.md` for the per-package gremlins
  efficacy floors. That standalone runbook was retired; the floors
  themselves live inline in `.github/workflows/mutation.yml`'s
  top-of-file comment table + matrix entries (the workflow IS the
  source of truth, not a doc that mirrored it). Repointed the
  Makefile comment to the workflow file. Pure doc fix; no
  behaviour change.
- **`internal/transport/grpc/codec.go` package-doc gRPC entry-point
  pointer unstuck from pre-C2.2 path.** The package comment told
  reviewers to read `cmd/clockify-mcp/grpc_on.go` to see how the
  separate go.mod is reached under `-tags=grpc`. That file no
  longer exists — the gRPC dispatcher was extracted into
  `internal/runtime/grpc.go` (`runGRPC`) during the dea1cc3 C2.2
  refactor, and `Runtime.Run` selects it when `MCP_TRANSPORT=grpc`.
  Repointed to the actual current path. Pure doc fix; no
  behaviour change.
- **`deploy/k8s/base/deployment.yaml` drops dead
  `docs/audit-chart-vs-config.md` pointer.** The gRPC-transport
  comment block listed both the Helm chart's deployment.yaml
  AND the audit doc as references for the full gap list. The
  audit doc was a W4-era internal tracker (mentioned in
  CHANGELOG history alongside "all 22 gaps closed") that's no
  longer in main. Dropped the second pointer; the Helm chart's
  deployment.yaml is the actionable reference for the
  multi-port + tcpSocket-probe gRPC pattern, and that pointer
  remains.
- **Fourth `w2-12-digest-pinning.md` reference cleared (sweep
  closed).** Iter55 (c5f1bdd) repointed three call sites
  (Dockerfile + check-overlay-structure.sh ×2) but missed the
  fourth in `deploy/k8s/overlays/prod/kustomization.yaml`'s
  policy block-comment. Now repointed to
  `image-digest-pinning.md`. Pure pointer fix; closes the
  iter55 sweep.
- **Stale `w2-12-digest-pinning.md` runbook references repointed.**
  The runbook was renamed `w2-12-digest-pinning.md` →
  `image-digest-pinning.md` (already documented in CHANGELOG)
  but three call sites still referenced the old name and pointed
  operators at a 404: `deploy/Dockerfile` (the comment near the
  base-image digest pin), and two sites in
  `scripts/check-overlay-structure.sh` (the policy block-comment
  and — most impactful — the operator-facing error message that
  prints when the structural guard trips). All three repointed
  to the current filename. Pure operator-doc-pointer fix; no
  behaviour change. The check-overlay-structure.sh error message
  in particular was a real bug operators would hit.
- **`deploy/helm/README.md` image.tag default cell corrected.**
  The Highlights table claimed `image.tag` defaults to `0.7.0`,
  which was wrong on two axes: (a) `values.yaml` actually
  defaults `tag: ""` and the chart template falls back to
  `.Chart.AppVersion` when blank — there is no literal default
  tag; (b) `0.7.0` doesn't match any current release and
  predates the v1.0 wire-format guarantee. Replaced with the
  accurate `""` (falls back to `.Chart.AppVersion`) so
  operators reading the table get a true picture of the chart's
  default behaviour.
- **`docs/verify-release.md` image name + example tag corrected.**
  Three image references named `ghcr.io/apet97/clockify-mcp` —
  the registry path is `ghcr.io/apet97/go-clockify` (the same
  one referenced in `verification.md`, the deploy templates,
  the k8s manifests, and the certificate-identity-regexp on
  the very next line). Operators following the verify-release
  guide would `crane digest` / `cosign verify` against a
  non-existent image and hit a 404 immediately. All three
  occurrences renamed to `go-clockify`. Same file: the
  example `TAG=v0.7.1` (and downstream literal artifact
  names like `clockify-mcp_0.7.1_*.tar.gz`) bumped to
  `v1.2.0` so the doc-as-runbook produces a verifiable result
  end-to-end without manual substitution. The image-name fix
  is a real bug operators would hit; the tag bump is the
  doc-currency hygiene that keeps the runbook copy-paste-able.
- **`docs/runbooks/image-digest-pinning.md` examples bumped to v1.2.0.**
  Three operator-facing copy-paste examples (the
  `docker buildx imagetools inspect` digest-resolution command,
  the Argo CD `images:` override, and the Flux `newTag:`
  override) all named `v1.0.0`. Operators following the
  runbook today should pin the current Active line — bumped
  to `v1.2.0`. The historical narrative line ("the overlay was
  stuck at 0.7.0 while the base pointed at v1.0.0") is left
  as-is since that's documenting a past incident, not a
  prescription.
- **k8s base manifest + Helm chart realigned to v1.2.0.**
  `deploy/k8s/base/deployment.yaml`'s pinned image was still
  `v1.0.0` — the comment on that line says "Bump on release"
  but the bump was missed for both v1.1.0 (2026-04-22) and
  v1.2.0 (2026-04-25). Operators applying the base verbatim
  via `kubectl apply -k deploy/k8s/base` were getting a
  release without the audit-finding security wave. The Helm
  chart's `version` and `appVersion` were similarly stuck at
  `1.0.0` — `helm list` would then show that misleading
  appVersion alongside whatever image tag the operator
  actually deployed. Both bumped to `v1.2.0`. Chart-comment
  guidance corrected: only Chart.yaml needs editing on
  release because `cmd/clockify-mcp/main.go` reads its
  version via ldflags + `debug.ReadBuildInfo()`, not from a
  literal source string.
- **`SUPPORT.md` version matrix realigned to v1.2.x as Active.**
  The matrix still named `v1.1.x` (released 2026-04-22) as the
  active line, but `v1.2.0` shipped 2026-04-25 with the audit-
  finding security wave and is now where new features land. New
  row added for `v1.2.x`; the `v1.1.x` row reclassified as
  superseded. The "Backports" example also pointed at a
  hypothetical `v1.1.2` that never shipped — replaced with a
  generic "latest `v1.2.x`" pointer that ages better.
- **Hosted-profile docs reference the new
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` escape hatch.**
  `docs/operators/shared-service.md` and
  `docs/deploy/production-profile-shared-service.md` previously
  documented `CLOCKIFY_WEBHOOK_VALIDATE_DNS` without naming its
  escape hatch — operators reading those docs would learn about the
  DNS gate but not how to admit a known-trusted hostname stuck
  behind split-horizon DNS. Both pages now point at the new env
  var and link to the `webhook-dns-validation.md` §4b runbook.
  Closes the doc-sync gap left after ab010e6 lit up the env
  surface.
- **`CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` env var lights up the
  webhook DNS-allowlist escape hatch.** Operators set a
  comma-separated list of hostnames that bypass the
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS` private-IP check. Each entry
  matches either exactly (`webhook.example.com`) or as a
  leading-dot suffix that anchors a full DNS label
  (`.example.com` matches `webhook.example.com` and
  `api.eu.example.com` but NOT `attacker.example.com.evil.com`).
  Whitespace around each entry is trimmed and empty entries are
  dropped, so a leading or trailing comma is harmless. Empty
  list (default) preserves the historical reject-on-private
  behaviour exactly. Use case: split-horizon DNS where a known-
  trusted hostname legitimately resolves to a private IP only
  on the control-plane network — see
  `docs/runbooks/webhook-dns-validation.md` §4b for the operator
  runbook. Surface change: `EnvSpec` entry, Helm
  `clockify.webhookAllowedDomains` value, k8s ConfigMap
  commented placeholder, runtime wiring through
  `internal/runtime/service.go` `newService`. Validator side
  landed in e0c825d; this commit closes the env-surface slice
  of the multi-commit feature. Helm `values.yaml` /
  `deployment.yaml` / k8s `configmap.yaml` updated in the same
  commit because `make release-check`'s `config-parity` gate
  refuses any new env var that isn't reachable through the
  deploy templates.
- **Structured risk taxonomy on `ToolDescriptor`.** New `RiskClass`
  bitmask (`Read | Write | Billing | Admin | PermissionChange |
  ExternalSideEffect | Destructive`) and `AuditKeys []string` fields,
  populated by default from the existing MCP boolean hints and
  refined per tool in `internal/tools/risk_overrides.go`. Audit
  recorder consumes `AuditKeys` so events for permission/billing
  changes carry the action-defining fields (role, status, quantity,
  unit_price) — not just the *_id arguments.
- **Hosted-profile error sanitisation.** Tool-error responses on
  `shared-service` and `prod-postgres` profiles omit upstream
  Clockify response bodies. Operator override:
  `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0/1`. Server-side slog records
  still carry the full APIError for debugging.
- **Hosted-profile webhook DNS validation.** `CreateWebhook` /
  `UpdateWebhook` resolve the host and reject any reply containing
  a private, reserved, link-local, or loopback IP — closes the
  literal-IP-only gap (a hostname pointing at 169.254.169.254 would
  previously sail through). Operator override:
  `CLOCKIFY_WEBHOOK_VALIDATE_DNS=0/1`.
- **Tier-2 activation enumerates the activated group.**
  `clockify_search_tools` responses now include an `activated_tools`
  list and a message naming every tool the activation brought online.
  Tool description spells out the contract: each Tier-2 group is the
  unit of activation.
- **Dry-run on `clockify_mark_invoice_paid` and
  `clockify_test_webhook`.** Both honour `dry_run:true`: GET the
  preview and skip the PUT/POST. Closes the inconsistency where
  `send_invoice` and `deactivate_user` already had handler-level
  dry-run.
- **Static path-safety gate.**
  `TestPathSafety_HandlersValidateIDsBeforeConcat` fails the build
  if any non-test file in `internal/tools/` concatenates a
  non-workspace ID into a URL path without calling
  `resolve.ValidateID` or a `resolve.Resolve*ID` helper.

### Security

- **Hosted profiles refuse `CLOCKIFY_INSECURE=1`.**
  `MCP_PROFILE=shared-service` and `prod-postgres` reject the
  override at startup with an actionable error. Local profiles
  preserve the existing developer behaviour.
- **`CLOCKIFY_WORKSPACE_ID` validated at startup.** Path-traversal
  shaped values (`/`, `?`, `#`, `%`, `..`, control bytes) now fail
  config load instead of silently propagating into every
  `/workspaces/{id}/...` call. `GetWorkspace` adds a belt-and-suspenders
  validate-before-concat in case `ResolveWorkspaceID` returns an
  auto-detected ID from a compromised upstream.

### Changed

- **`config_test.go` uses `maps.Copy` for fixture overlays.** Three
  `for k, v := range hostedProfileEnv { env[k] = v }` loops added
  during the audit-finding wave (Wave G + H) replaced with the
  idiomatic `maps.Copy` call. Functionally equivalent; clears the
  `mapsloop` lint hint.
- **`internal/tools/common.go` modernised.** `buildPaginationSchema`
  copies the optional `properties` overlay via `maps.Copy` instead of
  a hand-written for-range loop; `paginationFromArgs` clamps the
  `page` floor with `max()` instead of an `if`-statement. Pure
  refactor — no behaviour change. Clears the `mapsloop` and `minmax`
  hints accumulated on this file during the audit-finding wave.
- **`internal/mcp/transport_streamable_http.go` modernised.**
  `validateProtocolVersion` swaps a hand-written for-range protocol
  membership check to `slices.Contains(SupportedProtocolVersions, v)`;
  `addSessionToInitializeResult` swaps the map-overlay loop to
  `maps.Copy`. Pure refactor — no behaviour change. Clears the
  `slicescontains` and `mapsloop` hints on this file.
- **`internal/mcp/server.go` modernised.** The stdio scanner-buffer
  sizing collapses to `initial := min(64*1024, maxMsg)`, and the
  `initialize` protocol-version negotiation uses
  `slices.Contains(SupportedProtocolVersions, requested)` instead of
  a for-range scan with break. Pure refactor — no behaviour change.
  Clears the `minmax` and `slicescontains` hints on this file.
- **`internal/tools/reports.go` modernised.** Two `for k, v :=
  range src { dst[k] = v }` loops (the per-page query buffer in
  `aggregateEntriesRange` and the pagination-meta merge in
  `mergeMeta`) replaced with `maps.Copy`. The pre-allocated
  capacity hint on the per-page query map is preserved. Pure
  refactor — no behaviour change. Clears the `mapsloop` hints
  queued during ce5f12b.
- **`internal/policy/policy.go` modernised.** Three
  `for _, item := range strings.Split(os.Getenv(...), ",")`
  loops over the `CLOCKIFY_DENY_TOOLS` / `CLOCKIFY_DENY_GROUPS`
  / `CLOCKIFY_ALLOW_GROUPS` env vars rewritten as
  `for item := range strings.SplitSeq(...)` (same
  modernisation already applied to the new
  `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` parser in ab010e6); the
  `cloneBoolMap` helper's `for k, v := range in { out[k] = v }`
  body collapses to `maps.Copy(out, in)` while keeping the
  pre-allocated capacity hint. Pure refactor — no behaviour
  change. Clears the `stringsseq` and `mapsloop` gopls hints
  surfaced during the iter40 webhook-allowlist parser landing.
- **`internal/bootstrap/bootstrap.go` modernised.** The
  `CLOCKIFY_BOOTSTRAP_TOOLS` parser swaps
  `for _, t := range strings.Split(toolsStr, ",")` for
  `for t := range strings.SplitSeq(...)` and the `Clone`
  helper's `for k, v := range in { out[k] = v }` body
  collapses to `maps.Copy(out, in)` while keeping the
  pre-allocated capacity hint. Pure refactor — no behaviour
  change. Same lint sweep that cleared `policy.go` in
  0953132.
- **`internal/clockify/client.go` `cloneQuery` modernised.**
  The `for k, v := range in { out[k] = v }` body collapses
  to `maps.Copy(out, in)` while preserving the
  `make(map[string]string, len(in)+2)` capacity hint (the
  +2 buffer for the per-page `page` / `page-size` entries
  injected by callers like `aggregateEntriesRange`). Pure
  refactor — no behaviour change. Same lint sweep
  (`policy.go` 0953132, `bootstrap.go` 3c5592e).
- **`internal/jsonmergepatch/merge_patch.go` modernised.** Two
  hints cleared: the `applyAny` object-merge `for k, v := range
  prevObj { out[k] = v }` body collapses to
  `maps.Copy(out, prevObj)` while preserving the
  `make(map[string]any, len(prevObj)+len(patchObj))` capacity
  hint; the `hasNull` slice case `for _, inner := range val { if
  hasNull(inner) { return true } }` collapses to
  `slices.ContainsFunc(val, hasNull)`. Pure refactor — no
  behaviour change. Closes the iter43-queued lint sweep across
  the codebase.
- **`cmd/clockify-mcp/main_test.go` test fixtures modernised.**
  Two identical `for k, v := range overrides { env[k] = v }`
  helper bodies (in the strict-doctor mTLS-tenant-required and
  prod-control-plane-DSN test fixtures) collapse to
  `maps.Copy(env, overrides)`. Pure test-fixture refactor —
  no behaviour change. Same lint sweep continues into test
  files.
- **`internal/config/transport_auth_matrix_test.go` modernised.**
  Per-cell `for k, v := range tc.extra { envs[k] = v }` overlay
  collapses to `maps.Copy(envs, tc.extra)`. Pure test-fixture
  refactor — no behaviour change. Closes the iter46-queued
  test-file lint cleanup.

### Added

- **`internal/paths` package (foundation).** New `paths.Workspace(wsID, sub...)`
  helper validates the workspace ID via `resolve.ValidateID` and
  `url.PathEscape`-s every sub-segment before joining. Empty
  sub-segments and segments containing `/` are rejected at
  construction time so caller bugs surface locally rather than as a
  later 404. No callers migrated yet — this commit lands the
  foundation; future iterations swap handler-level
  `"/workspaces/"+wsID+"/..."` concats over to it.

### Changed

- **`GetWorkspace` migrated to `paths.Workspace`.** First caller of
  the new typed builder. Inline `resolve.ValidateID` + path concat
  swapped for one `paths.Workspace(wsID)` call; identical wire
  shape, identical validation semantics. The `resolve` import drops
  off this file.
- **`ListClients` + `CreateClient` migrated to `paths.Workspace`.**
  Both swap `"/workspaces/"+wsID+"/clients"` for
  `paths.Workspace(wsID, "clients")`. Identical wire shape; gains
  workspace-ID validation on every call (which `ResolveWorkspaceID`
  did not enforce on the env-supplied path).
- **`ListTags` + `CreateTag` migrated to `paths.Workspace`.** Same
  shape as `clients.go` — workspace-ID validation on every call,
  byte-identical wire output for normal Clockify IDs.
- **`ListUsers` migrated to `paths.Workspace`.** Last wsID-only
  caller before sub-segment ID territory (`projects.go`,
  `entries.go`, `tasks.go`, `tier2_*.go`). `WhoAmI` /
  `CurrentUser` hit `/user` (no workspace prefix) and stay as-is.
- **`projects.go` migrated to `paths.Workspace`.** First sub-segment
  exercise of the helper: `GetProject` now uses
  `paths.Workspace(wsID, "projects", projectID)` — the project ID
  comes from `resolve.ResolveProjectID` so it's already validated;
  the helper adds defensive percent-encoding on top.
  `ListProjects` and `CreateProject` use the simpler two-segment
  form. Identical wire shape for normal Clockify IDs.
- **`tasks.go` migrated to `paths.Workspace`.** `ListTasks` +
  `CreateTask` now use the four-segment form
  `paths.Workspace(wsID, "projects", projectID, "tasks")`. The
  project ID is resolved upstream via `resolve.ResolveProjectID`;
  the helper percent-encodes each segment. Identical wire shape for
  normal Clockify IDs.
- **`entries.go` migrated to `paths.Workspace`.** Seven concat sites
  across `GetEntry`, `AddEntry`, `UpdateEntry` (GET fetch + PUT),
  `DeleteEntry` (GET preview + DELETE), and `listEntriesWithQuery`
  (`/workspaces/<ws>/user/<uid>/time-entries`) all swap to
  `paths.Workspace(...)`. Largest single-file migration so far —
  exercises 2-, 3-, and 4-segment forms in one file.
- **`entries.go` modernised.** `ListEntries` clamps `pageSize` with
  `min()` instead of an `if`-statement; `UpdateEntry`'s outdated-URI
  loop uses `slices.Contains` instead of a hand-written found-loop.
  Pure refactor — no behaviour change. Clears the lint hints that
  surfaced during the f372814 migration.
- **`timer.go` migrated to `paths.Workspace`.** `StartTimer` POSTs
  to `paths.Workspace(wsID, "time-entries")`; `StopTimer` PATCHes
  `paths.Workspace(wsID, "user", user.ID, "time-entries")`.
  `TimerStatus` is unchanged — it routes through
  `listEntriesWithQuery`, which was migrated in f372814.
- **`reports.go` migrated to `paths.Workspace`.** Single concat in
  `aggregateEntriesRange` (used inside the pagination loop) swaps
  to `paths.Workspace(wsID, "user", user.ID, "time-entries")`. Same
  4-segment shape as the entries.go helper.
- **`workflows.go` migrated to `paths.Workspace`.** `LogTime` POSTs
  to `paths.Workspace(wsID, "time-entries")`; `FindAndUpdateEntry`
  PUTs to `paths.Workspace(wsID, "time-entries", entry.ID)`.
  `SwitchProject` is unchanged — it delegates to `StopTimer` /
  `StartTimer`, both already migrated in 3e7ae44.

### Security

- **`resources.go` migrated to `paths.Workspace` — adds first-ever
  `ValidateID` on URI-parsed IDs.** `ReadResource` parses workspace
  / user / project / entry / group IDs straight out of the
  `clockify://workspace/{id}/...` URI supplied by the MCP client.
  Pre-fix none of those IDs were validated before reaching the
  Clockify URL path. After migration, `paths.Workspace` runs
  `resolve.ValidateID` on the workspace ID and `url.PathEscape` on
  every sub-segment, so a URI containing `/`, `?`, `#`, `%`, `..`
  or a control byte is rejected at the resource layer instead of
  being silently forwarded.
- **`tier2_project_admin.go` migrated to `paths.Workspace`.** All 6
  concats (list templates, get template, create template, update
  estimate, set memberships, archive projects bulk) swap to the
  typed builder. First Tier-2 file in the migration; same pattern
  as the Tier-1 sweep — pure refactor for tools whose IDs are
  already validated upstream by `resolve.ValidateID`.
- **`tier2_shared_reports.go` migrated to `paths.Workspace`.** All
  7 concats — list, get, create, update, dry-run preview, delete,
  export — swap to the typed builder. `deleteSharedReport` builds
  `reportPath` once and reuses it for the dry-run GET preview and
  the actual DELETE.
- **`tier2_approvals.go` migrated to `paths.Workspace`.** All 8
  concats across 6 handlers (list, get, submit, approve, reject,
  withdraw). `approveTimesheet` and `rejectTimesheet` build
  `approvalPath` once at the top, used for both the dry-run GET
  preview and the PUT — same pattern as `entries.go`.
- **`tier2_custom_fields.go` migrated to `paths.Workspace`.** All 8
  concats: list/get/create/update; `DeleteCustomField` builds
  `fieldPath` once for dry-run GET + DELETE; `SetCustomFieldValue`
  picks the projects-vs-time-entries branch via the helper while
  keeping the conditional shape.
- **`tier2_groups_holidays.go` migrated to `paths.Workspace`.** All
  8 concats — 5× user-groups (list/get/create/update/delete) +
  3× holidays (list/create/delete). Standard `paths.Workspace`
  swaps; helper validates the workspace ID and percent-encodes
  every sub-segment.
- **`tier2_user_admin.go` migrated to `paths.Workspace`.** All 8
  concats across the admin surface (list/create/update/delete
  user-groups, add/remove user from group, update user role,
  deactivate user). The `update_user_role` and `deactivate_user`
  PUT paths target `/workspaces/<ws>/users/<uid>{,/roles}`, which
  carries `RiskAdmin | RiskPermissionChange` per the descriptor
  taxonomy — defence-in-depth percent-encoding is doubly welcome
  here.
- **`tier2_webhooks.go` migrated to `paths.Workspace`.** All 9
  concats — list/get/create/update/delete + ListWebhookEvents
  (special `/webhooks/events` literal sub-path) + TestWebhook
  (which has separate dry-run preview path and `/test` POST path).
  `DeleteWebhook` builds `webhookPath` once for the dry-run GET +
  DELETE pair. Webhooks carry `RiskExternalSideEffect`; the helper
  re-validates the workspace ID even though the literal-IP webhook
  URL check still runs in the body.
- **`tier2_scheduling.go` migrated to `paths.Workspace`.** All 10
  concats across 8 handlers (assignments CRUD, schedules CRUD,
  project-totals report, capacity filter). Cleanest pattern in the
  Tier-2 cluster — every concat was already in the form
  `path := "..."`, so the swap is a literal RHS replacement.
- **`tier2_expenses.go` migrated to `paths.Workspace`.** All 11
  concats: 5 expense handlers (list/get/create/update/delete) +
  4 category handlers (list/create/update/delete) + 1 report.
  `deleteExpense` builds `expensePath` once for dry-run preview +
  DELETE; `deleteExpenseCategory` uses minimal-fallback short-circuit
  before the path is needed.
- **`tier2_time_off.go` migrated to `paths.Workspace`.** All 12
  concats across 10 handlers — request CRUD nested under policy ID
  (5-segment), approve/deny PUTs (6-segment), policy CRUD
  (3-4 segment), balance lookup (6-segment).
  `deleteTimeOffRequest` and `updateTimeOffRequest` /
  `updateTimeOffPolicy` build path once for the GET-then-mutate
  pair. First Tier-2 file with consistent 6-segment paths.
- **`tier2_invoices.go` migrated to `paths.Workspace`.** All 15
  concats across 12 handlers — list/get/create/update/delete
  invoices, send (3-segment `/send`), mark-paid, item CRUD
  (4/5-segment `/items[/id]`), and report. `deleteInvoice` and
  `markInvoicePaid` build path once for the dry-run GET preview +
  the real PUT/DELETE; `sendInvoice` uses two paths (the bare
  invoice for the preview, plus the `/send` sub-path for the POST).
  Closes the Tier-2 caller-migration sweep (12/12 files).
- **`docs/tool-catalog.json` exposes `risk_class` + `audit_keys`.**
  The catalog generator now decomposes every tool's `mcp.RiskClass`
  bitmask into stable lowercase taxonomy names (`read`, `write`,
  `billing`, `admin`, `permission_change`, `external_side_effect`,
  `destructive`) and surfaces the `AuditKeys` slice. Consumers
  (policy agents, ops dashboards, audit harnesses) can now filter
  on the structured taxonomy without grep-ing source.
- **`docs/tool-catalog.md` gains a `Risk` column.** Both the Tier-1
  table and every Tier-2 group sub-table render the same risk
  taxonomy as inline-coded names joined with `, `. Empty risk (the
  zero-value bitmask, which never occurs today) renders as an em
  dash so the column never collapses to a blank cell. Closes the
  human-browsing gap left when 97c20da landed the JSON surface
  without touching the markdown rendering.

### Fixed

- **`internal/paths` package doc no longer leaks an absolute path.**
  The package comment previously pointed reviewers at
  `/Users/15x/.claude/plans/...` for the audit-finding context — a
  personal local path with no meaning on any other machine. Now
  references the in-repo CHANGELOG entries (0de5458, 1919006) and
  describes the migration sweep in past tense since it completed.
- **`/mcp/events` legacy-alias comment matches reality.** The
  comment in `streamableHTTPMux` previously promised the alias
  would be removed in v0.7 — the project shipped v1.0 in 2026-04-12
  and is now at v1.2.0 with the alias still mounted, so the comment
  was stale by ~9 months. Replaced with an ADR-0012 reference noting
  the route stays indefinitely (operator-facing route removal would
  need a v2.0 bump). `TestStreamableEventsBackCompatAlias` docstring
  updated in the same commit so test + production prose agree.
- **ADR-0002 file references unstuck from pre-C2.2 line numbers.**
  Two sites pointed at `internal/config/config.go:107-116` for the
  `MCP_TRANSPORT` validation switch (now at lines 239–247 after the
  auth surface grew) and one at `cmd/clockify-mcp/main.go:161-260`
  for dispatch wiring (which moved to `internal/runtime/runtime.go`
  in the dea1cc3 C2.2 refactor — main.go is now a 236-line shim).
  Replaced with function-name search anchors (`Load()`,
  `Runtime.Run`) so future reorgs do not invalidate the ADR again.
  Landed in two passes (References section in 6a3c25b, body
  paragraph in this commit) to stay within the per-commit file
  budget.
- **ADR-0003 file references unstuck from pre-1.x line numbers.**
  Four anchors invalidated by post-write growth of the auth surface
  in `internal/config/config.go` (the auth-mode switch + token-length
  checks moved ~130 lines down) and the gRPC transport file (the
  Authenticator-wiring block shifted ~10 lines after the TLS option
  block grew). Replaced with the same function-name + grep-string
  search anchors used in the ADR-0002 sweep so the same reorg-drift
  cannot recur.
- **ADR-0005 file references unstuck from pre-C2.2 line numbers.**
  Three stale anchors fixed: `internal/tools/context.go:75-90`
  (the activation handler grew past line 90 when the activated_tools
  enumeration landed); `bootstrap.go:55-68` (mis-described the
  `AlwaysVisible` location — that map starts at line 47, not 55);
  and `cmd/clockify-mcp/runtime.go:113-150` (file was removed in
  the dea1cc3 C2.2 refactor — wiring now lives in
  `internal/runtime/runtime.go` `New()`). Anchors at `bootstrap.go:71`
  and `:56` were verified still accurate and left as-is — this
  iteration only touched the genuinely stale ones.
- **ADR-0006 historical-rename anchors realigned.** The
  References section lists the source comments that say
  "See ADR 009" (the historical name for ADR-0006). Two of six
  line refs had drifted: `cmd/clockify-mcp/main.go:145` (now :147)
  and `scripts/check-build-tags.sh:64` (now :68). Line refs are
  intentional here — the comment text is the same in every file,
  so a grep would not disambiguate. The other four anchors
  (otel_on.go:15, otel_off.go:10, otel.go:5, span_emit_test.go:39)
  were verified still accurate.
- **`normalizeEndpoint` comment matches behaviour.** Doc now
  precisely describes the 24/32/36-char ID-shape match instead of
  overstating "any other non-letter leading segment". Companion
  test (`TestNormalizeEndpoint_NonIDShapesPreserved`) locks in
  both the collapse and the preserve paths so comment + code can
  no longer drift apart silently.
- **Stdio honours `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1`.** The
  flag is now assigned in `buildServer()`, so every transport
  (stdio, legacy_http, streamable_http session, grpc) picks it up
  uniformly. Pre-fix the assignment lived in `runLegacyHTTP()` and
  the streamable-session overlay only, leaving stdio operators
  with no way to opt in.
- **Name resolution accepts legitimate Clockify names with
  punctuation.** New `resolve.ValidateNameRef` is a permissive
  sibling of `ValidateID`: empty / oversized / control-byte input
  still fails, but `/`, `?`, `#`, `%`, `&`, `..`, and Unicode pass
  through. `resolveByNameOrID` and `ResolveUserID` now dispatch
  on shape — strict `ValidateID` only when the input is being
  returned verbatim as a path-segment ID, permissive `ValidateNameRef`
  when it goes to a `name=` query parameter (which `url.Values`
  safely encodes). Pre-fix, project / client / tag / task names
  like "ACME / Support" or "R&D 50%" failed validation before the
  safe lookup could run.

## [1.2.0] - 2026-04-25

> **Scope note.** Security-hardening wave following the
> 2026-04-25 audit, plus a follow-on wave that lands first-class
> gRPC release artifacts, MCP-path live safety tests, and a few
> supply-chain repairs that surfaced during release verification.
> Self-hosted single-tenant behaviour is preserved by default —
> every new restriction is opt-in via a flag (`MCP_OIDC_STRICT`,
> `MCP_REQUIRE_TENANT_CLAIM`, `MCP_DISABLE_INLINE_SECRETS`) so
> existing deployments continue to work unchanged.

### Added (post-Wave-G additions)

- **First-class gRPC release artifacts.** GoReleaser now publishes
  four new linux-only binaries:
  `clockify-mcp-grpc-{linux-x64,linux-arm64}` (private-network
  gRPC, no postgres) and
  `clockify-mcp-grpc-postgres-{linux-x64,linux-arm64}` (HA
  private-network gRPC + pgx control plane). Each ships through
  the same SBOM (syft) + cosign sigstore + SLSA build-provenance
  chain as the default and Postgres binaries.
  `scripts/check-release-assets.sh` raised `EXPECTED_COUNT` from
  34 → 46 and gained `GRPC_PLATFORMS` / `GRPC_POSTGRES_PLATFORMS`
  arrays; the regex was reordered so `-grpc-postgres` matches
  before `-grpc`. The hosted launch checklist references the new
  artifact names.
- **`scripts/check-grpc-release-parity.sh`** — release-blocking
  drift gate: the private-network-grpc profile doc must not claim
  tenant defaults to `X-Tenant-ID`, must not claim Docker images
  include gRPC unless either `.goreleaser.yaml` ships a `-grpc`
  artifact or the Dockerfile / docker-image workflow exposes a
  `GO_TAGS` build arg, and any doc reference to a `-grpc`
  artifact must be backed by a matching GoReleaser build id +
  asset-count enumeration. Wired into `verify-core` and
  `release-check`.
- **`make build-grpc` / `make build-grpc-postgres`** — local
  build targets that exercise the gRPC and gRPC+postgres tag
  matrices so `make verify` is honest about the private-network
  gRPC profile compiling against the working tree.
- **MCP-path live safety contracts** — three new tests that
  exercise the production enforcement / audit pipeline against a
  real Clockify backend instead of the bare tool handlers:
  - `TestLiveDryRunDoesNotMutate` (`tests/e2e_live_mcp_test.go`,
    build tag `livee2e`) — confirms `clockify_delete_entry` with
    `dry_run:true` previews via the GET counterpart and never
    deletes the entry upstream.
  - `TestLivePolicyTimeTrackingSafeBlocksProjectCreate`
    (`tests/e2e_live_mcp_test.go`) — confirms
    `CLOCKIFY_POLICY=time_tracking_safe` rejects
    `clockify_create_project` at the policy gate before the
    handler runs.
  - `TestLiveCreateUpdateDeleteEntryAuditPhases`
    (`internal/controlplane/postgres/live_audit_phases_test.go`,
    build tags `postgres,livee2e`) — confirms a real
    create→update→delete entry cycle persists six audit rows
    (3 intent + 3 outcome) in a Postgres-backed control plane,
    distinguished only by phase + outcome segments embedded in
    the synthesised `external_id`.
  All three are wired into `.github/workflows/live-contract.yml`
  under the existing `CLOCKIFY_LIVE_WRITE_ENABLED=true` gate.
  The audit-phase test additionally requires
  `MCP_LIVE_CONTROL_PLANE_DSN`; missing on a fork is a soft skip,
  missing on the main repo is a hard fail when the new repo
  variable `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` is set.
- **Docker `GO_TAGS` build-arg path.** `deploy/Dockerfile` now
  accepts `--build-arg GO_TAGS=grpc[,postgres]` so operators can
  build a gRPC-capable image directly from the published
  Dockerfile:
  `docker build --build-arg GO_TAGS=grpc,postgres -f deploy/Dockerfile -t clockify-mcp:grpc-postgres .`
  Default image is byte-equivalent (empty `-tags=""` is a Go
  toolchain no-op). The Dockerfile also copies `go.work`,
  `go.work.sum`, and the per-sub-module `go.mod` / `go.sum` pairs
  so tagged builds resolve workspace modules correctly without
  silently falling back to a stale remote sub-module version.
- **Docker PR-only gRPC smoke test** in
  `.github/workflows/docker-image.yml` builds a side image with
  `GO_TAGS=grpc,postgres` and verifies the runtime no longer hits
  the `!grpc` stub error, so the documented self-build path can't
  rot.
- **`internal/auditbridge/`** — shared `ToControlPlaneEvent(event,
  now)` helper used by both the runtime auditor and the live
  audit-phase contract test. Centralises the
  `mcp.AuditEvent → controlplane.AuditEvent` conversion plus the
  external_id synthesis that keeps PhaseIntent + PhaseOutcome rows
  distinct under the Postgres unique constraint. Three direct
  unit tests pin the field mapping, the IDs-differ contract, and
  nil-metadata defensive behaviour.
- **`internal/authn/category_test.go`** — pins the
  `FailureCategory` substring → bucket contract that every
  transport's auth-failure log/metric label depends on. Closes
  the pre-existing `internal/authn` coverage shortfall (86.0% →
  89.8%).

### Fixed (post-Wave-G)

- **FIPS binaries now get SLSA build provenance.** Every release
  back to v1.0.x shipped FIPS binaries cosign-signed but with no
  attest-build-provenance subject — `gh attestation verify
  clockify-mcp-fips-*` returned HTTP 404. The Wave-G FIPS row was
  added to `.goreleaser.yaml` without extending `release.yml`'s
  staging step or the `attest-build-provenance` subject-path.
  Operators running the FIPS binary previously could not satisfy
  the launch checklist's "SLSA build provenance attested" gate.
  Closes the gap manually surfaced during v1.1.0 verification.
- **Dockerfile previously failed on `GO_TAGS=grpc[,postgres]`.**
  The build context only included `go.mod`; tagged builds need
  `go.work` + the workspace sub-module manifests
  (`internal/transport/grpc`, `internal/controlplane/postgres`,
  `internal/tracing/otel`). Without them the Go toolchain either
  failed offline or silently downloaded a stale remote version of
  the sub-module — shipping an image whose gRPC code did not match
  its source tree. The published `docker build --build-arg
  GO_TAGS=grpc,postgres ...` recipe now actually works.

### Changed (post-Wave-G)

- **`docs/deploy/profile-private-network-grpc.md`** corrected:
  tenant extraction defaults to `MCP_MTLS_TENANT_SOURCE=cert`
  (not `X-Tenant-ID`); the Docker default image does NOT include
  gRPC; auth modes other than mtls are supported but not the
  recommended posture.
- **`docs/support-matrix.md`** gRPC row spells out both supported
  build paths (published artifact vs. self-build).
- **`docs/release/public-hosted-launch-checklist.md`** Storage row
  references `clockify-mcp-grpc-postgres-*` for HA gRPC; live
  coverage gate is now executable rather than "tracked or
  closed" prose.
- **`internal/runtime/service.go`** controlPlaneAuditor delegates
  to `internal/auditbridge.ToControlPlaneEvent` instead of
  inlining the conversion + ID synthesis. Same contract, one
  source of truth.

### Wave G — Security-hardening wave (the original 2026-04-25 audit)

> **Scope note.** Security-hardening wave following the
> 2026-04-25 audit. Six atomic commits closing the seven blockers
> the audit flagged for paid/public hosted-service deployment, plus
> the M-tier docs/defaults drift. Self-hosted single-tenant
> behaviour is preserved by default — every new restriction is
> opt-in via a flag (`MCP_OIDC_STRICT`, `MCP_REQUIRE_TENANT_CLAIM`,
> `MCP_DISABLE_INLINE_SECRETS`) so existing deployments continue to
> work unchanged.

### Added

- **Native TLS / mTLS on the streamable HTTP transport** via two
  new env vars: `MCP_HTTP_TLS_CERT` and `MCP_HTTP_TLS_KEY`.
  `MCP_TRANSPORT=streamable_http` with non-empty cert/key paths
  wraps the listener with `tls.NewListener`; combined with
  `MCP_AUTH_MODE=mtls` and `MCP_MTLS_CA_CERT_PATH`, it enables
  end-to-end mutually-authenticated TLS without a fronting proxy.
  The shared TLS helpers live in `internal/runtime/tlsutil.go`
  and are reused by the gRPC transport. Closes audit finding H3.
- **`MCP_OIDC_STRICT=1`** — fails `config.Load` when oidc is
  selected without `MCP_OIDC_AUDIENCE` or `MCP_RESOURCE_URI`,
  and rejects tokens missing an `exp` claim at the per-token
  level. Default unchanged (back-compat). Closes finding C1.
- **`MCP_REQUIRE_TENANT_CLAIM=1`** — rejects oidc tokens whose
  tenant claim is empty instead of falling back to
  `MCP_DEFAULT_TENANT_ID`. Required for any multi-tenant hosted
  deployment. Closes finding H6.
- **`MCP_DISABLE_INLINE_SECRETS=1`** — rejects credential refs
  with `backend=inline` so secrets are forced through env / file /
  external vault backends. Closes finding L3.
- **`time_tracking_safe` policy mode** — new `CLOCKIFY_POLICY`
  tier strictly between `read_only` and `safe_core`. Allows
  reads + own-time-entry mutations + timer control; blocks
  workspace-wide `create_*` tools (project / client / tag /
  task). Recommended default for untrusted AI agents. Closes
  finding M4.
- **Audit phase concept** — `AuditEvent` gains a `Phase` field
  (`PhaseIntent` / `PhaseOutcome`). Non-read-only tool calls
  now write a pre-handler intent record AND a post-handler
  outcome record. Empty `Phase` ("") preserved for backward
  compatibility with audit consumers that pre-date this change.
- **Hosted-service hardening guards** — production profile docs
  set `MCP_OIDC_STRICT=1` + `MCP_REQUIRE_TENANT_CLAIM=1` +
  `MCP_DISABLE_INLINE_SECRETS=1`. Branch-protection target
  state for paid launch documented in
  `docs/branch-protection.md`. Live contract tests now fail
  the workflow on the main repo when secrets are absent (forks
  keep the warning-and-skip behaviour). Closes findings M7, L2.
- **`deploy/k8s/overlays/legacy-http/`** — explicit opt-in
  overlay for operators still on pre-v1.1.0 clients that have
  not migrated to streamable HTTP.
- **`tests/deploy_defaults_test.go`** — guards the Dockerfile,
  Helm values, and Kustomize base against drifting back to
  `MCP_TRANSPORT=http` or `CLOCKIFY_POLICY=standard`.
- **`tests/doc_parity_test.go`** — asserts the README MCP
  protocol badge AND support matrix row both equal
  `mcp.SupportedProtocolVersions[0]`.

### Changed

- **fail_closed audit now actually blocks mutation.** Previously
  `MCP_AUDIT_DURABILITY=fail_closed` returned an error to the
  client AFTER the mutation had already committed upstream. The
  two-phase intent/outcome model writes the intent record
  pre-handler — when intent persistence fails in fail_closed
  mode, the handler is skipped entirely and the mutation never
  happens. The new behaviour is documented in
  `docs/runbooks/audit-durability.md`. Closes finding H5.
- **Default deployment transport flipped from `http` to
  `streamable_http`** in `deploy/Dockerfile`, the Helm
  `values.yaml`, and the Kustomize base. Closes finding H1.
- **Default deployment policy flipped from `standard` to
  `safe_core`** in the Helm `values.yaml` and Kustomize base
  (Dockerfile inherits via env). Closes finding M3.
- **`MCP_STRICT_HOST_CHECK=1`** by default in Dockerfile, Helm
  values, and Kustomize base — DNS-rebinding mitigated by
  default for any deployment exposed beyond loopback.
- **Stdio panic recovery returns a generic message** instead
  of the raw panic value. Full panic and stack remain in
  `slog.Error("panic_recovered", ...)`. Closes finding H7.
- **Config validation requires TLS cert material when
  `MCP_AUTH_MODE=mtls`** on either the streamable HTTP or
  gRPC transport. Previously these combinations would start
  successfully and fail every request at runtime; now they
  fail at startup with a message naming the missing variable.
  Closes finding H4.
- **README MCP protocol version** and back-compat list aligned
  with `SupportedProtocolVersions` (newest: `2025-11-25`).
  Closes finding H2.
- **Production-profile docs** corrected to reference `/health`
  and `/ready` (the actual registered routes; previously
  `/healthz` and `/readyz`). Closes finding M2.
- **`SECURITY.md` and `SUPPORT.md` version matrix** brought in
  line with v1.1.0's release date (2026-04-22). Closes
  finding M1.
- **`docs/branch-protection.md`** gains a "Target state for
  paid / public hosted launch" section documenting the
  governance tightening blocked on adding a second maintainer.
  Closes finding L2.
- **`scripts/check-doc-parity.sh`** excludes
  `docs/superpowers/` from the operator-facing parity scan
  (design specs by definition describe future state).

### Security

- The audit refactor closes the gap where `fail_closed`
  delivered post-hoc evidence rather than acting as a
  preventive control. Operators in fail_closed mode now have
  a real durability guarantee: a broken audit pipeline blocks
  mutations rather than committing them and complaining.
- Stdio panic recovery no longer leaks panic values
  (potentially containing request data, internal state, or
  upstream credential fragments) to MCP clients.
- OIDC accepts issuer-only tokens by default for back-compat;
  hosted deployments must opt into the strict mode flags.

### Fixed

> **Follow-up audit pass (2026-04-25).** Five additional atomic
> commits caught by a deeper read-through of the post-wave repo
> state. Each closes a defense-in-depth or wiring gap that the
> primary security wave did not cover.

- **Metrics listener no longer silently un-authenticates.**
  `mcp.ServeMetrics` and `metricsMux` now refuse
  `AuthMode=static_bearer` with an empty `BearerToken`. The
  production startup path through `cmd/clockify-mcp/main.go` was
  already protected by config-load validation
  (`internal/config/config.go:280-285`); this commit closes the same
  gap at the library API surface so a programmatic embedder building
  `MetricsServerOptions` directly cannot regress the property. Without
  the guard, `subtle.ConstantTimeCompare("","")==1` would treat any
  client (including a bare `Authorization: Bearer ` header) as
  authenticated despite a startup log claiming bearer mode.
- **OIDC strict mode rejects `authn.Config` without audience or
  resource URI.** `authn.New` now refuses to construct the OIDC
  authenticator when `OIDCStrict=true` with both `OIDCAudience` and
  `OIDCResourceURI` empty. Mirrors the pre-existing config-load
  check at `internal/config/config.go:360-361` (which only fires
  on the env-var path) and matches the documented `MCP_OIDC_STRICT`
  contract that strict mode binds tokens to this server.
- **Schema/handler date-time drift on `clockify_add_entry`,
  `clockify_list_entries`, `clockify_weekly_summary`, and several
  Tier 2 timesheet/approval/expenses tools.** The schema tightener
  added `format: "date-time"` to any string property mentioning
  "RFC3339" in its description, even when that description also
  documented a flexible parser (`natural language`, `YYYY-MM-DD`).
  The jsonschema validator's strict `time.Parse(time.RFC3339, ...)`
  then rejected valid input like `start="now"` before the handler's
  lenient `timeparse.ParseDatetime` ever saw it. The tightener now
  skips the `format` constraint on flexible-time fields;
  `docs/tool-catalog.json` regenerated to match.
- **Helm ServiceMonitor wires the dedicated metrics listener
  correctly.** Setting `metricsEndpoint.bind` non-empty now renders a
  `metrics` Service port + matching containerPort + ServiceMonitor
  `port: metrics` in one toggle. A new
  `metrics.serviceMonitor.bearerTokenSecret` block attaches the
  Authorization header that `static_bearer` auth requires. The
  previous chart left the dedicated listener unreachable from
  Prometheus and never carried an auth header. Kustomize base
  ServiceMonitor gains an inline comment clarifying that the
  `port: http` default works only with inline metrics
  (`MCP_HTTP_INLINE_METRICS_ENABLED=1`); use the Helm toggle or
  layer an overlay for the dedicated-listener pattern.
- **Postgres integration test gate fails loud when Testcontainers
  unavailable.** `internal/controlplane/postgres` now honours an
  `INTEGRATION_REQUIRED` env var that turns Testcontainers failure
  into `t.Fatal` instead of `t.Skip`. The Makefile target
  `test-postgres` sets the var so a Docker-less CI run can no longer
  report green vacuously. Developer laptops without Docker keep the
  historic skip behaviour by running `go test
  -tags=postgres,integration` directly.

## [1.1.0] - 2026-04-22

> **Scope note.** First minor release after v1.0.0 (2026-04-12).
> Lands the profile-centric configuration model, the `doctor`
> subcommand, governance alignment, the toolchain + build-tag
> matrix tightening, and the public-repo flip that makes SLSA
> attestation a mandatory gate.

### Added

- **Canonical deployment profiles (`--profile=<name>`).** Five
  code-enforced profiles — `local-stdio`,
  `single-tenant-http`, `shared-service`,
  `private-network-grpc`, `prod-postgres` — bundle the pinned
  defaults for each supported deployment shape. Apply via
  `clockify-mcp --profile=<name>` or `MCP_PROFILE=<name>`.
  Explicit env overrides always win; the Wave H fail-closed
  guards run unchanged. Each profile has a matching
  `deploy/examples/env.<name>.example` file and a doc in
  `docs/deploy/profile-<name>.md`.
- **`clockify-mcp doctor` subcommand.** Audits the effective
  configuration, attributing every spec'd env var as
  `explicit` / `profile` / `default` / `empty` via a
  text/tabwriter report. Exit code 0 on clean Load(), 2 on a
  Load() error. Takes the same `--profile=<name>` flag as the
  server.
- **`SUPPORT.md`** at repo root covers where to ask questions,
  response expectations (best effort, no SLA), v1.x wire-format
  stability guarantee, and the version support matrix.
- **Build-tag matrix workflow.** New
  `.github/workflows/build-matrix.yml` runs compile-only checks
  on six tag combinations (`grpc`, `fips`, `otel`, `pprof`,
  `grpc,otel`, `fips,grpc`) on every push, PR, and weekly cron.
- **Four new ADRs.** ADR-0013 (private-repo SLSA posture;
  superseded at release time), ADR-0014 (prod fail-closed
  defaults), ADR-0015 (profile-centric configuration model),
  ADR-0016 (single-maintainer governance reality).

### Changed

- **SLSA attestation is now mandatory on release.** The repo
  flipped to public on 2026-04-22, which unblocked the GitHub
  attestation service for this account tier.
  `actions/attest-build-provenance` in `release.yml` and
  `docker-image.yml` is no longer `continue-on-error: true`; the
  `gh attestation verify` step in `release-smoke.yml` no longer
  treats HTTP 404 as a skip. A missing or invalid attestation
  will now fail the release or the smoke. ADR-0013 is marked
  superseded; the workaround it documented is no longer live.
- **Production fail-closed defaults (`ENVIRONMENT=prod`).**
  With `ENVIRONMENT=prod` unset values of
  `MCP_HTTP_LEGACY_POLICY` resolve to `deny` (was `warn`) and
  `MCP_AUDIT_DURABILITY` resolves to `fail_closed` (was
  `best_effort`). Explicit operator values always win. ADR-0014
  captures the rationale. Load() also fails closed at the
  streamable_http + dev-DSN boundary without the explicit
  `MCP_ALLOW_DEV_BACKEND=1` acknowledgement.
- **Governance documentation aligned on single-maintainer
  reality.** `GOVERNANCE.md`, `.github/CODEOWNERS`, the PR
  template, and the new `SUPPORT.md` now tell one consistent
  story. Drops the `@backup-maintainer` placeholder that never
  resolved to a real handle. ADR-0016 codifies the decision.
- **Release smoke strips the `v` prefix before ghcr lookup.**
  `docker-image.yml` publishes semver tags without the leading
  `v` (via the metadata-action `{{version}}` pattern), so
  `release-smoke.yml` now normalises the tag before calling
  `cosign triangulate`. Closed the last layer tracked under
  issue #7.
- **Release smoke authenticates to ghcr.io.** Added a
  `cosign login ghcr.io` step so the container manifest lookup
  works after the visibility switch paths were proven.
- **Release smoke cosign version aligned with release signer.**
  Bumped cosign-installer to match `release.yml`'s v2.4.3 so
  the verifier can read the `--new-bundle-format` bundles the
  signer writes.
- **govulncheck pinned to a commit SHA** instead of `@master`,
  for supply-chain reproducibility. Tracked to revisit once a
  tagged release supports go1.25.

### Fixed

- **Issue #7 closed.** The four-layer smoke failure (SLSA 404
  on private repos, cosign format skew, ghcr auth, tag prefix)
  was resolved across PRs #16, #18, #19, #20.

### Security

- Streamable HTTP fail-closed guard at config load time
  prevents multi-process deployments from silently running
  against an in-memory control plane. The existing runtime
  guard remains as defence-in-depth.

## [1.0.3] - 2026-04-20

### Fixed

- **Release workflow continues past SLSA attestation failure.**
  `actions/attest-build-provenance` hard-fails on user-owned
  private repositories (GitHub feature gate). `release.yml`'s
  attestation step is now `continue-on-error: true`, same treatment
  as `docker-image.yml`. Attestations activate automatically if
  the repo moves to an org or goes public.

### Context

v1.0.1 and v1.0.2 both published 28 signed+SBOM'd assets to the
GitHub Release but never reached npm or completed SLSA attestation
because a downstream step in the release workflow killed the
pipeline under `set -e`. v1.0.3 is the first release to complete
the full pipeline — GitHub Release, cosign signatures, SBOMs, npm
publish, and reproducibility trigger. Code-wise v1.0.3 and v1.0.2
are identical except for this CHANGELOG entry and the workflow
fix.

## [1.0.2] - 2026-04-20

### Fixed

- **Release pipeline.** `scripts/check-release-assets.sh` now
  understands goreleaser 2.x's per-build output layout. Previous
  versions assumed every artefact sat at `dist/` top level, which
  was true for goreleaser 1.x but not 2.x — the latter places raw
  binaries and cosign sigstore bundles under per-build subdirs
  (`dist/clockify-mcp_linux_amd64_v1/clockify-mcp`). The v1.0.1
  release workflow hit this and the script flagged 18 of 28 assets
  as missing even though goreleaser had already published them
  under their correct names. The fix consults `dist/artifacts.json`
  for the name→path mapping when available, falls back to a
  recursive filesystem walk, and uses a precise regex to
  distinguish published assets from intermediate binary IDs. v1.0.1
  did ship its 28 assets to the GH Release but the post-check
  failure blocked npm publish + SLSA attestation; v1.0.2 is the
  clean end-to-end release.

## [1.0.1] - 2026-04-20

> **Scope note.** The eight days between v1.0.0 and v1.0.1 accumulated
> a large volume of backwards-compatible work — EnvSpec registry,
> Postgres control-plane backend, expanded auth matrix, audit
> retention reaper, transport parity matrix, async gRPC dispatch,
> SSE resume verification, and a full pre-ship gate (`make
> release-check`). No public API changed; tool names, resource URI
> templates, env-var surface, and protocol behaviour remain the v1
> baseline. The patch version reflects the absence of breaking
> changes, not the size of the delta.

### Added

- **Async gRPC Exchange dispatch.** `internal/transport/grpc/transport.go`
  now dispatches each inbound frame in its own goroutine and funnels
  all outbound frames through a single send-pump goroutine. A
  `notifications/cancelled` queued behind an in-flight `tools/call`
  now reaches the dispatcher immediately rather than waiting for the
  blocking handler to return. gRPC rows are re-enabled in the
  cancellation and `tools/list_changed` parity suites, giving those
  contracts full-transport coverage.
- **SSE `Last-Event-ID` resume parity test.** `tests/sse_resume_test.go`
  drives the streamable HTTP server through a drop-and-reconnect
  cycle, proving that `sessionEventHub`'s ring buffer replays the
  exact gap a client missed while disconnected.
- **Raw-send harness primitive + malformed-JSON parity.**
  `Transport.SendRaw` is now part of the `tests/harness` contract
  on stdio, legacy HTTP, streamable HTTP, and gRPC.
  `TestSizeLimit_MalformedJSONParity` sends a deliberately invalid
  frame and asserts every transport surfaces JSON-RPC parse error
  `-32700`. Closes the third boundary the size-limit suite had
  deferred alongside at-limit and over-limit.

- **Structured tool responses (A1).** Every successful `tools/call`
  now emits `structuredContent` alongside the existing text content
  block, validating against the tool's advertised `outputSchema`.
  Old clients that read `content[0].text` keep working unchanged.
- **Full auth matrix on legacy HTTP (A2).** `MCP_TRANSPORT=http` now
  plumbs `authn.Authenticator` (static_bearer / oidc / forward_auth).
  `mtls` on legacy HTTP is rejected at config load with a recovery
  hint (terminate TLS upstream and use `forward_auth`, or use gRPC).
- **SSE GET origin/CORS parity (A3).** `GET /mcp` now applies the
  same `AllowedOrigins` list and CORS headers as `POST /mcp`.
- **Configurable OIDC verify-cache TTL (A4).**
  `MCP_OIDC_VERIFY_CACHE_TTL` replaces the hardcoded 60s ceiling
  (clamped to `[1s, 5m]`). Startup logs a warning when raised above
  the default so the revocation tradeoff is visible.
- **Transport × auth matrix test (A5).** Every supported and
  unsupported combination is locked down at `config.Load()`.
- **Per-subject rate-limiter eviction (B3).** Idle subject entries
  are reaped on a background ticker
  (`CLOCKIFY_SUBJECT_IDLE_TTL`, `CLOCKIFY_SUBJECT_SWEEP_INTERVAL`).
  The subjects map no longer grows unbounded.
- **SSE observability counters (B4).**
  `clockify_mcp_sse_subscriber_drops_total{reason}`,
  `clockify_mcp_sse_replay_misses_total`, and
  `clockify_mcp_sessions_reaped_total{reason}` surface hub / reaper
  eviction reasons that were previously silent.
- **File-store audit cap (B5).** `MCP_CONTROL_PLANE_AUDIT_CAP`
  bounds the in-memory audit slice on the file-backed control
  plane; FIFO eviction keeps dev deployments from growing forever.
- **Fail-closed dev-backend guard (C1).** `streamable_http` refuses
  to start against a `memory`/`file://` control plane unless
  `MCP_ALLOW_DEV_BACKEND=1` acknowledges the single-process limits.
- **Bootstrap + policy drift tests (D1).** Every name in
  `AlwaysVisible`, `MinimalSet`, `Tier1Catalog`, `introspection`,
  and `safeCoreWrites` must resolve to a registered tool.
- **`make verify-bench` Makefile target (D3).** Capture a baseline
  with `make bench BENCH_OUT=.bench/baseline.txt`, then
  `make verify-bench` diffs fresh profiles via `benchstat`.
- **Descriptor-runtime contract tests (D4).** `action` const in
  every outputSchema must match the tool name; Tier 2 descriptors
  must carry `readOnlyHint`/`destructiveHint`/`idempotentHint` in
  their Annotations map.
- **Protocol-version compat suite (E1).** Negotiation, capability
  shape, and dual-emit tools/call are now asserted across
  `2024-11-05`, `2025-03-26`, and `2025-06-18`.
- **ADR 0010 — metrics stack direction (E3, proposed).** Keep the
  homegrown metrics facade for v0.x; revisit with an OTel adapter
  on the ADR 0006 pattern at v1.0.
- **Postgres control-plane backend (B1).** pgx-backed
  `controlplane.Store` implementation lives in a dedicated
  `internal/controlplane/postgres` sub-module behind `-tags=postgres`
  so the default binary stays stdlib-only (ADR 0001). Selected by
  `MCP_CONTROL_PLANE_DSN=postgres://...`; migrations are embedded,
  run under a `pg_advisory_lock`, and version-tracked in a
  `schema_migrations` table. testcontainers-based integration tests
  cover round-trip, migration idempotence, and concurrent writes.
- **Control-plane schema compat guard (E2, ADR 0011).** The applier
  refuses to boot when the database reports a schema newer than the
  embedded migrations, protecting against silent rollback over a
  forward-only change. Integration test plants a bogus version and
  asserts the refuse-to-start error.
- **`RetainAudit(ctx, maxAge)` on Store + retention reaper (B2).**
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` (default 720h, range 1h–8760h,
  0 disables) drives a 1h ticker that drops old audit events from
  both the file store and the Postgres store.
  `clockify_mcp_audit_events_retained_total{outcome="deleted|error"}`
  exposes the per-tick outcome.
- **`internal/runtime` scaffold (C2.1).** Dev-backend predicate,
  control-plane store construction (C1 fail-closed guard included),
  and the retention reaper moved out of `cmd/clockify-mcp` so the
  boot-time plumbing is unit-testable and reusable.
- **Transport dispatch extraction (C2.2).** The streamable_http,
  legacy http, grpc, and stdio arms now live in
  `internal/runtime/{streamable,legacy_http,grpc,grpc_stub,stdio}.go`
  behind `Runtime.Run(ctx)`. `cmd/clockify-mcp/main.go` is a
  ~120-line boot shim (logging, signals, OTel, metrics listener,
  BuildInfo gauge) that delegates the rest. gRPC stays behind
  `//go:build grpc` with a stub for the default binary so the
  ADR 0012 stdlib-only guarantee holds. `auth.go:buildAuthnConfig`
  deduplicates the three previously drifting `authn.Config`
  constructions (grpc had omitted `MTLSTenantHeader` and
  `OIDCVerifyCacheTTL`).

### Added (infrastructure)

- **ADR 0011 — control-plane schema versioning.** Forward-only
  embedded migrations + refuse-to-boot-on-future-schema, with
  `internal/controlplane/COMPAT.md` tracking every version and
  interface addition.
- **`-tags=postgres` CI gate.** `scripts/check-build-tags.sh`
  asserts zero pgx symbols / zero pgx rows in the default build
  and that `-tags=postgres` actually links pgx.
- **`Makefile` targets** `build-postgres` and `test-postgres` for
  the sub-module.

### Changed

- **Legacy HTTP `ServeHTTP` signature** now takes an
  `authn.Authenticator`. Callers that passed only a bearer token
  construct one via `authn.New(authn.Config{Mode: ModeStaticBearer, …})`.
- **`controlplane.Open` accepts options.** Add
  `controlplane.WithAuditCap(n)` to cap the file-backed audit
  slice; back-compat: zero args keeps the historical unbounded
  behaviour.
- **`controlplane.Store` is now an interface** (B1.0). The
  file-backed implementation is renamed to `DevFileStore`; external
  backends (Postgres today) plug in via `RegisterOpener`. Callers
  that typed `*controlplane.Store` switch to the interface type;
  in-package tests type-assert to `*DevFileStore` when they need
  unexported state. A `Close()` method releases backend-owned
  resources (pool, handles); the file store returns nil.

### Docs

- **Runbook rename.** `docs/runbooks/w2-12-digest-pinning.md` is now
  `docs/runbooks/image-digest-pinning.md`. Content unchanged; the
  internal wave label is dropped from the filename and title so the
  runbook reads as a durable operator doc rather than a ticket
  reference. The three callers in `docs/production-readiness.md`,
  `docs/verification.md`, and `docs/verify-release.md` follow.
- Auth × transport matrix in `docs/production-readiness.md` and
  `README.md` now matches the code. mTLS-on-legacy-http is
  documented as rejected; OIDC TTL + dev-backend knobs are
  listed in the main env-var table.
- `docs/production-readiness.md` gains a "Pick a control-plane
  backend" section. `MCP_CONTROL_PLANE_DSN`,
  `MCP_CONTROL_PLANE_AUDIT_CAP`, and
  `MCP_CONTROL_PLANE_AUDIT_RETENTION` are documented in the
  README env-var table.

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
