package css

import (
	"strings"
)

// ComponentValue represents a component value in CSS.
// It can be a preserved token, function, or simple/curly/square block.
type ComponentValue interface {
	componentValue()
	String() string
}

// PreservedToken wraps a token as a component value.
type PreservedToken struct {
	Token Token
}

func (PreservedToken) componentValue() {}
func (p PreservedToken) String() string {
	return p.Token.String()
}

// Function represents a CSS function.
type Function struct {
	Name   string
	Values []ComponentValue
}

func (Function) componentValue() {}
func (f Function) String() string {
	var sb strings.Builder
	sb.WriteString(f.Name)
	sb.WriteString("(")
	for i, v := range f.Values {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(v.String())
	}
	sb.WriteString(")")
	return sb.String()
}

// Block represents a CSS block (curly, square, or paren).
type Block struct {
	Token  Token // The opening token
	Values []ComponentValue
}

func (Block) componentValue() {}
func (b Block) String() string {
	var sb strings.Builder
	switch b.Token.Type {
	case TokenOpenCurly:
		sb.WriteString("{")
	case TokenOpenSquare:
		sb.WriteString("[")
	case TokenOpenParen:
		sb.WriteString("(")
	}
	for i, v := range b.Values {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(v.String())
	}
	switch b.Token.Type {
	case TokenOpenCurly:
		sb.WriteString("}")
	case TokenOpenSquare:
		sb.WriteString("]")
	case TokenOpenParen:
		sb.WriteString(")")
	}
	return sb.String()
}

// CSSParser parses CSS according to CSS Syntax Module Level 3.
type CSSParser struct {
	tokenizer *Tokenizer
	tokens    []Token
	pos       int
}

// NewCSSParser creates a new CSS parser.
func NewCSSParser(input string) *CSSParser {
	tokenizer := NewTokenizer(input)
	tokens := tokenizer.TokenizeAll()
	return &CSSParser{
		tokenizer: tokenizer,
		tokens:    tokens,
		pos:       0,
	}
}

