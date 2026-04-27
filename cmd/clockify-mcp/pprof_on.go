//go:build pprof

package main

import (
	"log/slog"
	"net/http"
	// Side-import registers /debug/pprof/* handlers on http.DefaultServeMux.
	// This is the ONLY file in the repo that imports net/http/pprof; the
	// //go:build pprof tag ensures the default build has zero pprof symbols
	// (enforced by the nm-gate in .github/workflows/ci.yml).
	_ "net/http/pprof"

	"github.com/apet97/go-clockify/internal/mcp"
)

// pprofExtras returns a single-entry ExtraHandler slice that mounts
// /debug/pprof/* on whichever HTTP transport is active. Called once from
// run() before ServeHTTP / ServeStreamableHTTP.
//
// Security: pprof endpoints are NOT gated behind the bearer token because
// they live outside the /mcp handler. The whole point of the build tag is
// that production binaries never link pprof; debug builds must only run on
// trusted networks (loopback or firewalled) — `/debug/pprof/heap` and
// `/debug/pprof/goroutine` leak process layout, allocation patterns, and
// goroutine stacks (including handler frame strings) to anyone who can
// reach the listener.
func pprofExtras() []mcp.ExtraHandler {
	slog.Warn("pprof_enabled",
		"hint", "build tag pprof mounted /debug/pprof/* — do not run in production",
	)
	return []mcp.ExtraHandler{
		{Pattern: "/debug/pprof/", Handler: http.DefaultServeMux},
	}
}
