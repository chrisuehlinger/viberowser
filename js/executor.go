package js

import (
	"strings"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
	"github.com/dop251/goja"
)

// iframeContent holds the contentWindow and contentDocument for an iframe.
type iframeContent struct {
	window   goja.Value
	document goja.Value
	goDoc    *dom.Document // The Go DOM document for the iframe
}

// IframeContentLoader is a callback for loading iframe content.
// It takes the iframe src URL and returns the loaded document, or nil if loading failed.
type IframeContentLoader func(src string) *dom.Document

// ScriptExecutor handles executing scripts in an HTML document.
type ScriptExecutor struct {
	runtime                  *Runtime
	domBinder                *DOMBinder
	eventBinder              *EventBinder
	mutationObserverManager  *MutationObserverManager
	iframeWindows            map[*dom.Element]goja.Value    // Cache of content windows per iframe element (legacy)
	iframeContents           map[*dom.Element]*iframeContent // Cache of content windows/documents per iframe
	iframeContentLoader      IframeContentLoader             // Callback for loading iframe content
	currentDocument          *dom.Document                   // Currently bound document
}

// NewScriptExecutor creates a new script executor.
func NewScriptExecutor(runtime *Runtime) *ScriptExecutor {
	domBinder := NewDOMBinder(runtime)
	eventBinder := NewEventBinder(runtime)
	eventBinder.SetupEventConstructors()

	// Set the event binder on DOM binder so all nodes get EventTarget methods
	domBinder.SetEventBinder(eventBinder)

	// Create mutation observer manager
	mutationManager := NewMutationObserverManager()

	se := &ScriptExecutor{
		runtime:                 runtime,
		domBinder:               domBinder,
		eventBinder:             eventBinder,
		mutationObserverManager: mutationManager,
		iframeWindows:           make(map[*dom.Element]goja.Value),
		iframeContents:          make(map[*dom.Element]*iframeContent),
	}

	// Set the iframe content provider on DOM binder
	domBinder.SetIframeContentProvider(se.getIframeContent)

	// Set up MutationObserver constructor
	SetupMutationObserver(runtime, domBinder, mutationManager)

	return se
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

// SetIframeContentLoader sets the callback for loading iframe content.
// This allows external code (like the WPT runner) to provide iframe content loading.
func (se *ScriptExecutor) SetIframeContentLoader(loader IframeContentLoader) {
	se.iframeContentLoader = loader
}

// SetStyleResolver sets the style resolver for getComputedStyle.
func (se *ScriptExecutor) SetStyleResolver(sr *css.StyleResolver) {
	se.domBinder.SetStyleResolver(sr)
	se.setupGetComputedStyle()
}

// setupGetComputedStyle sets up the window.getComputedStyle function.
func (se *ScriptExecutor) setupGetComputedStyle() {
	vm := se.runtime.vm
	window := vm.Get("window")
	if window == nil {
		return
	}
	windowObj := window.ToObject(vm)
	if windowObj == nil {
		return
	}

	// Override the stub getComputedStyle with a real implementation
	windowObj.Set("getComputedStyle", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}

		// Get the element argument
		arg := call.Arguments[0]
		if goja.IsNull(arg) || goja.IsUndefined(arg) {
			return goja.Null()
		}

		obj := arg.ToObject(vm)
		if obj == nil {
			return goja.Null()
		}

		// Get the Go element from the JS object
		goEl := obj.Get("_goElement")
		if goEl == nil || goja.IsUndefined(goEl) {
			return goja.Null()
		}

		el, ok := goEl.Export().(*dom.Element)
		if !ok || el == nil {
			return goja.Null()
		}

		// Get optional pseudo-element argument
		pseudoElt := ""
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) && !goja.IsUndefined(call.Arguments[1]) {
			pseudoElt = call.Arguments[1].String()
		}

		// Get computed style from DOM binder
		return se.domBinder.GetComputedStyle(el, pseudoElt)
	})

	// Also set it globally for scripts that use getComputedStyle without window prefix
	vm.Set("getComputedStyle", windowObj.Get("getComputedStyle"))
}

// SetupDocument sets up the document object and returns the JS document.
func (se *ScriptExecutor) SetupDocument(doc *dom.Document) {
	// Clear previous document's mutation callback if any
	if se.currentDocument != nil {
		dom.UnregisterMutationCallback(se.currentDocument, se.mutationObserverManager)
	}

	// Store current document and register mutation callback
	se.currentDocument = doc
	dom.RegisterMutationCallback(doc, se.mutationObserverManager)

	jsDoc := se.domBinder.BindDocument(doc)

	// Add event target methods to document
	se.eventBinder.BindEventTarget(jsDoc)

	// Also add event target methods to window
	window := se.runtime.vm.Get("window").ToObject(se.runtime.vm)
	if window != nil {
		se.eventBinder.BindEventTarget(window)
	}

	// Set up window.frames property to access iframe content windows
	se.setupWindowFrames(doc, window)

	// Add global addEventListener/removeEventListener/dispatchEvent
	// These are needed because in browsers, the global scope IS the window,
	// but in goja they are separate. Many scripts call addEventListener()
	// without the window. prefix.
	se.bindGlobalEventTargetMethods()
}

