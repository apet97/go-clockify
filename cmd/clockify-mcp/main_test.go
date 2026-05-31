package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/runtime"
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

func TestPrintHelpDoesNotDoublePrefixVVersions(t *testing.T) {
	oldVersion := version
	version = "v0.1.1"
	defer func() { version = oldVersion }()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = r.Close()
	})

	printHelp()
	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read help output: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "clockify-mcp vv0.1.1") {
		t.Fatalf("help output double-prefixed v version:\n%s", text)
	}
	if !strings.Contains(text, "clockify-mcp v0.1.1") {
		t.Fatalf("help output missing expected version:\n%s", text)
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

func TestNewClockifyClientUsesToolTimeoutAndRaisedResultCap(t *testing.T) {
	cfg := config.OneUserConfig{
		APIKey:             "test-key",
		BaseURL:            "http://127.0.0.1:1",
		ToolTimeout:        2 * time.Minute,
		MaxToolResultBytes: 32 * 1024 * 1024,
	}

	client := runtime.NewClockifyClient(cfg, 0)
	defer client.Close()

	if got := client.HTTPTimeout(); got != 2*time.Minute {
		t.Fatalf("HTTPTimeout = %s, want 2m", got)
	}
	if got, want := client.MaxResponseBodyBytes(), int64(32*1024*1024); got != want {
		t.Fatalf("MaxResponseBodyBytes = %d, want %d", got, want)
	}
}

func TestNewClockifyClientKeepsDefaultResponseCapWhenToolResultCapIsSmall(t *testing.T) {
	cfg := config.OneUserConfig{
		APIKey:             "test-key",
		BaseURL:            "http://127.0.0.1:1",
		ToolTimeout:        15 * time.Second,
		MaxToolResultBytes: config.DefaultMaxToolResultBytes,
	}

	client := runtime.NewClockifyClient(cfg, 0)
	defer client.Close()

	if got, want := client.MaxResponseBodyBytes(), int64(clockify.DefaultMaxResponseBodyBytes); got != want {
		t.Fatalf("MaxResponseBodyBytes = %d, want default %d", got, want)
	}
}

type slogLevelCase struct {
	known bool
}

// TestParseLogLevelDefaultsToWarn pins the stdio runtime log policy: an unset
// or unrecognised MCP_LOG_LEVEL resolves to warn so a clean MCP session leaves
// stderr quiet, while explicit known levels are still honoured.
func TestParseLogLevelDefaultsToWarn(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelWarn,
		"   ":     slog.LevelWarn,
		"garbage": slog.LevelWarn,
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for input, want := range cases {
		if got := parseLogLevel(input); got != want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestRedactIdentifierKeepsOnlySuffix(t *testing.T) {
	if got := redactIdentifier("000000000000000000000001"); got != "[redacted]...0001" {
		t.Fatalf("redactIdentifier = %q", got)
	}
	if got := redactIdentifier("abc"); got != "[redacted]" {
		t.Fatalf("short redactIdentifier = %q", got)
	}
	if got := redactIdentifier(" "); got != "(unset)" {
		t.Fatalf("empty redactIdentifier = %q", got)
	}
}

func TestRedactDoctorDiagnosticRedactsAPIKeyAndWorkspaceID(t *testing.T) {
	cfg := testOneUserConfig("test-secret-key", "000000000000000000000001")
	got := redactDoctorDiagnostic(errors.New("clockify GET /workspaces/000000000000000000000001 failed with test-secret-key"), cfg)
	if strings.Contains(got, cfg.APIKey) || strings.Contains(got, cfg.WorkspaceID) {
		t.Fatalf("diagnostic leaked secret: %q", got)
	}
	if !strings.Contains(got, "[redacted]...0001") {
		t.Fatalf("diagnostic missing redacted workspace suffix: %q", got)
	}
}

func TestRunDoctorOneUserSuccessRedactsAPIKey(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
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
		"CLOCKIFY_WORKSPACE_ID  [redacted]...0001",
		"CLOCKIFY_TIMEZONE      Europe/Belgrade",
		"MCP_LOG_LEVEL          debug",
		"CLOCKIFY_TOOL_TIMEOUT  45s",
		"CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS  4",
		"CLOCKIFY_MAX_MESSAGE_SIZE          4194304",
		"CLOCKIFY_MAX_TOOL_RESULT_BYTES     50000",
		"CLOCKIFY_TOOLSET                   default",
		"Registry loaded:    156 tools",
		"Advertised surface: 16 tools (toolset=default)",
		"CLOCKIFY_ENABLE_RAW_WRITES         false",
		"CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   (none)",
		"Result: OK",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") ||
		strings.Contains(out, "000000000000000000000001") || strings.Contains(stderr.String(), "000000000000000000000001") {
		t.Fatalf("doctor leaked API key or full workspace ID: stdout=%s stderr=%s", out, stderr.String())
	}
}

func TestRunDoctorLiveSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondDoctorJSON(t, w, map[string]any{"id": "user1"})
		case r.URL.Path == "/workspaces/000000000000000000000001":
			respondDoctorJSON(t, w, map[string]any{
				"id":                      "000000000000000000000001",
				"name":                    "Pinned",
				"featureSubscriptionType": "PRO",
				"features":                []string{"INVOICE", "EXPENSE", "TIME_OFF", "SCHEDULING", "APPROVAL", "WEBHOOK", "CUSTOM_FIELD", "REPORT"},
			})
		case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "OWNER":
			respondDoctorJSON(t, w, []map[string]any{{"id": "000000000000000000000001", "name": "Pinned"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	t.Setenv("CLOCKIFY_API_KEY", "test-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
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
		"Registry loaded:    156 tools",
		"Advertised surface: 150 tools (toolset=admin)",
		"CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS   hooks.example.com,.trusted.test",
		"GET /user                         OK",
		"user_id: [redacted]...ser1",
		"GET /workspaces/{workspaceId}     OK",
		"workspace_id: [redacted]...0001",
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
	if strings.Contains(out, "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") ||
		strings.Contains(out, "000000000000000000000001") || strings.Contains(stderr.String(), "000000000000000000000001") {
		t.Fatalf("doctor leaked API key or full workspace ID: stdout=%s stderr=%s", out, stderr.String())
	}
	if strings.Contains(out, "user1") || strings.Contains(stderr.String(), "user1") {
		t.Fatalf("doctor leaked full user ID: stdout=%s stderr=%s", out, stderr.String())
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
				case r.URL.Path == "/workspaces/000000000000000000000001":
					respondDoctorJSON(t, w, map[string]any{"id": "000000000000000000000001", "name": "Pinned"})
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
				case r.URL.Path == "/workspaces/000000000000000000000001":
					respondDoctorJSON(t, w, map[string]any{"id": "000000000000000000000001", "name": "Pinned"})
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
			t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
			t.Setenv("CLOCKIFY_BASE_URL", upstream.URL)

			var stdout, stderr bytes.Buffer
			code := runDoctor([]string{"--live"}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("runDoctor exit=%d want %d stderr=%s stdout=%s", code, tt.wantCode, stderr.String(), stdout.String())
			}
			if strings.Contains(stdout.String(), "test-secret-key") || strings.Contains(stderr.String(), "test-secret-key") {
				t.Fatalf("doctor leaked API key: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
			if strings.Contains(stdout.String(), "000000000000000000000001") || strings.Contains(stderr.String(), "000000000000000000000001") {
				t.Fatalf("doctor leaked full workspace ID: stdout=%s stderr=%s", stdout.String(), stderr.String())
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
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
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
	fake := testclockify.NewServer("000000000000000000000001")
	defer fake.Close()

	t.Setenv("CLOCKIFY_API_KEY", "stdio-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", fake.WorkspaceID)
	t.Setenv("CLOCKIFY_BASE_URL", fake.URL)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"cmd-smoke","version":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"clockify_tools_guide","arguments":{}}}`,
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
	if len(tools) != 16 {
		t.Fatalf("tools/list count=%d want 16", len(tools))
	}

	call := responses[3]
	structured := resultObject(t, call, "structuredContent")
	if structured["ok"] != true || structured["action"] != "clockify_tools_guide" {
		t.Fatalf("bad tools-guide structuredContent: %+v", structured)
	}
	ids := mapObject(t, structured, "ids")
	if ids["workspaceId"] != fake.WorkspaceID {
		t.Fatalf("tools-guide missing workspace ID: %+v", ids)
	}
}

func TestRunWithContextDefaultToolsetRejectsUnadvertisedDispatch(t *testing.T) {
	fake := testclockify.NewServer("000000000000000000000001")
	defer fake.Close()

	t.Setenv("CLOCKIFY_API_KEY", "stdio-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", fake.WorkspaceID)
	t.Setenv("CLOCKIFY_BASE_URL", fake.URL)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"cmd-smoke","version":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_clients_create","arguments":{"name":"Hidden Client"}}}`,
	}, "\n") + "\n"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	if err := runWithContext(ctx, strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("runWithContext: %v\nstdout=%s", err, stdout.String())
	}
	responses := decodeRunResponses(t, stdout.Bytes())
	call := responses[2]
	if call.Error == nil {
		t.Fatalf("expected hidden default tool to be rejected, got: %+v", call)
	}
	if call.Error["code"] != float64(-32602) || !strings.Contains(stringFromTestAny(call.Error["message"]), "unknown tool: clockify_clients_create") {
		t.Fatalf("unexpected hidden-tool error: %+v", call.Error)
	}
}

func TestRunWithContextStartupLogRedactsWorkspaceID(t *testing.T) {
	fake := testclockify.NewServer("000000000000000000000001")
	defer fake.Close()

	t.Setenv("CLOCKIFY_API_KEY", "stdio-secret-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", fake.WorkspaceID)
	t.Setenv("CLOCKIFY_BASE_URL", fake.URL)

	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runWithContext(ctx, strings.NewReader(""), io.Discard); err != nil {
		t.Fatalf("runWithContext: %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, "one_user_server_start") {
		t.Fatalf("startup log missing one_user_server_start: %s", out)
	}
	if strings.Contains(out, "stdio-secret-key") || strings.Contains(out, fake.WorkspaceID) {
		t.Fatalf("startup log leaked secret: %s", out)
	}
	if !strings.Contains(out, "[redacted]...0001") {
		t.Fatalf("startup log missing workspace suffix hint: %s", out)
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

func stringFromTestAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
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
	const wsID = "000000000000000000000001"
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
				case r.URL.Path == "/workspaces" && r.URL.Query().Get("roles") == "WORKSPACE_ADMIN":
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
			if strings.Contains(stdout.String(), wsID) || strings.Contains(stderr.String(), wsID) {
				t.Fatal("doctor leaked the full workspace ID")
			}
			if strings.Contains(stdout.String(), "u1") || strings.Contains(stderr.String(), "u1") {
				t.Fatal("doctor leaked the full user ID")
			}
		})
	}
}

func testOneUserConfig(apiKey, workspaceID string) config.OneUserConfig {
	return config.OneUserConfig{APIKey: apiKey, WorkspaceID: workspaceID}
}
