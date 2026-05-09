package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
)

const maxLoggedPanicStackFrames = 8

var (
	stackSanitizeGOROOTOnce sync.Once
	stackSanitizeGOROOT     string
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
	stack := sanitizedDebugStack()
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

func sanitizedDebugStack() string {
	return sanitizePanicStack(string(debug.Stack()))
}

func sanitizePanicStack(stack string) string {
	if strings.TrimSpace(stack) == "" {
		return stack
	}
	lines := strings.Split(stack, "\n")
	out := make([]string, 0, 1+maxLoggedPanicStackFrames*2)
	frames := 0
	for i, line := range lines {
		if i == 0 {
			out = append(out, line)
			continue
		}
		if line == "" {
			continue
		}
		if isStackFunctionLine(line) {
			if frames >= maxLoggedPanicStackFrames {
				out = append(out, "... stack truncated ...")
				break
			}
			frames++
			out = append(out, line)
			continue
		}
		if frames > 0 && frames <= maxLoggedPanicStackFrames {
			out = append(out, sanitizeStackPathLine(line))
		}
	}
	return strings.Join(out, "\n")
}

func isStackFunctionLine(line string) bool {
	return !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ")
}

func sanitizeStackPathLine(line string) string {
	replacements := make([]struct {
		prefix string
		label  string
	}, 0, 4)
	if goroot := goRootForStackSanitization(); goroot != "" {
		replacements = append(replacements, struct {
			prefix string
			label  string
		}{prefix: goroot, label: "$GOROOT"})
	}
	for _, gp := range filepath.SplitList(os.Getenv("GOPATH")) {
		replacements = append(replacements, struct {
			prefix string
			label  string
		}{prefix: gp, label: "$GOPATH"})
	}
	if wd, err := os.Getwd(); err == nil {
		replacements = append(replacements, struct {
			prefix string
			label  string
		}{prefix: wd, label: "$PWD"})
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements = append(replacements, struct {
			prefix string
			label  string
		}{prefix: home, label: "$HOME"})
	}
	for _, repl := range replacements {
		prefix := filepath.Clean(repl.prefix)
		if prefix == "." || prefix == "" {
			continue
		}
		line = strings.ReplaceAll(line, prefix, repl.label)
	}
	return line
}

func goRootForStackSanitization() string {
	stackSanitizeGOROOTOnce.Do(func() {
		if goroot := strings.TrimSpace(os.Getenv("GOROOT")); goroot != "" {
			stackSanitizeGOROOT = goroot
			return
		}
		goBin, err := exec.LookPath("go")
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, goBin, "env", "GOROOT").Output()
		if err != nil {
			return
		}
		stackSanitizeGOROOT = strings.TrimSpace(string(out))
	})
	return stackSanitizeGOROOT
}
