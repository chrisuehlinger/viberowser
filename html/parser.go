// Package html provides HTML parsing functionality.
package html

// Node represents a node in the HTML document tree.
type Node struct {
	Type       NodeType
	Data       string
	Attributes []Attribute
	Parent     *Node
	Children   []*Node
}

// NodeType represents the type of an HTML node.
type NodeType int

const (
	ElementNode NodeType = iota
	TextNode
	CommentNode
	DoctypeNode
	DocumentNode
)

// Attribute represents an HTML attribute.
type Attribute struct {
	Key   string
	Value string
}

// Parser handles HTML parsing.
type Parser struct {
	input string
	pos   int
}

// NewParser creates a new HTML parser for the given input.
func NewParser(input string) *Parser {
	return &Parser{
		input: input,
		pos:   0,
	}
}

// Parse parses the HTML input and returns a document node.
func (p *Parser) Parse() *Node {
	doc := &Node{
		Type: DocumentNode,
	}
	// TODO: Implement HTML parsing
	return doc
}
