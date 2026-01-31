package dom

import (
	"strings"
	"unsafe"
)

// unsafePointer returns an unsafe.Pointer for a node.
// Used for consistent ordering of disconnected nodes.
func unsafePointer(n *Node) unsafe.Pointer {
	return unsafe.Pointer(n)
}

// Node represents a node in the DOM tree. It is the base interface from which
// Document, Element, Text, Comment, and other node types inherit.
type Node struct {
	nodeType   NodeType
	nodeName   string
	nodeValue  *string // nil for Element, Document, DocumentFragment
	ownerDoc   *Document
	parentNode *Node
	childNodes *NodeList

	// First/last child and sibling pointers for efficient traversal
	firstChild  *Node
	lastChild   *Node
	prevSibling *Node
	nextSibling *Node

	// Type-specific data (only one will be non-nil based on nodeType)
	elementData  *elementData
	textData     *string
	commentData  *string
	documentData *documentData
	docTypeData  *docTypeData

	// Shadow DOM support: back-reference when this node is a ShadowRoot's underlying node
	shadowRoot *ShadowRoot
}

// ElementGeometry holds computed layout geometry for an element.
// This is set during layout computation and used by getBoundingClientRect.
type ElementGeometry struct {
	// Border box coordinates relative to the viewport
	X, Y, Width, Height float64

	// Box model dimensions
	ContentWidth, ContentHeight float64
	PaddingTop, PaddingRight, PaddingBottom, PaddingLeft float64
	BorderTop, BorderRight, BorderBottom, BorderLeft float64
	MarginTop, MarginRight, MarginBottom, MarginLeft float64

	// Offset properties for HTMLElement
	OffsetTop, OffsetLeft float64
	OffsetWidth, OffsetHeight float64
	OffsetParent *Element

	// Scroll properties
	ScrollTop, ScrollLeft float64
	ScrollWidth, ScrollHeight float64
	ClientTop, ClientLeft float64
	ClientWidth, ClientHeight float64
}

// elementData holds data specific to Element nodes.
type elementData struct {
	localName        string
	namespaceURI     string
	prefix           string
	tagName          string
	attributes       *NamedNodeMap
	classList        *DOMTokenList
	styleDeclaration *CSSStyleDeclaration
	id               string
	className        string

	// Layout geometry - set during layout computation
	geometry *ElementGeometry

	// Shadow DOM support: the shadow root attached to this element (if any)
	shadowRoot *ShadowRoot

	// Template contents: for <template> elements, the DocumentFragment that holds the content
	templateContent *DocumentFragment

	// Input element state - for checkbox, radio, and other input types
	inputData *InputData
}

// InputData holds state for HTMLInputElement.
// The "checked" property is separate from the "checked" attribute (defaultChecked).
// Similarly, "value" property is separate from "value" attribute (defaultValue).
type InputData struct {
	// Whether the checked state has been explicitly set (dirty checkedness flag)
	checkedDirty bool
	// The current checked state (for checkbox/radio)
	checked bool
	// Whether the value has been explicitly set (dirty value flag)
	valueDirty bool
	// The current value (for text, etc.)
	value string
}

// documentData holds data specific to Document nodes.
type documentData struct {
	doctype         *Node              // DocumentType node
	documentElement *Node              // root Element
	contentType     string             // The content type (MIME type) of the document
	implementation  *DOMImplementation // The document's DOMImplementation
	url             string             // The document's URL (defaults to "about:blank")
	characterSet    string             // The document's character encoding (defaults to "UTF-8")
	selection       *Selection         // The document's Selection object
}

// docTypeData holds data specific to DocumentType nodes.
type docTypeData struct {
	name     string
	publicId string
	systemId string
}

// newNode creates a new node with the given type and name.
func newNode(nodeType NodeType, nodeName string, ownerDoc *Document) *Node {
	n := &Node{
		nodeType: nodeType,
		nodeName: nodeName,
		ownerDoc: ownerDoc,
	}
	n.childNodes = newNodeList(n)
	return n
}

// NodeType returns the type of the node.
func (n *Node) NodeType() NodeType {
	return n.nodeType
}

// NodeName returns the name of the node.
// For elements, this is the tag name in uppercase.
// For text nodes, this is "#text".
// For comments, this is "#comment".
// For documents, this is "#document".
// For document fragments, this is "#document-fragment".
func (n *Node) NodeName() string {
	return n.nodeName
}

// NodeValue returns the value of the node.
// For text and comment nodes, this is the text content.
// For other nodes, this is nil (represented as empty string in JavaScript).
func (n *Node) NodeValue() string {
	if n.nodeValue != nil {
		return *n.nodeValue
	}
	return ""
}

// SetNodeValue sets the value of the node.
// This only has an effect on text, comment, CDATASection, and processing instruction nodes.
func (n *Node) SetNodeValue(value string) {
	var oldValue string
	switch n.nodeType {
	case TextNode, CDATASectionNode:
		if n.textData != nil {
			oldValue = *n.textData
			*n.textData = value
		}
		n.nodeValue = &value
		notifyCharacterDataMutation(n, oldValue)
	case CommentNode:
		if n.commentData != nil {
			oldValue = *n.commentData
			*n.commentData = value
		}
		n.nodeValue = &value
		notifyCharacterDataMutation(n, oldValue)
	case ProcessingInstructionNode:
		if n.nodeValue != nil {
			oldValue = *n.nodeValue
		}
		n.nodeValue = &value
		notifyCharacterDataMutation(n, oldValue)
	}
	// For other node types, this is a no-op per the spec
}

// OwnerDocument returns the Document that owns this node.
// For Document nodes, this returns nil.
func (n *Node) OwnerDocument() *Document {
	if n.nodeType == DocumentNode {
		return nil
	}
	return n.ownerDoc
}

// BaseURI returns the absolute base URL of the node.
// For nodes in a document, this returns the document's URL.
// For detached nodes, this returns the owner document's URL.
// For Document nodes, this returns the document's own URL.
func (n *Node) BaseURI() string {
	// For Document nodes, return the document's URL directly
	if n.nodeType == DocumentNode && n.documentData != nil {
		url := n.documentData.url
		if url == "" {
			return "about:blank"
		}
		return url
	}

	// For other nodes, get the URL from the owner document
	if n.ownerDoc != nil {
		return n.ownerDoc.URL()
	}

	// Fallback for nodes without an owner document
	return "about:blank"
}

// ParentNode returns the parent of this node.
func (n *Node) ParentNode() *Node {
	return n.parentNode
}

// ParentElement returns the parent Element, or nil if the parent is not an element.
func (n *Node) ParentElement() *Element {
	if n.parentNode != nil && n.parentNode.nodeType == ElementNode {
		return (*Element)(n.parentNode)
	}
	return nil
}

// ChildNodes returns a live NodeList of child nodes.
func (n *Node) ChildNodes() *NodeList {
	return n.childNodes
}

