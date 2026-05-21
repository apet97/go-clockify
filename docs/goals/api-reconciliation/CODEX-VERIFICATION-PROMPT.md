# Prompt for Codex — Independently verify the post-handoff fixes

Paste the section below into Codex (or any independent agent with shell + curl + a YAML/OpenAPI toolchain). It is self-contained; do not summarize or paraphrase.

---

You are auditing another agent's work. Do **not** trust its summary. Verify each claim against the repository state and the live Clockify API. Be adversarial: try to demote claims, not confirm them.

## Context

- **Repo:** `/Users/15x/Downloads/WORKING/addons-me/GOCLMCP`
- **Working folder:** `docs/goals/api-reconciliation/`
- **Spec under audit:** `docs/goals/api-reconciliation/TRUTH.openapi.yaml`
- **Handoff report:** `docs/goals/api-reconciliation/REPORT-FOR-CODEX.md` (read this first, but verify every claim)
- **Original completion notes:** `docs/goals/api-reconciliation/COMPLETION.md`
- **User-supplied canonical official specs (treat as authoritative for path/method/required fields):**
  - `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/BALANCEOPEANI.yaml`
  - `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/HOLIDAYSOPEAPI.YAML`
  - `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/POLICIESOPENAPI.YAML`
  - `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/TIMEOFFOPENAPI.YAML`
  - …plus the other 12 `.YAML` files in that directory (approvals, attendance, CFs, expenses, invoices, projects, scheduling, tasks, user-groups, user, webhooks, workspace).

- **Environment for live probes:** `CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` are set. The workspace is sacrificial. You may freely create/update/delete test entities, but **never delete a user, never delete yourself, never create a new workspace** (billing).
- **Auth header:** `X-Api-Key: $CLOCKIFY_API_KEY`. Endpoints live on TWO hosts:
  - `https://api.clockify.me/api/v1/...` (main)
  - `https://reports.api.clockify.me/v1/...` (shared-reports — note no `/api` segment)

## Verification tasks

Run these in order. Treat each as pass/fail. Report exact evidence (curl output, file diffs, validator output) for every claim — do not summarize.

### 1. Spec validity

```bash
cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP/docs/goals/api-reconciliation
openapi-spec-validator TRUTH.openapi.yaml
npx --yes @apidevtools/swagger-cli validate TRUTH.openapi.yaml
npx --yes @redocly/cli lint --max-problems 1000 TRUTH.openapi.yaml
```

Expected: each tool reports the file valid. `@redocly/cli lint` should emit no warnings or errors under the local reconciliation `redocly.yaml` policy. Do not consider the audit complete if any tool errors or warns.

### 2. Counts and provenance integrity

Confirm tally:

```bash
python3 -c "
import yaml
from collections import Counter
s = yaml.safe_load(open('TRUTH.openapi.yaml'))
c = Counter()
for p, methods in s['paths'].items():
    for m, op in methods.items():
        if m in ('get','post','put','patch','delete'):
            c[op.get('x-verification-status','MISSING')] += 1
print('paths:', len(s['paths']))
print('ops:', sum(c.values()))
for k, v in c.most_common():
    print(f'  {k}: {v}')
"
```

Current expected tally after reconciliation closure: 128 paths, 200 ops, statuses {LIVE-VERIFIED:148, LIVE-OVERRIDE:50, UNCONFIRMED-AGREE:1, UNRESOLVED-NO-LIVE:0, UNVERIFIABLE-DESTRUCTIVE:1}. **Compute the actual numbers; report any deviation.**

Additionally, every op must have an `x-evidence` array (or be a documented UNVERIFIABLE/UNCONFIRMED). Scan for ops with missing or empty `x-evidence`.

### 3. Live re-probe of every change made in this session

For each of the following, run the probe yourself and compare against the recorded shape in `TRUTH.openapi.yaml`. **Do not trust the saved `PROBE-LOG/*` files — re-fetch.**

#### 3a. Shared-reports (5 ops, reports host)

```bash
WS="$CLOCKIFY_WORKSPACE_ID"
H="X-Api-Key: $CLOCKIFY_API_KEY"
BASE="https://reports.api.clockify.me/v1"

# List
curl -sS -H "$H" "$BASE/workspaces/$WS/shared-reports?pageSize=5" | head -c 800; echo

# Pick any id from the list, then GET by id (NOT workspace-scoped)
RID="<one-id-from-list>"
curl -sS -H "$H" "$BASE/shared-reports/$RID?exportType=JSON" | head -c 800; echo

# Create / update / delete a throwaway report (cleanup required)
curl -sS -H "$H" -H 'Content-Type: application/json' -X POST "$BASE/workspaces/$WS/shared-reports" -d '{
  "name":"CODEX-VERIFY-'"$(date -u +%s)"'","type":"SUMMARY","isPublic":false,"fixedDate":true,
  "filter":{"dateRangeStart":"2026-04-01T00:00:00","dateRangeEnd":"2026-04-30T23:59:59",
            "dateRangeType":"LAST_MONTH","exportType":"JSON","timeZone":"UTC","weekStart":"MONDAY",
            "summaryFilter":{"groups":["PROJECT"]}}}'
# capture new id, then PUT, then DELETE; confirm DELETE returns 204
```

