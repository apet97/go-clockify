package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/tests/harness"
)

type parityUpstreamError struct {
	verbose   string
	sanitized string
}

func (e *parityUpstreamError) Error() string     { return e.verbose }
func (e *parityUpstreamError) Sanitized() string { return e.sanitized }

func TestParity_SanitizeUpstreamErrorsAcrossTransports(t *testing.T) {
	const leakedBody = "tenant=acme-internal user_email=private@example.com"
	const sanitized = "clockify GET /workspaces/ws1/users failed: 403 Forbidden"
	tool := mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        "upstream_error_probe",
			Description: "Returns a sanitizable upstream error",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(context.Context, map[string]any) (any, error) {
			return nil, &parityUpstreamError{
				verbose:   sanitized + ": " + leakedBody,
				sanitized: sanitized,
			}
		},
	}

	for name, factory := range allFactories() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h, err := factory(ctx, harness.Options{
				BearerToken:            strings.Repeat("s", 16),
				Tools:                  []mcp.ToolDescriptor{tool},
				SanitizeUpstreamErrors: true,
			})
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
			resp, err := h.CallTool(ctx, "upstream_error_probe", nil)
			if err != nil {
				t.Fatalf("tools/call: %v", err)
			}
			if resp.Error != nil {
				t.Fatalf("expected tool-level error result, got RPC error: %+v", resp.Error)
			}
			payload := string(resp.Result)
			if strings.Contains(payload, leakedBody) {
				t.Fatalf("sanitized transport leaked upstream body: %s", payload)
			}
			if !strings.Contains(payload, sanitized) {
				t.Fatalf("sanitized transport dropped method/path/status context: %s", payload)
			}
		})
	}
}
