//go:build livee2e

package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/tools"
	"github.com/apet97/go-clockify/internal/truncate"
)

// liveMCPHarness drives MCP-protocol calls (initialize → tools/call) against
// a real Clockify backend through the production enforcement pipeline. The
// pre-existing direct-handler harness (setupTestEnv / invokeTool) bypasses
// policy, dry-run interception, and audit emission — exactly the safety
// contracts the launch-blocking live tests need to assert. This harness
// reuses the same Pipeline + Gate wiring that internal/runtime/service.go
// uses in production so a regression in either layer surfaces here.
type liveMCPHarness struct {
	t        *testing.T
	cfg      config.Config
	Service  *tools.Service
	Server   *mcp.Server
	Auditor  *capturingAuditor
	Pipeline *enforcement.Pipeline
}

type liveMCPOptions struct {
	// PolicyMode selects the CLOCKIFY_POLICY mode applied to the pipeline.
	// Empty string defaults to policy.Standard so the harness behaves like
	// the historical setupTestEnv when callers omit this knob.
	PolicyMode policy.Mode
	// DryRunEnabled toggles the enforcement-layer dry-run intercept.
	// Destructive tools called with dry_run:true while this is true must be
	// short-circuited by the pipeline; the handler must not run.
	DryRunEnabled bool
	// AuditDurabilityMode mirrors MCP_AUDIT_DURABILITY. "fail_closed" makes
	// recordAuditIntent() abort the call when audit persistence fails.
	AuditDurabilityMode string
	// SessionID lets the caller correlate audit rows to a specific test run.
	// Empty value falls back to a synthesised "live-mcp-<unix-nano>" id.
	SessionID string
}

// capturingAuditor records every emitted mcp.AuditEvent in memory so a test
// can inspect intent/outcome ordering, phases, and outcomes without going
// near the control-plane store. Real Postgres-backed audit-row inspection
// lives under internal/controlplane/postgres/ where pgx is in scope.
type capturingAuditor struct {
	mu     sync.Mutex
	events []mcp.AuditEvent
}

func (c *capturingAuditor) RecordAudit(ev mcp.AuditEvent) error {
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
	return nil
}

func (c *capturingAuditor) snapshot() []mcp.AuditEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]mcp.AuditEvent, len(c.events))
	copy(out, c.events)
	return out
}

func (c *capturingAuditor) reset() {
	c.mu.Lock()
	c.events = nil
	c.mu.Unlock()
}

// setupLiveMCPHarness builds a production-shaped enforcement pipeline +
// MCP server backed by a real Clockify client. Honours the same env gates
// as setupTestEnv (CLOCKIFY_API_KEY + CLOCKIFY_RUN_LIVE_E2E=1) and pins
// CLOCKIFY_DRY_RUN=off so the harness fully owns the dry-run knob.
func setupLiveMCPHarness(t *testing.T, opts liveMCPOptions) *liveMCPHarness {
	t.Helper()
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live MCP-path tests since CLOCKIFY_API_KEY is not set")
	}
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("Skipping live MCP-path tests unless CLOCKIFY_RUN_LIVE_E2E=1")
	}
	t.Setenv("CLOCKIFY_DRY_RUN", "off")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	client := clockify.NewClient(cfg.APIKey, cfg.BaseURL, cfg.RequestTimeout, cfg.MaxRetries)
	t.Cleanup(client.Close)

	service := tools.New(client, cfg.WorkspaceID)

	mode := opts.PolicyMode
	if mode == "" {
		mode = policy.Standard
	}
	pol := &policy.Policy{Mode: mode}
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
		DryRun:     dryrun.Config{Enabled: opts.DryRunEnabled},
		Truncation: truncate.Config{},
	}
	gate := &enforcement.Gate{Policy: pol, Bootstrap: bc}

	auditor := &capturingAuditor{}
	server := mcp.NewServer("livee2e", registry, pipeline, gate)
	server.MaxMessageSize = 8 * 1024 * 1024
	server.Auditor = auditor
	server.AuditDurabilityMode = opts.AuditDurabilityMode
	server.AuditTenantID = "live-mcp"
	server.AuditSubject = "live-mcp"
	server.AuditTransport = "livee2e"
	if opts.SessionID == "" {
		server.AuditSessionID = fmt.Sprintf("live-mcp-%d", time.Now().UnixNano())
	} else {
		server.AuditSessionID = opts.SessionID
	}

	return &liveMCPHarness{
		t:        t,
		cfg:      cfg,
		Service:  service,
		Server:   server,
		Auditor:  auditor,
		Pipeline: pipeline,
	}
}

