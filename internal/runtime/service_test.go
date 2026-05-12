package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/tools"
)

// recordingStore captures every AppendAuditEvent call so the runtime
// auditor's ID synthesis can be inspected. Other Store methods return
// zero values — the runtime auditor only ever calls AppendAuditEvent.
type recordingStore struct {
	mu     sync.Mutex
	events []controlplane.AuditEvent
}

func (s *recordingStore) AppendAuditEvent(e controlplane.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *recordingStore) AppendAuditEventBatch(events []controlplane.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, events...)
	return nil
}

func (s *recordingStore) snapshot() []controlplane.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]controlplane.AuditEvent, len(s.events))
	copy(out, s.events)
	return out
}

// Stubs — never called by controlPlaneAuditor.RecordAudit, but the
// controlplane.Store interface requires them.
func (s *recordingStore) Tenant(string) (controlplane.TenantRecord, bool) {
	return controlplane.TenantRecord{}, false
}
func (s *recordingStore) PutTenant(controlplane.TenantRecord) error { return nil }
func (s *recordingStore) CredentialRef(string) (controlplane.CredentialRef, bool) {
	return controlplane.CredentialRef{}, false
}
func (s *recordingStore) PutCredentialRef(controlplane.CredentialRef) error { return nil }
func (s *recordingStore) Session(string) (controlplane.SessionRecord, bool) {
	return controlplane.SessionRecord{}, false
}
func (s *recordingStore) PutSession(controlplane.SessionRecord) error             { return nil }
func (s *recordingStore) DeleteSession(string) error                              { return nil }
func (s *recordingStore) RetainAudit(context.Context, time.Duration) (int, error) { return 0, nil }
func (s *recordingStore) Close() error                                            { return nil }

// TestControlPlaneAuditorAuditIDIncludesPhaseAndOutcome locks that the
// runtime auditor synthesises an external ID containing both the phase
// ("intent") and outcome ("attempted") segments so the Postgres
// ON CONFLICT (external_id) DO NOTHING dedupe cannot collapse the
// intent/outcome pair into a single row. It also pins the event-level
// metadata (tenant/subject/session/transport, phase, At) the runtime
// must preserve when bridging mcp.AuditEvent → controlplane.AuditEvent.
func TestControlPlaneAuditorAuditIDIncludesPhaseAndOutcome(t *testing.T) {
	store := &recordingStore{}
	auditor := controlPlaneAuditor{store: store}

	begin := time.Now()
	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:    "clockify_log_time",
		Action:  "create_entry",
		Outcome: "attempted",
		Phase:   mcp.PhaseIntent,
		Metadata: map[string]string{
			"tenant_id":  "acme",
			"subject":    "alice@example.com",
			"session_id": "sess-123",
			"transport":  "streamable_http",
		},
	}); err != nil {
		t.Fatalf("RecordAudit returned error: %v", err)
	}

	got := store.snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(got))
	}
	ev := got[0]

	if !strings.Contains(ev.ID, "intent") {
		t.Errorf("ID %q does not contain phase segment %q", ev.ID, "intent")
	}
	if !strings.Contains(ev.ID, "attempted") {
		t.Errorf("ID %q does not contain outcome segment %q", ev.ID, "attempted")
	}
	if !strings.Contains(ev.ID, "sess-123") {
		t.Errorf("ID %q does not contain session segment", ev.ID)
	}
	if !strings.Contains(ev.ID, "clockify_log_time") {
		t.Errorf("ID %q does not contain tool segment", ev.ID)
	}

	if ev.At.IsZero() {
		t.Error("event At is zero; runtime auditor must stamp the event time")
	}
	if ev.At.Before(begin.Add(-time.Second)) || ev.At.After(time.Now().Add(time.Second)) {
		t.Errorf("event At %v is outside the call window [%v..now]", ev.At, begin)
	}
	if ev.Phase != mcp.PhaseIntent {
		t.Errorf("Phase = %q, want %q", ev.Phase, mcp.PhaseIntent)
	}
	if ev.TenantID != "acme" {
		t.Errorf("TenantID = %q, want %q", ev.TenantID, "acme")
	}
	if ev.Subject != "alice@example.com" {
		t.Errorf("Subject = %q, want %q", ev.Subject, "alice@example.com")
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "sess-123")
	}
	if ev.Transport != "streamable_http" {
		t.Errorf("Transport = %q, want %q", ev.Transport, "streamable_http")
	}
	if ev.Tool != "clockify_log_time" {
		t.Errorf("Tool = %q, want %q", ev.Tool, "clockify_log_time")
	}
	if ev.Action != "create_entry" {
		t.Errorf("Action = %q, want %q", ev.Action, "create_entry")
	}
	if ev.Outcome != "attempted" {
		t.Errorf("Outcome = %q, want %q", ev.Outcome, "attempted")
	}
}

