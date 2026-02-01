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

func TestDOMBinderCreateElementInvalidName(t *testing.T) {
	// Test that createElement throws InvalidCharacterError for invalid names
	testCases := []struct {
		name      string
		wantError bool
	}{
		{"", true},         // empty
		{"1foo", true},     // starts with digit
		{"fo o", true},     // contains space
		{"<foo", true},     // starts with <
		{"foo>", true},     // contains >
		{"-foo", true},     // starts with -
		{".foo", true},     // starts with .
		{"foo", false},     // valid
		{"div", false},     // valid
		{"my-element", false}, // valid with hyphen
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRuntime()
			binder := NewDOMBinder(r)
			doc := dom.NewDocument()
			binder.BindDocument(doc)

			script := `
				(function() {
					try {
						document.createElement(` + "`" + tc.name + "`" + `);
						return "success";
					} catch (e) {
						return e.name;
					}
				})()
			`
			result, err := r.Execute(script)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			resultStr := result.String()
			if tc.wantError {
				if resultStr != "InvalidCharacterError" {
					t.Errorf("Expected InvalidCharacterError, got %v", resultStr)
				}
			} else {
				if resultStr != "success" {
					t.Errorf("Expected success, got %v", resultStr)
				}
			}
		})
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

func TestDocumentFragmentConstructorOwnerDocument(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test that new DocumentFragment() has ownerDocument set to the current document
	result, err := r.Execute(`
		var df = new DocumentFragment();
		df.ownerDocument === document;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected new DocumentFragment().ownerDocument to equal document")
	}

	// Test that ownerDocument is a Document node with children (html and head/body)
	result, err = r.Execute(`
		var df2 = new DocumentFragment();
		df2.ownerDocument !== null;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected ownerDocument to not be null")
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

func TestDocumentFragmentGetElementById(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test getElementById on DocumentFragment
	_, err := r.Execute(`
		var frag = document.createDocumentFragment();
		var div = document.createElement('div');
		div.id = 'myDiv';
		div.textContent = 'Hello';
		frag.appendChild(div);

		var span = document.createElement('span');
		span.id = 'mySpan';
		span.textContent = 'World';
		div.appendChild(span);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test finding element by ID
	result, err := r.Execute("frag.getElementById('myDiv').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "HelloWorld" {
		t.Errorf("Expected 'HelloWorld', got %v", result.String())
	}

	// Test finding nested element by ID
	result, err = r.Execute("frag.getElementById('mySpan').textContent")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "World" {
		t.Errorf("Expected 'World', got %v", result.String())
	}

	// Test non-existent ID returns null
	result, err = r.Execute("frag.getElementById('nonexistent') === null")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Errorf("Expected null for non-existent ID")
	}

	// Test empty string ID returns null (per spec)
	result, err = r.Execute("frag.getElementById('') === null")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Errorf("Expected null for empty string ID")
	}

	// Test that getElementById exists on the prototype
	result, err = r.Execute("typeof DocumentFragment.prototype.getElementById")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected DocumentFragment.prototype.getElementById to be a function, got %v", result.String())
	}

	// Test that getElementById can be called via prototype
	result, err = r.Execute("DocumentFragment.prototype.getElementById.call(frag, 'myDiv') === frag.getElementById('myDiv')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Errorf("Expected prototype method to work when called with call()")
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

func TestDOMParser(t *testing.T) {
	runtime := NewRuntime()
	binder := NewDOMBinder(runtime)

	// Need to bind a document first for the binder to work properly
	doc, _ := dom.ParseHTML("<html><body></body></html>")
	binder.BindDocument(doc)

	// Test that DOMParser exists
	result, err := runtime.Execute(`typeof DOMParser`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "function" {
		t.Errorf("DOMParser should be a function, got %s", result.String())
	}

	// Test DOMParser can be constructed
	result, err = runtime.Execute(`typeof new DOMParser()`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "object" {
		t.Errorf("new DOMParser() should return an object, got %s", result.String())
	}

	// Test instanceof
	result, err = runtime.Execute(`new DOMParser() instanceof DOMParser`)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToBoolean() {
		t.Error("new DOMParser() should be instanceof DOMParser")
	}

	// Test parseFromString with text/html
	result, err = runtime.Execute(`
		var parser = new DOMParser();
		var doc = parser.parseFromString('<div id="test">Hello</div>', 'text/html');
		doc.getElementById('test').textContent
	`)
	if err != nil {
		t.Fatal(err)
	}
	if result.String() != "Hello" {
		t.Errorf("Expected 'Hello', got %s", result.String())
	}

	// Test parseFromString with text/xml returns XMLDocument
	result, err = runtime.Execute(`
		var parser = new DOMParser();
		var doc = parser.parseFromString('<root><item>Test</item></root>', 'text/xml');
		doc instanceof XMLDocument
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToBoolean() {
		t.Error("parseFromString with text/xml should return XMLDocument")
	}

	// Test parseFromString with application/xml
	result, err = runtime.Execute(`
		var parser = new DOMParser();
		var doc = parser.parseFromString('<root>Test</root>', 'application/xml');
		doc instanceof XMLDocument
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToBoolean() {
		t.Error("parseFromString with application/xml should return XMLDocument")
	}

	// Test that parseFromString requires two arguments
	_, err = runtime.Execute(`
		var parser = new DOMParser();
		parser.parseFromString('<div>test</div>');
	`)
	if err == nil {
		t.Error("parseFromString with one argument should throw")
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

func TestDOMBinderProcessingInstruction(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML("<!DOCTYPE html><html><body></body></html>")
	binder.BindDocument(doc)

	// Test createProcessingInstruction exists
	result, err := r.Execute("typeof document.createProcessingInstruction")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected createProcessingInstruction to be a function, got %v", result.String())
	}

	// Test creating a processing instruction
	result, err = r.Execute(`
		var pi = document.createProcessingInstruction('xml-stylesheet', 'href="test.css"');
		pi.nodeType
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 7 {
		t.Errorf("Expected nodeType = 7, got %v", result.String())
	}

	// Test target property
	result, err = r.Execute(`pi.target`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "xml-stylesheet" {
		t.Errorf("Expected target = 'xml-stylesheet', got %v", result.String())
	}

	// Test nodeName is same as target
	result, err = r.Execute(`pi.nodeName`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "xml-stylesheet" {
		t.Errorf("Expected nodeName = 'xml-stylesheet', got %v", result.String())
	}

	// Test data property
	result, err = r.Execute(`pi.data`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "href=\"test.css\"" {
		t.Errorf("Expected data = 'href=\"test.css\"', got %v", result.String())
	}

	// Test nodeValue is same as data
	result, err = r.Execute(`pi.nodeValue`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "href=\"test.css\"" {
		t.Errorf("Expected nodeValue = 'href=\"test.css\"', got %v", result.String())
	}

	// Test length property
	result, err = r.Execute(`pi.length`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	expectedLen := len("href=\"test.css\"")
	if result.ToInteger() != int64(expectedLen) {
		t.Errorf("Expected length = %d, got %v", expectedLen, result.String())
	}

	// Test instanceof ProcessingInstruction
	result, err = r.Execute(`pi instanceof ProcessingInstruction`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected pi instanceof ProcessingInstruction to be true, got %v", result.String())
	}

	// Test instanceof CharacterData
	result, err = r.Execute(`pi instanceof CharacterData`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected pi instanceof CharacterData to be true, got %v", result.String())
	}

	// Test instanceof Node
	result, err = r.Execute(`pi instanceof Node`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected pi instanceof Node to be true, got %v", result.String())
	}

	// Test CharacterData methods
	result, err = r.Execute(`pi.substringData(0, 4)`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "href" {
		t.Errorf("Expected substringData(0, 4) = 'href', got %v", result.String())
	}

	// Test setting data
	result, err = r.Execute(`
		pi.data = 'new data';
		pi.data
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "new data" {
		t.Errorf("Expected data = 'new data', got %v", result.String())
	}

	// Test can be appended to document
	result, err = r.Execute(`
		var pi2 = document.createProcessingInstruction('test', 'value');
		document.body.appendChild(pi2);
		document.body.lastChild.target
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "test" {
		t.Errorf("Expected appended PI target = 'test', got %v", result.String())
	}
}

func TestDOMBinderProcessingInstructionErrors(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML("<!DOCTYPE html><html><body></body></html>")
	binder.BindDocument(doc)

	// Test error for invalid target (starts with number)
	_, err := r.Execute(`document.createProcessingInstruction('123invalid', 'data')`)
	if err == nil {
		t.Error("Expected error for invalid target starting with number")
	}

	// Test error for data containing "?>"
	_, err = r.Execute(`document.createProcessingInstruction('valid', 'data?>more')`)
	if err == nil {
		t.Error("Expected error for data containing '?>'")
	}

	// Test error for missing arguments
	_, err = r.Execute(`document.createProcessingInstruction('target')`)
	if err == nil {
		t.Error("Expected error for missing data argument")
	}
}

func TestDOMBinderCDATASection(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	// Create XML document using DOMImplementation
	impl := dom.NewDOMImplementation(nil)
	xmlDoc, _ := impl.CreateDocument("http://example.com", "root", nil)
	binder.BindDocument(xmlDoc)

	// Test createCDATASection exists
	result, err := r.Execute("typeof document.createCDATASection")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected createCDATASection to be a function, got %v", result.String())
	}

	// Test creating a CDATASection
	result, err = r.Execute(`
		var cdata = document.createCDATASection('test content');
		cdata.nodeType
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 4 {
		t.Errorf("Expected nodeType = 4, got %v", result.String())
	}

	// Test nodeName
	result, err = r.Execute(`cdata.nodeName`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "#cdata-section" {
		t.Errorf("Expected nodeName = '#cdata-section', got %v", result.String())
	}

	// Test data property
	result, err = r.Execute(`cdata.data`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "test content" {
		t.Errorf("Expected data = 'test content', got %v", result.String())
	}

	// Test length property
	result, err = r.Execute(`cdata.length`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 12 { // "test content" = 12 characters
		t.Errorf("Expected length = 12, got %v", result.String())
	}

	// Test instanceof CDATASection
	result, err = r.Execute(`cdata instanceof CDATASection`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected cdata instanceof CDATASection to be true, got %v", result.String())
	}

	// Test instanceof Text (CDATASection extends Text)
	result, err = r.Execute(`cdata instanceof Text`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected cdata instanceof Text to be true, got %v", result.String())
	}

	// Test instanceof CharacterData
	result, err = r.Execute(`cdata instanceof CharacterData`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "true" {
		t.Errorf("Expected cdata instanceof CharacterData to be true, got %v", result.String())
	}

	// Test setting data
	result, err = r.Execute(`
		cdata.data = 'new data';
		cdata.data
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "new data" {
		t.Errorf("Expected data = 'new data', got %v", result.String())
	}

	// Test CharacterData methods
	result, err = r.Execute(`cdata.substringData(0, 3)`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "new" {
		t.Errorf("Expected substringData(0, 3) = 'new', got %v", result.String())
	}
}

func TestDOMBinderCDATASectionErrors(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	// Test with HTML document - should throw NotSupportedError
	doc, _ := dom.ParseHTML("<!DOCTYPE html><html><body></body></html>")
	binder.BindDocument(doc)

	_, err := r.Execute(`document.createCDATASection('test')`)
	if err == nil {
		t.Error("Expected NotSupportedError for CDATASection in HTML document")
	}

	// Test with XML document - should throw InvalidCharacterError for data with "]]>"
	impl := dom.NewDOMImplementation(nil)
	xmlDoc, _ := impl.CreateDocument("http://example.com", "root", nil)

	// Create a new runtime for XML doc
	r2 := NewRuntime()
	binder2 := NewDOMBinder(r2)
	binder2.BindDocument(xmlDoc)

	_, err = r2.Execute(`document.createCDATASection('data with ]]> in it')`)
	if err == nil {
		t.Error("Expected InvalidCharacterError for data containing ']]>'")
	}
}

func TestBeforeAfterNullUndefined(t *testing.T) {
	r := NewRuntime()
	doc, _ := dom.ParseHTML(`<div id="parent"><span id="test">test</span></div>`)
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	// Test element.before(null)
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var child = document.getElementById('test');
		child.before(null);
		parent.innerHTML;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('parent').innerHTML")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	expected := `null<span id="test">test</span>`
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}

	// Test element.before(undefined)
	_, err = r.Execute(`
		var child = document.getElementById('test');
		child.before(undefined);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("document.getElementById('parent').innerHTML")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	expected = `nullundefined<span id="test">test</span>`
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}
}

func TestCommentBeforeNullUndefined(t *testing.T) {
	r := NewRuntime()
	doc, _ := dom.ParseHTML(`<div id="parent"><!--test--></div>`)
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	// Test comment.before(null)
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var comment = parent.childNodes[0];
		comment.before(null);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('parent').innerHTML")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	expected := `null<!--test-->`
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}
}

func TestTextBeforeNullUndefined(t *testing.T) {
	r := NewRuntime()
	doc, _ := dom.ParseHTML(`<div id="parent">test</div>`)
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	// Test text.before(null)
	_, err := r.Execute(`
		var parent = document.getElementById('parent');
		var text = parent.childNodes[0];
		text.before(null);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("document.getElementById('parent').innerHTML")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	expected := `nulltest`
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}
}

func TestDOMBinderCSSStyleDeclaration(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test style is an object
	result, err := r.Execute("typeof document.getElementById('test').style")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "object" {
		t.Errorf("Expected style to be object, got %s", result.String())
	}

	// Test initial cssText is empty
	result, err = r.Execute("document.getElementById('test').style.cssText")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "" {
		t.Errorf("Expected empty cssText, got %s", result.String())
	}

	// Test initial length is 0
	result, err = r.Execute("document.getElementById('test').style.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 0 {
		t.Errorf("Expected length 0, got %d", result.ToInteger())
	}

	// Test setProperty
	_, err = r.Execute("document.getElementById('test').style.setProperty('color', 'red')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test getPropertyValue
	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "red" {
		t.Errorf("Expected 'red', got %s", result.String())
	}

	// Test length is now 1
	result, err = r.Execute("document.getElementById('test').style.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected length 1, got %d", result.ToInteger())
	}

	// Test item(0)
	result, err = r.Execute("document.getElementById('test').style.item(0)")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "color" {
		t.Errorf("Expected 'color', got %s", result.String())
	}

	// Test cssText
	result, err = r.Execute("document.getElementById('test').style.cssText")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "color: red" {
		t.Errorf("Expected 'color: red', got %s", result.String())
	}
}

func TestDOMBinderCSSStyleDeclarationCamelCase(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test setting via camelCase property
	_, err := r.Execute("document.getElementById('test').style.backgroundColor = 'blue'")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test getting via camelCase property
	result, err := r.Execute("document.getElementById('test').style.backgroundColor")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "blue" {
		t.Errorf("Expected 'blue', got %s", result.String())
	}

	// Test that it's stored as kebab-case
	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('background-color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "blue" {
		t.Errorf("Expected 'blue', got %s", result.String())
	}

	// Test cssText uses kebab-case
	result, err = r.Execute("document.getElementById('test').style.cssText")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "background-color: blue" {
		t.Errorf("Expected 'background-color: blue', got %s", result.String())
	}
}

func TestDOMBinderCSSStyleDeclarationRemove(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Set multiple properties
	_, err := r.Execute(`
		var el = document.getElementById('test');
		el.style.setProperty('color', 'red');
		el.style.setProperty('width', '100px');
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test removeProperty
	result, err := r.Execute("document.getElementById('test').style.removeProperty('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "red" {
		t.Errorf("Expected old value 'red', got %s", result.String())
	}

	// Verify it's removed
	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "" {
		t.Errorf("Expected empty string, got %s", result.String())
	}

	// Verify length is now 1
	result, err = r.Execute("document.getElementById('test').style.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected length 1, got %d", result.ToInteger())
	}
}

func TestDOMBinderCSSStyleDeclarationPriority(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Set property with important
	_, err := r.Execute("document.getElementById('test').style.setProperty('color', 'red', 'important')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test getPropertyPriority
	result, err := r.Execute("document.getElementById('test').style.getPropertyPriority('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "important" {
		t.Errorf("Expected 'important', got %s", result.String())
	}

	// Test cssText includes !important
	result, err = r.Execute("document.getElementById('test').style.cssText")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "color: red !important" {
		t.Errorf("Expected 'color: red !important', got %s", result.String())
	}
}

func TestDOMBinderCSSStyleDeclarationSetCSSText(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Set cssText directly
	_, err := r.Execute("document.getElementById('test').style.cssText = 'color: green; font-size: 14px'")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify properties
	result, err := r.Execute("document.getElementById('test').style.getPropertyValue('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "green" {
		t.Errorf("Expected 'green', got %s", result.String())
	}

	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('font-size')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "14px" {
		t.Errorf("Expected '14px', got %s", result.String())
	}

	// Verify length
	result, err = r.Execute("document.getElementById('test').style.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected length 2, got %d", result.ToInteger())
	}
}

func TestDOMBinderCSSStyleDeclarationSyncToAttribute(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Set style via JS
	_, err := r.Execute("document.getElementById('test').style.color = 'purple'")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify style attribute is set
	result, err := r.Execute("document.getElementById('test').getAttribute('style')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "color: purple" {
		t.Errorf("Expected 'color: purple', got %s", result.String())
	}
}

func TestDOMBinderCSSStyleDeclarationFromAttribute(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" style="margin: 10px; padding: 5px"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Verify style properties are parsed from attribute
	result, err := r.Execute("document.getElementById('test').style.getPropertyValue('margin')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "10px" {
		t.Errorf("Expected '10px', got %s", result.String())
	}

	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('padding')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "5px" {
		t.Errorf("Expected '5px', got %s", result.String())
	}

	result, err = r.Execute("document.getElementById('test').style.length")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 2 {
		t.Errorf("Expected length 2, got %d", result.ToInteger())
	}
}

func TestDOMBinderCSSStyleDeclarationSetViaSetter(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test" style="color: red;"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test assigning a string to element.style (should set cssText)
	_, err := r.Execute("document.getElementById('test').style = 'background: blue'")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// The cssText should now be 'background: blue' (old styles replaced)
	result, err := r.Execute("document.getElementById('test').style.cssText")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "background: blue" {
		t.Errorf("Expected 'background: blue', got %s", result.String())
	}

	// The original color style should be gone
	result, err = r.Execute("document.getElementById('test').style.getPropertyValue('color')")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "" {
		t.Errorf("Expected empty color after style replacement, got %s", result.String())
	}
}

func TestHTMLElementTypeConstructors(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	tests := []struct {
		code     string
		expected string
	}{
		// Check constructors exist
		{"typeof HTMLElement", "function"},
		{"typeof HTMLDivElement", "function"},
		{"typeof HTMLAnchorElement", "function"},
		{"typeof HTMLSpanElement", "function"},
		{"typeof HTMLUnknownElement", "function"},
		{"typeof HTMLHeadingElement", "function"},
		{"typeof HTMLParagraphElement", "function"},

		// Check instanceof for various elements
		{"document.createElement('div') instanceof Element", "true"},
		{"document.createElement('div') instanceof HTMLElement", "true"},
		{"document.createElement('div') instanceof HTMLDivElement", "true"},
		{"document.createElement('div') instanceof Node", "true"},

		{"document.createElement('a') instanceof HTMLAnchorElement", "true"},
		{"document.createElement('span') instanceof HTMLSpanElement", "true"},
		{"document.createElement('p') instanceof HTMLParagraphElement", "true"},
		{"document.createElement('h1') instanceof HTMLHeadingElement", "true"},
		{"document.createElement('h2') instanceof HTMLHeadingElement", "true"},
		{"document.createElement('article') instanceof HTMLElement", "true"},
		{"document.createElement('section') instanceof HTMLElement", "true"},

		// Check that wrong types return false
		{"document.createElement('div') instanceof HTMLAnchorElement", "false"},
		{"document.createElement('a') instanceof HTMLDivElement", "false"},

		// Check prototype chain
		{"HTMLDivElement.prototype instanceof HTMLElement", "true"},
		{"HTMLElement.prototype instanceof Element", "true"},
		{"Element.prototype instanceof Node", "true"},

		// Check unknown elements
		{"document.createElement('custom-element') instanceof HTMLUnknownElement", "true"},
		{"document.createElement('custom-element') instanceof HTMLElement", "true"},
		{"document.createElement('custom-element') instanceof Element", "true"},

		// Check deprecated/legacy elements (WPT compatibility)
		{"typeof HTMLDirectoryElement", "function"},
		{"typeof HTMLFrameElement", "function"},
		{"typeof HTMLFrameSetElement", "function"},
		{"document.createElement('dir') instanceof HTMLDirectoryElement", "true"},
		{"document.createElement('frame') instanceof HTMLFrameElement", "true"},
		{"document.createElement('frameset') instanceof HTMLFrameSetElement", "true"},
		{"document.createElement('dir') instanceof HTMLElement", "true"},
		{"document.createElement('frame') instanceof HTMLElement", "true"},
		{"document.createElement('frameset') instanceof HTMLElement", "true"},
	}

	for _, tt := range tests {
		result, err := r.Execute(tt.code)
		if err != nil {
			t.Errorf("%s: error: %v", tt.code, err)
			continue
		}
		if result.String() != tt.expected {
			t.Errorf("%s: expected %s, got %s", tt.code, tt.expected, result.String())
		}
	}
}

func TestDOMBinderRange(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Create a document structure
	_, err := r.Execute(`
		var div = document.createElement('div');
		document.documentElement = div;
		var text = document.createTextNode('Hello World');
		div.appendChild(text);
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test createRange
	result, err := r.Execute("typeof document.createRange")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected 'function', got %v", result.String())
	}

	// Create a range
	_, err = r.Execute("var range = document.createRange()")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test Range instanceof
	result, err = r.Execute("range instanceof Range")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("range should be instanceof Range")
	}

	// Test Range constants
	tests := []struct {
		code     string
		expected int64
	}{
		{"Range.START_TO_START", 0},
		{"Range.START_TO_END", 1},
		{"Range.END_TO_END", 2},
		{"Range.END_TO_START", 3},
		{"range.START_TO_START", 0},
		{"range.START_TO_END", 1},
		{"range.END_TO_END", 2},
		{"range.END_TO_START", 3},
	}

	for _, tt := range tests {
		result, err := r.Execute(tt.code)
		if err != nil {
			t.Errorf("%s: error: %v", tt.code, err)
			continue
		}
		if result.ToInteger() != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.code, tt.expected, result.ToInteger())
		}
	}

	// Test collapsed property on new range
	result, err = r.Execute("range.collapsed")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("New range should be collapsed")
	}
}

func TestDOMBinderRangeSetStartEnd(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Setup
	_, err := r.Execute(`
		var div = document.createElement('div');
		var text = document.createTextNode('Hello World');
		div.appendChild(text);
		var range = document.createRange();
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Set start and end
	_, err = r.Execute(`
		range.setStart(text, 0);
		range.setEnd(text, 5);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test startOffset
	result, err := r.Execute("range.startOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 0 {
		t.Errorf("Expected startOffset 0, got %d", result.ToInteger())
	}

	// Test endOffset
	result, err = r.Execute("range.endOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 5 {
		t.Errorf("Expected endOffset 5, got %d", result.ToInteger())
	}

	// Test collapsed is false
	result, err = r.Execute("range.collapsed")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToBoolean() {
		t.Error("Range should not be collapsed")
	}

	// Test toString
	result, err = r.Execute("range.toString()")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", result.String())
	}
}

func TestDOMBinderRangeCollapse(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Setup
	_, err := r.Execute(`
		var div = document.createElement('div');
		var text = document.createTextNode('Hello');
		div.appendChild(text);
		var range = document.createRange();
		range.setStart(text, 0);
		range.setEnd(text, 5);
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Collapse to start
	_, err = r.Execute("range.collapse(true)")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err := r.Execute("range.collapsed")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Range should be collapsed")
	}

	result, err = r.Execute("range.endOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 0 {
		t.Errorf("Expected endOffset 0 after collapse to start, got %d", result.ToInteger())
	}
}

func TestDOMBinderRangeCloneRange(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Setup
	_, err := r.Execute(`
		var div = document.createElement('div');
		var text = document.createTextNode('Hello');
		div.appendChild(text);
		var range = document.createRange();
		range.setStart(text, 1);
		range.setEnd(text, 4);
		var clone = range.cloneRange();
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test clone has same offsets
	result, err := r.Execute("clone.startOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected startOffset 1, got %d", result.ToInteger())
	}

	result, err = r.Execute("clone.endOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 4 {
		t.Errorf("Expected endOffset 4, got %d", result.ToInteger())
	}

	// Test clone is independent
	_, err = r.Execute("range.setStart(text, 0)")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result, err = r.Execute("clone.startOffset")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Clone should be independent, expected startOffset 1, got %d", result.ToInteger())
	}
}

func TestDOMBinderRangeExtractContents(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Setup
	_, err := r.Execute(`
		var div = document.createElement('div');
		var text = document.createTextNode('Hello World');
		div.appendChild(text);
		var range = document.createRange();
		range.setStart(text, 0);
		range.setEnd(text, 5);
		var frag = range.extractContents();
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test fragment is DocumentFragment
	result, err := r.Execute("frag instanceof DocumentFragment")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("extractContents should return a DocumentFragment")
	}

	// Test original text was modified
	result, err = r.Execute("text.nodeValue")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != " World" {
		t.Errorf("Expected ' World', got '%s'", result.String())
	}
}

func TestDOMBinderRangeCreateContextualFragment(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Setup
	_, err := r.Execute(`
		var div = document.createElement('div');
		var text = document.createTextNode('Test');
		div.appendChild(text);
		var range = document.createRange();
		range.setStart(text, 0);
		var frag = range.createContextualFragment('<span>New Content</span>');
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test fragment is DocumentFragment
	result, err := r.Execute("frag instanceof DocumentFragment")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("createContextualFragment should return a DocumentFragment")
	}

	// Test fragment has content
	result, err = r.Execute("frag.firstChild.tagName")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "SPAN" {
		t.Errorf("Expected 'SPAN', got '%s'", result.String())
	}
}

func TestDOMBinderGeometryAPIs(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html><html><head></head><body></body></html>`)
	binder.BindDocument(doc)

	// Create an element and set its geometry
	_, err := r.Execute(`
		var div = document.createElement('div');
		document.body.appendChild(div);
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Get the Go element and set geometry
	divVal, _ := r.Execute("div")
	if divVal == nil {
		t.Fatal("Could not get div element")
	}
	divObj := divVal.ToObject(r.vm)
	goElVal := divObj.Get("_goElement")
	if goElVal == nil {
		t.Fatal("Could not get Go element")
	}
	goEl := goElVal.Export().(*dom.Element)
	geom := &dom.ElementGeometry{
		X:            10,
		Y:            20,
		Width:        100,
		Height:       50,
		OffsetWidth:  100,
		OffsetHeight: 50,
		OffsetTop:    20,
		OffsetLeft:   10,
		ClientWidth:  90,
		ClientHeight: 40,
		ClientTop:    5,
		ClientLeft:   5,
	}
	goEl.SetGeometry(geom)

	// Test getBoundingClientRect
	result, err := r.Execute("div.getBoundingClientRect().x")
	if err != nil {
		t.Fatalf("getBoundingClientRect failed: %v", err)
	}
	if result.ToFloat() != 10 {
		t.Errorf("Expected x=10, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.getBoundingClientRect().y")
	if err != nil {
		t.Fatalf("getBoundingClientRect failed: %v", err)
	}
	if result.ToFloat() != 20 {
		t.Errorf("Expected y=20, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.getBoundingClientRect().width")
	if err != nil {
		t.Fatalf("getBoundingClientRect failed: %v", err)
	}
	if result.ToFloat() != 100 {
		t.Errorf("Expected width=100, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.getBoundingClientRect().height")
	if err != nil {
		t.Fatalf("getBoundingClientRect failed: %v", err)
	}
	if result.ToFloat() != 50 {
		t.Errorf("Expected height=50, got %v", result.ToFloat())
	}

	// Test computed properties
	result, err = r.Execute("div.getBoundingClientRect().top")
	if err != nil {
		t.Fatalf("getBoundingClientRect.top failed: %v", err)
	}
	if result.ToFloat() != 20 {
		t.Errorf("Expected top=20, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.getBoundingClientRect().right")
	if err != nil {
		t.Fatalf("getBoundingClientRect.right failed: %v", err)
	}
	if result.ToFloat() != 110 {
		t.Errorf("Expected right=110, got %v", result.ToFloat())
	}

	// Test offsetWidth/offsetHeight
	result, err = r.Execute("div.offsetWidth")
	if err != nil {
		t.Fatalf("offsetWidth failed: %v", err)
	}
	if result.ToFloat() != 100 {
		t.Errorf("Expected offsetWidth=100, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.offsetHeight")
	if err != nil {
		t.Fatalf("offsetHeight failed: %v", err)
	}
	if result.ToFloat() != 50 {
		t.Errorf("Expected offsetHeight=50, got %v", result.ToFloat())
	}

	// Test clientWidth/clientHeight
	result, err = r.Execute("div.clientWidth")
	if err != nil {
		t.Fatalf("clientWidth failed: %v", err)
	}
	if result.ToFloat() != 90 {
		t.Errorf("Expected clientWidth=90, got %v", result.ToFloat())
	}

	result, err = r.Execute("div.clientHeight")
	if err != nil {
		t.Fatalf("clientHeight failed: %v", err)
	}
	if result.ToFloat() != 40 {
		t.Errorf("Expected clientHeight=40, got %v", result.ToFloat())
	}
}

func TestDOMBinderDOMRectConstructor(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test DOMRect constructor with all args
	result, err := r.Execute("new DOMRect(10, 20, 100, 50).x")
	if err != nil {
		t.Fatalf("DOMRect constructor failed: %v", err)
	}
	if result.ToFloat() != 10 {
		t.Errorf("Expected x=10, got %v", result.ToFloat())
	}

	// Test DOMRect constructor with partial args
	result, err = r.Execute("new DOMRect(5).y")
	if err != nil {
		t.Fatalf("DOMRect constructor with partial args failed: %v", err)
	}
	if result.ToFloat() != 0 {
		t.Errorf("Expected y=0, got %v", result.ToFloat())
	}

	// Test DOMRect computed properties
	result, err = r.Execute("var r = new DOMRect(10, 20, 100, 50); r.bottom")
	if err != nil {
		t.Fatalf("DOMRect.bottom failed: %v", err)
	}
	if result.ToFloat() != 70 {
		t.Errorf("Expected bottom=70, got %v", result.ToFloat())
	}

	// Test DOMRect.fromRect
	result, err = r.Execute("DOMRect.fromRect({x: 5, y: 10, width: 50, height: 25}).width")
	if err != nil {
		t.Fatalf("DOMRect.fromRect failed: %v", err)
	}
	if result.ToFloat() != 50 {
		t.Errorf("Expected width=50, got %v", result.ToFloat())
	}

	// Test DOMRectReadOnly
	result, err = r.Execute("new DOMRectReadOnly(1, 2, 3, 4).x")
	if err != nil {
		t.Fatalf("DOMRectReadOnly constructor failed: %v", err)
	}
	if result.ToFloat() != 1 {
		t.Errorf("Expected x=1, got %v", result.ToFloat())
	}
}

func TestDOMBinderScrollProperties(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html><html><head></head><body></body></html>`)
	binder.BindDocument(doc)

	_, err := r.Execute(`
		var div = document.createElement('div');
		document.body.appendChild(div);
	`)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Initially scroll values should be 0
	result, err := r.Execute("div.scrollTop")
	if err != nil {
		t.Fatalf("scrollTop read failed: %v", err)
	}
	if result.ToFloat() != 0 {
		t.Errorf("Expected scrollTop=0, got %v", result.ToFloat())
	}

	// Set scrollTop
	_, err = r.Execute("div.scrollTop = 50")
	if err != nil {
		t.Fatalf("scrollTop write failed: %v", err)
	}

	result, err = r.Execute("div.scrollTop")
	if err != nil {
		t.Fatalf("scrollTop read after write failed: %v", err)
	}
	if result.ToFloat() != 50 {
		t.Errorf("Expected scrollTop=50, got %v", result.ToFloat())
	}

	// Set scrollLeft
	_, err = r.Execute("div.scrollLeft = 30")
	if err != nil {
		t.Fatalf("scrollLeft write failed: %v", err)
	}

	result, err = r.Execute("div.scrollLeft")
	if err != nil {
		t.Fatalf("scrollLeft read after write failed: %v", err)
	}
	if result.ToFloat() != 30 {
		t.Errorf("Expected scrollLeft=30, got %v", result.ToFloat())
	}
}

func TestDOMTokenList_Iteration(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html><html><body><span id="test" class="a a b c"></span></body></html>`)
	executor.SetupDocument(doc)

	// Test that values() returns an iterator
	result, err := r.Execute(`
		var list = document.getElementById('test').classList;
		typeof list.values === 'function'
	`)
	if err != nil {
		t.Fatalf("Error checking values method: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected classList.values to be a function")
	}

	// Test that values is from Array.prototype
	result, err = r.Execute(`
		var list = document.getElementById('test').classList;
		list.values === Array.prototype.values
	`)
	if err != nil {
		t.Fatalf("Error checking values identity: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected classList.values === Array.prototype.values")
	}

	// Test that keys() works
	result, err = r.Execute(`
		var list = document.getElementById('test').classList;
		[...list.keys()].join(',')
	`)
	if err != nil {
		t.Fatalf("Error calling keys(): %v", err)
	}
	if result.String() != "0,1,2" {
		t.Errorf("Expected keys to be '0,1,2', got '%s'", result.String())
	}

	// Test that values() works
	result, err = r.Execute(`
		var list = document.getElementById('test').classList;
		[...list.values()].join(',')
	`)
	if err != nil {
		t.Fatalf("Error calling values(): %v", err)
	}
	if result.String() != "a,b,c" {
		t.Errorf("Expected values to be 'a,b,c', got '%s'", result.String())
	}

	// Test that Symbol.iterator works (for-of)
	result, err = r.Execute(`
		var list = document.getElementById('test').classList;
		[...list].join(',')
	`)
	if err != nil {
		t.Fatalf("Error using spread operator: %v", err)
	}
	if result.String() != "a,b,c" {
		t.Errorf("Expected [...list] to be 'a,b,c', got '%s'", result.String())
	}

	// Test forEach
	result, err = r.Execute(`
		var list = document.getElementById('test').classList;
		var result = [];
		list.forEach(function(v, k) { result.push(k + ':' + v); });
		result.join(',')
	`)
	if err != nil {
		t.Fatalf("Error calling forEach: %v", err)
	}
	if result.String() != "0:a,1:b,2:c" {
		t.Errorf("Expected forEach result to be '0:a,1:b,2:c', got '%s'", result.String())
	}
}

func TestCreateNodeIteratorWithForeignDocument(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	// Test creating a NodeIterator with a node from a foreign document
	_, err := r.Execute(`
		var foreignDoc = document.implementation.createHTMLDocument("");
		var foreignPara = foreignDoc.createElement("p");
		foreignPara.textContent = "Hello";
		foreignDoc.body.appendChild(foreignPara);
		
		// This should work - using a foreign node as root
		var iter = document.createNodeIterator(foreignPara);
		iter.root.nodeType; // Should be 1 (element)
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestCreateNodeIteratorWithVariousNodeTypes(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc := dom.NewDocument()
	binder.BindDocument(doc)

	tests := []struct {
		name   string
		script string
	}{
		{
			"comment node",
			`var comment = document.createComment("test"); document.createNodeIterator(comment);`,
		},
		{
			"text node",
			`var text = document.createTextNode("test"); document.createNodeIterator(text);`,
		},
		{
			"doctype",
			`document.createNodeIterator(document.doctype || document);`,
		},
		{
			"foreign element",
			`var foreignDoc = document.implementation.createHTMLDocument("");
			 var para = foreignDoc.createElement("p");
			 foreignDoc.body.appendChild(para);
			 document.createNodeIterator(para);`,
		},
		{
			"foreign text node",
			`var foreignDoc = document.implementation.createHTMLDocument("");
			 var text = foreignDoc.createTextNode("test");
			 document.createNodeIterator(text);`,
		},
		{
			"xml element",
			`var xmlDoc = document.implementation.createDocument(null, null, null);
			 var el = xmlDoc.createElement("test");
			 xmlDoc.appendChild(el);
			 document.createNodeIterator(el);`,
		},
		{
			"processing instruction",
			`var xmlDoc = document.implementation.createDocument(null, null, null);
			 var pi = xmlDoc.createProcessingInstruction("test", "data");
			 xmlDoc.appendChild(pi);
			 document.createNodeIterator(pi);`,
		},
		{
			"xml comment",
			`var xmlDoc = document.implementation.createDocument(null, null, null);
			 var comment = xmlDoc.createComment("test");
			 xmlDoc.appendChild(comment);
			 document.createNodeIterator(comment);`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Execute(tt.script)
			if err != nil {
				t.Errorf("%s failed: %v", tt.name, err)
			}
		})
	}
}

func TestWPTNodeIteratorRemovalSetup(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	// Create a document with a test div like the WPT test
	doc, err := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body><div id="test"></div></body>
</html>`)
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}
	binder.BindDocument(doc)

	// Run the WPT setup script
	_, err = r.Execute(`
		// Variables like the WPT test
		var testDiv, paras, detachedDiv, detachedPara1, detachedPara2,
		    foreignDoc, foreignPara1, foreignPara2, xmlDoc, xmlElement,
		    detachedXmlElement, detachedTextNode, foreignTextNode,
		    detachedForeignTextNode, xmlTextNode, detachedXmlTextNode,
		    processingInstruction, detachedProcessingInstruction, comment,
		    detachedComment, foreignComment, detachedForeignComment, xmlComment,
		    detachedXmlComment, docfrag, foreignDocfrag, xmlDocfrag, doctype,
		    foreignDoctype, xmlDoctype;

		testDiv = document.querySelector("#test");
		
		paras = [];
		paras.push(document.createElement("p"));
		paras[0].setAttribute("id", "a");
		paras[0].textContent = "test text";
		testDiv.appendChild(paras[0]);

		paras.push(document.createElement("p"));
		paras[1].setAttribute("id", "b");
		paras[1].textContent = "Ijklmnop\n";
		testDiv.appendChild(paras[1]);

		// Create para[5] with CDATA sections
		paras.push(document.createElement("p")); // 2
		paras.push(document.createElement("p")); // 3
		paras.push(document.createElement("p")); // 4
		paras.push(document.createElement("p")); // 5

		var xmlDocument = new Document();
		console.log("xmlDocument type:", typeof xmlDocument);
		
		// Create CDATA section
		var cdata = xmlDocument.createCDATASection("1234");
		console.log("cdata nodeType:", cdata.nodeType);
		paras[5].appendChild(cdata);
		testDiv.appendChild(paras[5]);

		// Create foreign document
		foreignDoc = document.implementation.createHTMLDocument("");
		foreignPara1 = foreignDoc.createElement("p");
		foreignPara1.appendChild(foreignDoc.createTextNode("Efghijkl"));
		foreignDoc.body.appendChild(foreignPara1);

		// Comment
		comment = document.createComment("Alphabet soup?");
		testDiv.appendChild(comment);

		// Now test createNodeIterator with these nodes
		var testNodes = [
			"paras[0]",
			"paras[0].firstChild",
			"paras[1].firstChild",
			"paras[5].firstChild",  // CDATA section
			"foreignPara1",
			"foreignPara1.firstChild",
			"comment"
		];

		var results = [];
		for (var i = 0; i < testNodes.length; i++) {
			var nodeName = testNodes[i];
			var node = eval(nodeName);
			try {
				var iter = document.createNodeIterator(node);
				results.push(nodeName + ": OK");
			} catch (e) {
				results.push(nodeName + ": FAIL - " + e.message);
			}
		}
		results.join("\n");
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestWPTNodeIteratorMultipleAdvancement(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML("<!DOCTYPE html><html><head></head><body><div id=\"test\"></div></body></html>")
	binder.BindDocument(doc)

	// Simulate the WPT test's loop pattern
	result, err := r.Execute(`
		var testDiv = document.querySelector("#test");
		
		var paras = [];
		for (var i = 0; i < 6; i++) {
			paras.push(document.createElement("p"));
			paras[i].textContent = "Text " + i;
			testDiv.appendChild(paras[i]);
		}
		
		var foreignDoc = document.implementation.createHTMLDocument("");
		var foreignPara1 = foreignDoc.createElement("p");
		foreignPara1.textContent = "Foreign";
		foreignDoc.body.appendChild(foreignPara1);
		
		var comment = document.createComment("test comment");
		testDiv.appendChild(comment);
		
		var doctype = document.doctype;
		
		var testNodesShort = [
			"paras[0]",
			"paras[0].firstChild",
			"paras[1].firstChild",
			"foreignPara1",
			"foreignPara1.firstChild",
			"document",
			"foreignDoc",
			"comment",
			"doctype"
		];
		
		var errors = [];
		
		// This is the pattern from the WPT test - for each test node, try all test nodes as roots
		for (var i = 0; i < testNodesShort.length; i++) {
			var nodeName = testNodesShort[i];
			var node = eval(nodeName);
			if (!node || !node.parentNode) {
				continue;
			}
			
			// Inner loop - try all test nodes as root
			for (var j = 0; j < testNodesShort.length; j++) {
				var rootName = testNodesShort[j];
				var root = eval(rootName);
				
				try {
					// Create iterator and advance it k times
					for (var k = 0; k < 5; k++) {
						var iter = document.createNodeIterator(root);
						for (var l = 0; l < k; l++) {
							iter.nextNode();
						}
					}
				} catch (e) {
					errors.push("Error with node=" + nodeName + ", root=" + rootName + ": " + e.message);
				}
			}
		}
		
		errors.length > 0 ? errors.join("\\n") : "All passed";
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	
	resultStr := result.String()
	if resultStr != "All passed" {
		t.Errorf("Errors occurred:\n%s", resultStr)
	}
}

func TestRangeSplitTextMutation(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><div id="test"></div></body>
</html>`)

	binder.BindDocument(doc)

	// Test that range updates when text is split
	result, err := r.Execute(`
		var div = document.getElementById('test');
		div.textContent = 'Ijklmnop';
		
		var textNode = div.firstChild;
		var result = {
			initialLength: textNode.length,
			initialData: textNode.data
		};
		
		var range = document.createRange();
		range.setStart(textNode, 1);
		range.setEnd(textNode, 3);
		
		result.rangeStartBefore = range.startOffset;
		result.rangeEndBefore = range.endOffset;
		result.endContainerBefore = range.endContainer.data;
		
		var newNode = textNode.splitText(1);
		
		result.oldNodeData = textNode.data;
		result.newNodeData = newNode.data;
		result.rangeStartAfter = range.startOffset;
		result.rangeEndAfter = range.endOffset;
		result.endContainerAfter = range.endContainer.data;
		result.endIsNewNode = range.endContainer === newNode;
		result.endIsOldNode = range.endContainer === textNode;
		
		result;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	obj := result.ToObject(r.vm)
	
	t.Logf("Initial length: %v", obj.Get("initialLength"))
	t.Logf("Initial data: %v", obj.Get("initialData"))
	t.Logf("Range before: start=%v, end=%v", obj.Get("rangeStartBefore"), obj.Get("rangeEndBefore"))
	t.Logf("End container before: %v", obj.Get("endContainerBefore"))
	t.Logf("Old node data: %v", obj.Get("oldNodeData"))
	t.Logf("New node data: %v", obj.Get("newNodeData"))
	t.Logf("Range after: start=%v, end=%v", obj.Get("rangeStartAfter"), obj.Get("rangeEndAfter"))
	t.Logf("End container after: %v", obj.Get("endContainerAfter"))
	t.Logf("End is new node: %v", obj.Get("endIsNewNode"))
	t.Logf("End is old node: %v", obj.Get("endIsOldNode"))

	// After split at offset 1:
	// - Old text becomes "I" (1 char)
	// - New text becomes "jklmnop" (7 chars)
	// - Range end offset was 3 which is > split offset 1
	// - So end should move to new node with offset 3-1=2
	
	if obj.Get("oldNodeData").String() != "I" {
		t.Errorf("Expected old node to have 'I', got '%s'", obj.Get("oldNodeData").String())
	}
	if obj.Get("newNodeData").String() != "jklmnop" {
		t.Errorf("Expected new node to have 'jklmnop', got '%s'", obj.Get("newNodeData").String())
	}
	if !obj.Get("endIsNewNode").ToBoolean() {
		t.Errorf("Expected end container to be new node, but endIsNewNode=%v, endIsOldNode=%v",
			obj.Get("endIsNewNode").ToBoolean(), obj.Get("endIsOldNode").ToBoolean())
	}
	if obj.Get("rangeEndAfter").ToInteger() != 2 {
		t.Errorf("Expected end offset to be 2, got %d", obj.Get("rangeEndAfter").ToInteger())
	}
}

func TestRangeSplitTextMutationForeignDoc(t *testing.T) {
	r := NewRuntime()
	binder := NewDOMBinder(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body></body>
</html>`)

	binder.BindDocument(doc)

	// Test that range updates in a foreign document
	result, err := r.Execute(`
		// Create a foreign document
		var foreignDoc = document.implementation.createHTMLDocument("");
		var foreignTextNode = foreignDoc.createTextNode('Ijklmnop');
		foreignDoc.body.appendChild(foreignTextNode);
		
		var result = {
			foreignDocExists: foreignDoc !== null,
			foreignTextNodeParent: foreignTextNode.parentNode !== null
		};
		
		// Create range in the foreign document
		var range = foreignDoc.createRange();
		range.setStart(foreignTextNode, 1);
		range.setEnd(foreignTextNode, 3);
		
		result.rangeCreated = range !== null;
		result.endOffsetBefore = range.endOffset;
		
		// Now split the text
		var newNode = foreignTextNode.splitText(1);
		
		result.oldNodeData = foreignTextNode.data;
		result.newNodeData = newNode.data;
		result.endOffsetAfter = range.endOffset;
		result.endContainerAfter = range.endContainer.data;
		result.endIsNewNode = range.endContainer === newNode;
		
		result;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	obj := result.ToObject(r.vm)
	
	t.Logf("Foreign doc exists: %v", obj.Get("foreignDocExists"))
	t.Logf("Foreign text node has parent: %v", obj.Get("foreignTextNodeParent"))
	t.Logf("Range created: %v", obj.Get("rangeCreated"))
	t.Logf("End offset before: %v", obj.Get("endOffsetBefore"))
	t.Logf("Old node data: %v", obj.Get("oldNodeData"))
	t.Logf("New node data: %v", obj.Get("newNodeData"))
	t.Logf("End offset after: %v", obj.Get("endOffsetAfter"))
	t.Logf("End container after: %v", obj.Get("endContainerAfter"))
	t.Logf("End is new node: %v", obj.Get("endIsNewNode"))
	
	if !obj.Get("endIsNewNode").ToBoolean() {
		t.Error("Expected end container to be the new node")
	}
}
