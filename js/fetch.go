// Package js provides JavaScript execution capabilities for the browser.
// This file implements the fetch API.
package js

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chrisuehlinger/viberowser/network"
	"github.com/dop251/goja"
)

// FetchManager manages fetch operations and related classes.
type FetchManager struct {
	runtime     *Runtime
	client      *network.Client
	baseURL     *url.URL
	documentURL *url.URL
}

// NewFetchManager creates a new fetch manager.
func NewFetchManager(runtime *Runtime, baseURL, documentURL *url.URL) *FetchManager {
	client, _ := network.NewClient(
		network.WithTimeout(0), // No default timeout, let AbortController handle it
		network.WithFollowRedirect(true),
	)

	return &FetchManager{
		runtime:     runtime,
		client:      client,
		baseURL:     baseURL,
		documentURL: documentURL,
	}
}

// SetupFetch installs fetch and related APIs on the global object.
func (m *FetchManager) SetupFetch() {
	m.setupHeaders()
	m.setupRequest()
	m.setupResponse()
	m.setupAbortController()
	m.setupFetchFunction()
}

// newHeaders creates a new Headers object from init data.
func (m *FetchManager) newHeaders(initData map[string][]string) *goja.Object {
	vm := m.runtime.VM()
	headers := vm.NewObject()

	headerData := initData
	if headerData == nil {
		headerData = make(map[string][]string)
	}

	// Store header data internally
	headers.Set("_data", headerData)

	// append(name, value)
	headers.Set("append", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		name := strings.ToLower(call.Arguments[0].String())
		value := call.Arguments[1].String()
		data := headers.Get("_data").Export().(map[string][]string)
		data[name] = append(data[name], value)
		return goja.Undefined()
	})

	// delete(name)
	headers.Set("delete", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := strings.ToLower(call.Arguments[0].String())
		data := headers.Get("_data").Export().(map[string][]string)
		delete(data, name)
		return goja.Undefined()
	})

	// get(name)
	headers.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := strings.ToLower(call.Arguments[0].String())
		data := headers.Get("_data").Export().(map[string][]string)
		if values, ok := data[name]; ok && len(values) > 0 {
			return vm.ToValue(strings.Join(values, ", "))
		}
		return goja.Null()
	})

	// getSetCookie()
	headers.Set("getSetCookie", func(call goja.FunctionCall) goja.Value {
		data := headers.Get("_data").Export().(map[string][]string)
		if values, ok := data["set-cookie"]; ok {
			ifaceValues := make([]interface{}, len(values))
			for i, v := range values {
				ifaceValues[i] = v
			}
			arr := vm.NewArray(ifaceValues...)
			return arr
		}
		return vm.NewArray()
	})

	// has(name)
	headers.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		name := strings.ToLower(call.Arguments[0].String())
		data := headers.Get("_data").Export().(map[string][]string)
		_, ok := data[name]
		return vm.ToValue(ok)
	})

	// set(name, value)
	headers.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		name := strings.ToLower(call.Arguments[0].String())
		value := call.Arguments[1].String()
		data := headers.Get("_data").Export().(map[string][]string)
		data[name] = []string{value}
		return goja.Undefined()
	})

	// entries() - returns iterator
	headers.Set("entries", func(call goja.FunctionCall) goja.Value {
		data := headers.Get("_data").Export().(map[string][]string)
		var entries [][]string
		for name, values := range data {
			for _, value := range values {
				entries = append(entries, []string{name, value})
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i][0] < entries[j][0]
		})
		return m.createIterator(entries)
	})

	// keys() - returns iterator
	headers.Set("keys", func(call goja.FunctionCall) goja.Value {
		data := headers.Get("_data").Export().(map[string][]string)
		var keys []string
		for name := range data {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		var entries [][]string
		for _, key := range keys {
			entries = append(entries, []string{key})
		}
		return m.createIterator(entries)
	})

	// values() - returns iterator
	headers.Set("values", func(call goja.FunctionCall) goja.Value {
		data := headers.Get("_data").Export().(map[string][]string)
		var keys []string
		for name := range data {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		var entries [][]string
		for _, key := range keys {
			for _, value := range data[key] {
				entries = append(entries, []string{value})
			}
		}
		return m.createIterator(entries)
	})

	// forEach(callback, thisArg)
	headers.Set("forEach", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}
		var thisArg goja.Value = goja.Undefined()
		if len(call.Arguments) > 1 {
			thisArg = call.Arguments[1]
		}
		data := headers.Get("_data").Export().(map[string][]string)
		var keys []string
		for name := range data {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, key := range keys {
			for _, value := range data[key] {
				callback(thisArg, vm.ToValue(value), vm.ToValue(key), headers)
			}
		}
		return goja.Undefined()
	})

	// Symbol.iterator
	headers.Set("@@iterator", headers.Get("entries"))

	return headers
}

