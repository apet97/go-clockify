//go:build livee2e

package e2e_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// liveCampaignContext bundles per-test state shared across the
// sacrificial-workspace validation campaign: the workspace ID, the workspace
// owner's user identity (so admin tests never deactivate or reassign the
// account that's running the test), a per-run prefix so every entity we
// create is identifiable for orphan sweeps, and a LIFO cleanup registry.
//
// Tests get one of these from setupLiveCampaign(t, h) after the existing
// liveMCPHarness has been constructed. The campaign helper relies on the
// harness for protocol calls and raw client access; it does not duplicate
// that wiring.
type liveCampaignContext struct {
	t           *testing.T
	h           *liveMCPHarness
	WorkspaceID string
	OwnerUserID string
	OwnerEmail  string
	RunID       string

	cleanupMu    sync.Mutex
	cleanupSteps []cleanupStep
}

// cleanupStep is one deletion the test wants run on exit. Kind is a short
// human-readable tag so the orphan log can group failures (e.g. "client",
// "project", "expense-category") without forcing every caller to invent
// its own categorisation.
type cleanupStep struct {
	Kind   string
	ID     string
	Delete func(context.Context) error
}

// setupLiveCampaign performs the safety preconditions every campaign test
// must clear: the master surface gate, the workspace-confirmation
// second-factor check, and owner detection. Returns a context bound to
// the test's lifecycle. Cleanup runs in LIFO order at test exit; a single
// failing deleter is logged via t.Logf and does not abort the sweep.
//
// The caller is expected to have already obtained a liveMCPHarness via
// setupLiveMCPHarness (which handles the API-key + RUN_LIVE_E2E gates and
// calls MarkLiveTestRan for the skip-sentinel). This helper layers the
// campaign-specific gates on top of that foundation.
func setupLiveCampaign(t *testing.T, h *liveMCPHarness) *liveCampaignContext {
	t.Helper()
	if os.Getenv("CLOCKIFY_LIVE_FULL_SURFACE_ENABLED") != "true" {
		t.Skip("set CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true to run sacrificial-workspace campaign tests")
	}
	wsID := os.Getenv("CLOCKIFY_WORKSPACE_ID")
	if wsID == "" {
		t.Fatal("CLOCKIFY_WORKSPACE_ID must be set for live campaign tests")
	}
	confirm := os.Getenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM")
	if confirm != wsID {
		t.Fatalf("CLOCKIFY_LIVE_WORKSPACE_CONFIRM=%q does not match CLOCKIFY_WORKSPACE_ID=%q — refusing to run mutating tests against an unconfirmed workspace", confirm, wsID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// clockify_whoami returns IdentityData: user is the User struct,
	// workspaceId is the resolved workspace. Owner identity lives under
	// data.user.id, not flat at data.id.
	whoami := h.callOK(ctx, "clockify_whoami", nil)
	data := extractDataMap(t, whoami)
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("clockify_whoami response missing user object: %#v", data)
	}
	ownerID, _ := user["id"].(string)
	ownerEmail, _ := user["email"].(string)
	if ownerID == "" {
		t.Fatalf("clockify_whoami returned no user id; cannot identify workspace owner: %#v", data)
	}

	c := &liveCampaignContext{
		t:           t,
		h:           h,
		WorkspaceID: wsID,
		OwnerUserID: ownerID,
		OwnerEmail:  ownerEmail,
		RunID:       newCampaignRunID(),
	}
	t.Cleanup(func() { c.flushCleanups() })
	t.Logf("campaign run: workspace=%s owner=%s prefix=%s", wsID, ownerID, c.RunID)
	return c
}

// requireCategory skips the test unless the named category gate is set to
// "true". Category gates carve the campaign into bite-sized blast-radius
// envelopes the maintainer can opt into independently — e.g. running the
// admin-class tests without enabling external-email side effects.
func requireCategory(t *testing.T, gate string) {
	t.Helper()
	if os.Getenv(gate) != "true" {
		t.Skipf("set %s=true to run this category", gate)
	}
}

// newCampaignRunID returns the per-run identity all entities created by a
// single test invocation share. Format: mcp-live-<UTC-unix>-<6-byte-hex>.
// The unix-prefix gives sortability; the random suffix makes parallel
// campaigns from different machines or accidental concurrent runs safe to
// disambiguate by name.
func newCampaignRunID() string {
	var buf [3]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Test-only fallback. Loses collision protection but the run can
		// still proceed; a duplicate prefix would only matter if two
		// campaigns ran against the same workspace at the same second,
		// which is itself ill-advised.
		return fmt.Sprintf("mcp-live-%d-noend", time.Now().UTC().Unix())
	}
	return fmt.Sprintf("mcp-live-%d-%s", time.Now().UTC().Unix(), hex.EncodeToString(buf[:]))
}

