package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

func TestSearchToolsActivateGroupViaMCP(t *testing.T) {
	client := clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0)
	service := tools.New(client, "ws1")
	registry := service.Registry()

	tier1Names := make(map[string]bool, len(registry))
	for _, d := range registry {
		tier1Names[d.Tool.Name] = true
	}

	bc := bootstrap.Config{Mode: bootstrap.Minimal}
	bc.SetTier1Tools(tier1Names)

	pipeline := &enforcement.Pipeline{Bootstrap: &bc}
	gate := &enforcement.Gate{Bootstrap: &bc}
	server := mcp.NewServer("test", registry, pipeline, gate)
	service.ActivateGroup = func(group string) (tools.ActivationResult, error) {
		descriptors, ok := service.Tier2Handlers(group)
		if !ok {
			return tools.ActivationResult{}, fmt.Errorf("unknown group: %s", group)
		}
		if err := server.ActivateGroup(group, descriptors); err != nil {
			return tools.ActivationResult{}, err
		}
		return tools.ActivationResult{
			Kind:      "group",
			Name:      group,
			Group:     group,
			ToolCount: len(descriptors),
		}, nil
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_search_tools","arguments":{"activate_group":"invoices"}}}`,
	}, "\n")

	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected initialize response, activation response, and notification; got %d lines: %s", len(lines), out.String())
	}

	var sawNotification bool
	for _, line := range lines {
		var resp mcp.Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		if resp.Method == "notifications/tools/list_changed" {
			sawNotification = true
			continue
		}
	}

	if !sawNotification {
		t.Fatal("expected notifications/tools/list_changed after activation")
	}

	var listOut strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`), &listOut); err != nil {
		t.Fatalf("tools/list run failed: %v", err)
	}

	var listResp mcp.Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(listOut.String())), &listResp); err != nil {
		t.Fatalf("unmarshal tools/list: %v", err)
	}
	result, ok := listResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result for tools/list, got %T", listResp.Result)
	}
	toolsList, _ := result["tools"].([]any)

	visible := map[string]bool{}
	for _, raw := range toolsList {
		toolMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := toolMap["name"].(string)
		visible[name] = true
	}
	if !visible["clockify_list_invoices"] {
		t.Fatal("expected invoice tools to become visible after activation")
	}
}
