package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadDataURL(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	resource := loader.Load(context.Background(), "data:text/css,body{color:red}", ResourceTypeStylesheet)

	if resource.Error != nil {
		t.Fatalf("Load() error = %v", resource.Error)
	}

	if resource.ContentType != "text/css" {
		t.Errorf("ContentType = %q, want %q", resource.ContentType, "text/css")
	}

	if string(resource.Content) != "body{color:red}" {
		t.Errorf("Content = %q, want %q", string(resource.Content), "body{color:red}")
	}

	if resource.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resource.StatusCode)
	}
}

func TestLoaderLoadHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("body { margin: 0; }"))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	resource := loader.LoadStylesheet(context.Background(), server.URL+"/style.css")

	if resource.Error != nil {
		t.Fatalf("LoadStylesheet() error = %v", resource.Error)
	}

	if !resource.IsSuccess() {
		t.Error("expected resource to be successful")
	}

	if resource.ContentType != "text/css" {
		t.Errorf("ContentType = %q, want %q", resource.ContentType, "text/css")
	}

	if string(resource.Content) != "body { margin: 0; }" {
		t.Errorf("Content = %q", string(resource.Content))
	}
}

func TestLoaderLoadLocal(t *testing.T) {
	// Create temp directory with test file
	tmpDir, err := os.MkdirTemp("", "loader_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test CSS file
	cssPath := filepath.Join(tmpDir, "style.css")
	if err := os.WriteFile(cssPath, []byte("div { padding: 10px; }"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client, WithLocalPath(tmpDir))

	// Load using path that maps to local file
	resource := loader.Load(context.Background(), "http://example.com/style.css", ResourceTypeStylesheet)

	if resource.Error != nil {
		t.Fatalf("Load() error = %v", resource.Error)
	}

	if string(resource.Content) != "div { padding: 10px; }" {
		t.Errorf("Content = %q", string(resource.Content))
	}
}

func TestLoaderCaching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Write([]byte("console.log('test');"))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	// First load
	resource1 := loader.LoadScript(context.Background(), server.URL+"/script.js")
	if resource1.Error != nil {
		t.Fatalf("first load error = %v", resource1.Error)
	}

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second load should be cached
	resource2 := loader.LoadScript(context.Background(), server.URL+"/script.js")
	if resource2.Error != nil {
		t.Fatalf("second load error = %v", resource2.Error)
	}

	if !resource2.Cached {
		t.Error("expected second load to be cached")
	}

	if callCount != 1 {
		t.Errorf("callCount after cache = %d, want 1", callCount)
	}

	if string(resource2.Content) != "console.log('test');" {
		t.Errorf("cached content incorrect")
	}
}

func TestLoaderRelativeURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css/style.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("body {}"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)
	loader.SetBaseURL(server.URL)

	// Load with relative URL
	resource := loader.LoadStylesheet(context.Background(), "/css/style.css")

	if resource.Error != nil {
		t.Fatalf("Load() error = %v", resource.Error)
	}

	if string(resource.Content) != "body {}" {
		t.Errorf("Content = %q", string(resource.Content))
	}
}

func TestLoaderResourceTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page.html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("body {}"))
		case "/script.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte("alert(1);"))
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0x89, 0x50, 0x4E, 0x47})
		}
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	tests := []struct {
		path        string
		loadFunc    func(context.Context, string) *Resource
		wantType    string
		wantContent string
	}{
		{"/page.html", loader.LoadDocument, "text/html", "<html></html>"},
		{"/style.css", loader.LoadStylesheet, "text/css", "body {}"},
		{"/script.js", loader.LoadScript, "application/javascript", "alert(1);"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resource := tt.loadFunc(context.Background(), server.URL+tt.path)

			if resource.Error != nil {
				t.Fatalf("Load error = %v", resource.Error)
			}

			if resource.ContentType != tt.wantType {
				t.Errorf("ContentType = %q, want %q", resource.ContentType, tt.wantType)
			}

			if string(resource.Content) != tt.wantContent {
				t.Errorf("Content = %q, want %q", string(resource.Content), tt.wantContent)
			}
		})
	}
}

func TestLoaderFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	resource := loader.Fetch(context.Background(), &FetchRequest{
		URL:    server.URL + "/api/test",
		Method: "GET",
	})

	if resource.Error != nil {
		t.Fatalf("Fetch() error = %v", resource.Error)
	}

	if resource.Type != ResourceTypeFetch {
		t.Errorf("Type = %v, want ResourceTypeFetch", resource.Type)
	}

	if string(resource.Content) != `{"status":"ok"}` {
		t.Errorf("Content = %q", string(resource.Content))
	}
}

func TestLoaderFetchNoStore(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	loader := NewLoader(client)

	// First fetch with no-store
	loader.Fetch(context.Background(), &FetchRequest{
		URL:   server.URL,
		Cache: "no-store",
	})

	// Second fetch - should not use cache
	loader.Fetch(context.Background(), &FetchRequest{
		URL:   server.URL,
		Cache: "no-store",
	})

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (no-store should bypass cache)", callCount)
	}
}

func TestResourceHelpers(t *testing.T) {
	successResource := &Resource{
		StatusCode: 200,
		Content:    []byte("test content"),
	}

	if !successResource.IsSuccess() {
		t.Error("200 response should be success")
	}

	if successResource.AsString() != "test content" {
		t.Errorf("AsString() = %q", successResource.AsString())
	}

	errorResource := &Resource{
		StatusCode: 404,
	}

	if errorResource.IsSuccess() {
		t.Error("404 response should not be success")
	}

	errResource := &Resource{
		Error: context.Canceled,
	}

	if errResource.IsSuccess() {
		t.Error("resource with error should not be success")
	}
}

func TestDocumentResourcesHelpers(t *testing.T) {
	dr := &DocumentResources{
		Stylesheets: []*Resource{
			{StatusCode: 200, Content: []byte("ok")},
			{StatusCode: 404},
			{StatusCode: 200, Content: []byte("also ok")},
		},
		Scripts: []*Resource{
			{StatusCode: 500},
			{StatusCode: 200, Content: []byte("script")},
		},
	}

	successfulCSS := dr.GetSuccessfulStylesheets()
	if len(successfulCSS) != 2 {
		t.Errorf("GetSuccessfulStylesheets() count = %d, want 2", len(successfulCSS))
	}

	successfulJS := dr.GetSuccessfulScripts()
	if len(successfulJS) != 1 {
		t.Errorf("GetSuccessfulScripts() count = %d, want 1", len(successfulJS))
	}
}

func TestLoaderClearCache(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	cache := NewCache(100)
	loader := NewLoader(client, WithCache(cache))

	// Add something to cache
	resp := &Response{StatusCode: 200, Body: []byte("test")}
	cache.Set("http://example.com/test", resp, http.Header{})

	if cache.Size() != 1 {
		t.Errorf("cache size before clear = %d, want 1", cache.Size())
	}

	loader.ClearCache()

	if cache.Size() != 0 {
		t.Errorf("cache size after clear = %d, want 0", cache.Size())
	}
}
