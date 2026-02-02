package js

import (
	"sync"

	"github.com/dop251/goja"
)

// EventPhase represents the phase of event dispatch.
type EventPhase int

const (
	EventPhaseNone      EventPhase = 0
	EventPhaseCapturing EventPhase = 1
	EventPhaseAtTarget  EventPhase = 2
	EventPhaseBubbling  EventPhase = 3
)

// Event represents a DOM event.
type Event struct {
	Type              string
	Target            *goja.Object
	CurrentTarget     *goja.Object
	EventPhase        EventPhase
	Bubbles           bool
	Cancelable        bool
	Composed          bool
	DefaultPrevented  bool
	StopPropagation   bool
	StopImmediate     bool
	IsTrusted         bool
	TimeStamp         float64
	CustomDetail      goja.Value
}

// eventListener represents a registered event listener.
type eventListener struct {
	id          int
	callback    goja.Callable // Non-nil if listener is a function
	value       goja.Value    // Original value for comparison
	isObject    bool          // True if listener is an object with handleEvent (not a function)
	listenerObj *goja.Object  // The object if isObject is true
	options     listenerOptions
	removed     bool // Per DOM spec: set when listener is removed, checked during iteration
}

// listenerOptions represents addEventListener options.
type listenerOptions struct {
	capture            bool
	once               bool
	passive            bool
	signal             *goja.Object // AbortSignal to watch for abort
	isOnErrorHandler   bool         // True if this is window.onerror handler (special calling convention)
}

// ListenerErrorHandler is called when an event listener throws an exception.
// It receives the error value (as goja.Value) and should report it to window.onerror.
type ListenerErrorHandler func(err goja.Value)

// EventTarget manages event listeners for a target.
type EventTarget struct {
	listeners    map[string][]*eventListener // Use pointers so removed flag works across copies
	nextID       int
	mu           sync.RWMutex
	errorHandler ListenerErrorHandler
}

// NewEventTarget creates a new EventTarget.
func NewEventTarget() *EventTarget {
	return &EventTarget{
		listeners: make(map[string][]*eventListener),
	}
}

// SetErrorHandler sets the handler called when a listener throws an exception.
func (et *EventTarget) SetErrorHandler(handler ListenerErrorHandler) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.errorHandler = handler
}

// AddEventListener registers an event listener (function or object with handleEvent).
func (et *EventTarget) AddEventListener(eventType string, callback goja.Callable, value goja.Value, isObject bool, listenerObj *goja.Object, opts listenerOptions) {
	et.mu.Lock()
	defer et.mu.Unlock()

	// Check for duplicate by comparing the underlying Value
	for _, l := range et.listeners[eventType] {
		if l.value.SameAs(value) && l.options.capture == opts.capture {
			return // Already registered
		}
	}

	et.nextID++
	et.listeners[eventType] = append(et.listeners[eventType], &eventListener{
		id:          et.nextID,
		callback:    callback,
		value:       value,
		isObject:    isObject,
		listenerObj: listenerObj,
		options:     opts,
	})
}

// RemoveEventListener unregisters an event listener.
func (et *EventTarget) RemoveEventListener(eventType string, value goja.Value, capture bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	listeners := et.listeners[eventType]
	for i, l := range listeners {
		if l.value.SameAs(value) && l.options.capture == capture {
			l.removed = true // Mark as removed so any ongoing iteration skips it
			et.listeners[eventType] = append(listeners[:i], listeners[i+1:]...)
			return
		}
	}
}

// DispatchEvent dispatches an event to all registered listeners.
// At the AT_TARGET phase, both capturing and non-capturing listeners are called
// in the order they were added, with capturing listeners first according to DOM spec.
// Per DOM spec (and Chromium/Firefox bug fixes), stopPropagation() in a capture listener
// at AT_TARGET should prevent bubble listeners on the same target from running.
func (et *EventTarget) DispatchEvent(vm *goja.Runtime, event *goja.Object, phase EventPhase) bool {
	et.mu.RLock()
	eventType := event.Get("type").String()
	// Copy the slice of pointers (not the listeners themselves)
	// This allows us to check the removed flag during iteration
	listeners := make([]*eventListener, len(et.listeners[eventType]))
	copy(listeners, et.listeners[eventType])
	et.mu.RUnlock()

	// Helper to check stopPropagation flag
	shouldStopPropagation := func() bool {
		stopProp := event.Get("_stopPropagation")
		return stopProp != nil && stopProp.ToBoolean()
	}

	// At the target phase, per DOM spec:
	// 1. Capturing listeners are invoked first
	// 2. Then stopPropagation is checked
	// 3. If not stopped, bubbling listeners are invoked
	// This matches the spec clarification in https://github.com/whatwg/dom/issues/916
	if phase == EventPhaseAtTarget {
		// Separate capturing and bubbling listeners
		capturingListeners := make([]*eventListener, 0)
		bubblingListeners := make([]*eventListener, 0)
		for _, l := range listeners {
			if l.options.capture {
				capturingListeners = append(capturingListeners, l)
			} else {
				bubblingListeners = append(bubblingListeners, l)
			}
		}

		// Invoke capturing listeners first
		et.invokeListeners(vm, event, capturingListeners)

		// Check stopPropagation before invoking bubble listeners
		if shouldStopPropagation() {
			return !event.Get("defaultPrevented").ToBoolean()
		}

		// Invoke bubbling listeners
		et.invokeListeners(vm, event, bubblingListeners)

		// Return true if default wasn't prevented
		if defaultPrevented := event.Get("defaultPrevented"); defaultPrevented != nil {
			return !defaultPrevented.ToBoolean()
		}
		return true
	}

	// Get currentTarget for 'this' binding
	currentTarget := event.Get("currentTarget")
	if currentTarget == nil || goja.IsUndefined(currentTarget) {
		currentTarget = goja.Undefined()
	}

	for _, l := range listeners {
		// Per DOM spec: skip listeners that have been removed
		if l.removed {
			continue
		}

		// Check phase - only filter by capture/non-capture for non-target phases
		if phase == EventPhaseCapturing && !l.options.capture {
			continue
		}
		if phase == EventPhaseBubbling && l.options.capture {
			continue
		}

		// Per DOM spec: If listener's once is true, remove the event listener
		// BEFORE invoking the callback. This ensures that if the callback
		// dispatches another event of the same type, this listener won't be called again.
		if l.options.once {
			et.mu.Lock()
			l.removed = true // Mark as removed so other iterations skip it
			eventListeners := et.listeners[eventType]
			for i, existing := range eventListeners {
				if existing.id == l.id {
					et.listeners[eventType] = append(eventListeners[:i], eventListeners[i+1:]...)
					break
				}
			}
			et.mu.Unlock()
		}

		var err error
		if l.isObject {
			// Per DOM spec, for object listeners, we must Get("handleEvent") on each dispatch.
			// This allows getters to be called each time.
			handleEventVal := l.listenerObj.Get("handleEvent")

			// If getting handleEvent throws, report to error handler
			// (In goja, Get doesn't throw but we handle the getter case below)

			// Check if handleEvent is callable
			handleEvent, isCallable := goja.AssertFunction(handleEventVal)
			if !isCallable {
				// Per DOM spec, if handleEvent is not callable, throw TypeError
				// and report to error handler
				if et.errorHandler != nil {
					// Create a proper TypeError and pass it to the error handler
					typeErr := vm.NewTypeError("handleEvent is not a function")
					et.errorHandler(typeErr)
				}
				// Continue to next listener
				continue
			}

			// Call handleEvent with the listener object as 'this'
			_, err = handleEvent(l.listenerObj, event)
		} else {
			// Call the function listener with currentTarget as 'this'
			// Per DOM spec, the 'this' value for function listeners is the currentTarget
			if l.options.isOnErrorHandler && eventType == "error" {
				// Per HTML spec, OnErrorEventHandler has a special calling convention:
				// (message, filename, lineno, colno, error) instead of (event)
				message := ""
				filename := ""
				var lineno, colno int64 = 0, 0
				var errorVal goja.Value = goja.Undefined()

				if msgVal := event.Get("message"); msgVal != nil && !goja.IsUndefined(msgVal) {
					message = msgVal.String()
				}
				if fnVal := event.Get("filename"); fnVal != nil && !goja.IsUndefined(fnVal) {
					filename = fnVal.String()
				}
				if lnVal := event.Get("lineno"); lnVal != nil && !goja.IsUndefined(lnVal) {
					lineno = lnVal.ToInteger()
				}
				if cnVal := event.Get("colno"); cnVal != nil && !goja.IsUndefined(cnVal) {
					colno = cnVal.ToInteger()
				}
				if errVal := event.Get("error"); errVal != nil {
					errorVal = errVal
				}

				_, err = l.callback(currentTarget,
					vm.ToValue(message),
					vm.ToValue(filename),
					vm.ToValue(lineno),
					vm.ToValue(colno),
					errorVal,
				)
			} else {
				_, err = l.callback(currentTarget, event)
			}
		}

		if err != nil {
			// An exception was thrown in the listener
			// Per HTML spec, we should report this to the global error handler
			// and continue dispatching to remaining listeners.
			if et.errorHandler != nil {
				// Convert the error to a goja.Value
				// goja errors are typically *goja.Exception
				if exc, ok := err.(*goja.Exception); ok {
					et.errorHandler(exc.Value())
				} else {
					// For other errors, wrap as string
					et.errorHandler(vm.ToValue(err.Error()))
				}
			}
		}

		// Check for stopImmediatePropagation
		if stopImmediate := event.Get("_stopImmediate"); stopImmediate != nil && stopImmediate.ToBoolean() {
			break
		}
	}

	// Return true if default wasn't prevented
	if defaultPrevented := event.Get("defaultPrevented"); defaultPrevented != nil {
		return !defaultPrevented.ToBoolean()
	}
	return true
}

