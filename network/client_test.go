package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want %v", client.timeout, 30*time.Second)
	}

	if client.maxRedirects != 10 {
		t.Errorf("default maxRedirects = %v, want %v", client.maxRedirects, 10)
	}
}

func TestClientOptions(t *testing.T) {
	client, err := NewClient(
		WithTimeout(60*time.Second),
		WithMaxRedirects(5),
		WithUserAgent("TestAgent/1.0"),
		WithFollowRedirect(false),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want %v", client.timeout, 60*time.Second)
	}

	if client.maxRedirects != 5 {
		t.Errorf("maxRedirects = %v, want %v", client.maxRedirects, 5)
	}

	if client.userAgent != "TestAgent/1.0" {
		t.Errorf("userAgent = %v, want %v", client.userAgent, "TestAgent/1.0")
	}
}

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}

	if string(resp.Body) != "Hello, World!" {
		t.Errorf("Body = %q, want %q", string(resp.Body), "Hello, World!")
	}
}

func TestClientHead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Length", "100")
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Head(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}

	// HEAD requests should have empty body
	if len(resp.Body) != 0 {
		t.Errorf("Body length = %d, want 0", len(resp.Body))
	}
}

func TestClientRedirects(t *testing.T) {
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.Write([]byte("Final destination"))
			return
		}
		redirectCount++
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer server.Close()

	client, err := NewClient(WithMaxRedirects(5))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Get(context.Background(), server.URL+"/start")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", resp.StatusCode)
	}

	if string(resp.Body) != "Final destination" {
		t.Errorf("Body = %q, want %q", string(resp.Body), "Final destination")
	}
}

func TestClientTooManyRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()

	client, err := NewClient(WithMaxRedirects(3))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Get(context.Background(), server.URL+"/loop")
	if err == nil {
		t.Error("expected error for too many redirects")
	}
}

func TestClientNoFollowRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer server.Close()

	client, err := NewClient(WithFollowRedirect(false))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if resp.StatusCode != http.StatusFound {
		t.Errorf("StatusCode = %v, want %v", resp.StatusCode, http.StatusFound)
	}
}

func TestClientCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{
				Name:  "session",
				Value: "abc123",
			})
			w.Write([]byte("Cookie set"))
			return
		}
		if r.URL.Path == "/get" {
			cookie, err := r.Cookie("session")
			if err != nil {
				w.Write([]byte("No cookie"))
				return
			}
			w.Write([]byte("Cookie: " + cookie.Value))
			return
		}
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Set cookie
	_, err = client.Get(context.Background(), server.URL+"/set")
	if err != nil {
		t.Fatalf("Get(/set) error = %v", err)
	}

	// Get cookie
	resp, err := client.Get(context.Background(), server.URL+"/get")
	if err != nil {
		t.Fatalf("Get(/get) error = %v", err)
	}

	if string(resp.Body) != "Cookie: abc123" {
		t.Errorf("Body = %q, want %q", string(resp.Body), "Cookie: abc123")
	}
}

func TestParseContentType(t *testing.T) {
	tests := []struct {
		contentType string
		wantMedia   string
		wantCharset string
	}{
		{"text/html", "text/html", ""},
		{"text/html; charset=utf-8", "text/html", "utf-8"},
		{"text/html; charset=UTF-8", "text/html", "utf-8"},
		{"text/html; charset=\"utf-8\"", "text/html", "utf-8"},
		{"application/json; charset=utf-8", "application/json", "utf-8"},
		{"", "application/octet-stream", ""},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			media, charset := ParseContentType(tt.contentType)
			if media != tt.wantMedia {
				t.Errorf("media = %q, want %q", media, tt.wantMedia)
			}
			if charset != tt.wantCharset {
				t.Errorf("charset = %q, want %q", charset, tt.wantCharset)
			}
		})
	}
}

func TestContentTypeHelpers(t *testing.T) {
	tests := []struct {
		contentType string
		isHTML      bool
		isCSS       bool
		isJS        bool
		isImage     bool
	}{
		{"text/html", true, false, false, false},
		{"text/html; charset=utf-8", true, false, false, false},
		{"application/xhtml+xml", true, false, false, false},
		{"text/css", false, true, false, false},
		{"text/javascript", false, false, true, false},
		{"application/javascript", false, false, true, false},
		{"image/png", false, false, false, true},
		{"image/jpeg", false, false, false, true},
		{"application/json", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			if got := IsHTMLContentType(tt.contentType); got != tt.isHTML {
				t.Errorf("IsHTMLContentType() = %v, want %v", got, tt.isHTML)
			}
			if got := IsCSSContentType(tt.contentType); got != tt.isCSS {
				t.Errorf("IsCSSContentType() = %v, want %v", got, tt.isCSS)
			}
			if got := IsJavaScriptContentType(tt.contentType); got != tt.isJS {
				t.Errorf("IsJavaScriptContentType() = %v, want %v", got, tt.isJS)
			}
			if got := IsImageContentType(tt.contentType); got != tt.isImage {
				t.Errorf("IsImageContentType() = %v, want %v", got, tt.isImage)
			}
		})
	}
}
