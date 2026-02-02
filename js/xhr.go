// Package js provides JavaScript execution capabilities for the browser.
// This file implements the XMLHttpRequest API.
package js

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chrisuehlinger/viberowser/network"
	"github.com/dop251/goja"
)

// XMLHttpRequest readyState constants
const (
	XHRReadyStateUnsent          = 0
	XHRReadyStateOpened          = 1
	XHRReadyStateHeadersReceived = 2
	XHRReadyStateLoading         = 3
	XHRReadyStateDone            = 4
)

// XMLHttpRequest represents an XHR object.
type XMLHttpRequest struct {
	runtime       *Runtime
	vm            *goja.Runtime
	client        *network.Client
	jsObject      *goja.Object
	baseURL       *url.URL // Base URL for resolving relative URLs
	documentURL   *url.URL // Document URL for same-origin checks

	// Request state
	method          string
	requestURL      string
	async           bool
	requestHeaders  map[string]string
	withCredentials bool
	timeout         time.Duration

	// Response state
	readyState     int
	status         int
	statusText     string
	responseText   string
	responseXML    goja.Value
	responseURL    string
	responseType   string
	response       goja.Value
	responseHeaders http.Header

	// State flags
	sendFlag     bool
	errorFlag    bool
	uploadComplete bool

	// Event handlers
	onreadystatechange goja.Callable
	onload             goja.Callable
	onerror            goja.Callable
	onloadstart        goja.Callable
	onloadend          goja.Callable
	onprogress         goja.Callable
	onabort            goja.Callable
	ontimeout          goja.Callable

	// Upload object (stub for now)
	upload *goja.Object

	// Abort handling
	abortMu     sync.Mutex
	aborted     bool
	cancelFunc  context.CancelFunc
}

// XHRManager manages XMLHttpRequest creation and lifecycle.
type XHRManager struct {
	runtime     *Runtime
	client      *network.Client
	baseURL     *url.URL
	documentURL *url.URL
}

// NewXHRManager creates a new XHR manager.
func NewXHRManager(runtime *Runtime, baseURL, documentURL *url.URL) *XHRManager {
	client, _ := network.NewClient(
		network.WithTimeout(0), // No default timeout, let XHR timeout handle it
		network.WithFollowRedirect(true),
	)

	return &XHRManager{
		runtime:     runtime,
		client:      client,
		baseURL:     baseURL,
		documentURL: documentURL,
	}
}

// SetupXMLHttpRequest installs the XMLHttpRequest constructor on the global object.
func (m *XHRManager) SetupXMLHttpRequest() {
	vm := m.runtime.VM()

	// Create XMLHttpRequest constructor
	xhrConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		xhr := m.createXHR()
		return xhr.jsObject
	})

	// Set up the constructor's static properties
	xhrObj := xhrConstructor.ToObject(vm)

	// ReadyState constants on constructor
	xhrObj.Set("UNSENT", XHRReadyStateUnsent)
	xhrObj.Set("OPENED", XHRReadyStateOpened)
	xhrObj.Set("HEADERS_RECEIVED", XHRReadyStateHeadersReceived)
	xhrObj.Set("LOADING", XHRReadyStateLoading)
	xhrObj.Set("DONE", XHRReadyStateDone)

	vm.Set("XMLHttpRequest", xhrConstructor)
}

// createXHR creates a new XMLHttpRequest instance.
func (m *XHRManager) createXHR() *XMLHttpRequest {
	vm := m.runtime.VM()

	xhr := &XMLHttpRequest{
		runtime:        m.runtime,
		vm:             vm,
		client:         m.client,
		baseURL:        m.baseURL,
		documentURL:    m.documentURL,
		readyState:     XHRReadyStateUnsent,
		requestHeaders: make(map[string]string),
		async:          true,
		responseType:   "",
	}

	// Create the JS object
	obj := vm.NewObject()
	xhr.jsObject = obj

	// ReadyState constants on instance
	obj.Set("UNSENT", XHRReadyStateUnsent)
	obj.Set("OPENED", XHRReadyStateOpened)
	obj.Set("HEADERS_RECEIVED", XHRReadyStateHeadersReceived)
	obj.Set("LOADING", XHRReadyStateLoading)
	obj.Set("DONE", XHRReadyStateDone)

	// Define properties with getters/setters
	xhr.defineProperties()

	// Define methods
	xhr.defineMethods()

	// Create upload object (stub)
	xhr.upload = vm.NewObject()
	xhr.upload.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})
	xhr.upload.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})
	obj.Set("upload", xhr.upload)

	return xhr
}

