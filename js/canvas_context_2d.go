package js

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/chrisuehlinger/viberowser/dom"
	"github.com/chrisuehlinger/viberowser/render"
	"github.com/dop251/goja"
)

// CanvasRenderingContext2D implements the CanvasRenderingContext2D API.
// Reference: https://html.spec.whatwg.org/multipage/canvas.html#canvasrenderingcontext2d
type CanvasRenderingContext2D struct {
	canvas      *render.Canvas
	canvasEl    *dom.Element
	width       int
	height      int
	fillStyle   color.RGBA
	strokeStyle color.RGBA
	lineWidth   float64
	lineCap     string
	lineJoin    string
	miterLimit  float64
	globalAlpha float64
	font        string
	textAlign   string
	textBaseline string

	// Current transformation matrix (a, b, c, d, e, f)
	// [ a c e ]
	// [ b d f ]
	// [ 0 0 1 ]
	transformA, transformB, transformC, transformD, transformE, transformF float64

	// Path state
	path        []pathCommand
	pathStartX  float64
	pathStartY  float64
	currentX    float64
	currentY    float64

	// State stack for save/restore
	stateStack []*canvasState
}

// pathCommand represents a single path drawing command.
type pathCommand interface {
	draw(ctx *CanvasRenderingContext2D, canvas *render.Canvas, fill bool)
}

// canvasState holds the state that can be saved/restored.
type canvasState struct {
	fillStyle    color.RGBA
	strokeStyle  color.RGBA
	lineWidth    float64
	lineCap      string
	lineJoin     string
	miterLimit   float64
	globalAlpha  float64
	font         string
	textAlign    string
	textBaseline string
	transformA   float64
	transformB   float64
	transformC   float64
	transformD   float64
	transformE   float64
	transformF   float64
}

// moveToCommand is a moveTo path command.
type moveToCommand struct {
	x, y float64
}

func (c *moveToCommand) draw(ctx *CanvasRenderingContext2D, canvas *render.Canvas, fill bool) {
	// MoveTo doesn't draw, it just sets the starting point
}

// lineToCommand is a lineTo path command.
type lineToCommand struct {
	fromX, fromY float64
	toX, toY     float64
}

func (c *lineToCommand) draw(ctx *CanvasRenderingContext2D, canvas *render.Canvas, fill bool) {
	if !fill {
		// Only stroke draws lines
		col := ctx.applyAlpha(ctx.strokeStyle)
		canvas.DrawLine(int(c.fromX), int(c.fromY), int(c.toX), int(c.toY), col)
	}
}

// arcCommand is an arc path command.
type arcCommand struct {
	x, y, radius  float64
	startAngle    float64
	endAngle      float64
	counterClockwise bool
}

func (c *arcCommand) draw(ctx *CanvasRenderingContext2D, canvas *render.Canvas, fill bool) {
	col := ctx.strokeStyle
	if fill {
		col = ctx.fillStyle
	}
	col = ctx.applyAlpha(col)

	// Draw arc as a series of line segments
	steps := 64 // Number of segments for the arc
	startAngle := c.startAngle
	endAngle := c.endAngle

	if c.counterClockwise {
		if endAngle >= startAngle {
			endAngle -= 2 * math.Pi
		}
	} else {
		if endAngle <= startAngle {
			endAngle += 2 * math.Pi
		}
	}

	angleRange := endAngle - startAngle
	for i := 0; i < steps; i++ {
		t1 := float64(i) / float64(steps)
		t2 := float64(i+1) / float64(steps)

		angle1 := startAngle + t1*angleRange
		angle2 := startAngle + t2*angleRange

		x1 := c.x + c.radius*math.Cos(angle1)
		y1 := c.y + c.radius*math.Sin(angle1)
		x2 := c.x + c.radius*math.Cos(angle2)
		y2 := c.y + c.radius*math.Sin(angle2)

		canvas.DrawLine(int(x1), int(y1), int(x2), int(y2), col)
	}
}

// rectCommand is a rect path command.
type rectCommand struct {
	x, y, width, height float64
}

