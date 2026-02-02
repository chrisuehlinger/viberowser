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
	skipped := 0
	for _, test := range result.Tests {
		if test.Status == StatusPass {
			passed++
		} else if test.Status == StatusSkip {
			// PRECONDITION_FAILED or NOTRUN - not a failure, just skipped
			skipped++
		} else {
			failed++
			t.Errorf("[FAIL] %s: %s", test.Name, test.Message)
		}
	}

	if len(result.Tests) == 0 {
		t.Errorf("No test results - harness may not have run properly")
	} else {
		total := passed + failed
		if total > 0 {
			t.Logf("Results: %d passed, %d failed, %d skipped (%.1f%% pass rate)",
				passed, failed, skipped, float64(passed)/float64(total)*100)
		} else {
			t.Logf("Results: %d skipped", skipped)
		}
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

		// Document tests (18 tests)
		"Document-adoptNode.html",
		"Document-constructor.html",
		"Document-createAttribute.html",
		"Document-createCDATASection.html",
		"Document-createComment.html",
		"Document-createElement.html",
		"Document-createElement-namespace.html",
		"Document-createElementNS.html",
		"Document-createEvent.https.html",
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

		// Element tests (27 tests)
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
		"Element-tagName.html",

		// MutationObserver tests (8 tests)
		"MutationObserver-attributes.html",
		"MutationObserver-characterData.html",
		"MutationObserver-childList.html",
		"MutationObserver-disconnect.html",
		"MutationObserver-inner-outer.html",
		"MutationObserver-sanity.html",
		"MutationObserver-takeRecords.html",
		"MutationObserver-textContent.html",

		// Node tests (24 tests)
		"Node-appendChild.html",
		"Node-baseURI.html",
		"Node-childNodes.html",
		"Node-childNodes-cache.html",
		"Node-childNodes-cache-2.html",
		"Node-cloneNode.html",
		"Node-cloneNode-document-with-doctype.html",
		"Node-cloneNode-svg.html",
		"Node-cloneNode-XMLDocument.html",
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
		"Node-normalize.html",
		"Node-parentElement.html",
		"Node-parentNode.html",
		"Node-removeChild.html",
		"Node-replaceChild.html",
		"Node-textContent.html",

		// NodeList tests (3 tests)
		"NodeList-Iterable.html",
		"NodeList-static-length-getter-tampered-1.html",
		"NodeList-static-length-getter-tampered-indexOf-1.html",

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

		// Other tests (12 tests)
		"append-on-Document.html",
		"attributes.html",
		"attributes-namednodemap.html",
		"case.html",
		"getElementsByClassName-32.html",
		"getElementsByClassName-empty-set.html",
		"getElementsByClassName-whitespace-class-names.html",
		"insert-adjacent.html",
		// "name-validation.html", // Commented out: takes too long (generates 10000+ test cases)
		"Node-properties.html",
		"prepend-on-Document.html",
		"querySelector-mixed-case.html",
		"remove-unscopable.html",
		"rootNode.html",
		"svg-template-querySelector.html",
	}
}

