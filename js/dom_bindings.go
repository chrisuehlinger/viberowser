package js

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
	"github.com/dop251/goja"
)

// utf16Length returns the length of a string in UTF-16 code units.
// This matches JavaScript's String.length behavior.
func utf16Length(s string) int {
	return len(stringToUTF16(s))
}

// gojaValueToUTF16 extracts UTF-16 code units from a goja.Value.
// This preserves surrogates that would be lost when converting to Go strings.
func gojaValueToUTF16(v goja.Value) []uint16 {
	if str, ok := v.(goja.String); ok {
		// Use goja's CharAt method to get exact UTF-16 code units
		length := str.Length()
		result := make([]uint16, length)
		for i := 0; i < length; i++ {
			result[i] = str.CharAt(i)
		}
		return result
	}
	// Fallback for non-String values
	return stringToUTF16(v.String())
}

// utf16ToGojaValue creates a goja.Value from UTF-16 code units.
// This preserves surrogates by using goja.StringFromUTF16.
func utf16ToGojaValue(vm *goja.Runtime, units []uint16) goja.Value {
	return vm.ToValue(goja.StringFromUTF16(units))
}

// stringToUTF16 converts a Go string (which may be WTF-8) to UTF-16 code units.
// This handles:
// - Normal UTF-8 encoded characters
// - WTF-8 encoded surrogates (which appear as 3-byte sequences starting with ED)
func stringToUTF16(s string) []uint16 {
	result := make([]uint16, 0, len(s))
	data := []byte(s)
	for i := 0; i < len(data); {
		// Check for WTF-8 encoded surrogate (ED A0-BF 80-BF)
		// High surrogate (D800-DBFF): ED A0-AF 80-BF
		// Low surrogate (DC00-DFFF): ED B0-BF 80-BF
		if i+2 < len(data) && data[i] == 0xED {
			b1, b2 := data[i+1], data[i+2]
			if (b1 >= 0xA0 && b1 <= 0xBF) && (b2 >= 0x80 && b2 <= 0xBF) {
				// Decode WTF-8 surrogate using standard 3-byte UTF-8 formula
				cu := uint16((uint32(0xED&0x0F) << 12) | (uint32(b1&0x3F) << 6) | uint32(b2&0x3F))
				result = append(result, cu)
				i += 3
				continue
			}
		}

		// Decode as normal UTF-8
		r, size := decodeRune(data[i:])
		if r >= 0x10000 {
			// Supplementary plane character - encode as surrogate pair
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)))
			result = append(result, uint16(0xDC00+(r&0x3FF)))
		} else {
			result = append(result, uint16(r))
		}
		i += size
	}
	return result
}

// decodeRune decodes a single UTF-8 rune from a byte slice.
// Returns the rune and the number of bytes consumed.
func decodeRune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0xFFFD, 1
	}
	c := b[0]
	if c < 0x80 {
		return rune(c), 1
	}
	if c < 0xC0 {
		return 0xFFFD, 1
	}
	if c < 0xE0 && len(b) >= 2 {
		return rune(c&0x1F)<<6 | rune(b[1]&0x3F), 2
	}
	if c < 0xF0 && len(b) >= 3 {
		return rune(c&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
	}
	if c < 0xF8 && len(b) >= 4 {
		return rune(c&0x07)<<18 | rune(b[1]&0x3F)<<12 | rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
	}
	return 0xFFFD, 1
}

// utf16ToString converts UTF-16 code units to a Go string using WTF-8 encoding.
// Unpaired surrogates are encoded as WTF-8 three-byte sequences, which allows
// round-tripping through Go strings.
func utf16ToString(codeUnits []uint16) string {
	// Estimate size: most code units are 1-3 bytes in WTF-8
	buf := make([]byte, 0, len(codeUnits)*3)

	for i := 0; i < len(codeUnits); i++ {
		cu := codeUnits[i]

		// Check for surrogate pair
		if cu >= 0xD800 && cu <= 0xDBFF && i+1 < len(codeUnits) {
			next := codeUnits[i+1]
			if next >= 0xDC00 && next <= 0xDFFF {
				// Valid surrogate pair - encode as single 4-byte UTF-8
				r := 0x10000 + (rune(cu-0xD800) << 10) + rune(next-0xDC00)
				buf = append(buf,
					byte(0xF0|(r>>18)),
					byte(0x80|((r>>12)&0x3F)),
					byte(0x80|((r>>6)&0x3F)),
					byte(0x80|(r&0x3F)))
				i++
				continue
			}
		}

		// Encode single code unit
		if cu < 0x80 {
			buf = append(buf, byte(cu))
		} else if cu < 0x800 {
			buf = append(buf, byte(0xC0|(cu>>6)), byte(0x80|(cu&0x3F)))
		} else {
			// 3-byte encoding (includes unpaired surrogates via WTF-8)
			buf = append(buf, byte(0xE0|(cu>>12)), byte(0x80|((cu>>6)&0x3F)), byte(0x80|(cu&0x3F)))
		}
	}

	return string(buf)
}

// utf16Substring extracts a substring using UTF-16 code unit offsets.
// This matches JavaScript's String.substring behavior for proper Unicode handling.
// Unpaired surrogates are preserved if the substring splits a surrogate pair.
func utf16Substring(s string, offset, count int) string {
	codeUnits := stringToUTF16(s)
	if offset >= len(codeUnits) {
		return ""
	}
	end := offset + count
	if end > len(codeUnits) {
		end = len(codeUnits)
	}
	// Convert back to string using WTF-8 to preserve unpaired surrogates
	return utf16ToString(codeUnits[offset:end])
}

// utf16ReplaceRange replaces a range of UTF-16 code units in a string.
// Unpaired surrogates are preserved if the operation splits a surrogate pair.
func utf16ReplaceRange(s string, offset, count int, replacement string) string {
	codeUnits := stringToUTF16(s)
	if offset > len(codeUnits) {
		offset = len(codeUnits)
	}
	end := offset + count
	if end > len(codeUnits) {
		end = len(codeUnits)
	}

	// Build result: before + replacement + after
	before := codeUnits[:offset]
	after := codeUnits[end:]
	replacementUnits := stringToUTF16(replacement)

	result := make([]uint16, 0, len(before)+len(replacementUnits)+len(after))
	result = append(result, before...)
	result = append(result, replacementUnits...)
	result = append(result, after...)

	return utf16ToString(result)
}

// asciiLowercase converts a string to lowercase using only ASCII rules.
// Unicode characters like Turkish I (U+0130) are not converted.
func asciiLowercase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// querySelectorWithContext finds the first descendant element matching a selector with context.
// This is used for scoped queries where :scope should match the element querySelector was called on.
func querySelectorWithContext(root *dom.Node, selector *css.CSSSelector, ctx *css.MatchContext, firstOnly bool) *dom.Element {
	// Traverse descendants
	for child := root.FirstChild(); child != nil; child = child.NextSibling() {
		if child.NodeType() == dom.ElementNode {
			el := (*dom.Element)(child)
			if selector.MatchElementWithContext(el, ctx) {
				return el
			}
			if result := querySelectorWithContext(child, selector, ctx, firstOnly); result != nil {
				return result
			}
		}
	}
	return nil
}

// querySelectorAllWithContext finds all descendant elements matching a selector with context.
func querySelectorAllWithContext(root *dom.Node, selector *css.CSSSelector, ctx *css.MatchContext) []*dom.Element {
	var results []*dom.Element
	querySelectorAllWithContextHelper(root, selector, ctx, &results)
	return results
}

func querySelectorAllWithContextHelper(root *dom.Node, selector *css.CSSSelector, ctx *css.MatchContext, results *[]*dom.Element) {
	for child := root.FirstChild(); child != nil; child = child.NextSibling() {
		if child.NodeType() == dom.ElementNode {
			el := (*dom.Element)(child)
			if selector.MatchElementWithContext(el, ctx) {
				*results = append(*results, el)
			}
			querySelectorAllWithContextHelper(child, selector, ctx, results)
		}
	}
}

// domExceptionCode returns the legacy exception code for a DOMException name.
func domExceptionCode(name string) int {
	codes := map[string]int{
		"IndexSizeError":             1,
		"HierarchyRequestError":      3,
		"WrongDocumentError":         4,
		"InvalidCharacterError":      5,
		"NoModificationAllowedError": 7,
		"NotFoundError":              8,
		"NotSupportedError":          9,
		"InUseAttributeError":        10,
		"InvalidStateError":          11,
		"SyntaxError":                12,
		"InvalidModificationError":   13,
		"NamespaceError":             14,
		"InvalidAccessError":         15,
		"TypeMismatchError":          17,
		"SecurityError":              18,
		"NetworkError":               19,
		"AbortError":                 20,
		"URLMismatchError":           21,
		"QuotaExceededError":         22,
		"TimeoutError":               23,
		"InvalidNodeTypeError":       24,
		"DataCloneError":             25,
	}
	if code, ok := codes[name]; ok {
		return code
	}
	return 0
}

// toUint32 converts a JavaScript value to an unsigned 32-bit integer per Web IDL.
// This handles overflow/underflow for CharacterData methods.
func toUint32(v goja.Value) uint32 {
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return 0
	}
	num := v.ToFloat()
	// Handle NaN
	if num != num {
		return 0
	}
	// Handle infinity
	if num == 0 || num > 4294967295 || num < -4294967295 {
		return uint32(int64(num) & 0xFFFFFFFF)
	}
	// Truncate to integer
	intVal := int64(num)
	// Apply modulo 2^32
	return uint32(intVal & 0xFFFFFFFF)
}

// DOMBinder provides methods to bind DOM objects to JavaScript.
// IframeContentProvider is a callback interface for providing iframe content.
// This allows the DOMBinder to get iframe contentWindow/contentDocument without
// directly depending on ScriptExecutor.
type IframeContentProvider func(iframe *dom.Element) (contentWindow goja.Value, contentDocument goja.Value)

type DOMBinder struct {
	runtime               *Runtime
	eventBinder           *EventBinder                // Event binder for adding EventTarget methods
	iframeContentProvider IframeContentProvider       // Callback for getting iframe content
	nodeMap               map[*dom.Node]*goja.Object  // Cache to return same JS object for same DOM node
	document              *dom.Document               // Current document for creating new nodes
	globalDocumentSet     bool                        // Track if global document has been set
	mainDocument          *dom.Document               // Main document associated with window (for event bubbling)

	// Style resolver for getComputedStyle
	styleResolver *css.StyleResolver

	// Prototype objects for instanceof checks
	nodeProto                    *goja.Object
	characterDataProto           *goja.Object
	textProto                    *goja.Object
	commentProto                 *goja.Object
	cdataSectionProto            *goja.Object
	processingInstructionProto   *goja.Object
	elementProto                 *goja.Object
	htmlElementProto             *goja.Object
	htmlElementProtoMap          map[string]*goja.Object // Maps tag name to specific prototype
	documentProto                *goja.Object
	documentTypeProto            *goja.Object
	xmlDocumentProto             *goja.Object
	documentFragmentProto        *goja.Object
	domExceptionProto            *goja.Object
	domImplementationProto       *goja.Object
	htmlCollectionProto          *goja.Object
	nodeListProto                *goja.Object
	namedNodeMapProto            *goja.Object
	attrProto                    *goja.Object
	cssStyleDeclarationProto     *goja.Object
	domImplementationCache       map[*dom.DOMImplementation]*goja.Object
	styleDeclarationCache        map[*dom.CSSStyleDeclaration]*goja.Object
	htmlCollectionMap            map[*goja.Object]*dom.HTMLCollection
	namedNodeMapMap              map[*goja.Object]*dom.NamedNodeMap
	nodeListCache                map[*dom.NodeList]*goja.Object
	domTokenListCache            map[*dom.DOMTokenList]*goja.Object
	attrCache                    map[*dom.Attr]*goja.Object
	rangeProto                   *goja.Object
	rangeCache                   map[*dom.Range]*goja.Object
	treeWalkerProto              *goja.Object
	nodeIteratorProto            *goja.Object
	shadowRootProto              *goja.Object
	shadowRootCache              map[*dom.ShadowRoot]*goja.Object
	selectionProto               *goja.Object
	selectionCache               map[*dom.Selection]*goja.Object

	// Event handler storage: maps JS object -> event type -> handler function
	eventHandlers                map[*goja.Object]map[string]goja.Value
}

// NewDOMBinder creates a new DOM binder for the given runtime.
func NewDOMBinder(runtime *Runtime) *DOMBinder {
	b := &DOMBinder{
		runtime:                runtime,
		nodeMap:                make(map[*dom.Node]*goja.Object),
		domImplementationCache: make(map[*dom.DOMImplementation]*goja.Object),
		styleDeclarationCache:  make(map[*dom.CSSStyleDeclaration]*goja.Object),
		htmlCollectionMap:      make(map[*goja.Object]*dom.HTMLCollection),
		namedNodeMapMap:        make(map[*goja.Object]*dom.NamedNodeMap),
		htmlElementProtoMap:    make(map[string]*goja.Object),
		nodeListCache:          make(map[*dom.NodeList]*goja.Object),
		domTokenListCache:      make(map[*dom.DOMTokenList]*goja.Object),
		attrCache:              make(map[*dom.Attr]*goja.Object),
		rangeCache:             make(map[*dom.Range]*goja.Object),
		shadowRootCache:        make(map[*dom.ShadowRoot]*goja.Object),
		selectionCache:         make(map[*dom.Selection]*goja.Object),
		eventHandlers:          make(map[*goja.Object]map[string]goja.Value),
	}
	b.setupPrototypes()
	return b
}

