package e2e_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/tests/harness"
)

func TestParity_ToolsCallSchemaValidationErrorCarriesPointer(t *testing.T) {
	for name, factory := range allFactories() {
		factory := factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, schemaValidationParityOptions())
			if err != nil {
				if errors.Is(err, harness.ErrGRPCUnavailable) {
					t.Skip("gRPC harness unavailable (requires -tags=grpc)")
				}
				t.Fatalf("factory: %v", err)
			}
			defer func() { _ = h.Close() }()

			if _, err := h.Initialize(ctx); err != nil {
				t.Fatalf("initialize: %v", err)
			}
			resp, err := h.CallTool(ctx, "schema_probe", map[string]any{
				"start":    "2026-04-11T09:00:00Z",
				"billable": "yes",
			})
			if err != nil {
				t.Fatalf("tools/call: %v", err)
			}
			if resp.Error == nil {
				t.Fatalf("expected JSON-RPC error, got result %s", string(resp.Result))
			}
			if resp.Error.Code != -32602 {
				t.Fatalf("error.code = %d, want -32602", resp.Error.Code)
			}
			if pointer, ok := errorDataPointer(resp.Error); !ok || pointer != "/billable" {
				t.Fatalf("error.data.pointer = %q (present=%v), want /billable; data=%#v",
					pointer, ok, resp.Error.Data)
			}
		})
	}
}

func schemaValidationParityOptions() harness.Options {
	return harness.Options{
		BearerToken: "schema-validation-token",
		Enforcement: &enforcement.Pipeline{},
		Tools: []mcp.ToolDescriptor{{
			Tool: mcp.Tool{
				Name:        "schema_probe",
				Description: "Schema validation parity probe",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"start"},
					"properties": map[string]any{
						"start":    map[string]any{"type": "string"},
						"billable": map[string]any{"type": "boolean"},
					},
				},
			},
			Handler: func(context.Context, map[string]any) (any, error) {
				return map[string]string{"status": "unexpected"}, nil
			},
			ReadOnlyHint: true,
		}},
	}
}

func errorDataPointer(err *harness.RPCError) (string, bool) {
	data, ok := err.Data.(map[string]any)
	if !ok {
		return "", false
	}
	pointer, ok := data["pointer"].(string)
	return pointer, ok
}
