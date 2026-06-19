# Findings — rates and reports money

Probe: `probes/rates-and-reports.sh` (PROBE_MUTATE=1).
Run: 2026-05-12T11:12Z, workspace `<REDACTED_ID>`, currency EUR.

## TL;DR

**Every monetary surface in Clockify is integer-cents (minor units of
the currency, no extra factor).** The primary API returns
`RateDtoV1{amount:int, currency:string}` where `amount` is **cents**.
The reports API (`reports.api.clockify.me/v1`) returns JSON `number`
fields (`amount`, `value`, `totalAmount`, `rate`, `earnedAmount`,
`costAmount`) that are still **cents** as decimals (e.g. `4321.0`
means €43.21). The MCP must render every one of them as
`amount / 100` with 2 dp.

## Scale evidence

We set the **task** hourly rate to amount `4321` cents (€43.21/hr)
and cost rate `876` cents (€8.76/hr), logged a 1-hour entry, and
read every monetary surface back:

| Surface | Field | Value returned | Interpretation |
|---|---|---|---|
| `GET /workspaces/{ws}/projects/{id}/tasks/{id}` | `hourlyRate.amount` | `4321` (int) | cents |
| `GET …` (task) | `costRate.amount` | `876` (int) | cents |
| `GET /workspaces` | `hourlyRate.amount` of workspace | `150` (int) | cents (€1.50/hr) |
| `GET /workspaces` | per-member `hourlyRate.amount` | `20000` (int) | cents (€200/hr) |
| `POST {reports}/reports/detailed` | `totals[0].amounts[type=EARNED].value` | `4321.0` (number) | cents |
| `POST …/reports/detailed` | `timeentries[0].rate`, `.amount`, `.earnedAmount`, `.earnedRate` | `4321.0` | cents |
| `POST …/reports/detailed` | `timeentries[0].costAmount`, `.costRate` | `876.0` | cents |
| `POST …/reports/detailed` | `totals[].amounts[type=PROFIT].value` | `3445.0` | cents (4321−876) |
| `POST …/reports/summary` | `groupOne[].amount` | `4321.0` | cents |
| `POST …/reports/summary` | `groupOne[].amounts[].value` | per-type cents | cents |
| `POST …/reports/summary` | `groupOne[].children[].amount`, `.amounts[].value` | recursive, cents | cents |
| `POST …/reports/summary` | `groupOne[].amounts[].amountByCurrency[].amount` | `4321.0` per currency | cents per currency |

Conclusion: **divide every monetary field by 100 to get currency
units**. No mixed scales.

## Live-status promotion scaffold

Rows below with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace on
2026-06-19. The detailed report body key is `timeentries` (lowercase `e`),
matching the live payload.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | reports.api.clockify.me | /workspaces/{workspaceId}/reports/summary | 200 | fixtures/live-shape/reports-summary.json |
| POST | reports.api.clockify.me | /workspaces/{workspaceId}/reports/detailed | 200 | fixtures/live-shape/reports-detailed.json |
| POST | reports.api.clockify.me | /workspaces/{workspaceId}/reports/weekly | 200 | fixtures/live-shape/reports-weekly.json |
| POST | reports.api.clockify.me | /workspaces/{workspaceId}/reports/attendance | 200 | fixtures/live-shape/reports-attendance.json |

## Per-entry money in detailed report (JSON_V1)

`timeentries[i]` carries the following monetary fields — go-clockify
does NOT need to derive them client-side:

```
amount         number   total billable amount for the entry (cents)
rate           number   effective hourly rate (cents) used to compute amount
earnedAmount   number   same as amount when amountShown=EARNED
earnedRate     number   same as rate
costAmount     number   internal cost for the entry (cents)
costRate       number   effective cost-hourly rate (cents)
currency       string   ISO code, e.g. "EUR"
```

So the MCP should pass these through verbatim into the typed
`MoneyView` envelope, not re-multiply.

## Summary report shape

Top-level fields actually returned (workspace tested on standard
plan):

```
totals[]          aggregate row (totalTime, totalBillableTime, entriesCount,
                  totalAmount, totalAmountByCurrency[], amounts[]{type,value,
                  amountByCurrency[]}, numOfCurrencies, _id)
groupOne[]        flat list at outermost grouping (PROJECT in our run); each
                  carries: currency, workspaceCurrencyCode, duration,
                  amount (number, cents), amounts[] (per-type), _id, name,
                  nameLowerCase, color, clientName, children[]
                  Recurses one level per additional grouping selected
                  (we requested [PROJECT, DATE] -> children keyed by date,
                  with the same {amount, amounts[], days[]} shape).
donutChart[]      empty in our run; documented as SummaryReportChartDto
groupTotals       {groupOneTotalCount, groupTwoTotalCount}
```

No `chart[]` field at the top level despite the OpenAPI doc claiming
one — that lives under `donutChart` in practice (workspace tier
dependent).

## Weekly report

The weekly endpoint **rejects ranges that aren't exactly 7 days**:

```
HTTP 200 body:
{"code":501,"message":"Please select date range of exactly 7 days for weekly report"}
```

Our probe used a ±1-day range to capture the freshly-logged entry,
which is 3 days. Re-run with `dateRangeStart = Monday 00:00:00.000`
and `dateRangeEnd = Sunday 23:59:59.999` of the target week, or
go-clockify can clamp/normalise before calling.

