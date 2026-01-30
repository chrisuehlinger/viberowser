// Package css provides CSS parsing functionality following CSS Syntax Module Level 3.
// Reference: https://www.w3.org/TR/css-syntax-3/
package css

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenType represents the type of a CSS token.
type TokenType int

const (
	// Token types per CSS Syntax Module Level 3 §4
	TokenEOF TokenType = iota
	TokenIdent
	TokenFunction
	TokenAtKeyword
	TokenHash
	TokenString
	TokenBadString
	TokenURL
	TokenBadURL
	TokenDelim
	TokenNumber
	TokenPercentage
	TokenDimension
	TokenWhitespace
	TokenCDO // <!--
	TokenCDC // -->
	TokenColon
	TokenSemicolon
	TokenComma
	TokenOpenSquare    // [
	TokenCloseSquare   // ]
	TokenOpenParen     // (
	TokenCloseParen    // )
	TokenOpenCurly     // {
	TokenCloseCurly    // }
	TokenComment       // Not in the spec as a token but useful
	TokenUnicodeRange  // U+xxxx-yyyy
)

// HashType indicates whether a hash token is an ID or unrestricted.
type HashType int

const (
	HashUnrestricted HashType = iota
	HashID
)

// NumberType indicates whether a number is integer or number.
type NumberType int

const (
	NumberInteger NumberType = iota
	NumberNumber
)

// Token represents a CSS token.
type Token struct {
	Type       TokenType
	Value      string      // The string value of the token
	NumValue   float64     // Numeric value for number/percentage/dimension
	NumType    NumberType  // Whether numeric value is integer or number
	Unit       string      // Unit for dimension tokens
	HashType   HashType    // Type flag for hash tokens
	StartRange rune        // For unicode-range tokens
	EndRange   rune        // For unicode-range tokens
	Delim      rune        // The delimiter character for delim tokens
	Line       int         // Line number in source
	Column     int         // Column number in source
}

func (t Token) String() string {
	switch t.Type {
	case TokenEOF:
		return "<EOF>"
	case TokenIdent:
		return fmt.Sprintf("<IDENT %q>", t.Value)
	case TokenFunction:
		return fmt.Sprintf("<FUNCTION %q>", t.Value)
	case TokenAtKeyword:
		return fmt.Sprintf("<AT-KEYWORD %q>", t.Value)
	case TokenHash:
		if t.HashType == HashID {
			return fmt.Sprintf("<HASH id %q>", t.Value)
		}
		return fmt.Sprintf("<HASH %q>", t.Value)
	case TokenString:
		return fmt.Sprintf("<STRING %q>", t.Value)
	case TokenBadString:
		return "<BAD-STRING>"
	case TokenURL:
		return fmt.Sprintf("<URL %q>", t.Value)
	case TokenBadURL:
		return "<BAD-URL>"
	case TokenDelim:
		return fmt.Sprintf("<DELIM %q>", string(t.Delim))
	case TokenNumber:
		if t.NumType == NumberInteger {
			return fmt.Sprintf("<NUMBER int %v>", t.NumValue)
		}
		return fmt.Sprintf("<NUMBER %v>", t.NumValue)
	case TokenPercentage:
		return fmt.Sprintf("<PERCENTAGE %v%%>", t.NumValue)
	case TokenDimension:
		return fmt.Sprintf("<DIMENSION %v%s>", t.NumValue, t.Unit)
	case TokenWhitespace:
		return "<WHITESPACE>"
	case TokenCDO:
		return "<CDO>"
	case TokenCDC:
		return "<CDC>"
	case TokenColon:
		return "<COLON>"
	case TokenSemicolon:
		return "<SEMICOLON>"
	case TokenComma:
		return "<COMMA>"
	case TokenOpenSquare:
		return "<[>"
	case TokenCloseSquare:
		return "<]>"
	case TokenOpenParen:
		return "<(>"
	case TokenCloseParen:
		return "<)>"
	case TokenOpenCurly:
		return "<{>"
	case TokenCloseCurly:
		return "<}>"
	case TokenComment:
		return "<COMMENT>"
	case TokenUnicodeRange:
		return fmt.Sprintf("<UNICODE-RANGE U+%X-U+%X>", t.StartRange, t.EndRange)
	default:
		return fmt.Sprintf("<UNKNOWN %d>", t.Type)
	}
}

// Tokenizer tokenizes CSS input according to CSS Syntax Module Level 3.
type Tokenizer struct {
	input  []rune
	pos    int
	line   int
	column int
}

