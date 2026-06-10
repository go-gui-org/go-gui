package gui

// rendererValidForDraw checks whether a RenderCmd has valid
// parameters for drawing. Returns false for NaN/Inf coordinates,
// negative sizes, nil pointers, etc.
func rendererValidForDraw(r RenderCmd) bool {
	switch r.Kind {
	case RenderClip:
		return validClipCmd(r)
	case RenderRect:
		return validRectCmd(r)
	case RenderStrokeRect:
		return validStrokeRectCmd(r)
	case RenderGradient:
		return validGradientCmd(r)
	case RenderCircle:
		return validCircleCmd(r)
	case RenderText:
		return validTextCmd(r)
	case RenderLayout:
		return validLayoutCmd(r)
	case RenderLayoutTransformed:
		return validLayoutTransformedCmd(r)
	case RenderImage:
		return validImageCmd(r)
	case RenderSvg:
		return validSvgCmd(r)
	case RenderFilterComposite:
		return validFilterCompositeCmd(r)
	case RenderStencilBegin, RenderStencilEnd:
		return validStencilCmd(r)
	case RenderGradientBorder:
		return validGradientBorderCmd(r)
	case RenderShadow:
		return validShadowCmd(r)
	case RenderBlur:
		return validBlurCmd(r)
	case RenderCustomShader:
		return validCustomShaderCmd(r)
	case RenderRotateBegin:
		return validRotateBeginCmd(r)
	default:
		return true
	}
}

func validClipCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W >= 0 && r.H >= 0
}

func validRectCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W >= 0 && r.H >= 0
}

func validStrokeRectCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.Radius, r.Thickness) &&
		r.W >= 0 && r.H >= 0 && r.Thickness > 0
}

func validGradientCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W >= 0 && r.H >= 0 && r.Gradient != nil
}

func validCircleCmd(r RenderCmd) bool {
	return f32AllFinite3(r.X, r.Y, r.Radius) && r.Radius > 0
}

func validTextCmd(r RenderCmd) bool {
	return f32AllFinite2(r.X, r.Y) && len(r.Text) > 0
}

func validLayoutCmd(r RenderCmd) bool {
	return f32AllFinite2(r.X, r.Y) && r.LayoutPtr != nil
}

func validLayoutTransformedCmd(r RenderCmd) bool {
	return f32AllFinite2(r.X, r.Y) &&
		r.LayoutPtr != nil && r.LayoutTransform != nil
}

func validImageCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && f32IsFinite(r.ClipRadius)
}

func validSvgCmd(r RenderCmd) bool {
	if !f32AllFinite3(r.X, r.Y, r.Scale) || r.Scale <= 0 {
		return false
	}
	if r.HasVertexAlpha &&
		(!f32IsFinite(r.VertexAlphaScale) ||
			r.VertexAlphaScale < 0 ||
			r.VertexAlphaScale > 1) {
		return false
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 {
		return false
	}
	if !f32AllFinite(r.Triangles) {
		return false
	}
	if len(r.VertexColors) > 0 &&
		len(r.VertexColors)*2 != len(r.Triangles) {
		return false
	}
	return true
}

func validFilterCompositeCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && r.Layers > 0
}

func validStencilCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.Radius) &&
		r.W > 0 && r.H > 0 && r.StencilDepth > 0
}

func validGradientBorderCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.Radius, r.Thickness) &&
		r.W >= 0 && r.H >= 0 && r.Thickness > 0 && r.Gradient != nil
}

func validShadowCmd(r RenderCmd) bool {
	return f32AllFinite6(r.X, r.Y, r.W, r.H, r.BlurRadius, r.Radius) &&
		r.W >= 0 && r.H >= 0 &&
		f32AllFinite2(r.OffsetX, r.OffsetY)
}

func validBlurCmd(r RenderCmd) bool {
	return f32AllFinite5(r.X, r.Y, r.W, r.H, r.BlurRadius) &&
		r.W >= 0 && r.H >= 0
}

func validCustomShaderCmd(r RenderCmd) bool {
	return f32AllFinite4(r.X, r.Y, r.W, r.H) &&
		r.W > 0 && r.H > 0 && r.Shader != nil
}

func validRotateBeginCmd(r RenderCmd) bool {
	return f32AllFinite3(r.RotAngle, r.RotCX, r.RotCY)
}

// f32AllFinite checks if all values in a slice are finite.
func f32AllFinite(values []float32) bool {
	for _, v := range values {
		if !f32IsFinite(v) {
			return false
		}
	}
	return true
}

func f32AllFinite2(a, b float32) bool {
	return f32IsFinite(a) && f32IsFinite(b)
}

func f32AllFinite3(a, b, c float32) bool {
	return f32IsFinite(a) && f32IsFinite(b) && f32IsFinite(c)
}

func f32AllFinite4(a, b, c, d float32) bool {
	return f32IsFinite(a) && f32IsFinite(b) &&
		f32IsFinite(c) && f32IsFinite(d)
}

func f32AllFinite5(a, b, c, d, e float32) bool {
	return f32AllFinite4(a, b, c, d) && f32IsFinite(e)
}

func f32AllFinite6(a, b, c, d, e, f float32) bool {
	return f32AllFinite4(a, b, c, d) &&
		f32IsFinite(e) && f32IsFinite(f)
}

// guardRendererOrSkip returns true if valid. Logs a warning (once
// per kind) for invalid renderers.
func guardRendererOrSkip(r RenderCmd, w *Window) bool {
	if rendererValidForDraw(r) {
		return true
	}
	bit := uint32(1) << r.Kind
	if w.renderGuardWarned&bit == 0 {
		w.renderGuardWarned |= bit
	}
	return false
}

// emitRendererIfValid appends r to the window's renderers if valid.
// Returns true if appended.
func emitRendererIfValid(r RenderCmd, w *Window) bool {
	if !rendererValidForDraw(r) {
		return false
	}
	w.renderers = append(w.renderers, r)
	return true
}

// emitRenderer appends r to the window's renderers, logging a
// warning if the renderer is invalid.
func emitRenderer(r RenderCmd, w *Window) {
	if emitRendererIfValid(r, w) {
		return
	}
	guardRendererOrSkip(r, w)
}
