//go:build legacy_platform

package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

func TestAgentToolChoiceAnnotations(t *testing.T) {
	svc := New(nil, "ws1")
	descriptors := map[string]bool{}
	check := func(name string, want map[string]any) {
		ds, ok := descriptorByName(svc, name)
		if !ok {
			t.Fatalf("missing descriptor %s", name)
		}
		descriptors[name] = true
		for key, wantValue := range want {
			got := ds.Tool.Annotations[key]
			if got != wantValue {
				t.Fatalf("%s annotation %s = %#v, want %#v", name, key, got, wantValue)
			}
		}
	}

	check("clockify_log_time", map[string]any{})
	if _, ok := descriptorByName(svc, "clockify_log_time"); ok {
		ds, _ := descriptorByName(svc, "clockify_log_time")
		if got, ok := ds.Tool.Annotations["preferOver"].([]string); !ok || len(got) != 1 || got[0] != "clockify_add_entry" {
			t.Fatalf("clockify_log_time preferOver = %#v", ds.Tool.Annotations["preferOver"])
		}
		if got, ok := ds.Tool.Annotations["bestToolFor"].([]string); !ok || len(got) == 0 {
			t.Fatalf("clockify_log_time bestToolFor = %#v", ds.Tool.Annotations["bestToolFor"])
		}
	}
	check("clockify_search_tools", map[string]any{"compatibilityShim": true, "primaryTool": "clockify_list_tools"})
	check("clockify_resolve_debug", map[string]any{"compatibilityShim": true, "primaryTool": "clockify_resolve_name"})

	if len(descriptors) != 3 {
		t.Fatalf("unexpected descriptor checks: %v", descriptors)
	}
}

func descriptorByName(svc *Service, name string) (mcp.ToolDescriptor, bool) {
	for _, descriptor := range svc.Registry() {
		if descriptor.Tool.Name == name {
			return descriptor, true
		}
	}
	for _, group := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(group.Name)
		if !ok {
			continue
		}
		for _, descriptor := range descriptors {
			if descriptor.Tool.Name == name {
				return descriptor, true
			}
		}
	}
	return mcp.ToolDescriptor{}, false
}
