package dom

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Element represents an element in the DOM tree.
// Element inherits from Node and provides element-specific properties and methods.
type Element Node

// AsNode returns the underlying Node.
func (e *Element) AsNode() *Node {
	return (*Node)(e)
}

// NodeType returns ElementNode (1).
func (e *Element) NodeType() NodeType {
	return ElementNode
}

// NodeName returns the tag name.
// For HTML namespace elements, this returns the uppercase version (same as tagName).
// For other namespaces (SVG, MathML), the original case is preserved.
func (e *Element) NodeName() string {
	return e.TagName()
}

// TagName returns the tag name in uppercase (for HTML elements).
func (e *Element) TagName() string {
	if e.AsNode().elementData != nil {
		return e.AsNode().elementData.tagName
	}
	return strings.ToUpper(e.AsNode().nodeName)
}

// LocalName returns the local name of the element (lowercase for HTML).
func (e *Element) LocalName() string {
	if e.AsNode().elementData != nil {
		return e.AsNode().elementData.localName
	}
	return strings.ToLower(e.AsNode().nodeName)
}

// NamespaceURI returns the namespace URI of the element.
func (e *Element) NamespaceURI() string {
	if e.AsNode().elementData != nil {
		return e.AsNode().elementData.namespaceURI
	}
	return ""
}

// Prefix returns the namespace prefix of the element.
func (e *Element) Prefix() string {
	if e.AsNode().elementData != nil {
		return e.AsNode().elementData.prefix
	}
	return ""
}

// isHTMLElementInHTMLDocument returns true if this element is an HTML element
// in an HTML document. This is used for case-insensitive attribute handling.
func (e *Element) isHTMLElementInHTMLDocument() bool {
	// Check if in HTML namespace
	if e.NamespaceURI() != HTMLNamespace {
		return false
	}
	// Check if owner document is an HTML document
	doc := e.AsNode().ownerDoc
	if doc == nil {
		return false
	}
	return doc.IsHTML()
}

// Id returns the id attribute value.
func (e *Element) Id() string {
	return e.GetAttribute("id")
}

// SetId sets the id attribute value.
func (e *Element) SetId(id string) {
	e.SetAttribute("id", id)
}

// ClassName returns the class attribute value.
func (e *Element) ClassName() string {
	return e.GetAttribute("class")
}

// SetClassName sets the class attribute value.
func (e *Element) SetClassName(className string) {
	e.SetAttribute("class", className)
}

// ClassList returns a DOMTokenList for the class attribute.
func (e *Element) ClassList() *DOMTokenList {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.classList == nil {
		e.AsNode().elementData.classList = newDOMTokenList(e, "class")
	}
	return e.AsNode().elementData.classList
}

// Attributes returns the NamedNodeMap of attributes.
func (e *Element) Attributes() *NamedNodeMap {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.attributes == nil {
		e.AsNode().elementData.attributes = newNamedNodeMap(e)
	}
	return e.AsNode().elementData.attributes
}

// GetAttribute returns the value of the attribute with the given name.
// For HTML elements in an HTML document, the name is lowercased before lookup.
func (e *Element) GetAttribute(name string) string {
	// For HTML elements in an HTML document, lowercase the name
	if e.isHTMLElementInHTMLDocument() {
		name = strings.ToLower(name)
	}
	return e.Attributes().GetValue(name)
}

// GetAttributeNS returns the value of the attribute with the given namespace and local name.
func (e *Element) GetAttributeNS(namespaceURI, localName string) string {
	attr := e.Attributes().GetNamedItemNS(namespaceURI, localName)
	if attr != nil {
		return attr.value
	}
	return ""
}

// SetAttribute sets the value of the attribute with the given name.
// For HTML elements in an HTML document, the name is lowercased.
func (e *Element) SetAttribute(name, value string) {
	e.SetAttributeWithError(name, value)
}

// SetAttributeWithError sets the value of the attribute with the given name.
// Returns an error if the name is invalid.
func (e *Element) SetAttributeWithError(name, value string) error {
	// Step 1: Validate name using DOM spec's "valid attribute local name" rules
	if !IsValidAttributeLocalName(name) {
		return ErrInvalidCharacter("The string contains invalid characters.")
	}

	// Step 2: For HTML elements in an HTML document, lowercase the name
	if e.isHTMLElementInHTMLDocument() {
		name = strings.ToLower(name)
	}

	e.Attributes().SetValue(name, value)
	return nil
}

// IsValidAttributeLocalName checks if a string is a valid attribute local name per DOM spec.
// A string is valid if its length is at least 1 and it does not contain:
// - ASCII whitespace (tab, newline, form feed, carriage return, space)
// - U+0000 NULL
// - U+002F (/)
// - U+003D (=)
// - U+003E (>)
func IsValidAttributeLocalName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		// ASCII whitespace
		if r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r' {
			return false
		}
		// NULL, /, =, >
		if r == '\x00' || r == '/' || r == '=' || r == '>' {
			return false
		}
	}
	return true
}

