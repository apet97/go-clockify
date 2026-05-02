//go:build postgres

// Cross-instance streamable-HTTP session rehydration E2E for the
// launch-candidate gate (Group 3 / ADR 0017). Boots TWO
// mcp.ServeStreamableHTTP listeners in-process against the same
// Postgres-backed control-plane store, exercises the
// "initialize on instance A, tools/call on instance B" flow that
// the band-aid (sessionAffinity: ClientIP) cannot cover when a pod
// restarts, evicts, rolling-upgrades, or fails over across AZs, and
// asserts that the rehydration path:
//
//   1. Returns the same successful tools/call response as the
//      local-hit path (no client-visible re-initialize required).
//   2. Writes audit rows from instance B that carry the same
//      session_id created on instance A — the audit pipeline does
//      not split-brain across the rehydration boundary.
//   3. Rejects a forged session_id whose persisted Subject/TenantID
//      do not match the request's freshly-authenticated principal
//      (strict re-authentication preserved per ADR 0017 Q2).
//   4. Surfaces an expired persisted session as
//      "session expired" with the row removed from the store
//      (preserves the eviction contract per ADR 0017 Q4 =
//      preserve stored ExpiresAt; expired-on-rehydration falls
//      through to the same destroy() path as a local-hit expiry).
//
// Test scaffolding (newFakeClockify, sharedSvcFactory,
// sharedSvcAuditor, sharedSvcClient, jsonrpcEnvelope, readBody,
// mustExec, waitForReady) is reused from e2e_shared_service_test.go;
// both files live in package postgres_test.
//
// First-commit posture: written failing-first per CLAUDE.md
// strict rule 3. Step 1 ("tools/call on instance B succeeds") is
// expected to be RED on current main because
// streamSessionManager.get is local-only and returns
// "session not found" on a miss — the implementation that closes
// the gap lands in the next commit. Drift check (mandatory):
// after this test goes GREEN, flip step 1's success assertion
// negative, confirm RED with "session not found" / 404, restore.
//
// Activation: build tag `postgres`. With the `integration` tag, the
// test reuses the package Testcontainers DSN when
// MCP_LIVE_CONTROL_PLANE_DSN is unset, which keeps `make
// test-postgres` self-contained. Without `integration`, the env var
// must point at a sacrificial Postgres database.

package postgres_test

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
)

const (
	rehydTenant      = "tenant-rehydration-svc-A"
	rehydTenantOther = "tenant-rehydration-svc-B"
	rehydCred        = "cred-rehydration-svc-A"
	rehydCredOther   = "cred-rehydration-svc-B"
	rehydWS          = "ws-rehydration-svc-A"
	rehydWSOther     = "ws-rehydration-svc-B"
	rehydSubject     = "alice@rehydration-svc.test"
	rehydSubjectOth  = "bot@rehydration-svc.test"
	rehydPrefix      = "rehydration-svc-"
)

