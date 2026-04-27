package dryrun

import (
	"fmt"
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
		"dry_run": true,
		"tool":    tool,
		"args":    args,
		"note":    "No changes were made.",
	}
}

func NotSupported(tool string) error {
	return fmt.Errorf("dry_run is not supported for non-destructive tool: %s", tool)
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
	"clockify_delete_entry":            "clockify_get_entry",
	"clockify_delete_invoice":          "clockify_get_invoice",
	"clockify_delete_expense":          "clockify_get_expense",
	"clockify_delete_custom_field":     "clockify_get_custom_field",
	"clockify_delete_assignment":       "clockify_get_assignment",
	"clockify_delete_shared_report":    "clockify_get_shared_report",
	"clockify_delete_time_off_request": "clockify_get_time_off_request",
	"clockify_delete_webhook":          "clockify_get_webhook",
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
	"clockify_delete_invoice_item":     true,
	"clockify_delete_expense_category": true,
	"clockify_delete_user_group":       true,
	"clockify_delete_user_group_admin": true,
	"clockify_delete_holiday":          true,
	"clockify_remove_user_from_group":  true,
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
		"dry_run": true,
		"tool":    toolName,
		"preview": result,
		"note":    "This is a dry-run preview. No changes were made.",
	}
}

// MinimalResult produces a dry-run envelope when no preview data is available.
func MinimalResult(toolName string, args map[string]any) map[string]any {
	return map[string]any{
		"dry_run":  true,
		"tool":     toolName,
		"args":     args,
		"resource": nil,
		"note":     "This is a dry-run preview. No changes were made. No preview data available for this tool.",
	}
}

// NotDestructiveError returns an error for non-destructive tools that
// received a dry_run flag.
func NotDestructiveError(toolName string) error {
	return fmt.Errorf("dry_run is not supported for non-destructive tool: %s", toolName)
}
