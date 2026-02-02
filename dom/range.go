package dom

// Range constants for compareBoundaryPoints
const (
	StartToStart = 0
	StartToEnd   = 1
	EndToEnd     = 2
	EndToStart   = 3
)

// Range represents a fragment of a document that can contain nodes and parts of text nodes.
// It is a live range, meaning it updates as the DOM changes.
type Range struct {
	startContainer *Node
	startOffset    int
	endContainer   *Node
	endOffset      int
	ownerDocument  *Document
}

// NewRange creates a new Range with both boundary points set to the document.
// The Range is automatically registered for live mutation tracking.
func NewRange(doc *Document) *Range {
	r := &Range{
		startContainer: doc.AsNode(),
		startOffset:    0,
		endContainer:   doc.AsNode(),
		endOffset:      0,
		ownerDocument:  doc,
	}
	// Register this range for live mutation tracking
	registerRange(r)
	return r
}

// StartContainer returns the node where the range starts.
func (r *Range) StartContainer() *Node {
	return r.startContainer
}

// StartOffset returns the offset within the start container.
func (r *Range) StartOffset() int {
	return r.startOffset
}

// EndContainer returns the node where the range ends.
func (r *Range) EndContainer() *Node {
	return r.endContainer
}

// EndOffset returns the offset within the end container.
func (r *Range) EndOffset() int {
	return r.endOffset
}

// Collapsed returns true if start and end are the same point.
func (r *Range) Collapsed() bool {
	return r.startContainer == r.endContainer && r.startOffset == r.endOffset
}

// CommonAncestorContainer returns the deepest node that contains both boundary points.
func (r *Range) CommonAncestorContainer() *Node {
	// Build ancestry chain for start container
	startAncestors := make(map[*Node]bool)
	for node := r.startContainer; node != nil; node = node.parentNode {
		startAncestors[node] = true
	}

	// Find the first end container ancestor that's also a start ancestor
	for node := r.endContainer; node != nil; node = node.parentNode {
		if startAncestors[node] {
			return node
		}
	}

	return nil
}

// SetStart sets the start boundary point of the range.
func (r *Range) SetStart(node *Node, offset int) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	// Check if node is a doctype
	if node.nodeType == DocumentTypeNode {
		return ErrInvalidNodeType("The supplied node is a DocumentType which is not a valid boundary point.")
	}

	// Check offset bounds
	length := nodeLength(node)
	if offset < 0 || offset > length {
		return ErrIndexSize("The offset is out of range.")
	}

	// Check if node is in a different tree than the current end container
	// If so, the range should collapse to the new start point
	nodeRoot := node.GetRootNode()
	endRoot := r.endContainer.GetRootNode()
	differentTree := nodeRoot != endRoot

	// Set the start boundary point
	r.startContainer = node
	r.startOffset = offset

	// If in different tree OR if start is after end, collapse to start
	if differentTree || r.comparePoints(r.startContainer, r.startOffset, r.endContainer, r.endOffset) > 0 {
		r.endContainer = r.startContainer
		r.endOffset = r.startOffset
	}

	return nil
}

// SetEnd sets the end boundary point of the range.
func (r *Range) SetEnd(node *Node, offset int) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	// Check if node is a doctype
	if node.nodeType == DocumentTypeNode {
		return ErrInvalidNodeType("The supplied node is a DocumentType which is not a valid boundary point.")
	}

	// Check offset bounds
	length := nodeLength(node)
	if offset < 0 || offset > length {
		return ErrIndexSize("The offset is out of range.")
	}

	// Check if node is in a different tree than the current start container
	// If so, the range should collapse to the new end point
	nodeRoot := node.GetRootNode()
	startRoot := r.startContainer.GetRootNode()
	differentTree := nodeRoot != startRoot

	// Set the end boundary point
	r.endContainer = node
	r.endOffset = offset

	// If in different tree OR if end is before start, collapse to end
	if differentTree || r.comparePoints(r.startContainer, r.startOffset, r.endContainer, r.endOffset) > 0 {
		r.startContainer = r.endContainer
		r.startOffset = r.endOffset
	}

	return nil
}

