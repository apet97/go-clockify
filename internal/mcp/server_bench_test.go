package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// BenchmarkDispatchToolsList exercises the JSON-RPC parse → route →
// serialize path that every MCP request flows through. The tool list
// is the cheapest non-trivial method in the server: no enforcement
// pipeline interaction, no upstream HTTP calls, just unmarshal,
// dispatch, marshal. A regression in the dispatcher's internal
// routing would show up here before it shows up anywhere else.
//
// Run: go test -bench=BenchmarkDispatch -benchtime=10x ./internal/mcp
func BenchmarkDispatchToolsList(b *testing.B) {
	server := newBenchServer(b, 5)
	server.initialized.Store(true) // skip the initialize handshake

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

// BenchmarkDispatchToolsListLarge measures tools/list with a registry size
// close to the full Clockify surface after Tier-2 activation. It keeps the
// same synthetic no-op descriptors as BenchmarkDispatchToolsList to avoid an
// import cycle from mcp back into internal/tools.
func BenchmarkDispatchToolsListLarge(b *testing.B) {
	server := newBenchServer(b, 124)
	server.initialized.Store(true)

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

func BenchmarkBuildToolList(b *testing.B) {
	server := newBenchServer(b, 156)
	for name, descriptor := range server.tools {
		if descriptor.Tool.Annotations == nil {
			descriptor.Tool.Annotations = map[string]any{}
		}
		descriptor.Tool.Annotations["priority"] = len(name) % 100
		server.tools[name] = descriptor
	}

	b.ReportAllocs()
	for b.Loop() {
		server.mu.Lock()
		tools := server.buildToolListLocked()
		server.mu.Unlock()
		if len(tools) != 156 {
			b.Fatalf("buildToolListLocked returned %d tools", len(tools))
		}
	}
}

func TestToolsListRepeatedCallsReuseSerializedResult(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "bench_tool", Description: "bench-only no-op"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{"ok": true}, nil
		},
		ReadOnlyHint: true,
	}})

	server.MarkInitialized(SupportedProtocolVersions[0], "test", "0")

	ctx := context.Background()
	for _, id := range []int{1, 2} {
		msg := mustMarshalRequestForTest(t, Request{
			JSONRPC: "2.0",
			ID:      id,
			Method:  "tools/list",
			Params:  map[string]any{},
		})
		if _, err := server.DispatchMessage(ctx, msg); err != nil {
			t.Fatalf("tools/list id=%d: %v", id, err)
		}
	}

	server.mu.RLock()
	valid := server.toolListResultJSONValid
	cached := append([]byte(nil), server.toolListResultJSON...)
	server.mu.RUnlock()
	if !valid {
		t.Fatal("tools/list serialized result cache was not populated")
	}
	if !json.Valid(cached) || !strings.Contains(string(cached), `"bench_tool"`) {
		t.Fatalf("cached tools/list result is not the expected JSON: %s", cached)
	}
}

// BenchmarkDispatchInitialize covers the protocol entry point.
// initialize runs once per session in production, but it's the
// canonical "small request, small response" measurement and the
// natural baseline against which tools/list is compared.
func BenchmarkDispatchInitialize(b *testing.B) {
	server := newBenchServer(b, 0)

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "bench", "version": "0"},
		},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Reset the initialized flag every iteration so we measure the
		// real handshake path, not a no-op replay.
		server.initialized.Store(false)
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

