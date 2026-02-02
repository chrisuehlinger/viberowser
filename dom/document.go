package dom

import (
	"encoding/xml"
	"io"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Document represents the entire HTML document.
type Document Node

// HTML namespace URI
const HTMLNamespace = "http://www.w3.org/1999/xhtml"

// toASCIILowercase converts ASCII letters A-Z to lowercase a-z.
// Non-ASCII characters and other characters are left unchanged.
// This implements the "ASCII lowercase" algorithm from the DOM spec.
func toASCIILowercase(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // 'a' - 'A' = 32
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// stripAndCollapseASCIIWhitespace implements the "strip and collapse ASCII whitespace"
// algorithm from the WHATWG Infra spec.
// It replaces sequences of ASCII whitespace with a single space and strips
// leading/trailing whitespace.
// Note: Uses isASCIIWhitespace from htmlcollection.go
func stripAndCollapseASCIIWhitespace(s string) string {
	var result strings.Builder
	inWhitespace := true // Start true to strip leading whitespace
	for _, r := range s {
		if isASCIIWhitespace(r) {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}
	// Strip trailing whitespace (if ends with space, remove it)
	str := result.String()
	if len(str) > 0 && str[len(str)-1] == ' ' {
		str = str[:len(str)-1]
	}
	return str
}

// toASCIIUppercase converts ASCII letters a-z to uppercase A-Z.
// Non-ASCII characters and other characters are left unchanged.
// This implements the "ASCII uppercase" algorithm from the DOM spec.
func toASCIIUppercase(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result = append(result, c-32) // 'A' - 'a' = -32
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

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

// NewXMLDocument creates a new empty XML Document (for new Document() constructor).
// Per DOM spec, the Document() constructor creates a document with contentType "application/xml".
func NewXMLDocument() *Document {
	node := newNode(DocumentNode, "#document", nil)
	node.documentData = &documentData{
		contentType: "application/xml",
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

// SetContentType sets the document's content type.
func (d *Document) SetContentType(contentType string) {
	d.AsNode().documentData.contentType = contentType
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
// Returns "CSS1Compat" for standards mode or limited-quirks mode.
// Returns "BackCompat" for quirks mode.
func (d *Document) CompatMode() string {
	// For XML documents, always return CSS1Compat
	if !d.IsHTML() {
		return "CSS1Compat"
	}
	// For HTML documents, check the document mode
	if d.AsNode().documentData.mode == QuirksMode {
		return "BackCompat"
	}
	return "CSS1Compat"
}

// Mode returns the document's rendering mode.
func (d *Document) Mode() DocumentMode {
	return d.AsNode().documentData.mode
}

// SetMode sets the document's rendering mode.
func (d *Document) SetMode(mode DocumentMode) {
	d.AsNode().documentData.mode = mode
}

// ReadyState returns the document's ready state.
// Returns "loading", "interactive", or "complete".
func (d *Document) ReadyState() DocumentReadyState {
	state := d.AsNode().documentData.readyState
	if state == "" {
		// Default to "complete" for documents without explicit state
		return ReadyStateComplete
	}
	return state
}

// SetReadyState sets the document's ready state.
func (d *Document) SetReadyState(state DocumentReadyState) {
	d.AsNode().documentData.readyState = state
}

// LastModified returns the document's last modification date.
// Returns a string in the format "MM/DD/YYYY hh:mm:ss" in the user's local timezone.
func (d *Document) LastModified() string {
	if d.AsNode().documentData.lastModified != "" {
		return d.AsNode().documentData.lastModified
	}
	// If not set, return current time
	return formatLastModified(time.Now())
}

// SetLastModified sets the document's last modification date.
func (d *Document) SetLastModified(lastMod string) {
	d.AsNode().documentData.lastModified = lastMod
}

// formatLastModified formats a time as "MM/DD/YYYY hh:mm:ss".
func formatLastModified(t time.Time) string {
	return t.Format("01/02/2006 15:04:05")
}

// Cookie returns the document's cookies.
// Returns an empty string for cookie-averse documents (those without a browsing context
// or with a non-HTTP(S) URL scheme).
func (d *Document) Cookie() string {
	// Per HTML spec, documents without a browsing context or with non-HTTP(S) URLs
	// are "cookie-averse" and should return empty string.
	// For now, we return the stored cookie value (empty by default).
	return d.AsNode().documentData.cookie
}

// SetCookie sets the document's cookie.
func (d *Document) SetCookie(cookie string) {
	d.AsNode().documentData.cookie = cookie
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

// Title returns the document title per the HTML spec.
// For SVG documents, it returns the text content of the first SVG title element
// that is a direct child of the documentElement.
// For HTML documents, it returns the text content of the first title element
// in the document.
// The result has ASCII whitespace stripped and collapsed.
func (d *Document) Title() string {
	docElement := d.DocumentElement()
	if docElement == nil {
		return ""
	}

	var value string

	// Check if this is an SVG document (documentElement is svg in SVG namespace)
	// Note: localName comparison is case-sensitive per spec ("svg" not "SVG")
	if docElement.NamespaceURI() == SVGNamespace &&
		docElement.LocalName() == "svg" {
		// For SVG: find first SVG title child of documentElement
		for child := docElement.AsNode().firstChild; child != nil; child = child.nextSibling {
			if child.nodeType == ElementNode {
				el := (*Element)(child)
				if el.NamespaceURI() == SVGNamespace &&
					strings.EqualFold(el.LocalName(), "title") {
					value = el.TextContent()
					break
				}
			}
		}
	} else {
		// For HTML: find first title element in the document (any namespace check is implicit)
		// The spec says "the first title element in the document, if there is one"
		titleEl := d.findFirstTitleElement(d.AsNode())
		if titleEl != nil {
			value = titleEl.TextContent()
		}
	}

	// Strip and collapse ASCII whitespace
	return stripAndCollapseASCIIWhitespace(value)
}

// findFirstTitleElement finds the first title element in HTML namespace
// in a depth-first traversal starting from the given node.
func (d *Document) findFirstTitleElement(node *Node) *Element {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.NamespaceURI() == HTMLNamespace &&
				strings.EqualFold(el.LocalName(), "title") {
				return el
			}
			// Recursively search children
			if found := d.findFirstTitleElement(child); found != nil {
				return found
			}
		}
	}
	return nil
}

// SetTitle sets the document title per the HTML spec.
// For SVG documents, it sets/creates an SVG title element as first child of documentElement.
// For HTML documents, it sets/creates an HTML title element in the head.
func (d *Document) SetTitle(title string) {
	docElement := d.DocumentElement()
	if docElement == nil {
		return
	}

	// Check if this is an SVG document (documentElement is svg in SVG namespace)
	// Note: localName comparison is case-sensitive per spec ("svg" not "SVG")
	if docElement.NamespaceURI() == SVGNamespace &&
		docElement.LocalName() == "svg" {
		// For SVG: find or create SVG title as first child of documentElement
		var existingTitle *Element
		for child := docElement.AsNode().firstChild; child != nil; child = child.nextSibling {
			if child.nodeType == ElementNode {
				el := (*Element)(child)
				if el.NamespaceURI() == SVGNamespace &&
					strings.EqualFold(el.LocalName(), "title") {
					existingTitle = el
					break
				}
			}
		}

		if existingTitle != nil {
			// Update existing title
			d.setTitleElementContent(existingTitle, title)
		} else {
			// Create new SVG title element and insert as first child
			titleEl := d.CreateElementNS(SVGNamespace, "title")
			d.setTitleElementContent(titleEl, title)
			docElement.AsNode().InsertBefore(titleEl.AsNode(), docElement.AsNode().firstChild)
		}
		return
	}

	// For HTML documents
	head := d.Head()
	if head == nil {
		return
	}

	// Find existing title element in document
	existingTitle := d.findFirstTitleElement(d.AsNode())
	if existingTitle != nil {
		d.setTitleElementContent(existingTitle, title)
		return
	}

	// Create new title element in head
	titleEl := d.CreateElement("title")
	d.setTitleElementContent(titleEl, title)
	head.AsNode().AppendChild(titleEl.AsNode())
}

// setTitleElementContent sets the text content of a title element.
// If the title is empty, it removes all children. Otherwise, it sets
// a single text node with the title content.
func (d *Document) setTitleElementContent(el *Element, title string) {
	// Per spec, setting string replaces all children with a single text node
	// (or no text node if empty)
	el.SetTextContent(title)
}

// Forms returns a live HTMLCollection of all form elements in the document.
// Per HTML spec, this includes only <form> elements in the HTML namespace.
// The collection is cached to ensure document.forms === document.forms.
func (d *Document) Forms() *HTMLCollection {
	if d.AsNode().documentData.formsCollection == nil {
		d.AsNode().documentData.formsCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			return el.NamespaceURI() == HTMLNamespace &&
				strings.EqualFold(el.LocalName(), "form")
		})
	}
	return d.AsNode().documentData.formsCollection
}

// Images returns a live HTMLCollection of all img elements in the document.
// Per HTML spec, this includes only <img> elements in the HTML namespace.
// The collection is cached to ensure document.images === document.images.
func (d *Document) Images() *HTMLCollection {
	if d.AsNode().documentData.imagesCollection == nil {
		d.AsNode().documentData.imagesCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			return el.NamespaceURI() == HTMLNamespace &&
				strings.EqualFold(el.LocalName(), "img")
		})
	}
	return d.AsNode().documentData.imagesCollection
}

// Links returns a live HTMLCollection of all <a> and <area> elements with href attributes.
// Per HTML spec, this includes only elements in the HTML namespace that have href attributes.
// The collection is cached to ensure document.links === document.links.
func (d *Document) Links() *HTMLCollection {
	if d.AsNode().documentData.linksCollection == nil {
		d.AsNode().documentData.linksCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			if el.NamespaceURI() != HTMLNamespace {
				return false
			}
			localName := strings.ToLower(el.LocalName())
			if localName != "a" && localName != "area" {
				return false
			}
			// Only include if they have an href attribute
			return el.HasAttribute("href")
		})
	}
	return d.AsNode().documentData.linksCollection
}

