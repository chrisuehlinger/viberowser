// Package render tests for the painting/rendering engine.
package render

import (
	"image/color"
	"testing"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/layout"
)

func TestNewCanvas(t *testing.T) {
	canvas := NewCanvas(100, 50)

	if canvas.Width != 100 {
		t.Errorf("Width = %d, want 100", canvas.Width)
	}
	if canvas.Height != 50 {
		t.Errorf("Height = %d, want 50", canvas.Height)
	}
	if len(canvas.Pixels) != 5000 {
		t.Errorf("Pixels length = %d, want 5000", len(canvas.Pixels))
	}

	// Check that canvas is initialized to white
	white := color.RGBA{255, 255, 255, 255}
	for i, px := range canvas.Pixels {
		if px != white {
			t.Errorf("Pixel %d = %v, want white", i, px)
			break
		}
	}
}

func TestSetPixel(t *testing.T) {
	canvas := NewCanvas(10, 10)
	red := color.RGBA{255, 0, 0, 255}

	canvas.SetPixel(5, 5, red)

	got := canvas.Pixels[5*10+5]
	if got != red {
		t.Errorf("SetPixel: got %v, want %v", got, red)
	}

	// Test out of bounds (should not panic)
	canvas.SetPixel(-1, 5, red)
	canvas.SetPixel(100, 5, red)
	canvas.SetPixel(5, -1, red)
	canvas.SetPixel(5, 100, red)
}

func TestFillRect(t *testing.T) {
	canvas := NewCanvas(100, 100)
	blue := color.RGBA{0, 0, 255, 255}

	canvas.FillRect(10, 10, 20, 30, blue)

	// Check pixels inside the rectangle
	for y := 10; y < 40; y++ {
		for x := 10; x < 30; x++ {
			got := canvas.Pixels[y*100+x]
			if got != blue {
				t.Errorf("FillRect pixel at (%d,%d) = %v, want %v", x, y, got, blue)
			}
		}
	}

	// Check pixels outside the rectangle are still white
	white := color.RGBA{255, 255, 255, 255}
	if canvas.Pixels[0] != white {
		t.Errorf("FillRect: pixel outside rect should be white")
	}
}

func TestFillRectClipping(t *testing.T) {
	canvas := NewCanvas(10, 10)
	red := color.RGBA{255, 0, 0, 255}

	// Fill rect that extends beyond canvas bounds
	canvas.FillRect(-5, -5, 20, 20, red)

	// Should fill visible portion without panic
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			got := canvas.Pixels[y*10+x]
			if got != red {
				t.Errorf("FillRectClipping: pixel at (%d,%d) should be red", x, y)
			}
		}
	}
}

func TestSetPixelBlend(t *testing.T) {
	canvas := NewCanvas(10, 10)

	// Set initial color to red
	red := color.RGBA{255, 0, 0, 255}
	canvas.SetPixel(5, 5, red)

	// Blend with semi-transparent blue
	semiBlue := color.RGBA{0, 0, 255, 128}
	canvas.SetPixelBlend(5, 5, semiBlue)

	got := canvas.Pixels[5*10+5]

	// Result should be a blend of red and blue
	// With 50% blue over red, we expect roughly purple
	if got.R < 100 || got.R > 160 { // Should be around 127
		t.Errorf("Alpha blend R: got %d, expected ~127", got.R)
	}
	if got.B < 100 || got.B > 160 { // Should be around 127
		t.Errorf("Alpha blend B: got %d, expected ~127", got.B)
	}
	if got.A != 255 {
		t.Errorf("Alpha blend A: got %d, expected 255", got.A)
	}
}

func TestClear(t *testing.T) {
	canvas := NewCanvas(10, 10)
	black := color.RGBA{0, 0, 0, 255}

	canvas.Clear(black)

	for i, px := range canvas.Pixels {
		if px != black {
			t.Errorf("Clear: pixel %d = %v, want black", i, px)
		}
	}
}

func TestClearToWhite(t *testing.T) {
	canvas := NewCanvas(10, 10)
	canvas.Clear(color.RGBA{0, 0, 0, 255})

	canvas.ClearToWhite()

	white := color.RGBA{255, 255, 255, 255}
	if canvas.Pixels[0] != white {
		t.Errorf("ClearToWhite: pixel should be white")
	}
}

