// Package render handles painting/rendering of the layout tree.
// Reference: https://www.w3.org/TR/CSS2/zindex.html for painting order
package render

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/layout"
)

// Canvas represents the rendering surface.
type Canvas struct {
	Pixels []color.RGBA
	Width  int
	Height int
}

// NewCanvas creates a new canvas with the given dimensions.
func NewCanvas(width, height int) *Canvas {
	pixels := make([]color.RGBA, width*height)
	// Initialize to white
	white := color.RGBA{255, 255, 255, 255}
	for i := range pixels {
		pixels[i] = white
	}
	return &Canvas{
		Pixels: pixels,
		Width:  width,
		Height: height,
	}
}

// Paint renders a layout tree to the canvas following CSS painting order.
// Reference: https://www.w3.org/TR/CSS2/zindex.html#painting-order
func (c *Canvas) Paint(layoutRoot *layout.LayoutBox) {
	if layoutRoot == nil {
		return
	}

	// Build display list for efficient rendering
	displayList := c.buildDisplayList(layoutRoot)

	// Execute display list commands
	for _, cmd := range displayList {
		cmd.Execute(c)
	}
}

// DisplayCommand represents a single painting operation.
type DisplayCommand interface {
	Execute(c *Canvas)
}

// SolidColorCommand paints a solid color rectangle.
type SolidColorCommand struct {
	Color color.RGBA
	Rect  layout.Rect
}

// Execute paints the solid color rectangle.
func (cmd *SolidColorCommand) Execute(c *Canvas) {
	c.FillRect(
		int(cmd.Rect.X),
		int(cmd.Rect.Y),
		int(cmd.Rect.Width),
		int(cmd.Rect.Height),
		cmd.Color,
	)
}

// BorderCommand paints borders around a rectangle.
type BorderCommand struct {
	Color       color.RGBA
	Rect        layout.Rect
	TopWidth    float64
	RightWidth  float64
	BottomWidth float64
	LeftWidth   float64
	Style       string // solid, dashed, dotted, etc.
}

// Execute paints the borders.
func (cmd *BorderCommand) Execute(c *Canvas) {
	x := int(cmd.Rect.X)
	y := int(cmd.Rect.Y)
	w := int(cmd.Rect.Width)
	h := int(cmd.Rect.Height)

	// Draw each border edge
	// Top border
	if cmd.TopWidth > 0 {
		c.drawBorderEdge(x, y, w, int(cmd.TopWidth), cmd.Color, cmd.Style, "top")
	}

	// Right border
	if cmd.RightWidth > 0 {
		c.drawBorderEdge(x+w-int(cmd.RightWidth), y, int(cmd.RightWidth), h, cmd.Color, cmd.Style, "right")
	}

	// Bottom border
	if cmd.BottomWidth > 0 {
		c.drawBorderEdge(x, y+h-int(cmd.BottomWidth), w, int(cmd.BottomWidth), cmd.Color, cmd.Style, "bottom")
	}

	// Left border
	if cmd.LeftWidth > 0 {
		c.drawBorderEdge(x, y, int(cmd.LeftWidth), h, cmd.Color, cmd.Style, "left")
	}
}

// drawBorderEdge draws a single border edge with the given style.
func (c *Canvas) drawBorderEdge(x, y, width, height int, col color.RGBA, style, edge string) {
	switch style {
	case "dashed":
		c.drawDashedBorder(x, y, width, height, col, edge)
	case "dotted":
		c.drawDottedBorder(x, y, width, height, col, edge)
	case "double":
		c.drawDoubleBorder(x, y, width, height, col, edge)
	default: // solid
		c.FillRect(x, y, width, height, col)
	}
}

// drawDashedBorder draws a dashed border.
func (c *Canvas) drawDashedBorder(x, y, width, height int, col color.RGBA, edge string) {
	dashLength := 6
	gapLength := 3

	if edge == "top" || edge == "bottom" {
		for px := x; px < x+width; {
			end := px + dashLength
			if end > x+width {
				end = x + width
			}
			c.FillRect(px, y, end-px, height, col)
			px = end + gapLength
		}
	} else {
		for py := y; py < y+height; {
			end := py + dashLength
			if end > y+height {
				end = y + height
			}
			c.FillRect(x, py, width, end-py, col)
			py = end + gapLength
		}
	}
}

