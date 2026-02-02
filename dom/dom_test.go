package dom

import (
	"testing"
)

func TestNewDocument(t *testing.T) {
	doc := NewDocument()
	if doc == nil {
		t.Fatal("NewDocument returned nil")
	}
	if doc.NodeType() != DocumentNode {
		t.Errorf("Expected DocumentNode, got %v", doc.NodeType())
	}
	if doc.NodeName() != "#document" {
		t.Errorf("Expected '#document', got %s", doc.NodeName())
	}
}

func TestDocument_CreateElement(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	if el == nil {
		t.Fatal("CreateElement returned nil")
	}
	if el.TagName() != "DIV" {
		t.Errorf("Expected tagName 'DIV', got '%s'", el.TagName())
	}
	if el.LocalName() != "div" {
		t.Errorf("Expected localName 'div', got '%s'", el.LocalName())
	}
	if el.NodeType() != ElementNode {
		t.Errorf("Expected ElementNode, got %v", el.NodeType())
	}
}

func TestDocument_CreateTextNode(t *testing.T) {
	doc := NewDocument()
	text := doc.CreateTextNode("Hello, World!")

	if text == nil {
		t.Fatal("CreateTextNode returned nil")
	}
	if text.NodeType() != TextNode {
		t.Errorf("Expected TextNode, got %v", text.NodeType())
	}
	if text.NodeValue() != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", text.NodeValue())
	}
}

func TestDocument_CreateComment(t *testing.T) {
	doc := NewDocument()
	comment := doc.CreateComment("This is a comment")

	if comment == nil {
		t.Fatal("CreateComment returned nil")
	}
	if comment.NodeType() != CommentNode {
		t.Errorf("Expected CommentNode, got %v", comment.NodeType())
	}
	if comment.NodeValue() != "This is a comment" {
		t.Errorf("Expected 'This is a comment', got '%s'", comment.NodeValue())
	}
}

func TestDocument_URL(t *testing.T) {
	// Test default URL is "about:blank"
	doc := NewDocument()
	if doc.URL() != "about:blank" {
		t.Errorf("Expected default URL 'about:blank', got '%s'", doc.URL())
	}

	// Test setting URL
	doc.SetURL("https://example.com/page.html")
	if doc.URL() != "https://example.com/page.html" {
		t.Errorf("Expected 'https://example.com/page.html', got '%s'", doc.URL())
	}

	// Test DocumentURI returns same as URL
	if doc.DocumentURI() != doc.URL() {
		t.Errorf("DocumentURI() should equal URL(), got '%s' vs '%s'", doc.DocumentURI(), doc.URL())
	}
}

func TestDocument_CreateCDATASection(t *testing.T) {
	// Test creating CDATASection in XML document
	impl := NewDOMImplementation(nil)
	xmlDoc, _ := impl.CreateDocument("http://example.com", "root", nil)

	cdata, err := xmlDoc.CreateCDATASectionWithError("CDATA content here")
	if err != nil {
		t.Fatalf("CreateCDATASection returned error for XML document: %v", err)
	}
	if cdata == nil {
		t.Fatal("CreateCDATASection returned nil for XML document")
	}
	if cdata.NodeType() != CDATASectionNode {
		t.Errorf("Expected CDATASectionNode, got %v", cdata.NodeType())
	}
	if cdata.NodeName() != "#cdata-section" {
		t.Errorf("Expected '#cdata-section', got '%s'", cdata.NodeName())
	}
	if cdata.NodeValue() != "CDATA content here" {
		t.Errorf("Expected 'CDATA content here', got '%s'", cdata.NodeValue())
	}
}

func TestDocument_CreateCDATASection_HTMLDocumentThrows(t *testing.T) {
	// Test that CDATASection throws NotSupportedError for HTML documents
	doc := NewDocument()

	_, err := doc.CreateCDATASectionWithError("test")
	if err == nil {
		t.Fatal("Expected error for CDATASection in HTML document")
	}
	domErr, ok := err.(*DOMError)
	if !ok {
		t.Fatalf("Expected DOMError, got %T", err)
	}
	if domErr.Name != "NotSupportedError" {
		t.Errorf("Expected NotSupportedError, got %s", domErr.Name)
	}

	// Also test the non-error version returns nil
	node := doc.CreateCDATASection("test")
	if node != nil {
		t.Error("Expected nil for CDATASection in HTML document")
	}
}

func TestDocument_CreateCDATASection_InvalidData(t *testing.T) {
	// Test that CDATASection throws InvalidCharacterError for data containing "]]>"
	impl := NewDOMImplementation(nil)
	xmlDoc, _ := impl.CreateDocument("http://example.com", "root", nil)

	_, err := xmlDoc.CreateCDATASectionWithError("data with ]]> in it")
	if err == nil {
		t.Fatal("Expected error for data containing ']]>'")
	}
	domErr, ok := err.(*DOMError)
	if !ok {
		t.Fatalf("Expected DOMError, got %T", err)
	}
	if domErr.Name != "InvalidCharacterError" {
		t.Errorf("Expected InvalidCharacterError, got %s", domErr.Name)
	}
}

func TestDocument_CreateDocumentFragment(t *testing.T) {
	doc := NewDocument()
	frag := doc.CreateDocumentFragment()

	if frag == nil {
		t.Fatal("CreateDocumentFragment returned nil")
	}
	if frag.NodeType() != DocumentFragmentNode {
		t.Errorf("Expected DocumentFragmentNode, got %v", frag.NodeType())
	}
}

