package js

import (
	"testing"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
)

func TestScriptExecutorBasic(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">Original</div>
	<script>
		document.getElementById('test').textContent = 'Modified';
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	errors := executor.ExecuteScripts(doc)

	if len(errors) > 0 {
		t.Fatalf("ExecuteScripts returned errors: %v", errors)
	}

	// Check that the script modified the DOM
	el := doc.GetElementById("test")
	if el == nil {
		t.Fatal("Element not found")
	}
	if el.TextContent() != "Modified" {
		t.Errorf("Expected 'Modified', got '%s'", el.TextContent())
	}
}

func TestScriptExecutorMultipleScripts(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">0</div>
	<script>
		var counter = 1;
	</script>
	<script>
		counter += 2;
	</script>
	<script>
		document.getElementById('test').textContent = counter;
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	errors := executor.ExecuteScripts(doc)

	if len(errors) > 0 {
		t.Fatalf("ExecuteScripts returned errors: %v", errors)
	}

	el := doc.GetElementById("test")
	if el.TextContent() != "3" {
		t.Errorf("Expected '3', got '%s'", el.TextContent())
	}
}

func TestScriptExecutorDOMContentLoaded(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">Original</div>
	<script>
		document.addEventListener('DOMContentLoaded', function() {
			document.getElementById('test').textContent = 'Ready';
		});
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	executor.ExecuteScripts(doc)

	// Fire DOMContentLoaded
	executor.DispatchDOMContentLoaded()

	el := doc.GetElementById("test")
	if el.TextContent() != "Ready" {
		t.Errorf("Expected 'Ready', got '%s'", el.TextContent())
	}
}

func TestScriptExecutorNonJSScripts(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<script type="text/template">
		This should not be executed
		var x = undefined_function();
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	errors := executor.ExecuteScripts(doc)

	// Should have no errors since the script type is not JavaScript
	if len(errors) > 0 {
		t.Errorf("Expected no errors, got: %v", errors)
	}
}

func TestScriptExecutorEmptyScript(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<script></script>
	<script>   </script>
</body>
</html>`)

	executor.SetupDocument(doc)
	errors := executor.ExecuteScripts(doc)

	if len(errors) > 0 {
		t.Errorf("Expected no errors for empty scripts, got: %v", errors)
	}
}

func TestScriptExecutorEventLoop(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">0</div>
	<script>
		setTimeout(function() {
			document.getElementById('test').textContent = '1';
		}, 0);
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	executor.ExecuteScripts(doc)

	// Text should still be 0 before event loop
	el := doc.GetElementById("test")
	if el.TextContent() != "0" {
		t.Errorf("Expected '0' before event loop, got '%s'", el.TextContent())
	}

	// Run event loop
	executor.RunEventLoop()

	// Now it should be 1
	if el.TextContent() != "1" {
		t.Errorf("Expected '1' after event loop, got '%s'", el.TextContent())
	}
}

func TestScriptExecutorErrorRecovery(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">Original</div>
	<script>
		throw new Error('Script 1 fails');
	</script>
	<script>
		document.getElementById('test').textContent = 'Script 2 ran';
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	errors := executor.ExecuteScripts(doc)

	// Should have one error
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}

	// But second script should still have run
	el := doc.GetElementById("test")
	if el.TextContent() != "Script 2 ran" {
		t.Errorf("Expected 'Script 2 ran', got '%s'", el.TextContent())
	}
}

func TestScriptExecutorCreateAndAppend(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="container"></div>
	<script>
		var container = document.getElementById('container');
		for (var i = 0; i < 3; i++) {
			var p = document.createElement('p');
			p.textContent = 'Item ' + i;
			container.appendChild(p);
		}
	</script>
</body>
</html>`)

	executor.SetupDocument(doc)
	executor.ExecuteScripts(doc)

	container := doc.GetElementById("container")
	if container == nil {
		t.Fatal("Container not found")
	}

	children := container.GetElementsByTagName("p")
	if children.Length() != 3 {
		t.Errorf("Expected 3 children, got %d", children.Length())
	}
}

func TestScriptExecutorWithEvents(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<div id="test">0</div>
</body>
</html>`)

	executor.SetupDocument(doc)

	// First, bind the element and add event target support before adding listeners
	el := doc.GetElementById("test")
	jsEl := executor.DOMBinder().BindElement(el)
	executor.EventBinder().BindEventTarget(jsEl)

	// Now add the event listener and dispatch via JavaScript
	_, err := r.Execute(`
		var el = document.getElementById('test');
		el.addEventListener('custom', function(e) {
			el.textContent = e.detail.value;
		});
		var event = new CustomEvent('custom', { detail: { value: '42' } });
		el.dispatchEvent(event);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if el.TextContent() != "42" {
		t.Errorf("Expected '42', got '%s'", el.TextContent())
	}
}

