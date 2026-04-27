//go:build otel

package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	// Side-import of the OTel sub-module. This is the ONLY main-module file
	// that imports github.com/apet97/go-clockify/internal/tracing/otel; the
	// //go:build otel tag ensures the default build never links it — the
	// top-level go.mod has zero go.opentelemetry.io rows and the nm-gate in
	// .github/workflows/ci.yml enforces the symbol absence. See ADR 009.
	otelsub "github.com/apet97/go-clockify/internal/tracing/otel"
)

// installOTel is called once from run(). It reads OTEL_EXPORTER_OTLP_ENDPOINT
// as a gate — when unset, tracing stays on the stdlib-only no-op path. When
// set, it delegates to the sub-module's Install which wires the OTLP HTTP
// exporter and installs an OpenTelemetry-backed tracing.Default. The returned
// shutdown closure is always safe to call; on failure we log and return a
// no-op so a misconfigured exporter cannot crash the server.
func installOTel(ctx context.Context) func() {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		return func() {}
	}
	shutdown, err := otelsub.Install(ctx)
	if err != nil {
		slog.Warn("otel_install_failed",
			"error", err.Error(),
			"hint", "tracing will fall back to no-op; check OTEL_EXPORTER_OTLP_ENDPOINT and the local OTLP collector reachability",
		)
		return func() {}
	}
	slog.Info("otel_installed", "endpoint", endpoint)
	return shutdown
}