func (c *rectCommand) draw(ctx *CanvasRenderingContext2D, canvas *render.Canvas, fill bool) {
	if fill {
		col := ctx.applyAlpha(ctx.fillStyle)
		canvas.FillRect(int(c.x), int(c.y), int(c.width), int(c.height), col)
	} else {
		col := ctx.applyAlpha(ctx.strokeStyle)
		// Draw rectangle outline
		canvas.DrawLine(int(c.x), int(c.y), int(c.x+c.width), int(c.y), col)
		canvas.DrawLine(int(c.x+c.width), int(c.y), int(c.x+c.width), int(c.y+c.height), col)
		canvas.DrawLine(int(c.x+c.width), int(c.y+c.height), int(c.x), int(c.y+c.height), col)
		canvas.DrawLine(int(c.x), int(c.y+c.height), int(c.x), int(c.y), col)
	}
}

// NewCanvasRenderingContext2D creates a new 2D rendering context for the given canvas element.
func NewCanvasRenderingContext2D(canvasEl *dom.Element, width, height int) *CanvasRenderingContext2D {
	ctx := &CanvasRenderingContext2D{
		canvas:       render.NewCanvas(width, height),
		canvasEl:     canvasEl,
		width:        width,
		height:       height,
		fillStyle:    color.RGBA{0, 0, 0, 255},   // Default black
		strokeStyle:  color.RGBA{0, 0, 0, 255},   // Default black
		lineWidth:    1.0,
		lineCap:      "butt",
		lineJoin:     "miter",
		miterLimit:   10.0,
		globalAlpha:  1.0,
		font:         "10px sans-serif",
		textAlign:    "start",
		textBaseline: "alphabetic",
		transformA:   1, transformB: 0, transformC: 0,
		transformD:   1, transformE: 0, transformF: 0,
		path:        make([]pathCommand, 0),
		stateStack:  make([]*canvasState, 0),
	}
	// Clear to transparent
	ctx.canvas.Clear(color.RGBA{0, 0, 0, 0})
	return ctx
}

// GetCanvas returns the underlying render canvas.
func (ctx *CanvasRenderingContext2D) GetCanvas() *render.Canvas {
	return ctx.canvas
}

// Resize recreates the canvas with new dimensions.
func (ctx *CanvasRenderingContext2D) Resize(width, height int) {
	ctx.width = width
	ctx.height = height
	ctx.canvas = render.NewCanvas(width, height)
	ctx.canvas.Clear(color.RGBA{0, 0, 0, 0})
}

// applyAlpha multiplies the color's alpha with globalAlpha.
func (ctx *CanvasRenderingContext2D) applyAlpha(col color.RGBA) color.RGBA {
	if ctx.globalAlpha >= 1.0 {
		return col
	}
	newAlpha := float64(col.A) * ctx.globalAlpha
	return color.RGBA{col.R, col.G, col.B, uint8(newAlpha)}
}

// Save pushes the current state onto the state stack.
func (ctx *CanvasRenderingContext2D) Save() {
	state := &canvasState{
		fillStyle:    ctx.fillStyle,
		strokeStyle:  ctx.strokeStyle,
		lineWidth:    ctx.lineWidth,
		lineCap:      ctx.lineCap,
		lineJoin:     ctx.lineJoin,
		miterLimit:   ctx.miterLimit,
		globalAlpha:  ctx.globalAlpha,
		font:         ctx.font,
		textAlign:    ctx.textAlign,
		textBaseline: ctx.textBaseline,
		transformA:   ctx.transformA,
		transformB:   ctx.transformB,
		transformC:   ctx.transformC,
		transformD:   ctx.transformD,
		transformE:   ctx.transformE,
		transformF:   ctx.transformF,
	}
	ctx.stateStack = append(ctx.stateStack, state)
}

// Restore pops the state stack and restores the saved state.
func (ctx *CanvasRenderingContext2D) Restore() {
	if len(ctx.stateStack) == 0 {
		return
	}
	state := ctx.stateStack[len(ctx.stateStack)-1]
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]

	ctx.fillStyle = state.fillStyle
	ctx.strokeStyle = state.strokeStyle
	ctx.lineWidth = state.lineWidth
	ctx.lineCap = state.lineCap
	ctx.lineJoin = state.lineJoin
	ctx.miterLimit = state.miterLimit
	ctx.globalAlpha = state.globalAlpha
	ctx.font = state.font
	ctx.textAlign = state.textAlign
	ctx.textBaseline = state.textBaseline
	ctx.transformA = state.transformA
	ctx.transformB = state.transformB
	ctx.transformC = state.transformC
	ctx.transformD = state.transformD
	ctx.transformE = state.transformE
	ctx.transformF = state.transformF
}

