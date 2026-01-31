package js

import (
	"testing"

	"github.com/AYColumbia/viberowser/dom"
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
