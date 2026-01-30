package html

import (
	"io"
	"strings"
	"testing"

	"golang.org/x/net/html/atom"
)

func TestParse_BasicDocument(t *testing.T) {
	input := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><p>Hello, World!</p></body>
</html>`

	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if doc.Type != DocumentNode {
		t.Errorf("Expected DocumentNode, got %v", doc.Type)
	}

	// Find the html element
	var htmlNode *Node
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == ElementNode && c.Data == "html" {
			htmlNode = c
			break
		}
	}
	if htmlNode == nil {
		t.Fatal("Could not find html element")
	}

	// Check that html has head and body children
	var hasHead, hasBody bool
	for c := htmlNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == ElementNode {
			if c.Data == "head" {
				hasHead = true
			} else if c.Data == "body" {
				hasBody = true
			}
		}
	}
	if !hasHead {
		t.Error("Missing head element")
	}
	if !hasBody {
		t.Error("Missing body element")
	}
}

func TestParse_MalformedHTML(t *testing.T) {
	// HTML5 parser should handle malformed HTML gracefully
	input := `<p>unclosed paragraph<div>nested div</p></div>`

	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should parse without error, the parser will fix up the structure
	if doc == nil {
		t.Fatal("Expected non-nil document")
	}
}

func TestParse_Attributes(t *testing.T) {
	input := `<div id="main" class="container" data-value="123">content</div>`

	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the div element
	var divNode *Node
	var findDiv func(*Node)
	findDiv = func(n *Node) {
		if n.Type == ElementNode && n.Data == "div" {
			divNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDiv(c)
			if divNode != nil {
				return
			}
		}
	}
	findDiv(doc)

	if divNode == nil {
		t.Fatal("Could not find div element")
	}

	if divNode.GetAttribute("id") != "main" {
		t.Errorf("Expected id='main', got '%s'", divNode.GetAttribute("id"))
	}
	if divNode.GetAttribute("class") != "container" {
		t.Errorf("Expected class='container', got '%s'", divNode.GetAttribute("class"))
	}
	if divNode.GetAttribute("data-value") != "123" {
		t.Errorf("Expected data-value='123', got '%s'", divNode.GetAttribute("data-value"))
	}
}

func TestNode_AppendChild(t *testing.T) {
	parent := &Node{Type: ElementNode, Data: "div"}
	child1 := &Node{Type: ElementNode, Data: "p"}
	child2 := &Node{Type: ElementNode, Data: "span"}

	parent.AppendChild(child1)
	parent.AppendChild(child2)

	if parent.FirstChild != child1 {
		t.Error("FirstChild should be child1")
	}
	if parent.LastChild != child2 {
		t.Error("LastChild should be child2")
	}
	if child1.NextSibling != child2 {
		t.Error("child1.NextSibling should be child2")
	}
	if child2.PrevSibling != child1 {
		t.Error("child2.PrevSibling should be child1")
	}
	if child1.Parent != parent {
		t.Error("child1.Parent should be parent")
	}
}

func TestNode_RemoveChild(t *testing.T) {
	parent := &Node{Type: ElementNode, Data: "div"}
	child1 := &Node{Type: ElementNode, Data: "p"}
	child2 := &Node{Type: ElementNode, Data: "span"}
	child3 := &Node{Type: ElementNode, Data: "a"}

	parent.AppendChild(child1)
	parent.AppendChild(child2)
	parent.AppendChild(child3)

	// Remove middle child
	parent.RemoveChild(child2)

	if child1.NextSibling != child3 {
		t.Error("child1.NextSibling should be child3 after removing child2")
	}
	if child3.PrevSibling != child1 {
		t.Error("child3.PrevSibling should be child1 after removing child2")
	}
	if child2.Parent != nil {
		t.Error("child2.Parent should be nil after removal")
	}
}

func TestNode_InsertBefore(t *testing.T) {
	parent := &Node{Type: ElementNode, Data: "div"}
	child1 := &Node{Type: ElementNode, Data: "p"}
	child3 := &Node{Type: ElementNode, Data: "a"}
	child2 := &Node{Type: ElementNode, Data: "span"}

	parent.AppendChild(child1)
	parent.AppendChild(child3)
	parent.InsertBefore(child2, child3)

	if child1.NextSibling != child2 {
		t.Error("child1.NextSibling should be child2")
	}
	if child2.NextSibling != child3 {
		t.Error("child2.NextSibling should be child3")
	}
	if child2.PrevSibling != child1 {
		t.Error("child2.PrevSibling should be child1")
	}
}

func TestNode_TextContent(t *testing.T) {
	doc, err := Parse(`<div>Hello <span>World</span>!</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the div element
	var divNode *Node
	var findDiv func(*Node)
	findDiv = func(n *Node) {
		if n.Type == ElementNode && n.Data == "div" {
			divNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDiv(c)
			if divNode != nil {
				return
			}
		}
	}
	findDiv(doc)

	if divNode == nil {
		t.Fatal("Could not find div element")
	}

	textContent := divNode.TextContent()
	if textContent != "Hello World!" {
		t.Errorf("Expected 'Hello World!', got '%s'", textContent)
	}
}