// setupPrototypes creates the prototype chain for DOM interfaces.
// This enables instanceof checks to work correctly.
func (b *DOMBinder) setupPrototypes() {
	vm := b.runtime.vm

	// Create Node prototype and constructor
	b.nodeProto = vm.NewObject()
	nodeConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Node is abstract - cannot be instantiated directly
		panic(vm.NewTypeError("Illegal constructor"))
	})
	nodeConstructorObj := nodeConstructor.ToObject(vm)
	nodeConstructorObj.Set("prototype", b.nodeProto)
	b.nodeProto.Set("constructor", nodeConstructorObj)

	// Set Node constants on the constructor AND prototype
	// Both locations are needed per the DOM spec for constants to be available
	// on the interface object, the prototype, and all instances
	nodeTypeConstants := map[string]int{
		"ELEMENT_NODE":                1,
		"ATTRIBUTE_NODE":              2,
		"TEXT_NODE":                   3,
		"CDATA_SECTION_NODE":          4,
		"ENTITY_REFERENCE_NODE":       5,
		"ENTITY_NODE":                 6,
		"PROCESSING_INSTRUCTION_NODE": 7,
		"COMMENT_NODE":                8,
		"DOCUMENT_NODE":               9,
		"DOCUMENT_TYPE_NODE":          10,
		"DOCUMENT_FRAGMENT_NODE":      11,
		"NOTATION_NODE":               12,
	}
	documentPositionConstants := map[string]int{
		"DOCUMENT_POSITION_DISCONNECTED":            0x01,
		"DOCUMENT_POSITION_PRECEDING":               0x02,
		"DOCUMENT_POSITION_FOLLOWING":               0x04,
		"DOCUMENT_POSITION_CONTAINS":                0x08,
		"DOCUMENT_POSITION_CONTAINED_BY":            0x10,
		"DOCUMENT_POSITION_IMPLEMENTATION_SPECIFIC": 0x20,
	}
	for name, value := range nodeTypeConstants {
		nodeConstructorObj.Set(name, value)
		b.nodeProto.Set(name, value)
	}
	for name, value := range documentPositionConstants {
		nodeConstructorObj.Set(name, value)
		b.nodeProto.Set(name, value)
	}

	// Add Node methods to prototype so Node.prototype.insertBefore etc. work
	// These methods get the node from 'this'
	b.setupNodePrototypeMethods()

	vm.Set("Node", nodeConstructorObj)

	// Create DOMException prototype and constructor
	b.domExceptionProto = vm.NewObject()
	// DOMException extends Error prototype
	errorProto := vm.Get("Error").ToObject(vm).Get("prototype").ToObject(vm)
	b.domExceptionProto.SetPrototype(errorProto)

	domExceptionConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		message := ""
		name := "Error"
		if len(call.Arguments) > 0 {
			message = call.Arguments[0].String()
		}
		if len(call.Arguments) > 1 {
			name = call.Arguments[1].String()
		}
		exc := call.This
		exc.Set("message", message)
		exc.Set("name", name)
		exc.Set("code", domExceptionCode(name))
		return exc
	})
	domExceptionConstructorObj := domExceptionConstructor.ToObject(vm)
	// Set the 'name' property to 'DOMException' so that typeof checks and
	// tests like assert_throws_dom can correctly identify it
	domExceptionConstructorObj.DefineDataProperty("name", vm.ToValue("DOMException"), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	domExceptionConstructorObj.Set("prototype", b.domExceptionProto)
	b.domExceptionProto.Set("constructor", domExceptionConstructorObj)

	// Set DOMException constants
	domExceptionConstructorObj.Set("INDEX_SIZE_ERR", 1)
	domExceptionConstructorObj.Set("DOMSTRING_SIZE_ERR", 2)
	domExceptionConstructorObj.Set("HIERARCHY_REQUEST_ERR", 3)
	domExceptionConstructorObj.Set("WRONG_DOCUMENT_ERR", 4)
	domExceptionConstructorObj.Set("INVALID_CHARACTER_ERR", 5)
	domExceptionConstructorObj.Set("NO_DATA_ALLOWED_ERR", 6)
	domExceptionConstructorObj.Set("NO_MODIFICATION_ALLOWED_ERR", 7)
	domExceptionConstructorObj.Set("NOT_FOUND_ERR", 8)
	domExceptionConstructorObj.Set("NOT_SUPPORTED_ERR", 9)
	domExceptionConstructorObj.Set("INUSE_ATTRIBUTE_ERR", 10)
	domExceptionConstructorObj.Set("INVALID_STATE_ERR", 11)
	domExceptionConstructorObj.Set("SYNTAX_ERR", 12)
	domExceptionConstructorObj.Set("INVALID_MODIFICATION_ERR", 13)
	domExceptionConstructorObj.Set("NAMESPACE_ERR", 14)
	domExceptionConstructorObj.Set("INVALID_ACCESS_ERR", 15)
	domExceptionConstructorObj.Set("VALIDATION_ERR", 16)
	domExceptionConstructorObj.Set("TYPE_MISMATCH_ERR", 17)
	domExceptionConstructorObj.Set("SECURITY_ERR", 18)
	domExceptionConstructorObj.Set("NETWORK_ERR", 19)
	domExceptionConstructorObj.Set("ABORT_ERR", 20)
	domExceptionConstructorObj.Set("URL_MISMATCH_ERR", 21)
	domExceptionConstructorObj.Set("QUOTA_EXCEEDED_ERR", 22)
	domExceptionConstructorObj.Set("TIMEOUT_ERR", 23)
	domExceptionConstructorObj.Set("INVALID_NODE_TYPE_ERR", 24)
	domExceptionConstructorObj.Set("DATA_CLONE_ERR", 25)

	vm.Set("DOMException", domExceptionConstructorObj)

	// Create CharacterData prototype (extends Node)
	b.characterDataProto = vm.NewObject()
	b.characterDataProto.SetPrototype(b.nodeProto)
	charDataConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	charDataConstructorObj := charDataConstructor.ToObject(vm)
	charDataConstructorObj.Set("prototype", b.characterDataProto)
	b.characterDataProto.Set("constructor", charDataConstructorObj)
	vm.Set("CharacterData", charDataConstructorObj)

	// Create Text prototype (extends CharacterData)
	b.textProto = vm.NewObject()
	b.textProto.SetPrototype(b.characterDataProto)
	textConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Text can be constructed with new Text() or new Text(data)
		// Per spec, undefined is treated as missing (empty string)
		data := ""
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
			data = call.Arguments[0].String()
		}
		// Create text node using document if available, otherwise detached
		var textNode *dom.Node
		if b.document != nil {
			textNode = b.document.CreateTextNode(data)
		} else {
			textNode = dom.NewTextNode(data)
		}
		return b.BindTextNode(textNode, call.This.Prototype())
	})
	textConstructorObj := textConstructor.ToObject(vm)
	textConstructorObj.Set("prototype", b.textProto)
	b.textProto.Set("constructor", textConstructorObj)
	vm.Set("Text", textConstructorObj)

	// Create Comment prototype (extends CharacterData)
	b.commentProto = vm.NewObject()
	b.commentProto.SetPrototype(b.characterDataProto)
	commentConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Comment can be constructed with new Comment() or new Comment(data)
		// Per spec, undefined is treated as missing (empty string)
		data := ""
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
			data = call.Arguments[0].String()
		}
		// Create comment node using document if available, otherwise detached
		var commentNode *dom.Node
		if b.document != nil {
			commentNode = b.document.CreateComment(data)
		} else {
			commentNode = dom.NewCommentNode(data)
		}
		return b.BindCommentNode(commentNode, call.This.Prototype())
	})
	commentConstructorObj := commentConstructor.ToObject(vm)
	commentConstructorObj.Set("prototype", b.commentProto)
	b.commentProto.Set("constructor", commentConstructorObj)
	vm.Set("Comment", commentConstructorObj)

	// Create CDATASection prototype (extends Text per DOM spec)
	b.cdataSectionProto = vm.NewObject()
	b.cdataSectionProto.SetPrototype(b.textProto)
	cdataSectionConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// CDATASection cannot be constructed directly - must use createCDATASection
		panic(vm.NewTypeError("Illegal constructor"))
	})
	cdataSectionConstructorObj := cdataSectionConstructor.ToObject(vm)
	cdataSectionConstructorObj.Set("prototype", b.cdataSectionProto)
	b.cdataSectionProto.Set("constructor", cdataSectionConstructorObj)
	vm.Set("CDATASection", cdataSectionConstructorObj)

	// Create ProcessingInstruction prototype (extends CharacterData)
	b.processingInstructionProto = vm.NewObject()
	b.processingInstructionProto.SetPrototype(b.characterDataProto)
	piConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// ProcessingInstruction cannot be constructed directly - must use createProcessingInstruction
		panic(vm.NewTypeError("Illegal constructor"))
	})
	piConstructorObj := piConstructor.ToObject(vm)
	piConstructorObj.Set("prototype", b.processingInstructionProto)
	b.processingInstructionProto.Set("constructor", piConstructorObj)
	vm.Set("ProcessingInstruction", piConstructorObj)

	// Create Element prototype (extends Node)
	b.elementProto = vm.NewObject()
	b.elementProto.SetPrototype(b.nodeProto)
	elementConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	elementConstructorObj := elementConstructor.ToObject(vm)
	elementConstructorObj.Set("prototype", b.elementProto)
	b.elementProto.Set("constructor", elementConstructorObj)
	vm.Set("Element", elementConstructorObj)

	// Set Symbol.unscopables for Element prototype per DOM spec
	// https://dom.spec.whatwg.org/#interface-element
	// These methods should not shadow window properties when used in 'with' statements
	_, _ = vm.RunString(`
		Element.prototype[Symbol.unscopables] = {
			slot: true,
			before: true,
			after: true,
			replaceWith: true,
			remove: true,
			prepend: true,
			append: true
		};
	`)

	// Create HTMLElement prototype (extends Element)
	b.htmlElementProto = vm.NewObject()
	b.htmlElementProto.SetPrototype(b.elementProto)
	htmlElementConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	htmlElementConstructorObj := htmlElementConstructor.ToObject(vm)
	htmlElementConstructorObj.Set("prototype", b.htmlElementProto)
	b.htmlElementProto.Set("constructor", htmlElementConstructorObj)
	vm.Set("HTMLElement", htmlElementConstructorObj)

	// Set up specific HTML element type constructors
	// Each extends HTMLElement and maps to specific tag names
	b.setupHTMLElementPrototypes()

	// Set up HTMLElement prototype methods (click, focus, blur, etc.)
	b.setupHTMLElementPrototypeMethods()

	// Create Document prototype (extends Node)
	b.documentProto = vm.NewObject()
	b.documentProto.SetPrototype(b.nodeProto)
	documentConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Per DOM spec, new Document() creates a document with contentType "application/xml"
		doc := dom.NewXMLDocument()
		return b.bindDocumentInternal(doc)
	})
	documentConstructorObj := documentConstructor.ToObject(vm)
	documentConstructorObj.Set("prototype", b.documentProto)
	b.documentProto.Set("constructor", documentConstructorObj)
	vm.Set("Document", documentConstructorObj)

	// Create XMLDocument prototype (extends Document)
	b.xmlDocumentProto = vm.NewObject()
	b.xmlDocumentProto.SetPrototype(b.documentProto)
	xmlDocumentConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// XMLDocument cannot be constructed directly via new
		panic(vm.NewTypeError("Illegal constructor"))
	})
	xmlDocumentConstructorObj := xmlDocumentConstructor.ToObject(vm)
	xmlDocumentConstructorObj.Set("prototype", b.xmlDocumentProto)
	b.xmlDocumentProto.Set("constructor", xmlDocumentConstructorObj)
	vm.Set("XMLDocument", xmlDocumentConstructorObj)

	// Create DocumentType prototype (extends Node)
	b.documentTypeProto = vm.NewObject()
	b.documentTypeProto.SetPrototype(b.nodeProto)
	documentTypeConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// DocumentType cannot be constructed directly via new
		panic(vm.NewTypeError("Illegal constructor"))
	})
	documentTypeConstructorObj := documentTypeConstructor.ToObject(vm)
	documentTypeConstructorObj.Set("prototype", b.documentTypeProto)
	b.documentTypeProto.Set("constructor", documentTypeConstructorObj)
	vm.Set("DocumentType", documentTypeConstructorObj)

	// Create DocumentFragment prototype (extends Node)
	b.documentFragmentProto = vm.NewObject()
	b.documentFragmentProto.SetPrototype(b.nodeProto)
	docFragConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// DocumentFragment can be constructed
		// Per spec, the ownerDocument is the associated document (current document)
		frag := b.document.CreateDocumentFragment()
		return b.BindDocumentFragment(frag)
	})
	docFragConstructorObj := docFragConstructor.ToObject(vm)
	docFragConstructorObj.Set("prototype", b.documentFragmentProto)
	b.documentFragmentProto.Set("constructor", docFragConstructorObj)
	vm.Set("DocumentFragment", docFragConstructorObj)

	// Add DocumentFragment prototype methods
	b.setupDocumentFragmentPrototypeMethods()

	// Create ShadowRoot prototype (extends DocumentFragment)
	b.shadowRootProto = vm.NewObject()
	b.shadowRootProto.SetPrototype(b.documentFragmentProto)
	shadowRootConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// ShadowRoot cannot be constructed directly - must use attachShadow
		panic(vm.NewTypeError("Illegal constructor"))
	})
	shadowRootConstructorObj := shadowRootConstructor.ToObject(vm)
	shadowRootConstructorObj.Set("prototype", b.shadowRootProto)
	b.shadowRootProto.Set("constructor", shadowRootConstructorObj)
	vm.Set("ShadowRoot", shadowRootConstructorObj)

	// Add ShadowRoot prototype methods
	b.setupShadowRootPrototypeMethods()

	// Create DOMImplementation prototype
	b.domImplementationProto = vm.NewObject()
	domImplConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	domImplConstructorObj := domImplConstructor.ToObject(vm)
	domImplConstructorObj.Set("prototype", b.domImplementationProto)
	b.domImplementationProto.Set("constructor", domImplConstructorObj)
	vm.Set("DOMImplementation", domImplConstructorObj)

	// Create Range prototype
	b.rangeProto = vm.NewObject()
	rangeConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Range can be constructed with new Range()
		// Per spec, it creates a Range whose start and end are set to (current document, 0)
		if b.document != nil {
			r := dom.NewRange(b.document)
			return b.BindRange(r)
		}
		// If no document, return empty object with prototype
		return call.This
	})
	rangeConstructorObj := rangeConstructor.ToObject(vm)
	rangeConstructorObj.Set("prototype", b.rangeProto)
	b.rangeProto.Set("constructor", rangeConstructorObj)

	// Range constants
	rangeConstructorObj.Set("START_TO_START", dom.StartToStart)
	rangeConstructorObj.Set("START_TO_END", dom.StartToEnd)
	rangeConstructorObj.Set("END_TO_END", dom.EndToEnd)
	rangeConstructorObj.Set("END_TO_START", dom.EndToStart)
	b.rangeProto.Set("START_TO_START", dom.StartToStart)
	b.rangeProto.Set("START_TO_END", dom.StartToEnd)
	b.rangeProto.Set("END_TO_END", dom.EndToEnd)
	b.rangeProto.Set("END_TO_START", dom.EndToStart)

	vm.Set("Range", rangeConstructorObj)

	// Create Selection prototype
	b.selectionProto = vm.NewObject()
	selectionConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Selection cannot be constructed directly - must use getSelection()
		panic(vm.NewTypeError("Illegal constructor"))
	})
	selectionConstructorObj := selectionConstructor.ToObject(vm)
	selectionConstructorObj.Set("prototype", b.selectionProto)
	b.selectionProto.Set("constructor", selectionConstructorObj)
	vm.Set("Selection", selectionConstructorObj)

	// Create DOMRect prototype and constructor
	domRectProto := vm.NewObject()
	domRectConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		x, y, width, height := 0.0, 0.0, 0.0, 0.0
		if len(call.Arguments) > 0 {
			x = call.Arguments[0].ToFloat()
		}
		if len(call.Arguments) > 1 {
			y = call.Arguments[1].ToFloat()
		}
		if len(call.Arguments) > 2 {
			width = call.Arguments[2].ToFloat()
		}
		if len(call.Arguments) > 3 {
			height = call.Arguments[3].ToFloat()
		}
		rect := dom.NewDOMRect(x, y, width, height)
		call.This.Set("_goRect", rect)

		// Define properties on the instance
		call.This.DefineAccessorProperty("x", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.X)
		}), vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			if len(fc.Arguments) > 0 {
				rect.X = fc.Arguments[0].ToFloat()
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("y", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Y)
		}), vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			if len(fc.Arguments) > 0 {
				rect.Y = fc.Arguments[0].ToFloat()
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("width", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Width)
		}), vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			if len(fc.Arguments) > 0 {
				rect.Width = fc.Arguments[0].ToFloat()
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("height", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Height)
		}), vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			if len(fc.Arguments) > 0 {
				rect.Height = fc.Arguments[0].ToFloat()
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("top", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Top())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("right", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Right())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("bottom", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Bottom())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("left", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Left())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		return call.This
	})
	domRectConstructorObj := domRectConstructor.ToObject(vm)
	domRectConstructorObj.Set("prototype", domRectProto)
	domRectProto.Set("constructor", domRectConstructorObj)

	// Static method: DOMRect.fromRect()
	domRectConstructorObj.Set("fromRect", func(call goja.FunctionCall) goja.Value {
		x, y, width, height := 0.0, 0.0, 0.0, 0.0
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			obj := call.Arguments[0].ToObject(vm)
			if xVal := obj.Get("x"); xVal != nil && !goja.IsUndefined(xVal) {
				x = xVal.ToFloat()
			}
			if yVal := obj.Get("y"); yVal != nil && !goja.IsUndefined(yVal) {
				y = yVal.ToFloat()
			}
			if wVal := obj.Get("width"); wVal != nil && !goja.IsUndefined(wVal) {
				width = wVal.ToFloat()
			}
			if hVal := obj.Get("height"); hVal != nil && !goja.IsUndefined(hVal) {
				height = hVal.ToFloat()
			}
		}
		rect := dom.NewDOMRect(x, y, width, height)
		return b.BindDOMRect(rect)
	})

	vm.Set("DOMRect", domRectConstructorObj)

	// Create DOMRectReadOnly - same as DOMRect but read-only properties
	domRectReadOnlyProto := vm.NewObject()
	domRectReadOnlyConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		x, y, width, height := 0.0, 0.0, 0.0, 0.0
		if len(call.Arguments) > 0 {
			x = call.Arguments[0].ToFloat()
		}
		if len(call.Arguments) > 1 {
			y = call.Arguments[1].ToFloat()
		}
		if len(call.Arguments) > 2 {
			width = call.Arguments[2].ToFloat()
		}
		if len(call.Arguments) > 3 {
			height = call.Arguments[3].ToFloat()
		}
		rect := dom.NewDOMRect(x, y, width, height)
		call.This.Set("_goRect", rect)

		// All properties are read-only for DOMRectReadOnly
		call.This.DefineAccessorProperty("x", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.X)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("y", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Y)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("width", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Width)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("height", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Height)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("top", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Top())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("right", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Right())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("bottom", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Bottom())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		call.This.DefineAccessorProperty("left", vm.ToValue(func(fc goja.FunctionCall) goja.Value {
			return vm.ToValue(rect.Left())
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

		return call.This
	})
	domRectReadOnlyConstructorObj := domRectReadOnlyConstructor.ToObject(vm)
	domRectReadOnlyConstructorObj.Set("prototype", domRectReadOnlyProto)
	domRectReadOnlyProto.Set("constructor", domRectReadOnlyConstructorObj)

	// Static method: DOMRectReadOnly.fromRect()
	domRectReadOnlyConstructorObj.Set("fromRect", func(call goja.FunctionCall) goja.Value {
		x, y, width, height := 0.0, 0.0, 0.0, 0.0
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			obj := call.Arguments[0].ToObject(vm)
			if xVal := obj.Get("x"); xVal != nil && !goja.IsUndefined(xVal) {
				x = xVal.ToFloat()
			}
			if yVal := obj.Get("y"); yVal != nil && !goja.IsUndefined(yVal) {
				y = yVal.ToFloat()
			}
			if wVal := obj.Get("width"); wVal != nil && !goja.IsUndefined(wVal) {
				width = wVal.ToFloat()
			}
			if hVal := obj.Get("height"); hVal != nil && !goja.IsUndefined(hVal) {
				height = hVal.ToFloat()
			}
		}
		rect := dom.NewDOMRect(x, y, width, height)
		return b.BindDOMRect(rect)
	})

	vm.Set("DOMRectReadOnly", domRectReadOnlyConstructorObj)

	// Create HTMLCollection prototype with item and namedItem methods
	b.htmlCollectionProto = vm.NewObject()
	htmlCollectionConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	htmlCollectionConstructorObj := htmlCollectionConstructor.ToObject(vm)
	htmlCollectionConstructorObj.Set("prototype", b.htmlCollectionProto)
	b.htmlCollectionProto.Set("constructor", htmlCollectionConstructorObj)

	// Helper to get collection from this object
	getCollection := func(thisObj *goja.Object) *dom.HTMLCollection {
		if thisObj == nil {
			return nil
		}
		return b.htmlCollectionMap[thisObj]
	}

	// Add length getter to prototype - it reads from the internal map
	b.htmlCollectionProto.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		col := getCollection(call.This.ToObject(vm))
		if col == nil {
			return vm.ToValue(0)
		}
		return vm.ToValue(col.Length())
	}), nil, goja.FLAG_TRUE, goja.FLAG_FALSE)

	// Add item method to prototype
	// Per the DOM spec, item() takes an "unsigned long" index which uses ToUint32 conversion.
	// This means values >= 2^32 wrap around (e.g., 4294967296 becomes 0).
	b.htmlCollectionProto.Set("item", func(call goja.FunctionCall) goja.Value {
		col := getCollection(call.This.ToObject(vm))
		if col == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		// ToUint32 conversion: convert to int64, then mask to 32 bits
		intVal := call.Arguments[0].ToInteger()
		index := int(uint32(intVal))
		el := col.Item(index)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// Add namedItem method to prototype
	b.htmlCollectionProto.Set("namedItem", func(call goja.FunctionCall) goja.Value {
		col := getCollection(call.This.ToObject(vm))
		if col == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		el := col.NamedItem(name)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// Add Symbol.iterator to make HTMLCollection iterable (uses Array.prototype's iterator)
	arrayProto := vm.Get("Array").ToObject(vm).Get("prototype").ToObject(vm)
	symbolSetup, _ := vm.RunString(`
		(function(target, arrayProto) {
			target[Symbol.iterator] = arrayProto[Symbol.iterator];
		})
	`)
	if symbolSetup != nil {
		fn, _ := goja.AssertFunction(symbolSetup)
		if fn != nil {
			fn(goja.Undefined(), b.htmlCollectionProto, arrayProto)
		}
	}

	vm.Set("HTMLCollection", htmlCollectionConstructorObj)

	// Set Symbol.toStringTag for HTMLCollection to ensure proper class string
	_, _ = vm.RunString(`
		Object.defineProperty(HTMLCollection.prototype, Symbol.toStringTag, {
			value: "HTMLCollection",
			writable: false,
			enumerable: false,
			configurable: true
		});
	`)

	// Create NodeList prototype
	b.nodeListProto = vm.NewObject()
	nodeListConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	nodeListConstructorObj := nodeListConstructor.ToObject(vm)
	nodeListConstructorObj.Set("prototype", b.nodeListProto)
	b.nodeListProto.Set("constructor", nodeListConstructorObj)
	vm.Set("NodeList", nodeListConstructorObj)

	// Set Symbol.toStringTag for NodeList to ensure proper class string
	_, _ = vm.RunString(`
		Object.defineProperty(NodeList.prototype, Symbol.toStringTag, {
			value: "NodeList",
			writable: false,
			enumerable: false,
			configurable: true
		});
	`)

	// Create NamedNodeMap prototype with methods
	b.namedNodeMapProto = vm.NewObject()
	namedNodeMapConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	namedNodeMapConstructorObj := namedNodeMapConstructor.ToObject(vm)
	namedNodeMapConstructorObj.Set("prototype", b.namedNodeMapProto)
	b.namedNodeMapProto.Set("constructor", namedNodeMapConstructorObj)

	// Helper to get NamedNodeMap from this object
	getNamedNodeMap := func(thisObj *goja.Object) *dom.NamedNodeMap {
		if thisObj == nil {
			return nil
		}
		return b.namedNodeMapMap[thisObj]
	}

	// Add length getter to prototype
	b.namedNodeMapProto.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return vm.ToValue(0)
		}
		return vm.ToValue(nnm.Length())
	}), nil, goja.FLAG_TRUE, goja.FLAG_FALSE)

	// Add item method to prototype
	b.namedNodeMapProto.Set("item", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		attr := nnm.Item(index)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// Add getNamedItem method to prototype
	b.namedNodeMapProto.Set("getNamedItem", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		attr := nnm.GetNamedItem(name)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// Add getNamedItemNS method to prototype
	b.namedNodeMapProto.Set("getNamedItemNS", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		attr := nnm.GetNamedItemNS(ns, localName)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// Add setNamedItem method to prototype
	b.namedNodeMapProto.Set("setNamedItem", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		jsAttr := call.Arguments[0].ToObject(vm)
		goAttr := b.extractAttr(jsAttr)
		if goAttr == nil {
			return goja.Null()
		}
		oldAttr := nnm.SetAttr(goAttr)
		if oldAttr == nil {
			return goja.Null()
		}
		return b.BindAttr(oldAttr)
	})

	// Add setNamedItemNS method to prototype
	b.namedNodeMapProto.Set("setNamedItemNS", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		jsAttr := call.Arguments[0].ToObject(vm)
		goAttr := b.extractAttr(jsAttr)
		if goAttr == nil {
			return goja.Null()
		}
		oldAttr := nnm.SetAttr(goAttr)
		if oldAttr == nil {
			return goja.Null()
		}
		return b.BindAttr(oldAttr)
	})

	// Add removeNamedItem method to prototype
	b.namedNodeMapProto.Set("removeNamedItem", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		attr := nnm.RemoveNamedItem(name)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// Add removeNamedItemNS method to prototype
	b.namedNodeMapProto.Set("removeNamedItemNS", func(call goja.FunctionCall) goja.Value {
		nnm := getNamedNodeMap(call.This.ToObject(vm))
		if nnm == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		attr := nnm.RemoveNamedItemNS(ns, localName)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	vm.Set("NamedNodeMap", namedNodeMapConstructorObj)

	// Create Attr prototype and constructor
	// Attr inherits from Node in the DOM, but we'll set it up as a simple prototype
	b.attrProto = vm.NewObject()
	attrConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	attrConstructorObj := attrConstructor.ToObject(vm)
	attrConstructorObj.Set("prototype", b.attrProto)
	b.attrProto.Set("constructor", attrConstructorObj)

	// Add Attr properties to prototype (read-only getters)
	// The actual values are set per-instance in BindAttr
	vm.Set("Attr", attrConstructorObj)

	// Create NodeFilter interface object with constants
	// NodeFilter is not constructable but has constants for filtering
	nodeFilterObj := vm.NewObject()

	// NodeFilter acceptNode return values
	nodeFilterObj.Set("FILTER_ACCEPT", 1)
	nodeFilterObj.Set("FILTER_REJECT", 2)
	nodeFilterObj.Set("FILTER_SKIP", 3)

	// NodeFilter whatToShow constants
	nodeFilterObj.Set("SHOW_ALL", uint32(0xFFFFFFFF))
	nodeFilterObj.Set("SHOW_ELEMENT", uint32(0x1))
	nodeFilterObj.Set("SHOW_ATTRIBUTE", uint32(0x2))
	nodeFilterObj.Set("SHOW_TEXT", uint32(0x4))
	nodeFilterObj.Set("SHOW_CDATA_SECTION", uint32(0x8))
	nodeFilterObj.Set("SHOW_ENTITY_REFERENCE", uint32(0x10))
	nodeFilterObj.Set("SHOW_ENTITY", uint32(0x20))
	nodeFilterObj.Set("SHOW_PROCESSING_INSTRUCTION", uint32(0x40))
	nodeFilterObj.Set("SHOW_COMMENT", uint32(0x80))
	nodeFilterObj.Set("SHOW_DOCUMENT", uint32(0x100))
	nodeFilterObj.Set("SHOW_DOCUMENT_TYPE", uint32(0x200))
	nodeFilterObj.Set("SHOW_DOCUMENT_FRAGMENT", uint32(0x400))
	nodeFilterObj.Set("SHOW_NOTATION", uint32(0x800))

	vm.Set("NodeFilter", nodeFilterObj)

	// Create TreeWalker prototype
	// TreeWalker is not directly constructable - it's created via document.createTreeWalker
	b.treeWalkerProto = vm.NewObject()
	treeWalkerConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	treeWalkerConstructorObj := treeWalkerConstructor.ToObject(vm)
	treeWalkerConstructorObj.Set("prototype", b.treeWalkerProto)
	b.treeWalkerProto.Set("constructor", treeWalkerConstructorObj)

	vm.Set("TreeWalker", treeWalkerConstructorObj)

	// Create NodeIterator prototype
	// NodeIterator is not directly constructable - it's created via document.createNodeIterator
	b.nodeIteratorProto = vm.NewObject()
	nodeIteratorConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	nodeIteratorConstructorObj := nodeIteratorConstructor.ToObject(vm)
	nodeIteratorConstructorObj.Set("prototype", b.nodeIteratorProto)
	b.nodeIteratorProto.Set("constructor", nodeIteratorConstructorObj)

	vm.Set("NodeIterator", nodeIteratorConstructorObj)

	// Create DOMParser prototype and constructor
	domParserProto := vm.NewObject()
	domParserConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// DOMParser can be constructed with new DOMParser()
		parser := call.This
		parser.SetPrototype(domParserProto)

		// parseFromString method
		parser.Set("parseFromString", func(fc goja.FunctionCall) goja.Value {
			if len(fc.Arguments) < 2 {
				// Per spec, parseFromString requires both arguments
				panic(vm.NewTypeError("Failed to execute 'parseFromString' on 'DOMParser': 2 arguments required"))
			}

			str := fc.Arguments[0].String()
			mimeType := fc.Arguments[1].String()

			switch mimeType {
			case "text/html":
				// Parse as HTML document
				doc, err := dom.ParseHTML(str)
				if err != nil {
					// For HTML, parsing errors don't throw - the parser is lenient
					// Return an empty document
					doc = dom.NewDocument()
				}
				return b.bindDocumentInternal(doc)

			case "text/xml", "application/xml", "application/xhtml+xml", "image/svg+xml":
				// For XML-based types, we should parse as XML
				// For now, we'll parse as HTML which handles most cases
				// TODO: Implement proper XML parsing
				doc, err := dom.ParseHTML(str)
				if err != nil {
					// For XML, create a document with a parsererror element
					doc = dom.NewDocument()
					html := doc.CreateElement("html")
					body := doc.CreateElement("body")
					parsererror := doc.CreateElement("parsererror")
					parsererror.SetTextContent("XML parsing error: " + err.Error())
					body.AsNode().AppendChild(parsererror.AsNode())
					html.AsNode().AppendChild(body.AsNode())
					doc.AsNode().AppendChild(html.AsNode())
				}
				// Set the content type to indicate this is an XML document
				// This is needed so that createCDATASection and other XML-only features work
				doc.SetContentType(mimeType)
				// For XML types, return XMLDocument
				return b.bindXMLDocument(doc)

			default:
				// Invalid MIME type - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'parseFromString' on 'DOMParser': The provided value '" + mimeType + "' is not a valid MIME type."))
			}
		})

		return parser
	})
	domParserConstructorObj := domParserConstructor.ToObject(vm)
	domParserConstructorObj.Set("prototype", domParserProto)
	domParserProto.Set("constructor", domParserConstructorObj)

	vm.Set("DOMParser", domParserConstructorObj)
}

// htmlElementTypeMap maps lowercase HTML tag names to their constructor names.
// This follows the HTML specification for element interfaces.
var htmlElementTypeMap = map[string]string{
	// Sections
	"article": "HTMLElement",
	"aside":   "HTMLElement",
	"footer":  "HTMLElement",
	"header":  "HTMLElement",
	"nav":     "HTMLElement",
	"section": "HTMLElement",
	"main":    "HTMLElement",
	"hgroup":  "HTMLElement",

	// Headings
	"h1": "HTMLHeadingElement",
	"h2": "HTMLHeadingElement",
	"h3": "HTMLHeadingElement",
	"h4": "HTMLHeadingElement",
	"h5": "HTMLHeadingElement",
	"h6": "HTMLHeadingElement",

	// Content grouping
	"p":          "HTMLParagraphElement",
	"hr":         "HTMLHRElement",
	"pre":        "HTMLPreElement",
	"blockquote": "HTMLQuoteElement",
	"ol":         "HTMLOListElement",
	"ul":         "HTMLUListElement",
	"menu":       "HTMLMenuElement",
	"li":         "HTMLLIElement",
	"dl":         "HTMLDListElement",
	"dt":         "HTMLElement",
	"dd":         "HTMLElement",
	"figure":     "HTMLElement",
	"figcaption": "HTMLElement",
	"div":        "HTMLDivElement",

	// Text-level semantics
	"a":      "HTMLAnchorElement",
	"em":     "HTMLElement",
	"strong": "HTMLElement",
	"small":  "HTMLElement",
	"s":      "HTMLElement",
	"cite":   "HTMLElement",
	"q":      "HTMLQuoteElement",
	"dfn":    "HTMLElement",
	"abbr":   "HTMLElement",
	"ruby":   "HTMLElement",
	"rt":     "HTMLElement",
	"rp":     "HTMLElement",
	"data":   "HTMLDataElement",
	"time":   "HTMLTimeElement",
	"code":   "HTMLElement",
	"var":    "HTMLElement",
	"samp":   "HTMLElement",
	"kbd":    "HTMLElement",
	"sub":    "HTMLElement",
	"sup":    "HTMLElement",
	"i":      "HTMLElement",
	"b":      "HTMLElement",
	"u":      "HTMLElement",
	"mark":   "HTMLElement",
	"bdi":    "HTMLElement",
	"bdo":    "HTMLElement",
	"span":   "HTMLSpanElement",
	"br":     "HTMLBRElement",
	"wbr":    "HTMLElement",

	// Edits
	"ins": "HTMLModElement",
	"del": "HTMLModElement",

	// Embedded content
	"picture": "HTMLPictureElement",
	"source":  "HTMLSourceElement",
	"img":     "HTMLImageElement",
	"iframe":  "HTMLIFrameElement",
	"embed":   "HTMLEmbedElement",
	"object":  "HTMLObjectElement",
	"param":   "HTMLParamElement",
	"video":   "HTMLVideoElement",
	"audio":   "HTMLAudioElement",
	"track":   "HTMLTrackElement",
	"map":     "HTMLMapElement",
	"area":    "HTMLAreaElement",

	// Tabular data
	"table":    "HTMLTableElement",
	"caption":  "HTMLTableCaptionElement",
	"colgroup": "HTMLTableColElement",
	"col":      "HTMLTableColElement",
	"tbody":    "HTMLTableSectionElement",
	"thead":    "HTMLTableSectionElement",
	"tfoot":    "HTMLTableSectionElement",
	"tr":       "HTMLTableRowElement",
	"td":       "HTMLTableCellElement",
	"th":       "HTMLTableCellElement",

	// Forms
	"form":     "HTMLFormElement",
	"label":    "HTMLLabelElement",
	"input":    "HTMLInputElement",
	"button":   "HTMLButtonElement",
	"select":   "HTMLSelectElement",
	"datalist": "HTMLDataListElement",
	"optgroup": "HTMLOptGroupElement",
	"option":   "HTMLOptionElement",
	"textarea": "HTMLTextAreaElement",
	"output":   "HTMLOutputElement",
	"progress": "HTMLProgressElement",
	"meter":    "HTMLMeterElement",
	"fieldset": "HTMLFieldSetElement",
	"legend":   "HTMLLegendElement",

	// Interactive elements
	"details": "HTMLDetailsElement",
	"summary": "HTMLElement",
	"dialog":  "HTMLDialogElement",

	// Scripting
	"script":   "HTMLScriptElement",
	"noscript": "HTMLElement",
	"template": "HTMLTemplateElement",
	"slot":     "HTMLSlotElement",
	"canvas":   "HTMLCanvasElement",

	// Document metadata
	"head":  "HTMLHeadElement",
	"title": "HTMLTitleElement",
	"base":  "HTMLBaseElement",
	"link":  "HTMLLinkElement",
	"meta":  "HTMLMetaElement",
	"style": "HTMLStyleElement",

	// Root elements
	"html": "HTMLHtmlElement",
	"body": "HTMLBodyElement",

	// Deprecated but still used
	"font":     "HTMLFontElement",
	"center":   "HTMLElement",
	"dir":      "HTMLDirectoryElement",
	"frame":    "HTMLFrameElement",
	"frameset": "HTMLFrameSetElement",

	// Unknown elements use HTMLUnknownElement
}

// setupHTMLElementPrototypes creates prototypes for specific HTML element types.
// Each type extends HTMLElement and is mapped to specific tag names.
func (b *DOMBinder) setupHTMLElementPrototypes() {
	vm := b.runtime.vm

	// Collect unique constructor names
	constructorNames := make(map[string]bool)
	for _, constructorName := range htmlElementTypeMap {
		if constructorName != "HTMLElement" {
			constructorNames[constructorName] = true
		}
	}

	// Create prototype for each unique constructor type
	for constructorName := range constructorNames {
		proto := vm.NewObject()
		proto.SetPrototype(b.htmlElementProto)

		constructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
			panic(vm.NewTypeError("Illegal constructor"))
		})
		constructorObj := constructor.ToObject(vm)
		constructorObj.Set("prototype", proto)
		proto.Set("constructor", constructorObj)
		vm.Set(constructorName, constructorObj)

		// Store prototype for later lookup
		b.htmlElementProtoMap[constructorName] = proto
	}

	// Create HTMLUnknownElement for unknown tags
	unknownProto := vm.NewObject()
	unknownProto.SetPrototype(b.htmlElementProto)
	unknownConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	unknownConstructorObj := unknownConstructor.ToObject(vm)
	unknownConstructorObj.Set("prototype", unknownProto)
	unknownProto.Set("constructor", unknownConstructorObj)
	vm.Set("HTMLUnknownElement", unknownConstructorObj)
	b.htmlElementProtoMap["HTMLUnknownElement"] = unknownProto
}

// setupHTMLElementPrototypeMethods adds methods like click(), focus(), blur() to HTMLElement.prototype.
// Per the HTML specification, these methods are available on all HTML elements.
func (b *DOMBinder) setupHTMLElementPrototypeMethods() {
	vm := b.runtime.vm

	// click() - Simulates a click on the element
	// Per HTML spec: https://html.spec.whatwg.org/multipage/interaction.html#dom-click
	// "If this element is a form control that is disabled, then return."
	b.htmlElementProto.Set("click", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		if thisObj == nil {
			return goja.Undefined()
		}

		// Per HTML spec: "If this element is a form control that is disabled, then return."
		// Check if this is a disabled form control
		goNode := b.getGoNode(thisObj)
		if goNode != nil && goNode.NodeType() == dom.ElementNode {
			el := (*dom.Element)(goNode)
			// Check if it's a form control (input, button, select, textarea) and disabled
			localName := el.LocalName()
			isFormControl := localName == "input" || localName == "button" ||
				localName == "select" || localName == "textarea"
			if isFormControl && el.Disabled() {
				return goja.Undefined()
			}
		}

		// Create a MouseEvent with type "click"
		// bubbles: true, cancelable: true, view: null, detail: 0
		mouseEventCtor := vm.Get("MouseEvent")
		if mouseEventCtor == nil || goja.IsUndefined(mouseEventCtor) {
			return goja.Undefined()
		}

		ctor, ok := goja.AssertConstructor(mouseEventCtor)
		if !ok {
			return goja.Undefined()
		}

		// Create event options
		options := vm.NewObject()
		options.Set("bubbles", true)
		options.Set("cancelable", true)
		options.Set("view", goja.Null())
		options.Set("detail", 0)
		options.Set("screenX", 0)
		options.Set("screenY", 0)
		options.Set("clientX", 0)
		options.Set("clientY", 0)
		options.Set("ctrlKey", false)
		options.Set("shiftKey", false)
		options.Set("altKey", false)
		options.Set("metaKey", false)
		options.Set("button", 0)
		options.Set("buttons", 0)
		options.Set("relatedTarget", goja.Null())

		// Create the event
		event, err := ctor(nil, vm.ToValue("click"), options)
		if err != nil {
			return goja.Undefined()
		}

		// isTrusted should be false for synthetic events (events created via script)
		event.Set("isTrusted", false)

		// Dispatch the event using dispatchEvent on the element
		dispatchEvent := thisObj.Get("dispatchEvent")
		if dispatchEvent == nil || goja.IsUndefined(dispatchEvent) {
			return goja.Undefined()
		}

		fn, ok := goja.AssertFunction(dispatchEvent)
		if !ok {
			return goja.Undefined()
		}

		fn(thisObj, event)

		return goja.Undefined()
	})

	// focus() - Gives focus to the element
	// Per HTML spec: https://html.spec.whatwg.org/multipage/interaction.html#dom-focus
	b.htmlElementProto.Set("focus", func(call goja.FunctionCall) goja.Value {
		// For now, this is a stub - focus handling requires more infrastructure
		return goja.Undefined()
	})

	// blur() - Removes focus from the element
	// Per HTML spec: https://html.spec.whatwg.org/multipage/interaction.html#dom-blur
	b.htmlElementProto.Set("blur", func(call goja.FunctionCall) goja.Value {
		// For now, this is a stub - blur handling requires more infrastructure
		return goja.Undefined()
	})
}

// getHTMLElementPrototype returns the appropriate prototype for a given tag name.
// The tagName should be the localName of the element as stored - no case conversion is done.
// For createElement() in HTML documents, localName is already lowercased.
// For createElementNS(), localName preserves case, so "SPAN" won't match "span" in the map.
func (b *DOMBinder) getHTMLElementPrototype(tagName string) *goja.Object {
	// Look up the constructor name for this tag - case-sensitive lookup
	// The htmlElementTypeMap keys are lowercase, so only lowercase localNames will match
	constructorName, ok := htmlElementTypeMap[tagName]
	if !ok {
		// Unknown HTML element - use HTMLUnknownElement
		if proto, ok := b.htmlElementProtoMap["HTMLUnknownElement"]; ok {
			return proto
		}
		return b.htmlElementProto
	}

	// If it's just HTMLElement, return the htmlElementProto directly
	if constructorName == "HTMLElement" {
		return b.htmlElementProto
	}

	// Return the specific element prototype
	if proto, ok := b.htmlElementProtoMap[constructorName]; ok {
		return proto
	}

	return b.htmlElementProto
}

// BindDocument creates a JavaScript document object from a DOM document.
func (b *DOMBinder) BindDocument(doc *dom.Document) *goja.Object {
	// Check cache first to ensure document identity
	if jsDoc, ok := b.nodeMap[doc.AsNode()]; ok {
		return jsDoc
	}

	vm := b.runtime.vm
	jsDoc := vm.NewObject()

	// Cache the document before setting up properties
	b.nodeMap[doc.AsNode()] = jsDoc

	// Set prototype for instanceof to work
	if b.documentProto != nil {
		jsDoc.SetPrototype(b.documentProto)
	}

	// Store the first document for creating new nodes via constructors (new Text(), etc.)
	// Only set this once so iframe documents don't replace it.
	if b.document == nil {
		b.document = doc
	}

	// Store reference to the Go document
	jsDoc.Set("_goDoc", doc)

	// Document properties
	jsDoc.Set("nodeType", int(dom.DocumentNode))
	jsDoc.Set("nodeName", "#document")

	// Document accessors
	jsDoc.DefineAccessorProperty("doctype", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		doctype := doc.Doctype()
		if doctype == nil {
			return goja.Null()
		}
		return b.BindNode(doctype)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("documentElement", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.DocumentElement()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("head", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.Head()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("body", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.Body()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("title", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.Title())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			doc.SetTitle(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Document metadata properties (per DOM spec)
	jsDoc.DefineAccessorProperty("URL", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.URL())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("documentURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.DocumentURI())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("compatMode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CompatMode())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("characterSet", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// charset and inputEncoding are aliases for characterSet (per HTML spec)
	jsDoc.DefineAccessorProperty("charset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("inputEncoding", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("contentType", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.ContentType())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// location is null for documents without a browsing context (per spec)
	jsDoc.DefineAccessorProperty("location", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// defaultView returns the window object for documents with a browsing context
	// For the main document, this is the global window object
	jsDoc.DefineAccessorProperty("defaultView", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return the global window object
		return vm.GlobalObject()
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Document methods
	jsDoc.Set("getElementById", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		id := call.Arguments[0].String()
		el := doc.GetElementById(id)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsDoc.Set("getElementsByTagName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		tagName := call.Arguments[0].String()
		collection := doc.GetElementsByTagName(tagName)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByTagNameNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return b.createEmptyHTMLCollection()
		}
		// First arg is namespace (can be null for no namespace, or "*" for any)
		var namespaceURI string
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		collection := doc.GetElementsByTagNameNS(namespaceURI, localName)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByClassName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		classNames := call.Arguments[0].String()
		collection := doc.GetElementsByClassName(classNames)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		name := call.Arguments[0].String()
		nodeList := doc.GetElementsByName(name)
		return b.BindNodeList(nodeList)
	})

	jsDoc.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return goja.Null()
		}
		// For document queries, :scope matches the document root
		ctx := &css.MatchContext{ScopeElement: nil}
		// Search from document element
		docEl := doc.DocumentElement()
		if docEl == nil {
			return goja.Null()
		}
		// Check document element itself
		if parsed.MatchElementWithContext(docEl, ctx) {
			return b.BindElement(docEl)
		}
		found := querySelectorWithContext(docEl.AsNode(), parsed, ctx, true)
		if found == nil {
			return goja.Null()
		}
		return b.BindElement(found)
	})

	jsDoc.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return b.createEmptyNodeList()
		}
		// For document queries, :scope matches the document root
		ctx := &css.MatchContext{ScopeElement: nil}
		// Search from document element
		docEl := doc.DocumentElement()
		if docEl == nil {
			return b.createEmptyNodeList()
		}
		var results []*dom.Element
		// Check document element itself
		if parsed.MatchElementWithContext(docEl, ctx) {
			results = append(results, docEl)
		}
		// Get descendant matches
		descendantResults := querySelectorAllWithContext(docEl.AsNode(), parsed, ctx)
		results = append(results, descendantResults...)
		nodes := make([]*dom.Node, len(results))
		for i, result := range results {
			nodes[i] = result.AsNode()
		}
		return b.BindNodeList(dom.NewStaticNodeList(nodes))
	})

	jsDoc.Set("createElement", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		tagName := call.Arguments[0].String()
		el, err := doc.CreateElementWithError(tagName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindElement(el)
	})

	jsDoc.Set("createElementNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		el, err := doc.CreateElementNSWithError(namespaceURI, qualifiedName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("Error", err.Error()))
		}
		return b.BindElement(el)
	})

	jsDoc.Set("createTextNode", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node := doc.CreateTextNode(data)
		return b.BindNode(node)
	})

	jsDoc.Set("createComment", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node := doc.CreateComment(data)
		return b.BindNode(node)
	})

	jsDoc.Set("createCDATASection", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node, err := doc.CreateCDATASectionWithError(data)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("NotSupportedError", err.Error()))
		}
		return b.BindCDATASectionNode(node, nil)
	})

	jsDoc.Set("createProcessingInstruction", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'createProcessingInstruction' on 'Document': 2 arguments required."))
		}
		target := call.Arguments[0].String()
		data := call.Arguments[1].String()

		node, err := doc.CreateProcessingInstructionWithError(target, data)
		if err != nil {
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindProcessingInstructionNode(node, nil)
	})

	jsDoc.Set("createDocumentFragment", func(call goja.FunctionCall) goja.Value {
		frag := doc.CreateDocumentFragment()
		return b.BindDocumentFragment(frag)
	})

	jsDoc.Set("createRange", func(call goja.FunctionCall) goja.Value {
		r := doc.CreateRange()
		return b.BindRange(r)
	})

	jsDoc.Set("getSelection", func(call goja.FunctionCall) goja.Value {
		selection := doc.GetSelection()
		return b.BindSelection(selection)
	})

	jsDoc.Set("createAttribute", func(call goja.FunctionCall) goja.Value {
		name := ""
		if len(call.Arguments) > 0 {
			name = call.Arguments[0].String()
		}
		attr, err := doc.CreateAttributeWithError(name)
		if err != nil {
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindAttr(attr)
	})

	jsDoc.Set("createAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'createAttributeNS' on 'Document': 2 arguments required."))
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		attr, err := doc.CreateAttributeNSWithError(namespaceURI, qualifiedName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindAttr(attr)
	})

	jsDoc.Set("importNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)

		// Check if it's an Attr node first (uses _goAttr, not _goNode)
		if goAttr := b.getGoAttr(nodeObj); goAttr != nil {
			// Clone the Attr preserving namespace information
			clonedAttr := dom.NewAttrNS(goAttr.NamespaceURI(), goAttr.Name(), goAttr.Value())
			return b.BindAttr(clonedAttr)
		}

		goNode := b.getGoNode(nodeObj)
		if goNode == nil {
			return goja.Null()
		}
		deep := false
		if len(call.Arguments) > 1 {
			deep = call.Arguments[1].ToBoolean()
		}
		imported := doc.ImportNode(goNode, deep)
		if imported == nil {
			return goja.Null()
		}
		return b.BindNode(imported)
	})

	jsDoc.Set("adoptNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)
		goNode := b.getGoNode(nodeObj)
		if goNode == nil {
			return goja.Null()
		}
		adopted, err := doc.AdoptNodeWithError(goNode)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("NotSupportedError", err.Error()))
		}
		if adopted == nil {
			return goja.Null()
		}
		return b.BindNode(adopted)
	})

	// implementation property
	jsDoc.DefineAccessorProperty("implementation", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.bindDOMImplementation(doc.Implementation())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin properties
	jsDoc.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindHTMLCollection(doc.Children())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("childElementCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.ChildElementCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("firstElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := doc.FirstElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("lastElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := doc.LastElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin methods
	jsDoc.Set("append", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := doc.AppendWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsDoc.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := doc.PrependWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsDoc.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		if err := doc.ReplaceChildrenWithError(nodes...); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	// moveBefore - state-preserving atomic move API
	jsDoc.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 1 is not of type 'Node'."))
		}

		movedObj := movedNode.ToObject(vm)
		goMovedNode := b.getGoNode(movedObj)
		if goMovedNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL
		if referenceNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
			refChildObj := referenceNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 2 is not of type 'Node'."))
			}
		}

		err := doc.AsNode().MoveBefore(goMovedNode, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Undefined()
		}
		return goja.Undefined()
	})

	// document.createEvent(interface) - legacy method to create events
	jsDoc.Set("createEvent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createEvent' on 'Document': 1 argument required"))
		}
		interfaceName := call.Arguments[0].String()
		return b.createEventForInterface(interfaceName)
	})

	// document.createTreeWalker(root, whatToShow, filter) - creates a TreeWalker
	jsDoc.Set("createTreeWalker", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': 1 argument required"))
		}

		// Get root node - must be a Node
		rootArg := call.Arguments[0]
		if goja.IsNull(rootArg) || goja.IsUndefined(rootArg) {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': parameter 1 is not of type 'Node'."))
		}

		rootObj := rootArg.ToObject(vm)
		root := b.getGoNode(rootObj)
		if root == nil {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Get whatToShow (default 0xFFFFFFFF = SHOW_ALL)
		var whatToShow uint32 = 0xFFFFFFFF
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
			if goja.IsNull(call.Arguments[1]) {
				whatToShow = 0
			} else {
				whatToShow = uint32(call.Arguments[1].ToInteger())
			}
		}

		// Get filter (can be null/undefined, a function, or an object with acceptNode method)
		var filter goja.Value = goja.Null()
		if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
			filter = call.Arguments[2]
		}

		// Create the TreeWalker
		tw := doc.CreateTreeWalker(root, whatToShow)
		return b.BindTreeWalker(tw, root, whatToShow, filter)
	})

	// document.createNodeIterator(root, whatToShow, filter) - creates a NodeIterator
	jsDoc.Set("createNodeIterator", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': 1 argument required"))
		}

		// Get root node - must be a Node
		rootArg := call.Arguments[0]
		if goja.IsNull(rootArg) || goja.IsUndefined(rootArg) {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': parameter 1 is not of type 'Node'."))
		}

		rootObj := rootArg.ToObject(vm)
		root := b.getGoNode(rootObj)
		if root == nil {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Get whatToShow (default 0xFFFFFFFF = SHOW_ALL)
		var whatToShow uint32 = 0xFFFFFFFF
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
			if goja.IsNull(call.Arguments[1]) {
				whatToShow = 0
			} else {
				whatToShow = uint32(call.Arguments[1].ToInteger())
			}
		}

		// Get filter (can be null/undefined, a function, or an object with acceptNode method)
		var filter goja.Value = goja.Null()
		if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
			filter = call.Arguments[2]
		}

		// Create the NodeIterator
		ni := doc.CreateNodeIterator(root, whatToShow)
		return b.BindNodeIterator(ni, root, whatToShow, filter)
	})

	// textContent is null for Document (per spec) and setting it does nothing
	jsDoc.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Setting textContent has no effect on Document nodes
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Child node properties (document can have children)
	b.bindNodeProperties(jsDoc, doc.AsNode())

	// Set document on runtime without mutex (we're already in runtime context)
	// Only set the global document once - the first document bound becomes the global one.
	// Subsequent documents (e.g., iframe documents accessed via ownerDocument) should not
	// replace the global document.
	if !b.globalDocumentSet {
		b.runtime.setDocumentDirect(jsDoc)
		b.globalDocumentSet = true
	}
	return jsDoc
}