// drawDottedBorder draws a dotted border.
func (c *Canvas) drawDottedBorder(x, y, width, height int, col color.RGBA, edge string) {
	dotSize := 2
	gapSize := 2

	if edge == "top" || edge == "bottom" {
		for px := x; px < x+width; px += dotSize + gapSize {
			dotWidth := dotSize
			if px+dotWidth > x+width {
				dotWidth = x + width - px
			}
			c.FillRect(px, y, dotWidth, height, col)
		}
	} else {
		for py := y; py < y+height; py += dotSize + gapSize {
			dotHeight := dotSize
			if py+dotHeight > y+height {
				dotHeight = y + height - py
			}
			c.FillRect(x, py, width, dotHeight, col)
		}
	}
}

// drawDoubleBorder draws a double border.
func (c *Canvas) drawDoubleBorder(x, y, width, height int, col color.RGBA, edge string) {
	if edge == "top" || edge == "bottom" {
		lineHeight := height / 3
		if lineHeight < 1 {
			lineHeight = 1
		}
		c.FillRect(x, y, width, lineHeight, col)
		c.FillRect(x, y+height-lineHeight, width, lineHeight, col)
	} else {
		lineWidth := width / 3
		if lineWidth < 1 {
			lineWidth = 1
		}
		c.FillRect(x, y, lineWidth, height, col)
		c.FillRect(x+width-lineWidth, y, lineWidth, height, col)
	}
}

// TextCommand paints text at a position.
type TextCommand struct {
	Text      string
	X, Y      float64
	Color     color.RGBA
	FontSize  float64
	FontStyle string
	FontWeight string
}

// Execute paints the text.
func (cmd *TextCommand) Execute(c *Canvas) {
	// Basic text rendering using a simple bitmap approach
	// For proper text rendering, a font library would be needed
	c.drawText(cmd.Text, int(cmd.X), int(cmd.Y), cmd.Color, cmd.FontSize, cmd.FontWeight)
}

// PaintContext holds state during painting.
type PaintContext struct {
	DisplayList []DisplayCommand
}

// buildDisplayList builds a list of display commands from the layout tree.
func (c *Canvas) buildDisplayList(root *layout.LayoutBox) []DisplayCommand {
	ctx := &PaintContext{
		DisplayList: make([]DisplayCommand, 0),
	}

	// Collect all stacking contexts
	stackingContexts := collectStackingContexts(root)

	// Sort by z-index
	sort.SliceStable(stackingContexts, func(i, j int) bool {
		return stackingContexts[i].ZIndex < stackingContexts[j].ZIndex
	})

	// Paint each stacking context
	for _, sc := range stackingContexts {
		c.paintStackingContext(sc, ctx)
	}

	return ctx.DisplayList
}

// StackingContextEntry represents a box that creates a stacking context.
type StackingContextEntry struct {
	Box    *layout.LayoutBox
	ZIndex int
}

// collectStackingContexts collects all stacking contexts from the layout tree.
func collectStackingContexts(root *layout.LayoutBox) []*StackingContextEntry {
	var contexts []*StackingContextEntry

	// The root always creates a stacking context
	contexts = append(contexts, &StackingContextEntry{
		Box:    root,
		ZIndex: root.ZIndex,
	})

	collectStackingContextsRecursive(root, &contexts)
	return contexts
}

func collectStackingContextsRecursive(box *layout.LayoutBox, contexts *[]*StackingContextEntry) {
	for _, child := range box.Children {
		if child.IsStackingContext {
			*contexts = append(*contexts, &StackingContextEntry{
				Box:    child,
				ZIndex: child.ZIndex,
			})
		}
		collectStackingContextsRecursive(child, contexts)
	}
}

// paintStackingContext paints a stacking context and its contents.
func (c *Canvas) paintStackingContext(sc *StackingContextEntry, ctx *PaintContext) {
	// CSS 2.1 Appendix E - Painting order:
	// 1. Background and borders of the element forming the stacking context
	// 2. Child stacking contexts with negative z-index
	// 3. In-flow, non-positioned, block-level descendants
	// 4. Non-positioned floats
	// 5. In-flow, non-positioned, inline-level descendants
	// 6. Child stacking contexts with z-index: auto or 0
	// 7. Child stacking contexts with positive z-index

	box := sc.Box

	// Skip if display: none
	if box.BoxType == layout.NoneBox {
		return
	}

	// 1. Paint background and borders
	c.paintBackground(box, ctx)
	c.paintBorders(box, ctx)

	// Paint children (simplified - full implementation would separate by category)
	c.paintChildren(box, ctx)
}

