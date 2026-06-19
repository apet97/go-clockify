# Finding scaffold: clients write-shape campaign

Rows with concrete status codes were captured by
`TestLiveRawClockifyWriteCRUDShapeOracle` against the sacrificial workspace on
2026-06-19. `TODO-live-2xx` rows are scaffolds only and are ignored by the
generator until a future live run captures that operation.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/clients | TODO-live-2xx | fixtures/live-shape/clients-list.json |
| POST | api.clockify.me | /workspaces/{workspaceId}/clients | 201 | fixtures/live-shape/clients-create.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | 200 | fixtures/live-shape/clients-get.json |
| PUT | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | 200 | fixtures/live-shape/clients-update.json |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/clients/{clientId} | TODO-live-2xx | fixtures/live-shape/clients-delete.txt |
