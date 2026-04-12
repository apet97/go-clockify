package mcp

import (
	"context"
	"strings"
	"testing"
)

func newPromptsTestServer() *Server {
	server := NewServer("test", nil, nil, nil)
	server.initialized.Store(true)
	return server
}

func TestPromptsListReturnsBuiltins(t *testing.T) {
	server := newPromptsTestServer()
	resp := server.handle(context.Background(), Request{JSONRPC: "2.0", ID: 1, Method: "prompts/list"})
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	prompts, _ := result["prompts"].([]Prompt)
	if len(prompts) != 5 {
		t.Fatalf("expected 5 builtin prompts, got %d", len(prompts))
	}
	wantOrder := []string{
		"log-week-from-calendar",
		"weekly-review",
		"find-unbilled-hours",
		"find-duplicate-entries",
		"generate-timesheet-report",
	}
	for i, want := range wantOrder {
		if prompts[i].Name != want {
			t.Fatalf("position %d: got %q want %q", i, prompts[i].Name, want)
		}
	}
}

func TestPromptsGetSubstitutesArguments(t *testing.T) {
	server := newPromptsTestServer()
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "prompts/get",
		Params: map[string]any{
			"name":      "weekly-review",
			"arguments": map[string]any{"week_start": "2026-04-06"},
		},
	})
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	messages, _ := result["messages"].([]PromptMessage)
	if len(messages) != 1 {
		t.Fatalf("messages: %+v", messages)
	}
	if !strings.Contains(messages[0].Content.Text, "2026-04-06") {
		t.Fatalf("substitution missing: %q", messages[0].Content.Text)
	}
}

func TestPromptsGetMissingRequiredArgument(t *testing.T) {
	server := newPromptsTestServer()
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "prompts/get",
		Params: map[string]any{
			"name":      "weekly-review",
			"arguments": map[string]any{},
		},
	})
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("expected -32602 for missing arg, got %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "week_start") {
		t.Fatalf("error should name the missing arg: %q", resp.Error.Message)
	}
}

func TestPromptsGetUnknownPromptRejected(t *testing.T) {
	server := newPromptsTestServer()
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "prompts/get",
		Params: map[string]any{"name": "nonexistent"},
	})
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("expected -32602 for unknown prompt, got %+v", resp.Error)
	}
}

func TestInitializeAdvertisesPromptsCapability(t *testing.T) {
	server := NewServer("test", nil, nil, nil)
	result := server.handleInitialize(map[string]any{})
	caps := result["capabilities"].(map[string]any)
	prompts, ok := caps["prompts"].(map[string]any)
	if !ok {
		t.Fatalf("prompts capability missing: %+v", caps)
	}
	if prompts["listChanged"] != true {
		t.Fatalf("listChanged flag: %+v", prompts)
	}
}
