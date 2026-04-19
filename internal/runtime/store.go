package runtime

import (
	"fmt"
	"os"

	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
)

// BuildStore opens the control-plane store for the given config and
// enforces the C1 fail-closed guard: streamable_http against a
// dev-only backend (memory/file) requires MCP_ALLOW_DEV_BACKEND=1.
// The file-backed store honours cfg.ControlPlaneAuditCap; the
// Postgres backend ignores it and relies on the B2 retention reaper
// instead (see RetainAuditLoop).
//
// Extracted from cmd/clockify-mcp/main.go as part of C2 so the
// control-plane wiring is unit-testable and reusable across
// transports.
func BuildStore(cfg config.Config) (controlplane.Store, error) {
	if cfg.Transport == "streamable_http" &&
		IsDevControlPlaneDSN(cfg.ControlPlaneDSN) &&
		os.Getenv("MCP_ALLOW_DEV_BACKEND") != "1" {
		return nil, fmt.Errorf("MCP_TRANSPORT=streamable_http with MCP_CONTROL_PLANE_DSN=%q (dev backend) is disallowed by default; set MCP_ALLOW_DEV_BACKEND=1 to acknowledge the single-process limits, or point MCP_CONTROL_PLANE_DSN at a production backend", cfg.ControlPlaneDSN)
	}
	return controlplane.Open(cfg.ControlPlaneDSN,
		controlplane.WithAuditCap(cfg.ControlPlaneAuditCap))
}
