// Package wpt provides Web Platform Test integration.
package wpt

import (
	"github.com/AYColumbia/viberowser/js"
	"github.com/dop251/goja"
)

// TestHarnessResult represents the result of a single testharness.js test.
type TestHarnessResult struct {
	Name    string
	Status  int // 0=PASS, 1=FAIL, 2=TIMEOUT, 3=NOTRUN
	Message string
	Stack   string
}

// TestHarnessStatus represents the overall harness status.
type TestHarnessStatus struct {
	Status  int // 0=OK, 1=ERROR, 2=TIMEOUT
	Message string
}

// Status constants for individual tests
const (
	TestStatusPass              = 0
	TestStatusFail              = 1
	TestStatusTimeout           = 2
	TestStatusNotRun            = 3
	TestStatusPreconditionFailed = 4
)

// Harness status constants
const (
	HarnessStatusOK      = 0
	HarnessStatusError   = 1
	HarnessStatusTimeout = 2
)

// TestHarnessBinding sets up testharness.js integration in a JS runtime.
// It provides the callback infrastructure needed to capture test results.
type TestHarnessBinding struct {
	runtime       *js.Runtime
	results       []TestHarnessResult
	harnessStatus TestHarnessStatus
	completed     bool
	onComplete    func([]TestHarnessResult, TestHarnessStatus)
}

// NewTestHarnessBinding creates a new test harness binding for the runtime.
func NewTestHarnessBinding(runtime *js.Runtime) *TestHarnessBinding {
	return &TestHarnessBinding{
		runtime:       runtime,
		results:       make([]TestHarnessResult, 0),
		harnessStatus: TestHarnessStatus{Status: HarnessStatusOK},
		completed:     false,
	}
}

// SetOnComplete sets a callback to be invoked when all tests complete.
func (thb *TestHarnessBinding) SetOnComplete(callback func([]TestHarnessResult, TestHarnessStatus)) {
	thb.onComplete = callback
}

// Setup installs the testharness.js callback functions into the JS runtime.
// This should be called BEFORE loading testharness.js so that our callbacks
// are registered when the harness initializes.
func (thb *TestHarnessBinding) Setup() {
	vm := thb.runtime.VM()

	// Install global completion_callback
	vm.Set("completion_callback", func(call goja.FunctionCall) goja.Value {
		thb.handleCompletionCallback(call)
		return goja.Undefined()
	})

	// Install global result_callback
	vm.Set("result_callback", func(call goja.FunctionCall) goja.Value {
		thb.handleResultCallback(call)
		return goja.Undefined()
	})

	// Install global start_callback
	vm.Set("start_callback", func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})
}

// SetupPostLoad registers our callbacks with testharness.js after it has loaded.
// This uses add_result_callback and add_completion_callback which are the
// proper API for the real testharness.js. When using the real API, we disable
// the global callbacks to avoid duplicate results.
func (thb *TestHarnessBinding) SetupPostLoad() {
	vm := thb.runtime.VM()

	// Create our result callback wrapper
	resultCallback := vm.ToValue(func(call goja.FunctionCall) goja.Value {
		thb.handleResultCallback(call)
		return goja.Undefined()
	})

	// Create our completion callback wrapper
	completionCallback := vm.ToValue(func(call goja.FunctionCall) goja.Value {
		thb.handleCompletionCallback(call)
		return goja.Undefined()
	})

	vm.Set("__viberowser_result_callback", resultCallback)
	vm.Set("__viberowser_completion_callback", completionCallback)

	// Try to register using the real testharness.js API
	// If the real API exists, use it and disable global callbacks to avoid duplicates
	code := `
		(function() {
			if (typeof add_result_callback === 'function' && typeof add_completion_callback === 'function') {
				add_result_callback(__viberowser_result_callback);
				add_completion_callback(__viberowser_completion_callback);
				// Disable global callbacks to avoid duplicates
				result_callback = function() {};
				completion_callback = function() {};
			}
			// If real API not available, global callbacks from Setup() will be used
		})();
	`

	thb.runtime.Execute(code)
}

// handleResultCallback processes individual test results as they come in.
func (thb *TestHarnessBinding) handleResultCallback(call goja.FunctionCall) {
	if len(call.Arguments) < 1 {
		return
	}

	vm := thb.runtime.VM()
	resultObj := call.Arguments[0].ToObject(vm)
	if resultObj == nil {
		return
	}

	result := TestHarnessResult{}

	// Extract name
	if name := resultObj.Get("name"); name != nil && !goja.IsUndefined(name) && !goja.IsNull(name) {
		result.Name = name.String()
	}

	// Extract status
	if status := resultObj.Get("status"); status != nil && !goja.IsUndefined(status) && !goja.IsNull(status) {
		result.Status = int(status.ToInteger())
	}

	// Extract message
	if msg := resultObj.Get("message"); msg != nil && !goja.IsUndefined(msg) && !goja.IsNull(msg) {
		result.Message = msg.String()
	}

	// Extract stack trace if available
	if stack := resultObj.Get("stack"); stack != nil && !goja.IsUndefined(stack) && !goja.IsNull(stack) {
		result.Stack = stack.String()
	}

	thb.results = append(thb.results, result)
}

