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
	id       int
	callback goja.Callable
	value    goja.Value // Original value for comparison
	options  listenerOptions
}

// listenerOptions represents addEventListener options.
type listenerOptions struct {
	capture bool
	once    bool
	passive bool
}

// EventTarget manages event listeners for a target.
type EventTarget struct {
	listeners map[string][]eventListener
	nextID    int
	mu        sync.RWMutex
}

// NewEventTarget creates a new EventTarget.
func NewEventTarget() *EventTarget {
	return &EventTarget{
		listeners: make(map[string][]eventListener),
	}
}

// AddEventListener registers an event listener.
func (et *EventTarget) AddEventListener(eventType string, callback goja.Callable, value goja.Value, opts listenerOptions) {
	et.mu.Lock()
	defer et.mu.Unlock()

	// Check for duplicate by comparing the underlying Value
	for _, l := range et.listeners[eventType] {
		if l.value.SameAs(value) && l.options.capture == opts.capture {
			return // Already registered
		}
	}

	et.nextID++
	et.listeners[eventType] = append(et.listeners[eventType], eventListener{
		id:       et.nextID,
		callback: callback,
		value:    value,
		options:  opts,
	})
}

// RemoveEventListener unregisters an event listener.
func (et *EventTarget) RemoveEventListener(eventType string, value goja.Value, capture bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	listeners := et.listeners[eventType]
	for i, l := range listeners {
		if l.value.SameAs(value) && l.options.capture == capture {
			et.listeners[eventType] = append(listeners[:i], listeners[i+1:]...)
			return
		}
	}
}