// parseHeadersInit parses a Headers init argument into a map.
func (m *FetchManager) parseHeadersInit(initArg goja.Value) map[string][]string {
	vm := m.runtime.VM()
	headerData := make(map[string][]string)

	if goja.IsNull(initArg) || goja.IsUndefined(initArg) {
		return headerData
	}

	initObj := initArg.ToObject(vm)
	if initObj == nil {
		return headerData
	}

	// Check if it's already a Headers object
	if data := initObj.Get("_data"); data != nil && !goja.IsUndefined(data) {
		if existing, ok := data.Export().(map[string][]string); ok {
			for k, v := range existing {
				headerData[k] = append([]string{}, v...)
			}
			return headerData
		}
	}

	// Check if it's an array of [name, value] pairs
	if arrVal := initObj.Get("length"); arrVal != nil && !goja.IsUndefined(arrVal) {
		length := int(arrVal.ToInteger())
		for i := 0; i < length; i++ {
			item := initObj.Get(vm.ToValue(i).String())
			if item != nil && !goja.IsUndefined(item) {
				itemObj := item.ToObject(vm)
				if itemObj != nil {
					nameVal := itemObj.Get("0")
					valueVal := itemObj.Get("1")
					if nameVal != nil && valueVal != nil {
						name := strings.ToLower(nameVal.String())
						value := valueVal.String()
						headerData[name] = append(headerData[name], value)
					}
				}
			}
		}
	} else {
		// Object with header name/value pairs
		for _, key := range initObj.Keys() {
			val := initObj.Get(key)
			if val != nil && !goja.IsUndefined(val) {
				name := strings.ToLower(key)
				headerData[name] = append(headerData[name], val.String())
			}
		}
	}

	return headerData
}

// setupHeaders sets up the Headers class.
func (m *FetchManager) setupHeaders() {
	vm := m.runtime.VM()

	headersConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		var initArg goja.Value = goja.Undefined()
		if len(call.Arguments) > 0 {
			initArg = call.Arguments[0]
		}
		headerData := m.parseHeadersInit(initArg)
		return m.newHeaders(headerData)
	})

	vm.Set("Headers", headersConstructor)
}

// createIterator creates a JavaScript iterator from entries.
func (m *FetchManager) createIterator(entries [][]string) goja.Value {
	vm := m.runtime.VM()
	iter := vm.NewObject()
	idx := 0

	iter.Set("next", func(call goja.FunctionCall) goja.Value {
		result := vm.NewObject()
		if idx >= len(entries) {
			result.Set("done", true)
			result.Set("value", goja.Undefined())
		} else {
			result.Set("done", false)
			if len(entries[idx]) == 1 {
				result.Set("value", vm.ToValue(entries[idx][0]))
			} else {
				arr := vm.NewArray(entries[idx][0], entries[idx][1])
				result.Set("value", arr)
			}
			idx++
		}
		return result
	})

	iter.Set("@@iterator", func(call goja.FunctionCall) goja.Value {
		return iter
	})

	return iter
}

