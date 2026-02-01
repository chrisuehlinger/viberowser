package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chrisuehlinger/viberowser/dom"
)

// ResourceType represents the type of a resource.
type ResourceType int

const (
	ResourceTypeUnknown ResourceType = iota
	ResourceTypeDocument
	ResourceTypeStylesheet
	ResourceTypeScript
	ResourceTypeImage
	ResourceTypeFont
	ResourceTypeXHR
	ResourceTypeFetch
)

// Resource represents a loaded resource.
type Resource struct {
	URL         string
	Type        ResourceType
	Content     []byte
	ContentType string
	Charset     string
	StatusCode  int
	Error       error
	Cached      bool
}

// IsSuccess returns true if the resource was loaded successfully.
func (r *Resource) IsSuccess() bool {
	return r.Error == nil && r.StatusCode >= 200 && r.StatusCode < 400
}

// AsString returns the resource content as a string.
func (r *Resource) AsString() string {
	return string(r.Content)
}

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// WithLocalPath sets a local path to load resources from before trying HTTP.
func WithLocalPath(path string) LoaderOption {
	return func(l *Loader) {
		l.localPath = path
	}
}

// WithCache enables caching with the specified cache.
func WithCache(cache *Cache) LoaderOption {
	return func(l *Loader) {
		l.cache = cache
	}
}

// Loader handles loading resources from HTTP or local filesystem.
type Loader struct {
	client    *Client
	cache     *Cache
	localPath string
	baseURL   string

	mu sync.RWMutex
}

// NewLoader creates a new resource loader.
func NewLoader(client *Client, opts ...LoaderOption) *Loader {
	l := &Loader{
		client: client,
		cache:  NewCache(1000),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// SetBaseURL sets the base URL for resolving relative URLs.
func (l *Loader) SetBaseURL(baseURL string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.baseURL = strings.TrimRight(baseURL, "/")
}

// GetBaseURL returns the current base URL.
func (l *Loader) GetBaseURL() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.baseURL
}

// SetLocalPath sets the local path for loading resources.
func (l *Loader) SetLocalPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.localPath = path
}

// Load loads a resource from the given URL.
func (l *Loader) Load(ctx context.Context, urlStr string, resourceType ResourceType) *Resource {
	// Handle data URLs
	if IsDataURL(urlStr) {
		return l.loadDataURL(urlStr, resourceType)
	}

	// Resolve relative URL
	l.mu.RLock()
	baseURL := l.baseURL
	localPath := l.localPath
	l.mu.RUnlock()

	if baseURL != "" && !IsAbsoluteURL(urlStr) {
		resolved, err := ResolveURL(baseURL, urlStr)
		if err != nil {
			return &Resource{
				URL:   urlStr,
				Type:  resourceType,
				Error: fmt.Errorf("failed to resolve URL: %w", err),
			}
		}
		urlStr = resolved
	}

	// Check cache first
	if entry, ok := l.cache.Get(urlStr); ok && !entry.IsExpired() {
		resp := entry.Response
		mediaType, charset := ParseContentType(resp.ContentType)
		return &Resource{
			URL:         urlStr,
			Type:        resourceType,
			Content:     resp.Body,
			ContentType: mediaType,
			Charset:     charset,
			StatusCode:  resp.StatusCode,
			Cached:      true,
		}
	}

	// Try local path first
	if localPath != "" {
		resource := l.loadFromLocal(urlStr, localPath, resourceType)
		if resource.Error == nil {
			return resource
		}
	}

	// Load from HTTP
	return l.loadFromHTTP(ctx, urlStr, resourceType)
}

// loadDataURL loads content from a data URL.
func (l *Loader) loadDataURL(urlStr string, resourceType ResourceType) *Resource {
	dataURL, err := ParseDataURL(urlStr)
	if err != nil {
		return &Resource{
			URL:   urlStr,
			Type:  resourceType,
			Error: err,
		}
	}

	return &Resource{
		URL:         urlStr,
		Type:        resourceType,
		Content:     dataURL.Data,
		ContentType: dataURL.MediaType,
		Charset:     dataURL.Charset,
		StatusCode:  200,
	}
}

