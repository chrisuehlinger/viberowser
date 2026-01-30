// Package css provides CSS cascade and style computation.
// Reference: https://www.w3.org/TR/css-cascade-4/
package css

import (
	"sort"
	"strings"

	"github.com/AYColumbia/viberowser/dom"
)

// CascadeOrigin represents the origin of a stylesheet in the cascade.
type CascadeOrigin int

const (
	OriginUserAgent CascadeOrigin = iota
	OriginUser
	OriginAuthor
)

// MatchedRule represents a CSS rule that matches an element, along with
// metadata used for cascade ordering.
type MatchedRule struct {
	Rule        *Rule
	Selector    *ComplexSelector
	Origin      CascadeOrigin
	Important   bool
	Specificity Specificity
	Order       int // Source order (for stable sorting)
}

// StyleResolver resolves computed styles for elements using the CSS cascade.
type StyleResolver struct {
	userAgentSheet *Stylesheet
	userSheets     []*Stylesheet
	authorSheets   []*Stylesheet
}

// NewStyleResolver creates a new style resolver.
func NewStyleResolver() *StyleResolver {
	return &StyleResolver{}
}

// SetUserAgentStylesheet sets the user agent stylesheet.
func (sr *StyleResolver) SetUserAgentStylesheet(ss *Stylesheet) {
	sr.userAgentSheet = ss
}

// AddUserStylesheet adds a user stylesheet.
func (sr *StyleResolver) AddUserStylesheet(ss *Stylesheet) {
	sr.userSheets = append(sr.userSheets, ss)
}

// AddAuthorStylesheet adds an author stylesheet.
func (sr *StyleResolver) AddAuthorStylesheet(ss *Stylesheet) {
	sr.authorSheets = append(sr.authorSheets, ss)
}

// ClearAuthorStylesheets clears all author stylesheets.
func (sr *StyleResolver) ClearAuthorStylesheets() {
	sr.authorSheets = nil
}

// collectMatchingRules collects all rules matching an element.
func (sr *StyleResolver) collectMatchingRules(el *dom.Element) []MatchedRule {
	var matched []MatchedRule
	order := 0

	// Collect from user agent stylesheet
	if sr.userAgentSheet != nil {
		for _, rule := range sr.userAgentSheet.Rules {
			if matches, sel := matchRuleToElement(&rule, el); matches {
				for _, decl := range rule.Declarations {
					matched = append(matched, MatchedRule{
						Rule:        &rule,
						Selector:    sel,
						Origin:      OriginUserAgent,
						Important:   decl.Important,
						Specificity: sel.CalculateSpecificity(),
						Order:       order,
					})
				}
				order++
			}
		}
	}

	// Collect from user stylesheets
	for _, ss := range sr.userSheets {
		for _, rule := range ss.Rules {
			if matches, sel := matchRuleToElement(&rule, el); matches {
				for _, decl := range rule.Declarations {
					matched = append(matched, MatchedRule{
						Rule:        &rule,
						Selector:    sel,
						Origin:      OriginUser,
						Important:   decl.Important,
						Specificity: sel.CalculateSpecificity(),
						Order:       order,
					})
				}
				order++
			}
		}
	}

	// Collect from author stylesheets
	for _, ss := range sr.authorSheets {
		for _, rule := range ss.Rules {
			if matches, sel := matchRuleToElement(&rule, el); matches {
				for _, decl := range rule.Declarations {
					matched = append(matched, MatchedRule{
						Rule:        &rule,
						Selector:    sel,
						Origin:      OriginAuthor,
						Important:   decl.Important,
						Specificity: sel.CalculateSpecificity(),
						Order:       order,
					})
				}
				order++
			}
		}
	}

	return matched
}

// matchRuleToElement checks if a rule matches an element, returning the matching selector.
func matchRuleToElement(rule *Rule, el *dom.Element) (bool, *ComplexSelector) {
	// Parse selector from selector text
	sel, err := ParseSelector(rule.SelectorText)
	if err != nil || sel == nil {
		return false, nil
	}

	for _, cs := range sel.ComplexSelectors {
		if cs.MatchElement(el) {
			return true, cs
		}
	}
	return false, nil
}