// FirstChild returns the first child node, or nil if there are no children.
func (n *Node) FirstChild() *Node {
	return n.firstChild
}

// LastChild returns the last child node, or nil if there are no children.
func (n *Node) LastChild() *Node {
	return n.lastChild
}

// PreviousSibling returns the previous sibling node, or nil if this is the first child.
func (n *Node) PreviousSibling() *Node {
	return n.prevSibling
}

// NextSibling returns the next sibling node, or nil if this is the last child.
func (n *Node) NextSibling() *Node {
	return n.nextSibling
}

// HasChildNodes returns true if this node has any child nodes.
func (n *Node) HasChildNodes() bool {
	return n.firstChild != nil
}

// IsConnected returns true if the node is connected to a document.
// A node is connected if its shadow-including root is a document.
// Per spec, this means traversing through shadow boundaries to check
// if the node is ultimately connected to a document.
func (n *Node) IsConnected() bool {
	root := n.GetShadowIncludingRoot()
	return root != nil && root.nodeType == DocumentNode
}

// GetShadowIncludingRoot returns the shadow-including root of this node.
// This traverses through shadow boundaries by going from shadow roots
// to their host elements.
func (n *Node) GetShadowIncludingRoot() *Node {
	root := n.GetRootNode()

	// If the root is a shadow root, continue through the host
	for root != nil && root.shadowRoot != nil {
		host := root.shadowRoot.Host()
		if host == nil {
			break
		}
		root = host.AsNode().GetRootNode()
	}

	return root
}

// TextContent returns the text content of the node and its descendants.
func (n *Node) TextContent() string {
	switch n.nodeType {
	case DocumentNode, DocumentTypeNode:
		return ""
	case TextNode, CommentNode, ProcessingInstructionNode, CDATASectionNode:
		return n.NodeValue()
	default:
		var sb strings.Builder
		n.collectTextContent(&sb)
		return sb.String()
	}
}

func (n *Node) collectTextContent(sb *strings.Builder) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		switch child.nodeType {
		case TextNode, CDATASectionNode:
			sb.WriteString(child.NodeValue())
		case ElementNode, DocumentFragmentNode:
			child.collectTextContent(sb)
		}
	}
}

// SetTextContent sets the text content of the node.
// For elements and document fragments, this replaces all children with a single text node.
// Per the DOM spec, this operation generates a single mutation record containing
// all removed nodes and the added text node (if any).
func (n *Node) SetTextContent(value string) {
	switch n.nodeType {
	case DocumentNode, DocumentTypeNode:
		// Do nothing per the spec
		return
	case TextNode, CommentNode, ProcessingInstructionNode, CDATASectionNode:
		n.SetNodeValue(value)
	default:
		// Collect all children to remove
		var removedNodes []*Node
		for child := n.firstChild; child != nil; child = child.nextSibling {
			removedNodes = append(removedNodes, child)
		}

		// Capture sibling info before modifications (there are no previous siblings since we're replacing all)
		var prevSib *Node // nil since we're removing from the beginning
		var nextSib *Node // nil since we're removing everything

		// Remove all children without individual notifications
		for n.firstChild != nil {
			n.removeChildInternal(n.firstChild)
		}

		// Create and add a new text node if value is not empty
		var addedNodes []*Node
		if value != "" {
			textNode := n.ownerDoc.CreateTextNode(value)
			n.insertBeforeInternal(textNode, nil)
			addedNodes = append(addedNodes, textNode)
		}

		// Send a single mutation notification for the entire operation
		if len(removedNodes) > 0 || len(addedNodes) > 0 {
			notifyChildListMutation(n, addedNodes, removedNodes, prevSib, nextSib)
		}
	}
}

// replaceChildrenImpl implements the ParentNode.replaceChildren() algorithm.
// Per DOM spec, this generates a single mutation record for the parent containing
// all removed children and all added nodes. Nodes that are moved from other parents
// generate separate removal records for those parents.
func (n *Node) replaceChildrenImpl(items []interface{}) error {
	doc := n.ownerDoc
	if doc == nil && n.nodeType == DocumentNode {
		doc = (*Document)(n)
	}
	if doc == nil {
		return nil
	}

	// Step 1: Convert items into nodes (creating text nodes for strings)
	var nodesToAdd []*Node
	for _, item := range items {
		switch v := item.(type) {
		case *Node:
			nodesToAdd = append(nodesToAdd, v)
		case *Element:
			nodesToAdd = append(nodesToAdd, v.AsNode())
		case string:
			nodesToAdd = append(nodesToAdd, doc.CreateTextNode(v))
		}
	}

	// Step 2: Validate the insertion BEFORE any mutations
	// Create a temporary fragment if we have multiple nodes to validate
	if len(nodesToAdd) > 0 {
		if len(nodesToAdd) == 1 {
			if err := n.validatePreInsertion(nodesToAdd[0], nil); err != nil {
				return err
			}
		} else {
			// Validate each node individually
			for _, node := range nodesToAdd {
				if err := n.validatePreInsertion(node, nil); err != nil {
					return err
				}
			}
		}
	}

	// Step 3: Remove nodes from their current parents (with individual notifications)
	// This must happen before we collect removedNodes from this parent
	for _, node := range nodesToAdd {
		if node.parentNode != nil && node.parentNode != n {
			// Capture sibling info for the removal notification
			nodePrevSib := node.prevSibling
			nodeNextSib := node.nextSibling
			oldParent := node.parentNode

			// Remove from old parent
			oldParent.removeChildInternal(node)

			// Notify about the removal from the old position
			notifyChildListMutation(oldParent, nil, []*Node{node}, nodePrevSib, nodeNextSib)
		}
	}

	// Step 4: Collect all children to remove from this parent
	var removedNodes []*Node
	for child := n.firstChild; child != nil; child = child.nextSibling {
		// Don't add nodes that are being re-added
		isBeingAdded := false
		for _, node := range nodesToAdd {
			if child == node {
				isBeingAdded = true
				break
			}
		}
		if !isBeingAdded {
			removedNodes = append(removedNodes, child)
		}
	}

	// Step 5: Remove all children from this parent (without notifications)
	for n.firstChild != nil {
		n.removeChildInternal(n.firstChild)
	}

	// Step 6: Insert all new nodes (without notifications)
	for _, node := range nodesToAdd {
		n.insertBeforeInternal(node, nil)
	}

	// Step 7: Send a single mutation notification for this parent
	if len(removedNodes) > 0 || len(nodesToAdd) > 0 {
		notifyChildListMutation(n, nodesToAdd, removedNodes, nil, nil)
	}

	return nil
}

// AppendChild adds a node to the end of the list of children of this node.
// For error-returning version, use AppendChildWithError.
func (n *Node) AppendChild(child *Node) *Node {
	result, _ := n.AppendChildWithError(child)
	return result
}

// AppendChildWithError adds a node to the end of the list of children of this node.
// Returns an error if the operation violates DOM hierarchy constraints.
func (n *Node) AppendChildWithError(child *Node) (*Node, error) {
	return n.InsertBeforeWithError(child, nil)
}

