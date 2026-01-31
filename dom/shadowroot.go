package dom

import "strings"

// ShadowRootMode indicates whether the shadow root is open or closed.
type ShadowRootMode string

const (
	// ShadowRootModeOpen means the shadow root's internal features are accessible
	// from JavaScript, e.g. using Element.shadowRoot.
	ShadowRootModeOpen ShadowRootMode = "open"
	// ShadowRootModeClosed means the shadow root's internal features are inaccessible
	// from JavaScript.
	ShadowRootModeClosed ShadowRootMode = "closed"
)

// shadowRootData holds data specific to ShadowRoot nodes.
type shadowRootData struct {
	mode              ShadowRootMode
	host              *Element // The element that hosts this shadow root
	delegatesFocus    bool
	slotAssignment    string // "manual" or "named"
	clonable          bool
	serializable      bool
}

// ShadowRoot represents a shadow root in the DOM tree.
// A shadow root is the root of a shadow tree. It is a DocumentFragment-like
// node that encapsulates a DOM tree.
type ShadowRoot struct {
	node *Node // Underlying node (uses DocumentFragmentNode type)
	data *shadowRootData
}

// NewShadowRoot creates a new shadow root attached to the given host element.
func NewShadowRoot(host *Element, mode ShadowRootMode, options map[string]interface{}) *ShadowRoot {
	// Create a document fragment node for the shadow root
	ownerDoc := host.AsNode().ownerDoc
	node := newNode(DocumentFragmentNode, "#document-fragment", ownerDoc)

	data := &shadowRootData{
		mode:           mode,
		host:           host,
		slotAssignment: "named", // default
	}

	// Handle options
	if options != nil {
		if df, ok := options["delegatesFocus"].(bool); ok {
			data.delegatesFocus = df
		}
		if sa, ok := options["slotAssignment"].(string); ok {
			data.slotAssignment = sa
		}
		if c, ok := options["clonable"].(bool); ok {
			data.clonable = c
		}
		if s, ok := options["serializable"].(bool); ok {
			data.serializable = s
		}
	}

	sr := &ShadowRoot{
		node: node,
		data: data,
	}

	// Store back-reference to shadow root in the node
	// We use a field in Node to track this
	node.shadowRoot = sr

	return sr
}

// AsNode returns the underlying Node.
func (sr *ShadowRoot) AsNode() *Node {
	return sr.node
}

// NodeType returns DocumentFragmentNode (11).
// Per spec, ShadowRoot has nodeType = 11 (DOCUMENT_FRAGMENT_NODE).
func (sr *ShadowRoot) NodeType() NodeType {
	return DocumentFragmentNode
}

// NodeName returns "#document-fragment".
func (sr *ShadowRoot) NodeName() string {
	return "#document-fragment"
}

// Mode returns the mode of this shadow root ("open" or "closed").
func (sr *ShadowRoot) Mode() ShadowRootMode {
	return sr.data.mode
}

// Host returns the element that hosts this shadow root.
func (sr *ShadowRoot) Host() *Element {
	return sr.data.host
}

// DelegatesFocus returns whether this shadow root delegates focus.
func (sr *ShadowRoot) DelegatesFocus() bool {
	return sr.data.delegatesFocus
}

// SlotAssignment returns the slot assignment mode.
func (sr *ShadowRoot) SlotAssignment() string {
	return sr.data.slotAssignment
}

// Clonable returns whether the shadow root is clonable.
func (sr *ShadowRoot) Clonable() bool {
	return sr.data.clonable
}

// Serializable returns whether the shadow root is serializable.
func (sr *ShadowRoot) Serializable() bool {
	return sr.data.serializable
}

// OwnerDocument returns the owner document.
// Per spec, this is the host element's node document.
func (sr *ShadowRoot) OwnerDocument() *Document {
	return sr.node.ownerDoc
}

// Children returns an HTMLCollection of child elements.
func (sr *ShadowRoot) Children() *HTMLCollection {
	return newHTMLCollection(sr.node, func(el *Element) bool {
		return el.AsNode().parentNode == sr.node
	})
}

