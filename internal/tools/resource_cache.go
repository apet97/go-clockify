package tools

import (
	"container/list"
	"sync"
)

// resourceStateCache is a bounded LRU that holds the last-emitted
// serialisation of each subscribed resource URI. It powers delta-sync
// on notifications/resources/updated: the mutation path re-reads the
// resource through ResourceProvider, diffs it against the cached prior
// snapshot, and emits the smallest JSON Merge Patch that apples to
// yield the fresh state. When the cache is empty for a URI (fresh
// subscription or eviction), the emitter falls back to format=none so
// subscribed clients know to re-fetch.
//
// Default capacity is 1024 URIs. Subscribing to more than the cap just
// means the oldest URIs lose their baseline and the next notification
// for those URIs is a format=none miss, which is still correct — no
// data is lost, just bandwidth.
type resourceStateCache struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List
	index map[string]*list.Element
}

type resourceCacheEntry struct {
	uri  string
	data []byte
}

func newResourceStateCache(capacity int) *resourceStateCache {
	if capacity <= 0 {
		capacity = 1024
	}
	return &resourceStateCache{
		cap:   capacity,
		ll:    list.New(),
		index: make(map[string]*list.Element, capacity),
	}
}

// get returns the last cached serialisation for uri and whether it was
// present. Hits refresh the LRU position; misses do not.
func (c *resourceStateCache) get(uri string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[uri]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	entry, ok := el.Value.(*resourceCacheEntry)
	if !ok {
		return nil, false
	}
	out := make([]byte, len(entry.data))
	copy(out, entry.data)
	return out, true
}

// put stores data for uri, refreshing its LRU position. Evicts the
// oldest entry when the cache is at capacity.
func (c *resourceStateCache) put(uri string, data []byte) {
	if c == nil {
		return
	}
	// Copy defensive: we intentionally own the bytes once they enter the
	// cache so subsequent writers can mutate their own slices without
	// affecting cached state.
	snapshot := make([]byte, len(data))
	copy(snapshot, data)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[uri]; ok {
		c.ll.MoveToFront(el)
		if existing, typeOK := el.Value.(*resourceCacheEntry); typeOK {
			existing.data = snapshot
		}
		return
	}
	el := c.ll.PushFront(&resourceCacheEntry{uri: uri, data: snapshot})
	c.index[uri] = el
	if c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest != nil {
			if entry, ok := oldest.Value.(*resourceCacheEntry); ok {
				delete(c.index, entry.uri)
			}
			c.ll.Remove(oldest)
		}
	}
}

// drop removes uri from the cache if present. Called when the resource
// is confirmed deleted by a mutation path so the next subscribe can
// emit a clean format=none on first notification.
func (c *resourceStateCache) drop(uri string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[uri]; ok {
		delete(c.index, uri)
		c.ll.Remove(el)
	}
}

// len returns the current entry count. Test-only helper; not used by
// production code paths.
func (c *resourceStateCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
