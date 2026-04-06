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
	ConfirmPattern  Action = iota // remove confirm flag, call handler normally for preview
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

// confirmTools use confirm-pattern interception.
var confirmTools = map[string]bool{
	"clockify_send_invoice":      true,
	"clockify_approve_timesheet": true,
	"clockify_reject_timesheet":  true,
	"clockify_deactivate_user":   true,
}

// minimalTools use minimal fallback (no GET counterpart).
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
// interception strategy. It consumes (deletes) the dry_run key from args.
// Returns (action, true) when dry-run is active, or (0, false) otherwise.
func CheckDryRun(toolName string, args map[string]any, isDestructive bool) (Action, bool) {
	v, ok := args["dry_run"]
	if !ok {
		return 0, false
	}
	b, isBool := v.(bool)
	if !isBool || !b {
		return 0, false
	}

	// Consume the flag.
	delete(args, "dry_run")

	if !isDestructive {
		return NotDestructive, true
	}
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
