package tools

import (
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/jsonschema"
)

// TestResolveNameInputSchema_EnumConstraint pins the enum tightening on
// the legacy name resolver's entity_type field. Before this change the
// schema accepted any string and the handler did the run-time check via
// a switch statement; spec-strict MCP clients had no way to know the
// valid set up front. The enum surfaces the contract through tools/list
// so validators reject typos at parse time, before any handler runs.
//
// Drift check: remove the enum entry from registry.go's schemaEnum call
// (collapse to schemaString) and the "invalid entity_type rejected"
// assertion fails because jsonschema.Validate accepts any string under
// {"type":"string"} alone.
func TestResolveNameInputSchema_EnumConstraint(t *testing.T) {
	schema := resolveNameInputSchema()

	// Every documented enum value must validate cleanly.
	for _, valid := range []string{"project", "client", "tag", "user", "task"} {
		args := map[string]any{
			"entity_type": valid,
			"name_or_id":  "test",
		}
		if err := jsonschema.Validate(schema, args); err != nil {
			t.Fatalf("entity_type=%q rejected: %v", valid, err)
		}
	}

	// Bogus values must be rejected by the validator, NOT by the
	// downstream handler switch statement.
	for _, invalid := range []string{"workspace", "team", "TASK", "Project", "", "tasks"} {
		args := map[string]any{
			"entity_type": invalid,
			"name_or_id":  "test",
		}
		err := jsonschema.Validate(schema, args)
		if err == nil {
			t.Fatalf("entity_type=%q accepted by schema; enum constraint missing", invalid)
		}
		// Sanity: the error references the offending field.
		if !strings.Contains(err.Error(), "entity_type") &&
			!strings.Contains(err.Error(), "enum") {
			t.Fatalf("entity_type=%q rejection error did not mention field/enum: %v", invalid, err)
		}
	}
}
