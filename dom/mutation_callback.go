package dom

// MutationCallback is an interface for receiving notifications about DOM mutations.
// This is used to implement MutationObserver functionality.
type MutationCallback interface {
	// OnChildListMutation is called when children are added or removed.
	OnChildListMutation(
		target *Node,
		addedNodes []*Node,
		removedNodes []*Node,
		previousSibling *Node,
		nextSibling *Node,
	)

	// OnAttributeMutation is called when an attribute is changed.
	OnAttributeMutation(
		target *Node,
		attributeName string,
		attributeNamespace string,
		oldValue string,
	)

	// OnCharacterDataMutation is called when character data is changed.
	// This is typically called for full data replacement (e.g., setting nodeValue).
	OnCharacterDataMutation(
		target *Node,
		oldValue string,
	)

	// OnReplaceData is called when the "replace data" algorithm is invoked.
	// This provides the specific offset, count, and replacement data needed
	// for precise Range boundary point adjustments per the DOM spec.
	OnReplaceData(
		target *Node,
		offset int,
		count int,
		data string,
	)
}

// mutationCallbacks stores registered mutation callbacks for a document.
var mutationCallbacks = make(map[*Document][]MutationCallback)

// RegisterMutationCallback registers a callback to receive mutation notifications for a document.
func RegisterMutationCallback(doc *Document, callback MutationCallback) {
	if doc == nil || callback == nil {
		return
	}
	mutationCallbacks[doc] = append(mutationCallbacks[doc], callback)
}

// UnregisterMutationCallback removes a callback from a document.
func UnregisterMutationCallback(doc *Document, callback MutationCallback) {
	if doc == nil {
		return
	}
	callbacks := mutationCallbacks[doc]
	for i, cb := range callbacks {
		if cb == callback {
			mutationCallbacks[doc] = append(callbacks[:i], callbacks[i+1:]...)
			return
		}
	}
}

// ClearMutationCallbacks removes all callbacks for a document.
func ClearMutationCallbacks(doc *Document) {
	delete(mutationCallbacks, doc)
}

// notifyChildListMutation notifies all registered callbacks about a childList mutation.
func notifyChildListMutation(
	target *Node,
	addedNodes []*Node,
	removedNodes []*Node,
	previousSibling *Node,
	nextSibling *Node,
) {
	if target == nil || target.ownerDoc == nil {
		return
	}
	callbacks := mutationCallbacks[target.ownerDoc]
	for _, cb := range callbacks {
		cb.OnChildListMutation(target, addedNodes, removedNodes, previousSibling, nextSibling)
	}
}

// notifyAttributeMutation notifies all registered callbacks about an attribute mutation.
func notifyAttributeMutation(
	target *Node,
	attributeName string,
	attributeNamespace string,
	oldValue string,
) {
	if target == nil || target.ownerDoc == nil {
		return
	}
	callbacks := mutationCallbacks[target.ownerDoc]
	for _, cb := range callbacks {
		cb.OnAttributeMutation(target, attributeName, attributeNamespace, oldValue)
	}
}

// notifyCharacterDataMutation notifies all registered callbacks about a character data mutation.
func notifyCharacterDataMutation(
	target *Node,
	oldValue string,
) {
	if target == nil || target.ownerDoc == nil {
		return
	}
	callbacks := mutationCallbacks[target.ownerDoc]
	for _, cb := range callbacks {
		cb.OnCharacterDataMutation(target, oldValue)
	}
}

// notifyReplaceData notifies all registered callbacks about a "replace data" operation.
// This is used for insertData, deleteData, replaceData, and substringData operations.
func notifyReplaceData(
	target *Node,
	offset int,
	count int,
	data string,
) {
	NotifyReplaceData(target, offset, count, data)
}

// NotifyReplaceData notifies all registered callbacks about a "replace data" operation.
// This is exported for use by JavaScript bindings that need to trigger range mutations.
func NotifyReplaceData(
	target *Node,
	offset int,
	count int,
	data string,
) {
	if target == nil || target.ownerDoc == nil {
		return
	}
	callbacks := mutationCallbacks[target.ownerDoc]
	for _, cb := range callbacks {
		cb.OnReplaceData(target, offset, count, data)
	}
}
