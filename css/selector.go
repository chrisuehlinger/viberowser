package css

import (
	"strings"
)

// CSSSelector represents a parsed CSS selector.
type CSSSelector struct {
	// A selector is a list of complex selectors separated by commas
	ComplexSelectors []*ComplexSelector
}

// ComplexSelector is a chain of compound selectors separated by combinators.
type ComplexSelector struct {
	Compounds []*CompoundSelector
}

// CompoundSelector is a sequence of simple selectors.
type CompoundSelector struct {
	TypeSelector      *TypeSelector
	IDSelectors       []string
	ClassSelectors    []string
	AttributeMatchers []*AttributeMatcher
	PseudoClasses     []*PseudoClassSelector
	PseudoElement     *PseudoElementSelector
	Combinator        CombinatorType // Combinator following this compound selector
}

// CombinatorType represents the type of combinator.
type CombinatorType int

const (
	CombinatorNone       CombinatorType = iota
	CombinatorDescendant                // (whitespace)
	CombinatorChild                     // >
	CombinatorNextSibling               // +
	CombinatorSubsequentSibling         // ~
	CombinatorColumn                    // || (CSS Grid)
)

// TypeSelector represents a type (tag) selector.
type TypeSelector struct {
	Namespace string // "*" for any, "" for no namespace, or actual namespace
	Name      string // "*" for universal, or tag name
}

// AttributeMatcher represents an attribute selector.
type AttributeMatcher struct {
	Namespace     string
	Name          string
	Operator      AttributeOperator
	Value         string
	CaseInsensitive bool
}

// AttributeOperator represents the operator in an attribute selector.
type AttributeOperator int

const (
	AttrExists       AttributeOperator = iota // [attr]
	AttrEquals                                // [attr=value]
	AttrIncludes                              // [attr~=value]
	AttrDashMatch                             // [attr|=value]
	AttrPrefix                                // [attr^=value]
	AttrSuffix                                // [attr$=value]
	AttrSubstring                             // [attr*=value]
)

// PseudoClassSelector represents a pseudo-class.
type PseudoClassSelector struct {
	Name     string
	Argument string           // For functional pseudo-classes like :nth-child(2n+1)
	Selector *CSSSelector     // For pseudo-classes like :not(), :is(), :where(), :has()
}

// PseudoElementSelector represents a pseudo-element.
type PseudoElementSelector struct {
	Name     string
	Argument string // For functional pseudo-elements
}

// SelectorParser parses CSS selectors.
type SelectorParser struct {
	tokens []Token
	pos    int
}

// ParseSelector parses a CSS selector string.
func ParseSelector(input string) (*CSSSelector, error) {
	tokenizer := NewTokenizer(input)
	tokens := tokenizer.TokenizeAll()
	parser := &SelectorParser{tokens: tokens, pos: 0}
	return parser.parseSelector()
}

// ParseSelectorFromTokens parses a selector from component values.
func ParseSelectorFromTokens(tokens []Token) (*CSSSelector, error) {
	parser := &SelectorParser{tokens: tokens, pos: 0}
	return parser.parseSelector()
}

func (p *SelectorParser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *SelectorParser) peek(offset int) Token {
	pos := p.pos + offset
	if pos >= len(p.tokens) || pos < 0 {
		return Token{Type: TokenEOF}
	}
	return p.tokens[pos]
}

func (p *SelectorParser) consume() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *SelectorParser) skipWhitespace() bool {
	skipped := false
	for p.current().Type == TokenWhitespace {
		p.consume()
		skipped = true
	}
	return skipped
}

// parseSelector parses a selector list.
func (p *SelectorParser) parseSelector() (*CSSSelector, error) {
	selector := &CSSSelector{}

	p.skipWhitespace()

	for {
		complex, err := p.parseComplexSelector()
		if err != nil {
			return nil, err
		}
		if complex != nil {
			selector.ComplexSelectors = append(selector.ComplexSelectors, complex)
		}

		p.skipWhitespace()

		if p.current().Type == TokenComma {
			p.consume()
			p.skipWhitespace()
			continue
		}

		break
	}

	return selector, nil
}