// SetAttributeNS sets the value of the attribute with the given namespace.
func (e *Element) SetAttributeNS(namespaceURI, qualifiedName, value string) {
	e.SetAttributeNSWithError(namespaceURI, qualifiedName, value)
}

// SetAttributeNSWithError sets the value of the attribute with the given namespace.
// Returns an error if the qualified name or namespace is invalid.
func (e *Element) SetAttributeNSWithError(namespaceURI, qualifiedName, value string) error {
	// Step 1: Empty qualified name should throw InvalidCharacterError
	if qualifiedName == "" {
		return ErrInvalidCharacter("The string contains invalid characters.")
	}

	// Validate and extract the qualified name using the spec algorithm
	namespace, prefix, localName, err := ValidateAndExtractQualifiedName(namespaceURI, qualifiedName)
	if err != nil {
		return err
	}

	// Look for an existing attribute with the same namespace and local name
	existingAttr := e.Attributes().GetNamedItemNS(namespace, localName)
	if existingAttr != nil {
		// Just update the value, don't change the prefix
		existingAttr.value = value
		return nil
	}

	// Create new attribute with the validated values
	attr := &Attr{
		namespaceURI: namespace,
		prefix:       prefix,
		localName:    localName,
		name:         qualifiedName,
		value:        value,
	}
	e.Attributes().SetAttr(attr)
	return nil
}

// HasAttribute returns true if the element has the given attribute.
// For HTML elements in an HTML document, the name is lowercased before lookup.
func (e *Element) HasAttribute(name string) bool {
	// For HTML elements in an HTML document, lowercase the name
	if e.isHTMLElementInHTMLDocument() {
		name = strings.ToLower(name)
	}
	return e.Attributes().Has(name)
}

// HasAttributeNS returns true if the element has the attribute with the given namespace.
func (e *Element) HasAttributeNS(namespaceURI, localName string) bool {
	return e.Attributes().HasNS(namespaceURI, localName)
}

// RemoveAttribute removes the attribute with the given name.
// For HTML elements in an HTML document, the name is lowercased before lookup.
func (e *Element) RemoveAttribute(name string) {
	// For HTML elements in an HTML document, lowercase the name
	if e.isHTMLElementInHTMLDocument() {
		name = strings.ToLower(name)
	}
	e.Attributes().RemoveNamedItem(name)
}

// RemoveAttributeNS removes the attribute with the given namespace.
func (e *Element) RemoveAttributeNS(namespaceURI, localName string) {
	e.Attributes().RemoveNamedItemNS(namespaceURI, localName)
}

// ToggleAttribute toggles the presence of an attribute.
// If force is provided, it forces add (true) or remove (false).
// Returns true if the attribute is present after the operation.
func (e *Element) ToggleAttribute(name string, force ...bool) bool {
	result, _ := e.ToggleAttributeWithError(name, force...)
	return result
}

// ToggleAttributeWithError toggles the presence of an attribute.
// Returns an error if the name is invalid.
func (e *Element) ToggleAttributeWithError(name string, force ...bool) (bool, error) {
	// Step 1: Validate name using DOM spec's "valid attribute local name" rules
	if !IsValidAttributeLocalName(name) {
		return false, ErrInvalidCharacter("The string contains invalid characters.")
	}

	// Step 2: For HTML elements in an HTML document, lowercase the name
	if e.isHTMLElementInHTMLDocument() {
		name = strings.ToLower(name)
	}

	has := e.Attributes().Has(name)

	if len(force) > 0 {
		if force[0] {
			if !has {
				e.Attributes().SetValue(name, "")
			}
			return true, nil
		} else {
			if has {
				e.Attributes().RemoveNamedItem(name)
			}
			return false, nil
		}
	}

	if has {
		e.Attributes().RemoveNamedItem(name)
		return false, nil
	}
	e.Attributes().SetValue(name, "")
	return true, nil
}

// GetAttributeNode returns the Attr for the given attribute name.
func (e *Element) GetAttributeNode(name string) *Attr {
	return e.Attributes().GetNamedItem(name)
}

// GetAttributeNodeNS returns the Attr for the given namespace and local name.
func (e *Element) GetAttributeNodeNS(namespaceURI, localName string) *Attr {
	return e.Attributes().GetNamedItemNS(namespaceURI, localName)
}

// SetAttributeNode sets an attribute node.
func (e *Element) SetAttributeNode(attr *Attr) *Attr {
	result, _ := e.SetAttributeNodeWithError(attr)
	return result
}

// SetAttributeNodeWithError sets an attribute node.
// Returns an error if the attr is already owned by another element.
func (e *Element) SetAttributeNodeWithError(attr *Attr) (*Attr, error) {
	if attr == nil {
		return nil, nil
	}
	// Check if the attribute is already owned by another element
	if attr.ownerElement != nil && attr.ownerElement != e {
		return nil, ErrInUseAttribute("The attribute is already in use by another element.")
	}
	return e.Attributes().SetAttr(attr), nil
}