The response shape (per OpenAPI + the summary shape we observed)
is the same monetary scale: cents everywhere. `groupOne[].amount`,
`groupOne[].totals[].amounts[].value`, and `totalsByDay[].amount`
are all cents.

## Attendance report

`POST {reports}/reports/attendance` returns
`{"entities":[AttendanceDto…]}`. **No monetary fields** — confirmed
in our run, the response carries only break / capacity / time
totals (no hourly rate × duration). The MCP should not surface money
on this tool.

## Project-level rate: there is no PUT /projects/{id}/hourly-rate

The probe initially tried `PUT /workspaces/{ws}/projects/{id}/hourly-rate`
and got a 200 / null body — but the project's `hourlyRate.amount`
stayed at `0`. Re-reading `PROJECTSDOC.md` confirms **that route
does not exist**. The only documented per-project rate routes are:

  - `PUT /workspaces/{ws}/projects/{id}/users/{userId}/hourly-rate`
    (PROJECTSDOC.md:3934) — per-member project override.
  - `PUT /workspaces/{ws}/projects/{id}/users/{userId}/cost-rate`
    (PROJECTSDOC.md:3621) — per-member cost override.

The **project-default** rate is set only via the project itself:

  - `POST /workspaces/{ws}/projects` — body includes `hourlyRate`
    `{amount, since}` and `costRate {amount, since}`
    (PROJECTSDOC.md:405–410, 493–520).
  - `PUT  /workspaces/{ws}/projects/{id}` — same body shape.

Clockify silently 200s undocumented PUTs that match a route prefix,
which is what tripped the probe. The task-level routes
(`/projects/{id}/tasks/{tid}/hourly-rate`, …/cost-rate) **do**
exist and persisted as expected (TASKDOC.md:540, 714).

### Time-effective rates (`since`)

Every Rate request shape — workspace, project, task, project-member —
carries an optional `since` field:

```
since   string   ISO-8601 yyyy-MM-ddThh:mm:ssZ
                 Represents a date and time. Default: "##default".
```

(PROJECTSDOC.md:384, 412, 502, 524, 787, 792, 2039, 2043.) Rates only
apply to entries logged after `since`. A rate set without `since`
defaults to "##default" — which in practice means **the rate is
prospective and does not retroactively change earnings on historical
entries**. This is critical for the MCP:

- When setting a rate, callers should pass `since` if they want a
  specific effective date, or accept the implicit-prospective
  default if not.
- When reading an entry's `earnedAmount`, that value reflects the
  rate that was effective **at the entry's start time**, not the
  current project/task rate — so a fresh rate change does *not*
  recompute historical totals via the reports endpoints either.

**Implication for go-clockify:** when emitting an effective-rate
hierarchy, the MCP should:

1. Report which scope actually supplied the rate (`rate_scope`).
2. Use the rate fields that the **reports.api** returns alongside
   each entry (`amount`, `rate`, `earnedAmount`, `earnedRate`,
   `costAmount`, `costRate` — see the fixture above) as the
   authoritative per-entry earning, because that's what Clockify
   already resolved against the time-effective `since` curve.
3. Only fall back to "derive from project hourly × duration" when
   the entry was not visible in a reports response (e.g. running
   entry, single-entry path, ad-hoc compute).
4. Surface the project-default rate setter via project
   create/update body (`hourlyRate`, `costRate`, optional `since`),
   **not** an imagined `/projects/{id}/hourly-rate` endpoint.

## Membership rates

`Project.memberships[].hourlyRate` and `…costRate` come back as
`null` until explicitly set on the membership row. The workspace
list (`/workspaces`) shows that user-level overrides on the
**workspace membership** itself exist (`workspaces[].memberships[]`),
some carrying `{amount:123400}` (€1234.00) and others null. That is
the most-specific lookup behind a project membership.

## "##default" sentinel

Documented in `TASKDOC.md:567` / `PROJECTSDOC.md:813`. Not seen in
this run (the workspace was already EUR with concrete amounts). The
shape is the literal string `"##default"` in place of a Rate object,
meaning "fall through to parent." `Rate.UnmarshalJSON` must accept
both shapes.

## Go-clockify checklist (consumed by next commits)

1. `Rate{Amount int64 cents; Currency string}` — accept object,
   `"##default"`, or `null`. `MoneyView.AmountCents = r.Amount`,
   `MoneyView.AmountDecimal = fmt(r.Amount/100, 2dp)`.
2. `MoneyFromReportNumber(v float64, currency string)` — multiply
   by `1` (no rescale); round to int64 cents with banker's rounding;
   record both raw float and integer cents in MoneyView for audit.
3. Detailed report path: take `timeentries[i].{amount, rate,
   earnedAmount, earnedRate, costAmount, costRate, currency}` and
   `totals[i].{totalAmount, amounts[]}` verbatim into MoneyViews.
4. Summary report path: walk `groupOne[]` recursively, mapping
   `amount` and `amounts[].value` into MoneyViews per node.
5. Weekly tool: enforce 7-day window or clamp before calling, then
   re-use summary's MoneyView mapping.
6. Attendance tool: no money; just pass `entities[]` shape through.