// parseComplexSelector parses a complex selector.
func (p *SelectorParser) parseComplexSelector() (*ComplexSelector, error) {
	complex := &ComplexSelector{}

	for {
		compound, err := p.parseCompoundSelector()
		if err != nil {
			return nil, err
		}
		if compound == nil {
			break
		}

		complex.Compounds = append(complex.Compounds, compound)

		// Check for combinator
		hadWhitespace := p.skipWhitespace()

		tok := p.current()
		switch tok.Type {
		case TokenDelim:
			switch tok.Delim {
			case '>':
				p.consume()
				compound.Combinator = CombinatorChild
				p.skipWhitespace()
			case '+':
				p.consume()
				compound.Combinator = CombinatorNextSibling
				p.skipWhitespace()
			case '~':
				p.consume()
				compound.Combinator = CombinatorSubsequentSibling
				p.skipWhitespace()
			case '|':
				if p.peek(1).Type == TokenDelim && p.peek(1).Delim == '|' {
					p.consume()
					p.consume()
					compound.Combinator = CombinatorColumn
					p.skipWhitespace()
				} else {
					goto done
				}
			default:
				if hadWhitespace {
					compound.Combinator = CombinatorDescendant
				} else {
					goto done
				}
			}
		case TokenEOF, TokenComma, TokenOpenCurly:
			goto done
		default:
			if hadWhitespace {
				compound.Combinator = CombinatorDescendant
			} else {
				goto done
			}
		}
	}

done:
	if len(complex.Compounds) == 0 {
		return nil, nil
	}
	return complex, nil
}

// parseCompoundSelector parses a compound selector.
func (p *SelectorParser) parseCompoundSelector() (*CompoundSelector, error) {
	compound := &CompoundSelector{}
	hasContent := false

	// Check for type selector first
	if p.isTypeSelector() {
		ts, err := p.parseTypeSelector()
		if err != nil {
			return nil, err
		}
		compound.TypeSelector = ts
		hasContent = true
	}

	// Parse simple selectors
	for {
		tok := p.current()
		switch tok.Type {
		case TokenHash:
			if tok.HashType == HashID {
				p.consume()
				compound.IDSelectors = append(compound.IDSelectors, tok.Value)
				hasContent = true
			} else {
				goto done
			}

		case TokenDelim:
			switch tok.Delim {
			case '.':
				p.consume()
				if p.current().Type == TokenIdent {
					compound.ClassSelectors = append(compound.ClassSelectors, p.consume().Value)
					hasContent = true
				}
			case '*':
				if compound.TypeSelector == nil && !hasContent {
					p.consume()
					compound.TypeSelector = &TypeSelector{Name: "*"}
					hasContent = true
				} else {
					goto done
				}
			case ':':
				p.consume()
				if p.current().Type == TokenColon {
					// Pseudo-element
					p.consume()
					pe, err := p.parsePseudoElement()
					if err != nil {
						return nil, err
					}
					compound.PseudoElement = pe
					hasContent = true
				} else {
					goto done
				}
			default:
				goto done
			}

		case TokenColon:
			p.consume()
			if p.current().Type == TokenColon {
				// Pseudo-element with ::
				p.consume()
				pe, err := p.parsePseudoElement()
				if err != nil {
					return nil, err
				}
				compound.PseudoElement = pe
				hasContent = true
			} else {
				// Pseudo-class
				pc, err := p.parsePseudoClass()
				if err != nil {
					return nil, err
				}
				compound.PseudoClasses = append(compound.PseudoClasses, pc)
				hasContent = true
			}

		case TokenOpenSquare:
			attr, err := p.parseAttributeSelector()
			if err != nil {
				return nil, err
			}
			compound.AttributeMatchers = append(compound.AttributeMatchers, attr)
			hasContent = true

		default:
			goto done
		}
	}

done:
	if !hasContent {
		return nil, nil
	}
	return compound, nil
}

// isTypeSelector checks if current position starts a type selector.
func (p *SelectorParser) isTypeSelector() bool {
	tok := p.current()
	if tok.Type == TokenIdent {
		return true
	}
	if tok.Type == TokenDelim && (tok.Delim == '*' || tok.Delim == '|') {
		return true
	}
	return false
}

