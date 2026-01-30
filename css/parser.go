// Package css provides CSS parsing functionality following CSS Syntax Module Level 3.
// Reference: https://www.w3.org/TR/css-syntax-3/
package css

import (
	"strings"
)

// Stylesheet represents a parsed CSS stylesheet with high-level API.
type Stylesheet struct {
	Rules []Rule
}

// Rule represents a CSS style rule (qualified rule with selector and declarations).
type Rule struct {
	Selectors    []Selector
	Declarations []Declaration
	SelectorText string
	Specificity  Specificity
}

// Selector represents a CSS selector (legacy API - use CSSSelector for full support).
type Selector struct {
	Type       SelectorType
	TagName    string
	ID         string
	Classes    []string
	Attributes []AttributeSelector
	Combinator string
	Next       *Selector
}

// AttributeSelector represents an attribute selector in legacy API.
type AttributeSelector struct {
	Name     string
	Operator string
	Value    string
}

// SelectorType represents the type of selector.
type SelectorType int

const (
	SimpleSelector SelectorType = iota
	ClassSelector
	IDSelector
	UniversalSelector
	AttributeSelectorType
	PseudoClassSelectorType
	PseudoElementSelectorType
)

// Declaration represents a CSS property declaration.
type Declaration struct {
	Property  string
	Value     Value
	Important bool
	RawValue  string
}

// Value represents a CSS value.
type Value struct {
	Type    ValueType
	Keyword string
	Length  float64
	Unit    string
	Color   Color
	Values  []Value // For multi-value properties
	Raw     string  // Raw string representation
}

// ValueType represents the type of CSS value.
type ValueType int

const (
	KeywordValue ValueType = iota
	LengthValue
	ColorValue
	PercentageValue
	NumberValue
	StringValue
	URLValue
	FunctionValue
	ListValue
)

// Color represents a CSS color value.
type Color struct {
	R, G, B, A uint8
}

// Parser handles CSS parsing with legacy API.
type Parser struct {
	input  string
	pos    int
	parser *CSSParser
}

// NewParser creates a new CSS parser for the given input.
func NewParser(input string) *Parser {
	return &Parser{
		input:  input,
		pos:    0,
		parser: NewCSSParser(input),
	}
}

// Parse parses the CSS input and returns a stylesheet.
func (p *Parser) Parse() *Stylesheet {
	parsed := p.parser.ParseStylesheet()
	return convertParsedStylesheet(parsed)
}

// convertParsedStylesheet converts a ParsedStylesheet to the legacy Stylesheet format.
func convertParsedStylesheet(parsed *ParsedStylesheet) *Stylesheet {
	ss := &Stylesheet{}

	for _, cssRule := range parsed.Rules {
		switch r := cssRule.(type) {
		case *QualifiedRule:
			rule := convertQualifiedRule(r)
			if rule != nil {
				ss.Rules = append(ss.Rules, *rule)
			}
		case *AtRule:
			// Handle at-rules (like @media, @import, etc.)
			// For now, skip them as we focus on style rules
		}
	}

	return ss
}

// writeComponentValue writes component values to a string builder for selector text.
func writeComponentValue(sb *strings.Builder, cvs []ComponentValue) {
	for _, cv := range cvs {
		switch v := cv.(type) {
		case PreservedToken:
			switch v.Token.Type {
			case TokenIdent:
				sb.WriteString(v.Token.Value)
			case TokenHash:
				sb.WriteString("#")
				sb.WriteString(v.Token.Value)
			case TokenDelim:
				sb.WriteRune(v.Token.Delim)
			case TokenWhitespace:
				sb.WriteString(" ")
			case TokenColon:
				sb.WriteString(":")
			case TokenOpenSquare:
				sb.WriteString("[")
			case TokenCloseSquare:
				sb.WriteString("]")
			case TokenOpenParen:
				sb.WriteString("(")
			case TokenCloseParen:
				sb.WriteString(")")
			case TokenString:
				sb.WriteString("\"")
				sb.WriteString(v.Token.Value)
				sb.WriteString("\"")
			case TokenComma:
				sb.WriteString(",")
			case TokenNumber:
				sb.WriteString(v.Token.Value)
			case TokenDimension:
				sb.WriteString(v.Token.Value)
				sb.WriteString(v.Token.Unit)
			case TokenPercentage:
				sb.WriteString(v.Token.Value)
				sb.WriteString("%")
			}
		case *Block:
			// Write opening bracket
			switch v.Token.Type {
			case TokenOpenSquare:
				sb.WriteString("[")
			case TokenOpenParen:
				sb.WriteString("(")
			case TokenOpenCurly:
				sb.WriteString("{")
			}
			// Write contents
			writeComponentValue(sb, v.Values)
			// Write closing bracket
			switch v.Token.Type {
			case TokenOpenSquare:
				sb.WriteString("]")
			case TokenOpenParen:
				sb.WriteString(")")
			case TokenOpenCurly:
				sb.WriteString("}")
			}
		case *Function:
			sb.WriteString(v.Name)
			sb.WriteString("(")
			writeComponentValue(sb, v.Values)
			sb.WriteString(")")
		}
	}
}

