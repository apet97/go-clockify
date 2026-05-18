package tools

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var envVarToken = regexp.MustCompile(`CLOCKIFY_[A-Z0-9_]+`)

// envVarLintExceptions are env tokens that legitimately appear in README but
// are not read by the config parser (live-test harness vars, doc examples).
var envVarLintExceptions = map[string]bool{
	"CLOCKIFY_RUN_LIVE_E2E":              true,
	"CLOCKIFY_LIVE_PREFIX":               true,
	"CLOCKIFY_LIVE_WORKSPACE_CONFIRM":    true,
	"CLOCKIFY_LIVE_OPTIONAL_DOMAINS":     true,
	"CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS":  true,
	"CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS": true,
	"CLOCKIFY_LIVE_ADMIN_ENABLED":        true,
	"CLOCKIFY_LIVE_BILLING_ENABLED":      true,
	"CLOCKIFY_LIVE_SETTINGS_ENABLED":     true,
}

func TestReadmeEnvVarsExistInConfigParser(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	configSrc, err := os.ReadFile("../../internal/config/oneuser.go")
	if err != nil {
		t.Fatalf("read config source: %v", err)
	}
	mainSrc, err := os.ReadFile("../../cmd/clockify-mcp/main.go")
	if err != nil {
		t.Fatalf("read main source: %v", err)
	}
	sources := string(configSrc) + string(mainSrc)

	seen := map[string]bool{}
	for _, token := range envVarToken.FindAllString(string(readme), -1) {
		if seen[token] || envVarLintExceptions[token] {
			continue
		}
		seen[token] = true
		if !strings.Contains(sources, `"`+token+`"`) {
			t.Errorf("README mentions env var %q but no parser in internal/config/oneuser.go or cmd/clockify-mcp/main.go reads it", token)
		}
	}
}
