package js

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestXMLHttpRequestBasic(t *testing.T) {
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

	// Test that XMLHttpRequest constructor exists
	result, err := runtime.Execute("typeof XMLHttpRequest")
	if err != nil {
		t.Fatalf("Failed to check XMLHttpRequest type: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected XMLHttpRequest to be a function, got %s", result.String())
	}

	// Test creating an instance
	_, err = runtime.Execute(`
		var xhr = new XMLHttpRequest();
		xhr.readyState;
	`)
	if err != nil {
		t.Fatalf("Failed to create XMLHttpRequest: %v", err)
	}

	// Test readyState constants
	result, err = runtime.Execute("XMLHttpRequest.UNSENT")
	if err != nil || result.ToInteger() != 0 {
		t.Errorf("Expected UNSENT to be 0, got %v", result)
	}

	result, err = runtime.Execute("XMLHttpRequest.DONE")
	if err != nil || result.ToInteger() != 4 {
		t.Errorf("Expected DONE to be 4, got %v", result)
	}

	// Test open() method
	_, err = runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test");
		xhr.readyState;
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to open XMLHttpRequest: %v", err)
	}
}

func TestXMLHttpRequestSync(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name": "test", "value": 42}`))
		} else {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("X-Custom-Header", "test-value")
			w.Write([]byte("Hello, World!"))
		}
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test synchronous request
	result, err := runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test", false); // synchronous
		xhr.send();
		xhr.responseText;
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute sync XHR: %v", err)
	}
	if result.String() != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", result.String())
	}

	// Test status
	result, err = runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test", false);
		xhr.send();
		xhr.status;
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if result.ToInteger() != 200 {
		t.Errorf("Expected status 200, got %d", result.ToInteger())
	}

	// Test getResponseHeader
	result, err = runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test", false);
		xhr.send();
		xhr.getResponseHeader("X-Custom-Header");
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to get response header: %v", err)
	}
	if result.String() != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result.String())
	}

	// Test getAllResponseHeaders
	result, err = runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test", false);
		xhr.send();
		xhr.getAllResponseHeaders().includes("content-type");
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to get all response headers: %v", err)
	}
	if !result.ToBoolean() {
		t.Errorf("Expected getAllResponseHeaders to include content-type")
	}
}

func TestXMLHttpRequestAsync(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Async Response"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test async request with onload callback
	_, err := runtime.Execute(fmt.Sprintf(`
		var result = null;
		var xhr = new XMLHttpRequest();
		xhr.onload = function() {
			result = xhr.responseText;
		};
		xhr.open("GET", "%s/test", true);
		xhr.send();
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to start async XHR: %v", err)
	}

	// Process the event loop to allow async operations to complete
	maxIterations := 100
	for i := 0; i < maxIterations; i++ {
		runtime.ProcessTimers()
		runtime.RunEventLoop()
		time.Sleep(10 * time.Millisecond)

		// Check if result is set
		result, err := runtime.Execute("result")
		if err != nil {
			t.Fatalf("Failed to check result: %v", err)
		}
		if result.String() == "Async Response" {
			return // Success
		}
	}

	// Check final result
	result, err := runtime.Execute("result")
	if err != nil {
		t.Fatalf("Failed to get final result: %v", err)
	}
	if result.String() != "Async Response" {
		t.Errorf("Expected 'Async Response', got '%s'", result.String())
	}
}

func TestXMLHttpRequestOnreadystatechange(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Test"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test onreadystatechange callback with synchronous request
	result, err := runtime.Execute(fmt.Sprintf(`
		var states = [];
		var xhr = new XMLHttpRequest();
		xhr.onreadystatechange = function() {
			states.push(xhr.readyState);
		};
		xhr.open("GET", "%s/test", false);
		xhr.send();
		states.join(",");
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute XHR with onreadystatechange: %v", err)
	}
	// Should see: OPENED (1), HEADERS_RECEIVED (2), LOADING (3), DONE (4)
	expected := "1,2,3,4"
	if result.String() != expected {
		t.Errorf("Expected states '%s', got '%s'", expected, result.String())
	}
}

func TestXMLHttpRequestSetRequestHeader(t *testing.T) {
	// Create a test server that echoes headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customHeader := r.Header.Get("X-Custom-Request")
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(customHeader))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test setRequestHeader
	result, err := runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "%s/test", false);
		xhr.setRequestHeader("X-Custom-Request", "custom-value");
		xhr.send();
		xhr.responseText;
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to send request with custom header: %v", err)
	}
	if result.String() != "custom-value" {
		t.Errorf("Expected 'custom-value', got '%s'", result.String())
	}
}

func TestXMLHttpRequestJSON(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name": "test", "value": 42}`))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test JSON responseType
	result, err := runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.responseType = "json";
		xhr.open("GET", "%s/json", false);
		xhr.send();
		xhr.response ? xhr.response.name : "null";
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if result.String() != "test" {
		t.Errorf("Expected 'test', got '%s'", result.String())
	}
}

func TestXMLHttpRequestPost(t *testing.T) {
	var receivedBody string
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			receivedBody = string(body[:n])
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test POST request with body
	result, err := runtime.Execute(fmt.Sprintf(`
		var xhr = new XMLHttpRequest();
		xhr.open("POST", "%s/test", false);
		xhr.setRequestHeader("Content-Type", "application/json");
		xhr.send('{"test": "data"}');
		xhr.status;
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	if result.ToInteger() != 200 {
		t.Errorf("Expected status 200, got %d", result.ToInteger())
	}
	if receivedBody != `{"test": "data"}` {
		t.Errorf("Expected body '{\"test\": \"data\"}', got '%s'", receivedBody)
	}
}

func TestXMLHttpRequestAbort(t *testing.T) {
	// Create a slow test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Write([]byte("Should not receive this"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test abort
	_, err := runtime.Execute(fmt.Sprintf(`
		var aborted = false;
		var xhr = new XMLHttpRequest();
		xhr.onabort = function() {
			aborted = true;
		};
		xhr.open("GET", "%s/test", true);
		xhr.send();
		xhr.abort();
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to abort XHR: %v", err)
	}

	// Check readyState after abort
	result, err := runtime.Execute("xhr.readyState")
	if err != nil {
		t.Fatalf("Failed to get readyState after abort: %v", err)
	}
	if result.ToInteger() != 0 { // UNSENT
		t.Errorf("Expected readyState 0 after abort, got %d", result.ToInteger())
	}
}

func TestXMLHttpRequestEventListeners(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Test"))
	}))
	defer server.Close()

	// Set up runtime and document
	runtime := NewRuntime()
	doc := dom.NewDocument()
	doc.SetURL(server.URL + "/test.html")
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Test addEventListener
	result, err := runtime.Execute(fmt.Sprintf(`
		var events = [];
		var xhr = new XMLHttpRequest();
		xhr.addEventListener("loadstart", function() { events.push("loadstart"); });
		xhr.addEventListener("load", function() { events.push("load"); });
		xhr.addEventListener("loadend", function() { events.push("loadend"); });
		xhr.open("GET", "%s/test", false);
		xhr.send();
		events.join(",");
	`, server.URL))
	if err != nil {
		t.Fatalf("Failed to execute XHR with event listeners: %v", err)
	}
	// Should see: loadstart, load, loadend
	if !containsAll(result.String(), []string{"loadstart", "load", "loadend"}) {
		t.Errorf("Expected events to contain loadstart,load,loadend, got '%s'", result.String())
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !containsString(s, part) {
			return false
		}
	}
	return true
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