// TestControlPlaneAuditorIntentAndOutcomeIDsDiffer is the regression
// test for the runtime side of the dedupe-collision bug. Two RecordAudit
// calls for the same tool/session in the same nanosecond — one
// PhaseIntent, one PhaseOutcome — must yield distinct external IDs.
// Without phase+outcome in the synthesised ID, the Postgres
// ON CONFLICT (external_id) DO NOTHING insert would collapse the pair
// into a single row and the audit trail would lose the outcome record.
func TestControlPlaneAuditorIntentAndOutcomeIDsDiffer(t *testing.T) {
	store := &recordingStore{}
	auditor := controlPlaneAuditor{store: store}

	meta := map[string]string{
		"tenant_id":  "acme",
		"subject":    "alice",
		"session_id": "sess-xyz",
		"transport":  "streamable_http",
	}

	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:     "clockify_add_entry",
		Action:   "create_entry",
		Outcome:  "attempted",
		Phase:    mcp.PhaseIntent,
		Metadata: meta,
	}); err != nil {
		t.Fatalf("intent RecordAudit: %v", err)
	}
	if err := auditor.RecordAudit(mcp.AuditEvent{
		Tool:     "clockify_add_entry",
		Action:   "create_entry",
		Outcome:  "succeeded",
		Phase:    mcp.PhaseOutcome,
		Metadata: meta,
	}); err != nil {
		t.Fatalf("outcome RecordAudit: %v", err)
	}

	got := store.snapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].ID == got[1].ID {
		t.Fatalf("intent and outcome share ID %q — Postgres ON CONFLICT would collapse them into one row", got[0].ID)
	}
	if got[0].Phase != mcp.PhaseIntent || got[1].Phase != mcp.PhaseOutcome {
		t.Fatalf("phase order corrupted: got %q then %q", got[0].Phase, got[1].Phase)
	}
}

// TestControlPlaneAuditorNilStoreNoOps locks the documented "store-less"
// behaviour — a controlPlaneAuditor with a nil store must silently
// succeed rather than panic, so the runtime can wire it unconditionally.
func TestControlPlaneAuditorNilStoreNoOps(t *testing.T) {
	auditor := controlPlaneAuditor{store: nil}
	if err := auditor.RecordAudit(mcp.AuditEvent{Tool: "x", Phase: mcp.PhaseOutcome}); err != nil {
		t.Fatalf("nil-store RecordAudit returned error: %v", err)
	}
}

// tenantRuntimeStore is a controlplane.Store fixture that lets
// per-test cases inject a single Tenant + CredentialRef pair so
// tenantRuntime can be exercised without standing up a full backend.
// Only the read paths exercised by tenantRuntime are populated; the
// rest fall back to the recordingStore zero behaviour.
type tenantRuntimeStore struct {
	recordingStore
	tenant     controlplane.TenantRecord
	credential controlplane.CredentialRef
}

func (s *tenantRuntimeStore) Tenant(id string) (controlplane.TenantRecord, bool) {
	if s.tenant.ID != id {
		return controlplane.TenantRecord{}, false
	}
	return s.tenant, true
}