// invokeListeners invokes a list of event listeners for an event.
// This is a helper used by DispatchEvent for the AT_TARGET phase.
func (et *EventTarget) invokeListeners(vm *goja.Runtime, event *goja.Object, listeners []*eventListener) {
	eventType := event.Get("type").String()

	// Get currentTarget for 'this' binding
	currentTarget := event.Get("currentTarget")
	if currentTarget == nil || goja.IsUndefined(currentTarget) {
		currentTarget = goja.Undefined()
	}

	for _, l := range listeners {
		// Per DOM spec: skip listeners that have been removed
		if l.removed {
			continue
		}

		// Per DOM spec: If listener's once is true, remove the event listener
		// BEFORE invoking the callback.
		if l.options.once {
			et.mu.Lock()
			l.removed = true
			eventListeners := et.listeners[eventType]
			for i, existing := range eventListeners {
				if existing.id == l.id {
					et.listeners[eventType] = append(eventListeners[:i], eventListeners[i+1:]...)
					break
				}
			}
			et.mu.Unlock()
		}

		var err error
		if l.isObject {
			handleEventVal := l.listenerObj.Get("handleEvent")
			handleEvent, isCallable := goja.AssertFunction(handleEventVal)
			if !isCallable {
				if et.errorHandler != nil {
					typeErr := vm.NewTypeError("handleEvent is not a function")
					et.errorHandler(typeErr)
				}
				continue
			}
			_, err = handleEvent(l.listenerObj, event)
		} else {
			if l.options.isOnErrorHandler && eventType == "error" {
				message := ""
				filename := ""
				var lineno, colno int64 = 0, 0
				var errorVal goja.Value = goja.Undefined()

				if msgVal := event.Get("message"); msgVal != nil && !goja.IsUndefined(msgVal) {
					message = msgVal.String()
				}
				if fnVal := event.Get("filename"); fnVal != nil && !goja.IsUndefined(fnVal) {
					filename = fnVal.String()
				}
				if lnVal := event.Get("lineno"); lnVal != nil && !goja.IsUndefined(lnVal) {
					lineno = lnVal.ToInteger()
				}
				if cnVal := event.Get("colno"); cnVal != nil && !goja.IsUndefined(cnVal) {
					colno = cnVal.ToInteger()
				}
				if errVal := event.Get("error"); errVal != nil {
					errorVal = errVal
				}

				_, err = l.callback(currentTarget,
					vm.ToValue(message),
					vm.ToValue(filename),
					vm.ToValue(lineno),
					vm.ToValue(colno),
					errorVal,
				)
			} else {
				_, err = l.callback(currentTarget, event)
			}
		}

		if err != nil {
			if et.errorHandler != nil {
				if exc, ok := err.(*goja.Exception); ok {
					et.errorHandler(exc.Value())
				} else {
					et.errorHandler(vm.ToValue(err.Error()))
				}
			}
		}

		// Check for stopImmediatePropagation
		if stopImmediate := event.Get("_stopImmediate"); stopImmediate != nil && stopImmediate.ToBoolean() {
			break
		}
	}
}

// HasEventListeners returns true if there are any listeners for the event type.
func (et *EventTarget) HasEventListeners(eventType string) bool {
	et.mu.RLock()
	defer et.mu.RUnlock()
	return len(et.listeners[eventType]) > 0
}

// NodeResolver is a function type that resolves parent nodes for event propagation.
// Given a JS object, it returns the parent JS object (or nil if no parent).
type NodeResolver func(obj *goja.Object) *goja.Object

// ShadowRootChecker is a function type that checks if a JS object is inside a shadow tree.
// Returns true if the object's root node is a ShadowRoot.
type ShadowRootChecker func(obj *goja.Object) bool

// ShadowHostResolver is a function type that returns the shadow host for a shadow root.
// Given a JS object representing a shadow root, returns the host element JS object.
// Returns nil if the object is not a shadow root.
type ShadowHostResolver func(obj *goja.Object) *goja.Object

// RelatedTargetRetargeter is a function type that retargets an object against another object.
// This implements the DOM spec "retarget" algorithm:
// https://dom.spec.whatwg.org/#retarget
// Given objects A and B, it returns the retargeted A against B.
// If A is in a shadow tree that is a shadow-including ancestor of B, it returns A.
// Otherwise, it returns the shadow host (walking up shadow boundaries as needed).
type RelatedTargetRetargeter func(a *goja.Object, b *goja.Object) *goja.Object

// ActivationResult holds the result of a pre-activation step.
type ActivationResult struct {
	Element          *goja.Object // The element with activation behavior
	PreviousChecked  bool         // Previous checked state (for checkbox/radio)
	HasActivation    bool         // Whether activation behavior was triggered
	ActivationType   string       // Type of activation ("checkbox", "radio", etc.)
}

// ActivationHandler is a function that handles activation behavior.
// It takes the event path and returns the activation result (or nil if no activation).
// If the event path contains an element with activation behavior, this handler runs
// the pre-click activation step and returns the result for possible rollback.
type ActivationHandler func(eventPath []*goja.Object, event *goja.Object) *ActivationResult

// ActivationCancelHandler is called when activation is canceled (defaultPrevented).
// It receives the activation result from the pre-activation step and reverts the state.
type ActivationCancelHandler func(result *ActivationResult)

// ActivationCompleteHandler is called when activation completes successfully (not canceled).
// It receives the activation result and fires input/change events as appropriate.
type ActivationCompleteHandler func(result *ActivationResult)

// EventBinder provides methods to add event handling to JS objects.
type EventBinder struct {
	runtime                     *Runtime
	targetMap                   map[*goja.Object]*EventTarget
	mu                          sync.RWMutex
	eventProtos                 map[string]*goja.Object // Map of event interface names to their prototypes
	nodeResolver                NodeResolver            // Function to resolve parent nodes for event propagation
	shadowRootChecker           ShadowRootChecker       // Function to check if a node is inside a shadow tree
	shadowHostResolver          ShadowHostResolver      // Function to get shadow host from shadow root
	relatedTargetRetargeter     RelatedTargetRetargeter // Function to retarget relatedTarget against currentTarget
	activationHandler           ActivationHandler       // Handler for pre-click activation behavior
	activationCancelHandler     ActivationCancelHandler // Handler for canceled activation
	activationCompleteHandler   ActivationCompleteHandler // Handler for successful activation
	listenerErrorHandler        ListenerErrorHandler    // Handler for exceptions in event listeners
}

// NewEventBinder creates a new event binder.
func NewEventBinder(runtime *Runtime) *EventBinder {
	return &EventBinder{
		runtime:     runtime,
		targetMap:   make(map[*goja.Object]*EventTarget),
		eventProtos: make(map[string]*goja.Object),
	}
}

// SetNodeResolver sets the function used to resolve parent nodes for event propagation.
func (eb *EventBinder) SetNodeResolver(resolver NodeResolver) {
	eb.nodeResolver = resolver
}

// SetShadowRootChecker sets the function used to check if a node is inside a shadow tree.
// This is used to determine whether window.event should be set during event dispatch.
// Per HTML spec, window.event should be undefined when listeners inside shadow trees are invoked.
func (eb *EventBinder) SetShadowRootChecker(checker ShadowRootChecker) {
	eb.shadowRootChecker = checker
}

// SetShadowHostResolver sets the function used to get the shadow host from a shadow root.
// This is used for composed events to propagate across shadow boundaries.
func (eb *EventBinder) SetShadowHostResolver(resolver ShadowHostResolver) {
	eb.shadowHostResolver = resolver
}

// SetRelatedTargetRetargeter sets the function used to retarget relatedTarget.
// This implements the DOM spec "retarget" algorithm for events like focus/blur
// that have a relatedTarget which should not leak shadow DOM internals.
func (eb *EventBinder) SetRelatedTargetRetargeter(retargeter RelatedTargetRetargeter) {
	eb.relatedTargetRetargeter = retargeter
}

// SetActivationHandlers sets the handlers for activation behavior.
// The activation handler runs pre-click activation for elements with activation behavior.
// The cancel handler reverts the activation if defaultPrevented is set.
// The complete handler fires input/change events after successful activation.
func (eb *EventBinder) SetActivationHandlers(handler ActivationHandler, cancelHandler ActivationCancelHandler, completeHandler ActivationCompleteHandler) {
	eb.activationHandler = handler
	eb.activationCancelHandler = cancelHandler
	eb.activationCompleteHandler = completeHandler
}

// SetListenerErrorHandler sets the handler for exceptions thrown by event listeners.
// This is typically used to call window.onerror.
func (eb *EventBinder) SetListenerErrorHandler(handler ListenerErrorHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.listenerErrorHandler = handler
	// Update existing targets
	for _, target := range eb.targetMap {
		target.SetErrorHandler(handler)
	}
}

// GetOrCreateTarget gets or creates an EventTarget for a JS object.
func (eb *EventBinder) GetOrCreateTarget(obj *goja.Object) *EventTarget {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if target, ok := eb.targetMap[obj]; ok {
		return target
	}

	target := NewEventTarget()
	// Set the error handler if one has been configured
	if eb.listenerErrorHandler != nil {
		target.SetErrorHandler(eb.listenerErrorHandler)
	}
	eb.targetMap[obj] = target
	return target
}