// SetStartBefore sets the start to immediately before the given node.
func (r *Range) SetStartBefore(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}
	parent := node.parentNode
	if parent == nil {
		return ErrInvalidNodeType("The node has no parent.")
	}
	return r.SetStart(parent, indexOfChild(parent, node))
}

// SetStartAfter sets the start to immediately after the given node.
func (r *Range) SetStartAfter(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}
	parent := node.parentNode
	if parent == nil {
		return ErrInvalidNodeType("The node has no parent.")
	}
	return r.SetStart(parent, indexOfChild(parent, node)+1)
}

// SetEndBefore sets the end to immediately before the given node.
func (r *Range) SetEndBefore(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}
	parent := node.parentNode
	if parent == nil {
		return ErrInvalidNodeType("The node has no parent.")
	}
	return r.SetEnd(parent, indexOfChild(parent, node))
}

// SetEndAfter sets the end to immediately after the given node.
func (r *Range) SetEndAfter(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}
	parent := node.parentNode
	if parent == nil {
		return ErrInvalidNodeType("The node has no parent.")
	}
	return r.SetEnd(parent, indexOfChild(parent, node)+1)
}

// Collapse collapses the range to one of its boundary points.
// If toStart is true, collapses to the start; otherwise to the end.
func (r *Range) Collapse(toStart bool) {
	if toStart {
		r.endContainer = r.startContainer
		r.endOffset = r.startOffset
	} else {
		r.startContainer = r.endContainer
		r.startOffset = r.endOffset
	}
}

// SelectNode sets the range to contain the given node and its contents.
func (r *Range) SelectNode(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}
	parent := node.parentNode
	if parent == nil {
		return ErrInvalidNodeType("The node has no parent.")
	}

	index := indexOfChild(parent, node)
	r.startContainer = parent
	r.startOffset = index
	r.endContainer = parent
	r.endOffset = index + 1
	return nil
}

// SelectNodeContents sets the range to contain the contents of the given node.
func (r *Range) SelectNodeContents(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	// Check if node is a doctype
	if node.nodeType == DocumentTypeNode {
		return ErrInvalidNodeType("The supplied node is a DocumentType.")
	}

	r.startContainer = node
	r.startOffset = 0
	r.endContainer = node
	r.endOffset = nodeLength(node)
	return nil
}

// CompareBoundaryPoints compares the boundary points of two ranges.
// Returns -1, 0, or 1 depending on whether the first point is before, equal to, or after the second.
func (r *Range) CompareBoundaryPoints(how int, sourceRange *Range) (int, error) {
	if sourceRange == nil {
		return 0, ErrNotFound("Source range is null")
	}

	// Check if ranges are in different documents
	if r.ownerDocument != sourceRange.ownerDocument {
		return 0, ErrWrongDocument("The two Ranges are not in the same tree.")
	}

	var thisContainer, sourceContainer *Node
	var thisOffset, sourceOffset int

	switch how {
	case StartToStart:
		thisContainer = r.startContainer
		thisOffset = r.startOffset
		sourceContainer = sourceRange.startContainer
		sourceOffset = sourceRange.startOffset
	case StartToEnd:
		thisContainer = r.endContainer
		thisOffset = r.endOffset
		sourceContainer = sourceRange.startContainer
		sourceOffset = sourceRange.startOffset
	case EndToEnd:
		thisContainer = r.endContainer
		thisOffset = r.endOffset
		sourceContainer = sourceRange.endContainer
		sourceOffset = sourceRange.endOffset
	case EndToStart:
		thisContainer = r.startContainer
		thisOffset = r.startOffset
		sourceContainer = sourceRange.endContainer
		sourceOffset = sourceRange.endOffset
	default:
		return 0, ErrNotSupported("Invalid comparison type")
	}

	return r.comparePoints(thisContainer, thisOffset, sourceContainer, sourceOffset), nil
}