Verify (must match the spec):
- List returns wrapped `{reports:[...], count:N}` — **not** a bare array.
- GET-by-id is NOT workspace-scoped on the reports host; workspace-scoped GET returns 405.
- POST without `filter.dateRangeStart/End` → 400. POST with `type=SUMMARY` and no `summaryFilter` → 501.
- DELETE returns 204 no body.
- Same path on `api.clockify.me/api/v1/...` returns 404 (`No static resource`).

#### 3b. `GET /workspaces/{wId}/users` direct curl

```bash
curl -sSI -H "$H" "https://api.clockify.me/api/v1/workspaces/$WS/users?page-size=1&page=1"
curl -sS -H "$H" "https://api.clockify.me/api/v1/workspaces/$WS/users?page-size=1&page=1" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert isinstance(d, list), 'expected bare array'
print('count=', len(d), 'first-keys=', list(d[0].keys()))
"
```

Verify: status 200, bare array, response cardinality matches `page-size=1`. If page-size returns >1, the recorded "pagination respected" claim is wrong — flag it.

### 4. Cross-check against user-supplied canonical specs

For each YAML in `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/`, extract `paths × methods × required-body-fields × required-query-params` and diff against `TRUTH.openapi.yaml`. Use:

```bash
python3 - <<'PY'
import yaml, glob, os
truth = yaml.safe_load(open('TRUTH.openapi.yaml'))
truth_ops = {(p.replace('/v1',''), m): truth['paths'][p][m]
             for p in truth['paths']
             for m in truth['paths'][p]
             if m in ('get','post','put','patch','delete')}

for f in sorted(glob.glob('/Users/15x/Downloads/WORKING/clockify-api-probe-lab/realOPENAPI/*')):
    name = os.path.basename(f)
    s = yaml.safe_load(open(f))
    for p, methods in (s.get('paths') or {}).items():
        # canonical paths embed /v1 — normalize
        np = p.replace('/v1','')
        for m, op in methods.items():
            if m not in ('get','post','put','patch','delete'): continue
            key = (np, m)
            if key not in truth_ops:
                print(f'[MISSING in TRUTH] {name} :: {m.upper()} {p}')
PY
```

Any `[MISSING in TRUTH]` lines are gaps. Investigate each — is it absent on live, or is it a real missing entry?

### 5. Path consolidation correctness

The handoff merged two alias paths into one:
- Removed: `/workspaces/{workspaceId}/shared-reports/{sharedReportId}` (DELETE)
- Kept: `/workspaces/{workspaceId}/shared-reports/{id}` (PUT + DELETE)

Verify the removed path no longer exists in the spec AND that no operationId is orphaned. Grep:

```bash
grep -n 'sharedReportId' TRUTH.openapi.yaml || echo 'no sharedReportId — good'
```

Confirm `delete.workspaces.shared-reports` operationId now lives on the `{id}` path.

### 6. Residual status attack — try to promote remaining non-live rows

No `UNRESOLVED-NO-LIVE` ops remain. Two non-live rows remain partitioned out of hard truth assertions:
1. `POST /workspaces/{workspaceId}/users` — currently `UNCONFIRMED-AGREE`; live disposable invite attempts were blocked by the sacrificial workspace seat limit.
2. `POST /workspaces` — currently `UNVERIFIABLE-DESTRUCTIVE`; creating an account-level workspace is outside the sacrificial-workspace cleanup envelope.

For each row you can safely promote with direct evidence, update `TRUTH.openapi.yaml` with the appropriate status, add evidence to `PROBE-LOG/`, and re-run section 1.

### 7. Safety/cleanup

- After every probe that creates an entity (shared reports, projects, policies), DELETE it before the audit ends.
- Use prefix `CODEX-VERIFY-<unix-ts>` for every test name so any leftovers are easy to find.
- Run `scripts/live-clean-prefix --prefix CODEX-VERIFY-` (or equivalent) at the end and report leftover count.

## Deliverables

1. A new file `docs/goals/api-reconciliation/CODEX-VERIFICATION-RESULTS.md` containing:
   - Section-by-section pass/fail with exact evidence.
   - Any defects found in the handoff work, with the corrective edit (PR-style diff).
   - Updated tallies if anything changed.
2. A list of every leftover test entity (should be empty).
3. A go/no-go verdict on whether `TRUTH.openapi.yaml` is ready to be used as the source of truth for the `nightly-api-drift-detection` goal (`docs/goals/nightly-api-drift-detection.md`).

## Hard rules

- Do **not** delete any user (`DELETE /users/{userId}`). Do **not** create a workspace. Do **not** invite real users or expose emails; any disposable invite probe requires explicit operator authorization and redacted receipts.
- Do **not** trust the handoff agent's summary — verify every numeric claim.
- Do **not** skip a section because it "looks fine" — run the command and paste output.
- If you discover the handoff agent was wrong, say so plainly with evidence.
