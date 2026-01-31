package dom

import (
	"testing"
)

func TestCSSStyleDeclarationBasic(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")

	sd := el.Style()

	// Initially empty
	if sd.Length() != 0 {
		t.Errorf("Expected length 0, got %d", sd.Length())
	}
	if sd.CSSText() != "" {
		t.Errorf("Expected empty cssText, got %q", sd.CSSText())
	}
}

func TestCSSStyleDeclarationSetProperty(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	// Set a property
	sd.SetProperty("color", "red")

	if sd.Length() != 1 {
		t.Errorf("Expected length 1, got %d", sd.Length())
	}
	if sd.GetPropertyValue("color") != "red" {
		t.Errorf("Expected color 'red', got %q", sd.GetPropertyValue("color"))
	}
	if sd.Item(0) != "color" {
		t.Errorf("Expected item(0) 'color', got %q", sd.Item(0))
	}

	// Check cssText
	if sd.CSSText() != "color: red" {
		t.Errorf("Expected cssText 'color: red', got %q", sd.CSSText())
	}

	// Check attribute synced
	if el.GetAttribute("style") != "color: red" {
		t.Errorf("Expected style attribute 'color: red', got %q", el.GetAttribute("style"))
	}
}

func TestCSSStyleDeclarationCamelCase(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	// Set using camelCase
	sd.SetProperty("backgroundColor", "#fff")

	// Should be stored as kebab-case
	if sd.GetPropertyValue("background-color") != "#fff" {
		t.Errorf("Expected background-color '#fff', got %q", sd.GetPropertyValue("background-color"))
	}

	// camelCase lookup should also work
	if sd.GetPropertyValue("backgroundColor") != "#fff" {
		t.Errorf("Expected backgroundColor '#fff', got %q", sd.GetPropertyValue("backgroundColor"))
	}

	// cssText should use kebab-case
	if sd.CSSText() != "background-color: #fff" {
		t.Errorf("Expected cssText 'background-color: #fff', got %q", sd.CSSText())
	}
}

func TestCSSStyleDeclarationRemoveProperty(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	sd.SetProperty("color", "blue")
	sd.SetProperty("width", "100px")

	if sd.Length() != 2 {
		t.Errorf("Expected length 2, got %d", sd.Length())
	}

	// Remove a property
	oldVal := sd.RemoveProperty("color")
	if oldVal != "blue" {
		t.Errorf("Expected old value 'blue', got %q", oldVal)
	}
	if sd.Length() != 1 {
		t.Errorf("Expected length 1, got %d", sd.Length())
	}
	if sd.GetPropertyValue("color") != "" {
		t.Errorf("Expected empty color, got %q", sd.GetPropertyValue("color"))
	}

	// width should still be there
	if sd.GetPropertyValue("width") != "100px" {
		t.Errorf("Expected width '100px', got %q", sd.GetPropertyValue("width"))
	}
}

func TestCSSStyleDeclarationImportant(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	sd.SetProperty("color", "red", "important")

	if sd.GetPropertyPriority("color") != "important" {
		t.Errorf("Expected priority 'important', got %q", sd.GetPropertyPriority("color"))
	}
	if sd.CSSText() != "color: red !important" {
		t.Errorf("Expected cssText 'color: red !important', got %q", sd.CSSText())
	}
}

func TestCSSStyleDeclarationSetCSSText(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	sd.SetCSSText("color: green; background: yellow !important; font-size: 14px")

	if sd.Length() != 3 {
		t.Errorf("Expected length 3, got %d", sd.Length())
	}
	if sd.GetPropertyValue("color") != "green" {
		t.Errorf("Expected color 'green', got %q", sd.GetPropertyValue("color"))
	}
	if sd.GetPropertyValue("background") != "yellow" {
		t.Errorf("Expected background 'yellow', got %q", sd.GetPropertyValue("background"))
	}
	if sd.GetPropertyPriority("background") != "important" {
		t.Errorf("Expected priority 'important', got %q", sd.GetPropertyPriority("background"))
	}
	if sd.GetPropertyValue("font-size") != "14px" {
		t.Errorf("Expected font-size '14px', got %q", sd.GetPropertyValue("font-size"))
	}
}

func TestCSSStyleDeclarationFromAttribute(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	el.SetAttribute("style", "margin: 10px; padding: 5px")

	sd := el.Style()

	if sd.Length() != 2 {
		t.Errorf("Expected length 2, got %d", sd.Length())
	}
	if sd.GetPropertyValue("margin") != "10px" {
		t.Errorf("Expected margin '10px', got %q", sd.GetPropertyValue("margin"))
	}
	if sd.GetPropertyValue("padding") != "5px" {
		t.Errorf("Expected padding '5px', got %q", sd.GetPropertyValue("padding"))
	}
}

func TestNormalizeCSSPropertyName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"color", "color"},
		{"backgroundColor", "background-color"},
		{"background-color", "background-color"},
		{"marginTop", "margin-top"},
		{"WebkitTransform", "webkit-transform"},
		{"fontSize", "font-size"},
		{"", ""},
	}

	for _, tc := range tests {
		result := normalizeCSSPropertyName(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeCSSPropertyName(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestCamelCasePropertyName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"color", "color"},
		{"background-color", "backgroundColor"},
		{"margin-top", "marginTop"},
		{"-webkit-transform", "webkitTransform"},
		{"font-size", "fontSize"},
		{"", ""},
	}

	for _, tc := range tests {
		result := camelCasePropertyName(tc.input)
		if result != tc.expected {
			t.Errorf("camelCasePropertyName(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestCSSStyleDeclarationMultipleProperties(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	sd.SetProperty("color", "red")
	sd.SetProperty("width", "100px")
	sd.SetProperty("height", "50px")

	// Check order preservation
	if sd.Item(0) != "color" || sd.Item(1) != "width" || sd.Item(2) != "height" {
		t.Errorf("Order not preserved: got %q, %q, %q", sd.Item(0), sd.Item(1), sd.Item(2))
	}

	// Update existing property
	sd.SetProperty("color", "blue")
	if sd.GetPropertyValue("color") != "blue" {
		t.Errorf("Expected color 'blue', got %q", sd.GetPropertyValue("color"))
	}
	// Order should still be preserved (color first)
	if sd.Item(0) != "color" {
		t.Errorf("Expected color still at position 0, got %q", sd.Item(0))
	}
}

func TestCSSStyleDeclarationEmptyValueRemoves(t *testing.T) {
	doc := NewDocument()
	el := doc.CreateElement("div")
	sd := el.Style()

	sd.SetProperty("color", "red")
	sd.SetProperty("color", "") // Should remove the property

	if sd.Length() != 0 {
		t.Errorf("Expected length 0, got %d", sd.Length())
	}
	if sd.GetPropertyValue("color") != "" {
		t.Errorf("Expected empty color, got %q", sd.GetPropertyValue("color"))
	}
}
