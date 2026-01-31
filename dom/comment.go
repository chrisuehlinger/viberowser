package dom

// Comment represents a comment node in the DOM.
type Comment Node

// AsNode returns the underlying Node.
func (c *Comment) AsNode() *Node {
	return (*Node)(c)
}

// NodeType returns CommentNode (8).
func (c *Comment) NodeType() NodeType {
	return CommentNode
}

// NodeName returns "#comment".
func (c *Comment) NodeName() string {
	return "#comment"
}

// Data returns the comment content.
func (c *Comment) Data() string {
	return c.AsNode().NodeValue()
}

// SetData sets the comment content.
func (c *Comment) SetData(data string) {
	c.AsNode().SetNodeValue(data)
}

// Length returns the length of the comment content.
func (c *Comment) Length() int {
	return len(c.Data())
}

// SubstringData extracts a substring of the comment.
func (c *Comment) SubstringData(offset, count int) string {
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

// AppendData appends a string to the comment.
func (c *Comment) AppendData(data string) {
	current := c.Data()
	offset := len(current)
	c.replaceDataInternal(offset, 0, data)
}

// InsertData inserts a string at the given offset.
func (c *Comment) InsertData(offset int, data string) {
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
func (c *Comment) DeleteData(offset, count int) {
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
func (c *Comment) ReplaceData(offset, count int, data string) {
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
func (c *Comment) replaceDataInternal(offset, count int, data string) {
	current := c.Data()
	end := offset + count
	if end > len(current) {
		end = len(current)
	}

	notifyReplaceData(c.AsNode(), offset, count, data)

	newValue := current[:offset] + data + current[end:]
	c.AsNode().nodeValue = &newValue
}

// CloneNode clones this comment node.
func (c *Comment) CloneNode(deep bool) *Comment {
	clone := c.AsNode().ownerDoc.CreateComment(c.Data())
	return (*Comment)(clone)
}

// Before inserts nodes before this comment node.
// Implements the ChildNode.before() algorithm from DOM spec.
func (c *Comment) Before(nodes ...interface{}) {
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

// After inserts nodes after this comment node.
// Implements the ChildNode.after() algorithm from DOM spec.
func (c *Comment) After(nodes ...interface{}) {
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

// ReplaceWith replaces this comment node with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (c *Comment) ReplaceWith(nodes ...interface{}) {
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

// Remove removes this comment node from its parent.
func (c *Comment) Remove() {
	if c.AsNode().parentNode != nil {
		c.AsNode().parentNode.RemoveChild(c.AsNode())
	}
}

// NewCommentNode creates a new detached comment node with the given data.
// The node has no owner document.
func NewCommentNode(data string) *Node {
	node := newNode(CommentNode, "#comment", nil)
	node.commentData = &data
	node.nodeValue = &data
	return node
}
