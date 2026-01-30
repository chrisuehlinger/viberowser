// Package html provides HTML parsing functionality using golang.org/x/net/html
// as the underlying parser implementation.
package html

import (
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// NodeType represents the type of an HTML node.
type NodeType int

const (
	ErrorNode NodeType = iota
	TextNode
	DocumentNode
	ElementNode
	CommentNode
	DoctypeNode
)

// Attribute represents an HTML attribute.
type Attribute struct {
	Namespace string
	Key       string
	Value     string
}

// Node represents a node in the HTML document tree.
// This is our DOM-compatible node structure that wraps the underlying
// golang.org/x/net/html node.
type Node struct {
	Type       NodeType
	Data       string    // For elements: tag name; for text: text content
	DataAtom   atom.Atom // Atom for known HTML elements
	Namespace  string    // Namespace URI (for SVG, MathML, etc.)
	Attributes []Attribute

	Parent          *Node
	FirstChild      *Node
	LastChild       *Node
	PrevSibling     *Node
	NextSibling     *Node
}

// AppendChild adds a child node to the end of this node's children.
func (n *Node) AppendChild(c *Node) {
	if c.Parent != nil {
		c.Parent.RemoveChild(c)
	}
	c.Parent = n
	c.PrevSibling = n.LastChild
	c.NextSibling = nil
	if n.LastChild != nil {
		n.LastChild.NextSibling = c
	} else {
		n.FirstChild = c
	}
	n.LastChild = c
}

// RemoveChild removes a child node from this node's children.
func (n *Node) RemoveChild(c *Node) {
	if c.Parent != n {
		return
	}
	if c.PrevSibling != nil {
		c.PrevSibling.NextSibling = c.NextSibling
	} else {
		n.FirstChild = c.NextSibling
	}
	if c.NextSibling != nil {
		c.NextSibling.PrevSibling = c.PrevSibling
	} else {
		n.LastChild = c.PrevSibling
	}
	c.Parent = nil
	c.PrevSibling = nil
	c.NextSibling = nil
}

// InsertBefore inserts newChild before oldChild. If oldChild is nil,
// newChild is appended to the end.
func (n *Node) InsertBefore(newChild, oldChild *Node) {
	if oldChild == nil {
		n.AppendChild(newChild)
		return
	}
	if oldChild.Parent != n {
		return
	}
	if newChild.Parent != nil {
		newChild.Parent.RemoveChild(newChild)
	}
	newChild.Parent = n
	newChild.NextSibling = oldChild
	newChild.PrevSibling = oldChild.PrevSibling
	if oldChild.PrevSibling != nil {
		oldChild.PrevSibling.NextSibling = newChild
	} else {
		n.FirstChild = newChild
	}
	oldChild.PrevSibling = newChild
}

// Children returns a slice of all child nodes.
func (n *Node) Children() []*Node {
	var children []*Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		children = append(children, c)
	}
	return children
}

// GetAttribute returns the value of the specified attribute, or empty string if not found.
func (n *Node) GetAttribute(key string) string {
	for _, attr := range n.Attributes {
		if attr.Key == key {
			return attr.Value
		}
	}
	return ""
}

// SetAttribute sets an attribute value, creating it if it doesn't exist.
func (n *Node) SetAttribute(key, value string) {
	for i, attr := range n.Attributes {
		if attr.Key == key {
			n.Attributes[i].Value = value
			return
		}
	}
	n.Attributes = append(n.Attributes, Attribute{Key: key, Value: value})
}

// HasAttribute returns true if the node has the specified attribute.
func (n *Node) HasAttribute(key string) bool {
	for _, attr := range n.Attributes {
		if attr.Key == key {
			return true
		}
	}
	return false
}

// RemoveAttribute removes an attribute from the node.
func (n *Node) RemoveAttribute(key string) {
	for i, attr := range n.Attributes {
		if attr.Key == key {
			n.Attributes = append(n.Attributes[:i], n.Attributes[i+1:]...)
			return
		}
	}
}

// TextContent returns the text content of a node and its descendants.
func (n *Node) TextContent() string {
	var sb strings.Builder
	n.collectTextContent(&sb)
	return sb.String()
}

func (n *Node) collectTextContent(sb *strings.Builder) {
	if n.Type == TextNode {
		sb.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.collectTextContent(sb)
	}
}

// Parse parses HTML from a string and returns a document node.
func Parse(htmlContent string) (*Node, error) {
	return ParseReader(strings.NewReader(htmlContent))
}

// ParseReader parses HTML from an io.Reader and returns a document node.
func ParseReader(r io.Reader) (*Node, error) {
	netNode, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	return convertNode(netNode), nil
}

// ParseFragment parses an HTML fragment in the context of a parent element.
func ParseFragment(fragment string, context *Node) ([]*Node, error) {
	return ParseFragmentReader(strings.NewReader(fragment), context)
}