// defineProperties sets up the properties with getters/setters.
func (xhr *XMLHttpRequest) defineProperties() {
	obj := xhr.jsObject

	// readyState (read-only)
	obj.DefineAccessorProperty("readyState",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return xhr.vm.ToValue(xhr.readyState)
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// status (read-only)
	obj.DefineAccessorProperty("status",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if xhr.readyState < XHRReadyStateHeadersReceived {
				return xhr.vm.ToValue(0)
			}
			return xhr.vm.ToValue(xhr.status)
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// statusText (read-only)
	obj.DefineAccessorProperty("statusText",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if xhr.readyState < XHRReadyStateHeadersReceived {
				return xhr.vm.ToValue("")
			}
			return xhr.vm.ToValue(xhr.statusText)
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// responseText (read-only)
	obj.DefineAccessorProperty("responseText",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if xhr.responseType != "" && xhr.responseType != "text" {
				panic(xhr.vm.NewTypeError("InvalidStateError: responseText is not available when responseType is set"))
			}
			return xhr.vm.ToValue(xhr.responseText)
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// responseXML (read-only) - stub
	obj.DefineAccessorProperty("responseXML",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if xhr.responseType != "" && xhr.responseType != "document" {
				panic(xhr.vm.NewTypeError("InvalidStateError: responseXML is not available when responseType is set"))
			}
			// TODO: Implement XML/HTML parsing
			return goja.Null()
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// response (read-only)
	obj.DefineAccessorProperty("response",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			switch xhr.responseType {
			case "", "text":
				return xhr.vm.ToValue(xhr.responseText)
			case "json":
				if xhr.readyState != XHRReadyStateDone {
					return goja.Null()
				}
				if xhr.responseText == "" {
					return goja.Null()
				}
				// Parse JSON
				result, err := xhr.vm.RunString("JSON.parse(" + jsonQuote(xhr.responseText) + ")")
				if err != nil {
					return goja.Null()
				}
				return result
			case "arraybuffer", "blob":
				// TODO: Implement ArrayBuffer/Blob
				return goja.Null()
			case "document":
				// TODO: Implement document parsing
				return goja.Null()
			default:
				return xhr.vm.ToValue(xhr.responseText)
			}
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// responseURL (read-only)
	obj.DefineAccessorProperty("responseURL",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return xhr.vm.ToValue(xhr.responseURL)
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// responseType (read-write)
	obj.DefineAccessorProperty("responseType",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return xhr.vm.ToValue(xhr.responseType)
		}),
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			if xhr.readyState >= XHRReadyStateLoading {
				panic(xhr.vm.NewTypeError("InvalidStateError: Cannot set responseType after loading has started"))
			}
			val := call.Arguments[0].String()
			switch val {
			case "", "text", "json", "arraybuffer", "blob", "document":
				xhr.responseType = val
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// timeout (read-write)
	obj.DefineAccessorProperty("timeout",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return xhr.vm.ToValue(int64(xhr.timeout / time.Millisecond))
		}),
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			ms := call.Arguments[0].ToInteger()
			xhr.timeout = time.Duration(ms) * time.Millisecond
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// withCredentials (read-write)
	obj.DefineAccessorProperty("withCredentials",
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return xhr.vm.ToValue(xhr.withCredentials)
		}),
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				return goja.Undefined()
			}
			xhr.withCredentials = call.Arguments[0].ToBoolean()
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Event handler properties
	xhr.defineEventHandler("onreadystatechange", &xhr.onreadystatechange)
	xhr.defineEventHandler("onload", &xhr.onload)
	xhr.defineEventHandler("onerror", &xhr.onerror)
	xhr.defineEventHandler("onloadstart", &xhr.onloadstart)
	xhr.defineEventHandler("onloadend", &xhr.onloadend)
	xhr.defineEventHandler("onprogress", &xhr.onprogress)
	xhr.defineEventHandler("onabort", &xhr.onabort)
	xhr.defineEventHandler("ontimeout", &xhr.ontimeout)
}

// defineEventHandler creates getter/setter for an event handler property.
func (xhr *XMLHttpRequest) defineEventHandler(name string, handler *goja.Callable) {
	xhr.jsObject.DefineAccessorProperty(name,
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if *handler == nil {
				return goja.Null()
			}
			return xhr.vm.ToValue(*handler)
		}),
		xhr.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
				*handler = nil
				return goja.Undefined()
			}
			fn, ok := goja.AssertFunction(call.Arguments[0])
			if ok {
				*handler = fn
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// defineMethods sets up the XHR methods.
func (xhr *XMLHttpRequest) defineMethods() {
	obj := xhr.jsObject

	// open(method, url [, async [, user [, password]]])
	obj.Set("open", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(xhr.vm.NewTypeError("XMLHttpRequest.open requires at least 2 arguments"))
		}

		method := strings.ToUpper(call.Arguments[0].String())
		urlStr := call.Arguments[1].String()

		async := true
		if len(call.Arguments) > 2 {
			async = call.Arguments[2].ToBoolean()
		}

		// Validate method
		validMethods := map[string]bool{
			"GET": true, "POST": true, "PUT": true, "DELETE": true,
			"HEAD": true, "OPTIONS": true, "PATCH": true,
		}
		if !validMethods[method] {
			panic(xhr.vm.NewTypeError("Invalid HTTP method: " + method))
		}

		// Resolve URL relative to base URL
		var resolvedURL *url.URL
		var err error
		if xhr.baseURL != nil {
			resolvedURL, err = xhr.baseURL.Parse(urlStr)
		} else {
			resolvedURL, err = url.Parse(urlStr)
		}
		if err != nil {
			panic(xhr.vm.NewTypeError("Invalid URL: " + urlStr))
		}

		// Reset state
		xhr.method = method
		xhr.requestURL = resolvedURL.String()
		xhr.async = async
		xhr.requestHeaders = make(map[string]string)
		xhr.sendFlag = false
		xhr.errorFlag = false
		xhr.responseText = ""
		xhr.responseURL = ""
		xhr.status = 0
		xhr.statusText = ""
		xhr.responseHeaders = nil

		// Abort any pending request
		xhr.abortMu.Lock()
		if xhr.cancelFunc != nil {
			xhr.cancelFunc()
			xhr.cancelFunc = nil
		}
		xhr.aborted = false
		xhr.abortMu.Unlock()

		// Set readyState to OPENED
		xhr.setReadyState(XHRReadyStateOpened)

		return goja.Undefined()
	})

	// setRequestHeader(name, value)
	obj.Set("setRequestHeader", func(call goja.FunctionCall) goja.Value {
		if xhr.readyState != XHRReadyStateOpened || xhr.sendFlag {
			panic(xhr.vm.NewTypeError("InvalidStateError: Cannot set request header in this state"))
		}

		if len(call.Arguments) < 2 {
			panic(xhr.vm.NewTypeError("XMLHttpRequest.setRequestHeader requires 2 arguments"))
		}

		name := call.Arguments[0].String()
		value := call.Arguments[1].String()

		// Validate header name (basic validation)
		if name == "" {
			panic(xhr.vm.NewTypeError("Invalid header name"))
		}

		// Forbidden headers that cannot be set
		forbiddenHeaders := map[string]bool{
			"accept-charset":           true,
			"accept-encoding":          true,
			"access-control-request-headers": true,
			"access-control-request-method":  true,
			"connection":               true,
			"content-length":           true,
			"cookie":                   true,
			"cookie2":                  true,
			"date":                     true,
			"dnt":                      true,
			"expect":                   true,
			"host":                     true,
			"keep-alive":               true,
			"origin":                   true,
			"referer":                  true,
			"te":                       true,
			"trailer":                  true,
			"transfer-encoding":        true,
			"upgrade":                  true,
			"via":                      true,
		}

		lowerName := strings.ToLower(name)
		if forbiddenHeaders[lowerName] || strings.HasPrefix(lowerName, "proxy-") || strings.HasPrefix(lowerName, "sec-") {
			// Silently ignore forbidden headers per spec
			return goja.Undefined()
		}

		// Combine with existing header value if present
		if existing, ok := xhr.requestHeaders[name]; ok {
			xhr.requestHeaders[name] = existing + ", " + value
		} else {
			xhr.requestHeaders[name] = value
		}

		return goja.Undefined()
	})

	// send([body])
	obj.Set("send", func(call goja.FunctionCall) goja.Value {
		if xhr.readyState != XHRReadyStateOpened || xhr.sendFlag {
			panic(xhr.vm.NewTypeError("InvalidStateError: Cannot call send in this state"))
		}

		var body io.Reader
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			bodyStr := call.Arguments[0].String()
			body = strings.NewReader(bodyStr)
		}

		xhr.sendFlag = true
		xhr.uploadComplete = body == nil

		// Fire loadstart event
		xhr.fireProgressEvent("loadstart", false, 0, 0)

		if xhr.async {
			// Asynchronous request
			go xhr.doRequest(body)
		} else {
			// Synchronous request
			xhr.doRequest(body)
		}

		return goja.Undefined()
	})

	// abort()
	obj.Set("abort", func(call goja.FunctionCall) goja.Value {
		xhr.abortMu.Lock()
		xhr.aborted = true
		if xhr.cancelFunc != nil {
			xhr.cancelFunc()
		}
		xhr.abortMu.Unlock()

		// Reset state if request is in progress
		if xhr.readyState == XHRReadyStateOpened && xhr.sendFlag ||
			xhr.readyState == XHRReadyStateHeadersReceived ||
			xhr.readyState == XHRReadyStateLoading {

			xhr.sendFlag = false
			xhr.setReadyState(XHRReadyStateDone)
			xhr.fireProgressEvent("abort", false, 0, 0)
			xhr.fireProgressEvent("loadend", false, 0, 0)
		}

		xhr.readyState = XHRReadyStateUnsent

		return goja.Undefined()
	})

	// getResponseHeader(name)
	obj.Set("getResponseHeader", func(call goja.FunctionCall) goja.Value {
		if xhr.readyState < XHRReadyStateHeadersReceived || xhr.responseHeaders == nil {
			return goja.Null()
		}

		if len(call.Arguments) < 1 {
			return goja.Null()
		}

		name := call.Arguments[0].String()
		value := xhr.responseHeaders.Get(name)
		if value == "" {
			return goja.Null()
		}
		return xhr.vm.ToValue(value)
	})

	// getAllResponseHeaders()
	obj.Set("getAllResponseHeaders", func(call goja.FunctionCall) goja.Value {
		if xhr.readyState < XHRReadyStateHeadersReceived || xhr.responseHeaders == nil {
			return xhr.vm.ToValue("")
		}

		var result strings.Builder
		for name, values := range xhr.responseHeaders {
			for _, value := range values {
				result.WriteString(strings.ToLower(name))
				result.WriteString(": ")
				result.WriteString(value)
				result.WriteString("\r\n")
			}
		}
		return xhr.vm.ToValue(result.String())
	})

	// overrideMimeType(mime) - stub
	obj.Set("overrideMimeType", func(call goja.FunctionCall) goja.Value {
		// TODO: Implement MIME type override
		return goja.Undefined()
	})

	// EventTarget methods (simplified)
	obj.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
		// For now, map to on* handlers
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		eventType := call.Arguments[0].String()
		handler, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			return goja.Undefined()
		}

		switch eventType {
		case "readystatechange":
			xhr.onreadystatechange = handler
		case "load":
			xhr.onload = handler
		case "error":
			xhr.onerror = handler
		case "loadstart":
			xhr.onloadstart = handler
		case "loadend":
			xhr.onloadend = handler
		case "progress":
			xhr.onprogress = handler
		case "abort":
			xhr.onabort = handler
		case "timeout":
			xhr.ontimeout = handler
		}
		return goja.Undefined()
	})

	obj.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		eventType := call.Arguments[0].String()

		switch eventType {
		case "readystatechange":
			xhr.onreadystatechange = nil
		case "load":
			xhr.onload = nil
		case "error":
			xhr.onerror = nil
		case "loadstart":
			xhr.onloadstart = nil
		case "loadend":
			xhr.onloadend = nil
		case "progress":
			xhr.onprogress = nil
		case "abort":
			xhr.onabort = nil
		case "timeout":
			xhr.ontimeout = nil
		}
		return goja.Undefined()
	})

	obj.Set("dispatchEvent", func(call goja.FunctionCall) goja.Value {
		return xhr.vm.ToValue(true)
	})
}