// InsertBefore inserts a node before a reference child node.
// If refChild is nil, the node is appended to the end.
// For error-returning version, use InsertBeforeWithError.
func (n *Node) InsertBefore(newChild, refChild *Node) *Node {
	result, _ := n.InsertBeforeWithError(newChild, refChild)
	return result
}

// InsertBeforeWithError inserts a node before a reference child node.
// If refChild is nil, the node is appended to the end.
// Returns an error if the operation violates DOM hierarchy constraints.
func (n *Node) InsertBeforeWithError(newChild, refChild *Node) (*Node, error) {
	// Validate the insertion according to DOM spec
	if err := n.validatePreInsertion(newChild, refChild); err != nil {
		return nil, err
	}
	return n.insertBefore(newChild, refChild), nil
}

// validatePreInsertion implements the pre-insertion validation steps from the DOM spec.
// https://dom.spec.whatwg.org/#concept-node-pre-insert
func (n *Node) validatePreInsertion(node, child *Node) error {
	return n.validatePreInsertionOrReplace(node, child, false)
}

func (n *Node) validatePreReplace(node, child *Node) error {
	return n.validatePreInsertionOrReplace(node, child, true)
}

func (n *Node) validatePreInsertionOrReplace(node, child *Node, isReplace bool) error {
	// Step 1: If parent is not a Document, DocumentFragment, or Element node, throw HierarchyRequestError
	if !n.canHaveChildren() {
		return ErrHierarchyRequest("The operation would yield an incorrect node tree.")
	}

	// Step 2: If node is a host-including inclusive ancestor of parent, throw HierarchyRequestError
	if n.isInclusiveAncestor(node) {
		return ErrHierarchyRequest("The new child element contains the parent.")
	}

	// Step 3: If child is non-null and its parent is not parent, throw NotFoundError
	if child != nil && child.parentNode != n {
		return ErrNotFound("The node before which the new node is to be inserted is not a child of this node.")
	}

	// Step 4: If node is not a DocumentFragment, DocumentType, Element, Text, ProcessingInstruction, or Comment node
	if !n.isValidChildType(node) {
		return ErrHierarchyRequest("The operation would yield an incorrect node tree.")
	}

	// Step 5: If node is a Text node and parent is a document, or node is a doctype and parent is not a document
	if node.nodeType == TextNode && n.nodeType == DocumentNode {
		return ErrHierarchyRequest("Cannot insert Text node as a direct child of Document.")
	}
	if node.nodeType == DocumentTypeNode && n.nodeType != DocumentNode {
		return ErrHierarchyRequest("DocumentType nodes can only be children of Document.")
	}

	// Step 6: If parent is a document, special validation for document children
	if n.nodeType == DocumentNode {
		if err := n.validateDocumentInsertionOrReplace(node, child, isReplace); err != nil {
			return err
		}
	}

	return nil
}

// canHaveChildren returns true if this node can have child nodes.
func (n *Node) canHaveChildren() bool {
	switch n.nodeType {
	case DocumentNode, DocumentFragmentNode, ElementNode:
		return true
	default:
		return false
	}
}

// isInclusiveAncestor returns true if node is this node or an ancestor of this node.
func (n *Node) isInclusiveAncestor(node *Node) bool {
	if node == nil {
		return false
	}
	for current := n; current != nil; current = current.parentNode {
		if current == node {
			return true
		}
	}
	return false
}

// isValidChildType returns true if node is a valid type for children.
func (n *Node) isValidChildType(node *Node) bool {
	if node == nil {
		return false
	}
	switch node.nodeType {
	case DocumentFragmentNode, DocumentTypeNode, ElementNode, TextNode,
		ProcessingInstructionNode, CommentNode, CDATASectionNode:
		return true
	default:
		// Document nodes and other types cannot be children
		return false
	}
}

// validateDocumentInsertion performs additional validation for inserting into a Document node.
func (n *Node) validateDocumentInsertion(node, child *Node) error {
	return n.validateDocumentInsertionOrReplace(node, child, false)
}

// validateDocumentInsertionOrReplace performs validation for inserting into a Document node.
// The child parameter is the reference child for insertBefore, or the child being replaced for replaceChild.
// When isReplace is true, we exclude child from counts since it will be replaced.
func (n *Node) validateDocumentInsertionOrReplace(node, child *Node, isReplace bool) error {
	// Determine which node to exclude from counts (only exclude when replacing)
	var exclude *Node
	if isReplace {
		exclude = child
	}

	switch node.nodeType {
	case DocumentFragmentNode:
		// Count element children in the fragment
		elementCount := 0
		hasText := false
		for c := node.firstChild; c != nil; c = c.nextSibling {
			if c.nodeType == ElementNode {
				elementCount++
			}
			if c.nodeType == TextNode {
				hasText = true
			}
		}

		// A document fragment with text nodes cannot be inserted
		if hasText {
			return ErrHierarchyRequest("Cannot insert Text node as a direct child of Document.")
		}

		// A document fragment with more than one element cannot be inserted
		if elementCount > 1 {
			return ErrHierarchyRequest("Document can have only one element child.")
		}

		// If the fragment has an element, check if document already has one (excluding child if replacing)
		// and also check doctype positioning
		if elementCount == 1 {
			if n.hasElementChildExcluding(exclude) {
				return ErrHierarchyRequest("Document already has a document element.")
			}
			// Check if a doctype follows the reference child
			// When replacing an element, we don't need to check this
			if child != nil && !(isReplace && child.nodeType == ElementNode) {
				if child.nodeType == DocumentTypeNode || n.doctypeFollows(child) {
					return ErrHierarchyRequest("Cannot insert element before doctype.")
				}
			}
		}

	case ElementNode:
		// Document can only have one element child (excluding child if replacing)
		if n.hasElementChildExcluding(exclude) {
			return ErrHierarchyRequest("Document already has a document element.")
		}
		// Check if a doctype follows the reference child
		// When replacing an element, we don't need to check this
		if child != nil && !(isReplace && child.nodeType == ElementNode) {
			if child.nodeType == DocumentTypeNode || n.doctypeFollows(child) {
				return ErrHierarchyRequest("Cannot insert element before doctype.")
			}
		}

	case DocumentTypeNode:
		// Document can only have one doctype (excluding child if replacing)
		if n.hasDoctypeExcluding(exclude) {
			return ErrHierarchyRequest("Document already has a doctype.")
		}
		// Doctype cannot be inserted after an element (excluding child if it's an element being replaced)
		if n.hasElementChildExcluding(exclude) {
			// Check if child is null (append) or if element precedes child
			if child == nil || n.elementPrecedesExcluding(child, exclude) {
				return ErrHierarchyRequest("Cannot insert doctype after document element.")
			}
		}
	}

	return nil
}

