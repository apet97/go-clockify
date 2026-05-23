package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/safety"
)

func TestHighRiskCallWithoutTokenFailsBeforeHandler(t *testing.T) {
	called := false
	srv := confirmationTestServer(func(context.Context, map[string]any) (any, error) {
		called = true
		return map[string]any{"ok": true, "action": "danger"}, nil
	})

	res, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("handler ran before confirmation")
	}
	env := res.(map[string]any)
	if env["ok"] != false {
		t.Fatalf("ok = %v, want false", env["ok"])
	}
	errMap, _ := env["error"].(map[string]any)
	if got := errMap["code"]; got != "confirmation_required" {
		t.Fatalf("error.code = %v", got)
	}
}

func TestHighRiskDryRunIssuesConfirmationToken(t *testing.T) {
	calls := 0
	srv := confirmationTestServer(func(_ context.Context, args map[string]any) (any, error) {
		calls++
		if args["dry_run"] != true {
			t.Fatalf("handler args dry_run = %v", args["dry_run"])
		}
		return map[string]any{"ok": true, "action": "danger", "dry_run": true}, nil
	})

	res, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t1", "dry_run": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	env := res.(map[string]any)
	confirmation, _ := env["confirmation"].(map[string]any)
	token, _ := confirmation["confirm_token"].(string)
	if len(token) < 16 || strings.Contains(token, "t1") {
		t.Fatalf("bad confirmation token: %q", token)
	}
	if confirmation["required"] != true || confirmation["preview_hash"] == "" || confirmation["expires_at"] == "" {
		t.Fatalf("bad confirmation metadata: %#v", confirmation)
	}
}

func TestCentralDryRunPreviewDoesNotCallHandler(t *testing.T) {
	called := false
	srv := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "central_preview",
			Description: "central preview",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"dry_run": map[string]any{"type": "boolean"}},
			},
		},
		Handler: func(context.Context, map[string]any) (any, error) {
			called = true
			return map[string]any{"ok": true}, nil
		},
		RiskClass:            RiskAdmin,
		CentralDryRunPreview: true,
	}})
	srv.ConfirmationStore = safety.NewTokenStore(safety.TokenStoreOptions{})
	srv.WorkspaceIDForSafety = "workspace_123"

	res, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "central_preview",
		Arguments: map[string]any{"dry_run": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("central dry-run preview should not invoke handler")
	}
	env := res.(map[string]any)
	if env["dry_run"] != true || env["changed"] != false {
		t.Fatalf("bad central dry-run envelope: %#v", env)
	}
	if env["confirmation"] == nil {
		t.Fatalf("missing confirmation metadata: %#v", env)
	}
}

func TestHighRiskConfirmedCallStripsTokenBeforeHandler(t *testing.T) {
	var token string
	srv := confirmationTestServer(func(_ context.Context, args map[string]any) (any, error) {
		if args["dry_run"] == true {
			return map[string]any{"ok": true, "action": "danger", "dry_run": true}, nil
		}
		if _, ok := args["confirm_token"]; ok {
			t.Fatal("handler received confirm_token")
		}
		return map[string]any{"ok": true, "action": "danger", "confirmed": true}, nil
	})
	preview, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t1", "dry_run": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	token = preview.(map[string]any)["confirmation"].(map[string]any)["confirm_token"].(string)

	res, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t1", "confirm_token": token},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["confirmed"] != true {
		t.Fatalf("confirmed result = %#v", res)
	}
}

func TestHighRiskMismatchedTokenFailsBeforeHandler(t *testing.T) {
	liveCalls := 0
	srv := confirmationTestServer(func(_ context.Context, args map[string]any) (any, error) {
		if args["dry_run"] == true {
			return map[string]any{"ok": true, "action": "danger", "dry_run": true}, nil
		}
		liveCalls++
		return map[string]any{"ok": true, "action": "danger"}, nil
	})
	preview, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t1", "dry_run": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := preview.(map[string]any)["confirmation"].(map[string]any)["confirm_token"].(string)
	res, err := srv.callTool(context.Background(), ToolCallParams{
		Name:      "danger",
		Arguments: map[string]any{"target_id": "t2", "confirm_token": token},
	})
	if err != nil {
		t.Fatal(err)
	}
	if liveCalls != 0 {
		t.Fatalf("live handler calls = %d, want 0", liveCalls)
	}
	env := res.(map[string]any)
	errMap, _ := env["error"].(map[string]any)
	if got := errMap["code"]; got != "confirmation_mismatch" {
		t.Fatalf("error.code = %v, result %#v", got, env)
	}
}

func confirmationTestServer(handler ToolHandler) *Server {
	srv := NewServer("test", []ToolDescriptor{{
		Tool: Tool{
			Name:        "danger",
			Description: "danger",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"target_id":     map[string]any{"type": "string"},
					"dry_run":       map[string]any{"type": "boolean"},
					"confirm_token": map[string]any{"type": "string"},
				},
			},
		},
		Handler:   handler,
		RiskClass: RiskDestructive,
	}})
	srv.ConfirmationStore = safety.NewTokenStore(safety.TokenStoreOptions{
		TTL: time.Minute,
		Now: func() time.Time {
			return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
		},
	})
	srv.WorkspaceIDForSafety = "workspace_123"
	return srv
}