func TestStreamableHTTPCrossInstanceRehydration(t *testing.T) {
	dsn := e2eControlPlaneDSN(t, "MCP_LIVE_CONTROL_PLANE_DSN not set; skipping cross-instance rehydration E2E")

	store, err := controlplane.Open(dsn)
	if err != nil {
		t.Fatalf("open postgres control-plane store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer verifyCancel()
	verifyPool, err := pgxpool.New(verifyCtx, dsn)
	if err != nil {
		t.Fatalf("verifier pool: %v", err)
	}
	t.Cleanup(verifyPool.Close)

	// Pre-emptive cleanup so a previous interrupted run does not skew
	// the row counts. Tagged-prefix keeps blast radius scoped; cleanup
	// also runs in t.Cleanup so an in-flight failure does not leave
	// pollution behind.
	cleanup := func(ctx context.Context) {
		for _, sql := range []string{
			`DELETE FROM audit_events  WHERE tenant_id LIKE 'tenant-rehydration-svc-%'`,
			`DELETE FROM sessions      WHERE id LIKE 'sess-rehydration-svc-%' OR tenant_id LIKE 'tenant-rehydration-svc-%'`,
			`DELETE FROM tenants       WHERE id LIKE 'tenant-rehydration-svc-%'`,
			`DELETE FROM credential_refs WHERE id LIKE 'cred-rehydration-svc-%'`,
		} {
			if _, err := verifyPool.Exec(ctx, sql); err != nil {
				t.Logf("cleanup %q: %v", sql, err)
			}
		}
	}
	cleanup(verifyCtx)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanup(ctx)
	})

	fake := newFakeClockify(t)
	clockifyBaseURL := fake.URL + "/v1"
	if !strings.HasPrefix(clockifyBaseURL, "http://127.0.0.1:") &&
		!strings.HasPrefix(clockifyBaseURL, "http://[::1]:") {
		t.Fatalf("fake clockify must bind to loopback, got %q", clockifyBaseURL)
	}

	for _, ref := range []controlplane.CredentialRef{
		{ID: rehydCred, Backend: "inline", Reference: "rehydration-key", Workspace: rehydWS, BaseURL: clockifyBaseURL},
		{ID: rehydCredOther, Backend: "inline", Reference: "rehydration-key", Workspace: rehydWSOther, BaseURL: clockifyBaseURL},
	} {
		if err := store.PutCredentialRef(ref); err != nil {
			t.Fatalf("seed credential ref %s: %v", ref.ID, err)
		}
	}
	for _, tenant := range []controlplane.TenantRecord{
		{
			ID:              rehydTenant,
			CredentialRefID: rehydCred,
			WorkspaceID:     rehydWS,
			BaseURL:         clockifyBaseURL,
			PolicyMode:      string(policy.Standard),
		},
		{
			ID:              rehydTenantOther,
			CredentialRefID: rehydCredOther,
			WorkspaceID:     rehydWSOther,
			BaseURL:         clockifyBaseURL,
			PolicyMode:      string(policy.Standard),
		},
	} {
		if err := store.PutTenant(tenant); err != nil {
			t.Fatalf("seed tenant %s: %v", tenant.ID, err)
		}
	}

	auth, err := authn.New(authn.Config{Mode: authn.ModeForwardAuth})
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}
	factory := sharedSvcFactory(store)

	// Two listeners, same store, same Factory closure — the deploy
	// shape under test is two replicas behind a load balancer that
	// routed `initialize` to instance A and the next request to
	// instance B (the failure mode the band-aid does NOT cover).
	srvA, baseURLA, stopA := startStreamableInstance(t, store, auth, factory)
	srvB, baseURLB, stopB := startStreamableInstance(t, store, auth, factory)
	t.Cleanup(stopA)
	t.Cleanup(stopB)
	_, _ = srvA, srvB // listeners owned by the goroutines started inside startStreamableInstance

	traffCtx, traffCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer traffCancel()

	// --- Step 1: cross-instance happy path --------------------------
	// initialize on A, tools/call on B with the same session ID. On
	// current main this fails with 404 "session not found" because
	// streamSessionManager.get is local-only. After the rehydration
	// fix lands, B reads the session record from the shared store
	// and rebuilds the per-tenant runtime via the Factory.
	clientA := &sharedSvcClient{t: t, baseURL: baseURLA, user: rehydSubject, tenant: rehydTenant}
	clientA.initialize(traffCtx)
	if clientA.sessID == "" {
		t.Fatalf("initialize on instance A returned no session id")
	}

	clientB := &sharedSvcClient{
		t:       t,
		baseURL: baseURLB,
		user:    rehydSubject,
		tenant:  rehydTenant,
		sessID:  clientA.sessID,
	}
	// callTool fatals on non-200 → on current main this fires the
	// failing-first assertion: "tools/call ... returned 404: ...
	// invalid session". The implementation commit makes it pass.
	clientB.callTool(traffCtx, "clockify_add_entry", map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		"end":         time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		"description": "rehydration cross-instance entry",
	})

	// Allow the audit pipeline a tick to flush before reading.
	time.Sleep(150 * time.Millisecond)

	// Audit rows for the cross-instance write must carry the same
	// session_id created on instance A. A rehydration that produced
	// a fresh session ID would split-brain the audit trail.
	var auditCount int
	if err := verifyPool.QueryRow(verifyCtx, `
		SELECT count(*) FROM audit_events
		 WHERE session_id = $1
		   AND tenant_id  = $2
		   AND tool       = 'clockify_add_entry'
	`, clientA.sessID, rehydTenant).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_events for cross-instance write: %v", err)
	}
	if auditCount < 2 {
		t.Fatalf("expected ≥2 audit rows (intent + outcome) on session %q tenant %q, got %d",
			clientA.sessID, rehydTenant, auditCount)
	}

	// --- Step 2: cross-tenant rejection on rehydration -------------
	// A request that presents the rehydrated session ID but a
	// different X-Forwarded-Tenant must be rejected with the same
	// "session principal mismatch" 403 the local-hit path returns.
	// Strict re-auth (ADR 0017 Q2) is preserved across rehydration:
	// the persisted Subject/TenantID are checked against the
	// freshly-authenticated principal, not trusted unconditionally.
	stolenStatus, stolenBody := rawCallTool(t, traffCtx, baseURLB, clientA.sessID, rehydSubjectOth, rehydTenantOther,
		"clockify_list_projects", map[string]any{})
	if stolenStatus != http.StatusForbidden {
		t.Fatalf("cross-tenant replay on instance B: expected 403, got %d (body: %s)", stolenStatus, stolenBody)
	}
	if !strings.Contains(strings.ToLower(stolenBody), "principal") &&
		!strings.Contains(strings.ToLower(stolenBody), "mismatch") {
		t.Fatalf("cross-tenant replay body did not mention principal mismatch: %s", stolenBody)
	}
	// Verify no audit row was written for the rejected attempt: the
	// per-request auth check happens before the handler dispatch, so
	// the audit pipeline never sees the call.
	var rejectedAudits int
	if err := verifyPool.QueryRow(verifyCtx, `
		SELECT count(*) FROM audit_events
		 WHERE session_id = $1
		   AND tenant_id  = $2
	`, clientA.sessID, rehydTenantOther).Scan(&rejectedAudits); err != nil {
		t.Fatalf("query audit_events for cross-tenant reject: %v", err)
	}
	if rejectedAudits != 0 {
		t.Fatalf("cross-tenant rejected request leaked %d audit rows for tenant %q on session %q",
			rejectedAudits, rehydTenantOther, clientA.sessID)
	}

	// --- Step 3: expired-session handling on rehydration -----------
	// Pre-write a SessionRecord with ExpiresAt in the past, then send
	// a tools/call to instance B with that ID. The rehydration path
	// must surface "session expired" (404) and remove the row from
	// the store so the next request does not see a phantom entry.
	expiredID := "sess-rehydration-svc-expired-001"
	pastCreated := time.Now().UTC().Add(-2 * time.Hour)
	pastExpired := time.Now().UTC().Add(-1 * time.Minute)
	if err := store.PutSession(controlplane.SessionRecord{
		ID:              expiredID,
		TenantID:        rehydTenant,
		Subject:         rehydSubject,
		Transport:       "streamable_http",
		CreatedAt:       pastCreated,
		ExpiresAt:       pastExpired,
		LastSeenAt:      pastCreated,
		WorkspaceID:     rehydWS,
		ClockifyBaseURL: clockifyBaseURL,
	}); err != nil {
		t.Fatalf("seed expired session: %v", err)
	}
	expStatus, expBody := rawCallTool(t, traffCtx, baseURLB, expiredID, rehydSubject, rehydTenant,
		"clockify_list_projects", map[string]any{})
	if expStatus != http.StatusNotFound {
		t.Fatalf("expired-session replay on instance B: expected 404, got %d (body: %s)", expStatus, expBody)
	}
	if !strings.Contains(strings.ToLower(expBody), "expired") &&
		!strings.Contains(strings.ToLower(expBody), "invalid session") {
		t.Fatalf("expired-session reject body did not mention expiry: %s", expBody)
	}
	// The rehydration path must DELETE the row from the store on
	// expiry, mirroring the local-hit eviction in get(). Otherwise a
	// stale row sits forever consuming index pages.
	var stillThere int
	if err := verifyPool.QueryRow(verifyCtx,
		`SELECT count(*) FROM sessions WHERE id = $1`, expiredID,
	).Scan(&stillThere); err != nil {
		t.Fatalf("query expired session row: %v", err)
	}
	if stillThere != 0 {
		t.Fatalf("expired session %q was not removed from store after rehydration reject", expiredID)
	}
}

