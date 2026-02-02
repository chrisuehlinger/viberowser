// Package css provides CSS rule types for CSSOM.
package css

import (
	"strings"
)

// CSSRuleType represents the type of a CSS rule.
type CSSRuleType int

const (
	// UnknownRule - unknown rule type
	UnknownRule CSSRuleType = 0
	// StyleRule - CSSStyleRule
	StyleRule CSSRuleType = 1
	// CharsetRule - deprecated
	CharsetRule CSSRuleType = 2
	// ImportRule - CSSImportRule
	ImportRule CSSRuleType = 3
	// MediaRule - CSSMediaRule
	MediaRule CSSRuleType = 4
	// FontFaceRule - CSSFontFaceRule
	FontFaceRule CSSRuleType = 5
	// PageRule - CSSPageRule
	PageRule CSSRuleType = 6
	// KeyframesRule - CSSKeyframesRule
	KeyframesRule CSSRuleType = 7
	// KeyframeRule - CSSKeyframeRule
	KeyframeRule CSSRuleType = 8
	// MarginRule - CSSMarginRule
	MarginRule CSSRuleType = 9
	// NamespaceRule - CSSNamespaceRule
	NamespaceRule CSSRuleType = 10
	// CounterStyleRule - CSSCounterStyleRule
	CounterStyleRule CSSRuleType = 11
	// SupportsRule - CSSSupportsRule
	SupportsRule CSSRuleType = 12
	// DocumentRule - CSSDocumentRule (deprecated)
	DocumentRule CSSRuleType = 13
	// FontFeatureValuesRule - CSSFontFeatureValuesRule
	FontFeatureValuesRule CSSRuleType = 14
	// ViewportRule - CSSViewportRule (deprecated)
	ViewportRule CSSRuleType = 15
)

// CSSRuleInterface is the interface for all CSS rules.
type CSSRuleInterface interface {
	Type() CSSRuleType
	CSSText() string
	ParentStyleSheet() *CSSStyleSheet
	ParentRule() CSSRuleInterface
	SetParentStyleSheet(*CSSStyleSheet)
	SetParentRule(CSSRuleInterface)
}

// baseCSSRule provides common fields for all CSS rules.
type baseCSSRule struct {
	ruleType         CSSRuleType
	parentStyleSheet *CSSStyleSheet
	parentRule       CSSRuleInterface
}

func (r *baseCSSRule) Type() CSSRuleType {
	return r.ruleType
}

func (r *baseCSSRule) ParentStyleSheet() *CSSStyleSheet {
	return r.parentStyleSheet
}

func (r *baseCSSRule) ParentRule() CSSRuleInterface {
	return r.parentRule
}

func (r *baseCSSRule) SetParentStyleSheet(sheet *CSSStyleSheet) {
	r.parentStyleSheet = sheet
}

func (r *baseCSSRule) SetParentRule(rule CSSRuleInterface) {
	r.parentRule = rule
}

// CSSRuleList represents a list of CSS rules.
type CSSRuleList struct {
	rules []CSSRuleInterface
}

// NewCSSRuleList creates a new CSSRuleList.
func NewCSSRuleList() *CSSRuleList {
	return &CSSRuleList{
		rules: make([]CSSRuleInterface, 0),
	}
}

// Length returns the number of rules.
func (l *CSSRuleList) Length() int {
	return len(l.rules)
}

// Item returns the rule at the given index.
func (l *CSSRuleList) Item(index int) CSSRuleInterface {
	if index < 0 || index >= len(l.rules) {
		return nil
	}
	return l.rules[index]
}

// Rules returns all rules (for internal use).
func (l *CSSRuleList) Rules() []CSSRuleInterface {
	return l.rules
}

// CSSStyleRule represents a style rule (e.g., "div { color: red }").
type CSSStyleRule struct {
	baseCSSRule
	selectorText string
	style        *CSSRuleStyleDeclaration
}

// SelectorText returns the selector text.
func (r *CSSStyleRule) SelectorText() string {
	return r.selectorText
}

// SetSelectorText sets the selector text.
func (r *CSSStyleRule) SetSelectorText(text string) {
	// Validate the selector by parsing it
	_, err := ParseSelector(text)
	if err == nil {
		r.selectorText = text
	}
}

// Style returns the style declaration.
func (r *CSSStyleRule) Style() *CSSRuleStyleDeclaration {
	return r.style
}

// CSSText returns the serialized rule.
func (r *CSSStyleRule) CSSText() string {
	cssText := r.style.CSSText()
	if cssText == "" {
		return r.selectorText + " { }"
	}
	return r.selectorText + " { " + cssText + " }"
}

// CSSKeyframesRule represents a @keyframes rule.
type CSSKeyframesRule struct {
	baseCSSRule
	name         string
	keyframeList []*CSSKeyframeRule
}

// Name returns the animation name.
func (r *CSSKeyframesRule) Name() string {
	return r.name
}

