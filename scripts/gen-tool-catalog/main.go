// gen-tool-catalog walks the one-user full-access registry and emits
// a machine-readable catalog (JSON) and a human-readable rendering
// (Markdown) for docs/tool-catalog.{json,md}.
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
// a tool. No network calls, no real Clockify client — the descriptor
// builders only need the Service struct populated with nil fields.
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
// MCP's Tool struct plus the ToolDescriptor hints so consumers can
// filter by read/write/destructive without parsing Markdown.
//
// RiskClass and AuditKeys surface the structured taxonomy so
// consumers can filter on billing / admin / permission_change /
// external_side_effect without grep-ing source.
type catalogTool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	Category     string         `json:"category,omitempty"`
	HandlerKind  string         `json:"handler_kind,omitempty"`
	Method       string         `json:"method,omitempty"`
	Path         string         `json:"path,omitempty"`
	ReadOnly     bool           `json:"read_only"`
	Destructive  bool           `json:"destructive"`
	Idempotent   bool           `json:"idempotent"`
	DryRun       bool           `json:"dry_run"`
	RiskClass    []string       `json:"risk_class,omitempty"`
	AuditKeys    []string       `json:"audit_keys,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

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
	Toolset   string        `json:"toolset,omitempty"`
	Tools     []catalogTool `json:"tools"`
}

func main() {
	outDir := flag.String("out", "docs", "output directory for tool-catalog.{json,md}")
	flag.Parse()

	svc := &tools.Service{}
	registry, err := svc.FullAccessRegistryChecked()
	if err != nil {
		log.Fatalf("build registry: %v", err)
	}

	cat := catalog{
		Generator: "scripts/gen-tool-catalog — DO NOT EDIT BY HAND; run `make gen-tool-catalog` to refresh",
		Toolset:   "all",
		Tools:     toCatalog(registry),
	}
	defaultCat := catalog{
		Generator: cat.Generator,
		Toolset:   "default",
		Tools:     toCatalog(svc.RegistryForToolset("default")),
	}

	if err := writeJSON(filepath.Join(*outDir, "tool-catalog.json"), cat); err != nil {
		log.Fatalf("write json: %v", err)
	}
	if err := writeMarkdown(filepath.Join(*outDir, "tool-catalog.md"), cat); err != nil {
		log.Fatalf("write md: %v", err)
	}
	if err := writeJSON(filepath.Join(*outDir, "default-toolset.json"), defaultCat); err != nil {
		log.Fatalf("write default json: %v", err)
	}
	if err := writeMarkdown(filepath.Join(*outDir, "default-toolset.md"), defaultCat); err != nil {
		log.Fatalf("write default md: %v", err)
	}
	fmt.Printf("wrote %d tools to %s/tool-catalog.{json,md}\n", len(cat.Tools), *outDir)
	fmt.Printf("wrote %d tools to %s/default-toolset.{json,md}\n", len(defaultCat.Tools), *outDir)
}

func toCatalog(ds []mcp.ToolDescriptor) []catalogTool {
	out := make([]catalogTool, 0, len(ds))
	for _, d := range ds {
		category, _ := d.Tool.Annotations["category"].(string)
		handlerKind, _ := d.Tool.Annotations["handlerKind"].(string)
		method, _ := d.Tool.Annotations["method"].(string)
		path, _ := d.Tool.Annotations["path"].(string)
		out = append(out, catalogTool{
			Name:         d.Tool.Name,
			Title:        d.Tool.Title,
			Description:  d.Tool.Description,
			Category:     category,
			HandlerKind:  handlerKind,
			Method:       method,
			Path:         path,
			ReadOnly:     d.ReadOnlyHint,
			Destructive:  d.DestructiveHint,
			Idempotent:   d.IdempotentHint,
			DryRun:       annotationBool(d.Tool.Annotations, "dryRun"),
			RiskClass:    riskClassNames(d.RiskClass),
			AuditKeys:    d.AuditKeys,
			InputSchema:  d.Tool.InputSchema,
			OutputSchema: d.Tool.OutputSchema,
			Annotations:  d.Tool.Annotations,
		})
	}
	return out
}

func annotationBool(annotations map[string]any, key string) bool {
	v, _ := annotations[key].(bool)
	return v
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
	if c.Toolset == "default" {
		b.WriteString("# Default toolset catalog\n\n")
	} else {
		b.WriteString("# Tool catalog\n\n")
	}
	b.WriteString("Autogenerated by `scripts/gen-tool-catalog`. Do not edit by hand;\n")
	b.WriteString("re-run `make gen-tool-catalog` after changing any tool descriptor.\n\n")
	if c.Toolset == "default" {
		fmt.Fprintf(&b, "- Tools: **%d** (the everyday surface advertised by `CLOCKIFY_TOOLSET=default`).\n\n", len(c.Tools))
		b.WriteString("- Start here for normal agent sessions; use the full catalog only when a workflow or domain result points beyond this surface.\n\n")
	} else {
		fmt.Fprintf(&b, "- Tools: **%d** (all registered at startup; workflow tools first, domain tools second, raw API fallback last).\n\n", len(c.Tools))
		fmt.Fprintf(&b, "- `tools/list` returns the advertised toolset, not the loaded registry. The default toolset advertises 16 everyday tools; `CLOCKIFY_TOOLSET=all` advertises all %d startup tools. For default/core/business/admin, unadvertised tools are not dispatch-callable. Large advertised surfaces are cursor-paginated.\n\n", len(c.Tools))
	}
	writeCookbookLinks(&b)
	writeTimeEntryGuidance(&b)

	b.WriteString("## Tools\n\n")
	writeTable(&b, c.Tools)
	writeAuditKeysSection(&b, c)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeCookbookLinks(b *strings.Builder) {
	b.WriteString("## Agent cookbook recipes\n\n")
	b.WriteString("Intent-keyed examples live in [agent-cookbook.md](agent-cookbook.md):\n\n")
	b.WriteString("| Intent | Primary tools |\n")
	b.WriteString("|--------|---------------|\n")
	b.WriteString("| [First call orientation](agent-cookbook.md#first-call-orientation) | `clockify_status`, `clockify_tools_guide` |\n")
	b.WriteString("| [Set up client project task tag](agent-cookbook.md#set-up-client-project-task-tag) | `clockify_create_work_package` |\n")
	b.WriteString("| [Log finished work](agent-cookbook.md#log-finished-work) | `clockify_log_work` |\n")
	b.WriteString("| [Start stop switch timer](agent-cookbook.md#start-stop-switch-timer) | `clockify_start_work`, `clockify_stop_work`, `clockify_switch_work` |\n")
	b.WriteString("| [Review day week](agent-cookbook.md#review-day-week) | `clockify_review_day`, `clockify_review_week` |\n")
	b.WriteString("| [Fix entry](agent-cookbook.md#fix-entry) | `clockify_fix_entry` |\n")
	b.WriteString("| [Invoice client](agent-cookbook.md#invoice-client) | `clockify_invoice_client_work` |\n")
	b.WriteString("| [Record expense](agent-cookbook.md#record-expense) | `clockify_record_expense` |\n")
	b.WriteString("| [Time off](agent-cookbook.md#time-off) | `clockify_request_time_off` |\n")
	b.WriteString("| [Schedule work](agent-cookbook.md#schedule-work) | `clockify_schedule_work` |\n")
	b.WriteString("| [Webhook](agent-cookbook.md#webhook) | `clockify_setup_webhook` |\n")
	b.WriteString("| [Demo smoke](agent-cookbook.md#demo-smoke) | `clockify_demo_seed`, `clockify_demo_cleanup` |\n\n")
}

func writeTimeEntryGuidance(b *strings.Builder) {
	b.WriteString("## Workflow tool choice\n\n")
	b.WriteString("Use `clockify_status` as the first call, then `clockify_tools_guide` when\n")
	b.WriteString("choosing between workflow, domain, and raw fallback tools. For finished\n")
	b.WriteString("past work, prefer `clockify_log_work`; it requires both `start` and `end`\n")
	b.WriteString("and returns the created `entryId` plus review/fix next actions. For live\n")
	b.WriteString("timers, use `clockify_start_work`, `clockify_stop_work`, and\n")
	b.WriteString("`clockify_switch_work`. Use `clockify_review_day` or\n")
	b.WriteString("`clockify_review_week` before catch-up or cleanup work, then\n")
	b.WriteString("`clockify_fix_entry` for one exact entry.\n\n")

	b.WriteString("## Time formats\n\n")
	b.WriteString("Time-entry and report range fields accept RFC3339 values and\n")
	b.WriteString("common flexible forms parsed in the requested `timezone`, `CLOCKIFY_TIMEZONE`,\n")
	b.WriteString("or local/server timezone when no timezone is supplied:\n")
	b.WriteString("`YYYY-MM-DD`, `YYYY-MM-DD HH:MM`, `today HH:MM`, `yesterday HH:MM`,\n")
	b.WriteString("and `now`. Fields whose schema still says `RFC3339` only are stricter;\n")
	b.WriteString("prefer the documented format on each tool descriptor.\n\n")
}

func writeAuditKeysSection(b *strings.Builder, c catalog) {
	var rows []catalogTool
	for _, t := range c.Tools {
		if len(t.AuditKeys) > 0 {
			rows = append(rows, t)
		}
	}
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	b.WriteString("\n## Audit-tracked argument capture\n\n")
	b.WriteString("Tools below record action-defining arguments alongside the\n")
	b.WriteString("default `*_id` capture in audit events. The per-tool `audit_keys`\n")
	b.WriteString("list also surfaces in `docs/tool-catalog.json`.\n\n")
	b.WriteString("| Tool | Audit Keys |\n")
	b.WriteString("|------|------------|\n")
	for _, r := range rows {
		parts := make([]string, len(r.AuditKeys))
		for i, k := range r.AuditKeys {
			parts[i] = "`" + k + "`"
		}
		fmt.Fprintf(b, "| `%s` | %s |\n", r.Name, strings.Join(parts, ", "))
	}
}

func writeTable(b *strings.Builder, rows []catalogTool) {
	b.WriteString("| Tool | Category | Read-only | Destructive | Idempotent | Dry-run | Risk | Description |\n")
	b.WriteString("|------|----------|-----------|-------------|------------|---------|------|-------------|\n")
	for _, t := range rows {
		desc := strings.ReplaceAll(t.Description, "|", "\\|")
		desc = strings.ReplaceAll(desc, "\n", " ")
		fmt.Fprintf(b, "| `%s` | %s | %s | %s | %s | %s | %s | %s |\n",
			t.Name, categoryCell(t.Category), yn(t.ReadOnly), yn(t.Destructive), yn(t.Idempotent), yn(t.DryRun), riskCell(t.RiskClass), desc)
	}
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func categoryCell(c string) string {
	if c == "" {
		return "—"
	}
	return "`" + c + "`"
}

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
