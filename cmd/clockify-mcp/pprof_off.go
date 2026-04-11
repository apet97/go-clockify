//go:build !pprof

package main

import "github.com/apet97/go-clockify/internal/mcp"

// pprofExtras is the default-build stub. Returns nil so the transport's
// mountExtras helper is a no-op and net/http/pprof is never linked.
// Rebuild with `go build -tags=pprof` to expose /debug/pprof/* on the
// HTTP transport (see docs/runbooks/oom-or-goroutine-leak.md).
func pprofExtras() []mcp.ExtraHandler { return nil }
