# Finding scaffold: tags write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` (create/update/get) and
`TestLiveRawClockifyReadSideSchemaDiff` (the list GET) against the
sacrificial workspace; both decoded the live 2xx body into the
typed `internal/clockify` model with no unknown fields. The DELETE 200 was
captured on 2026-06-20 by the same test's teardown DELETE (the cleanup helper
now records the real teardown HTTP status instead of discarding it).
`TODO-live-2xx` rows are scaffolds only and are ignored by the generator until a
future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/tags | 200 | fixtures/live-shape/tags-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/tags | 201 | fixtures/live-shape/tags-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | 200 | fixtures/live-shape/tags-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | 200 | fixtures/live-shape/tags-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/tags/{tagId} | 200 | fixtures/live-shape/tags-delete.txt |
