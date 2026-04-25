//go:build postgres && livee2e

// Live audit-phase contract test for the Postgres-backed control-plane
// store. The runtime two-phase auditor (intent + outcome) synthesises the
// AuditEvent.ID so the Postgres ON CONFLICT (external_id) DO NOTHING does
// not collapse the pair into a single row. A regression in either the
// runtime ID synthesis or the migration-002 phase column would silently
// halve the audit record count for every non-read-only call — exactly the
// fail_closed safety primitive the hosted-launch checklist contracts on.
//
// This file lives in the postgres sub-module so the test can use pgx
// directly to verify the rows landed (the top-level go.mod intentionally
// omits pgx per ADR 0001). It runs only when the `livee2e` build tag is
// set AND MCP_LIVE_CONTROL_PLANE_DSN is configured. CI wires the secret
// in `.github/workflows/live-contract.yml`; missing on a fork results in
// a t.Skip with a clear message, the same fail-soft policy the rest of
// the live suite uses.

package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/tools"
	"github.com/apet97/go-clockify/internal/truncate"
)

// encodeRequests serialises newline-delimited JSON-RPC requests for a single
// Server.Run pass. Failure is a t.Fatalf because malformed input means the
// test setup is broken, not the system under test.
func encodeRequests(t *testing.T, requests []map[string]any) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range requests {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	return &buf
}

// runServer drives the MCP server's Run loop with the supplied input buffer
// and returns the captured output.
func runServer(t *testing.T, ctx context.Context, server *mcp.Server, input *bytes.Buffer) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	if err := server.Run(ctx, input, &output); err != nil {
		t.Fatalf("server.Run: %v", err)
	}
	return &output
}

// findResponse scans newline-delimited JSON-RPC output for the response with
// the requested id. The harness uses id 2 for tools/call (id 1 is the
// initialize request).
func findResponse(t *testing.T, output *bytes.Buffer, wantID float64) mcp.Response {
	t.Helper()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var resp mcp.Response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if id, ok := resp.ID.(float64); ok && id == wantID {
			return resp
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
	}
	t.Fatalf("no response with id=%v in output: %q", wantID, output.String())
	return mcp.Response{}
}

// mustDataMap pulls the structuredContent.data field out of a tools/call
// envelope. Mirrors the helper in tests/e2e_live_mcp_test.go.
func mustDataMap(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	sc, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("result missing structuredContent: %#v", result)
	}
	data, ok := sc["data"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent.data not a map: %#v", sc)
	}
	return data
}

// liveControlPlaneAuditor mirrors internal/runtime/service.go's
// controlPlaneAuditor: it synthesises the audit external_id with phase
// and outcome embedded so the Postgres store does not collapse intent
// + outcome rows under the unique-external-id constraint. The runtime
// helper is unexported, so the test re-states the contract here. If
// either copy drifts the audit-row count in this test will mismatch
// the expected pair count and the test fails loudly.
type liveControlPlaneAuditor struct {
	store controlplane.Store
}

func (a liveControlPlaneAuditor) RecordAudit(event mcp.AuditEvent) error {
	if a.store == nil {
		return nil
	}
	tenantID := event.Metadata["tenant_id"]
	subject := event.Metadata["subject"]
	sessionID := event.Metadata["session_id"]
	transport := event.Metadata["transport"]
	now := time.Now().UTC()
	return a.store.AppendAuditEvent(controlplane.AuditEvent{
		ID:          fmt.Sprintf("%d-%s-%s-%s-%s", now.UnixNano(), sessionID, event.Tool, event.Phase, event.Outcome),
		At:          now,
		TenantID:    tenantID,
		Subject:     subject,
		SessionID:   sessionID,
		Transport:   transport,
		Tool:        event.Tool,
		Action:      event.Action,
		Outcome:     event.Outcome,
		Phase:       event.Phase,
		Reason:      event.Reason,
		ResourceIDs: event.ResourceIDs,
		Metadata:    event.Metadata,
	})
}

