package mcp

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketAllowsBurstUpToCapacityThenDenies(t *testing.T) {
	b := newTokenBucket(120)
	allowed := 0
	for i := 0; i < 120; i++ {
		if b.allow() {
			allowed++
		}
	}
	if allowed != 120 {
		t.Fatalf("burst allowed %d calls, want 120", allowed)
	}
	if b.allow() {
		t.Fatal("call 121 should be denied — the bucket is empty")
	}
}

func TestTokenBucketNilAlwaysAllows(t *testing.T) {
	var b *tokenBucket // newTokenBucket(0) returns nil
	if newTokenBucket(0) != nil {
		t.Fatal("newTokenBucket(0) should return nil (disabled)")
	}
	for i := 0; i < 5; i++ {
		if !b.allow() {
			t.Fatal("a nil token bucket must always allow")
		}
	}
}

func TestFamilyLimiterCapsWritesAtTwo(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	r1, err := l.acquire(ctx, RiskWrite)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	r2, err := l.acquire(ctx, RiskWrite)
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	// Third write must block until a slot frees.
	deadlineCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(deadlineCtx, RiskWrite); err == nil {
		t.Fatal("third concurrent write should have blocked past the cap")
	}
	r1()
	r2()
}

func TestFamilyLimiterSerializesHighRiskWrites(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	release, err := l.acquire(ctx, RiskDestructive)
	if err != nil {
		t.Fatalf("acquire high-risk: %v", err)
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(deadlineCtx, RiskBilling); err == nil {
		t.Fatal("a second concurrent high-risk write should have blocked (cap is 1)")
	}
	release()
	// After release a high-risk slot is free again.
	release2, err := l.acquire(ctx, RiskAdmin)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestFamilyLimiterDoesNotCapReads(t *testing.T) {
	l := newFamilyLimiter()
	ctx := context.Background()
	releases := make([]func(), 0, 10)
	for i := 0; i < 10; i++ {
		r, err := l.acquire(ctx, RiskRead)
		if err != nil {
			t.Fatalf("read acquire %d: %v", i, err)
		}
		releases = append(releases, r)
	}
	for _, r := range releases {
		r()
	}
}

func TestRateLimitedEnvelopeHasStructuredRecovery(t *testing.T) {
	env := rateLimitedEnvelope("clockify_status")
	if env["ok"] != false {
		t.Fatalf("ok = %v, want false", env["ok"])
	}
	if env["action"] != "clockify_status" {
		t.Fatalf("action = %v, want clockify_status", env["action"])
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok || errObj["code"] != "rate_limited" {
		t.Fatalf("error = %v, want code rate_limited", env["error"])
	}
	rec, ok := env["recovery"].(map[string]any)
	if !ok || rec["retryable"] != true || rec["hint"] == "" {
		t.Fatalf("recovery = %v, want a retryable hint", env["recovery"])
	}
}