// handleCompletionCallback processes the completion callback from testharness.js.
func (thb *TestHarnessBinding) handleCompletionCallback(call goja.FunctionCall) {
	if thb.completed {
		return
	}
	thb.completed = true

	vm := thb.runtime.VM()

	// First argument is array of tests
	if len(call.Arguments) >= 1 {
		testsVal := call.Arguments[0]
		if testsVal != nil && !goja.IsUndefined(testsVal) && !goja.IsNull(testsVal) {
			testsObj := testsVal.ToObject(vm)
			if testsObj != nil {
				// Get length property
				lengthVal := testsObj.Get("length")
				if lengthVal != nil && !goja.IsUndefined(lengthVal) {
					length := int(lengthVal.ToInteger())
					// Clear and rebuild results from completion callback
					// (more reliable than incremental collection)
					thb.results = make([]TestHarnessResult, 0, length)
					for i := 0; i < length; i++ {
						itemVal := testsObj.Get(vm.ToValue(i).String())
						if itemVal != nil && !goja.IsUndefined(itemVal) && !goja.IsNull(itemVal) {
							itemObj := itemVal.ToObject(vm)
							if itemObj != nil {
								result := TestHarnessResult{}
								if name := itemObj.Get("name"); name != nil && !goja.IsUndefined(name) && !goja.IsNull(name) {
									result.Name = name.String()
								}
								if status := itemObj.Get("status"); status != nil && !goja.IsUndefined(status) && !goja.IsNull(status) {
									result.Status = int(status.ToInteger())
								}
								if msg := itemObj.Get("message"); msg != nil && !goja.IsUndefined(msg) && !goja.IsNull(msg) {
									result.Message = msg.String()
								}
								if stack := itemObj.Get("stack"); stack != nil && !goja.IsUndefined(stack) && !goja.IsNull(stack) {
									result.Stack = stack.String()
								}
								thb.results = append(thb.results, result)
							}
						}
					}
				}
			}
		}
	}

	// Second argument is harness status
	if len(call.Arguments) >= 2 {
		statusVal := call.Arguments[1]
		if statusVal != nil && !goja.IsUndefined(statusVal) && !goja.IsNull(statusVal) {
			statusObj := statusVal.ToObject(vm)
			if statusObj != nil {
				if status := statusObj.Get("status"); status != nil && !goja.IsUndefined(status) && !goja.IsNull(status) {
					thb.harnessStatus.Status = int(status.ToInteger())
				}
				if msg := statusObj.Get("message"); msg != nil && !goja.IsUndefined(msg) && !goja.IsNull(msg) {
					thb.harnessStatus.Message = msg.String()
				}
			}
		}
	}

	// Invoke callback if set
	if thb.onComplete != nil {
		thb.onComplete(thb.results, thb.harnessStatus)
	}
}

// Results returns the collected test results.
func (thb *TestHarnessBinding) Results() []TestHarnessResult {
	return thb.results
}

// HarnessStatus returns the overall harness status.
func (thb *TestHarnessBinding) HarnessStatus() TestHarnessStatus {
	return thb.harnessStatus
}

// IsCompleted returns true if the test completion callback has been invoked.
func (thb *TestHarnessBinding) IsCompleted() bool {
	return thb.completed
}

// Summary returns pass/fail/timeout/notrun counts.
func (thb *TestHarnessBinding) Summary() (passed, failed, timeout, notrun int) {
	for _, result := range thb.results {
		switch result.Status {
		case TestStatusPass:
			passed++
		case TestStatusFail:
			failed++
		case TestStatusTimeout:
			timeout++
		case TestStatusNotRun:
			notrun++
		}
	}
	return
}

// StatusString returns a human-readable status string.
func StatusString(status int) string {
	switch status {
	case TestStatusPass:
		return "PASS"
	case TestStatusFail:
		return "FAIL"
	case TestStatusTimeout:
		return "TIMEOUT"
	case TestStatusNotRun:
		return "NOTRUN"
	case TestStatusPreconditionFailed:
		return "PRECONDITION_FAILED"
	default:
		return "UNKNOWN"
	}
}

// HarnessStatusString returns a human-readable harness status string.
func HarnessStatusString(status int) string {
	switch status {
	case HarnessStatusOK:
		return "OK"
	case HarnessStatusError:
		return "ERROR"
	case HarnessStatusTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}
