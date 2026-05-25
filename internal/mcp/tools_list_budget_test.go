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

func TestDefaultToolsListAdvertisesOutputSchemas(t *testing.T) {
	raw := dispatchToolsList(t, "default")
	var resp struct {
		Result struct {
			Tools []struct {
				Name         string         `json:"name"`
				OutputSchema map[string]any `json:"outputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list: %v\n%s", err, raw)
	}
	if len(resp.Result.Tools) != 16 {
		t.Fatalf("default tools/list returned %d tools, want 16", len(resp.Result.Tools))
	}
	for _, tool := range resp.Result.Tools {
		if len(tool.OutputSchema) == 0 {
			t.Fatalf("%s missing advertised outputSchema in default tools/list", tool.Name)
		}
		required, _ := tool.OutputSchema["required"].([]any)
		if len(required) == 0 {
			t.Fatalf("%s outputSchema missing required envelope keys: %+v", tool.Name, tool.OutputSchema)
		}
	}
}

func TestEveryAdvertisedToolHasOutputSchema(t *testing.T) {
	for _, toolset := range []string{"default", "core", "business", "admin", "all"} {
		t.Run(toolset, func(t *testing.T) {
			for _, tool := range collectToolsListPages(t, toolset) {
				if len(tool.OutputSchema) == 0 {
					t.Fatalf("%s tool %s missing advertised outputSchema", toolset, tool.Name)
				}
			}
		})
	}
}

type listedToolForBudget struct {
	Name         string         `json:"name"`
	OutputSchema map[string]any `json:"outputSchema"`
}

func collectToolsListPages(t *testing.T, toolset string) []listedToolForBudget {
	t.Helper()
	var out []listedToolForBudget
	cursor := ""
	for {
		raw := dispatchToolsListWithCursor(t, toolset, cursor)
		var resp struct {
			Result struct {
				Tools      []listedToolForBudget `json:"tools"`
				NextCursor string                `json:"nextCursor"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("unmarshal tools/list page: %v\n%s", err, raw)
		}
		out = append(out, resp.Result.Tools...)
		if resp.Result.NextCursor == "" {
			return out
		}
		cursor = resp.Result.NextCursor
	}
}

func dispatchToolsListWithCursor(t *testing.T, toolset, cursor string) []byte {
	t.Helper()
	svc := &tools.Service{}
	reg, err := svc.FullAccessRegistryChecked()
	if err != nil {
		t.Fatalf("FullAccessRegistryChecked: %v", err)
	}
	server := mcp.NewServer("test", reg)
	server.SetAdvertisedTools(svc.RegistryForToolset(toolset))
	server.MarkInitialized(mcp.SupportedProtocolVersions[0], "test", "0")

	params := ""
	if cursor != "" {
		params = `,"params":{"cursor":"` + cursor + `"}`
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"`+params+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("tools/list response is not JSON: %s", raw)
	}
	return raw
}

func dispatchToolsList(t *testing.T, toolset string) []byte {
	t.Helper()
	svc := &tools.Service{}
	reg, err := svc.FullAccessRegistryChecked()
	if err != nil {
		t.Fatalf("FullAccessRegistryChecked: %v", err)
	}
	server := mcp.NewServer("test", reg)
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
