package js

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
	"github.com/dop251/goja"
)

// utf16Length returns the length of a string in UTF-16 code units.
// This matches JavaScript's String.length behavior.
func utf16Length(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// utf16Substring extracts a substring using UTF-16 code unit offsets.
// This matches JavaScript's String.substring behavior for proper Unicode handling.
func utf16Substring(s string, offset, count int) string {
	codeUnits := utf16.Encode([]rune(s))
	if offset >= len(codeUnits) {
		return ""
	}
	end := offset + count
	if end > len(codeUnits) {
		end = len(codeUnits)
	}
	// Convert back to string
	return string(utf16.Decode(codeUnits[offset:end]))
}

// utf16ReplaceRange replaces a range of UTF-16 code units in a string.
func utf16ReplaceRange(s string, offset, count int, replacement string) string {
	codeUnits := utf16.Encode([]rune(s))
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
	replacementUnits := utf16.Encode([]rune(replacement))

	result := make([]uint16, 0, len(before)+len(replacementUnits)+len(after))
	result = append(result, before...)
	result = append(result, replacementUnits...)
	result = append(result, after...)

	return string(utf16.Decode(result))
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

	// Create Document prototype (extends Node)
	b.documentProto = vm.NewObject()
	b.documentProto.SetPrototype(b.nodeProto)
	documentConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		// Per DOM spec, new Document() creates a new Document
		doc := dom.NewDocument()
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
		frag := dom.NewDocumentFragment()
		return b.BindDocumentFragment(frag)
	})
	docFragConstructorObj := docFragConstructor.ToObject(vm)
	docFragConstructorObj.Set("prototype", b.documentFragmentProto)
	b.documentFragmentProto.Set("constructor", docFragConstructorObj)
	vm.Set("DocumentFragment", docFragConstructorObj)

	// Create DOMImplementation prototype
	b.domImplementationProto = vm.NewObject()
	domImplConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	domImplConstructorObj := domImplConstructor.ToObject(vm)
	domImplConstructorObj.Set("prototype", b.domImplementationProto)
	b.domImplementationProto.Set("constructor", domImplConstructorObj)
	vm.Set("DOMImplementation", domImplConstructorObj)

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
	b.htmlCollectionProto.Set("item", func(call goja.FunctionCall) goja.Value {
		col := getCollection(call.This.ToObject(vm))
		if col == nil {
			return goja.Null()
		}
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
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

	// Create NodeList prototype
	b.nodeListProto = vm.NewObject()
	nodeListConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	nodeListConstructorObj := nodeListConstructor.ToObject(vm)
	nodeListConstructorObj.Set("prototype", b.nodeListProto)
	b.nodeListProto.Set("constructor", nodeListConstructorObj)
	vm.Set("NodeList", nodeListConstructorObj)

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
	"font":   "HTMLFontElement",
	"center": "HTMLElement",

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

// getHTMLElementPrototype returns the appropriate prototype for a given tag name.
func (b *DOMBinder) getHTMLElementPrototype(tagName string) *goja.Object {
	lowerTag := strings.ToLower(tagName)

	// Look up the constructor name for this tag
	constructorName, ok := htmlElementTypeMap[lowerTag]
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

	// Store current document for creating new nodes
	b.document = doc

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

	jsDoc.Set("getElementsByClassName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		classNames := call.Arguments[0].String()
		collection := doc.GetElementsByClassName(classNames)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		el := doc.QuerySelector(selector)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsDoc.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		nodeList := doc.QuerySelectorAll(selector)
		return b.BindNodeList(nodeList)
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
		attr := doc.CreateAttributeNS(namespaceURI, qualifiedName)
		return b.BindAttr(attr)
	})

	jsDoc.Set("importNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)
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
		adopted := doc.AdoptNode(goNode)
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
	// Note: Only set global document for the first/main document
	b.runtime.setDocumentDirect(jsDoc)
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

	jsDoc.Set("getElementsByClassName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		classNames := call.Arguments[0].String()
		collection := doc.GetElementsByClassName(classNames)
		return b.BindHTMLCollection(collection)
	})

	jsDoc.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		el := doc.QuerySelector(selector)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsDoc.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		nodeList := doc.QuerySelectorAll(selector)
		return b.BindNodeList(nodeList)
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
		attr := doc.CreateAttributeNS(namespaceURI, qualifiedName)
		return b.BindAttr(attr)
	})

	jsDoc.Set("importNode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		nodeObj := call.Arguments[0].ToObject(vm)
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
		adopted := doc.AdoptNode(goNode)
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
	// Use specific HTML element prototype for HTML elements
	ns := el.NamespaceURI()
	if ns == "" || ns == dom.HTMLNamespace {
		// HTML element - use specific HTML element prototype
		jsEl.SetPrototype(b.getHTMLElementPrototype(el.LocalName()))
	} else if b.elementProto != nil {
		// Non-HTML element (like SVG) - use generic Element prototype
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

	// Query methods
	jsEl.Set("querySelector", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		found := el.QuerySelector(selector)
		if found == nil {
			return goja.Null()
		}
		return b.BindElement(found)
	})

	jsEl.Set("querySelectorAll", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyNodeList()
		}
		selector := call.Arguments[0].String()
		nodeList := el.QuerySelectorAll(selector)
		return b.BindNodeList(nodeList)
	})

	jsEl.Set("getElementsByTagName", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return b.createEmptyHTMLCollection()
		}
		tagName := call.Arguments[0].String()
		collection := el.GetElementsByTagName(tagName)
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

	jsEl.Set("matches", func(call goja.FunctionCall) goja.Value {
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
	})

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

	// Add iframe-specific properties (contentWindow, contentDocument, src)
	if el.LocalName() == "iframe" {
		b.bindIframeProperties(jsEl, el)
	}

	// Bind common node properties and methods
	b.bindNodeProperties(jsEl, node)

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
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
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
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
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
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
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
	jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsNode.DefineAccessorProperty("nodeValue", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(node.NodeValue())
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := ""
			if !goja.IsNull(call.Arguments[0]) {
				val = call.Arguments[0].String()
			}
			node.SetNodeValue(val)
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
func (b *DOMBinder) bindCharacterDataMethods(jsNode *goja.Object, node *dom.Node) {
	vm := b.runtime.vm

	jsNode.Set("substringData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'substringData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])

		data := node.NodeValue()
		length := uint32(utf16Length(data))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		return vm.ToValue(utf16Substring(data, int(offset), int(count)))
	})

	jsNode.Set("appendData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 1 argument
		if len(call.Arguments) < 1 {
			panic(vm.NewTypeError("Failed to execute 'appendData' on 'CharacterData': 1 argument required"))
		}

		data := call.Arguments[0].String()
		node.SetNodeValue(node.NodeValue() + data)
		return goja.Undefined()
	})

	jsNode.Set("insertData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'insertData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		data := call.Arguments[1].String()

		current := node.NodeValue()
		length := uint32(utf16Length(current))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Insert at offset
		newValue := utf16ReplaceRange(current, int(offset), 0, data)
		node.SetNodeValue(newValue)
		return goja.Undefined()
	})

	jsNode.Set("deleteData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 2 arguments
		if len(call.Arguments) < 2 {
			panic(vm.NewTypeError("Failed to execute 'deleteData' on 'CharacterData': 2 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])

		current := node.NodeValue()
		length := uint32(utf16Length(current))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Delete from offset
		newValue := utf16ReplaceRange(current, int(offset), int(count), "")
		node.SetNodeValue(newValue)
		return goja.Undefined()
	})

	jsNode.Set("replaceData", func(call goja.FunctionCall) goja.Value {
		// Per spec: requires 3 arguments
		if len(call.Arguments) < 3 {
			panic(vm.NewTypeError("Failed to execute 'replaceData' on 'CharacterData': 3 arguments required"))
		}

		offset := toUint32(call.Arguments[0])
		count := toUint32(call.Arguments[1])
		data := call.Arguments[2].String()

		current := node.NodeValue()
		length := uint32(utf16Length(current))

		// Check offset bounds
		if offset > length {
			b.throwIndexSizeError(vm)
		}

		// Replace at offset
		newValue := utf16ReplaceRange(current, int(offset), int(count), data)
		node.SetNodeValue(newValue)
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
		// Remove from cache since it's been detached
		delete(b.nodeMap, result)
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
		root := node.GetRootNode()
		if root == nil {
			return goja.Null()
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
		delete(b.nodeMap, result)
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
		root := node.GetRootNode()
		if root == nil {
			return goja.Null()
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
	target.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(nodeList.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

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
		prop := call.Arguments[1]
		propStr := prop.String()

		// Check if it's a numeric index
		if isNumericString(propStr) {
			index := int(prop.ToInteger())
			return vm.ToValue(index >= 0 && index < nodeList.Length())
		}

		// Check if target has the property
		return vm.ToValue(target.Get(propStr) != nil)
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
		} else if propStr == "length" {
			desc := vm.NewObject()
			desc.Set("value", nodeList.Length())
			desc.Set("writable", false)
			desc.Set("enumerable", false)
			desc.Set("configurable", false)
			return desc
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
func (b *DOMBinder) BindHTMLCollection(collection *dom.HTMLCollection) *goja.Object {
	vm := b.runtime.vm
	jsCol := vm.NewObject()

	// Set prototype for instanceof to work
	if b.htmlCollectionProto != nil {
		jsCol.SetPrototype(b.htmlCollectionProto)
	}

	// Register the collection in our internal map for prototype methods
	b.htmlCollectionMap[jsCol] = collection

	// Array-like indexed properties - these should be enumerable
	// Note: This snapshot may become stale for live collections
	length := collection.Length()
	for i := 0; i < length; i++ {
		idx := i
		jsCol.DefineAccessorProperty(vm.ToValue(idx).String(), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			el := collection.Item(idx)
			if el == nil {
				return goja.Undefined()
			}
			return b.BindElement(el)
		}), nil, goja.FLAG_TRUE, goja.FLAG_TRUE)
	}

	// Named properties - non-enumerable own properties (in tree order)
	namedProps := collection.NamedProperties()
	for _, prop := range namedProps {
		// Don't overwrite numeric indices - check if name is a valid array index
		isNumeric := true
		for _, c := range prop.Name {
			if c < '0' || c > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric && len(prop.Name) > 0 {
			continue
		}
		boundEl := b.BindElement(prop.Element)
		jsCol.DefineDataProperty(prop.Name, boundEl, goja.FLAG_TRUE, goja.FLAG_TRUE, goja.FLAG_FALSE)
	}

	return jsCol
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

			// Try named property (attribute name)
			attr := nnm.GetNamedItem(property)
			if attr != nil {
				return b.BindAttr(attr)
			}

			// Fall back to prototype chain (for methods like item, getNamedItem, etc.)
			return target.Get(property)
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

	return vm.ToValue(proxy).ToObject(vm)
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
