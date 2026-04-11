package enforcement

import (
	"testing"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

// mustPolicy builds a Policy via FromEnv (which has no required env vars in
// its default form) so each test gets an independent instance.
func mustPolicy(t *testing.T) *policy.Policy {
	t.Helper()
	pol, err := policy.FromEnv()
	if err != nil {
		t.Fatalf("policy.FromEnv: %v", err)
	}
	return pol
}

// TestPipelineClone covers the deep-copy helper that produces an independent
// Pipeline so per-tenant runtimes can mutate Policy/Bootstrap without
// aliasing the parent.
func TestPipelineClone(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		var p *Pipeline
		if got := p.Clone(); got != nil {
			t.Fatalf("nil Clone should return nil, got %#v", got)
		}
	})

	t.Run("deep_copies_policy_and_bootstrap", func(t *testing.T) {
		baseBootstrap := bootstrap.Config{Mode: bootstrap.FullTier1}
		original := &Pipeline{
			Policy:     mustPolicy(t),
			Bootstrap:  &baseBootstrap,
			RateLimit:  ratelimit.New(1, 1, 0),
			DryRun:     dryrun.Config{Enabled: true},
			Truncation: truncate.Config{Enabled: true, TokenBudget: 1024},
		}
		cloned := original.Clone()
		if cloned == nil {
			t.Fatal("expected non-nil clone")
		}
		if cloned == original {
			t.Fatal("Clone returned same pointer")
		}
		if cloned.Policy == original.Policy {
			t.Fatal("Policy should be deep-copied")
		}
		if cloned.Bootstrap == original.Bootstrap {
			t.Fatal("Bootstrap should be deep-copied")
		}
		if cloned.RateLimit != original.RateLimit {
			t.Fatal("RateLimit pointer should be shared (singleton)")
		}
		if cloned.DryRun != original.DryRun {
			t.Fatal("DryRun config should match by value")
		}
		if cloned.Truncation != original.Truncation {
			t.Fatal("Truncation config should match by value")
		}
	})
}

// TestGateClone covers the Activator's deep-copy helper and its small
// IsGroupAllowed / OnActivate behaviour.
func TestGateClone(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		var g *Gate
		if got := g.Clone(); got != nil {
			t.Fatalf("nil Clone should return nil, got %#v", got)
		}
	})

	t.Run("deep_copies_policy_and_bootstrap", func(t *testing.T) {
		baseBootstrap := bootstrap.Config{Mode: bootstrap.FullTier1}
		original := &Gate{
			Policy:    mustPolicy(t),
			Bootstrap: &baseBootstrap,
		}
		cloned := original.Clone()
		if cloned == nil {
			t.Fatal("expected non-nil clone")
		}
		if cloned == original {
			t.Fatal("Clone returned same pointer")
		}
		if cloned.Policy == original.Policy {
			t.Fatal("Policy should be deep-copied")
		}
		if cloned.Bootstrap == original.Bootstrap {
			t.Fatal("Bootstrap should be deep-copied")
		}
	})

	t.Run("on_activate_marks_visible", func(t *testing.T) {
		bc := bootstrap.Config{Mode: bootstrap.Minimal}
		bc.SetTier1Tools(map[string]bool{"clockify_test": true})
		gate := &Gate{Bootstrap: &bc}
		gate.OnActivate([]string{"clockify_test"})
		if !bc.IsVisible("clockify_test") {
			t.Fatal("OnActivate should mark tool visible")
		}
	})

	t.Run("is_group_allowed_default_true", func(t *testing.T) {
		gate := &Gate{}
		if !gate.IsGroupAllowed("anything") {
			t.Fatal("nil-policy gate should allow any group")
		}
	})
}
