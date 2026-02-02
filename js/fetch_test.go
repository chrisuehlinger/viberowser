package js

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestFetchBasic(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test that fetch exists
	result, err := runtime.Execute("typeof fetch")
	if err != nil {
		t.Fatalf("Failed to check fetch type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected fetch to be a function, got %s", result.String())
	}

	// Test Headers class
	result, err = runtime.Execute("typeof Headers")
	if err != nil {
		t.Fatalf("Failed to check Headers type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected Headers to be a function, got %s", result.String())
	}

	// Test Request class
	result, err = runtime.Execute("typeof Request")
	if err != nil {
		t.Fatalf("Failed to check Request type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected Request to be a function, got %s", result.String())
	}

	// Test Response class
	result, err = runtime.Execute("typeof Response")
	if err != nil {
		t.Fatalf("Failed to check Response type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected Response to be a function, got %s", result.String())
	}

	// Test AbortController class
	result, err = runtime.Execute("typeof AbortController")
	if err != nil {
		t.Fatalf("Failed to check AbortController type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected AbortController to be a function, got %s", result.String())
	}
}

func TestHeadersClass(t *testing.T) {
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL("http://localhost/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test Headers constructor with object
	_, err := runtime.Execute(`
		var h = new Headers({'Content-Type': 'application/json', 'X-Custom': 'value'});
	`)
	if err != nil {
		t.Fatalf("Failed to create Headers with object: %v", err)
	}

	// Test get()
	result, err := runtime.Execute("h.get('content-type')")
	if err != nil {
		t.Fatalf("Failed to get header: %v", err)
	}
	if result.String() != "application/json" {
		t.Errorf("Expected 'application/json', got '%s'", result.String())
	}

	// Test has()
	result, err = runtime.Execute("h.has('x-custom')")
	if err != nil {
		t.Fatalf("Failed to check header: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected has('x-custom') to be true")
	}

	// Test set()
	_, err = runtime.Execute("h.set('x-custom', 'new-value')")
	if err != nil {
		t.Fatalf("Failed to set header: %v", err)
	}

	result, err = runtime.Execute("h.get('x-custom')")
	if err != nil {
		t.Fatalf("Failed to get header after set: %v", err)
	}
	if result.String() != "new-value" {
		t.Errorf("Expected 'new-value', got '%s'", result.String())
	}

	// Test append()
	_, err = runtime.Execute("h.append('x-custom', 'appended')")
	if err != nil {
		t.Fatalf("Failed to append header: %v", err)
	}

	result, err = runtime.Execute("h.get('x-custom')")
	if err != nil {
		t.Fatalf("Failed to get header after append: %v", err)
	}
	if result.String() != "new-value, appended" {
		t.Errorf("Expected 'new-value, appended', got '%s'", result.String())
	}

	// Test delete()
	_, err = runtime.Execute("h.delete('x-custom')")
	if err != nil {
		t.Fatalf("Failed to delete header: %v", err)
	}

	result, err = runtime.Execute("h.has('x-custom')")
	if err != nil {
		t.Fatalf("Failed to check header after delete: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected has('x-custom') to be false after delete")
	}
}

func TestRequestClass(t *testing.T) {
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL("http://localhost/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test Request constructor with URL
	result, err := runtime.Execute(`
		var req = new Request('http://example.com/api');
		req.url;
	`)
	if err != nil {
		t.Fatalf("Failed to create Request: %v", err)
	}
	if result.String() != "http://example.com/api" {
		t.Errorf("Expected 'http://example.com/api', got '%s'", result.String())
	}

	// Test default method
	result, err = runtime.Execute("req.method")
	if err != nil {
		t.Fatalf("Failed to get method: %v", err)
	}
	if result.String() != "GET" {
		t.Errorf("Expected 'GET', got '%s'", result.String())
	}

	// Test Request with options
	result, err = runtime.Execute(`
		var req2 = new Request('http://example.com/api', {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: '{"key": "value"}'
		});
		req2.method;
	`)
	if err != nil {
		t.Fatalf("Failed to create Request with options: %v", err)
	}
	if result.String() != "POST" {
		t.Errorf("Expected 'POST', got '%s'", result.String())
	}

	// Test bodyUsed initially false
	result, err = runtime.Execute("req2.bodyUsed")
	if err != nil {
		t.Fatalf("Failed to get bodyUsed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected bodyUsed to be false initially")
	}
}

func TestResponseClass(t *testing.T) {
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL("http://localhost/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test Response constructor
	result, err := runtime.Execute(`
		var resp = new Response('Hello', {status: 200, statusText: 'OK'});
		resp.status;
	`)
	if err != nil {
		t.Fatalf("Failed to create Response: %v", err)
	}
	if result.ToInteger() != 200 {
		t.Errorf("Expected 200, got %d", result.ToInteger())
	}

	// Test ok property
	result, err = runtime.Execute("resp.ok")
	if err != nil {
		t.Fatalf("Failed to get ok: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected ok to be true for 200 status")
	}

	// Test Response.error()
	result, err = runtime.Execute(`
		var errResp = Response.error();
		errResp.type;
	`)
	if err != nil {
		t.Fatalf("Failed to create error Response: %v", err)
	}
	if result.String() != "error" {
		t.Errorf("Expected 'error', got '%s'", result.String())
	}

	// Test Response.redirect()
	result, err = runtime.Execute(`
		var redirResp = Response.redirect('http://example.com', 302);
		redirResp.status;
	`)
	if err != nil {
		t.Fatalf("Failed to create redirect Response: %v", err)
	}
	if result.ToInteger() != 302 {
		t.Errorf("Expected 302, got %d", result.ToInteger())
	}

	// Test Response.json()
	result, err = runtime.Execute(`
		var jsonResp = Response.json({name: 'test', value: 42});
		jsonResp.headers.get('content-type');
	`)
	if err != nil {
		t.Fatalf("Failed to create JSON Response: %v", err)
	}
	if result.String() != "application/json" {
		t.Errorf("Expected 'application/json', got '%s'", result.String())
	}
}

func TestAbortController(t *testing.T) {
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL("http://localhost/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test AbortController creation
	_, err := runtime.Execute(`
		var controller = new AbortController();
		var signal = controller.signal;
	`)
	if err != nil {
		t.Fatalf("Failed to create AbortController: %v", err)
	}

	// Test signal.aborted initially false
	result, err := runtime.Execute("signal.aborted")
	if err != nil {
		t.Fatalf("Failed to check aborted: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected aborted to be false initially")
	}

	// Test abort()
	_, err = runtime.Execute("controller.abort()")
	if err != nil {
		t.Fatalf("Failed to abort: %v", err)
	}

	result, err = runtime.Execute("signal.aborted")
	if err != nil {
		t.Fatalf("Failed to check aborted after abort: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected aborted to be true after abort()")
	}

	// Test AbortSignal.abort()
	result, err = runtime.Execute(`
		var preAborted = AbortSignal.abort();
		preAborted.aborted;
	`)
	if err != nil {
		t.Fatalf("Failed to create pre-aborted signal: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected AbortSignal.abort() to return aborted signal")
	}
}

func TestFetchWithTextResponse(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with text response
	_, err := runtime.Execute(fmt.Sprintf(`
		var fetchResult = null;
		var fetchError = null;
		fetch('%s/text')
			.then(function(response) { return response.text(); })
			.then(function(text) { fetchResult = text; })
			.catch(function(err) { fetchError = err; });
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	// Run the event loop to process the async request
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("fetchResult")
		if result != nil && !result.Equals(runtime.vm.ToValue(nil)) && result.String() == "Hello, World!" {
			break
		}
	}

	// Check the result
	result, err := runtime.Execute("fetchResult")
	if err != nil {
		t.Fatalf("Failed to get fetchResult: %v", err)
	}
	if result.String() != "Hello, World!" {
		// Check for errors
		errResult, _ := runtime.Execute("fetchError ? fetchError.toString() : 'no error'")
		t.Errorf("Expected 'Hello, World!', got '%s', error: %v", result.String(), errResult)
	}
}

func TestFetchWithJSONResponse(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":  "test",
			"value": 42,
		})
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with JSON response
	_, err := runtime.Execute(fmt.Sprintf(`
		var jsonResult = null;
		var jsonError = null;
		fetch('%s/json')
			.then(function(response) { return response.json(); })
			.then(function(data) { jsonResult = data; })
			.catch(function(err) { jsonError = err; });
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	// Run the event loop to process the async request
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("jsonResult && jsonResult.name")
		if result != nil && result.String() == "test" {
			break
		}
	}

	// Check the result
	result, err := runtime.Execute("jsonResult ? jsonResult.name : null")
	if err != nil {
		t.Fatalf("Failed to get jsonResult.name: %v", err)
	}
	if result.String() != "test" {
		errResult, _ := runtime.Execute("jsonError ? jsonError.toString() : 'no error'")
		t.Errorf("Expected 'test', got '%s', error: %v", result.String(), errResult)
	}

	result, err = runtime.Execute("jsonResult ? jsonResult.value : null")
	if err != nil {
		t.Fatalf("Failed to get jsonResult.value: %v", err)
	}
	if result.ToInteger() != 42 {
		t.Errorf("Expected 42, got %d", result.ToInteger())
	}
}

func TestFetchWithPOST(t *testing.T) {
	var receivedMethod string
	var receivedBody string
	var receivedContentType string

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")

		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		receivedBody = strings.TrimSpace(string(body[:n]))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with POST
	_, err := runtime.Execute(fmt.Sprintf(`
		var postResult = null;
		var postError = null;
		fetch('%s/api', {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify({key: 'value'})
		})
			.then(function(response) { return response.json(); })
			.then(function(data) { postResult = data; })
			.catch(function(err) { postError = err; });
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch POST: %v", err)
	}

	// Run the event loop
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("postResult && postResult.received")
		if result != nil && result.ToBoolean() {
			break
		}
	}

	// Check the request was received correctly
	if receivedMethod != "POST" {
		t.Errorf("Expected POST method, got %s", receivedMethod)
	}

	if receivedContentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", receivedContentType)
	}

	if !strings.Contains(receivedBody, "key") {
		t.Errorf("Expected body to contain 'key', got '%s'", receivedBody)
	}
}

func TestFetchResponseProperties(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Custom", "custom-value")
		w.WriteHeader(http.StatusCreated) // 201
		w.Write([]byte("Created"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch response properties
	_, err := runtime.Execute(fmt.Sprintf(`
		var respStatus = null;
		var respOk = null;
		var respUrl = null;
		var respError = null;
		fetch('%s/create')
			.then(function(response) {
				respStatus = response.status;
				respOk = response.ok;
				respUrl = response.url;
				return response.text();
			})
			.catch(function(err) { respError = err; });
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	// Run the event loop
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("respStatus")
		if result != nil && result.ToInteger() == 201 {
			break
		}
	}

	// Check response properties
	result, err := runtime.Execute("respStatus")
	if err != nil {
		t.Fatalf("Failed to get respStatus: %v", err)
	}
	if result.ToInteger() != 201 {
		errResult, _ := runtime.Execute("respError ? respError.toString() : 'no error'")
		t.Errorf("Expected status 201, got %d, error: %v", result.ToInteger(), errResult)
	}

	result, err = runtime.Execute("respOk")
	if err != nil {
		t.Fatalf("Failed to get respOk: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected ok to be true for 201 status")
	}

	result, err = runtime.Execute("respUrl")
	if err != nil {
		t.Fatalf("Failed to get respUrl: %v", err)
	}
	if !strings.Contains(result.String(), "/create") {
		t.Errorf("Expected URL to contain '/create', got '%s'", result.String())
	}
}

func TestFetchNetworkError(t *testing.T) {
	// Set up runtime and document with a URL that won't work
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL("http://localhost/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with invalid URL
	_, err := runtime.Execute(`
		var networkError = null;
		var networkSuccess = false;
		fetch('http://localhost:99999/invalid')
			.then(function(response) { networkSuccess = true; })
			.catch(function(err) { networkError = err; });
	`)
	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	// Run the event loop
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("networkError !== null")
		if result != nil && result.ToBoolean() {
			break
		}
	}

	// Check that we got an error
	result, err := runtime.Execute("networkError !== null")
	if err != nil {
		t.Fatalf("Failed to check networkError: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected networkError to be set")
	}

	result, err = runtime.Execute("networkSuccess")
	if err != nil {
		t.Fatalf("Failed to check networkSuccess: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected networkSuccess to be false")
	}
}

func TestFetchAbort(t *testing.T) {
	// Create a slow test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Slow response
		w.Write([]byte("Slow response"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with abort
	_, err := runtime.Execute(fmt.Sprintf(`
		var abortError = null;
		var abortSuccess = false;
		var controller = new AbortController();

		fetch('%s/slow', {signal: controller.signal})
			.then(function(response) { abortSuccess = true; })
			.catch(function(err) { abortError = err; });

		// Abort immediately
		controller.abort();
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch with abort: %v", err)
	}

	// Run the event loop
	maxWait := 50
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("abortError !== null")
		if result != nil && result.ToBoolean() {
			break
		}
	}

	// Check that we got an abort error
	result, err := runtime.Execute("abortError !== null")
	if err != nil {
		t.Fatalf("Failed to check abortError: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected abortError to be set")
	}

	result, err = runtime.Execute("abortSuccess")
	if err != nil {
		t.Fatalf("Failed to check abortSuccess: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected abortSuccess to be false")
	}
}

func TestFetchPreAbortedSignal(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Should not reach here"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test fetch with pre-aborted signal
	_, err := runtime.Execute(fmt.Sprintf(`
		var preAbortError = null;
		var preAbortSuccess = false;
		var signal = AbortSignal.abort();

		fetch('%s/test', {signal: signal})
			.then(function(response) { preAbortSuccess = true; })
			.catch(function(err) { preAbortError = err; });
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute fetch with pre-aborted signal: %v", err)
	}

	// Run the event loop
	maxWait := 20
	for i := 0; i < maxWait; i++ {
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		result, _ := runtime.Execute("preAbortError !== null")
		if result != nil && result.ToBoolean() {
			break
		}
	}

	// Check that we got an abort error immediately
	result, err := runtime.Execute("preAbortError !== null")
	if err != nil {
		t.Fatalf("Failed to check preAbortError: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected preAbortError to be set for pre-aborted signal")
	}

	result, err = runtime.Execute("preAbortSuccess")
	if err != nil {
		t.Fatalf("Failed to check preAbortSuccess: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected preAbortSuccess to be false")
	}
}
