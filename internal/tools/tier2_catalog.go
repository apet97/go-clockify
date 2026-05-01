package tools

import (
	"sort"
	"sync"

	"github.com/apet97/go-clockify/internal/mcp"
)

// Tier2Group defines a lazily-activated group of related tools.
type Tier2Group struct {
	Name        string
	Description string
	Keywords    []string
	ToolNames   []string
	Builder     func(s *Service) []mcp.ToolDescriptor
}

// Tier2Groups is the global registry of Tier 2 tool groups.
var Tier2Groups = map[string]Tier2Group{}

var tier2CatalogIndex struct {
	once       sync.Once
	groupNames []string
	toolGroup  map[string]string
}

func init() {
	// Groups are registered by each tier2_*.go file via registerTier2Group.
}

func registerTier2Group(g Tier2Group) {
	Tier2Groups[g.Name] = g
}

func Tier2GroupNames() []string {
	names := tier2GroupNames()
	out := make([]string, len(names))
	copy(out, names)
	return out
}

func tier2GroupNames() []string {
	tier2CatalogIndex.once.Do(buildTier2CatalogIndex)
	return tier2CatalogIndex.groupNames
}

func Tier2GroupForTool(toolName string) (string, bool) {
	tier2CatalogIndex.once.Do(buildTier2CatalogIndex)
	groupName, ok := tier2CatalogIndex.toolGroup[toolName]
	return groupName, ok
}

func buildTier2CatalogIndex() {
	tier2CatalogIndex.groupNames = make([]string, 0, len(Tier2Groups))
	tier2CatalogIndex.toolGroup = make(map[string]string)
	for name, group := range Tier2Groups {
		tier2CatalogIndex.groupNames = append(tier2CatalogIndex.groupNames, name)
		for _, toolName := range group.ToolNames {
			tier2CatalogIndex.toolGroup[toolName] = name
		}
	}
	sort.Strings(tier2CatalogIndex.groupNames)
}

// Tier2Handlers returns the descriptors for a named Tier 2 group. Every
// returned descriptor has its Tool.OutputSchema populated with at least
// envelopeOpaque so MCP clients see a consistent envelope shape across
// all 91 lazy-loaded tools.
func (s *Service) Tier2Handlers(groupName string) ([]mcp.ToolDescriptor, bool) {
	g, ok := Tier2Groups[groupName]
	if !ok {
		return nil, false
	}
	return applyOpaqueOutputSchemas(normalizeDescriptors(g.Builder(s))), true
}
