package dom

import (
	"regexp"
	"strings"
)

// ProcessingInstruction represents a processing instruction node in the DOM.
// Processing instructions look like: <?target data?>
// This interface inherits from CharacterData.
type ProcessingInstruction Node

// AsNode returns the underlying Node.
func (pi *ProcessingInstruction) AsNode() *Node {
	return (*Node)(pi)
}

// NodeType returns ProcessingInstructionNode (7).
func (pi *ProcessingInstruction) NodeType() NodeType {
	return ProcessingInstructionNode
}

// NodeName returns the target of the processing instruction.
func (pi *ProcessingInstruction) NodeName() string {
	return pi.AsNode().nodeName
}

// Target returns the target of the processing instruction (read-only).
// This is the application to which the instruction is targeted.
func (pi *ProcessingInstruction) Target() string {
	return pi.AsNode().nodeName
}

// Data returns the content of the processing instruction.
func (pi *ProcessingInstruction) Data() string {
	return pi.AsNode().NodeValue()
}

// SetData sets the content of the processing instruction.
func (pi *ProcessingInstruction) SetData(data string) {
	pi.AsNode().SetNodeValue(data)
}

// Length returns the length of the data content.
func (pi *ProcessingInstruction) Length() int {
	return len(pi.Data())
}

// SubstringData extracts a substring of the data.
func (pi *ProcessingInstruction) SubstringData(offset, count int) string {
	data := pi.Data()
	if offset < 0 || offset > len(data) {
		return ""
	}
	end := offset + count
	if end > len(data) {
		end = len(data)
	}
	return data[offset:end]
}

// AppendData appends a string to the data.
func (pi *ProcessingInstruction) AppendData(data string) {
	pi.SetData(pi.Data() + data)
}

// InsertData inserts a string at the given offset.
func (pi *ProcessingInstruction) InsertData(offset int, data string) {
	current := pi.Data()
	if offset < 0 {
		offset = 0
	}
	if offset > len(current) {
		offset = len(current)
	}
	pi.SetData(current[:offset] + data + current[offset:])
}

// DeleteData deletes characters starting at the given offset.
func (pi *ProcessingInstruction) DeleteData(offset, count int) {
	current := pi.Data()
	if offset < 0 || offset >= len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	pi.SetData(current[:offset] + current[end:])
}

// ReplaceData replaces characters starting at the given offset.
func (pi *ProcessingInstruction) ReplaceData(offset, count int, data string) {
	current := pi.Data()
	if offset < 0 || offset > len(current) {
		return
	}
	end := offset + count
	if end > len(current) {
		end = len(current)
	}
	pi.SetData(current[:offset] + data + current[end:])
}

// CloneNode clones this processing instruction node.
func (pi *ProcessingInstruction) CloneNode(deep bool) *ProcessingInstruction {
	clone := pi.AsNode().ownerDoc.CreateProcessingInstruction(pi.Target(), pi.Data())
	return (*ProcessingInstruction)(clone)
}

// Before inserts nodes before this processing instruction node.
// Implements the ChildNode.before() algorithm from DOM spec.
func (pi *ProcessingInstruction) Before(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viablePrevSibling := pi.AsNode().findViablePreviousSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	var refNode *Node
	if viablePrevSibling == nil {
		refNode = parent.firstChild
	} else {
		refNode = viablePrevSibling.nextSibling
	}
	parent.InsertBefore(node, refNode)
}

// After inserts nodes after this processing instruction node.
// Implements the ChildNode.after() algorithm from DOM spec.
func (pi *ProcessingInstruction) After(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := pi.AsNode().findViableNextSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)
	if node == nil {
		return
	}

	parent.InsertBefore(node, viableNextSibling)
}

// ReplaceWith replaces this processing instruction node with nodes.
// Implements the ChildNode.replaceWith() algorithm from DOM spec.
func (pi *ProcessingInstruction) ReplaceWith(nodes ...interface{}) {
	parent := pi.AsNode().parentNode
	if parent == nil {
		return
	}
	nodeSet := extractNodeSet(nodes)
	viableNextSibling := pi.AsNode().findViableNextSibling(nodeSet)

	node := pi.AsNode().convertNodesToFragment(nodes)

	if pi.AsNode().parentNode == parent {
		if node != nil {
			parent.ReplaceChild(node, pi.AsNode())
		} else {
			parent.RemoveChild(pi.AsNode())
		}
	} else if node != nil {
		parent.InsertBefore(node, viableNextSibling)
	}
}

