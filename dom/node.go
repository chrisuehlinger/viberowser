package dom

import (
	"strings"
)

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
}

// documentData holds data specific to Document nodes.
type documentData struct {
	doctype         *Node              // DocumentType node
	documentElement *Node              // root Element
	contentType     string             // The content type (MIME type) of the document
	implementation  *DOMImplementation // The document's DOMImplementation
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
	switch n.nodeType {
	case TextNode, CDATASectionNode:
		if n.textData != nil {
			*n.textData = value
		}
		n.nodeValue = &value
	case CommentNode:
		if n.commentData != nil {
			*n.commentData = value
		}
		n.nodeValue = &value
	case ProcessingInstructionNode:
		n.nodeValue = &value
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
// A node is connected if its root is a document.
func (n *Node) IsConnected() bool {
	root := n.GetRootNode()
	return root != nil && root.nodeType == DocumentNode
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
func (n *Node) SetTextContent(value string) {
	switch n.nodeType {
	case DocumentNode, DocumentTypeNode:
		// Do nothing per the spec
		return
	case TextNode, CommentNode, ProcessingInstructionNode, CDATASectionNode:
		n.SetNodeValue(value)
	default:
		// Remove all children
		for n.firstChild != nil {
			n.RemoveChild(n.firstChild)
		}
		// Add a new text node if value is not empty
		if value != "" {
			textNode := n.ownerDoc.CreateTextNode(value)
			n.AppendChild(textNode)
		}
	}
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
		if err := n.validateDocumentInsertion(node, child); err != nil {
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

		// If the fragment has an element, check if document already has one
		// and also check doctype positioning
		if elementCount == 1 {
			if n.hasElementChild() {
				return ErrHierarchyRequest("Document already has a document element.")
			}
			// Check if the reference child is a doctype or a doctype follows the reference child
			// Per DOM spec: cannot insert element before a doctype
			if child != nil && (child.nodeType == DocumentTypeNode || n.doctypeFollows(child)) {
				return ErrHierarchyRequest("Cannot insert element before doctype.")
			}
		}

	case ElementNode:
		// Document can only have one element child
		if n.hasElementChild() {
			return ErrHierarchyRequest("Document already has a document element.")
		}
		// Check if the reference child is a doctype or a doctype follows the reference child
		// Per DOM spec: cannot insert element before a doctype
		if child != nil && (child.nodeType == DocumentTypeNode || n.doctypeFollows(child)) {
			return ErrHierarchyRequest("Cannot insert element before doctype.")
		}

	case DocumentTypeNode:
		// Document can only have one doctype
		if n.hasDoctype() {
			return ErrHierarchyRequest("Document already has a doctype.")
		}
		// Doctype cannot be inserted after an element
		if n.hasElementChild() {
			// Check if child is null (append) or if element precedes child
			if child == nil || n.elementPrecedes(child) {
				return ErrHierarchyRequest("Cannot insert doctype after document element.")
			}
		}
	}

	return nil
}

// hasElementChild returns true if this node has an element child.
func (n *Node) hasElementChild() bool {
	for c := n.firstChild; c != nil; c = c.nextSibling {
		if c.nodeType == ElementNode {
			return true
		}
	}
	return false
}

// hasDoctype returns true if this document has a doctype child.
func (n *Node) hasDoctype() bool {
	for c := n.firstChild; c != nil; c = c.nextSibling {
		if c.nodeType == DocumentTypeNode {
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
	for c := n.firstChild; c != nil && c != child; c = c.nextSibling {
		if c.nodeType == ElementNode {
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
		for _, child := range children {
			n.insertBefore(child, refChild)
		}
		return newChild
	}

	// If inserting a node before itself, return early (no-op)
	if newChild == refChild {
		return newChild
	}

	// Remove from current parent if necessary
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

	return newChild
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

	return child, nil
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
	if oldChild.parentNode != n {
		return nil, ErrNotFound("The node to be replaced is not a child of this node.")
	}

	// Validate the insertion (treating oldChild as the reference child position)
	if err := n.validatePreInsertion(newChild, oldChild); err != nil {
		return nil, err
	}

	// Insert the new child before the old child
	n.insertBefore(newChild, oldChild)

	// Remove the old child
	return n.RemoveChildWithError(oldChild)
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
		return DocumentPositionDisconnected | DocumentPositionImplementationSpecific | DocumentPositionPreceding
	}

	// Check containment
	if n.Contains(other) {
		return DocumentPositionContainedBy | DocumentPositionFollowing
	}
	if other.Contains(n) {
		return DocumentPositionContains | DocumentPositionPreceding
	}

	// Find the document order by traversing
	// This is a simplified implementation
	return DocumentPositionPreceding // Placeholder
}

// IsSameNode returns true if this node is the same node as the given node.
func (n *Node) IsSameNode(other *Node) bool {
	return n == other
}

// IsEqualNode returns true if this node is equal to the given node.
func (n *Node) IsEqualNode(other *Node) bool {
	if other == nil {
		return false
	}
	if n.nodeType != other.nodeType {
		return false
	}
	if n.nodeName != other.nodeName {
		return false
	}
	if n.NodeValue() != other.NodeValue() {
		return false
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
	case ElementNode:
		if n.elementData != nil {
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
	}
	if n.parentNode != nil {
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
