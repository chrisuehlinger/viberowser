package dom

import (
	"strings"
	"unicode"
)

// InnerText returns the "rendered" text content of an element.
// This is different from TextContent in that it respects CSS styling:
// - Hidden elements (display:none, visibility:hidden) are excluded
// - Block-level elements add line breaks
// - Whitespace is collapsed according to CSS white-space rules
// - <br> elements produce line breaks
//
// Per HTML spec, for elements that are not being rendered (display:none or
// SVG/MathML elements), innerText falls back to textContent behavior.
func (e *Element) InnerText() string {
	// Per spec: SVG and MathML elements don't support innerText
	ns := e.NamespaceURI()
	if ns == SVGNamespace || ns == MathMLNamespace {
		// Return undefined in JS, which we represent as empty string
		return ""
	}

	// Elements that hide their children for innerText purposes
	localName := e.LocalName()
	switch localName {
	case "textarea", "input":
		// Form controls don't expose their content through innerText
		return ""
	case "iframe", "audio", "video", "canvas", "object":
		// These replaced elements hide their fallback content
		return ""
	}

	// Check if element is being rendered
	// For now, we'll check display:none via style attribute
	// A full implementation would use computed styles
	if !e.isBeingRendered() {
		// Not being rendered - use textContent-like behavior but preserve whitespace
		var sb strings.Builder
		e.collectInnerTextNotRendered(e.AsNode(), &sb)
		return sb.String()
	}

	// Element is being rendered - compute rendered text
	var result strings.Builder

	// Determine initial whitespace mode from the element itself
	initialWhiteSpace := e.getWhiteSpaceMode(e, "normal")

	// Check visibility of root element
	initialVisibility := "visible"
	style := e.GetAttribute("style")
	styleLower := strings.ToLower(style)
	if strings.Contains(styleLower, "visibility") {
		if strings.Contains(styleLower, "hidden") {
			initialVisibility = "hidden"
		} else if strings.Contains(styleLower, "collapse") {
			initialVisibility = "collapse"
		}
	}

	ctx := &innerTextContext{
		whiteSpaceMode:        initialWhiteSpace,
		lastOutputWasNewline:  true, // Start as if after a newline (for leading whitespace)
		lastOutputWasSpace:    false,
		afterBlockStart:       true,
		needsLeadingNewline:   false,
		needsTrailingNewlines: 0,
		visibility:            initialVisibility,
	}

	e.collectInnerText(e.AsNode(), &result, ctx)

	// Trim trailing whitespace (but only regular spaces in normal mode)
	text := result.String()
	if initialWhiteSpace == "normal" || initialWhiteSpace == "nowrap" {
		text = strings.TrimRight(text, " ")
	}

	return text
}

// innerTextContext tracks state during innerText collection
type innerTextContext struct {
	whiteSpaceMode        string
	lastOutputWasNewline  bool
	lastOutputWasSpace    bool
	afterBlockStart       bool
	needsLeadingNewline   bool
	needsTrailingNewlines int    // Number of pending newlines
	visibility            string // "visible", "hidden", or "collapse"
}

// isBeingRendered checks if the element is being rendered.
// An element is not rendered if:
// - It or an ancestor has display:none
// - It's in a non-rendered container (canvas fallback, audio/video fallback, etc.)
func (e *Element) isBeingRendered() bool {
	// Check if this element or ancestors have display:none via inline style
	current := e
	for current != nil {
		style := current.GetAttribute("style")
		if style != "" {
			styleLower := strings.ToLower(style)
			if strings.Contains(styleLower, "display") && strings.Contains(styleLower, "none") {
				// Has display:none - but the target element itself is still "rendered" for innerText purposes
				if current == e {
					// The element itself has display:none, treat as not rendered
					return false
				}
				// An ancestor has display:none - check if we're the target
				// Target elements in display:none containers still use textContent-like behavior
				return false
			}
		}
		parent := current.AsNode().parentNode
		if parent == nil || parent.nodeType != ElementNode {
			break
		}
		current = (*Element)(parent)
	}
	return true
}

