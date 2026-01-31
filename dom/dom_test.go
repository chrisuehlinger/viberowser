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

	classList.Add("foo", "bar", "baz")
	if !classList.Contains("foo") {
		t.Error("Expected classList to contain 'foo'")
	}
	if !classList.Contains("bar") {
		t.Error("Expected classList to contain 'bar'")
	}
	if classList.Length() != 3 {
		t.Errorf("Expected 3 classes, got %d", classList.Length())
	}

	classList.Remove("bar")
	if classList.Contains("bar") {
		t.Error("Expected classList not to contain 'bar' after removal")
	}

	result := classList.Toggle("qux")
	if !result {
		t.Error("Expected toggle to return true when adding")
	}
	if !classList.Contains("qux") {
		t.Error("Expected classList to contain 'qux'")
	}

	result = classList.Toggle("qux")
	if result {
		t.Error("Expected toggle to return false when removing")
	}
	if classList.Contains("qux") {
		t.Error("Expected classList not to contain 'qux' after toggle")
	}

	// Test Replace
	classList.Add("old")
	if !classList.Replace("old", "new") {
		t.Error("Expected Replace to return true")
	}
	if classList.Contains("old") {
		t.Error("Expected classList not to contain 'old' after replace")
	}
	if !classList.Contains("new") {
		t.Error("Expected classList to contain 'new' after replace")
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
