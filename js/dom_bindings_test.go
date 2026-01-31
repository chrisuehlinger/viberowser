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

func TestDOMBinderParentNodeProperties(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="parent">
		<span id="first">First</span>
		<p id="middle">Middle</p>
		<span id="last">Last</span>
	</div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test children
	result, err := r.Execute("document.getElementById('parent').children.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected 3 children, got %v", result.ToInteger())
	}

	// Test childElementCount
	result, err = r.Execute("document.getElementById('parent').childElementCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected childElementCount of 3, got %v", result.ToInteger())
	}

	// Test firstElementChild
	result, err = r.Execute("document.getElementById('parent').firstElementChild.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "first" {
		t.Errorf("Expected firstElementChild id 'first', got %v", result.String())
	}

	// Test lastElementChild
	result, err = r.Execute("document.getElementById('parent').lastElementChild.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "last" {
		t.Errorf("Expected lastElementChild id 'last', got %v", result.String())
	}

	// Test previousElementSibling
	result, err = r.Execute("document.getElementById('middle').previousElementSibling.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "first" {
		t.Errorf("Expected previousElementSibling id 'first', got %v", result.String())
	}

	// Test nextElementSibling
	result, err = r.Execute("document.getElementById('middle').nextElementSibling.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "last" {
		t.Errorf("Expected nextElementSibling id 'last', got %v", result.String())
	}

	// Test document.children (should have html element)
	result, err = r.Execute("document.children.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected document to have 1 child element, got %v", result.ToInteger())
	}

	// Test document.firstElementChild
	result, err = r.Execute("document.firstElementChild.tagName")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "HTML" {
		t.Errorf("Expected document.firstElementChild to be HTML, got %v", result.String())
	}
}

