//go:build !js

package sdl2

import (
	"math"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

type roundedClipState struct {
	tex        *sdl.Texture
	prevTarget *sdl.Texture
	cmd        gui.RenderCmd
}

// renderersDraw iterates render commands and draws them.
func (b *Backend) renderersDraw(w *gui.Window) {
	cmds := w.Renderers()
	for i := range cmds {
		r := &cmds[i]
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderRect:
			b.drawRect(r)
		case gui.RenderStrokeRect:
			b.drawStrokeRect(r)
		case gui.RenderText:
			b.drawText(r)
		case gui.RenderCircle:
			b.drawCircle(r)
		case gui.RenderLine:
			b.drawLine(r)
		case gui.RenderShadow:
			b.drawShadow(r)
		case gui.RenderBlur:
			b.drawBlur(r)
		case gui.RenderGradient:
			b.drawGradient(r)
		case gui.RenderGradientBorder:
			b.drawGradientBorder(r)
		case gui.RenderImage:
			b.drawImage(r)
		case gui.RenderSvg:
			b.drawSvg(r)
		case gui.RenderLayout:
			b.drawLayout(r)
		case gui.RenderLayoutTransformed:
			b.drawLayoutTransformed(r)
		case gui.RenderTextPath:
			b.drawTextPath(r)
		case gui.RenderRTF:
			b.drawRtf(r)
		case gui.RenderFilterBegin:
			b.beginFilter(r)
		case gui.RenderFilterEnd:
			b.endFilter()
		case gui.RenderStencilBegin:
			b.beginStencilClip(r)
		case gui.RenderStencilEnd:
			b.endStencilClip()

		case gui.RenderRotateBegin,
			gui.RenderRotateEnd:
			// Rotation requires MVP transform — unsupported in SDL2.
		case gui.RenderFilterComposite,
			gui.RenderLayoutPlaced,
			gui.RenderCustomShader:
			// Skip — requires GPU pipeline or unsupported in SDL2.
		default:
			// Unimplemented render kinds are silently skipped.
		}
	}
}

func (b *Backend) currentTargetSize(
	target *sdl.Texture,
) (int32, int32, error) {
	if target == nil {
		return b.renderer.GetOutputSize()
	}
	_, _, w, h, err := target.Query()
	return w, h, err
}

func (b *Backend) currentClipState() (sdl.Rect, bool) {
	return b.renderer.GetClipRect(), b.renderer.IsClipEnabled()
}

func (b *Backend) applyClipState(rect sdl.Rect, enabled bool) {
	if enabled {
		_ = b.renderer.SetClipRect(&rect)
		return
	}
	_ = b.renderer.SetClipRect(nil)
}

func (b *Backend) beginStencilClip(r *gui.RenderCmd) {
	if r.Radius <= 0 {
		b.drawClip(r)
		return
	}
	b.beginRoundedClip(r)
}

func (b *Backend) endStencilClip() {
	if len(b.roundedClipStack) == 0 {
		return
	}
	state := b.roundedClipStack[len(b.roundedClipStack)-1]
	b.roundedClipStack = b.roundedClipStack[:len(b.roundedClipStack)-1]

	currClip, clipEnabled := b.currentClipState()
	_ = b.renderer.SetRenderTarget(state.prevTarget)
	b.applyClipState(currClip, clipEnabled)

	dst := sdl.FRect{
		X: state.cmd.X * b.dpiScale,
		Y: state.cmd.Y * b.dpiScale,
		W: state.cmd.W * b.dpiScale,
		H: state.cmd.H * b.dpiScale,
	}
	srcNorm, ok := textureRegionUV(state.tex, dst)
	if ok {
		b.drawTextureRoundedRegion(
			state.tex, dst, srcNorm, state.cmd.Radius*b.dpiScale)
	}
	_ = state.tex.Destroy()
}

