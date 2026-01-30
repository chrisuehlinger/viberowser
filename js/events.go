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
func (et *EventTarget) DispatchEvent(vm *goja.Runtime, event *goja.Object, phase EventPhase) bool {
	et.mu.RLock()
	eventType := event.Get("type").String()
	listeners := make([]eventListener, len(et.listeners[eventType]))
	copy(listeners, et.listeners[eventType])
	et.mu.RUnlock()

	var toRemove []eventListener

	for _, l := range listeners {
		// Check phase
		if phase == EventPhaseCapturing && !l.options.capture {
			continue
		}
		if phase == EventPhaseBubbling && l.options.capture {
			continue
		}

		// Call the listener
		l.callback(goja.Undefined(), event)

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

// EventBinder provides methods to add event handling to JS objects.
type EventBinder struct {
	runtime     *Runtime
	targetMap   map[*goja.Object]*EventTarget
	mu          sync.RWMutex
}

// NewEventBinder creates a new event binder.
func NewEventBinder(runtime *Runtime) *EventBinder {
	return &EventBinder{
		runtime:   runtime,
		targetMap: make(map[*goja.Object]*EventTarget),
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

		target := eb.GetOrCreateTarget(obj)
		target.AddEventListener(eventType, callback, call.Arguments[1], opts)
		return goja.Undefined()
	})

	obj.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}

		eventType := call.Arguments[0].String()
		// Verify the second argument is a function
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

		target := eb.GetOrCreateTarget(obj)
		target.RemoveEventListener(eventType, call.Arguments[1], capture)
		return goja.Undefined()
	})

	obj.Set("dispatchEvent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(true)
		}

		event := call.Arguments[0].ToObject(vm)
		if event == nil {
			return vm.ToValue(true)
		}

		// Set the target
		event.Set("target", obj)
		event.Set("currentTarget", obj)
		event.Set("eventPhase", int(EventPhaseAtTarget))

		target := eb.GetOrCreateTarget(obj)
		return vm.ToValue(target.DispatchEvent(vm, event, EventPhaseAtTarget))
	})
}

// CreateEvent creates a new Event object.
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

	// Constants
	event.Set("NONE", int(EventPhaseNone))
	event.Set("CAPTURING_PHASE", int(EventPhaseCapturing))
	event.Set("AT_TARGET", int(EventPhaseAtTarget))
	event.Set("BUBBLING_PHASE", int(EventPhaseBubbling))

	return event
}

// SetupEventConstructors sets up Event and CustomEvent constructors on the global object.
func (eb *EventBinder) SetupEventConstructors() {
	vm := eb.runtime.vm

	// Event constructor
	vm.Set("Event", func(call goja.ConstructorCall) *goja.Object {
		eventType := ""
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		var options map[string]interface{}
		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				options = make(map[string]interface{})
				if v := optObj.Get("bubbles"); v != nil && !goja.IsUndefined(v) {
					options["bubbles"] = v.ToBoolean()
				}
				if v := optObj.Get("cancelable"); v != nil && !goja.IsUndefined(v) {
					options["cancelable"] = v.ToBoolean()
				}
				if v := optObj.Get("composed"); v != nil && !goja.IsUndefined(v) {
					options["composed"] = v.ToBoolean()
				}
			}
		}

		return eb.CreateEvent(eventType, options)
	})

	// CustomEvent constructor
	vm.Set("CustomEvent", func(call goja.ConstructorCall) *goja.Object {
		eventType := ""
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		var options map[string]interface{}
		var detail goja.Value = goja.Null()

		if len(call.Arguments) > 1 {
			optObj := call.Arguments[1].ToObject(vm)
			if optObj != nil {
				options = make(map[string]interface{})
				if v := optObj.Get("bubbles"); v != nil && !goja.IsUndefined(v) {
					options["bubbles"] = v.ToBoolean()
				}
				if v := optObj.Get("cancelable"); v != nil && !goja.IsUndefined(v) {
					options["cancelable"] = v.ToBoolean()
				}
				if v := optObj.Get("composed"); v != nil && !goja.IsUndefined(v) {
					options["composed"] = v.ToBoolean()
				}
				if v := optObj.Get("detail"); v != nil && !goja.IsUndefined(v) {
					detail = v
				}
			}
		}

		event := eb.CreateEvent(eventType, options)
		event.Set("detail", detail)

		return event
	})

	// Error event constructor
	vm.Set("ErrorEvent", func(call goja.ConstructorCall) *goja.Object {
		eventType := "error"
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		event := eb.CreateEvent(eventType, nil)

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

		return event
	})
}

// ClearTargets clears all event target registrations.
func (eb *EventBinder) ClearTargets() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.targetMap = make(map[*goja.Object]*EventTarget)
}
