package js

import (
	"testing"

	"github.com/chrisuehlinger/viberowser/dom"
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

func TestWheelEventConstructor(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test basic WheelEvent constructor
	result, err := r.Execute(`
		var event = new WheelEvent('wheel');
		event instanceof WheelEvent;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent constructor should create a WheelEvent instance")
	}

	// Test WheelEvent extends MouseEvent
	result, err = r.Execute(`event instanceof MouseEvent`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent should extend MouseEvent")
	}

	// Test WheelEvent extends UIEvent
	result, err = r.Execute(`event instanceof UIEvent`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent should extend UIEvent")
	}

	// Test default property values
	result, err = r.Execute(`event.deltaX === 0.0 && event.deltaY === 0.0 && event.deltaZ === 0.0`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent should have default delta values of 0")
	}

	result, err = r.Execute(`event.deltaMode === 0`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent deltaMode should default to 0 (DOM_DELTA_PIXEL)")
	}

	// Test WheelEvent with options
	result, err = r.Execute(`
		var event2 = new WheelEvent('wheel', {
			deltaX: 3.1,
			deltaY: 3.1,
			deltaZ: 3.1,
			deltaMode: 40
		});
		event2.deltaX === 3.1 && event2.deltaY === 3.1 && event2.deltaZ === 3.1 && event2.deltaMode === 40;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent should accept delta options")
	}

	// Test inherited MouseEvent properties
	result, err = r.Execute(`
		var event3 = new WheelEvent('wheel', {
			clientX: 40,
			clientY: 40,
			screenX: 40,
			screenY: 40,
			button: 40,
			buttons: 40,
			ctrlKey: true
		});
		event3.clientX === 40 && event3.screenX === 40 && event3.button === 40 && event3.ctrlKey === true;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("WheelEvent should inherit MouseEvent properties")
	}
}

func TestAbortControllerBasic(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	// Test AbortController creation and that signal has addEventListener
	result, err := r.Execute(`
		var controller = new AbortController();
		typeof controller.signal.addEventListener === 'function';
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal should have addEventListener method")
	}

	// Test initial signal state
	result, err = r.Execute(`
		controller.signal.aborted === false;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal should start as non-aborted")
	}

	// Test abort
	result, err = r.Execute(`
		controller.abort();
		controller.signal.aborted === true;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal should be aborted after controller.abort()")
	}

	// Test abort reason
	result, err = r.Execute(`
		var controller2 = new AbortController();
		controller2.abort("test reason");
		controller2.signal.reason === "test reason";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal.reason should match the abort reason")
	}
}

func TestAbortSignalRemovesListener(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	// Test that aborting removes the listener
	result, err := r.Execute(`
		var count = 0;
		var controller = new AbortController();
		document.addEventListener('test', function() { count++; }, { signal: controller.signal });

		// First dispatch should work
		document.dispatchEvent(new Event('test'));
		var countAfterFirst = count;

		// Abort the controller
		controller.abort();

		// Second dispatch should not call the handler
		document.dispatchEvent(new Event('test'));
		var countAfterSecond = count;

		countAfterFirst === 1 && countAfterSecond === 1;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Aborting should remove the event listener")
	}
}

func TestAbortSignalAlreadyAborted(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	// Test that an already-aborted signal prevents adding the listener
	result, err := r.Execute(`
		var count = 0;
		var controller = new AbortController();
		controller.abort();  // Abort before adding listener

		document.addEventListener('test', function() { count++; }, { signal: controller.signal });
		document.dispatchEvent(new Event('test'));

		count === 0;  // Listener should not have been added
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Already-aborted signal should prevent adding listener")
	}
}

func TestAbortSignalAbortStatic(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test AbortSignal.abort() static method
	result, err := r.Execute(`
		var signal = AbortSignal.abort();
		signal.aborted === true;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal.abort() should return an already-aborted signal")
	}

	// Test AbortSignal.abort(reason)
	result, err = r.Execute(`
		var signal2 = AbortSignal.abort("custom reason");
		signal2.aborted === true && signal2.reason === "custom reason";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("AbortSignal.abort(reason) should set the abort reason")
	}
}

func TestAbortSignalThrowIfAborted(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test throwIfAborted on non-aborted signal
	result, err := r.Execute(`
		var controller = new AbortController();
		var threw = false;
		try {
			controller.signal.throwIfAborted();
		} catch (e) {
			threw = true;
		}
		threw === false;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("throwIfAborted should not throw on non-aborted signal")
	}

	// Test throwIfAborted on aborted signal
	result, err = r.Execute(`
		var controller2 = new AbortController();
		controller2.abort("test");
		var threw2 = false;
		var reason2 = null;
		try {
			controller2.signal.throwIfAborted();
		} catch (e) {
			threw2 = true;
			reason2 = e;
		}
		threw2 === true && reason2 === "test";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("throwIfAborted should throw the abort reason on aborted signal")
	}
}

func TestAbortSignalOnAbortHandler(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test onabort event handler
	result, err := r.Execute(`
		var controller = new AbortController();
		var onabortCalled = false;
		controller.signal.onabort = function() { onabortCalled = true; };
		controller.abort();
		onabortCalled === true;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("onabort handler should be called when signal is aborted")
	}
}

func TestAbortSignalWithOnce(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)
	eventBinder.BindEventTarget(r.vm.Get("document").ToObject(r.vm))

	// Test signal option combined with once option
	result, err := r.Execute(`
		var count = 0;
		var controller = new AbortController();
		document.addEventListener('test', function() { count++; }, { signal: controller.signal, once: true });

		// First dispatch should work and remove listener due to once
		document.dispatchEvent(new Event('test'));

		// Second dispatch should not work (listener was removed by once)
		document.dispatchEvent(new Event('test'));

		count === 1;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("signal option should work together with once option")
	}
}

func TestEventTargetConstructor(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test that EventTarget constructor exists and creates usable objects
	result, err := r.Execute(`
		var et = new EventTarget();
		typeof et.addEventListener === 'function' &&
		typeof et.removeEventListener === 'function' &&
		typeof et.dispatchEvent === 'function';
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("EventTarget should have all required methods")
	}

	// Test using EventTarget like in WPT tests
	result, err = r.Execute(`
		var count = 0;
		function handler() { count++; }
		var et = new EventTarget();
		var controller = new AbortController();
		et.addEventListener('test', handler, { signal: controller.signal });
		et.dispatchEvent(new Event('test'));
		var countAfterFirst = count; // Should be 1
		et.dispatchEvent(new Event('test'));
		var countAfterSecond = count; // Should be 2
		controller.abort();
		et.dispatchEvent(new Event('test'));
		var countAfterAbort = count; // Should still be 2
		et.addEventListener('test', handler, { signal: controller.signal });
		et.dispatchEvent(new Event('test'));
		var countAfterReAdd = count; // Should still be 2 (signal already aborted)

		countAfterFirst === 1 && countAfterSecond === 2 && countAfterAbort === 2 && countAfterReAdd === 2;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("EventTarget with AbortController should work as in WPT tests")
	}
}

func TestAbortSignalNullThrows(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test that passing null as signal throws TypeError
	result, err := r.Execute(`
		var et = new EventTarget();
		var threw = false;
		var errorType = "";
		try {
			et.addEventListener("foo", function() {}, { signal: null });
		} catch (e) {
			threw = true;
			errorType = e.name || e.constructor.name;
		}
		threw && errorType === "TypeError";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Passing null as signal should throw TypeError")
	}
}

