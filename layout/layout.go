// Package layout handles the CSS visual formatting model and box layout.
// Reference: https://www.w3.org/TR/CSS2/visuren.html and https://www.w3.org/TR/css-box-3/
package layout

import (
	"math"
	"strings"

	"github.com/chrisuehlinger/viberowser/css"
	"github.com/chrisuehlinger/viberowser/dom"
)

// Dimensions represents the dimensions of a layout box including content, padding, border, and margin.
type Dimensions struct {
	Content Rect
	Padding EdgeSizes
	Border  EdgeSizes
	Margin  EdgeSizes
}

// Rect represents a rectangular area.
type Rect struct {
	X, Y, Width, Height float64
}

// EdgeSizes represents the sizes of edges (top, right, bottom, left).
type EdgeSizes struct {
	Top, Right, Bottom, Left float64
}

// BoxType represents the type of layout box.
type BoxType int

const (
	BlockBox BoxType = iota
	InlineBox
	InlineBlockBox
	AnonymousBlockBox
	AnonymousInlineBox
	NoneBox // display: none
)

// PositionType represents the CSS position property values.
type PositionType int

const (
	PositionStatic PositionType = iota
	PositionRelative
	PositionAbsolute
	PositionFixed
	PositionSticky
)

// FloatType represents the CSS float property values.
type FloatType int

const (
	FloatNone FloatType = iota
	FloatLeft
	FloatRight
)

// OverflowType represents the CSS overflow property values.
type OverflowType int

const (
	OverflowVisible OverflowType = iota
	OverflowHidden
	OverflowScroll
	OverflowAuto
)

// BoxSizing represents the CSS box-sizing property values.
type BoxSizing int

const (
	BoxSizingContentBox BoxSizing = iota
	BoxSizingBorderBox
)

// LayoutBox represents a box in the layout tree.
type LayoutBox struct {
	Dimensions   Dimensions
	BoxType      BoxType
	Element      *dom.Element
	ComputedStyle *css.ComputedStyle
	Children     []*LayoutBox

	// Positioning
	Position     PositionType
	Float        FloatType
	ZIndex       int
	IsStackingContext bool

	// Offset for positioned elements (top, right, bottom, left)
	OffsetTop    float64
	OffsetRight  float64
	OffsetBottom float64
	OffsetLeft   float64
	HasOffsetTop    bool
	HasOffsetRight  bool
	HasOffsetBottom bool
	HasOffsetLeft   bool

	// For inline content
	LineBoxes    []*LineBox
	TextContent  string

	// Overflow handling
	Overflow     OverflowType
	OverflowX    OverflowType
	OverflowY    OverflowType

	// Box sizing
	BoxSizing    BoxSizing

	// For absolute positioning reference
	ContainingBlock *LayoutBox

	// Anonymous box parent reference
	Parent *LayoutBox
}

// LineBox represents a line of inline content.
type LineBox struct {
	Rect        Rect
	InlineItems []*InlineItem
	Baseline    float64
}

// InlineItem represents an inline-level item within a line.
type InlineItem struct {
	Rect       Rect
	LayoutBox  *LayoutBox
	Text       string
	Start      int // Character offset for text
	End        int
}

// Float represents a floated element.
type Float struct {
	Box       *LayoutBox
	Type      FloatType
	Clearance float64
}

// FormattingContext represents a block or inline formatting context.
type FormattingContext struct {
	Type   BoxType
	Floats []Float
	ClearLeft  float64
	ClearRight float64
	CurrentY   float64
}

// LayoutContext holds state during layout computation.
type LayoutContext struct {
	ViewportWidth  float64
	ViewportHeight float64

	// Containing block stack
	ContainingBlocks []*Dimensions

	// Current formatting context
	FormattingContext *FormattingContext

	// Floats in the current block formatting context
	LeftFloats  []*Float
	RightFloats []*Float
}

// NewLayoutContext creates a new layout context with the given viewport dimensions.
func NewLayoutContext(viewportWidth, viewportHeight float64) *LayoutContext {
	initial := &Dimensions{
		Content: Rect{
			X:      0,
			Y:      0,
			Width:  viewportWidth,
			Height: viewportHeight,
		},
	}
	return &LayoutContext{
		ViewportWidth:    viewportWidth,
		ViewportHeight:   viewportHeight,
		ContainingBlocks: []*Dimensions{initial},
	}
}

