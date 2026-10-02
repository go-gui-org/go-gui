package main

// The six workloads of Qt's qcpainterbench, drawn with go-gui's DrawContext.
//
// This is a port from behavior, not from code. The constants and formulas come
// from the written description in docs/specs/qcpainterbench.md. The Qt sources are
// GPL-3.0-only, so no Qt code is copied here.
//
// Where go-gui has no direct equivalent of a QCanvasPainter operation, the
// workload emulates it. Each emulation has a comment that starts "Emulated:" so
// it is easy to find, and the README lists them all.

import (
	"math"
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
)

// Test bits. The values match Qt's enabledTests mask, so a mask from a Qt run means
// the same thing here.
const (
	testRuler = 1 << iota
	testCircles
	testLines
	testBars
	testIcons
	testFlower

	testAll = testRuler | testCircles | testLines | testBars | testIcons |
		testFlower
	// testDefault is Qt's default: every test except the flower.
	testDefault = testAll &^ testFlower
)

// passStep is how far Qt moves the animation time between render passes. Every
// pass draws the same geometry; this offset is the only difference.
const passStep = 0.3

// flowerSegs is the number of line segments each quadratic of the flower outline
// is flattened to. 12 quadratics × 16 = 192 segments around the outline. That is
// smooth at the default window size, and the outline is built only once.
const flowerSegs = 16

// bezierSegs is the number of line segments each cubic of a line graph is
// flattened to. The graphs are rebuilt every pass, so this is per-frame work.
const bezierSegs = 16

// maxLabel is one more than the largest ruler or icon number that has a
// prebuilt label. The narrowest ruler spacing is 0.01·w from 0.05·w, so at most 95
// ticks fit on any canvas width.
const maxLabel = 128

// Colors from the Qt benchmark.
var (
	colWhite     = gui.RGBA(255, 255, 255, 255)
	colGray      = gui.RGBA(180, 180, 180, 255)
	colBlack     = gui.RGBA(0, 0, 0, 255)
	colAreaLow   = gui.RGBA(180, 190, 40, 20)   // line graph area, bottom
	colAreaHigh  = gui.RGBA(255, 255, 255, 150) // line graph area, top
	colBar       = gui.RGBA(255, 255, 255, 80)  // bar fill
	colRingBg    = gui.RGBA(215, 215, 215, 50)  // gauge ring background
	colTick      = gui.RGBA(0xE0, 0xE0, 0xE0, 255)
	colTickLabel = gui.RGBA(0xE0, 0xE0, 0xB0, 255)
	colPetalLine = gui.RGBA(0, 0, 0, 0x40)
)

// labels holds the strings "0".."127". Formatting a number per label per frame
// would allocate on the hot path and measure strconv, not the renderer.
var labels = func() [maxLabel]string {
	var l [maxLabel]string
	for i := range l {
		l[i] = strconv.Itoa(i)
	}
	return l
}()

// scene holds the scratch buffers the workloads reuse across frames, so a
// steady-state frame does not allocate.
type scene struct {
	curve []float32 // flattened line graph points
	tris  []float32 // triangle list for gradient fills
	rot   []float32 // flower outline after rotation

	// flower is the unrotated flower outline. Qt builds its path once and
	// rebuilds it only on resize. flowerKey is the box the outline was built
	// for.
	flower    []float32
	flowerKey [3]float32

	stops [2]gui.GradientStop
	grad  gui.CanvasGradient

	// iconSrc names the icon image in go-gui's in-memory image registry.
	iconSrc string
	// style is the base text style. Only Size and Color change per call.
	style gui.TextStyle
}

