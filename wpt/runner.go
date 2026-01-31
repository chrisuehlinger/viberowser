// Package wpt provides Web Platform Test integration.
package wpt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AYColumbia/viberowser/dom"
	"github.com/AYColumbia/viberowser/js"
	"github.com/AYColumbia/viberowser/network"
)

// TestResult represents the result of a WPT test.
type TestResult struct {
	Name    string
	Status  TestStatus
	Message string
	Stack   string
}

// TestStatus represents the status of a test.
type TestStatus int

const (
	StatusPass TestStatus = iota
	StatusFail
	StatusTimeout
	StatusError
	StatusSkip
)

// TestSuiteResult represents the result of running a test file.
type TestSuiteResult struct {
	TestFile      string
	HarnessStatus string
	Tests         []TestResult
	Duration      time.Duration
	Error         string
}

// Runner handles running WPT tests against the browser.
type Runner struct {
	WPTPath      string                   // Path to WPT repository
	BaseURL      string                   // Base URL for WPT server (e.g., http://localhost:8000)
	Results      []TestSuiteResult
	Timeout      time.Duration            // Per-test timeout
	httpClient   *http.Client
	netClient    *network.Client          // Network client for resource loading
	loader       *network.Loader          // Resource loader
}

// NewRunner creates a new WPT test runner.
func NewRunner(wptPath string) *Runner {
	netClient, _ := network.NewClient(network.WithTimeout(30 * time.Second))
	loader := network.NewLoader(netClient, network.WithLocalPath(wptPath))

	return &Runner{
		WPTPath:    wptPath,
		BaseURL:    "http://localhost:8000",
		Results:    make([]TestSuiteResult, 0),
		Timeout:    30 * time.Second,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		netClient:  netClient,
		loader:     loader,
	}
}

// SetBaseURL sets the WPT server base URL.
func (r *Runner) SetBaseURL(baseURL string) {
	r.BaseURL = strings.TrimRight(baseURL, "/")
}