// startStreamableInstance boots a single mcp.ServeStreamableHTTP
// listener against the supplied store + Factory, returning the
// base URL and a stop function. The caller registers stop in
// t.Cleanup; the helper does NOT itself register cleanups so the
// caller controls ordering when both listeners share state.
func startStreamableInstance(
	t *testing.T,
	store controlplane.Store,
	auth authn.Authenticator,
	factory mcp.StreamableSessionFactory,
) (net.Listener, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	baseURL := "http://" + ln.Addr().String()
	srvCtx, srvCancel := context.WithCancel(context.Background())
	srvDone := make(chan struct{})
	go func() {
		_ = mcp.ServeStreamableHTTP(srvCtx, mcp.StreamableHTTPOptions{
			Listener:      ln,
			MaxBodySize:   4 * 1024 * 1024,
			SessionTTL:    5 * time.Minute,
			Authenticator: auth,
			ControlPlane:  store,
			Factory:       factory,
		})
		close(srvDone)
	}()
	if err := waitForReady(baseURL+"/health", 5*time.Second); err != nil {
		srvCancel()
		<-srvDone
		t.Fatalf("wait for ready (%s): %v", baseURL, err)
	}
	stop := func() {
		srvCancel()
		select {
		case <-srvDone:
		case <-time.After(10 * time.Second):
			t.Logf("streamable HTTP server at %s did not shut down within 10s", baseURL)
		}
	}
	return ln, baseURL, stop
}

// rawCallTool sends a tools/call request without going through
// sharedSvcClient (which fatals on non-200) so the test can assert
// on the exact status + body returned for the rejection paths
// (cross-tenant replay, expired session). Headers mirror the
// forward_auth contract: X-Forwarded-User + X-Forwarded-Tenant.
func rawCallTool(
	t *testing.T,
	ctx context.Context,
	baseURL, sessID, user, tenant, tool string,
	args map[string]any,
) (int, string) {
	t.Helper()
	body := jsonrpcEnvelope(99, "tools/call", map[string]any{
		"name":      tool,
		"arguments": args,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-User", user)
	req.Header.Set("X-Forwarded-Tenant", tenant)
	req.Header.Set("MCP-Session-Id", sessID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, readBody(resp)
}