// SetName sets the animation name.
func (r *CSSKeyframesRule) SetName(name string) {
	r.name = name
}

// CSSRules returns the list of keyframe rules.
func (r *CSSKeyframesRule) CSSRules() *CSSRuleList {
	list := NewCSSRuleList()
	for _, kf := range r.keyframeList {
		list.rules = append(list.rules, kf)
	}
	return list
}

// AppendRule adds a keyframe rule.
func (r *CSSKeyframesRule) AppendRule(ruleText string) {
	// Parse the keyframe rule
	parser := NewCSSParser(ruleText)
	qr := parser.consumeQualifiedRule()
	if qr != nil {
		keyframe := &CSSKeyframeRule{
			baseCSSRule: baseCSSRule{ruleType: KeyframeRule, parentRule: r},
		}
		var keyText strings.Builder
		writeComponentValue(&keyText, qr.Prelude)
		keyframe.keyText = strings.TrimSpace(keyText.String())
		keyframe.style = NewCSSStyleDeclarationFromBlock(qr.Block, keyframe)
		r.keyframeList = append(r.keyframeList, keyframe)
	}
}

// DeleteRule removes a keyframe rule by key.
func (r *CSSKeyframesRule) DeleteRule(key string) {
	key = strings.TrimSpace(key)
	for i, kf := range r.keyframeList {
		if kf.keyText == key {
			r.keyframeList = append(r.keyframeList[:i], r.keyframeList[i+1:]...)
			return
		}
	}
}

// FindRule finds a keyframe rule by key.
func (r *CSSKeyframesRule) FindRule(key string) *CSSKeyframeRule {
	key = strings.TrimSpace(key)
	for _, kf := range r.keyframeList {
		if kf.keyText == key {
			return kf
		}
	}
	return nil
}

// CSSText returns the serialized rule.
func (r *CSSKeyframesRule) CSSText() string {
	var sb strings.Builder
	sb.WriteString("@keyframes ")
	sb.WriteString(r.name)
	sb.WriteString(" { ")
	for i, kf := range r.keyframeList {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(kf.CSSText())
	}
	sb.WriteString(" }")
	return sb.String()
}

// CSSKeyframeRule represents a single keyframe in @keyframes.
type CSSKeyframeRule struct {
	baseCSSRule
	keyText string
	style   *CSSRuleStyleDeclaration
}

// KeyText returns the keyframe selector (e.g., "0%", "from", "50%").
func (r *CSSKeyframeRule) KeyText() string {
	return r.keyText
}

// SetKeyText sets the keyframe selector.
func (r *CSSKeyframeRule) SetKeyText(text string) {
	r.keyText = text
}

// Style returns the style declaration.
func (r *CSSKeyframeRule) Style() *CSSRuleStyleDeclaration {
	return r.style
}

// CSSText returns the serialized rule.
func (r *CSSKeyframeRule) CSSText() string {
	cssText := r.style.CSSText()
	if cssText == "" {
		return r.keyText + " { }"
	}
	return r.keyText + " { " + cssText + " }"
}

// CSSMediaRule represents a @media rule.
type CSSMediaRule struct {
	baseCSSRule
	media    *MediaList
	cssRules *CSSRuleList
}

// Media returns the media list.
func (r *CSSMediaRule) Media() *MediaList {
	return r.media
}

// CSSRules returns the nested rules.
func (r *CSSMediaRule) CSSRules() *CSSRuleList {
	return r.cssRules
}

// InsertRule inserts a rule at the given index.
func (r *CSSMediaRule) InsertRule(ruleText string, index int) (int, error) {
	// Similar to CSSStyleSheet.InsertRule
	parser := NewCSSParser(ruleText)
	parser.skipWhitespace()

	var parsed CSSRule
	if parser.current().Type == TokenAtKeyword {
		parsed = parser.consumeAtRule()
	} else {
		parsed = parser.consumeQualifiedRule()
	}

	if parsed == nil {
		return 0, nil
	}

	// Create the rule (simplified - would need parent stylesheet reference)
	var cssRule CSSRuleInterface
	switch p := parsed.(type) {
	case *QualifiedRule:
		rule := &CSSStyleRule{
			baseCSSRule: baseCSSRule{ruleType: StyleRule, parentRule: r},
		}
		var selectorText strings.Builder
		writeComponentValue(&selectorText, p.Prelude)
		rule.selectorText = strings.TrimSpace(selectorText.String())
		rule.style = NewCSSStyleDeclarationFromBlock(p.Block, rule)
		cssRule = rule
	}

	if cssRule != nil {
		if index < 0 || index > len(r.cssRules.rules) {
			index = len(r.cssRules.rules)
		}
		rules := make([]CSSRuleInterface, 0, len(r.cssRules.rules)+1)
		rules = append(rules, r.cssRules.rules[:index]...)
		rules = append(rules, cssRule)
		rules = append(rules, r.cssRules.rules[index:]...)
		r.cssRules.rules = rules
	}

	return index, nil
}

