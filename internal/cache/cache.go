package cache

import (
	"crypto/md5"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached HTTP response
type CacheEntry struct {
	Data         []byte
	ETag         string
	LastModified string
	CachedAt     time.Time
	TTL          time.Duration
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Since(e.CachedAt) > e.TTL
}

// Cache provides in-memory caching with smart invalidation
type Cache struct {
	entries   map[string]*CacheEntry
	mutex     sync.RWMutex
	logger    *slog.Logger
	defaultTTL time.Duration
	httpClient *http.Client
}

// CacheOption configures the cache
type CacheOption func(*Cache)

// New creates a new cache instance
func New(opts ...CacheOption) *Cache {
	c := &Cache{
		entries:    make(map[string]*CacheEntry),
		logger:     slog.Default().With("component", "cache"),
		defaultTTL: 5 * time.Minute,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	
	for _, opt := range opts {
		opt(c)
	}
	
	return c
}

// WithLogger sets the logger for the cache
func WithLogger(logger *slog.Logger) CacheOption {
	return func(c *Cache) {
		c.logger = logger.With("component", "cache")
	}
}

// WithTTL sets the default TTL for cache entries
func WithTTL(ttl time.Duration) CacheOption {
	return func(c *Cache) {
		c.defaultTTL = ttl
	}
}

// WithHTTPClient sets the HTTP client for validation requests
func WithHTTPClient(client *http.Client) CacheOption {
	return func(c *Cache) {
		c.httpClient = client
	}
}

// BuildKey creates a cache key from URL and parameters
func (c *Cache) BuildKey(baseURL, endpoint string, params map[string]string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		// Fallback to simple concatenation if URL parsing fails
		c.logger.Warn("Failed to parse URL for cache key", "url", baseURL, "error", err)
		return fmt.Sprintf("%s%s", baseURL, endpoint)
	}
	
	// Handle case where baseURL doesn't have a scheme
	if u.Host == "" && u.Path != "" {
		// This means the "URL" was actually just a path or invalid
		return fmt.Sprintf("%s%s", baseURL, endpoint)
	}
	
	u.Path = endpoint
	
	// Sort parameters for consistent cache keys
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		
		var paramPairs []string
		for _, k := range keys {
			paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", k, params[k]))
		}
		
		query := strings.Join(paramPairs, "&")
		u.RawQuery = query
	}
	
	// Hash long URLs to keep keys manageable
	key := u.String()
	if len(key) > 200 {
		hash := md5.Sum([]byte(key))
		key = fmt.Sprintf("hash:%x", hash)
	}
	
	return key
}

// Get retrieves data from cache with smart validation
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()
	
	if !exists {
		c.logger.Debug("Cache miss", "key", key)
		return nil, false
	}
	
	// Check TTL expiration
	if entry.IsExpired() {
		c.logger.Debug("Cache entry expired", "key", key, "age", time.Since(entry.CachedAt))
		c.Delete(key)
		return nil, false
	}
	
	c.logger.Debug("Cache hit", "key", key, "age", time.Since(entry.CachedAt))
	return entry.Data, true
}

// Set stores data in cache with metadata
func (c *Cache) Set(key string, data []byte, etag, lastModified string) {
	entry := &CacheEntry{
		Data:         make([]byte, len(data)),
		ETag:         etag,
		LastModified: lastModified,
		CachedAt:     time.Now(),
		TTL:          c.defaultTTL,
	}
	copy(entry.Data, data)
	
	c.mutex.Lock()
	c.entries[key] = entry
	c.mutex.Unlock()
	
	c.logger.Debug("Cached entry", "key", key, "size", len(data), "etag", etag)
}

// Validate checks if cached content is still valid using HTTP headers
func (c *Cache) Validate(key, originalURL string) ([]byte, bool) {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	// If TTL hasn't expired, return cached data
	if !entry.IsExpired() {
		return entry.Data, true
	}
	
	// Perform conditional request to validate cache
	req, err := http.NewRequest("HEAD", originalURL, nil)
	if err != nil {
		c.logger.Error("Failed to create validation request", "url", originalURL, "error", err)
		c.Delete(key)
		return nil, false
	}
	
	// Add conditional headers
	if entry.ETag != "" {
		req.Header.Set("If-None-Match", entry.ETag)
	}
	if entry.LastModified != "" {
		req.Header.Set("If-Modified-Since", entry.LastModified)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("Failed to validate cache entry", "url", originalURL, "error", err)
		// Network error - keep cached entry for now
		return entry.Data, true
	}
	defer resp.Body.Close()
	
	// 304 Not Modified - content hasn't changed
	if resp.StatusCode == http.StatusNotModified {
		c.logger.Debug("Cache entry validated as current", "key", key)
		// Update cache timestamp to extend TTL
		c.mutex.Lock()
		entry.CachedAt = time.Now()
		c.mutex.Unlock()
		return entry.Data, true
	}
	
	// Content has changed, invalidate cache
	c.logger.Debug("Cache entry invalidated by server", "key", key, "status", resp.StatusCode)
	c.Delete(key)
	return nil, false
}

// Delete removes an entry from cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	delete(c.entries, key)
	c.mutex.Unlock()
	
	c.logger.Debug("Deleted cache entry", "key", key)
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mutex.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.mutex.Unlock()
	
	c.logger.Info("Cleared all cache entries")
}

// Stats returns cache statistics
func (c *Cache) Stats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	totalSize := 0
	expiredCount := 0
	
	for _, entry := range c.entries {
		totalSize += len(entry.Data)
		if entry.IsExpired() {
			expiredCount++
		}
	}
	
	return map[string]interface{}{
		"total_entries":   len(c.entries),
		"expired_entries": expiredCount,
		"total_size":      totalSize,
		"default_ttl":     c.defaultTTL.String(),
	}
}

// CleanExpired removes all expired entries
func (c *Cache) CleanExpired() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	var expiredKeys []string
	for key, entry := range c.entries {
		if entry.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}
	
	for _, key := range expiredKeys {
		delete(c.entries, key)
	}
	
	if len(expiredKeys) > 0 {
		c.logger.Debug("Cleaned expired cache entries", "count", len(expiredKeys))
	}
	
	return len(expiredKeys)
}