// CurrentContainingBlock returns the current containing block.
func (ctx *LayoutContext) CurrentContainingBlock() *Dimensions {
	if len(ctx.ContainingBlocks) == 0 {
		return nil
	}
	return ctx.ContainingBlocks[len(ctx.ContainingBlocks)-1]
}

// PushContainingBlock pushes a new containing block onto the stack.
func (ctx *LayoutContext) PushContainingBlock(dims *Dimensions) {
	ctx.ContainingBlocks = append(ctx.ContainingBlocks, dims)
}

// PopContainingBlock pops the current containing block from the stack.
func (ctx *LayoutContext) PopContainingBlock() {
	if len(ctx.ContainingBlocks) > 1 {
		ctx.ContainingBlocks = ctx.ContainingBlocks[:len(ctx.ContainingBlocks)-1]
	}
}

// BuildLayoutTree constructs a layout tree from a DOM element and computed styles.
func BuildLayoutTree(element *dom.Element, styleResolver *css.StyleResolver, ctx *LayoutContext) *LayoutBox {
	if element == nil {
		return nil
	}

	computedStyle := styleResolver.ResolveStyles(element, nil)
	return buildLayoutBoxRecursive(element, computedStyle, styleResolver, nil, ctx)
}

func buildLayoutBoxRecursive(element *dom.Element, computedStyle *css.ComputedStyle, styleResolver *css.StyleResolver, parentStyle *css.ComputedStyle, ctx *LayoutContext) *LayoutBox {
	if computedStyle == nil {
		return nil
	}

	// Check display value
	displayVal := computedStyle.GetComputedStyleProperty("display")
	if displayVal == "none" {
		return nil
	}

	box := &LayoutBox{
		Element:       element,
		ComputedStyle: computedStyle,
	}

	// Determine box type
	box.BoxType = determineBoxType(displayVal)

	// Determine position type
	box.Position = determinePositionType(computedStyle.GetComputedStyleProperty("position"))

	// Determine float type
	box.Float = determineFloatType(computedStyle.GetComputedStyleProperty("float"))

	// Determine overflow
	box.Overflow = determineOverflowType(computedStyle.GetComputedStyleProperty("overflow"))
	box.OverflowX = determineOverflowType(computedStyle.GetComputedStyleProperty("overflow-x"))
	box.OverflowY = determineOverflowType(computedStyle.GetComputedStyleProperty("overflow-y"))

	// Determine box-sizing
	box.BoxSizing = determineBoxSizing(computedStyle.GetComputedStyleProperty("box-sizing"))

	// Parse z-index
	zIndexVal := computedStyle.GetComputedStyleProperty("z-index")
	if zIndexVal != "auto" && zIndexVal != "" {
		box.ZIndex = parseInt(zIndexVal)
	}

	// Check if this creates a stacking context
	box.IsStackingContext = isStackingContext(box)

	// Parse position offsets
	parsePositionOffsets(box, computedStyle)

	// Build children recursively
	node := element.AsNode()
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.NodeType() == dom.ElementNode {
			childElement := (*dom.Element)(child)
			childStyle := styleResolver.ResolveStyles(childElement, computedStyle)
			childBox := buildLayoutBoxRecursive(childElement, childStyle, styleResolver, computedStyle, ctx)
			if childBox != nil {
				childBox.Parent = box
				box.Children = append(box.Children, childBox)
			}
		} else if child.NodeType() == dom.TextNode {
			// Create inline box for text
			textContent := strings.TrimSpace(child.TextContent())
			if textContent != "" {
				textBox := &LayoutBox{
					BoxType:     InlineBox,
					TextContent: textContent,
					ComputedStyle: computedStyle,
					Parent:      box,
				}
				box.Children = append(box.Children, textBox)
			}
		}
	}

	// Handle anonymous boxes if needed
	normalizeBoxTree(box)

	return box
}

// determineBoxType determines the box type from the display value.
func determineBoxType(display string) BoxType {
	switch strings.ToLower(display) {
	case "block":
		return BlockBox
	case "inline":
		return InlineBox
	case "inline-block":
		return InlineBlockBox
	case "none":
		return NoneBox
	default:
		return InlineBox
	}
}

// determinePositionType determines the position type from the position value.
func determinePositionType(position string) PositionType {
	switch strings.ToLower(position) {
	case "relative":
		return PositionRelative
	case "absolute":
		return PositionAbsolute
	case "fixed":
		return PositionFixed
	case "sticky":
		return PositionSticky
	default:
		return PositionStatic
	}
}

