package safety

import (
	"strings"
)

// Requirement captures the safety preconditions a tool call must
// satisfy before it can be executed. RequiresDryRun forces the caller
// to first run with dry_run=true so the user previews the operation;
// RequiresConfirmation forces a follow-up call carrying a
// confirmation token bound to the dry-run preview. Reason explains
// which classifier triggered the requirement and is surfaced to the
// MCP client in the recovery envelope.
type Requirement struct {
	RequiresDryRun       bool
	RequiresConfirmation bool
	Reason               string
}

// RequirementForRisk classifies a tool call's safety needs from its
// declared risk labels, destructive flag, and (for raw fallback)
// HTTP method. Raw DELETE/POST/PUT/PATCH always require both dry-run
// and confirmation. Destructive tools always require both. Tools
// tagged billing, admin, permission_change, or external_side_effect
// require confirmation (dry-run only when also destructive). All
// other risk labels are passthrough.
func RequirementForRisk(riskNames []string, destructive bool, method string) Requirement {
	upperMethod := strings.ToUpper(strings.TrimSpace(method))
	if upperMethod == "DELETE" {
		return Requirement{
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			Reason:               "raw DELETE requires dry-run preview and confirmation",
		}
	}
	if upperMethod == "POST" || upperMethod == "PUT" || upperMethod == "PATCH" {
		return Requirement{
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			Reason:               "raw mutating write requires dry-run preview and confirmation",
		}
	}
	if destructive {
		return Requirement{
			RequiresDryRun:       true,
			RequiresConfirmation: true,
			Reason:               "destructive tool requires dry-run preview and confirmation",
		}
	}
	for _, risk := range riskNames {
		switch strings.ToLower(strings.TrimSpace(risk)) {
		case "billing", "admin", "permission_change", "external_side_effect", "destructive":
			return Requirement{
				RequiresDryRun:       risk == "destructive",
				RequiresConfirmation: true,
				Reason:               risk + " tool requires confirmation",
			}
		}
	}
	return Requirement{}
}