// FillRect fills a rectangle.
func (ctx *CanvasRenderingContext2D) FillRect(x, y, width, height float64) {
	col := ctx.applyAlpha(ctx.fillStyle)
	ctx.canvas.FillRect(int(x), int(y), int(width), int(height), col)
}

// StrokeRect draws a rectangle outline.
func (ctx *CanvasRenderingContext2D) StrokeRect(x, y, width, height float64) {
	col := ctx.applyAlpha(ctx.strokeStyle)
	// Draw rectangle outline using lines
	ctx.canvas.DrawLine(int(x), int(y), int(x+width), int(y), col)
	ctx.canvas.DrawLine(int(x+width), int(y), int(x+width), int(y+height), col)
	ctx.canvas.DrawLine(int(x+width), int(y+height), int(x), int(y+height), col)
	ctx.canvas.DrawLine(int(x), int(y+height), int(x), int(y), col)
}

// ClearRect clears a rectangle to transparent.
func (ctx *CanvasRenderingContext2D) ClearRect(x, y, width, height float64) {
	transparent := color.RGBA{0, 0, 0, 0}
	for py := int(y); py < int(y+height) && py < ctx.height; py++ {
		for px := int(x); px < int(x+width) && px < ctx.width; px++ {
			if px >= 0 && py >= 0 {
				ctx.canvas.SetPixel(px, py, transparent)
			}
		}
	}
}

// BeginPath starts a new path.
func (ctx *CanvasRenderingContext2D) BeginPath() {
	ctx.path = make([]pathCommand, 0)
}

// MoveTo moves the current point.
func (ctx *CanvasRenderingContext2D) MoveTo(x, y float64) {
	ctx.currentX = x
	ctx.currentY = y
	ctx.pathStartX = x
	ctx.pathStartY = y
	ctx.path = append(ctx.path, &moveToCommand{x, y})
}

// LineTo adds a line to the path.
func (ctx *CanvasRenderingContext2D) LineTo(x, y float64) {
	ctx.path = append(ctx.path, &lineToCommand{ctx.currentX, ctx.currentY, x, y})
	ctx.currentX = x
	ctx.currentY = y
}

// ClosePath closes the current path.
func (ctx *CanvasRenderingContext2D) ClosePath() {
	if ctx.currentX != ctx.pathStartX || ctx.currentY != ctx.pathStartY {
		ctx.path = append(ctx.path, &lineToCommand{ctx.currentX, ctx.currentY, ctx.pathStartX, ctx.pathStartY})
	}
	ctx.currentX = ctx.pathStartX
	ctx.currentY = ctx.pathStartY
}

// Arc adds an arc to the path.
func (ctx *CanvasRenderingContext2D) Arc(x, y, radius, startAngle, endAngle float64, counterClockwise bool) {
	// Calculate starting point
	startX := x + radius*math.Cos(startAngle)
	startY := y + radius*math.Sin(startAngle)

	// If there's an existing path, draw line to arc start
	if len(ctx.path) > 0 {
		ctx.path = append(ctx.path, &lineToCommand{ctx.currentX, ctx.currentY, startX, startY})
	}

	ctx.path = append(ctx.path, &arcCommand{x, y, radius, startAngle, endAngle, counterClockwise})

	// Update current point to end of arc
	endX := x + radius*math.Cos(endAngle)
	endY := y + radius*math.Sin(endAngle)
	ctx.currentX = endX
	ctx.currentY = endY

	// Set path start if this is the first command
	if len(ctx.path) == 1 {
		ctx.pathStartX = startX
		ctx.pathStartY = startY
	}
}

// Rect adds a rectangle to the path.
func (ctx *CanvasRenderingContext2D) Rect(x, y, width, height float64) {
	ctx.path = append(ctx.path, &rectCommand{x, y, width, height})
	ctx.currentX = x
	ctx.currentY = y
}

// Fill fills the current path.
func (ctx *CanvasRenderingContext2D) Fill() {
	for _, cmd := range ctx.path {
		cmd.draw(ctx, ctx.canvas, true)
	}
}

// Stroke strokes the current path.
func (ctx *CanvasRenderingContext2D) Stroke() {
	for _, cmd := range ctx.path {
		cmd.draw(ctx, ctx.canvas, false)
	}
}

