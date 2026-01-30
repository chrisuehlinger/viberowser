package css

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/AYColumbia/viberowser/dom"
)

// MatchElement tests if a selector matches an element.
func (s *CSSSelector) MatchElement(el *dom.Element) bool {
	for _, cs := range s.ComplexSelectors {
		if cs.MatchElement(el) {
			return true
		}
	}
	return false
}

// MatchElement tests if a complex selector matches an element.
func (cs *ComplexSelector) MatchElement(el *dom.Element) bool {
	if len(cs.Compounds) == 0 {
		return false
	}

	// Start from the last compound selector (the subject)
	i := len(cs.Compounds) - 1
	currentEl := el

	// Match the rightmost compound against the subject element
	if !cs.Compounds[i].MatchElement(currentEl) {
		return false
	}

	// Work backwards through the compound selectors
	for i > 0 {
		combinator := cs.Compounds[i-1].Combinator
		i--

		switch combinator {
		case CombinatorDescendant:
			// Match any ancestor
			matched := false
			for ancestor := currentEl.AsNode().ParentElement(); ancestor != nil; ancestor = ancestor.AsNode().ParentElement() {
				if cs.Compounds[i].MatchElement(ancestor) {
					currentEl = ancestor
					matched = true
					break
				}
			}
			if !matched {
				return false
			}

		case CombinatorChild:
			// Match direct parent
			parent := currentEl.AsNode().ParentElement()
			if parent == nil || !cs.Compounds[i].MatchElement(parent) {
				return false
			}
			currentEl = parent

		case CombinatorNextSibling:
			// Match immediately preceding sibling
			prev := currentEl.PreviousElementSibling()
			if prev == nil || !cs.Compounds[i].MatchElement(prev) {
				return false
			}
			currentEl = prev

		case CombinatorSubsequentSibling:
			// Match any preceding sibling
			matched := false
			for prev := currentEl.PreviousElementSibling(); prev != nil; prev = prev.PreviousElementSibling() {
				if cs.Compounds[i].MatchElement(prev) {
					currentEl = prev
					matched = true
					break
				}
			}
			if !matched {
				return false
			}

		default:
			return false
		}
	}

	return true
}

// MatchElement tests if a compound selector matches an element.
func (c *CompoundSelector) MatchElement(el *dom.Element) bool {
	// Type selector
	if c.TypeSelector != nil {
		if !matchTypeSelector(c.TypeSelector, el) {
			return false
		}
	}

	// ID selectors
	for _, id := range c.IDSelectors {
		if el.Id() != id {
			return false
		}
	}

	// Class selectors
	for _, class := range c.ClassSelectors {
		if !el.ClassList().Contains(class) {
			return false
		}
	}

	// Attribute selectors
	for _, attr := range c.AttributeMatchers {
		if !matchAttributeSelector(attr, el) {
			return false
		}
	}

	// Pseudo-classes
	for _, pc := range c.PseudoClasses {
		if !matchPseudoClass(pc, el) {
			return false
		}
	}

	// Pseudo-element (for matching purposes, we just check it's valid)
	// Actual rendering is handled elsewhere

	return true
}

func matchTypeSelector(ts *TypeSelector, el *dom.Element) bool {
	if ts.Name == "*" {
		return true
	}
	return strings.EqualFold(el.LocalName(), ts.Name)
}

func matchAttributeSelector(attr *AttributeMatcher, el *dom.Element) bool {
	if !el.HasAttribute(attr.Name) {
		return false
	}

	if attr.Operator == AttrExists {
		return true
	}

	attrValue := el.GetAttribute(attr.Name)
	matchValue := attr.Value

	if attr.CaseInsensitive {
		attrValue = strings.ToLower(attrValue)
		matchValue = strings.ToLower(matchValue)
	}

	switch attr.Operator {
	case AttrEquals:
		return attrValue == matchValue
	case AttrIncludes:
		for _, word := range strings.Fields(attrValue) {
			if attr.CaseInsensitive {
				word = strings.ToLower(word)
			}
			if word == matchValue {
				return true
			}
		}
		return false
	case AttrDashMatch:
		return attrValue == matchValue || strings.HasPrefix(attrValue, matchValue+"-")
	case AttrPrefix:
		return strings.HasPrefix(attrValue, matchValue)
	case AttrSuffix:
		return strings.HasSuffix(attrValue, matchValue)
	case AttrSubstring:
		return strings.Contains(attrValue, matchValue)
	}

	return false
}

