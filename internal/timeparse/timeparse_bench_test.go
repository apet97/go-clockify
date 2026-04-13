package timeparse

import (
	"testing"
	"time"
)

// BenchmarkParseDatetime exercises the production hot path: every tool
// that accepts a "start"/"end" parameter routes through ParseDatetime
// before reaching the Clockify client. A regression here multiplies
// across every dispatched call.
//
// The corpus mixes the natural-language shortcuts that dominate real
// usage with the strict ISO-8601 form that machine clients send, so
// the benchmark catches regressions in either path.
//
// Run:    go test -bench=BenchmarkParseDatetime -benchtime=10x ./internal/timeparse
// Compare: benchstat across runs (see docs/performance.md).
func BenchmarkParseDatetime(b *testing.B) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		b.Fatalf("LoadLocation: %v", err)
	}
	corpus := []string{
		"now",
		"today",
		"yesterday",
		"2026-04-13T00:00:00Z",
		"2026-04-13",
		"2026-04-13T15:30:00-04:00",
		"PT1H",
		"P1D",
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		input := corpus[i%len(corpus)]
		_, _ = ParseDatetime(input, loc)
		i++
	}
}

// BenchmarkParseDuration covers the second hot path — every duration
// argument (entry length, time-off duration, billable rate window)
// routes through this function before hitting the Clockify client.
func BenchmarkParseDuration(b *testing.B) {
	corpus := []string{
		"PT1H",
		"PT30M",
		"PT1H30M",
		"P1D",
		"P1DT2H",
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		input := corpus[i%len(corpus)]
		_, _ = ParseDuration(input)
		i++
	}
}
