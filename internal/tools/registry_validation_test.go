package tools

import (
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

func TestValidateRegistryRejectsDuplicateNames(t *testing.T) {
	svc := &Service{}
	reg := svc.FullAccessRegistry()
	bad := append(cloneToolDescriptors(reg[:1]), reg[0])

	err := ValidateRegistry(bad)
	if err == nil || !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("ValidateRegistry duplicate error = %v", err)
	}
}

func TestValidateRegistryRejectsMissingOutputSchema(t *testing.T) {
	svc := &Service{}
	reg := svc.FullAccessRegistry()
	bad := cloneToolDescriptors(reg[:1])
	bad[0].Tool.OutputSchema = nil

	err := ValidateRegistry(bad)
	if err == nil || !strings.Contains(err.Error(), "output schema") {
		t.Fatalf("ValidateRegistry missing output schema error = %v", err)
	}
}

func TestValidateRegistryRejectsRawToolBeforeEnd(t *testing.T) {
	svc := &Service{}
	reg := svc.FullAccessRegistry()
	if len(reg) < 3 {
		t.Fatal("registry too small")
	}
	raw := reg[len(reg)-1]
	nonRaw := reg[0]
	bad := []mcp.ToolDescriptor{raw, nonRaw}

	err := ValidateRegistry(bad)
	if err == nil || !strings.Contains(err.Error(), "raw tools must be last") {
		t.Fatalf("ValidateRegistry raw order error = %v", err)
	}
}

func TestFullAccessRegistryPassesValidation(t *testing.T) {
	svc := &Service{}
	if err := ValidateRegistry(svc.FullAccessRegistry()); err != nil {
		t.Fatalf("FullAccessRegistry validation failed: %v", err)
	}
}
