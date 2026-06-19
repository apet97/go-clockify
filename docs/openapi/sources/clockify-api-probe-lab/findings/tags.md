# Finding scaffold: tags write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace on
2026-06-19. `TODO-live-2xx` rows are scaffolds only and are ignored by the
generator until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/tags | TODO-live-2xx | fixtures/live-shape/tags-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/tags | 201 | fixtures/live-shape/tags-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | 200 | fixtures/live-shape/tags-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | 200 | fixtures/live-shape/tags-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | TODO-live-2xx | fixtures/live-shape/tags-delete.txt |
