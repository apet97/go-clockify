package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/testclockify"
)

func TestEffectiveVersionFallsBackToDev(t *testing.T) {
	old := version
	version = "dev"
	defer func() { version = old }()

	if got := effectiveVersion(); got == "" {
		t.Fatal("effectiveVersion returned empty string")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := map[string]slogLevelCase{
		"debug":   {known: true},
		"info":    {known: true},
		"warn":    {known: true},
		"warning": {known: true},
		"error":   {known: true},
		"weird":   {known: false},
	}
	for input, tt := range tests {
		if got := isKnownLogLevel(input); got != tt.known {
			t.Fatalf("isKnownLogLevel(%q)=%v want %v", input, got, tt.known)
		}
	}
}

type slogLevelCase struct {
	known bool
}

func TestRunDoctorOneUserSuccessRedactsAPIKey(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
	t.Setenv("CLOCKIFY_TIMEZONE", "Europe/Belgrade")
	t.Setenv("CLOCKIFY_BASE_URL", "https://api.clockify.me/api/v1")
	t.Setenv("MCP_LOG_LEVEL", "debug")

	var stdout, stderr bytes.Buffer
	code := runDoctor(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDoctor exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"one-user configuration",
		"CLOCKIFY_API_KEY       set (redacted)",
		"CLOCKIFY_WORKSPACE_ID  65b382b606de527a7ee2b60e",
		"CLOCKIFY_TIMEZONE      Europe/Belgrade",
		"MCP_LOG_LEVEL          debug",
		"CLOCKIFY_TOOL_TIMEOUT  45s",
		"CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS  4",
		"CLOCKIFY_MAX_MESSAGE_SIZE          4194304",
		"CLOCKIFY_MAX_TOOL_RESULT_BYTES     50000",
		"CLOCKIFY_TOOLSET                   all",
		"CLOCKIFY_ENABLE_RAW_WRITES         false",
		"CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   (none)",
		"Result: OK",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") {
		t.Fatalf("doctor leaked API key: stdout=%s stderr=%s", out, stderr.String())
	}
}

func TestRunDoctorLiveSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondDoctorJSON(t, w, map[string]any{"id": "user1"})
		case r.URL.Path == "/workspaces/65b382b606de527a7ee2b60e":
			respondDoctorJSON(t, w, map[string]any{
				"id":                      "65b382b606de527a7ee2b60e",
				"name":                    "Pinned",
				"featureSubscriptionType": "PRO",
				"features":                []string{"INVOICE", "EXPENSE", "TIME_OFF", "SCHEDULING", "APPROVAL", "WEBHOOK", "CUSTOM_FIELD", "REPORT"},
			})
		case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
			respondDoctorJSON(t, w, []map[string]any{{"id": "65b382b606de527a7ee2b60e", "name": "Pinned"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
	t.Setenv("CLOCKIFY_BASE_URL", upstream.URL)
	t.Setenv("CLOCKIFY_TOOLSET", "admin")
	t.Setenv("CLOCKIFY_ENABLE_RAW_WRITES", "true")
	t.Setenv("CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS", " hooks.example.com, .trusted.test ")

	var stdout, stderr bytes.Buffer
	code := runDoctor([]string{"--live"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDoctor exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"CLOCKIFY_ENABLE_RAW_WRITES         true",
		"CLOCKIFY_TOOLSET                   admin",
		"CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   hooks.example.com,.trusted.test",
		"GET /user                         OK",
		"user_id: user1",
		"GET /workspaces/{workspaceId}     OK",
		"workspace_id: 65b382b606de527a7ee2b60e",
		"workspace_name: Pinned",
		"feature_plan: PRO",
		"feature_flags: INVOICE,EXPENSE,TIME_OFF,SCHEDULING,APPROVAL,WEBHOOK,CUSTOM_FIELD,REPORT",
		"optional_features: invoices=available,expenses=available,timeOff=available,scheduling=available,approvals=available,webhooks=available,customFields=available,reports=available",
		"GET /workspaces?roles=OWNER       OK",
		"Result: OK",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor live output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") {
		t.Fatalf("doctor leaked API key: stdout=%s stderr=%s", out, stderr.String())
	}
}

func TestRunDoctorLiveExitCodes(t *testing.T) {
	tests := map[string]struct {
		handler  http.HandlerFunc
		wantCode int
	}{
		"auth": {
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			wantCode: 3,
		},
		"workspace missing": {
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user" {
					respondDoctorJSON(t, w, map[string]any{"id": "user1"})
					return
				}
				http.NotFound(w, r)
			},
			wantCode: 4,
		},
		"non owner": {
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "user1"})
				case r.URL.Path == "/workspaces/65b382b606de527a7ee2b60e":
					respondDoctorJSON(t, w, map[string]any{"id": "65b382b606de527a7ee2b60e", "name": "Pinned"})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
					respondDoctorJSON(t, w, []map[string]any{{"id": "other", "name": "Other"}})
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 0,
		},
		"owner check forbidden": {
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "user1"})
				case r.URL.Path == "/workspaces/65b382b606de527a7ee2b60e":
					respondDoctorJSON(t, w, map[string]any{"id": "65b382b606de527a7ee2b60e", "name": "Pinned"})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
					http.Error(w, "forbidden", http.StatusForbidden)
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 3,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			upstream := httptest.NewServer(tt.handler)
			defer upstream.Close()

			t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
			t.Setenv("CLOCKIFY_BASE_URL", upstream.URL)

			var stdout, stderr bytes.Buffer
			code := runDoctor([]string{"--live"}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("runDoctor exit=%d want %d stderr=%s stdout=%s", code, tt.wantCode, stderr.String(), stdout.String())
			}
			if strings.Contains(stdout.String(), "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") {
				t.Fatalf("doctor leaked API key: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunDoctorDoesNotCallClockify(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected network call", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "65b382b606de527a7ee2b60e")
	t.Setenv("CLOCKIFY_BASE_URL", upstream.URL)

	var stdout, stderr bytes.Buffer
	code := runDoctor(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDoctor exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("doctor made %d upstream calls", got)
	}
}

func TestRunDoctorOneUserMissingConfig(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "")

	var stdout, stderr bytes.Buffer
	code := runDoctor(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runDoctor exit=%d want 2", code)
	}
	if !strings.Contains(stdout.String(), "Result: ERROR") || !strings.Contains(stdout.String(), "Recovery:") {
		t.Fatalf("doctor failure output missing recovery:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "profile") || strings.Contains(stdout.String(), "ten"+"ant") || strings.Contains(stdout.String(), "policy") {
		t.Fatalf("doctor failure output reintroduced old product language:\n%s", stdout.String())
	}
}

func TestRunWithContextStdioSmokeUsesCommandWiring(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()

	t.Setenv("CLOCKIFY_API_KEY", "stdio-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", fake.WorkspaceID)
	t.Setenv("CLOCKIFY_BASE_URL", fake.URL)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"cmd-smoke","version":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"clockify_clients_create","arguments":{"name":"Command Smoke Client"}}}`,
	}, "\n") + "\n"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	if err := runWithContext(ctx, strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("runWithContext: %v\nstdout=%s", err, stdout.String())
	}
	if strings.Contains(stdout.String(), "stdio-secret-key") {
		t.Fatalf("stdio smoke leaked API key: %s", stdout.String())
	}

	responses := decodeRunResponses(t, stdout.Bytes())
	if len(responses) != 3 {
		t.Fatalf("response count=%d want 3: %s", len(responses), stdout.String())
	}
	initialize := responses[1]
	if initialize.Error != nil {
		t.Fatalf("initialize error: %+v", initialize.Error)
	}
	capabilities := resultObject(t, initialize, "capabilities")
	toolsCap := mapObject(t, capabilities, "tools")
	if _, ok := toolsCap["listChanged"]; ok {
		t.Fatalf("production command should not advertise tools.listChanged with StaticToolList=true: %+v", toolsCap)
	}

	list := responses[2]
	tools := arrayField(t, resultObject(t, list, ""), "tools")
	if len(tools) != 156 {
		t.Fatalf("tools/list count=%d want 156", len(tools))
	}

	call := responses[3]
	structured := resultObject(t, call, "structuredContent")
	if structured["ok"] != true || structured["action"] != "clockify_clients_create" {
		t.Fatalf("bad client-create structuredContent: %+v", structured)
	}
	ids := mapObject(t, structured, "ids")
	if ids["workspaceId"] != fake.WorkspaceID || ids["clientId"] == "" {
		t.Fatalf("client-create missing IDs: %+v", ids)
	}
}

type testRPCResponse struct {
	ID     int            `json:"id"`
	Result map[string]any `json:"result"`
	Error  map[string]any `json:"error,omitempty"`
}

func decodeRunResponses(t *testing.T, raw []byte) map[int]testRPCResponse {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	out := map[int]testRPCResponse{}
	for {
		var resp testRPCResponse
		if err := dec.Decode(&resp); err != nil {
			if errors.Is(err, io.EOF) {
				return out
			}
			t.Fatalf("decode response stream: %v\nraw=%s", err, string(raw))
		}
		out[resp.ID] = resp
	}
}

func resultObject(t *testing.T, resp testRPCResponse, key string) map[string]any {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("response %d returned error: %+v", resp.ID, resp.Error)
	}
	if key == "" {
		return resp.Result
	}
	return mapObject(t, resp.Result, key)
}

func mapObject(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := parent[key]
	if !ok {
		t.Fatalf("missing object key %q in %+v", key, parent)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("%s type=%T want object: %+v", key, raw, raw)
	}
	return obj
}

func arrayField(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()
	raw, ok := parent[key]
	if !ok {
		t.Fatalf("missing array key %q in %+v", key, parent)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s type=%T want array: %+v", key, raw, raw)
	}
	return arr
}

func respondDoctorJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

func TestWriteStartupErrorAddsDoctorHintForConfigErrors(t *testing.T) {
	t.Run("config error suggests doctor", func(t *testing.T) {
		var buf bytes.Buffer
		writeStartupError(&buf, &startupConfigError{errors.New("CLOCKIFY_API_KEY is required")})
		out := buf.String()
		if !strings.Contains(out, "error: CLOCKIFY_API_KEY is required") {
			t.Fatalf("missing error line: %q", out)
		}
		if !strings.Contains(out, "clockify-mcp doctor") {
			t.Fatalf("config error missing doctor hint: %q", out)
		}
	})
	t.Run("runtime error does not suggest doctor", func(t *testing.T) {
		var buf bytes.Buffer
		writeStartupError(&buf, errors.New("stdio transport closed"))
		out := buf.String()
		if !strings.Contains(out, "error: stdio transport closed") {
			t.Fatalf("missing error line: %q", out)
		}
		if strings.Contains(out, "doctor") {
			t.Fatalf("runtime error should not suggest doctor: %q", out)
		}
	})
}

func TestRunWithContextClassifiesConfigError(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "")

	err := runWithContext(context.Background(), strings.NewReader(""), io.Discard)
	if err == nil {
		t.Fatal("expected a config-load error, got nil")
	}
	var cfgErr *startupConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("config-load failure not classified as *startupConfigError: %T %v", err, err)
	}
}

func TestRunDoctorLiveRoleVerdicts(t *testing.T) {
	const wsID = "65b382b606de527a7ee2b60e"
	cases := []struct {
		name     string
		handler  http.HandlerFunc
		wantCode int
		wantText string
	}{
		{
			name: "owner",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "u1"})
				case r.URL.Path == "/workspaces/"+wsID:
					respondDoctorJSON(t, w, map[string]any{"id": wsID, "name": "Pinned"})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
					respondDoctorJSON(t, w, []map[string]any{{"id": wsID}})
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 0,
			wantText: "OK (owner)",
		},
		{
			name: "workspace_admin",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "u1"})
				case r.URL.Path == "/workspaces/"+wsID:
					respondDoctorJSON(t, w, map[string]any{"id": wsID, "name": "Pinned"})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
					respondDoctorJSON(t, w, []map[string]any{})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "ADMIN":
					respondDoctorJSON(t, w, []map[string]any{{"id": wsID}})
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 0,
			wantText: "OK (workspace_admin)",
		},
		{
			name: "member_or_unknown",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "u1"})
				case "/workspaces/" + wsID:
					respondDoctorJSON(t, w, map[string]any{"id": wsID, "name": "Pinned"})
				case "/workspaces":
					respondDoctorJSON(t, w, []map[string]any{})
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 0,
			wantText: "OK with warning (member_or_unknown)",
		},
		{
			name: "auth_failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			wantCode: 3,
		},
		{
			name: "workspace_failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user" {
					respondDoctorJSON(t, w, map[string]any{"id": "u1"})
					return
				}
				http.NotFound(w, r)
			},
			wantCode: 4,
		},
		{
			name: "owner_probe_server_error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondDoctorJSON(t, w, map[string]any{"id": "u1"})
				case r.URL.Path == "/workspaces/"+wsID:
					respondDoctorJSON(t, w, map[string]any{"id": wsID, "name": "Pinned"})
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
					http.Error(w, "boom", http.StatusInternalServerError)
				default:
					http.NotFound(w, r)
				}
			},
			wantCode: 5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(tc.handler)
			defer upstream.Close()
			t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
			t.Setenv("CLOCKIFY_WORKSPACE_ID", wsID)
			t.Setenv("CLOCKIFY_BASE_URL", upstream.URL)
			var stdout, stderr bytes.Buffer
			code := runDoctor([]string{"--live"}, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout=%s\nstderr=%s", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if tc.wantText != "" && !strings.Contains(stdout.String(), tc.wantText) {
				t.Fatalf("stdout missing %q:\n%s", tc.wantText, stdout.String())
			}
			if strings.Contains(stdout.String(), "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") {
				t.Fatal("doctor leaked the API key")
			}
		})
	}
}
