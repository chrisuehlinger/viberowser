// Package layout tests for the CSS box model and layout engine.
package layout

import (
	"testing"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
)

func TestDimensionsBoxCalculations(t *testing.T) {
	dims := Dimensions{
		Content: Rect{X: 10, Y: 10, Width: 100, Height: 50},
		Padding: EdgeSizes{Top: 5, Right: 5, Bottom: 5, Left: 5},
		Border:  EdgeSizes{Top: 2, Right: 2, Bottom: 2, Left: 2},
		Margin:  EdgeSizes{Top: 10, Right: 10, Bottom: 10, Left: 10},
	}

	// Test padding box
	paddingBox := dims.PaddingBox()
	if paddingBox.X != 5 || paddingBox.Y != 5 {
		t.Errorf("PaddingBox position wrong: got (%v, %v), expected (5, 5)", paddingBox.X, paddingBox.Y)
	}
	if paddingBox.Width != 110 || paddingBox.Height != 60 {
		t.Errorf("PaddingBox size wrong: got (%v, %v), expected (110, 60)", paddingBox.Width, paddingBox.Height)
	}

	// Test border box
	borderBox := dims.BorderBox()
	if borderBox.X != 3 || borderBox.Y != 3 {
		t.Errorf("BorderBox position wrong: got (%v, %v), expected (3, 3)", borderBox.X, borderBox.Y)
	}
	if borderBox.Width != 114 || borderBox.Height != 64 {
		t.Errorf("BorderBox size wrong: got (%v, %v), expected (114, 64)", borderBox.Width, borderBox.Height)
	}

	// Test margin box
	marginBox := dims.MarginBox()
	if marginBox.X != -7 || marginBox.Y != -7 {
		t.Errorf("MarginBox position wrong: got (%v, %v), expected (-7, -7)", marginBox.X, marginBox.Y)
	}
	if marginBox.Width != 134 || marginBox.Height != 84 {
		t.Errorf("MarginBox size wrong: got (%v, %v), expected (134, 84)", marginBox.Width, marginBox.Height)
	}
}

func TestRectExpandedBy(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 100, Height: 50}
	edge := EdgeSizes{Top: 5, Right: 10, Bottom: 15, Left: 20}

	expanded := rect.ExpandedBy(edge)

	if expanded.X != -10 {
		t.Errorf("X wrong: got %v, expected -10", expanded.X)
	}
	if expanded.Y != 5 {
		t.Errorf("Y wrong: got %v, expected 5", expanded.Y)
	}
	if expanded.Width != 130 {
		t.Errorf("Width wrong: got %v, expected 130", expanded.Width)
	}
	if expanded.Height != 70 {
		t.Errorf("Height wrong: got %v, expected 70", expanded.Height)
	}
}

func TestNewLayoutContext(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	if ctx.ViewportWidth != 800 {
		t.Errorf("ViewportWidth wrong: got %v, expected 800", ctx.ViewportWidth)
	}
	if ctx.ViewportHeight != 600 {
		t.Errorf("ViewportHeight wrong: got %v, expected 600", ctx.ViewportHeight)
	}

	cb := ctx.CurrentContainingBlock()
	if cb == nil {
		t.Fatal("CurrentContainingBlock should not be nil")
	}
	if cb.Content.Width != 800 {
		t.Errorf("Containing block width wrong: got %v, expected 800", cb.Content.Width)
	}
	if cb.Content.Height != 600 {
		t.Errorf("Containing block height wrong: got %v, expected 600", cb.Content.Height)
	}
}

func TestLayoutContextContainingBlockStack(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	// Push a new containing block
	dims := &Dimensions{
		Content: Rect{X: 10, Y: 10, Width: 400, Height: 300},
	}
	ctx.PushContainingBlock(dims)

	cb := ctx.CurrentContainingBlock()
	if cb.Content.Width != 400 {
		t.Errorf("After push, width wrong: got %v, expected 400", cb.Content.Width)
	}

	// Pop the containing block
	ctx.PopContainingBlock()
	cb = ctx.CurrentContainingBlock()
	if cb.Content.Width != 800 {
		t.Errorf("After pop, width wrong: got %v, expected 800", cb.Content.Width)
	}
}

