package gui

import "math"

// renderDrawCanvas renders cached draw-canvas triangle batches.
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
		if ok && cached.Version == shape.Version &&
			cached.tessWidth == cw && cached.tessHeight == ch &&
			cached.Scale == scale {
			needsDraw = false
		}
	}

	if needsDraw && shape.events != nil && shape.events.OnDraw != nil {
		dc := DrawContext{
			Width:       cw,
			Height:      ch,
			Scale:       scale,
			textMeasure: w.textMeasurer,
		}
		shape.events.OnDraw(&dc)
		cached = drawCanvasCache{
			Version:    shape.Version,
			tessWidth:  cw,
			tessHeight: ch,
			Scale:      scale,
			Batches:    dc.batches,
			Texts:      dc.texts,
			Images:     dc.images,
		}
		if key != "" {
			sm.Set(key, cached)
		}
	}

	if len(cached.Batches) == 0 && len(cached.Texts) == 0 &&
		len(cached.Images) == 0 {
		return
	}

	// Content origin accounts for padding.
	ox := shape.X + shape.PaddingLeft()
	oy := shape.Y + shape.PaddingTop()

	// Clip to content area.
	effClip := clip
	if shape.Clip {
		effClip = drawClip{X: ox, Y: oy, Width: cw, Height: ch}
		emitClipCmd(effClip, w)
	}

	// Images emit first so they act as the back layer. DrawContext
	// records images in a separate slice from triangle batches and
	// text, so emission order is the only thing that decides z-stacking
	// between them. Putting images first lets tile-map consumers draw
	// markers / HUD chips / labels *over* tile images in the same
	// DrawCanvas. Reverse ordering would be correct only if triangles/
	// text were meant as backgrounds — which no in-tree consumer wants.
	emitDrawCanvasImages(cached.Images, ox, oy, effClip, w)

	for _, batch := range cached.Batches {
		emitRenderer(RenderCmd{
			Kind:      RenderSvg,
			Triangles: batch.Triangles,
			Color:     batch.Color,
			X:         ox,
			Y:         oy,
			Scale:     1.0,
		}, w)
	}

	// Emit deferred text commands.
	for i := range cached.Texts {
		t := &cached.Texts[i]
		fontAscent := t.Style.Size * 0.8
		var textWidth float32
		if w.textMeasurer != nil {
			fontAscent = w.textMeasurer.FontAscent(t.Style)
			textWidth = w.textMeasurer.TextWidth(t.Text, t.Style)
		}

		tx := ox + t.X
		ty := oy + t.Y
		rotated := t.Style.RotationRadians != 0

		if rotated {
			deg := t.Style.RotationRadians * (180 / math.Pi)
			emitRenderer(RenderCmd{
				Kind:     RenderRotateBegin,
				RotAngle: deg,
				RotCX:    tx,
				RotCY:    ty,
			}, w)
		}

		emitRenderer(RenderCmd{
			Kind:         RenderText,
			X:            tx,
			Y:            ty,
			Color:        t.Style.Color,
			Text:         t.Text,
			FontName:     t.Style.Family,
			FontSize:     t.Style.Size,
			FontAscent:   fontAscent,
			TextWidth:    textWidth,
			TextStylePtr: w.scratch.renderTextStyles.alloc(t.Style),
			TextGradient: t.Style.Gradient,
		}, w)

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
	images []DrawCanvasImageEntry, ox, oy float32, clip drawClip, w *Window,
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
			bg = im.BgColor
			op := im.bgOpacity.Get(1.0)
			if !isFiniteF(op) {
				op = 1.0
			}
			if op = clampUnit(op); op < 1.0 {
				bg = bg.WithOpacity(op)
			}
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
