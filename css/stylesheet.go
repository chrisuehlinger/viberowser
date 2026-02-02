// Package css provides CSSStyleSheet and CSSOM APIs.
package css

import (
	"fmt"
	"strings"
)

// CSSStyleSheet represents a CSS stylesheet.
// Reference: https://drafts.csswg.org/cssom/#cssstylesheet
type CSSStyleSheet struct {
	// ownerNode is the DOM node that owns this stylesheet (style or link element)
	ownerNode interface{}

	// disabled indicates whether the stylesheet is disabled
	disabled bool

	// href is the location of the stylesheet (for linked stylesheets)
	href string

	// title is the advisory title
	title string

	// media is the media queries for the stylesheet
	media *MediaList

	// cssRules is the list of CSS rules in this stylesheet
	cssRules *CSSRuleList

	// ownerRule is the @import rule if this stylesheet was imported
	ownerRule *CSSImportRule

	// parentStyleSheet is the parent stylesheet (for imported stylesheets)
	parentStyleSheet *CSSStyleSheet

	// type is always "text/css"
	cssType string

	// The parsed stylesheet data
	parsed *ParsedStylesheet
}

// NewCSSStyleSheet creates a new CSSStyleSheet from CSS text.
func NewCSSStyleSheet(cssText string, ownerNode interface{}) *CSSStyleSheet {
	sheet := &CSSStyleSheet{
		ownerNode: ownerNode,
		disabled:  false,
		cssType:   "text/css",
		media:     NewMediaList(""),
	}

	// Parse the CSS text
	parser := NewCSSParser(cssText)
	sheet.parsed = parser.ParseStylesheet()

	// Convert parsed rules to CSSOM rules
	sheet.cssRules = NewCSSRuleList()
	for _, rule := range sheet.parsed.Rules {
		cssRule := sheet.createCSSRule(rule)
		if cssRule != nil {
			cssRule.SetParentStyleSheet(sheet)
			sheet.cssRules.rules = append(sheet.cssRules.rules, cssRule)
		}
	}

	return sheet
}

// createCSSRule creates an appropriate CSSRule from a parsed rule.
func (s *CSSStyleSheet) createCSSRule(parsed CSSRule) CSSRuleInterface {
	switch r := parsed.(type) {
	case *QualifiedRule:
		return s.createStyleRule(r)
	case *AtRule:
		return s.createAtRule(r)
	default:
		return nil
	}
}

// createStyleRule creates a CSSStyleRule from a QualifiedRule.
func (s *CSSStyleSheet) createStyleRule(qr *QualifiedRule) *CSSStyleRule {
	rule := &CSSStyleRule{
		baseCSSRule: baseCSSRule{ruleType: StyleRule},
	}

	// Build selector text from prelude
	var selectorText strings.Builder
	writeComponentValue(&selectorText, qr.Prelude)
	rule.selectorText = strings.TrimSpace(selectorText.String())

	// Parse declarations from block
	rule.style = NewCSSStyleDeclarationFromBlock(qr.Block, rule)

	return rule
}

// createAtRule creates an appropriate at-rule from an AtRule.
func (s *CSSStyleSheet) createAtRule(ar *AtRule) CSSRuleInterface {
	name := strings.ToLower(ar.Name)
	switch name {
	case "keyframes", "-webkit-keyframes":
		return s.createKeyframesRule(ar)
	case "media":
		return s.createMediaRule(ar)
	case "import":
		return s.createImportRule(ar)
	case "font-face":
		return s.createFontFaceRule(ar)
	case "namespace":
		return s.createNamespaceRule(ar)
	case "supports":
		return s.createSupportsRule(ar)
	default:
		// Return a generic at-rule
		return &CSSGenericAtRule{
			baseCSSRule: baseCSSRule{ruleType: UnknownRule},
			name:        ar.Name,
		}
	}
}