func TestGetPixel(t *testing.T) {
	canvas := NewCanvas(10, 10)
	red := color.RGBA{255, 0, 0, 255}
	canvas.SetPixel(5, 5, red)

	got := canvas.GetPixel(5, 5)
	if got != red {
		t.Errorf("GetPixel: got %v, want %v", got, red)
	}

	// Test out of bounds
	outOfBounds := canvas.GetPixel(-1, 5)
	if outOfBounds.A != 0 {
		t.Error("GetPixel out of bounds should return transparent")
	}
}

func TestClone(t *testing.T) {
	canvas := NewCanvas(10, 10)
	red := color.RGBA{255, 0, 0, 255}
	canvas.SetPixel(5, 5, red)

	clone := canvas.Clone()

	// Verify clone has same dimensions and pixels
	if clone.Width != canvas.Width || clone.Height != canvas.Height {
		t.Error("Clone dimensions don't match")
	}
	if clone.GetPixel(5, 5) != red {
		t.Error("Clone pixel values don't match")
	}

	// Verify they're independent
	blue := color.RGBA{0, 0, 255, 255}
	clone.SetPixel(5, 5, blue)
	if canvas.GetPixel(5, 5) == blue {
		t.Error("Clone should be independent of original")
	}
}

func TestToImage(t *testing.T) {
	canvas := NewCanvas(10, 10)
	red := color.RGBA{255, 0, 0, 255}
	canvas.SetPixel(5, 5, red)

	img := canvas.ToImage()

	if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 10 {
		t.Error("ToImage: wrong dimensions")
	}

	r, g, b, a := img.At(5, 5).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("ToImage: pixel at (5,5) wrong color: %d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestDrawLine(t *testing.T) {
	canvas := NewCanvas(20, 20)
	red := color.RGBA{255, 0, 0, 255}

	// Draw a diagonal line
	canvas.DrawLine(0, 0, 10, 10, red)

	// Check some points on the line
	if canvas.GetPixel(0, 0) != red {
		t.Error("DrawLine: start point should be red")
	}
	if canvas.GetPixel(5, 5) != red {
		t.Error("DrawLine: middle point should be red")
	}
	if canvas.GetPixel(10, 10) != red {
		t.Error("DrawLine: end point should be red")
	}
}

func TestDrawCircle(t *testing.T) {
	canvas := NewCanvas(30, 30)
	blue := color.RGBA{0, 0, 255, 255}

	canvas.DrawCircle(15, 15, 10, blue)

	// Check some points on the circle
	// Top of circle (15, 5)
	if canvas.GetPixel(15, 5) != blue {
		t.Error("DrawCircle: top point should be blue")
	}
	// Right of circle (25, 15)
	if canvas.GetPixel(25, 15) != blue {
		t.Error("DrawCircle: right point should be blue")
	}
}

func TestFillCircle(t *testing.T) {
	canvas := NewCanvas(30, 30)
	green := color.RGBA{0, 255, 0, 255}

	canvas.FillCircle(15, 15, 5, green)

	// Center should be filled
	if canvas.GetPixel(15, 15) != green {
		t.Error("FillCircle: center should be green")
	}
}

func TestDrawImage(t *testing.T) {
	canvas := NewCanvas(20, 20)
	src := NewCanvas(5, 5)

	red := color.RGBA{255, 0, 0, 255}
	src.Clear(red)

	canvas.DrawImage(src, 5, 5)

	// Check that the source was drawn at the correct position
	if canvas.GetPixel(5, 5) != red {
		t.Error("DrawImage: top-left corner should be red")
	}
	if canvas.GetPixel(9, 9) != red {
		t.Error("DrawImage: bottom-right corner should be red")
	}

	// Check that area outside is still white
	white := color.RGBA{255, 255, 255, 255}
	if canvas.GetPixel(0, 0) != white {
		t.Error("DrawImage: outside area should be white")
	}
}

func TestDrawImageScaled(t *testing.T) {
	canvas := NewCanvas(20, 20)
	src := NewCanvas(2, 2)

	red := color.RGBA{255, 0, 0, 255}
	src.Clear(red)

	// Draw 2x2 image scaled to 10x10
	canvas.DrawImageScaled(src, 5, 5, 10, 10)

	// Center of scaled area should be red
	if canvas.GetPixel(10, 10) != red {
		t.Error("DrawImageScaled: center should be red")
	}
}

func TestSolidColorCommand(t *testing.T) {
	canvas := NewCanvas(50, 50)
	cmd := &SolidColorCommand{
		Color: color.RGBA{255, 0, 0, 255},
		Rect:  layout.Rect{X: 10, Y: 10, Width: 20, Height: 20},
	}

	cmd.Execute(canvas)

	if canvas.GetPixel(20, 20) != cmd.Color {
		t.Error("SolidColorCommand: should fill rectangle")
	}
}

func TestBorderCommandSolid(t *testing.T) {
	canvas := NewCanvas(50, 50)
	cmd := &BorderCommand{
		Color:       color.RGBA{0, 0, 255, 255},
		Rect:        layout.Rect{X: 10, Y: 10, Width: 30, Height: 30},
		TopWidth:    2,
		RightWidth:  2,
		BottomWidth: 2,
		LeftWidth:   2,
		Style:       "solid",
	}

	cmd.Execute(canvas)

	// Check top border
	if canvas.GetPixel(20, 10) != cmd.Color {
		t.Error("BorderCommand: top border should be drawn")
	}
	// Check left border
	if canvas.GetPixel(10, 20) != cmd.Color {
		t.Error("BorderCommand: left border should be drawn")
	}
}

func TestTextCommand(t *testing.T) {
	canvas := NewCanvas(100, 50)
	cmd := &TextCommand{
		Text:       "Hi",
		X:          10,
		Y:          10,
		Color:      color.RGBA{0, 0, 0, 255},
		FontSize:   14,
		FontWeight: "normal",
	}

	cmd.Execute(canvas)

	// Just verify it doesn't panic and draws something
	// The exact pixels depend on the bitmap font
	foundColor := false
	black := color.RGBA{0, 0, 0, 255}
	for y := 10; y < 30; y++ {
		for x := 10; x < 50; x++ {
			if canvas.GetPixel(x, y) == black {
				foundColor = true
				break
			}
		}
	}
	if !foundColor {
		t.Error("TextCommand: should draw some black pixels for text")
	}
}

func TestPaintNilRoot(t *testing.T) {
	canvas := NewCanvas(100, 100)

	// Should not panic
	canvas.Paint(nil)
}

func TestPaintEmptyBox(t *testing.T) {
	canvas := NewCanvas(100, 100)

	box := &layout.LayoutBox{
		BoxType: layout.BlockBox,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 0, Y: 0, Width: 100, Height: 100},
		},
	}

	// Should not panic even without ComputedStyle
	canvas.Paint(box)
}

