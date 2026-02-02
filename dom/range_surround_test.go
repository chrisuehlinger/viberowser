package dom

import (
	"fmt"
	"testing"
)

func TestSurroundContentsBasic(t *testing.T) {
	// Create a simple document
	doc, _ := ParseHTML("<html><body><p id='test'>Hello World</p></body></html>")
	
	// Create a range
	r := NewRange(doc)
	
	// Get the text node inside <p>
	body := doc.Body()
	p := body.AsNode().FirstChild()
	text := p.FirstChild()
	
	fmt.Printf("Text node: %q\n", text.NodeValue())
	
	// Set range to select "World"
	err := r.SetStart(text, 6)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text, 11)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	fmt.Printf("Range: %q\n", r.ToString())
	
	// Create a new element to wrap
	span := doc.CreateElement("span")
	
	// Test surroundContents
	err = r.SurroundContents(span.AsNode())
	if err != nil {
		t.Fatal("SurroundContents error:", err)
	}
	
	// Check the result
	fmt.Printf("P text content: %s\n", p.TextContent())
	fmt.Printf("P children count: %d\n", p.ChildNodes().Length())
	
	for child := p.FirstChild(); child != nil; child = child.NextSibling() {
		fmt.Printf("  Child: type=%d, value=%q, nodeName=%s\n", child.NodeType(), child.NodeValue(), child.NodeName())
	}
	
	// After surroundContents, range should select the newParent
	fmt.Printf("\nRange after surroundContents:\n")
	fmt.Printf("  startContainer: %s\n", r.StartContainer().NodeName())
	fmt.Printf("  startOffset: %d\n", r.StartOffset())
	fmt.Printf("  endContainer: %s\n", r.EndContainer().NodeName())
	fmt.Printf("  endOffset: %d\n", r.EndOffset())
}

// Test that partially contained non-Text nodes throw InvalidStateError
func TestSurroundContentsPartiallyContainedNonText(t *testing.T) {
	// Create HTML: <div><p>ab</p>cd</div>
	doc, _ := ParseHTML("<html><body><div id='test'><p>ab</p>cd</div></body></html>")
	
	r := NewRange(doc)
	
	body := doc.Body()
	div := body.AsNode().FirstChild()
	p := div.FirstChild()
	pText := p.FirstChild()
	
	// Get the text node "cd" after the <p>
	textAfter := p.NextSibling()
	if textAfter == nil {
		t.Fatal("Expected text node after p")
	}
	
	// Range from inside <p> text to text after </p>
	// This means <p> is partially contained (non-Text)
	err := r.SetStart(pText, 1) // start at "b" inside p
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(textAfter, 1) // end at "c" in "cd"
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	fmt.Printf("Range contents: %q\n", r.ToString())
	
	// Create wrapper
	span := doc.CreateElement("span")
	
	// This should throw InvalidStateError because <p> is partially contained
	err = r.SurroundContents(span.AsNode())
	if err == nil {
		t.Fatal("Expected InvalidStateError for partially contained non-Text node")
	}
	
	domErr, ok := err.(*DOMError)
	if !ok {
		t.Fatalf("Expected DOMError, got %T: %v", err, err)
	}
	
	if domErr.Name != "InvalidStateError" {
		t.Fatalf("Expected InvalidStateError, got %s", domErr.Name)
	}
	
	fmt.Printf("Correctly threw: %s\n", domErr.Name)
}