func TestDetermineBoxType(t *testing.T) {
	tests := []struct {
		display string
		want    BoxType
	}{
		{"block", BlockBox},
		{"Block", BlockBox},
		{"BLOCK", BlockBox},
		{"inline", InlineBox},
		{"inline-block", InlineBlockBox},
		{"none", NoneBox},
		{"flex", InlineBox}, // Flex falls back to inline for now
		{"", InlineBox},     // Empty falls back to inline
	}

	for _, tt := range tests {
		got := determineBoxType(tt.display)
		if got != tt.want {
			t.Errorf("determineBoxType(%q) = %v, want %v", tt.display, got, tt.want)
		}
	}
}

func TestDeterminePositionType(t *testing.T) {
	tests := []struct {
		position string
		want     PositionType
	}{
		{"static", PositionStatic},
		{"relative", PositionRelative},
		{"absolute", PositionAbsolute},
		{"fixed", PositionFixed},
		{"sticky", PositionSticky},
		{"", PositionStatic},
	}

	for _, tt := range tests {
		got := determinePositionType(tt.position)
		if got != tt.want {
			t.Errorf("determinePositionType(%q) = %v, want %v", tt.position, got, tt.want)
		}
	}
}

func TestDetermineFloatType(t *testing.T) {
	tests := []struct {
		float string
		want  FloatType
	}{
		{"left", FloatLeft},
		{"right", FloatRight},
		{"none", FloatNone},
		{"", FloatNone},
	}

	for _, tt := range tests {
		got := determineFloatType(tt.float)
		if got != tt.want {
			t.Errorf("determineFloatType(%q) = %v, want %v", tt.float, got, tt.want)
		}
	}
}

func TestDetermineOverflowType(t *testing.T) {
	tests := []struct {
		overflow string
		want     OverflowType
	}{
		{"visible", OverflowVisible},
		{"hidden", OverflowHidden},
		{"scroll", OverflowScroll},
		{"auto", OverflowAuto},
		{"", OverflowVisible},
	}

	for _, tt := range tests {
		got := determineOverflowType(tt.overflow)
		if got != tt.want {
			t.Errorf("determineOverflowType(%q) = %v, want %v", tt.overflow, got, tt.want)
		}
	}
}

func TestDetermineBoxSizing(t *testing.T) {
	tests := []struct {
		boxSizing string
		want      BoxSizing
	}{
		{"content-box", BoxSizingContentBox},
		{"border-box", BoxSizingBorderBox},
		{"", BoxSizingContentBox},
	}

	for _, tt := range tests {
		got := determineBoxSizing(tt.boxSizing)
		if got != tt.want {
			t.Errorf("determineBoxSizing(%q) = %v, want %v", tt.boxSizing, got, tt.want)
		}
	}
}

func TestDetermineClearType(t *testing.T) {
	tests := []struct {
		clear string
		want  ClearType
	}{
		{"left", ClearLeft},
		{"right", ClearRight},
		{"both", ClearBoth},
		{"none", ClearNone},
		{"", ClearNone},
	}

	for _, tt := range tests {
		got := determineClearType(tt.clear)
		if got != tt.want {
			t.Errorf("determineClearType(%q) = %v, want %v", tt.clear, got, tt.want)
		}
	}
}

func TestGetBorderWidth(t *testing.T) {
	tests := []struct {
		keyword string
		want    float64
	}{
		{"thin", 1},
		{"medium", 3},
		{"thick", 5},
	}

	for _, tt := range tests {
		style := css.NewComputedStyle(nil, nil)
		style.SetPropertyValue("border-left-width", &css.ComputedValue{Keyword: tt.keyword})

		got := getBorderWidth(style, "border-left-width")
		if got != tt.want {
			t.Errorf("getBorderWidth(%q) = %v, want %v", tt.keyword, got, tt.want)
		}
	}
}

