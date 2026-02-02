// Package js provides JavaScript execution capabilities for the browser.
// This file implements the Location interface (window.location / document.location).
package js

import (
	"net/url"
	"sync"

	"github.com/dop251/goja"
)

// LocationManager manages the window.location object and navigation.
type LocationManager struct {
	runtime *Runtime
	url     *url.URL
	mu      sync.RWMutex

	// navigationCallback is called when location changes trigger navigation
	// The callback receives the new URL and whether to replace history (vs push)
	navigationCallback func(newURL string, replace bool)
}

// NewLocationManager creates a new location manager.
func NewLocationManager(runtime *Runtime) *LocationManager {
	initialURL, _ := url.Parse("about:blank")
	return &LocationManager{
		runtime: runtime,
		url:     initialURL,
	}
}

// SetURL sets the current URL for the location object.
func (m *LocationManager) SetURL(urlStr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return
	}
	m.url = parsed
}

// GetURL returns the current URL.
func (m *LocationManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.url == nil {
		return "about:blank"
	}
	return m.url.String()
}

// SetNavigationCallback sets the callback for navigation events.
func (m *LocationManager) SetNavigationCallback(callback func(newURL string, replace bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.navigationCallback = callback
}

// SetupLocation creates and installs the location object on window.
func (m *LocationManager) SetupLocation() *goja.Object {
	vm := m.runtime.VM()
	location := vm.NewObject()

	// Define href as accessor property
	location.DefineAccessorProperty("href",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil {
				return vm.ToValue("about:blank")
			}
			return vm.ToValue(m.url.String())
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.navigate(call.Arguments[0].String(), false)
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// protocol
	location.DefineAccessorProperty("protocol",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil || m.url.Scheme == "" {
				return vm.ToValue("about:")
			}
			return vm.ToValue(m.url.Scheme + ":")
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setProtocol(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// host (hostname:port)
	location.DefineAccessorProperty("host",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil {
				return vm.ToValue("")
			}
			return vm.ToValue(m.url.Host)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setHost(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// hostname
	location.DefineAccessorProperty("hostname",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil {
				return vm.ToValue("")
			}
			return vm.ToValue(m.url.Hostname())
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setHostname(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// port
	location.DefineAccessorProperty("port",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil {
				return vm.ToValue("")
			}
			return vm.ToValue(m.url.Port())
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setPort(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// pathname
	location.DefineAccessorProperty("pathname",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil || m.url.Path == "" {
				return vm.ToValue("/")
			}
			return vm.ToValue(m.url.Path)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setPathname(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// search (including the leading ?)
	location.DefineAccessorProperty("search",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil || m.url.RawQuery == "" {
				return vm.ToValue("")
			}
			return vm.ToValue("?" + m.url.RawQuery)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setSearch(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// hash (including the leading #)
	location.DefineAccessorProperty("hash",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil || m.url.Fragment == "" {
				return vm.ToValue("")
			}
			return vm.ToValue("#" + m.url.Fragment)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			m.setHash(call.Arguments[0].String())
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// origin (read-only)
	location.DefineAccessorProperty("origin",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.url == nil {
				return vm.ToValue("null")
			}
			// For about: and javascript: URLs, origin is "null"
			if m.url.Scheme == "about" || m.url.Scheme == "javascript" {
				return vm.ToValue("null")
			}
			return vm.ToValue(m.url.Scheme + "://" + m.url.Host)
		}),
		nil, // read-only
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ancestorOrigins (read-only, returns empty array for top-level)
	ancestorOrigins := vm.NewArray()
	ancestorOrigins.DefineAccessorProperty("length",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(0)
		}),
		nil,
		goja.FLAG_FALSE, goja.FLAG_TRUE)
	location.Set("ancestorOrigins", ancestorOrigins)

	// assign(url) - navigate to a new URL
	location.Set("assign", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		m.navigate(call.Arguments[0].String(), false)
		return goja.Undefined()
	})

	// replace(url) - navigate without adding history entry
	location.Set("replace", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		m.navigate(call.Arguments[0].String(), true)
		return goja.Undefined()
	})

	// reload() - reload the current page
	location.Set("reload", func(call goja.FunctionCall) goja.Value {
		m.reload()
		return goja.Undefined()
	})

	// toString() returns href
	location.Set("toString", func(call goja.FunctionCall) goja.Value {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.url == nil {
			return vm.ToValue("about:blank")
		}
		return vm.ToValue(m.url.String())
	})

	// valueOf() returns the location object itself (for object coercion)
	location.Set("valueOf", func(call goja.FunctionCall) goja.Value {
		return location
	})

	// Install on window and global
	window := vm.Get("window")
	if window != nil && !goja.IsUndefined(window) {
		windowObj := window.ToObject(vm)
		if windowObj != nil {
			windowObj.Set("location", location)
		}
	}
	vm.Set("location", location)

	return location
}

// navigate handles navigation to a new URL.
func (m *LocationManager) navigate(urlStr string, replace bool) {
	m.mu.Lock()

	// Resolve relative URL against current URL
	var newURL *url.URL
	var err error

	parsed, err := url.Parse(urlStr)
	if err != nil {
		m.mu.Unlock()
		return
	}

	if parsed.IsAbs() {
		newURL = parsed
	} else if m.url != nil {
		newURL = m.url.ResolveReference(parsed)
	} else {
		newURL = parsed
	}

	m.url = newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	// Trigger navigation callback if set
	if callback != nil {
		callback(newURL.String(), replace)
	}
}

// reload handles page reload.
func (m *LocationManager) reload() {
	m.mu.RLock()
	currentURL := ""
	if m.url != nil {
		currentURL = m.url.String()
	}
	callback := m.navigationCallback
	m.mu.RUnlock()

	// Trigger navigation callback with current URL
	if callback != nil && currentURL != "" {
		callback(currentURL, true) // Replace since we're reloading
	}
}

// setProtocol changes the protocol/scheme.
func (m *LocationManager) setProtocol(protocol string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	// Remove trailing colon if present
	scheme := protocol
	if len(scheme) > 0 && scheme[len(scheme)-1] == ':' {
		scheme = scheme[:len(scheme)-1]
	}

	// Create new URL with changed scheme
	newURL := *m.url
	newURL.Scheme = scheme
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	// Changing protocol triggers navigation
	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setHost changes the host (hostname:port).
func (m *LocationManager) setHost(host string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	newURL := *m.url
	newURL.Host = host
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	// Changing host triggers navigation
	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setHostname changes just the hostname.
func (m *LocationManager) setHostname(hostname string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	port := m.url.Port()
	newURL := *m.url
	if port != "" {
		newURL.Host = hostname + ":" + port
	} else {
		newURL.Host = hostname
	}
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setPort changes the port.
func (m *LocationManager) setPort(port string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	hostname := m.url.Hostname()
	newURL := *m.url
	if port == "" {
		newURL.Host = hostname
	} else {
		newURL.Host = hostname + ":" + port
	}
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setPathname changes the path.
func (m *LocationManager) setPathname(pathname string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	newURL := *m.url
	newURL.Path = pathname
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setSearch changes the query string.
func (m *LocationManager) setSearch(search string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	// Remove leading ? if present
	query := search
	if len(query) > 0 && query[0] == '?' {
		query = query[1:]
	}

	newURL := *m.url
	newURL.RawQuery = query
	m.url = &newURL
	callback := m.navigationCallback
	m.mu.Unlock()

	if callback != nil {
		callback(m.url.String(), false)
	}
}

// setHash changes the fragment.
func (m *LocationManager) setHash(hash string) {
	m.mu.Lock()

	if m.url == nil {
		m.mu.Unlock()
		return
	}

	// Remove leading # if present
	fragment := hash
	if len(fragment) > 0 && fragment[0] == '#' {
		fragment = fragment[1:]
	}

	newURL := *m.url
	newURL.Fragment = fragment
	m.url = &newURL
	m.mu.Unlock()

	// Hash changes typically don't trigger full navigation
	// but may trigger hashchange event - not implemented yet
}

// UpdateFromURL updates the location from a URL string without triggering navigation.
// This is used when the browser navigates for other reasons.
func (m *LocationManager) UpdateFromURL(urlStr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return
	}
	m.url = parsed
}
