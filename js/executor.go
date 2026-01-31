package js

import (
	"strings"

	"github.com/AYColumbia/viberowser/dom"
	"github.com/dop251/goja"
)

// ScriptExecutor handles executing scripts in an HTML document.
type ScriptExecutor struct {
	runtime     *Runtime
	domBinder   *DOMBinder
	eventBinder *EventBinder
}

// NewScriptExecutor creates a new script executor.
func NewScriptExecutor(runtime *Runtime) *ScriptExecutor {
	domBinder := NewDOMBinder(runtime)
	eventBinder := NewEventBinder(runtime)
	eventBinder.SetupEventConstructors()

	return &ScriptExecutor{
		runtime:     runtime,
		domBinder:   domBinder,
		eventBinder: eventBinder,
	}
}

// Runtime returns the underlying JS runtime.
func (se *ScriptExecutor) Runtime() *Runtime {
	return se.runtime
}

// DOMBinder returns the DOM binder.
func (se *ScriptExecutor) DOMBinder() *DOMBinder {
	return se.domBinder
}

// EventBinder returns the event binder.
func (se *ScriptExecutor) EventBinder() *EventBinder {
	return se.eventBinder
}

// SetupDocument sets up the document object and returns the JS document.
func (se *ScriptExecutor) SetupDocument(doc *dom.Document) {
	jsDoc := se.domBinder.BindDocument(doc)

	// Add event target methods to document
	se.eventBinder.BindEventTarget(jsDoc)

	// Also add event target methods to window
	window := se.runtime.vm.Get("window").ToObject(se.runtime.vm)
	if window != nil {
		se.eventBinder.BindEventTarget(window)
	}

	// Add global addEventListener/removeEventListener/dispatchEvent
	// These are needed because in browsers, the global scope IS the window,
	// but in goja they are separate. Many scripts call addEventListener()
	// without the window. prefix.
	se.bindGlobalEventTargetMethods()
}

// bindGlobalEventTargetMethods adds global addEventListener/removeEventListener/dispatchEvent
// that delegate to the window object.
func (se *ScriptExecutor) bindGlobalEventTargetMethods() {
	vm := se.runtime.vm
	window := vm.Get("window").ToObject(vm)
	if window == nil {
		return
	}

	// Get the EventTarget for window
	windowTarget := se.eventBinder.GetOrCreateTarget(window)

	// addEventListener
	vm.Set("addEventListener", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}

		eventType := call.Arguments[0].String()
		callback, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			return goja.Undefined()
		}

		opts := listenerOptions{}
		if len(call.Arguments) > 2 {
			arg := call.Arguments[2]
			if arg.ExportType().Kind().String() == "bool" {
				opts.capture = arg.ToBoolean()
			} else if obj := arg.ToObject(vm); obj != nil {
				if v := obj.Get("capture"); v != nil {
					opts.capture = v.ToBoolean()
				}
				if v := obj.Get("once"); v != nil {
					opts.once = v.ToBoolean()
				}
				if v := obj.Get("passive"); v != nil {
					opts.passive = v.ToBoolean()
				}
			}
		}

		windowTarget.AddEventListener(eventType, callback, call.Arguments[1], opts)
		return goja.Undefined()
	}))

	// removeEventListener
	vm.Set("removeEventListener", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}

		eventType := call.Arguments[0].String()
		_, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			return goja.Undefined()
		}

		capture := false
		if len(call.Arguments) > 2 {
			arg := call.Arguments[2]
			if arg.ExportType().Kind().String() == "bool" {
				capture = arg.ToBoolean()
			} else if obj := arg.ToObject(vm); obj != nil {
				if v := obj.Get("capture"); v != nil {
					capture = v.ToBoolean()
				}
			}
		}

		windowTarget.RemoveEventListener(eventType, call.Arguments[1], capture)
		return goja.Undefined()
	}))

	// dispatchEvent
	vm.Set("dispatchEvent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(true)
		}

		event := call.Arguments[0].ToObject(vm)
		if event == nil {
			return vm.ToValue(true)
		}

		event.Set("target", window)
		event.Set("currentTarget", window)
		event.Set("eventPhase", int(EventPhaseAtTarget))

		return vm.ToValue(windowTarget.DispatchEvent(vm, event, EventPhaseAtTarget))
	}))
}

