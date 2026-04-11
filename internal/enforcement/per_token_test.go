package enforcement

import (
	"context"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/ratelimit"
)

// TestBeforeCallIsolatesPerSubjectBudgets asserts that the enforcement
// pipeline reads Principal from the request context and isolates per-token
// rate budgets between subjects — subject A can exhaust its own window
// without affecting subject B's budget.
func TestBeforeCallIsolatesPerSubjectBudgets(t *testing.T) {
	rl := ratelimit.NewWithAcquireTimeout(0, 1000, 60000, 0)
	// Configure per-token: 0 concurrency cap, 1 call/window per subject.
	rlPtr := rl
	setPerTokenLimits(t, rlPtr, 0, 1)

	pipe := &Pipeline{
		Policy:    standardPolicy(),
		Bootstrap: fullTier1Bootstrap("probe"),
		RateLimit: rl,
	}

	alice := &authn.Principal{Subject: "alice", TenantID: "t1", AuthMode: authn.ModeOIDC}
	bob := &authn.Principal{Subject: "bob", TenantID: "t1", AuthMode: authn.ModeOIDC}
	ctxA := authn.WithPrincipal(context.Background(), alice)
	ctxB := authn.WithPrincipal(context.Background(), bob)

	// Alice's first call succeeds.
	_, rel, err := pipe.BeforeCall(ctxA, "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err != nil {
		t.Fatalf("alice 1: %v", err)
	}
	if rel != nil {
		rel()
	}

	// Alice's second call is rejected by the per-token layer.
	_, _, err = pipe.BeforeCall(ctxA, "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err == nil {
		t.Fatal("expected alice's second call to be rejected")
	}

	// Bob's call still succeeds — isolation.
	_, rel, err = pipe.BeforeCall(ctxB, "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if rel != nil {
		rel()
	}
}

// TestBeforeCallAnonymousFallsBackToGlobal asserts that when no Principal is
// on the context, the pipeline does NOT bucket by subject — the global
// budget is the only gate.
func TestBeforeCallAnonymousFallsBackToGlobal(t *testing.T) {
	rl := ratelimit.NewWithAcquireTimeout(0, 2, 60000, 0)
	// Per-token window cap of 1: if anonymous traffic were bucketed by subject,
	// the second anon call would fail. Because subject="" disables per-token,
	// both calls must hit the global window (cap 2).
	setPerTokenLimits(t, rl, 0, 1)

	pipe := &Pipeline{
		Policy:    standardPolicy(),
		Bootstrap: fullTier1Bootstrap("probe"),
		RateLimit: rl,
	}

	_, rel1, err := pipe.BeforeCall(context.Background(), "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err != nil {
		t.Fatalf("anon 1: %v", err)
	}
	if rel1 != nil {
		rel1()
	}
	_, rel2, err := pipe.BeforeCall(context.Background(), "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err != nil {
		t.Fatalf("anon 2: %v", err)
	}
	if rel2 != nil {
		rel2()
	}

	// The third anon call hits the global window cap.
	_, _, err = pipe.BeforeCall(context.Background(), "probe", nil, mcp.ToolHints{ReadOnly: true}, noLookup)
	if err == nil {
		t.Fatal("expected global rejection on third anon call")
	}
}

// setPerTokenLimits is a test hook that sets the per-token fields on a
// RateLimiter without exposing them publicly. The ratelimit package deliberately
// keeps them unexported since they're configured via env in production.
func setPerTokenLimits(t *testing.T, rl *ratelimit.RateLimiter, maxConc, maxWin int) {
	t.Helper()
	// Uses ratelimit test helper WithPerTokenLimits which was added
	// alongside this test file — see ratelimit/per_token_hooks.go.
	ratelimit.SetPerTokenLimitsForTest(rl, maxConc, int64(maxWin))
}