// setupRequest sets up the Request class.
func (m *FetchManager) setupRequest() {
	vm := m.runtime.VM()

	requestConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Request requires at least 1 argument"))
		}

		request := vm.NewObject()

		// Default values
		requestURL := ""
		method := "GET"
		headers := vm.NewObject()
		body := goja.Null()
		mode := "cors"
		credentials := "same-origin"
		cache := "default"
		redirect := "follow"
		referrer := "about:client"
		referrerPolicy := ""
		integrity := ""
		keepalive := false
		signal := goja.Null()

		// Parse first argument (URL or Request)
		firstArg := call.Arguments[0]
		if firstArg.ExportType().Kind().String() == "string" {
			requestURL = firstArg.String()
		} else {
			// Clone from another Request
			inputObj := firstArg.ToObject(vm)
			if inputObj != nil {
				if v := inputObj.Get("url"); v != nil && !goja.IsUndefined(v) {
					requestURL = v.String()
				}
				if v := inputObj.Get("method"); v != nil && !goja.IsUndefined(v) {
					method = v.String()
				}
				if v := inputObj.Get("headers"); v != nil && !goja.IsUndefined(v) {
					headers = v.ToObject(vm)
				}
				if v := inputObj.Get("mode"); v != nil && !goja.IsUndefined(v) {
					mode = v.String()
				}
				if v := inputObj.Get("credentials"); v != nil && !goja.IsUndefined(v) {
					credentials = v.String()
				}
				if v := inputObj.Get("redirect"); v != nil && !goja.IsUndefined(v) {
					redirect = v.String()
				}
			}
		}

		// Resolve relative URL
		if m.baseURL != nil && requestURL != "" {
			if resolved, err := m.baseURL.Parse(requestURL); err == nil {
				requestURL = resolved.String()
			}
		}

		// Parse options (second argument)
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
			opts := call.Arguments[1].ToObject(vm)
			if opts != nil {
				if v := opts.Get("method"); v != nil && !goja.IsUndefined(v) {
					method = strings.ToUpper(v.String())
				}
				if v := opts.Get("headers"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					// Create new Headers from the provided value
					headersConstructor := vm.Get("Headers")
					if headersConstructor != nil {
						if fn, ok := goja.AssertFunction(headersConstructor); ok {
							result, err := fn(goja.Undefined(), v)
							if err == nil {
								headers = result.ToObject(vm)
							}
						}
					}
				}
				if v := opts.Get("body"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					body = v
				}
				if v := opts.Get("mode"); v != nil && !goja.IsUndefined(v) {
					mode = v.String()
				}
				if v := opts.Get("credentials"); v != nil && !goja.IsUndefined(v) {
					credentials = v.String()
				}
				if v := opts.Get("cache"); v != nil && !goja.IsUndefined(v) {
					cache = v.String()
				}
				if v := opts.Get("redirect"); v != nil && !goja.IsUndefined(v) {
					redirect = v.String()
				}
				if v := opts.Get("referrer"); v != nil && !goja.IsUndefined(v) {
					referrer = v.String()
				}
				if v := opts.Get("referrerPolicy"); v != nil && !goja.IsUndefined(v) {
					referrerPolicy = v.String()
				}
				if v := opts.Get("integrity"); v != nil && !goja.IsUndefined(v) {
					integrity = v.String()
				}
				if v := opts.Get("keepalive"); v != nil && !goja.IsUndefined(v) {
					keepalive = v.ToBoolean()
				}
				if v := opts.Get("signal"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					signal = v
				}
			}
		}

		// Set request properties
		request.Set("url", requestURL)
		request.Set("method", method)
		request.Set("headers", headers)
		request.Set("mode", mode)
		request.Set("credentials", credentials)
		request.Set("cache", cache)
		request.Set("redirect", redirect)
		request.Set("referrer", referrer)
		request.Set("referrerPolicy", referrerPolicy)
		request.Set("integrity", integrity)
		request.Set("keepalive", keepalive)
		request.Set("signal", signal)

		// Store body internally
		request.Set("_body", body)
		request.Set("_bodyUsed", false)

		// bodyUsed property
		request.DefineAccessorProperty("bodyUsed",
			vm.ToValue(func(call goja.FunctionCall) goja.Value {
				return request.Get("_bodyUsed")
			}),
			nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		// Body mixin methods
		m.addBodyMixin(request)

		// clone()
		request.Set("clone", func(call goja.FunctionCall) goja.Value {
			if request.Get("_bodyUsed").ToBoolean() {
				panic(vm.NewTypeError("Cannot clone a Request with a used body"))
			}
			// Create new request with same properties
			constructor := vm.Get("Request")
			if fn, ok := goja.AssertFunction(constructor); ok {
				clone, _ := fn(goja.Undefined(), request)
				return clone
			}
			return goja.Undefined()
		})

		return request
	})

	vm.Set("Request", requestConstructor)
}

// responseOptions holds options for creating a Response.
type responseOptions struct {
	status     int
	statusText string
	headers    map[string][]string
}

// newResponse creates a new Response object.
func (m *FetchManager) newResponse(body goja.Value, opts responseOptions) *goja.Object {
	vm := m.runtime.VM()
	response := vm.NewObject()

	if opts.status == 0 {
		opts.status = 200
	}

	headers := m.newHeaders(opts.headers)

	// Set response properties
	response.Set("status", opts.status)
	response.Set("statusText", opts.statusText)
	response.Set("headers", headers)
	response.Set("ok", opts.status >= 200 && opts.status < 300)
	response.Set("redirected", false)
	response.Set("type", "default")
	response.Set("url", "")

	// Store body internally
	response.Set("_body", body)
	response.Set("_bodyUsed", false)

	// bodyUsed property
	response.DefineAccessorProperty("bodyUsed",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return response.Get("_bodyUsed")
		}),
		nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Body mixin methods
	m.addBodyMixin(response)

	// clone()
	response.Set("clone", func(call goja.FunctionCall) goja.Value {
		if response.Get("_bodyUsed").ToBoolean() {
			panic(vm.NewTypeError("Cannot clone a Response with a used body"))
		}
		// Clone headers
		headersData := m.parseHeadersInit(response.Get("headers"))
		clone := m.newResponse(response.Get("_body"), responseOptions{
			status:     int(response.Get("status").ToInteger()),
			statusText: response.Get("statusText").String(),
			headers:    headersData,
		})
		clone.Set("ok", response.Get("ok"))
		clone.Set("redirected", response.Get("redirected"))
		clone.Set("type", response.Get("type"))
		clone.Set("url", response.Get("url"))
		return clone
	})

	return response
}

