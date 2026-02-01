// Package wpt provides Web Platform Test integration.
//
// This file (wpt_suite_test.go) provides systematic WPT test running as part of
// the Go test suite. These tests are designed to be run with `go test ./wpt/...`.
//
// The tests are organized into suites:
//   - TestWPT_DOMNodes: Runs all passing DOM node tests from /dom/nodes/
//   - TestWPT_DOMTraversal: Runs all passing DOM traversal tests from /dom/traversal/
//   - TestWPT_DOMRanges: Runs all passing DOM range tests from /dom/ranges/
//   - TestWPT_DOMEvents: Runs all passing DOM event tests from /dom/events/
//   - TestWPT_DOMCollections: Runs all passing DOM collections tests from /dom/collections/
//
// To discover all WPT tests and see current pass/fail status:
//
//	WPT_DISCOVER=1 go test -v ./wpt/... -run TestWPT_DiscoverAll
//
// The tests are skipped in short mode (`go test -short`).
//
// See wpttest_test.go for debug/development test functions.
package wpt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// WPTTestSuite defines a collection of WPT test files to run.
type WPTTestSuite struct {
	Name     string   // Name of the suite (e.g., "DOM Nodes")
	BasePath string   // Base path relative to WPT root (e.g., "/dom/nodes")
	Tests    []string // List of test file names (without path prefix)
}

// getWPTPath returns the path to the WPT repository, or skips if not available.
func getWPTPath(t *testing.T) string {
	wptPath := "/workspaces/wpt"
	if _, err := os.Stat(wptPath); os.IsNotExist(err) {
		t.Skip("WPT not available at /workspaces/wpt")
	}
	return wptPath
}

// runWPTTest runs a single WPT test file as a Go subtest.
func runWPTTest(t *testing.T, runner *Runner, testPath string) {
	result := runner.RunTestFile(testPath)

	// Log harness status
	if result.HarnessStatus != "" && result.HarnessStatus != "OK" {
		t.Logf("Harness status: %s", result.HarnessStatus)
	}
	if result.Error != "" {
		t.Errorf("Harness error: %s", result.Error)
		return
	}

	// Track results
	passed := 0
	failed := 0
	for _, test := range result.Tests {
		if test.Status == StatusPass {
			passed++
		} else {
			failed++
			t.Errorf("[FAIL] %s: %s", test.Name, test.Message)
		}
	}

	if len(result.Tests) == 0 {
		t.Errorf("No test results - harness may not have run properly")
	} else {
		t.Logf("Results: %d passed, %d failed (%.1f%%)", passed, failed,
			float64(passed)/float64(passed+failed)*100)
	}
}

