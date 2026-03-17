package plane

import (
	"sync"
	"time"
)

// cacheItem holds a cached value and its expiration time.
type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// Cache provides a simple in-memory TTL cache for Plane API data.
// It is safe for concurrent use.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

// NewCache creates a new empty cache.
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]cacheItem),
	}
}

// Get returns the cached value and true if the key exists and has not expired.
// Expired entries are deleted on access.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	return item.value, true
}

// Set stores a value in the cache with the given TTL.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a specific key from the cache.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// InvalidateAll clears the entire cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}
