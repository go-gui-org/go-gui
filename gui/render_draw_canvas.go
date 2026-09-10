package gui

import (
	"log"
	"math"
)

// callOnDrawSafe invokes the app's OnDraw callback, isolating a
// widget panic so one canvas cannot abort the frame's render walk.
// Returns false when the callback panicked; the caller then falls
// back to an empty canvas. Warns once per window.
func callOnDrawSafe(dc *DrawContext, shape *Shape, w *Window) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			if !w.drawPanicWarned {
				w.drawPanicWarned = true
				log.Printf("gui: DrawCanvas OnDraw panicked "+
					"(id %q) — canvas skipped: %v",
					shape.idKey(), r)
			}
		}
	}()
	shape.events.OnDraw(dc)
	return true
}

// renderDrawCanvas renders cached draw-canvas triangle batches.
//
//nolint:gocyclo // cache states x deferred text/image/gradient emit
func renderDrawCanvas(shape *Shape, clip drawClip, w *Window) {
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}
	// Background, border, effects.
	renderContainer(shape, ColorTransparent, clip, w)

	sm := StateMap[string, drawCanvasCache](w, nsDrawCanvas, capModerate)

	// Content dimensions account for padding.
	cw := shape.Width - shape.paddingWidth()
	ch := shape.Height - shape.paddingHeight()

	scale := w.BackingScale
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		scale = 1
	}

	var cached drawCanvasCache
	needsDraw := true

	// Skip cache when ID is empty to avoid collisions between
	// multiple ID-less DrawCanvas widgets.
	key := shape.idKey()
	if key != "" {
		var ok bool
		cached, ok = sm.Get(key)
		// The entry is still claimed for its buffers when
		// alwaysRedraw skips the version test — that is the half of
		// the cache an animated canvas actually uses.
		if ok && !shape.alwaysRedraw && cached.Version == shape.Version &&
			cached.tessWidth == cw && cached.tessHeight == ch &&
			cached.Scale == scale {
			needsDraw = false
		}
	}

	if needsDraw && shape.events != nil && shape.events.OnDraw != nil {
		// The outgoing entry is about to be replaced, so its buffers
		// are free for this pass to write into. An animated canvas
		// redraws every frame at nearly the same size, so after two
		// frames the whole tessellation runs allocation-free.
		dc := &w.scratch.canvasCtx
		reuse := cached
		if cached.pass == w.renderPass {
			// Already redrawn in this same list: its buffers are live
			// behind an emitted command. Start clean instead.
			reuse = drawCanvasCache{}
		}
		dc.resetFor(cw, ch, scale, w.textMeasurer, reuse)
		// A panic mid-tessellation leaves partial buffers that are
		// unusable, so the entry keeps its empty content: the canvas
		// draws nothing and the frame continues. Only a Version bump
		// (or alwaysRedraw) retries.
		cached = drawCanvasCache{
			Version:    shape.Version,
			pass:       w.renderPass,
			tessWidth:  cw,
			tessHeight: ch,
			Scale:      scale,
		}
		if callOnDrawSafe(dc, shape, w) {
			cached.Batches = dc.batches
			cached.spare = dc.batchPool
			cached.Gradients = dc.gradients
			cached.gradSpare = dc.gradientPool
			cached.Texts = dc.texts
			cached.Images = dc.images
		}
		if key != "" {
			sm.Set(key, cached)
		}
	}

	if len(cached.Batches) == 0 && len(cached.Texts) == 0 &&
		len(cached.Images) == 0 && len(cached.Gradients) == 0 {
		return
	}

	// Content origin accounts for padding.
	ox := shape.X + shape.PaddingLeft()
	oy := shape.Y + shape.PaddingTop()

	// Clip to content area, intersected with what the canvas already
	// inherits — the same bleed clipContentBox fixes for containers: a
	// canvas scrolled half out of its viewport has a content box whose
	// top edge is above that viewport, so emitting it raw paints the
	// canvas over whatever sits above the scroll panel.
	//
	// The origin here stays LTR (ox uses PaddingLeft unconditionally)
	// because a canvas draws in its own coordinate space, so this does
	// not go through clipContentBox, whose inset mirrors under RTL.
	// The clip must match this origin, not the engine's text direction.
	effClip := clip
	if shape.Clip {
		content := drawClip{X: ox, Y: oy, Width: cw, Height: ch}
		// No overlap leaves effClip empty, which clips the canvas away
		// entirely. That is the safe direction: an invalid or dropped
		// RenderClip would leave the parent's wider scissor in force.
		effClip, _ = rectIntersection(content, clip)
		emitClipCmd(effClip, w)
	}

	// Images emit first so they act as the back layer. DrawContext
	// records images in a separate slice from triangle batches and
	// text, so emission order is the only thing that decides z-stacking
	// between them. Putting images first lets tile-map consumers draw
	// markers / HUD chips / labels *over* tile images in the same
	// DrawCanvas. Reverse ordering would be correct only if triangles/
	// text were meant as backgrounds — which no in-tree consumer wants.
	emitDrawCanvasImages(cached.Images, ox, oy, effClip, shape, w)

	// A gradient batch differs only by its VertexColors; every backend
	// that consumes RenderSvg already reads that channel for SVG
	// gradients, so a canvas gradient needs nothing below this line.
	// validSvgCmd rejects a batch whose two lengths disagree.
	//
	// Lowered radial fills interleave here by the batch counter each
	// one recorded, so a halo drawn before the disc it surrounds still
	// paints before it. This is the only ordering the canvas keeps:
	// images are always the back layer and text always the front,
	// whatever order the OnDraw callback used.
	emitDrawCanvasGeometry(&cached, ox, oy, shape, w)

	// Emit deferred text commands.
	for i := range cached.Texts {
		t := &cached.Texts[i]
		fontAscent := t.Style.Size * 0.8
		var textWidth float32
		if w.textMeasurer != nil {
			fontAscent = w.textMeasurer.FontAscent(t.Style)
			textWidth = w.textMeasurer.TextWidth(t.Text, t.Style)
		}

		// Widget-level fade and disabled dim, mirroring
		// renderText: the cached style must not be dimmed in
		// place — the cache entry is recycled by the next
		// redraw — so dim a local copy. Metrics above stay
		// on the undimmed style; color carries no width.
		style := t.Style
		style.Color = dimColor(style.Color,
			shape.Opacity, shape.Disabled)
		style.BgColor = dimColor(style.BgColor,
			shape.Opacity, shape.Disabled)
		style.StrokeColor = dimColor(style.StrokeColor,
			shape.Opacity, shape.Disabled)

		tx := ox + t.X
		ty := oy + t.Y
		// Affine (skew/squash/arbitrary) takes precedence over
		// rotation. Rotation keeps the GPU MVP path via
		// RotateBegin/End; affine carries LayoutTransform on the
		// RenderText so the backend can draw a cached layout
		// with DrawLayoutTransformed (see #436). Identity
		// affines are treated as unset so they do not emit a
		// transform.
		hasAffine := t.Style.AffineTransform != nil &&
			!affineTransformIsIdentity(*t.Style.AffineTransform)
		rotated := !hasAffine && t.Style.RotationRadians != 0

		if rotated {
			deg := t.Style.RotationRadians * (180 / math.Pi)
			emitRenderer(RenderCmd{
				Kind:     RenderRotateBegin,
				RotAngle: deg,
				RotCX:    tx,
				RotCY:    ty,
			}, w)
		}

		cmd := RenderCmd{
			Kind:         RenderText,
			X:            tx,
			Y:            ty,
			Color:        style.Color,
			Text:         t.Text,
			FontName:     style.Family,
			FontSize:     style.Size,
			FontAscent:   fontAscent,
			TextWidth:    textWidth,
			TextStylePtr: w.scratch.renderTextStyles.alloc(style),
			TextGradient: dimmedTextGradient(style.Gradient,
				shape.Opacity, shape.Disabled),
		}
		if hasAffine {
			cmd.LayoutTransform = w.scratch.renderAffineTransforms.alloc(
				*t.Style.AffineTransform)
		}
		emitRenderer(cmd, w)

		if rotated {
			emitRenderer(RenderCmd{
				Kind: RenderRotateEnd,
			}, w)
		}
	}

	// Restore parent clip.
	if shape.Clip {
		emitClipCmd(clip, w)
	}
}

