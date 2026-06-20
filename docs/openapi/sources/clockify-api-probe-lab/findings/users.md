# Finding: users (read-side live promotions)

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox by the read-side
schema oracle. Clean canonical paths (no query string, canonical
`{workspaceId}`/`{userId}` placeholders) so the generator's
`normalize_path` matches the merged operation key and `status_bucket`
flips each op to `live-success`. Host is the default `api.clockify.me`.
Fixtures are documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /user | 200 | fixtures/live-shape/current-user.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/users | 200 | fixtures/live-shape/workspace-users.json |
| GET | api.clockify.me | /workspaces/{workspaceId}/users/{userId}/managers | 200 | fixtures/live-shape/user-managers.json |