func (b *Backend) beginRoundedClip(r *gui.RenderCmd) {
	prevTarget := b.renderer.GetRenderTarget()
	currClip, clipEnabled := b.currentClipState()
	w, h, err := b.currentTargetSize(prevTarget)
	if err != nil {
		b.drawClip(r)
		return
	}

	tex, err := b.renderer.CreateTexture(
		sdl.PIXELFORMAT_RGBA8888,
		sdl.TEXTUREACCESS_TARGET,
		w, h,
	)
	if err != nil {
		b.drawClip(r)
		return
	}
	_ = tex.SetBlendMode(sdl.BLENDMODE_BLEND)

	b.roundedClipStack = append(b.roundedClipStack, roundedClipState{
		tex:        tex,
		prevTarget: prevTarget,
		cmd:        *r,
	})

	_ = b.renderer.SetRenderTarget(tex)
	b.applyClipState(currClip, clipEnabled)
	_ = b.renderer.SetDrawColor(0, 0, 0, 0)
	_ = b.renderer.Clear()
}

func (b *Backend) drawClip(r *gui.RenderCmd) {
	s := b.dpiScale
	rect := sdl.Rect{
		X: int32(r.X * s),
		Y: int32(r.Y * s),
		W: int32(r.W * s),
		H: int32(r.H * s),
	}
	_ = b.renderer.SetClipRect(&rect)
}

func (b *Backend) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	s := b.dpiScale
	if r.Radius > 0 {
		b.fillRoundedRect(
			r.X*s, r.Y*s, r.W*s, r.H*s,
			r.Radius*s, r.Color)
		return
	}
	_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
	rect := sdl.FRect{
		X: r.X * s, Y: r.Y * s,
		W: r.W * s, H: r.H * s,
	}
	_ = b.renderer.FillRectF(&rect)
}

func (b *Backend) drawStrokeRect(r *gui.RenderCmd) {
	s := b.dpiScale
	if r.Radius > 0 {
		b.strokeRoundedRect(
			r.X*s, r.Y*s, r.W*s, r.H*s,
			r.Radius*s, r.Color)
		return
	}
	_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
	rect := sdl.FRect{
		X: r.X * s, Y: r.Y * s,
		W: r.W * s, H: r.H * s,
	}
	_ = b.renderer.DrawRectF(&rect)
}

func (b *Backend) drawText(r *gui.RenderCmd) {
	if b.textSys == nil || len(r.Text) == 0 {
		return
	}
	var cfg glyph.TextConfig
	if r.TextStylePtr != nil {
		cfg = guiStyleToGlyphConfig(*r.TextStylePtr)
		cfg.Gradient = r.TextGradient
	} else {
		cfg = glyph.TextConfig{
			Style: glyph.TextStyle{
				FontName: r.FontName,
				Size:     r.FontSize,
				Color: glyph.Color{
					R: r.Color.R,
					G: r.Color.G,
					B: r.Color.B,
					A: r.Color.A,
				},
			},
			Block: glyph.DefaultBlockStyle(),
		}
	}
	if r.W > 0 {
		cfg.Block.Wrap = glyph.WrapWord
		cfg.Block.Width = r.W
	}
	_ = b.textSys.DrawText(r.X, r.Y, r.Text, cfg)
}

func (b *Backend) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	s := b.dpiScale
	cx := r.X * s
	cy := r.Y * s
	rad := r.Radius * s
	_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
	ri := int(rad)
	for dy := -ri; dy <= ri; dy++ {
		dx := int(math.Sqrt(float64(rad*rad) - float64(dy*dy)))
		y := cy + float32(dy)
		_ = b.renderer.DrawLineF(cx-float32(dx), y, cx+float32(dx), y)
	}
}