// comparePoints compares two boundary points.
// Returns -1 if (nodeA, offsetA) is before (nodeB, offsetB), 0 if equal, 1 if after.
func (r *Range) comparePoints(nodeA *Node, offsetA int, nodeB *Node, offsetB int) int {
	if nodeA == nodeB {
		if offsetA < offsetB {
			return -1
		}
		if offsetA > offsetB {
			return 1
		}
		return 0
	}

	// Check if nodeA is an ancestor of nodeB
	if isAncestor(nodeA, nodeB) {
		child := nodeB
		for child.parentNode != nodeA {
			child = child.parentNode
		}
		if indexOfChild(nodeA, child) < offsetA {
			return 1
		}
		return -1
	}

	// Check if nodeB is an ancestor of nodeA
	if isAncestor(nodeB, nodeA) {
		child := nodeA
		for child.parentNode != nodeB {
			child = child.parentNode
		}
		if indexOfChild(nodeB, child) < offsetB {
			return -1
		}
		return 1
	}

	// Neither is an ancestor - find common ancestor and compare
	return r.compareSiblingOrder(nodeA, nodeB)
}

// compareSiblingOrder compares two nodes that share a common ancestor.
func (r *Range) compareSiblingOrder(nodeA, nodeB *Node) int {
	// Build paths from each node to root
	pathA := make([]*Node, 0)
	for n := nodeA; n != nil; n = n.parentNode {
		pathA = append([]*Node{n}, pathA...)
	}

	pathB := make([]*Node, 0)
	for n := nodeB; n != nil; n = n.parentNode {
		pathB = append([]*Node{n}, pathB...)
	}

	// Find where paths diverge
	var ancestorA, ancestorB *Node
	for i := 0; i < len(pathA) && i < len(pathB); i++ {
		if pathA[i] != pathB[i] {
			if i > 0 {
				ancestorA = pathA[i]
				ancestorB = pathB[i]
			}
			break
		}
	}

	if ancestorA == nil || ancestorB == nil {
		return 0
	}

	// Compare sibling order
	parent := ancestorA.parentNode
	for child := parent.firstChild; child != nil; child = child.nextSibling {
		if child == ancestorA {
			return -1
		}
		if child == ancestorB {
			return 1
		}
	}

	return 0
}

// DeleteContents removes the contents of the range from the document.
func (r *Range) DeleteContents() error {
	if r.Collapsed() {
		return nil
	}

	// If start and end are in the same text node, just delete the text
	if r.startContainer == r.endContainer {
		if r.startContainer.nodeType == TextNode {
			text := r.startContainer.NodeValue()
			// Use UTF-16 aware slicing since offsets are in UTF-16 code units
			newText := UTF16SliceTo(text, r.startOffset) + UTF16SliceFrom(text, r.endOffset)
			r.startContainer.SetNodeValue(newText)
			r.endOffset = r.startOffset
			return nil
		}
	}

	// Extract and discard (similar to extractContents but we don't return anything)
	_, err := r.ExtractContents()
	return err
}

