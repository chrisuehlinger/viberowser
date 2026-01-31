package dom

// CDATASection represents a CDATA section in an XML document.
// CDATASection inherits from Text and has no additional attributes or methods.
// However, the nodeName is "#cdata-section" and nodeType is CDATASectionNode (4).
//
// Per the DOM spec, CDATA sections are only valid in XML documents.
// In HTML documents, they are not allowed and createCDATASection throws NotSupportedError.
type CDATASection Node

// AsNode returns the underlying Node.
func (c *CDATASection) AsNode() *Node {
	return (*Node)(c)
}

// NodeType returns CDATASectionNode (4).
func (c *CDATASection) NodeType() NodeType {
	return CDATASectionNode
}

// NodeName returns "#cdata-section".
func (c *CDATASection) NodeName() string {
	return "#cdata-section"
}

// Data returns the text content.
func (c *CDATASection) Data() string {
	return c.AsNode().NodeValue()
}

// SetData sets the text content.
func (c *CDATASection) SetData(data string) {
	c.AsNode().SetNodeValue(data)
}

// Length returns the length of the text content.
func (c *CDATASection) Length() int {
	return len(c.Data())
}

// SubstringData extracts a substring of the text.
func (c *CDATASection) SubstringData(offset, count int) string {
	data := c.Data()
	if offset < 0 || offset > len(data) {
		return ""
	}
	end := offset + count
	if end > len(data) {
		end = len(data)
	}
	return data[offset:end]
}

// AppendData appends a string to the text.
func (c *CDATASection) AppendData(data string) {
	current := c.Data()
	offset := len(current)
	c.replaceDataInternal(offset, 0, data)
}

// InsertData inserts a string at the given offset.
func (c *CDATASection) InsertData(offset int, data string) {
	current := c.Data()
	if offset < 0 {
		offset = 0
	}
	if offset > len(current) {
		offset = len(current)
	}
	c.replaceDataInternal(offset, 0, data)
}

// DeleteData deletes characters starting at the given offset.
func (c *CDATASection) DeleteData(offset, count int) {
	current := c.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	if count < 0 {
		count = 0
	}
	if offset+count > len(current) {
		count = len(current) - offset
	}
	c.replaceDataInternal(offset, count, "")
}

// ReplaceData replaces characters starting at the given offset.
func (c *CDATASection) ReplaceData(offset, count int, data string) {
	current := c.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	if count < 0 {
		count = 0
	}
	if offset+count > len(current) {
		count = len(current) - offset
	}
	c.replaceDataInternal(offset, count, data)
}

// replaceDataInternal implements the DOM "replace data" algorithm.
func (c *CDATASection) replaceDataInternal(offset, count int, data string) {
	current := c.Data()
	end := offset + count
	if end > len(current) {
		end = len(current)
	}

	notifyReplaceData(c.AsNode(), offset, count, data)

	newValue := current[:offset] + data + current[end:]
	c.AsNode().nodeValue = &newValue
}

// SplitText splits this CDATASection node at the given offset.
// Returns the new CDATASection node containing the text after the offset.
func (c *CDATASection) SplitText(offset int) *CDATASection {
	data := c.Data()
	if offset < 0 || offset > len(data) {
		return nil
	}

	// Create new CDATASection node with the text after offset
	newData := data[offset:]
	newNode, _ := c.AsNode().ownerDoc.CreateCDATASectionWithError(newData)
	if newNode == nil {
		return nil
	}
	newCDATA := (*CDATASection)(newNode)

	// Truncate this node
	c.SetData(data[:offset])

	// Insert new node after this one
	parent := c.AsNode().parentNode
	if parent != nil {
		parent.InsertBefore(newNode, c.AsNode().nextSibling)
	}

	return newCDATA
}

// CloneNode clones this CDATASection node.
func (c *CDATASection) CloneNode(deep bool) *CDATASection {
	clone, _ := c.AsNode().ownerDoc.CreateCDATASectionWithError(c.Data())
	return (*CDATASection)(clone)
}

// Before inserts nodes before this CDATASection node.
// Implements the ChildNode.before() algorithm from DOM spec.
func (c *CDATASection) Before(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viablePrevSibling := c.AsNode().findViablePreviousSibling(nodeSet)

	node := c.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	var refNode *Node
	if viablePrevSibling == nil {
		refNode = parent.firstChild
	} else {
		refNode = viablePrevSibling.nextSibling
	}
	parent.InsertBefore(node, refNode)
}

// After inserts nodes after this CDATASection node.
// Implements the ChildNode.after() algorithm from DOM spec.
func (c *CDATASection) After(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := c.AsNode().findViableNextSibling(nodeSet)

	node := c.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	parent.InsertBefore(node, viableNextSibling)
}

// ReplaceWith replaces this CDATASection node with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (c *CDATASection) ReplaceWith(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := c.AsNode().findViableNextSibling(nodeSet)

	node := c.AsNode().convertNodesToFragment(nodes)

	if c.AsNode().parentNode == parent {
		if node != nil {
			parent.ReplaceChild(node, c.AsNode())
		} else {
			parent.RemoveChild(c.AsNode())
		}
	} else if node != nil {
		parent.InsertBefore(node, viableNextSibling)
	}
}

// Remove removes this CDATASection node from its parent.
func (c *CDATASection) Remove() {
	if c.AsNode().parentNode != nil {
		c.AsNode().parentNode.RemoveChild(c.AsNode())
	}
}

// NewCDATASectionNode creates a new detached CDATASection node with the given data.
// The node has no owner document.
func NewCDATASectionNode(data string) *Node {
	node := newNode(CDATASectionNode, "#cdata-section", nil)
	node.textData = &data
	node.nodeValue = &data
	return node
}