// parseTypeSelector parses a type selector.
func (p *SelectorParser) parseTypeSelector() (*TypeSelector, error) {
	ts := &TypeSelector{}

	tok := p.current()

	// Handle namespace prefix
	if tok.Type == TokenDelim && tok.Delim == '*' {
		p.consume()
		if p.current().Type == TokenDelim && p.current().Delim == '|' {
			p.consume()
			ts.Namespace = "*"
			// Now parse the element name
			tok = p.current()
		} else {
			ts.Name = "*"
			return ts, nil
		}
	} else if tok.Type == TokenDelim && tok.Delim == '|' {
		p.consume()
		ts.Namespace = ""
		tok = p.current()
	} else if tok.Type == TokenIdent {
		// Could be namespace or element name
		next := p.peek(1)
		if next.Type == TokenDelim && next.Delim == '|' {
			ts.Namespace = tok.Value
			p.consume() // ident
			p.consume() // |
			tok = p.current()
		}
	}

	// Parse element name
	if tok.Type == TokenIdent {
		ts.Name = strings.ToLower(p.consume().Value)
	} else if tok.Type == TokenDelim && tok.Delim == '*' {
		p.consume()
		ts.Name = "*"
	} else if ts.Namespace != "" {
		// Had namespace but no element name
		ts.Name = "*"
	}

	return ts, nil
}

// parseAttributeSelector parses an attribute selector.
func (p *SelectorParser) parseAttributeSelector() (*AttributeMatcher, error) {
	p.consume() // [

	attr := &AttributeMatcher{}

	p.skipWhitespace()

	// Parse namespace (if any) and attribute name
	tok := p.current()
	if tok.Type == TokenDelim && tok.Delim == '*' {
		p.consume()
		if p.current().Type == TokenDelim && p.current().Delim == '|' {
			p.consume()
			attr.Namespace = "*"
		}
	} else if tok.Type == TokenDelim && tok.Delim == '|' {
		p.consume()
		attr.Namespace = ""
	} else if tok.Type == TokenIdent {
		// Check if this is a namespace prefix (ident followed by | followed by ident)
		// NOT if it's followed by |= which is an operator
		next := p.peek(1)
		nextNext := p.peek(2)
		if next.Type == TokenDelim && next.Delim == '|' && nextNext.Type == TokenIdent {
			attr.Namespace = tok.Value
			p.consume() // ident
			p.consume() // |
		}
	}

	// Attribute name
	if p.current().Type == TokenIdent {
		attr.Name = strings.ToLower(p.consume().Value)
	}

	p.skipWhitespace()

	// Check for operator
	tok = p.current()
	if tok.Type == TokenCloseSquare {
		p.consume()
		attr.Operator = AttrExists
		return attr, nil
	}

	// Parse operator
	if tok.Type == TokenDelim {
		switch tok.Delim {
		case '=':
			p.consume()
			attr.Operator = AttrEquals
		case '~':
			p.consume()
			if p.current().Type == TokenDelim && p.current().Delim == '=' {
				p.consume()
				attr.Operator = AttrIncludes
			}
		case '|':
			p.consume()
			if p.current().Type == TokenDelim && p.current().Delim == '=' {
				p.consume()
				attr.Operator = AttrDashMatch
			}
		case '^':
			p.consume()
			if p.current().Type == TokenDelim && p.current().Delim == '=' {
				p.consume()
				attr.Operator = AttrPrefix
			}
		case '$':
			p.consume()
			if p.current().Type == TokenDelim && p.current().Delim == '=' {
				p.consume()
				attr.Operator = AttrSuffix
			}
		case '*':
			p.consume()
			if p.current().Type == TokenDelim && p.current().Delim == '=' {
				p.consume()
				attr.Operator = AttrSubstring
			}
		}
	}

	p.skipWhitespace()

	// Parse value
	tok = p.current()
	if tok.Type == TokenString || tok.Type == TokenIdent {
		attr.Value = p.consume().Value
	}

	p.skipWhitespace()

	// Check for case-insensitivity flag
	tok = p.current()
	if tok.Type == TokenIdent && (tok.Value == "i" || tok.Value == "I" || tok.Value == "s" || tok.Value == "S") {
		if tok.Value == "i" || tok.Value == "I" {
			attr.CaseInsensitive = true
		}
		p.consume()
		p.skipWhitespace()
	}

	// Consume closing bracket
	if p.current().Type == TokenCloseSquare {
		p.consume()
	}

	return attr, nil
}