func (s *tenantRuntimeStore) CredentialRef(id string) (controlplane.CredentialRef, bool) {
	if s.credential.ID != id {
		return controlplane.CredentialRef{}, false
	}
	return s.credential, true
}

// TestTenantRuntime_HostedRejectsTenantHTTP locks the per-tenant
// equivalent of the env-level CLOCKIFY_BASE_URL guardrail: a tenant
// credential that resolves a remote http baseURL must be refused
// when the deployment runs under a hosted profile, even though the
// material was supplied through the trusted control plane.
func TestTenantRuntime_HostedRejectsTenantHTTP(t *testing.T) {
	store := &tenantRuntimeStore{
		tenant: controlplane.TenantRecord{
			ID:              "acme",
			CredentialRefID: "cred-1",
			BaseURL:         "http://upstream.example.com/api/v1",
		},
		credential: controlplane.CredentialRef{
			ID:        "cred-1",
			Backend:   "inline",
			Reference: "secret-key",
			Workspace: "ws-1",
		},
	}
	deps := runtimeDeps{
		cfg: config.Config{
			Profile: "shared-service",
		},
		policy:    &policy.Policy{Mode: policy.Standard},
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for tenant http baseURL under hosted profile")
	}
	if !strings.Contains(err.Error(), "must use https") && !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https/url error, got: %v", err)
	}
}

// TestTenantRuntime_HostedRejectsLoopback locks the loopback close
// of the same gate. Loopback http is acceptable in self-hosted
// stdio installs but never in hosted profiles — a tenant supplying
// http://127.0.0.1 to a sidecar proxy could otherwise hairpin
// cleartext traffic through a production gateway.
func TestTenantRuntime_HostedRejectsLoopback(t *testing.T) {
	store := &tenantRuntimeStore{
		tenant: controlplane.TenantRecord{
			ID:              "acme",
			CredentialRefID: "cred-1",
			BaseURL:         "http://127.0.0.1:8080/api/v1",
		},
		credential: controlplane.CredentialRef{
			ID:        "cred-1",
			Backend:   "inline",
			Reference: "secret-key",
			Workspace: "ws-1",
		},
	}
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "prod-postgres"},
		policy:    &policy.Policy{Mode: policy.Standard},
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for loopback http tenant baseURL under hosted profile")
	}
}

// TestTenantRuntime_NonHostedAllowsLoopback verifies the documented
// permissive path: a self-hosted profile (or no profile) keeps
// accepting loopback http baseURLs from tenant credentials so local
// docker-compose / dev backends keep working.
func TestTenantRuntime_NonHostedAllowsLoopback(t *testing.T) {
	store := &tenantRuntimeStore{
		tenant: controlplane.TenantRecord{
			ID:              "acme",
			CredentialRefID: "cred-1",
			BaseURL:         "http://127.0.0.1:8080/api/v1",
		},
		credential: controlplane.CredentialRef{
			ID:        "cred-1",
			Backend:   "inline",
			Reference: "secret-key",
			Workspace: "ws-1",
		},
	}
	deps := runtimeDeps{
		cfg:       config.Config{Profile: ""},
		policy:    &policy.Policy{Mode: policy.Standard},
		bootstrap: bootstrap.Config{},
	}
	rt, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err != nil {
		t.Fatalf("expected loopback http to be accepted in non-hosted profile, got: %v", err)
	}
	if rt == nil || rt.Server == nil {
		t.Fatal("expected non-nil runtime + server")
	}
	if rt.Close != nil {
		rt.Close()
	}
}