// rawCall sends initialize → notifications/initialized → tools/call through
// the MCP server's Run loop and returns the parsed tools/call response. Each
// invocation creates a fresh Run; the underlying server, auditor, policy,
// and clockify client are shared so audit captures and policy state persist
// across calls.
func (h *liveMCPHarness) rawCall(ctx context.Context, tool string, args map[string]any) (mcp.Response, error) {
	h.t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "livee2e", "version": "test"},
			"capabilities":    map[string]any{},
		},
	}
	initNote := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      tool,
			"arguments": args,
		},
	}

	var input bytes.Buffer
	enc := json.NewEncoder(&input)
	for _, r := range []map[string]any{initReq, initNote, callReq} {
		if err := enc.Encode(r); err != nil {
			return mcp.Response{}, fmt.Errorf("encode request: %w", err)
		}
	}
	var output bytes.Buffer
	if err := h.Server.Run(ctx, &input, &output); err != nil {
		return mcp.Response{}, fmt.Errorf("server.Run: %w", err)
	}

	scanner := bufio.NewScanner(&output)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var resp mcp.Response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		// tools/call always carries id == 2 in this harness.
		if id, ok := resp.ID.(float64); ok && id == 2 {
			return resp, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return mcp.Response{}, fmt.Errorf("scan output: %w", err)
	}
	return mcp.Response{}, fmt.Errorf("no tools/call response in output: %q", output.String())
}

// callOK runs rawCall and returns the unwrapped result envelope. Fails the
// test on a JSON-RPC error or a content+isError tool error response.
func (h *liveMCPHarness) callOK(ctx context.Context, tool string, args map[string]any) map[string]any {
	h.t.Helper()
	resp, err := h.rawCall(ctx, tool, args)
	if err != nil {
		h.t.Fatalf("rawCall %s: %v", tool, err)
	}
	if resp.Error != nil {
		h.t.Fatalf("tool %s returned RPC error: code=%d msg=%s", tool, resp.Error.Code, resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		h.t.Fatalf("tool %s result was not a map: %T", tool, resp.Result)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		h.t.Fatalf("tool %s returned isError result: %v", tool, result["content"])
	}
	return result
}

// callExpectError runs rawCall expecting either a non-nil RPC error or a
// content+isError tool result. Returns a single concatenated error string
// suitable for substring assertions.
func (h *liveMCPHarness) callExpectError(ctx context.Context, tool string, args map[string]any) string {
	h.t.Helper()
	resp, err := h.rawCall(ctx, tool, args)
	if err != nil {
		h.t.Fatalf("rawCall %s: %v", tool, err)
	}
	if resp.Error != nil {
		return resp.Error.Message
	}
	if result, ok := resp.Result.(map[string]any); ok {
		if isErr, _ := result["isError"].(bool); isErr {
			content, _ := result["content"].([]any)
			var b strings.Builder
			for _, item := range content {
				if entry, ok := item.(map[string]any); ok {
					if text, ok := entry["text"].(string); ok {
						b.WriteString(text)
					}
				}
			}
			return b.String()
		}
	}
	h.t.Fatalf("tool %s expected an error response but got success: %#v", tool, resp.Result)
	return ""
}

// extractDataMap pulls the structuredContent.data field out of a tools/call
// envelope. Tier-1 tools dual-emit as text + structuredContent (see
// internal/mcp/server.go); the structured payload is the canonical machine-
// readable form.
func extractDataMap(t *testing.T, result map[string]any) map[string]any {
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

// listProjectsRaw fetches the workspace projects bypassing the MCP path so
// the test can verify the actual upstream state independently of the path
// it just exercised.
func (h *liveMCPHarness) listProjectsRaw(ctx context.Context) []clockify.Project {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		h.t.Fatalf("resolve workspace: %v", err)
	}
	var projects []clockify.Project
	if err := h.Service.Client.Get(ctx, "/workspaces/"+wsID+"/projects", nil, &projects); err != nil {
		h.t.Fatalf("list projects: %v", err)
	}
	return projects
}

// getEntryRaw fetches a single time entry directly through the Clockify
// client so the test can confirm the dry-run path did not delete it.
func (h *liveMCPHarness) getEntryRaw(ctx context.Context, entryID string) (clockify.TimeEntry, error) {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return clockify.TimeEntry{}, err
	}
	var entry clockify.TimeEntry
	if err := h.Service.Client.Get(ctx, "/workspaces/"+wsID+"/time-entries/"+entryID, nil, &entry); err != nil {
		return clockify.TimeEntry{}, err
	}
	return entry, nil
}