// current returns the current token.
func (p *CSSParser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

// peek returns the token at offset from current position.
func (p *CSSParser) peek(offset int) Token {
	pos := p.pos + offset
	if pos >= len(p.tokens) || pos < 0 {
		return Token{Type: TokenEOF}
	}
	return p.tokens[pos]
}

// consume consumes and returns the current token.
func (p *CSSParser) consume() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// reconsume backs up one token.
func (p *CSSParser) reconsume() {
	if p.pos > 0 {
		p.pos--
	}
}

// skipWhitespace skips whitespace tokens.
func (p *CSSParser) skipWhitespace() {
	for p.current().Type == TokenWhitespace {
		p.consume()
	}
}

// ParseStylesheet parses the input as a stylesheet.
func (p *CSSParser) ParseStylesheet() *ParsedStylesheet {
	return &ParsedStylesheet{
		Rules: p.consumeRuleList(true),
	}
}

// ParsedStylesheet represents a parsed CSS stylesheet.
type ParsedStylesheet struct {
	Rules []CSSRule
}

// CSSRule is an interface for CSS rules.
type CSSRule interface {
	cssRule()
}

// QualifiedRule represents a qualified rule (e.g., style rules).
type QualifiedRule struct {
	Prelude []ComponentValue
	Block   *Block
}

func (QualifiedRule) cssRule() {}

// AtRule represents an at-rule.
type AtRule struct {
	Name    string
	Prelude []ComponentValue
	Block   *Block // nil if the at-rule has no block
}

func (AtRule) cssRule() {}

// consumeRuleList consumes a list of rules.
func (p *CSSParser) consumeRuleList(topLevel bool) []CSSRule {
	var rules []CSSRule

	for {
		tok := p.current()
		switch tok.Type {
		case TokenEOF:
			return rules
		case TokenWhitespace:
			p.consume()
		case TokenCDO, TokenCDC:
			if topLevel {
				p.consume()
			} else {
				rule := p.consumeQualifiedRule()
				if rule != nil {
					rules = append(rules, rule)
				}
			}
		case TokenAtKeyword:
			rule := p.consumeAtRule()
			if rule != nil {
				rules = append(rules, rule)
			}
		default:
			rule := p.consumeQualifiedRule()
			if rule != nil {
				rules = append(rules, rule)
			}
		}
	}
}

// consumeAtRule consumes an at-rule.
func (p *CSSParser) consumeAtRule() *AtRule {
	tok := p.consume() // @keyword
	rule := &AtRule{
		Name: tok.Value,
	}

	for {
		tok := p.current()
		switch tok.Type {
		case TokenEOF:
			return rule
		case TokenSemicolon:
			p.consume()
			return rule
		case TokenOpenCurly:
			block := p.consumeBlock()
			rule.Block = block
			return rule
		default:
			rule.Prelude = append(rule.Prelude, p.consumeComponentValue())
		}
	}
}

// consumeQualifiedRule consumes a qualified rule.
func (p *CSSParser) consumeQualifiedRule() *QualifiedRule {
	rule := &QualifiedRule{}

	for {
		tok := p.current()
		switch tok.Type {
		case TokenEOF:
			// Parse error - return nil
			return nil
		case TokenOpenCurly:
			block := p.consumeBlock()
			rule.Block = block
			return rule
		default:
			rule.Prelude = append(rule.Prelude, p.consumeComponentValue())
		}
	}
}

// consumeBlock consumes a block.
func (p *CSSParser) consumeBlock() *Block {
	tok := p.consume() // Opening bracket
	block := &Block{Token: tok}

	var endToken TokenType
	switch tok.Type {
	case TokenOpenCurly:
		endToken = TokenCloseCurly
	case TokenOpenSquare:
		endToken = TokenCloseSquare
	case TokenOpenParen:
		endToken = TokenCloseParen
	default:
		return block
	}

	for {
		tok := p.current()
		if tok.Type == endToken || tok.Type == TokenEOF {
			p.consume()
			return block
		}
		block.Values = append(block.Values, p.consumeComponentValue())
	}
}

// consumeComponentValue consumes a component value.
func (p *CSSParser) consumeComponentValue() ComponentValue {
	tok := p.consume()

	switch tok.Type {
	case TokenOpenCurly, TokenOpenSquare, TokenOpenParen:
		p.reconsume()
		return p.consumeBlock()
	case TokenFunction:
		return p.consumeFunction(tok.Value)
	default:
		return PreservedToken{Token: tok}
	}
}

// consumeFunction consumes a function.
func (p *CSSParser) consumeFunction(name string) *Function {
	fn := &Function{Name: name}

	for {
		tok := p.current()
		switch tok.Type {
		case TokenEOF, TokenCloseParen:
			p.consume()
			return fn
		default:
			fn.Values = append(fn.Values, p.consumeComponentValue())
		}
	}
}

// ParseDeclarationList parses a list of declarations.
func (p *CSSParser) ParseDeclarationList() []*CSSDeclaration {
	return p.consumeDeclarationList()
}

// CSSDeclaration represents a CSS declaration.
type CSSDeclaration struct {
	Property  string
	Value     []ComponentValue
	Important bool
}

// consumeDeclarationList consumes a list of declarations.
func (p *CSSParser) consumeDeclarationList() []*CSSDeclaration {
	var declarations []*CSSDeclaration

	for {
		tok := p.current()
		switch tok.Type {
		case TokenEOF:
			return declarations
		case TokenWhitespace, TokenSemicolon:
			p.consume()
		case TokenAtKeyword:
			// At-rules in declaration list - skip for now
			p.consumeAtRule()
		case TokenIdent:
			// Try to consume a declaration
			var tempTokens []Token
			for p.current().Type != TokenSemicolon && p.current().Type != TokenEOF {
				tempTokens = append(tempTokens, p.consume())
			}
			decl := parseDeclarationFromTokens(tempTokens)
			if decl != nil {
				declarations = append(declarations, decl)
			}
		default:
			// Parse error - skip to next semicolon
			for p.current().Type != TokenSemicolon && p.current().Type != TokenEOF {
				p.consume()
			}
		}
	}
}

// parseDeclarationFromTokens parses a declaration from a slice of tokens.
func parseDeclarationFromTokens(tokens []Token) *CSSDeclaration {
	if len(tokens) == 0 {
		return nil
	}

	// Skip leading whitespace
	pos := 0
	for pos < len(tokens) && tokens[pos].Type == TokenWhitespace {
		pos++
	}

	if pos >= len(tokens) || tokens[pos].Type != TokenIdent {
		return nil
	}

	decl := &CSSDeclaration{
		Property: tokens[pos].Value,
	}
	pos++

	// Skip whitespace
	for pos < len(tokens) && tokens[pos].Type == TokenWhitespace {
		pos++
	}

	// Expect colon
	if pos >= len(tokens) || tokens[pos].Type != TokenColon {
		return nil
	}
	pos++

	// Skip whitespace
	for pos < len(tokens) && tokens[pos].Type == TokenWhitespace {
		pos++
	}

	// Consume value tokens
	for pos < len(tokens) {
		tok := tokens[pos]
		if tok.Type == TokenWhitespace {
			// Check for trailing whitespace
			allWS := true
			for i := pos; i < len(tokens); i++ {
				if tokens[i].Type != TokenWhitespace {
					allWS = false
					break
				}
			}
			if allWS {
				break
			}
		}
		decl.Value = append(decl.Value, PreservedToken{Token: tok})
		pos++
	}

	// Check for !important
	if len(decl.Value) >= 2 {
		last := len(decl.Value) - 1
		// Find last non-whitespace
		for last >= 0 {
			if pt, ok := decl.Value[last].(PreservedToken); ok && pt.Token.Type == TokenWhitespace {
				last--
			} else {
				break
			}
		}
		if last >= 1 {
			if pt, ok := decl.Value[last].(PreservedToken); ok {
				if pt.Token.Type == TokenIdent && strings.EqualFold(pt.Token.Value, "important") {
					// Check for ! before it
					prev := last - 1
					for prev >= 0 {
						if pt2, ok := decl.Value[prev].(PreservedToken); ok && pt2.Token.Type == TokenWhitespace {
							prev--
						} else {
							break
						}
					}
					if prev >= 0 {
						if pt2, ok := decl.Value[prev].(PreservedToken); ok && pt2.Token.Type == TokenDelim && pt2.Token.Delim == '!' {
							decl.Important = true
							decl.Value = decl.Value[:prev]
						}
					}
				}
			}
		}
	}

	// Trim trailing whitespace from value
	for len(decl.Value) > 0 {
		if pt, ok := decl.Value[len(decl.Value)-1].(PreservedToken); ok && pt.Token.Type == TokenWhitespace {
			decl.Value = decl.Value[:len(decl.Value)-1]
		} else {
			break
		}
	}

	return decl
}

// ParseBlockContents parses the contents of a block as declarations.
func ParseBlockContents(block *Block) []*CSSDeclaration {
	if block == nil {
		return nil
	}

	// Convert component values back to tokens
	var tokens []Token
	for _, cv := range block.Values {
		tokens = append(tokens, componentValueToTokens(cv)...)
	}

	// Create a parser for the tokens
	parser := &CSSParser{
		tokens: tokens,
		pos:    0,
	}

	return parser.consumeDeclarationList()
}

// componentValueToTokens converts a component value back to tokens.
func componentValueToTokens(cv ComponentValue) []Token {
	switch v := cv.(type) {
	case PreservedToken:
		return []Token{v.Token}
	case *Function:
		tokens := []Token{{Type: TokenFunction, Value: v.Name}}
		for _, val := range v.Values {
			tokens = append(tokens, componentValueToTokens(val)...)
		}
		tokens = append(tokens, Token{Type: TokenCloseParen})
		return tokens
	case *Block:
		tokens := []Token{v.Token}
		for _, val := range v.Values {
			tokens = append(tokens, componentValueToTokens(val)...)
		}
		switch v.Token.Type {
		case TokenOpenCurly:
			tokens = append(tokens, Token{Type: TokenCloseCurly})
		case TokenOpenSquare:
			tokens = append(tokens, Token{Type: TokenCloseSquare})
		case TokenOpenParen:
			tokens = append(tokens, Token{Type: TokenCloseParen})
		}
		return tokens
	default:
		return nil
	}
}
