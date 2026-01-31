// Package dom provides DOM implementation.
package dom

// DOMRect represents a rectangle in the DOM.
// It implements the DOMRect interface per the Geometry Interfaces spec.
// https://drafts.fxtf.org/geometry/#DOMRect
type DOMRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// NewDOMRect creates a new DOMRect with the given dimensions.
func NewDOMRect(x, y, width, height float64) *DOMRect {
	return &DOMRect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

// Top returns the top edge (y for positive height, y + height for negative).
func (r *DOMRect) Top() float64 {
	if r.Height < 0 {
		return r.Y + r.Height
	}
	return r.Y
}

// Right returns the right edge (x + width for positive width, x for negative).
func (r *DOMRect) Right() float64 {
	if r.Width < 0 {
		return r.X
	}
	return r.X + r.Width
}

// Bottom returns the bottom edge (y + height for positive height, y for negative).
func (r *DOMRect) Bottom() float64 {
	if r.Height < 0 {
		return r.Y
	}
	return r.Y + r.Height
}

// Left returns the left edge (x for positive width, x + width for negative).
func (r *DOMRect) Left() float64 {
	if r.Width < 0 {
		return r.X + r.Width
	}
	return r.X
}

// DOMRectReadOnly is a read-only version of DOMRect.
// In this implementation, both are the same since Go doesn't have read-only types,
// but the API exposes them differently.
type DOMRectReadOnly = DOMRect

// DOMRectList represents a list of DOMRect objects.
type DOMRectList struct {
	items []*DOMRect
}

// NewDOMRectList creates a new DOMRectList.
func NewDOMRectList(items []*DOMRect) *DOMRectList {
	return &DOMRectList{items: items}
}

// Length returns the number of rectangles.
func (l *DOMRectList) Length() int {
	return len(l.items)
}

// Item returns the rectangle at the given index, or nil if out of bounds.
func (l *DOMRectList) Item(index int) *DOMRect {
	if index < 0 || index >= len(l.items) {
		return nil
	}
	return l.items[index]
}
