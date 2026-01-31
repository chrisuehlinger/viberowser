package dom

import (
	"regexp"
	"strings"
)

// ProcessingInstruction represents a processing instruction node in the DOM.
// Processing instructions look like: <?target data?>
// This interface inherits from CharacterData.
type ProcessingInstruction Node

// AsNode returns the underlying Node.
func (pi *ProcessingInstruction) AsNode() *Node {
	return (*Node)(pi)
}

// NodeType returns ProcessingInstructionNode (7).
func (pi *ProcessingInstruction) NodeType() NodeType {
	return ProcessingInstructionNode
}

// NodeName returns the target of the processing instruction.
func (pi *ProcessingInstruction) NodeName() string {
	return pi.AsNode().nodeName
}

// Target returns the target of the processing instruction (read-only).
// This is the application to which the instruction is targeted.
func (pi *ProcessingInstruction) Target() string {
	return pi.AsNode().nodeName
}

// Data returns the content of the processing instruction.
func (pi *ProcessingInstruction) Data() string {
	return pi.AsNode().NodeValue()
}

// SetData sets the content of the processing instruction.
func (pi *ProcessingInstruction) SetData(data string) {
	pi.AsNode().SetNodeValue(data)
}

// Length returns the length of the data content.
func (pi *ProcessingInstruction) Length() int {
	return len(pi.Data())
}

// SubstringData extracts a substring of the data.
func (pi *ProcessingInstruction) SubstringData(offset, count int) string {
	data := pi.Data()
	if offset < 0 || offset > len(data) {
		return ""
	}
	end := offset + count
	if end > len(data) {
		end = len(data)
	}
	return data[offset:end]
}

// AppendData appends a string to the data.
func (pi *ProcessingInstruction) AppendData(data string) {
	pi.SetData(pi.Data() + data)
}

// InsertData inserts a string at the given offset.
func (pi *ProcessingInstruction) InsertData(offset int, data string) {
	current := pi.Data()
	if offset < 0 {
		offset = 0
	}
	if offset > len(current) {
		offset = len(current)
	}
	pi.SetData(current[:offset] + data + current[offset:])
}

// DeleteData deletes characters starting at the given offset.
func (pi *ProcessingInstruction) DeleteData(offset, count int) {
	current := pi.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	pi.SetData(current[:offset] + current[end:])
}

// ReplaceData replaces characters starting at the given offset.
func (pi *ProcessingInstruction) ReplaceData(offset, count int, data string) {
	current := pi.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	pi.SetData(current[:offset] + data + current[end:])
}

// CloneNode clones this processing instruction node.
func (pi *ProcessingInstruction) CloneNode(deep bool) *ProcessingInstruction {
	clone := pi.AsNode().ownerDoc.CreateProcessingInstruction(pi.Target(), pi.Data())
	return (*ProcessingInstruction)(clone)
}

// Before inserts nodes before this processing instruction node.
// Implements the ChildNode.before() algorithm from DOM spec.
func (pi *ProcessingInstruction) Before(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viablePrevSibling := pi.AsNode().findViablePreviousSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)
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

// After inserts nodes after this processing instruction node.
// Implements the ChildNode.after() algorithm from DOM spec.
func (pi *ProcessingInstruction) After(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := pi.AsNode().findViableNextSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	parent.InsertBefore(node, viableNextSibling)
}

// ReplaceWith replaces this processing instruction node with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (pi *ProcessingInstruction) ReplaceWith(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := pi.AsNode().findViableNextSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)

	if pi.AsNode().parentNode == parent {
		if node != nil {
			parent.ReplaceChild(node, pi.AsNode())
		} else {
			parent.RemoveChild(pi.AsNode())
		}
	} else if node != nil {
		parent.InsertBefore(node, viableNextSibling)
	}
}

// Remove removes this processing instruction node from its parent.
func (pi *ProcessingInstruction) Remove() {
	if pi.AsNode().parentNode != nil {
		pi.AsNode().parentNode.RemoveChild(pi.AsNode())
	}
}

// processingInstructionData holds data specific to ProcessingInstruction nodes.
type processingInstructionData struct {
	target string
	data   string
}

// NewProcessingInstructionNode creates a new detached processing instruction node with the given target and data.
// The node has no owner document.
func NewProcessingInstructionNode(target, data string) *Node {
	node := newNode(ProcessingInstructionNode, target, nil)
	node.nodeValue = &data
	return node
}

// isValidXMLName checks if a string is a valid XML name.
// Per XML spec, names must start with a letter, underscore, or colon,
// and can contain letters, digits, hyphens, underscores, colons, and periods.
func isValidXMLName(name string) bool {
	if name == "" {
		return false
	}

	// XML Name regex pattern
	// NameStartChar ::= ":" | [A-Z] | "_" | [a-z] | [#xC0-#xD6] | [#xD8-#xF6] | etc.
	// NameChar ::= NameStartChar | "-" | "." | [0-9] | #xB7 | [#x0300-#x036F] | [#x203F-#x2040]
	// For simplicity, we use a basic pattern that covers most common cases

	// Check first character
	firstChar := rune(name[0])
	if !isXMLNameStartChar(firstChar) {
		return false
	}

	// Check remaining characters
	for _, ch := range name[1:] {
		if !isXMLNameChar(ch) {
			return false
		}
	}

	return true
}

// isXMLNameStartChar checks if a rune is a valid XML name start character.
func isXMLNameStartChar(ch rune) bool {
	return ch == ':' ||
		(ch >= 'A' && ch <= 'Z') ||
		ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 0xC0 && ch <= 0xD6) ||
		(ch >= 0xD8 && ch <= 0xF6) ||
		(ch >= 0xF8 && ch <= 0x2FF) ||
		(ch >= 0x370 && ch <= 0x37D) ||
		(ch >= 0x37F && ch <= 0x1FFF) ||
		(ch >= 0x200C && ch <= 0x200D) ||
		(ch >= 0x2070 && ch <= 0x218F) ||
		(ch >= 0x2C00 && ch <= 0x2FEF) ||
		(ch >= 0x3001 && ch <= 0xD7FF) ||
		(ch >= 0xF900 && ch <= 0xFDCF) ||
		(ch >= 0xFDF0 && ch <= 0xFFFD) ||
		(ch >= 0x10000 && ch <= 0xEFFFF)
}

// isXMLNameChar checks if a rune is a valid XML name character.
func isXMLNameChar(ch rune) bool {
	return isXMLNameStartChar(ch) ||
		ch == '-' ||
		ch == '.' ||
		(ch >= '0' && ch <= '9') ||
		ch == 0xB7 ||
		(ch >= 0x0300 && ch <= 0x036F) ||
		(ch >= 0x203F && ch <= 0x2040)
}

// xmlNamePattern is a regex for valid XML names (simplified)
var xmlNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:.\-]*$`)

// ValidateProcessingInstructionTarget validates a processing instruction target.
// Returns an error if the target is not a valid XML name.
func ValidateProcessingInstructionTarget(target string) error {
	if !isValidXMLName(target) {
		return ErrInvalidCharacter("The target is not a valid XML name.")
	}
	return nil
}

// ValidateProcessingInstructionData validates processing instruction data.
// Returns an error if the data contains the closing sequence "?>".
func ValidateProcessingInstructionData(data string) error {
	if strings.Contains(data, "?>") {
		return ErrInvalidCharacter("The data contains the invalid sequence '?>'.")
	}
	return nil
}