// SetAttributeNodeNS sets an attribute node with namespace support.
func (e *Element) SetAttributeNodeNS(attr *Attr) *Attr {
	result, _ := e.SetAttributeNodeNSWithError(attr)
	return result
}

// SetAttributeNodeNSWithError sets an attribute node with namespace support.
// Returns an error if the attr is already owned by another element.
func (e *Element) SetAttributeNodeNSWithError(attr *Attr) (*Attr, error) {
	if attr == nil {
		return nil, nil
	}
	// Check if the attribute is already owned by another element
	if attr.ownerElement != nil && attr.ownerElement != e {
		return nil, ErrInUseAttribute("The attribute is already in use by another element.")
	}
	return e.Attributes().SetAttr(attr), nil
}

// RemoveAttributeNode removes an attribute node.
func (e *Element) RemoveAttributeNode(attr *Attr) *Attr {
	if attr == nil {
		return nil
	}
	return e.Attributes().RemoveNamedItem(attr.name)
}

// Children returns an HTMLCollection of child elements.
func (e *Element) Children() *HTMLCollection {
	return newHTMLCollection(e.AsNode(), func(el *Element) bool {
		return el.AsNode().parentNode == e.AsNode()
	})
}

// ChildElementCount returns the number of child elements.
func (e *Element) ChildElementCount() int {
	count := 0
	for child := e.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			count++
		}
	}
	return count
}

// FirstElementChild returns the first child element.
func (e *Element) FirstElementChild() *Element {
	for child := e.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// LastElementChild returns the last child element.
func (e *Element) LastElementChild() *Element {
	for child := e.AsNode().lastChild; child != nil; child = child.prevSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// PreviousElementSibling returns the previous sibling element.
func (e *Element) PreviousElementSibling() *Element {
	for sibling := e.AsNode().prevSibling; sibling != nil; sibling = sibling.prevSibling {
		if sibling.nodeType == ElementNode {
			return (*Element)(sibling)
		}
	}
	return nil
}

// NextElementSibling returns the next sibling element.
func (e *Element) NextElementSibling() *Element {
	for sibling := e.AsNode().nextSibling; sibling != nil; sibling = sibling.nextSibling {
		if sibling.nodeType == ElementNode {
			return (*Element)(sibling)
		}
	}
	return nil
}

// GetElementsByTagName returns an HTMLCollection of descendants with the given tag name.
func (e *Element) GetElementsByTagName(tagName string) *HTMLCollection {
	return NewHTMLCollectionByTagName(e.AsNode(), tagName)
}

// GetElementsByTagNameNS returns an HTMLCollection of descendants with the given namespace and local name.
func (e *Element) GetElementsByTagNameNS(namespaceURI, localName string) *HTMLCollection {
	return newHTMLCollection(e.AsNode(), func(el *Element) bool {
		if localName != "*" && el.LocalName() != localName {
			return false
		}
		if namespaceURI != "*" && el.NamespaceURI() != namespaceURI {
			return false
		}
		return true
	})
}

// GetElementsByClassName returns an HTMLCollection of descendants with the given class name(s).
func (e *Element) GetElementsByClassName(classNames string) *HTMLCollection {
	return NewHTMLCollectionByClassName(e.AsNode(), classNames)
}

// QuerySelector returns the first descendant element matching the selector.
func (e *Element) QuerySelector(selector string) *Element {
	results := e.querySelectorAll(selector, true)
	if len(results) > 0 {
		return results[0]
	}
	return nil
}

// QuerySelectorAll returns all descendant elements matching the selector.
func (e *Element) QuerySelectorAll(selector string) *NodeList {
	results := e.querySelectorAll(selector, false)
	nodes := make([]*Node, len(results))
	for i, el := range results {
		nodes[i] = el.AsNode()
	}
	return NewStaticNodeList(nodes)
}

// querySelectorAll is the internal implementation.
func (e *Element) querySelectorAll(selector string, firstOnly bool) []*Element {
	// Parse and match the selector
	// This is a simplified implementation - full CSS selector support is complex
	var results []*Element
	e.traverseForSelector(e.AsNode(), selector, firstOnly, &results)
	return results
}

func (e *Element) traverseForSelector(node *Node, selector string, firstOnly bool, results *[]*Element) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.Matches(selector) {
				*results = append(*results, el)
				if firstOnly {
					return
				}
			}
			e.traverseForSelector(child, selector, firstOnly, results)
			if firstOnly && len(*results) > 0 {
				return
			}
		}
	}
}

// Matches returns true if the element matches the given selector.
func (e *Element) Matches(selector string) bool {
	// Simplified selector matching - handles basic cases
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}

	// Handle comma-separated selectors
	if strings.Contains(selector, ",") {
		for _, part := range strings.Split(selector, ",") {
			if e.matchesSingleSelector(strings.TrimSpace(part)) {
				return true
			}
		}
		return false
	}

	return e.matchesSingleSelector(selector)
}

