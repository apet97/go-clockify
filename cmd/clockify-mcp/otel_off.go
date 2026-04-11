//go:build !otel

package main

import "context"

// installOTel is the default-build stub. Returns a no-op shutdown so the
// caller in run() can unconditionally `defer shutdown()`. Rebuild with
// `go build -tags=otel` to link the OTel sub-module and wire an OTLP
// exporter via OTEL_EXPORTER_OTLP_ENDPOINT. See ADR 009.
func installOTel(_ context.Context) func() { return func() {} }