// fillRoundedRect draws a filled rectangle with rounded corners
// using three rectangles and four filled quarter-circle fans.
func (b *Backend) fillRoundedRect(x, y, w, h, rad float32, c gui.Color) {
	rad = min(rad, w/2, h/2)
	_ = b.renderer.SetDrawColor(c.R, c.G, c.B, c.A)

	// Center strip.
	_ = b.renderer.FillRectF(&sdl.FRect{
		X: x + rad, Y: y, W: w - 2*rad, H: h,
	})
	// Left strip.
	_ = b.renderer.FillRectF(&sdl.FRect{
		X: x, Y: y + rad, W: rad, H: h - 2*rad,
	})
	// Right strip.
	_ = b.renderer.FillRectF(&sdl.FRect{
		X: x + w - rad, Y: y + rad, W: rad, H: h - 2*rad,
	})

	// Four corner quarter-circles (scanline fill).
	ri := int(rad)
	for dy := 0; dy <= ri; dy++ {
		dx := int(math.Sqrt(float64(rad*rad) - float64(dy*dy)))
		fy := float32(dy)
		fx := float32(dx)
		// Top-left.
		_ = b.renderer.DrawLineF(x+rad-fx, y+rad-fy, x+rad, y+rad-fy)
		// Top-right.
		_ = b.renderer.DrawLineF(x+w-rad, y+rad-fy, x+w-rad+fx, y+rad-fy)
		// Bottom-left.
		_ = b.renderer.DrawLineF(x+rad-fx, y+h-rad+fy, x+rad, y+h-rad+fy)
		// Bottom-right.
		_ = b.renderer.DrawLineF(x+w-rad, y+h-rad+fy, x+w-rad+fx, y+h-rad+fy)
	}
}

func (b *Backend) drawLine(r *gui.RenderCmd) {
	s := b.dpiScale
	x0 := r.X * s
	y0 := r.Y * s
	x1 := r.OffsetX * s
	y1 := r.OffsetY * s

	dx := x1 - x0
	dy := y1 - y0
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 0.001 {
		return
	}

	thick := max(r.Thickness*s, 1.0)
	nx := -dy / length * thick * 0.5
	ny := dx / length * thick * 0.5

	c := sdl.Color{
		R: r.Color.R,
		G: r.Color.G,
		B: r.Color.B,
		A: r.Color.A,
	}
	verts := []sdl.Vertex{
		{
			Position: sdl.FPoint{X: x0 + nx, Y: y0 + ny},
			Color:    c,
		},
		{
			Position: sdl.FPoint{X: x1 + nx, Y: y1 + ny},
			Color:    c,
		},
		{
			Position: sdl.FPoint{X: x1 - nx, Y: y1 - ny},
			Color:    c,
		},
		{
			Position: sdl.FPoint{X: x0 - nx, Y: y0 - ny},
			Color:    c,
		},
	}
	indices := []int32{0, 1, 2, 0, 2, 3}
	_ = b.renderer.RenderGeometry(nil, verts, indices)
}

func (b *Backend) drawShadow(r *gui.RenderCmd) {
	s := b.dpiScale
	// Hard shadow: single offset rect, no blur.
	if r.BlurRadius == 0 {
		x := (r.X + r.OffsetX) * s
		y := (r.Y + r.OffsetY) * s
		w := r.W * s
		h := r.H * s
		if r.Radius > 0 {
			b.fillRoundedRect(x, y, w, h, r.Radius*s, r.Color)
		} else {
			_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
			_ = b.renderer.FillRectF(&sdl.FRect{X: x, Y: y, W: w, H: h})
		}
		return
	}
	// 3 concentric offset rects at decreasing alpha.
	for i := range 3 {
		off := float32(i+1) * r.BlurRadius * 0.5 * s
		a := r.Color.A / uint8(i+2)
		x := (r.X+r.OffsetX)*s - off
		y := (r.Y+r.OffsetY)*s - off
		w := r.W*s + 2*off
		h := r.H*s + 2*off
		if r.Radius > 0 {
			c := gui.RGBA(r.Color.R, r.Color.G, r.Color.B, a)
			b.fillRoundedRect(x, y, w, h, r.Radius*s+off, c)
		} else {
			_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, a)
			_ = b.renderer.FillRectF(&sdl.FRect{X: x, Y: y, W: w, H: h})
		}
	}
}

