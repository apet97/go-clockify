# Finding scaffold: projects write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace.
The DELETE 200 was captured on 2026-06-20 by the same test's archive-first
teardown DELETE (the cleanup helper now records the real teardown HTTP status
instead of discarding it). `TODO-live-2xx` rows are scaffolds only and are
ignored by the generator until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/projects | 200 | fixtures/live-shape/projects-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/projects | 201 | fixtures/live-shape/projects-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | 200 | fixtures/live-shape/projects-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | 200 | fixtures/live-shape/projects-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId} | 200 | fixtures/live-shape/projects-delete.txt |
