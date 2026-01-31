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
	c.SetData(c.Data() + data)
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
	c.SetData(current[:offset] + data + current[offset:])
}

// DeleteData deletes characters starting at the given offset.
func (c *Comment) DeleteData(offset, count int) {
	current := c.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	c.SetData(current[:offset] + current[end:])
}

// ReplaceData replaces characters starting at the given offset.
func (c *Comment) ReplaceData(offset, count int, data string) {
	current := c.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	c.SetData(current[:offset] + data + current[end:])
}

// CloneNode clones this comment node.
func (c *Comment) CloneNode(deep bool) *Comment {
	clone := c.AsNode().ownerDoc.CreateComment(c.Data())
	return (*Comment)(clone)
}

// Before inserts nodes before this comment node.
func (c *Comment) Before(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = c.AsNode().ownerDoc.CreateTextNode(v)
		}
		if node != nil {
			parent.InsertBefore(node, c.AsNode())
		}
	}
}

// After inserts nodes after this comment node.
func (c *Comment) After(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	ref := c.AsNode().nextSibling
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = c.AsNode().ownerDoc.CreateTextNode(v)
		}
		if node != nil {
			parent.InsertBefore(node, ref)
		}
	}
}

// ReplaceWith replaces this comment node with nodes.
func (c *Comment) ReplaceWith(nodes ...interface{}) {
	parent := c.AsNode().parentNode
	if parent == nil {
		return
	}
	ref := c.AsNode().nextSibling
	parent.RemoveChild(c.AsNode())
	for _, item := range nodes {
		var node *Node
		switch v := item.(type) {
		case *Node:
			node = v
		case *Element:
			node = v.AsNode()
		case string:
			node = c.AsNode().ownerDoc.CreateTextNode(v)
		}
		if node != nil {
			parent.InsertBefore(node, ref)
		}
	}
}

// Remove removes this comment node from its parent.
func (c *Comment) Remove() {
	if c.AsNode().parentNode != nil {
		c.AsNode().parentNode.RemoveChild(c.AsNode())
	}
}
