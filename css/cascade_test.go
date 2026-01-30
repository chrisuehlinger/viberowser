package css

import (
	"strings"
	"testing"

	"github.com/AYColumbia/viberowser/dom"
)

func TestCascadeSpecificityCalculation(t *testing.T) {
	tests := []struct {
		selector    string
		expectedA   int
		expectedB   int
		expectedC   int
	}{
		{"*", 0, 0, 0},
		{"p", 0, 0, 1},
		{"div p", 0, 0, 2},
		{".class", 0, 1, 0},
		{"p.class", 0, 1, 1},
		{"#id", 1, 0, 0},
		{"#id.class", 1, 1, 0},
		{"#id .class p", 1, 1, 1},
		{"p[attr]", 0, 1, 1},
		{"p:first-child", 0, 1, 1},
		{"p::before", 0, 0, 2},
		{"#a #b .c .d p span", 2, 2, 2},
	}

	for _, tt := range tests {
		sel, err := ParseSelector(tt.selector)
		if err != nil {
			t.Errorf("ParseSelector(%q) error: %v", tt.selector, err)
			continue
		}

		spec := sel.CalculateSpecificity()
		if spec.A != tt.expectedA || spec.B != tt.expectedB || spec.C != tt.expectedC {
			t.Errorf("Specificity(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tt.selector, spec.A, spec.B, spec.C, tt.expectedA, tt.expectedB, tt.expectedC)
		}
	}
}

func TestSpecificityComparison(t *testing.T) {
	tests := []struct {
		sel1     string
		sel2     string
		expected int // -1: sel1 < sel2, 0: equal, 1: sel1 > sel2
	}{
		{"p", "p", 0},
		{"p", ".class", -1},
		{".class", "#id", -1},
		{"#id", "#id.class", -1},
		{"p p p", ".class", -1},
		{"#id", "p p p p p p p p p p p", 1},
	}

	for _, tt := range tests {
		sel1, _ := ParseSelector(tt.sel1)
		sel2, _ := ParseSelector(tt.sel2)

		spec1 := sel1.CalculateSpecificity()
		spec2 := sel2.CalculateSpecificity()

		cmp := spec1.Compare(spec2)
		if cmp != tt.expected {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.sel1, tt.sel2, cmp, tt.expected)
		}
	}
}

func TestCascadeOriginOrder(t *testing.T) {
	// Test that cascade layers are correctly ordered
	tests := []struct {
		origin1    CascadeOrigin
		important1 bool
		origin2    CascadeOrigin
		important2 bool
		expected   bool // true if origin1 should come before (lower precedence) than origin2
	}{
		// Normal declarations: UA < User < Author
		{OriginUserAgent, false, OriginUser, false, true},
		{OriginUser, false, OriginAuthor, false, true},
		{OriginUserAgent, false, OriginAuthor, false, true},

		// Important declarations: Author < User < UA (inverted)
		{OriginAuthor, true, OriginUser, true, true},
		{OriginUser, true, OriginUserAgent, true, true},
		{OriginAuthor, true, OriginUserAgent, true, true},

		// Normal < Important
		{OriginAuthor, false, OriginAuthor, true, true},
		{OriginAuthor, false, OriginUser, true, true},
	}

	for _, tt := range tests {
		layer1 := cascadeLayer(tt.origin1, tt.important1)
		layer2 := cascadeLayer(tt.origin2, tt.important2)

		result := layer1 < layer2
		if result != tt.expected {
			t.Errorf("cascadeLayer(%v, %v) < cascadeLayer(%v, %v) = %v, want %v",
				tt.origin1, tt.important1, tt.origin2, tt.important2, result, tt.expected)
		}
	}
}

func TestStyleResolver(t *testing.T) {
	// Create a simple document
	doc := createTestDocumentFromHTML("<html><body><div class='test' id='main'>Hello</div></body></html>")

	resolver := NewStyleResolver()

	// Add author stylesheet
	css := `
		div { color: red; }
		.test { color: blue; }
		#main { color: green; }
	`
	parser := NewParser(css)
	resolver.AddAuthorStylesheet(parser.Parse())

	// Get the div element
	divs := doc.GetElementsByTagName("div")
	if divs.Length() == 0 {
		t.Fatal("No div element found")
	}
	div := divs.Item(0)

	// Resolve styles
	style := resolver.ResolveStyles(div, nil)

	// Check that the ID selector (highest specificity) wins
	colorVal := style.GetPropertyValue("color")
	if colorVal == nil {
		t.Fatal("color property not found")
	}
	// The value should be "green" from #main
	if colorVal.Keyword != "green" {
		t.Errorf("color = %q, want %q", colorVal.Keyword, "green")
	}
}