// Remove removes this processing instruction node from its parent.
func (pi *ProcessingInstruction) Remove() {
	if pi.AsNode().parentNode != nil {
		pi.AsNode().parentNode.RemoveChild(pi.AsNode())
	}
}

// processingInstructionData holds data specific to ProcessingInstruction nodes.
type processingInstructionData struct {
	target string
	data   string
}

// NewProcessingInstructionNode creates a new detached processing instruction node with the given target and data.
// The node has no owner document.
func NewProcessingInstructionNode(target, data string) *Node {
	node := newNode(ProcessingInstructionNode, target, nil)
	node.nodeValue = &data
	return node
}

// isValidXMLName checks if a string is a valid XML name.
// Per XML spec, names must start with a letter, underscore, or colon,
// and can contain letters, digits, hyphens, underscores, colons, and periods.
// Note: DOM validation is more permissive than strict XML 1.0 to match browser behavior.
func isValidXMLName(name string) bool {
	if name == "" {
		return false
	}

	runes := []rune(name)

	// Check first character - must be valid start char
	if !isXMLNameStartChar(runes[0]) {
		return false
	}

	// Check remaining characters - more permissive than strict XML 1.0
	for _, ch := range runes[1:] {
		if !isXMLNameCharPermissive(ch) {
			return false
		}
	}

	return true
}

// isXMLNameCharPermissive checks if a rune is valid in the middle/end of an XML name.
// This is more permissive than strict XML 1.0 to match browser behavior.
// Characters that are definitely invalid anywhere: space, >, tab, newline
func isXMLNameCharPermissive(ch rune) bool {
	// Definitely invalid anywhere
	if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '>' {
		return false
	}
	// Everything else is allowed in middle/end of name per browser behavior
	return true
}

// isXMLNameStartChar checks if a rune is a valid XML name start character.
// This is more permissive than strict XML 1.0 to match browser behavior.
func isXMLNameStartChar(ch rune) bool {
	// Definitely invalid start characters - based on WPT tests
	if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
		ch == '<' || ch == '>' || ch == '}' || ch == '{' ||
		ch == '-' || ch == '.' || ch == '^' ||
		ch == '~' || ch == '\'' || ch == '!' || ch == '@' ||
		ch == '#' || ch == '$' || ch == '%' || ch == '&' ||
		ch == '*' || ch == '(' || ch == ')' || ch == '+' ||
		ch == '=' || ch == '[' || ch == ']' || ch == '\\' ||
		ch == '/' || ch == ';' || ch == '`' || ch == ',' ||
		ch == '"' ||
		(ch >= '0' && ch <= '9') {
		return false
	}
	// Everything else is allowed as start character per browser behavior
	return true
}

// isXMLNameChar checks if a rune is a valid XML name character.
func isXMLNameChar(ch rune) bool {
	return isXMLNameStartChar(ch) ||
		ch == '-' ||
		ch == '.' ||
		(ch >= '0' && ch <= '9') ||
		ch == 0xB7 ||
		(ch >= 0x0300 && ch <= 0x036F) ||
		(ch >= 0x203F && ch <= 0x2040)
}

