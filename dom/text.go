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
func (t *Text) AppendData(data string) {
	t.SetData(t.Data() + data)
}

// InsertData inserts a string at the given offset.
func (t *Text) InsertData(offset int, data string) {
	current := t.Data()
	if offset < 0 {
		offset = 0
	}
	if offset > len(current) {
		offset = len(current)
	}
	t.SetData(current[:offset] + data + current[offset:])
}

// DeleteData deletes characters starting at the given offset.
func (t *Text) DeleteData(offset, count int) {
	current := t.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	t.SetData(current[:offset] + current[end:])
}

// ReplaceData replaces characters starting at the given offset.
func (t *Text) ReplaceData(offset, count int, data string) {
	current := t.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	t.SetData(current[:offset] + data + current[end:])
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
