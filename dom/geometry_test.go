package dom

import (
	"testing"
)

func TestNewDOMRect(t *testing.T) {
	rect := NewDOMRect(10, 20, 100, 50)
	if rect == nil {
		t.Fatal("NewDOMRect returned nil")
	}
	if rect.X != 10 {
		t.Errorf("Expected X=10, got %v", rect.X)
	}
	if rect.Y != 20 {
		t.Errorf("Expected Y=20, got %v", rect.Y)
	}
	if rect.Width != 100 {
		t.Errorf("Expected Width=100, got %v", rect.Width)
	}
	if rect.Height != 50 {
		t.Errorf("Expected Height=50, got %v", rect.Height)
	}
}

func TestDOMRect_Edges(t *testing.T) {
	// Test with positive dimensions
	rect := NewDOMRect(10, 20, 100, 50)
	if rect.Top() != 20 {
		t.Errorf("Expected Top=20, got %v", rect.Top())
	}
	if rect.Left() != 10 {
		t.Errorf("Expected Left=10, got %v", rect.Left())
	}
	if rect.Right() != 110 {
		t.Errorf("Expected Right=110, got %v", rect.Right())
	}
	if rect.Bottom() != 70 {
		t.Errorf("Expected Bottom=70, got %v", rect.Bottom())
	}
}

func TestDOMRect_NegativeWidth(t *testing.T) {
	// Test with negative width
	rect := NewDOMRect(100, 20, -50, 30)
	if rect.Left() != 50 {
		t.Errorf("Expected Left=50 (x + negative width), got %v", rect.Left())
	}
	if rect.Right() != 100 {
		t.Errorf("Expected Right=100 (x for negative width), got %v", rect.Right())
	}
}

func TestDOMRect_NegativeHeight(t *testing.T) {
	// Test with negative height
	rect := NewDOMRect(10, 100, 50, -30)
	if rect.Top() != 70 {
		t.Errorf("Expected Top=70 (y + negative height), got %v", rect.Top())
	}
	if rect.Bottom() != 100 {
		t.Errorf("Expected Bottom=100 (y for negative height), got %v", rect.Bottom())
	}
}

func TestDOMRectList(t *testing.T) {
	rect1 := NewDOMRect(0, 0, 10, 10)
	rect2 := NewDOMRect(20, 20, 30, 30)
	list := NewDOMRectList([]*DOMRect{rect1, rect2})

	if list.Length() != 2 {
		t.Errorf("Expected Length=2, got %v", list.Length())
	}

	first := list.Item(0)
	if first != rect1 {
		t.Error("Expected Item(0) to return rect1")
	}

	second := list.Item(1)
	if second != rect2 {
		t.Error("Expected Item(1) to return rect2")
	}

	outOfBounds := list.Item(5)
	if outOfBounds != nil {
		t.Error("Expected Item(5) to return nil for out of bounds")
	}

	negativeIndex := list.Item(-1)
	if negativeIndex != nil {
		t.Error("Expected Item(-1) to return nil for negative index")
	}
}

func TestElement_Geometry(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	// Before setting geometry, should return nil
	if el.Geometry() != nil {
		t.Error("Expected Geometry() to return nil before setting")
	}

	// GetBoundingClientRect should return zero rect when no geometry
	rect := el.GetBoundingClientRect()
	if rect.X != 0 || rect.Y != 0 || rect.Width != 0 || rect.Height != 0 {
		t.Errorf("Expected zero rect, got (%v, %v, %v, %v)", rect.X, rect.Y, rect.Width, rect.Height)
	}

	// Set geometry
	geom := &ElementGeometry{
		X:            50,
		Y:            100,
		Width:        200,
		Height:       150,
		OffsetWidth:  200,
		OffsetHeight: 150,
		OffsetTop:    100,
		OffsetLeft:   50,
		ClientWidth:  180,
		ClientHeight: 130,
		ClientTop:    5,
		ClientLeft:   5,
	}
	el.SetGeometry(geom)

	// Verify geometry is set
	if el.Geometry() != geom {
		t.Error("Geometry() should return the set geometry")
	}

	// Test GetBoundingClientRect
	rect = el.GetBoundingClientRect()
	if rect.X != 50 || rect.Y != 100 || rect.Width != 200 || rect.Height != 150 {
		t.Errorf("Expected rect (50, 100, 200, 150), got (%v, %v, %v, %v)",
			rect.X, rect.Y, rect.Width, rect.Height)
	}

	// Test GetClientRects
	rects := el.GetClientRects()
	if rects.Length() != 1 {
		t.Errorf("Expected 1 rect, got %v", rects.Length())
	}

	// Test offset properties
	if el.OffsetWidth() != 200 {
		t.Errorf("Expected OffsetWidth=200, got %v", el.OffsetWidth())
	}
	if el.OffsetHeight() != 150 {
		t.Errorf("Expected OffsetHeight=150, got %v", el.OffsetHeight())
	}
	if el.OffsetTop() != 100 {
		t.Errorf("Expected OffsetTop=100, got %v", el.OffsetTop())
	}
	if el.OffsetLeft() != 50 {
		t.Errorf("Expected OffsetLeft=50, got %v", el.OffsetLeft())
	}

	// Test client properties
	if el.ClientWidth() != 180 {
		t.Errorf("Expected ClientWidth=180, got %v", el.ClientWidth())
	}
	if el.ClientHeight() != 130 {
		t.Errorf("Expected ClientHeight=130, got %v", el.ClientHeight())
	}
	if el.ClientTop() != 5 {
		t.Errorf("Expected ClientTop=5, got %v", el.ClientTop())
	}
	if el.ClientLeft() != 5 {
		t.Errorf("Expected ClientLeft=5, got %v", el.ClientLeft())
	}
}

func TestElement_ScrollProperties(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	// Initially scroll values should be 0
	if el.ScrollTop() != 0 {
		t.Errorf("Expected ScrollTop=0, got %v", el.ScrollTop())
	}
	if el.ScrollLeft() != 0 {
		t.Errorf("Expected ScrollLeft=0, got %v", el.ScrollLeft())
	}

	// Set scroll values
	el.SetScrollTop(50)
	el.SetScrollLeft(30)

	if el.ScrollTop() != 50 {
		t.Errorf("Expected ScrollTop=50, got %v", el.ScrollTop())
	}
	if el.ScrollLeft() != 30 {
		t.Errorf("Expected ScrollLeft=30, got %v", el.ScrollLeft())
	}

	// Negative values should be clamped to 0
	el.SetScrollTop(-10)
	el.SetScrollLeft(-20)

	if el.ScrollTop() != 0 {
		t.Errorf("Expected ScrollTop=0 after setting negative, got %v", el.ScrollTop())
	}
	if el.ScrollLeft() != 0 {
		t.Errorf("Expected ScrollLeft=0 after setting negative, got %v", el.ScrollLeft())
	}
}