// NewTokenizer creates a new CSS tokenizer.
func NewTokenizer(input string) *Tokenizer {
	// Preprocess input per §3.3
	preprocessed := preprocessInput(input)
	return &Tokenizer{
		input:  []rune(preprocessed),
		pos:    0,
		line:   1,
		column: 1,
	}
}

// preprocessInput performs preprocessing per CSS Syntax §3.3.
// - Replace CR LF and CR with LF
// - Replace U+0000 with U+FFFD
// - Replace formfeed with LF
func preprocessInput(input string) string {
	var sb strings.Builder
	sb.Grow(len(input))

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\r':
			// Replace CR LF with LF, or CR alone with LF
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++ // Skip the LF
			}
			sb.WriteRune('\n')
		case '\f':
			sb.WriteRune('\n')
		case 0:
			sb.WriteRune('\uFFFD')
		default:
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// peek returns the current code point without consuming it.
func (t *Tokenizer) peek() rune {
	if t.pos >= len(t.input) {
		return -1 // EOF
	}
	return t.input[t.pos]
}

// peekN returns the code point at offset n from current position.
func (t *Tokenizer) peekN(n int) rune {
	pos := t.pos + n
	if pos >= len(t.input) || pos < 0 {
		return -1
	}
	return t.input[pos]
}

// consume consumes and returns the current code point.
func (t *Tokenizer) consume() rune {
	if t.pos >= len(t.input) {
		return -1
	}
	r := t.input[t.pos]
	t.pos++
	if r == '\n' {
		t.line++
		t.column = 1
	} else {
		t.column++
	}
	return r
}

// reconsume backs up one code point.
func (t *Tokenizer) reconsume() {
	if t.pos > 0 {
		t.pos--
		if t.input[t.pos] == '\n' {
			t.line--
			// Column is approximate after reconsume across newline
			t.column = 1
		} else {
			t.column--
		}
	}
}

// isWhitespace returns true if r is a CSS whitespace character.
func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n'
}

// isDigit returns true if r is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isHexDigit returns true if r is a hex digit.
func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// isLetter returns true if r is a letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isNonASCII returns true if r is a non-ASCII code point.
func isNonASCII(r rune) bool {
	return r >= 0x80
}

// isNameStartCodePoint returns true if r can start an identifier.
func isNameStartCodePoint(r rune) bool {
	return isLetter(r) || isNonASCII(r) || r == '_'
}

// isNameCodePoint returns true if r can be part of an identifier.
func isNameCodePoint(r rune) bool {
	return isNameStartCodePoint(r) || isDigit(r) || r == '-'
}

// startsWithValidEscape checks if the next two code points are a valid escape.
func (t *Tokenizer) startsWithValidEscape() bool {
	return t.peek() == '\\' && t.peekN(1) != '\n'
}

// startsWithValidEscapeAt checks if code points at offset are a valid escape.
func (t *Tokenizer) startsWithValidEscapeAt(offset int) bool {
	return t.peekN(offset) == '\\' && t.peekN(offset+1) != '\n'
}

// startsIdentifier checks if the next code points would start an identifier.
func (t *Tokenizer) startsIdentifier() bool {
	return t.startsIdentifierAt(0)
}

// startsIdentifierAt checks if code points at offset would start an identifier.
func (t *Tokenizer) startsIdentifierAt(offset int) bool {
	first := t.peekN(offset)
	if isNameStartCodePoint(first) {
		return true
	}
	if first == '-' {
		second := t.peekN(offset + 1)
		if isNameStartCodePoint(second) || second == '-' || t.startsWithValidEscapeAt(offset+1) {
			return true
		}
		return false
	}
	if first == '\\' {
		return t.startsWithValidEscapeAt(offset)
	}
	return false
}

// startsNumber checks if the next code points would start a number.
func (t *Tokenizer) startsNumber() bool {
	first := t.peek()
	if isDigit(first) {
		return true
	}
	if first == '+' || first == '-' {
		second := t.peekN(1)
		if isDigit(second) {
			return true
		}
		if second == '.' && isDigit(t.peekN(2)) {
			return true
		}
		return false
	}
	if first == '.' {
		return isDigit(t.peekN(1))
	}
	return false
}

// consumeEscape consumes an escape sequence and returns the code point.
func (t *Tokenizer) consumeEscape() rune {
	// Assumes the backslash has already been consumed
	r := t.consume()
	if r == -1 {
		return '\uFFFD'
	}
	if isHexDigit(r) {
		// Hex escape
		hex := string(r)
		for i := 0; i < 5 && isHexDigit(t.peek()); i++ {
			hex += string(t.consume())
		}
		// Consume optional whitespace after hex escape
		if isWhitespace(t.peek()) {
			t.consume()
		}
		val, _ := strconv.ParseInt(hex, 16, 32)
		if val == 0 || val > 0x10FFFF || (val >= 0xD800 && val <= 0xDFFF) {
			return '\uFFFD'
		}
		return rune(val)
	}
	return r
}

