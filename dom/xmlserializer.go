package dom

import (
	"fmt"
	"strings"
)

// SerializeToXML serializes a Node to an XML string per the DOM Parsing and Serialization spec.
// This is used by XMLSerializer.serializeToString().
// Per spec, it throws InvalidStateError if the node cannot be serialized.
func SerializeToXML(node *Node) (string, error) {
	var sb strings.Builder
	if err := serializeNodeToXML(node, &sb, nil, make(map[string]string), 1); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// namespacePrefixMap tracks namespace URI to prefix mappings for XML serialization.
// The "add" method generates new prefixes when needed.
type namespacePrefixMap struct {
	// Mapping from namespace URI to list of prefixes (can have multiple)
	ns map[string][]string
	// Index for generating new prefixes
	prefixIndex int
}

func newNamespacePrefixMap() *namespacePrefixMap {
	return &namespacePrefixMap{
		ns:          make(map[string][]string),
		prefixIndex: 1,
	}
}

// hasPrefix checks if a prefix is registered for a namespace URI
func (m *namespacePrefixMap) hasPrefix(ns, prefix string) bool {
	prefixes := m.ns[ns]
	for _, p := range prefixes {
		return p == prefix
	}
	return false
}

// getPreferredPrefix returns a suitable prefix for the namespace URI
func (m *namespacePrefixMap) getPreferredPrefix(ns string, preferredPrefix string) string {
	prefixes := m.ns[ns]
	// If preferred prefix is in the list, use it
	for _, p := range prefixes {
		if p == preferredPrefix {
			return p
		}
	}
	// Otherwise return the last one (most recently added)
	if len(prefixes) > 0 {
		return prefixes[len(prefixes)-1]
	}
	return ""
}

// add adds a prefix for a namespace URI
func (m *namespacePrefixMap) add(ns, prefix string) {
	m.ns[ns] = append(m.ns[ns], prefix)
}

// generatePrefix generates a new unique prefix
func (m *namespacePrefixMap) generatePrefix(ns string) string {
	prefix := fmt.Sprintf("ns%d", m.prefixIndex)
	m.prefixIndex++
	m.add(ns, prefix)
	return prefix
}

// copy creates a copy of the prefix map for child scopes
func (m *namespacePrefixMap) copy() *namespacePrefixMap {
	newMap := &namespacePrefixMap{
		ns:          make(map[string][]string),
		prefixIndex: m.prefixIndex,
	}
	for k, v := range m.ns {
		newSlice := make([]string, len(v))
		copy(newSlice, v)
		newMap.ns[k] = newSlice
	}
	return newMap
}

// serializeNodeToXML recursively serializes a node to XML.
// inheritedNS is the namespace inherited from the parent element.
// prefixMap tracks namespace-to-prefix mappings.
// prefixIndex is used to generate unique prefixes.
func serializeNodeToXML(n *Node, sb *strings.Builder, inheritedNS *string, localPrefixes map[string]string, prefixIndex int) error {
	switch n.nodeType {
	case ElementNode:
		return serializeElementToXML((*Element)(n), sb, inheritedNS, localPrefixes, prefixIndex)

	case TextNode:
		// Escape text content for XML: & < >
		text := n.NodeValue()
		sb.WriteString(escapeXMLText(text))
		return nil

	case CommentNode:
		// Check for invalid comment content
		data := n.NodeValue()
		if strings.Contains(data, "--") {
			return ErrInvalidState("Comment data contains '--' which is not allowed in XML.")
		}
		if strings.HasSuffix(data, "-") {
			return ErrInvalidState("Comment data ends with '-' which is not allowed in XML.")
		}
		sb.WriteString("<!--")
		sb.WriteString(data)
		sb.WriteString("-->")
		return nil

	case CDATASectionNode:
		// CDATA sections are serialized with <![CDATA[...]]>
		data := n.NodeValue()
		if strings.Contains(data, "]]>") {
			return ErrInvalidState("CDATA section contains ']]>' which is not allowed.")
		}
		sb.WriteString("<![CDATA[")
		sb.WriteString(data)
		sb.WriteString("]]>")
		return nil

	case ProcessingInstructionNode:
		target := n.nodeName
		data := n.NodeValue()
		if strings.Contains(data, "?>") {
			return ErrInvalidState("ProcessingInstruction data contains '?>' which is not allowed.")
		}
		sb.WriteString("<?")
		sb.WriteString(target)
		if data != "" {
			sb.WriteString(" ")
			sb.WriteString(data)
		}
		sb.WriteString("?>")
		return nil

	case DocumentNode:
		// Serialize all children
		for child := n.firstChild; child != nil; child = child.nextSibling {
			if err := serializeNodeToXML(child, sb, nil, make(map[string]string), prefixIndex); err != nil {
				return err
			}
		}
		return nil

	case DocumentFragmentNode:
		// Serialize all children
		for child := n.firstChild; child != nil; child = child.nextSibling {
			if err := serializeNodeToXML(child, sb, inheritedNS, localPrefixes, prefixIndex); err != nil {
				return err
			}
		}
		return nil

	case DocumentTypeNode:
		// Serialize doctype
		name := n.DoctypeName()
		publicId := n.DoctypePublicId()
		systemId := n.DoctypeSystemId()
		if name != "" {
			sb.WriteString("<!DOCTYPE ")
			sb.WriteString(name)
			if publicId != "" || systemId != "" {
				if publicId != "" {
					sb.WriteString(" PUBLIC \"")
					sb.WriteString(publicId)
					sb.WriteString("\"")
					if systemId != "" {
						sb.WriteString(" \"")
						sb.WriteString(systemId)
						sb.WriteString("\"")
					}
				} else {
					sb.WriteString(" SYSTEM \"")
					sb.WriteString(systemId)
					sb.WriteString("\"")
				}
			}
			sb.WriteString(">")
		}
		return nil

	case AttributeNode:
		// Attributes are serialized as part of elements
		return nil

	default:
		// Unknown node type - skip
		return nil
	}
}

// serializeElementToXML serializes an element and its children to XML.
func serializeElementToXML(el *Element, sb *strings.Builder, inheritedNS *string, localPrefixes map[string]string, prefixIndex int) error {
	// Get element's namespace and local name
	ns := el.NamespaceURI()
	localName := el.LocalName()
	prefix := el.Prefix()

	// Create a copy of local prefixes for this element's scope
	scopedPrefixes := make(map[string]string)
	for k, v := range localPrefixes {
		scopedPrefixes[k] = v
	}

	// Determine the qualified name and any namespace declarations needed
	var qualifiedName string
	var nsDeclarations []string

	// Track if we need to declare the element's namespace
	needsNSDecl := false
	var declPrefix string
	var declNS string

	if ns != "" {
		// Element has a namespace
		if prefix != "" {
			// Element has a prefix - check if it's already declared
			qualifiedName = prefix + ":" + localName
			existingNS, exists := scopedPrefixes[prefix]
			if !exists || existingNS != ns {
				// Need to declare this prefix
				needsNSDecl = true
				declPrefix = prefix
				declNS = ns
				scopedPrefixes[prefix] = ns
			}
		} else {
			// No prefix - check if namespace matches inherited
			qualifiedName = localName
			if inheritedNS == nil || *inheritedNS != ns {
				// Need to declare default namespace
				needsNSDecl = true
				declPrefix = ""
				declNS = ns
			}
		}
	} else {
		// No namespace
		qualifiedName = localName
		if inheritedNS != nil && *inheritedNS != "" {
			// Need to undeclare the default namespace
			needsNSDecl = true
			declPrefix = ""
			declNS = ""
		}
	}

	// Start element tag
	sb.WriteString("<")
	sb.WriteString(qualifiedName)

	// Add namespace declaration if needed
	if needsNSDecl {
		if declPrefix == "" {
			nsDeclarations = append(nsDeclarations, fmt.Sprintf(` xmlns="%s"`, escapeXMLAttrValue(declNS)))
		} else {
			nsDeclarations = append(nsDeclarations, fmt.Sprintf(` xmlns:%s="%s"`, declPrefix, escapeXMLAttrValue(declNS)))
		}
	}

	// Serialize attributes
	attrs := el.Attributes()
	for i := 0; i < attrs.Length(); i++ {
		attr := attrs.Item(i)
		if attr == nil {
			continue
		}

		attrNS := attr.NamespaceURI()
		attrLocalName := attr.LocalName()
		attrPrefix := attr.Prefix()
		attrValue := attr.Value()

		// Skip xmlns declarations - we handle them separately
		if attrNS == XMLNSNamespaceURI || attr.Name() == "xmlns" {
			// Check if this is a prefix declaration we need to track
			if attrLocalName != "xmlns" && attrPrefix == "xmlns" {
				scopedPrefixes[attrLocalName] = attrValue
			}
			continue
		}

		var attrQualifiedName string
		if attrNS != "" && attrNS != ns {
			// Attribute has a different namespace than the element
			if attrPrefix != "" {
				attrQualifiedName = attrPrefix + ":" + attrLocalName
				// Check if we need to declare this namespace
				existingNS, exists := scopedPrefixes[attrPrefix]
				if !exists || existingNS != attrNS {
					nsDeclarations = append(nsDeclarations, fmt.Sprintf(` xmlns:%s="%s"`, attrPrefix, escapeXMLAttrValue(attrNS)))
					scopedPrefixes[attrPrefix] = attrNS
				}
			} else {
				// Need to generate a prefix for this namespace
				genPrefix := fmt.Sprintf("ns%d", prefixIndex)
				prefixIndex++
				attrQualifiedName = genPrefix + ":" + attrLocalName
				nsDeclarations = append(nsDeclarations, fmt.Sprintf(` xmlns:%s="%s"`, genPrefix, escapeXMLAttrValue(attrNS)))
				scopedPrefixes[genPrefix] = attrNS
			}
		} else {
			attrQualifiedName = attrLocalName
		}

		sb.WriteString(" ")
		sb.WriteString(attrQualifiedName)
		sb.WriteString("=\"")
		sb.WriteString(escapeXMLAttrValue(attrValue))
		sb.WriteString("\"")
	}

	// Write namespace declarations
	for _, decl := range nsDeclarations {
		sb.WriteString(decl)
	}

	// Check for void elements in HTML namespace
	if ns == HTMLNamespace && isVoidElement(strings.ToLower(localName)) && el.AsNode().firstChild == nil {
		sb.WriteString(" />")
		return nil
	}

	// Check if element has children
	if el.AsNode().firstChild == nil {
		sb.WriteString("/>")
		return nil
	}

	sb.WriteString(">")

	// Determine the namespace for children
	var childInheritedNS *string
	if ns != "" && prefix == "" {
		childInheritedNS = &ns
	} else if ns == "" && inheritedNS != nil {
		// Undeclared default namespace
		empty := ""
		childInheritedNS = &empty
	} else {
		childInheritedNS = inheritedNS
	}

	// Serialize children
	for child := el.AsNode().firstChild; child != nil; child = child.nextSibling {
		if err := serializeNodeToXML(child, sb, childInheritedNS, scopedPrefixes, prefixIndex); err != nil {
			return err
		}
	}

	// End tag
	sb.WriteString("</")
	sb.WriteString(qualifiedName)
	sb.WriteString(">")

	return nil
}

// escapeXMLText escapes text content for XML: & < >
func escapeXMLText(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// escapeXMLAttrValue escapes attribute values for XML: & < > " '
func escapeXMLAttrValue(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		case '"':
			sb.WriteString("&quot;")
		case '\'':
			sb.WriteString("&apos;")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
