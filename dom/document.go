package dom

import (
	"strings"

	"golang.org/x/net/html"
)

// Document represents the entire HTML document.
type Document Node

// NewDocument creates a new empty Document.
func NewDocument() *Document {
	node := newNode(DocumentNode, "#document", nil)
	node.documentData = &documentData{}
	doc := (*Document)(node)
	node.ownerDoc = doc
	return doc
}

// AsNode returns the underlying Node.
func (d *Document) AsNode() *Node {
	return (*Node)(d)
}

// NodeType returns DocumentNode (9).
func (d *Document) NodeType() NodeType {
	return DocumentNode
}

// NodeName returns "#document".
func (d *Document) NodeName() string {
	return "#document"
}

// Doctype returns the DocumentType node, or nil if there is none.
func (d *Document) Doctype() *Node {
	for child := d.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == DocumentTypeNode {
			return child
		}
	}
	return nil
}

// DocumentElement returns the root element of the document.
func (d *Document) DocumentElement() *Element {
	for child := d.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// Head returns the <head> element.
func (d *Document) Head() *Element {
	docEl := d.DocumentElement()
	if docEl == nil {
		return nil
	}
	for child := docEl.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if strings.EqualFold(el.TagName(), "HEAD") {
				return el
			}
		}
	}
	return nil
}

// Body returns the <body> element.
func (d *Document) Body() *Element {
	docEl := d.DocumentElement()
	if docEl == nil {
		return nil
	}
	for child := docEl.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if strings.EqualFold(el.TagName(), "BODY") {
				return el
			}
		}
	}
	return nil
}

// Title returns the document title.
func (d *Document) Title() string {
	head := d.Head()
	if head == nil {
		return ""
	}
	for child := head.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if strings.EqualFold(el.TagName(), "TITLE") {
				return el.TextContent()
			}
		}
	}
	return ""
}

// SetTitle sets the document title.
func (d *Document) SetTitle(title string) {
	head := d.Head()
	if head == nil {
		return
	}

	// Find existing title element
	for child := head.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if strings.EqualFold(el.TagName(), "TITLE") {
				el.SetTextContent(title)
				return
			}
		}
	}

	// Create new title element
	titleEl := d.CreateElement("title")
	titleEl.SetTextContent(title)
	head.AsNode().AppendChild(titleEl.AsNode())
}

// CreateElement creates a new element with the given tag name.
func (d *Document) CreateElement(tagName string) *Element {
	// For HTML documents, tag names are lowercased for storage but uppercased for TagName
	localName := strings.ToLower(tagName)
	upperTagName := strings.ToUpper(tagName)

	node := newNode(ElementNode, upperTagName, d)
	node.elementData = &elementData{
		localName: localName,
		tagName:   upperTagName,
	}
	node.elementData.attributes = newNamedNodeMap((*Element)(node))

	return (*Element)(node)
}

// CreateElementNS creates a new element with the given namespace and qualified name.
func (d *Document) CreateElementNS(namespaceURI, qualifiedName string) *Element {
	prefix, localName := parseQualifiedName(qualifiedName)

	node := newNode(ElementNode, qualifiedName, d)
	node.elementData = &elementData{
		localName:    localName,
		namespaceURI: namespaceURI,
		prefix:       prefix,
		tagName:      strings.ToUpper(qualifiedName),
	}
	node.elementData.attributes = newNamedNodeMap((*Element)(node))

	return (*Element)(node)
}

// CreateTextNode creates a new text node with the given data.
func (d *Document) CreateTextNode(data string) *Node {
	node := newNode(TextNode, "#text", d)
	node.textData = &data
	node.nodeValue = &data
	return node
}

// CreateComment creates a new comment node with the given data.
func (d *Document) CreateComment(data string) *Node {
	node := newNode(CommentNode, "#comment", d)
	node.commentData = &data
	node.nodeValue = &data
	return node
}

