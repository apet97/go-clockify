//go:build !pprof

package main

import "github.com/apet97/go-clockify/internal/mcp"

// pprofExtras is the default-build stub. Returns nil so the transport's
// mountExtras helper is a no-op and net/http/pprof is never linked.
// Rebuild with `go build -tags=pprof` to expose /debug/pprof/* on the
// HTTP transport — only on a trusted network (loopback or firewalled),
// since the pprof handlers are not gated by the bearer token. See the
// `pprofExtras` doc in pprof_on.go for the full security caveat.
func pprofExtras() []mcp.ExtraHandler { return nil }
