// Package css provides CSS parsing functionality.
package css

// Stylesheet represents a parsed CSS stylesheet.
type Stylesheet struct {
	Rules []Rule
}

// Rule represents a CSS rule.
type Rule struct {
	Selectors    []Selector
	Declarations []Declaration
}

// Selector represents a CSS selector.
type Selector struct {
	Type     SelectorType
	TagName  string
	ID       string
	Classes  []string
	Combinator string
	Next     *Selector
}

// SelectorType represents the type of selector.
type SelectorType int

const (
	SimpleSelector SelectorType = iota
	ClassSelector
	IDSelector
	UniversalSelector
)

// Declaration represents a CSS property declaration.
type Declaration struct {
	Property string
	Value    Value
}

// Value represents a CSS value.
type Value struct {
	Type    ValueType
	Keyword string
	Length  float64
	Unit    string
	Color   Color
}

// ValueType represents the type of CSS value.
type ValueType int

const (
	KeywordValue ValueType = iota
	LengthValue
	ColorValue
)

// Color represents a CSS color value.
type Color struct {
	R, G, B, A uint8
}

// Parser handles CSS parsing.
type Parser struct {
	input string
	pos   int
}

// NewParser creates a new CSS parser for the given input.
func NewParser(input string) *Parser {
	return &Parser{
		input: input,
		pos:   0,
	}
}

// Parse parses the CSS input and returns a stylesheet.
func (p *Parser) Parse() *Stylesheet {
	// TODO: Implement CSS parsing
	return &Stylesheet{}
}