// sortedByPrecedence sorts matched rules by cascade precedence.
// Order (highest to lowest):
// 1. Important user agent declarations
// 2. Important user declarations
// 3. Important author declarations
// 4. Normal author declarations
// 5. Normal user declarations
// 6. Normal user agent declarations
// Within each group, sort by specificity, then source order.
func sortByPrecedence(rules []MatchedRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		a, b := rules[i], rules[j]

		// Calculate cascade layer value
		aLayer := cascadeLayer(a.Origin, a.Important)
		bLayer := cascadeLayer(b.Origin, b.Important)

		if aLayer != bLayer {
			return aLayer < bLayer
		}

		// Same layer, compare by specificity
		cmp := a.Specificity.Compare(b.Specificity)
		if cmp != 0 {
			return cmp < 0
		}

		// Same specificity, use source order
		return a.Order < b.Order
	})
}

// cascadeLayer returns a numeric value for cascade ordering.
// Lower values have lower precedence.
func cascadeLayer(origin CascadeOrigin, important bool) int {
	if important {
		// Important declarations (inverted order)
		switch origin {
		case OriginAuthor:
			return 3
		case OriginUser:
			return 4
		case OriginUserAgent:
			return 5
		}
	} else {
		// Normal declarations
		switch origin {
		case OriginUserAgent:
			return 0
		case OriginUser:
			return 1
		case OriginAuthor:
			return 2
		}
	}
	return 0
}

// ComputedStyle represents the final computed style values for an element.
type ComputedStyle struct {
	// The element this style applies to
	element *dom.Element

	// Property values (property name -> computed value)
	values map[string]*ComputedValue

	// Parent computed style (for inheritance)
	parent *ComputedStyle
}

// ComputedValue represents a computed CSS value.
type ComputedValue struct {
	// The original declaration
	Value Value

	// Resolved values
	Length    float64 // For length values (in pixels)
	Color     Color   // For color values
	Keyword   string  // For keyword values
	IsInherit bool    // Whether this is the 'inherit' keyword
	IsInitial bool    // Whether this is the 'initial' keyword
	IsUnset   bool    // Whether this is the 'unset' keyword
	IsRevert  bool    // Whether this is the 'revert' keyword
}

// NewComputedStyle creates a new computed style for an element.
func NewComputedStyle(el *dom.Element, parent *ComputedStyle) *ComputedStyle {
	return &ComputedStyle{
		element: el,
		values:  make(map[string]*ComputedValue),
		parent:  parent,
	}
}

// GetPropertyValue returns the computed value for a property.
func (cs *ComputedStyle) GetPropertyValue(property string) *ComputedValue {
	property = strings.ToLower(property)
	return cs.values[property]
}

// SetPropertyValue sets a computed value for a property.
func (cs *ComputedStyle) SetPropertyValue(property string, value *ComputedValue) {
	property = strings.ToLower(property)
	cs.values[property] = value
}

// ResolveStyles computes the final style for an element.
func (sr *StyleResolver) ResolveStyles(el *dom.Element, parent *ComputedStyle) *ComputedStyle {
	computed := NewComputedStyle(el, parent)

	// Step 1: Apply default/initial values
	applyInitialValues(computed)

	// Step 2: Apply inherited properties from parent
	if parent != nil {
		applyInheritedProperties(computed, parent)
	}

	// Step 3: Collect all matching rules
	matched := sr.collectMatchingRules(el)

	// Step 4: Sort by cascade precedence
	sortByPrecedence(matched)

	// Step 5: Apply declarations in order (later declarations override earlier ones)
	for _, mr := range matched {
		for _, decl := range mr.Rule.Declarations {
			applyDeclaration(computed, &decl, parent)
		}
	}

	// Step 6: Parse and apply inline styles
	if el.HasAttribute("style") {
		inlineStyle := el.GetAttribute("style")
		applyInlineStyle(computed, inlineStyle, parent)
	}

	// Step 7: Compute relative values (em, rem, %, etc.)
	resolveRelativeValues(computed, parent)

	return computed
}

// applyInitialValues sets initial values for all properties.
func applyInitialValues(cs *ComputedStyle) {
	for prop, def := range PropertyDefaults {
		cs.values[prop] = &ComputedValue{
			Keyword:   def.InitialValue,
			IsInitial: true,
		}
	}
}

// applyInheritedProperties inherits values from parent.
func applyInheritedProperties(cs *ComputedStyle, parent *ComputedStyle) {
	for prop, def := range PropertyDefaults {
		if def.Inherited {
			if parentVal := parent.values[prop]; parentVal != nil {
				// Create a copy of the parent value
				val := *parentVal
				val.IsInherit = false // It's now the actual value
				cs.values[prop] = &val
			}
		}
	}
}

