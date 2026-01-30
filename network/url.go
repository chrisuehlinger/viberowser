package network

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// ResolveURL resolves a reference URL against a base URL.
// If ref is already absolute, it is returned as-is.
// If ref is relative, it is resolved against base.
func ResolveURL(base, ref string) (string, error) {
	// Handle empty reference
	if ref == "" {
		return base, nil
	}

	// Handle data URLs - they're always absolute
	if strings.HasPrefix(strings.ToLower(ref), "data:") {
		return ref, nil
	}

	// Handle javascript: URLs
	if strings.HasPrefix(strings.ToLower(ref), "javascript:") {
		return ref, nil
	}

	// Handle mailto: URLs
	if strings.HasPrefix(strings.ToLower(ref), "mailto:") {
		return ref, nil
	}

	// Handle fragment-only references
	if strings.HasPrefix(ref, "#") {
		baseURL, err := url.Parse(base)
		if err != nil {
			return "", fmt.Errorf("invalid base URL: %w", err)
		}
		baseURL.Fragment = ref[1:]
		return baseURL.String(), nil
	}

	// Parse the reference URL
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid reference URL: %w", err)
	}

	// If reference is already absolute, return it
	if refURL.IsAbs() {
		return refURL.String(), nil
	}

	// Parse the base URL
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Resolve the reference against the base
	resolved := baseURL.ResolveReference(refURL)
	return resolved.String(), nil
}

// NormalizeURL normalizes a URL for comparison and caching.
func NormalizeURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Lowercase scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove default ports
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = u.Host[:len(u.Host)-3]
	} else if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = u.Host[:len(u.Host)-4]
	}

	// Sort query parameters (optional, for cache key normalization)
	if u.RawQuery != "" {
		values := u.Query()
		u.RawQuery = values.Encode()
	}

	return u.String(), nil
}

// IsAbsoluteURL returns true if the URL is absolute (has a scheme).
func IsAbsoluteURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.IsAbs()
}

// IsDataURL returns true if the URL is a data URL.
func IsDataURL(urlStr string) bool {
	return strings.HasPrefix(strings.ToLower(urlStr), "data:")
}

// DataURL represents a parsed data URL.
type DataURL struct {
	MediaType string
	Charset   string
	Base64    bool
	Data      []byte
}

// ParseDataURL parses a data URL and returns its components.
// Format: data:[<mediatype>][;base64],<data>
func ParseDataURL(urlStr string) (*DataURL, error) {
	if !IsDataURL(urlStr) {
		return nil, fmt.Errorf("not a data URL")
	}

	// Remove the "data:" prefix
	content := urlStr[5:]

	// Find the comma separating metadata from data
	commaIdx := strings.Index(content, ",")
	if commaIdx == -1 {
		return nil, fmt.Errorf("invalid data URL: missing comma")
	}

	metadata := content[:commaIdx]
	data := content[commaIdx+1:]

	result := &DataURL{
		MediaType: "text/plain",
		Charset:   "US-ASCII",
	}

	// Parse metadata
	if metadata != "" {
		parts := strings.Split(metadata, ";")
		for i, part := range parts {
			if i == 0 && !strings.Contains(part, "=") && part != "base64" {
				// First part is the media type
				if part != "" {
					result.MediaType = part
				}
			} else if part == "base64" {
				result.Base64 = true
			} else if strings.HasPrefix(strings.ToLower(part), "charset=") {
				result.Charset = part[8:]
			}
		}
	}

	// Decode the data
	if result.Base64 {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
		result.Data = decoded
	} else {
		// URL-decode the data
		decoded, err := url.QueryUnescape(data)
		if err != nil {
			return nil, fmt.Errorf("failed to URL-decode data: %w", err)
		}
		result.Data = []byte(decoded)
	}

	return result, nil
}

// GetOrigin returns the origin of a URL (scheme + host + port).
func GetOrigin(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if !u.IsAbs() {
		return "", fmt.Errorf("URL is not absolute")
	}

	origin := u.Scheme + "://" + u.Host
	return origin, nil
}

// IsSameOrigin checks if two URLs have the same origin.
func IsSameOrigin(url1, url2 string) bool {
	// Normalize both URLs first
	norm1, err1 := NormalizeURL(url1)
	norm2, err2 := NormalizeURL(url2)

	if err1 != nil || err2 != nil {
		return false
	}

	origin1, err1 := GetOrigin(norm1)
	origin2, err2 := GetOrigin(norm2)

	if err1 != nil || err2 != nil {
		return false
	}

	return strings.EqualFold(origin1, origin2)
}

// ExtractPath returns the path component of a URL.
func ExtractPath(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Path
}

// ExtractFilename returns the filename from a URL path.
func ExtractFilename(urlStr string) string {
	path := ExtractPath(urlStr)
	if path == "" || path == "/" {
		return ""
	}

	// A trailing slash indicates a directory, not a file
	if strings.HasSuffix(path, "/") {
		return ""
	}

	// Find last slash
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return path
	}

	return path[lastSlash+1:]
}

// ExtractExtension returns the file extension from a URL path.
func ExtractExtension(urlStr string) string {
	filename := ExtractFilename(urlStr)
	if filename == "" {
		return ""
	}

	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 || lastDot == len(filename)-1 {
		return ""
	}

	return strings.ToLower(filename[lastDot+1:])
}

// GuessContentType attempts to guess the content type from a URL.
func GuessContentType(urlStr string) string {
	ext := ExtractExtension(urlStr)
	switch ext {
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "js", "mjs":
		return "text/javascript"
	case "json":
		return "application/json"
	case "xml":
		return "application/xml"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "webp":
		return "image/webp"
	case "ico":
		return "image/x-icon"
	case "woff":
		return "font/woff"
	case "woff2":
		return "font/woff2"
	case "ttf":
		return "font/ttf"
	case "otf":
		return "font/otf"
	default:
		return "application/octet-stream"
	}
}
