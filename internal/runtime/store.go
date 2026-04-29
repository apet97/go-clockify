package runtime

import (
	"fmt"
	"os"

	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
)

// BuildStore opens the control-plane store for the given config and
// enforces the C1 fail-closed guard as defence-in-depth: streamable_http
// or grpc against a dev-only backend (memory/file) requires
// MCP_ALLOW_DEV_BACKEND=1. The primary guard lives in config.Load() so
// operators see the error at startup, not at first request; this
// second check catches any caller that bypasses Load().
//
// Both transports are deployed multi-replica behind a load balancer in
// production, and the private-network-grpc profile pairs grpc with
// fail_closed audit — a memory backend cannot honour fail_closed across
// pod restarts. The predicate matches config.Load() exactly.
//
// The file-backed store honours cfg.ControlPlaneAuditCap; the
// Postgres backend ignores it and relies on the B2 retention reaper
// instead (see RetainAuditLoop).
//
// Extracted from cmd/clockify-mcp/main.go as part of C2 so the
// control-plane wiring is unit-testable and reusable across
// transports.
func BuildStore(cfg config.Config) (controlplane.Store, error) {
	if (cfg.Transport == "streamable_http" || cfg.Transport == "grpc") &&
		config.IsDevControlPlaneDSN(cfg.ControlPlaneDSN) &&
		os.Getenv("MCP_ALLOW_DEV_BACKEND") != "1" {
		return nil, fmt.Errorf("MCP_TRANSPORT=%q with MCP_CONTROL_PLANE_DSN=%q (dev backend) is disallowed by default; set MCP_ALLOW_DEV_BACKEND=1 to acknowledge the single-process limits, or point MCP_CONTROL_PLANE_DSN at a production backend", cfg.Transport, cfg.ControlPlaneDSN)
	}
	return controlplane.Open(cfg.ControlPlaneDSN,
		controlplane.WithAuditCap(cfg.ControlPlaneAuditCap))
}