// applyDeclaration applies a single declaration to computed style.
func applyDeclaration(cs *ComputedStyle, decl *Declaration, parent *ComputedStyle) {
	prop := strings.ToLower(decl.Property)

	// Handle CSS-wide keywords
	switch strings.ToLower(decl.Value.Keyword) {
	case "inherit":
		if parent != nil {
			if parentVal := parent.values[prop]; parentVal != nil {
				val := *parentVal
				cs.values[prop] = &val
			}
		}
		return
	case "initial":
		if def, ok := PropertyDefaults[prop]; ok {
			cs.values[prop] = &ComputedValue{
				Keyword:   def.InitialValue,
				IsInitial: true,
			}
		}
		return
	case "unset":
		if def, ok := PropertyDefaults[prop]; ok {
			if def.Inherited && parent != nil {
				if parentVal := parent.values[prop]; parentVal != nil {
					val := *parentVal
					cs.values[prop] = &val
				}
			} else {
				cs.values[prop] = &ComputedValue{
					Keyword:   def.InitialValue,
					IsInitial: true,
				}
			}
		}
		return
	case "revert":
		// Revert to the cascade origin - for now, treat as unset
		if def, ok := PropertyDefaults[prop]; ok {
			cs.values[prop] = &ComputedValue{
				Keyword:   def.InitialValue,
				IsInitial: true,
			}
		}
		return
	}

	// Apply the value
	cs.values[prop] = computeValue(&decl.Value, prop)
}

// applyInlineStyle parses and applies inline style attribute.
func applyInlineStyle(cs *ComputedStyle, style string, parent *ComputedStyle) {
	// Parse inline style as declarations
	// Wrap in a block for the parser
	parser := NewCSSParser("{" + style + "}")
	// Consume the component values which will include the block
	cv := parser.consumeComponentValue()
	block, ok := cv.(*Block)
	if !ok || block == nil {
		return
	}

	declarations := ParseBlockContents(block)
	for _, decl := range declarations {
		legacyDecl := convertDeclaration(decl)
		applyDeclaration(cs, &legacyDecl, parent)
	}
}

// computeValue converts a CSS Value to a ComputedValue.
func computeValue(val *Value, property string) *ComputedValue {
	cv := &ComputedValue{
		Value: *val,
	}

	switch val.Type {
	case KeywordValue:
		cv.Keyword = val.Keyword
	case LengthValue:
		cv.Length = val.Length
	case ColorValue:
		cv.Color = val.Color
	case PercentageValue:
		cv.Length = val.Length // Will be resolved later
	case NumberValue:
		cv.Length = val.Length
	}

	return cv
}

// resolveRelativeValues resolves relative units to absolute values.
func resolveRelativeValues(cs *ComputedStyle, parent *ComputedStyle) {
	// Get the font-size for em calculations
	var fontSize float64 = 16 // Default
	if fs := cs.values["font-size"]; fs != nil {
		fontSize = fs.Length
		if fontSize == 0 {
			fontSize = 16
		}
	}

	// Get root font-size for rem calculations
	var rootFontSize float64 = 16 // Default
	rootStyle := cs
	for rootStyle.parent != nil {
		rootStyle = rootStyle.parent
	}
	if rfs := rootStyle.values["font-size"]; rfs != nil && rfs.Length > 0 {
		rootFontSize = rfs.Length
	}

	for prop, val := range cs.values {
		if val == nil {
			continue
		}

		switch val.Value.Type {
		case LengthValue:
			val.Length = resolveLength(val.Value.Length, val.Value.Unit, fontSize, rootFontSize)
		case PercentageValue:
			// Resolve percentages based on property
			val.Length = resolvePercentage(val.Value.Length, prop, parent)
		}
	}
}

// resolveLength converts a length value to pixels.
func resolveLength(value float64, unit string, fontSize, rootFontSize float64) float64 {
	switch strings.ToLower(unit) {
	case "px":
		return value
	case "em":
		return value * fontSize
	case "rem":
		return value * rootFontSize
	case "pt":
		return value * 96 / 72 // 1pt = 96/72 px
	case "pc":
		return value * 16 // 1pc = 16px
	case "in":
		return value * 96 // 1in = 96px
	case "cm":
		return value * 96 / 2.54 // 1cm = 96/2.54 px
	case "mm":
		return value * 96 / 25.4 // 1mm = 96/25.4 px
	case "q":
		return value * 96 / 101.6 // 1q = 96/101.6 px
	case "ex":
		return value * fontSize * 0.5 // Approximate ex as 0.5em
	case "ch":
		return value * fontSize * 0.5 // Approximate ch as 0.5em
	case "vw", "vh", "vmin", "vmax":
		// Viewport units need viewport size - use placeholder
		return value * 10 // Will be resolved properly when viewport is known
	default:
		return value
	}
}

