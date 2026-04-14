package ratelimit

import (
	"context"
	"testing"
)

// BenchmarkAcquireForSubjectSteady measures the steady-state cost of
// the three-tier limiter when the caller is well below both global and
// per-token caps. Each iteration traverses the global window check +
// concurrency semaphore + per-subject map lookup + per-subject window
// check — the full happy path for an authenticated production call.
func BenchmarkAcquireForSubjectSteady(b *testing.B) {
	rl := New(10000, 1000000, 60000)
	rl.SetPerTokenLimits(1000, 100000)
	const subject = "bench-subject"
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		release, _, err := rl.AcquireForSubject(ctx, subject)
		if err != nil {
			b.Fatalf("AcquireForSubject: %v", err)
		}
		if release != nil {
			release()
		}
	}
}

// BenchmarkAcquireForSubjectNoPerToken measures the global-only path
// (empty subject → per-token sub-layer skipped). This is the floor
// cost of the limiter and the baseline for measuring per-token overhead.
func BenchmarkAcquireForSubjectNoPerToken(b *testing.B) {
	rl := New(10000, 1000000, 60000)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		release, _, err := rl.AcquireForSubject(ctx, "")
		if err != nil {
			b.Fatalf("AcquireForSubject: %v", err)
		}
		if release != nil {
			release()
		}
	}
}

// BenchmarkAcquireGlobal isolates the global-only Acquire path without
// going through AcquireForSubject. Useful as a delta against
// BenchmarkAcquireForSubjectNoPerToken to attribute the subject-branch
// overhead.
func BenchmarkAcquireGlobal(b *testing.B) {
	rl := New(10000, 1000000, 60000)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		release, err := rl.Acquire(ctx)
		if err != nil {
			b.Fatalf("Acquire: %v", err)
		}
		if release != nil {
			release()
		}
	}
}