// paint draws one frame: every enabled test, count times. This is Qt's paint()
// loop. tests is a mask of test bits. t is the animation time in seconds.
func (s *scene) paint(dc *gui.DrawContext, w, h, t float32, tests, count int) {
	if s.iconSrc == "" {
		s.iconSrc = iconImage()
		s.style = gui.CurrentTheme().TextStyleBodySmall
	}
	sz := min(w, h)
	for range count {
		if tests&testRuler != 0 {
			s.ruler(dc, 0, h*0.02, w, h*0.05, t)
		}
		if tests&testCircles != 0 {
			big := 50 + sz*0.5
			small := 20 + sz*0.2
			s.gauge(dc, w/2-big/2, h*0.1, big, 8, t*2)
			s.gauge(dc, w*0.05, h*0.55, small, 6, t*3)
			s.gauge(dc, w-small-w*0.05, h*0.55, small, 3, t)
		}
		if tests&testLines != 0 {
			s.lineGraph(dc, 0, h, w, -h, 4, t)
			s.lineGraph(dc, 0, h, w, -h*0.8, 6, t+10)
			s.lineGraph(dc, 0, h, w, -h*0.6, 12, t/2)
		}
		if tests&testBars != 0 {
			s.barGraph(dc, 0, h, w, -h*0.8, 6, t*3)
			s.barGraph(dc, 0, h, w, -h*0.4, 10, t+2)
			s.barGraph(dc, 0, h, w, -h*0.3, 20, t*2+2)
			s.barGraph(dc, 0, h, w, -h*0.2, 40, t*3+2)
		}
		if tests&testIcons != 0 {
			s.icons(dc, 0, h*0.2, w, h*0.2, sz, 20, t)
		}
		if tests&testFlower != 0 {
			fs := 80 + sz*0.6
			s.flowerDraw(dc, w/2-fs/2, h-fs, fs, t)
		}
		t += passStep
	}
}

// ruler draws evenly spaced vertical ticks with number labels. The spacing
// breathes with sin(t), so ticks and labels appear and disappear.
func (s *scene) ruler(dc *gui.DrawContext, x, y, w, h, t float32) {
	posX := x + w*0.05
	space := w*0.03 + sin(t)*w*0.02
	if space <= 0 {
		return
	}
	style := s.style
	style.Size = 10 + w*0.01
	style.Color = colTickLabel
	for i := 0; posX < w; i++ {
		tick := h * 0.2
		label := false
		switch {
		case i%10 == 0:
			tick, label = h*0.5, true
		case i%5 == 0:
			tick, label = h*0.3, space > w*0.02
		}
		if label && i < maxLabel {
			centerText(dc, posX, y+h, labels[i], style)
		}
		// Qt collects all ticks in one path and strokes it once. go-gui has no
		// path object; a Line per tick lands in the same color batch, so the GPU
		// still sees one draw.
		dc.Line(posX, y, posX, y+tick, colTick, 1)
		posX += space
	}
}

// gauge draws a set of concentric rings: a faint background circle, and over it
// an arc whose length and alpha follow sin(t).
func (s *scene) gauge(dc *gui.DrawContext, x, y, size float32, items int, t float32) {
	barWidth := 0.3 * size / float32(items)
	margin := 0.2 * barWidth
	prog := 0.6 + 0.4*sin(t*0.8)
	lw := barWidth * prog
	cx, cy := x+size/2, y+size/2
	r1 := size/2 - lw

	r := r1
	for range items {
		dc.Circle(cx, cy, r, colRingBg, lw)
		r -= lw + margin
	}

	r = r1
	alpha := uint8(255 * prog)
	for i := range items {
		frac := float32(items-i) / float32(items)
		// Qt draws counter-clockwise from a0 back to -π/2. Going clockwise from
		// -π/2 by the same angle covers the same points.
		start := float32(-math.Pi / 2)
		sweep := 2 * math.Pi * frac * prog
		f := float32(i) / float32(items)
		c := gui.RGBA(uint8(200-150*f), uint8(200-50*f), uint8(100+50*f), alpha)
		dc.Arc(cx, cy, r, r, start, sweep, c, lw)
		// Emulated: round caps. go-gui strokes have no caps, so each end gets a
		// half disc. The half disc starts at the end's own radial angle, so it
		// meets the stroke along one diameter and does not overlap it (an
		// overlap would show at alpha < 255).
		end := start + sweep
		sx, sy := cx+r*cos(start), cy+r*sin(start)
		ex, ey := cx+r*cos(end), cy+r*sin(end)
		dc.FilledArc(sx, sy, lw/2, lw/2, start, -math.Pi, c)
		dc.FilledArc(ex, ey, lw/2, lw/2, end, math.Pi, c)
		r -= lw + margin
	}
}

