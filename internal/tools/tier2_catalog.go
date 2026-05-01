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
	if s == nil {
		return applyOpaqueOutputSchemas(normalizeDescriptors(g.Builder(s))), true
	}

	s.tier2CacheMu.Lock()
	if cached, ok := s.tier2Cache[groupName]; ok {
		s.tier2CacheMu.Unlock()
		return cloneToolDescriptors(cached), true
	}

	descriptors := applyOpaqueOutputSchemas(normalizeDescriptors(g.Builder(s)))
	if s.tier2Cache == nil {
		s.tier2Cache = make(map[string][]mcp.ToolDescriptor, len(Tier2Groups))
	}
	s.tier2Cache[groupName] = descriptors
	s.tier2CacheMu.Unlock()
	return cloneToolDescriptors(descriptors), true
}

func cloneToolDescriptors(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, len(in))
	for i, descriptor := range in {
		out[i] = descriptor
		out[i].Tool.InputSchema = cloneDescriptorMap(descriptor.Tool.InputSchema)
		out[i].Tool.OutputSchema = cloneOutputSchema(descriptor.Tool)
		out[i].Tool.Annotations = cloneAnnotations(descriptor.Tool.Annotations)
		out[i].AuditKeys = append([]string(nil), descriptor.AuditKeys...)
	}
	return out
}

func cloneOutputSchema(tool mcp.Tool) map[string]any {
	if tool.OutputSchema == nil {
		return nil
	}
	if isOpaqueOutputSchema(tool.OutputSchema, tool.Name) {
		return envelopeOpaque(tool.Name)
	}
	return cloneDescriptorMap(tool.OutputSchema)
}

func isOpaqueOutputSchema(schema map[string]any, action string) bool {
	typ, _ := schema["type"].(string)
	required, _ := schema["required"].([]string)
	properties, _ := schema["properties"].(map[string]any)
	actionSchema, _ := properties["action"].(map[string]any)
	constValue, _ := actionSchema["const"].(string)
	return typ == "object" &&
		len(required) == 2 &&
		required[0] == "ok" &&
		required[1] == "action" &&
		constValue == action
}

func cloneAnnotations(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneDescriptorMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneDescriptorValue(value)
	}
	return out
}

func cloneDescriptorValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneDescriptorMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneDescriptorValue(item)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}
