package dom

import "sync"

// RangeRegistry tracks all live Range objects for a document.
// This is used to update Range boundary points when DOM mutations occur.
type RangeRegistry struct {
	mu     sync.RWMutex
	ranges map[*Range]struct{}
}

// rangeRegistries maps documents to their range registries.
var rangeRegistries = make(map[*Document]*RangeRegistry)
var registryMu sync.RWMutex

// getRangeRegistry returns the RangeRegistry for a document, creating one if needed.
func getRangeRegistry(doc *Document) *RangeRegistry {
	if doc == nil {
		return nil
	}

	registryMu.RLock()
	registry := rangeRegistries[doc]
	registryMu.RUnlock()

	if registry == nil {
		registryMu.Lock()
		// Double-check after acquiring write lock
		registry = rangeRegistries[doc]
		if registry == nil {
			registry = &RangeRegistry{
				ranges: make(map[*Range]struct{}),
			}
			rangeRegistries[doc] = registry
			// Register our mutation callback handler for this document
			RegisterMutationCallback(doc, &rangeMutationHandler{doc: doc})
		}
		registryMu.Unlock()
	}

	return registry
}

// registerRange adds a Range to its document's registry.
func registerRange(r *Range) {
	if r == nil || r.ownerDocument == nil {
		return
	}
	registry := getRangeRegistry(r.ownerDocument)
	registry.mu.Lock()
	registry.ranges[r] = struct{}{}
	registry.mu.Unlock()
}

// unregisterRange removes a Range from its document's registry.
// This is called when a Range is no longer needed (e.g., garbage collected).
func unregisterRange(r *Range) {
	if r == nil || r.ownerDocument == nil {
		return
	}
	registryMu.RLock()
	registry := rangeRegistries[r.ownerDocument]
	registryMu.RUnlock()

	if registry != nil {
		registry.mu.Lock()
		delete(registry.ranges, r)
		registry.mu.Unlock()
	}
}

// rangeMutationHandler implements MutationCallback to update Ranges on DOM mutations.
type rangeMutationHandler struct {
	doc *Document
}

// OnChildListMutation handles insertions and removals of child nodes.
func (h *rangeMutationHandler) OnChildListMutation(
	target *Node,
	addedNodes []*Node,
	removedNodes []*Node,
	previousSibling *Node,
	nextSibling *Node,
) {
	registryMu.RLock()
	registry := rangeRegistries[h.doc]
	registryMu.RUnlock()

	if registry == nil {
		return
	}

	registry.mu.RLock()
	// Make a copy of ranges to avoid holding lock during updates
	ranges := make([]*Range, 0, len(registry.ranges))
	for r := range registry.ranges {
		ranges = append(ranges, r)
	}
	registry.mu.RUnlock()

	// Process removals first (per DOM spec order)
	for _, removedNode := range removedNodes {
		oldIndex := getOldIndex(removedNode, previousSibling, nextSibling)
		for _, r := range ranges {
			updateRangeForRemoval(r, target, removedNode, oldIndex)
		}
	}

	// Process insertions
	// Calculate the starting index for inserted nodes
	if len(addedNodes) > 0 {
		startIndex := 0
		if previousSibling != nil {
			startIndex = indexOfChild(target, previousSibling) + 1
		}

		for i, insertedNode := range addedNodes {
			newIndex := startIndex + i
			for _, r := range ranges {
				updateRangeForInsertion(r, target, insertedNode, newIndex)
			}
		}
	}
}

// OnAttributeMutation handles attribute changes (no effect on Ranges).
func (h *rangeMutationHandler) OnAttributeMutation(
	target *Node,
	attributeName string,
	attributeNamespace string,
	oldValue string,
) {
	// Attribute mutations don't affect Range boundary points
}

// OnCharacterDataMutation handles text content changes (full data replacement).
// NOTE: This is now a no-op for Range mutation purposes.
// All character data mutations should go through OnReplaceData with proper offset/count.
// This callback is still fired by SetNodeValue but we ignore it to avoid double-processing
// when OnReplaceData was already called with the precise mutation parameters.
func (h *rangeMutationHandler) OnCharacterDataMutation(
	target *Node,
	oldValue string,
) {
	// No-op: Range mutations are now handled exclusively by OnReplaceData.
	// This ensures precise offset/count handling per the DOM spec.
}

// OnReplaceData handles the "replace data" algorithm for character data nodes.
// This provides precise offset/count/data information for Range adjustments.
func (h *rangeMutationHandler) OnReplaceData(
	target *Node,
	offset int,
	count int,
	data string,
) {
	registryMu.RLock()
	registry := rangeRegistries[h.doc]
	registryMu.RUnlock()

	if registry == nil {
		return
	}

	registry.mu.RLock()
	ranges := make([]*Range, 0, len(registry.ranges))
	for r := range registry.ranges {
		ranges = append(ranges, r)
	}
	registry.mu.RUnlock()

	dataLength := len(data)

	for _, r := range ranges {
		updateRangeForReplaceData(r, target, offset, count, dataLength)
	}
}

// getOldIndex determines the index a removed node had before removal.
// This is calculated based on the sibling information.
func getOldIndex(removedNode *Node, previousSibling, nextSibling *Node) int {
	if previousSibling != nil {
		// The removed node was after previousSibling
		return indexOfChild(previousSibling.parentNode, previousSibling) + 1
	}
	// The removed node was the first child
	return 0
}

