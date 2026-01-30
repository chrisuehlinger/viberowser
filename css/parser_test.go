package css

import (
	"testing"
)

func TestParserBasicStylesheet(t *testing.T) {
	css := `
		body {
			color: black;
		}
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "body" {
		t.Errorf("expected selector 'body', got %q", rule.SelectorText)
	}

	if len(rule.Declarations) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(rule.Declarations))
	}

	decl := rule.Declarations[0]
	if decl.Property != "color" {
		t.Errorf("expected property 'color', got %q", decl.Property)
	}

	if decl.RawValue != "black" {
		t.Errorf("expected value 'black', got %q", decl.RawValue)
	}
}

func TestParserMultipleRules(t *testing.T) {
	css := `
		h1 { font-size: 24px; }
		h2 { font-size: 20px; }
		p { line-height: 1.5; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(stylesheet.Rules))
	}

	expectedSelectors := []string{"h1", "h2", "p"}
	for i, sel := range expectedSelectors {
		if stylesheet.Rules[i].SelectorText != sel {
			t.Errorf("rule %d: expected selector %q, got %q", i, sel, stylesheet.Rules[i].SelectorText)
		}
	}
}

func TestParserMultipleDeclarations(t *testing.T) {
	css := `
		div {
			color: red;
			background: blue;
			margin: 10px;
		}
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	rule := stylesheet.Rules[0]
	if len(rule.Declarations) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(rule.Declarations))
	}

	expectedProps := []string{"color", "background", "margin"}
	for i, prop := range expectedProps {
		if rule.Declarations[i].Property != prop {
			t.Errorf("declaration %d: expected property %q, got %q", i, prop, rule.Declarations[i].Property)
		}
	}
}

func TestParserImportantDeclaration(t *testing.T) {
	css := `
		p { color: red !important; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	decl := stylesheet.Rules[0].Declarations[0]
	if !decl.Important {
		t.Error("expected declaration to be important")
	}
}

func TestParserColorValues(t *testing.T) {
	css := `
		div {
			color: #f00;
			background: #ff0000;
			border-color: #ff000080;
		}
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	rule := stylesheet.Rules[0]
	if len(rule.Declarations) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(rule.Declarations))
	}

	// Check #f00 (short hex)
	if rule.Declarations[0].Value.Color.R != 255 {
		t.Errorf("expected R=255 for #f00, got %d", rule.Declarations[0].Value.Color.R)
	}

	// Check #ff0000 (full hex)
	if rule.Declarations[1].Value.Color.R != 255 || rule.Declarations[1].Value.Color.G != 0 {
		t.Errorf("expected R=255, G=0 for #ff0000")
	}

	// Check alpha
	if rule.Declarations[2].Value.Color.A != 128 {
		t.Errorf("expected A=128 for #ff000080, got %d", rule.Declarations[2].Value.Color.A)
	}
}

func TestParserLengthValues(t *testing.T) {
	css := `
		div {
			width: 100px;
			height: 50%;
			margin: 2em;
		}
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	rule := stylesheet.Rules[0]

	// Check 100px
	if rule.Declarations[0].Value.Length != 100 || rule.Declarations[0].Value.Unit != "px" {
		t.Errorf("expected 100px, got %v%s", rule.Declarations[0].Value.Length, rule.Declarations[0].Value.Unit)
	}

	// Check 50%
	if rule.Declarations[1].Value.Length != 50 || rule.Declarations[1].Value.Unit != "%" {
		t.Errorf("expected 50%%, got %v%s", rule.Declarations[1].Value.Length, rule.Declarations[1].Value.Unit)
	}

	// Check 2em
	if rule.Declarations[2].Value.Length != 2 || rule.Declarations[2].Value.Unit != "em" {
		t.Errorf("expected 2em, got %v%s", rule.Declarations[2].Value.Length, rule.Declarations[2].Value.Unit)
	}
}

func TestParserComplexSelector(t *testing.T) {
	css := `
		div.container#main { color: black; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "div.container#main" {
		t.Errorf("expected selector 'div.container#main', got %q", rule.SelectorText)
	}
}

func TestParserDescendantCombinator(t *testing.T) {
	css := `
		div p { color: black; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	if len(stylesheet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet.Rules))
	}

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "div p" {
		t.Errorf("expected selector 'div p', got %q", rule.SelectorText)
	}
}

