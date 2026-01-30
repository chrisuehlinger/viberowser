// Package js provides JavaScript execution capabilities for the browser.
// It uses the goja JavaScript engine (pure Go ES5.1+ implementation).
package js

import (
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// Runtime wraps a goja JavaScript runtime with browser-specific functionality.
type Runtime struct {
	vm         *goja.Runtime
	document   *goja.Object
	window     *goja.Object
	console    *goja.Object
	timers     *timerManager
	eventLoop  *eventLoop
	mu         sync.Mutex
	errors     []error
	onError    func(error)
}

// NewRuntime creates a new JavaScript runtime.
func NewRuntime() *Runtime {
	vm := goja.New()

	r := &Runtime{
		vm:        vm,
		timers:    newTimerManager(),
		eventLoop: newEventLoop(),
		errors:    make([]error, 0),
	}

	// Set up global objects
	r.setupConsole()
	r.setupTimers()
	r.setupWindow()

	return r
}

// VM returns the underlying goja runtime.
func (r *Runtime) VM() *goja.Runtime {
	return r.vm
}

// SetDocument sets the document object for this runtime.
func (r *Runtime) SetDocument(doc *goja.Object) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.document = doc
	r.vm.Set("document", doc)
}

// SetOnError sets a callback for JavaScript errors.
func (r *Runtime) SetOnError(handler func(error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onError = handler
}

// Execute runs JavaScript code and returns the result.
func (r *Runtime) Execute(code string) (goja.Value, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.vm.RunString(code)
	if err != nil {
		r.errors = append(r.errors, err)
		if r.onError != nil {
			r.onError(err)
		}
	}
	return result, err
}

// ExecuteScript runs JavaScript code from a script element.
// It handles errors gracefully and doesn't stop execution of subsequent scripts.
func (r *Runtime) ExecuteScript(code, src string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	program, err := goja.Compile(src, code, true)
	if err != nil {
		r.errors = append(r.errors, err)
		if r.onError != nil {
			r.onError(err)
		}
		return err
	}

	_, err = r.vm.RunProgram(program)
	if err != nil {
		r.errors = append(r.errors, err)
		if r.onError != nil {
			r.onError(err)
		}
	}
	return err
}

// Errors returns all errors that occurred during execution.
func (r *Runtime) Errors() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]error{}, r.errors...)
}

// ClearErrors clears the error list.
func (r *Runtime) ClearErrors() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = r.errors[:0]
}

// RunEventLoop processes pending timers and callbacks.
// Returns true if there are more events to process.
func (r *Runtime) RunEventLoop() bool {
	return r.eventLoop.runOnce(r)
}

// ProcessTimers checks and executes any due timers.
func (r *Runtime) ProcessTimers() {
	r.timers.process(r)
}

// HasPendingWork returns true if there are timers or callbacks waiting.
func (r *Runtime) HasPendingWork() bool {
	return r.timers.hasPending() || r.eventLoop.hasPending()
}

// setupConsole creates the console object with log, warn, error, etc.
func (r *Runtime) setupConsole() {
	console := r.vm.NewObject()

	// console.log
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println(args)
		return goja.Undefined()
	})

	// console.warn
	console.Set("warn", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println("[WARN]", args)
		return goja.Undefined()
	})

	// console.error
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println("[ERROR]", args)
		return goja.Undefined()
	})

	// console.info
	console.Set("info", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println("[INFO]", args)
		return goja.Undefined()
	})

	// console.debug
	console.Set("debug", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println("[DEBUG]", args)
		return goja.Undefined()
	})

	// console.trace
	console.Set("trace", func(call goja.FunctionCall) goja.Value {
		args := formatArgs(call.Arguments)
		fmt.Println("[TRACE]", args)
		return goja.Undefined()
	})

	// console.assert
	console.Set("assert", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || !call.Arguments[0].ToBoolean() {
			args := "Assertion failed"
			if len(call.Arguments) > 1 {
				args = formatArgs(call.Arguments[1:])
			}
			fmt.Println("[ASSERT]", args)
		}
		return goja.Undefined()
	})

	// console.clear (no-op in non-interactive context)
	console.Set("clear", func(call goja.FunctionCall) goja.Value {
		return goja.Undefined()
	})

	// console.count
	counts := make(map[string]int)
	console.Set("count", func(call goja.FunctionCall) goja.Value {
		label := "default"
		if len(call.Arguments) > 0 {
			label = call.Arguments[0].String()
		}
		counts[label]++
		fmt.Printf("%s: %d\n", label, counts[label])
		return goja.Undefined()
	})

	// console.countReset
	console.Set("countReset", func(call goja.FunctionCall) goja.Value {
		label := "default"
		if len(call.Arguments) > 0 {
			label = call.Arguments[0].String()
		}
		delete(counts, label)
		return goja.Undefined()
	})

	// console.time / console.timeEnd
	times := make(map[string]time.Time)
	console.Set("time", func(call goja.FunctionCall) goja.Value {
		label := "default"
		if len(call.Arguments) > 0 {
			label = call.Arguments[0].String()
		}
		times[label] = time.Now()
		return goja.Undefined()
	})

	console.Set("timeEnd", func(call goja.FunctionCall) goja.Value {
		label := "default"
		if len(call.Arguments) > 0 {
			label = call.Arguments[0].String()
		}
		if start, ok := times[label]; ok {
			fmt.Printf("%s: %v\n", label, time.Since(start))
			delete(times, label)
		}
		return goja.Undefined()
	})

	console.Set("timeLog", func(call goja.FunctionCall) goja.Value {
		label := "default"
		if len(call.Arguments) > 0 {
			label = call.Arguments[0].String()
		}
		if start, ok := times[label]; ok {
			fmt.Printf("%s: %v\n", label, time.Since(start))
		}
		return goja.Undefined()
	})

	r.console = console
	r.vm.Set("console", console)
}

