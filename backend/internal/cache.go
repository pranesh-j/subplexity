// File: backend/internal/cache/cache.go

package cache

import (
	"container/list"
	"sync"
	"time"
)

// Item represents a cached item with expiration
type Item struct {
	Key        string
	Value      interface{}
	Expiration int64
}

// Cache provides a thread-safe LRU cache implementation with TTL
type Cache struct {
	maxItems    int
	items       map[string]*list.Element
	evictList   *list.List
	mu          sync.RWMutex
	defaultTTL  time.Duration
	janitor     *janitor
	onEvict     func(key string, value interface{})
	sizeBytes   int64
	maxSizeBytes int64
}

// Config holds cache configuration options
type Config struct {
	MaxItems     int
	DefaultTTL   time.Duration
	MaxSizeBytes int64
	CleanupInterval time.Duration
	OnEvict     func(key string, value interface{})
}

// DefaultConfig creates a default cache configuration
func DefaultConfig() Config {
	return Config{
		MaxItems:       1000,
		DefaultTTL:     15 * time.Minute,
		MaxSizeBytes:   50 * 1024 * 1024, // 50MB
		CleanupInterval: time.Minute,
		OnEvict:        nil,
	}
}

// NewCache creates a new cache with the given configuration
func NewCache(config Config) *Cache {
	if config.MaxItems <= 0 {
		config.MaxItems = DefaultConfig().MaxItems
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = DefaultConfig().DefaultTTL
	}
	if config.MaxSizeBytes <= 0 {
		config.MaxSizeBytes = DefaultConfig().MaxSizeBytes
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultConfig().CleanupInterval
	}

	cache := &Cache{
		maxItems:     config.MaxItems,
		items:        make(map[string]*list.Element, config.MaxItems),
		evictList:    list.New(),
		defaultTTL:   config.DefaultTTL,
		onEvict:      config.OnEvict,
		maxSizeBytes: config.MaxSizeBytes,
	}
	
	// Start the background cleanup process
	cache.janitor = &janitor{
		interval: config.CleanupInterval,
		stop:     make(chan struct{}),
	}
	
	go cache.janitor.run(cache)
	
	return cache
}

// Set adds an item to the cache with the default TTL
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL adds an item to the cache with a specific TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	// Estimate size (rough approximation)
	size := estimateSize(value)
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if item is already in cache and remove it
	if element, exists := c.items[key]; exists {
		c.evictList.Remove(element)
		delete(c.items, key)
		item := element.Value.(*Item)
		c.sizeBytes -= estimateSize(item.Value)
		if c.onEvict != nil {
			c.onEvict(key, item.Value)
		}
	}
	
	// Check if adding this would exceed size limit
	if c.sizeBytes+size > c.maxSizeBytes {
		c.evictOldest()
	}
	
	// Check if we need to make room
	for c.evictList.Len() >= c.maxItems {
		c.evictOldest()
	}
	
	// Create expiration time
	expiration := time.Now().Add(ttl).UnixNano()
	
	// Create cache entry
	item := &Item{
		Key:        key,
		Value:      value,
		Expiration: expiration,
	}
	
	// Add to cache
	element := c.evictList.PushFront(item)
	c.items[key] = element
	c.sizeBytes += size
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	element, found := c.items[key]
	if !found {
		return nil, false
	}
	
	item := element.Value.(*Item)
	
	// Check if item has expired
	if item.Expiration < time.Now().UnixNano() {
		c.removeElement(element)
		return nil, false
	}
	
	// Move to front (recently used)
	c.evictList.MoveToFront(element)
	
	return item.Value, true
}

// Remove item from cache
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	element, found := c.items[key]
	if !found {
		return false
	}
	
	c.removeElement(element)
	return true
}

// removeElement is an internal function to remove an element from the cache
func (c *Cache) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	item := e.Value.(*Item)
	delete(c.items, item.Key)
	c.sizeBytes -= estimateSize(item.Value)
	if c.onEvict != nil {
		c.onEvict(item.Key, item.Value)
	}
}

// evictOldest removes the oldest item from the cache 
func (c *Cache) evictOldest() {
	element := c.evictList.Back()
	if element != nil {
		c.removeElement(element)
	}
}

// Len returns the number of items in the cache
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictList.Len()
}

// Size returns the estimated memory size of the cache in bytes
func (c *Cache) Size() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sizeBytes
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.onEvict != nil {
		for _, element := range c.items {
			item := element.Value.(*Item)
			c.onEvict(item.Key, item.Value)
		}
	}
	
	c.items = make(map[string]*list.Element, c.maxItems)
	c.evictList = list.New()
	c.sizeBytes = 0
}

// deleteExpired deletes expired items from the cache
func (c *Cache) deleteExpired() {
	now := time.Now().UnixNano()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for _, element := range c.items {
		item := element.Value.(*Item)
		if item.Expiration < now {
			c.removeElement(element)
		}
	}
}

// Close stops the janitor
func (c *Cache) Close() {
	close(c.janitor.stop)
}

// janitor cleans up expired items at regular intervals
type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

// run runs the janitor process
func (j *janitor) run(c *Cache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			return
		}
	}
}

// estimateSize estimates the memory size of a value (very rough approximation)
func estimateSize(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		return int64(len(val))
	case []byte:
		return int64(len(val))
	case []interface{}:
		var size int64
		for _, item := range val {
			size += estimateSize(item)
		}
		return size
	case map[string]interface{}:
		var size int64
		for k, v := range val {
			size += int64(len(k)) + estimateSize(v)
		}
		return size
	default:
		// Very rough approximation
		return 100
	}
}