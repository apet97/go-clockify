package mcp

import (
	"encoding/json"
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
			fast := toolCallParamsFromMap(tc.in)

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

// TestToolCallParamsFromMap_GracefullyHandlesWrongTypes documents the
// new contract explicitly: when a client sends the right key with the
// wrong type, the fast path leaves the struct field zero-valued rather
// than returning an error. The downstream callTool routing then
// surfaces the problem as "tool not found" or a per-handler validation
// failure. json.Unmarshal's default behaviour would return a type-error
// for the same input, but both routes produce a -32602-class failure
// to the client, so the observable contract is the same.
func TestToolCallParamsFromMap_GracefullyHandlesWrongTypes(t *testing.T) {
	// Malformed JSON shape: name is a number, arguments is a string.
	// The fast path must not panic and must yield a zero struct.
	in := map[string]any{
		"name":      float64(42),
		"arguments": "not-a-map",
		"_meta":     "not-a-map",
	}
	p := toolCallParamsFromMap(in)
	if p.Name != "" {
		t.Errorf("wrong-type name: got %q, want empty", p.Name)
	}
	if p.Arguments != nil {
		t.Errorf("wrong-type arguments: got %v, want nil", p.Arguments)
	}
	if p.Meta != nil {
		t.Errorf("wrong-type meta: got %v, want nil", p.Meta)
	}
	// Sanity: the decoded struct round-trips through encoding/json
	// without error, proving it is a valid zero value.
	if _, err := json.Marshal(p); err != nil {
		t.Errorf("zero struct should marshal cleanly: %v", err)
	}
}
