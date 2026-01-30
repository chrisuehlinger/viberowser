package js

import (
	"github.com/AYColumbia/viberowser/dom"
	"github.com/dop251/goja"
)

// DOMBinder provides methods to bind DOM objects to JavaScript.
type DOMBinder struct {
	runtime *Runtime
	nodeMap map[*dom.Node]*goja.Object // Cache to return same JS object for same DOM node
}

// NewDOMBinder creates a new DOM binder for the given runtime.
func NewDOMBinder(runtime *Runtime) *DOMBinder {
	return &DOMBinder{
		runtime: runtime,
		nodeMap: make(map[*dom.Node]*goja.Object),
	}
}

// BindDocument creates a JavaScript document object from a DOM document.
func (b *DOMBinder) BindDocument(doc *dom.Document) *goja.Object {
	vm := b.runtime.vm
	jsDoc := vm.NewObject()

	// Store reference to the Go document
	jsDoc.Set("_goDoc", doc)

	// Document properties
	jsDoc.Set("nodeType", int(dom.DocumentNode))
	jsDoc.Set("nodeName", "#document")

	// Document accessors
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

	jsDoc.Set("createDocumentFragment", func(call goja.FunctionCall) goja.Value {
		frag := doc.CreateDocumentFragment()
		return b.BindDocumentFragment(frag)
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

	// Child node properties (document can have children)
	b.bindNodeProperties(jsDoc, doc.AsNode())

	b.runtime.SetDocument(jsDoc)
	return jsDoc
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
			el.SetTextContent(call.Arguments[0].String())
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

	// DOM manipulation methods
	jsEl.Set("remove", func(call goja.FunctionCall) goja.Value {
		el.Remove()
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
	}

	// For text, comment, and other nodes
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

	// For Text nodes, add data property
	if node.NodeType() == dom.TextNode || node.NodeType() == dom.CommentNode {
		jsNode.DefineAccessorProperty("data", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(node.NodeValue())
		}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 {
				node.SetNodeValue(call.Arguments[0].String())
			}
			return goja.Undefined()
		}), goja.FLAG_FALSE, goja.FLAG_TRUE)

		jsNode.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(len(node.NodeValue()))
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}

	b.bindNodeProperties(jsNode, node)

	// Cache the binding
	b.nodeMap[node] = jsNode

	return jsNode
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
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		childObj := call.Arguments[0].ToObject(vm)
		goChild := b.getGoNode(childObj)
		if goChild == nil {
			return goja.Null()
		}
		result := node.AppendChild(goChild)
		if result == nil {
			return goja.Null()
		}
		return b.BindNode(result)
	})

	jsObj.Set("insertBefore", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		newChildObj := call.Arguments[0].ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		if goNewChild == nil {
			return goja.Null()
		}

		var goRefChild *dom.Node
		if len(call.Arguments) > 1 && !goja.IsNull(call.Arguments[1]) {
			refChildObj := call.Arguments[1].ToObject(vm)
			goRefChild = b.getGoNode(refChildObj)
		}

		result := node.InsertBefore(goNewChild, goRefChild)
		if result == nil {
			return goja.Null()
		}
		return b.BindNode(result)
	})

	jsObj.Set("removeChild", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		childObj := call.Arguments[0].ToObject(vm)
		goChild := b.getGoNode(childObj)
		if goChild == nil {
			return goja.Null()
		}
		result := node.RemoveChild(goChild)
		if result == nil {
			return goja.Null()
		}
		// Remove from cache since it's been detached
		delete(b.nodeMap, result)
		return b.BindNode(result)
	})

	jsObj.Set("replaceChild", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Null()
		}
		newChildObj := call.Arguments[0].ToObject(vm)
		oldChildObj := call.Arguments[1].ToObject(vm)
		goNewChild := b.getGoNode(newChildObj)
		goOldChild := b.getGoNode(oldChildObj)
		if goNewChild == nil || goOldChild == nil {
			return goja.Null()
		}
		result := node.ReplaceChild(goNewChild, goOldChild)
		if result == nil {
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

// BindDOMTokenList creates a JavaScript DOMTokenList object.
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
		// Most token lists don't have a defined set of supported tokens
		return vm.ToValue(true)
	})

	jsList.Set("toString", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(tokenList.Value())
	})

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
		for i := 0; i < tokenList.Length(); i++ {
			token := tokenList.Item(i)
			callback(thisArg, vm.ToValue(token), vm.ToValue(i), jsList)
		}
		return goja.Undefined()
	})

	return jsList
}

// createEmptyNodeList returns an empty NodeList-like object.
func (b *DOMBinder) createEmptyNodeList() *goja.Object {
	return b.bindStaticNodeList(nil)
}

// createEmptyHTMLCollection returns an empty HTMLCollection-like object.
func (b *DOMBinder) createEmptyHTMLCollection() *goja.Object {
	vm := b.runtime.vm
	jsCol := vm.NewObject()
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
