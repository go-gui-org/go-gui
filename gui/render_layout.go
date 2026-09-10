package gui

// renderLayout walks the layout tree and emits RenderCmd entries
// into window.renderers. Clip rectangles bracket clipped children.
//
// Every bracket opened here is closed by a deferred call, so a panic
// in a child unwinds through balanced Begin/End pairs and restores
// inFilter, stencilDepth and clipRadius. That state lives on the
// Window across frames, so leaking it would corrupt every later
// frame, not just the aborted one.
func renderLayout(layout *Layout, bgColor Color, clip drawClip, w *Window) {
	renderLayoutDepth(layout, bgColor, clip, w, 0)
}

func renderLayoutDepth(layout *Layout, bgColor Color, clip drawClip, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	// Emit filter bracket when ColorFilter is set (containers only).
	fx := layout.Shape.fx
	hasColorFilter := fx != nil && fx.ColorFilter != nil && !w.inFilter
	if hasColorFilter {
		w.inFilter = true
		emitRenderer(RenderCmd{
			Kind:        RenderFilterBegin,
			BlurRadius:  fx.BlurRadius,
			Layers:      1,
			ColorMatrix: &fx.ColorFilter.matrix,
		}, w)
		defer func() {
			emitRenderer(RenderCmd{Kind: RenderFilterEnd}, w)
			w.inFilter = false
		}()
	}

	renderShape(layout.Shape, bgColor, clip, w)

	shapeClip := clip
	if layout.Shape.OverDraw {
		shapeClip = layout.Shape.shapeClip
		if layout.Shape.scrollbarOrientation == scrollbarVertical {
			shapeClip.Y = clip.Y
			shapeClip.Height = clip.Height
		}
		if layout.Shape.scrollbarOrientation == scrollbarHorizontal {
			shapeClip.X = clip.X
			shapeClip.Width = clip.Width
		}
		emitClipCmd(shapeClip, w)
		defer func() {
			emitClipCmd(clip, w)
		}()
	} else if layout.Shape.Clip {
		shapeClip = clipContentBox(layout.Shape)
		emitClipCmd(shapeClip, w)
		defer func() {
			emitClipCmd(clip, w)
		}()
	}

	// Emit stencil clip bracket before children.
	if layout.Shape.clipContents {
		didIncrement := false
		if w.stencilDepth < 255 {
			w.stencilDepth++
			didIncrement = true
		}
		emitRenderer(RenderCmd{
			Kind:         RenderStencilBegin,
			X:            layout.Shape.X,
			Y:            layout.Shape.Y,
			W:            layout.Shape.Width,
			H:            layout.Shape.Height,
			Radius:       layout.Shape.Radius,
			StencilDepth: w.stencilDepth,
		}, w)
		// Also apply scissor clip as optimization (avoids
		// rasterizing fragments outside bounding rect).
		scissored := false
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
			shapeClip = layout.Shape.shapeClip
			emitClipCmd(shapeClip, w)
			scissored = true
		}
		defer func() {
			// Restore scissor if we pushed one.
			if scissored {
				emitClipCmd(clip, w)
			}
			emitRenderer(RenderCmd{
				Kind:         RenderStencilEnd,
				X:            layout.Shape.X,
				Y:            layout.Shape.Y,
				W:            layout.Shape.Width,
				H:            layout.Shape.Height,
				Radius:       layout.Shape.Radius,
				StencilDepth: w.stencilDepth,
			}, w)
			if didIncrement {
				w.stencilDepth--
			}
		}()
	}

	// Propagate rounded clip radius to child images. Deferred only
	// when changed: most shapes are not clipping containers, and an
	// untouched value needs no restore.
	savedClipRadius := w.clipRadius
	w.clipRadius = resolveClipRadius(savedClipRadius, layout.Shape)
	if w.clipRadius != savedClipRadius {
		defer func() {
			w.clipRadius = savedClipRadius
		}()
	}

	// Emit rotation bracket before children.
	if turns := layout.Shape.QuarterTurns; turns > 0 {
		cx := layout.Shape.X + layout.Shape.Width/2
		cy := layout.Shape.Y + layout.Shape.Height/2
		emitRenderer(RenderCmd{
			Kind:     RenderRotateBegin,
			RotAngle: float32(turns) * 90,
			RotCX:    cx,
			RotCY:    cy,
		}, w)
		defer func() {
			emitRenderer(RenderCmd{Kind: RenderRotateEnd}, w)
		}()
	}

	color := bgColor
	if layout.Shape.Color != ColorTransparent {
		color = layout.Shape.Color
	}
	for i := range layout.Children {
		renderLayoutDepth(&layout.Children[i], color, shapeClip, w, depth+1)
	}
}