// BindEventTarget adds EventTarget interface methods to a JS object.
func (eb *EventBinder) BindEventTarget(obj *goja.Object) {
	vm := eb.runtime.vm

	obj.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
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
			if call.Arguments[2].ExportType().Kind().String() == "bool" {
				opts.capture = call.Arguments[2].ToBoolean()
			} else {
				optObj := call.Arguments[2].ToObject(vm)
				if optObj != nil {
					if v := optObj.Get("capture"); v != nil && !goja.IsUndefined(v) {
						opts.capture = v.ToBoolean()
					}
					if v := optObj.Get("once"); v != nil && !goja.IsUndefined(v) {
						opts.once = v.ToBoolean()
					}
					if v := optObj.Get("passive"); v != nil && !goja.IsUndefined(v) {
						opts.passive = v.ToBoolean()
					}
					if v := optObj.Get("signal"); v != nil && !goja.IsUndefined(v) {
						// Per DOM spec, if signal is null, throw TypeError
						if goja.IsNull(v) {
							panic(vm.NewTypeError("Failed to execute 'addEventListener': member signal is not of type AbortSignal."))
						}
						signalObj := v.ToObject(vm)
						if signalObj != nil {
							// Check if the signal is already aborted
							abortedVal := signalObj.Get("aborted")
							if abortedVal != nil && abortedVal.ToBoolean() {
								// Signal is already aborted, don't add the listener
								return goja.Undefined()
							}
							opts.signal = signalObj
						}
					}
				}
			}
		}

		target := eb.GetOrCreateTarget(obj)
		target.AddEventListener(eventType, callback, listenerArg, isObject, listenerObj, opts)
		return goja.Undefined()
	})

	obj.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}

		eventType := call.Arguments[0].String()
		capture := false
		if len(call.Arguments) > 2 {
			if call.Arguments[2].ExportType().Kind().String() == "bool" {
				capture = call.Arguments[2].ToBoolean()
			} else {
				optObj := call.Arguments[2].ToObject(vm)
				if optObj != nil {
					if v := optObj.Get("capture"); v != nil && !goja.IsUndefined(v) {
						capture = v.ToBoolean()
					}
				}
			}
		}

		target := eb.GetOrCreateTarget(obj)
		target.RemoveEventListener(eventType, call.Arguments[1], capture)
		return goja.Undefined()
	})

	obj.Set("dispatchEvent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}

		event := call.Arguments[0].ToObject(vm)
		if event == nil {
			return vm.ToValue(false)
		}

		// Per DOM spec: If event's dispatch flag is set, throw InvalidStateError
		dispatchFlag := event.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			panic(eb.createDOMException("InvalidStateError", "The event is already being dispatched."))
		}

		// Per DOM spec: If event's initialized flag is not set, throw InvalidStateError
		initializedFlag := event.Get("_initialized")
		if initializedFlag != nil && !initializedFlag.ToBoolean() {
			panic(eb.createDOMException("InvalidStateError", "The event object was not properly initialized."))
		}

		// Set dispatch flag
		event.Set("_dispatch", true)

		// Per DOM spec: When an event is dispatched by script, isTrusted becomes false
		// This handles the case where a trusted event is captured and re-dispatched by user script
		event.Set("isTrusted", false)

		// Set target
		event.Set("target", obj)

		// Per HTML spec: Save the previous window.event value to support nested event dispatch.
		// We will set window.event appropriately before each listener invocation based on
		// whether the currentTarget is inside a shadow tree.
		window := vm.Get("window")
		var previousWindowEvent goja.Value
		var windowObj *goja.Object
		if window != nil && !goja.IsUndefined(window) {
			windowObj = window.ToObject(vm)
			if windowObj != nil {
				previousWindowEvent = windowObj.Get("event")
			}
		}

		// Defer restoration of window.event
		defer func() {
			if windowObj != nil {
				windowObj.Set("event", previousWindowEvent)
			}
		}()

		// Helper to set window.event based on currentTarget.
		// Per HTML spec, window.event should be undefined when listeners inside shadow trees are invoked.
		setWindowEvent := func(currentTarget *goja.Object) {
			if windowObj == nil {
				return
			}
			// Check if currentTarget is inside a shadow tree
			if eb.shadowRootChecker != nil && eb.shadowRootChecker(currentTarget) {
				// Inside shadow tree - window.event should be undefined
				windowObj.Set("event", goja.Undefined())
			} else {
				// Outside shadow tree - window.event should be the event
				windowObj.Set("event", event)
			}
		}

		// Check if the event is composed (crosses shadow boundaries)
		composed := false
		if composedVal := event.Get("composed"); composedVal != nil {
			composed = composedVal.ToBoolean()
		}

		// Build event path from target up to root
		// The path is ordered from the target to the root (for bubbling)
		// For composed events, the path crosses shadow boundaries via the shadow host.
		eventPath := []*goja.Object{obj}
		if eb.nodeResolver != nil {
			current := obj
			for {
				parent := eb.nodeResolver(current)
				if parent == nil {
					// If no parent and event is composed, check if we're at a shadow root
					// and need to continue to the shadow host
					if composed && eb.shadowHostResolver != nil {
						host := eb.shadowHostResolver(current)
						if host != nil {
							// Continue from the shadow host
							eventPath = append(eventPath, host)
							current = host
							continue
						}
					}
					break
				}
				eventPath = append(eventPath, parent)
				current = parent
			}
		}

		// Store the path for composedPath() - need to store in event
		// composedPath returns path from target to root, but we need the full path
		// which during dispatch is from target through ancestors
		event.Set("_eventPath", eventPath)

		// Update composedPath to return the stored path
		event.Set("composedPath", func(call goja.FunctionCall) goja.Value {
			pathVal := event.Get("_eventPath")
			if pathVal == nil || goja.IsUndefined(pathVal) || goja.IsNull(pathVal) {
				return vm.ToValue([]interface{}{})
			}
			// Export and convert to array
			if exported, ok := pathVal.Export().([]*goja.Object); ok {
				result := make([]interface{}, len(exported))
				for i, obj := range exported {
					result[i] = obj
				}
				return vm.ToValue(result)
			}
			return vm.ToValue([]interface{}{})
		})

		bubbles := false
		if bubblesVal := event.Get("bubbles"); bubblesVal != nil {
			bubbles = bubblesVal.ToBoolean()
		}

		shouldStopPropagation := func() bool {
			stopProp := event.Get("_stopPropagation")
			return stopProp != nil && stopProp.ToBoolean()
		}

		// Pre-click activation behavior (for checkbox, radio, etc.)
		// Per HTML spec, activation behavior runs BEFORE the click event is dispatched.
		// This only applies to MouseEvent click events (not plain Event clicks).
		// See: https://html.spec.whatwg.org/multipage/webappapis.html#activation-behavior
		var activationResult *ActivationResult
		if eb.activationHandler != nil {
			activationResult = eb.activationHandler(eventPath, event)
		}

		// Get the original relatedTarget (if any) for retargeting during dispatch
		// Per DOM spec, relatedTarget must be retargeted against each currentTarget
		// to prevent leaking shadow DOM internals.
		originalRelatedTarget := event.Get("relatedTarget")
		hasRelatedTarget := originalRelatedTarget != nil && !goja.IsUndefined(originalRelatedTarget) && !goja.IsNull(originalRelatedTarget)
		var originalRelatedTargetObj *goja.Object
		if hasRelatedTarget {
			originalRelatedTargetObj = originalRelatedTarget.ToObject(vm)
		}


		// Helper to set retargeted relatedTarget for a given currentTarget
		setRetargetedRelatedTarget := func(currentTarget *goja.Object) {
			if !hasRelatedTarget {
				return
			}
			if eb.relatedTargetRetargeter == nil {
				return
			}
			retargeted := eb.relatedTargetRetargeter(originalRelatedTargetObj, currentTarget)
			if retargeted != nil {
				event.Set("relatedTarget", retargeted)
			} else {
				event.Set("relatedTarget", goja.Null())
			}
		}

		// Phase 1: Capturing phase (root to target, excluding target)
		// Walk from the end of eventPath (root) to the beginning (target)
		for i := len(eventPath) - 1; i > 0; i-- {
			if shouldStopPropagation() {
				break
			}
			currentTarget := eventPath[i]
			event.Set("currentTarget", currentTarget)
			event.Set("eventPhase", int(EventPhaseCapturing))
			setWindowEvent(currentTarget)
			setRetargetedRelatedTarget(currentTarget)
			target := eb.GetOrCreateTarget(currentTarget)
			target.DispatchEvent(vm, event, EventPhaseCapturing)
		}

		// Phase 2: At target (target itself)
		if !shouldStopPropagation() {
			event.Set("currentTarget", obj)
			event.Set("eventPhase", int(EventPhaseAtTarget))
			setWindowEvent(obj)
			setRetargetedRelatedTarget(obj)
			target := eb.GetOrCreateTarget(obj)
			target.DispatchEvent(vm, event, EventPhaseAtTarget)
		}

		// Phase 3: Bubbling phase (target to root, excluding target)
		if bubbles {
			for i := 1; i < len(eventPath); i++ {
				if shouldStopPropagation() {
					break
				}
				currentTarget := eventPath[i]
				event.Set("currentTarget", currentTarget)
				event.Set("eventPhase", int(EventPhaseBubbling))
				setWindowEvent(currentTarget)
				setRetargetedRelatedTarget(currentTarget)
				target := eb.GetOrCreateTarget(currentTarget)
				target.DispatchEvent(vm, event, EventPhaseBubbling)
			}
		}

		// Clear event path after dispatch
		event.Set("_eventPath", nil)

		// Clear dispatch flag and stop propagation flag after dispatch
		event.Set("_dispatch", false)
		event.Set("_stopPropagation", false)
		event.Set("_stopImmediate", false)
		event.Set("eventPhase", int(EventPhaseNone))
		event.Set("currentTarget", goja.Null())

		// Check if default was prevented
		defaultPrevented := false
		if dp := event.Get("defaultPrevented"); dp != nil {
			defaultPrevented = dp.ToBoolean()
		}

		// Handle post-activation behavior
		if activationResult != nil && activationResult.HasActivation {
			if defaultPrevented {
				// Legacy-canceled-activation-behavior: revert the activation
				// Per HTML spec, this reverts the checkbox/radio checked state if preventDefault was called
				if eb.activationCancelHandler != nil {
					eb.activationCancelHandler(activationResult)
				}
			} else {
				// Activation completed successfully - fire input and change events
				if eb.activationCompleteHandler != nil {
					eb.activationCompleteHandler(activationResult)
				}
			}
		}

		// Return true if default wasn't prevented
		return vm.ToValue(!defaultPrevented)
	})
}

