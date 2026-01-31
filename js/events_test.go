package js

import (
	"testing"

	"github.com/AYColumbia/viberowser/dom"
)

func TestEventBasic(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var clicked = false;
		document.addEventListener('click', function() {
			clicked = true;
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event
	_, err = r.Execute(`
		var event = new Event('click');
		document.dispatchEvent(event);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("clicked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Event listener was not called")
	}
}

func TestEventRemoveListener(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var count = 0;
		function handler() {
			count++;
		}
		document.addEventListener('test', handler);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event
	_, err = r.Execute(`
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("count")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected count = 1, got %v", result.ToInteger())
	}

	// Remove listener and dispatch again
	_, err = r.Execute(`
		document.removeEventListener('test', handler);
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("count")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected count = 1 (unchanged), got %v", result.ToInteger())
	}
}

func TestEventOnce(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var count = 0;
		document.addEventListener('test', function() {
			count++;
		}, { once: true });
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event twice
	_, err = r.Execute(`
		document.dispatchEvent(new Event('test'));
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("count")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected count = 1 (once only), got %v", result.ToInteger())
	}
}

func TestEventPreventDefault(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var result = null;
		document.addEventListener('test', function(e) {
			e.preventDefault();
			result = e.defaultPrevented;
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch cancelable event
	_, err = r.Execute(`
		var event = new Event('test', { cancelable: true });
		document.dispatchEvent(event);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("result")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected defaultPrevented to be true")
	}
}

func TestEventStopImmediatePropagation(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var calls = [];
		document.addEventListener('test', function(e) {
			calls.push(1);
			e.stopImmediatePropagation();
		});
		document.addEventListener('test', function(e) {
			calls.push(2);
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event
	_, err = r.Execute(`
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("calls.join(',')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "1" {
		t.Errorf("Expected '1', got %v", result.String())
	}
}

func TestCustomEvent(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var receivedDetail = null;
		document.addEventListener('custom', function(e) {
			receivedDetail = e.detail;
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch custom event with detail
	_, err = r.Execute(`
		var event = new CustomEvent('custom', { detail: { foo: 'bar' } });
		document.dispatchEvent(event);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("receivedDetail.foo")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "bar" {
		t.Errorf("Expected 'bar', got %v", result.String())
	}
}

func TestEventMultipleListeners(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var calls = [];
		document.addEventListener('test', function() { calls.push('a'); });
		document.addEventListener('test', function() { calls.push('b'); });
		document.addEventListener('test', function() { calls.push('c'); });
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event
	_, err = r.Execute(`
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("calls.join(',')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "a,b,c" {
		t.Errorf("Expected 'a,b,c', got %v", result.String())
	}
}

func TestEventDuplicateListener(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	_, err := r.Execute(`
		var count = 0;
		function handler() { count++; }
		document.addEventListener('test', handler);
		document.addEventListener('test', handler); // Duplicate - should be ignored
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Dispatch event
	_, err = r.Execute(`
		document.dispatchEvent(new Event('test'));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("count")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected count = 1 (duplicate ignored), got %v", result.ToInteger())
	}
}

// TestHTMLElementClick tests that HTMLElement.click() dispatches a click event
func TestHTMLElementClick(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="target">Click me</div>
</body>
</html>`)

	executor.SetupDocument(doc)

	// Test that click method exists
	result, err := r.Execute(`
		var target = document.getElementById('target');
		typeof target.click
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected click to be a function, got %v", result.String())
	}

	// Test that click() dispatches an event
	_, err = r.Execute(`
		var clicked = false;
		var target = document.getElementById('target');
		target.addEventListener('click', function(e) {
			clicked = true;
		});
		target.click();
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("clicked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("click() should have dispatched a click event")
	}

	// Test that the event has correct properties
	_, err = r.Execute(`
		var isBubbling = null;
		var isCancelable = null;
		var target = document.getElementById('target');
		target.addEventListener('click', function(e) {
			isBubbling = e.bubbles;
			isCancelable = e.cancelable;
		});
		target.click();
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check that it's a MouseEvent with bubbles and cancelable true
	result, err = r.Execute("isBubbling")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("click event should bubble")
	}

	result, err = r.Execute("isCancelable")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("click event should be cancelable")
	}
}

func TestCheckboxActivationBehavior(t *testing.T) {
	se := NewScriptExecutor(NewRuntime())
	doc := dom.NewDocument()
	// Add HTML structure with body
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	doc.Append(html)
	html.AsNode().AppendChild(body.AsNode())
	se.SetupDocument(doc)

	// Create a checkbox input
	_, err := se.Runtime().Execute(`
		var input = document.createElement('input');
		input.type = 'checkbox';
		document.body.appendChild(input);
		var checkedDuringClick = null;
		var inputFired = false;
		var changeFired = false;
		input.onclick = function() { checkedDuringClick = input.checked; };
		input.oninput = function() { inputFired = true; };
		input.onchange = function() { changeFired = true; };
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Click the checkbox
	_, err = se.Runtime().Execute(`input.click();`)
	if err != nil {
		t.Fatalf("Click failed: %v", err)
	}

	// Check that checkbox is checked after click
	result, err := se.Runtime().Execute("input.checked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Checkbox should be checked after click")
	}

	// Check that checkbox was already checked during onclick
	result, err = se.Runtime().Execute("checkedDuringClick")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Checkbox should be checked during onclick handler")
	}

	// Check that input and change events fired
	result, err = se.Runtime().Execute("inputFired")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("input event should fire after checkbox click")
	}

	result, err = se.Runtime().Execute("changeFired")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("change event should fire after checkbox click")
	}
}

func TestCheckboxPreventDefault(t *testing.T) {
	se := NewScriptExecutor(NewRuntime())
	doc := dom.NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	doc.Append(html)
	html.AsNode().AppendChild(body.AsNode())
	se.SetupDocument(doc)

	// Create a checkbox input and prevent default on click
	_, err := se.Runtime().Execute(`
		var input = document.createElement('input');
		input.type = 'checkbox';
		document.body.appendChild(input);
		input.onclick = function(e) { e.preventDefault(); };
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Click the checkbox (with MouseEvent to trigger activation)
	_, err = se.Runtime().Execute(`input.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));`)
	if err != nil {
		t.Fatalf("Click failed: %v", err)
	}

	// Check that checkbox is NOT checked after preventDefault
	result, err := se.Runtime().Execute("input.checked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Checkbox should NOT be checked after preventDefault")
	}
}

func TestDisabledCheckboxClick(t *testing.T) {
	se := NewScriptExecutor(NewRuntime())
	doc := dom.NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	doc.Append(html)
	html.AsNode().AppendChild(body.AsNode())
	se.SetupDocument(doc)

	// Create a disabled checkbox
	_, err := se.Runtime().Execute(`
		var input = document.createElement('input');
		input.type = 'checkbox';
		input.disabled = true;
		document.body.appendChild(input);
		var clicked = false;
		input.onclick = function() { clicked = true; };
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Click() on disabled element should do nothing
	_, err = se.Runtime().Execute(`input.click();`)
	if err != nil {
		t.Fatalf("Click failed: %v", err)
	}

	// Check that onclick was NOT called
	result, err := se.Runtime().Execute("clicked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("click() on disabled element should not fire click event")
	}

	// Check that checkbox is NOT checked
	result, err = se.Runtime().Execute("input.checked")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("disabled checkbox should NOT be checked after click()")
	}
}