// BenchmarkDispatchToolsCall exercises the full tools/call path against
// a no-op handler. Unlike BenchmarkDispatchToolsList this bench decodes
// ToolCallParams out of the request envelope, so it's the canonical
// measurement for the decodeParams optimization in G3a. A regression
// in the marshal→unmarshal roundtrip, the callTool dispatch, or the
// result envelope assembly surfaces here first.
//
// The handler returns a fixed map to keep mustJSON's cost stable
// between runs — variation in marshaled payload size would obscure
// the dispatcher-level delta we're trying to measure.
func BenchmarkDispatchToolsCall(b *testing.B) {
	quietSlogForBenchmark(b)

	server := newBenchServer(b, 1)
	server.initialized.Store(true)

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: ToolCallParams{
			Name:      "bench_tool",
			Arguments: map[string]any{"k": "v"},
		},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

func BenchmarkDispatchToolsCallAliasArgument(b *testing.B) {
	quietSlogForBenchmark(b)

	server := newAliasBenchServer(b)
	server.initialized.Store(true)

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: ToolCallParams{
			Name:      "bench_tool",
			Arguments: map[string]any{"client_id": "client-1"},
		},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

func BenchmarkDispatchToolsCallLargePayload(b *testing.B) {
	quietSlogForBenchmark(b)

	payload := largeToolPayload()
	server := newPayloadBenchServer(b, payload)
	server.initialized.Store(true)

	msg := mustMarshalRequest(b, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: ToolCallParams{
			Name:      "bench_tool",
			Arguments: map[string]any{"include_entries": true},
		},
	})

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err := server.DispatchMessage(ctx, msg)
		if err != nil {
			b.Fatalf("DispatchMessage: %v", err)
		}
	}
}

// newBenchServer builds a minimal Server with `n` no-op tool
// descriptors. Enforcement and Activator are nil — per the docs on
// mcp.NewServer that means "no filtering" — so the bench measures
// pure dispatcher overhead without enforcement-layer noise.
func newBenchServer(b *testing.B, n int) *Server {
	b.Helper()
	descriptors := make([]ToolDescriptor, 0, n)
	for i := range n {
		name := "bench_tool"
		if i > 0 {
			name = fmt.Sprintf("bench_tool_%03d", i)
		}
		descriptors = append(descriptors, ToolDescriptor{
			Tool: Tool{
				Name:        name,
				Description: "bench-only no-op",
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"ok": true}, nil
			},
			ReadOnlyHint: true,
		})
	}
	return NewServer("bench", descriptors)
}

func newPayloadBenchServer(b *testing.B, payload any) *Server {
	b.Helper()
	return NewServer("bench", []ToolDescriptor{{
		Tool: Tool{
			Name:        "bench_tool",
			Description: "bench-only large payload",
		},
		Handler: func(context.Context, map[string]any) (any, error) {
			return payload, nil
		},
		ReadOnlyHint: true,
	}})

}

func newAliasBenchServer(b *testing.B) *Server {
	b.Helper()
	return NewServer("bench", []ToolDescriptor{{
		Tool: Tool{
			Name:        "bench_tool",
			Description: "bench-only alias schema",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []any{"client"},
				"properties": map[string]any{
					"client":    map[string]any{"type": "string"},
					"client_id": map[string]any{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{"ok": true}, nil
		},
		ReadOnlyHint: true,
	}})
}

func largeToolPayload() map[string]any {
	entries := make([]map[string]any, 512)
	description := strings.Repeat("x", 512)
	for i := range entries {
		entries[i] = map[string]any{
			"id":          fmt.Sprintf("entry-%04d", i),
			"description": description,
			"project":     "large-workspace",
			"duration":    3600 + i,
			"billable":    i%2 == 0,
		}
	}
	return map[string]any{
		"ok":     true,
		"action": "bench_large_payload",
		"data": map[string]any{
			"entries": entries,
			"totals":  map[string]any{"entries": len(entries), "seconds": len(entries) * 3600},
		},
	}
}

func mustMarshalRequest(b *testing.B, req Request) []byte {
	b.Helper()
	out, err := json.Marshal(req)
	if err != nil {
		b.Fatalf("marshal request: %v", err)
	}
	return out
}

func mustMarshalRequestForTest(t *testing.T, req Request) []byte {
	t.Helper()
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return out
}

func quietSlogForBenchmark(b *testing.B) {
	b.Helper()

	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Cleanup(func() {
		slog.SetDefault(prev)
	})
}
