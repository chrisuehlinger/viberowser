// Package layout handles CSS Flexbox layout algorithm.
// Reference: https://drafts.csswg.org/css-flexbox-1/
package layout

import (
	"math"
	"sort"
)

// FlexDirection represents the flex-direction property values.
type FlexDirection int

const (
	FlexDirectionRow FlexDirection = iota
	FlexDirectionRowReverse
	FlexDirectionColumn
	FlexDirectionColumnReverse
)

// FlexWrap represents the flex-wrap property values.
type FlexWrap int

const (
	FlexWrapNowrap FlexWrap = iota
	FlexWrapWrap
	FlexWrapWrapReverse
)

// JustifyContent represents the justify-content property values.
type JustifyContent int

const (
	JustifyFlexStart JustifyContent = iota
	JustifyFlexEnd
	JustifyCenter
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// AlignItems represents the align-items property values.
type AlignItems int

const (
	AlignItemsStretch AlignItems = iota
	AlignItemsFlexStart
	AlignItemsFlexEnd
	AlignItemsCenter
	AlignItemsBaseline
)

// AlignContent represents the align-content property values.
type AlignContent int

const (
	AlignContentStretch AlignContent = iota
	AlignContentFlexStart
	AlignContentFlexEnd
	AlignContentCenter
	AlignContentSpaceBetween
	AlignContentSpaceAround
)

// AlignSelf represents the align-self property values.
type AlignSelf int

const (
	AlignSelfAuto AlignSelf = iota
	AlignSelfFlexStart
	AlignSelfFlexEnd
	AlignSelfCenter
	AlignSelfBaseline
	AlignSelfStretch
)

// FlexItem holds flex item-specific layout data during flex calculations.
type FlexItem struct {
	Box          *LayoutBox
	Order        int
	FlexGrow     float64
	FlexShrink   float64
	FlexBasis    float64
	FlexBasisAuto bool
	AlignSelf    AlignSelf

	// Computed values during layout
	MainSize       float64
	CrossSize      float64
	HypotheticalMainSize float64
	TargetMainSize float64
	Frozen         bool
	Violation      float64
}

// FlexLine represents a line in flex layout (for wrap).
type FlexLine struct {
	Items     []*FlexItem
	MainSize  float64
	CrossSize float64
}

// FlexContainer holds flex container layout state.
type FlexContainer struct {
	Direction      FlexDirection
	Wrap           FlexWrap
	JustifyContent JustifyContent
	AlignItems     AlignItems
	AlignContent   AlignContent

	// Calculated axes
	IsRowDirection bool
	MainSize       float64
	CrossSize      float64
}

// layoutFlex performs the flexbox layout algorithm.
func (box *LayoutBox) layoutFlex(ctx *LayoutContext, containingBlock *Dimensions) {
	// Step 1: Initialize flex container
	container := initFlexContainer(box)

	// Calculate container's main and cross sizes from containing block
	box.calculateFlexContainerWidth(containingBlock)
	box.calculateFlexContainerPosition(containingBlock, ctx)

	// Handle explicit height if set (important for column flex containers)
	if box.ComputedStyle != nil {
		heightVal := box.ComputedStyle.GetPropertyValue("height")
		if heightVal != nil && (heightVal.Length > 0 || (heightVal.Keyword != "auto" && heightVal.Keyword != "")) {
			box.Dimensions.Content.Height = heightVal.Length
		}
	}

	// Get the available main/cross size
	availableMain := box.Dimensions.Content.Width
	availableCross := box.Dimensions.Content.Height
	if !container.IsRowDirection {
		availableMain, availableCross = availableCross, availableMain
	}

	// Step 2: Collect flex items and calculate their properties
	items := collectFlexItems(box, container, ctx)
	if len(items) == 0 {
		return
	}

	// Step 3: Determine the main size of flex items
	for _, item := range items {
		calculateHypotheticalMainSize(item, container, availableMain, ctx)
	}

	// Step 4: Collect flex items into flex lines
	lines := collectFlexLines(items, container, availableMain)

	// Step 5: Resolve flexible lengths (grow/shrink)
	for _, line := range lines {
		resolveFlexibleLengths(line, availableMain)
	}

	// Step 6: Determine cross size of each flex line
	for _, line := range lines {
		determineLineCrossSize(line, container, ctx, availableCross)
	}

	// Step 7: Align flex items on cross axis within each line
	for _, line := range lines {
		alignItemsCrossAxis(line, container)
	}

	// Step 8: Align flex lines (if wrapping)
	totalCrossSize := alignFlexLines(lines, container, availableCross)

	// Step 9: Main axis alignment (justify-content)
	justifyInfos := make([]*JustifyInfo, len(lines))
	for i, line := range lines {
		justifyInfos[i] = justifyMainAxis(line, container, availableMain)
	}

	// Step 10: Position flex items
	positionFlexItems(box, lines, container, justifyInfos)

	// Set the container's height based on content (if auto)
	heightKeyword := getKeyword(box.ComputedStyle, "height")
	if heightKeyword == "auto" || heightKeyword == "" {
		if container.IsRowDirection {
			box.Dimensions.Content.Height = totalCrossSize
		} else {
			box.Dimensions.Content.Height = calculateColumnFlexHeight(lines)
		}
	}

	// Handle relative positioning
	if box.Position == PositionRelative {
		box.applyRelativePosition()
	}
}

// initFlexContainer initializes flex container properties from computed style.
func initFlexContainer(box *LayoutBox) *FlexContainer {
	style := box.ComputedStyle
	container := &FlexContainer{
		Direction:      FlexDirectionRow,
		Wrap:           FlexWrapNowrap,
		JustifyContent: JustifyFlexStart,
		AlignItems:     AlignItemsStretch,
		AlignContent:   AlignContentStretch,
		IsRowDirection: true,
	}

	if style == nil {
		return container
	}

	// Parse flex-direction
	switch getKeyword(style, "flex-direction") {
	case "row":
		container.Direction = FlexDirectionRow
		container.IsRowDirection = true
	case "row-reverse":
		container.Direction = FlexDirectionRowReverse
		container.IsRowDirection = true
	case "column":
		container.Direction = FlexDirectionColumn
		container.IsRowDirection = false
	case "column-reverse":
		container.Direction = FlexDirectionColumnReverse
		container.IsRowDirection = false
	}

	// Parse flex-wrap
	switch getKeyword(style, "flex-wrap") {
	case "nowrap":
		container.Wrap = FlexWrapNowrap
	case "wrap":
		container.Wrap = FlexWrapWrap
	case "wrap-reverse":
		container.Wrap = FlexWrapWrapReverse
	}

	// Parse justify-content
	switch getKeyword(style, "justify-content") {
	case "flex-start":
		container.JustifyContent = JustifyFlexStart
	case "flex-end":
		container.JustifyContent = JustifyFlexEnd
	case "center":
		container.JustifyContent = JustifyCenter
	case "space-between":
		container.JustifyContent = JustifySpaceBetween
	case "space-around":
		container.JustifyContent = JustifySpaceAround
	case "space-evenly":
		container.JustifyContent = JustifySpaceEvenly
	}

	// Parse align-items
	switch getKeyword(style, "align-items") {
	case "stretch":
		container.AlignItems = AlignItemsStretch
	case "flex-start":
		container.AlignItems = AlignItemsFlexStart
	case "flex-end":
		container.AlignItems = AlignItemsFlexEnd
	case "center":
		container.AlignItems = AlignItemsCenter
	case "baseline":
		container.AlignItems = AlignItemsBaseline
	}

	// Parse align-content
	switch getKeyword(style, "align-content") {
	case "stretch":
		container.AlignContent = AlignContentStretch
	case "flex-start":
		container.AlignContent = AlignContentFlexStart
	case "flex-end":
		container.AlignContent = AlignContentFlexEnd
	case "center":
		container.AlignContent = AlignContentCenter
	case "space-between":
		container.AlignContent = AlignContentSpaceBetween
	case "space-around":
		container.AlignContent = AlignContentSpaceAround
	}

	return container
}

// calculateFlexContainerWidth calculates the width of a flex container.
func (box *LayoutBox) calculateFlexContainerWidth(containingBlock *Dimensions) {
	// Flex containers use block-level width calculation
	box.calculateBlockWidth(containingBlock)
}

// calculateFlexContainerPosition calculates the position of a flex container.
func (box *LayoutBox) calculateFlexContainerPosition(containingBlock *Dimensions, ctx *LayoutContext) {
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

	// Calculate Y position - for flex containers not stacked, use containingBlock.Content.Y
	// The containingBlock.Content.Height is used for stacking in block flow context
	box.Dimensions.Content.Y = containingBlock.Content.Y + containingBlock.Content.Height +
		box.Dimensions.Margin.Top +
		box.Dimensions.Border.Top +
		box.Dimensions.Padding.Top
}

// collectFlexItems gathers all flex items and their properties.
func collectFlexItems(box *LayoutBox, container *FlexContainer, ctx *LayoutContext) []*FlexItem {
	items := make([]*FlexItem, 0, len(box.Children))

	for _, child := range box.Children {
		// Absolutely positioned children are not flex items
		if child.Position == PositionAbsolute || child.Position == PositionFixed {
			continue
		}

		item := &FlexItem{
			Box:       child,
			FlexGrow:  0,
			FlexShrink: 1,
			AlignSelf: AlignSelfAuto,
		}

		style := child.ComputedStyle
		if style != nil {
			// Parse order
			orderVal := style.GetPropertyValue("order")
			if orderVal != nil && orderVal.Length != 0 {
				item.Order = int(orderVal.Length)
			}

			// Parse flex-grow
			flexGrowVal := style.GetPropertyValue("flex-grow")
			if flexGrowVal != nil {
				item.FlexGrow = flexGrowVal.Length
			}

			// Parse flex-shrink
			flexShrinkVal := style.GetPropertyValue("flex-shrink")
			if flexShrinkVal != nil {
				item.FlexShrink = flexShrinkVal.Length
			} else {
				item.FlexShrink = 1 // Default
			}

			// Parse flex-basis
			flexBasisVal := style.GetPropertyValue("flex-basis")
			if flexBasisVal != nil {
				if flexBasisVal.Keyword == "auto" {
					item.FlexBasisAuto = true
				} else if flexBasisVal.Length > 0 {
					item.FlexBasis = flexBasisVal.Length
					item.FlexBasisAuto = false
				} else {
					item.FlexBasisAuto = true
				}
			} else {
				item.FlexBasisAuto = true
			}

			// Parse align-self
			switch getKeyword(style, "align-self") {
			case "auto":
				item.AlignSelf = AlignSelfAuto
			case "flex-start":
				item.AlignSelf = AlignSelfFlexStart
			case "flex-end":
				item.AlignSelf = AlignSelfFlexEnd
			case "center":
				item.AlignSelf = AlignSelfCenter
			case "baseline":
				item.AlignSelf = AlignSelfBaseline
			case "stretch":
				item.AlignSelf = AlignSelfStretch
			}
		}

		// Calculate item's margins, padding, borders
		calculateFlexItemBoxModel(item, ctx)

		items = append(items, item)
	}

	// Sort by order property
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})

	return items
}

