package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDockerfileDefaultsHardening locks in that the published image
// defaults to the spec-strict streamable HTTP transport with
// DNS-rebinding protection enabled. Drift here re-introduces the H1
// finding from the 2026-04-25 audit (Docker image silently shipping
// the deprecated legacy http transport).
func TestDockerfileDefaultsHardening(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", "deploy", "Dockerfile"))

	transportRe := regexp.MustCompile(`(?m)^ENV MCP_TRANSPORT=(\S+)`)
	m := transportRe.FindStringSubmatch(raw)
	if len(m) != 2 {
		t.Fatalf("Dockerfile missing ENV MCP_TRANSPORT line")
	}
	if m[1] == "http" {
		t.Errorf("Dockerfile ENV MCP_TRANSPORT=http (legacy); audit finding H1 says default must be streamable_http or stricter")
	}
	if m[1] != "streamable_http" {
		t.Errorf("Dockerfile ENV MCP_TRANSPORT=%q; expected streamable_http (or a future hardened default)", m[1])
	}

	if !regexp.MustCompile(`(?m)^ENV MCP_STRICT_HOST_CHECK=1`).MatchString(raw) {
		t.Errorf("Dockerfile missing ENV MCP_STRICT_HOST_CHECK=1 (DNS-rebinding mitigation expected by default)")
	}
}

// TestKustomizeBaseDefaultsHardening locks in the same hardening for
// the Kustomize base manifests so a `kubectl apply -k deploy/k8s/base`
// renders the spec-strict transport.
func TestKustomizeBaseDefaultsHardening(t *testing.T) {
	deployment := mustRead(t, filepath.Join("..", "deploy", "k8s", "base", "deployment.yaml"))
	configmap := mustRead(t, filepath.Join("..", "deploy", "k8s", "base", "configmap.yaml"))

	// The base deployment env block must set MCP_TRANSPORT to a non-legacy value.
	transportRe := regexp.MustCompile(`(?ms)- name:\s*MCP_TRANSPORT.*?value:\s*(\S+)`)
	m := transportRe.FindStringSubmatch(deployment)
	if len(m) != 2 {
		t.Fatalf("k8s base deployment.yaml missing MCP_TRANSPORT env entry")
	}
	val := strings.Trim(m[1], `"`)
	if val == "http" {
		t.Errorf("k8s base MCP_TRANSPORT=http; audit finding H1 says default must not be the legacy transport")
	}

	if !strings.Contains(configmap, `CLOCKIFY_POLICY: "safe_core"`) {
		t.Errorf("k8s base configmap missing CLOCKIFY_POLICY: \"safe_core\"; audit finding M3 says network deployments should not default to standard")
	}

	strictHostRe := regexp.MustCompile(`(?ms)- name:\s*MCP_STRICT_HOST_CHECK.*?value:\s*"?1"?`)
	if !strictHostRe.MatchString(deployment) {
		t.Errorf("k8s base deployment.yaml missing MCP_STRICT_HOST_CHECK=1 env entry")
	}
}

// TestHelmDefaultsHardening checks the Helm chart's values.yaml in
// place. Rendering with `helm template` would be more thorough but adds
// a binary dependency; the values file is the contract operators
// override, so a textual check here covers the regression we care about.
func TestHelmDefaultsHardening(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", "deploy", "helm", "clockify-mcp", "values.yaml"))

	if regexp.MustCompile(`(?m)^\s*mode:\s*"http"`).MatchString(raw) {
		t.Errorf("Helm values.yaml has transport.mode: \"http\" (legacy); audit finding H1 says default must be streamable_http")
	}
	if !regexp.MustCompile(`(?m)^\s*mode:\s*"streamable_http"`).MatchString(raw) {
		t.Errorf("Helm values.yaml missing transport.mode: \"streamable_http\"")
	}
	if regexp.MustCompile(`CLOCKIFY_POLICY:\s*"standard"`).MatchString(raw) {
		t.Errorf("Helm values.yaml has CLOCKIFY_POLICY: \"standard\" default; audit finding M3 says use safe_core or stricter")
	}
	if !regexp.MustCompile(`CLOCKIFY_POLICY:\s*"safe_core"`).MatchString(raw) {
		t.Errorf("Helm values.yaml CLOCKIFY_POLICY default should be safe_core (or read_only / time_tracking_safe)")
	}
	if !regexp.MustCompile(`(?m)^\s*strictHostCheck:\s*"1"`).MatchString(raw) {
		t.Errorf("Helm values.yaml strictHostCheck default should be \"1\"")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
