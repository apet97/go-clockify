package safety

import "testing"

func TestRequirementForRiskRequiresConfirmationForHighRisk(t *testing.T) {
	for _, risk := range []string{"destructive", "billing", "admin", "permission_change", "external_side_effect"} {
		req := RequirementForRisk([]string{risk}, false, "")
		if !req.RequiresConfirmation {
			t.Fatalf("%s did not require confirmation: %#v", risk, req)
		}
	}
}

func TestRequirementForRiskLeavesOrdinaryWritesAlone(t *testing.T) {
	req := RequirementForRisk([]string{"write"}, false, "")
	if req.RequiresConfirmation {
		t.Fatalf("ordinary write required confirmation: %#v", req)
	}
}

func TestRequirementForRiskRequiresConfirmationForRawDelete(t *testing.T) {
	req := RequirementForRisk([]string{"write"}, false, "DELETE")
	if !req.RequiresDryRun || !req.RequiresConfirmation {
		t.Fatalf("raw DELETE requirement = %#v", req)
	}
}