// calculateFlexItemBoxModel calculates padding, border, margin for a flex item.
func calculateFlexItemBoxModel(item *FlexItem, ctx *LayoutContext) {
	box := item.Box
	style := box.ComputedStyle
	if style == nil {
		return
	}

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
}

// calculateHypotheticalMainSize calculates the hypothetical main size of a flex item.
func calculateHypotheticalMainSize(item *FlexItem, container *FlexContainer, availableMain float64, ctx *LayoutContext) {
	box := item.Box
	style := box.ComputedStyle

	var baseSize float64

	// Determine base size from flex-basis or content
	if item.FlexBasisAuto {
		// Use the main size property if set, otherwise content size
		if container.IsRowDirection {
			widthVal := style.GetPropertyValue("width")
			// Width is explicitly set if it has a length > 0 or is not "auto"
			if widthVal != nil && (widthVal.Length > 0 || (widthVal.Keyword != "auto" && widthVal.Keyword != "")) {
				baseSize = widthVal.Length
			} else {
				baseSize = estimateContentMainSize(item, container, ctx)
			}
		} else {
			heightVal := style.GetPropertyValue("height")
			// Height is explicitly set if it has a length > 0 or is not "auto"
			if heightVal != nil && (heightVal.Length > 0 || (heightVal.Keyword != "auto" && heightVal.Keyword != "")) {
				baseSize = heightVal.Length
			} else {
				baseSize = estimateContentMainSize(item, container, ctx)
			}
		}
	} else {
		baseSize = item.FlexBasis
	}

	// Add padding and border in the main axis
	if container.IsRowDirection {
		baseSize += box.Dimensions.Padding.Left + box.Dimensions.Padding.Right
		baseSize += box.Dimensions.Border.Left + box.Dimensions.Border.Right
	} else {
		baseSize += box.Dimensions.Padding.Top + box.Dimensions.Padding.Bottom
		baseSize += box.Dimensions.Border.Top + box.Dimensions.Border.Bottom
	}

	// Apply min/max constraints
	baseSize = applyMainSizeConstraints(item, container, baseSize)

	item.HypotheticalMainSize = baseSize
	item.MainSize = baseSize
}

