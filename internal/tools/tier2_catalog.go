package tools

import "goclmcp/internal/mcp"

// Tier2Group defines a lazily-activated group of related tools.
type Tier2Group struct {
	Name        string
	Description string
	Keywords    []string
	Builder     func(s *Service) []mcp.ToolDescriptor
}

// Tier2Groups is the global registry of Tier 2 tool groups.
var Tier2Groups = map[string]Tier2Group{}

func init() {
	// Groups are registered by each tier2_*.go file via registerTier2Group.
}

func registerTier2Group(g Tier2Group) {
	Tier2Groups[g.Name] = g
}

// Tier2Handlers returns the descriptors for a named Tier 2 group.
func (s *Service) Tier2Handlers(groupName string) ([]mcp.ToolDescriptor, bool) {
	g, ok := Tier2Groups[groupName]
	if !ok {
		return nil, false
	}
	return g.Builder(s), true
}