// TestBuildServer_PropagatesSanitizeUpstreamErrors guards the central-
// wiring fix: every transport (stdio, legacy_http, streamable session,
// grpc) must observe cfg.SanitizeUpstreamErrors, which means buildServer
// itself must assign it. Pre-fix only legacy_http and streamable_http's
// per-session overlay set the flag, so an stdio operator setting
// CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1 saw no effect at all.
func TestBuildServer_PropagatesSanitizeUpstreamErrors(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"sanitize_off", false},
		{"sanitize_on", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := clockify.NewClient("k", "https://api.clockify.me/api/v1", time.Second, 0)
			service := tools.New(client, "ws-test")
			pol := &policy.Policy{Mode: policy.Standard}
			bc := &bootstrap.Config{}
			deps := runtimeDeps{
				cfg:    config.Config{SanitizeUpstreamErrors: c.want},
				policy: pol,
			}
			server := buildServer("test-version", deps, service, pol, bc)
			if server.SanitizeUpstreamErrors != c.want {
				t.Fatalf("server.SanitizeUpstreamErrors = %v, want %v", server.SanitizeUpstreamErrors, c.want)
			}
		})
	}
}

func TestBuildServer_ActivateToolByTier2Name(t *testing.T) {
	client := clockify.NewClient("k", "https://api.clockify.me/api/v1", time.Second, 0)
	service := tools.New(client, "ws-test")
	pol := &policy.Policy{Mode: policy.Standard}
	bc := &bootstrap.Config{Mode: bootstrap.Minimal}
	deps := runtimeDeps{cfg: config.Config{}}
	server := buildServer("test-version", deps, service, pol, bc)
	if server == nil {
		t.Fatal("expected server")
	}

	result, err := service.ActivateTool(context.Background(), "clockify_send_invoice")
	if err != nil {
		t.Fatalf("activate tool by tier2 name: %v", err)
	}
	if result.Kind != "tool" || result.Name != "clockify_send_invoice" || result.Group != "invoices" {
		t.Fatalf("unexpected activation result: %+v", result)
	}
	if result.ToolCount != 12 {
		t.Fatalf("expected invoices tool count 12, got %d", result.ToolCount)
	}
	found := false
	for _, name := range result.ActivatedTools {
		if name == "clockify_send_invoice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("activated tools does not include requested tool: %+v", result.ActivatedTools)
	}
}

func TestBuildServer_ActivateToolClassifiesPolicyHiddenTools(t *testing.T) {
	client := clockify.NewClient("k", "https://api.clockify.me/api/v1", time.Second, 0)
	service := tools.New(client, "ws-test")
	pol := &policy.Policy{Mode: policy.ReadOnly}
	bc := &bootstrap.Config{Mode: bootstrap.Minimal}
	deps := runtimeDeps{cfg: config.Config{}}
	server := buildServer("test-version", deps, service, pol, bc)
	if server == nil {
		t.Fatal("expected server")
	}

	result, err := service.ActivateTool(context.Background(), "clockify_create_project")
	if err != nil {
		t.Fatalf("activate tier1 write tool: %v", err)
	}
	if len(result.VisibleActivatedTools) != 0 {
		t.Fatalf("write tool should remain hidden under read_only policy, got visible=%v", result.VisibleActivatedTools)
	}
	if len(result.ActivatedToolsBlockedByPolicy) != 1 || result.ActivatedToolsBlockedByPolicy[0] != "clockify_create_project" {
		t.Fatalf("expected policy-blocked classification, got hidden=%v blocked=%v",
			result.ActivatedToolsHiddenByBootstrap, result.ActivatedToolsBlockedByPolicy)
	}
	if server.VisibleToolNames()["clockify_create_project"] {
		t.Fatal("policy-blocked tool should not appear in tools/list visibility")
	}
}

