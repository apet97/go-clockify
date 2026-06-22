# Finding scaffold: time entries write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` (create/update/get) and
`TestLiveRawClockifyReadSideSchemaDiff` (the per-user list GET) against the
sacrificial workspace; both decoded the live 2xx body into the
typed `internal/clockify` model with no unknown fields. The DELETE 204 was
captured on 2026-06-20 by the same test's teardown DELETE (the cleanup helper
now records the real teardown HTTP status instead of discarding it).
`TODO-live-2xx` rows are scaffolds only and are ignored by the generator until a
future live run captures that operation. If the workspace has required
time-entry custom fields, capture the raw `customFields` payload first and
remove the test skip.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/user/{userId}/time-entries | 200 | fixtures/live-shape/time-entries-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/time-entries | 201 | fixtures/live-shape/time-entries-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/time-entries/{timeEntryId} | 200 | fixtures/live-shape/time-entries-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/time-entries/{timeEntryId} | 200 | fixtures/live-shape/time-entries-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/time-entries/{timeEntryId} | 204 | fixtures/live-shape/time-entries-delete.txt |
| POST | api.clockify.me | /api/v1/workspaces/{workspaceId}/user/{userId}/time-entries | 201 | live probe 2026-06-22 (Leftovers:0) |
| PUT | api.clockify.me | /api/v1/workspaces/{workspaceId}/user/{userId}/time-entries | 200 | live probe 2026-06-22 (Leftovers:0) |
| PATCH | api.clockify.me | /api/v1/workspaces/{workspaceId}/user/{userId}/time-entries | 200 | live probe 2026-06-22 (Leftovers:0) |
| POST | api.clockify.me | /api/v1/workspaces/{workspaceId}/user/{userId}/time-entries/{timeEntryId}/duplicate | 201 | live probe 2026-06-22 (Leftovers:0) |
| PATCH | api.clockify.me | /api/v1/workspaces/{workspaceId}/time-entries/invoiced | 200 | live probe 2026-06-22 (Leftovers:0) |

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox; clean canonical
path so the generator's `normalize_path` matches the merged operation key
and `status_bucket` flips the op to `live-success`. Fixtures are
documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/time-entries/status/in-progress | 200 | fixtures/live-shape/time-entries-in-progress.json |