func (b *Backend) drawBlur(r *gui.RenderCmd) {
	// Placeholder: single rect at reduced alpha.
	s := b.dpiScale
	a := r.Color.A / 2
	_ = b.renderer.SetDrawColor(r.Color.R, r.Color.G, r.Color.B, a)
	rect := sdl.FRect{
		X: r.X * s, Y: r.Y * s,
		W: r.W * s, H: r.H * s,
	}
	_ = b.renderer.FillRectF(&rect)
}

func (b *Backend) drawGradient(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	s := b.dpiScale
	dx, dy := gui.GradientDir(r.Gradient, r.W, r.H)
	stops := gui.NormalizeGradientStopsInto(
		r.Gradient.Stops, &b.normBuf, &b.sampledBuf)
	if len(stops) == 0 {
		return
	}

	const bands = 32
	// Determine if gradient is more horizontal or vertical.
	horizontal := math.Abs(float64(dx)) >= math.Abs(float64(dy))

	for i := range bands {
		t := float32(i) / float32(bands)
		t2 := float32(i+1) / float32(bands)
		c := gui.SampleGradientStopColor(stops, (t+t2)/2)
		_ = b.renderer.SetDrawColor(c.R, c.G, c.B, c.A)
		var rect sdl.FRect
		if horizontal {
			rect = sdl.FRect{
				X: (r.X + t*r.W) * s,
				Y: r.Y * s,
				W: (t2 - t) * r.W * s,
				H: r.H * s,
			}
		} else {
			rect = sdl.FRect{
				X: r.X * s,
				Y: (r.Y + t*r.H) * s,
				W: r.W * s,
				H: (t2 - t) * r.H * s,
			}
		}
		_ = b.renderer.FillRectF(&rect)
	}
}

func (b *Backend) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	stops := gui.NormalizeGradientStopsInto(
		r.Gradient.Stops, &b.normBuf, &b.sampledBuf)
	if len(stops) == 0 {
		return
	}
	s := b.dpiScale
	th := r.Thickness * s
	// 4 border rects with sampled colors at 0.0, 0.25, 0.5, 0.75.
	positions := [4]float32{0.0, 0.25, 0.5, 0.75}
	rects := [4]sdl.FRect{
		{X: r.X * s, Y: r.Y * s, W: r.W * s, H: th},          // top
		{X: r.X * s, Y: (r.Y+r.H)*s - th, W: r.W * s, H: th}, // bottom
		{X: r.X * s, Y: r.Y * s, W: th, H: r.H * s},          // left
		{X: (r.X+r.W)*s - th, Y: r.Y * s, W: th, H: r.H * s}, // right
	}
	for i := range 4 {
		c := gui.SampleGradientStopColor(stops, positions[i])
		_ = b.renderer.SetDrawColor(c.R, c.G, c.B, c.A)
		_ = b.renderer.FillRectF(&rects[i])
	}
}

// strokeRoundedRect draws a stroked rectangle with rounded corners
// using four lines and four quarter-circle arcs (midpoint algorithm).
func (b *Backend) strokeRoundedRect(x, y, w, h, rad float32, c gui.Color) {
	rad = min(rad, w/2, h/2)
	_ = b.renderer.SetDrawColor(c.R, c.G, c.B, c.A)

	// Straight edges.
	_ = b.renderer.DrawLineF(x+rad, y, x+w-rad, y)     // top
	_ = b.renderer.DrawLineF(x+rad, y+h, x+w-rad, y+h) // bottom
	_ = b.renderer.DrawLineF(x, y+rad, x, y+h-rad)     // left
	_ = b.renderer.DrawLineF(x+w, y+rad, x+w, y+h-rad) // right

	// Quarter-circle arcs at each corner (midpoint algorithm).
	cx := [4]float32{x + rad, x + w - rad, x + w - rad, x + rad}
	cy := [4]float32{y + rad, y + rad, y + h - rad, y + h - rad}
	sx := [4]float32{-1, 1, 1, -1}
	sy := [4]float32{-1, -1, 1, 1}

	ri := int(rad)
	px, py := ri, 0
	d := 1 - ri
	for px >= py {
		fx, fy := float32(px), float32(py)
		for i := range 4 {
			_ = b.renderer.DrawPointF(cx[i]+fx*sx[i], cy[i]+fy*sy[i])
			_ = b.renderer.DrawPointF(cx[i]+fy*sx[i], cy[i]+fx*sy[i])
		}
		py++
		if d < 0 {
			d += 2*py + 1
		} else {
			px--
			d += 2*(py-px) + 1
		}
	}
}