func TestElement_Attributes(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	el.SetAttribute("id", "main")
	el.SetAttribute("class", "container")
	el.SetAttribute("data-value", "123")

	if el.GetAttribute("id") != "main" {
		t.Errorf("Expected id='main', got '%s'", el.GetAttribute("id"))
	}
	if el.GetAttribute("class") != "container" {
		t.Errorf("Expected class='container', got '%s'", el.GetAttribute("class"))
	}
	if el.GetAttribute("data-value") != "123" {
		t.Errorf("Expected data-value='123', got '%s'", el.GetAttribute("data-value"))
	}
	if !el.HasAttribute("id") {
		t.Error("Expected HasAttribute('id') to be true")
	}

	el.RemoveAttribute("id")
	if el.HasAttribute("id") {
		t.Error("Expected HasAttribute('id') to be false after removal")
	}
}

func TestElement_ClassList(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	classList := el.ClassList()

	if err := classList.Add("foo", "bar", "baz"); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !classList.Contains("foo") {
		t.Error("Expected classList to contain 'foo'")
	}
	if !classList.Contains("bar") {
		t.Error("Expected classList to contain 'bar'")
	}
	if classList.Length() != 3 {
		t.Errorf("Expected 3 classes, got %d", classList.Length())
	}

	if err := classList.Remove("bar"); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if classList.Contains("bar") {
		t.Error("Expected classList not to contain 'bar' after removal")
	}

	result, err := classList.Toggle("qux")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected toggle to return true when adding")
	}
	if !classList.Contains("qux") {
		t.Error("Expected classList to contain 'qux'")
	}

	result, err = classList.Toggle("qux")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result {
		t.Error("Expected toggle to return false when removing")
	}
	if classList.Contains("qux") {
		t.Error("Expected classList not to contain 'qux' after toggle")
	}

	// Test Replace
	if err := classList.Add("old"); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if replaced, _ := classList.Replace("old", "new"); !replaced {
		t.Error("Expected Replace to return true")
	}
	if classList.Contains("old") {
		t.Error("Expected classList not to contain 'old' after replace")
	}
	if !classList.Contains("new") {
		t.Error("Expected classList to contain 'new' after replace")
	}

	// Test validation errors
	// Empty string should throw SyntaxError
	if err := classList.Add(""); err == nil {
		t.Error("Expected error for empty token")
	} else if err.Type != "SyntaxError" {
		t.Errorf("Expected SyntaxError, got %s", err.Type)
	}

	// Whitespace should throw InvalidCharacterError
	if err := classList.Add("foo bar"); err == nil {
		t.Error("Expected error for token with whitespace")
	} else if err.Type != "InvalidCharacterError" {
		t.Errorf("Expected InvalidCharacterError, got %s", err.Type)
	}

	// Tab should throw InvalidCharacterError
	if err := classList.Add("foo\tbar"); err == nil {
		t.Error("Expected error for token with tab")
	} else if err.Type != "InvalidCharacterError" {
		t.Errorf("Expected InvalidCharacterError, got %s", err.Type)
	}

	// Toggle empty should throw SyntaxError
	if _, err := classList.Toggle(""); err == nil {
		t.Error("Expected error for empty token in toggle")
	} else if err.Type != "SyntaxError" {
		t.Errorf("Expected SyntaxError, got %s", err.Type)
	}

	// Contains returns false for invalid tokens (per spec, no error)
	if classList.Contains("") {
		t.Error("Expected contains to return false for empty token")
	}
	if classList.Contains(" ") {
		t.Error("Expected contains to return false for whitespace token")
	}
}

func TestNode_AppendChild(t *testing.T) {
	doc := NewDocument()
	parent := doc.CreateElement("div")
	child1 := doc.CreateElement("p")
	child2 := doc.CreateElement("span")

	parent.AsNode().AppendChild(child1.AsNode())
	parent.AsNode().AppendChild(child2.AsNode())

	if parent.AsNode().FirstChild() != child1.AsNode() {
		t.Error("FirstChild should be child1")
	}
	if parent.AsNode().LastChild() != child2.AsNode() {
		t.Error("LastChild should be child2")
	}
	if child1.AsNode().ParentNode() != parent.AsNode() {
		t.Error("child1.ParentNode should be parent")
	}
	if child1.AsNode().NextSibling() != child2.AsNode() {
		t.Error("child1.NextSibling should be child2")
	}
	if child2.AsNode().PreviousSibling() != child1.AsNode() {
		t.Error("child2.PreviousSibling should be child1")
	}
}

func TestNode_RemoveChild(t *testing.T) {
	doc := NewDocument()
	parent := doc.CreateElement("div")
	child1 := doc.CreateElement("p")
	child2 := doc.CreateElement("span")
	child3 := doc.CreateElement("a")

	parent.AsNode().AppendChild(child1.AsNode())
	parent.AsNode().AppendChild(child2.AsNode())
	parent.AsNode().AppendChild(child3.AsNode())

	// Remove middle child
	parent.AsNode().RemoveChild(child2.AsNode())

	if child1.AsNode().NextSibling() != child3.AsNode() {
		t.Error("child1.NextSibling should be child3 after removing child2")
	}
	if child3.AsNode().PreviousSibling() != child1.AsNode() {
		t.Error("child3.PreviousSibling should be child1 after removing child2")
	}
	if child2.AsNode().ParentNode() != nil {
		t.Error("child2.ParentNode should be nil after removal")
	}
}

