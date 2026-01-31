package dom

// Text represents a text node in the DOM.
type Text Node

// AsNode returns the underlying Node.
func (t *Text) AsNode() *Node {
	return (*Node)(t)
}

// NodeType returns TextNode (3).
func (t *Text) NodeType() NodeType {
	return TextNode
}

// NodeName returns "#text".
func (t *Text) NodeName() string {
	return "#text"
}

// Data returns the text content.
func (t *Text) Data() string {
	return t.AsNode().NodeValue()
}

// SetData sets the text content.
func (t *Text) SetData(data string) {
	t.AsNode().SetNodeValue(data)
}

// Length returns the length of the text content.
func (t *Text) Length() int {
	return len(t.Data())
}

// WholeText returns the text of this node and all adjacent text nodes.
func (t *Text) WholeText() string {
	// Find the first text node in the sequence
	first := t.AsNode()
	for first.prevSibling != nil && first.prevSibling.nodeType == TextNode {
		first = first.prevSibling
	}

	// Concatenate all adjacent text nodes
	var result string
	for node := first; node != nil && node.nodeType == TextNode; node = node.nextSibling {
		result += node.NodeValue()
	}
	return result
}

// SubstringData extracts a substring of the text.
func (t *Text) SubstringData(offset, count int) string {
	data := t.Data()
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
// This is equivalent to insertData(length, data).
func (t *Text) AppendData(data string) {
	current := t.Data()
	offset := len(current)
	t.replaceDataInternal(offset, 0, data)
}

// InsertData inserts a string at the given offset.
// This is equivalent to replaceData(offset, 0, data).
func (t *Text) InsertData(offset int, data string) {
	current := t.Data()
	if offset < 0 {
		offset = 0
	}
	if offset > len(current) {
		offset = len(current)
	}
	t.replaceDataInternal(offset, 0, data)
}

// DeleteData deletes characters starting at the given offset.
// This is equivalent to replaceData(offset, count, "").
func (t *Text) DeleteData(offset, count int) {
	current := t.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	if count < 0 {
		count = 0
	}
	// Clamp count to not exceed available characters
	if offset+count > len(current) {
		count = len(current) - offset
	}
	t.replaceDataInternal(offset, count, "")
}

// ReplaceData replaces characters starting at the given offset.
func (t *Text) ReplaceData(offset, count int, data string) {
	current := t.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	if count < 0 {
		count = 0
	}
	// Clamp count to not exceed available characters
	if offset+count > len(current) {
		count = len(current) - offset
	}
	t.replaceDataInternal(offset, count, data)
}

// replaceDataInternal implements the DOM "replace data" algorithm.
// It updates the data and notifies mutation callbacks with the specific offset/count/data.
func (t *Text) replaceDataInternal(offset, count int, data string) {
	current := t.Data()
	end := offset + count
	if end > len(current) {
		end = len(current)
	}

	// Notify ReplaceData mutation BEFORE changing the data
	// This allows handlers to use the original data if needed
	notifyReplaceData(t.AsNode(), offset, count, data)

	// Update the data
	newValue := current[:offset] + data + current[end:]
	t.AsNode().nodeValue = &newValue
}

// SplitText splits this text node at the given offset.
// Returns the new text node containing the text after the offset.
func (t *Text) SplitText(offset int) *Text {
	data := t.Data()
	if offset < 0 || offset > len(data) {
		return nil
	}

	// Create new text node with the text after offset
	newData := data[offset:]
	newNode := t.AsNode().ownerDoc.CreateTextNode(newData)
	newText := (*Text)(newNode)

	// Truncate this node
	t.SetData(data[:offset])

	// Insert new node after this one
	parent := t.AsNode().parentNode
	if parent != nil {
		parent.InsertBefore(newNode, t.AsNode().nextSibling)
	}

	return newText
}

// CloneNode clones this text node.
func (t *Text) CloneNode(deep bool) *Text {
	clone := t.AsNode().ownerDoc.CreateTextNode(t.Data())
	return (*Text)(clone)
}

// IsElementContentWhitespace returns true if this is element content whitespace.
// This is a simplified implementation.
func (t *Text) IsElementContentWhitespace() bool {
	for _, r := range t.Data() {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

// Before inserts nodes before this text node.
// Implements the ChildNode.before() algorithm from DOM spec.
func (t *Text) Before(nodes ...interface{}) {
	parent := t.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viablePrevSibling := t.AsNode().findViablePreviousSibling(nodeSet)

	node := t.AsNode().convertNodesToFragment(nodes)
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

// After inserts nodes after this text node.
// Implements the ChildNode.after() algorithm from DOM spec.
func (t *Text) After(nodes ...interface{}) {
	parent := t.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := t.AsNode().findViableNextSibling(nodeSet)

	node := t.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	parent.InsertBefore(node, viableNextSibling)
}

// ReplaceWith replaces this text node with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (t *Text) ReplaceWith(nodes ...interface{}) {
	parent := t.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := t.AsNode().findViableNextSibling(nodeSet)

	node := t.AsNode().convertNodesToFragment(nodes)

	if t.AsNode().parentNode == parent {
		if node != nil {
			parent.ReplaceChild(node, t.AsNode())
		} else {
			parent.RemoveChild(t.AsNode())
		}
	} else if node != nil {
		parent.InsertBefore(node, viableNextSibling)
	}
}

// Remove removes this text node from its parent.
func (t *Text) Remove() {
	if t.AsNode().parentNode != nil {
		t.AsNode().parentNode.RemoveChild(t.AsNode())
	}
}

// NewTextNode creates a new detached text node with the given data.
// The node has no owner document.
func NewTextNode(data string) *Node {
	node := newNode(TextNode, "#text", nil)
	node.textData = &data
	node.nodeValue = &data
	return node
}
