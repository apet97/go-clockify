# Finding: approval-requests (read-side live promotion)

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox by the read-side
schema oracle. Clean canonical path (no query string, canonical
`{workspaceId}` placeholder) so the generator's `normalize_path` matches
the merged operation key and `status_bucket` flips the op to
`live-success`. Host is the default `api.clockify.me`. Fixtures are
documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/approval-requests | 200 | fixtures/live-shape/approval-requests-list.json |

## Submit-for-user write-side promotion (2026-08-04/05, clockify-ts-sdk Slice 1)

`POST .../approval-requests/users/{userId}` with `{period:"WEEKLY",
periodStart}` against a real user returned 201 with a PENDING request; the
request was withdrawn afterward via `PATCH .../approval-requests/{id}`
(`updateApprovalRequest`, already `live-success`) with
`{state:"WITHDRAWN_SUBMISSION"}`. Leftovers: 0 (no PENDING approval request
remains for the probed period).

Separately, `resubmitEntriesForApproval` and
`resubmitEntriesForApprovalForUser` are confirmed real routes (structured
501 business responses: "Cannot resubmit entries for user ..., workspace
..., period ..." — not a 404/405) but the probe could not reproduce their
actual precondition. Withdrawing the submitted request and retrying still
501'd, and the state machine rejects a direct WITHDRAWN_SUBMISSION ->
APPROVED transition ("Can't update status from WITHDRAWN_SUBMISSION to
APPROVED"), so "resubmit" likely requires an APPROVED request whose
underlying time entries were edited afterward — a scenario this probe did
not construct. Left at `probe-documented`/`documented`; a future session
with a realistic edited-after-approval fixture can complete the promotion.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/approval-requests/users/{userId} | 201 | live-probe 2026-08-04/05, id 6a72818f59e113bde6d50a70, withdrawn at teardown |