func (e *Element) matchesSingleSelector(selector string) bool {
	// Handle compound selectors (e.g., "div.class#id")
	// This is a simplified parser

	// Handle universal selector
	if selector == "*" {
		return true
	}

	// Handle ID selector
	if strings.HasPrefix(selector, "#") {
		id := selector[1:]
		if !strings.Contains(id, ".") && !strings.Contains(id, "[") {
			return e.Id() == id
		}
	}

	// Handle class selector
	if strings.HasPrefix(selector, ".") {
		className := selector[1:]
		if !strings.Contains(className, ".") && !strings.Contains(className, "[") {
			return e.ClassList().Contains(className)
		}
	}

	// Handle tag selector
	if !strings.ContainsAny(selector, ".#[]") {
		return strings.EqualFold(e.TagName(), strings.ToUpper(selector))
	}

	// Handle compound selectors
	return e.matchesCompoundSelector(selector)
}

func (e *Element) matchesCompoundSelector(selector string) bool {
	// Parse compound selector
	current := selector
	tagName := ""

	// Extract tag name if present
	idx := strings.IndexAny(current, ".#[")
	if idx == 0 {
		// No tag name, starts with class/id/attr
		tagName = "*"
	} else if idx > 0 {
		tagName = current[:idx]
		current = current[idx:]
	} else {
		tagName = current
		current = ""
	}

	// Check tag name
	if tagName != "*" && !strings.EqualFold(e.TagName(), strings.ToUpper(tagName)) {
		return false
	}

	// Parse and check classes and IDs
	for len(current) > 0 {
		if current[0] == '.' {
			// Class selector
			end := strings.IndexAny(current[1:], ".#[")
			var className string
			if end == -1 {
				className = current[1:]
				current = ""
			} else {
				className = current[1 : end+1]
				current = current[end+1:]
			}
			if !e.ClassList().Contains(className) {
				return false
			}
		} else if current[0] == '#' {
			// ID selector
			end := strings.IndexAny(current[1:], ".#[")
			var id string
			if end == -1 {
				id = current[1:]
				current = ""
			} else {
				id = current[1 : end+1]
				current = current[end+1:]
			}
			if e.Id() != id {
				return false
			}
		} else if current[0] == '[' {
			// Attribute selector
			end := strings.Index(current, "]")
			if end == -1 {
				return false
			}
			attrSelector := current[1:end]
			current = current[end+1:]

			if !e.matchesAttributeSelector(attrSelector) {
				return false
			}
		} else {
			break
		}
	}

	return true
}

func (e *Element) matchesAttributeSelector(selector string) bool {
	// Handle attribute presence: [attr]
	// Handle attribute value: [attr=value], [attr~=value], [attr|=value], etc.

	if strings.Contains(selector, "=") {
		// Parse operator and value
		var attrName, op, value string

		for i, r := range selector {
			if r == '=' || r == '~' || r == '|' || r == '^' || r == '$' || r == '*' {
				if i+1 < len(selector) && selector[i+1] == '=' {
					attrName = selector[:i]
					op = string(selector[i : i+2])
					value = strings.Trim(selector[i+2:], "\"'")
				} else if r == '=' {
					attrName = selector[:i]
					op = "="
					value = strings.Trim(selector[i+1:], "\"'")
				}
				break
			}
		}

		if attrName == "" {
			return false
		}

		attrValue := e.GetAttribute(attrName)
		if !e.HasAttribute(attrName) {
			return false
		}

		switch op {
		case "=":
			return attrValue == value
		case "~=":
			// Word match
			for _, word := range strings.Fields(attrValue) {
				if word == value {
					return true
				}
			}
			return false
		case "|=":
			// Hyphen-separated prefix match
			return attrValue == value || strings.HasPrefix(attrValue, value+"-")
		case "^=":
			return strings.HasPrefix(attrValue, value)
		case "$=":
			return strings.HasSuffix(attrValue, value)
		case "*=":
			return strings.Contains(attrValue, value)
		}
		return false
	}

	// Attribute presence
	return e.HasAttribute(selector)
}

// Closest returns the closest ancestor element (or self) matching the selector.
func (e *Element) Closest(selector string) *Element {
	current := e
	for current != nil {
		if current.Matches(selector) {
			return current
		}
		parent := current.AsNode().parentNode
		if parent == nil || parent.nodeType != ElementNode {
			break
		}
		current = (*Element)(parent)
	}
	return nil
}

// InnerHTML returns the HTML content of the element.
func (e *Element) InnerHTML() string {
	var sb strings.Builder
	for child := e.AsNode().firstChild; child != nil; child = child.nextSibling {
		serializeNode(child, &sb)
	}
	return sb.String()
}

// SetInnerHTML sets the HTML content of the element.
func (e *Element) SetInnerHTML(htmlContent string) error {
	// Remove all children
	for e.AsNode().firstChild != nil {
		e.AsNode().RemoveChild(e.AsNode().firstChild)
	}

	// Parse the new HTML
	if htmlContent == "" {
		return nil
	}

	// Use the HTML parser to parse the fragment
	// We need to parse in the context of this element
	doc := e.AsNode().ownerDoc
	if doc == nil {
		return nil
	}

	nodes, err := parseHTMLFragment(htmlContent, e)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		e.AsNode().AppendChild(node)
	}

	return nil
}