// renderShape dispatches to the type-specific renderer, applying
// opacity when needed.
func renderShape(shape *Shape, parentColor Color, clip drawClip, w *Window) {
	// Degrade safely if a text-like shape is missing text config.
	if (shape.shapeType == shapeText || shape.shapeType == shapeRTF) &&
		shape.TC == nil {
		return
	}

	if shape.Opacity < 1.0 {
		origColor := shape.Color
		origBorder := shape.ColorBorder
		shape.Color = shape.Color.WithOpacity(shape.Opacity)
		shape.ColorBorder = shape.ColorBorder.WithOpacity(shape.Opacity)
		// Deferred: a panic below must not leave the shape dimmed
		// for the next frame. Shapes persist in the layout tree.
		defer func() {
			shape.Color = origColor
			shape.ColorBorder = origBorder
		}()
		renderShapeInner(shape, parentColor, clip, w)
	} else {
		renderShapeInner(shape, parentColor, clip, w)
	}
}

// renderShapeInner dispatches to the type-specific renderer after
// visibility checks.
func renderShapeInner(shape *Shape, parentColor Color, clip drawClip, w *Window) {
	hasBorder := shape.SizeBorder > 0 && shape.ColorBorder != ColorTransparent
	hasText := shape.shapeType == shapeText && shape.TC != nil
	isImage := shape.shapeType == shapeImage
	isSvg := shape.shapeType == shapeSVG
	isCanvas := shape.shapeType == shapeDrawCanvas
	isTermGrid := shape.shapeType == shapeTermGrid
	hasFX := shape.fx != nil && (shape.fx.Gradient != nil ||
		shape.fx.BorderGradient != nil || shape.fx.Shadow != nil)

	isRTF := shape.shapeType == shapeRTF

	if shape.Color == ColorTransparent && !hasFX && !hasBorder &&
		!hasText && !isImage && !isSvg && !isCanvas && !isRTF &&
		!isTermGrid {
		return
	}

	switch shape.shapeType {
	case shapeRectangle:
		renderContainer(shape, parentColor, clip, w)
	case shapeText:
		renderText(shape, clip, w)
	case shapeImage:
		renderImage(shape, clip, w)
	case shapeCircle:
		renderCircle(shape, clip, w)
	case shapeRTF:
		renderRtf(shape, clip, w)
	case shapeSVG:
		renderSvg(shape, clip, w)
	case shapeDrawCanvas:
		renderDrawCanvas(shape, clip, w)
	case shapeTermGrid:
		renderTermGrid(shape, clip, w)
	case shapeNone:
		// no-op
	}
}

// renderContainer draws a rectangle (possibly with shadow, gradient,
// blur, or border).
func renderContainer(shape *Shape, _ Color, clip drawClip, w *Window) {
	fx := shape.fx
	hasFX := fx != nil

	// Shadow
	if hasFX && fx.Shadow != nil &&
		fx.Shadow.Color.A > 0 &&
		(fx.Shadow.BlurRadius > 0 || fx.Shadow.OffsetX != 0 ||
			fx.Shadow.OffsetY != 0 || fx.Shadow.Spread > 0) {
		emitRenderer(RenderCmd{
			Kind:       RenderShadow,
			X:          shape.X,
			Y:          shape.Y,
			W:          shape.Width,
			H:          shape.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.Shadow.BlurRadius,
			Spread:     fx.Shadow.Spread,
			Color: dimColor(fx.Shadow.Color,
				shape.Opacity, shape.Disabled),
			OffsetX: fx.Shadow.OffsetX,
			OffsetY: fx.Shadow.OffsetY,
		}, w)
	}

	// Custom shader
	if hasFX && fx.Shader != nil {
		emitRenderer(RenderCmd{
			Kind:   RenderCustomShader,
			X:      shape.X,
			Y:      shape.Y,
			W:      shape.Width,
			H:      shape.Height,
			Radius: shape.Radius,
			// Opacity 1: renderShape already scaled shape.Color
			// by shape.Opacity, so only the disabled dim is left.
			// The gradient and shadow siblings pass shape.Opacity
			// because their colors are separate fields.
			Color: dimColor(shape.Color,
				1.0, shape.Disabled),
			Shader: fx.Shader,
		}, w)
	} else

	// Gradient fill
	if hasFX && fx.Gradient != nil {
		emitRenderer(RenderCmd{
			Kind:   RenderGradient,
			X:      shape.X,
			Y:      shape.Y,
			W:      shape.Width,
			H:      shape.Height,
			Radius: shape.Radius,
			Gradient: dimmedGradient(fx.Gradient,
				shape.Opacity, shape.Disabled),
		}, w)
	} else if hasFX && fx.BlurRadius > 0 && shape.Color.A > 0 &&
		fx.ColorFilter == nil {
		// SDF blur (skipped when ColorFilter is set; FBO blur
		// handles it via the filter bracket pipeline).
		c := shape.Color
		if shape.Disabled {
			c = dimAlpha(c)
		}
		emitRenderer(RenderCmd{
			Kind:       RenderBlur,
			X:          shape.X,
			Y:          shape.Y,
			W:          shape.Width,
			H:          shape.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.BlurRadius,
			Color:      c,
		}, w)
	} else {
		// Border gradient or plain rectangle
		if hasFX && fx.BorderGradient != nil {
			emitRenderer(RenderCmd{
				Kind:      RenderGradientBorder,
				X:         shape.X,
				Y:         shape.Y,
				W:         shape.Width,
				H:         shape.Height,
				Radius:    shape.Radius,
				Thickness: shape.SizeBorder,
				Gradient: dimmedGradient(fx.BorderGradient,
					shape.Opacity, shape.Disabled),
			}, w)
		} else {
			renderRectangle(shape, clip, w)
		}
	}
}