// CreateDocumentFragment creates a new empty document fragment.
func (d *Document) CreateDocumentFragment() *DocumentFragment {
	node := newNode(DocumentFragmentNode, "#document-fragment", d)
	return (*DocumentFragment)(node)
}

// CreateAttribute creates a new attribute with the given name.
func (d *Document) CreateAttribute(name string) *Attr {
	return NewAttr(name, "")
}

// CreateAttributeNS creates a new attribute with the given namespace.
func (d *Document) CreateAttributeNS(namespaceURI, qualifiedName string) *Attr {
	return NewAttrNS(namespaceURI, qualifiedName, "")
}

// GetElementById returns the element with the given id.
func (d *Document) GetElementById(id string) *Element {
	return d.findElementById(d.AsNode(), id)
}

func (d *Document) findElementById(node *Node, id string) *Element {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.Id() == id {
				return el
			}
			result := d.findElementById(child, id)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// GetElementsByTagName returns an HTMLCollection of elements with the given tag name.
func (d *Document) GetElementsByTagName(tagName string) *HTMLCollection {
	return NewHTMLCollectionByTagName(d.AsNode(), tagName)
}

// GetElementsByTagNameNS returns an HTMLCollection of elements with the given namespace and local name.
func (d *Document) GetElementsByTagNameNS(namespaceURI, localName string) *HTMLCollection {
	return newHTMLCollection(d.AsNode(), func(el *Element) bool {
		if localName != "*" && el.LocalName() != localName {
			return false
		}
		if namespaceURI != "*" && el.NamespaceURI() != namespaceURI {
			return false
		}
		return true
	})
}

// GetElementsByClassName returns an HTMLCollection of elements with the given class name(s).
func (d *Document) GetElementsByClassName(classNames string) *HTMLCollection {
	return NewHTMLCollectionByClassName(d.AsNode(), classNames)
}

// QuerySelector returns the first element matching the selector.
func (d *Document) QuerySelector(selector string) *Element {
	// Search from document element
	docEl := d.DocumentElement()
	if docEl == nil {
		return nil
	}

	// Check document element itself
	if docEl.Matches(selector) {
		return docEl
	}

	return docEl.QuerySelector(selector)
}

// QuerySelectorAll returns all elements matching the selector.
func (d *Document) QuerySelectorAll(selector string) *NodeList {
	docEl := d.DocumentElement()
	if docEl == nil {
		return NewStaticNodeList(nil)
	}

	var results []*Node

	// Check document element itself
	if docEl.Matches(selector) {
		results = append(results, docEl.AsNode())
	}

	// Get descendant matches
	descendantList := docEl.QuerySelectorAll(selector)
	for i := 0; i < descendantList.Length(); i++ {
		results = append(results, descendantList.Item(i))
	}

	return NewStaticNodeList(results)
}

// Children returns an HTMLCollection of child elements.
func (d *Document) Children() *HTMLCollection {
	return newHTMLCollection(d.AsNode(), func(el *Element) bool {
		return el.AsNode().parentNode == d.AsNode()
	})
}

// ChildElementCount returns the number of child elements.
func (d *Document) ChildElementCount() int {
	count := 0
	for child := d.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			count++
		}
	}
	return count
}

// FirstElementChild returns the first child element (same as DocumentElement for Document).
func (d *Document) FirstElementChild() *Element {
	return d.DocumentElement()
}

// LastElementChild returns the last child element.
func (d *Document) LastElementChild() *Element {
	for child := d.AsNode().lastChild; child != nil; child = child.prevSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// Append appends nodes or strings to this document.
func (d *Document) Append(nodes ...interface{}) {
	for _, item := range nodes {
		switch v := item.(type) {
		case *Node:
			d.AsNode().AppendChild(v)
		case *Element:
			d.AsNode().AppendChild(v.AsNode())
		case string:
			d.AsNode().AppendChild(d.CreateTextNode(v))
		}
	}
}

// Prepend prepends nodes or strings to this document.
func (d *Document) Prepend(nodes ...interface{}) {
	firstChild := d.AsNode().firstChild
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = d.CreateTextNode(v)
		}
		if node != nil {
			d.AsNode().InsertBefore(node, firstChild)
		}
	}
}

