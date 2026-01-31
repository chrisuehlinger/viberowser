package dom

import (
	"encoding/xml"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Document represents the entire HTML document.
type Document Node

// HTML namespace URI
const HTMLNamespace = "http://www.w3.org/1999/xhtml"

// NewDocument creates a new empty HTML Document.
func NewDocument() *Document {
	node := newNode(DocumentNode, "#document", nil)
	node.documentData = &documentData{
		contentType: "text/html",
	}
	doc := (*Document)(node)
	node.ownerDoc = doc
	return doc
}

// IsHTML returns true if this is an HTML document.
func (d *Document) IsHTML() bool {
	return d.AsNode().documentData.contentType == "text/html"
}

// ContentType returns the MIME type of the document.
func (d *Document) ContentType() string {
	if d.AsNode().documentData.contentType == "" {
		return "text/html" // Default to HTML for backwards compatibility
	}
	return d.AsNode().documentData.contentType
}

// URL returns the document's URL. Defaults to "about:blank".
func (d *Document) URL() string {
	if d.AsNode().documentData.url == "" {
		return "about:blank"
	}
	return d.AsNode().documentData.url
}

// SetURL sets the document's URL.
func (d *Document) SetURL(url string) {
	d.AsNode().documentData.url = url
}

// DocumentURI returns the document's URI. Same as URL per spec.
func (d *Document) DocumentURI() string {
	return d.URL()
}

// CharacterSet returns the document's character encoding. Defaults to "UTF-8".
func (d *Document) CharacterSet() string {
	if d.AsNode().documentData.characterSet == "" {
		return "UTF-8"
	}
	return d.AsNode().documentData.characterSet
}

// CompatMode returns the document's compatibility mode.
// Returns "CSS1Compat" for standards mode (always for XML documents).
func (d *Document) CompatMode() string {
	// For XML documents, always return CSS1Compat
	// For HTML documents, this would depend on the doctype
	// For now, we return CSS1Compat (standards mode) as default
	return "CSS1Compat"
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
// Per DOM spec, the element's namespace is the HTML namespace when document is an
// HTML document or document's content type is "application/xhtml+xml"; otherwise null.
// This method ignores errors for backwards compatibility. Use CreateElementWithError
// for proper error handling.
func (d *Document) CreateElement(tagName string) *Element {
	el, _ := d.CreateElementWithError(tagName)
	return el
}

// CreateElementWithError creates a new element with the given tag name.
// Returns an InvalidCharacterError if the tag name is not a valid XML Name.
// Per DOM spec: https://dom.spec.whatwg.org/#dom-document-createelement
func (d *Document) CreateElementWithError(tagName string) (*Element, error) {
	// Per DOM spec step 1: If localName does not match the Name production,
	// throw an InvalidCharacterError DOMException.
	if !isValidXMLName(tagName) {
		return nil, ErrInvalidCharacter("The string contains invalid characters.")
	}

	// Determine namespace based on document type
	// Per DOM spec: "The element's namespace is the HTML namespace when document is an
	// HTML document or document's content type is 'application/xhtml+xml'; otherwise null."
	namespace := ""
	contentType := d.ContentType()
	if d.IsHTML() || contentType == "application/xhtml+xml" {
		namespace = HTMLNamespace
	}

	var localName, resultTagName string
	// Per DOM spec step 2: "If context object is an HTML document, let localName be converted to ASCII lowercase."
	// Only true HTML documents (text/html) should lowercase. XHTML documents preserve case.
	if d.IsHTML() {
		// For HTML documents, tag names are lowercased for storage but uppercased for TagName
		localName = strings.ToLower(tagName)
		resultTagName = strings.ToUpper(tagName)
	} else {
		// For XML/XHTML documents, preserve case exactly
		localName = tagName
		resultTagName = tagName
	}

	node := newNode(ElementNode, resultTagName, d)
	node.elementData = &elementData{
		localName:    localName,
		tagName:      resultTagName,
		namespaceURI: namespace,
	}
	node.elementData.attributes = newNamedNodeMap((*Element)(node))

	return (*Element)(node), nil
}

// CreateElementNS creates a new element with the given namespace and qualified name.
func (d *Document) CreateElementNS(namespaceURI, qualifiedName string) *Element {
	el, _ := d.CreateElementNSWithError(namespaceURI, qualifiedName)
	return el
}

// CreateElementNSWithError creates a new element with the given namespace and qualified name.
// Returns an error if the qualified name is invalid or the namespace is incorrect.
func (d *Document) CreateElementNSWithError(namespaceURI, qualifiedName string) (*Element, error) {
	// Validate and extract the qualified name
	namespace, prefix, localName, err := ValidateAndExtractQualifiedName(namespaceURI, qualifiedName)
	if err != nil {
		return nil, err
	}

	// For HTML namespace, tagName is uppercase. For other namespaces, preserve case.
	tagName := qualifiedName
	if namespace == HTMLNamespace {
		tagName = strings.ToUpper(qualifiedName)
	}

	node := newNode(ElementNode, qualifiedName, d)
	node.elementData = &elementData{
		localName:    localName,
		namespaceURI: namespace,
		prefix:       prefix,
		tagName:      tagName,
	}
	node.elementData.attributes = newNamedNodeMap((*Element)(node))

	return (*Element)(node), nil
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

// CreateCDATASection creates a new CDATASection node with the given data.
// Per the DOM spec, this method throws a NotSupportedError for HTML documents.
// Returns nil if this is an HTML document.
func (d *Document) CreateCDATASection(data string) *Node {
	node, _ := d.CreateCDATASectionWithError(data)
	return node
}

// CreateCDATASectionWithError creates a new CDATASection node with the given data.
// Per the DOM spec, this method throws a NotSupportedError for HTML documents.
// Returns an error if this is an HTML document or if data contains "]]>".
func (d *Document) CreateCDATASectionWithError(data string) (*Node, error) {
	// Per DOM spec, throw NotSupportedError for HTML documents
	if d.IsHTML() {
		return nil, ErrNotSupported("CDATASection nodes are not allowed in HTML documents.")
	}

	// Per DOM spec, throw InvalidCharacterError if data contains "]]>"
	if containsCDATASectionClose(data) {
		return nil, ErrInvalidCharacter("CDATASection data cannot contain ']]>'.")
	}

	node := newNode(CDATASectionNode, "#cdata-section", d)
	node.textData = &data
	node.nodeValue = &data
	return node, nil
}

// containsCDATASectionClose checks if data contains the CDATA section close delimiter "]]>".
func containsCDATASectionClose(data string) bool {
	return strings.Contains(data, "]]>")
}

// CreateProcessingInstruction creates a new processing instruction node.
// The target is the application to which the instruction is targeted.
// The data is the content of the processing instruction.
// Returns nil if target is not a valid XML name or data contains "?>".
func (d *Document) CreateProcessingInstruction(target, data string) *Node {
	// Validate target
	if err := ValidateProcessingInstructionTarget(target); err != nil {
		return nil
	}
	// Validate data
	if err := ValidateProcessingInstructionData(data); err != nil {
		return nil
	}

	node := newNode(ProcessingInstructionNode, target, d)
	node.nodeValue = &data
	return node
}

// CreateProcessingInstructionWithError creates a new processing instruction node.
// Returns an error if target is not a valid XML name or data contains "?>".
func (d *Document) CreateProcessingInstructionWithError(target, data string) (*Node, error) {
	// Validate target
	if err := ValidateProcessingInstructionTarget(target); err != nil {
		return nil, err
	}
	// Validate data
	if err := ValidateProcessingInstructionData(data); err != nil {
		return nil, err
	}

	node := newNode(ProcessingInstructionNode, target, d)
	node.nodeValue = &data
	return node, nil
}

// CreateDocumentFragment creates a new empty document fragment.
func (d *Document) CreateDocumentFragment() *DocumentFragment {
	node := newNode(DocumentFragmentNode, "#document-fragment", d)
	return (*DocumentFragment)(node)
}

// CreateAttribute creates a new attribute with the given name.
// For HTML documents, the name is lowercased per the spec.
// Returns nil if the name is empty.
func (d *Document) CreateAttribute(name string) *Attr {
	attr, _ := d.CreateAttributeWithError(name)
	return attr
}

// CreateAttributeWithError creates a new attribute with the given name.
// For HTML documents, the name is lowercased per the spec.
// Returns an InvalidCharacterError if the name is empty.
func (d *Document) CreateAttributeWithError(name string) (*Attr, error) {
	// Per DOM spec, if localName does not match the Name production, throw InvalidCharacterError
	// In practice, browsers only check for empty string
	if name == "" {
		return nil, ErrInvalidCharacter("The string contains invalid characters.")
	}

	// For HTML documents, lowercase the name
	localName := name
	if d.IsHTML() {
		localName = strings.ToLower(name)
	}

	attr := NewAttr(localName, "")
	return attr, nil
}

// CreateAttributeNS creates a new attribute with the given namespace.
func (d *Document) CreateAttributeNS(namespaceURI, qualifiedName string) *Attr {
	return NewAttrNS(namespaceURI, qualifiedName, "")
}

// GetElementById returns the element with the given id.
// Per DOM spec, returns null if id is empty string since elements
// with empty id attribute are not considered to have an ID.
func (d *Document) GetElementById(id string) *Element {
	if id == "" {
		return nil
	}
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
// For error handling, use AppendWithError.
func (d *Document) Append(nodes ...interface{}) {
	_ = d.AppendWithError(nodes...)
}

// AppendWithError appends nodes or strings to this document.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.append() algorithm from the DOM spec.
func (d *Document) AppendWithError(nodes ...interface{}) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes/strings into a single node (or document fragment)
	node := d.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return nil
	}

	// Append the node using the error-returning method
	_, err := d.AsNode().AppendChildWithError(node)
	return err
}

// Prepend prepends nodes or strings to this document.
// For error handling, use PrependWithError.
func (d *Document) Prepend(nodes ...interface{}) {
	_ = d.PrependWithError(nodes...)
}

// PrependWithError prepends nodes or strings to this document.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.prepend() algorithm from the DOM spec.
func (d *Document) PrependWithError(nodes ...interface{}) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes/strings into a single node (or document fragment)
	node := d.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return nil
	}

	// Insert before the first child using the error-returning method
	firstChild := d.AsNode().firstChild
	_, err := d.AsNode().InsertBeforeWithError(node, firstChild)
	return err
}

