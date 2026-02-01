package js

import (
	"testing"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestMutationObserverBasic(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Test that MutationObserver is defined
	result, err := r.Execute("typeof MutationObserver")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected MutationObserver to be 'function', got %v", result.String())
	}
}

func TestMutationObserverConstructor(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Test creating a MutationObserver
	_, err := r.Execute(`
		var observer = new MutationObserver(function(mutations) {});
		observer !== undefined
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestMutationObserverObserve(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Test calling observe
	_, err := r.Execute(`
		var observer = new MutationObserver(function(mutations) {});
		var target = document.getElementById('test');
		observer.observe(target, { childList: true });
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestMutationObserverChildListCallback(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer and perform a mutation
	_, err := r.Execute(`
		var callbackCalled = false;
		var mutationCount = 0;
		var addedNodesLength = 0;

		var observer = new MutationObserver(function(mutations) {
			callbackCalled = true;
			mutationCount = mutations.length;
			if (mutations.length > 0) {
				addedNodesLength = mutations[0].addedNodes.length;
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });

		// Add a child
		var newChild = document.createElement('span');
		target.appendChild(newChild);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop to process microtasks
	se.RunEventLoop()

	// Check if callback was called
	result, err := r.Execute("callbackCalled")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected callback to be called")
	}

	// Check mutation count
	result, err = r.Execute("mutationCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 mutation, got %d", result.ToInteger())
	}

	// Check addedNodes length
	result, err = r.Execute("addedNodesLength")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 added node, got %d", result.ToInteger())
	}
}

func TestMutationObserverDisconnect(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer, disconnect, then perform a mutation
	_, err := r.Execute(`
		var callbackCalled = false;

		var observer = new MutationObserver(function(mutations) {
			callbackCalled = true;
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });
		observer.disconnect();

		// Add a child (should not trigger callback)
		var newChild = document.createElement('span');
		target.appendChild(newChild);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop to process microtasks
	se.RunEventLoop()

	// Check if callback was NOT called
	result, err := r.Execute("callbackCalled")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected callback NOT to be called after disconnect")
	}
}

func TestMutationObserverTakeRecords(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer and perform a mutation, then takeRecords
	_, err := r.Execute(`
		var callbackCalled = false;
		var takenRecords = [];

		var observer = new MutationObserver(function(mutations) {
			callbackCalled = true;
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });

		// Add a child
		var newChild = document.createElement('span');
		target.appendChild(newChild);

		// Take records before event loop processes them
		takenRecords = observer.takeRecords();
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check taken records length
	result, err := r.Execute("takenRecords.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 taken record, got %d", result.ToInteger())
	}

	// Run the event loop
	se.RunEventLoop()

	// Check that callback was NOT called (records were taken)
	result, err = r.Execute("callbackCalled")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected callback NOT to be called after takeRecords")
	}
}

func TestMutationObserverRemovedNodes(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"><span id="child">Hello</span></div></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer and remove a child
	_, err := r.Execute(`
		var removedNodesLength = 0;

		var observer = new MutationObserver(function(mutations) {
			if (mutations.length > 0) {
				removedNodesLength = mutations[0].removedNodes.length;
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });

		// Remove the child
		var child = document.getElementById('child');
		target.removeChild(child);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check removedNodes length
	result, err := r.Execute("removedNodesLength")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 removed node, got %d", result.ToInteger())
	}
}

func TestMutationObserverSubtree(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"><div id="inner"></div></div></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer with subtree option
	_, err := r.Execute(`
		var callbackCalled = false;

		var observer = new MutationObserver(function(mutations) {
			callbackCalled = true;
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true, subtree: true });

		// Add a child to the inner div (should trigger because of subtree)
		var inner = document.getElementById('inner');
		var newChild = document.createElement('span');
		inner.appendChild(newChild);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check if callback was called
	result, err := r.Execute("callbackCalled")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected callback to be called with subtree option")
	}
}

func TestMutationObserverInnerHTML(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><p id="test">old text</p></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer and use innerHTML
	_, err := r.Execute(`
		var recordCount = 0;
		var records = [];

		var observer = new MutationObserver(function(mutations) {
			recordCount = mutations.length;
			for (var i = 0; i < mutations.length; i++) {
				records.push({
					type: mutations[i].type,
					addedCount: mutations[i].addedNodes.length,
					removedCount: mutations[i].removedNodes.length
				});
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });

		// Replace content with innerHTML
		target.innerHTML = "new text";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check that only 1 mutation record was generated for innerHTML
	result, err := r.Execute("recordCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 mutation record for innerHTML, got %d", result.ToInteger())
	}

	// Check that the single record has both addedNodes and removedNodes
	result, err = r.Execute("records[0].addedCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 added node, got %d", result.ToInteger())
	}

	result, err = r.Execute("records[0].removedCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 removed node, got %d", result.ToInteger())
	}
}

func TestMutationObserverInnerHTMLWithMultipleChildren(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><p id="test">old text</p></body>
</html>`)

	se.SetupDocument(doc)

	// Set up observer and use innerHTML to add multiple children
	_, err := r.Execute(`
		var recordCount = 0;
		var records = [];

		var observer = new MutationObserver(function(mutations) {
			recordCount = mutations.length;
			for (var i = 0; i < mutations.length; i++) {
				records.push({
					type: mutations[i].type,
					addedCount: mutations[i].addedNodes.length,
					removedCount: mutations[i].removedNodes.length
				});
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true });

		// Replace content with multiple children
		target.innerHTML = "<span>new</span><span>text</span>";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check that only 1 mutation record was generated for innerHTML
	result, err := r.Execute("recordCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 mutation record for innerHTML, got %d", result.ToInteger())
	}

	// Check that the single record has 2 addedNodes and 1 removedNode
	result, err = r.Execute("records[0].addedCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2 added nodes, got %d", result.ToInteger())
	}

	result, err = r.Execute("records[0].removedCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 removed node, got %d", result.ToInteger())
	}
}

