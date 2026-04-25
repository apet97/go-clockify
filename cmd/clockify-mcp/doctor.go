package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/apet97/go-clockify/internal/config"
)

// runDoctor is the `clockify-mcp doctor` subcommand. It audits the
// effective configuration, attributes the source of every spec'd
// env var (explicit | profile | default | empty), and reports any
// error Load() would raise at startup. Exit code is 0 on a clean
// load, 2 on a Load() error, and 3 when --strict finds hosted-service
// posture or backend-check failures.
//
// Invocation:
//
//	clockify-mcp doctor [--profile=<name>] [--strict] [--allow-broad-policy] [--check-backends]
//
// --profile is an alias for MCP_PROFILE=<name>; main() translated it
// into the env before reaching this function. runDoctor also honours
// it directly so tests and embedders get the same behaviour.
func runDoctor(args []string) int {
	return runDoctorReport(args, os.Stdout)
}

func runDoctorReport(args []string, out io.Writer) int {
	opts := parseDoctorArgs(args)
	// Snapshot the env BEFORE Load() runs. applyProfile() inside
	// Load() uses os.Setenv to materialise unset profile keys; if we
	// read the env after Load() we could not distinguish an
	// operator-explicit value from a profile-populated default. The
	// snapshot is limited to keys surfaced in AllSpecs() to keep the
	// map small and the attribution deterministic.
	preLoad := map[string]string{}
	for _, s := range config.AllSpecs() {
		preLoad[s.Name] = strings.TrimSpace(os.Getenv(s.Name))
	}
	// Resolve the active profile once so we can answer "was this key
	// set by profile?" without re-reading MCP_PROFILE later.
	var profile *config.Profile
	if name := preLoad["MCP_PROFILE"]; name != "" {
		if p, err := config.ProfileByName(name); err == nil {
			profile = p
		}
		// An unknown profile name is still reported; Load() will
		// surface the actionable error at the top of the report.
	}

	cfg, cfgErr := config.Load()
	var strictFindings []doctorFinding
	if opts.strict {
		strictFindings = strictDoctorFindings(cfg, cfgErr, opts.allowBroadPolicy)
	}
	if opts.checkBackends && cfgErr == nil {
		strictFindings = append(strictFindings, backendDoctorFindings(cfg)...)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	// pf wraps fmt.Fprintf so the errcheck linter is satisfied.
	// Writes to stdout via tabwriter never produce an actionable
	// error at this call site — if stdout is broken, the process
	// has bigger problems than a doctor report.
	pf := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }
	pln := func(s string) { _, _ = fmt.Fprintln(w, s) }
	defer func() { _ = w.Flush() }()

	pln("clockify-mcp doctor — effective configuration audit")
	pln("")
	if profile != nil {
		pf("Profile:\t%s\t%s\n", profile.Name, profile.Summary)
	} else if preLoad["MCP_PROFILE"] != "" {
		pf("Profile:\t%s\t(unknown — see error below)\n", preLoad["MCP_PROFILE"])
	} else {
		pf("Profile:\t(none)\tExplicit env only; see --help for --profile options\n")
	}
	pln("")

	if cfgErr != nil {
		pf("Load() result:\tERROR\t%s\n", cfgErr.Error())
	} else {
		pf("Load() result:\tOK\ttransport=%s; auth=%s; audit=%s\n",
			cfg.Transport, cfg.AuthMode, cfg.AuditDurabilityMode)
	}
	pln("")

	if opts.strict {
		if len(strictFindings) == 0 {
			message := "no hosted-service findings"
			if opts.checkBackends {
				message = "no hosted-service or backend findings"
			}
			pf("Strict posture:\tOK\t%s\n", message)
		} else {
			pf("Strict posture:\tERROR\t%d finding(s)\n", len(strictFindings))
			pln("Severity\tKey\tMessage")
			for _, f := range strictFindings {
				pf("%s\t%s\t%s\n", f.Severity, f.Key, f.Message)
			}
		}
		pln("")
	}

	// Group specs by their EnvSpec.Group in the same display order
	// gen-config-docs uses, then alphabetise within each group.
	groups := groupSpecs(config.AllSpecs())
	groupOrder := []string{
		"Profile", "Core", "Safety", "Performance", "Bootstrap",
		"Transport", "Auth", "Metrics", "ControlPlane", "Audit",
		"Logging", "Deploy",
	}
	var profileKeys map[string]bool
	if profile != nil {
		profileKeys = make(map[string]bool, len(profile.Env))
		for k := range profile.Env {
			profileKeys[k] = true
		}
	}

	for _, g := range groupOrder {
		specs, ok := groups[g]
		if !ok {
			continue
		}
		sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
		pf("--- %s ---\n", g)
		pln("Variable\tEffective\tSource")
		for _, s := range specs {
			effective := strings.TrimSpace(os.Getenv(s.Name))
			source := attributeSource(s, preLoad[s.Name], profileKeys)
			display := effective
			if display == "" {
				display = "—"
			}
			pf("%s\t%s\t%s\n", s.Name, display, source)
		}
		pln("")
	}

	if cfgErr != nil {
		return 2
	}
	if opts.strict && len(strictFindings) > 0 {
		return 3
	}
	return 0
}

