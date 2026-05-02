//go:build postgres

// Shared-service Postgres end-to-end test for the launch-candidate
// gate (Group 2 of docs/launch-candidate-checklist.md). Boots
// mcp.ServeStreamableHTTP in-process against a Postgres-backed
// control plane, drives multi-tenant traffic over the streamable
// HTTP transport with forward_auth principals, and asserts:
//
//  1. Each tenant's audit_events rows are stamped with the correct
//     tenant_id and stay invisible from queries scoped to the other
//     tenant_id.
//  2. Each tenant's session row carries the principal-supplied
//     tenant_id (not a default fallback).
//  3. Per-tenant policy_mode is honored: time_tracking_safe blocks
//     clockify_create_project before the audit pipeline emits.
//  4. Read-only tools (clockify_list_projects) emit zero audit rows.
//
// The test uses a local httptest fake Clockify so the contract it
// proves is the wiring (config -> store -> transport -> audit), not
// the upstream behaviour. Real-Clockify audit invariants are pinned
// separately by TestLiveCreateUpdateDeleteEntryAuditPhases.
//
// The Factory closure below mirrors the production
// internal/runtime/service.go::tenantRuntime shape; if that drifts
// (vault lookup logic, per-tenant policy override, audit wiring),
// this test must be updated to track. The drift risk is documented
// inline at sharedSvcFactory below.
//
// Activation: build tag `postgres` (the postgres sub-module is gated
// by it) plus a non-empty MCP_LIVE_CONTROL_PLANE_DSN env var. CI
// runs this in a per-PR job with a postgres:16-alpine service
// container; locally the Railway sacrificial cluster
// (clockify_mcp_e2e DB) is the convention.

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/auditbridge"
	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/tools"
	"github.com/apet97/go-clockify/internal/truncate"
)

const (
	sharedSvcTenantA  = "tenant-svc-e2e-A"
	sharedSvcTenantB  = "tenant-svc-e2e-B"
	sharedSvcCredA    = "cred-svc-e2e-A"
	sharedSvcCredB    = "cred-svc-e2e-B"
	sharedSvcWSA      = "ws-svc-e2e-A"
	sharedSvcWSB      = "ws-svc-e2e-B"
	sharedSvcSubjectA = "alice@svc-e2e.test"
	sharedSvcSubjectB = "bot@svc-e2e.test"
)

// sharedSvcAuditor mirrors internal/runtime/service.go::controlPlaneAuditor
// (which is unexported). The conversion logic lives in
// internal/auditbridge so the production path and this test exercise
// the same external_id synthesis.
type sharedSvcAuditor struct {
	store controlplane.Store
}

func (a sharedSvcAuditor) RecordAudit(event mcp.AuditEvent) error {
	if a.store == nil {
		return nil
	}
	return a.store.AppendAuditEvent(auditbridge.ToControlPlaneEvent(event, time.Now().UTC()))
}

// fakeClockify serves the minimum endpoint surface the three tools
// in the traffic mix touch: GET/POST projects and POST time-entries.
// It accepts any X-Api-Key header — auth is a transport-side concern
// the test exercises through forward_auth, not the Clockify upstream.
type fakeClockify struct {
	mu     sync.Mutex
	projID int
	entID  int
}