// ExtractContents moves the contents of the range into a DocumentFragment and returns it.
func (r *Range) ExtractContents() (*DocumentFragment, error) {
	frag := r.ownerDocument.CreateDocumentFragment()

	if r.Collapsed() {
		return frag, nil
	}

	// If start and end are in the same text node
	if r.startContainer == r.endContainer && r.startContainer.nodeType == TextNode {
		clone := r.startContainer.CloneNode(false)
		text := r.startContainer.NodeValue()
		// Use UTF-16 aware slicing since offsets are in UTF-16 code units
		clone.SetNodeValue(UTF16Substring(text, r.startOffset, r.endOffset))

		// Remove the extracted text from original
		r.startContainer.SetNodeValue(UTF16SliceTo(text, r.startOffset) + UTF16SliceFrom(text, r.endOffset))

		(*Node)(frag).AppendChild(clone)
		r.endOffset = r.startOffset
		return frag, nil
	}

	// Complex case: range spans multiple nodes
	commonAncestor := r.CommonAncestorContainer()
	if commonAncestor == nil {
		return frag, nil
	}

	// If start container is text and partially selected, split it
	var firstPartiallyContained *Node
	if r.startContainer.nodeType == TextNode && r.startOffset > 0 {
		text := r.startContainer.NodeValue()
		firstPartiallyContained = r.startContainer.CloneNode(false)
		// Use UTF-16 aware slicing
		firstPartiallyContained.SetNodeValue(UTF16SliceFrom(text, r.startOffset))
		r.startContainer.SetNodeValue(UTF16SliceTo(text, r.startOffset))
	}

	// If end container is text and partially selected, split it
	var lastPartiallyContained *Node
	if r.endContainer.nodeType == TextNode && r.endOffset < UTF16Length(r.endContainer.NodeValue()) {
		text := r.endContainer.NodeValue()
		lastPartiallyContained = r.endContainer.CloneNode(false)
		// Use UTF-16 aware slicing
		lastPartiallyContained.SetNodeValue(UTF16SliceTo(text, r.endOffset))
		r.endContainer.SetNodeValue(UTF16SliceFrom(text, r.endOffset))
	}

	// Find contained children
	containedChildren := r.getContainedChildren(commonAncestor)

	// Move contained children to fragment
	for _, child := range containedChildren {
		if child.parentNode != nil {
			child.parentNode.RemoveChild(child)
		}
		(*Node)(frag).AppendChild(child)
	}

	// Add partial text nodes
	if firstPartiallyContained != nil {
		(*Node)(frag).InsertBefore(firstPartiallyContained, (*Node)(frag).firstChild)
	}
	if lastPartiallyContained != nil {
		(*Node)(frag).AppendChild(lastPartiallyContained)
	}

	// Collapse range
	r.endContainer = r.startContainer
	r.endOffset = r.startOffset

	return frag, nil
}

// CloneContents returns a DocumentFragment containing a copy of the range's contents.
func (r *Range) CloneContents() (*DocumentFragment, error) {
	frag := r.ownerDocument.CreateDocumentFragment()

	if r.Collapsed() {
		return frag, nil
	}

	// If start and end are in the same text node
	if r.startContainer == r.endContainer && r.startContainer.nodeType == TextNode {
		clone := r.startContainer.CloneNode(false)
		text := r.startContainer.NodeValue()
		// Use UTF-16 aware slicing
		clone.SetNodeValue(UTF16Substring(text, r.startOffset, r.endOffset))
		(*Node)(frag).AppendChild(clone)
		return frag, nil
	}

	// Complex case: clone nodes in range
	commonAncestor := r.CommonAncestorContainer()
	if commonAncestor == nil {
		return frag, nil
	}

	// Get all contained children and clone them
	containedChildren := r.getContainedChildren(commonAncestor)
	for _, child := range containedChildren {
		clone := child.CloneNode(true)
		(*Node)(frag).AppendChild(clone)
	}

	// Handle partial text nodes
	if r.startContainer.nodeType == TextNode && r.startOffset > 0 {
		text := r.startContainer.NodeValue()
		// Use UTF-16 aware slicing
		textNode := r.ownerDocument.CreateTextNode(UTF16SliceFrom(text, r.startOffset))
		// Insert at beginning
		if (*Node)(frag).firstChild != nil {
			(*Node)(frag).InsertBefore(textNode, (*Node)(frag).firstChild)
		} else {
			(*Node)(frag).AppendChild(textNode)
		}
	}

	if r.endContainer.nodeType == TextNode && r.endOffset < UTF16Length(r.endContainer.NodeValue()) {
		text := r.endContainer.NodeValue()
		// Use UTF-16 aware slicing
		textNode := r.ownerDocument.CreateTextNode(UTF16SliceTo(text, r.endOffset))
		(*Node)(frag).AppendChild(textNode)
	}

	return frag, nil
}

