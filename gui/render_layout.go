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

// renderOnlyAmend re-runs an animated widget's AmendLayout on a
// render-only frame, which reuses the last layout, before the widget
// and its children are drawn. Gated on the pass: in a full frame
// layoutAmend has just run it.
func renderOnlyAmend(layout *Layout, w *Window) {
	if !w.renderOnlyPass {
		return
	}
	if ev := layout.Shape.events; ev != nil && ev.amendOnRender &&
		ev.AmendLayout != nil {
		ev.AmendLayout(EventCtx{layout, nil, w})
	}
}

func renderLayoutDepth(layout *Layout, bgColor Color, clip drawClip, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	renderOnlyAmend(layout, w)
	// The transform bracket opens first, so the view's own fill, its
	// filter and all its children are drawn through it. Inside, the clip
	// the parent passed is in the wrong frame, so drawing culls against
	// nothing (xfUnboundedClip). The scissor still cuts in the window.
	//
	// A view turned to show its hidden back face draws nothing, nor do
	// its children.
	if xf := layout.Shape.xf; xf != nil {
		if opened, unbounded := renderTransformBegin(layout.Shape, clip, w); opened {
			if unbounded {
				clip = xfUnboundedClip
			}
			defer renderTransformEnd(w)
		} else if xf.hidden {
			return
		}
	}
	// Emit filter bracket when ColorFilter is set (containers only).
	fx := layout.Shape.fx
	hasColorFilter := fx != nil && fx.ColorFilter != nil && !w.inFilter
	if hasColorFilter {
		// The bracket opens only if the Begin landed: a non-finite
		// color matrix drops it (validFilterBeginCmd), and the End is
		// always valid, so emitting one regardless would leave the GPU
		// backends compositing a stale filter layer over the frame.
		// inFilter is set inside the same test, so a dropped Begin does
		// not suppress a descendant's own filter either.
		opened := emitRendererIfValid(RenderCmd{
			Kind:        RenderFilterBegin,
			BlurRadius:  sanitizeFilterBlur(fx.BlurRadius),
			Layers:      1,
			ColorMatrix: &fx.ColorFilter.Matrix,
		}, w)
		if opened {
			w.inFilter = true
			defer func() {
				emitRenderer(RenderCmd{Kind: RenderFilterEnd}, w)
				w.inFilter = false
			}()
		}
	}

	renderShape(layout.Shape, bgColor, clip, w)
	renderLayoutChildren(layout, bgColor, clip, w, depth)
	// A group-box title sits on the top border, in the band it reserves
	// above it, outside the box's content box. So the box draws it after
	// its children, which the title covers like a legend, and after the
	// brackets renderLayoutChildren opened around them, so in the
	// parent's clip, not its own. The transform and filter brackets
	// above are still open: the title turns and filters with its box
	// (#1238).
	if groupTitleOf(layout.Shape) != nil {
		renderGroupBoxTitle(layout, clip, w)
	}
}

