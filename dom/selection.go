package dom

// Selection represents a user's selection of text in the document.
// Per the Selection API specification, each Document has an associated Selection object.
type Selection struct {
	// The document this selection belongs to
	document *Document

	// The ranges that make up this selection.
	// Per spec, most browsers only support a single range.
	ranges []*Range
}

// NewSelection creates a new Selection for the given document.
func NewSelection(doc *Document) *Selection {
	return &Selection{
		document: doc,
		ranges:   make([]*Range, 0),
	}
}

// AnchorNode returns the node in which the selection begins.
// Returns nil if the selection is empty.
func (s *Selection) AnchorNode() *Node {
	if len(s.ranges) == 0 {
		return nil
	}
	return s.ranges[0].StartContainer()
}

// AnchorOffset returns the offset within the anchor node where the selection starts.
func (s *Selection) AnchorOffset() int {
	if len(s.ranges) == 0 {
		return 0
	}
	return s.ranges[0].StartOffset()
}

// FocusNode returns the node in which the selection ends.
// Returns nil if the selection is empty.
func (s *Selection) FocusNode() *Node {
	if len(s.ranges) == 0 {
		return nil
	}
	return s.ranges[0].EndContainer()
}

// FocusOffset returns the offset within the focus node where the selection ends.
func (s *Selection) FocusOffset() int {
	if len(s.ranges) == 0 {
		return 0
	}
	return s.ranges[0].EndOffset()
}

// IsCollapsed returns true if the selection's start and end points are at the same position.
func (s *Selection) IsCollapsed() bool {
	if len(s.ranges) == 0 {
		return true
	}
	return s.ranges[0].Collapsed()
}

// RangeCount returns the number of ranges in the selection.
func (s *Selection) RangeCount() int {
	return len(s.ranges)
}

// Type returns the type of the current selection.
// Returns "None", "Caret", or "Range".
func (s *Selection) Type() string {
	if len(s.ranges) == 0 {
		return "None"
	}
	if s.ranges[0].Collapsed() {
		return "Caret"
	}
	return "Range"
}

// Direction returns the direction of the current selection.
// Returns "none", "forward", or "backward".
// For now, we always return "forward" since we don't track selection direction.
func (s *Selection) Direction() string {
	if len(s.ranges) == 0 {
		return "none"
	}
	return "forward"
}

// GetRangeAt returns the range at the given index.
// Returns nil and an error if the index is out of bounds.
func (s *Selection) GetRangeAt(index int) (*Range, error) {
	if index < 0 || index >= len(s.ranges) {
		return nil, ErrIndexSize("Index out of range")
	}
	return s.ranges[index], nil
}

// AddRange adds a Range to the selection.
// Per spec, most browsers only support a single range.
func (s *Selection) AddRange(r *Range) {
	if r == nil {
		return
	}
	// Most browsers only support one range - we follow that behavior
	if len(s.ranges) == 0 {
		s.ranges = append(s.ranges, r)
	}
	// If a range already exists, some browsers ignore the addition
}

// RemoveRange removes a Range from the selection.
func (s *Selection) RemoveRange(r *Range) error {
	if r == nil {
		return nil
	}
	for i, existing := range s.ranges {
		if existing == r {
			s.ranges = append(s.ranges[:i], s.ranges[i+1:]...)
			return nil
		}
	}
	return ErrNotFound("The given range is not in the selection")
}

// RemoveAllRanges removes all ranges from the selection.
func (s *Selection) RemoveAllRanges() {
	s.ranges = s.ranges[:0]
}

// Empty is an alias for RemoveAllRanges.
func (s *Selection) Empty() {
	s.RemoveAllRanges()
}

// Collapse collapses the selection to a single point.
func (s *Selection) Collapse(node *Node, offset int) error {
	if node == nil {
		s.RemoveAllRanges()
		return nil
	}

	// Create a new range collapsed at the specified point
	r := NewRange(s.document)
	if err := r.SetStart(node, offset); err != nil {
		return err
	}
	r.Collapse(true)

	s.ranges = []*Range{r}
	return nil
}