// InsertNode inserts a node at the start of the range.
// Per https://dom.spec.whatwg.org/#dom-range-insertnode
func (r *Range) InsertNode(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	// Check if start container is a Text node (includes CDATASection)
	// Per DOM spec, CDATASection inherits from Text
	if r.startContainer.nodeType == TextNode || r.startContainer.nodeType == CDATASectionNode {
		parent := r.startContainer.parentNode
		if parent == nil {
			return ErrHierarchyRequest("Cannot insert into an orphan text node")
		}

		// Split the text node if needed
		textLen := UTF16Length(r.startContainer.NodeValue())
		if r.startOffset > 0 && r.startOffset < textLen {
			text := r.startContainer.NodeValue()
			// Use UTF-16 aware slicing
			r.startContainer.SetNodeValue(UTF16SliceTo(text, r.startOffset))
			// Create appropriate node type for the split
			var newText *Node
			if r.startContainer.nodeType == CDATASectionNode {
				newText = r.ownerDocument.CreateCDATASection(UTF16SliceFrom(text, r.startOffset))
			} else {
				newText = r.ownerDocument.CreateTextNode(UTF16SliceFrom(text, r.startOffset))
			}
			if newText != nil {
				parent.InsertBefore(newText, r.startContainer.nextSibling)
			}
		}

		// Insert the node after the start container
		parent.InsertBefore(node, r.startContainer.nextSibling)
	} else {
		// Insert at the offset position
		refChild := r.startContainer.firstChild
		for i := 0; i < r.startOffset && refChild != nil; i++ {
			refChild = refChild.nextSibling
		}
		r.startContainer.InsertBefore(node, refChild)
	}

	return nil
}

// SurroundContents wraps the range contents with a new parent element.
// Per https://dom.spec.whatwg.org/#dom-range-surroundcontents
func (r *Range) SurroundContents(newParent *Node) error {
	if newParent == nil {
		return ErrNotFound("New parent is null")
	}

	// Step 1: If a non-Text node is partially contained in this range, throw InvalidStateError.
	// A node is "partially contained" if it is an ancestor container of the range's start
	// but not its end, or vice versa.
	// We traverse from commonAncestorContainer to check all nodes in the range.
	commonAncestor := r.CommonAncestorContainer()
	if commonAncestor != nil {
		// Traverse all nodes from commonAncestor in tree order
		stop := nextNodeDescendants(commonAncestor)
		for node := commonAncestor; node != stop; node = nextNodeInTree(node) {
			// A node is partially contained if it's an inclusive ancestor of start but not end,
			// or vice versa
			if !isTextLike(node) && r.isPartiallyContained(node) {
				return ErrInvalidState("Range partially selects a non-Text node")
			}
		}
	}

	// Step 2: If newParent is a Document, DocumentType, or DocumentFragment, throw InvalidNodeTypeError
	if newParent.nodeType == DocumentNode || newParent.nodeType == DocumentTypeNode || newParent.nodeType == DocumentFragmentNode {
		return ErrInvalidNodeType("Invalid new parent type")
	}

	// Step 3: Let fragment be the result of extracting this range
	frag, err := r.ExtractContents()
	if err != nil {
		return err
	}

	// Step 4: If newParent has children, replace all with null within newParent
	for newParent.firstChild != nil {
		newParent.RemoveChild(newParent.firstChild)
	}

	// Step 5: Insert newParent into this range
	if err := r.InsertNode(newParent); err != nil {
		return err
	}

	// Step 6: Append fragment to newParent
	newParent.AppendChild((*Node)(frag))

	// Step 7: Select newParent within this range
	return r.SelectNode(newParent)
}

// isTextLike returns true if the node is a Text or CDATASection node
func isTextLike(node *Node) bool {
	return node.nodeType == TextNode || node.nodeType == CDATASectionNode
}

// isPartiallyContained returns true if node is partially contained in the range.
// A node is partially contained if it is an inclusive ancestor of the range's start
// but not its end, or vice versa.
func (r *Range) isPartiallyContained(node *Node) bool {
	isAncestorOfStart := r.isInclusiveAncestor(node, r.startContainer)
	isAncestorOfEnd := r.isInclusiveAncestor(node, r.endContainer)
	return (isAncestorOfStart && !isAncestorOfEnd) || (!isAncestorOfStart && isAncestorOfEnd)
}

