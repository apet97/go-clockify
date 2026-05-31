package tools

import (
	"sync"
	"time"

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

// get returns a live (non-expired) cached ID for key. The locking lives with
// the cache state so callers do not touch the mutex directly.
func (c *resolverCacheState) get(key resolveKey, now time.Time) (string, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !now.Before(entry.expiresAt) {
		return "", false
	}
	return entry.id, true
}

// set stores id under key with the given expiry. When the cache reaches the
// sweep threshold it first drops every expired entry so a long-lived Service
// does not grow without bound.
func (c *resolverCacheState) set(key resolveKey, id string, expiresAt time.Time) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[resolveKey]resolveEntry)
	}
	if len(c.entries) >= resolveCacheSweepThreshold {
		now := time.Now()
		for cachedKey, entry := range c.entries {
			if !now.Before(entry.expiresAt) {
				delete(c.entries, cachedKey)
			}
		}
	}
	c.entries[key] = resolveEntry{id: id, expiresAt: expiresAt}
	c.mu.Unlock()
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
