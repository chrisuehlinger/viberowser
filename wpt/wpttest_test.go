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
