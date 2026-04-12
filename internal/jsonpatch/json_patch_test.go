package jsonpatch

import (
	"encoding/json"
	"testing"
	"testing/quick"
)

func TestDiff_AddField(t *testing.T) {
	prev := `{"a":1}`
	curr := `{"a":1,"b":2}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_RemoveField(t *testing.T) {
	prev := `{"a":1,"b":2}`
	curr := `{"a":1}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_ReplaceField(t *testing.T) {
	prev := `{"a":1}`
	curr := `{"a":2}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	var ops []Operation
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Op != "replace" || ops[0].Path != "/a" {
		t.Fatalf("unexpected ops: %v", ops)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_NestedObject(t *testing.T) {
	prev := `{"a":{"b":1,"c":2}}`
	curr := `{"a":{"b":1,"c":3,"d":4}}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_Array(t *testing.T) {
	prev := `{"a":[1,2,3]}`
	curr := `{"a":[1,4,3]}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_ArrayGrow(t *testing.T) {
	prev := `[1,2]`
	curr := `[1,2,3]`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_ArrayShrink(t *testing.T) {
	prev := `[1,2,3]`
	curr := `[1]`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_EscapedPointer(t *testing.T) {
	prev := `{"a/b":1,"~c":2}`
	curr := `{"a/b":10,"~c":20}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	var ops []Operation
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.Path == "/a~1b" || op.Path == "/~0c" {
			continue
		}
		t.Fatalf("unexpected path: %q", op.Path)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_NoDifference(t *testing.T) {
	doc := `{"a":1,"b":[1,2]}`
	patch, err := Diff([]byte(doc), []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	var ops []Operation
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected empty patch, got %d ops", len(ops))
	}
}

func TestDiff_TopLevelReplace(t *testing.T) {
	prev := `"hello"`
	curr := `"world"`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestDiff_NullValues(t *testing.T) {
	prev := `{"a":1}`
	curr := `{"a":null}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, curr, string(result))
}

func TestApply_RFC6902_A1(t *testing.T) {
	doc := `{"foo":"bar"}`
	patch := `[{"op":"add","path":"/baz","value":"qux"}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"baz":"qux","foo":"bar"}`, string(result))
}

func TestApply_RFC6902_A2(t *testing.T) {
	doc := `{"foo":["bar","baz"]}`
	patch := `[{"op":"add","path":"/foo/1","value":"qux"}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"foo":["bar","qux","baz"]}`, string(result))
}

func TestApply_RFC6902_A3(t *testing.T) {
	doc := `{"baz":"qux","foo":"bar"}`
	patch := `[{"op":"remove","path":"/baz"}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"foo":"bar"}`, string(result))
}

func TestApply_RFC6902_A4(t *testing.T) {
	doc := `{"foo":["bar","qux","baz"]}`
	patch := `[{"op":"remove","path":"/foo/1"}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"foo":["bar","baz"]}`, string(result))
}

func TestApply_RFC6902_A5(t *testing.T) {
	doc := `{"baz":"qux","foo":"bar"}`
	patch := `[{"op":"replace","path":"/baz","value":"boo"}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"baz":"boo","foo":"bar"}`, string(result))
}

func TestApply_RFC6902_A10(t *testing.T) {
	doc := `{"foo":"bar"}`
	patch := `[{"op":"add","path":"/child","value":{"grandchild":{}}}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"foo":"bar","child":{"grandchild":{}}}`, string(result))
}

func TestApply_RFC6902_A14(t *testing.T) {
	doc := `{"foo":"bar"}`
	patch := `[{"op":"add","path":"/baz","value":null}]`
	result, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, `{"baz":null,"foo":"bar"}`, string(result))
}

func TestDiffApply_RoundTrip(t *testing.T) {
	f := func(seed uint8) bool {
		prev := randomJSON(int64(seed))
		curr := randomJSON(int64(seed) + 128)
		prevBytes, _ := json.Marshal(prev)
		currBytes, _ := json.Marshal(curr)
		patch, err := Diff(prevBytes, currBytes)
		if err != nil {
			return false
		}
		result, err := Apply(prevBytes, patch)
		if err != nil {
			return false
		}
		var resultVal any
		if err := json.Unmarshal(result, &resultVal); err != nil {
			return false
		}
		return jsonEqual(resultVal, curr)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

func randomJSON(seed int64) any {
	keys := []string{"a", "b", "c", "d", "e"}
	obj := map[string]any{}
	for i, k := range keys {
		v := (seed + int64(i)*37) % 7
		switch v {
		case 0:
			obj[k] = float64(seed+int64(i)) / 3.0
		case 1:
			obj[k] = k + "_val"
		case 2:
			obj[k] = seed%2 == 0
		case 3:
			obj[k] = nil
		case 4:
			obj[k] = []any{float64(i), k}
		case 5:
			obj[k] = map[string]any{"nested": float64(i)}
		default:
			// omit key
		}
	}
	return obj
}

func assertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()
	var e, a any
	if err := json.Unmarshal([]byte(expected), &e); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}
	if err := json.Unmarshal([]byte(actual), &a); err != nil {
		t.Fatalf("unmarshal actual: %v; raw=%s", err, actual)
	}
	if !jsonEqual(e, a) {
		t.Fatalf("JSON mismatch:\nexpected: %s\n  actual: %s", expected, actual)
	}
}