// InitEventObject initializes an existing object with Event properties and methods.
// This is used by constructors to support ES6 class inheritance where call.This must be used.
func (eb *EventBinder) InitEventObject(event *goja.Object, eventType string) {
	vm := eb.runtime.vm

	event.Set("type", eventType)
	event.Set("target", goja.Null())
	event.Set("currentTarget", goja.Null())
	event.Set("eventPhase", int(EventPhaseNone))
	event.Set("bubbles", false)
	event.Set("cancelable", false)
	event.Set("composed", false)
	event.Set("defaultPrevented", false)
	event.Set("isTrusted", false)
	// DOMHighResTimeStamp: milliseconds since time origin
	event.Set("timeStamp", eb.runtime.Now())

	// Internal flags
	event.Set("_stopPropagation", false)
	event.Set("_stopImmediate", false)
	// Events created via constructor are initialized
	event.Set("_initialized", true)
	event.Set("_dispatch", false)

	// Methods
	event.Set("preventDefault", func(call goja.FunctionCall) goja.Value {
		if event.Get("cancelable").ToBoolean() {
			event.Set("defaultPrevented", true)
		}
		return goja.Undefined()
	})

	event.Set("stopPropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		return goja.Undefined()
	})

	event.Set("stopImmediatePropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		event.Set("_stopImmediate", true)
		return goja.Undefined()
	})

	event.Set("composedPath", func(call goja.FunctionCall) goja.Value {
		// Return the path of targets
		return vm.ToValue([]interface{}{})
	})

	// cancelBubble - legacy property that maps to stop propagation flag
	// Per DOM spec: getter returns stop propagation flag, setter can only set it to true
	event.DefineAccessorProperty("cancelBubble",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			stopProp := event.Get("_stopPropagation")
			if stopProp != nil {
				return vm.ToValue(stopProp.ToBoolean())
			}
			return vm.ToValue(false)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && call.Arguments[0].ToBoolean() {
				event.Set("_stopPropagation", true)
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// returnValue - legacy property that is the inverse of defaultPrevented
	// Per DOM spec: getter returns !defaultPrevented, setter calls preventDefault() when set to false
	event.DefineAccessorProperty("returnValue",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			defaultPrevented := event.Get("defaultPrevented")
			if defaultPrevented != nil {
				return vm.ToValue(!defaultPrevented.ToBoolean())
			}
			return vm.ToValue(true)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && !call.Arguments[0].ToBoolean() {
				// Setting returnValue to false is equivalent to calling preventDefault()
				if event.Get("cancelable").ToBoolean() {
					event.Set("defaultPrevented", true)
				}
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// srcElement - legacy alias for target per DOM spec
	// getter returns event.target, setter is a no-op (read-only)
	event.DefineAccessorProperty("srcElement",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return event.Get("target")
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// initEvent(type, bubbles, cancelable) - legacy method
	event.Set("initEvent", func(call goja.FunctionCall) goja.Value {
		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := event.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Per DOM spec, the type argument is required
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'initEvent' on 'Event': 1 argument required, but only 0 present."))
		}

		// Get type argument
		newType := call.Arguments[0].String()

		// Get bubbles argument
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}

		// Get cancelable argument
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}

		// Set the initialized flag
		event.Set("_initialized", true)
		event.Set("_stopPropagation", false)
		event.Set("_stopImmediate", false)
		event.Set("defaultPrevented", false)
		event.Set("type", newType)
		event.Set("bubbles", bubbles)
		event.Set("cancelable", cancelable)

		return goja.Undefined()
	})

	// Constants
	event.Set("NONE", int(EventPhaseNone))
	event.Set("CAPTURING_PHASE", int(EventPhaseCapturing))
	event.Set("AT_TARGET", int(EventPhaseAtTarget))
	event.Set("BUBBLING_PHASE", int(EventPhaseBubbling))
}

// CreateEvent creates a new Event object with the given prototype.
// Events created via the Event constructor (new Event(type)) are initialized by default.
func (eb *EventBinder) CreateEvent(eventType string, options map[string]interface{}) *goja.Object {
	vm := eb.runtime.vm
	event := vm.NewObject()

	event.Set("type", eventType)
	event.Set("target", goja.Null())
	event.Set("currentTarget", goja.Null())
	event.Set("eventPhase", int(EventPhaseNone))
	event.Set("bubbles", false)
	event.Set("cancelable", false)
	event.Set("composed", false)
	event.Set("defaultPrevented", false)
	event.Set("isTrusted", false)
	// DOMHighResTimeStamp: milliseconds since time origin
	event.Set("timeStamp", eb.runtime.Now())

	// Internal flags
	event.Set("_stopPropagation", false)
	event.Set("_stopImmediate", false)
	// Events created via constructor are initialized
	event.Set("_initialized", true)
	event.Set("_dispatch", false)

	// Apply options
	if options != nil {
		if v, ok := options["bubbles"]; ok {
			event.Set("bubbles", v)
		}
		if v, ok := options["cancelable"]; ok {
			event.Set("cancelable", v)
		}
		if v, ok := options["composed"]; ok {
			event.Set("composed", v)
		}
		if v, ok := options["detail"]; ok {
			event.Set("detail", v)
		}
	}

	// Methods
	event.Set("preventDefault", func(call goja.FunctionCall) goja.Value {
		if event.Get("cancelable").ToBoolean() {
			event.Set("defaultPrevented", true)
		}
		return goja.Undefined()
	})

	event.Set("stopPropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		return goja.Undefined()
	})

	event.Set("stopImmediatePropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		event.Set("_stopImmediate", true)
		return goja.Undefined()
	})

	event.Set("composedPath", func(call goja.FunctionCall) goja.Value {
		// Return the path of targets
		return vm.ToValue([]interface{}{})
	})

	// cancelBubble - legacy property that maps to stop propagation flag
	// Per DOM spec: getter returns stop propagation flag, setter can only set it to true
	event.DefineAccessorProperty("cancelBubble",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			stopProp := event.Get("_stopPropagation")
			if stopProp != nil {
				return vm.ToValue(stopProp.ToBoolean())
			}
			return vm.ToValue(false)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && call.Arguments[0].ToBoolean() {
				event.Set("_stopPropagation", true)
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// returnValue - legacy property that is the inverse of defaultPrevented
	// Per DOM spec: getter returns !defaultPrevented, setter calls preventDefault() when set to false
	event.DefineAccessorProperty("returnValue",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			defaultPrevented := event.Get("defaultPrevented")
			if defaultPrevented != nil {
				return vm.ToValue(!defaultPrevented.ToBoolean())
			}
			return vm.ToValue(true)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && !call.Arguments[0].ToBoolean() {
				// Setting returnValue to false is equivalent to calling preventDefault()
				if event.Get("cancelable").ToBoolean() {
					event.Set("defaultPrevented", true)
				}
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// srcElement - legacy alias for target per DOM spec
	// getter returns event.target, setter is a no-op (read-only)
	event.DefineAccessorProperty("srcElement",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return event.Get("target")
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// initEvent(type, bubbles, cancelable) - legacy method
	event.Set("initEvent", func(call goja.FunctionCall) goja.Value {
		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := event.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Per DOM spec, the type argument is required
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'initEvent' on 'Event': 1 argument required, but only 0 present."))
		}

		// Get type argument
		newType := call.Arguments[0].String()

		// Get bubbles argument
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}

		// Get cancelable argument
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}

		// Set the initialized flag
		event.Set("_initialized", true)
		event.Set("_stopPropagation", false)
		event.Set("_stopImmediate", false)
		event.Set("defaultPrevented", false)
		event.Set("type", newType)
		event.Set("bubbles", bubbles)
		event.Set("cancelable", cancelable)

		return goja.Undefined()
	})

	// Constants
	event.Set("NONE", int(EventPhaseNone))
	event.Set("CAPTURING_PHASE", int(EventPhaseCapturing))
	event.Set("AT_TARGET", int(EventPhaseAtTarget))
	event.Set("BUBBLING_PHASE", int(EventPhaseBubbling))

	return event
}

// GetEventProto returns the prototype for a given event interface.
func (eb *EventBinder) GetEventProto(name string) *goja.Object {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.eventProtos[name]
}

// createEventConstructor creates an event constructor with proper prototype chain.
// This supports ES6 class inheritance by using call.This and conditionally setting prototype.
func (eb *EventBinder) createEventConstructor(name string, parentProto *goja.Object, initFunc func(event *goja.Object, call goja.ConstructorCall)) {
	vm := eb.runtime.vm

	// Create prototype
	proto := vm.NewObject()
	if parentProto != nil {
		proto.SetPrototype(parentProto)
	}
	proto.Set("NONE", int(EventPhaseNone))
	proto.Set("CAPTURING_PHASE", int(EventPhaseCapturing))
	proto.Set("AT_TARGET", int(EventPhaseAtTarget))
	proto.Set("BUBBLING_PHASE", int(EventPhaseBubbling))

	// Get the Object.prototype for comparison
	objectProto := vm.GlobalObject().Get("Object").ToObject(vm).Get("prototype").ToObject(vm)

	// Create constructor
	ctor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		eventType := ""
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		// Use call.This to support ES6 class inheritance
		// When called via super() from a subclass, goja sets up the prototype chain on call.This
		event := call.This

		// Initialize event properties and methods on the object
		eb.InitEventObject(event, eventType)

		// Only set prototype if this is a direct construction (not from a subclass).
		// When called from a subclass via super(), the prototype is already set by goja.
		// We check if the prototype is Object.prototype (default for new objects) or nil.
		currentProto := event.Prototype()
		if currentProto == nil || currentProto == objectProto {
			event.SetPrototype(proto)
		}

		// Apply bubbles/cancelable from options
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("bubbles"); v != nil && !goja.IsUndefined(v) {
					event.Set("bubbles", v.ToBoolean())
				}
				if v := optObj.Get("cancelable"); v != nil && !goja.IsUndefined(v) {
					event.Set("cancelable", v.ToBoolean())
				}
				if v := optObj.Get("composed"); v != nil && !goja.IsUndefined(v) {
					event.Set("composed", v.ToBoolean())
				}
			}
		}

		// Call the init function for subclass-specific properties
		if initFunc != nil {
			initFunc(event, call)
		}

		return event
	})

	ctorObj := ctor.ToObject(vm)
	ctorObj.Set("prototype", proto)
	proto.Set("constructor", ctorObj)

	// Set constants on constructor
	ctorObj.Set("NONE", int(EventPhaseNone))
	ctorObj.Set("CAPTURING_PHASE", int(EventPhaseCapturing))
	ctorObj.Set("AT_TARGET", int(EventPhaseAtTarget))
	ctorObj.Set("BUBBLING_PHASE", int(EventPhaseBubbling))

	// Store prototype and set global
	eb.mu.Lock()
	eb.eventProtos[name] = proto
	eb.mu.Unlock()

	vm.Set(name, ctorObj)
}