// determineFloatType determines the float type from the float value.
func determineFloatType(float string) FloatType {
	switch strings.ToLower(float) {
	case "left":
		return FloatLeft
	case "right":
		return FloatRight
	default:
		return FloatNone
	}
}

// determineOverflowType determines the overflow type from the overflow value.
func determineOverflowType(overflow string) OverflowType {
	switch strings.ToLower(overflow) {
	case "hidden":
		return OverflowHidden
	case "scroll":
		return OverflowScroll
	case "auto":
		return OverflowAuto
	default:
		return OverflowVisible
	}
}

// determineBoxSizing determines the box-sizing from the value.
func determineBoxSizing(boxSizing string) BoxSizing {
	if strings.ToLower(boxSizing) == "border-box" {
		return BoxSizingBorderBox
	}
	return BoxSizingContentBox
}

// isStackingContext checks if a box creates a stacking context.
func isStackingContext(box *LayoutBox) bool {
	if box.Position == PositionAbsolute || box.Position == PositionRelative || box.Position == PositionFixed {
		if box.ZIndex != 0 {
			return true
		}
	}
	if box.ComputedStyle != nil {
		opacity := box.ComputedStyle.GetComputedStyleProperty("opacity")
		if opacity != "" && opacity != "1" {
			return true
		}
	}
	return false
}

// parsePositionOffsets parses top, right, bottom, left values.
func parsePositionOffsets(box *LayoutBox, style *css.ComputedStyle) {
	if style == nil {
		return
	}

	topVal := style.GetPropertyValue("top")
	if topVal != nil && topVal.Keyword != "auto" {
		box.OffsetTop = topVal.Length
		box.HasOffsetTop = true
	}

	rightVal := style.GetPropertyValue("right")
	if rightVal != nil && rightVal.Keyword != "auto" {
		box.OffsetRight = rightVal.Length
		box.HasOffsetRight = true
	}

	bottomVal := style.GetPropertyValue("bottom")
	if bottomVal != nil && bottomVal.Keyword != "auto" {
		box.OffsetBottom = bottomVal.Length
		box.HasOffsetBottom = true
	}

	leftVal := style.GetPropertyValue("left")
	if leftVal != nil && leftVal.Keyword != "auto" {
		box.OffsetLeft = leftVal.Length
		box.HasOffsetLeft = true
	}
}

// normalizeBoxTree creates anonymous boxes as needed per CSS spec.
// Block boxes cannot have inline children mixed with block children.
func normalizeBoxTree(box *LayoutBox) {
	if box.BoxType != BlockBox || len(box.Children) == 0 {
		return
	}

	hasBlockChildren := false
	hasInlineChildren := false

	for _, child := range box.Children {
		if child.BoxType == BlockBox {
			hasBlockChildren = true
		} else if child.BoxType == InlineBox {
			hasInlineChildren = true
		}
	}

	// If we have both block and inline children, wrap inline runs in anonymous blocks
	if hasBlockChildren && hasInlineChildren {
		var newChildren []*LayoutBox
		var currentInlineRun []*LayoutBox

		for _, child := range box.Children {
			if child.BoxType == BlockBox {
				// Flush any inline run
				if len(currentInlineRun) > 0 {
					anonBox := &LayoutBox{
						BoxType:  AnonymousBlockBox,
						Children: currentInlineRun,
						Parent:   box,
					}
					for _, c := range currentInlineRun {
						c.Parent = anonBox
					}
					newChildren = append(newChildren, anonBox)
					currentInlineRun = nil
				}
				newChildren = append(newChildren, child)
			} else {
				currentInlineRun = append(currentInlineRun, child)
			}
		}

		// Flush remaining inline run
		if len(currentInlineRun) > 0 {
			anonBox := &LayoutBox{
				BoxType:  AnonymousBlockBox,
				Children: currentInlineRun,
				Parent:   box,
			}
			for _, c := range currentInlineRun {
				c.Parent = anonBox
			}
			newChildren = append(newChildren, anonBox)
		}

		box.Children = newChildren
	}
}

// parseInt parses an integer from a string, returning 0 on error.
func parseInt(s string) int {
	var result int
	_, _ = stringToInt(s, &result)
	return result
}

func stringToInt(s string, result *int) (bool, error) {
	n := 0
	negative := false
	i := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		i = 1
	}

	for ; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}

	if negative {
		n = -n
	}
	*result = n
	return true, nil
}

