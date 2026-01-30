// Package layout handles the layout/box model calculations.
package layout

import (
	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/html"
)

// Dimensions represents the dimensions of a layout box.
type Dimensions struct {
	Content Rect
	Padding EdgeSizes
	Border  EdgeSizes
	Margin  EdgeSizes
}

// Rect represents a rectangular area.
type Rect struct {
	X, Y, Width, Height float64
}

// EdgeSizes represents the sizes of edges (top, right, bottom, left).
type EdgeSizes struct {
	Top, Right, Bottom, Left float64
}

// BoxType represents the type of layout box.
type BoxType int

const (
	BlockBox BoxType = iota
	InlineBox
	AnonymousBox
)

// LayoutBox represents a box in the layout tree.
type LayoutBox struct {
	Dimensions Dimensions
	BoxType    BoxType
	StyledNode *StyledNode
	Children   []*LayoutBox
}

// StyledNode represents an HTML node with computed styles.
type StyledNode struct {
	Node     *html.Node
	Styles   map[string]css.Value
	Children []*StyledNode
}

// BuildLayoutTree constructs a layout tree from a styled node tree.
func BuildLayoutTree(styledNode *StyledNode, containingBlock Dimensions) *LayoutBox {
	// TODO: Implement layout tree construction
	return &LayoutBox{}
}

// PaddingBox returns the area covered by content and padding.
func (d *Dimensions) PaddingBox() Rect {
	return d.Content.ExpandedBy(d.Padding)
}

// BorderBox returns the area covered by content, padding, and border.
func (d *Dimensions) BorderBox() Rect {
	return d.PaddingBox().ExpandedBy(d.Border)
}

// MarginBox returns the area covered by content, padding, border, and margin.
func (d *Dimensions) MarginBox() Rect {
	return d.BorderBox().ExpandedBy(d.Margin)
}

// ExpandedBy returns a rectangle expanded by the given edge sizes.
func (r Rect) ExpandedBy(edge EdgeSizes) Rect {
	return Rect{
		X:      r.X - edge.Left,
		Y:      r.Y - edge.Top,
		Width:  r.Width + edge.Left + edge.Right,
		Height: r.Height + edge.Top + edge.Bottom,
	}
}
