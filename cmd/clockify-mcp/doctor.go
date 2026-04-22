package main

import (
	"fmt"
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
// load and 2 on a Load() error — the same rule used by `doctor`
// tooling elsewhere in the ecosystem.
//
// Invocation:
//
//	clockify-mcp doctor [--profile=<name>]
//
// --profile is an alias for MCP_PROFILE=<name>; main() translated it
// into the env before reaching this function, so doctor does not
// need to parse it itself. Any other args are ignored for now (keep
// surface minimal until a real use case shows up).
func runDoctor(_ []string) int {
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "clockify-mcp doctor — effective configuration audit")
	fmt.Fprintln(w, "")
	if profile != nil {
		fmt.Fprintf(w, "Profile:\t%s\t%s\n", profile.Name, profile.Summary)
	} else if preLoad["MCP_PROFILE"] != "" {
		fmt.Fprintf(w, "Profile:\t%s\t(unknown — see error below)\n", preLoad["MCP_PROFILE"])
	} else {
		fmt.Fprintf(w, "Profile:\t(none)\tExplicit env only; see --help for --profile options\n")
	}
	fmt.Fprintln(w, "")

	if cfgErr != nil {
		fmt.Fprintf(w, "Load() result:\tERROR\t%s\n", cfgErr.Error())
	} else {
		fmt.Fprintf(w, "Load() result:\tOK\ttransport=%s; auth=%s; audit=%s\n",
			cfg.Transport, cfg.AuthMode, cfg.AuditDurabilityMode)
	}
	fmt.Fprintln(w, "")

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
		fmt.Fprintf(w, "--- %s ---\n", g)
		fmt.Fprintln(w, "Variable\tEffective\tSource")
		for _, s := range specs {
			effective := strings.TrimSpace(os.Getenv(s.Name))
			source := attributeSource(s, preLoad[s.Name], profileKeys)
			display := effective
			if display == "" {
				display = "—"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, display, source)
		}
		fmt.Fprintln(w, "")
	}

	if cfgErr != nil {
		return 2
	}
	return 0
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
