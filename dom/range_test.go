package dom

import (
	"testing"
)

func TestNewRange(t *testing.T) {
	doc := NewDocument()
	r := doc.CreateRange()

	if r == nil {
		t.Fatal("CreateRange returned nil")
	}

	if r.StartContainer() != doc.AsNode() {
		t.Error("StartContainer should be document")
	}
	if r.StartOffset() != 0 {
		t.Error("StartOffset should be 0")
	}
	if r.EndContainer() != doc.AsNode() {
		t.Error("EndContainer should be document")
	}
	if r.EndOffset() != 0 {
		t.Error("EndOffset should be 0")
	}
	if !r.Collapsed() {
		t.Error("Range should be collapsed")
	}
}

func TestRange_SetStartEnd(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()

	// Set start to position 0 of text
	if err := r.SetStart(text, 0); err != nil {
		t.Fatalf("SetStart failed: %v", err)
	}

	// Set end to position 5 of text
	if err := r.SetEnd(text, 5); err != nil {
		t.Fatalf("SetEnd failed: %v", err)
	}

	if r.StartContainer() != text {
		t.Error("StartContainer should be text node")
	}
	if r.StartOffset() != 0 {
		t.Error("StartOffset should be 0")
	}
	if r.EndContainer() != text {
		t.Error("EndContainer should be text node")
	}
	if r.EndOffset() != 5 {
		t.Error("EndOffset should be 5")
	}
	if r.Collapsed() {
		t.Error("Range should not be collapsed")
	}
}

func TestRange_SetStartBefore_SetStartAfter(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	span1 := doc.CreateElement("span")
	span2 := doc.CreateElement("span")
	div.AsNode().AppendChild(span1.AsNode())
	div.AsNode().AppendChild(span2.AsNode())

	r := doc.CreateRange()

	// Set start before span2
	if err := r.SetStartBefore(span2.AsNode()); err != nil {
		t.Fatalf("SetStartBefore failed: %v", err)
	}

	if r.StartContainer() != div.AsNode() {
		t.Error("StartContainer should be div")
	}
	if r.StartOffset() != 1 {
		t.Errorf("StartOffset should be 1, got %d", r.StartOffset())
	}

	// Set start after span1
	if err := r.SetStartAfter(span1.AsNode()); err != nil {
		t.Fatalf("SetStartAfter failed: %v", err)
	}

	if r.StartContainer() != div.AsNode() {
		t.Error("StartContainer should be div")
	}
	if r.StartOffset() != 1 {
		t.Errorf("StartOffset should be 1, got %d", r.StartOffset())
	}
}

func TestRange_Collapse(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	// Collapse to start
	r.Collapse(true)
	if !r.Collapsed() {
		t.Error("Range should be collapsed")
	}
	if r.EndOffset() != 0 {
		t.Error("EndOffset should be 0 after collapse to start")
	}

	// Reset and collapse to end
	r.SetEnd(text, 5)
	r.Collapse(false)
	if !r.Collapsed() {
		t.Error("Range should be collapsed")
	}
	if r.StartOffset() != 5 {
		t.Error("StartOffset should be 5 after collapse to end")
	}
}

func TestRange_SelectNode(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	span := doc.CreateElement("span")
	div.AsNode().AppendChild(span.AsNode())

	r := doc.CreateRange()
	if err := r.SelectNode(span.AsNode()); err != nil {
		t.Fatalf("SelectNode failed: %v", err)
	}

	if r.StartContainer() != div.AsNode() {
		t.Error("StartContainer should be div")
	}
	if r.StartOffset() != 0 {
		t.Errorf("StartOffset should be 0, got %d", r.StartOffset())
	}
	if r.EndOffset() != 1 {
		t.Errorf("EndOffset should be 1, got %d", r.EndOffset())
	}
}

func TestRange_SelectNodeContents(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	if err := r.SelectNodeContents(text); err != nil {
		t.Fatalf("SelectNodeContents failed: %v", err)
	}

	if r.StartContainer() != text {
		t.Error("StartContainer should be text node")
	}
	if r.StartOffset() != 0 {
		t.Error("StartOffset should be 0")
	}
	if r.EndOffset() != 5 {
		t.Errorf("EndOffset should be 5, got %d", r.EndOffset())
	}
}

func TestRange_ToString(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	result := r.ToString()
	if result != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", result)
	}

	// Test with different range
	r.SetStart(text, 6)
	r.SetEnd(text, 11)

	result = r.ToString()
	if result != "World" {
		t.Errorf("Expected 'World', got '%s'", result)
	}
}

func TestRange_CloneRange(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 1)
	r.SetEnd(text, 4)

	clone := r.CloneRange()

	if clone.StartContainer() != r.StartContainer() {
		t.Error("Clone should have same start container")
	}
	if clone.StartOffset() != r.StartOffset() {
		t.Error("Clone should have same start offset")
	}
	if clone.EndContainer() != r.EndContainer() {
		t.Error("Clone should have same end container")
	}
	if clone.EndOffset() != r.EndOffset() {
		t.Error("Clone should have same end offset")
	}

	// Modify original, clone should be unchanged
	r.SetStart(text, 0)
	if clone.StartOffset() != 1 {
		t.Error("Clone should be independent of original")
	}
}

