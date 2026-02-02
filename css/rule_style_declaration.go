// Package css provides CSSRuleStyleDeclaration for rule-based style declarations.
package css

import (
	"regexp"
	"sort"
	"strings"
)

// CSSRuleStyleDeclaration represents a style declaration within a CSS rule.
// Unlike inline style declarations, these belong to a parent rule.
type CSSRuleStyleDeclaration struct {
	// The parent rule this style declaration belongs to
	parentRule CSSRuleInterface

	// Parsed declarations (property name -> declaration)
	declarations map[string]*ruleStyleProperty

	// Order in which properties were set (for cssText serialization)
	propertyOrder []string
}

// ruleStyleProperty holds a single CSS property's value and priority.
type ruleStyleProperty struct {
	value    string
	priority string // "important" or ""
}

// NewCSSRuleStyleDeclaration creates a new CSSRuleStyleDeclaration for a rule.
func NewCSSRuleStyleDeclaration(parentRule CSSRuleInterface) *CSSRuleStyleDeclaration {
	return &CSSRuleStyleDeclaration{
		parentRule:   parentRule,
		declarations: make(map[string]*ruleStyleProperty),
	}
}

// NewCSSStyleDeclarationFromBlock creates a style declaration from a parsed block.
func NewCSSStyleDeclarationFromBlock(block *Block, parentRule CSSRuleInterface) *CSSRuleStyleDeclaration {
	sd := NewCSSRuleStyleDeclaration(parentRule)
	if block == nil {
		return sd
	}

	// Parse declarations from block
	declarations := ParseBlockContents(block)
	for _, decl := range declarations {
		property := normalizeRuleCSSPropertyName(decl.Property)
		if property == "" {
			continue
		}

		// Build value string
		var valueStr strings.Builder
		for _, cv := range decl.Value {
			switch v := cv.(type) {
			case PreservedToken:
				switch v.Token.Type {
				case TokenIdent:
					valueStr.WriteString(v.Token.Value)
				case TokenNumber:
					valueStr.WriteString(v.Token.Value)
				case TokenPercentage:
					valueStr.WriteString(v.Token.Value)
					valueStr.WriteString("%")
				case TokenDimension:
					valueStr.WriteString(v.Token.Value)
					valueStr.WriteString(v.Token.Unit)
				case TokenString:
					valueStr.WriteString("\"")
					valueStr.WriteString(v.Token.Value)
					valueStr.WriteString("\"")
				case TokenHash:
					valueStr.WriteString("#")
					valueStr.WriteString(v.Token.Value)
				case TokenWhitespace:
					valueStr.WriteString(" ")
				case TokenDelim:
					valueStr.WriteRune(v.Token.Delim)
				case TokenComma:
					valueStr.WriteString(",")
				case TokenURL:
					valueStr.WriteString("url(")
					valueStr.WriteString(v.Token.Value)
					valueStr.WriteString(")")
				}
			case *Function:
				valueStr.WriteString(v.Name)
				valueStr.WriteString("(")
				// Recursively add function values
				for i, fcv := range v.Values {
					if i > 0 {
						valueStr.WriteString(" ")
					}
					valueStr.WriteString(fcv.String())
				}
				valueStr.WriteString(")")
			}
		}

		value := strings.TrimSpace(valueStr.String())
		if value == "" {
			continue
		}

		priority := ""
		if decl.Important {
			priority = "important"
		}

		if _, exists := sd.declarations[property]; !exists {
			sd.propertyOrder = append(sd.propertyOrder, property)
		}
		sd.declarations[property] = &ruleStyleProperty{
			value:    value,
			priority: priority,
		}
	}

	return sd
}