// BindIframeDocument binds a document for use in an iframe without setting it as the global document.
// Used for iframe content documents to avoid replacing the parent window's document.
func (b *DOMBinder) BindIframeDocument(doc *dom.Document) *goja.Object {
	return b.bindDocumentInternal(doc)
}

// BindIframeDocumentWithWindow binds a document for use in an iframe with a specific contentWindow.
// The contentWindow will be returned by the document's defaultView property.
func (b *DOMBinder) BindIframeDocumentWithWindow(doc *dom.Document, contentWindow *goja.Object) *goja.Object {
	// First, bind the document normally (which sets defaultView to null)
	jsDoc := b.bindDocumentInternal(doc)

	// Now override defaultView to return the contentWindow
	vm := b.runtime.vm
	jsDoc.DefineAccessorProperty("defaultView", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return contentWindow
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	return jsDoc
}

// bindDocumentInternal binds a document without setting it as the global document.
// Used for documents created via createHTMLDocument, createDocument, etc.
func (b *DOMBinder) bindDocumentInternal(doc *dom.Document) *goja.Object {
	// Check cache first to ensure document identity
	if jsDoc, ok := b.nodeMap[doc.AsNode()]; ok {
		return jsDoc
	}

	vm := b.runtime.vm
	jsDoc := vm.NewObject()

	// Cache the document before setting up properties to maintain identity
	b.nodeMap[doc.AsNode()] = jsDoc

	// Set prototype for instanceof to work
	if b.documentProto != nil {
		jsDoc.SetPrototype(b.documentProto)
	}

	// Store reference to the Go document
	jsDoc.Set("_goDoc", doc)

	// Document properties
	jsDoc.Set("nodeType", int(dom.DocumentNode))
	jsDoc.Set("nodeName", "#document")

	// Document accessors
	jsDoc.DefineAccessorProperty("doctype", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		doctype := doc.Doctype()
		if doctype == nil {
			return goja.Null()
		}
		return b.BindNode(doctype)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("documentElement", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.DocumentElement()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("head", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.Head()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("body", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		el := doc.Body()
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("title", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.Title())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			doc.SetTitle(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Document metadata properties (per DOM spec)
	jsDoc.DefineAccessorProperty("URL", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.URL())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("documentURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.DocumentURI())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("compatMode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CompatMode())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("characterSet", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// charset and inputEncoding are aliases for characterSet (per HTML spec)
	jsDoc.DefineAccessorProperty("charset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("inputEncoding", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.CharacterSet())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("contentType", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.ContentType())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// location is null for documents without a browsing context (per spec)
	jsDoc.DefineAccessorProperty("location", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// defaultView returns null for documents without a browsing context
	// (bindDocumentInternal is used for createHTMLDocument, createDocument, new Document(), cloneNode, etc.)
	// NOTE: We use goja.FLAG_TRUE for configurable so this can be overridden for iframe documents
	// that have a browsing context (see BindIframeDocumentWithWindow).
	jsDoc.DefineAccessorProperty("defaultView", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), nil, goja.FLAG_TRUE, goja.FLAG_TRUE)

	// Document methods
	jsDoc.Set("getElementById", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		id := call.Arguments[0].String()
		el := doc.GetElementById(id)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsDoc.Set("getElementsByTagName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		tagName := call.Arguments[0].String()
		collection := doc.GetElementsByTagName(tagName)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByTagNameNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return b.createEmptyHTMLCollection()
		}
		// First arg is namespace (can be null for no namespace, or "*" for any)
		var namespaceURI string
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		collection := doc.GetElementsByTagNameNS(namespaceURI, localName)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByClassName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		classNames := call.Arguments[0].String()
		collection := doc.GetElementsByClassName(classNames)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("getElementsByName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		name := call.Arguments[0].String()
		nodeList := doc.GetElementsByName(name)
		return b.BindNodeList(nodeList)
	})

	jsDoc.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return goja.Null()
		}
		// For document queries, :scope matches the document root
		ctx := &css.MatchContext{ScopeElement: nil}
		// Search from document element
		docEl := doc.DocumentElement()
		if docEl == nil {
			return goja.Null()
		}
		// Check document element itself
		if parsed.MatchElementWithContext(docEl, ctx) {
			return b.BindElement(docEl)
		}
		found := querySelectorWithContext(docEl.AsNode(), parsed, ctx, true)
		if found == nil {
			return goja.Null()
		}
		return b.BindElement(found)
	})

	jsDoc.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return b.createEmptyNodeList()
		}
		// For document queries, :scope matches the document root
		ctx := &css.MatchContext{ScopeElement: nil}
		// Search from document element
		docEl := doc.DocumentElement()
		if docEl == nil {
			return b.createEmptyNodeList()
		}
		var results []*dom.Element
		// Check document element itself
		if parsed.MatchElementWithContext(docEl, ctx) {
			results = append(results, docEl)
		}
		// Get descendant matches
		descendantResults := querySelectorAllWithContext(docEl.AsNode(), parsed, ctx)
		results = append(results, descendantResults...)
		nodes := make([]*dom.Node, len(results))
		for i, result := range results {
			nodes[i] = result.AsNode()
		}
		return b.BindNodeList(dom.NewStaticNodeList(nodes))
	})

	jsDoc.Set("createElement", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		tagName := call.Arguments[0].String()
		el, err := doc.CreateElementWithError(tagName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindElement(el)
	})

	jsDoc.Set("createElementNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		el, err := doc.CreateElementNSWithError(namespaceURI, qualifiedName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("Error", err.Error()))
		}
		return b.BindElement(el)
	})

	jsDoc.Set("createTextNode", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node := doc.CreateTextNode(data)
		return b.BindNode(node)
	})

	jsDoc.Set("createComment", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node := doc.CreateComment(data)
		return b.BindNode(node)
	})

	jsDoc.Set("createCDATASection", func(call goja.FunctionCall) goja.Value {
		data := ""
		if len(call.Arguments) > 0 {
			data = call.Arguments[0].String()
		}
		node, err := doc.CreateCDATASectionWithError(data)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("NotSupportedError", err.Error()))
		}
		return b.BindCDATASectionNode(node, nil)
	})

	jsDoc.Set("createProcessingInstruction", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'createProcessingInstruction' on 'Document': 2 arguments required."))
		}
		target := call.Arguments[0].String()
		data := call.Arguments[1].String()

		node, err := doc.CreateProcessingInstructionWithError(target, data)
		if err != nil {
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindProcessingInstructionNode(node, nil)
	})

	jsDoc.Set("createDocumentFragment", func(call goja.FunctionCall) goja.Value {
		frag := doc.CreateDocumentFragment()
		return b.BindDocumentFragment(frag)
	})

	jsDoc.Set("createRange", func(call goja.FunctionCall) goja.Value {
		r := doc.CreateRange()
		return b.BindRange(r)
	})

	jsDoc.Set("getSelection", func(call goja.FunctionCall) goja.Value {
		selection := doc.GetSelection()
		return b.BindSelection(selection)
	})

	jsDoc.Set("createAttribute", func(call goja.FunctionCall) goja.Value {
		name := ""
		if len(call.Arguments) > 0 {
			name = call.Arguments[0].String()
		}
		attr, err := doc.CreateAttributeWithError(name)
		if err != nil {
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindAttr(attr)
	})

	jsDoc.Set("createAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'createAttributeNS' on 'Document': 2 arguments required."))
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		attr, err := doc.CreateAttributeNSWithError(namespaceURI, qualifiedName)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidCharacterError", err.Error()))
		}
		return b.BindAttr(attr)
	})

	jsDoc.Set("importNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)

		// Check if it's an Attr node first (uses _goAttr, not _goNode)
		if goAttr := b.getGoAttr(nodeObj); goAttr != nil {
			// Clone the Attr preserving namespace information
			clonedAttr := dom.NewAttrNS(goAttr.NamespaceURI(), goAttr.Name(), goAttr.Value())
			return b.BindAttr(clonedAttr)
		}

		goNode := b.getGoNode(nodeObj)
		if goNode == nil {
			return goja.Null()
		}
		deep := false
		if len(call.Arguments) > 1 {
			deep = call.Arguments[1].ToBoolean()
		}
		imported := doc.ImportNode(goNode, deep)
		if imported == nil {
			return goja.Null()
		}
		return b.BindNode(imported)
	})

	jsDoc.Set("adoptNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)
		goNode := b.getGoNode(nodeObj)
		if goNode == nil {
			return goja.Null()
		}
		adopted, err := doc.AdoptNodeWithError(goNode)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("NotSupportedError", err.Error()))
		}
		if adopted == nil {
			return goja.Null()
		}
		return b.BindNode(adopted)
	})

	// implementation property
	jsDoc.DefineAccessorProperty("implementation", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.bindDOMImplementation(doc.Implementation())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin properties
	jsDoc.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindHTMLCollection(doc.Children())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("childElementCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(doc.ChildElementCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("firstElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := doc.FirstElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsDoc.DefineAccessorProperty("lastElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := doc.LastElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin methods
	jsDoc.Set("append", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := doc.AppendWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsDoc.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := doc.PrependWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsDoc.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		if err := doc.ReplaceChildrenWithError(nodes...); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	// moveBefore - state-preserving atomic move API
	jsDoc.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 1 is not of type 'Node'."))
		}

		movedObj := movedNode.ToObject(vm)
		goMovedNode := b.getGoNode(movedObj)
		if goMovedNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL
		if referenceNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
			refChildObj := referenceNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Document': parameter 2 is not of type 'Node'."))
			}
		}

		err := doc.AsNode().MoveBefore(goMovedNode, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Undefined()
		}
		return goja.Undefined()
	})

	// document.createEvent(interface) - legacy method to create events
	jsDoc.Set("createEvent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createEvent' on 'Document': 1 argument required"))
		}
		interfaceName := call.Arguments[0].String()
		return b.createEventForInterface(interfaceName)
	})

	// document.createTreeWalker(root, whatToShow, filter) - creates a TreeWalker
	jsDoc.Set("createTreeWalker", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': 1 argument required"))
		}

		// Get root node - must be a Node
		rootArg := call.Arguments[0]
		if goja.IsNull(rootArg) || goja.IsUndefined(rootArg) {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': parameter 1 is not of type 'Node'."))
		}

		rootObj := rootArg.ToObject(vm)
		root := b.getGoNode(rootObj)
		if root == nil {
			panic(vm.NewTypeError("Failed to execute 'createTreeWalker' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Get whatToShow (default 0xFFFFFFFF = SHOW_ALL)
		var whatToShow uint32 = 0xFFFFFFFF
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
			if goja.IsNull(call.Arguments[1]) {
				whatToShow = 0
			} else {
				whatToShow = uint32(call.Arguments[1].ToInteger())
			}
		}

		// Get filter (can be null/undefined, a function, or an object with acceptNode method)
		var filter goja.Value = goja.Null()
		if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
			filter = call.Arguments[2]
		}

		// Create the TreeWalker
		tw := doc.CreateTreeWalker(root, whatToShow)
		return b.BindTreeWalker(tw, root, whatToShow, filter)
	})

	// document.createNodeIterator(root, whatToShow, filter) - creates a NodeIterator
	jsDoc.Set("createNodeIterator", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': 1 argument required"))
		}

		// Get root node - must be a Node
		rootArg := call.Arguments[0]
		if goja.IsNull(rootArg) || goja.IsUndefined(rootArg) {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': parameter 1 is not of type 'Node'."))
		}

		rootObj := rootArg.ToObject(vm)
		root := b.getGoNode(rootObj)
		if root == nil {
			panic(vm.NewTypeError("Failed to execute 'createNodeIterator' on 'Document': parameter 1 is not of type 'Node'."))
		}

		// Get whatToShow (default 0xFFFFFFFF = SHOW_ALL)
		var whatToShow uint32 = 0xFFFFFFFF
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
			if goja.IsNull(call.Arguments[1]) {
				whatToShow = 0
			} else {
				whatToShow = uint32(call.Arguments[1].ToInteger())
			}
		}

		// Get filter (can be null/undefined, a function, or an object with acceptNode method)
		var filter goja.Value = goja.Null()
		if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
			filter = call.Arguments[2]
		}

		// Create the NodeIterator
		ni := doc.CreateNodeIterator(root, whatToShow)
		return b.BindNodeIterator(ni, root, whatToShow, filter)
	})

	// textContent is null for Document (per spec) and setting it does nothing
	jsDoc.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Setting textContent has no effect on Document nodes
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Child node properties (document can have children)
	b.bindNodeProperties(jsDoc, doc.AsNode())

	// Do NOT set global document here - this is for internal documents
	return jsDoc
}

// bindXMLDocument creates a JavaScript object for an XMLDocument.
// This is used for documents created via createDocument.
func (b *DOMBinder) bindXMLDocument(doc *dom.Document) *goja.Object {
	// Use the same binding as bindDocumentInternal, but set XMLDocument prototype
	jsDoc := b.bindDocumentInternal(doc)

	// Override the prototype to be XMLDocument
	if b.xmlDocumentProto != nil {
		jsDoc.SetPrototype(b.xmlDocumentProto)
	}

	return jsDoc
}

// bindDOMImplementation creates a JavaScript object for a DOMImplementation.
func (b *DOMBinder) bindDOMImplementation(impl *dom.DOMImplementation) *goja.Object {
	if impl == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.domImplementationCache[impl]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsImpl := vm.NewObject()

	// Set prototype for instanceof to work
	if b.domImplementationProto != nil {
		jsImpl.SetPrototype(b.domImplementationProto)
	}

	// createHTMLDocument(title)
	jsImpl.Set("createHTMLDocument", func(call goja.FunctionCall) goja.Value {
		var title *string
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
			t := call.Arguments[0].String()
			title = &t
		}
		doc := impl.CreateHTMLDocument(title)
		return b.bindDocumentInternal(doc)
	})

	// createDocument(namespaceURI, qualifiedName, doctype)
	jsImpl.Set("createDocument", func(call goja.FunctionCall) goja.Value {
		// Per WebIDL, createDocument requires at least 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'createDocument' on 'DOMImplementation': 2 arguments required, but only " + fmt.Sprint(len(call.Arguments)) + " present."))
		}

		namespaceURI := ""
		qualifiedName := ""
		var doctype *dom.Node

		if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		arg := call.Arguments[1]
		// Per spec: null → empty string, undefined → "undefined"
		if goja.IsNull(arg) {
			qualifiedName = ""
		} else if goja.IsUndefined(arg) {
			qualifiedName = "undefined"
		} else {
			qualifiedName = arg.String()
		}
		if len(call.Arguments) > 2 {
			arg := call.Arguments[2]
			if !goja.IsNull(arg) && !goja.IsUndefined(arg) {
				// Must be a DocumentType node or throw TypeError
				obj := arg.ToObject(vm)
				node := b.getGoNode(obj)
				if node == nil || node.NodeType() != dom.DocumentTypeNode {
					panic(vm.NewTypeError("Failed to execute 'createDocument' on 'DOMImplementation': parameter 3 is not of type 'DocumentType'."))
				}
				doctype = node
			}
		}

		doc, err := impl.CreateDocument(namespaceURI, qualifiedName, doctype)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("Error", err.Error()))
		}
		return b.bindXMLDocument(doc)
	})

	// createDocumentType(qualifiedName, publicId, systemId)
	jsImpl.Set("createDocumentType", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			panic(vm.NewTypeError("Failed to execute 'createDocumentType': 3 arguments required"))
		}
		qualifiedName := call.Arguments[0].String()
		publicId := call.Arguments[1].String()
		systemId := call.Arguments[2].String()

		doctype, err := impl.CreateDocumentType(qualifiedName, publicId, systemId)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("Error", err.Error()))
		}
		return b.BindNode(doctype)
	})

	// hasFeature() - always returns true per spec
	jsImpl.Set("hasFeature", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(true)
	})

	// Cache
	b.domImplementationCache[impl] = jsImpl

	return jsImpl
}

// BindElement creates a JavaScript element object from a DOM element.
func (b *DOMBinder) BindElement(el *dom.Element) *goja.Object {
	if el == nil {
		return nil
	}

	node := el.AsNode()

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsEl := vm.NewObject()

	// Set prototype for instanceof to work
	// Use specific HTML element prototype ONLY for elements with HTML namespace.
	// Elements created with createElementNS(null, ...) or createElementNS("", ...)
	// should NOT be HTMLElement instances, even in HTML documents.
	ns := el.NamespaceURI()
	if ns == dom.HTMLNamespace {
		// HTML element - use specific HTML element prototype
		jsEl.SetPrototype(b.getHTMLElementPrototype(el.LocalName()))
	} else if b.elementProto != nil {
		// Non-HTML element (like SVG) or element with null/empty namespace - use generic Element prototype
		jsEl.SetPrototype(b.elementProto)
	}

	// Store reference to the Go element
	jsEl.Set("_goElement", el)
	jsEl.Set("_goNode", node)

	// Node properties
	jsEl.Set("nodeType", int(dom.ElementNode))
	jsEl.DefineAccessorProperty("nodeName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.NodeName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Element properties
	jsEl.DefineAccessorProperty("tagName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.TagName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("localName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.LocalName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("namespaceURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		ns := el.NamespaceURI()
		if ns == "" {
			return goja.Null()
		}
		return vm.ToValue(ns)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("prefix", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		prefix := el.Prefix()
		if prefix == "" {
			return goja.Null()
		}
		return vm.ToValue(prefix)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("id", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.Id())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetId(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("className", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ClassName())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetClassName(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("classList", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindDOMTokenList(el.ClassList())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Per spec, assigning to classList sets the class attribute value
		if len(call.Arguments) > 0 {
			el.SetClassName(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("style", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindCSSStyleDeclaration(el.Style())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Per CSSOM spec, setting element.style = "string" is equivalent to setting style.cssText
		if len(call.Arguments) > 0 {
			el.Style().SetCSSText(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("innerHTML", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.InnerHTML())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetInnerHTML(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("outerHTML", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.OuterHTML())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetOuterHTML(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			arg := call.Arguments[0]
			// Per spec, null and undefined are treated as empty string
			var value string
			if goja.IsNull(arg) || goja.IsUndefined(arg) {
				value = ""
			} else {
				value = arg.String()
			}
			el.SetTextContent(value)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Attribute methods
	jsEl.Set("getAttribute", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		if !el.HasAttribute(name) {
			return goja.Null()
		}
		return vm.ToValue(el.GetAttribute(name))
	})

	jsEl.Set("getAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		if !el.HasAttributeNS(ns, localName) {
			return goja.Null()
		}
		return vm.ToValue(el.GetAttributeNS(ns, localName))
	})

	jsEl.Set("setAttribute", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		value := call.Arguments[1].String()

		// Check if this is an event handler content attribute
		lowerName := strings.ToLower(name)
		if strings.HasPrefix(lowerName, "on") && len(lowerName) > 2 {
			// Check if it's a known event handler attribute
			for _, attr := range eventHandlerAttributes {
				if lowerName == attr {
					// Create an event handler function per HTML spec
					// The handler should:
					// 1. Have 'this' bound to the element (handled in event dispatch)
					// 2. Have a scope chain including the element (respecting @@unscopables)
					//
					// Per HTML spec (https://html.spec.whatwg.org/multipage/webappapis.html#getting-the-current-value-of-the-event-handler):
					// For regular elements, the scope chain is: element
					// For body/frameset elements, the scope chain is: element, then document
					// Note: form owner is not included for simplicity
					//
					// Since goja's with statement automatically respects Symbol.unscopables,
					// unscopable properties (like prepend, append, remove, etc.) won't be
					// found in the element scope, allowing them to fall through to global scope
					handlerCode := fmt.Sprintf(`(function(__handler_el__) {
						return function(event) {
							with (__handler_el__) {
								%s
							}
						};
					})`, value)

					handlerFactory, err := vm.RunString(handlerCode)
					if err == nil {
						// Call the factory with the element to bind it
						factory, _ := goja.AssertFunction(handlerFactory)
						if factory != nil {
							handlerVal, callErr := factory(goja.Undefined(), jsEl)
							if callErr == nil {
								// Set the event handler via the property
								jsEl.Set(attr, handlerVal)
							}
						}
					}
					// Still set the attribute on the element
					break
				}
			}
		}

		if err := el.SetAttributeWithError(name, value); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
		}
		return goja.Undefined()
	})

	jsEl.Set("setAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			return goja.Undefined()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		value := call.Arguments[2].String()
		if err := el.SetAttributeNSWithError(ns, qualifiedName, value); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
		}
		return goja.Undefined()
	})

	jsEl.Set("hasAttribute", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		name := call.Arguments[0].String()
		return vm.ToValue(el.HasAttribute(name))
	})

	jsEl.Set("hasAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(false)
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		return vm.ToValue(el.HasAttributeNS(ns, localName))
	})

	jsEl.Set("removeAttribute", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		el.RemoveAttribute(name)
		return goja.Undefined()
	})

	jsEl.Set("removeAttributeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		el.RemoveAttributeNS(ns, localName)
		return goja.Undefined()
	})

	jsEl.Set("toggleAttribute", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		name := call.Arguments[0].String()
		var result bool
		var err error
		if len(call.Arguments) > 1 {
			force := call.Arguments[1].ToBoolean()
			result, err = el.ToggleAttributeWithError(name, force)
		} else {
			result, err = el.ToggleAttributeWithError(name)
		}
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
		}
		return vm.ToValue(result)
	})

	jsEl.Set("hasAttributes", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.Attributes().Length() > 0)
	})

	jsEl.Set("getAttributeNames", func(call goja.FunctionCall) goja.Value {
		attrs := el.Attributes()
		names := make([]string, attrs.Length())
		for i := 0; i < attrs.Length(); i++ {
			attr := attrs.Item(i)
			if attr != nil {
				names[i] = attr.Name()
			}
		}
		return vm.ToValue(names)
	})

	// attributes property - returns NamedNodeMap
	jsEl.DefineAccessorProperty("attributes", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindNamedNodeMap(el.Attributes())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// getAttributeNode method - returns Attr or null
	jsEl.Set("getAttributeNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		attr := el.GetAttributeNode(name)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// getAttributeNodeNS method - returns Attr or null
	jsEl.Set("getAttributeNodeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		ns := ""
		if !goja.IsNull(call.Arguments[0]) {
			ns = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		attr := el.GetAttributeNodeNS(ns, localName)
		if attr == nil {
			return goja.Null()
		}
		return b.BindAttr(attr)
	})

	// setAttributeNode method
	jsEl.Set("setAttributeNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		jsAttr := call.Arguments[0].ToObject(vm)
		goAttr := b.extractAttr(jsAttr)
		if goAttr == nil {
			return goja.Null()
		}
		oldAttr, err := el.SetAttributeNodeWithError(goAttr)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
		}
		if oldAttr == nil {
			return goja.Null()
		}
		return b.BindAttr(oldAttr)
	})

	// setAttributeNodeNS method
	jsEl.Set("setAttributeNodeNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		jsAttr := call.Arguments[0].ToObject(vm)
		goAttr := b.extractAttr(jsAttr)
		if goAttr == nil {
			return goja.Null()
		}
		oldAttr, err := el.SetAttributeNodeNSWithError(goAttr)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
		}
		if oldAttr == nil {
			return goja.Null()
		}
		return b.BindAttr(oldAttr)
	})

	// removeAttributeNode method
	jsEl.Set("removeAttributeNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		jsAttr := call.Arguments[0].ToObject(vm)
		goAttr := b.extractAttr(jsAttr)
		if goAttr == nil {
			return goja.Null()
		}
		removedAttr := el.RemoveAttributeNode(goAttr)
		if removedAttr == nil {
			return goja.Null()
		}
		return b.BindAttr(removedAttr)
	})

	// Query methods - use CSS matcher with scope context for proper :scope support
	jsEl.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return goja.Null()
		}
		// Create scope context pointing to the element querySelector was called on
		ctx := &css.MatchContext{ScopeElement: el}
		found := querySelectorWithContext(el.AsNode(), parsed, ctx, true)
		if found == nil {
			return goja.Null()
		}
		return b.BindElement(found)
	})

	jsEl.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selectorStr := call.Arguments[0].String()
		parsed, err := css.ParseSelector(selectorStr)
		if err != nil {
			return b.createEmptyNodeList()
		}
		// Create scope context pointing to the element querySelectorAll was called on
		ctx := &css.MatchContext{ScopeElement: el}
		results := querySelectorAllWithContext(el.AsNode(), parsed, ctx)
		nodes := make([]*dom.Node, len(results))
		for i, result := range results {
			nodes[i] = result.AsNode()
		}
		return b.BindNodeList(dom.NewStaticNodeList(nodes))
	})

	jsEl.Set("getElementsByTagName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		tagName := call.Arguments[0].String()
		collection := el.GetElementsByTagName(tagName)
		return b.BindHTMLCollection(collection)
	})

	jsEl.Set("getElementsByTagNameNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return b.createEmptyHTMLCollection()
		}
		// First arg is namespace (can be null for no namespace, or "*" for any)
		var namespaceURI string
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		localName := call.Arguments[1].String()
		collection := el.GetElementsByTagNameNS(namespaceURI, localName)
		return b.BindHTMLCollection(collection)
	})

	jsEl.Set("getElementsByClassName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		classNames := call.Arguments[0].String()
		collection := el.GetElementsByClassName(classNames)
		return b.BindHTMLCollection(collection)
	})

	// Shadow DOM methods
	jsEl.DefineAccessorProperty("shadowRoot", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		sr := el.ShadowRoot()
		if sr == nil {
			return goja.Null()
		}
		return b.BindShadowRoot(sr)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.Set("attachShadow", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'attachShadow' on 'Element': 1 argument required, but only 0 present."))
		}

		initObj := call.Arguments[0].ToObject(vm)
		if initObj == nil {
			panic(vm.NewTypeError("Failed to execute 'attachShadow' on 'Element': The provided value is not of type 'ShadowRootInit'."))
		}

		// Get mode (required)
		modeVal := initObj.Get("mode")
		if modeVal == nil || goja.IsUndefined(modeVal) {
			panic(vm.NewTypeError("Failed to execute 'attachShadow' on 'Element': Failed to read the 'mode' property from 'ShadowRootInit': Required member is undefined."))
		}
		modeStr := modeVal.String()
		var mode dom.ShadowRootMode
		if modeStr == "open" {
			mode = dom.ShadowRootModeOpen
		} else if modeStr == "closed" {
			mode = dom.ShadowRootModeClosed
		} else {
			panic(vm.NewTypeError("Failed to execute 'attachShadow' on 'Element': The provided value '" + modeStr + "' is not a valid enum value of type ShadowRootMode."))
		}

		// Get optional parameters
		options := make(map[string]interface{})
		if df := initObj.Get("delegatesFocus"); df != nil && !goja.IsUndefined(df) {
			options["delegatesFocus"] = df.ToBoolean()
		}
		if sa := initObj.Get("slotAssignment"); sa != nil && !goja.IsUndefined(sa) {
			options["slotAssignment"] = sa.String()
		}
		if c := initObj.Get("clonable"); c != nil && !goja.IsUndefined(c) {
			options["clonable"] = c.ToBoolean()
		}
		if s := initObj.Get("serializable"); s != nil && !goja.IsUndefined(s) {
			options["serializable"] = s.ToBoolean()
		}

		sr, err := el.AttachShadow(mode, options)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			panic(vm.NewTypeError(err.Error()))
		}

		return b.BindShadowRoot(sr)
	})

	matchesFunc := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		selector := call.Arguments[0].String()
		// Use CSS selector matcher for proper pseudo-class support
		parsed, err := css.ParseSelector(selector)
		if err != nil {
			return vm.ToValue(false)
		}
		// For matches(), scope is the element itself
		ctx := &css.MatchContext{ScopeElement: el}
		return vm.ToValue(parsed.MatchElementWithContext(el, ctx))
	}
	jsEl.Set("matches", matchesFunc)
	jsEl.Set("webkitMatchesSelector", matchesFunc)

	jsEl.Set("closest", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		// Parse the selector once
		parsed, err := css.ParseSelector(selector)
		if err != nil {
			return goja.Null()
		}
		// For closest(), scope is the element closest was called on
		ctx := &css.MatchContext{ScopeElement: el}
		// Walk up the tree
		for current := el; current != nil; {
			if parsed.MatchElementWithContext(current, ctx) {
				return b.BindElement(current)
			}
			parent := current.AsNode().ParentNode()
			if parent == nil || parent.NodeType() != dom.ElementNode {
				break
			}
			current = (*dom.Element)(parent)
		}
		return goja.Null()
	})

	// ParentNode mixin properties
	jsEl.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindHTMLCollection(el.Children())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("childElementCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ChildElementCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("firstElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := el.FirstElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("lastElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := el.LastElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// NonDocumentTypeChildNode mixin properties
	jsEl.DefineAccessorProperty("previousElementSibling", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		sibling := el.PreviousElementSibling()
		if sibling == nil {
			return goja.Null()
		}
		return b.BindElement(sibling)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("nextElementSibling", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		sibling := el.NextElementSibling()
		if sibling == nil {
			return goja.Null()
		}
		return b.BindElement(sibling)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// DOM manipulation methods - ChildNode mixin
	jsEl.Set("remove", func(call goja.FunctionCall) goja.Value {
		el.Remove()
		return goja.Undefined()
	})

	jsEl.Set("before", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		el.Before(nodes...)
		return goja.Undefined()
	})

	jsEl.Set("after", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		el.After(nodes...)
		return goja.Undefined()
	})

	jsEl.Set("replaceWith", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		el.ReplaceWith(nodes...)
		return goja.Undefined()
	})

	// ParentNode mixin methods
	jsEl.Set("append", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := el.AppendWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsEl.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		err := el.PrependWithError(nodes...)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsEl.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		if err := el.ReplaceChildrenWithError(nodes...); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	// moveBefore - state-preserving atomic move API
	jsEl.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Element': parameter 1 is not of type 'Node'."))
		}

		movedObj := movedNode.ToObject(vm)
		goMovedNode := b.getGoNode(movedObj)
		if goMovedNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Element': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL
		if referenceNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Element': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
			refChildObj := referenceNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Element': parameter 2 is not of type 'Node'."))
			}
		}

		err := el.AsNode().MoveBefore(goMovedNode, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Undefined()
		}
		return goja.Undefined()
	})

	// insertAdjacentElement
	jsEl.Set("insertAdjacentElement", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		position := call.Arguments[0].String()
		// Get the element argument
		elementArg := call.Arguments[1]
		if goja.IsNull(elementArg) || goja.IsUndefined(elementArg) {
			return goja.Null()
		}
		// Convert to Element
		elementObj := elementArg.ToObject(b.runtime.vm)
		nodeVal := elementObj.Get("_goNode")
		if nodeVal == nil || goja.IsUndefined(nodeVal) || goja.IsNull(nodeVal) {
			return goja.Null()
		}
		goNode, ok := nodeVal.Export().(*dom.Node)
		if !ok || goNode == nil {
			return goja.Null()
		}
		if goNode.NodeType() != dom.ElementNode {
			return goja.Null()
		}
		insertedEl, err := el.InsertAdjacentElement(position, (*dom.Element)(goNode))
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(b.runtime.vm, domErr)
			}
			panic(b.runtime.vm.NewGoError(err))
		}
		if insertedEl == nil {
			return goja.Null()
		}
		return b.BindElement(insertedEl)
	})

	// insertAdjacentText
	jsEl.Set("insertAdjacentText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		position := call.Arguments[0].String()
		data := call.Arguments[1].String()
		err := el.InsertAdjacentText(position, data)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(b.runtime.vm, domErr)
			}
			panic(b.runtime.vm.NewGoError(err))
		}
		return goja.Undefined()
	})

	// Geometry methods
	jsEl.Set("getBoundingClientRect", func(call goja.FunctionCall) goja.Value {
		rect := el.GetBoundingClientRect()
		return b.BindDOMRect(rect)
	})

	jsEl.Set("getClientRects", func(call goja.FunctionCall) goja.Value {
		rects := el.GetClientRects()
		return b.BindDOMRectList(rects)
	})

	// HTMLElement geometry properties
	jsEl.DefineAccessorProperty("offsetWidth", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.OffsetWidth())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("offsetHeight", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.OffsetHeight())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("offsetTop", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.OffsetTop())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("offsetLeft", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.OffsetLeft())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("offsetParent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		parent := el.OffsetParent()
		if parent == nil {
			return goja.Null()
		}
		return b.BindElement(parent)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("clientWidth", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ClientWidth())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("clientHeight", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ClientHeight())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("clientTop", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ClientTop())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("clientLeft", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ClientLeft())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("scrollWidth", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ScrollWidth())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("scrollHeight", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ScrollHeight())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("scrollTop", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ScrollTop())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].ToFloat()
			el.SetScrollTop(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("scrollLeft", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.ScrollLeft())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].ToFloat()
			el.SetScrollLeft(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Add iframe-specific properties (contentWindow, contentDocument, src)
	if el.LocalName() == "iframe" {
		b.bindIframeProperties(jsEl, el)
	}

	// Add template-specific properties (content)
	if el.LocalName() == "template" && ns == dom.HTMLNamespace {
		b.bindTemplateProperties(jsEl, el)
	}

	// Add anchor-specific properties (href with URL encoding)
	if el.LocalName() == "a" && ns == dom.HTMLNamespace {
		b.bindAnchorProperties(jsEl, el)
	}

	// Add input-specific properties (type, checked, disabled, value, etc.)
	if el.LocalName() == "input" && ns == dom.HTMLNamespace {
		b.bindInputProperties(jsEl, el)
	}

	// Bind common node properties and methods
	b.bindNodeProperties(jsEl, node)

	// Add event handler IDL attributes (onclick, onload, etc.)
	// Only for HTML elements, per spec
	if ns == dom.HTMLNamespace {
		b.bindEventHandlerAttributes(jsEl)
		// Process any existing event handler content attributes
		// This handles elements created from HTML parsing or template cloning
		b.processEventHandlerContentAttributes(jsEl, el)
		// Add HTML global attributes (title, lang, hidden, dir, tabIndex, etc.)
		b.bindHTMLGlobalAttributes(jsEl, el)
	}

	// Cache the binding
	b.nodeMap[node] = jsEl

	return jsEl
}