func TestNode_InsertBefore(t *testing.T) {
	doc := NewDocument()
	parent := doc.CreateElement("div")
	child1 := doc.CreateElement("p")
	child3 := doc.CreateElement("a")
	child2 := doc.CreateElement("span")

	parent.AsNode().AppendChild(child1.AsNode())
	parent.AsNode().AppendChild(child3.AsNode())
	parent.AsNode().InsertBefore(child2.AsNode(), child3.AsNode())

	if child1.AsNode().NextSibling() != child2.AsNode() {
		t.Error("child1.NextSibling should be child2")
	}
	if child2.AsNode().NextSibling() != child3.AsNode() {
		t.Error("child2.NextSibling should be child3")
	}
	if child2.AsNode().PreviousSibling() != child1.AsNode() {
		t.Error("child2.PreviousSibling should be child1")
	}
}

func TestNode_TextContent(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	span := doc.CreateElement("span")
	text1 := doc.CreateTextNode("Hello ")
	text2 := doc.CreateTextNode("World")

	span.AsNode().AppendChild(text2)
	div.AsNode().AppendChild(text1)
	div.AsNode().AppendChild(span.AsNode())

	if div.TextContent() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", div.TextContent())
	}
}

func TestNode_SetTextContent(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	child := doc.CreateElement("p")
	div.AsNode().AppendChild(child.AsNode())

	div.AsNode().SetTextContent("New text")

	if div.AsNode().FirstChild() == nil {
		t.Fatal("Expected a text node child")
	}
	if div.AsNode().FirstChild().NodeType() != TextNode {
		t.Error("Expected child to be a TextNode")
	}
	if div.TextContent() != "New text" {
		t.Errorf("Expected 'New text', got '%s'", div.TextContent())
	}
}

func TestNode_CloneNode(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	div.SetAttribute("id", "original")
	div.SetAttribute("class", "container")
	child := doc.CreateElement("p")
	child.SetAttribute("class", "child")
	div.AsNode().AppendChild(child.AsNode())

	// Shallow clone
	shallowClone := div.CloneNode(false)
	if shallowClone.GetAttribute("id") != "original" {
		t.Error("Shallow clone should have the same attributes")
	}
	if shallowClone.AsNode().FirstChild() != nil {
		t.Error("Shallow clone should not have children")
	}

	// Deep clone
	deepClone := div.CloneNode(true)
	if deepClone.GetAttribute("id") != "original" {
		t.Error("Deep clone should have the same attributes")
	}
	if deepClone.AsNode().FirstChild() == nil {
		t.Error("Deep clone should have children")
	}
	if (*Element)(deepClone.AsNode().FirstChild()).GetAttribute("class") != "child" {
		t.Error("Deep clone's child should have the same attributes")
	}
}

func TestDocument_GetElementById(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div := doc.CreateElement("div")
	div.SetAttribute("id", "main")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div.AsNode())

	found := doc.GetElementById("main")
	if found == nil {
		t.Fatal("GetElementById returned nil")
	}
	if found != div {
		t.Error("GetElementById returned wrong element")
	}

	notFound := doc.GetElementById("nonexistent")
	if notFound != nil {
		t.Error("GetElementById should return nil for nonexistent id")
	}
}

func TestDocument_GetElementsByTagName(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div1 := doc.CreateElement("div")
	div2 := doc.CreateElement("div")
	p := doc.CreateElement("p")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div1.AsNode())
	body.AsNode().AppendChild(div2.AsNode())
	div1.AsNode().AppendChild(p.AsNode())

	divs := doc.GetElementsByTagName("div")
	if divs.Length() != 2 {
		t.Errorf("Expected 2 divs, got %d", divs.Length())
	}

	all := doc.GetElementsByTagName("*")
	if all.Length() != 5 { // html, body, div1, div2, p
		t.Errorf("Expected 5 elements, got %d", all.Length())
	}
}

func TestDocument_GetElementsByClassName(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div1 := doc.CreateElement("div")
	div2 := doc.CreateElement("div")
	div3 := doc.CreateElement("div")

	div1.SetAttribute("class", "foo bar")
	div2.SetAttribute("class", "foo baz")
	div3.SetAttribute("class", "bar baz")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div1.AsNode())
	body.AsNode().AppendChild(div2.AsNode())
	body.AsNode().AppendChild(div3.AsNode())

	fooElements := doc.GetElementsByClassName("foo")
	if fooElements.Length() != 2 {
		t.Errorf("Expected 2 elements with class 'foo', got %d", fooElements.Length())
	}

	fooBarElements := doc.GetElementsByClassName("foo bar")
	if fooBarElements.Length() != 1 {
		t.Errorf("Expected 1 element with classes 'foo bar', got %d", fooBarElements.Length())
	}

	// Test empty string returns empty collection
	emptyElements := doc.GetElementsByClassName("")
	if emptyElements.Length() != 0 {
		t.Errorf("Expected 0 elements for empty class name, got %d", emptyElements.Length())
	}

	// Test whitespace-only returns empty collection
	whitespaceElements := doc.GetElementsByClassName("   ")
	if whitespaceElements.Length() != 0 {
		t.Errorf("Expected 0 elements for whitespace-only class name, got %d", whitespaceElements.Length())
	}
}

func TestElement_Matches(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	div.SetAttribute("id", "main")
	div.SetAttribute("class", "container active")

	tests := []struct {
		selector string
		expected bool
	}{
		{"div", true},
		{"span", false},
		{"*", true},
		{"#main", true},
		{"#other", false},
		{".container", true},
		{".active", true},
		{".nonexistent", false},
		{"div.container", true},
		{"div#main", true},
		{"div#main.container", true},
		{"div#main.container.active", true},
		{"span#main", false},
		{"[id]", true},
		{"[class]", true},
		{"[nonexistent]", false},
		{"[id=main]", true},
		{"[id=other]", false},
		{"[class~=container]", true},
		{"[class~=active]", true},
		{"[class~=nonexistent]", false},
	}

	for _, tt := range tests {
		result := div.Matches(tt.selector)
		if result != tt.expected {
			t.Errorf("Matches(%q) = %v, want %v", tt.selector, result, tt.expected)
		}
	}
}

