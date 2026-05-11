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
