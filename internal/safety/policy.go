package safety

import (
	"strings"
)

type Requirement struct {
	RequiresDryRun       bool
	RequiresConfirmation bool
	Reason               string
}

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