// PaddingBox returns the area covered by content and padding.
func (d *Dimensions) PaddingBox() Rect {
	return d.Content.ExpandedBy(d.Padding)
}

// BorderBox returns the area covered by content, padding, and border.
func (d *Dimensions) BorderBox() Rect {
	return d.PaddingBox().ExpandedBy(d.Border)
}

// MarginBox returns the area covered by content, padding, border, and margin.
func (d *Dimensions) MarginBox() Rect {
	return d.BorderBox().ExpandedBy(d.Margin)
}

// ExpandedBy returns a rectangle expanded by the given edge sizes.
func (r Rect) ExpandedBy(edge EdgeSizes) Rect {
	return Rect{
		X:      r.X - edge.Left,
		Y:      r.Y - edge.Top,
		Width:  r.Width + edge.Left + edge.Right,
		Height: r.Height + edge.Top + edge.Bottom,
	}
}

// Layout computes the layout for a box and its children.
func (box *LayoutBox) Layout(ctx *LayoutContext) {
	containingBlock := ctx.CurrentContainingBlock()
	if containingBlock == nil {
		return
	}

	switch box.BoxType {
	case BlockBox, AnonymousBlockBox:
		box.layoutBlock(ctx, containingBlock)
	case InlineBox, InlineBlockBox:
		box.layoutInline(ctx, containingBlock)
	}
}

// layoutBlock performs block layout algorithm.
func (box *LayoutBox) layoutBlock(ctx *LayoutContext, containingBlock *Dimensions) {
	// Calculate width first (depends on containing block)
	box.calculateBlockWidth(containingBlock)

	// Position the box
	box.calculateBlockPosition(containingBlock, ctx)

	// Layout children
	box.layoutBlockChildren(ctx)

	// Calculate height after children are laid out
	box.calculateBlockHeight(containingBlock)

	// Handle relative positioning
	if box.Position == PositionRelative {
		box.applyRelativePosition()
	}
}

// calculateBlockWidth calculates the width of a block-level box.
func (box *LayoutBox) calculateBlockWidth(containingBlock *Dimensions) {
	style := box.ComputedStyle
	if style == nil {
		box.Dimensions.Content.Width = containingBlock.Content.Width
		return
	}

	// Get width value
	width := getLength(style, "width")
	widthKeyword := getKeyword(style, "width")

	// Get margin values
	marginLeft := getLength(style, "margin-left")
	marginRight := getLength(style, "margin-right")
	marginLeftKeyword := getKeyword(style, "margin-left")
	marginRightKeyword := getKeyword(style, "margin-right")

	// Get padding values
	paddingLeft := getLength(style, "padding-left")
	paddingRight := getLength(style, "padding-right")

	// Get border values
	borderLeft := getBorderWidth(style, "border-left-width")
	borderRight := getBorderWidth(style, "border-right-width")

	// Determine which values are auto (empty string means default/auto for these properties)
	widthAuto := widthKeyword == "auto" || (widthKeyword == "" && width == 0)
	marginLeftAuto := marginLeftKeyword == "auto"
	marginRightAuto := marginRightKeyword == "auto"

	// Calculate total horizontal space (use 0 for auto width in initial calculation)
	widthForCalc := width
	if widthAuto {
		widthForCalc = 0
	}
	total := marginLeft + marginRight + paddingLeft + paddingRight + borderLeft + borderRight + widthForCalc

	// Handle auto values
	if !widthAuto && total > containingBlock.Content.Width {
		// Over-constrained: treat auto margins as zero
		if marginLeftAuto {
			marginLeft = 0
		}
		if marginRightAuto {
			marginRight = 0
		}
	}

	underflow := containingBlock.Content.Width - total

	if !widthAuto && !marginLeftAuto && !marginRightAuto {
		// Over-constrained: add underflow to margin-right
		marginRight += underflow
	} else if !widthAuto && !marginLeftAuto && marginRightAuto {
		marginRight = underflow
	} else if !widthAuto && marginLeftAuto && !marginRightAuto {
		marginLeft = underflow
	} else if widthAuto {
		if marginLeftAuto {
			marginLeft = 0
		}
		if marginRightAuto {
			marginRight = 0
		}
		if underflow >= 0 {
			width = underflow
		} else {
			width = 0
			marginRight += underflow
		}
	} else if !widthAuto && marginLeftAuto && marginRightAuto {
		// Center the element
		marginLeft = underflow / 2
		marginRight = underflow / 2
	}

	// Apply box-sizing adjustment
	if box.BoxSizing == BoxSizingBorderBox {
		// Width includes padding and border
		contentWidth := width - paddingLeft - paddingRight - borderLeft - borderRight
		if contentWidth < 0 {
			contentWidth = 0
		}
		box.Dimensions.Content.Width = contentWidth
	} else {
		box.Dimensions.Content.Width = width
	}

	box.Dimensions.Margin.Left = marginLeft
	box.Dimensions.Margin.Right = marginRight
	box.Dimensions.Padding.Left = paddingLeft
	box.Dimensions.Padding.Right = paddingRight
	box.Dimensions.Border.Left = borderLeft
	box.Dimensions.Border.Right = borderRight
}

