package mcp

import (
	"context"
	"strings"
	"testing"
)

func newPromptsTestServer() *Server {
	server := NewServer("test", nil)
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
	if len(prompts) != 11 {
		t.Fatalf("expected 11 builtin prompts, got %d", len(prompts))
	}
	wantOrder := []string{
		"demo-full-workspace-story",
		"setup-client-project-task",
		"log-week",
		"safe-daily-time-tracking",
		"invoice-client",
		"cleanup-demo",
		"review-week",
		"create-expense",
		"request-time-off",
		"schedule-week",
		"setup-webhook",
	}
	for i, want := range wantOrder {
		if prompts[i].Name != want {
			t.Fatalf("position %d: got %q want %q", i, prompts[i].Name, want)
		}
	}
}

func TestPromptsGetWorksForEachBuiltin(t *testing.T) {
	server := newPromptsTestServer()
	for _, name := range []string{
		"demo-full-workspace-story",
		"setup-client-project-task",
		"log-week",
		"safe-daily-time-tracking",
		"invoice-client",
		"cleanup-demo",
		"review-week",
		"create-expense",
		"request-time-off",
		"schedule-week",
		"setup-webhook",
	} {
		t.Run(name, func(t *testing.T) {
			resp := server.handle(context.Background(), Request{
				JSONRPC: "2.0", ID: 1, Method: "prompts/get",
				Params: map[string]any{"name": name, "arguments": map[string]any{}},
			})
			if resp.Error != nil {
				t.Fatalf("error: %+v", resp.Error)
			}
			result := resp.Result.(map[string]any)
			messages, _ := result["messages"].([]PromptMessage)
			if len(messages) != 1 {
				t.Fatalf("messages: %+v", messages)
			}
			if strings.Contains(messages[0].Content.Text, "{{") {
				t.Fatalf("prompt left an unsubstituted placeholder: %q", messages[0].Content.Text)
			}
		})
	}
}

func TestBuiltinPromptsPreferStructuredToolGuidance(t *testing.T) {
	server := newPromptsTestServer()
	firstTools := map[string]string{
		"demo-full-workspace-story": "clockify_demo_seed",
		"setup-client-project-task": "clockify_create_work_package",
		"log-week":                  "clockify_log_work",
		"safe-daily-time-tracking":  "clockify_status",
		"invoice-client":            "clockify_status",
		"cleanup-demo":              "clockify_demo_cleanup",
		"review-week":               "clockify_review_week",
		"create-expense":            "clockify_record_expense",
		"request-time-off":          "clockify_request_time_off",
		"schedule-week":             "clockify_schedule_work",
		"setup-webhook":             "clockify_setup_webhook",
	}
	for name, firstTool := range firstTools {
		t.Run(name, func(t *testing.T) {
			text := getPromptText(t, server, name, map[string]any{})
			for _, want := range []string{"First call `" + firstTool + "`", "Use returned IDs", "paid feature is unavailable", "Summarize what changed"} {
				if !strings.Contains(text, want) {
					t.Fatalf("prompt %q missing %q in %q", name, want, text)
				}
			}
			for _, notWant := range []string{
				"confirm" + "ation",
				"activate",
				"ten" + "ant",
				"hos" + "ted",
				"policy " + "mode",
				"Tier " + "2",
			} {
				if strings.Contains(strings.ToLower(text), strings.ToLower(notWant)) {
					t.Fatalf("prompt %q contains forbidden language %q in %q", name, notWant, text)
				}
			}
		})
	}
}

func TestBuiltinPromptsEncodeOwnerModeSafetyContracts(t *testing.T) {
	server := newPromptsTestServer()
	cases := map[string]struct {
		want    []string
		notWant []string
	}{
		"safe-daily-time-tracking": {
			want: []string{
				"only core time-tracking tools",
				"Do not use invoice, expense, admin, webhook, or raw fallback tools",
			},
		},
		"invoice-client": {
			want: []string{
				"First call `clockify_status` to check workspace feature availability",
				"currency",
			},
		},
		"create-expense": {
			want: []string{
				"receipt/file details only when the expense tool schema requires them",
				"paid feature is unavailable",
			},
		},
		"setup-webhook": {
			want: []string{
				"dry_run=true",
				"Review the preview and DNS/URL validation result before making a non-dry-run create call",
			},
			notWant: []string{"first make a non-dry-run"},
		},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			text := getPromptText(t, server, name, map[string]any{})
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Fatalf("prompt %q missing %q in %q", name, want, text)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(strings.ToLower(text), strings.ToLower(notWant)) {
					t.Fatalf("prompt %q contains forbidden guidance %q in %q", name, notWant, text)
				}
			}
		})
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

func TestSubstituteArgsPrefersLongestPlaceholderNames(t *testing.T) {
	got := substituteArgs("{{a}} / {{a}}b}}", map[string]any{
		"a":    "SHORT",
		"a}}b": "LONG",
	})
	if got != "SHORT / LONG" {
		t.Fatalf("substituteArgs overlap = %q", got)
	}
}

func TestInitializeAdvertisesPromptsCapability(t *testing.T) {
	server := NewServer("test", nil)
	result := server.handleInitialize(map[string]any{})
	caps := result["capabilities"].(map[string]any)
	prompts, ok := caps["prompts"].(map[string]any)
	if !ok {
		t.Fatalf("prompts capability missing: %+v", caps)
	}
	if _, ok := prompts["listChanged"]; ok {
		t.Fatalf("static prompts should not advertise listChanged: %+v", prompts)
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