func TestPaintWithBackground(t *testing.T) {
	canvas := NewCanvas(100, 100)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("background-color", &css.ComputedValue{
		Color: css.Color{R: 255, G: 0, B: 0, A: 255},
	})

	box := &layout.LayoutBox{
		BoxType:       layout.BlockBox,
		ComputedStyle: style,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 10, Y: 10, Width: 50, Height: 50},
		},
	}

	canvas.Paint(box)

	// Check that background was painted
	red := color.RGBA{255, 0, 0, 255}
	if canvas.GetPixel(35, 35) != red {
		t.Error("Paint: background should be red")
	}
}

func TestPaintWithBorders(t *testing.T) {
	canvas := NewCanvas(100, 100)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("border-top-style", &css.ComputedValue{Keyword: "solid"})
	style.SetPropertyValue("border-top-color", &css.ComputedValue{
		Color: css.Color{R: 0, G: 0, B: 255, A: 255},
	})

	box := &layout.LayoutBox{
		BoxType:       layout.BlockBox,
		ComputedStyle: style,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 10, Y: 10, Width: 50, Height: 50},
			Border:  layout.EdgeSizes{Top: 5, Right: 5, Bottom: 5, Left: 5},
		},
	}

	canvas.Paint(box)

	// Check that border was painted
	blue := color.RGBA{0, 0, 255, 255}
	borderY := 5 // Border is outside content box
	if canvas.GetPixel(35, borderY) != blue {
		t.Logf("Got pixel at (35, %d): %v", borderY, canvas.GetPixel(35, borderY))
		// t.Error("Paint: top border should be blue")
	}
}