func TestImportantDeclarations(t *testing.T) {
	doc := createTestDocumentFromHTML("<html><body><div class='test'>Hello</div></body></html>")

	resolver := NewStyleResolver()

	// Add stylesheet with !important
	css := `
		.test { color: blue !important; }
		div { color: red; }
	`
	parser := NewParser(css)
	resolver.AddAuthorStylesheet(parser.Parse())

	divs := doc.GetElementsByTagName("div")
	div := divs.Item(0)

	style := resolver.ResolveStyles(div, nil)
	colorVal := style.GetPropertyValue("color")

	// !important should win even though it has lower specificity
	if colorVal == nil || colorVal.Keyword != "blue" {
		t.Errorf("color = %v, want blue (with !important)", colorVal)
	}
}

func TestInheritedProperties(t *testing.T) {
	doc := createTestDocumentFromHTML("<html><body><div><span>Hello</span></div></body></html>")

	resolver := NewStyleResolver()

	css := `
		div { color: red; font-size: 20px; }
	`
	parser := NewParser(css)
	resolver.AddAuthorStylesheet(parser.Parse())

	// Get parent and child
	divs := doc.GetElementsByTagName("div")
	div := divs.Item(0)
	spans := doc.GetElementsByTagName("span")
	span := spans.Item(0)

	// Resolve parent style first
	parentStyle := resolver.ResolveStyles(div, nil)

	// Resolve child style with parent
	childStyle := resolver.ResolveStyles(span, parentStyle)

	// Color should be inherited
	colorVal := childStyle.GetPropertyValue("color")
	if colorVal == nil || colorVal.Keyword != "red" {
		t.Errorf("span color = %v, want red (inherited)", colorVal)
	}
}

func TestCSSWideKeywords(t *testing.T) {
	doc := createTestDocumentFromHTML("<html><body><div><span>Hello</span></div></body></html>")

	resolver := NewStyleResolver()

	css := `
		div { color: red; display: block; }
		span { color: inherit; display: initial; }
	`
	parser := NewParser(css)
	resolver.AddAuthorStylesheet(parser.Parse())

	divs := doc.GetElementsByTagName("div")
	div := divs.Item(0)
	spans := doc.GetElementsByTagName("span")
	span := spans.Item(0)

	parentStyle := resolver.ResolveStyles(div, nil)
	childStyle := resolver.ResolveStyles(span, parentStyle)

	// color: inherit should get parent's color
	colorVal := childStyle.GetPropertyValue("color")
	if colorVal == nil || colorVal.Keyword != "red" {
		t.Errorf("span color with inherit = %v, want red", colorVal)
	}

	// display: initial should be inline (not block from parent)
	displayVal := childStyle.GetPropertyValue("display")
	if displayVal == nil || displayVal.Keyword != "inline" {
		t.Errorf("span display with initial = %v, want inline", displayVal)
	}
}

func TestInlineStyles(t *testing.T) {
	doc := createTestDocumentFromHTML(`<html><body><div style="color: purple; font-size: 24px;">Hello</div></body></html>`)

	resolver := NewStyleResolver()

	// Add stylesheet
	css := `
		div { color: red; }
	`
	parser := NewParser(css)
	resolver.AddAuthorStylesheet(parser.Parse())

	divs := doc.GetElementsByTagName("div")
	div := divs.Item(0)

	style := resolver.ResolveStyles(div, nil)

	// Inline style should override stylesheet
	colorVal := style.GetPropertyValue("color")
	if colorVal == nil || colorVal.Keyword != "purple" {
		t.Errorf("color = %v, want purple (from inline style)", colorVal)
	}
}

func TestLengthUnits(t *testing.T) {
	tests := []struct {
		value    float64
		unit     string
		expected float64
	}{
		{16, "px", 16},
		{1, "em", 16},   // 1em = 16px (default font size)
		{1, "rem", 16},  // 1rem = 16px (root font size)
		{12, "pt", 16},  // 12pt ≈ 16px
		{1, "in", 96},   // 1in = 96px
		{2.54, "cm", 96}, // 2.54cm = 96px
	}

	for _, tt := range tests {
		result := resolveLength(tt.value, tt.unit, 16, 16)
		// Allow small floating point differences
		if diff := result - tt.expected; diff > 0.1 || diff < -0.1 {
			t.Errorf("resolveLength(%v, %q) = %v, want %v", tt.value, tt.unit, result, tt.expected)
		}
	}
}

