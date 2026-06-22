# Finding scaffold: tasks write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` (create/update/get) and
`TestLiveRawClockifyReadSideSchemaDiff` (the list GET) against the
sacrificial workspace; both decoded the live 2xx body into the
typed `internal/clockify` model with no unknown fields. The DELETE 200 was
captured on 2026-06-20 by the same test's mark-done-first teardown DELETE (the
cleanup helper now records the real teardown HTTP status instead of discarding
it). `TODO-live-2xx` rows are scaffolds only and are ignored by the generator
until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks | 200 | fixtures/live-shape/tasks-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks | 201 | fixtures/live-shape/tasks-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | 200 | fixtures/live-shape/tasks-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | 200 | fixtures/live-shape/tasks-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | 200 | fixtures/live-shape/tasks-delete.txt |
| PUT | api.clockify.me | /api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/cost-rate | 200 | live probe 2026-06-22 (Leftovers:0) |
| PUT | api.clockify.me | /api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/hourly-rate | 200 | live probe 2026-06-22 (Leftovers:0) |