func TestElement_QuerySelector(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div := doc.CreateElement("div")
	div.SetAttribute("class", "container")
	p := doc.CreateElement("p")
	p.SetAttribute("id", "para")
	span := doc.CreateElement("span")
	span.SetAttribute("class", "highlight")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div.AsNode())
	div.AsNode().AppendChild(p.AsNode())
	p.AsNode().AppendChild(span.AsNode())

	// Test from body
	found := body.QuerySelector(".container")
	if found == nil {
		t.Fatal("QuerySelector returned nil")
	}
	if found != div {
		t.Error("QuerySelector returned wrong element")
	}

	// Test nested
	found = body.QuerySelector("#para")
	if found == nil {
		t.Fatal("QuerySelector for #para returned nil")
	}
	if found != p {
		t.Error("QuerySelector returned wrong element for #para")
	}

	// Test deeply nested
	found = body.QuerySelector(".highlight")
	if found == nil {
		t.Fatal("QuerySelector for .highlight returned nil")
	}
	if found != span {
		t.Error("QuerySelector returned wrong element for .highlight")
	}
}

func TestElement_QuerySelectorAll(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div1 := doc.CreateElement("div")
	div2 := doc.CreateElement("div")
	div1.SetAttribute("class", "item")
	div2.SetAttribute("class", "item")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div1.AsNode())
	body.AsNode().AppendChild(div2.AsNode())

	found := body.QuerySelectorAll(".item")
	if found.Length() != 2 {
		t.Errorf("Expected 2 elements, got %d", found.Length())
	}
}

func TestElement_Closest(t *testing.T) {
	doc := NewDocument()
	html := doc.CreateElement("html")
	body := doc.CreateElement("body")
	div := doc.CreateElement("div")
	div.SetAttribute("class", "container")
	p := doc.CreateElement("p")
	span := doc.CreateElement("span")

	doc.AsNode().AppendChild(html.AsNode())
	html.AsNode().AppendChild(body.AsNode())
	body.AsNode().AppendChild(div.AsNode())
	div.AsNode().AppendChild(p.AsNode())
	p.AsNode().AppendChild(span.AsNode())

	// Test from span
	found := span.Closest(".container")
	if found == nil {
		t.Fatal("Closest returned nil")
	}
	if found != div {
		t.Error("Closest returned wrong element")
	}

	// Test matching self
	found = div.Closest(".container")
	if found != div {
		t.Error("Closest should match self")
	}

	// Test not found
	found = span.Closest(".nonexistent")
	if found != nil {
		t.Error("Closest should return nil when not found")
	}
}

func TestElement_InnerHTML(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	p := doc.CreateElement("p")
	text := doc.CreateTextNode("Hello")

	p.AsNode().AppendChild(text)
	div.AsNode().AppendChild(p.AsNode())

	innerHTML := div.InnerHTML()
	if innerHTML != "<p>Hello</p>" {
		t.Errorf("Expected '<p>Hello</p>', got '%s'", innerHTML)
	}
}

func TestElement_SetInnerHTML(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	doc.AsNode().AppendChild(div.AsNode())

	err := div.SetInnerHTML("<p>New content</p>")
	if err != nil {
		t.Fatalf("SetInnerHTML failed: %v", err)
	}

	// Check that old children are gone and new ones are added
	if div.AsNode().FirstChild() == nil {
		t.Fatal("Expected child after SetInnerHTML")
	}
	if div.AsNode().FirstChild().NodeType() != ElementNode {
		t.Error("Expected element child")
	}
	firstChild := (*Element)(div.AsNode().FirstChild())
	if firstChild.TagName() != "P" {
		t.Errorf("Expected P element, got %s", firstChild.TagName())
	}
}

func TestElement_OuterHTML(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	div.SetAttribute("id", "test")
	text := doc.CreateTextNode("Content")
	div.AsNode().AppendChild(text)

	outerHTML := div.OuterHTML()
	if outerHTML != `<div id="test">Content</div>` {
		t.Errorf("Expected '<div id=\"test\">Content</div>', got '%s'", outerHTML)
	}
}

func TestElement_InsertAdjacentHTML(t *testing.T) {
	doc := NewDocument()
	body := doc.CreateElement("body")
	doc.AsNode().AppendChild(body.AsNode())

	// Create initial structure: <body><div id="target">Content</div></body>
	div := doc.CreateElement("div")
	div.SetAttribute("id", "target")
	text := doc.CreateTextNode("Content")
	div.AsNode().AppendChild(text)
	body.AsNode().AppendChild(div.AsNode())

	// Test beforebegin
	err := div.InsertAdjacentHTML("beforebegin", "<p>Before</p>")
	if err != nil {
		t.Fatalf("InsertAdjacentHTML beforebegin failed: %v", err)
	}
	if body.AsNode().FirstChild().NodeName() != "P" {
		t.Errorf("Expected P element before div, got %s", body.AsNode().FirstChild().NodeName())
	}

	// Test afterbegin
	err = div.InsertAdjacentHTML("afterbegin", "<span>Start</span>")
	if err != nil {
		t.Fatalf("InsertAdjacentHTML afterbegin failed: %v", err)
	}
	if div.AsNode().FirstChild().NodeName() != "SPAN" {
		t.Errorf("Expected SPAN element at start of div, got %s", div.AsNode().FirstChild().NodeName())
	}

	// Test beforeend
	err = div.InsertAdjacentHTML("beforeend", "<em>End</em>")
	if err != nil {
		t.Fatalf("InsertAdjacentHTML beforeend failed: %v", err)
	}
	if div.AsNode().LastChild().NodeName() != "EM" {
		t.Errorf("Expected EM element at end of div, got %s", div.AsNode().LastChild().NodeName())
	}

	// Test afterend
	err = div.InsertAdjacentHTML("afterend", "<b>After</b>")
	if err != nil {
		t.Fatalf("InsertAdjacentHTML afterend failed: %v", err)
	}
	if div.AsNode().NextSibling().NodeName() != "B" {
		t.Errorf("Expected B element after div, got %s", div.AsNode().NextSibling().NodeName())
	}

	// Test invalid position
	err = div.InsertAdjacentHTML("invalid", "<span>Test</span>")
	if err == nil {
		t.Error("Expected error for invalid position")
	}
}