// SetupEventConstructors sets up Event and related event constructors on the global object.
func (eb *EventBinder) SetupEventConstructors() {
	vm := eb.runtime.vm

	// EventTarget constructor - allows creating standalone EventTarget objects
	eventTargetProto := vm.NewObject()
	eventTargetCtor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		et := call.This
		et.SetPrototype(eventTargetProto)
		eb.BindEventTarget(et)
		return et
	})
	eventTargetCtorObj := eventTargetCtor.ToObject(vm)
	eventTargetCtorObj.Set("prototype", eventTargetProto)
	eventTargetProto.Set("constructor", eventTargetCtorObj)
	vm.Set("EventTarget", eventTargetCtorObj)

	// Store the EventTarget prototype
	eb.mu.Lock()
	eb.eventProtos["EventTarget"] = eventTargetProto
	eb.mu.Unlock()

	// Event - base event constructor
	eb.createEventConstructor("Event", nil, nil)
	eventProto := eb.GetEventProto("Event")

	// CustomEvent - extends Event
	eb.createEventConstructor("CustomEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("detail", goja.Null())
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v)
				}
			}
		}

		// initCustomEvent(type, bubbles, cancelable, detail) - legacy method
		// Per DOM spec, this is used to initialize CustomEvents created via createEvent()
		event.Set("initCustomEvent", func(call goja.FunctionCall) goja.Value {
			// Per DOM spec: If event's dispatch flag is set, terminate these steps
			dispatchFlag := event.Get("_dispatch")
			if dispatchFlag != nil && dispatchFlag.ToBoolean() {
				return goja.Undefined()
			}

			// Per DOM spec, the type argument is required
			if len(call.Arguments) < 1 {
				panic(vm.NewTypeError("Failed to execute 'initCustomEvent' on 'CustomEvent': 1 argument required, but only 0 present."))
			}

			// Get type argument
			newType := call.Arguments[0].String()

			// Get bubbles argument (optional, defaults to false)
			bubbles := false
			if len(call.Arguments) > 1 {
				bubbles = call.Arguments[1].ToBoolean()
			}

			// Get cancelable argument (optional, defaults to false)
			cancelable := false
			if len(call.Arguments) > 2 {
				cancelable = call.Arguments[2].ToBoolean()
			}

			// Get detail argument (optional, defaults to null)
			detail := goja.Null()
			if len(call.Arguments) > 3 {
				detail = call.Arguments[3]
			}

			// Set the initialized flag and properties
			event.Set("_initialized", true)
			event.Set("_stopPropagation", false)
			event.Set("_stopImmediate", false)
			event.Set("defaultPrevented", false)
			event.Set("type", newType)
			event.Set("bubbles", bubbles)
			event.Set("cancelable", cancelable)
			event.Set("detail", detail)

			return goja.Undefined()
		})
	})

	// UIEvent - extends Event
	eb.createEventConstructor("UIEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					// view must be null or a Window object (the global object)
					// Per UIEvents spec, view should be null, undefined, or a Window
					if !goja.IsNull(v) && !goja.IsUndefined(v) {
						// Check if it's a Window (global object)
						viewObj := v.ToObject(vm)
						globalObj := vm.GlobalObject()
						if viewObj == nil || viewObj != globalObj {
							panic(vm.NewTypeError("Failed to construct 'UIEvent': member view is not of type Window."))
						}
					}
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
			}
		}
	})
	uiEventProto := eb.GetEventProto("UIEvent")

	// Add initUIEvent legacy method to UIEvent prototype
	uiEventProto.Set("initUIEvent", func(call goja.FunctionCall) goja.Value {
		// Get the 'this' event object
		thisObj := call.This.ToObject(vm)
		if thisObj == nil {
			return goja.Undefined()
		}

		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := thisObj.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Get arguments: initUIEvent(type, bubbles, cancelable, view, detail)
		typeArg := ""
		if len(call.Arguments) > 0 {
			typeArg = call.Arguments[0].String()
		}
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}
		view := goja.Null()
		if len(call.Arguments) > 3 && !goja.IsUndefined(call.Arguments[3]) && !goja.IsNull(call.Arguments[3]) {
			view = call.Arguments[3]
		}
		detail := int64(0)
		if len(call.Arguments) > 4 {
			detail = call.Arguments[4].ToInteger()
		}

		// Initialize base Event properties
		thisObj.Set("_initialized", true)
		thisObj.Set("_stopPropagation", false)
		thisObj.Set("_stopImmediate", false)
		thisObj.Set("defaultPrevented", false)
		thisObj.Set("type", typeArg)
		thisObj.Set("bubbles", bubbles)
		thisObj.Set("cancelable", cancelable)

		// Initialize UIEvent properties
		thisObj.Set("view", view)
		thisObj.Set("detail", detail)

		return goja.Undefined()
	})

	// MouseEvent - extends UIEvent
	eb.createEventConstructor("MouseEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		// Set UIEvent defaults
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		// Set MouseEvent defaults
		event.Set("screenX", 0)
		event.Set("screenY", 0)
		event.Set("clientX", 0)
		event.Set("clientY", 0)
		event.Set("ctrlKey", false)
		event.Set("shiftKey", false)
		event.Set("altKey", false)
		event.Set("metaKey", false)
		event.Set("button", 0)
		event.Set("buttons", 0)
		event.Set("relatedTarget", goja.Null())
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				// UIEvent properties
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
				// MouseEvent properties
				if v := optObj.Get("screenX"); v != nil && !goja.IsUndefined(v) {
					event.Set("screenX", v.ToInteger())
				}
				if v := optObj.Get("screenY"); v != nil && !goja.IsUndefined(v) {
					event.Set("screenY", v.ToInteger())
				}
				if v := optObj.Get("clientX"); v != nil && !goja.IsUndefined(v) {
					event.Set("clientX", v.ToInteger())
				}
				if v := optObj.Get("clientY"); v != nil && !goja.IsUndefined(v) {
					event.Set("clientY", v.ToInteger())
				}
				if v := optObj.Get("button"); v != nil && !goja.IsUndefined(v) {
					event.Set("button", v.ToInteger())
				}
				if v := optObj.Get("buttons"); v != nil && !goja.IsUndefined(v) {
					event.Set("buttons", v.ToInteger())
				}
				if v := optObj.Get("relatedTarget"); v != nil && !goja.IsUndefined(v) {
					event.Set("relatedTarget", v)
				}
				// EventModifierInit properties
				if v := optObj.Get("ctrlKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("ctrlKey", v.ToBoolean())
				}
				if v := optObj.Get("shiftKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("shiftKey", v.ToBoolean())
				}
				if v := optObj.Get("altKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("altKey", v.ToBoolean())
				}
				if v := optObj.Get("metaKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("metaKey", v.ToBoolean())
				}
			}
		}
	})

	// Add initMouseEvent legacy method to MouseEvent prototype
	mouseEventProtoForInit := eb.GetEventProto("MouseEvent")
	mouseEventProtoForInit.Set("initMouseEvent", func(call goja.FunctionCall) goja.Value {
		// Get the 'this' event object
		thisObj := call.This.ToObject(vm)
		if thisObj == nil {
			return goja.Undefined()
		}

		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := thisObj.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Get arguments: initMouseEvent(type, bubbles, cancelable, view, detail,
		//   screenX, screenY, clientX, clientY, ctrlKey, altKey, shiftKey, metaKey, button, relatedTarget)
		typeArg := ""
		if len(call.Arguments) > 0 {
			typeArg = call.Arguments[0].String()
		}
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}
		view := goja.Null()
		if len(call.Arguments) > 3 && !goja.IsUndefined(call.Arguments[3]) && !goja.IsNull(call.Arguments[3]) {
			view = call.Arguments[3]
		}
		detail := int64(0)
		if len(call.Arguments) > 4 {
			detail = call.Arguments[4].ToInteger()
		}
		screenX := int64(0)
		if len(call.Arguments) > 5 {
			screenX = call.Arguments[5].ToInteger()
		}
		screenY := int64(0)
		if len(call.Arguments) > 6 {
			screenY = call.Arguments[6].ToInteger()
		}
		clientX := int64(0)
		if len(call.Arguments) > 7 {
			clientX = call.Arguments[7].ToInteger()
		}
		clientY := int64(0)
		if len(call.Arguments) > 8 {
			clientY = call.Arguments[8].ToInteger()
		}
		ctrlKey := false
		if len(call.Arguments) > 9 {
			ctrlKey = call.Arguments[9].ToBoolean()
		}
		altKey := false
		if len(call.Arguments) > 10 {
			altKey = call.Arguments[10].ToBoolean()
		}
		shiftKey := false
		if len(call.Arguments) > 11 {
			shiftKey = call.Arguments[11].ToBoolean()
		}
		metaKey := false
		if len(call.Arguments) > 12 {
			metaKey = call.Arguments[12].ToBoolean()
		}
		button := int64(0)
		if len(call.Arguments) > 13 {
			button = call.Arguments[13].ToInteger()
		}
		relatedTarget := goja.Null()
		if len(call.Arguments) > 14 && !goja.IsUndefined(call.Arguments[14]) && !goja.IsNull(call.Arguments[14]) {
			relatedTarget = call.Arguments[14]
		}

		// Initialize base Event properties
		thisObj.Set("_initialized", true)
		thisObj.Set("_stopPropagation", false)
		thisObj.Set("_stopImmediate", false)
		thisObj.Set("defaultPrevented", false)
		thisObj.Set("type", typeArg)
		thisObj.Set("bubbles", bubbles)
		thisObj.Set("cancelable", cancelable)

		// Initialize UIEvent properties
		thisObj.Set("view", view)
		thisObj.Set("detail", detail)

		// Initialize MouseEvent properties
		thisObj.Set("screenX", screenX)
		thisObj.Set("screenY", screenY)
		thisObj.Set("clientX", clientX)
		thisObj.Set("clientY", clientY)
		thisObj.Set("ctrlKey", ctrlKey)
		thisObj.Set("altKey", altKey)
		thisObj.Set("shiftKey", shiftKey)
		thisObj.Set("metaKey", metaKey)
		thisObj.Set("button", button)
		thisObj.Set("relatedTarget", relatedTarget)

		return goja.Undefined()
	})

	// FocusEvent - extends UIEvent
	eb.createEventConstructor("FocusEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		// Set UIEvent defaults
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		// Set FocusEvent defaults
		event.Set("relatedTarget", goja.Null())
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				// UIEvent properties
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
				// FocusEvent properties
				if v := optObj.Get("relatedTarget"); v != nil && !goja.IsUndefined(v) {
					event.Set("relatedTarget", v)
				}
			}
		}
	})

	// KeyboardEvent - extends UIEvent
	eb.createEventConstructor("KeyboardEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		// Set UIEvent defaults
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		// Set KeyboardEvent defaults
		event.Set("key", "")
		event.Set("code", "")
		event.Set("location", 0)
		event.Set("ctrlKey", false)
		event.Set("shiftKey", false)
		event.Set("altKey", false)
		event.Set("metaKey", false)
		event.Set("repeat", false)
		event.Set("isComposing", false)
		event.Set("charCode", 0)
		event.Set("keyCode", 0)
		event.Set("which", 0)
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				// UIEvent properties
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
				// KeyboardEvent properties
				if v := optObj.Get("key"); v != nil && !goja.IsUndefined(v) {
					event.Set("key", v.String())
				}
				if v := optObj.Get("code"); v != nil && !goja.IsUndefined(v) {
					event.Set("code", v.String())
				}
				if v := optObj.Get("location"); v != nil && !goja.IsUndefined(v) {
					event.Set("location", v.ToInteger())
				}
				if v := optObj.Get("repeat"); v != nil && !goja.IsUndefined(v) {
					event.Set("repeat", v.ToBoolean())
				}
				if v := optObj.Get("isComposing"); v != nil && !goja.IsUndefined(v) {
					event.Set("isComposing", v.ToBoolean())
				}
				if v := optObj.Get("charCode"); v != nil && !goja.IsUndefined(v) {
					event.Set("charCode", v.ToInteger())
				}
				if v := optObj.Get("keyCode"); v != nil && !goja.IsUndefined(v) {
					event.Set("keyCode", v.ToInteger())
				}
				if v := optObj.Get("which"); v != nil && !goja.IsUndefined(v) {
					event.Set("which", v.ToInteger())
				}
				// EventModifierInit properties
				if v := optObj.Get("ctrlKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("ctrlKey", v.ToBoolean())
				}
				if v := optObj.Get("shiftKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("shiftKey", v.ToBoolean())
				}
				if v := optObj.Get("altKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("altKey", v.ToBoolean())
				}
				if v := optObj.Get("metaKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("metaKey", v.ToBoolean())
				}
			}
		}
	})

	// Add initKeyboardEvent legacy method to KeyboardEvent prototype
	keyboardEventProtoForInit := eb.GetEventProto("KeyboardEvent")
	keyboardEventProtoForInit.Set("initKeyboardEvent", func(call goja.FunctionCall) goja.Value {
		// Get the 'this' event object
		thisObj := call.This.ToObject(vm)
		if thisObj == nil {
			return goja.Undefined()
		}

		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := thisObj.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Get arguments: initKeyboardEvent(type, bubbles, cancelable, view, key, location, ctrlKey, altKey, shiftKey, metaKey)
		// Note: The old spec had different arguments, modern spec is simplified
		typeArg := ""
		if len(call.Arguments) > 0 {
			typeArg = call.Arguments[0].String()
		}
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}
		view := goja.Null()
		if len(call.Arguments) > 3 && !goja.IsUndefined(call.Arguments[3]) && !goja.IsNull(call.Arguments[3]) {
			view = call.Arguments[3]
		}
		key := ""
		if len(call.Arguments) > 4 {
			key = call.Arguments[4].String()
		}
		location := int64(0)
		if len(call.Arguments) > 5 {
			location = call.Arguments[5].ToInteger()
		}
		ctrlKey := false
		if len(call.Arguments) > 6 {
			ctrlKey = call.Arguments[6].ToBoolean()
		}
		altKey := false
		if len(call.Arguments) > 7 {
			altKey = call.Arguments[7].ToBoolean()
		}
		shiftKey := false
		if len(call.Arguments) > 8 {
			shiftKey = call.Arguments[8].ToBoolean()
		}
		metaKey := false
		if len(call.Arguments) > 9 {
			metaKey = call.Arguments[9].ToBoolean()
		}

		// Initialize base Event properties
		thisObj.Set("_initialized", true)
		thisObj.Set("_stopPropagation", false)
		thisObj.Set("_stopImmediate", false)
		thisObj.Set("defaultPrevented", false)
		thisObj.Set("type", typeArg)
		thisObj.Set("bubbles", bubbles)
		thisObj.Set("cancelable", cancelable)

		// Initialize UIEvent properties
		thisObj.Set("view", view)
		thisObj.Set("detail", 0) // KeyboardEvent always has detail of 0

		// Initialize KeyboardEvent properties
		thisObj.Set("key", key)
		thisObj.Set("location", location)
		thisObj.Set("ctrlKey", ctrlKey)
		thisObj.Set("altKey", altKey)
		thisObj.Set("shiftKey", shiftKey)
		thisObj.Set("metaKey", metaKey)

		return goja.Undefined()
	})

	// CompositionEvent - extends UIEvent
	eb.createEventConstructor("CompositionEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		// Set UIEvent defaults
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		// Set CompositionEvent defaults
		event.Set("data", "")
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				// UIEvent properties
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
				// CompositionEvent properties
				if v := optObj.Get("data"); v != nil && !goja.IsUndefined(v) {
					event.Set("data", v.String())
				}
			}
		}
	})

	// TextEvent - extends UIEvent (deprecated but needed for compatibility)
	eb.createEventConstructor("TextEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		event.Set("data", "")
	})

	// MessageEvent - extends Event
	eb.createEventConstructor("MessageEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("data", goja.Null())
		event.Set("origin", "")
		event.Set("lastEventId", "")
		event.Set("source", goja.Null())
		event.Set("ports", vm.ToValue([]interface{}{}))
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("data"); v != nil && !goja.IsUndefined(v) {
					event.Set("data", v)
				}
				if v := optObj.Get("origin"); v != nil && !goja.IsUndefined(v) {
					event.Set("origin", v.String())
				}
			}
		}
	})

	// StorageEvent - extends Event
	eb.createEventConstructor("StorageEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("key", goja.Null())
		event.Set("oldValue", goja.Null())
		event.Set("newValue", goja.Null())
		event.Set("url", "")
		event.Set("storageArea", goja.Null())
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("key"); v != nil && !goja.IsUndefined(v) {
					event.Set("key", v)
				}
				if v := optObj.Get("oldValue"); v != nil && !goja.IsUndefined(v) {
					event.Set("oldValue", v)
				}
				if v := optObj.Get("newValue"); v != nil && !goja.IsUndefined(v) {
					event.Set("newValue", v)
				}
			}
		}
	})

	// HashChangeEvent - extends Event
	eb.createEventConstructor("HashChangeEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("oldURL", "")
		event.Set("newURL", "")
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("oldURL"); v != nil && !goja.IsUndefined(v) {
					event.Set("oldURL", v.String())
				}
				if v := optObj.Get("newURL"); v != nil && !goja.IsUndefined(v) {
					event.Set("newURL", v.String())
				}
			}
		}
	})

	// BeforeUnloadEvent - extends Event
	eb.createEventConstructor("BeforeUnloadEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("returnValue", "")
	})

	// DeviceMotionEvent - extends Event
	eb.createEventConstructor("DeviceMotionEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("acceleration", goja.Null())
		event.Set("accelerationIncludingGravity", goja.Null())
		event.Set("rotationRate", goja.Null())
		event.Set("interval", 0)
	})

	// DeviceOrientationEvent - extends Event
	eb.createEventConstructor("DeviceOrientationEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("alpha", goja.Null())
		event.Set("beta", goja.Null())
		event.Set("gamma", goja.Null())
		event.Set("absolute", false)
	})

	// DragEvent - extends MouseEvent
	mouseEventProto := eb.GetEventProto("MouseEvent")
	eb.createEventConstructor("DragEvent", mouseEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		event.Set("screenX", 0)
		event.Set("screenY", 0)
		event.Set("clientX", 0)
		event.Set("clientY", 0)
		event.Set("ctrlKey", false)
		event.Set("shiftKey", false)
		event.Set("altKey", false)
		event.Set("metaKey", false)
		event.Set("button", 0)
		event.Set("buttons", 0)
		event.Set("relatedTarget", goja.Null())
		event.Set("dataTransfer", goja.Null())
	})

	// WheelEvent - extends MouseEvent
	eb.createEventConstructor("WheelEvent", mouseEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		// Set UIEvent defaults
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		// Set MouseEvent defaults
		event.Set("screenX", 0)
		event.Set("screenY", 0)
		event.Set("clientX", 0)
		event.Set("clientY", 0)
		event.Set("ctrlKey", false)
		event.Set("shiftKey", false)
		event.Set("altKey", false)
		event.Set("metaKey", false)
		event.Set("button", 0)
		event.Set("buttons", 0)
		event.Set("relatedTarget", goja.Null())
		// Set WheelEvent-specific defaults
		event.Set("deltaX", 0.0)
		event.Set("deltaY", 0.0)
		event.Set("deltaZ", 0.0)
		event.Set("deltaMode", 0) // 0 = DOM_DELTA_PIXEL, 1 = DOM_DELTA_LINE, 2 = DOM_DELTA_PAGE
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				// UIEvent properties
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
				// MouseEvent properties
				if v := optObj.Get("screenX"); v != nil && !goja.IsUndefined(v) {
					event.Set("screenX", v.ToInteger())
				}
				if v := optObj.Get("screenY"); v != nil && !goja.IsUndefined(v) {
					event.Set("screenY", v.ToInteger())
				}
				if v := optObj.Get("clientX"); v != nil && !goja.IsUndefined(v) {
					event.Set("clientX", v.ToInteger())
				}
				if v := optObj.Get("clientY"); v != nil && !goja.IsUndefined(v) {
					event.Set("clientY", v.ToInteger())
				}
				if v := optObj.Get("button"); v != nil && !goja.IsUndefined(v) {
					event.Set("button", v.ToInteger())
				}
				if v := optObj.Get("buttons"); v != nil && !goja.IsUndefined(v) {
					event.Set("buttons", v.ToInteger())
				}
				if v := optObj.Get("relatedTarget"); v != nil && !goja.IsUndefined(v) {
					event.Set("relatedTarget", v)
				}
				// EventModifierInit properties
				if v := optObj.Get("ctrlKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("ctrlKey", v.ToBoolean())
				}
				if v := optObj.Get("shiftKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("shiftKey", v.ToBoolean())
				}
				if v := optObj.Get("altKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("altKey", v.ToBoolean())
				}
				if v := optObj.Get("metaKey"); v != nil && !goja.IsUndefined(v) {
					event.Set("metaKey", v.ToBoolean())
				}
				// WheelEvent-specific properties
				if v := optObj.Get("deltaX"); v != nil && !goja.IsUndefined(v) {
					event.Set("deltaX", v.ToFloat())
				}
				if v := optObj.Get("deltaY"); v != nil && !goja.IsUndefined(v) {
					event.Set("deltaY", v.ToFloat())
				}
				if v := optObj.Get("deltaZ"); v != nil && !goja.IsUndefined(v) {
					event.Set("deltaZ", v.ToFloat())
				}
				if v := optObj.Get("deltaMode"); v != nil && !goja.IsUndefined(v) {
					event.Set("deltaMode", v.ToInteger())
				}
			}
		}
	})

	// TouchEvent - extends UIEvent
	eb.createEventConstructor("TouchEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		event.Set("touches", vm.ToValue([]interface{}{}))
		event.Set("targetTouches", vm.ToValue([]interface{}{}))
		event.Set("changedTouches", vm.ToValue([]interface{}{}))
		event.Set("ctrlKey", false)
		event.Set("shiftKey", false)
		event.Set("altKey", false)
		event.Set("metaKey", false)
	})

	// ErrorEvent - extends Event
	eb.createEventConstructor("ErrorEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("message", "")
		event.Set("filename", "")
		event.Set("lineno", 0)
		event.Set("colno", 0)
		event.Set("error", goja.Null())
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) && !goja.IsNull(call.Arguments[1]) {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("message"); v != nil && !goja.IsUndefined(v) {
					event.Set("message", v.String())
				}
				if v := optObj.Get("filename"); v != nil && !goja.IsUndefined(v) {
					event.Set("filename", v.String())
				}
				if v := optObj.Get("lineno"); v != nil && !goja.IsUndefined(v) {
					event.Set("lineno", v.ToInteger())
				}
				if v := optObj.Get("colno"); v != nil && !goja.IsUndefined(v) {
					event.Set("colno", v.ToInteger())
				}
				if v := optObj.Get("error"); v != nil && !goja.IsUndefined(v) {
					event.Set("error", v)
				}
			}
		}
	})

	// Set up AbortController and AbortSignal
	eb.setupAbortController()
}

