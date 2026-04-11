//go:build otel

// otel.go is only compiled when -tags=otel is passed to `go build`. It
// wires an OTLP HTTP exporter + W3C trace-context propagator and replaces
// tracing.Default with an OpenTelemetry-backed tracer.
//
// This file is the only place in the codebase that imports
// go.opentelemetry.io/* packages, so `go build` (no tags) produces a
// binary with zero otel symbols. Enforced by the `verify-no-otel-default`
// CI job which runs `go tool nm` and asserts the substring is absent.

package tracing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// otelTracer is the OpenTelemetry-backed tracer installed under -tags=otel.
type otelTracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	propag   propagation.TextMapPropagator
}

type otelSpan struct{ s trace.Span }

func (o otelSpan) SetAttribute(key string, value any) {
	o.s.SetAttributes(attributeFrom(key, value))
}
func (o otelSpan) RecordError(err error) {
	if err == nil {
		return
	}
	o.s.RecordError(err)
}
func (o otelSpan) End() { o.s.End() }

func attributeFrom(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

func (o *otelTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	ctx, span := o.tracer.Start(ctx, name)
	return ctx, otelSpan{s: span}
}

func (o *otelTracer) InjectHTTPHeaders(ctx context.Context, headers map[string][]string) {
	carrier := propagation.HeaderCarrier(headers)
	o.propag.Inject(ctx, carrier)
}

func (o *otelTracer) Shutdown(ctx context.Context) error {
	return o.provider.Shutdown(ctx)
}

// init replaces tracing.Default with an OTLP-backed tracer when the `otel`
// build tag is set and OTEL_EXPORTER_OTLP_ENDPOINT is configured. Failing
// to construct the exporter falls back silently to the no-op tracer so a
// misconfigured deployment does not crash the whole process.
func init() {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		return
	}
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("clockify-mcp"),
		),
	)
	if err != nil {
		return
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	SetDefault(&otelTracer{
		provider: provider,
		tracer:   provider.Tracer("clockify-mcp"),
		propag:   otel.GetTextMapPropagator(),
	})
}
