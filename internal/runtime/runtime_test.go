//go:build legacy_platform

package runtime

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/confirmation"
)

// resetConfirmationEnv clears the confirmation knobs and re-asserts the
// canonical dry-run default so each test case starts from a known
// posture. Uses t.Setenv so cleanup is automatic.
func resetConfirmationEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKENS", "")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKEN_SECRET", "")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKEN_TTL", "")
	t.Setenv("CLOCKIFY_DRY_RUN", "")
	// Self-hosted shape — Runtime.New checks IsHostedProfile(cfg.Profile)
	// against the supplied Config rather than re-reading MCP_PROFILE.
	t.Setenv("MCP_PROFILE", "")
}

func selfHostedConfig() config.Config {
	return config.Config{
		Profile:        "", // self-hosted
		Transport:      "stdio",
		MaxMessageSize: 1 << 20,
	}
}

// TestRuntimeNewRejectsTokensEnabledWithDryRunDisabled pins the
// pre-merge fix-forward: a deployment that turns dry-run off while
// keeping confirmation tokens enabled would leave every high-risk
// tool permanently unexecutable (no mint path), so Runtime.New must
// refuse to boot with a clear message.
func TestRuntimeNewRejectsTokensEnabledWithDryRunDisabled(t *testing.T) {
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_DRY_RUN", "disabled")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKENS", "enabled")
	// Supply an explicit secret so the loader gets past the
	// hosted-secret check even in a self-hosted profile; the dry-run
	// guard must fire before any secret-derived signer is built.
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKEN_SECRET", hex.EncodeToString(make([]byte, confirmation.MinSecretBytes)))

	_, err := New(selfHostedConfig(), NewOpts{})
	if err == nil {
		t.Fatal("Runtime.New must reject CLOCKIFY_CONFIRMATION_TOKENS=enabled with CLOCKIFY_DRY_RUN=disabled")
	}
	if !strings.Contains(err.Error(), "CLOCKIFY_CONFIRMATION_TOKENS=enabled requires CLOCKIFY_DRY_RUN=enabled") {
		t.Fatalf("err = %v, want the documented dry-run-required marker", err)
	}
}

// TestRuntimeNewAllowsTokensDisabledWithDryRunDisabled pins the
// other half of the contract: an operator who genuinely wants both
// the gate and dry-run off (legacy / break-glass posture) must still
// be able to boot. The combination removes the gate entirely;
// high-risk tools execute without a preview-first contract.
func TestRuntimeNewAllowsTokensDisabledWithDryRunDisabled(t *testing.T) {
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_DRY_RUN", "disabled")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKENS", "disabled")

	rt, err := New(selfHostedConfig(), NewOpts{})
	if err != nil {
		t.Fatalf("Runtime.New: %v", err)
	}
	if rt.deps.confirmation != nil {
		t.Fatal("Runtime.deps.confirmation must be nil when tokens are disabled")
	}
}

// TestRuntimeNewAllowsTokensEnabledWithDryRunEnabled pins the
// happy-path: both knobs at their secure defaults boots a Runtime
// with a usable signer.
func TestRuntimeNewAllowsTokensEnabledWithDryRunEnabled(t *testing.T) {
	resetConfirmationEnv(t)
	// Explicit `enabled` is identical to the default; keep it explicit
	// so the test exercises the same code path operators see when they
	// pin the value in their deployment manifest.
	t.Setenv("CLOCKIFY_DRY_RUN", "enabled")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKENS", "enabled")
	t.Setenv("CLOCKIFY_CONFIRMATION_TOKEN_SECRET", hex.EncodeToString(make([]byte, confirmation.MinSecretBytes)))

	rt, err := New(selfHostedConfig(), NewOpts{})
	if err != nil {
		t.Fatalf("Runtime.New: %v", err)
	}
	if rt.deps.confirmation == nil {
		t.Fatal("Runtime.deps.confirmation must be non-nil when tokens are enabled")
	}
}

// resetCeilingEnv clears every knob the transport-scoped ceiling gate
// reads so each test starts from a known posture. Uses t.Setenv so
// cleanup is automatic.
func resetCeilingEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CLOCKIFY_POLICY", "")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "")
	t.Setenv("CLOCKIFY_DENY_TOOLS", "")
	t.Setenv("CLOCKIFY_DENY_GROUPS", "")
	t.Setenv("CLOCKIFY_ALLOW_GROUPS", "")
	t.Setenv("MCP_PROFILE", "")
}