// OuterHTML returns the HTML of the element including the element itself.
func (e *Element) OuterHTML() string {
	var sb strings.Builder
	serializeNode(e.AsNode(), &sb)
	return sb.String()
}

// SetOuterHTML replaces this element with the parsed HTML.
func (e *Element) SetOuterHTML(htmlContent string) error {
	parent := e.AsNode().parentNode
	if parent == nil {
		return nil
	}

	doc := e.AsNode().ownerDoc
	if doc == nil {
		return nil
	}

	nodes, err := parseHTMLFragment(htmlContent, (*Element)(parent))
	if err != nil {
		return err
	}

	// Insert new nodes before this element
	for _, node := range nodes {
		parent.InsertBefore(node, e.AsNode())
	}

	// Remove this element
	parent.RemoveChild(e.AsNode())

	return nil
}

// TextContent returns the text content of the element.
func (e *Element) TextContent() string {
	return e.AsNode().TextContent()
}

// SetTextContent sets the text content of the element.
func (e *Element) SetTextContent(text string) {
	e.AsNode().SetTextContent(text)
}

// serializeNode serializes a node to HTML.
func serializeNode(n *Node, sb *strings.Builder) {
	switch n.nodeType {
	case TextNode:
		sb.WriteString(html.EscapeString(n.NodeValue()))
	case CommentNode:
		sb.WriteString("<!--")
		sb.WriteString(n.NodeValue())
		sb.WriteString("-->")
	case ElementNode:
		el := (*Element)(n)
		tagName := strings.ToLower(el.TagName())
		sb.WriteString("<")
		sb.WriteString(tagName)

		// Write attributes
		attrs := el.Attributes()
		for i := 0; i < attrs.Length(); i++ {
			attr := attrs.Item(i)
			if attr != nil {
				sb.WriteString(" ")
				sb.WriteString(attr.name)
				sb.WriteString("=\"")
				sb.WriteString(html.EscapeString(attr.value))
				sb.WriteString("\"")
			}
		}

		// Check for void elements
		if isVoidElement(tagName) {
			sb.WriteString(">")
			return
		}

		sb.WriteString(">")

		// Write children
		for child := n.firstChild; child != nil; child = child.nextSibling {
			serializeNode(child, sb)
		}

		sb.WriteString("</")
		sb.WriteString(tagName)
		sb.WriteString(">")
	case DocumentFragmentNode:
		for child := n.firstChild; child != nil; child = child.nextSibling {
			serializeNode(child, sb)
		}
	}
}

// isVoidElement returns true if the element is a void element.
func isVoidElement(tagName string) bool {
	switch tagName {
	case "area", "base", "br", "col", "embed", "hr", "img", "input",
		"link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}

// parseHTMLFragment parses an HTML fragment in the context of an element.
func parseHTMLFragment(htmlContent string, context *Element) ([]*Node, error) {
	// Use golang.org/x/net/html to parse the fragment
	tagName := strings.ToLower(context.TagName())
	contextNode := &html.Node{
		Type:     html.ElementNode,
		DataAtom: lookupAtom(tagName),
		Data:     tagName,
	}

	nodes, err := html.ParseFragment(strings.NewReader(htmlContent), contextNode)
	if err != nil {
		return nil, err
	}

	result := make([]*Node, 0, len(nodes))
	doc := context.AsNode().ownerDoc
	for _, n := range nodes {
		result = append(result, convertHTMLNode(n, doc))
	}

	return result, nil
}

// convertHTMLNode converts an html.Node to a dom.Node.
func convertHTMLNode(n *html.Node, doc *Document) *Node {
	var node *Node

	switch n.Type {
	case html.TextNode:
		node = doc.CreateTextNode(n.Data)
	case html.ElementNode:
		el := doc.CreateElement(n.Data)
		for _, attr := range n.Attr {
			el.SetAttribute(attr.Key, attr.Val)
		}
		node = el.AsNode()
	case html.CommentNode:
		node = doc.CreateComment(n.Data)
	case html.DocumentNode:
		// Shouldn't happen in fragment parsing
		node = newNode(DocumentNode, "#document", doc)
	default:
		// Fallback
		node = doc.CreateTextNode(n.Data)
	}

	// Convert children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		child := convertHTMLNode(c, doc)
		node.AppendChild(child)
	}

	return node
}

// Append appends nodes or strings to this element.
// For error handling, use AppendWithError.
func (e *Element) Append(nodes ...interface{}) {
	_ = e.AppendWithError(nodes...)
}

// AppendWithError appends nodes or strings to this element.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.append() algorithm from the DOM spec.
func (e *Element) AppendWithError(nodes ...interface{}) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes/strings into a single node (or document fragment)
	node := e.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return nil
	}

	// Append the node using the error-returning method
	_, err := e.AsNode().AppendChildWithError(node)
	return err
}

// Prepend prepends nodes or strings to this element.
// For error handling, use PrependWithError.
func (e *Element) Prepend(nodes ...interface{}) {
	_ = e.PrependWithError(nodes...)
}