func matchPseudoClass(pc *PseudoClassSelector, el *dom.Element) bool {
	switch pc.Name {
	case "root":
		// Matches the root element of the document
		parent := el.AsNode().ParentNode()
		return parent != nil && parent.NodeType() == dom.DocumentNode

	case "empty":
		// Matches elements with no children (including text)
		return !el.AsNode().HasChildNodes()

	case "first-child":
		return el.PreviousElementSibling() == nil

	case "last-child":
		return el.NextElementSibling() == nil

	case "only-child":
		return el.PreviousElementSibling() == nil && el.NextElementSibling() == nil

	case "first-of-type":
		tagName := el.LocalName()
		for prev := el.PreviousElementSibling(); prev != nil; prev = prev.PreviousElementSibling() {
			if prev.LocalName() == tagName {
				return false
			}
		}
		return true

	case "last-of-type":
		tagName := el.LocalName()
		for next := el.NextElementSibling(); next != nil; next = next.NextElementSibling() {
			if next.LocalName() == tagName {
				return false
			}
		}
		return true

	case "only-of-type":
		tagName := el.LocalName()
		for prev := el.PreviousElementSibling(); prev != nil; prev = prev.PreviousElementSibling() {
			if prev.LocalName() == tagName {
				return false
			}
		}
		for next := el.NextElementSibling(); next != nil; next = next.NextElementSibling() {
			if next.LocalName() == tagName {
				return false
			}
		}
		return true

	case "nth-child":
		return matchNthChild(pc.Argument, el, false, false)

	case "nth-last-child":
		return matchNthChild(pc.Argument, el, true, false)

	case "nth-of-type":
		return matchNthChild(pc.Argument, el, false, true)

	case "nth-last-of-type":
		return matchNthChild(pc.Argument, el, true, true)

	case "not":
		if pc.Selector != nil {
			return !pc.Selector.MatchElement(el)
		}
		return true

	case "is", "where", "matches", "any":
		if pc.Selector != nil {
			return pc.Selector.MatchElement(el)
		}
		return false

	case "has":
		if pc.Selector != nil {
			// :has() matches if any descendant matches the relative selector
			return hasMatchingDescendant(el, pc.Selector)
		}
		return false

	case "enabled":
		return isEnabled(el)

	case "disabled":
		return isDisabled(el)

	case "checked":
		return isChecked(el)

	case "required":
		return el.HasAttribute("required")

	case "optional":
		return !el.HasAttribute("required") && isFormElement(el)

	case "read-only":
		return isReadOnly(el)

	case "read-write":
		return !isReadOnly(el) && isEditableElement(el)

	case "link":
		return isLink(el) && !isVisited(el)

	case "visited":
		return isLink(el) && isVisited(el)

	case "hover", "active", "focus", "focus-within", "focus-visible":
		// These are dynamic states that need to be tracked elsewhere
		// For now, return false
		return false

	case "target":
		// Would need to know the current fragment identifier
		return false

	case "lang":
		return matchLang(pc.Argument, el)

	case "dir":
		return matchDir(pc.Argument, el)

	case "scope":
		// In querySelector context, matches the element being queried from
		// Here we just match root
		parent := el.AsNode().ParentNode()
		return parent != nil && parent.NodeType() == dom.DocumentNode

	default:
		// Unknown pseudo-class - don't match
		return false
	}
}

// matchNthChild implements :nth-child, :nth-last-child, :nth-of-type, :nth-last-of-type
func matchNthChild(arg string, el *dom.Element, fromLast bool, ofType bool) bool {
	arg = strings.TrimSpace(strings.ToLower(arg))

	// Handle special keywords
	if arg == "odd" {
		arg = "2n+1"
	} else if arg == "even" {
		arg = "2n"
	}

	// Parse An+B syntax
	a, b := parseAnPlusB(arg)

	// Calculate the element's position
	pos := 1
	tagName := el.LocalName()

	if fromLast {
		for next := el.NextElementSibling(); next != nil; next = next.NextElementSibling() {
			if !ofType || next.LocalName() == tagName {
				pos++
			}
		}
	} else {
		for prev := el.PreviousElementSibling(); prev != nil; prev = prev.PreviousElementSibling() {
			if !ofType || prev.LocalName() == tagName {
				pos++
			}
		}
	}

	// Check if pos matches An+B
	if a == 0 {
		return pos == b
	}

	// pos = a*n + b where n >= 0
	// n = (pos - b) / a
	diff := pos - b
	if a > 0 {
		return diff >= 0 && diff%a == 0
	} else {
		return diff <= 0 && diff%a == 0
	}
}