// setupAbortController sets up the AbortController and AbortSignal constructors.
// Per DOM spec:
// - AbortController has a signal property (AbortSignal) and an abort(reason) method
// - AbortSignal is an EventTarget with aborted property, reason property, and throwIfAborted() method
// - AbortSignal has static methods: abort(reason) and timeout(milliseconds)
func (eb *EventBinder) setupAbortController() {
	vm := eb.runtime.vm
	eventProto := eb.GetEventProto("Event")

	// Create AbortSignal prototype - AbortSignal extends EventTarget
	abortSignalProto := vm.NewObject()

	// AbortSignal constructor (not directly constructible by users)
	abortSignalCtor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Per DOM spec, AbortSignal cannot be directly constructed
		panic(vm.NewTypeError("Illegal constructor"))
	})
	abortSignalCtorObj := abortSignalCtor.ToObject(vm)
	abortSignalCtorObj.Set("prototype", abortSignalProto)
	abortSignalProto.Set("constructor", abortSignalCtorObj)

	// AbortSignal.abort(reason) - static method that returns an already-aborted signal
	abortSignalCtorObj.Set("abort", func(call goja.FunctionCall) goja.Value {
		signal := eb.createAbortSignal(true) // Create already-aborted signal

		// Set the abort reason
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
			signal.Set("reason", call.Arguments[0])
		} else {
			// Default reason is AbortError DOMException
			signal.Set("reason", eb.createDOMException("AbortError", "signal is aborted without reason"))
		}

		return signal
	})

	// AbortSignal.timeout(milliseconds) - static method that aborts after a delay
	abortSignalCtorObj.Set("timeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'timeout' on 'AbortSignal': 1 argument required"))
		}

		ms := call.Arguments[0].ToInteger()
		signal := eb.createAbortSignal(false) // Create non-aborted signal

		// Schedule the abort after the timeout
		// Note: This requires setTimeout to be available in the runtime
		setTimeoutVal := vm.Get("setTimeout")
		if setTimeoutVal != nil && !goja.IsUndefined(setTimeoutVal) {
			if setTimeout, ok := goja.AssertFunction(setTimeoutVal); ok {
				// Create a callback that aborts the signal with TimeoutError
				callback := vm.ToValue(func(call goja.FunctionCall) goja.Value {
					// Only abort if not already aborted
					if !signal.Get("aborted").ToBoolean() {
						signal.Set("aborted", true)
						signal.Set("reason", eb.createDOMException("TimeoutError", "signal timed out"))

						// Dispatch abort event
						eb.dispatchAbortEvent(signal)
					}
					return goja.Undefined()
				})
				setTimeout(goja.Undefined(), callback, vm.ToValue(ms))
			}
		}

		return signal
	})

	// AbortSignal.any(signals) - static method that returns a signal that aborts when any of the given signals abort
	abortSignalCtorObj.Set("any", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'any' on 'AbortSignal': 1 argument required"))
		}

		// Get the iterable of signals
		signalsArg := call.Arguments[0]
		if goja.IsNull(signalsArg) || goja.IsUndefined(signalsArg) {
			panic(vm.NewTypeError("Failed to execute 'any' on 'AbortSignal': The provided value is not iterable"))
		}

		signalsObj := signalsArg.ToObject(vm)
		if signalsObj == nil {
			panic(vm.NewTypeError("Failed to execute 'any' on 'AbortSignal': The provided value is not iterable"))
		}

		// Check if any signal is already aborted
		lengthVal := signalsObj.Get("length")
		if lengthVal == nil || goja.IsUndefined(lengthVal) {
			// Not an array-like object
			panic(vm.NewTypeError("Failed to execute 'any' on 'AbortSignal': The provided value is not iterable"))
		}

		length := int(lengthVal.ToInteger())
		for i := 0; i < length; i++ {
			signalVal := signalsObj.Get(vm.ToValue(i).String())
			if signalVal == nil || goja.IsUndefined(signalVal) {
				continue
			}
			signalObj := signalVal.ToObject(vm)
			if signalObj == nil {
				continue
			}
			// Check if this signal is already aborted
			abortedVal := signalObj.Get("aborted")
			if abortedVal != nil && abortedVal.ToBoolean() {
				// Return a signal that's already aborted with this signal's reason
				result := eb.createAbortSignal(true)
				reasonVal := signalObj.Get("reason")
				if reasonVal != nil && !goja.IsUndefined(reasonVal) {
					result.Set("reason", reasonVal)
				} else {
					result.Set("reason", eb.createDOMException("AbortError", "signal is aborted without reason"))
				}
				return result
			}
		}

		// Create a new signal that will abort when any of the source signals abort
		resultSignal := eb.createAbortSignal(false)

		// Add abort listeners to each source signal
		for i := 0; i < length; i++ {
			signalVal := signalsObj.Get(vm.ToValue(i).String())
			if signalVal == nil || goja.IsUndefined(signalVal) {
				continue
			}
			signalObj := signalVal.ToObject(vm)
			if signalObj == nil {
				continue
			}

			// Add an abort listener
			addEventListenerVal := signalObj.Get("addEventListener")
			if addEventListenerVal != nil && !goja.IsUndefined(addEventListenerVal) {
				if addEventListenerFn, ok := goja.AssertFunction(addEventListenerVal); ok {
					// Create a callback that aborts the result signal
					localSignalObj := signalObj // Capture for closure
					callback := vm.ToValue(func(call goja.FunctionCall) goja.Value {
						// Only abort if not already aborted
						if !resultSignal.Get("aborted").ToBoolean() {
							resultSignal.Set("aborted", true)
							// Copy the reason from the source signal
							reasonVal := localSignalObj.Get("reason")
							if reasonVal != nil && !goja.IsUndefined(reasonVal) {
								resultSignal.Set("reason", reasonVal)
							} else {
								resultSignal.Set("reason", eb.createDOMException("AbortError", "signal is aborted without reason"))
							}
							// Dispatch abort event
							eb.dispatchAbortEvent(resultSignal)
						}
						return goja.Undefined()
					})
					addEventListenerFn(signalObj, vm.ToValue("abort"), callback)
				}
			}
		}

		return resultSignal
	})

	vm.Set("AbortSignal", abortSignalCtorObj)

	// Store the AbortSignal prototype for later use
	eb.mu.Lock()
	eb.eventProtos["AbortSignal"] = abortSignalProto
	eb.mu.Unlock()

	// Create AbortController constructor
	abortControllerProto := vm.NewObject()

	abortControllerCtor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		controller := call.This
		controller.SetPrototype(abortControllerProto)

		// Create the associated AbortSignal
		signal := eb.createAbortSignal(false)
		controller.Set("signal", signal)

		// Store a reference to the controller on the signal for internal use
		signal.Set("_controller", controller)

		// abort(reason) method
		controller.Set("abort", func(call goja.FunctionCall) goja.Value {
			// Get the signal
			signalVal := controller.Get("signal")
			if signalVal == nil || goja.IsUndefined(signalVal) {
				return goja.Undefined()
			}
			signalObj := signalVal.ToObject(vm)
			if signalObj == nil {
				return goja.Undefined()
			}

			// If already aborted, do nothing
			if signalObj.Get("aborted").ToBoolean() {
				return goja.Undefined()
			}

			// Set aborted to true
			signalObj.Set("aborted", true)

			// Set the abort reason
			if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
				signalObj.Set("reason", call.Arguments[0])
			} else {
				// Default reason is AbortError DOMException
				signalObj.Set("reason", eb.createDOMException("AbortError", "signal is aborted without reason"))
			}

			// Dispatch "abort" event on the signal
			eb.dispatchAbortEvent(signalObj)

			return goja.Undefined()
		})

		return controller
	})

	abortControllerCtorObj := abortControllerCtor.ToObject(vm)
	abortControllerCtorObj.Set("prototype", abortControllerProto)
	abortControllerProto.Set("constructor", abortControllerCtorObj)

	vm.Set("AbortController", abortControllerCtorObj)

	// Store event proto reference for AbortSignal event dispatch
	_ = eventProto

	// Make event-related interface objects non-enumerable on the global object
	// Per the Web IDL specification, interface objects should be defined with enumerable: false
	_, _ = vm.RunString(`
		(function() {
			var eventInterfaces = [
				"EventTarget", "Event", "CustomEvent",
				"UIEvent", "MouseEvent", "FocusEvent", "KeyboardEvent",
				"CompositionEvent", "TextEvent", "MessageEvent", "StorageEvent",
				"HashChangeEvent", "BeforeUnloadEvent", "DeviceMotionEvent",
				"DeviceOrientationEvent", "DragEvent", "WheelEvent", "TouchEvent",
				"ErrorEvent", "AbortController", "AbortSignal"
			];
			var globalObj = typeof window !== 'undefined' ? window : this;
			eventInterfaces.forEach(function(name) {
				if (name in globalObj) {
					Object.defineProperty(globalObj, name, {
						value: globalObj[name],
						writable: true,
						enumerable: false,
						configurable: true
					});
				}
			});
		})();
	`)
}