// setupWindowFrames sets up the window.frames property to provide access to iframe content windows.
// In browsers, window.frames is an array-like object where frames[i] returns the contentWindow
// of the i-th iframe element in document order.
func (se *ScriptExecutor) setupWindowFrames(doc *dom.Document, window *goja.Object) {
	if window == nil {
		return
	}

	vm := se.runtime.vm

	// Create a frames object that dynamically looks up iframes
	frames := vm.NewObject()

	// Define length as a getter that counts iframes dynamically
	frames.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		iframes := doc.GetElementsByTagName("iframe")
		return vm.ToValue(iframes.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Create a proxy-like object using defineProperty for numeric indices
	// We need to make frames[0], frames[1], etc. work dynamically
	// goja doesn't support Proxy, so we use a dynamic accessor approach

	// Set up the frames object on window
	window.Set("frames", frames)
	// Also set it as a global for direct access
	vm.Set("frames", frames)

	// We need to intercept numeric property access on frames.
	// Since goja doesn't have Proxy, we'll use a workaround:
	// Create indexed properties 0-9 that check for iframes dynamically.
	// This covers most practical use cases.
	for i := 0; i < 10; i++ {
		idx := i
		frames.DefineAccessorProperty(vm.ToValue(idx).String(), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			iframes := doc.GetElementsByTagName("iframe")
			if idx >= iframes.Length() {
				return goja.Undefined()
			}
			iframe := iframes.Item(idx)
			if iframe == nil {
				return goja.Undefined()
			}
			// Return the contentWindow for this iframe
			return se.getIframeContentWindow(iframe, doc)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
}

// getIframeContent returns the contentWindow and contentDocument for an iframe element.
// This is used by DOMBinder to provide iframe.contentWindow and iframe.contentDocument properties.
func (se *ScriptExecutor) getIframeContent(iframe *dom.Element) (goja.Value, goja.Value) {
	// Check cache first
	if cached, ok := se.iframeContents[iframe]; ok {
		return cached.window, cached.document
	}

	vm := se.runtime.vm

	// Create a window object for this iframe
	contentWindow := vm.NewObject()

	// Get the iframe's src attribute to determine document content
	src := iframe.GetAttribute("src")
	if src == "" {
		src = "about:blank"
	}

	// Try to load iframe content using the loader
	var iframeDoc *dom.Document
	if se.iframeContentLoader != nil {
		iframeDoc = se.iframeContentLoader(src)
	}

	// If no loader or loading failed, create a minimal blank document
	if iframeDoc == nil {
		iframeDoc = dom.NewDocument()
		// Create basic document structure: html > head + body
		html := iframeDoc.CreateElement("html")
		head := iframeDoc.CreateElement("head")
		body := iframeDoc.CreateElement("body")
		iframeDoc.AsNode().AppendChild(html.AsNode())
		html.AsNode().AppendChild(head.AsNode())
		html.AsNode().AppendChild(body.AsNode())
	}

	// Bind the iframe document to JavaScript
	jsIframeDoc := se.domBinder.BindDocument(iframeDoc)

	// Set up the content window properties
	contentWindow.Set("document", jsIframeDoc)
	contentWindow.Set("window", contentWindow)
	contentWindow.Set("self", contentWindow)

	// Set parent and top references
	parentWindow := vm.Get("window")
	contentWindow.Set("parent", parentWindow)
	contentWindow.Set("top", parentWindow) // Simplified: assume single level of nesting

	// Add frameElement reference back to the iframe
	contentWindow.Set("frameElement", se.domBinder.BindElement(iframe))

	// Add basic window properties (stub location object)
	location := vm.NewObject()
	location.Set("href", src)
	contentWindow.Set("location", location)

	// Copy DOM constructors from parent window so instanceof checks work
	// These need to be available on every Window object in a browser
	if parentWindowObj := parentWindow.ToObject(vm); parentWindowObj != nil {
		constructorsToCopy := []string{
			"Element", "Node", "Document", "DocumentType", "DocumentFragment",
			"Text", "Comment", "CDATASection", "ProcessingInstruction",
			"Attr", "HTMLElement", "HTMLDocument", "XMLDocument",
			"CharacterData", "DOMException", "DOMTokenList", "NamedNodeMap",
			"NodeList", "HTMLCollection",
		}
		for _, name := range constructorsToCopy {
			if val := parentWindowObj.Get(name); val != nil && !goja.IsUndefined(val) {
				contentWindow.Set(name, val)
			}
		}
	}

	// Cache both
	se.iframeContents[iframe] = &iframeContent{
		window:   contentWindow,
		document: jsIframeDoc,
		goDoc:    iframeDoc,
	}
	// Also cache in legacy map for compatibility
	se.iframeWindows[iframe] = contentWindow

	return contentWindow, jsIframeDoc
}

// getIframeContentWindow returns a window-like object for an iframe element.
// This window object has its own document for the iframe's content.
// The content window is cached per iframe element to ensure identity.
func (se *ScriptExecutor) getIframeContentWindow(iframe *dom.Element, parentDoc *dom.Document) goja.Value {
	contentWindow, _ := se.getIframeContent(iframe)
	return contentWindow
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