// SetPosition is an alias for Collapse.
func (s *Selection) SetPosition(node *Node, offset int) error {
	return s.Collapse(node, offset)
}

// CollapseToStart collapses the selection to the start of the first range.
func (s *Selection) CollapseToStart() error {
	if len(s.ranges) == 0 {
		return ErrInvalidState("No ranges in selection")
	}
	return s.Collapse(s.ranges[0].StartContainer(), s.ranges[0].StartOffset())
}

// CollapseToEnd collapses the selection to the end of the last range.
func (s *Selection) CollapseToEnd() error {
	if len(s.ranges) == 0 {
		return ErrInvalidState("No ranges in selection")
	}
	lastRange := s.ranges[len(s.ranges)-1]
	return s.Collapse(lastRange.EndContainer(), lastRange.EndOffset())
}

// Extend moves the focus of the selection to a specified point.
func (s *Selection) Extend(node *Node, offset int) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	if len(s.ranges) == 0 {
		return ErrInvalidState("No ranges in selection")
	}

	r := s.ranges[0]
	// Keep the anchor, move the focus (end)
	if err := r.SetEnd(node, offset); err != nil {
		return err
	}

	return nil
}

// SelectAllChildren adds all the children of the specified node to the selection.
func (s *Selection) SelectAllChildren(node *Node) error {
	if node == nil {
		return ErrNotFound("Node is null")
	}

	// Create a new range that spans all children
	r := NewRange(s.document)
	if err := r.SelectNodeContents(node); err != nil {
		return err
	}

	s.ranges = []*Range{r}
	return nil
}

// SetBaseAndExtent sets the selection to be a range including parts of two DOM nodes.
func (s *Selection) SetBaseAndExtent(anchorNode *Node, anchorOffset int, focusNode *Node, focusOffset int) error {
	if anchorNode == nil || focusNode == nil {
		return ErrNotFound("Node is null")
	}

	r := NewRange(s.document)
	if err := r.SetStart(anchorNode, anchorOffset); err != nil {
		return err
	}
	if err := r.SetEnd(focusNode, focusOffset); err != nil {
		return err
	}

	s.ranges = []*Range{r}
	return nil
}

// ContainsNode indicates if a certain node is part of the selection.
// If partialContainment is true, returns true if any part of the node is in the selection.
// If partialContainment is false, returns true only if the entire node is in the selection.
func (s *Selection) ContainsNode(node *Node, partialContainment bool) bool {
	if node == nil || len(s.ranges) == 0 {
		return false
	}

	for _, r := range s.ranges {
		if partialContainment {
			if r.IntersectsNode(node) {
				return true
			}
		} else {
			// Check if the entire node is contained
			if r.containsNode(node) {
				return true
			}
		}
	}

	return false
}

// DeleteFromDocument removes the content of the selection from the document.
func (s *Selection) DeleteFromDocument() error {
	for _, r := range s.ranges {
		if err := r.DeleteContents(); err != nil {
			return err
		}
	}
	return nil
}

// ToString returns a string representing the text content of the selection.
func (s *Selection) ToString() string {
	if len(s.ranges) == 0 {
		return ""
	}

	var result string
	for _, r := range s.ranges {
		result += r.ToString()
	}
	return result
}

// Modify changes the current selection.
// alter is one of "move" or "extend"
// direction is one of "forward", "backward", "left", or "right"
// granularity is one of "character", "word", "sentence", "line", "paragraph", "lineboundary", "sentenceboundary", "paragraphboundary", or "documentboundary"
// This is a non-standard but widely supported method.
func (s *Selection) Modify(alter, direction, granularity string) {
	// This is a complex method that depends on text layout.
	// For now, we provide a stub implementation.
	// A full implementation would require integration with the layout engine.
}
