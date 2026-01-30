// Package wpt provides Web Platform Test integration.
package wpt

// TestResult represents the result of a WPT test.
type TestResult struct {
	Name    string
	Status  TestStatus
	Message string
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

// Runner handles running WPT tests against the browser.
type Runner struct {
	WPTPath string
	Results []TestResult
}

// NewRunner creates a new WPT test runner.
func NewRunner(wptPath string) *Runner {
	return &Runner{
		WPTPath: wptPath,
		Results: make([]TestResult, 0),
	}
}

// RunTest runs a single WPT test.
func (r *Runner) RunTest(testPath string) TestResult {
	// TODO: Implement WPT test running
	return TestResult{
		Name:    testPath,
		Status:  StatusSkip,
		Message: "Not implemented",
	}
}

// Summary returns a summary of the test results.
func (r *Runner) Summary() (passed, failed, skipped int) {
	for _, result := range r.Results {
		switch result.Status {
		case StatusPass:
			passed++
		case StatusFail, StatusTimeout, StatusError:
			failed++
		case StatusSkip:
			skipped++
		}
	}
	return
}
