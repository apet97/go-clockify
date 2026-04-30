package mcp

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/apet97/go-clockify/internal/metrics"
)

// PanicResponseSink receives the JSON-RPC response built from a
// recovered panic. Callers supply a sink that pushes the response
// onto the active transport (writeResponse for stdio, json.Encoder
// for streamable HTTP, gRPC SendMsg pump).
type PanicResponseSink func(Response)

// RecoverDispatch is the deferred function that wraps a tool
// dispatch goroutine. It recovers a panic, emits a structured
// metric + log event, and (when a panic was caught) hands a stable
// JSON-RPC tool-error envelope to sink. No-op when the dispatch
// returns normally.
//
// Usage at every transport's tool-dispatch site:
//
//	defer mcp.RecoverDispatch(reqID, "stdio_tool_dispatch", toolName, sink)
//	resp := s.handle(ctx, r)
//	...
//
// Why a single helper rather than ad-hoc defer blocks at each site:
// stdio, streamable HTTP, and gRPC all need identical metric
// labelling, log shape, and response envelope. Centralising the
// behaviour keeps cross-transport parity tests honest and prevents
// regressions where one transport returns a leakier shape than
// another.
//
// recover() works here because RecoverDispatch IS the deferred
// function — `defer fn(...)` calls fn at defer-pop time, and
// recover() inside fn sees the panic. Wrapping this in an IIFE at
// the call site (`defer func(){ RecoverDispatch(...) }()`) would
// break that invariant.
func RecoverDispatch(reqID any, site, toolHint string, sink PanicResponseSink) {
	rec := recover()
	if rec == nil {
		return
	}
	metrics.PanicsRecoveredTotal.Inc(site)
	stack := string(debug.Stack())
	slog.Error("panic_recovered",
		"site", site,
		"tool", toolHint,
		"panic", fmt.Sprintf("%v", rec),
		"stack", stack,
	)
	if sink == nil {
		return
	}
	// Generic message — the panic value and stack are already in the
	// slog event above. Returning the raw recovered value to the
	// client risks leaking internal state, request data, or upstream
	// error strings; the client gets a stable identifier instead.
	sink(Response{
		JSONRPC: "2.0",
		ID:      reqID,
		Result: map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "internal tool error; request logged",
			}},
			"isError": true,
		},
	})
}