// Scripts returns a live HTMLCollection of all script elements in the document.
// Per HTML spec, this includes only <script> elements in the HTML namespace.
// The collection is cached to ensure document.scripts === document.scripts.
func (d *Document) Scripts() *HTMLCollection {
	if d.AsNode().documentData.scriptsCollection == nil {
		d.AsNode().documentData.scriptsCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			return el.NamespaceURI() == HTMLNamespace &&
				strings.EqualFold(el.LocalName(), "script")
		})
	}
	return d.AsNode().documentData.scriptsCollection
}

// Embeds returns a live HTMLCollection of all embed elements in the document.
// Per HTML spec, this includes only <embed> elements in the HTML namespace.
// The collection is cached to ensure document.embeds === document.embeds.
// Note: Plugins() also returns this same collection per spec.
func (d *Document) Embeds() *HTMLCollection {
	if d.AsNode().documentData.embedsCollection == nil {
		d.AsNode().documentData.embedsCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			return el.NamespaceURI() == HTMLNamespace &&
				strings.EqualFold(el.LocalName(), "embed")
		})
	}
	return d.AsNode().documentData.embedsCollection
}

// Plugins returns a live HTMLCollection of all embed elements in the document.
// Per HTML spec, this is an alias for Embeds() and returns the same collection object.
func (d *Document) Plugins() *HTMLCollection {
	return d.Embeds()
}