// deleteEntryRaw removes a time entry directly. Used in cleanup paths so a
// test failure doesn't leak entities into the sacrificial workspace.
func (h *liveMCPHarness) deleteEntryRaw(ctx context.Context, entryID string) error {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/time-entries/"+entryID)
}

// deleteProjectRaw removes a project directly for cleanup.
func (h *liveMCPHarness) deleteProjectRaw(ctx context.Context, projectID string) error {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/projects/"+projectID)
}

// deleteClientRaw removes a client directly for cleanup.
func (h *liveMCPHarness) deleteClientRaw(ctx context.Context, clientID string) error {
	h.t.Helper()
	wsID, err := h.Service.ResolveWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return h.Service.Client.Delete(ctx, "/workspaces/"+wsID+"/clients/"+clientID)
}

// requireWriteEnabled gates destructive live tests on the same repo
// variable the existing TestE2EMutating workflow uses. Hosting either the
// dry-run or audit-phase test on the read-only matrix would still create
// real entities (the dry-run setup needs a real entry to preview against).
func requireWriteEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("CLOCKIFY_LIVE_WRITE_ENABLED") != "true" {
		t.Skip("Skipping live mutating test unless CLOCKIFY_LIVE_WRITE_ENABLED=true")
	}
}

// ---------------------------------------------------------------------------
// TestLiveDryRunDoesNotMutate
// ---------------------------------------------------------------------------
//
// Why this test exists: the enforcement-layer dry-run intercept is the safety
// guarantee that a destructive tool called with dry_run:true previews via the
// matching GET handler instead of executing the destructive handler. A
// regression that lets the destructive handler run anyway is invisible to
// clients (the dry-run envelope still gets returned) but silently destroys
// data — exactly the failure mode this test catches.
//
// The flow:
//  1. Create a real entry through the MCP path so the destructive target
//     exists in the sacrificial workspace.
//  2. Issue clockify_delete_entry with dry_run:true through the MCP server.
//  3. Assert the response carries the dry-run envelope (dry_run:true,
//     preview field populated by the GET counterpart).
//  4. Read the entry back through the raw Clockify client and assert it
//     still exists.
//  5. Clean up by actually deleting it (cleanup is best-effort but logs
//     loudly so leaked sacrificial-workspace entities don't accumulate).
func TestLiveDryRunDoesNotMutate(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{
		PolicyMode:    policy.Standard,
		DryRunEnabled: true,
		SessionID:     fmt.Sprintf("live-dryrun-%d", time.Now().UnixNano()),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a transient entry through the MCP path so the test exercises
	// the same plumbing the dry-run target will use moments later.
	prefix := fmt.Sprintf("AG_DRYRUN_%d", time.Now().UnixNano())
	start := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	addResult := h.callOK(ctx, "clockify_add_entry", map[string]any{
		"description": prefix + "_entry",
		"start":       start,
		"end":         end,
	})
	addData := extractDataMap(t, addResult)
	entryID, _ := addData["id"].(string)
	if entryID == "" {
		t.Fatalf("clockify_add_entry returned no entry id: %#v", addData)
	}
	t.Logf("created sacrificial entry: %s", entryID)
	t.Cleanup(func() {
		// Best-effort cleanup so a failed assertion doesn't leak.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.deleteEntryRaw(cleanupCtx, entryID); err != nil {
			t.Logf("cleanup delete %s failed (entry may have already been deleted): %v", entryID, err)
		}
	})

	// Reset audit captures so we only see events from the dry-run call.
	h.Auditor.reset()

	// The dry_run path is the actual contract under test.
	dryResp := h.callOK(ctx, "clockify_delete_entry", map[string]any{
		"entry_id": entryID,
		"dry_run":  true,
	})
	// Tool envelope shape: structuredContent.data should carry dry_run:true
	// and a preview field populated by the GET counterpart. dryrun.WrapResult
	// also lives at result-envelope level for compat with older clients.
	dryData := extractDataMap(t, dryResp)
	if v, _ := dryData["dry_run"].(bool); !v {
		t.Fatalf("dry-run delete envelope missing dry_run:true: %#v", dryData)
	}
	if dryData["preview"] == nil {
		t.Fatalf("dry-run delete envelope missing preview field: %#v", dryData)
	}

	// Audit invariant: the destructive handler must NOT have produced any
	// outcome row for this entry id. The dry-run intercept emits a single
	// best-effort record with outcome="dry_run" and phase="" (see
	// internal/mcp/tools.go). A regression that lets the destructive
	// handler run would surface as an additional intent + outcome pair
	// here.
	for _, ev := range h.Auditor.snapshot() {
		if ev.Phase == mcp.PhaseIntent || ev.Phase == mcp.PhaseOutcome {
			t.Fatalf("dry-run delete emitted phased audit record %q for tool %s — interception was bypassed", ev.Phase, ev.Tool)
		}
	}

	// Cross-check via the raw client that the entry still exists upstream.
	if _, err := h.getEntryRaw(ctx, entryID); err != nil {
		t.Fatalf("entry %s missing after dry-run delete (interception failed): %v", entryID, err)
	}
}

