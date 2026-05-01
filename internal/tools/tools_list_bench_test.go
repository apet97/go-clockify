package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

// BenchmarkDispatchToolsListRealRegistry measures the actual Clockify
// tools/list payload shape rather than synthetic no-schema descriptors. The
// custom metrics report the serialized JSON-RPC payload size, the contribution
// from outputSchema fields, and the visible tool count.
func BenchmarkDispatchToolsListRealRegistry(b *testing.B) {
	cases := []struct {
		name     string
		allTools bool
	}{
		{name: "Tier1"},
		{name: "AllTools", allTools: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			server := newToolsListBenchServer(b, tc.allTools)
			serverResponse := dispatchToolsListBenchRequest(b, server)
			payloadBytes, outputSchemaBytes, toolCount := measureToolsListPayload(b, serverResponse)

			msg := mustMarshalToolsListBenchRequest(b)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				out, err := server.DispatchMessage(ctx, msg)
				if err != nil {
					b.Fatalf("DispatchMessage: %v", err)
				}
				if len(out) == 0 {
					b.Fatal("empty tools/list response")
				}
			}
			b.ReportMetric(float64(payloadBytes), "payload_B")
			b.ReportMetric(float64(outputSchemaBytes), "output_schema_B")
			b.ReportMetric(float64(toolCount), "tools")
		})
	}
}

func newToolsListBenchServer(b *testing.B, allTools bool) *mcp.Server {
	b.Helper()
	svc := newActivationService(b)
	descriptors := append([]mcp.ToolDescriptor{}, svc.Registry()...)
	if allTools {
		for _, group := range tools.Tier2GroupNames() {
			tier2, ok := svc.Tier2Handlers(group)
			if !ok {
				b.Fatalf("Tier2Handlers(%q) missing", group)
			}
			descriptors = append(descriptors, tier2...)
		}
	}
	server := mcp.NewServer("bench", descriptors, nil, nil)
	initializeToolsListBenchServer(b, server)
	serverResponse := dispatchToolsListBenchRequest(b, server)
	if len(serverResponse) == 0 {
		b.Fatal("warm tools/list response was empty")
	}
	return server
}

func initializeToolsListBenchServer(b *testing.B, server *mcp.Server) {
	b.Helper()
	req, err := json.Marshal(mcp.Request{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params:  map[string]any{},
	})
	if err != nil {
		b.Fatalf("marshal initialize request: %v", err)
	}
	if _, err := server.DispatchMessage(context.Background(), req); err != nil {
		b.Fatalf("DispatchMessage initialize: %v", err)
	}
}

func dispatchToolsListBenchRequest(b *testing.B, server *mcp.Server) []byte {
	b.Helper()
	serverResponse, err := server.DispatchMessage(context.Background(), mustMarshalToolsListBenchRequest(b))
	if err != nil {
		b.Fatalf("DispatchMessage warmup: %v", err)
	}
	return serverResponse
}

func mustMarshalToolsListBenchRequest(b *testing.B) []byte {
	b.Helper()
	out, err := json.Marshal(mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if err != nil {
		b.Fatalf("marshal tools/list request: %v", err)
	}
	return out
}

func measureToolsListPayload(b *testing.B, raw []byte) (payloadBytes, outputSchemaBytes, toolCount int) {
	b.Helper()
	var decoded struct {
		Result struct {
			Tools []struct {
				OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		b.Fatalf("unmarshal tools/list response: %v", err)
	}
	for _, tool := range decoded.Result.Tools {
		if len(tool.OutputSchema) > 0 && string(tool.OutputSchema) != "null" {
			outputSchemaBytes += len(tool.OutputSchema)
		}
	}
	return len(raw), outputSchemaBytes, len(decoded.Result.Tools)
}