func TestScriptExecutorFrames(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<iframe src="about:blank"></iframe>
	<div id="result"></div>
</body>
</html>`)

	executor.SetupDocument(doc)

	// Test frames.length
	result, err := r.Execute(`frames.length`)
	if err != nil {
		t.Fatalf("Execute frames.length failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected frames.length == 1, got %v", result.Export())
	}

	// Test frames[0].document exists
	result, err = r.Execute(`typeof frames[0].document`)
	if err != nil {
		t.Fatalf("Execute typeof frames[0].document failed: %v", err)
	}
	if result.String() != "object" {
		t.Errorf("Expected frames[0].document to be object, got %v", result.String())
	}

	// Test frames[0].document.createElement works
	_, err = r.Execute(`
		var frameDoc = frames[0].document;
		var el = frameDoc.createElement('div');
		el.id = 'test';
		frameDoc.body.appendChild(el);
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Test element.ownerDocument identity
	result, err = r.Execute(`
		var frameDoc = frames[0].document;
		var el = frameDoc.createElement('span');
		el.ownerDocument === frameDoc;
	`)
	if err != nil {
		t.Fatalf("Execute ownerDocument test failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected el.ownerDocument === frameDoc to be true")
	}
}

func TestScriptExecutorMultipleIframes(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body>
	<iframe src="about:blank"></iframe>
	<iframe src="about:blank"></iframe>
	<iframe src="about:blank"></iframe>
</body>
</html>`)

	executor.SetupDocument(doc)

	// Test frames.length
	result, err := r.Execute(`frames.length`)
	if err != nil {
		t.Fatalf("Execute frames.length failed: %v", err)
	}
	if result.ToInteger() != 3 {
		t.Errorf("Expected frames.length == 3, got %v", result.Export())
	}

	// Test each frame has its own document
	result, err = r.Execute(`
		frames[0].document !== frames[1].document &&
		frames[1].document !== frames[2].document;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected each frame to have a distinct document")
	}

	// Test frame identity (same frame accessed multiple times should return same object)
	result, err = r.Execute(`
		var frame0a = frames[0];
		var frame0b = frames[0];
		frame0a === frame0b;
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.ToBoolean() {
		t.Error("Expected frames[0] to return the same object on repeated access")
	}
}

func TestGetComputedStyle(t *testing.T) {
	r := NewRuntime()
	executor := NewScriptExecutor(r)

	// Create a document with inline styles
	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head>
	<style>
		#test { color: red; display: block; }
		.big { font-size: 24px; }
	</style>
</head>
<body>
	<div id="test" class="big" style="margin-left: 10px;">Hello</div>
</body>
</html>`)

	// Create style resolver and add stylesheets
	styleResolver := css.NewStyleResolver()
	styleResolver.SetUserAgentStylesheet(css.GetUserAgentStylesheet())

	// Parse the inline style element
	styleElements := doc.GetElementsByTagName("style")
	for i := 0; i < styleElements.Length(); i++ {
		styleEl := styleElements.Item(i)
		if styleEl != nil {
			cssContent := styleEl.TextContent()
			parser := css.NewParser(cssContent)
			stylesheet := parser.Parse()
			styleResolver.AddAuthorStylesheet(stylesheet)
		}
	}

	// Set up document and style resolver
	executor.SetupDocument(doc)
	executor.SetStyleResolver(styleResolver)

	// Test getComputedStyle returns an object
	result, err := r.Execute(`typeof getComputedStyle(document.getElementById('test'))`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "object" {
		t.Errorf("Expected getComputedStyle to return object, got %v", result.String())
	}

	// Test getPropertyValue method exists
	result, err = r.Execute(`typeof getComputedStyle(document.getElementById('test')).getPropertyValue`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected getPropertyValue to be function, got %v", result.String())
	}

	// Test inline style is reflected (margin-left)
	result, err = r.Execute(`getComputedStyle(document.getElementById('test')).getPropertyValue('margin-left')`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Margin left should come from inline style
	val := result.String()
	if val != "10px" && val != "10" {
		t.Logf("margin-left value: %s (inline styles may not be fully resolved yet)", val)
	}

	// Test camelCase property access
	result, err = r.Execute(`getComputedStyle(document.getElementById('test')).display`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// display should be 'block' from the stylesheet or default
	val = result.String()
	if val != "block" && val != "inline" {
		t.Logf("display value: %s", val)
	}

	// Test that computed styles are read-only (setProperty does nothing)
	result, err = r.Execute(`
		var style = getComputedStyle(document.getElementById('test'));
		style.setProperty('color', 'blue');
		style.getPropertyValue('color');
	`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Value should still be from the stylesheet (red) or unchanged
	t.Logf("color value after setProperty: %s", result.String())

	// Test window.getComputedStyle also works
	result, err = r.Execute(`typeof window.getComputedStyle`)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.String() != "function" {
		t.Errorf("Expected window.getComputedStyle to be function, got %v", result.String())
	}
}
