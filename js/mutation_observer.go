// Package js provides JavaScript execution capabilities for the browser.
package js

import (
	"fmt"
	"sync"

	"github.com/chrisuehlinger/viberowser/dom"
	"github.com/dop251/goja"
)

// MutationRecord represents a mutation that has been observed.
type MutationRecord struct {
	Type               string      // "childList", "attributes", or "characterData"
	Target             *dom.Node   // The node that was mutated
	AddedNodes         []*dom.Node // Nodes added (childList mutations)
	RemovedNodes       []*dom.Node // Nodes removed (childList mutations)
	PreviousSibling    *dom.Node   // Previous sibling of added/removed nodes
	NextSibling        *dom.Node   // Next sibling of added/removed nodes
	AttributeName      string      // Name of changed attribute (attributes mutations)
	AttributeNamespace string      // Namespace of changed attribute
	OldValue           string      // Previous value (if oldValue option was set)
}

// MutationObserverOptions holds the options for observing mutations.
type MutationObserverOptions struct {
	ChildList                   bool     // Observe child list changes
	Attributes                  bool     // Observe attribute changes
	CharacterData               bool     // Observe character data changes
	Subtree                     bool     // Observe descendants too
	AttributeOldValue           bool     // Record old attribute values
	CharacterDataOldValue       bool     // Record old character data values
	AttributeFilter             []string // Only observe specific attributes
	attributesExplicitlySet     bool     // Was attributes explicitly set in the options?
	characterDataExplicitlySet  bool     // Was characterData explicitly set in the options?
}

// MutationObserver observes DOM mutations and calls a callback when they occur.
type MutationObserver struct {
	callback       goja.Callable
	vm             *goja.Runtime
	eventLoop      *eventLoop
	domBinder      *DOMBinder
	targets        map[*dom.Node]*MutationObserverOptions
	pendingRecords []MutationRecord
	isScheduled    bool
	jsObserver     *goja.Object // The JavaScript object representing this observer
	mu             sync.Mutex
}

// IframeLoadCallback is called when an iframe is added to the DOM.
// The callback receives the iframe element that was added.
type IframeLoadCallback func(iframe *dom.Element)

// MutationObserverManager manages all active mutation observers for a runtime.
type MutationObserverManager struct {
	observers          []*MutationObserver
	iframeLoadCallback IframeLoadCallback
	mu                 sync.RWMutex
}

// NewMutationObserverManager creates a new manager for mutation observers.
func NewMutationObserverManager() *MutationObserverManager {
	return &MutationObserverManager{
		observers: make([]*MutationObserver, 0),
	}
}

// SetIframeLoadCallback sets a callback to be called when iframes are added to the DOM.
func (m *MutationObserverManager) SetIframeLoadCallback(callback IframeLoadCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iframeLoadCallback = callback
}

// OnChildListMutation implements dom.MutationCallback interface.
func (m *MutationObserverManager) OnChildListMutation(
	target *dom.Node,
	addedNodes []*dom.Node,
	removedNodes []*dom.Node,
	previousSibling *dom.Node,
	nextSibling *dom.Node,
) {
	m.NotifyChildListMutation(target, addedNodes, removedNodes, previousSibling, nextSibling)

	// Check if any added nodes are iframes (or contain iframes) and trigger load events
	m.mu.RLock()
	callback := m.iframeLoadCallback
	m.mu.RUnlock()

	if callback != nil {
		for _, node := range addedNodes {
			m.findIframesAndNotify(node, callback)
		}
	}
}

// findIframesAndNotify recursively finds iframe elements in a node tree and notifies the callback.
func (m *MutationObserverManager) findIframesAndNotify(node *dom.Node, callback IframeLoadCallback) {
	if node == nil {
		return
	}

	// Check if this node is an iframe element
	if node.NodeType() == dom.ElementNode {
		el := (*dom.Element)(node)
		if el != nil {
			tagName := el.TagName()
			if tagName == "IFRAME" || tagName == "iframe" {
				// Only fire load event if the iframe is connected to the document
				if node.IsConnected() {
					callback(el)
				}
			}
		}
	}

	// Recursively check child nodes
	childNodes := node.ChildNodes()
	for i := 0; i < childNodes.Length(); i++ {
		m.findIframesAndNotify(childNodes.Item(i), callback)
	}
}