// parseAnPlusB parses an An+B expression.
func parseAnPlusB(s string) (int, int) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "")

	// Handle special keywords
	if s == "odd" {
		return 2, 1
	}
	if s == "even" {
		return 2, 0
	}

	// Try to match patterns
	// "n" -> 1n+0
	// "-n" -> -1n+0
	// "3n" -> 3n+0
	// "n+2" -> 1n+2
	// "3n+2" -> 3n+2
	// "3n-2" -> 3n-2
	// "-n+2" -> -1n+2
	// "5" -> 0n+5

	// Check for just a number
	if n, err := strconv.Atoi(s); err == nil {
		return 0, n
	}

	// Look for 'n'
	nIdx := strings.Index(s, "n")
	if nIdx == -1 {
		return 0, 0
	}

	// Parse A
	aStr := s[:nIdx]
	var a int
	if aStr == "" || aStr == "+" {
		a = 1
	} else if aStr == "-" {
		a = -1
	} else {
		a, _ = strconv.Atoi(aStr)
	}

	// Parse B
	bStr := s[nIdx+1:]
	var b int
	if bStr == "" {
		b = 0
	} else {
		b, _ = strconv.Atoi(bStr)
	}

	return a, b
}

func hasMatchingDescendant(el *dom.Element, sel *CSSSelector) bool {
	// Check children recursively
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		if sel.MatchElement(child) {
			return true
		}
		if hasMatchingDescendant(child, sel) {
			return true
		}
	}
	return false
}

func isEnabled(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "button" || tagName == "input" || tagName == "select" || tagName == "textarea" {
		return !el.HasAttribute("disabled")
	}
	return false
}

func isDisabled(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "button" || tagName == "input" || tagName == "select" || tagName == "textarea" {
		return el.HasAttribute("disabled")
	}
	return false
}

func isChecked(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "input" {
		inputType := strings.ToLower(el.GetAttribute("type"))
		if inputType == "checkbox" || inputType == "radio" {
			return el.HasAttribute("checked")
		}
	} else if tagName == "option" {
		return el.HasAttribute("selected")
	}
	return false
}

func isFormElement(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	return tagName == "input" || tagName == "select" || tagName == "textarea"
}

func isReadOnly(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "input" || tagName == "textarea" {
		return el.HasAttribute("readonly") || el.HasAttribute("disabled")
	}
	return true
}

func isEditableElement(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "input" {
		inputType := strings.ToLower(el.GetAttribute("type"))
		// Text-like inputs are editable
		switch inputType {
		case "text", "password", "email", "url", "tel", "search", "number", "":
			return true
		}
	}
	if tagName == "textarea" {
		return true
	}
	// Check for contenteditable
	if el.HasAttribute("contenteditable") {
		val := el.GetAttribute("contenteditable")
		return val != "false"
	}
	return false
}

func isLink(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())
	if tagName == "a" || tagName == "area" {
		return el.HasAttribute("href")
	}
	return false
}

func isVisited(el *dom.Element) bool {
	// We don't track visited links for privacy reasons
	return false
}

func matchLang(lang string, el *dom.Element) bool {
	lang = strings.ToLower(lang)

	// Walk up the tree looking for lang attribute
	for current := el; current != nil; current = current.AsNode().ParentElement() {
		if current.HasAttribute("lang") {
			elLang := strings.ToLower(current.GetAttribute("lang"))
			if elLang == lang || strings.HasPrefix(elLang, lang+"-") {
				return true
			}
			return false
		}
	}
	return false
}

func matchDir(dir string, el *dom.Element) bool {
	dir = strings.ToLower(dir)

	// Walk up the tree looking for dir attribute
	for current := el; current != nil; current = current.AsNode().ParentElement() {
		if current.HasAttribute("dir") {
			elDir := strings.ToLower(current.GetAttribute("dir"))
			return elDir == dir
		}
	}

	// Default is ltr
	return dir == "ltr"
}

// QuerySelector returns the first element matching the selector.
func QuerySelector(root *dom.Node, selectorStr string) *dom.Element {
	selector, err := ParseSelector(selectorStr)
	if err != nil {
		return nil
	}

	return querySelectorInternal(root, selector, true)
}

// QuerySelectorAll returns all elements matching the selector.
func QuerySelectorAll(root *dom.Node, selectorStr string) []*dom.Element {
	selector, err := ParseSelector(selectorStr)
	if err != nil {
		return nil
	}

	return querySelectorAllInternal(root, selector)
}

func querySelectorInternal(node *dom.Node, selector *CSSSelector, firstOnly bool) *dom.Element {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.NodeType() == dom.ElementNode {
			el := (*dom.Element)(child)
			if selector.MatchElement(el) {
				return el
			}
			if result := querySelectorInternal(child, selector, firstOnly); result != nil {
				return result
			}
		}
	}
	return nil
}

func querySelectorAllInternal(node *dom.Node, selector *CSSSelector) []*dom.Element {
	var results []*dom.Element

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.NodeType() == dom.ElementNode {
			el := (*dom.Element)(child)
			if selector.MatchElement(el) {
				results = append(results, el)
			}
			results = append(results, querySelectorAllInternal(child, selector)...)
		}
	}

	return results
}

// Helper function to match simple patterns (used for basic testing)
var _ = regexp.MustCompile // Ensure regexp is available if needed