// isInclusiveAncestor returns true if ancestor is an inclusive ancestor of node
func (r *Range) isInclusiveAncestor(ancestor, node *Node) bool {
	for n := node; n != nil; n = n.parentNode {
		if n == ancestor {
			return true
		}
	}
	return false
}

// nextNodeInTree returns the next node in tree order (pre-order traversal)
func nextNodeInTree(node *Node) *Node {
	if node.firstChild != nil {
		return node.firstChild
	}
	return nextNodeDescendants(node)
}

// nextNodeDescendants returns the next node after node and all its descendants
func nextNodeDescendants(node *Node) *Node {
	for node != nil {
		if node.nextSibling != nil {
			return node.nextSibling
		}
		node = node.parentNode
	}
	return nil
}

// CloneRange returns a copy of this range.
// The cloned Range is also registered for live mutation tracking.
func (r *Range) CloneRange() *Range {
	clone := &Range{
		startContainer: r.startContainer,
		startOffset:    r.startOffset,
		endContainer:   r.endContainer,
		endOffset:      r.endOffset,
		ownerDocument:  r.ownerDocument,
	}
	// Register the cloned range for live mutation tracking
	registerRange(clone)
	return clone
}

// Detach is a no-op per the current DOM specification.
// Previously it was used to free up resources, but now ranges are garbage collected.
func (r *Range) Detach() {
	// No-op per spec
}

// ToString returns the text content of the range.
func (r *Range) ToString() string {
	if r.Collapsed() {
		return ""
	}

	// If range is within a single text node
	if r.startContainer == r.endContainer && r.startContainer.nodeType == TextNode {
		text := r.startContainer.NodeValue()
		// Use UTF-16 aware slicing since offsets are in UTF-16 code units
		return UTF16Substring(text, r.startOffset, r.endOffset)
	}

	// Build string by traversing all text nodes in document order within the range
	var result string

	// Get the common ancestor to limit traversal
	commonAncestor := r.CommonAncestorContainer()
	if commonAncestor == nil {
		return ""
	}

	// Traverse all nodes in document order within the range
	r.traverseTextNodes(commonAncestor, func(textNode *Node) bool {
		text := textNode.NodeValue()

		// Determine what portion of this text node is in the range
		// Offsets are in UTF-16 code units
		var startIdx, endIdx int

		if textNode == r.startContainer {
			startIdx = r.startOffset
		} else {
			startIdx = 0
		}

		if textNode == r.endContainer {
			endIdx = r.endOffset
		} else {
			endIdx = UTF16Length(text)
		}

		if startIdx < endIdx {
			// Use UTF-16 aware slicing
			result += UTF16Substring(text, startIdx, endIdx)
		}

		return true // continue traversal
	})

	return result
}

// traverseTextNodes traverses all text nodes in document order within the range.
// The callback is called for each text node. Return false from callback to stop traversal.
func (r *Range) traverseTextNodes(root *Node, callback func(*Node) bool) {
	// Perform a depth-first traversal starting from root
	var traverse func(node *Node) bool
	traverse = func(node *Node) bool {
		// Check if this node is before the range starts
		if r.isNodeBeforeRange(node) {
			// Skip this node but continue with siblings
			return true
		}

		// Check if this node is after the range ends
		if r.isNodeAfterRange(node) {
			// Stop traversal
			return false
		}

		// If this is a text node and it's within the range, call callback
		if node.nodeType == TextNode {
			if r.nodeIntersectsRange(node) {
				if !callback(node) {
					return false
				}
			}
		}

		// Traverse children
		for child := node.firstChild; child != nil; child = child.nextSibling {
			if !traverse(child) {
				return false
			}
		}

		return true
	}

	traverse(root)
}

// isNodeBeforeRange checks if the entire node is before the range.
func (r *Range) isNodeBeforeRange(node *Node) bool {
	parent := node.parentNode
	if parent == nil {
		return false
	}

	nodeEnd := indexOfChild(parent, node) + 1
	return r.comparePoints(parent, nodeEnd, r.startContainer, r.startOffset) <= 0
}