func TestNode_SetAttribute(t *testing.T) {
	node := &Node{Type: ElementNode, Data: "div"}

	node.SetAttribute("id", "test")
	if node.GetAttribute("id") != "test" {
		t.Errorf("Expected id='test', got '%s'", node.GetAttribute("id"))
	}

	// Update existing attribute
	node.SetAttribute("id", "updated")
	if node.GetAttribute("id") != "updated" {
		t.Errorf("Expected id='updated', got '%s'", node.GetAttribute("id"))
	}
}

func TestNode_HasAttribute(t *testing.T) {
	node := &Node{Type: ElementNode, Data: "div"}
	node.SetAttribute("id", "test")

	if !node.HasAttribute("id") {
		t.Error("Expected HasAttribute('id') to be true")
	}
	if node.HasAttribute("class") {
		t.Error("Expected HasAttribute('class') to be false")
	}
}

func TestNode_RemoveAttribute(t *testing.T) {
	node := &Node{Type: ElementNode, Data: "div"}
	node.SetAttribute("id", "test")
	node.SetAttribute("class", "container")

	node.RemoveAttribute("id")

	if node.HasAttribute("id") {
		t.Error("Expected HasAttribute('id') to be false after removal")
	}
	if !node.HasAttribute("class") {
		t.Error("Expected HasAttribute('class') to still be true")
	}
}

func TestNode_Children(t *testing.T) {
	parent := &Node{Type: ElementNode, Data: "div"}
	child1 := &Node{Type: ElementNode, Data: "p"}
	child2 := &Node{Type: ElementNode, Data: "span"}
	child3 := &Node{Type: ElementNode, Data: "a"}

	parent.AppendChild(child1)
	parent.AppendChild(child2)
	parent.AppendChild(child3)

	children := parent.Children()
	if len(children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(children))
	}
	if children[0] != child1 || children[1] != child2 || children[2] != child3 {
		t.Error("Children slice has wrong order")
	}
}

