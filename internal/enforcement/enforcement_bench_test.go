package enforcement

import (
	"context"
	"math"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/ratelimit"
)

// BenchmarkPipelineBeforeCall measures the steady-state cost of the
// per-call enforcement pipeline: policy check + global rate-limit
// acquire + per-token scope resolution. Limits are set wide enough that
// the rate limiter never rejects — this isolates the CPU cost of the
// happy path from contention dynamics. This is the path every
// production tools/call traverses when an OIDC principal is on the
// context.
func BenchmarkPipelineBeforeCall(b *testing.B) {
	rl := ratelimit.New(10000, math.MaxInt64, 60000)
	rl.SetPerTokenLimits(1000, math.MaxInt64)
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
	}
	principal := &authn.Principal{Subject: "bench-subject"}
	ctx := authn.WithPrincipal(context.Background(), principal)
	hints := mcp.ToolHints{ReadOnly: true}
	args := map[string]any{}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, release, err := p.BeforeCall(ctx, "clockify_list_entries", args, hints, nil, noLookup)
		if err != nil {
			b.Fatalf("BeforeCall: %v", err)
		}
		if release != nil {
			release()
		}
	}
}

// BenchmarkPipelineBeforeCallNoPrincipal covers the unauthenticated
// fallback path where the per-token sub-layer is skipped entirely.
// This is the stdio transport's hot path when no authenticator is
// installed.
func BenchmarkPipelineBeforeCallNoPrincipal(b *testing.B) {
	rl := ratelimit.New(10000, math.MaxInt64, 60000)
	p := &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: rl,
	}
	ctx := context.Background()
	hints := mcp.ToolHints{ReadOnly: true}
	args := map[string]any{}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, release, err := p.BeforeCall(ctx, "clockify_list_entries", args, hints, nil, noLookup)
		if err != nil {
			b.Fatalf("BeforeCall: %v", err)
		}
		if release != nil {
			release()
		}
	}
}