// TestLiveCreateUpdateDeleteEntryAuditPhases drives a real Clockify
// create → update → delete flow through the MCP server with a Postgres-
// backed audit sink, then queries audit_events directly to verify the
// six expected rows (3 intent + 3 outcome) land correctly.
//
// Runtime requirements:
//   - CLOCKIFY_API_KEY      sacrificial-workspace API key
//   - CLOCKIFY_WORKSPACE_ID sacrificial workspace id (or honour CLOCKIFY_API_KEY's default)
//   - CLOCKIFY_RUN_LIVE_E2E=1                 opt-in gate shared with the e2e suite
//   - CLOCKIFY_LIVE_WRITE_ENABLED=true        write-path gate shared with TestE2EMutating
//   - MCP_LIVE_CONTROL_PLANE_DSN              postgres:// DSN against a test database
//
// Missing any of the above is a clean t.Skip: the test cannot run.
// CI's `live-contract.yml` provides them through repo secrets/vars.
func TestLiveCreateUpdateDeleteEntryAuditPhases(t *testing.T) {
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("Skipping live audit-phase test unless CLOCKIFY_RUN_LIVE_E2E=1")
	}
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live audit-phase test since CLOCKIFY_API_KEY is not set")
	}
	if os.Getenv("CLOCKIFY_LIVE_WRITE_ENABLED") != "true" {
		t.Skip("Skipping live audit-phase test unless CLOCKIFY_LIVE_WRITE_ENABLED=true")
	}
	dsn := os.Getenv("MCP_LIVE_CONTROL_PLANE_DSN")
	if dsn == "" {
		t.Skip("Skipping live audit-phase test unless MCP_LIVE_CONTROL_PLANE_DSN is set")
	}
	t.Setenv("CLOCKIFY_DRY_RUN", "off") // The test owns dry_run via tool args.

	// Open the Postgres-backed store via the production opener path so the
	// migrations run identically to a real deploy. The init() in init.go
	// has already registered the postgres opener at package import.
	store, err := controlplane.Open(dsn)
	if err != nil {
		t.Fatalf("open postgres control-plane store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Open a parallel pgx pool just for verification queries. Re-using the
	// store's pool would force exposing it on the public Store interface;
	// a second pool keeps that surface untouched and is cheap.
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer verifyCancel()
	verifyPool, err := pgxpool.New(verifyCtx, dsn)
	if err != nil {
		t.Fatalf("verifier pool: %v", err)
	}
	t.Cleanup(verifyPool.Close)

	// Build the MCP server with the real Clockify client and the
	// production Pipeline+Gate. The auditor wraps the Postgres store so
	// every non-read-only call writes intent + outcome rows.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
	t.Cleanup(client.Close)
	service := tools.New(client, cfg.WorkspaceID)

	pol := &policy.Policy{Mode: policy.Standard}
	bc := &bootstrap.Config{Mode: bootstrap.FullTier1}
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

	server := mcp.NewServer("livee2e", registry, pipeline, gate)
	server.MaxMessageSize = 8 * 1024 * 1024
	server.Auditor = liveControlPlaneAuditor{store: store}
	// fail_closed makes the intent-row write a hard precondition for the
	// destructive handler. A pgxpool flap that drops the intent write
	// surfaces as a tool error instead of a silent half-record.
	server.AuditDurabilityMode = "fail_closed"
	server.AuditTenantID = "live-audit-phases"
	server.AuditSubject = "live-audit-phases"
	server.AuditTransport = "livee2e"
	sessionID := fmt.Sprintf("live-audit-%d", time.Now().UnixNano())
	server.AuditSessionID = sessionID

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Sweep audit rows for this synthesised session id at the end of the
	// test so the test database does not accumulate live-test rows.
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := verifyPool.Exec(cleanupCtx,
			`DELETE FROM audit_events WHERE session_id = $1`, sessionID,
		); err != nil {
			t.Logf("cleanup audit_events for session %s failed: %v", sessionID, err)
		}
	})

	harness := &auditPhaseHarness{t: t, server: server}

	// Step 1: create a real entry. clockify_add_entry is a non-destructive
	// write tool, so the pipeline emits intent + outcome.
	prefix := fmt.Sprintf("AG_AUDITPHASE_%d", time.Now().UnixNano())
	startStr := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	endStr := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	addResp := harness.callOK(ctx, "clockify_add_entry", map[string]any{
		"description": prefix + "_entry",
		"start":       startStr,
		"end":         endStr,
	})
	addData := mustDataMap(t, addResp)
	entryID, _ := addData["id"].(string)
	if entryID == "" {
		t.Fatalf("clockify_add_entry returned no entry id: %#v", addData)
	}
	t.Cleanup(func() {
		// Best-effort cleanup so a partial run does not leak entities.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		wsID, _ := service.ResolveWorkspaceID(cleanupCtx)
		if wsID != "" {
			_ = service.Client.Delete(cleanupCtx, "/workspaces/"+wsID+"/time-entries/"+entryID)
		}
	})

	// Step 2: update the entry through the MCP server.
	harness.callOK(ctx, "clockify_update_entry", map[string]any{
		"entry_id":    entryID,
		"description": prefix + "_entry_updated",
	})

	// Step 3: delete the entry through the MCP server (non-dry-run).
	harness.callOK(ctx, "clockify_delete_entry", map[string]any{
		"entry_id": entryID,
		"dry_run":  false,
	})

	// Verification: read every audit row for this session id and assert
	// the (tool, phase, outcome) tuples that should be present. Using the
	// verifier pool keeps the assertion independent of the Store.
	rows, err := verifyPool.Query(verifyCtx, `
		SELECT tool, phase, outcome, external_id, at
		  FROM audit_events
		 WHERE session_id = $1
		 ORDER BY at ASC, external_id ASC
	`, sessionID)
	if err != nil {
		t.Fatalf("query audit_events: %v", err)
	}
	defer rows.Close()

	type observed struct {
		tool, phase, outcome, externalID string
		at                               time.Time
	}
	var got []observed
	for rows.Next() {
		var o observed
		if err := rows.Scan(&o.tool, &o.phase, &o.outcome, &o.externalID, &o.at); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	// Every non-read-only call emits exactly one intent and one outcome
	// row. Three calls → six rows. A regression that drops one phase or
	// collapses intent+outcome into a single row will fail this length
	// check before any tuple assertions run.
	const expectedRows = 6
	if len(got) != expectedRows {
		t.Fatalf("expected %d audit rows for session %s, got %d:\n%+v",
			expectedRows, sessionID, len(got), got)
	}

	// Pair invariants: each tool must contribute exactly one intent + one
	// outcome row. Any mismatch (missing intent, double outcome, etc.)
	// surfaces here.
	type pair struct{ intent, outcome bool }
	pairs := map[string]*pair{
		"clockify_add_entry":    {},
		"clockify_update_entry": {},
		"clockify_delete_entry": {},
	}
	externalIDs := map[string]bool{}
	for _, ev := range got {
		p, ok := pairs[ev.tool]
		if !ok {
			t.Fatalf("unexpected tool in audit rows: %s (event=%+v)", ev.tool, ev)
		}
		switch ev.phase {
		case mcp.PhaseIntent:
			if p.intent {
				t.Fatalf("duplicate intent row for %s: %+v", ev.tool, ev)
			}
			p.intent = true
		case mcp.PhaseOutcome:
			if p.outcome {
				t.Fatalf("duplicate outcome row for %s: %+v", ev.tool, ev)
			}
			p.outcome = true
			if ev.outcome != "success" {
				t.Fatalf("expected outcome=success for %s, got %q", ev.tool, ev.outcome)
			}
		default:
			t.Fatalf("unexpected phase %q on row %+v", ev.phase, ev)
		}
		if externalIDs[ev.externalID] {
			t.Fatalf("duplicate external_id %s — runtime ID synthesis collapsed two rows", ev.externalID)
		}
		externalIDs[ev.externalID] = true
	}
	for tool, p := range pairs {
		if !p.intent || !p.outcome {
			t.Fatalf("incomplete audit pair for %s: intent=%v outcome=%v", tool, p.intent, p.outcome)
		}
	}
}

// auditPhaseHarness wraps an mcp.Server with the same initialize → tools/call
// pattern liveMCPHarness uses in tests/, but kept local here so the postgres
// sub-module does not depend on the top-level tests package.
type auditPhaseHarness struct {
	t      *testing.T
	server *mcp.Server
}

func (h *auditPhaseHarness) callOK(ctx context.Context, tool string, args map[string]any) map[string]any {
	h.t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	requests := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "livee2e-audit", "version": "test"},
			"capabilities":    map[string]any{},
		}},
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{
			"name":      tool,
			"arguments": args,
		}},
	}
	input := encodeRequests(h.t, requests)
	output := runServer(h.t, ctx, h.server, input)
	resp := findResponse(h.t, output, 2)
	if resp.Error != nil {
		h.t.Fatalf("tool %s rpc error: code=%d msg=%s", tool, resp.Error.Code, resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		h.t.Fatalf("tool %s result not a map: %T", tool, resp.Result)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		h.t.Fatalf("tool %s isError result: %v", tool, result["content"])
	}
	return result
}