// hasElementChild returns true if this node has an element child.
func (n *Node) hasElementChild() bool {
	return n.hasElementChildExcluding(nil)
}

// hasElementChildExcluding returns true if this node has an element child other than exclude.
func (n *Node) hasElementChildExcluding(exclude *Node) bool {
	for c := n.firstChild; c != nil; c = c.nextSibling {
		if c != exclude && c.nodeType == ElementNode {
			return true
		}
	}
	return false
}

// hasDoctype returns true if this document has a doctype child.
func (n *Node) hasDoctype() bool {
	return n.hasDoctypeExcluding(nil)
}

// hasDoctypeExcluding returns true if this document has a doctype child other than exclude.
func (n *Node) hasDoctypeExcluding(exclude *Node) bool {
	for c := n.firstChild; c != nil; c = c.nextSibling {
		if c != exclude && c.nodeType == DocumentTypeNode {
			return true
		}
	}
	return false
}

// doctypeFollows returns true if there is a doctype node following the given child.
func (n *Node) doctypeFollows(child *Node) bool {
	for c := child.nextSibling; c != nil; c = c.nextSibling {
		if c.nodeType == DocumentTypeNode {
			return true
		}
	}
	return false
}

// elementPrecedes returns true if there is an element node preceding the given child (or if child is nil, anywhere).
func (n *Node) elementPrecedes(child *Node) bool {
	return n.elementPrecedesExcluding(child, nil)
}

// elementPrecedesExcluding returns true if there is an element node preceding the given child,
// excluding the specified node from consideration.
func (n *Node) elementPrecedesExcluding(child, exclude *Node) bool {
	for c := n.firstChild; c != nil && c != child; c = c.nextSibling {
		if c != exclude && c.nodeType == ElementNode {
			return true
		}
	}
	return false
}