// ExecuteScripts finds and executes all script elements in the document.
func (se *ScriptExecutor) ExecuteScripts(doc *dom.Document) []error {
	scripts := doc.GetElementsByTagName("script")
	var errors []error

	for i := 0; i < scripts.Length(); i++ {
		script := scripts.Item(i)
		if script == nil {
			continue
		}

		err := se.executeScript(script)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// executeScript executes a single script element.
func (se *ScriptExecutor) executeScript(script *dom.Element) error {
	// Check if this is JavaScript (or has no type, which defaults to JavaScript)
	scriptType := script.GetAttribute("type")
	if scriptType != "" && scriptType != "text/javascript" && scriptType != "application/javascript" && scriptType != "module" {
		// Not JavaScript, skip
		return nil
	}

	// Check for src attribute (external script)
	src := script.GetAttribute("src")
	if src != "" {
		// TODO: Fetch and execute external script
		// For now, we skip external scripts
		return nil
	}

	// Get inline script content
	code := script.TextContent()
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}

	// Get script location for error reporting
	id := script.GetAttribute("id")
	if id == "" {
		id = "inline"
	}

	return se.runtime.ExecuteScript(code, id)
}

// ExecuteInlineHandler executes an inline event handler (onclick, onload, etc.).
func (se *ScriptExecutor) ExecuteInlineHandler(code string, thisObj interface{}) error {
	// Wrap the code in a function
	wrapped := "(function() { " + code + " })"

	result, err := se.runtime.Execute(wrapped)
	if err != nil {
		return err
	}

	// Call the function
	fn, ok := result.Export().(func())
	if ok {
		fn()
	}

	return nil
}

// DispatchEvent dispatches a DOM event on an element.
func (se *ScriptExecutor) DispatchEvent(jsElement *dom.Element, eventType string, options map[string]interface{}) bool {
	jsEl := se.domBinder.BindElement(jsElement)
	event := se.eventBinder.CreateEvent(eventType, options)

	// Set event properties
	event.Set("target", jsEl)
	event.Set("currentTarget", jsEl)
	event.Set("eventPhase", int(EventPhaseAtTarget))

	target := se.eventBinder.GetOrCreateTarget(jsEl)
	return target.DispatchEvent(se.runtime.vm, event, EventPhaseAtTarget)
}

// DispatchLoadEvent dispatches a load event on the window.
func (se *ScriptExecutor) DispatchLoadEvent() {
	window := se.runtime.vm.Get("window").ToObject(se.runtime.vm)
	if window == nil {
		return
	}

	event := se.eventBinder.CreateEvent("load", map[string]interface{}{
		"bubbles":    false,
		"cancelable": false,
	})

	event.Set("target", window)
	event.Set("currentTarget", window)
	event.Set("eventPhase", int(EventPhaseAtTarget))
	event.Set("isTrusted", true)

	target := se.eventBinder.GetOrCreateTarget(window)
	target.DispatchEvent(se.runtime.vm, event, EventPhaseAtTarget)
}

// DispatchDOMContentLoaded dispatches the DOMContentLoaded event on the document.
func (se *ScriptExecutor) DispatchDOMContentLoaded() {
	doc := se.runtime.vm.Get("document")
	if doc == nil || doc.ToObject(se.runtime.vm) == nil {
		return
	}
	docObj := doc.ToObject(se.runtime.vm)

	event := se.eventBinder.CreateEvent("DOMContentLoaded", map[string]interface{}{
		"bubbles":    true,
		"cancelable": false,
	})

	event.Set("target", docObj)
	event.Set("currentTarget", docObj)
	event.Set("eventPhase", int(EventPhaseAtTarget))
	event.Set("isTrusted", true)

	target := se.eventBinder.GetOrCreateTarget(docObj)
	target.DispatchEvent(se.runtime.vm, event, EventPhaseAtTarget)
}

// RunEventLoop runs the event loop until there's no more work.
func (se *ScriptExecutor) RunEventLoop() {
	for se.runtime.HasPendingWork() {
		se.runtime.RunEventLoop()
	}
}

// RunEventLoopOnce runs one iteration of the event loop.
func (se *ScriptExecutor) RunEventLoopOnce() bool {
	return se.runtime.RunEventLoop()
}

// Cleanup clears caches and releases resources.
func (se *ScriptExecutor) Cleanup() {
	se.domBinder.ClearCache()
	se.eventBinder.ClearTargets()
	se.runtime.ClearErrors()
}

// ExecuteExternalScript executes an external script with the given content.
// The scriptURL is used for error reporting.
func (se *ScriptExecutor) ExecuteExternalScript(content string, scriptURL string) error {
	code := strings.TrimSpace(content)
	if code == "" {
		return nil
	}

	return se.runtime.ExecuteScript(code, scriptURL)
}
