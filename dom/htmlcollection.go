package dom

import "strings"

// HTMLCollection represents a live collection of elements. Unlike NodeList,
// HTMLCollection only contains Element nodes.
type HTMLCollection struct {
	// The root element to search from
	root *Node

	// Filter function that determines which elements are included
	filter func(*Element) bool

	// For named access - maps name/id to elements
	// This is computed lazily when needed
}

// newHTMLCollection creates a new HTMLCollection with the given root and filter.
func newHTMLCollection(root *Node, filter func(*Element) bool) *HTMLCollection {
	return &HTMLCollection{
		root:   root,
		filter: filter,
	}
}

// NewHTMLCollectionByTagName creates an HTMLCollection of elements with the given tag name.
func NewHTMLCollectionByTagName(root *Node, tagName string) *HTMLCollection {
	tagName = strings.ToUpper(tagName)
	return newHTMLCollection(root, func(el *Element) bool {
		if tagName == "*" {
			return true
		}
		return el.TagName() == tagName
	})
}

// NewHTMLCollectionByClassName creates an HTMLCollection of elements with the given class name(s).
func NewHTMLCollectionByClassName(root *Node, classNames string) *HTMLCollection {
	classes := strings.Fields(classNames)
	return newHTMLCollection(root, func(el *Element) bool {
		classList := el.ClassList()
		for _, class := range classes {
			if !classList.Contains(class) {
				return false
			}
		}
		return true
	})
}

// collectElements traverses the DOM tree and collects matching elements.
func (hc *HTMLCollection) collectElements() []*Element {
	var elements []*Element
	hc.traverse(hc.root, &elements)
	return elements
}

func (hc *HTMLCollection) traverse(node *Node, elements *[]*Element) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if hc.filter(el) {
				*elements = append(*elements, el)
			}
			hc.traverse(child, elements)
		}
	}
}

// Length returns the number of elements in the collection.
func (hc *HTMLCollection) Length() int {
	return len(hc.collectElements())
}

// Item returns the element at the given index, or nil if out of bounds.
func (hc *HTMLCollection) Item(index int) *Element {
	elements := hc.collectElements()
	if index < 0 || index >= len(elements) {
		return nil
	}
	return elements[index]
}

// NamedItem returns the element with the given id or name attribute.
// If multiple elements match, returns the first one.
// Note: The `name` attribute is only checked for elements in the HTML namespace.
func (hc *HTMLCollection) NamedItem(name string) *Element {
	elements := hc.collectElements()
	// First look for id match
	for _, el := range elements {
		if el.Id() == name {
			return el
		}
	}
	// Then look for name attribute match (only for HTML namespace elements)
	for _, el := range elements {
		ns := el.AsNode().elementData.namespaceURI
		if ns == HTMLNamespace && el.GetAttribute("name") == name {
			return el
		}
	}
	return nil
}

// ToSlice returns all elements as a slice.
func (hc *HTMLCollection) ToSlice() []*Element {
	return hc.collectElements()
}

// NamedProperty represents a named property with its name and element.
type NamedProperty struct {
	Name    string
	Element *Element
}

// NamedProperties returns an ordered list of all named properties (id and name attributes)
// that should be exposed on this collection. According to the HTML spec,
// only HTML namespace elements expose their `name` attribute; non-HTML elements
// only expose `id`. The order is preserved (first occurrence in tree order wins).
func (hc *HTMLCollection) NamedProperties() []NamedProperty {
	var result []NamedProperty
	seen := make(map[string]bool)
	elements := hc.collectElements()

	// Per the spec, we iterate in tree order and only add the first element
	// for each name. We process both id and name in the same pass to maintain
	// the correct tree order.
	for _, el := range elements {
		// Check id first
		id := el.Id()
		if id != "" && !seen[id] {
			result = append(result, NamedProperty{Name: id, Element: el})
			seen[id] = true
		}

		// Check name attribute (only for HTML namespace elements)
		// Note: Elements created with createElementNS("", ...) have empty namespace,
		// which is NOT the HTML namespace.
		ns := el.AsNode().elementData.namespaceURI
		isHTML := ns == HTMLNamespace

		if isHTML {
			name := el.GetAttribute("name")
			if name != "" && !seen[name] {
				result = append(result, NamedProperty{Name: name, Element: el})
				seen[name] = true
			}
		}
	}

	return result
}