// LivePrefix returns a deterministic, sortable, human-readable name for
// an entity of the given kind and per-test sequence number. Tests use
// this for every entity they create so an orphan sweep can find them by
// name pattern without relying on any internal Clockify metadata.
func (c *liveCampaignContext) LivePrefix(kind string, i int) string {
	return fmt.Sprintf("%s-%s-%d", c.RunID, kind, i)
}

// RegisterCleanup pushes a deletion step onto the registry. The closure
// runs at test exit time. The registry is a stack — callers create
// dependents (entries inside a project) after their parents (the project)
// so the LIFO sweep deletes in the correct dependency order.
func (c *liveCampaignContext) RegisterCleanup(kind, id string, deleter func(context.Context) error) {
	c.cleanupMu.Lock()
	c.cleanupSteps = append(c.cleanupSteps, cleanupStep{Kind: kind, ID: id, Delete: deleter})
	c.cleanupMu.Unlock()
}

// flushCleanups runs every registered deleter in LIFO order. Errors are
// logged via t.Logf — a transient API failure during cleanup must not
// mask the actual test failure that may have triggered it.
func (c *liveCampaignContext) flushCleanups() {
	c.cleanupMu.Lock()
	steps := append([]cleanupStep(nil), c.cleanupSteps...)
	c.cleanupMu.Unlock()

	for i := len(steps) - 1; i >= 0; i-- {
		st := steps[i]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := st.Delete(ctx)
		cancel()
		if err != nil {
			c.t.Logf("cleanup %s/%s failed (best-effort, may already be gone): %v", st.Kind, st.ID, err)
		}
	}
}

// activateTier2 brings a Tier-2 group's descriptors online on the
// underlying MCP server. Mirrors the production path
// (internal/runtime/service.go:113-128) without going through
// clockify_search_tools, because setupLiveMCPHarness intentionally does
// not wire Service.ActivateGroup. Idempotent: a duplicate activation
// surfaced by the server as "already" is treated as a no-op.
func (c *liveCampaignContext) activateTier2(group string) {
	c.t.Helper()
	descriptors, ok := c.h.Service.Tier2Handlers(group)
	if !ok {
		c.t.Fatalf("Tier 2 group %q is not registered (Tier2Handlers returned false)", group)
	}
	if err := c.h.Server.ActivateGroup(group, descriptors); err != nil {
		if !strings.Contains(err.Error(), "already") {
			c.t.Fatalf("activate Tier 2 group %q: %v", group, err)
		}
	}
}

// rawDeletePath performs a DELETE through the raw Clockify client,
// bypassing the MCP path. Cleanups use this to avoid re-triggering
// policy or dry-run interception that the test under test is asserting
// against.
func (c *liveCampaignContext) rawDeletePath(ctx context.Context, path string) error {
	return c.h.Service.Client.Delete(ctx, "/workspaces/"+c.WorkspaceID+path)
}

// rawGetPath performs a GET through the raw Clockify client. Used as an
// independent verification mechanism: a test asserts something through
// the MCP path, then re-reads via raw to confirm the upstream actually
// matches.
func (c *liveCampaignContext) rawGetPath(ctx context.Context, path string, out any) error {
	return c.h.Service.Client.Get(ctx, "/workspaces/"+c.WorkspaceID+path, nil, out)
}

// extractList pulls a slice-shaped envelope out of a tools/call result.
//
// Tool result shapes vary on the wire: Tier-1 list tools (list_projects,
// list_tags, etc.) put the slice directly at structuredContent.data;
// Tier-2 list tools sometimes wrap it (e.g. structuredContent.data.items
// or structuredContent.data.entries). When fields is empty the slice
// must be at data; otherwise the named fields are tried in order and
// the first match wins. Returns nil when no slice is found — callers
// treat that as "empty list", which is the correct semantics for an
// empty-but-valid sacrificial workspace.
func extractList(t *testing.T, result map[string]any, fields ...string) []any {
	t.Helper()
	sc, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("result missing structuredContent: %#v", result)
	}
	if list, ok := sc["data"].([]any); ok {
		return list
	}
	data, ok := sc["data"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent.data is neither a slice nor a map: %#v", sc)
	}
	for _, f := range fields {
		if v, ok := data[f].([]any); ok {
			return v
		}
	}
	return nil
}
