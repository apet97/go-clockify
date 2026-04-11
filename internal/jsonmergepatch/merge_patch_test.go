package jsonmergepatch

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestRFC7396Vectors exercises the 14 test vectors listed in RFC 7396
// §3. Each row is target/patch/expected — Apply(target, patch) should
// equal expected. Doubling as unit tests for the Apply implementation.
func TestRFC7396Vectors(t *testing.T) {
	vectors := []struct {
		name   string
		target string
		patch  string
		want   string
	}{
		{`add-key`, `{"a":"b"}`, `{"a":"c"}`, `{"a":"c"}`},
		{`add-new-key`, `{"a":"b"}`, `{"b":"c"}`, `{"a":"b","b":"c"}`},
		{`delete-key`, `{"a":"b"}`, `{"a":null}`, `{}`},
		{`delete-only-listed`, `{"a":"b","b":"c"}`, `{"a":null}`, `{"b":"c"}`},
		{`replace-value-with-array`, `{"a":["b"]}`, `{"a":"c"}`, `{"a":"c"}`},
		{`replace-value-with-array-2`, `{"a":"c"}`, `{"a":["b"]}`, `{"a":["b"]}`},
		{`replace-nested-object`, `{"a":{"b":"c"}}`, `{"a":{"b":"d","c":null}}`, `{"a":{"b":"d"}}`},
		{`replace-scalar-with-array`, `{"a":[{"b":"c"}]}`, `{"a":[1]}`, `{"a":[1]}`},
		{`replace-with-scalar`, `["a","b"]`, `["c","d"]`, `["c","d"]`},
		{`replace-obj-with-obj`, `{"a":"b"}`, `{"a":"c","b":null}`, `{"a":"c"}`},
		{`add-to-empty`, `{}`, `{"a":{"bb":{"ccc":null}}}`, `{"a":{"bb":{}}}`},
		{`null-in-nested-delete`, `{"a":"b"}`, `null`, `null`},
		// RFC 7396 §3 row: '{"a":"foo"}' + '"bar"' → '"bar"'
		{`scalar-replaces-object`, `{"a":"foo"}`, `"bar"`, `"bar"`},
		// '{"e":null}' + '{"a":1}' → '{"e":null,"a":1}' — merge into
		// existing object. Note: null in target is preserved (not
		// deleted) because delete only triggers when null appears in
		// the patch, not the target.
		{`null-in-target-preserved`, `{"e":null}`, `{"a":1}`, `{"e":null,"a":1}`},
	}
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			got, err := Apply([]byte(v.target), []byte(v.patch))
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if !jsonEqual(t, got, []byte(v.want)) {
				t.Fatalf("Apply(%s, %s) = %s, want %s", v.target, v.patch, got, v.want)
			}
		})
	}
}

// TestDiffApplyRoundTrip covers the property Apply(prev, Diff(prev,
// curr)) == curr for hand-picked object pairs that exercise additions,
// deletions, nested objects, scalar-to-array transitions, and
// array-element changes.
func TestDiffApplyRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		prev string
		curr string
	}{
		{`empty-to-empty`, `{}`, `{}`},
		{`add-single-key`, `{}`, `{"a":1}`},
		{`remove-single-key`, `{"a":1}`, `{}`},
		{`modify-single-key`, `{"a":1}`, `{"a":2}`},
		{`swap-type`, `{"a":1}`, `{"a":"one"}`},
		{`nested-add`, `{"a":{"b":1}}`, `{"a":{"b":1,"c":2}}`},
		{`nested-remove`, `{"a":{"b":1,"c":2}}`, `{"a":{"b":1}}`},
		{`deep-modify`, `{"a":{"b":{"c":1}}}`, `{"a":{"b":{"c":2}}}`},
		{`array-replace`, `{"arr":[1,2,3]}`, `{"arr":[1,2,3,4]}`},
		{`array-to-scalar`, `{"x":[1]}`, `{"x":"one"}`},
		{`mixed-changes`, `{"a":1,"b":2,"c":3}`, `{"a":10,"c":3,"d":4}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patch, err := Diff([]byte(c.prev), []byte(c.curr))
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			out, err := Apply([]byte(c.prev), patch)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if !jsonEqual(t, out, []byte(c.curr)) {
				t.Fatalf("round-trip failed: Apply(%s, %s) = %s, want %s",
					c.prev, patch, out, c.curr)
			}
		})
	}
}

// TestDiffMinimal verifies the generated patch does not include members
// that are identical in prev and curr. A minimal patch is both a
// correctness property (RFC 7396 says members unchanged in the target
// should not appear) and a bandwidth optimisation for the delta-sync
// wire.
func TestDiffMinimal(t *testing.T) {
	prev := `{"id":"abc","description":"old","billable":true,"projectId":"p1"}`
	curr := `{"id":"abc","description":"new","billable":true,"projectId":"p1"}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(patch, &decoded); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 key in minimal patch, got %d: %s", len(decoded), patch)
	}
	if decoded["description"] != "new" {
		t.Fatalf("expected description=new, got %v", decoded["description"])
	}
}