// consumeName consumes an identifier and returns the string.
func (t *Tokenizer) consumeName() string {
	var result strings.Builder
	for {
		r := t.consume()
		if isNameCodePoint(r) {
			result.WriteRune(r)
		} else if r == '\\' && t.peek() != '\n' {
			result.WriteRune(t.consumeEscape())
		} else {
			// Only reconsume if we actually consumed something (not EOF)
			if r != -1 {
				t.reconsume()
			}
			return result.String()
		}
	}
}

// consumeNumber consumes a number and returns value, type representation, and type.
func (t *Tokenizer) consumeNumber() (float64, string, NumberType) {
	var repr strings.Builder
	numType := NumberInteger

	// Sign
	if t.peek() == '+' || t.peek() == '-' {
		repr.WriteRune(t.consume())
	}

	// Integer part
	for isDigit(t.peek()) {
		repr.WriteRune(t.consume())
	}

	// Decimal part
	if t.peek() == '.' && isDigit(t.peekN(1)) {
		repr.WriteRune(t.consume()) // .
		numType = NumberNumber
		for isDigit(t.peek()) {
			repr.WriteRune(t.consume())
		}
	}

	// Exponent part
	if t.peek() == 'e' || t.peek() == 'E' {
		next := t.peekN(1)
		if isDigit(next) || ((next == '+' || next == '-') && isDigit(t.peekN(2))) {
			repr.WriteRune(t.consume()) // e or E
			numType = NumberNumber
			if t.peek() == '+' || t.peek() == '-' {
				repr.WriteRune(t.consume())
			}
			for isDigit(t.peek()) {
				repr.WriteRune(t.consume())
			}
		}
	}

	val, _ := strconv.ParseFloat(repr.String(), 64)
	return val, repr.String(), numType
}

// consumeNumericToken consumes a numeric token.
func (t *Tokenizer) consumeNumericToken() Token {
	line, col := t.line, t.column
	numVal, repr, numType := t.consumeNumber()

	if t.startsIdentifier() {
		unit := t.consumeName()
		return Token{
			Type:     TokenDimension,
			Value:    repr,
			NumValue: numVal,
			NumType:  numType,
			Unit:     unit,
			Line:     line,
			Column:   col,
		}
	}

	if t.peek() == '%' {
		t.consume()
		return Token{
			Type:     TokenPercentage,
			Value:    repr,
			NumValue: numVal,
			NumType:  numType,
			Line:     line,
			Column:   col,
		}
	}

	return Token{
		Type:     TokenNumber,
		Value:    repr,
		NumValue: numVal,
		NumType:  numType,
		Line:     line,
		Column:   col,
	}
}

// consumeString consumes a string token.
func (t *Tokenizer) consumeString(endChar rune) Token {
	line, col := t.line, t.column
	var result strings.Builder

	for {
		r := t.consume()
		switch {
		case r == endChar:
			return Token{
				Type:   TokenString,
				Value:  result.String(),
				Line:   line,
				Column: col,
			}
		case r == -1:
			// EOF - parse error but return what we have
			return Token{
				Type:   TokenString,
				Value:  result.String(),
				Line:   line,
				Column: col,
			}
		case r == '\n':
			// Parse error - bad string
			t.reconsume()
			return Token{
				Type:   TokenBadString,
				Line:   line,
				Column: col,
			}
		case r == '\\':
			next := t.peek()
			if next == -1 {
				// EOF after backslash - ignore
				continue
			}
			if next == '\n' {
				// Escaped newline - consume it and continue
				t.consume()
			} else {
				result.WriteRune(t.consumeEscape())
			}
		default:
			result.WriteRune(r)
		}
	}
}

