package gui

import (
	"github.com/go-gui-org/go-glyph"
)

// rectsOverlap checks if two rectangles overlap (strict <).
func rectsOverlap(r1, r2 drawClip) bool {
	return r1.X < (r2.X+r2.Width) && r2.X < (r1.X+r1.Width) &&
		r1.Y < (r2.Y+r2.Height) && r2.Y < (r1.Y+r1.Height)
}

// dimAlpha halves the alpha for visually indicating disabled state.
func dimAlpha(c Color) Color {
	c.A /= 2
	return c
}

// dimColor applies widget-level opacity, then the disabled dim, in
// that order: WithOpacity scales the caller's alpha, dimAlpha
// halves whatever remains. It mirrors what renderText does inline
// so every path that cannot rely on renderShape's Color mutation —
// shadows, gradients, images, grids, canvas content — dims
// identically. An opacity at or above 1 applies nothing; NaN
// compares false both ways and also applies nothing, matching
// renderShape, which takes the unmodified branch for NaN.
func dimColor(c Color, opacity float32, disabled bool) Color {
	if opacity < 1.0 {
		c = c.WithOpacity(opacity)
	}
	if disabled {
		c = dimAlpha(c)
	}
	return c
}

// dimmedGradient returns def unchanged when neither opacity nor
// disabled dimming applies; otherwise a copy with every stop run
// through dimColor. A RenderGradient command carries no color of
// its own, so a disabled or faded gradient container would
// otherwise paint at full strength while its rect sibling dims.
// The copy is heap-allocated, so callers must only reach it when
// dimming actually applies — the fast path above is that gate.
func dimmedGradient(
	def *GradientDef, opacity float32, disabled bool,
) *GradientDef {
	if def == nil || (!disabled && !(opacity < 1.0)) {
		return def
	}
	out := *def
	stops := make([]GradientStop, len(def.Stops))
	for i, s := range def.Stops {
		s.Color = dimColor(s.Color, opacity, disabled)
		stops[i] = s
	}
	out.Stops = stops
	return &out
}

// dimmedTextGradient returns cfg unchanged when neither opacity
// nor disabled dimming applies; otherwise a copy with every stop
// alpha scaled by opacity and halved when disabled. Text gradients
// live in the glyph module's color type, so this mirrors
// dimmedGradient across the module boundary — the conversion is a
// field-wise alpha copy, no cross-module change.
func dimmedTextGradient(
	cfg *glyph.GradientConfig, opacity float32, disabled bool,
) *glyph.GradientConfig {
	if cfg == nil || (!disabled && !(opacity < 1.0)) {
		return cfg
	}
	out := *cfg
	stops := make([]glyph.GradientStop, len(cfg.Stops))
	for i, s := range cfg.Stops {
		a := s.Color.A
		if opacity < 1.0 {
			a = uint8(float32(a) * f32Clamp(opacity, 0, 1))
		}
		if disabled {
			a /= 2
		}
		s.Color.A = a
		stops[i] = s
	}
	out.Stops = stops
	return &out
}

// dimmedTermGrid returns tg unchanged when neither opacity nor
// disabled dimming applies; otherwise a copy with every cell
// foreground/background, the cursor, the selection, and the grid
// font colors run through dimColor. The grid buffer is shared
// across frames, so dimming in place would persist into the next
// frame and stack — the copy is the same reason renderShape
// restores shape.Color via defer.
func dimmedTermGrid(
	tg *TermGridData, opacity float32, disabled bool,
) *TermGridData {
	if tg == nil || (!disabled && !(opacity < 1.0)) {
		return tg
	}
	out := *tg
	cells := make([]TermCell, len(tg.Cells))
	for i, c := range tg.Cells {
		c.FG = dimColor(c.FG, opacity, disabled)
		c.BG = dimColor(c.BG, opacity, disabled)
		cells[i] = c
	}
	out.Cells = cells
	out.Cursor.Color = dimColor(tg.Cursor.Color, opacity, disabled)
	out.Selection.Color = dimColor(
		tg.Selection.Color, opacity, disabled)
	out.Style.Color = dimColor(tg.Style.Color, opacity, disabled)
	out.Style.BgColor = dimColor(
		tg.Style.BgColor, opacity, disabled)
	out.Style.StrokeColor = dimColor(
		tg.Style.StrokeColor, opacity, disabled)
	return &out
}

// dimmedVColors returns vcols unchanged when neither opacity nor
// disabled dimming applies; otherwise an arena copy with every
// color run through dimColor. Canvas batches belong to the cache
// entry and are recycled by the next redraw, so — like the grid
// buffer above — they must never be dimmed in place.
func dimmedVColors(
	vcols []Color, opacity float32, disabled bool, w *Window,
) []Color {
	if len(vcols) == 0 || (!disabled && !(opacity < 1.0)) {
		return vcols
	}
	out := w.scratch.takeVColors(len(vcols))
	for i, c := range vcols {
		out[i] = dimColor(c, opacity, disabled)
	}
	return out
}

// resolveClipRadius computes the effective rounded clip radius for
// nested clipping containers.
func resolveClipRadius(parentRadius float32, shape *Shape) float32 {
	if !shape.Clip {
		return parentRadius
	}
	var baseRadius float32
	if shape.shapeType == shapeCircle {
		baseRadius = f32Min(shape.Width, shape.Height) / 2
	} else {
		baseRadius = shape.Radius
	}
	if !f32IsFinite(baseRadius) || baseRadius <= 0 {
		return parentRadius
	}
	leftInset := shape.Padding.Left + shape.SizeBorder
	rightInset := shape.Padding.Right + shape.SizeBorder
	topInset := shape.Padding.Top + shape.SizeBorder
	bottomInset := shape.Padding.Bottom + shape.SizeBorder
	inset := max(leftInset, rightInset, topInset, bottomInset)
	localRadius := f32Max(0, baseRadius-inset)
	if localRadius <= 0 {
		return parentRadius
	}
	if !f32IsFinite(parentRadius) || parentRadius <= 0 {
		return localRadius
	}
	return f32Min(parentRadius, localRadius)
}

