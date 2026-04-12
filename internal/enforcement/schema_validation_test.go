package enforcement

import (
	"context"
	"errors"
	"testing"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/ratelimit"
)

// logTimeSchema mirrors the tightened InputSchema for clockify_log_time
// as produced by tools.normalizeDescriptors + tightenInputSchema. Kept
// local to this test so the enforcement package does not import
// internal/tools (which would be a layering violation).
func logTimeSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"start", "end"},
		"properties": map[string]any{
			"project_id":  map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"start":       map[string]any{"type": "string", "format": "date-time", "description": "RFC3339 timestamp"},
			"end":         map[string]any{"type": "string", "format": "date-time", "description": "RFC3339 timestamp"},
			"billable":    map[string]any{"type": "boolean"},
			"dry_run":     map[string]any{"type": "boolean"},
		},
	}
}

func newSchemaTestPipeline() *Pipeline {
	return &Pipeline{
		Policy:    standardPolicy(),
		RateLimit: ratelimit.New(10, 1000, 60000),
		DryRun:    dryrun.Config{Enabled: false},
	}
}

func TestBeforeCall_SchemaValidation_UnknownKey(t *testing.T) {
	p := newSchemaTestPipeline()
	args := map[string]any{
		"start": "2026-04-11T09:00:00Z",
		"end":   "2026-04-11T10:00:00Z",
		"bogus": "not a real field",
	}
	_, _, err := p.BeforeCall(context.Background(), "clockify_log_time", args, mcp.ToolHints{}, logTimeSchema(), noLookup)
	if err == nil {
		t.Fatal("expected rejection for unknown key")
	}
	var ipe *mcp.InvalidParamsError
	if !errors.As(err, &ipe) {
		t.Fatalf("want *mcp.InvalidParamsError, got %T: %v", err, err)
	}
	if ipe.Pointer != "/bogus" {
		t.Errorf("pointer = %q, want /bogus", ipe.Pointer)
	}
}

func TestBeforeCall_SchemaValidation_WrongType(t *testing.T) {
	p := newSchemaTestPipeline()
	args := map[string]any{
		"start":    "2026-04-11T09:00:00Z",
		"end":      "2026-04-11T10:00:00Z",
		"billable": "yes", // should be boolean
	}
	_, _, err := p.BeforeCall(context.Background(), "clockify_log_time", args, mcp.ToolHints{}, logTimeSchema(), noLookup)
	if err == nil {
		t.Fatal("expected rejection for wrong type")
	}
	var ipe *mcp.InvalidParamsError
	if !errors.As(err, &ipe) {
		t.Fatalf("want *mcp.InvalidParamsError, got %T: %v", err, err)
	}
	if ipe.Pointer != "/billable" {
		t.Errorf("pointer = %q, want /billable", ipe.Pointer)
	}
}

func TestBeforeCall_SchemaValidation_MissingRequired(t *testing.T) {
	p := newSchemaTestPipeline()
	args := map[string]any{
		"end": "2026-04-11T10:00:00Z",
	}
	_, _, err := p.BeforeCall(context.Background(), "clockify_log_time", args, mcp.ToolHints{}, logTimeSchema(), noLookup)
	if err == nil {
		t.Fatal("expected rejection for missing required")
	}
	var ipe *mcp.InvalidParamsError
	if !errors.As(err, &ipe) {
		t.Fatalf("want *mcp.InvalidParamsError, got %T: %v", err, err)
	}
	if ipe.Pointer != "/start" {
		t.Errorf("pointer = %q, want /start", ipe.Pointer)
	}
}

func TestBeforeCall_SchemaValidation_InvalidDateTime(t *testing.T) {
	p := newSchemaTestPipeline()
	args := map[string]any{
		"start": "not a date",
		"end":   "2026-04-11T10:00:00Z",
	}
	_, _, err := p.BeforeCall(context.Background(), "clockify_log_time", args, mcp.ToolHints{}, logTimeSchema(), noLookup)
	if err == nil {
		t.Fatal("expected rejection for invalid date-time")
	}
	var ipe *mcp.InvalidParamsError
	if !errors.As(err, &ipe) {
		t.Fatalf("want *mcp.InvalidParamsError, got %T: %v", err, err)
	}
	if ipe.Pointer != "/start" {
		t.Errorf("pointer = %q, want /start", ipe.Pointer)
	}
}

func TestBeforeCall_SchemaValidation_HappyPath(t *testing.T) {
	p := newSchemaTestPipeline()
	args := map[string]any{
		"start":       "2026-04-11T09:00:00Z",
		"end":         "2026-04-11T10:00:00Z",
		"project":     "Acme",
		"description": "work",
		"billable":    true,
	}
	_, release, err := p.BeforeCall(context.Background(), "clockify_log_time", args, mcp.ToolHints{}, logTimeSchema(), noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release == nil {
		t.Fatal("expected release from rate limiter")
	}
	release()
}

func TestBeforeCall_SchemaValidation_NilSchemaSkips(t *testing.T) {
	p := newSchemaTestPipeline()
	// With nil schema, validation is skipped — extra keys should pass.
	args := map[string]any{"bogus": "x"}
	_, release, err := p.BeforeCall(context.Background(), "clockify_list_entries", args, mcp.ToolHints{ReadOnly: true}, nil, noLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release != nil {
		release()
	}
}
