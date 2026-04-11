package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newPerTokenLimiter(maxConcurrent int, maxWindow int64, perTokenConcurrent int, perTokenWindow int64) *RateLimiter {
	rl := NewWithAcquireTimeout(maxConcurrent, maxWindow, 60000, 20*time.Millisecond)
	rl.perTokenMaxConcurrent = perTokenConcurrent
	rl.perTokenMaxPerWindow = perTokenWindow
	return rl
}

func TestAcquireForSubjectIsolatesSubjects(t *testing.T) {
	// Global budget comfortably wide; per-subject budget tight.
	rl := newPerTokenLimiter(100, 1000, 0, 2)

	// Subject A burns both of its window slots.
	rel1, scope, err := rl.AcquireForSubject(context.Background(), "alice")
	if err != nil || scope != ScopePerToken {
		t.Fatalf("acquire 1: %v scope=%s", err, scope)
	}
	defer rel1()
	rel2, _, err := rl.AcquireForSubject(context.Background(), "alice")
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	defer rel2()

	// Third call from alice should fail the per-token window.
	_, scope, err = rl.AcquireForSubject(context.Background(), "alice")
	if err == nil {
		t.Fatal("expected per-token rejection for alice")
	}
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("wrong error: %v", err)
	}
	if scope != ScopePerToken {
		t.Fatalf("scope: %s", scope)
	}

	// Bob should still get through — isolation.
	rel3, _, err := rl.AcquireForSubject(context.Background(), "bob")
	if err != nil {
		t.Fatalf("bob acquire: %v", err)
	}
	defer rel3()
}

func TestAcquireForSubjectFallsBackToGlobalWhenSubjectEmpty(t *testing.T) {
	rl := newPerTokenLimiter(0, 2, 0, 1)
	// Subject="" disables the per-token layer — two calls should pass even
	// though perTokenMaxPerWindow is 1.
	rel1, scope, err := rl.AcquireForSubject(context.Background(), "")
	if err != nil || scope != ScopeGlobal {
		t.Fatalf("1: %v scope=%s", err, scope)
	}
	defer rel1()
	rel2, _, err := rl.AcquireForSubject(context.Background(), "")
	if err != nil {
		t.Fatalf("2: %v", err)
	}
	defer rel2()

	// Third fails on the GLOBAL window (2/window).
	_, scope, err = rl.AcquireForSubject(context.Background(), "")
	if err == nil {
		t.Fatal("expected global rejection")
	}
	if scope != ScopeGlobal {
		t.Fatalf("scope should be global: %s", scope)
	}
}

func TestAcquireForSubjectRespectsGlobalCap(t *testing.T) {
	// Global 1/window, per-token permissive.
	rl := newPerTokenLimiter(0, 1, 0, 100)
	rel, scope, err := rl.AcquireForSubject(context.Background(), "alice")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer rel()
	if scope != ScopePerToken {
		t.Fatalf("scope expected per_token when subject is non-empty: %s", scope)
	}

	// Second call by bob must be blocked by the global cap even though
	// bob's own per-token budget is untouched.
	_, scope, err = rl.AcquireForSubject(context.Background(), "bob")
	if err == nil {
		t.Fatal("expected global rejection")
	}
	if scope != ScopeGlobal {
		t.Fatalf("scope should be global: %s", scope)
	}
}

func TestAcquireForSubjectDisabledPerTokenLayer(t *testing.T) {
	rl := NewWithAcquireTimeout(100, 1000, 60000, 20*time.Millisecond)
	// Per-token not configured — layer disabled; scope returns ScopeGlobal
	// regardless of subject value.
	rel, scope, err := rl.AcquireForSubject(context.Background(), "alice")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer rel()
	if scope != ScopeGlobal {
		t.Fatalf("scope: %s", scope)
	}
}
