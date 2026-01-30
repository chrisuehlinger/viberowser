// Package render handles painting/rendering of the layout tree.
package render

import (
	"image"
	"image/color"

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

// Paint renders a layout tree to the canvas.
func (c *Canvas) Paint(layoutRoot *layout.LayoutBox) {
	// TODO: Implement painting
}

// SetPixel sets a single pixel on the canvas.
func (c *Canvas) SetPixel(x, y int, col color.RGBA) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.Pixels[y*c.Width+x] = col
	}
}

// FillRect fills a rectangle with the given color.
func (c *Canvas) FillRect(x, y, width, height int, col color.RGBA) {
	for py := y; py < y+height; py++ {
		for px := x; px < x+width; px++ {
			c.SetPixel(px, py, col)
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
