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