// roundedImageClip holds clipped image draw parameters including
// mapped UV coordinates for SDF rounded clipping.
type roundedImageClip struct {
	X, Y   float32
	W, H   float32
	U0, V0 float32
	U1, V1 float32
}

// roundedImageClipParams computes the intersection of an image rect
// and a clip rect, mapping UV coordinates for the visible portion.
// Returns ok=false if there is no overlap.
func roundedImageClipParams(imgX, imgY, imgW, imgH float32, clip drawClip) (roundedImageClip, bool) {
	if imgW <= 0 || imgH <= 0 || clip.Width <= 0 || clip.Height <= 0 {
		return roundedImageClip{}, false
	}
	imgRight := imgX + imgW
	imgBottom := imgY + imgH
	clipRight := clip.X + clip.Width
	clipBottom := clip.Y + clip.Height
	x := f32Max(imgX, clip.X)
	y := f32Max(imgY, clip.Y)
	right := f32Min(imgRight, clipRight)
	bottom := f32Min(imgBottom, clipBottom)
	w := right - x
	h := bottom - y
	if w <= 0 || h <= 0 {
		return roundedImageClip{}, false
	}
	// Shrink into content box instead of cropping edge pixels.
	isInside := x >= imgX && y >= imgY && right <= imgRight && bottom <= imgBottom
	clipsSize := w < imgW || h < imgH
	anchorTopLeft := x == imgX && y == imgY
	if isInside && clipsSize && anchorTopLeft {
		return roundedImageClip{
			X: x, Y: y, W: w, H: h,
			U0: -1, V0: -1, U1: 1, V1: 1,
		}, true
	}
	invW := float32(2.0) / imgW
	invH := float32(2.0) / imgH
	return roundedImageClip{
		X: x, Y: y, W: w, H: h,
		U0: -1 + (x-imgX)*invW,
		V0: -1 + (y-imgY)*invH,
		U1: -1 + (right-imgX)*invW,
		V1: -1 + (bottom-imgY)*invH,
	}, true
}

// shapeBounds returns the shape's bounding rectangle as a
// drawClip.
func shapeBounds(shape *Shape) drawClip {
	return drawClip{
		X: shape.X, Y: shape.Y,
		Width: shape.Width, Height: shape.Height,
	}
}

// clipContentBox returns the clip rect a clipping container imposes
// on its children: the shape's padding-inset content box, intersected
// with the clip the shape itself already inherits (shapeClip, which
// is bounds ∩ ancestor clips).
//
// The inset is taken from the shape's *bounds*, never from the
// already-clipped rect: a container scrolled half out of its viewport
// has a shapeClip whose top edge is the viewport, and adding the
// padding there both hides a band of real content and — because the
// height is measured from the uninset top — lets the bottom escape.
// Deriving the box from the bounds and intersecting once is the only
// form that stays correct under partial clipping.
func clipContentBox(shape *Shape) drawClip {
	var padX float32
	if effectiveTextDir(shape) == TextDirRTL {
		padX = shape.Padding.Right + shape.SizeBorder
	} else {
		padX = shape.PaddingLeft()
	}
	content := drawClip{
		X:      shape.X + padX,
		Y:      shape.Y + shape.PaddingTop(),
		Width:  f32Max(0, shape.Width-shape.paddingWidth()),
		Height: f32Max(0, shape.Height-shape.paddingHeight()),
	}
	r, ok := rectIntersection(content, shape.shapeClip)
	if !ok {
		return drawClip{}
	}
	return r
}

// emitClipCmd emits a RenderClip command for the given clip rect.
func emitClipCmd(clip drawClip, w *Window) {
	emitRenderer(RenderCmd{
		Kind: RenderClip,
		X:    clip.X,
		Y:    clip.Y,
		W:    clip.Width,
		H:    clip.Height,
	}, w)
}

// quantizedScissorClip truncates clip coordinates to integer
// multiples of scale, matching sokol's scissor rect behavior.
func quantizedScissorClip(clip drawClip, scale float32) drawClip {
	if scale <= 0 {
		return clip
	}
	sx := int(clip.X * scale)
	sy := int(clip.Y * scale)
	sw := int(clip.Width * scale)
	sh := int(clip.Height * scale)
	return drawClip{
		X:      float32(sx) / scale,
		Y:      float32(sy) / scale,
		Width:  float32(sw) / scale,
		Height: float32(sh) / scale,
	}
}

// svgCmdVertex maps one vertex of a RenderSvg command to page space,
// in the same order the GPU and soft backends use: the command's own
// affine (an animateTransform, or a canvas Translate/ScaleBy) first,
// then the command origin and scale.
//
// Split out from pdfRenderSvg so the order is assertable without a
// PDF writer in the way.
func svgCmdVertex(cmd RenderCmd, svgScale, vx, vy float32) (float32, float32) {
	if cmd.HasXform {
		vx = vx*cmd.ScaleX + cmd.TransX
		vy = vy*cmd.ScaleY + cmd.TransY
	}
	return cmd.X + vx*svgScale, cmd.Y + vy*svgScale
}
