package wpt

import (
	"context"
	"testing"
	"time"

	"github.com/AYColumbia/viberowser/dom"
	"github.com/AYColumbia/viberowser/js"
)

func TestMinimalTestHarness(t *testing.T) {
	// Create runner without WPT path - will use embedded testharness.js
	runner := NewRunner("")
	runner.BaseURL = "" // Disable HTTP loading

	// We'll create a simple in-memory test using our embedded harness
	// For now, just verify the harness code parses correctly
	result := runner.RunTestFile("nonexistent.html")
	if result.Error == "" {
		t.Error("Expected error for nonexistent file")
	}
}

func TestTestHarnessBinding(t *testing.T) {
	// Test that our binding captures results correctly
	// This tests the embedded minimal testharness.js
	// Note: testharness.js is loaded as part of our test helper before the inline script runs

	testHTML := `<!DOCTYPE html>
<html>
<head>
<title>Test Document</title>
</head>
<body>
<script id="test-script">
test(function() {
    assert_equals(1, 1, "1 should equal 1");
}, "basic equality test");

test(function() {
    assert_true(true, "true should be true");
}, "basic boolean test");
</script>
</body>
</html>`

	// Parse and run the test using our infrastructure directly
	result := runTestFromHTML(t, testHTML)

	// Verify results
	if result.HarnessStatus != "OK" {
		t.Errorf("Expected harness status OK, got %s (error: %s)", result.HarnessStatus, result.Error)
	}

	if len(result.Tests) != 2 {
		t.Errorf("Expected 2 tests, got %d", len(result.Tests))
	}

	for _, test := range result.Tests {
		if test.Status != StatusPass {
			t.Errorf("Test '%s' failed: %s", test.Name, test.Message)
		}
	}
}

func TestFailingTest(t *testing.T) {
	testHTML := `<!DOCTYPE html>
<html>
<head>
<title>Failing Test</title>
</head>
<body>
<script id="test-script">
test(function() {
    assert_equals(1, 2, "1 should not equal 2");
}, "intentionally failing test");
</script>
</body>
</html>`

	result := runTestFromHTML(t, testHTML)

	if result.HarnessStatus != "OK" {
		t.Errorf("Expected harness status OK, got %s (error: %s)", result.HarnessStatus, result.Error)
	}

	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
		return
	}

	if result.Tests[0].Status != StatusFail {
		t.Errorf("Expected test to fail, but it passed or had status %d", result.Tests[0].Status)
	}
}

func TestAsyncTest(t *testing.T) {
	testHTML := `<!DOCTYPE html>
<html>
<head>
<title>Async Test</title>
</head>
<body>
<script id="test-script">
async_test(function(t) {
    setTimeout(t.step_func_done(function() {
        assert_true(true, "async callback ran");
    }), 10);
}, "async test with setTimeout");
</script>
</body>
</html>`

	result := runTestFromHTML(t, testHTML)

	if result.HarnessStatus != "OK" {
		t.Errorf("Expected harness status OK, got %s (error: %s)", result.HarnessStatus, result.Error)
	}

	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
		return
	}

	if result.Tests[0].Status != StatusPass {
		t.Errorf("Expected async test to pass, but it failed: %s", result.Tests[0].Message)
	}
}

func TestDOMAccess(t *testing.T) {
	testHTML := `<!DOCTYPE html>
<html>
<head>
<title>DOM Access Test</title>
</head>
<body>
<div id="test-element">Hello</div>
<script id="test-script">
test(function() {
    var el = document.getElementById("test-element");
    assert_not_equals(el, null, "element should exist");
    assert_equals(el.textContent, "Hello", "text content should match");
}, "getElementById works");

test(function() {
    var el = document.querySelector("#test-element");
    assert_not_equals(el, null, "element should exist");
    assert_equals(el.id, "test-element", "id should match");
}, "querySelector works");
</script>
</body>
</html>`

	result := runTestFromHTML(t, testHTML)

	if result.HarnessStatus != "OK" {
		t.Errorf("Expected harness status OK, got %s (error: %s)", result.HarnessStatus, result.Error)
	}

	if len(result.Tests) != 2 {
		t.Errorf("Expected 2 tests, got %d", len(result.Tests))
	}

	for _, test := range result.Tests {
		if test.Status != StatusPass {
			t.Errorf("Test '%s' failed: %s", test.Name, test.Message)
		}
	}
}

// runTestFromHTML is a helper that runs a test from HTML content directly.
func runTestFromHTML(t *testing.T, htmlContent string) TestSuiteResult {
	start := time.Now()
	result := TestSuiteResult{
		TestFile: "inline-test.html",
		Tests:    make([]TestResult, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse HTML
	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		result.Error = err.Error()
		result.HarnessStatus = "ERROR"
		result.Duration = time.Since(start)
		return result
	}

	// Create JS runtime
	runtime := js.NewRuntime()
	binding := NewTestHarnessBinding(runtime)

	// Set up completion channel
	completionChan := make(chan struct{})
	binding.SetOnComplete(func(results []TestHarnessResult, status TestHarnessStatus) {
		close(completionChan)
	})

	// Setup test harness callbacks FIRST (before loading testharness.js)
	binding.Setup()

	// Create script executor and bind document
	executor := js.NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Load embedded testharness.js BEFORE executing any inline scripts
	// This is critical - the test functions must be defined before tests call them
	_, err = runtime.Execute(minimalTestHarnessJS)
	if err != nil {
		t.Logf("Error loading testharness.js: %v", err)
		result.Error = err.Error()
		result.HarnessStatus = "ERROR"
		result.Duration = time.Since(start)
		return result
	}
	t.Log("Loaded testharness.js successfully")

	// Now execute inline scripts (which will use the test functions)
	errors := executor.ExecuteScripts(doc)
	for _, e := range errors {
		t.Logf("Script execution error: %v", e)
	}
	t.Logf("Executed %d scripts, %d errors", len(errors), len(errors))

	// Dispatch events
	executor.DispatchDOMContentLoaded()
	executor.DispatchLoadEvent()

	// Check if already completed (synchronous tests)
	if binding.IsCompleted() {
		t.Log("Tests completed synchronously")
	}

	// Run event loop until completion or timeout
	done := false
	for !done {
		select {
		case <-completionChan:
			t.Log("Completion channel signaled")
			done = true
		case <-ctx.Done():
			result.Error = "Test timeout"
			result.HarnessStatus = "TIMEOUT"
			done = true
		case <-time.After(10 * time.Millisecond):
			// Process timers and event loop periodically
			runtime.ProcessTimers()
			executor.RunEventLoopOnce()
		}
	}

	// Collect results
	if binding.IsCompleted() {
		result.HarnessStatus = HarnessStatusString(binding.HarnessStatus().Status)
		for _, tr := range binding.Results() {
			result.Tests = append(result.Tests, TestResult{
				Name:    tr.Name,
				Status:  convertTestStatus(tr.Status),
				Message: tr.Message,
				Stack:   tr.Stack,
			})
		}
	}

	result.Duration = time.Since(start)
	return result
}
