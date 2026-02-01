package dom

import "strings"

// NamedNodeMap represents a collection of Attr objects. It is used for the
// Element.attributes property.
type NamedNodeMap struct {
	ownerElement *Element
	attrs        []*Attr
}

// newNamedNodeMap creates a new NamedNodeMap for the given element.
func newNamedNodeMap(element *Element) *NamedNodeMap {
	return &NamedNodeMap{
		ownerElement: element,
		attrs:        make([]*Attr, 0),
	}
}

// Length returns the number of attributes in the map.
func (nm *NamedNodeMap) Length() int {
	return len(nm.attrs)
}

// Item returns the attribute at the given index, or nil if out of bounds.
func (nm *NamedNodeMap) Item(index int) *Attr {
	if index < 0 || index >= len(nm.attrs) {
		return nil
	}
	return nm.attrs[index]
}

// GetNamedItem returns the attribute with the given name, or nil if not found.
func (nm *NamedNodeMap) GetNamedItem(name string) *Attr {
	for _, attr := range nm.attrs {
		if attr.name == name {
			return attr
		}
	}
	return nil
}

// GetNamedItemNS returns the attribute with the given namespace and local name.
func (nm *NamedNodeMap) GetNamedItemNS(namespaceURI, localName string) *Attr {
	for _, attr := range nm.attrs {
		if attr.namespaceURI == namespaceURI && attr.localName == localName {
			return attr
		}
	}
	return nil
}

// SetNamedItem adds or replaces an attribute.
// Returns the replaced attribute if any, or nil.
func (nm *NamedNodeMap) SetNamedItem(attr *Node) *Attr {
	if attr == nil || attr.nodeType != AttributeNode {
		return nil
	}

	// Create an Attr from the Node
	newAttr := &Attr{
		ownerElement: nm.ownerElement,
		name:         attr.nodeName,
		localName:    attr.nodeName,
		value:        attr.NodeValue(),
	}

	return nm.setAttr(newAttr)
}

// SetNamedItemNS adds or replaces an attribute with namespace support.
func (nm *NamedNodeMap) SetNamedItemNS(attr *Node) *Attr {
	return nm.SetNamedItem(attr)
}

// setAttr is the internal method to add or replace an attribute.
// Per the DOM spec, attributes are identified by namespace + localName,
// not by qualified name.
func (nm *NamedNodeMap) setAttr(attr *Attr) *Attr {
	if attr == nil {
		return nil
	}

	attr.ownerElement = nm.ownerElement

	// Check if an attribute with this namespace + localName already exists
	for i, existing := range nm.attrs {
		if existing.namespaceURI == attr.namespaceURI && existing.localName == attr.localName {
			oldValue := existing.value
			nm.attrs[i] = attr
			existing.ownerElement = nil
			// Notify about attribute change (use localName per DOM spec)
			if nm.ownerElement != nil {
				notifyAttributeMutation(nm.ownerElement.AsNode(), attr.localName, attr.namespaceURI, oldValue)
			}
			return existing
		}
	}

	// Add new attribute
	nm.attrs = append(nm.attrs, attr)
	// Notify about new attribute (old value is empty for new attributes, use localName per DOM spec)
	if nm.ownerElement != nil {
		notifyAttributeMutation(nm.ownerElement.AsNode(), attr.localName, attr.namespaceURI, "")
	}
	return nil
}

// SetAttr adds or replaces an attribute using an Attr object.
func (nm *NamedNodeMap) SetAttr(attr *Attr) *Attr {
	return nm.setAttr(attr)
}

// RemoveNamedItem removes the attribute with the given name.
// Returns the removed attribute.
func (nm *NamedNodeMap) RemoveNamedItem(name string) *Attr {
	for i, attr := range nm.attrs {
		if attr.name == name {
			oldValue := attr.value
			nm.attrs = append(nm.attrs[:i], nm.attrs[i+1:]...)
			// Notify about attribute removal before clearing owner (use localName per DOM spec)
			if nm.ownerElement != nil {
				notifyAttributeMutation(nm.ownerElement.AsNode(), attr.localName, attr.namespaceURI, oldValue)
			}
			attr.ownerElement = nil
			return attr
		}
	}
	return nil
}

// RemoveNamedItemNS removes the attribute with the given namespace and local name.
func (nm *NamedNodeMap) RemoveNamedItemNS(namespaceURI, localName string) *Attr {
	for i, attr := range nm.attrs {
		if attr.namespaceURI == namespaceURI && attr.localName == localName {
			oldValue := attr.value
			nm.attrs = append(nm.attrs[:i], nm.attrs[i+1:]...)
			// Notify about attribute removal before clearing owner (use localName per DOM spec)
			if nm.ownerElement != nil {
				notifyAttributeMutation(nm.ownerElement.AsNode(), attr.localName, namespaceURI, oldValue)
			}
			attr.ownerElement = nil
			return attr
		}
	}
	return nil
}

// GetValue returns the value of the attribute with the given name, or empty string.
func (nm *NamedNodeMap) GetValue(name string) string {
	attr := nm.GetNamedItem(name)
	if attr != nil {
		return attr.value
	}
	return ""
}

// SetValue sets the value of the attribute with the given name.
// If the attribute doesn't exist, it is created.
func (nm *NamedNodeMap) SetValue(name, value string) {
	attr := nm.GetNamedItem(name)
	if attr != nil {
		oldValue := attr.value
		attr.value = value
		// Notify about attribute change (use localName per DOM spec)
		if nm.ownerElement != nil {
			notifyAttributeMutation(nm.ownerElement.AsNode(), attr.localName, attr.namespaceURI, oldValue)
		}
	} else {
		nm.setAttr(NewAttr(name, value))
	}
}

// Has returns true if an attribute with the given name exists.
func (nm *NamedNodeMap) Has(name string) bool {
	return nm.GetNamedItem(name) != nil
}

// HasNS returns true if an attribute with the given namespace and local name exists.
func (nm *NamedNodeMap) HasNS(namespaceURI, localName string) bool {
	return nm.GetNamedItemNS(namespaceURI, localName) != nil
}

// Names returns a slice of all attribute names.
func (nm *NamedNodeMap) Names() []string {
	names := make([]string, len(nm.attrs))
	for i, attr := range nm.attrs {
		names[i] = attr.name
	}
	return names
}

// OwnerElement returns the element that owns this NamedNodeMap.
func (nm *NamedNodeMap) OwnerElement() *Element {
	return nm.ownerElement
}

// Clone creates a deep copy of this NamedNodeMap.
func (nm *NamedNodeMap) Clone(newOwner *Element) *NamedNodeMap {
	clone := newNamedNodeMap(newOwner)
	for _, attr := range nm.attrs {
		newAttr := &Attr{
			ownerElement: newOwner,
			namespaceURI: attr.namespaceURI,
			prefix:       attr.prefix,
			localName:    attr.localName,
			name:         attr.name,
			value:        attr.value,
		}
		clone.attrs = append(clone.attrs, newAttr)
	}
	return clone
}

// parseQualifiedName splits a qualified name into prefix and local name.
func parseQualifiedName(qualifiedName string) (prefix, localName string) {
	if idx := strings.Index(qualifiedName, ":"); idx >= 0 {
		return qualifiedName[:idx], qualifiedName[idx+1:]
	}
	return "", qualifiedName
}