// bindIframeProperties adds HTMLIFrameElement-specific properties.
func (b *DOMBinder) bindIframeProperties(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// src attribute property
	jsEl.DefineAccessorProperty("src", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		src := el.GetAttribute("src")
		return vm.ToValue(src)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("src", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// contentWindow property - returns the Window object for the iframe
	jsEl.DefineAccessorProperty("contentWindow", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if b.iframeContentProvider == nil {
			return goja.Null()
		}
		contentWindow, _ := b.iframeContentProvider(el)
		if contentWindow == nil {
			return goja.Null()
		}
		return contentWindow
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// contentDocument property - returns the Document for the iframe
	jsEl.DefineAccessorProperty("contentDocument", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if b.iframeContentProvider == nil {
			return goja.Null()
		}
		_, contentDocument := b.iframeContentProvider(el)
		if contentDocument == nil {
			return goja.Null()
		}
		return contentDocument
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// bindTemplateProperties adds HTMLTemplateElement-specific properties.
func (b *DOMBinder) bindTemplateProperties(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// content property - returns the template's DocumentFragment containing its contents
	// Per HTML spec: https://html.spec.whatwg.org/multipage/scripting.html#the-template-element
	jsEl.DefineAccessorProperty("content", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		content := el.TemplateContent()
		if content == nil {
			return goja.Null()
		}
		return b.BindDocumentFragment(content)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// encodeURLForHref encodes non-ASCII characters in a URL per WHATWG URL spec.
func encodeURLForHref(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	// Encode non-ASCII characters in each component
	// The query string needs special handling - Go's url.Parse preserves non-ASCII chars
	if parsed.RawQuery != "" {
		// Encode non-ASCII characters in the query string
		encoded := ""
		for _, r := range parsed.RawQuery {
			if r > 127 {
				// Encode non-ASCII as UTF-8 percent-encoded
				encoded += url.QueryEscape(string(r))
			} else {
				encoded += string(r)
			}
		}
		parsed.RawQuery = encoded
	}

	return parsed.String()
}

// bindAnchorProperties adds HTMLAnchorElement-specific properties with proper URL encoding.
func (b *DOMBinder) bindAnchorProperties(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// href property with URL encoding per WHATWG URL spec
	jsEl.DefineAccessorProperty("href", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		href := el.GetAttribute("href")
		if href == "" {
			return vm.ToValue("")
		}
		return vm.ToValue(encodeURLForHref(href))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			hrefVal := call.Arguments[0].String()
			// Store the URL as-is; encoding happens in the getter
			el.SetAttribute("href", hrefVal)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// bindInputProperties adds HTMLInputElement-specific properties.
// Per HTML spec: https://html.spec.whatwg.org/multipage/input.html
func (b *DOMBinder) bindInputProperties(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// type property - the type of input (checkbox, radio, text, etc.)
	jsEl.DefineAccessorProperty("type", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.InputType())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetInputType(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// checked property - the current checked state for checkbox/radio
	// Per HTML spec, this is the checkedness, not the checked attribute
	jsEl.DefineAccessorProperty("checked", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.Checked())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetChecked(call.Arguments[0].ToBoolean())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// defaultChecked property - reflects the checked attribute
	jsEl.DefineAccessorProperty("defaultChecked", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.DefaultChecked())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetDefaultChecked(call.Arguments[0].ToBoolean())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// disabled property - whether the input is disabled
	jsEl.DefineAccessorProperty("disabled", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.Disabled())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetDisabled(call.Arguments[0].ToBoolean())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// value property - the current value
	// Per HTML spec, this is separate from the value attribute (defaultValue)
	jsEl.DefineAccessorProperty("value", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.InputValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetInputValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// defaultValue property - reflects the value attribute
	jsEl.DefineAccessorProperty("defaultValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.DefaultValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetDefaultValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// bindHTMLGlobalAttributes adds HTMLElement global attributes.
// Per HTML spec: https://html.spec.whatwg.org/multipage/dom.html#global-attributes
// These are available on all HTML elements.
func (b *DOMBinder) bindHTMLGlobalAttributes(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// title - reflects the title attribute (advisory information/tooltip)
	jsEl.DefineAccessorProperty("title", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("title"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("title", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// lang - reflects the lang attribute (language)
	jsEl.DefineAccessorProperty("lang", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("lang"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("lang", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// dir - reflects the dir attribute (text direction: ltr, rtl, auto)
	jsEl.DefineAccessorProperty("dir", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("dir"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("dir", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// hidden - boolean reflecting the hidden attribute
	jsEl.DefineAccessorProperty("hidden", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.HasAttribute("hidden"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if call.Arguments[0].ToBoolean() {
				el.SetAttribute("hidden", "")
			} else {
				el.RemoveAttribute("hidden")
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// tabIndex - reflects the tabindex attribute
	// Default is -1 for most elements, 0 for interactive elements
	jsEl.DefineAccessorProperty("tabIndex", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if el.HasAttribute("tabindex") {
			val := el.GetAttribute("tabindex")
			// Parse as integer
			var tabIndex int
			_, err := fmt.Sscanf(val, "%d", &tabIndex)
			if err == nil {
				return vm.ToValue(tabIndex)
			}
		}
		// Default tabindex based on element type
		// Interactive elements (a with href, button, input, select, textarea) default to 0
		// Others default to -1
		localName := el.LocalName()
		switch localName {
		case "a":
			if el.HasAttribute("href") {
				return vm.ToValue(0)
			}
		case "button", "input", "select", "textarea":
			return vm.ToValue(0)
		case "summary":
			return vm.ToValue(0)
		}
		return vm.ToValue(-1)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].ToInteger()
			el.SetAttribute("tabindex", fmt.Sprintf("%d", val))
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// accessKey - reflects the accesskey attribute
	jsEl.DefineAccessorProperty("accessKey", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("accesskey"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("accesskey", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// draggable - boolean-ish reflecting the draggable attribute
	// Can be true, false, or auto (default)
	jsEl.DefineAccessorProperty("draggable", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		val := el.GetAttribute("draggable")
		if val == "true" {
			return vm.ToValue(true)
		}
		// Default: img and a elements with href are draggable
		if val == "" {
			localName := el.LocalName()
			if localName == "img" {
				return vm.ToValue(true)
			}
			if localName == "a" && el.HasAttribute("href") {
				return vm.ToValue(true)
			}
		}
		return vm.ToValue(false)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if call.Arguments[0].ToBoolean() {
				el.SetAttribute("draggable", "true")
			} else {
				el.SetAttribute("draggable", "false")
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// spellcheck - boolean reflecting the spellcheck attribute
	// Default is element-dependent
	jsEl.DefineAccessorProperty("spellcheck", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		val := el.GetAttribute("spellcheck")
		if val == "false" {
			return vm.ToValue(false)
		}
		if val == "true" {
			return vm.ToValue(true)
		}
		// Default: true for editable elements
		return vm.ToValue(true)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if call.Arguments[0].ToBoolean() {
				el.SetAttribute("spellcheck", "true")
			} else {
				el.SetAttribute("spellcheck", "false")
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// translate - boolean reflecting the translate attribute
	jsEl.DefineAccessorProperty("translate", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		val := el.GetAttribute("translate")
		if val == "no" {
			return vm.ToValue(false)
		}
		// Default is true (inherit from parent or true)
		return vm.ToValue(true)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if call.Arguments[0].ToBoolean() {
				el.SetAttribute("translate", "yes")
			} else {
				el.SetAttribute("translate", "no")
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// contentEditable - reflects the contenteditable attribute
	jsEl.DefineAccessorProperty("contentEditable", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		val := el.GetAttribute("contenteditable")
		if val == "" {
			return vm.ToValue("inherit")
		}
		if val == "true" || val == "" {
			return vm.ToValue("true")
		}
		if val == "false" {
			return vm.ToValue("false")
		}
		return vm.ToValue("inherit")
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].String()
			if val == "inherit" {
				el.RemoveAttribute("contenteditable")
			} else {
				el.SetAttribute("contenteditable", val)
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// isContentEditable - returns whether element is editable (read-only)
	jsEl.DefineAccessorProperty("isContentEditable", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Walk up the tree to find if any ancestor has contenteditable="true"
		current := el
		for current != nil {
			val := current.GetAttribute("contenteditable")
			if val == "true" {
				return vm.ToValue(true)
			}
			if val == "false" {
				return vm.ToValue(false)
			}
			parent := current.AsNode().ParentElement()
			if parent == nil {
				break
			}
			current = parent
		}
		return vm.ToValue(false)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// inputMode - reflects the inputmode attribute
	jsEl.DefineAccessorProperty("inputMode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("inputmode"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("inputmode", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// enterKeyHint - reflects the enterkeyhint attribute
	jsEl.DefineAccessorProperty("enterKeyHint", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("enterkeyhint"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("enterkeyhint", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// autocapitalize - reflects the autocapitalize attribute
	jsEl.DefineAccessorProperty("autocapitalize", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("autocapitalize"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("autocapitalize", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// nonce - reflects the nonce attribute (used for CSP)
	jsEl.DefineAccessorProperty("nonce", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(el.GetAttribute("nonce"))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			el.SetAttribute("nonce", call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)
}

// BindNode creates a JavaScript object from a DOM node.
func (b *DOMBinder) BindNode(node *dom.Node) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	// Check node type and delegate to appropriate binder
	switch node.NodeType() {
	case dom.ElementNode:
		return b.BindElement((*dom.Element)(node))
	case dom.DocumentNode:
		return b.BindDocument((*dom.Document)(node))
	case dom.DocumentFragmentNode:
		return b.BindDocumentFragment((*dom.DocumentFragment)(node))
	case dom.TextNode:
		return b.BindTextNode(node, nil)
	case dom.CommentNode:
		return b.BindCommentNode(node, nil)
	case dom.CDATASectionNode:
		return b.BindCDATASectionNode(node, nil)
	case dom.ProcessingInstructionNode:
		return b.BindProcessingInstructionNode(node, nil)
	case dom.DocumentTypeNode:
		return b.BindDocumentTypeNode(node)
	}

	// For other nodes
	vm := b.runtime.vm
	jsNode := vm.NewObject()

	jsNode.Set("_goNode", node)

	jsNode.Set("nodeType", int(node.NodeType()))
	jsNode.Set("nodeName", node.NodeName())

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			node.SetNodeValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			node.SetTextContent(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	b.bindNodeProperties(jsNode, node)

	// Cache the binding
	b.nodeMap[node] = jsNode

	return jsNode
}

// BindTextNode creates a JavaScript object from a DOM text node with proper prototype.
func (b *DOMBinder) BindTextNode(node *dom.Node, proto *goja.Object) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsNode := vm.NewObject()

	// Set prototype for instanceof to work
	if proto != nil {
		jsNode.SetPrototype(proto)
	} else if b.textProto != nil {
		jsNode.SetPrototype(b.textProto)
	}

	jsNode.Set("_goNode", node)
	jsNode.Set("nodeType", int(dom.TextNode))
	jsNode.Set("nodeName", "#text")

	// CharacterData properties
	// Per spec, setting data/nodeValue/textContent to null should result in empty string
	// We use WTF-8 encoding internally to preserve unpaired surrogates.
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			// SetNodeValue handles NotifyReplaceData and notifyCharacterDataMutation internally
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			// SetNodeValue handles NotifyReplaceData and notifyCharacterDataMutation internally
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			// SetTextContent eventually calls SetNodeValue which handles notifications
			node.SetTextContent(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return length in UTF-16 code units per the CharacterData spec
		return vm.ToValue(utf16Length(node.NodeValue()))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// CharacterData mutation methods
	b.bindCharacterDataMethods(jsNode, node)

	// Text-specific methods
	// splitText(offset) - splits the Text node at offset and returns the remainder
	// Per DOM spec (https://dom.spec.whatwg.org/#dom-text-splittext):
	// 1. Create new node with data after offset
	// 2. If parent exists, insert new node
	// 3. Update live ranges (move boundaries to new node) - BEFORE truncating
	// 4. Replace data (truncate old node) - AFTER updating ranges
	jsNode.Set("splitText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'splitText' on 'Text': 1 argument required"))
		}

		offset := toUint32(call.Arguments[0])
		data := node.NodeValue()
		length := uint32(utf16Length(data))

		// Check offset bounds - per spec, throw IndexSizeError if offset > length
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Get the count of characters to extract
		count := length - offset

		// Extract the new data for the new text node
		newData := utf16Substring(data, int(offset), int(count))

		// Create a new Text node with the extracted data
		// IMPORTANT: Use the text node's owner document, not the binder's main document
		// This ensures correct behavior for nodes in foreign documents
		var newTextNode *dom.Node
		ownerDoc := node.OwnerDocument()
		if ownerDoc != nil {
			newTextNode = ownerDoc.CreateTextNode(newData)
		} else if b.document != nil {
			newTextNode = b.document.CreateTextNode(newData)
		} else {
			newTextNode = dom.NewTextNode(newData)
		}

		// Step 7a: If this node has a parent, insert the new node
		if parent := node.ParentNode(); parent != nil {
			nextSibling := node.NextSibling()
			if nextSibling != nil {
				parent.InsertBefore(newTextNode, nextSibling)
			} else {
				parent.AppendChild(newTextNode)
			}

			// Steps 7b-7e: Move range boundary points from old node to new node
			// This MUST happen BEFORE the replace data step
			dom.NotifySplitText(node, int(offset), newTextNode)
		}

		// Step 8: Replace data with node node, offset offset, count count, and data ""
		// This truncates the old node and notifies callbacks about the data change
		dom.NotifyReplaceData(node, int(offset), int(count), "")

		// Truncate the current node's data at offset without triggering another notification
		node.SetNodeValueRaw(utf16Substring(data, 0, int(offset)))

		return b.BindTextNode(newTextNode, nil)
	})

	// wholeText - returns text of all logically adjacent Text nodes
	jsNode.DefineAccessorProperty("wholeText", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Collect text from all adjacent Text nodes
		var result string

		// Go backwards to find the start
		current := node.PreviousSibling()
		var textNodes []*dom.Node
		for current != nil && current.NodeType() == dom.TextNode {
			textNodes = append([]*dom.Node{current}, textNodes...)
			current = current.PreviousSibling()
		}
		// Add this node
		textNodes = append(textNodes, node)
		// Go forwards
		current = node.NextSibling()
		for current != nil && current.NodeType() == dom.TextNode {
			textNodes = append(textNodes, current)
			current = current.NextSibling()
		}

		// Concatenate all text content
		for _, textNode := range textNodes {
			result += textNode.NodeValue()
		}
		return vm.ToValue(result)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ChildNode mixin methods
	b.bindCharacterDataChildNodeMixin(jsNode, node)

	// Common node properties
	b.bindNodeProperties(jsNode, node)

	// Cache
	b.nodeMap[node] = jsNode

	return jsNode
}

// BindCommentNode creates a JavaScript object from a DOM comment node with proper prototype.
func (b *DOMBinder) BindCommentNode(node *dom.Node, proto *goja.Object) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsNode := vm.NewObject()

	// Set prototype for instanceof to work
	if proto != nil {
		jsNode.SetPrototype(proto)
	} else if b.commentProto != nil {
		jsNode.SetPrototype(b.commentProto)
	}

	jsNode.Set("_goNode", node)
	jsNode.Set("nodeType", int(dom.CommentNode))
	jsNode.Set("nodeName", "#comment")

	// CharacterData properties
	// Per spec, setting data/nodeValue/textContent to null should result in empty string
	// We use WTF-8 encoding internally to preserve unpaired surrogates.
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			dom.NotifyReplaceData(node, 0, oldLength, val)
			node.SetTextContent(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return length in UTF-16 code units per the CharacterData spec
		return vm.ToValue(utf16Length(node.NodeValue()))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// CharacterData mutation methods
	b.bindCharacterDataMethods(jsNode, node)

	// ChildNode mixin methods
	b.bindCharacterDataChildNodeMixin(jsNode, node)

	// Common node properties
	b.bindNodeProperties(jsNode, node)

	// Cache
	b.nodeMap[node] = jsNode

	return jsNode
}

// BindCDATASectionNode creates a JavaScript object from a DOM CDATASection node with proper prototype.
func (b *DOMBinder) BindCDATASectionNode(node *dom.Node, proto *goja.Object) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsNode := vm.NewObject()

	// Set prototype for instanceof to work
	if proto != nil {
		jsNode.SetPrototype(proto)
	} else if b.cdataSectionProto != nil {
		jsNode.SetPrototype(b.cdataSectionProto)
	}

	jsNode.Set("_goNode", node)
	jsNode.Set("nodeType", int(dom.CDATASectionNode))
	jsNode.Set("nodeName", "#cdata-section")

	// CharacterData properties
	// Per spec, setting data/nodeValue/textContent to null should result in empty string
	// We use WTF-8 encoding internally to preserve unpaired surrogates.
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			dom.NotifyReplaceData(node, 0, oldLength, val)
			node.SetTextContent(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return length in UTF-16 code units per the CharacterData spec
		return vm.ToValue(utf16Length(node.NodeValue()))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// CharacterData mutation methods
	b.bindCharacterDataMethods(jsNode, node)

	// ChildNode mixin methods
	b.bindCharacterDataChildNodeMixin(jsNode, node)

	// Common node properties
	b.bindNodeProperties(jsNode, node)

	// Cache
	b.nodeMap[node] = jsNode

	return jsNode
}

// BindProcessingInstructionNode creates a JavaScript object from a DOM ProcessingInstruction node.
func (b *DOMBinder) BindProcessingInstructionNode(node *dom.Node, proto *goja.Object) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsNode := vm.NewObject()

	// Set prototype for instanceof to work
	if proto != nil {
		jsNode.SetPrototype(proto)
	} else if b.processingInstructionProto != nil {
		jsNode.SetPrototype(b.processingInstructionProto)
	}

	jsNode.Set("_goNode", node)
	jsNode.Set("nodeType", int(dom.ProcessingInstructionNode))

	// nodeName is the target for ProcessingInstruction
	jsNode.DefineAccessorProperty("nodeName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// target property (read-only) - same as nodeName for ProcessingInstruction
	jsNode.DefineAccessorProperty("target", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// CharacterData properties
	// Per spec, setting data/nodeValue/textContent to null should result in empty string
	// We use WTF-8 encoding internally to preserve unpaired surrogates.
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, stringToUTF16(node.NodeValue()))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			var newValue string
			if goja.IsNull(call.Arguments[0]) {
				newValue = ""
			} else {
				units := gojaValueToUTF16(call.Arguments[0])
				newValue = utf16ToString(units)
			}
			dom.NotifyReplaceData(node, 0, oldLength, newValue)
			node.SetNodeValue(newValue)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			oldUnits := stringToUTF16(node.NodeValue())
			oldLength := len(oldUnits)
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			dom.NotifyReplaceData(node, 0, oldLength, val)
			node.SetTextContent(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Return length in UTF-16 code units per the CharacterData spec
		return vm.ToValue(utf16Length(node.NodeValue()))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// CharacterData mutation methods
	b.bindCharacterDataMethods(jsNode, node)

	// ChildNode mixin methods
	b.bindCharacterDataChildNodeMixin(jsNode, node)

	// Common node properties
	b.bindNodeProperties(jsNode, node)

	// Cache
	b.nodeMap[node] = jsNode

	return jsNode
}

// BindDocumentTypeNode creates a JavaScript object from a DOM DocumentType node.
// DocumentType nodes have specific properties (name, publicId, systemId) and implement ChildNode mixin.
func (b *DOMBinder) BindDocumentTypeNode(node *dom.Node) *goja.Object {
	if node == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsNode := vm.NewObject()

	// Set prototype (use DocumentType prototype)
	if b.documentTypeProto != nil {
		jsNode.SetPrototype(b.documentTypeProto)
	}

	jsNode.Set("_goNode", node)
	jsNode.Set("nodeType", int(dom.DocumentTypeNode))

	// DocumentType-specific properties (all read-only)
	// nodeName is the "name" for DocumentType
	jsNode.DefineAccessorProperty("nodeName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.DoctypeName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// name property
	jsNode.DefineAccessorProperty("name", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.DoctypeName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// publicId property
	jsNode.DefineAccessorProperty("publicId", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.DoctypePublicId())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// systemId property
	jsNode.DefineAccessorProperty("systemId", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.DoctypeSystemId())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// nodeValue is null for DocumentType (per spec)
	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Setting nodeValue has no effect on DocumentType nodes
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// textContent is null for DocumentType (per spec)
	jsNode.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Setting textContent has no effect on DocumentType nodes
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ChildNode mixin methods (remove, before, after, replaceWith)
	b.bindCharacterDataChildNodeMixin(jsNode, node)

	// Common node properties
	b.bindNodeProperties(jsNode, node)

	// Cache
	b.nodeMap[node] = jsNode

	return jsNode
}

// createDOMException creates a proper DOMException object using the global constructor.
func (b *DOMBinder) createDOMException(name, message string) *goja.Object {
	vm := b.runtime.vm

	// Use the DOMException constructor that was set up in setupPrototypes
	// This ensures instanceof DOMException works correctly
	domExceptionCtor := vm.Get("DOMException")
	if domExceptionCtor == nil || goja.IsUndefined(domExceptionCtor) {
		// Fallback: create a basic object
		exc := vm.NewObject()
		exc.Set("name", name)
		exc.Set("message", message)
		exc.Set("code", domExceptionCode(name))
		return exc
	}

	// Call: new DOMException(message, name)
	ctor, ok := goja.AssertConstructor(domExceptionCtor)
	if !ok {
		// Fallback
		exc := vm.NewObject()
		exc.Set("name", name)
		exc.Set("message", message)
		exc.Set("code", domExceptionCode(name))
		return exc
	}

	exc, err := ctor(nil, vm.ToValue(message), vm.ToValue(name))
	if err != nil {
		// Fallback
		fallback := vm.NewObject()
		fallback.Set("name", name)
		fallback.Set("message", message)
		fallback.Set("code", domExceptionCode(name))
		return fallback
	}
	return exc
}

// throwIndexSizeError throws a DOMException with name "IndexSizeError".
func (b *DOMBinder) throwIndexSizeError(vm *goja.Runtime) {
	exc := b.createDOMException("IndexSizeError", "The index is not in the allowed range.")
	panic(vm.ToValue(exc))
}

// createEventForInterface creates an event for the document.createEvent() legacy API.
// The interface name is case-insensitive. Returns an event with the correct prototype.
func (b *DOMBinder) createEventForInterface(interfaceName string) *goja.Object {
	vm := b.runtime.vm

	// Map of interface names (lowercase) to their target interface
	// Per DOM spec, document.createEvent() uses a specific mapping
	interfaceMap := map[string]string{
		// Event and aliases
		"event":       "Event",
		"events":      "Event",
		"htmlevents":  "Event",
		"svgevents":   "Event",
		// UIEvent and aliases
		"uievent":  "UIEvent",
		"uievents": "UIEvent",
		// MouseEvent and aliases
		"mouseevent":  "MouseEvent",
		"mouseevents": "MouseEvent",
		// FocusEvent
		"focusevent": "FocusEvent",
		// KeyboardEvent
		"keyboardevent": "KeyboardEvent",
		// CompositionEvent
		"compositionevent": "CompositionEvent",
		// TextEvent
		"textevent": "TextEvent",
		// CustomEvent
		"customevent": "CustomEvent",
		// MessageEvent
		"messageevent": "MessageEvent",
		// StorageEvent
		"storageevent": "StorageEvent",
		// HashChangeEvent
		"hashchangeevent": "HashChangeEvent",
		// BeforeUnloadEvent
		"beforeunloadevent": "BeforeUnloadEvent",
		// DeviceMotionEvent
		"devicemotionevent": "DeviceMotionEvent",
		// DeviceOrientationEvent
		"deviceorientationevent": "DeviceOrientationEvent",
		// DragEvent
		"dragevent": "DragEvent",
		// TouchEvent - optional, depends on platform
		"touchevent": "TouchEvent",
	}

	// Lookup case-insensitively using ASCII lowercase only
	// (Unicode characters like Turkish I should not match)
	lowerName := asciiLowercase(interfaceName)
	targetInterface, ok := interfaceMap[lowerName]
	if !ok {
		// Throw NotSupportedError for unknown interface
		exc := b.createDOMException("NotSupportedError",
			"The provided event type ('"+interfaceName+"') is invalid.")
		panic(vm.ToValue(exc))
	}

	// Get the constructor from the global object
	ctor := vm.Get(targetInterface)
	if ctor == nil || goja.IsUndefined(ctor) {
		// If the constructor doesn't exist, create a basic Event
		ctor = vm.Get("Event")
		if ctor == nil || goja.IsUndefined(ctor) {
			// Last resort: create a plain object
			event := vm.NewObject()
			b.initEventObject(event)
			return event
		}
	}

	// Try to call as constructor with empty type
	constructor, ok := goja.AssertConstructor(ctor)
	if !ok {
		// Fallback: create an event with correct prototype
		event := vm.NewObject()
		proto := ctor.ToObject(vm).Get("prototype")
		if proto != nil && !goja.IsUndefined(proto) {
			event.SetPrototype(proto.ToObject(vm))
		}
		b.initEventObject(event)
		return event
	}

	// Call constructor with empty string for type (per spec)
	event, err := constructor(nil, vm.ToValue(""))
	if err != nil {
		// Fallback
		event := vm.NewObject()
		b.initEventObject(event)
		return event
	}

	// Per DOM spec: Events created via createEvent() must NOT be initialized.
	// The initEvent() method must be called before dispatch.
	event.Set("_initialized", false)

	return event
}

// initEventObject initializes a plain event object with required properties.
// Events created via createEvent() start with _initialized=false and must have
// initEvent() called before they can be dispatched.
func (b *DOMBinder) initEventObject(event *goja.Object) {
	vm := b.runtime.vm
	event.Set("type", "")
	event.Set("target", goja.Null())
	event.Set("currentTarget", goja.Null())
	event.Set("eventPhase", 0)
	event.Set("bubbles", false)
	event.Set("cancelable", false)
	event.Set("defaultPrevented", false)
	event.Set("composed", false)
	event.Set("isTrusted", false)
	event.Set("timeStamp", float64(0))

	// Internal flags for event dispatch
	event.Set("_initialized", false)
	event.Set("_dispatch", false)
	event.Set("_stopPropagation", false)
	event.Set("_stopImmediate", false)

	// Methods
	event.Set("preventDefault", func(call goja.FunctionCall) goja.Value {
		if event.Get("cancelable").ToBoolean() {
			event.Set("defaultPrevented", true)
		}
		return goja.Undefined()
	})

	event.Set("stopPropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		return goja.Undefined()
	})

	event.Set("stopImmediatePropagation", func(call goja.FunctionCall) goja.Value {
		event.Set("_stopPropagation", true)
		event.Set("_stopImmediate", true)
		return goja.Undefined()
	})

	event.Set("composedPath", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue([]interface{}{})
	})

	// cancelBubble - legacy property that maps to stop propagation flag
	// Per DOM spec: getter returns stop propagation flag, setter can only set it to true
	event.DefineAccessorProperty("cancelBubble",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			stopProp := event.Get("_stopPropagation")
			if stopProp != nil {
				return vm.ToValue(stopProp.ToBoolean())
			}
			return vm.ToValue(false)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && call.Arguments[0].ToBoolean() {
				event.Set("_stopPropagation", true)
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// returnValue - legacy property that is the inverse of defaultPrevented
	// Per DOM spec: getter returns !defaultPrevented, setter calls preventDefault() when set to false
	event.DefineAccessorProperty("returnValue",
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			defaultPrevented := event.Get("defaultPrevented")
			if defaultPrevented != nil {
				return vm.ToValue(!defaultPrevented.ToBoolean())
			}
			return vm.ToValue(true)
		}),
		vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && !call.Arguments[0].ToBoolean() {
				// Setting returnValue to false is equivalent to calling preventDefault()
				if event.Get("cancelable").ToBoolean() {
					event.Set("defaultPrevented", true)
				}
			}
			return goja.Undefined()
		}),
		goja.FLAG_FALSE, goja.FLAG_TRUE)

	// initEvent(type, bubbles, cancelable) - legacy method to initialize an event
	// Per DOM spec, this must be called on events created via createEvent() before dispatch
	event.Set("initEvent", func(call goja.FunctionCall) goja.Value {
		// Get type argument (required, but defaults to empty string if missing)
		eventType := ""
		if len(call.Arguments) > 0 {
			eventType = call.Arguments[0].String()
		}

		// Get bubbles argument (optional, defaults to false)
		bubbles := false
		if len(call.Arguments) > 1 {
			bubbles = call.Arguments[1].ToBoolean()
		}

		// Get cancelable argument (optional, defaults to false)
		cancelable := false
		if len(call.Arguments) > 2 {
			cancelable = call.Arguments[2].ToBoolean()
		}

		// Per DOM spec: If event's dispatch flag is set, terminate these steps
		dispatchFlag := event.Get("_dispatch")
		if dispatchFlag != nil && dispatchFlag.ToBoolean() {
			return goja.Undefined()
		}

		// Set the initialized flag
		event.Set("_initialized", true)

		// Clear the stop propagation flag
		event.Set("_stopPropagation", false)
		event.Set("_stopImmediate", false)

		// Reset default prevented
		event.Set("defaultPrevented", false)

		// Set the event properties
		event.Set("type", eventType)
		event.Set("bubbles", bubbles)
		event.Set("cancelable", cancelable)

		return goja.Undefined()
	})

	// Constants
	event.Set("NONE", 0)
	event.Set("CAPTURING_PHASE", 1)
	event.Set("AT_TARGET", 2)
	event.Set("BUBBLING_PHASE", 3)
}

// throwDOMError throws a DOMException from a dom.DOMError.
func (b *DOMBinder) throwDOMError(vm *goja.Runtime, err *dom.DOMError) {
	exc := b.createDOMException(err.Name, err.Message)
	panic(vm.ToValue(exc))
}

// throwHierarchyRequestError throws a DOMException with name "HierarchyRequestError".
func (b *DOMBinder) throwHierarchyRequestError(vm *goja.Runtime, message string) {
	exc := b.createDOMException("HierarchyRequestError", message)
	panic(vm.ToValue(exc))
}

// throwNotFoundError throws a DOMException with name "NotFoundError".
func (b *DOMBinder) throwNotFoundError(vm *goja.Runtime, message string) {
	exc := b.createDOMException("NotFoundError", message)
	panic(vm.ToValue(exc))
}

// bindCharacterDataMethods adds the CharacterData mutation methods to a node.
// These are: substringData, appendData, insertData, deleteData, replaceData
// All methods properly handle UTF-16 surrogate pairs, preserving unpaired surrogates.
func (b *DOMBinder) bindCharacterDataMethods(jsNode *goja.Object, node *dom.Node) {
	vm := b.runtime.vm

	jsNode.Set("substringData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'substringData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])

		// Get data as UTF-16 code units
		dataUnits := stringToUTF16(node.NodeValue())
		length := uint32(len(dataUnits))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Extract substring as UTF-16 code units
		end := int(offset) + int(count)
		if end > len(dataUnits) {
			end = len(dataUnits)
		}
		resultUnits := dataUnits[offset:end]

		// Return as goja String to preserve surrogates
		return utf16ToGojaValue(vm, resultUnits)
	})

	jsNode.Set("appendData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 1 argument
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'appendData' on 'CharacterData': 1 argument required"))
		}

		// Get data to append as UTF-16 code units
		appendUnits := gojaValueToUTF16(call.Arguments[0])
		// Get current data as UTF-16 code units
		currentUnits := stringToUTF16(node.NodeValue())

		// Notify ReplaceData mutation: appendData is equivalent to insertData(length, data)
		offset := len(currentUnits)
		dom.NotifyReplaceData(node, offset, 0, utf16ToString(appendUnits))

		// Concatenate
		newUnits := append(currentUnits, appendUnits...)
		// Store as WTF-8 without triggering another notification
		node.SetNodeValueRaw(utf16ToString(newUnits))
		return goja.Undefined()
	})

	jsNode.Set("insertData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'insertData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		insertUnits := gojaValueToUTF16(call.Arguments[1])

		currentUnits := stringToUTF16(node.NodeValue())
		length := uint32(len(currentUnits))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Notify ReplaceData mutation: insertData is equivalent to replaceData(offset, 0, data)
		dom.NotifyReplaceData(node, int(offset), 0, utf16ToString(insertUnits))

		// Build result: before + insert + after
		newUnits := make([]uint16, 0, len(currentUnits)+len(insertUnits))
		newUnits = append(newUnits, currentUnits[:offset]...)
		newUnits = append(newUnits, insertUnits...)
		newUnits = append(newUnits, currentUnits[offset:]...)

		// Set value without triggering another notification
		node.SetNodeValueRaw(utf16ToString(newUnits))
		return goja.Undefined()
	})

	jsNode.Set("deleteData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'deleteData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])

		currentUnits := stringToUTF16(node.NodeValue())
		length := uint32(len(currentUnits))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Calculate end position (clamped to length)
		end := int(offset) + int(count)
		if end > len(currentUnits) {
			end = len(currentUnits)
		}

		// Calculate actual deletion size (end - offset, which is clamped)
		actualDeleteCount := end - int(offset)

		// Notify ReplaceData mutation: deleteData is equivalent to replaceData(offset, count, "")
		dom.NotifyReplaceData(node, int(offset), actualDeleteCount, "")

		// Build result: before + after
		newUnits := make([]uint16, 0, len(currentUnits)-actualDeleteCount)
		newUnits = append(newUnits, currentUnits[:offset]...)
		newUnits = append(newUnits, currentUnits[end:]...)

		// Set value without triggering another notification
		node.SetNodeValueRaw(utf16ToString(newUnits))
		return goja.Undefined()
	})

	jsNode.Set("replaceData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 3 arguments
		if len(call.Arguments) < 3 {
			panic(vm.NewTypeError("Failed to execute 'replaceData' on 'CharacterData': 3 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])
		replaceUnits := gojaValueToUTF16(call.Arguments[2])

		currentUnits := stringToUTF16(node.NodeValue())
		length := uint32(len(currentUnits))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Calculate end position (clamped to length)
		end := int(offset) + int(count)
		if end > len(currentUnits) {
			end = len(currentUnits)
		}

		// Calculate actual deletion size (end - offset, which is clamped)
		actualDeleteCount := end - int(offset)

		// Notify ReplaceData mutation
		dom.NotifyReplaceData(node, int(offset), actualDeleteCount, utf16ToString(replaceUnits))

		// Build result: before + replacement + after
		newUnits := make([]uint16, 0, len(currentUnits)-actualDeleteCount+len(replaceUnits))
		newUnits = append(newUnits, currentUnits[:offset]...)
		newUnits = append(newUnits, replaceUnits...)
		newUnits = append(newUnits, currentUnits[end:]...)

		// Set value without triggering another notification
		node.SetNodeValueRaw(utf16ToString(newUnits))
		return goja.Undefined()
	})
}

// bindCharacterDataChildNodeMixin adds ChildNode mixin methods to a CharacterData node.
// Implements the ChildNode mixin from the DOM spec.
func (b *DOMBinder) bindCharacterDataChildNodeMixin(jsNode *goja.Object, node *dom.Node) {
	jsNode.Set("before", func(call goja.FunctionCall) goja.Value {
		parent := node.ParentNode()
		if parent == nil {
			return goja.Undefined()
		}
		items := b.convertJSNodesToGo(call.Arguments)

		// Find viable previous sibling (first preceding sibling not in nodes)
		nodeSet := b.extractNodeSet(items)
		var viablePrevSibling *dom.Node
		for sibling := node.PreviousSibling(); sibling != nil; sibling = sibling.PreviousSibling() {
			if !nodeSet[sibling] {
				viablePrevSibling = sibling
				break
			}
		}

		// Convert items to a node (or DocumentFragment if multiple)
		newNode := b.convertItemsToNode(node, items)
		if newNode == nil {
			return goja.Undefined()
		}

		// Insert before this node (after viable previous sibling)
		var refNode *dom.Node
		if viablePrevSibling == nil {
			refNode = parent.FirstChild()
		} else {
			refNode = viablePrevSibling.NextSibling()
		}
		parent.InsertBefore(newNode, refNode)
		return goja.Undefined()
	})

	jsNode.Set("after", func(call goja.FunctionCall) goja.Value {
		parent := node.ParentNode()
		if parent == nil {
			return goja.Undefined()
		}
		items := b.convertJSNodesToGo(call.Arguments)

		// Find viable next sibling (first following sibling not in nodes)
		nodeSet := b.extractNodeSet(items)
		var viableNextSibling *dom.Node
		for sibling := node.NextSibling(); sibling != nil; sibling = sibling.NextSibling() {
			if !nodeSet[sibling] {
				viableNextSibling = sibling
				break
			}
		}

		// Convert items to a node (or DocumentFragment if multiple)
		newNode := b.convertItemsToNode(node, items)
		if newNode == nil {
			return goja.Undefined()
		}

		// Insert before viable next sibling
		parent.InsertBefore(newNode, viableNextSibling)
		return goja.Undefined()
	})

	jsNode.Set("replaceWith", func(call goja.FunctionCall) goja.Value {
		parent := node.ParentNode()
		if parent == nil {
			return goja.Undefined()
		}
		items := b.convertJSNodesToGo(call.Arguments)

		// Find viable next sibling (first following sibling not in nodes)
		nodeSet := b.extractNodeSet(items)
		var viableNextSibling *dom.Node
		for sibling := node.NextSibling(); sibling != nil; sibling = sibling.NextSibling() {
			if !nodeSet[sibling] {
				viableNextSibling = sibling
				break
			}
		}

		// Convert items to a node (or DocumentFragment if multiple)
		newNode := b.convertItemsToNode(node, items)

		// If this node's parent is still parent, replace; otherwise insert
		if node.ParentNode() == parent {
			if newNode != nil {
				parent.ReplaceChild(newNode, node)
			} else {
				// No replacement nodes, just remove this node
				parent.RemoveChild(node)
			}
		} else if newNode != nil {
			parent.InsertBefore(newNode, viableNextSibling)
		}
		return goja.Undefined()
	})

	jsNode.Set("remove", func(call goja.FunctionCall) goja.Value {
		if node.ParentNode() != nil {
			node.ParentNode().RemoveChild(node)
		}
		return goja.Undefined()
	})
}

// extractNodeSet builds a set of DOM nodes from the items slice.
func (b *DOMBinder) extractNodeSet(items []interface{}) map[*dom.Node]bool {
	result := make(map[*dom.Node]bool)
	for _, item := range items {
		if n, ok := item.(*dom.Node); ok {
			result[n] = true
		}
	}
	return result
}

// convertItemsToNode converts items (nodes/strings) to a single node or DocumentFragment.
func (b *DOMBinder) convertItemsToNode(contextNode *dom.Node, items []interface{}) *dom.Node {
	doc := contextNode.OwnerDocument()
	if doc == nil {
		return nil
	}

	nodes := make([]*dom.Node, 0, len(items))
	for _, item := range items {
		var n *dom.Node
		switch v := item.(type) {
		case *dom.Node:
			n = v
		case string:
			n = doc.CreateTextNode(v)
		}
		if n != nil {
			nodes = append(nodes, n)
		}
	}

	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	// Create a DocumentFragment and append all nodes
	frag := doc.CreateDocumentFragment()
	fragNode := (*dom.Node)(frag)
	for _, n := range nodes {
		fragNode.AppendChild(n)
	}
	return fragNode
}

// BindDocumentFragment creates a JavaScript object from a DOM document fragment.
func (b *DOMBinder) BindDocumentFragment(frag *dom.DocumentFragment) *goja.Object {
	if frag == nil {
		return nil
	}

	node := (*dom.Node)(frag)

	// Check cache
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsFrag := vm.NewObject()

	// Set prototype for instanceof to work
	if b.documentFragmentProto != nil {
		jsFrag.SetPrototype(b.documentFragmentProto)
	}

	jsFrag.Set("_goNode", node)
	jsFrag.Set("_goFragment", frag)

	jsFrag.Set("nodeType", int(dom.DocumentFragmentNode))
	jsFrag.Set("nodeName", "#document-fragment")

	// Query methods
	jsFrag.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		// Search through children
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if child.NodeType() == dom.ElementNode {
				el := (*dom.Element)(child)
				if el.Matches(selector) {
					return b.BindElement(el)
				}
				found := el.QuerySelector(selector)
				if found != nil {
					return b.BindElement(found)
				}
			}
		}
		return goja.Null()
	})

	jsFrag.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		var results []*dom.Node
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if child.NodeType() == dom.ElementNode {
				el := (*dom.Element)(child)
				if el.Matches(selector) {
					results = append(results, child)
				}
				nodeList := el.QuerySelectorAll(selector)
				for i := 0; i < nodeList.Length(); i++ {
					results = append(results, nodeList.Item(i))
				}
			}
		}
		return b.bindStaticNodeList(results)
	})

	// ParentNode mixin properties
	jsFrag.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindHTMLCollection(frag.Children())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsFrag.DefineAccessorProperty("childElementCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(frag.ChildElementCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsFrag.DefineAccessorProperty("firstElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := frag.FirstElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsFrag.DefineAccessorProperty("lastElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := frag.LastElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin methods
	jsFrag.Set("append", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		frag.Append(nodes...)
		return goja.Undefined()
	})

	jsFrag.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		frag.Prepend(nodes...)
		return goja.Undefined()
	})

	jsFrag.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		if err := frag.ReplaceChildrenWithError(nodes...); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	// moveBefore - state-preserving atomic move API
	jsFrag.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'DocumentFragment': parameter 1 is not of type 'Node'."))
		}

		movedObj := movedNode.ToObject(vm)
		goMovedNode := b.getGoNode(movedObj)
		if goMovedNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'DocumentFragment': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL
		if referenceNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'DocumentFragment': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
			refChildObj := referenceNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'DocumentFragment': parameter 2 is not of type 'Node'."))
			}
		}

		err := node.MoveBefore(goMovedNode, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Undefined()
		}
		return goja.Undefined()
	})

	// getElementById is now on the prototype (setupDocumentFragmentPrototypeMethods)
	// but we keep an instance method for efficiency since it can directly access frag

	// textContent - like Element, returns concatenated text and allows setting
	jsFrag.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			arg := call.Arguments[0]
			// Per spec, null and undefined are treated as empty string
			var value string
			if goja.IsNull(arg) || goja.IsUndefined(arg) {
				value = ""
			} else {
				value = arg.String()
			}
			node.SetTextContent(value)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	b.bindNodeProperties(jsFrag, node)

	// Cache the binding
	b.nodeMap[node] = jsFrag

	return jsFrag
}

// BindShadowRoot creates a JavaScript object from a DOM ShadowRoot.
func (b *DOMBinder) BindShadowRoot(sr *dom.ShadowRoot) *goja.Object {
	if sr == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.shadowRootCache[sr]; ok {
		return jsObj
	}

	node := sr.AsNode()

	// Also check node map (shouldn't be there, but just in case)
	if jsObj, ok := b.nodeMap[node]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsSR := vm.NewObject()

	// Set prototype for instanceof to work
	if b.shadowRootProto != nil {
		jsSR.SetPrototype(b.shadowRootProto)
	}

	jsSR.Set("_goNode", node)
	jsSR.Set("_goShadowRoot", sr)

	jsSR.Set("nodeType", int(dom.DocumentFragmentNode))
	jsSR.Set("nodeName", "#document-fragment")

	// ShadowRoot-specific properties
	jsSR.DefineAccessorProperty("mode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(string(sr.Mode()))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("host", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		host := sr.Host()
		if host == nil {
			return goja.Null()
		}
		return b.BindElement(host)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("delegatesFocus", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.DelegatesFocus())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("slotAssignment", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.SlotAssignment())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("clonable", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.Clonable())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("serializable", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.Serializable())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// innerHTML
	jsSR.DefineAccessorProperty("innerHTML", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.InnerHTML())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sr.SetInnerHTML(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Query methods
	jsSR.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		el := sr.QuerySelector(selector)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsSR.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		nodeList := sr.QuerySelectorAll(selector)
		return b.BindNodeList(nodeList)
	})

	jsSR.Set("getElementById", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		id := call.Arguments[0].String()
		el := sr.GetElementById(id)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// ParentNode mixin properties
	jsSR.DefineAccessorProperty("children", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindHTMLCollection(sr.Children())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("childElementCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sr.ChildElementCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("firstElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := sr.FirstElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSR.DefineAccessorProperty("lastElementChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := sr.LastElementChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindElement(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ParentNode mixin methods
	jsSR.Set("append", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		sr.Append(nodes...)
		return goja.Undefined()
	})

	jsSR.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		sr.Prepend(nodes...)
		return goja.Undefined()
	})

	jsSR.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		sr.ReplaceChildren(nodes...)
		return goja.Undefined()
	})

	// moveBefore - state-preserving atomic move API
	jsSR.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'ShadowRoot': parameter 1 is not of type 'Node'."))
		}

		movedObj := movedNode.ToObject(vm)
		goMovedNode := b.getGoNode(movedObj)
		if goMovedNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'ShadowRoot': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL
		if referenceNode == nil {
			panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'ShadowRoot': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
			refChildObj := referenceNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'ShadowRoot': parameter 2 is not of type 'Node'."))
			}
		}

		err := node.MoveBefore(goMovedNode, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Undefined()
		}
		return goja.Undefined()
	})

	// textContent
	jsSR.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.TextContent())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			arg := call.Arguments[0]
			var value string
			if goja.IsNull(arg) || goja.IsUndefined(arg) {
				value = ""
			} else {
				value = arg.String()
			}
			node.SetTextContent(value)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	b.bindNodeProperties(jsSR, node)

	// Cache the binding
	b.shadowRootCache[sr] = jsSR
	b.nodeMap[node] = jsSR

	return jsSR
}

// BindRange creates a JavaScript object from a DOM Range.
func (b *DOMBinder) BindRange(r *dom.Range) *goja.Object {
	if r == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.rangeCache[r]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsRange := vm.NewObject()

	// Set prototype for instanceof to work
	if b.rangeProto != nil {
		jsRange.SetPrototype(b.rangeProto)
	}

	jsRange.Set("_goRange", r)

	// Range constants
	jsRange.Set("START_TO_START", dom.StartToStart)
	jsRange.Set("START_TO_END", dom.StartToEnd)
	jsRange.Set("END_TO_END", dom.EndToEnd)
	jsRange.Set("END_TO_START", dom.EndToStart)

	// Accessor properties
	jsRange.DefineAccessorProperty("startContainer", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		container := r.StartContainer()
		if container == nil {
			return goja.Null()
		}
		return b.BindNode(container)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRange.DefineAccessorProperty("startOffset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(r.StartOffset())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRange.DefineAccessorProperty("endContainer", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		container := r.EndContainer()
		if container == nil {
			return goja.Null()
		}
		return b.BindNode(container)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRange.DefineAccessorProperty("endOffset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(r.EndOffset())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRange.DefineAccessorProperty("collapsed", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(r.Collapsed())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRange.DefineAccessorProperty("commonAncestorContainer", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		container := r.CommonAncestorContainer()
		if container == nil {
			return goja.Null()
		}
		return b.BindNode(container)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Methods
	jsRange.Set("setStart", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'setStart' on 'Range': 2 arguments required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setStart' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setStart' on 'Range': parameter 1 is not of type 'Node'."))
		}
		offset := int(call.Arguments[1].ToInteger())
		if err := r.SetStart(node, offset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("setEnd", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'setEnd' on 'Range': 2 arguments required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setEnd' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setEnd' on 'Range': parameter 1 is not of type 'Node'."))
		}
		offset := int(call.Arguments[1].ToInteger())
		if err := r.SetEnd(node, offset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("setStartBefore", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'setStartBefore' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setStartBefore' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setStartBefore' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SetStartBefore(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("setStartAfter", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'setStartAfter' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setStartAfter' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setStartAfter' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SetStartAfter(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("setEndBefore", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'setEndBefore' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setEndBefore' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setEndBefore' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SetEndBefore(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("setEndAfter", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'setEndAfter' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setEndAfter' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'setEndAfter' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SetEndAfter(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("collapse", func(call goja.FunctionCall) goja.Value {
		toStart := false
		if len(call.Arguments) > 0 {
			toStart = call.Arguments[0].ToBoolean()
		}
		r.Collapse(toStart)
		return goja.Undefined()
	})

	jsRange.Set("selectNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'selectNode' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'selectNode' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'selectNode' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SelectNode(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("selectNodeContents", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'selectNodeContents' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'selectNodeContents' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'selectNodeContents' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SelectNodeContents(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("compareBoundaryPoints", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'compareBoundaryPoints' on 'Range': 2 arguments required."))
		}
		how := int(call.Arguments[0].ToInteger())
		rangeArg := call.Arguments[1]
		if goja.IsNull(rangeArg) || goja.IsUndefined(rangeArg) {
			panic(vm.NewTypeError("Failed to execute 'compareBoundaryPoints' on 'Range': parameter 2 is not of type 'Range'."))
		}
		rangeObj := rangeArg.ToObject(vm)
		sourceRange := b.getGoRange(rangeObj)
		if sourceRange == nil {
			panic(vm.NewTypeError("Failed to execute 'compareBoundaryPoints' on 'Range': parameter 2 is not of type 'Range'."))
		}
		result, err := r.CompareBoundaryPoints(how, sourceRange)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("WrongDocumentError", err.Error()))
		}
		return vm.ToValue(result)
	})

	jsRange.Set("deleteContents", func(call goja.FunctionCall) goja.Value {
		if err := r.DeleteContents(); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("extractContents", func(call goja.FunctionCall) goja.Value {
		frag, err := r.ExtractContents()
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return b.BindDocumentFragment(frag)
	})

	jsRange.Set("cloneContents", func(call goja.FunctionCall) goja.Value {
		frag, err := r.CloneContents()
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return b.BindDocumentFragment(frag)
	})

	jsRange.Set("insertNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'insertNode' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'insertNode' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'insertNode' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.InsertNode(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("surroundContents", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'surroundContents' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'surroundContents' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'surroundContents' on 'Range': parameter 1 is not of type 'Node'."))
		}
		if err := r.SurroundContents(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidStateError", err.Error()))
		}
		return goja.Undefined()
	})

	jsRange.Set("cloneRange", func(call goja.FunctionCall) goja.Value {
		clone := r.CloneRange()
		return b.BindRange(clone)
	})

	jsRange.Set("detach", func(call goja.FunctionCall) goja.Value {
		r.Detach()
		return goja.Undefined()
	})

	jsRange.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(r.ToString())
	})

	jsRange.Set("createContextualFragment", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'createContextualFragment' on 'Range': 1 argument required."))
		}
		fragment := call.Arguments[0].String()
		frag, err := r.CreateContextualFragment(fragment)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("SyntaxError", err.Error()))
		}
		return b.BindDocumentFragment(frag)
	})

	jsRange.Set("isPointInRange", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'isPointInRange' on 'Range': 2 arguments required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			return vm.ToValue(false)
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			return vm.ToValue(false)
		}
		offset := int(call.Arguments[1].ToInteger())
		return vm.ToValue(r.IsPointInRange(node, offset))
	})

	jsRange.Set("comparePoint", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'comparePoint' on 'Range': 2 arguments required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'comparePoint' on 'Range': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'comparePoint' on 'Range': parameter 1 is not of type 'Node'."))
		}
		offset := int(call.Arguments[1].ToInteger())
		result, err := r.ComparePoint(node, offset)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("WrongDocumentError", err.Error()))
		}
		return vm.ToValue(result)
	})

	jsRange.Set("intersectsNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'intersectsNode' on 'Range': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			return vm.ToValue(false)
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(r.IntersectsNode(node))
	})

	// Cache the binding
	b.rangeCache[r] = jsRange

	return jsRange
}

// BindSelection creates a JavaScript object from a DOM Selection.
func (b *DOMBinder) BindSelection(s *dom.Selection) *goja.Object {
	if s == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.selectionCache[s]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsSelection := vm.NewObject()

	// Set prototype for instanceof to work
	if b.selectionProto != nil {
		jsSelection.SetPrototype(b.selectionProto)
	}

	jsSelection.Set("_goSelection", s)

	// Read-only accessor properties
	jsSelection.DefineAccessorProperty("anchorNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		node := s.AnchorNode()
		if node == nil {
			return goja.Null()
		}
		return b.BindNode(node)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("anchorOffset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.AnchorOffset())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("focusNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		node := s.FocusNode()
		if node == nil {
			return goja.Null()
		}
		return b.BindNode(node)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("focusOffset", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.FocusOffset())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("isCollapsed", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.IsCollapsed())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("rangeCount", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.RangeCount())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("type", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.Type())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsSelection.DefineAccessorProperty("direction", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.Direction())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Methods
	jsSelection.Set("getRangeAt", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'getRangeAt' on 'Selection': 1 argument required."))
		}
		index := int(call.Arguments[0].ToInteger())
		r, err := s.GetRangeAt(index)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return b.BindRange(r)
	})

	jsSelection.Set("addRange", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'addRange' on 'Selection': 1 argument required."))
		}
		rangeArg := call.Arguments[0]
		if goja.IsNull(rangeArg) || goja.IsUndefined(rangeArg) {
			panic(vm.NewTypeError("Failed to execute 'addRange' on 'Selection': parameter 1 is not of type 'Range'."))
		}
		r := b.getGoRange(rangeArg.ToObject(vm))
		if r == nil {
			panic(vm.NewTypeError("Failed to execute 'addRange' on 'Selection': parameter 1 is not of type 'Range'."))
		}
		s.AddRange(r)
		return goja.Undefined()
	})

	jsSelection.Set("removeRange", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'removeRange' on 'Selection': 1 argument required."))
		}
		rangeArg := call.Arguments[0]
		if goja.IsNull(rangeArg) || goja.IsUndefined(rangeArg) {
			panic(vm.NewTypeError("Failed to execute 'removeRange' on 'Selection': parameter 1 is not of type 'Range'."))
		}
		r := b.getGoRange(rangeArg.ToObject(vm))
		if r == nil {
			panic(vm.NewTypeError("Failed to execute 'removeRange' on 'Selection': parameter 1 is not of type 'Range'."))
		}
		if err := s.RemoveRange(r); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("NotFoundError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("removeAllRanges", func(call goja.FunctionCall) goja.Value {
		s.RemoveAllRanges()
		return goja.Undefined()
	})

	jsSelection.Set("empty", func(call goja.FunctionCall) goja.Value {
		s.Empty()
		return goja.Undefined()
	})

	jsSelection.Set("collapse", func(call goja.FunctionCall) goja.Value {
		var node *dom.Node
		offset := 0
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			node = b.getGoNode(call.Arguments[0].ToObject(vm))
		}
		if len(call.Arguments) > 1 {
			offset = int(call.Arguments[1].ToInteger())
		}
		if err := s.Collapse(node, offset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("setPosition", func(call goja.FunctionCall) goja.Value {
		var node *dom.Node
		offset := 0
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			node = b.getGoNode(call.Arguments[0].ToObject(vm))
		}
		if len(call.Arguments) > 1 {
			offset = int(call.Arguments[1].ToInteger())
		}
		if err := s.SetPosition(node, offset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("collapseToStart", func(call goja.FunctionCall) goja.Value {
		if err := s.CollapseToStart(); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidStateError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("collapseToEnd", func(call goja.FunctionCall) goja.Value {
		if err := s.CollapseToEnd(); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidStateError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("extend", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'extend' on 'Selection': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'extend' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'extend' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		offset := 0
		if len(call.Arguments) > 1 {
			offset = int(call.Arguments[1].ToInteger())
		}
		if err := s.Extend(node, offset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidStateError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("selectAllChildren", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'selectAllChildren' on 'Selection': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			panic(vm.NewTypeError("Failed to execute 'selectAllChildren' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			panic(vm.NewTypeError("Failed to execute 'selectAllChildren' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		if err := s.SelectAllChildren(node); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("InvalidNodeTypeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("setBaseAndExtent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 4 {
			panic(vm.NewTypeError("Failed to execute 'setBaseAndExtent' on 'Selection': 4 arguments required."))
		}
		anchorNodeArg := call.Arguments[0]
		if goja.IsNull(anchorNodeArg) || goja.IsUndefined(anchorNodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setBaseAndExtent' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		anchorNode := b.getGoNode(anchorNodeArg.ToObject(vm))
		if anchorNode == nil {
			panic(vm.NewTypeError("Failed to execute 'setBaseAndExtent' on 'Selection': parameter 1 is not of type 'Node'."))
		}
		anchorOffset := int(call.Arguments[1].ToInteger())
		focusNodeArg := call.Arguments[2]
		if goja.IsNull(focusNodeArg) || goja.IsUndefined(focusNodeArg) {
			panic(vm.NewTypeError("Failed to execute 'setBaseAndExtent' on 'Selection': parameter 3 is not of type 'Node'."))
		}
		focusNode := b.getGoNode(focusNodeArg.ToObject(vm))
		if focusNode == nil {
			panic(vm.NewTypeError("Failed to execute 'setBaseAndExtent' on 'Selection': parameter 3 is not of type 'Node'."))
		}
		focusOffset := int(call.Arguments[3].ToInteger())
		if err := s.SetBaseAndExtent(anchorNode, anchorOffset, focusNode, focusOffset); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("IndexSizeError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("containsNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'containsNode' on 'Selection': 1 argument required."))
		}
		nodeArg := call.Arguments[0]
		if goja.IsNull(nodeArg) || goja.IsUndefined(nodeArg) {
			return vm.ToValue(false)
		}
		node := b.getGoNode(nodeArg.ToObject(vm))
		if node == nil {
			return vm.ToValue(false)
		}
		partialContainment := false
		if len(call.Arguments) > 1 {
			partialContainment = call.Arguments[1].ToBoolean()
		}
		return vm.ToValue(s.ContainsNode(node, partialContainment))
	})

	jsSelection.Set("deleteFromDocument", func(call goja.FunctionCall) goja.Value {
		if err := s.DeleteFromDocument(); err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				panic(b.createDOMException(domErr.Name, domErr.Message))
			}
			panic(b.createDOMException("HierarchyRequestError", err.Error()))
		}
		return goja.Undefined()
	})

	jsSelection.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.ToString())
	})

	jsSelection.Set("modify", func(call goja.FunctionCall) goja.Value {
		alter := ""
		direction := ""
		granularity := ""
		if len(call.Arguments) > 0 {
			alter = call.Arguments[0].String()
		}
		if len(call.Arguments) > 1 {
			direction = call.Arguments[1].String()
		}
		if len(call.Arguments) > 2 {
			granularity = call.Arguments[2].String()
		}
		s.Modify(alter, direction, granularity)
		return goja.Undefined()
	})

	// Cache the binding
	b.selectionCache[s] = jsSelection

	return jsSelection
}

// BindTreeWalker creates a JavaScript TreeWalker object.
// The filter can be null, a function, or an object with an acceptNode method.
func (b *DOMBinder) BindTreeWalker(tw *dom.TreeWalker, root *dom.Node, whatToShow uint32, filter goja.Value) *goja.Object {
	vm := b.runtime.vm
	jsTreeWalker := vm.NewObject()

	// Set prototype for instanceof to work
	if b.treeWalkerProto != nil {
		jsTreeWalker.SetPrototype(b.treeWalkerProto)
	}

	// Store the Go TreeWalker and filter
	jsTreeWalker.Set("_goTreeWalker", tw)
	jsTreeWalker.Set("_filter", filter)

	// Read-only root property
	jsRootBound := b.BindNode(root)
	jsTreeWalker.DefineAccessorProperty("root", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return jsRootBound
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only whatToShow property
	jsTreeWalker.DefineAccessorProperty("whatToShow", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(whatToShow)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only filter property
	jsTreeWalker.DefineAccessorProperty("filter", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if goja.IsNull(filter) || goja.IsUndefined(filter) {
			return goja.Null()
		}
		return filter
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// currentNode property (read/write)
	jsTreeWalker.DefineAccessorProperty("currentNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		current := tw.CurrentNode()
		if current == nil {
			return goja.Null()
		}
		return b.BindNode(current)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		arg := call.Arguments[0]
		if goja.IsNull(arg) || goja.IsUndefined(arg) {
			panic(vm.NewTypeError("Failed to set the 'currentNode' property on 'TreeWalker': The provided value is not of type 'Node'."))
		}
		nodeObj := arg.ToObject(vm)
		goNode := b.getGoNode(nodeObj)
		if goNode == nil {
			panic(vm.NewTypeError("Failed to set the 'currentNode' property on 'TreeWalker': The provided value is not of type 'Node'."))
		}
		tw.SetCurrentNode(goNode)
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Active flag for detecting recursive filter calls (per DOM spec)
	activeFlag := false

	// Helper function to call the filter
	callFilter := func(node *dom.Node) int {
		// If no filter, accept all
		if goja.IsNull(filter) || goja.IsUndefined(filter) {
			return 1 // FILTER_ACCEPT
		}

		// Set the active flag before calling filter
		activeFlag = true
		defer func() { activeFlag = false }()

		jsNode := b.BindNode(node)

		// If filter is callable (a function), call it directly
		if filterFunc, ok := goja.AssertFunction(filter); ok {
			result, err := filterFunc(goja.Undefined(), jsNode)
			if err != nil {
				panic(err)
			}
			return int(result.ToInteger())
		}

		// If filter is an object, get its acceptNode property
		filterObj := filter.ToObject(vm)
		acceptNodeVal := filterObj.Get("acceptNode")

		// acceptNode must be a function
		acceptNodeFunc, ok := goja.AssertFunction(acceptNodeVal)
		if !ok {
			panic(vm.NewTypeError("Failed to execute 'acceptNode' on 'NodeFilter': acceptNode is not a function"))
		}

		result, err := acceptNodeFunc(filterObj, jsNode)
		if err != nil {
			panic(err)
		}
		return int(result.ToInteger())
	}

	// Helper to check active flag and throw if set
	checkActiveFlag := func(methodName string) {
		if activeFlag {
			panic(b.createDOMException("InvalidStateError", "Failed to execute '"+methodName+"' on 'TreeWalker': The TreeWalker is in an invalid state."))
		}
	}

	// Helper to check if a node matches whatToShow
	matchesWhatToShow := func(node *dom.Node) bool {
		if whatToShow == 0xFFFFFFFF {
			return true // SHOW_ALL
		}
		nodeType := node.NodeType()
		// Map node type to the bit position (1 << (nodeType - 1))
		mask := uint32(1 << (nodeType - 1))
		return (whatToShow & mask) != 0
	}

	// filterNode applies whatToShow and filter to a node
	filterNode := func(node *dom.Node) int {
		if !matchesWhatToShow(node) {
			return 3 // FILTER_SKIP
		}
		return callFilter(node)
	}

	// parentNode method
	jsTreeWalker.Set("parentNode", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("parentNode")
		node := tw.CurrentNode()
		for node != nil && node != root {
			parent := node.ParentNode()
			if parent == nil || parent == root {
				break
			}
			result := filterNode(parent)
			if result == 1 { // FILTER_ACCEPT
				tw.SetCurrentNode(parent)
				return b.BindNode(parent)
			}
			node = parent
		}
		return goja.Null()
	})

	// firstChild method
	jsTreeWalker.Set("firstChild", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("firstChild")
		return b.traverseChildren(tw, root, filterNode, true)
	})

	// lastChild method
	jsTreeWalker.Set("lastChild", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("lastChild")
		return b.traverseChildren(tw, root, filterNode, false)
	})

	// nextSibling method
	jsTreeWalker.Set("nextSibling", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("nextSibling")
		return b.traverseSiblings(tw, root, filterNode, true)
	})

	// previousSibling method
	jsTreeWalker.Set("previousSibling", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("previousSibling")
		return b.traverseSiblings(tw, root, filterNode, false)
	})

	// nextNode method
	jsTreeWalker.Set("nextNode", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("nextNode")
		node := tw.CurrentNode()
		result := 1 // FILTER_ACCEPT (start assuming accept to enter first child)

		for {
			// Try to traverse into children if the current node was accepted
			for result != 2 { // not FILTER_REJECT
				firstChild := node.FirstChild()
				if firstChild == nil {
					break
				}
				node = firstChild
				result = filterNode(node)
				if result == 1 { // FILTER_ACCEPT
					tw.SetCurrentNode(node)
					return b.BindNode(node)
				}
			}

			// Try siblings and ancestors' siblings
			for {
				if node == root {
					return goja.Null()
				}

				sibling := node.NextSibling()
				for sibling != nil {
					node = sibling
					result = filterNode(node)
					if result == 1 { // FILTER_ACCEPT
						tw.SetCurrentNode(node)
						return b.BindNode(node)
					}
					if result == 3 { // FILTER_SKIP - try children
						break
					}
					// FILTER_REJECT - try next sibling
					sibling = node.NextSibling()
				}

				if sibling != nil {
					break // We found a node to descend into
				}

				// No more siblings, go to parent
				parent := node.ParentNode()
				if parent == nil || parent == root {
					return goja.Null()
				}
				node = parent
				result = 2 // Don't try to descend when going back up
			}
		}
	})

	// previousNode method
	jsTreeWalker.Set("previousNode", func(call goja.FunctionCall) goja.Value {
		checkActiveFlag("previousNode")
		node := tw.CurrentNode()

		for node != root {
			// Try previous sibling
			sibling := node.PreviousSibling()
			for sibling != nil {
				node = sibling
				result := filterNode(node)

				// If accepted or skipped, try to descend to last descendant
				for result != 2 { // not FILTER_REJECT
					lastChild := node.LastChild()
					if lastChild == nil {
						break
					}
					node = lastChild
					result = filterNode(node)
				}

				if result == 1 { // FILTER_ACCEPT
					tw.SetCurrentNode(node)
					return b.BindNode(node)
				}

				// If rejected or skipped, try previous sibling
				sibling = node.PreviousSibling()
			}

			// No more siblings, try parent
			parent := node.ParentNode()
			if parent == nil || parent == root {
				return goja.Null()
			}
			node = parent
			result := filterNode(node)
			if result == 1 { // FILTER_ACCEPT
				tw.SetCurrentNode(node)
				return b.BindNode(node)
			}
		}
		return goja.Null()
	})

	// toString method
	jsTreeWalker.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue("[object TreeWalker]")
	})

	return jsTreeWalker
}

// traverseChildren is a helper for TreeWalker.firstChild and lastChild
func (b *DOMBinder) traverseChildren(tw *dom.TreeWalker, root *dom.Node, filterNode func(*dom.Node) int, first bool) goja.Value {
	node := tw.CurrentNode()
	var child *dom.Node
	if first {
		child = node.FirstChild()
	} else {
		child = node.LastChild()
	}

	for child != nil {
		result := filterNode(child)
		if result == 1 { // FILTER_ACCEPT
			tw.SetCurrentNode(child)
			return b.BindNode(child)
		}
		if result == 3 { // FILTER_SKIP - try grandchildren
			var grandchild *dom.Node
			if first {
				grandchild = child.FirstChild()
			} else {
				grandchild = child.LastChild()
			}
			if grandchild != nil {
				child = grandchild
				continue
			}
		}
		// Move to next/previous sibling
		for {
			var sibling *dom.Node
			if first {
				sibling = child.NextSibling()
			} else {
				sibling = child.PreviousSibling()
			}
			if sibling != nil {
				child = sibling
				break
			}
			// Go up to parent
			parent := child.ParentNode()
			if parent == nil || parent == node || parent == root {
				return goja.Null()
			}
			child = parent
		}
	}
	return goja.Null()
}

// traverseSiblings is a helper for TreeWalker.nextSibling and previousSibling
// This follows the DOM spec "traverse siblings" algorithm.
func (b *DOMBinder) traverseSiblings(tw *dom.TreeWalker, root *dom.Node, filterNode func(*dom.Node) int, next bool) goja.Value {
	node := tw.CurrentNode()
	if node == root {
		return goja.Null()
	}

	for {
		// Get node's sibling
		var sibling *dom.Node
		if next {
			sibling = node.NextSibling()
		} else {
			sibling = node.PreviousSibling()
		}

		// While sibling is not null
		for sibling != nil {
			// Set node to sibling
			node = sibling

			// Filter node and let result be the return value
			result := filterNode(node)

			// If result is FILTER_ACCEPT, set currentNode and return node
			if result == 1 { // FILTER_ACCEPT
				tw.SetCurrentNode(node)
				return b.BindNode(node)
			}

			// Set sibling to node's first/last child
			if next {
				sibling = node.FirstChild()
			} else {
				sibling = node.LastChild()
			}

			// If result is FILTER_REJECT or sibling is null,
			// set sibling to node's next/prev sibling
			if result == 2 || sibling == nil { // FILTER_REJECT or no child
				if next {
					sibling = node.NextSibling()
				} else {
					sibling = node.PreviousSibling()
				}
			}
		}

		// Set node to its parent
		node = node.ParentNode()

		// If node is null or is root, return null
		if node == nil || node == root {
			return goja.Null()
		}

		// Filter node and if the return value is FILTER_ACCEPT, return null
		if filterNode(node) == 1 { // FILTER_ACCEPT
			return goja.Null()
		}

		// Run these substeps again (continue the outer loop)
	}
}

// BindNodeIterator creates a JavaScript NodeIterator object.
// The filter can be null, a function, or an object with an acceptNode method.
func (b *DOMBinder) BindNodeIterator(ni *dom.NodeIterator, root *dom.Node, whatToShow uint32, filter goja.Value) *goja.Object {
	vm := b.runtime.vm
	jsNodeIterator := vm.NewObject()

	// Set prototype for instanceof to work
	if b.nodeIteratorProto != nil {
		jsNodeIterator.SetPrototype(b.nodeIteratorProto)
	}

	// Store the Go NodeIterator and filter
	jsNodeIterator.Set("_goNodeIterator", ni)
	jsNodeIterator.Set("_filter", filter)

	// Read-only root property
	jsRootBound := b.BindNode(root)
	jsNodeIterator.DefineAccessorProperty("root", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return jsRootBound
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only whatToShow property
	jsNodeIterator.DefineAccessorProperty("whatToShow", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(whatToShow)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only filter property
	jsNodeIterator.DefineAccessorProperty("filter", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if goja.IsNull(filter) || goja.IsUndefined(filter) {
			return goja.Null()
		}
		return filter
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only referenceNode property
	jsNodeIterator.DefineAccessorProperty("referenceNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		refNode := ni.ReferenceNode()
		if refNode == nil {
			return goja.Null()
		}
		return b.BindNode(refNode)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only pointerBeforeReferenceNode property
	jsNodeIterator.DefineAccessorProperty("pointerBeforeReferenceNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ni.PointerBeforeReferenceNode())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Active flag for detecting recursive filter calls (per DOM spec)
	activeFlag := false

	// Helper function to call the filter
	callFilter := func(node *dom.Node) int {
		// If no filter, accept all
		if goja.IsNull(filter) || goja.IsUndefined(filter) {
			return 1 // FILTER_ACCEPT
		}

		// Check for recursive filter calls
		if activeFlag {
			panic(b.createDOMException("InvalidStateError", "Failed to execute filter: The NodeIterator is in an invalid state."))
		}

		// Set the active flag before calling filter
		activeFlag = true
		defer func() { activeFlag = false }()

		jsNode := b.BindNode(node)

		// If filter is callable (a function), call it directly
		if filterFunc, ok := goja.AssertFunction(filter); ok {
			result, err := filterFunc(goja.Undefined(), jsNode)
			if err != nil {
				panic(err)
			}
			return int(result.ToInteger())
		}

		// If filter is an object, get its acceptNode property
		filterObj := filter.ToObject(vm)
		acceptNodeVal := filterObj.Get("acceptNode")

		// acceptNode must be a function
		acceptNodeFunc, ok := goja.AssertFunction(acceptNodeVal)
		if !ok {
			panic(vm.NewTypeError("Failed to execute 'acceptNode' on 'NodeFilter': acceptNode is not a function"))
		}

		result, err := acceptNodeFunc(filterObj, jsNode)
		if err != nil {
			panic(err)
		}
		return int(result.ToInteger())
	}

	// Helper to check if a node matches whatToShow
	matchesWhatToShow := func(node *dom.Node) bool {
		if whatToShow == 0xFFFFFFFF {
			return true // SHOW_ALL
		}
		nodeType := node.NodeType()
		// Map node type to the bit position (1 << (nodeType - 1))
		mask := uint32(1 << (nodeType - 1))
		return (whatToShow & mask) != 0
	}

	// filterNode applies whatToShow and filter to a node
	filterNode := func(node *dom.Node) int {
		if !matchesWhatToShow(node) {
			return 3 // FILTER_SKIP
		}
		return callFilter(node)
	}

	// Helper to check if node is in the iterator collection (within root's subtree)
	isInCollection := func(node *dom.Node) bool {
		if node == nil {
			return false
		}
		for n := node; n != nil; n = n.ParentNode() {
			if n == root {
				return true
			}
		}
		return false
	}

	// nextNode method - implements the DOM "traverse" algorithm with direction "next"
	jsNodeIterator.Set("nextNode", func(call goja.FunctionCall) goja.Value {
		// Let node be referenceNode
		node := ni.ReferenceNode()
		// Let beforeNode be pointerBeforeReferenceNode
		beforeNode := ni.PointerBeforeReferenceNode()

		for {
			if !beforeNode {
				// Find the first node following node in document order that is in the collection
				node = b.nextNodeInDocumentOrder(node, root)
				if node == nil {
					return goja.Null()
				}
			}
			beforeNode = false

			// Filter node
			result := filterNode(node)
			if result == 1 { // FILTER_ACCEPT
				ni.SetReferenceNode(node, false)
				return b.BindNode(node)
			}
			// If FILTER_SKIP or FILTER_REJECT, continue to next node
		}
	})

	// previousNode method - implements the DOM "traverse" algorithm with direction "previous"
	jsNodeIterator.Set("previousNode", func(call goja.FunctionCall) goja.Value {
		// Let node be referenceNode
		node := ni.ReferenceNode()
		// Let beforeNode be pointerBeforeReferenceNode
		beforeNode := ni.PointerBeforeReferenceNode()

		for {
			if beforeNode {
				// Find the first node preceding node in document order that is in the collection
				node = b.previousNodeInDocumentOrder(node, root)
				if node == nil {
					return goja.Null()
				}
			}
			beforeNode = true

			// Filter node
			result := filterNode(node)
			if result == 1 { // FILTER_ACCEPT
				ni.SetReferenceNode(node, true)
				return b.BindNode(node)
			}
			// If FILTER_SKIP or FILTER_REJECT, continue to previous node
		}
	})

	// detach method - does nothing per modern spec (kept for backwards compatibility)
	jsNodeIterator.Set("detach", func(call goja.FunctionCall) goja.Value {
		// Per DOM spec: "The detach() method steps are to do nothing."
		return goja.Undefined()
	})

	// toString method
	jsNodeIterator.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue("[object NodeIterator]")
	})

	// Helper for getting the node at a position - not used but kept for completeness
	_ = isInCollection

	return jsNodeIterator
}

// nextNodeInDocumentOrder finds the next node in document order within the subtree rooted at root.
// The "iterator collection" is all nodes that are inclusive descendants of root, in document order.
func (b *DOMBinder) nextNodeInDocumentOrder(node *dom.Node, root *dom.Node) *dom.Node {
	if node == nil {
		return nil
	}

	// Try first child (descendants are always in collection)
	if child := node.FirstChild(); child != nil {
		return child
	}

	// Can't go outside the root subtree
	for node != root {
		// Try next sibling
		if sibling := node.NextSibling(); sibling != nil {
			return sibling
		}
		// Go up to parent
		node = node.ParentNode()
		if node == nil {
			return nil
		}
	}

	return nil
}

// previousNodeInDocumentOrder finds the previous node in document order within the subtree rooted at root.
// The "iterator collection" is all nodes that are inclusive descendants of root, in document order.
func (b *DOMBinder) previousNodeInDocumentOrder(node *dom.Node, root *dom.Node) *dom.Node {
	if node == nil || node == root {
		return nil
	}

	// Try previous sibling's deepest last descendant
	if sibling := node.PreviousSibling(); sibling != nil {
		// Go to the deepest last descendant
		for sibling.LastChild() != nil {
			sibling = sibling.LastChild()
		}
		return sibling
	}

	// Try parent (but not if it is root - root is the boundary, not part of what we can return)
	parent := node.ParentNode()
	if parent == nil || parent == root {
		return nil
	}
	return parent
}

// getGoRange extracts the Go Range from a JavaScript object.
func (b *DOMBinder) getGoRange(obj *goja.Object) *dom.Range {
	if obj == nil {
		return nil
	}
	goRange := obj.Get("_goRange")
	if goRange == nil || goja.IsUndefined(goRange) || goja.IsNull(goRange) {
		return nil
	}
	if r, ok := goRange.Export().(*dom.Range); ok {
		return r
	}
	return nil
}

// bindNodeProperties adds common Node interface properties and methods to a JS object.
func (b *DOMBinder) bindNodeProperties(jsObj *goja.Object, node *dom.Node) {
	vm := b.runtime.vm

	// Add EventTarget methods (addEventListener, removeEventListener, dispatchEvent)
	// All Nodes are EventTargets per the DOM specification
	if b.eventBinder != nil {
		b.eventBinder.BindEventTarget(jsObj)
	}

	// Parent node properties
	jsObj.DefineAccessorProperty("parentNode", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		parent := node.ParentNode()
		if parent == nil {
			return goja.Null()
		}
		return b.BindNode(parent)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("parentElement", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		parent := node.ParentElement()
		if parent == nil {
			return goja.Null()
		}
		return b.BindElement(parent)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Sibling properties
	jsObj.DefineAccessorProperty("previousSibling", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		sibling := node.PreviousSibling()
		if sibling == nil {
			return goja.Null()
		}
		return b.BindNode(sibling)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("nextSibling", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		sibling := node.NextSibling()
		if sibling == nil {
			return goja.Null()
		}
		return b.BindNode(sibling)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Child properties
	jsObj.DefineAccessorProperty("firstChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := node.FirstChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindNode(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("lastChild", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		child := node.LastChild()
		if child == nil {
			return goja.Null()
		}
		return b.BindNode(child)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("childNodes", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindNodeList(node.ChildNodes())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("ownerDocument", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		doc := node.OwnerDocument()
		if doc == nil {
			return goja.Null()
		}
		return b.BindDocument(doc)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// baseURI - returns the absolute base URL of the node
	jsObj.DefineAccessorProperty("baseURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.BaseURI())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsObj.DefineAccessorProperty("isConnected", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.IsConnected())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// nodeValue - returns null for Element, Document, DocumentFragment, DocumentType
	// Text, Comment, ProcessingInstruction, CDATASection override this in their own bindings
	jsObj.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		switch node.NodeType() {
		case dom.TextNode, dom.CommentNode, dom.ProcessingInstructionNode, dom.CDATASectionNode:
			return vm.ToValue(node.NodeValue())
		default:
			return goja.Null()
		}
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Child methods
	jsObj.Set("hasChildNodes", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.HasChildNodes())
	})

	jsObj.Set("appendChild", func(call goja.FunctionCall) goja.Value {
		// Per WebIDL, appendChild requires 1 argument and it must be a Node
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			panic(vm.NewTypeError("Failed to execute 'appendChild' on 'Node': 1 argument required, but only 0 present."))
		}

		arg := call.Arguments[0]
		// Check if it's an object that could be a Node
		if !goja.IsNull(arg) && !goja.IsUndefined(arg) {
			childObj := arg.ToObject(vm)
			goChild := b.getGoNode(childObj)
			if goChild == nil {
				// Argument is not a Node - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'appendChild' on 'Node': parameter 1 is not of type 'Node'."))
			}

			result, err := node.AppendChildWithError(goChild)
			if err != nil {
				if domErr, ok := err.(*dom.DOMError); ok {
					b.throwDOMError(vm, domErr)
				}
				return goja.Null()
			}
			return b.BindNode(result)
		}
		// null or undefined - throw TypeError
		panic(vm.NewTypeError("Failed to execute 'appendChild' on 'Node': parameter 1 is not of type 'Node'."))
	})

	// insertBefore uses typed parameters (newNode, refNode goja.Value) so that
	// insertBefore.length correctly returns 2, which WPT tests rely on to detect
	// whether to pass null as the second argument.
	jsObj.Set("insertBefore", func(newNode, refNode goja.Value) goja.Value {
		// First argument must be a Node (not null or undefined or missing)
		// When an argument is missing, goja passes nil for typed parameters
		if newNode == nil || goja.IsNull(newNode) || goja.IsUndefined(newNode) {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 1 is not of type 'Node'."))
		}

		newChildObj := newNode.ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		if goNewChild == nil {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required per WebIDL - nil means argument was not provided
		if refNode == nil {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined (null and undefined treated as null)
		var goRefChild *dom.Node
		if !goja.IsNull(refNode) && !goja.IsUndefined(refNode) {
			refChildObj := refNode.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				// Not a Node and not null - throw TypeError
				panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 2 is not of type 'Node'."))
			}
		}

		result, err := node.InsertBeforeWithError(goNewChild, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		return b.BindNode(result)
	})

	jsObj.Set("removeChild", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			panic(vm.NewTypeError("Failed to execute 'removeChild' on 'Node': 1 argument required."))
		}

		childObj := call.Arguments[0].ToObject(vm)
		goChild := b.getGoNode(childObj)
		if goChild == nil {
			panic(vm.NewTypeError("Failed to execute 'removeChild' on 'Node': parameter 1 is not of type 'Node'."))
		}

		result, err := node.RemoveChildWithError(goChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		// Don't remove from cache - the node identity should remain stable
		// even when detached from the DOM tree
		return b.BindNode(result)
	})

	jsObj.Set("replaceChild", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': 2 arguments required."))
		}

		arg0 := call.Arguments[0]
		arg1 := call.Arguments[1]

		if goja.IsNull(arg0) || goja.IsUndefined(arg0) {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 1 is not of type 'Node'."))
		}
		if goja.IsNull(arg1) || goja.IsUndefined(arg1) {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 2 is not of type 'Node'."))
		}

		newChildObj := arg0.ToObject(vm)
		oldChildObj := arg1.ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		goOldChild := b.getGoNode(oldChildObj)

		if goNewChild == nil {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 1 is not of type 'Node'."))
		}
		if goOldChild == nil {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 2 is not of type 'Node'."))
		}

		result, err := node.ReplaceChildWithError(goNewChild, goOldChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		// Return the existing JS binding for the removed node to preserve object identity
		return b.BindNode(result)
	})

	// moveBefore - state-preserving atomic move API
	// Only available on ParentNode types (Document, DocumentFragment, Element)
	// Not on Text, Comment, DocumentType, ProcessingInstruction, etc.
	nodeType := node.NodeType()
	if nodeType == dom.DocumentNode || nodeType == dom.DocumentFragmentNode || nodeType == dom.ElementNode {
		jsObj.Set("moveBefore", func(movedNode, referenceNode goja.Value) goja.Value {
			// First argument must be a Node (not null or undefined or missing)
			if movedNode == nil || goja.IsNull(movedNode) || goja.IsUndefined(movedNode) {
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Node': parameter 1 is not of type 'Node'."))
			}

			movedObj := movedNode.ToObject(vm)
			goMovedNode := b.getGoNode(movedObj)
			if goMovedNode == nil {
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Node': parameter 1 is not of type 'Node'."))
			}

			// Second argument is required per WebIDL
			if referenceNode == nil {
				panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Node': 2 arguments required, but only 1 present."))
			}

			// Second argument can be Node, null, or undefined (null and undefined treated as null)
			var goRefChild *dom.Node
			if !goja.IsNull(referenceNode) && !goja.IsUndefined(referenceNode) {
				refChildObj := referenceNode.ToObject(vm)
				goRefChild = b.getGoNode(refChildObj)
				if goRefChild == nil {
					// Not a Node and not null - throw TypeError
					panic(vm.NewTypeError("Failed to execute 'moveBefore' on 'Node': parameter 2 is not of type 'Node'."))
				}
			}

			err := node.MoveBefore(goMovedNode, goRefChild)
			if err != nil {
				if domErr, ok := err.(*dom.DOMError); ok {
					b.throwDOMError(vm, domErr)
				}
				return goja.Undefined()
			}
			return goja.Undefined()
		})
	}

	jsObj.Set("cloneNode", func(call goja.FunctionCall) goja.Value {
		deep := false
		if len(call.Arguments) > 0 {
			deep = call.Arguments[0].ToBoolean()
		}
		clone := node.CloneNode(deep)
		if clone == nil {
			return goja.Null()
		}
		return b.BindNode(clone)
	})

	jsObj.Set("contains", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		if goOther == nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(node.Contains(goOther))
	})

	jsObj.Set("normalize", func(call goja.FunctionCall) goja.Value {
		node.Normalize()
		return goja.Undefined()
	})

	jsObj.Set("isSameNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		return vm.ToValue(node.IsSameNode(goOther))
	})

	jsObj.Set("isEqualNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		return vm.ToValue(node.IsEqualNode(goOther))
	})

	jsObj.Set("getRootNode", func(call goja.FunctionCall) goja.Value {
		// Check for options argument with composed property
		composed := false
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			optObj := call.Arguments[0].ToObject(vm)
			if composedVal := optObj.Get("composed"); composedVal != nil && !goja.IsUndefined(composedVal) {
				composed = composedVal.ToBoolean()
			}
		}

		root := node.GetRootNodeWithOptions(composed)
		if root == nil {
			return goja.Null()
		}

		// Check if root is a shadow root - if so, bind it as ShadowRoot
		if root.IsShadowRoot() {
			return b.BindShadowRoot(root.GetShadowRoot())
		}
		return b.BindNode(root)
	})

	jsObj.Set("compareDocumentPosition", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(0)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		if goOther == nil {
			return vm.ToValue(0)
		}
		return vm.ToValue(int(node.CompareDocumentPosition(goOther)))
	})

	// lookupNamespaceURI returns the namespace URI associated with a given prefix
	jsObj.Set("lookupNamespaceURI", func(call goja.FunctionCall) goja.Value {
		var prefix string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			prefix = call.Arguments[0].String()
		}
		result := node.LookupNamespaceURI(prefix)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})

	// isDefaultNamespace checks if the given namespace URI is the default namespace
	jsObj.Set("isDefaultNamespace", func(call goja.FunctionCall) goja.Value {
		var namespaceURI string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		return vm.ToValue(node.IsDefaultNamespace(namespaceURI))
	})

	// lookupPrefix returns the prefix associated with a given namespace URI
	jsObj.Set("lookupPrefix", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			return goja.Null()
		}
		namespaceURI := call.Arguments[0].String()
		result := node.LookupPrefix(namespaceURI)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})
}

// setupNodePrototypeMethods adds Node methods to the prototype so that
// Node.prototype.insertBefore, Node.prototype.appendChild, etc. exist.
// This is needed for WPT tests that do: var f = Node.prototype.insertBefore; f.call(node, ...)
func (b *DOMBinder) setupNodePrototypeMethods() {
	vm := b.runtime.vm

	// appendChild - prototype version that gets node from 'this'
	b.nodeProto.Set("appendChild", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			panic(vm.NewTypeError("Failed to execute 'appendChild' on 'Node': 1 argument required, but only 0 present."))
		}

		arg := call.Arguments[0]
		childObj := arg.ToObject(vm)
		goChild := b.getGoNode(childObj)
		if goChild == nil {
			panic(vm.NewTypeError("Failed to execute 'appendChild' on 'Node': parameter 1 is not of type 'Node'."))
		}

		result, err := node.AppendChildWithError(goChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		return b.BindNode(result)
	})

	// insertBefore - prototype version
	b.nodeProto.Set("insertBefore", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		// First argument must be a Node
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 1 is not of type 'Node'."))
		}

		newChildObj := call.Arguments[0].ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		if goNewChild == nil {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 1 is not of type 'Node'."))
		}

		// Second argument is required
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': 2 arguments required, but only 1 present."))
		}

		// Second argument can be Node, null, or undefined
		var goRefChild *dom.Node
		refArg := call.Arguments[1]
		if !goja.IsNull(refArg) && !goja.IsUndefined(refArg) {
			refChildObj := refArg.ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
			if goRefChild == nil {
				panic(vm.NewTypeError("Failed to execute 'insertBefore' on 'Node': parameter 2 is not of type 'Node'."))
			}
		}

		result, err := node.InsertBeforeWithError(goNewChild, goRefChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		return b.BindNode(result)
	})

	// removeChild - prototype version
	b.nodeProto.Set("removeChild", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			panic(vm.NewTypeError("Failed to execute 'removeChild' on 'Node': 1 argument required."))
		}

		childObj := call.Arguments[0].ToObject(vm)
		goChild := b.getGoNode(childObj)
		if goChild == nil {
			panic(vm.NewTypeError("Failed to execute 'removeChild' on 'Node': parameter 1 is not of type 'Node'."))
		}

		result, err := node.RemoveChildWithError(goChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		// Don't remove from cache - the node identity should remain stable
		// even when detached from the DOM tree
		return b.BindNode(result)
	})

	// replaceChild - prototype version
	b.nodeProto.Set("replaceChild", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': 2 arguments required."))
		}

		arg0 := call.Arguments[0]
		arg1 := call.Arguments[1]

		if goja.IsNull(arg0) || goja.IsUndefined(arg0) {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 1 is not of type 'Node'."))
		}
		if goja.IsNull(arg1) || goja.IsUndefined(arg1) {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 2 is not of type 'Node'."))
		}

		newChildObj := arg0.ToObject(vm)
		oldChildObj := arg1.ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		goOldChild := b.getGoNode(oldChildObj)

		if goNewChild == nil {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 1 is not of type 'Node'."))
		}
		if goOldChild == nil {
			panic(vm.NewTypeError("Failed to execute 'replaceChild' on 'Node': parameter 2 is not of type 'Node'."))
		}

		result, err := node.ReplaceChildWithError(goNewChild, goOldChild)
		if err != nil {
			if domErr, ok := err.(*dom.DOMError); ok {
				b.throwDOMError(vm, domErr)
			}
			return goja.Null()
		}
		// Return the existing JS binding for the removed node to preserve object identity
		return b.BindNode(result)
	})

	// hasChildNodes - prototype version
	b.nodeProto.Set("hasChildNodes", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		return vm.ToValue(node.HasChildNodes())
	})

	// cloneNode - prototype version
	b.nodeProto.Set("cloneNode", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		deep := false
		if len(call.Arguments) > 0 {
			deep = call.Arguments[0].ToBoolean()
		}
		clone := node.CloneNode(deep)
		if clone == nil {
			return goja.Null()
		}
		return b.BindNode(clone)
	})

	// contains - prototype version
	b.nodeProto.Set("contains", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		if goOther == nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(node.Contains(goOther))
	})

	// normalize - prototype version
	b.nodeProto.Set("normalize", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		node.Normalize()
		return goja.Undefined()
	})

	// isSameNode - prototype version
	b.nodeProto.Set("isSameNode", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		return vm.ToValue(node.IsSameNode(goOther))
	})

	// isEqualNode - prototype version
	b.nodeProto.Set("isEqualNode", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		return vm.ToValue(node.IsEqualNode(goOther))
	})

	// getRootNode - prototype version
	b.nodeProto.Set("getRootNode", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		// Check for options argument with composed property
		composed := false
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			optObj := call.Arguments[0].ToObject(vm)
			if composedVal := optObj.Get("composed"); composedVal != nil && !goja.IsUndefined(composedVal) {
				composed = composedVal.ToBoolean()
			}
		}

		root := node.GetRootNodeWithOptions(composed)
		if root == nil {
			return goja.Null()
		}

		// Check if root is a shadow root - if so, bind it as ShadowRoot
		if root.IsShadowRoot() {
			return b.BindShadowRoot(root.GetShadowRoot())
		}
		return b.BindNode(root)
	})

	// compareDocumentPosition - prototype version
	b.nodeProto.Set("compareDocumentPosition", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		if len(call.Arguments) < 1 {
			return vm.ToValue(0)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		goOther := b.getGoNode(otherObj)
		if goOther == nil {
			return vm.ToValue(0)
		}
		return vm.ToValue(int(node.CompareDocumentPosition(goOther)))
	})

	// lookupNamespaceURI - prototype version
	b.nodeProto.Set("lookupNamespaceURI", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		var prefix string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			prefix = call.Arguments[0].String()
		}
		result := node.LookupNamespaceURI(prefix)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})

	// isDefaultNamespace - prototype version
	b.nodeProto.Set("isDefaultNamespace", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		var namespaceURI string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		return vm.ToValue(node.IsDefaultNamespace(namespaceURI))
	})

	// lookupPrefix - prototype version
	b.nodeProto.Set("lookupPrefix", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)
		node := b.getGoNode(thisObj)
		if node == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			return goja.Null()
		}
		namespaceURI := call.Arguments[0].String()
		result := node.LookupPrefix(namespaceURI)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})
}

