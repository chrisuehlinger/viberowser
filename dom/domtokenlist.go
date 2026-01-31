package dom

import (
	"fmt"
	"strings"
)

// TokenValidationError represents an error during token validation.
type TokenValidationError struct {
	Type    string // "SyntaxError" or "InvalidCharacterError"
	Message string
}

func (e *TokenValidationError) Error() string {
	return e.Message
}

// validateToken checks if a token is valid per the DOMTokenList spec.
// Returns nil if valid, or a TokenValidationError if invalid.
func validateToken(token string) *TokenValidationError {
	// Per spec: if token is empty, throw SyntaxError
	if token == "" {
		return &TokenValidationError{
			Type:    "SyntaxError",
			Message: "The token provided must not be empty.",
		}
	}
	// Per spec: if token contains ASCII whitespace, throw InvalidCharacterError
	if strings.ContainsAny(token, " \t\n\r\f") {
		return &TokenValidationError{
			Type:    "InvalidCharacterError",
			Message: fmt.Sprintf("The token provided ('%s') contains HTML space characters, which are not valid in tokens.", token),
		}
	}
	return nil
}

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
// Per spec, the attribute is only set if it exists or if tokens are being added.
// If the attribute didn't exist and no tokens are being added, it stays unset.
func (dtl *DOMTokenList) setTokens(tokens []string) {
	if dtl.element == nil {
		return
	}
	// If there are tokens, always set the attribute
	if len(tokens) > 0 {
		dtl.element.SetAttribute(dtl.attrName, strings.Join(tokens, " "))
		return
	}
	// If the attribute exists, set it to empty string
	if dtl.element.HasAttribute(dtl.attrName) {
		dtl.element.SetAttribute(dtl.attrName, "")
	}
	// If the attribute didn't exist and tokens is empty, don't create it
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
// Per spec, contains() returns false for invalid tokens (empty or with whitespace).
func (dtl *DOMTokenList) Contains(token string) bool {
	if err := validateToken(token); err != nil {
		return false
	}
	for _, t := range dtl.tokens() {
		if t == token {
			return true
		}
	}
	return false
}

// Add adds one or more tokens to the list.
// Returns an error if any token is empty or contains whitespace.
func (dtl *DOMTokenList) Add(tokens ...string) *TokenValidationError {
	// Validate all tokens first
	for _, token := range tokens {
		if err := validateToken(token); err != nil {
			return err
		}
	}

	current := dtl.tokens()
	for _, token := range tokens {
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
	return nil
}

// Remove removes one or more tokens from the list.
// Returns an error if any token is empty or contains whitespace.
func (dtl *DOMTokenList) Remove(tokens ...string) *TokenValidationError {
	// Validate all tokens first
	for _, token := range tokens {
		if err := validateToken(token); err != nil {
			return err
		}
	}

	current := dtl.tokens()
	toRemove := make(map[string]bool)
	for _, token := range tokens {
		toRemove[token] = true
	}

	var result []string
	for _, t := range current {
		if !toRemove[t] {
			result = append(result, t)
		}
	}
	dtl.setTokens(result)
	return nil
}

// Toggle toggles the presence of a token.
// If force is provided, it forces add (true) or remove (false).
// Returns true if the token is present after the operation, and an error if the token is invalid.
func (dtl *DOMTokenList) Toggle(token string, force ...bool) (bool, *TokenValidationError) {
	if err := validateToken(token); err != nil {
		return false, err
	}

	contains := dtl.Contains(token)

	if len(force) > 0 {
		if force[0] {
			if !contains {
				dtl.Add(token)
			}
			return true, nil
		} else {
			if contains {
				dtl.Remove(token)
			}
			return false, nil
		}
	}

	if contains {
		dtl.Remove(token)
		return false, nil
	}
	dtl.Add(token)
	return true, nil
}

// Replace replaces an old token with a new token.
// Returns true if the old token was found and replaced, and an error if either token is invalid.
// Per spec, empty string errors (SyntaxError) are thrown before whitespace errors (InvalidCharacterError).
func (dtl *DOMTokenList) Replace(oldToken, newToken string) (bool, *TokenValidationError) {
	// Check for empty strings first (SyntaxError takes precedence)
	if oldToken == "" {
		return false, &TokenValidationError{
			Type:    "SyntaxError",
			Message: "The token provided must not be empty.",
		}
	}
	if newToken == "" {
		return false, &TokenValidationError{
			Type:    "SyntaxError",
			Message: "The token provided must not be empty.",
		}
	}
	// Then check for whitespace (InvalidCharacterError)
	if strings.ContainsAny(oldToken, " \t\n\r\f") {
		return false, &TokenValidationError{
			Type:    "InvalidCharacterError",
			Message: fmt.Sprintf("The token provided ('%s') contains HTML space characters, which are not valid in tokens.", oldToken),
		}
	}
	if strings.ContainsAny(newToken, " \t\n\r\f") {
		return false, &TokenValidationError{
			Type:    "InvalidCharacterError",
			Message: fmt.Sprintf("The token provided ('%s') contains HTML space characters, which are not valid in tokens.", newToken),
		}
	}

	current := dtl.tokens()

	// Find the first occurrence of oldToken
	oldIdx := -1
	for i, t := range current {
		if t == oldToken {
			oldIdx = i
			break
		}
	}

	if oldIdx == -1 {
		return false, nil
	}

	// Find if newToken already exists (and at what position)
	newTokenIdx := -1
	for i, t := range current {
		if t == newToken && i != oldIdx {
			newTokenIdx = i
			break
		}
	}

	// Build result: replace oldToken with newToken, deduplicate
	result := make([]string, 0, len(current))
	newTokenAdded := false

	for i, t := range current {
		if t == oldToken {
			if i == oldIdx {
				// This is the first oldToken - replace with newToken
				// But only if newToken doesn't already exist earlier
				if newTokenIdx == -1 || newTokenIdx > oldIdx {
					result = append(result, newToken)
					newTokenAdded = true
				}
				// If newToken exists earlier, just skip oldToken
			}
			// Skip all subsequent occurrences of oldToken
		} else if t == newToken {
			// Keep newToken if it exists before oldIdx
			if i == newTokenIdx && newTokenIdx < oldIdx {
				result = append(result, t)
				newTokenAdded = true
			}
			// Skip any subsequent occurrences of newToken
		} else {
			result = append(result, t)
		}
	}

	// Handle the case where oldToken == newToken
	// In this case, newTokenIdx will be -1 (we skip matching indices)
	// and we just need to deduplicate all occurrences of oldToken
	if oldToken == newToken && !newTokenAdded {
		result = append([]string{newToken}, result...)
	}

	dtl.setTokens(result)
	return true, nil
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