// doRequest performs the actual HTTP request.
func (xhr *XMLHttpRequest) doRequest(body io.Reader) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			xhr.handleError("NetworkError: Failed to read request body")
			return
		}
	}

	// Create context with timeout and cancellation
	ctx := context.Background()
	if xhr.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, xhr.timeout)
		defer cancel()
	}

	ctx, xhr.cancelFunc = context.WithCancel(ctx)

	// Build request
	req := &network.Request{
		Method:  xhr.method,
		URL:     xhr.requestURL,
		Headers: xhr.requestHeaders,
	}
	if len(bodyBytes) > 0 {
		req.Body = bytes.NewReader(bodyBytes)
	}

	// Perform request
	resp, err := xhr.client.Do(ctx, req)

	// Check if aborted
	xhr.abortMu.Lock()
	aborted := xhr.aborted
	xhr.abortMu.Unlock()
	if aborted {
		return
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			xhr.handleTimeout()
		} else {
			xhr.handleError("NetworkError: " + err.Error())
		}
		return
	}

	// Set response data
	xhr.status = resp.StatusCode
	xhr.statusText = http.StatusText(resp.StatusCode)
	xhr.responseHeaders = resp.Headers
	xhr.responseURL = resp.URL.String()

	// Transition through states
	if xhr.async {
		xhr.runtime.eventLoop.queueGoFunc(func() {
			xhr.setReadyState(XHRReadyStateHeadersReceived)
		})
		xhr.runtime.eventLoop.queueGoFunc(func() {
			xhr.setReadyState(XHRReadyStateLoading)
		})
		xhr.runtime.eventLoop.queueGoFunc(func() {
			xhr.responseText = string(resp.Body)
			xhr.setReadyState(XHRReadyStateDone)
			xhr.fireProgressEvent("load", true, int64(len(resp.Body)), int64(len(resp.Body)))
			xhr.fireProgressEvent("loadend", false, int64(len(resp.Body)), int64(len(resp.Body)))
		})
	} else {
		xhr.setReadyState(XHRReadyStateHeadersReceived)
		xhr.setReadyState(XHRReadyStateLoading)
		xhr.responseText = string(resp.Body)
		xhr.setReadyState(XHRReadyStateDone)
		xhr.fireProgressEvent("load", true, int64(len(resp.Body)), int64(len(resp.Body)))
		xhr.fireProgressEvent("loadend", false, int64(len(resp.Body)), int64(len(resp.Body)))
	}
}

