// Package dom provides DOM interfaces as specified by the DOM Living Standard.
// https://dom.spec.whatwg.org/
package dom

// NodeType represents the type of a Node as defined in the DOM specification.
type NodeType uint16

const (
	// ElementNode represents an Element node.
	ElementNode NodeType = 1
	// AttributeNode represents an Attr node (deprecated but still defined).
	AttributeNode NodeType = 2
	// TextNode represents a Text node.
	TextNode NodeType = 3
	// CDATASectionNode represents a CDATASection node.
	CDATASectionNode NodeType = 4
	// EntityReferenceNode is obsolete.
	EntityReferenceNode NodeType = 5
	// EntityNode is obsolete.
	EntityNode NodeType = 6
	// ProcessingInstructionNode represents a ProcessingInstruction node.
	ProcessingInstructionNode NodeType = 7
	// CommentNode represents a Comment node.
	CommentNode NodeType = 8
	// DocumentNode represents a Document node.
	DocumentNode NodeType = 9
	// DocumentTypeNode represents a DocumentType node.
	DocumentTypeNode NodeType = 10
	// DocumentFragmentNode represents a DocumentFragment node.
	DocumentFragmentNode NodeType = 11
	// NotationNode is obsolete.
	NotationNode NodeType = 12
)

// String returns the string representation of the NodeType.
func (nt NodeType) String() string {
	switch nt {
	case ElementNode:
		return "ELEMENT_NODE"
	case AttributeNode:
		return "ATTRIBUTE_NODE"
	case TextNode:
		return "TEXT_NODE"
	case CDATASectionNode:
		return "CDATA_SECTION_NODE"
	case ProcessingInstructionNode:
		return "PROCESSING_INSTRUCTION_NODE"
	case CommentNode:
		return "COMMENT_NODE"
	case DocumentNode:
		return "DOCUMENT_NODE"
	case DocumentTypeNode:
		return "DOCUMENT_TYPE_NODE"
	case DocumentFragmentNode:
		return "DOCUMENT_FRAGMENT_NODE"
	default:
		return "UNKNOWN_NODE"
	}
}