// renderLayoutChildren draws layout's children inside the brackets the
// shape opens around them: an OverDraw or Clip scissor, the rounded
// stencil, the clip radius handed to images, the quarter-turn. They
// close as it returns, so the caller can draw over the children
// outside them. Split from renderLayoutDepth so that neither function
// has more defers than the compiler open-codes (at most 8, and defers
// times returns at most 15); past that every defer on this per-shape,
// per-frame path is a runtime record.
func renderLayoutChildren(layout *Layout, bgColor Color, clip drawClip, w *Window, depth int) {
	shapeClip := clip
	if layout.Shape.OverDraw {
		shapeClip = renderShapeClip(layout.Shape, clip, w)
		if layout.Shape.scrollbarOrientation == ScrollbarVertical {
			shapeClip.Y = clip.Y
			shapeClip.Height = clip.Height
		}
		if layout.Shape.scrollbarOrientation == ScrollbarHorizontal {
			shapeClip.X = clip.X
			shapeClip.Width = clip.Width
		}
		emitClipCmd(shapeClip, w)
		defer func() {
			emitClipCmd(clip, w)
		}()
	} else if layout.Shape.Clip {
		shapeClip = clipContentBoxIn(layout.Shape,
			renderShapeClip(layout.Shape, clip, w))
		emitClipCmd(shapeClip, w)
		stencil, stencilled := xfClipStencilBegin(shapeClip, w)
		defer func() {
			if stencilled {
				xfClipStencilEnd(stencil, w)
			}
			emitClipCmd(clip, w)
		}()
	}

	// Emit stencil clip bracket before children.
	if layout.Shape.clipContents {
		// StencilDepth is a uint8 and the GPU stencil buffer saturates
		// at 255, so a bracket that cannot claim a depth of its own is
		// skipped whole rather than emitted at the parent's depth: its
		// End would decrement the coverage the parent still needs, and
		// every later sibling of that parent would clip against a value
		// one too low. The scissor below still bounds the subtree, so
		// the loss at 255 nested clipping containers is the rounded
		// corner, not the clip. maxEventDepth (256) allows one level
		// past the cap, which is how this became reachable at all.
		stencilled := w.stencilDepth < 255
		// The mask is the area inside the border, with the inner radius
		// (outer radius less the border). Children sit inside the border,
		// and a mask with the outer rect lets a child's fill cover the
		// border at each rounded corner (issue #820). Begin and End use
		// the same quad: End decrements what Begin incremented.
		// A non-finite border draws nothing (renderContainer skips it), so
		// it insets nothing. max(0, NaN) is NaN, which would make every
		// stencil field NaN and the render guard would drop the mask.
		sb := layout.Shape.SizeBorder
		if !f32IsFinite(sb) {
			sb = 0
		}
		sb = max(0, sb)
		// The body, not the bounds: a group-box title's band sits
		// above the border (containerBody). A Circle masks to its circle.
		body, radius := contentMask(layout.Shape)
		stencil := RenderCmd{
			X:      body.X + sb,
			Y:      body.Y + sb,
			W:      max(0, body.Width-2*sb),
			H:      max(0, body.Height-2*sb),
			Radius: max(0, radius-sb),
		}
		if stencilled {
			w.stencilDepth++
			stencil.Kind = RenderStencilBegin
			stencil.StencilDepth = w.stencilDepth
			emitRenderer(stencil, w)
		}
		// Also apply scissor clip as optimization (avoids
		// rasterizing fragments outside bounding rect).
		scissored := false
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
			shapeClip = renderShapeClip(layout.Shape, clip, w)
			emitClipCmd(shapeClip, w)
			scissored = true
		}
		defer func() {
			// Restore scissor if we pushed one.
			if scissored {
				emitClipCmd(clip, w)
			}
			if stencilled {
				stencil.Kind = RenderStencilEnd
				stencil.StencilDepth = w.stencilDepth
				emitRenderer(stencil, w)
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

	// Emit rotation bracket before children. The children are drawn in
	// their unturned frame, so their clip is turned back into it.
	if layout.Shape.QuarterTurns > 0 {
		shapeClip = renderQuarterTurnBegin(layout.Shape, shapeClip, w)
		defer renderQuarterTurnEnd(w)
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
	hasFX := shape.fx != nil && (shape.fx.Gradient != nil ||
		shape.fx.BorderGradient != nil || shape.fx.Shadow != nil ||
		shape.fx.Shader != nil)

	isRTF := shape.shapeType == shapeRTF

	if shape.Color == ColorTransparent && !hasFX && !hasBorder &&
		!hasText && !isImage && !isSvg && !isCanvas && !isRTF {
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
	case shapeNone:
		// no-op
	}
}

// renderContainer draws a rectangle (possibly with shadow, gradient,
// blur, or border) over its body: the shape's bounds, less the band a
// group-box title reserves above its border (containerBody).
func renderContainer(shape *Shape, _ Color, clip drawClip, w *Window) {
	fx := shape.fx
	hasFX := fx != nil
	body := containerBody(shape)

	// Shadow
	if hasFX && fx.Shadow != nil &&
		fx.Shadow.Color.A > 0 &&
		(fx.Shadow.BlurRadius > 0 || fx.Shadow.OffsetX != 0 ||
			fx.Shadow.OffsetY != 0 || fx.Shadow.Spread > 0) {
		emitRenderer(RenderCmd{
			Kind:       RenderShadow,
			X:          body.X,
			Y:          body.Y,
			W:          body.Width,
			H:          body.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.Shadow.BlurRadius,
			Spread:     fx.Shadow.Spread,
			Color: dimColor(fx.Shadow.Color,
				shape.Opacity, shape.Disabled),
			OffsetX: fx.Shadow.OffsetX,
			OffsetY: fx.Shadow.OffsetY,
		}, w)
	}

	// Fill. Exactly one of these paints the body. The border is not
	// part of any branch: it is drawn after the switch, so every fill
	// gets one (issue #589).
	switch {
	case hasFX && fx.Shader != nil:
		// Append first, then populate the pointed-to built-in block. If
		// validation rejects the command, the shader was not drawn and
		// must neither start its clock nor keep the frame loop running.
		if emitRendererIfValid(RenderCmd{
			Kind:   RenderCustomShader,
			X:      body.X,
			Y:      body.Y,
			W:      body.Width,
			H:      body.Height,
			Radius: shape.Radius,
			// Opacity 1: renderShape already scaled shape.Color
			// by shape.Opacity, so only the disabled dim is left.
			// The gradient and shadow siblings pass shape.Opacity
			// because their colors are separate fields.
			Color: dimColor(shape.Color,
				1.0, fillDims(shape)),
			Shader:         fx.Shader,
			shaderBuiltins: &fx.shaderBuiltins,
		}, w) {
			w.packShaderShape(shape, body.Width, body.Height)
			if fx.Shader.Surface != nil {
				w.pixelSurfaces.drew = true
			}
		}
	case hasFX && fx.Gradient != nil:
		emitRenderer(RenderCmd{
			Kind:   RenderGradient,
			X:      body.X,
			Y:      body.Y,
			W:      body.Width,
			H:      body.Height,
			Radius: shape.Radius,
			Gradient: dimmedGradient(fx.Gradient,
				shape.Opacity, shape.Disabled),
		}, w)
	case hasFX && fx.BlurRadius > 0 && shape.Color.A > 0 &&
		fx.ColorFilter == nil:
		// SDF blur (skipped when ColorFilter is set; FBO blur
		// handles it via the filter bracket pipeline).
		c := shape.Color
		if fillDims(shape) {
			c = dimAlpha(c)
		}
		emitRenderer(RenderCmd{
			Kind:       RenderBlur,
			X:          body.X,
			Y:          body.Y,
			W:          body.Width,
			H:          body.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.BlurRadius,
			Color:      c,
		}, w)
	default:
		// A BorderGradient used to take this branch in place of the
		// solid fill, so a container with both lost its Color.
		renderRectangleFill(shape, clip, w)
	}

	// Border, on top of the fill.
	if rectsOverlap(body, clip) {
		renderShapeBorder(shape, body, shape.Radius, w)
	}
}

// fillDims reports whether a shape's own fill takes the disabled dim.
// A disabled shape that carries an explicit disabled fill
// (ColorSet.Disabled, #741) paints that color as given: the caller
// chose the disabled look, so halving it again would not be that look.
// Only the fill is exempt; border, text and children still dim.
func fillDims(shape *Shape) bool {
	return shape.Disabled && !shape.colorDisabled.IsSet()
}

// renderRectangleFill draws a shape's solid body, if it has a visible
// Color and overlaps the clip.
func renderRectangleFill(shape *Shape, clip drawClip, w *Window) {
	dr := containerBody(shape)
	c := shape.Color
	if fillDims(shape) {
		c = dimAlpha(c)
	}
	if c.A > 0 && rectsOverlap(dr, clip) {
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
}

// renderShapeBorder draws a shape's border over its bounds dr. One
// border at most: a BorderGradient wins over ColorBorder. Nothing is
// drawn when SizeBorder is 0 or when the solid border color is
// invisible. The caller has already culled dr against the clip. radius
// is a parameter because a circle strokes at half its short side, not
// at shape.Radius.
func renderShapeBorder(shape *Shape, dr drawClip, radius float32, w *Window) {
	// Written as !(> 0), not <= 0, so a NaN width is skipped here, as
	// it was before, instead of reaching emitRenderer and tripping the
	// render guard.
	if !(shape.SizeBorder > 0) {
		return
	}
	if fx := shape.fx; fx != nil && fx.BorderGradient != nil {
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
		return
	}
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

// circleRing returns the circle a Circle draws: the largest that fits
// its body (the shape's rectangular region, less the band a group-box
// title reserves above it, containerBody), centred in it. It gives the
// square around the circle, its centre and its radius. The fill, the
// border, the ClipContents mask and the clip radius all take it, so
// they cannot drift apart. The body need not be square (a title's band
// leaves a square box's body wider than tall); a stroke or a mask over
// the whole of it would be a rounded box around the circle.
func circleRing(shape *Shape) (square drawClip, cx, cy, radius float32) {
	body := containerBody(shape)
	radius = f32Min(body.Width, body.Height) / 2
	cx = body.X + body.Width/2
	cy = body.Y + body.Height/2
	// A square body is its own square, kept exact.
	square = body
	if body.Width != body.Height {
		square = drawClip{X: cx - radius, Y: cy - radius,
			Width: 2 * radius, Height: 2 * radius}
	}
	return square, cx, cy, radius
}

// contentMask returns the rectangle and corner radius a ClipContents
// container masks its children to, before the border inset: its body
// with its corner radius, or a Circle's circle.
func contentMask(shape *Shape) (drawClip, float32) {
	if shape.shapeType == shapeCircle {
		square, _, _, radius := circleRing(shape)
		return square, radius
	}
	return containerBody(shape), shape.Radius
}

// renderCircle draws a shape as a circle, the one circleRing places.
func renderCircle(shape *Shape, clip drawClip, w *Window) {
	ring, cx, cy, radius := circleRing(shape)
	c := shape.Color
	if fillDims(shape) {
		c = dimAlpha(c)
	}

	if rectsOverlap(ring, clip) {
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

		renderShapeBorder(shape, ring, radius, w)
	}
}

// Text rendering functions are in render_text.go.