// ---------------------------------------------------------------------------
// TestLivePolicyTimeTrackingSafeBlocksProjectCreate
// ---------------------------------------------------------------------------
//
// Why this test exists: time_tracking_safe is the AI-facing default for the
// shared-service profile (see docs/deploy/production-profile-shared-service.md).
// A drift that leaks workspace-wide writes through under that policy fails
// the very promise the profile was added to make. The pipeline must reject
// clockify_create_project before the handler runs; the handler sitting
// behind the policy gate must never see the call.
//
// Note: this test goes through the MCP enforcement pipeline. Direct raw-
// handler invocation bypasses the policy gate entirely and would actually
// create the project, which is exactly the kind of regression this test
// is meant to detect.
func TestLivePolicyTimeTrackingSafeBlocksProjectCreate(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{
		PolicyMode: policy.TimeTrackingSafe,
		SessionID:  fmt.Sprintf("live-policy-%d", time.Now().UnixNano()),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("AG_POLICY_%d_project", time.Now().UnixNano())
	t.Cleanup(func() {
		// If the policy gate failed, the project may exist. Sweep it.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, p := range h.listProjectsRaw(cleanupCtx) {
			if p.Name == uniqueName {
				if err := h.deleteProjectRaw(cleanupCtx, p.ID); err != nil {
					t.Logf("cleanup delete project %s failed: %v", p.ID, err)
				}
			}
		}
	})

	errMsg := h.callExpectError(ctx, "clockify_create_project", map[string]any{
		"name": uniqueName,
	})
	if !strings.Contains(strings.ToLower(errMsg), "blocked by policy") &&
		!strings.Contains(strings.ToLower(errMsg), "time_tracking_safe") {
		t.Fatalf("expected policy-deny error mentioning blocked by policy or time_tracking_safe, got: %q", errMsg)
	}
	t.Logf("policy correctly blocked project create: %s", errMsg)

	// Independent verification via the raw upstream: no project with the
	// unique name should exist. If it does, the policy gate leaked.
	for _, p := range h.listProjectsRaw(ctx) {
		if p.Name == uniqueName {
			t.Fatalf("policy regression: project %q was created under time_tracking_safe (id=%s)", uniqueName, p.ID)
		}
	}
}
