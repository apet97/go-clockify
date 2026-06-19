# Finding scaffold: tasks write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace on
2026-06-19. `TODO-live-2xx` rows are scaffolds only and are ignored by the
generator until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks | TODO-live-2xx | fixtures/live-shape/tasks-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks | 201 | fixtures/live-shape/tasks-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | 200 | fixtures/live-shape/tasks-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | 200 | fixtures/live-shape/tasks-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId} | TODO-live-2xx | fixtures/live-shape/tasks-delete.txt |
