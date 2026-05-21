# LOOP-PROMPT — re-read this whole file on every iteration

You are running inside a self-paced `/loop`. Your job is to drive an
adversarial reconciliation of three Clockify API sources, none of which
you trust, into a single 100%-truthful OpenAPI file at
`docs/goals/api-reconciliation/TRUTH.openapi.yaml`. Full context is in
`docs/goals/api-reconciliation/PLAN.md`. Re-read it if anything below is
ambiguous.

**Working directory:** `/Users/15x/Downloads/WORKING/addons-me/GOCLMCP`.
Confirm with `pwd` if unsure.

---

## Adversarial posture (read this every iteration)

1. **Trust nothing by default.** Not the official Clockify OpenAPI. Not our
   generated OpenAPI. Not your prior iteration's notes. Only PROBE-LOG
   entries from the live sacrificial workspace and dated within the last
   24 hours.
2. **No claim without a receipt.** Every fact you write into
   `TRUTH.openapi.yaml` that is not `UNCONFIRMED-AGREE` or
   `UNVERIFIABLE-DESTRUCTIVE` must cite at least one PROBE-LOG file path
   in its `x-evidence` array.
3. **Live overrides spec.** When live disagrees with either spec, live wins
   and the disagreement is logged in `DISCREPANCIES.md`.
4. **Probe negatives, not just happy path.** For every endpoint, attempt:
   happy-path, missing-required-param, wrong-type, wrong-method, empty-list
   (when applicable), pagination edges (when applicable). Each probe is its
   own PROBE-LOG file.
5. **"Both specs agree" is not verification.** That status is
   `UNCONFIRMED-AGREE`. Only a live probe earns `LIVE-VERIFIED`.
6. **Stale probes don't count.** A PROBE-LOG entry > 24 h old does not
   confirm anything; re-probe.

---

## Mode dispatch

Read state in this order. The first matching mode is the one you run this
iteration. Run exactly one mode per iteration.

| Mode | Trigger | Action section |
| --- | --- | --- |
| HALT-BUDGET | `LAST-ACTION.txt` shows ≥ 200 iterations OR elapsed wall-clock since `SOURCES/FETCHED-AT.txt` > 24 h | §H |
| HALT-DONE | `COMPLETION.md` exists | §X (no ScheduleWakeup) |
| FINAL | `DEVIL-ADVOCATE.md` exists AND it reports no outstanding demotions | §3 |
| DEVIL | `PROGRESS.md` exists AND has zero rows in `pending` or `in-progress` AND `DEVIL-ADVOCATE.md` does not exist | §2 |
| WORK | `PROGRESS.md` exists AND has rows in `pending` or `in-progress` | §1 |
| SETUP | `docs/goals/api-reconciliation/SOURCES/` does not exist | §0 |

If multiple triggers fire, the table above is in **priority order top-to-bottom**.

---

## §0 — SETUP mode

1. `mkdir -p docs/goals/api-reconciliation/{SOURCES,PROBE-LOG}`.
2. Fetch the official spec:
   `curl -sSfL https://docs.clockify.me/openapi.json -o docs/goals/api-reconciliation/SOURCES/official.openapi.json`.
   If the fetch fails, write a clear error to `LAST-ACTION.txt` and
   ScheduleWakeup for retry in 300 s. **Do not invent the spec.**
3. Capture metadata:
   ```
   echo "url=https://docs.clockify.me/openapi.json" > docs/goals/api-reconciliation/SOURCES/FETCHED-AT.txt
   date -u +"fetched_at_utc=%Y-%m-%dT%H:%M:%SZ" >> docs/goals/api-reconciliation/SOURCES/FETCHED-AT.txt
   shasum -a 256 docs/goals/api-reconciliation/SOURCES/official.openapi.json >> docs/goals/api-reconciliation/SOURCES/FETCHED-AT.txt
   ```
4. `cp docs/openapi/clockify-openapi.yaml docs/goals/api-reconciliation/SOURCES/ours.openapi.yaml`.
5. Enumerate every `<method> <path>` pair in **both** specs. Deduplicate.
   Generate a stable `endpoint_id` like `time-off.requests.list` from the
   path + method. Write `PROGRESS.md` with columns:
   `endpoint_id | method | path | source(official|ours|both) | status | last_probe_at | notes`.
   Initial status: `pending`. Sort by `endpoint_id`.
