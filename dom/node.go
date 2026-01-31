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
	localName   string
	namespaceURI string
	prefix      string
	tagName     string
	attributes  *NamedNodeMap
	classList   *DOMTokenList
	id          string
	className   string
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
// This only has an effect on text and comment nodes.
func (n *Node) SetNodeValue(value string) {
	switch n.nodeType {
	case TextNode:
		if n.textData != nil {
			*n.textData = value
		}
		n.nodeValue = &value
	case CommentNode:
		if n.commentData != nil {
			*n.commentData = value
		}
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

// TextContent returns the text content of the node and its descendants.
func (n *Node) TextContent() string {
	switch n.nodeType {
	case DocumentNode, DocumentTypeNode:
		return ""
	case TextNode, CommentNode, ProcessingInstructionNode:
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
		case TextNode:
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
	case TextNode, CommentNode, ProcessingInstructionNode:
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
func (n *Node) AppendChild(child *Node) *Node {
	return n.insertBefore(child, nil)
}

// InsertBefore inserts a node before a reference child node.
// If refChild is nil, the node is appended to the end.
func (n *Node) InsertBefore(newChild, refChild *Node) *Node {
	return n.insertBefore(newChild, refChild)
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

	// Remove from current parent if necessary
	if newChild.parentNode != nil {
		newChild.parentNode.RemoveChild(newChild)
	}

	// Set the new parent
	newChild.parentNode = n
	newChild.ownerDoc = n.ownerDoc

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

// RemoveChild removes a child node from this node.
func (n *Node) RemoveChild(child *Node) *Node {
	if child == nil || child.parentNode != n {
		return nil
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

	return child
}

// ReplaceChild replaces a child node with a new node.
func (n *Node) ReplaceChild(newChild, oldChild *Node) *Node {
	if oldChild == nil || oldChild.parentNode != n {
		return nil
	}
	n.InsertBefore(newChild, oldChild)
	return n.RemoveChild(oldChild)
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
	case TextNode:
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
