// Package otel is the OpenTelemetry-backed tracer implementation for
// clockify-mcp. It lives in a dedicated Go sub-module so the top-level
// github.com/apet97/go-clockify go.mod carries zero OTel rows, preserving
// the "stdlib-only default build" invariant documented in ADR 001 and
// closed by ADR 009.
//
// Callers wire this package by passing `-tags=otel` to `go build` — the
// build-tagged file cmd/clockify-mcp/otel_on.go then imports this package
// and calls Install during startup. Under the default build,
// cmd/clockify-mcp/otel_off.go returns a no-op stub and the sub-module is
// never compiled.
package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/apet97/go-clockify/internal/tracing"
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

func (o *otelTracer) Start(ctx context.Context, name string) (context.Context, tracing.Span) {
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

// Install wires an OTLP HTTP exporter, registers an OpenTelemetry-backed
// tracer as tracing.Default, and returns a shutdown closure. Callers must
// defer the closure during graceful shutdown. The exporter reads its
// endpoint from the standard OTEL_EXPORTER_OTLP_* env vars (as interpreted
// by otlptracehttp.New); Install propagates any exporter-construction error
// back to the caller so a misconfigured deployment surfaces immediately
// instead of silently running without tracing.
func Install(ctx context.Context) (func(), error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otel: create exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("clockify-mcp"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: create resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tracing.SetDefault(&otelTracer{
		provider: provider,
		tracer:   provider.Tracer("clockify-mcp"),
		propag:   otel.GetTextMapPropagator(),
	})
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Shutdown(shutdownCtx)
	}, nil
}