// ParseFragmentReader parses an HTML fragment from a reader.
func ParseFragmentReader(r io.Reader, context *Node) ([]*Node, error) {
	var contextNode *html.Node
	if context != nil && context.Type == ElementNode {
		contextNode = &html.Node{
			Type:     html.ElementNode,
			DataAtom: context.DataAtom,
			Data:     context.Data,
		}
	}
	netNodes, err := html.ParseFragment(r, contextNode)
	if err != nil {
		return nil, err
	}
	nodes := make([]*Node, len(netNodes))
	for i, nn := range netNodes {
		nodes[i] = convertNode(nn)
	}
	return nodes, nil
}

// convertNode converts a golang.org/x/net/html node to our Node type.
func convertNode(n *html.Node) *Node {
	if n == nil {
		return nil
	}
	node := &Node{
		Type:      convertNodeType(n.Type),
		Data:      n.Data,
		DataAtom:  n.DataAtom,
		Namespace: n.Namespace,
	}
	// Convert attributes
	for _, attr := range n.Attr {
		node.Attributes = append(node.Attributes, Attribute{
			Namespace: attr.Namespace,
			Key:       attr.Key,
			Value:     attr.Val,
		})
	}
	// Convert children recursively
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		child := convertNode(c)
		node.AppendChild(child)
	}
	return node
}

// convertNodeType converts golang.org/x/net/html.NodeType to our NodeType.
func convertNodeType(nt html.NodeType) NodeType {
	switch nt {
	case html.ErrorNode:
		return ErrorNode
	case html.TextNode:
		return TextNode
	case html.DocumentNode:
		return DocumentNode
	case html.ElementNode:
		return ElementNode
	case html.CommentNode:
		return CommentNode
	case html.DoctypeNode:
		return DoctypeNode
	default:
		return ErrorNode
	}
}

// Tokenizer wraps the underlying HTML tokenizer for low-level token access.
type Tokenizer struct {
	z *html.Tokenizer
}

// TokenType represents the type of an HTML token.
type TokenType int

const (
	ErrorToken TokenType = iota
	TextToken
	StartTagToken
	EndTagToken
	SelfClosingTagToken
	CommentToken
	DoctypeToken
)

// Token represents an HTML token.
type Token struct {
	Type       TokenType
	Data       string
	DataAtom   atom.Atom
	Attributes []Attribute
}

// NewTokenizer creates a new tokenizer for the given reader.
func NewTokenizer(r io.Reader) *Tokenizer {
	return &Tokenizer{z: html.NewTokenizer(r)}
}

// NewTokenizerString creates a new tokenizer for the given string.
func NewTokenizerString(s string) *Tokenizer {
	return NewTokenizer(strings.NewReader(s))
}

// Next advances the tokenizer and returns the type of the next token.
func (t *Tokenizer) Next() TokenType {
	tt := t.z.Next()
	return convertTokenType(tt)
}

// Token returns the current token.
func (t *Tokenizer) Token() Token {
	tok := t.z.Token()
	return Token{
		Type:       convertTokenType(tok.Type),
		Data:       tok.Data,
		DataAtom:   tok.DataAtom,
		Attributes: convertAttributes(tok.Attr),
	}
}

// Text returns the unescaped text of a text or comment token.
func (t *Tokenizer) Text() string {
	return string(t.z.Text())
}

// TagName returns the lower-cased name of a tag token.
func (t *Tokenizer) TagName() (name []byte, hasAttr bool) {
	return t.z.TagName()
}

// TagAttr returns the next attribute key-value pair of the current tag token.
func (t *Tokenizer) TagAttr() (key, val []byte, moreAttr bool) {
	return t.z.TagAttr()
}

// Err returns the error associated with the most recent ErrorToken.
func (t *Tokenizer) Err() error {
	return t.z.Err()
}

// Raw returns the raw bytes of the current token.
func (t *Tokenizer) Raw() []byte {
	return t.z.Raw()
}

func convertTokenType(tt html.TokenType) TokenType {
	switch tt {
	case html.ErrorToken:
		return ErrorToken
	case html.TextToken:
		return TextToken
	case html.StartTagToken:
		return StartTagToken
	case html.EndTagToken:
		return EndTagToken
	case html.SelfClosingTagToken:
		return SelfClosingTagToken
	case html.CommentToken:
		return CommentToken
	case html.DoctypeToken:
		return DoctypeToken
	default:
		return ErrorToken
	}
}

func convertAttributes(attrs []html.Attribute) []Attribute {
	result := make([]Attribute, len(attrs))
	for i, a := range attrs {
		result[i] = Attribute{
			Namespace: a.Namespace,
			Key:       a.Key,
			Value:     a.Val,
		}
	}
	return result
}
