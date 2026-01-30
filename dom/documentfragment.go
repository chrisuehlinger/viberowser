package dom

// DocumentFragment represents a minimal document object that has no parent.
// It is used to hold a portion of a document tree that can be moved to the document.
type DocumentFragment Node

// AsNode returns the underlying Node.
func (df *DocumentFragment) AsNode() *Node {
	return (*Node)(df)
}

// NodeType returns DocumentFragmentNode (11).
func (df *DocumentFragment) NodeType() NodeType {
	return DocumentFragmentNode
}

// NodeName returns "#document-fragment".
func (df *DocumentFragment) NodeName() string {
	return "#document-fragment"
}

// Children returns an HTMLCollection of child elements.
func (df *DocumentFragment) Children() *HTMLCollection {
	return newHTMLCollection(df.AsNode(), func(el *Element) bool {
		return el.AsNode().parentNode == df.AsNode()
	})
}

// ChildElementCount returns the number of child elements.
func (df *DocumentFragment) ChildElementCount() int {
	count := 0
	for child := df.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			count++
		}
	}
	return count
}

// FirstElementChild returns the first child element.
func (df *DocumentFragment) FirstElementChild() *Element {
	for child := df.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// LastElementChild returns the last child element.
func (df *DocumentFragment) LastElementChild() *Element {
	for child := df.AsNode().lastChild; child != nil; child = child.prevSibling {
		if child.nodeType == ElementNode {
			return (*Element)(child)
		}
	}
	return nil
}

// GetElementById returns the element with the given id.
func (df *DocumentFragment) GetElementById(id string) *Element {
	return df.findElementById(df.AsNode(), id)
}

func (df *DocumentFragment) findElementById(node *Node, id string) *Element {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if child.nodeType == ElementNode {
			el := (*Element)(child)
			if el.Id() == id {
				return el
			}
			result := df.findElementById(child, id)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// QuerySelector returns the first element matching the selector.
func (df *DocumentFragment) QuerySelector(selector string) *Element {
	for child := df.AsNode().firstChild; child != nil; child = child.nextSibling {
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
func (df *DocumentFragment) QuerySelectorAll(selector string) *NodeList {
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
	traverse(df.AsNode())

	return NewStaticNodeList(results)
}

// Append appends nodes or strings to this fragment.
func (df *DocumentFragment) Append(nodes ...interface{}) {
	for _, item := range nodes {
		switch v := item.(type) {
		case *Node:
			df.AsNode().AppendChild(v)
		case *Element:
			df.AsNode().AppendChild(v.AsNode())
		case string:
			df.AsNode().AppendChild(df.AsNode().ownerDoc.CreateTextNode(v))
		}
	}
}

// Prepend prepends nodes or strings to this fragment.
func (df *DocumentFragment) Prepend(nodes ...interface{}) {
	firstChild := df.AsNode().firstChild
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = df.AsNode().ownerDoc.CreateTextNode(v)
		}
		if node != nil {
			df.AsNode().InsertBefore(node, firstChild)
		}
	}
}

// ReplaceChildren replaces all children with the given nodes.
func (df *DocumentFragment) ReplaceChildren(nodes ...interface{}) {
	// Remove all children
	for df.AsNode().firstChild != nil {
		df.AsNode().RemoveChild(df.AsNode().firstChild)
	}
	// Append new children
	df.Append(nodes...)
}

// CloneNode clones this document fragment.
func (df *DocumentFragment) CloneNode(deep bool) *DocumentFragment {
	clone := df.AsNode().CloneNode(deep)
	return (*DocumentFragment)(clone)
}
