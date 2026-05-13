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
	if len(prompts) != 10 {
		t.Fatalf("expected 10 builtin prompts, got %d", len(prompts))
	}
	wantOrder := []string{
		"log-week-from-calendar",
		"weekly-review",
		"find-unbilled-hours",
		"find-duplicate-entries",
		"generate-timesheet-report",
		"invoice-review-and-send",
		"approval-cycle",
		"time-off-review",
		"scheduling-capacity-review",
		"webhook-rollout-check",
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

func TestBuiltinPromptsPreferStructuredToolGuidance(t *testing.T) {
	server := newPromptsTestServer()
	tests := []struct {
		name        string
		args        map[string]any
		want        []string
		notWant     []string
		description string
	}{
		{
			name: "log-week-from-calendar",
			args: map[string]any{
				"week_start":   "2026-04-06",
				"calendar_uri": "calendar://work",
			},
			want:        []string{"clockify_log_time", "allow_overlap:true"},
			notWant:     []string{"clockify_add_entry"},
			description: "calendar prompt should use the canonical finished-entry helper",
		},
		{
			name:        "weekly-review",
			args:        map[string]any{"week_start": "2026-04-06"},
			want:        []string{"clockify_timesheet_review", "clockify_weekly_summary"},
			description: "weekly review should start with the structured review helper",
		},
		{
			name:        "find-duplicate-entries",
			args:        map[string]any{},
			want:        []string{"clockify_timesheet_review", "overlap issues"},
			notWant:     []string{"clockify_list_entries", "{{lookback_days}}"},
			description: "duplicate scan should use review issues instead of manual entry loops",
		},
		{
			name: "invoice-review-and-send",
			args: map[string]any{"client": "Acme", "since": "2026-05-01", "until": "2026-05-31"},
			want: []string{"clockify_send_invoice", "dry-run preview", "external side effect"},
		},
		{
			name:        "approval-cycle",
			args:        map[string]any{"period": "2026-W19"},
			want:        []string{"clockify_list_approval_requests", "dry_run:true"},
			description: "approval recipe should require read-before-write and previews",
		},
		{
			name:        "time-off-review",
			args:        map[string]any{"user": "Ada"},
			want:        []string{"policies", "balances", "dry_run:true"},
			description: "time-off recipe should review balance context before writes",
		},
		{
			name:        "scheduling-capacity-review",
			args:        map[string]any{"user": "Ada", "week_start": "2026-05-11"},
			want:        []string{"per-user capacity totals", "dry_run:true"},
			description: "scheduling recipe should prefer typed capacity tools",
		},
		{
			name:        "webhook-rollout-check",
			args:        map[string]any{"url": "https://hooks.example.com/clockify", "event": "TIME_ENTRY_CREATED"},
			want:        []string{"DNS validation", "mask any auth token", "external side effect"},
			description: "webhook recipe should foreground DNS and token safety",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := getPromptText(t, server, tt.name, tt.args)
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Fatalf("%s: missing %q in %q", tt.description, want, text)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(text, notWant) {
					t.Fatalf("%s: unexpected %q in %q", tt.description, notWant, text)
				}
			}
		})
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

func getPromptText(t *testing.T, server *Server, name string, args map[string]any) string {
	t.Helper()
	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0", ID: 1, Method: "prompts/get",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
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
	return messages[0].Content.Text
}