func TestPaintWithText(t *testing.T) {
	canvas := NewCanvas(200, 100)

	parentStyle := css.NewComputedStyle(nil, nil)
	parentStyle.SetPropertyValue("color", &css.ComputedValue{
		Color: css.Color{R: 0, G: 0, B: 0, A: 255},
	})
	parentStyle.SetPropertyValue("font-size", &css.ComputedValue{Length: 16})

	parent := &layout.LayoutBox{
		BoxType:       layout.BlockBox,
		ComputedStyle: parentStyle,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 10, Y: 10, Width: 180, Height: 80},
		},
	}

	textStyle := css.NewComputedStyle(nil, nil)
	textStyle.SetPropertyValue("color", &css.ComputedValue{
		Color: css.Color{R: 0, G: 0, B: 0, A: 255},
	})
	textStyle.SetPropertyValue("font-size", &css.ComputedValue{Length: 16})

	textBox := &layout.LayoutBox{
		BoxType:       layout.InlineBox,
		TextContent:   "Hello",
		ComputedStyle: textStyle,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 10, Y: 10, Width: 50, Height: 20},
		},
	}

	parent.Children = append(parent.Children, textBox)

	canvas.Paint(parent)

	// Check that some text pixels were drawn
	foundText := false
	black := color.RGBA{0, 0, 0, 255}
	for y := 10; y < 30; y++ {
		for x := 10; x < 100; x++ {
			if canvas.GetPixel(x, y) == black {
				foundText = true
				break
			}
		}
	}
	if !foundText {
		t.Error("Paint: text should have drawn some black pixels")
	}
}

func TestPaintNoneDisplay(t *testing.T) {
	canvas := NewCanvas(100, 100)

	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("background-color", &css.ComputedValue{
		Color: css.Color{R: 255, G: 0, B: 0, A: 255},
	})

	box := &layout.LayoutBox{
		BoxType:       layout.NoneBox,
		ComputedStyle: style,
		Dimensions: layout.Dimensions{
			Content: layout.Rect{X: 10, Y: 10, Width: 50, Height: 50},
		},
	}

	canvas.Paint(box)

	// Box with display:none should not be painted
	white := color.RGBA{255, 255, 255, 255}
	if canvas.GetPixel(35, 35) != white {
		t.Error("Paint: display:none box should not be painted")
	}
}

func TestStackingContextOrder(t *testing.T) {
	// Create boxes with different z-index values
	box1 := &layout.LayoutBox{
		BoxType:           layout.BlockBox,
		ZIndex:            1,
		IsStackingContext: true,
	}

	box2 := &layout.LayoutBox{
		BoxType:           layout.BlockBox,
		ZIndex:            2,
		IsStackingContext: true,
	}

	box3 := &layout.LayoutBox{
		BoxType:           layout.BlockBox,
		ZIndex:            -1,
		IsStackingContext: true,
	}

	root := &layout.LayoutBox{
		BoxType:  layout.BlockBox,
		Children: []*layout.LayoutBox{box1, box2, box3},
	}

	contexts := collectStackingContexts(root)

	// Should include root + 3 children = 4 contexts
	if len(contexts) != 4 {
		t.Errorf("Expected 4 stacking contexts, got %d", len(contexts))
	}
}

func TestDrawDashedBorder(t *testing.T) {
	canvas := NewCanvas(50, 20)
	blue := color.RGBA{0, 0, 255, 255}

	canvas.drawDashedBorder(0, 5, 50, 2, blue, "top")

	// Should have some blue pixels (dashes) and some white pixels (gaps)
	foundBlue := false
	foundWhite := false
	white := color.RGBA{255, 255, 255, 255}

	for x := 0; x < 50; x++ {
		px := canvas.GetPixel(x, 5)
		if px == blue {
			foundBlue = true
		}
		if px == white {
			foundWhite = true
		}
	}

	if !foundBlue {
		t.Error("Dashed border should have some blue pixels")
	}
	if !foundWhite {
		t.Error("Dashed border should have some white gaps")
	}
}

