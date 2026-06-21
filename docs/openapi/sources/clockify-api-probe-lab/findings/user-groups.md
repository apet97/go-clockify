# Finding: user-groups (read-side live promotion)

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox by the read-side
schema oracle. Clean canonical path (no query string, canonical
`{workspaceId}` placeholder) so the generator's `normalize_path` matches
the merged operation key and `status_bucket` flips the op to
`live-success`. Host is the default `api.clockify.me`. Fixtures are
documentary + gitignored.

Note: only the workspace-level list flips here. The single-group routes
`GET /user-groups/{groupId}` and `GET /user-groups/{groupId}/users` both
returned 405 this session and stay probe-documented/unsupported (phantom
read routes — Clockify does not serve a single-group GET).

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/user-groups | 200 | fixtures/live-shape/user-groups-list.json |

## Live write-side promotion (2026-06-21)

Captured live this session against the sandbox: a clean create -> update ->
delete cycle on a `sdk-live-probe-ug`-prefixed group (POST 201, PUT 200,
DELETE 200), deleted at teardown — **Leftovers:0** (the workspace
`user-groups` list shows zero `sdk-live-probe-ug` residue afterward). Clean
canonical paths so `normalize_path` binds the operation keys and
`status_bucket` flips each op to `live-success`. Fixtures are documentary
(no body captured; the create/update bodies are `{name}`).

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/user-groups | 201 | live-probe 2026-06-21 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/user-groups/{groupId} | 200 | live-probe 2026-06-21 (documentary) |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/user-groups/{groupId} | 200 | live-probe 2026-06-21 (documentary) |