// OnAttributeMutation implements dom.MutationCallback interface.
func (m *MutationObserverManager) OnAttributeMutation(
	target *dom.Node,
	attributeName string,
	attributeNamespace string,
	oldValue string,
) {
	m.NotifyAttributeMutation(target, attributeName, attributeNamespace, oldValue)
}

// OnCharacterDataMutation implements dom.MutationCallback interface.
func (m *MutationObserverManager) OnCharacterDataMutation(
	target *dom.Node,
	oldValue string,
) {
	m.NotifyCharacterDataMutation(target, oldValue)
}

// OnReplaceData handles the "replace data" algorithm notification.
// This is used for Range boundary point adjustments, NOT for MutationObserver.
// MutationObserver notifications are handled separately via OnCharacterDataMutation
// which receives the correct oldValue.
func (m *MutationObserverManager) OnReplaceData(
	target *dom.Node,
	offset int,
	count int,
	data string,
) {
	// No-op for MutationObserver purposes.
	// The OnCharacterDataMutation callback (called separately by the DOM layer)
	// handles MutationObserver notifications with the correct oldValue.
	// OnReplaceData is only used by RangeMutationHandler for boundary updates.
}

// OnSplitText handles the splitText algorithm notification.
// This is used for Range boundary point adjustments, NOT for MutationObserver.
// MutationObserver handles splitText through the childList mutation (insertion)
// and characterData mutation (truncation).
func (m *MutationObserverManager) OnSplitText(
	oldNode *dom.Node,
	splitOffset int,
	newNode *dom.Node,
) {
	// No-op for MutationObserver purposes.
	// OnSplitText is only used by RangeMutationHandler for boundary updates.
}

// Register adds an observer to the manager.
func (m *MutationObserverManager) Register(observer *MutationObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, observer)
}

// Unregister removes an observer from the manager.
func (m *MutationObserverManager) Unregister(observer *MutationObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, obs := range m.observers {
		if obs == observer {
			m.observers = append(m.observers[:i], m.observers[i+1:]...)
			return
		}
	}
}

// NotifyChildListMutation notifies all observers about a childList mutation.
func (m *MutationObserverManager) NotifyChildListMutation(
	target *dom.Node,
	addedNodes []*dom.Node,
	removedNodes []*dom.Node,
	previousSibling *dom.Node,
	nextSibling *dom.Node,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record := MutationRecord{
		Type:            "childList",
		Target:          target,
		AddedNodes:      addedNodes,
		RemovedNodes:    removedNodes,
		PreviousSibling: previousSibling,
		NextSibling:     nextSibling,
	}

	for _, observer := range m.observers {
		if observer.shouldObserve(target, "childList") {
			observer.queueRecord(record)
		}
	}
}

