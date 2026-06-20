# Finding: member-profile (read-side live promotion)

## Live read-side promotions (2026-06-20)

Captured HTTP 200 live this session against the sandbox by the read-side
schema oracle. Clean canonical path (no query string, canonical
`{workspaceId}`/`{userId}` placeholders) so the generator's
`normalize_path` matches the merged operation key and `status_bucket`
flips the op to `live-success`. Host is the default `api.clockify.me`.
Fixtures are documentary + gitignored.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| GET | api.clockify.me | /workspaces/{workspaceId}/member-profile/{userId} | 200 | fixtures/live-shape/member-profile.json |