// renderRectangle draws a shape as a filled rectangle with optional
// stroke border.
func renderRectangle(shape *Shape, clip drawClip, w *Window) {
	dr := shapeBounds(shape)
	c := shape.Color
	if shape.Disabled {
		c = dimAlpha(c)
	}

	if rectsOverlap(dr, clip) {
		// Fill
		if c.A > 0 {
			emitRenderer(RenderCmd{
				Kind:   RenderRect,
				X:      dr.X,
				Y:      dr.Y,
				W:      dr.Width,
				H:      dr.Height,
				Color:  c,
				Fill:   true,
				Radius: shape.Radius,
			}, w)
		}
		// Border
		if shape.SizeBorder > 0 {
			cb := shape.ColorBorder
			if shape.Disabled {
				cb = dimAlpha(cb)
			}
			if cb.A > 0 {
				emitRenderer(RenderCmd{
					Kind:      RenderStrokeRect,
					X:         dr.X,
					Y:         dr.Y,
					W:         dr.Width,
					H:         dr.Height,
					Color:     cb,
					Radius:    shape.Radius,
					Thickness: shape.SizeBorder,
				}, w)
			}
		}
	}
}

// renderCircle draws a shape as a circle in the middle of the
// shape's rectangular region.
func renderCircle(shape *Shape, clip drawClip, w *Window) {
	dr := shapeBounds(shape)
	c := shape.Color
	if shape.Disabled {
		c = dimAlpha(c)
	}

	if rectsOverlap(dr, clip) {
		radius := f32Min(shape.Width, shape.Height) / 2
		cx := shape.X + shape.Width/2
		cy := shape.Y + shape.Height/2

		if c.A > 0 {
			emitRenderer(RenderCmd{
				Kind:   RenderCircle,
				X:      cx,
				Y:      cy,
				Radius: radius,
				Fill:   true,
				Color:  c,
			}, w)
		}

		// Border
		fx := shape.fx
		if fx != nil && fx.BorderGradient != nil && shape.SizeBorder > 0 {
			emitRenderer(RenderCmd{
				Kind:      RenderGradientBorder,
				X:         dr.X,
				Y:         dr.Y,
				W:         dr.Width,
				H:         dr.Height,
				Radius:    radius,
				Thickness: shape.SizeBorder,
				Gradient: dimmedGradient(fx.BorderGradient,
					shape.Opacity, shape.Disabled),
			}, w)
		} else if shape.SizeBorder > 0 {
			cb := shape.ColorBorder
			if shape.Disabled {
				cb = dimAlpha(cb)
			}
			if cb.A > 0 {
				emitRenderer(RenderCmd{
					Kind:      RenderStrokeRect,
					X:         dr.X,
					Y:         dr.Y,
					W:         dr.Width,
					H:         dr.Height,
					Color:     cb,
					Radius:    radius,
					Thickness: shape.SizeBorder,
				}, w)
			}
		}
	}
}

// Text rendering functions are in render_text.go.
