package enforcement

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// handlerRecorder returns a ToolHandler that records whether it was called
// and returns the given result/error.
func handlerRecorder(result any, err error) (mcp.ToolHandler, *atomic.Int32) {
	var calls atomic.Int32
	return func(_ context.Context, _ map[string]any) (any, error) {
		calls.Add(1)
		return result, err
	}, &calls
}

// lookupWith returns a lookupHandler function backed by a fixed map.
func lookupWith(handlers map[string]mcp.ToolHandler) func(string) (mcp.ToolHandler, bool) {
	return func(name string) (mcp.ToolHandler, bool) {
		h, ok := handlers[name]
		return h, ok
	}
}

// noLookup is a lookupHandler that never finds anything.
func noLookup(string) (mcp.ToolHandler, bool) { return nil, false }

// fullTier1Bootstrap creates a bootstrap.Config in FullTier1 mode with the
// given tool names registered as Tier 1.
func fullTier1Bootstrap(names ...string) *bootstrap.Config {
	tier1 := make(map[string]bool, len(names))
	for _, n := range names {
		tier1[n] = true
	}
	cfg := &bootstrap.Config{Mode: bootstrap.FullTier1, Tier1Names: tier1}
	return cfg
}

// minimalBootstrap creates a bootstrap.Config in Minimal mode.
func minimalBootstrap() *bootstrap.Config {
	return &bootstrap.Config{Mode: bootstrap.Minimal}
}