// convertQualifiedRule converts a QualifiedRule to the legacy Rule format.
func convertQualifiedRule(qr *QualifiedRule) *Rule {
	if qr == nil || qr.Block == nil {
		return nil
	}

	rule := &Rule{}

	// Convert prelude to selector text
	var selectorText strings.Builder
	writeComponentValue(&selectorText, qr.Prelude)
	rule.SelectorText = strings.TrimSpace(selectorText.String())

	// Parse selectors
	cssSelector, err := ParseSelector(rule.SelectorText)
	if err == nil && cssSelector != nil {
		rule.Specificity = cssSelector.CalculateSpecificity()
		rule.Selectors = convertCSSSelector(cssSelector)
	}

	// Parse declarations
	declarations := ParseBlockContents(qr.Block)
	for _, decl := range declarations {
		rule.Declarations = append(rule.Declarations, convertDeclaration(decl))
	}

	return rule
}

// convertCSSSelector converts a CSSSelector to legacy Selector format.
func convertCSSSelector(sel *CSSSelector) []Selector {
	var selectors []Selector

	for _, cs := range sel.ComplexSelectors {
		if len(cs.Compounds) > 0 {
			selector := convertComplexSelector(cs)
			selectors = append(selectors, selector)
		}
	}

	return selectors
}

// convertComplexSelector converts a ComplexSelector to legacy format.
func convertComplexSelector(cs *ComplexSelector) Selector {
	if len(cs.Compounds) == 0 {
		return Selector{}
	}

	var first *Selector
	var current *Selector

	for i, compound := range cs.Compounds {
		sel := convertCompoundSelector(compound)

		if i < len(cs.Compounds)-1 {
			switch compound.Combinator {
			case CombinatorDescendant:
				sel.Combinator = " "
			case CombinatorChild:
				sel.Combinator = ">"
			case CombinatorNextSibling:
				sel.Combinator = "+"
			case CombinatorSubsequentSibling:
				sel.Combinator = "~"
			}
		}

		if first == nil {
			first = &sel
			current = first
		} else {
			current.Next = &sel
			current = current.Next
		}
	}

	if first != nil {
		return *first
	}
	return Selector{}
}

// convertCompoundSelector converts a CompoundSelector to legacy format.
func convertCompoundSelector(compound *CompoundSelector) Selector {
	sel := Selector{}

	if compound.TypeSelector != nil {
		if compound.TypeSelector.Name == "*" {
			sel.Type = UniversalSelector
		} else {
			sel.Type = SimpleSelector
			sel.TagName = compound.TypeSelector.Name
		}
	}

	if len(compound.IDSelectors) > 0 {
		sel.Type = IDSelector
		sel.ID = compound.IDSelectors[0]
	}

	sel.Classes = compound.ClassSelectors

	for _, attr := range compound.AttributeMatchers {
		attrSel := AttributeSelector{
			Name:  attr.Name,
			Value: attr.Value,
		}
		switch attr.Operator {
		case AttrExists:
			attrSel.Operator = ""
		case AttrEquals:
			attrSel.Operator = "="
		case AttrIncludes:
			attrSel.Operator = "~="
		case AttrDashMatch:
			attrSel.Operator = "|="
		case AttrPrefix:
			attrSel.Operator = "^="
		case AttrSuffix:
			attrSel.Operator = "$="
		case AttrSubstring:
			attrSel.Operator = "*="
		}
		sel.Attributes = append(sel.Attributes, attrSel)
	}

	return sel
}