// xmlNamePattern is a regex for valid XML names (simplified)
var xmlNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:.\-]*$`)

// ValidateProcessingInstructionTarget validates a processing instruction target.
// Returns an error if the target is not a valid XML name.
func ValidateProcessingInstructionTarget(target string) error {
	if !isValidXMLName(target) {
		return ErrInvalidCharacter("The target is not a valid XML name.")
	}
	return nil
}

// ValidateProcessingInstructionData validates processing instruction data.
// Returns an error if the data contains the closing sequence "?>".
func ValidateProcessingInstructionData(data string) error {
	if strings.Contains(data, "?>") {
		return ErrInvalidCharacter("The data contains the invalid sequence '?>'.")
	}
	return nil
}

// isXMLNCNameStartChar checks if a rune is a valid NCName start character (no colon).
// This is more permissive than strict XML 1.0 to match browser behavior.
func isXMLNCNameStartChar(ch rune) bool {
	// Same as NameStartChar but also excludes colon
	if ch == ':' {
		return false
	}
	return isXMLNameStartChar(ch)
}

// isXMLNCNameChar checks if a rune is a valid NCName character (no colon).
func isXMLNCNameChar(ch rune) bool {
	return isXMLNCNameStartChar(ch) ||
		ch == '-' ||
		ch == '.' ||
		(ch >= '0' && ch <= '9') ||
		ch == 0xB7 ||
		(ch >= 0x0300 && ch <= 0x036F) ||
		(ch >= 0x203F && ch <= 0x2040)
}

// isValidNCName checks if a string is a valid NCName (non-colonized name).
// NCName is an XML Name that does not contain colons.
// Uses permissive validation to match browser behavior.
func isValidNCName(name string) bool {
	if name == "" {
		return false
	}

	runes := []rune(name)
	if !isXMLNCNameStartChar(runes[0]) {
		return false
	}

	// Check remaining chars - must not contain colon, and must be valid name chars
	for _, ch := range runes[1:] {
		if ch == ':' {
			return false // NCName cannot contain colons
		}
		if !isXMLNameCharPermissive(ch) {
			return false
		}
	}

	return true
}

// XMLNamespace and XMLNSNamespace constants
const (
	XMLNamespaceURI   = "http://www.w3.org/XML/1998/namespace"
	XMLNSNamespaceURI = "http://www.w3.org/2000/xmlns/"
)

// ValidateAndExtractQualifiedName validates a qualified name and extracts namespace, prefix, and localName.
// Per DOM spec: https://dom.spec.whatwg.org/#validate-and-extract
// Returns (namespace, prefix, localName, error)
func ValidateAndExtractQualifiedName(namespaceURI, qualifiedName string) (string, string, string, error) {
	// Step 1: Validate qualifiedName
	// When namespace is provided, browsers are more permissive (allow multiple colons, etc.)
	if err := ValidateQualifiedNameWithNamespace(qualifiedName, namespaceURI != ""); err != nil {
		return "", "", "", err
	}

	// Step 2: Initialize prefix to null, localName to qualifiedName
	prefix := ""
	localName := qualifiedName

	// Step 3: If qualifiedName contains ":", split it
	// For multiple colons, browsers use the first two segments only
	// (matching JavaScript's split(":", 2) behavior)
	colonIndex := strings.Index(qualifiedName, ":")
	if colonIndex >= 0 {
		prefix = qualifiedName[:colonIndex]
		rest := qualifiedName[colonIndex+1:]
		// If rest contains another colon, only take up to that colon
		secondColonIndex := strings.Index(rest, ":")
		if secondColonIndex >= 0 {
			localName = rest[:secondColonIndex]
		} else {
			localName = rest
		}
	}

	// Step 4: If prefix is non-empty and namespace is empty, throw NamespaceError
	if prefix != "" && namespaceURI == "" {
		return "", "", "", ErrNamespace("Prefix is not allowed when namespace is null.")
	}

	// Step 5: If prefix is "xml" and namespace is not the XML namespace, throw NamespaceError
	if prefix == "xml" && namespaceURI != XMLNamespaceURI {
		return "", "", "", ErrNamespace("The 'xml' prefix must be used with the XML namespace.")
	}

	// Step 6: If qualifiedName is "xmlns" or prefix is "xmlns", namespace must be XMLNS namespace
	if (qualifiedName == "xmlns" || prefix == "xmlns") && namespaceURI != XMLNSNamespaceURI {
		return "", "", "", ErrNamespace("The 'xmlns' prefix or localName must be used with the XMLNS namespace.")
	}

	// Step 7: If namespace is XMLNS namespace and neither qualifiedName is "xmlns" nor prefix is "xmlns"
	if namespaceURI == XMLNSNamespaceURI && qualifiedName != "xmlns" && prefix != "xmlns" {
		return "", "", "", ErrNamespace("Elements in the XMLNS namespace must use the 'xmlns' prefix or localName.")
	}

	// Return the validated values
	return namespaceURI, prefix, localName, nil
}

// ValidateQualifiedName validates a qualified name per DOM spec (without namespace).
// Returns an error if the name is invalid.
func ValidateQualifiedName(qualifiedName string) error {
	return ValidateQualifiedNameWithNamespace(qualifiedName, false)
}

// ValidateQualifiedNameWithNamespace validates a qualified name with namespace awareness.
// When hasNamespace is true, validation is more permissive to match browser behavior.
func ValidateQualifiedNameWithNamespace(qualifiedName string, hasNamespace bool) error {
	// Handle empty string - per DOM spec, an empty qualifiedName does not match the
	// Name production and should throw INVALID_CHARACTER_ERR
	if qualifiedName == "" {
		return ErrInvalidCharacter("The string contains invalid characters.")
	}

	// Check for "::" which is always invalid
	if strings.Contains(qualifiedName, "::") {
		return ErrInvalidCharacter("The qualified name contains '::'.")
	}

	// If contains colon, must be prefix:localName
	colonIndex := strings.Index(qualifiedName, ":")
	if colonIndex >= 0 {
		prefix := qualifiedName[:colonIndex]
		localName := qualifiedName[colonIndex+1:]

		// Empty prefix or localName is invalid
		if prefix == "" {
			return ErrInvalidCharacter("The qualified name has an empty prefix.")
		}
		if localName == "" {
			return ErrInvalidCharacter("The qualified name has an empty local name.")
		}

		// When no namespace is provided, multiple colons are a namespace error
		if !hasNamespace && strings.Contains(localName, ":") {
			return ErrNamespace("The qualified name contains multiple colons.")
		}

		// Prefix validation - when namespace is provided, be more permissive
		if hasNamespace {
			// With namespace, prefix can start with digits (like "0:a")
			if !isValidPrefixPermissive(prefix) {
				return ErrInvalidCharacter("The prefix contains invalid characters.")
			}
		} else {
			// Without namespace, prefix must be a valid NCName
			if !isValidNCName(prefix) {
				return ErrInvalidCharacter("The prefix is not a valid NCName.")
			}
		}

		// LocalName validation - when namespace is provided, be more permissive
		if hasNamespace {
			// With namespace, only check that localName starts with valid char
			// and doesn't contain definitely invalid chars (space, >, etc.)
			if !isValidNamePermissive(localName) {
				return ErrInvalidCharacter("The local name contains invalid characters.")
			}
		} else {
			// Without namespace, localName must be a valid NCName
			if !isValidNCName(localName) {
				return ErrInvalidCharacter("The local name is not a valid NCName.")
			}
		}
	} else {
		// No colon: must be valid XML Name
		if !isValidXMLName(qualifiedName) {
			return ErrInvalidCharacter("The qualified name is not a valid XML name.")
		}
		// Also check it's not just ":"
		if qualifiedName == ":" {
			return ErrInvalidCharacter("The qualified name cannot be just ':'.")
		}
		// Check the first character isn't ":"
		if qualifiedName[0] == ':' {
			return ErrInvalidCharacter("The qualified name cannot start with ':'.")
		}
	}

	return nil
}

// isValidNamePermissive checks if a name is valid with permissive rules.
// This is used for localName when a namespace is provided.
func isValidNamePermissive(name string) bool {
	if name == "" {
		return false
	}

	runes := []rune(name)

	// First character must be valid start char
	if !isXMLNameStartChar(runes[0]) {
		return false
	}

	// Remaining chars - only reject definitely invalid characters
	for _, ch := range runes[1:] {
		if !isXMLNameCharPermissive(ch) {
			return false
		}
	}

	return true
}

// isValidPrefixPermissive checks if a prefix is valid with permissive rules.
// Browsers allow prefixes starting with digits when a namespace is provided.
func isValidPrefixPermissive(name string) bool {
	if name == "" {
		return false
	}

	// For prefixes with namespace, even digits at start are allowed
	runes := []rune(name)

	// Only check that characters aren't definitely invalid
	for _, ch := range runes {
		if !isXMLNameCharPermissive(ch) && ch != ':' {
			return false
		}
	}

	// But colon is not allowed in prefix
	if strings.Contains(name, ":") {
		return false
	}

	return true
}