// NotifyAttributeMutation notifies all observers about an attribute mutation.
func (m *MutationObserverManager) NotifyAttributeMutation(
	target *dom.Node,
	attributeName string,
	attributeNamespace string,
	oldValue string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, observer := range m.observers {
		if observer.shouldObserve(target, "attributes") {
			opts := observer.getOptions(target)
			if opts == nil {
				continue
			}

			// Check attribute filter
			if len(opts.AttributeFilter) > 0 {
				found := false
				for _, name := range opts.AttributeFilter {
					if name == attributeName {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Create a record for this observer with appropriate oldValue
			record := MutationRecord{
				Type:               "attributes",
				Target:             target,
				AttributeName:      attributeName,
				AttributeNamespace: attributeNamespace,
			}

			// Only include old value if requested
			if opts.AttributeOldValue {
				record.OldValue = oldValue
			}

			observer.queueRecord(record)
		}
	}
}

// NotifyCharacterDataMutation notifies all observers about a characterData mutation.
func (m *MutationObserverManager) NotifyCharacterDataMutation(
	target *dom.Node,
	oldValue string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, observer := range m.observers {
		if observer.shouldObserve(target, "characterData") {
			opts := observer.getOptions(target)
			if opts == nil {
				continue
			}

			// Create a record for this observer with appropriate oldValue
			record := MutationRecord{
				Type:   "characterData",
				Target: target,
			}

			// Only include old value if requested
			if opts.CharacterDataOldValue {
				record.OldValue = oldValue
			}

			observer.queueRecord(record)
		}
	}
}

// getOptions returns the options for observing the given target or its ancestors.
func (mo *MutationObserver) getOptions(target *dom.Node) *MutationObserverOptions {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	// Check exact target first
	if opts, ok := mo.targets[target]; ok {
		return opts
	}

	// Check ancestors if subtree is enabled
	for node := target.ParentNode(); node != nil; node = node.ParentNode() {
		if opts, ok := mo.targets[node]; ok && opts.Subtree {
			return opts
		}
	}

	return nil
}

// shouldObserve returns true if this observer should receive notifications for the given target and mutation type.
func (mo *MutationObserver) shouldObserve(target *dom.Node, mutationType string) bool {
	opts := mo.getOptions(target)
	if opts == nil {
		return false
	}

	switch mutationType {
	case "childList":
		return opts.ChildList
	case "attributes":
		return opts.Attributes
	case "characterData":
		return opts.CharacterData
	default:
		return false
	}
}

// queueRecord adds a mutation record to the pending queue.
func (mo *MutationObserver) queueRecord(record MutationRecord) {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	mo.pendingRecords = append(mo.pendingRecords, record)

	// Schedule callback if not already scheduled
	if !mo.isScheduled {
		mo.isScheduled = true
		// Use Promise.resolve().then() to schedule the callback as a microtask.
		// This integrates with goja's internal Promise job queue so that
		// mutation callbacks run when await Promise.resolve() is called.
		deliverFunc := mo.vm.ToValue(func(call goja.FunctionCall) goja.Value {
			mo.deliverRecords()
			return goja.Undefined()
		})

		// Get Promise constructor and call Promise.resolve().then(deliverFunc)
		promiseVal := mo.vm.Get("Promise")
		if promiseVal != nil && !goja.IsUndefined(promiseVal) {
			promiseObj := promiseVal.ToObject(mo.vm)
			resolveMethod, _ := goja.AssertFunction(promiseObj.Get("resolve"))
			if resolveMethod != nil {
				resolved, _ := resolveMethod(promiseObj)
				if resolved != nil && !goja.IsUndefined(resolved) {
					resolvedObj := resolved.ToObject(mo.vm)
					thenMethod, _ := goja.AssertFunction(resolvedObj.Get("then"))
					if thenMethod != nil {
						thenMethod(resolvedObj, deliverFunc)
					}
				}
			}
		}
	}
}

// deliverRecords delivers pending mutation records to the callback.
func (mo *MutationObserver) deliverRecords() (goja.Value, error) {
	mo.mu.Lock()
	records := mo.pendingRecords
	mo.pendingRecords = nil
	mo.isScheduled = false
	mo.mu.Unlock()

	if len(records) == 0 {
		return goja.Undefined(), nil
	}

	// Convert records to JavaScript array
	jsRecords := mo.vm.NewArray()
	for i, record := range records {
		jsRecord := mo.createJSRecord(record)
		// Use integer index for proper array behavior
		jsRecords.Set(fmt.Sprintf("%d", i), mo.vm.ToValue(jsRecord))
	}
	// Ensure length is set for Array.isArray() to work
	jsRecords.Set("length", len(records))

	// Call the callback with `this` set to the observer and passing (records, observer) as args
	_, err := mo.callback(mo.jsObserver, jsRecords, mo.jsObserver)
	return goja.Undefined(), err
}

// createJSRecord creates a JavaScript MutationRecord object.
func (mo *MutationObserver) createJSRecord(record MutationRecord) *goja.Object {
	jsRecord := mo.vm.NewObject()

	// Set prototype for instanceof MutationRecord to work
	if mutationRecordProto != nil {
		jsRecord.SetPrototype(mutationRecordProto)
	}

	jsRecord.Set("type", record.Type)

	// Target node
	if record.Target != nil && mo.domBinder != nil {
		jsRecord.Set("target", mo.domBinder.BindNode(record.Target))
	} else {
		jsRecord.Set("target", goja.Null())
	}

	// AddedNodes as NodeList-like array
	addedNodes := mo.vm.NewArray()
	for i, node := range record.AddedNodes {
		if mo.domBinder != nil {
			addedNodes.Set(fmt.Sprintf("%d", i), mo.domBinder.BindNode(node))
		}
	}
	addedNodes.Set("length", len(record.AddedNodes))
	jsRecord.Set("addedNodes", addedNodes)

	// RemovedNodes as NodeList-like array
	removedNodes := mo.vm.NewArray()
	for i, node := range record.RemovedNodes {
		if mo.domBinder != nil {
			removedNodes.Set(fmt.Sprintf("%d", i), mo.domBinder.BindNode(node))
		}
	}
	removedNodes.Set("length", len(record.RemovedNodes))
	jsRecord.Set("removedNodes", removedNodes)

	// Previous and next siblings
	if record.PreviousSibling != nil && mo.domBinder != nil {
		jsRecord.Set("previousSibling", mo.domBinder.BindNode(record.PreviousSibling))
	} else {
		jsRecord.Set("previousSibling", goja.Null())
	}

	if record.NextSibling != nil && mo.domBinder != nil {
		jsRecord.Set("nextSibling", mo.domBinder.BindNode(record.NextSibling))
	} else {
		jsRecord.Set("nextSibling", goja.Null())
	}

	// Attribute info
	if record.AttributeName != "" {
		jsRecord.Set("attributeName", record.AttributeName)
	} else {
		jsRecord.Set("attributeName", goja.Null())
	}

	if record.AttributeNamespace != "" {
		jsRecord.Set("attributeNamespace", record.AttributeNamespace)
	} else {
		jsRecord.Set("attributeNamespace", goja.Null())
	}

	// Old value
	if record.OldValue != "" {
		jsRecord.Set("oldValue", record.OldValue)
	} else {
		jsRecord.Set("oldValue", goja.Null())
	}

	return jsRecord
}

// observe starts observing a target node with the given options.
func (mo *MutationObserver) observe(target *dom.Node, options *MutationObserverOptions) error {
	if target == nil {
		return fmt.Errorf("target is null")
	}

	// Validate and apply implicit settings per spec
	// https://dom.spec.whatwg.org/#dom-mutationobserver-observe

	// If attributeOldValue is true and attributes is explicitly false, throw
	if options.AttributeOldValue && options.attributesExplicitlySet && !options.Attributes {
		return fmt.Errorf("The options object may not set 'attributeOldValue' to true when 'attributes' is false")
	}

	// If attributeFilter is present and attributes is explicitly false, throw
	if len(options.AttributeFilter) > 0 && options.attributesExplicitlySet && !options.Attributes {
		return fmt.Errorf("The options object may not set 'attributeFilter' when 'attributes' is false")
	}

	// If characterDataOldValue is true and characterData is explicitly false, throw
	if options.CharacterDataOldValue && options.characterDataExplicitlySet && !options.CharacterData {
		return fmt.Errorf("The options object may not set 'characterDataOldValue' to true when 'characterData' is false")
	}

	// If attributeOldValue is true but attributes is not set, implicitly enable attributes
	if options.AttributeOldValue && !options.attributesExplicitlySet {
		options.Attributes = true
	}

	// If characterDataOldValue is true but characterData is not set, implicitly enable characterData
	if options.CharacterDataOldValue && !options.characterDataExplicitlySet {
		options.CharacterData = true
	}

	// If attributeFilter is present but attributes is not set, implicitly enable attributes
	if len(options.AttributeFilter) > 0 && !options.attributesExplicitlySet {
		options.Attributes = true
	}

	// Now check if at least one observation type is enabled
	if !options.ChildList && !options.Attributes && !options.CharacterData {
		return fmt.Errorf("The options object must set at least one of 'attributes', 'characterData', or 'childList' to true")
	}

	mo.mu.Lock()
	mo.targets[target] = options
	mo.mu.Unlock()

	return nil
}

// disconnect stops all observation.
func (mo *MutationObserver) disconnect() {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	mo.targets = make(map[*dom.Node]*MutationObserverOptions)
	mo.pendingRecords = nil
	mo.isScheduled = false
}

// takeRecords returns and clears the pending mutation records.
func (mo *MutationObserver) takeRecords() []MutationRecord {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	records := mo.pendingRecords
	mo.pendingRecords = nil
	return records
}

// bindObserverMethods adds the MutationObserver methods to a JavaScript object.
func (mo *MutationObserver) bindObserverMethods(jsObserver *goja.Object) {
	vm := mo.vm

	// observe(target, options)
	jsObserver.Set("observe", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'observe' on 'MutationObserver': 2 arguments required"))
		}

		// Get target node
		targetArg := call.Arguments[0]
		if goja.IsNull(targetArg) || goja.IsUndefined(targetArg) {
			panic(vm.NewTypeError("Failed to execute 'observe' on 'MutationObserver': parameter 1 is not of type 'Node'"))
		}

		targetObj := targetArg.ToObject(vm)
		target := mo.domBinder.getGoNode(targetObj)
		if target == nil {
			panic(vm.NewTypeError("Failed to execute 'observe' on 'MutationObserver': parameter 1 is not of type 'Node'"))
		}

		// Parse options
		options := &MutationObserverOptions{}
		optionsArg := call.Arguments[1]
		if !goja.IsNull(optionsArg) && !goja.IsUndefined(optionsArg) {
			optionsObj := optionsArg.ToObject(vm)
			if optionsObj != nil {
				if v := optionsObj.Get("childList"); v != nil && !goja.IsUndefined(v) {
					options.ChildList = v.ToBoolean()
				}
				if v := optionsObj.Get("attributes"); v != nil && !goja.IsUndefined(v) {
					options.Attributes = v.ToBoolean()
					options.attributesExplicitlySet = true
				}
				if v := optionsObj.Get("characterData"); v != nil && !goja.IsUndefined(v) {
					options.CharacterData = v.ToBoolean()
					options.characterDataExplicitlySet = true
				}
				if v := optionsObj.Get("subtree"); v != nil && !goja.IsUndefined(v) {
					options.Subtree = v.ToBoolean()
				}
				if v := optionsObj.Get("attributeOldValue"); v != nil && !goja.IsUndefined(v) {
					options.AttributeOldValue = v.ToBoolean()
				}
				if v := optionsObj.Get("characterDataOldValue"); v != nil && !goja.IsUndefined(v) {
					options.CharacterDataOldValue = v.ToBoolean()
				}
				if v := optionsObj.Get("attributeFilter"); v != nil && !goja.IsUndefined(v) {
					filterObj := v.ToObject(vm)
					if filterObj != nil {
						length := filterObj.Get("length")
						if length != nil && !goja.IsUndefined(length) {
							n := int(length.ToInteger())
							options.AttributeFilter = make([]string, n)
							for i := 0; i < n; i++ {
								item := filterObj.Get(fmt.Sprintf("%d", i))
								if item != nil && !goja.IsUndefined(item) {
									options.AttributeFilter[i] = item.String()
								}
							}
						}
					}
				}
			}
		}

		err := mo.observe(target, options)
		if err != nil {
			panic(vm.NewTypeError(err.Error()))
		}

		return goja.Undefined()
	})

	// disconnect()
	jsObserver.Set("disconnect", func(call goja.FunctionCall) goja.Value {
		mo.disconnect()
		return goja.Undefined()
	})

	// takeRecords()
	jsObserver.Set("takeRecords", func(call goja.FunctionCall) goja.Value {
		records := mo.takeRecords()
		jsRecords := vm.NewArray()
		for i, record := range records {
			jsRecord := mo.createJSRecord(record)
			jsRecords.Set(fmt.Sprintf("%d", i), jsRecord)
		}
		return jsRecords
	})
}

// mutationRecordProto holds the MutationRecord prototype for instanceof checks
var mutationRecordProto *goja.Object

// SetupMutationObserver sets up the MutationObserver constructor on the runtime.
func SetupMutationObserver(runtime *Runtime, domBinder *DOMBinder, manager *MutationObserverManager) {
	vm := runtime.vm

	// Create MutationRecord prototype and constructor for instanceof to work
	mutationRecordProto = vm.NewObject()
	mutationRecordConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	mutationRecordConstructorObj := mutationRecordConstructor.ToObject(vm)
	mutationRecordConstructorObj.Set("prototype", mutationRecordProto)
	mutationRecordProto.Set("constructor", mutationRecordConstructorObj)
	vm.Set("MutationRecord", mutationRecordConstructorObj)

	// Create MutationObserver constructor
	vm.Set("MutationObserver", func(call goja.ConstructorCall) *goja.Object {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to construct 'MutationObserver': 1 argument required"))
		}

		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			panic(vm.NewTypeError("Failed to construct 'MutationObserver': parameter 1 is not a function"))
		}

		// Get the JS object that will represent this observer
		jsObserver := call.This

		// Create the observer
		observer := &MutationObserver{
			callback:       callback,
			vm:             vm,
			eventLoop:      runtime.eventLoop,
			domBinder:      domBinder,
			targets:        make(map[*dom.Node]*MutationObserverOptions),
			pendingRecords: make([]MutationRecord, 0),
			jsObserver:     jsObserver, // Store reference for callback invocation
		}

		// Register with manager
		if manager != nil {
			manager.Register(observer)
		}

		// Bind methods to the JS object
		observer.bindObserverMethods(jsObserver)

		return jsObserver
	})
}