// createAbortSignal creates a new AbortSignal object.
func (eb *EventBinder) createAbortSignal(aborted bool) *goja.Object {
	vm := eb.runtime.vm

	signal := vm.NewObject()

	// Get and set the AbortSignal prototype
	eb.mu.RLock()
	proto := eb.eventProtos["AbortSignal"]
	eb.mu.RUnlock()
	if proto != nil {
		signal.SetPrototype(proto)
	}

	// Set initial properties
	signal.Set("aborted", aborted)
	if aborted {
		signal.Set("reason", eb.createDOMException("AbortError", "signal is aborted without reason"))
	} else {
		signal.Set("reason", goja.Undefined())
	}

	// onabort event handler property
	signal.Set("onabort", goja.Null())

	// throwIfAborted() method
	signal.Set("throwIfAborted", func(call goja.FunctionCall) goja.Value {
		if signal.Get("aborted").ToBoolean() {
			reason := signal.Get("reason")
			if reason != nil && !goja.IsUndefined(reason) {
				panic(reason)
			}
			panic(eb.createDOMException("AbortError", "signal is aborted without reason"))
		}
		return goja.Undefined()
	})

	// Make the signal an EventTarget
	eb.BindEventTarget(signal)

	return signal
}

// dispatchAbortEvent dispatches an "abort" event on the given AbortSignal.
func (eb *EventBinder) dispatchAbortEvent(signal *goja.Object) {
	vm := eb.runtime.vm

	// Create the abort event
	abortEvent := eb.CreateEvent("abort", map[string]interface{}{
		"bubbles":    false,
		"cancelable": false,
	})

	// Dispatch the event
	dispatchEventVal := signal.Get("dispatchEvent")
	if dispatchEventVal != nil && !goja.IsUndefined(dispatchEventVal) {
		if dispatchEventFn, ok := goja.AssertFunction(dispatchEventVal); ok {
			dispatchEventFn(signal, abortEvent)
		}
	}

	// Also call the onabort handler if set
	onabortVal := signal.Get("onabort")
	if onabortVal != nil && !goja.IsNull(onabortVal) && !goja.IsUndefined(onabortVal) {
		if onabortFn, ok := goja.AssertFunction(onabortVal); ok {
			onabortFn(signal, abortEvent)
		}
	}

	// Trigger removal of all listeners that were added with this signal
	eb.removeListenersForSignal(signal)

	_ = vm
}

