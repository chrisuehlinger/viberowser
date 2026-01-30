package network

import (
	"net/http"
	"testing"
	"time"
)

func TestCacheBasic(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{
		StatusCode:  200,
		Body:        []byte("test content"),
		ContentType: "text/plain",
	}

	headers := http.Header{}
	headers.Set("Cache-Control", "max-age=3600")

	cache.Set("http://example.com/test", resp, headers)

	entry, ok := cache.Get("http://example.com/test")
	if !ok {
		t.Fatal("expected to find cached entry")
	}

	if string(entry.Response.Body) != "test content" {
		t.Errorf("Body = %q, want %q", string(entry.Response.Body), "test content")
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{
		StatusCode: 200,
		Body:       []byte("test"),
	}

	// Set with very short max-age
	headers := http.Header{}
	headers.Set("Cache-Control", "max-age=0")

	cache.Set("http://example.com/test", resp, headers)

	entry, ok := cache.Get("http://example.com/test")
	if !ok {
		t.Fatal("expected to find cached entry")
	}

	// Entry should be expired immediately
	if !entry.IsExpired() {
		t.Error("expected entry to be expired")
	}
}

func TestCacheNoStore(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{
		StatusCode: 200,
		Body:       []byte("secret"),
	}

	headers := http.Header{}
	headers.Set("Cache-Control", "no-store")

	cache.Set("http://example.com/secret", resp, headers)

	_, ok := cache.Get("http://example.com/secret")
	if ok {
		t.Error("no-store response should not be cached")
	}
}

func TestCacheETagRevalidation(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{
		StatusCode: 200,
		Body:       []byte("test"),
	}

	headers := http.Header{}
	headers.Set("ETag", `"abc123"`)

	cache.Set("http://example.com/test", resp, headers)

	entry, ok := cache.Get("http://example.com/test")
	if !ok {
		t.Fatal("expected to find cached entry")
	}

	if entry.ETag != `"abc123"` {
		t.Errorf("ETag = %q, want %q", entry.ETag, `"abc123"`)
	}

	if !entry.CanRevalidate() {
		t.Error("expected entry to be revalidatable")
	}
}

func TestCacheLastModified(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{
		StatusCode: 200,
		Body:       []byte("test"),
	}

	headers := http.Header{}
	headers.Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")

	cache.Set("http://example.com/test", resp, headers)

	entry, ok := cache.Get("http://example.com/test")
	if !ok {
		t.Fatal("expected to find cached entry")
	}

	if entry.LastMod != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Errorf("LastMod = %q", entry.LastMod)
	}

	if !entry.CanRevalidate() {
		t.Error("expected entry to be revalidatable")
	}
}

func TestCacheEviction(t *testing.T) {
	cache := NewCache(3)

	headers := http.Header{}
	headers.Set("Cache-Control", "max-age=3600")

	// Add 3 entries
	for i := 0; i < 3; i++ {
		resp := &Response{StatusCode: 200, Body: []byte{byte(i)}}
		cache.Set("http://example.com/"+string(rune('a'+i)), resp, headers)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	if cache.Size() != 3 {
		t.Errorf("Size = %d, want 3", cache.Size())
	}

	// Add 4th entry, should evict oldest
	resp := &Response{StatusCode: 200, Body: []byte{3}}
	cache.Set("http://example.com/d", resp, headers)

	if cache.Size() != 3 {
		t.Errorf("Size after eviction = %d, want 3", cache.Size())
	}

	// First entry should be evicted
	_, ok := cache.Get("http://example.com/a")
	if ok {
		t.Error("oldest entry should have been evicted")
	}
}

func TestCacheDelete(t *testing.T) {
	cache := NewCache(100)

	resp := &Response{StatusCode: 200, Body: []byte("test")}
	headers := http.Header{}

	cache.Set("http://example.com/test", resp, headers)

	cache.Delete("http://example.com/test")

	_, ok := cache.Get("http://example.com/test")
	if ok {
		t.Error("deleted entry should not be found")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewCache(100)

	headers := http.Header{}
	for i := 0; i < 10; i++ {
		resp := &Response{StatusCode: 200}
		cache.Set("http://example.com/"+string(rune('a'+i)), resp, headers)
	}

	if cache.Size() != 10 {
		t.Errorf("Size = %d, want 10", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Size after clear = %d, want 0", cache.Size())
	}
}

func TestCacheCleanup(t *testing.T) {
	cache := NewCache(100)

	// Add entry with immediate expiration
	resp1 := &Response{StatusCode: 200, Body: []byte("expired")}
	headers1 := http.Header{}
	headers1.Set("Cache-Control", "max-age=0")
	cache.Set("http://example.com/expired", resp1, headers1)

	// Add entry with long expiration
	resp2 := &Response{StatusCode: 200, Body: []byte("valid")}
	headers2 := http.Header{}
	headers2.Set("Cache-Control", "max-age=3600")
	cache.Set("http://example.com/valid", resp2, headers2)

	cache.Cleanup()

	// Expired entry should be removed
	_, ok := cache.Get("http://example.com/expired")
	if ok {
		entry, _ := cache.Get("http://example.com/expired")
		if !entry.IsExpired() {
			t.Error("expected entry to be expired and removed")
		}
	}

	// Valid entry should remain
	_, ok = cache.Get("http://example.com/valid")
	if !ok {
		t.Error("valid entry should still be present")
	}
}

func TestParseCacheControl(t *testing.T) {
	tests := []struct {
		value         string
		wantMaxAge    time.Duration
		wantHasMaxAge bool
		wantMustReval bool
	}{
		{"max-age=3600", 3600 * time.Second, true, false},
		{"max-age=60, must-revalidate", 60 * time.Second, true, true},
		{"no-cache, no-store", 0, false, false},
		{"public, max-age=86400", 86400 * time.Second, true, false},
		{"private, max-age=0, must-revalidate", 0, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			maxAge, hasMaxAge, mustReval := parseCacheControl(tt.value)
			if maxAge != tt.wantMaxAge {
				t.Errorf("maxAge = %v, want %v", maxAge, tt.wantMaxAge)
			}
			if hasMaxAge != tt.wantHasMaxAge {
				t.Errorf("hasMaxAge = %v, want %v", hasMaxAge, tt.wantHasMaxAge)
			}
			if mustReval != tt.wantMustReval {
				t.Errorf("mustRevalidate = %v, want %v", mustReval, tt.wantMustReval)
			}
		})
	}
}

func TestContainsDirective(t *testing.T) {
	tests := []struct {
		cacheControl string
		directive    string
		want         bool
	}{
		{"no-store", "no-store", true},
		{"no-cache, no-store", "no-store", true},
		{"max-age=3600", "no-store", false},
		{"public, max-age=3600", "public", true},
		{"", "no-store", false},
	}

	for _, tt := range tests {
		t.Run(tt.cacheControl+"_"+tt.directive, func(t *testing.T) {
			if got := containsDirective(tt.cacheControl, tt.directive); got != tt.want {
				t.Errorf("containsDirective() = %v, want %v", got, tt.want)
			}
		})
	}
}