// convertDeclaration converts a CSSDeclaration to legacy Declaration format.
func convertDeclaration(decl *CSSDeclaration) Declaration {
	d := Declaration{
		Property:  decl.Property,
		Important: decl.Important,
	}

	// Build raw value string
	var rawValue strings.Builder
	for _, cv := range decl.Value {
		switch v := cv.(type) {
		case PreservedToken:
			switch v.Token.Type {
			case TokenIdent:
				rawValue.WriteString(v.Token.Value)
			case TokenNumber:
				rawValue.WriteString(v.Token.Value)
			case TokenPercentage:
				rawValue.WriteString(v.Token.Value)
				rawValue.WriteString("%")
			case TokenDimension:
				rawValue.WriteString(v.Token.Value)
				rawValue.WriteString(v.Token.Unit)
			case TokenString:
				rawValue.WriteString("\"")
				rawValue.WriteString(v.Token.Value)
				rawValue.WriteString("\"")
			case TokenHash:
				rawValue.WriteString("#")
				rawValue.WriteString(v.Token.Value)
			case TokenWhitespace:
				rawValue.WriteString(" ")
			case TokenDelim:
				rawValue.WriteRune(v.Token.Delim)
			case TokenComma:
				rawValue.WriteString(",")
			case TokenURL:
				rawValue.WriteString("url(")
				rawValue.WriteString(v.Token.Value)
				rawValue.WriteString(")")
			}
		case *Function:
			rawValue.WriteString(v.Name)
			rawValue.WriteString("(")
			// Function arguments would go here
			rawValue.WriteString(")")
		}
	}
	d.RawValue = strings.TrimSpace(rawValue.String())

	// Parse value
	d.Value = parseValue(decl.Value)

	return d
}

// parseValue parses component values into a Value.
func parseValue(cvs []ComponentValue) Value {
	if len(cvs) == 0 {
		return Value{}
	}

	// Simple case: single token
	if len(cvs) == 1 {
		switch v := cvs[0].(type) {
		case PreservedToken:
			return parseTokenValue(v.Token)
		case *Function:
			return parseFunctionValue(v)
		}
	}

	// Multiple values
	var values []Value
	for _, cv := range cvs {
		switch v := cv.(type) {
		case PreservedToken:
			if v.Token.Type != TokenWhitespace {
				values = append(values, parseTokenValue(v.Token))
			}
		case *Function:
			values = append(values, parseFunctionValue(v))
		}
	}

	if len(values) == 1 {
		return values[0]
	}

	return Value{
		Type:   ListValue,
		Values: values,
	}
}

// parseTokenValue parses a single token into a Value.
func parseTokenValue(tok Token) Value {
	switch tok.Type {
	case TokenIdent:
		return Value{
			Type:    KeywordValue,
			Keyword: tok.Value,
			Raw:     tok.Value,
		}
	case TokenNumber:
		return Value{
			Type:   NumberValue,
			Length: tok.NumValue,
			Raw:    tok.Value,
		}
	case TokenPercentage:
		return Value{
			Type:   PercentageValue,
			Length: tok.NumValue,
			Unit:   "%",
			Raw:    tok.Value + "%",
		}
	case TokenDimension:
		return Value{
			Type:   LengthValue,
			Length: tok.NumValue,
			Unit:   tok.Unit,
			Raw:    tok.Value + tok.Unit,
		}
	case TokenString:
		return Value{
			Type: StringValue,
			Raw:  tok.Value,
		}
	case TokenHash:
		color := parseHashColor(tok.Value)
		return Value{
			Type:  ColorValue,
			Color: color,
			Raw:   "#" + tok.Value,
		}
	case TokenURL:
		return Value{
			Type: URLValue,
			Raw:  tok.Value,
		}
	default:
		return Value{
			Raw: tok.Value,
		}
	}
}