// paintBackground paints the background of a box.
func (c *Canvas) paintBackground(box *layout.LayoutBox, ctx *PaintContext) {
	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Get background color
	bgColor := getBackgroundColor(style)
	if bgColor.A == 0 {
		// Transparent background
		return
	}

	// Paint background over the border box
	borderBox := box.Dimensions.BorderBox()

	ctx.DisplayList = append(ctx.DisplayList, &SolidColorCommand{
		Color: bgColor,
		Rect:  borderBox,
	})
}

// paintBorders paints the borders of a box.
func (c *Canvas) paintBorders(box *layout.LayoutBox, ctx *PaintContext) {
	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Check if any border is visible
	topWidth := box.Dimensions.Border.Top
	rightWidth := box.Dimensions.Border.Right
	bottomWidth := box.Dimensions.Border.Bottom
	leftWidth := box.Dimensions.Border.Left

	if topWidth == 0 && rightWidth == 0 && bottomWidth == 0 && leftWidth == 0 {
		return
	}

	// Get border styles and colors
	borderBox := box.Dimensions.BorderBox()

	// Get border colors (default to currentColor/black)
	borderColor := getBorderColor(style, "border-top-color")
	borderStyle := getBorderStyle(style, "border-top-style")

	// Only paint if border style is not "none" or "hidden"
	if borderStyle == "none" || borderStyle == "hidden" {
		return
	}

	ctx.DisplayList = append(ctx.DisplayList, &BorderCommand{
		Color:       borderColor,
		Rect:        borderBox,
		TopWidth:    topWidth,
		RightWidth:  rightWidth,
		BottomWidth: bottomWidth,
		LeftWidth:   leftWidth,
		Style:       borderStyle,
	})
}

// paintChildren paints the children of a box.
func (c *Canvas) paintChildren(box *layout.LayoutBox, ctx *PaintContext) {
	for _, child := range box.Children {
		// Skip stacking contexts - they're painted separately
		if child.IsStackingContext {
			continue
		}

		// Paint inline text content
		if child.TextContent != "" {
			c.paintText(child, ctx)
		} else {
			// Recursively paint child boxes
			c.paintBackground(child, ctx)
			c.paintBorders(child, ctx)
			c.paintChildren(child, ctx)
		}
	}
}

// paintText paints text content.
func (c *Canvas) paintText(box *layout.LayoutBox, ctx *PaintContext) {
	style := box.ComputedStyle
	if style == nil {
		return
	}

	// Get text color
	textColor := getTextColor(style)

	// Get font properties
	fontSize := getFontSize(style)
	fontWeight := getFontWeight(style)
	fontStyle := getFontStyle(style)

	// Calculate text position
	x := box.Dimensions.Content.X
	y := box.Dimensions.Content.Y

	ctx.DisplayList = append(ctx.DisplayList, &TextCommand{
		Text:       box.TextContent,
		X:          x,
		Y:          y,
		Color:      textColor,
		FontSize:   fontSize,
		FontWeight: fontWeight,
		FontStyle:  fontStyle,
	})
}

// getBackgroundColor extracts the background color from computed style.
func getBackgroundColor(style *css.ComputedStyle) color.RGBA {
	val := style.GetPropertyValue("background-color")
	if val == nil {
		return color.RGBA{0, 0, 0, 0} // Transparent
	}

	// Check for transparent keyword
	if val.Keyword == "transparent" {
		return color.RGBA{0, 0, 0, 0}
	}

	cssColor := val.Color
	return color.RGBA{
		R: cssColor.R,
		G: cssColor.G,
		B: cssColor.B,
		A: cssColor.A,
	}
}