// setupTimers creates setTimeout, setInterval, clearTimeout, clearInterval.
func (r *Runtime) setupTimers() {
	// setTimeout
	r.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}

		delay := int64(0)
		if len(call.Arguments) > 1 {
			delay = call.Arguments[1].ToInteger()
		}
		if delay < 0 {
			delay = 0
		}

		// Get additional arguments to pass to callback
		var args []goja.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		id := r.timers.setTimeout(callback, time.Duration(delay)*time.Millisecond, args)
		return r.vm.ToValue(id)
	})

	// setInterval
	r.vm.Set("setInterval", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}

		delay := int64(0)
		if len(call.Arguments) > 1 {
			delay = call.Arguments[1].ToInteger()
		}
		if delay < 0 {
			delay = 0
		}
		// Minimum interval of 4ms per HTML spec
		if delay < 4 {
			delay = 4
		}

		// Get additional arguments to pass to callback
		var args []goja.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		id := r.timers.setInterval(callback, time.Duration(delay)*time.Millisecond, args)
		return r.vm.ToValue(id)
	})

	// clearTimeout
	r.vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		id := int(call.Arguments[0].ToInteger())
		r.timers.clearTimer(id)
		return goja.Undefined()
	})

	// clearInterval
	r.vm.Set("clearInterval", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		id := int(call.Arguments[0].ToInteger())
		r.timers.clearTimer(id)
		return goja.Undefined()
	})

	// requestAnimationFrame (simplified - uses 16ms timeout to approximate 60fps)
	r.vm.Set("requestAnimationFrame", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}

		// Pass timestamp as argument
		timestamp := float64(time.Now().UnixNano()) / 1e6
		args := []goja.Value{r.vm.ToValue(timestamp)}

		id := r.timers.setTimeout(callback, 16*time.Millisecond, args)
		return r.vm.ToValue(id)
	})

	// cancelAnimationFrame
	r.vm.Set("cancelAnimationFrame", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		id := int(call.Arguments[0].ToInteger())
		r.timers.clearTimer(id)
		return goja.Undefined()
	})
}

// setupWindow creates a basic window object.
func (r *Runtime) setupWindow() {
	window := r.vm.NewObject()

	// Make window a reference to the global object
	r.vm.Set("window", window)
	r.vm.Set("self", window)
	r.vm.Set("globalThis", window)

	// window.location (basic stub)
	location := r.vm.NewObject()
	location.Set("href", "about:blank")
	location.Set("protocol", "about:")
	location.Set("host", "")
	location.Set("hostname", "")
	location.Set("port", "")
	location.Set("pathname", "blank")
	location.Set("search", "")
	location.Set("hash", "")
	location.Set("origin", "null")
	window.Set("location", location)
	r.vm.Set("location", location)

	// window.navigator (basic stub)
	navigator := r.vm.NewObject()
	navigator.Set("userAgent", "Viberowser/1.0")
	navigator.Set("language", "en-US")
	navigator.Set("languages", []string{"en-US", "en"})
	navigator.Set("platform", "Viberowser")
	navigator.Set("onLine", true)
	navigator.Set("cookieEnabled", false)
	window.Set("navigator", navigator)
	r.vm.Set("navigator", navigator)

	// window.innerWidth/innerHeight (stubs)
	window.Set("innerWidth", 1024)
	window.Set("innerHeight", 768)
	window.Set("outerWidth", 1024)
	window.Set("outerHeight", 768)

	// window.devicePixelRatio
	window.Set("devicePixelRatio", 1.0)

	// window.alert, window.confirm, window.prompt (stubs)
	window.Set("alert", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			fmt.Println("[ALERT]", call.Arguments[0].String())
		}
		return goja.Undefined()
	})

	window.Set("confirm", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			fmt.Println("[CONFIRM]", call.Arguments[0].String())
		}
		return r.vm.ToValue(true)
	})

	window.Set("prompt", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			fmt.Println("[PROMPT]", call.Arguments[0].String())
		}
		return goja.Null()
	})

	// window.getComputedStyle (stub)
	window.Set("getComputedStyle", func(call goja.FunctionCall) goja.Value {
		// Return an empty style object
		style := r.vm.NewObject()
		style.Set("getPropertyValue", func(call goja.FunctionCall) goja.Value {
			return r.vm.ToValue("")
		})
		return style
	})

	// window.matchMedia (stub)
	window.Set("matchMedia", func(call goja.FunctionCall) goja.Value {
		result := r.vm.NewObject()
		result.Set("matches", false)
		result.Set("media", "")
		result.Set("addListener", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
		result.Set("removeListener", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
		result.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
		result.Set("removeEventListener", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
		return result
	})

	// window.performance (basic)
	performance := r.vm.NewObject()
	startTime := time.Now()
	performance.Set("now", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(float64(time.Since(startTime).Nanoseconds()) / 1e6)
	})
	performance.Set("timeOrigin", float64(startTime.UnixNano())/1e6)
	window.Set("performance", performance)
	r.vm.Set("performance", performance)

	// queueMicrotask
	r.vm.Set("queueMicrotask", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}
		r.eventLoop.queueMicrotask(callback, nil)
		return goja.Undefined()
	})

	r.window = window
}

// formatArgs formats function call arguments for console output.
func formatArgs(args []goja.Value) string {
	if len(args) == 0 {
		return ""
	}

	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += formatValue(arg)
	}
	return result
}

// formatValue formats a single value for output.
func formatValue(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) {
		return "undefined"
	}
	if goja.IsNull(v) {
		return "null"
	}
	return v.String()
}