func TestBuildServer_DeactivateGroupRemovesTier2Tools(t *testing.T) {
	client := clockify.NewClient("k", "https://api.clockify.me/api/v1", time.Second, 0)
	service := tools.New(client, "ws-test")
	pol := &policy.Policy{Mode: policy.Standard}
	bc := &bootstrap.Config{Mode: bootstrap.Minimal}
	deps := runtimeDeps{cfg: config.Config{}}
	server := buildServer("test-version", deps, service, pol, bc)
	if server == nil {
		t.Fatal("expected server")
	}

	if _, err := service.ActivateGroup(context.Background(), "invoices"); err != nil {
		t.Fatalf("activate invoices: %v", err)
	}
	if !server.VisibleToolNames()["clockify_list_invoices"] {
		t.Fatal("expected invoice tool visible after activation")
	}
	result, err := service.DeactivateGroup(context.Background(), "invoices")
	if err != nil {
		t.Fatalf("deactivate invoices: %v", err)
	}
	if result.ToolCount != 12 {
		t.Fatalf("ToolCount=%d, want 12", result.ToolCount)
	}
	if server.VisibleToolNames()["clockify_list_invoices"] {
		t.Fatal("expected invoice tool hidden after deactivation")
	}
}

// --------------------------------------------------------------------
// ADR 0021 hosted tenant policy ceiling tests
// --------------------------------------------------------------------

// hostedCeilingStore builds a tenantRuntimeStore for ceiling tests
// with a default-secure inline credential pointing at https upstream
// (so URL-safety gates do not interfere with the ceiling assertion).
func hostedCeilingStore(tenantID string, tenantMode policy.Mode, denyTools, denyGroups, allowGroups []string) *tenantRuntimeStore {
	return &tenantRuntimeStore{
		tenant: controlplane.TenantRecord{
			ID:              tenantID,
			CredentialRefID: "cred-1",
			BaseURL:         "https://upstream.example.com/api/v1",
			PolicyMode:      string(tenantMode),
			DenyTools:       denyTools,
			DenyGroups:      denyGroups,
			AllowGroups:     allowGroups,
		},
		credential: controlplane.CredentialRef{
			ID:        "cred-1",
			Backend:   "inline",
			Reference: "secret-key",
			Workspace: "ws-1",
		},
	}
}

// TestTenantRuntime_BroadeningRejectedUnderExplicitCeiling pins the
// core ADR 0021 invariant: a hosted profile with an explicit
// time_tracking_safe ceiling refuses a tenant that requests
// `standard`. The error must be operator-actionable so misconfigured
// rows show up at session-create time, not at first tool call.
func TestTenantRuntime_BroadeningRejectedUnderExplicitCeiling(t *testing.T) {
	store := hostedCeilingStore("acme", policy.Standard, nil, nil, nil)
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.TimeTrackingSafe, Ceiling: policy.TimeTrackingSafe},
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for tenant broadening past ceiling")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'exceeds' error, got: %v", err)
	}
}

// TestTenantRuntime_BroadeningRejectedUnderImplicitCeiling pins the
// implicit-ceiling case: no explicit MCP_TENANT_POLICY_CEILING set,
// process mode still acts as the ceiling. A hosted operator who
// forgets to set the env var still cannot have tenants broaden past
// the process posture.
func TestTenantRuntime_BroadeningRejectedUnderImplicitCeiling(t *testing.T) {
	store := hostedCeilingStore("acme", policy.SafeCore, nil, nil, nil)
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.TimeTrackingSafe}, // no explicit Ceiling
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for tenant broadening past implicit process ceiling")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'exceeds' error, got: %v", err)
	}
}

// TestTenantRuntime_NarrowingAllowed verifies a tenant pinned to a
// stricter mode than the process builds successfully. Mirrors the
// vanilla hosted-profile shape where CLOCKIFY_POLICY and the ceiling
// are both pinned to time_tracking_safe.
func TestTenantRuntime_NarrowingAllowed(t *testing.T) {
	store := hostedCeilingStore("acme", policy.ReadOnly, nil, nil, nil)
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.TimeTrackingSafe, Ceiling: policy.TimeTrackingSafe},
		bootstrap: bootstrap.Config{},
	}
	rt, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err != nil {
		t.Fatalf("expected narrowing tenant to be accepted, got: %v", err)
	}
	if rt == nil || rt.Server == nil {
		t.Fatal("expected non-nil runtime + server")
	}
	if rt.Close != nil {
		rt.Close()
	}
}

