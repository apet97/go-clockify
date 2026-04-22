package config

import "strings"

// IsDevControlPlaneDSN reports whether dsn names one of the dev-only
// control-plane backends. "memory" / "memory://" keep state in process
// memory; a bare path or "file://..." rewrites a JSON file on every
// mutation. Neither is correct for a multi-process production
// deployment of streamable_http.
//
// This predicate is the single source of truth for the fail-closed
// guard in Load() and the defence-in-depth guard in
// internal/runtime.BuildStore. Both callers refuse to run
// streamable_http against a dev DSN unless MCP_ALLOW_DEV_BACKEND=1
// is explicit — Load() catches it at config time so operators see
// the error on startup instead of at first request.
func IsDevControlPlaneDSN(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" || trimmed == "memory" || trimmed == "memory://" {
		return true
	}
	if strings.HasPrefix(trimmed, "file://") {
		return true
	}
	// A bare path with no scheme (resolvePath accepts these) is also
	// a file-backed store — treat it the same as file://.
	if !strings.Contains(trimmed, "://") {
		return true
	}
	return false
}