// collectInnerTextNotRendered collects text when element is not being rendered
// This is similar to textContent but preserves whitespace
func (e *Element) collectInnerTextNotRendered(n *Node, sb *strings.Builder) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		switch child.nodeType {
		case TextNode, CDATASectionNode:
			sb.WriteString(child.NodeValue())
		case ElementNode:
			childEl := (*Element)(child)
			// Skip children of replaced elements that have fallback content
			localName := childEl.LocalName()
			if localName == "textarea" || localName == "input" ||
				localName == "iframe" || localName == "audio" ||
				localName == "video" || localName == "canvas" {
				continue
			}
			e.collectInnerTextNotRendered(child, sb)
		}
	}
}

// collectInnerText recursively collects rendered text
func (e *Element) collectInnerText(n *Node, result *strings.Builder, ctx *innerTextContext) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		switch child.nodeType {
		case TextNode, CDATASectionNode:
			text := child.NodeValue()
			e.processText(text, result, ctx)

		case ElementNode:
			childEl := (*Element)(child)
			e.processElement(childEl, result, ctx)
		}
	}
}

// collectSVGInnerText processes SVG content, skipping non-rendered elements
func (e *Element) collectSVGInnerText(n *Node, result *strings.Builder, ctx *innerTextContext) {
	// SVG elements that don't render their content
	nonRenderedSVG := map[string]bool{
		"defs": true, "symbol": true, "clippath": true, "mask": true,
		"pattern": true, "lineargradient": true, "radialgradient": true,
		"filter": true, "marker": true, "stop": true,
	}

	for child := n.firstChild; child != nil; child = child.nextSibling {
		switch child.nodeType {
		case TextNode, CDATASectionNode:
			text := child.NodeValue()
			e.processText(text, result, ctx)

		case ElementNode:
			childEl := (*Element)(child)
			localName := childEl.LocalName()

			// Skip non-rendered SVG elements
			if nonRenderedSVG[localName] {
				continue
			}

			// foreignObject goes back to HTML processing
			if localName == "foreignobject" {
				e.collectInnerText(child, result, ctx)
				continue
			}

			// Other SVG elements - recurse
			e.collectSVGInnerText(child, result, ctx)
		}
	}
}

// processText processes a text node's content according to CSS white-space rules
func (e *Element) processText(text string, result *strings.Builder, ctx *innerTextContext) {
	if text == "" {
		return
	}

	// Don't output text if visibility is hidden
	if ctx.visibility == "hidden" || ctx.visibility == "collapse" {
		return
	}

	// Determine white-space mode from parent context
	preserveNewlines := ctx.whiteSpaceMode == "pre" || ctx.whiteSpaceMode == "pre-wrap" || ctx.whiteSpaceMode == "pre-line"
	preserveSpaces := ctx.whiteSpaceMode == "pre" || ctx.whiteSpaceMode == "pre-wrap"

	var sb strings.Builder
	prevWasSpace := ctx.lastOutputWasSpace || ctx.afterBlockStart

	for _, r := range text {
		switch r {
		case '\n':
			if preserveNewlines {
				sb.WriteRune('\n')
				prevWasSpace = true
				ctx.lastOutputWasNewline = true
				ctx.lastOutputWasSpace = false
			} else {
				// Convert to space and collapse
				if !prevWasSpace {
					sb.WriteRune(' ')
					prevWasSpace = true
					ctx.lastOutputWasSpace = true
				}
			}
		case '\r':
			if preserveNewlines {
				sb.WriteRune('\n')
				prevWasSpace = true
				ctx.lastOutputWasNewline = true
				ctx.lastOutputWasSpace = false
			} else {
				if !prevWasSpace {
					sb.WriteRune(' ')
					prevWasSpace = true
					ctx.lastOutputWasSpace = true
				}
			}
		case '\t':
			if preserveSpaces {
				sb.WriteRune('\t')
				prevWasSpace = false // Tabs don't collapse in pre mode
			} else {
				if !prevWasSpace {
					sb.WriteRune(' ')
					prevWasSpace = true
					ctx.lastOutputWasSpace = true
				}
			}
		case ' ':
			if preserveSpaces {
				sb.WriteRune(' ')
				prevWasSpace = false // Spaces don't collapse in pre mode
			} else {
				if !prevWasSpace {
					sb.WriteRune(' ')
					prevWasSpace = true
					ctx.lastOutputWasSpace = true
				}
			}
		default:
			sb.WriteRune(r)
			prevWasSpace = false
			ctx.lastOutputWasNewline = false
			ctx.lastOutputWasSpace = false
			ctx.afterBlockStart = false
		}
	}

	processed := sb.String()
	if processed != "" {
		// Flush any pending newlines before content
		if ctx.needsTrailingNewlines > 0 && !ctx.afterBlockStart {
			for i := 0; i < ctx.needsTrailingNewlines; i++ {
				result.WriteRune('\n')
			}
			ctx.needsTrailingNewlines = 0
			ctx.lastOutputWasNewline = true
		}

		// Write the processed text
		result.WriteString(processed)
	}
}

