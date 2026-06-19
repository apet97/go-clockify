# Finding scaffold: projects write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace on
2026-06-19. `TODO-live-2xx` rows are scaffolds only and are ignored by the
generator until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/projects | TODO-live-2xx | fixtures/live-shape/projects-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/projects | 201 | fixtures/live-shape/projects-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | 200 | fixtures/live-shape/projects-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | 200 | fixtures/live-shape/projects-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | TODO-live-2xx | fixtures/live-shape/projects-delete.txt |
