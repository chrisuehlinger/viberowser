package js

import (
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
type DOMBinder struct {
	runtime  *Runtime
	nodeMap  map[*dom.Node]*goja.Object // Cache to return same JS object for same DOM node
	document *dom.Document              // Current document for creating new nodes

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
	documentProto                *goja.Object
	documentFragmentProto        *goja.Object
	domExceptionProto            *goja.Object
	domImplementationProto       *goja.Object
	htmlCollectionProto          *goja.Object
	nodeListProto                *goja.Object
	cssStyleDeclarationProto     *goja.Object
	domImplementationCache       map[*dom.DOMImplementation]*goja.Object
	styleDeclarationCache        map[*dom.CSSStyleDeclaration]*goja.Object
}

// NewDOMBinder creates a new DOM binder for the given runtime.
func NewDOMBinder(runtime *Runtime) *DOMBinder {
	b := &DOMBinder{
		runtime:                runtime,
		nodeMap:                make(map[*dom.Node]*goja.Object),
		domImplementationCache: make(map[*dom.DOMImplementation]*goja.Object),
		styleDeclarationCache:  make(map[*dom.CSSStyleDeclaration]*goja.Object),
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

	// Set Node constants on the constructor
	nodeConstructorObj.Set("ELEMENT_NODE", int(dom.ElementNode))
	nodeConstructorObj.Set("TEXT_NODE", int(dom.TextNode))
	nodeConstructorObj.Set("COMMENT_NODE", int(dom.CommentNode))
	nodeConstructorObj.Set("DOCUMENT_NODE", int(dom.DocumentNode))
	nodeConstructorObj.Set("DOCUMENT_FRAGMENT_NODE", int(dom.DocumentFragmentNode))
	nodeConstructorObj.Set("DOCUMENT_TYPE_NODE", int(dom.DocumentTypeNode))

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

	// Create Document prototype (extends Node)
	b.documentProto = vm.NewObject()
	b.documentProto.SetPrototype(b.nodeProto)
	documentConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	documentConstructorObj := documentConstructor.ToObject(vm)
	documentConstructorObj.Set("prototype", b.documentProto)
	b.documentProto.Set("constructor", documentConstructorObj)
	vm.Set("Document", documentConstructorObj)

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

	// Create HTMLCollection prototype
	b.htmlCollectionProto = vm.NewObject()
	htmlCollectionConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})
	htmlCollectionConstructorObj := htmlCollectionConstructor.ToObject(vm)
	htmlCollectionConstructorObj.Set("prototype", b.htmlCollectionProto)
	b.htmlCollectionProto.Set("constructor", htmlCollectionConstructorObj)
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
		el := doc.CreateElement(tagName)
		return b.BindElement(el)
	})

	jsDoc.Set("createElementNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		el := doc.CreateElementNS(namespaceURI, qualifiedName)
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
		doc.Append(nodes...)
		return goja.Undefined()
	})

	jsDoc.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		doc.Prepend(nodes...)
		return goja.Undefined()
	})

	jsDoc.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		doc.ReplaceChildren(nodes...)
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
	vm := b.runtime.vm
	jsDoc := vm.NewObject()

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
		el := doc.CreateElement(tagName)
		return b.BindElement(el)
	})

	jsDoc.Set("createElementNS", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		namespaceURI := ""
		if !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		qualifiedName := call.Arguments[1].String()
		el := doc.CreateElementNS(namespaceURI, qualifiedName)
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
		doc.Append(nodes...)
		return goja.Undefined()
	})

	jsDoc.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		doc.Prepend(nodes...)
		return goja.Undefined()
	})

	jsDoc.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		doc.ReplaceChildren(nodes...)
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
		title := ""
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
			title = call.Arguments[0].String()
		}
		doc := impl.CreateHTMLDocument(title)
		return b.bindDocumentInternal(doc)
	})

	// createDocument(namespaceURI, qualifiedName, doctype)
	jsImpl.Set("createDocument", func(call goja.FunctionCall) goja.Value {
		namespaceURI := ""
		qualifiedName := ""
		var doctype *dom.Node

		if len(call.Arguments) > 0 && !goja.IsNull(call.Arguments[0]) {
			namespaceURI = call.Arguments[0].String()
		}
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) {
			qualifiedName = call.Arguments[1].String()
		}
		if len(call.Arguments) > 2 && !goja.IsNull(call.Arguments[2]) && !goja.IsUndefined(call.Arguments[2]) {
			obj := call.Arguments[2].ToObject(vm)
			doctype = b.getGoNode(obj)
		}

		doc := impl.CreateDocument(namespaceURI, qualifiedName, doctype)
		return b.bindDocumentInternal(doc)
	})

	// createDocumentType(qualifiedName, publicId, systemId)
	jsImpl.Set("createDocumentType", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			panic(vm.NewTypeError("Failed to execute 'createDocumentType': 3 arguments required"))
		}
		qualifiedName := call.Arguments[0].String()
		publicId := call.Arguments[1].String()
		systemId := call.Arguments[2].String()

		doctype := impl.CreateDocumentType(qualifiedName, publicId, systemId)
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
	if b.elementProto != nil {
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
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsEl.DefineAccessorProperty("style", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindCSSStyleDeclaration(el.Style())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

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
		if !goja.IsNull(call.Arguments[0]) {
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
		el.SetAttribute(name, value)
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
		el.SetAttributeNS(ns, qualifiedName, value)
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
		if !goja.IsNull(call.Arguments[0]) {
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
		if len(call.Arguments) > 1 {
			force := call.Arguments[1].ToBoolean()
			return vm.ToValue(el.ToggleAttribute(name, force))
		}
		return vm.ToValue(el.ToggleAttribute(name))
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
		return vm.ToValue(el.Matches(selector))
	})

	jsEl.Set("closest", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		selector := call.Arguments[0].String()
		found := el.Closest(selector)
		if found == nil {
			return goja.Null()
		}
		return b.BindElement(found)
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
		el.Append(nodes...)
		return goja.Undefined()
	})

	jsEl.Set("prepend", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		el.Prepend(nodes...)
		return goja.Undefined()
	})

	jsEl.Set("replaceChildren", func(call goja.FunctionCall) goja.Value {
		nodes := b.convertJSNodesToGo(call.Arguments)
		el.ReplaceChildren(nodes...)
		return goja.Undefined()
	})

	// Bind common node properties and methods
	b.bindNodeProperties(jsEl, node)

	// Cache the binding
	b.nodeMap[node] = jsEl

	return jsEl
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

	// Set prototype (use the generic node prototype)
	if b.nodeProto != nil {
		jsNode.SetPrototype(b.nodeProto)
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
		frag.ReplaceChildren(nodes...)
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
		delete(b.nodeMap, result)
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

// BindNodeList creates a JavaScript NodeList object.
func (b *DOMBinder) BindNodeList(nodeList *dom.NodeList) *goja.Object {
	vm := b.runtime.vm
	jsList := vm.NewObject()

	// Set prototype for instanceof to work
	if b.nodeListProto != nil {
		jsList.SetPrototype(b.nodeListProto)
	}

	jsList.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(nodeList.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsList.Set("item", func(call goja.FunctionCall) goja.Value {
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

	// Array-like indexing via proxy or direct property setting
	for i := 0; i < nodeList.Length(); i++ {
		idx := i
		jsList.DefineAccessorProperty(vm.ToValue(idx).String(), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			node := nodeList.Item(idx)
			if node == nil {
				return goja.Undefined()
			}
			return b.BindNode(node)
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
		for i := 0; i < nodeList.Length(); i++ {
			node := nodeList.Item(i)
			jsNode := b.BindNode(node)
			callback(thisArg, jsNode, vm.ToValue(i), jsList)
		}
		return goja.Undefined()
	})

	jsList.Set("entries", func(call goja.FunctionCall) goja.Value {
		// Return an iterator-like array of [index, value] pairs
		entries := make([]interface{}, nodeList.Length())
		for i := 0; i < nodeList.Length(); i++ {
			entries[i] = []interface{}{i, b.BindNode(nodeList.Item(i))}
		}
		return vm.ToValue(entries)
	})

	jsList.Set("keys", func(call goja.FunctionCall) goja.Value {
		keys := make([]int, nodeList.Length())
		for i := 0; i < nodeList.Length(); i++ {
			keys[i] = i
		}
		return vm.ToValue(keys)
	})

	jsList.Set("values", func(call goja.FunctionCall) goja.Value {
		values := make([]interface{}, nodeList.Length())
		for i := 0; i < nodeList.Length(); i++ {
			values[i] = b.BindNode(nodeList.Item(i))
		}
		return vm.ToValue(values)
	})

	return jsList
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

	jsCol.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(collection.Length())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	jsCol.Set("item", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		el := collection.Item(index)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	jsCol.Set("namedItem", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		name := call.Arguments[0].String()
		el := collection.NamedItem(name)
		if el == nil {
			return goja.Null()
		}
		return b.BindElement(el)
	})

	// Array-like indexing
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
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}

	return jsCol
}

// BindDOMTokenList creates a JavaScript DOMTokenList object with dynamic numeric indexing.
func (b *DOMBinder) BindDOMTokenList(tokenList *dom.DOMTokenList) *goja.Object {
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
		for _, arg := range call.Arguments {
			tokenList.Add(arg.String())
		}
		return goja.Undefined()
	})

	jsList.Set("remove", func(call goja.FunctionCall) goja.Value {
		for _, arg := range call.Arguments {
			tokenList.Remove(arg.String())
		}
		return goja.Undefined()
	})

	jsList.Set("toggle", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		token := call.Arguments[0].String()
		if len(call.Arguments) > 1 {
			force := call.Arguments[1].ToBoolean()
			return vm.ToValue(tokenList.Toggle(token, force))
		}
		return vm.ToValue(tokenList.Toggle(token))
	})

	jsList.Set("replace", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(false)
		}
		oldToken := call.Arguments[0].String()
		newToken := call.Arguments[1].String()
		return vm.ToValue(tokenList.Replace(oldToken, newToken))
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
	// Set prototype for instanceof to work
	if b.htmlCollectionProto != nil {
		jsCol.SetPrototype(b.htmlCollectionProto)
	}
	jsCol.Set("length", 0)
	jsCol.Set("item", func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	})
	jsCol.Set("namedItem", func(call goja.FunctionCall) goja.Value {
		return goja.Null()
	})
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

	vm := b.runtime.vm
	jsAttr := vm.NewObject()

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
