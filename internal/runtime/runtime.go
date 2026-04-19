// Package runtime gathers the boot-time scaffolding for clockify-mcp
// that used to live inline in cmd/clockify-mcp/main.go: control-plane
// selection, dev-backend predicate, and the background audit-retention
// reaper. C2 extracts these so main.go can eventually shrink to a
// thin entrypoint; the transport dispatch itself is still in main for
// now and will follow in a later pass.
//
// The package deliberately stays below the transports in the import
// graph — internal/mcp, internal/authn, internal/controlplane may all
// import runtime but not vice versa. Keep it that way.
package runtime

import (
	"strings"
)

// IsDevControlPlaneDSN reports whether dsn names one of the dev-only
// control-plane backends. "memory" / "memory://" keep state in process
// memory; a bare path or "file://..." rewrites a JSON file on every
// mutation. Neither is correct for a multi-process production
// deployment of streamable_http; C1 fails closed unless the operator
// acknowledges the tradeoff via MCP_ALLOW_DEV_BACKEND=1.
//
// Extracted from cmd/clockify-mcp so other packages (runtime builders,
// tests, future transports) can share the same predicate without
// re-importing main.
func IsDevControlPlaneDSN(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" || trimmed == "memory" || trimmed == "memory://" {
		return true
	}
	if strings.HasPrefix(trimmed, "file://") {
		return true
	}
	// A bare path with no scheme (resolvePath accepts these) is also a
	// file-backed store — treat it the same as file://.
	if !strings.Contains(trimmed, "://") {
		return true
	}
	return false
}