// processElement processes an element node
func (e *Element) processElement(el *Element, result *strings.Builder, ctx *innerTextContext) {
	localName := el.LocalName()
	ns := el.NamespaceURI()

	// SVG namespace elements don't contribute to innerText, except:
	// - foreignObject
	// - text elements (like <text>) which should render their content
	if ns == SVGNamespace {
		if localName == "foreignobject" {
			e.collectInnerText(el.AsNode(), result, ctx)
		}
		return
	}

	// For HTML namespace <svg> elements (parsed as HTML), check for non-rendered children
	if localName == "svg" {
		// Process SVG children, but skip non-rendered SVG elements like <defs>
		e.collectSVGInnerText(el.AsNode(), result, ctx)
		return
	}

	if ns == MathMLNamespace || localName == "math" {
		return
	}

	// Check display and visibility
	style := el.GetAttribute("style")
	styleLower := strings.ToLower(style)

	// Check display:none
	if strings.Contains(styleLower, "display") && strings.Contains(styleLower, "none") {
		return
	}

	// Update visibility for this element
	oldVisibility := ctx.visibility
	if strings.Contains(styleLower, "visibility") {
		if strings.Contains(styleLower, "visible") {
			ctx.visibility = "visible"
		} else if strings.Contains(styleLower, "hidden") {
			ctx.visibility = "hidden"
		} else if strings.Contains(styleLower, "collapse") {
			ctx.visibility = "collapse"
		}
	}

	// Handle replaced elements that hide their content
	switch localName {
	case "textarea":
		// Form controls don't render their text children
		return
	case "select":
		// Select renders option contents
		e.processSelectElement(el, result, ctx)
		return
	case "option", "optgroup":
		// Options and optgroups in a non-select context are block-like
		e.flushNewlines(result, ctx, 1)
		e.collectInnerText(el.AsNode(), result, ctx)
		e.flushNewlines(result, ctx, 1)
		return
	case "iframe", "audio", "video", "object":
		// These elements hide their fallback content when rendered
		return
	case "canvas", "img":
		// These are replaced elements that act as atomic inlines
		// They prevent whitespace collapse around them
		// Check for display:block on these elements
		if strings.Contains(styleLower, "display") && strings.Contains(styleLower, "block") {
			e.trimTrailingSpace(result)
			e.flushNewlines(result, ctx, 1)
			ctx.afterBlockStart = true
			ctx.needsTrailingNewlines = 1
			return
		}
		// These elements act as atomic inlines - they prevent whitespace collapsing
		ctx.lastOutputWasSpace = false
		ctx.lastOutputWasNewline = false
		ctx.afterBlockStart = false
		return
	case "input":
		// Input elements act as atomic inlines - they prevent whitespace collapsing
		// Check for display:block
		if strings.Contains(styleLower, "display") && strings.Contains(styleLower, "block") {
			e.trimTrailingSpace(result)
			e.flushNewlines(result, ctx, 1)
			ctx.afterBlockStart = true
			ctx.needsTrailingNewlines = 1
			return
		}
		ctx.lastOutputWasSpace = false
		ctx.lastOutputWasNewline = false
		ctx.afterBlockStart = false
		return
	case "script", "style", "noscript", "template":
		// These elements don't render their content
		return
	case "br":
		// <br> produces a newline - but first trim trailing space
		e.trimTrailingSpace(result)
		result.WriteRune('\n')
		ctx.lastOutputWasNewline = true
		ctx.lastOutputWasSpace = false
		ctx.afterBlockStart = true // Next text will have leading space removed
		ctx.needsTrailingNewlines = 0
		return
	case "rp":
		// rp content is hidden (it's parentheses around ruby annotation)
		return
	}

	// Check if this is an atomic inline element (inline-block, inline-flex, inline-grid)
	isAtomicInline := e.isAtomicInline(el)
	if isAtomicInline {
		// Atomic inlines prevent whitespace collapsing around them
		// but their internal whitespace follows normal rules
		ctx.lastOutputWasSpace = false
		ctx.lastOutputWasNewline = false
		ctx.afterBlockStart = false
	}

	// Determine if this is a block-level element
	isBlock := e.isBlockElement(el)
	isParagraphLike := localName == "p"

	// Save whitespace mode and update for this element
	oldMode := ctx.whiteSpaceMode
	ctx.whiteSpaceMode = e.getWhiteSpaceMode(el, ctx.whiteSpaceMode)

	if isBlock {
		// Block elements need newlines before and after
		if isParagraphLike {
			e.flushNewlines(result, ctx, 2) // Paragraphs get blank lines
		} else {
			e.flushNewlines(result, ctx, 1)
		}
		ctx.afterBlockStart = true
	}

	// Process children
	e.collectInnerText(el.AsNode(), result, ctx)

	if isBlock {
		// Trim trailing spaces from block content
		// Mark pending newlines for after block
		if isParagraphLike {
			ctx.needsTrailingNewlines = 2
		} else {
			ctx.needsTrailingNewlines = 1
		}
		ctx.afterBlockStart = false
	}

	// Restore whitespace mode and visibility
	ctx.whiteSpaceMode = oldMode
	ctx.visibility = oldVisibility
}