// estimateContentMainSize estimates the content-based main size of a flex item.
func estimateContentMainSize(item *FlexItem, container *FlexContainer, ctx *LayoutContext) float64 {
	box := item.Box

	// For text content, estimate based on text length
	if box.TextContent != "" {
		fontSize := 16.0
		if box.ComputedStyle != nil {
			fs := getLength(box.ComputedStyle, "font-size")
			if fs > 0 {
				fontSize = fs
			}
		}
		if container.IsRowDirection {
			return float64(len(box.TextContent)) * fontSize * 0.6
		}
		return fontSize * 1.2
	}

	// For boxes with children, sum children's main sizes
	var totalSize float64
	for _, child := range box.Children {
		childItem := &FlexItem{Box: child}
		calculateFlexItemBoxModel(childItem, ctx)
		childSize := estimateContentMainSize(childItem, container, ctx)
		if container.IsRowDirection {
			childSize += child.Dimensions.Margin.Left + child.Dimensions.Margin.Right
		} else {
			childSize += child.Dimensions.Margin.Top + child.Dimensions.Margin.Bottom
		}
		totalSize += childSize
	}

	return totalSize
}

// applyMainSizeConstraints applies min/max constraints to the main size.
func applyMainSizeConstraints(item *FlexItem, container *FlexContainer, size float64) float64 {
	style := item.Box.ComputedStyle
	if style == nil {
		return size
	}

	var minProp, maxProp string
	if container.IsRowDirection {
		minProp, maxProp = "min-width", "max-width"
	} else {
		minProp, maxProp = "min-height", "max-height"
	}

	minSize := getLength(style, minProp)
	if size < minSize {
		size = minSize
	}

	maxKeyword := getKeyword(style, maxProp)
	if maxKeyword != "none" && maxKeyword != "" {
		maxSize := getLength(style, maxProp)
		if maxSize > 0 && size > maxSize {
			size = maxSize
		}
	}

	return size
}