// setReadyState sets the readyState and fires the readystatechange event.
func (xhr *XMLHttpRequest) setReadyState(state int) {
	xhr.readyState = state
	if xhr.onreadystatechange != nil {
		// Create event object
		event := xhr.vm.NewObject()
		event.Set("type", "readystatechange")
		event.Set("target", xhr.jsObject)
		event.Set("currentTarget", xhr.jsObject)

		xhr.onreadystatechange(xhr.jsObject, event)
	}
}

// handleError handles a network error.
func (xhr *XMLHttpRequest) handleError(message string) {
	xhr.errorFlag = true
	xhr.sendFlag = false

	if xhr.async {
		xhr.runtime.eventLoop.queueGoFunc(func() {
			xhr.setReadyState(XHRReadyStateDone)
			xhr.fireProgressEvent("error", false, 0, 0)
			xhr.fireProgressEvent("loadend", false, 0, 0)
		})
	} else {
		xhr.setReadyState(XHRReadyStateDone)
		xhr.fireProgressEvent("error", false, 0, 0)
		xhr.fireProgressEvent("loadend", false, 0, 0)
	}
}

// handleTimeout handles a request timeout.
func (xhr *XMLHttpRequest) handleTimeout() {
	xhr.errorFlag = true
	xhr.sendFlag = false

	if xhr.async {
		xhr.runtime.eventLoop.queueGoFunc(func() {
			xhr.setReadyState(XHRReadyStateDone)
			xhr.fireProgressEvent("timeout", false, 0, 0)
			xhr.fireProgressEvent("loadend", false, 0, 0)
		})
	} else {
		xhr.setReadyState(XHRReadyStateDone)
		xhr.fireProgressEvent("timeout", false, 0, 0)
		xhr.fireProgressEvent("loadend", false, 0, 0)
	}
}