// lineGraph draws a smooth curve through items samples. The area under it has a
// vertical gradient, the curve has a gray stroke, and each sample has a dot. h is
// negative: the graph grows up from the baseline y.
func (s *scene) lineGraph(dc *gui.DrawContext, x, y, w, h float32, items int,
	t float32) {
	dx := w / float32(items-1)
	dot := 4 + w*0.005

	// Flatten the cubic segments. Qt's control points sit dx/2 to each side of
	// the samples, at the samples' own heights.
	s.curve = s.curve[:0]
	px, py := x, y+h*sample(0, t)
	s.curve = append(s.curve, px, py)
	for i := 1; i < items; i++ {
		nx, ny := x+float32(i)*dx, y+h*sample(i, t)
		s.curve = appendCubic(s.curve, px, py, px+dx/2, py, nx-dx/2, ny, nx, ny)
		px, py = nx, ny
	}

	// Emulated: filled path. go-gui has no general path fill, so the area is
	// built as triangles. Its x coordinate only increases along the curve, so the
	// area is a strip of quads from each curve segment down to the baseline.
	s.tris = s.tris[:0]
	for i := 0; i+3 < len(s.curve); i += 2 {
		x0, y0 := s.curve[i], s.curve[i+1]
		x1, y1 := s.curve[i+2], s.curve[i+3]
		s.tris = append(s.tris,
			x0, y0, x1, y1, x1, y,
			x0, y0, x1, y, x0, y)
	}
	s.stops = [2]gui.GradientStop{
		{Color: colAreaLow, Pos: 0},
		{Color: colAreaHigh, Pos: 1},
	}
	s.grad = gui.CanvasGradient{
		Stops: s.stops[:],
		X1:    x, Y1: y,
		X2: x, Y2: y + h,
	}
	dc.FillTrianglesGradient(s.tris, &s.grad)

	// Qt strokes this with round joins (the Circles test sets them and nothing
	// resets them). go-gui joins are miter, falling back to bevel.
	dc.PolylineJoined(s.curve, colGray, 1+dot*0.2)

	for i := range items {
		cx, cy := x+float32(i)*dx, y+h*sample(i, t)
		dc.FilledCircle(cx, cy, dot*0.8, colWhite)
		dc.Circle(cx, cy, dot*0.8, colBlack, dot*0.2)
	}
}

// sample is the height of line graph sample i at time t, as a fraction of the
// graph height.
func sample(i int, t float32) float32 {
	return (0.5 + sin(float32(i+1)*t*0.2)*0.1) * 0.8
}

// barGraph draws items bars whose heights follow a sine wave. h is negative: bars
// grow up from the baseline y.
func (s *scene) barGraph(dc *gui.DrawContext, x, y, w, h float32, items int,
	t float32) {
	dx := w / float32(items)
	barWidth := dx * 0.8
	margin := dx - barWidth
	for i := range items {
		v := 0.5 + sin(float32(i)*0.1+t)*0.5
		sx := x + float32(i)*dx + margin/2
		// Qt truncates toward zero with (int) casts and offsets by half a pixel
		// to land the 1px stroke on pixel centers. The height is negative.
		bx := float32(int(sx)) + 0.5
		by := float32(int(y)) + 1.5
		bw := float32(int(barWidth))
		bh := float32(int(h * v))
		if bh == 0 {
			// Qt still strokes a zero-height rect, as a line.
			dc.Line(bx, by, bx+bw, by, colBlack, 1)
			continue
		}
		// go-gui rects need a positive height: move the top up instead.
		top, hh := by+bh, -bh
		dc.FilledRect(bx, top, bw, hh, colBar)
		dc.Rect(bx, top, bw, hh, colBlack, 1)
	}
}

// icons draws items small images that bob up and down, each with a number
// centered on it.
func (s *scene) icons(dc *gui.DrawContext, x, y, w, h, sz float32, items int,
	t float32) {
	size := 16 + sz*0.05
	style := s.style
	style.Size = size * 0.5
	style.Color = colWhite
	for i := range items {
		xp := x + (w-size)/float32(items)*float32(i)
		yp := y + h*0.5 + h*sin(float32(i+1)*t*0.1)*0.5
		dc.Image(xp, yp, size, size, s.iconSrc, gui.Opt[float32]{}, gui.Color{})
		centerText(dc, xp+size/2, yp+size/2, labels[i+1], style)
	}
}

