// gen-tool-catalog walks the Tier-1 registry and every Tier-2 group
// builder and emits a machine-readable catalog (JSON) and a
// human-readable rendering (Markdown) for docs/tool-catalog.{json,md}.
//
// Usage:
//
//	go run ./scripts/gen-tool-catalog -out docs
//
// The generator is deterministic: runs from the same code emit
// byte-identical output. CI uses the drift check
//
//	make gen-tool-catalog && git diff --exit-code docs/tool-catalog.*
//
// to refuse PRs that forget to regenerate after adding or changing
// a tool. No network calls, no real Clockify client — the builders
// only need the Service struct populated with nil fields.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/tools"
)

// catalogTool is the JSON shape emitted for each tool. Fields match
// MCP's Tool struct plus the ToolDescriptor hints and the group
// membership so consumers can filter by read/write/destructive or
// tier without parsing Markdown.
//
// RiskClass and AuditKeys surface the structured taxonomy added in
// the 2026-04-27 audit-finding wave so consumers (policy, ops
// dashboards, agent harnesses) can filter on billing / admin /
// permission_change / external_side_effect without grep-ing source.
// The taxonomy mapping mirrors internal/tools/risk_overrides.go.
type catalogTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Group        string         `json:"group"`
	Tier         int            `json:"tier"`
	ReadOnly     bool           `json:"read_only"`
	Destructive  bool           `json:"destructive"`
	Idempotent   bool           `json:"idempotent"`
	RiskClass    []string       `json:"risk_class,omitempty"`
	AuditKeys    []string       `json:"audit_keys,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

// riskClassNames decomposes a mcp.RiskClass bitmask into the stable
// lowercase taxonomy names emitted in the catalog. Order is fixed so
// catalog output is byte-deterministic.
func riskClassNames(rc mcp.RiskClass) []string {
	if rc == 0 {
		return nil
	}
	type entry struct {
		bit  mcp.RiskClass
		name string
	}
	all := []entry{
		{mcp.RiskRead, "read"},
		{mcp.RiskWrite, "write"},
		{mcp.RiskBilling, "billing"},
		{mcp.RiskAdmin, "admin"},
		{mcp.RiskPermissionChange, "permission_change"},
		{mcp.RiskExternalSideEffect, "external_side_effect"},
		{mcp.RiskDestructive, "destructive"},
	}
	out := make([]string, 0, 2)
	for _, e := range all {
		if rc.Has(e.bit) {
			out = append(out, e.name)
		}
	}
	return out
}

type catalog struct {
	Generator string        `json:"generator"`
	Tier1     []catalogTool `json:"tier1"`
	Tier2     []catalogTool `json:"tier2"`
}

func main() {
	outDir := flag.String("out", "docs", "output directory for tool-catalog.{json,md}")
	flag.Parse()

	svc := &tools.Service{}
	t1 := toCatalog(svc.Registry(), "tier1", 1)
	t2 := tier2Catalog(svc)

	cat := catalog{
		Generator: "scripts/gen-tool-catalog — DO NOT EDIT BY HAND; run `make gen-tool-catalog` to refresh",
		Tier1:     t1,
		Tier2:     t2,
	}

	if err := writeJSON(filepath.Join(*outDir, "tool-catalog.json"), cat); err != nil {
		log.Fatalf("write json: %v", err)
	}
	if err := writeMarkdown(filepath.Join(*outDir, "tool-catalog.md"), cat); err != nil {
		log.Fatalf("write md: %v", err)
	}
	fmt.Printf("wrote %d tier-1 + %d tier-2 tools to %s/tool-catalog.{json,md}\n",
		len(t1), len(t2), *outDir)
}

func toCatalog(ds []mcp.ToolDescriptor, group string, tier int) []catalogTool {
	out := make([]catalogTool, 0, len(ds))
	for _, d := range ds {
		out = append(out, catalogTool{
			Name:         d.Tool.Name,
			Description:  d.Tool.Description,
			Group:        group,
			Tier:         tier,
			ReadOnly:     d.ReadOnlyHint,
			Destructive:  d.DestructiveHint,
			Idempotent:   d.IdempotentHint,
			RiskClass:    riskClassNames(d.RiskClass),
			AuditKeys:    d.AuditKeys,
			InputSchema:  d.Tool.InputSchema,
			OutputSchema: d.Tool.OutputSchema,
			Annotations:  d.Tool.Annotations,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func tier2Catalog(svc *tools.Service) []catalogTool {
	groups := make([]string, 0, len(tools.Tier2Groups))
	for name := range tools.Tier2Groups {
		groups = append(groups, name)
	}
	sort.Strings(groups)

	var out []catalogTool
	for _, gname := range groups {
		handlers, ok := svc.Tier2Handlers(gname)
		if !ok {
			continue
		}
		out = append(out, toCatalog(handlers, gname, 2)...)
	}
	return out
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeMarkdown(path string, c catalog) error {
	var b strings.Builder
	b.WriteString("# Tool catalog\n\n")
	b.WriteString("Autogenerated by `scripts/gen-tool-catalog`. Do not edit by hand;\n")
	b.WriteString("re-run `make gen-tool-catalog` after changing any tool descriptor.\n\n")
	fmt.Fprintf(&b, "- Tier 1 tools: **%d** (always registered; visible in `tools/list`).\n", len(c.Tier1))
	fmt.Fprintf(&b, "- Tier 2 tools: **%d** (lazily activated via `clockify_search_tools` with `activate_group` / `activate_tool`).\n\n", len(c.Tier2))

	b.WriteString("## Tier 1\n\n")
	writeTable(&b, c.Tier1, false)

	b.WriteString("\n## Tier 2\n\n")
	// Group tier-2 by group name for readability.
	byGroup := map[string][]catalogTool{}
	var groupOrder []string
	for _, t := range c.Tier2 {
		if _, ok := byGroup[t.Group]; !ok {
			groupOrder = append(groupOrder, t.Group)
		}
		byGroup[t.Group] = append(byGroup[t.Group], t)
	}
	sort.Strings(groupOrder)
	for _, g := range groupOrder {
		fmt.Fprintf(&b, "### `%s`\n\n", g)
		writeTable(&b, byGroup[g], true)
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeTable(b *strings.Builder, rows []catalogTool, hideGroup bool) {
	if hideGroup {
		b.WriteString("| Tool | Read-only | Destructive | Idempotent | Risk | Description |\n")
		b.WriteString("|------|-----------|-------------|------------|------|-------------|\n")
	} else {
		b.WriteString("| Tool | Group | Read-only | Destructive | Idempotent | Risk | Description |\n")
		b.WriteString("|------|-------|-----------|-------------|------------|------|-------------|\n")
	}
	for _, t := range rows {
		desc := strings.ReplaceAll(t.Description, "|", "\\|")
		desc = strings.ReplaceAll(desc, "\n", " ")
		if hideGroup {
			fmt.Fprintf(b, "| `%s` | %s | %s | %s | %s | %s |\n",
				t.Name, yn(t.ReadOnly), yn(t.Destructive), yn(t.Idempotent), riskCell(t.RiskClass), desc)
		} else {
			fmt.Fprintf(b, "| `%s` | `%s` | %s | %s | %s | %s | %s |\n",
				t.Name, t.Group, yn(t.ReadOnly), yn(t.Destructive), yn(t.Idempotent), riskCell(t.RiskClass), desc)
		}
	}
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// riskCell renders a tool's RiskClass taxonomy as a comma-separated
// list of inline-coded names for the markdown table. Empty input
// renders as an em dash so the column never collapses to a blank
// cell that would distort the table.
func riskCell(names []string) string {
	if len(names) == 0 {
		return "—"
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = "`" + n + "`"
	}
	return strings.Join(parts, ", ")
}
