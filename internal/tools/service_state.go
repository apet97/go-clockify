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

// cachedUserSnapshot returns a copy of the cached current user, if one has
// been stored. The lock dance lives with the state holder so callers do not
// reach into the mutex directly.
func (c *identityCacheState) cachedUserSnapshot() (clockify.User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cachedUser == nil {
		return clockify.User{}, false
	}
	return *c.cachedUser, true
}

// storeUser caches a copy of the current user for subsequent lookups.
func (c *identityCacheState) storeUser(user clockify.User) {
	c.mu.Lock()
	c.cachedUser = &user
	c.mu.Unlock()
}

// cachedWorkspaceID returns the auto-detected workspace ID, if one has been
// cached.
func (c *identityCacheState) cachedWorkspaceID() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cachedWSID == "" {
		return "", false
	}
	return c.cachedWSID, true
}

// storeWorkspaceID caches the auto-detected workspace ID.
func (c *identityCacheState) storeWorkspaceID(id string) {
	c.mu.Lock()
	c.cachedWSID = id
	c.mu.Unlock()
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