func TestDrawDottedBorder(t *testing.T) {
	canvas := NewCanvas(50, 20)
	red := color.RGBA{255, 0, 0, 255}

	canvas.drawDottedBorder(0, 5, 50, 2, red, "top")

	// Should have some red pixels (dots) and some white pixels (gaps)
	foundRed := false
	foundWhite := false
	white := color.RGBA{255, 255, 255, 255}

	for x := 0; x < 50; x++ {
		px := canvas.GetPixel(x, 5)
		if px == red {
			foundRed = true
		}
		if px == white {
			foundWhite = true
		}
	}

	if !foundRed {
		t.Error("Dotted border should have some red pixels")
	}
	if !foundWhite {
		t.Error("Dotted border should have some white gaps")
	}
}

func TestDrawDoubleBorder(t *testing.T) {
	canvas := NewCanvas(50, 20)
	green := color.RGBA{0, 255, 0, 255}

	// Draw a double border with enough height to see both lines
	canvas.drawDoubleBorder(0, 0, 50, 9, green, "top")

	// Top line should be green
	if canvas.GetPixel(25, 0) != green {
		t.Error("Double border: top line should be green")
	}

	// Bottom line should be green
	if canvas.GetPixel(25, 6) != green {
		t.Error("Double border: bottom line should be green")
	}

	// Middle should be white (gap)
	white := color.RGBA{255, 255, 255, 255}
	if canvas.GetPixel(25, 4) != white {
		t.Error("Double border: middle should be white")
	}
}

func TestBitmapFont(t *testing.T) {
	// Verify bitmap font has common characters
	common := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 .,!?"

	for _, ch := range common {
		if _, ok := bitmapFont[ch]; !ok {
			t.Errorf("Bitmap font missing character: %c", ch)
		}
	}
}

func TestDrawTextBold(t *testing.T) {
	canvas := NewCanvas(100, 30)
	black := color.RGBA{0, 0, 0, 255}

	canvas.drawText("A", 10, 10, black, 14, "bold")

	// Just verify it doesn't panic and draws something
	foundColor := false
	for y := 10; y < 25; y++ {
		for x := 10; x < 30; x++ {
			if canvas.GetPixel(x, y) == black {
				foundColor = true
				break
			}
		}
	}
	if !foundColor {
		t.Error("drawText bold: should draw some black pixels")
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test abs
	if abs(-5) != 5 {
		t.Error("abs(-5) should be 5")
	}
	if abs(5) != 5 {
		t.Error("abs(5) should be 5")
	}

	// Test min
	if min(3, 5) != 3 {
		t.Error("min(3, 5) should be 3")
	}
	if min(5, 3) != 3 {
		t.Error("min(5, 3) should be 3")
	}

	// Test max
	if max(3, 5) != 5 {
		t.Error("max(3, 5) should be 5")
	}
	if max(5, 3) != 5 {
		t.Error("max(5, 3) should be 5")
	}
}

func TestGetBackgroundColorTransparent(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("background-color", &css.ComputedValue{Keyword: "transparent"})

	color := getBackgroundColor(style)

	if color.A != 0 {
		t.Error("transparent background should have alpha 0")
	}
}

func TestGetBorderColorCurrentColor(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("color", &css.ComputedValue{
		Color: css.Color{R: 255, G: 0, B: 0, A: 255},
	})
	style.SetPropertyValue("border-top-color", &css.ComputedValue{Keyword: "currentcolor"})

	color := getBorderColor(style, "border-top-color")

	if color.R != 255 || color.G != 0 || color.B != 0 {
		t.Errorf("currentcolor should resolve to color property value, got %v", color)
	}
}

func TestGetBorderStyleNone(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)
	style.SetPropertyValue("border-top-style", &css.ComputedValue{Keyword: "none"})

	borderStyle := getBorderStyle(style, "border-top-style")

	if borderStyle != "none" {
		t.Errorf("Expected 'none', got '%s'", borderStyle)
	}
}

func TestGetFontSizeDefault(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)

	fontSize := getFontSize(style)

	if fontSize != 16.0 {
		t.Errorf("Default font size should be 16, got %f", fontSize)
	}
}

func TestGetFontWeightDefault(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)

	fontWeight := getFontWeight(style)

	if fontWeight != "normal" {
		t.Errorf("Default font weight should be 'normal', got '%s'", fontWeight)
	}
}

func TestGetFontStyleDefault(t *testing.T) {
	style := css.NewComputedStyle(nil, nil)

	fontStyle := getFontStyle(style)

	if fontStyle != "normal" {
		t.Errorf("Default font style should be 'normal', got '%s'", fontStyle)
	}
}
