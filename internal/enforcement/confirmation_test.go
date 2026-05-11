package enforcement

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/confirmation"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
)

// newConfirmationSigner builds a Signer with a random secret and a
// short TTL for test determinism.
func newConfirmationSigner(t *testing.T) *confirmation.Signer {
	t.Helper()
	secret, err := confirmation.NewRandomSecret()
	if err != nil {
		t.Fatalf("NewRandomSecret: %v", err)
	}
	s, err := confirmation.NewSigner(confirmation.Config{
		Enabled: true,
		Secret:  secret,
		TTL:     5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

// highRiskHints returns a ToolHints literal with the supplied
// RiskClass bits — the tests pick a representative class for each
// high-risk bit so the table-driven assertion below stays readable.
func highRiskHints(rc mcp.RiskClass) mcp.ToolHints {
	return mcp.ToolHints{
		ReadOnly:    false,
		Destructive: rc.Has(mcp.RiskDestructive),
		Idempotent:  false,
		RiskClass:   rc,
	}
}

func recordingHandler() (mcp.ToolHandler, *atomic.Int32) {
	var calls atomic.Int32
	return func(_ context.Context, _ map[string]any) (any, error) {
		calls.Add(1)
		return "ran", nil
	}, &calls
}

// TestBeforeCall_HighRiskWithoutToken_Rejected pins the gate's
// primary contract: a non-dry-run high-risk call without a
// confirmation_token argument is rejected with
// ErrConfirmationRequired before the handler runs and before any
// rate-limit slot is consumed.
func TestBeforeCall_HighRiskWithoutToken_Rejected(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       standardPolicy(),
		Confirmation: signer,
	}
	handler, calls := recordingHandler()
	lookup := lookupWith(map[string]mcp.ToolHandler{"clockify_delete_invoice": handler})

	_, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1"},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("err = %v, want ErrConfirmationRequired", err)
	}
	if calls.Load() != 0 {
		t.Fatal("handler must not run when token missing")
	}
}

// TestBeforeCall_HighRiskDryRun_MintsTokenAndSkipsHandler verifies
// the mint path: a high-risk dry_run:true call returns a result that
// includes confirmation_required=true and a non-empty
// confirmation_token. The destructive handler must not execute.
func TestBeforeCall_HighRiskDryRun_MintsTokenAndSkipsHandler(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       standardPolicy(),
		DryRun:       dryrun.Config{Enabled: true},
		Confirmation: signer,
	}
	handler, calls := recordingHandler()
	lookup := lookupWith(map[string]mcp.ToolHandler{"clockify_delete_invoice": handler})

	result, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1", "dry_run": true},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if err != nil {
		t.Fatalf("BeforeCall err = %v", err)
	}
	if result == nil {
		t.Fatal("BeforeCall must return a dry-run result")
	}
	envelope, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result must be a map, got %T", result)
	}
	if envelope["confirmation_required"] != true {
		t.Errorf("confirmation_required = %v, want true", envelope["confirmation_required"])
	}
	token, _ := envelope["confirmation_token"].(string)
	if token == "" {
		t.Error("confirmation_token must be present and non-empty")
	}
	if envelope["confirmation_expires_at"] == nil {
		t.Error("confirmation_expires_at must be present")
	}
	if calls.Load() != 0 {
		t.Fatal("handler must not run on dry-run path")
	}
}

// TestBeforeCall_HighRiskWithValidToken_AllowsExecution verifies the
// happy execution path: re-submit with the minted token + dry_run
// removed, the gate passes through and the handler runs.
func TestBeforeCall_HighRiskWithValidToken_AllowsExecution(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       standardPolicy(),
		DryRun:       dryrun.Config{Enabled: true},
		Confirmation: signer,
	}
	handler, calls := recordingHandler()
	lookup := lookupWith(map[string]mcp.ToolHandler{"clockify_delete_invoice": handler})

	// Step 1: dry-run to mint.
	args := map[string]any{"invoice_id": "inv-1", "dry_run": true}
	previewResult, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		args,
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if err != nil {
		t.Fatalf("Mint preview err = %v", err)
	}
	envelope, _ := previewResult.(map[string]any)
	token, _ := envelope["confirmation_token"].(string)
	if token == "" {
		t.Fatal("dry-run did not produce a confirmation_token")
	}

	// Step 2: real execution call with the token.
	result, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1", "confirmation_token": token},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if err != nil {
		t.Fatalf("BeforeCall with valid token err = %v", err)
	}
	if result != nil {
		t.Fatalf("BeforeCall with valid token must return nil result, got %T", result)
	}
	// Caller proceeds to invoke the handler. We don't invoke it here
	// directly — the test pins the gate, not the dispatch loop.
	_ = calls
}

// TestBeforeCall_HighRiskWithChangedArgs_RejectsToken verifies the
// args binding. A token minted for {invoice_id:inv-1} must not allow
// a {invoice_id:inv-2} execution.
func TestBeforeCall_HighRiskWithChangedArgs_RejectsToken(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       standardPolicy(),
		DryRun:       dryrun.Config{Enabled: true},
		Confirmation: signer,
	}
	handler, calls := recordingHandler()
	lookup := lookupWith(map[string]mcp.ToolHandler{"clockify_delete_invoice": handler})

	previewResult, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1", "dry_run": true},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if err != nil {
		t.Fatalf("Mint preview err = %v", err)
	}
	envelope, _ := previewResult.(map[string]any)
	token, _ := envelope["confirmation_token"].(string)
	if token == "" {
		t.Fatal("dry-run did not produce a confirmation_token")
	}

	_, _, err = p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-2", "confirmation_token": token},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		lookup,
	)
	if err == nil {
		t.Fatal("expected error on changed-args + old token")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "confirmation") {
		t.Fatalf("error should mention confirmation, got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("handler must not run on invalid-token path")
	}
}

