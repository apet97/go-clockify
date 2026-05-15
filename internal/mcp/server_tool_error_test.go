package mcp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	logslog "github.com/apet97/go-clockify/internal/logging"
)

type fakeUpstreamError struct {
	verbose string
}

func (e *fakeUpstreamError) Error() string { return e.verbose }

// TestToolErrorsExposeVerboseMessage locks in the one-user behavior: tool
// errors flow through to the MCP client unchanged for local diagnostics.
func TestToolErrorsExposeVerboseMessage(t *testing.T) {
	wantBody := "insufficient permissions " + "ten" + "ant=acme-internal"
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "leaky"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return nil, &fakeUpstreamError{
				verbose: "clockify PUT /workspaces/ws1/invoices/inv1 failed: 403 Forbidden: " + wantBody,
			}
		},
	}})

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"leaky","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, wantBody) {
		t.Fatalf("local-mode response should include verbose body, got: %s", got)
	}
}

func TestToolErrorSecretCanariesDoNotReachClientOrLogs(t *testing.T) {
	const (
		apiKeyCanary      = "canary-clockify-api-key-1234567890"
		webhookAuthCanary = "canary-webhook-auth-token-1234567890"
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request","apiKey":"` + apiKeyCanary + `","authToken":"` + webhookAuthCanary + `"}`))
	}))
	defer ts.Close()

	client := clockify.NewClient(apiKeyCanary, ts.URL, 5*time.Second, 0)
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(logslog.NewRedactingHandler(slog.NewJSONHandler(&logs, nil))))
	t.Cleanup(func() { slog.SetDefault(prev) })

	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "api_error"},
		Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			var out map[string]any
			return nil, client.Get(ctx, "/user", nil, &out)
		},
	}})

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_error","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	for _, got := range []struct {
		name  string
		value string
	}{
		{"tools/call output", out.String()},
		{"debug logs", logs.String()},
	} {
		if strings.Contains(got.value, apiKeyCanary) || strings.Contains(got.value, webhookAuthCanary) {
			t.Fatalf("%s leaked secret canary: %s", got.name, got.value)
		}
	}
}

func TestToolsCallTimeoutReturnsStructuredRecoveryEnvelope(t *testing.T) {
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "slow", InputSchema: map[string]any{"type": "object"}},
		Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}})

	server.ToolTimeout = time.Millisecond
	server.MarkInitialized("2025-11-25", "timeout-test", "test")

	resp := server.handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "slow", "arguments": map[string]any{}},
	})
	if resp.Error != nil {
		t.Fatalf("expected tool-error result, got JSON-RPC error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type=%T want map", resp.Result)
	}
	if result["isError"] != true {
		t.Fatalf("isError=%v want true", result["isError"])
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent missing or wrong type: %+v", result)
	}
	if structured["ok"] != false {
		t.Fatalf("structuredContent.ok=%v want false", structured["ok"])
	}
	errObj, ok := structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent.error missing: %+v", structured)
	}
	if errObj["code"] != "timeout" {
		t.Fatalf("error.code=%v want timeout", errObj["code"])
	}
	recovery, ok := structured["recovery"].(map[string]any)
	if !ok || recovery["hint"] == "" {
		t.Fatalf("missing recovery hint: %+v", structured)
	}
}
