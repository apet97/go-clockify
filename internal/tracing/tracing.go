// Package tracing is the stdlib-only tracing facade. The default build
// links an in-package no-op tracer that compiles without any OpenTelemetry
// dependencies. Building with `-tags=otel` replaces the no-op with a real
// OTLP exporter + W3C trace-context propagator.
//
// Design goals:
//   - `go build` (no tags) MUST produce a binary with zero opentelemetry.io
//     symbols. `go tool nm clockify-mcp | grep opentelemetry` returns empty.
//   - Tag-neutral call sites (server.callTool, clockify.client.doOnce)
//     import this package and nothing else; they can sprinkle Start/End/
//     SetAttribute calls that compile to no-ops in the default build.
//   - Production tracing is opt-in for users who explicitly rebuild with
//     `-tags=otel` and set OTLP env vars at runtime.
package tracing

import "context"

// Span is the tracing-facade span returned by Tracer.Start. Callers close
// the span with End() and may set attributes / record errors mid-flight.
type Span interface {
	SetAttribute(key string, value any)
	RecordError(err error)
	End()
}

// Tracer is the top-level tracing facade. Default is always non-nil —
// initialised to a no-op tracer in the tag-neutral path and optionally
// replaced by an OTLP-backed tracer when the `otel` build tag is set.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
	InjectHTTPHeaders(ctx context.Context, headers map[string][]string)
	Shutdown(ctx context.Context) error
}

// noopTracer is the default tracer used on every build. The otel build
// replaces Default at init time with an OTLP exporter; the default build
// stays no-op and carries zero opentelemetry.io symbols.
type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}
func (noopTracer) InjectHTTPHeaders(_ context.Context, _ map[string][]string) {}
func (noopTracer) Shutdown(_ context.Context) error                           { return nil }

type noopSpan struct{}

func (noopSpan) SetAttribute(_ string, _ any) {}
func (noopSpan) RecordError(_ error)          {}
func (noopSpan) End()                         {}

// Default is the package-level tracer every call site should use. Never nil.
// Call sites dereference Default on every Start — replacing Default after
// spans are live is not supported.
var Default Tracer = noopTracer{}

// SetDefault installs a new Tracer. Call at program startup before any
// traced code runs; concurrent replacement while spans are in flight is
// not supported.
func SetDefault(t Tracer) {
	if t == nil {
		Default = noopTracer{}
		return
	}
	Default = t
}
