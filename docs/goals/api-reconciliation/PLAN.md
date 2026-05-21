# Plan: produce a 100% truthful Clockify OpenAPI

Adversarial reconciliation of three sources, none of which is trusted:

1. **Official Clockify OpenAPI** at <https://docs.clockify.me/openapi.json>.
2. **Our generated OpenAPI** at `docs/openapi/clockify-openapi.yaml`.
3. **Live Clockify** behavior against the sacrificial workspace.

Output: a sovereign new file `docs/goals/api-reconciliation/TRUTH.openapi.yaml`
in which every fact (method, path, required params, optional params, request
body, response shape, status codes, pagination, auth, errors) is tagged with
a verification status and a pointer to the evidence that proves it.

This is **not** a refactor of our existing OpenAPI generator. That can come
later, once we know what the truth actually is.

## Why this is hard

Each source lies in a different direction:

- Official spec lies because Clockify hasn't kept it current (AGENTS.md's
  "Known Clockify API Gotchas" list catalogs 27+ confirmed cases, e.g.
  time-off-list is POST not GET, holiday update is partial, etc.).
- Our generated spec lies because it reflects only what we've *used* — it
  may overspecify or underspecify and inherits our handler shape, not the
  upstream shape.
- Live Clockify is ground truth for whatever we actually probe, but only
  for that day, that workspace, that auth, that input.

"Three agree" is the only `VERIFIED` floor. Two sources agreeing without
a fresh live probe is `UNCONFIRMED-AGREE`. Disagreements without a live
probe are `UNRESOLVED-NO-LIVE`.

## Deliverables

In `docs/goals/api-reconciliation/`:

| File | Purpose |
| --- | --- |
| `TRUTH.openapi.yaml` | The reconciled spec. Every operation carries `x-verification-status` and `x-evidence` pointers. |
| `PROGRESS.md` | Ledger of every endpoint × {pending, in-progress, probed, reconciled, devil-advocated}. |
| `DISCREPANCIES.md` | One row per finding: `endpoint`, `fact`, `official-says`, `ours-says`, `live-says`, `chosen-truth`, `evidence-paths`, `loser-list`. |
| `SOURCES/official.openapi.json` | Snapshot of the official spec at setup time, with a sibling `FETCHED-AT.txt`. |
| `SOURCES/ours.openapi.yaml` | Snapshot of our generated spec at setup time. |
| `PROBE-LOG/YYYY-MM-DD-HHMMSS-<endpoint-id>-<probe-kind>.json` | Raw probe request/response, one file per probe. These are the receipts. |
| `PROBE-LOG/INDEX.md` | Index by endpoint → list of probe files. |
| `DEVIL-ADVOCATE.md` | Final-phase contradiction-hunt report. |
| `COMPLETION.md` | End-of-run report: counts, what's verified, what's not, why. |

## Verification statuses

Every operation in `TRUTH.openapi.yaml` carries `x-verification-status`:

| Status | Meaning |
| --- | --- |
| `LIVE-VERIFIED` | All facts confirmed by at least one fresh live probe (≤ 24 h old) with a PROBE-LOG entry. Three sources agree (or live overrode the others). |
| `UNCONFIRMED-AGREE` | Official and ours agree; no live probe possible (e.g. destructive admin endpoint we won't fire). |
| `LIVE-OVERRIDE` | Live disagreed with at least one source; live wins. Disagreement detail in DISCREPANCIES.md. |
| `UNRESOLVED-NO-LIVE` | Sources disagree, no safe live probe was possible. Flagged for manual review. |
| `UNVERIFIABLE-DESTRUCTIVE` | Endpoint mutates state we can't safely create or revert (subscription, owner transfer, etc.). Recorded as-stated by the more credible source plus a written reason. |

The TRUTH file is "100% truthful" iff every endpoint is `LIVE-VERIFIED`,
`LIVE-OVERRIDE`, or has an explicit `UNVERIFIABLE-DESTRUCTIVE` justification.
`UNCONFIRMED-AGREE` and `UNRESOLVED-NO-LIVE` are work-in-progress markers, not
final.

## Adversarial rules (non-negotiable)

1. **No claim without a receipt.** Every fact written into TRUTH.openapi.yaml
   that is not `UNCONFIRMED-AGREE` or `UNVERIFIABLE-DESTRUCTIVE` must cite a
   PROBE-LOG file in its `x-evidence` array.
2. **Default trust = zero.** Do not write a fact as truth just because both
   specs say so. That earns `UNCONFIRMED-AGREE`, not `LIVE-VERIFIED`.
3. **Live wins.** When live disagrees with either spec, live is truth and the
   disagreement goes into DISCREPANCIES.md.
4. **Stale probes don't count.** If the most recent PROBE-LOG entry for an
   endpoint is > 24 h old, the endpoint is no longer `LIVE-VERIFIED`. Re-probe.
5. **Probe more than the happy path.** For each endpoint, attempt at minimum:
   - happy path (valid inputs)
   - missing required param
   - wrong type for a known param
   - wrong HTTP method (e.g. GET against a POST endpoint)
   - empty result (filter to zero matches)
   - pagination edges where applicable (`page_size=1`, `page_size=200`)
   Each probe is its own PROBE-LOG file with `probe_kind` set.
6. **Negative results are facts.** A 405 on `GET /time-off/requests` is the
   strongest evidence we can collect for the POST-not-GET gotcha. Record it.
7. **Devil's advocate phase is mandatory.** When all endpoints have at least
   one verification pass, the loop re-enters and re-probes a sample to look
   for self-contradictions in TRUTH.yaml. No exit without it.

## Probe taxonomy and write discipline

| Probe class | Endpoint shape | Strategy |
| --- | --- | --- |
| Read-only | `GET` | Probe freely. ~70% of endpoints. |
| Write-reversible | `POST` / `PUT` / `PATCH` that we can delete after | Create with prefix, assert, delete with the prefix sweeper. |
| Write-irreversible | `PUT /workspaces/{}/billing-info`, owner transfer, subscription changes | Do not probe. Record `UNVERIFIABLE-DESTRUCTIVE` with reason. |
| Side-effect | Webhook test, invoice send | Do not probe in the loop. Already documented as `unsupported` in our MCP. Record `UNVERIFIABLE-EXTERNAL`. |

**Workspace cleanup is mandatory at iteration start and iteration end.** Every
created entity uses prefix `MCP-LIVE-RECON-YYYYMMDD-HHMM-`. The loop runs
`scripts/live-clean-prefix MCP-LIVE-RECON-` at the start of each iteration to
clear stragglers from prior runs, and again at the end. The expectation,
mirroring CLAUDE.md, is `Leftovers: 0` after the sweep.

## Phases

The loop is one prompt that dispatches on state. Each iteration ends by
writing `LAST-ACTION.txt` so the next iteration can pick up cleanly.

### Phase 0 — Setup (first iteration only)

Triggered when `SOURCES/` is missing. Actions:

1. `mkdir -p docs/goals/api-reconciliation/{SOURCES,PROBE-LOG}`.
2. Fetch the official spec to `SOURCES/official.openapi.json`. Record fetch
   time in `SOURCES/FETCHED-AT.txt` with the URL and SHA256.
3. Copy `docs/openapi/clockify-openapi.yaml` to `SOURCES/ours.openapi.yaml`.
4. Build `PROGRESS.md` by enumerating every operation in both specs, deduping
   on `<method> <path>`. Sort. Mark each `pending`.
5. Initialize `TRUTH.openapi.yaml` with `info`, `servers`, and empty `paths`.
6. Initialize `DISCREPANCIES.md` with the table header.
7. Initialize `PROBE-LOG/INDEX.md` with the table header.
8. Run `scripts/live-clean-prefix MCP-LIVE-RECON-` once to start clean.
9. Write `LAST-ACTION.txt`: `phase=setup; done=<UTC ts>`.

### Phase 1 — Reconciliation work (most iterations)

Triggered when PROGRESS.md has rows in `pending` or `in-progress`.

For each iteration, pick a batch of **5 endpoints** by:

- Skip `UNVERIFIABLE-DESTRUCTIVE` endpoints (handled in a separate sweep).
- Prefer read-only endpoints first, then write-reversible.
- Round-robin across categories (entries, projects, clients, tags, tasks,
  reports, …) so a single category's quirks don't dominate one iteration.

For each endpoint in the batch:

1. Mark `in-progress` in PROGRESS.md.
2. Run the probe taxonomy (happy + minimum 3 negatives + pagination edges
   when applicable). Write each probe to PROBE-LOG.
3. Extract facts from official, ours, and live.
4. Compare on every fact. Record disagreements in DISCREPANCIES.md.
5. Write the merged truth into `TRUTH.openapi.yaml` under the endpoint's
   `paths` entry, with `x-verification-status` and `x-evidence`.
6. Mark `probed` in PROGRESS.md.
7. End iteration: cleanup, update LAST-ACTION.txt, ScheduleWakeup.

### Phase 2 — Devil's advocate (one iteration)

Triggered when PROGRESS.md has no `pending` or `in-progress` rows. Actions:

1. Sample 20 random `LIVE-VERIFIED` endpoints uniformly across categories.
2. Re-probe each one with a different probe shape than what was originally
   captured (e.g. if the original was happy-path, do a negative; if the
   original was an empty-list, do a populated list).
3. For each mismatch with what TRUTH.yaml says, file a `DEVIL-FINDING` row in
   DEVIL-ADVOCATE.md and demote the endpoint's status to `LIVE-OVERRIDE` with
   a new evidence entry.
4. Pick 10 random `UNCONFIRMED-AGREE` endpoints. Try harder to find a safe
   live probe. Promote on success; leave the status if truly unprobable.
5. Write DEVIL-ADVOCATE.md with the full audit report.
6. End iteration: cleanup, update LAST-ACTION.txt, ScheduleWakeup.

### Phase 3 — Final stamp (one iteration, terminal)

Triggered when DEVIL-ADVOCATE.md exists and no demotions are still pending.
Actions:

1. Validate TRUTH.openapi.yaml as YAML and as OpenAPI 3.x.
2. Generate COMPLETION.md: counts per status, list of `UNRESOLVED-NO-LIVE`
   and `UNVERIFIABLE-DESTRUCTIVE` operations, raw probe count, total probe
   bytes, elapsed time.
3. Final cleanup pass: `scripts/live-clean-prefix MCP-LIVE-RECON-`.
4. Update LAST-ACTION.txt: `phase=final; done=<UTC ts>`.
5. **Do not call ScheduleWakeup.** The loop exits.

## Exit criteria

The loop terminates only when **all** are true:

- PROGRESS.md has zero `pending` and zero `in-progress` rows.
- DEVIL-ADVOCATE.md exists and reports no outstanding demotions.
- COMPLETION.md exists.
- `TRUTH.openapi.yaml` parses as valid OpenAPI 3.x.
- Workspace sweep returns `Leftovers: 0`.

## Model, launch, and budget

- **Model: Sonnet 4.6.** Cheaper per iteration; tool-use reliability is fine
  for this work. Reserve Opus 4.7 manually for stuck iterations
  (`/model opus`, run one iteration, then `/model sonnet`).
- **Per-iteration target: < 5 minutes.** Keeps the Anthropic prompt cache
  warm. Five endpoints per batch is sized for this.
- **Hard budget: 200 iterations or 24 hours, whichever first.** Encoded as
  a check in the loop prompt. If the budget hits before exit criteria,
  the loop writes a `BUDGET-HALT.md` and exits.
- **Launch:**
  ```
  cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
  claude --model sonnet
  /loop Read docs/goals/api-reconciliation/LOOP-PROMPT.md and execute it. Self-pace.
  ```
  The `/loop` prompt is intentionally short. The substance lives in
  `LOOP-PROMPT.md` and is re-read on every iteration.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Workspace junk accumulates | Prefix sweep at start and end of every iteration. Hard `MCP-LIVE-RECON-` prefix everywhere. |
| Probe runs hit Clockify rate limits | One iteration = one batch of 5 endpoints, paced over up to 5 minutes. If rate-limited, the iteration records the 429 as a probe result and skips the offending fact (not the whole iteration). |
| Sonnet hallucinates a fact without a probe | Adversarial rule (1) is enforced by the LOOP-PROMPT.md's "before writing TRUTH" checklist. The post-iteration sanity check greps TRUTH.yaml for any `x-verification-status` not in the allowed enum and any `LIVE-VERIFIED` entry without an `x-evidence` pointer. |
| Sonnet trusts the official spec because it's "official" | Adversarial rule (2) is restated at the top of the LOOP-PROMPT.md and reinforced by the column ordering in DISCREPANCIES.md (live-says before either spec). |
| Loop gets stuck on one endpoint | Per-endpoint budget: 8 probes max. After that, mark `UNRESOLVED-NO-LIVE` with a `stuck-reason` and move on. The devil's advocate phase will return to it. |
| Loop exits before doing devil's advocate | Phase gating in the prompt. Cannot enter Phase 3 without Phase 2 artifacts on disk. |
| Loss of session context between iterations | All state is in files. The prompt is self-contained and re-read each iteration. No conversational memory is required. |

## Manual escape hatches

- **Abort cleanly:** `/exit` and do not relaunch. Re-running the same `/loop`
  prompt picks up where it left off.
- **Full reset:** `rm -rf docs/goals/api-reconciliation/{PROGRESS.md,DISCREPANCIES.md,TRUTH.openapi.yaml,PROBE-LOG,DEVIL-ADVOCATE.md,COMPLETION.md,LAST-ACTION.txt,SOURCES}` then relaunch `/loop`. Keeps PLAN.md and LOOP-PROMPT.md.
- **Partial reset (re-do a specific endpoint):** in PROGRESS.md set its row
  back to `pending` and remove its probe files from PROBE-LOG; the loop will
  re-probe.
- **Force Opus for one iteration:** `/model opus`, watch one iteration land,
  then `/model sonnet`.

## What this plan deliberately does not do

- Patch `docs/openapi/clockify-openapi.yaml` or its generator. That is a
  follow-up.
- Update `AGENTS.md` Known-Gotchas list. That is a follow-up that consumes
  TRUTH.openapi.yaml as input.
- Touch any code in `internal/`. Reconciliation is read-only with respect to
  the MCP code base.
- Make any non-sacrificial-workspace API calls.