// TestDiffRemovalEncodedAsNull checks that a member present in prev and
// missing from curr is encoded as an explicit null in the patch (RFC
// 7396's delete signalling).
func TestDiffRemovalEncodedAsNull(t *testing.T) {
	prev := `{"id":"abc","tagIds":["t1","t2"]}`
	curr := `{"id":"abc"}`
	patch, err := Diff([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(patch, &decoded); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	v, ok := decoded["tagIds"]
	if !ok {
		t.Fatalf("patch missing tagIds entry: %s", patch)
	}
	if v != nil {
		t.Fatalf("expected tagIds=null, got %v", v)
	}
}

// TestDiffEmptyForIdentical returns an empty object patch when there
// is nothing to change. Apply(prev, emptyPatch) == prev.
func TestDiffEmptyForIdentical(t *testing.T) {
	prev := `{"a":1,"b":2}`
	patch, err := Diff([]byte(prev), []byte(prev))
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if string(patch) != "{}" {
		t.Fatalf("expected empty-object patch, got %s", patch)
	}
	applied, err := Apply([]byte(prev), patch)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !jsonEqual(t, applied, []byte(prev)) {
		t.Fatalf("Apply empty patch changed prev: %s → %s", prev, applied)
	}
}

// TestDiffOrFullNullDetection confirms the DiffOrFull fallback triggers
// when the target document carries an explicit null value anywhere in
// its tree. Without the fallback, the RFC-compliant diff would lose
// the null entry because the merge-patch apply algorithm treats null
// members as deletions.
func TestDiffOrFullNullDetection(t *testing.T) {
	prev := `{"id":"abc","description":"old"}`
	curr := `{"id":"abc","description":null,"projectId":"p1"}`
	patch, format, err := DiffOrFull([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatalf("DiffOrFull: %v", err)
	}
	if format != FormatFull {
		t.Fatalf("expected format=%s, got %s", FormatFull, format)
	}
	if !jsonEqual(t, patch, []byte(curr)) {
		t.Fatalf("expected full replacement, got %s", patch)
	}
}

// TestDiffOrFullMerge confirms the happy path: no nulls in curr means
// format=merge and a minimal patch.
func TestDiffOrFullMerge(t *testing.T) {
	prev := `{"id":"abc","description":"old"}`
	curr := `{"id":"abc","description":"new"}`
	patch, format, err := DiffOrFull([]byte(prev), []byte(curr))
	if err != nil {
		t.Fatalf("DiffOrFull: %v", err)
	}
	if format != FormatMerge {
		t.Fatalf("expected format=%s, got %s", FormatMerge, format)
	}
	var decoded map[string]any
	if err := json.Unmarshal(patch, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["description"] != "new" || len(decoded) != 1 {
		t.Fatalf("unexpected minimal patch: %s", patch)
	}
}

// TestDiffNonObjectInputs covers the RFC 7396 §2 special case: when
// either input is not a JSON object, the patch is just curr.
func TestDiffNonObjectInputs(t *testing.T) {
	for _, pair := range [][2]string{
		{`[1,2,3]`, `[4,5,6]`},
		{`{"a":1}`, `"scalar"`},
		{`42`, `{"a":1}`},
	} {
		patch, err := Diff([]byte(pair[0]), []byte(pair[1]))
		if err != nil {
			t.Fatalf("Diff(%s,%s): %v", pair[0], pair[1], err)
		}
		if !jsonEqual(t, patch, []byte(pair[1])) {
			t.Fatalf("expected %s, got %s", pair[1], patch)
		}
	}
}

// jsonEqual compares two JSON documents for structural equality by
// round-tripping both through the decoder.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var ax, bx any
	if err := json.Unmarshal(a, &ax); err != nil {
		t.Fatalf("unmarshal a=%s: %v", a, err)
	}
	if err := json.Unmarshal(b, &bx); err != nil {
		t.Fatalf("unmarshal b=%s: %v", b, err)
	}
	ac, _ := json.Marshal(ax)
	bc, _ := json.Marshal(bx)
	return bytes.Equal(ac, bc)
}
