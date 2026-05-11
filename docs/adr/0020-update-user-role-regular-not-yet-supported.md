# 0020 - UpdateUserRole REGULAR path: not yet supported

## Status

Accepted — 2026-05-11

## Context

`clockify_update_user_role` exposes four role values:
`WORKSPACE_ADMIN`, `PROJECT_MANAGER`, `TEAM_MANAGER`, and `REGULAR`.

The first three are **grant** operations: a `PUT` to
`/v1/workspaces/{ws}/users/{userId}/roles` with `{"role": "<ROLE>"}` in
the body adds the named role to the user's grant set while keeping them
an active workspace member.

Setting a user back to **REGULAR** is semantically different: it
**strips** all elevated role grants (WORKSPACE_ADMIN / PROJECT_MANAGER /
TEAM_MANAGER) from the user, leaving them as a plain, active workspace
member who can still log time. It does **not** remove the user from the
workspace — that is the separate `clockify_deactivate_user` operation
(`PUT /workspaces/{ws}/users/{userId}` with `{"status":"INACTIVE"}`).

The live Clockify API performs a role-strip via:

```
DELETE /v1/workspaces/{ws}/users/{userId}/roles
Content-Type: application/json

{"entityId": "<workspaceId>", "role": "<ROLE_TO_REMOVE>"}
```

One `DELETE` call is required per elevated grant the user currently
holds.  The internal `clockify.Client` type exposes a `Delete(ctx,
path, &out)` method that does not carry a request body, so the wire
contract for role-strip cannot be satisfied without extending the
client.

## Decision

Do **not** implement the REGULAR path yet.  The `UpdateUserRole`
function returns an explicit, accurate `not-yet-supported` error when
`role == "REGULAR"`.  The error message:

1. Correctly describes what REGULAR means (strips elevated grants,
   user stays in workspace).
2. States that `clockify_deactivate_user` is **NOT** a substitute
   (deactivate removes the user entirely).
3. Explains the technical blocker (client `Delete` lacks body support).
4. Points callers to the Clockify web UI or a direct API call as the
   workaround.

The `REGULAR` value is retained in the `validRoles` set so that the
input-validation message stays accurate, and the REGULAR guard fires
**after** validation, before any workspace-ID resolution or HTTP call.

## Consequences

- Callers asking to strip elevated roles receive a clear, actionable
  error instead of a wrong or misleading one.
- No silent misbehaviour: the old code would have sent
  `PUT /roles` with `{"role":"REGULAR"}`, which the Clockify API
  either rejects or interprets incorrectly.
- A future implementation must:
  1. Add `Client.DeleteWithBody(ctx, path string, body any, out any) error`
     to `internal/clockify/client.go` (or a purpose-built
     `DeleteWithBody` variant).
  2. Fetch the user's current role grants and issue one DELETE per
     elevated role.
  3. Pin the wire contract with a unit test (method=DELETE, correct
     body shape, one call per elevated grant).
  4. Update this ADR status to Superseded and reference the
     implementing commit.

## Alternatives considered

- **Implement now (Option B):** Requires `Client.DeleteWithBody`, live
  API verification, and a meaningful contract test.  Deferred to avoid
  P1 contract risk without confirmation beyond the agent-29 probe
  transcript.
- **Remove REGULAR from the enum:** Would break callers who pass it in
  and expect a meaningful error.  Keeping it with a guard is strictly
  more informative.