// calculateBlockPosition calculates the position of a block-level box.
func (box *LayoutBox) calculateBlockPosition(containingBlock *Dimensions, ctx *LayoutContext) {
	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Get margin, padding, border for top and bottom
	marginTop := getLength(style, "margin-top")
	marginBottom := getLength(style, "margin-bottom")
	paddingTop := getLength(style, "padding-top")
	paddingBottom := getLength(style, "padding-bottom")
	borderTop := getBorderWidth(style, "border-top-width")
	borderBottom := getBorderWidth(style, "border-bottom-width")

	box.Dimensions.Margin.Top = marginTop
	box.Dimensions.Margin.Bottom = marginBottom
	box.Dimensions.Padding.Top = paddingTop
	box.Dimensions.Padding.Bottom = paddingBottom
	box.Dimensions.Border.Top = borderTop
	box.Dimensions.Border.Bottom = borderBottom

	// Calculate X position
	box.Dimensions.Content.X = containingBlock.Content.X +
		box.Dimensions.Margin.Left +
		box.Dimensions.Border.Left +
		box.Dimensions.Padding.Left

	// Calculate Y position (will be updated based on siblings in parent layout)
	box.Dimensions.Content.Y = containingBlock.Content.Y + containingBlock.Content.Height +
		box.Dimensions.Margin.Top +
		box.Dimensions.Border.Top +
		box.Dimensions.Padding.Top
}

// layoutBlockChildren lays out children of a block box.
func (box *LayoutBox) layoutBlockChildren(ctx *LayoutContext) {
	// Track the current Y position for placing children
	ctx.PushContainingBlock(&box.Dimensions)
	defer ctx.PopContainingBlock()

	for _, child := range box.Children {
		child.Layout(ctx)
		// Accumulate child's margin box height
		box.Dimensions.Content.Height += child.Dimensions.MarginBox().Height
	}
}

// calculateBlockHeight calculates the height of a block-level box.
func (box *LayoutBox) calculateBlockHeight(containingBlock *Dimensions) {
	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Check for explicit height
	height := getLength(style, "height")
	heightKeyword := getKeyword(style, "height")
	// Height is explicit if it's not "auto" and not empty with zero length
	heightExplicit := heightKeyword != "auto" && (heightKeyword != "" || height > 0)
	if heightExplicit {

		// Apply box-sizing adjustment
		if box.BoxSizing == BoxSizingBorderBox {
			contentHeight := height -
				box.Dimensions.Padding.Top - box.Dimensions.Padding.Bottom -
				box.Dimensions.Border.Top - box.Dimensions.Border.Bottom
			if contentHeight < 0 {
				contentHeight = 0
			}
			box.Dimensions.Content.Height = contentHeight
		} else {
			box.Dimensions.Content.Height = height
		}
	}

	// Apply min-height
	minHeight := getLength(style, "min-height")
	if box.Dimensions.Content.Height < minHeight {
		box.Dimensions.Content.Height = minHeight
	}

	// Apply max-height
	maxHeightKeyword := getKeyword(style, "max-height")
	if maxHeightKeyword != "none" && maxHeightKeyword != "" {
		maxHeight := getLength(style, "max-height")
		if box.Dimensions.Content.Height > maxHeight {
			box.Dimensions.Content.Height = maxHeight
		}
	}
}