func newFakeClockify(t *testing.T) *httptest.Server {
	t.Helper()
	fc := &fakeClockify{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/workspaces/", fc.serveWorkspaceScoped)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (fc *fakeClockify) serveWorkspaceScoped(w http.ResponseWriter, r *http.Request) {
	// Path shape: /v1/workspaces/{wsID}/projects[?...] or
	// /v1/workspaces/{wsID}/time-entries[/{id}]. The tools call paths
	// that begin with /workspaces/... which the client appends to
	// baseURL — set baseURL to "<server>/v1" so the full URL lines up.
	rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	wsID := parts[0]
	resource := parts[1]
	switch resource {
	case "projects":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, []clockify.Project{
				{ID: "proj-fake-1", Name: "Fake Project 1"},
			})
		case http.MethodPost:
			fc.mu.Lock()
			fc.projID++
			id := fmt.Sprintf("proj-fake-created-%d", fc.projID)
			fc.mu.Unlock()
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			name, _ := payload["name"].(string)
			writeJSON(w, clockify.Project{ID: id, Name: name})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "time-entries":
		switch r.Method {
		case http.MethodPost:
			fc.mu.Lock()
			fc.entID++
			id := fmt.Sprintf("entry-fake-%s-%d", wsID, fc.entID)
			fc.mu.Unlock()
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			start, _ := payload["start"].(string)
			end, _ := payload["end"].(string)
			desc, _ := payload["description"].(string)
			writeJSON(w, clockify.TimeEntry{
				ID:          id,
				Description: desc,
				WorkspaceID: wsID,
				TimeInterval: clockify.TimeInterval{
					Start: start,
					End:   end,
				},
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// sharedSvcFactory constructs the per-session runtime by mirroring
// the shape of internal/runtime/service.go::tenantRuntime — same
// per-tenant policy override, same per-tenant Clockify client,
// same enforcement Pipeline + Gate. It does NOT go through the
// vault layer (there are no secrets to resolve in the test) and
// uses a fixed-string API key the fake accepts unconditionally.
//
// If tenantRuntime in production grows behaviour this closure
// does not mirror, the test silently asserts the wrong contract.
// Keep them in sync; consider extracting a shared
// internal/runtime/factory helper if drift becomes a recurring
// pain.
func sharedSvcFactory(store controlplane.Store) mcp.StreamableSessionFactory {
	auditor := sharedSvcAuditor{store: store}
	return func(_ context.Context, principal authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
		tenant, ok := store.Tenant(principal.TenantID)
		if !ok {
			return nil, fmt.Errorf("tenant %q not found in control plane", principal.TenantID)
		}
		client := clockify.NewClient("svc-e2e-key", tenant.BaseURL, 30*time.Second, 0)
		client.SetUserAgent("clockify-mcp-svc-e2e/test")

		pol := &policy.Policy{Mode: policy.Mode(tenant.PolicyMode)}
		bc := &bootstrap.Config{Mode: bootstrap.FullTier1}
		service := tools.New(client, tenant.WorkspaceID)
		registry := service.Registry()
		tier1 := make(map[string]bool, len(registry))
		for _, d := range registry {
			tier1[d.Tool.Name] = true
		}
		pol.SetTier1Tools(tier1)
		bc.SetTier1Tools(tier1)
		service.PolicyDescribe = pol.Describe

		pipeline := &enforcement.Pipeline{
			Policy:     pol,
			Bootstrap:  bc,
			DryRun:     dryrun.Config{Enabled: false},
			Truncation: truncate.Config{},
		}
		gate := &enforcement.Gate{Policy: pol, Bootstrap: bc}

		server := mcp.NewServer("svc-e2e", registry, pipeline, gate)
		server.MaxMessageSize = 8 * 1024 * 1024
		server.Auditor = auditor
		// best_effort keeps the test focused on the isolation contract;
		// the audit-fail-closed behaviour is pinned by
		// TestLiveCreateUpdateDeleteEntryAuditPhases.
		server.AuditDurabilityMode = "best_effort"

		return &mcp.StreamableSessionRuntime{
			Server:          server,
			Close:           client.Close,
			TenantID:        tenant.ID,
			WorkspaceID:     tenant.WorkspaceID,
			ClockifyBaseURL: tenant.BaseURL,
		}, nil
	}
}

// sharedSvcClient is a tiny HTTP client that drives the streamable
// HTTP listener as a forward_auth principal: every request stamps
// the X-Forwarded-User and X-Forwarded-Tenant headers. The session
// ID returned by the initialize response is captured for use on
// subsequent tools/call requests against the same session.
type sharedSvcClient struct {
	t        *testing.T
	baseURL  string
	user     string
	tenant   string
	sessID   string
	requests int
}

func (c *sharedSvcClient) initialize(ctx context.Context) {
	c.t.Helper()
	c.requests++
	body := jsonrpcEnvelope(c.requests, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "svc-e2e", "version": "test"},
		"capabilities":    map[string]any{},
	})
	resp := c.do(ctx, body, "")
	if c.sessID == "" {
		// Streamable HTTP returns the session ID in MCP-Session-Id (and
		// X-MCP-Session-ID for legacy clients). Either header works.
		c.sessID = resp.Header.Get("MCP-Session-Id")
		if c.sessID == "" {
			c.sessID = resp.Header.Get("X-MCP-Session-ID")
		}
	}
	if c.sessID == "" {
		c.t.Fatalf("initialize for tenant %q returned no session id; status=%d body=%s",
			c.tenant, resp.StatusCode, readBody(resp))
	}
	_ = resp.Body.Close()
}

func (c *sharedSvcClient) callTool(ctx context.Context, tool string, args map[string]any) map[string]any {
	c.t.Helper()
	c.requests++
	body := jsonrpcEnvelope(c.requests, "tools/call", map[string]any{
		"name":      tool,
		"arguments": args,
	})
	resp := c.do(ctx, body, c.sessID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("tools/call %s for tenant %q returned %d: %s", tool, c.tenant, resp.StatusCode, readBody(resp))
	}
	var rpc struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		c.t.Fatalf("decode tools/call response: %v", err)
	}
	if rpc.Error != nil {
		c.t.Fatalf("tools/call %s for tenant %q returned rpc error: code=%d msg=%s",
			tool, c.tenant, rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result
}

// callToolExpectError sends a tools/call expecting an isError envelope
// (policy-blocked or handler-error path) and returns the textual error
// message. Mirrors the e2e_live_mcp_test.go::callExpectError helper.
func (c *sharedSvcClient) callToolExpectError(ctx context.Context, tool string, args map[string]any) string {
	c.t.Helper()
	c.requests++
	body := jsonrpcEnvelope(c.requests, "tools/call", map[string]any{
		"name":      tool,
		"arguments": args,
	})
	resp := c.do(ctx, body, c.sessID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("tools/call %s for tenant %q expected http 200 + isError, got %d: %s",
			tool, c.tenant, resp.StatusCode, readBody(resp))
	}
	var rpc struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		c.t.Fatalf("decode tools/call response: %v", err)
	}
	if rpc.Error != nil {
		// Policy-block currently surfaces as an MCP isError envelope on
		// the success channel, not as a JSON-RPC -32xxx error. If a
		// future refactor moves it to the rpc-error channel, prefer
		// that path explicitly here.
		return rpc.Error.Message
	}
	if isErr, _ := rpc.Result["isError"].(bool); !isErr {
		c.t.Fatalf("tools/call %s for tenant %q expected isError=true, got %#v", tool, c.tenant, rpc.Result)
	}
	contents, _ := rpc.Result["content"].([]any)
	for _, item := range contents {
		m, _ := item.(map[string]any)
		if text, _ := m["text"].(string); text != "" {
			return text
		}
	}
	return ""
}

func (c *sharedSvcClient) do(ctx context.Context, body []byte, sessID string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-User", c.user)
	req.Header.Set("X-Forwarded-Tenant", c.tenant)
	if sessID != "" {
		req.Header.Set("MCP-Session-Id", sessID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("send request: %v", err)
	}
	return resp
}

func jsonrpcEnvelope(id int, method string, params map[string]any) []byte {
	env := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	b, _ := json.Marshal(env)
	return b
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(b)
}

func TestSharedServicePostgresE2E(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MCP_LIVE_CONTROL_PLANE_DSN"))
	if dsn == "" {
		if os.Getenv("INTEGRATION_REQUIRED") == "1" {
			t.Fatalf("MCP_LIVE_CONTROL_PLANE_DSN unset under INTEGRATION_REQUIRED=1")
		}
		t.Skip("MCP_LIVE_CONTROL_PLANE_DSN not set; skipping shared-service Postgres E2E")
	}

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
	// the row counts. Tagged-prefix keeps blast radius scoped.
	mustExec(t, verifyCtx, verifyPool,
		`DELETE FROM audit_events WHERE tenant_id LIKE 'tenant-svc-e2e-%'`)
	mustExec(t, verifyCtx, verifyPool,
		`DELETE FROM sessions WHERE tenant_id LIKE 'tenant-svc-e2e-%'`)
	mustExec(t, verifyCtx, verifyPool,
		`DELETE FROM tenants WHERE id LIKE 'tenant-svc-e2e-%'`)
	mustExec(t, verifyCtx, verifyPool,
		`DELETE FROM credential_refs WHERE id LIKE 'cred-svc-e2e-%'`)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = verifyPool.Exec(cleanupCtx, `DELETE FROM audit_events WHERE tenant_id LIKE 'tenant-svc-e2e-%'`)
		_, _ = verifyPool.Exec(cleanupCtx, `DELETE FROM sessions WHERE tenant_id LIKE 'tenant-svc-e2e-%'`)
		_, _ = verifyPool.Exec(cleanupCtx, `DELETE FROM tenants WHERE id LIKE 'tenant-svc-e2e-%'`)
		_, _ = verifyPool.Exec(cleanupCtx, `DELETE FROM credential_refs WHERE id LIKE 'cred-svc-e2e-%'`)
	})

	// Stand up the fake Clockify upstream. Both tenants point at the
	// same fake; tenant identity is solely a transport+control-plane
	// concept here.
	fake := newFakeClockify(t)
	fakeURL, err := url.Parse(fake.URL)
	if err != nil {
		t.Fatalf("parse fake url: %v", err)
	}
	clockifyBaseURL := fakeURL.String() + "/v1"

	// Cross-check: the tenants we are about to seed point at the fake,
	// not real Clockify. If a misconfiguration ever pointed at
	// api.clockify.me here, the assertion fires before any HTTP call.
	if !strings.HasPrefix(clockifyBaseURL, "http://127.0.0.1:") &&
		!strings.HasPrefix(clockifyBaseURL, "http://[::1]:") {
		t.Fatalf("fake clockify must bind to loopback, got %q", clockifyBaseURL)
	}

	// Seed credentials and tenants. Per-tenant policy_mode = standard
	// for the operator persona, time_tracking_safe for the AI-facing
	// persona; the gate must honor the tenant's setting per-session.
	for _, ref := range []controlplane.CredentialRef{
		{ID: sharedSvcCredA, Backend: "inline", Reference: "svc-e2e-key", Workspace: sharedSvcWSA, BaseURL: clockifyBaseURL},
		{ID: sharedSvcCredB, Backend: "inline", Reference: "svc-e2e-key", Workspace: sharedSvcWSB, BaseURL: clockifyBaseURL},
	} {
		if err := store.PutCredentialRef(ref); err != nil {
			t.Fatalf("seed credential ref %s: %v", ref.ID, err)
		}
	}
	for _, tenant := range []controlplane.TenantRecord{
		{
			ID:              sharedSvcTenantA,
			CredentialRefID: sharedSvcCredA,
			WorkspaceID:     sharedSvcWSA,
			BaseURL:         clockifyBaseURL,
			PolicyMode:      string(policy.Standard),
		},
		{
			ID:              sharedSvcTenantB,
			CredentialRefID: sharedSvcCredB,
			WorkspaceID:     sharedSvcWSB,
			BaseURL:         clockifyBaseURL,
			PolicyMode:      string(policy.TimeTrackingSafe),
		},
	} {
		if err := store.PutTenant(tenant); err != nil {
			t.Fatalf("seed tenant %s: %v", tenant.ID, err)
		}
	}

	// Listener on a kernel-assigned ephemeral port so the test can
	// run concurrently with anything else binding 127.0.0.1.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	baseURL := "http://" + ln.Addr().String()

	auth, err := authn.New(authn.Config{
		Mode: authn.ModeForwardAuth,
		// Empty trusted-proxies list = trust every source. Adequate for
		// a 127.0.0.1-bound test fixture; production deployments must
		// constrain this. Comment kept here so a future reader does
		// not interpret the empty list as a recommended posture.
	})
	if err != nil {
		t.Fatalf("authn.New: %v", err)
	}

	srvCtx, srvCancel := context.WithCancel(context.Background())
	srvDone := make(chan struct{})
	go func() {
		_ = mcp.ServeStreamableHTTP(srvCtx, mcp.StreamableHTTPOptions{
			Listener:      ln,
			MaxBodySize:   4 * 1024 * 1024,
			SessionTTL:    5 * time.Minute,
			Authenticator: auth,
			ControlPlane:  store,
			Factory:       sharedSvcFactory(store),
		})
		close(srvDone)
	}()
	t.Cleanup(func() {
		srvCancel()
		select {
		case <-srvDone:
		case <-time.After(10 * time.Second):
			t.Logf("streamable HTTP server did not shut down within 10s")
		}
	})

	if err := waitForReady(baseURL+"/health", 5*time.Second); err != nil {
		t.Fatalf("wait for ready: %v", err)
	}

	traffCtx, traffCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer traffCancel()

	// Two principals, one listener. Forward-auth headers identify the
	// tenant per-request; the session manager creates a fresh session
	// (and a fresh per-session Server via the Factory) per principal.
	clientA := &sharedSvcClient{t: t, baseURL: baseURL, user: sharedSvcSubjectA, tenant: sharedSvcTenantA}
	clientB := &sharedSvcClient{t: t, baseURL: baseURL, user: sharedSvcSubjectB, tenant: sharedSvcTenantB}
	clientA.initialize(traffCtx)
	clientB.initialize(traffCtx)

	if clientA.sessID == clientB.sessID {
		t.Fatalf("expected distinct session ids per tenant, got duplicate %q", clientA.sessID)
	}

	// Call 1: tenant A operator reads projects (read-only, no audit).
	clientA.callTool(traffCtx, "clockify_list_projects", map[string]any{})
	// Call 2: tenant A operator writes a time entry (allowed under
	// standard policy → 1 intent + 1 outcome).
	clientA.callTool(traffCtx, "clockify_add_entry", map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		"end":         time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		"description": "svc-e2e A entry",
	})
	// Call 3: tenant B AI-facing reads projects (read-only, no audit).
	clientB.callTool(traffCtx, "clockify_list_projects", map[string]any{})
	// Call 4: tenant B AI-facing tries to create a project. Policy gate
	// must reject (time_tracking_safe blocks project mutations); the
	// expected audit row count is asserted below.
	errMsg := clientB.callToolExpectError(traffCtx, "clockify_create_project", map[string]any{
		"name": "svc-e2e B project",
	})
	if !strings.Contains(strings.ToLower(errMsg), "policy") &&
		!strings.Contains(strings.ToLower(errMsg), "time_tracking_safe") &&
		!strings.Contains(strings.ToLower(errMsg), "blocked") {
		t.Fatalf("call 4: expected policy-block message, got %q", errMsg)
	}
	// Call 5: tenant B AI-facing writes a time entry (allowed under
	// time_tracking_safe → 1 intent + 1 outcome).
	clientB.callTool(traffCtx, "clockify_add_entry", map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		"end":         time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		"description": "svc-e2e B entry",
	})

	// Allow the audit pipeline a tick to flush before reading.
	time.Sleep(100 * time.Millisecond)

	// --- Assertion 1: audit row count + tenant_id partitioning ------
	rows, err := verifyPool.Query(verifyCtx, `
		SELECT tenant_id, tool, phase, outcome
		  FROM audit_events
		 WHERE session_id IN ($1, $2)
		 ORDER BY at ASC, external_id ASC
	`, clientA.sessID, clientB.sessID)
	if err != nil {
		t.Fatalf("query audit_events: %v", err)
	}
	defer rows.Close()
	type observed struct {
		tenantID, tool, phase, outcome string
	}
	var got []observed
	for rows.Next() {
		var o observed
		if err := rows.Scan(&o.tenantID, &o.tool, &o.phase, &o.outcome); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	// Expect exactly 5 rows across both tenants:
	//   tenant A clockify_add_entry: intent + outcome (2 rows)
	//   tenant B clockify_create_project: 1 policy-denied row
	//     (phase="" because the gate runs before the two-phase
	//      audit pipeline; it stamps a single-shot outcome row
	//      to leave a forensic trail of the blocked attempt)
	//   tenant B clockify_add_entry: intent + outcome (2 rows)
	// A regression in the gate that lets the create_project through
	// would produce 6 rows (intent + outcome) instead of 1; a regression
	// that drops the policy-denied audit altogether would produce 4.
	const expectedRows = 5
	if len(got) != expectedRows {
		t.Fatalf("expected %d audit rows across tenants, got %d:\n%+v", expectedRows, len(got), got)
	}

	type rowKey struct{ tenantID, tool, phase, outcome string }
	expectedByKey := map[rowKey]int{
		{sharedSvcTenantA, "clockify_add_entry", "intent", "attempted"}:    1,
		{sharedSvcTenantA, "clockify_add_entry", "outcome", "success"}:     1,
		{sharedSvcTenantB, "clockify_create_project", "", "policy_denied"}: 1,
		{sharedSvcTenantB, "clockify_add_entry", "intent", "attempted"}:    1,
		{sharedSvcTenantB, "clockify_add_entry", "outcome", "success"}:     1,
	}
	gotByKey := map[rowKey]int{}
	for _, o := range got {
		gotByKey[rowKey{o.tenantID, o.tool, o.phase, o.outcome}]++
	}
	for key, want := range expectedByKey {
		if gotByKey[key] != want {
			t.Fatalf("audit row count for %+v = %d, want %d (full set: %+v)",
				key, gotByKey[key], want, got)
		}
	}
	for key, count := range gotByKey {
		if _, ok := expectedByKey[key]; !ok {
			t.Fatalf("unexpected audit row %+v (count=%d, full set: %+v)", key, count, got)
		}
	}

	// --- Assertion 2 (cross-tenant negative) ------------------------
	// THE primary contract this test asserts: no audit row for tenant A
	// is reachable via a query scoped to tenant B's session, and vice
	// versa. A tenant_id mis-stamping bug would make this query return
	// non-zero. Drift-check target.
	var crossAB int
	if err := verifyPool.QueryRow(verifyCtx,
		`SELECT count(*) FROM audit_events WHERE tenant_id = $1 AND session_id = $2`,
		sharedSvcTenantA, clientB.sessID,
	).Scan(&crossAB); err != nil {
		t.Fatalf("cross-tenant A→B query: %v", err)
	}
	if crossAB != 0 {
		t.Fatalf("expected 0 audit rows for cross-tenant query (tenant A, session B), got %d", crossAB)
	}
	var crossBA int
	if err := verifyPool.QueryRow(verifyCtx,
		`SELECT count(*) FROM audit_events WHERE tenant_id = $1 AND session_id = $2`,
		sharedSvcTenantB, clientA.sessID,
	).Scan(&crossBA); err != nil {
		t.Fatalf("cross-tenant B→A query: %v", err)
	}
	if crossBA != 0 {
		t.Fatalf("expected 0 audit rows for cross-tenant query (tenant B, session A), got %d", crossBA)
	}

	// --- Assertion 3: sessions tenant_id partitioning ---------------
	var sessTenantA, sessTenantB string
	if err := verifyPool.QueryRow(verifyCtx,
		`SELECT tenant_id FROM sessions WHERE id = $1`, clientA.sessID,
	).Scan(&sessTenantA); err != nil {
		t.Fatalf("query session A: %v", err)
	}
	if err := verifyPool.QueryRow(verifyCtx,
		`SELECT tenant_id FROM sessions WHERE id = $1`, clientB.sessID,
	).Scan(&sessTenantB); err != nil {
		t.Fatalf("query session B: %v", err)
	}
	if sessTenantA != sharedSvcTenantA {
		t.Fatalf("session A tenant_id = %q, want %q", sessTenantA, sharedSvcTenantA)
	}
	if sessTenantB != sharedSvcTenantB {
		t.Fatalf("session B tenant_id = %q, want %q", sessTenantB, sharedSvcTenantB)
	}
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func waitForReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("ready probe %s timed out after %s", url, timeout)
}