func TestParseFragment(t *testing.T) {
	// Parse a fragment in the context of a div element
	context := &Node{Type: ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := ParseFragment(`<p>Paragraph 1</p><p>Paragraph 2</p>`, context)
	if err != nil {
		t.Fatalf("ParseFragment failed: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	for i, n := range nodes {
		if n.Type != ElementNode || n.Data != "p" {
			t.Errorf("Node %d: expected p element, got %s", i, n.Data)
		}
	}
}

func TestTokenizer_BasicUsage(t *testing.T) {
	input := `<div id="test">Hello <span>World</span></div>`
	tokenizer := NewTokenizerString(input)

	var tokens []Token
	for {
		tt := tokenizer.Next()
		if tt == ErrorToken {
			err := tokenizer.Err()
			if err != io.EOF {
				t.Fatalf("Tokenizer error: %v", err)
			}
			break
		}
		tokens = append(tokens, tokenizer.Token())
	}

	// Expected sequence: StartTag(div), Text, StartTag(span), Text, EndTag(span), EndTag(div)
	expectedTypes := []TokenType{
		StartTagToken,     // <div>
		TextToken,         // Hello
		StartTagToken,     // <span>
		TextToken,         // World
		EndTagToken,       // </span>
		EndTagToken,       // </div>
	}

	if len(tokens) != len(expectedTypes) {
		t.Errorf("Expected %d tokens, got %d", len(expectedTypes), len(tokens))
		for i, tok := range tokens {
			t.Logf("Token %d: type=%d, data=%q", i, tok.Type, tok.Data)
		}
		return
	}

	for i, expected := range expectedTypes {
		if tokens[i].Type != expected {
			t.Errorf("Token %d: expected type %d, got %d (data=%q)", i, expected, tokens[i].Type, tokens[i].Data)
		}
	}

	// Check first token attributes
	if tokens[0].Data != "div" {
		t.Errorf("First token should be div, got %s", tokens[0].Data)
	}
	if len(tokens[0].Attributes) != 1 || tokens[0].Attributes[0].Key != "id" || tokens[0].Attributes[0].Value != "test" {
		t.Error("First token should have id='test' attribute")
	}
}

func TestTokenizer_Comments(t *testing.T) {
	input := `<!-- This is a comment --><div>content</div>`
	tokenizer := NewTokenizerString(input)

	tt := tokenizer.Next()
	if tt != CommentToken {
		t.Errorf("Expected CommentToken, got %v", tt)
	}

	tok := tokenizer.Token()
	expected := " This is a comment "
	if tok.Data != expected {
		t.Errorf("Expected comment data %q, got %q", expected, tok.Data)
	}
}

func TestTokenizer_Doctype(t *testing.T) {
	input := `<!DOCTYPE html><html></html>`
	tokenizer := NewTokenizerString(input)

	tt := tokenizer.Next()
	if tt != DoctypeToken {
		t.Errorf("Expected DoctypeToken, got %v", tt)
	}

	tok := tokenizer.Token()
	if tok.Data != "html" {
		t.Errorf("Expected doctype 'html', got %q", tok.Data)
	}
}

func TestParse_EntityDecoding(t *testing.T) {
	input := `<p>&lt;script&gt;alert('XSS')&lt;/script&gt;</p>`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the p element and check its text content
	var pNode *Node
	var findP func(*Node)
	findP = func(n *Node) {
		if n.Type == ElementNode && n.Data == "p" {
			pNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findP(c)
			if pNode != nil {
				return
			}
		}
	}
	findP(doc)

	if pNode == nil {
		t.Fatal("Could not find p element")
	}

	expected := "<script>alert('XSS')</script>"
	if pNode.TextContent() != expected {
		t.Errorf("Expected text content %q, got %q", expected, pNode.TextContent())
	}
}

func TestParse_SelfClosingTags(t *testing.T) {
	input := `<div><br/><img src="test.png"/><input type="text"/></div>`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the div and count its element children
	var divNode *Node
	var findDiv func(*Node)
	findDiv = func(n *Node) {
		if n.Type == ElementNode && n.Data == "div" {
			divNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDiv(c)
			if divNode != nil {
				return
			}
		}
	}
	findDiv(doc)

	if divNode == nil {
		t.Fatal("Could not find div element")
	}

	var elementCount int
	for c := divNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == ElementNode {
			elementCount++
		}
	}

	if elementCount != 3 {
		t.Errorf("Expected 3 element children (br, img, input), got %d", elementCount)
	}
}

func TestParse_NestedElements(t *testing.T) {
	input := `<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find the ul and count li children
	var ulNode *Node
	var findUL func(*Node)
	findUL = func(n *Node) {
		if n.Type == ElementNode && n.Data == "ul" {
			ulNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findUL(c)
			if ulNode != nil {
				return
			}
		}
	}
	findUL(doc)

	if ulNode == nil {
		t.Fatal("Could not find ul element")
	}

	var liCount int
	for c := ulNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == ElementNode && c.Data == "li" {
			liCount++
		}
	}

	if liCount != 3 {
		t.Errorf("Expected 3 li elements, got %d", liCount)
	}
}

func TestParse_TableStructure(t *testing.T) {
	input := `<table><tr><td>Cell 1</td><td>Cell 2</td></tr></table>`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// The HTML5 parser should add tbody automatically
	var tableNode *Node
	var findTable func(*Node)
	findTable = func(n *Node) {
		if n.Type == ElementNode && n.Data == "table" {
			tableNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTable(c)
			if tableNode != nil {
				return
			}
		}
	}
	findTable(doc)

	if tableNode == nil {
		t.Fatal("Could not find table element")
	}

	// HTML5 parser adds tbody automatically
	var hasTbody bool
	for c := tableNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == ElementNode && c.Data == "tbody" {
			hasTbody = true
			break
		}
	}

	if !hasTbody {
		t.Error("Expected HTML5 parser to add tbody element")
	}
}

func TestParseReader(t *testing.T) {
	input := strings.NewReader(`<html><body><p>Test</p></body></html>`)
	doc, err := ParseReader(input)
	if err != nil {
		t.Fatalf("ParseReader failed: %v", err)
	}

	if doc == nil || doc.Type != DocumentNode {
		t.Error("Expected valid DocumentNode")
	}
}