// TestBeforeCall_AllHighRiskBitsRequireConfirmation runs a table of
// every individual high-risk bit and asserts each one triggers the
// gate when set alone. The combination check in mcp/risk_class_high_risk_test.go
// covers compound classes — this test pins the per-bit behaviour at
// the Pipeline boundary.
func TestBeforeCall_AllHighRiskBitsRequireConfirmation(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{Policy: standardPolicy(), Confirmation: signer}
	cases := []struct {
		name string
		rc   mcp.RiskClass
	}{
		{"billing", mcp.RiskBilling},
		{"admin", mcp.RiskAdmin},
		{"permission_change", mcp.RiskPermissionChange},
		{"external_side_effect", mcp.RiskExternalSideEffect},
		{"destructive", mcp.RiskDestructive},
		{"billing_destructive", mcp.RiskBilling | mcp.RiskDestructive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := p.BeforeCall(
				context.Background(),
				"clockify_high_risk_test_tool",
				map[string]any{"id": "x"},
				highRiskHints(c.rc),
				nil,
				noLookup,
			)
			if !errors.Is(err, ErrConfirmationRequired) {
				t.Fatalf("rc=%b err = %v, want ErrConfirmationRequired", c.rc, err)
			}
		})
	}
}

// TestBeforeCall_RiskWriteAloneDoesNotRequireConfirmation pins the
// other direction: an ordinary write (no high-risk bits) flows
// through the gate untouched. Otherwise the time-tracking surface
// would suddenly need the dry-run-first flow.
func TestBeforeCall_RiskWriteAloneDoesNotRequireConfirmation(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{Policy: standardPolicy(), Confirmation: signer}
	hints := mcp.ToolHints{RiskClass: mcp.RiskWrite}
	_, _, err := p.BeforeCall(
		context.Background(),
		"clockify_update_entry",
		map[string]any{"entry_id": "e1"},
		hints,
		nil,
		noLookup,
	)
	if err != nil {
		t.Fatalf("write tool err = %v, want nil", err)
	}
}

// TestBeforeCall_ReadOnlyDoesNotRequireConfirmation pins the obvious
// case so a future bug that treats RiskRead as high-risk (e.g. by
// flipping IsHighRisk to !=0) is caught.
func TestBeforeCall_ReadOnlyDoesNotRequireConfirmation(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{Policy: standardPolicy(), Confirmation: signer}
	hints := mcp.ToolHints{ReadOnly: true, RiskClass: mcp.RiskRead}
	_, _, err := p.BeforeCall(
		context.Background(),
		"clockify_list_entries",
		map[string]any{},
		hints,
		nil,
		noLookup,
	)
	if err != nil {
		t.Fatalf("read-only err = %v, want nil", err)
	}
}

// TestBeforeCall_PolicyDeniedTrumpsConfirmation pins ordering: a
// policy-denied high-risk tool returns the policy denial, not a
// confirmation prompt. Otherwise an agent could learn from the error
// whether a denied tool is high-risk.
func TestBeforeCall_PolicyDeniedTrumpsConfirmation(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       denyToolPolicy("clockify_delete_invoice"),
		Confirmation: signer,
	}
	_, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1"},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		noLookup,
	)
	if err == nil {
		t.Fatal("expected policy denial error")
	}
	if errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("policy-denied call surfaced confirmation error instead of policy: %v", err)
	}
	if !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("expected policy error, got %v", err)
	}
}

// TestBeforeCall_NilConfirmationSignerBypassesGate pins the explicit
// opt-out path: Pipeline.Confirmation=nil disables the gate
// entirely, so legacy / development deployments retain their old
// behaviour.
func TestBeforeCall_NilConfirmationSignerBypassesGate(t *testing.T) {
	p := &Pipeline{Policy: standardPolicy(), Confirmation: nil}
	_, _, err := p.BeforeCall(
		context.Background(),
		"clockify_delete_invoice",
		map[string]any{"invoice_id": "inv-1"},
		highRiskHints(mcp.RiskDestructive|mcp.RiskBilling),
		nil,
		noLookup,
	)
	if err != nil {
		t.Fatalf("nil-signer pipeline must not gate: %v", err)
	}
}

// TestBeforeCall_HighRiskNonDestructiveDryRun_StillMintsToken pins
// the send_invoice case: a high-risk tool whose handler implements
// its own dry-run (because the descriptor is toolRW, not
// toolDestructive) must still mint a confirmation token on dry-run.
// Without this, an agent that previews clockify_send_invoice would
// see no token and be unable to execute. The minimal preview here
// is the gate's responsibility — richer previews remain a separate
// concern for the agent (call the read counterpart first).
func TestBeforeCall_HighRiskNonDestructiveDryRun_StillMintsToken(t *testing.T) {
	signer := newConfirmationSigner(t)
	p := &Pipeline{
		Policy:       standardPolicy(),
		DryRun:       dryrun.Config{Enabled: true},
		Confirmation: signer,
	}
	hints := mcp.ToolHints{
		ReadOnly:    false,
		Destructive: false, // RW tool, not toolDestructive
		RiskClass:   mcp.RiskWrite | mcp.RiskBilling | mcp.RiskExternalSideEffect,
	}
	result, _, err := p.BeforeCall(
		context.Background(),
		"clockify_send_invoice",
		map[string]any{"invoice_id": "inv-1", "dry_run": true},
		hints,
		nil,
		noLookup,
	)
	if err != nil {
		t.Fatalf("BeforeCall err = %v", err)
	}
	envelope, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	token, _ := envelope["confirmation_token"].(string)
	if token == "" {
		t.Fatal("non-destructive high-risk dry-run did not produce a token")
	}
}
