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

	// Marketing claims do not belong in registry metadata. The audit-
	// driven 2026-04-25 wave removed "Production-grade" / "best-in-class"
	// language from prose docs (commits e12189a, d3db3ce); the OCI label
	// description is the last surface that still parroted the puffery
	// when this guard was added.
	puffery := regexp.MustCompile(`(?i)production-grade|enterprise-grade|best-in-class`)
	for line := range strings.SplitSeq(raw, "\n") {
		if !strings.Contains(line, "org.opencontainers.image.description") {
			continue
		}
		if puffery.MatchString(line) {
			t.Errorf("Dockerfile OCI description contains marketing puffery: %q", strings.TrimSpace(line))
		}
	}
}

// TestComposeDefaultsHardening locks in that deploy/docker-compose.yml
// applies the single-tenant streamable HTTP profile, binds the host
// port to loopback only, persists state on a named volume, and does
// not regress to the legacy http transport or the broad standard
// policy. Audit findings H1 (legacy default) and M3 (broad policy)
// were already fixed in deploy/k8s/* and deploy/helm/* — this test
// stops Compose from quietly drifting back.
func TestComposeDefaultsHardening(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", "deploy", "docker-compose.yml"))

	if regexp.MustCompile(`(?m)^\s*-\s*MCP_TRANSPORT\s*=\s*http\s*$`).MatchString(raw) {
		t.Errorf("docker-compose.yml hardcodes MCP_TRANSPORT=http (legacy); single-tenant profile selects streamable_http")
	}
	if !regexp.MustCompile(`(?m)^\s*-\s*MCP_PROFILE\s*=\s*single-tenant-http\s*$`).MatchString(raw) {
		t.Errorf("docker-compose.yml missing MCP_PROFILE=single-tenant-http; profile is the canonical way to wire control-plane + auth defaults")
	}
	// The default branch of CLOCKIFY_POLICY must not fall back to
	// "standard". time_tracking_safe is the recommended AI-facing
	// default; broader modes require an explicit operator override.
	policyRe := regexp.MustCompile(`(?m)^\s*-\s*CLOCKIFY_POLICY\s*=\$\{CLOCKIFY_POLICY:-([^}]+)\}\s*$`)
	pm := policyRe.FindStringSubmatch(raw)
	if len(pm) != 2 {
		t.Fatalf("docker-compose.yml missing CLOCKIFY_POLICY env with explicit default")
	}
	switch pm[1] {
	case "time_tracking_safe":
		// allowed
	case "safe_core", "read_only":
		t.Errorf("docker-compose.yml defaults CLOCKIFY_POLICY=%q; expected time_tracking_safe for bundled AI-facing defaults", pm[1])
	case "standard", "full":
		t.Errorf("docker-compose.yml defaults CLOCKIFY_POLICY=%q; AI-facing deployments should default to time_tracking_safe", pm[1])
	default:
		t.Errorf("docker-compose.yml CLOCKIFY_POLICY default %q is not a recognised policy mode", pm[1])
	}

	originsRe := regexp.MustCompile(`(?m)^\s*-\s*MCP_ALLOWED_ORIGINS\s*=\$\{MCP_ALLOWED_ORIGINS:-([^}]+)\}\s*$`)
	om := originsRe.FindStringSubmatch(raw)
	if len(om) != 2 {
		t.Fatalf("docker-compose.yml missing MCP_ALLOWED_ORIGINS env with explicit non-empty Caddy host default")
	}
	if strings.TrimSpace(om[1]) == "" {
		t.Errorf("docker-compose.yml defaults MCP_ALLOWED_ORIGINS to empty; strict host check behind Caddy needs the public host")
	}
	if om[1] != "https://your-domain.example.com" {
		t.Errorf("docker-compose.yml MCP_ALLOWED_ORIGINS default = %q, want bundled Caddy public host", om[1])
	}

	// Loopback-only host bind ensures the service is not directly
	// reachable from the public network without going through Caddy.
	if !regexp.MustCompile(`(?m)^\s*-\s*"127\.0\.0\.1:8080:8080"`).MatchString(raw) {
		t.Errorf("docker-compose.yml clockify-mcp port should be bound to 127.0.0.1:8080:8080 to keep the public surface behind Caddy")
	}

	if !strings.Contains(raw, "clockify_mcp_data:/var/lib/clockify-mcp") {
		t.Errorf("docker-compose.yml missing persistent volume mount for /var/lib/clockify-mcp; single-tenant profile expects file-backed control plane")
	}
	if !regexp.MustCompile(`(?ms)^volumes:\s*$.*?clockify_mcp_data:`).MatchString(raw) {
		t.Errorf("docker-compose.yml missing top-level clockify_mcp_data volume declaration")
	}
}