// FillText draws filled text.
func (ctx *CanvasRenderingContext2D) FillText(text string, x, y float64) {
	// Parse font size from font string (e.g., "16px Arial")
	fontSize := 16.0
	parts := strings.Fields(ctx.font)
	for _, part := range parts {
		if strings.HasSuffix(part, "px") {
			if sz, err := strconv.ParseFloat(strings.TrimSuffix(part, "px"), 64); err == nil {
				fontSize = sz
				break
			}
		}
	}

	col := ctx.applyAlpha(ctx.fillStyle)

	// Use basic text rendering - this is a simplified implementation
	// Real implementation would need proper font metrics
	scale := fontSize / 7.0 // Base bitmap font is 7 pixels tall
	charWidth := int(5 * scale)
	spacing := int(1 * scale)

	currentX := int(x)
	for _, ch := range text {
		bitmap, ok := render.GetBitmapFont(ch)
		if !ok {
			bitmap, _ = render.GetBitmapFont('?')
		}
		drawBitmapChar(ctx.canvas, currentX, int(y)-int(fontSize), bitmap, scale, col)
		currentX += charWidth + spacing
	}
}

// StrokeText draws stroked text.
func (ctx *CanvasRenderingContext2D) StrokeText(text string, x, y float64) {
	// For simplicity, just use FillText with stroke color
	originalFill := ctx.fillStyle
	ctx.fillStyle = ctx.strokeStyle
	ctx.FillText(text, x, y)
	ctx.fillStyle = originalFill
}

// MeasureText returns the metrics of the given text.
func (ctx *CanvasRenderingContext2D) MeasureText(text string) float64 {
	fontSize := 16.0
	parts := strings.Fields(ctx.font)
	for _, part := range parts {
		if strings.HasSuffix(part, "px") {
			if sz, err := strconv.ParseFloat(strings.TrimSuffix(part, "px"), 64); err == nil {
				fontSize = sz
				break
			}
		}
	}

	scale := fontSize / 7.0
	charWidth := 5 * scale
	spacing := 1 * scale

	// Calculate total width
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	return float64(len(runes))*charWidth + float64(len(runes)-1)*spacing
}

// Translate translates the canvas origin.
func (ctx *CanvasRenderingContext2D) Translate(x, y float64) {
	ctx.transformE += ctx.transformA*x + ctx.transformC*y
	ctx.transformF += ctx.transformB*x + ctx.transformD*y
}

// Rotate rotates the canvas.
func (ctx *CanvasRenderingContext2D) Rotate(angle float64) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)

	a := ctx.transformA*cos + ctx.transformC*sin
	b := ctx.transformB*cos + ctx.transformD*sin
	c := ctx.transformA*-sin + ctx.transformC*cos
	d := ctx.transformB*-sin + ctx.transformD*cos

	ctx.transformA = a
	ctx.transformB = b
	ctx.transformC = c
	ctx.transformD = d
}

// Scale scales the canvas.
func (ctx *CanvasRenderingContext2D) Scale(x, y float64) {
	ctx.transformA *= x
	ctx.transformB *= x
	ctx.transformC *= y
	ctx.transformD *= y
}

// SetTransform resets and sets the transformation matrix.
func (ctx *CanvasRenderingContext2D) SetTransform(a, b, c, d, e, f float64) {
	ctx.transformA = a
	ctx.transformB = b
	ctx.transformC = c
	ctx.transformD = d
	ctx.transformE = e
	ctx.transformF = f
}

// ResetTransform resets to the identity matrix.
func (ctx *CanvasRenderingContext2D) ResetTransform() {
	ctx.transformA = 1
	ctx.transformB = 0
	ctx.transformC = 0
	ctx.transformD = 1
	ctx.transformE = 0
	ctx.transformF = 0
}

// Transform multiplies the current transformation matrix.
func (ctx *CanvasRenderingContext2D) Transform(a, b, c, d, e, f float64) {
	newA := ctx.transformA*a + ctx.transformC*b
	newB := ctx.transformB*a + ctx.transformD*b
	newC := ctx.transformA*c + ctx.transformC*d
	newD := ctx.transformB*c + ctx.transformD*d
	newE := ctx.transformA*e + ctx.transformC*f + ctx.transformE
	newF := ctx.transformB*e + ctx.transformD*f + ctx.transformF

	ctx.transformA = newA
	ctx.transformB = newB
	ctx.transformC = newC
	ctx.transformD = newD
	ctx.transformE = newE
	ctx.transformF = newF
}

