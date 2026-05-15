package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestResultEnvelopeRawAPIOutputRedactsCredentialFields(t *testing.T) {
	const (
		apiKeyCanary     = "ck_live_result_envelope_canary"
		authTokenCanary  = "webhook-result-envelope-token"
		bearerCanary     = "Bearer result-envelope-bearer"
		cookieCanary     = "clockify_session=result-envelope-cookie"
		credentialCanary = "username:result-envelope-password"
	)

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/clients" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{
			"id":          "client-1",
			"name":        "Safe Client",
			"apiKey":      apiKeyCanary,
			"authToken":   authTokenCanary,
			"bearer":      bearerCanary,
			"cookies":     cookieCanary,
			"credentials": credentialCanary,
			"nested": map[string]any{
				"api_key": apiKeyCanary,
			},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.RawAPIGet(context.Background(), map[string]any{"path": "/workspaces/{workspaceId}/clients"})
	if err != nil {
		t.Fatalf("RawAPIGet failed: %v", err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	if result.IDs["workspaceId"] != "ws1" || result.IDs["rawApiId"] != "client-1" {
		t.Fatalf("safe ids were not preserved: %+v", result.IDs)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(raw)
	for _, forbidden := range []string{
		apiKeyCanary,
		authTokenCanary,
		bearerCanary,
		cookieCanary,
		credentialCanary,
		`"apiKey"`,
		`"api_key"`,
		`"authToken"`,
		`"bearer"`,
		`"cookies"`,
		`"credentials"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("result envelope leaked %q in %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "Safe Client") {
		t.Fatalf("sanitization removed benign data: %s", text)
	}
}