// readOnlyPolicy returns a policy in read_only mode.
func readOnlyPolicy() *policy.Policy {
	return &policy.Policy{Mode: policy.ReadOnly, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
}

// standardPolicy returns a policy in standard mode.
func standardPolicy() *policy.Policy {
	return &policy.Policy{Mode: policy.Standard, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
}

// denyToolPolicy returns a standard-mode policy that explicitly denies the given tool.
func denyToolPolicy(name string) *policy.Policy {
	return &policy.Policy{
		Mode:         policy.Standard,
		DeniedTools:  map[string]bool{name: true},
		DeniedGroups: map[string]bool{},
	}
}

// denyGroupPolicy returns a standard-mode policy that explicitly denies the given group.
func denyGroupPolicy(group string) *policy.Policy {
	return &policy.Policy{
		Mode:         policy.Standard,
		DeniedTools:  map[string]bool{},
		DeniedGroups: map[string]bool{group: true},
	}
}

// ---------------------------------------------------------------------------
// Pipeline.FilterTool
// ---------------------------------------------------------------------------

func TestFilterTool_VisibleAndAllowed(t *testing.T) {
	p := &Pipeline{
		Bootstrap: fullTier1Bootstrap("clockify_list_entries"),
		Policy:    standardPolicy(),
	}
	hints := mcp.ToolHints{ReadOnly: true}
	if !p.FilterTool("clockify_list_entries", hints) {
		t.Fatal("expected tool to be visible and allowed")
	}
}

func TestFilterTool_HiddenByBootstrap(t *testing.T) {
	// Bootstrap only knows about clockify_whoami; "clockify_list_entries" is not Tier 1.
	p := &Pipeline{
		Bootstrap: fullTier1Bootstrap("clockify_whoami"),
		Policy:    standardPolicy(),
	}
	hints := mcp.ToolHints{ReadOnly: true}
	if p.FilterTool("clockify_list_entries", hints) {
		t.Fatal("expected tool to be hidden by bootstrap")
	}
}

func TestFilterTool_BlockedByPolicy(t *testing.T) {
	p := &Pipeline{
		Bootstrap: fullTier1Bootstrap("clockify_list_entries"),
		Policy:    denyToolPolicy("clockify_list_entries"),
	}
	hints := mcp.ToolHints{ReadOnly: true}
	if p.FilterTool("clockify_list_entries", hints) {
		t.Fatal("expected tool to be blocked by policy deny list")
	}
}

func TestFilterTool_ReadOnlyPolicyBlocksWriteTool(t *testing.T) {
	p := &Pipeline{
		Bootstrap: fullTier1Bootstrap("clockify_add_entry"),
		Policy:    readOnlyPolicy(),
	}
	hints := mcp.ToolHints{ReadOnly: false}
	if p.FilterTool("clockify_add_entry", hints) {
		t.Fatal("expected write tool to be blocked by read_only policy")
	}
}

func TestFilterTool_NilBootstrap(t *testing.T) {
	p := &Pipeline{
		Bootstrap: nil,
		Policy:    standardPolicy(),
	}
	hints := mcp.ToolHints{ReadOnly: true}
	if !p.FilterTool("anything", hints) {
		t.Fatal("expected tool to pass when bootstrap is nil")
	}
}

func TestFilterTool_NilPolicy(t *testing.T) {
	p := &Pipeline{
		Bootstrap: fullTier1Bootstrap("clockify_list_entries"),
		Policy:    nil,
	}
	hints := mcp.ToolHints{ReadOnly: false}
	if !p.FilterTool("clockify_list_entries", hints) {
		t.Fatal("expected tool to pass when policy is nil")
	}
}

func TestFilterTool_BothNil(t *testing.T) {
	p := &Pipeline{}
	hints := mcp.ToolHints{ReadOnly: false}
	if !p.FilterTool("any_tool", hints) {
		t.Fatal("expected tool to pass when both bootstrap and policy are nil")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — policy blocking
// ---------------------------------------------------------------------------

func TestBeforeCall_PolicyBlocks(t *testing.T) {
	p := &Pipeline{
		Policy: readOnlyPolicy(),
	}
	hints := mcp.ToolHints{ReadOnly: false}
	_, _, err := p.BeforeCall(context.Background(), "clockify_add_entry", map[string]any{}, hints, noLookup)
	if err == nil {
		t.Fatal("expected error from policy block")
	}
	if !strings.Contains(err.Error(), "tool blocked by policy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBeforeCall_PolicyDenyTool(t *testing.T) {
	p := &Pipeline{
		Policy: denyToolPolicy("clockify_list_entries"),
	}
	hints := mcp.ToolHints{ReadOnly: true}
	_, _, err := p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err == nil {
		t.Fatal("expected error from explicit deny")
	}
	if !strings.Contains(err.Error(), "explicitly denied") {
		t.Fatalf("expected 'explicitly denied' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — rate limiting
// ---------------------------------------------------------------------------

func TestBeforeCall_RateLimitExhausted(t *testing.T) {
	// maxConcurrent=1, maxPerWindow=1000 (high), window=60s
	rl := ratelimit.New(1, 1000, 60000)
	// Acquire the one available slot.
	release1, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}
	defer release1()

	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
	}
	hints := mcp.ToolHints{ReadOnly: true}
	_, _, err = p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBeforeCall_RateLimitWindowExhausted(t *testing.T) {
	// maxConcurrent=0 (disabled), maxPerWindow=1, window=60s
	rl := ratelimit.New(0, 1, 60000)
	// Use the one allowed call in this window.
	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire should succeed: %v", err)
	}
	rel()

	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
	}
	hints := mcp.ToolHints{ReadOnly: true}
	_, _, err = p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err == nil {
		t.Fatal("expected rate limit error for window exhaustion")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBeforeCall_NilRateLimit(t *testing.T) {
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: nil,
	}
	hints := mcp.ToolHints{ReadOnly: true}
	result, release, err := p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for normal call")
	}
	if release != nil {
		t.Fatal("expected nil release when rate limiter is nil")
	}
}

func TestBeforeCall_ReleaseFunction(t *testing.T) {
	rl := ratelimit.New(2, 1000, 60000)
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
	}
	hints := mcp.ToolHints{ReadOnly: true}
	_, release, err := p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release == nil {
		t.Fatal("expected non-nil release function")
	}
	// Calling release should not panic.
	release()
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — nil policy (no policy check)
// ---------------------------------------------------------------------------

func TestBeforeCall_NilPolicy(t *testing.T) {
	p := &Pipeline{Policy: nil}
	hints := mcp.ToolHints{ReadOnly: false}
	result, _, err := p.BeforeCall(context.Background(), "clockify_add_entry", map[string]any{}, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error with nil policy: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for normal pass-through")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — normal path (no dry-run, no block)
// ---------------------------------------------------------------------------

func TestBeforeCall_NormalPassThrough(t *testing.T) {
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: ratelimit.New(10, 1000, 60000),
		DryRun:    dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: true}
	// No dry_run in args, so no interception.
	result, release, err := p.BeforeCall(context.Background(), "clockify_list_entries", map[string]any{}, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for normal call")
	}
	if release == nil {
		t.Fatal("expected non-nil release from rate limiter")
	}
	release()
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: non-destructive write tool passes through
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_NonDestructivePassThrough(t *testing.T) {
	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	// Non-destructive write tool with dry_run=true: enforcement passes through,
	// leaving the flag in args so the handler's own dry-run logic can run.
	hints := mcp.ToolHints{ReadOnly: false, Destructive: false}
	args := map[string]any{"dry_run": true, "description": "test"}
	result, _, err := p.BeforeCall(context.Background(), "clockify_add_entry", args, hints, noLookup)
	if err != nil {
		t.Fatalf("expected no error for non-destructive tool pass-through, got: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result (pass through to handler)")
	}
	// dry_run flag must remain in args for the handler.
	if _, exists := args["dry_run"]; !exists {
		t.Fatal("expected dry_run key to remain in args for handler-level dry-run")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run disabled via CLOCKIFY_DRY_RUN=off
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRunDisabled_SkipsIntercept(t *testing.T) {
	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: false},
	}
	// Destructive tool with dry_run=true, but enforcement dry-run is disabled.
	// The flag passes through to the handler (not consumed by enforcement).
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "entry_id": "e1"}
	result, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, noLookup)
	if err != nil {
		t.Fatalf("expected no error when dry-run is disabled, got: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result (pass through to handler)")
	}
	// dry_run flag must remain in args since enforcement didn't consume it.
	if _, exists := args["dry_run"]; !exists {
		t.Fatal("expected dry_run key to remain in args when enforcement dry-run is disabled")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: MinimalFallback
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_MinimalFallback(t *testing.T) {
	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	// Use a tool in the minimalTools map.
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "group_id": "g1"}
	result, release, err := p.BeforeCall(context.Background(), "clockify_delete_holiday", args, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from minimal fallback dry-run")
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["dry_run"] != true {
		t.Fatal("expected dry_run=true in result")
	}
	if m["tool"] != "clockify_delete_holiday" {
		t.Fatalf("expected tool=clockify_delete_holiday, got %v", m["tool"])
	}
	if release != nil {
		release()
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: destructive tool falls to MinimalFallback
// (clockify_send_invoice is no longer in confirmTools; it's a toolRW tool
// that handles dry-run at the handler level. This test uses Destructive: true
// to exercise the enforcement fallback path.)
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_DestructiveFallsToMinimal(t *testing.T) {
	handler, calls := handlerRecorder(map[string]any{"id": "inv1", "status": "sent"}, nil)
	lookup := lookupWith(map[string]mcp.ToolHandler{
		"clockify_send_invoice": handler,
	})

	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "invoice_id": "inv1"}

	result, release, err := p.BeforeCall(context.Background(), "clockify_send_invoice", args, hints, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Handler should NOT be called — MinimalFallback doesn't invoke the handler.
	if calls.Load() != 0 {
		t.Fatalf("expected handler NOT called for minimal fallback, got %d calls", calls.Load())
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["dry_run"] != true {
		t.Fatal("expected dry_run=true in minimal result")
	}
	// Minimal result has resource=nil, not a "preview" key.
	if _, hasPrev := m["preview"]; hasPrev {
		t.Fatal("expected minimal result without preview key")
	}
	if release != nil {
		release()
	}
	_ = lookup
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: PreviewTool (happy path)
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_PreviewTool(t *testing.T) {
	handler, calls := handlerRecorder(map[string]any{"id": "e1", "description": "test entry"}, nil)
	lookup := lookupWith(map[string]mcp.ToolHandler{
		"clockify_get_entry": handler,
	})

	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	// clockify_delete_entry → preview tool is clockify_get_entry
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "entry_id": "e1"}

	result, release, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected preview handler called once, got %d", calls.Load())
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["dry_run"] != true {
		t.Fatal("expected dry_run=true in wrapped result")
	}
	if m["preview"] == nil {
		t.Fatal("expected preview field in wrapped result")
	}
	if m["tool"] != "clockify_delete_entry" {
		t.Fatalf("expected tool=clockify_delete_entry, got %v", m["tool"])
	}
	if release != nil {
		release()
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: PreviewTool (preview handler NOT found → MinimalResult)
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_PreviewTool_HandlerNotFound(t *testing.T) {
	// Lookup exists but does NOT have "clockify_get_entry" registered.
	lookup := lookupWith(map[string]mcp.ToolHandler{})

	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "entry_id": "e1"}

	result, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if _, hasPrev := m["preview"]; hasPrev {
		t.Fatal("expected minimal result without preview key")
	}
	_ = lookup // suppress unused
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run: PreviewTool handler returns error
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_PreviewTool_HandlerError(t *testing.T) {
	handler, _ := handlerRecorder(nil, errors.New("not found"))
	lookup := lookupWith(map[string]mcp.ToolHandler{
		"clockify_get_entry": handler,
	})

	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "entry_id": "e1"}

	_, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, lookup)
	if err == nil {
		t.Fatal("expected error when preview handler fails")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run releases rate limiter slot on error
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_ReleasesOnError(t *testing.T) {
	rl := ratelimit.New(1, 1000, 60000)
	// PreviewTool handler that returns an error.
	handler, _ := handlerRecorder(nil, errors.New("not found"))
	lookup := lookupWith(map[string]mcp.ToolHandler{
		"clockify_get_entry": handler,
	})

	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
		DryRun:    dryrun.Config{Enabled: true},
	}
	// Destructive tool with dry_run → PreviewTool path → handler error.
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "entry_id": "e1"}

	_, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, lookup)
	if err == nil {
		t.Fatal("expected error from preview handler")
	}

	// The semaphore slot should have been released. Verify by acquiring again.
	rel, err := rl.Acquire(context.Background())
	if err != nil {
		t.Fatalf("slot should have been released after dry-run error, but Acquire failed: %v", err)
	}
	rel()
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run returns result WITH release function
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_ReturnsRelease(t *testing.T) {
	rl := ratelimit.New(2, 1000, 60000)
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
		DryRun:    dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "group_id": "g1"}

	result, release, err := p.BeforeCall(context.Background(), "clockify_delete_holiday", args, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected dry-run result")
	}
	if release == nil {
		t.Fatal("expected non-nil release function from rate-limited dry-run")
	}
	release()
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run NOT triggered when dry_run flag absent
// ---------------------------------------------------------------------------

func TestBeforeCall_NoDryRunFlag(t *testing.T) {
	handler, calls := handlerRecorder(nil, nil)
	lookup := lookupWith(map[string]mcp.ToolHandler{
		"clockify_delete_entry": handler,
	})

	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	// No dry_run key in args.
	args := map[string]any{"entry_id": "e1"}

	result, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result when dry_run is not in args")
	}
	if calls.Load() != 0 {
		t.Fatal("handler should not have been called by BeforeCall without dry_run")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — dry-run with dry_run=false (not active)
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRunFalse(t *testing.T) {
	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": false}

	result, _, err := p.BeforeCall(context.Background(), "clockify_delete_entry", args, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result when dry_run=false")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — default case in executeDryRun (unknown action code)
// ---------------------------------------------------------------------------

func TestBeforeCall_DryRun_DefaultDestructiveMinimal(t *testing.T) {
	// Use a destructive tool name that is NOT in confirmTools, previewMap, or
	// minimalTools. CheckDryRun returns MinimalFallback for any other destructive tool.
	p := &Pipeline{
		Policy: standardPolicy(),
		DryRun: dryrun.Config{Enabled: true},
	}
	hints := mcp.ToolHints{ReadOnly: false, Destructive: true}
	args := map[string]any{"dry_run": true, "id": "x"}

	result, _, err := p.BeforeCall(context.Background(), "clockify_archive_project", args, hints, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["dry_run"] != true {
		t.Fatal("expected dry_run in minimal result")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.AfterCall
// ---------------------------------------------------------------------------

func TestAfterCall_TruncationEnabled_LargeResult(t *testing.T) {
	p := &Pipeline{
		Truncation: truncate.Config{Enabled: true, TokenBudget: 50},
	}
	// Build a result that exceeds 50 tokens (~200 bytes of JSON).
	bigData := make([]any, 100)
	for i := range bigData {
		bigData[i] = map[string]any{"entry": strings.Repeat("x", 40)}
	}
	input := map[string]any{"data": bigData}

	result, err := p.AfterCall(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if _, hasTrunc := m["_truncation"]; !hasTrunc {
		t.Fatal("expected _truncation metadata in truncated result")
	}
}

func TestAfterCall_TruncationDisabled(t *testing.T) {
	p := &Pipeline{
		Truncation: truncate.Config{Enabled: false, TokenBudget: 0},
	}
	input := map[string]any{"key": "value"}
	result, err := p.AfterCall(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if _, hasTrunc := m["_truncation"]; hasTrunc {
		t.Fatal("did not expect truncation metadata when truncation is disabled")
	}
}

func TestAfterCall_TruncationEnabled_SmallResult(t *testing.T) {
	p := &Pipeline{
		Truncation: truncate.Config{Enabled: true, TokenBudget: 8000},
	}
	input := map[string]any{"ok": true, "data": "small"}
	result, err := p.AfterCall(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if _, hasTrunc := m["_truncation"]; hasTrunc {
		t.Fatal("did not expect truncation metadata for small result within budget")
	}
}

// ---------------------------------------------------------------------------
// Gate.IsGroupAllowed
// ---------------------------------------------------------------------------

func TestGate_IsGroupAllowed_PolicyAllows(t *testing.T) {
	g := &Gate{
		Policy: standardPolicy(),
	}
	if !g.IsGroupAllowed("invoices") {
		t.Fatal("expected group to be allowed under standard policy")
	}
}

func TestGate_IsGroupAllowed_PolicyDenies(t *testing.T) {
	g := &Gate{
		Policy: denyGroupPolicy("invoices"),
	}
	if g.IsGroupAllowed("invoices") {
		t.Fatal("expected group to be denied")
	}
}

func TestGate_IsGroupAllowed_ReadOnlyPolicyDeniesAll(t *testing.T) {
	g := &Gate{
		Policy: readOnlyPolicy(),
	}
	if g.IsGroupAllowed("invoices") {
		t.Fatal("expected read_only policy to deny all groups")
	}
}

func TestGate_IsGroupAllowed_NilPolicy(t *testing.T) {
	g := &Gate{
		Policy: nil,
	}
	if !g.IsGroupAllowed("invoices") {
		t.Fatal("expected group to be allowed when policy is nil")
	}
}

// ---------------------------------------------------------------------------
// Gate.OnActivate
// ---------------------------------------------------------------------------

func TestGate_OnActivate_WithBootstrap(t *testing.T) {
	bs := minimalBootstrap()
	g := &Gate{
		Bootstrap: bs,
	}
	names := []string{"clockify_list_invoices", "clockify_get_invoice"}
	g.OnActivate(names)

	// Verify the tools are now visible via bootstrap.
	for _, name := range names {
		if !bs.IsVisible(name) {
			t.Fatalf("expected %s to be visible after OnActivate", name)
		}
	}
}

func TestGate_OnActivate_NilBootstrap(t *testing.T) {
	g := &Gate{
		Bootstrap: nil,
	}
	// Should not panic.
	g.OnActivate([]string{"clockify_list_invoices"})
}

func TestGate_OnActivate_EmptyList(t *testing.T) {
	bs := minimalBootstrap()
	g := &Gate{Bootstrap: bs}
	// Should not panic with empty slice.
	g.OnActivate([]string{})
}

// ---------------------------------------------------------------------------
// Integration-style: Pipeline + Gate together
// ---------------------------------------------------------------------------

func TestPipelineAndGate_Integration(t *testing.T) {
	tier1Names := map[string]bool{
		"clockify_list_entries": true,
		"clockify_add_entry":    true,
		"clockify_whoami":       true,
		"clockify_search_tools": true,
	}
	bs := &bootstrap.Config{
		Mode:       bootstrap.FullTier1,
		Tier1Names: tier1Names,
	}
	pol := standardPolicy()

	pipeline := &Pipeline{
		Policy:     pol,
		Bootstrap:  bs,
		RateLimit:  ratelimit.New(5, 100, 60000),
		DryRun:     dryrun.Config{Enabled: true},
		Truncation: truncate.Config{Enabled: true, TokenBudget: 8000},
	}
	gate := &Gate{
		Policy:    pol,
		Bootstrap: bs,
	}

	// 1. Tier 1 tool visible.
	if !pipeline.FilterTool("clockify_list_entries", mcp.ToolHints{ReadOnly: true}) {
		t.Fatal("expected Tier 1 tool to be visible")
	}

	// 2. Tier 2 tool not visible before activation.
	if pipeline.FilterTool("clockify_list_invoices", mcp.ToolHints{ReadOnly: true}) {
		t.Fatal("expected Tier 2 tool to NOT be visible before activation")
	}

	// 3. Gate allows group.
	if !gate.IsGroupAllowed("invoices") {
		t.Fatal("expected invoices group to be allowed")
	}

	// 4. Activate Tier 2 tools.
	gate.OnActivate([]string{"clockify_list_invoices", "clockify_get_invoice"})

	// 5. Now Tier 2 tool IS visible (bootstrap marks it active).
	if !pipeline.FilterTool("clockify_list_invoices", mcp.ToolHints{ReadOnly: true}) {
		t.Fatal("expected Tier 2 tool to be visible after activation")
	}

	// 6. BeforeCall on read tool passes through.
	result, release, err := pipeline.BeforeCall(
		context.Background(),
		"clockify_list_entries",
		map[string]any{},
		mcp.ToolHints{ReadOnly: true},
		noLookup,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for normal call")
	}
	if release != nil {
		release()
	}

	// 7. AfterCall with small result unchanged.
	out, err := pipeline.AfterCall(map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	if _, hasTrunc := m["_truncation"]; hasTrunc {
		t.Fatal("small result should not be truncated")
	}
}

// ---------------------------------------------------------------------------
// Pipeline.BeforeCall — verify it implements mcp.Enforcement interface
// ---------------------------------------------------------------------------

func TestPipeline_ImplementsEnforcement(t *testing.T) {
	var _ mcp.Enforcement = (*Pipeline)(nil)
}

func TestGate_ImplementsActivator(t *testing.T) {
	var _ mcp.Activator = (*Gate)(nil)
}