func TestText_SplitText(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	text := (*Text)(doc.CreateTextNode("Hello World"))
	div.AsNode().AppendChild(text.AsNode())

	newText := text.SplitText(6)
	if newText == nil {
		t.Fatal("SplitText returned nil")
	}
	if text.Data() != "Hello " {
		t.Errorf("Expected 'Hello ', got '%s'", text.Data())
	}
	if newText.Data() != "World" {
		t.Errorf("Expected 'World', got '%s'", newText.Data())
	}
	if text.AsNode().NextSibling() != newText.AsNode() {
		t.Error("New text node should be next sibling")
	}
}

func TestDocumentFragment_AppendToParent(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	frag := doc.CreateDocumentFragment()

	p1 := doc.CreateElement("p")
	p2 := doc.CreateElement("p")
	frag.AsNode().AppendChild(p1.AsNode())
	frag.AsNode().AppendChild(p2.AsNode())

	div.AsNode().AppendChild(frag.AsNode())

	// Fragment should be empty after appending
	if frag.AsNode().FirstChild() != nil {
		t.Error("Fragment should be empty after appending to parent")
	}

	// Children should be in div
	if div.AsNode().FirstChild() != p1.AsNode() {
		t.Error("First child of div should be p1")
	}
	if div.AsNode().LastChild() != p2.AsNode() {
		t.Error("Last child of div should be p2")
	}
}

func TestNode_Contains(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	p := doc.CreateElement("p")
	span := doc.CreateElement("span")

	div.AsNode().AppendChild(p.AsNode())
	p.AsNode().AppendChild(span.AsNode())

	if !div.AsNode().Contains(p.AsNode()) {
		t.Error("div should contain p")
	}
	if !div.AsNode().Contains(span.AsNode()) {
		t.Error("div should contain span")
	}
	if !div.AsNode().Contains(div.AsNode()) {
		t.Error("div should contain itself")
	}
	if p.AsNode().Contains(div.AsNode()) {
		t.Error("p should not contain div")
	}
}

func TestNode_Normalize(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	text1 := doc.CreateTextNode("Hello ")
	text2 := doc.CreateTextNode("World")
	text3 := doc.CreateTextNode("")

	div.AsNode().AppendChild(text1)
	div.AsNode().AppendChild(text2)
	div.AsNode().AppendChild(text3)

	div.AsNode().Normalize()

	// Should have one text node
	count := 0
	for child := div.AsNode().FirstChild(); child != nil; child = child.NextSibling() {
		count++
	}
	if count != 1 {
		t.Errorf("Expected 1 child after normalize, got %d", count)
	}

	if div.TextContent() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", div.TextContent())
	}
}