// setupDocumentFragmentPrototypeMethods adds methods to DocumentFragment.prototype.
// This ensures that DocumentFragment.prototype.getElementById exists as a function,
// which is required by WPT tests like dom/nodes/DocumentFragment-getElementById.html.
func (b *DOMBinder) setupDocumentFragmentPrototypeMethods() {
	vm := b.runtime.vm

	// getElementById - gets the fragment from 'this' and searches for element by ID
	b.documentFragmentProto.Set("getElementById", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)

		// Get the DocumentFragment from the JS object
		fragVal := thisObj.Get("_goFragment")
		if fragVal == nil || goja.IsUndefined(fragVal) || goja.IsNull(fragVal) {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		frag, ok := fragVal.Export().(*dom.DocumentFragment)
		if !ok || frag == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		id := call.Arguments[0].String()

		// Per spec, empty string ID returns null
		if id == "" {
			return goja.Null()
		}

		el := frag.GetElementById(id)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// querySelector - prototype method
	b.documentFragmentProto.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)

		fragVal := thisObj.Get("_goFragment")
		if fragVal == nil || goja.IsUndefined(fragVal) || goja.IsNull(fragVal) {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		frag, ok := fragVal.Export().(*dom.DocumentFragment)
		if !ok || frag == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		el := frag.QuerySelector(selector)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// querySelectorAll - prototype method
	b.documentFragmentProto.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		thisObj := call.This.ToObject(vm)

		fragVal := thisObj.Get("_goFragment")
		if fragVal == nil || goja.IsUndefined(fragVal) || goja.IsNull(fragVal) {
			panic(vm.NewTypeError("Illegal invocation"))
		}
		frag, ok := fragVal.Export().(*dom.DocumentFragment)
		if !ok || frag == nil {
			panic(vm.NewTypeError("Illegal invocation"))
		}

		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		nodeList := frag.QuerySelectorAll(selector)
		return b.bindStaticNodeList(nodeList.ToSlice())
	})
}

