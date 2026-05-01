package tools

import (
	"reflect"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

func TestTier2HandlersRepeatedCallsStable(t *testing.T) {
	svc := &Service{}

	for _, groupName := range Tier2GroupNames() {
		first, ok := svc.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned !ok on first call", groupName)
		}
		second, ok := svc.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned !ok on second call", groupName)
		}
		assertDescriptorSlicesEqual(t, groupName, first, second)
	}
}

func TestTier2HandlersDefensiveCopies(t *testing.T) {
	svc := &Service{}
	descriptors, ok := svc.Tier2Handlers("invoices")
	if !ok {
		t.Fatal("invoices group not registered")
	}
	if len(descriptors) == 0 {
		t.Fatal("invoices group returned no descriptors")
	}

	first := descriptors[0]
	wantName := first.Tool.Name
	wantInputType := first.Tool.InputSchema["type"]
	inputProps := first.Tool.InputSchema["properties"].(map[string]any)
	pageSchema := inputProps["page"].(map[string]any)
	wantPageType := pageSchema["type"]
	wantOutputType := first.Tool.OutputSchema["type"]
	outputProps := first.Tool.OutputSchema["properties"].(map[string]any)
	actionSchema := outputProps["action"].(map[string]any)
	wantActionConst := actionSchema["const"]
	wantReadOnlyAnnotation := first.Tool.Annotations["readOnlyHint"]

	auditIndex := descriptorIndex(descriptors, "clockify_create_invoice")
	if auditIndex < 0 {
		t.Fatal("clockify_create_invoice descriptor not found")
	}
	if len(descriptors[auditIndex].AuditKeys) == 0 {
		t.Fatal("clockify_create_invoice has no audit keys")
	}
	required := descriptors[auditIndex].Tool.InputSchema["required"].([]string)
	if len(required) == 0 {
		t.Fatal("clockify_create_invoice has no required fields")
	}
	wantAuditKey := descriptors[auditIndex].AuditKeys[0]
	wantRequired := required[0]

	descriptors[0].Tool.Name = "caller_mutated_name"
	descriptors[0].Tool.InputSchema["type"] = "caller_mutated_input"
	pageSchema["type"] = "caller_mutated_page"
	inputProps["caller_mutated"] = map[string]any{"type": "string"}
	descriptors[0].Tool.OutputSchema["type"] = "caller_mutated_output"
	actionSchema["const"] = "caller_mutated_action"
	descriptors[0].Tool.Annotations["readOnlyHint"] = "caller_mutated_annotation"
	descriptors[auditIndex].AuditKeys[0] = "caller_mutated_audit"
	required[0] = "caller_mutated_required"

	after, ok := svc.Tier2Handlers("invoices")
	if !ok {
		t.Fatal("invoices group not registered after mutation")
	}
	if got := after[0].Tool.Name; got != wantName {
		t.Fatalf("descriptor name mutated cached storage: got %q, want %q", got, wantName)
	}
	if got := after[0].Tool.InputSchema["type"]; got != wantInputType {
		t.Fatalf("input schema type mutated cached storage: got %v, want %v", got, wantInputType)
	}
	afterInputProps := after[0].Tool.InputSchema["properties"].(map[string]any)
	if _, ok := afterInputProps["caller_mutated"]; ok {
		t.Fatal("input schema properties mutation leaked into cached storage")
	}
	afterPageSchema := afterInputProps["page"].(map[string]any)
	if got := afterPageSchema["type"]; got != wantPageType {
		t.Fatalf("nested input schema mutated cached storage: got %v, want %v", got, wantPageType)
	}
	if got := after[0].Tool.OutputSchema["type"]; got != wantOutputType {
		t.Fatalf("output schema type mutated cached storage: got %v, want %v", got, wantOutputType)
	}
	afterOutputProps := after[0].Tool.OutputSchema["properties"].(map[string]any)
	afterActionSchema := afterOutputProps["action"].(map[string]any)
	if got := afterActionSchema["const"]; got != wantActionConst {
		t.Fatalf("nested output schema mutated cached storage: got %v, want %v", got, wantActionConst)
	}
	if got := after[0].Tool.Annotations["readOnlyHint"]; got != wantReadOnlyAnnotation {
		t.Fatalf("annotations mutated cached storage: got %v, want %v", got, wantReadOnlyAnnotation)
	}
	afterAuditIndex := descriptorIndex(after, "clockify_create_invoice")
	if afterAuditIndex < 0 {
		t.Fatal("clockify_create_invoice descriptor not found after mutation")
	}
	if got := after[afterAuditIndex].AuditKeys[0]; got != wantAuditKey {
		t.Fatalf("audit keys mutated cached storage: got %q, want %q", got, wantAuditKey)
	}
	afterRequired := after[afterAuditIndex].Tool.InputSchema["required"].([]string)
	if got := afterRequired[0]; got != wantRequired {
		t.Fatalf("required fields mutated cached storage: got %q, want %q", got, wantRequired)
	}
}

func assertDescriptorSlicesEqual(t *testing.T, groupName string, want, got []mcp.ToolDescriptor) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("group %s: second call returned %d descriptors, want %d", groupName, len(got), len(want))
	}
	for i := range want {
		assertDescriptorEqual(t, groupName, i, want[i], got[i])
	}
}

func assertDescriptorEqual(t *testing.T, groupName string, index int, want, got mcp.ToolDescriptor) {
	t.Helper()
	if want.Tool.Name != got.Tool.Name {
		t.Fatalf("group %s descriptor %d name = %q, want %q", groupName, index, got.Tool.Name, want.Tool.Name)
	}
	if want.Tool.Description != got.Tool.Description {
		t.Fatalf("group %s descriptor %s description changed", groupName, want.Tool.Name)
	}
	if !reflect.DeepEqual(want.Tool.InputSchema, got.Tool.InputSchema) {
		t.Fatalf("group %s descriptor %s input schema changed", groupName, want.Tool.Name)
	}
	if !reflect.DeepEqual(want.Tool.OutputSchema, got.Tool.OutputSchema) {
		t.Fatalf("group %s descriptor %s output schema changed", groupName, want.Tool.Name)
	}
	if !reflect.DeepEqual(want.Tool.Annotations, got.Tool.Annotations) {
		t.Fatalf("group %s descriptor %s annotations changed", groupName, want.Tool.Name)
	}
	if want.ReadOnlyHint != got.ReadOnlyHint ||
		want.DestructiveHint != got.DestructiveHint ||
		want.IdempotentHint != got.IdempotentHint ||
		want.RiskClass != got.RiskClass {
		t.Fatalf("group %s descriptor %s hints/risk changed", groupName, want.Tool.Name)
	}
	if !reflect.DeepEqual(want.AuditKeys, got.AuditKeys) {
		t.Fatalf("group %s descriptor %s audit keys changed", groupName, want.Tool.Name)
	}
	if (want.Handler == nil) != (got.Handler == nil) {
		t.Fatalf("group %s descriptor %s handler nilness changed", groupName, want.Tool.Name)
	}
}

func descriptorIndex(descriptors []mcp.ToolDescriptor, name string) int {
	for i, descriptor := range descriptors {
		if descriptor.Tool.Name == name {
			return i
		}
	}
	return -1
}