// parseHashColor parses a hex color string.
func parseHashColor(hex string) Color {
	var r, g, b, a uint8 = 0, 0, 0, 255

	switch len(hex) {
	case 3: // #RGB
		r = parseHexDigit(hex[0]) * 17
		g = parseHexDigit(hex[1]) * 17
		b = parseHexDigit(hex[2]) * 17
	case 4: // #RGBA
		r = parseHexDigit(hex[0]) * 17
		g = parseHexDigit(hex[1]) * 17
		b = parseHexDigit(hex[2]) * 17
		a = parseHexDigit(hex[3]) * 17
	case 6: // #RRGGBB
		r = parseHexDigit(hex[0])*16 + parseHexDigit(hex[1])
		g = parseHexDigit(hex[2])*16 + parseHexDigit(hex[3])
		b = parseHexDigit(hex[4])*16 + parseHexDigit(hex[5])
	case 8: // #RRGGBBAA
		r = parseHexDigit(hex[0])*16 + parseHexDigit(hex[1])
		g = parseHexDigit(hex[2])*16 + parseHexDigit(hex[3])
		b = parseHexDigit(hex[4])*16 + parseHexDigit(hex[5])
		a = parseHexDigit(hex[6])*16 + parseHexDigit(hex[7])
	}

	return Color{R: r, G: g, B: b, A: a}
}

func parseHexDigit(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

// parseFunctionValue parses a function into a Value.
func parseFunctionValue(fn *Function) Value {
	name := strings.ToLower(fn.Name)

	switch name {
	case "rgb", "rgba":
		return parseRGBFunction(fn)
	case "hsl", "hsla":
		return parseHSLFunction(fn)
	case "url":
		return parseURLFunction(fn)
	case "calc", "min", "max", "clamp":
		return parseMathFunction(fn)
	default:
		return Value{
			Type: FunctionValue,
			Raw:  fn.String(),
		}
	}
}

// parseRGBFunction parses rgb() or rgba() function.
func parseRGBFunction(fn *Function) Value {
	var r, g, b, a float64 = 0, 0, 0, 1

	nums := extractNumbers(fn.Values)
	if len(nums) >= 3 {
		r = clampColor(nums[0])
		g = clampColor(nums[1])
		b = clampColor(nums[2])
	}
	if len(nums) >= 4 {
		a = clamp01(nums[3])
	}

	return Value{
		Type: ColorValue,
		Color: Color{
			R: uint8(r),
			G: uint8(g),
			B: uint8(b),
			A: uint8(a * 255),
		},
		Raw: fn.String(),
	}
}

// parseHSLFunction parses hsl() or hsla() function.
func parseHSLFunction(fn *Function) Value {
	var h, s, l, a float64 = 0, 0, 0, 1

	nums := extractNumbers(fn.Values)
	if len(nums) >= 3 {
		h = nums[0]
		s = clamp01(nums[1] / 100)
		l = clamp01(nums[2] / 100)
	}
	if len(nums) >= 4 {
		a = clamp01(nums[3])
	}

	// Convert HSL to RGB
	r, g, b := hslToRGB(h, s, l)

	return Value{
		Type: ColorValue,
		Color: Color{
			R: uint8(r * 255),
			G: uint8(g * 255),
			B: uint8(b * 255),
			A: uint8(a * 255),
		},
		Raw: fn.String(),
	}
}

// hslToRGB converts HSL to RGB values (0-1 range).
func hslToRGB(h, s, l float64) (r, g, b float64) {
	h = h - float64(int(h/360))*360
	if h < 0 {
		h += 360
	}
	h /= 360

	if s == 0 {
		return l, l, l
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	r = hueToRGB(p, q, h+1.0/3.0)
	g = hueToRGB(p, q, h)
	b = hueToRGB(p, q, h-1.0/3.0)

	return
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// parseURLFunction parses url() function.
func parseURLFunction(fn *Function) Value {
	var url string
	for _, cv := range fn.Values {
		if pt, ok := cv.(PreservedToken); ok {
			if pt.Token.Type == TokenString || pt.Token.Type == TokenURL {
				url = pt.Token.Value
				break
			}
		}
	}
	return Value{
		Type: URLValue,
		Raw:  url,
	}
}

// parseMathFunction parses calc(), min(), max(), clamp() functions.
func parseMathFunction(fn *Function) Value {
	return Value{
		Type: FunctionValue,
		Raw:  fn.String(),
	}
}

// extractNumbers extracts numeric values from component values.
func extractNumbers(cvs []ComponentValue) []float64 {
	var nums []float64
	for _, cv := range cvs {
		if pt, ok := cv.(PreservedToken); ok {
			switch pt.Token.Type {
			case TokenNumber, TokenPercentage, TokenDimension:
				nums = append(nums, pt.Token.NumValue)
			}
		}
	}
	return nums
}

func clampColor(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
