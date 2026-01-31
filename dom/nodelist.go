package dom

// NodeListType indicates the type of NodeList
type NodeListType int

const (
	// NodeListTypeStatic is a snapshot NodeList (e.g., from querySelectorAll)
	NodeListTypeStatic NodeListType = iota
	// NodeListTypeChildNodes is a live list of child nodes
	NodeListTypeChildNodes
	// NodeListTypeFiltered is a live list filtered by a predicate (e.g., getElementsByName)
	NodeListTypeFiltered
)

// NodeList represents a collection of nodes. It can be either live (automatically
// updated when the DOM changes) or static (a snapshot at a point in time).
type NodeList struct {
	// The type of NodeList
	listType NodeListType

	// For live child node lists, this is the parent node
	parent *Node

	// For static NodeLists, this holds the nodes
	staticNodes []*Node

	// For filtered live lists, this is the root and filter function
	root   *Node
	filter func(*Node) bool

	// Whether this is a live or static NodeList (kept for compatibility)
	isLive bool
}

// newNodeList creates a new live NodeList for the given parent node.
func newNodeList(parent *Node) *NodeList {
	return &NodeList{
		listType: NodeListTypeChildNodes,
		parent:   parent,
		isLive:   true,
	}
}

// NewLiveNodeList creates a new live NodeList that filters nodes by a predicate.
// The filter is called for each element in tree order under the root.
func NewLiveNodeList(root *Node, filter func(*Node) bool) *NodeList {
	return &NodeList{
		listType: NodeListTypeFiltered,
		root:     root,
		filter:   filter,
		isLive:   true,
	}
}

// collectFilteredNodes traverses the tree and collects matching nodes.
func (nl *NodeList) collectFilteredNodes() []*Node {
	if nl.listType != NodeListTypeFiltered {
		return nil
	}
	var nodes []*Node
	nl.traverseFiltered(nl.root, &nodes)
	return nodes
}

func (nl *NodeList) traverseFiltered(node *Node, nodes *[]*Node) {
	for child := node.firstChild; child != nil; child = child.nextSibling {
		if nl.filter(child) {
			*nodes = append(*nodes, child)
		}
		// Recursively traverse children
		nl.traverseFiltered(child, nodes)
	}
}

// NewStaticNodeList creates a new static NodeList from a slice of nodes.
func NewStaticNodeList(nodes []*Node) *NodeList {
	staticCopy := make([]*Node, len(nodes))
	copy(staticCopy, nodes)
	return &NodeList{
		staticNodes: staticCopy,
		isLive:      false,
	}
}

// Length returns the number of nodes in the collection.
func (nl *NodeList) Length() int {
	switch nl.listType {
	case NodeListTypeChildNodes:
		count := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			count++
		}
		return count
	case NodeListTypeFiltered:
		return len(nl.collectFilteredNodes())
	default: // NodeListTypeStatic
		return len(nl.staticNodes)
	}
}

// Item returns the node at the given index, or nil if the index is out of bounds.
func (nl *NodeList) Item(index int) *Node {
	if index < 0 {
		return nil
	}

	switch nl.listType {
	case NodeListTypeChildNodes:
		i := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			if i == index {
				return child
			}
			i++
		}
		return nil
	case NodeListTypeFiltered:
		nodes := nl.collectFilteredNodes()
		if index >= len(nodes) {
			return nil
		}
		return nodes[index]
	default: // NodeListTypeStatic
		if index >= len(nl.staticNodes) {
			return nil
		}
		return nl.staticNodes[index]
	}
}

// ForEach calls the given function for each node in the collection.
func (nl *NodeList) ForEach(fn func(node *Node, index int)) {
	switch nl.listType {
	case NodeListTypeChildNodes:
		i := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			fn(child, i)
			i++
		}
	case NodeListTypeFiltered:
		for i, node := range nl.collectFilteredNodes() {
			fn(node, i)
		}
	default: // NodeListTypeStatic
		for i, node := range nl.staticNodes {
			fn(node, i)
		}
	}
}

// Entries returns an iterator that yields [index, node] pairs.
func (nl *NodeList) Entries() [][2]interface{} {
	var entries [][2]interface{}
	nl.ForEach(func(node *Node, index int) {
		entries = append(entries, [2]interface{}{index, node})
	})
	return entries
}

// Keys returns an iterator that yields indices.
func (nl *NodeList) Keys() []int {
	var keys []int
	nl.ForEach(func(node *Node, index int) {
		keys = append(keys, index)
	})
	return keys
}

// Values returns an iterator that yields nodes.
func (nl *NodeList) Values() []*Node {
	var values []*Node
	nl.ForEach(func(node *Node, index int) {
		values = append(values, node)
	})
	return values
}

// ToSlice returns all nodes as a slice.
func (nl *NodeList) ToSlice() []*Node {
	return nl.Values()
}