// DOMTraversalPassingTests returns the list of DOM traversal tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-02.
func DOMTraversalPassingTests() []string {
	return []string{
		"NodeFilter-constants.html",
		"NodeIterator.html",
		"NodeIterator-removal.html",
		"TreeWalker.html",
		"TreeWalker-basic.html",
		"TreeWalker-currentNode.html",
		"TreeWalker-acceptNode-filter.html",
		"TreeWalker-acceptNode-filter-cross-realm.html",
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
// This list is based on actual passing tests as of 2026-02-02.
// Note: Range-set.html is excluded because it times out (too many test cases).
// The Range-mutations-* tests are also run separately due to their size.
func DOMRangesPassingTests() []string {
	return []string{
		"Range-attributes.html",
		"Range-cloneContents.html",
		"Range-cloneRange.html",
		"Range-collapse.html",
		"Range-commonAncestorContainer-2.html",
		"Range-commonAncestorContainer.html",
		"Range-compareBoundaryPoints.html",
		"Range-comparePoint-2.html",
		"Range-constructor.html",
		"Range-deleteContents.html",
		"Range-detach.html",
		"Range-extractContents.html",
		"Range-intersectsNode.html",
		"Range-intersectsNode-2.html",
		"Range-intersectsNode-binding.html",
		"Range-intersectsNode-shadow.html",
		"Range-mutations-appendChild.html",
		"Range-mutations-insertBefore.html",
		"Range-mutations-removeChild.html",
		"Range-mutations-replaceChild.html",
		"Range-mutations-splitText.html",
		"Range-selectNode.html",
		// "Range-set.html", // Times out due to large number of test cases
	}
}

// DOMListsPassingTests returns the list of DOM lists tests that are expected to pass.
// This list is based on actual passing tests as of 2026-02-01.
func DOMListsPassingTests() []string {
	return []string{
		"DOMTokenList-Iterable.html",
		"DOMTokenList-iteration.html",
		"DOMTokenList-stringifier.html",
		"DOMTokenList-value.html",
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
// This list is based on actual passing tests as of 2026-02-02.
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
		"Event-dispatch-detached-input-and-change.html",
		"Event-dispatch-multiple-cancelBubble.html",
		"Event-dispatch-multiple-stopPropagation.html",
		"Event-dispatch-omitted-capture.html",
		"Event-dispatch-on-disabled-elements.html",
		"Event-dispatch-order-at-target.html",
		"Event-dispatch-order.html",
		"Event-dispatch-other-document.html",
		"Event-dispatch-propagation-stopped.html",
		"Event-dispatch-redispatch.html",
		"Event-dispatch-reenter.html",
		"Event-dispatch-target-moved.html",
		"Event-dispatch-target-removed.html",
		"Event-dispatch-throwing.html",
		"event-disabled-dynamic.html",
		"Event-init-while-dispatching.html",
		"Event-initEvent.html",
		"Event-propagation.html",
		"Event-returnValue.html",
		"Event-stopImmediatePropagation.html",
		"Event-stopPropagation-cancel-bubbling.html",
		"Event-timestamp-high-resolution.html",
		"Event-timestamp-safe-resolution.html",
		"EventListener-handleEvent.html",
		"EventTarget-dispatchEvent.html",
		"EventTarget-dispatchEvent-returnvalue.html",
		"EventTarget-this-of-listener.html",
		"Event-type.html",
		"Event-type-empty.html",
		"event-src-element-nullable.html",
		"label-default-action.html",
		"remove-all-listeners.html",
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

// TestWPT_DOMLists runs all passing DOM lists tests.
func TestWPT_DOMLists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMListsPassingTests()
	t.Logf("Running %d DOM lists tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/lists/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// DOMTopLevelPassingTests returns the list of top-level DOM tests that are expected to pass.
// These are tests in /dom/ (not in subdirectories like /dom/nodes).
// This list is based on actual passing tests as of 2026-02-01.
func DOMTopLevelPassingTests() []string {
	return []string{
		"eventPathRemoved.html",
		"historical.html",
		"historical-mutation-events.html",
		"interface-objects.html",
		"svg-insert-crash.html",
	}
}

// TestWPT_DOMTopLevel runs all passing top-level DOM tests.
func TestWPT_DOMTopLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := DOMTopLevelPassingTests()
	t.Logf("Running %d top-level DOM tests", len(tests))

	for _, testFile := range tests {
		testPath := "/dom/" + testFile
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

// HTMLDOMGlobalAttributesPassingTests returns the list of HTML/dom global attribute tests that pass.
// This list is based on actual passing tests as of 2026-02-02.
func HTMLDOMGlobalAttributesPassingTests() []string {
	return []string{
		// Dataset tests (6 tests)
		"dataset.html",
		"dataset-delete.html",
		"dataset-enumeration.html",
		"dataset-get.html",
		"dataset-prototype.html",
		"dataset-set.html",
	}
}

// HTMLDOMDocumentTreeAccessorsPassingTests returns the list of HTML/dom document tree accessor tests that pass.
// This list is based on actual passing tests as of 2026-02-02.
func HTMLDOMDocumentTreeAccessorsPassingTests() []string {
	return []string{
		// Document collection tests
		"document.embeds-document.plugins-01.html",
		"document.head-01.html",
		"document.scripts.html",
		// Document.title tests
		"document.title-01.html",
		"document.title-03.html",
		"document.title-05.html",
		"document.title-06.html",
		"document.title-07.html",
		"document.title-08.html",
		"document.title-09.html",
		"document.title-not-in-html-svg.html",
		// Document named item tests (document[name] access)
		"nameditem-01.html",
		"nameditem-02.html",
		"nameditem-03.html",
		"nameditem-04.html",
		"nameditem-05.html",
		"nameditem-06.html",
		"nameditem-07.html",
		"nameditem-08.html",
		// Note: nameditem-names.html has 1 failing test due to HTML parsing of void elements
	}
}

// HTMLDOMGetElementsByNamePassingTests returns the list of getElementsByName tests that pass.
// This list is based on actual passing tests as of 2026-02-02.
func HTMLDOMGetElementsByNamePassingTests() []string {
	return []string{
		"document.getElementsByName-case.html",
		"document.getElementsByName-id.html",
		"document.getElementsByName-liveness.html",
		"document.getElementsByName-same.html",
		"document.getElementsByName-null-undef.html",
		"document.getElementsByName-param.html",
		"document.getElementsByName-interface.html",
		"document.getElementsByName-newelements.html",
		"document.getElementsByName-namespace.html",
	}
}

// TestWPT_HTMLDOMGlobalAttributes runs all passing HTML/dom/elements/global-attributes tests.
func TestWPT_HTMLDOMGlobalAttributes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := HTMLDOMGlobalAttributesPassingTests()
	t.Logf("Running %d HTML/dom global attributes tests", len(tests))

	for _, testFile := range tests {
		testPath := "/html/dom/elements/global-attributes/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_HTMLDOMDocumentTreeAccessors runs all passing HTML/dom/documents/dom-tree-accessors tests.
func TestWPT_HTMLDOMDocumentTreeAccessors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := HTMLDOMDocumentTreeAccessorsPassingTests()
	t.Logf("Running %d HTML/dom document tree accessor tests", len(tests))

	for _, testFile := range tests {
		testPath := "/html/dom/documents/dom-tree-accessors/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}

// TestWPT_HTMLDOMGetElementsByName runs all passing getElementsByName tests.
func TestWPT_HTMLDOMGetElementsByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WPT tests in short mode")
	}

	wptPath := getWPTPath(t)
	runner := NewRunner(wptPath)
	runner.Timeout = 30 * time.Second

	tests := HTMLDOMGetElementsByNamePassingTests()
	t.Logf("Running %d getElementsByName tests", len(tests))

	for _, testFile := range tests {
		testPath := "/html/dom/documents/dom-tree-accessors/document.getElementsByName/" + testFile
		t.Run(testFile, func(t *testing.T) {
			runWPTTest(t, runner, testPath)
		})
	}
}