// TestTenantRuntime_ProcessExceedsCeilingFailsClosed pins the
// fail-closed shape for the misconfiguration where an operator
// overrode CLOCKIFY_POLICY to standard while leaving the hosted
// MCP_TENANT_POLICY_CEILING=time_tracking_safe default in place.
// The error must fire even when the tenant record carries no
// PolicyMode override — the implicit-inherit path must not bypass
// the ceiling check. ADR 0021.
func TestTenantRuntime_ProcessExceedsCeilingFailsClosed(t *testing.T) {
	store := hostedCeilingStore("acme", "", nil, nil, nil) // empty tenant PolicyMode
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.Standard, Ceiling: policy.TimeTrackingSafe},
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for process mode broader than explicit ceiling")
	}
	if !strings.Contains(err.Error(), "process mode") || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'process mode ... exceeds ceiling' error, got: %v", err)
	}
}

// TestTenantRuntime_EmptyTenantModeInheritsProcess pins the
// inheritance path. An empty tenant mode must not trigger ceiling
// enforcement; the session takes the process mode.
func TestTenantRuntime_EmptyTenantModeInheritsProcess(t *testing.T) {
	store := hostedCeilingStore("acme", "", nil, nil, nil)
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.TimeTrackingSafe, Ceiling: policy.TimeTrackingSafe},
		bootstrap: bootstrap.Config{},
	}
	rt, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err != nil {
		t.Fatalf("expected empty tenant mode to inherit process mode, got: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if rt.Close != nil {
		rt.Close()
	}
}

// TestTenantRuntime_UnknownTenantModeFailsClosed pins the unknown-
// enum fail-closed shape. A corrupted Postgres row carrying
// "tenant-admin" must not silently sail through as Standard or Full.
func TestTenantRuntime_UnknownTenantModeFailsClosed(t *testing.T) {
	store := hostedCeilingStore("acme", policy.Mode("tenant-admin"), nil, nil, nil)
	deps := runtimeDeps{
		cfg:       config.Config{Profile: "shared-service"},
		policy:    &policy.Policy{Mode: policy.TimeTrackingSafe, Ceiling: policy.TimeTrackingSafe},
		bootstrap: bootstrap.Config{},
	}
	_, err := tenantRuntime(context.Background(), "acme", deps, store)
	if err == nil {
		t.Fatal("expected error for unknown tenant policyMode")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected 'invalid' error, got: %v", err)
	}
}

// Helper-level deny-union, allow-intersect, and ceiling-propagation
// tests now live in internal/tenantpolicy alongside the Derive helper
// so the same test surface covers both the production runtime and
// the shared-service Postgres E2E factory closure that imports the
// package directly. See internal/tenantpolicy/tenantpolicy_test.go.

// TestTenantRuntime_SelfRegisteredBootstrapSatisfiesImplicitCeiling
// verifies the single-tenant-http bootstrap path: the auto-registered
// tenant carries PolicyMode = process mode, so under the implicit
// ceiling (process mode itself) the tenant always satisfies it.
// Exercises both time_tracking_safe and standard process modes.
func TestTenantRuntime_SelfRegisteredBootstrapSatisfiesImplicitCeiling(t *testing.T) {
	for _, mode := range []policy.Mode{policy.TimeTrackingSafe, policy.Standard} {
		t.Run(string(mode), func(t *testing.T) {
			// Mirror bootstrapDefaultTenant's seed shape: PolicyMode = process mode.
			store := hostedCeilingStore("self", mode, nil, nil, nil)
			deps := runtimeDeps{
				cfg:       config.Config{Profile: "single-tenant-http"},
				policy:    &policy.Policy{Mode: mode}, // no explicit Ceiling for single-tenant-http
				bootstrap: bootstrap.Config{},
			}
			rt, err := tenantRuntime(context.Background(), "self", deps, store)
			if err != nil {
				t.Fatalf("bootstrap tenant under %s must satisfy implicit ceiling: %v", mode, err)
			}
			if rt != nil && rt.Close != nil {
				rt.Close()
			}
		})
	}
}
