package tools

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// TestOkHelperReturnsToolResult guards the T2.1 convergence: ok() now
// returns ToolResult (the legacy ResultEnvelope name was retired). The
// fields the helper does not populate (Entity, IDs, Changed, Warnings,
// Next) must stay zero-valued so the wire shape matches the historical
// four-key narrow envelope.
func TestOkHelperReturnsToolResult(t *testing.T) {
	env := ok("clockify_status", nil, nil)
	if reflect.TypeFor[ToolResult]().Name() != "ToolResult" {
		t.Fatalf("ok() returned %T, want ToolResult", env)
	}
	if env.Entity != "" || env.IDs != nil || env.Warnings != nil || env.Next != nil {
		t.Fatalf("narrow envelope unexpectedly populated rich fields: %+v", env)
	}
	if hasAnyChange(env.Changed) {
		t.Fatalf("narrow envelope unexpectedly populated Changed: %+v", env.Changed)
	}
}

// TestOkHelperEmitsNarrowWireShape guards the ~824 narrow return sites:
// ok(action, data, meta) must serialize to exactly the four keys
// {ok, action, data, meta}. Adding fields here is a public wire change
// and must be done with intent, not by accident.
func TestOkHelperEmitsNarrowWireShape(t *testing.T) {
	env := ok("clockify_status", map[string]any{"hello": "world"}, map[string]any{"workspaceId": "ws1"})
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"action", "data", "meta", "ok"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("narrow envelope keys = %v, want %v (raw: %s)", keys, want, raw)
	}
}

// TestRichEnvelopeIncludesChangedWhenSet guards the existing ToolResult
// sites: a result with a non-empty ChangeSet must serialize the
// "changed" key. Read-only handlers that pass ChangeSet{} get the
// field omitted (no meaningless "changed":{} noise).
func TestRichEnvelopeIncludesChangedWhenSet(t *testing.T) {
	withChanges := result(
		"clockify_clients_create",
		"client",
		map[string]string{"clientId": "c1"},
		map[string]any{"id": "c1", "name": "Acme"},
		ChangeSet{Created: []EntityRef{{Type: "client", ID: "c1", Name: "Acme"}}},
		nil,
		nil,
	)
	raw, err := json.Marshal(withChanges)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	changed, ok := got["changed"].(map[string]any)
	if !ok {
		t.Fatalf("changed key missing or wrong shape, raw: %s", raw)
	}
	created, _ := changed["created"].([]any)
	if len(created) != 1 {
		t.Fatalf("changed.created = %v, want 1 entry; raw: %s", created, raw)
	}

	noChanges := result(
		"clockify_clients_list",
		"client",
		nil,
		[]map[string]any{{"id": "c1"}},
		ChangeSet{},
		nil,
		nil,
	)
	raw2, err := json.Marshal(noChanges)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got2 := map[string]any{}
	if err := json.Unmarshal(raw2, &got2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got2["changed"]; present {
		t.Fatalf("changed should be omitted for zero ChangeSet, raw: %s", raw2)
	}
}

// TestNarrowEnvelopeOmitsZeroEntityAndIDs guards that the T2.1
// migration did not start emitting empty entity/ids/warnings/next for
// the ~824 narrow return sites — only fields actively set by the
// handler should reach the wire.
func TestNarrowEnvelopeOmitsZeroEntityAndIDs(t *testing.T) {
	env := ok("clockify_anything", nil, nil)
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, forbidden := range []string{"entity", "ids", "changed", "warnings", "next"} {
		if _, present := got[forbidden]; present {
			t.Fatalf("zero envelope leaked %q: %s", forbidden, raw)
		}
	}
}
