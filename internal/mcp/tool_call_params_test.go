package mcp

import (
	"reflect"
	"testing"
)

// TestToolCallParamsFromMap_MatchesDecodeParams asserts the G3a fast
// path (toolCallParamsFromMap) produces the same ToolCallParams struct
// as the historical json.Marshal → json.Unmarshal roundtrip that
// decodeParams still implements. Covers the matrix of shapes a client
// can realistically send: bare name, name + args, name + _meta with
// string token, name + _meta with numeric token, empty _meta, and
// extra ignorable keys.
//
// This is the correctness guard for the G3a optimization — if a shape
// surfaces a difference, the fast path is wrong and must be fixed.
func TestToolCallParamsFromMap_MatchesDecodeParams(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
	}{
		{
			name: "bare_name",
			in: map[string]any{
				"name": "clockify_log_time",
			},
		},
		{
			name: "name_and_args",
			in: map[string]any{
				"name": "clockify_log_time",
				"arguments": map[string]any{
					"start":       "2026-01-01T09:00:00Z",
					"end":         "2026-01-01T10:00:00Z",
					"description": "example",
				},
			},
		},
		{
			name: "name_with_meta_string_token",
			in: map[string]any{
				"name":      "clockify_list_entries",
				"arguments": map[string]any{"page": float64(1)},
				"_meta":     map[string]any{"progressToken": "prog-abc"},
			},
		},
		{
			name: "name_with_meta_numeric_token",
			in: map[string]any{
				"name":  "clockify_list_entries",
				"_meta": map[string]any{"progressToken": float64(42)},
			},
		},
		{
			name: "empty_meta_object",
			in: map[string]any{
				"name":  "clockify_list_entries",
				"_meta": map[string]any{},
			},
		},
		{
			name: "extra_unknown_key",
			in: map[string]any{
				"name":        "clockify_list_entries",
				"_additional": "ignored",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fast, err := toolCallParamsFromMap(tc.in)
			if err != nil {
				t.Fatalf("toolCallParamsFromMap: %v", err)
			}

			var slow ToolCallParams
			if err := decodeParams(tc.in, &slow); err != nil {
				t.Fatalf("decodeParams: %v", err)
			}

			if fast.Name != slow.Name {
				t.Errorf("Name: fast=%q slow=%q", fast.Name, slow.Name)
			}
			if !reflect.DeepEqual(fast.Arguments, slow.Arguments) {
				t.Errorf("Arguments diverge:\n  fast=%+v\n  slow=%+v", fast.Arguments, slow.Arguments)
			}
			// Both paths should either return nil meta or a struct with
			// the same ProgressToken.
			switch {
			case fast.Meta == nil && slow.Meta == nil:
				// match
			case fast.Meta == nil || slow.Meta == nil:
				t.Errorf("Meta nilness diverges: fast=%v slow=%v", fast.Meta, slow.Meta)
			default:
				if !reflect.DeepEqual(fast.Meta.ProgressToken, slow.Meta.ProgressToken) {
					t.Errorf("ProgressToken diverges: fast=%v slow=%v",
						fast.Meta.ProgressToken, slow.Meta.ProgressToken)
				}
			}
		})
	}
}

// TestToolCallParamsFromMap_RejectsWrongTypes documents the protocol
// contract: malformed tools/call params are JSON-RPC invalid params, not
// silently-zeroed tool execution requests.
func TestToolCallParamsFromMap_RejectsWrongTypes(t *testing.T) {
	// Malformed JSON shape: name is a number, arguments is a string.
	in := map[string]any{
		"name":      float64(42),
		"arguments": "not-a-map",
		"_meta":     "not-a-map",
	}
	if _, err := toolCallParamsFromMap(in); err == nil {
		t.Fatal("expected wrong-type fields to be rejected")
	}
}