// Test that Document, DocumentType, DocumentFragment throw InvalidNodeTypeError
func TestSurroundContentsInvalidNewParent(t *testing.T) {
	doc, _ := ParseHTML("<html><body><p>Hello</p></body></html>")
	
	r := NewRange(doc)
	
	body := doc.Body()
	p := body.AsNode().FirstChild()
	text := p.FirstChild()
	
	err := r.SetStart(text, 0)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text, 5)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	// Test with DocumentFragment - should throw InvalidNodeTypeError
	docfrag := doc.CreateDocumentFragment()
	err = r.SurroundContents((*Node)(docfrag))
	if err == nil {
		t.Fatal("Expected InvalidNodeTypeError for DocumentFragment")
	}
	
	domErr, ok := err.(*DOMError)
	if !ok {
		t.Fatalf("Expected DOMError, got %T: %v", err, err)
	}
	
	if domErr.Name != "InvalidNodeTypeError" {
		t.Fatalf("Expected InvalidNodeTypeError, got %s", domErr.Name)
	}
	
	fmt.Printf("DocumentFragment correctly threw: %s\n", domErr.Name)
	
	// Test with Document - should throw InvalidNodeTypeError
	err = r.SurroundContents(doc.AsNode())
	if err == nil {
		t.Fatal("Expected InvalidNodeTypeError for Document")
	}
	
	domErr, ok = err.(*DOMError)
	if !ok {
		t.Fatalf("Expected DOMError, got %T: %v", err, err)
	}
	
	if domErr.Name != "InvalidNodeTypeError" {
		t.Fatalf("Expected InvalidNodeTypeError, got %s", domErr.Name)
	}
	
	fmt.Printf("Document correctly threw: %s\n", domErr.Name)
}

// Test surroundContents clears newParent's children
func TestSurroundContentsClearsNewParent(t *testing.T) {
	doc, _ := ParseHTML("<html><body><p>Hello</p></body></html>")
	
	r := NewRange(doc)
	
	body := doc.Body()
	p := body.AsNode().FirstChild()
	text := p.FirstChild()
	
	err := r.SetStart(text, 0)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text, 5)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	// Create a span that already has children
	span := doc.CreateElement("span")
	spanChild := doc.CreateTextNode("existing content")
	span.AsNode().AppendChild(spanChild)
	
	fmt.Printf("Span children before: %d\n", span.AsNode().ChildNodes().Length())
	
	// surroundContents should clear the span's existing children
	err = r.SurroundContents(span.AsNode())
	if err != nil {
		t.Fatal("SurroundContents error:", err)
	}
	
	// Check that the span now contains the range contents
	fmt.Printf("Span children after: %d\n", span.AsNode().ChildNodes().Length())
	fmt.Printf("Span text content: %q\n", span.AsNode().TextContent())
	
	if span.AsNode().TextContent() != "Hello" {
		t.Fatalf("Expected span content 'Hello', got %q", span.AsNode().TextContent())
	}
}

// Test surroundContents with CDATA sections (they should be treated like text)
func TestSurroundContentsCDATA(t *testing.T) {
	// Create an XML document (CDATA only works in XML, not HTML)
	impl := NewDOMImplementation(nil)
	xmlDoc, err := impl.CreateDocument("http://example.com", "root", nil)
	if err != nil {
		t.Fatalf("Failed to create XML document: %v", err)
	}

	root := xmlDoc.DocumentElement()

	// Create CDATA section
	cdata := xmlDoc.CreateCDATASection("Hello World")
	if cdata == nil {
		t.Fatal("Failed to create CDATA section")
	}
	root.AsNode().AppendChild(cdata)

	r := NewRange(xmlDoc)

	// Range from inside CDATA
	err = r.SetStart(cdata, 0)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(cdata, 5)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	// Create wrapper element
	wrapper := xmlDoc.CreateElement("wrapper")
	
	// This should work because CDATA is treated like text
	err = r.SurroundContents(wrapper.AsNode())
	if err != nil {
		t.Fatalf("SurroundContents should work with CDATA sections: %v", err)
	}
	
	fmt.Println("CDATA surroundContents succeeded")
}