// ReplaceChildren replaces all children with the given nodes.
// For error handling, use ReplaceChildrenWithError.
func (d *Document) ReplaceChildren(nodes ...interface{}) {
	_ = d.ReplaceChildrenWithError(nodes...)
}

// ReplaceChildrenWithError replaces all children with the given nodes.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.replaceChildren() algorithm from the DOM spec.
// Per spec, validation happens BEFORE any children are removed.
func (d *Document) ReplaceChildrenWithError(nodes ...interface{}) error {
	// Step 1: Convert nodes/strings into a single node (or document fragment)
	var node *Node
	if len(nodes) > 0 {
		node = d.AsNode().convertNodesToFragment(nodes)
	}

	// Step 2: Validate the insertion BEFORE removing any children
	// This ensures we throw HierarchyRequestError before any mutation
	if node != nil {
		if err := d.AsNode().validatePreInsertion(node, nil); err != nil {
			return err
		}
	}

	// Step 3: Remove all existing children
	for d.AsNode().firstChild != nil {
		d.AsNode().RemoveChild(d.AsNode().firstChild)
	}

	// Step 4: Insert the new node(s)
	if node != nil {
		d.AsNode().AppendChild(node)
	}

	return nil
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

// DOMImplementation provides methods for creating DOM objects.
type DOMImplementation struct {
	document *Document // The associated document
}

// NewDOMImplementation creates a new DOMImplementation for the given document.
func NewDOMImplementation(doc *Document) *DOMImplementation {
	return &DOMImplementation{document: doc}
}

// Implementation returns the DOMImplementation for this document.
func (d *Document) Implementation() *DOMImplementation {
	// Each document should have its own implementation instance
	// We create it on demand and cache it
	if d.AsNode().documentData.implementation == nil {
		d.AsNode().documentData.implementation = NewDOMImplementation(d)
	}
	return d.AsNode().documentData.implementation
}

// CreateHTMLDocument creates a new HTML document with the given title.
// If title is nil, no title element is created.
// If title is non-nil (even empty string), a title element is created.
func (impl *DOMImplementation) CreateHTMLDocument(title *string) *Document {
	// Create a new document
	doc := NewDocument()

	// Create and append doctype
	doctype := newNode(DocumentTypeNode, "html", doc)
	doctype.docTypeData = &docTypeData{
		name:     "html",
		publicId: "",
		systemId: "",
	}
	doc.AsNode().AppendChild(doctype)

	// Create html element
	html := doc.CreateElement("html")
	doc.AsNode().AppendChild(html.AsNode())

	// Create head element
	head := doc.CreateElement("head")
	html.AsNode().AppendChild(head.AsNode())

	// Create title element only if title argument was provided (per DOM spec)
	if title != nil {
		titleEl := doc.CreateElement("title")
		if *title != "" {
			titleEl.SetTextContent(*title)
		}
		head.AsNode().AppendChild(titleEl.AsNode())
	}

	// Create body element
	body := doc.CreateElement("body")
	html.AsNode().AppendChild(body.AsNode())

	return doc
}

// CreateDocument creates a new XML document with the given namespace and qualified name.
func (impl *DOMImplementation) CreateDocument(namespaceURI, qualifiedName string, doctype *Node) (*Document, error) {
	return impl.CreateDocumentWithError(namespaceURI, qualifiedName, doctype)
}

// CreateDocumentWithError creates a new XML document with validation.
// Returns an error if the qualified name is invalid.
func (impl *DOMImplementation) CreateDocumentWithError(namespaceURI, qualifiedName string, doctype *Node) (*Document, error) {
	// Validate qualifiedName if not empty
	if qualifiedName != "" {
		_, _, _, err := ValidateAndExtractQualifiedName(namespaceURI, qualifiedName)
		if err != nil {
			return nil, err
		}
	}

	doc := NewDocument()

	// Per DOM spec, content type is determined by namespace:
	// - HTML namespace → application/xhtml+xml
	// - SVG namespace → image/svg+xml
	// - Otherwise → application/xml
	switch namespaceURI {
	case HTMLNamespace:
		doc.AsNode().documentData.contentType = "application/xhtml+xml"
	case "http://www.w3.org/2000/svg":
		doc.AsNode().documentData.contentType = "image/svg+xml"
	default:
		doc.AsNode().documentData.contentType = "application/xml"
	}

	// Add doctype if provided
	if doctype != nil {
		doc.AsNode().AppendChild(doctype)
	}

	// Create root element if qualified name is provided
	if qualifiedName != "" {
		root, err := doc.CreateElementNSWithError(namespaceURI, qualifiedName)
		if err != nil {
			return nil, err
		}
		doc.AsNode().AppendChild(root.AsNode())
	}

	return doc, nil
}

// CreateDocumentType creates a new DocumentType node.
// The ownerDocument is set to the associated document of this implementation.
func (impl *DOMImplementation) CreateDocumentType(qualifiedName, publicId, systemId string) (*Node, error) {
	// Per browser behavior, only reject names containing '>' or whitespace
	// Browsers are very permissive with doctype names
	for _, ch := range qualifiedName {
		if ch == '>' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			return nil, ErrInvalidCharacter("The string did not match the expected pattern.")
		}
	}

	doctype := newNode(DocumentTypeNode, qualifiedName, impl.document)
	doctype.docTypeData = &docTypeData{
		name:     qualifiedName,
		publicId: publicId,
		systemId: systemId,
	}
	return doctype, nil
}