// processSelectElement handles <select> elements specially
func (e *Element) processSelectElement(el *Element, result *strings.Builder, ctx *innerTextContext) {
	firstOption := true
	for child := el.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType != ElementNode {
			continue
		}
		childEl := (*Element)(child)
		localName := childEl.LocalName()

		if localName == "option" {
			// Add newline before each option (except the first)
			if !firstOption {
				result.WriteRune('\n')
			}
			text := childEl.TextContent()
			result.WriteString(text)
			firstOption = false
			ctx.lastOutputWasNewline = false
		} else if localName == "optgroup" {
			// Process options within optgroup
			e.processSelectElement(childEl, result, ctx)
			firstOption = false
		}
	}
}

// trimTrailingSpace removes trailing spaces from the result (but not newlines)
func (e *Element) trimTrailingSpace(result *strings.Builder) {
	s := result.String()
	trimmed := strings.TrimRight(s, " ")
	if len(trimmed) < len(s) {
		result.Reset()
		result.WriteString(trimmed)
	}
}

// flushNewlines writes pending newlines if needed
func (e *Element) flushNewlines(result *strings.Builder, ctx *innerTextContext, count int) {
	// Only add newlines if there's content and we're not at the start
	if result.Len() == 0 {
		return
	}

	// Calculate how many newlines we need
	needed := count
	if ctx.needsTrailingNewlines > needed {
		needed = ctx.needsTrailingNewlines
	}

	// Write the newlines
	for i := 0; i < needed; i++ {
		result.WriteRune('\n')
	}

	ctx.needsTrailingNewlines = 0
	ctx.lastOutputWasNewline = true
	ctx.lastOutputWasSpace = false
	ctx.afterBlockStart = true
}

// hasVisibleDescendant checks if an element with visibility:hidden has any
// visible descendants
func (e *Element) hasVisibleDescendant(el *Element) bool {
	for child := el.AsNode().firstChild; child != nil; child = child.nextSibling {
		if child.nodeType != ElementNode {
			continue
		}
		childEl := (*Element)(child)
		style := childEl.GetAttribute("style")
		styleLower := strings.ToLower(style)
		if strings.Contains(styleLower, "visibility") && strings.Contains(styleLower, "visible") {
			return true
		}
		if e.hasVisibleDescendant(childEl) {
			return true
		}
	}
	return false
}

// isAtomicInline checks if an element is an atomic inline (inline-block, inline-flex, inline-grid)
func (e *Element) isAtomicInline(el *Element) bool {
	style := el.GetAttribute("style")
	styleLower := strings.ToLower(style)
	if strings.Contains(styleLower, "display") {
		if strings.Contains(styleLower, "inline-block") ||
			strings.Contains(styleLower, "inline-flex") ||
			strings.Contains(styleLower, "inline-grid") ||
			strings.Contains(styleLower, "inline-table") {
			return true
		}
	}
	return false
}