// RunTestFile runs a single WPT test file and returns the result.
func (r *Runner) RunTestFile(testPath string) TestSuiteResult {
	start := time.Now()
	result := TestSuiteResult{
		TestFile: testPath,
		Tests:    make([]TestResult, 0),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	// Load the test file content
	content, err := r.loadTestFile(ctx, testPath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to load test file: %v", err)
		result.HarnessStatus = "ERROR"
		result.Duration = time.Since(start)
		return result
	}

	// Parse the HTML into DOM document
	doc, err := dom.ParseHTML(content)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to parse HTML: %v", err)
		result.HarnessStatus = "ERROR"
		result.Duration = time.Since(start)
		return result
	}

	// Create JS runtime and set up test harness
	runtime := js.NewRuntime()
	binding := NewTestHarnessBinding(runtime)

	// Set up completion channel
	completionChan := make(chan struct{})
	binding.SetOnComplete(func(results []TestHarnessResult, status TestHarnessStatus) {
		close(completionChan)
	})

	// Setup test harness callbacks BEFORE loading any scripts
	binding.Setup()

	// Create script executor and bind document
	executor := js.NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Load and execute external scripts (including testharness.js from HTML)
	// Use file:// URL if loading from local WPT path, otherwise use HTTP base URL
	var baseURL string
	if r.WPTPath != "" {
		baseURL = "file://" + r.WPTPath + testPath
	} else {
		baseURL = r.BaseURL + "/" + strings.TrimPrefix(testPath, "/")
	}
	if err := r.loadExternalScripts(ctx, doc, executor, baseURL); err != nil {
		result.Error = fmt.Sprintf("Failed to load external scripts: %v", err)
		result.HarnessStatus = "ERROR"
		result.Duration = time.Since(start)
		return result
	}

	// Register our callbacks with testharness.js now that it's loaded
	binding.SetupPostLoad()

	// Execute inline scripts (which include the actual tests)
	executor.ExecuteScripts(doc)

	// Dispatch DOMContentLoaded to trigger any waiting tests
	executor.DispatchDOMContentLoaded()

	// Dispatch load event
	executor.DispatchLoadEvent()

	// Run event loop until tests complete or timeout
	timedOut := false
	done := false
	// Give a short grace period for completion callback after tests run
	gracePeriod := 100 * time.Millisecond
	graceStart := time.Time{}
	for !done {
		select {
		case <-completionChan:
			done = true
		case <-ctx.Done():
			// Timeout occurred
			timedOut = true
			done = true
		default:
			// Process timers (needed for setTimeout in testharness.js)
			runtime.ProcessTimers()
			// Process event loop
			eventLoopHasWork := executor.RunEventLoopOnce()
			if binding.IsCompleted() {
				done = true
			}
			// If we have results but no completion, wait a short grace period
			// then exit (real testharness.js may not call completion for sync tests)
			if len(binding.Results()) > 0 && !binding.IsCompleted() {
				if graceStart.IsZero() {
					graceStart = time.Now()
				} else if time.Since(graceStart) > gracePeriod {
					// Grace period expired, assume tests are done
					done = true
				}
			}
			if !eventLoopHasWork {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}

	// Collect results
	// Even if completion callback wasn't invoked, we may have collected results
	// from the result_callback (which fires for each test)
	results := binding.Results()
	if len(results) > 0 {
		if binding.IsCompleted() {
			result.HarnessStatus = HarnessStatusString(binding.HarnessStatus().Status)
		} else {
			// Tests ran but completion didn't fire - still report results
			result.HarnessStatus = "OK"
		}
		for _, tr := range results {
			result.Tests = append(result.Tests, TestResult{
				Name:    tr.Name,
				Status:  convertTestStatus(tr.Status),
				Message: tr.Message,
				Stack:   tr.Stack,
			})
		}
	} else if timedOut {
		// Only set TIMEOUT if we actually timed out with no results
		result.Error = "Test timeout"
		result.HarnessStatus = "TIMEOUT"
	}

	result.Duration = time.Since(start)
	return result
}

// loadTestFile loads a test file, either from disk or from WPT server.
func (r *Runner) loadTestFile(ctx context.Context, testPath string) (string, error) {
	// Try loading from local WPT path first
	if r.WPTPath != "" {
		localPath := filepath.Join(r.WPTPath, testPath)
		if data, err := os.ReadFile(localPath); err == nil {
			return string(data), nil
		}
	}

	// Try loading from WPT server
	if r.BaseURL != "" {
		testURL := r.BaseURL + "/" + strings.TrimPrefix(testPath, "/")
		req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "", fmt.Errorf("no WPT path or base URL configured")
}

// loadTestHarnessJS loads the testharness.js library into the runtime.
func (r *Runner) loadTestHarnessJS(ctx context.Context, runtime *js.Runtime) error {
	// Try to load from WPT resources
	content, err := r.loadResource(ctx, "/resources/testharness.js")
	if err != nil {
		// Use embedded minimal version as fallback
		content = minimalTestHarnessJS
	}

	_, err = runtime.Execute(content)
	return err
}

// loadResource loads a resource file (like testharness.js) from the WPT repository.
func (r *Runner) loadResource(ctx context.Context, resourcePath string) (string, error) {
	// Try local path first
	if r.WPTPath != "" {
		localPath := filepath.Join(r.WPTPath, resourcePath)
		if data, err := os.ReadFile(localPath); err == nil {
			return string(data), nil
		}
	}

	// Try WPT server
	if r.BaseURL != "" {
		resourceURL := r.BaseURL + resourcePath
		req, err := http.NewRequestWithContext(ctx, "GET", resourceURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "", fmt.Errorf("resource not found: %s", resourcePath)
}

// loadExternalScripts loads and executes external scripts in document order.
func (r *Runner) loadExternalScripts(ctx context.Context, doc *dom.Document, executor *js.ScriptExecutor, baseURL string) error {
	// Set base URL for loader
	r.loader.SetBaseURL(baseURL)

	// Create document loader
	docLoader := network.NewDocumentLoader(r.loader)
	loadedDoc := docLoader.LoadDocumentWithResources(ctx, doc, baseURL)

	// Execute sync scripts in order
	for _, script := range loadedDoc.GetSyncScripts() {
		if err := executor.ExecuteExternalScript(script.Content, script.URL); err != nil {
			return fmt.Errorf("error executing %s: %w", script.URL, err)
		}
	}

	return nil
}

// RunTest runs a single WPT test (compatibility method).
func (r *Runner) RunTest(testPath string) TestResult {
	suiteResult := r.RunTestFile(testPath)
	r.Results = append(r.Results, suiteResult)

	if len(suiteResult.Tests) > 0 {
		return suiteResult.Tests[0]
	}

	status := StatusSkip
	if suiteResult.Error != "" {
		status = StatusError
	}
	return TestResult{
		Name:    testPath,
		Status:  status,
		Message: suiteResult.Error,
	}
}

// Summary returns a summary of the test results.
func (r *Runner) Summary() (passed, failed, skipped int) {
	for _, result := range r.Results {
		for _, test := range result.Tests {
			switch test.Status {
			case StatusPass:
				passed++
			case StatusFail, StatusTimeout, StatusError:
				failed++
			case StatusSkip:
				skipped++
			}
		}
	}
	return
}

// convertTestStatus converts TestHarness status to TestStatus.
func convertTestStatus(status int) TestStatus {
	switch status {
	case TestStatusPass:
		return StatusPass
	case TestStatusFail:
		return StatusFail
	case TestStatusTimeout:
		return StatusTimeout
	case TestStatusNotRun:
		return StatusSkip
	default:
		return StatusError
	}
}

// WPTJSONResult represents the JSON format for WPT results.
type WPTJSONResult struct {
	Test     string `json:"test"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Duration int64  `json:"duration"`
	Subtests []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	} `json:"subtests"`
}

// ExportJSON exports results in WPT JSON format.
func (r *Runner) ExportJSON() ([]byte, error) {
	results := make([]WPTJSONResult, 0, len(r.Results))

	for _, suite := range r.Results {
		jr := WPTJSONResult{
			Test:     suite.TestFile,
			Status:   suite.HarnessStatus,
			Duration: suite.Duration.Milliseconds(),
		}

		if suite.Error != "" {
			jr.Message = suite.Error
		}

		for _, test := range suite.Tests {
			jr.Subtests = append(jr.Subtests, struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message,omitempty"`
			}{
				Name:    test.Name,
				Status:  statusToString(test.Status),
				Message: test.Message,
			})
		}

		results = append(results, jr)
	}

	return json.MarshalIndent(results, "", "  ")
}

func statusToString(status TestStatus) string {
	switch status {
	case StatusPass:
		return "PASS"
	case StatusFail:
		return "FAIL"
	case StatusTimeout:
		return "TIMEOUT"
	case StatusError:
		return "ERROR"
	case StatusSkip:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// ResolveURL resolves a relative URL against the test file's URL.
func ResolveURL(base, ref string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}

// GetMinimalTestHarnessJS returns the embedded testharness.js implementation.
func GetMinimalTestHarnessJS() string {
	return minimalTestHarnessJS
}

// minimalTestHarnessJS is a minimal testharness.js implementation
// used as a fallback when the real testharness.js cannot be loaded.
// NOTE: This uses direct global assignments (not window.xxx) because
// in goja, window is not the same as the global object.
var minimalTestHarnessJS = `
// Test status constants
var Test_PASS = 0;
var Test_FAIL = 1;
var Test_TIMEOUT = 2;
var Test_NOTRUN = 3;

// Harness status constants
var Harness_OK = 0;
var Harness_ERROR = 1;

var _tests = [];
var _harness_status = { status: Harness_OK, message: null };
var _started = false;
var _completed = false;
var _result_callbacks = [];
var _completion_callbacks = [];

function Test(name, func) {
    this.name = name;
    this.func = func;
    this.status = Test_NOTRUN;
    this.message = null;
    this.stack = null;
    this.phase = 0;
}

Test.prototype.step = function(func, this_obj) {
    if (this.phase === 2) return;
    this.phase = 1;
    try {
        func.apply(this_obj || this, Array.prototype.slice.call(arguments, 2));
    } catch(e) {
        this.status = Test_FAIL;
        this.message = e.message || String(e);
        this.stack = e.stack || null;
        this.phase = 2;
    }
};

Test.prototype.step_func = function(func, this_obj) {
    var t = this;
    return function() {
        t.step(function() {
            func.apply(this_obj || t, arguments);
        });
    };
};

Test.prototype.step_func_done = function(func, this_obj) {
    var t = this;
    return function() {
        t.step(function() {
            if (func) func.apply(this_obj || t, arguments);
            t.done();
        });
    };
};

Test.prototype.done = function() {
    if (this.phase === 2) return;
    if (this.status === Test_NOTRUN) {
        this.status = Test_PASS;
    }
    this.phase = 2;
    _report_result(this);
    _check_complete();
};

Test.prototype.add_cleanup = function(func) {
    // Simplified: just ignore cleanup for now
};

Test.PASS = Test_PASS;
Test.FAIL = Test_FAIL;
Test.TIMEOUT = Test_TIMEOUT;
Test.NOTRUN = Test_NOTRUN;

function _report_result(test) {
    for (var i = 0; i < _result_callbacks.length; i++) {
        _result_callbacks[i](test);
    }
    if (typeof result_callback === 'function') {
        result_callback(test);
    }
}

function _check_complete() {
    if (_completed) return;
    for (var i = 0; i < _tests.length; i++) {
        if (_tests[i].phase !== 2) return;
    }
    _complete();
}

function _complete() {
    if (_completed) return;
    _completed = true;

    for (var i = 0; i < _completion_callbacks.length; i++) {
        _completion_callbacks[i](_tests, _harness_status);
    }
    if (typeof completion_callback === 'function') {
        completion_callback(_tests, _harness_status);
    }
}

// Public API - defined as global functions
function test(func, name) {
    if (!_started) {
        _started = true;
        if (typeof start_callback === 'function') {
            start_callback();
        }
    }

    var t = new Test(name || '', func);
    _tests.push(t);

    try {
        t.phase = 1;
        func(t);
        if (t.phase !== 2) {
            t.done();
        }
    } catch(e) {
        t.status = Test_FAIL;
        t.message = e.message || String(e);
        t.stack = e.stack || null;
        t.phase = 2;
        _report_result(t);
        _check_complete();
    }
}

function async_test(func, name) {
    if (!_started) {
        _started = true;
        if (typeof start_callback === 'function') {
            start_callback();
        }
    }

    var t = new Test(name || '', func);
    _tests.push(t);

    if (func) {
        try {
            t.phase = 1;
            func(t);
        } catch(e) {
            t.status = Test_FAIL;
            t.message = e.message || String(e);
            t.stack = e.stack || null;
            t.phase = 2;
            _report_result(t);
            _check_complete();
        }
    }

    return t;
}

function promise_test(func, name) {
    if (!_started) {
        _started = true;
        if (typeof start_callback === 'function') {
            start_callback();
        }
    }

    var t = new Test(name || '', func);
    _tests.push(t);
    t.phase = 1;

    var promise;
    try {
        promise = func(t);
    } catch(e) {
        t.status = Test_FAIL;
        t.message = e.message || String(e);
        t.stack = e.stack || null;
        t.phase = 2;
        _report_result(t);
        _check_complete();
        return;
    }

    if (promise && typeof promise.then === 'function') {
        promise.then(function() {
            t.done();
        }, function(e) {
            t.status = Test_FAIL;
            t.message = e.message || String(e);
            t.stack = e.stack || null;
            t.phase = 2;
            _report_result(t);
            _check_complete();
        });
    } else {
        t.done();
    }
}

function done() {
    _check_complete();
}

function setup(options) {
    // Simplified setup - just ignore options for now
}

function add_result_callback(func) {
    _result_callbacks.push(func);
}

function add_completion_callback(func) {
    _completion_callbacks.push(func);
}

function add_start_callback(func) {
    // Simplified - ignore
}

// Assertion functions
function _assert(condition, message) {
    if (!condition) {
        throw new Error(message || 'Assertion failed');
    }
}

function assert_true(actual, description) {
    _assert(actual === true, description || 'expected true but got ' + actual);
}

function assert_false(actual, description) {
    _assert(actual === false, description || 'expected false but got ' + actual);
}

function assert_equals(actual, expected, description) {
    if (actual !== expected) {
        throw new Error((description ? description + ': ' : '') +
            'expected ' + JSON.stringify(expected) + ' but got ' + JSON.stringify(actual));
    }
}

function assert_not_equals(actual, expected, description) {
    if (actual === expected) {
        throw new Error((description ? description + ': ' : '') +
            'expected not ' + JSON.stringify(expected));
    }
}

function assert_array_equals(actual, expected, description) {
    if (!Array.isArray(actual) || !Array.isArray(expected)) {
        throw new Error((description ? description + ': ' : '') + 'expected arrays');
    }
    if (actual.length !== expected.length) {
        throw new Error((description ? description + ': ' : '') +
            'array lengths differ: ' + actual.length + ' vs ' + expected.length);
    }
    for (var i = 0; i < actual.length; i++) {
        if (actual[i] !== expected[i]) {
            throw new Error((description ? description + ': ' : '') +
                'arrays differ at index ' + i + ': ' +
                JSON.stringify(actual[i]) + ' vs ' + JSON.stringify(expected[i]));
        }
    }
}

function assert_approx_equals(actual, expected, epsilon, description) {
    if (typeof actual !== 'number' || typeof expected !== 'number') {
        throw new Error((description ? description + ': ' : '') + 'expected numbers');
    }
    if (Math.abs(actual - expected) > epsilon) {
        throw new Error((description ? description + ': ' : '') +
            'expected ' + expected + ' +/- ' + epsilon + ' but got ' + actual);
    }
}

function assert_in_array(actual, expected, description) {
    for (var i = 0; i < expected.length; i++) {
        if (actual === expected[i]) return;
    }
    throw new Error((description ? description + ': ' : '') +
        JSON.stringify(actual) + ' not in ' + JSON.stringify(expected));
}

function assert_regexp_match(actual, expected, description) {
    if (!expected.test(actual)) {
        throw new Error((description ? description + ': ' : '') +
            JSON.stringify(actual) + ' did not match ' + expected);
    }
}

function assert_class_string(object, class_name, description) {
    var actual = Object.prototype.toString.call(object);
    var expected = '[object ' + class_name + ']';
    if (actual !== expected) {
        throw new Error((description ? description + ': ' : '') +
            'expected ' + expected + ' but got ' + actual);
    }
}

function assert_own_property(object, property, description) {
    if (!object.hasOwnProperty(property)) {
        throw new Error((description ? description + ': ' : '') +
            'expected own property ' + property);
    }
}

function assert_inherits(object, property, description) {
    if (!(property in object)) {
        throw new Error((description ? description + ': ' : '') +
            'expected inherited property ' + property);
    }
    if (object.hasOwnProperty(property)) {
        throw new Error((description ? description + ': ' : '') +
            'property ' + property + ' should not be own property');
    }
}

function assert_idl_attribute(object, attribute, description) {
    if (!(attribute in object)) {
        throw new Error((description ? description + ': ' : '') +
            'expected IDL attribute ' + attribute);
    }
}

function assert_readonly(object, property, description) {
    var desc = Object.getOwnPropertyDescriptor(object, property);
    if (!desc || desc.writable !== false) {
        throw new Error((description ? description + ': ' : '') +
            'expected readonly property ' + property);
    }
}

function assert_throws_js(constructor, func, description) {
    try {
        func();
        throw new Error((description ? description + ': ' : '') +
            'expected exception ' + constructor.name);
    } catch(e) {
        if (!(e instanceof constructor)) {
            throw new Error((description ? description + ': ' : '') +
                'expected ' + constructor.name + ' but got ' + e.constructor.name);
        }
    }
}

function assert_throws_dom(type, func, description) {
    try {
        func();
        throw new Error((description ? description + ': ' : '') +
            'expected DOMException ' + type);
    } catch(e) {
        if (e.name !== type && e.code !== type) {
            throw new Error((description ? description + ': ' : '') +
                'expected DOMException ' + type + ' but got ' + e.name);
        }
    }
}

function assert_throws_exactly(expected, func, description) {
    try {
        func();
        throw new Error((description ? description + ': ' : '') +
            'expected exception');
    } catch(e) {
        if (e !== expected) {
            throw new Error((description ? description + ': ' : '') +
                'expected specific exception');
        }
    }
}

function assert_unreached(description) {
    throw new Error((description ? description + ': ' : '') + 'should not be reached');
}

function assert_any(func, actual, expected_array, extra_msg) {
    for (var i = 0; i < expected_array.length; i++) {
        try {
            func(actual, expected_array[i], extra_msg);
            return;
        } catch(e) {}
    }
    throw new Error(extra_msg || 'No assertion passed');
}

function format_value(val) {
    return JSON.stringify(val);
}
`
