package wpt

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AYColumbia/viberowser/dom"
	"github.com/AYColumbia/viberowser/js"
	"github.com/AYColumbia/viberowser/network"
)

// TestWPTRunnerDebug is like TestWPTRunner but with more debug output
func TestWPTRunnerDebug(t *testing.T) {
	wptPath := "/workspaces/wpt"
	testPath := "/dom/nodes/Document-createComment.html"

	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	// Load test file
	content, err := os.ReadFile(wptPath + testPath)
	if err != nil {
		t.Fatalf("Failed to load test: %v", err)
	}
	t.Logf("Test file loaded: %d bytes", len(content))

	// Parse HTML
	doc, err := dom.ParseHTML(string(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	t.Log("HTML parsed")

	// Create runtime
	runtime := js.NewRuntime()
	binding := NewTestHarnessBinding(runtime)

	completionChan := make(chan struct{})
	binding.SetOnComplete(func(results []TestHarnessResult, status TestHarnessStatus) {
		t.Logf("OnComplete: %d results", len(results))
		close(completionChan)
	})

	binding.Setup()
	t.Log("Binding setup")

	// Create executor
	executor := js.NewScriptExecutor(runtime)
	executor.SetupDocument(doc)
	t.Log("Document setup")

	// Load external scripts - use the runner's approach
	netClient, _ := network.NewClient(network.WithTimeout(30 * time.Second))
	loader := network.NewLoader(netClient, network.WithLocalPath(wptPath))

	baseURL := "file://" + wptPath + testPath
	loader.SetBaseURL(baseURL)
	docLoader := network.NewDocumentLoader(loader)
	loadedDoc := docLoader.LoadDocumentWithResources(context.Background(), doc, baseURL)

	t.Logf("Loaded %d scripts, %d errors", len(loadedDoc.Scripts), len(loadedDoc.Errors))
	for _, err := range loadedDoc.Errors {
		t.Logf("  Load error: %v", err)
	}

	for _, s := range loadedDoc.GetSyncScripts() {
		t.Logf("Executing: %s (%d bytes)", s.URL, len(s.Content))
		err := executor.ExecuteExternalScript(s.Content, s.URL)
		if err != nil {
			t.Logf("  Exec error: %v", err)
		}
	}

	// SetupPostLoad like the runner does
	binding.SetupPostLoad()
	t.Log("SetupPostLoad called")

	// Execute inline scripts
	errors := executor.ExecuteScripts(doc)
	t.Logf("Inline scripts: %d errors", len(errors))
	for _, err := range errors {
		t.Logf("  Script error: %v", err)
	}

	// Dispatch events
	executor.DispatchDOMContentLoaded()
	executor.DispatchLoadEvent()
	t.Log("Events dispatched")

	t.Logf("Results so far: %d", len(binding.Results()))
	t.Logf("Completed: %v", binding.IsCompleted())

	// Run event loop like the runner does
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := false
	for !done {
		select {
		case <-completionChan:
			t.Log("Completion channel signaled!")
			done = true
		case <-ctx.Done():
			t.Log("Context timeout")
			done = true
		default:
			if !executor.RunEventLoopOnce() && binding.IsCompleted() {
				t.Log("Event loop done and completed")
				done = true
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	t.Logf("Final: Completed=%v, Results=%d", binding.IsCompleted(), len(binding.Results()))
	passed, failed, timeout, notrun := binding.Summary()
	t.Logf("Summary: %d passed, %d failed, %d timeout, %d notrun", passed, failed, timeout, notrun)

	for _, r := range binding.Results() {
		t.Logf("  %s: status=%d", r.Name, r.Status)
	}

	if len(binding.Results()) == 0 {
		t.Error("Expected some results")
	}
}

func TestWPTRunner(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 5 * time.Second

	result := runner.RunTestFile("/dom/nodes/Document-createComment.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	for _, test := range result.Tests {
		t.Logf("  %s: status=%d message=%s", test.Name, test.Status, test.Message)
	}

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTNodeAppendChild tests the Node.appendChild WPT test specifically
func TestWPTNodeAppendChild(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Node-appendChild.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTCharacterDataAppendChild tests the CharacterData.appendChild WPT test
func TestWPTCharacterDataAppendChild(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/CharacterData-appendChild.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTNodeInsertBefore tests the Node.insertBefore WPT test
func TestWPTNodeInsertBefore(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Node-insertBefore.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTNodeChildNodes tests the Node.childNodes WPT test
func TestWPTNodeChildNodes(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Node-childNodes.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTNodeReplaceChild tests the Node.replaceChild WPT test
func TestWPTNodeReplaceChild(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Node-replaceChild.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTManual manually steps through WPT loading to diagnose issues
func TestWPTManual(t *testing.T) {
	wptPath := "/workspaces/wpt"
	testPath := "/dom/nodes/Document-createComment.html"

	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	netClient, _ := network.NewClient(network.WithTimeout(30 * time.Second))
	loader := network.NewLoader(netClient, network.WithLocalPath(wptPath))

	// Load and parse HTML
	content, _ := os.ReadFile(wptPath + testPath)
	doc, _ := dom.ParseHTML(string(content))
	t.Log("HTML parsed")

	// Create runtime
	runtime := js.NewRuntime()
	binding := NewTestHarnessBinding(runtime)

	resultCount := 0
	binding.SetOnComplete(func(results []TestHarnessResult, status TestHarnessStatus) {
		t.Logf("OnComplete called with %d results, status=%d", len(results), status.Status)
	})

	binding.Setup()
	t.Log("Binding setup")

	// Create executor
	executor := js.NewScriptExecutor(runtime)
	executor.SetupDocument(doc)
	t.Log("Document setup")

	// DON'T load testharness.js manually - let it load via external scripts
	// This matches what a real browser does

	// Load external scripts
	baseURL := "file://" + wptPath + testPath
	loader.SetBaseURL(baseURL)
	docLoader := network.NewDocumentLoader(loader)
	loadedDoc := docLoader.LoadDocumentWithResources(context.Background(), doc, baseURL)

	t.Logf("Loaded %d scripts", len(loadedDoc.Scripts))
	for _, s := range loadedDoc.GetSyncScripts() {
		t.Logf("Executing: %s (%d bytes)", s.URL, len(s.Content))
		err := executor.ExecuteExternalScript(s.Content, s.URL)
		if err != nil {
			t.Logf("  Error: %v", err)
		}
	}

	// Register callbacks after testharness.js is loaded
	binding.SetupPostLoad()
	t.Log("SetupPostLoad called")

	// Execute inline scripts
	errors := executor.ExecuteScripts(doc)
	t.Logf("Inline scripts: %d errors", len(errors))
	for _, err := range errors {
		t.Logf("  Error: %v", err)
	}

	// Dispatch events
	executor.DispatchDOMContentLoaded()
	executor.DispatchLoadEvent()
	t.Log("Events dispatched")

	// Check current state
	t.Logf("Results so far: %d", len(binding.Results()))
	t.Logf("Completed: %v", binding.IsCompleted())

	// Run event loop
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Log("Timeout")
			goto done
		default:
			if binding.IsCompleted() {
				t.Log("Completed!")
				goto done
			}
			runtime.ProcessTimers()
			executor.RunEventLoopOnce()
			time.Sleep(10 * time.Millisecond)

			newCount := len(binding.Results())
			if newCount > resultCount {
				t.Logf("Got %d results", newCount)
				resultCount = newCount
			}
		}
	}

done:
	t.Logf("Final: Completed=%v, Results=%d", binding.IsCompleted(), len(binding.Results()))
	for _, r := range binding.Results() {
		t.Logf("  %s: status=%d", r.Name, r.Status)
	}
}

// TestWPTAttributes tests the attributes WPT test
func TestWPTAttributes(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 60 * time.Second

	result := runner.RunTestFile("/dom/nodes/attributes.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTTreeWalkerBasic tests the TreeWalker basic test
func TestWPTTreeWalkerBasic(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/traversal/TreeWalker-basic.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTTreeWalkerAll runs all TreeWalker tests from the WPT suite
func TestWPTTreeWalkerAll(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	tests := []string{
		"/dom/traversal/TreeWalker.html",
		"/dom/traversal/TreeWalker-basic.html",
		"/dom/traversal/TreeWalker-currentNode.html",
		"/dom/traversal/TreeWalker-acceptNode-filter.html",
		"/dom/traversal/TreeWalker-traversal-skip.html",
		"/dom/traversal/TreeWalker-traversal-reject.html",
		"/dom/traversal/TreeWalker-traversal-skip-most.html",
		"/dom/traversal/TreeWalker-walking-outside-a-tree.html",
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	totalPassed := 0
	totalFailed := 0

	for _, testPath := range tests {
		result := runner.RunTestFile(testPath)
		passed := 0
		failed := 0
		for _, test := range result.Tests {
			if test.Status == StatusPass {
				passed++
			} else {
				failed++
				t.Logf("  FAIL [%s] %s: %s", testPath, test.Name, test.Message)
			}
		}
		totalPassed += passed
		totalFailed += failed
		t.Logf("%s: %d passed, %d failed", testPath, passed, failed)
	}

	t.Logf("TOTAL: %d passed, %d failed", totalPassed, totalFailed)

	if totalFailed > 0 {
		t.Errorf("Some TreeWalker tests failed")
	}
}

// TestWPTTreeWalkerFilter tests TreeWalker with filter functions
func TestWPTTreeWalkerFilter(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/traversal/TreeWalker-acceptNode-filter.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTNodeFilterConstants tests the NodeFilter constants test
func TestWPTNodeFilterConstants(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/traversal/NodeFilter-constants.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if len(result.Tests) == 0 {
		t.Errorf("Expected some test results, got none")
	}
}

// TestWPTDocumentAdoptNode tests the Document.adoptNode WPT test
func TestWPTDocumentAdoptNode(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Document-adoptNode.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)

	if failed > 0 {
		t.Errorf("Some Document.adoptNode tests failed")
	}
}

// TestWPTDocumentConstructor tests the Document constructor WPT test
func TestWPTDocumentConstructor(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	runner := NewRunner(wptPath)
	runner.Timeout = 10 * time.Second

	result := runner.RunTestFile("/dom/nodes/Document-constructor.html")

	t.Logf("HarnessStatus: %s", result.HarnessStatus)
	t.Logf("Error: %s", result.Error)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Tests: %d", len(result.Tests))

	passed := 0
	failed := 0
	for _, test := range result.Tests {
		statusStr := "PASS"
		if test.Status != StatusPass {
			statusStr = "FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  [%s] %s: %s", statusStr, test.Name, test.Message)
	}

	t.Logf("Summary: %d passed, %d failed", passed, failed)
}

// TestIframeContentDocumentURL tests that iframe.contentDocument.URL is set correctly
func TestIframeContentDocumentURL(t *testing.T) {
	wptPath := "/workspaces/wpt"

	// Check if WPT exists
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available")
	}

	// Create a test document with an iframe
	doc, err := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<iframe id="test-iframe"></iframe>
</body>
</html>`)
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	// Create runtime and executor
	runtime := js.NewRuntime()
	executor := js.NewScriptExecutor(runtime)

	// Set up iframe content loader with a known URL
	expectedURL := "file://" + wptPath + "/common/blank.html"
	executor.SetIframeContentLoader(func(src string) (*dom.Document, string) {
		iframeDoc := dom.NewDocument()
		html := iframeDoc.CreateElement("html")
		body := iframeDoc.CreateElement("body")
		iframeDoc.AsNode().AppendChild(html.AsNode())
		html.AsNode().AppendChild(body.AsNode())
		// Return the document with a fixed URL
		return iframeDoc, expectedURL
	})

	executor.SetupDocument(doc)

	// Set the iframe src
	_, err = runtime.Execute(`document.getElementById('test-iframe').src = '/common/blank.html'`)
	if err != nil {
		t.Fatalf("Failed to set iframe src: %v", err)
	}

	// Get the contentDocument.URL
	result, err := runtime.Execute(`document.getElementById('test-iframe').contentDocument.URL`)
	if err != nil {
		t.Fatalf("Failed to get contentDocument.URL: %v", err)
	}

	if result.String() != expectedURL {
		t.Errorf("Expected contentDocument.URL to be '%s', got '%s'", expectedURL, result.String())
	}
}