// PrependWithError prepends nodes or strings to this element.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.prepend() algorithm from the DOM spec.
func (e *Element) PrependWithError(nodes ...interface{}) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes/strings into a single node (or document fragment)
	node := e.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return nil
	}

	// Insert before the first child using the error-returning method
	firstChild := e.AsNode().firstChild
	_, err := e.AsNode().InsertBeforeWithError(node, firstChild)
	return err
}

// Before inserts nodes before this element.
// Implements the ChildNode.before() algorithm from DOM spec.
func (e *Element) Before(nodes ...interface{}) {
	parent := e.AsNode().parentNode
	if parent == nil {
		return
	}
	// Step 3: Find viable previous sibling (first preceding sibling not in nodes)
	nodeSet := extractNodeSet(nodes)
	viablePrevSibling := e.AsNode().findViablePreviousSibling(nodeSet)

	// Step 4: Convert nodes into a node
	node := e.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	// Step 5: Pre-insert node into parent before this element
	// If viable previous sibling is null, insert as first child
	// Otherwise, insert after viable previous sibling
	var refNode *Node
	if viablePrevSibling == nil {
		refNode = parent.firstChild
	} else {
		refNode = viablePrevSibling.nextSibling
	}
	parent.InsertBefore(node, refNode)
}

// After inserts nodes after this element.
// Implements the ChildNode.after() algorithm from DOM spec.
func (e *Element) After(nodes ...interface{}) {
	parent := e.AsNode().parentNode
	if parent == nil {
		return
	}
	// Step 3: Find viable next sibling (first following sibling not in nodes)
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := e.AsNode().findViableNextSibling(nodeSet)

	// Step 4: Convert nodes into a node
	node := e.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	// Step 5: Pre-insert node into parent before viable next sibling
	parent.InsertBefore(node, viableNextSibling)
}

// ReplaceWith replaces this element with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (e *Element) ReplaceWith(nodes ...interface{}) {
	parent := e.AsNode().parentNode
	if parent == nil {
		return
	}
	// Step 3: Find viable next sibling (first following sibling not in nodes)
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := e.AsNode().findViableNextSibling(nodeSet)

	// Step 4: Convert nodes into a node
	node := e.AsNode().convertNodesToFragment(nodes)

	// Step 5: If this element's parent is parent, replace this with node
	// Otherwise, pre-insert node into parent before viable next sibling
	if e.AsNode().parentNode == parent {
		if node != nil {
			parent.ReplaceChild(node, e.AsNode())
		} else {
			// No replacement nodes, just remove this element
			parent.RemoveChild(e.AsNode())
		}
	} else if node != nil {
		parent.InsertBefore(node, viableNextSibling)
	}
}

// Remove removes this element from its parent.
func (e *Element) Remove() {
	if e.AsNode().parentNode != nil {
		e.AsNode().parentNode.RemoveChild(e.AsNode())
	}
}

// ReplaceChildren replaces all children with the given nodes.
// For error handling, use ReplaceChildrenWithError.
func (e *Element) ReplaceChildren(nodes ...interface{}) {
	_ = e.ReplaceChildrenWithError(nodes...)
}

// ReplaceChildrenWithError replaces all children with the given nodes.
// Returns an error if any validation fails (e.g., HierarchyRequestError).
// Implements the ParentNode.replaceChildren() algorithm from the DOM spec.
// Per spec, validation happens BEFORE any children are removed.
func (e *Element) ReplaceChildrenWithError(nodes ...interface{}) error {
	// Step 1: Convert nodes/strings into a single node (or document fragment)
	var node *Node
	if len(nodes) > 0 {
		node = e.AsNode().convertNodesToFragment(nodes)
	}

	// Step 2: Validate the insertion BEFORE removing any children
	// This ensures we throw HierarchyRequestError before any mutation
	if node != nil {
		if err := e.AsNode().validatePreInsertion(node, nil); err != nil {
			return err
		}
	}

	// Step 3: Remove all existing children
	for e.AsNode().firstChild != nil {
		e.AsNode().RemoveChild(e.AsNode().firstChild)
	}

	// Step 4: Insert the new node(s)
	if node != nil {
		e.AsNode().AppendChild(node)
	}

	return nil
}

// InsertAdjacentElement inserts an element at the specified position relative to this element.
// Position can be: "beforebegin", "afterbegin", "beforeend", "afterend"
// Returns the inserted element, or nil if insertion fails (e.g., no parent for beforebegin/afterend).
func (e *Element) InsertAdjacentElement(position string, element *Element) (*Element, error) {
	if element == nil {
		return nil, nil
	}
	node := element.AsNode()
	err := e.insertAdjacentNode(position, node)
	if err != nil {
		return nil, err
	}
	return element, nil
}

// InsertAdjacentText inserts a text node at the specified position relative to this element.
// Position can be: "beforebegin", "afterbegin", "beforeend", "afterend"
func (e *Element) InsertAdjacentText(position string, data string) error {
	doc := e.AsNode().ownerDoc
	if doc == nil {
		return ErrHierarchyRequest("element has no owner document")
	}
	textNode := doc.CreateTextNode(data)
	return e.insertAdjacentNode(position, textNode)
}