// HasFeature returns true. Per spec, always returns true for compatibility.
func (impl *DOMImplementation) HasFeature() bool {
	return true
}

// ParseXML parses an XML string and returns a Document.
// The returned document has content type "application/xml".
func ParseXML(xmlContent string) (*Document, error) {
	doc := NewDocument()
	doc.AsNode().documentData.contentType = "application/xml"

	decoder := xml.NewDecoder(strings.NewReader(xmlContent))

	// Stack to track current parent node during parsing
	var stack []*Node
	stack = append(stack, doc.AsNode())

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		currentParent := stack[len(stack)-1]

		switch t := token.(type) {
		case xml.StartElement:
			// Create element with proper namespace handling
			var el *Element
			if t.Name.Space != "" {
				// Element has a namespace
				qualifiedName := t.Name.Local
				if prefix := findPrefixForNamespace(t, t.Name.Space); prefix != "" {
					qualifiedName = prefix + ":" + t.Name.Local
				}
				el, _ = doc.CreateElementNSWithError(t.Name.Space, qualifiedName)
			} else {
				el = doc.CreateElement(t.Name.Local)
			}

			// Set attributes
			for _, attr := range t.Attr {
				if attr.Name.Space != "" {
					// Namespaced attribute
					qualifiedName := attr.Name.Local
					if attr.Name.Space == "http://www.w3.org/2000/xmlns/" {
						qualifiedName = "xmlns:" + attr.Name.Local
					}
					el.SetAttributeNS(attr.Name.Space, qualifiedName, attr.Value)
				} else if attr.Name.Local == "xmlns" {
					// Default namespace declaration
					el.SetAttribute("xmlns", attr.Value)
				} else {
					el.SetAttribute(attr.Name.Local, attr.Value)
				}
			}

			currentParent.AppendChild(el.AsNode())
			stack = append(stack, el.AsNode())

		case xml.EndElement:
			// Pop from stack
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			text := string(t)
			// Only create text node if it has non-whitespace content or there's already content
			if strings.TrimSpace(text) != "" || (currentParent.NodeType() == ElementNode && currentParent.firstChild != nil) {
				textNode := doc.CreateTextNode(text)
				currentParent.AppendChild(textNode)
			}

		case xml.Comment:
			comment := doc.CreateComment(string(t))
			currentParent.AppendChild(comment)

		case xml.ProcInst:
			// Processing instruction
			pi := doc.CreateProcessingInstruction(t.Target, string(t.Inst))
			currentParent.AppendChild(pi)

		case xml.Directive:
			// Handle DOCTYPE declaration
			directive := string(t)
			if strings.HasPrefix(strings.ToUpper(directive), "DOCTYPE") {
				// Parse DOCTYPE - simplified parsing
				parts := strings.Fields(directive)
				if len(parts) >= 2 {
					doctypeName := parts[1]
					doctype := newNode(DocumentTypeNode, doctypeName, doc)
					doctype.docTypeData = &docTypeData{
						name: doctypeName,
					}
					currentParent.AppendChild(doctype)
				}
			}
		}
	}

	return doc, nil
}

// findPrefixForNamespace looks for a namespace prefix in the element's attributes.
func findPrefixForNamespace(el xml.StartElement, ns string) string {
	for _, attr := range el.Attr {
		if attr.Name.Space == "http://www.w3.org/2000/xmlns/" && attr.Value == ns {
			return attr.Name.Local
		}
		if attr.Name.Local == "xmlns" && attr.Value == ns {
			return "" // default namespace, no prefix
		}
	}
	return ""
}

// ParseXHTML parses an XHTML string and returns a Document.
// XHTML is parsed like XML but elements get the XHTML namespace.
func ParseXHTML(xhtmlContent string) (*Document, error) {
	doc, err := ParseXML(xhtmlContent)
	if err != nil {
		return nil, err
	}

	// Set content type to XHTML
	doc.AsNode().documentData.contentType = "application/xhtml+xml"

	return doc, nil
}

// CreateRange creates a new Range with both boundary points set to the beginning of the document.
func (d *Document) CreateRange() *Range {
	return NewRange(d)
}
