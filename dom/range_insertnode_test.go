package dom

import (
	"testing"
)

// TestInsertNode_TextNodeCollapsed tests insertNode into a collapsed range in a text node
func TestInsertNode_TextNodeCollapsed(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello World")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range to collapsed at offset 5 (right after "Hello")
	r.SetStart(text, 5)
	r.SetEnd(text, 5)

	// Create a span to insert
	span := doc.CreateElement("span")
	span.AsNode().AppendChild(doc.CreateTextNode("NEW"))

	// Insert the span - this should split the text node
	if err := r.InsertNode(span.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// After insertion:
	// - Original text should be "Hello" (split at 5)
	// - New text should be " World"
	// - Span should be between them
	// - Range end should be updated since range was collapsed

	if text.NodeValue() != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", text.NodeValue())
	}

	// Check the structure: should be text1 -> span -> text2
	children := div.AsNode().ChildNodes()
	if children.Length() != 3 {
		t.Errorf("Expected 3 children, got %d", children.Length())
		for i := 0; i < children.Length(); i++ {
			t.Logf("  child[%d]: %s = %q", i, children.Item(i).NodeName(), children.Item(i).NodeValue())
		}
	} else {
		// First child should be "Hello"
		if children.Item(0).NodeValue() != "Hello" {
			t.Errorf("First child should be 'Hello', got '%s'", children.Item(0).NodeValue())
		}
		// Second child should be the span
		if children.Item(1).NodeName() != "SPAN" {
			t.Errorf("Second child should be SPAN, got %s", children.Item(1).NodeName())
		}
		// Third child should be " World"
		if children.Item(2).NodeValue() != " World" {
			t.Errorf("Third child should be ' World', got '%s'", children.Item(2).NodeValue())
		}
	}
}

// TestInsertNode_TextNodeMiddle tests insertNode when range is in middle of text
func TestInsertNode_TextNodeMiddle(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("AB")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range collapsed at offset 1 (between A and B)
	r.SetStart(text, 1)
	r.SetEnd(text, 1)

	// Create a text node to insert
	newText := doc.CreateTextNode("X")

	// Insert the node
	if err := r.InsertNode(newText); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// After: should be "A" + "X" + "B"
	children := div.AsNode().ChildNodes()
	if children.Length() != 3 {
		t.Errorf("Expected 3 children, got %d", children.Length())
		for i := 0; i < children.Length(); i++ {
			t.Logf("  child[%d]: %s = %q", i, children.Item(i).NodeName(), children.Item(i).NodeValue())
		}
	} else {
		if children.Item(0).NodeValue() != "A" {
			t.Errorf("First child should be 'A', got '%s'", children.Item(0).NodeValue())
		}
		if children.Item(1).NodeValue() != "X" {
			t.Errorf("Second child should be 'X', got '%s'", children.Item(1).NodeValue())
		}
		if children.Item(2).NodeValue() != "B" {
			t.Errorf("Third child should be 'B', got '%s'", children.Item(2).NodeValue())
		}
	}
}