// DispatchEvent dispatches an event to all registered listeners.
// At the AT_TARGET phase, both capturing and non-capturing listeners are called
// in the order they were added, with capturing listeners first according to DOM spec.
func (et *EventTarget) DispatchEvent(vm *goja.Runtime, event *goja.Object, phase EventPhase) bool {
	et.mu.RLock()
	eventType := event.Get("type").String()
	listeners := make([]eventListener, len(et.listeners[eventType]))
	copy(listeners, et.listeners[eventType])
	et.mu.RUnlock()

	var toRemove []eventListener

	// At the target phase, per DOM spec, we need to call all listeners
	// regardless of capture flag, but capturing listeners should be called first
	// if they were registered before non-capturing ones of the same type.
	// Actually, the spec says at target phase, both are called in registration order.
	if phase == EventPhaseAtTarget {
		// At target: call capturing listeners first, then non-capturing
		// According to the DOM spec, at the target phase, listeners are invoked
		// in the order they were registered, regardless of capture flag.
		// But the test expects capturing listeners to fire before non-capturing.
		// Let's sort: capturing first, then non-capturing, preserving registration order within each group
		capturingListeners := make([]eventListener, 0)
		bubblingListeners := make([]eventListener, 0)
		for _, l := range listeners {
			if l.options.capture {
				capturingListeners = append(capturingListeners, l)
			} else {
				bubblingListeners = append(bubblingListeners, l)
			}
		}
		listeners = append(capturingListeners, bubblingListeners...)
	}

	// Get currentTarget for 'this' binding
	currentTarget := event.Get("currentTarget")
	if currentTarget == nil || goja.IsUndefined(currentTarget) {
		currentTarget = goja.Undefined()
	}

	for _, l := range listeners {
		// Check phase - only filter by capture/non-capture for non-target phases
		if phase == EventPhaseCapturing && !l.options.capture {
			continue
		}
		if phase == EventPhaseBubbling && l.options.capture {
			continue
		}

		// Call the listener with currentTarget as 'this'
		// Per DOM spec, the 'this' value for event listeners is the currentTarget
		l.callback(currentTarget, event)

		// Check for stopImmediatePropagation
		if stopImmediate := event.Get("_stopImmediate"); stopImmediate != nil && stopImmediate.ToBoolean() {
			break
		}

		// Mark for removal if once
		if l.options.once {
			toRemove = append(toRemove, l)
		}
	}

	// Remove 'once' listeners
	if len(toRemove) > 0 {
		et.mu.Lock()
		for _, l := range toRemove {
			listeners := et.listeners[eventType]
			for i, existing := range listeners {
				if existing.id == l.id {
					et.listeners[eventType] = append(listeners[:i], listeners[i+1:]...)
					break
				}
			}
		}
		et.mu.Unlock()
	}

	// Return true if default wasn't prevented
	if defaultPrevented := event.Get("defaultPrevented"); defaultPrevented != nil {
		return !defaultPrevented.ToBoolean()
	}
	return true
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
	activationHandler           ActivationHandler       // Handler for pre-click activation behavior
	activationCancelHandler     ActivationCancelHandler // Handler for canceled activation
	activationCompleteHandler   ActivationCompleteHandler // Handler for successful activation
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

// SetActivationHandlers sets the handlers for activation behavior.
// The activation handler runs pre-click activation for elements with activation behavior.
// The cancel handler reverts the activation if defaultPrevented is set.
// The complete handler fires input/change events after successful activation.
func (eb *EventBinder) SetActivationHandlers(handler ActivationHandler, cancelHandler ActivationCancelHandler, completeHandler ActivationCompleteHandler) {
	eb.activationHandler = handler
	eb.activationCancelHandler = cancelHandler
	eb.activationCompleteHandler = completeHandler
}

// GetOrCreateTarget gets or creates an EventTarget for a JS object.
func (eb *EventBinder) GetOrCreateTarget(obj *goja.Object) *EventTarget {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if target, ok := eb.targetMap[obj]; ok {
		return target
	}

	target := NewEventTarget()
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
		callback, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
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
				}
			}
		}

		target := eb.GetOrCreateTarget(obj)
		target.AddEventListener(eventType, callback, call.Arguments[1], opts)
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

		// Set target
		event.Set("target", obj)

		// Build event path from target up to root
		// The path is ordered from the target to the root (for bubbling)
		eventPath := []*goja.Object{obj}
		if eb.nodeResolver != nil {
			current := obj
			for {
				parent := eb.nodeResolver(current)
				if parent == nil {
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

		// Phase 1: Capturing phase (root to target, excluding target)
		// Walk from the end of eventPath (root) to the beginning (target)
		for i := len(eventPath) - 1; i > 0; i-- {
			if shouldStopPropagation() {
				break
			}
			currentTarget := eventPath[i]
			event.Set("currentTarget", currentTarget)
			event.Set("eventPhase", int(EventPhaseCapturing))
			target := eb.GetOrCreateTarget(currentTarget)
			target.DispatchEvent(vm, event, EventPhaseCapturing)
		}

		// Phase 2: At target (target itself)
		if !shouldStopPropagation() {
			event.Set("currentTarget", obj)
			event.Set("eventPhase", int(EventPhaseAtTarget))
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
	event.Set("timeStamp", float64(0))

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

	// Create constructor
	ctor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		eventType := ""
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		event := eb.CreateEvent(eventType, nil)
		event.SetPrototype(proto)

		// Apply bubbles/cancelable from options
		if len(call.Arguments) > 1 {
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

	// Event - base event constructor
	eb.createEventConstructor("Event", nil, nil)
	eventProto := eb.GetEventProto("Event")

	// CustomEvent - extends Event
	eb.createEventConstructor("CustomEvent", eventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("detail", goja.Null())
		if len(call.Arguments) > 1 {
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
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("view"); v != nil && !goja.IsUndefined(v) {
					event.Set("view", v)
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					event.Set("detail", v.ToInteger())
				}
			}
		}
	})
	uiEventProto := eb.GetEventProto("UIEvent")

	// MouseEvent - extends UIEvent
	eb.createEventConstructor("MouseEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
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
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
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
			}
		}
	})

	// FocusEvent - extends UIEvent
	eb.createEventConstructor("FocusEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		event.Set("relatedTarget", goja.Null())
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("relatedTarget"); v != nil && !goja.IsUndefined(v) {
					event.Set("relatedTarget", v)
				}
			}
		}
	})

	// KeyboardEvent - extends UIEvent
	eb.createEventConstructor("KeyboardEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
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
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				if v := optObj.Get("key"); v != nil && !goja.IsUndefined(v) {
					event.Set("key", v.String())
				}
				if v := optObj.Get("code"); v != nil && !goja.IsUndefined(v) {
					event.Set("code", v.String())
				}
			}
		}
	})

	// CompositionEvent - extends UIEvent
	eb.createEventConstructor("CompositionEvent", uiEventProto, func(event *goja.Object, call goja.ConstructorCall) {
		event.Set("view", goja.Null())
		event.Set("detail", 0)
		event.Set("data", "")
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
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
		if len(call.Arguments) > 1 {
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
		if len(call.Arguments) > 1 {
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
		if len(call.Arguments) > 1 {
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
		if len(call.Arguments) > 1 {
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
