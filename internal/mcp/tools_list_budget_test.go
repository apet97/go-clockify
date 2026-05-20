package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

func TestToolsListWithinMessageBudget(t *testing.T) {
	raw := dispatchToolsList(t, "all")
	if len(raw) >= 1<<20 {
		t.Fatalf("tools/list response size=%d, want < 1 MiB", len(raw))
	}
}

func TestToolsListBudgetWire(t *testing.T) {
	tests := map[string]int{
		"default": 80 * 1024,
		"all":     280 * 1024,
	}
	for toolset, wireBudgetBytes := range tests {
		t.Run(toolset, func(t *testing.T) {
			raw := dispatchToolsList(t, toolset)
			if len(raw) > wireBudgetBytes {
				t.Fatalf("tools/list wire size %d exceeds budget %d (~%d KB over)",
					len(raw), wireBudgetBytes, (len(raw)-wireBudgetBytes)/1024)
			}
			t.Logf("tools/list wire size (%s): %d bytes (%.1f KB)", toolset, len(raw), float64(len(raw))/1024.0)
		})
	}
}

func dispatchToolsList(t *testing.T, toolset string) []byte {
	t.Helper()
	svc := &tools.Service{}
	server := mcp.NewServer("test", svc.FullAccessRegistry())
	server.SetAdvertisedTools(svc.RegistryForToolset(toolset))
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "test", "0")

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("tools/list response is not JSON: %s", raw)
	}
	return raw
}
