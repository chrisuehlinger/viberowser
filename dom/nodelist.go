package dom

// NodeList represents a collection of nodes. It can be either live (automatically
// updated when the DOM changes) or static (a snapshot at a point in time).
type NodeList struct {
	// For live NodeLists, this is the parent node
	parent *Node

	// For static NodeLists, this holds the nodes
	staticNodes []*Node

	// Whether this is a live or static NodeList
	isLive bool
}

// newNodeList creates a new live NodeList for the given parent node.
func newNodeList(parent *Node) *NodeList {
	return &NodeList{
		parent: parent,
		isLive: true,
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
	if nl.isLive {
		count := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			count++
		}
		return count
	}
	return len(nl.staticNodes)
}

// Item returns the node at the given index, or nil if the index is out of bounds.
func (nl *NodeList) Item(index int) *Node {
	if index < 0 {
		return nil
	}

	if nl.isLive {
		i := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			if i == index {
				return child
			}
			i++
		}
		return nil
	}

	if index >= len(nl.staticNodes) {
		return nil
	}
	return nl.staticNodes[index]
}

// ForEach calls the given function for each node in the collection.
func (nl *NodeList) ForEach(fn func(node *Node, index int)) {
	if nl.isLive {
		i := 0
		for child := nl.parent.firstChild; child != nil; child = child.nextSibling {
			fn(child, i)
			i++
		}
	} else {
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