// ReplaceChildren replaces all children with the given nodes.
func (d *Document) ReplaceChildren(nodes ...interface{}) {
	// Remove all children
	for d.AsNode().firstChild != nil {
		d.AsNode().RemoveChild(d.AsNode().firstChild)
	}
	// Append new children
	d.Append(nodes...)
}

// ImportNode imports a node from another document.
func (d *Document) ImportNode(node *Node, deep bool) *Node {
	if node == nil {
		return nil
	}

	clone := node.CloneNode(deep)
	d.adoptNode(clone)
	return clone
}

// AdoptNode adopts a node from another document.
func (d *Document) AdoptNode(node *Node) *Node {
	if node == nil {
		return nil
	}

	// Remove from current parent
	if node.parentNode != nil {
		node.parentNode.RemoveChild(node)
	}

	d.adoptNode(node)
	return node
}

func (d *Document) adoptNode(node *Node) {
	node.ownerDoc = d
	for child := node.firstChild; child != nil; child = child.nextSibling {
		d.adoptNode(child)
	}
}

// CreateNodeIterator creates a NodeIterator for traversing the document.
// This is a simplified stub - full implementation would be more complex.
func (d *Document) CreateNodeIterator(root *Node, whatToShow uint32) *NodeIterator {
	return &NodeIterator{
		root:       root,
		whatToShow: whatToShow,
		current:    root,
	}
}

// CreateTreeWalker creates a TreeWalker for traversing the document.
// This is a simplified stub - full implementation would be more complex.
func (d *Document) CreateTreeWalker(root *Node, whatToShow uint32) *TreeWalker {
	return &TreeWalker{
		root:        root,
		whatToShow:  whatToShow,
		currentNode: root,
	}
}

// NodeIterator provides a way to iterate over nodes in a subtree.
type NodeIterator struct {
	root       *Node
	whatToShow uint32
	current    *Node
}

// NextNode returns the next node in document order.
func (ni *NodeIterator) NextNode() *Node {
	// Simplified implementation
	if ni.current == nil {
		return nil
	}

	// Try first child
	if ni.current.firstChild != nil {
		ni.current = ni.current.firstChild
		return ni.current
	}

	// Try next sibling
	if ni.current.nextSibling != nil {
		ni.current = ni.current.nextSibling
		return ni.current
	}

	// Go up and try next sibling
	for ni.current.parentNode != nil {
		ni.current = ni.current.parentNode
		if ni.current == ni.root {
			ni.current = nil
			return nil
		}
		if ni.current.nextSibling != nil {
			ni.current = ni.current.nextSibling
			return ni.current
		}
	}

	ni.current = nil
	return nil
}

// TreeWalker provides a way to walk the document tree.
type TreeWalker struct {
	root        *Node
	whatToShow  uint32
	currentNode *Node
}

// CurrentNode returns the current node.
func (tw *TreeWalker) CurrentNode() *Node {
	return tw.currentNode
}

// SetCurrentNode sets the current node.
func (tw *TreeWalker) SetCurrentNode(node *Node) {
	tw.currentNode = node
}

// ParentNode navigates to the parent node.
func (tw *TreeWalker) ParentNode() *Node {
	if tw.currentNode.parentNode != nil && tw.currentNode.parentNode != tw.root {
		tw.currentNode = tw.currentNode.parentNode
		return tw.currentNode
	}
	return nil
}

// FirstChild navigates to the first child.
func (tw *TreeWalker) FirstChild() *Node {
	if tw.currentNode.firstChild != nil {
		tw.currentNode = tw.currentNode.firstChild
		return tw.currentNode
	}
	return nil
}