// fireProgressEvent fires a progress event.
func (xhr *XMLHttpRequest) fireProgressEvent(eventType string, lengthComputable bool, loaded, total int64) {
	event := xhr.vm.NewObject()
	event.Set("type", eventType)
	event.Set("target", xhr.jsObject)
	event.Set("currentTarget", xhr.jsObject)
	event.Set("lengthComputable", lengthComputable)
	event.Set("loaded", loaded)
	event.Set("total", total)

	var handler goja.Callable
	switch eventType {
	case "load":
		handler = xhr.onload
	case "error":
		handler = xhr.onerror
	case "loadstart":
		handler = xhr.onloadstart
	case "loadend":
		handler = xhr.onloadend
	case "progress":
		handler = xhr.onprogress
	case "abort":
		handler = xhr.onabort
	case "timeout":
		handler = xhr.ontimeout
	}

	if handler != nil {
		handler(xhr.jsObject, event)
	}
}

// jsonQuote quotes a string for safe inclusion in JSON.parse().
func jsonQuote(s string) string {
	var result strings.Builder
	result.WriteString(`"`)
	for _, r := range s {
		switch r {
		case '"':
			result.WriteString(`\"`)
		case '\\':
			result.WriteString(`\\`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		default:
			if r < 0x20 {
				result.WriteString(`\u00`)
				result.WriteByte("0123456789abcdef"[r>>4])
				result.WriteByte("0123456789abcdef"[r&0xf])
			} else {
				result.WriteRune(r)
			}
		}
	}
	result.WriteString(`"`)
	return result.String()
}
