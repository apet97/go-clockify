# Finding scaffold: clients write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` (create/update/get) and
`TestLiveRawClockifyReadSideSchemaDiff` (the list GET) against the
sacrificial workspace; both decoded the live 2xx body into the
typed `internal/clockify` model with no unknown fields. The DELETE 200 was
captured on 2026-06-20 by the same test's archive-first teardown DELETE (the
cleanup helper now records the real teardown HTTP status instead of discarding
it). `TODO-live-2xx` rows are scaffolds only and are ignored by the generator
until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/clients | 200 | fixtures/live-shape/clients-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/clients | 201 | fixtures/live-shape/clients-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | 200 | fixtures/live-shape/clients-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | 200 | fixtures/live-shape/clients-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | 200 | fixtures/live-shape/clients-delete.txt |