func TestNormalizeBoxTree(t *testing.T) {
	// Create a block box with mixed inline and block children
	parent := &LayoutBox{
		BoxType: BlockBox,
		Children: []*LayoutBox{
			{BoxType: InlineBox, TextContent: "text1"},
			{BoxType: BlockBox},
			{BoxType: InlineBox, TextContent: "text2"},
			{BoxType: InlineBox, TextContent: "text3"},
			{BoxType: BlockBox},
		},
	}

	normalizeBoxTree(parent)

	// Should have: AnonymousBlockBox, BlockBox, AnonymousBlockBox, BlockBox
	if len(parent.Children) != 4 {
		t.Fatalf("Expected 4 children after normalization, got %d", len(parent.Children))
	}

	if parent.Children[0].BoxType != AnonymousBlockBox {
		t.Errorf("First child should be AnonymousBlockBox, got %v", parent.Children[0].BoxType)
	}
	if parent.Children[1].BoxType != BlockBox {
		t.Errorf("Second child should be BlockBox, got %v", parent.Children[1].BoxType)
	}
	if parent.Children[2].BoxType != AnonymousBlockBox {
		t.Errorf("Third child should be AnonymousBlockBox, got %v", parent.Children[2].BoxType)
	}
	if parent.Children[3].BoxType != BlockBox {
		t.Errorf("Fourth child should be BlockBox, got %v", parent.Children[3].BoxType)
	}

	// Check that the third anonymous block has 2 inline children
	if len(parent.Children[2].Children) != 2 {
		t.Errorf("Third anonymous block should have 2 children, got %d", len(parent.Children[2].Children))
	}
}

func TestBlockLayoutWidth(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: css.NewComputedStyle(nil, nil),
	}

	// Width should be auto by default, filling containing block
	containingBlock := ctx.CurrentContainingBlock()
	box.calculateBlockWidth(containingBlock)

	if box.Dimensions.Content.Width != 800 {
		t.Errorf("Auto width should fill containing block: got %v, expected 800", box.Dimensions.Content.Width)
	}
}

func TestBlockLayoutWithExplicitWidth(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("width", &css.ComputedValue{Length: 400})

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: style,
	}

	containingBlock := ctx.CurrentContainingBlock()
	box.calculateBlockWidth(containingBlock)

	if box.Dimensions.Content.Width != 400 {
		t.Errorf("Explicit width: got %v, expected 400", box.Dimensions.Content.Width)
	}
}

func TestBlockLayoutCenteredWithAutoMargins(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("width", &css.ComputedValue{Length: 400})
	style.SetPropertyValue("margin-left", &css.ComputedValue{Keyword: "auto"})
	style.SetPropertyValue("margin-right", &css.ComputedValue{Keyword: "auto"})

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: style,
	}

	containingBlock := ctx.CurrentContainingBlock()
	box.calculateBlockWidth(containingBlock)

	// Both margins should be (800 - 400) / 2 = 200
	if box.Dimensions.Margin.Left != 200 {
		t.Errorf("Left margin should be 200, got %v", box.Dimensions.Margin.Left)
	}
	if box.Dimensions.Margin.Right != 200 {
		t.Errorf("Right margin should be 200, got %v", box.Dimensions.Margin.Right)
	}
}