func TestParserChildCombinator(t *testing.T) {
	css := `
		ul > li { list-style: none; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "ul > li" {
		t.Errorf("expected selector 'ul > li', got %q", rule.SelectorText)
	}
}

func TestParserSiblingCombinators(t *testing.T) {
	tests := []struct {
		css      string
		expected string
	}{
		{`h1 + p { color: red; }`, "h1 + p"},
		{`h1 ~ p { color: blue; }`, "h1 ~ p"},
	}

	for _, tt := range tests {
		parser := NewParser(tt.css)
		stylesheet := parser.Parse()

		if len(stylesheet.Rules) != 1 {
			t.Fatalf("expected 1 rule for %q", tt.css)
		}

		rule := stylesheet.Rules[0]
		if rule.SelectorText != tt.expected {
			t.Errorf("expected selector %q, got %q", tt.expected, rule.SelectorText)
		}
	}
}

func TestParserSelectorList(t *testing.T) {
	css := `
		h1, h2, h3 { font-weight: bold; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "h1, h2, h3" {
		t.Errorf("expected selector 'h1, h2, h3', got %q", rule.SelectorText)
	}
}

func TestParserAttributeSelector(t *testing.T) {
	tests := []struct {
		css      string
		expected string
	}{
		{`a[href] { color: blue; }`, `a[href]`},
		{`input[type="text"] { border: 1px; }`, `input[type="text"]`},
	}

	for _, tt := range tests {
		parser := NewParser(tt.css)
		stylesheet := parser.Parse()

		if len(stylesheet.Rules) != 1 {
			t.Fatalf("expected 1 rule for %q", tt.css)
		}

		rule := stylesheet.Rules[0]
		if rule.SelectorText != tt.expected {
			t.Errorf("expected selector %q, got %q", tt.expected, rule.SelectorText)
		}
	}
}

func TestParserPseudoClass(t *testing.T) {
	css := `
		a:hover { text-decoration: underline; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "a:hover" {
		t.Errorf("expected selector 'a:hover', got %q", rule.SelectorText)
	}
}

func TestParserPseudoElement(t *testing.T) {
	css := `
		p::first-line { font-weight: bold; }
	`

	parser := NewParser(css)
	stylesheet := parser.Parse()

	rule := stylesheet.Rules[0]
	if rule.SelectorText != "p::first-line" {
		t.Errorf("expected selector 'p::first-line', got %q", rule.SelectorText)
	}
}

func TestParserSpecificity(t *testing.T) {
	tests := []struct {
		selector string
		a, b, c  int
	}{
		{"p", 0, 0, 1},
		{".class", 0, 1, 0},
		{"#id", 1, 0, 0},
		{"p.class", 0, 1, 1},
		{"#id.class", 1, 1, 0},
		{"div p", 0, 0, 2},
		{"div.class p.class", 0, 2, 2},
		{"#id div.class p", 1, 1, 2},
	}

	for _, tt := range tests {
		css := tt.selector + " { color: black; }"
		parser := NewParser(css)
		stylesheet := parser.Parse()

		if len(stylesheet.Rules) != 1 {
			t.Fatalf("expected 1 rule for %q", tt.selector)
		}

		spec := stylesheet.Rules[0].Specificity
		if spec.A != tt.a || spec.B != tt.b || spec.C != tt.c {
			t.Errorf("selector %q: expected specificity (%d,%d,%d), got (%d,%d,%d)",
				tt.selector, tt.a, tt.b, tt.c, spec.A, spec.B, spec.C)
		}
	}
}

func TestCSSParserStylesheet(t *testing.T) {
	css := `
		/* Comment */
		@import url("styles.css");

		body {
			margin: 0;
			padding: 0;
		}

		.container {
			max-width: 1200px;
		}
	`

	parser := NewCSSParser(css)
	stylesheet := parser.ParseStylesheet()

	if len(stylesheet.Rules) < 2 {
		t.Fatalf("expected at least 2 rules, got %d", len(stylesheet.Rules))
	}

	// First rule should be an at-rule (@import)
	atRule, ok := stylesheet.Rules[0].(*AtRule)
	if !ok {
		t.Fatalf("expected first rule to be AtRule")
	}
	if atRule.Name != "import" {
		t.Errorf("expected @import, got @%s", atRule.Name)
	}

	// Second rule should be a qualified rule (body)
	qRule, ok := stylesheet.Rules[1].(*QualifiedRule)
	if !ok {
		t.Fatalf("expected second rule to be QualifiedRule")
	}
	if qRule.Block == nil {
		t.Error("expected qualified rule to have a block")
	}
}

func TestCSSParserDeclarationList(t *testing.T) {
	css := `color: red; background: blue; font-size: 16px`

	parser := NewCSSParser(css)
	declarations := parser.ParseDeclarationList()

	if len(declarations) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(declarations))
	}

	expectedProps := []string{"color", "background", "font-size"}
	for i, prop := range expectedProps {
		if declarations[i].Property != prop {
			t.Errorf("declaration %d: expected %q, got %q", i, prop, declarations[i].Property)
		}
	}
}