// insertAdjacentNode is the internal implementation for insertAdjacent* methods.
func (e *Element) insertAdjacentNode(position string, node *Node) error {
	// Normalize position to lowercase for case-insensitive comparison
	pos := strings.ToLower(position)

	switch pos {
	case "beforebegin":
		// Insert before this element
		parent := e.AsNode().parentNode
		if parent == nil {
			// No parent, nothing to do (per spec, we just return)
			return nil
		}
		_, err := parent.InsertBeforeWithError(node, e.AsNode())
		return err

	case "afterbegin":
		// Insert as first child
		_, err := e.AsNode().InsertBeforeWithError(node, e.AsNode().firstChild)
		return err

	case "beforeend":
		// Insert as last child (same as appendChild)
		_, err := e.AsNode().AppendChildWithError(node)
		return err

	case "afterend":
		// Insert after this element
		parent := e.AsNode().parentNode
		if parent == nil {
			// No parent, nothing to do (per spec, we just return)
			return nil
		}
		_, err := parent.InsertBeforeWithError(node, e.AsNode().nextSibling)
		return err

	default:
		// Invalid position
		return ErrSyntax("The value provided ('" + position + "') is not one of 'beforebegin', 'afterbegin', 'beforeend', or 'afterend'.")
	}
}

// CloneNode clones this element.
func (e *Element) CloneNode(deep bool) *Element {
	clonedNode := e.AsNode().CloneNode(deep)
	return (*Element)(clonedNode)
}

// Style returns the CSSStyleDeclaration for this element's inline styles.
func (e *Element) Style() *CSSStyleDeclaration {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.styleDeclaration == nil {
		e.AsNode().elementData.styleDeclaration = NewCSSStyleDeclaration(e)
	}
	return e.AsNode().elementData.styleDeclaration
}

// Geometry returns the element's layout geometry.
// Returns nil if layout has not been computed.
func (e *Element) Geometry() *ElementGeometry {
	if e.AsNode().elementData == nil {
		return nil
	}
	return e.AsNode().elementData.geometry
}

// SetGeometry sets the element's layout geometry.
// This is called by the layout engine after layout computation.
func (e *Element) SetGeometry(g *ElementGeometry) {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	e.AsNode().elementData.geometry = g
}

// GetBoundingClientRect returns a DOMRect representing the element's border box.
// If layout has not been computed, returns a zero-sized rect.
func (e *Element) GetBoundingClientRect() *DOMRect {
	geom := e.Geometry()
	if geom == nil {
		return NewDOMRect(0, 0, 0, 0)
	}
	return NewDOMRect(geom.X, geom.Y, geom.Width, geom.Height)
}

// GetClientRects returns a DOMRectList of rectangles for the element.
// For block elements, this is typically a single rectangle.
func (e *Element) GetClientRects() *DOMRectList {
	rect := e.GetBoundingClientRect()
	// For now, return a single rect (proper inline element handling would return multiple)
	return NewDOMRectList([]*DOMRect{rect})
}

// OffsetWidth returns the layout width including padding and border.
func (e *Element) OffsetWidth() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.OffsetWidth
}

// OffsetHeight returns the layout height including padding and border.
func (e *Element) OffsetHeight() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.OffsetHeight
}

// OffsetTop returns the distance from the top of the offset parent.
func (e *Element) OffsetTop() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.OffsetTop
}

// OffsetLeft returns the distance from the left of the offset parent.
func (e *Element) OffsetLeft() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.OffsetLeft
}

// OffsetParent returns the offset parent element.
func (e *Element) OffsetParent() *Element {
	geom := e.Geometry()
	if geom == nil {
		return nil
	}
	return geom.OffsetParent
}

// ClientWidth returns the inner width (content + padding) without border.
func (e *Element) ClientWidth() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ClientWidth
}

// ClientHeight returns the inner height (content + padding) without border.
func (e *Element) ClientHeight() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ClientHeight
}

// ClientTop returns the top border width.
func (e *Element) ClientTop() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ClientTop
}

// ClientLeft returns the left border width.
func (e *Element) ClientLeft() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ClientLeft
}

// ScrollWidth returns the total width of the scrollable content.
func (e *Element) ScrollWidth() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ScrollWidth
}

// ScrollHeight returns the total height of the scrollable content.
func (e *Element) ScrollHeight() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ScrollHeight
}

// ScrollTop returns the scroll offset from the top.
func (e *Element) ScrollTop() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ScrollTop
}

// SetScrollTop sets the scroll offset from the top.
func (e *Element) SetScrollTop(value float64) {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.geometry == nil {
		e.AsNode().elementData.geometry = &ElementGeometry{}
	}
	if value < 0 {
		value = 0
	}
	e.AsNode().elementData.geometry.ScrollTop = value
}

