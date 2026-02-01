package css

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/AYColumbia/viberowser/dom"
)

// MatchContext holds context for selector matching.
type MatchContext struct {
	// ScopeElement is the element that :scope should match against.
	// If nil, :scope matches the document root.
	ScopeElement *dom.Element
}

// MatchElement tests if a selector matches an element.
func (s *CSSSelector) MatchElement(el *dom.Element) bool {
	return s.MatchElementWithContext(el, nil)
}

// MatchElementWithContext tests if a selector matches an element with a match context.
func (s *CSSSelector) MatchElementWithContext(el *dom.Element, ctx *MatchContext) bool {
	for _, cs := range s.ComplexSelectors {
		if cs.MatchElementWithContext(el, ctx) {
			return true
		}
	}
	return false
}

// MatchElement tests if a complex selector matches an element.
func (cs *ComplexSelector) MatchElement(el *dom.Element) bool {
	return cs.MatchElementWithContext(el, nil)
}

// MatchElementWithContext tests if a complex selector matches an element with context.
func (cs *ComplexSelector) MatchElementWithContext(el *dom.Element, ctx *MatchContext) bool {
	if len(cs.Compounds) == 0 {
		return false
	}

	// Start from the last compound selector (the subject)
	i := len(cs.Compounds) - 1
	currentEl := el

	// Match the rightmost compound against the subject element
	if !cs.Compounds[i].MatchElementWithContext(currentEl, ctx) {
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
				if cs.Compounds[i].MatchElementWithContext(ancestor, ctx) {
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
			if parent == nil || !cs.Compounds[i].MatchElementWithContext(parent, ctx) {
				return false
			}
			currentEl = parent

		case CombinatorNextSibling:
			// Match immediately preceding sibling
			prev := currentEl.PreviousElementSibling()
			if prev == nil || !cs.Compounds[i].MatchElementWithContext(prev, ctx) {
				return false
			}
			currentEl = prev

		case CombinatorSubsequentSibling:
			// Match any preceding sibling
			matched := false
			for prev := currentEl.PreviousElementSibling(); prev != nil; prev = prev.PreviousElementSibling() {
				if cs.Compounds[i].MatchElementWithContext(prev, ctx) {
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
	return c.MatchElementWithContext(el, nil)
}

// MatchElementWithContext tests if a compound selector matches an element with context.
func (c *CompoundSelector) MatchElementWithContext(el *dom.Element, ctx *MatchContext) bool {
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
		if !matchPseudoClassWithContext(pc, el, ctx) {
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
	// Handle namespace selectors
	// attr.Namespace can be:
	//   "" - no namespace specified, match by qualified name
	//   "*" - any namespace (or no namespace), match by local name
	//   other - specific namespace prefix (not implemented for matching)

	// Per Selectors Level 4 spec:
	// For HTML elements in HTML documents, attribute names in selectors are case-insensitive.
	// For SVG/MathML/XML elements, attribute names are case-sensitive.
	isHTMLElement := el.NamespaceURI() == "http://www.w3.org/1999/xhtml" || el.NamespaceURI() == ""

	var matchedAttrValue string
	var found bool

	if attr.Namespace == "*" {
		// Match any attribute with the given local name in any namespace
		attrs := el.Attributes()
		for i := 0; i < attrs.Length(); i++ {
			a := attrs.Item(i)
			attrLocalName := a.LocalName()
			selectorAttrName := attr.Name
			if isHTMLElement {
				// Case-insensitive match for HTML elements
				if strings.EqualFold(attrLocalName, selectorAttrName) {
					matchedAttrValue = a.Value()
					found = true
					break
				}
			} else {
				// Case-sensitive match for SVG/MathML elements
				if attrLocalName == selectorAttrName {
					matchedAttrValue = a.Value()
					found = true
					break
				}
			}
		}
	} else if attr.Namespace == "" {
		// No namespace specified - match by qualified name (normal case)
		// For HTML elements, HasAttribute is already case-insensitive (it lowercases the name)
		// For SVG/MathML elements, HasAttribute is case-sensitive
		// But we also need to handle the case where the selector has different case
		if isHTMLElement {
			// HTML: element.HasAttribute already lowercases, but we need to lowercase the selector name too
			attrNameLower := strings.ToLower(attr.Name)
			if el.HasAttribute(attrNameLower) {
				matchedAttrValue = el.GetAttribute(attrNameLower)
				found = true
			}
		} else {
			// SVG/MathML: case-sensitive match
			if el.HasAttribute(attr.Name) {
				matchedAttrValue = el.GetAttribute(attr.Name)
				found = true
			}
		}
	} else {
		// Specific namespace - would need to map prefix to URI
		// For now, fall back to qualified name match
		if isHTMLElement {
			attrNameLower := strings.ToLower(attr.Name)
			if el.HasAttribute(attrNameLower) {
				matchedAttrValue = el.GetAttribute(attrNameLower)
				found = true
			}
		} else {
			if el.HasAttribute(attr.Name) {
				matchedAttrValue = el.GetAttribute(attr.Name)
				found = true
			}
		}
	}

	if !found {
		return false
	}

	if attr.Operator == AttrExists {
		return true
	}

	attrValue := matchedAttrValue
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
	return matchPseudoClassWithContext(pc, el, nil)
}

func matchPseudoClassWithContext(pc *PseudoClassSelector, el *dom.Element, ctx *MatchContext) bool {
	switch pc.Name {
	case "root":
		// Matches the root element of the document
		parent := el.AsNode().ParentNode()
		return parent != nil && parent.NodeType() == dom.DocumentNode

	case "empty":
		// Matches elements with no children (including text)
		return !el.AsNode().HasChildNodes()

	case "first-child":
		// :first-child matches an element that is the first element child of its parent
		parent := el.AsNode().ParentNode()
		if parent == nil {
			return false
		}
		for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
			if child.NodeType() == dom.ElementNode {
				return (*dom.Element)(child) == el
			}
		}
		return false

	case "last-child":
		// :last-child matches an element that is the last element child of its parent
		parent := el.AsNode().ParentNode()
		if parent == nil {
			return false
		}
		for child := parent.LastChild(); child != nil; child = child.PreviousSibling() {
			if child.NodeType() == dom.ElementNode {
				return (*dom.Element)(child) == el
			}
		}
		return false

	case "only-child":
		return matchPseudoClassWithContext(&PseudoClassSelector{Name: "first-child"}, el, ctx) &&
			matchPseudoClassWithContext(&PseudoClassSelector{Name: "last-child"}, el, ctx)

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
			return !pc.Selector.MatchElementWithContext(el, ctx)
		}
		return true

	case "is", "where", "matches", "any":
		if pc.Selector != nil {
			return pc.Selector.MatchElementWithContext(el, ctx)
		}
		return false

	case "has":
		if pc.Selector != nil {
			// :has() matches if any element matches the relative selector
			// The selector inside :has() is a relative selector which may have a leading combinator
			return matchHasSelector(el, pc.Selector, ctx)
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
		// :scope matches the element that the selector is being matched against
		// In closest() context, this is the element closest was called on
		if ctx != nil && ctx.ScopeElement != nil {
			return el == ctx.ScopeElement
		}
		// Default to document root if no scope context
		parent := el.AsNode().ParentNode()
		return parent != nil && parent.NodeType() == dom.DocumentNode

	case "invalid":
		return isInvalid(el)

	case "valid":
		return isValid(el)

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
	return hasMatchingDescendantWithContext(el, sel, nil)
}

func hasMatchingDescendantWithContext(el *dom.Element, sel *CSSSelector, ctx *MatchContext) bool {
	// Check children recursively
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		if sel.MatchElementWithContext(child, ctx) {
			return true
		}
		if hasMatchingDescendantWithContext(child, sel, ctx) {
			return true
		}
	}
	return false
}

// matchHasSelector checks if any element matches the relative selector inside :has().
// The selector may have a leading combinator that determines how to search.
func matchHasSelector(subject *dom.Element, sel *CSSSelector, ctx *MatchContext) bool {
	for _, cs := range sel.ComplexSelectors {
		if matchHasComplexSelector(subject, cs, ctx) {
			return true
		}
	}
	return false
}

// matchHasComplexSelector handles a single complex selector within :has().
func matchHasComplexSelector(subject *dom.Element, cs *ComplexSelector, ctx *MatchContext) bool {
	switch cs.LeadingCombinator {
	case CombinatorChild:
		// :has(> selector) - check direct children only
		for child := subject.FirstElementChild(); child != nil; child = child.NextElementSibling() {
			if matchRelativeSelector(child, cs, ctx) {
				return true
			}
		}
		return false

	case CombinatorNextSibling:
		// :has(+ selector) - check next sibling only
		next := subject.NextElementSibling()
		if next != nil && matchRelativeSelector(next, cs, ctx) {
			return true
		}
		return false

	case CombinatorSubsequentSibling:
		// :has(~ selector) - check all following siblings
		for next := subject.NextElementSibling(); next != nil; next = next.NextElementSibling() {
			if matchRelativeSelector(next, cs, ctx) {
				return true
			}
		}
		return false

	default:
		// :has(selector) with no leading combinator or CombinatorDescendant - check all descendants
		return hasMatchingDescendantForRelative(subject, cs, ctx)
	}
}

// matchRelativeSelector checks if an element matches the compound selectors in a relative selector.
func matchRelativeSelector(el *dom.Element, cs *ComplexSelector, ctx *MatchContext) bool {
	if len(cs.Compounds) == 0 {
		return false
	}

	// Start from the first compound (which is what the leading combinator points to)
	i := 0
	currentEl := el

	// Match the first compound
	if !cs.Compounds[i].MatchElementWithContext(currentEl, ctx) {
		return false
	}

	// If there's only one compound, we matched
	if len(cs.Compounds) == 1 {
		return true
	}

	// Continue with remaining compounds (they have combinators between them)
	for i < len(cs.Compounds)-1 {
		combinator := cs.Compounds[i].Combinator
		i++

		switch combinator {
		case CombinatorDescendant:
			// Match any descendant
			matched := false
			descendants := getAllDescendants(currentEl)
			for _, desc := range descendants {
				if cs.Compounds[i].MatchElementWithContext(desc, ctx) {
					currentEl = desc
					matched = true
					break
				}
			}
			if !matched {
				return false
			}

		case CombinatorChild:
			// Match any direct child
			matched := false
			for child := currentEl.FirstElementChild(); child != nil; child = child.NextElementSibling() {
				if cs.Compounds[i].MatchElementWithContext(child, ctx) {
					currentEl = child
					matched = true
					break
				}
			}
			if !matched {
				return false
			}

		case CombinatorNextSibling:
			// Match next sibling
			next := currentEl.NextElementSibling()
			if next == nil || !cs.Compounds[i].MatchElementWithContext(next, ctx) {
				return false
			}
			currentEl = next

		case CombinatorSubsequentSibling:
			// Match any following sibling
			matched := false
			for next := currentEl.NextElementSibling(); next != nil; next = next.NextElementSibling() {
				if cs.Compounds[i].MatchElementWithContext(next, ctx) {
					currentEl = next
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

// hasMatchingDescendantForRelative checks descendants for a relative selector.
func hasMatchingDescendantForRelative(el *dom.Element, cs *ComplexSelector, ctx *MatchContext) bool {
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		if matchRelativeSelector(child, cs, ctx) {
			return true
		}
		if hasMatchingDescendantForRelative(child, cs, ctx) {
			return true
		}
	}
	return false
}

// getAllDescendants returns all descendant elements.
func getAllDescendants(el *dom.Element) []*dom.Element {
	var result []*dom.Element
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		result = append(result, child)
		result = append(result, getAllDescendants(child)...)
	}
	return result
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

// isInvalid checks if an element matches the :invalid pseudo-class.
// An element matches :invalid if it has constraints and fails constraint validation.
func isInvalid(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())

	switch tagName {
	case "form":
		// A form is invalid if any of its form controls are invalid
		// Check descendants
		for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
			if isInvalid(child) {
				return true
			}
			// Check nested children too
			if hasInvalidDescendant(child) {
				return true
			}
		}
		return false

	case "fieldset":
		// A fieldset is invalid if any of its descendants are invalid
		for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
			if isInvalid(child) {
				return true
			}
			if hasInvalidDescendant(child) {
				return true
			}
		}
		return false

	case "input":
		// Check required attribute
		if el.HasAttribute("required") {
			value := el.GetAttribute("value")
			if value == "" {
				return true
			}
		}
		return false

	case "select":
		// Select is invalid if required and no option is selected
		if el.HasAttribute("required") {
			// Check if any option has selected attribute
			hasSelected := false
			for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
				if strings.ToLower(child.TagName()) == "option" {
					if child.HasAttribute("selected") {
						hasSelected = true
						break
					}
				}
			}
			if !hasSelected {
				return true
			}
		}
		return false

	case "textarea":
		// Textarea is invalid if required and empty
		if el.HasAttribute("required") {
			// TextContent would be the value
			text := el.AsNode().TextContent()
			if text == "" {
				return true
			}
		}
		return false
	}

	return false
}

func hasInvalidDescendant(el *dom.Element) bool {
	for child := el.FirstElementChild(); child != nil; child = child.NextElementSibling() {
		if isInvalid(child) {
			return true
		}
		if hasInvalidDescendant(child) {
			return true
		}
	}
	return false
}

// isValid checks if an element matches the :valid pseudo-class.
func isValid(el *dom.Element) bool {
	tagName := strings.ToLower(el.TagName())

	// Only form-associated elements can be :valid
	switch tagName {
	case "form", "fieldset", "input", "select", "textarea":
		return !isInvalid(el)
	}
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