// collectFlexLines groups flex items into flex lines.
func collectFlexLines(items []*FlexItem, container *FlexContainer, availableMain float64) []*FlexLine {
	if len(items) == 0 {
		return nil
	}

	// If no wrapping, all items go in one line
	if container.Wrap == FlexWrapNowrap {
		return []*FlexLine{{Items: items}}
	}

	// Multi-line flex container
	var lines []*FlexLine
	var currentLine *FlexLine
	var currentMainSize float64

	for _, item := range items {
		itemMainSize := item.HypotheticalMainSize
		if container.IsRowDirection {
			itemMainSize += item.Box.Dimensions.Margin.Left + item.Box.Dimensions.Margin.Right
		} else {
			itemMainSize += item.Box.Dimensions.Margin.Top + item.Box.Dimensions.Margin.Bottom
		}

		if currentLine == nil {
			currentLine = &FlexLine{Items: []*FlexItem{item}}
			currentMainSize = itemMainSize
		} else if currentMainSize+itemMainSize > availableMain && len(currentLine.Items) > 0 {
			// Start new line
			lines = append(lines, currentLine)
			currentLine = &FlexLine{Items: []*FlexItem{item}}
			currentMainSize = itemMainSize
		} else {
			currentLine.Items = append(currentLine.Items, item)
			currentMainSize += itemMainSize
		}
	}

	if currentLine != nil {
		lines = append(lines, currentLine)
	}

	// Handle wrap-reverse
	if container.Wrap == FlexWrapWrapReverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	return lines
}