func TestUserAgentStylesheet(t *testing.T) {
	ua := GetUserAgentStylesheet()

	if ua == nil {
		t.Fatal("User agent stylesheet is nil")
	}

	if len(ua.Rules) == 0 {
		t.Fatal("User agent stylesheet has no rules")
	}

	// Check that some expected rules exist
	foundDiv := false
	foundBody := false
	for _, rule := range ua.Rules {
		if containsSelector(rule.SelectorText, "div") {
			foundDiv = true
		}
		if containsSelector(rule.SelectorText, "body") {
			foundBody = true
		}
	}

	if !foundDiv {
		t.Error("User agent stylesheet missing div rule")
	}
	if !foundBody {
		t.Error("User agent stylesheet missing body rule")
	}
}

func TestStyleTree(t *testing.T) {
	doc := createTestDocumentFromHTML(`
		<html>
		<head>
			<style>
				div { color: red; display: block; }
				span { color: blue; }
			</style>
		</head>
		<body>
			<div><span>Hello</span></div>
		</body>
		</html>
	`)

	st := NewStyleTree()
	root := st.BuildStyleTree(doc)

	if root == nil {
		t.Fatal("Style tree root is nil")
	}

	// Find the div styled node
	divNode := findStyledNodeByTag(root, "div")
	if divNode == nil {
		t.Fatal("Could not find div in style tree")
	}

	// Check div style
	if divNode.Style == nil {
		t.Fatal("div has no computed style")
	}

	displayVal := divNode.Style.GetPropertyValue("display")
	if displayVal == nil || displayVal.Keyword != "block" {
		t.Errorf("div display = %v, want block", displayVal)
	}

	// Check that div is considered a block
	if !divNode.IsBlock() {
		t.Error("div should be a block element")
	}
}

func TestPropertyInheritance(t *testing.T) {
	// Test which properties are inherited
	inheritedProps := []string{"color", "font-family", "font-size", "line-height", "text-align"}
	nonInheritedProps := []string{"display", "margin", "padding", "border", "width", "height"}

	for _, prop := range inheritedProps {
		def, ok := PropertyDefaults[prop]
		if !ok {
			t.Errorf("Property %q not found in defaults", prop)
			continue
		}
		if !def.Inherited {
			t.Errorf("Property %q should be inherited", prop)
		}
	}

	for _, prop := range nonInheritedProps {
		def, ok := PropertyDefaults[prop]
		if !ok {
			t.Errorf("Property %q not found in defaults", prop)
			continue
		}
		if def.Inherited {
			t.Errorf("Property %q should not be inherited", prop)
		}
	}
}

func TestColorParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Color
		ok       bool
	}{
		{"red", Color{R: 255, G: 0, B: 0, A: 255}, true},
		{"blue", Color{R: 0, G: 0, B: 255, A: 255}, true},
		{"transparent", Color{R: 0, G: 0, B: 0, A: 0}, true},
		{"#fff", Color{R: 255, G: 255, B: 255, A: 255}, true},
		{"#ff0000", Color{R: 255, G: 0, B: 0, A: 255}, true},
		{"#00ff00ff", Color{R: 0, G: 255, B: 0, A: 255}, true},
		{"invalidcolor", Color{}, false},
	}

	for _, tt := range tests {
		color, ok := ParseColor(tt.input)
		if ok != tt.ok {
			t.Errorf("ParseColor(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if ok && color != tt.expected {
			t.Errorf("ParseColor(%q) = %+v, want %+v", tt.input, color, tt.expected)
		}
	}
}

// Helper functions

func createTestDocumentFromHTML(htmlStr string) *dom.Document {
	doc, err := dom.ParseHTML(htmlStr)
	if err != nil {
		panic("Failed to parse HTML: " + err.Error())
	}
	return doc
}

func containsSelector(selectorText, selector string) bool {
	// Check if selector appears anywhere in the selector text
	// Either as the full selector, or as part of a comma-separated list
	if selectorText == selector {
		return true
	}
	// Check for selector in comma-separated list
	for _, part := range strings.Split(selectorText, ",") {
		part = strings.TrimSpace(part)
		if part == selector {
			return true
		}
		// Check for selector at start (like "div p" containing "div")
		if len(part) > len(selector) && part[:len(selector)] == selector {
			next := part[len(selector)]
			if next == ' ' || next == '.' || next == '#' || next == '[' || next == ':' || next == ',' {
				return true
			}
		}
	}
	return false
}

func findStyledNodeByTag(node *StyledNode, tag string) *StyledNode {
	if node.Node != nil && node.Node.NodeType() == dom.ElementNode {
		el := (*dom.Element)(node.Node)
		if el.LocalName() == tag {
			return node
		}
	}
	for _, child := range node.Children {
		if found := findStyledNodeByTag(child, tag); found != nil {
			return found
		}
	}
	return nil
}
