package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

func TestAcquireForSubjectConcurrentWindowRolloverDoesNotOverAdmit(t *testing.T) {
	const goroutines = 64

	for attempt := 0; attempt < 50; attempt++ {
		rl := newPerTokenLimiter(0, 0, 0, 1)
		sub := rl.subjectLimiterFor("alice")
		sub.windowStart.Store(time.Now().Add(-time.Hour).UnixMilli())
		sub.windowCount.Store(1)

		var successes atomic.Int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, goroutines)

		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				<-start

				release, scope, err := rl.AcquireForSubject(context.Background(), "alice")
				if err != nil {
					if scope != ScopePerToken || !errors.Is(err, ErrRateLimitExceeded) {
						errs <- fmt.Errorf("unexpected rejection: scope=%s err=%v", scope, err)
					}
					return
				}
				if scope != ScopePerToken {
					errs <- fmt.Errorf("unexpected success scope: %s", scope)
					release()
					return
				}
				successes.Add(1)
				release()
			}()
		}

		close(start)
		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				t.Fatalf("attempt %d: %v", attempt, err)
			}
		}
		if got := successes.Load(); got != 1 {
			t.Fatalf("attempt %d: successful acquires = %d; want 1", attempt, got)
		}
	}
}