// parseColor parses a CSS color string into RGBA.
func parseColor(colorStr string) (color.RGBA, bool) {
	colorStr = strings.TrimSpace(colorStr)
	colorStr = strings.ToLower(colorStr)

	// Named colors
	namedColors := map[string]color.RGBA{
		"black":       {0, 0, 0, 255},
		"white":       {255, 255, 255, 255},
		"red":         {255, 0, 0, 255},
		"green":       {0, 128, 0, 255},
		"blue":        {0, 0, 255, 255},
		"yellow":      {255, 255, 0, 255},
		"cyan":        {0, 255, 255, 255},
		"magenta":     {255, 0, 255, 255},
		"gray":        {128, 128, 128, 255},
		"grey":        {128, 128, 128, 255},
		"orange":      {255, 165, 0, 255},
		"purple":      {128, 0, 128, 255},
		"pink":        {255, 192, 203, 255},
		"transparent": {0, 0, 0, 0},
	}

	if col, ok := namedColors[colorStr]; ok {
		return col, true
	}

	// Hex color: #RGB, #RGBA, #RRGGBB, #RRGGBBAA
	if strings.HasPrefix(colorStr, "#") {
		hex := colorStr[1:]
		switch len(hex) {
		case 3: // #RGB
			r, _ := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
			g, _ := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
			b, _ := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
			return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, true
		case 4: // #RGBA
			r, _ := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
			g, _ := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
			b, _ := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
			a, _ := strconv.ParseUint(string(hex[3])+string(hex[3]), 16, 8)
			return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, true
		case 6: // #RRGGBB
			r, _ := strconv.ParseUint(hex[0:2], 16, 8)
			g, _ := strconv.ParseUint(hex[2:4], 16, 8)
			b, _ := strconv.ParseUint(hex[4:6], 16, 8)
			return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, true
		case 8: // #RRGGBBAA
			r, _ := strconv.ParseUint(hex[0:2], 16, 8)
			g, _ := strconv.ParseUint(hex[2:4], 16, 8)
			b, _ := strconv.ParseUint(hex[4:6], 16, 8)
			a, _ := strconv.ParseUint(hex[6:8], 16, 8)
			return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, true
		}
	}

	// rgb() and rgba()
	if strings.HasPrefix(colorStr, "rgb(") || strings.HasPrefix(colorStr, "rgba(") {
		colorStr = strings.TrimPrefix(colorStr, "rgba(")
		colorStr = strings.TrimPrefix(colorStr, "rgb(")
		colorStr = strings.TrimSuffix(colorStr, ")")
		parts := strings.Split(colorStr, ",")
		if len(parts) >= 3 {
			r, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			g, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			b, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			a := 255
			if len(parts) >= 4 {
				alphaStr := strings.TrimSpace(parts[3])
				if af, err := strconv.ParseFloat(alphaStr, 64); err == nil {
					a = int(af * 255)
				}
			}
			return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, true
		}
	}

	return color.RGBA{0, 0, 0, 255}, false
}

// colorToString converts RGBA to CSS color string.
func colorToString(col color.RGBA) string {
	if col.A == 255 {
		return "#" + hexByte(col.R) + hexByte(col.G) + hexByte(col.B)
	}
	return "rgba(" + strconv.Itoa(int(col.R)) + ", " + strconv.Itoa(int(col.G)) + ", " + strconv.Itoa(int(col.B)) + ", " + strconv.FormatFloat(float64(col.A)/255, 'f', -1, 64) + ")"
}

func hexByte(b byte) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0xf]})
}

// drawBitmapChar draws a single character using the bitmap font.
func drawBitmapChar(canvas *render.Canvas, x, y int, bitmap []uint8, scale float64, col color.RGBA) {
	for row := 0; row < 7; row++ {
		rowBits := bitmap[row]
		for colIdx := 0; colIdx < 5; colIdx++ {
			if rowBits&(0x10>>colIdx) != 0 {
				px := x + int(float64(colIdx)*scale)
				py := y + int(float64(row)*scale)
				pixelSize := int(scale)
				if pixelSize < 1 {
					pixelSize = 1
				}
				for dy := 0; dy < pixelSize; dy++ {
					for dx := 0; dx < pixelSize; dx++ {
						canvas.SetPixelBlend(px+dx, py+dy, col)
					}
				}
			}
		}
	}
}