// consumeURL consumes a URL token (unquoted URL).
func (t *Tokenizer) consumeURL() Token {
	line, col := t.line, t.column
	var result strings.Builder

	// Consume leading whitespace
	for isWhitespace(t.peek()) {
		t.consume()
	}

	for {
		r := t.consume()
		switch {
		case r == ')':
			return Token{
				Type:   TokenURL,
				Value:  result.String(),
				Line:   line,
				Column: col,
			}
		case r == -1:
			// EOF - parse error but return what we have
			return Token{
				Type:   TokenURL,
				Value:  result.String(),
				Line:   line,
				Column: col,
			}
		case isWhitespace(r):
			// Consume trailing whitespace
			for isWhitespace(t.peek()) {
				t.consume()
			}
			if t.peek() == ')' || t.peek() == -1 {
				if t.peek() == ')' {
					t.consume()
				}
				return Token{
					Type:   TokenURL,
					Value:  result.String(),
					Line:   line,
					Column: col,
				}
			}
			// Whitespace in middle - bad URL
			t.consumeBadURLRemnants()
			return Token{
				Type:   TokenBadURL,
				Line:   line,
				Column: col,
			}
		case r == '"' || r == '\'' || r == '(' || isNonPrintable(r):
			// Bad URL
			t.consumeBadURLRemnants()
			return Token{
				Type:   TokenBadURL,
				Line:   line,
				Column: col,
			}
		case r == '\\':
			if t.startsWithValidEscape() {
				t.reconsume()
				t.consume() // re-consume backslash
				result.WriteRune(t.consumeEscape())
			} else {
				// Bad URL
				t.consumeBadURLRemnants()
				return Token{
					Type:   TokenBadURL,
					Line:   line,
					Column: col,
				}
			}
		default:
			result.WriteRune(r)
		}
	}
}

// consumeBadURLRemnants consumes the remnants of a bad URL.
func (t *Tokenizer) consumeBadURLRemnants() {
	for {
		r := t.consume()
		if r == ')' || r == -1 {
			return
		}
		if r == '\\' && t.peek() != '\n' && t.peek() != -1 {
			t.consume() // Consume escaped character
		}
	}
}

// isNonPrintable returns true if r is a non-printable code point.
func isNonPrintable(r rune) bool {
	return (r >= 0 && r <= 0x08) || r == 0x0B || (r >= 0x0E && r <= 0x1F) || r == 0x7F
}

// consumeIdentLikeToken consumes an ident-like token.
func (t *Tokenizer) consumeIdentLikeToken() Token {
	line, col := t.line, t.column
	name := t.consumeName()

	// Check for url()
	if strings.EqualFold(name, "url") && t.peek() == '(' {
		t.consume() // (

		// Check for quoted URL
		for isWhitespace(t.peek()) {
			t.consume()
		}

		if t.peek() == '"' || t.peek() == '\'' {
			// This is url("...") or url('...') - return function token
			return Token{
				Type:   TokenFunction,
				Value:  name,
				Line:   line,
				Column: col,
			}
		}

		// Unquoted URL
		return t.consumeURL()
	}

	if t.peek() == '(' {
		t.consume()
		return Token{
			Type:   TokenFunction,
			Value:  name,
			Line:   line,
			Column: col,
		}
	}

	return Token{
		Type:   TokenIdent,
		Value:  name,
		Line:   line,
		Column: col,
	}
}

// consumeHashToken consumes a hash token.
func (t *Tokenizer) consumeHashToken() Token {
	line, col := t.line, t.column
	t.consume() // #

	if isNameCodePoint(t.peek()) || t.startsWithValidEscape() {
		hashType := HashUnrestricted
		if t.startsIdentifier() {
			hashType = HashID
		}
		name := t.consumeName()
		return Token{
			Type:     TokenHash,
			Value:    name,
			HashType: hashType,
			Line:     line,
			Column:   col,
		}
	}

	return Token{
		Type:   TokenDelim,
		Delim:  '#',
		Line:   line,
		Column: col,
	}
}

// consumeComment consumes a comment.
func (t *Tokenizer) consumeComment() Token {
	line, col := t.line, t.column
	t.consume() // /
	t.consume() // *

	for {
		r := t.consume()
		if r == -1 {
			break
		}
		if r == '*' && t.peek() == '/' {
			t.consume()
			break
		}
	}

	return Token{
		Type:   TokenComment,
		Line:   line,
		Column: col,
	}
}

// consumeUnicodeRange consumes a unicode-range token.
func (t *Tokenizer) consumeUnicodeRange() Token {
	line, col := t.line, t.column

	// Consume hex digits (up to 6)
	var hex strings.Builder
	for i := 0; i < 6 && isHexDigit(t.peek()); i++ {
		hex.WriteRune(t.consume())
	}

	// Count and consume ? wildcards
	wildcards := 0
	for wildcards+hex.Len() < 6 && t.peek() == '?' {
		t.consume()
		wildcards++
	}

	var start, end rune
	if wildcards > 0 {
		// Calculate range from wildcards
		startHex := hex.String() + strings.Repeat("0", wildcards)
		endHex := hex.String() + strings.Repeat("F", wildcards)
		startVal, _ := strconv.ParseInt(startHex, 16, 32)
		endVal, _ := strconv.ParseInt(endHex, 16, 32)
		start = rune(startVal)
		end = rune(endVal)
	} else {
		startVal, _ := strconv.ParseInt(hex.String(), 16, 32)
		start = rune(startVal)
		end = start

		// Check for range
		if t.peek() == '-' && isHexDigit(t.peekN(1)) {
			t.consume() // -
			var endHex strings.Builder
			for i := 0; i < 6 && isHexDigit(t.peek()); i++ {
				endHex.WriteRune(t.consume())
			}
			endVal, _ := strconv.ParseInt(endHex.String(), 16, 32)
			end = rune(endVal)
		}
	}

	return Token{
		Type:       TokenUnicodeRange,
		StartRange: start,
		EndRange:   end,
		Line:       line,
		Column:     col,
	}
}