// Test that ProcessingInstruction and Comment are treated as character data
// but NOT as "text" for the partially contained check
func TestSurroundContentsCharacterDataPartiallyContained(t *testing.T) {
	// Create a document with Comment and ProcessingInstruction inside an element
	doc, _ := ParseHTML(`<html><body><div id="test">text<!--comment-->more</div></body></html>`)
	
	r := NewRange(doc)
	
	body := doc.Body()
	div := body.AsNode().FirstChild()
	
	// Get the nodes: text, comment, text
	text1 := div.FirstChild()        // "text"
	comment := text1.NextSibling()    // <!--comment-->
	text2 := comment.NextSibling()    // "more"
	
	fmt.Printf("Nodes: %s, %s, %s\n", text1.NodeName(), comment.NodeName(), text2.NodeName())
	
	// Range spanning from text1 to text2
	// This means the comment is "contained", not "partially contained"
	err := r.SetStart(text1, 2)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text2, 2)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}
	
	// Create wrapper
	wrapper := doc.CreateElement("span")
	
	// This should work - comment is fully contained
	err = r.SurroundContents(wrapper.AsNode())
	if err != nil {
		t.Fatalf("SurroundContents error: %v", err)
	}
	
	fmt.Printf("Result: %s\n", div.TextContent())
}

// Test surroundContents with empty (collapsed) range
func TestSurroundContentsEmptyRange(t *testing.T) {
	doc, _ := ParseHTML("<html><body><p>Hello World</p></body></html>")

	r := NewRange(doc)

	body := doc.Body()
	p := body.AsNode().FirstChild()
	text := p.FirstChild()

	// Set range to collapsed (empty) position
	err := r.SetStart(text, 5)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text, 5)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}

	if !r.Collapsed() {
		t.Fatal("Expected range to be collapsed")
	}

	// Create wrapper
	span := doc.CreateElement("span")

	// surroundContents on empty range should insert an empty wrapper
	err = r.SurroundContents(span.AsNode())
	if err != nil {
		t.Fatalf("SurroundContents error on empty range: %v", err)
	}

	// Check that the span was inserted
	fmt.Printf("P children count: %d\n", p.ChildNodes().Length())

	// The range should now select the empty span
	if r.StartContainer() != p {
		t.Errorf("Expected startContainer to be P, got %s", r.StartContainer().NodeName())
	}

	fmt.Println("Empty range surroundContents succeeded")
}

// Test that surroundContents properly selects the new parent after operation
func TestSurroundContentsSelectsNewParent(t *testing.T) {
	doc, _ := ParseHTML("<html><body><p>Hello World</p></body></html>")

	r := NewRange(doc)

	body := doc.Body()
	p := body.AsNode().FirstChild()
	text := p.FirstChild()

	// Set range to "World"
	err := r.SetStart(text, 6)
	if err != nil {
		t.Fatal("SetStart error:", err)
	}
	err = r.SetEnd(text, 11)
	if err != nil {
		t.Fatal("SetEnd error:", err)
	}

	// Create wrapper
	span := doc.CreateElement("span")

	err = r.SurroundContents(span.AsNode())
	if err != nil {
		t.Fatal("SurroundContents error:", err)
	}

	// After surroundContents, range should "select" the newParent
	// This means startContainer and endContainer should be newParent's parent (p)
	// and the offsets should span just the newParent
	if r.StartContainer() != p {
		t.Errorf("Expected startContainer to be P, got %s", r.StartContainer().NodeName())
	}
	if r.EndContainer() != p {
		t.Errorf("Expected endContainer to be P, got %s", r.EndContainer().NodeName())
	}

	// The span should be at index 1 (after the "Hello " text node)
	expectedStartOffset := 1
	expectedEndOffset := 2

	if r.StartOffset() != expectedStartOffset {
		t.Errorf("Expected startOffset %d, got %d", expectedStartOffset, r.StartOffset())
	}
	if r.EndOffset() != expectedEndOffset {
		t.Errorf("Expected endOffset %d, got %d", expectedEndOffset, r.EndOffset())
	}

	fmt.Printf("Range correctly selects newParent: (%s, %d) to (%s, %d)\n",
		r.StartContainer().NodeName(), r.StartOffset(),
		r.EndContainer().NodeName(), r.EndOffset())
}
