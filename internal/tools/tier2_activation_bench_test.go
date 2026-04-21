package tools_test

// Activation-path micro-benchmarks. These measure the cost of
// materialising a Tier-2 group's descriptor slice — the step that
// runs when a client activates a Tier-2 group via
// clockify_search_tools { "activate_group": "<group>" }. It is
// distinct from the dispatch benches in writes_bench_test.go and
// tier2_writes_bench_test.go, which exercise a full tools/call
// through the enforcement pipeline.
//
// The activation hot path matters because:
//
//   1. It runs on every group activation from every client — so a
//      per-activation slowdown amplifies across the fleet.
//   2. It allocates the descriptor slice that then gets passed to
//      server.AddTools, so allocation count here directly affects
//      the steady-state GC pressure of a long-lived server.
//
// The BenchmarkTier2ActivationLookup variant measures a cold lookup
// (the path a client actually takes). BenchmarkTier2ActivationAll
// iterates every group so the regression gate catches an allocation
// change anywhere in the registry, not just a specific group.
//
// Run locally:
//
//	go test -bench=BenchmarkTier2Activation -benchmem -benchtime=1s \
//	  -run='^$' ./internal/tools/...

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/tools"
)

// newActivationService stands up a bare Service{} with a cheap stub
// clockify client. No upstream is hit — the Tier-2 builders only
// read the Service fields to wire handlers; they don't call out.
func newActivationService(b *testing.B) *tools.Service {
	b.Helper()
	c := clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0)
	return tools.New(c, "ws1")
}

// BenchmarkTier2ActivationLookup measures the cost of one
// activation call for the invoices group, which is the most
// descriptor-heavy Tier-2 group (11 tools). Every iteration
// includes the map lookup in Tier2Groups, the builder call, the
// normalizeDescriptors pass, and the applyOpaqueOutputSchemas pass.
func BenchmarkTier2ActivationLookup(b *testing.B) {
	svc := newActivationService(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds, ok := svc.Tier2Handlers("invoices")
		if !ok || len(ds) == 0 {
			b.Fatalf("invoices group missing or empty")
		}
	}
}

// BenchmarkTier2ActivationAll iterates every registered Tier-2
// group in one iteration. A regression in any single group's
// builder surfaces here even if its group-specific bench isn't
// written. The operation count per iteration is len(Tier2Groups),
// so b.N should be read as "sweeps," not "activations."
func BenchmarkTier2ActivationAll(b *testing.B) {
	svc := newActivationService(b)
	names := make([]string, 0, len(tools.Tier2Groups))
	for n := range tools.Tier2Groups {
		names = append(names, n)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, n := range names {
			ds, ok := svc.Tier2Handlers(n)
			if !ok || len(ds) == 0 {
				b.Fatalf("group %s empty", n)
			}
		}
	}
}
