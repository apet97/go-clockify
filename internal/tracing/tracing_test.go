package tracing

import (
	"context"
	"errors"
	"testing"
)

func TestNoopTracerIsDefault(t *testing.T) {
	// Under the default (no-otel) build, Default must be the noop tracer.
	// Under the otel build the `init()` in otel.go may have replaced it
	// if OTEL_EXPORTER_OTLP_ENDPOINT is set, but in test runs we do not
	// set that env var so it remains the noop tracer.
	if _, ok := Default.(noopTracer); !ok {
		t.Skipf("Default is %T (otel build with exporter installed) — skipping noop-identity assertion", Default)
	}
}

func TestNoopTracerStartReturnsUsableSpan(t *testing.T) {
	ctx := context.Background()
	outCtx, span := Default.Start(ctx, "noop.probe")
	if outCtx != ctx {
		t.Fatal("noop Start must return the same ctx")
	}
	if span == nil {
		t.Fatal("span must not be nil")
	}
	// Exercise every Span method — noop must accept all without panicking.
	span.SetAttribute("k", "v")
	span.SetAttribute("n", 42)
	span.SetAttribute("b", true)
	span.RecordError(errors.New("boom"))
	span.RecordError(nil)
	span.End()
}

func TestSetDefaultNilFallsBackToNoop(t *testing.T) {
	original := Default
	defer func() { Default = original }()

	// Install a recording tracer, then nil out — should revert to noop.
	SetDefault(&stubTracer{})
	if _, ok := Default.(noopTracer); ok {
		t.Fatal("SetDefault(stub) should replace Default")
	}
	SetDefault(nil)
	if _, ok := Default.(noopTracer); !ok {
		t.Fatalf("SetDefault(nil) should revert to noopTracer, got %T", Default)
	}
}

func TestInjectHTTPHeadersNoop(t *testing.T) {
	headers := map[string][]string{"X-Existing": {"v"}}
	Default.InjectHTTPHeaders(context.Background(), headers)
	if _, added := headers["Traceparent"]; added {
		t.Fatal("noop InjectHTTPHeaders should not add Traceparent")
	}
}

func TestShutdownNoop(t *testing.T) {
	if err := Default.Shutdown(context.Background()); err != nil {
		t.Fatalf("noop Shutdown: %v", err)
	}
}

// stubTracer is a test-only tracer used to verify SetDefault replaces the
// package-level Default. It records nothing — the identity check is the
// only assertion.
type stubTracer struct{}

func (stubTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}
func (stubTracer) InjectHTTPHeaders(_ context.Context, _ map[string][]string) {}
func (stubTracer) Shutdown(_ context.Context) error                           { return nil }
