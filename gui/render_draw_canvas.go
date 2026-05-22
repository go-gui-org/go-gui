package gui

import "math"

// renderDrawCanvas renders cached draw-canvas triangle batches.
func renderDrawCanvas(shape *Shape, clip DrawClip, w *Window) {
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}
	// Background, border, effects.
	renderContainer(shape, ColorTransparent, clip, w)

	sm := StateMap[string, DrawCanvasCache](w, nsDrawCanvas, capModerate)

	// Content dimensions account for padding.
	cw := shape.Width - shape.PaddingWidth()
	ch := shape.Height - shape.PaddingHeight()

	scale := w.BackingScale
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		scale = 1
	}

	var cached DrawCanvasCache
	needsDraw := true

	// Skip cache when ID is empty to avoid collisions between
	// multiple ID-less DrawCanvas widgets.
	if shape.ID != "" {
		var ok bool
		cached, ok = sm.Get(shape.ID)
		if ok && cached.Version == shape.Version &&
			cached.TessWidth == cw && cached.TessHeight == ch &&
			cached.Scale == scale {
			needsDraw = false
		}
	}

	if needsDraw && shape.Events != nil && shape.Events.OnDraw != nil {
		dc := DrawContext{
			Width:       cw,
			Height:      ch,
			Scale:       scale,
			textMeasure: w.textMeasurer,
		}
		shape.Events.OnDraw(&dc)
		cached = DrawCanvasCache{
			Version:    shape.Version,
			TessWidth:  cw,
			TessHeight: ch,
			Scale:      scale,
			Batches:    dc.batches,
			Texts:      dc.texts,
			Images:     dc.images,
		}
		if shape.ID != "" {
			sm.Set(shape.ID, cached)
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
	if shape.Clip {
		emitClipCmd(DrawClip{X: ox, Y: oy, Width: cw, Height: ch}, w)
	}

	// Images emit first so they act as the back layer. DrawContext
	// records images in a separate slice from triangle batches and
	// text, so emission order is the only thing that decides z-stacking
	// between them. Putting images first lets tile-map consumers draw
	// markers / HUD chips / labels *over* tile images in the same
	// DrawCanvas. Reverse ordering would be correct only if triangles/
	// text were meant as backgrounds — which no in-tree consumer wants.
	emitDrawCanvasImages(cached.Images, ox, oy, w)

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
func emitDrawCanvasImages(
	images []DrawCanvasImageEntry, ox, oy float32, w *Window,
) {
	for i := range images {
		im := &images[i]
		if !isFiniteF(im.X) || !isFiniteF(im.Y) ||
			!isFiniteF(im.W) || !isFiniteF(im.H) ||
			im.W <= 0 || im.H <= 0 || im.Src == "" {
			continue
		}
		resource := ResolveImageSrcWithFetcher(w, im.Src, im.Fetcher)
		if resource == "" {
			continue
		}
		bg := ColorTransparent
		if im.BgColor.IsSet() {
			bg = im.BgColor
			op := im.BgOpacity.Get(1.0)
			if !isFiniteF(op) {
				op = 1.0
			}
			if op = clampUnit(op); op < 1.0 {
				bg = bg.WithOpacity(op)
			}
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
}