// DOMNodesPassingTests returns the list of DOM node tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
// Run with WPT_DISCOVER=1 to regenerate this list.
func DOMNodesPassingTests() []string {
	return []string{
		// CharacterData tests (9 tests)
		"CharacterData-appendChild.html",
		"CharacterData-appendData.html",
		"CharacterData-data.html",
		"CharacterData-deleteData.html",
		"CharacterData-insertData.html",
		"CharacterData-remove.html",
		"CharacterData-replaceData.html",
		"CharacterData-substringData.html",
		"CharacterData-surrogates.html",

		// ChildNode tests (3 tests)
		"ChildNode-after.html",
		"ChildNode-before.html",
		"ChildNode-replaceWith.html",

		// Comment tests (1 test)
		"Comment-constructor.html",

		// DOMImplementation tests (5 tests)
		"DOMImplementation-createDocument.html",
		"DOMImplementation-createDocumentType.html",
		"DOMImplementation-createHTMLDocument.html",
		"DOMImplementation-createHTMLDocument-with-saved-implementation.html",
		"DOMImplementation-hasFeature.html",

		// Document tests (17 tests)
		"Document-adoptNode.html",
		"Document-constructor.html",
		"Document-createAttribute.html",
		"Document-createCDATASection.html",
		"Document-createComment.html",
		"Document-createElement.html",
		"Document-createElement-namespace.html",
		"Document-createElementNS.html",
		"Document-createProcessingInstruction.html",
		"Document-createTextNode.html",
		"Document-createTreeWalker.html",
		"Document-doctype.html",
		"Document-getElementById.html",
		"Document-getElementsByClassName.html",
		"Document-getElementsByTagName.html",
		"Document-getElementsByTagNameNS.html",
		"Document-implementation.html",
		"Document-importNode.html",

		// DocumentFragment tests (3 tests)
		"DocumentFragment-constructor.html",
		"DocumentFragment-getElementById.html",
		"DocumentFragment-querySelectorAll-after-modification.html",

		// DocumentType tests (2 tests)
		"DocumentType-literal.html",
		"DocumentType-remove.html",

		// Element tests (26 tests)
		"Element-childElement-null.html",
		"Element-childElementCount.html",
		"Element-childElementCount-dynamic-add.html",
		"Element-childElementCount-dynamic-remove.html",
		"Element-childElementCount-nochild.html",
		"Element-children.html",
		"Element-classlist.html",
		"Element-closest.html",
		"Element-firstElementChild.html",
		"Element-firstElementChild-namespace.html",
		"Element-getElementsByClassName.html",
		"Element-getElementsByTagName.html",
		"Element-getElementsByTagNameNS.html",
		"Element-hasAttribute.html",
		"Element-hasAttributes.html",
		"Element-insertAdjacentElement.html",
		"Element-insertAdjacentText.html",
		"Element-lastElementChild.html",
		"Element-matches-namespaced-elements.html",
		"Element-nextElementSibling.html",
		"Element-previousElementSibling.html",
		"Element-remove.html",
		"Element-removeAttribute.html",
		"Element-removeAttributeNS.html",
		"Element-setAttribute.html",
		"Element-setAttribute-crbug-1138487.html",
		"Element-siblingElement-null.html",

		// MutationObserver tests (5 tests)
		"MutationObserver-childList.html",
		"MutationObserver-inner-outer.html",
		"MutationObserver-sanity.html",
		"MutationObserver-takeRecords.html",
		"MutationObserver-textContent.html",

		// Node tests (22 tests)
		"Node-appendChild.html",
		"Node-baseURI.html",
		"Node-childNodes.html",
		"Node-childNodes-cache.html",
		"Node-childNodes-cache-2.html",
		"Node-cloneNode.html",
		"Node-cloneNode-document-with-doctype.html",
		"Node-compareDocumentPosition.html",
		"Node-constants.html",
		"Node-contains.html",
		"Node-insertBefore.html",
		"Node-isConnected.html",
		"Node-isConnected-shadow-dom.html",
		"Node-isEqualNode.html",
		"Node-isSameNode.html",
		"Node-lookupNamespaceURI.html",
		"Node-mutation-adoptNode.html",
		"Node-nodeName.html",
		"Node-nodeValue.html",
		"Node-parentElement.html",
		"Node-parentNode.html",
		"Node-removeChild.html",
		"Node-replaceChild.html",
		"Node-textContent.html",

		// ParentNode tests (9 tests)
		"ParentNode-append.html",
		"ParentNode-children.html",
		"ParentNode-prepend.html",
		"ParentNode-querySelector-case-insensitive.html",
		"ParentNode-querySelector-scope.html",
		"ParentNode-querySelectorAll-removed-elements.html",
		"ParentNode-querySelectors-exclusive.html",
		"ParentNode-querySelectors-namespaces.html",
		"ParentNode-querySelectors-space-and-dash-attribute-value.html",
		"ParentNode-replaceChildren.html",

		// Text tests (3 tests)
		"Text-constructor.html",
		"Text-splitText.html",
		"Text-wholeText.html",

		// Other tests (11 tests)
		"append-on-Document.html",
		"attributes.html",
		"attributes-namednodemap.html",
		"case.html",
		"getElementsByClassName-32.html",
		"getElementsByClassName-empty-set.html",
		"getElementsByClassName-whitespace-class-names.html",
		"insert-adjacent.html",
		"name-validation.html",
		"Node-properties.html",
		"prepend-on-Document.html",
		"remove-unscopable.html",
		"rootNode.html",
		"svg-template-querySelector.html",
	}
}

// DOMTraversalPassingTests returns the list of DOM traversal tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
func DOMTraversalPassingTests() []string {
	return []string{
		"NodeFilter-constants.html",
		"NodeIterator.html",
		"NodeIterator-removal.html",
		"TreeWalker.html",
		"TreeWalker-basic.html",
		"TreeWalker-currentNode.html",
		"TreeWalker-acceptNode-filter.html",
		"TreeWalker-previousNodeLastChildReject.html",
		"TreeWalker-previousSiblingLastChildSkip.html",
		"TreeWalker-realm.html",
		"TreeWalker-traversal-skip.html",
		"TreeWalker-traversal-reject.html",
		"TreeWalker-traversal-skip-most.html",
		"TreeWalker-walking-outside-a-tree.html",
	}
}

// DOMRangesPassingTests returns the list of DOM range tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
func DOMRangesPassingTests() []string {
	return []string{
		"Range-attributes.html",
		"Range-cloneContents.html",
		"Range-cloneRange.html",
		"Range-collapse.html",
		"Range-commonAncestorContainer-2.html",
		"Range-commonAncestorContainer.html",
		"Range-comparePoint-2.html",
		"Range-constructor.html",
		"Range-detach.html",
		"Range-selectNode.html",
		"Range-stringifier.html",
	}
}