// isBlockElement checks if an element is block-level
func (e *Element) isBlockElement(el *Element) bool {
	// Check inline style for display property
	style := el.GetAttribute("style")
	styleLower := strings.ToLower(style)
	if strings.Contains(styleLower, "display") {
		if strings.Contains(styleLower, "inline-block") ||
			strings.Contains(styleLower, "inline-flex") ||
			strings.Contains(styleLower, "inline-grid") ||
			strings.Contains(styleLower, "inline-table") {
			return false
		}
		if strings.Contains(styleLower, "block") ||
			strings.Contains(styleLower, "flex") ||
			strings.Contains(styleLower, "grid") ||
			strings.Contains(styleLower, "table") ||
			strings.Contains(styleLower, "list-item") {
			return true
		}
	}

	// Check for float (makes element block-like for innerText)
	if strings.Contains(styleLower, "float") &&
		(strings.Contains(styleLower, "left") || strings.Contains(styleLower, "right")) {
		return true
	}

	// Check for position:absolute or position:fixed
	if strings.Contains(styleLower, "position") &&
		(strings.Contains(styleLower, "absolute") || strings.Contains(styleLower, "fixed")) {
		return true
	}

	// Default block elements
	localName := el.LocalName()
	switch localName {
	case "address", "article", "aside", "blockquote", "center", "dd", "details",
		"dialog", "dir", "div", "dl", "dt", "fieldset", "figcaption", "figure",
		"footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hgroup",
		"hr", "li", "listing", "main", "menu", "nav", "ol", "p", "plaintext",
		"pre", "section", "summary", "ul", "xmp":
		return true
	case "table", "thead", "tbody", "tfoot", "tr", "caption":
		return true
	}

	return false
}

// getWhiteSpaceMode determines the white-space CSS property value
func (e *Element) getWhiteSpaceMode(el *Element, inherited string) string {
	// Check inline style first
	style := el.GetAttribute("style")
	styleLower := strings.ToLower(style)
	if strings.Contains(styleLower, "white-space") {
		if strings.Contains(styleLower, "pre-line") {
			return "pre-line"
		}
		if strings.Contains(styleLower, "pre-wrap") {
			return "pre-wrap"
		}
		if strings.Contains(styleLower, "pre") {
			return "pre"
		}
		if strings.Contains(styleLower, "nowrap") {
			return "nowrap"
		}
		if strings.Contains(styleLower, "normal") {
			return "normal"
		}
	}

	// Check if this is a preformatted element
	localName := el.LocalName()
	switch localName {
	case "pre", "listing", "plaintext", "xmp":
		return "pre"
	case "textarea":
		return "pre-wrap"
	}

	return inherited
}

// OuterText returns the same value as InnerText.
// Per HTML spec, the outerText getter is equivalent to innerText getter.
func (e *Element) OuterText() string {
	// Per spec: SVG and MathML elements don't support outerText
	ns := e.NamespaceURI()
	if ns == SVGNamespace || ns == MathMLNamespace {
		return ""
	}
	return e.InnerText()
}

// SetInnerText replaces the element's children with text content.
// Newlines in the input are converted to <br> elements.
func (e *Element) SetInnerText(text string) {
	// Per spec: SVG and MathML elements don't support innerText setter
	ns := e.NamespaceURI()
	localName := e.LocalName()
	if ns == SVGNamespace || ns == MathMLNamespace {
		return
	}
	// Also check by tag name in case namespace wasn't set properly by parser
	if localName == "svg" || localName == "math" {
		return
	}

	n := e.AsNode()
	doc := n.ownerDoc
	if doc == nil {
		return
	}

	// Remove all existing children
	for n.firstChild != nil {
		n.RemoveChild(n.firstChild)
	}

	// If text is empty, we're done
	if text == "" {
		return
	}

	// Convert the text to nodes, replacing newlines with <br> elements
	nodes := e.textToNodes(text, doc)
	for _, node := range nodes {
		n.AppendChild(node)
	}
}