// isNodeAfterRange checks if the entire node is after the range.
func (r *Range) isNodeAfterRange(node *Node) bool {
	parent := node.parentNode
	if parent == nil {
		return false
	}

	nodeStart := indexOfChild(parent, node)
	return r.comparePoints(parent, nodeStart, r.endContainer, r.endOffset) >= 0
}

// nodeIntersectsRange checks if a node intersects the range.
func (r *Range) nodeIntersectsRange(node *Node) bool {
	parent := node.parentNode
	if parent == nil {
		// Root node - check if it's part of the range
		return true
	}

	nodeStart := indexOfChild(parent, node)
	nodeEnd := nodeStart + 1

	// Node starts after range ends
	if r.comparePoints(parent, nodeStart, r.endContainer, r.endOffset) >= 0 {
		return false
	}

	// Node ends before range starts
	if r.comparePoints(parent, nodeEnd, r.startContainer, r.startOffset) <= 0 {
		return false
	}

	return true
}

// CreateContextualFragment parses the given HTML and returns a DocumentFragment.
func (r *Range) CreateContextualFragment(fragment string) (*DocumentFragment, error) {
	// Determine context element
	var context *Element
	if r.startContainer.nodeType == ElementNode {
		context = (*Element)(r.startContainer)
	} else if r.startContainer.parentNode != nil && r.startContainer.parentNode.nodeType == ElementNode {
		context = (*Element)(r.startContainer.parentNode)
	}

	// Parse the fragment
	doc, err := ParseHTML("<body>" + fragment + "</body>")
	if err != nil {
		return nil, err
	}

	// Create a DocumentFragment and move parsed nodes into it
	frag := r.ownerDocument.CreateDocumentFragment()
	body := doc.Body()
	if body != nil {
		for body.AsNode().firstChild != nil {
			child := body.AsNode().firstChild
			body.AsNode().RemoveChild(child)
			// Adopt into our document
			r.ownerDocument.AdoptNode(child)
			(*Node)(frag).AppendChild(child)
		}
	}

	// If we have a context, we might need to unwrap
	_ = context // May be used for more complex parsing in the future

	return frag, nil
}

// IsPointInRange returns true if the given point is within the range.
func (r *Range) IsPointInRange(node *Node, offset int) bool {
	if node == nil {
		return false
	}

	// Check same document
	if node.OwnerDocument() != r.ownerDocument && node != r.ownerDocument.AsNode() {
		return false
	}

	// Check if doctype
	if node.nodeType == DocumentTypeNode {
		return false
	}

	// Check bounds
	if offset < 0 || offset > nodeLength(node) {
		return false
	}

	// Compare to start
	if r.comparePoints(node, offset, r.startContainer, r.startOffset) < 0 {
		return false
	}

	// Compare to end
	if r.comparePoints(node, offset, r.endContainer, r.endOffset) > 0 {
		return false
	}

	return true
}

// ComparePoint compares a point to the range.
// Returns -1 if before, 0 if inside, 1 if after.
func (r *Range) ComparePoint(node *Node, offset int) (int, error) {
	if node == nil {
		return 0, ErrNotFound("Node is null")
	}

	// Check same document
	if node.OwnerDocument() != r.ownerDocument && node != r.ownerDocument.AsNode() {
		return 0, ErrWrongDocument("Node is not in the same document")
	}

	// Check if doctype
	if node.nodeType == DocumentTypeNode {
		return 0, ErrInvalidNodeType("Node is a DocumentType")
	}

	// Check bounds
	if offset < 0 || offset > nodeLength(node) {
		return 0, ErrIndexSize("Offset is out of range")
	}

	// Compare to start
	if r.comparePoints(node, offset, r.startContainer, r.startOffset) < 0 {
		return -1, nil
	}

	// Compare to end
	if r.comparePoints(node, offset, r.endContainer, r.endOffset) > 0 {
		return 1, nil
	}

	return 0, nil
}