// setupShadowRootPrototypeMethods sets up methods on the ShadowRoot prototype.
func (b *DOMBinder) setupShadowRootPrototypeMethods() {
	// ShadowRoot inherits from DocumentFragment, so most methods are already available
	// through the prototype chain. We just need to add ShadowRoot-specific behavior
	// if any (most properties are set in BindShadowRoot as they're instance-specific).

	// The following methods from DocumentFragment prototype will work via inheritance:
	// - getElementById
	// - querySelector
	// - querySelectorAll
}

// getGoNode extracts the Go *dom.Node from a JavaScript object.
func (b *DOMBinder) getGoNode(obj *goja.Object) *dom.Node {
	if obj == nil {
		return nil
	}

	// Try _goNode first
	if v := obj.Get("_goNode"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		if node, ok := v.Export().(*dom.Node); ok {
			return node
		}
	}

	// Try _goElement
	if v := obj.Get("_goElement"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		if el, ok := v.Export().(*dom.Element); ok {
			return el.AsNode()
		}
	}

	// Try _goDoc
	if v := obj.Get("_goDoc"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		if doc, ok := v.Export().(*dom.Document); ok {
			return doc.AsNode()
		}
	}

	return nil
}

// getGoAttr extracts a Go Attr from a JavaScript Attr object.
func (b *DOMBinder) getGoAttr(obj *goja.Object) *dom.Attr {
	if obj == nil {
		return nil
	}
	if v := obj.Get("_goAttr"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		if attr, ok := v.Export().(*dom.Attr); ok {
			return attr
		}
	}
	return nil
}