// DOMCollectionsPassingTests returns the list of DOM collections tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
func DOMCollectionsPassingTests() []string {
	return []string{
		"HTMLCollection-delete.html",
		"HTMLCollection-empty-name.html",
		"HTMLCollection-iterator.html",
		"HTMLCollection-own-props.html",
		"HTMLCollection-supported-property-indices.html",
		"HTMLCollection-supported-property-names.html",
		"namednodemap-supported-property-names.html",
	}
}

// DOMEventsPassingTests returns the list of DOM event tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
func DOMEventsPassingTests() []string {
	return []string{
		"CustomEvent.html",
		"Event-cancelBubble.html",
		"Event-constants.html",
		"Event-defaultPrevented-after-dispatch.html",
		"Event-defaultPrevented.html",
		"Event-dispatch-bubble-canceled.html",
		"Event-dispatch-bubbles-false.html",
		"Event-dispatch-bubbles-true.html",
		"Event-dispatch-click.tentative.html",
		"Event-dispatch-detached-click.html",
		"Event-dispatch-multiple-cancelBubble.html",
		"Event-dispatch-multiple-stopPropagation.html",
		"Event-dispatch-omitted-capture.html",
		"Event-dispatch-order-at-target.html",
		"Event-dispatch-order.html",
		"Event-dispatch-other-document.html",
		"Event-dispatch-propagation-stopped.html",
		"Event-dispatch-reenter.html",
		"Event-dispatch-target-moved.html",
		"Event-dispatch-target-removed.html",
		"Event-initEvent.html",
		"Event-propagation.html",
		"Event-returnValue.html",
		"Event-stopImmediatePropagation.html",
	}
}

// TestWPT_DOMNodes runs all passing DOM node tests.
func TestWPT_DOMNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMNodesPassingTests()
	t.Logf("Running %d DOM node tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/nodes/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_DOMTraversal runs all passing DOM traversal tests.
func TestWPT_DOMTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMTraversalPassingTests()
	t.Logf("Running %d DOM traversal tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/traversal/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_DOMRanges runs all passing DOM range tests.
func TestWPT_DOMRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMRangesPassingTests()
	t.Logf("Running %d DOM range tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/ranges/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_DOMEvents runs all passing DOM event tests.
func TestWPT_DOMEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMEventsPassingTests()
	t.Logf("Running %d DOM event tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/events/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_DOMCollections runs all passing DOM collections tests.
func TestWPT_DOMCollections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMCollectionsPassingTests()
	t.Logf("Running %d DOM collections tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/collections/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_DiscoverAll discovers and reports on all WPT tests in dom/nodes and dom/traversal.
// This is primarily for discovery - it logs pass/fail without failing the test.
func TestWPT_DiscoverAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT discovery in short mode")
	}

	// Only run this test when explicitly requested
	if os.Getenv("WPT_DISCOVER") != "1" {
		t.Skip("Set WPT_DISCOVER=1 to run WPT discovery")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	dirs := []string{
		"/dom/nodes",
		"/dom/traversal",
		"/dom/ranges",
		"/dom/events",
	}

	for _, dir := range dirs {
		t.Run(strings.TrimPrefix(dir, "/"), func(t *testing.T) {
			fullPath := filepath.Join(wptPath, dir)
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				t.Fatalf("Failed to read directory: %v", err)
			}

			totalPassed := 0
			totalFailed := 0
			passingTests := []string{}
			failingTests := []string{}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
					continue
				}

				testPath := dir + "/" + entry.Name()
				result := runner.RunTestFile(testPath)

				passed := 0
				failed := 0
				for _, test := range result.Tests {
					if test.Status == StatusPass {
						passed++
					} else {
						failed++
					}
				}

				if failed == 0 && passed > 0 {
					totalPassed++
					passingTests = append(passingTests, entry.Name())
				} else {
					totalFailed++
					failingTests = append(failingTests, entry.Name())
				}
			}

			t.Logf("Directory %s: %d passing, %d failing", dir, totalPassed, totalFailed)
			t.Logf("Pass rate: %.1f%%", float64(totalPassed)/float64(totalPassed+totalFailed)*100)

			t.Log("\n=== PASSING TESTS ===")
			for _, name := range passingTests {
				t.Logf("  %s", name)
			}

			t.Log("\n=== FAILING TESTS ===")
			for _, name := range failingTests {
				t.Logf("  %s", name)
			}
		})
	}
}