// getBorderColor extracts the border color from computed style.
func getBorderColor(style *css.ComputedStyle, property string) color.RGBA {
	val := style.GetPropertyValue(property)
	if val == nil {
		// Default to currentColor (which defaults to black for now)
		return color.RGBA{0, 0, 0, 255}
	}

	// Handle currentcolor keyword
	if val.Keyword == "currentcolor" || val.Keyword == "currentColor" {
		// Get the color property
		colorVal := style.GetPropertyValue("color")
		if colorVal != nil {
			cssColor := colorVal.Color
			return color.RGBA{R: cssColor.R, G: cssColor.G, B: cssColor.B, A: cssColor.A}
		}
		return color.RGBA{0, 0, 0, 255}
	}

	cssColor := val.Color
	return color.RGBA{
		R: cssColor.R,
		G: cssColor.G,
		B: cssColor.B,
		A: cssColor.A,
	}
}

// getBorderStyle extracts the border style from computed style.
func getBorderStyle(style *css.ComputedStyle, property string) string {
	val := style.GetPropertyValue(property)
	if val == nil {
		return "none"
	}
	if val.Keyword != "" {
		return val.Keyword
	}
	return "solid"
}

// getTextColor extracts the text color from computed style.
func getTextColor(style *css.ComputedStyle) color.RGBA {
	val := style.GetPropertyValue("color")
	if val == nil {
		return color.RGBA{0, 0, 0, 255} // Black
	}

	cssColor := val.Color
	if cssColor.A == 0 && cssColor.R == 0 && cssColor.G == 0 && cssColor.B == 0 {
		// If color is not set, default to black
		return color.RGBA{0, 0, 0, 255}
	}

	return color.RGBA{
		R: cssColor.R,
		G: cssColor.G,
		B: cssColor.B,
		A: cssColor.A,
	}
}

// getFontSize extracts the font size from computed style.
func getFontSize(style *css.ComputedStyle) float64 {
	val := style.GetPropertyValue("font-size")
	if val == nil {
		return 16.0 // Default
	}
	if val.Length > 0 {
		return val.Length
	}
	return 16.0
}

// getFontWeight extracts the font weight from computed style.
func getFontWeight(style *css.ComputedStyle) string {
	val := style.GetPropertyValue("font-weight")
	if val == nil {
		return "normal"
	}
	if val.Keyword != "" {
		return val.Keyword
	}
	return "normal"
}

// getFontStyle extracts the font style from computed style.
func getFontStyle(style *css.ComputedStyle) string {
	val := style.GetPropertyValue("font-style")
	if val == nil {
		return "normal"
	}
	if val.Keyword != "" {
		return val.Keyword
	}
	return "normal"
}

// SetPixel sets a single pixel on the canvas.
func (c *Canvas) SetPixel(x, y int, col color.RGBA) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.Pixels[y*c.Width+x] = col
	}
}

// SetPixelBlend sets a pixel with alpha compositing.
func (c *Canvas) SetPixelBlend(x, y int, col color.RGBA) {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}

	idx := y*c.Width + x
	dst := c.Pixels[idx]

	// Alpha compositing (Porter-Duff source over)
	srcA := float64(col.A) / 255.0
	dstA := float64(dst.A) / 255.0
	outA := srcA + dstA*(1-srcA)

	if outA == 0 {
		c.Pixels[idx] = color.RGBA{0, 0, 0, 0}
		return
	}

	outR := (float64(col.R)*srcA + float64(dst.R)*dstA*(1-srcA)) / outA
	outG := (float64(col.G)*srcA + float64(dst.G)*dstA*(1-srcA)) / outA
	outB := (float64(col.B)*srcA + float64(dst.B)*dstA*(1-srcA)) / outA

	c.Pixels[idx] = color.RGBA{
		R: uint8(math.Round(outR)),
		G: uint8(math.Round(outG)),
		B: uint8(math.Round(outB)),
		A: uint8(math.Round(outA * 255)),
	}
}

// FillRect fills a rectangle with the given color.
func (c *Canvas) FillRect(x, y, width, height int, col color.RGBA) {
	// Clip to canvas bounds
	x1 := max(x, 0)
	y1 := max(y, 0)
	x2 := min(x+width, c.Width)
	y2 := min(y+height, c.Height)

	// Use blending if alpha is not fully opaque
	if col.A < 255 {
		for py := y1; py < y2; py++ {
			for px := x1; px < x2; px++ {
				c.SetPixelBlend(px, py, col)
			}
		}
	} else {
		for py := y1; py < y2; py++ {
			for px := x1; px < x2; px++ {
				c.Pixels[py*c.Width+px] = col
			}
		}
	}
}