6. Initialize `TRUTH.openapi.yaml` with `openapi: 3.0.3`, `info` copied from
   our spec (we trust the meta; this isn't a fact about endpoints), and
   empty `paths: {}`.
7. Initialize `DISCREPANCIES.md` with this table header:
   ```
   | endpoint_id | fact | live-says | official-says | ours-says | chosen | evidence | losers |
   |---|---|---|---|---|---|---|---|
   ```
8. Initialize `PROBE-LOG/INDEX.md` with header:
   `| endpoint_id | probe_kind | timestamp_utc | file |`.
9. Run cleanup once: `bash scripts/live-clean-prefix MCP-LIVE-RECON-` (or
   the project's documented invocation). Confirm `Leftovers: 0`.
10. Append to `LAST-ACTION.txt`: `iteration=1; mode=SETUP; done_at=<UTC ts>; next=WORK`.
11. ScheduleWakeup with delaySeconds≈90 to start work immediately, prompt
    unchanged. **Do not** start work this iteration.

---

## §1 — WORK mode

You will reconcile **5 endpoints** this iteration. No more. No fewer unless
fewer than 5 are pending.

### 1.1 Iteration prologue

1. `bash scripts/live-clean-prefix MCP-LIVE-RECON-`. Confirm `Leftovers: 0`.
2. Set timestamp variable mentally: `TS=$(date -u +%Y%m%d-%H%M%S)`.
3. Read `PROGRESS.md`. Select up to 5 endpoints with status `pending` or
   `in-progress`:
   - Prefer read-only (`GET`) first.
   - Round-robin across categories (entries, projects, clients, tags,
     tasks, reports, time-off, scheduling, custom-fields, invoices,
     expenses, holidays, groups, users, webhooks, audit-logs, workspace).
   - Skip `UNVERIFIABLE-DESTRUCTIVE` candidates (these are handled in §2 +
     a future manual sweep).
4. For each selected endpoint, set its row to `in-progress` and record
   `last_attempt_iteration=<n>`.

### 1.2 Per-endpoint reconciliation

For each of the 5 endpoints, in order:

1. **Extract** the endpoint's spec from both `SOURCES/official.openapi.json`
   and `SOURCES/ours.openapi.yaml`. If either source is missing the
   endpoint, record that as a fact for DISCREPANCIES.md.
2. **Probe** the endpoint live. Use the connected Clockify MCP tools
   (`clockify_api_get` for GET; `clockify_api_request`
   for non-GET — but only after confirming the endpoint is reversible).
   Tag every created entity with prefix `MCP-LIVE-RECON-${TS}-`.
   - **Per-endpoint probe budget: 8 calls.** If you exhaust it without
     extracting all needed facts, mark the missing facts `UNRESOLVED-NO-LIVE`
     with `stuck_reason` and move on.
3. **Probes to attempt** (do them in order; abort the rest for this endpoint
   only when a hard 4xx makes further probes meaningless):
   - **happy-path:** minimal valid request. Capture response status, top-level
     keys, types, and pagination signals.
   - **missing-required:** strip one required param. Expect 4xx; record the
     code and error body shape.
   - **wrong-type:** send a known param with the wrong JSON type. Record.
   - **wrong-method:** send the same path with the opposite method (e.g.
     `GET` against a documented `POST` like
     `/workspaces/{}/time-off/requests`). Record method-correction evidence.
   - **empty-list:** filter to zero results (e.g.
     `name=zzz-nonexistent-${TS}`). Record empty-shape (`[]` vs `{items:[]}`).
   - **pagination-edge:** `page_size=1` then `page_size=200`. Record what
     the server actually returns vs requests (Clockify caps audit logs;
     account for this).
4. **Write each probe** to its own file in
   `PROBE-LOG/${TS}-<endpoint_id>-<probe_kind>.json` with:
   ```json
   {
     "endpoint_id": "...",
     "method": "...",
     "path": "...",
     "probe_kind": "happy-path|missing-required|wrong-type|wrong-method|empty-list|pagination-edge|other",
     "timestamp_utc": "...",
     "request": { "method": "...", "path": "...", "query": {...}, "headers_redacted": {...}, "body": ... },
     "response": { "status": 0, "headers_redacted": {...}, "body_keys": [...], "body_sample": "first 2 KB only" },
     "facts_observed": { ... },
     "notes": "..."
   }
   ```
   **Redact** `X-Api-Key`/`x-api-key`/any token before writing. Truncate
   `body_sample` to 2 KB. Append a row to `PROBE-LOG/INDEX.md`.
5. **Compare** the three sources on each fact: `method`, `required_query`,
   `optional_query`, `path_params`, `request_body`, `response_top_keys`,
   `response_item_shape`, `status_codes`, `pagination_style`,
   `auth_header_required`, `error_envelope`. For each fact:
   - If live present: live wins. If official disagrees, append a row to
     `DISCREPANCIES.md` with `losers=official` (and similarly for ours).
   - If no live for this fact and official == ours: status
     `UNCONFIRMED-AGREE` for this fact.
   - If no live for this fact and official != ours: status
     `UNRESOLVED-NO-LIVE`; pick the more credible side as a placeholder
     with `chosen_provisional`, but the operation's overall status downgrades.
6. **Write the merged operation** into `TRUTH.openapi.yaml` under
   `paths["<path>"]["<method>"]`. The operation object MUST include:
   ```yaml
   x-verification-status: LIVE-VERIFIED   # or one of the other 4
   x-evidence:
     - PROBE-LOG/${TS}-<endpoint_id>-happy-path.json
     - PROBE-LOG/${TS}-<endpoint_id>-empty-list.json
   x-reconciled-at: ${TS}
   ```
   Status rules:
   - `LIVE-VERIFIED` requires at least one live probe and zero
     unreconciled discrepancies among collected facts.
   - `LIVE-OVERRIDE` if live disagreed with at least one source.
   - `UNCONFIRMED-AGREE` if no live probe was attempted and the two specs
     agree (rare; only when destructive and a probe is unsafe).
   - `UNRESOLVED-NO-LIVE` if specs disagree and no safe live probe was
     possible.
   - `UNVERIFIABLE-DESTRUCTIVE` for endpoints we will not probe (subscription
     changes, owner transfer, billing). Include `x-unverifiable-reason`.
7. **Update PROGRESS.md** for this endpoint: status `probed`,
   `last_probe_at=${TS}`.

### 1.3 Iteration epilogue

1. `bash scripts/live-clean-prefix MCP-LIVE-RECON-`. Confirm `Leftovers: 0`.
   If not zero, log to LAST-ACTION.txt and do NOT exit — clean again.
2. Validate yaml: `python3 -c "import yaml; yaml.safe_load(open('docs/goals/api-reconciliation/TRUTH.openapi.yaml'))"`.
   If parse fails, fix only the syntax (do not invent semantics) and
   re-validate.
3. **Sanity-grep TRUTH.yaml.** Any operation marked `LIVE-VERIFIED` MUST
   have at least one `x-evidence` entry that resolves to an existing file.
   If a violation is found, downgrade the offender to `UNRESOLVED-NO-LIVE`
   in this iteration and log to DISCREPANCIES.md as
   `fact=internal-bookkeeping`.
4. Append to `LAST-ACTION.txt`:
   `iteration=<n>; mode=WORK; endpoints=[<csv of ids>]; done_at=<UTC ts>; next=WORK-or-DEVIL`.
5. ScheduleWakeup with delaySeconds=90 (under 5 min keeps prompt cache warm),
   prompt unchanged. Exit cleanly.

---

## §2 — DEVIL'S ADVOCATE mode

You have just learned everything is "done". Assume that's a lie.

1. `bash scripts/live-clean-prefix MCP-LIVE-RECON-`.
2. Sample **20** `LIVE-VERIFIED` endpoints uniformly across categories
   (use a deterministic stride; document the stride in DEVIL-ADVOCATE.md).
3. For each, re-probe with a **different** `probe_kind` than the original.
   If you only have `happy-path` evidence, do `missing-required` or
   `wrong-method`. If you only have read probes, do an empty-list probe.
4. For each disagreement between the re-probe and TRUTH.yaml, file a row
   in DEVIL-ADVOCATE.md:
   ```
   | endpoint_id | fact | truth-said | re-probe-said | new-evidence | new-status |
   ```
   And demote the operation's `x-verification-status` to `LIVE-OVERRIDE`
   with the new evidence appended.
5. Sample **10** `UNCONFIRMED-AGREE` endpoints. For each, attempt a safe
   live probe. Promote to `LIVE-VERIFIED` on success. Leave alone if truly
   unprobable (note the reason).
6. Write a complete DEVIL-ADVOCATE.md:
   - sampling stride used
   - the 30 sampled endpoint_ids
   - per-endpoint outcome (confirmed | demoted | promoted)
   - any new DISCREPANCIES.md entries created
7. `bash scripts/live-clean-prefix MCP-LIVE-RECON-`. Confirm `Leftovers: 0`.
8. Append to `LAST-ACTION.txt`:
   `iteration=<n>; mode=DEVIL; done_at=<UTC ts>; next=FINAL`.
9. ScheduleWakeup with delaySeconds=90, prompt unchanged.

If demotions during devil's advocate created new `in-progress` or `pending`
work (e.g. operations needing fresh probes), the next iteration will run WORK
mode again. That is correct — repeat WORK → DEVIL cycles until DEVIL finds
nothing.

---

## §3 — FINAL mode

1. Parse TRUTH.openapi.yaml. Count operations by `x-verification-status`.
2. Validate against OpenAPI 3.x. If you can't run a validator, do at least:
   `python3 -c "import yaml,sys; d=yaml.safe_load(open('docs/goals/api-reconciliation/TRUTH.openapi.yaml')); assert d.get('openapi','').startswith('3.'); assert 'paths' in d"`.
3. Write `COMPLETION.md`:
   - timestamps for setup, first WORK, last WORK, DEVIL, FINAL
   - counts by status
   - list of every `UNRESOLVED-NO-LIVE` and `UNVERIFIABLE-DESTRUCTIVE`
     operation with its reason
   - probe count and total bytes (`find PROBE-LOG -name '*.json' | wc -l`,
     `du -sh PROBE-LOG`)
   - one-paragraph honest summary: did we reach "100% truthful"? what's
     left? what next?
4. Final sweep: `bash scripts/live-clean-prefix MCP-LIVE-RECON-`.
5. Append `LAST-ACTION.txt`: `iteration=<n>; mode=FINAL; done_at=<UTC ts>; next=EXIT`.
6. **Do not** call ScheduleWakeup. The loop exits.

---

## §H — HALT-BUDGET mode

1. Run `bash scripts/live-clean-prefix MCP-LIVE-RECON-`.
2. Write `BUDGET-HALT.md` with the current PROGRESS.md status counts and a
   one-line reason (`iterations>=200` or `elapsed>24h`).
3. Update LAST-ACTION.txt: `mode=HALT-BUDGET; done_at=<UTC ts>; next=EXIT`.
4. **Do not** call ScheduleWakeup. Exit.

## §X — HALT-DONE mode

The loop already finished. Do nothing except confirm via a no-op text:
`Reconciliation already complete; see COMPLETION.md`. Do not ScheduleWakeup.

---

## Anti-patterns (do not do)

- **Do not** invent endpoints that aren't in either spec just because you
  saw a related URL pattern.
- **Do not** mark `LIVE-VERIFIED` based on memory of an earlier iteration —
  the rule is *fresh probe within 24 h, attached to this operation*.
- **Do not** edit `docs/openapi/clockify-openapi.yaml`, `internal/`, or any
  file outside `docs/goals/api-reconciliation/`. The reconciliation is
  read-only with respect to the MCP source.
- **Do not** skip cleanup at iteration end because PROGRESS.md looks tidy.
  The workspace doesn't read PROGRESS.md.
- **Do not** widen the per-iteration scope to "just one more endpoint".
  Five. Always five.
- **Do not** echo or log secret values. `X-Api-Key` / `x-api-key` /
  workspace IDs are redacted in PROBE-LOG entries.
- **Do not** call ScheduleWakeup in §3 or §H or §X. The loop must exit
  cleanly in those modes.
- **Do not** trust the official spec just because it's authoritative —
  the receipts in AGENTS.md "Known Clockify API Gotchas" show it lies.

---

## Tone

You are skeptical. You are precise. You distrust everyone, including
yourself five iterations ago. When in doubt, you probe. When the probe
disagrees with the spec, you side with the probe and you write down which
spec lied and where. You finish the iteration on schedule with a clean
workspace, or you log why you didn't.

Now: read mode dispatch, run exactly one mode, write your evidence, exit.