// setupResponse sets up the Response class.
func (m *FetchManager) setupResponse() {
	vm := m.runtime.VM()

	responseConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Default values
		body := goja.Value(goja.Null())
		opts := responseOptions{status: 200}

		// Parse body (first argument)
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			body = call.Arguments[0]
		}

		// Parse options (second argument)
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
			optsArg := call.Arguments[1].ToObject(vm)
			if optsArg != nil {
				if v := optsArg.Get("status"); v != nil && !goja.IsUndefined(v) {
					opts.status = int(v.ToInteger())
				}
				if v := optsArg.Get("statusText"); v != nil && !goja.IsUndefined(v) {
					opts.statusText = v.String()
				}
				if v := optsArg.Get("headers"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					opts.headers = m.parseHeadersInit(v)
				}
			}
		}

		return m.newResponse(body, opts)
	})

	// Static methods
	responseObj := responseConstructor.ToObject(vm)

	// Response.error()
	responseObj.Set("error", func(call goja.FunctionCall) goja.Value {
		response := m.newResponse(goja.Null(), responseOptions{status: 0})
		response.Set("type", "error")
		response.Set("ok", false)
		return response
	})

	// Response.redirect(url, status)
	responseObj.Set("redirect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Response.redirect requires at least 1 argument"))
		}
		redirectURL := call.Arguments[0].String()
		status := 302
		if len(call.Arguments) > 1 {
			status = int(call.Arguments[1].ToInteger())
		}
		// Validate status
		if status != 301 && status != 302 && status != 303 && status != 307 && status != 308 {
			panic(vm.NewTypeError("Invalid redirect status code"))
		}
		response := m.newResponse(goja.Null(), responseOptions{
			status: status,
			headers: map[string][]string{
				"location": {redirectURL},
			},
		})
		return response
	})

	// Response.json(data, init)
	responseObj.Set("json", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Response.json requires at least 1 argument"))
		}
		data := call.Arguments[0].Export()
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			panic(vm.NewTypeError("Failed to serialize data to JSON"))
		}
		jsonStr := string(jsonBytes)

		opts := responseOptions{status: 200}
		opts.headers = map[string][]string{
			"content-type": {"application/json"},
		}

		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
			optsArg := call.Arguments[1].ToObject(vm)
			if v := optsArg.Get("status"); v != nil && !goja.IsUndefined(v) {
				opts.status = int(v.ToInteger())
			}
			if v := optsArg.Get("statusText"); v != nil && !goja.IsUndefined(v) {
				opts.statusText = v.String()
			}
			if v := optsArg.Get("headers"); v != nil && !goja.IsUndefined(v) {
				// Merge headers from options with content-type
				extraHeaders := m.parseHeadersInit(v)
				for k, v := range extraHeaders {
					opts.headers[k] = v
				}
			}
		}

		response := m.newResponse(vm.ToValue(jsonStr), opts)
		return response
	})

	vm.Set("Response", responseConstructor)
}

