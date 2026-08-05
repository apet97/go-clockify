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
| POST | api.clockify.me | /api/v1/workspaces/{workspaceId}/users/info | 200 | live probe 2026-06-22 (Leftovers:0) |

## Membership-status promotion (2026-08-04/05, clockify-ts-sdk Slice 1)

`PUT .../users/{userId}` with `{status:"ACTIVE"|"INACTIVE"}` — this is
workspace-**membership** status, not the global user account status (a
separate, unrelated field). Used an already-INACTIVE test-owned member,
flipped it to ACTIVE (confirmed via a follow-up workspace GET, since the
PUT's own response body is a workspace snapshot that can lag one read
behind the write), then flipped it back to INACTIVE and reconfirmed.
Leftovers: 0 (member restored to its original membership status).

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| PUT | api.clockify.me | /workspaces/{workspaceId}/users/{userId} | 200 | live-probe 2026-08-04/05, member 67a60c1ee0109319d0b9dfc5, ACTIVE then restored to INACTIVE |
