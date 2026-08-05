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
| PATCH | api.clockify.me | /api/v1/workspaces/{workspaceId}/projects/{projectId}/template | 200 | live probe 2026-06-22 (Leftovers:0) |
| PATCH | api.clockify.me | /api/v1/workspaces/{workspaceId}/projects/{projectId}/estimate | 200 | live probe 2026-06-22 (Leftovers:0) |

## From-template create promotion (2026-08-04/05, clockify-ts-sdk Slice 1)

Marked a real project as a template via the `updateProjectTemplate` PATCH
above (`{isTemplate:true}`), created a new project from it via
`POST .../projects/from-template` with `{name, templateProjectId}`, then
archived (`PUT {archived:true}`) and deleted the new project, and reverted
the original project's `isTemplate` flag back to `false`. Leftovers: 0.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/projects/from-template | 200 | live-probe 2026-08-04/05, template 6a6b3ebb5e5bb14ab2c7506e, new project 6a7282b5c11f13fa07554a5b archived+deleted at teardown |
