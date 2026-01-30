// Package network provides HTTP client and resource loading functionality.
package network

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

// Client is an HTTP client with cookie support and configurable behavior.
type Client struct {
	httpClient     *http.Client
	cookieJar      http.CookieJar
	timeout        time.Duration
	maxRedirects   int
	userAgent      string
	followRedirect bool

	mu sync.RWMutex
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
	}
}

// WithMaxRedirects sets the maximum number of redirects to follow.
func WithMaxRedirects(n int) ClientOption {
	return func(c *Client) {
		c.maxRedirects = n
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithFollowRedirect enables or disables redirect following.
func WithFollowRedirect(follow bool) ClientOption {
	return func(c *Client) {
		c.followRedirect = follow
	}
}

// NewClient creates a new HTTP client with the given options.
func NewClient(opts ...ClientOption) (*Client, error) {
	// Create cookie jar with public suffix list for proper cookie handling
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	c := &Client{
		cookieJar:      jar,
		timeout:        30 * time.Second,
		maxRedirects:   10,
		userAgent:      "Viberowser/1.0",
		followRedirect: true,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Create transport with sensible defaults
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   c.timeout,
	}

	// Configure redirect policy
	if c.followRedirect {
		c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= c.maxRedirects {
				return fmt.Errorf("stopped after %d redirects", c.maxRedirects)
			}
			return nil
		}
	} else {
		c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return c, nil
}

// Request represents an HTTP request.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    io.Reader
}

// Response represents an HTTP response.
type Response struct {
	StatusCode    int
	Status        string
	Headers       http.Header
	Body          []byte
	ContentType   string
	ContentLength int64
	URL           *url.URL // Final URL after redirects
	Cached        bool     // Whether this response was served from cache
}

// Get performs an HTTP GET request.
func (c *Client) Get(ctx context.Context, urlStr string) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodGet,
		URL:    urlStr,
	})
}

// Head performs an HTTP HEAD request.
func (c *Client) Head(ctx context.Context, urlStr string) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodHead,
		URL:    urlStr,
	})
}

// Post performs an HTTP POST request.
func (c *Client) Post(ctx context.Context, urlStr string, contentType string, body io.Reader) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPost,
		URL:    urlStr,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		Body: body,
	})
}

// Do performs an HTTP request.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	httpReq.Header.Set("User-Agent", c.userAgent)
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("Accept-Encoding", "gzip")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Set custom headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle gzip encoding
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Read body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Response{
		StatusCode:    resp.StatusCode,
		Status:        resp.Status,
		Headers:       resp.Header,
		Body:          body,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		URL:           resp.Request.URL,
	}, nil
}

// SetCookies sets cookies for a URL.
func (c *Client) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.cookieJar.SetCookies(u, cookies)
}

// Cookies returns the cookies for a URL.
func (c *Client) Cookies(u *url.URL) []*http.Cookie {
	return c.cookieJar.Cookies(u)
}

// ClearCookies clears all cookies by creating a new cookie jar.
func (c *Client) ClearCookies() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	c.cookieJar = jar
	c.httpClient.Jar = jar
	return nil
}

// ParseContentType parses a Content-Type header and returns the media type and charset.
func ParseContentType(contentType string) (mediaType string, charset string) {
	if contentType == "" {
		return "application/octet-stream", ""
	}

	// Split by semicolon
	parts := strings.Split(contentType, ";")
	mediaType = strings.TrimSpace(parts[0])

	// Look for charset parameter
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "charset=") {
			charset = strings.TrimPrefix(part[8:], "\"")
			charset = strings.TrimSuffix(charset, "\"")
			charset = strings.ToLower(charset)
			break
		}
	}

	return mediaType, charset
}

// IsHTMLContentType returns true if the content type indicates HTML.
func IsHTMLContentType(contentType string) bool {
	mediaType, _ := ParseContentType(contentType)
	mediaType = strings.ToLower(mediaType)
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

// IsCSSContentType returns true if the content type indicates CSS.
func IsCSSContentType(contentType string) bool {
	mediaType, _ := ParseContentType(contentType)
	return strings.ToLower(mediaType) == "text/css"
}

// IsJavaScriptContentType returns true if the content type indicates JavaScript.
func IsJavaScriptContentType(contentType string) bool {
	mediaType, _ := ParseContentType(contentType)
	mediaType = strings.ToLower(mediaType)
	return mediaType == "text/javascript" ||
		mediaType == "application/javascript" ||
		mediaType == "application/x-javascript"
}

// IsImageContentType returns true if the content type indicates an image.
func IsImageContentType(contentType string) bool {
	mediaType, _ := ParseContentType(contentType)
	return strings.HasPrefix(strings.ToLower(mediaType), "image/")
}
