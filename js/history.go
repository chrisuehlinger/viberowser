// Package js provides JavaScript execution capabilities for the browser.
// This file implements the History API.
package js

import (
	"net/url"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// HistoryEntry represents a single entry in the session history.
type HistoryEntry struct {
	URL   string
	Title string // Note: title is largely ignored by browsers per spec
	State interface{}
}

// HistoryManager manages the History API and session history.
type HistoryManager struct {
	runtime     *Runtime
	entries     []HistoryEntry
	index       int // Current position in history
	baseURL     *url.URL
	documentURL *url.URL
	eventBinder *EventBinder

	// Scroll restoration mode
	scrollRestoration string // "auto" or "manual"

	mu sync.RWMutex
}

// NewHistoryManager creates a new history manager.
func NewHistoryManager(runtime *Runtime, baseURL, documentURL *url.URL, eventBinder *EventBinder) *HistoryManager {
	initialURL := "about:blank"
	if documentURL != nil {
		initialURL = documentURL.String()
	}

	hm := &HistoryManager{
		runtime:           runtime,
		entries:           make([]HistoryEntry, 0),
		index:             -1,
		baseURL:           baseURL,
		documentURL:       documentURL,
		eventBinder:       eventBinder,
		scrollRestoration: "auto",
	}

	// Add initial entry for the current document
	hm.entries = append(hm.entries, HistoryEntry{
		URL:   initialURL,
		Title: "",
		State: nil,
	})
	hm.index = 0

	return hm
}

// SetupHistory installs the history object on the window.
func (m *HistoryManager) SetupHistory() {
	vm := m.runtime.VM()
	window := vm.Get("window")
	if window == nil || goja.IsUndefined(window) {
		return
	}
	windowObj := window.ToObject(vm)
	if windowObj == nil {
		return
	}

	history := vm.NewObject()

	// history.length - returns the number of entries in the joint session history
	history.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return vm.ToValue(len(m.entries))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// history.state - returns the current state object
	history.DefineAccessorProperty("state", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.index < 0 || m.index >= len(m.entries) {
			return goja.Null()
		}
		state := m.entries[m.index].State
		if state == nil {
			return goja.Null()
		}
		return vm.ToValue(state)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// history.scrollRestoration - controls scroll position restoration
	history.DefineAccessorProperty("scrollRestoration",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return vm.ToValue(m.scrollRestoration)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			value := call.Arguments[0].String()
			if value == "auto" || value == "manual" {
				m.mu.Lock()
				m.scrollRestoration = value
				m.mu.Unlock()
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// history.pushState(state, title[, url])
	history.Set("pushState", func(call goja.FunctionCall) goja.Value {
		return m.pushState(call)
	})

	// history.replaceState(state, title[, url])
	history.Set("replaceState", func(call goja.FunctionCall) goja.Value {
		return m.replaceState(call)
	})

	// history.go(delta)
	history.Set("go", func(call goja.FunctionCall) goja.Value {
		return m.go_(call)
	})

	// history.back()
	history.Set("back", func(call goja.FunctionCall) goja.Value {
		return m.back(call)
	})

	// history.forward()
	history.Set("forward", func(call goja.FunctionCall) goja.Value {
		return m.forward(call)
	})

	// Set history on window
	windowObj.Set("history", history)

	// Also set as global for scripts that access history without window prefix
	vm.Set("history", history)
}

// pushState adds a new entry to the session history.
func (m *HistoryManager) pushState(call goja.FunctionCall) goja.Value {
	vm := m.runtime.VM()

	// Get state argument (required, can be null)
	var state interface{}
	if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
		// Clone the state using structured clone algorithm (simplified)
		state = m.cloneState(call.Arguments[0])
	}

	// Get title argument (largely ignored by browsers)
	title := ""
	if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
		title = call.Arguments[1].String()
	}

	// Get URL argument (optional)
	newURL := ""
	if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
		urlArg := call.Arguments[2].String()
		resolvedURL, err := m.resolveURL(urlArg)
		if err != nil {
			// Per spec, throw a SecurityError if the URL is not same-origin
			panic(vm.NewTypeError("Failed to execute 'pushState': Invalid URL"))
		}
		// Check same-origin
		if !m.isSameOrigin(resolvedURL) {
			panic(vm.NewTypeError("Failed to execute 'pushState': URL origin does not match document origin"))
		}
		newURL = resolvedURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current URL if not specified
	if newURL == "" {
		if m.index >= 0 && m.index < len(m.entries) {
			newURL = m.entries[m.index].URL
		} else if m.documentURL != nil {
			newURL = m.documentURL.String()
		} else {
			newURL = "about:blank"
		}
	}

	// Truncate forward history
	if m.index < len(m.entries)-1 {
		m.entries = m.entries[:m.index+1]
	}

	// Add new entry
	m.entries = append(m.entries, HistoryEntry{
		URL:   newURL,
		Title: title,
		State: state,
	})
	m.index = len(m.entries) - 1

	// Update document URL and window.location
	m.updateLocation(newURL)

	return goja.Undefined()
}

// replaceState replaces the current entry in the session history.
func (m *HistoryManager) replaceState(call goja.FunctionCall) goja.Value {
	vm := m.runtime.VM()

	// Get state argument
	var state interface{}
	if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
		state = m.cloneState(call.Arguments[0])
	}

	// Get title argument
	title := ""
	if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
		title = call.Arguments[1].String()
	}

	// Get URL argument (optional)
	newURL := ""
	if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
		urlArg := call.Arguments[2].String()
		resolvedURL, err := m.resolveURL(urlArg)
		if err != nil {
			panic(vm.NewTypeError("Failed to execute 'replaceState': Invalid URL"))
		}
		if !m.isSameOrigin(resolvedURL) {
			panic(vm.NewTypeError("Failed to execute 'replaceState': URL origin does not match document origin"))
		}
		newURL = resolvedURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current URL if not specified
	if newURL == "" {
		if m.index >= 0 && m.index < len(m.entries) {
			newURL = m.entries[m.index].URL
		} else if m.documentURL != nil {
			newURL = m.documentURL.String()
		} else {
			newURL = "about:blank"
		}
	}

	// Replace current entry
	if m.index >= 0 && m.index < len(m.entries) {
		m.entries[m.index] = HistoryEntry{
			URL:   newURL,
			Title: title,
			State: state,
		}
	}

	// Update document URL and window.location
	m.updateLocation(newURL)

	return goja.Undefined()
}