// IntersectsNode returns true if the range intersects the given node.
// Per DOM spec: https://dom.spec.whatwg.org/#dom-range-intersectsnode
// 1. If range's root is not the same as node's root, return false
// 2. Let parent be node's parent
// 3. If parent is null, return true
// 4. Let offset be node's index
// 5. If (parent, offset) is before range's end and (parent, offset + 1) is after range's start, return true
// 6. Return false
func (r *Range) IntersectsNode(node *Node) bool {
	if node == nil {
		return false
	}

	// Step 1: Check if range's root is the same as node's root
	// The range's root is the root of the tree containing the start container
	rangeRoot := r.startContainer.GetRootNode()
	nodeRoot := node.GetRootNode()
	if rangeRoot != nodeRoot {
		return false
	}

	// Step 2-3: If parent is null, return true
	parent := node.parentNode
	if parent == nil {
		return true
	}

	// Step 4: Get node's index within parent
	offset := indexOfChild(parent, node)

	// Step 5: Check if (parent, offset) is before range's end AND (parent, offset+1) is after range's start
	// "before" means comparePoints returns < 0
	// "after" means comparePoints returns > 0
	beforeEnd := r.comparePoints(parent, offset, r.endContainer, r.endOffset) < 0
	afterStart := r.comparePoints(parent, offset+1, r.startContainer, r.startOffset) > 0

	if beforeEnd && afterStart {
		return true
	}

	// Step 6: Return false
	return false
}

// Helper functions

// nodeLength returns the length of a node for range purposes.
// For text/comment/processing instruction nodes, it's the data length in UTF-16 code units.
// For other nodes, it's the number of child nodes.
// Per DOM spec, Range offsets are measured in UTF-16 code units for character data nodes.
func nodeLength(node *Node) int {
	switch node.nodeType {
	case TextNode, CommentNode, ProcessingInstructionNode, CDATASectionNode:
		return UTF16Length(node.NodeValue())
	default:
		count := 0
		for child := node.firstChild; child != nil; child = child.nextSibling {
			count++
		}
		return count
	}
}

// indexOfChild returns the index of a child within its parent.
func indexOfChild(parent, child *Node) int {
	index := 0
	for c := parent.firstChild; c != nil; c = c.nextSibling {
		if c == child {
			return index
		}
		index++
	}
	return -1
}

// isAncestor returns true if ancestor is an ancestor of node.
func isAncestor(ancestor, node *Node) bool {
	for n := node.parentNode; n != nil; n = n.parentNode {
		if n == ancestor {
			return true
		}
	}
	return false
}

// getContainedChildren returns child nodes that are fully contained in the range.
func (r *Range) getContainedChildren(ancestor *Node) []*Node {
	var result []*Node

	for child := ancestor.firstChild; child != nil; child = child.nextSibling {
		if r.containsNode(child) {
			result = append(result, child)
		}
	}

	return result
}

// containsNode returns true if the node is fully contained in the range.
func (r *Range) containsNode(node *Node) bool {
	parent := node.parentNode
	if parent == nil {
		return false
	}

	index := indexOfChild(parent, node)

	// Node start must be at or after range start
	if r.comparePoints(parent, index, r.startContainer, r.startOffset) < 0 {
		return false
	}

	// Node end must be at or before range end
	if r.comparePoints(parent, index+1, r.endContainer, r.endOffset) > 0 {
		return false
	}

	return true
}

// getTextContent returns the text content of a node.
func getTextContent(node *Node) string {
	switch node.nodeType {
	case TextNode, CDATASectionNode:
		return node.NodeValue()
	case ElementNode, DocumentFragmentNode:
		var result string
		for child := node.firstChild; child != nil; child = child.nextSibling {
			result += getTextContent(child)
		}
		return result
	default:
		return ""
	}
}

// ErrInvalidNodeType creates an InvalidNodeTypeError.
func ErrInvalidNodeType(message string) *DOMError {
	return &DOMError{Name: "InvalidNodeTypeError", Message: message}
}