// configForTransport returns a minimal Config carrying the named
// transport; everything else is left at the policy package's default
// shape so the test surfaces the ceiling gate, not unrelated config
// validation.
func configForTransport(transport string) config.Config {
	return config.Config{
		Profile:        "",
		Transport:      transport,
		MaxMessageSize: 1 << 20,
	}
}

// TestRuntimeNew_RejectsProcessExceedingCeilingOnStreamableHTTP pins
// the transport-scoped ceiling gate moved out of policy.FromEnv per
// the PR #99 review final blocker. The pair (CLOCKIFY_POLICY=standard,
// MCP_TENANT_POLICY_CEILING=time_tracking_safe) is rejected at
// Runtime.New only when streamable_http is the selected transport;
// other transports do not consume control-plane tenant records, so
// the ceiling has no runtime effect there and the gate must not fire.
func TestRuntimeNew_RejectsProcessExceedingCeilingOnStreamableHTTP(t *testing.T) {
	resetCeilingEnv(t)
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_POLICY", "standard")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	_, err := New(configForTransport("streamable_http"), NewOpts{})
	if err == nil {
		t.Fatal("Runtime.New must reject CLOCKIFY_POLICY > MCP_TENANT_POLICY_CEILING under streamable_http")
	}
	for _, want := range []string{
		"CLOCKIFY_POLICY",
		"MCP_TENANT_POLICY_CEILING",
		"standard",
		"time_tracking_safe",
		"lower one to match",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name %q so the operator can act on it; got: %v", want, err)
		}
	}
}

// TestRuntimeNew_AllowsProcessExceedingCeilingOnStdio confirms the
// gate is transport-scoped. stdio is single-user / single-tenant and
// does not consume control-plane TenantRecord overrides; an inherited
// MCP_TENANT_POLICY_CEILING is a no-op at runtime, so failing startup
// there would reject a configuration with no runtime effect. This is
// the bad case the reviewer flagged.
func TestRuntimeNew_AllowsProcessExceedingCeilingOnStdio(t *testing.T) {
	resetCeilingEnv(t)
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_POLICY", "standard")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	rt, err := New(configForTransport("stdio"), NewOpts{})
	if err != nil {
		t.Fatalf("Runtime.New under stdio must accept the pair (env-var is informational only on stdio): %v", err)
	}
	if rt.Policy().Mode != "standard" {
		t.Errorf("policy mode = %q, want standard (unchanged)", rt.Policy().Mode)
	}
}

// TestRuntimeNew_AllowsProcessExceedingCeilingOnGRPC mirrors the
// stdio carve-out for gRPC. The gRPC transport authenticates via its
// own interceptor and does not flow through tenantRuntime /
// tenantpolicy.Derive, so MCP_TENANT_POLICY_CEILING is informational
// at most. Failing startup there for "policy > ceiling" would penalise
// an operator who inherited the env from a hosted profile in their
// shell while running the gRPC binary.
func TestRuntimeNew_AllowsProcessExceedingCeilingOnGRPC(t *testing.T) {
	resetCeilingEnv(t)
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_POLICY", "standard")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	rt, err := New(configForTransport("grpc"), NewOpts{})
	if err != nil {
		t.Fatalf("Runtime.New under grpc must accept the pair (env-var is informational only on grpc): %v", err)
	}
	if rt.Policy().Mode != "standard" {
		t.Errorf("policy mode = %q, want standard (unchanged)", rt.Policy().Mode)
	}
}

// TestRuntimeNew_AcceptsAlignedProcessAndCeilingOnStreamableHTTP pins
// the vanilla hosted-deployment shape: CLOCKIFY_POLICY and
// MCP_TENANT_POLICY_CEILING both pinned to time_tracking_safe (the
// shared-service profile default). Runtime.New must boot cleanly.
func TestRuntimeNew_AcceptsAlignedProcessAndCeilingOnStreamableHTTP(t *testing.T) {
	resetCeilingEnv(t)
	resetConfirmationEnv(t)
	t.Setenv("CLOCKIFY_POLICY", "time_tracking_safe")
	t.Setenv("MCP_TENANT_POLICY_CEILING", "time_tracking_safe")

	rt, err := New(configForTransport("streamable_http"), NewOpts{})
	if err != nil {
		t.Fatalf("Runtime.New must accept aligned process+ceiling: %v", err)
	}
	if rt.Policy().Ceiling != "time_tracking_safe" {
		t.Errorf("policy ceiling = %q, want time_tracking_safe", rt.Policy().Ceiling)
	}
}