// go_ navigates to a position relative to the current entry.
func (m *HistoryManager) go_(call goja.FunctionCall) goja.Value {
	delta := 0
	if len(call.Arguments) > 0 {
		delta = int(call.Arguments[0].ToInteger())
	}

	if delta == 0 {
		// go(0) should reload the page, but we just return for now
		// since we're not doing full navigation
		return goja.Undefined()
	}

	m.mu.Lock()
	newIndex := m.index + delta

	// Bounds check
	if newIndex < 0 || newIndex >= len(m.entries) {
		m.mu.Unlock()
		return goja.Undefined()
	}

	m.index = newIndex
	entry := m.entries[newIndex]
	m.mu.Unlock()

	// Update location
	m.mu.Lock()
	m.updateLocation(entry.URL)
	m.mu.Unlock()

	// Fire popstate event asynchronously (per spec)
	// We use queueGoFunc because we need to call a Go function, not a JS callable
	state := entry.State // capture for closure
	m.runtime.eventLoop.queueGoFunc(func() {
		m.firePopStateEvent(state)
	})

	return goja.Undefined()
}

// back navigates back one entry.
func (m *HistoryManager) back(call goja.FunctionCall) goja.Value {
	return m.go_(goja.FunctionCall{
		Arguments: []goja.Value{m.runtime.VM().ToValue(-1)},
	})
}

// forward navigates forward one entry.
func (m *HistoryManager) forward(call goja.FunctionCall) goja.Value {
	return m.go_(goja.FunctionCall{
		Arguments: []goja.Value{m.runtime.VM().ToValue(1)},
	})
}