func TestMutationObserverNamespacedAttributeName(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><p id="test"></p></body>
</html>`)

	se.SetupDocument(doc)

	// Test that attributeName is the localName, not the qualified name
	_, err := r.Execute(`
		var receivedAttributeName = null;
		var receivedAttributeNamespace = null;

		var observer = new MutationObserver(function(mutations) {
			if (mutations.length > 0) {
				receivedAttributeName = mutations[0].attributeName;
				receivedAttributeNamespace = mutations[0].attributeNamespace;
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { attributes: true, attributeOldValue: true });

		// Set a namespaced attribute with prefix
		target.setAttributeNS("http://www.w3.org/XML/1998/namespace", "xml:lang", "42");
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check attributeName - should be "lang" (localName), not "xml:lang" (qualified name)
	result, err := r.Execute("receivedAttributeName")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "lang" {
		t.Errorf("Expected attributeName to be 'lang', got '%s'", result.String())
	}

	// Check attributeNamespace
	result, err = r.Execute("receivedAttributeNamespace")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "http://www.w3.org/XML/1998/namespace" {
		t.Errorf("Expected attributeNamespace to be 'http://www.w3.org/XML/1998/namespace', got '%s'", result.String())
	}
}

func TestMutationObserverInnerHTMLAndAttribute(t *testing.T) {
	r := NewRuntime()
	se := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><p id="test">old text</p></body>
</html>`)

	se.SetupDocument(doc)

	// Replicate the WPT test: innerHTML + className change
	_, err := r.Execute(`
		var recordCount = 0;
		var records = [];
		var totalCalls = 0;

		var observer = new MutationObserver(function(mutations) {
			totalCalls++;
			recordCount += mutations.length;
			for (var i = 0; i < mutations.length; i++) {
				records.push({
					type: mutations[i].type,
					target: mutations[i].target.id || mutations[i].target.nodeName,
					addedCount: mutations[i].addedNodes ? mutations[i].addedNodes.length : 0,
					removedCount: mutations[i].removedNodes ? mutations[i].removedNodes.length : 0,
					attributeName: mutations[i].attributeName
				});
			}
		});

		var target = document.getElementById('test');
		observer.observe(target, { childList: true, attributes: true });

		// Do the same operations as WPT test
		target.innerHTML = "new text";
		target.className = "c01";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Run the event loop
	se.RunEventLoop()

	// Check that exactly 2 mutation records were generated: 1 childList + 1 attributes
	result, err := r.Execute("recordCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		// Print out what we got for debugging
		r.Execute(`
			console.log("Total calls: " + totalCalls);
			console.log("Got " + recordCount + " records:");
			for (var i = 0; i < records.length; i++) {
				console.log("  Record " + i + ": type=" + records[i].type +
					" target=" + records[i].target +
					" added=" + records[i].addedCount +
					" removed=" + records[i].removedCount +
					" attr=" + records[i].attributeName);
			}
		`)
		se.RunEventLoop()
		t.Errorf("Expected 2 mutation records, got %d", result.ToInteger())
	}
}