func TestParseHTML(t *testing.T) {
	doc, err := ParseHTML(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><p id="para">Hello, World!</p></body>
</html>`)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if doc == nil {
		t.Fatal("ParseHTML returned nil document")
	}

	docEl := doc.DocumentElement()
	if docEl == nil {
		t.Fatal("No document element")
	}
	if docEl.TagName() != "HTML" {
		t.Errorf("Expected HTML element, got %s", docEl.TagName())
	}

	head := doc.Head()
	if head == nil {
		t.Fatal("No head element")
	}

	body := doc.Body()
	if body == nil {
		t.Fatal("No body element")
	}

	title := doc.Title()
	if title != "Test" {
		t.Errorf("Expected title 'Test', got '%s'", title)
	}

	para := doc.GetElementById("para")
	if para == nil {
		t.Fatal("Could not find paragraph")
	}
	if para.TextContent() != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", para.TextContent())
	}
}

func TestNamedNodeMap(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	el.SetAttribute("id", "test")
	el.SetAttribute("class", "container")
	el.SetAttribute("data-value", "123")

	attrs := el.Attributes()
	if attrs.Length() != 3 {
		t.Errorf("Expected 3 attributes, got %d", attrs.Length())
	}

	// Test GetNamedItem
	idAttr := attrs.GetNamedItem("id")
	if idAttr == nil {
		t.Fatal("GetNamedItem returned nil")
	}
	if idAttr.Value() != "test" {
		t.Errorf("Expected value 'test', got '%s'", idAttr.Value())
	}

	// Test Item
	for i := 0; i < attrs.Length(); i++ {
		attr := attrs.Item(i)
		if attr == nil {
			t.Errorf("Item(%d) returned nil", i)
		}
	}

	// Test RemoveNamedItem
	removed := attrs.RemoveNamedItem("class")
	if removed == nil {
		t.Error("RemoveNamedItem returned nil")
	}
	if attrs.Length() != 2 {
		t.Errorf("Expected 2 attributes after removal, got %d", attrs.Length())
	}
	if attrs.GetNamedItem("class") != nil {
		t.Error("class attribute should be removed")
	}
}

func TestElement_ToggleAttribute(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("input")

	// Add attribute
	result := el.ToggleAttribute("disabled")
	if !result {
		t.Error("Expected toggle to return true when adding")
	}
	if !el.HasAttribute("disabled") {
		t.Error("Expected 'disabled' attribute to exist")
	}

	// Remove attribute
	result = el.ToggleAttribute("disabled")
	if result {
		t.Error("Expected toggle to return false when removing")
	}
	if el.HasAttribute("disabled") {
		t.Error("Expected 'disabled' attribute to be removed")
	}

	// Force add
	result = el.ToggleAttribute("readonly", true)
	if !result {
		t.Error("Expected toggle with force=true to return true")
	}

	// Force remove
	result = el.ToggleAttribute("readonly", false)
	if result {
		t.Error("Expected toggle with force=false to return false")
	}
}

func TestElement_Children(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	p := doc.CreateElement("p")
	span := doc.CreateElement("span")
	text := doc.CreateTextNode("text")

	div.AsNode().AppendChild(text)
	div.AsNode().AppendChild(p.AsNode())
	div.AsNode().AppendChild(span.AsNode())

	children := div.Children()
	if children.Length() != 2 {
		t.Errorf("Expected 2 element children, got %d", children.Length())
	}

	if div.ChildElementCount() != 2 {
		t.Errorf("Expected ChildElementCount of 2, got %d", div.ChildElementCount())
	}

	if div.FirstElementChild() != p {
		t.Error("FirstElementChild should be p")
	}

	if div.LastElementChild() != span {
		t.Error("LastElementChild should be span")
	}
}

func TestNodeList(t *testing.T) {
	doc := NewDocument()
	div := doc.CreateElement("div")
	p1 := doc.CreateElement("p")
	p2 := doc.CreateElement("p")

	div.AsNode().AppendChild(p1.AsNode())
	div.AsNode().AppendChild(p2.AsNode())

	childNodes := div.AsNode().ChildNodes()
	if childNodes.Length() != 2 {
		t.Errorf("Expected 2 child nodes, got %d", childNodes.Length())
	}

	if childNodes.Item(0) != p1.AsNode() {
		t.Error("Item(0) should be p1")
	}
	if childNodes.Item(1) != p2.AsNode() {
		t.Error("Item(1) should be p2")
	}
	if childNodes.Item(-1) != nil {
		t.Error("Item(-1) should be nil")
	}
	if childNodes.Item(5) != nil {
		t.Error("Item(5) should be nil")
	}

	// Test that it's live
	p3 := doc.CreateElement("p")
	div.AsNode().AppendChild(p3.AsNode())
	if childNodes.Length() != 3 {
		t.Errorf("Live NodeList should have 3 items, got %d", childNodes.Length())
	}
}

func TestDOMTokenList_Deduplication(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	// Test that duplicate classes are deduplicated
	el.SetAttribute("class", "a a")
	classList := el.ClassList()

	// Per spec, 'a a' should have length 1 (duplicates removed)
	if classList.Length() != 1 {
		t.Errorf("Expected length 1 for 'a a', got %d", classList.Length())
	}

	// Test that Item(0) returns "a"
	if classList.Item(0) != "a" {
		t.Errorf("Expected Item(0) to be 'a', got '%s'", classList.Item(0))
	}

	// Test with more duplicates
	el.SetAttribute("class", "a b a c b a")
	if classList.Length() != 3 {
		t.Errorf("Expected length 3 for 'a b a c b a', got %d", classList.Length())
	}

	// Order should be preserved (first occurrence)
	if classList.Item(0) != "a" {
		t.Errorf("Expected Item(0) to be 'a', got '%s'", classList.Item(0))
	}
	if classList.Item(1) != "b" {
		t.Errorf("Expected Item(1) to be 'b', got '%s'", classList.Item(1))
	}
	if classList.Item(2) != "c" {
		t.Errorf("Expected Item(2) to be 'c', got '%s'", classList.Item(2))
	}
}

func TestNode_HierarchyRequestError(t *testing.T) {
	doc := NewDocument()

	// Test 1: Text node cannot have children
	text := doc.CreateTextNode("hello")
	child := doc.CreateTextNode("world")
	_, err := text.AppendChildWithError(child)
	if err == nil {
		t.Error("Expected HierarchyRequestError when appending to Text node")
	}
	if domErr, ok := err.(*DOMError); ok {
		if domErr.Name != "HierarchyRequestError" {
			t.Errorf("Expected HierarchyRequestError, got %s", domErr.Name)
		}
	}

	// Test 2: Comment node cannot have children
	comment := doc.CreateComment("comment")
	_, err = comment.AppendChildWithError(child)
	if err == nil {
		t.Error("Expected HierarchyRequestError when appending to Comment node")
	}

	// Test 3: Cannot append an ancestor to a descendant
	parent := doc.CreateElement("div")
	childEl := doc.CreateElement("span")
	grandchild := doc.CreateElement("a")
	parent.AsNode().AppendChild(childEl.AsNode())
	childEl.AsNode().AppendChild(grandchild.AsNode())

	_, err = grandchild.AsNode().AppendChildWithError(parent.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when creating circular reference")
	}
	if domErr, ok := err.(*DOMError); ok {
		if domErr.Name != "HierarchyRequestError" {
			t.Errorf("Expected HierarchyRequestError, got %s", domErr.Name)
		}
	}

	// Test 4: Element cannot have Document child
	newDoc := NewDocument()
	_, err = parent.AsNode().AppendChildWithError(newDoc.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when appending Document to Element")
	}

	// Test 5: Document can only have one element child
	doc2 := NewDocument()
	html := doc2.CreateElement("html")
	doc2.AsNode().AppendChild(html.AsNode())

	extraElement := doc2.CreateElement("extra")
	_, err = doc2.AsNode().AppendChildWithError(extraElement.AsNode())
	if err == nil {
		t.Error("Expected HierarchyRequestError when adding second element to Document")
	}

	// Test 6: NotFoundError when refChild is not a child
	parent2 := doc.CreateElement("div")
	notChild := doc.CreateElement("notchild")
	newNode := doc.CreateElement("new")
	_, err = parent2.AsNode().InsertBeforeWithError(newNode.AsNode(), notChild.AsNode())
	if err == nil {
		t.Error("Expected NotFoundError when refChild is not a child")
	}
	if domErr, ok := err.(*DOMError); ok {
		if domErr.Name != "NotFoundError" {
			t.Errorf("Expected NotFoundError, got %s", domErr.Name)
		}
	}
}

func TestDocument_CreateProcessingInstruction(t *testing.T) {
	doc := NewDocument()

	// Test basic creation
	pi := doc.CreateProcessingInstruction("xml-stylesheet", "href='styles.css' type='text/css'")
	if pi == nil {
		t.Fatal("CreateProcessingInstruction returned nil")
	}
	if pi.NodeType() != ProcessingInstructionNode {
		t.Errorf("Expected ProcessingInstructionNode (7), got %v", pi.NodeType())
	}
	if pi.NodeName() != "xml-stylesheet" {
		t.Errorf("Expected nodeName 'xml-stylesheet', got '%s'", pi.NodeName())
	}
	if pi.NodeValue() != "href='styles.css' type='text/css'" {
		t.Errorf("Expected correct nodeValue, got '%s'", pi.NodeValue())
	}

	// Test as ProcessingInstruction type
	piNode := (*ProcessingInstruction)(pi)
	if piNode.Target() != "xml-stylesheet" {
		t.Errorf("Expected target 'xml-stylesheet', got '%s'", piNode.Target())
	}
	if piNode.Data() != "href='styles.css' type='text/css'" {
		t.Errorf("Expected correct data, got '%s'", piNode.Data())
	}

	// Test CharacterData methods
	if piNode.Length() != len("href='styles.css' type='text/css'") {
		t.Errorf("Expected length %d, got %d", len("href='styles.css' type='text/css'"), piNode.Length())
	}

	substr := piNode.SubstringData(0, 4)
	if substr != "href" {
		t.Errorf("Expected 'href', got '%s'", substr)
	}

	// Test SetData
	piNode.SetData("new data")
	if piNode.Data() != "new data" {
		t.Errorf("Expected 'new data', got '%s'", piNode.Data())
	}

	// Verify nodeValue is also updated
	if pi.NodeValue() != "new data" {
		t.Errorf("Expected nodeValue 'new data', got '%s'", pi.NodeValue())
	}
}

func TestDocument_CreateProcessingInstruction_InvalidTarget(t *testing.T) {
	doc := NewDocument()

	// Test invalid target (starts with number)
	pi := doc.CreateProcessingInstruction("123invalid", "data")
	if pi != nil {
		t.Error("Expected nil for invalid target starting with number")
	}

	// Test invalid target (starts with hyphen)
	pi = doc.CreateProcessingInstruction("-invalid", "data")
	if pi != nil {
		t.Error("Expected nil for invalid target starting with hyphen")
	}

	// Test empty target
	pi = doc.CreateProcessingInstruction("", "data")
	if pi != nil {
		t.Error("Expected nil for empty target")
	}
}

func TestDocument_CreateProcessingInstruction_InvalidData(t *testing.T) {
	doc := NewDocument()

	// Test data containing "?>"
	pi := doc.CreateProcessingInstruction("target", "some data ?> more data")
	if pi != nil {
		t.Error("Expected nil for data containing '?>'")
	}
}

func TestDocument_CreateProcessingInstructionWithError(t *testing.T) {
	doc := NewDocument()

	// Test valid creation
	pi, err := doc.CreateProcessingInstructionWithError("valid", "data")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if pi == nil {
		t.Fatal("Expected non-nil PI")
	}

	// Test invalid target
	pi, err = doc.CreateProcessingInstructionWithError("123", "data")
	if err == nil {
		t.Error("Expected error for invalid target")
	}
	if pi != nil {
		t.Error("Expected nil PI for invalid target")
	}

	// Test invalid data
	pi, err = doc.CreateProcessingInstructionWithError("target", "data?>more")
	if err == nil {
		t.Error("Expected error for invalid data")
	}
	if pi != nil {
		t.Error("Expected nil PI for invalid data")
	}
}

func TestProcessingInstruction_CanBeChildOfDocument(t *testing.T) {
	doc := NewDocument()
	pi := doc.CreateProcessingInstruction("xml-stylesheet", "href='test.css'")

	// ProcessingInstruction can be appended to Document
	_, err := doc.AsNode().AppendChildWithError(pi)
	if err != nil {
		t.Errorf("Expected ProcessingInstruction to be valid child of Document, got error: %v", err)
	}

	if doc.AsNode().FirstChild() != pi {
		t.Error("Expected ProcessingInstruction to be first child of Document")
	}
}

func TestProcessingInstruction_CanBeChildOfElement(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	pi := doc.CreateProcessingInstruction("php", "echo 'hello';")

	// ProcessingInstruction can be appended to Element
	_, err := el.AsNode().AppendChildWithError(pi)
	if err != nil {
		t.Errorf("Expected ProcessingInstruction to be valid child of Element, got error: %v", err)
	}

	if el.AsNode().FirstChild() != pi {
		t.Error("Expected ProcessingInstruction to be first child of Element")
	}
}

// TestNode_NormalizeXML tests normalize with XML nodes
func TestNode_NormalizeXML(t *testing.T) {
	// Create an XML document
	doc, err := ParseXML("<div/>")
	if err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	t.Logf("doc type: %d", doc.AsNode().NodeType())
	docChildren := doc.AsNode().ChildNodes()
	t.Logf("doc.childNodes.length: %d", docChildren.Length())
	for i := 0; i < docChildren.Length(); i++ {
		child := docChildren.Item(i)
		t.Logf("  doc child %d: nodeType=%d, nodeName=%s", i, child.NodeType(), child.NodeName())
	}

	div := doc.DocumentElement()
	if div == nil {
		t.Fatal("doc.documentElement is nil")
	}
	t.Logf("documentElement: %s", div.TagName())

	// Create nodes
	t1 := doc.CreateTextNode("a")
	t.Logf("Created t1 (text 'a')")

	t2 := doc.CreateProcessingInstruction("pi", "")
	t.Logf("Created t2 (PI)")

	t3 := doc.CreateTextNode("b")
	t.Logf("Created t3 (text 'b')")

	t4, err := doc.CreateCDATASectionWithError("")
	if err != nil {
		t.Fatalf("Failed to create CDATA: %v", err)
	}
	t.Logf("Created t4 (CDATA)")

	t5 := doc.CreateTextNode("c")
	t.Logf("Created t5 (text 'c')")

	t6 := doc.CreateComment("")
	t.Logf("Created t6 (Comment)")

	t7 := doc.CreateTextNode("d")
	t.Logf("Created t7 (text 'd')")

	t8 := doc.CreateElement("el")
	t.Logf("Created t8 (Element 'el')")

	t9 := doc.CreateTextNode("e")
	t.Logf("Created t9 (text 'e')")

	// Append nodes
	div.AsNode().AppendChild(t1)
	div.AsNode().AppendChild(t2)
	div.AsNode().AppendChild(t3)
	div.AsNode().AppendChild(t4)
	div.AsNode().AppendChild(t5)
	div.AsNode().AppendChild(t6)
	div.AsNode().AppendChild(t7)
	div.AsNode().AppendChild(t8.AsNode())
	div.AsNode().AppendChild(t9)

	t.Log("\nBefore normalize:")
	children := div.AsNode().ChildNodes()
	t.Logf("div.childNodes.length: %d", children.Length())
	for i := 0; i < children.Length(); i++ {
		child := children.Item(i)
		t.Logf("  child %d: nodeType=%d, nodeName=%s, nodeValue=%q", i, child.NodeType(), child.NodeName(), child.NodeValue())
	}

	// Normalize
	div.AsNode().Normalize()

	t.Log("\nAfter normalize:")
	children = div.AsNode().ChildNodes()
	t.Logf("div.childNodes.length: %d", children.Length())
	for i := 0; i < children.Length(); i++ {
		child := children.Item(i)
		t.Logf("  child %d: nodeType=%d, nodeName=%s, nodeValue=%q", i, child.NodeType(), child.NodeName(), child.NodeValue())
	}

	// Expect 9 children (no merging since they're separated by non-text nodes)
	if children.Length() != 9 {
		t.Errorf("Expected 9 children, got %d", children.Length())
	}
}

func TestSerializeToXML(t *testing.T) {
	// Test simple element
	doc := NewDocument()
	div := doc.CreateElement("div")
	div.SetTextContent("Hello")

	result, err := SerializeToXML(div.AsNode())
	if err != nil {
		t.Fatal(err)
	}
	// Div is in HTML namespace, so it gets xmlns attribute
	expected := `<div xmlns="http://www.w3.org/1999/xhtml">Hello</div>`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Test nested elements
	parent := doc.CreateElement("div")
	child := doc.CreateElement("span")
	child.SetTextContent("nested")
	parent.AsNode().AppendChild(child.AsNode())

	result, err = SerializeToXML(parent.AsNode())
	if err != nil {
		t.Fatal(err)
	}
	// Child in same namespace should not repeat xmlns
	expected = `<div xmlns="http://www.w3.org/1999/xhtml"><span>nested</span></div>`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Test element with attributes
	el := doc.CreateElement("div")
	el.SetAttribute("id", "test")
	el.SetAttribute("class", "foo bar")

	result, err = SerializeToXML(el.AsNode())
	if err != nil {
		t.Fatal(err)
	}
	// Should contain the attributes (order may vary)
	if !contains(result, `id="test"`) || !contains(result, `class="foo bar"`) {
		t.Errorf("Expected attributes in result, got %q", result)
	}

	// Test comment node
	comment := doc.CreateComment("This is a comment")
	result, err = SerializeToXML(comment)
	if err != nil {
		t.Fatal(err)
	}
	expected = "<!--This is a comment-->"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Test invalid comment (contains --)
	badComment := doc.CreateComment("bad--comment")
	_, err = SerializeToXML(badComment)
	if err == nil {
		t.Error("Expected error for comment containing '--'")
	}

	// Test text with special characters
	textNode := doc.CreateTextNode("<script>&amp;</script>")
	result, err = SerializeToXML(textNode)
	if err != nil {
		t.Fatal(err)
	}
	expected = "&lt;script&gt;&amp;amp;&lt;/script&gt;"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Test document serialization
	htmlDoc, _ := ParseHTML("<html><body><div>Test</div></body></html>")
	result, err = SerializeToXML(htmlDoc.AsNode())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "<html") || !contains(result, "<body") || !contains(result, "<div>Test</div>") {
		t.Errorf("Expected full HTML document, got %q", result)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