// FillRectBlend fills a rectangle with alpha compositing.
func (c *Canvas) FillRectBlend(x, y, width, height int, col color.RGBA) {
	for py := y; py < y+height; py++ {
		for px := x; px < x+width; px++ {
			c.SetPixelBlend(px, py, col)
		}
	}
}

// ToImage converts the canvas to a Go image.Image.
func (c *Canvas) ToImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			img.Set(x, y, c.Pixels[y*c.Width+x])
		}
	}
	return img
}

// Basic bitmap font for text rendering (5x7 pixel characters)
// This is a minimal implementation - for production use, integrate a proper font library
var bitmapFont = map[rune][]uint8{
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0E},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J': {0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C},
	'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0E, 0x11, 0x10, 0x0E, 0x01, 0x11, 0x0E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'a': {0x00, 0x00, 0x0E, 0x01, 0x0F, 0x11, 0x0F},
	'b': {0x10, 0x10, 0x16, 0x19, 0x11, 0x11, 0x1E},
	'c': {0x00, 0x00, 0x0E, 0x10, 0x10, 0x11, 0x0E},
	'd': {0x01, 0x01, 0x0D, 0x13, 0x11, 0x11, 0x0F},
	'e': {0x00, 0x00, 0x0E, 0x11, 0x1F, 0x10, 0x0E},
	'f': {0x06, 0x09, 0x08, 0x1C, 0x08, 0x08, 0x08},
	'g': {0x00, 0x0F, 0x11, 0x11, 0x0F, 0x01, 0x0E},
	'h': {0x10, 0x10, 0x16, 0x19, 0x11, 0x11, 0x11},
	'i': {0x04, 0x00, 0x0C, 0x04, 0x04, 0x04, 0x0E},
	'j': {0x02, 0x00, 0x06, 0x02, 0x02, 0x12, 0x0C},
	'k': {0x10, 0x10, 0x12, 0x14, 0x18, 0x14, 0x12},
	'l': {0x0C, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'm': {0x00, 0x00, 0x1A, 0x15, 0x15, 0x11, 0x11},
	'n': {0x00, 0x00, 0x16, 0x19, 0x11, 0x11, 0x11},
	'o': {0x00, 0x00, 0x0E, 0x11, 0x11, 0x11, 0x0E},
	'p': {0x00, 0x00, 0x1E, 0x11, 0x1E, 0x10, 0x10},
	'q': {0x00, 0x00, 0x0D, 0x13, 0x0F, 0x01, 0x01},
	'r': {0x00, 0x00, 0x16, 0x19, 0x10, 0x10, 0x10},
	's': {0x00, 0x00, 0x0E, 0x10, 0x0E, 0x01, 0x1E},
	't': {0x08, 0x08, 0x1C, 0x08, 0x08, 0x09, 0x06},
	'u': {0x00, 0x00, 0x11, 0x11, 0x11, 0x13, 0x0D},
	'v': {0x00, 0x00, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'w': {0x00, 0x00, 0x11, 0x11, 0x15, 0x15, 0x0A},
	'x': {0x00, 0x00, 0x11, 0x0A, 0x04, 0x0A, 0x11},
	'y': {0x00, 0x00, 0x11, 0x11, 0x0F, 0x01, 0x0E},
	'z': {0x00, 0x00, 0x1F, 0x02, 0x04, 0x08, 0x1F},
	'0': {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1': {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'2': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3': {0x1F, 0x02, 0x04, 0x02, 0x01, 0x11, 0x0E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
	' ': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	'.': {0x00, 0x00, 0x00, 0x00, 0x00, 0x0C, 0x0C},
	',': {0x00, 0x00, 0x00, 0x00, 0x0C, 0x0C, 0x08},
	':': {0x00, 0x0C, 0x0C, 0x00, 0x0C, 0x0C, 0x00},
	';': {0x00, 0x0C, 0x0C, 0x00, 0x0C, 0x0C, 0x08},
	'!': {0x04, 0x04, 0x04, 0x04, 0x04, 0x00, 0x04},
	'?': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x00, 0x04},
	'-': {0x00, 0x00, 0x00, 0x1F, 0x00, 0x00, 0x00},
	'+': {0x00, 0x04, 0x04, 0x1F, 0x04, 0x04, 0x00},
	'=': {0x00, 0x00, 0x1F, 0x00, 0x1F, 0x00, 0x00},
	'(': {0x02, 0x04, 0x08, 0x08, 0x08, 0x04, 0x02},
	')': {0x08, 0x04, 0x02, 0x02, 0x02, 0x04, 0x08},
	'[': {0x0E, 0x08, 0x08, 0x08, 0x08, 0x08, 0x0E},
	']': {0x0E, 0x02, 0x02, 0x02, 0x02, 0x02, 0x0E},
	'{': {0x02, 0x04, 0x04, 0x08, 0x04, 0x04, 0x02},
	'}': {0x08, 0x04, 0x04, 0x02, 0x04, 0x04, 0x08},
	'/': {0x00, 0x01, 0x02, 0x04, 0x08, 0x10, 0x00},
	'\\': {0x00, 0x10, 0x08, 0x04, 0x02, 0x01, 0x00},
	'<': {0x02, 0x04, 0x08, 0x10, 0x08, 0x04, 0x02},
	'>': {0x08, 0x04, 0x02, 0x01, 0x02, 0x04, 0x08},
	'\'': {0x04, 0x04, 0x08, 0x00, 0x00, 0x00, 0x00},
	'"': {0x0A, 0x0A, 0x14, 0x00, 0x00, 0x00, 0x00},
	'_': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1F},
	'@': {0x0E, 0x11, 0x17, 0x15, 0x17, 0x10, 0x0E},
	'#': {0x0A, 0x0A, 0x1F, 0x0A, 0x1F, 0x0A, 0x0A},
	'$': {0x04, 0x0F, 0x14, 0x0E, 0x05, 0x1E, 0x04},
	'%': {0x18, 0x19, 0x02, 0x04, 0x08, 0x13, 0x03},
	'^': {0x04, 0x0A, 0x11, 0x00, 0x00, 0x00, 0x00},
	'&': {0x0C, 0x12, 0x14, 0x08, 0x15, 0x12, 0x0D},
	'*': {0x00, 0x04, 0x15, 0x0E, 0x15, 0x04, 0x00},
	'|': {0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'`': {0x08, 0x04, 0x02, 0x00, 0x00, 0x00, 0x00},
	'~': {0x00, 0x00, 0x08, 0x15, 0x02, 0x00, 0x00},
}

// drawText draws text at the given position using a simple bitmap font.
func (c *Canvas) drawText(text string, x, y int, col color.RGBA, fontSize float64, fontWeight string) {
	// Calculate scale factor based on font size (base font is 7 pixels tall)
	scale := fontSize / 7.0
	if scale < 1 {
		scale = 1
	}

	// Make text bold by drawing twice if weight is bold/bolder/700+
	bold := strings.ToLower(fontWeight) == "bold" || fontWeight == "700" ||
		fontWeight == "800" || fontWeight == "900"

	charWidth := int(5 * scale)
	charHeight := int(7 * scale)
	spacing := int(1 * scale)

	currentX := x
	for _, ch := range text {
		bitmap, ok := bitmapFont[ch]
		if !ok {
			// Use a default character for unknown characters
			bitmap = bitmapFont['?']
		}

		c.drawChar(currentX, y, bitmap, charWidth, charHeight, scale, col, bold)
		currentX += charWidth + spacing
	}
}

// drawChar draws a single character from the bitmap font.
func (c *Canvas) drawChar(x, y int, bitmap []uint8, width, height int, scale float64, textColor color.RGBA, bold bool) {
	for row := 0; row < 7; row++ {
		rowBits := bitmap[row]
		for colIdx := 0; colIdx < 5; colIdx++ {
			if rowBits&(0x10>>colIdx) != 0 {
				// Draw scaled pixel
				px := x + int(float64(colIdx)*scale)
				py := y + int(float64(row)*scale)
				pixelSize := int(scale)
				if pixelSize < 1 {
					pixelSize = 1
				}

				for dy := 0; dy < pixelSize; dy++ {
					for dx := 0; dx < pixelSize; dx++ {
						c.SetPixelBlend(px+dx, py+dy, textColor)
						// Draw extra pixel for bold
						if bold {
							c.SetPixelBlend(px+dx+1, py+dy, textColor)
						}
					}
				}
			}
		}
	}
}

// DrawLine draws a line from (x1, y1) to (x2, y2) using Bresenham's algorithm.
func (c *Canvas) DrawLine(x1, y1, x2, y2 int, col color.RGBA) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)

	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}

	err := dx - dy

	for {
		c.SetPixelBlend(x1, y1, col)

		if x1 == x2 && y1 == y2 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// DrawCircle draws a circle outline using Bresenham's circle algorithm.
func (c *Canvas) DrawCircle(cx, cy, radius int, col color.RGBA) {
	x := radius
	y := 0
	err := 0

	for x >= y {
		c.SetPixelBlend(cx+x, cy+y, col)
		c.SetPixelBlend(cx+y, cy+x, col)
		c.SetPixelBlend(cx-y, cy+x, col)
		c.SetPixelBlend(cx-x, cy+y, col)
		c.SetPixelBlend(cx-x, cy-y, col)
		c.SetPixelBlend(cx-y, cy-x, col)
		c.SetPixelBlend(cx+y, cy-x, col)
		c.SetPixelBlend(cx+x, cy-y, col)

		if err <= 0 {
			y++
			err += 2*y + 1
		}
		if err > 0 {
			x--
			err -= 2*x + 1
		}
	}
}

// FillCircle fills a circle using Bresenham's algorithm.
func (c *Canvas) FillCircle(cx, cy, radius int, col color.RGBA) {
	x := radius
	y := 0
	err := 0

	for x >= y {
		c.drawHorizontalLine(cx-x, cx+x, cy+y, col)
		c.drawHorizontalLine(cx-x, cx+x, cy-y, col)
		c.drawHorizontalLine(cx-y, cx+y, cy+x, col)
		c.drawHorizontalLine(cx-y, cx+y, cy-x, col)

		if err <= 0 {
			y++
			err += 2*y + 1
		}
		if err > 0 {
			x--
			err -= 2*x + 1
		}
	}
}

// drawHorizontalLine draws a horizontal line.
func (c *Canvas) drawHorizontalLine(x1, x2, y int, col color.RGBA) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		c.SetPixelBlend(x, y, col)
	}
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Clear clears the canvas to the given color.
func (c *Canvas) Clear(col color.RGBA) {
	for i := range c.Pixels {
		c.Pixels[i] = col
	}
}