// createKeyframesRule creates a CSSKeyframesRule.
func (s *CSSStyleSheet) createKeyframesRule(ar *AtRule) *CSSKeyframesRule {
	rule := &CSSKeyframesRule{
		baseCSSRule:  baseCSSRule{ruleType: KeyframesRule},
		keyframeList: make([]*CSSKeyframeRule, 0),
	}

	// Get animation name from prelude
	for _, cv := range ar.Prelude {
		if pt, ok := cv.(PreservedToken); ok && pt.Token.Type == TokenIdent {
			rule.name = pt.Token.Value
			break
		}
	}

	// Parse keyframe rules from block
	if ar.Block != nil {
		// Re-parse block contents as rules
		blockParser := &CSSParser{
			tokens: componentValuesToTokens(ar.Block.Values),
			pos:    0,
		}
		for {
			if blockParser.current().Type == TokenEOF {
				break
			}
			blockParser.skipWhitespace()
			if blockParser.current().Type == TokenEOF {
				break
			}

			qr := blockParser.consumeQualifiedRule()
			if qr != nil {
				keyframe := s.createKeyframeRule(qr)
				if keyframe != nil {
					keyframe.SetParentRule(rule)
					rule.keyframeList = append(rule.keyframeList, keyframe)
				}
			}
		}
	}

	return rule
}

// createKeyframeRule creates a CSSKeyframeRule from a QualifiedRule.
func (s *CSSStyleSheet) createKeyframeRule(qr *QualifiedRule) *CSSKeyframeRule {
	rule := &CSSKeyframeRule{
		baseCSSRule: baseCSSRule{ruleType: KeyframeRule},
	}

	// Build keyText from prelude (e.g., "0%", "from", "to", "50%")
	var keyText strings.Builder
	writeComponentValue(&keyText, qr.Prelude)
	rule.keyText = strings.TrimSpace(keyText.String())

	// Parse declarations from block
	rule.style = NewCSSStyleDeclarationFromBlock(qr.Block, rule)

	return rule
}

// createMediaRule creates a CSSMediaRule.
func (s *CSSStyleSheet) createMediaRule(ar *AtRule) *CSSMediaRule {
	rule := &CSSMediaRule{
		baseCSSRule: baseCSSRule{ruleType: MediaRule},
		cssRules:    NewCSSRuleList(),
	}

	// Build media query text from prelude
	var mediaText strings.Builder
	writeComponentValue(&mediaText, ar.Prelude)
	rule.media = NewMediaList(strings.TrimSpace(mediaText.String()))

	// Parse nested rules from block
	if ar.Block != nil {
		blockParser := &CSSParser{
			tokens: componentValuesToTokens(ar.Block.Values),
			pos:    0,
		}
		nestedRules := blockParser.consumeRuleList(false)
		for _, nested := range nestedRules {
			cssRule := s.createCSSRule(nested)
			if cssRule != nil {
				cssRule.SetParentStyleSheet(s)
				cssRule.SetParentRule(rule)
				rule.cssRules.rules = append(rule.cssRules.rules, cssRule)
			}
		}
	}

	return rule
}

// createImportRule creates a CSSImportRule.
func (s *CSSStyleSheet) createImportRule(ar *AtRule) *CSSImportRule {
	rule := &CSSImportRule{
		baseCSSRule: baseCSSRule{ruleType: ImportRule},
	}

	// Parse href and media from prelude
	for _, cv := range ar.Prelude {
		switch v := cv.(type) {
		case PreservedToken:
			if v.Token.Type == TokenString || v.Token.Type == TokenURL {
				rule.href = v.Token.Value
			}
		case *Function:
			if strings.ToLower(v.Name) == "url" {
				for _, fcv := range v.Values {
					if pt, ok := fcv.(PreservedToken); ok {
						if pt.Token.Type == TokenString {
							rule.href = pt.Token.Value
							break
						}
					}
				}
			}
		}
	}

	// Media defaults to all
	rule.media = NewMediaList("")

	return rule
}