func TestAbortDuringDispatch(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test that aborting from a listener removes future listeners
	result, err := r.Execute(`
		var count = 0;
		function handler() {
			count++;
		}
		var et = new EventTarget();
		var controller = new AbortController();
		// First listener aborts the controller
		et.addEventListener('test', function() {
			controller.abort();
		}, { signal: controller.signal });
		// Second listener should not be called because controller was aborted
		et.addEventListener('test', handler, { signal: controller.signal });
		et.dispatchEvent(new Event('test'));
		count === 0;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Aborting from a listener should remove future listeners")
	}
}

func TestAbortWithMultipleEvents(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test that a single abort removes listeners from multiple event types
	result, err := r.Execute(`
		var count = 0;
		function handler() {
			count++;
		}
		var et = new EventTarget();
		var controller = new AbortController();
		et.addEventListener('first', handler, { signal: controller.signal, once: true });
		et.addEventListener('second', handler, { signal: controller.signal, once: true });
		controller.abort();
		et.dispatchEvent(new Event('first'));
		et.dispatchEvent(new Event('second'));
		count === 0;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Aborting should remove listeners from multiple event types")
	}
}

func TestAbortWithCaptureFlag(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test signal option with capture flag
	result, err := r.Execute(`
		var count = 0;
		function handler() {
			count++;
		}
		var et = new EventTarget();
		var controller = new AbortController();
		et.addEventListener('test', handler, { signal: controller.signal, capture: true });
		controller.abort();
		et.dispatchEvent(new Event('test'));
		count === 0;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Signal option should work with capture flag")
	}
}

func TestAbortSignalRemoveEventListenerStillWorks(t *testing.T) {
	r := NewRuntime()
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	// Test that removeEventListener still works with signal option
	result, err := r.Execute(`
		var count = 0;
		function handler() {
			count++;
		}
		var et = new EventTarget();
		var controller = new AbortController();
		et.addEventListener('test', handler, { signal: controller.signal });
		et.removeEventListener('test', handler);
		et.dispatchEvent(new Event('test'));
		count === 0;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("removeEventListener should still work with signal option")
	}
}

