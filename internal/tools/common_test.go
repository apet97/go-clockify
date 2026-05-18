package tools

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

// wireArgs round-trips an args map through JSON exactly as the stdio MCP
// transport does: json.Decoder.UseNumber() turns every numeric literal into a
// json.Number. Tool tests that build map[string]any directly never exercise
// this, which is how the json.Number extraction bug shipped unnoticed.
func wireArgs(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode args: %v", err)
	}
	return out
}

// TestNumericArgsHandleWireJSONNumber gates the whole numeric-argument class:
// every extractor must read a json.Number, the type a number actually has
// once it crosses the stdio transport.
func TestNumericArgsHandleWireJSONNumber(t *testing.T) {
	args := wireArgs(t, map[string]any{
		"amount":    25.5,
		"page_size": 200,
		"value":     1500,
		"nested":    map[string]any{"estimate": 500000},
	})
	if _, isNum := args["amount"].(json.Number); !isNum {
		t.Fatalf("setup: amount should decode as json.Number, got %T", args["amount"])
	}
	if got, ok := numberArg(args, "amount"); !ok || got != 25.5 {
		t.Errorf("numberArg(amount) = %v, %v; want 25.5, true", got, ok)
	}
	if got := intArg(args, "page_size", 50); got != 200 {
		t.Errorf("intArg(page_size) = %d; want 200", got)
	}
	if got, ok := int64Arg(args, "value"); !ok || got != 1500 {
		t.Errorf("int64Arg(value) = %d, %v; want 1500, true", got, ok)
	}
	nested, _ := args["nested"].(map[string]any)
	if got, ok := numberFromMap(nested, "estimate"); !ok || got != 500000 {
		t.Errorf("numberFromMap(estimate) = %v, %v; want 500000, true", got, ok)
	}
}

func TestNumberFromAny(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{"json.Number int", json.Number("42"), 42, true},
		{"json.Number float", json.Number("3.5"), 3.5, true},
		{"json.Number malformed", json.Number("12x"), 0, false},
		{"float64", float64(7), 7, true},
		{"int", 9, 9, true},
		{"int64", int64(11), 11, true},
		{"string", "nope", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	} {
		if got, ok := numberFromAny(tc.in); got != tc.want || ok != tc.ok {
			t.Errorf("numberFromAny(%s) = %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestJSONTypeName(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want string
	}{
		{json.Number("1"), "number"},
		{float64(1), "number"},
		{"x", "string"},
		{true, "boolean"},
		{nil, "null"},
		{map[string]any{}, "object"},
		{[]any{}, "array"},
	} {
		if got := jsonTypeName(tc.in); got != tc.want {
			t.Errorf("jsonTypeName(%T) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestIntArgFloat64Normal(t *testing.T) {
	got := intArg(map[string]any{"page": 5.0}, "page", 1)
	if got != 5 {
		t.Errorf("intArg(5.0) = %d; want 5", got)
	}
}

func TestIntArgFloat64NaN(t *testing.T) {
	got := intArg(map[string]any{"page": math.NaN()}, "page", 1)
	if got != 1 {
		t.Errorf("intArg(NaN) = %d; want fallback 1", got)
	}
}

func TestIntArgFloat64PosInf(t *testing.T) {
	got := intArg(map[string]any{"page": math.Inf(1)}, "page", 1)
	if got != 1 {
		t.Errorf("intArg(+Inf) = %d; want fallback 1", got)
	}
}

func TestIntArgFloat64NegInf(t *testing.T) {
	got := intArg(map[string]any{"page": math.Inf(-1)}, "page", 1)
	if got != 1 {
		t.Errorf("intArg(-Inf) = %d; want fallback 1", got)
	}
}

func TestIntArgFloat64Overflow(t *testing.T) {
	got := intArg(map[string]any{"page": 1e19}, "page", 1)
	if got != 1 {
		t.Errorf("intArg(1e19) = %d; want fallback 1", got)
	}
}

func TestInt64ArgRejectsNonFiniteValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"overflow", 9.99e30},
	} {
		if got, ok := int64Arg(map[string]any{"x": tc.in}, "x"); ok {
			t.Fatalf("%s: int64Arg = %d, true; want 0, false", tc.name, got)
		}
	}
	if got, ok := int64Arg(map[string]any{"x": int64(42)}, "x"); !ok || got != 42 {
		t.Fatalf("int64Arg(int64(42)) = %d, %v; want 42, true", got, ok)
	}
}

func TestIntArgMissing(t *testing.T) {
	got := intArg(map[string]any{}, "page", 42)
	if got != 42 {
		t.Errorf("intArg(missing) = %d; want fallback 42", got)
	}
}

func TestIntArgInt(t *testing.T) {
	got := intArg(map[string]any{"x": 7}, "x", 0)
	if got != 7 {
		t.Errorf("intArg(int 7) = %d; want 7", got)
	}
}

func TestStringArg(t *testing.T) {
	got := stringArg(map[string]any{"name": "test"}, "name")
	if got != "test" {
		t.Errorf("stringArg = %q; want test", got)
	}
}

func TestStringArgMissing(t *testing.T) {
	got := stringArg(map[string]any{}, "name")
	if got != "" {
		t.Errorf("stringArg(missing) = %q; want empty", got)
	}
}

func TestBoolArg(t *testing.T) {
	got := boolArg(map[string]any{"flag": true}, "flag")
	if !got {
		t.Error("boolArg(true) = false; want true")
	}
}

func TestBoolArgMissing(t *testing.T) {
	got := boolArg(map[string]any{}, "flag")
	if got {
		t.Error("boolArg(missing) = true; want false")
	}
}

func TestOKLeavesNilMetaOmitted(t *testing.T) {
	result := ok("clockify_probe", map[string]any{"ok": true}, nil)
	if result.Meta != nil {
		t.Fatalf("expected nil meta, got %+v", result.Meta)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := decoded["meta"]; ok {
		t.Fatalf("meta should be omitted for nil meta, got JSON %s", raw)
	}
}
