package dryrun

import (
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// Backward-compatible helpers
// ---------------------------------------------------------------------------

func Enabled(args map[string]any) bool {
	if args == nil {
		return false
	}
	v, ok := args["dry_run"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func Preview(tool string, args map[string]any) map[string]any {
	return map[string]any{
		"dry_run":    true,
		"tool":       tool,
		"args":       args,
		"note":       "No changes were made.",
		"validation": unknownValidation("generic dry-run preview did not run tool-specific validation"),
	}
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	Enabled bool
}

// ConfigFromEnv reads CLOCKIFY_DRY_RUN. Enabled by default.
// Set to "off", "disabled", "0", or "false" to disable.
func ConfigFromEnv() Config {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CLOCKIFY_DRY_RUN")))
	switch v {
	case "off", "disabled", "0", "false":
		return Config{Enabled: false}
	default:
		return Config{Enabled: true}
	}
}

// ---------------------------------------------------------------------------
// Action enum
// ---------------------------------------------------------------------------

type Action int

const (
	ConfirmPattern  Action = iota // remove confirm flag and return a minimal preview envelope without executing the handler
	PreviewTool                   // call GET counterpart instead of DELETE
	MinimalFallback               // echo parameters, no API call
	NotDestructive                // error: dry_run on non-destructive tool
)

// ---------------------------------------------------------------------------
// Static maps
// ---------------------------------------------------------------------------

// previewMap maps destructive delete tools to their GET counterparts.
var previewMap = map[string]string{
	"clockify_clients_delete":                "clockify_clients_get",
	"clockify_projects_delete":               "clockify_projects_get",
	"clockify_tasks_delete":                  "clockify_tasks_get",
	"clockify_tags_delete":                   "clockify_tags_get",
	"clockify_entries_delete":                "clockify_entries_get",
	"clockify_invoices_delete":               "clockify_invoices_get",
	"clockify_expenses_delete":               "clockify_expenses_get",
	"clockify_custom_fields_delete":          "clockify_custom_fields_get",
	"clockify_time_off_requests_delete":      "clockify_time_off_requests_get",
	"clockify_scheduling_assignments_delete": "clockify_scheduling_assignments_get",
	"clockify_webhooks_delete":               "clockify_webhooks_get",
	"clockify_groups_delete":                 "clockify_groups_get",
	"clockify_holidays_delete":               "clockify_holidays_get",
}

// confirmTools maps destructive tools that use confirm-pattern interception.
// These tools must be registered with toolDestructive() to reach this path.
// Note: tools registered with toolRW() handle dry-run at the handler level
// and never reach this map (CheckDryRun passes through for non-destructive tools).
//
// Audit finding 6 follow-up: today ConfirmPattern returns the same minimal
// envelope as MinimalFallback (executeDryRun in internal/enforcement),
// so moving a tool between the two maps changes nothing observable. A
// real confirmation-token requirement on non-dry-run execution
// (e.g. confirm:"delete_invoice_item:inv1:item7") would block the
// most dangerous "agent fires off a destructive call without dry-run
// first" scenarios; that is tracked as a separate follow-up rather
// than half-wired here. The current safety story is: dry-run is on
// by default (CLOCKIFY_DRY_RUN=enabled), the policy gate denies most
// destructive tools by default, and operators must explicitly switch
// CLOCKIFY_POLICY=full to expose them.
var confirmTools = map[string]bool{}

// minimalTools use minimal fallback (no GET counterpart). These deletes
// have no Clockify GET endpoint that returns the doomed resource shape,
// so dry-run echoes the supplied IDs without any preview content.
var minimalTools = map[string]bool{
	"clockify_invoices_items_delete":      true,
	"clockify_invoices_payments_delete":   true,
	"clockify_expenses_categories_delete": true,
}

// ---------------------------------------------------------------------------
// Interception logic
// ---------------------------------------------------------------------------

// CheckDryRun inspects args for a dry_run flag and determines the
// interception strategy for destructive tools. For non-destructive tools,
// it leaves the flag in args and returns (0, false) so the handler's own
// dry-run logic can run. For destructive tools, it consumes the flag and
// returns the appropriate interception action.
func CheckDryRun(toolName string, args map[string]any, isDestructive bool) (Action, bool) {
	v, ok := args["dry_run"]
	if !ok {
		return 0, false
	}
	b, isBool := v.(bool)
	if !isBool || !b {
		return 0, false
	}

	// Non-destructive tools handle dry-run at the handler level.
	// Do NOT consume the flag — let it pass through.
	if !isDestructive {
		return 0, false
	}

	// Consume the flag for destructive tools (enforcement handles dry-run).
	delete(args, "dry_run")
	if confirmTools[toolName] {
		return ConfirmPattern, true
	}
	if _, found := previewMap[toolName]; found {
		return PreviewTool, true
	}
	if minimalTools[toolName] {
		return MinimalFallback, true
	}
	// Default for any other destructive tool.
	return MinimalFallback, true
}

// PreviewToolFor returns the GET counterpart for a destructive tool.
func PreviewToolFor(toolName string) (string, bool) {
	t, ok := previewMap[toolName]
	return t, ok
}

// BuildPreviewArgs extracts all keys ending in "_id" except "workspace_id"
// from args and returns a new map containing only those fields.
func BuildPreviewArgs(args map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range args {
		if k == "workspace_id" {
			continue
		}
		if strings.HasSuffix(k, "_id") {
			out[k] = v
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Result wrappers
// ---------------------------------------------------------------------------

// WrapResult wraps an API result in a dry-run envelope.
func WrapResult(result any, toolName string) map[string]any {
	return map[string]any{
		"dry_run":    true,
		"tool":       toolName,
		"preview":    result,
		"note":       "This is a dry-run preview. No changes were made.",
		"validation": unknownValidation("destructive dry-run preview fetched the target resource but did not validate every execution precondition"),
	}
}

// MinimalResult produces a dry-run envelope when no preview data is available.
func MinimalResult(toolName string, args map[string]any) map[string]any {
	return map[string]any{
		"dry_run":    true,
		"tool":       toolName,
		"args":       args,
		"resource":   nil,
		"note":       "This is a dry-run preview. No changes were made. No preview data available for this tool.",
		"validation": unknownValidation("minimal destructive dry-run has no safe read endpoint to validate the target"),
	}
}

func unknownValidation(message string) map[string]any {
	return map[string]any{
		"status":          "unknown",
		"preview_quality": "minimal_no_handler_validation",
		"warnings": []map[string]any{{
			"code":    "not_validated",
			"message": message,
		}},
	}
}
