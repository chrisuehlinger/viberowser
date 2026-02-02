package js

import (
	"testing"
)

func TestLocationProperties(t *testing.T) {
	runtime := NewRuntime()

	// Set initial URL
	runtime.LocationManager().SetURL("https://example.com:8080/path/to/page?query=value#section")

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "href returns full URL",
			code:     "location.href",
			expected: "https://example.com:8080/path/to/page?query=value#section",
		},
		{
			name:     "protocol returns scheme with colon",
			code:     "location.protocol",
			expected: "https:",
		},
		{
			name:     "host returns hostname:port",
			code:     "location.host",
			expected: "example.com:8080",
		},
		{
			name:     "hostname returns hostname only",
			code:     "location.hostname",
			expected: "example.com",
		},
		{
			name:     "port returns port number",
			code:     "location.port",
			expected: "8080",
		},
		{
			name:     "pathname returns path",
			code:     "location.pathname",
			expected: "/path/to/page",
		},
		{
			name:     "search returns query with ?",
			code:     "location.search",
			expected: "?query=value",
		},
		{
			name:     "hash returns fragment with #",
			code:     "location.hash",
			expected: "#section",
		},
		{
			name:     "origin returns scheme://host",
			code:     "location.origin",
			expected: "https://example.com:8080",
		},
		{
			name:     "toString returns href",
			code:     "location.toString()",
			expected: "https://example.com:8080/path/to/page?query=value#section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.Execute(tt.code)
			if err != nil {
				t.Fatalf("Failed to execute %q: %v", tt.code, err)
			}
			if result.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.String())
			}
		})
	}
}

func TestLocationMethods(t *testing.T) {
	t.Run("assign exists and is callable", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		// Check that assign is a function
		result, err := runtime.Execute("typeof location.assign")
		if err != nil {
			t.Fatalf("Failed to check assign type: %v", err)
		}
		if result.String() != "function" {
			t.Errorf("Expected assign to be a function, got %q", result.String())
		}
	})

	t.Run("replace exists and is callable", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		result, err := runtime.Execute("typeof location.replace")
		if err != nil {
			t.Fatalf("Failed to check replace type: %v", err)
		}
		if result.String() != "function" {
			t.Errorf("Expected replace to be a function, got %q", result.String())
		}
	})

	t.Run("reload exists and is callable", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		result, err := runtime.Execute("typeof location.reload")
		if err != nil {
			t.Fatalf("Failed to check reload type: %v", err)
		}
		if result.String() != "function" {
			t.Errorf("Expected reload to be a function, got %q", result.String())
		}
	})
}

func TestLocationPropertySetters(t *testing.T) {
	t.Run("setting href changes URL", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		_, err := runtime.Execute("location.href = 'https://other.com/new'")
		if err != nil {
			t.Fatalf("Failed to set href: %v", err)
		}

		result, err := runtime.Execute("location.href")
		if err != nil {
			t.Fatalf("Failed to get href: %v", err)
		}
		if result.String() != "https://other.com/new" {
			t.Errorf("Expected 'https://other.com/new', got %q", result.String())
		}
	})

	t.Run("setting pathname changes path", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/old")

		_, err := runtime.Execute("location.pathname = '/new/path'")
		if err != nil {
			t.Fatalf("Failed to set pathname: %v", err)
		}

		result, err := runtime.Execute("location.pathname")
		if err != nil {
			t.Fatalf("Failed to get pathname: %v", err)
		}
		if result.String() != "/new/path" {
			t.Errorf("Expected '/new/path', got %q", result.String())
		}
	})

	t.Run("setting search changes query", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		_, err := runtime.Execute("location.search = '?foo=bar'")
		if err != nil {
			t.Fatalf("Failed to set search: %v", err)
		}

		result, err := runtime.Execute("location.search")
		if err != nil {
			t.Fatalf("Failed to get search: %v", err)
		}
		if result.String() != "?foo=bar" {
			t.Errorf("Expected '?foo=bar', got %q", result.String())
		}
	})

	t.Run("setting hash changes fragment", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		_, err := runtime.Execute("location.hash = '#section'")
		if err != nil {
			t.Fatalf("Failed to set hash: %v", err)
		}

		result, err := runtime.Execute("location.hash")
		if err != nil {
			t.Fatalf("Failed to get hash: %v", err)
		}
		if result.String() != "#section" {
			t.Errorf("Expected '#section', got %q", result.String())
		}
	})

	t.Run("setting hostname changes host", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com:8080/path")

		_, err := runtime.Execute("location.hostname = 'newhost.com'")
		if err != nil {
			t.Fatalf("Failed to set hostname: %v", err)
		}

		// Hostname should change but port should be preserved
		result, err := runtime.Execute("location.host")
		if err != nil {
			t.Fatalf("Failed to get host: %v", err)
		}
		if result.String() != "newhost.com:8080" {
			t.Errorf("Expected 'newhost.com:8080', got %q", result.String())
		}
	})

	t.Run("setting port changes port", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com:8080/path")

		_, err := runtime.Execute("location.port = '9000'")
		if err != nil {
			t.Fatalf("Failed to set port: %v", err)
		}

		result, err := runtime.Execute("location.port")
		if err != nil {
			t.Fatalf("Failed to get port: %v", err)
		}
		if result.String() != "9000" {
			t.Errorf("Expected '9000', got %q", result.String())
		}
	})
}