// resolveFlexibleLengths implements the flexible length resolution algorithm.
func resolveFlexibleLengths(line *FlexLine, availableMain float64) {
	items := line.Items
	if len(items) == 0 {
		return
	}

	// Calculate initial used space (hypothetical sizes + margins)
	usedMain := 0.0
	for _, item := range items {
		usedMain += item.HypotheticalMainSize
		if item.Box.ComputedStyle != nil {
			style := item.Box.ComputedStyle
			usedMain += getLength(style, "margin-left") + getLength(style, "margin-right")
		}
	}

	freeSpace := availableMain - usedMain

	// Determine if we're growing or shrinking
	growing := freeSpace > 0

	// Initialize target sizes to hypothetical sizes
	for _, item := range items {
		item.TargetMainSize = item.HypotheticalMainSize
	}

	// Calculate total flex factor
	totalFlex := 0.0
	for _, item := range items {
		if growing {
			totalFlex += item.FlexGrow
		} else {
			totalFlex += item.FlexShrink * item.HypotheticalMainSize
		}
	}

	// If no flex, nothing to distribute
	if totalFlex == 0 {
		for _, item := range items {
			item.MainSize = item.HypotheticalMainSize
		}
		return
	}

	// Distribute free space according to flex factors
	for _, item := range items {
		var ratio float64
		if growing {
			if item.FlexGrow > 0 {
				ratio = item.FlexGrow / totalFlex
				item.TargetMainSize = item.HypotheticalMainSize + freeSpace*ratio
			}
		} else {
			if item.FlexShrink > 0 && totalFlex > 0 {
				ratio = (item.FlexShrink * item.HypotheticalMainSize) / totalFlex
				// For shrinking, freeSpace is negative
				item.TargetMainSize = item.HypotheticalMainSize + freeSpace*ratio
			}
		}

		// Apply min/max constraints
		if item.Box.ComputedStyle != nil {
			minSize := getLength(item.Box.ComputedStyle, "min-width")
			if item.TargetMainSize < minSize {
				item.TargetMainSize = minSize
			}

			maxKeyword := getKeyword(item.Box.ComputedStyle, "max-width")
			if maxKeyword != "none" && maxKeyword != "" {
				maxSize := getLength(item.Box.ComputedStyle, "max-width")
				if maxSize > 0 && item.TargetMainSize > maxSize {
					item.TargetMainSize = maxSize
				}
			}
		}

		// Ensure non-negative
		if item.TargetMainSize < 0 {
			item.TargetMainSize = 0
		}

		item.MainSize = item.TargetMainSize
	}
}

