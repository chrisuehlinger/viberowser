package dom

import "strings"

// DOMTokenList represents a set of space-separated tokens.
// It is used for Element.classList.
type DOMTokenList struct {
	element   *Element
	attrName  string // The attribute this token list is associated with (e.g., "class")
}

// newDOMTokenList creates a new DOMTokenList for the given element and attribute.
func newDOMTokenList(element *Element, attrName string) *DOMTokenList {
	return &DOMTokenList{
		element:  element,
		attrName: attrName,
	}
}

// tokens returns the current list of tokens (deduplicated, preserving order).
func (dtl *DOMTokenList) tokens() []string {
	if dtl.element == nil {
		return nil
	}
	value := dtl.element.GetAttribute(dtl.attrName)
	if value == "" {
		return nil
	}
	// Split by whitespace and deduplicate while preserving order
	allTokens := strings.Fields(value)
	seen := make(map[string]bool)
	result := make([]string, 0, len(allTokens))
	for _, token := range allTokens {
		if !seen[token] {
			seen[token] = true
			result = append(result, token)
		}
	}
	return result
}

// setTokens sets the tokens back to the attribute.
func (dtl *DOMTokenList) setTokens(tokens []string) {
	if dtl.element == nil {
		return
	}
	if len(tokens) == 0 {
		dtl.element.RemoveAttribute(dtl.attrName)
	} else {
		dtl.element.SetAttribute(dtl.attrName, strings.Join(tokens, " "))
	}
}

// Length returns the number of tokens.
func (dtl *DOMTokenList) Length() int {
	return len(dtl.tokens())
}

// Item returns the token at the given index, or empty string if out of bounds.
func (dtl *DOMTokenList) Item(index int) string {
	tokens := dtl.tokens()
	if index < 0 || index >= len(tokens) {
		return ""
	}
	return tokens[index]
}

// Contains returns true if the given token is in the list.
func (dtl *DOMTokenList) Contains(token string) bool {
	for _, t := range dtl.tokens() {
		if t == token {
			return true
		}
	}
	return false
}

// Add adds one or more tokens to the list.
func (dtl *DOMTokenList) Add(tokens ...string) {
	current := dtl.tokens()
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		// Check if already present
		found := false
		for _, t := range current {
			if t == token {
				found = true
				break
			}
		}
		if !found {
			current = append(current, token)
		}
	}
	dtl.setTokens(current)
}

// Remove removes one or more tokens from the list.
func (dtl *DOMTokenList) Remove(tokens ...string) {
	current := dtl.tokens()
	toRemove := make(map[string]bool)
	for _, token := range tokens {
		toRemove[strings.TrimSpace(token)] = true
	}

	var result []string
	for _, t := range current {
		if !toRemove[t] {
			result = append(result, t)
		}
	}
	dtl.setTokens(result)
}

// Toggle toggles the presence of a token.
// If force is provided, it forces add (true) or remove (false).
// Returns true if the token is present after the operation.
func (dtl *DOMTokenList) Toggle(token string, force ...bool) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	contains := dtl.Contains(token)

	if len(force) > 0 {
		if force[0] {
			if !contains {
				dtl.Add(token)
			}
			return true
		} else {
			if contains {
				dtl.Remove(token)
			}
			return false
		}
	}

	if contains {
		dtl.Remove(token)
		return false
	}
	dtl.Add(token)
	return true
}

// Replace replaces an old token with a new token.
// Returns true if the old token was found and replaced.
func (dtl *DOMTokenList) Replace(oldToken, newToken string) bool {
	oldToken = strings.TrimSpace(oldToken)
	newToken = strings.TrimSpace(newToken)

	if oldToken == "" || newToken == "" {
		return false
	}

	// If old and new are the same, no change needed
	if oldToken == newToken {
		return dtl.Contains(oldToken)
	}

	current := dtl.tokens()
	oldIdx := -1
	for i, t := range current {
		if t == oldToken {
			oldIdx = i
			break
		}
	}

	if oldIdx == -1 {
		return false
	}

	// Replace at the old position and remove any subsequent occurrences of newToken
	result := make([]string, 0, len(current))
	for i, t := range current {
		if i == oldIdx {
			result = append(result, newToken)
		} else if t != newToken {
			result = append(result, t)
		}
	}
	dtl.setTokens(result)
	return true
}

// Supports returns true if the given token is supported.
// This is used for feature detection with certain attributes.
// For classList, this always returns true.
func (dtl *DOMTokenList) Supports(token string) bool {
	return true
}

// Value returns the underlying string value.
func (dtl *DOMTokenList) Value() string {
	if dtl.element == nil {
		return ""
	}
	return dtl.element.GetAttribute(dtl.attrName)
}

// SetValue sets the underlying string value.
func (dtl *DOMTokenList) SetValue(value string) {
	if dtl.element == nil {
		return
	}
	dtl.element.SetAttribute(dtl.attrName, value)
}

// String returns the string representation (same as Value).
func (dtl *DOMTokenList) String() string {
	return dtl.Value()
}

// Entries returns an iterator-like slice of [index, token] pairs.
func (dtl *DOMTokenList) Entries() [][2]interface{} {
	tokens := dtl.tokens()
	entries := make([][2]interface{}, len(tokens))
	for i, token := range tokens {
		entries[i] = [2]interface{}{i, token}
	}
	return entries
}

// ForEach calls the given function for each token.
func (dtl *DOMTokenList) ForEach(fn func(token string, index int)) {
	for i, token := range dtl.tokens() {
		fn(token, i)
	}
}

// Keys returns an iterator-like slice of indices.
func (dtl *DOMTokenList) Keys() []int {
	tokens := dtl.tokens()
	keys := make([]int, len(tokens))
	for i := range tokens {
		keys[i] = i
	}
	return keys
}

// Values returns an iterator-like slice of tokens.
func (dtl *DOMTokenList) Values() []string {
	return dtl.tokens()
}
