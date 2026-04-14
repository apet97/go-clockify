package otel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/tracing"
)

// TestInstallEmitsSpanToOTLPEndpoint is a CI regression gate for the
// otel sub-module's exporter pipeline. check-build-tags.sh and the
// default `go test -tags=otel ./internal/tracing/...` call exercise the
// facade (no-op vs real), the symbol-presence check, and the build-tag
// wiring -- but nothing actually boots the exporter, emits a span, and
// verifies that a POST landed on an OTLP /v1/traces endpoint. Any of the
// following could break and leave CI green:
//
//   - exporter initialisation silently misconfigured (wrong endpoint
//     parsing, wrong propagator),
//   - batcher never flushed (Shutdown() returns nil without waiting),
//   - span dropped before export because SetTracerProvider is racing
//     with Start().
//
// The test stands up an in-process HTTP server as a fake OTLP collector,
// sets the OTLP env vars to point at it, installs the tracer, emits one
// span via the tracing facade (the same facade internal/mcp/server.go
// and internal/clockify/client.go use), invokes the Install-returned
// shutdown hook to flush, and asserts that at least one POST with a
// request body landed on /v1/traces.
//
// This lives in the sub-module (not internal/tracing) because the
// sub-module has the OTel dependencies; the top-level module has zero
// by design (ADR 001 / ADR 009). It is picked up automatically by
// scripts/check-build-tags.sh's sub-module test pass.
func TestInstallEmitsSpanToOTLPEndpoint(t *testing.T) {
	var (
		exportCount int64
		bodySeen    int64
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The OTLP HTTP exporter posts to <endpoint>/v1/traces.
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v1/traces") {
			atomic.AddInt64(&exportCount, 1)
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 {
				atomic.AddInt64(&bodySeen, 1)
			}
		}
		// A 200 with empty body is accepted by otlptracehttp as a
		// successful export for the purposes of this test. We do not
		// decode the protobuf payload -- the structural assertion is
		// that SOME bytes arrived, not that the encoding is byte-identical
		// to the spec.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Point the OTLP HTTP exporter at the httptest server. The URL is
	// http://, so otlptracehttp sees the scheme and drops into insecure
	// mode automatically.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	// Short export interval so TestMain doesn't have to wait the
	// default 5s batch window. The batcher also flushes on Shutdown(),
	// which is what we actually rely on.
	t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "100")

	ctx := context.Background()
	shutdown, err := Install(ctx)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Emit one span via the tracing facade. This is the SAME path
	// internal/mcp/server.go:749 and internal/clockify/client.go:176
	// take, so if this test passes, the real server's spans will also
	// land on the real OTLP endpoint.
	_, span := tracing.Default.Start(ctx, "test.span")
	span.SetAttribute("tool.name", "clockify_test")
	span.SetAttribute("outcome", "ok")
	span.End()

	// Flush the batcher. Shutdown() drains any pending spans before
	// returning; if it does not, the export count below will stay at 0
	// and the test fails loud.
	shutdown()

	// Shutdown returns synchronously, but the httptest handler runs in
	// a separate goroutine and the batcher may post after Shutdown's
	// caller has returned in some SDK versions. Poll for up to 5 s so
	// the test is robust to that timing without being flaky in the
	// happy path (which usually completes in <100 ms).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&exportCount) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := atomic.LoadInt64(&exportCount); got == 0 {
		t.Fatalf("no OTLP export observed; exporter pipeline is broken " +
			"(no POST to /v1/traces arrived within 5s of span emission + shutdown)")
	}
	if got := atomic.LoadInt64(&bodySeen); got == 0 {
		t.Fatalf("OTLP POST arrived but had zero body bytes; span was not serialised")
	}
}