// resolvePercentage resolves a percentage value based on property.
func resolvePercentage(percent float64, property string, parent *ComputedStyle) float64 {
	// Percentage resolution depends on the property
	switch property {
	case "font-size":
		if parent != nil {
			if pfs := parent.values["font-size"]; pfs != nil {
				return (percent / 100) * pfs.Length
			}
		}
		return (percent / 100) * 16
	case "width", "left", "right", "margin-left", "margin-right", "padding-left", "padding-right":
		// Percentage of containing block width - use placeholder
		return percent // Will be resolved during layout
	case "height", "top", "bottom", "margin-top", "margin-bottom", "padding-top", "padding-bottom":
		// Percentage of containing block height - use placeholder
		return percent // Will be resolved during layout
	case "line-height":
		// Percentage of font-size
		if parent != nil {
			if fs := parent.values["font-size"]; fs != nil {
				return (percent / 100) * fs.Length
			}
		}
		return percent
	default:
		return percent
	}
}

// PropertyDefault defines default values and inheritance for CSS properties.
type PropertyDefault struct {
	InitialValue string
	Inherited    bool
}

// PropertyDefaults contains default values for CSS properties.
var PropertyDefaults = map[string]PropertyDefault{
	// Box model
	"display":        {InitialValue: "inline", Inherited: false},
	"position":       {InitialValue: "static", Inherited: false},
	"float":          {InitialValue: "none", Inherited: false},
	"clear":          {InitialValue: "none", Inherited: false},
	"overflow":       {InitialValue: "visible", Inherited: false},
	"overflow-x":     {InitialValue: "visible", Inherited: false},
	"overflow-y":     {InitialValue: "visible", Inherited: false},
	"visibility":     {InitialValue: "visible", Inherited: true},
	"z-index":        {InitialValue: "auto", Inherited: false},
	"box-sizing":     {InitialValue: "content-box", Inherited: false},

	// Sizing
	"width":      {InitialValue: "auto", Inherited: false},
	"height":     {InitialValue: "auto", Inherited: false},
	"min-width":  {InitialValue: "0", Inherited: false},
	"min-height": {InitialValue: "0", Inherited: false},
	"max-width":  {InitialValue: "none", Inherited: false},
	"max-height": {InitialValue: "none", Inherited: false},

	// Margins
	"margin":        {InitialValue: "0", Inherited: false},
	"margin-top":    {InitialValue: "0", Inherited: false},
	"margin-right":  {InitialValue: "0", Inherited: false},
	"margin-bottom": {InitialValue: "0", Inherited: false},
	"margin-left":   {InitialValue: "0", Inherited: false},

	// Padding
	"padding":        {InitialValue: "0", Inherited: false},
	"padding-top":    {InitialValue: "0", Inherited: false},
	"padding-right":  {InitialValue: "0", Inherited: false},
	"padding-bottom": {InitialValue: "0", Inherited: false},
	"padding-left":   {InitialValue: "0", Inherited: false},

	// Borders
	"border":              {InitialValue: "none", Inherited: false},
	"border-width":        {InitialValue: "medium", Inherited: false},
	"border-top-width":    {InitialValue: "medium", Inherited: false},
	"border-right-width":  {InitialValue: "medium", Inherited: false},
	"border-bottom-width": {InitialValue: "medium", Inherited: false},
	"border-left-width":   {InitialValue: "medium", Inherited: false},
	"border-style":        {InitialValue: "none", Inherited: false},
	"border-top-style":    {InitialValue: "none", Inherited: false},
	"border-right-style":  {InitialValue: "none", Inherited: false},
	"border-bottom-style": {InitialValue: "none", Inherited: false},
	"border-left-style":   {InitialValue: "none", Inherited: false},
	"border-color":        {InitialValue: "currentcolor", Inherited: false},
	"border-top-color":    {InitialValue: "currentcolor", Inherited: false},
	"border-right-color":  {InitialValue: "currentcolor", Inherited: false},
	"border-bottom-color": {InitialValue: "currentcolor", Inherited: false},
	"border-left-color":   {InitialValue: "currentcolor", Inherited: false},
	"border-radius":       {InitialValue: "0", Inherited: false},

	// Positioning
	"top":    {InitialValue: "auto", Inherited: false},
	"right":  {InitialValue: "auto", Inherited: false},
	"bottom": {InitialValue: "auto", Inherited: false},
	"left":   {InitialValue: "auto", Inherited: false},

	// Text
	"color":           {InitialValue: "black", Inherited: true},
	"font-family":     {InitialValue: "serif", Inherited: true},
	"font-size":       {InitialValue: "medium", Inherited: true},
	"font-style":      {InitialValue: "normal", Inherited: true},
	"font-weight":     {InitialValue: "normal", Inherited: true},
	"font-variant":    {InitialValue: "normal", Inherited: true},
	"line-height":     {InitialValue: "normal", Inherited: true},
	"letter-spacing":  {InitialValue: "normal", Inherited: true},
	"word-spacing":    {InitialValue: "normal", Inherited: true},
	"text-align":      {InitialValue: "start", Inherited: true},
	"text-decoration": {InitialValue: "none", Inherited: false},
	"text-transform":  {InitialValue: "none", Inherited: true},
	"text-indent":     {InitialValue: "0", Inherited: true},
	"white-space":     {InitialValue: "normal", Inherited: true},
	"vertical-align":  {InitialValue: "baseline", Inherited: false},
	"direction":       {InitialValue: "ltr", Inherited: true},
	"unicode-bidi":    {InitialValue: "normal", Inherited: false},

	// Background
	"background":            {InitialValue: "transparent", Inherited: false},
	"background-color":      {InitialValue: "transparent", Inherited: false},
	"background-image":      {InitialValue: "none", Inherited: false},
	"background-repeat":     {InitialValue: "repeat", Inherited: false},
	"background-position":   {InitialValue: "0% 0%", Inherited: false},
	"background-attachment": {InitialValue: "scroll", Inherited: false},
	"background-size":       {InitialValue: "auto", Inherited: false},

	// Lists
	"list-style":          {InitialValue: "disc", Inherited: true},
	"list-style-type":     {InitialValue: "disc", Inherited: true},
	"list-style-position": {InitialValue: "outside", Inherited: true},
	"list-style-image":    {InitialValue: "none", Inherited: true},

	// Tables
	"table-layout":    {InitialValue: "auto", Inherited: false},
	"border-collapse": {InitialValue: "separate", Inherited: true},
	"border-spacing":  {InitialValue: "0", Inherited: true},
	"empty-cells":     {InitialValue: "show", Inherited: true},
	"caption-side":    {InitialValue: "top", Inherited: true},

	// Flexbox
	"flex-direction":  {InitialValue: "row", Inherited: false},
	"flex-wrap":       {InitialValue: "nowrap", Inherited: false},
	"justify-content": {InitialValue: "flex-start", Inherited: false},
	"align-items":     {InitialValue: "stretch", Inherited: false},
	"align-content":   {InitialValue: "stretch", Inherited: false},
	"flex-grow":       {InitialValue: "0", Inherited: false},
	"flex-shrink":     {InitialValue: "1", Inherited: false},
	"flex-basis":      {InitialValue: "auto", Inherited: false},
	"order":           {InitialValue: "0", Inherited: false},
	"align-self":      {InitialValue: "auto", Inherited: false},

	// Grid
	"grid-template-columns": {InitialValue: "none", Inherited: false},
	"grid-template-rows":    {InitialValue: "none", Inherited: false},
	"grid-column":           {InitialValue: "auto", Inherited: false},
	"grid-row":              {InitialValue: "auto", Inherited: false},
	"gap":                   {InitialValue: "0", Inherited: false},

	// Other
	"cursor":        {InitialValue: "auto", Inherited: true},
	"opacity":       {InitialValue: "1", Inherited: false},
	"content":       {InitialValue: "normal", Inherited: false},
	"quotes":        {InitialValue: "auto", Inherited: true},
	"counter-reset": {InitialValue: "none", Inherited: false},
	"outline":       {InitialValue: "none", Inherited: false},
}

// GetComputedStyleProperty is a helper to get a specific property value.
func (cs *ComputedStyle) GetComputedStyleProperty(property string) string {
	val := cs.GetPropertyValue(property)
	if val == nil {
		return ""
	}
	if val.Keyword != "" {
		return val.Keyword
	}
	if val.Value.Raw != "" {
		return val.Value.Raw
	}
	return ""
}

// GetLength returns the computed length value for a property in pixels.
func (cs *ComputedStyle) GetLength(property string) float64 {
	val := cs.GetPropertyValue(property)
	if val == nil {
		return 0
	}
	return val.Length
}

// GetColor returns the computed color value for a property.
func (cs *ComputedStyle) GetColor(property string) Color {
	val := cs.GetPropertyValue(property)
	if val == nil {
		return Color{}
	}
	return val.Color
}
