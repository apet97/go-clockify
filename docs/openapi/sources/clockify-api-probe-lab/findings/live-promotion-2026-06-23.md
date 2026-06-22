# Finding: live write+read promotions (2026-06-23)

Captured live this session against the sacrificial sandbox
(`65b382b606de527a7ee2b60e`) with an API key, via the SDK probe harness.
Each row is a clean canonical path (canonical `{workspaceId}` + entity-id
placeholders) so the generator's `normalize_path` matches the merged
operation key and `status_bucket` flips the op to `live-success`. Host is
the default `api.clockify.me`. Group/role/project-template writes were
cleaned up at teardown (Leftovers:0); rate/balance/status writes were
applied to the sacrificial workspace. Fixtures are documentary.

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /workspaces/{workspaceId}/webhooks/{webhookId}/logs | 200 | live-probe 2026-06-23 (documentary) |
| GET | api.clockify.me | /workspaces/{workspaceId}/addons/{addonId}/webhooks | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/hourly-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/cost-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/users/{userId}/hourly-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/users/{userId}/cost-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate | 200 | live-probe 2026-06-23 (documentary) |
| PUT | api.clockify.me | /workspaces/{workspaceId}/invoices/settings | 200 | live-probe 2026-06-23 (documentary) |
| POST | api.clockify.me | /workspaces/{workspaceId}/users/{userId}/roles | 201 | live-probe 2026-06-23 (documentary) |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/users/{userId}/roles | 204 | live-probe 2026-06-23 (documentary) |
| DELETE | api.clockify.me | /workspaces/{workspaceId}/user-groups/{groupId}/users/{userId} | 200 | live-probe 2026-06-23 (documentary) |
| GET | api.clockify.me | /workspaces/{workspaceId}/entities/created | 200 | live-probe 2026-06-23 (documentary) |
| GET | api.clockify.me | /workspaces/{workspaceId}/entities/deleted | 200 | live-probe 2026-06-23 (documentary) |
| GET | api.clockify.me | /workspaces/{workspaceId}/entities/updated | 200 | live-probe 2026-06-23 (documentary) |
| PATCH | api.clockify.me | /workspaces/{workspaceId}/member-profile/{userId} | 200 | live-probe 2026-06-23 (documentary) |
| PATCH | api.clockify.me | /workspaces/{workspaceId}/time-off/balance/policy/{policyId} | 204 | live-probe 2026-06-23 (documentary) |
| PATCH | api.clockify.me | /workspaces/{workspaceId}/invoices/{invoiceId}/status | 200 | live-probe 2026-06-23 (documentary) |
