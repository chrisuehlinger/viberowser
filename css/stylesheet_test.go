package css

import (
	"testing"
)

func TestNewCSSStyleSheet(t *testing.T) {
	cssText := `
		body { color: red; }
		.container { width: 100px; }
	`

	sheet := NewCSSStyleSheet(cssText, nil)

	if sheet == nil {
		t.Fatal("expected non-nil stylesheet")
	}

	if sheet.Type() != "text/css" {
		t.Errorf("expected type 'text/css', got %q", sheet.Type())
	}

	if sheet.CSSRules().Length() != 2 {
		t.Errorf("expected 2 rules, got %d", sheet.CSSRules().Length())
	}
}

func TestCSSStyleSheetInsertRule(t *testing.T) {
	sheet := NewCSSStyleSheet("", nil)

	idx, err := sheet.InsertRule("div { color: blue; }", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	if sheet.CSSRules().Length() != 1 {
		t.Errorf("expected 1 rule, got %d", sheet.CSSRules().Length())
	}

	rule := sheet.CSSRules().Item(0)
	if rule == nil {
		t.Fatal("expected rule at index 0")
	}

	if rule.Type() != StyleRule {
		t.Errorf("expected StyleRule type, got %d", rule.Type())
	}

	styleRule, ok := rule.(*CSSStyleRule)
	if !ok {
		t.Fatal("expected *CSSStyleRule")
	}

	if styleRule.SelectorText() != "div" {
		t.Errorf("expected selector 'div', got %q", styleRule.SelectorText())
	}
}

func TestCSSStyleSheetDeleteRule(t *testing.T) {
	cssText := `
		body { color: red; }
		.container { width: 100px; }
	`

	sheet := NewCSSStyleSheet(cssText, nil)

	if sheet.CSSRules().Length() != 2 {
		t.Fatalf("expected 2 rules initially, got %d", sheet.CSSRules().Length())
	}

	err := sheet.DeleteRule(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sheet.CSSRules().Length() != 1 {
		t.Errorf("expected 1 rule after deletion, got %d", sheet.CSSRules().Length())
	}

	// The remaining rule should be .container
	rule := sheet.CSSRules().Item(0).(*CSSStyleRule)
	if rule.SelectorText() != ".container" {
		t.Errorf("expected selector '.container', got %q", rule.SelectorText())
	}
}

func TestCSSStyleRuleStyle(t *testing.T) {
	cssText := `div { color: red; background: blue; }`

	sheet := NewCSSStyleSheet(cssText, nil)

	rule := sheet.CSSRules().Item(0).(*CSSStyleRule)
	style := rule.Style()

	if style.GetPropertyValue("color") != "red" {
		t.Errorf("expected color 'red', got %q", style.GetPropertyValue("color"))
	}

	if style.GetPropertyValue("background") != "blue" {
		t.Errorf("expected background 'blue', got %q", style.GetPropertyValue("background"))
	}

	// Test setting a property
	style.SetProperty("margin", "10px")
	if style.GetPropertyValue("margin") != "10px" {
		t.Errorf("expected margin '10px', got %q", style.GetPropertyValue("margin"))
	}
}

func TestCSSKeyframesRule(t *testing.T) {
	cssText := `
		@keyframes fadeIn {
			from { opacity: 0; }
			to { opacity: 1; }
		}
	`

	sheet := NewCSSStyleSheet(cssText, nil)

	if sheet.CSSRules().Length() != 1 {
		t.Fatalf("expected 1 rule, got %d", sheet.CSSRules().Length())
	}

	rule := sheet.CSSRules().Item(0)
	if rule.Type() != KeyframesRule {
		t.Fatalf("expected KeyframesRule type, got %d", rule.Type())
	}

	keyframesRule := rule.(*CSSKeyframesRule)
	if keyframesRule.Name() != "fadeIn" {
		t.Errorf("expected name 'fadeIn', got %q", keyframesRule.Name())
	}

	cssRules := keyframesRule.CSSRules()
	if cssRules.Length() != 2 {
		t.Errorf("expected 2 keyframe rules, got %d", cssRules.Length())
	}
}

func TestCSSMediaRule(t *testing.T) {
	cssText := `
		@media screen and (max-width: 600px) {
			body { font-size: 14px; }
		}
	`

	sheet := NewCSSStyleSheet(cssText, nil)

	if sheet.CSSRules().Length() != 1 {
		t.Fatalf("expected 1 rule, got %d", sheet.CSSRules().Length())
	}

	rule := sheet.CSSRules().Item(0)
	if rule.Type() != MediaRule {
		t.Fatalf("expected MediaRule type, got %d", rule.Type())
	}

	mediaRule := rule.(*CSSMediaRule)
	if mediaRule.Media().MediaText() != "screen and (max-width: 600px)" {
		t.Errorf("expected media text 'screen and (max-width: 600px)', got %q", mediaRule.Media().MediaText())
	}

	if mediaRule.CSSRules().Length() != 1 {
		t.Errorf("expected 1 nested rule, got %d", mediaRule.CSSRules().Length())
	}
}

func TestCSSRuleText(t *testing.T) {
	cssText := `div { color: red; margin: 10px; }`

	sheet := NewCSSStyleSheet(cssText, nil)
	rule := sheet.CSSRules().Item(0)

	text := rule.CSSText()
	// The exact format may vary, but it should contain the selector and properties
	if text == "" {
		t.Error("expected non-empty cssText")
	}

	t.Logf("CSSText: %s", text)
}

func TestCSSStyleSheetDisabled(t *testing.T) {
	sheet := NewCSSStyleSheet("body { color: red; }", nil)

	if sheet.Disabled() {
		t.Error("expected sheet to not be disabled initially")
	}

	sheet.SetDisabled(true)
	if !sheet.Disabled() {
		t.Error("expected sheet to be disabled after SetDisabled(true)")
	}

	sheet.SetDisabled(false)
	if sheet.Disabled() {
		t.Error("expected sheet to not be disabled after SetDisabled(false)")
	}
}

func TestMediaList(t *testing.T) {
	ml := NewMediaList("screen, print")

	if ml.MediaText() != "screen, print" {
		t.Errorf("expected 'screen, print', got %q", ml.MediaText())
	}

	if ml.Length() != 2 {
		t.Errorf("expected length 2, got %d", ml.Length())
	}

	if ml.Item(0) != "screen" {
		t.Errorf("expected item(0) 'screen', got %q", ml.Item(0))
	}

	if ml.Item(1) != "print" {
		t.Errorf("expected item(1) 'print', got %q", ml.Item(1))
	}

	ml.AppendMedium("tv")
	if ml.Length() != 3 {
		t.Errorf("expected length 3 after append, got %d", ml.Length())
	}

	ml.DeleteMedium("print")
	if ml.Length() != 2 {
		t.Errorf("expected length 2 after delete, got %d", ml.Length())
	}
}