// loadFromLocal attempts to load a resource from the local filesystem.
func (l *Loader) loadFromLocal(urlStr string, basePath string, resourceType ResourceType) *Resource {
	// Extract path from URL
	path := ExtractPath(urlStr)
	if path == "" {
		path = "/"
	}

	// Determine the local path to read
	var localPath string

	// Check if this is a file:// URL with an absolute path that already exists
	if strings.HasPrefix(urlStr, "file://") && filepath.IsAbs(path) {
		// For file:// URLs, try the absolute path directly first
		if _, err := os.Stat(path); err == nil {
			localPath = path
		} else {
			// Fall back to relative path within basePath
			localPath = filepath.Join(basePath, path)
		}
	} else if filepath.IsAbs(path) && strings.HasPrefix(path, basePath) {
		// Path is already absolute and within basePath
		localPath = path
	} else {
		// Build local path from relative path
		localPath = filepath.Join(basePath, path)
	}

	// Read file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return &Resource{
			URL:   urlStr,
			Type:  resourceType,
			Error: err,
		}
	}

	// Guess content type from extension
	contentType := GuessContentType(urlStr)

	return &Resource{
		URL:         urlStr,
		Type:        resourceType,
		Content:     content,
		ContentType: contentType,
		StatusCode:  200,
	}
}

// loadFromHTTP loads a resource via HTTP.
func (l *Loader) loadFromHTTP(ctx context.Context, urlStr string, resourceType ResourceType) *Resource {
	resp, err := l.client.Get(ctx, urlStr)
	if err != nil {
		return &Resource{
			URL:   urlStr,
			Type:  resourceType,
			Error: err,
		}
	}

	mediaType, charset := ParseContentType(resp.ContentType)

	resource := &Resource{
		URL:         urlStr,
		Type:        resourceType,
		Content:     resp.Body,
		ContentType: mediaType,
		Charset:     charset,
		StatusCode:  resp.StatusCode,
	}

	// Cache successful responses
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		l.cache.Set(urlStr, resp, resp.Headers)
	}

	return resource
}

// LoadDocument loads an HTML document.
func (l *Loader) LoadDocument(ctx context.Context, urlStr string) *Resource {
	return l.Load(ctx, urlStr, ResourceTypeDocument)
}

// LoadStylesheet loads a CSS stylesheet.
func (l *Loader) LoadStylesheet(ctx context.Context, urlStr string) *Resource {
	return l.Load(ctx, urlStr, ResourceTypeStylesheet)
}

// LoadScript loads a JavaScript file.
func (l *Loader) LoadScript(ctx context.Context, urlStr string) *Resource {
	return l.Load(ctx, urlStr, ResourceTypeScript)
}

// LoadImage loads an image.
func (l *Loader) LoadImage(ctx context.Context, urlStr string) *Resource {
	return l.Load(ctx, urlStr, ResourceTypeImage)
}