// textToNodes converts text to a slice of nodes, converting newlines to <br>
func (e *Element) textToNodes(text string, doc *Document) []*Node {
	var nodes []*Node

	// Process text, converting \r\n, \r, and \n to <br> elements
	var current strings.Builder
	i := 0
	for i < len(text) {
		r := rune(text[i])

		if r == '\r' {
			// Flush current text
			if current.Len() > 0 {
				nodes = append(nodes, doc.CreateTextNode(current.String()))
				current.Reset()
			}
			// Add <br>
			nodes = append(nodes, doc.CreateElement("br").AsNode())
			// Check for \r\n
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			i++
			continue
		}

		if r == '\n' {
			// Flush current text
			if current.Len() > 0 {
				nodes = append(nodes, doc.CreateTextNode(current.String()))
				current.Reset()
			}
			// Add <br>
			nodes = append(nodes, doc.CreateElement("br").AsNode())
			i++
			continue
		}

		current.WriteRune(r)
		i++
	}

	// Flush remaining text
	if current.Len() > 0 {
		nodes = append(nodes, doc.CreateTextNode(current.String()))
	}

	return nodes
}

// SetOuterText replaces this element with text content.
// Newlines in the input are converted to <br> elements.
// The resulting text nodes are merged with adjacent text nodes.
func (e *Element) SetOuterText(text string) error {
	// Per spec: SVG and MathML elements don't support outerText setter
	ns := e.NamespaceURI()
	localName := e.LocalName()
	if ns == SVGNamespace || ns == MathMLNamespace {
		return nil
	}
	// Also check by tag name in case namespace wasn't set properly by parser
	if localName == "svg" || localName == "math" {
		return nil
	}

	n := e.AsNode()
	parent := n.parentNode
	if parent == nil {
		return ErrNoModificationAllowed("Cannot set outerText on an element without a parent")
	}

	doc := n.ownerDoc
	if doc == nil {
		return nil
	}

	// Get siblings before modification
	prevSib := n.prevSibling
	nextSib := n.nextSibling

	// Create the new nodes
	newNodes := e.textToNodes(text, doc)

	// If there are no new nodes, insert an empty text node
	if len(newNodes) == 0 {
		newNodes = append(newNodes, doc.CreateTextNode(""))
	}

	// Remove this element from parent
	parent.RemoveChild(n)

	// Insert the new nodes where this element was
	for _, newNode := range newNodes {
		parent.InsertBefore(newNode, nextSib)
	}

	// Merge adjacent text nodes
	e.mergeAdjacentTextNodes(parent, prevSib, nextSib, newNodes)

	return nil
}

// mergeAdjacentTextNodes merges the newly inserted text nodes with adjacent ones
func (e *Element) mergeAdjacentTextNodes(parent *Node, prevSib, nextSib *Node, newNodes []*Node) {
	if len(newNodes) == 0 {
		return
	}

	firstNew := newNodes[0]
	lastNew := newNodes[len(newNodes)-1]

	// Merge first new node with previous sibling if both are text
	if prevSib != nil && prevSib.nodeType == TextNode && firstNew.nodeType == TextNode {
		// Append the new text to the previous text node
		prevText := prevSib.NodeValue()
		newText := firstNew.NodeValue()
		prevSib.SetNodeValue(prevText + newText)
		parent.RemoveChild(firstNew)

		// Update lastNew if there was only one new node
		if lastNew == firstNew {
			lastNew = prevSib
		}
	}

	// Merge last new node with next sibling if both are text
	if nextSib != nil && nextSib.nodeType == TextNode && lastNew.nodeType == TextNode {
		// Prepend the new text to the next text node
		lastText := lastNew.NodeValue()
		nextText := nextSib.NodeValue()
		lastNew.SetNodeValue(lastText + nextText)
		parent.RemoveChild(nextSib)
	}
}

// transformText applies text-transform CSS property
func (e *Element) transformText(text string, transform string) string {
	switch strings.ToLower(transform) {
	case "uppercase":
		return strings.ToUpper(text)
	case "lowercase":
		return strings.ToLower(text)
	case "capitalize":
		return capitalizeWords(text)
	default:
		return text
	}
}

// capitalizeWords capitalizes the first letter of each word
func capitalizeWords(s string) string {
	var result strings.Builder
	capitalizeNext := true

	for _, r := range s {
		if unicode.IsSpace(r) {
			result.WriteRune(r)
			capitalizeNext = true
		} else if capitalizeNext && unicode.IsLetter(r) {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
			capitalizeNext = false
		}
	}

	return result.String()
}
