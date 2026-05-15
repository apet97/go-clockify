package tools

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestOneUserWriteResultsIncludeIDsAndChanged(t *testing.T) {
	upstream := newOneUserCoverageUpstream()
	defer upstream.Close()

	svc := New(clockify.NewClient("test-key", upstream.URL, time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.EnableRawWrites = true
	svc.DefaultTimezone = time.UTC
	server := mcp.NewServer("test", svc.FullAccessRegistry(), nil, nil)
	initializeServer(t, server)

	exceptions := map[string]string{
		"clockify_entries_timer_stop": "idempotent no-op when the fake server reports no running timer",
		"clockify_demo_cleanup":       "idempotent cleanup has no matching fake demo resources to delete",
	}

	for _, descriptor := range svc.FullAccessRegistry() {
		if descriptor.ReadOnlyHint {
			continue
		}
		t.Run(descriptor.Tool.Name, func(t *testing.T) {
			result := callToolOK(t, server, descriptor.Tool.Name, writeContractArgs(descriptor.Tool))
			if len(result.IDs) == 0 {
				t.Fatalf("write result returned no ids: %+v", result)
			}
			if hasAnyChange(result.Changed) {
				return
			}
			if reason, ok := exceptions[descriptor.Tool.Name]; ok {
				t.Logf("changed exception: %s", reason)
				return
			}
			t.Fatalf("write result returned no changed.created/updated/deleted/reused: %+v", result.Changed)
		})
	}
}

func writeContractArgs(tool mcp.Tool) map[string]any {
	args := oneUserCoverageArgs(tool)
	switch tool.Name {
	case "clockify_log_work":
		// clockify_log_work guards against overlapping entries; this
		// contract test only asserts the write-envelope shape, so bypass
		// the guard rather than depend on the fixture's seeded entries.
		args["allow_overlap"] = true
	case "clockify_fix_entry":
		args = map[string]any{
			"entry_id":    oneUserCoverageID("entry_id"),
			"description": "Coverage fixed entry",
		}
	case "clockify_api_request":
		args = map[string]any{
			"method": "POST",
			"path":   "/workspaces/{workspaceId}/clients",
			"body":   map[string]any{"name": "Raw Client"},
		}
	}
	return args
}