// ChildElementCount returns the number of child elements.
func (sr *ShadowRoot) ChildElementCount() int {
	count := 0
	for child := sr.node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			count++
		}
	}
	return count
}

// FirstElementChild returns the first child element.
func (sr *ShadowRoot) FirstElementChild() *Element {
	for child := sr.node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// LastElementChild returns the last child element.
func (sr *ShadowRoot) LastElementChild() *Element {
	for child := sr.node.lastChild; child != nil; child = child.prevSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// GetElementById returns the element with the given id.
func (sr *ShadowRoot) GetElementById(id string) *Element {
	if id == "" {
		return nil
	}
	return sr.findElementById(sr.node, id)
}

func (sr *ShadowRoot) findElementById(node *Node, id string) *Element {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.Id() == id {
				return el
			}
			result := sr.findElementById(child, id)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// QuerySelector returns the first element matching the selector.
func (sr *ShadowRoot) QuerySelector(selector string) *Element {
	for child := sr.node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.Matches(selector) {
				return el
			}
			result := el.QuerySelector(selector)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// QuerySelectorAll returns all elements matching the selector.
func (sr *ShadowRoot) QuerySelectorAll(selector string) *NodeList {
	var results []*Node

	var traverse func(*Node)
	traverse = func(node *Node) {
		for child := node.firstChild; child != nil; child = child.nextSibling {
			if child.nodeType == ElementNode {
				el := (*Element)(child)
				if el.Matches(selector) {
					results = append(results, child)
				}
				traverse(child)
			}
		}
	}
	traverse(sr.node)

	return NewStaticNodeList(results)
}

// InnerHTML gets or sets the HTML content inside the shadow root.
func (sr *ShadowRoot) InnerHTML() string {
	// Use the same serialization as Element
	if sr.node.firstChild == nil {
		return ""
	}
	// Serialize all children
	var sb strings.Builder
	for child := sr.node.firstChild; child != nil; child = child.nextSibling {
		serializeNode(child, &sb)
	}
	return sb.String()
}

// SetInnerHTML sets the HTML content inside the shadow root.
func (sr *ShadowRoot) SetInnerHTML(htmlContent string) {
	// Remove all existing children
	for sr.node.firstChild != nil {
		sr.node.RemoveChild(sr.node.firstChild)
	}

	// Parse the HTML and append the nodes
	if htmlContent != "" && sr.node.ownerDoc != nil {
		// Create a temporary context element for parsing
		contextEl := sr.data.host
		if contextEl == nil {
			// Use a div as fallback context
			contextEl = sr.node.ownerDoc.CreateElement("div")
		}

		nodes, err := parseHTMLFragment(htmlContent, contextEl)
		if err != nil {
			return
		}

		for _, node := range nodes {
			sr.node.AppendChild(node)
		}
	}
}

// Append appends nodes or strings to this shadow root.
func (sr *ShadowRoot) Append(nodes ...interface{}) {
	for _, item := range nodes {
		switch v := item.(type) {
		case *Node:
			sr.node.AppendChild(v)
		case *Element:
			sr.node.AppendChild(v.AsNode())
		case string:
			sr.node.AppendChild(sr.node.ownerDoc.CreateTextNode(v))
		}
	}
}

// Prepend prepends nodes or strings to this shadow root.
func (sr *ShadowRoot) Prepend(nodes ...interface{}) {
	firstChild := sr.node.firstChild
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = sr.node.ownerDoc.CreateTextNode(v)
		}
		if node != nil {
			sr.node.InsertBefore(node, firstChild)
		}
	}
}

// ReplaceChildren replaces all children with the given nodes.
func (sr *ShadowRoot) ReplaceChildren(nodes ...interface{}) {
	// Remove all existing children
	for sr.node.firstChild != nil {
		sr.node.RemoveChild(sr.node.firstChild)
	}
	// Add new nodes
	sr.Append(nodes...)
}

// IsShadowRoot returns true. Used for type checking.
func (sr *ShadowRoot) IsShadowRoot() bool {
	return true
}