// layoutInline performs inline layout algorithm.
func (box *LayoutBox) layoutInline(ctx *LayoutContext, containingBlock *Dimensions) {
	// For inline boxes, dimensions are based on content
	style := box.ComputedStyle

	if style != nil {
		box.Dimensions.Padding.Left = getLength(style, "padding-left")
		box.Dimensions.Padding.Right = getLength(style, "padding-right")
		box.Dimensions.Padding.Top = getLength(style, "padding-top")
		box.Dimensions.Padding.Bottom = getLength(style, "padding-bottom")
		box.Dimensions.Border.Left = getBorderWidth(style, "border-left-width")
		box.Dimensions.Border.Right = getBorderWidth(style, "border-right-width")
		box.Dimensions.Border.Top = getBorderWidth(style, "border-top-width")
		box.Dimensions.Border.Bottom = getBorderWidth(style, "border-bottom-width")
		box.Dimensions.Margin.Left = getLength(style, "margin-left")
		box.Dimensions.Margin.Right = getLength(style, "margin-right")
	}

	// Inline boxes participate in inline formatting context
	// For now, estimate content dimensions based on text
	if box.TextContent != "" {
		fontSize := 16.0
		if style != nil {
			fontSize = getLength(style, "font-size")
			if fontSize == 0 {
				fontSize = 16.0
			}
		}
		// Approximate: 0.6 average character width, line height = 1.2 * font-size
		box.Dimensions.Content.Width = float64(len(box.TextContent)) * fontSize * 0.6
		box.Dimensions.Content.Height = fontSize * 1.2
	}

	// Layout children
	for _, child := range box.Children {
		child.Layout(ctx)
	}
}

// applyRelativePosition applies relative positioning offsets.
func (box *LayoutBox) applyRelativePosition() {
	if box.HasOffsetLeft {
		box.Dimensions.Content.X += box.OffsetLeft
	} else if box.HasOffsetRight {
		box.Dimensions.Content.X -= box.OffsetRight
	}

	if box.HasOffsetTop {
		box.Dimensions.Content.Y += box.OffsetTop
	} else if box.HasOffsetBottom {
		box.Dimensions.Content.Y -= box.OffsetBottom
	}
}

// getLength retrieves a length value from computed style.
func getLength(style *css.ComputedStyle, property string) float64 {
	if style == nil {
		return 0
	}
	val := style.GetPropertyValue(property)
	if val == nil {
		return 0
	}
	return val.Length
}

// getKeyword retrieves a keyword value from computed style.
func getKeyword(style *css.ComputedStyle, property string) string {
	if style == nil {
		return ""
	}
	val := style.GetPropertyValue(property)
	if val == nil {
		return ""
	}
	return val.Keyword
}

// getBorderWidth retrieves a border width value, handling keywords.
func getBorderWidth(style *css.ComputedStyle, property string) float64 {
	if style == nil {
		return 0
	}
	val := style.GetPropertyValue(property)
	if val == nil {
		return 0
	}

	// Handle keyword values
	switch val.Keyword {
	case "thin":
		return 1
	case "medium":
		return 3
	case "thick":
		return 5
	default:
		return val.Length
	}
}

// CollapseMarginsBlock implements vertical margin collapsing for block boxes.
// This should be called during layout to handle adjacent margins.
func CollapseMarginsBlock(boxes []*LayoutBox) {
	for i := 1; i < len(boxes); i++ {
		prevBox := boxes[i-1]
		currBox := boxes[i]

		// Get the margins
		prevMarginBottom := prevBox.Dimensions.Margin.Bottom
		currMarginTop := currBox.Dimensions.Margin.Top

		// Collapse: use the larger margin
		collapsedMargin := math.Max(prevMarginBottom, currMarginTop)

		// Adjust the current box's position
		adjustment := (prevMarginBottom + currMarginTop) - collapsedMargin
		currBox.Dimensions.Content.Y -= adjustment

		// Zero out the collapsed margins (or keep track for rendering)
		// For simplicity, we adjust position and leave margins as-is
	}
}

