// Package css provides styled tree construction.
package css

import (
	"github.com/chrisuehlinger/viberowser/dom"
)

// StyledNode represents a DOM node with computed styles.
type StyledNode struct {
	Node     *dom.Node
	Style    *ComputedStyle
	Children []*StyledNode
}

// StyleTree represents a styled document tree.
type StyleTree struct {
	Root     *StyledNode
	Resolver *StyleResolver

	// Cache of computed styles by element
	styleCache map[*dom.Element]*ComputedStyle
}

// NewStyleTree creates a new style tree.
func NewStyleTree() *StyleTree {
	return &StyleTree{
		Resolver:   NewStyleResolver(),
		styleCache: make(map[*dom.Element]*ComputedStyle),
	}
}

// BuildStyleTree constructs a styled tree from a DOM document.
func (st *StyleTree) BuildStyleTree(doc *dom.Document) *StyledNode {
	// Clear cache
	st.styleCache = make(map[*dom.Element]*ComputedStyle)

	// Set up user agent stylesheet
	st.Resolver.SetUserAgentStylesheet(GetUserAgentStylesheet())

	// Collect author stylesheets from the document
	st.collectStylesheets(doc)

	// Build the styled tree
	docNode := (*dom.Node)(doc)
	st.Root = st.buildStyledNode(docNode, nil)
	return st.Root
}

// collectStylesheets finds all <style> and <link rel="stylesheet"> elements.
func (st *StyleTree) collectStylesheets(doc *dom.Document) {
	st.Resolver.ClearAuthorStylesheets()

	// Find all style elements
	styleElements := doc.GetElementsByTagName("style")
	for i := 0; i < styleElements.Length(); i++ {
		el := styleElements.Item(i)
		if el != nil {
			cssText := el.AsNode().TextContent()
			parser := NewParser(cssText)
			ss := parser.Parse()
			st.Resolver.AddAuthorStylesheet(ss)
		}
	}

	// Note: <link rel="stylesheet"> would need HTTP fetching
	// which is handled by the resource loader
}

// AddStylesheet adds an author stylesheet to the resolver.
func (st *StyleTree) AddStylesheet(css string) {
	parser := NewParser(css)
	ss := parser.Parse()
	st.Resolver.AddAuthorStylesheet(ss)
}

// AddParsedStylesheet adds a pre-parsed stylesheet to the resolver.
func (st *StyleTree) AddParsedStylesheet(ss *Stylesheet) {
	st.Resolver.AddAuthorStylesheet(ss)
}

// buildStyledNode recursively builds a styled node from a DOM node.
func (st *StyleTree) buildStyledNode(node *dom.Node, parentStyle *ComputedStyle) *StyledNode {
	sn := &StyledNode{
		Node: node,
	}

	// Compute style for elements
	if node.NodeType() == dom.ElementNode {
		el := (*dom.Element)(node)
		sn.Style = st.computeElementStyle(el, parentStyle)
	} else {
		// Non-element nodes inherit from parent
		sn.Style = parentStyle
	}

	// Build children
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		childSN := st.buildStyledNode(child, sn.Style)
		sn.Children = append(sn.Children, childSN)
	}

	return sn
}

// computeElementStyle computes the style for an element.
func (st *StyleTree) computeElementStyle(el *dom.Element, parentStyle *ComputedStyle) *ComputedStyle {
	// Check cache
	if cached, ok := st.styleCache[el]; ok {
		return cached
	}

	// Compute style
	style := st.Resolver.ResolveStyles(el, parentStyle)

	// Cache it
	st.styleCache[el] = style

	return style
}

// GetComputedStyle returns the computed style for an element.
func (st *StyleTree) GetComputedStyle(el *dom.Element) *ComputedStyle {
	if cached, ok := st.styleCache[el]; ok {
		return cached
	}

	// Need to find parent style
	var parentStyle *ComputedStyle
	if parentEl := el.AsNode().ParentElement(); parentEl != nil {
		parentStyle = st.GetComputedStyle(parentEl)
	}

	return st.computeElementStyle(el, parentStyle)
}

// InvalidateElement invalidates the cached style for an element and its descendants.
func (st *StyleTree) InvalidateElement(el *dom.Element) {
	delete(st.styleCache, el)

	// Invalidate children
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		st.InvalidateElement(child)
	}
}

// InvalidateAll clears the entire style cache.
func (st *StyleTree) InvalidateAll() {
	st.styleCache = make(map[*dom.Element]*ComputedStyle)
	st.Root = nil
}

// GetDisplay returns the display value for a styled node.
func (sn *StyledNode) GetDisplay() string {
	if sn.Style == nil {
		return "inline"
	}
	if val := sn.Style.GetPropertyValue("display"); val != nil {
		if val.Keyword != "" {
			return val.Keyword
		}
	}
	return "inline"
}

// IsBlock returns true if this node generates a block box.
func (sn *StyledNode) IsBlock() bool {
	display := sn.GetDisplay()
	switch display {
	case "block", "flex", "grid", "table", "list-item",
		"table-row-group", "table-header-group", "table-footer-group",
		"table-row", "table-column-group", "table-column", "table-cell",
		"table-caption":
		return true
	default:
		return false
	}
}

// IsInline returns true if this node generates an inline box.
func (sn *StyledNode) IsInline() bool {
	display := sn.GetDisplay()
	switch display {
	case "inline", "inline-block", "inline-flex", "inline-grid", "inline-table":
		return true
	default:
		return false
	}
}

// IsHidden returns true if this node should not be rendered.
func (sn *StyledNode) IsHidden() bool {
	display := sn.GetDisplay()
	if display == "none" {
		return true
	}

	if sn.Style != nil {
		if val := sn.Style.GetPropertyValue("visibility"); val != nil {
			if val.Keyword == "hidden" || val.Keyword == "collapse" {
				return true
			}
		}
	}

	return false
}