// addBodyMixin adds Body mixin methods to a Request or Response object.
func (m *FetchManager) addBodyMixin(obj *goja.Object) {
	vm := m.runtime.VM()

	// text() - returns Promise<string>
	obj.Set("text", func(call goja.FunctionCall) goja.Value {
		if obj.Get("_bodyUsed").ToBoolean() {
			return m.createRejectedPromise(vm.NewTypeError("Body has already been consumed"))
		}
		obj.Set("_bodyUsed", true)

		body := obj.Get("_body")
		if goja.IsNull(body) || goja.IsUndefined(body) {
			return m.createResolvedPromise(vm.ToValue(""))
		}

		// Handle string body
		if body.ExportType().Kind().String() == "string" {
			return m.createResolvedPromise(body)
		}

		// Handle ArrayBuffer or other typed data
		return m.createResolvedPromise(vm.ToValue(body.String()))
	})

	// json() - returns Promise<any>
	obj.Set("json", func(call goja.FunctionCall) goja.Value {
		if obj.Get("_bodyUsed").ToBoolean() {
			return m.createRejectedPromise(vm.NewTypeError("Body has already been consumed"))
		}
		obj.Set("_bodyUsed", true)

		body := obj.Get("_body")
		if goja.IsNull(body) || goja.IsUndefined(body) {
			syntaxErr, _ := vm.RunString("new SyntaxError('Unexpected end of JSON input')")
			return m.createRejectedPromise(syntaxErr)
		}

		bodyStr := body.String()
		result, err := vm.RunString("JSON.parse(" + jsonQuote(bodyStr) + ")")
		if err != nil {
			syntaxErr, _ := vm.RunString("new SyntaxError('Invalid JSON')")
			return m.createRejectedPromise(syntaxErr)
		}
		return m.createResolvedPromise(result)
	})

	// blob() - returns Promise<Blob> (stub - Blob not implemented)
	obj.Set("blob", func(call goja.FunctionCall) goja.Value {
		if obj.Get("_bodyUsed").ToBoolean() {
			return m.createRejectedPromise(vm.NewTypeError("Body has already been consumed"))
		}
		obj.Set("_bodyUsed", true)

		// For now, return a simple object representing the blob
		body := obj.Get("_body")
		blob := vm.NewObject()
		if goja.IsNull(body) || goja.IsUndefined(body) {
			blob.Set("size", 0)
			blob.Set("type", "")
		} else {
			bodyStr := body.String()
			blob.Set("size", len(bodyStr))
			blob.Set("type", "")
		}
		blob.Set("text", func(call goja.FunctionCall) goja.Value {
			if goja.IsNull(body) || goja.IsUndefined(body) {
				return m.createResolvedPromise(vm.ToValue(""))
			}
			return m.createResolvedPromise(vm.ToValue(body.String()))
		})
		return m.createResolvedPromise(blob)
	})

	// arrayBuffer() - returns Promise<ArrayBuffer> (stub)
	obj.Set("arrayBuffer", func(call goja.FunctionCall) goja.Value {
		if obj.Get("_bodyUsed").ToBoolean() {
			return m.createRejectedPromise(vm.NewTypeError("Body has already been consumed"))
		}
		obj.Set("_bodyUsed", true)

		body := obj.Get("_body")
		if goja.IsNull(body) || goja.IsUndefined(body) {
			// Return empty ArrayBuffer
			result, _ := vm.RunString("new ArrayBuffer(0)")
			return m.createResolvedPromise(result)
		}

		bodyStr := body.String()
		// Create Uint8Array from string bytes and get its buffer
		result, err := vm.RunString("(function(s) { var arr = new Uint8Array(s.length); for (var i = 0; i < s.length; i++) arr[i] = s.charCodeAt(i); return arr.buffer; })")
		if err != nil {
			return m.createRejectedPromise(vm.NewTypeError("Failed to create ArrayBuffer"))
		}
		if fn, ok := goja.AssertFunction(result); ok {
			ab, _ := fn(goja.Undefined(), vm.ToValue(bodyStr))
			return m.createResolvedPromise(ab)
		}
		return m.createRejectedPromise(vm.NewTypeError("Failed to create ArrayBuffer"))
	})

	// formData() - returns Promise<FormData> (stub)
	obj.Set("formData", func(call goja.FunctionCall) goja.Value {
		if obj.Get("_bodyUsed").ToBoolean() {
			return m.createRejectedPromise(vm.NewTypeError("Body has already been consumed"))
		}
		obj.Set("_bodyUsed", true)

		// FormData parsing is complex - return empty FormData for now
		formData := vm.NewObject()
		formData.Set("_data", make(map[string][]string))
		formData.Set("append", func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
		formData.Set("delete", func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
		formData.Set("get", func(call goja.FunctionCall) goja.Value { return goja.Null() })
		formData.Set("getAll", func(call goja.FunctionCall) goja.Value { return vm.NewArray() })
		formData.Set("has", func(call goja.FunctionCall) goja.Value { return vm.ToValue(false) })
		formData.Set("set", func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
		return m.createResolvedPromise(formData)
	})
}

// createResolvedPromise creates a Promise that resolves with the given value.
func (m *FetchManager) createResolvedPromise(value goja.Value) goja.Value {
	vm := m.runtime.VM()
	promiseVal := vm.Get("Promise")
	promiseObj := promiseVal.ToObject(vm)
	resolveMethod, _ := goja.AssertFunction(promiseObj.Get("resolve"))
	result, _ := resolveMethod(promiseObj, value)
	return result
}

// createRejectedPromise creates a Promise that rejects with the given error.
func (m *FetchManager) createRejectedPromise(err goja.Value) goja.Value {
	vm := m.runtime.VM()
	promiseVal := vm.Get("Promise")
	promiseObj := promiseVal.ToObject(vm)
	rejectMethod, _ := goja.AssertFunction(promiseObj.Get("reject"))
	result, _ := rejectMethod(promiseObj, err)
	return result
}

// setupAbortController sets up AbortController and AbortSignal.
func (m *FetchManager) setupAbortController() {
	vm := m.runtime.VM()

	// AbortSignal class
	abortSignalConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		signal := vm.NewObject()
		signal.Set("aborted", false)
		signal.Set("reason", goja.Undefined())
		signal.Set("_listeners", make([]goja.Callable, 0))

		// onabort event handler
		var onabort goja.Callable
		signal.DefineAccessorProperty("onabort",
			vm.ToValue(func(call goja.FunctionCall) goja.Value {
				if onabort == nil {
					return goja.Null()
				}
				return vm.ToValue(onabort)
			}),
			vm.ToValue(func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
					onabort = nil
					return goja.Undefined()
				}
				fn, ok := goja.AssertFunction(call.Arguments[0])
				if ok {
					onabort = fn
				}
				return goja.Undefined()
			}),
			goja.FLAG_FALSE, goja.FLAG_TRUE)

		signal.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				return goja.Undefined()
			}
			eventType := call.Arguments[0].String()
			if eventType != "abort" {
				return goja.Undefined()
			}
			callback, ok := goja.AssertFunction(call.Arguments[1])
			if !ok {
				return goja.Undefined()
			}
			listeners := signal.Get("_listeners").Export().([]goja.Callable)
			listeners = append(listeners, callback)
			signal.Set("_listeners", listeners)
			return goja.Undefined()
		})

		signal.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
			// Simplified - just return undefined
			return goja.Undefined()
		})

		signal.Set("throwIfAborted", func(call goja.FunctionCall) goja.Value {
			if signal.Get("aborted").ToBoolean() {
				reason := signal.Get("reason")
				if goja.IsUndefined(reason) {
					panic(vm.NewGoError(abortError{message: "signal is aborted without reason"}))
				}
				panic(reason.Export())
			}
			return goja.Undefined()
		})

		// Internal method to trigger abort
		signal.Set("_abort", func(reason goja.Value) {
			if signal.Get("aborted").ToBoolean() {
				return
			}
			signal.Set("aborted", true)
			if goja.IsUndefined(reason) {
				reason = vm.NewGoError(abortError{message: "The operation was aborted."})
			}
			signal.Set("reason", reason)

			// Fire abort event
			event := vm.NewObject()
			event.Set("type", "abort")
			event.Set("target", signal)

			if onabort != nil {
				onabort(signal, event)
			}

			listeners := signal.Get("_listeners").Export().([]goja.Callable)
			for _, listener := range listeners {
				listener(signal, event)
			}
		})

		return signal
	})

	// AbortSignal static methods
	signalObj := abortSignalConstructor.ToObject(vm)

	// AbortSignal.abort(reason)
	signalObj.Set("abort", func(call goja.FunctionCall) goja.Value {
		if fn, ok := goja.AssertFunction(abortSignalConstructor); ok {
			signal, _ := fn(goja.Undefined())
			signalObj := signal.ToObject(vm)
			signalObj.Set("aborted", true)
			if len(call.Arguments) > 0 {
				signalObj.Set("reason", call.Arguments[0])
			} else {
				signalObj.Set("reason", vm.NewGoError(abortError{message: "The operation was aborted."}))
			}
			return signal
		}
		return goja.Undefined()
	})

	// AbortSignal.timeout(ms)
	signalObj.Set("timeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("AbortSignal.timeout requires 1 argument"))
		}
		ms := call.Arguments[0].ToInteger()

		if fn, ok := goja.AssertFunction(abortSignalConstructor); ok {
			signal, _ := fn(goja.Undefined())
			signalObj := signal.ToObject(vm)

			// Schedule abort after timeout
			go func() {
				time.Sleep(time.Duration(ms) * time.Millisecond)
				m.runtime.eventLoop.queueGoFunc(func() {
					if abortFn := signalObj.Get("_abort"); abortFn != nil {
						if fn, ok := goja.AssertFunction(abortFn); ok {
							fn(signalObj, vm.NewGoError(timeoutError{message: "The operation timed out."}))
						}
					}
				})
			}()

			return signal
		}
		return goja.Undefined()
	})

	// AbortSignal.any(signals)
	signalObj.Set("any", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("AbortSignal.any requires 1 argument"))
		}

		if fn, ok := goja.AssertFunction(abortSignalConstructor); ok {
			signal, _ := fn(goja.Undefined())
			signalObj := signal.ToObject(vm)

			signals := call.Arguments[0].ToObject(vm)
			length := int(signals.Get("length").ToInteger())

			for i := 0; i < length; i++ {
				s := signals.Get(vm.ToValue(i).String()).ToObject(vm)
				if s.Get("aborted").ToBoolean() {
					signalObj.Set("aborted", true)
					signalObj.Set("reason", s.Get("reason"))
					return signal
				}

				// Add listener to abort this signal when any input signal aborts
				addListener := s.Get("addEventListener")
				if addListenerFn, ok := goja.AssertFunction(addListener); ok {
					addListenerFn(s, vm.ToValue("abort"), vm.ToValue(func(call goja.FunctionCall) goja.Value {
						if abortFn := signalObj.Get("_abort"); abortFn != nil {
							if fn, ok := goja.AssertFunction(abortFn); ok {
								fn(signalObj, s.Get("reason"))
							}
						}
						return goja.Undefined()
					}))
				}
			}

			return signal
		}
		return goja.Undefined()
	})

	vm.Set("AbortSignal", abortSignalConstructor)

	// AbortController class
	abortControllerConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		controller := vm.NewObject()

		// Create associated signal
		if fn, ok := goja.AssertFunction(abortSignalConstructor); ok {
			signal, _ := fn(goja.Undefined())
			controller.Set("signal", signal)
		}

		// abort(reason)
		controller.Set("abort", func(call goja.FunctionCall) goja.Value {
			signal := controller.Get("signal").ToObject(vm)
			if signal.Get("aborted").ToBoolean() {
				return goja.Undefined()
			}

			var reason goja.Value = goja.Undefined()
			if len(call.Arguments) > 0 {
				reason = call.Arguments[0]
			}

			if abortFn := signal.Get("_abort"); abortFn != nil {
				if fn, ok := goja.AssertFunction(abortFn); ok {
					fn(signal, reason)
				}
			}
			return goja.Undefined()
		})

		return controller
	})

	vm.Set("AbortController", abortControllerConstructor)
}

