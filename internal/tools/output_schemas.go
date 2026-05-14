package tools

import "github.com/apet97/go-clockify/internal/mcp"

// applyOpaqueOutputSchemas gives every descriptor that lacks an
// outputSchema a generic envelopeOpaque schema keyed by tool name. Used
// by the domain-handler families so every tool advertises at least the
// envelope wrapper without hand-crafting per-tool typed schemas.
func applyOpaqueOutputSchemas(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if in[i].Tool.OutputSchema == nil {
			in[i].Tool.OutputSchema = envelopeOpaque(in[i].Tool.Name)
		}
	}
	return in
}