// BindNodeList creates a JavaScript NodeList object.
// The object is cached so that the same DOM NodeList returns the same JS object.
func (b *DOMBinder) BindNodeList(nodeList *dom.NodeList) *goja.Object {
	// Check cache first - return same JS object for same NodeList
	if cached, ok := b.nodeListCache[nodeList]; ok {
		return cached
	}

	// Create the NodeList object using a dynamic proxy for live collection behavior
	jsList := b.createNodeListProxy(nodeList)

	// Cache it
	b.nodeListCache[nodeList] = jsList

	return jsList
}

// createNodeListProxy creates a NodeList with a Proxy for dynamic indexed access.
// This ensures the collection is "live" - accessing list[0] always gets the current first child.
func (b *DOMBinder) createNodeListProxy(nodeList *dom.NodeList) *goja.Object {
	vm := b.runtime.vm

	// Create the target object with methods and prototype
	target := vm.NewObject()

	// Set prototype for instanceof to work
	if b.nodeListProto != nil {
		target.SetPrototype(b.nodeListProto)
	}

	// Define length as a getter (live property)
	// Per spec: length is non-enumerable, but we make it configurable to satisfy proxy invariants
	target.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(nodeList.Length())
	}), nil, goja.FLAG_TRUE, goja.FLAG_FALSE)

	// item() method
	target.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		node := nodeList.Item(index)
		if node == nil {
			return goja.Null()
		}
		return b.BindNode(node)
	})

	// Get Array.prototype methods and set them on NodeList
	// Per spec, NodeList should use Array.prototype's iterator methods
	arrayProto := vm.Get("Array").ToObject(vm).Get("prototype").ToObject(vm)

	// Symbol.iterator - copy from Array.prototype using DefineDataPropertySymbol
	// We need to get the symbol and set it properly
	_, _ = vm.RunString(`
		(function(target, arrayProto) {
			target[Symbol.iterator] = arrayProto[Symbol.iterator];
		})
	`)
	symbolSetup, _ := vm.RunString(`
		(function(target, arrayProto) {
			target[Symbol.iterator] = arrayProto[Symbol.iterator];
		})
	`)
	if symbolSetup != nil {
		fn, _ := goja.AssertFunction(symbolSetup)
		if fn != nil {
			fn(goja.Undefined(), target, arrayProto)
		}
	}

	// keys, values, entries, forEach - copy from Array.prototype
	for _, method := range []string{"keys", "values", "entries", "forEach"} {
		m := arrayProto.Get(method)
		if m != nil && !goja.IsUndefined(m) {
			target.Set(method, m)
		}
	}

	// Create a Proxy to handle dynamic indexed access
	// The proxy intercepts get operations for numeric indices
	proxyHandler := vm.NewObject()

	// Store binder reference for use in handler closures
	binder := b

	// get trap - intercepts property access
	proxyHandler.Set("get", func(call goja.FunctionCall) goja.Value {
		targetObj := call.Arguments[0].ToObject(vm)
		prop := call.Arguments[1]
		propStr := prop.String()

		// Check if it's a numeric index
		if isNumericString(propStr) {
			index := parseNumericString(propStr)
			if index >= 0 && index < nodeList.Length() {
				node := nodeList.Item(index)
				if node != nil {
					return binder.BindNode(node)
				}
			}
			return goja.Undefined()
		}

		// For non-numeric properties, use Reflect.get to properly handle prototype chain
		reflect := vm.Get("Reflect").ToObject(vm)
		reflectGet, _ := goja.AssertFunction(reflect.Get("get"))
		if reflectGet != nil {
			result, err := reflectGet(goja.Undefined(), targetObj, prop)
			if err == nil {
				return result
			}
		}

		// Fallback to direct target access
		return targetObj.Get(propStr)
	})

	// has trap - for "in" operator
	proxyHandler.Set("has", func(call goja.FunctionCall) goja.Value {
		targetObj := call.Arguments[0].ToObject(vm)
		prop := call.Arguments[1]
		propStr := prop.String()

		// Check if it's a numeric index
		if isNumericString(propStr) {
			index := int(prop.ToInteger())
			return vm.ToValue(index >= 0 && index < nodeList.Length())
		}

		// For Symbols and other properties, use Reflect.has to check target properly
		// This handles Symbol.iterator and other symbol properties
		reflect := vm.Get("Reflect").ToObject(vm)
		reflectHas, _ := goja.AssertFunction(reflect.Get("has"))
		if reflectHas != nil {
			result, err := reflectHas(goja.Undefined(), targetObj, prop)
			if err == nil {
				return result
			}
		}

		// Fallback to direct target check
		val := targetObj.Get(propStr)
		return vm.ToValue(val != nil && !goja.IsUndefined(val))
	})

	// ownKeys trap - for Object.getOwnPropertyNames
	proxyHandler.Set("ownKeys", func(call goja.FunctionCall) goja.Value {
		length := nodeList.Length()
		keys := make([]interface{}, length+1)
		for i := 0; i < length; i++ {
			keys[i] = vm.ToValue(i).String()
		}
		keys[length] = "length"
		return vm.ToValue(keys)
	})

	// getOwnPropertyDescriptor trap
	proxyHandler.Set("getOwnPropertyDescriptor", func(call goja.FunctionCall) goja.Value {
		targetObj := call.Arguments[0].ToObject(vm)
		prop := call.Arguments[1]
		propStr := prop.String()

		if isNumericString(propStr) {
			index := int(prop.ToInteger())
			if index >= 0 && index < nodeList.Length() {
				desc := vm.NewObject()
				node := nodeList.Item(index)
				if node != nil {
					desc.Set("value", binder.BindNode(node))
				} else {
					desc.Set("value", goja.Undefined())
				}
				desc.Set("writable", false)
				desc.Set("enumerable", true)
				desc.Set("configurable", true)
				return desc
			}
		}

		// For other properties (length, Symbol.iterator, methods, etc.), use Reflect.getOwnPropertyDescriptor
		// This ensures consistency with the target's actual descriptors
		reflect := vm.Get("Reflect").ToObject(vm)
		reflectGetOwnPropDesc, _ := goja.AssertFunction(reflect.Get("getOwnPropertyDescriptor"))
		if reflectGetOwnPropDesc != nil {
			result, err := reflectGetOwnPropDesc(goja.Undefined(), targetObj, prop)
			if err == nil {
				return result
			}
		}

		return goja.Undefined()
	})

	// Create the proxy via JavaScript's new Proxy() constructor
	// Store handler temporarily in a unique location
	vm.Set("__nodeListTarget", target)
	vm.Set("__nodeListHandler", proxyHandler)

	result, err := vm.RunString("new Proxy(__nodeListTarget, __nodeListHandler)")

	// Clean up temporary globals
	vm.Set("__nodeListTarget", goja.Undefined())
	vm.Set("__nodeListHandler", goja.Undefined())

	if err != nil {
		// Fallback to non-proxy version if Proxy fails
		return target
	}

	return result.ToObject(vm)
}

// isNumericString checks if a string represents a non-negative integer
func isNumericString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseNumericString parses a string as a non-negative integer
func parseNumericString(s string) int {
	result := 0
	for _, c := range s {
		result = result*10 + int(c-'0')
	}
	return result
}

// bindStaticNodeList creates a NodeList-like object from a slice of nodes.
func (b *DOMBinder) bindStaticNodeList(nodes []*dom.Node) *goja.Object {
	vm := b.runtime.vm
	jsList := vm.NewObject()

	// Set prototype for instanceof to work
	if b.nodeListProto != nil {
		jsList.SetPrototype(b.nodeListProto)
	}

	length := len(nodes)

	jsList.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(length)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsList.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		if index < 0 || index >= length {
			return goja.Null()
		}
		return b.BindNode(nodes[index])
	})

	// Array-like indexing
	for i := 0; i < length; i++ {
		idx := i
		jsList.DefineAccessorProperty(vm.ToValue(idx).String(), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if idx >= length {
				return goja.Undefined()
			}
			return b.BindNode(nodes[idx])
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}

	jsList.Set("forEach", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		callback, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}
		var thisArg goja.Value = goja.Undefined()
		if len(call.Arguments) > 1 {
			thisArg = call.Arguments[1]
		}
		for i := 0; i < length; i++ {
			jsNode := b.BindNode(nodes[i])
			callback(thisArg, jsNode, vm.ToValue(i), jsList)
		}
		return goja.Undefined()
	})

	return jsList
}

// BindHTMLCollection creates a JavaScript HTMLCollection object.
// Uses a Proxy to provide live access to elements (HTMLCollection is a live collection).
func (b *DOMBinder) BindHTMLCollection(collection *dom.HTMLCollection) *goja.Object {
	vm := b.runtime.vm
	jsCol := vm.NewObject()

	// Set prototype for instanceof to work
	if b.htmlCollectionProto != nil {
		jsCol.SetPrototype(b.htmlCollectionProto)
	}

	// Register the collection in our internal map for prototype methods
	b.htmlCollectionMap[jsCol] = collection

	// Store expando properties (user-set properties that are not DOM properties)
	// The value is a struct to track property descriptor attributes
	type expandoValue struct {
		value        goja.Value
		writable     bool
		enumerable   bool
		configurable bool
	}
	expandos := make(map[string]expandoValue)

	// Helper to check if a string is a valid array index.
	// Per the ECMAScript spec, a valid array index is a non-negative integer
	// less than 2^32 - 1 (4294967295). The string representation must be the
	// canonical form (no leading zeros except for "0" itself).
	isArrayIndex := func(s string) bool {
		if len(s) == 0 {
			return false
		}
		// Check for leading zero (invalid except for "0" itself)
		if len(s) > 1 && s[0] == '0' {
			return false
		}
		// Check all characters are digits
		for _, c := range s {
			if c < '0' || c > '9' {
				return false
			}
		}
		// Parse the number and check it's a valid array index (< 2^32 - 1)
		// We need to use uint64 to avoid overflow
		var n uint64
		for _, c := range s {
			digit := uint64(c - '0')
			// Check for overflow before multiplying
			if n > (1<<32-2)/10 {
				return false // Would overflow
			}
			n = n*10 + digit
			if n > 1<<32-2 { // 4294967294 is the max array index
				return false
			}
		}
		return true
	}

	// Helper to parse array index - only call after isArrayIndex returns true
	parseArrayIndex := func(s string) int {
		var n int
		for _, c := range s {
			n = n*10 + int(c-'0')
		}
		return n
	}

	// Create a proxy for live access to the collection
	proxy := vm.NewProxy(jsCol, &goja.ProxyTrapConfig{
		// Get property - intercept numeric indices and named properties
		Get: func(target *goja.Object, property string, receiver goja.Value) goja.Value {
			// Check for expandos first
			if exp, ok := expandos[property]; ok {
				return exp.value
			}

			// Check numeric index
			if isArrayIndex(property) {
				idx := parseArrayIndex(property)
				el := collection.Item(idx)
				if el == nil {
					return goja.Undefined()
				}
				return b.BindElement(el)
			}

			// Check named item
			el := collection.NamedItem(property)
			if el != nil {
				return b.BindElement(el)
			}

			// Fall through to target object (for prototype methods etc)
			return target.Get(property)
		},

		// GetIdx for numeric indices
		// Note: goja calls this for integers. We need to handle:
		// - Negative indices: treat as named properties (e.g., "-1" as id)
		// - Values >= 2^32-1: treat as named properties (not valid array indices)
		GetIdx: func(target *goja.Object, idx int, receiver goja.Value) goja.Value {
			// Check if idx is a valid array index (0 to 2^32-2)
			// Negative indices or values >= 2^32-1 are named properties
			const maxArrayIndex = 1<<32 - 2 // 4294967294
			if idx < 0 || int64(idx) > maxArrayIndex {
				name := fmt.Sprintf("%d", idx)
				el := collection.NamedItem(name)
				if el != nil {
					return b.BindElement(el)
				}
				// Fall through to target for prototype methods
				return target.Get(name)
			}
			el := collection.Item(idx)
			if el == nil {
				return goja.Undefined()
			}
			return b.BindElement(el)
		},

		// Set property - handle expando vs named property shadowing
		// Returning false will cause strict mode to throw TypeError
		Set: func(target *goja.Object, property string, value goja.Value, receiver goja.Value) bool {
			// Numeric indices are read-only - always reject for any index
			if isArrayIndex(property) {
				return false
			}

			// Check if named property already exists in DOM
			el := collection.NamedItem(property)
			if el != nil {
				// Named property exists - reject the set (strict mode will throw)
				return false
			}

			// No DOM property - store as expando (default writable, enumerable, configurable)
			expandos[property] = expandoValue{value: value, writable: true, enumerable: true, configurable: true}
			return true
		},

		// SetIdx for numeric indices
		// Note: Values outside valid array index range are treated as named properties.
		SetIdx: func(target *goja.Object, idx int, value goja.Value, receiver goja.Value) bool {
			const maxArrayIndex = 1<<32 - 2
			if idx < 0 || int64(idx) > maxArrayIndex {
				// Non-array-index values are named properties
				name := fmt.Sprintf("%d", idx)
				el := collection.NamedItem(name)
				if el != nil {
					return false // Named property exists, reject
				}
				expandos[name] = expandoValue{value: value, writable: true, enumerable: true, configurable: true}
				return true
			}
			// Valid array indices are read-only
			return false
		},

		// Has property
		Has: func(target *goja.Object, property string) bool {
			if _, ok := expandos[property]; ok {
				return true
			}
			if isArrayIndex(property) {
				idx := parseArrayIndex(property)
				return idx < collection.Length()
			}
			return collection.NamedItem(property) != nil || target.Get(property) != nil
		},

		// HasIdx for numeric indices
		// Note: Values outside valid array index range are treated as named properties.
		HasIdx: func(target *goja.Object, idx int) bool {
			const maxArrayIndex = 1<<32 - 2
			if idx < 0 || int64(idx) > maxArrayIndex {
				name := fmt.Sprintf("%d", idx)
				if _, ok := expandos[name]; ok {
					return true
				}
				return collection.NamedItem(name) != nil
			}
			return idx < collection.Length()
		},

		// OwnKeys - return all indexed and named properties
		OwnKeys: func(target *goja.Object) *goja.Object {
			keys := make([]interface{}, 0)
			// Add numeric indices
			for i := 0; i < collection.Length(); i++ {
				keys = append(keys, vm.ToValue(i).String())
			}
			// Add named properties
			for _, prop := range collection.NamedProperties() {
				keys = append(keys, prop.Name)
			}
			// Add expandos
			for k := range expandos {
				keys = append(keys, k)
			}
			return vm.ToValue(keys).ToObject(vm)
		},

		// GetOwnPropertyDescriptor
		GetOwnPropertyDescriptor: func(target *goja.Object, prop string) goja.PropertyDescriptor {
			// Check expandos
			if exp, ok := expandos[prop]; ok {
				writable := goja.FLAG_FALSE
				if exp.writable {
					writable = goja.FLAG_TRUE
				}
				enumerable := goja.FLAG_FALSE
				if exp.enumerable {
					enumerable = goja.FLAG_TRUE
				}
				configurable := goja.FLAG_FALSE
				if exp.configurable {
					configurable = goja.FLAG_TRUE
				}
				return goja.PropertyDescriptor{
					Value:        exp.value,
					Writable:     writable,
					Enumerable:   enumerable,
					Configurable: configurable,
				}
			}

			// Check numeric index
			if isArrayIndex(prop) {
				idx := parseArrayIndex(prop)
				if idx < collection.Length() {
					el := collection.Item(idx)
					return goja.PropertyDescriptor{
						Value:        b.BindElement(el),
						Writable:     goja.FLAG_FALSE,
						Enumerable:   goja.FLAG_TRUE,
						Configurable: goja.FLAG_TRUE,
					}
				}
				return goja.PropertyDescriptor{}
			}

			// Check named property
			el := collection.NamedItem(prop)
			if el != nil {
				return goja.PropertyDescriptor{
					Value:        b.BindElement(el),
					Writable:     goja.FLAG_FALSE,
					Enumerable:   goja.FLAG_FALSE, // Named props are not enumerable
					Configurable: goja.FLAG_TRUE,
				}
			}

			return goja.PropertyDescriptor{}
		},

		// GetOwnPropertyDescriptorIdx for numeric indices
		// Note: Values outside valid array index range are treated as named properties.
		GetOwnPropertyDescriptorIdx: func(target *goja.Object, idx int) goja.PropertyDescriptor {
			const maxArrayIndex = 1<<32 - 2
			if idx < 0 || int64(idx) > maxArrayIndex {
				name := fmt.Sprintf("%d", idx)
				// Check expandos first
				if exp, ok := expandos[name]; ok {
					writable := goja.FLAG_FALSE
					if exp.writable {
						writable = goja.FLAG_TRUE
					}
					enumerable := goja.FLAG_FALSE
					if exp.enumerable {
						enumerable = goja.FLAG_TRUE
					}
					configurable := goja.FLAG_FALSE
					if exp.configurable {
						configurable = goja.FLAG_TRUE
					}
					return goja.PropertyDescriptor{
						Value:        exp.value,
						Writable:     writable,
						Enumerable:   enumerable,
						Configurable: configurable,
					}
				}
				// Check named property
				el := collection.NamedItem(name)
				if el != nil {
					return goja.PropertyDescriptor{
						Value:        b.BindElement(el),
						Writable:     goja.FLAG_FALSE,
						Enumerable:   goja.FLAG_FALSE,
						Configurable: goja.FLAG_TRUE,
					}
				}
				return goja.PropertyDescriptor{}
			}
			if idx < collection.Length() {
				el := collection.Item(idx)
				return goja.PropertyDescriptor{
					Value:        b.BindElement(el),
					Writable:     goja.FLAG_FALSE,
					Enumerable:   goja.FLAG_TRUE,
					Configurable: goja.FLAG_TRUE,
				}
			}
			return goja.PropertyDescriptor{}
		},

		// Delete property
		DeleteProperty: func(target *goja.Object, property string) bool {
			// Check if it's an expando
			if exp, ok := expandos[property]; ok {
				// Can only delete configurable expandos
				if exp.configurable {
					delete(expandos, property)
					return true
				}
				return false // Non-configurable, strict mode will throw
			}

			// DOM properties (indices and named items) cannot be deleted
			// Return false to throw TypeError in strict mode
			if isArrayIndex(property) {
				idx := parseArrayIndex(property)
				if idx < collection.Length() {
					return false // Index exists, can't delete
				}
				// Index doesn't exist, deletion is vacuously successful
				return true
			}

			if collection.NamedItem(property) != nil {
				return false // Named item exists, can't delete
			}

			// Property doesn't exist, deletion is vacuously successful
			return true
		},

		// DeletePropertyIdx for numeric indices
		// Note: Values outside valid array index range are treated as named properties.
		DeletePropertyIdx: func(target *goja.Object, idx int) bool {
			const maxArrayIndex = 1<<32 - 2
			if idx < 0 || int64(idx) > maxArrayIndex {
				name := fmt.Sprintf("%d", idx)
				// Check expandos first
				if exp, ok := expandos[name]; ok {
					if exp.configurable {
						delete(expandos, name)
						return true
					}
					return false // Non-configurable
				}
				// Check named property
				if collection.NamedItem(name) != nil {
					return false // Can't delete named property
				}
				return true // Doesn't exist
			}
			if idx < collection.Length() {
				return false // Index exists, can't delete
			}
			return true // Index doesn't exist, vacuously successful
		},

		// DefineProperty - prevent defining properties that shadow DOM properties
		DefineProperty: func(target *goja.Object, property string, desc goja.PropertyDescriptor) bool {
			// Can't define properties at any numeric index
			if isArrayIndex(property) {
				return false // Always reject, strict mode will throw
			}

			// Can't define properties that shadow existing named DOM properties
			if collection.NamedItem(property) != nil {
				return false // Strict mode will throw
			}

			// Store as expando with all descriptor flags
			// Default to false for attributes not specified (as per Object.defineProperty behavior)
			writable := desc.Writable == goja.FLAG_TRUE
			enumerable := desc.Enumerable == goja.FLAG_TRUE
			configurable := desc.Configurable == goja.FLAG_TRUE
			expandos[property] = expandoValue{value: desc.Value, writable: writable, enumerable: enumerable, configurable: configurable}

			// For non-configurable properties, we must also define on the target object
			// to satisfy goja's proxy invariant checks
			if !configurable {
				target.DefineDataProperty(property, desc.Value, desc.Writable, desc.Enumerable, desc.Configurable)
			}
			return true
		},

		// DefinePropertyIdx for numeric indices
		// Note: Values outside valid array index range are treated as named properties.
		DefinePropertyIdx: func(target *goja.Object, idx int, desc goja.PropertyDescriptor) bool {
			const maxArrayIndex = 1<<32 - 2
			if idx < 0 || int64(idx) > maxArrayIndex {
				name := fmt.Sprintf("%d", idx)
				// Can't define properties that shadow existing named DOM properties
				if collection.NamedItem(name) != nil {
					return false
				}
				// Store as expando with all descriptor flags
				writable := desc.Writable == goja.FLAG_TRUE
				enumerable := desc.Enumerable == goja.FLAG_TRUE
				configurable := desc.Configurable == goja.FLAG_TRUE
				expandos[name] = expandoValue{value: desc.Value, writable: writable, enumerable: enumerable, configurable: configurable}

				// For non-configurable properties, define on target for proxy invariants
				if !configurable {
					target.DefineDataProperty(name, desc.Value, desc.Writable, desc.Enumerable, desc.Configurable)
				}
				return true
			}
			// Valid array indices always reject
			return false
		},
	})

	// Get the proxy object
	proxyObj := vm.ToValue(proxy).ToObject(vm)

	// Update the map to point to the proxy
	b.htmlCollectionMap[proxyObj] = collection

	return proxyObj
}

// BindDOMTokenList creates a JavaScript DOMTokenList object with dynamic numeric indexing.
// The binding is cached so that the same JS object is returned for the same DOMTokenList.
func (b *DOMBinder) BindDOMTokenList(tokenList *dom.DOMTokenList) *goja.Object {
	// Check cache first
	if cached, ok := b.domTokenListCache[tokenList]; ok {
		return cached
	}

	vm := b.runtime.vm
	jsList := vm.NewObject()

	jsList.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(tokenList.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsList.DefineAccessorProperty("value", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(tokenList.Value())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			tokenList.SetValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsList.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		token := tokenList.Item(index)
		if token == "" {
			return goja.Null()
		}
		return vm.ToValue(token)
	})

	jsList.Set("contains", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		token := call.Arguments[0].String()
		return vm.ToValue(tokenList.Contains(token))
	})

	jsList.Set("add", func(call goja.FunctionCall) goja.Value {
		tokens := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			tokens[i] = arg.String()
		}
		if err := tokenList.Add(tokens...); err != nil {
			panic(b.createDOMException(err.Type, err.Message))
		}
		return goja.Undefined()
	})

	jsList.Set("remove", func(call goja.FunctionCall) goja.Value {
		tokens := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			tokens[i] = arg.String()
		}
		if err := tokenList.Remove(tokens...); err != nil {
			panic(b.createDOMException(err.Type, err.Message))
		}
		return goja.Undefined()
	})

	jsList.Set("toggle", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		token := call.Arguments[0].String()
		var result bool
		var err *dom.TokenValidationError
		if len(call.Arguments) > 1 {
			force := call.Arguments[1].ToBoolean()
			result, err = tokenList.Toggle(token, force)
		} else {
			result, err = tokenList.Toggle(token)
		}
		if err != nil {
			panic(b.createDOMException(err.Type, err.Message))
		}
		return vm.ToValue(result)
	})

	jsList.Set("replace", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(false)
		}
		oldToken := call.Arguments[0].String()
		newToken := call.Arguments[1].String()
		result, err := tokenList.Replace(oldToken, newToken)
		if err != nil {
			panic(b.createDOMException(err.Type, err.Message))
		}
		return vm.ToValue(result)
	})

	jsList.Set("supports", func(call goja.FunctionCall) goja.Value {
		// classList doesn't have a defined set of supported tokens, so supports() throws TypeError
		// Per spec: https://dom.spec.whatwg.org/#dom-domtokenlist-supports
		panic(vm.NewTypeError("classList.supports is not supported"))
	})

	jsList.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(tokenList.Value())
	})

	// Add Array.prototype methods for iteration (per DOMTokenList spec)
	// These should be the actual Array.prototype methods
	// We need to set Symbol.iterator and the iteration methods (keys, values, entries, forEach)
	setupFn, _ := vm.RunString(`
		(function(list) {
			list[Symbol.iterator] = Array.prototype[Symbol.iterator];
			list.keys = Array.prototype.keys;
			list.values = Array.prototype.values;
			list.entries = Array.prototype.entries;
			list.forEach = Array.prototype.forEach;
		})
	`)
	if setupFn != nil && !goja.IsUndefined(setupFn) {
		if fn, ok := goja.AssertFunction(setupFn); ok {
			fn(goja.Undefined(), jsList)
		}
	}

	// Create a proxy to intercept numeric index access (e.g., classList[0])
	proxy := vm.NewProxy(jsList, &goja.ProxyTrapConfig{
		GetIdx: func(target *goja.Object, property int, receiver goja.Value) goja.Value {
			// Handle numeric index access: classList[0] returns token or undefined
			if property < 0 || property >= tokenList.Length() {
				return goja.Undefined()
			}
			token := tokenList.Item(property)
			return vm.ToValue(token)
		},
		HasIdx: func(target *goja.Object, property int) bool {
			// Check if numeric index exists
			return property >= 0 && property < tokenList.Length()
		},
		OwnKeys: func(target *goja.Object) *goja.Object {
			// Return array of keys including numeric indices
			length := tokenList.Length()
			keys := make([]interface{}, 0, length+5)
			// Add numeric indices first
			for i := 0; i < length; i++ {
				keys = append(keys, vm.ToValue(i).String())
			}
			// Add named properties
			keys = append(keys, "length", "value", "item", "contains", "add", "remove", "toggle", "replace", "supports", "toString", "forEach")
			return vm.ToValue(keys).ToObject(vm)
		},
		GetOwnPropertyDescriptorIdx: func(target *goja.Object, prop int) goja.PropertyDescriptor {
			if prop >= 0 && prop < tokenList.Length() {
				return goja.PropertyDescriptor{
					Value:        vm.ToValue(tokenList.Item(prop)),
					Writable:     goja.FLAG_FALSE,
					Enumerable:   goja.FLAG_TRUE,
					Configurable: goja.FLAG_TRUE,
				}
			}
			return goja.PropertyDescriptor{}
		},
	})

	// Get the proxy object
	proxyObj := vm.ToValue(proxy).ToObject(vm)

	// Cache the binding
	b.domTokenListCache[tokenList] = proxyObj

	return proxyObj
}

// createEmptyNodeList returns an empty NodeList-like object.
func (b *DOMBinder) createEmptyNodeList() *goja.Object {
	return b.bindStaticNodeList(nil)
}

// createEmptyHTMLCollection returns an empty HTMLCollection-like object.
func (b *DOMBinder) createEmptyHTMLCollection() *goja.Object {
	vm := b.runtime.vm
	jsCol := vm.NewObject()
	// Set prototype for instanceof to work - length, item, namedItem are on the prototype
	if b.htmlCollectionProto != nil {
		jsCol.SetPrototype(b.htmlCollectionProto)
	}
	// Register nil collection - prototype methods will handle this gracefully
	b.htmlCollectionMap[jsCol] = nil
	return jsCol
}

// ClearCache clears the node binding cache.
func (b *DOMBinder) ClearCache() {
	b.nodeMap = make(map[*dom.Node]*goja.Object)
}

// SetStyleResolver sets the style resolver for getComputedStyle.
func (b *DOMBinder) SetStyleResolver(sr *css.StyleResolver) {
	b.styleResolver = sr
}

// SetEventBinder sets the event binder for adding EventTarget methods to nodes.
func (b *DOMBinder) SetEventBinder(eb *EventBinder) {
	b.eventBinder = eb
}

// SetIframeContentProvider sets the callback for providing iframe contentWindow/contentDocument.
func (b *DOMBinder) SetIframeContentProvider(provider IframeContentProvider) {
	b.iframeContentProvider = provider
}

// SetMainDocument sets the document that is associated with the window.
// This document will have events bubble to the window object.
func (b *DOMBinder) SetMainDocument(doc *dom.Document) {
	b.mainDocument = doc
}

// IsMainDocument returns true if the given document is the main document
// associated with the window (for event bubbling purposes).
func (b *DOMBinder) IsMainDocument(doc *dom.Document) bool {
	return b.mainDocument != nil && b.mainDocument == doc
}

// GetComputedStyle returns the computed style for an element.
// This implements window.getComputedStyle().
func (b *DOMBinder) GetComputedStyle(el *dom.Element, pseudoElt string) *goja.Object {
	// Compute the style using the style resolver
	var computedStyle *css.ComputedStyle
	if b.styleResolver != nil {
		// Find parent computed style for inheritance
		var parentStyle *css.ComputedStyle
		if parentNode := el.AsNode().ParentNode(); parentNode != nil {
			// Check if parent is an element
			if parentNode.NodeType() == dom.ElementNode {
				parentEl := (*dom.Element)(parentNode)
				// Recursively get parent's computed style
				// For simplicity, we compute it on demand here
				parentStyle = b.styleResolver.ResolveStyles(parentEl, nil)
			}
		}
		computedStyle = b.styleResolver.ResolveStyles(el, parentStyle)
	}

	return b.bindComputedStyleDeclaration(computedStyle, el)
}

