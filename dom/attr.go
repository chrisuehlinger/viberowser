package dom

import "strings"

// Attr represents an attribute of an Element.
type Attr struct {
	ownerElement *Element
	namespaceURI string
	prefix       string
	localName    string
	name         string
	value        string
}

// NewAttr creates a new Attr with the given name and value.
func NewAttr(name, value string) *Attr {
	return &Attr{
		localName: name,
		name:      name,
		value:     value,
	}
}

// NewAttrNS creates a new Attr with the given namespace, name, and value.
func NewAttrNS(namespaceURI, qualifiedName, value string) *Attr {
	prefix := ""
	localName := qualifiedName

	if idx := strings.Index(qualifiedName, ":"); idx >= 0 {
		prefix = qualifiedName[:idx]
		localName = qualifiedName[idx+1:]
	}

	return &Attr{
		namespaceURI: namespaceURI,
		prefix:       prefix,
		localName:    localName,
		name:         qualifiedName,
		value:        value,
	}
}

// NodeType returns AttributeNode (2).
func (a *Attr) NodeType() NodeType {
	return AttributeNode
}

// NodeName returns the attribute name.
func (a *Attr) NodeName() string {
	return a.name
}

// NodeValue returns the attribute value.
func (a *Attr) NodeValue() string {
	return a.value
}

// SetNodeValue sets the attribute value.
func (a *Attr) SetNodeValue(value string) {
	a.value = value
}

// OwnerElement returns the element that owns this attribute.
func (a *Attr) OwnerElement() *Element {
	return a.ownerElement
}

// OwnerDocument returns the Document that owns this attribute.
// For Attr nodes, this is determined via the ownerElement.
func (a *Attr) OwnerDocument() *Document {
	if a.ownerElement != nil {
		return a.ownerElement.AsNode().OwnerDocument()
	}
	return nil
}

// BaseURI returns the absolute base URL of this attribute.
// For Attr nodes, this is the same as the ownerElement's baseURI,
// or the owner document's URL if no owner element.
func (a *Attr) BaseURI() string {
	if a.ownerElement != nil {
		return a.ownerElement.AsNode().BaseURI()
	}
	// For unattached attrs, return about:blank (no document context)
	return "about:blank"
}

// NamespaceURI returns the namespace URI of the attribute.
func (a *Attr) NamespaceURI() string {
	return a.namespaceURI
}

// Prefix returns the namespace prefix of the attribute.
func (a *Attr) Prefix() string {
	return a.prefix
}

// LocalName returns the local name of the attribute.
func (a *Attr) LocalName() string {
	return a.localName
}

// Name returns the qualified name of the attribute.
func (a *Attr) Name() string {
	return a.name
}

// Value returns the attribute value.
func (a *Attr) Value() string {
	return a.value
}

// SetValue sets the attribute value.
func (a *Attr) SetValue(value string) {
	a.value = value
	// Update the element's attribute if attached
	if a.ownerElement != nil {
		// The change is reflected directly since we're modifying the Attr that's stored
	}
}

// Specified always returns true (historical).
func (a *Attr) Specified() bool {
	return true
}

// CloneNode creates a copy of this attribute.
func (a *Attr) CloneNode(deep bool) *Node {
	// Attr nodes don't have children, so deep is ignored
	clone := NewAttr(a.name, a.value)
	clone.namespaceURI = a.namespaceURI
	clone.prefix = a.prefix
	clone.localName = a.localName
	// Don't copy ownerElement - the clone is unattached

	// Return as a Node - but Attr is a special case
	// In practice, cloning attrs returns an Attr, not a Node
	// This is a simplified implementation
	value := clone.value
	node := &Node{
		nodeType:  AttributeNode,
		nodeName:  clone.name,
		nodeValue: &value,
	}
	return node
}

// LookupNamespaceURI returns the namespace URI for the given prefix.
// For Attr nodes, this delegates to the owner element if connected.
// Disconnected Attrs have no namespace context and return empty for all prefixes.
func (a *Attr) LookupNamespaceURI(prefix string) string {
	// If connected to an element, delegate to the element
	// (which will handle the special xml/xmlns prefixes)
	if a.ownerElement != nil {
		return (*Node)(a.ownerElement).LookupNamespaceURI(prefix)
	}
	// Disconnected attrs have no namespace context
	return ""
}

// IsDefaultNamespace returns true if the given namespace URI is the default namespace.
func (a *Attr) IsDefaultNamespace(namespaceURI string) bool {
	defaultNS := a.LookupNamespaceURI("")
	return defaultNS == namespaceURI
}

// LookupPrefix returns the prefix associated with a given namespace URI.
func (a *Attr) LookupPrefix(namespaceURI string) string {
	if a.ownerElement != nil {
		return (*Node)(a.ownerElement).LookupPrefix(namespaceURI)
	}
	return ""
}