// ClearToWhite clears the canvas to white.
func (c *Canvas) ClearToWhite() {
	c.Clear(color.RGBA{255, 255, 255, 255})
}

// GetPixel returns the color of a pixel at the given coordinates.
func (c *Canvas) GetPixel(x, y int) color.RGBA {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return color.RGBA{0, 0, 0, 0}
	}
	return c.Pixels[y*c.Width+x]
}

// Clone creates a copy of the canvas.
func (c *Canvas) Clone() *Canvas {
	newPixels := make([]color.RGBA, len(c.Pixels))
	copy(newPixels, c.Pixels)
	return &Canvas{
		Pixels: newPixels,
		Width:  c.Width,
		Height: c.Height,
	}
}

// DrawImage draws another canvas onto this canvas at the given position.
func (c *Canvas) DrawImage(src *Canvas, x, y int) {
	for sy := 0; sy < src.Height; sy++ {
		for sx := 0; sx < src.Width; sx++ {
			srcColor := src.Pixels[sy*src.Width+sx]
			c.SetPixelBlend(x+sx, y+sy, srcColor)
		}
	}
}

// DrawImageScaled draws another canvas onto this canvas, scaled to fit the given dimensions.
func (c *Canvas) DrawImageScaled(src *Canvas, x, y, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	scaleX := float64(src.Width) / float64(width)
	scaleY := float64(src.Height) / float64(height)

	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			sx := int(float64(dx) * scaleX)
			sy := int(float64(dy) * scaleY)
			if sx < src.Width && sy < src.Height {
				srcColor := src.Pixels[sy*src.Width+sx]
				c.SetPixelBlend(x+dx, y+dy, srcColor)
			}
		}
	}
}
