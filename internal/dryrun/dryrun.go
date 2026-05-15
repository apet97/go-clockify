// Package dryrun builds the standard dry-run preview envelopes that tool
// handlers return when a caller passes dry_run:true.
package dryrun

// Enabled reports whether args carries dry_run:true.
func Enabled(args map[string]any) bool {
	if args == nil {
		return false
	}
	b, ok := args["dry_run"].(bool)
	return ok && b
}

// Preview is the generic dry-run envelope for a tool that ran no
// tool-specific validation.
func Preview(tool string, args map[string]any) map[string]any {
	return map[string]any{
		"dry_run":    true,
		"tool":       tool,
		"args":       args,
		"note":       "No changes were made.",
		"validation": unknownValidation("generic dry-run preview did not run tool-specific validation"),
	}
}

// WrapResult wraps a fetched target resource in a destructive-tool dry-run
// envelope.
func WrapResult(result any, toolName string) map[string]any {
	return map[string]any{
		"dry_run":    true,
		"tool":       toolName,
		"preview":    result,
		"note":       "This is a dry-run preview. No changes were made.",
		"validation": unknownValidation("destructive dry-run preview fetched the target resource but did not validate every execution precondition"),
	}
}

// MinimalResult is the dry-run envelope for a destructive tool with no safe
// read endpoint to preview the target.
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