// CSSText returns the textual representation of the declaration block.
func (sd *CSSRuleStyleDeclaration) CSSText() string {
	if len(sd.declarations) == 0 {
		return ""
	}

	var parts []string
	for _, prop := range sd.propertyOrder {
		if sp, ok := sd.declarations[prop]; ok {
			part := prop + ": " + sp.value
			if sp.priority == "important" {
				part += " !important"
			}
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

// SetCSSText parses and sets all properties from a CSS text string.
func (sd *CSSRuleStyleDeclaration) SetCSSText(cssText string) {
	sd.declarations = make(map[string]*ruleStyleProperty)
	sd.propertyOrder = nil
	sd.parseFromText(cssText)
}

// parseFromText parses a style text string into declarations.
func (sd *CSSRuleStyleDeclaration) parseFromText(styleText string) {
	// Split by semicolons
	parts := strings.Split(styleText, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by first colon
		colonIdx := strings.Index(part, ":")
		if colonIdx == -1 {
			continue
		}

		property := strings.TrimSpace(part[:colonIdx])
		value := strings.TrimSpace(part[colonIdx+1:])

		if property == "" || value == "" {
			continue
		}

		// Check for !important
		priority := ""
		lowerValue := strings.ToLower(value)
		if strings.HasSuffix(lowerValue, "!important") {
			priority = "important"
			value = strings.TrimSpace(value[:len(value)-len("!important")])
		} else if strings.Contains(lowerValue, "!") {
			// Handle "value ! important" format
			importantIdx := strings.LastIndex(lowerValue, "!")
			rest := strings.TrimSpace(lowerValue[importantIdx+1:])
			if rest == "important" {
				priority = "important"
				value = strings.TrimSpace(value[:importantIdx])
			}
		}

		property = normalizeRuleCSSPropertyName(property)
		if property != "" {
			if _, exists := sd.declarations[property]; !exists {
				sd.propertyOrder = append(sd.propertyOrder, property)
			}
			sd.declarations[property] = &ruleStyleProperty{
				value:    value,
				priority: priority,
			}
		}
	}
}

// Length returns the number of properties set.
func (sd *CSSRuleStyleDeclaration) Length() int {
	return len(sd.declarations)
}

// Item returns the property name at the given index.
func (sd *CSSRuleStyleDeclaration) Item(index int) string {
	if index < 0 || index >= len(sd.propertyOrder) {
		return ""
	}
	return sd.propertyOrder[index]
}

// GetPropertyValue returns the value of a CSS property.
func (sd *CSSRuleStyleDeclaration) GetPropertyValue(property string) string {
	property = normalizeRuleCSSPropertyName(property)
	if sp, ok := sd.declarations[property]; ok {
		return sp.value
	}
	return ""
}

// GetPropertyPriority returns the priority of a CSS property ("important" or "").
func (sd *CSSRuleStyleDeclaration) GetPropertyPriority(property string) string {
	property = normalizeRuleCSSPropertyName(property)
	if sp, ok := sd.declarations[property]; ok {
		return sp.priority
	}
	return ""
}

// SetProperty sets a CSS property with an optional priority.
func (sd *CSSRuleStyleDeclaration) SetProperty(property, value string, priority ...string) {
	property = normalizeRuleCSSPropertyName(property)
	if property == "" {
		return
	}

	// Empty value removes the property
	if value == "" {
		sd.RemoveProperty(property)
		return
	}

	pri := ""
	if len(priority) > 0 && strings.ToLower(priority[0]) == "important" {
		pri = "important"
	}

	// Check if property already exists
	if _, exists := sd.declarations[property]; !exists {
		sd.propertyOrder = append(sd.propertyOrder, property)
	}

	sd.declarations[property] = &ruleStyleProperty{
		value:    value,
		priority: pri,
	}
}

// RemoveProperty removes a CSS property and returns its old value.
func (sd *CSSRuleStyleDeclaration) RemoveProperty(property string) string {
	property = normalizeRuleCSSPropertyName(property)
	if sp, ok := sd.declarations[property]; ok {
		oldValue := sp.value
		delete(sd.declarations, property)
		// Remove from order
		for i, p := range sd.propertyOrder {
			if p == property {
				sd.propertyOrder = append(sd.propertyOrder[:i], sd.propertyOrder[i+1:]...)
				break
			}
		}
		return oldValue
	}
	return ""
}

// ParentRule returns the parent CSS rule.
func (sd *CSSRuleStyleDeclaration) ParentRule() CSSRuleInterface {
	return sd.parentRule
}

// PropertyNames returns all property names in declaration order.
func (sd *CSSRuleStyleDeclaration) PropertyNames() []string {
	result := make([]string, len(sd.propertyOrder))
	copy(result, sd.propertyOrder)
	return result
}

// GetAllProperties returns a sorted list of all CSS properties.
func (sd *CSSRuleStyleDeclaration) GetAllProperties() []string {
	result := make([]string, 0, len(sd.declarations))
	for prop := range sd.declarations {
		result = append(result, prop)
	}
	sort.Strings(result)
	return result
}

// normalizeRuleCSSPropertyName converts camelCase to kebab-case and lowercases.
var ruleCssPropertyPattern = regexp.MustCompile(`^-?[a-zA-Z][a-zA-Z0-9-]*$`)

func normalizeRuleCSSPropertyName(name string) string {
	if name == "" {
		return ""
	}

	// If already kebab-case, just lowercase
	if strings.Contains(name, "-") {
		return strings.ToLower(name)
	}

	// Convert camelCase to kebab-case
	var result strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('-')
			}
			result.WriteByte(byte(r - 'A' + 'a'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