// LayoutAbsolutePositioned lays out absolutely positioned boxes.
func LayoutAbsolutePositioned(box *LayoutBox, ctx *LayoutContext) {
	containingBlock := ctx.CurrentContainingBlock()
	if containingBlock == nil {
		return
	}

	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Calculate width
	widthKeyword := getKeyword(style, "width")
	if widthKeyword == "auto" {
		// Shrink-to-fit width or based on left/right constraints
		if box.HasOffsetLeft && box.HasOffsetRight {
			box.Dimensions.Content.Width = containingBlock.Content.Width -
				box.OffsetLeft - box.OffsetRight -
				box.Dimensions.Margin.Left - box.Dimensions.Margin.Right -
				box.Dimensions.Padding.Left - box.Dimensions.Padding.Right -
				box.Dimensions.Border.Left - box.Dimensions.Border.Right
		} else {
			// Shrink to fit
			box.Dimensions.Content.Width = 0 // Will be determined by content
		}
	} else {
		box.Dimensions.Content.Width = getLength(style, "width")
	}

	// Calculate height
	heightKeyword := getKeyword(style, "height")
	if heightKeyword == "auto" {
		if box.HasOffsetTop && box.HasOffsetBottom {
			box.Dimensions.Content.Height = containingBlock.Content.Height -
				box.OffsetTop - box.OffsetBottom -
				box.Dimensions.Margin.Top - box.Dimensions.Margin.Bottom -
				box.Dimensions.Padding.Top - box.Dimensions.Padding.Bottom -
				box.Dimensions.Border.Top - box.Dimensions.Border.Bottom
		} else {
			box.Dimensions.Content.Height = 0 // Will be determined by content
		}
	} else {
		box.Dimensions.Content.Height = getLength(style, "height")
	}

	// Calculate position
	if box.HasOffsetLeft {
		box.Dimensions.Content.X = containingBlock.Content.X + box.OffsetLeft +
			box.Dimensions.Margin.Left + box.Dimensions.Border.Left + box.Dimensions.Padding.Left
	} else if box.HasOffsetRight {
		box.Dimensions.Content.X = containingBlock.Content.X + containingBlock.Content.Width -
			box.OffsetRight - box.Dimensions.Margin.Right - box.Dimensions.Border.Right -
			box.Dimensions.Padding.Right - box.Dimensions.Content.Width
	} else {
		// Use static position
		box.Dimensions.Content.X = containingBlock.Content.X +
			box.Dimensions.Margin.Left + box.Dimensions.Border.Left + box.Dimensions.Padding.Left
	}

	if box.HasOffsetTop {
		box.Dimensions.Content.Y = containingBlock.Content.Y + box.OffsetTop +
			box.Dimensions.Margin.Top + box.Dimensions.Border.Top + box.Dimensions.Padding.Top
	} else if box.HasOffsetBottom {
		box.Dimensions.Content.Y = containingBlock.Content.Y + containingBlock.Content.Height -
			box.OffsetBottom - box.Dimensions.Margin.Bottom - box.Dimensions.Border.Bottom -
			box.Dimensions.Padding.Bottom - box.Dimensions.Content.Height
	} else {
		// Use static position
		box.Dimensions.Content.Y = containingBlock.Content.Y +
			box.Dimensions.Margin.Top + box.Dimensions.Border.Top + box.Dimensions.Padding.Top
	}
}

// ClearType represents the CSS clear property values.
type ClearType int

const (
	ClearNone ClearType = iota
	ClearLeft
	ClearRight
	ClearBoth
)

// determineClearType determines the clear type from the clear value.
func determineClearType(clear string) ClearType {
	switch strings.ToLower(clear) {
	case "left":
		return ClearLeft
	case "right":
		return ClearRight
	case "both":
		return ClearBoth
	default:
		return ClearNone
	}
}

// LayoutFloat lays out a floated box.
func LayoutFloat(box *LayoutBox, ctx *LayoutContext) {
	containingBlock := ctx.CurrentContainingBlock()
	if containingBlock == nil {
		return
	}

	// Calculate width (floats must have explicit width or shrink-to-fit)
	style := box.ComputedStyle
	widthKeyword := getKeyword(style, "width")
	if widthKeyword == "auto" {
		// Shrink-to-fit
		box.Dimensions.Content.Width = 0 // Will be determined by content
	} else {
		box.Dimensions.Content.Width = getLength(style, "width")
	}

	// Get padding and borders
	box.Dimensions.Padding.Left = getLength(style, "padding-left")
	box.Dimensions.Padding.Right = getLength(style, "padding-right")
	box.Dimensions.Padding.Top = getLength(style, "padding-top")
	box.Dimensions.Padding.Bottom = getLength(style, "padding-bottom")
	box.Dimensions.Border.Left = getBorderWidth(style, "border-left-width")
	box.Dimensions.Border.Right = getBorderWidth(style, "border-right-width")
	box.Dimensions.Border.Top = getBorderWidth(style, "border-top-width")
	box.Dimensions.Border.Bottom = getBorderWidth(style, "border-bottom-width")
	box.Dimensions.Margin.Left = getLength(style, "margin-left")
	box.Dimensions.Margin.Right = getLength(style, "margin-right")
	box.Dimensions.Margin.Top = getLength(style, "margin-top")
	box.Dimensions.Margin.Bottom = getLength(style, "margin-bottom")

	// Position based on float direction
	if box.Float == FloatLeft {
		box.Dimensions.Content.X = containingBlock.Content.X +
			box.Dimensions.Margin.Left + box.Dimensions.Border.Left + box.Dimensions.Padding.Left
	} else if box.Float == FloatRight {
		box.Dimensions.Content.X = containingBlock.Content.X + containingBlock.Content.Width -
			box.Dimensions.Margin.Right - box.Dimensions.Border.Right -
			box.Dimensions.Padding.Right - box.Dimensions.Content.Width
	}

	box.Dimensions.Content.Y = containingBlock.Content.Y + containingBlock.Content.Height +
		box.Dimensions.Margin.Top + box.Dimensions.Border.Top + box.Dimensions.Padding.Top

	// Layout children to determine height
	ctx.PushContainingBlock(&box.Dimensions)
	for _, child := range box.Children {
		child.Layout(ctx)
		box.Dimensions.Content.Height += child.Dimensions.MarginBox().Height
	}
	ctx.PopContainingBlock()

	// Apply explicit height if set
	heightKeyword := getKeyword(style, "height")
	if heightKeyword != "auto" && heightKeyword != "" {
		box.Dimensions.Content.Height = getLength(style, "height")
	}
}