// ScrollLeft returns the scroll offset from the left.
func (e *Element) ScrollLeft() float64 {
	geom := e.Geometry()
	if geom == nil {
		return 0
	}
	return geom.ScrollLeft
}

// SetScrollLeft sets the scroll offset from the left.
func (e *Element) SetScrollLeft(value float64) {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.geometry == nil {
		e.AsNode().elementData.geometry = &ElementGeometry{}
	}
	if value < 0 {
		value = 0
	}
	e.AsNode().elementData.geometry.ScrollLeft = value
}

// lookupAtom looks up an atom for the given tag name.
func lookupAtom(tagName string) atom.Atom {
	return atom.Lookup([]byte(tagName))
}

// ShadowRoot returns the shadow root attached to this element, or nil if none.
// For closed shadow roots, this returns nil unless accessed from within the shadow tree.
func (e *Element) ShadowRoot() *ShadowRoot {
	if e.AsNode().elementData == nil {
		return nil
	}
	sr := e.AsNode().elementData.shadowRoot
	if sr == nil {
		return nil
	}
	// Per spec, closed shadow roots are not exposed via element.shadowRoot
	if sr.Mode() == ShadowRootModeClosed {
		return nil
	}
	return sr
}

// GetShadowRoot returns the shadow root attached to this element, or nil if none.
// Unlike ShadowRoot(), this returns the shadow root regardless of mode (internal use).
func (e *Element) GetShadowRoot() *ShadowRoot {
	if e.AsNode().elementData == nil {
		return nil
	}
	return e.AsNode().elementData.shadowRoot
}

// AttachShadow attaches a shadow DOM tree to this element and returns the ShadowRoot.
// Returns an error if:
// - The element is not a valid shadow host (custom element or specific HTML elements)
// - The element already has an attached shadow root
func (e *Element) AttachShadow(mode ShadowRootMode, options map[string]interface{}) (*ShadowRoot, error) {
	// Check if element can have a shadow root
	if !e.canAttachShadow() {
		return nil, ErrNotSupported("Failed to execute 'attachShadow' on 'Element': This element does not support attachShadow")
	}

	// Check if already has a shadow root
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	if e.AsNode().elementData.shadowRoot != nil {
		return nil, ErrNotSupported("Failed to execute 'attachShadow' on 'Element': Shadow root cannot be created on a host which already hosts a shadow tree.")
	}

	// Validate mode
	if mode != ShadowRootModeOpen && mode != ShadowRootModeClosed {
		return nil, ErrNotSupported("Failed to execute 'attachShadow' on 'Element': The provided value '" + string(mode) + "' is not a valid enum value of type ShadowRootMode.")
	}

	// Create the shadow root
	sr := NewShadowRoot(e, mode, options)

	// Attach it to this element
	e.AsNode().elementData.shadowRoot = sr

	return sr, nil
}

// canAttachShadow returns true if this element can have a shadow root attached.
// Per spec, valid shadow hosts are:
// - Custom elements (element names containing a hyphen)
// - article, aside, blockquote, body, div, footer, h1-h6, header, main, nav, p, section, span
func (e *Element) canAttachShadow() bool {
	localName := e.LocalName()
	ns := e.NamespaceURI()

	// Only HTML namespace elements can be shadow hosts (for built-in elements)
	if ns != "" && ns != HTMLNamespace {
		return false
	}

	// Custom elements (autonomous custom elements have a hyphen in their name)
	if strings.Contains(localName, "-") {
		return true
	}

	// Valid built-in shadow hosts
	validHosts := map[string]bool{
		"article":    true,
		"aside":      true,
		"blockquote": true,
		"body":       true,
		"div":        true,
		"footer":     true,
		"h1":         true,
		"h2":         true,
		"h3":         true,
		"h4":         true,
		"h5":         true,
		"h6":         true,
		"header":     true,
		"main":       true,
		"nav":        true,
		"p":          true,
		"section":    true,
		"span":       true,
	}

	return validHosts[localName]
}

// TemplateContent returns the template content DocumentFragment for template elements.
// Returns nil for non-template elements.
// Per HTML spec, this creates the template contents DocumentFragment on first access if needed.
func (e *Element) TemplateContent() *DocumentFragment {
	// Only template elements have template contents
	if e.LocalName() != "template" || e.NamespaceURI() != HTMLNamespace {
		return nil
	}

	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}

	// Create template contents lazily if not already created
	if e.AsNode().elementData.templateContent == nil {
		// Per spec, template contents is owned by the template element's node document
		doc := e.AsNode().ownerDoc
		node := newNode(DocumentFragmentNode, "#document-fragment", doc)
		e.AsNode().elementData.templateContent = (*DocumentFragment)(node)
	}

	return e.AsNode().elementData.templateContent
}

// SetTemplateContent sets the template content DocumentFragment for template elements.
// This is used during HTML parsing to set up template contents.
func (e *Element) SetTemplateContent(content *DocumentFragment) {
	if e.AsNode().elementData == nil {
		e.AsNode().elementData = &elementData{}
	}
	e.AsNode().elementData.templateContent = content
}