func (n *Node) insertBefore(newChild, refChild *Node) *Node {
	if newChild == nil {
		return nil
	}

	// If newChild is a DocumentFragment, insert all its children
	if newChild.nodeType == DocumentFragmentNode {
		// Collect children first to avoid modifying during iteration
		var children []*Node
		for child := newChild.firstChild; child != nil; child = child.nextSibling {
			children = append(children, child)
		}

		// Get sibling info for mutation notification before any insertions
		var prevSib *Node
		if refChild != nil {
			prevSib = refChild.prevSibling
		} else {
			prevSib = n.lastChild
		}

		// Insert all children without individual notifications
		for _, child := range children {
			n.insertBeforeNoNotify(child, refChild)
		}

		// Send a single mutation notification for all children
		if len(children) > 0 {
			notifyChildListMutation(n, children, nil, prevSib, refChild)
		}
		return newChild
	}

	// If inserting a node before itself, return early (no-op)
	if newChild == refChild {
		return newChild
	}

	// Get sibling info before any modifications for mutation notification
	var prevSib *Node
	if refChild != nil {
		prevSib = refChild.prevSibling
	} else {
		prevSib = n.lastChild
	}

	// Remove from current parent if necessary (this will trigger its own mutation notification)
	if newChild.parentNode != nil {
		newChild.parentNode.RemoveChild(newChild)
	}

	// Set the new parent
	newChild.parentNode = n

	// Adopt the node to this document if needed
	if n.ownerDoc != nil && newChild.ownerDoc != n.ownerDoc {
		adoptNode(newChild, n.ownerDoc)
	} else if n.nodeType == DocumentNode {
		// If parent is a Document, set ownerDoc to the document itself
		doc := (*Document)(n)
		adoptNode(newChild, doc)
	}

	if refChild == nil {
		// Append to the end
		newChild.prevSibling = n.lastChild
		newChild.nextSibling = nil
		if n.lastChild != nil {
			n.lastChild.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		n.lastChild = newChild
	} else {
		// Insert before refChild
		newChild.prevSibling = refChild.prevSibling
		newChild.nextSibling = refChild
		if refChild.prevSibling != nil {
			refChild.prevSibling.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		refChild.prevSibling = newChild
	}

	// Notify about the insertion
	notifyChildListMutation(n, []*Node{newChild}, nil, prevSib, refChild)

	return newChild
}

// insertBeforeNoNotify inserts a node without triggering mutation notifications.
// Used for batch operations like DocumentFragment insertion.
func (n *Node) insertBeforeNoNotify(newChild, refChild *Node) {
	if newChild == nil {
		return
	}

	// Remove from current parent if necessary (without notification)
	if newChild.parentNode != nil {
		newChild.parentNode.removeChildInternal(newChild)
	}

	// Set the new parent
	newChild.parentNode = n

	// Adopt the node to this document if needed
	if n.ownerDoc != nil && newChild.ownerDoc != n.ownerDoc {
		adoptNode(newChild, n.ownerDoc)
	} else if n.nodeType == DocumentNode {
		doc := (*Document)(n)
		adoptNode(newChild, doc)
	}

	if refChild == nil {
		// Append to the end
		newChild.prevSibling = n.lastChild
		newChild.nextSibling = nil
		if n.lastChild != nil {
			n.lastChild.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		n.lastChild = newChild
	} else {
		// Insert before refChild
		newChild.prevSibling = refChild.prevSibling
		newChild.nextSibling = refChild
		if refChild.prevSibling != nil {
			refChild.prevSibling.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		refChild.prevSibling = newChild
	}
}

// adoptNode recursively sets the ownerDocument for a node and its descendants.
func adoptNode(node *Node, doc *Document) {
	node.ownerDoc = doc
	for child := node.firstChild; child != nil; child = child.nextSibling {
		adoptNode(child, doc)
	}
}

// RemoveChild removes a child node from this node.
// For error-returning version, use RemoveChildWithError.
func (n *Node) RemoveChild(child *Node) *Node {
	result, _ := n.RemoveChildWithError(child)
	return result
}

// RemoveChildWithError removes a child node from this node.
// Returns an error if the child is not a child of this node.
func (n *Node) RemoveChildWithError(child *Node) (*Node, error) {
	if child == nil {
		return nil, ErrNotFound("The node to be removed is null.")
	}
	if child.parentNode != n {
		return nil, ErrNotFound("The node to be removed is not a child of this node.")
	}

	// Capture sibling info before removal for mutation notification
	prevSib := child.prevSibling
	nextSib := child.nextSibling

	n.removeChildInternal(child)

	// Notify about the removal
	notifyChildListMutation(n, nil, []*Node{child}, prevSib, nextSib)

	return child, nil
}

// removeChildInternal removes a child from this node's children list.
// This is the internal implementation that does not check if child is actually a child.
func (n *Node) removeChildInternal(child *Node) {
	// Update sibling pointers
	if child.prevSibling != nil {
		child.prevSibling.nextSibling = child.nextSibling
	} else {
		n.firstChild = child.nextSibling
	}

	if child.nextSibling != nil {
		child.nextSibling.prevSibling = child.prevSibling
	} else {
		n.lastChild = child.prevSibling
	}

	// Clear the removed node's pointers
	child.parentNode = nil
	child.prevSibling = nil
	child.nextSibling = nil
}

// insertBeforeInternal inserts a node before a reference child without validation.
// If refChild is nil, appends to the end.
func (n *Node) insertBeforeInternal(newChild, refChild *Node) {
	if newChild == nil {
		return
	}

	// Set the new parent
	newChild.parentNode = n

	// Adopt the node to this document if needed
	if n.ownerDoc != nil && newChild.ownerDoc != n.ownerDoc {
		adoptNode(newChild, n.ownerDoc)
	} else if n.nodeType == DocumentNode {
		doc := (*Document)(n)
		adoptNode(newChild, doc)
	}

	if refChild == nil {
		// Append to the end
		newChild.prevSibling = n.lastChild
		newChild.nextSibling = nil
		if n.lastChild != nil {
			n.lastChild.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		n.lastChild = newChild
	} else {
		// Insert before refChild
		newChild.prevSibling = refChild.prevSibling
		newChild.nextSibling = refChild
		if refChild.prevSibling != nil {
			refChild.prevSibling.nextSibling = newChild
		} else {
			n.firstChild = newChild
		}
		refChild.prevSibling = newChild
	}
}

// ReplaceChild replaces a child node with a new node.
// For error-returning version, use ReplaceChildWithError.
func (n *Node) ReplaceChild(newChild, oldChild *Node) *Node {
	result, _ := n.ReplaceChildWithError(newChild, oldChild)
	return result
}

// ReplaceChildWithError replaces a child node with a new node.
// Returns an error if the operation violates DOM hierarchy constraints.
func (n *Node) ReplaceChildWithError(newChild, oldChild *Node) (*Node, error) {
	if oldChild == nil {
		return nil, ErrNotFound("The node to be replaced is null.")
	}

	// Validate the replacement following the DOM spec order:
	// 1. Check if parent is a valid parent node type
	// 2. Check if node is an ancestor of parent
	// 3. Check if child is a child of parent
	// 4-6. Other validation checks (excluding oldChild from element/doctype counts)
	if err := n.validatePreReplace(newChild, oldChild); err != nil {
		return nil, err
	}

	// If replacing a node with itself, do nothing (just return the node)
	if newChild == oldChild {
		return oldChild, nil
	}

	// Get the next sibling of oldChild before any tree modifications
	referenceChild := oldChild.nextSibling

	// Handle the case where newChild is the next sibling of oldChild
	// After removing newChild, referenceChild would become invalid
	if referenceChild == newChild {
		referenceChild = newChild.nextSibling
	}

	// Handle DocumentFragment: insert all its children
	if newChild.nodeType == DocumentFragmentNode {
		// Capture sibling info before any modifications for mutation notification
		prevSib := oldChild.prevSibling
		nextSib := oldChild.nextSibling

		// Collect all children first
		var children []*Node
		for child := newChild.firstChild; child != nil; child = child.nextSibling {
			children = append(children, child)
		}

		// Remove the old child
		n.removeChildInternal(oldChild)

		// Insert each child from the fragment at the position
		for _, child := range children {
			n.insertBeforeInternal(child, referenceChild)
		}

		// Notify about the replacement (removed oldChild, added all fragment children)
		notifyChildListMutation(n, children, []*Node{oldChild}, prevSib, nextSib)

		return oldChild, nil
	}

	// For non-DocumentFragment nodes:
	// If newChild is already in the tree, we need to remove it first.
	// Per DOM spec, if the newChild has a parent, we generate a separate
	// mutation record for its removal from the original position.
	if newChild.parentNode != nil {
		// Capture sibling info for the removal notification
		newChildPrevSib := newChild.prevSibling
		newChildNextSib := newChild.nextSibling
		oldParent := newChild.parentNode

		// Remove newChild from its current position
		oldParent.removeChildInternal(newChild)

		// Notify about the removal from the old position
		notifyChildListMutation(oldParent, nil, []*Node{newChild}, newChildPrevSib, newChildNextSib)
	}

	// Capture sibling info for the replacement notification AFTER removing newChild.
	// This ensures we capture the correct siblings reflecting the tree state
	// after the source removal but before the target replacement.
	prevSib := oldChild.prevSibling
	nextSib := oldChild.nextSibling

	// Remove the old child from its parent
	n.removeChildInternal(oldChild)

	// Insert newChild at oldChild's position
	n.insertBeforeInternal(newChild, referenceChild)

	// Notify about the replacement
	notifyChildListMutation(n, []*Node{newChild}, []*Node{oldChild}, prevSib, nextSib)

	return oldChild, nil
}

// CloneNode creates a copy of this node.
// If deep is true, all descendants are also cloned.
func (n *Node) CloneNode(deep bool) *Node {
	clone := n.shallowClone()

	if deep {
		for child := n.firstChild; child != nil; child = child.nextSibling {
			childClone := child.CloneNode(true)
			clone.AppendChild(childClone)
		}
	}

	return clone
}

func (n *Node) shallowClone() *Node {
	clone := newNode(n.nodeType, n.nodeName, n.ownerDoc)

	if n.nodeValue != nil {
		value := *n.nodeValue
		clone.nodeValue = &value
	}

	switch n.nodeType {
	case ElementNode:
		if n.elementData != nil {
			clone.elementData = &elementData{
				localName:    n.elementData.localName,
				namespaceURI: n.elementData.namespaceURI,
				prefix:       n.elementData.prefix,
				tagName:      n.elementData.tagName,
				id:           n.elementData.id,
				className:    n.elementData.className,
			}
			// Clone attributes
			clone.elementData.attributes = newNamedNodeMap((*Element)(clone))
			if n.elementData.attributes != nil {
				for i := 0; i < n.elementData.attributes.Length(); i++ {
					attr := n.elementData.attributes.Item(i)
					if attr != nil {
						clone.elementData.attributes.SetNamedItem(attr.CloneNode(false))
					}
				}
			}
		}
	case TextNode, CDATASectionNode:
		if n.textData != nil {
			text := *n.textData
			clone.textData = &text
		}
	case CommentNode:
		if n.commentData != nil {
			comment := *n.commentData
			clone.commentData = &comment
		}
	case DocumentTypeNode:
		if n.docTypeData != nil {
			clone.docTypeData = &docTypeData{
				name:     n.docTypeData.name,
				publicId: n.docTypeData.publicId,
				systemId: n.docTypeData.systemId,
			}
		}
	case DocumentNode:
		if n.documentData != nil {
			clone.documentData = &documentData{
				contentType: n.documentData.contentType,
				// doctype, documentElement are tracked via children
				// implementation is created lazily when accessed
			}
		} else {
			// Ensure documentData is always initialized for Document nodes
			clone.documentData = &documentData{
				contentType: "text/html",
			}
		}
		// Set ownerDoc to point to itself for Document nodes
		clone.ownerDoc = (*Document)(clone)
	}

	return clone
}

// Normalize merges adjacent text nodes and removes empty text nodes.
func (n *Node) Normalize() {
	var nodesToRemove []*Node

	for child := n.firstChild; child != nil; {
		next := child.nextSibling

		if child.nodeType == TextNode {
			// Remove empty text nodes
			if child.NodeValue() == "" {
				nodesToRemove = append(nodesToRemove, child)
			} else {
				// Merge adjacent text nodes
				for next != nil && next.nodeType == TextNode {
					child.SetNodeValue(child.NodeValue() + next.NodeValue())
					nodesToRemove = append(nodesToRemove, next)
					next = next.nextSibling
				}
			}
		} else if child.nodeType == ElementNode {
			// Recursively normalize children
			child.Normalize()
		}

		child = next
	}

	for _, node := range nodesToRemove {
		n.RemoveChild(node)
	}
}

// Contains returns true if the given node is a descendant of this node.
func (n *Node) Contains(other *Node) bool {
	if other == nil {
		return false
	}
	if other == n {
		return true
	}
	for node := other.parentNode; node != nil; node = node.parentNode {
		if node == n {
			return true
		}
	}
	return false
}

// GetRootNode returns the root of the tree containing this node.
func (n *Node) GetRootNode() *Node {
	root := n
	for root.parentNode != nil {
		root = root.parentNode
	}
	return root
}

// GetRootNodeWithOptions returns the root of the tree containing this node.
// If composed is true, it returns the shadow-including root (traverses shadow boundaries).
// If composed is false (default), it returns the normal root (which may be a ShadowRoot).
func (n *Node) GetRootNodeWithOptions(composed bool) *Node {
	if composed {
		return n.GetShadowIncludingRoot()
	}
	return n.GetRootNode()
}

// IsShadowRoot returns true if this node is the underlying node of a ShadowRoot.
func (n *Node) IsShadowRoot() bool {
	return n.shadowRoot != nil
}

// GetShadowRoot returns the ShadowRoot if this node is its underlying node, or nil.
func (n *Node) GetShadowRoot() *ShadowRoot {
	return n.shadowRoot
}

// CompareDocumentPosition returns a bitmask indicating the position of the given node relative to this node.
func (n *Node) CompareDocumentPosition(other *Node) uint16 {
	const (
		DocumentPositionDisconnected           = 0x01
		DocumentPositionPreceding              = 0x02
		DocumentPositionFollowing              = 0x04
		DocumentPositionContains               = 0x08
		DocumentPositionContainedBy            = 0x10
		DocumentPositionImplementationSpecific = 0x20
	)

	if n == other {
		return 0
	}

	if other == nil {
		return DocumentPositionDisconnected | DocumentPositionImplementationSpecific
	}

	// Check if they're in the same tree
	root1 := n.GetRootNode()
	root2 := other.GetRootNode()
	if root1 != root2 {
		// For disconnected nodes, we need to provide a consistent ordering.
		// Use pointer comparison for consistency.
		if uintptr(unsafePointer(n)) < uintptr(unsafePointer(other)) {
			return DocumentPositionDisconnected | DocumentPositionImplementationSpecific | DocumentPositionFollowing
		}
		return DocumentPositionDisconnected | DocumentPositionImplementationSpecific | DocumentPositionPreceding
	}

	// Check containment
	if n.Contains(other) {
		return DocumentPositionContainedBy | DocumentPositionFollowing
	}
	if other.Contains(n) {
		return DocumentPositionContains | DocumentPositionPreceding
	}

	// Neither node contains the other, so we need to find their relative
	// position in tree order (pre-order depth-first traversal).
	// Find the common ancestor and compare positions.
	if n.precedesInTreeOrder(other) {
		return DocumentPositionFollowing
	}
	return DocumentPositionPreceding
}

// precedesInTreeOrder returns true if n comes before other in tree order
// (pre-order depth-first traversal). This assumes both nodes are in the
// same tree and neither contains the other.
func (n *Node) precedesInTreeOrder(other *Node) bool {
	// Build the ancestor chains for both nodes
	var nAncestors []*Node
	for node := n; node != nil; node = node.parentNode {
		nAncestors = append(nAncestors, node)
	}

	var otherAncestors []*Node
	for node := other; node != nil; node = node.parentNode {
		otherAncestors = append(otherAncestors, node)
	}

	// Find the common ancestor by walking from the roots
	// The chains are in child-to-root order, so reverse iterate
	nLen := len(nAncestors)
	otherLen := len(otherAncestors)

	// Find where the ancestor chains diverge
	minLen := nLen
	if otherLen < minLen {
		minLen = otherLen
	}

	divergePoint := 0
	for i := 0; i < minLen; i++ {
		nIdx := nLen - 1 - i
		otherIdx := otherLen - 1 - i
		if nAncestors[nIdx] != otherAncestors[otherIdx] {
			break
		}
		divergePoint = i + 1
	}

	// Get the children of the common ancestor that lead to each node
	var nChild, otherChild *Node
	if divergePoint < nLen {
		nChild = nAncestors[nLen-1-divergePoint]
	}
	if divergePoint < otherLen {
		otherChild = otherAncestors[otherLen-1-divergePoint]
	}

	// Compare sibling order
	// Find which child comes first among the common ancestor's children
	commonAncestor := nAncestors[nLen-divergePoint]
	for child := commonAncestor.firstChild; child != nil; child = child.nextSibling {
		if child == nChild {
			return true // n's branch comes first
		}
		if child == otherChild {
			return false // other's branch comes first
		}
	}

	// This shouldn't happen if both nodes are in the same tree
	return false
}

// IsSameNode returns true if this node is the same node as the given node.
func (n *Node) IsSameNode(other *Node) bool {
	return n == other
}

// IsEqualNode returns true if this node is equal to the given node.
// Per DOM spec, equality is based on node type and type-specific properties:
// - Element: namespace, namespace prefix, local name, attributes
// - DocumentType: name, public ID, system ID
// - ProcessingInstruction: target, data
// - Text/Comment: data
func (n *Node) IsEqualNode(other *Node) bool {
	if other == nil {
		return false
	}
	if n.nodeType != other.nodeType {
		return false
	}

	// Type-specific comparison
	switch n.nodeType {
	case ElementNode:
		if !n.elementsEqual(other) {
			return false
		}
	case DocumentTypeNode:
		if !n.doctypesEqual(other) {
			return false
		}
	case ProcessingInstructionNode:
		// Compare target (nodeName) and data (nodeValue)
		if n.nodeName != other.nodeName {
			return false
		}
		if n.NodeValue() != other.NodeValue() {
			return false
		}
	case TextNode, CDATASectionNode, CommentNode:
		// Compare data (nodeValue)
		if n.NodeValue() != other.NodeValue() {
			return false
		}
	case DocumentNode, DocumentFragmentNode:
		// Documents and DocumentFragments compare only on children
		// (no additional properties to compare)
	}

	// Compare children count
	count1, count2 := 0, 0
	for c := n.firstChild; c != nil; c = c.nextSibling {
		count1++
	}
	for c := other.firstChild; c != nil; c = c.nextSibling {
		count2++
	}
	if count1 != count2 {
		return false
	}

	// Compare children recursively
	c1, c2 := n.firstChild, other.firstChild
	for c1 != nil && c2 != nil {
		if !c1.IsEqualNode(c2) {
			return false
		}
		c1, c2 = c1.nextSibling, c2.nextSibling
	}

	return true
}

// elementsEqual compares two Element nodes for equality per DOM spec.
// Elements are compared on namespace, namespace prefix, local name, and attributes.
func (n *Node) elementsEqual(other *Node) bool {
	e1 := n.elementData
	e2 := other.elementData
	if e1 == nil || e2 == nil {
		return e1 == e2
	}

	// Compare namespace URI
	if e1.namespaceURI != e2.namespaceURI {
		return false
	}
	// Compare prefix
	if e1.prefix != e2.prefix {
		return false
	}
	// Compare local name
	if e1.localName != e2.localName {
		return false
	}

	// Compare number of attributes
	count1, count2 := 0, 0
	if e1.attributes != nil {
		count1 = e1.attributes.Length()
	}
	if e2.attributes != nil {
		count2 = e2.attributes.Length()
	}
	if count1 != count2 {
		return false
	}

	// Compare each attribute: for each attr in e1, find matching attr in e2
	// Attributes match on namespace URI, local name, and value (NOT prefix)
	if e1.attributes != nil {
		for i := 0; i < e1.attributes.Length(); i++ {
			attr1 := e1.attributes.Item(i)
			if attr1 == nil {
				continue
			}
			// Find matching attribute in e2 by namespace URI and local name
			var attr2 *Attr
			if e2.attributes != nil {
				attr2 = e2.attributes.GetNamedItemNS(attr1.NamespaceURI(), attr1.LocalName())
			}
			if attr2 == nil {
				return false
			}
			// Compare values
			if attr1.Value() != attr2.Value() {
				return false
			}
		}
	}

	return true
}

// doctypesEqual compares two DocumentType nodes for equality.
// Doctypes are compared on name, public ID, and system ID.
func (n *Node) doctypesEqual(other *Node) bool {
	d1 := n.docTypeData
	d2 := other.docTypeData
	if d1 == nil || d2 == nil {
		return d1 == d2
	}

	if d1.name != d2.name {
		return false
	}
	if d1.publicId != d2.publicId {
		return false
	}
	if d1.systemId != d2.systemId {
		return false
	}
	return true
}

// LookupPrefix returns the namespace prefix for the given namespace URI, if any.
func (n *Node) LookupPrefix(namespaceURI string) string {
	if namespaceURI == "" {
		return ""
	}
	return n.lookupPrefix(namespaceURI)
}

func (n *Node) lookupPrefix(namespaceURI string) string {
	switch n.nodeType {
	case ElementNode:
		if n.elementData != nil && n.elementData.namespaceURI == namespaceURI {
			if n.elementData.prefix != "" {
				return n.elementData.prefix
			}
		}
		// Check attributes for xmlns:prefix declarations
		if n.elementData != nil && n.elementData.attributes != nil {
			for i := 0; i < n.elementData.attributes.Length(); i++ {
				attr := n.elementData.attributes.Item(i)
				if attr != nil && strings.HasPrefix(attr.Name(), "xmlns:") {
					if attr.Value() == namespaceURI {
						return strings.TrimPrefix(attr.Name(), "xmlns:")
					}
				}
			}
		}
	}
	if n.parentNode != nil && n.parentNode.nodeType == ElementNode {
		return n.parentNode.lookupPrefix(namespaceURI)
	}
	return ""
}

// LookupNamespaceURI returns the namespace URI for the given prefix.
func (n *Node) LookupNamespaceURI(prefix string) string {
	return n.lookupNamespaceURI(prefix)
}

func (n *Node) lookupNamespaceURI(prefix string) string {
	switch n.nodeType {
	case DocumentNode:
		// Document delegates to its document element (first child element)
		for child := n.firstChild; child != nil; child = child.nextSibling {
			if child.nodeType == ElementNode {
				return child.lookupNamespaceURI(prefix)
			}
		}
		return ""

	case ElementNode:
		// Handle special prefixes only for Element nodes (per DOM spec)
		// These are always available when you reach an Element context
		if prefix == "xml" {
			return "http://www.w3.org/XML/1998/namespace"
		}
		if prefix == "xmlns" {
			return "http://www.w3.org/2000/xmlns/"
		}

		if n.elementData != nil {
			// Check if the element's namespace matches the prefix
			if n.elementData.prefix == prefix && n.elementData.namespaceURI != "" {
				return n.elementData.namespaceURI
			}
			// Check xmlns attributes
			if n.elementData.attributes != nil {
				attrName := "xmlns"
				if prefix != "" {
					attrName = "xmlns:" + prefix
				}
				attr := n.elementData.attributes.GetNamedItem(attrName)
				if attr != nil {
					return attr.Value()
				}
			}
		}
		// Element continues to parent for further lookup,
		// but only if parent is another Element (not Document, DocumentFragment, etc.)
		if n.parentNode != nil && n.parentNode.nodeType == ElementNode {
			return n.parentNode.lookupNamespaceURI(prefix)
		}
		return ""

	case DocumentTypeNode, DocumentFragmentNode:
		// These nodes cannot have namespaces, return empty
		return ""
	}

	// For other nodes (Text, Comment, etc.), delegate to parent Element only
	// If the parent is Document, we don't inherit namespace from document's element
	if n.parentNode != nil && n.parentNode.nodeType == ElementNode {
		return n.parentNode.lookupNamespaceURI(prefix)
	}
	return ""
}

// IsDefaultNamespace returns true if the given namespace URI is the default namespace.
func (n *Node) IsDefaultNamespace(namespaceURI string) bool {
	defaultNS := n.LookupNamespaceURI("")
	return defaultNS == namespaceURI
}

// convertNodesToFragment converts a list of nodes and strings into a DocumentFragment.
// This implements the "converting nodes into a node" algorithm from the DOM spec.
// If there's only one node and no strings, it returns that node directly.
// Otherwise, it creates a DocumentFragment containing all nodes/strings.
func (n *Node) convertNodesToFragment(items []interface{}) *Node {
	doc := n.ownerDoc
	if doc == nil {
		return nil
	}

	// Count actual nodes
	nodes := make([]*Node, 0, len(items))
	for _, item := range items {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = doc.CreateTextNode(v)
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	// Create a DocumentFragment and append all nodes
	frag := doc.CreateDocumentFragment()
	fragNode := (*Node)(frag)
	for _, node := range nodes {
		fragNode.AppendChild(node)
	}
	return fragNode
}

// findViablePreviousSibling finds the first preceding sibling not in the nodes set.
// This implements step 3 of the "before" algorithm.
func (n *Node) findViablePreviousSibling(nodeSet map[*Node]bool) *Node {
	for sibling := n.prevSibling; sibling != nil; sibling = sibling.prevSibling {
		if !nodeSet[sibling] {
			return sibling
		}
	}
	return nil
}

// findViableNextSibling finds the first following sibling not in the nodes set.
// This implements step 3 of the "after" algorithm.
func (n *Node) findViableNextSibling(nodeSet map[*Node]bool) *Node {
	for sibling := n.nextSibling; sibling != nil; sibling = sibling.nextSibling {
		if !nodeSet[sibling] {
			return sibling
		}
	}
	return nil
}

// extractNodeSet builds a set of DOM nodes from the items slice.
func extractNodeSet(items []interface{}) map[*Node]bool {
	result := make(map[*Node]bool)
	for _, item := range items {
		switch v := item.(type) {
		case *Node:
			result[v] = true
		case *Element:
			result[v.AsNode()] = true
		}
	}
	return result
}

// DocumentType accessor methods

// DoctypeName returns the name of a DocumentType node, or empty string for other node types.
func (n *Node) DoctypeName() string {
	if n.nodeType == DocumentTypeNode && n.docTypeData != nil {
		return n.docTypeData.name
	}
	return ""
}

// DoctypePublicId returns the publicId of a DocumentType node, or empty string for other node types.
func (n *Node) DoctypePublicId() string {
	if n.nodeType == DocumentTypeNode && n.docTypeData != nil {
		return n.docTypeData.publicId
	}
	return ""
}

// DoctypeSystemId returns the systemId of a DocumentType node, or empty string for other node types.
func (n *Node) DoctypeSystemId() string {
	if n.nodeType == DocumentTypeNode && n.docTypeData != nil {
		return n.docTypeData.systemId
	}
	return ""
}

// MoveBefore atomically moves a node to a new position in the DOM tree while preserving state.
// Unlike InsertBefore, this is specifically for moving nodes that are already in a tree.
// Returns an error if the operation violates DOM hierarchy constraints.
//
// Pre-move validity checks (different from pre-insertion):
// 1. Both parent and node must be connected, or share the same shadow-including root
// 2. Parent must be a Document, DocumentFragment, or Element
// 3. Node must not be a host-including inclusive ancestor of parent
// 4. Node must be an Element or CharacterData node
// 5. If child is non-null, its parent must be parent
func (n *Node) MoveBefore(node, child *Node) error {
	// Validate the move according to the moveBefore spec
	if err := n.validatePreMove(node, child); err != nil {
		return err
	}

	// If moving a node before itself, no-op
	if node == child {
		return nil
	}

	// Capture old parent and siblings for mutation notification
	oldParent := node.parentNode
	oldPrevSib := node.prevSibling
	oldNextSib := node.nextSibling

	// Get sibling info for the insertion point before any modifications
	var newPrevSib *Node
	if child != nil {
		newPrevSib = child.prevSibling
	} else {
		newPrevSib = n.lastChild
	}

	// Remove from old parent (without triggering mutation notification yet)
	if oldParent != nil {
		oldParent.removeChildInternal(node)
	}

	// Insert at new position (without triggering mutation notification yet)
	n.insertBeforeInternal(node, child)

	// Notify about the move: removal from old parent, then insertion to new parent
	// Per spec, moveBefore generates both a removal and insertion MutationRecord
	if oldParent != nil {
		notifyChildListMutation(oldParent, nil, []*Node{node}, oldPrevSib, oldNextSib)
	}
	notifyChildListMutation(n, []*Node{node}, nil, newPrevSib, child)

	return nil
}

// validatePreMove implements the pre-move validation steps from the DOM spec.
// https://whatpr.org/dom/1307.html#concept-node-ensure-pre-move-validity
func (n *Node) validatePreMove(node, child *Node) error {
	// Step 1: If either parent or node are not connected, then check if they share
	// the same shadow-including root
	parentRoot := n.GetShadowIncludingRoot()
	nodeRoot := node.GetShadowIncludingRoot()

	// If they're in different trees (disconnected from each other), it's an error
	// Note: Two nodes in the same disconnected tree are allowed
	if parentRoot != nodeRoot {
		return ErrHierarchyRequest("The operation would yield an incorrect node tree.")
	}

	// Step 2: Parent's shadow-including root must be same as node's shadow-including root
	// (Already checked above since we require same tree)

	// Step 3: If parent is not a Document, DocumentFragment, or Element node
	if !n.canHaveChildren() {
		return ErrHierarchyRequest("The operation would yield an incorrect node tree.")
	}

	// Step 4: If node is a host-including inclusive ancestor of parent
	if n.isInclusiveAncestor(node) {
		return ErrHierarchyRequest("The new child element contains the parent.")
	}

	// Step 5: If node is not an Element or CharacterData node
	if !isElementOrCharacterData(node) {
		return ErrHierarchyRequest("The operation would yield an incorrect node tree.")
	}

	// Step 6: If child is non-null and its parent is not parent
	if child != nil && child.parentNode != n {
		return ErrNotFound("The node before which the new node is to be inserted is not a child of this node.")
	}

	// Additional validation for Document parent - reuse the pre-insertion validation logic
	// since moveBefore must follow the same Document child constraints as insertBefore
	if n.nodeType == DocumentNode {
		// For moveBefore into a Document, we need to exclude the node being moved
		// from element/doctype counts if it's currently a child of this document
		var exclude *Node
		if node.parentNode == n {
			exclude = node
		}
		if err := n.validateDocumentMove(node, child, exclude); err != nil {
			return err
		}
	}

	return nil
}

// validateDocumentMove validates moving a node into a Document.
// This is similar to validateDocumentInsertionOrReplace but specific to moveBefore.
func (n *Node) validateDocumentMove(node, child, exclude *Node) error {
	switch node.nodeType {
	case ElementNode:
		// Document can only have one element child
		// If node is already a child of this document, it's being moved within the document
		// so we don't count it towards the element limit
		if n.hasElementChildExcluding(exclude) {
			return ErrHierarchyRequest("Document already has a document element.")
		}
		// Check if a doctype follows the reference child
		if child != nil && child != exclude {
			if child.nodeType == DocumentTypeNode || n.doctypeFollows(child) {
				return ErrHierarchyRequest("Cannot insert element before doctype.")
			}
		}

	case TextNode:
		// Text nodes cannot be direct children of Document
		return ErrHierarchyRequest("Cannot insert Text node as a direct child of Document.")
	}

	return nil
}

// isElementOrCharacterData returns true if the node is an Element or CharacterData node.
// CharacterData includes Text, Comment, CDATASection, and ProcessingInstruction.
func isElementOrCharacterData(node *Node) bool {
	if node == nil {
		return false
	}
	switch node.nodeType {
	case ElementNode, TextNode, CommentNode, CDATASectionNode, ProcessingInstructionNode:
		return true
	default:
		return false
	}
}
