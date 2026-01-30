package network

import (
	"testing"
)

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		ref     string
		want    string
		wantErr bool
	}{
		{
			name: "absolute URL unchanged",
			base: "http://example.com/page.html",
			ref:  "https://other.com/resource.css",
			want: "https://other.com/resource.css",
		},
		{
			name: "relative path",
			base: "http://example.com/dir/page.html",
			ref:  "style.css",
			want: "http://example.com/dir/style.css",
		},
		{
			name: "relative path with dots",
			base: "http://example.com/dir/sub/page.html",
			ref:  "../style.css",
			want: "http://example.com/dir/style.css",
		},
		{
			name: "absolute path",
			base: "http://example.com/dir/page.html",
			ref:  "/css/style.css",
			want: "http://example.com/css/style.css",
		},
		{
			name: "fragment only",
			base: "http://example.com/page.html",
			ref:  "#section",
			want: "http://example.com/page.html#section",
		},
		{
			name: "data URL unchanged",
			base: "http://example.com/page.html",
			ref:  "data:text/css,body{color:red}",
			want: "data:text/css,body{color:red}",
		},
		{
			name: "javascript URL unchanged",
			base: "http://example.com/page.html",
			ref:  "javascript:void(0)",
			want: "javascript:void(0)",
		},
		{
			name: "empty reference returns base",
			base: "http://example.com/page.html",
			ref:  "",
			want: "http://example.com/page.html",
		},
		{
			name: "protocol-relative URL",
			base: "https://example.com/page.html",
			ref:  "//cdn.example.com/script.js",
			want: "https://cdn.example.com/script.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveURL(tt.base, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDataURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantType    string
		wantCharset string
		wantBase64  bool
		wantData    string
		wantErr     bool
	}{
		{
			name:        "simple text",
			url:         "data:,Hello%20World",
			wantType:    "text/plain",
			wantCharset: "US-ASCII",
			wantData:    "Hello World",
		},
		{
			name:        "text with media type",
			url:         "data:text/html,<h1>Hello</h1>",
			wantType:    "text/html",
			wantCharset: "US-ASCII",
			wantData:    "<h1>Hello</h1>",
		},
		{
			name:        "css data URL",
			url:         "data:text/css,body{color:red}",
			wantType:    "text/css",
			wantCharset: "US-ASCII",
			wantData:    "body{color:red}",
		},
		{
			name:        "base64 encoded",
			url:         "data:text/plain;base64,SGVsbG8gV29ybGQ=",
			wantType:    "text/plain",
			wantCharset: "US-ASCII",
			wantBase64:  true,
			wantData:    "Hello World",
		},
		{
			name:        "with charset",
			url:         "data:text/html;charset=utf-8,<h1>Hello</h1>",
			wantType:    "text/html",
			wantCharset: "utf-8",
			wantData:    "<h1>Hello</h1>",
		},
		{
			name:    "invalid - not data URL",
			url:     "http://example.com",
			wantErr: true,
		},
		{
			name:    "invalid - missing comma",
			url:     "data:text/plain",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDataURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDataURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.MediaType != tt.wantType {
				t.Errorf("MediaType = %v, want %v", got.MediaType, tt.wantType)
			}
			if got.Charset != tt.wantCharset {
				t.Errorf("Charset = %v, want %v", got.Charset, tt.wantCharset)
			}
			if got.Base64 != tt.wantBase64 {
				t.Errorf("Base64 = %v, want %v", got.Base64, tt.wantBase64)
			}
			if string(got.Data) != tt.wantData {
				t.Errorf("Data = %v, want %v", string(got.Data), tt.wantData)
			}
		})
	}
}

func TestIsAbsoluteURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"ftp://files.example.com", true},
		{"/path/to/file", false},
		{"../relative/path", false},
		{"file.html", false},
		{"data:text/plain,test", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := IsAbsoluteURL(tt.url); got != tt.want {
				t.Errorf("IsAbsoluteURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"HTTP://EXAMPLE.COM/Path", "http://example.com/Path"},
		{"http://example.com:80/path", "http://example.com/path"},
		{"https://example.com:443/path", "https://example.com/path"},
		{"http://example.com:8080/path", "http://example.com:8080/path"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, err := NormalizeURL(tt.url)
			if err != nil {
				t.Errorf("NormalizeURL() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOrigin(t *testing.T) {
	tests := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"http://example.com/path", "http://example.com", false},
		{"https://example.com:8080/path", "https://example.com:8080", false},
		{"/relative/path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, err := GetOrigin(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrigin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSameOrigin(t *testing.T) {
	tests := []struct {
		url1 string
		url2 string
		want bool
	}{
		{"http://example.com/path1", "http://example.com/path2", true},
		{"http://example.com", "https://example.com", false},
		{"http://example.com", "http://other.com", false},
		{"http://example.com:80", "http://example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.url1+"_"+tt.url2, func(t *testing.T) {
			if got := IsSameOrigin(tt.url1, tt.url2); got != tt.want {
				t.Errorf("IsSameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://example.com/path/to/file.html", "file.html"},
		{"http://example.com/script.js", "script.js"},
		{"http://example.com/", ""},
		{"http://example.com", ""},
		{"http://example.com/path/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := ExtractFilename(tt.url); got != tt.want {
				t.Errorf("ExtractFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractExtension(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://example.com/file.html", "html"},
		{"http://example.com/script.js", "js"},
		{"http://example.com/style.CSS", "css"},
		{"http://example.com/noextension", ""},
		{"http://example.com/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := ExtractExtension(tt.url); got != tt.want {
				t.Errorf("ExtractExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuessContentType(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://example.com/page.html", "text/html"},
		{"http://example.com/style.css", "text/css"},
		{"http://example.com/script.js", "text/javascript"},
		{"http://example.com/data.json", "application/json"},
		{"http://example.com/image.png", "image/png"},
		{"http://example.com/photo.jpg", "image/jpeg"},
		{"http://example.com/unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := GuessContentType(tt.url); got != tt.want {
				t.Errorf("GuessContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}