// TestWindowEventLegacy tests that window.event is set during event dispatch
func TestWindowEventLegacy(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html><html><body></body></html>`)
	executor.SetupDocument(doc)

	// Test 1: window.event exists and is initially undefined
	result, err := r.Execute(`
		'event' in window && window.event === undefined;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("window.event should exist and be undefined initially")
	}

	// Test 2: window.event is set during dispatch
	result, err = r.Execute(`
		var eventDuringDispatch = null;
		var target = document.createElement('div');
		var clickEvent = new Event('click');
		target.addEventListener('click', function(e) {
			eventDuringDispatch = window.event;
		});
		target.dispatchEvent(clickEvent);
		eventDuringDispatch === clickEvent;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("window.event should be set to the current event during dispatch")
	}

	// Test 3: window.event is undefined after dispatch
	result, err = r.Execute(`
		window.event === undefined;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("window.event should be undefined after dispatch")
	}

	// Test 4: Nested events restore window.event correctly
	result, err = r.Execute(`
		var target1 = document.createElement('div');
		var target2 = document.createElement('div');
		var outerEventDuringOuter = null;
		var outerEventAfterInner = null;
		var innerEventDuringInner = null;

		target2.addEventListener('inner', function(e) {
			innerEventDuringInner = window.event;
		});

		target1.addEventListener('outer', function(e) {
			outerEventDuringOuter = window.event;
			target2.dispatchEvent(new Event('inner'));
			outerEventAfterInner = window.event;
		});

		var outerEvent = new Event('outer');
		target1.dispatchEvent(outerEvent);

		outerEventDuringOuter === outerEvent &&
		innerEventDuringInner.type === 'inner' &&
		outerEventAfterInner === outerEvent;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("window.event should be correctly restored after nested dispatch")
	}
}

func TestEventSubclassConstructors(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test that class extending Event works
	result, err := r.Execute(`
		class SubclassedEvent extends Event {
			constructor(name, props) {
				super(name, props);
				if (props && typeof(props) == "object" && "customProp" in props) {
					this.customProp = props.customProp;
				} else {
					this.customProp = 5;
				}
			}

			get fixedProp() {
				return 17;
			}
		}

		var e = new SubclassedEvent('test');
		e instanceof Event && e instanceof SubclassedEvent && e.type === 'test' && e.customProp === 5 && e.fixedProp === 17;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Class extending Event should work correctly")
	}

	// Test with custom props
	result, err = r.Execute(`
		var e2 = new SubclassedEvent('test', { customProp: 8, bubbles: true });
		e2.customProp === 8 && e2.fixedProp === 17 && e2.bubbles === true;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("SubclassedEvent with custom props should work correctly")
	}
}

func TestUIEventViewTypeValidation(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test that passing wrong type for view throws TypeError
	result, err := r.Execute(`
		var threw = false;
		var errorType = "";
		try {
			new UIEvent("x", { view: 7 });
		} catch (e) {
			threw = true;
			errorType = e.name || e.constructor.name;
		}
		threw && errorType === "TypeError";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("UIEvent with wrong view type should throw TypeError")
	}
}

func TestEventTimeStamp(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)
	eventBinder := NewEventBinder(r)
	eventBinder.SetupEventConstructors()

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test that Event.timeStamp is a DOMHighResTimeStamp (positive number relative to timeOrigin)
	result, err := r.Execute(`
		var event = new Event('test');
		event.timeStamp >= 0 && typeof event.timeStamp === 'number';
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Event.timeStamp should be a non-negative number")
	}

	// Test that timeStamp is relative to performance.timeOrigin
	result, err = r.Execute(`
		var event = new Event('test');
		var now = performance.now();
		// Event timeStamp should be close to performance.now() (within 100ms)
		Math.abs(event.timeStamp - now) < 100;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Event.timeStamp should be close to performance.now()")
	}

	// Test that createEvent also sets timeStamp
	result, err = r.Execute(`
		var event2 = document.createEvent('Event');
		event2.initEvent('click', true, true);
		event2.timeStamp >= 0;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Events created via createEvent should have non-negative timeStamp")
	}
}