// BindCanvasContext2D binds a CanvasRenderingContext2D to a JavaScript object.
func (b *DOMBinder) BindCanvasContext2D(ctx *CanvasRenderingContext2D) *goja.Object {
	vm := b.runtime.vm

	obj := vm.NewObject()

	// Set the canvas property (reference to the canvas element)
	obj.DefineAccessorProperty("canvas", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return b.BindElement(ctx.canvasEl)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// fillStyle
	obj.DefineAccessorProperty("fillStyle", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(colorToString(ctx.fillStyle))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if col, ok := parseColor(call.Arguments[0].String()); ok {
				ctx.fillStyle = col
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// strokeStyle
	obj.DefineAccessorProperty("strokeStyle", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(colorToString(ctx.strokeStyle))
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			if col, ok := parseColor(call.Arguments[0].String()); ok {
				ctx.strokeStyle = col
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// lineWidth
	obj.DefineAccessorProperty("lineWidth", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.lineWidth)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			ctx.lineWidth = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// lineCap
	obj.DefineAccessorProperty("lineCap", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.lineCap)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].String()
			if val == "butt" || val == "round" || val == "square" {
				ctx.lineCap = val
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// lineJoin
	obj.DefineAccessorProperty("lineJoin", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.lineJoin)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].String()
			if val == "round" || val == "bevel" || val == "miter" {
				ctx.lineJoin = val
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// miterLimit
	obj.DefineAccessorProperty("miterLimit", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.miterLimit)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			ctx.miterLimit = call.Arguments[0].ToFloat()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// globalAlpha
	obj.DefineAccessorProperty("globalAlpha", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.globalAlpha)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			alpha := call.Arguments[0].ToFloat()
			if alpha >= 0 && alpha <= 1 {
				ctx.globalAlpha = alpha
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// font
	obj.DefineAccessorProperty("font", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.font)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			ctx.font = call.Arguments[0].String()
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// textAlign
	obj.DefineAccessorProperty("textAlign", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.textAlign)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].String()
			if val == "start" || val == "end" || val == "left" || val == "right" || val == "center" {
				ctx.textAlign = val
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// textBaseline
	obj.DefineAccessorProperty("textBaseline", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(ctx.textBaseline)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			val := call.Arguments[0].String()
			if val == "top" || val == "hanging" || val == "middle" || val == "alphabetic" || val == "ideographic" || val == "bottom" {
				ctx.textBaseline = val
			}
		}
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// Methods

	// save()
	obj.Set("save", func(call goja.FunctionCall) goja.Value {
		ctx.Save()
		return goja.Undefined()
	})

	// restore()
	obj.Set("restore", func(call goja.FunctionCall) goja.Value {
		ctx.Restore()
		return goja.Undefined()
	})

	// fillRect(x, y, width, height)
	obj.Set("fillRect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 4 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			w := call.Arguments[2].ToFloat()
			h := call.Arguments[3].ToFloat()
			ctx.FillRect(x, y, w, h)
		}
		return goja.Undefined()
	})

	// strokeRect(x, y, width, height)
	obj.Set("strokeRect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 4 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			w := call.Arguments[2].ToFloat()
			h := call.Arguments[3].ToFloat()
			ctx.StrokeRect(x, y, w, h)
		}
		return goja.Undefined()
	})

	// clearRect(x, y, width, height)
	obj.Set("clearRect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 4 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			w := call.Arguments[2].ToFloat()
			h := call.Arguments[3].ToFloat()
			ctx.ClearRect(x, y, w, h)
		}
		return goja.Undefined()
	})

	// beginPath()
	obj.Set("beginPath", func(call goja.FunctionCall) goja.Value {
		ctx.BeginPath()
		return goja.Undefined()
	})

	// moveTo(x, y)
	obj.Set("moveTo", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 2 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			ctx.MoveTo(x, y)
		}
		return goja.Undefined()
	})

	// lineTo(x, y)
	obj.Set("lineTo", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 2 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			ctx.LineTo(x, y)
		}
		return goja.Undefined()
	})

	// closePath()
	obj.Set("closePath", func(call goja.FunctionCall) goja.Value {
		ctx.ClosePath()
		return goja.Undefined()
	})

	// arc(x, y, radius, startAngle, endAngle, counterclockwise?)
	obj.Set("arc", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 5 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			radius := call.Arguments[2].ToFloat()
			startAngle := call.Arguments[3].ToFloat()
			endAngle := call.Arguments[4].ToFloat()
			ccw := false
			if len(call.Arguments) >= 6 {
				ccw = call.Arguments[5].ToBoolean()
			}
			ctx.Arc(x, y, radius, startAngle, endAngle, ccw)
		}
		return goja.Undefined()
	})

	// rect(x, y, width, height)
	obj.Set("rect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 4 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			w := call.Arguments[2].ToFloat()
			h := call.Arguments[3].ToFloat()
			ctx.Rect(x, y, w, h)
		}
		return goja.Undefined()
	})

	// fill()
	obj.Set("fill", func(call goja.FunctionCall) goja.Value {
		ctx.Fill()
		return goja.Undefined()
	})

	// stroke()
	obj.Set("stroke", func(call goja.FunctionCall) goja.Value {
		ctx.Stroke()
		return goja.Undefined()
	})

	// fillText(text, x, y, maxWidth?)
	obj.Set("fillText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 3 {
			text := call.Arguments[0].String()
			x := call.Arguments[1].ToFloat()
			y := call.Arguments[2].ToFloat()
			ctx.FillText(text, x, y)
		}
		return goja.Undefined()
	})

	// strokeText(text, x, y, maxWidth?)
	obj.Set("strokeText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 3 {
			text := call.Arguments[0].String()
			x := call.Arguments[1].ToFloat()
			y := call.Arguments[2].ToFloat()
			ctx.StrokeText(text, x, y)
		}
		return goja.Undefined()
	})

	// measureText(text)
	obj.Set("measureText", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 1 {
			text := call.Arguments[0].String()
			width := ctx.MeasureText(text)
			// Return TextMetrics object
			metrics := vm.NewObject()
			metrics.Set("width", width)
			return metrics
		}
		return goja.Undefined()
	})

	// translate(x, y)
	obj.Set("translate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 2 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			ctx.Translate(x, y)
		}
		return goja.Undefined()
	})

	// rotate(angle)
	obj.Set("rotate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 1 {
			angle := call.Arguments[0].ToFloat()
			ctx.Rotate(angle)
		}
		return goja.Undefined()
	})

	// scale(x, y)
	obj.Set("scale", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 2 {
			x := call.Arguments[0].ToFloat()
			y := call.Arguments[1].ToFloat()
			ctx.Scale(x, y)
		}
		return goja.Undefined()
	})

	// setTransform(a, b, c, d, e, f)
	obj.Set("setTransform", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 6 {
			a := call.Arguments[0].ToFloat()
			b := call.Arguments[1].ToFloat()
			c := call.Arguments[2].ToFloat()
			d := call.Arguments[3].ToFloat()
			e := call.Arguments[4].ToFloat()
			f := call.Arguments[5].ToFloat()
			ctx.SetTransform(a, b, c, d, e, f)
		}
		return goja.Undefined()
	})

	// resetTransform()
	obj.Set("resetTransform", func(call goja.FunctionCall) goja.Value {
		ctx.ResetTransform()
		return goja.Undefined()
	})

	// transform(a, b, c, d, e, f)
	obj.Set("transform", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) >= 6 {
			a := call.Arguments[0].ToFloat()
			b := call.Arguments[1].ToFloat()
			c := call.Arguments[2].ToFloat()
			d := call.Arguments[3].ToFloat()
			e := call.Arguments[4].ToFloat()
			f := call.Arguments[5].ToFloat()
			ctx.Transform(a, b, c, d, e, f)
		}
		return goja.Undefined()
	})

	// getTransform()
	obj.Set("getTransform", func(call goja.FunctionCall) goja.Value {
		matrix := vm.NewObject()
		matrix.Set("a", ctx.transformA)
		matrix.Set("b", ctx.transformB)
		matrix.Set("c", ctx.transformC)
		matrix.Set("d", ctx.transformD)
		matrix.Set("e", ctx.transformE)
		matrix.Set("f", ctx.transformF)
		return matrix
	})

	return obj
}