// NextToken returns the next token from the input.
func (t *Tokenizer) NextToken() Token {
	// Consume comments (they don't produce tokens in most uses)
	for t.peek() == '/' && t.peekN(1) == '*' {
		t.consumeComment()
	}

	line, col := t.line, t.column
	r := t.consume()

	switch {
	case r == -1:
		return Token{Type: TokenEOF, Line: line, Column: col}

	case isWhitespace(r):
		for isWhitespace(t.peek()) {
			t.consume()
		}
		return Token{Type: TokenWhitespace, Line: line, Column: col}

	case r == '"':
		return t.consumeString('"')

	case r == '\'':
		return t.consumeString('\'')

	case r == '#':
		t.reconsume()
		return t.consumeHashToken()

	case r == '(':
		return Token{Type: TokenOpenParen, Line: line, Column: col}

	case r == ')':
		return Token{Type: TokenCloseParen, Line: line, Column: col}

	case r == '+':
		if t.startsNumber() {
			t.reconsume()
			return t.consumeNumericToken()
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == ',':
		return Token{Type: TokenComma, Line: line, Column: col}

	case r == '-':
		if t.startsNumber() {
			t.reconsume()
			return t.consumeNumericToken()
		}
		if t.peek() == '-' && t.peekN(1) == '>' {
			t.consume()
			t.consume()
			return Token{Type: TokenCDC, Line: line, Column: col}
		}
		if t.startsIdentifier() {
			t.reconsume()
			return t.consumeIdentLikeToken()
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == '.':
		if t.startsNumber() {
			t.reconsume()
			return t.consumeNumericToken()
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == ':':
		return Token{Type: TokenColon, Line: line, Column: col}

	case r == ';':
		return Token{Type: TokenSemicolon, Line: line, Column: col}

	case r == '<':
		if t.peek() == '!' && t.peekN(1) == '-' && t.peekN(2) == '-' {
			t.consume()
			t.consume()
			t.consume()
			return Token{Type: TokenCDO, Line: line, Column: col}
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == '@':
		if t.startsIdentifier() {
			name := t.consumeName()
			return Token{Type: TokenAtKeyword, Value: name, Line: line, Column: col}
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == '[':
		return Token{Type: TokenOpenSquare, Line: line, Column: col}

	case r == '\\':
		if t.peek() != '\n' {
			t.reconsume()
			return t.consumeIdentLikeToken()
		}
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}

	case r == ']':
		return Token{Type: TokenCloseSquare, Line: line, Column: col}

	case r == '{':
		return Token{Type: TokenOpenCurly, Line: line, Column: col}

	case r == '}':
		return Token{Type: TokenCloseCurly, Line: line, Column: col}

	case r == 'U' || r == 'u':
		// Check for unicode-range
		if t.peek() == '+' && (isHexDigit(t.peekN(1)) || t.peekN(1) == '?') {
			t.consume() // +
			return t.consumeUnicodeRange()
		}
		t.reconsume()
		return t.consumeIdentLikeToken()

	case isDigit(r):
		t.reconsume()
		return t.consumeNumericToken()

	case isNameStartCodePoint(r):
		t.reconsume()
		return t.consumeIdentLikeToken()

	default:
		return Token{Type: TokenDelim, Delim: r, Line: line, Column: col}
	}
}

// TokenizeAll tokenizes the entire input and returns all tokens.
func (t *Tokenizer) TokenizeAll() []Token {
	var tokens []Token
	for {
		tok := t.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens
}

// TokenizeAllSkipWS tokenizes and returns tokens, skipping whitespace.
func (t *Tokenizer) TokenizeAllSkipWS() []Token {
	var tokens []Token
	for {
		tok := t.NextToken()
		if tok.Type == TokenWhitespace {
			continue
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens
}

// Helper for unicode handling
func init() {
	// Verify utf8 is available
	_ = utf8.RuneError
	_ = unicode.IsLetter
}
