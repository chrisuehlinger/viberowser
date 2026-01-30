package js

import (
	"testing"

	"github.com/AYColumbia/viberowser/dom"
)

func TestDOMBinderDocument(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><div id="test">Hello</div></body>
</html>`)

	binder.BindDocument(doc)

	// Test document is bound
	result, err := r.Execute("typeof document")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "object" {
		t.Errorf("Expected 'object', got %v", result.String())
	}

	// Test getElementById
	result, err = r.Execute("document.getElementById('test').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Hello" {
		t.Errorf("Expected 'Hello', got %v", result.String())
	}
}

func TestDOMBinderQuerySelector(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div class="container">
		<p id="first">First</p>
		<p class="highlight">Second</p>
	</div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test querySelector by ID
	result, err := r.Execute("document.querySelector('#first').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "First" {
		t.Errorf("Expected 'First', got %v", result.String())
	}

	// Test querySelector by class
	result, err = r.Execute("document.querySelector('.highlight').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Second" {
		t.Errorf("Expected 'Second', got %v", result.String())
	}

	// Test querySelectorAll
	result, err = r.Execute("document.querySelectorAll('p').length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2, got %v", result.ToInteger())
	}
}

func TestDOMBinderCreateElement(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test createElement
	_, err := r.Execute(`
		var div = document.createElement('div');
		div.id = 'created';
		div.textContent = 'Created!';
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("div.tagName")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "DIV" {
		t.Errorf("Expected 'DIV', got %v", result.String())
	}

	result, err = r.Execute("div.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "created" {
		t.Errorf("Expected 'created', got %v", result.String())
	}
}

func TestDOMBinderAttributes(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" data-value="123" class="foo bar"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test getAttribute
	result, err := r.Execute("document.getElementById('test').getAttribute('data-value')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "123" {
		t.Errorf("Expected '123', got %v", result.String())
	}

	// Test setAttribute
	_, err = r.Execute("document.getElementById('test').setAttribute('data-value', '456')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').getAttribute('data-value')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "456" {
		t.Errorf("Expected '456', got %v", result.String())
	}

	// Test hasAttribute
	result, err = r.Execute("document.getElementById('test').hasAttribute('data-value')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected hasAttribute to return true")
	}

	// Test removeAttribute
	_, err = r.Execute("document.getElementById('test').removeAttribute('data-value')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').hasAttribute('data-value')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected hasAttribute to return false after removal")
	}
}

func TestDOMBinderClassList(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" class="foo bar"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test classList.contains
	result, err := r.Execute("document.getElementById('test').classList.contains('foo')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected classList.contains('foo') to be true")
	}

	// Test classList.add
	_, err = r.Execute("document.getElementById('test').classList.add('baz')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').classList.contains('baz')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected classList.contains('baz') to be true after add")
	}

	// Test classList.remove
	_, err = r.Execute("document.getElementById('test').classList.remove('foo')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').classList.contains('foo')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Expected classList.contains('foo') to be false after remove")
	}

	// Test classList.toggle
	_, err = r.Execute("document.getElementById('test').classList.toggle('toggled')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').classList.contains('toggled')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected classList.contains('toggled') to be true after toggle")
	}
}

func TestDOMBinderChildNodes(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="parent">
		<span>One</span>
		<span>Two</span>
		<span>Three</span>
	</div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test appendChild
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var newSpan = document.createElement('span');
		newSpan.textContent = 'Four';
		parent.appendChild(newSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('parent').getElementsByTagName('span').length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 4 {
		t.Errorf("Expected 4, got %v", result.ToInteger())
	}

	// Test removeChild
	_, err = r.Execute(`
		var parent = document.getElementById('parent');
		var spans = parent.getElementsByTagName('span');
		parent.removeChild(spans.item(0));
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('parent').getElementsByTagName('span').length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected 3, got %v", result.ToInteger())
	}
}

func TestDOMBinderInnerHTML(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test setting innerHTML
	_, err := r.Execute(`
		var div = document.getElementById('test');
		div.innerHTML = '<p>New content</p><span>More</span>';
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('test').innerHTML")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "<p>New content</p><span>More</span>" {
		t.Errorf("Expected '<p>New content</p><span>More</span>', got %v", result.String())
	}

	// Test getting children after innerHTML
	result, err = r.Execute("document.getElementById('test').childNodes.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2, got %v", result.ToInteger())
	}
}

func TestDOMBinderTextContent(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"><span>Hello</span> <span>World</span></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test getting textContent
	result, err := r.Execute("document.getElementById('test').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%v'", result.String())
	}

	// Test setting textContent
	_, err = r.Execute("document.getElementById('test').textContent = 'New text'")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('test').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "New text" {
		t.Errorf("Expected 'New text', got '%v'", result.String())
	}

	// Verify children were replaced with text node
	result, err = r.Execute("document.getElementById('test').childNodes.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 child node, got %v", result.ToInteger())
	}
}

func TestDOMBinderNodeList(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<p>One</p>
	<p>Two</p>
	<p>Three</p>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test NodeList length
	result, err := r.Execute("document.querySelectorAll('p').length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected 3, got %v", result.ToInteger())
	}

	// Test NodeList item()
	result, err = r.Execute("document.querySelectorAll('p').item(1).textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Two" {
		t.Errorf("Expected 'Two', got %v", result.String())
	}

	// Test NodeList forEach
	_, err = r.Execute(`
		var texts = [];
		document.querySelectorAll('p').forEach(function(p) {
			texts.push(p.textContent);
		});
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("texts.join(',')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "One,Two,Three" {
		t.Errorf("Expected 'One,Two,Three', got %v", result.String())
	}
}

func TestDOMBinderDocumentFragment(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	_, err := r.Execute(`
		var frag = document.createDocumentFragment();
		var p1 = document.createElement('p');
		p1.textContent = 'First';
		var p2 = document.createElement('p');
		p2.textContent = 'Second';
		frag.appendChild(p1);
		frag.appendChild(p2);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test fragment has children
	result, err := r.Execute("frag.childNodes.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2, got %v", result.ToInteger())
	}
}

func TestDOMBinderCloneNode(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"><span>Child</span></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test shallow clone
	_, err := r.Execute(`
		var original = document.getElementById('test');
		var shallow = original.cloneNode(false);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("shallow.childNodes.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 0 {
		t.Errorf("Expected shallow clone to have 0 children, got %v", result.ToInteger())
	}

	// Test deep clone
	_, err = r.Execute(`
		var deep = original.cloneNode(true);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("deep.childNodes.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected deep clone to have 1 child, got %v", result.ToInteger())
	}
}