// determineLineCrossSize determines the cross size of a flex line.
func determineLineCrossSize(line *FlexLine, container *FlexContainer, ctx *LayoutContext, availableCross float64) {
	maxCross := 0.0

	for _, item := range line.Items {
		// Calculate cross size based on content or explicit size
		var crossSize float64

		style := item.Box.ComputedStyle
		if container.IsRowDirection {
			heightVal := style.GetPropertyValue("height")
			if heightVal != nil && (heightVal.Length > 0 || (heightVal.Keyword != "auto" && heightVal.Keyword != "")) {
				crossSize = heightVal.Length
			} else {
				crossSize = estimateContentCrossSize(item, container, ctx)
			}
			crossSize += item.Box.Dimensions.Padding.Top + item.Box.Dimensions.Padding.Bottom
			crossSize += item.Box.Dimensions.Border.Top + item.Box.Dimensions.Border.Bottom
		} else {
			widthVal := style.GetPropertyValue("width")
			if widthVal != nil && (widthVal.Length > 0 || (widthVal.Keyword != "auto" && widthVal.Keyword != "")) {
				crossSize = widthVal.Length
			} else {
				crossSize = estimateContentCrossSize(item, container, ctx)
			}
			crossSize += item.Box.Dimensions.Padding.Left + item.Box.Dimensions.Padding.Right
			crossSize += item.Box.Dimensions.Border.Left + item.Box.Dimensions.Border.Right
		}

		item.CrossSize = crossSize
		if crossSize > maxCross {
			maxCross = crossSize
		}
	}

	line.CrossSize = maxCross

	// Apply stretch for items with align-self: stretch
	for _, item := range line.Items {
		alignSelf := item.AlignSelf
		if alignSelf == AlignSelfAuto {
			// Use container's align-items
			switch container.AlignItems {
			case AlignItemsStretch:
				alignSelf = AlignSelfStretch
			case AlignItemsFlexStart:
				alignSelf = AlignSelfFlexStart
			case AlignItemsFlexEnd:
				alignSelf = AlignSelfFlexEnd
			case AlignItemsCenter:
				alignSelf = AlignSelfCenter
			case AlignItemsBaseline:
				alignSelf = AlignSelfBaseline
			}
		}

		if alignSelf == AlignSelfStretch {
			item.CrossSize = line.CrossSize
		}
	}
}

// estimateContentCrossSize estimates the cross-axis content size.
func estimateContentCrossSize(item *FlexItem, container *FlexContainer, ctx *LayoutContext) float64 {
	box := item.Box

	if box.TextContent != "" {
		fontSize := 16.0
		if box.ComputedStyle != nil {
			fs := getLength(box.ComputedStyle, "font-size")
			if fs > 0 {
				fontSize = fs
			}
		}
		if container.IsRowDirection {
			return fontSize * 1.2
		}
		return float64(len(box.TextContent)) * fontSize * 0.6
	}

	// For boxes with children, estimate based on content
	var totalSize float64
	for _, child := range box.Children {
		childItem := &FlexItem{Box: child}
		childSize := estimateContentCrossSize(childItem, container, ctx)
		if container.IsRowDirection {
			childSize += child.Dimensions.Margin.Top + child.Dimensions.Margin.Bottom
		} else {
			childSize += child.Dimensions.Margin.Left + child.Dimensions.Margin.Right
		}
		if childSize > totalSize {
			totalSize = childSize
		}
	}

	return totalSize
}

// alignItemsCrossAxis aligns items on the cross axis within a line.
func alignItemsCrossAxis(line *FlexLine, container *FlexContainer) {
	for _, item := range line.Items {
		alignSelf := item.AlignSelf
		if alignSelf == AlignSelfAuto {
			switch container.AlignItems {
			case AlignItemsFlexStart:
				alignSelf = AlignSelfFlexStart
			case AlignItemsFlexEnd:
				alignSelf = AlignSelfFlexEnd
			case AlignItemsCenter:
				alignSelf = AlignSelfCenter
			case AlignItemsBaseline:
				alignSelf = AlignSelfBaseline
			default:
				alignSelf = AlignSelfStretch
			}
		}

		// The actual offset is calculated during positioning
		// Store the alignment for use later
	}
}

// alignFlexLines aligns flex lines within the container.
func alignFlexLines(lines []*FlexLine, container *FlexContainer, availableCross float64) float64 {
	if len(lines) == 0 {
		return 0
	}

	// Calculate total cross size of all lines
	totalCross := 0.0
	for _, line := range lines {
		totalCross += line.CrossSize
	}

	return totalCross
}

// JustifyInfo holds justify-content spacing information for a line.
type JustifyInfo struct {
	StartOffset  float64
	BetweenSpace float64
}