type doctorOptions struct {
	strict           bool
	allowBroadPolicy bool
	checkBackends    bool
}

func parseDoctorArgs(args []string) doctorOptions {
	var opts doctorOptions
	for _, a := range args {
		switch {
		case a == "--strict":
			opts.strict = true
		case a == "--allow-broad-policy":
			opts.allowBroadPolicy = true
		case a == "--check-backends":
			opts.checkBackends = true
			opts.strict = true
		case strings.HasPrefix(a, "--profile="):
			_ = os.Setenv("MCP_PROFILE", strings.TrimPrefix(a, "--profile="))
		}
	}
	return opts
}

type doctorFinding struct {
	Severity string
	Key      string
	Message  string
}

func strictDoctorFindings(cfg config.Config, cfgErr error, allowBroadPolicy bool) []doctorFinding {
	var findings []doctorFinding
	add := func(key, message string) {
		findings = append(findings, doctorFinding{
			Severity: "ERROR",
			Key:      key,
			Message:  message,
		})
	}

	transport := effectiveDoctorTransport(cfg, cfgErr)
	authMode := effectiveDoctorAuthMode(cfg, cfgErr, transport)

	if transport == "http" {
		add("MCP_TRANSPORT", "legacy http transport is forbidden in hosted strict posture; use streamable_http or grpc")
	}
	if authMode == "oidc" {
		if !effectiveDoctorBool(cfg.OIDCStrict, cfgErr, "MCP_OIDC_STRICT") {
			add("MCP_OIDC_STRICT", "OIDC hosted strict posture requires MCP_OIDC_STRICT=1")
		}
		if effectiveDoctorString(cfg.OIDCAudience, cfgErr, "MCP_OIDC_AUDIENCE", "") == "" &&
			effectiveDoctorString(cfg.OIDCResourceURI, cfgErr, "MCP_RESOURCE_URI", "") == "" {
			add("MCP_OIDC_AUDIENCE/MCP_RESOURCE_URI", "OIDC hosted strict posture requires MCP_OIDC_AUDIENCE or MCP_RESOURCE_URI")
		}
		if !effectiveDoctorBool(cfg.RequireTenantClaim, cfgErr, "MCP_REQUIRE_TENANT_CLAIM") {
			add("MCP_REQUIRE_TENANT_CLAIM", "OIDC hosted strict posture requires MCP_REQUIRE_TENANT_CLAIM=1")
		}
	}
	if !effectiveDoctorBool(cfg.DisableInlineSecrets, cfgErr, "MCP_DISABLE_INLINE_SECRETS") {
		add("MCP_DISABLE_INLINE_SECRETS", "hosted strict posture requires MCP_DISABLE_INLINE_SECRETS=1")
	}
	if effectiveDoctorBool(cfg.ExposeAuthErrors, cfgErr, "MCP_EXPOSE_AUTH_ERRORS") {
		add("MCP_EXPOSE_AUTH_ERRORS", "hosted strict posture requires MCP_EXPOSE_AUTH_ERRORS=0 or unset")
	}

	controlPlaneDSN := effectiveDoctorString(cfg.ControlPlaneDSN, cfgErr, "MCP_CONTROL_PLANE_DSN", "memory")
	if !strings.HasPrefix(controlPlaneDSN, "postgres://") && !strings.HasPrefix(controlPlaneDSN, "postgresql://") {
		add("MCP_CONTROL_PLANE_DSN", "hosted strict posture requires a postgres:// or postgresql:// control-plane DSN")
	}

	auditDurability := effectiveDoctorAuditDurability(cfg, cfgErr)
	if auditDurability != "fail_closed" {
		add("MCP_AUDIT_DURABILITY", "hosted strict posture requires MCP_AUDIT_DURABILITY=fail_closed")
	}

	policyMode := effectiveDoctorPolicyMode()
	switch policyMode {
	case "read_only", "time_tracking_safe":
		// allowed
	case "safe_core", "standard", "full":
		if !allowBroadPolicy {
			add("CLOCKIFY_POLICY", "hosted strict posture requires CLOCKIFY_POLICY no broader than time_tracking_safe; pass --allow-broad-policy only for a documented trusted-operator exception")
		}
	default:
		add("CLOCKIFY_POLICY", "unsupported policy mode; expected read_only, time_tracking_safe, safe_core, standard, or full")
	}

	mtlsTenantSource := effectiveDoctorString(cfg.MTLSTenantSource, cfgErr, "MCP_MTLS_TENANT_SOURCE", "cert")
	if mtlsTenantSource == "header" || mtlsTenantSource == "header_or_cert" {
		add("MCP_MTLS_TENANT_SOURCE", "hosted strict posture requires MCP_MTLS_TENANT_SOURCE=cert")
	}

	// mTLS-specific gate: when the deployment is actually selecting
	// mtls auth, MCP_REQUIRE_MTLS_TENANT must be on. Without it, a
	// client whose cert exposes no tenant identity silently collapses
	// onto MCP_DEFAULT_TENANT_ID — exactly the multi-tenant footgun
	// MCP_REQUIRE_TENANT_CLAIM=1 closes for OIDC. The "tenant source =
	// cert" half of the requirement is already enforced by the
	// universal MCP_MTLS_TENANT_SOURCE check above, so we don't repeat
	// it here. Non-mTLS auth modes are not subject to this check.
	if authMode == "mtls" && !effectiveDoctorBool(cfg.RequireMTLSTenant, cfgErr, "MCP_REQUIRE_MTLS_TENANT") {
		add("MCP_REQUIRE_MTLS_TENANT", "hosted strict mTLS posture requires MCP_REQUIRE_MTLS_TENANT=1")
	}

	return findings
}

func isDoctorPostgresDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}

func effectiveDoctorTransport(cfg config.Config, cfgErr error) string {
	if cfgErr == nil && cfg.Transport != "" {
		return cfg.Transport
	}
	return effectiveDoctorString("", cfgErr, "MCP_TRANSPORT", "stdio")
}

func effectiveDoctorAuthMode(cfg config.Config, cfgErr error, transport string) string {
	if cfgErr == nil && cfg.AuthMode != "" {
		return cfg.AuthMode
	}
	if raw := strings.TrimSpace(os.Getenv("MCP_AUTH_MODE")); raw != "" {
		return raw
	}
	switch transport {
	case "streamable_http":
		return "oidc"
	case "http":
		return "static_bearer"
	default:
		return ""
	}
}

func effectiveDoctorAuditDurability(cfg config.Config, cfgErr error) string {
	if cfgErr == nil && cfg.AuditDurabilityMode != "" {
		return cfg.AuditDurabilityMode
	}
	if raw := strings.TrimSpace(os.Getenv("MCP_AUDIT_DURABILITY")); raw != "" {
		return raw
	}
	if strings.TrimSpace(os.Getenv("ENVIRONMENT")) == "prod" {
		return "fail_closed"
	}
	return "best_effort"
}

func effectiveDoctorPolicyMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_POLICY")))
	if mode == "" {
		return "standard"
	}
	return mode
}

func effectiveDoctorString(loaded string, cfgErr error, key, fallback string) string {
	if cfgErr == nil && loaded != "" {
		return loaded
	}
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		return raw
	}
	return fallback
}

func effectiveDoctorBool(loaded bool, cfgErr error, key string) bool {
	if cfgErr == nil {
		return loaded
	}
	return strings.TrimSpace(os.Getenv(key)) == "1"
}

// groupSpecs bins specs by their Group field. An empty group becomes
// "Misc" to mirror the gen-config-docs convention. Returns a copy so
// the caller can mutate the slices without poisoning the registry.
func groupSpecs(specs []config.EnvSpec) map[string][]config.EnvSpec {
	out := map[string][]config.EnvSpec{}
	for _, s := range specs {
		g := s.Group
		if g == "" {
			g = "Misc"
		}
		out[g] = append(out[g], s)
	}
	return out
}

// attributeSource reports how the given spec's effective value came
// to be: "explicit" means the operator set it before Load() ran,
// "profile" means the active profile's applyProfile() filled it in,
// "default" means the spec declared a default string, and "empty"
// means the variable is not set and has no documented default.
func attributeSource(s config.EnvSpec, preLoadValue string, profileKeys map[string]bool) string {
	if preLoadValue != "" {
		return "explicit"
	}
	if profileKeys[s.Name] {
		return "profile"
	}
	if s.Default != "" {
		return "default"
	}
	return "empty"
}