// UpdateElementGeometries walks the layout tree and updates each element's geometry
// for use by getBoundingClientRect and related APIs.
func UpdateElementGeometries(box *LayoutBox, parentOffsetParent *dom.Element, parentX, parentY float64) {
	if box == nil {
		return
	}

	// Track the offset parent for children
	nextOffsetParent := parentOffsetParent
	nextParentX := parentX
	nextParentY := parentY

	// Update this element's geometry if it has a DOM element
	if box.Element != nil {
		borderBox := box.Dimensions.BorderBox()

		// Calculate scroll dimensions (for now, same as client dimensions)
		// In a real implementation, this would account for overflow content
		clientWidth := box.Dimensions.Content.Width + box.Dimensions.Padding.Left + box.Dimensions.Padding.Right
		clientHeight := box.Dimensions.Content.Height + box.Dimensions.Padding.Top + box.Dimensions.Padding.Bottom

		geom := &dom.ElementGeometry{
			// Border box coordinates relative to viewport
			X:      borderBox.X,
			Y:      borderBox.Y,
			Width:  borderBox.Width,
			Height: borderBox.Height,

			// Box model dimensions
			ContentWidth:  box.Dimensions.Content.Width,
			ContentHeight: box.Dimensions.Content.Height,
			PaddingTop:    box.Dimensions.Padding.Top,
			PaddingRight:  box.Dimensions.Padding.Right,
			PaddingBottom: box.Dimensions.Padding.Bottom,
			PaddingLeft:   box.Dimensions.Padding.Left,
			BorderTop:     box.Dimensions.Border.Top,
			BorderRight:   box.Dimensions.Border.Right,
			BorderBottom:  box.Dimensions.Border.Bottom,
			BorderLeft:    box.Dimensions.Border.Left,
			MarginTop:     box.Dimensions.Margin.Top,
			MarginRight:   box.Dimensions.Margin.Right,
			MarginBottom:  box.Dimensions.Margin.Bottom,
			MarginLeft:    box.Dimensions.Margin.Left,

			// Offset properties
			OffsetTop:    borderBox.Y - parentY,
			OffsetLeft:   borderBox.X - parentX,
			OffsetWidth:  borderBox.Width,
			OffsetHeight: borderBox.Height,
			OffsetParent: parentOffsetParent,

			// Client properties (content + padding, no border/scrollbar)
			ClientTop:    box.Dimensions.Border.Top,
			ClientLeft:   box.Dimensions.Border.Left,
			ClientWidth:  clientWidth,
			ClientHeight: clientHeight,

			// Scroll properties (for now, same as client for non-scrolling elements)
			ScrollWidth:  clientWidth,
			ScrollHeight: clientHeight,
			ScrollTop:    0,
			ScrollLeft:   0,
		}

		box.Element.SetGeometry(geom)

		// Positioned elements become offset parents for their descendants
		if box.Position != PositionStatic {
			nextOffsetParent = box.Element
			nextParentX = borderBox.X
			nextParentY = borderBox.Y
		}
	}

	// Recursively update children
	for _, child := range box.Children {
		UpdateElementGeometries(child, nextOffsetParent, nextParentX, nextParentY)
	}
}
