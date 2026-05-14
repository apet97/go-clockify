package main

import (
	"bytes"
	"strings"
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
	if strings.Contains(stdout.String(), "profile") || strings.Contains(stdout.String(), "tenant") || strings.Contains(stdout.String(), "policy") {
		t.Fatalf("doctor failure output reintroduced old product language:\n%s", stdout.String())
	}
}
