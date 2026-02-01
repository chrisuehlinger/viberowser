package js

import (
	"strings"
	"testing"
	"time"
)

func TestRuntimeBasic(t *testing.T) {
	r := NewRuntime()

	// Test basic execution
	result, err := r.Execute("1 + 2")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected 3, got %v", result.ToInteger())
	}
}

func TestRuntimeVariables(t *testing.T) {
	r := NewRuntime()

	// Test variable assignment and retrieval
	_, err := r.Execute("var x = 42;")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("x")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 42 {
		t.Errorf("Expected 42, got %v", result.ToInteger())
	}
}

func TestRuntimeFunctions(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		function add(a, b) {
			return a + b;
		}
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("add(3, 4)")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 7 {
		t.Errorf("Expected 7, got %v", result.ToInteger())
	}
}

func TestRuntimeConsole(t *testing.T) {
	r := NewRuntime()

	// Test console.log doesn't throw
	_, err := r.Execute(`console.log("test message")`)
	if err != nil {
		t.Fatalf("console.log failed: %v", err)
	}

	// Test other console methods
	_, err = r.Execute(`
		console.warn("warning");
		console.error("error");
		console.info("info");
		console.debug("debug");
	`)
	if err != nil {
		t.Fatalf("console methods failed: %v", err)
	}
}

func TestRuntimeSetTimeout(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		var called = false;
		setTimeout(function() {
			called = true;
		}, 10);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Process timers
	time.Sleep(20 * time.Millisecond)
	r.ProcessTimers()

	result, err := r.Execute("called")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("setTimeout callback was not called")
	}
}

func TestRuntimeClearTimeout(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		var called = false;
		var id = setTimeout(function() {
			called = true;
		}, 10);
		clearTimeout(id);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Process timers
	time.Sleep(20 * time.Millisecond)
	r.ProcessTimers()

	result, err := r.Execute("called")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("setTimeout callback was called after clearTimeout")
	}
}

func TestRuntimeSetInterval(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		var count = 0;
		var id = setInterval(function() {
			count++;
		}, 10);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Process timers multiple times
	for i := 0; i < 5; i++ {
		time.Sleep(15 * time.Millisecond)
		r.ProcessTimers()
	}

	// Clear interval
	_, _ = r.Execute("clearInterval(id)")

	result, err := r.Execute("count")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() < 3 {
		t.Errorf("Expected count >= 3, got %v", result.ToInteger())
	}
}

func TestRuntimeWindow(t *testing.T) {
	r := NewRuntime()

	// Test window object exists
	result, err := r.Execute("typeof window")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "object" {
		t.Errorf("Expected 'object', got %v", result.String())
	}

	// Test window properties
	result, err = r.Execute("window.innerWidth")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1024 {
		t.Errorf("Expected 1024, got %v", result.ToInteger())
	}

	// Test navigator
	result, err = r.Execute("navigator.userAgent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result.String(), "Viberowser") {
		t.Errorf("Expected userAgent to contain 'Viberowser', got %v", result.String())
	}
}

func TestRuntimePerformance(t *testing.T) {
	r := NewRuntime()

	result, err := r.Execute("performance.now()")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	now := result.ToFloat()
	if now < 0 {
		t.Errorf("Expected performance.now() > 0, got %v", now)
	}

	// Wait a bit and check that time has passed
	time.Sleep(10 * time.Millisecond)

	result, err = r.Execute("performance.now()")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	later := result.ToFloat()
	if later <= now {
		t.Errorf("Expected performance.now() to increase, got %v then %v", now, later)
	}
}

func TestRuntimeErrorHandling(t *testing.T) {
	r := NewRuntime()

	// Test syntax error
	_, err := r.Execute("this is not valid javascript")
	if err == nil {
		t.Error("Expected error for invalid JavaScript")
	}

	// Check error is recorded
	errors := r.Errors()
	if len(errors) == 0 {
		t.Error("Expected error to be recorded")
	}

	// Clear errors
	r.ClearErrors()
	errors = r.Errors()
	if len(errors) != 0 {
		t.Errorf("Expected errors to be cleared, got %d", len(errors))
	}
}

func TestRuntimeRequestAnimationFrame(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		var timestamp = null;
		requestAnimationFrame(function(ts) {
			timestamp = ts;
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Process timers
	time.Sleep(20 * time.Millisecond)
	r.ProcessTimers()

	result, err := r.Execute("timestamp")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToFloat() <= 0 {
		t.Errorf("Expected timestamp > 0, got %v", result.ToFloat())
	}
}

func TestRuntimeGlobalThis(t *testing.T) {
	r := NewRuntime()

	// Test globalThis
	result, err := r.Execute("globalThis === window")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected globalThis === window")
	}

	// Test self
	result, err = r.Execute("self === window")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected self === window")
	}
}

func TestRuntimeQueueMicrotask(t *testing.T) {
	r := NewRuntime()

	_, err := r.Execute(`
		var order = [];
		queueMicrotask(function() {
			order.push(1);
		});
		order.push(0);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run event loop to process microtasks
	r.RunEventLoop()

	result, err := r.Execute("order.join(',')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "0,1" {
		t.Errorf("Expected '0,1', got %v", result.String())
	}
}

func TestRuntimePanicRecovery(t *testing.T) {
	r := NewRuntime()

	// Test that we recover from parser panics gracefully
	// Unicode escapes like \u{10ffff} can cause goja to panic
	// We should handle this gracefully without crashing
	code := `var x = "\u{10ffff}";` // This may panic in some goja versions
	err := r.ExecuteScript(code, "test.js")

	// The function should not have panicked, but might return an error
	// We just verify that we didn't crash and the error is handled
	if err != nil {
		// Error is expected if goja panics - verify it contains panic info
		if !strings.Contains(err.Error(), "panic") && !strings.Contains(err.Error(), "unicode") {
			// This might be a different error, still OK as long as we didn't crash
			t.Logf("Got error (expected for unicode escape): %v", err)
		}
	}

	// Verify the runtime is still usable after the error
	result, err := r.Execute("1 + 1")
	if err != nil {
		t.Errorf("Runtime should still work after panic recovery: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2, got %v", result.ToInteger())
	}
}