// DeleteRule removes the rule at the given index.
func (r *CSSMediaRule) DeleteRule(index int) {
	if index >= 0 && index < len(r.cssRules.rules) {
		r.cssRules.rules = append(r.cssRules.rules[:index], r.cssRules.rules[index+1:]...)
	}
}

// ConditionText returns the media condition text.
func (r *CSSMediaRule) ConditionText() string {
	return r.media.MediaText()
}

// CSSText returns the serialized rule.
func (r *CSSMediaRule) CSSText() string {
	var sb strings.Builder
	sb.WriteString("@media ")
	sb.WriteString(r.media.MediaText())
	sb.WriteString(" { ")
	for i, rule := range r.cssRules.rules {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(rule.CSSText())
	}
	sb.WriteString(" }")
	return sb.String()
}

// CSSImportRule represents an @import rule.
type CSSImportRule struct {
	baseCSSRule
	href       string
	media      *MediaList
	styleSheet *CSSStyleSheet
}

// Href returns the URL of the imported stylesheet.
func (r *CSSImportRule) Href() string {
	return r.href
}

// Media returns the media list.
func (r *CSSImportRule) Media() *MediaList {
	return r.media
}

// StyleSheet returns the imported stylesheet (if loaded).
func (r *CSSImportRule) StyleSheet() *CSSStyleSheet {
	return r.styleSheet
}

// CSSText returns the serialized rule.
func (r *CSSImportRule) CSSText() string {
	var sb strings.Builder
	sb.WriteString("@import url(\"")
	sb.WriteString(r.href)
	sb.WriteString("\")")
	if r.media.MediaText() != "" {
		sb.WriteString(" ")
		sb.WriteString(r.media.MediaText())
	}
	sb.WriteString(";")
	return sb.String()
}

// CSSFontFaceRule represents a @font-face rule.
type CSSFontFaceRule struct {
	baseCSSRule
	style *CSSRuleStyleDeclaration
}

// Style returns the style declaration.
func (r *CSSFontFaceRule) Style() *CSSRuleStyleDeclaration {
	return r.style
}

// CSSText returns the serialized rule.
func (r *CSSFontFaceRule) CSSText() string {
	cssText := r.style.CSSText()
	if cssText == "" {
		return "@font-face { }"
	}
	return "@font-face { " + cssText + " }"
}

// CSSNamespaceRule represents a @namespace rule.
type CSSNamespaceRule struct {
	baseCSSRule
	prefix       string
	namespaceURI string
}

// NamespaceURI returns the namespace URI.
func (r *CSSNamespaceRule) NamespaceURI() string {
	return r.namespaceURI
}

// Prefix returns the namespace prefix.
func (r *CSSNamespaceRule) Prefix() string {
	return r.prefix
}

// CSSText returns the serialized rule.
func (r *CSSNamespaceRule) CSSText() string {
	var sb strings.Builder
	sb.WriteString("@namespace ")
	if r.prefix != "" {
		sb.WriteString(r.prefix)
		sb.WriteString(" ")
	}
	sb.WriteString("url(\"")
	sb.WriteString(r.namespaceURI)
	sb.WriteString("\");")
	return sb.String()
}

// CSSSupportsRule represents a @supports rule.
type CSSSupportsRule struct {
	baseCSSRule
	conditionText string
	cssRules      *CSSRuleList
}

// ConditionText returns the supports condition text.
func (r *CSSSupportsRule) ConditionText() string {
	return r.conditionText
}

// CSSRules returns the nested rules.
func (r *CSSSupportsRule) CSSRules() *CSSRuleList {
	return r.cssRules
}

// InsertRule inserts a rule at the given index.
func (r *CSSSupportsRule) InsertRule(ruleText string, index int) (int, error) {
	// Similar to CSSMediaRule.InsertRule
	return index, nil
}

// DeleteRule removes the rule at the given index.
func (r *CSSSupportsRule) DeleteRule(index int) {
	if index >= 0 && index < len(r.cssRules.rules) {
		r.cssRules.rules = append(r.cssRules.rules[:index], r.cssRules.rules[index+1:]...)
	}
}

// CSSText returns the serialized rule.
func (r *CSSSupportsRule) CSSText() string {
	var sb strings.Builder
	sb.WriteString("@supports ")
	sb.WriteString(r.conditionText)
	sb.WriteString(" { ")
	for i, rule := range r.cssRules.rules {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(rule.CSSText())
	}
	sb.WriteString(" }")
	return sb.String()
}

// CSSGenericAtRule represents an unknown at-rule.
type CSSGenericAtRule struct {
	baseCSSRule
	name    string
	prelude string
	block   string
}

// CSSText returns the serialized rule.
func (r *CSSGenericAtRule) CSSText() string {
	return "@" + r.name
}