// flowerDraw draws a six-petal flower that rocks ±20° and shifts color with t.
func (s *scene) flowerDraw(dc *gui.DrawContext, x, y, size, t float32) {
	cx, cy := x+size/2, y+size/2
	outline := s.flowerOutline(x, y, size)

	// Emulated: rotation. go-gui transforms are scale and translate only, so the
	// cached outline is rotated on the CPU every frame.
	a := sin(t) * 20 * math.Pi / 180
	ca, sa := cos(a), sin(a)
	s.rot = s.rot[:0]
	for i := 0; i+1 < len(outline); i += 2 {
		dx, dy := outline[i]-cx, outline[i+1]-cy
		s.rot = append(s.rot, cx+dx*ca-dy*sa, cy+dx*sa+dy*ca)
	}

	// Emulated: filled path. The outline's angle around the center only turns
	// one way (TestFanCoversFlower), so a triangle fan from the center covers
	// each petal exactly once.
	s.tris = s.tris[:0]
	for i := 0; i+3 < len(s.rot); i += 2 {
		s.tris = append(s.tris, cx, cy,
			s.rot[i], s.rot[i+1], s.rot[i+2], s.rot[i+3])
	}
	start := gui.RGBA(uint8((0.5+sin(t*2)*0.5)*255), 0,
		uint8((0.5+sin(t+math.Pi)*0.5)*255), 255)
	s.stops = [2]gui.GradientStop{
		{Color: start, Pos: 0},
		{Color: colWhite, Pos: 1},
	}
	s.grad = gui.CanvasGradient{
		Stops:  s.stops[:],
		Radial: true,
		CX:     cx, CY: cy, R: size / 2,
	}
	dc.FillTrianglesGradient(s.tris, &s.grad)
	dc.PolylineJoined(s.rot, colPetalLine, 4)

	// Qt draws the center dot after resetting the transform: it does not rotate.
	dc.FilledCircle(cx, cy, 0.1*size, colWhite)
}

// flowerOutline returns the closed flower outline for a size×size box at (x, y),
// unrotated. It is built once per box and cached, as Qt caches its path.
//
// The outline is six petals. Each petal is two quadratic curves: from the center
// out to a tip, and back to the center. The control and tip points are on a
// circle of radius size/2, 30° apart, going clockwise from the top.
func (s *scene) flowerOutline(x, y, size float32) []float32 {
	key := [3]float32{x, y, size}
	if s.flower != nil && s.flowerKey == key {
		return s.flower
	}
	cx, cy := x+size/2, y+size/2
	leaf := size / 2
	at := func(i int) (float32, float32) {
		a := 2*math.Pi*(1-float32(i)/12) - math.Pi/2
		return cx + cos(a)*leaf, cy + sin(a)*leaf
	}
	pts := make([]float32, 0, (12*flowerSegs+1)*2)
	pts = append(pts, cx, cy)
	for i := 0; i < 12; i += 2 {
		c1x, c1y := at(i)
		tx, ty := at(i + 1)
		c2x, c2y := at(i + 2)
		pts = appendQuad(pts, cx, cy, c1x, c1y, tx, ty)
		pts = appendQuad(pts, tx, ty, c2x, c2y, cx, cy)
	}
	s.flower, s.flowerKey = pts, key
	return pts
}

// appendQuad appends the flattened quadratic from (x0,y0) to (x2,y2), excluding
// the start point, which the caller already has.
func appendQuad(dst []float32, x0, y0, x1, y1, x2, y2 float32) []float32 {
	for k := 1; k <= flowerSegs; k++ {
		u := float32(k) / flowerSegs
		v := 1 - u
		dst = append(dst,
			v*v*x0+2*v*u*x1+u*u*x2,
			v*v*y0+2*v*u*y1+u*u*y2)
	}
	return dst
}

// appendCubic appends the flattened cubic from (x0,y0) to (x3,y3), excluding the
// start point, which the caller already has.
func appendCubic(dst []float32, x0, y0, x1, y1, x2, y2, x3, y3 float32) []float32 {
	for k := 1; k <= bezierSegs; k++ {
		u := float32(k) / bezierSegs
		v := 1 - u
		a, b, c, d := v*v*v, 3*v*v*u, 3*v*u*u, u*u*u
		dst = append(dst,
			a*x0+b*x1+c*x2+d*x3,
			a*y0+b*y1+c*y2+d*y3)
	}
	return dst
}

// centerText draws text centered on (cx, cy), like Qt's center alignment with a
// middle baseline. Text measures each string every call, as Qt does.
func centerText(dc *gui.DrawContext, cx, cy float32, text string,
	style gui.TextStyle) {
	tw := dc.TextWidth(text, style)
	fh := dc.FontHeight(style)
	dc.Text(cx-tw/2, cy-fh/2, text, style)
}

func sin(v float32) float32 { return float32(math.Sin(float64(v))) }
func cos(v float32) float32 { return float32(math.Cos(float64(v))) }