// TestCaddyfilePreservesProxyHeaders pins that the bundled Caddyfile
// preserves the externally observed Host and forwarded scheme so the
// MCP server's strict-host check sees the public domain rather than
// the container's internal address. Without these headers, enabling
// MCP_STRICT_HOST_CHECK behind Caddy would 403 every legitimate
// request.
func TestCaddyfilePreservesProxyHeaders(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", "deploy", "Caddyfile"))
	if !regexp.MustCompile(`(?m)^\s*header_up\s+Host\s+\{host\}\s*$`).MatchString(raw) {
		t.Errorf("Caddyfile reverse_proxy block missing `header_up Host {host}` (required so MCP_STRICT_HOST_CHECK sees the public domain)")
	}
	if !regexp.MustCompile(`(?m)^\s*header_up\s+X-Forwarded-Proto\s+\{scheme\}\s*$`).MatchString(raw) {
		t.Errorf("Caddyfile reverse_proxy block missing `header_up X-Forwarded-Proto {scheme}` (required for any future absolute-URL responses)")
	}
}

// TestDockerImageWorkflowSmokeEnvHardening locks the env block that
// the PR Docker smoke uses against the now-default streamable HTTP
// transport. Without MCP_AUTH_MODE=static_bearer + a memory backend
// + MCP_ALLOW_DEV_BACKEND=1, the image can't start because
// streamable_http defaults to OIDC and refuses memory-backed control
// planes. Without a dedicated metrics bind, /metrics is absent on
// streamable_http (inline metrics only mounts on legacy http).
func TestDockerImageWorkflowSmokeEnvHardening(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", ".github", "workflows", "docker-image.yml"))

	required := []string{
		"-e MCP_AUTH_MODE=static_bearer",
		"-e MCP_CONTROL_PLANE_DSN=memory",
		"-e MCP_ALLOW_DEV_BACKEND=1",
		"-e MCP_METRICS_BIND=:8082",
		"-e MCP_METRICS_BEARER_TOKEN=",
	}
	for _, frag := range required {
		if !strings.Contains(raw, frag) {
			t.Errorf("docker-image.yml smoke step missing %q (streamable_http default needs static-bearer + memory + dev-backend + dedicated metrics listener)", frag)
		}
	}
	// The legacy MCP_HTTP_INLINE_METRICS_ENABLED flag is a no-op on
	// streamable_http; carrying it forward would mislead future readers.
	if strings.Contains(raw, "-e MCP_HTTP_INLINE_METRICS_ENABLED=true") {
		t.Errorf("docker-image.yml smoke still sets MCP_HTTP_INLINE_METRICS_ENABLED=true; inline metrics is legacy-http only — use MCP_METRICS_BIND instead")
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

	if !strings.Contains(configmap, `CLOCKIFY_POLICY: "time_tracking_safe"`) {
		t.Errorf("k8s base configmap missing CLOCKIFY_POLICY: \"time_tracking_safe\"; AI-facing defaults should not allow workspace object creation")
	}

	strictHostRe := regexp.MustCompile(`(?ms)- name:\s*MCP_STRICT_HOST_CHECK.*?value:\s*"?1"?`)
	if !strictHostRe.MatchString(deployment) {
		t.Errorf("k8s base deployment.yaml missing MCP_STRICT_HOST_CHECK=1 env entry")
	}
}

// TestKustomizeProdOverlayDoesNotWidenPolicy ensures the production
// overlay does not undo the AI-facing base policy by replacing it with
// broad standard/full access.
func TestKustomizeProdOverlayDoesNotWidenPolicy(t *testing.T) {
	raw := mustRead(t, filepath.Join("..", "deploy", "k8s", "overlays", "prod", "kustomization.yaml"))
	if regexp.MustCompile(`(?m)value:\s*"standard"`).MatchString(raw) || regexp.MustCompile(`(?m)value:\s*"full"`).MatchString(raw) {
		t.Errorf("prod overlay widens CLOCKIFY_POLICY to standard/full; default prod overlay should stay time_tracking_safe")
	}
	if !regexp.MustCompile(`(?m)value:\s*"time_tracking_safe"`).MatchString(raw) {
		t.Errorf("prod overlay should pin CLOCKIFY_POLICY to time_tracking_safe")
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
		t.Errorf("Helm values.yaml has CLOCKIFY_POLICY: \"standard\" default; AI-facing defaults should use time_tracking_safe")
	}
	if !regexp.MustCompile(`CLOCKIFY_POLICY:\s*"time_tracking_safe"`).MatchString(raw) {
		t.Errorf("Helm values.yaml CLOCKIFY_POLICY default should be time_tracking_safe")
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
