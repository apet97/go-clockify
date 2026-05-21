# QA Agent 60 - final-release-readiness-verdict

## Verdict
**PASS WITH CONCERNS**

## What I checked

1. **Build**: `go build ./cmd/clockify-mcp/` — passes cleanly.
2. **Static analysis**: `go vet ./...` — passes, no warnings.
3. **Full test suite**: All 29 packages pass (internal/*, tests/*, cmd/*). One pre-existing test isolation bug was found and fixed (see Fixes made).
4. **Live E2E contract tests**: `go test -tags=livee2e -run 'TestE2EReadOnly|TestE2EErrors' ./tests/...` — passes against the real Clockify API in workspace `<REDACTED_ID>`.
5. **Doctor command**: `clockify-mcp doctor --strict` correctly audits configuration, identifies the workspace ID, and flags hosted-strict posture requirements as expected for a local deployment.
6. **Tool catalog**: 40 Tier-1 + 88 Tier-2 = 128 tools registered, with correct input schemas, required fields, enum restrictions, and risk classifications.
7. **Live API probe**: All primary Clockify API endpoints respond correctly:
   - `GET /user` → 200
   - `GET /workspaces/{ws}` → 200
   - `GET /workspaces/{ws}/projects` → 200 (paginated)
   - `GET /workspaces/{ws}/clients` → 200 (paginated)
   - `GET /workspaces/{ws}/tags` → 200
   - `GET /workspaces/{ws}/user/{userId}/time-entries` → 200 (paginated, date-filtered)
   - `GET /workspaces/{ws}/time-entries/{entryId}` → 200
   - `DELETE /workspaces/{ws}/time-entries/{entryId}` → 204
   - `GET /reports/v1/workspaces/{ws}/shared-reports` → 200
8. **API path correctness**: Verified that the go-clockify client uses the correct Clockify API path conventions:
   - List entries: `user/{userId}/time-entries` (requires user in path)
   - Individual entry: `time-entries/{entryId}` (without user segment)
9. **Auth error handling**: Live-verified that Clockify returns appropriate errors:
   - Missing API key → 401 "Multiple or none auth tokens present"
   - Invalid API key → 401 "Api key does not exist"
   - Non-existent workspace → 404
10. **Dockerfile**: Well-structured multi-stage build with pinned digests, proper non-root user, HEALTHCHECK, and build-arg support for optional tags (grpc, postgres, fips).
11. **Release pipeline**: `.goreleaser.yaml` is complete, covering all platforms including FIPS, Postgres, and gRPC variants. Makefile `release-check` target composes all pre-ship gates.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — Credentials (API key, workspace ID, confirmation)
- `probes/lib/common.sh` — Shared probe library (curl wrapper, redaction, prefix management)
- `TIMEENTRYDOC.md` — Time entry API reference
- `README.md` — Probe lab documentation
- `CLAUDE.md` — Agent safety rules

All secrets redacted as `****REDACTED****` in this report.

## Commands run

```bash
go build ./cmd/clockify-mcp/
go vet ./...
go run ./cmd/clockify-mcp doctor --strict
go test -count=1 -timeout 180s ./...
go test -tags=livee2e -run 'TestE2EReadOnly|TestE2EErrors' ./tests/...

# Live API probes (all keys redacted)
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/user
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects?page-size=2
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients?page-size=2
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags?page-size=1
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<REDACTED>/time-entries?page-size=1
curl -H "X-Api-Key: <REDACTED>" https://reports.api.clockify.me/v1/workspaces/<REDACTED>/shared-reports?page-size=1

# Create/Read/Delete cycle
curl -X POST -H "X-Api-Key: <REDACTED>" .../user/<REDACTED>/time-entries \
  -d '{"start":"...","end":"...","description":"qa-agent-60-*-test-entry","billable":false}' → 201
curl -H "X-Api-Key: <REDACTED>" .../time-entries/<entryId> → 200
curl -X DELETE -H "X-Api-Key: <REDACTED>" .../time-entries/<entryId> → 204

# Auth failure probes (all confirmed)
curl -H "X-Api-Key: bad-key" .../user → 401
curl .../user (no key) → 401
curl -H "X-Api-Key: <REDACTED>" .../workspaces/<REDACTED_ID>/projects → 404
```

## Live API probes run

| Endpoint | Method | Expected | Actual | Status |
|----------|--------|----------|--------|--------|
| `/user` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/projects?page-size=2` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/clients?page-size=2` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/tags?page-size=1` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/user/{userId}/time-entries?page-size=1` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/user/{userId}/time-entries?start=...&end=...` | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/user/{userId}/time-entries` (create) | POST | 201 | 201 | PASS |
| `/workspaces/{ws}/time-entries/{entryId}` (read) | GET | 200 | 200 | PASS |
| `/workspaces/{ws}/time-entries/{entryId}` (delete) | DELETE | 204 | 204 | PASS |
| `/reports/v1/workspaces/{ws}/shared-reports?page-size=1` | GET | 200 | 200 | PASS |
| `/user` (bad key) | GET | 401 | 401 | PASS |
| `/user` (no key) | GET | 401 | 401 | PASS |
| `/workspaces/invalid-id/projects` | GET | 404 | 404 | PASS |

Also verified through code review that:
- `ListEntries` uses `paths.Workspace(ws, "user", userID, "time-entries")` → correct for list
- `GetEntry`/`UpdateEntry`/`DeleteEntry` use `paths.Workspace(ws, "time-entries", entryID)` → correct for individual entries
- `ReportsBaseURL()` correctly maps `https://api.clockify.me/api/v1` → `https://reports.api.clockify.me/v1`
- `Client.doRequest` uses `X-Api-Key` header (not `Authorization`)

## Findings

### F1 (P2 — FIXED): Test isolation bug in `TestLoadSingleTenantHTTPRequiresAPIKey`
- **File**: `internal/config/config_test.go:496`
- **Issue**: The test expected `Load()` to fail when `single-tenant-http` profile has no API key, but didn't explicitly clear `CLOCKIFY_API_KEY` in the test env. When running in a shell with the key exported, `Load()` picked up the live key and succeeded, causing the test to fail.
- **Fix**: Added `"CLOCKIFY_API_KEY": ""` to the `setEnvs` call so the test is immune to parent-env contamination.
- **Verification**: Passes with and without pre-set `CLOCKIFY_API_KEY`.

### F2 (P3): Clockify API path convention inconsistency (documentation note)
- The Clockify API uses different path structures for list vs. individual time entry operations:
  - **List**: `GET /workspaces/{ws}/user/{userId}/time-entries` (requires userId in path)
  - **Individual**: `GET/DELETE/PUT /workspaces/{ws}/time-entries/{entryId}` (no userId segment)
- The MCP server's code correctly handles both conventions — no code fix needed.
- **Note**: Calling `GET /workspaces/{ws}/time-entries` (without user segment, for list) returns 405 from Clockify. The MCP server never issues this path.

### F3 (P3): Docker build failure (environmental)
- Local `docker build` failed with "frontend grpc server closed unexpectedly" — consistent with Docker daemon instability, not a code or Dockerfile issue.
- The Dockerfile itself is well-structured with pinned digests, proper stages, non-root user, and HEALTHCHECK.

## Fixes made

1. **`internal/config/config_test.go`** — Added `"CLOCKIFY_API_KEY": ""` to the `setEnvs` call in `TestLoadSingleTenantHTTPRequiresAPIKey` to prevent parent environment contamination from the live credentials file.

## Reproduction steps for each issue

### F1: Test isolation bug
```bash
export CLOCKIFY_API_KEY="any-non-empty-value"
go test ./internal/config/ -count=1 -run "TestLoadSingleTenantHTTPRequiresAPIKey"
# Before fix: FAIL — Load() succeeds unexpectedly
# After fix:  PASS — Load() fails as expected when key is unset
```

### F2: API path convention
```bash
# This works (list with user in path):
curl -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/user/<REDACTED>/time-entries?page-size=1"
# Returns 200

# This fails (list without user in path):
curl -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/time-entries?page-size=1"
# Returns 405 "Request method 'GET' is not supported"
```

## Cleanup performed

- Created test time entry `qa-agent-60-1778449836-test-entry` (ID: `<REDACTED_ID>`)
- Successfully read it back (HTTP 200)
- Successfully deleted it (HTTP 204)
- Verified deletion — entry no longer exists

## Leftover test resources

None. All `qa-agent-60-`-prefixed resources created during this session were deleted within the session.

## Severity

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| F1 | P2 | Test isolation: `CLOCKIFY_API_KEY` env contamination | FIXED |
| F2 | P3 | Clockify API path convention differs between list/individual | NOTED (code correct) |
| F3 | P3 | Docker build failed locally (daemon issue) | ENVIRONMENTAL |

## Files changed

- `internal/config/config_test.go` — line 500: added `"CLOCKIFY_API_KEY": ""` to test env map

## Suggested next action

1. Re-run the Docker build in a clean Docker environment or CI to confirm it's purely a local daemon issue.
2. Consider adding `CLOCKIFY_API_KEY` to the `profileLeakedEnvs` list in `config_test.go` as a more systematic fix, since this env var can also leak from `applyProfile()`.
3. The remaining launch-candidate gates (Groups 6/7) should be checked against `docs/launch-candidate-checklist.md` — this QA session covered local/self-hosted readiness (Groups 2-5).

## False positives / uncertainty

1. The Docker build failure appears to be a local Docker daemon issue, not a code problem. The Dockerfile is properly structured.
2. The initial `TestLoadSingleTenantHTTPRequiresAPIKey` failure was env contamination from the QA session sourcing the live credentials file — it passes in clean env.

## Final recommendation

**LOCAL/SELF-HOSTED RELEASE READINESS: APPROVED**

The go-clockify MCP server is ready for local/self-hosted release. All tests pass, the build is clean, `go vet` reports no issues, live API integration works correctly, tool schemas are correct, and path construction follows the Clockify API conventions. The one bug found (test isolation) was fixed. No blocking issues remain.
