package js

import (
	"testing"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestHTMLStyleElementSheet(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<style id="testStyle">
body { color: red; }
.container { width: 100px; }
</style>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	tests := []struct {
		name   string
		script string
		want   interface{}
	}{
		{
			name:   "style.sheet exists",
			script: "document.getElementById('testStyle').sheet !== null",
			want:   true,
		},
		{
			name:   "sheet.cssRules exists",
			script: "document.getElementById('testStyle').sheet.cssRules !== undefined",
			want:   true,
		},
		{
			name:   "sheet.cssRules.length is 2",
			script: "document.getElementById('testStyle').sheet.cssRules.length",
			want:   int64(2),
		},
		{
			name:   "sheet.type is text/css",
			script: "document.getElementById('testStyle').sheet.type",
			want:   "text/css",
		},
		{
			name:   "first rule is CSSStyleRule",
			script: "document.getElementById('testStyle').sheet.cssRules[0].type === 1",
			want:   true,
		},
		{
			name:   "first rule selectorText",
			script: "document.getElementById('testStyle').sheet.cssRules[0].selectorText",
			want:   "body",
		},
		{
			name:   "first rule style.color",
			script: "document.getElementById('testStyle').sheet.cssRules[0].style.color",
			want:   "red",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := runtime.Execute(tt.script)
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			got := val.Export()
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestCSSStyleSheetInsertRule(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<style id="testStyle"></style>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	tests := []struct {
		name   string
		script string
		want   interface{}
	}{
		{
			name:   "insertRule returns index",
			script: "document.getElementById('testStyle').sheet.insertRule('div { color: blue; }', 0)",
			want:   int64(0),
		},
		{
			name:   "cssRules.length is 1 after insert",
			script: "document.getElementById('testStyle').sheet.cssRules.length",
			want:   int64(1),
		},
		{
			name:   "inserted rule selectorText",
			script: "document.getElementById('testStyle').sheet.cssRules[0].selectorText",
			want:   "div",
		},
		{
			name:   "insertRule at index 1",
			script: "document.getElementById('testStyle').sheet.insertRule('span { font-size: 12px; }', 1)",
			want:   int64(1),
		},
		{
			name:   "cssRules.length is 2 after second insert",
			script: "document.getElementById('testStyle').sheet.cssRules.length",
			want:   int64(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := runtime.Execute(tt.script)
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			got := val.Export()
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestCSSStyleSheetDeleteRule(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<style id="testStyle">
body { color: red; }
.container { width: 100px; }
</style>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Initial state
	val, err := runtime.Execute("document.getElementById('testStyle').sheet.cssRules.length")
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.Export().(int64) != 2 {
		t.Fatalf("expected 2 rules initially, got %v", val.Export())
	}

	// Delete first rule
	_, err = runtime.Execute("document.getElementById('testStyle').sheet.deleteRule(0)")
	if err != nil {
		t.Fatalf("deleteRule error: %v", err)
	}

	// Check length after delete
	val, err = runtime.Execute("document.getElementById('testStyle').sheet.cssRules.length")
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.Export().(int64) != 1 {
		t.Errorf("expected 1 rule after delete, got %v", val.Export())
	}

	// Check remaining rule is .container
	val, err = runtime.Execute("document.getElementById('testStyle').sheet.cssRules[0].selectorText")
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.Export().(string) != ".container" {
		t.Errorf("expected '.container' selector, got %v", val.Export())
	}
}

func TestCSSStyleRuleModification(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<style id="testStyle">
div { color: red; }
</style>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(htmlContent)
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)
	executor.SetupDocument(doc)

	// Get initial color
	val, err := runtime.Execute("document.getElementById('testStyle').sheet.cssRules[0].style.color")
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.Export().(string) != "red" {
		t.Errorf("expected 'red', got %v", val.Export())
	}

	// Modify the style
	_, err = runtime.Execute("document.getElementById('testStyle').sheet.cssRules[0].style.setProperty('color', 'blue')")
	if err != nil {
		t.Fatalf("setProperty error: %v", err)
	}

	// Check modified color
	val, err = runtime.Execute("document.getElementById('testStyle').sheet.cssRules[0].style.color")
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.Export().(string) != "blue" {
		t.Errorf("expected 'blue' after modification, got %v", val.Export())
	}
}
