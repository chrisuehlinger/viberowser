package dom

// StaticRange represents a simple range that doesn't update when the DOM changes.
// Unlike Range, StaticRange is not live and doesn't track mutations.
// Per the DOM specification, StaticRange is a simpler alternative to Range
// when live updates are not needed.
type StaticRange struct {
	startContainer *Node
	startOffset    int
	endContainer   *Node
	endOffset      int
}

// StaticRangeInit contains the initialization parameters for creating a StaticRange.
type StaticRangeInit struct {
	StartContainer *Node
	StartOffset    int
	EndContainer   *Node
	EndOffset      int
}

// NewStaticRange creates a new StaticRange from the given initialization parameters.
// Returns an error if any of the containers are DocumentType or Attr nodes.
func NewStaticRange(init StaticRangeInit) (*StaticRange, error) {
	// Validate start container
	if init.StartContainer == nil {
		return nil, &DOMError{Name: "TypeError", Message: "startContainer is required"}
	}
	if init.StartContainer.nodeType == DocumentTypeNode || init.StartContainer.nodeType == AttributeNode {
		return nil, &DOMError{
			Name:    "InvalidNodeTypeError",
			Message: "startContainer cannot be a DocumentType or Attr node",
		}
	}

	// Validate end container
	if init.EndContainer == nil {
		return nil, &DOMError{Name: "TypeError", Message: "endContainer is required"}
	}
	if init.EndContainer.nodeType == DocumentTypeNode || init.EndContainer.nodeType == AttributeNode {
		return nil, &DOMError{
			Name:    "InvalidNodeTypeError",
			Message: "endContainer cannot be a DocumentType or Attr node",
		}
	}

	// Note: Per the spec, StaticRange does NOT validate that offsets are within bounds.
	// This is intentional - the offsets can be greater than the node length.

	return &StaticRange{
		startContainer: init.StartContainer,
		startOffset:    init.StartOffset,
		endContainer:   init.EndContainer,
		endOffset:      init.EndOffset,
	}, nil
}

// StartContainer returns the node where the range starts.
func (r *StaticRange) StartContainer() *Node {
	return r.startContainer
}

// StartOffset returns the offset within the start container.
func (r *StaticRange) StartOffset() int {
	return r.startOffset
}

// EndContainer returns the node where the range ends.
func (r *StaticRange) EndContainer() *Node {
	return r.endContainer
}

// EndOffset returns the offset within the end container.
func (r *StaticRange) EndOffset() int {
	return r.endOffset
}

// Collapsed returns true if start and end are the same point.
func (r *StaticRange) Collapsed() bool {
	return r.startContainer == r.endContainer && r.startOffset == r.endOffset
}
