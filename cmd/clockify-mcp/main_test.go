package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
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
			wantCode: 5,
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

func respondDoctorJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}
