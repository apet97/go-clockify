package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
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

// TestHelmServiceMonitorPortMatchesService renders the Helm chart with
// the ServiceMonitor enabled in two configurations — with and without
// a dedicated metrics listener — and asserts the ServiceMonitor scrapes
// a port name that the rendered Service actually exposes. Catches the
// "ServiceMonitor scrapes wrong port" drift the 2026-04-25 audit found:
// when an operator sets metricsEndpoint.bind=":9091", the chart must
// expose a `metrics` Service port AND the ServiceMonitor must reference
// that port name, not `http`.
//
// Skips when `helm` is not on PATH (laptop without the deploy toolchain
// installed); CI's deploy-render job covers it.
func TestHelmServiceMonitorPortMatchesService(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not on PATH; skipping render-based ServiceMonitor wiring check")
	}
	chartDir, err := filepath.Abs(filepath.Join("..", "deploy", "helm", "clockify-mcp"))
	if err != nil {
		t.Fatalf("abs chart dir: %v", err)
	}

	t.Run("inline_metrics_default_uses_http_port", func(t *testing.T) {
		out := helmTemplate(t, chartDir,
			"--set", "metrics.serviceMonitor.enabled=true",
		)
		// SM should scrape "http" when no dedicated listener is configured.
		if !regexp.MustCompile(`(?m)^\s+- port: http\s*$`).MatchString(out) {
			t.Errorf("expected ServiceMonitor port=http when metricsEndpoint.bind is empty; got render:\n%s", excerptServiceMonitor(out))
		}
		// And no `metrics` Service port should be rendered.
		if regexp.MustCompile(`(?m)^\s+- name: metrics\s*$`).MatchString(out) {
			t.Errorf("expected no `metrics` Service port when metricsEndpoint.bind is empty; got render:\n%s", excerptServiceAndDeployment(out))
		}
	})

	t.Run("dedicated_metrics_listener_uses_metrics_port_and_auth", func(t *testing.T) {
		out := helmTemplate(t, chartDir,
			"--set", "metrics.serviceMonitor.enabled=true",
			"--set", "metricsEndpoint.bind=:9091",
			"--set", "metricsEndpoint.authMode=static_bearer",
			"--set", "metrics.serviceMonitor.bearerTokenSecret.name=clockify-mcp-secrets",
		)

		// Service must expose a `metrics` port (targetPort: metrics).
		if !regexp.MustCompile(`(?ms)- name: metrics\s+port: 9091\s+targetPort: metrics`).MatchString(out) {
			t.Errorf("expected Service to expose `metrics` port (9091, targetPort=metrics) when metricsEndpoint.bind set; got render:\n%s", excerptServiceAndDeployment(out))
		}
		// Deployment must expose a `metrics` containerPort.
		if !regexp.MustCompile(`(?ms)- name: metrics\s+containerPort: 9091`).MatchString(out) {
			t.Errorf("expected Deployment containerPort `metrics` when metricsEndpoint.bind set; got render:\n%s", excerptServiceAndDeployment(out))
		}
		// ServiceMonitor must scrape the metrics port, not http.
		if !regexp.MustCompile(`(?m)^\s+- port: metrics\s*$`).MatchString(out) {
			t.Errorf("expected ServiceMonitor port=metrics when dedicated listener configured; got render:\n%s", excerptServiceMonitor(out))
		}
		if regexp.MustCompile(`(?ms)kind: ServiceMonitor.*?- port: http`).MatchString(out) {
			t.Errorf("ServiceMonitor still scrapes `port: http` despite dedicated listener — that's the regression the audit caught")
		}
		// And it must carry an Authorization header.
		if !regexp.MustCompile(`(?ms)authorization:\s+type: Bearer\s+credentials:\s+name: clockify-mcp-secrets`).MatchString(out) {
			t.Errorf("expected ServiceMonitor to carry Bearer auth referencing the configured Secret; got render:\n%s", excerptServiceMonitor(out))
		}
	})
}

func helmTemplate(t *testing.T, chartDir string, args ...string) string {
	t.Helper()
	full := append([]string{"template", "clockify-mcp", chartDir}, args...)
	cmd := exec.Command("helm", full...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template failed: %v\n--- stderr ---\n%s", err, errb.String())
	}
	return out.String()
}

func excerptServiceMonitor(rendered string) string {
	idx := strings.Index(rendered, "kind: ServiceMonitor")
	if idx < 0 {
		return "(no ServiceMonitor in render)"
	}
	end := idx + 800
	if end > len(rendered) {
		end = len(rendered)
	}
	return rendered[idx:end]
}

func excerptServiceAndDeployment(rendered string) string {
	idx := strings.Index(rendered, "kind: Service\n")
	if idx < 0 {
		idx = strings.Index(rendered, "kind: Service ") // template variation
	}
	if idx < 0 {
		return "(no Service in render)"
	}
	end := idx + 1500
	if end > len(rendered) {
		end = len(rendered)
	}
	return rendered[idx:end]
}
