package gui

// renderLayout walks the layout tree and emits RenderCmd entries
// into window.renderers. Clip rectangles bracket clipped children.
func renderLayout(layout *Layout, bgColor Color, clip drawClip, w *Window) {
	// Emit filter bracket when ColorFilter is set (containers only).
	fx := layout.Shape.fx
	hasColorFilter := fx != nil && fx.ColorFilter != nil && !w.inFilter
	if hasColorFilter {
		w.inFilter = true
		emitRenderer(RenderCmd{
			Kind:        RenderFilterBegin,
			BlurRadius:  fx.BlurRadius,
			Layers:      1,
			ColorMatrix: &fx.ColorFilter.Matrix,
		}, w)
	}

	renderShape(layout.Shape, bgColor, clip, w)

	shapeClip := clip
	if layout.Shape.OverDraw {
		shapeClip = layout.Shape.shapeClip
		if layout.Shape.ScrollbarOrientation == ScrollbarVertical {
			shapeClip.Y = clip.Y
			shapeClip.Height = clip.Height
		}
		if layout.Shape.ScrollbarOrientation == ScrollbarHorizontal {
			shapeClip.X = clip.X
			shapeClip.Width = clip.Width
		}
		emitClipCmd(shapeClip, w)
	} else if layout.Shape.Clip {
		sc := layout.Shape.shapeClip
		isRTL := effectiveTextDir(layout.Shape) == TextDirRTL
		var padX float32
		if isRTL {
			padX = layout.Shape.Padding.Right + layout.Shape.SizeBorder
		} else {
			padX = layout.Shape.PaddingLeft()
		}
		shapeClip = drawClip{
			X:      sc.X + padX,
			Y:      sc.Y + layout.Shape.PaddingTop(),
			Width:  f32Max(0, sc.Width-layout.Shape.paddingWidth()),
			Height: f32Max(0, sc.Height-layout.Shape.paddingHeight()),
		}
		emitClipCmd(shapeClip, w)
	}

	// Emit stencil clip bracket before children.
	didIncrement := false
	if layout.Shape.ClipContents {
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
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
			shapeClip = layout.Shape.shapeClip
			emitClipCmd(shapeClip, w)
		}
	}

	// Propagate rounded clip radius to child images.
	savedClipRadius := w.clipRadius
	w.clipRadius = resolveClipRadius(savedClipRadius, layout.Shape)

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
	}

	color := bgColor
	if layout.Shape.Color != ColorTransparent {
		color = layout.Shape.Color
	}
	for i := range layout.Children {
		renderLayout(&layout.Children[i], color, shapeClip, w)
	}

	if layout.Shape.QuarterTurns > 0 {
		emitRenderer(RenderCmd{Kind: RenderRotateEnd}, w)
	}

	w.clipRadius = savedClipRadius

	if layout.Shape.ClipContents {
		// Restore scissor if we pushed one.
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
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
	}

	if layout.Shape.Clip || layout.Shape.OverDraw {
		emitClipCmd(clip, w)
	}

	if hasColorFilter {
		emitRenderer(RenderCmd{Kind: RenderFilterEnd}, w)
		w.inFilter = false
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
		renderShapeInner(shape, parentColor, clip, w)
		shape.Color = origColor
		shape.ColorBorder = origBorder
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
		shape.fx.BorderGradient != nil)

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
		(fx.Shadow.BlurRadius > 0 || fx.Shadow.OffsetX != 0 || fx.Shadow.OffsetY != 0) {
		emitRenderer(RenderCmd{
			Kind:       RenderShadow,
			X:          shape.X,
			Y:          shape.Y,
			W:          shape.Width,
			H:          shape.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.Shadow.BlurRadius,
			Color:      fx.Shadow.Color,
			OffsetX:    fx.Shadow.OffsetX,
			OffsetY:    fx.Shadow.OffsetY,
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
			Color:  shape.Color,
			Shader: fx.Shader,
		}, w)
	} else

	// Gradient fill
	if hasFX && fx.Gradient != nil {
		emitRenderer(RenderCmd{
			Kind:     RenderGradient,
			X:        shape.X,
			Y:        shape.Y,
			W:        shape.Width,
			H:        shape.Height,
			Radius:   shape.Radius,
			Gradient: fx.Gradient,
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
				Gradient:  fx.BorderGradient,
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
				Gradient:  fx.BorderGradient,
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
