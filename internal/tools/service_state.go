package tools

import (
	"sync"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// Small state holders keep Service focused on runtime dependencies and
// product configuration while leaving caches close to their owning concern.
type identityCacheState struct {
	mu         sync.RWMutex
	cachedUser *clockify.User
	cachedWSID string
}

type resolverCacheState struct {
	mu      sync.RWMutex
	entries map[resolveKey]resolveEntry
}

type resourceEmitterState struct {
	mu            sync.RWMutex
	cache         *resourceStateCache
	demoResources map[string]demoResourceState
}

type registryState struct {
	once               sync.Once
	descriptors        []mcp.ToolDescriptor
	err                error
	toolsResourceOnce  sync.Once
	toolsResourceCache map[string]any
}