// createFontFaceRule creates a CSSFontFaceRule.
func (s *CSSStyleSheet) createFontFaceRule(ar *AtRule) *CSSFontFaceRule {
	rule := &CSSFontFaceRule{
		baseCSSRule: baseCSSRule{ruleType: FontFaceRule},
	}

	// Parse declarations from block
	rule.style = NewCSSStyleDeclarationFromBlock(ar.Block, rule)

	return rule
}

// createNamespaceRule creates a CSSNamespaceRule.
func (s *CSSStyleSheet) createNamespaceRule(ar *AtRule) *CSSNamespaceRule {
	rule := &CSSNamespaceRule{
		baseCSSRule: baseCSSRule{ruleType: NamespaceRule},
	}

	// Parse prefix and namespace URI from prelude
	for _, cv := range ar.Prelude {
		if pt, ok := cv.(PreservedToken); ok {
			switch pt.Token.Type {
			case TokenIdent:
				if rule.prefix == "" {
					rule.prefix = pt.Token.Value
				}
			case TokenString, TokenURL:
				rule.namespaceURI = pt.Token.Value
			}
		}
	}

	return rule
}

// createSupportsRule creates a CSSSupportsRule.
func (s *CSSStyleSheet) createSupportsRule(ar *AtRule) *CSSSupportsRule {
	rule := &CSSSupportsRule{
		baseCSSRule: baseCSSRule{ruleType: SupportsRule},
		cssRules:    NewCSSRuleList(),
	}

	// Build condition text from prelude
	var condText strings.Builder
	writeComponentValue(&condText, ar.Prelude)
	rule.conditionText = strings.TrimSpace(condText.String())

	// Parse nested rules from block
	if ar.Block != nil {
		blockParser := &CSSParser{
			tokens: componentValuesToTokens(ar.Block.Values),
			pos:    0,
		}
		nestedRules := blockParser.consumeRuleList(false)
		for _, nested := range nestedRules {
			cssRule := s.createCSSRule(nested)
			if cssRule != nil {
				cssRule.SetParentStyleSheet(s)
				cssRule.SetParentRule(rule)
				rule.cssRules.rules = append(rule.cssRules.rules, cssRule)
			}
		}
	}

	return rule
}

// componentValuesToTokens converts component values to tokens.
func componentValuesToTokens(cvs []ComponentValue) []Token {
	var tokens []Token
	for _, cv := range cvs {
		tokens = append(tokens, componentValueToTokens(cv)...)
	}
	return tokens
}

// OwnerNode returns the owner node of this stylesheet.
func (s *CSSStyleSheet) OwnerNode() interface{} {
	return s.ownerNode
}

// Disabled returns whether the stylesheet is disabled.
func (s *CSSStyleSheet) Disabled() bool {
	return s.disabled
}

// SetDisabled sets whether the stylesheet is disabled.
func (s *CSSStyleSheet) SetDisabled(disabled bool) {
	s.disabled = disabled
}

// Href returns the location of the stylesheet.
func (s *CSSStyleSheet) Href() string {
	return s.href
}

// SetHref sets the location of the stylesheet.
func (s *CSSStyleSheet) SetHref(href string) {
	s.href = href
}

// Title returns the advisory title.
func (s *CSSStyleSheet) Title() string {
	return s.title
}

// Media returns the media queries.
func (s *CSSStyleSheet) Media() *MediaList {
	return s.media
}

// CSSRules returns the list of CSS rules.
func (s *CSSStyleSheet) CSSRules() *CSSRuleList {
	return s.cssRules
}

// OwnerRule returns the @import rule if this was imported.
func (s *CSSStyleSheet) OwnerRule() *CSSImportRule {
	return s.ownerRule
}