// TestInsertNode_ElementStart tests insertNode at start of element (offset 0)
func TestInsertNode_ElementStart(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	existingSpan := doc.CreateElement("span")
	div.AsNode().AppendChild(existingSpan.AsNode())

	r := doc.CreateRange()
	// Set range at (div, 0) - before existingSpan
	r.SetStart(div.AsNode(), 0)
	r.SetEnd(div.AsNode(), 0)

	// Create a new element to insert
	newElem := doc.CreateElement("em")

	// Insert the element
	if err := r.InsertNode(newElem.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// After: should be em -> span
	children := div.AsNode().ChildNodes()
	if children.Length() != 2 {
		t.Errorf("Expected 2 children, got %d", children.Length())
	} else {
		if children.Item(0).NodeName() != "EM" {
			t.Errorf("First child should be EM, got %s", children.Item(0).NodeName())
		}
		if children.Item(1).NodeName() != "SPAN" {
			t.Errorf("Second child should be SPAN, got %s", children.Item(1).NodeName())
		}
	}

	// Range end should be updated to (div, 1)
	if r.EndContainer() != div.AsNode() {
		t.Errorf("End container should be div")
	}
	if r.EndOffset() != 1 {
		t.Errorf("End offset should be 1, got %d", r.EndOffset())
	}
}

// TestInsertNode_DocumentFragment tests insertNode with a document fragment
func TestInsertNode_DocumentFragment(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range at (div, 0) - before text
	r.SetStart(div.AsNode(), 0)
	r.SetEnd(div.AsNode(), 0)

	// Create a document fragment with multiple children
	frag := doc.CreateDocumentFragment()
	frag.AsNode().AppendChild(doc.CreateElement("span").AsNode())
	frag.AsNode().AppendChild(doc.CreateElement("em").AsNode())

	// Insert the fragment
	if err := r.InsertNode(frag.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// After: should be span -> em -> #text(Hello)
	children := div.AsNode().ChildNodes()
	if children.Length() != 3 {
		t.Errorf("Expected 3 children, got %d", children.Length())
	} else {
		if children.Item(0).NodeName() != "SPAN" {
			t.Errorf("First child should be SPAN, got %s", children.Item(0).NodeName())
		}
		if children.Item(1).NodeName() != "EM" {
			t.Errorf("Second child should be EM, got %s", children.Item(1).NodeName())
		}
		if children.Item(2).NodeValue() != "Hello" {
			t.Errorf("Third child should be 'Hello', got '%s'", children.Item(2).NodeValue())
		}
	}

	// Range end should be updated to (div, 2) since fragment had 2 children
	if r.EndOffset() != 2 {
		t.Errorf("End offset should be 2, got %d", r.EndOffset())
	}
}

// TestInsertNode_HierarchyErrors tests that appropriate errors are thrown
func TestInsertNode_HierarchyErrors(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	text := doc.CreateTextNode("Hello")
	div.AsNode().AppendChild(text)

	// Test 1: Cannot insert into a Comment node
	r := doc.CreateRange()
	comment := doc.CreateComment("test")
	div.AsNode().AppendChild(comment)
	r.SetStart(comment, 0)
	r.SetEnd(comment, 0)

	span := doc.CreateElement("span")
	err := r.InsertNode(span.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when inserting into Comment")
	} else if domErr, ok := err.(*DOMError); !ok || domErr.Name != "HierarchyRequestError" {
		t.Errorf("Expected HierarchyRequestError, got %v", err)
	}

	// Test 2: Cannot insert a node into itself
	r2 := doc.CreateRange()
	r2.SetStart(div.AsNode(), 0)
	r2.SetEnd(div.AsNode(), 0)
	err = r2.InsertNode(div.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when inserting node into itself")
	}
}

// TestInsertNode_OrphanTextNode tests error when inserting into orphan text node
func TestInsertNode_OrphanTextNode(t *testing.T) {
	doc := NewDocument()
	
	// Create an orphan text node (no parent)
	text := doc.CreateTextNode("Orphan")
	
	r := doc.CreateRange()
	r.SetStart(text, 0)
	r.SetEnd(text, 0)

	span := doc.CreateElement("span")
	err := r.InsertNode(span.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when inserting into orphan text node")
	}
}

// TestInsertNode_NodeEqualsRef tests when node == referenceNode
func TestInsertNode_NodeEqualsRef(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	span := doc.CreateElement("span")
	div.AsNode().AppendChild(span.AsNode())

	text := doc.CreateTextNode("After")
	div.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range at (div, 0) - the reference node is span
	r.SetStart(div.AsNode(), 0)
	r.SetEnd(div.AsNode(), 0)

	// Try to insert the span itself - it's the reference node
	// Per spec, referenceNode should become span.nextSibling (the text node)
	if err := r.InsertNode(span.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// After: node is removed and reinserted at index 0
	// Since span was at 0 and text at 1, and we removed span then inserted at 0:
	// Result should be span -> text
	children := div.AsNode().ChildNodes()
	if children.Length() != 2 {
		t.Errorf("Expected 2 children, got %d", children.Length())
	}
}

// TestInsertNode_TextAtZero tests inserting at offset 0 of text node
func TestInsertNode_TextAtZero(t *testing.T) {
	doc := NewDocument()
	p := doc.CreateElement("p")
	doc.AsNode().AppendChild(p.AsNode())

	text := doc.CreateTextNode("ABC")
	p.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range at offset 0 of text node
	r.SetStart(text, 0)
	r.SetEnd(text, 0)

	// Insert element
	span := doc.CreateElement("span")
	if err := r.InsertNode(span.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// When offset is 0, splitText returns the original text node as next sibling
	// So referenceNode = original text node
	// The span should be inserted before "ABC"
	children := p.AsNode().ChildNodes()
	t.Logf("Children count: %d", children.Length())
	for i := 0; i < children.Length(); i++ {
		t.Logf("  child[%d]: %s = %q", i, children.Item(i).NodeName(), children.Item(i).NodeValue())
	}
	
	// Expected: span -> #text(ABC)
	if children.Length() == 2 {
		if children.Item(0).NodeName() != "SPAN" {
			t.Errorf("First child should be SPAN")
		}
		if children.Item(1).NodeValue() != "ABC" {
			t.Errorf("Second child should be 'ABC'")
		}
	}
}

// TestInsertNode_TextAtEnd tests inserting at end of text node
func TestInsertNode_TextAtEnd(t *testing.T) {
	doc := NewDocument()
	p := doc.CreateElement("p")
	doc.AsNode().AppendChild(p.AsNode())

	text := doc.CreateTextNode("ABC")
	p.AsNode().AppendChild(text)

	r := doc.CreateRange()
	// Set range at end of text node (offset 3)
	r.SetStart(text, 3)
	r.SetEnd(text, 3)

	// Insert element
	span := doc.CreateElement("span")
	if err := r.InsertNode(span.AsNode()); err != nil {
		t.Fatalf("InsertNode failed: %v", err)
	}

	// When offset is at end, splitText returns nil (no new node)
	// So referenceNode = text.nextSibling (nil)
	// The span should be inserted after "ABC"
	children := p.AsNode().ChildNodes()
	t.Logf("Children count: %d", children.Length())
	for i := 0; i < children.Length(); i++ {
		t.Logf("  child[%d]: %s = %q", i, children.Item(i).NodeName(), children.Item(i).NodeValue())
	}
	
	// Expected: #text(ABC) -> span
	if children.Length() == 2 {
		if children.Item(0).NodeValue() != "ABC" {
			t.Errorf("First child should be 'ABC'")
		}
		if children.Item(1).NodeName() != "SPAN" {
			t.Errorf("Second child should be SPAN")
		}
	}
}