// LastChild navigates to the last child.
func (tw *TreeWalker) LastChild() *Node {
	if tw.currentNode.lastChild != nil {
		tw.currentNode = tw.currentNode.lastChild
		return tw.currentNode
	}
	return nil
}

// NextSibling navigates to the next sibling.
func (tw *TreeWalker) NextSibling() *Node {
	if tw.currentNode.nextSibling != nil {
		tw.currentNode = tw.currentNode.nextSibling
		return tw.currentNode
	}
	return nil
}

// PreviousSibling navigates to the previous sibling.
func (tw *TreeWalker) PreviousSibling() *Node {
	if tw.currentNode.prevSibling != nil {
		tw.currentNode = tw.currentNode.prevSibling
		return tw.currentNode
	}
	return nil
}

// NextNode navigates to the next node in document order.
func (tw *TreeWalker) NextNode() *Node {
	// First try first child
	if tw.currentNode.firstChild != nil {
		tw.currentNode = tw.currentNode.firstChild
		return tw.currentNode
	}

	// Try next sibling
	if tw.currentNode.nextSibling != nil {
		tw.currentNode = tw.currentNode.nextSibling
		return tw.currentNode
	}

	// Go up and try next sibling
	node := tw.currentNode
	for node.parentNode != nil && node.parentNode != tw.root {
		node = node.parentNode
		if node.nextSibling != nil {
			tw.currentNode = node.nextSibling
			return tw.currentNode
		}
	}

	return nil
}

// PreviousNode navigates to the previous node in document order.
func (tw *TreeWalker) PreviousNode() *Node {
	// Try previous sibling's last descendant
	if tw.currentNode.prevSibling != nil {
		node := tw.currentNode.prevSibling
		for node.lastChild != nil {
			node = node.lastChild
		}
		tw.currentNode = node
		return tw.currentNode
	}

	// Try parent
	if tw.currentNode.parentNode != nil && tw.currentNode.parentNode != tw.root {
		tw.currentNode = tw.currentNode.parentNode
		return tw.currentNode
	}

	return nil
}

// ParseHTML parses an HTML string and returns a Document.
func ParseHTML(htmlContent string) (*Document, error) {
	doc := NewDocument()

	// Parse using golang.org/x/net/html
	netDoc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	// Convert the parsed tree to our DOM structure
	convertHTMLTree(netDoc, doc.AsNode(), doc)

	return doc, nil
}

// convertHTMLTree converts an html.Node tree to our DOM tree.
func convertHTMLTree(src *html.Node, parent *Node, doc *Document) {
	for c := src.FirstChild; c != nil; c = c.NextSibling {
		var node *Node

		switch c.Type {
		case html.TextNode:
			node = doc.CreateTextNode(c.Data)

		case html.ElementNode:
			el := doc.CreateElement(c.Data)
			for _, attr := range c.Attr {
				if attr.Namespace != "" {
					el.SetAttributeNS(attr.Namespace, attr.Key, attr.Val)
				} else {
					el.SetAttribute(attr.Key, attr.Val)
				}
			}
			node = el.AsNode()

		case html.CommentNode:
			node = doc.CreateComment(c.Data)

		case html.DoctypeNode:
			node = newNode(DocumentTypeNode, c.Data, doc)
			node.docTypeData = &docTypeData{
				name:     c.Data,
				publicId: "",
				systemId: "",
			}
			for _, attr := range c.Attr {
				if attr.Key == "public" {
					node.docTypeData.publicId = attr.Val
				} else if attr.Key == "system" {
					node.docTypeData.systemId = attr.Val
				}
			}

		case html.DocumentNode:
			// Don't create a new document node, just process children
			convertHTMLTree(c, parent, doc)
			continue

		default:
			continue
		}

		if node != nil {
			parent.AppendChild(node)
			// Process children
			if c.Type == html.ElementNode {
				convertHTMLTree(c, node, doc)
			}
		}
	}
}
