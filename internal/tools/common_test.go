package tools

import (
	"math"
	"testing"
)

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