// justifyMainAxis calculates spacing for justify-content but doesn't modify items.
func justifyMainAxis(line *FlexLine, container *FlexContainer, availableMain float64) *JustifyInfo {
	info := &JustifyInfo{}

	if len(line.Items) == 0 {
		return info
	}

	// Calculate used main size
	usedMain := 0.0
	for _, item := range line.Items {
		usedMain += item.MainSize
		if container.IsRowDirection {
			usedMain += item.Box.Dimensions.Margin.Left + item.Box.Dimensions.Margin.Right
		} else {
			usedMain += item.Box.Dimensions.Margin.Top + item.Box.Dimensions.Margin.Bottom
		}
	}

	freeSpace := availableMain - usedMain
	if freeSpace < 0 {
		freeSpace = 0
	}

	line.MainSize = usedMain
	numItems := len(line.Items)

	// Calculate spacing based on justify-content
	switch container.JustifyContent {
	case JustifyFlexStart:
		info.StartOffset = 0
		info.BetweenSpace = 0
	case JustifyFlexEnd:
		info.StartOffset = freeSpace
		info.BetweenSpace = 0
	case JustifyCenter:
		info.StartOffset = freeSpace / 2
		info.BetweenSpace = 0
	case JustifySpaceBetween:
		info.StartOffset = 0
		if numItems > 1 {
			info.BetweenSpace = freeSpace / float64(numItems-1)
		}
	case JustifySpaceAround:
		if numItems > 0 {
			info.BetweenSpace = freeSpace / float64(numItems)
			info.StartOffset = info.BetweenSpace / 2
		}
	case JustifySpaceEvenly:
		if numItems > 0 {
			info.BetweenSpace = freeSpace / float64(numItems+1)
			info.StartOffset = info.BetweenSpace
		}
	}

	return info
}

// positionFlexItems positions all flex items in the container.
func positionFlexItems(box *LayoutBox, lines []*FlexLine, container *FlexContainer, justifyInfos []*JustifyInfo) {
	var crossPos float64

	// Handle reverse directions
	isMainReverse := container.Direction == FlexDirectionRowReverse ||
		container.Direction == FlexDirectionColumnReverse

	contentX := box.Dimensions.Content.X
	contentY := box.Dimensions.Content.Y
	containerMain := box.Dimensions.Content.Width
	if !container.IsRowDirection {
		containerMain = box.Dimensions.Content.Height
	}

	for lineIdx, line := range lines {
		justifyInfo := justifyInfos[lineIdx]

		// Start position for this line
		var lineMainPos float64
		if isMainReverse {
			lineMainPos = containerMain
		} else {
			lineMainPos = justifyInfo.StartOffset
		}

		for i, item := range line.Items {
			itemBox := item.Box

			// Get margins for this item
			var marginStart, marginEnd float64
			if container.IsRowDirection {
				marginStart = itemBox.Dimensions.Margin.Left
				marginEnd = itemBox.Dimensions.Margin.Right
			} else {
				marginStart = itemBox.Dimensions.Margin.Top
				marginEnd = itemBox.Dimensions.Margin.Bottom
			}

			// Calculate main axis position
			var itemMainPos float64
			if isMainReverse {
				lineMainPos -= item.MainSize + marginEnd
				itemMainPos = lineMainPos
			} else {
				itemMainPos = lineMainPos + marginStart
				lineMainPos = itemMainPos + item.MainSize + marginEnd
				// Add between-space for next item
				if i < len(line.Items)-1 {
					lineMainPos += justifyInfo.BetweenSpace
				}
			}

			// Calculate cross axis position based on alignment
			alignSelf := item.AlignSelf
			if alignSelf == AlignSelfAuto {
				switch container.AlignItems {
				case AlignItemsFlexStart:
					alignSelf = AlignSelfFlexStart
				case AlignItemsFlexEnd:
					alignSelf = AlignSelfFlexEnd
				case AlignItemsCenter:
					alignSelf = AlignSelfCenter
				default:
					alignSelf = AlignSelfStretch
				}
			}

			var itemCrossPos float64
			var crossMarginStart float64
			if container.IsRowDirection {
				crossMarginStart = itemBox.Dimensions.Margin.Top
			} else {
				crossMarginStart = itemBox.Dimensions.Margin.Left
			}

			switch alignSelf {
			case AlignSelfFlexStart:
				itemCrossPos = crossPos + crossMarginStart
			case AlignSelfFlexEnd:
				var crossMarginEnd float64
				if container.IsRowDirection {
					crossMarginEnd = itemBox.Dimensions.Margin.Bottom
				} else {
					crossMarginEnd = itemBox.Dimensions.Margin.Right
				}
				itemCrossPos = crossPos + line.CrossSize - item.CrossSize - crossMarginEnd
			case AlignSelfCenter:
				itemCrossPos = crossPos + (line.CrossSize-item.CrossSize)/2
			default: // AlignSelfStretch
				itemCrossPos = crossPos + crossMarginStart
			}

			// Set final content dimensions
			if container.IsRowDirection {
				itemBox.Dimensions.Content.Width = item.MainSize -
					itemBox.Dimensions.Padding.Left - itemBox.Dimensions.Padding.Right -
					itemBox.Dimensions.Border.Left - itemBox.Dimensions.Border.Right
				itemBox.Dimensions.Content.Height = item.CrossSize -
					itemBox.Dimensions.Padding.Top - itemBox.Dimensions.Padding.Bottom -
					itemBox.Dimensions.Border.Top - itemBox.Dimensions.Border.Bottom

				itemBox.Dimensions.Content.X = contentX + itemMainPos +
					itemBox.Dimensions.Border.Left + itemBox.Dimensions.Padding.Left
				itemBox.Dimensions.Content.Y = contentY + itemCrossPos +
					itemBox.Dimensions.Border.Top + itemBox.Dimensions.Padding.Top
			} else {
				itemBox.Dimensions.Content.Height = item.MainSize -
					itemBox.Dimensions.Padding.Top - itemBox.Dimensions.Padding.Bottom -
					itemBox.Dimensions.Border.Top - itemBox.Dimensions.Border.Bottom
				itemBox.Dimensions.Content.Width = item.CrossSize -
					itemBox.Dimensions.Padding.Left - itemBox.Dimensions.Padding.Right -
					itemBox.Dimensions.Border.Left - itemBox.Dimensions.Border.Right

				itemBox.Dimensions.Content.Y = contentY + itemMainPos +
					itemBox.Dimensions.Border.Top + itemBox.Dimensions.Padding.Top
				itemBox.Dimensions.Content.X = contentX + itemCrossPos +
					itemBox.Dimensions.Border.Left + itemBox.Dimensions.Padding.Left
			}

			// Ensure dimensions are non-negative
			if itemBox.Dimensions.Content.Width < 0 {
				itemBox.Dimensions.Content.Width = 0
			}
			if itemBox.Dimensions.Content.Height < 0 {
				itemBox.Dimensions.Content.Height = 0
			}

			// Layout children of flex item
			layoutFlexItemChildren(itemBox)
		}

		crossPos += line.CrossSize
	}
}

