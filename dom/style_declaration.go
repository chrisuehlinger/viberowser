// Package dom provides a CSSStyleDeclaration implementation.
package dom

import (
	"regexp"
	"sort"
	"strings"
)

// CSSStyleDeclaration represents an element's inline style.
// It provides methods for getting and setting individual CSS properties.
type CSSStyleDeclaration struct {
	// The element this style declaration belongs to
	element *Element

	// Parsed declarations (property name -> declaration)
	declarations map[string]*styleProperty

	// Order in which properties were set (for cssText serialization)
	propertyOrder []string
}

// styleProperty holds a single CSS property's value and priority.
type styleProperty struct {
	value     string
	priority  string // "important" or ""
}

// NewCSSStyleDeclaration creates a new CSSStyleDeclaration for an element.
func NewCSSStyleDeclaration(element *Element) *CSSStyleDeclaration {
	sd := &CSSStyleDeclaration{
		element:      element,
		declarations: make(map[string]*styleProperty),
	}
	// Parse initial style attribute
	if element != nil && element.HasAttribute("style") {
		sd.parseFromAttribute(element.GetAttribute("style"))
	}
	return sd
}

// CSSText returns the textual representation of the declaration block.
func (sd *CSSStyleDeclaration) CSSText() string {
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
func (sd *CSSStyleDeclaration) SetCSSText(cssText string) {
	sd.declarations = make(map[string]*styleProperty)
	sd.propertyOrder = nil
	sd.parseFromAttribute(cssText)
	sd.syncToAttribute()
}

// Length returns the number of properties set.
func (sd *CSSStyleDeclaration) Length() int {
	return len(sd.declarations)
}

// Item returns the property name at the given index.
func (sd *CSSStyleDeclaration) Item(index int) string {
	if index < 0 || index >= len(sd.propertyOrder) {
		return ""
	}
	return sd.propertyOrder[index]
}

// GetPropertyValue returns the value of a CSS property.
func (sd *CSSStyleDeclaration) GetPropertyValue(property string) string {
	property = normalizeCSSPropertyName(property)
	if sp, ok := sd.declarations[property]; ok {
		return sp.value
	}
	return ""
}

// GetPropertyPriority returns the priority of a CSS property ("important" or "").
func (sd *CSSStyleDeclaration) GetPropertyPriority(property string) string {
	property = normalizeCSSPropertyName(property)
	if sp, ok := sd.declarations[property]; ok {
		return sp.priority
	}
	return ""
}

// SetProperty sets a CSS property with an optional priority.
func (sd *CSSStyleDeclaration) SetProperty(property, value string, priority ...string) {
	property = normalizeCSSPropertyName(property)
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

	sd.declarations[property] = &styleProperty{
		value:    value,
		priority: pri,
	}
	sd.syncToAttribute()
}

// RemoveProperty removes a CSS property and returns its old value.
func (sd *CSSStyleDeclaration) RemoveProperty(property string) string {
	property = normalizeCSSPropertyName(property)
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
		sd.syncToAttribute()
		return oldValue
	}
	return ""
}

// parseFromAttribute parses a style attribute string into declarations.
func (sd *CSSStyleDeclaration) parseFromAttribute(styleAttr string) {
	// Split by semicolons
	parts := strings.Split(styleAttr, ";")
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

		property = normalizeCSSPropertyName(property)
		if property != "" {
			if _, exists := sd.declarations[property]; !exists {
				sd.propertyOrder = append(sd.propertyOrder, property)
			}
			sd.declarations[property] = &styleProperty{
				value:    value,
				priority: priority,
			}
		}
	}
}

// syncToAttribute syncs the declarations back to the element's style attribute.
func (sd *CSSStyleDeclaration) syncToAttribute() {
	if sd.element == nil {
		return
	}

	cssText := sd.CSSText()
	if cssText == "" {
		sd.element.RemoveAttribute("style")
	} else {
		// Directly set attribute without triggering re-parse
		sd.element.Attributes().SetValue("style", cssText)
	}
}

// RefreshFromAttribute reloads declarations from the element's style attribute.
// Call this when the style attribute is changed externally.
func (sd *CSSStyleDeclaration) RefreshFromAttribute() {
	sd.declarations = make(map[string]*styleProperty)
	sd.propertyOrder = nil
	if sd.element != nil && sd.element.HasAttribute("style") {
		sd.parseFromAttribute(sd.element.GetAttribute("style"))
	}
}

// PropertyNames returns all property names in declaration order.
func (sd *CSSStyleDeclaration) PropertyNames() []string {
	result := make([]string, len(sd.propertyOrder))
	copy(result, sd.propertyOrder)
	return result
}

// normalizeCSSPropertyName converts camelCase to kebab-case and lowercases.
// Examples: "backgroundColor" -> "background-color", "WebkitTransform" -> "-webkit-transform"
func normalizeCSSPropertyName(name string) string {
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

// camelCasePropertyName converts kebab-case to camelCase.
// Examples: "background-color" -> "backgroundColor", "-webkit-transform" -> "WebkitTransform"
func camelCasePropertyName(name string) string {
	if name == "" {
		return ""
	}

	// Handle vendor prefixes
	if strings.HasPrefix(name, "-") {
		name = name[1:]
	}

	parts := strings.Split(name, "-")
	var result strings.Builder
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			result.WriteString(part)
		} else {
			result.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return result.String()
}

// isValidCSSPropertyName checks if a name is a valid CSS property name.
var cssPropertyPattern = regexp.MustCompile(`^-?[a-zA-Z][a-zA-Z0-9-]*$`)

func isValidCSSPropertyName(name string) bool {
	return cssPropertyPattern.MatchString(name)
}

// GetAllProperties returns a sorted list of all CSS properties.
// This is useful for implementing the style object's enumerable properties.
func (sd *CSSStyleDeclaration) GetAllProperties() []string {
	result := make([]string, 0, len(sd.declarations))
	for prop := range sd.declarations {
		result = append(result, prop)
	}
	sort.Strings(result)
	return result
}

// ParentRule returns the parent CSS rule (null for inline styles).
func (sd *CSSStyleDeclaration) ParentRule() interface{} {
	return nil
}
