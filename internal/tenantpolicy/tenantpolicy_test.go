package tenantpolicy

import (
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/policy"
)

// TestDerive_DenyToolsUnioned pins the narrowing-only merge contract
// for tenant DenyTools. Tenant entries are added on top of the
// process deny list; the process list is never erased.
func TestDerive_DenyToolsUnioned(t *testing.T) {
	process := &policy.Policy{
		Mode:        policy.Standard,
		DeniedTools: map[string]bool{"clockify_send_invoice": true},
	}
	tenant := controlplane.TenantRecord{
		ID:        "acme",
		DenyTools: []string{"clockify_delete_entry"},
	}
	pol, err := Derive(process, tenant)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !pol.DeniedTools["clockify_send_invoice"] {
		t.Error("expected process-level deny to survive (clockify_send_invoice)")
	}
	if !pol.DeniedTools["clockify_delete_entry"] {
		t.Error("expected tenant-level deny to be added (clockify_delete_entry)")
	}
}

// TestDerive_DenyGroupsUnioned pins the same shape for DenyGroups.
func TestDerive_DenyGroupsUnioned(t *testing.T) {
	process := &policy.Policy{
		Mode:         policy.Standard,
		DeniedGroups: map[string]bool{"invoices": true},
	}
	tenant := controlplane.TenantRecord{
		ID:         "acme",
		DenyGroups: []string{"user_management"},
	}
	pol, err := Derive(process, tenant)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !pol.DeniedGroups["invoices"] {
		t.Error("expected process-level group deny to survive (invoices)")
	}
	if !pol.DeniedGroups["user_management"] {
		t.Error("expected tenant-level group deny to be added (user_management)")
	}
}

// TestDerive_AllowGroupsIgnoredUnderBlockingMode pins the silent-skip
// behaviour for AllowGroups under a group-blocking effective mode.
// The list is dropped (would be a no-op via IsGroupAllowed anyway)
// and TenantAllowGroupsIgnored is set so clockify_policy_info can
// surface the diagnostic.
func TestDerive_AllowGroupsIgnoredUnderBlockingMode(t *testing.T) {
	process := &policy.Policy{Mode: policy.SafeCore}
	tenant := controlplane.TenantRecord{
		ID:          "acme",
		AllowGroups: []string{"invoices"},
	}
	pol, err := Derive(process, tenant)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if pol.AllowedGroups != nil && pol.AllowedGroups["invoices"] {
		t.Error("expected AllowGroups to be silently dropped under safe_core mode")
	}
	if !pol.TenantAllowGroupsIgnored {
		t.Error("expected TenantAllowGroupsIgnored marker to be set")
	}
}

// TestDerive_AllowGroupsIntersect pins the intersection narrowing for
// AllowGroups under a non-blocking mode. Tenant cannot widen the
// process allowlist; it can only narrow.
func TestDerive_AllowGroupsIntersect(t *testing.T) {
	process := &policy.Policy{
		Mode:          policy.Standard,
		AllowedGroups: map[string]bool{"invoices": true, "user_management": true},
	}
	tenant := controlplane.TenantRecord{
		ID:          "acme",
		AllowGroups: []string{"invoices", "scheduling"}, // "scheduling" must not be added; only "invoices" survives
	}
	pol, err := Derive(process, tenant)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !pol.AllowedGroups["invoices"] {
		t.Error("expected invoices to remain in intersected allowlist")
	}
	if pol.AllowedGroups["user_management"] {
		t.Error("expected user_management to be removed by tenant intersection")
	}
	if pol.AllowedGroups["scheduling"] {
		t.Error("expected scheduling to be rejected (not in process allowlist)")
	}
	if pol.TenantAllowGroupsIgnored {
		t.Error("ignored marker must not be set under standard mode")
	}
}

// TestDerive_AllowGroupsAppliesWhenProcessUnset covers the carve-out:
// when the process did not set an allowlist, the tenant list defines
// the whitelist (which is still narrowing relative to "everything
// allowed").
func TestDerive_AllowGroupsAppliesWhenProcessUnset(t *testing.T) {
	process := &policy.Policy{Mode: policy.Standard} // AllowedGroups nil
	tenant := controlplane.TenantRecord{
		ID:          "acme",
		AllowGroups: []string{"invoices"},
	}
	pol, err := Derive(process, tenant)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !pol.AllowedGroups["invoices"] {
		t.Error("expected tenant AllowGroups to define the whitelist when process had none")
	}
	if pol.AllowedGroups["scheduling"] {
		t.Error("tenant AllowGroups must only contain its declared entries")
	}
}

// TestDerive_CeilingErrorPropagates confirms the helper surfaces the
// EffectiveTenantMode error verbatim. ADR 0021's narrowing contract
// stays at the boundary tested via runtime/E2E integration; the
// helper-level test keeps the error shape stable even if upstream
// wrapping changes.
func TestDerive_CeilingErrorPropagates(t *testing.T) {
	process := &policy.Policy{Mode: policy.TimeTrackingSafe, Ceiling: policy.TimeTrackingSafe}
	tenant := controlplane.TenantRecord{
		ID:         "acme",
		PolicyMode: string(policy.Standard),
	}
	_, err := Derive(process, tenant)
	if err == nil {
		t.Fatal("expected error for tenant broadening past ceiling")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'exceeds' error, got: %v", err)
	}
}

// TestDerive_ProcessExceedsCeilingFailsClosed confirms the helper
// also rejects the process-broadens-past-ceiling misconfiguration.
// Pinned at this layer so a future regression in EffectiveTenantMode
// (or in any caller that imports this package) would surface here
// without requiring runtime to run.
func TestDerive_ProcessExceedsCeilingFailsClosed(t *testing.T) {
	process := &policy.Policy{Mode: policy.Standard, Ceiling: policy.TimeTrackingSafe}
	tenant := controlplane.TenantRecord{ID: "acme"} // empty PolicyMode
	_, err := Derive(process, tenant)
	if err == nil {
		t.Fatal("expected error for process mode broader than explicit ceiling")
	}
	if !strings.Contains(err.Error(), "process mode") || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'process mode ... exceeds ceiling', got: %v", err)
	}
}
