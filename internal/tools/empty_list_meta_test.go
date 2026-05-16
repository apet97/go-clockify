package tools

import (
	"strings"
	"testing"
)

// TestEmptyListMeta locks P2-11: a list result with count==0 gains a
// nextAction hint (naming a create tool when one exists); a non-empty result
// and a meta without a count are both left untouched.
func TestEmptyListMeta(t *testing.T) {
	withCreate := emptyListMeta(map[string]any{"count": 0}, "clockify_tags_create")
	hint, _ := withCreate["nextAction"].(string)
	if hint == "" {
		t.Fatal("count==0 should add a nextAction hint")
	}
	if !strings.Contains(hint, "clockify_tags_create") {
		t.Errorf("hint should name the create tool: %q", hint)
	}

	noCreate := emptyListMeta(map[string]any{"count": 0}, "")
	if h, _ := noCreate["nextAction"].(string); h == "" {
		t.Error("count==0 should add a nextAction hint even without a create tool")
	}

	nonEmpty := emptyListMeta(map[string]any{"count": 3}, "clockify_tags_create")
	if _, ok := nonEmpty["nextAction"]; ok {
		t.Error("a non-empty result must not get a nextAction hint")
	}

	absent := emptyListMeta(map[string]any{"workspaceId": "ws"}, "clockify_tags_create")
	if _, ok := absent["nextAction"]; ok {
		t.Error("a meta without a count must be left untouched")
	}
}
