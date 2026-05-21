package tools

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/jsonschema"
)

// runtimeEnvelopeShapes returns the structured envelopes the MCP runtime can
// substitute for a tool's normal result: the size-cap envelope and the
// timeout / cancelled envelopes. Each must validate against every tool's
// advertised outputSchema, otherwise a real client would reject a legitimate
// runtime response.
func runtimeEnvelopeShapes(toolName string) map[string]map[string]any {
	return map[string]map[string]any{
		"result_too_large": {
			"ok":     false,
			"action": toolName,
			"error": map[string]any{
				"code":    "result_too_large",
				"message": toolName + " produced an oversized result.",
			},
			"recovery": map[string]any{
				"hint":      "Narrow the query (date range, page size) and retry.",
				"retryable": true,
			},
			"meta": map[string]any{
				"resultBytes":          float64(123456),
				"maxToolResultBytes":   float64(50000),
				"size_capped":          true,
				"truncationAttempted":  true,
				"truncationSuccessful": false,
			},
		},
		"timeout": {
			"ok":     false,
			"action": toolName,
			"error": map[string]any{
				"code":    "timeout",
				"message": "context deadline exceeded",
			},
			"recovery": map[string]any{
				"hint":      "The tool exceeded CLOCKIFY_TOOL_TIMEOUT. Retry.",
				"retryable": true,
			},
		},
		"cancelled": {
			"ok":     false,
			"action": toolName,
			"error": map[string]any{
				"code":    "cancelled",
				"message": "context canceled",
			},
			"recovery": map[string]any{
				"hint":      "The tool was cancelled before completion. Retry.",
				"retryable": true,
			},
		},
		"rate_limited": {
			"ok":     false,
			"action": toolName,
			"error": map[string]any{
				"code":    "rate_limited",
				"message": "tool invocation rate limit exceeded; retry shortly",
			},
			"recovery": map[string]any{
				"hint":      "The server is rate-limiting tool calls (CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE). Wait a moment and retry.",
				"retryable": true,
			},
		},
	}
}

func TestRuntimeEnvelopesConformToEveryToolOutputSchema(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	descriptors := svc.FullAccessRegistry()
	if len(descriptors) != 156 {
		t.Fatalf("FullAccessRegistry returned %d tools, want 156", len(descriptors))
	}
	for _, d := range descriptors {
		if d.Tool.OutputSchema == nil {
			t.Errorf("%s has no output schema", d.Tool.Name)
			continue
		}
		for shape, envelope := range runtimeEnvelopeShapes(d.Tool.Name) {
			if err := jsonschema.Validate(d.Tool.OutputSchema, envelope); err != nil {
				t.Errorf("%s: %s envelope fails its advertised output schema: %v", d.Tool.Name, shape, err)
			}
		}
	}
}