// abortError represents an AbortError DOMException.
type abortError struct {
	message string
}

func (e abortError) Error() string {
	return e.message
}

// timeoutError represents a TimeoutError DOMException.
type timeoutError struct {
	message string
}

func (e timeoutError) Error() string {
	return e.message
}

// setupFetchFunction sets up the global fetch function.
func (m *FetchManager) setupFetchFunction() {
	vm := m.runtime.VM()

	vm.Set("fetch", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return m.createRejectedPromise(vm.NewTypeError("fetch requires at least 1 argument"))
		}

		// Parse arguments
		var requestURL string
		var method = "GET"
		var headers = make(map[string]string)
		var bodyStr string
		var redirect = "follow"
		var signal goja.Value = goja.Null()

		firstArg := call.Arguments[0]
		if firstArg.ExportType().Kind().String() == "string" {
			requestURL = firstArg.String()
		} else {
			// Request object
			reqObj := firstArg.ToObject(vm)
			if reqObj != nil {
				if v := reqObj.Get("url"); v != nil && !goja.IsUndefined(v) {
					requestURL = v.String()
				}
				if v := reqObj.Get("method"); v != nil && !goja.IsUndefined(v) {
					method = v.String()
				}
				if v := reqObj.Get("headers"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					headersObj := v.ToObject(vm)
					if headersObj != nil {
						if data := headersObj.Get("_data"); data != nil && !goja.IsUndefined(data) {
							headerData := data.Export().(map[string][]string)
							for name, values := range headerData {
								headers[name] = strings.Join(values, ", ")
							}
						}
					}
				}
				if v := reqObj.Get("_body"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					bodyStr = v.String()
				}
				if v := reqObj.Get("redirect"); v != nil && !goja.IsUndefined(v) {
					redirect = v.String()
				}
				if v := reqObj.Get("signal"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					signal = v
				}
			}
		}

		// Parse options (second argument)
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
			opts := call.Arguments[1].ToObject(vm)
			if opts != nil {
				if v := opts.Get("method"); v != nil && !goja.IsUndefined(v) {
					method = strings.ToUpper(v.String())
				}
				if v := opts.Get("headers"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					headersArg := v.ToObject(vm)
					if headersArg != nil {
						// Check if it's a Headers object
						if data := headersArg.Get("_data"); data != nil && !goja.IsUndefined(data) {
							headerData := data.Export().(map[string][]string)
							for name, values := range headerData {
								headers[name] = strings.Join(values, ", ")
							}
						} else {
							// Plain object
							for _, key := range headersArg.Keys() {
								val := headersArg.Get(key)
								if val != nil && !goja.IsUndefined(val) {
									headers[strings.ToLower(key)] = val.String()
								}
							}
						}
					}
				}
				if v := opts.Get("body"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					bodyStr = v.String()
				}
				if v := opts.Get("redirect"); v != nil && !goja.IsUndefined(v) {
					redirect = v.String()
				}
				if v := opts.Get("signal"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
					signal = v
				}
			}
		}

		// Resolve relative URL
		if m.baseURL != nil && requestURL != "" {
			if resolved, err := m.baseURL.Parse(requestURL); err == nil {
				requestURL = resolved.String()
			}
		}

		// Validate URL
		parsedURL, err := url.Parse(requestURL)
		if err != nil {
			return m.createRejectedPromise(vm.NewTypeError("Invalid URL: " + requestURL))
		}

		// Create Promise using JavaScript to properly use 'new'
		// We need to store the resolve/reject functions so we can call them later
		vm.Set("__fetchResolve", nil)
		vm.Set("__fetchReject", nil)
		promise, err := vm.RunString(`
			(function() {
				var p = new Promise(function(resolve, reject) {
					__fetchResolve = resolve;
					__fetchReject = reject;
				});
				return p;
			})()
		`)
		if err != nil {
			return m.createRejectedPromise(vm.NewTypeError("Failed to create Promise: " + err.Error()))
		}

		resolvePromise, _ := goja.AssertFunction(vm.Get("__fetchResolve"))
		rejectPromise, _ := goja.AssertFunction(vm.Get("__fetchReject"))

		// Clean up global temporaries
		vm.Set("__fetchResolve", goja.Undefined())
		vm.Set("__fetchReject", goja.Undefined())

		// Check if already aborted
		if !goja.IsNull(signal) && !goja.IsUndefined(signal) {
			signalObj := signal.ToObject(vm)
			if signalObj.Get("aborted").ToBoolean() {
				reason := signalObj.Get("reason")
				if goja.IsUndefined(reason) {
					reason = vm.NewGoError(abortError{message: "The operation was aborted."})
				}
				rejectPromise(goja.Undefined(), reason)
				return promise
			}
		}

		// Create context for cancellation
		ctx := context.Background()
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithCancel(ctx)

		// Track abort signal
		var aborted bool
		var abortMu sync.Mutex
		var abortReason goja.Value

		if !goja.IsNull(signal) && !goja.IsUndefined(signal) {
			signalObj := signal.ToObject(vm)
			addListener := signalObj.Get("addEventListener")
			if addListenerFn, ok := goja.AssertFunction(addListener); ok {
				addListenerFn(signalObj, vm.ToValue("abort"), vm.ToValue(func(call goja.FunctionCall) goja.Value {
					abortMu.Lock()
					aborted = true
					abortReason = signalObj.Get("reason")
					abortMu.Unlock()
					cancelFunc()
					return goja.Undefined()
				}))
			}
		}

		// Perform request asynchronously
		go func() {
			defer cancelFunc()

			// Build request
			var body io.Reader
			if bodyStr != "" {
				body = strings.NewReader(bodyStr)
			}

			req := &network.Request{
				Method:  method,
				URL:     requestURL,
				Headers: headers,
				Body:    body,
			}

			// Configure redirect handling
			client := m.client
			if redirect == "manual" || redirect == "error" {
				// Create a new client that doesn't follow redirects
				client, _ = network.NewClient(
					network.WithTimeout(0),
					network.WithFollowRedirect(false),
				)
			}

			// Perform request
			resp, err := client.Do(ctx, req)

			// Check if aborted
			abortMu.Lock()
			wasAborted := aborted
			reason := abortReason
			abortMu.Unlock()

			if wasAborted {
				m.runtime.eventLoop.queueGoFunc(func() {
					if goja.IsUndefined(reason) || goja.IsNull(reason) {
						reason = vm.NewGoError(abortError{message: "The operation was aborted."})
					}
					rejectPromise(goja.Undefined(), reason)
				})
				return
			}

			if err != nil {
				m.runtime.eventLoop.queueGoFunc(func() {
					rejectPromise(goja.Undefined(), vm.NewTypeError("Network error: "+err.Error()))
				})
				return
			}

			// Handle redirect modes
			if redirect == "error" && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
				m.runtime.eventLoop.queueGoFunc(func() {
					rejectPromise(goja.Undefined(), vm.NewTypeError("Redirect not allowed"))
				})
				return
			}

			// Create Response object
			m.runtime.eventLoop.queueGoFunc(func() {
				// Convert http.Header to map[string][]string
				headersMap := make(map[string][]string)
				for name, values := range resp.Headers {
					headersMap[strings.ToLower(name)] = values
				}

				// Create response using helper
				response := m.newResponse(vm.ToValue(string(resp.Body)), responseOptions{
					status:     resp.StatusCode,
					statusText: http.StatusText(resp.StatusCode),
					headers:    headersMap,
				})

				response.Set("url", parsedURL.String())
				if resp.URL != nil {
					response.Set("url", resp.URL.String())
					// Check if redirected
					if resp.URL.String() != requestURL {
						response.Set("redirected", true)
					}
				}
				response.Set("type", "basic") // Default for same-origin requests

				resolvePromise(goja.Undefined(), response)
			})
		}()

		return promise
	})
}

// FetchRequest holds the state for a fetch operation.
type FetchRequest struct {
	manager      *FetchManager
	url          string
	method       string
	headers      map[string]string
	body         []byte
	redirect     string
	signal       goja.Value
	cancelFunc   context.CancelFunc
	aborted      bool
	abortMu      sync.Mutex
}