// firePopStateEvent fires a popstate event on the window.
func (m *HistoryManager) firePopStateEvent(state interface{}) {
	vm := m.runtime.VM()
	window := vm.Get("window")
	if window == nil || goja.IsUndefined(window) {
		return
	}
	windowObj := window.ToObject(vm)
	if windowObj == nil {
		return
	}

	// Create PopStateEvent
	event := m.eventBinder.CreateEvent("popstate", map[string]interface{}{
		"bubbles":    false,
		"cancelable": false,
	})

	// Set the state property
	if state != nil {
		event.Set("state", vm.ToValue(state))
	} else {
		event.Set("state", goja.Null())
	}

	// Set event target properties
	event.Set("target", windowObj)
	event.Set("currentTarget", windowObj)
	event.Set("eventPhase", int(EventPhaseAtTarget))
	event.Set("isTrusted", true)

	// Dispatch the event
	target := m.eventBinder.GetOrCreateTarget(windowObj)
	target.DispatchEvent(vm, event, EventPhaseAtTarget)
}

// updateLocation updates the document URL and window.location.
func (m *HistoryManager) updateLocation(urlStr string) {
	vm := m.runtime.VM()

	// Parse the URL for location components
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return
	}

	// Update document URL if we have a way to do so
	// (The document would need a SetURL method exposed to JS)

	// Update window.location properties
	location := vm.Get("location")
	if location == nil || goja.IsUndefined(location) {
		return
	}
	locationObj := location.ToObject(vm)
	if locationObj == nil {
		return
	}

	// Update location properties
	locationObj.Set("href", urlStr)
	locationObj.Set("protocol", parsedURL.Scheme+":")
	locationObj.Set("host", parsedURL.Host)
	locationObj.Set("hostname", parsedURL.Hostname())
	locationObj.Set("port", parsedURL.Port())
	locationObj.Set("pathname", parsedURL.Path)
	if parsedURL.Path == "" {
		locationObj.Set("pathname", "/")
	}
	locationObj.Set("search", "")
	if parsedURL.RawQuery != "" {
		locationObj.Set("search", "?"+parsedURL.RawQuery)
	}
	locationObj.Set("hash", "")
	if parsedURL.Fragment != "" {
		locationObj.Set("hash", "#"+parsedURL.Fragment)
	}

	// Update origin
	origin := parsedURL.Scheme + "://" + parsedURL.Host
	locationObj.Set("origin", origin)

	// Also update document.URL if accessible
	doc := vm.Get("document")
	if doc != nil && !goja.IsUndefined(doc) {
		docObj := doc.ToObject(vm)
		if docObj != nil {
			// Check if there's a way to update the URL
			// The Go Document should have a SetURL method
			if goDoc := docObj.Get("_goDocument"); goDoc != nil && !goja.IsUndefined(goDoc) {
				// We can't directly call SetURL here, but we can set properties
				docObj.Set("URL", urlStr)
				docObj.Set("documentURI", urlStr)
			}
		}
	}
}

// resolveURL resolves a URL relative to the base URL.
func (m *HistoryManager) resolveURL(urlStr string) (string, error) {
	if urlStr == "" {
		if m.documentURL != nil {
			return m.documentURL.String(), nil
		}
		return "about:blank", nil
	}

	// Parse the URL
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// If it's an absolute URL, return it
	if parsed.IsAbs() {
		return parsed.String(), nil
	}

	// Resolve relative to base URL
	if m.baseURL != nil {
		resolved := m.baseURL.ResolveReference(parsed)
		return resolved.String(), nil
	}

	// No base URL, try to use as-is
	return urlStr, nil
}

// isSameOrigin checks if the given URL has the same origin as the document.
func (m *HistoryManager) isSameOrigin(urlStr string) bool {
	if m.documentURL == nil {
		return true // Can't check, allow it
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Same origin means same scheme, host, and port
	return strings.EqualFold(parsed.Scheme, m.documentURL.Scheme) &&
		strings.EqualFold(parsed.Host, m.documentURL.Host)
}

// cloneState performs a simplified structured clone of the state object.
// A full implementation would use the structured clone algorithm per spec.
func (m *HistoryManager) cloneState(value goja.Value) interface{} {
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return nil
	}

	// Export the value to Go interface{}
	// This is a simplified approach - a full implementation would need
	// to properly implement the structured clone algorithm
	return value.Export()
}

// GetCurrentURL returns the current URL from the history.
func (m *HistoryManager) GetCurrentURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.index >= 0 && m.index < len(m.entries) {
		return m.entries[m.index].URL
	}
	return "about:blank"
}

// GetState returns the current state object.
func (m *HistoryManager) GetState() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.index >= 0 && m.index < len(m.entries) {
		return m.entries[m.index].State
	}
	return nil
}
