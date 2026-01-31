package dom

import (
	"testing"
)

func TestNewSelection(t *testing.T) {
	doc := NewDocument()
	sel := doc.GetSelection()

	if sel == nil {
		t.Fatal("GetSelection returned nil")
	}

	if sel.RangeCount() != 0 {
		t.Errorf("Expected rangeCount 0, got %d", sel.RangeCount())
	}

	if !sel.IsCollapsed() {
		t.Error("Selection should be collapsed when empty")
	}

	if sel.Type() != "None" {
		t.Errorf("Expected type 'None', got %s", sel.Type())
	}

	if sel.AnchorNode() != nil {
		t.Error("AnchorNode should be nil for empty selection")
	}

	if sel.FocusNode() != nil {
		t.Error("FocusNode should be nil for empty selection")
	}
}

func TestSelection_AddRange(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	sel := doc.GetSelection()
	r := doc.CreateRange()

	if err := r.SetStart(text, 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}
	if err := r.SetEnd(text, 5); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}

	sel.AddRange(r)

	if sel.RangeCount() != 1 {
		t.Errorf("Expected rangeCount 1, got %d", sel.RangeCount())
	}

	if sel.Type() != "Range" {
		t.Errorf("Expected type 'Range', got %s", sel.Type())
	}

	if sel.AnchorNode() != text {
		t.Error("AnchorNode should be the text node")
	}

	if sel.AnchorOffset() != 0 {
		t.Errorf("Expected anchorOffset 0, got %d", sel.AnchorOffset())
	}

	if sel.FocusNode() != text {
		t.Error("FocusNode should be the text node")
	}

	if sel.FocusOffset() != 5 {
		t.Errorf("Expected focusOffset 5, got %d", sel.FocusOffset())
	}

	if sel.ToString() != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", sel.ToString())
	}
}

func TestSelection_RemoveAllRanges(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	sel := doc.GetSelection()
	r := doc.CreateRange()

	if err := r.SetStart(text, 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}
	if err := r.SetEnd(text, 5); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}

	sel.AddRange(r)

	if sel.RangeCount() != 1 {
		t.Errorf("Expected rangeCount 1 before removeAllRanges, got %d", sel.RangeCount())
	}

	sel.RemoveAllRanges()

	if sel.RangeCount() != 0 {
		t.Errorf("Expected rangeCount 0 after removeAllRanges, got %d", sel.RangeCount())
	}

	if sel.Type() != "None" {
		t.Errorf("Expected type 'None' after removeAllRanges, got %s", sel.Type())
	}
}

func TestSelection_Collapse(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	sel := doc.GetSelection()

	if err := sel.Collapse(text, 5); err != nil {
		t.Fatalf("Collapse failed: %v", err)
	}

	if sel.RangeCount() != 1 {
		t.Errorf("Expected rangeCount 1 after collapse, got %d", sel.RangeCount())
	}

	if !sel.IsCollapsed() {
		t.Error("Selection should be collapsed after Collapse")
	}

	if sel.Type() != "Caret" {
		t.Errorf("Expected type 'Caret' after collapse, got %s", sel.Type())
	}

	if sel.AnchorNode() != text {
		t.Error("AnchorNode should be the text node")
	}

	if sel.AnchorOffset() != 5 {
		t.Errorf("Expected anchorOffset 5, got %d", sel.AnchorOffset())
	}
}

func TestSelection_GetRangeAt(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	sel := doc.GetSelection()
	r := doc.CreateRange()

	if err := r.SetStart(text, 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}
	if err := r.SetEnd(text, 5); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}

	sel.AddRange(r)

	// GetRangeAt(0) should return the added range
	got, err := sel.GetRangeAt(0)
	if err != nil {
		t.Fatalf("GetRangeAt(0) failed: %v", err)
	}
	if got != r {
		t.Error("GetRangeAt(0) should return the same range that was added")
	}

	// GetRangeAt(1) should return an error
	_, err = sel.GetRangeAt(1)
	if err == nil {
		t.Error("GetRangeAt(1) should return an error for empty index")
	}
}

func TestSelection_SelectAllChildren(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text1 := doc.CreateTextNode("Hello ")
	div.AsNode().AppendChild(text1)

	span := doc.CreateElement("span")
	span.AsNode().AppendChild(doc.CreateTextNode("World"))
	div.AsNode().AppendChild(span.AsNode())

	sel := doc.GetSelection()

	if err := sel.SelectAllChildren(div.AsNode()); err != nil {
		t.Fatalf("SelectAllChildren failed: %v", err)
	}

	if sel.RangeCount() != 1 {
		t.Errorf("Expected rangeCount 1 after selectAllChildren, got %d", sel.RangeCount())
	}

	if sel.IsCollapsed() {
		t.Error("Selection should not be collapsed after SelectAllChildren with children")
	}

	// The selection should contain "Hello World"
	str := sel.ToString()
	if str != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", str)
	}
}

func TestSelection_ContainsNode(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text1 := doc.CreateTextNode("Hello ")
	div.AsNode().AppendChild(text1)

	span := doc.CreateElement("span")
	span.AsNode().AppendChild(doc.CreateTextNode("World"))
	div.AsNode().AppendChild(span.AsNode())

	sel := doc.GetSelection()
	r := doc.CreateRange()

	// Select around text1 - from start of div to just after text1
	if err := r.SetStart(div.AsNode(), 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}
	if err := r.SetEnd(div.AsNode(), 1); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}

	sel.AddRange(r)

	// text1 should be fully contained (at index 0)
	if !sel.ContainsNode(text1, false) {
		t.Error("text1 should be fully contained in selection")
	}

	// span should not be fully contained (at index 1, but range ends at 1)
	if sel.ContainsNode(span.AsNode(), false) {
		t.Error("span should not be fully contained in selection")
	}

	// span is at index 1, and range ends at offset 1 in div, so they touch at the boundary
	// According to the spec, this counts as intersection for partialContainment=true
	// Let's test with a range that doesn't touch span at all
	sel.RemoveAllRanges()
	r2 := doc.CreateRange()
	if err := r2.SetStart(text1, 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}
	if err := r2.SetEnd(text1, 3); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}
	sel.AddRange(r2)

	// text1 should partially intersect (selection is inside text1)
	if !sel.ContainsNode(text1, true) {
		t.Error("text1 should intersect selection with partialContainment=true")
	}

	// span should not intersect at all
	if sel.ContainsNode(span.AsNode(), true) {
		t.Error("span should not intersect when range is inside text1")
	}
}

func TestSelection_Identity(t *testing.T) {
	doc := NewDocument()

	sel1 := doc.GetSelection()
	sel2 := doc.GetSelection()

	if sel1 != sel2 {
		t.Error("GetSelection should return the same Selection object for the same document")
	}
}