// emitDrawCanvasGeometry emits the triangle batches and the lowered
// radial fills in the order the OnDraw callback produced them, walking
// the two lists together by each gradient's batch counter.
func emitDrawCanvasGeometry(cached *drawCanvasCache,
	ox, oy float32, shape *Shape, w *Window) {
	gi := 0
	for bi := range cached.Batches {
		for gi < len(cached.Gradients) &&
			cached.Gradients[gi].afterBatch <= bi {
			emitDrawCanvasGradient(&cached.Gradients[gi], ox, oy,
				shape, w)
			gi++
		}
		batch := &cached.Batches[bi]
		// The canvas transform rides on the command, not on the
		// vertices: every backend applies v*S+T before the X/Y
		// origin and Scale below, so the triangles stay in the local
		// coordinates the caller drew in. Scale stays 1 — it is the
		// SVG path's own factor, and the two compose correctly.
		emitRenderer(RenderCmd{
			Kind:      RenderSvg,
			Triangles: batch.Triangles,
			VertexColors: dimmedVColors(batch.VertexColors,
				shape.Opacity, shape.Disabled, w),
			Color: dimColor(batch.Color,
				shape.Opacity, shape.Disabled),
			X:        ox,
			Y:        oy,
			Scale:    1.0,
			HasXform: batch.hasXform,
			ScaleX:   batch.xf.sx,
			ScaleY:   batch.xf.sy,
			TransX:   batch.xf.tx,
			TransY:   batch.xf.ty,
		}, w)
	}
	for ; gi < len(cached.Gradients); gi++ {
		emitDrawCanvasGradient(&cached.Gradients[gi], ox, oy,
			shape, w)
	}
}