// drawTextPath renders text along an SVG path using per-glyph
// placement with rotation following the path tangent.
func (b *Backend) drawTextPath(r *gui.RenderCmd) {
	if b.textSys == nil || r.TextPath == nil || r.TextStylePtr == nil {
		return
	}
	tp := r.TextPath

	cfg := guiStyleToGlyphConfig(*r.TextStylePtr)
	layout, err := b.textSys.LayoutTextCached(r.Text, cfg)
	if err != nil {
		return
	}
	positions := layout.GlyphPositions()
	if len(positions) == 0 {
		return
	}

	// Compute total advance from real glyph metrics.
	var totalAdvance float32
	for _, p := range positions {
		totalAdvance += p.Advance
	}

	// Apply text-anchor adjustment.
	offset := tp.Offset
	switch tp.Anchor {
	case gui.SvgTextAnchorMiddle:
		offset -= totalAdvance / 2
	case gui.SvgTextAnchorEnd:
		offset -= totalAdvance
	}

	// method=stretch: scale advances to fill remaining path.
	advScale := float32(1)
	if tp.Method == gui.SvgTextPathMethodStretch && totalAdvance > 0 {
		remaining := tp.TotalLen - offset
		if remaining > 0 {
			advScale = remaining / totalAdvance
		}
	}

	// Build per-glyph placements along the path.
	n := len(layout.Glyphs)
	if cap(b.textPathPlacements) < n {
		b.textPathPlacements = make([]glyph.GlyphPlacement, n)
	}
	placements := b.textPathPlacements[:n]
	for i := range placements {
		placements[i] = glyph.GlyphPlacement{X: -9999, Y: -9999}
	}

	cumAdv := float32(0)
	for _, p := range positions {
		advance := p.Advance * advScale
		centerDist := offset + cumAdv + advance/2
		px, py, angle := gui.SamplePathAt(
			tp.Polyline, tp.Table, centerDist)

		// Offset glyph origin back along tangent by half
		// advance so glyph center sits on the path point.
		halfAdv := advance / 2
		cosA := float32(math.Cos(float64(angle)))
		sinA := float32(math.Sin(float64(angle)))
		gx := px + r.X - halfAdv*cosA
		gy := py + r.Y - halfAdv*sinA

		placements[p.Index] = glyph.GlyphPlacement{
			X: gx, Y: gy, Angle: angle,
		}
		cumAdv += advance
	}

	b.textSys.DrawLayoutPlaced(layout, placements)
}

func (b *Backend) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient,
		)
		return
	}
	b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
}

func (b *Backend) drawLayoutTransformed(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil ||
		r.LayoutTransform == nil {
		return
	}
	if r.TextGradient != nil {
		b.textSys.DrawLayoutTransformedWithGradient(
			*r.LayoutPtr, r.X, r.Y,
			*r.LayoutTransform, r.TextGradient,
		)
		return
	}
	b.textSys.DrawLayoutTransformed(
		*r.LayoutPtr, r.X, r.Y, *r.LayoutTransform,
	)
}

func (b *Backend) drawRtf(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
}