// parsePseudoClass parses a pseudo-class selector.
func (p *SelectorParser) parsePseudoClass() (*PseudoClassSelector, error) {
	pc := &PseudoClassSelector{}

	tok := p.current()
	if tok.Type == TokenIdent {
		pc.Name = strings.ToLower(p.consume().Value)
	} else if tok.Type == TokenFunction {
		pc.Name = strings.ToLower(p.consume().Value)
		// Parse functional argument
		p.skipWhitespace()

		// Check if this is a selector-taking pseudo-class
		switch pc.Name {
		case "not", "is", "where", "has":
			// Parse selector argument
			var tokens []Token
			depth := 1
			for {
				tok := p.current()
				if tok.Type == TokenEOF {
					break
				}
				if tok.Type == TokenOpenParen {
					depth++
				} else if tok.Type == TokenCloseParen {
					depth--
					if depth == 0 {
						p.consume()
						break
					}
				}
				tokens = append(tokens, p.consume())
			}
			subParser := &SelectorParser{tokens: tokens, pos: 0}
			selector, _ := subParser.parseSelector()
			pc.Selector = selector
		default:
			// Consume until closing paren
			var arg strings.Builder
			depth := 1
			for {
				tok := p.current()
				if tok.Type == TokenEOF {
					break
				}
				if tok.Type == TokenOpenParen {
					depth++
					arg.WriteString("(")
				} else if tok.Type == TokenCloseParen {
					depth--
					if depth == 0 {
						p.consume()
						break
					}
					arg.WriteString(")")
				} else if tok.Type == TokenWhitespace {
					arg.WriteString(" ")
				} else if tok.Type == TokenIdent {
					arg.WriteString(tok.Value)
				} else if tok.Type == TokenNumber {
					arg.WriteString(tok.Value)
				} else if tok.Type == TokenDimension {
					arg.WriteString(tok.Value)
					arg.WriteString(tok.Unit)
				} else if tok.Type == TokenDelim {
					arg.WriteRune(tok.Delim)
				}
				p.consume()
			}
			pc.Argument = strings.TrimSpace(arg.String())
		}
	}

	return pc, nil
}

// parsePseudoElement parses a pseudo-element selector.
func (p *SelectorParser) parsePseudoElement() (*PseudoElementSelector, error) {
	pe := &PseudoElementSelector{}

	tok := p.current()
	if tok.Type == TokenIdent {
		pe.Name = strings.ToLower(p.consume().Value)
	} else if tok.Type == TokenFunction {
		pe.Name = strings.ToLower(p.consume().Value)
		// Parse functional argument
		var arg strings.Builder
		depth := 1
		for {
			tok := p.current()
			if tok.Type == TokenEOF {
				break
			}
			if tok.Type == TokenOpenParen {
				depth++
			} else if tok.Type == TokenCloseParen {
				depth--
				if depth == 0 {
					p.consume()
					break
				}
			}
			arg.WriteString(tok.Value)
			p.consume()
		}
		pe.Argument = arg.String()
	}

	return pe, nil
}

// Specificity represents CSS selector specificity.
// Per https://www.w3.org/TR/selectors-4/#specificity
type Specificity struct {
	A int // ID selectors
	B int // Class selectors, attribute selectors, pseudo-classes
	C int // Type selectors, pseudo-elements
}

// Compare compares two specificities. Returns -1, 0, or 1.
func (s Specificity) Compare(other Specificity) int {
	if s.A != other.A {
		if s.A > other.A {
			return 1
		}
		return -1
	}
	if s.B != other.B {
		if s.B > other.B {
			return 1
		}
		return -1
	}
	if s.C != other.C {
		if s.C > other.C {
			return 1
		}
		return -1
	}
	return 0
}

// Less returns true if this specificity is less than the other.
func (s Specificity) Less(other Specificity) bool {
	return s.Compare(other) < 0
}

// CalculateSpecificity calculates the specificity of a complex selector.
func (cs *ComplexSelector) CalculateSpecificity() Specificity {
	var spec Specificity
	for _, compound := range cs.Compounds {
		spec.A += len(compound.IDSelectors)
		spec.B += len(compound.ClassSelectors)
		spec.B += len(compound.AttributeMatchers)
		spec.B += len(compound.PseudoClasses)
		if compound.TypeSelector != nil && compound.TypeSelector.Name != "*" {
			spec.C++
		}
		if compound.PseudoElement != nil {
			spec.C++
		}
	}
	return spec
}

// CalculateSpecificity returns the maximum specificity of any complex selector.
func (s *CSSSelector) CalculateSpecificity() Specificity {
	var maxSpec Specificity
	for _, cs := range s.ComplexSelectors {
		spec := cs.CalculateSpecificity()
		if maxSpec.Less(spec) {
			maxSpec = spec
		}
	}
	return maxSpec
}
