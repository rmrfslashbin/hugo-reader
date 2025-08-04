package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cache := New()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.entries)
	assert.Equal(t, 5*time.Minute, cache.defaultTTL)
}

func TestCache_BuildKey(t *testing.T) {
	cache := New()
	
	tests := []struct {
		name     string
		baseURL  string
		endpoint string
		params   map[string]string
		want     string
	}{
		{
			name:     "simple URL with endpoint",
			baseURL:  "https://example.com",
			endpoint: "/api/taxonomies.json",
			params:   nil,
			want:     "https://example.com/api/taxonomies.json",
		},
		{
			name:     "URL with parameters",
			baseURL:  "https://example.com",
			endpoint: "/search.json",
			params:   map[string]string{"q": "golang", "limit": "10"},
			want:     "https://example.com/search.json?limit=10&q=golang",
		},
		{
			name:     "invalid URL fallback",
			baseURL:  "not-a-url",
			endpoint: "/test",
			params:   nil,
			want:     "not-a-url/test",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.BuildKey(tt.baseURL, tt.endpoint, tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCache_SetAndGet(t *testing.T) {
	cache := New()
	key := "test-key"
	data := []byte("test data")
	
	// Test cache miss
	result, found := cache.Get(key)
	assert.False(t, found)
	assert.Nil(t, result)
	
	// Test cache set and hit
	cache.Set(key, data, "etag123", "Mon, 01 Jan 2024 00:00:00 GMT")
	result, found = cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, data, result)
}

func TestCache_Expiration(t *testing.T) {
	cache := New(WithTTL(10 * time.Millisecond))
	key := "test-key"
	data := []byte("test data")
	
	cache.Set(key, data, "", "")
	
	// Should be available immediately
	result, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, data, result)
	
	// Wait for expiration
	time.Sleep(20 * time.Millisecond)
	
	// Should be expired and removed
	result, found = cache.Get(key)
	assert.False(t, found)
	assert.Nil(t, result)
}

func TestCache_Delete(t *testing.T) {
	cache := New()
	key := "test-key"
	data := []byte("test data")
	
	cache.Set(key, data, "", "")
	
	// Verify it exists
	_, found := cache.Get(key)
	assert.True(t, found)
	
	// Delete it
	cache.Delete(key)
	
	// Verify it's gone
	_, found = cache.Get(key)
	assert.False(t, found)
}

func TestCache_Clear(t *testing.T) {
	cache := New()
	
	// Add multiple entries
	cache.Set("key1", []byte("data1"), "", "")
	cache.Set("key2", []byte("data2"), "", "")
	
	// Verify they exist
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	assert.True(t, found1)
	assert.True(t, found2)
	
	// Clear cache
	cache.Clear()
	
	// Verify they're gone
	_, found1 = cache.Get("key1")
	_, found2 = cache.Get("key2")
	assert.False(t, found1)
	assert.False(t, found2)
}

func TestCache_Stats(t *testing.T) {
	cache := New(WithTTL(10 * time.Millisecond))
	
	// Add some entries
	cache.Set("key1", []byte("data1"), "", "")
	cache.Set("key2", []byte("longer data here"), "", "")
	
	stats := cache.Stats()
	
	assert.Equal(t, 2, stats["total_entries"])
	assert.Equal(t, 0, stats["expired_entries"])
	assert.Equal(t, 21, stats["total_size"]) // len("data1") + len("longer data here")
	assert.Equal(t, "10ms", stats["default_ttl"])
	
	// Wait for expiration
	time.Sleep(20 * time.Millisecond)
	
	stats = cache.Stats()
	assert.Equal(t, 2, stats["total_entries"])
	assert.Equal(t, 2, stats["expired_entries"])
}

func TestCache_CleanExpired(t *testing.T) {
	cache := New(WithTTL(10 * time.Millisecond))
	
	// Add entries
	cache.Set("key1", []byte("data1"), "", "")
	cache.Set("key2", []byte("data2"), "", "")
	
	// Wait for expiration
	time.Sleep(20 * time.Millisecond)
	
	// Clean expired entries
	removedCount := cache.CleanExpired()
	assert.Equal(t, 2, removedCount)
	
	// Verify cache is empty
	stats := cache.Stats()
	assert.Equal(t, 0, stats["total_entries"])
}

func TestCacheEntry_IsExpired(t *testing.T) {
	entry := &CacheEntry{
		CachedAt: time.Now().Add(-1 * time.Hour),
		TTL:      30 * time.Minute,
	}
	
	assert.True(t, entry.IsExpired())
	
	entry.CachedAt = time.Now()
	assert.False(t, entry.IsExpired())
}