// LoadDocumentResources finds and loads all external resources in a document.
// This includes stylesheets, scripts, and images.
func (l *Loader) LoadDocumentResources(ctx context.Context, doc *dom.Document) (*DocumentResources, error) {
	resources := &DocumentResources{
		Stylesheets: make([]*Resource, 0),
		Scripts:     make([]*Resource, 0),
		Images:      make([]*Resource, 0),
	}

	// Find and load stylesheets
	linkElements := doc.GetElementsByTagName("link")
	for i := 0; i < linkElements.Length(); i++ {
		el := linkElements.Item(i)
		if el == nil {
			continue
		}

		rel := el.GetAttribute("rel")
		if strings.ToLower(rel) != "stylesheet" {
			continue
		}

		href := el.GetAttribute("href")
		if href == "" {
			continue
		}

		resource := l.LoadStylesheet(ctx, href)
		resources.Stylesheets = append(resources.Stylesheets, resource)
	}

	// Find and load scripts with src attribute
	scriptElements := doc.GetElementsByTagName("script")
	for i := 0; i < scriptElements.Length(); i++ {
		el := scriptElements.Item(i)
		if el == nil {
			continue
		}

		src := el.GetAttribute("src")
		if src == "" {
			continue // Inline script, skip
		}

		// Check script type
		scriptType := el.GetAttribute("type")
		if scriptType != "" && scriptType != "text/javascript" && scriptType != "application/javascript" && scriptType != "module" {
			continue // Not JavaScript
		}

		resource := l.LoadScript(ctx, src)
		resources.Scripts = append(resources.Scripts, resource)
	}

	// Find and load images
	imgElements := doc.GetElementsByTagName("img")
	for i := 0; i < imgElements.Length(); i++ {
		el := imgElements.Item(i)
		if el == nil {
			continue
		}

		src := el.GetAttribute("src")
		if src == "" {
			continue
		}

		resource := l.LoadImage(ctx, src)
		resources.Images = append(resources.Images, resource)
	}

	return resources, nil
}

// DocumentResources contains all loaded external resources for a document.
type DocumentResources struct {
	Stylesheets []*Resource
	Scripts     []*Resource
	Images      []*Resource
}

// GetSuccessfulStylesheets returns only successfully loaded stylesheets.
func (dr *DocumentResources) GetSuccessfulStylesheets() []*Resource {
	var result []*Resource
	for _, r := range dr.Stylesheets {
		if r.IsSuccess() {
			result = append(result, r)
		}
	}
	return result
}

// GetSuccessfulScripts returns only successfully loaded scripts.
func (dr *DocumentResources) GetSuccessfulScripts() []*Resource {
	var result []*Resource
	for _, r := range dr.Scripts {
		if r.IsSuccess() {
			result = append(result, r)
		}
	}
	return result
}

// FetchRequest represents a fetch API request.
type FetchRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    io.Reader
	Mode    string // cors, no-cors, same-origin
	Cache   string // default, no-store, reload, no-cache, force-cache, only-if-cached
}

// Fetch performs a fetch request.
func (l *Loader) Fetch(ctx context.Context, req *FetchRequest) *Resource {
	if req.Method == "" {
		req.Method = http.MethodGet
	}

	// Handle cache mode
	if req.Cache == "no-store" || req.Cache == "reload" {
		// Don't use cache
	} else if req.Cache == "force-cache" || req.Cache == "only-if-cached" {
		if entry, ok := l.cache.Get(req.URL); ok {
			resp := entry.Response
			mediaType, charset := ParseContentType(resp.ContentType)
			return &Resource{
				URL:         req.URL,
				Type:        ResourceTypeFetch,
				Content:     resp.Body,
				ContentType: mediaType,
				Charset:     charset,
				StatusCode:  resp.StatusCode,
				Cached:      true,
			}
		}
		if req.Cache == "only-if-cached" {
			return &Resource{
				URL:   req.URL,
				Type:  ResourceTypeFetch,
				Error: fmt.Errorf("resource not in cache"),
			}
		}
	}

	// Build request
	httpReq := &Request{
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
	}

	resp, err := l.client.Do(ctx, httpReq)
	if err != nil {
		return &Resource{
			URL:   req.URL,
			Type:  ResourceTypeFetch,
			Error: err,
		}
	}

	mediaType, charset := ParseContentType(resp.ContentType)

	resource := &Resource{
		URL:         req.URL,
		Type:        ResourceTypeFetch,
		Content:     resp.Body,
		ContentType: mediaType,
		Charset:     charset,
		StatusCode:  resp.StatusCode,
	}

	// Cache if appropriate
	if req.Cache != "no-store" && resp.StatusCode >= 200 && resp.StatusCode < 400 {
		l.cache.Set(req.URL, resp, resp.Headers)
	}

	return resource
}

// ClearCache clears the loader's cache.
func (l *Loader) ClearCache() {
	l.cache.Clear()
}