// emitDrawCanvasGradient emits one lowered radial fill. The quad is
// the circle's bounding square, so the command's corner radius is
// half its width: that rounds the quad down to exactly the circle the
// shader's radial ramp already reaches the end of.
func emitDrawCanvasGradient(e *DrawCanvasGradientEntry,
	ox, oy float32, shape *Shape, w *Window) {
	emitRenderer(RenderCmd{
		Kind:     RenderGradient,
		Gradient: dimmedGradient(&e.Def, shape.Opacity, shape.Disabled),
		X:        ox + e.X,
		Y:        oy + e.Y,
		W:        e.W,
		H:        e.H,
		Radius:   e.W / 2,
	}, w)
}

// emitDrawCanvasImages emits RenderImage cmds for cached entries.
// Skips any entry with non-finite coords/size or empty src; clamps
// opacity into [0, 1]. Defense in depth: callers via DrawContext.Image
// already reject bad inputs, but the cache field is public.
//
// For http/https srcs the entry is resolved to a local cache path
// via ResolveImageSrc; an in-flight download returns "" and the
// emit is skipped this frame. The window redraws when the
// download completes.
//
// clip is the clip rect in effect for the canvas — the content box
// when the shape clips, otherwise the parent's. An entry drawn via
// ImageClipped narrows it to that entry's own rect (intersected,
// since RenderClip *replaces* the scissor rather than nesting) and
// restores clip afterwards, so a clipped entry cannot leak its
// scissor onto the entries that follow.
func emitDrawCanvasImages(
	images []DrawCanvasImageEntry, ox, oy float32, clip drawClip,
	shape *Shape, w *Window,
) {
	// narrowed tracks whether the scissor currently in the command stream is
	// one entry's own clip rather than the canvas clip, so the restore is
	// emitted exactly once — before the next unclipped entry, or after the
	// loop — instead of once per clipped entry.
	narrowed := false
	for i := range images {
		im := &images[i]
		if !isFiniteF(im.X) || !isFiniteF(im.Y) ||
			!isFiniteF(im.W) || !isFiniteF(im.H) ||
			im.W <= 0 || im.H <= 0 || im.Src == "" {
			continue
		}
		resource := resolveImageSrcWithFetcher(w, im.Src, im.fetcher)
		if resource == "" {
			continue
		}
		bg := ColorTransparent
		if im.BgColor.IsSet() {
			op := im.bgOpacity.Get(1.0)
			if !isFiniteF(op) {
				op = 1.0
			}
			// The entry's own opacity folds into the
			// widget's before the single multiply, so a
			// faded canvas over a faded entry dims once.
			bg = dimColor(im.BgColor,
				clampUnit(op)*shape.Opacity, shape.Disabled)
		}
		if im.Clipped {
			sub, ok := intersectClips(clip, drawClip{
				X: ox + im.ClipX, Y: oy + im.ClipY,
				Width: im.ClipW, Height: im.ClipH,
			})
			if !ok {
				continue // clipped away entirely
			}
			emitClipCmd(sub, w)
			narrowed = true
		} else if narrowed {
			emitClipCmd(clip, w)
			narrowed = false
		}
		emitRenderer(RenderCmd{
			Kind:     RenderImage,
			X:        ox + im.X,
			Y:        oy + im.Y,
			W:        im.W,
			H:        im.H,
			Color:    bg,
			Resource: resource,
		}, w)
	}
	if narrowed {
		emitClipCmd(clip, w)
	}
}

// intersectClips returns the overlap of two clip rects. Reports false when
// they do not overlap, or when either is degenerate.
func intersectClips(a, b drawClip) (drawClip, bool) {
	if !isFiniteF(b.X) || !isFiniteF(b.Y) ||
		!isFiniteF(b.Width) || !isFiniteF(b.Height) ||
		b.Width <= 0 || b.Height <= 0 {
		return drawClip{}, false
	}
	x := f32Max(a.X, b.X)
	y := f32Max(a.Y, b.Y)
	right := f32Min(a.X+a.Width, b.X+b.Width)
	bottom := f32Min(a.Y+a.Height, b.Y+b.Height)
	if right <= x || bottom <= y {
		return drawClip{}, false
	}
	return drawClip{X: x, Y: y, Width: right - x, Height: bottom - y}, true
}