// bindComputedStyleDeclaration creates a read-only CSSStyleDeclaration for computed styles.
func (b *DOMBinder) bindComputedStyleDeclaration(cs *css.ComputedStyle, el *dom.Element) *goja.Object {
	vm := b.runtime.vm
	jsSD := vm.NewObject()

	// Set prototype if we have one
	if b.cssStyleDeclarationProto != nil {
		jsSD.SetPrototype(b.cssStyleDeclarationProto)
	}

	// Helper function to get property value as string
	getPropertyValue := func(property string) string {
		property = strings.ToLower(property)
		// Convert camelCase to kebab-case
		property = camelToKebab(property)

		if cs == nil {
			return ""
		}

		val := cs.GetPropertyValue(property)
		if val == nil {
			// Check for default value
			if def, ok := css.PropertyDefaults[property]; ok {
				return def.InitialValue
			}
			return ""
		}

		// Return the value as a string
		if val.Keyword != "" {
			return val.Keyword
		}
		if val.Value.Raw != "" {
			return val.Value.Raw
		}
		// For length values, format as pixels
		if val.Value.Type == css.LengthValue || val.Value.Type == css.NumberValue {
			return formatCSSLength(val.Length)
		}
		// For color values
		if val.Value.Type == css.ColorValue {
			return formatCSSColor(val.Color)
		}

		return ""
	}

	// Get all property names
	getPropertyNames := func() []string {
		// Return all known CSS properties
		names := make([]string, 0, len(css.PropertyDefaults))
		for prop := range css.PropertyDefaults {
			names = append(names, prop)
		}
		return names
	}

	// cssText property (read-only for computed styles)
	jsSD.DefineAccessorProperty("cssText", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		// Computed styles have empty cssText per spec
		return vm.ToValue("")
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// length property
	jsSD.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(len(css.PropertyDefaults))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// parentRule property (always null for computed styles)
	jsSD.DefineAccessorProperty("parentRule", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// item(index) method
	jsSD.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		index := int(call.Arguments[0].ToInteger())
		names := getPropertyNames()
		if index < 0 || index >= len(names) {
			return vm.ToValue("")
		}
		return vm.ToValue(names[index])
	})

	// getPropertyValue(property) method
	jsSD.Set("getPropertyValue", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		property := call.Arguments[0].String()
		return vm.ToValue(getPropertyValue(property))
	})

	// getPropertyPriority(property) method - always returns "" for computed styles
	jsSD.Set("getPropertyPriority", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue("")
	})

	// setProperty - no-op for computed styles (they're read-only)
	jsSD.Set("setProperty", func(call goja.FunctionCall) goja.Value {
		// Computed styles are read-only, silently ignore
		return goja.Undefined()
	})

	// removeProperty - no-op for computed styles
	jsSD.Set("removeProperty", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue("")
	})

	// Set up camelCase property access for common CSS properties
	b.setupComputedStylePropertyProxy(jsSD, getPropertyValue)

	return jsSD
}

// setupComputedStylePropertyProxy sets up read-only property access for computed styles.
func (b *DOMBinder) setupComputedStylePropertyProxy(jsSD *goja.Object, getPropertyValue func(string) string) {
	vm := b.runtime.vm

	// Common CSS properties to expose as camelCase
	cssProperties := []string{
		"alignContent", "alignItems", "alignSelf",
		"background", "backgroundColor", "backgroundImage", "backgroundPosition",
		"backgroundRepeat", "backgroundSize",
		"border", "borderBottom", "borderBottomColor", "borderBottomStyle",
		"borderBottomWidth", "borderCollapse", "borderColor", "borderLeft",
		"borderLeftColor", "borderLeftStyle", "borderLeftWidth", "borderRadius",
		"borderRight", "borderRightColor", "borderRightStyle", "borderRightWidth",
		"borderSpacing", "borderStyle", "borderTop", "borderTopColor",
		"borderTopStyle", "borderTopWidth", "borderWidth",
		"bottom", "boxSizing", "clear", "clip", "color", "content", "cursor",
		"direction", "display",
		"emptyCells",
		"flex", "flexBasis", "flexDirection", "flexFlow", "flexGrow", "flexShrink", "flexWrap",
		"float", "font", "fontFamily", "fontSize", "fontStyle", "fontVariant", "fontWeight",
		"gap", "gridColumn", "gridRow", "gridTemplateColumns", "gridTemplateRows",
		"height",
		"justifyContent",
		"left", "letterSpacing", "lineHeight", "listStyle", "listStyleImage",
		"listStylePosition", "listStyleType",
		"margin", "marginBottom", "marginLeft", "marginRight", "marginTop",
		"maxHeight", "maxWidth", "minHeight", "minWidth",
		"opacity", "order", "outline", "outlineColor", "outlineStyle", "outlineWidth",
		"overflow", "overflowX", "overflowY",
		"padding", "paddingBottom", "paddingLeft", "paddingRight", "paddingTop",
		"position",
		"quotes",
		"right",
		"tableLayout", "textAlign", "textDecoration", "textIndent", "textTransform",
		"top",
		"unicodeBidi",
		"verticalAlign", "visibility",
		"whiteSpace", "width", "wordSpacing",
		"zIndex",
	}

	for _, camelProp := range cssProperties {
		prop := camelProp // Capture for closure
		jsSD.DefineAccessorProperty(prop, vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(getPropertyValue(prop))
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
}

// formatCSSLength formats a length value for CSS output.
func formatCSSLength(length float64) string {
	if length == 0 {
		return "0px"
	}
	// Use integer if it's a whole number
	if length == float64(int64(length)) {
		return itoa(int(length)) + "px"
	}
	// Format with up to 2 decimal places
	return ftoa(length) + "px"
}

// formatCSSColor formats a color value for CSS output.
func formatCSSColor(c css.Color) string {
	if c.A == 255 {
		return "rgb(" + itoa(int(c.R)) + ", " + itoa(int(c.G)) + ", " + itoa(int(c.B)) + ")"
	}
	return "rgba(" + itoa(int(c.R)) + ", " + itoa(int(c.G)) + ", " + itoa(int(c.B)) + ", " + ftoa(float64(c.A)/255.0) + ")"
}

// itoa converts an int to string (simple implementation to avoid fmt import).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// ftoa converts a float to string with 2 decimal places.
func ftoa(f float64) string {
	// Simple implementation - multiply by 100, round, format
	i := int(f*100 + 0.5)
	whole := i / 100
	frac := i % 100
	if frac == 0 {
		return itoa(whole)
	}
	if frac%10 == 0 {
		return itoa(whole) + "." + itoa(frac/10)
	}
	fracStr := itoa(frac)
	if len(fracStr) == 1 {
		fracStr = "0" + fracStr
	}
	return itoa(whole) + "." + fracStr
}

// BindAttr creates a JavaScript object from a DOM Attr.
// Attr objects have specific properties: value, nodeValue, textContent, localName,
// namespaceURI, prefix, name, nodeName, specified, ownerElement.
func (b *DOMBinder) BindAttr(attr *dom.Attr) *goja.Object {
	if attr == nil {
		return nil
	}

	// Check cache for existing binding
	if cached, ok := b.attrCache[attr]; ok {
		return cached
	}

	vm := b.runtime.vm
	jsAttr := vm.NewObject()

	// Set prototype so instanceof Attr works
	if b.attrProto != nil {
		jsAttr.SetPrototype(b.attrProto)
	}

	// Store reference to Go Attr for extraction
	jsAttr.Set("_goAttr", attr)

	// Node-like properties
	jsAttr.Set("nodeType", int(dom.AttributeNode))
	jsAttr.Set("nodeName", attr.Name())

	// Value property with getter/setter
	jsAttr.DefineAccessorProperty("value", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.Value())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			attr.SetValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// nodeValue - same as value for Attr
	jsAttr.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.Value())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			attr.SetValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// textContent - same as value for Attr
	jsAttr.DefineAccessorProperty("textContent", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.Value())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			attr.SetValue(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Read-only properties
	jsAttr.DefineAccessorProperty("localName", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.LocalName())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsAttr.DefineAccessorProperty("namespaceURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		ns := attr.NamespaceURI()
		if ns == "" {
			return goja.Null()
		}
		return vm.ToValue(ns)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsAttr.DefineAccessorProperty("prefix", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		p := attr.Prefix()
		if p == "" {
			return goja.Null()
		}
		return vm.ToValue(p)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsAttr.DefineAccessorProperty("name", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.Name())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// specified is always true (historical)
	jsAttr.DefineAccessorProperty("specified", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(true)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ownerElement - null for newly created attributes
	jsAttr.DefineAccessorProperty("ownerElement", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		owner := attr.OwnerElement()
		if owner == nil {
			return goja.Null()
		}
		return b.BindElement(owner)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// ownerDocument - returns the document that owns this attribute
	jsAttr.DefineAccessorProperty("ownerDocument", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		doc := attr.OwnerDocument()
		if doc == nil {
			return goja.Null()
		}
		return b.BindDocument(doc)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// baseURI - returns the absolute base URL of this attribute
	jsAttr.DefineAccessorProperty("baseURI", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(attr.BaseURI())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// isSameNode - returns true if the given node is the same as this one
	jsAttr.Set("isSameNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) {
			return vm.ToValue(false)
		}
		otherObj := call.Arguments[0].ToObject(vm)
		otherAttr := b.getGoAttr(otherObj)
		if otherAttr == nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(attr == otherAttr)
	})

	// cloneNode - clones the attribute (deep param is ignored for Attr)
	jsAttr.Set("cloneNode", func(call goja.FunctionCall) goja.Value {
		// Preserve namespace information when cloning
		clonedAttr := dom.NewAttrNS(attr.NamespaceURI(), attr.Name(), attr.Value())
		return b.BindAttr(clonedAttr)
	})

	// lookupNamespaceURI - returns the namespace URI for the given prefix
	jsAttr.Set("lookupNamespaceURI", func(call goja.FunctionCall) goja.Value {
		var prefix string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			prefix = call.Arguments[0].String()
		}
		result := attr.LookupNamespaceURI(prefix)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})

	// isDefaultNamespace - checks if the given namespace URI is the default namespace
	jsAttr.Set("isDefaultNamespace", func(call goja.FunctionCall) goja.Value {
		var namespaceURI string
		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) && !goja.IsUndefined(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		return vm.ToValue(attr.IsDefaultNamespace(namespaceURI))
	})

	// lookupPrefix - returns the prefix associated with a given namespace URI
	jsAttr.Set("lookupPrefix", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
			return goja.Null()
		}
		namespaceURI := call.Arguments[0].String()
		result := attr.LookupPrefix(namespaceURI)
		if result == "" {
			return goja.Null()
		}
		return vm.ToValue(result)
	})

	// Store in cache
	b.attrCache[attr] = jsAttr

	return jsAttr
}

// convertJSNodesToGo converts JavaScript arguments to Go interface{} slice for ParentNode/ChildNode methods.
// These methods accept nodes or strings.
func (b *DOMBinder) convertJSNodesToGo(args []goja.Value) []interface{} {
	result := make([]interface{}, 0, len(args))
	for _, arg := range args {
		// null and undefined are converted to strings "null" and "undefined"
		if goja.IsNull(arg) {
			result = append(result, "null")
			continue
		}
		if goja.IsUndefined(arg) {
			result = append(result, "undefined")
			continue
		}
		// Check if it's a string
		if arg.ExportType().Kind().String() == "string" {
			result = append(result, arg.String())
			continue
		}
		// Try to get it as a node
		obj := arg.ToObject(b.runtime.vm)
		if node := b.getGoNode(obj); node != nil {
			result = append(result, node)
		} else {
			// Fallback: convert to string
			result = append(result, arg.String())
		}
	}
	return result
}

// BindCSSStyleDeclaration creates a JavaScript CSSStyleDeclaration object.
func (b *DOMBinder) BindCSSStyleDeclaration(sd *dom.CSSStyleDeclaration) *goja.Object {
	if sd == nil {
		return nil
	}

	// Check cache
	if jsObj, ok := b.styleDeclarationCache[sd]; ok {
		return jsObj
	}

	vm := b.runtime.vm
	jsSD := vm.NewObject()

	// Set prototype if we have one
	if b.cssStyleDeclarationProto != nil {
		jsSD.SetPrototype(b.cssStyleDeclarationProto)
	}

	// Store reference to the Go object
	jsSD.Set("_goStyleDeclaration", sd)

	// cssText property (getter and setter)
	jsSD.DefineAccessorProperty("cssText", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sd.CSSText())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			sd.SetCSSText(call.Arguments[0].String())
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// length property (getter only)
	jsSD.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sd.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// parentRule property (always null for inline styles)
	jsSD.DefineAccessorProperty("parentRule", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// item(index) method
	jsSD.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		index := int(call.Arguments[0].ToInteger())
		return vm.ToValue(sd.Item(index))
	})

	// getPropertyValue(property) method
	jsSD.Set("getPropertyValue", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		property := call.Arguments[0].String()
		return vm.ToValue(sd.GetPropertyValue(property))
	})

	// getPropertyPriority(property) method
	jsSD.Set("getPropertyPriority", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		property := call.Arguments[0].String()
		return vm.ToValue(sd.GetPropertyPriority(property))
	})

	// setProperty(property, value, priority) method
	jsSD.Set("setProperty", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		property := call.Arguments[0].String()
		value := call.Arguments[1].String()
		priority := ""
		if len(call.Arguments) > 2 {
			priority = call.Arguments[2].String()
		}
		sd.SetProperty(property, value, priority)
		return goja.Undefined()
	})

	// removeProperty(property) method
	jsSD.Set("removeProperty", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		property := call.Arguments[0].String()
		return vm.ToValue(sd.RemoveProperty(property))
	})

	// Set up a Proxy to handle camelCase property access (e.g., element.style.backgroundColor)
	// For goja, we use the dynamic approach with __get and __set
	b.setupStylePropertyProxy(jsSD, sd)

	// Cache the object
	b.styleDeclarationCache[sd] = jsSD

	return jsSD
}

// setupStylePropertyProxy sets up dynamic property access for CSS properties on a style object.
// This allows accessing properties like element.style.backgroundColor or element.style["background-color"].
func (b *DOMBinder) setupStylePropertyProxy(jsSD *goja.Object, sd *dom.CSSStyleDeclaration) {
	vm := b.runtime.vm

	// Common CSS properties to expose as camelCase (a subset)
	cssProperties := []string{
		"alignContent", "alignItems", "alignSelf", "animation", "animationDelay",
		"animationDirection", "animationDuration", "animationFillMode", "animationIterationCount",
		"animationName", "animationPlayState", "animationTimingFunction",
		"background", "backgroundAttachment", "backgroundClip", "backgroundColor",
		"backgroundImage", "backgroundOrigin", "backgroundPosition", "backgroundRepeat",
		"backgroundSize", "border", "borderBottom", "borderBottomColor", "borderBottomLeftRadius",
		"borderBottomRightRadius", "borderBottomStyle", "borderBottomWidth", "borderCollapse",
		"borderColor", "borderImage", "borderLeft", "borderLeftColor", "borderLeftStyle",
		"borderLeftWidth", "borderRadius", "borderRight", "borderRightColor", "borderRightStyle",
		"borderRightWidth", "borderSpacing", "borderStyle", "borderTop", "borderTopColor",
		"borderTopLeftRadius", "borderTopRightRadius", "borderTopStyle", "borderTopWidth",
		"borderWidth", "bottom", "boxShadow", "boxSizing", "captionSide", "clear",
		"clip", "color", "columnCount", "columnFill", "columnGap", "columnRule",
		"columnRuleColor", "columnRuleStyle", "columnRuleWidth", "columns", "columnSpan",
		"columnWidth", "content", "counterIncrement", "counterReset", "cursor", "direction",
		"display", "emptyCells", "flex", "flexBasis", "flexDirection", "flexFlow",
		"flexGrow", "flexShrink", "flexWrap", "float", "font", "fontFamily",
		"fontSize", "fontSizeAdjust", "fontStretch", "fontStyle", "fontVariant",
		"fontWeight", "gap", "grid", "gridArea", "gridAutoColumns", "gridAutoFlow",
		"gridAutoRows", "gridColumn", "gridColumnEnd", "gridColumnStart", "gridGap",
		"gridRow", "gridRowEnd", "gridRowStart", "gridTemplate", "gridTemplateAreas",
		"gridTemplateColumns", "gridTemplateRows", "height", "justifyContent", "left",
		"letterSpacing", "lineHeight", "listStyle", "listStyleImage", "listStylePosition",
		"listStyleType", "margin", "marginBottom", "marginLeft", "marginRight", "marginTop",
		"maxHeight", "maxWidth", "minHeight", "minWidth", "objectFit", "objectPosition",
		"opacity", "order", "orphans", "outline", "outlineColor", "outlineOffset",
		"outlineStyle", "outlineWidth", "overflow", "overflowX", "overflowY", "padding",
		"paddingBottom", "paddingLeft", "paddingRight", "paddingTop", "pageBreakAfter",
		"pageBreakBefore", "pageBreakInside", "perspective", "perspectiveOrigin",
		"placeContent", "placeItems", "placeSelf", "pointerEvents", "position", "quotes",
		"resize", "right", "rowGap", "tableLayout", "textAlign", "textAlignLast",
		"textDecoration", "textDecorationColor", "textDecorationLine", "textDecorationStyle",
		"textIndent", "textOverflow", "textShadow", "textTransform", "top", "transform",
		"transformOrigin", "transformStyle", "transition", "transitionDelay",
		"transitionDuration", "transitionProperty", "transitionTimingFunction",
		"unicodeBidi", "userSelect", "verticalAlign", "visibility", "whiteSpace",
		"widows", "width", "wordBreak", "wordSpacing", "wordWrap", "zIndex",
		// Vendor-prefixed properties
		"WebkitAnimation", "WebkitTransform", "WebkitTransition", "MozAnimation",
		"MozTransform", "MozTransition", "msAnimation", "msTransform", "msTransition",
	}

	// Set up getter/setter for each CSS property
	for _, propName := range cssProperties {
		prop := propName // Capture for closure
		jsSD.DefineAccessorProperty(prop, vm.ToValue(func(call goja.FunctionCall) goja.Value {
			// Convert camelCase to kebab-case for lookup
			kebabName := camelToKebab(prop)
			return vm.ToValue(sd.GetPropertyValue(kebabName))
		}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 {
				kebabName := camelToKebab(prop)
				value := call.Arguments[0].String()
				if value == "" {
					sd.RemoveProperty(kebabName)
				} else {
					sd.SetProperty(kebabName, value)
				}
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
}

// camelToKebab converts camelCase to kebab-case.
// Examples: "backgroundColor" -> "background-color", "WebkitTransform" -> "-webkit-transform"
func camelToKebab(s string) string {
	if s == "" {
		return ""
	}

	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '-')
			}
			result = append(result, byte(r-'A'+'a'))
		} else {
			result = append(result, byte(r))
		}
	}

	// Handle vendor prefixes (Webkit, Moz, ms, O -> -webkit, -moz, -ms, -o)
	str := string(result)
	prefixes := []string{"webkit-", "moz-", "ms-", "o-"}
	for _, prefix := range prefixes {
		if len(str) > len(prefix) && str[:len(prefix)] == prefix {
			return "-" + str
		}
	}

	return str
}

// BindNamedNodeMap creates a JavaScript object from a DOM NamedNodeMap.
// The NamedNodeMap is array-like with indexed access and methods like item(), getNamedItem().
// Per the WebIDL spec, only numeric indices and named properties are own properties,
// while methods like item() and getNamedItem() are on the prototype.
func (b *DOMBinder) BindNamedNodeMap(nnm *dom.NamedNodeMap) *goja.Object {
	if nnm == nil {
		return nil
	}

	vm := b.runtime.vm
	jsMap := vm.NewObject()

	// Set prototype for instanceof and method access
	if b.namedNodeMapProto != nil {
		jsMap.SetPrototype(b.namedNodeMapProto)
	}

	// Register the NamedNodeMap in our internal map for prototype methods
	b.namedNodeMapMap[jsMap] = nnm

	// Create a proxy to control property enumeration
	// Per WPT:
	// - Enumerable own properties: only numeric indices (e.g., ["0", "1"])
	// - Object.getOwnPropertyNames: numeric indices + named properties (attribute names)
	// - Named properties: for HTML elements in HTML docs, only lowercase names
	proxy := vm.NewProxy(jsMap, &goja.ProxyTrapConfig{
		// Get handles property access for numeric indices and named properties
		Get: func(target *goja.Object, property string, receiver goja.Value) (value goja.Value) {
			// Try numeric index first
			if idx, isNum := parseNumericIndex(property); isNum {
				if idx >= 0 && idx < nnm.Length() {
					attr := nnm.Item(idx)
					if attr != nil {
						return b.BindAttr(attr)
					}
				}
				return goja.Undefined()
			}

			// Per WebIDL, check prototype chain FIRST for built-in properties
			// Named properties should not shadow prototype properties (methods like item, setNamedItem, etc.)
			// or special properties like "length"
			protoVal := target.Get(property)
			if protoVal != nil && !goja.IsUndefined(protoVal) {
				return protoVal
			}

			// Only if property is not on prototype, try named property (attribute name)
			attr := nnm.GetNamedItem(property)
			if attr != nil {
				return b.BindAttr(attr)
			}

			// Property not found
			return goja.Undefined()
		},

		// OwnKeys returns only numeric indices and named attribute properties
		OwnKeys: func(target *goja.Object) *goja.Object {
			keys := make([]interface{}, 0)

			// Add numeric indices
			for i := 0; i < nnm.Length(); i++ {
				keys = append(keys, vm.ToValue(i).String())
			}

			// Add named properties (attribute names)
			// For HTML elements in HTML documents, only lowercase names are exposed
			seen := make(map[string]bool)
			ownerEl := nnm.OwnerElement()
			isHTMLElement := ownerEl != nil && ownerEl.NamespaceURI() == dom.HTMLNamespace
			isHTMLDoc := ownerEl != nil && ownerEl.AsNode().OwnerDocument() != nil && ownerEl.AsNode().OwnerDocument().ContentType() == "text/html"

			for i := 0; i < nnm.Length(); i++ {
				attr := nnm.Item(i)
				if attr == nil {
					continue
				}
				name := attr.Name()

				// Skip if already seen
				if seen[name] {
					continue
				}

				// For HTML elements in HTML documents, only expose lowercase names
				if isHTMLElement && isHTMLDoc {
					if !isLowercase(name) {
						continue
					}
				}

				seen[name] = true
				keys = append(keys, name)
			}

			return vm.ToValue(keys).ToObject(vm)
		},

		// GetOwnPropertyDescriptor returns descriptors for own properties
		GetOwnPropertyDescriptor: func(target *goja.Object, prop string) goja.PropertyDescriptor {
			// Check numeric index
			if idx, isNum := parseNumericIndex(prop); isNum && idx >= 0 && idx < nnm.Length() {
				attr := nnm.Item(idx)
				if attr != nil {
					return goja.PropertyDescriptor{
						Value:        b.BindAttr(attr),
						Writable:     goja.FLAG_FALSE,
						Enumerable:   goja.FLAG_TRUE, // Numeric indices are enumerable
						Configurable: goja.FLAG_TRUE,
					}
				}
			}

			// Check named property
			attr := nnm.GetNamedItem(prop)
			if attr != nil {
				return goja.PropertyDescriptor{
					Value:        b.BindAttr(attr),
					Writable:     goja.FLAG_FALSE,
					Enumerable:   goja.FLAG_FALSE, // Named properties are NOT enumerable
					Configurable: goja.FLAG_TRUE,
				}
			}

			// Return empty for non-existent properties
			return goja.PropertyDescriptor{}
		},
	})

	// Get the proxy as an object and also register it in the map
	// so that prototype methods can find the NamedNodeMap
	proxyObj := vm.ToValue(proxy).ToObject(vm)
	b.namedNodeMapMap[proxyObj] = nnm

	return proxyObj
}

// parseNumericIndex parses a string as a numeric index.
// Returns the index and true if it's a valid non-negative integer.
func parseNumericIndex(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	// Check for leading zeros (invalid except for "0")
	if len(s) > 1 && s[0] == '0' {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// isLowercase returns true if the string contains only lowercase ASCII letters,
// digits, and allowed characters like '-' and ':'.
func isLowercase(s string) bool {
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

// extractAttr extracts a Go *dom.Attr from a JavaScript Attr object.
func (b *DOMBinder) extractAttr(jsAttr *goja.Object) *dom.Attr {
	if jsAttr == nil {
		return nil
	}
	// Try to get internal reference first
	if goAttr, ok := jsAttr.Get("_goAttr").Export().(*dom.Attr); ok {
		return goAttr
	}
	// If not available, we can't extract
	return nil
}

// BindDOMRect creates a JavaScript DOMRect object from a Go DOMRect.
func (b *DOMBinder) BindDOMRect(rect *dom.DOMRect) goja.Value {
	if rect == nil {
		return goja.Null()
	}

	vm := b.runtime.vm
	jsRect := vm.NewObject()

	// Store the Go rect for internal access
	jsRect.Set("_goRect", rect)

	// Define the x, y, width, height properties (these are read/write in DOMRect)
	jsRect.DefineAccessorProperty("x", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.X)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			rect.X = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("y", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Y)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			rect.Y = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("width", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Width)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			rect.Width = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("height", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Height)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			rect.Height = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Computed properties (read-only)
	jsRect.DefineAccessorProperty("top", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Top())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("right", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Right())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("bottom", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Bottom())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsRect.DefineAccessorProperty("left", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(rect.Left())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// toJSON method
	jsRect.Set("toJSON", func(call goja.FunctionCall) goja.Value {
		obj := vm.NewObject()
		obj.Set("x", rect.X)
		obj.Set("y", rect.Y)
		obj.Set("width", rect.Width)
		obj.Set("height", rect.Height)
		obj.Set("top", rect.Top())
		obj.Set("right", rect.Right())
		obj.Set("bottom", rect.Bottom())
		obj.Set("left", rect.Left())
		return obj
	})

	return jsRect
}

// BindDOMRectList creates a JavaScript DOMRectList object from a Go DOMRectList.
func (b *DOMBinder) BindDOMRectList(list *dom.DOMRectList) goja.Value {
	if list == nil {
		return goja.Null()
	}

	vm := b.runtime.vm
	jsList := vm.NewObject()

	// length property
	jsList.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(list.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// item method
	jsList.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		rect := list.Item(index)
		if rect == nil {
			return goja.Null()
		}
		return b.BindDOMRect(rect)
	})

	// Add indexed properties for array-like access
	for i := 0; i < list.Length(); i++ {
		idx := i // capture for closure
		jsList.DefineAccessorProperty(fmt.Sprintf("%d", i), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			rect := list.Item(idx)
			if rect == nil {
				return goja.Undefined()
			}
			return b.BindDOMRect(rect)
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}

	return jsList
}

// Event handler attribute names (GlobalEventHandlers + WindowEventHandlers + DocumentAndElementEventHandlers)
var eventHandlerAttributes = []string{
	// GlobalEventHandlers
	"onabort", "onauxclick", "onbeforeinput", "onbeforetoggle", "onblur", "oncancel",
	"oncanplay", "oncanplaythrough", "onchange", "onclick", "onclose", "oncontextlost",
	"oncontextmenu", "oncontextrestored", "oncopy", "oncuechange", "oncut", "ondblclick",
	"ondrag", "ondragend", "ondragenter", "ondragleave", "ondragover", "ondragstart",
	"ondrop", "ondurationchange", "onemptied", "onended", "onerror", "onfocus",
	"onformdata", "ongotpointercapture", "oninput", "oninvalid", "onkeydown", "onkeypress",
	"onkeyup", "onload", "onloadeddata", "onloadedmetadata", "onloadstart",
	"onlostpointercapture", "onmousedown", "onmouseenter", "onmouseleave", "onmousemove",
	"onmouseout", "onmouseover", "onmouseup", "onpaste", "onpause", "onplay", "onplaying",
	"onpointercancel", "onpointerdown", "onpointerenter", "onpointerleave", "onpointermove",
	"onpointerout", "onpointerover", "onpointerup", "onprogress", "onratechange", "onreset",
	"onresize", "onscroll", "onscrollend", "onsecuritypolicyviolation", "onseeked",
	"onseeking", "onselect", "onslotchange", "onstalled", "onsubmit", "onsuspend",
	"ontimeupdate", "ontoggle", "onvolumechange", "onwaiting", "onwebkitanimationend",
	"onwebkitanimationiteration", "onwebkitanimationstart", "onwebkittransitionend",
	"onwheel",
	// WindowEventHandlers (subset that applies to elements via body/frameset)
	"onafterprint", "onbeforeprint", "onbeforeunload", "onhashchange", "onlanguagechange",
	"onmessage", "onmessageerror", "onoffline", "ononline", "onpagehide", "onpageshow",
	"onpopstate", "onrejectionhandled", "onstorage", "onunhandledrejection", "onunload",
}

// bindEventHandlerAttributes adds event handler IDL attributes (onclick, onload, etc.) to a JS object.
// This implements the HTML spec's event handler content attributes and IDL attributes.
func (b *DOMBinder) bindEventHandlerAttributes(jsObj *goja.Object) {
	vm := b.runtime.vm

	// Initialize handler storage for this object
	if _, exists := b.eventHandlers[jsObj]; !exists {
		b.eventHandlers[jsObj] = make(map[string]goja.Value)
	}

	for _, attrName := range eventHandlerAttributes {
		eventType := strings.TrimPrefix(attrName, "on")
		attrNameCopy := attrName
		eventTypeCopy := eventType

		// Define getter and setter for the event handler property
		jsObj.DefineAccessorProperty(attrNameCopy,
			// Getter
			vm.ToValue(func(call goja.FunctionCall) goja.Value {
				handlers := b.eventHandlers[jsObj]
				if handlers == nil {
					return goja.Null()
				}
				handler, exists := handlers[eventTypeCopy]
				if !exists {
					return goja.Null()
				}
				return handler
			}),
			// Setter
			vm.ToValue(func(call goja.FunctionCall) goja.Value {
				handlers := b.eventHandlers[jsObj]
				if handlers == nil {
					handlers = make(map[string]goja.Value)
					b.eventHandlers[jsObj] = handlers
				}

				// Remove old handler if exists
				if oldHandler, exists := handlers[eventTypeCopy]; exists && oldHandler != nil && !goja.IsNull(oldHandler) {
					// Remove the old event listener
					if b.eventBinder != nil {
						target := b.eventBinder.GetOrCreateTarget(jsObj)
						target.RemoveEventListener(eventTypeCopy, oldHandler, false)
					}
				}

				if len(call.Arguments) == 0 || goja.IsNull(call.Arguments[0]) || goja.IsUndefined(call.Arguments[0]) {
					// Clear the handler
					delete(handlers, eventTypeCopy)
					return goja.Undefined()
				}

				newHandler := call.Arguments[0]

				// Check if the value is callable
				callable, ok := goja.AssertFunction(newHandler)
				if !ok {
					// Per spec, non-function values are ignored (handler set to null)
					delete(handlers, eventTypeCopy)
					return goja.Undefined()
				}

				// Store the handler
				handlers[eventTypeCopy] = newHandler

				// Add as event listener (these are always functions, not objects)
				if b.eventBinder != nil {
					target := b.eventBinder.GetOrCreateTarget(jsObj)
					target.AddEventListener(eventTypeCopy, callable, newHandler, false, nil, listenerOptions{})
				}

				return goja.Undefined()
			}),
			goja.FLAG_FALSE, goja.FLAG_TRUE,
		)
	}
}

// processEventHandlerContentAttributes checks for existing event handler content attributes
// (like oninput="...", onclick="...") and converts them to event listeners.
// This is called during element binding to handle elements created from HTML or cloned.
func (b *DOMBinder) processEventHandlerContentAttributes(jsEl *goja.Object, el *dom.Element) {
	vm := b.runtime.vm

	// Check each known event handler attribute
	for _, attrName := range eventHandlerAttributes {
		// Check if the element has this attribute set in the DOM
		if !el.HasAttribute(attrName) {
			continue
		}

		attrValue := el.GetAttribute(attrName)
		if attrValue == "" {
			continue
		}

		// Create an event handler function per HTML spec
		// The handler should have 'this' bound to the element and scope chain including the element
		handlerCode := fmt.Sprintf(`(function(__handler_el__) {
			return function(event) {
				with (__handler_el__) {
					%s
				}
			};
		})`, attrValue)

		handlerFactory, err := vm.RunString(handlerCode)
		if err != nil {
			continue
		}

		factory, ok := goja.AssertFunction(handlerFactory)
		if !ok {
			continue
		}

		handlerVal, callErr := factory(goja.Undefined(), jsEl)
		if callErr != nil {
			continue
		}

		// Set the event handler via the IDL property (e.g., element.oninput = handlerVal)
		jsEl.Set(attrName, handlerVal)
	}
}