// ParentStyleSheet returns the parent stylesheet.
func (s *CSSStyleSheet) ParentStyleSheet() *CSSStyleSheet {
	return s.parentStyleSheet
}

// Type returns the stylesheet type (always "text/css").
func (s *CSSStyleSheet) Type() string {
	return s.cssType
}

// InsertRule inserts a rule at the given index and returns the index.
func (s *CSSStyleSheet) InsertRule(ruleText string, index int) (int, error) {
	// Parse the rule
	parser := NewCSSParser(ruleText)
	parser.skipWhitespace()

	var parsed CSSRule
	if parser.current().Type == TokenAtKeyword {
		parsed = parser.consumeAtRule()
	} else {
		parsed = parser.consumeQualifiedRule()
	}

	if parsed == nil {
		return 0, fmt.Errorf("SyntaxError: invalid rule")
	}

	cssRule := s.createCSSRule(parsed)
	if cssRule == nil {
		return 0, fmt.Errorf("SyntaxError: invalid rule")
	}

	// Validate index
	if index < 0 || index > len(s.cssRules.rules) {
		return 0, fmt.Errorf("IndexSizeError: index out of bounds")
	}

	// Insert the rule
	cssRule.SetParentStyleSheet(s)
	rules := make([]CSSRuleInterface, 0, len(s.cssRules.rules)+1)
	rules = append(rules, s.cssRules.rules[:index]...)
	rules = append(rules, cssRule)
	rules = append(rules, s.cssRules.rules[index:]...)
	s.cssRules.rules = rules

	return index, nil
}

// DeleteRule removes the rule at the given index.
func (s *CSSStyleSheet) DeleteRule(index int) error {
	if index < 0 || index >= len(s.cssRules.rules) {
		return fmt.Errorf("IndexSizeError: index out of bounds")
	}

	s.cssRules.rules = append(s.cssRules.rules[:index], s.cssRules.rules[index+1:]...)
	return nil
}

// CSSText returns the serialized stylesheet.
func (s *CSSStyleSheet) CSSText() string {
	var sb strings.Builder
	for i, rule := range s.cssRules.rules {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(rule.CSSText())
	}
	return sb.String()
}

// MediaList represents a list of media queries.
type MediaList struct {
	mediaText string
	queries   []string
}

// NewMediaList creates a new MediaList from media text.
func NewMediaList(mediaText string) *MediaList {
	ml := &MediaList{mediaText: mediaText}
	if mediaText != "" {
		ml.queries = strings.Split(mediaText, ",")
		for i := range ml.queries {
			ml.queries[i] = strings.TrimSpace(ml.queries[i])
		}
	}
	return ml
}

// MediaText returns the serialized media queries.
func (ml *MediaList) MediaText() string {
	return ml.mediaText
}

// SetMediaText sets the media queries.
func (ml *MediaList) SetMediaText(text string) {
	ml.mediaText = text
	ml.queries = strings.Split(text, ",")
	for i := range ml.queries {
		ml.queries[i] = strings.TrimSpace(ml.queries[i])
	}
}

// Length returns the number of media queries.
func (ml *MediaList) Length() int {
	if ml.mediaText == "" {
		return 0
	}
	return len(ml.queries)
}

// Item returns the media query at the given index.
func (ml *MediaList) Item(index int) string {
	if index < 0 || index >= len(ml.queries) {
		return ""
	}
	return ml.queries[index]
}

// AppendMedium adds a media query.
func (ml *MediaList) AppendMedium(medium string) {
	ml.queries = append(ml.queries, medium)
	ml.mediaText = strings.Join(ml.queries, ", ")
}

// DeleteMedium removes a media query.
func (ml *MediaList) DeleteMedium(medium string) {
	for i, q := range ml.queries {
		if q == medium {
			ml.queries = append(ml.queries[:i], ml.queries[i+1:]...)
			ml.mediaText = strings.Join(ml.queries, ", ")
			return
		}
	}
}
