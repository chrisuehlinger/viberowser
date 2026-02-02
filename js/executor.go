package js

import (
	"net/url"
	"strings"

	"github.com/chrisuehlinger/viberowser/css"
	"github.com/chrisuehlinger/viberowser/dom"
	"github.com/dop251/goja"
)

// iframeContent holds the contentWindow and contentDocument for an iframe.
type iframeContent struct {
	window   goja.Value
	document goja.Value
	goDoc    *dom.Document // The Go DOM document for the iframe
}

// IframeContentLoader is a callback for loading iframe content.
// It takes the iframe src URL and returns the loaded document and the final URL
// (which may differ from src if there were redirects), or nil if loading failed.
type IframeContentLoader func(src string) (doc *dom.Document, finalURL string)

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
	xhrManager               *XHRManager                     // XMLHttpRequest manager
	fetchManager             *FetchManager                   // Fetch API manager
	historyManager           *HistoryManager                 // History API manager
}

// NewScriptExecutor creates a new script executor.
func NewScriptExecutor(runtime *Runtime) *ScriptExecutor {
	domBinder := NewDOMBinder(runtime)
	eventBinder := NewEventBinder(runtime)
	eventBinder.SetupEventConstructors()

	// Set the event binder on DOM binder so all nodes get EventTarget methods
	domBinder.SetEventBinder(eventBinder)

	// Set the node resolver so events can propagate through the DOM tree
	eventBinder.SetNodeResolver(func(obj *goja.Object) *goja.Object {
		if obj == nil {
			return nil
		}
		// Get the Go node from the JS object
		goNode := domBinder.getGoNode(obj)
		if goNode == nil {
			return nil
		}
		// Get the parent node
		parentNode := goNode.ParentNode()
		if parentNode == nil {
			// If this is the main document (has a browsing context), return the window
			// This allows events to bubble from Document to Window
			if goNode.NodeType() == dom.DocumentNode {
				goDoc := (*dom.Document)(goNode)
				if domBinder.IsMainDocument(goDoc) {
					return runtime.vm.Get("window").ToObject(runtime.vm)
				}
			}
			return nil
		}
		// Return the JS binding for the parent node
		return domBinder.BindNode(parentNode)
	})

	// Set the shadow root checker so window.event is handled correctly for shadow DOM.
	// Per HTML spec, window.event should be undefined when listeners inside shadow trees are invoked.
	eventBinder.SetShadowRootChecker(func(obj *goja.Object) bool {
		if obj == nil {
			return false
		}
		// Get the Go node from the JS object
		goNode := domBinder.getGoNode(obj)
		if goNode == nil {
			return false
		}
		// Check if the node's root is a ShadowRoot
		root := goNode.GetRootNode()
		return root != nil && root.IsShadowRoot()
	})

	// Set the shadow host resolver for composed events to cross shadow boundaries.
	// This returns the shadow host element when given a shadow root.
	eventBinder.SetShadowHostResolver(func(obj *goja.Object) *goja.Object {
		if obj == nil {
			return nil
		}
		// Check if this object has _goShadowRoot (meaning it's a ShadowRoot binding)
		if v := obj.Get("_goShadowRoot"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
			if sr, ok := v.Export().(*dom.ShadowRoot); ok && sr != nil {
				host := sr.Host()
				if host != nil {
					return domBinder.BindElement(host)
				}
			}
		}
		// Also check if the node's root is a ShadowRoot (the node is inside a shadow tree)
		// In this case, we need to get the shadow root first, then the host
		goNode := domBinder.getGoNode(obj)
		if goNode != nil {
			root := goNode.GetRootNode()
			if root != nil && root.IsShadowRoot() {
				sr := root.GetShadowRoot()
				if sr != nil {
					host := sr.Host()
					if host != nil {
						return domBinder.BindElement(host)
					}
				}
			}
		}
		return nil
	})

	// Set activation handlers for click events on checkbox/radio inputs
	// Per HTML spec, activation behavior runs BEFORE onclick fires
	eventBinder.SetActivationHandlers(
		// Activation handler: runs pre-click activation for elements with activation behavior
		func(eventPath []*goja.Object, event *goja.Object) *ActivationResult {
			// Only handle MouseEvent click events
			// Per spec, activation behavior only triggers for MouseEvent, not plain Event
			eventType := ""
			if typeVal := event.Get("type"); typeVal != nil {
				eventType = typeVal.String()
			}
			if eventType != "click" {
				return nil
			}

			// Check if this is a MouseEvent (has button property)
			// Plain Event clicks should not trigger activation behavior
			buttonVal := event.Get("button")
			if buttonVal == nil || goja.IsUndefined(buttonVal) {
				return nil
			}

			// Check if the event bubbles - if not, only check the target (first in path)
			// Per HTML spec, non-bubbling events only trigger activation on the target
			bubbles := false
			if bubblesVal := event.Get("bubbles"); bubblesVal != nil {
				bubbles = bubblesVal.ToBoolean()
			}

			// Determine which elements to check for activation behavior
			// If event bubbles, check whole path; otherwise just the target
			pathToCheck := eventPath
			if !bubbles && len(eventPath) > 0 {
				pathToCheck = eventPath[:1] // Only the target
			}

			// Find the first element in the path with activation behavior
			// Per DOM spec, only one activation behavior runs per dispatch
			for _, jsObj := range pathToCheck {
				goNode := domBinder.getGoNode(jsObj)
				if goNode == nil || goNode.NodeType() != dom.ElementNode {
					continue
				}
				el := (*dom.Element)(goNode)

				// Check for checkbox/radio activation behavior
				if el.IsCheckable() {
					// Pre-click activation: toggle the checked state BEFORE dispatch
					previousChecked := el.Checked()
					el.SetChecked(!previousChecked)

					return &ActivationResult{
						Element:         jsObj,
						PreviousChecked: previousChecked,
						HasActivation:   true,
						ActivationType:  el.InputType(),
					}
				}
			}
			return nil
		},
		// Cancel handler: reverts activation if defaultPrevented
		func(result *ActivationResult) {
			if result == nil || !result.HasActivation {
				return
			}
			// Get the Go element from the JS object
			goNode := domBinder.getGoNode(result.Element)
			if goNode == nil || goNode.NodeType() != dom.ElementNode {
				return
			}
			el := (*dom.Element)(goNode)
			// Revert to previous checked state
			el.SetChecked(result.PreviousChecked)
		},
		// Complete handler: fires input and change events after successful activation
		func(result *ActivationResult) {
			if result == nil || !result.HasActivation || result.Element == nil {
				return
			}

			vm := runtime.vm

			// Fire 'input' event on the element
			// Per HTML spec, input event bubbles but is not cancelable
			inputEvent := eventBinder.CreateEvent("input", map[string]interface{}{
				"bubbles":    true,
				"cancelable": false,
			})

			dispatchEvent := result.Element.Get("dispatchEvent")
			if dispatchEvent != nil && !goja.IsUndefined(dispatchEvent) {
				if fn, ok := goja.AssertFunction(dispatchEvent); ok {
					fn(result.Element, inputEvent)
				}
			}

			// Fire 'change' event on the element
			// Per HTML spec, change event bubbles but is not cancelable
			changeEvent := eventBinder.CreateEvent("change", map[string]interface{}{
				"bubbles":    true,
				"cancelable": false,
			})

			if fn, ok := goja.AssertFunction(dispatchEvent); ok {
				fn(result.Element, changeEvent)
			}

			// Avoid unused variable warning
			_ = vm
		},
	)

	// Set up listener error handler to dispatch ErrorEvent on window
	// Per HTML spec, exceptions in event listeners should be reported via error events
	eventBinder.SetListenerErrorHandler(func(err goja.Value) {
		// Get window
		window := runtime.vm.Get("window")
		if window == nil || goja.IsUndefined(window) {
			return
		}
		windowObj := window.ToObject(runtime.vm)
		if windowObj == nil {
			return
		}

		// Extract error message
		message := ""
		if err != nil && !goja.IsUndefined(err) && !goja.IsNull(err) {
			errObj := err.ToObject(runtime.vm)
			if errObj != nil {
				if msgVal := errObj.Get("message"); msgVal != nil && !goja.IsUndefined(msgVal) {
					message = msgVal.String()
				} else {
					message = err.String()
				}
			} else {
				message = err.String()
			}
		}

		// Create and dispatch ErrorEvent on window
		// Per HTML spec, uncaught exceptions dispatch an ErrorEvent on window
		errorEventOptions := map[string]interface{}{
			"bubbles":    false,
			"cancelable": true,
		}
		errorEvent := eventBinder.CreateEvent("error", errorEventOptions)
		errorEvent.Set("message", message)
		errorEvent.Set("filename", "")
		errorEvent.Set("lineno", 0)
		errorEvent.Set("colno", 0)
		errorEvent.Set("error", err)

		// Dispatch the error event on window
		// This will trigger event listeners added via addEventListener
		dispatchEventFn := windowObj.Get("dispatchEvent")
		if dispatchEventFn != nil && !goja.IsUndefined(dispatchEventFn) {
			if fn, ok := goja.AssertFunction(dispatchEventFn); ok {
				fn(windowObj, errorEvent)
			}
		}
	})

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

// setupGetSelection sets up the window.getSelection function.
func (se *ScriptExecutor) setupGetSelection(doc *dom.Document) {
	vm := se.runtime.vm
	window := vm.Get("window")
	if window == nil {
		return
	}
	windowObj := window.ToObject(vm)
	if windowObj == nil {
		return
	}

	// Set up window.getSelection to return the document's Selection
	windowObj.Set("getSelection", func(call goja.FunctionCall) goja.Value {
		selection := doc.GetSelection()
		return se.domBinder.BindSelection(selection)
	})

	// Also set it globally for scripts that use getSelection without window prefix
	vm.Set("getSelection", windowObj.Get("getSelection"))
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

	// Mark this as the main document (associated with window) for event bubbling
	se.domBinder.SetMainDocument(doc)

	jsDoc := se.domBinder.BindDocument(doc)

	// Add event target methods to document
	se.eventBinder.BindEventTarget(jsDoc)

	// Also add event target methods to window
	window := se.runtime.vm.Get("window").ToObject(se.runtime.vm)
	if window != nil {
		se.eventBinder.BindEventTarget(window)
		// Bind event handler IDL attributes (onload, onerror, etc.) to window
		// Per HTML spec, Window has both GlobalEventHandlers and WindowEventHandlers
		se.domBinder.BindWindowEventHandlers(window)
	}

	// Set up window.frames property to access iframe content windows
	se.setupWindowFrames(doc, window)

	// Set up window.getSelection() to return the document's selection
	se.setupGetSelection(doc)

	// Add global addEventListener/removeEventListener/dispatchEvent
	// These are needed because in browsers, the global scope IS the window,
	// but in goja they are separate. Many scripts call addEventListener()
	// without the window. prefix.
	se.bindGlobalEventTargetMethods()

	// Setup XMLHttpRequest with the document's URL as the base
	se.setupXMLHttpRequest(doc)

	// Setup fetch API with the document's URL as the base
	se.setupFetch(doc)

	// Setup History API with the document's URL as the base
	se.setupHistory(doc)

	// Set up named iframe access so iframes with name attributes are accessible as globals
	se.setupNamedIframeAccess()
}

// setupXMLHttpRequest sets up the XMLHttpRequest constructor with the document's URL.
func (se *ScriptExecutor) setupXMLHttpRequest(doc *dom.Document) {
	docURL := doc.URL()
	var baseURL, documentURL *url.URL
	var err error

	if docURL != "" && docURL != "about:blank" {
		baseURL, err = url.Parse(docURL)
		if err != nil {
			baseURL = nil
		}
		documentURL = baseURL
	}

	se.xhrManager = NewXHRManager(se.runtime, baseURL, documentURL)
	se.xhrManager.SetupXMLHttpRequest()
}

// setupFetch sets up the fetch API with the document's URL.
func (se *ScriptExecutor) setupFetch(doc *dom.Document) {
	docURL := doc.URL()
	var baseURL, documentURL *url.URL
	var err error

	if docURL != "" && docURL != "about:blank" {
		baseURL, err = url.Parse(docURL)
		if err != nil {
			baseURL = nil
		}
		documentURL = baseURL
	}

	se.fetchManager = NewFetchManager(se.runtime, baseURL, documentURL)
	se.fetchManager.SetupFetch()
}

// setupHistory sets up the History API with the document's URL.
func (se *ScriptExecutor) setupHistory(doc *dom.Document) {
	docURL := doc.URL()
	var baseURL, documentURL *url.URL
	var err error

	if docURL != "" && docURL != "about:blank" {
		baseURL, err = url.Parse(docURL)
		if err != nil {
			baseURL = nil
		}
		documentURL = baseURL
	}

	se.historyManager = NewHistoryManager(se.runtime, baseURL, documentURL, se.eventBinder)
	se.historyManager.SetupHistory()
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

	// Store the doc reference for use in the named access callback
	se.currentDocument = doc
}

// setupNamedIframeAccess sets up named iframe access after scripts are parsed.
// This is called after the document is fully loaded so we can find all named iframes.
// Named iframes should be accessible as window[name] and as global variables.
func (se *ScriptExecutor) setupNamedIframeAccess() {
	if se.currentDocument == nil {
		return
	}

	vm := se.runtime.vm
	doc := se.currentDocument
	window := vm.Get("window").ToObject(vm)
	if window == nil {
		return
	}

	// Find all iframes with name attributes and register them
	iframes := doc.GetElementsByTagName("iframe")
	for i := 0; i < iframes.Length(); i++ {
		iframe := iframes.Item(i)
		if iframe == nil {
			continue
		}
		name := iframe.GetAttribute("name")
		if name == "" {
			continue
		}

		// Create a getter for this named iframe on both window and global scope.
		// We need to capture iframe by value for the closure.
		iframeElement := iframe
		getter := vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return se.getIframeContentWindow(iframeElement, doc)
		})

		// Set on window
		window.DefineAccessorProperty(name, getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		// Also set as global variable (for direct access without window. prefix)
		vm.GlobalObject().DefineAccessorProperty(name, getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}

	// Set up named access for elements with id attributes.
	// Per HTML spec, elements with id attributes should be accessible as window[id].
	// This is a simplified implementation that covers common use cases.
	se.setupNamedElementAccess()
}

// setupNamedElementAccess sets up named access for elements with id attributes.
// Per HTML spec, elements with id attributes should be accessible on the window object.
func (se *ScriptExecutor) setupNamedElementAccess() {
	if se.currentDocument == nil {
		return
	}

	vm := se.runtime.vm
	doc := se.currentDocument
	window := vm.Get("window").ToObject(vm)
	if window == nil {
		return
	}

	globalObj := vm.GlobalObject()

	// Find all elements with id attributes
	// This is a simplified approach - in a full implementation, we'd need to
	// handle dynamic DOM changes with mutation observers.
	allElements := doc.GetElementsByTagName("*")
	for i := 0; i < allElements.Length(); i++ {
		element := allElements.Item(i)
		if element == nil {
			continue
		}

		// Skip iframes as they're handled separately in setupNamedIframeAccess
		tagName := element.TagName()
		if tagName == "IFRAME" || tagName == "iframe" {
			continue
		}

		id := element.GetAttribute("id")
		if id == "" {
			continue
		}

		// Create a getter for this named element.
		// Capture element by value for the closure.
		el := element
		getter := vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return se.domBinder.BindElement(el)
		})

		// Set on window and global scope
		window.DefineAccessorProperty(id, getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
		globalObj.DefineAccessorProperty(id, getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
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
	var finalURL string
	if se.iframeContentLoader != nil {
		iframeDoc, finalURL = se.iframeContentLoader(src)
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
		finalURL = src // Use src as the URL for blank documents
	}

	// Set the document's URL to the final URL (which may differ from src due to redirects)
	if finalURL != "" {
		iframeDoc.SetURL(finalURL)
	}

	// Bind the iframe document to JavaScript without replacing global document.
	// Pass the contentWindow so the document's defaultView property returns it.
	jsIframeDoc := se.domBinder.BindIframeDocumentWithWindow(iframeDoc, contentWindow)

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

	// Copy JavaScript builtin constructors from the global object.
	// These are needed for cross-realm tests that access Object, TypeError, Proxy, etc.
	// on iframe contentWindows.
	globalObj := vm.GlobalObject()
	jsBuiltins := []string{
		"Object", "Array", "Function", "String", "Number", "Boolean",
		"Symbol", "Error", "TypeError", "ReferenceError", "SyntaxError",
		"RangeError", "URIError", "EvalError", "Promise", "Proxy", "Reflect",
		"Map", "Set", "WeakMap", "WeakSet", "Date", "RegExp", "JSON", "Math",
		"parseInt", "parseFloat", "isNaN", "isFinite", "encodeURI", "decodeURI",
		"encodeURIComponent", "decodeURIComponent", "escape", "unescape",
		"eval", "Infinity", "NaN", "undefined", "ArrayBuffer", "DataView",
		"Int8Array", "Uint8Array", "Uint8ClampedArray", "Int16Array",
		"Uint16Array", "Int32Array", "Uint32Array", "Float32Array", "Float64Array",
		"BigInt", "BigInt64Array", "BigUint64Array",
	}
	for _, name := range jsBuiltins {
		if val := globalObj.Get(name); val != nil && !goja.IsUndefined(val) {
			contentWindow.Set(name, val)
		}
	}

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
		listenerArg := call.Arguments[1]

		// Per DOM spec, listener can be a function or an object with handleEvent method
		var callback goja.Callable
		var isObject bool
		var listenerObj *goja.Object

		if fn, ok := goja.AssertFunction(listenerArg); ok {
			// Listener is a function
			callback = fn
			isObject = false
		} else if listenerArg != nil && !goja.IsNull(listenerArg) && !goja.IsUndefined(listenerArg) {
			// Check if it's an object (could have handleEvent)
			listenerObj = listenerArg.ToObject(vm)
			if listenerObj != nil {
				// Accept any object - we'll check handleEvent at dispatch time per DOM spec
				isObject = true
			} else {
				// Not a function and not an object - ignore
				return goja.Undefined()
			}
		} else {
			// null or undefined - ignore
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

		windowTarget.AddEventListener(eventType, callback, listenerArg, isObject, listenerObj, opts)
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

	// Set the dispatch flag so attempts to re-dispatch during the event will throw
	event.Set("_dispatch", true)

	target := se.eventBinder.GetOrCreateTarget(window)
	target.DispatchEvent(se.runtime.vm, event, EventPhaseAtTarget)

	// Clear the dispatch flag after dispatching
	event.Set("_dispatch", false)
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

	// Set the dispatch flag so attempts to re-dispatch during the event will throw
	event.Set("_dispatch", true)

	target := se.eventBinder.GetOrCreateTarget(docObj)
	target.DispatchEvent(se.runtime.vm, event, EventPhaseAtTarget)

	// Clear the dispatch flag after dispatching
	event.Set("_dispatch", false)
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