func TestLocationNavigationCallback(t *testing.T) {
	t.Run("assign triggers navigation callback", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		var navigatedURL string
		var isReplace bool
		runtime.LocationManager().SetNavigationCallback(func(url string, replace bool) {
			navigatedURL = url
			isReplace = replace
		})

		_, err := runtime.Execute("location.assign('https://example.com/new')")
		if err != nil {
			t.Fatalf("Failed to call assign: %v", err)
		}

		if navigatedURL != "https://example.com/new" {
			t.Errorf("Expected navigation to 'https://example.com/new', got %q", navigatedURL)
		}
		if isReplace {
			t.Error("assign should not use replace")
		}
	})

	t.Run("replace triggers navigation callback with replace=true", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		var navigatedURL string
		var isReplace bool
		runtime.LocationManager().SetNavigationCallback(func(url string, replace bool) {
			navigatedURL = url
			isReplace = replace
		})

		_, err := runtime.Execute("location.replace('https://example.com/new')")
		if err != nil {
			t.Fatalf("Failed to call replace: %v", err)
		}

		if navigatedURL != "https://example.com/new" {
			t.Errorf("Expected navigation to 'https://example.com/new', got %q", navigatedURL)
		}
		if !isReplace {
			t.Error("replace should use replace")
		}
	})

	t.Run("setting href triggers navigation callback", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		var navigatedURL string
		runtime.LocationManager().SetNavigationCallback(func(url string, replace bool) {
			navigatedURL = url
		})

		_, err := runtime.Execute("location.href = 'https://example.com/new'")
		if err != nil {
			t.Fatalf("Failed to set href: %v", err)
		}

		if navigatedURL != "https://example.com/new" {
			t.Errorf("Expected navigation to 'https://example.com/new', got %q", navigatedURL)
		}
	})
}

func TestLocationRelativeURL(t *testing.T) {
	t.Run("assign resolves relative URLs", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/dir/page")

		_, err := runtime.Execute("location.assign('other')")
		if err != nil {
			t.Fatalf("Failed to call assign: %v", err)
		}

		result, err := runtime.Execute("location.href")
		if err != nil {
			t.Fatalf("Failed to get href: %v", err)
		}
		if result.String() != "https://example.com/dir/other" {
			t.Errorf("Expected 'https://example.com/dir/other', got %q", result.String())
		}
	})

	t.Run("href setter resolves relative URLs", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/dir/page")

		_, err := runtime.Execute("location.href = '/absolute'")
		if err != nil {
			t.Fatalf("Failed to set href: %v", err)
		}

		result, err := runtime.Execute("location.href")
		if err != nil {
			t.Fatalf("Failed to get href: %v", err)
		}
		if result.String() != "https://example.com/absolute" {
			t.Errorf("Expected 'https://example.com/absolute', got %q", result.String())
		}
	})
}

func TestLocationOrigin(t *testing.T) {
	t.Run("origin is null for about: URLs", func(t *testing.T) {
		runtime := NewRuntime()
		// Default URL is about:blank

		result, err := runtime.Execute("location.origin")
		if err != nil {
			t.Fatalf("Failed to get origin: %v", err)
		}
		if result.String() != "null" {
			t.Errorf("Expected 'null', got %q", result.String())
		}
	})

	t.Run("origin is scheme://host for http URLs", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com:8080/path")

		result, err := runtime.Execute("location.origin")
		if err != nil {
			t.Fatalf("Failed to get origin: %v", err)
		}
		if result.String() != "https://example.com:8080" {
			t.Errorf("Expected 'https://example.com:8080', got %q", result.String())
		}
	})
}

func TestLocationEmptyComponents(t *testing.T) {
	t.Run("pathname defaults to / for empty path", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com")

		result, err := runtime.Execute("location.pathname")
		if err != nil {
			t.Fatalf("Failed to get pathname: %v", err)
		}
		if result.String() != "/" {
			t.Errorf("Expected '/', got %q", result.String())
		}
	})

	t.Run("search is empty for no query", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		result, err := runtime.Execute("location.search")
		if err != nil {
			t.Fatalf("Failed to get search: %v", err)
		}
		if result.String() != "" {
			t.Errorf("Expected '', got %q", result.String())
		}
	})

	t.Run("hash is empty for no fragment", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		result, err := runtime.Execute("location.hash")
		if err != nil {
			t.Fatalf("Failed to get hash: %v", err)
		}
		if result.String() != "" {
			t.Errorf("Expected '', got %q", result.String())
		}
	})

	t.Run("port is empty for default ports", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		result, err := runtime.Execute("location.port")
		if err != nil {
			t.Fatalf("Failed to get port: %v", err)
		}
		if result.String() != "" {
			t.Errorf("Expected '', got %q", result.String())
		}
	})
}

func TestLocationWindowAndGlobal(t *testing.T) {
	t.Run("location is accessible as window.location", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/")

		result, err := runtime.Execute("window.location === location")
		if err != nil {
			t.Fatalf("Failed to compare: %v", err)
		}
		if result.String() != "true" {
			t.Errorf("Expected window.location === location to be true")
		}
	})

	t.Run("location properties accessible via window.location", func(t *testing.T) {
		runtime := NewRuntime()
		runtime.LocationManager().SetURL("https://example.com/path")

		result, err := runtime.Execute("window.location.pathname")
		if err != nil {
			t.Fatalf("Failed to get pathname: %v", err)
		}
		if result.String() != "/path" {
			t.Errorf("Expected '/path', got %q", result.String())
		}
	})
}

func TestLocationAncestorOrigins(t *testing.T) {
	t.Run("ancestorOrigins exists and has length 0", func(t *testing.T) {
		runtime := NewRuntime()

		result, err := runtime.Execute("location.ancestorOrigins.length")
		if err != nil {
			t.Fatalf("Failed to get ancestorOrigins.length: %v", err)
		}
		if result.String() != "0" {
			t.Errorf("Expected ancestorOrigins.length to be 0, got %q", result.String())
		}
	})
}