// removeListenersForSignal removes all event listeners that were registered with the given signal.
func (eb *EventBinder) removeListenersForSignal(signal *goja.Object) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Iterate over all targets and remove listeners that have this signal
	for _, target := range eb.targetMap {
		target.mu.Lock()
		for eventType, listeners := range target.listeners {
			// Filter out listeners with this signal
			filtered := make([]*eventListener, 0, len(listeners))
			for _, l := range listeners {
				if l.options.signal == nil || l.options.signal != signal {
					filtered = append(filtered, l)
				} else {
					// Mark as removed so ongoing iterations skip it
					l.removed = true
				}
			}
			target.listeners[eventType] = filtered
		}
		target.mu.Unlock()
	}
}

// ClearTargets clears all event target registrations.
func (eb *EventBinder) ClearTargets() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.targetMap = make(map[*goja.Object]*EventTarget)
}

// createDOMException creates a DOMException using the global constructor.
// This ensures the exception has the proper prototype chain for assert_throws_dom.
func (eb *EventBinder) createDOMException(name, message string) goja.Value {
	vm := eb.runtime.vm

	// Try to use the global DOMException constructor
	domExcCtor := vm.Get("DOMException")
	if domExcCtor != nil && !goja.IsUndefined(domExcCtor) {
		ctor, ok := goja.AssertConstructor(domExcCtor)
		if ok {
			exc, err := ctor(nil, vm.ToValue(message), vm.ToValue(name))
			if err == nil {
				return exc
			}
		}
	}

	// Fallback: create a plain object with name and message
	exc := vm.NewObject()
	exc.Set("name", name)
	exc.Set("message", message)

	// Set the appropriate error code
	errorCodes := map[string]int{
		"IndexSizeError":             1,
		"DOMStringSizeError":         2,
		"HierarchyRequestError":      3,
		"WrongDocumentError":         4,
		"InvalidCharacterError":      5,
		"NoDataAllowedError":         6,
		"NoModificationAllowedError": 7,
		"NotFoundError":              8,
		"NotSupportedError":          9,
		"InUseAttributeError":        10,
		"InvalidStateError":          11,
		"SyntaxError":                12,
		"InvalidModificationError":   13,
		"NamespaceError":             14,
		"InvalidAccessError":         15,
		"ValidationError":            16,
		"TypeMismatchError":          17,
		"SecurityError":              18,
		"NetworkError":               19,
		"AbortError":                 20,
		"URLMismatchError":           21,
		"QuotaExceededError":         22,
		"TimeoutError":               23,
		"InvalidNodeTypeError":       24,
		"DataCloneError":             25,
	}
	if code, ok := errorCodes[name]; ok {
		exc.Set("code", code)
	} else {
		exc.Set("code", 0)
	}

	return exc
}