func TestRange_CommonAncestorContainer(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	span1 := doc.CreateElement("span")
	span2 := doc.CreateElement("span")
	div.AsNode().AppendChild(span1.AsNode())
	div.AsNode().AppendChild(span2.AsNode())

	text1 := doc.CreateTextNode("Hello")
	text2 := doc.CreateTextNode("World")
	span1.AsNode().AppendChild(text1)
	span2.AsNode().AppendChild(text2)

	r := doc.CreateRange()
	r.SetStart(text1, 0)
	r.SetEnd(text2, 5)

	ancestor := r.CommonAncestorContainer()
	if ancestor != div.AsNode() {
		t.Error("Common ancestor should be div")
	}
}

func TestRange_CompareBoundaryPoints(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r1 := doc.CreateRange()
	r1.SetStart(text, 0)
	r1.SetEnd(text, 5)

	r2 := doc.CreateRange()
	r2.SetStart(text, 6)
	r2.SetEnd(text, 11)

	// r1 start is before r2 start
	result, err := r1.CompareBoundaryPoints(StartToStart, r2)
	if err != nil {
		t.Fatalf("CompareBoundaryPoints failed: %v", err)
	}
	if result != -1 {
		t.Errorf("Expected -1, got %d", result)
	}

	// r1 end is before r2 end
	result, err = r1.CompareBoundaryPoints(EndToEnd, r2)
	if err != nil {
		t.Fatalf("CompareBoundaryPoints failed: %v", err)
	}
	if result != -1 {
		t.Errorf("Expected -1, got %d", result)
	}
}

func TestRange_DeleteContents(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	if err := r.DeleteContents(); err != nil {
		t.Fatalf("DeleteContents failed: %v", err)
	}

	if text.NodeValue() != " World" {
		t.Errorf("Expected ' World', got '%s'", text.NodeValue())
	}
}

func TestRange_CloneContents(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	frag, err := r.CloneContents()
	if err != nil {
		t.Fatalf("CloneContents failed: %v", err)
	}

	if frag == nil {
		t.Fatal("CloneContents returned nil")
	}

	// Original should be unchanged
	if text.NodeValue() != "Hello World" {
		t.Error("Original text should be unchanged")
	}

	// Fragment should contain "Hello"
	if (*Node)(frag).FirstChild() == nil {
		t.Fatal("Fragment should have a child")
	}
	clonedText := (*Node)(frag).FirstChild().NodeValue()
	if clonedText != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", clonedText)
	}
}

func TestRange_ExtractContents(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	frag, err := r.ExtractContents()
	if err != nil {
		t.Fatalf("ExtractContents failed: %v", err)
	}

	if frag == nil {
		t.Fatal("ExtractContents returned nil")
	}

	// Original should be modified
	if text.NodeValue() != " World" {
		t.Errorf("Original text should be ' World', got '%s'", text.NodeValue())
	}

	// Fragment should contain "Hello"
	if (*Node)(frag).FirstChild() == nil {
		t.Fatal("Fragment should have a child")
	}
	extractedText := (*Node)(frag).FirstChild().NodeValue()
	if extractedText != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", extractedText)
	}
}

func TestRange_InsertNode(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 5)
	r.SetEnd(text, 5)

	span := doc.CreateElement("span")
	if err := r.InsertNode(span.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// Text should be split and span inserted
	if text.NodeValue() != "Hello" {
		t.Errorf("First text should be 'Hello', got '%s'", text.NodeValue())
	}
}

func TestRange_IndexSizeError(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()

	// Offset too large
	err := r.SetStart(text, 100)
	if err == nil {
		t.Error("Expected IndexSizeError")
	}
	if domErr, ok := err.(*DOMError); ok {
		if domErr.Name != "IndexSizeError" {
			t.Errorf("Expected IndexSizeError, got %s", domErr.Name)
		}
	}
}

func TestRange_IsPointInRange(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 5)

	// Point in range
	if !r.IsPointInRange(text, 3) {
		t.Error("Point should be in range")
	}

	// Point before range
	// This is tricky - offset 0 is at the start, so it should be in range
	if !r.IsPointInRange(text, 0) {
		t.Error("Start point should be in range")
	}

	// Point after range
	if r.IsPointInRange(text, 10) {
		t.Error("Point should not be in range")
	}
}

func TestRange_IntersectsNode(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	span1 := doc.CreateElement("span")
	span2 := doc.CreateElement("span")
	span3 := doc.CreateElement("span")
	span4 := doc.CreateElement("span")
	div.AsNode().AppendChild(span1.AsNode())
	div.AsNode().AppendChild(span2.AsNode())
	div.AsNode().AppendChild(span3.AsNode())
	div.AsNode().AppendChild(span4.AsNode())

	r := doc.CreateRange()
	// Range covers span2 only (start after span1, end before span3)
	r.SetStartAfter(span1.AsNode())  // start at (div, 1)
	r.SetEndBefore(span3.AsNode())   // end at (div, 2)

	if !r.IntersectsNode(span2.AsNode()) {
		t.Error("Range should intersect span2")
	}
	// Note: Per DOM spec, nodes that touch the range boundary still intersect
	// To truly test non-intersection, we need nodes completely outside the range
	if r.IntersectsNode(span4.AsNode()) {
		t.Error("Range should not intersect span4 (completely after range)")
	}
}
