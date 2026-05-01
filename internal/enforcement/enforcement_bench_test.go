package enforcement

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/truncate"
)

var benchAfterCallSink any

type benchEnvelope struct {
	OK     bool           `json:"ok"`
	Action string         `json:"action"`
	Data   any            `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

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

// BenchmarkPipelineAfterCallSmallResult measures the default successful
// tools/call post-processing path when truncation is enabled but the typed
// result is well within budget.
func BenchmarkPipelineAfterCallSmallResult(b *testing.B) {
	p := &Pipeline{Truncation: truncate.Config{Enabled: true, TokenBudget: 8000}}
	result := benchEnvelope{
		OK:     true,
		Action: "clockify_quick_report",
		Data: map[string]any{
			"entries":      3,
			"totalSeconds": 5400,
			"topProject":   "MCP runtime",
		},
		Meta: map[string]any{"workspaceId": "workspace-123"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		out, err := p.AfterCall(result)
		if err != nil {
			b.Fatalf("AfterCall: %v", err)
		}
		benchAfterCallSink = out
	}
}

// BenchmarkPipelineAfterCallLargeResult keeps the over-budget path covered:
// typed results still need to be converted into a generic JSON tree before
// truncate.Truncate can walk and reduce nested arrays.
func BenchmarkPipelineAfterCallLargeResult(b *testing.B) {
	p := &Pipeline{Truncation: truncate.Config{Enabled: true, TokenBudget: 200}}
	items := make([]any, 500)
	for i := range items {
		items[i] = map[string]any{
			"id":   i,
			"text": strings.Repeat("x", 200),
		}
	}
	result := benchEnvelope{
		OK:     true,
		Action: "clockify_detailed_report",
		Data:   items,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		out, err := p.AfterCall(result)
		if err != nil {
			b.Fatalf("AfterCall: %v", err)
		}
		benchAfterCallSink = out
	}
}
