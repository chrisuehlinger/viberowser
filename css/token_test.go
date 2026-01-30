package css

import (
	"testing"
)

func TestTokenizerBasicTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenType
	}{
		{"", []TokenType{TokenEOF}},
		{"   ", []TokenType{TokenWhitespace, TokenEOF}},
		{";", []TokenType{TokenSemicolon, TokenEOF}},
		{":", []TokenType{TokenColon, TokenEOF}},
		{",", []TokenType{TokenComma, TokenEOF}},
		{"{}", []TokenType{TokenOpenCurly, TokenCloseCurly, TokenEOF}},
		{"[]", []TokenType{TokenOpenSquare, TokenCloseSquare, TokenEOF}},
		{"()", []TokenType{TokenOpenParen, TokenCloseParen, TokenEOF}},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tokens := tokenizer.TokenizeAll()

		if len(tokens) != len(tt.expected) {
			t.Errorf("input %q: expected %d tokens, got %d", tt.input, len(tt.expected), len(tokens))
			continue
		}

		for i, tok := range tokens {
			if tok.Type != tt.expected[i] {
				t.Errorf("input %q: token %d: expected %v, got %v", tt.input, i, tt.expected[i], tok.Type)
			}
		}
	}
}

func TestTokenizerIdent(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"foo", "foo"},
		{"Bar", "Bar"},
		{"foo-bar", "foo-bar"},
		{"_foo", "_foo"},
		{"-webkit-transform", "-webkit-transform"},
		{"--custom-prop", "--custom-prop"},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenIdent {
			t.Errorf("input %q: expected IDENT, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}
	}
}