// Anchors returns a live HTMLCollection of all <a> elements with name attributes.
// Per HTML spec, this includes only <a> elements in the HTML namespace with name attributes.
// Note: This is a legacy API; Links() is preferred for finding hyperlinks.
// The collection is cached to ensure document.anchors === document.anchors.
func (d *Document) Anchors() *HTMLCollection {
	if d.AsNode().documentData.anchorsCollection == nil {
		d.AsNode().documentData.anchorsCollection = newHTMLCollection(d.AsNode(), func(el *Element) bool {
			if el.NamespaceURI() != HTMLNamespace {
				return false
			}
			if !strings.EqualFold(el.LocalName(), "a") {
				return false
			}
			// Only include if they have a name attribute
			return el.HasAttribute("name")
		})
	}
	return d.AsNode().documentData.anchorsCollection
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
// Note: Browsers are more permissive than strict XML 1.0, so we use permissive validation.
func (d *Document) CreateElementWithError(tagName string) (*Element, error) {
	// Per DOM spec step 1: If localName does not match the Name production,
	// throw an InvalidCharacterError DOMException.
	// Note: We use permissive validation to match browser behavior.
	if !isValidXMLNamePermissive(tagName) {
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
	// IMPORTANT: The spec requires ASCII lowercase/uppercase only - Unicode case conversion
	// would incorrectly transform characters like the Kelvin sign (U+212A) or Turkish letters.
	if d.IsHTML() {
		// For HTML documents, tag names are ASCII lowercased for storage but ASCII uppercased for TagName
		localName = toASCIILowercase(tagName)
		resultTagName = toASCIIUppercase(tagName)
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

	// Compute tagName: if there's a prefix, it's prefix:localName; otherwise just localName
	// This is important for cases like "f:o:o" where tagName should be "f:o" (prefix + ":" + localName)
	var tagName string
	if prefix != "" {
		tagName = prefix + ":" + localName
	} else {
		tagName = localName
	}

	// Per DOM spec, tagName is ASCII uppercase only when:
	// 1. The element is in the HTML namespace, AND
	// 2. The ownerDocument is an HTML document (contentType "text/html")
	// For XML documents (including XHTML), preserve case.
	if namespace == HTMLNamespace && d.IsHTML() {
		tagName = toASCIIUppercase(tagName)
	}

	node := newNode(ElementNode, tagName, d)
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
// Returns an InvalidCharacterError if the name is invalid.
func (d *Document) CreateAttributeWithError(name string) (*Attr, error) {
	// Per DOM spec and WPT name-validation.html, validate the attribute name
	// Invalid chars: empty, whitespace, NULL, /, =, >
	if !IsValidAttributeLocalName(name) {
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
	attr, _ := d.CreateAttributeNSWithError(namespaceURI, qualifiedName)
	return attr
}

// CreateAttributeNSWithError creates a new attribute with the given namespace.
// Returns an error if the qualified name is invalid.
func (d *Document) CreateAttributeNSWithError(namespaceURI, qualifiedName string) (*Attr, error) {
	// Validate using the same logic as setAttributeNS
	_, _, _, err := ValidateAndExtractQualifiedName(namespaceURI, qualifiedName)
	if err != nil {
		return nil, err
	}
	return NewAttrNS(namespaceURI, qualifiedName, ""), nil
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

// GetElementsByName returns a live NodeList of elements with the given name attribute.
// Per HTML spec, this method is only defined for HTML documents and only matches
// elements in the HTML namespace.
// The name attribute is matched case-sensitively.
func (d *Document) GetElementsByName(name string) *NodeList {
	return NewLiveNodeList(d.AsNode(), func(node *Node) bool {
		if node.nodeType != ElementNode {
			return false
		}
		el := (*Element)(node)
		// Only match elements in the HTML namespace
		if el.NamespaceURI() != HTMLNamespace {
			return false
		}
		return el.GetAttribute("name") == name
	})
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
// Per spec, this generates a single mutation record for the parent containing
// all removed children and all added nodes.
func (d *Document) ReplaceChildrenWithError(nodes ...interface{}) error {
	return d.AsNode().replaceChildrenImpl(nodes)
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
	result, _ := d.AdoptNodeWithError(node)
	return result
}

// AdoptNodeWithError adopts a node from another document, returning an error
// if the node cannot be adopted (e.g., Document nodes cannot be adopted).
func (d *Document) AdoptNodeWithError(node *Node) (*Node, error) {
	if node == nil {
		return nil, nil
	}

	// Per DOM spec, Document nodes cannot be adopted
	if node.nodeType == DocumentNode {
		return nil, ErrNotSupported("Document nodes cannot be adopted")
	}

	// Remove from current parent
	if node.parentNode != nil {
		node.parentNode.RemoveChild(node)
	}

	d.adoptNode(node)
	return node, nil
}

func (d *Document) adoptNode(node *Node) {
	node.ownerDoc = d
	for child := node.firstChild; child != nil; child = child.nextSibling {
		d.adoptNode(child)
	}
}

// CreateNodeIterator creates a NodeIterator for traversing the document.
func (d *Document) CreateNodeIterator(root *Node, whatToShow uint32) *NodeIterator {
	ni := &NodeIterator{
		document:                   d,
		root:                       root,
		whatToShow:                 whatToShow,
		referenceNode:              root,
		pointerBeforeReferenceNode: true,
	}
	// Register the iterator with root's node document for pre-removal steps.
	// Per DOM spec, pre-removal steps are run for iterators whose root's node document
	// matches the removed node's node document.
	rootDoc := root.ownerDoc
	if root.nodeType == DocumentNode {
		rootDoc = (*Document)(root)
	}
	if rootDoc != nil {
		rootDoc.registerNodeIterator(ni)
	} else {
		// Fallback to creating document if root has no owner
		d.registerNodeIterator(ni)
	}
	return ni
}

// registerNodeIterator adds an iterator to the document's list of active iterators.
func (d *Document) registerNodeIterator(ni *NodeIterator) {
	n := (*Node)(d)
	if n.documentData == nil {
		return
	}
	n.documentData.nodeIterators = append(n.documentData.nodeIterators, ni)
}

// unregisterNodeIterator removes an iterator from the document's list.
func (d *Document) unregisterNodeIterator(ni *NodeIterator) {
	n := (*Node)(d)
	if n.documentData == nil {
		return
	}
	iterators := n.documentData.nodeIterators
	for i, iter := range iterators {
		if iter == ni {
			// Remove by swapping with last and truncating
			iterators[i] = iterators[len(iterators)-1]
			n.documentData.nodeIterators = iterators[:len(iterators)-1]
			return
		}
	}
}

// notifyNodeIteratorsOfRemoval runs pre-removal steps for all NodeIterators
// when a node is about to be removed. This implements the DOM spec's
// "pre-removing steps" for NodeIterator.
func (d *Document) notifyNodeIteratorsOfRemoval(node *Node) {
	n := (*Node)(d)
	if n.documentData == nil {
		return
	}
	for _, ni := range n.documentData.nodeIterators {
		ni.preRemovingSteps(node)
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
// Implements the DOM NodeIterator interface.
type NodeIterator struct {
	document                   *Document
	root                       *Node
	whatToShow                 uint32
	referenceNode              *Node
	pointerBeforeReferenceNode bool
}

// Detach removes this iterator from the document's list of active iterators.
// This is a no-op in modern DOM (iterators no longer need explicit detachment)
// but we use it to clean up the registry.
func (ni *NodeIterator) Detach() {
	if ni.document != nil {
		ni.document.unregisterNodeIterator(ni)
	}
}

// preRemovingSteps runs the pre-removal steps for this iterator when a node
// is being removed. Implements the DOM spec's NodeIterator pre-removing steps.
func (ni *NodeIterator) preRemovingSteps(toBeRemoved *Node) {
	// "If the node being removed is an inclusive ancestor of root, terminate."
	// This handles the case where the root itself or an ancestor of root is being removed.
	if isInclusiveAncestor(toBeRemoved, ni.root) {
		return
	}
	// "If the node being removed is not an inclusive ancestor of referenceNode, terminate."
	if !isInclusiveAncestor(toBeRemoved, ni.referenceNode) {
		return
	}

	// "If the pointerBeforeReferenceNode attribute value is false, set the
	// referenceNode attribute to the first node preceding the node being
	// removed, and terminate these steps."
	if !ni.pointerBeforeReferenceNode {
		ni.referenceNode = precedingNode(toBeRemoved, ni.root)
		return
	}

	// "If there is a node following the last inclusive descendant of the node
	// being removed, set the referenceNode attribute to the first such node,
	// and terminate these steps."
	next := followingNode(lastInclusiveDescendant(toBeRemoved), ni.root)
	if next != nil {
		ni.referenceNode = next
		return
	}

	// "Set the referenceNode attribute to the first node preceding the node
	// being removed and set the pointerBeforeReferenceNode attribute to false."
	ni.referenceNode = precedingNode(toBeRemoved, ni.root)
	ni.pointerBeforeReferenceNode = false
}

// isInclusiveAncestor returns true if ancestor is an inclusive ancestor of node.
func isInclusiveAncestor(ancestor, node *Node) bool {
	for n := node; n != nil; n = n.parentNode {
		if n == ancestor {
			return true
		}
	}
	return false
}

// lastInclusiveDescendant returns the last inclusive descendant of node.
func lastInclusiveDescendant(node *Node) *Node {
	for node.lastChild != nil {
		node = node.lastChild
	}
	return node
}

// precedingNode returns the first node that precedes node in tree order,
// constrained to the subtree rooted at root. Returns nil if no such node exists.
func precedingNode(node, root *Node) *Node {
	if node == root {
		return nil
	}
	// If node has a previous sibling, return its last inclusive descendant
	if node.prevSibling != nil {
		return lastInclusiveDescendant(node.prevSibling)
	}
	// Otherwise return the parent (if within root's subtree)
	parent := node.parentNode
	if parent == root {
		return root
	}
	return parent
}

// followingNode returns the first node that follows node in tree order,
// constrained to the subtree rooted at root. Returns nil if no such node exists.
func followingNode(node, root *Node) *Node {
	// Check descendants first (first child)
	if node.firstChild != nil {
		return node.firstChild
	}
	// Then check following siblings, walking up ancestors
	for n := node; n != nil && n != root; n = n.parentNode {
		if n.nextSibling != nil {
			return n.nextSibling
		}
	}
	return nil
}

// Root returns the root node of the iterator.
func (ni *NodeIterator) Root() *Node {
	return ni.root
}

// WhatToShow returns the whatToShow value.
func (ni *NodeIterator) WhatToShow() uint32 {
	return ni.whatToShow
}

// ReferenceNode returns the reference node.
func (ni *NodeIterator) ReferenceNode() *Node {
	return ni.referenceNode
}

// PointerBeforeReferenceNode returns whether the pointer is before the reference node.
func (ni *NodeIterator) PointerBeforeReferenceNode() bool {
	return ni.pointerBeforeReferenceNode
}

// SetReferenceNode sets the reference node and pointer position.
func (ni *NodeIterator) SetReferenceNode(node *Node, before bool) {
	ni.referenceNode = node
	ni.pointerBeforeReferenceNode = before
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

	// Determine document mode from doctype before converting tree
	doc.SetMode(determineDocumentMode(netDoc))

	// Convert the parsed tree to our DOM structure
	convertHTMLTree(netDoc, doc.AsNode(), doc)

	return doc, nil
}

// determineDocumentMode determines the document mode (quirks, limited-quirks, or no-quirks)
// based on the doctype per the HTML spec.
// See: https://html.spec.whatwg.org/multipage/parsing.html#the-initial-insertion-mode
func determineDocumentMode(doc *html.Node) DocumentMode {
	// Find the doctype node
	var doctype *html.Node
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.DoctypeNode {
			doctype = c
			break
		}
	}

	// No doctype = quirks mode
	if doctype == nil {
		return QuirksMode
	}

	// Get doctype properties
	name := strings.ToLower(doctype.Data)
	var publicId, systemId string
	for _, attr := range doctype.Attr {
		if attr.Key == "public" {
			publicId = attr.Val
		} else if attr.Key == "system" {
			systemId = attr.Val
		}
	}

	publicIdLower := strings.ToLower(publicId)
	systemIdLower := strings.ToLower(systemId)

	// Per HTML spec, these conditions trigger quirks mode:
	// 1. Force-quirks flag is set (not applicable for golang.org/x/net/html)
	// 2. Name is not "html" (case-insensitive)
	if name != "html" {
		return QuirksMode
	}

	// 3. Public identifier is set to specific values (case-insensitive)
	quirksPublicIds := []string{
		"-//w3o//dtd w3 html strict 3.0//en//",
		"-/w3c/dtd html 4.0 transitional/en",
		"html",
	}
	for _, qid := range quirksPublicIds {
		if publicIdLower == qid {
			return QuirksMode
		}
	}

	// 4. Public identifier starts with specific prefixes
	quirksPublicIdPrefixes := []string{
		"+//silmaril//dtd html pro v0r11 19970101//",
		"-//as//dtd html 3.0 aswedit + extensions//",
		"-//advasoft ltd//dtd html 3.0 aswedit + extensions//",
		"-//ietf//dtd html 2.0 level 1//",
		"-//ietf//dtd html 2.0 level 2//",
		"-//ietf//dtd html 2.0 strict level 1//",
		"-//ietf//dtd html 2.0 strict level 2//",
		"-//ietf//dtd html 2.0 strict//",
		"-//ietf//dtd html 2.0//",
		"-//ietf//dtd html 2.1e//",
		"-//ietf//dtd html 3.0//",
		"-//ietf//dtd html 3.2 final//",
		"-//ietf//dtd html 3.2//",
		"-//ietf//dtd html 3//",
		"-//ietf//dtd html level 0//",
		"-//ietf//dtd html level 1//",
		"-//ietf//dtd html level 2//",
		"-//ietf//dtd html level 3//",
		"-//ietf//dtd html strict level 0//",
		"-//ietf//dtd html strict level 1//",
		"-//ietf//dtd html strict level 2//",
		"-//ietf//dtd html strict level 3//",
		"-//ietf//dtd html strict//",
		"-//ietf//dtd html//",
		"-//metrius//dtd metrius presentational//",
		"-//microsoft//dtd internet explorer 2.0 html strict//",
		"-//microsoft//dtd internet explorer 2.0 html//",
		"-//microsoft//dtd internet explorer 2.0 tables//",
		"-//microsoft//dtd internet explorer 3.0 html strict//",
		"-//microsoft//dtd internet explorer 3.0 html//",
		"-//microsoft//dtd internet explorer 3.0 tables//",
		"-//netscape comm. corp.//dtd html//",
		"-//netscape comm. corp.//dtd strict html//",
		"-//o'reilly and associates//dtd html 2.0//",
		"-//o'reilly and associates//dtd html extended 1.0//",
		"-//o'reilly and associates//dtd html extended relaxed 1.0//",
		"-//sq//dtd html 2.0 hotmetal + extensions//",
		"-//softquad software//dtd hotmetal pro 6.0::19990601::extensions to html 4.0//",
		"-//softquad//dtd hotmetal pro 4.0::19971010::extensions to html 4.0//",
		"-//spyglass//dtd html 2.0 extended//",
		"-//sun microsystems corp.//dtd hotjava html//",
		"-//sun microsystems corp.//dtd hotjava strict html//",
		"-//w3c//dtd html 3 1995-03-24//",
		"-//w3c//dtd html 3.2 draft//",
		"-//w3c//dtd html 3.2 final//",
		"-//w3c//dtd html 3.2//",
		"-//w3c//dtd html 3.2s draft//",
		"-//w3c//dtd html 4.0 frameset//",
		"-//w3c//dtd html 4.0 transitional//",
		"-//w3c//dtd html experimental 19960712//",
		"-//w3c//dtd html experimental 970421//",
		"-//w3c//dtd w3 html//",
		"-//w3o//dtd w3 html 3.0//",
		"-//webtechs//dtd mozilla html 2.0//",
		"-//webtechs//dtd mozilla html//",
	}
	for _, prefix := range quirksPublicIdPrefixes {
		if strings.HasPrefix(publicIdLower, prefix) {
			return QuirksMode
		}
	}

	// 5. System identifier is missing and public identifier starts with specific prefixes
	if systemId == "" {
		missingSysIdQuirksPrefixes := []string{
			"-//w3c//dtd html 4.01 frameset//",
			"-//w3c//dtd html 4.01 transitional//",
		}
		for _, prefix := range missingSysIdQuirksPrefixes {
			if strings.HasPrefix(publicIdLower, prefix) {
				return QuirksMode
			}
		}
	}

	// 6. System identifier is specific value
	if systemIdLower == "http://www.ibm.com/data/dtd/v11/ibmxhtml1-transitional.dtd" {
		return QuirksMode
	}

	// Check for limited-quirks mode
	// Public identifier starts with specific prefixes
	limitedQuirksPrefixes := []string{
		"-//w3c//dtd xhtml 1.0 frameset//",
		"-//w3c//dtd xhtml 1.0 transitional//",
	}
	for _, prefix := range limitedQuirksPrefixes {
		if strings.HasPrefix(publicIdLower, prefix) {
			return LimitedQuirksMode
		}
	}

	// System identifier is not missing and public identifier starts with specific prefixes
	if systemId != "" {
		sysIdLimitedQuirksPrefixes := []string{
			"-//w3c//dtd html 4.01 frameset//",
			"-//w3c//dtd html 4.01 transitional//",
		}
		for _, prefix := range sysIdLimitedQuirksPrefixes {
			if strings.HasPrefix(publicIdLower, prefix) {
				return LimitedQuirksMode
			}
		}
	}

	// Default: no-quirks mode
	return NoQuirksMode
}

// SVG and MathML namespace URIs
const SVGNamespace = "http://www.w3.org/2000/svg"
const MathMLNamespace = "http://www.w3.org/1998/Math/MathML"

// Other common namespace URIs
const XLinkNamespace = "http://www.w3.org/1999/xlink"
const XMLNamespace = "http://www.w3.org/XML/1998/namespace"
const XMLNSNamespace = "http://www.w3.org/2000/xmlns/"

// convertAttrNamespace converts short namespace prefixes from golang.org/x/net/html
// to their full URI forms as required by the DOM spec.
func convertAttrNamespace(ns string) string {
	switch ns {
	case "xmlns":
		return XMLNSNamespace
	case "xlink":
		return XLinkNamespace
	case "xml":
		return XMLNamespace
	default:
		return ns
	}
}

// convertHTMLTree converts an html.Node tree to our DOM tree.
func convertHTMLTree(src *html.Node, parent *Node, doc *Document) {
	for c := src.FirstChild; c != nil; c = c.NextSibling {
		var node *Node

		switch c.Type {
		case html.TextNode:
			node = doc.CreateTextNode(c.Data)

		case html.ElementNode:
			// Convert short namespace names from golang.org/x/net/html to full URIs
			namespace := c.Namespace
			switch namespace {
			case "":
				namespace = HTMLNamespace
			case "svg":
				namespace = SVGNamespace
			case "math":
				namespace = MathMLNamespace
			}

			el := doc.CreateElementNS(namespace, c.Data)
			for _, attr := range c.Attr {
				if attr.Namespace != "" {
					// Convert short namespace prefixes to full URIs
					attrNS := convertAttrNamespace(attr.Namespace)
					// Reconstruct qualified name: golang.org/x/net/html gives us
					// the local name in Key and the namespace prefix in Namespace
					// For example: xmlns:xlink has Namespace="xmlns", Key="xlink"
					// We need to pass "xmlns:xlink" as the qualified name
					qualifiedName := attr.Namespace + ":" + attr.Key
					el.SetAttributeNS(attrNS, qualifiedName, attr.Val)
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
				el := (*Element)(node)
				// Per HTML spec, template element children go into the template contents
				// instead of being direct children of the template element
				if el.LocalName() == "template" && el.NamespaceURI() == HTMLNamespace {
					// Get or create the template content DocumentFragment
					content := el.TemplateContent()
					// Convert children into the template content
					convertHTMLTree(c, content.AsNode(), doc)
				} else {
					convertHTMLTree(c, node, doc)
				}
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

// GetSelection returns the Selection object for this document.
// Per the Selection API spec, each document has an associated Selection.
func (d *Document) GetSelection() *Selection {
	if d.AsNode().documentData.selection == nil {
		d.AsNode().documentData.selection = NewSelection(d)
	}
	return d.AsNode().documentData.selection
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
	// Per spec: "Create a Text node, and set its data attribute to the string given by
	// the method's argument (which could be the empty string). Append it to the title element."
	// This means we ALWAYS create a text node when title is provided, even for empty strings.
	if title != nil {
		titleEl := doc.CreateElement("title")
		textNode := doc.CreateTextNode(*title)
		titleEl.AsNode().AppendChild(textNode)
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
// Per WPT name-validation.html, invalid doctype names contain:
// NULL (0x00), tab (0x09), newline (0x0A), form feed (0x0C), carriage return (0x0D),
// space (0x20), or greater than (0x3E)
func (impl *DOMImplementation) CreateDocumentType(qualifiedName, publicId, systemId string) (*Node, error) {
	// Validate doctype name - reject NULL, ASCII whitespace, and >
	for _, ch := range qualifiedName {
		if ch == 0 || ch == '\t' || ch == '\n' || ch == '\f' || ch == '\r' ||
			ch == ' ' || ch == '>' {
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

// ActiveElement returns the currently focused element in the document.
// Per HTML spec, returns body element (or null if no body) when no element has focus.
func (d *Document) ActiveElement() *Element {
	n := d.AsNode()
	if n.documentData == nil {
		return nil
	}
	if n.documentData.focusedElement != nil {
		return (*Element)(n.documentData.focusedElement)
	}
	// Default to body when no element is focused
	return d.Body()
}

// SetFocusedElement sets the currently focused element.
// This is called by the JavaScript bindings when focus changes.
// Pass nil to indicate no element has focus (falls back to body).
func (d *Document) SetFocusedElement(el *Element) {
	n := d.AsNode()
	if n.documentData == nil {
		return
	}
	if el != nil {
		n.documentData.focusedElement = el.AsNode()
	} else {
		n.documentData.focusedElement = nil
	}
}

// GetFocusedElement returns the raw focused element pointer (may be nil).
// This returns nil if focus is on body/document, unlike ActiveElement which returns body.
func (d *Document) GetFocusedElement() *Element {
	n := d.AsNode()
	if n.documentData == nil {
		return nil
	}
	if n.documentData.focusedElement != nil {
		return (*Element)(n.documentData.focusedElement)
	}
	return nil
}

// DocumentNamedItem represents the result of a document named item access.
// It can be a single element or a collection of elements.
type DocumentNamedItem struct {
	Element    *Element        // Single element (when only one matches)
	Collection *HTMLCollection // Collection (when multiple match)
}

// isNamedAccessibleElement returns true if the element type is one that
// exposes names to document named item access per the HTML spec.
// Per https://html.spec.whatwg.org/multipage/dom.html#dom-document-nameditem
// The exposed elements are: embed, form, iframe, img, object (with name)
// and object (with id), img (with both name and id).
func isNamedAccessibleElement(el *Element) bool {
	if el.NamespaceURI() != HTMLNamespace {
		return false
	}
	localName := strings.ToLower(el.LocalName())
	switch localName {
	case "embed", "form", "iframe", "img", "object":
		return true
	default:
		return false
	}
}

// shouldExposeNameAttribute returns true if the element exposes its name attribute.
// Per the HTML spec, embed, form, iframe, img, and object expose their name attribute.
func shouldExposeNameAttribute(el *Element) bool {
	localName := strings.ToLower(el.LocalName())
	switch localName {
	case "embed", "form", "iframe", "img", "object":
		return true
	default:
		return false
	}
}

// shouldExposeIdAttribute returns true if the element exposes its id attribute
// for document named property access.
// Per the HTML spec, only object and img (when img has name) expose id.
// Actually, per closer reading: object always exposes id, but img only exposes
// id when it also has a name attribute.
func shouldExposeIdAttribute(el *Element) bool {
	localName := strings.ToLower(el.LocalName())
	switch localName {
	case "object":
		return true
	case "img":
		// img exposes id only if it has a name attribute
		return el.GetAttribute("name") != ""
	default:
		return false
	}
}

// isInsideExcludedElement returns true if the element is inside an embed or object element.
// Elements inside these are not exposed per the HTML spec.
func isInsideExcludedElement(el *Element) bool {
	for parent := el.AsNode().parentNode; parent != nil; parent = parent.parentNode {
		if parent.nodeType == ElementNode {
			parentEl := (*Element)(parent)
			if parentEl.NamespaceURI() == HTMLNamespace {
				localName := strings.ToLower(parentEl.LocalName())
				if localName == "embed" || localName == "object" {
					return true
				}
			}
		}
	}
	return false
}

// NamedItem returns the named item(s) with the given name from the document.
// This implements the HTML spec's "dom-document-nameditem" algorithm.
// Returns nil if no matching elements are found.
// Per the HTML spec:
// - Returns a single Element if exactly one element matches
// - Returns an HTMLCollection if multiple elements match
// - For iframes, the Window (contentWindow) should be returned, but that's
//   handled at the JavaScript binding level.
func (d *Document) NamedItem(name string) *DocumentNamedItem {
	if name == "" {
		return nil
	}

	var matches []*Element

	// Traverse the document tree looking for matching elements
	d.collectNamedElements(d.AsNode(), name, &matches)

	if len(matches) == 0 {
		return nil
	}

	if len(matches) == 1 {
		return &DocumentNamedItem{Element: matches[0]}
	}

	// Multiple matches - return as a collection
	collection := newHTMLCollection(d.AsNode(), func(el *Element) bool {
		// Check if this element matches the name
		if !isNamedAccessibleElement(el) {
			return false
		}
		if isInsideExcludedElement(el) {
			return false
		}
		// Check name attribute
		if shouldExposeNameAttribute(el) && el.GetAttribute("name") == name {
			return true
		}
		// Check id attribute
		if shouldExposeIdAttribute(el) && el.Id() == name {
			return true
		}
		return false
	})

	return &DocumentNamedItem{Collection: collection}
}

// collectNamedElements collects elements that match the given name.
func (d *Document) collectNamedElements(node *Node, name string, matches *[]*Element) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)

			// Skip elements inside excluded containers
			if isInsideExcludedElement(el) {
				continue
			}

			if isNamedAccessibleElement(el) {
				// Check name attribute
				if shouldExposeNameAttribute(el) && el.GetAttribute("name") == name {
					*matches = append(*matches, el)
				} else if shouldExposeIdAttribute(el) && el.Id() == name {
					// Check id attribute (only for object, and img with name)
					*matches = append(*matches, el)
				}
			}

			// Recurse into children (but not into embed/object)
			localName := strings.ToLower(el.LocalName())
			if el.NamespaceURI() != HTMLNamespace || (localName != "embed" && localName != "object") {
				d.collectNamedElements(child, name, matches)
			}
		}
	}
}

// NamedProperties returns all named properties exposed by the document.
// This returns names in tree order, suitable for Object.getOwnPropertyNames().
func (d *Document) NamedProperties() []string {
	seen := make(map[string]bool)
	var names []string

	d.collectNamedProperties(d.AsNode(), seen, &names)

	return names
}

// collectNamedProperties collects all exposed named property names.
func (d *Document) collectNamedProperties(node *Node, seen map[string]bool, names *[]string) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)

			// Skip elements inside excluded containers
			if isInsideExcludedElement(el) {
				continue
			}

			if isNamedAccessibleElement(el) {
				// Check name attribute
				if shouldExposeNameAttribute(el) {
					name := el.GetAttribute("name")
					if name != "" && !seen[name] {
						seen[name] = true
						*names = append(*names, name)
					}
				}
				// Check id attribute (only for object, and img with name)
				if shouldExposeIdAttribute(el) {
					id := el.Id()
					if id != "" && !seen[id] {
						seen[id] = true
						*names = append(*names, id)
					}
				}
			}

			// Recurse into children (but not into embed/object)
			localName := strings.ToLower(el.LocalName())
			if el.NamespaceURI() != HTMLNamespace || (localName != "embed" && localName != "object") {
				d.collectNamedProperties(child, seen, names)
			}
		}
	}
}

// HasNamedItem returns true if the document has a named item with the given name.
func (d *Document) HasNamedItem(name string) bool {
	return d.NamedItem(name) != nil
}