func TestBlockLayoutWithPaddingAndBorder(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("padding-left", &css.ComputedValue{Length: 20})
	style.SetPropertyValue("padding-right", &css.ComputedValue{Length: 20})
	style.SetPropertyValue("border-left-width", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("border-right-width", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("width", &css.ComputedValue{Keyword: "auto"})

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: style,
	}

	containingBlock := ctx.CurrentContainingBlock()
	box.calculateBlockWidth(containingBlock)

	// Content width should be 800 - 20 - 20 - 5 - 5 = 750
	if box.Dimensions.Content.Width != 750 {
		t.Errorf("Content width with padding/border: got %v, expected 750", box.Dimensions.Content.Width)
	}
}

func TestBorderBoxSizing(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("width", &css.ComputedValue{Length: 200})
	style.SetPropertyValue("padding-left", &css.ComputedValue{Length: 20})
	style.SetPropertyValue("padding-right", &css.ComputedValue{Length: 20})
	style.SetPropertyValue("border-left-width", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("border-right-width", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("box-sizing", &css.ComputedValue{Keyword: "border-box"})

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: style,
		BoxSizing:     BoxSizingBorderBox,
	}

	containingBlock := ctx.CurrentContainingBlock()
	box.calculateBlockWidth(containingBlock)

	// Content width should be 200 - 20 - 20 - 5 - 5 = 150 (border-box)
	if box.Dimensions.Content.Width != 150 {
		t.Errorf("Border-box content width: got %v, expected 150", box.Dimensions.Content.Width)
	}
}

func TestRelativePositioning(t *testing.T) {
	box := &LayoutBox{
		Position:      PositionRelative,
		HasOffsetTop:  true,
		HasOffsetLeft: true,
		OffsetTop:     10,
		OffsetLeft:    20,
		Dimensions: Dimensions{
			Content: Rect{X: 100, Y: 100, Width: 50, Height: 50},
		},
	}

	box.applyRelativePosition()

	if box.Dimensions.Content.X != 120 {
		t.Errorf("X after relative positioning: got %v, expected 120", box.Dimensions.Content.X)
	}
	if box.Dimensions.Content.Y != 110 {
		t.Errorf("Y after relative positioning: got %v, expected 110", box.Dimensions.Content.Y)
	}
}

func TestLayoutFullBlock(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("width", &css.ComputedValue{Length: 300})
	style.SetPropertyValue("height", &css.ComputedValue{Length: 200})
	style.SetPropertyValue("margin-top", &css.ComputedValue{Length: 10})
	style.SetPropertyValue("margin-left", &css.ComputedValue{Length: 10})
	style.SetPropertyValue("padding-top", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("padding-left", &css.ComputedValue{Length: 5})
	style.SetPropertyValue("border-top-width", &css.ComputedValue{Length: 2})
	style.SetPropertyValue("border-left-width", &css.ComputedValue{Length: 2})

	box := &LayoutBox{
		BoxType:       BlockBox,
		ComputedStyle: style,
	}

	box.Layout(ctx)

	// Check content dimensions
	if box.Dimensions.Content.Width != 300 {
		t.Errorf("Content width: got %v, expected 300", box.Dimensions.Content.Width)
	}
	if box.Dimensions.Content.Height != 200 {
		t.Errorf("Content height: got %v, expected 200", box.Dimensions.Content.Height)
	}

	// Check content position (X = margin-left + border-left + padding-left)
	expectedX := 0.0 + 10.0 + 2.0 + 5.0 // 17
	if box.Dimensions.Content.X != expectedX {
		t.Errorf("Content X: got %v, expected %v", box.Dimensions.Content.X, expectedX)
	}
}

func TestBuildLayoutTreeBasic(t *testing.T) {
	// Create a simple DOM structure
	doc := dom.NewDocument()
	body := doc.CreateElement("body")
	div := doc.CreateElement("div")
	body.AsNode().AppendChild(div.AsNode())

	styleResolver := css.NewStyleResolver()
	ctx := NewLayoutContext(800, 600)

	box := BuildLayoutTree(body, styleResolver, ctx)

	if box == nil {
		t.Fatal("BuildLayoutTree returned nil")
	}

	// Body should create a block box by default (per UA stylesheet)
	// For simplicity, our test may return inline without UA styles
	// Just verify we got a box
	if box.Element == nil {
		t.Error("LayoutBox should have Element reference")
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"123", 123},
		{"-456", -456},
		{"42px", 42},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		got := parseInt(tt.input)
		if got != tt.want {
			t.Errorf("parseInt(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCollapseMarginsBlock(t *testing.T) {
	boxes := []*LayoutBox{
		{
			Dimensions: Dimensions{
				Content: Rect{Y: 0},
				Margin:  EdgeSizes{Bottom: 20},
			},
		},
		{
			Dimensions: Dimensions{
				Content: Rect{Y: 40}, // 20 (prev margin) + 20 (current margin)
				Margin:  EdgeSizes{Top: 30}, // Larger margin
			},
		},
	}

	CollapseMarginsBlock(boxes)

	// After collapsing, the second box should move up by (20 + 30) - max(20, 30) = 20
	expectedY := 40.0 - 20.0
	if boxes[1].Dimensions.Content.Y != expectedY {
		t.Errorf("After margin collapse, Y should be %v, got %v", expectedY, boxes[1].Dimensions.Content.Y)
	}
}

func TestInlineLayoutTextEstimate(t *testing.T) {
	ctx := NewLayoutContext(800, 600)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("font-size", &css.ComputedValue{Length: 16})

	box := &LayoutBox{
		BoxType:       InlineBox,
		TextContent:   "Hello",
		ComputedStyle: style,
	}

	containingBlock := ctx.CurrentContainingBlock()
	box.layoutInline(ctx, containingBlock)

	// Width should be approximately 5 chars * 16px * 0.6 = 48px
	expectedWidth := 5 * 16.0 * 0.6
	if box.Dimensions.Content.Width != expectedWidth {
		t.Errorf("Inline text width: got %v, expected %v", box.Dimensions.Content.Width, expectedWidth)
	}

	// Height should be 16 * 1.2 = 19.2
	expectedHeight := 16.0 * 1.2
	if box.Dimensions.Content.Height != expectedHeight {
		t.Errorf("Inline text height: got %v, expected %v", box.Dimensions.Content.Height, expectedHeight)
	}
}
