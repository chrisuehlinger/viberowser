package network

import (
	"net/http"
	"sync"
	"time"
)

// CacheEntry represents a cached HTTP response.
type CacheEntry struct {
	Response      *Response
	ETag          string
	LastMod       string
	MaxAge        time.Duration
	HasMaxAge     bool // Whether max-age was explicitly set (including 0)
	Expires       time.Time
	CachedAt      time.Time
	MustRevalid   bool // must-revalidate directive
}

// IsExpired returns true if the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	// If max-age was explicitly set, use it (including max-age=0 which means immediately expired)
	if e.HasMaxAge {
		return time.Since(e.CachedAt) > e.MaxAge
	}
	if !e.Expires.IsZero() {
		return time.Now().After(e.Expires)
	}
	// Default expiration of 5 minutes for entries without explicit expiration
	return time.Since(e.CachedAt) > 5*time.Minute
}

// CanRevalidate returns true if this entry can be revalidated.
func (e *CacheEntry) CanRevalidate() bool {
	return e.ETag != "" || e.LastMod != ""
}

// Cache provides in-memory HTTP caching.
type Cache struct {
	entries map[string]*CacheEntry
	maxSize int
	mu      sync.RWMutex
}

// NewCache creates a new cache with the specified maximum number of entries.
func NewCache(maxSize int) *Cache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &Cache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
	}
}

// Get retrieves a cached response if available and not expired.
func (c *Cache) Get(url string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[url]
	if !ok {
		return nil, false
	}

	return entry, true
}

// Set stores a response in the cache.
func (c *Cache) Set(url string, resp *Response, headers http.Header) {
	entry := &CacheEntry{
		Response: resp,
		CachedAt: time.Now(),
	}

	// Parse Cache-Control header
	cacheControl := headers.Get("Cache-Control")
	if cacheControl != "" {
		entry.MaxAge, entry.HasMaxAge, entry.MustRevalid = parseCacheControl(cacheControl)
	}

	// Check for no-store
	if containsDirective(cacheControl, "no-store") {
		return // Don't cache
	}

	// Get ETag and Last-Modified for revalidation
	entry.ETag = headers.Get("ETag")
	entry.LastMod = headers.Get("Last-Modified")

	// Parse Expires header if no max-age
	if entry.MaxAge == 0 {
		if expires := headers.Get("Expires"); expires != "" {
			if t, err := http.ParseTime(expires); err == nil {
				entry.Expires = t
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[url] = entry
}

// Delete removes an entry from the cache.
func (c *Cache) Delete(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, url)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the oldest entries to make room.
// Must be called with c.mu held.
func (c *Cache) evictOldest() {
	// Find and remove oldest entry
	var oldestURL string
	var oldestTime time.Time

	for url, entry := range c.entries {
		if oldestURL == "" || entry.CachedAt.Before(oldestTime) {
			oldestURL = url
			oldestTime = entry.CachedAt
		}
	}

	if oldestURL != "" {
		delete(c.entries, oldestURL)
	}
}

// Cleanup removes all expired entries from the cache.
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for url, entry := range c.entries {
		if entry.IsExpired() {
			delete(c.entries, url)
		}
	}
}

// parseCacheControl parses a Cache-Control header value.
func parseCacheControl(value string) (maxAge time.Duration, hasMaxAge bool, mustRevalidate bool) {
	// Simple parsing of max-age and must-revalidate
	directives := splitDirectives(value)

	for _, d := range directives {
		if len(d) > 8 && d[:8] == "max-age=" {
			var seconds int
			if _, err := parsePositiveInt(d[8:]); err == nil {
				seconds, _ = parsePositiveInt(d[8:])
				maxAge = time.Duration(seconds) * time.Second
				hasMaxAge = true
			}
		}
		if d == "must-revalidate" {
			mustRevalidate = true
		}
	}

	return
}

// containsDirective checks if a Cache-Control header contains a specific directive.
func containsDirective(cacheControl, directive string) bool {
	directives := splitDirectives(cacheControl)
	for _, d := range directives {
		if d == directive {
			return true
		}
	}
	return false
}

// splitDirectives splits a Cache-Control header into individual directives.
func splitDirectives(value string) []string {
	var result []string
	var current string

	for _, r := range value {
		switch r {
		case ',':
			if trimmed := trimSpace(current); trimmed != "" {
				result = append(result, trimmed)
			}
			current = ""
		default:
			current += string(r)
		}
	}

	if trimmed := trimSpace(current); trimmed != "" {
		result = append(result, trimmed)
	}

	return result
}

// trimSpace trims whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}

// parsePositiveInt parses a positive integer from a string.
func parsePositiveInt(s string) (int, error) {
	result := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}