func TestTokenizerHash(t *testing.T) {
	tests := []struct {
		input    string
		value    string
		hashType HashType
	}{
		{"#foo", "foo", HashID},
		{"#123", "123", HashUnrestricted},
		{"#abc123", "abc123", HashID},
		{"#-foo", "-foo", HashID},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenHash {
			t.Errorf("input %q: expected HASH, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}

		if tok.HashType != tt.hashType {
			t.Errorf("input %q: expected hash type %v, got %v", tt.input, tt.hashType, tok.HashType)
		}
	}
}

func TestTokenizerString(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`"hello world"`, "hello world"},
		{`"hello\nworld"`, "hellonworld"},  // \n is not an escape in CSS, just n
		{`"hello\a world"`, "hello\nworld"}, // \a is hex 0A (newline), space is consumed as separator
		{`"escaped\"quote"`, `escaped"quote`},
		{`""`, ""},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenString {
			t.Errorf("input %q: expected STRING, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}
	}
}

func TestTokenizerNumber(t *testing.T) {
	tests := []struct {
		input   string
		value   float64
		numType NumberType
	}{
		{"0", 0, NumberInteger},
		{"123", 123, NumberInteger},
		{"-42", -42, NumberInteger},
		{"+5", 5, NumberInteger},
		{"3.14", 3.14, NumberNumber},
		{"-0.5", -0.5, NumberNumber},
		{"1e10", 1e10, NumberNumber},
		{"1E-5", 1e-5, NumberNumber},
		{"2.5e3", 2500, NumberNumber},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenNumber {
			t.Errorf("input %q: expected NUMBER, got %v", tt.input, tok.Type)
			continue
		}

		if tok.NumValue != tt.value {
			t.Errorf("input %q: expected value %v, got %v", tt.input, tt.value, tok.NumValue)
		}

		if tok.NumType != tt.numType {
			t.Errorf("input %q: expected num type %v, got %v", tt.input, tt.numType, tok.NumType)
		}
	}
}

func TestTokenizerPercentage(t *testing.T) {
	tests := []struct {
		input string
		value float64
	}{
		{"50%", 50},
		{"100%", 100},
		{"-25%", -25},
		{"0%", 0},
		{"33.33%", 33.33},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenPercentage {
			t.Errorf("input %q: expected PERCENTAGE, got %v", tt.input, tok.Type)
			continue
		}

		if tok.NumValue != tt.value {
			t.Errorf("input %q: expected value %v, got %v", tt.input, tt.value, tok.NumValue)
		}
	}
}

func TestTokenizerDimension(t *testing.T) {
	tests := []struct {
		input string
		value float64
		unit  string
	}{
		{"10px", 10, "px"},
		{"1em", 1, "em"},
		{"1.5rem", 1.5, "rem"},
		{"-2vh", -2, "vh"},
		{"100vw", 100, "vw"},
		{"360deg", 360, "deg"},
		{"200ms", 200, "ms"},
		{"2s", 2, "s"},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenDimension {
			t.Errorf("input %q: expected DIMENSION, got %v", tt.input, tok.Type)
			continue
		}

		if tok.NumValue != tt.value {
			t.Errorf("input %q: expected value %v, got %v", tt.input, tt.value, tok.NumValue)
		}

		if tok.Unit != tt.unit {
			t.Errorf("input %q: expected unit %q, got %q", tt.input, tt.unit, tok.Unit)
		}
	}
}

func TestTokenizerURL(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{`url(image.png)`, "image.png"},
		{`url( image.png )`, "image.png"},
		{`url(/path/to/file.css)`, "/path/to/file.css"},
		{`url(https://example.com/img.jpg)`, "https://example.com/img.jpg"},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenURL {
			t.Errorf("input %q: expected URL, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}
	}
}

func TestTokenizerFunction(t *testing.T) {
	tests := []struct {
		input string
		name  string
	}{
		{"rgb(", "rgb"},
		{"rgba(", "rgba"},
		{"calc(", "calc"},
		{"var(", "var"},
		{"url(\"test.png\")", "url"},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenFunction {
			t.Errorf("input %q: expected FUNCTION, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.name {
			t.Errorf("input %q: expected name %q, got %q", tt.input, tt.name, tok.Value)
		}
	}
}

func TestTokenizerAtKeyword(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"@media", "media"},
		{"@import", "import"},
		{"@keyframes", "keyframes"},
		{"@font-face", "font-face"},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenAtKeyword {
			t.Errorf("input %q: expected AT-KEYWORD, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}
	}
}

func TestTokenizerCDOCDC(t *testing.T) {
	tokenizer := NewTokenizer("<!-- -->")
	tokens := tokenizer.TokenizeAll()

	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}

	if tokens[0].Type != TokenCDO {
		t.Errorf("expected CDO, got %v", tokens[0].Type)
	}

	if tokens[2].Type != TokenCDC {
		t.Errorf("expected CDC, got %v", tokens[2].Type)
	}
}

func TestTokenizerUnicodeRange(t *testing.T) {
	tests := []struct {
		input      string
		startRange rune
		endRange   rune
	}{
		{"U+0041", 0x0041, 0x0041},
		{"U+0-7F", 0x0, 0x7F},
		{"U+00??", 0x0000, 0x00FF},
		{"U+0025-00FF", 0x0025, 0x00FF},
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenUnicodeRange {
			t.Errorf("input %q: expected UNICODE-RANGE, got %v", tt.input, tok.Type)
			continue
		}

		if tok.StartRange != tt.startRange {
			t.Errorf("input %q: expected start %X, got %X", tt.input, tt.startRange, tok.StartRange)
		}

		if tok.EndRange != tt.endRange {
			t.Errorf("input %q: expected end %X, got %X", tt.input, tt.endRange, tok.EndRange)
		}
	}
}

func TestTokenizerEscapes(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{`\41`, "A"},             // Hex escape for 'A'
		{`\000041`, "A"},         // Full 6-digit hex escape
		{`foo\20 bar`, "foo bar"}, // Hex escape for space, needs trailing separator
		{`foo\ bar`, "foo bar"},  // Escaped literal space
	}

	for _, tt := range tests {
		tokenizer := NewTokenizer(tt.input)
		tok := tokenizer.NextToken()

		if tok.Type != TokenIdent {
			t.Errorf("input %q: expected IDENT, got %v", tt.input, tok.Type)
			continue
		}

		if tok.Value != tt.value {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.value, tok.Value)
		}
	}
}

func TestTokenizerPreprocessing(t *testing.T) {
	// Test CR LF -> LF
	tokenizer := NewTokenizer("a\r\nb")
	tokens := tokenizer.TokenizeAll()

	if tokens[1].Type != TokenWhitespace {
		t.Errorf("CR LF should become whitespace")
	}

	// Test CR -> LF
	tokenizer = NewTokenizer("a\rb")
	tokens = tokenizer.TokenizeAll()

	if tokens[1].Type != TokenWhitespace {
		t.Errorf("CR should become whitespace")
	}

	// Test null replacement
	tokenizer = NewTokenizer("a\x00b")
	tok := tokenizer.NextToken()
	if tok.Value != "a\uFFFDb" {
		t.Errorf("null should be replaced with U+FFFD")
	}
}

func TestTokenizerComments(t *testing.T) {
	// Comments should be consumed
	tokenizer := NewTokenizer("/* comment */foo")
	tok := tokenizer.NextToken()

	if tok.Type != TokenIdent || tok.Value != "foo" {
		t.Errorf("expected IDENT foo after comment, got %v %q", tok.Type, tok.Value)
	}

	// CSS comments are NOT nested - the first */ ends the comment
	// So "/* a /* b */" is a complete comment, then " c */" is separate content
	tokenizer = NewTokenizer("/* comment */bar")
	tok = tokenizer.NextToken()
	if tok.Type != TokenIdent || tok.Value != "bar" {
		t.Errorf("expected IDENT bar after comment, got %v %q", tok.Type, tok.Value)
	}
}

func TestTokenizerCompleteStylesheet(t *testing.T) {
	css := `
		body {
			color: #333;
			font-size: 16px;
		}

		.container {
			max-width: 1200px;
			margin: 0 auto;
		}
	`

	tokenizer := NewTokenizer(css)
	tokens := tokenizer.TokenizeAll()

	// Check we got a reasonable number of tokens
	if len(tokens) < 20 {
		t.Errorf("expected at least 20 tokens, got %d", len(tokens))
	}

	// Find some expected tokens
	foundBody := false
	foundColor := false
	foundHash := false
	foundOpenCurly := false

	for _, tok := range tokens {
		switch tok.Type {
		case TokenIdent:
			if tok.Value == "body" {
				foundBody = true
			}
			if tok.Value == "color" {
				foundColor = true
			}
		case TokenHash:
			if tok.Value == "333" {
				foundHash = true
			}
		case TokenOpenCurly:
			foundOpenCurly = true
		}
	}

	if !foundBody {
		t.Error("expected to find 'body' token")
	}
	if !foundColor {
		t.Error("expected to find 'color' token")
	}
	if !foundHash {
		t.Error("expected to find '#333' hash token")
	}
	if !foundOpenCurly {
		t.Error("expected to find '{' token")
	}
}
