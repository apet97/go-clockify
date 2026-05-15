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
	}}, nil, nil)

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
	}}, nil, nil)

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