// layoutFlexItemChildren lays out the children of a flex item.
func layoutFlexItemChildren(box *LayoutBox) {
	ctx := &LayoutContext{
		ViewportWidth:    box.Dimensions.Content.Width,
		ViewportHeight:   box.Dimensions.Content.Height,
		ContainingBlocks: []*Dimensions{&box.Dimensions},
	}

	currentY := box.Dimensions.Content.Y
	for _, child := range box.Children {
		child.Layout(ctx)
		childHeight := child.Dimensions.MarginBox().Height
		child.Dimensions.Content.Y = currentY + child.Dimensions.Margin.Top +
			child.Dimensions.Border.Top + child.Dimensions.Padding.Top
		currentY += childHeight
	}
}

// calculateColumnFlexHeight calculates total height for column flex containers.
func calculateColumnFlexHeight(lines []*FlexLine) float64 {
	maxHeight := 0.0
	for _, line := range lines {
		lineHeight := 0.0
		for _, item := range line.Items {
			itemHeight := item.MainSize
			if container := item.Box; container != nil {
				itemHeight += item.Box.Dimensions.Margin.Top + item.Box.Dimensions.Margin.Bottom
			}
			lineHeight += itemHeight
		}
		if lineHeight > maxHeight {
			maxHeight = lineHeight
		}
	}
	return maxHeight
}

// Helper to get max of two floats
func maxFloat(a, b float64) float64 {
	return math.Max(a, b)
}

// Helper to get min of two floats
func minFloat(a, b float64) float64 {
	return math.Min(a, b)
}