// updateRangeForRemoval updates a Range's boundary points after a node is removed.
// Implements the DOM spec "remove" algorithm for live ranges:
// https://dom.spec.whatwg.org/#concept-node-remove
func updateRangeForRemoval(r *Range, parent *Node, removedNode *Node, oldIndex int) {
	// For each live range whose start node is removedNode or a descendant of removedNode,
	// set the start boundary point to (parent, oldIndex)
	if r.startContainer == removedNode || isDescendant(r.startContainer, removedNode) {
		r.startContainer = parent
		r.startOffset = oldIndex
	}

	// For each live range whose end node is removedNode or a descendant of removedNode,
	// set the end boundary point to (parent, oldIndex)
	if r.endContainer == removedNode || isDescendant(r.endContainer, removedNode) {
		r.endContainer = parent
		r.endOffset = oldIndex
	}

	// For each live range whose start node is parent and start offset is greater than oldIndex,
	// subtract 1 from the start offset
	if r.startContainer == parent && r.startOffset > oldIndex {
		r.startOffset--
	}

	// For each live range whose end node is parent and end offset is greater than oldIndex,
	// subtract 1 from the end offset
	if r.endContainer == parent && r.endOffset > oldIndex {
		r.endOffset--
	}
}

// updateRangeForInsertion updates a Range's boundary points after a node is inserted.
// Implements the DOM spec "insert" algorithm for live ranges:
// https://dom.spec.whatwg.org/#concept-node-insert
func updateRangeForInsertion(r *Range, parent *Node, insertedNode *Node, newIndex int) {
	// For each live range whose start node is parent and start offset is greater than newIndex,
	// add 1 to the start offset
	if r.startContainer == parent && r.startOffset > newIndex {
		r.startOffset++
	}

	// For each live range whose end node is parent and end offset is greater than newIndex,
	// add 1 to the end offset
	if r.endContainer == parent && r.endOffset > newIndex {
		r.endOffset++
	}
}

// updateRangeForCharacterDataChange updates a Range's boundary points after character data changes.
// This handles the general case of text replacement/modification.
// The DOM spec defines this in terms of "replace data" algorithm.
// When the entire data is replaced (like setting nodeValue), this is equivalent to replaceData(0, oldLength, newData).
func updateRangeForCharacterDataChange(r *Range, node *Node, oldLength, newLength int) {
	// For complete data replacement (offset=0, count=oldLength):
	// Per DOM spec "replace data" algorithm:
	//
	// "For every boundary point whose node is node, and whose offset is
	// greater than offset but less than or equal to offset plus count, set
	// its offset to offset."
	// -> For offsets in (0, oldLength], set to 0
	//
	// "For every boundary point whose node is node, and whose offset is
	// greater than offset plus count, add the length of data to its offset,
	// then subtract count from it."
	// -> For offsets > oldLength, add (newLength - oldLength)

	// Handle start container
	if r.startContainer == node {
		if r.startOffset > oldLength {
			// Offset was past the old length, adjust by the length difference
			r.startOffset += newLength - oldLength
		} else if r.startOffset > 0 {
			// Offset was within the replaced content (0, oldLength]
			// Per spec, set to the replacement offset (which is 0)
			r.startOffset = 0
		}
		// offset == 0 stays at 0
	}

	// Handle end container
	if r.endContainer == node {
		if r.endOffset > oldLength {
			// Offset was past the old length, adjust by the length difference
			r.endOffset += newLength - oldLength
		} else if r.endOffset > 0 {
			// Offset was within the replaced content (0, oldLength]
			// Per spec, set to the replacement offset (which is 0)
			r.endOffset = 0
		}
		// offset == 0 stays at 0
	}
}

// updateRangeForReplaceData updates a Range's boundary points after a "replace data" operation.
// Implements the DOM spec algorithm:
// https://dom.spec.whatwg.org/#concept-cd-replace
//
// Parameters:
//   - node: the character data node being modified
//   - offset: the starting offset of the replacement
//   - count: the number of characters being replaced
//   - dataLength: the length of the replacement data
func updateRangeForReplaceData(r *Range, node *Node, offset, count, dataLength int) {
	// Per DOM spec "replace data" algorithm:
	//
	// "For every boundary point whose node is node, and whose offset is
	// greater than offset but less than or equal to offset plus count, set
	// its offset to offset."
	//
	// "For every boundary point whose node is node, and whose offset is
	// greater than offset plus count, add the length of data to its offset,
	// then subtract count from it."

	// Handle start container
	if r.startContainer == node {
		if r.startOffset > offset && r.startOffset <= offset+count {
			// Offset is within the replaced range (offset, offset+count]
			r.startOffset = offset
		} else if r.startOffset > offset+count {
			// Offset is after the replaced range
			r.startOffset += dataLength - count
		}
	}

	// Handle end container
	if r.endContainer == node {
		if r.endOffset > offset && r.endOffset <= offset+count {
			// Offset is within the replaced range (offset, offset+count]
			r.endOffset = offset
		} else if r.endOffset > offset+count {
			// Offset is after the replaced range
			r.endOffset += dataLength - count
		}
	}
}

// isDescendant returns true if node is a descendant of potentialAncestor.
func isDescendant(node, potentialAncestor *Node) bool {
	for n := node; n != nil; n = n.parentNode {
		if n == potentialAncestor {
			return true
		}
	}
	return false
}

// ClearRangeRegistry removes all registered ranges for a document.
// This should be called when a document is being cleaned up.
func ClearRangeRegistry(doc *Document) {
	if doc == nil {
		return
	}
	registryMu.Lock()
	delete(rangeRegistries, doc)
	registryMu.Unlock()
}