func TestDOMBinderParentNodeMixinMethods(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="parent"><span>Initial</span></div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test element.append()
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var newSpan = document.createElement('span');
		newSpan.textContent = 'Appended';
		parent.append(newSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('parent').lastElementChild.textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Appended" {
		t.Errorf("Expected 'Appended', got %v", result.String())
	}

	// Test element.append() with string
	_, err = r.Execute(`
		var parent = document.getElementById('parent');
		parent.append(' text');
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('parent').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "InitialAppended text" {
		t.Errorf("Expected 'InitialAppended text', got %v", result.String())
	}

	// Test element.prepend()
	_, err = r.Execute(`
		var parent = document.getElementById('parent');
		var firstSpan = document.createElement('span');
		firstSpan.textContent = 'First';
		parent.prepend(firstSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('parent').firstElementChild.textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "First" {
		t.Errorf("Expected 'First', got %v", result.String())
	}

	// Test element.replaceChildren()
	_, err = r.Execute(`
		var parent = document.getElementById('parent');
		var newP = document.createElement('p');
		newP.textContent = 'Replaced';
		parent.replaceChildren(newP);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('parent').childElementCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 child after replaceChildren, got %v", result.ToInteger())
	}

	result, err = r.Execute("document.getElementById('parent').firstElementChild.tagName")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "P" {
		t.Errorf("Expected 'P', got %v", result.String())
	}
}

func TestDOMBinderChildNodeMixinMethods(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="parent">
		<span id="first">First</span>
		<span id="middle">Middle</span>
		<span id="last">Last</span>
	</div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Test element.before()
	_, err := r.Execute(`
		var middle = document.getElementById('middle');
		var newSpan = document.createElement('span');
		newSpan.id = 'before-middle';
		newSpan.textContent = 'Before Middle';
		middle.before(newSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('middle').previousElementSibling.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "before-middle" {
		t.Errorf("Expected 'before-middle', got %v", result.String())
	}

	// Test element.after()
	_, err = r.Execute(`
		var middle = document.getElementById('middle');
		var newSpan = document.createElement('span');
		newSpan.id = 'after-middle';
		newSpan.textContent = 'After Middle';
		middle.after(newSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('middle').nextElementSibling.id")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "after-middle" {
		t.Errorf("Expected 'after-middle', got %v", result.String())
	}

	// Test element.replaceWith()
	_, err = r.Execute(`
		var afterMiddle = document.getElementById('after-middle');
		var replacement = document.createElement('p');
		replacement.id = 'replacement';
		replacement.textContent = 'Replacement';
		afterMiddle.replaceWith(replacement);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('replacement') !== null")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected replacement element to exist")
	}

	result, err = r.Execute("document.getElementById('after-middle')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "null" {
		t.Error("Expected after-middle to be removed after replaceWith")
	}

	// Test element.remove()
	_, err = r.Execute(`
		document.getElementById('replacement').remove();
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('replacement')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "null" {
		t.Error("Expected replacement to be removed after remove()")
	}
}

func TestDOMBinderTextNodeChildNodeMixin(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="parent"><span id="span1">First</span></div>
</body>
</html>`)

	binder.BindDocument(doc)

	// Create a text node and test ChildNode methods
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var textNode = document.createTextNode('Hello');
		parent.appendChild(textNode);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test text node before()
	_, err = r.Execute(`
		var textNode = document.getElementById('parent').lastChild;
		var newSpan = document.createElement('span');
		newSpan.id = 'before-text';
		textNode.before(newSpan);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('before-text') !== null")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected before-text element to exist")
	}

	// Test text node remove()
	_, err = r.Execute(`
		var parent = document.getElementById('parent');
		var textNode = parent.lastChild;
		textNode.remove();
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify text node was removed
	result, err = r.Execute("document.getElementById('parent').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "First" {
		t.Errorf("Expected 'First' after text node removal, got %v", result.String())
	}
}

func TestDOMBinderDocumentFragmentMixinMethods(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test DocumentFragment ParentNode mixin
	_, err := r.Execute(`
		var frag = document.createDocumentFragment();
		var p1 = document.createElement('p');
		p1.textContent = 'First';
		var p2 = document.createElement('p');
		p2.textContent = 'Second';
		frag.append(p1, p2);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("frag.childElementCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2 children in fragment, got %v", result.ToInteger())
	}

	// Test fragment.prepend()
	_, err = r.Execute(`
		var p0 = document.createElement('p');
		p0.textContent = 'Zero';
		frag.prepend(p0);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("frag.firstElementChild.textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Zero" {
		t.Errorf("Expected 'Zero', got %v", result.String())
	}

	// Test fragment.replaceChildren()
	_, err = r.Execute(`
		var newP = document.createElement('p');
		newP.textContent = 'Only';
		frag.replaceChildren(newP);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("frag.childElementCount")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected 1 child after replaceChildren, got %v", result.ToInteger())
	}
}

func TestDOMImplementation(t *testing.T) {
	doc, _ := dom.ParseHTML("<html><body></body></html>")
	runtime := NewRuntime()
	binder := NewDOMBinder(runtime)
	binder.BindDocument(doc)

	// Test that DOMImplementation exists
	result, err := runtime.Execute(`typeof DOMImplementation`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "function" {
		t.Errorf("DOMImplementation should be a function, got %s", result.String())
	}

	// Test that document.implementation exists
	result, err = runtime.Execute(`typeof document.implementation`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "object" {
		t.Errorf("document.implementation should be an object, got %s", result.String())
	}

	// Test instanceof
	result, err = runtime.Execute(`document.implementation instanceof DOMImplementation`)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToBoolean() {
		t.Error("document.implementation should be instanceof DOMImplementation")
	}

	// Test createHTMLDocument
	result, err = runtime.Execute(`
		var doc = document.implementation.createHTMLDocument("Test");
		doc.title
	`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "Test" {
		t.Errorf("Expected title 'Test', got %s", result.String())
	}

	// Test createHTMLDocument returns different implementation
	result, err = runtime.Execute(`
		var doc = document.implementation.createHTMLDocument("Test");
		document.implementation !== doc.implementation
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToBoolean() {
		t.Error("Different documents should have different implementations")
	}
}

func TestDOMTokenList_IndexAccess(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" class="foo bar baz">Test</div></body>
</html>`)

	binder.BindDocument(doc)

	// Test numeric index access
	result, err := r.Execute("document.getElementById('test').classList[0]")
	if err != nil {
		t.Fatalf("Error accessing classList[0]: %v", err)
	}
	if result.String() != "foo" {
		t.Errorf("Expected classList[0] to be 'foo', got '%s'", result.String())
	}

	result, err = r.Execute("document.getElementById('test').classList[1]")
	if err != nil {
		t.Fatalf("Error accessing classList[1]: %v", err)
	}
	if result.String() != "bar" {
		t.Errorf("Expected classList[1] to be 'bar', got '%s'", result.String())
	}

	result, err = r.Execute("document.getElementById('test').classList[2]")
	if err != nil {
		t.Fatalf("Error accessing classList[2]: %v", err)
	}
	if result.String() != "baz" {
		t.Errorf("Expected classList[2] to be 'baz', got '%s'", result.String())
	}

	// Test out of bounds - should return undefined
	result, err = r.Execute("document.getElementById('test').classList[3]")
	if err != nil {
		t.Fatalf("Error accessing classList[3]: %v", err)
	}
	if result.String() != "undefined" {
		t.Errorf("Expected classList[3] to be undefined, got '%s'", result.String())
	}

	// Test that item() still works
	result, err = r.Execute("document.getElementById('test').classList.item(0)")
	if err != nil {
		t.Fatalf("Error calling classList.item(0): %v", err)
	}
	if result.String() != "foo" {
		t.Errorf("Expected classList.item(0) to be 'foo', got '%s'", result.String())
	}

	// Test item() out of bounds - should return null
	result, err = r.Execute("document.getElementById('test').classList.item(99)")
	if err != nil {
		t.Fatalf("Error calling classList.item(99): %v", err)
	}
	if result.String() != "null" {
		t.Errorf("Expected classList.item(99) to be null, got '%s'", result.String())
	}

	// Test length
	result, err = r.Execute("document.getElementById('test').classList.length")
	if err != nil {
		t.Fatalf("Error accessing classList.length: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected classList.length to be 3, got %d", result.ToInteger())
	}
}

func TestDOMTokenList_Deduplication(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" class="a a">Test</div></body>
</html>`)

	binder.BindDocument(doc)

	// Test that duplicate classes are deduplicated
	result, err := r.Execute("document.getElementById('test').classList.length")
	if err != nil {
		t.Fatalf("Error accessing classList.length: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected classList.length to be 1 for 'a a', got %d", result.ToInteger())
	}

	// Test that index access works correctly after deduplication
	result, err = r.Execute("document.getElementById('test').classList[0]")
	if err != nil {
		t.Fatalf("Error accessing classList[0]: %v", err)
	}
	if result.String() != "a" {
		t.Errorf("Expected classList[0] to be 'a', got '%s'", result.String())
	}
}

func TestInsertBeforeLength(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML("<!DOCTYPE html><html><body></body></html>")
	binder.BindDocument(doc)

	// Test that insertBefore.length is 2
	result, err := r.Execute("document.body.insertBefore.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	t.Logf("insertBefore.length = %v", result.String())
	if result.ToInteger() != 2 {
		t.Errorf("Expected insertBefore.length = 2, got %v", result.String())
	}
}
